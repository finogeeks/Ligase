package util

import (
	"context"
	"fmt"

	//"encoding/json"
	"math/rand"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/finogeeks/ligase/plugins/message/internals"
	log "github.com/finogeeks/ligase/skunkworks/log"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// JSONResponse represents an HTTP response which contains a JSON body.
type JSONResponse struct {
	// HTTP status code.
	Code int
	// JSON represents the JSON that should be serialized and sent to the client
	JSON interface{}
	// Headers represent any headers that should be sent to the client
	Headers map[string]string
}

// Is2xx returns true if the Code is between 200 and 299.
func (r JSONResponse) Is2xx() bool {
	return r.Code/100 == 2
}

// RedirectResponse returns a JSONResponse which 302s the client to the given location.
func RedirectResponse(location string) JSONResponse {
	headers := make(map[string]string)
	headers["Location"] = location
	return JSONResponse{
		Code:    302,
		JSON:    struct{}{},
		Headers: headers,
	}
}

// MessageResponse returns a JSONResponse with a 'message' key containing the given text.
func MessageResponse(code int, msg string) JSONResponse {
	return JSONResponse{
		Code: code,
		JSON: struct {
			Message string `json:"message"`
		}{msg},
	}
}

// ErrorResponse returns an HTTP 500 JSONResponse with the stringified form of the given error.
func ErrorResponse(err error) JSONResponse {
	return MessageResponse(500, err.Error())
}

// JSONRequestHandler represents an interface that must be satisfied in order to respond to incoming
// HTTP requests with JSON.
type JSONRequestHandler interface {
	OnIncomingRequest(req *http.Request) JSONResponse
}

// jsonRequestHandlerWrapper is a wrapper to allow in-line functions to conform to util.JSONRequestHandler
type jsonRequestHandlerWrapper struct {
	function func(req *http.Request) JSONResponse
}

// OnIncomingRequest implements util.JSONRequestHandler
func (r *jsonRequestHandlerWrapper) OnIncomingRequest(req *http.Request) JSONResponse {
	return r.function(req)
}

// NewJSONRequestHandler converts the given OnIncomingRequest function into a JSONRequestHandler
func NewJSONRequestHandler(f func(req *http.Request) JSONResponse) JSONRequestHandler {
	return &jsonRequestHandlerWrapper{f}
}

// Protect panicking HTTP requests from taking down the entire process, and log them using
// the correct logger, returning a 500 with a JSON response rather than abruptly closing the
// connection. The http.Request MUST have a ctxValueLogger.
func Protect(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				fields := GetLogFields(req.Context())
				fields = append(fields, log.KeysAndValues{"panic", r}...)
				log.Errorw(fmt.Sprintf("Request panicked!\n%s", debug.Stack()), fields)
				respond(w, req, MessageResponse(500, "Internal Server Error"))
			}
		}()
		handler(w, req)
	}
}

// RequestWithLogging sets up standard logging for http.Requests.
// http.Requests will have a logger (with a request ID/method/path logged) attached to the Context.
// This can be accessed via GetLogger(Context).
func RequestWithLogging(req *http.Request) *http.Request {
	reqID := RandomString(12)
	// Set a Logger and request ID on the context
	ctx := ContextWithLogFields(req.Context(), log.KeysAndValues{
		"req.method", req.Method, "req.path", req.URL.Path, "req.id", reqID,
	})

	ctx = context.WithValue(ctx, ctxValueRequestID, reqID)
	req = req.WithContext(ctx)

	fields := GetLogFields(req.Context())
	log.Printw("Incoming request", fields)

	return req
}

// MakeJSONAPI creates an HTTP handler which always responds to incoming requests with JSON responses.
// Incoming http.Requests will have a logger (with a request ID/method/path logged) attached to the Context.
// This can be accessed via GetLogger(Context).
func MakeJSONAPI(handler JSONRequestHandler) http.HandlerFunc {
	return Protect(func(w http.ResponseWriter, req *http.Request) {
		now := time.Now()
		req = RequestWithLogging(req)

		if req.Method == "OPTIONS" {
			SetCORSHeaders(w)
			w.WriteHeader(200)
			return
		}
		last := now
		now = time.Now()
		log.Debugf("MakeJSONAPI before handle use %v\n", now.Sub(last))
		last = now

		res := handler.OnIncomingRequest(req)

		now = time.Now()
		log.Debugf("MakeJSONAPI handle use %v\n", now.Sub(last))
		last = now

		// Set common headers returned regardless of the outcome of the request
		w.Header().Set("Content-Type", "application/json")
		SetCORSHeaders(w)

		now = time.Now()
		log.Debugf("MakeJSONAPI set handler use %v\n", now.Sub(last))
		last = now

		respond(w, req, res)

		now = time.Now()
		log.Debugf("MakeJSONAPI respond use %v\n", now.Sub(last))
		last = now
	})
}

func respond(w http.ResponseWriter, req *http.Request, res JSONResponse) {
	fields := GetLogFields(req.Context())

	// Set custom headers
	if res.Headers != nil {
		for h, val := range res.Headers {
			w.Header().Set(h, val)
		}
	}

	// Marshal JSON response into raw bytes to send as the HTTP body
	var resBytes []byte
	if v, ok := res.JSON.(*internals.RawBytes); ok {
		resBytes = *v
	} else {
		var err error
		resBytes, err = json.Marshal(res.JSON)
		if err != nil {
			log.Errorw("Failed to marshal JSONResponse", log.KeysAndValues{"error", err})
			// this should never fail to be marshalled so drop err to the floor
			res = MessageResponse(500, "Internal Server Error")
			resBytes, _ = json.Marshal(res.JSON)
		}
	}

	// Set status code and write the body
	w.WriteHeader(res.Code)
	fields = append(fields, log.KeysAndValues{"code", res.Code}...)
	log.Infow(fmt.Sprintf("Responding (%d bytes)", len(resBytes)), fields)
	//TODO: remove logger
	fields = append(fields, log.KeysAndValues{"resString", string(resBytes)}...)
	log.Debugw("Responding result", fields)
	w.Write(resBytes)
}

// WithCORSOptions intercepts all OPTIONS requests and responds with CORS headers. The request handler
// is not invoked when this happens.
func WithCORSOptions(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method == "OPTIONS" {
			SetCORSHeaders(w)
			return
		}
		handler(w, req)
	}
}

// SetCORSHeaders sets unrestricted origin Access-Control headers on the response writer
func SetCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
}

const alphanumerics = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RandomString generates a pseudo-random string of length n.
func RandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = alphanumerics[rand.Int63()%int64(len(alphanumerics))]
	}
	return string(b)
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

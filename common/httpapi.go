// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package common

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	util "github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	log "github.com/finogeeks/ligase/skunkworks/log"
	hm "github.com/finogeeks/ligase/skunkworks/monitor/go-client/httpmonitor"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
)

// MakeAuthAPI turns a util.JSONRequestHandler function into an http.Handler which checks the access token in the request.
func MakeAuthAPI(
	metricsName string, cache service.Cache, cfg config.Dendrite, devFilter *filter.SimpleFilter,
	histogram mon.LabeledHistogram, /* counter mon.LabeledCounter*/
	f func(*http.Request, *authtypes.Device) util.JSONResponse,
) http.Handler {
	h := func(req *http.Request) util.JSONResponse {
		start := time.Now()

		if config.DebugRequest {
			requestDump, err := httputil.DumpRequest(req, true)
			if err != nil {
				log.Errorf("MakeAuthAPI Debug request: %s %v", metricsName, err)
			}
			log.Infof("MakeAuthAPI Debug request: %s %s", metricsName, string(requestDump))
		}

		var resErr *util.JSONResponse
		token, err := extractAccessToken(req)
		if err != nil {
			log.Infof("missing token req:%s", req.RequestURI)
			resErr = &util.JSONResponse{
				Code: http.StatusUnauthorized,
				JSON: jsonerror.MissingToken(err.Error()),
			}
		}
		var device *authtypes.Device
		if resErr == nil {
			// device, resErr := auth.VerifyAccessToken(req, deviceDB)
			device, resErr = VerifyToken(token, req.RequestURI, cache, cfg, devFilter)
		}

		verifyTokenEnd := time.Now()
		log.Debugf("MakeAuthAPI before process use %v", verifyTokenEnd.Sub(start))

		if resErr != nil {
			return *resErr
		}

		res := f(req, device)

		duration := float64(time.Since(start)) / float64(time.Millisecond)
		code := strconv.Itoa(res.Code)
		if req.Method != "OPTION" {
			histogram.WithLabelValues(req.Method, metricsName, code).Observe(duration)
			// counter.WithLabelValues(req.Method, metricsName, code).Inc()
		}

		return res
	}
	return MakeExternalAPI(metricsName, h)
}

// MakeExternalAPI turns a util.JSONRequestHandler function into an http.Handler.
// This is used for APIs that are called from the internet.
func MakeExternalAPI(metricsName string, f func(*http.Request) util.JSONResponse) http.Handler {
	h := util.MakeJSONAPI(util.NewJSONRequestHandler(f))
	withSpan := func(w http.ResponseWriter, req *http.Request) {
		if config.DebugRequest {
			requestDump, err := httputil.DumpRequest(req, true)
			if err != nil {
				log.Errorf("MakeExternalAPI Debug request: %s %v", metricsName, err)
			}
			log.Infof("MakeExternalAPI Debug request: %s %s", metricsName, string(requestDump))
		}

		span := opentracing.StartSpan(metricsName)
		defer span.Finish()
		req = req.WithContext(opentracing.ContextWithSpan(req.Context(), span))
		h.ServeHTTP(w, req)
	}

	return http.HandlerFunc(withSpan)
}

// MakeInternalAPI turns a util.JSONRequestHandler function into an http.Handler.
// This is used for APIs that are internal to dendrite.
// If we are passed a tracing context in the request headers then we use that
// as the parent of any tracing spans we create.
func MakeInternalAPI(metricsName string, f func(*http.Request) util.JSONResponse) http.Handler {
	h := util.MakeJSONAPI(util.NewJSONRequestHandler(f))
	withSpan := func(w http.ResponseWriter, req *http.Request) {
		if config.DebugRequest {
			requestDump, err := httputil.DumpRequest(req, true)
			if err != nil {
				log.Errorf("MakeInternalAPI Debug request: %s %v", metricsName, err)
			}
			log.Infof("MakeInternalAPI Debug request: %s %s", metricsName, string(requestDump))
		}

		carrier := opentracing.HTTPHeadersCarrier(req.Header)
		tracer := opentracing.GlobalTracer()
		clientContext, err := tracer.Extract(opentracing.HTTPHeaders, carrier)
		var span opentracing.Span
		if err == nil {
			// Default to a span without RPC context.
			span = tracer.StartSpan(metricsName)
		} else {
			// Set the RPC context.
			span = tracer.StartSpan(metricsName, ext.RPCServerOption(clientContext))
		}
		defer span.Finish()
		req = req.WithContext(opentracing.ContextWithSpan(req.Context(), span))
		h.ServeHTTP(w, req)
	}

	return http.HandlerFunc(withSpan)
}

func MakeInternalAuthAPI(metricsName string, cache service.Cache, f func(*http.Request, *authtypes.Device) util.JSONResponse) http.Handler {
	h := func(req *http.Request) util.JSONResponse {
		now := time.Now()
		if config.DebugRequest {
			requestDump, err := httputil.DumpRequest(req, true)
			if err != nil {
				log.Errorf("MakeInternalAuthAPI Debug request: %s %v\n", metricsName, err)
			}
			log.Infof("MakeInternalAuthAPI Debug request: %s %s\n", metricsName, string(requestDump))
		}

		userId := req.URL.Query().Get("userId")
		if userId == "" {
			return util.JSONResponse{
				Code: http.StatusUnauthorized,
				JSON: jsonerror.MissingToken("missing userId"),
			}
		}

		var device authtypes.Device
		device.UserID = userId
		device.ID = userId
		device.DeviceType = "internal"
		device.IsHuman = true

		last := now
		now = time.Now()
		log.Debugf("MakeAuthAPI before process use %v\n", now.Sub(last))
		last = now

		return f(req, &device)
	}
	return MakeExternalAPI(metricsName, h)
}

// MakeFedAPI makes an http.Handler that checks matrix federation authentication.
func MakeFedAPI(
	metricsName string,
	serverName gomatrixserverlib.ServerName,
	keyRing gomatrixserverlib.KeyRing,
	histogram mon.LabeledHistogram, /*counter mon.LabeledCounter,*/
	f func(*http.Request, *gomatrixserverlib.FederationRequest) util.JSONResponse,
) http.Handler {
	h := func(req *http.Request) util.JSONResponse {
		start := time.Now()

		if config.DebugRequest {
			requestDump, err := httputil.DumpRequest(req, true)
			if err != nil {
				log.Errorf("MakeFedAPI Debug request: %s %v", metricsName, err)
			}
			log.Infof("MakeFedAPI Debug request: %s %s", metricsName, string(requestDump))
		}

		fedReq, _ /*errResp*/ := gomatrixserverlib.VerifyHTTPRequest(
			req, time.Now(), serverName, nil,
		)
		// if fedReq == nil {
		// 	return errResp
		// }
		res := f(req, fedReq)

		duration := float64(time.Since(start)) / float64(time.Millisecond)
		code := strconv.Itoa(res.Code)
		if req.Method != "OPTION" {
			histogram.WithLabelValues(req.Method, metricsName, code).Observe(duration)
			// counter.WithLabelValues(req.Method, metricsName, code).Inc()
		}

		return res
	}
	return MakeExternalAPI(metricsName, h)
}

// SetupHTTPAPI registers an HTTP API mux under /api and sets up a metrics
// listener.
func SetupHTTPAPI(servMux *http.ServeMux, apiMux http.Handler) {
	// This is deprecated.
	//servMux.Handle("/metrics", promhttp.Handler())
	servMux.Handle("/api/", hm.Wrap(StripPrefix("/api", apiMux)))
}

func StripPrefix(prefix string, h http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := strings.TrimPrefix(r.URL.Path, prefix); len(p) < len(r.URL.Path) {
			r2 := new(http.Request)
			*r2 = *r
			r2.URL = new(url.URL)
			*r2.URL = *r.URL
			r2.URL.Path = p
			h.ServeHTTP(w, r2)
		} else {
			http.NotFound(w, r)
		}
	})
}

// WrapHandlerInCORS adds CORS headers to all responses, including all error
// responses.
// Handles OPTIONS requests directly.
func WrapHandlerInCORS(h http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			// Its easiest just to always return a 200 OK for everything. Whether
			// this is technically correct or not is a question, but in the end this
			// is what a lot of other people do (including synapse) and the clients
			// are perfectly happy with it.
			w.WriteHeader(http.StatusOK)
		} else {
			h.ServeHTTP(w, r)
		}
	})
}

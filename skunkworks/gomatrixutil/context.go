package util

import (
	"context"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

// contextKeys is a type alias for string to namespace Context keys per-package.
type contextKeys string

// ctxValueRequestID is the key to extract the request ID for an HTTP request
const ctxValueRequestID = contextKeys("requestid")

// GetRequestID returns the request ID associated with this context, or the empty string
// if one is not associated with this context.
func GetRequestID(ctx context.Context) string {
	id := ctx.Value(ctxValueRequestID)
	if id == nil {
		return ""
	}
	return id.(string)
}

const ctxValueLogFields = contextKeys("logFields")

func GetLogFields(ctx context.Context) log.KeysAndValues {
	fields := ctx.Value(ctxValueLogFields)
	if fields == nil {
		return log.KeysAndValues{"context", "missing"}
	}
	return fields.(log.KeysAndValues)
}

// ContextWithLogger creates a new context, which will use the given logger.
func ContextWithLogFields(ctx context.Context, fields log.KeysAndValues) context.Context {
	return context.WithValue(ctx, ctxValueLogFields, fields)
}

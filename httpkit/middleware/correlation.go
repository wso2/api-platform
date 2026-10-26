package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

// CorrelationIDHeader is the canonical header name for the correlation / trace ID.
const CorrelationIDHeader = "X-Correlation-ID"

// maxCorrelationIDLength fits a UUID, a 32-hex trace ID or a W3C traceparent (55),
// and matches the 64-character columns that store correlation IDs.
const maxCorrelationIDLength = 64

// unexported key types prevent collisions with keys defined in other packages.
type correlationIDKeyType struct{}
type loggerKeyType struct{}

// CorrelationIDMiddleware reads the X-Correlation-ID request header (or
// generates a new UUID v4 if it is absent or invalid), stores it and a correlation-aware
// *slog.Logger in the request context, and echoes the ID in the response
// header.
//
// Register this as the outermost middleware so all subsequent handlers have
// access to a correlation-aware logger.
func CorrelationIDMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(CorrelationIDHeader)
			if !validCorrelationID(id) {
				id = uuid.New().String()
			}
			ctx := context.WithValue(r.Context(), correlationIDKeyType{}, id)
			ctx = context.WithValue(ctx, loggerKeyType{}, log.With(slog.String("correlation_id", id)))
			w.Header().Set(CorrelationIDHeader, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetCorrelationID returns the correlation ID stored by CorrelationIDMiddleware.
// Returns an empty string when the middleware has not run.
func GetCorrelationID(r *http.Request) string {
	return GetCorrelationIDFromContext(r.Context())
}

// GetCorrelationIDFromContext returns the correlation ID stored in ctx by
// CorrelationIDMiddleware. Returns an empty string when the middleware has not
// run or when ctx carries no correlation ID (e.g. context.Background()).
func GetCorrelationIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(correlationIDKeyType{}).(string)
	return id
}

// GetLogger returns the correlation-aware logger stored by
// CorrelationIDMiddleware. Falls back to fallback when the middleware has not
// run (e.g. in tests).
func GetLogger(r *http.Request, fallback *slog.Logger) *slog.Logger {
	if log, ok := r.Context().Value(loggerKeyType{}).(*slog.Logger); ok {
		return log
	}
	return fallback
}

// validCorrelationID accepts a caller-supplied ID only when it is non-empty, at most
// maxCorrelationIDLength bytes and printable ASCII without spaces.
// Allowed characters: A-Z, a-z, 0-9, and !"#$%&'()*+,-./:;<=>?@[\]^_`{|}~
func validCorrelationID(id string) bool {
	if id == "" || len(id) > maxCorrelationIDLength {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] <= ' ' || id[i] > '~' {
			return false
		}
	}
	return true
}

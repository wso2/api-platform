package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

// CorrelationIDHeader is the canonical header name for the correlation / trace ID.
const CorrelationIDHeader = "X-Correlation-ID"

// unexported key types prevent collisions with keys defined in other packages.
type correlationIDKeyType struct{}
type loggerKeyType struct{}

// CorrelationIDMiddleware reads the X-Correlation-ID request header (or
// generates a new UUID v4 if absent), stores it and a correlation-aware
// *slog.Logger in the request context, and echoes the ID in the response
// header.
//
// Register this as the outermost middleware so all subsequent handlers have
// access to a correlation-aware logger.
func CorrelationIDMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(CorrelationIDHeader)
			if id == "" {
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
	id, _ := r.Context().Value(correlationIDKeyType{}).(string)
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

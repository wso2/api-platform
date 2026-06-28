package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/wso2/go-httpkit/httputil"
)

// RecoveryMiddleware catches panics from downstream handlers, logs the stack
// trace, and writes a 500 JSON response so the server stays alive.
func RecoveryMiddleware(fallback *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					GetLogger(r, fallback).Error("panic recovered",
						slog.Any("panic", rec),
						slog.String("stack", string(debug.Stack())),
					)
					httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

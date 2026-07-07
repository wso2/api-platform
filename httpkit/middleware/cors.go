package middleware

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// CORSOptions configures CORS behaviour for CORSMiddleware and WithCORS.
type CORSOptions struct {
	// AllowedOrigins lists origins that may access the resource.
	// Use ["*"] to allow any origin (cannot be combined with AllowCredentials).
	AllowedOrigins []string
	// AllowedMethods lists the HTTP methods permitted for the resource.
	AllowedMethods []string
	// AllowedHeaders lists the request headers the browser may send.
	AllowedHeaders []string
	// AllowCredentials indicates whether cookies / auth headers are allowed.
	AllowCredentials bool
	// MaxAge is the number of seconds the preflight response may be cached.
	MaxAge int
}

// DefaultAllowedHeaders are the headers permitted by default when building
// CORSOptions.
var DefaultAllowedHeaders = []string{"Content-Type", "Authorization", "X-Correlation-ID"}

// CORSMiddleware applies CORS headers globally to every route handled by the
// wrapped handler.
func CORSMiddleware(opts CORSOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			applyCORSHeaders(w, r, opts)
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WithCORS wraps handler with per-route CORS and returns the (pattern,
// wrappedHandler) pair so it can be spread directly into mux.HandleFunc:
//
//	mux.HandleFunc(middleware.WithCORS("GET /users/{id}", handler, opts))
func WithCORS(pattern string, h http.HandlerFunc, opts CORSOptions) (string, http.HandlerFunc) {
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyCORSHeaders(w, r, opts)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r)
	})
	return pattern, wrapped
}

func applyCORSHeaders(w http.ResponseWriter, r *http.Request, opts CORSOptions) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}

	allowedOrigin := resolveOrigin(origin, opts.AllowedOrigins)
	if allowedOrigin == "" {
		return
	}

	w.Header().Add("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)

	// Never pair a wildcard origin with credentials: the combination is spec-invalid
	// (browsers reject it for credentialed requests) and, if a browser ever did honor it,
	// would let any site read authenticated responses. Only send the credentials header
	// when the origin was resolved to a specific, allowlisted value.
	if opts.AllowCredentials && allowedOrigin != "*" {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Preflight-only headers
	if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
		if len(opts.AllowedMethods) > 0 {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(opts.AllowedMethods, ", "))
		}
		if len(opts.AllowedHeaders) > 0 {
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(opts.AllowedHeaders, ", "))
		}
		if opts.MaxAge > 0 {
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(opts.MaxAge))
		}
	}
}

func resolveOrigin(origin string, allowed []string) string {
	if len(allowed) == 0 {
		return "*"
	}
	if slices.Contains(allowed, "*") {
		return "*"
	}
	if slices.Contains(allowed, origin) {
		return origin
	}
	return ""
}

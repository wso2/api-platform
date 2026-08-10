/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package server

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"api-control-plane-bff/internal/config"
	"api-control-plane-bff/internal/session"
)

// chain applies middlewares in order (outermost first).
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// securityHeaders sets global response headers. Strict-Transport-Security is
// only sent when the deployment expects HTTPS (Session.Cookie.Secure) — an
// unconditional HSTS header would pin a plain-HTTP local-dev origin to HTTPS
// for a year, breaking the very deployment that set Secure=false to allow it.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		if s.cfg.Session.Cookie.Secure {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "frame-ancestors 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Permitted-Cross-Domain-Policies", "none")
		next.ServeHTTP(w, r)
	})
}

// requestID always generates a fresh, server-side correlation id and exposes
// it on the response — never an inbound X-Request-Id, which is echoed
// verbatim into access logs and returned to the client as trackingId on
// server errors. Trusting a client-supplied value there would let a caller
// pick its own correlation key, collide with another request's, or inject
// arbitrary text into log records and error payloads.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Request-Id", hex.EncodeToString(b))
		next.ServeHTTP(w, r)
	})
}

// recoverPanic converts panics into a 500 without leaking stack traces.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "err", rec, "path", r.URL.Path)
				writeServerErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", w.Header().Get("X-Request-Id"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// requireCSRF rejects state-mutating requests lacking the custom header. CORS is
// closed, so cross-site attackers cannot set this header.
func (s *Server) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			// Safe methods: no CSRF token required.
		default:
			if r.Header.Get(config.CSRFHeaderName) == "" {
				writeErrorJSON(w, http.StatusForbidden, "MISSING_CSRF_HEADER", "missing CSRF header")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// sessionContext resolves the caller's session (if any) once per request and
// stashes it on the request context via session.WithContext, using the exact
// same cookie lookup + decode path handleSession itself uses. Runs for every
// route on this mux — including a host's Options.ExtraRoutes handlers — so
// any of them can read identity via session.FromContext the same way a
// default handler would, with no per-feature auth wiring. A request with no
// (or an invalid) session cookie simply proceeds with nothing stashed;
// FromContext's ok=false is how a handler distinguishes that case, and
// whether that's an error is up to the handler — this middleware never
// rejects a request itself (routes below decide their own auth requirement).
func (s *Server) sessionContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token, ok := s.tokenFromCookie(r); ok && !tokenExpired(token) {
			u := s.userFromToken(r.Context(), token)
			r = r.WithContext(session.WithContext(r.Context(), u))
		}
		next.ServeHTTP(w, r)
	})
}

// logRequests emits a structured access log line per request.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"dur_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Flush propagates flushes so streaming (SSE) works through the wrapper.
func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

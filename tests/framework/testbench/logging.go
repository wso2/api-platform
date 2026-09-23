/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package testbench

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// RequestIDHeader carries a correlation id in and out of every mock service. A caller that
// sets it has its own value echoed back and logged, so a suite step can tie an assertion to
// the exact request the testbench saw; otherwise one is generated here.
const RequestIDHeader = "X-Request-Id"

// maxLoggedBodyBytes bounds a body excerpt written to a log line. Enough to recognise a
// payload, far short of reproducing it.
const maxLoggedBodyBytes = 512

type requestInfoCtxKey struct{}

// requestInfo accumulates what is known about a request as it travels the middleware chain.
// The access log runs outermost, so it cannot see a context value that PartitionRouter adds
// further in; both share this holder instead.
type requestInfo struct {
	id        string
	partition string
	service   string
	// bodyBytes is the request body actually consumed, filled in by limitBody's drain.
	bodyBytes int64
	// truncated records that the body hit the configured ceiling.
	truncated bool
}

func requestInfoFrom(ctx context.Context) (*requestInfo, bool) {
	info, ok := ctx.Value(requestInfoCtxKey{}).(*requestInfo)
	return info, ok
}

// RequestIDFromContext returns the correlation id assigned to this request, for a service
// that wants to name it in its own log lines.
func RequestIDFromContext(ctx context.Context) string {
	if info, ok := requestInfoFrom(ctx); ok {
		return info.id
	}
	return ""
}

// ServiceLogger returns a logger already tagged with this request's service, correlation id,
// and partition, so a handler logs consistently without restating them.
func ServiceLogger(ctx context.Context) *slog.Logger {
	log := slog.Default()
	info, ok := requestInfoFrom(ctx)
	if !ok {
		return log
	}
	attrs := []any{}
	if info.service != "" {
		attrs = append(attrs, "service", info.service)
	}
	if info.id != "" {
		attrs = append(attrs, "request_id", info.id)
	}
	if info.partition != "" {
		attrs = append(attrs, "partition", info.partition)
	}
	if len(attrs) == 0 {
		return log
	}
	return log.With(attrs...)
}

// newRequestID produces a short random correlation id. Randomness only has to distinguish
// concurrent requests within one run, so 8 bytes is ample and cheap.
func newRequestID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// A correlation id is a diagnostic; losing entropy must never fail a request.
		return "unknown"
	}
	return hex.EncodeToString(buf[:])
}

// responseRecorder observes what a handler wrote so the access log can report the status and
// size the client actually received, rather than what the handler intended.
type responseRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
	// failure retains the start of an error response body. Every mock writes its reason into
	// the body it returns, so capturing that is what puts the *why* of a 4xx/5xx into the log
	// without each of the thirteen services having to log for itself.
	failure []byte
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		// A duplicate WriteHeader is a handler bug that net/http only warns about; name it.
		slog.Warn("testbench handler wrote the response header twice",
			"first_status", r.status, "ignored_status", status)
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		// net/http implies 200 on the first Write; record the same so the log agrees.
		r.status = http.StatusOK
		r.wroteHeader = true
	}
	if r.status >= http.StatusBadRequest && len(r.failure) < maxLoggedBodyBytes {
		room := maxLoggedBodyBytes - len(r.failure)
		if room > len(b) {
			room = len(b)
		}
		r.failure = append(r.failure, b[:room]...)
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	if err != nil {
		// A write failure means the client is already gone or the connection broke mid-response;
		// the handler's own return value is usually discarded, so this is the only record.
		slog.Warn("testbench response write failed",
			"status", r.status, "written_bytes", r.bytes, "error", err)
	}
	return n, err
}

// Flush forwards to the underlying writer when it supports streaming, which the MCP and
// analytics services rely on; without it, wrapping would silently disable flushing.
func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets net/http and httptest reach the original writer, keeping
// http.ResponseController working through this wrapper.
func (r *responseRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// observability assigns a correlation id, records the request, recovers a panic, and emits
// one access line per request. It wraps every service uniformly so no mock has to implement
// its own logging, and so a request that never reaches a handler is still accounted for.
func observability(service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()

		id := strings.TrimSpace(r.Header.Get(RequestIDHeader))
		if id == "" {
			id = newRequestID()
		}
		info := &requestInfo{id: id, service: service}
		r = r.WithContext(context.WithValue(r.Context(), requestInfoCtxKey{}, info))
		w.Header().Set(RequestIDHeader, id)

		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

		log := slog.With(
			"service", service,
			"request_id", id,
			"method", r.Method,
			"path", r.URL.Path,
		)
		log.Debug("testbench request received",
			"has_query", r.URL.RawQuery != "",
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
			"content_type", r.Header.Get("Content-Type"),
			"content_length", r.ContentLength,
			"host", r.Host,
			"proto", r.Proto,
		)

		emitAccess := func() {
			attrs := []any{
				"status", rec.status,
				"duration_ms", time.Since(started).Milliseconds(),
				"response_bytes", rec.bytes,
				"request_bytes", info.bodyBytes,
			}
			if info.partition != "" {
				attrs = append(attrs, "partition", info.partition)
			}
			if info.truncated {
				attrs = append(attrs, "request_body_truncated", true)
			}
			attrs = append(attrs, "has_query", r.URL.RawQuery != "")
			if reason := safeFailureExcerpt(rec); reason != "" {
				attrs = append(attrs, "reason", reason)
			}

			// Severity follows the response: a mock returning 5xx is a fault worth surfacing at
			// error level even though the mock itself is behaving as programmed.
			switch {
			case rec.status >= http.StatusInternalServerError:
				log.Error("testbench request failed", attrs...)
			case rec.status >= http.StatusBadRequest:
				log.Warn("testbench request rejected", attrs...)
			default:
				log.Info("testbench request served", attrs...)
			}
		}

		defer func() {
			if recovered := recover(); recovered != nil {
				// A panic before the response starts must not take the shared testbench down with
				// it: 13 services share this process, and every concurrent block depends on them.
				log.Error("testbench handler panicked",
					"panic", recovered,
					"stack", string(debug.Stack()),
					"duration_ms", time.Since(started).Milliseconds(),
				)
				if rec.wroteHeader || rec.bytes > 0 {
					// Once a response has started, its status and bytes cannot be withdrawn. Let
					// net/http abort the connection instead of appending panic text to a 200 body.
					emitAccess()
					panic(recovered)
				}
				http.Error(rec, "testbench handler panicked", http.StatusInternalServerError)
				emitAccess()
			}
		}()

		next.ServeHTTP(rec, r)
		emitAccess()
	})
}

// excerpt renders a bounded, single-line view of a payload for a log field.
func excerpt(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	s := string(b)
	if len(s) > maxLoggedBodyBytes {
		s = s[:maxLoggedBodyBytes] + "…(truncated)"
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", "")
}

// safeFailureExcerpt keeps useful plain-text failure messages while suppressing response
// bodies that may reflect request headers, query parameters, or credentials. The testbench's
// echo and backend services intentionally return JSON reflections, so there is no reliable
// generic sanitizer for those bodies.
func safeFailureExcerpt(r *responseRecorder) string {
	if r == nil || strings.HasPrefix(strings.ToLower(r.Header().Get("Content-Type")), "application/json") {
		return ""
	}
	return excerpt(r.failure)
}

// Fail writes an error response and logs why, with this request's service, correlation id,
// and partition already attached. Prefer it over http.Error in a mock service: the access
// line records the status either way, but only the caller knows the structured detail behind
// it, and that detail is what turns a 400 in a CI log into an actionable one.
func Fail(w http.ResponseWriter, r *http.Request, status int, message string, attrs ...any) {
	all := append([]any{"status", status, "message", message}, attrs...)
	log := ServiceLogger(r.Context())
	if status >= http.StatusInternalServerError {
		log.Error("testbench service returned a server error", all...)
	} else {
		log.Warn("testbench service rejected a request", all...)
	}
	http.Error(w, message, status)
}

// WriteJSON encodes value as the response body, logging an encode or write failure that a
// handler would otherwise discard. A mock that half-writes a body produces a client-side
// parse error with no server-side trace, which is the hardest shape of failure to chase.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		ServiceLogger(r.Context()).Error("testbench could not encode a response",
			"status", status, "error", err, "value_type", fmt.Sprintf("%T", value))
		http.Error(w, "testbench response encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		ServiceLogger(r.Context()).Warn("testbench could not write a response",
			"status", status, "body_bytes", len(body), "error", err)
	}
}

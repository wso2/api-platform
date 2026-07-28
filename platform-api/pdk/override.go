/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package pdk

import (
	"bytes"
	"net/http"
)

// RouteOverride declares that a plugin decorates one existing core route.
//
// Pattern must be an EXISTING core route pattern, matched as an exact string
// against the patterns core registered (for example
// "GET /api/v0.9/gateways/{gatewayId}"). A pattern core does not register — a
// typo, or a version that has moved on — aborts startup rather than silently
// doing nothing. Overriding a plugin's route, the webhook receiver, or the
// health endpoint is not supported; only core routes are recorded.
//
// Wrap receives the ORIGINAL core handler as next and returns the handler that
// is registered under the same pattern on the real mux. Because the pattern is
// unchanged, path wildcards are resolved by the mux before Wrap runs and
// r.PathValue still works inside the core handler.
//
// This is an auth-sensitive surface:
//   - The route's required scopes are unchanged by an override. The scope
//     registry is keyed by OpenAPI path/method, so the original requirement
//     stays in force; a plugin that needs different scopes declares them in its
//     own OpenAPISpec, and must re-declare the core scopes it does not intend to
//     drop (GO-AUTH-007).
//   - A decorator must scope tenant data by the organization in the request
//     context, never by anything in the request body or query (GO-AUTH-005).
//   - Rewriting r.URL.Path inside Wrap does not re-route the request; the
//     handler for this pattern is already selected. Do not use an override to
//     redirect traffic to a different endpoint (GO-AUTH-017).
type RouteOverride struct {
	// Pattern is an existing core route pattern, matched exactly.
	Pattern string
	// Wrap decorates the original core handler. It must not be nil.
	Wrap func(next http.Handler) http.Handler
}

// RouteOverrideProvider is an OPTIONAL interface a Plugin may implement to
// decorate existing core routes. Return an empty slice to decorate none.
//
// Every returned override is validated at startup: a nil Wrap, an empty
// Pattern, a pattern claimed by another plugin, or a pattern core does not
// register aborts startup with an error naming the plugin and the pattern.
type RouteOverrideProvider interface {
	RouteOverrides() []RouteOverride
}

// CapturedResponse is what a core handler wrote, captured in memory by Invoke
// so a decorator can inspect or reshape it before anything reaches the client.
type CapturedResponse struct {
	// Status is the status code the handler wrote, defaulting to 200 when it
	// wrote a body without calling WriteHeader.
	Status int
	// Header holds the headers the handler set.
	Header http.Header
	// Body is everything the handler wrote, or nil if it wrote nothing.
	Body []byte
}

// captureWriter buffers a handler's response instead of sending it.
//
// It deliberately implements neither http.Flusher nor http.Hijacker: a
// streaming or WebSocket handler then fails visibly under an override rather
// than silently buffering the whole stream. Streaming routes cannot be
// overridden.
type captureWriter struct {
	header      http.Header
	body        bytes.Buffer
	status      int
	wroteHeader bool
}

func (c *captureWriter) Header() http.Header { return c.header }

func (c *captureWriter) WriteHeader(status int) {
	// A second WriteHeader is ignored, matching net/http.
	if c.wroteHeader {
		return
	}
	c.status = status
	c.wroteHeader = true
}

func (c *captureWriter) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		// Writing a body without a status implies 200, matching net/http.
		c.WriteHeader(http.StatusOK)
	}
	return c.body.Write(b)
}

// Invoke runs next with r and returns what it wrote, without sending anything to
// the client. Use it when a decorator needs to read or reshape the core
// response; a decorator that only observes the outcome should wrap the real
// http.ResponseWriter instead, which costs nothing.
//
// The captured response is held entirely in memory, so an override on an
// endpoint that returns a large payload buffers all of it. Only explicitly
// overridden routes pay this, and the operator chooses those routes.
//
// The writer handed to next implements neither http.Flusher nor http.Hijacker,
// so a streaming or hijacking handler fails visibly instead of stalling.
func Invoke(next http.Handler, r *http.Request) *CapturedResponse {
	cw := &captureWriter{header: make(http.Header), status: http.StatusOK}
	next.ServeHTTP(cw, r)

	res := &CapturedResponse{Status: cw.status, Header: cw.header}
	if cw.body.Len() > 0 {
		res.Body = cw.body.Bytes()
	}
	return res
}

// WriteCaptured sends a captured response to the client unchanged. Use it to
// pass an error response through untouched rather than re-encoding it, which
// would flatten core's error DTO.
//
// The captured Content-Length is dropped and left for net/http to recompute: a
// decorator that rewrote the body would otherwise emit a stale length.
func WriteCaptured(w http.ResponseWriter, res *CapturedResponse) {
	if res == nil {
		return
	}
	dst := w.Header()
	for k, vs := range res.Header {
		if http.CanonicalHeaderKey(k) == "Content-Length" {
			continue
		}
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
	status := res.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if len(res.Body) > 0 {
		_, _ = w.Write(res.Body)
	}
}

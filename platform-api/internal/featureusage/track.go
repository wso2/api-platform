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

package featureusage

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MiddlewareConfig configures Middleware.
type MiddlewareConfig struct {
	// Reporter returns the reporter, looked up on every call so the
	// middleware can be installed before the reporter exists. nil or Noop
	// leaves every request untouched.
	Reporter func() Reporter
	// Index holds the bound routes. A nil index binds nothing.
	Index *RouteIndex
	// Routes resolves the route pattern a request will match: the mux the
	// middleware wraps.
	Routes RouteResolver
	// Identity returns the platform organization UUID the request acts in
	// and the acting user's identity-provider subject, both from the
	// authenticated request context (never from request input). A request
	// with no organization is not reported: it cannot be attributed.
	Identity func(r *http.Request) (orgUUID, actorSubject string)
}

// Middleware reports every call to a bound route (see bindings.go) as one
// occurrence of its feature: successful or failed, with the status the caller
// received and the dimensions its binding reads from the request.
//
// Install it innermost, directly around the mux, after authentication and
// organization resolution: it reads the organization from the request
// context, and it reads forms from the same *http.Request the handler parsed.
// A route a plugin decorates with a copied request still reports the call, but
// without the dimensions read from its form.
//
// Nothing about the request changes for the handler, except that for a
// binding that reads the JSON body, the body is read through a tee that keeps
// up to maxCapturedBody bytes of what the handler reads.
func Middleware(cfg MiddlewareConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if cfg.Index.Len() == 0 || cfg.Reporter == nil || cfg.Routes == nil || cfg.Identity == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rep := cfg.Reporter()
			if _, off := rep.(noop); rep == nil || off {
				next.ServeHTTP(w, r)
				return
			}
			route := cfg.Index.lookup(cfg.Routes, r)
			if route == nil {
				next.ServeHTTP(w, r)
				return
			}
			orgUUID, actor := cfg.Identity(r)
			if orgUUID == "" {
				next.ServeHTTP(w, r)
				return
			}

			started := time.Now()
			var body *capturedBody
			if route.captureJSON && r.Body != nil && r.Body != http.NoBody {
				body = &capturedBody{ReadCloser: r.Body}
				r.Body = body
			}
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)

			status := sw.status
			if status == 0 {
				status = http.StatusOK // nothing written: net/http sends 200
			}
			rep.Record(Occurrence{
				Feature:      route.feature.Code,
				OrgUUID:      orgUUID,
				ActorSubject: actor,
				// Every call is its own occurrence; the ID only lets a
				// redelivered event be deduplicated.
				OccurrenceID: uuid.NewString(),
				Invocation: Invocation{
					Method:      r.Method,
					URI:         reportedOrigin + route.path,
					Status:      status,
					RequestTime: started,
				},
				Dimensions: route.dimensions(viewOf(r, route.path, body)),
			})
		})
	}
}

// dimensions resolves the binding's dimensions for a served request. A value
// the taxonomy does not accept is left out; a required dimension left out
// makes the reporter drop the event, with the dimension named in the log.
func (br *boundRoute) dimensions(v *requestView) map[string]any {
	dims := make(map[string]any, len(br.binding.Set)+len(br.binding.From))
	for name, value := range br.binding.Set {
		dims[name] = value
	}
	for name, src := range br.binding.From {
		value, ok := src.resolve(v)
		if ok && br.feature.Dimensions[name].accepts(value) {
			dims[name] = value
		}
	}
	return dims
}

// viewOf collects what a source may read from a served request: its path
// parameters, the form the handler parsed, if any, and the JSON body it read.
func viewOf(r *http.Request, routePath string, body *capturedBody) *requestView {
	v := &requestView{pathValues: pathParamsFor(routePath, r.URL.EscapedPath())}
	switch {
	case r.MultipartForm != nil:
		v.form = &formView{values: r.MultipartForm.Value, files: r.MultipartForm.File}
	case r.PostForm != nil:
		v.form = &formView{values: r.PostForm}
	}
	if body != nil {
		v.body = body.captured()
	}
	return v
}

// maxCapturedBody is how much of a JSON request body is kept for a binding
// that reads it. The JSON bodies bound today are small objects; a larger body
// leaves its dimensions unknown rather than being buffered.
const maxCapturedBody = 64 << 10

// capturedBody keeps a copy of what the handler reads from the request body,
// up to maxCapturedBody bytes.
type capturedBody struct {
	io.ReadCloser
	buf      []byte
	overflow bool
}

func (c *capturedBody) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	if n > 0 && !c.overflow {
		if len(c.buf)+n > maxCapturedBody {
			c.overflow, c.buf = true, nil
		} else {
			c.buf = append(c.buf, p[:n]...)
		}
	}
	return n, err
}

// captured returns what was read, or nil when nothing was or it overflowed.
func (c *capturedBody) captured() []byte {
	if c.overflow || len(c.buf) == 0 {
		return nil
	}
	return c.buf
}

// statusWriter remembers the final status a handler sent.
type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader records the first final status. An informational 1xx response
// (e.g. 103 Early Hints) is passed on but not recorded: the handler sends its
// real answer after it. 101 Switching Protocols is final.
func (s *statusWriter) WriteHeader(code int) {
	informational := code >= 100 && code < 200 && code != http.StatusSwitchingProtocols
	if s.status == 0 && !informational {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK // net/http's implicit status
	}
	return s.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// RecordOperation reports work that did the job of the bound REST API
// operation op without going through its route, such as an MCP tool call. The
// event names the endpoint actually invoked: method and route, the route as a
// template path (for an MCP tool, POST /api/v0.9/mcp). status is the HTTP
// status the same work answers with on op's route, so a failure is reported
// as one even when the endpoint itself answered 200. Only the binding's Set
// dimensions apply, since there is no REST request to read.
//
// Nothing is reported when rep is nil or off, op is not bound, route is not a
// template path, or there is no organization.
func RecordOperation(rep Reporter, op, orgUUID, actorSubject, method, route string, status int, started time.Time) {
	if _, off := rep.(noop); rep == nil || off || orgUUID == "" || !strings.HasPrefix(route, "/") {
		return
	}
	b, ok := bindingByOp[op]
	if !ok {
		return
	}
	dims := make(map[string]any, len(b.Set))
	for name, value := range b.Set {
		dims[name] = value
	}
	rep.Record(Occurrence{
		Feature:      b.Feature,
		OrgUUID:      orgUUID,
		ActorSubject: actorSubject,
		OccurrenceID: uuid.NewString(),
		Invocation:   Invocation{Method: method, URI: reportedOrigin + route, Status: status, RequestTime: started},
		Dimensions:   dims,
	})
}

// bindingByOp indexes bindings by operation.
var bindingByOp = func() map[string]Binding {
	out := make(map[string]Binding, len(bindings))
	for _, b := range bindings {
		out[b.Op] = b
	}
	return out
}()

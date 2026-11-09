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
	"context"
	"maps"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Call collects what a handler learns about one tracked REST API call: whose
// call it is, what it created, and the feature's dimensions. The handler
// records facts as it learns them, and Track reports the call once the
// response is written — with the status actually sent, so a failed call is
// reported as a failure.
//
// Every method is safe on a nil *Call, which is what CallFrom returns for an
// untracked request, so a handler records facts without checking.
type Call struct {
	mu           sync.Mutex
	orgUUID      string
	actorSubject string
	resourceUUID string
	dimensions   map[string]any
}

type callKey struct{}

// CallFrom returns the call Track is tracking for ctx, or nil.
func CallFrom(ctx context.Context) *Call {
	c, _ := ctx.Value(callKey{}).(*Call)
	return c
}

// SetOrganization records the platform organization UUID the call acts in.
// A call with no organization is not reported: it cannot be attributed.
func (c *Call) SetOrganization(orgUUID string) {
	c.set(func() { c.orgUUID = orgUUID })
}

// SetActor records the acting user's identity-provider subject, as the
// request authenticated it. It is read from the request, not looked up, so
// recording it never delays the response.
func (c *Call) SetActor(subject string) {
	c.set(func() { c.actorSubject = subject })
}

// SetResource records the UUID of what a successful call created. It seeds
// the event's deduplication key; it is never sent.
func (c *Call) SetResource(resourceUUID string) {
	c.set(func() { c.resourceUUID = resourceUUID })
}

// SetDimension records one taxonomy dimension. The taxonomy check happens
// when the call is reported, so an undeclared name or value drops the event.
func (c *Call) SetDimension(name string, value any) {
	c.set(func() { c.dimensions[name] = value })
}

func (c *Call) set(apply func()) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	apply()
}

// occurrence builds the occurrence for the finished call, or reports false
// when the call never reached an organization.
func (c *Call) occurrence(feature string, inv Invocation) (Occurrence, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.orgUUID == "" {
		return Occurrence{}, false
	}
	// A created resource identifies a successful call for good; a failed call
	// has no such identity, so it gets its own random one, which keeps two
	// distinct failures from being deduplicated into one.
	occurrenceID := c.resourceUUID
	if occurrenceID == "" || inv.Status < 200 || inv.Status > 299 {
		occurrenceID = uuid.NewString()
	}
	return Occurrence{
		Feature:      feature,
		OrgUUID:      c.orgUUID,
		ActorSubject: c.actorSubject,
		OccurrenceID: occurrenceID,
		Invocation:   inv,
		Dimensions:   maps.Clone(c.dimensions),
	}, true
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

// Track wraps the handler of a REST API route so that every call it serves is
// reported as one event of feature — success or failure, with the status the
// call answered with. Each call starts with the defaults as its dimensions;
// next adds what it learns through CallFrom(r.Context()).
//
// reporter is looked up on every call, so the route can be registered before
// the reporter exists. When it is nil or reporting is off, next is served
// untouched. Register Track outside any error-mapping middleware, so the
// status it sees is the one the caller received.
func Track(reporter func() Reporter, feature string, defaults map[string]any, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rep := reporter()
		if _, off := rep.(noop); rep == nil || off {
			next.ServeHTTP(w, r)
			return
		}
		started := time.Now()
		call := &Call{dimensions: maps.Clone(defaults)}
		if call.dimensions == nil {
			call.dimensions = map[string]any{}
		}
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r.WithContext(context.WithValue(r.Context(), callKey{}, call)))

		status := sw.status
		if status == 0 {
			status = http.StatusOK // nothing written: net/http sends 200
		}
		if o, ok := call.occurrence(feature, InvocationFromRequest(r, status, started)); ok {
			rep.Record(o)
		}
	})
}

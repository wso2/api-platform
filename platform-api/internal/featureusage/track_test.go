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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// capture is a Reporter that keeps what it is given.
type capture struct {
	mu  sync.Mutex
	got []Occurrence
}

func (c *capture) Record(o Occurrence) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, o)
}

func (c *capture) Close(context.Context) error { return nil }

func (c *capture) all() []Occurrence {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Occurrence(nil), c.got...)
}

// only returns the single recorded occurrence, failing otherwise.
func (c *capture) only(t *testing.T) Occurrence {
	t.Helper()
	got := c.all()
	if len(got) != 1 {
		t.Fatalf("recorded %d occurrences, want 1: %+v", len(got), got)
	}
	return got[0]
}

// testSpec declares the operations the middleware tests bind.
const testSpec = `openapi: 3.0.3
servers:
  - url: https://localhost:9243/api/v0.9
paths:
  /rest-apis:
    parameters:
      - name: ignored
        in: query
    get:
      operationId: ListRESTAPIs
    post:
      operationId: CreateRESTAPI
  /rest-apis/validate-openapi:
    post:
      operationId: ValidateOpenAPISpec
  /graphql-apis:
    post:
      operationId: CreateGraphQLAPI
  /apis/{apiType}/{apiId}/docs:
    post:
      operationId: CreateAPIDocument
  /gateways:
    post:
      operationId: CreateGateway
`

// testBindings bind the operations of testSpec to real taxonomy features, one
// per kind of source.
var testBindings = []Binding{
	{Op: "CreateRESTAPI", Feature: featureRESTAPICreate},
	{Op: "ValidateOpenAPISpec", Feature: "apip.manage-apis.api-contract-validate",
		Set: Dims{"contract_type": "openapi"}, From: Sources{"contract_source": Present("file", "url")}},
	{Op: "CreateGraphQLAPI", Feature: "apip.manage-apis.graphql-api-create", From: Sources{
		"schema_source": FormJSON("metadata", "/schemaSource").Default("introspection"),
	}},
	{Op: "CreateAPIDocument", Feature: "apip.manage-apis.api-document-create", From: Sources{
		"api_type":      Path("apiType").Map(apiTypes),
		"document_type": Form("type").Map(documentTypes),
	}},
	{Op: "CreateGateway", Feature: "apip.gateway.gateway-register", From: Sources{
		"gateway_type": JSON("/functionalityType").Default("regular"),
		"is_critical":  JSON("/isCritical").Default(false),
	}},
}

const (
	orgHeader   = "X-Test-Org"
	actorHeader = "X-Test-Actor"
)

// testServer is a mux of handlers that parse their requests the way Platform
// API's handlers do, wrapped in the middleware.
type testServer struct {
	rep     *capture
	handler http.Handler
	// lastBody is the body the last handler read, to show the tee is invisible.
	lastBody []byte
}

func newTestServer(t *testing.T, rep Reporter) *testServer {
	t.Helper()
	s := &testServer{}
	if c, ok := rep.(*capture); ok {
		s.rep = c
	}
	status := func(r *http.Request, ok int) int {
		if n, err := strconv.Atoi(r.URL.Query().Get("status")); err == nil {
			return n
		}
		return ok
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0.9/rest-apis", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("POST /api/v0.9/rest-apis", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status(r, http.StatusCreated))
	})
	// Parses its form, like the import, validate and document handlers.
	parseForm := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("early") == "1" { // rejected before the form is read
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(status(r, http.StatusCreated))
	}
	mux.HandleFunc("POST /api/v0.9/rest-apis/validate-openapi", parseForm)
	mux.HandleFunc("POST /api/v0.9/graphql-apis", parseForm)
	mux.HandleFunc("POST /api/v0.9/apis/{apiType}/{apiId}/docs", parseForm)
	// Decodes a JSON body, like the gateway handler.
	mux.HandleFunc("POST /api/v0.9/gateways", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("early") == "1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.lastBody, _ = io.ReadAll(r.Body)
		if !json.Valid(s.lastBody) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(status(r, http.StatusCreated))
	})

	tax := mustTaxonomy(t)
	index, err := newRouteIndex([]byte(testSpec), mux, tax, testBindings, nil)
	if err != nil {
		t.Fatalf("route index: %v", err)
	}
	if index.Len() != len(testBindings) {
		t.Fatalf("route index binds %d routes, want %d", index.Len(), len(testBindings))
	}
	s.handler = Middleware(MiddlewareConfig{
		Reporter: func() Reporter { return rep },
		Index:    index,
		Routes:   mux,
		Identity: func(r *http.Request) (string, string) { return r.Header.Get(orgHeader), r.Header.Get(actorHeader) },
	})(mux)
	return s
}

func (s *testServer) send(t *testing.T, method, target, contentType string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set(orgHeader, testOrgUUID)
	req.Header.Set(actorHeader, testUserUUID)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func multipartBody(t *testing.T, fields map[string]string, files ...string) ([]byte, string) {
	t.Helper()
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for name, value := range fields {
		_ = w.WriteField(name, value)
	}
	for _, name := range files {
		part, _ := w.CreateFormFile(name, "spec.yaml")
		_, _ = part.Write([]byte("openapi: 3.0.0"))
	}
	_ = w.Close()
	return buf.Bytes(), w.FormDataContentType()
}

func TestMiddlewareReportsASuccessfulCall(t *testing.T) {
	rep := &capture{}
	s := newTestServer(t, rep)
	before := time.Now()
	if rec := s.send(t, http.MethodPost, "/api/v0.9/rest-apis?owner=alice", "application/json", []byte(`{}`)); rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want the handler's 201", rec.Code)
	}

	o := rep.only(t)
	if o.Feature != featureRESTAPICreate || o.OrgUUID != testOrgUUID || o.ActorSubject != testUserUUID || !canonicalUUID(o.OccurrenceID) {
		t.Errorf("unexpected occurrence: %+v", o)
	}
	if len(o.Dimensions) != 0 {
		t.Errorf("dimensions = %v, want none for a feature without any", o.Dimensions)
	}
	// The route template, never the query or the request's own host.
	inv := o.Invocation
	if inv.Method != http.MethodPost || inv.Status != http.StatusCreated ||
		inv.URI != "https://platform-api/api/v0.9/rest-apis" || inv.RequestTime.Before(before) {
		t.Errorf("invocation = %+v", inv)
	}
	event, err := validate(mustTaxonomy(t), enabledFeatures, o, time.Now())
	if err != nil {
		t.Fatalf("a reported call failed validation: %v", err)
	}
	if event.metadata["outcome"] != "success" {
		t.Errorf("outcome = %v, want success", event.metadata["outcome"])
	}
}

func TestMiddlewareReportsAFailedCallWithItsStatus(t *testing.T) {
	rep := &capture{}
	s := newTestServer(t, rep)
	s.send(t, http.MethodPost, "/api/v0.9/rest-apis?status=409", "", nil)
	s.send(t, http.MethodPost, "/api/v0.9/rest-apis?status=409", "", nil)
	got := rep.all()
	if len(got) != 2 || got[0].Invocation.Status != http.StatusConflict {
		t.Fatalf("got %+v, want two events with status 409", got)
	}
	if got[0].OccurrenceID == got[1].OccurrenceID {
		t.Error("two calls must be two events with distinct IDs")
	}
}

// An informational response (103 Early Hints) is not the call's status.
func TestMiddlewareRecordsTheFinalStatus(t *testing.T) {
	rep := &capture{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v0.9/rest-apis", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("POST /api/v0.9/rest-apis/validate-openapi", func(w http.ResponseWriter, r *http.Request) {}) // nothing written: 200
	index, err := newRouteIndex([]byte(testSpec), mux, mustTaxonomy(t), testBindings, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := Middleware(MiddlewareConfig{Reporter: func() Reporter { return rep }, Index: index, Routes: mux,
		Identity: func(*http.Request) (string, string) { return testOrgUUID, "" }})(mux)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v0.9/rest-apis", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v0.9/rest-apis/validate-openapi", nil))
	got := rep.all()
	if len(got) != 2 || got[0].Invocation.Status != http.StatusCreated || got[1].Invocation.Status != http.StatusOK {
		t.Fatalf("got %+v, want 201 then the implicit 200", got)
	}
}

// A call that never reached an organization cannot be attributed.
func TestMiddlewareSkipsACallWithoutAnOrganization(t *testing.T) {
	rep := &capture{}
	s := newTestServer(t, rep)
	req := httptest.NewRequest(http.MethodPost, "/api/v0.9/rest-apis", nil)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want the handler's 201", rec.Code)
	}
	if n := len(rep.all()); n != 0 {
		t.Fatalf("recorded %d occurrences for an unattributable call", n)
	}
}

func TestMiddlewareIgnoresAnUnboundRoute(t *testing.T) {
	rep := &capture{}
	s := newTestServer(t, rep)
	s.send(t, http.MethodGet, "/api/v0.9/rest-apis", "", nil)
	s.send(t, http.MethodPost, "/api/v0.9/no-such-route", "", nil)
	if n := len(rep.all()); n != 0 {
		t.Fatalf("recorded %d occurrences for unbound routes", n)
	}
}

// With reporting off the request reaches the handler untouched: no tee on its
// body.
func TestMiddlewarePassesThroughWhenReportingIsOff(t *testing.T) {
	for name, rep := range map[string]Reporter{"no reporter": nil, "reporting off": Noop()} {
		t.Run(name, func(t *testing.T) {
			mux := http.NewServeMux()
			var teed bool
			mux.HandleFunc("POST /api/v0.9/gateways", func(w http.ResponseWriter, r *http.Request) {
				_, teed = r.Body.(*capturedBody)
				w.WriteHeader(http.StatusCreated)
			})
			index, err := newRouteIndex([]byte(testSpec), mux, mustTaxonomy(t), testBindings, nil)
			if err != nil {
				t.Fatal(err)
			}
			h := Middleware(MiddlewareConfig{Reporter: func() Reporter { return rep }, Index: index, Routes: mux,
				Identity: func(*http.Request) (string, string) { return testOrgUUID, "" }})(mux)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v0.9/gateways", strings.NewReader(`{}`)))
			if rec.Code != http.StatusCreated || teed {
				t.Errorf("status = %d, teed = %v; want the handler's 201 on the original body", rec.Code, teed)
			}
		})
	}
}

func TestMiddlewareReadsPathAndFormDimensions(t *testing.T) {
	tests := map[string]struct {
		target string
		fields map[string]string
		want   map[string]any
	}{
		"well-known type": {"/api/v0.9/apis/rest-api/orders/docs", map[string]string{"type": "HowTo"},
			map[string]any{"api_type": "rest-api", "document_type": "how-to"}},
		"any case, Go form of the type": {"/api/v0.9/apis/GraphQLApi/orders/docs", map[string]string{"type": "publicforum"},
			map[string]any{"api_type": "graphql-api", "document_type": "public-forum"}},
		// An unknown type is free text: left out, never sent as typed.
		"unknown type": {"/api/v0.9/apis/rest-api/orders/docs", map[string]string{"type": "Runbook by alice"},
			map[string]any{"api_type": "rest-api"}},
		"no type": {"/api/v0.9/apis/rest-api/orders/docs", map[string]string{"displayName": "x"},
			map[string]any{"api_type": "rest-api"}},
		// mcp-proxy is not in the feature's api_type enum, so it is left out
		// rather than sent; the reporter then drops the event.
		"type outside the enum": {"/api/v0.9/apis/mcp-proxy/orders/docs", map[string]string{"type": "HowTo"},
			map[string]any{"document_type": "how-to"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rep := &capture{}
			s := newTestServer(t, rep)
			body, ct := multipartBody(t, tc.fields)
			s.send(t, http.MethodPost, tc.target, ct, body)
			o := rep.only(t)
			if !sameDims(o.Dimensions, tc.want) {
				t.Errorf("dimensions = %v, want %v", o.Dimensions, tc.want)
			}
			// The URI is the template: no API type or handle from the path.
			if o.Invocation.URI != "https://platform-api/api/v0.9/apis/{apiType}/{apiId}/docs" {
				t.Errorf("URI = %q", o.Invocation.URI)
			}
		})
	}
}

func TestMiddlewareReadsPresenceNotContent(t *testing.T) {
	tests := map[string]struct {
		fields map[string]string
		files  []string
		want   map[string]any
	}{
		"file":    {nil, []string{"file"}, map[string]any{"contract_type": "openapi", "contract_source": "file"}},
		"url":     {map[string]string{"url": "https://internal.example.com/spec"}, nil, map[string]any{"contract_type": "openapi", "contract_source": "url"}},
		"neither": {map[string]string{"other": "x"}, nil, map[string]any{"contract_type": "openapi"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rep := &capture{}
			s := newTestServer(t, rep)
			body, ct := multipartBody(t, tc.fields, tc.files...)
			s.send(t, http.MethodPost, "/api/v0.9/rest-apis/validate-openapi", ct, body)
			if o := rep.only(t); !sameDims(o.Dimensions, tc.want) {
				t.Errorf("dimensions = %v, want %v", o.Dimensions, tc.want)
			}
		})
	}
}

func TestMiddlewareReadsJSONInAFormField(t *testing.T) {
	tests := map[string]struct {
		target string
		fields map[string]string
		want   map[string]any
	}{
		"declared":         {"/api/v0.9/graphql-apis", map[string]string{"metadata": `{"schemaSource":"url","sdlUrl":"https://x"}`}, map[string]any{"schema_source": "url"}},
		"omitted defaults": {"/api/v0.9/graphql-apis", map[string]string{"metadata": `{"displayName":"x"}`}, map[string]any{"schema_source": "introspection"}},
		"not an enum value": {"/api/v0.9/graphql-apis", map[string]string{"metadata": `{"schemaSource":"carrier pigeon"}`},
			map[string]any{}},
		"malformed metadata": {"/api/v0.9/graphql-apis", map[string]string{"metadata": `{"schemaSource":`}, map[string]any{}},
		// The form was never parsed, so the API's default is not assumed.
		"rejected before the form": {"/api/v0.9/graphql-apis?early=1", map[string]string{"metadata": `{}`}, map[string]any{}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rep := &capture{}
			s := newTestServer(t, rep)
			body, ct := multipartBody(t, tc.fields)
			s.send(t, http.MethodPost, tc.target, ct, body)
			if o := rep.only(t); !sameDims(o.Dimensions, tc.want) {
				t.Errorf("dimensions = %v, want %v", o.Dimensions, tc.want)
			}
		})
	}
}

func TestMiddlewareReadsTheJSONBodyTheHandlerRead(t *testing.T) {
	tests := map[string]struct {
		target string
		body   string
		want   map[string]any
	}{
		"declared":         {"/api/v0.9/gateways", `{"name":"gw","functionalityType":"ai","isCritical":true}`, map[string]any{"gateway_type": "ai", "is_critical": true}},
		"omitted defaults": {"/api/v0.9/gateways", `{"name":"gw"}`, map[string]any{"gateway_type": "regular", "is_critical": false}},
		"wrong types":      {"/api/v0.9/gateways", `{"functionalityType":7,"isCritical":"yes"}`, map[string]any{}},
		"malformed":        {"/api/v0.9/gateways", `{"functionalityType":"ai"`, map[string]any{}},
		"never read":       {"/api/v0.9/gateways?early=1", `{"functionalityType":"ai"}`, map[string]any{}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rep := &capture{}
			s := newTestServer(t, rep)
			s.send(t, http.MethodPost, tc.target, "application/json", []byte(tc.body))
			if o := rep.only(t); !sameDims(o.Dimensions, tc.want) {
				t.Errorf("dimensions = %v, want %v", o.Dimensions, tc.want)
			}
			if !strings.Contains(tc.target, "early") && string(s.lastBody) != tc.body {
				t.Errorf("the handler read %q, want the body as sent", s.lastBody)
			}
		})
	}
}

// A body larger than the capture limit is not buffered: the handler still
// reads all of it, and the dimensions it would have carried are unknown.
func TestMiddlewareDoesNotBufferALargeBody(t *testing.T) {
	rep := &capture{}
	s := newTestServer(t, rep)
	body := `{"functionalityType":"ai","padding":"` + strings.Repeat("x", maxCapturedBody) + `"}`
	s.send(t, http.MethodPost, "/api/v0.9/gateways", "application/json", []byte(body))
	if len(s.lastBody) != len(body) {
		t.Fatalf("the handler read %d bytes, want %d", len(s.lastBody), len(body))
	}
	if o := rep.only(t); len(o.Dimensions) != 0 {
		t.Errorf("dimensions = %v, want none from an uncaptured body", o.Dimensions)
	}
}

// A required dimension the request did not yield drops the event in the
// reporter, naming the dimension, never a value.
func TestAMissingRequiredDimensionIsRejected(t *testing.T) {
	rep := &capture{}
	s := newTestServer(t, rep)
	s.send(t, http.MethodPost, "/api/v0.9/gateways?early=1", "application/json", []byte(`{}`))
	_, err := validate(mustTaxonomy(t), boundFeatures(testBindings), rep.only(t), time.Now())
	if err == nil || !strings.Contains(err.Error(), `required dimension "gateway_type" is missing`) {
		t.Fatalf("err = %v, want the missing gateway_type named", err)
	}
}

// RecordOperation reports work done outside a REST route as the operation it
// did the job of, under the endpoint actually invoked.
func TestRecordOperation(t *testing.T) {
	rep := &capture{}
	started := time.Now()
	RecordOperation(rep, "CreateRESTAPI", testOrgUUID, testUserUUID, http.MethodPost, "/api/v0.9/mcp", http.StatusBadRequest, started)

	o := rep.only(t)
	if o.Feature != featureRESTAPICreate || o.OrgUUID != testOrgUUID || o.ActorSubject != testUserUUID ||
		o.Invocation.URI != "https://platform-api/api/v0.9/mcp" || o.Invocation.Status != http.StatusBadRequest ||
		!o.Invocation.RequestTime.Equal(started) || !canonicalUUID(o.OccurrenceID) {
		t.Errorf("unexpected occurrence: %+v", o)
	}
	event, err := validate(mustTaxonomy(t), enabledFeatures, o, time.Now())
	if err != nil {
		t.Fatalf("a recorded operation failed validation: %v", err)
	}
	if event.metadata["outcome"] != "failure" {
		t.Errorf("outcome = %v, want failure for status 400", event.metadata["outcome"])
	}

	// Set dimensions come along; there is no request to read the rest from.
	rep = &capture{}
	RecordOperation(rep, "DeleteGraphQLAPI", testOrgUUID, "", http.MethodPost, "/api/v0.9/mcp", http.StatusNoContent, started)
	if o := rep.only(t); o.Feature != "apip.manage-apis.api-delete" || o.Dimensions["api_type"] != "graphql-api" {
		t.Errorf("unexpected occurrence: %+v", o)
	}
}

func TestRecordOperationReportsNothingWithoutWhatItNeeds(t *testing.T) {
	rep := &capture{}
	RecordOperation(rep, "CreateRESTAPI", "", "", http.MethodPost, "/api/v0.9/mcp", 201, time.Now())             // no organization
	RecordOperation(rep, "CreateRESTAPI", testOrgUUID, "", http.MethodPost, "api/v0.9/mcp", 201, time.Now())     // not a template path
	RecordOperation(rep, "NoSuchOperation", testOrgUUID, "", http.MethodPost, "/api/v0.9/mcp", 201, time.Now())  // not bound
	RecordOperation(nil, "CreateRESTAPI", testOrgUUID, "", http.MethodPost, "/api/v0.9/mcp", 201, time.Now())    // no reporter
	RecordOperation(Noop(), "CreateRESTAPI", testOrgUUID, "", http.MethodPost, "/api/v0.9/mcp", 201, time.Now()) // reporting off
	if n := len(rep.all()); n != 0 {
		t.Fatalf("recorded %d occurrences, want none", n)
	}
}

func sameDims(got, want map[string]any) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

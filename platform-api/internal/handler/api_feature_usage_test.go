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

// Feature-usage reporting through the real handlers: the featureusage
// middleware wraps the handlers' mux the way the server wraps it, with the
// bindings in featureusage/bindings.go and resources/openapi.yaml. These tests
// show one event per call, successful or failed, and that dimensions read
// from a form are the ones the handler itself parsed.

package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/featureusage"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

// recordingUsage is a featureusage.Reporter that keeps what it is given.
type recordingUsage struct {
	mu          sync.Mutex
	occurrences []featureusage.Occurrence
}

func (r *recordingUsage) Record(o featureusage.Occurrence) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.occurrences = append(r.occurrences, o)
}

func (r *recordingUsage) Close(context.Context) error { return nil }

func (r *recordingUsage) recorded() []featureusage.Occurrence {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]featureusage.Occurrence(nil), r.occurrences...)
}

// onlyOccurrence returns the single recorded occurrence, failing otherwise.
func (r *recordingUsage) onlyOccurrence(t *testing.T) featureusage.Occurrence {
	t.Helper()
	got := r.recorded()
	if len(got) != 1 {
		t.Fatalf("recorded %d occurrences, want 1", len(got))
	}
	return got[0]
}

var openAPIDocument = sync.OnceValues(func() ([]byte, error) {
	return os.ReadFile("../../resources/openapi.yaml")
})

// withFeatureUsage wraps mux in the feature-usage middleware as the server
// does, reporting to rep.
func withFeatureUsage(t *testing.T, mux *http.ServeMux, rep featureusage.Reporter) http.Handler {
	t.Helper()
	spec, err := openAPIDocument()
	if err != nil {
		t.Fatal(err)
	}
	tax, err := featureusage.DefaultTaxonomy()
	if err != nil {
		t.Fatal(err)
	}
	index, err := featureusage.NewRouteIndex(spec, mux, tax, nil)
	if err != nil {
		t.Fatal(err)
	}
	return featureusage.Middleware(featureusage.MiddlewareConfig{
		Reporter: func() featureusage.Reporter { return rep },
		Index:    index,
		Routes:   mux,
		Identity: func(r *http.Request) (string, string) {
			org, _ := middleware.GetOrganizationFromRequest(r)
			actor, _ := middleware.GetActorIdentityFromRequest(r)
			return org, actor
		},
	})(mux)
}

// newTrackedOAEnv is the OpenAPI handler environment with reporting on.
func newTrackedOAEnv(t *testing.T) *oaEnv {
	env := newOAEnv(t)
	env.handler = withFeatureUsage(t, env.mux, env.usage)
	return env
}

const featureRESTAPICreate = "apip.manage-apis.rest-api-create"

// wantEvent is what one reported creation call should look like.
type wantEvent struct {
	route  string
	status int
}

// assertEvent checks the single reported occurrence against want.
func assertEvent(t *testing.T, env *oaEnv, want wantEvent, before time.Time) {
	t.Helper()
	o := env.usage.onlyOccurrence(t)
	if o.Feature != featureRESTAPICreate || o.OrgUUID != oaOrg {
		t.Errorf("unexpected occurrence: %+v", o)
	}
	// rest-api-create declares no dimensions but outcome, which the reporter
	// derives from the status.
	if len(o.Dimensions) != 0 {
		t.Errorf("dimensions = %v, want none", o.Dimensions)
	}
	inv := o.Invocation
	if inv.Method != http.MethodPost || inv.Status != want.status || inv.URI != "https://platform-api"+want.route {
		t.Errorf("invocation = %s %s %d, want POST https://platform-api%s %d", inv.Method, inv.URI, inv.Status, want.route, want.status)
	}
	if inv.RequestTime.Before(before) || inv.RequestTime.After(time.Now()) {
		t.Errorf("request time %s is not when the call was received", inv.RequestTime)
	}
	if _, err := uuid.Parse(o.OccurrenceID); err != nil {
		t.Errorf("occurrence ID = %q, want a random UUID", o.OccurrenceID)
	}
}

func importAPI(t *testing.T, env *oaEnv) int {
	t.Helper()
	body, ct := oaImportBody(fullImportFields(), "orders.yaml", []byte(validOpenAPI))
	return env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct).Code
}

func TestImportOpenAPI_ReportsACreation(t *testing.T) {
	env := newTrackedOAEnv(t)
	before := time.Now()
	if code := importAPI(t, env); code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", code)
	}
	assertEvent(t, env, wantEvent{importPath, http.StatusCreated}, before)
}

// A rejected import is reported as the failure the caller saw, and the
// validation error the caller gets is unchanged.
func TestImportOpenAPI_ReportsARejectedImport(t *testing.T) {
	env := newTrackedOAEnv(t)
	before := time.Now()
	fields := []oaFormField{
		{"version", oaVersion}, {"context", oaContext}, {"projectId", oaProjectHand},
		{"upstream", oaMainUpstream}, // displayName missing
	}
	body, ct := oaImportBody(fields, "orders.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct)

	oaAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
	if !strings.Contains(rec.Body.String(), "displayName is required") {
		t.Errorf("the validation error changed: %s", rec.Body.String())
	}
	assertEvent(t, env, wantEvent{importPath, http.StatusBadRequest}, before)
}

// A rolled-back create is reported as the failure the caller saw.
func TestImportOpenAPI_ReportsARollbackAsAFailure(t *testing.T) {
	env := newTrackedOAEnv(t)
	env.docRepo.createErr = errors.New("disk full")
	env.docRepo.upsertErr = errors.New("disk full")
	before := time.Now()

	if code := importAPI(t, env); code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", code)
	}
	assertEvent(t, env, wantEvent{importPath, http.StatusInternalServerError}, before)
}

func TestImportOpenAPI_ReportsAFailedCreate(t *testing.T) {
	env := newTrackedOAEnv(t)
	fields := fullImportFields()
	for i := range fields {
		if fields[i].name == "projectId" {
			fields[i].value = "no-such-project"
		}
	}
	before := time.Now()
	body, ct := oaImportBody(fields, "orders.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	assertEvent(t, env, wantEvent{importPath, http.StatusNotFound}, before)
}

// A call with no organization cannot be attributed and is not reported.
func TestImportOpenAPI_CallWithoutAnOrganizationIsNotReported(t *testing.T) {
	env := newTrackedOAEnv(t)
	body, ct := oaImportBody(fullImportFields(), "orders.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPost, importPath, "", "alice", bytes.NewReader(body), ct)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if n := len(env.usage.recorded()); n != 0 {
		t.Errorf("recorded %d occurrences for an unattributable call", n)
	}
}

const createPath = constants.APIBasePath + "/rest-apis"

func createJSON(project, upstream string) string {
	return `{"displayName":"` + oaDisplayName + `","context":"` + oaContext + `","version":"` + oaVersion +
		`","projectId":"` + project + `","upstream":` + upstream + `}`
}

func TestCreateAPI_ReportsACreation(t *testing.T) {
	env := newTrackedOAEnv(t)
	before := time.Now()
	rec := env.do(t, http.MethodPost, createPath+"?trace=alice", oaOrg, "alice",
		strings.NewReader(createJSON(oaProjectHand, oaMainUpstream)), "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	// The query string is never part of the reported call.
	assertEvent(t, env, wantEvent{createPath, http.StatusCreated}, before)
}

func TestCreateAPI_ReportsFailures(t *testing.T) {
	tests := map[string]struct {
		body   string
		status int
	}{
		"unknown project":   {createJSON("no-such-project", oaMainUpstream), http.StatusNotFound},
		"missing upstream":  {createJSON(oaProjectHand, `{"main":{}}`), http.StatusBadRequest},
		"malformed request": {`{"displayName":`, http.StatusBadRequest},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			env := newTrackedOAEnv(t)
			before := time.Now()
			rec := env.do(t, http.MethodPost, createPath, oaOrg, "alice", strings.NewReader(tc.body), "application/json")
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.status, rec.Body.String())
			}
			assertEvent(t, env, wantEvent{createPath, tc.status}, before)
		})
	}
}

// Validation reports whether the contract came as a file or a URL, never the
// URL itself.
func TestValidateOpenAPI_IsReportedAsAContractValidation(t *testing.T) {
	env := newTrackedOAEnv(t)
	body, ct := oaImportBody(nil, "orders.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPost, constants.APIBasePath+"/rest-apis/validate-openapi", oaOrg, "alice", bytes.NewReader(body), ct)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	o := env.usage.onlyOccurrence(t)
	if o.Feature != "apip.manage-apis.api-contract-validate" ||
		o.Dimensions["contract_type"] != "openapi" || o.Dimensions["contract_source"] != "file" {
		t.Errorf("unexpected occurrence: %+v", o)
	}
}

// The document handler parses the form; the middleware reads the document
// type from that same parsed form, in the taxonomy's spelling whatever case
// the caller used. A type the API rejects is free text: the failed call is
// reported without it, never with the text as typed.
func TestCreateDocument_ReportsTheDocumentType(t *testing.T) {
	tests := map[string]struct {
		docType string
		status  int
		want    map[string]any
	}{
		"canonical":    {"SupportForum", http.StatusCreated, map[string]any{"api_type": "rest-api", "document_type": "support-forum"}},
		"lower case":   {"howto", http.StatusCreated, map[string]any{"api_type": "rest-api", "document_type": "how-to"}},
		"unknown type": {"Runbook for alice", http.StatusBadRequest, map[string]any{"api_type": "rest-api"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			env := newDocHEnv(t, nil)
			usage := &recordingUsage{}
			handler := withFeatureUsage(t, env.mux, usage)
			body, ct := docHForm(t, map[string]string{
				"type": tc.docType, "displayName": "Getting Started", "inlineContent": "# Hello",
			})
			req := middleware.WithOrganization(httptestRequest(http.MethodPost, docHDocsBase, body, ct), docHOrg)
			rec := serve(handler, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.status, rec.Body.String())
			}
			o := usage.onlyOccurrence(t)
			if o.Feature != "apip.manage-apis.api-document-create" || o.Invocation.Status != tc.status {
				t.Errorf("unexpected occurrence: %+v", o)
			}
			if len(o.Dimensions) != len(tc.want) {
				t.Errorf("dimensions = %v, want %v", o.Dimensions, tc.want)
			}
			for k, v := range tc.want {
				if o.Dimensions[k] != v {
					t.Errorf("dimensions = %v, want %v", o.Dimensions, tc.want)
				}
			}
			if o.Invocation.URI != "https://platform-api/api/v0.9/apis/{apiType}/{apiId}/docs" {
				t.Errorf("URI = %q, want the route template", o.Invocation.URI)
			}
		})
	}
}

func TestDeleteThumbnail_ReportsTheAPIType(t *testing.T) {
	env := newDocHEnv(t, nil)
	usage := &recordingUsage{}
	handler := withFeatureUsage(t, env.mux, usage)
	req := middleware.WithOrganization(httptestRequest(http.MethodDelete, docHThumbPath, nil, ""), docHOrg)
	serve(handler, req)
	o := usage.onlyOccurrence(t)
	if o.Feature != "apip.manage-apis.api-thumbnail-remove" || o.Dimensions["api_type"] != "rest-api" {
		t.Errorf("unexpected occurrence: %+v", o)
	}
}

func httptestRequest(method, target string, body io.Reader, contentType string) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

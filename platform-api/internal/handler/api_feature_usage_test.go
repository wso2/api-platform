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

// Feature-usage reporting for REST API creation: one
// apip.manage-apis.create-rest-api event per call to a REST API creation route
// (POST /rest-apis, POST /rest-apis/import-openapi), successful or failed,
// described by that call and its status, with the creation_method the console
// declares ("contract" when nobody declares one).

package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/featureusage"
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

// wantEvent is what one reported creation call should look like.
type wantEvent struct {
	route   string
	status  int
	method  featureusage.CreationMethod
	sandbox any    // true/false, or nil when the call failed before it was known
	actor   string // the caller's subject, read before any validation
}

// assertEvent checks the single reported occurrence against want.
func assertEvent(t *testing.T, env *oaEnv, want wantEvent, before time.Time) {
	t.Helper()
	o := env.usage.onlyOccurrence(t)
	if o.Feature != featureusage.FeatureCreateRESTAPI || o.OrgUUID != oaOrg || o.ActorSubject != want.actor {
		t.Errorf("unexpected occurrence: %+v", o)
	}
	if o.Dimensions["creation_method"] != string(want.method) {
		t.Errorf("creation_method = %v, want %s", o.Dimensions["creation_method"], want.method)
	}
	if got, present := o.Dimensions["has_sandbox_upstream"]; got != want.sandbox || (want.sandbox == nil && present) {
		t.Errorf("has_sandbox_upstream = %v (present %v), want %v", got, present, want.sandbox)
	}
	inv := o.Invocation
	if inv.Method != http.MethodPost || inv.Status != want.status || inv.URI != "http://example.com"+want.route {
		t.Errorf("invocation = %s %s %d, want POST http://example.com%s %d", inv.Method, inv.URI, inv.Status, want.route, want.status)
	}
	if inv.RequestTime.Before(before) || inv.RequestTime.After(time.Now()) {
		t.Errorf("request time %s is not when the call was received", inv.RequestTime)
	}
	// A successful creation is identified by the API it created; a failed
	// call gets its own random ID.
	if succeeded := want.status >= 200 && want.status < 300; succeeded != (o.OccurrenceID == oaAPIUUID) {
		t.Errorf("occurrence ID = %q for status %d", o.OccurrenceID, want.status)
	}
}

func importWithMethod(t *testing.T, env *oaEnv, method string) int {
	t.Helper()
	fields := fullImportFields()
	if method != "" {
		fields = append(fields, oaFormField{"creationMethod", method})
	}
	body, ct := oaImportBody(fields, "orders.yaml", []byte(validOpenAPI))
	return env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct).Code
}

func TestImportOpenAPI_ReportsACreationWithTheDeclaredMethod(t *testing.T) {
	for declared, want := range map[string]featureusage.CreationMethod{
		"":         featureusage.CreationMethodContract, // direct API / CLI caller
		"contract": featureusage.CreationMethodContract,
		"scratch":  featureusage.CreationMethodScratch, // console "Start from scratch"
	} {
		t.Run("creationMethod="+declared, func(t *testing.T) {
			env := newOAEnv(t)
			before := time.Now()
			if code := importWithMethod(t, env, declared); code != http.StatusCreated {
				t.Fatalf("status = %d, want 201", code)
			}
			assertEvent(t, env, wantEvent{importPath, http.StatusCreated, want, false, "alice"}, before)
		})
	}
}

// An undeclared or invalid method is not a scratch declaration: the rejected
// call is reported as a failed "contract" creation.
func TestImportOpenAPI_ReportsARejectedCreationMethod(t *testing.T) {
	env := newOAEnv(t)
	before := time.Now()
	fields := append(fullImportFields(), oaFormField{"creationMethod", "imported"})
	body, ct := oaImportBody(fields, "orders.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct)

	oaAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
	if env.apiRepo.createdID != "" {
		t.Error("an API was created despite the invalid creationMethod")
	}
	assertEvent(t, env, wantEvent{importPath, http.StatusBadRequest, featureusage.CreationMethodContract, false, "alice"}, before)
}

// A scratch import that fails validation is still reported as scratch, and
// the validation error the caller gets is unchanged.
func TestImportOpenAPI_ReportsARejectedScratchImportAsScratch(t *testing.T) {
	env := newOAEnv(t)
	before := time.Now()
	fields := []oaFormField{
		{"version", oaVersion}, {"context", oaContext}, {"projectId", oaProjectHand},
		{"upstream", oaMainUpstream}, {"creationMethod", "scratch"}, // displayName missing
	}
	body, ct := oaImportBody(fields, "orders.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct)

	oaAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
	if !strings.Contains(rec.Body.String(), "displayName is required") {
		t.Errorf("the validation error changed: %s", rec.Body.String())
	}
	assertEvent(t, env, wantEvent{importPath, http.StatusBadRequest, featureusage.CreationMethodScratch, false, "alice"}, before)
}

// Both problems present: the field check still answers first, as before.
func TestImportOpenAPI_InvalidMethodDoesNotChangeTheValidationOrder(t *testing.T) {
	env := newOAEnv(t)
	fields := []oaFormField{
		{"version", oaVersion}, {"context", oaContext}, {"projectId", oaProjectHand},
		{"upstream", oaMainUpstream}, {"creationMethod", "imported"}, // displayName missing too
	}
	body, ct := oaImportBody(fields, "orders.yaml", []byte(validOpenAPI))
	rec := env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct)
	oaAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
	if !strings.Contains(rec.Body.String(), "displayName is required") {
		t.Errorf("want the displayName error first, got: %s", rec.Body.String())
	}
}

// A rolled-back create is reported as the failure the caller saw.
func TestImportOpenAPI_ReportsARollbackAsAFailure(t *testing.T) {
	env := newOAEnv(t)
	env.docRepo.createErr = errors.New("disk full")
	env.docRepo.upsertErr = errors.New("disk full")
	before := time.Now()

	if code := importWithMethod(t, env, "scratch"); code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", code)
	}
	assertEvent(t, env, wantEvent{importPath, http.StatusInternalServerError, featureusage.CreationMethodScratch, false, "alice"}, before)
}

func TestImportOpenAPI_ReportsAFailedCreate(t *testing.T) {
	env := newOAEnv(t)
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
	assertEvent(t, env, wantEvent{importPath, http.StatusNotFound, featureusage.CreationMethodContract, false, "alice"}, before)
}

// A call with no organization cannot be attributed and is not reported.
func TestImportOpenAPI_CallWithoutAnOrganizationIsNotReported(t *testing.T) {
	env := newOAEnv(t)
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

// POST /rest-apis always carries the definition itself: "contract".
func TestCreateAPI_ReportsAContractCreation(t *testing.T) {
	for name, tc := range map[string]struct {
		upstream string
		sandbox  bool
	}{
		"main upstream only":      {oaMainUpstream, false},
		"with a sandbox upstream": {`{"main":{"url":"http://upstream:3000"},"sandbox":{"url":"http://sandbox:3000"}}`, true},
	} {
		t.Run(name, func(t *testing.T) {
			env := newOAEnv(t)
			before := time.Now()
			rec := env.do(t, http.MethodPost, createPath+"?trace=alice", oaOrg, "alice",
				strings.NewReader(createJSON(oaProjectHand, tc.upstream)), "application/json")
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
			}
			// The query string is never part of the reported call.
			assertEvent(t, env, wantEvent{createPath, http.StatusCreated, featureusage.CreationMethodContract, tc.sandbox, "alice"}, before)
		})
	}
}

func TestCreateAPI_ReportsFailures(t *testing.T) {
	tests := map[string]struct {
		body    string
		status  int
		sandbox any
		actor   string
	}{
		"unknown project":   {createJSON("no-such-project", oaMainUpstream), http.StatusNotFound, false, "alice"},
		"missing upstream":  {createJSON(oaProjectHand, `{"main":{}}`), http.StatusBadRequest, false, "alice"},
		"malformed request": {`{"displayName":`, http.StatusBadRequest, nil, "alice"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			env := newOAEnv(t)
			before := time.Now()
			rec := env.do(t, http.MethodPost, createPath, oaOrg, "alice", strings.NewReader(tc.body), "application/json")
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.status, rec.Body.String())
			}
			assertEvent(t, env, wantEvent{createPath, tc.status, featureusage.CreationMethodContract, tc.sandbox, tc.actor}, before)
		})
	}
}

func TestParseCreationMethod(t *testing.T) {
	for raw, want := range map[string]featureusage.CreationMethod{
		"":           featureusage.CreationMethodContract,
		"  ":         featureusage.CreationMethodContract,
		"scratch":    featureusage.CreationMethodScratch,
		" contract ": featureusage.CreationMethodContract,
	} {
		got, err := parseCreationMethod(raw)
		if err != nil || got != want {
			t.Errorf("parseCreationMethod(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"Scratch", "file", "url"} {
		if _, err := parseCreationMethod(raw); err == nil {
			t.Errorf("parseCreationMethod(%q) accepted an unknown method", raw)
		}
	}
}

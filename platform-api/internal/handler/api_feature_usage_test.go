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
// apip.manage-apis.rest-api-create event per call to a REST API creation route
// (POST /rest-apis, POST /rest-apis/import-openapi), successful or failed,
// described only by the endpoint invocation: the route, the status, the
// caller and the organization.

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
	route  string
	status int
	actor  string // the caller's subject, read before any validation
}

// assertEvent checks the single reported occurrence against want.
func assertEvent(t *testing.T, env *oaEnv, want wantEvent, before time.Time) {
	t.Helper()
	o := env.usage.onlyOccurrence(t)
	if o.Feature != featureusage.FeatureCreateRESTAPI || o.OrgUUID != oaOrg || o.ActorSubject != want.actor {
		t.Errorf("unexpected occurrence: %+v", o)
	}
	// The event is the endpoint invocation only: no feature dimensions.
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
	// A successful creation is identified by the API it created; a failed
	// call gets its own random ID.
	if succeeded := want.status >= 200 && want.status < 300; succeeded != (o.OccurrenceID == oaAPIUUID) {
		t.Errorf("occurrence ID = %q for status %d", o.OccurrenceID, want.status)
	}
}

func importAPI(t *testing.T, env *oaEnv) int {
	t.Helper()
	body, ct := oaImportBody(fullImportFields(), "orders.yaml", []byte(validOpenAPI))
	return env.do(t, http.MethodPost, importPath, oaOrg, "alice", bytes.NewReader(body), ct).Code
}

func TestImportOpenAPI_ReportsACreation(t *testing.T) {
	env := newOAEnv(t)
	before := time.Now()
	if code := importAPI(t, env); code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", code)
	}
	assertEvent(t, env, wantEvent{importPath, http.StatusCreated, "alice"}, before)
}

// A rejected import is reported as the failure the caller saw, and the
// validation error the caller gets is unchanged.
func TestImportOpenAPI_ReportsARejectedImport(t *testing.T) {
	env := newOAEnv(t)
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
	assertEvent(t, env, wantEvent{importPath, http.StatusBadRequest, "alice"}, before)
}

// A rolled-back create is reported as the failure the caller saw.
func TestImportOpenAPI_ReportsARollbackAsAFailure(t *testing.T) {
	env := newOAEnv(t)
	env.docRepo.createErr = errors.New("disk full")
	env.docRepo.upsertErr = errors.New("disk full")
	before := time.Now()

	if code := importAPI(t, env); code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", code)
	}
	assertEvent(t, env, wantEvent{importPath, http.StatusInternalServerError, "alice"}, before)
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
	assertEvent(t, env, wantEvent{importPath, http.StatusNotFound, "alice"}, before)
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

func TestCreateAPI_ReportsACreation(t *testing.T) {
	env := newOAEnv(t)
	before := time.Now()
	rec := env.do(t, http.MethodPost, createPath+"?trace=alice", oaOrg, "alice",
		strings.NewReader(createJSON(oaProjectHand, oaMainUpstream)), "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	// The query string is never part of the reported call.
	assertEvent(t, env, wantEvent{createPath, http.StatusCreated, "alice"}, before)
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
			env := newOAEnv(t)
			before := time.Now()
			rec := env.do(t, http.MethodPost, createPath, oaOrg, "alice", strings.NewReader(tc.body), "application/json")
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.status, rec.Body.String())
			}
			assertEvent(t, env, wantEvent{createPath, tc.status, "alice"}, before)
		})
	}
}

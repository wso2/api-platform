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

// Integration tests for the /api-portals handler, covering the full
// route → handler → service → repository stack backed by SQLite.

package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"

	_ "github.com/mattn/go-sqlite3"
)

const apiPortalTestBase = "/api/v0.9/api-portals"
const apiPortalTestOrg = "org-portal-it"
const apiPortalTestUser = "sub-portal-tester"

// setupAPIPortalHandlerEnv brings up the full API-Portal handler stack against a
// fresh SQLite database and seeds the parent organization row the FK requires.
func setupAPIPortalHandlerEnv(t *testing.T) (http.Handler, *database.DB, func()) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "api-portal-test.db")
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db := &database.DB{DB: sqlDB}

	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	if _, err = db.Exec(
		`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		 VALUES (?, ?, 'Portal Test Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`,
		apiPortalTestOrg, "test-org-"+apiPortalTestOrg,
	); err != nil {
		t.Fatalf("insert org: %v", err)
	}

	portalRepo := repository.NewAPIPortalRepo(db)
	orgRepo := repository.NewOrganizationRepo(db)
	identityService := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	svc := service.NewAPIPortalService(portalRepo, orgRepo, noopAudit{}, identityService, slog.Default())
	h := NewAPIPortalHandler(svc, identityService, slog.Default())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return middleware.NewTestContextMiddleware(mux), db, func() { _ = sqlDB.Close() }
}

// apiPortalTestRequest builds a request with the test auth headers set.
func apiPortalTestRequest(t *testing.T, method, path string, body []byte) *http.Request {
	t.Helper()
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("X-Test-User", apiPortalTestUser)
	r.Header.Set("X-Test-Org", apiPortalTestOrg)
	return r
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// Minimal response shapes for decoding — mirror the fields the handler emits.
// Using a dedicated local shape avoids the pointer maze of api.ApiPortalResponse.
type apiPortalResp struct {
	Id             string                 `json:"id"`
	Handle         string                 `json:"handle"`
	Name           string                 `json:"name"`
	Description    *string                `json:"description,omitempty"`
	Url            *string                `json:"url,omitempty"`
	WorkflowStatus string                 `json:"workflowStatus"`
	AuthType       string                 `json:"authType"`
	Config         map[string]interface{} `json:"config,omitempty"`
}

type apiPortalListResp struct {
	Count      int             `json:"count"`
	List       []apiPortalResp `json:"list"`
	Pagination struct {
		Total  int `json:"total"`
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	} `json:"pagination"`
}

type apiPortalErrorResp struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// --- CREATE ---

func TestAPIPortalHandler_Create_HappyPath(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{
		"name":     "Acme Portal",
		"handle":   "acme",
		"authType": "local",
		"config":   map[string]any{"foo": "bar"},
	})
	req := apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Create: want 201, got %d: %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasSuffix(loc, "/api-portals/acme") {
		t.Errorf("Location header wrong: %q", loc)
	}
	var got apiPortalResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Id != "acme" || got.Handle != "acme" || got.Name != "Acme Portal" ||
		got.AuthType != "local" || got.WorkflowStatus != "pending" {
		t.Errorf("response fields wrong: %+v", got)
	}
	if got.Config["foo"] != "bar" {
		t.Errorf("config round-trip failed: %v", got.Config)
	}
}

func TestAPIPortalHandler_Create_MissingName(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{
		"handle":   "acme",
		"authType": "local",
	})
	req := apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for missing name, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIPortalHandler_Create_WithActiveStatus(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{
		"name":           "Acme Portal",
		"handle":         "acme-active",
		"authType":       "local",
		"url":            "https://acme.example.com",
		"workflowStatus": "active",
	})
	req := apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("Create: want 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var got apiPortalResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.WorkflowStatus != "active" {
		t.Errorf("want active, got %q", got.WorkflowStatus)
	}
}

func TestAPIPortalHandler_Create_ActiveWithoutURL(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{
		"name":           "Acme Portal",
		"handle":         "acme-bad",
		"authType":       "local",
		"workflowStatus": "active",
	})
	req := apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Create: want 400 for active without url, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIPortalHandler_Create_HandleConflict(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{"name": "a", "handle": "dup", "authType": "local"})
	req := apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first Create: want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Second POST with the same handle must be 409.
	req2 := apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("duplicate Create: want 409, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var errBody apiPortalErrorResp
	if err := json.Unmarshal(rec2.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if errBody.Code != "API_PORTAL_EXISTS" {
		t.Errorf("error code: want API_PORTAL_EXISTS, got %q", errBody.Code)
	}
}

func TestAPIPortalHandler_Create_MissingOrg(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{"name": "a", "handle": "acme", "authType": "local"})
	// Deliberately DO NOT set X-Test-Org; expect 401 from the handler's org guard.
	req := httptest.NewRequest(http.MethodPost, apiPortalTestBase, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", apiPortalTestUser)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for missing org context, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- GET (single) ---

func TestAPIPortalHandler_Get_HappyPath(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	// Seed via POST.
	body := mustJSON(t, map[string]any{"name": "Acme", "handle": "acme", "authType": "local"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed Create failed: %d %s", rec.Code, rec.Body.String())
	}

	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, apiPortalTestRequest(t, http.MethodGet, apiPortalTestBase+"/acme", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("Get: want 200, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var got apiPortalResp
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Handle != "acme" || got.Name != "Acme" {
		t.Errorf("Get response wrong: %+v", got)
	}
}

func TestAPIPortalHandler_Get_NotFound(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodGet, apiPortalTestBase+"/ghost", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("Get missing: want 404, got %d: %s", rec.Code, rec.Body.String())
	}
	var errBody apiPortalErrorResp
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if errBody.Code != "API_PORTAL_NOT_FOUND" {
		t.Errorf("error code: want API_PORTAL_NOT_FOUND, got %q", errBody.Code)
	}
}

// --- LIST ---

func TestAPIPortalHandler_List_HappyPath(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	// Seed 3 portals.
	for _, h := range []string{"one", "two", "three"} {
		body := mustJSON(t, map[string]any{"name": "P " + h, "handle": h, "authType": "local"})
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body))
		if rec.Code != http.StatusCreated {
			t.Fatalf("seed %s: %d %s", h, rec.Code, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodGet, apiPortalTestBase, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("List: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got apiPortalListResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Count != 3 || got.Pagination.Total != 3 || len(got.List) != 3 {
		t.Errorf("counts wrong: %+v", got)
	}
	if got.Pagination.Limit != 20 {
		t.Errorf("default limit: want 20, got %d", got.Pagination.Limit)
	}
}

func TestAPIPortalHandler_List_WorkflowStatusFilter(t *testing.T) {
	r, db, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	// Seed 3 portals (handle min length is 3). WorkflowStatus can't be set on
	// Create body (it defaults to "pending" server-side), so bump one row via
	// SQL directly to exercise the status filter.
	for _, h := range []string{"aaa", "bbb", "ccc"} {
		body := mustJSON(t, map[string]any{"name": "P " + h, "handle": h, "authType": "local"})
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body))
		if rec.Code != http.StatusCreated {
			t.Fatalf("seed %s: %d %s", h, rec.Code, rec.Body.String())
		}
	}
	if _, err := db.Exec(`UPDATE api_portals SET workflow_status = 'active' WHERE handle = 'ccc'`); err != nil {
		t.Fatalf("bump status: %v", err)
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodGet, apiPortalTestBase+"?workflowStatus=active", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("List: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got apiPortalListResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Count != 1 || got.List[0].Handle != "ccc" {
		t.Errorf("filter miss: %+v", got)
	}
}

// --- UPDATE ---

func TestAPIPortalHandler_Update_HappyPath(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	// Seed.
	body := mustJSON(t, map[string]any{"name": "old", "handle": "acme", "authType": "local"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed: %d %s", rec.Code, rec.Body.String())
	}

	// Update name + authType.
	patch := mustJSON(t, map[string]any{"name": "new", "authType": "oauth2"})
	putRec := httptest.NewRecorder()
	r.ServeHTTP(putRec, apiPortalTestRequest(t, http.MethodPut, apiPortalTestBase+"/acme", patch))
	if putRec.Code != http.StatusOK {
		t.Fatalf("Update: want 200, got %d: %s", putRec.Code, putRec.Body.String())
	}
	var got apiPortalResp
	if err := json.Unmarshal(putRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "new" || got.AuthType != "oauth2" {
		t.Errorf("mutable fields not applied: %+v", got)
	}
	if got.Handle != "acme" {
		t.Errorf("handle mutated: %q", got.Handle)
	}
}

func TestAPIPortalHandler_Update_NotFound(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	patch := mustJSON(t, map[string]any{"name": "x"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodPut, apiPortalTestBase+"/ghost", patch))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- DELETE ---

func TestAPIPortalHandler_Delete_HappyPath(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{"name": "x", "handle": "gone", "authType": "local"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed: %d %s", rec.Code, rec.Body.String())
	}

	delRec := httptest.NewRecorder()
	r.ServeHTTP(delRec, apiPortalTestRequest(t, http.MethodDelete, apiPortalTestBase+"/gone", nil))
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("Delete: want 204, got %d: %s", delRec.Code, delRec.Body.String())
	}

	// Subsequent Get is 404.
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, apiPortalTestRequest(t, http.MethodGet, apiPortalTestBase+"/gone", nil))
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("Get after Delete: want 404, got %d", getRec.Code)
	}
}

func TestAPIPortalHandler_Delete_NotFound(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodDelete, apiPortalTestBase+"/ghost", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("Delete missing: want 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

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
	"github.com/wso2/api-platform/platform-api/internal/vault"

	_ "github.com/mattn/go-sqlite3"
)

// apiPortalTestVault returns a deterministic in-house vault for integration tests.
func apiPortalTestVault(t *testing.T) vault.SecretVault {
	t.Helper()
	v, err := vault.NewInHouseVault(bytes.Repeat([]byte("t"), 32))
	if err != nil {
		t.Fatalf("test vault: %v", err)
	}
	return v
}

const apiPortalTestBase = "/api/v0.9/api-portals"
const apiPortalTestOrg = "org-portal-it"
const apiPortalTestUser = "sub-portal-tester"

// apiPortalTestSharedKey is a syntactically valid 64-char hex value used
// throughout the integration tests. Cryptographically bogus (all-a); its role
// is just to pass service.validateAndEncryptSharedKey's format check so the
// vault.Encrypt path actually runs.
const apiPortalTestSharedKey = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

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
	svc := service.NewAPIPortalService(portalRepo, orgRepo, noopAudit{}, apiPortalTestVault(t), nil, identityService, slog.Default())
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
// Note the absence of any sharedKey / authType / authConfig field — the response
// schema does not declare them, and a rogue field appearing in the wire body
// would surface as a UnmarshalTypeError on strict decode, but this shape is
// lenient (accepts unknown fields) so we also add explicit assertions below
// that any suspicious response body payload text doesn't contain the raw.
type apiPortalResp struct {
	Id          string                 `json:"id"`
	Handle      string                 `json:"handle"`
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	Url         string                 `json:"url"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
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
		"name":      "Acme Portal",
		"handle":    "acme",
		"url":       "https://acme.example.com",
		"sharedKey": apiPortalTestSharedKey,
		"metadata":  map[string]any{"loginEnvironment": "development"},
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
		got.Url != "https://acme.example.com" {
		t.Errorf("response fields wrong: %+v", got)
	}
	if got.Metadata["loginEnvironment"] != "development" {
		t.Errorf("metadata round-trip failed: %v", got.Metadata)
	}
	// Belt-and-suspenders: the raw sharedKey MUST NOT appear anywhere in the
	// response body — not as a field, not embedded in another string, not
	// leaked via any error message.
	if strings.Contains(rec.Body.String(), apiPortalTestSharedKey) {
		t.Errorf("raw sharedKey leaked in Create response body: %s", rec.Body.String())
	}
}

func TestAPIPortalHandler_Create_SharedKey_EncryptedInDB(t *testing.T) {
	r, db, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{
		"name":      "Acme",
		"handle":    "acme-enc",
		"url":       "https://acme.example.com",
		"sharedKey": apiPortalTestSharedKey,
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("Create: want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// DB must NOT contain the plaintext sharedKey.
	var stored []byte
	if err := db.QueryRow(`SELECT internal_auth_key FROM api_portals WHERE handle = 'acme-enc'`).Scan(&stored); err != nil {
		t.Fatalf("query internal_auth_key: %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("internal_auth_key is empty")
	}
	if bytes.Contains(stored, []byte(apiPortalTestSharedKey)) {
		t.Errorf("plaintext sharedKey found in internal_auth_key blob: % x", stored)
	}
}

func TestAPIPortalHandler_Create_MissingName(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{
		"handle":    "acme",
		"url":       "https://acme.example.com",
		"sharedKey": apiPortalTestSharedKey,
	})
	req := apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for missing name, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIPortalHandler_Create_MissingURL(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{
		"name":      "Acme Portal",
		"handle":    "acme-nourl",
		"sharedKey": apiPortalTestSharedKey,
	})
	req := apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Create: want 400 for missing url, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIPortalHandler_Create_MissingSharedKey(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{
		"name":   "Acme",
		"handle": "acme-nokey",
		"url":    "https://acme.example.com",
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for missing sharedKey, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIPortalHandler_Create_HandleConflict(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{
		"name":      "a",
		"handle":    "dup",
		"url":       "https://a.example.com",
		"sharedKey": apiPortalTestSharedKey,
	})
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

	body := mustJSON(t, map[string]any{
		"name":      "a",
		"handle":    "acme",
		"url":       "https://acme.example.com",
		"sharedKey": apiPortalTestSharedKey,
	})
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
	body := mustJSON(t, map[string]any{
		"name":      "Acme",
		"handle":    "acme",
		"url":       "https://acme.example.com",
		"sharedKey": apiPortalTestSharedKey,
	})
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
	// GET must never surface the raw sharedKey — belt-and-suspenders check
	// on top of the response DTO having no sharedKey field.
	if strings.Contains(getRec.Body.String(), apiPortalTestSharedKey) {
		t.Errorf("raw sharedKey leaked in Get response body: %s", getRec.Body.String())
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
		body := mustJSON(t, map[string]any{
			"name":      "P " + h,
			"handle":    h,
			"url":       "https://" + h + ".example.com",
			"sharedKey": apiPortalTestSharedKey,
		})
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
	// List responses must never surface any sharedKey.
	if strings.Contains(rec.Body.String(), apiPortalTestSharedKey) {
		t.Errorf("raw sharedKey leaked in List response body: %s", rec.Body.String())
	}
}

// --- UPDATE ---

func TestAPIPortalHandler_Update_HappyPath(t *testing.T) {
	r, _, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	// Seed.
	body := mustJSON(t, map[string]any{
		"name":      "old",
		"handle":    "acme",
		"url":       "https://acme.example.com",
		"sharedKey": apiPortalTestSharedKey,
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed: %d %s", rec.Code, rec.Body.String())
	}

	// Update name — no sharedKey → InternalAuthKey untouched (rotation path
	// tested separately).
	patch := mustJSON(t, map[string]any{
		"name":        "new",
		"description": "an updated portal",
	})
	putRec := httptest.NewRecorder()
	r.ServeHTTP(putRec, apiPortalTestRequest(t, http.MethodPut, apiPortalTestBase+"/acme", patch))
	if putRec.Code != http.StatusOK {
		t.Fatalf("Update: want 200, got %d: %s", putRec.Code, putRec.Body.String())
	}
	var got apiPortalResp
	if err := json.Unmarshal(putRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "new" {
		t.Errorf("mutable fields not applied: %+v", got)
	}
	if got.Handle != "acme" {
		t.Errorf("handle mutated: %q", got.Handle)
	}
	if strings.Contains(putRec.Body.String(), apiPortalTestSharedKey) {
		t.Errorf("raw sharedKey leaked in Update response body: %s", putRec.Body.String())
	}
}

func TestAPIPortalHandler_Update_RotateSharedKey(t *testing.T) {
	// PUT with a fresh sharedKey rotates the stored ciphertext. Verify
	// end-to-end that (1) the response is 200 with no key material, (2) the
	// DB blob changed from what Create wrote, (3) the new plaintext is not
	// visible in the DB blob.
	r, db, cleanup := setupAPIPortalHandlerEnv(t)
	t.Cleanup(cleanup)

	body := mustJSON(t, map[string]any{
		"name":      "Acme",
		"handle":    "acme",
		"url":       "https://acme.example.com",
		"sharedKey": apiPortalTestSharedKey,
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed: %d %s", rec.Code, rec.Body.String())
	}
	var beforeKey []byte
	if err := db.QueryRow(`SELECT internal_auth_key FROM api_portals WHERE handle = 'acme'`).Scan(&beforeKey); err != nil {
		t.Fatalf("query internal_auth_key: %v", err)
	}

	newSharedKey := strings.Repeat("b", 64)
	patch := mustJSON(t, map[string]any{"sharedKey": newSharedKey})
	putRec := httptest.NewRecorder()
	r.ServeHTTP(putRec, apiPortalTestRequest(t, http.MethodPut, apiPortalTestBase+"/acme", patch))
	if putRec.Code != http.StatusOK {
		t.Fatalf("Update: want 200, got %d: %s", putRec.Code, putRec.Body.String())
	}
	if strings.Contains(putRec.Body.String(), newSharedKey) {
		t.Errorf("rotated sharedKey leaked in Update response: %s", putRec.Body.String())
	}

	var afterKey []byte
	if err := db.QueryRow(`SELECT internal_auth_key FROM api_portals WHERE handle = 'acme'`).Scan(&afterKey); err != nil {
		t.Fatalf("query internal_auth_key: %v", err)
	}
	if bytes.Equal(beforeKey, afterKey) {
		t.Error("internal_auth_key not rotated in DB")
	}
	if bytes.Contains(afterKey, []byte(newSharedKey)) {
		t.Errorf("plaintext rotated sharedKey found in DB blob: % x", afterKey)
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

	body := mustJSON(t, map[string]any{
		"name":      "x",
		"handle":    "gone",
		"url":       "https://gone.example.com",
		"sharedKey": apiPortalTestSharedKey,
	})
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

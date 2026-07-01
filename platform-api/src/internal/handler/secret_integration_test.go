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

package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/service"
	"platform-api/src/internal/vault"

	_ "github.com/mattn/go-sqlite3"
)

// setupSecretTestEnv creates a full handler stack backed by an in-memory SQLite DB.
func setupSecretTestEnv(t *testing.T) (http.Handler, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	if _, err = sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	db := &database.DB{DB: sqlDB}

	schemaPath := filepath.Join("..", "database", "schema.sqlite.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed to read schema: %v", err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatalf("failed to apply schema: %v", err)
	}

	if _, err = db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, created_at, updated_at)
		VALUES ('org-it-001', 'test-org', 'Test Org', 'default', datetime('now'), datetime('now'))`); err != nil {
		t.Fatalf("failed to insert org: %v", err)
	}
	if _, err = db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, created_at, updated_at)
		VALUES ('org-it-002', 'test-org-b', 'Test Org B', 'default', datetime('now'), datetime('now'))`); err != nil {
		t.Fatalf("failed to insert org-b: %v", err)
	}

	v, err := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}

	repo := repository.NewSecretRepo(db)
	svc := service.NewSecretService(repo, v)
	h := NewSecretHandler(svc, slog.Default())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	cleanup := func() { sqlDB.Close() }
	return middleware.NewTestContextMiddleware(mux), cleanup
}

// multipartForm encodes fields as multipart/form-data and returns the body buffer
// and the Content-Type header value (which includes the boundary).
func multipartForm(fields map[string]string) (*bytes.Buffer, string) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	w.Close()
	return buf, w.FormDataContentType()
}

// doRequest is a helper that creates a request, sets common test headers, and returns the response.
// Pass a non-nil fields map to send a multipart/form-data body; pass nil for requests with no body.
func doRequest(r http.Handler, method, path string, fields map[string]string, withAuth bool) *httptest.ResponseRecorder {
	var body *bytes.Buffer
	contentType := ""
	if fields != nil {
		body, contentType = multipartForm(fields)
	} else {
		body = &bytes.Buffer{}
	}

	req, _ := http.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if withAuth {
		req.Header.Set("X-Test-Org", "org-it-001")
		req.Header.Set("X-Test-User", "alice")
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// createSecret is a helper that POSTs a secret via multipart/form-data.
func createSecret(r http.Handler, handle, displayName, value string) *httptest.ResponseRecorder {
	return doRequest(r, http.MethodPost, "/api/v1/secrets", map[string]string{
		"handle":      handle,
		"name":        displayName,
		"value":       value,
	}, true)
}

func parseBody(w *httptest.ResponseRecorder) map[string]interface{} {
	var m map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return m
}

// TC-IT-01: Create returns 201 with handle and value fields.
func TestSecretHandler_Create_201(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	w := createSecret(r, "it-key", "IT Key", "plaintext")
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseBody(w)
	if resp["handle"] != "it-key" {
		t.Errorf("expected handle=it-key, got %v", resp["handle"])
	}
	// Subsequent GET should not have value field.
	wGet := doRequest(r, http.MethodGet, "/api/v1/secrets/it-key", nil, true)
	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET, got %d", wGet.Code)
	}
	getRespBytes := wGet.Body.Bytes()
	var getRespMap map[string]interface{}
	_ = json.Unmarshal(getRespBytes, &getRespMap)
	if _, hasValue := getRespMap["value"]; hasValue {
		t.Errorf("GET response should not contain value field")
	}
}

// TC-IT-02: Creating the same handle twice returns 409.
func TestSecretHandler_Create_409_DuplicateHandle(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	createSecret(r, "dup-key", "Dup Key", "val1")
	w := createSecret(r, "dup-key", "Dup Key 2", "val2")
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// TC-IT-03: Create without org header returns 401.
func TestSecretHandler_Create_401_NoOrg(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	body, ct := multipartForm(map[string]string{"handle": "no-org-key", "name": "No Org Key", "value": "val"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/secrets", body)
	req.Header.Set("Content-Type", ct)
	// Intentionally NOT setting X-Test-Org
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TC-IT-04: List returns pagination object with total, limit, offset.
func TestSecretHandler_List_ReturnsPaginationObject(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	for i := 1; i <= 3; i++ {
		createSecret(r, fmt.Sprintf("key-%d", i), fmt.Sprintf("Key %d", i), "val")
	}

	w := doRequest(r, http.MethodGet, "/api/v1/secrets", nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseBody(w)
	if _, hasList := resp["list"]; !hasList {
		t.Error("response should have list field")
	}
	if _, hasCount := resp["count"]; hasCount {
		t.Error("response should NOT have top-level count field")
	}

	pagination, ok := resp["pagination"].(map[string]interface{})
	if !ok {
		t.Fatalf("pagination field missing or wrong type: %v", resp["pagination"])
	}

	if int(pagination["total"].(float64)) != 3 {
		t.Errorf("expected total=3, got %v", pagination["total"])
	}
	if int(pagination["limit"].(float64)) != 25 {
		t.Errorf("expected limit=25, got %v", pagination["limit"])
	}
	if int(pagination["offset"].(float64)) != 0 {
		t.Errorf("expected offset=0, got %v", pagination["offset"])
	}
}

// TC-IT-05: List with limit/offset returns correct page.
func TestSecretHandler_List_LimitOffset(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	for i := 1; i <= 5; i++ {
		createSecret(r, fmt.Sprintf("page-key-%d", i), fmt.Sprintf("Page Key %d", i), "val")
	}

	w1 := doRequest(r, http.MethodGet, "/api/v1/secrets?limit=2&offset=0", nil, true)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w1.Code)
	}
	resp1 := parseBody(w1)
	list1 := resp1["list"].([]interface{})
	if len(list1) != 2 {
		t.Errorf("expected 2 items, got %d", len(list1))
	}
	pagination1 := resp1["pagination"].(map[string]interface{})
	if int(pagination1["total"].(float64)) != 5 {
		t.Errorf("expected total=5, got %v", pagination1["total"])
	}
	if int(pagination1["limit"].(float64)) != 2 {
		t.Errorf("expected limit=2, got %v", pagination1["limit"])
	}

	w2 := doRequest(r, http.MethodGet, "/api/v1/secrets?limit=2&offset=2", nil, true)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
	resp2 := parseBody(w2)
	list2 := resp2["list"].([]interface{})
	if len(list2) != 2 {
		t.Errorf("expected 2 items in page 2, got %d", len(list2))
	}
}

// TC-IT-06: List with future updatedAfter returns empty list.
func TestSecretHandler_List_UpdatedAfter(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	createSecret(r, "future-key", "Future Key", "val")

	w := doRequest(r, http.MethodGet, "/api/v1/secrets?updatedAfter=2099-01-01T00:00:00Z", nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseBody(w)
	list := resp["list"].([]interface{})
	if len(list) != 0 {
		t.Errorf("expected empty list for future updatedAfter, got %d items", len(list))
	}
}

// TC-IT-07: List with invalid updatedAfter returns 400.
func TestSecretHandler_List_InvalidUpdatedAfter_400(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	w := doRequest(r, http.MethodGet, "/api/v1/secrets?updatedAfter=not-a-date", nil, true)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TC-IT-08: Get by handle returns 200 without value field.
func TestSecretHandler_Get_200(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	createSecret(r, "get-key", "Get Key", "secret-val")

	w := doRequest(r, http.MethodGet, "/api/v1/secrets/get-key", nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseBody(w)
	if resp["handle"] != "get-key" {
		t.Errorf("expected handle=get-key, got %v", resp["handle"])
	}
	if _, hasValue := resp["value"]; hasValue {
		t.Errorf("GET by handle should NOT contain value field")
	}
}

// TC-IT-09: Get non-existent secret returns 404.
func TestSecretHandler_Get_404_NotFound(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	w := doRequest(r, http.MethodGet, "/api/v1/secrets/ghost", nil, true)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// TC-IT-10: Update returns 200 with new value.
func TestSecretHandler_Update_200(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	createSecret(r, "update-key", "Update Key", "old-value")

	w := doRequest(r, http.MethodPut, "/api/v1/secrets/update-key", map[string]string{"value": "new-value"}, true)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

}

// TC-IT-10b: PUT on a DEPRECATED secret reactivates it (status → ACTIVE) and re-encrypts the value.
func TestSecretHandler_Update_ReactivatesDeprecatedSecret(t *testing.T) {
	tmpDir := t.TempDir()
	sqlDB, err := sql.Open("sqlite3", filepath.Join(tmpDir, "test-reactivate.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.Exec("PRAGMA foreign_keys = ON")
	db := &database.DB{DB: sqlDB}

	schema, _ := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	db.Exec(string(schema))
	db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, created_at, updated_at)
		VALUES ('org-react-it', 'org-react', 'Org React', 'default', datetime('now'), datetime('now'))`)

	v, _ := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	repo := repository.NewSecretRepo(db)
	svc := service.NewSecretService(repo, v)

	mux := http.NewServeMux()
	NewSecretHandler(svc, slog.Default()).RegisterRoutes(mux)
	r := middleware.NewTestContextMiddleware(mux)

	// Create then soft-delete (deprecate) the secret.
	body, ct := multipartForm(map[string]string{"handle": "react-key", "name": "React Key", "value": "old-val"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/secrets", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-Test-Org", "org-react-it")
	req.Header.Set("X-Test-User", "alice")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", w.Code)
	}

	delReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/secrets/react-key", nil)
	delReq.Header.Set("X-Test-Org", "org-react-it")
	delReq.Header.Set("X-Test-User", "alice")
	wDel := httptest.NewRecorder()
	r.ServeHTTP(wDel, delReq)
	if wDel.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", wDel.Code)
	}

	// Confirm it is DEPRECATED before rotation.
	var statusBefore string
	sqlDB.QueryRow(`SELECT status FROM secrets WHERE handle = 'react-key'`).Scan(&statusBefore)
	if statusBefore != "DEPRECATED" {
		t.Fatalf("expected DEPRECATED before rotation, got %s", statusBefore)
	}

	// Rotate — PUT should reactivate.
	putBody, putCT := multipartForm(map[string]string{"value": "new-val"})
	putReq, _ := http.NewRequest(http.MethodPut, "/api/v1/secrets/react-key", putBody)
	putReq.Header.Set("Content-Type", putCT)
	putReq.Header.Set("X-Test-Org", "org-react-it")
	putReq.Header.Set("X-Test-User", "alice")
	wPut := httptest.NewRecorder()
	r.ServeHTTP(wPut, putReq)
	if wPut.Code != http.StatusOK {
		t.Fatalf("rotate: expected 200, got %d: %s", wPut.Code, wPut.Body.String())
	}

	// Status must be ACTIVE and value must decrypt to the new plaintext.
	var statusAfter string
	sqlDB.QueryRow(`SELECT status FROM secrets WHERE handle = 'react-key'`).Scan(&statusAfter)
	if statusAfter != "ACTIVE" {
		t.Errorf("expected ACTIVE after rotation, got %s", statusAfter)
	}

	plaintext, err := svc.Decrypt("org-react-it", "react-key")
	if err != nil {
		t.Fatalf("Decrypt after reactivation: %v", err)
	}
	if plaintext != "new-val" {
		t.Errorf("expected plaintext=new-val, got %s", plaintext)
	}
}

// TC-IT-11: Delete unreferenced secret returns 204.
func TestSecretHandler_Delete_204_Unreferenced(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	createSecret(r, "del-key", "Del Key", "val")

	w := doRequest(r, http.MethodDelete, "/api/v1/secrets/del-key", nil, true)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

// TC-IT-12: Delete a secret referenced by an artifact returns 409.
func TestSecretHandler_Delete_409_ReferencedByArtifact(t *testing.T) {
	// Need access to the DB to insert artifact and ref directly.
	// We set up a separate env with direct DB access.
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-ref.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer sqlDB.Close()

	sqlDB.Exec("PRAGMA foreign_keys = ON")
	db := &database.DB{DB: sqlDB}

	schemaPath := filepath.Join("..", "database", "schema.sqlite.sql")
	schema, _ := os.ReadFile(schemaPath)
	db.Exec(string(schema))

	db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, created_at, updated_at)
		VALUES ('org-it-001', 'test-org', 'Test Org', 'default', datetime('now'), datetime('now'))`)

	// Insert a project (required by artifacts via rest_apis)
	db.Exec(`INSERT INTO projects (uuid, handle, display_name, organization_uuid, created_at, updated_at)
		VALUES ('proj-001', 'test-proj', 'Test Project', 'org-it-001', datetime('now'), datetime('now'))`)

	// Insert an artifact
	db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid)
		VALUES ('art-001', 'RestApi', 'org-it-001')`)

	v, _ := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	repo := repository.NewSecretRepo(db)
	svc := service.NewSecretService(repo, v)
	h := NewSecretHandler(svc, slog.Default())

	mux2 := http.NewServeMux()
	h.RegisterRoutes(mux2)
	r2 := middleware.NewTestContextMiddleware(mux2)

	// Create the secret via the handler
	w := createSecretOnRouter(r2, "ref-key", "Ref Key", "val")
	if w.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", w.Code, w.Body.String())
	}

	// Insert the artifact and artifact_secret_ref row
	if _, err = db.Exec(`INSERT OR IGNORE INTO artifacts (uuid, type, organization_uuid) VALUES ('art-001', 'RestApi', 'org-it-001')`); err != nil {
		t.Fatalf("failed to insert artifact: %v", err)
	}
	_, err = db.Exec(`INSERT INTO artifact_secret_refs (organization_uuid, artifact_uuid, secret_handle, gateway_id)
		VALUES ('org-it-001', 'art-001', 'ref-key', '')`)
	if err != nil {
		t.Fatalf("failed to insert artifact_secret_ref: %v", err)
	}

	// Now delete should return 409
	wDel := doRequest(r2, http.MethodDelete, "/api/v1/secrets/ref-key", nil, true)
	if wDel.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", wDel.Code, wDel.Body.String())
	}

	resp := parseBody(wDel)
	refs, ok := resp["references"].([]interface{})
	if !ok || len(refs) == 0 {
		t.Errorf("expected non-empty references array, got: %v", resp)
	}
}

// doRequestAs is like doRequest but targets a specific org with no body.
func doRequestAs(r http.Handler, method, path, orgID string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, path, &bytes.Buffer{})
	req.Header.Set("X-Test-Org", orgID)
	req.Header.Set("X-Test-User", "alice")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// createSecretOnRouter is like createSecret but uses a provided router.
func createSecretOnRouter(r http.Handler, handle, displayName, value string) *httptest.ResponseRecorder {
	body, ct := multipartForm(map[string]string{
		"handle":      handle,
		"name":        displayName,
		"value":       value,
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/secrets", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-Test-Org", "org-it-001")
	req.Header.Set("X-Test-User", "alice")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TC-IT-13: Delete non-existent secret returns 404.
func TestSecretHandler_Delete_404_NotFound(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	w := doRequest(r, http.MethodDelete, "/api/v1/secrets/ghost", nil, true)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// TC-IT-10: Org B cannot see org A's secrets — list returns empty.
func TestSecretHandler_List_DifferentOrg_Empty(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	// Create secret as org A
	w := createSecret(r, "org-a-secret", "Org A Secret", "val")
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// List as org B — must return empty list, not org A's secret
	w = doRequestAs(r, http.MethodGet, "/api/v1/secrets", "org-it-002")
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	resp := parseBody(w)
	list, ok := resp["list"].([]interface{})
	if !ok {
		t.Fatal("expected list array")
	}
	if len(list) != 0 {
		t.Errorf("org B should see 0 secrets, got %d", len(list))
	}

	pagination, ok := resp["pagination"].(map[string]interface{})
	if !ok {
		t.Fatal("expected pagination object")
	}
	if pagination["total"] != float64(0) {
		t.Errorf("pagination.total = %v, want 0", pagination["total"])
	}
}

// TC-IT-13: Get secret from different org returns 404 — no existence leak.
func TestSecretHandler_Get_DifferentOrg_404(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	// Create secret as org A
	w := createSecret(r, "org-a-only", "Org A Only", "val")
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Attempt to GET that secret as org B — must get 404, not 403
	w = doRequestAs(r, http.MethodGet, "/api/v1/secrets/org-a-only", "org-it-002")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 (no existence leak), got %d: %s", w.Code, w.Body.String())
	}
}

// TC-IT-14: List response items should not have a value key.
func TestSecretHandler_Create_ValueNotInListResponse(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	createSecret(r, "no-val-key", "No Val Key", "secret123")

	w := doRequest(r, http.MethodGet, "/api/v1/secrets", nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := parseBody(w)
	list, ok := resp["list"].([]interface{})
	if !ok {
		t.Fatal("expected list array")
	}

	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if _, hasValue := m["value"]; hasValue {
			t.Errorf("list item should NOT contain value field, got: %v", m)
		}
	}
}

// TC-60: DELETE soft-deletes — status becomes DEPRECATED, physical row is retained.
// Verified by: (a) 204 on first delete, (b) Exists() returns false (ACTIVE filter),
// (c) the DB row still exists with status=DEPRECATED (checked via direct SQL),
// (d) Decrypt returns "secret is deprecated" — not "not found".
func TestSecretHandler_Delete_SoftDeletesRow(t *testing.T) {
	// Build isolated stack so we can access the DB and service directly.
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-sd.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.Exec("PRAGMA foreign_keys = ON")
	db := &database.DB{DB: sqlDB}

	schema, _ := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	db.Exec(string(schema))
	db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, created_at, updated_at)
		VALUES ('org-sd-it', 'org-sd', 'Org SD', 'default', datetime('now'), datetime('now'))`)

	v, _ := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	repo := repository.NewSecretRepo(db)
	svc := service.NewSecretService(repo, v)

	mux3 := http.NewServeMux()
	NewSecretHandler(svc, slog.Default()).RegisterRoutes(mux3)
	r := middleware.NewTestContextMiddleware(mux3)

	// (a) Create and DELETE → 204
	createBody, createCT := multipartForm(map[string]string{
		"handle": "soft-del-key", "name": "Soft Del Key", "value": "plaintext",
	})
	createReq, _ := http.NewRequest(http.MethodPost, "/api/v1/secrets", createBody)
	createReq.Header.Set("Content-Type", createCT)
	createReq.Header.Set("X-Test-Org", "org-sd-it")
	createReq.Header.Set("X-Test-User", "alice")
	wCreate := httptest.NewRecorder()
	r.ServeHTTP(wCreate, createReq)
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", wCreate.Code)
	}

	delReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/secrets/soft-del-key", nil)
	delReq.Header.Set("X-Test-Org", "org-sd-it")
	delReq.Header.Set("X-Test-User", "alice")
	wDel := httptest.NewRecorder()
	r.ServeHTTP(wDel, delReq)
	if wDel.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", wDel.Code)
	}

	// (b) Exists() returns false — secret no longer ACTIVE
	exists, err := repo.Exists("org-sd-it", "soft-del-key")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("Exists should return false after soft-delete")
	}

	// (c) Physical row still present with status=DEPRECATED
	var status string
	err = sqlDB.QueryRow(
		`SELECT status FROM secrets WHERE organization_uuid = ? AND handle = ?`,
		"org-sd-it", "soft-del-key",
	).Scan(&status)
	if err != nil {
		t.Fatalf("DB row missing after soft-delete: %v", err)
	}
	if status != "DEPRECATED" {
		t.Errorf("expected status DEPRECATED, got %q", status)
	}

	// (d) Decrypt returns "secret is deprecated", not "not found"
	_, decryptErr := svc.Decrypt("org-sd-it", "soft-del-key")
	if decryptErr == nil {
		t.Fatal("expected error decrypting DEPRECATED secret")
	}
	if decryptErr.Error() != "secret is deprecated" {
		t.Errorf("expected 'secret is deprecated', got %q", decryptErr.Error())
	}
}

// TC-61: DEPRECATED secret — Decrypt returns error (platform API contract the GW
// controller relies on to skip /value calls for DEPRECATED items).
func TestSecretService_Decrypt_DeprecatedSecretReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-dep.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.Exec("PRAGMA foreign_keys = ON")
	db := &database.DB{DB: sqlDB}

	schema, _ := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	db.Exec(string(schema))
	db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, created_at, updated_at)
		VALUES ('org-dep-it', 'org-dep', 'Org Dep', 'default', datetime('now'), datetime('now'))`)

	v, _ := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	repo := repository.NewSecretRepo(db)
	svc := service.NewSecretService(repo, v)

	// Create and then soft-delete a secret
	_, err = svc.Create("org-dep-it", "alice", &dto.CreateSecretRequest{
		Handle: "dep-secret",
		Value:  "plaintext",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err = svc.Delete("org-dep-it", "dep-secret", "alice"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Decrypt must return an error for DEPRECATED secret — not the plaintext
	_, err = svc.Decrypt("org-dep-it", "dep-secret")
	if err == nil {
		t.Fatal("expected error decrypting DEPRECATED secret, got nil")
	}
	if err.Error() != "secret is deprecated" {
		t.Errorf("expected 'secret is deprecated', got %q", err.Error())
	}
}

// TC-34: Ciphertext stored in DB is not the plaintext value.
func TestSecretRepo_CiphertextStoredNotPlaintext(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-ct.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.Exec("PRAGMA foreign_keys = ON")
	db := &database.DB{DB: sqlDB}

	schema, _ := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	db.Exec(string(schema))
	db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, created_at, updated_at)
		VALUES ('org-ct-001', 'org-ct', 'Org CT', 'default', datetime('now'), datetime('now'))`)

	v, _ := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	repo := repository.NewSecretRepo(db)
	svc := service.NewSecretService(repo, v)

	_, err = svc.Create("org-ct-001", "alice", &dto.CreateSecretRequest{Handle: "ct-key", Value: "sk-abc123"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var ciphertext []byte
	err = sqlDB.QueryRow(`SELECT ciphertext FROM secrets WHERE handle = ?`, "ct-key").Scan(&ciphertext)
	if err != nil {
		t.Fatalf("query ciphertext: %v", err)
	}
	if len(ciphertext) == 0 {
		t.Error("ciphertext should be non-empty")
	}
	if string(ciphertext) == "sk-abc123" {
		t.Error("ciphertext should NOT equal the plaintext value")
	}
}

// TC-36: Provider stored in DB is IN_BUILT.
func TestSecretRepo_ProviderIsInBuilt(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-prov.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.Exec("PRAGMA foreign_keys = ON")
	db := &database.DB{DB: sqlDB}

	schema, _ := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	db.Exec(string(schema))
	db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, created_at, updated_at)
		VALUES ('org-prov-001', 'org-prov', 'Org Prov', 'default', datetime('now'), datetime('now'))`)

	v, _ := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	repo := repository.NewSecretRepo(db)
	svc := service.NewSecretService(repo, v)

	_, err = svc.Create("org-prov-001", "alice", &dto.CreateSecretRequest{Handle: "prov-key", Value: "somevalue"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var provider string
	err = sqlDB.QueryRow(`SELECT provider FROM secrets WHERE handle = ?`, "prov-key").Scan(&provider)
	if err != nil {
		t.Fatalf("query provider: %v", err)
	}
	if provider != "IN_BUILT" {
		t.Errorf("expected provider=IN_BUILT, got %q", provider)
	}
}

// TC-37: Decrypt returns original plaintext after Create.
func TestSecretService_DecryptReturnsOriginalPlaintext(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-dec.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.Exec("PRAGMA foreign_keys = ON")
	db := &database.DB{DB: sqlDB}

	schema, _ := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	db.Exec(string(schema))
	db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, created_at, updated_at)
		VALUES ('org-dec-001', 'org-dec', 'Org Dec', 'default', datetime('now'), datetime('now'))`)

	v, _ := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	repo := repository.NewSecretRepo(db)
	svc := service.NewSecretService(repo, v)

	_, err = svc.Create("org-dec-001", "alice", &dto.CreateSecretRequest{Handle: "dec-key", Value: "sk-original"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	plaintext, err := svc.Decrypt("org-dec-001", "dec-key")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plaintext != "sk-original" {
		t.Errorf("expected plaintext=sk-original, got %q", plaintext)
	}
}

// TC-62: Creating a resource referencing a DEPRECATED secret handle is rejected at
// validation time (ValidateSecretRefs returns ErrSecretRefMissing for DEPRECATED).
func TestSecretService_ValidateSecretRefs_DeprecatedHandleRejected(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-val.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.Exec("PRAGMA foreign_keys = ON")
	db := &database.DB{DB: sqlDB}

	schema, _ := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	db.Exec(string(schema))
	db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, created_at, updated_at)
		VALUES ('org-val-it', 'org-val', 'Org Val', 'default', datetime('now'), datetime('now'))`)

	v, _ := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	repo := repository.NewSecretRepo(db)
	svc := service.NewSecretService(repo, v)

	// Create then deprecate the secret
	_, err = svc.Create("org-val-it", "alice", &dto.CreateSecretRequest{
		Handle: "wso2-openai-key",
		Value:  "sk-abc",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err = svc.Delete("org-val-it", "wso2-openai-key", "alice"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Validate a config that references the now-DEPRECATED handle — must be rejected
	config := `{{ secret "wso2-openai-key" }}`
	err = svc.ValidateSecretRefs("org-val-it", config)
	if err == nil {
		t.Fatal("expected validation error for DEPRECATED secret ref, got nil")
	}
	if !errors.Is(err, constants.ErrSecretRefMissing) {
		t.Errorf("expected ErrSecretRefMissing, got %v", err)
	}
}

// TestSecretHandler_List_LimitCappedAt100 verifies that requesting limit=999 is
// silently capped to 100 (scenario 89).
func TestSecretHandler_List_LimitCappedAt100(t *testing.T) {
	r, cleanup := setupSecretTestEnv(t)
	defer cleanup()

	w := doRequest(r, http.MethodGet, "/api/v1/secrets?limit=999", nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseBody(w)
	pagination, ok := resp["pagination"].(map[string]interface{})
	if !ok {
		t.Fatalf("pagination field missing or wrong type: %v", resp["pagination"])
	}
	if int(pagination["limit"].(float64)) != 100 {
		t.Errorf("expected limit capped at 100, got %v", pagination["limit"])
	}
}

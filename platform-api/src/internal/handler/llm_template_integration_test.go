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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"platform-api/src/internal/database"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	_ "github.com/mattn/go-sqlite3"
)

const tmplBase = "/api/v0.9/llm-provider-templates"
const tmplOrg = "org-it-001"

type noopAudit struct{}

func (noopAudit) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	return nil
}

// setupLLMTemplateEnv builds the real route → handler → service → repository
// stack over an in-memory SQLite DB, seeded with the shipped built-in templates.
func setupLLMTemplateEnv(t *testing.T) (http.Handler, *database.DB, func()) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
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
	if _, err = db.Exec(`INSERT INTO organizations (uuid, handle, name, region, created_at, updated_at)
		VALUES ('` + tmplOrg + `', 'test-org', 'Test Org', 'default', datetime('now'), datetime('now'))`); err != nil {
		t.Fatalf("insert org: %v", err)
	}

	repo := repository.NewLLMProviderTemplateRepo(db)

	// Seed the shipped built-in (wso2) templates so we can exercise the
	// built-in-only rules (toggle allowed, update/delete read-only).
	builtins, err := utils.LoadLLMProviderTemplatesFromDirectory(
		filepath.Join("..", "..", "resources", "default-llm-provider-templates"))
	if err != nil {
		t.Fatalf("load built-ins: %v", err)
	}
	if err := service.NewLLMTemplateSeeder(repo, builtins).SeedForOrg(tmplOrg); err != nil {
		t.Fatalf("seed built-ins: %v", err)
	}

	svc := service.NewLLMProviderTemplateService(repo, noopAudit{})
	h := NewLLMHandler(svc, nil, nil, slog.Default())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	return middleware.NewTestContextMiddleware(mux), db, func() { _ = sqlDB.Close() }
}

func doJSON(t *testing.T, r http.Handler, method, path, body string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("X-Test-Org", tmplOrg)
		req.Header.Set("X-Test-User", "alice")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func bodyMap(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return m
}

// queryPath builds the URL-encoded ?query=groupId:<gid> collection path.
func queryPath(gid string) string {
	return tmplBase + "?query=" + url.QueryEscape("groupId:"+gid)
}

// createFamily POSTs a new custom family and returns its handle + groupId.
func createFamily(t *testing.T, r http.Handler, name string) (handle, groupID string) {
	t.Helper()
	body := `{"name":"` + name + `","managedBy":"customer","metadata":{"endpointUrl":"https://api.example.com"}}`
	w := doJSON(t, r, http.MethodPost, tmplBase, body, true)
	if w.Code != http.StatusCreated {
		t.Fatalf("create family %q: expected 201, got %d: %s", name, w.Code, w.Body.String())
	}
	m := bodyMap(t, w)
	handle, _ = m["id"].(string)
	groupID, _ = m["groupId"].(string)
	if handle == "" || groupID == "" {
		t.Fatalf("create family %q: missing id/groupId in %v", name, m)
	}
	return handle, groupID
}

// ---- Auth -----------------------------------------------------------------

func TestLLMTemplateHTTP_ListRequiresOrg_401(t *testing.T) {
	r, _, cleanup := setupLLMTemplateEnv(t)
	defer cleanup()

	if w := doJSON(t, r, http.MethodGet, tmplBase, "", false); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without org, got %d", w.Code)
	}
}

// ---- Create (happy + errors) ----------------------------------------------

func TestLLMTemplateHTTP_CreateFamily_Errors(t *testing.T) {
	r, _, cleanup := setupLLMTemplateEnv(t)
	defer cleanup()

	// Missing endpointUrl -> 400.
	w := doJSON(t, r, http.MethodPost, tmplBase, `{"name":"No Endpoint","managedBy":"customer"}`, true)
	if w.Code != http.StatusBadRequest {
		t.Errorf("missing endpoint: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// Reserved managedBy=wso2 -> 400.
	w = doJSON(t, r, http.MethodPost, tmplBase,
		`{"name":"Reserved","managedBy":"wso2","metadata":{"endpointUrl":"https://x"}}`, true)
	if w.Code != http.StatusBadRequest {
		t.Errorf("reserved managedBy: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// Duplicate handle -> 409.
	createFamily(t, r, "Dup Family")
	w = doJSON(t, r, http.MethodPost, tmplBase,
		`{"name":"Dup Family","managedBy":"customer","metadata":{"endpointUrl":"https://x"}}`, true)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// ---- Get by handle --------------------------------------------------------

func TestLLMTemplateHTTP_GetByHandle(t *testing.T) {
	r, _, cleanup := setupLLMTemplateEnv(t)
	defer cleanup()

	handle, groupID := createFamily(t, r, "Get Me")

	w := doJSON(t, r, http.MethodGet, tmplBase+"/"+handle, "", true)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	m := bodyMap(t, w)
	if m["version"] != "v1.0" {
		t.Errorf("expected version v1.0, got %v", m["version"])
	}
	if m["groupId"] != groupID {
		t.Errorf("expected groupId %q, got %v", groupID, m["groupId"])
	}

	// Unknown handle -> 404.
	if w := doJSON(t, r, http.MethodGet, tmplBase+"/does-not-exist", "", true); w.Code != http.StatusNotFound {
		t.Errorf("unknown handle: expected 404, got %d", w.Code)
	}
}

// ---- Versions: create + list via ?query=groupId ---------------------------

func TestLLMTemplateHTTP_Versions(t *testing.T) {
	r, _, cleanup := setupLLMTemplateEnv(t)
	defer cleanup()

	_, groupID := createFamily(t, r, "Versioned")

	// Create v2.0 in the family.
	v2 := `{"name":"Versioned","version":"v2.0","metadata":{"endpointUrl":"https://api.example.com"}}`
	if w := doJSON(t, r, http.MethodPost, queryPath(groupID), v2, true); w.Code != http.StatusCreated {
		t.Fatalf("create v2.0: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// List versions -> 2 rows.
	w := doJSON(t, r, http.MethodGet, queryPath(groupID), "", true)
	if w.Code != http.StatusOK {
		t.Fatalf("list versions: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	m := bodyMap(t, w)
	if list, ok := m["list"].([]any); !ok || len(list) != 2 {
		t.Errorf("expected 2 versions, got %v", m["list"])
	}

	// Duplicate version -> 409.
	if w := doJSON(t, r, http.MethodPost, queryPath(groupID), v2, true); w.Code != http.StatusConflict {
		t.Errorf("duplicate version: expected 409, got %d: %s", w.Code, w.Body.String())
	}

	// Invalid version format -> 400.
	bad := `{"name":"Versioned","version":"2.0","metadata":{"endpointUrl":"https://x"}}`
	if w := doJSON(t, r, http.MethodPost, queryPath(groupID), bad, true); w.Code != http.StatusBadRequest {
		t.Errorf("invalid version: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// Version in a non-existent family -> 404.
	if w := doJSON(t, r, http.MethodPost, queryPath("no-such-family"), v2, true); w.Code != http.StatusNotFound {
		t.Errorf("version for missing family: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// List versions of a non-existent family -> 404.
	if w := doJSON(t, r, http.MethodGet, queryPath("no-such-family"), "", true); w.Code != http.StatusNotFound {
		t.Errorf("list missing family: expected 404, got %d", w.Code)
	}
}

// ---- Malformed ?query=groupId: (present but blank) ------------------------

func TestLLMTemplateHTTP_BlankGroupIDQuery(t *testing.T) {
	r, _, cleanup := setupLLMTemplateEnv(t)
	defer cleanup()

	blank := tmplBase + "?query=" + url.QueryEscape("groupId:")

	// POST with a blank groupId must NOT create a new family.
	body := `{"name":"Should Not Create","version":"v1.0","metadata":{"endpointUrl":"https://api.example.com"}}`
	if w := doJSON(t, r, http.MethodPost, blank, body, true); w.Code != http.StatusBadRequest {
		t.Errorf("POST blank groupId: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// GET with a blank groupId must NOT list the whole collection.
	if w := doJSON(t, r, http.MethodGet, blank, "", true); w.Code != http.StatusBadRequest {
		t.Errorf("GET blank groupId: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ---- PATCH enable/disable by handle (built-in only) -----------------------

func TestLLMTemplateHTTP_ToggleByHandle(t *testing.T) {
	r, _, cleanup := setupLLMTemplateEnv(t)
	defer cleanup()

	// Built-in openai is toggleable.
	w := doJSON(t, r, http.MethodPatch, tmplBase+"/openai", `{"enabled":false}`, true)
	if w.Code != http.StatusOK {
		t.Fatalf("disable built-in: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if bodyMap(t, w)["enabled"] != false {
		t.Errorf("expected enabled=false after disable")
	}
	if w := doJSON(t, r, http.MethodPatch, tmplBase+"/openai", `{"enabled":true}`, true); w.Code != http.StatusOK {
		t.Errorf("re-enable built-in: expected 200, got %d", w.Code)
	}

	// Custom template cannot be toggled -> 403.
	handle, _ := createFamily(t, r, "Custom Toggle")
	if w := doJSON(t, r, http.MethodPatch, tmplBase+"/"+handle, `{"enabled":false}`, true); w.Code != http.StatusForbidden {
		t.Errorf("toggle custom: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	// Unknown handle -> 404.
	if w := doJSON(t, r, http.MethodPatch, tmplBase+"/nope", `{"enabled":false}`, true); w.Code != http.StatusNotFound {
		t.Errorf("toggle unknown: expected 404, got %d", w.Code)
	}

	// Missing "enabled" field -> 400.
	if w := doJSON(t, r, http.MethodPatch, tmplBase+"/openai", `{}`, true); w.Code != http.StatusBadRequest {
		t.Errorf("missing enabled: expected 400, got %d", w.Code)
	}
}

// ---- PUT update (built-in read-only) --------------------------------------

func TestLLMTemplateHTTP_UpdateReadOnlyBuiltin(t *testing.T) {
	r, _, cleanup := setupLLMTemplateEnv(t)
	defer cleanup()

	body := `{"id":"openai","name":"Hacked","version":"v1.0","metadata":{"endpointUrl":"https://x"}}`
	if w := doJSON(t, r, http.MethodPut, tmplBase+"/openai", body, true); w.Code != http.StatusForbidden {
		t.Errorf("update built-in: expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// ---- DELETE by handle (happy + read-only + in-use + unknown) --------------

func TestLLMTemplateHTTP_DeleteByHandle(t *testing.T) {
	r, db, cleanup := setupLLMTemplateEnv(t)
	defer cleanup()

	// Custom family + a second version; delete the second version.
	_, groupID := createFamily(t, r, "Delete Me")
	v2 := `{"name":"Delete Me","version":"v2.0","metadata":{"endpointUrl":"https://x"}}`
	w := doJSON(t, r, http.MethodPost, queryPath(groupID), v2, true)
	if w.Code != http.StatusCreated {
		t.Fatalf("create v2.0: got %d: %s", w.Code, w.Body.String())
	}
	v2Handle, _ := bodyMap(t, w)["id"].(string)

	if w := doJSON(t, r, http.MethodDelete, tmplBase+"/"+v2Handle, "", true); w.Code != http.StatusNoContent {
		t.Fatalf("delete v2.0: expected 204, got %d: %s", w.Code, w.Body.String())
	}
	// Now gone -> 404.
	if w := doJSON(t, r, http.MethodDelete, tmplBase+"/"+v2Handle, "", true); w.Code != http.StatusNotFound {
		t.Errorf("delete again: expected 404, got %d", w.Code)
	}

	// Built-in version is read-only -> 403.
	if w := doJSON(t, r, http.MethodDelete, tmplBase+"/openai", "", true); w.Code != http.StatusForbidden {
		t.Errorf("delete built-in: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	// In-use version cannot be deleted -> 409.
	inUseHandle, _ := createFamily(t, r, "In Use")
	var tuuid string
	if err := db.QueryRow(
		`SELECT uuid FROM llm_provider_templates WHERE handle = ? AND organization_uuid = ?`,
		inUseHandle, tmplOrg).Scan(&tuuid); err != nil {
		t.Fatalf("lookup template uuid: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO llm_providers (uuid, handle, name, version, template_uuid, configuration, organization_uuid)
		 VALUES ('prov-inuse', 'in-use-prov', 'In Use Prov', 'v1.0', ?, '{}', ?)`,
		tuuid, tmplOrg); err != nil {
		t.Fatalf("insert provider: %v", err)
	}
	if w := doJSON(t, r, http.MethodDelete, tmplBase+"/"+inUseHandle, "", true); w.Code != http.StatusConflict {
		t.Errorf("delete in-use: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

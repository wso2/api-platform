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
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/platform-api/internal/vault"

	_ "github.com/mattn/go-sqlite3"
)

const provBase = "/api/v0.9/llm-providers"
const provOrg = "org-prov-it-001"

// setupLLMProviderEnv builds the real route -> handler -> service -> repository
// stack over an in-memory SQLite DB, seeded with the shipped built-in templates
// and a test organization.
func setupLLMProviderEnv(t *testing.T) (http.Handler, func()) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB.Exec("PRAGMA foreign_keys = ON")
	db := &database.DB{DB: sqlDB}

	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	if _, err = db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES ('` + provOrg + `', 'test-prov-org', 'Test Prov Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`); err != nil {
		t.Fatalf("insert org: %v", err)
	}

	templateRepo := repository.NewLLMProviderTemplateRepo(db)
	providerRepo := repository.NewLLMProviderRepo(db)
	orgRepo := repository.NewOrganizationRepo(db)

	builtins, err := utils.LoadLLMProviderTemplatesFromDirectory(
		filepath.Join("..", "..", "resources", "default-llm-provider-templates"))
	if err != nil {
		t.Fatalf("load built-ins: %v", err)
	}
	seeder := service.NewLLMTemplateSeeder(templateRepo, builtins)
	if err := seeder.SeedForOrg(provOrg); err != nil {
		t.Fatalf("seed built-ins: %v", err)
	}

	identityService := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	cfg := &config.Server{}

	providerService := service.NewLLMProviderService(
		providerRepo, templateRepo, orgRepo, seeder,
		nil, nil, nil, // deploymentRepo, gatewayRepo, gatewayEventsService: unused by Create in these tests
		slog.Default(), noopAudit{}, cfg, identityService,
	)

	secretRepo := repository.NewSecretRepo(db)
	v, err := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	secretService := service.NewSecretService(secretRepo, v, identityService)
	providerService.SetSecretService(secretService)

	h := NewLLMHandler(nil, providerService, nil, identityService, slog.Default())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	return middleware.NewTestContextMiddleware(mux), func() { _ = sqlDB.Close() }
}

// doProviderJSON issues an HTTP request against r, scoped to provOrg (auth=true
// sets X-Test-Org/X-Test-User; auth=false omits them to exercise the 401 path).
// Self-contained rather than reusing llm_template_integration_test.go's doJSON,
// which hardcodes a different org constant.
func doProviderJSON(t *testing.T, r http.Handler, method, path, body string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("X-Test-Org", provOrg)
		req.Header.Set("X-Test-User", "alice")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// validProviderBody returns a minimal, valid create-provider JSON body backed
// by the built-in "openai" template. id is left auto-generated unless idSuffix
// is non-empty, so successive calls in the same test don't collide on handle.
func validProviderBody(idSuffix string) string {
	name := "Test Provider"
	if idSuffix != "" {
		name = "Test Provider " + idSuffix
	}
	return fmt.Sprintf(`{
		"displayName": %q,
		"version": "v1.0",
		"template": "openai",
		"upstream": {"main": {"url": "https://api.openai.com/v1"}},
		"accessControl": {"mode": "allow_all"}
	}`, name)
}

// ---- Auth -------------------------------------------------------------

func TestLLMProviderHTTP_CreateRequiresOrg_401(t *testing.T) {
	r, cleanup := setupLLMProviderEnv(t)
	defer cleanup()

	w := doProviderJSON(t, r, http.MethodPost, provBase, validProviderBody(""), false)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without org, got %d: %s", w.Code, w.Body.String())
	}
	body := bodyMap(t, w)
	if body["status"] != "error" {
		t.Errorf("expected status=error, got %v", body["status"])
	}
	if body["code"] != "UNAUTHORIZED" {
		t.Errorf("expected code=UNAUTHORIZED, got %v", body["code"])
	}
}

// ---- Create: validation errors -----------------------------------------

func TestLLMProviderHTTP_Create_InvalidBody_400(t *testing.T) {
	r, cleanup := setupLLMProviderEnv(t)
	defer cleanup()

	w := doProviderJSON(t, r, http.MethodPost, provBase, `not-json`, true)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLLMProviderHTTP_Create_MissingRequiredFields_400(t *testing.T) {
	r, cleanup := setupLLMProviderEnv(t)
	defer cleanup()

	// Missing displayName/version/template entirely.
	w := doProviderJSON(t, r, http.MethodPost, provBase, `{"upstream":{"main":{"url":"https://x"}},"accessControl":{"mode":"allow_all"}}`, true)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing required fields: expected 400, got %d: %s", w.Code, w.Body.String())
	}
	body := bodyMap(t, w)
	if body["status"] != "error" || body["code"] == nil || body["message"] == nil {
		t.Errorf("expected standard error shape, got %v", body)
	}
}

func TestLLMProviderHTTP_Create_UnknownTemplate_400(t *testing.T) {
	r, cleanup := setupLLMProviderEnv(t)
	defer cleanup()

	body := `{
		"displayName": "Bad Template Provider",
		"version": "v1.0",
		"template": "does-not-exist",
		"upstream": {"main": {"url": "https://api.example.com"}},
		"accessControl": {"mode": "allow_all"}
	}`
	w := doProviderJSON(t, r, http.MethodPost, provBase, body, true)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown template: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ---- Create: conflicts --------------------------------------------------

func TestLLMProviderHTTP_Create_DuplicateHandle_409(t *testing.T) {
	r, cleanup := setupLLMProviderEnv(t)
	defer cleanup()

	body := `{
		"id": "dup-provider",
		"displayName": "Dup Provider",
		"version": "v1.0",
		"template": "openai",
		"upstream": {"main": {"url": "https://api.openai.com/v1"}},
		"accessControl": {"mode": "allow_all"}
	}`
	if w := doProviderJSON(t, r, http.MethodPost, provBase, body, true); w.Code != http.StatusCreated {
		t.Fatalf("first create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	w := doProviderJSON(t, r, http.MethodPost, provBase, body, true)
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate handle: expected 409, got %d: %s", w.Code, w.Body.String())
	}
	respBody := bodyMap(t, w)
	if respBody["status"] != "error" {
		t.Errorf("expected status=error, got %v", respBody["status"])
	}
}

// ---- Create: secret placeholder validation ------------------------------

func TestLLMProviderHTTP_Create_MissingSecretRef_400(t *testing.T) {
	r, cleanup := setupLLMProviderEnv(t)
	defer cleanup()

	body := `{
		"displayName": "Secret Ref Provider",
		"version": "v1.0",
		"template": "openai",
		"upstream": {
			"main": {
				"url": "https://api.openai.com/v1",
				"auth": {"type": "api-key", "header": "Authorization", "value": "{{ secret \"does-not-exist\" }}"}
			}
		},
		"accessControl": {"mode": "allow_all"}
	}`
	w := doProviderJSON(t, r, http.MethodPost, provBase, body, true)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing secret ref: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ---- Create: happy path (sanity check for the error tests above) -------

func TestLLMProviderHTTP_Create_Success(t *testing.T) {
	r, cleanup := setupLLMProviderEnv(t)
	defer cleanup()

	w := doProviderJSON(t, r, http.MethodPost, provBase, validProviderBody(""), true)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	body := bodyMap(t, w)
	if body["displayName"] != "Test Provider" {
		t.Errorf("expected displayName echoed back, got %v", body["displayName"])
	}
}

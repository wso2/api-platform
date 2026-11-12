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
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

func TestFormScopes(t *testing.T) {
	multipartReq := func(values ...string) *http.Request {
		buf := &bytes.Buffer{}
		w := multipart.NewWriter(buf)
		_ = w.WriteField("value", "v")
		for _, v := range values {
			_ = w.WriteField("scopes", v)
		}
		w.Close()
		r := httptest.NewRequest(http.MethodPost, "/", buf)
		r.Header.Set("Content-Type", w.FormDataContentType())
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		return r
	}

	got, present := formScopes(multipartReq())
	if present || got != nil {
		t.Errorf("absent: got %v, %v", got, present)
	}

	got, present = formScopes(multipartReq("project:payments", " artifact:RestApi:orders , org"))
	want := []string{"project:payments", "artifact:RestApi:orders", "org"}
	if !present || !reflect.DeepEqual(got, want) {
		t.Errorf("multipart: got %v, %v; want %v", got, present, want)
	}

	got, present = formScopes(multipartReq(""))
	if !present || len(got) != 0 {
		t.Errorf("present but empty: got %v, %v", got, present)
	}

	form := url.Values{"scopes": {"project:a,project:b"}}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = r.ParseForm()
	got, present = formScopes(r)
	if !present || !reflect.DeepEqual(got, []string{"project:a", "project:b"}) {
		t.Errorf("urlencoded: got %v, %v", got, present)
	}
}

// setupScopedSecretTestEnv is setupSecretTestEnv with the scope resolver wired
// as the server wires it, plus projects "payments" (holding REST API "orders")
// and "billing". REST API "orders" references the secret handle "in-use".
func setupScopedSecretTestEnv(t *testing.T) http.Handler {
	t.Helper()

	sqlDB, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if _, err = sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}
	db := &database.DB{DB: sqlDB}
	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	if err != nil {
		t.Fatalf("failed to read schema: %v", err)
	}
	for _, stmt := range []string{
		string(schema),
		`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
			VALUES ('org-it-001', 'test-org', 'Test Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`,
		`INSERT INTO projects (uuid, handle, display_name, description, organization_uuid, created_at, updated_at)
			VALUES ('proj-it-pay', 'payments', 'Payments', '', 'org-it-001', datetime('now'), datetime('now'))`,
		`INSERT INTO projects (uuid, handle, display_name, description, organization_uuid, created_at, updated_at)
			VALUES ('proj-it-bill', 'billing', 'Billing', '', 'org-it-001', datetime('now'), datetime('now'))`,
		`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('art-it-orders', 'RestApi', 'org-it-001')`,
		`INSERT INTO rest_apis (uuid, organization_uuid, handle, display_name, version, project_uuid, lifecycle_status, configuration, created_at, updated_at)
			VALUES ('art-it-orders', 'org-it-001', 'orders', 'Orders', 'v1.0', 'proj-it-pay', 'CREATED', '{}', datetime('now'), datetime('now'))`,
		`INSERT INTO artifact_secret_refs (organization_uuid, artifact_uuid, secret_handle, gateway_id)
			VALUES ('org-it-001', 'art-it-orders', 'in-use', '')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	v, err := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}
	identityService := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	svc := service.NewSecretService(repository.NewSecretRepo(db), v, identityService)
	svc.SetScopeResolver(service.NewSecretScopeResolver(repository.NewProjectRepo(db), repository.NewArtifactRepo(db)))

	mux := http.NewServeMux()
	NewSecretHandler(svc, identityService, slog.Default()).RegisterRoutes(mux)
	return middleware.NewTestContextMiddleware(mux)
}

// doScopedRequest sends a multipart body in which "scopes" may repeat.
func doScopedRequest(h http.Handler, method, path string, fields map[string]string, scopes []string) *httptest.ResponseRecorder {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	for _, s := range scopes {
		_ = w.WriteField("scopes", s)
	}
	w.Close()
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Test-Org", "org-it-001")
	req.Header.Set("X-Test-User", "alice")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func responseScopes(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()
	var body struct {
		Scopes []string `json:"scopes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	return body.Scopes
}

func TestSecretHandler_Scopes_CreateGetUpdate(t *testing.T) {
	h := setupScopedSecretTestEnv(t)

	w := doScopedRequest(h, http.MethodPost, "/api/v0.9/secrets",
		map[string]string{"id": "pay-key", "displayName": "Pay Key", "value": "v1"},
		[]string{"project:payments", "artifact:RestApi:orders"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	want := []string{"artifact:RestApi:orders", "project:payments"}
	if got := responseScopes(t, w); !reflect.DeepEqual(got, want) {
		t.Errorf("create scopes = %v, want %v", got, want)
	}

	w = doScopedRequest(h, http.MethodGet, "/api/v0.9/secrets/pay-key", nil, nil)
	if got := responseScopes(t, w); w.Code != http.StatusOK || !reflect.DeepEqual(got, want) {
		t.Errorf("get: %d scopes = %v, want %v", w.Code, got, want)
	}

	// No scopes field: the scopes stay.
	w = doScopedRequest(h, http.MethodPut, "/api/v0.9/secrets/pay-key", map[string]string{"value": "v2"}, nil)
	if got := responseScopes(t, w); w.Code != http.StatusOK || !reflect.DeepEqual(got, want) {
		t.Errorf("update without scopes: %d scopes = %v, want %v", w.Code, got, want)
	}

	// Scopes given: they replace.
	w = doScopedRequest(h, http.MethodPut, "/api/v0.9/secrets/pay-key", map[string]string{"value": "v3"}, []string{"org"})
	if got := responseScopes(t, w); w.Code != http.StatusOK || !reflect.DeepEqual(got, []string{"org"}) {
		t.Errorf("update to org: %d scopes = %v", w.Code, got)
	}
}

func TestSecretHandler_Scopes_DefaultIsOrg(t *testing.T) {
	h := setupScopedSecretTestEnv(t)
	w := doScopedRequest(h, http.MethodPost, "/api/v0.9/secrets",
		map[string]string{"id": "plain", "displayName": "Plain", "value": "v"}, nil)
	if got := responseScopes(t, w); w.Code != http.StatusCreated || !reflect.DeepEqual(got, []string{"org"}) {
		t.Errorf("create: %d scopes = %v", w.Code, got)
	}
}

func TestSecretHandler_Scopes_Invalid400(t *testing.T) {
	h := setupScopedSecretTestEnv(t)
	for _, scopes := range [][]string{
		{"project:nope"},
		{"artifact:RestApi:nope"},
		{"artifact:Unknown:orders"},
		{"org,project:payments"},
		{"bogus"},
	} {
		w := doScopedRequest(h, http.MethodPost, "/api/v0.9/secrets",
			map[string]string{"id": "bad", "displayName": "Bad", "value": "v"}, scopes)
		if w.Code != http.StatusBadRequest {
			t.Errorf("scopes %v: got %d %s, want 400", scopes, w.Code, w.Body.String())
		}
	}
	// None of the refused requests stored the secret.
	if w := doScopedRequest(h, http.MethodGet, "/api/v0.9/secrets/bad", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("refused secret was stored: GET %d", w.Code)
	}
}

func TestSecretHandler_Scopes_NarrowingGuard(t *testing.T) {
	h := setupScopedSecretTestEnv(t)
	w := doScopedRequest(h, http.MethodPost, "/api/v0.9/secrets",
		map[string]string{"id": "in-use", "displayName": "In Use", "value": "v"}, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}

	// Narrowing to the project that holds the referencing API is allowed, and so
	// is narrowing to the API itself.
	for _, scopes := range [][]string{{"project:payments"}, {"artifact:RestApi:orders"}} {
		w = doScopedRequest(h, http.MethodPut, "/api/v0.9/secrets/in-use", map[string]string{"value": "v2"}, scopes)
		if w.Code != http.StatusOK {
			t.Fatalf("narrow to %v: %d %s", scopes, w.Code, w.Body.String())
		}
	}

	// Narrowing to another project would leave "orders" referencing a secret it
	// can no longer use: refused, and the scopes are unchanged.
	w = doScopedRequest(h, http.MethodPut, "/api/v0.9/secrets/in-use", map[string]string{"value": "v3"},
		[]string{"project:billing"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("narrow away from the reference: got %d %s, want 400", w.Code, w.Body.String())
	}
	w = doScopedRequest(h, http.MethodGet, "/api/v0.9/secrets/in-use", nil, nil)
	if got := responseScopes(t, w); !reflect.DeepEqual(got, []string{"artifact:RestApi:orders"}) {
		t.Errorf("scopes after refusal = %v", got)
	}
}

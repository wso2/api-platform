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

// End-to-end coverage for POST /rest-apis/{restApiId}/api-keys over the real handler,
// service and SQLite-backed repository stack. The service-level unit tests in
// internal/service/apikey_create_test.go pin the key-material rules; these pin the things
// only an HTTP round trip can see — the status code, the JSON wire format, and the
// Location header.

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
	"regexp"
	"testing"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"

	_ "github.com/mattn/go-sqlite3"
)

const (
	apiKeyITOrgID     = "org-key-001"
	apiKeyITProjectID = "proj-key-001"
	apiKeyITAPIUUID   = "api-key-001"
	apiKeyITAPIHandle = "petstore"
	apiKeyITUser      = "sub-key-user"
)

// hexKeyRegex matches what utils.GenerateAPIKey produces: 32 crypto/rand bytes hex-encoded.
var hexKeyRegex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// noopEventHub satisfies the GatewayEventsService dependency. The seeded API is associated
// with no gateway, so nothing is ever published — the key is persisted centrally and any
// gateway associated later picks it up via the deploy-time backfill.
type noopEventHub struct{}

func (noopEventHub) Initialize() error                               { return nil }
func (noopEventHub) RegisterGateway(string) error                    { return nil }
func (noopEventHub) PublishEvent(string, eventhub.Event) error       { return nil }
func (noopEventHub) Subscribe(string) (<-chan eventhub.Event, error) { return nil, nil }
func (noopEventHub) Unsubscribe(string, <-chan eventhub.Event) error { return nil }
func (noopEventHub) UnsubscribeAll(string) error                     { return nil }
func (noopEventHub) CleanUpEvents() error                            { return nil }
func (noopEventHub) Close() error                                    { return nil }

// setupAPIKeyHandlerTestEnv builds the full API key handler stack over a real SQLite
// database carrying the production schema, with one REST API seeded to hang keys off.
func setupAPIKeyHandlerTestEnv(t *testing.T) (http.Handler, *database.DB, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "api-key-handler-test.db")
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db := &database.DB{DB: sqlDB}

	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	// api_keys.artifact_uuid is a foreign key into artifacts, and the handle is resolved
	// against rest_apis, so both rows are required for a key to be creatable.
	seed := []struct {
		desc  string
		query string
		args  []any
	}{
		{"organization", `INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid)
			VALUES (?, 'key-org', 'Key Org', 'default', 'idp-ref')`, []any{apiKeyITOrgID}},
		{"project", `INSERT INTO projects (uuid, handle, display_name, organization_uuid)
			VALUES (?, 'default', 'Default', ?)`, []any{apiKeyITProjectID, apiKeyITOrgID}},
		{"artifact", `INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, 'REST_API', ?)`,
			[]any{apiKeyITAPIUUID, apiKeyITOrgID}},
		{"rest api", `INSERT INTO rest_apis (uuid, organization_uuid, handle, display_name, version, project_uuid, configuration)
			VALUES (?, ?, ?, 'Petstore', 'v1.0', ?, '{}')`,
			[]any{apiKeyITAPIUUID, apiKeyITOrgID, apiKeyITAPIHandle, apiKeyITProjectID}},
	}
	for _, s := range seed {
		if _, err := db.Exec(s.query, s.args...); err != nil {
			t.Fatalf("seed %s: %v", s.desc, err)
		}
	}

	registry := repository.NewArtifactTableRegistry()
	identityService := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	apiKeyService := service.NewAPIKeyService(
		repository.NewAPIRepo(db),
		repository.NewArtifactRepo(db, registry),
		repository.NewAPIKeyRepo(db, registry),
		service.NewGatewayEventsService(noopEventHub{}, identityService, slog.Default()),
		noopAudit{},
		nil, // hashingAlgorithms — defaults to [sha256]
		slog.Default(),
	)

	mux := http.NewServeMux()
	NewAPIKeyHandler(apiKeyService, identityService, middleware.ValidationModeScope, slog.Default()).
		RegisterRoutes(mux)

	return middleware.NewTestContextMiddleware(mux), db, func() { sqlDB.Close() }
}

// postAPIKey issues the create request with the org/user context the auth middleware would
// otherwise have populated from a verified token.
func postAPIKey(t *testing.T, r http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost,
		constants.APIBasePath+"/rest-apis/"+apiKeyITAPIHandle+"/api-keys", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Org", apiKeyITOrgID)
	req.Header.Set("X-Test-User", apiKeyITUser)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// TestAPIKeyHandler_CreateWithoutSuppliedValue is the end-to-end reproduction from
// https://github.com/wso2/api-platform/issues/3252. The exact body below used to come back
// 400 {"code":"VALIDATION_FAILED","message":"API key value is required"}.
func TestAPIKeyHandler_CreateWithoutSuppliedValue(t *testing.T) {
	r, db, cleanup := setupAPIKeyHandlerTestEnv(t)
	t.Cleanup(cleanup)

	rec := postAPIKey(t, r, `{"displayName": "api-key-1-test"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Decoded into a map, not the typed response, so an absent field is distinguishable
	// from a zero value.
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	apiKey, ok := body["apiKey"].(string)
	if !ok {
		t.Fatalf("response carried no apiKey; the generated key is unrecoverable after this response: %s", rec.Body.String())
	}
	if !hexKeyRegex.MatchString(apiKey) {
		t.Errorf("generated key %q is not 64 lowercase hex characters", apiKey)
	}

	keyID, ok := body["keyId"].(string)
	if !ok || keyID == "" {
		t.Fatalf("response carried no keyId: %s", rec.Body.String())
	}
	if want := constants.APIBasePath + "/rest-apis/" + apiKeyITAPIHandle + "/api-keys/" + keyID; rec.Header().Get("Location") != want {
		t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), want)
	}

	// The row must hold a hash and a mask, never the plaintext.
	var masked, hashes string
	if err := db.QueryRow(`SELECT masked_api_key, api_key_hashes FROM api_keys WHERE artifact_uuid = ? AND handle = ?`,
		apiKeyITAPIUUID, keyID).Scan(&masked, &hashes); err != nil {
		t.Fatalf("persisted key not found for handle %q: %v", keyID, err)
	}
	if masked != "***"+apiKey[len(apiKey)-5:] {
		t.Errorf("persisted masked key = %q, does not mask the returned key", masked)
	}
	if hashes == "" || masked == apiKey {
		t.Error("plaintext key material leaked into a persisted column")
	}
}

// TestAPIKeyHandler_CreateWithSuppliedValueOmitsApiKey covers the inject path. The response
// must not carry the field at all — `omitempty` on the generated struct is what keeps a
// caller's own secret from being echoed back, so assert absence rather than emptiness.
func TestAPIKeyHandler_CreateWithSuppliedValueOmitsApiKey(t *testing.T) {
	r, _, cleanup := setupAPIKeyHandlerTestEnv(t)
	t.Cleanup(cleanup)

	rec := postAPIKey(t, r, `{"displayName": "Injected Key", "apiKey": "sk_example_1234567890abcdef"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, present := body["apiKey"]; present {
		t.Errorf("response echoed a caller-supplied key: %s", rec.Body.String())
	}
}

// TestAPIKeyHandler_CreateWithEmptyBody covers a body carrying neither id nor displayName.
// The handlers used to pre-derive the name via utils.GenerateHandle, which errors on an
// empty source, so this returned 400 — even though nothing enforces displayName at runtime.
// Naming now belongs to the service, which falls back to "<handle>-key-<8 hex>".
func TestAPIKeyHandler_CreateWithEmptyBody(t *testing.T) {
	r, _, cleanup := setupAPIKeyHandlerTestEnv(t)
	t.Cleanup(cleanup)

	rec := postAPIKey(t, r, `{}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	keyID, _ := body["keyId"].(string)
	if want := regexp.MustCompile(`^` + apiKeyITAPIHandle + `-key-[0-9a-f]{8}$`); !want.MatchString(keyID) {
		t.Errorf("derived keyId = %q, want the <handle>-key-<8 hex> fallback shape", keyID)
	}
	if _, ok := body["apiKey"].(string); !ok {
		t.Errorf("response carried no generated apiKey: %s", rec.Body.String())
	}
}

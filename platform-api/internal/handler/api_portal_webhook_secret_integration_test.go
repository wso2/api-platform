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

// Integration tests for POST /api-portals/{apiPortalId}/webhook-secret and
// the webhookSecretConfigured flag on GET /api-portals/{apiPortalId}. Covers
// FR-006, FR-007, FR-009, FR-010, and the receiverPath shape described in
// research.md Decision 5.

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
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"

	_ "github.com/mattn/go-sqlite3"
)


// setupAPIPortalWebhookSecretEnv is a sibling of setupAPIPortalHandlerEnv
// (api_portal_integration_test.go) that wires the per-portal webhook secret
// service and the webhook config into the handler so GenerateWebhookSecret
// and the webhookSecretConfigured flag on GetAPIPortal both work.
func setupAPIPortalWebhookSecretEnv(t *testing.T) (http.Handler, *database.DB, func()) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "api-portal-webhook-test.db")
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
	webhookSecretRepo := repository.NewAPIPortalWebhookSecretRepo(db)
	orgRepo := repository.NewOrganizationRepo(db)
	v := apiPortalTestVault(t)
	keyRegistry := service.NewPortalWebhookKeyRegistry(portalRepo, webhookSecretRepo, v, 5*time.Minute)
	webhookSecrets := service.NewAPIPortalWebhookSecretService(portalRepo, webhookSecretRepo, v, 10*time.Minute, keyRegistry, slog.Default())
	identityService := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))

	svc := service.NewAPIPortalService(portalRepo, orgRepo, noopAudit{}, v, nil, webhookSecrets, identityService, slog.Default())
	h := NewAPIPortalHandler(svc, identityService, slog.Default())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return middleware.NewTestContextMiddleware(mux), db, func() { _ = sqlDB.Close() }
}

// createTestAPIPortal issues POST /api-portals so subsequent webhook-secret
// calls have a real portal row to attach to. Returns the created portal's
// handle. Lets the tests focus on the webhook-secret flow rather than
// re-asserting the create path (already covered in
// api_portal_integration_test.go).
func createTestAPIPortal(t *testing.T, r http.Handler) string {
	t.Helper()
	body := mustJSON(t, map[string]any{
		"name":      "Webhook Portal",
		"handle":    "webhook-portal",
		"url":       "https://webhook-portal.example.com",
		"sharedKey": apiPortalTestSharedKey,
	})
	req := apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase, body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("fixture create portal: want 201, got %d: %s", rec.Code, rec.Body.String())
	}
	return "webhook-portal"
}

// webhookSecretResp mirrors api.GenerateApiPortalWebhookSecretResponse so the
// test can assert field names and types without importing the generated DTO.
type webhookSecretResp struct {
	Secret      string `json:"secret"`
	ReceiverPath string `json:"receiverPath"`
}

// hexSecretPattern matches the 64-char lowercase hex secret shape FR-006
// commits to (32 bytes of entropy, hex-encoded).
var hexSecretPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestAPIPortalHandler_WebhookSecret_FirstCallGeneratesAndReturnsPlaintext(t *testing.T) {
	r, _, cleanup := setupAPIPortalWebhookSecretEnv(t)
	t.Cleanup(cleanup)

	handle := createTestAPIPortal(t, r)
	req := apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase+"/"+handle+"/webhook-secret", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("webhook-secret: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got webhookSecretResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !hexSecretPattern.MatchString(got.Secret) {
		t.Errorf("FR-006: secret must be 64 lowercase hex characters, got %q (length %d)", got.Secret, len(got.Secret))
	}
	wantPrefix := "/api/internal/v0.9/webhook/portals/"
	if !strings.HasPrefix(got.ReceiverPath, wantPrefix) || !strings.HasSuffix(got.ReceiverPath, "/events") {
		t.Errorf("receiverPath shape wrong: %q (want prefix %q, suffix %q)", got.ReceiverPath, wantPrefix, "/events")
	}
}

func TestAPIPortalHandler_WebhookSecret_SecondCallRotates(t *testing.T) {
	r, db, cleanup := setupAPIPortalWebhookSecretEnv(t)
	t.Cleanup(cleanup)

	handle := createTestAPIPortal(t, r)
	path := apiPortalTestBase + "/" + handle + "/webhook-secret"

	// First call establishes the secret.
	first := httptest.NewRecorder()
	r.ServeHTTP(first, apiPortalTestRequest(t, http.MethodPost, path, nil))
	var firstResp webhookSecretResp
	if err := json.Unmarshal(first.Body.Bytes(), &firstResp); err != nil {
		t.Fatalf("decode first: %v", err)
	}

	// Second call rotates.
	second := httptest.NewRecorder()
	r.ServeHTTP(second, apiPortalTestRequest(t, http.MethodPost, path, nil))
	if second.Code != http.StatusOK {
		t.Fatalf("second webhook-secret: want 200, got %d: %s", second.Code, second.Body.String())
	}
	var secondResp webhookSecretResp
	if err := json.Unmarshal(second.Body.Bytes(), &secondResp); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	if secondResp.Secret == firstResp.Secret {
		t.Fatal("FR-008: second call MUST return a different secret than the first")
	}
	if secondResp.ReceiverPath != firstResp.ReceiverPath {
		t.Errorf("receiverPath must stay stable across rotations; got %q then %q", firstResp.ReceiverPath, secondResp.ReceiverPath)
	}

	// The database row MUST now carry both current and previous ciphertexts
	// plus a non-null previous_valid_until, so the receiver's grace window
	// has something to fall back to (FR-008).
	var currEnc, prevEnc []byte
	var prevValidUntil sql.NullTime
	err := db.QueryRow(`SELECT secret_enc, previous_secret_enc, previous_valid_until
	                    FROM api_portal_webhook_secrets ORDER BY created_at DESC LIMIT 1`).
		Scan(&currEnc, &prevEnc, &prevValidUntil)
	if err != nil {
		t.Fatalf("scan row: %v", err)
	}
	if len(currEnc) == 0 || len(prevEnc) == 0 {
		t.Fatalf("expected both current and previous ciphertext populated after rotation (current=%d bytes, previous=%d bytes)", len(currEnc), len(prevEnc))
	}
	if !prevValidUntil.Valid {
		t.Fatal("previous_valid_until MUST be set after rotation; it is the receiver's grace deadline")
	}
	if time.Until(prevValidUntil.Time) <= 0 {
		t.Fatalf("previous_valid_until must be in the future after a rotation; got %s", prevValidUntil.Time)
	}
}

func TestAPIPortalHandler_WebhookSecret_PortalGetSurfacesConfiguredFlagButNeverPlaintext(t *testing.T) {
	r, _, cleanup := setupAPIPortalWebhookSecretEnv(t)
	t.Cleanup(cleanup)

	handle := createTestAPIPortal(t, r)

	// Before the first webhook-secret call: the GET response MUST NOT claim
	// a secret is configured (even if the field is omitted or false, it must
	// not be true).
	getBefore := httptest.NewRecorder()
	r.ServeHTTP(getBefore, apiPortalTestRequest(t, http.MethodGet, apiPortalTestBase+"/"+handle, nil))
	if getBefore.Code != http.StatusOK {
		t.Fatalf("GET before: want 200, got %d: %s", getBefore.Code, getBefore.Body.String())
	}
	var beforeBody map[string]any
	if err := json.Unmarshal(getBefore.Body.Bytes(), &beforeBody); err != nil {
		t.Fatalf("decode before: %v", err)
	}
	if v, ok := beforeBody["webhookSecretConfigured"]; ok && v == true {
		t.Fatalf("webhookSecretConfigured must not be true before any secret has been generated; got %v", beforeBody["webhookSecretConfigured"])
	}

	// Call webhook-secret to establish the ciphertext row.
	post := httptest.NewRecorder()
	r.ServeHTTP(post, apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase+"/"+handle+"/webhook-secret", nil))
	if post.Code != http.StatusOK {
		t.Fatalf("POST webhook-secret: want 200, got %d: %s", post.Code, post.Body.String())
	}
	var postResp webhookSecretResp
	_ = json.Unmarshal(post.Body.Bytes(), &postResp)

	// After the call: GET surfaces webhookSecretConfigured: true, but MUST NOT
	// surface the plaintext anywhere in the body (not as `secret`, not
	// embedded in any string).
	getAfter := httptest.NewRecorder()
	r.ServeHTTP(getAfter, apiPortalTestRequest(t, http.MethodGet, apiPortalTestBase+"/"+handle, nil))
	if getAfter.Code != http.StatusOK {
		t.Fatalf("GET after: want 200, got %d: %s", getAfter.Code, getAfter.Body.String())
	}
	var afterBody map[string]any
	if err := json.Unmarshal(getAfter.Body.Bytes(), &afterBody); err != nil {
		t.Fatalf("decode after: %v", err)
	}
	if v, ok := afterBody["webhookSecretConfigured"]; !ok || v != true {
		t.Fatalf("FR-010: expected webhookSecretConfigured=true after generate, got %v (present=%v)", v, ok)
	}
	if _, present := afterBody["secret"]; present {
		t.Fatal("FR-010: GET response MUST NOT contain a `secret` field")
	}
	// Belt-and-suspenders: the plaintext MUST NOT be embedded anywhere in the
	// body — not as a field, not inside another string.
	if bytes.Contains(getAfter.Body.Bytes(), []byte(postResp.Secret)) {
		t.Fatalf("FR-010: plaintext secret leaked in GET response body: %s", getAfter.Body.String())
	}
}

func TestAPIPortalHandler_WebhookSecret_HandleNotFoundReturns404(t *testing.T) {
	r, _, cleanup := setupAPIPortalWebhookSecretEnv(t)
	t.Cleanup(cleanup)

	// No portal with handle "nonexistent" was created, so the service's
	// handle→portal lookup fails. The handler must surface this as a 404
	// scoped by the caller's org — not a 500, and not a 200 that silently
	// generates a secret for nothing.
	req := apiPortalTestRequest(t, http.MethodPost, apiPortalTestBase+"/nonexistent/webhook-secret", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for unknown portal handle, got %d: %s", rec.Code, rec.Body.String())
	}
}

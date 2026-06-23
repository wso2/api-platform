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
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"platform-api/src/config"
	"platform-api/src/internal/database"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/service"
	"platform-api/src/internal/vault"

	"github.com/gin-gonic/gin"
)

// testHashToken replicates the private hashToken function in the service package.
func testHashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// gatewaySecretTestEnv holds everything needed to exercise the gateway secret endpoints.
type gatewaySecretTestEnv struct {
	router     *gin.Engine
	sqlDB      *sql.DB
	db         *database.DB
	svc        *service.SecretService
	orgID      string
	gatewayID  string
	plainToken string
}

// setupGatewaySecretTestEnv creates a full handler stack backed by a fresh in-memory SQLite DB,
// inserting one organization and one gateway with a known API token.
func setupGatewaySecretTestEnv(t *testing.T) (*gatewaySecretTestEnv, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "gw-test.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err = sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	db := &database.DB{DB: sqlDB}

	schemaPath := filepath.Join("..", "database", "schema.sqlite.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	const orgID = "org-gw-001"
	const gatewayID = "gw-001"
	const plainToken = "super-secret-token"

	if _, err = db.Exec(`INSERT INTO organizations (uuid, handle, name, region, created_at, updated_at)
		VALUES (?, 'gw-org', 'GW Org', 'default', datetime('now'), datetime('now'))`, orgID); err != nil {
		t.Fatalf("insert organization: %v", err)
	}

	// Insert gateway (description must be non-NULL — repo scans it into string)
	if _, err = db.Exec(`INSERT INTO gateways (uuid, organization_uuid, name, display_name, description, vhost, version, is_active, created_at, updated_at)
		VALUES (?, ?, 'test-gw', 'Test GW', '', 'localhost', '1.0', 1, datetime('now'), datetime('now'))`, gatewayID, orgID); err != nil {
		t.Fatalf("insert gateway: %v", err)
	}

	// Insert gateway token
	tokenHash := testHashToken(plainToken)
	if _, err = db.Exec(`INSERT INTO gateway_tokens (uuid, gateway_uuid, token_hash, salt, status, created_at)
		VALUES ('tok-001', ?, ?, 'dummy-salt', 'active', datetime('now'))`, gatewayID, tokenHash); err != nil {
		t.Fatalf("insert gateway token: %v", err)
	}

	v, err := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}

	secretRepo := repository.NewSecretRepo(db)
	secretSvc := service.NewSecretService(secretRepo, v)

	deploymentRepo := repository.NewDeploymentRepo(db)
	gatewayRepo := repository.NewGatewayRepo(db)

	gatewaySvc := service.NewGatewayService(gatewayRepo, nil, nil, nil, nil, slog.Default(), false, false)

	cfg := &config.Server{}
	gwInternalSvc := service.NewGatewayInternalAPIService(
		nil, nil, nil, nil, nil, nil, nil, nil,
		deploymentRepo, gatewayRepo,
		nil, nil, nil, nil,
		secretRepo,
		cfg, slog.Default(),
	)

	h := NewGatewayInternalAPIHandler(gatewaySvc, gwInternalSvc, secretSvc, slog.Default())

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.RegisterRoutes(r)

	env := &gatewaySecretTestEnv{
		router:     r,
		sqlDB:      sqlDB,
		db:         db,
		svc:        secretSvc,
		orgID:      orgID,
		gatewayID:  gatewayID,
		plainToken: plainToken,
	}
	cleanup := func() { sqlDB.Close() }
	return env, cleanup
}

// doGWRequest sends a request with the given api-key header value.
func doGWRequest(r *gin.Engine, method, path, apiKey string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, path, nil)
	if apiKey != "" {
		req.Header.Set("api-key", apiKey)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// insertArtifactSecretRef inserts a row linking a secret handle to a gateway via an artifact.
func insertArtifactSecretRef(t *testing.T, db *database.DB, orgID, artifactUUID, secretHandle, gatewayID string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT OR IGNORE INTO artifact_secret_refs (organization_id, artifact_uuid, secret_handle, gateway_id, created_at)
		 VALUES (?, ?, ?, ?, datetime('now'))`,
		orgID, artifactUUID, secretHandle, gatewayID,
	)
	if err != nil {
		t.Fatalf("insertArtifactSecretRef: %v", err)
	}
}

// insertArtifact inserts a minimal artifact row.
func insertArtifact(t *testing.T, db *database.DB, orgID, uuid, handle string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT OR IGNORE INTO artifacts (uuid, handle, name, version, kind, organization_uuid, created_at, updated_at)
		 VALUES (?, ?, ?, '1.0', 'REST', ?, datetime('now'), datetime('now'))`,
		uuid, handle, handle, orgID,
	)
	if err != nil {
		t.Fatalf("insertArtifact: %v", err)
	}
}

// createSecretDirect creates a secret through the service layer.
func createSecretDirect(t *testing.T, svc *service.SecretService, orgID, handle, value string) {
	t.Helper()
	_, err := svc.Create(orgID, "alice", &dto.CreateSecretRequest{Handle: handle, Value: value})
	if err != nil {
		t.Fatalf("createSecretDirect(%q): %v", handle, err)
	}
}

// parseGWBody parses the response body into a map.
func parseGWBody(w *httptest.ResponseRecorder) map[string]interface{} {
	var m map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return m
}

// listFromBody extracts the "list" slice from a GW response body.
func listFromBody(t *testing.T, w *httptest.ResponseRecorder) []interface{} {
	t.Helper()
	body := parseGWBody(w)
	list, ok := body["list"].([]interface{})
	if !ok {
		t.Fatalf("expected 'list' array in response, got: %v", body)
	}
	return list
}

// TC-20: GET /api/internal/v1/secrets without api-key returns 401.
func TestGatewaySecretHandler_NoAPIKey_401(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TC-21: GET /api/internal/v1/secrets with invalid api-key returns 401.
func TestGatewaySecretHandler_InvalidAPIKey_401(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets", "wrong-token")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TC-22: GET returns only secrets deployed to this gateway; unrelated secrets are absent.
func TestGatewaySecretHandler_ListOnlyGatewaySecrets(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	// Create two secrets
	createSecretDirect(t, env.svc, env.orgID, "gw-secret", "val1")
	createSecretDirect(t, env.svc, env.orgID, "other-secret", "val2")

	// Only link "gw-secret" to the gateway
	insertArtifact(t, env.db, env.orgID, "art-gw-22", "api-22")
	insertArtifactSecretRef(t, env.db, env.orgID, "art-gw-22", "gw-secret", env.gatewayID)

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets", env.plainToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	list := listFromBody(t, w)
	if len(list) != 1 {
		t.Fatalf("expected 1 secret, got %d: %v", len(list), list)
	}
	item := list[0].(map[string]interface{})
	if item["handle"] != "gw-secret" {
		t.Errorf("expected handle=gw-secret, got %v", item["handle"])
	}
}

// TC-23: Valid api-key but no artifact_secret_refs rows → empty list.
func TestGatewaySecretHandler_EmptyList_NoDeployments(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets", env.plainToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := parseGWBody(w)
	list, ok := body["list"].([]interface{})
	if !ok {
		// nil list also ok — check count
		list = nil
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
	count, _ := body["count"].(float64)
	if count != 0 {
		t.Errorf("expected count=0, got %v", count)
	}
}

// TC-24: ?updatedAfter with far-future timestamp returns empty list.
func TestGatewaySecretHandler_UpdatedAfterFilter(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	createSecretDirect(t, env.svc, env.orgID, "ua-secret", "val")
	insertArtifact(t, env.db, env.orgID, "art-ua-24", "api-ua-24")
	insertArtifactSecretRef(t, env.db, env.orgID, "art-ua-24", "ua-secret", env.gatewayID)

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets?updatedAfter=2099-01-01T00:00:00Z", env.plainToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	list := listFromBody(t, w)
	if len(list) != 0 {
		t.Errorf("expected empty list for far-future updatedAfter, got %d items", len(list))
	}
}

// TC-25: ?updatedAfter with invalid value returns 400.
func TestGatewaySecretHandler_InvalidUpdatedAfter_400(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets?updatedAfter=bad-date", env.plainToken)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TC-26: Without ?includeValues=true, items must not have a "value" field.
func TestGatewaySecretHandler_NoValueByDefault(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	createSecretDirect(t, env.svc, env.orgID, "nv-secret", "top-secret")
	insertArtifact(t, env.db, env.orgID, "art-nv-26", "api-nv-26")
	insertArtifactSecretRef(t, env.db, env.orgID, "art-nv-26", "nv-secret", env.gatewayID)

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets", env.plainToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	list := listFromBody(t, w)
	if len(list) == 0 {
		t.Fatal("expected at least one item")
	}
	for _, raw := range list {
		item := raw.(map[string]interface{})
		if _, hasValue := item["value"]; hasValue {
			t.Errorf("list item should NOT have 'value' field by default, got: %v", item)
		}
	}
}

// TC-27: GET /api/internal/v1/secrets/:id/value returns {"value":"plaintext"}.
func TestGatewaySecretHandler_GetSecretValue(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	createSecretDirect(t, env.svc, env.orgID, "val-secret", "my-plaintext")

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets/val-secret/value", env.plainToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := parseGWBody(w)
	if body["value"] != "my-plaintext" {
		t.Errorf("expected value=my-plaintext, got %v", body["value"])
	}
}

// TC-28: After rotating (updating) a secret, GET /value returns the new plaintext.
func TestGatewaySecretHandler_GetSecretValue_AfterRotation(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	createSecretDirect(t, env.svc, env.orgID, "rot-secret", "old-value")
	_, err := env.svc.Update(env.orgID, "rot-secret", "alice", &dto.UpdateSecretRequest{Value: "new-value"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets/rot-secret/value", env.plainToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := parseGWBody(w)
	if body["value"] != "new-value" {
		t.Errorf("expected value=new-value, got %v", body["value"])
	}
}

// TC-29: GET /value for non-existent handle returns 404.
func TestGatewaySecretHandler_GetSecretValue_NotFound(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets/ghost-handle/value", env.plainToken)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TC-30: Secret deployed for GW-A is not returned for GW-B.
func TestGatewaySecretHandler_SecretNotReturnedForOtherGateway(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	// Create a second gateway
	gwBID := "gw-002"
	plainTokenB := "token-for-gw-b"
	if _, err := env.db.Exec(`INSERT INTO gateways (uuid, organization_uuid, name, display_name, description, vhost, version, is_active, created_at, updated_at)
		VALUES (?, ?, 'test-gw-b', 'Test GW B', '', 'localhost2', '1.0', 1, datetime('now'), datetime('now'))`, gwBID, env.orgID); err != nil {
		t.Fatalf("insert second gateway: %v", err)
	}
	hashB := testHashToken(plainTokenB)
	if _, err := env.db.Exec(`INSERT INTO gateway_tokens (uuid, gateway_uuid, token_hash, salt, status, created_at)
		VALUES ('tok-002', ?, ?, 'dummy-salt', 'active', datetime('now'))`, gwBID, hashB); err != nil {
		t.Fatalf("insert second gateway token: %v", err)
	}

	// Deploy secret only to GW-A
	createSecretDirect(t, env.svc, env.orgID, "gwa-only", "val")
	insertArtifact(t, env.db, env.orgID, "art-30", "api-30")
	insertArtifactSecretRef(t, env.db, env.orgID, "art-30", "gwa-only", env.gatewayID)

	// GW-B should get empty list
	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets", plainTokenB)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := parseGWBody(w)
	count, _ := body["count"].(float64)
	list, _ := body["list"].([]interface{})
	if len(list) != 0 || count != 0 {
		t.Errorf("GW-B should see 0 secrets, got %d (count=%v)", len(list), count)
	}
}

// TC-31: Secret deployed to both GW-A and GW-B is returned for each.
func TestGatewaySecretHandler_SharedSecretReturnedForBothGateways(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	// Second gateway
	gwBID := "gw-003"
	plainTokenB := "token-shared-gw-b"
	if _, err := env.db.Exec(`INSERT INTO gateways (uuid, organization_uuid, name, display_name, description, vhost, version, is_active, created_at, updated_at)
		VALUES (?, ?, 'test-gw-shared', 'Test GW Shared', '', 'localhost3', '1.0', 1, datetime('now'), datetime('now'))`, gwBID, env.orgID); err != nil {
		t.Fatalf("insert shared gateway: %v", err)
	}
	hashB := testHashToken(plainTokenB)
	if _, err := env.db.Exec(`INSERT INTO gateway_tokens (uuid, gateway_uuid, token_hash, salt, status, created_at)
		VALUES ('tok-003', ?, ?, 'dummy-salt', 'active', datetime('now'))`, gwBID, hashB); err != nil {
		t.Fatalf("insert shared gateway token: %v", err)
	}

	createSecretDirect(t, env.svc, env.orgID, "shared-secret", "shared-val")
	insertArtifact(t, env.db, env.orgID, "art-31-a", "api-31-a")
	insertArtifact(t, env.db, env.orgID, "art-31-b", "api-31-b")
	insertArtifactSecretRef(t, env.db, env.orgID, "art-31-a", "shared-secret", env.gatewayID)
	insertArtifactSecretRef(t, env.db, env.orgID, "art-31-b", "shared-secret", gwBID)

	// GW-A sees it
	wA := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets", env.plainToken)
	if wA.Code != http.StatusOK {
		t.Fatalf("GW-A: expected 200, got %d", wA.Code)
	}
	listA := listFromBody(t, wA)
	if len(listA) != 1 {
		t.Errorf("GW-A: expected 1, got %d", len(listA))
	}

	// GW-B sees it
	wB := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets", plainTokenB)
	if wB.Code != http.StatusOK {
		t.Fatalf("GW-B: expected 200, got %d", wB.Code)
	}
	listB := listFromBody(t, wB)
	if len(listB) != 1 {
		t.Errorf("GW-B: expected 1, got %d", len(listB))
	}
}

// TC-32: After removing artifact_secret_refs row, secret no longer appears in list.
func TestGatewaySecretHandler_SecretGoneAfterUndeploy(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	createSecretDirect(t, env.svc, env.orgID, "undeploy-secret", "val")
	insertArtifact(t, env.db, env.orgID, "art-32", "api-32")
	insertArtifactSecretRef(t, env.db, env.orgID, "art-32", "undeploy-secret", env.gatewayID)

	// Confirm it's present
	w1 := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets", env.plainToken)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w1.Code)
	}
	if len(listFromBody(t, w1)) != 1 {
		t.Fatal("expected secret to be present before undeploy")
	}

	// Remove the ref row
	if _, err := env.db.Exec(`DELETE FROM artifact_secret_refs WHERE organization_id = ? AND artifact_uuid = ?`,
		env.orgID, "art-32"); err != nil {
		t.Fatalf("delete artifact secret ref: %v", err)
	}

	// Now it must be absent
	w2 := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets", env.plainToken)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
	list2 := listFromBody(t, w2)
	if len(list2) != 0 {
		t.Errorf("expected empty list after undeploy, got %d items", len(list2))
	}
}

// TC-33: Two artifact_secret_refs rows with same handle + gateway → appears once.
func TestGatewaySecretHandler_DeduplicatesHandleAcrossArtifacts(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	createSecretDirect(t, env.svc, env.orgID, "dedup-secret", "val")
	insertArtifact(t, env.db, env.orgID, "art-33-a", "api-33-a")
	insertArtifact(t, env.db, env.orgID, "art-33-b", "api-33-b")
	insertArtifactSecretRef(t, env.db, env.orgID, "art-33-a", "dedup-secret", env.gatewayID)
	insertArtifactSecretRef(t, env.db, env.orgID, "art-33-b", "dedup-secret", env.gatewayID)

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets", env.plainToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	list := listFromBody(t, w)
	if len(list) != 1 {
		t.Errorf("expected 1 deduplicated item, got %d: %v", len(list), list)
	}
}

// TC-63: ?includeValues=true → active secret item has "value" field with decrypted plaintext.
func TestGatewaySecretHandler_IncludeValues_ActiveSecretHasValue(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	createSecretDirect(t, env.svc, env.orgID, "iv-active", "the-plaintext")
	insertArtifact(t, env.db, env.orgID, "art-63", "api-63")
	insertArtifactSecretRef(t, env.db, env.orgID, "art-63", "iv-active", env.gatewayID)

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets?includeValues=true", env.plainToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	list := listFromBody(t, w)
	if len(list) == 0 {
		t.Fatal("expected at least one item")
	}
	item := list[0].(map[string]interface{})
	val, hasVal := item["value"]
	if !hasVal {
		t.Errorf("expected 'value' field in item, got: %v", item)
	}
	if val != "the-plaintext" {
		t.Errorf("expected value=the-plaintext, got %v", val)
	}
}

// TC-64: DEPRECATED secret with ?includeValues=true → item has no "value" field.
func TestGatewaySecretHandler_IncludeValues_DeprecatedSecretFails(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	createSecretDirect(t, env.svc, env.orgID, "dep-iv", "val")
	insertArtifact(t, env.db, env.orgID, "art-64", "api-64")
	insertArtifactSecretRef(t, env.db, env.orgID, "art-64", "dep-iv", env.gatewayID)

	// Mark the secret DEPRECATED directly in SQL (bypassing the ref-check in Delete)
	_, err := env.sqlDB.Exec(`UPDATE secrets SET status = 'DEPRECATED', updated_at = datetime('now') WHERE handle = ?`, "dep-iv")
	if err != nil {
		t.Fatalf("deprecate secret: %v", err)
	}

	// Decrypting a DEPRECATED secret returns an error, so the whole bulk request
	// must fail with 500 so the caller can retry rather than receiving a partial response.
	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets?includeValues=true", env.plainToken)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when deprecated secret in list, got %d: %s", w.Code, w.Body.String())
	}
}

// TC-65: No deployments with ?includeValues=true → {"list":[],"count":0}.
func TestGatewaySecretHandler_IncludeValues_EmptyList(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets?includeValues=true", env.plainToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := parseGWBody(w)
	list, _ := body["list"].([]interface{})
	count, _ := body["count"].(float64)
	if len(list) != 0 || count != 0 {
		t.Errorf("expected empty list, got %d items (count=%v)", len(list), count)
	}
}

// TC-66: ?includeValues=true → items have both "hash" and "value" fields.
func TestGatewaySecretHandler_IncludeValues_HashPresentAlongsideValue(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()

	createSecretDirect(t, env.svc, env.orgID, "hash-val", "my-val")
	insertArtifact(t, env.db, env.orgID, "art-66", "api-66")
	insertArtifactSecretRef(t, env.db, env.orgID, "art-66", "hash-val", env.gatewayID)

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/secrets?includeValues=true", env.plainToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	list := listFromBody(t, w)
	if len(list) == 0 {
		t.Fatal("expected at least one item")
	}
	item := list[0].(map[string]interface{})
	if _, hasHash := item["hash"]; !hasHash {
		t.Errorf("expected 'hash' field in item, got: %v", item)
	}
	if _, hasVal := item["value"]; !hasVal {
		t.Errorf("expected 'value' field in item, got: %v", item)
	}
}

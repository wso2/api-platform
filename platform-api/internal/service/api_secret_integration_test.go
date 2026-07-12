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

// Integration tests for REST API secret management, backed by a real SQLite
// DB (not mocks). These exercise the actual artifact_secret_refs reference
// tracking that only the repository layer populates — a scenario the mocked
// unit tests in api_test.go cannot cover, since mockAPIRepository never
// touches that table. This automates the manual curl-based validation used
// to confirm the fix against a live platform-api instance.

package service

import (
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/platform-api/internal/vault"

	_ "github.com/mattn/go-sqlite3"
)

const apiSecretITOrgUUID = "org-api-secret-it"
const apiSecretITProjectUUID = "proj-api-secret-it"

// setupAPISecretTestEnv creates a real SQLite-backed APIService and
// SecretService, wired together exactly as server.go wires them in
// production (APIService.SetSecretService), plus a seeded org and project.
func setupAPISecretTestEnv(t *testing.T) (*APIService, *SecretService, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "api-secret-it.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB.Exec("PRAGMA foreign_keys = ON")
	db := &database.DB{DB: sqlDB}

	schemaPath := filepath.Join("..", "database", "schema.sqlite.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	if _, err = db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, 'api-secret-it-org', 'API Secret IT Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`, apiSecretITOrgUUID); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	if _, err = db.Exec(`INSERT INTO projects (uuid, handle, display_name, description, organization_uuid, created_at, updated_at)
		VALUES (?, 'api-secret-it-proj', 'API Secret IT Project', '', ?, datetime('now'), datetime('now'))`, apiSecretITProjectUUID, apiSecretITOrgUUID); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	v, err := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}

	identity := NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	secretRepo := repository.NewSecretRepo(db)
	secretSvc := NewSecretService(secretRepo, v, identity)

	apiRepo := repository.NewAPIRepo(db)
	projectRepo := repository.NewProjectRepo(db)
	auditRepo := repository.NewAuditRepo(db)

	apiSvc := NewAPIService(apiRepo, projectRepo, nil, nil, nil, nil, nil, nil, &utils.APIUtil{}, slog.Default(), auditRepo, identity)
	apiSvc.SetSecretService(secretSvc)

	cleanup := func() { sqlDB.Close() }
	return apiSvc, secretSvc, cleanup
}

// createTestSecret creates a secret directly via SecretService and fails the
// test on error.
func createTestSecret(t *testing.T, svc *SecretService, orgUUID, handle, value string) {
	t.Helper()
	_, err := svc.Create(orgUUID, "alice", &dto.CreateSecretRequest{
		Handle:      handle,
		DisplayName: handle,
		Value:       value,
	})
	if err != nil {
		t.Fatalf("failed to create secret %q: %v", handle, err)
	}
}

// TestAPIServiceSecretLifecycle_Integration exercises the full secret
// lifecycle for a REST API's upstream auth against a real DB: create with a
// placeholder, redaction on read, delete-protection while referenced,
// preservation across an unrelated update, rotation cleanup, and rejection
// of both upstream and policy placeholders that don't resolve.
func TestAPIServiceSecretLifecycle_Integration(t *testing.T) {
	apiSvc, secretSvc, cleanup := setupAPISecretTestEnv(t)
	defer cleanup()

	createTestSecret(t, secretSvc, apiSecretITOrgUUID, "it-secret-a", "sk-real-backend-token-A")
	createTestSecret(t, secretSvc, apiSecretITOrgUUID, "it-secret-b", "sk-real-backend-token-B")

	// --- Create with a placeholder referencing secret A ---
	createReq := &api.CreateRESTAPIRequest{
		DisplayName: "IT REST API",
		Context:     "/it-rest-api",
		Version:     "v1",
		ProjectId:   "api-secret-it-proj",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{
				Url: utils.StringPtrIfNotEmpty("https://backend.internal/api"),
				Auth: &api.UpstreamAuth{
					Type:   upstreamAuthTypePtr("bearer"),
					Header: ptr("Authorization"),
					Value:  ptr(`{{ secret "it-secret-a" }}`),
				},
			},
		},
	}
	created, err := apiSvc.CreateAPI(createReq, apiSecretITOrgUUID, "alice")
	if err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}

	// --- Redaction: the create response must never carry the real value ---
	if created.Upstream.Main.Auth == nil {
		t.Fatal("expected auth block to be present in create response")
	}
	if created.Upstream.Main.Auth.Value != nil {
		t.Errorf("expected auth value to be redacted in create response, got %q", *created.Upstream.Main.Auth.Value)
	}
	if created.Upstream.Main.Auth.Header == nil || *created.Upstream.Main.Auth.Header != "Authorization" {
		t.Errorf("expected auth header to survive redaction, got %v", created.Upstream.Main.Auth.Header)
	}

	apiUUID := created.Id
	if apiUUID == nil || *apiUUID == "" {
		t.Fatal("expected created API to have an id")
	}

	// --- Delete-protection: secret A must be blocked while referenced ---
	if err := secretSvc.Delete(apiSecretITOrgUUID, "it-secret-a", "alice"); err == nil {
		t.Fatal("expected secret A deletion to be blocked while referenced by the API, got nil error")
	} else {
		var inUse *SecretInUseError
		if !errors.As(err, &inUse) {
			t.Errorf("expected SecretInUseError, got: %v", err)
		}
	}

	// --- Preservation: update the URL only, omit auth entirely ---
	updateReq := &api.RESTAPI{
		DisplayName: "IT REST API",
		Context:     "/it-rest-api",
		Version:     "v1",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{Url: utils.StringPtrIfNotEmpty("https://backend-v2.internal/api")},
		},
	}
	updated, err := apiSvc.UpdateAPIByHandle(*apiUUID, updateReq, apiSecretITOrgUUID, "alice")
	if err != nil {
		t.Fatalf("UpdateAPI (URL-only) failed: %v", err)
	}
	if updated.Upstream.Main.Url == nil || *updated.Upstream.Main.Url != "https://backend-v2.internal/api" {
		t.Errorf("expected URL to be updated, got %v", updated.Upstream.Main.Url)
	}
	// Secret A must still be protected — proves auth wasn't wiped by the update.
	if err := secretSvc.Delete(apiSecretITOrgUUID, "it-secret-a", "alice"); err == nil {
		t.Fatal("expected secret A deletion to still be blocked after a URL-only update, got nil error")
	}

	// --- Rotation cleanup: switch auth to secret B ---
	rotateReq := &api.RESTAPI{
		DisplayName: "IT REST API",
		Context:     "/it-rest-api",
		Version:     "v1",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{
				Url: utils.StringPtrIfNotEmpty("https://backend-v2.internal/api"),
				Auth: &api.UpstreamAuth{
					Type:   upstreamAuthTypePtr("bearer"),
					Header: ptr("Authorization"),
					Value:  ptr(`{{ secret "it-secret-b" }}`),
				},
			},
		},
	}
	if _, err := apiSvc.UpdateAPIByHandle(*apiUUID, rotateReq, apiSecretITOrgUUID, "alice"); err != nil {
		t.Fatalf("UpdateAPI (rotate) failed: %v", err)
	}

	secretA, err := secretSvc.Get(apiSecretITOrgUUID, "it-secret-a")
	if err != nil {
		t.Fatalf("failed to fetch secret A after rotation: %v", err)
	}
	if secretA.Status != string(model.SecretStatusDeprecated) {
		t.Errorf("expected secret A to be deprecated after rotation, got status=%q", secretA.Status)
	}
	secretB, err := secretSvc.Get(apiSecretITOrgUUID, "it-secret-b")
	if err != nil {
		t.Fatalf("failed to fetch secret B after rotation: %v", err)
	}
	if secretB.Status != string(model.SecretStatusActive) {
		t.Errorf("expected secret B to remain active after rotation, got status=%q", secretB.Status)
	}
	// Secret A is no longer referenced, so it can now be hard-deleted.
	if err := secretSvc.Delete(apiSecretITOrgUUID, "it-secret-a", "alice"); err != nil {
		t.Errorf("expected secret A to be deletable after rotation freed it, got: %v", err)
	}

	// --- Validation: a placeholder in upstream.auth that doesn't resolve is rejected ---
	badUpstreamReq := &api.RESTAPI{
		DisplayName: "IT REST API",
		Context:     "/it-rest-api",
		Version:     "v1",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{
				Url: utils.StringPtrIfNotEmpty("https://backend-v2.internal/api"),
				Auth: &api.UpstreamAuth{
					Type:  upstreamAuthTypePtr("bearer"),
					Value: ptr(`{{ secret "nonexistent-handle" }}`),
				},
			},
		},
	}
	if _, err := apiSvc.UpdateAPIByHandle(*apiUUID, badUpstreamReq, apiSecretITOrgUUID, "alice"); err == nil {
		t.Fatal("expected update with a nonexistent upstream secret ref to be rejected, got nil error")
	} else if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected a validation error for missing secret ref, got: %v", err)
	}

	// --- Validation: the same check covers policy params, not just upstream ---
	params := map[string]interface{}{"password": `{{ secret "nonexistent-policy-handle" }}`}
	badPolicyReq := &api.RESTAPI{
		DisplayName: "IT REST API",
		Context:     "/it-rest-api",
		Version:     "v1",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{Url: utils.StringPtrIfNotEmpty("https://backend-v2.internal/api")},
		},
		Policies: &[]api.Policy{{Name: "basic-auth", Version: "v1", Params: &params}},
	}
	if _, err := apiSvc.UpdateAPIByHandle(*apiUUID, badPolicyReq, apiSecretITOrgUUID, "alice"); err == nil {
		t.Fatal("expected update with a nonexistent policy secret ref to be rejected, got nil error")
	} else if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected a validation error for missing secret ref, got: %v", err)
	}
}

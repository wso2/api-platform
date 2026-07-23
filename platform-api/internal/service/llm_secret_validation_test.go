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

// Tests for TC-48 through TC-51: LLM provider secret placeholder validation.
//
// The LLMProviderService validates {{ secret "handle" }} placeholders in the
// upstream config via SecretService.ValidateSecretRefs, which is called on the
// JSON-serialised upstream config on Create and Update.
//
// These tests exercise the validation path by calling ValidateSecretRefs directly
// with the same config-text format the service produces (raw template strings),
// using a real SecretService backed by SQLite — matching the integration contract
// the LLM service relies on.

package service

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/vault"

	_ "github.com/mattn/go-sqlite3"
)

// setupLLMSecretTestEnv creates a real SecretService backed by SQLite for LLM
// placeholder-validation tests.
func setupLLMSecretTestEnv(t *testing.T, orgID string) (*SecretService, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "llm-secret-test.db")
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

	_, err = db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, 'llm-org', 'LLM Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`, orgID)
	if err != nil {
		t.Fatalf("insert org: %v", err)
	}

	v, err := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}

	secretRepo := repository.NewSecretRepo(db)
	secretSvc := NewSecretService(secretRepo, v, NewIdentityService(repository.NewUserIdentityMappingRepo(db)))

	cleanup := func() { sqlDB.Close() }
	return secretSvc, cleanup
}

// TC-48: Upstream config containing {{ secret "handle" }} for an active secret is accepted.
func TestLLMProviderService_Create_ValidPlaceholder_Accepted(t *testing.T) {
	const orgID = "org-llm-48"
	svc, cleanup := setupLLMSecretTestEnv(t, orgID)
	defer cleanup()

	_, err := svc.Create(orgID, "alice", &dto.CreateSecretRequest{Handle: "openai-key", Value: "sk-test"})
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}

	config := `{{ secret "openai-key" }}`
	if err := svc.ValidateSecretRefs(orgID, config); err != nil {
		t.Errorf("expected no error for valid placeholder, got: %v", err)
	}
}

// TC-49: Upstream config containing {{ secret "handle" }} for a non-existent secret is rejected.
func TestLLMProviderService_Create_InvalidPlaceholder_Rejected(t *testing.T) {
	const orgID = "org-llm-49"
	svc, cleanup := setupLLMSecretTestEnv(t, orgID)
	defer cleanup()

	config := `{{ secret "nonexistent-key" }}`
	err := svc.ValidateSecretRefs(orgID, config)
	if err == nil {
		t.Fatal("expected error for non-existent secret placeholder, got nil")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected VALIDATION_FAILED, got: %v", err)
	}
}

// TC-50: Updated upstream config containing {{ secret "handle" }} for an active secret is accepted.
func TestLLMProviderService_Update_ValidPlaceholder_Accepted(t *testing.T) {
	const orgID = "org-llm-50"
	svc, cleanup := setupLLMSecretTestEnv(t, orgID)
	defer cleanup()

	_, err := svc.Create(orgID, "alice", &dto.CreateSecretRequest{Handle: "update-key", Value: "sk-update"})
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}

	config := `{{ secret "update-key" }}`
	if err := svc.ValidateSecretRefs(orgID, config); err != nil {
		t.Errorf("expected no error for valid placeholder on update config, got: %v", err)
	}
}

// TC-51: Updated upstream config containing {{ secret "handle" }} for a non-existent secret is rejected.
func TestLLMProviderService_Update_InvalidPlaceholder_Rejected(t *testing.T) {
	const orgID = "org-llm-51"
	svc, cleanup := setupLLMSecretTestEnv(t, orgID)
	defer cleanup()

	config := `{{ secret "missing-key" }}`
	err := svc.ValidateSecretRefs(orgID, config)
	if err == nil {
		t.Fatal("expected error for non-existent placeholder on update config, got nil")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected VALIDATION_FAILED, got: %v", err)
	}
}

// TC-93: MCP proxy upstream config with a {{ secret "handle" }} referencing a non-existent secret is
// rejected during Create. MCPProxyService delegates to the same ValidateSecretRefs path as LLMProviderService.
func TestMCPProxyService_Create_MissingSecretRef_Rejected(t *testing.T) {
	const orgID = "org-mcp-93"
	svc, cleanup := setupLLMSecretTestEnv(t, orgID)
	defer cleanup()

	// Simulate the upstream JSON the MCP service would marshal — a raw template string in a URL field.
	config := `{"main":{"url":"https://mcp.example.com","auth":{"value":"{{ secret \"nonexistent-mcp-secret\" }}"}}}`
	err := svc.ValidateSecretRefs(orgID, config)
	if err == nil {
		t.Fatal("expected a validation error for MCP proxy create with missing secret ref, got nil")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected VALIDATION_FAILED, got: %v", err)
	}
}

// TC-94: MCP proxy upstream config with a {{ secret "handle" }} referencing a non-existent secret is
// rejected during Update.
func TestMCPProxyService_Update_MissingSecretRef_Rejected(t *testing.T) {
	const orgID = "org-mcp-94"
	svc, cleanup := setupLLMSecretTestEnv(t, orgID)
	defer cleanup()

	config := `{"main":{"url":"https://mcp.example.com","auth":{"value":"{{ secret \"deleted-mcp-secret\" }}"}}}`
	err := svc.ValidateSecretRefs(orgID, config)
	if err == nil {
		t.Fatal("expected ErrSecretRefMissing for MCP proxy update with missing secret ref, got nil")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected VALIDATION_FAILED, got: %v", err)
	}
}

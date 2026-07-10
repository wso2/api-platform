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

// Integration test for MCP Proxy rotation cleanup, backed by a real SQLite
// DB. No mockMCPProxyRepo exists in this package (unlike LLM Provider/Proxy),
// so this uses the real repository layer instead of hand-rolling one —
// it also exercises the real artifact_secret_refs tracking, which a mock
// cannot.

package service

import (
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/platform-api/internal/vault"

	_ "github.com/mattn/go-sqlite3"
)

const mcpSecretITOrgUUID = "org-mcp-secret-it"

// TestMCPProxyServiceUpdate_CleansUpRotatedSecret_Integration proves rotating
// an MCP proxy's upstream credential deprecates the secret it replaced,
// against a real DB — the same cleanupRotatedSecret path LLM Provider,
// LLM Proxy, and REST API are also tested against.
func TestMCPProxyServiceUpdate_CleansUpRotatedSecret_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "mcp-secret-it.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
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
		VALUES (?, 'mcp-secret-it-org', 'MCP Secret IT Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`, mcpSecretITOrgUUID); err != nil {
		t.Fatalf("insert org: %v", err)
	}

	v, err := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	identity := NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	secretRepo := repository.NewSecretRepo(db)
	secretSvc := NewSecretService(secretRepo, v, identity)

	createTestSecret(t, secretSvc, mcpSecretITOrgUUID, "mcp-it-secret-a", "sk-mcp-real-token-A")
	createTestSecret(t, secretSvc, mcpSecretITOrgUUID, "mcp-it-secret-b", "sk-mcp-real-token-B")

	mcpRepo := repository.NewMCPProxyRepo(db)
	mcpSvc := NewMCPProxyService(mcpRepo, nil, nil, nil, nil, slog.Default(), repository.NewAuditRepo(db), &config.Server{}, identity)
	mcpSvc.WithSecretService(secretSvc)

	createReq := &api.MCPProxy{
		DisplayName: "IT MCP Proxy",
		Version:     "v1.0",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{
				Url: utils.StringPtrIfNotEmpty("https://mcp-backend.internal"),
				Auth: &api.UpstreamAuth{
					Type:   upstreamAuthTypePtr("bearer"),
					Header: ptr("Authorization"),
					Value:  ptr(`{{ secret "mcp-it-secret-a" }}`),
				},
			},
		},
	}
	created, err := mcpSvc.Create(mcpSecretITOrgUUID, "alice", createReq)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rotateReq := &api.MCPProxy{
		DisplayName: "IT MCP Proxy",
		Version:     "v1.0",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{
				Url: utils.StringPtrIfNotEmpty("https://mcp-backend.internal"),
				Auth: &api.UpstreamAuth{
					Type:   upstreamAuthTypePtr("bearer"),
					Header: ptr("Authorization"),
					Value:  ptr(`{{ secret "mcp-it-secret-b" }}`),
				},
			},
		},
	}
	if _, err := mcpSvc.Update(mcpSecretITOrgUUID, *created.Id, "alice", rotateReq); err != nil {
		t.Fatalf("Update (rotate) failed: %v", err)
	}

	secretA, err := secretSvc.Get(mcpSecretITOrgUUID, "mcp-it-secret-a")
	if err != nil {
		t.Fatalf("failed to fetch secret A after rotation: %v", err)
	}
	if secretA.Status != string(model.SecretStatusDeprecated) {
		t.Errorf("expected secret A to be deprecated after rotation, got status=%q", secretA.Status)
	}
	secretB, err := secretSvc.Get(mcpSecretITOrgUUID, "mcp-it-secret-b")
	if err != nil {
		t.Fatalf("failed to fetch secret B after rotation: %v", err)
	}
	if secretB.Status != string(model.SecretStatusActive) {
		t.Errorf("expected secret B to remain active after rotation, got status=%q", secretB.Status)
	}
}

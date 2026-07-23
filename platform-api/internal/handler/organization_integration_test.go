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

// Tests GET /api/v0.9/organizations: membership-filtered listing for ordinary
// callers, list-all for callers with the ap:organization:manage scope, and
// the lazy membership heal that makes a pre-existing organization (e.g. one
// seeded before the caller ever registered) visible on first list.

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
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"

	_ "github.com/mattn/go-sqlite3"
)

// setupOrganizationTestEnv creates a full OrganizationHandler stack backed by
// SQLite, along with the underlying DB for direct fixture setup.
func setupOrganizationHandlerTestEnv(t *testing.T) (http.Handler, *database.DB, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "org-handler-test.db")
	// Enable foreign-key enforcement for every pooled connection via the DSN,
	// not just the one connection a PRAGMA Exec would happen to run on.
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db := &database.DB{DB: sqlDB}

	schemaPath := filepath.Join("..", "database", "schema.sqlite.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	identityService := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	orgRepo := repository.NewOrganizationRepo(db)
	orgService := service.NewOrganizationService(
		orgRepo,
		repository.NewProjectRepo(db),
		nil, // applicationRepo — unused by RegisterOrganization/ListOrganizations
		nil, // apiRepo
		nil, // gatewayRepo
		nil, // llmProviderRepo
		nil, // llmProxyRepo
		nil, // mcpProxyRepo
		nil, // llmTemplateSeeder — nil-checked, best-effort
		noopAudit{},
		repository.NewUserOrganizationMappingRepo(db),
		identityService,
		&config.Server{},
		slog.Default(),
	)

	h := NewOrganizationHandler(orgService, identityService, middleware.ValidationModeScope, slog.Default())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	cleanup := func() { sqlDB.Close() }
	return middleware.NewTestContextMiddleware(mux), db, cleanup
}

type organizationListResponse struct {
	Count int `json:"count"`
	List  []struct {
		Id string `json:"id"`
	} `json:"list"`
	Pagination struct {
		Total int `json:"total"`
	} `json:"pagination"`
}

func TestOrganizationHandler_ListOrganizations_MembershipFiltered(t *testing.T) {
	r, db, cleanup := setupOrganizationHandlerTestEnv(t)
	t.Cleanup(cleanup)

	// A pre-existing org with no membership rows (mirrors the file-based
	// seeded org, or any org created before this user ever interacted with it).
	orgRepo := repository.NewOrganizationRepo(db)
	if err := orgRepo.CreateOrganization(&model.Organization{ID: "org-other", Handle: "other-org", Name: "Other Org", Region: "us"}); err != nil {
		t.Fatalf("failed to seed org-other: %v", err)
	}

	// Caller registers their own org via the handler.
	registerBody, err := json.Marshal(map[string]any{
		"displayName": "My Org",
		"region":      "us",
	})
	if err != nil {
		t.Fatalf("failed to marshal register body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v0.9/organizations", bytes.NewReader(registerBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", "sub-member")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("RegisterOrganization: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Listing as the same non-admin caller must show only their own org, not org-other.
	listReq := httptest.NewRequest(http.MethodGet, "/api/v0.9/organizations", nil)
	listReq.Header.Set("X-Test-User", "sub-member")
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("ListOrganizations: expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var body organizationListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Count != 1 || body.Pagination.Total != 1 {
		t.Fatalf("expected exactly 1 visible org for a non-admin member, got count=%d total=%d body=%s",
			body.Count, body.Pagination.Total, listRec.Body.String())
	}
	if len(body.List) != 1 || body.List[0].Id == "other-org" {
		t.Fatalf("expected the caller's own org, not org-other: %+v", body.List)
	}
}

func TestOrganizationHandler_ListOrganizations_ManageScopeSeesAll(t *testing.T) {
	r, db, cleanup := setupOrganizationHandlerTestEnv(t)
	t.Cleanup(cleanup)

	orgRepo := repository.NewOrganizationRepo(db)
	for _, id := range []string{"org-a", "org-b", "org-c"} {
		if err := orgRepo.CreateOrganization(&model.Organization{ID: id, Handle: id, Name: id, Region: "us"}); err != nil {
			t.Fatalf("failed to seed %s: %v", id, err)
		}
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v0.9/organizations", nil)
	listReq.Header.Set("X-Test-User", "sub-admin")
	listReq.Header.Set("X-Test-Scope", "ap:organization:manage")
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("ListOrganizations: expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var body organizationListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Count != 3 || body.Pagination.Total != 3 {
		t.Fatalf("expected all 3 orgs visible to an ap:organization:manage caller, got count=%d total=%d body=%s",
			body.Count, body.Pagination.Total, listRec.Body.String())
	}
}

func TestOrganizationHandler_ListOrganizations_HealsMembershipForResolvedOrg(t *testing.T) {
	r, db, cleanup := setupOrganizationHandlerTestEnv(t)
	t.Cleanup(cleanup)

	orgRepo := repository.NewOrganizationRepo(db)
	if err := orgRepo.CreateOrganization(&model.Organization{ID: "org-seeded", Handle: "seeded-org", Name: "Seeded Org", Region: "us"}); err != nil {
		t.Fatalf("failed to seed org-seeded: %v", err)
	}

	// Simulate OrganizationResolverMiddleware having resolved the caller's
	// token org claim to this pre-existing org's platform UUID.
	listReq := httptest.NewRequest(http.MethodGet, "/api/v0.9/organizations", nil)
	listReq.Header.Set("X-Test-User", "sub-seeded-caller")
	listReq.Header.Set("X-Test-Org", "org-seeded")
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("ListOrganizations: expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var body organizationListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Count != 1 || len(body.List) != 1 || body.List[0].Id != "seeded-org" {
		t.Fatalf("expected the resolved org to be healed into view, got %+v", body)
	}

	identityService := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	userUUID, err := identityService.ToInternalUUID("sub-seeded-caller")
	if err != nil {
		t.Fatalf("ToInternalUUID failed: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_organization_mappings WHERE user_uuid = ? AND org_uuid = ?`,
		userUUID, "org-seeded").Scan(&count); err != nil {
		t.Fatalf("failed to query membership: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected the heal to have created a membership row, got count=%d", count)
	}
}

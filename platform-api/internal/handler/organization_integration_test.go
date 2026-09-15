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

// Tests GET /api/v0.9/organizations: claim-filtered listing, list-all for
// ap:organization:manage callers, and the membership heal on first list.

package handler

import (
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

	h := NewOrganizationHandler(orgService, identityService, slog.Default())
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

func TestOrganizationHandler_ListOrganizations_ClaimFilteredByOrganizationsClaim(t *testing.T) {
	r, db, cleanup := setupOrganizationHandlerTestEnv(t)
	t.Cleanup(cleanup)

	orgRepo := repository.NewOrganizationRepo(db)
	for _, h := range []string{"org-a", "org-b", "org-c"} {
		if err := orgRepo.CreateOrganization(&model.Organization{ID: "id-" + h, Handle: h, Name: h, Region: "us"}); err != nil {
			t.Fatalf("failed to seed %s: %v", h, err)
		}
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v0.9/organizations", nil)
	listReq.Header.Set("X-Test-User", "sub-member")
	listReq.Header.Set("X-Test-Organizations", "org-a org-c")
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("ListOrganizations: expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var body organizationListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Count != 2 || body.Pagination.Total != 2 {
		t.Fatalf("expected exactly 2 orgs matching the organizations claim, got count=%d total=%d body=%s",
			body.Count, body.Pagination.Total, listRec.Body.String())
	}
	for _, org := range body.List {
		if org.Id == "org-b" {
			t.Fatalf("expected org-b to be excluded (not in the organizations claim): %+v", body.List)
		}
	}
}

func TestOrganizationHandler_ListOrganizations_FallsBackToOrganizationClaimWhenOrganizationsClaimAbsent(t *testing.T) {
	r, db, cleanup := setupOrganizationHandlerTestEnv(t)
	t.Cleanup(cleanup)

	orgRepo := repository.NewOrganizationRepo(db)
	for _, h := range []string{"org-a", "org-b"} {
		if err := orgRepo.CreateOrganization(&model.Organization{ID: "id-" + h, Handle: h, Name: h, Region: "us"}); err != nil {
			t.Fatalf("failed to seed %s: %v", h, err)
		}
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v0.9/organizations", nil)
	listReq.Header.Set("X-Test-User", "sub-member")
	listReq.Header.Set("X-Test-Org-Handle", "org-a")
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
		t.Fatalf("expected exactly 1 visible org falling back to the organization claim, got count=%d total=%d body=%s",
			body.Count, body.Pagination.Total, listRec.Body.String())
	}
	if len(body.List) != 1 || body.List[0].Id != "org-a" {
		t.Fatalf("expected only org-a (the current-org claim), got %+v", body.List)
	}
}

func TestOrganizationHandler_ListOrganizations_NoOrgClaimsYieldsEmptyList(t *testing.T) {
	r, db, cleanup := setupOrganizationHandlerTestEnv(t)
	t.Cleanup(cleanup)

	orgRepo := repository.NewOrganizationRepo(db)
	if err := orgRepo.CreateOrganization(&model.Organization{ID: "id-org-a", Handle: "org-a", Name: "org-a", Region: "us"}); err != nil {
		t.Fatalf("failed to seed org-a: %v", err)
	}

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
	if body.Count != 0 || body.Pagination.Total != 0 {
		t.Fatalf("expected zero visible orgs with no org claims present, got count=%d total=%d body=%s",
			body.Count, body.Pagination.Total, listRec.Body.String())
	}
}

func TestOrganizationHandler_ListOrganizations_ManageScopeStillClaimFiltered(t *testing.T) {
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
	listReq.Header.Set("X-Test-Org-Handle", "org-a")
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("ListOrganizations: expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var body organizationListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Count != 1 || body.Pagination.Total != 1 || body.List[0].Id != "org-a" {
		t.Fatalf("expected ap:organization:manage to still be claim-filtered to org-a only, got count=%d total=%d body=%s",
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

	listReq := httptest.NewRequest(http.MethodGet, "/api/v0.9/organizations", nil)
	listReq.Header.Set("X-Test-User", "sub-seeded-caller")
	listReq.Header.Set("X-Test-Org", "org-seeded")
	listReq.Header.Set("X-Test-Org-Handle", "seeded-org")
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

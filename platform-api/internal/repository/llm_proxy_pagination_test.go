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

package repository

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

// createTestLLMProxyFixtureProvider creates a template + provider in orgUUID
// and returns the provider's UUID, which every LLM proxy fixture needs as its
// ProviderUUID foreign key.
func createTestLLMProxyFixtureProvider(t *testing.T, db *database.DB, orgUUID string) string {
	t.Helper()
	templateRepo := NewLLMProviderTemplateRepo(db)
	template := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               "template-" + orgUUID,
		GroupID:          "template-" + orgUUID,
		Name:             "Test Template",
		ManagedBy:        "wso2",
		Version:          "v1.0",
	}
	if err := templateRepo.Create(template); err != nil {
		t.Fatalf("failed to create template fixture: %v", err)
	}

	providerRepo := NewLLMProviderRepo(db)
	provider := &model.LLMProvider{
		OrganizationUUID: orgUUID,
		ID:               "provider-" + orgUUID,
		Name:             "Test Provider",
		Version:          "v1.0",
		TemplateUUID:     template.UUID,
		CreatedBy:        "test-user",
		UpdatedBy:        "test-user",
	}
	if err := providerRepo.Create(provider); err != nil {
		t.Fatalf("failed to create provider fixture: %v", err)
	}
	return provider.UUID
}

func createTestLLMProxy(t *testing.T, repo LLMProxyRepository, orgUUID, projectUUID, providerUUID, handle, name string) {
	t.Helper()
	proxy := &model.LLMProxy{
		OrganizationUUID: orgUUID,
		ProjectUUID:      projectUUID,
		ID:               handle,
		Name:             name,
		Version:          "v1.0",
		ProviderUUID:     providerUUID,
		CreatedBy:        "test-user",
		UpdatedBy:        "test-user",
	}
	if err := repo.Create(proxy); err != nil {
		t.Fatalf("failed to create LLM proxy fixture %q: %v", handle, err)
	}
}

// TestLLMProxyRepo_ListByProject_PaginationAndSearch exercises the search +
// pagination added to the project-scoped list, mirroring the MCP Proxy fix in
// the same PR.
func TestLLMProxyRepo_ListByProject_PaginationAndSearch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-llm-proxy-pagination"
	projectUUID := "project-llm-proxy-pagination"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)
	providerUUID := createTestLLMProxyFixtureProvider(t, db, orgUUID)

	repo := NewLLMProxyRepo(db)
	createTestLLMProxy(t, repo, orgUUID, projectUUID, providerUUID, "weather-proxy", "Weather Proxy")
	createTestLLMProxy(t, repo, orgUUID, projectUUID, providerUUID, "billing-proxy", "Billing Proxy")
	createTestLLMProxy(t, repo, orgUUID, projectUUID, providerUUID, "support-proxy", "Support Proxy")

	page, err := repo.ListByProject(orgUUID, projectUUID, ListOptions{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("ListByProject failed: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("expected a 2-item page, got %d — limit was not applied", len(page))
	}

	secondPage, err := repo.ListByProject(orgUUID, projectUUID, ListOptions{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("ListByProject (offset) failed: %v", err)
	}
	if len(secondPage) != 1 {
		t.Fatalf("expected the 1 remaining item on the second page, got %d — offset was not applied", len(secondPage))
	}

	results, err := repo.ListByProject(orgUUID, projectUUID, ListOptions{Limit: 20, Offset: 0, Search: "weather"})
	if err != nil {
		t.Fatalf("ListByProject with search failed: %v", err)
	}
	if len(results) != 1 || results[0].ID != "weather-proxy" {
		t.Fatalf("expected only the weather-proxy match, got %+v", results)
	}

	total, err := repo.CountByProject(orgUUID, projectUUID, "")
	if err != nil {
		t.Fatalf("CountByProject failed: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}

	filtered, err := repo.CountByProject(orgUUID, projectUUID, "billing")
	if err != nil {
		t.Fatalf("CountByProject with search failed: %v", err)
	}
	if filtered != 1 {
		t.Fatalf("expected filtered count 1 for 'billing', got %d", filtered)
	}
}

// TestLLMProxyRepo_List_And_Count_PaginationAndSearch exercises the org-level
// (non-project-scoped) List/Count the same way.
func TestLLMProxyRepo_List_And_Count_PaginationAndSearch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-llm-proxy-org-level"
	projectUUID := "project-llm-proxy-org-level"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)
	providerUUID := createTestLLMProxyFixtureProvider(t, db, orgUUID)

	repo := NewLLMProxyRepo(db)
	createTestLLMProxy(t, repo, orgUUID, projectUUID, providerUUID, "alpha-proxy", "Alpha Proxy")
	createTestLLMProxy(t, repo, orgUUID, projectUUID, providerUUID, "beta-proxy", "Beta Proxy")

	page, err := repo.List(orgUUID, ListOptions{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected a 1-item page, got %d", len(page))
	}

	total, err := repo.Count(orgUUID, "")
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}

	filtered, err := repo.Count(orgUUID, "alpha")
	if err != nil {
		t.Fatalf("Count with search failed: %v", err)
	}
	if filtered != 1 {
		t.Fatalf("expected filtered count 1 for 'alpha', got %d", filtered)
	}

	searchedPage, err := repo.List(orgUUID, ListOptions{Limit: 20, Offset: 0, Search: "alpha"})
	if err != nil {
		t.Fatalf("List with search failed: %v", err)
	}
	if len(searchedPage) != 1 || searchedPage[0].ID != "alpha-proxy" {
		t.Fatalf("expected only the alpha-proxy match, got %+v", searchedPage)
	}
}

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

	"github.com/wso2/api-platform/platform-api/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

func createTestMCPProxy(t *testing.T, repo *MCPProxyRepo, orgUUID, projectUUID, handle, name string) {
	t.Helper()
	proxy := &model.MCPProxy{
		OrganizationUUID: orgUUID,
		ProjectUUID:      &projectUUID,
		Handle:           handle,
		Name:             name,
		Version:          "v1.0",
		CreatedBy:        "test-user",
		UpdatedBy:        "test-user",
	}
	if err := repo.Create(proxy); err != nil {
		t.Fatalf("failed to create MCP proxy fixture %q: %v", handle, err)
	}
}

// TestMCPProxyRepo_ListByProject_HonorsPagination is the regression guard for
// the bug this PR fixes: ListByProject used to take no limit/offset at all and
// always returned every row for the project, silently ignoring the caller's
// page size — MCPProxyService.ListByProject's response claimed a bounded page
// while `resp.List` actually held the whole project.
func TestMCPProxyRepo_ListByProject_HonorsPagination(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-mcp-list-by-project"
	projectUUID := "project-mcp-list-by-project"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	repo := NewMCPProxyRepo(db)
	for i := 0; i < 5; i++ {
		createTestMCPProxy(t, repo, orgUUID, projectUUID, "mcp-"+string(rune('a'+i)), "MCP "+string(rune('A'+i)))
	}

	page, err := repo.ListByProject(orgUUID, projectUUID, ListOptions{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("ListByProject failed: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("expected a 2-item page, got %d items — limit was not applied", len(page))
	}

	secondPage, err := repo.ListByProject(orgUUID, projectUUID, ListOptions{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("ListByProject (offset) failed: %v", err)
	}
	if len(secondPage) != 2 {
		t.Fatalf("expected a 2-item second page, got %d items — offset was not applied", len(secondPage))
	}
	if page[0].UUID == secondPage[0].UUID {
		t.Fatalf("offset page returned the same first item as the first page — offset was not applied")
	}

	count, err := repo.CountByProject(orgUUID, projectUUID, "")
	if err != nil {
		t.Fatalf("CountByProject failed: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected total count 5 regardless of page size, got %d", count)
	}
}

// TestMCPProxyRepo_ListByProject_Search verifies the search clause added
// alongside pagination narrows both ListByProject and CountByProject to
// matching display names, case-insensitively.
func TestMCPProxyRepo_ListByProject_Search(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-mcp-search"
	projectUUID := "project-mcp-search"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	repo := NewMCPProxyRepo(db)
	createTestMCPProxy(t, repo, orgUUID, projectUUID, "weather-server", "Weather Server")
	createTestMCPProxy(t, repo, orgUUID, projectUUID, "billing-server", "Billing Server")

	results, err := repo.ListByProject(orgUUID, projectUUID, ListOptions{Limit: 20, Offset: 0, Search: "weather"})
	if err != nil {
		t.Fatalf("ListByProject with search failed: %v", err)
	}
	if len(results) != 1 || results[0].Handle != "weather-server" {
		t.Fatalf("expected only the weather-server match, got %+v", results)
	}

	count, err := repo.CountByProject(orgUUID, projectUUID, "weather")
	if err != nil {
		t.Fatalf("CountByProject with search failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1 for the search term, got %d", count)
	}
}

// TestMCPProxyRepo_List_And_Count_HonorPaginationAndSearch exercises the
// org-level (non-project-scoped) List/Count — the direct-SQL replacement for
// Count's old generic-artifact-table delegation, which could not filter by
// search.
func TestMCPProxyRepo_List_And_Count_HonorPaginationAndSearch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-mcp-org-level"
	projectUUID := "project-mcp-org-level"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	repo := NewMCPProxyRepo(db)
	createTestMCPProxy(t, repo, orgUUID, projectUUID, "alpha-server", "Alpha Server")
	createTestMCPProxy(t, repo, orgUUID, projectUUID, "beta-server", "Beta Server")
	createTestMCPProxy(t, repo, orgUUID, projectUUID, "gamma-server", "Gamma Server")

	page, err := repo.List(orgUUID, ListOptions{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("expected a 2-item page, got %d", len(page))
	}

	total, err := repo.Count(orgUUID, "")
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}

	filtered, err := repo.Count(orgUUID, "beta")
	if err != nil {
		t.Fatalf("Count with search failed: %v", err)
	}
	if filtered != 1 {
		t.Fatalf("expected filtered count 1 for 'beta', got %d", filtered)
	}

	searchedPage, err := repo.List(orgUUID, ListOptions{Limit: 20, Offset: 0, Search: "beta"})
	if err != nil {
		t.Fatalf("List with search failed: %v", err)
	}
	if len(searchedPage) != 1 || searchedPage[0].Handle != "beta-server" {
		t.Fatalf("expected only the beta-server match, got %+v", searchedPage)
	}
}

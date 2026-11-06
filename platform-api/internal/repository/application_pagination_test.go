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

func createTestApplication(t *testing.T, repo ApplicationRepository, orgUUID, projectUUID, handle, name, appType string) {
	t.Helper()
	app := &model.Application{
		UUID:             handle + "-uuid",
		Handle:           handle,
		ProjectUUID:      projectUUID,
		OrganizationUUID: orgUUID,
		Name:             name,
		Type:             appType,
		CreatedBy:        "test-user",
		UpdatedBy:        "test-user",
	}
	if err := repo.CreateApplication(app); err != nil {
		t.Fatalf("failed to create application fixture %q: %v", handle, err)
	}
}

// TestApplicationRepo_GetByProjectIDPaginated_FiltersByType is the
// repository-level guard for the GenAI Applications `type` filter added
// alongside server-side pagination: it must narrow both the page and the
// total to rows of exactly that type, and leave other types untouched when no
// filter is supplied.
func TestApplicationRepo_GetByProjectIDPaginated_FiltersByType(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-app-type-filter"
	projectUUID := "project-app-type-filter"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	repo := NewApplicationRepo(db, NewArtifactTableRegistry())
	createTestApplication(t, repo, orgUUID, projectUUID, "app-1", "App One", "genai")
	createTestApplication(t, repo, orgUUID, projectUUID, "app-2", "App Two", "other")
	createTestApplication(t, repo, orgUUID, projectUUID, "app-3", "App Three", "genai")

	genaiApps, err := repo.GetApplicationsByProjectIDPaginated(projectUUID, orgUUID, "genai", ListOptions{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("GetApplicationsByProjectIDPaginated failed: %v", err)
	}
	if len(genaiApps) != 2 {
		t.Fatalf("expected 2 genai applications, got %d", len(genaiApps))
	}
	for _, app := range genaiApps {
		if app.Type != "genai" {
			t.Fatalf("expected only genai-typed applications, got one of type %q", app.Type)
		}
	}

	genaiCount, err := repo.CountApplicationsByProjectID(projectUUID, orgUUID, "genai", "")
	if err != nil {
		t.Fatalf("CountApplicationsByProjectID failed: %v", err)
	}
	if genaiCount != 2 {
		t.Fatalf("expected filtered count 2, got %d", genaiCount)
	}

	allApps, err := repo.GetApplicationsByProjectIDPaginated(projectUUID, orgUUID, "", ListOptions{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("GetApplicationsByProjectIDPaginated (unfiltered) failed: %v", err)
	}
	if len(allApps) != 3 {
		t.Fatalf("expected all 3 applications with no type filter, got %d", len(allApps))
	}
}

// TestApplicationRepo_GetByProjectIDPaginated_Search verifies the search
// clause added alongside the type filter and pagination.
func TestApplicationRepo_GetByProjectIDPaginated_Search(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-app-search"
	projectUUID := "project-app-search"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	repo := NewApplicationRepo(db, NewArtifactTableRegistry())
	createTestApplication(t, repo, orgUUID, projectUUID, "weather-app", "Weather App", "genai")
	createTestApplication(t, repo, orgUUID, projectUUID, "billing-app", "Billing App", "genai")

	results, err := repo.GetApplicationsByProjectIDPaginated(projectUUID, orgUUID, "genai", ListOptions{Limit: 20, Offset: 0, Search: "weather"})
	if err != nil {
		t.Fatalf("GetApplicationsByProjectIDPaginated with search failed: %v", err)
	}
	if len(results) != 1 || results[0].Handle != "weather-app" {
		t.Fatalf("expected only the weather-app match, got %+v", results)
	}

	count, err := repo.CountApplicationsByProjectID(projectUUID, orgUUID, "genai", "weather")
	if err != nil {
		t.Fatalf("CountApplicationsByProjectID with search failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected filtered count 1, got %d", count)
	}
}

// TestApplicationRepo_GetByProjectIDPaginated_HonorsPagination verifies
// limit/offset narrow the page while the count stays unaffected.
func TestApplicationRepo_GetByProjectIDPaginated_HonorsPagination(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-app-pagination"
	projectUUID := "project-app-pagination"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	repo := NewApplicationRepo(db, NewArtifactTableRegistry())
	for i := 0; i < 5; i++ {
		createTestApplication(t, repo, orgUUID, projectUUID, "app-"+string(rune('a'+i)), "App "+string(rune('A'+i)), "genai")
	}

	page, err := repo.GetApplicationsByProjectIDPaginated(projectUUID, orgUUID, "", ListOptions{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("GetApplicationsByProjectIDPaginated failed: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("expected a 2-item page, got %d", len(page))
	}

	secondPage, err := repo.GetApplicationsByProjectIDPaginated(projectUUID, orgUUID, "", ListOptions{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("GetApplicationsByProjectIDPaginated (offset) failed: %v", err)
	}
	if len(secondPage) != 2 {
		t.Fatalf("expected a 2-item second page, got %d", len(secondPage))
	}
	if page[0].UUID == secondPage[0].UUID {
		t.Fatalf("offset page returned the same first item as the first page — offset was not applied")
	}

	total, err := repo.CountApplicationsByProjectID(projectUUID, orgUUID, "", "")
	if err != nil {
		t.Fatalf("CountApplicationsByProjectID failed: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total 5 regardless of page size, got %d", total)
	}
}

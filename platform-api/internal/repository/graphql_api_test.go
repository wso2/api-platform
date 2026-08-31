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
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

// This file is the GraphQL counterpart to api_test.go — real SQLite (via
// setupTestDB/setupTestDBWithoutForeignKeys, shared with api_deployments_test.go),
// not the mock repo used by internal/service/graphql_api_test.go. Mirrors the
// same coverage REST APIs already have at this layer, since the mock-repo
// service tests can't catch a broken SQL query, a wrong column mapping, or a
// missed artifact-row insert.

func newTestGraphQLAPI(handle, orgUUID, projectUUID string) *model.GraphQLAPI {
	return &model.GraphQLAPI{
		Handle:         handle,
		Name:           "Countries GraphQL API",
		Version:        "v1.0",
		Description:    "Test GraphQL API",
		CreatedBy:      "test-user",
		UpdatedBy:      "test-user",
		ProjectID:      projectUUID,
		OrganizationID: orgUUID,
		Configuration: model.GraphQLAPIConfig{
			Name:              "Countries GraphQL API",
			Version:           "v1.0",
			Context:           strPtr("/countries/$version"),
			SDL:               "type Query { countries: [Country] }\ntype Country { code: String name: String }",
			IntrospectionMode: "SDL",
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: "https://countries.trevorblades.com/graphql",
				},
			},
			Policies: []model.Policy{
				{Name: "jwt-auth", Version: "v1"},
			},
			SubscriptionPlans: []string{"Gold", "Silver"},
		},
	}
}

func TestGraphQLAPIRepo_CreateAndRead(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-crud-001"
	projectUUID := "project-graphql-crud-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := newTestGraphQLAPI("countries-graphql", orgUUID, projectUUID)

	if err := repo.Create(api); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if api.ID == "" {
		t.Fatal("Create should set api.ID")
	}

	created, err := repo.GetByUUID(api.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID failed: %v", err)
	}
	if created == nil {
		t.Fatal("GetByUUID returned nil")
	}

	if created.Handle != api.Handle || created.Name != api.Name || created.Version != api.Version {
		t.Fatalf("GetByUUID returned unexpected metadata: %+v", created)
	}
	if created.Description != api.Description || created.CreatedBy != api.CreatedBy || created.ProjectID != api.ProjectID {
		t.Fatalf("GetByUUID returned unexpected details: %+v", created)
	}
	if created.OrganizationID != api.OrganizationID {
		t.Fatalf("GetByUUID returned unexpected organization: %+v", created)
	}
	if created.UpdatedBy == "" {
		t.Fatal("expected updated_by to be set on creation, got empty string")
	}
}

// TestGraphQLAPIRepo_CreateAndRead_FullConfiguration is the GraphQL counterpart
// to TestAPIRepo_CreateAndRead_FullConfiguration — round-trips sdl,
// introspectionMode, upstream, policies, and subscriptionPlans through the
// configuration BLOB.
func TestGraphQLAPIRepo_CreateAndRead_FullConfiguration(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-crud-002"
	projectUUID := "project-graphql-crud-002"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := newTestGraphQLAPI("countries-graphql-full", orgUUID, projectUUID)

	if err := repo.Create(api); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	created, err := repo.GetByUUID(api.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID failed: %v", err)
	}
	if created == nil {
		t.Fatal("GetByUUID returned nil")
	}

	if !reflect.DeepEqual(created.Configuration, api.Configuration) {
		t.Fatalf("Full configuration mismatch. expected=%+v actual=%+v", api.Configuration, created.Configuration)
	}
}

// TestGraphQLAPIRepo_CreateSetsArtifactKind guards the artifact-type insertion
// behavior confirmed earlier in this session: Create must insert an artifacts
// row with type=GraphQLApi, exactly mirroring how rest_apis/Create sets
// type=RestApi (see constants.GraphQLApi usage in Create above).
func TestGraphQLAPIRepo_CreateSetsArtifactKind(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-kind-001"
	projectUUID := "project-graphql-kind-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := newTestGraphQLAPI("kind-graphql", orgUUID, projectUUID)
	if err := repo.Create(api); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var artifactType string
	err := db.QueryRow("SELECT type FROM artifacts WHERE uuid = ?", api.ID).Scan(&artifactType)
	if err != nil {
		t.Fatalf("failed to read artifact type: %v", err)
	}
	if artifactType != constants.GraphQLApi {
		t.Fatalf("expected artifact type %s, got %s", constants.GraphQLApi, artifactType)
	}
}

func TestGraphQLAPIRepo_GetByHandle(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-handle-001"
	projectUUID := "project-graphql-handle-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := newTestGraphQLAPI("handle-graphql", orgUUID, projectUUID)
	if err := repo.Create(api); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	found, err := repo.GetByHandle(api.Handle, orgUUID)
	if err != nil {
		t.Fatalf("GetByHandle failed: %v", err)
	}
	if found == nil || found.ID != api.ID {
		t.Fatalf("GetByHandle returned unexpected result: %+v", found)
	}

	notFound, err := repo.GetByHandle("does-not-exist", orgUUID)
	if err != nil {
		t.Fatalf("GetByHandle for unknown handle returned error: %v", err)
	}
	if notFound != nil {
		t.Fatalf("expected nil for unknown handle, got %+v", notFound)
	}
}

// TestGraphQLAPIRepo_CrossOrgIsolation guards GO-AUTH-005-style tenant
// isolation at the repository layer: a handle/UUID that exists in one org must
// never resolve when queried with a different org's UUID.
func TestGraphQLAPIRepo_CrossOrgIsolation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-iso-001"
	otherOrgUUID := "org-graphql-iso-002"
	projectUUID := "project-graphql-iso-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)
	createTestOrganizationAndProject(t, db, otherOrgUUID, "project-graphql-iso-002")

	api := newTestGraphQLAPI("iso-graphql", orgUUID, projectUUID)
	if err := repo.Create(api); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if found, err := repo.GetByHandle(api.Handle, otherOrgUUID); err != nil || found != nil {
		t.Fatalf("GetByHandle across orgs = (%+v, %v), want (nil, nil)", found, err)
	}
	if found, err := repo.GetByUUID(api.ID, otherOrgUUID); err != nil || found != nil {
		t.Fatalf("GetByUUID across orgs = (%+v, %v), want (nil, nil)", found, err)
	}
}

// TestGraphQLAPIRepo_CreateSameHandleDifferentOrgs_Succeeds is the mirror
// image of TestGraphQLAPIRepo_CrossOrgIsolation: the same handle string must
// be independently creatable in two different orgs (the uniqueness
// constraint is scoped to org_id, not global) — otherwise a tenant could be
// blocked from using a handle another, unrelated tenant already picked.
func TestGraphQLAPIRepo_CreateSameHandleDifferentOrgs_Succeeds(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-samehandle-001"
	otherOrgUUID := "org-graphql-samehandle-002"
	projectUUID := "project-graphql-samehandle-001"
	otherProjectUUID := "project-graphql-samehandle-002"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)
	createTestOrganizationAndProject(t, db, otherOrgUUID, otherProjectUUID)

	first := newTestGraphQLAPI("shared-handle", orgUUID, projectUUID)
	if err := repo.Create(first); err != nil {
		t.Fatalf("Create in first org failed: %v", err)
	}

	second := newTestGraphQLAPI("shared-handle", otherOrgUUID, otherProjectUUID)
	if err := repo.Create(second); err != nil {
		t.Fatalf("Create with the same handle in a different org should succeed, got: %v", err)
	}

	if found, err := repo.GetByHandle("shared-handle", orgUUID); err != nil || found == nil {
		t.Fatalf("GetByHandle in first org = (%+v, %v), want a result", found, err)
	}
	if found, err := repo.GetByHandle("shared-handle", otherOrgUUID); err != nil || found == nil {
		t.Fatalf("GetByHandle in second org = (%+v, %v), want a result", found, err)
	}
}

func TestGraphQLAPIRepo_List(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-list-001"
	projectUUID := "project-graphql-list-001"
	otherProjectUUID := "project-graphql-list-002"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	projectQuery := `INSERT INTO projects (uuid, handle, display_name, organization_uuid, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`
	if _, err := db.Exec(projectQuery, otherProjectUUID, "other-project-list-001", "Other Project", orgUUID); err != nil {
		t.Fatalf("failed to create second project: %v", err)
	}

	apiInProject := newTestGraphQLAPI("list-graphql-a", orgUUID, projectUUID)
	apiInOtherProject := newTestGraphQLAPI("list-graphql-b", orgUUID, otherProjectUUID)
	if err := repo.Create(apiInProject); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := repo.Create(apiInOtherProject); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	all, err := repo.List(orgUUID, "", ListOptions{Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("List (no project filter) failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 GraphQL APIs for org, got %d", len(all))
	}

	filtered, err := repo.List(orgUUID, projectUUID, ListOptions{Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("List (project filter) failed: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Handle != apiInProject.Handle {
		t.Fatalf("expected only %s scoped to project, got %+v", apiInProject.Handle, filtered)
	}

	otherOrg := "org-graphql-list-002"
	createTestOrganizationAndProject(t, db, otherOrg, "project-graphql-list-other-org")
	emptyList, err := repo.List(otherOrg, "", ListOptions{Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("List for a different org failed: %v", err)
	}
	if len(emptyList) != 0 {
		t.Fatalf("expected empty list for a different org, got %+v", emptyList)
	}
}

// TestGraphQLAPIRepo_List_PaginationBoundaries exercises an actual page
// boundary (limit smaller than the total row count, non-zero offset) —
// TestGraphQLAPIRepo_List only ever passes limit=100 against a 1-2 row
// dataset, which can't distinguish "pagination works" from "pagination is a
// no-op because nothing was ever truncated."
func TestGraphQLAPIRepo_List_PaginationBoundaries(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-page-001"
	projectUUID := "project-graphql-page-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	// Created in order a, b, c; List orders by created_at DESC, so the
	// expected page order is c, b, a.
	for _, handle := range []string{"page-graphql-a", "page-graphql-b", "page-graphql-c"} {
		if err := repo.Create(newTestGraphQLAPI(handle, orgUUID, projectUUID)); err != nil {
			t.Fatalf("Create %s failed: %v", handle, err)
		}
	}

	page1, err := repo.List(orgUUID, "", ListOptions{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("List (limit=1, offset=0) failed: %v", err)
	}
	if len(page1) != 1 || page1[0].Handle != "page-graphql-c" {
		t.Fatalf("expected page 1 = [page-graphql-c], got %+v", page1)
	}

	page2, err := repo.List(orgUUID, "", ListOptions{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("List (limit=1, offset=1) failed: %v", err)
	}
	if len(page2) != 1 || page2[0].Handle != "page-graphql-b" {
		t.Fatalf("expected page 2 = [page-graphql-b], got %+v", page2)
	}

	pastEnd, err := repo.List(orgUUID, "", ListOptions{Limit: 10, Offset: 3})
	if err != nil {
		t.Fatalf("List (offset past the end) failed: %v", err)
	}
	if len(pastEnd) != 0 {
		t.Fatalf("expected an empty page once offset exceeds the row count, got %+v", pastEnd)
	}
}

// TestGraphQLAPIRepo_List_Search pins the fix for the gap where List/
// CountByProject silently ignored the spec's documented query parameter: a
// search with no matching handle must return an empty result and a total of
// 0, not the whole unfiltered collection.
func TestGraphQLAPIRepo_List_Search(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-search-001"
	projectUUID := "project-graphql-search-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	if err := repo.Create(newTestGraphQLAPI("countries-graphql-api", orgUUID, projectUUID)); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	matched, err := repo.List(orgUUID, projectUUID, ListOptions{Limit: 100, Search: "countries"})
	if err != nil {
		t.Fatalf("List (matching search) failed: %v", err)
	}
	if len(matched) != 1 {
		t.Fatalf("expected 1 match for a search matching the handle, got %d", len(matched))
	}

	noMatch, err := repo.List(orgUUID, projectUUID, ListOptions{Limit: 100, Search: "zzz-no-match"})
	if err != nil {
		t.Fatalf("List (non-matching search) failed: %v", err)
	}
	if len(noMatch) != 0 {
		t.Fatalf("expected an empty result for a non-matching search, got %+v", noMatch)
	}

	total, err := repo.CountByProject(orgUUID, projectUUID, "zzz-no-match")
	if err != nil {
		t.Fatalf("CountByProject (non-matching search) failed: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected total 0 for a non-matching search, got %d", total)
	}
}

// TestGraphQLAPIRepo_List_SortBy pins sortBy=name changing the ordering
// (previously always ORDER BY created_at DESC regardless of the request),
// and that an unrecognized sortBy token falls back to the default order
// (matching the shared allowlist's documented fallback behavior) rather than
// erroring.
func TestGraphQLAPIRepo_List_SortBy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-sort-001"
	projectUUID := "project-graphql-sort-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	fixtures := []struct{ handle, name string }{
		{"sort-graphql-a", "Charlie API"},
		{"sort-graphql-b", "Alpha API"},
		{"sort-graphql-c", "Bravo API"},
	}
	for _, f := range fixtures {
		a := newTestGraphQLAPI(f.handle, orgUUID, projectUUID)
		a.Name = f.name
		if err := repo.Create(a); err != nil {
			t.Fatalf("Create %s failed: %v", f.handle, err)
		}
	}

	byNameAsc, err := repo.List(orgUUID, projectUUID, ListOptions{Limit: 100, SortBy: "name", SortOrder: "asc"})
	if err != nil {
		t.Fatalf("List (sortBy=name, asc) failed: %v", err)
	}
	if len(byNameAsc) != 3 || byNameAsc[0].Name != "Alpha API" || byNameAsc[1].Name != "Bravo API" || byNameAsc[2].Name != "Charlie API" {
		names := make([]string, len(byNameAsc))
		for i, a := range byNameAsc {
			names[i] = a.Name
		}
		t.Fatalf("expected [Alpha API, Bravo API, Charlie API], got %v", names)
	}

	// An unrecognized sortBy token must fall back to the default order
	// (created_at) rather than erroring or being interpolated into SQL —
	// creation order here is a, b, c, so default DESC order is c, b, a.
	fallback, err := repo.List(orgUUID, projectUUID, ListOptions{Limit: 100, SortBy: "not-a-real-column"})
	if err != nil {
		t.Fatalf("List (unrecognized sortBy) failed: %v", err)
	}
	if len(fallback) != 3 || fallback[0].Handle != "sort-graphql-c" || fallback[1].Handle != "sort-graphql-b" || fallback[2].Handle != "sort-graphql-a" {
		handles := make([]string, len(fallback))
		for i, a := range fallback {
			handles[i] = a.Handle
		}
		t.Fatalf("expected fallback to default created_at DESC order [c, b, a], got %v", handles)
	}
}

func TestGraphQLAPIRepo_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-update-001"
	projectUUID := "project-graphql-update-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := newTestGraphQLAPI("update-graphql", orgUUID, projectUUID)
	if err := repo.Create(api); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	api.Name = "Updated Countries API"
	api.Description = "Updated description"
	api.Configuration.SDL = "type Query { countries: [Country] country(code: ID!): Country }\ntype Country { code: String }"
	api.Configuration.IntrospectionMode = "ENDPOINT"

	if err := repo.Update(api); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, err := repo.GetByUUID(api.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID failed: %v", err)
	}
	if updated == nil {
		t.Fatal("GetByUUID returned nil")
	}
	if updated.Name != api.Name || updated.Description != api.Description {
		t.Fatalf("Update changes not persisted: %+v", updated)
	}
	if updated.Configuration.SDL != api.Configuration.SDL || updated.Configuration.IntrospectionMode != api.Configuration.IntrospectionMode {
		t.Fatalf("Update did not persist configuration changes: %+v", updated.Configuration)
	}
}

func TestGraphQLAPIRepo_Update_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-update-404"
	projectUUID := "project-graphql-update-404"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	ghost := newTestGraphQLAPI("does-not-exist", orgUUID, projectUUID)
	err := repo.Update(ghost)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Update on a non-existent handle = %v, want sql.ErrNoRows", err)
	}
}

func TestGraphQLAPIRepo_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-delete-001"
	projectUUID := "project-graphql-delete-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := newTestGraphQLAPI("delete-graphql", orgUUID, projectUUID)
	if err := repo.Create(api); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Delete(api.Handle, orgUUID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	deleted, err := repo.GetByUUID(api.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID failed: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected GraphQL API to be deleted, got: %+v", deleted)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM artifacts WHERE uuid = ?", api.ID).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("failed to verify artifact cleanup: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected artifact row to be removed, found %d", count)
	}

	exists, err := repo.Exists(api.Handle, orgUUID)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("expected handle to no longer exist after delete")
	}
}

// TestGraphQLAPIRepo_Delete_CascadesRelatedRows is the real cascade test
// TestGraphQLAPIRepo_Delete couldn't be: that test never creates any
// deployment or gateway-association rows, so its own "0 rows remain" check
// is trivially true whether or not ON DELETE CASCADE actually fires. This
// test seeds a deployment and an artifact_gateway_mappings row first, so the
// post-delete zero-count genuinely exercises the FK chain
// (deployments/artifact_gateway_mappings -> artifacts(uuid) ON DELETE CASCADE)
// rather than asserting over an empty table. This is the first cascade-delete
// test in the repo for any artifact kind.
func TestGraphQLAPIRepo_Delete_CascadesRelatedRows(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-cascade-001"
	projectUUID := "project-graphql-cascade-001"
	gatewayUUID := "gateway-graphql-cascade-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)
	createTestGateway(t, db, gatewayUUID, orgUUID)

	api := newTestGraphQLAPI("cascade-graphql", orgUUID, projectUUID)
	if err := repo.Create(api); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	insertDeployment(t, db, "deployment-graphql-cascade-001", "cascade-deployment", api.ID, orgUUID, gatewayUUID, time.Now())

	mappingQuery := `
		INSERT INTO artifact_gateway_mappings (artifact_uuid, organization_uuid, gateway_uuid, created_at, updated_at)
		VALUES (?, ?, ?, datetime('now'), datetime('now'))
	`
	if _, err := db.Exec(mappingQuery, api.ID, orgUUID, gatewayUUID); err != nil {
		t.Fatalf("failed to seed artifact_gateway_mappings: %v", err)
	}

	if err := repo.Delete(api.Handle, orgUUID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	for _, tbl := range []string{"artifacts", "graphql_apis", "deployments", "artifact_gateway_mappings"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM "+tbl+" WHERE "+cascadeFKColumn(tbl)+" = ?", api.ID).Scan(&count); err != nil {
			t.Fatalf("failed to verify %s cleanup: %v", tbl, err)
		}
		if count != 0 {
			t.Errorf("expected all %s rows for this artifact to be gone after delete, found %d", tbl, count)
		}
	}
}

// cascadeFKColumn returns the column each table keys its artifact reference
// by — "uuid" for the artifact's own primary-key tables, "artifact_uuid" for
// the generic child tables that reference it.
func cascadeFKColumn(table string) string {
	if table == "artifacts" || table == "graphql_apis" {
		return "uuid"
	}
	return "artifact_uuid"
}

func TestGraphQLAPIRepo_Delete_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-delete-404"
	projectUUID := "project-graphql-delete-404"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	err := repo.Delete("does-not-exist", orgUUID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Delete on a non-existent handle = %v, want sql.ErrNoRows", err)
	}
}

func TestGraphQLAPIRepo_Exists(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-exists-001"
	projectUUID := "project-graphql-exists-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := newTestGraphQLAPI("exists-graphql", orgUUID, projectUUID)
	if err := repo.Create(api); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	exists, err := repo.Exists(api.Handle, orgUUID)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected handle to exist")
	}

	exists, err = repo.Exists("unknown-handle", orgUUID)
	if err != nil {
		t.Fatalf("Exists for unknown handle failed: %v", err)
	}
	if exists {
		t.Fatal("expected unknown handle to not exist")
	}
}

// TestGraphQLAPIRepo_CreateRecordsArtifactSecretRefs guards the {{ secret "..." }}
// reference-tracking path shared with REST (upsertArtifactSecretRefs) — a
// GraphQL upstream auth value referencing a secret must be recorded the same
// way a REST API's would be, so the secret's "in use" delete-protection sees it.
func TestGraphQLAPIRepo_CreateRecordsArtifactSecretRefs(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewGraphQLAPIRepo(db, NewArtifactTableRegistry())

	orgUUID := "org-graphql-secretref-001"
	projectUUID := "project-graphql-secretref-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := newTestGraphQLAPI("secretref-graphql", orgUUID, projectUUID)
	api.Configuration.Upstream.Main.Auth = &model.UpstreamAuth{
		Type:   "header",
		Header: "Authorization",
		Value:  `{{ secret "upstream-token" }}`,
	}
	if err := repo.Create(api); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var refCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM artifact_secret_refs WHERE artifact_uuid = ? AND secret_handle = ?", api.ID, "upstream-token").Scan(&refCount); err != nil {
		t.Fatalf("failed to count artifact_secret_refs: %v", err)
	}
	if refCount == 0 {
		t.Fatal("expected an artifact_secret_refs row recording the {{ secret \"upstream-token\" }} reference")
	}
}

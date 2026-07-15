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

// Tests the "forceful user removal" flow described in the audit-fields
// follow-up to #2371/#2370: when the user_idp_references row backing a
// created_by/updated_by UUID is deleted (or the UUID is repointed), audit
// identity fields must resolve to constants.DeletedUser instead of leaking
// the raw internal UUID or erroring.

package service

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	_ "github.com/mattn/go-sqlite3"
)

// setupIdentityTestDB creates a real SQLite-backed DB plus a real
// IdentityService for tests that need genuine sub<->UUID mapping rows
// (as opposed to the passthroughIdentityRepo used elsewhere in this package).
func setupIdentityTestDB(t *testing.T) (*database.DB, *IdentityService, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "identity-test.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
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

	identity := NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	cleanup := func() { sqlDB.Close() }
	return db, identity, cleanup
}

// TestIdentityService_SubForUUID_DeletedUser mimics forcefully removing a
// user: mint a real UUID for "sub-a", store it as an artifact's created_by,
// delete the user_idp_references row, and confirm resolution falls back to
// constants.DeletedUser rather than erroring or returning the raw UUID.
func TestIdentityService_SubForUUID_DeletedUser(t *testing.T) {
	db, identity, cleanup := setupIdentityTestDB(t)
	t.Cleanup(cleanup)

	userUUID, err := identity.ToInternalUUID("sub-a")
	if err != nil {
		t.Fatalf("ToInternalUUID failed: %v", err)
	}

	resolved, err := identity.SubForUUID(userUUID)
	if err != nil {
		t.Fatalf("SubForUUID (before delete) failed: %v", err)
	}
	if resolved != "sub-a" {
		t.Fatalf("expected resolution to sub-a before delete, got %q", resolved)
	}

	if _, err := db.Exec(`DELETE FROM user_idp_references WHERE idp_id = 'sub-a'`); err != nil {
		t.Fatalf("failed to delete user_idp_references row: %v", err)
	}

	resolved, err = identity.SubForUUID(userUUID)
	if err != nil {
		t.Fatalf("SubForUUID (after delete) failed: %v", err)
	}
	if resolved != constants.DeletedUser {
		t.Fatalf("expected resolution to fall back to %q after the mapping row is deleted, got %q", constants.DeletedUser, resolved)
	}
}

// TestIdentityService_SubsForUUIDs_DeletedUser exercises the batch resolver
// used by list responses: a mix of a still-mapped UUID, a UUID whose mapping
// was deleted, and an unmapped/anonymous UUID that never had a row.
func TestIdentityService_SubsForUUIDs_DeletedUser(t *testing.T) {
	db, identity, cleanup := setupIdentityTestDB(t)
	t.Cleanup(cleanup)

	aliveUUID, err := identity.ToInternalUUID("sub-alive")
	if err != nil {
		t.Fatalf("ToInternalUUID(sub-alive) failed: %v", err)
	}
	removedUUID, err := identity.ToInternalUUID("sub-removed")
	if err != nil {
		t.Fatalf("ToInternalUUID(sub-removed) failed: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM user_idp_references WHERE idp_id = 'sub-removed'`); err != nil {
		t.Fatalf("failed to delete user_idp_references row: %v", err)
	}
	neverMappedUUID := "00000000-0000-0000-0000-000000000000"

	resolved, err := identity.SubsForUUIDs([]string{aliveUUID, removedUUID, neverMappedUUID})
	if err != nil {
		t.Fatalf("SubsForUUIDs failed: %v", err)
	}
	if resolved[aliveUUID] != "sub-alive" {
		t.Fatalf("expected %q to resolve to sub-alive, got %q", aliveUUID, resolved[aliveUUID])
	}
	if resolved[removedUUID] != constants.DeletedUser {
		t.Fatalf("expected removed mapping to resolve to %q, got %q", constants.DeletedUser, resolved[removedUUID])
	}
	if resolved[neverMappedUUID] != constants.DeletedUser {
		t.Fatalf("expected never-mapped UUID to resolve to %q, got %q", constants.DeletedUser, resolved[neverMappedUUID])
	}
}

// TestAPIService_ModelToRESTAPI_DeletedUser exercises the actual response
// path a REST API's detail/list response takes: create an API whose
// created_by/updated_by are a real internal UUID, delete the backing
// user_idp_references row (the "forcefully remove an existing user" flow),
// and confirm the API's response fields resolve to constants.DeletedUser
// instead of leaking the raw UUID.
func TestAPIService_ModelToRESTAPI_DeletedUser(t *testing.T) {
	db, identity, cleanup := setupIdentityTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-deleted-user"
	projectUUID := "project-deleted-user"
	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, 'deleted-user-org', 'Deleted User Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`, orgUUID); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO projects (uuid, handle, display_name, organization_uuid, description, created_at, updated_at)
		VALUES (?, 'deleted-user-project', 'Deleted User Project', ?, '', datetime('now'), datetime('now'))`, projectUUID, orgUUID); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	actorUUID, err := identity.ToInternalUUID("sub-to-delete")
	if err != nil {
		t.Fatalf("ToInternalUUID failed: %v", err)
	}

	apiRepo := repository.NewAPIRepo(db)
	apiModel := &model.API{
		Handle:          "deleted-user-api",
		Name:            "Deleted User API",
		Version:         "1.0.0",
		CreatedBy:       actorUUID,
		UpdatedBy:       actorUUID,
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "CREATED",
		Configuration: model.RestAPIConfig{
			Name:      "Deleted User API",
			Version:   "1.0.0",
			Transport: []string{"https"},
		},
	}
	if err := apiRepo.CreateAPI(apiModel); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}

	apiSvc := &APIService{
		apiRepo:     apiRepo,
		projectRepo: repository.NewProjectRepo(db),
		apiUtil:     &utils.APIUtil{},
		identity:    identity,
	}

	before, err := apiSvc.modelToRESTAPI(apiModel)
	if err != nil {
		t.Fatalf("modelToRESTAPI (before delete) failed: %v", err)
	}
	if before.CreatedBy == nil || *before.CreatedBy != "sub-to-delete" {
		t.Fatalf("expected createdBy to resolve to sub-to-delete before delete, got %v", before.CreatedBy)
	}
	if before.UpdatedBy == nil || *before.UpdatedBy != "sub-to-delete" {
		t.Fatalf("expected updatedBy to resolve to sub-to-delete before delete, got %v", before.UpdatedBy)
	}

	// Mimic forcefully removing the user: delete their user_idp_references row.
	if _, err := db.Exec(`DELETE FROM user_idp_references WHERE idp_id = 'sub-to-delete'`); err != nil {
		t.Fatalf("failed to delete user_idp_references row: %v", err)
	}

	// Re-fetch from the DB (fresh model instance) rather than reusing apiModel,
	// so this exercises the same read path a real GET request would take.
	reread, err := apiRepo.GetAPIByUUID(apiModel.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetAPIByUUID failed: %v", err)
	}
	if reread == nil {
		t.Fatal("GetAPIByUUID returned nil")
	}

	after, err := apiSvc.modelToRESTAPI(reread)
	if err != nil {
		t.Fatalf("modelToRESTAPI (after delete) failed: %v", err)
	}
	if after.CreatedBy == nil || *after.CreatedBy != constants.DeletedUser {
		t.Fatalf("expected createdBy to resolve to %q after the user is removed, got %v", constants.DeletedUser, after.CreatedBy)
	}
	if after.UpdatedBy == nil || *after.UpdatedBy != constants.DeletedUser {
		t.Fatalf("expected updatedBy to resolve to %q after the user is removed, got %v", constants.DeletedUser, after.UpdatedBy)
	}
}

// TestAPIService_GetAPIsByOrganization_BatchResolvesDeletedUser exercises the
// list-endpoint identity resolution path (batched via
// IdentityService.ResolveIdentityFields, not the per-item ResolveIdentityField
// used by detail responses) with a mix of an alive user and a removed one,
// confirming each list item resolves independently and correctly.
func TestAPIService_GetAPIsByOrganization_BatchResolvesDeletedUser(t *testing.T) {
	db, identity, cleanup := setupIdentityTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-list-deleted-user"
	projectUUID := "project-list-deleted-user"
	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, 'list-deleted-user-org', 'List Deleted User Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`, orgUUID); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO projects (uuid, handle, display_name, organization_uuid, description, created_at, updated_at)
		VALUES (?, 'list-deleted-user-project', 'List Deleted User Project', ?, '', datetime('now'), datetime('now'))`, projectUUID, orgUUID); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	aliveUUID, err := identity.ToInternalUUID("sub-list-alive")
	if err != nil {
		t.Fatalf("ToInternalUUID(sub-list-alive) failed: %v", err)
	}
	removedUUID, err := identity.ToInternalUUID("sub-list-removed")
	if err != nil {
		t.Fatalf("ToInternalUUID(sub-list-removed) failed: %v", err)
	}

	apiRepo := repository.NewAPIRepo(db)
	for _, tc := range []struct {
		handle    string
		createdBy string
	}{
		{"list-deleted-user-api-alive", aliveUUID},
		{"list-deleted-user-api-removed", removedUUID},
	} {
		m := &model.API{
			Handle:          tc.handle,
			Name:            tc.handle,
			Version:         "1.0.0",
			CreatedBy:       tc.createdBy,
			UpdatedBy:       tc.createdBy,
			ProjectID:       projectUUID,
			OrganizationID:  orgUUID,
			LifeCycleStatus: "CREATED",
			Configuration: model.RestAPIConfig{
				Name:      tc.handle,
				Version:   "1.0.0",
				Transport: []string{"https"},
			},
		}
		if err := apiRepo.CreateAPI(m); err != nil {
			t.Fatalf("CreateAPI(%s) failed: %v", tc.handle, err)
		}
	}

	if _, err := db.Exec(`DELETE FROM user_idp_references WHERE idp_id = 'sub-list-removed'`); err != nil {
		t.Fatalf("failed to delete user_idp_references row: %v", err)
	}

	apiSvc := &APIService{
		apiRepo:     apiRepo,
		projectRepo: repository.NewProjectRepo(db),
		apiUtil:     &utils.APIUtil{},
		identity:    identity,
	}

	list, _, err := apiSvc.GetAPIsByOrganization(orgUUID, "", repository.ListOptions{Limit: 100})
	if err != nil {
		t.Fatalf("GetAPIsByOrganization failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 APIs, got %d", len(list))
	}

	byHandle := make(map[string]*string, len(list))
	for i := range list {
		byHandle[*list[i].Id] = list[i].CreatedBy
	}
	if createdBy := byHandle["list-deleted-user-api-alive"]; createdBy == nil || *createdBy != "sub-list-alive" {
		t.Fatalf("expected the alive user's API to resolve to sub-list-alive, got %v", createdBy)
	}
	if createdBy := byHandle["list-deleted-user-api-removed"]; createdBy == nil || *createdBy != constants.DeletedUser {
		t.Fatalf("expected the removed user's API to resolve to %q, got %v", constants.DeletedUser, createdBy)
	}
}

// TestAPIService_UpdateAPI_PreservesCreatedByAcrossDifferentActor guards a
// regression in UpdateAPI: its response resolved createdBy to
// constants.DeletedUser whenever the update was performed by an actor
// different from the creator, even though the persisted created_by column
// was never touched and a plain GET after the same update resolved
// correctly. Root cause: applyAPIUpdates builds its return DTO from
// modelToRESTAPI(existingAPIModel), which already resolves CreatedBy from a
// raw UUID to a raw username (e.g. "sub-creator"); RESTAPIToModel then fed
// that already-resolved username back in as if it were the internal UUID,
// and the response's second identity-resolution pass found no
// user_idp_references row named "sub-creator", falling back to
// constants.DeletedUser. Fixed by re-deriving CreatedBy from
// existingAPIModel (the raw UUID) immediately before the final response
// conversion.
func TestAPIService_UpdateAPI_PreservesCreatedByAcrossDifferentActor(t *testing.T) {
	db, identity, cleanup := setupIdentityTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-update-actor"
	projectUUID := "project-update-actor"
	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, 'update-actor-org', 'Update Actor Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`, orgUUID); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO projects (uuid, handle, display_name, organization_uuid, description, created_at, updated_at)
		VALUES (?, 'update-actor-project', 'Update Actor Project', ?, '', datetime('now'), datetime('now'))`, projectUUID, orgUUID); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	creatorUUID, err := identity.ToInternalUUID("sub-creator")
	if err != nil {
		t.Fatalf("ToInternalUUID(creator) failed: %v", err)
	}
	updaterUUID, err := identity.ToInternalUUID("sub-updater")
	if err != nil {
		t.Fatalf("ToInternalUUID(updater) failed: %v", err)
	}

	apiRepo := repository.NewAPIRepo(db)
	apiModel := &model.API{
		Handle:          "update-actor-api",
		Name:            "Update Actor API",
		Version:         "1.0.0",
		CreatedBy:       creatorUUID,
		UpdatedBy:       creatorUUID,
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "CREATED",
		Configuration: model.RestAPIConfig{
			Name:      "Update Actor API",
			Version:   "1.0.0",
			Transport: []string{"https"},
		},
	}
	if err := apiRepo.CreateAPI(apiModel); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}

	apiSvc := &APIService{
		apiRepo:     apiRepo,
		projectRepo: repository.NewProjectRepo(db),
		apiUtil:     &utils.APIUtil{},
		identity:    identity,
		auditRepo:   &noopAuditRepo{},
	}

	updateReq := &api.RESTAPI{
		DisplayName: "Update Actor API (renamed)",
		Version:     "1.0.0",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{Url: ptr("https://example.com")},
		},
	}
	updated, err := apiSvc.UpdateAPI(apiModel.ID, updateReq, orgUUID, updaterUUID)
	if err != nil {
		t.Fatalf("UpdateAPI failed: %v", err)
	}
	if updated.CreatedBy == nil || *updated.CreatedBy != "sub-creator" {
		t.Fatalf("expected createdBy to remain resolved to the original creator (sub-creator), got %v", updated.CreatedBy)
	}
	if updated.UpdatedBy == nil || *updated.UpdatedBy != "sub-updater" {
		t.Fatalf("expected updatedBy to resolve to the new actor (sub-updater), got %v", updated.UpdatedBy)
	}

	// A subsequent GET must resolve identically — confirms the fix isn't
	// merely masking the bug in one response while leaving stored data (or
	// other read paths) inconsistent.
	reread, err := apiRepo.GetAPIByUUID(apiModel.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetAPIByUUID failed: %v", err)
	}
	reGet, err := apiSvc.modelToRESTAPI(reread)
	if err != nil {
		t.Fatalf("modelToRESTAPI failed: %v", err)
	}
	if reGet.CreatedBy == nil || *reGet.CreatedBy != "sub-creator" {
		t.Fatalf("expected a subsequent GET to also resolve createdBy to sub-creator, got %v", reGet.CreatedBy)
	}
}

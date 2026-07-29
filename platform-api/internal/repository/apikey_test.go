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

// createTestArtifactAPI creates a minimal REST API and returns its artifact UUID,
// for tests that only need a valid artifacts row to attach an API key to.
func createTestArtifactAPI(t *testing.T, db *database.DB, orgUUID, projectUUID string) string {
	t.Helper()
	apiRepo := NewAPIRepo(db)
	api := &model.API{
		Handle:          "artifact-fixture",
		Name:            "Artifact Fixture",
		Version:         "1.0.0",
		CreatedBy:       "fixture-user",
		UpdatedBy:       "fixture-user",
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "CREATED",
		Configuration: model.RestAPIConfig{
			Name:      "Artifact Fixture",
			Version:   "1.0.0",
			Transport: []string{"https"},
		},
	}
	if err := apiRepo.CreateAPI(api); err != nil {
		t.Fatalf("failed to create artifact fixture API: %v", err)
	}
	return api.ID
}

func TestAPIKeyRepo_CreateAndRead_SetsUpdatedBy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-apikey-updatedby"
	projectUUID := "project-apikey-updatedby"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)
	artifactUUID := createTestArtifactAPI(t, db, orgUUID, projectUUID)

	repo := NewAPIKeyRepo(db, nil)
	key := &model.APIKey{
		UUID:           "apikey-uuid-1",
		ArtifactUUID:   artifactUUID,
		Name:           "key-1",
		DisplayName:    "Key 1",
		MaskedAPIKey:   "ab12",
		APIKeyHashes:   `{"sha256":"hash1"}`,
		Status:         "active",
		CreatedBy:      "test-user",
		UpdatedBy:      "test-user",
		AllowedTargets: "ALL",
	}
	if err := repo.Create(key); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.GetByArtifactAndName(artifactUUID, "key-1")
	if err != nil {
		t.Fatalf("GetByArtifactAndName failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetByArtifactAndName returned nil")
	}
	if got.UpdatedBy == "" {
		t.Fatal("expected updated_by to be set on creation, got empty string")
	}
	if got.UpdatedBy != got.CreatedBy {
		t.Fatalf("expected updated_by == created_by on creation, got created_by=%q updated_by=%q", got.CreatedBy, got.UpdatedBy)
	}
}

func TestAPIKeyRepo_UpdateAndRevoke_SetUpdatedByWithoutTouchingCreatedBy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-apikey-update-revoke"
	projectUUID := "project-apikey-update-revoke"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)
	artifactUUID := createTestArtifactAPI(t, db, orgUUID, projectUUID)

	repo := NewAPIKeyRepo(db, nil)
	key := &model.APIKey{
		UUID:           "apikey-uuid-2",
		ArtifactUUID:   artifactUUID,
		Name:           "key-2",
		DisplayName:    "Key 2",
		MaskedAPIKey:   "cd34",
		APIKeyHashes:   `{"sha256":"hash2"}`,
		Status:         "active",
		CreatedBy:      "creator-user",
		UpdatedBy:      "creator-user",
		AllowedTargets: "ALL",
	}
	if err := repo.Create(key); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update by a different actor: updated_by must change, created_by must not.
	key.MaskedAPIKey = "ef56"
	key.UpdatedBy = "updater-user"
	if err := repo.Update(key); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	afterUpdate, err := repo.GetByArtifactAndName(artifactUUID, "key-2")
	if err != nil {
		t.Fatalf("GetByArtifactAndName after update failed: %v", err)
	}
	if afterUpdate.UpdatedBy != "updater-user" {
		t.Fatalf("expected updated_by to change to updater-user, got %q", afterUpdate.UpdatedBy)
	}
	if afterUpdate.CreatedBy != "creator-user" {
		t.Fatalf("Update must not touch created_by, got %q", afterUpdate.CreatedBy)
	}

	// Revoke by yet another actor.
	if err := repo.Revoke(artifactUUID, "key-2", "revoker-user"); err != nil {
		t.Fatalf("Revoke failed: %v", err)
	}

	afterRevoke, err := repo.GetByArtifactAndName(artifactUUID, "key-2")
	if err != nil {
		t.Fatalf("GetByArtifactAndName after revoke failed: %v", err)
	}
	if afterRevoke.Status != "revoked" {
		t.Fatalf("expected status revoked, got %q", afterRevoke.Status)
	}
	if afterRevoke.UpdatedBy != "revoker-user" {
		t.Fatalf("expected updated_by to change to revoker-user, got %q", afterRevoke.UpdatedBy)
	}
	if afterRevoke.CreatedBy != "creator-user" {
		t.Fatalf("Revoke must not touch created_by, got %q", afterRevoke.CreatedBy)
	}
}

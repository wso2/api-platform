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
	"fmt"
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

// TestLifecycle_APIKeyCreateListRevoke drives the real APIKeyRepo through key
// creation, lookup, listing, update and revocation. The key hangs off a REST
// API artifact created via APIRepo, so this exercises the api_keys ->
// artifacts foreign key alongside the API key generation flow.
func TestLifecycle_APIKeyCreateListRevoke(t *testing.T) {
	tdb := openSchemaDB(t)
	orgID, projID := seedOrgProject(t, tdb, "key")

	// An API key references an artifact; create a REST API to back it.
	apiRepo := NewAPIRepo(tdb.db)
	api := &model.API{
		Handle:          "key-api-" + id()[:8],
		Name:            "Key API " + id()[:6],
		Version:         "v1.0",
		ProjectID:       projID,
		OrganizationID:  orgID,
		LifeCycleStatus: "CREATED",
		Configuration:   model.RestAPIConfig{Name: "Key API", Version: "v1.0"},
	}
	if err := apiRepo.CreateAPI(api); err != nil {
		t.Fatalf("CreateAPI (for key) failed: %v", err)
	}
	artifactUUID := api.ID

	keyRepo := NewAPIKeyRepo(tdb.db, NewArtifactTableRegistry())

	// --- Create two API keys on the same artifact ---
	const n = 2
	for i := range n {
		key := &model.APIKey{
			UUID:           id(),
			ArtifactUUID:   artifactUUID,
			Name:           fmt.Sprintf("key-%d", i),
			DisplayName:    fmt.Sprintf("Key %d", i),
			MaskedAPIKey:   "ab12",
			APIKeyHashes:   `{"sha256":"` + id() + `"}`,
			Status:         "active",
			CreatedBy:      "it-user",
			UpdatedBy:      "it-user",
			AllowedTargets: "ALL",
		}
		if err := keyRepo.Create(key); err != nil {
			t.Fatalf("APIKey Create %d failed: %v", i, err)
		}
	}

	// --- Read back a single key and confirm the round-trip ---
	got, err := keyRepo.GetByArtifactAndName(artifactUUID, "key-0")
	if err != nil {
		t.Fatalf("GetByArtifactAndName failed: %v", err)
	}
	if got == nil || got.ArtifactUUID != artifactUUID || got.Status != "active" {
		t.Fatalf("GetByArtifactAndName round-trip mismatch: %+v", got)
	}
	if got.AllowedTargets != "ALL" {
		t.Fatalf("APIKey allowed_targets did not round-trip: %q", got.AllowedTargets)
	}

	// --- List by artifact ---
	keys, err := keyRepo.ListByArtifact(artifactUUID)
	if err != nil {
		t.Fatalf("ListByArtifact failed: %v", err)
	}
	if len(keys) != n {
		t.Fatalf("ListByArtifact: want %d, got %d", n, len(keys))
	}

	// --- Update the key material ---
	got.MaskedAPIKey = "cd34"
	got.APIKeyHashes = `{"sha256":"` + id() + `"}`
	if err := keyRepo.Update(got); err != nil {
		t.Fatalf("APIKey Update failed: %v", err)
	}
	afterUpdate, err := keyRepo.GetByArtifactAndName(artifactUUID, "key-0")
	if err != nil {
		t.Fatalf("GetByArtifactAndName after update failed: %v", err)
	}
	if afterUpdate.MaskedAPIKey != "cd34" {
		t.Fatalf("APIKey Update did not persist masked key: %q", afterUpdate.MaskedAPIKey)
	}

	// --- Revoke the key ---
	if err := keyRepo.Revoke(artifactUUID, "key-0", "it-revoker"); err != nil {
		t.Fatalf("APIKey Revoke failed: %v", err)
	}
	revoked, err := keyRepo.GetByArtifactAndName(artifactUUID, "key-0")
	if err != nil {
		t.Fatalf("GetByArtifactAndName after revoke failed: %v", err)
	}
	if revoked.Status != "revoked" {
		t.Fatalf("APIKey Revoke did not persist status: %q", revoked.Status)
	}
	if revoked.UpdatedBy != "it-revoker" {
		t.Fatalf("APIKey Revoke did not persist updated_by actor: %q", revoked.UpdatedBy)
	}
	if revoked.CreatedBy != "it-user" {
		t.Fatalf("APIKey Revoke must not touch created_by: %q", revoked.CreatedBy)
	}
}

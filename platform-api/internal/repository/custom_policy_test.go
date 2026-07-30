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
	"encoding/json"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

// TestDeleteCustomPolicyIfUnusedPurgesOrphanUsages covers installs that
// already have a stale usage row from before the ArtifactRepo.Delete fix: a
// usage row pointing at an artifact that no longer exists must not keep
// blocking the policy's deletion, and the orphan row itself is cleaned up.
func TestDeleteCustomPolicyIfUnusedPurgesOrphanUsages(t *testing.T) {
	db, cleanup := setupTestDBWithoutForeignKeys(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-orphan-purge"
	createTestOrganizationAndProject(t, db, orgUUID, "project-orphan-purge")

	customPolicyRepo := NewCustomPolicyRepo(db)
	policy := &model.CustomPolicy{
		UUID:             "policy-orphan-purge",
		OrganizationUUID: orgUUID,
		Name:             "custom-policy-orphan-purge",
		Version:          "v1.0.0",
	}
	if err := customPolicyRepo.InsertCustomPolicy(policy); err != nil {
		t.Fatalf("create custom policy: %v", err)
	}

	const missingArtifactUUID = "artifact-does-not-exist"
	if err := customPolicyRepo.InsertCustomPolicyUsage(policy.UUID, missingArtifactUUID); err != nil {
		t.Fatalf("insert orphan usage: %v", err)
	}

	purged, err := customPolicyRepo.DeleteCustomPolicyIfUnused(orgUUID, policy.UUID)
	if err != nil {
		t.Fatalf("DeleteCustomPolicyIfUnused() error = %v, want nil", err)
	}
	if purged != 1 {
		t.Fatalf("DeleteCustomPolicyIfUnused() purged = %d, want 1", purged)
	}

	stored, err := customPolicyRepo.GetCustomPolicyByUUID(orgUUID, policy.UUID)
	if err != nil {
		t.Fatalf("GetCustomPolicyByUUID() error = %v", err)
	}
	if stored != nil {
		t.Fatalf("GetCustomPolicyByUUID() = %+v, want policy deleted", stored)
	}

	usages, err := customPolicyRepo.GetCustomPolicyUsagesByAPIUUID(missingArtifactUUID)
	if err != nil {
		t.Fatalf("GetCustomPolicyUsagesByAPIUUID() error = %v", err)
	}
	if len(usages) != 0 {
		t.Fatalf("orphan usage row still present: %v", usages)
	}
}

// TestDeleteCustomPolicyIfUnusedBlocksWhenArtifactExists confirms the orphan
// purge cannot loosen the guard: a usage row backed by a live artifact still
// blocks deletion with PolicyInUse.
func TestDeleteCustomPolicyIfUnusedBlocksWhenArtifactExists(t *testing.T) {
	db, cleanup := setupTestDBWithoutForeignKeys(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-orphan-block"
	createTestOrganizationAndProject(t, db, orgUUID, "project-orphan-block")

	customPolicyRepo := NewCustomPolicyRepo(db)
	policy := &model.CustomPolicy{
		UUID:             "policy-orphan-block",
		OrganizationUUID: orgUUID,
		Name:             "custom-policy-orphan-block",
		Version:          "v1.0.0",
		PolicyDefinition: json.RawMessage("{}"),
	}
	if err := customPolicyRepo.InsertCustomPolicy(policy); err != nil {
		t.Fatalf("create custom policy: %v", err)
	}

	const liveArtifactUUID = "artifact-still-live"
	if _, err := db.Exec(db.Rebind(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`),
		liveArtifactUUID, "LlmProvider", orgUUID); err != nil {
		t.Fatalf("insert live artifact: %v", err)
	}
	if err := customPolicyRepo.InsertCustomPolicyUsage(policy.UUID, liveArtifactUUID); err != nil {
		t.Fatalf("insert usage: %v", err)
	}

	purged, err := customPolicyRepo.DeleteCustomPolicyIfUnused(orgUUID, policy.UUID)
	if !apperror.PolicyInUse.Is(err) {
		t.Fatalf("DeleteCustomPolicyIfUnused() error = %v, want PolicyInUse", err)
	}
	if purged != 0 {
		t.Fatalf("DeleteCustomPolicyIfUnused() purged = %d, want 0 (no orphans present)", purged)
	}

	stored, err := customPolicyRepo.GetCustomPolicyByUUID(orgUUID, policy.UUID)
	if err != nil {
		t.Fatalf("GetCustomPolicyByUUID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("GetCustomPolicyByUUID() = nil, want policy still present")
	}
}

// TestCountCustomPolicyUsagesIgnoresOrphans confirms the count consumed by any
// caller of the join table also treats an artifact-less usage row as absent.
func TestCountCustomPolicyUsagesIgnoresOrphans(t *testing.T) {
	db, cleanup := setupTestDBWithoutForeignKeys(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-count-orphan"
	createTestOrganizationAndProject(t, db, orgUUID, "project-count-orphan")

	customPolicyRepo := NewCustomPolicyRepo(db)
	policy := &model.CustomPolicy{
		UUID:             "policy-count-orphan",
		OrganizationUUID: orgUUID,
		Name:             "custom-policy-count-orphan",
		Version:          "v1.0.0",
	}
	if err := customPolicyRepo.InsertCustomPolicy(policy); err != nil {
		t.Fatalf("create custom policy: %v", err)
	}
	if err := customPolicyRepo.InsertCustomPolicyUsage(policy.UUID, "artifact-does-not-exist"); err != nil {
		t.Fatalf("insert orphan usage: %v", err)
	}

	count, err := customPolicyRepo.CountCustomPolicyUsages(policy.UUID)
	if err != nil {
		t.Fatalf("CountCustomPolicyUsages() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("CountCustomPolicyUsages() = %d, want 0", count)
	}
}

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
)

// createTestUser inserts a user_idp_references row and returns its UUID.
func createTestUser(t *testing.T, identityRepo UserIdentityMappingRepository, idpID string) string {
	t.Helper()
	uuid, err := identityRepo.GetOrCreateUUID(idpID)
	if err != nil {
		t.Fatalf("Failed to create test user %q: %v", idpID, err)
	}
	return uuid
}

func TestUserOrganizationMappingRepo_AddMembership(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	identityRepo := NewUserIdentityMappingRepo(db)
	mappingRepo := NewUserOrganizationMappingRepo(db)

	userUUID := createTestUser(t, identityRepo, "user-add-membership")
	orgUUID := "org-add-membership"
	createTestOrganizationAndProject(t, db, orgUUID, "project-add-membership")

	if err := mappingRepo.AddMembership(userUUID, orgUUID); err != nil {
		t.Fatalf("AddMembership failed: %v", err)
	}

	// Idempotent: a duplicate pair is a no-op, not an error.
	if err := mappingRepo.AddMembership(userUUID, orgUUID); err != nil {
		t.Fatalf("AddMembership (duplicate) should be a no-op, got error: %v", err)
	}

	orgRepo := NewOrganizationRepo(db)
	orgs, err := orgRepo.ListOrganizationsForUser(userUUID, 20, 0)
	if err != nil {
		t.Fatalf("ListOrganizationsForUser failed: %v", err)
	}
	if len(orgs) != 1 {
		t.Fatalf("expected exactly 1 membership row despite duplicate AddMembership calls, got %d", len(orgs))
	}
}

func TestUserOrganizationMappingRepo_AddMembership_UnknownUserOrOrgFails(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	identityRepo := NewUserIdentityMappingRepo(db)
	mappingRepo := NewUserOrganizationMappingRepo(db)

	userUUID := createTestUser(t, identityRepo, "user-fk-check")
	orgUUID := "org-fk-check"
	createTestOrganizationAndProject(t, db, orgUUID, "project-fk-check")

	if err := mappingRepo.AddMembership("does-not-exist", orgUUID); err == nil {
		t.Fatal("expected AddMembership to fail for an unknown user UUID (FK violation)")
	}
	if err := mappingRepo.AddMembership(userUUID, "does-not-exist"); err == nil {
		t.Fatal("expected AddMembership to fail for an unknown org UUID (FK violation)")
	}
}

func TestOrganizationRepo_ListAndCountOrganizationsForUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	identityRepo := NewUserIdentityMappingRepo(db)
	mappingRepo := NewUserOrganizationMappingRepo(db)
	orgRepo := NewOrganizationRepo(db)

	userUUID := createTestUser(t, identityRepo, "user-list-orgs")

	// Three orgs; the user is a member of exactly two.
	createTestOrganizationAndProject(t, db, "org-list-1", "project-list-1")
	createTestOrganizationAndProject(t, db, "org-list-2", "project-list-2")
	createTestOrganizationAndProject(t, db, "org-list-3", "project-list-3")

	if err := mappingRepo.AddMembership(userUUID, "org-list-1"); err != nil {
		t.Fatalf("AddMembership failed: %v", err)
	}
	if err := mappingRepo.AddMembership(userUUID, "org-list-2"); err != nil {
		t.Fatalf("AddMembership failed: %v", err)
	}

	total, err := orgRepo.CountOrganizationsForUser(userUUID)
	if err != nil {
		t.Fatalf("CountOrganizationsForUser failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected count 2, got %d", total)
	}

	orgs, err := orgRepo.ListOrganizationsForUser(userUUID, 20, 0)
	if err != nil {
		t.Fatalf("ListOrganizationsForUser failed: %v", err)
	}
	if len(orgs) != 2 {
		t.Fatalf("expected 2 orgs, got %d", len(orgs))
	}
	seen := map[string]bool{}
	for _, org := range orgs {
		seen[org.ID] = true
		if org.ID == "org-list-3" {
			t.Fatalf("org-list-3 should not appear for a user with no membership row for it")
		}
	}
	if !seen["org-list-1"] || !seen["org-list-2"] {
		t.Fatalf("expected both org-list-1 and org-list-2 in result, got %+v", orgs)
	}

	// Pagination: limit=1 offset=1 should return exactly one of the two, newest first.
	page, err := orgRepo.ListOrganizationsForUser(userUUID, 1, 1)
	if err != nil {
		t.Fatalf("ListOrganizationsForUser (paginated) failed: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 org on the second page, got %d", len(page))
	}

	// A user with no memberships sees nothing.
	otherUser := createTestUser(t, identityRepo, "user-no-memberships")
	none, err := orgRepo.ListOrganizationsForUser(otherUser, 20, 0)
	if err != nil {
		t.Fatalf("ListOrganizationsForUser (no memberships) failed: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected 0 orgs for a user with no memberships, got %d", len(none))
	}
	noneCount, err := orgRepo.CountOrganizationsForUser(otherUser)
	if err != nil {
		t.Fatalf("CountOrganizationsForUser (no memberships) failed: %v", err)
	}
	if noneCount != 0 {
		t.Fatalf("expected count 0 for a user with no memberships, got %d", noneCount)
	}
}

func TestOrganizationRepo_DeleteOrganization_RemovesMembership(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	identityRepo := NewUserIdentityMappingRepo(db)
	mappingRepo := NewUserOrganizationMappingRepo(db)
	orgRepo := NewOrganizationRepo(db)

	userUUID := createTestUser(t, identityRepo, "user-delete-org")
	orgUUID := "org-delete-me"
	createTestOrganizationAndProject(t, db, orgUUID, "project-delete-me")

	if err := mappingRepo.AddMembership(userUUID, orgUUID); err != nil {
		t.Fatalf("AddMembership failed: %v", err)
	}

	if err := orgRepo.DeleteOrganization(orgUUID); err != nil {
		t.Fatalf("DeleteOrganization failed: %v", err)
	}

	orgs, err := orgRepo.ListOrganizationsForUser(userUUID, 20, 0)
	if err != nil {
		t.Fatalf("ListOrganizationsForUser after delete failed: %v", err)
	}
	if len(orgs) != 0 {
		t.Fatalf("expected membership row to be removed after org deletion, got %d orgs", len(orgs))
	}
}

func TestUserOrganizationMappingRepo_DeleteByUserAndByOrg(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	identityRepo := NewUserIdentityMappingRepo(db)
	mappingRepo := NewUserOrganizationMappingRepo(db)
	orgRepo := NewOrganizationRepo(db)

	userA := createTestUser(t, identityRepo, "user-delete-a")
	userB := createTestUser(t, identityRepo, "user-delete-b")
	createTestOrganizationAndProject(t, db, "org-delete-by-x", "project-delete-by-x")

	if err := mappingRepo.AddMembership(userA, "org-delete-by-x"); err != nil {
		t.Fatalf("AddMembership failed: %v", err)
	}
	if err := mappingRepo.AddMembership(userB, "org-delete-by-x"); err != nil {
		t.Fatalf("AddMembership failed: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}
	if err := mappingRepo.DeleteByUser(tx, userA); err != nil {
		t.Fatalf("DeleteByUser failed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit tx: %v", err)
	}

	remaining, err := orgRepo.CountOrganizationsForUser(userA)
	if err != nil {
		t.Fatalf("CountOrganizationsForUser failed: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected userA to have 0 memberships after DeleteByUser, got %d", remaining)
	}
	stillThere, err := orgRepo.CountOrganizationsForUser(userB)
	if err != nil {
		t.Fatalf("CountOrganizationsForUser failed: %v", err)
	}
	if stillThere != 1 {
		t.Fatalf("expected userB's membership to be untouched, got %d", stillThere)
	}
}

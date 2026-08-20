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
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// createTestAPIPortalOrg inserts the organization row api_portals references via its FK.
// The organizations table has no other prerequisite so this is a single INSERT.
func createTestAPIPortalOrg(t *testing.T, db *database.DB, orgUUID string) {
	t.Helper()
	q := `
		INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, ?, ?, 'default', 'idp-ref', datetime('now'), datetime('now'))
	`
	if _, err := db.Exec(q, orgUUID, "test-org-"+orgUUID, "Test Org"); err != nil {
		t.Fatalf("failed to insert test organization: %v", err)
	}
}

// newTestAPIPortal returns a valid *model.APIPortal with sensible defaults.
// Individual tests override the fields they care about.
func newTestAPIPortal(uuid, orgUUID, handle string) *model.APIPortal {
	return &model.APIPortal{
		ID:             uuid,
		OrganizationID: orgUUID,
		Handle:         handle,
		Name:           "Portal " + handle,
		Description:    "test portal",
		URL:            "https://" + handle + ".example.com",
		WorkflowStatus: constants.APIPortalWorkflowStatusPending,
		AuthType:       constants.APIPortalAuthTypeLocal,
		AuthConfig:     map[string]interface{}{"foo": "bar"},
		CreatedBy:      "tester",
		UpdatedBy:      "tester",
	}
}

func TestAPIPortalRepo_CreateAndGet(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-crud"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	portal := newTestAPIPortal("portal-001", orgUUID, "acme")
	if err := repo.Create(portal); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Get by UUID.
	got, err := repo.GetByUUID(portal.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByUUID: expected row, got nil")
	}
	if got.Handle != portal.Handle || got.Name != portal.Name || got.URL != portal.URL {
		t.Errorf("GetByUUID: field mismatch; got %+v", got)
	}
	if got.AuthConfig["foo"] != "bar" {
		t.Errorf("configuration not round-tripped; got %v", got.AuthConfig)
	}

	// Get by handle.
	got2, err := repo.GetByHandleAndOrgID(portal.Handle, orgUUID)
	if err != nil {
		t.Fatalf("GetByHandleAndOrgID: %v", err)
	}
	if got2 == nil || got2.ID != portal.ID {
		t.Errorf("GetByHandleAndOrgID mismatch; got %+v", got2)
	}
}

func TestAPIPortalRepo_Create_SetsDefaults(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-defaults"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	portal := newTestAPIPortal("portal-defaults", orgUUID, "defaults")
	// Explicitly leave timestamps zero; expect Create to populate them.
	portal.CreatedAt = time.Time{}
	portal.UpdatedAt = time.Time{}

	before := time.Now().UTC().Add(-time.Second)
	if err := repo.Create(portal); err != nil {
		t.Fatalf("Create: %v", err)
	}
	after := time.Now().UTC().Add(time.Second)

	if portal.CreatedAt.Before(before) || portal.CreatedAt.After(after) {
		t.Errorf("CreatedAt not set to ~now: got %v", portal.CreatedAt)
	}
	if portal.UpdatedAt.Before(before) || portal.UpdatedAt.After(after) {
		t.Errorf("UpdatedAt not set to ~now: got %v", portal.UpdatedAt)
	}
}

func TestAPIPortalRepo_Create_AuthConfigRoundTrip_Nil(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-cfg-nil"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	portal := newTestAPIPortal("portal-cfg-nil", orgUUID, "cfg-nil")
	portal.AuthConfig = nil // will be stored as {} and read back as empty map

	if err := repo.Create(portal); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.GetByUUID(portal.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if got.AuthConfig == nil {
		t.Fatal("AuthConfig is nil after round-trip; expected non-nil empty map")
	}
	if len(got.AuthConfig) != 0 {
		t.Errorf("AuthConfig expected empty; got %v", got.AuthConfig)
	}
}

func TestAPIPortalRepo_Create_AuthConfigRoundTrip_Populated(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-cfg-full"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	portal := newTestAPIPortal("portal-cfg-full", orgUUID, "cfg-full")
	portal.AuthConfig = map[string]interface{}{
		"stsTokenUrl": "https://sts.example.com/token",
		"clientId":    "abc",
		"audience":    []interface{}{"aud-1", "aud-2"},
	}

	if err := repo.Create(portal); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.GetByUUID(portal.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if got.AuthConfig["stsTokenUrl"] != "https://sts.example.com/token" {
		t.Errorf("stsTokenUrl round-trip failed; got %v", got.AuthConfig["stsTokenUrl"])
	}
	if got.AuthConfig["clientId"] != "abc" {
		t.Errorf("clientId round-trip failed; got %v", got.AuthConfig["clientId"])
	}
	aud, ok := got.AuthConfig["audience"].([]interface{})
	if !ok || len(aud) != 2 || aud[0] != "aud-1" || aud[1] != "aud-2" {
		t.Errorf("audience round-trip failed; got %v", got.AuthConfig["audience"])
	}
}

func TestAPIPortalRepo_Create_DuplicateHandle(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-dup"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	if err := repo.Create(newTestAPIPortal("portal-dup-1", orgUUID, "dup")); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	err := repo.Create(newTestAPIPortal("portal-dup-2", orgUUID, "dup"))
	if err == nil {
		t.Fatal("expected duplicate handle to fail, got nil")
	}
	if !IsUniqueViolation(err) {
		t.Errorf("expected unique-constraint violation, got %v", err)
	}
}

func TestAPIPortalRepo_Get_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-nf"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	got, err := repo.GetByUUID("does-not-exist", orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID: unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("GetByUUID: expected nil for missing row, got %+v", got)
	}
	got2, err := repo.GetByHandleAndOrgID("no-such-handle", orgUUID)
	if err != nil {
		t.Fatalf("GetByHandleAndOrgID: unexpected error: %v", err)
	}
	if got2 != nil {
		t.Errorf("GetByHandleAndOrgID: expected nil for missing row, got %+v", got2)
	}
}

func TestAPIPortalRepo_Get_CrossOrgIsolation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgA = "org-portal-a"
	const orgB = "org-portal-b"
	createTestAPIPortalOrg(t, db, orgA)
	createTestAPIPortalOrg(t, db, orgB)

	repo := NewAPIPortalRepo(db)
	if err := repo.Create(newTestAPIPortal("portal-a", orgA, "shared-handle")); err != nil {
		t.Fatalf("Create A: %v", err)
	}
	if err := repo.Create(newTestAPIPortal("portal-b", orgB, "shared-handle")); err != nil {
		t.Fatalf("Create B (different org, same handle allowed): %v", err)
	}
	// A's portal-a must not be visible when querying org B.
	got, err := repo.GetByUUID("portal-a", orgB)
	if err != nil {
		t.Fatalf("GetByUUID cross-org: %v", err)
	}
	if got != nil {
		t.Errorf("cross-org leak: got %+v", got)
	}
}

func TestAPIPortalRepo_ListPaginated(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-list"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	// Insert 5 portals with staggered createdAt to make ordering deterministic.
	for i, handle := range []string{"aa", "bb", "cc", "dd", "ee"} {
		p := newTestAPIPortal("portal-"+handle, orgUUID, handle)
		if err := repo.Create(p); err != nil {
			t.Fatalf("Create %s: %v", handle, err)
		}
		// Nudge each row's created_at forward so DESC ordering is stable.
		p.CreatedAt = time.Now().UTC().Add(time.Duration(i) * time.Millisecond)
		if _, err := db.Exec(`UPDATE api_portals SET created_at = ? WHERE uuid = ?`, p.CreatedAt, p.ID); err != nil {
			t.Fatalf("nudge created_at: %v", err)
		}
	}

	// Page 1: limit 2 → newest first ("ee", "dd").
	page1, err := repo.ListPaginated(orgUUID, nil, ListOptions{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("ListPaginated page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page 1 size: want 2, got %d", len(page1))
	}
	if page1[0].Handle != "ee" || page1[1].Handle != "dd" {
		t.Errorf("page 1 order: got %s, %s", page1[0].Handle, page1[1].Handle)
	}

	// Page 2: offset 2, limit 2 → "cc", "bb".
	page2, err := repo.ListPaginated(orgUUID, nil, ListOptions{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("ListPaginated page 2: %v", err)
	}
	if len(page2) != 2 || page2[0].Handle != "cc" || page2[1].Handle != "bb" {
		t.Errorf("page 2: %+v", page2)
	}

	// Count without filter.
	total, err := repo.Count(orgUUID, nil, "")
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if total != 5 {
		t.Errorf("Count: want 5, got %d", total)
	}
}

func TestAPIPortalRepo_ListPaginated_WorkflowStatusFilter(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-status"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	// 2 pending, 1 active.
	p1 := newTestAPIPortal("p1", orgUUID, "p1")
	p1.WorkflowStatus = constants.APIPortalWorkflowStatusPending
	if err := repo.Create(p1); err != nil {
		t.Fatalf("Create p1: %v", err)
	}
	p2 := newTestAPIPortal("p2", orgUUID, "p2")
	p2.WorkflowStatus = constants.APIPortalWorkflowStatusPending
	if err := repo.Create(p2); err != nil {
		t.Fatalf("Create p2: %v", err)
	}
	p3 := newTestAPIPortal("p3", orgUUID, "p3")
	p3.WorkflowStatus = constants.APIPortalWorkflowStatusActive
	if err := repo.Create(p3); err != nil {
		t.Fatalf("Create p3: %v", err)
	}

	active := constants.APIPortalWorkflowStatusActive
	got, err := repo.ListPaginated(orgUUID, &active, ListOptions{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("ListPaginated: %v", err)
	}
	if len(got) != 1 || got[0].Handle != "p3" {
		t.Errorf("want 1 active portal (p3); got %+v", got)
	}

	// Count with same filter must also reflect it (pagination-total consistency).
	total, err := repo.Count(orgUUID, &active, "")
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if total != 1 {
		t.Errorf("filtered count: want 1, got %d", total)
	}
}

func TestAPIPortalRepo_ListPaginated_Search(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-search"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	for _, h := range []string{"acme-dev", "acme-prod", "other-portal"} {
		if err := repo.Create(newTestAPIPortal("portal-"+h, orgUUID, h)); err != nil {
			t.Fatalf("Create %s: %v", h, err)
		}
	}
	got, err := repo.ListPaginated(orgUUID, nil, ListOptions{Limit: 10, Offset: 0, Search: "acme"})
	if err != nil {
		t.Fatalf("ListPaginated: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 acme results, got %d: %+v", len(got), got)
	}
}

func TestAPIPortalRepo_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-upd"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	portal := newTestAPIPortal("portal-upd", orgUUID, "upd")
	if err := repo.Create(portal); err != nil {
		t.Fatalf("Create: %v", err)
	}
	origCreatedAt := portal.CreatedAt

	// Mutate every whitelisted field + attempt to mutate an immutable one (handle).
	// OrganizationID is left untouched because the UPDATE uses it in the WHERE
	// clause for org isolation; cross-org attempts are covered by
	// TestAPIPortalRepo_Update_CrossOrgIsolation.
	portal.Name = "Renamed"
	portal.Description = "new description"
	portal.URL = "https://renamed.example.com"
	portal.WorkflowStatus = constants.APIPortalWorkflowStatusActive
	portal.AuthType = constants.APIPortalAuthTypeOAuth2
	portal.AuthConfig = map[string]interface{}{"stsTokenUrl": "https://sts/x"}
	portal.UpdatedBy = "editor"
	portal.Handle = "attempted-rename" // immutable — must NOT stick

	if err := repo.Update(portal); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByUUID("portal-upd", orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if got == nil {
		t.Fatal("row disappeared after Update")
	}
	if got.Name != "Renamed" || got.Description != "new description" ||
		got.URL != "https://renamed.example.com" ||
		got.WorkflowStatus != constants.APIPortalWorkflowStatusActive ||
		got.AuthType != constants.APIPortalAuthTypeOAuth2 ||
		got.UpdatedBy != "editor" {
		t.Errorf("mutable fields not persisted; got %+v", got)
	}
	if got.AuthConfig["stsTokenUrl"] != "https://sts/x" {
		t.Errorf("configuration not persisted; got %v", got.AuthConfig)
	}
	if got.Handle != "upd" {
		t.Errorf("handle was mutated despite being immutable; want %q, got %q", "upd", got.Handle)
	}
	if !got.CreatedAt.Equal(origCreatedAt) {
		t.Errorf("created_at was touched; before %v, after %v", origCreatedAt, got.CreatedAt)
	}
}

func TestAPIPortalRepo_Update_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-upd-nf"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	err := repo.Update(newTestAPIPortal("ghost", orgUUID, "ghost"))
	if err == nil {
		t.Fatal("expected Update on missing row to error")
	}
	if !strings.Contains(err.Error(), "api portal not found") {
		t.Errorf("want error containing %q, got %q", "api portal not found", err.Error())
	}
}

func TestAPIPortalRepo_Update_CrossOrgIsolation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgA = "org-portal-upd-a"
	const orgB = "org-portal-upd-b"
	createTestAPIPortalOrg(t, db, orgA)
	createTestAPIPortalOrg(t, db, orgB)

	repo := NewAPIPortalRepo(db)
	if err := repo.Create(newTestAPIPortal("portal-a", orgA, "iso")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Attempt to update A's portal claiming to be in org B — must be rejected as not-found.
	portal := newTestAPIPortal("portal-a", orgB, "iso")
	portal.Name = "hijack"
	err := repo.Update(portal)
	if err == nil {
		t.Fatal("expected Update with wrong org to error as not-found")
	}
	if !strings.Contains(err.Error(), "api portal not found") {
		t.Errorf("want error containing %q, got %q", "api portal not found", err.Error())
	}
}

func TestAPIPortalRepo_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-del"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	portal := newTestAPIPortal("portal-del", orgUUID, "del")
	if err := repo.Create(portal); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Delete(portal.ID, orgUUID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err := repo.GetByUUID(portal.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID after Delete: %v", err)
	}
	if got != nil {
		t.Errorf("row still present after Delete: %+v", got)
	}
}

func TestAPIPortalRepo_Delete_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-del-nf"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	err := repo.Delete("ghost", orgUUID)
	if err == nil {
		t.Fatal("expected Delete on missing row to error")
	}
	if !strings.Contains(err.Error(), "api portal not found") {
		t.Errorf("want error containing %q, got %q", "api portal not found", err.Error())
	}
}

func TestAPIPortalRepo_Exists(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-exists"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	ok, err := repo.Exists("nope", orgUUID)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if ok {
		t.Error("Exists: expected false for missing row")
	}
	if err := repo.Create(newTestAPIPortal("portal-e", orgUUID, "here")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	ok, err = repo.Exists("here", orgUUID)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !ok {
		t.Error("Exists: expected true for existing row")
	}
}

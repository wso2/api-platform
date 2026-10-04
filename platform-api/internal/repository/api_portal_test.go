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
	"bytes"
	"errors"
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
// Individual tests override the fields they care about. InternalAuthKey is
// populated with a non-empty byte slice because the column is NOT NULL; test
// bytes stand in for what would be AES-GCM ciphertext produced by
// service.validateAndEncryptSharedKey in the live code.
func newTestAPIPortal(uuid, orgUUID, handle string) *model.APIPortal {
	return &model.APIPortal{
		ID:              uuid,
		OrganizationID:  orgUUID,
		Handle:          handle,
		Name:            "Portal " + handle,
		Description:     "test portal",
		URL:             "https://" + handle + ".example.com",
		Status:          constants.APIPortalStatusPending,
		InternalAuthKey: []byte("test-ciphertext-" + handle),
		CreatedBy:       "tester",
		UpdatedBy:       "tester",
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
	if !bytes.Equal(got.InternalAuthKey, portal.InternalAuthKey) {
		t.Errorf("InternalAuthKey not round-tripped; want %q got %q",
			portal.InternalAuthKey, got.InternalAuthKey)
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

func TestAPIPortalRepo_Create_MetadataRoundTrip_Nil(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-meta-nil"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	portal := newTestAPIPortal("portal-meta-nil", orgUUID, "meta-nil")
	portal.Metadata = nil // stored as SQL NULL, read back as nil map

	if err := repo.Create(portal); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.GetByUUID(portal.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if got.Metadata != nil {
		t.Errorf("Metadata: want nil after round-trip (column is nullable and marshalAPIPortalBlob returns nil for empty maps); got %v", got.Metadata)
	}
}

func TestAPIPortalRepo_Create_MetadataRoundTrip_Populated(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-meta-full"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	portal := newTestAPIPortal("portal-meta-full", orgUUID, "meta-full")
	portal.Metadata = map[string]interface{}{
		"loginEnvironment": "development",
		"tags":             []interface{}{"beta", "internal"},
	}

	if err := repo.Create(portal); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.GetByUUID(portal.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if got.Metadata["loginEnvironment"] != "development" {
		t.Errorf("loginEnvironment round-trip failed; got %v", got.Metadata["loginEnvironment"])
	}
	tags, ok := got.Metadata["tags"].([]interface{})
	if !ok || len(tags) != 2 || tags[0] != "beta" || tags[1] != "internal" {
		t.Errorf("tags round-trip failed; got %v", got.Metadata["tags"])
	}
}

func TestAPIPortalRepo_Create_InternalAuthKeyRoundTrip(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-key-rt"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	// Simulate what service.validateAndEncryptSharedKey produces: opaque bytes
	// that are neither valid UTF-8 nor a stable text encoding. The column is
	// BYTEA / BLOB / VARBINARY and must survive verbatim.
	binaryCiphertext := []byte{0x00, 0xff, 0x10, 0x7f, 0x80, 0xaa, 0x55, 0xde, 0xad, 0xbe, 0xef}

	portal := newTestAPIPortal("portal-key-rt", orgUUID, "key-rt")
	portal.InternalAuthKey = binaryCiphertext

	if err := repo.Create(portal); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.GetByUUID(portal.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if !bytes.Equal(got.InternalAuthKey, binaryCiphertext) {
		t.Errorf("InternalAuthKey bytes corrupted through round-trip;\n want % x\n got  % x", binaryCiphertext, got.InternalAuthKey)
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
	page1, err := repo.ListPaginated(orgUUID, ListOptions{Limit: 2, Offset: 0})
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
	page2, err := repo.ListPaginated(orgUUID, ListOptions{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("ListPaginated page 2: %v", err)
	}
	if len(page2) != 2 || page2[0].Handle != "cc" || page2[1].Handle != "bb" {
		t.Errorf("page 2: %+v", page2)
	}

	// Count without filter.
	total, err := repo.Count(orgUUID, "")
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if total != 5 {
		t.Errorf("Count: want 5, got %d", total)
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
	got, err := repo.ListPaginated(orgUUID, ListOptions{Limit: 10, Offset: 0, Search: "acme"})
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
	portal.Status = constants.APIPortalStatusActive
	portal.InternalAuthKey = []byte("rotated-ciphertext")
	portal.Metadata = map[string]interface{}{"loginEnvironment": "production"}
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
		got.Status != constants.APIPortalStatusActive ||
		got.UpdatedBy != "editor" {
		t.Errorf("mutable fields not persisted; got %+v", got)
	}
	if !bytes.Equal(got.InternalAuthKey, []byte("rotated-ciphertext")) {
		t.Errorf("InternalAuthKey not persisted; want %q got %q",
			"rotated-ciphertext", got.InternalAuthKey)
	}
	if got.Metadata["loginEnvironment"] != "production" {
		t.Errorf("metadata not persisted; got %v", got.Metadata)
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

func TestAPIPortalRepo_UpdateStatus_HappyPath(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-status-update"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	portal := newTestAPIPortal("portal-us", orgUUID, "us-target")
	// Start in pending, matching how the cloud plugin's Create writes.
	portal.Status = constants.APIPortalStatusPending
	if err := repo.Create(portal); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdateStatus(portal.ID, orgUUID, "poller", constants.APIPortalStatusActive); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, err := repo.GetByUUID(portal.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if got.Status != constants.APIPortalStatusActive {
		t.Errorf("status not flipped; want %q got %q", constants.APIPortalStatusActive, got.Status)
	}
	if got.UpdatedBy != "poller" {
		t.Errorf("UpdatedBy not stamped; got %q", got.UpdatedBy)
	}
}

func TestAPIPortalRepo_UpdateStatus_MissingRow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAPIPortalRepo(db)
	// UpdateStatus must return an error when no row matches; a silent zero-rows
	// update would let the poller mark a deleted portal as active/failed against
	// a row that no longer exists.
	err := repo.UpdateStatus("nonexistent-uuid", "nonexistent-org", "poller", constants.APIPortalStatusActive)
	if err == nil {
		t.Fatal("UpdateStatus on missing row must return an error")
	}
	if !strings.Contains(err.Error(), "api portal not found") {
		t.Errorf("error should name the missing row; got %q", err.Error())
	}
}

// UpdateStatus enforces the pending-only source guard so a poller that races
// a terminal state cannot overwrite it. The write must return
// ErrAPIPortalNotPending and leave the stored status unchanged.
func TestAPIPortalRepo_UpdateStatus_RejectsNonPendingSource(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-status-guard"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	portal := newTestAPIPortal("portal-gd", orgUUID, "guard-target")
	portal.Status = constants.APIPortalStatusPending
	if err := repo.Create(portal); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.UpdateStatus(portal.ID, orgUUID, "poller", constants.APIPortalStatusActive); err != nil {
		t.Fatalf("first UpdateStatus (pending -> active) must succeed: %v", err)
	}
	// Second UpdateStatus on the now-active row is the race we're protecting
	// against: a late poller tick from a lagging replica trying to write
	// "failed" over a portal that has already reached "active".
	err := repo.UpdateStatus(portal.ID, orgUUID, "poller", constants.APIPortalStatusFailed)
	if !errors.Is(err, ErrAPIPortalNotPending) {
		t.Fatalf("update on non-pending row must return ErrAPIPortalNotPending; got %v", err)
	}
	got, err := repo.GetByUUID(portal.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if got.Status != constants.APIPortalStatusActive {
		t.Errorf("stored status must remain %q after rejected update; got %q", constants.APIPortalStatusActive, got.Status)
	}
}

func TestAPIPortalRepo_GetStatusByHandle(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-status-get"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	portal := newTestAPIPortal("portal-gs", orgUUID, "gs-target")
	portal.Status = constants.APIPortalStatusFailed
	if err := repo.Create(portal); err != nil {
		t.Fatalf("Create: %v", err)
	}

	status, err := repo.GetStatusByHandle("gs-target", orgUUID)
	if err != nil {
		t.Fatalf("GetStatusByHandle: %v", err)
	}
	if status != constants.APIPortalStatusFailed {
		t.Errorf("want %q got %q", constants.APIPortalStatusFailed, status)
	}
}

func TestAPIPortalRepo_ListStatusesByOrg(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-status-list"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	for handle, status := range map[string]string{
		"alpha": constants.APIPortalStatusActive,
		"beta":  constants.APIPortalStatusPending,
		"gamma": constants.APIPortalStatusFailed,
	} {
		p := newTestAPIPortal("portal-"+handle, orgUUID, handle)
		p.Status = status
		if err := repo.Create(p); err != nil {
			t.Fatalf("Create %q: %v", handle, err)
		}
	}

	got, err := repo.ListStatusesByOrg(orgUUID)
	if err != nil {
		t.Fatalf("ListStatusesByOrg: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("want 3 entries, got %d: %+v", len(got), got)
	}
	if got["alpha"] != constants.APIPortalStatusActive ||
		got["beta"] != constants.APIPortalStatusPending ||
		got["gamma"] != constants.APIPortalStatusFailed {
		t.Errorf("map contents wrong: %+v", got)
	}
}

// ListLoginEnvironmentsByOrg is the plugin-only companion accessor that lets
// callers hydrate list-view rows with loginEnvironment without exposing the
// field on ApiPortalListItem. Rows whose metadata omits the key must be
// dropped from the result, empty metadata is safe, and unrelated orgs must
// not leak in.
func TestAPIPortalRepo_ListLoginEnvironmentsByOrg(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-loginenvs"
	const otherOrgUUID = "org-portal-loginenvs-other"
	createTestAPIPortalOrg(t, db, orgUUID)
	createTestAPIPortalOrg(t, db, otherOrgUUID)

	repo := NewAPIPortalRepo(db)

	// Three portals in the target org: one with loginEnvironment set, one with
	// a metadata blob that doesn't carry the key, one with no metadata at all.
	withEnv := newTestAPIPortal("portal-with-env", orgUUID, "with-env")
	withEnv.Metadata = map[string]interface{}{"loginEnvironment": "production", "unrelated": "value"}
	if err := repo.Create(withEnv); err != nil {
		t.Fatalf("Create with-env: %v", err)
	}
	otherKey := newTestAPIPortal("portal-other-key", orgUUID, "other-key")
	otherKey.Metadata = map[string]interface{}{"unrelated": "value"}
	if err := repo.Create(otherKey); err != nil {
		t.Fatalf("Create other-key: %v", err)
	}
	noMeta := newTestAPIPortal("portal-no-meta", orgUUID, "no-meta")
	if err := repo.Create(noMeta); err != nil {
		t.Fatalf("Create no-meta: %v", err)
	}

	// Portal in a different org with loginEnvironment set — must not leak into the target org's result.
	other := newTestAPIPortal("portal-other-org", otherOrgUUID, "other-org")
	other.Metadata = map[string]interface{}{"loginEnvironment": "staging"}
	if err := repo.Create(other); err != nil {
		t.Fatalf("Create other-org: %v", err)
	}

	got, err := repo.ListLoginEnvironmentsByOrg(orgUUID)
	if err != nil {
		t.Fatalf("ListLoginEnvironmentsByOrg: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 entry (only with-env qualifies), got %d: %+v", len(got), got)
	}
	if got["with-env"] != "production" {
		t.Errorf("with-env should map to \"production\", got %q", got["with-env"])
	}
	if _, exists := got["other-key"]; exists {
		t.Error("portal without loginEnvironment key must be omitted from the map")
	}
	if _, exists := got["no-meta"]; exists {
		t.Error("portal with no metadata must be omitted from the map")
	}
	if _, exists := got["other-org"]; exists {
		t.Error("portal from a different org must not appear in the target org's result")
	}
}

func TestAPIPortalRepo_ListLoginEnvironmentsByOrg_EmptyOrg(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-loginenvs-empty"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	got, err := repo.ListLoginEnvironmentsByOrg(orgUUID)
	if err != nil {
		t.Fatalf("ListLoginEnvironmentsByOrg on empty org must not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty org should return empty map, got %+v", got)
	}
}

func TestAPIPortalRepo_ListStatusesByOrg_EmptyOrg(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-portal-status-empty"
	createTestAPIPortalOrg(t, db, orgUUID)

	repo := NewAPIPortalRepo(db)
	got, err := repo.ListStatusesByOrg(orgUUID)
	if err != nil {
		t.Fatalf("ListStatusesByOrg on empty org must not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty org should return empty map, got %+v", got)
	}
}

func TestAPIPortalRepo_ListByStatus_CrossOrg(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Two separate orgs, each with portals in different states. The plugin
	// poller's ResumePending scans across every org for the pending set, so
	// this must not be filtered to a single org.
	const orgA = "org-portal-lbs-a"
	const orgB = "org-portal-lbs-b"
	createTestAPIPortalOrg(t, db, orgA)
	createTestAPIPortalOrg(t, db, orgB)

	repo := NewAPIPortalRepo(db)
	fixtures := []struct {
		org, handle, status string
	}{
		{orgA, "a-pending-1", constants.APIPortalStatusPending},
		{orgA, "a-active-1", constants.APIPortalStatusActive},
		{orgB, "b-pending-1", constants.APIPortalStatusPending},
		{orgB, "b-failed-1", constants.APIPortalStatusFailed},
	}
	for _, f := range fixtures {
		p := newTestAPIPortal("portal-"+f.handle, f.org, f.handle)
		p.Status = f.status
		if err := repo.Create(p); err != nil {
			t.Fatalf("Create %q: %v", f.handle, err)
		}
	}

	pending, err := repo.ListByStatus(constants.APIPortalStatusPending)
	if err != nil {
		t.Fatalf("ListByStatus(pending): %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("want 2 pending across orgs, got %d", len(pending))
	}
	// Cross-org selection must include rows from both orgs.
	seen := map[string]bool{}
	for _, p := range pending {
		seen[p.Handle] = true
		if p.Status != constants.APIPortalStatusPending {
			t.Errorf("row %q returned with wrong status %q", p.Handle, p.Status)
		}
	}
	if !seen["a-pending-1"] || !seen["b-pending-1"] {
		t.Errorf("cross-org selection missed a row; got %+v", seen)
	}
}

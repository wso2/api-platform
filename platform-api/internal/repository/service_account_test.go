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
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func newTestServiceAccount(orgID, uuid, handle string) *model.ServiceAccount {
	return &model.ServiceAccount{
		UUID: uuid, OrganizationID: orgID, Handle: handle, DisplayName: handle, Owner: "team", Description: "test",
		ClientID: "sa_org_" + handle, ClientSecretHash: "hash-" + handle, MaskedSecret: "***abcde",
		Roles: "ap_sa_reader",
	}
}

func TestServiceAccountRepo_CRUDAndDeleteKeepsLedger(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	createTestOrganizationAndProject(t, db, "org-sa", "proj-sa")
	repo := NewServiceAccountRepo(db)

	sa := newTestServiceAccount("org-sa", "11111111-0000-0000-0000-000000000001", "ci-bot")
	if err := repo.Create(sa, sa.Subject("org")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(newTestServiceAccount("org-sa", "11111111-0000-0000-0000-000000000002", "ci-bot"), "sa:org:ci-bot:x"); !apperror.ServiceAccountExists.Is(err) {
		t.Fatalf("duplicate handle: want ServiceAccountExists, got %v", err)
	}

	got, err := repo.GetByClientID(sa.ClientID)
	if err != nil || got.UUID != sa.UUID || got.Owner != "team" || got.LastUsedAt != nil {
		t.Fatalf("GetByClientID = %+v, %v", got, err)
	}
	if err := repo.TouchLastUsed(sa.UUID, time.Now(), "203.0.113.7"); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetByHandle("org-sa", "ci-bot")
	if got.LastUsedAt == nil || got.LastUsedIP != "203.0.113.7" {
		t.Fatalf("last used not recorded: %+v", got)
	}

	if got.TokenVersion != 1 {
		t.Fatalf("new account token version = %d, want 1", got.TokenVersion)
	}

	now := time.Now().UTC()
	rev := &model.ServiceAccountRevocation{AccountUUID: sa.UUID, OrganizationID: "org-sa", MinTokenVersion: 2, ExpiresAt: now.Add(time.Hour)}
	if err := repo.Delete("org-sa", sa.UUID, 2, rev); !apperror.Conflict.Is(err) {
		t.Fatalf("Delete with a stale version: want conflict, got %v", err)
	}
	if err := repo.Delete("org-sa", sa.UUID, 1, rev); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByHandle("org-sa", "ci-bot"); !apperror.ServiceAccountNotFound.Is(err) {
		t.Fatalf("want not found after delete, got %v", err)
	}
	active, err := repo.ListActive(now)
	if err != nil || len(active) != 1 || active[0].AccountUUID != sa.UUID {
		t.Fatalf("ledger row must outlive the account: %+v, %v", active, err)
	}
}

func TestServiceAccountRepo_RevokeIsMonotonic(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	repo := NewServiceAccountRepo(db)

	t0 := time.Now().UTC().Truncate(time.Millisecond)
	rev := func(minVersion int64) *model.ServiceAccountRevocation {
		return &model.ServiceAccountRevocation{AccountUUID: "acc", OrganizationID: "org", MinTokenVersion: minVersion, ExpiresAt: t0.Add(time.Hour)}
	}
	for _, v := range []int64{2, 4, 3} {
		if err := repo.Revoke(rev(v)); err != nil {
			t.Fatalf("Revoke(%d): %v", v, err)
		}
	}
	active, err := repo.ListActive(t0)
	if err != nil || len(active) != 1 {
		t.Fatalf("ListActive = %v, %v", active, err)
	}
	if active[0].MinTokenVersion != 4 {
		t.Fatalf("watermark moved backwards: got %d, want 4", active[0].MinTokenVersion)
	}

	if n, err := repo.PruneExpired(t0.Add(2 * time.Hour)); err != nil || n != 1 {
		t.Fatalf("PruneExpired = %d, %v", n, err)
	}
}

// A write based on a stale read (the account was disabled or revoked since)
// must conflict rather than overwrite the newer status.
func TestServiceAccountRepo_UpdateRejectsStaleRead(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	createTestOrganizationAndProject(t, db, "org-sa", "proj-sa")
	repo := NewServiceAccountRepo(db)

	sa := newTestServiceAccount("org-sa", "11111111-0000-0000-0000-000000000004", "ci-bot")
	if err := repo.Create(sa, sa.Subject("org")); err != nil {
		t.Fatal(err)
	}
	stale, _ := repo.GetByHandle("org-sa", "ci-bot")

	disabled, _ := repo.GetByHandle("org-sa", "ci-bot")
	disabled.Status, disabled.TokenVersion = model.ServiceAccountStatusDisabled, 2
	if err := repo.Update(disabled, 1, model.ServiceAccountStatusActive, nil); err != nil {
		t.Fatalf("disable: %v", err)
	}

	stale.Description = "edited from a stale read"
	if err := repo.Update(stale, stale.TokenVersion, stale.Status, nil); !apperror.Conflict.Is(err) {
		t.Fatalf("stale update: want conflict, got %v", err)
	}
	if err := repo.UpdateSecret(stale, stale.TokenVersion, nil); !apperror.Conflict.Is(err) {
		t.Fatalf("stale secret update: want conflict, got %v", err)
	}
	if got, _ := repo.GetByHandle("org-sa", "ci-bot"); got.Status != model.ServiceAccountStatusDisabled || got.TokenVersion != 2 {
		t.Fatalf("stale write took effect: %+v", got)
	}

	missing := newTestServiceAccount("org-sa", "11111111-0000-0000-0000-00000000dead", "gone")
	if err := repo.Update(missing, 1, model.ServiceAccountStatusActive, nil); !apperror.ServiceAccountNotFound.Is(err) {
		t.Fatalf("missing account: want not found, got %v", err)
	}
}

func TestServiceAccountRepo_ForeignReservedIdentities(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	createTestOrganizationAndProject(t, db, "org-sa", "proj-sa")
	repo := NewServiceAccountRepo(db)

	sa := newTestServiceAccount("org-sa", "11111111-0000-0000-0000-000000000003", "ci-bot")
	if err := repo.Create(sa, sa.Subject("org")); err != nil {
		t.Fatal(err)
	}
	// A minted identity stays legitimate after its account is deleted.
	if err := repo.Delete("org-sa", sa.UUID, 1, nil); err != nil {
		t.Fatal(err)
	}
	if foreign, err := repo.ForeignReservedIdentities(); err != nil || len(foreign) != 0 {
		t.Fatalf("minted identity reported as foreign: %v, %v", foreign, err)
	}

	if _, err := db.Exec(`INSERT INTO user_idp_references (uuid, idp_id) VALUES ('u-1', 'sa:org:ci-bot:11111111-0000-0000-0000-000000000009')`); err != nil {
		t.Fatal(err)
	}
	if foreign, err := repo.ForeignReservedIdentities(); err != nil || len(foreign) != 1 {
		t.Fatalf("want one foreign identity, got %v, %v", foreign, err)
	}
}

func TestServiceAccountRepo_ListAndCount(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	createTestOrganizationAndProject(t, db, "org-sa", "proj-sa")
	createTestOrganizationAndProject(t, db, "org-other", "proj-other")
	repo := NewServiceAccountRepo(db)

	for uuid, h := range map[string]string{
		"11111111-0000-0000-0000-000000000100": "a-bot",
		"11111111-0000-0000-0000-000000000101": "b-bot",
		"11111111-0000-0000-0000-000000000102": "c-bot",
	} {
		sa := newTestServiceAccount("org-sa", uuid, h)
		if err := repo.Create(sa, sa.Subject("org")); err != nil {
			t.Fatal(err)
		}
	}
	other := newTestServiceAccount("org-other", "11111111-0000-0000-0000-000000000200", "z-bot")
	if err := repo.Create(other, other.Subject("other")); err != nil {
		t.Fatal(err)
	}

	if n, err := repo.Count("org-sa", ""); err != nil || n != 3 {
		t.Fatalf("Count = %d, %v", n, err)
	}
	page, err := repo.List("org-sa", "", 2, 0)
	if err != nil || len(page) != 2 {
		t.Fatalf("first page = %d, %v", len(page), err)
	}
	rest, err := repo.List("org-sa", "", 2, 2)
	if err != nil || len(rest) != 1 {
		t.Fatalf("second page = %d, %v", len(rest), err)
	}
	seen := map[string]bool{}
	for _, sa := range append(page, rest...) {
		if sa.OrganizationID != "org-sa" || seen[sa.Handle] {
			t.Fatalf("unexpected row %+v", sa)
		}
		seen[sa.Handle] = true
	}
}

func TestServiceAccountRepo_GetByClientIDUnknown(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	if _, err := NewServiceAccountRepo(db).GetByClientID("sa_org_nobody"); !apperror.ServiceAccountNotFound.Is(err) {
		t.Fatalf("unknown client ID: want not found, got %v", err)
	}
}

func TestServiceAccountRepo_PruneKeepsLiveRows(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	repo := NewServiceAccountRepo(db)
	now := time.Now().UTC()
	if err := repo.Revoke(&model.ServiceAccountRevocation{AccountUUID: "acc", OrganizationID: "org-sa", MinTokenVersion: 2, ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.PruneExpired(now); err != nil || n != 0 {
		t.Fatalf("PruneExpired removed a live row: %d, %v", n, err)
	}
}

func TestServiceAccountRepo_SurfacesDatabaseErrors(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	repo := NewServiceAccountRepo(db)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	sa := newTestServiceAccount("org-sa", "11111111-0000-0000-0000-000000000001", "ci-bot")
	rev := &model.ServiceAccountRevocation{AccountUUID: sa.UUID, OrganizationID: "org-sa", MinTokenVersion: 2, ExpiresAt: time.Now()}
	checks := map[string]func() error{
		"create":        func() error { return repo.Create(sa, sa.Subject("org")) },
		"get by handle": func() error { _, err := repo.GetByHandle("org-sa", "ci-bot"); return err },
		"get by client": func() error { _, err := repo.GetByClientID(sa.ClientID); return err },
		"list":          func() error { _, err := repo.List("org-sa", "", 10, 0); return err },
		"count":         func() error { _, err := repo.Count("org-sa", ""); return err },
		"update":        func() error { return repo.Update(sa, 1, model.ServiceAccountStatusActive, rev) },
		"update secret": func() error { return repo.UpdateSecret(sa, 1, rev) },
		"delete":        func() error { return repo.Delete("org-sa", sa.UUID, 1, rev) },
		"touch":         func() error { return repo.TouchLastUsed(sa.UUID, time.Now(), "203.0.113.7") },
		"foreign":       func() error { _, err := repo.ForeignReservedIdentities(); return err },
		"revoke":        func() error { return repo.Revoke(rev) },
		"list active":   func() error { _, err := repo.ListActive(time.Now()); return err },
		"prune":         func() error { _, err := repo.PruneExpired(time.Now()); return err },
	}
	for name, fn := range checks {
		t.Run(name, func(t *testing.T) {
			err := fn()
			if err == nil {
				t.Fatal("call on a closed database succeeded")
			}
			if apperror.ServiceAccountNotFound.Is(err) {
				t.Fatalf("closed database reported as not-found: %v", err)
			}
		})
	}
}

// Update and UpdateSecret write rev in the same transaction; other replicas
// see a revoke only through this row.
func TestServiceAccountRepo_WritesWatermarkWithChange(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	createTestOrganizationAndProject(t, db, "org-sa", "proj-sa")
	repo := NewServiceAccountRepo(db)
	sa := newTestServiceAccount("org-sa", "11111111-0000-0000-0000-000000000004", "ci-bot")
	if err := repo.Create(sa, sa.Subject("org")); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	watermark := func() int64 {
		t.Helper()
		active, err := repo.ListActive(now)
		if err != nil || len(active) != 1 {
			t.Fatalf("ListActive = %v, %v", active, err)
		}
		return active[0].MinTokenVersion
	}
	rev := func(v int64) *model.ServiceAccountRevocation {
		return &model.ServiceAccountRevocation{AccountUUID: sa.UUID, OrganizationID: "org-sa", MinTokenVersion: v, ExpiresAt: now.Add(time.Hour)}
	}

	sa.Status, sa.TokenVersion = model.ServiceAccountStatusDisabled, 2
	if err := repo.Update(sa, 1, model.ServiceAccountStatusActive, rev(2)); err != nil {
		t.Fatal(err)
	}
	if got := watermark(); got != 2 {
		t.Fatalf("after Update: watermark %d, want 2", got)
	}
	sa.ClientSecretHash, sa.TokenVersion = "hash-new", 3
	if err := repo.UpdateSecret(sa, 2, rev(3)); err != nil {
		t.Fatal(err)
	}
	if got := watermark(); got != 3 {
		t.Fatalf("after UpdateSecret: watermark %d, want 3", got)
	}
}

func TestServiceAccountRepo_ListSearch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	createTestOrganizationAndProject(t, db, "org-sa", "proj-sa")
	repo := NewServiceAccountRepo(db)

	for i, handle := range []string{"ci-bot", "nightly-report", "data_100"} {
		sa := newTestServiceAccount("org-sa", "22222222-0000-0000-0000-00000000000"+string(rune('1'+i)), handle)
		if handle == "nightly-report" {
			sa.DisplayName, sa.Owner = "Nightly Report", "Data-Team@example.com"
		}
		if err := repo.Create(sa, sa.Subject("org")); err != nil {
			t.Fatalf("Create %s: %v", handle, err)
		}
	}

	for _, tc := range []struct {
		search string
		want   int
	}{
		{"", 3},
		{"NIGHTLY", 1},   // name, case-insensitive
		{"ci-b", 1},      // handle
		{"data-team", 1}, // owner
		{"_", 1},         // a literal underscore, not a single-character wildcard
		{"%", 0},         // a literal percent sign
		{"missing", 0},
	} {
		got, err := repo.List("org-sa", tc.search, 20, 0)
		if err != nil {
			t.Fatalf("List(%q): %v", tc.search, err)
		}
		n, err := repo.Count("org-sa", tc.search)
		if err != nil {
			t.Fatalf("Count(%q): %v", tc.search, err)
		}
		if len(got) != tc.want || n != tc.want {
			t.Errorf("search %q: list %d, count %d, want %d", tc.search, len(got), n, tc.want)
		}
	}
}

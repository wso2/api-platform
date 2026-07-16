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

package service

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"database/sql"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"

	_ "github.com/mattn/go-sqlite3"
)

// setupOrganizationTestEnv creates a real OrganizationService backed by SQLite.
func setupOrganizationTestEnv(t *testing.T) (*OrganizationService, *database.DB, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "org-test.db")
	// Enable foreign-key enforcement for every pooled connection via the DSN,
	// not just the one connection a PRAGMA Exec would happen to run on.
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
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

	orgRepo := repository.NewOrganizationRepo(db)
	svc := &OrganizationService{
		orgRepo:            orgRepo,
		projectRepo:        repository.NewProjectRepo(db),
		auditRepo:          &noopAuditRepo{},
		userOrgMappingRepo: repository.NewUserOrganizationMappingRepo(db),
		identity:           NewIdentityService(repository.NewUserIdentityMappingRepo(db)),
		slogger:            slog.Default(),
	}

	cleanup := func() { sqlDB.Close() }
	return svc, db, cleanup
}

// TestOrganizationService_RegisterOrganization_RecordsMembership verifies that
// registering an organization also records the creator's membership row, so
// GET /organizations (membership-filtered) shows it to them immediately.
func TestOrganizationService_RegisterOrganization_RecordsMembership(t *testing.T) {
	svc, db, cleanup := setupOrganizationTestEnv(t)
	t.Cleanup(cleanup)

	actorUUID, err := svc.identity.ToInternalUUID("sub-org-creator")
	if err != nil {
		t.Fatalf("ToInternalUUID failed: %v", err)
	}

	org, err := svc.RegisterOrganization("org-register-1", "register-org", "Register Org", "us", "", actorUUID)
	if err != nil {
		t.Fatalf("RegisterOrganization failed: %v", err)
	}
	if org == nil {
		t.Fatal("RegisterOrganization returned nil organization")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_organization_mappings WHERE user_uuid = ? AND org_uuid = ?`,
		actorUUID, "org-register-1").Scan(&count); err != nil {
		t.Fatalf("failed to query membership: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected a membership row for the registering user, got count=%d", count)
	}
}

// TestOrganizationService_ListOrganizationsForUser_HealsMembership verifies
// the lazy-heal behavior: an organization predating user_organization_mappings
// (e.g. the file-based seeded org) becomes visible to a caller on their first
// list call, and a membership row is created as a side effect.
func TestOrganizationService_ListOrganizationsForUser_HealsMembership(t *testing.T) {
	svc, db, cleanup := setupOrganizationTestEnv(t)
	t.Cleanup(cleanup)

	// Simulate a pre-existing org with no membership rows (e.g. seeded at startup).
	preexistingOrg := &model.Organization{
		ID:     "org-preexisting",
		Handle: "preexisting-org",
		Name:   "Preexisting Org",
		Region: "us",
	}
	if err := svc.orgRepo.CreateOrganization(preexistingOrg); err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	userUUID, err := svc.identity.ToInternalUUID("sub-heal-user")
	if err != nil {
		t.Fatalf("ToInternalUUID failed: %v", err)
	}

	// Before healing, the user has no visibility into the org.
	orgs, total, err := svc.ListOrganizationsForUser(userUUID, "", 20, 0)
	if err != nil {
		t.Fatalf("ListOrganizationsForUser (pre-heal) failed: %v", err)
	}
	if total != 0 || len(orgs) != 0 {
		t.Fatalf("expected no visible orgs before heal, got total=%d orgs=%+v", total, orgs)
	}

	// Simulate the caller's token resolving to this org (as the resolver
	// middleware would populate via GetOrganizationFromRequest).
	orgs, total, err = svc.ListOrganizationsForUser(userUUID, "org-preexisting", 20, 0)
	if err != nil {
		t.Fatalf("ListOrganizationsForUser (heal) failed: %v", err)
	}
	if total != 1 || len(orgs) != 1 {
		t.Fatalf("expected the healed org to be visible, got total=%d orgs=%+v", total, orgs)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_organization_mappings WHERE user_uuid = ? AND org_uuid = ?`,
		userUUID, "org-preexisting").Scan(&count); err != nil {
		t.Fatalf("failed to query membership: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected the heal to create a membership row, got count=%d", count)
	}
}

// TestOrganizationService_ListOrganizationsForUser_SkipsHealOnEmptyIDs
// verifies that an empty user or org UUID never attempts a heal write (which
// would otherwise fail its foreign keys) and simply lists existing membership.
func TestOrganizationService_ListOrganizationsForUser_SkipsHealOnEmptyIDs(t *testing.T) {
	svc, _, cleanup := setupOrganizationTestEnv(t)
	t.Cleanup(cleanup)

	userUUID, err := svc.identity.ToInternalUUID("sub-no-heal")
	if err != nil {
		t.Fatalf("ToInternalUUID failed: %v", err)
	}

	if _, _, err := svc.ListOrganizationsForUser(userUUID, "", 20, 0); err != nil {
		t.Fatalf("expected no error when resolvedOrgUUID is empty, got: %v", err)
	}
	if _, _, err := svc.ListOrganizationsForUser("", "some-org", 20, 0); err != nil {
		t.Fatalf("expected no error when userUUID is empty, got: %v", err)
	}
}

// mockOrgRepoForErrors embeds the interface so only the methods under test
// need overriding, then forces both ListOrganizationsForUser paths to fail.
type mockOrgRepoForErrors struct {
	repository.OrganizationRepository
}

func (m *mockOrgRepoForErrors) CountOrganizationsForUser(userUUID string) (int, error) {
	return 0, errors.New("count boom")
}

func (m *mockOrgRepoForErrors) ListOrganizationsForUser(userUUID string, limit, offset int) ([]*model.Organization, error) {
	return nil, errors.New("list boom")
}

// TestOrganizationService_ListOrganizationsForUser_PropagatesRepoErrors
// verifies that repository failures surface to the caller instead of being
// swallowed (unlike the best-effort membership heal, which only logs).
func TestOrganizationService_ListOrganizationsForUser_PropagatesRepoErrors(t *testing.T) {
	svc := &OrganizationService{
		orgRepo:            &mockOrgRepoForErrors{},
		userOrgMappingRepo: &noopUserOrgMappingRepo{},
		identity:           newTestIdentityService(),
		slogger:            slog.Default(),
	}

	if _, _, err := svc.ListOrganizationsForUser("user-uuid", "", 20, 0); err == nil {
		t.Fatal("expected CountOrganizationsForUser error to propagate")
	}
}

// noopUserOrgMappingRepo satisfies repository.UserOrganizationMappingRepository
// without a real DB, for tests that never reach AddMembership (userUUID or
// resolvedOrgUUID empty) or don't care about its outcome.
type noopUserOrgMappingRepo struct{}

func (noopUserOrgMappingRepo) AddMembership(userUUID, orgUUID string) error { return nil }
func (noopUserOrgMappingRepo) DeleteByUser(tx *sql.Tx, userUUID string) error { return nil }
func (noopUserOrgMappingRepo) DeleteByOrg(tx *sql.Tx, orgUUID string) error   { return nil }

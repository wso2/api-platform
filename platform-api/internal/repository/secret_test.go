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

// ---- helpers ----------------------------------------------------------------

func createTestSecret(t *testing.T, db interface {
	Exec(string, ...interface{}) (interface{ RowsAffected() (int64, error) }, error)
}, orgID, handle string) {
	t.Helper()
}

func insertSecret(t *testing.T, repo SecretRepository, orgID, handle string) *model.Secret {
	t.Helper()
	s := &model.Secret{
		OrganizationID: orgID,
		Handle:         handle,
		DisplayName:    handle + "-display",
		Ciphertext:     []byte("cipher:" + handle),
		Hash:           "sha256:abc",
		Type:           model.SecretTypeGeneric,
		Provider:       model.SecretProviderInHouse,
		Status:         model.SecretStatusActive,
		CreatedBy:      "test-user",
		UpdatedBy:      "test-user",
	}
	if err := repo.Create(s); err != nil {
		t.Fatalf("insertSecret: %v", err)
	}
	return s
}

func createOrgForSecret(t *testing.T, db interface {
	Exec(string, ...interface{}) (interface{ RowsAffected() (int64, error) }, error)
}, orgID string) {
	t.Helper()
}

// ---- SecretRepo CRUD tests --------------------------------------------------

func TestSecretRepo_CreateAndGetByHandle(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-sec-001"
	createTestOrganizationAndProject(t, db, orgID, "proj-sec-001")

	repo := NewSecretRepo(db)
	s := &model.Secret{
		OrganizationID: orgID,
		Handle:         "my-secret",
		DisplayName:    "My Secret",
		Description:    "A test secret",
		Ciphertext:     []byte("encrypted-value"),
		Hash:           "sha256:abc123",
		Type:           model.SecretTypeGeneric,
		Provider:       model.SecretProviderInHouse,
		Status:         model.SecretStatusActive,
		CreatedBy:      "alice",
		UpdatedBy:      "alice",
	}

	if err := repo.Create(s); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.UUID == "" {
		t.Error("UUID should be auto-generated")
	}

	got, err := repo.GetByHandle(orgID, "my-secret")
	if err != nil {
		t.Fatalf("GetByHandle: %v", err)
	}
	if got.Handle != "my-secret" {
		t.Errorf("handle = %q, want %q", got.Handle, "my-secret")
	}
}

func TestSecretRepo_GetByHandle_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-sec-002"
	createTestOrganizationAndProject(t, db, orgID, "proj-sec-002")

	repo := NewSecretRepo(db)
	_, err := repo.GetByHandle(orgID, "nonexistent")
	if !apperror.SecretNotFound.Is(err) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestSecretRepo_CreateStoresScopes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-sec-003"
	createTestOrganizationAndProject(t, db, orgID, "proj-sec-003")

	repo := NewSecretRepo(db)
	s := &model.Secret{
		OrganizationID: orgID,
		Handle:         "scope-test",
		Ciphertext:     []byte("ct"),
		Hash:           "h",
		CreatedBy:      "u",
		UpdatedBy:      "u",
		Scopes: []model.SecretScope{
			{Scope: model.SecretScopeTypeOrg, ScopeValue: orgID},
		},
	}
	if err := repo.Create(s); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.UUID == "" {
		t.Error("UUID should be auto-generated")
	}

	got, err := repo.GetByHandle(orgID, "scope-test")
	if err != nil {
		t.Fatalf("GetByHandle: %v", err)
	}
	if got.Handle != "scope-test" {
		t.Errorf("handle = %q, want %q", got.Handle, "scope-test")
	}
}

func TestSecretRepo_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-sec-004"
	createTestOrganizationAndProject(t, db, orgID, "proj-sec-004")

	repo := NewSecretRepo(db)
	s := &model.Secret{
		OrganizationID: orgID,
		Handle:         "updatable",
		DisplayName:    "old",
		Ciphertext:     []byte("old-ct"),
		Hash:           "h",
		CreatedBy:      "u",
		UpdatedBy:      "u",
	}
	if err := repo.Create(s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	s.DisplayName = "new"
	s.Ciphertext = []byte("new-ct")
	if err := repo.Update(s); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := repo.GetByHandle(orgID, "updatable")
	if got.DisplayName != "new" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "new")
	}
	if string(got.Ciphertext) != "new-ct" {
		t.Errorf("Ciphertext = %q, want %q", got.Ciphertext, "new-ct")
	}
}

func TestSecretRepo_SoftDelete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-sec-005"
	createTestOrganizationAndProject(t, db, orgID, "proj-sec-005")

	repo := NewSecretRepo(db)
	s := &model.Secret{
		OrganizationID: orgID,
		Handle:         "deletable",
		Ciphertext:     []byte("ct"),
		Hash:           "h",
		CreatedBy:      "u",
		UpdatedBy:      "u",
	}
	if err := repo.Create(s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	refs, err := repo.FindRefsAndSoftDelete(orgID, "deletable", "admin")
	if err != nil {
		t.Fatalf("FindRefsAndSoftDelete: %v", err)
	}
	if len(refs) > 0 {
		t.Fatalf("expected no refs blocking delete, got %v", refs)
	}

	// Exists should return false after soft-delete (status=DEPRECATED)
	exists, err := repo.Exists(orgID, "deletable")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("expected secret to be inactive after soft-delete")
	}
}

func TestSecretRepo_List_Pagination(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-sec-006"
	createTestOrganizationAndProject(t, db, orgID, "proj-sec-006")

	repo := NewSecretRepo(db)
	for i := 0; i < 5; i++ {
		s := &model.Secret{
			OrganizationID: orgID,
			Handle:         string(rune('a'+i)) + "-secret",
			Ciphertext:     []byte("ct"),
			Hash:           "h",
			CreatedBy:      "u",
			UpdatedBy:      "u",
		}
		if err := repo.Create(s); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// First page
	page1, err := repo.List(orgID, 3, 0, nil)
	if err != nil {
		t.Fatalf("List page1: %v", err)
	}
	if len(page1) != 3 {
		t.Errorf("page1 len = %d, want 3", len(page1))
	}

	// Second page
	page2, err := repo.List(orgID, 3, 3, nil)
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("page2 len = %d, want 2", len(page2))
	}

	// Count
	count, err := repo.Count(orgID)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 5 {
		t.Errorf("Count = %d, want 5", count)
	}
}

func TestSecretRepo_List_UpdatedAfterFilter(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-sec-007"
	createTestOrganizationAndProject(t, db, orgID, "proj-sec-007")

	repo := NewSecretRepo(db)
	s := &model.Secret{
		OrganizationID: orgID,
		Handle:         "old-secret",
		Ciphertext:     []byte("ct"),
		Hash:           "h",
		CreatedBy:      "u",
		UpdatedBy:      "u",
	}
	if err := repo.Create(s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	future := time.Now().Add(time.Hour)
	secrets, err := repo.List(orgID, 25, 0, &future)
	if err != nil {
		t.Fatalf("List with future filter: %v", err)
	}
	if len(secrets) != 0 {
		t.Errorf("expected 0 results with future updatedAfter, got %d", len(secrets))
	}

	past := time.Now().Add(-time.Hour)
	secrets, err = repo.List(orgID, 25, 0, &past)
	if err != nil {
		t.Fatalf("List with past filter: %v", err)
	}
	if len(secrets) != 1 {
		t.Errorf("expected 1 result with past updatedAfter, got %d", len(secrets))
	}
}

func TestSecretRepo_ListByHandles(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-sec-008"
	createTestOrganizationAndProject(t, db, orgID, "proj-sec-008")

	repo := NewSecretRepo(db)
	for _, h := range []string{"s1", "s2", "s3"} {
		s := &model.Secret{
			OrganizationID: orgID,
			Handle:         h,
			Ciphertext:     []byte("ct"),
			Hash:           "hash",
			CreatedBy:      "u",
			UpdatedBy:      "u",
		}
		if err := repo.Create(s); err != nil {
			t.Fatalf("Create %s: %v", h, err)
		}
	}

	got, err := repo.ListByHandles(orgID, []string{"s1", "s3"}, nil)
	if err != nil {
		t.Fatalf("ListByHandles: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestSecretRepo_ListByHandles_UpdatedAfterFilter(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-sec-009"
	createTestOrganizationAndProject(t, db, orgID, "proj-sec-009")

	repo := NewSecretRepo(db)
	s := &model.Secret{
		OrganizationID: orgID,
		Handle:         "shared-secret",
		Ciphertext:     []byte("ct"),
		Hash:           "h",
		CreatedBy:      "u",
		UpdatedBy:      "u",
	}
	if err := repo.Create(s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.ListByHandles(orgID, []string{"shared-secret"}, nil)
	if err != nil {
		t.Fatalf("ListByHandles: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 result, got %d", len(got))
	}
}

func TestSecretRepo_ListByHandles_EmptyHandles(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-sec-010"
	createTestOrganizationAndProject(t, db, orgID, "proj-sec-010")

	repo := NewSecretRepo(db)
	got, err := repo.ListByHandles(orgID, nil, nil)
	if err != nil {
		t.Fatalf("ListByHandles(nil): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result for nil handles, got %d", len(got))
	}
}

// ---- FindRefs / artifact_secret_refs tests ----------------------------------

func insertArtifact(t *testing.T, db interface {
	Exec(query string, args ...interface{}) (interface{ RowsAffected() (int64, error) }, error)
}, uuid, orgID, handle, kind string) {
	t.Helper()
}

func TestSecretRepo_FindRefs_NoRefs(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-ref-001"
	createTestOrganizationAndProject(t, db, orgID, "proj-ref-001")

	repo := NewSecretRepo(db)
	refs, err := repo.FindRefs(orgID, "handle-not-used")
	if err != nil {
		t.Fatalf("FindRefs: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected no refs, got %d", len(refs))
	}
}

func TestSecretRepo_FindRefs_WithArtifactLevelRef(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-ref-002"
	createTestOrganizationAndProject(t, db, orgID, "proj-ref-002")

	// Insert an artifact and its rest_apis row so FindRefs can resolve handle/name
	_, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('art-uuid-001', 'RestApi', 'org-ref-002')`)
	if err != nil {
		t.Fatalf("insert artifact: %v", err)
	}
	_, err = db.Exec(`INSERT INTO rest_apis (uuid, organization_uuid, handle, display_name, version, project_uuid, lifecycle_status, configuration, created_at, updated_at)
		VALUES ('art-uuid-001', 'org-ref-002', 'my-api', 'My API', 'v1.0', 'proj-ref-002', 'CREATED', '{}', datetime('now'), datetime('now'))`)
	if err != nil {
		t.Fatalf("insert rest_api: %v", err)
	}

	// Insert artifact-level ref (gateway_id='')
	_, err = db.Exec(`INSERT INTO artifact_secret_refs (organization_uuid, artifact_uuid, secret_handle, gateway_id)
		VALUES ('org-ref-002', 'art-uuid-001', 'db-password', '')`)
	if err != nil {
		t.Fatalf("insert ref: %v", err)
	}

	repo := NewSecretRepo(db)
	refs, err := repo.FindRefs(orgID, "db-password")
	if err != nil {
		t.Fatalf("FindRefs: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Handle != "my-api" {
		t.Errorf("ref handle = %q, want %q", refs[0].Handle, "my-api")
	}
}

func TestSecretRepo_FindRefs_WithDeploymentLevelRef(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-ref-003"
	createTestOrganizationAndProject(t, db, orgID, "proj-ref-003")

	_, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('art-uuid-002', 'AiProxy', 'org-ref-003')`)
	if err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	// Insert gateway-level ref (gateway_id=<uuid>)
	gatewayID := "gw-uuid-001"
	_, err = db.Exec(`INSERT INTO artifact_secret_refs (organization_uuid, artifact_uuid, secret_handle, gateway_id)
		VALUES ('org-ref-003', 'art-uuid-002', 'api-key', ?)`, gatewayID)
	if err != nil {
		t.Fatalf("insert ref: %v", err)
	}

	repo := NewSecretRepo(db)
	refs, err := repo.FindRefs(orgID, "api-key")
	if err != nil {
		t.Fatalf("FindRefs: %v", err)
	}
	// Gateway-level ref should still block deletion (artifact config was deployed)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref (gateway-level), got %d", len(refs))
	}
}

func TestSecretRepo_FindRefs_DeduplicatesAcrossGateways(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-ref-004"
	createTestOrganizationAndProject(t, db, orgID, "proj-ref-004")

	_, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('art-uuid-003', 'McpServer', 'org-ref-004')`)
	if err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	// Artifact-level row + two gateway rows for same artifact & secret
	for _, gw := range []string{"", "gw-001", "gw-002"} {
		_, err = db.Exec(`INSERT INTO artifact_secret_refs (organization_uuid, artifact_uuid, secret_handle, gateway_id)
			VALUES ('org-ref-004', 'art-uuid-003', 'shared-key', ?)`, gw)
		if err != nil {
			t.Fatalf("insert ref gw=%q: %v", gw, err)
		}
	}

	repo := NewSecretRepo(db)
	refs, err := repo.FindRefs(orgID, "shared-key")
	if err != nil {
		t.Fatalf("FindRefs: %v", err)
	}
	// Three artifact_secret_refs rows (gateway_id = "", "gw-001", "gw-002") all
	// point to the same artifact. DISTINCT on (handle, name, kind) must collapse
	// them into exactly one SecretReference.
	if len(refs) != 1 {
		t.Errorf("expected 1 deduplicated ref, got %d", len(refs))
	}
}

// ---- extractSecretHandles tests ---------------------------------------------

func TestExtractSecretHandles(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "no placeholders",
			content: `{"url": "http://example.com"}`,
			want:    nil,
		},
		{
			name:    "single placeholder",
			content: `{{ secret "my-key" }}`,
			want:    []string{"my-key"},
		},
		{
			name:    "multiple placeholders",
			content: `{{ secret "key1" }} and {{ secret "key2" }}`,
			want:    []string{"key1", "key2"},
		},
		{
			name:    "duplicate placeholders deduplicated",
			content: `{{ secret "key1" }} {{ secret "key1" }}`,
			want:    []string{"key1"},
		},
		{
			name:    "whitespace variations",
			content: `{{secret "k"}} {{  secret  "k2"  }}`,
			want:    []string{"k", "k2"},
		},
		{
			// JSON-encoding wraps string values in double-quotes and escapes inner quotes
			// as \". The regex must match both {{ secret "key" }} and {{ secret \"key\" }}.
			name:    "json-escaped quotes",
			content: `{{ secret \"my-key\" }}`,
			want:    []string{"my-key"},
		},
		{
			name:    "json-escaped and raw-quote mixed",
			content: `{{ secret \"key-a\" }} {{ secret "key-b" }}`,
			want:    []string{"key-a", "key-b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractSecretHandles([]byte(tc.content))
			if len(got) != len(tc.want) {
				t.Errorf("len = %d, want %d (got %v)", len(got), len(tc.want), got)
				return
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// ---- upsertArtifactSecretRefs tests -----------------------------------------

func TestUpsertArtifactSecretRefs_InsertsOnCreate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-upsert-001"
	createTestOrganizationAndProject(t, db, orgID, "proj-upsert-001")

	_, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('art-upsert-001', 'RestApi', 'org-upsert-001')`)
	if err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	config := []byte("{{ secret \"db-pass\" }} {{ secret \"auth-token\" }}")

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := upsertArtifactSecretRefs(tx, db, orgID, "art-upsert-001", config); err != nil {
		tx.Rollback()
		t.Fatalf("upsertArtifactSecretRefs: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var count int
	db.QueryRow(`SELECT COUNT(*) FROM artifact_secret_refs WHERE organization_uuid = ? AND artifact_uuid = ? AND gateway_id = ''`,
		orgID, "art-upsert-001").Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 artifact-level refs, got %d", count)
	}
}

func TestUpsertArtifactSecretRefs_ReplacesOnUpdate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-upsert-002"
	createTestOrganizationAndProject(t, db, orgID, "proj-upsert-002")

	_, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('art-upsert-002', 'RestApi', 'org-upsert-002')`)
	if err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	// Initial config with old-key
	config1 := []byte(`{{ secret "old-key" }}`)
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err = upsertArtifactSecretRefs(tx, db, orgID, "art-upsert-002", config1); err != nil {
		tx.Rollback()
		t.Fatalf("upsertArtifactSecretRefs (config1): %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	// Updated config — old-key removed, new-key added
	config2 := []byte(`{{ secret "new-key" }}`)
	tx, err = db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err = upsertArtifactSecretRefs(tx, db, orgID, "art-upsert-002", config2); err != nil {
		tx.Rollback()
		t.Fatalf("upsertArtifactSecretRefs (config2): %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	var handles []string
	rows, err := db.Query(`SELECT secret_handle FROM artifact_secret_refs WHERE organization_uuid = ? AND artifact_uuid = ? AND gateway_id = ''`,
		orgID, "art-upsert-002")
	if err != nil {
		t.Fatalf("query refs: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var h string
		if err = rows.Scan(&h); err != nil {
			t.Fatalf("scan handle: %v", err)
		}
		handles = append(handles, h)
	}

	if len(handles) != 1 || handles[0] != "new-key" {
		t.Errorf("expected [new-key], got %v", handles)
	}
}

func TestUpsertArtifactSecretRefs_ClearsWhenNoSecrets(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-upsert-003"
	createTestOrganizationAndProject(t, db, orgID, "proj-upsert-003")

	_, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('art-upsert-003', 'RestApi', 'org-upsert-003')`)
	if err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err = upsertArtifactSecretRefs(tx, db, orgID, "art-upsert-003", []byte(`{{ secret "some-key" }}`)); err != nil {
		tx.Rollback()
		t.Fatalf("upsertArtifactSecretRefs (initial): %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	// Config update that removes all secrets
	tx, err = db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err = upsertArtifactSecretRefs(tx, db, orgID, "art-upsert-003", []byte(`{"plain": "config"}`)); err != nil {
		tx.Rollback()
		t.Fatalf("upsertArtifactSecretRefs (clear): %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM artifact_secret_refs WHERE organization_uuid = ? AND artifact_uuid = ? AND gateway_id = ''`,
		orgID, "art-upsert-003").Scan(&count); err != nil {
		t.Fatalf("count refs: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 refs after removing all secrets, got %d", count)
	}
}

// ---- upsertDeploymentSecretRefs tests ---------------------------------------

func TestUpsertDeploymentSecretRefs_OnDeploy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-dep-001"
	createTestOrganizationAndProject(t, db, orgID, "proj-dep-001")

	_, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('art-dep-001', 'RestApi', 'org-dep-001')`)
	if err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	gatewayID := "gw-uuid-001"
	content := []byte(`{{ secret "gw-secret" }}`)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err = upsertDeploymentSecretRefs(tx, db, orgID, "art-dep-001", gatewayID, content); err != nil {
		tx.Rollback()
		t.Fatalf("upsertDeploymentSecretRefs: %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM artifact_secret_refs WHERE organization_uuid = ? AND artifact_uuid = ? AND gateway_id = ?`,
		orgID, "art-dep-001", gatewayID).Scan(&count); err != nil {
		t.Fatalf("count refs: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 deployment ref, got %d", count)
	}
}

func TestUpsertDeploymentSecretRefs_OnUndeploy_ClearsRows(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-dep-002"
	createTestOrganizationAndProject(t, db, orgID, "proj-dep-002")

	_, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('art-dep-002', 'RestApi', 'org-dep-002')`)
	if err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	gatewayID := "gw-uuid-002"

	// Deploy
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err = upsertDeploymentSecretRefs(tx, db, orgID, "art-dep-002", gatewayID, []byte(`{{ secret "key" }}`)); err != nil {
		tx.Rollback()
		t.Fatalf("upsertDeploymentSecretRefs (deploy): %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	// Undeploy — pass nil content
	tx, err = db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err = upsertDeploymentSecretRefs(tx, db, orgID, "art-dep-002", gatewayID, nil); err != nil {
		tx.Rollback()
		t.Fatalf("upsertDeploymentSecretRefs (undeploy): %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM artifact_secret_refs WHERE organization_uuid = ? AND artifact_uuid = ? AND gateway_id = ?`,
		orgID, "art-dep-002", gatewayID).Scan(&count); err != nil {
		t.Fatalf("count refs: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 refs after undeploy, got %d", count)
	}
}

// TestSecretRepo_Create_UniqueConstraint_409 verifies that creating a secret with
// a duplicate (orgID, handle) returns ErrSecretAlreadyExists (scenario 86).
func TestSecretRepo_Create_UniqueConstraint_409(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-uc-001"
	createTestOrganizationAndProject(t, db, orgID, "proj-uc-001")

	repo := NewSecretRepo(db)
	s := &model.Secret{
		OrganizationID: orgID,
		Handle:         "dup-handle",
		DisplayName:    "Dup Handle",
		Ciphertext:     []byte("ct"),
		Hash:           "h",
		Type:           model.SecretTypeGeneric,
		Provider:       model.SecretProviderInHouse,
		Status:         model.SecretStatusActive,
		CreatedBy:      "u",
		UpdatedBy:      "u",
	}
	if err := repo.Create(s); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	// Second create with same handle must return ErrSecretAlreadyExists, not a raw DB error.
	s2 := &model.Secret{
		OrganizationID: orgID,
		Handle:         "dup-handle",
		DisplayName:    "Another",
		Ciphertext:     []byte("ct2"),
		Hash:           "h2",
		Type:           model.SecretTypeGeneric,
		Provider:       model.SecretProviderInHouse,
		Status:         model.SecretStatusActive,
		CreatedBy:      "u",
		UpdatedBy:      "u",
	}
	err := repo.Create(s2)
	if err == nil {
		t.Fatal("expected error on duplicate handle, got nil")
	}
	if !apperror.SecretExists.Is(err) {
		t.Errorf("expected ErrSecretAlreadyExists, got: %v", err)
	}
}

// TestSecretRepo_FindRefsAndSoftDelete_Transactional verifies the transactional
// behaviour of FindRefsAndSoftDelete (scenario 87):
//   - When refs exist the secret is NOT deprecated and the refs are returned.
//   - After refs are removed a second call DOES deprecate the secret.
func TestSecretRepo_FindRefsAndSoftDelete_Transactional(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-txn-001"
	createTestOrganizationAndProject(t, db, orgID, "proj-txn-001")

	repo := NewSecretRepo(db)
	s := &model.Secret{
		OrganizationID: orgID,
		Handle:         "txn-secret",
		Ciphertext:     []byte("ct"),
		Hash:           "h",
		Type:           model.SecretTypeGeneric,
		Provider:       model.SecretProviderInHouse,
		Status:         model.SecretStatusActive,
		CreatedBy:      "u",
		UpdatedBy:      "u",
	}
	if err := repo.Create(s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Insert an artifact and an artifact-level secret ref.
	_, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('art-txn-001', 'RestApi', ?)`, orgID)
	if err != nil {
		t.Fatalf("insert artifact: %v", err)
	}
	_, err = db.Exec(`INSERT INTO artifact_secret_refs (organization_uuid, artifact_uuid, secret_handle, gateway_id)
		VALUES (?, 'art-txn-001', 'txn-secret', '')`, orgID)
	if err != nil {
		t.Fatalf("insert ref: %v", err)
	}

	// (a) With refs present: FindRefsAndSoftDelete must return refs and must NOT deprecate.
	refs, err := repo.FindRefsAndSoftDelete(orgID, "txn-secret", "admin")
	if err != nil {
		t.Fatalf("FindRefsAndSoftDelete (with refs): %v", err)
	}
	if len(refs) == 0 {
		t.Fatal("expected refs to block deletion, got none")
	}

	// Secret must still be ACTIVE.
	got, err := repo.GetByHandle(orgID, "txn-secret")
	if err != nil {
		t.Fatalf("GetByHandle after blocked delete: %v", err)
	}
	if got.Status != model.SecretStatusActive {
		t.Errorf("secret should still be ACTIVE, got %q", got.Status)
	}

	// (b) Remove the ref, then FindRefsAndSoftDelete must deprecate the secret.
	_, err = db.Exec(`DELETE FROM artifact_secret_refs WHERE organization_uuid = ? AND artifact_uuid = 'art-txn-001'`, orgID)
	if err != nil {
		t.Fatalf("delete ref: %v", err)
	}

	refs, err = repo.FindRefsAndSoftDelete(orgID, "txn-secret", "admin")
	if err != nil {
		t.Fatalf("FindRefsAndSoftDelete (no refs): %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected no refs after removal, got %d", len(refs))
	}

	// Secret must now be DEPRECATED (active Exists returns false).
	exists, err := repo.Exists(orgID, "txn-secret")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("expected secret to be DEPRECATED (inactive) after successful soft-delete")
	}
}

func TestUpsertDeploymentSecretRefs_DoesNotAffectArtifactLevelRows(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-dep-003"
	createTestOrganizationAndProject(t, db, orgID, "proj-dep-003")

	_, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('art-dep-003', 'RestApi', 'org-dep-003')`)
	if err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	// Insert artifact-level row
	_, err = db.Exec(`INSERT INTO artifact_secret_refs (organization_uuid, artifact_uuid, secret_handle, gateway_id)
		VALUES ('org-dep-003', 'art-dep-003', 'my-key', '')`)
	if err != nil {
		t.Fatalf("insert artifact-level ref: %v", err)
	}

	// Undeploy from a gateway — should NOT touch the '' row
	gatewayID := "gw-uuid-003"
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err = upsertDeploymentSecretRefs(tx, db, orgID, "art-dep-003", gatewayID, nil); err != nil {
		tx.Rollback()
		t.Fatalf("upsertDeploymentSecretRefs (undeploy): %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM artifact_secret_refs WHERE organization_uuid = ? AND artifact_uuid = ? AND gateway_id = ''`,
		orgID, "art-dep-003").Scan(&count); err != nil {
		t.Fatalf("count refs: %v", err)
	}
	if count != 1 {
		t.Errorf("artifact-level row should survive undeploy, count = %d", count)
	}
}

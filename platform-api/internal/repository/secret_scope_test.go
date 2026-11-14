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
	"sort"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

func scopeKeys(scopes []model.SecretScope) []string {
	out := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		out = append(out, sc.Scope+"="+sc.ScopeValue)
	}
	sort.Strings(out)
	return out
}

func TestSecretRepo_GetScopeInfoByHandles(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-scope-001"
	createTestOrganizationAndProject(t, db, orgID, "proj-scope-001")
	repo := NewSecretRepo(db)

	scoped := &model.Secret{
		OrganizationID: orgID, Handle: "scoped", Ciphertext: []byte("ct"), Hash: "h",
		Status: model.SecretStatusActive, CreatedBy: "u", UpdatedBy: "u",
		Scopes: []model.SecretScope{
			{Scope: model.SecretScopeTypeProject, ScopeValue: "proj-scope-001"},
			{Scope: model.SecretScopeTypeArtifact, ScopeValue: "art-1"},
		},
	}
	if err := repo.Create(scoped); err != nil {
		t.Fatalf("Create scoped: %v", err)
	}
	unscoped := insertSecret(t, repo, orgID, "unscoped")

	got, err := repo.GetScopeInfoByHandles(orgID, []string{"scoped", "unscoped", "missing"})
	if err != nil {
		t.Fatalf("GetScopeInfoByHandles: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(got), got)
	}
	if got["scoped"].UUID != scoped.UUID || got["scoped"].Status != model.SecretStatusActive {
		t.Errorf("scoped info = %+v", got["scoped"])
	}
	if keys := scopeKeys(got["scoped"].Scopes); len(keys) != 2 ||
		keys[0] != "artifact=art-1" || keys[1] != "project=proj-scope-001" {
		t.Errorf("scoped scopes = %v", keys)
	}
	if got["unscoped"].UUID != unscoped.UUID || len(got["unscoped"].Scopes) != 0 {
		t.Errorf("unscoped info = %+v", got["unscoped"])
	}
	if _, ok := got["missing"]; ok {
		t.Error("a missing handle should have no entry")
	}

	// Another organization sees none of them.
	other, err := repo.GetScopeInfoByHandles("org-other", []string{"scoped"})
	if err != nil {
		t.Fatalf("GetScopeInfoByHandles other org: %v", err)
	}
	if len(other) != 0 {
		t.Errorf("other org got %+v", other)
	}

	empty, err := repo.GetScopeInfoByHandles(orgID, nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("no handles: got %+v, %v", empty, err)
	}
}

func TestSecretRepo_ReplaceScopes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-scope-002"
	createTestOrganizationAndProject(t, db, orgID, "proj-scope-002")
	repo := NewSecretRepo(db)

	s := &model.Secret{
		OrganizationID: orgID, Handle: "rotating", Ciphertext: []byte("ct"), Hash: "h",
		Status: model.SecretStatusActive, CreatedBy: "u", UpdatedBy: "u",
		Scopes: []model.SecretScope{{Scope: model.SecretScopeTypeOrg, ScopeValue: orgID}},
	}
	if err := repo.Create(s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.ReplaceScopes(s.UUID, []model.SecretScope{
		{Scope: model.SecretScopeTypeProject, ScopeValue: "proj-scope-002"},
		{Scope: model.SecretScopeTypeArtifact, ScopeValue: "art-9"},
	}); err != nil {
		t.Fatalf("ReplaceScopes: %v", err)
	}
	got, err := repo.GetScopeInfoByHandles(orgID, []string{"rotating"})
	if err != nil {
		t.Fatalf("GetScopeInfoByHandles: %v", err)
	}
	if keys := scopeKeys(got["rotating"].Scopes); len(keys) != 2 ||
		keys[0] != "artifact=art-9" || keys[1] != "project=proj-scope-002" {
		t.Errorf("scopes after replace = %v", keys)
	}

	if err := repo.ReplaceScopes(s.UUID, []model.SecretScope{{Scope: model.SecretScopeTypeOrg, ScopeValue: orgID}}); err != nil {
		t.Fatalf("ReplaceScopes back to org: %v", err)
	}
	got, _ = repo.GetScopeInfoByHandles(orgID, []string{"rotating"})
	if keys := scopeKeys(got["rotating"].Scopes); len(keys) != 1 || keys[0] != "org="+orgID {
		t.Errorf("scopes after second replace = %v", keys)
	}
}

func TestSecretRepo_FindRefArtifactUUIDs(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgID := "org-scope-003"
	createTestOrganizationAndProject(t, db, orgID, "proj-scope-003")
	for _, id := range []string{"art-a", "art-b", "art-c"} {
		if _, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, 'RestApi', ?)`, id, orgID); err != nil {
			t.Fatalf("insert artifact: %v", err)
		}
	}
	for _, row := range [][3]string{
		{"art-a", "shared-key", ""},
		{"art-a", "shared-key", "gw-1"},
		{"art-b", "shared-key", "gw-2"},
		{"art-c", "other-key", ""},
	} {
		if _, err := db.Exec(`INSERT INTO artifact_secret_refs (organization_uuid, artifact_uuid, secret_handle, gateway_id)
			VALUES (?, ?, ?, ?)`, orgID, row[0], row[1], row[2]); err != nil {
			t.Fatalf("insert ref: %v", err)
		}
	}

	repo := NewSecretRepo(db)
	got, err := repo.FindRefArtifactUUIDs(orgID, "shared-key")
	if err != nil {
		t.Fatalf("FindRefArtifactUUIDs: %v", err)
	}
	sort.Strings(got)
	if len(got) != 2 || got[0] != "art-a" || got[1] != "art-b" {
		t.Errorf("artifacts = %v, want [art-a art-b]", got)
	}

	none, err := repo.FindRefArtifactUUIDs("org-other", "shared-key")
	if err != nil || len(none) != 0 {
		t.Errorf("other org: got %v, %v", none, err)
	}
}

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

	"github.com/wso2/api-platform/platform-api/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

func TestLLMProviderRepoUpdateWithCustomPolicyUsagesRollsBackOnInsertFailure(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const (
		orgUUID     = "org-policy-rollback"
		projectUUID = "project-policy-rollback"
	)
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	templateRepo := NewLLMProviderTemplateRepo(db)
	template := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               "policy-template",
		GroupID:          "policy-template",
		Name:             "Policy Template",
		ManagedBy:        "organization",
		Version:          "v1.0",
	}
	if err := templateRepo.Create(template); err != nil {
		t.Fatalf("create template: %v", err)
	}

	customPolicyRepo := NewCustomPolicyRepo(db)
	policy := &model.CustomPolicy{
		UUID:             "policy-existing",
		OrganizationUUID: orgUUID,
		Name:             "custom-policy",
		Version:          "v1.0.0",
	}
	if err := customPolicyRepo.InsertCustomPolicy(policy); err != nil {
		t.Fatalf("create custom policy: %v", err)
	}

	providerRepo := NewLLMProviderRepo(db)
	provider := &model.LLMProvider{
		OrganizationUUID: orgUUID,
		ID:               "provider",
		Name:             "Original name",
		Version:          "v1.0",
		TemplateUUID:     template.UUID,
	}
	if err := providerRepo.CreateWithCustomPolicyUsages(provider, []string{policy.UUID}); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	provider.Name = "Changed name"
	err := providerRepo.UpdateWithCustomPolicyUsages(provider, []string{"missing-policy"})
	if err == nil || !strings.Contains(err.Error(), "failed to persist custom policy usages") {
		t.Fatalf("UpdateWithCustomPolicyUsages() error = %v, want policy usage insert failure", err)
	}

	stored, err := providerRepo.GetByID(provider.ID, orgUUID)
	if err != nil {
		t.Fatalf("get provider after failed update: %v", err)
	}
	if stored.Name != "Original name" {
		t.Fatalf("provider name = %q, want transaction rollback to preserve Original name", stored.Name)
	}
	usages, err := customPolicyRepo.GetCustomPolicyUsagesByAPIUUID(provider.UUID)
	if err != nil {
		t.Fatalf("get usages after failed update: %v", err)
	}
	if len(usages) != 1 || usages[0] != policy.UUID {
		t.Fatalf("policy usages = %v, want [%s]", usages, policy.UUID)
	}
}

// TestLLMProviderTemplateRepo_GetByID_ExactVersion verifies that GetByID honours
// the exact handle it is given (rather than always returning the family's latest
// version), and returns nil for an unknown handle. This is the resolution that
// keeps a provider bound to the specific template version selected at creation
// time.
func TestLLMProviderTemplateRepo_GetByID_ExactVersion(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewLLMProviderTemplateRepo(db)

	orgUUID := "org-tpl-001"
	projectUUID := "project-tpl-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	now := time.Now()
	// Built-in v1.0 — seeded with handle == group_id (no version suffix), as
	// the template loader does (managedBy "wso2").
	v1 := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               "mistralai",
		GroupID:          "mistralai",
		Name:             "Mistral",
		ManagedBy:        "wso2",
		Version:          "v1.0",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := repo.Create(v1); err != nil {
		t.Fatalf("failed to create v1.0: %v", err)
	}
	v1UUID := v1.UUID

	// Custom v2.0 spun off the built-in — version-suffixed handle, managedBy
	// "organization"; this demotes v1.0 and becomes the family's latest.
	v2 := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               "mistralai-v2-0",
		GroupID:          "mistralai",
		Name:             "Mistral",
		ManagedBy:        "organization",
		Version:          "v2.0",
	}
	if err := repo.CreateNewVersion(v2); err != nil {
		t.Fatalf("failed to create v2.0: %v", err)
	}

	// Built-in handle must resolve to its own v1.0 — NOT the latest (v2.0). This
	// is the bug fix: previously GetByID always returned the is_latest row.
	got, err := repo.GetByID("mistralai", orgUUID)
	if err != nil {
		t.Fatalf("GetByID(builtin) error: %v", err)
	}
	if got == nil || got.Version != "v1.0" || got.UUID != v1UUID {
		t.Fatalf("GetByID(builtin) = %+v, want version v1.0 (uuid %s)", got, v1UUID)
	}
	if got.ManagedBy != "wso2" {
		t.Errorf("GetByID(builtin).ManagedBy = %q, want wso2", got.ManagedBy)
	}

	// The version-specific custom handle must return v2.0.
	got, err = repo.GetByID("mistralai-v2-0", orgUUID)
	if err != nil {
		t.Fatalf("GetByID(v2) error: %v", err)
	}
	if got == nil || got.Version != "v2.0" || got.ManagedBy != "organization" {
		t.Fatalf("GetByID(v2) = %+v, want version v2.0 (managedBy organization)", got)
	}

	// An unknown handle resolves to nothing (no spurious latest match).
	got, err = repo.GetByID("does-not-exist", orgUUID)
	if err != nil {
		t.Fatalf("GetByID(unknown) error: %v", err)
	}
	if got != nil {
		t.Fatalf("GetByID(unknown) = %+v, want nil", got)
	}
}

// CreateImportedVersion must decide is_latest from the family it reads inside its own transaction
// (not from any pre-computed value), so a lower version added after a higher one never steals the
// latest slot and a genuine duplicate version is rejected. This is the regression guard for the
// concurrent-import latest race: the decision + demotion + insert are one atomic unit.
func TestLLMProviderTemplateRepo_CreateImportedVersion_DecidesLatestFromFamily(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewLLMProviderTemplateRepo(db)
	orgUUID := "org-imp-001"
	projectUUID := "project-imp-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	// Seed the family's first version as the current latest.
	v2 := &model.LLMProviderTemplate{OrganizationUUID: orgUUID, ID: "openai-v2", GroupID: "openai", Name: "OpenAI", Version: "v2.0", Origin: "gateway_api"}
	madeLatest, err := repo.CreateImportedVersion(v2)
	if err != nil || !madeLatest {
		t.Fatalf("CreateImportedVersion(v2.0) = (%v, %v), want (true, nil)", madeLatest, err)
	}

	// A lower version arriving afterwards must NOT become latest and must NOT demote v2.0.
	v1 := &model.LLMProviderTemplate{OrganizationUUID: orgUUID, ID: "openai-v1", GroupID: "openai", Name: "OpenAI", Version: "v1.0", Origin: "gateway_api"}
	madeLatest, err = repo.CreateImportedVersion(v1)
	if err != nil {
		t.Fatalf("CreateImportedVersion(v1.0) error: %v", err)
	}
	if madeLatest {
		t.Errorf("v1.0 must not become latest when v2.0 already exists")
	}
	if got, _ := repo.GetByID("openai-v2", orgUUID); got == nil || !got.IsLatest {
		t.Errorf("v2.0 must remain is_latest after a lower version is added")
	}

	// A higher version arriving afterwards becomes latest and demotes v2.0.
	v3 := &model.LLMProviderTemplate{OrganizationUUID: orgUUID, ID: "openai-v3", GroupID: "openai", Name: "OpenAI", Version: "v3.0", Origin: "gateway_api"}
	madeLatest, err = repo.CreateImportedVersion(v3)
	if err != nil || !madeLatest {
		t.Fatalf("CreateImportedVersion(v3.0) = (%v, %v), want (true, nil)", madeLatest, err)
	}
	if got, _ := repo.GetByID("openai-v2", orgUUID); got == nil || got.IsLatest {
		t.Errorf("v2.0 must be demoted once v3.0 joins the family")
	}

	// A duplicate version (different handle, same group_id+version) is rejected, not retried forever.
	dup := &model.LLMProviderTemplate{OrganizationUUID: orgUUID, ID: "openai-v3-dup", GroupID: "openai", Name: "OpenAI", Version: "v3.0", Origin: "gateway_api"}
	if _, err := repo.CreateImportedVersion(dup); err == nil {
		t.Errorf("CreateImportedVersion(duplicate v3.0) = nil error, want a version-exists error")
	}

	// Exactly one latest survives in the family.
	all, err := repo.ListVersions("openai", orgUUID, 100, 0)
	if err != nil {
		t.Fatalf("ListVersions error: %v", err)
	}
	latestCount := 0
	for _, v := range all {
		if v.IsLatest {
			latestCount++
		}
	}
	if latestCount != 1 {
		t.Errorf("family has %d is_latest rows, want exactly 1", latestCount)
	}
}

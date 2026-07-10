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

	"github.com/wso2/api-platform/platform-api/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

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
		GroupID:   "mistralai",
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
		GroupID:   "mistralai",
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

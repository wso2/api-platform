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

	"github.com/wso2/api-platform/platform-api/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

// TestLLMProviderRepo_CreateAndRead_SetsUpdatedBy guards against a prior gap
// where the llm_providers INSERT omitted updated_by, and GetByID/List never
// selected it back even after it was persisted.
func TestLLMProviderRepo_CreateAndRead_SetsUpdatedBy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-llm-provider-updatedby"
	projectUUID := "project-llm-provider-updatedby"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	templateRepo := NewLLMProviderTemplateRepo(db)
	template := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               "test-template",
		GroupID:          "test-template",
		Name:             "Test Template",
		ManagedBy:        "wso2",
		Version:          "v1.0",
	}
	if err := templateRepo.Create(template); err != nil {
		t.Fatalf("failed to create template fixture: %v", err)
	}

	providerRepo := NewLLMProviderRepo(db)
	provider := &model.LLMProvider{
		OrganizationUUID: orgUUID,
		ID:               "test-provider",
		Name:             "Test Provider",
		Version:          "v1.0",
		TemplateUUID:     template.UUID,
		CreatedBy:        "test-user",
		UpdatedBy:        "test-user",
	}
	if err := providerRepo.Create(provider); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := providerRepo.GetByID(provider.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}
	if got.UpdatedBy == "" {
		t.Fatal("expected updated_by to be set on creation, got empty string")
	}
	if got.UpdatedBy != got.CreatedBy {
		t.Fatalf("expected updated_by == created_by on creation, got created_by=%q updated_by=%q", got.CreatedBy, got.UpdatedBy)
	}

	list, err := providerRepo.List(orgUUID, 20, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || list[0].UpdatedBy != "test-user" {
		t.Fatalf("expected List to also return updated_by, got %+v", list)
	}
}

// TestLLMProxyRepo_CreateAndRead_SetsUpdatedBy guards against a prior gap
// where the llm_proxies INSERT omitted updated_by, and GetByID/List never
// selected it back even after it was persisted.
func TestLLMProxyRepo_CreateAndRead_SetsUpdatedBy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	orgUUID := "org-llm-proxy-updatedby"
	projectUUID := "project-llm-proxy-updatedby"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	templateRepo := NewLLMProviderTemplateRepo(db)
	template := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               "test-template-proxy",
		GroupID:          "test-template-proxy",
		Name:             "Test Template",
		ManagedBy:        "wso2",
		Version:          "v1.0",
	}
	if err := templateRepo.Create(template); err != nil {
		t.Fatalf("failed to create template fixture: %v", err)
	}

	providerRepo := NewLLMProviderRepo(db)
	provider := &model.LLMProvider{
		OrganizationUUID: orgUUID,
		ID:               "test-provider-for-proxy",
		Name:             "Test Provider",
		Version:          "v1.0",
		TemplateUUID:     template.UUID,
		CreatedBy:        "test-user",
		UpdatedBy:        "test-user",
	}
	if err := providerRepo.Create(provider); err != nil {
		t.Fatalf("failed to create provider fixture: %v", err)
	}

	proxyRepo := NewLLMProxyRepo(db)
	proxy := &model.LLMProxy{
		OrganizationUUID: orgUUID,
		ProjectUUID:      projectUUID,
		ID:               "test-proxy",
		Name:             "Test Proxy",
		Version:          "v1.0",
		ProviderUUID:     provider.UUID,
		CreatedBy:        "test-user",
		UpdatedBy:        "test-user",
	}
	if err := proxyRepo.Create(proxy); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := proxyRepo.GetByID(proxy.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}
	if got.UpdatedBy == "" {
		t.Fatal("expected updated_by to be set on creation, got empty string")
	}
	if got.UpdatedBy != got.CreatedBy {
		t.Fatalf("expected updated_by == created_by on creation, got created_by=%q updated_by=%q", got.CreatedBy, got.UpdatedBy)
	}

	list, err := proxyRepo.List(orgUUID, 20, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || list[0].UpdatedBy != "test-user" {
		t.Fatalf("expected List to also return updated_by, got %+v", list)
	}
}

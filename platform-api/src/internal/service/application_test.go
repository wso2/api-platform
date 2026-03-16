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
	"testing"
	"time"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

type mockApplicationRepository struct {
	repository.ApplicationRepository
	app                 *model.Application
	mappedKeys          []*model.ApplicationAPIKey
	apiKeysByLookupID   map[string]*model.ApplicationAPIKey
	existingByName      *model.Application
	appErr              error
	mappedErr           error
	existingByNameErr   error
	handleExists        bool
	handleExistsErr     error
	createErr           error
	addMappedCalled     bool
	replaceMappedCalled bool
	removeMappedCalled  bool
	createCalled        bool
	createdApplication  *model.Application
}

func (m *mockApplicationRepository) GetApplicationByIDOrHandle(appIDOrHandle, orgID string) (*model.Application, error) {
	return m.app, m.appErr
}

func (m *mockApplicationRepository) ListMappedAPIKeys(applicationUUID string) ([]*model.ApplicationAPIKey, error) {
	return m.mappedKeys, m.mappedErr
}

func (m *mockApplicationRepository) GetAPIKeyByID(keyID, orgID string) (*model.ApplicationAPIKey, error) {
	if m.apiKeysByLookupID == nil {
		return nil, nil
	}
	return m.apiKeysByLookupID[keyID], nil
}

func (m *mockApplicationRepository) AddApplicationAPIKeys(applicationUUID string, apiKeyIDs []string) error {
	m.addMappedCalled = true
	return nil
}

func (m *mockApplicationRepository) ReplaceApplicationAPIKeys(applicationUUID string, apiKeyIDs []string) error {
	m.replaceMappedCalled = true
	return nil
}

func (m *mockApplicationRepository) RemoveApplicationAPIKey(applicationUUID, apiKeyID string) error {
	m.removeMappedCalled = true
	return nil
}

func (m *mockApplicationRepository) GetApplicationByNameInProject(name, projectID, orgID string) (*model.Application, error) {
	return m.existingByName, m.existingByNameErr
}

func (m *mockApplicationRepository) CheckApplicationHandleExists(handle, orgID string) (bool, error) {
	return m.handleExists, m.handleExistsErr
}

func (m *mockApplicationRepository) CreateApplication(app *model.Application) error {
	m.createCalled = true
	m.createdApplication = app
	return m.createErr
}

type mockProjectRepository struct {
	repository.ProjectRepository
	projectByUUID    *model.Project
	projectByUUIDErr error
}

func (m *mockProjectRepository) GetProjectByUUID(projectID string) (*model.Project, error) {
	return m.projectByUUID, m.projectByUUIDErr
}

type mockApplicationOrganizationRepository struct {
	repository.OrganizationRepository
	org *model.Organization
	err error
}

func (m *mockApplicationOrganizationRepository) GetOrganizationByUUID(orgID string) (*model.Organization, error) {
	return m.org, m.err
}

func TestListMappedAPIKeys_ReturnsUnifiedMappingsWithMetadata(t *testing.T) {
	createdAt := time.Now().Add(-time.Hour)
	updatedAt := time.Now()

	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1"},
		mappedKeys: []*model.ApplicationAPIKey{
			{
				ID:             "key-1",
				APIKeyUUID:     "api-key-uuid-1",
				Name:           "my-key",
				ArtifactID:     "artifact-1",
				ArtifactHandle: "orders-api",
				ArtifactKind:   "RestApi",
				Status:         "ACTIVE",
				CreatedBy:      "user-1",
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
			},
			{
				ID:             "key-2",
				APIKeyUUID:     "api-key-uuid-2",
				Name:           "other-key",
				ArtifactID:     "artifact-2",
				ArtifactHandle: "payments-api",
				ArtifactKind:   "RestApi",
				Status:         "ACTIVE",
				CreatedBy:      "user-2",
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
			},
		},
	}

	svc := &ApplicationService{appRepo: appRepo}

	resp, err := svc.ListMappedAPIKeys("my-app", "org-1")
	if err != nil {
		t.Fatalf("ListMappedAPIKeys returned error: %v", err)
	}

	if resp.Count != 2 {
		t.Fatalf("expected count 2, got %d", resp.Count)
	}
	if len(resp.List) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(resp.List))
	}

	if resp.List[0].KeyId != "my-key" {
		t.Fatalf("expected first keyId my-key, got %s", resp.List[0].KeyId)
	}
	if resp.List[1].KeyId != "other-key" {
		t.Fatalf("expected second keyId other-key, got %s", resp.List[1].KeyId)
	}

	if resp.List[0].Status == nil || *resp.List[0].Status != "ACTIVE" {
		t.Fatalf("expected ACTIVE status for first mapping")
	}
	if resp.List[0].UserId == nil || *resp.List[0].UserId != "user-1" {
		t.Fatalf("expected first mapping userId user-1")
	}
	if resp.List[1].UserId == nil || *resp.List[1].UserId != "user-2" {
		t.Fatalf("expected second mapping userId user-2")
	}
}

func TestAddMappedAPIKeys_RejectsWhenRequesterIsNotCreator(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1"},
		apiKeysByLookupID: map[string]*model.ApplicationAPIKey{
			"key-1": {
				ID:        "api-key-db-id-1",
				Name:      "key-1",
				CreatedBy: "creator-user",
			},
		},
	}

	svc := &ApplicationService{appRepo: appRepo}

	_, err := svc.AddMappedAPIKeys("my-app", &dto.AddApplicationAPIKeysRequest{ApiKeyIds: []string{"key-1"}}, "org-1", "different-user")
	if !errors.Is(err, constants.ErrAPIKeyForbidden) {
		t.Fatalf("expected ErrAPIKeyForbidden, got %v", err)
	}
	if appRepo.addMappedCalled {
		t.Fatalf("expected AddApplicationAPIKeys not to be called when requester is not creator")
	}
}

func TestReplaceMappedAPIKeys_RejectsWhenRequesterIsNotCreator(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1"},
		apiKeysByLookupID: map[string]*model.ApplicationAPIKey{
			"key-1": {
				ID:        "api-key-db-id-1",
				Name:      "key-1",
				CreatedBy: "creator-user",
			},
		},
	}

	svc := &ApplicationService{appRepo: appRepo}

	_, err := svc.ReplaceMappedAPIKeys("my-app", &dto.ReplaceApplicationAPIKeysRequest{ApiKeyIds: []string{"key-1"}}, "org-1", "different-user")
	if !errors.Is(err, constants.ErrAPIKeyForbidden) {
		t.Fatalf("expected ErrAPIKeyForbidden, got %v", err)
	}
	if appRepo.replaceMappedCalled {
		t.Fatalf("expected ReplaceApplicationAPIKeys not to be called when requester is not creator")
	}
}

func TestReplaceMappedAPIKeys_AllowsRemovalForNonCreator(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1"},
		mappedKeys: []*model.ApplicationAPIKey{
			{
				ID:         "api-key-db-id-1",
				APIKeyUUID: "api-key-db-id-1",
				Name:       "key-1",
				CreatedBy:  "creator-user",
			},
		},
		apiKeysByLookupID: map[string]*model.ApplicationAPIKey{
			"key-1": {
				ID:        "api-key-db-id-1",
				Name:      "key-1",
				CreatedBy: "creator-user",
			},
		},
	}

	svc := &ApplicationService{appRepo: appRepo}

	_, err := svc.ReplaceMappedAPIKeys("my-app", &dto.ReplaceApplicationAPIKeysRequest{ApiKeyIds: []string{"key-1"}}, "org-1", "different-user")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !appRepo.replaceMappedCalled {
		t.Fatalf("expected ReplaceApplicationAPIKeys to be called")
	}
}

func TestRemoveMappedAPIKey_AllowsWhenRequesterIsNotCreator(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1"},
		apiKeysByLookupID: map[string]*model.ApplicationAPIKey{
			"key-1": {
				ID:        "api-key-db-id-1",
				Name:      "key-1",
				CreatedBy: "creator-user",
			},
		},
	}

	svc := &ApplicationService{appRepo: appRepo}

	err := svc.RemoveMappedAPIKey("my-app", "key-1", "org-1", "different-user")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !appRepo.removeMappedCalled {
		t.Fatalf("expected RemoveApplicationAPIKey to be called")
	}
}

func TestCreateApplication_LeavesProjectUnmappedWhenProjectIDMissing(t *testing.T) {
	orgID := "org-1"

	appRepo := &mockApplicationRepository{}
	projectRepo := &mockProjectRepository{}
	orgRepo := &mockApplicationOrganizationRepository{
		org: &model.Organization{ID: orgID},
	}

	svc := &ApplicationService{
		appRepo:     appRepo,
		projectRepo: projectRepo,
		orgRepo:     orgRepo,
	}

	resp, err := svc.CreateApplication(&dto.CreateApplicationRequest{
		Name:      "Sample App",
		ProjectId: "",
		Type:      "genai",
	}, orgID)
	if err != nil {
		t.Fatalf("CreateApplication returned error: %v", err)
	}

	if !appRepo.createCalled {
		t.Fatalf("expected CreateApplication repository method to be called")
	}
	if appRepo.createdApplication == nil {
		t.Fatalf("expected created application to be captured")
	}
	if appRepo.createdApplication.ProjectUUID != "" {
		t.Fatalf("expected application project UUID to be empty, got %s", appRepo.createdApplication.ProjectUUID)
	}
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
	if resp.ProjectId != "" {
		t.Fatalf("expected response projectId to be empty, got %s", resp.ProjectId)
	}
}

func TestCreateApplication_ValidatesProvidedProjectID(t *testing.T) {
	orgID := "org-1"

	appRepo := &mockApplicationRepository{}
	projectRepo := &mockProjectRepository{}
	orgRepo := &mockApplicationOrganizationRepository{
		org: &model.Organization{ID: orgID},
	}

	svc := &ApplicationService{
		appRepo:     appRepo,
		projectRepo: projectRepo,
		orgRepo:     orgRepo,
	}

	_, err := svc.CreateApplication(&dto.CreateApplicationRequest{
		Name:      "Sample App",
		ProjectId: "project-123",
		Type:      "genai",
	}, orgID)
	if !errors.Is(err, constants.ErrProjectNotFound) {
		t.Fatalf("expected ErrProjectNotFound, got %v", err)
	}
}

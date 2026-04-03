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

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type mockApplicationRepository struct {
	repository.ApplicationRepository
	app                *model.Application
	applications       []*model.Application
	mappedKeys         []*model.ApplicationAPIKey
	apiKeysByLookupKey map[string]*model.ApplicationAPIKey
	artifactByID       map[string]*model.Artifact
	deployedGatewayIDs map[string][]string
	existingByName     *model.Application
	appErr             error
	mappedErr          error
	artifactErr        error
	deployedGatewayErr error
	existingByNameErr  error
	handleExists       bool
	handleExistsErr    error
	createErr          error
	addMappedCalled    bool
	removeMappedCalled bool
	createCalled       bool
	createdApplication *model.Application
	addedAPIKeyIDs     []string
	removedAPIKeyID    string
}

func (m *mockApplicationRepository) GetApplicationByIDOrHandle(appIDOrHandle, orgID string) (*model.Application, error) {
	return m.app, m.appErr
}

func (m *mockApplicationRepository) GetApplicationsByProjectID(projectID, orgID string) ([]*model.Application, error) {
	if m.appErr != nil {
		return nil, m.appErr
	}
	return m.applications, nil
}

func (m *mockApplicationRepository) GetApplicationsByOrganizationID(orgID string) ([]*model.Application, error) {
	if m.appErr != nil {
		return nil, m.appErr
	}
	return m.applications, nil
}

func (m *mockApplicationRepository) GetApplicationsByProjectIDPaginated(projectID, orgID string, limit, offset int) ([]*model.Application, error) {
	if m.appErr != nil {
		return nil, m.appErr
	}

	end := offset + limit
	if offset > len(m.applications) {
		return []*model.Application{}, nil
	}
	if end > len(m.applications) {
		end = len(m.applications)
	}

	return m.applications[offset:end], nil
}

func (m *mockApplicationRepository) GetApplicationsByOrganizationIDPaginated(orgID string, limit, offset int) ([]*model.Application, error) {
	if m.appErr != nil {
		return nil, m.appErr
	}

	end := offset + limit
	if offset > len(m.applications) {
		return []*model.Application{}, nil
	}
	if end > len(m.applications) {
		end = len(m.applications)
	}

	return m.applications[offset:end], nil
}

func (m *mockApplicationRepository) CountApplicationsByProjectID(projectID, orgID string) (int, error) {
	if m.appErr != nil {
		return 0, m.appErr
	}

	return len(m.applications), nil
}

func (m *mockApplicationRepository) CountApplicationsByOrganizationID(orgID string) (int, error) {
	if m.appErr != nil {
		return 0, m.appErr
	}

	return len(m.applications), nil
}

func (m *mockApplicationRepository) ListMappedAPIKeys(applicationUUID string) ([]*model.ApplicationAPIKey, error) {
	return m.mappedKeys, m.mappedErr
}

func (m *mockApplicationRepository) GetAPIKeyByNameAndArtifactHandle(keyName, artifactHandle, orgID string) (*model.ApplicationAPIKey, error) {
	if m.apiKeysByLookupKey == nil {
		return nil, nil
	}
	return m.apiKeysByLookupKey[apiKeyLookupKey(keyName, artifactHandle)], nil
}

func (m *mockApplicationRepository) AddApplicationAPIKeys(applicationUUID string, apiKeyIDs []string) error {
	m.addMappedCalled = true
	m.addedAPIKeyIDs = append([]string(nil), apiKeyIDs...)
	return nil
}

func (m *mockApplicationRepository) RemoveApplicationAPIKey(applicationUUID, apiKeyID string) error {
	m.removeMappedCalled = true
	m.removedAPIKeyID = apiKeyID
	return nil
}

func (m *mockApplicationRepository) GetArtifactByUUID(artifactID, orgID string) (*model.Artifact, error) {
	if m.artifactErr != nil {
		return nil, m.artifactErr
	}
	if m.artifactByID == nil {
		return nil, nil
	}
	return m.artifactByID[artifactID], nil
}

func (m *mockApplicationRepository) GetDeployedGatewayIDsByArtifactUUID(artifactID, orgID string) ([]string, error) {
	if m.deployedGatewayErr != nil {
		return nil, m.deployedGatewayErr
	}
	if m.deployedGatewayIDs == nil {
		return nil, nil
	}
	return m.deployedGatewayIDs[artifactID], nil
}

func apiKeyLookupKey(keyName, artifactHandle string) string {
	return keyName + "|" + artifactHandle
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

	resp, err := svc.ListMappedAPIKeys("my-app", "org-1", 20, 0)
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
	if resp.List[0].AssociatedEntity.Id != "orders-api" {
		t.Fatalf("expected first mapping associated entity id orders-api, got %s", resp.List[0].AssociatedEntity.Id)
	}
}

func TestListMappedAPIKeys_AppliesPagination(t *testing.T) {
	createdAt := time.Now().Add(-time.Hour)
	updatedAt := time.Now()

	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1"},
		mappedKeys: []*model.ApplicationAPIKey{
			{ID: "key-1", APIKeyUUID: "uuid-1", Name: "key-1", ArtifactID: "artifact-1", ArtifactHandle: "api-1", ArtifactKind: "RestApi", CreatedAt: createdAt, UpdatedAt: updatedAt},
			{ID: "key-2", APIKeyUUID: "uuid-2", Name: "key-2", ArtifactID: "artifact-2", ArtifactHandle: "api-2", ArtifactKind: "RestApi", CreatedAt: createdAt, UpdatedAt: updatedAt},
			{ID: "key-3", APIKeyUUID: "uuid-3", Name: "key-3", ArtifactID: "artifact-3", ArtifactHandle: "api-3", ArtifactKind: "RestApi", CreatedAt: createdAt, UpdatedAt: updatedAt},
		},
	}

	svc := &ApplicationService{appRepo: appRepo}

	resp, err := svc.ListMappedAPIKeys("my-app", "org-1", 1, 1)
	if err != nil {
		t.Fatalf("ListMappedAPIKeys returned error: %v", err)
	}

	if resp.Count != 1 {
		t.Fatalf("expected count 1, got %d", resp.Count)
	}
	if len(resp.List) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(resp.List))
	}
	if resp.List[0].KeyId != "key-2" {
		t.Fatalf("expected paginated keyId key-2, got %s", resp.List[0].KeyId)
	}
	if resp.Pagination.Total != 3 {
		t.Fatalf("expected total 3, got %d", resp.Pagination.Total)
	}
	if resp.Pagination.Limit != 1 {
		t.Fatalf("expected limit 1, got %d", resp.Pagination.Limit)
	}
	if resp.Pagination.Offset != 1 {
		t.Fatalf("expected offset 1, got %d", resp.Pagination.Offset)
	}
}

func TestListMappedAPIKeys_LimitOneReturnsFirstPage(t *testing.T) {
	createdAt := time.Now().Add(-time.Hour)
	updatedAt := time.Now()

	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1"},
		mappedKeys: []*model.ApplicationAPIKey{
			{ID: "key-1", APIKeyUUID: "uuid-1", Name: "key-1", ArtifactID: "artifact-1", ArtifactHandle: "api-1", ArtifactKind: "RestApi", CreatedAt: createdAt, UpdatedAt: updatedAt},
			{ID: "key-2", APIKeyUUID: "uuid-2", Name: "key-2", ArtifactID: "artifact-2", ArtifactHandle: "api-2", ArtifactKind: "RestApi", CreatedAt: createdAt, UpdatedAt: updatedAt},
		},
	}

	svc := &ApplicationService{appRepo: appRepo}

	resp, err := svc.ListMappedAPIKeys("my-app", "org-1", 1, 0)
	if err != nil {
		t.Fatalf("ListMappedAPIKeys returned error: %v", err)
	}

	if resp.Count != 1 {
		t.Fatalf("expected count 1, got %d", resp.Count)
	}
	if len(resp.List) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(resp.List))
	}
	if resp.List[0].KeyId != "key-1" {
		t.Fatalf("expected first page keyId key-1, got %s", resp.List[0].KeyId)
	}
	if resp.Pagination.Total != 2 {
		t.Fatalf("expected total 2, got %d", resp.Pagination.Total)
	}
	if resp.Pagination.Limit != 1 {
		t.Fatalf("expected limit 1, got %d", resp.Pagination.Limit)
	}
	if resp.Pagination.Offset != 0 {
		t.Fatalf("expected offset 0, got %d", resp.Pagination.Offset)
	}
}

func TestGetApplicationsByOrganization_AppliesPagination(t *testing.T) {
	orgID := "org-1"
	projectID := "11111111-1111-1111-1111-111111111111"

	appRepo := &mockApplicationRepository{
		applications: []*model.Application{
			{UUID: "app-1", Handle: "app-1", OrganizationUUID: orgID, ProjectUUID: projectID, Name: "App 1", Type: "genai"},
			{UUID: "app-2", Handle: "app-2", OrganizationUUID: orgID, ProjectUUID: projectID, Name: "App 2", Type: "genai"},
			{UUID: "app-3", Handle: "app-3", OrganizationUUID: orgID, ProjectUUID: projectID, Name: "App 3", Type: "genai"},
		},
	}
	projectRepo := &mockProjectRepository{
		projectByUUID: &model.Project{ID: projectID, OrganizationID: orgID},
	}
	orgRepo := &mockApplicationOrganizationRepository{
		org: &model.Organization{ID: orgID},
	}

	svc := &ApplicationService{
		appRepo:     appRepo,
		projectRepo: projectRepo,
		orgRepo:     orgRepo,
	}

	resp, err := svc.GetApplicationsByOrganization(orgID, projectID, 1, 1)
	if err != nil {
		t.Fatalf("GetApplicationsByOrganization returned error: %v", err)
	}

	if resp.Count != 1 {
		t.Fatalf("expected count 1, got %d", resp.Count)
	}
	if len(resp.List) != 1 {
		t.Fatalf("expected list length 1, got %d", len(resp.List))
	}
	if resp.List[0].Id != "app-2" {
		t.Fatalf("expected app id app-2, got %s", resp.List[0].Id)
	}
	if resp.Pagination.Total != 3 {
		t.Fatalf("expected total 3, got %d", resp.Pagination.Total)
	}
	if resp.Pagination.Limit != 1 {
		t.Fatalf("expected limit 1, got %d", resp.Pagination.Limit)
	}
	if resp.Pagination.Offset != 1 {
		t.Fatalf("expected offset 1, got %d", resp.Pagination.Offset)
	}
}

func TestAddMappedAPIKeys_RejectsWhenRequesterIsNotCreator(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1"},
		apiKeysByLookupKey: map[string]*model.ApplicationAPIKey{
			apiKeyLookupKey("key-1", "orders-api"): {
				ID:        "api-key-db-id-1",
				Name:      "key-1",
				CreatedBy: "creator-user",
			},
		},
	}

	svc := &ApplicationService{appRepo: appRepo}

	_, err := svc.AddMappedAPIKeys("my-app", &api.AddApplicationAPIKeysRequest{ApiKeys: []api.APIKeyMappingSelector{{
		KeyId: "key-1",
		AssociatedEntity: api.APIKeyMappingAssociatedEntity{
			Id: "orders-api",
		},
	}}}, "org-1", "different-user")
	if !errors.Is(err, constants.ErrAPIKeyForbidden) {
		t.Fatalf("expected ErrAPIKeyForbidden, got %v", err)
	}
	if appRepo.addMappedCalled {
		t.Fatalf("expected AddApplicationAPIKeys not to be called when requester is not creator")
	}
}

func TestRemoveMappedAPIKey_AllowsWhenRequesterIsNotCreator(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1"},
		apiKeysByLookupKey: map[string]*model.ApplicationAPIKey{
			apiKeyLookupKey("key-1", "orders-api"): {
				ID:        "api-key-db-id-1",
				Name:      "key-1",
				CreatedBy: "creator-user",
			},
		},
	}

	svc := &ApplicationService{appRepo: appRepo}

	err := svc.RemoveMappedAPIKey("my-app", "key-1", "orders-api", "org-1", "different-user")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !appRepo.removeMappedCalled {
		t.Fatalf("expected RemoveApplicationAPIKey to be called")
	}
}

func TestAddMappedAPIKeys_ResolvesByAssociatedEntityID(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1"},
		apiKeysByLookupKey: map[string]*model.ApplicationAPIKey{
			apiKeyLookupKey("shared-key", "entity-a"): {
				ID:        "api-key-db-id-a",
				Name:      "shared-key",
				CreatedBy: "creator-user",
			},
			apiKeyLookupKey("shared-key", "entity-b"): {
				ID:        "api-key-db-id-b",
				Name:      "shared-key",
				CreatedBy: "creator-user",
			},
		},
	}

	svc := &ApplicationService{appRepo: appRepo}

	_, err := svc.AddMappedAPIKeys("my-app", &api.AddApplicationAPIKeysRequest{ApiKeys: []api.APIKeyMappingSelector{{
		KeyId: "shared-key",
		AssociatedEntity: api.APIKeyMappingAssociatedEntity{
			Id: "entity-b",
		},
	}}}, "org-1", "creator-user")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(appRepo.addedAPIKeyIDs) != 1 || appRepo.addedAPIKeyIDs[0] != "api-key-db-id-b" {
		t.Fatalf("expected add call to resolve entity-b key uuid, got %#v", appRepo.addedAPIKeyIDs)
	}
}

func TestAddMappedAPIKeys_DoesNotFailWhenBroadcastResolutionFails(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", Handle: "my-app", OrganizationUUID: "org-1"},
		apiKeysByLookupKey: map[string]*model.ApplicationAPIKey{
			apiKeyLookupKey("key-1", "orders-api"): {
				ID:        "api-key-db-id-1",
				Name:      "key-1",
				CreatedBy: "creator-user",
			},
		},
		mappedKeys: []*model.ApplicationAPIKey{
			{APIKeyUUID: "api-key-db-id-1", ArtifactID: "artifact-1"},
		},
		artifactErr: errors.New("artifact lookup failed"),
	}

	svc := &ApplicationService{appRepo: appRepo, gatewayEventsService: &GatewayEventsService{}}

	_, err := svc.AddMappedAPIKeys("my-app", &api.AddApplicationAPIKeysRequest{ApiKeys: []api.APIKeyMappingSelector{{
		KeyId: "key-1",
		AssociatedEntity: api.APIKeyMappingAssociatedEntity{
			Id: "orders-api",
		},
	}}}, "org-1", "creator-user")
	if err != nil {
		t.Fatalf("expected nil error when broadcast fails, got %v", err)
	}
	if !appRepo.addMappedCalled {
		t.Fatalf("expected AddApplicationAPIKeys to be called")
	}
}

func TestRemoveMappedAPIKey_DoesNotFailWhenBroadcastResolutionFails(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", Handle: "my-app", OrganizationUUID: "org-1"},
		apiKeysByLookupKey: map[string]*model.ApplicationAPIKey{
			apiKeyLookupKey("key-1", "orders-api"): {
				ID:         "api-key-db-id-1",
				APIKeyUUID: "api-key-db-id-1",
				Name:       "key-1",
				ArtifactID: "artifact-1",
				CreatedBy:  "creator-user",
			},
		},
		artifactErr: errors.New("artifact lookup failed"),
	}

	svc := &ApplicationService{appRepo: appRepo, gatewayEventsService: &GatewayEventsService{}}

	err := svc.RemoveMappedAPIKey("my-app", "key-1", "orders-api", "org-1", "creator-user")
	if err != nil {
		t.Fatalf("expected nil error when broadcast fails, got %v", err)
	}
	if !appRepo.removeMappedCalled {
		t.Fatalf("expected RemoveApplicationAPIKey to be called")
	}
}

func TestCreateApplication_RequiresProjectID(t *testing.T) {
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

	resp, err := svc.CreateApplication(&api.CreateApplicationRequest{
		Name: "Sample App",
		Type: api.ApplicationType("genai"),
	}, orgID)
	if !errors.Is(err, constants.ErrProjectNotFound) {
		t.Fatalf("expected ErrProjectNotFound, got %v", err)
	}
	if resp != nil {
		t.Fatalf("expected nil response when project id is missing")
	}
	if appRepo.createCalled {
		t.Fatalf("expected repository create not to be called when project id is missing")
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

	projectUUID := openapi_types.UUID(uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	_, err := svc.CreateApplication(&api.CreateApplicationRequest{
		Name:      "Sample App",
		ProjectId: projectUUID,
		Type:      api.ApplicationType("genai"),
	}, orgID)
	if !errors.Is(err, constants.ErrProjectNotFound) {
		t.Fatalf("expected ErrProjectNotFound, got %v", err)
	}
}

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
)

type mockApplicationRepository struct {
	repository.ApplicationRepository
	app                     *model.Application
	applications            []*model.Application
	mappedKeys              []*model.ApplicationAPIKey
	mappedAssociations      []*model.ApplicationAssociationTarget
	apiKeysByLookupKey      map[string]*model.ApplicationAPIKey
	artifactByID            map[string]*model.Artifact
	artifactsByLookup       map[string]*model.Artifact
	proxyProjectByID        map[string]string
	deployedGatewayIDs      map[string][]string
	existingByName          *model.Application
	appErr                  error
	mappedErr               error
	artifactErr             error
	deployedGatewayErr      error
	existingByNameErr       error
	handleExists            bool
	handleExistsErr         error
	createErr               error
	appsByAPIKey            []*model.Application
	appsByAPIKeyErr         error
	getAppsByKeyCalled      bool
	removeAllCalled         bool
	removedAllAPIKeyID      string
	deleteCalled            bool
	deployedGatewayLookups  []string
	addMappedCalled         bool
	removeMappedCalled      bool
	createCalled            bool
	createdApplication      *model.Application
	addedAPIKeyIDs          []string
	removedAPIKeyID         string
	addAssociationsCalled   bool
	addedAssociationIDs     []string
	removeAssociationCalled bool
	removedAssociationID    string
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

func (m *mockApplicationRepository) GetAssociationTargetByUUID(artifactID, orgID string) (*model.Artifact, error) {
	if m.artifactErr != nil {
		return nil, m.artifactErr
	}
	if m.artifactByID == nil {
		return nil, nil
	}
	return m.artifactByID[artifactID], nil
}

func (m *mockApplicationRepository) GetAssociationTargetByIDOrHandle(artifactIDOrHandle, orgID string) (*model.Artifact, error) {
	if m.artifactErr != nil {
		return nil, m.artifactErr
	}
	if m.artifactsByLookup == nil {
		return nil, nil
	}
	return m.artifactsByLookup[artifactIDOrHandle], nil
}

func (m *mockApplicationRepository) GetAssociationTargetByIDOrHandleAndKind(artifactIDOrHandle, kind, orgID string) (*model.Artifact, error) {
	if m.artifactErr != nil {
		return nil, m.artifactErr
	}
	if m.artifactsByLookup == nil {
		return nil, nil
	}
	artifact := m.artifactsByLookup[artifactIDOrHandle]
	if artifact == nil {
		return nil, nil
	}
	if artifact.Type != kind {
		return nil, nil
	}
	return artifact, nil
}

func (m *mockApplicationRepository) GetLLMProxyProjectUUID(artifactUUID, orgID string) (string, error) {
	if m.artifactErr != nil {
		return "", m.artifactErr
	}
	if m.proxyProjectByID == nil {
		return "", nil
	}
	return m.proxyProjectByID[artifactUUID], nil
}

func (m *mockApplicationRepository) ListApplicationAssociations(applicationUUID string) ([]*model.ApplicationAssociationTarget, error) {
	return m.mappedAssociations, m.mappedErr
}

func (m *mockApplicationRepository) AddApplicationAssociations(applicationUUID string, associationUUIDs []string) error {
	m.addAssociationsCalled = true
	m.addedAssociationIDs = append([]string(nil), associationUUIDs...)
	return nil
}

func (m *mockApplicationRepository) RemoveApplicationAssociation(applicationUUID, associationUUID string) error {
	m.removeAssociationCalled = true
	m.removedAssociationID = associationUUID
	return nil
}

func (m *mockApplicationRepository) GetApplicationsByAPIKeyID(apiKeyID, orgID string) ([]*model.Application, error) {
	m.getAppsByKeyCalled = true
	return m.appsByAPIKey, m.appsByAPIKeyErr
}

func (m *mockApplicationRepository) DeleteApplication(appID, orgID string) error {
	m.deleteCalled = true
	return nil
}

func (m *mockApplicationRepository) RemoveAPIKeyFromAllApplications(apiKeyID string) error {
	m.removeAllCalled = true
	m.removedAllAPIKeyID = apiKeyID
	return nil
}

func (m *mockApplicationRepository) GetDeployedGatewayIDsByArtifactUUID(artifactID, orgID string) ([]string, error) {
	m.deployedGatewayLookups = append(m.deployedGatewayLookups, artifactID)
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

func (m *mockProjectRepository) GetProjectByHandleAndOrgID(handle, orgID string) (*model.Project, error) {
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
				ArtifactType:   "RestApi",
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
				ArtifactType:   "RestApi",
				Status:         "ACTIVE",
				CreatedBy:      "user-2",
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
			},
		},
	}

	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

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
			{ID: "key-1", APIKeyUUID: "uuid-1", Name: "key-1", ArtifactID: "artifact-1", ArtifactHandle: "api-1", ArtifactType: "RestApi", CreatedAt: createdAt, UpdatedAt: updatedAt},
			{ID: "key-2", APIKeyUUID: "uuid-2", Name: "key-2", ArtifactID: "artifact-2", ArtifactHandle: "api-2", ArtifactType: "RestApi", CreatedAt: createdAt, UpdatedAt: updatedAt},
			{ID: "key-3", APIKeyUUID: "uuid-3", Name: "key-3", ArtifactID: "artifact-3", ArtifactHandle: "api-3", ArtifactType: "RestApi", CreatedAt: createdAt, UpdatedAt: updatedAt},
		},
	}

	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

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
			{ID: "key-1", APIKeyUUID: "uuid-1", Name: "key-1", ArtifactID: "artifact-1", ArtifactHandle: "api-1", ArtifactType: "RestApi", CreatedAt: createdAt, UpdatedAt: updatedAt},
			{ID: "key-2", APIKeyUUID: "uuid-2", Name: "key-2", ArtifactID: "artifact-2", ArtifactHandle: "api-2", ArtifactType: "RestApi", CreatedAt: createdAt, UpdatedAt: updatedAt},
		},
	}

	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

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

func TestListMappedAPIKeysForAssociation_FiltersToAssociation(t *testing.T) {
	createdAt := time.Now().Add(-time.Hour)
	updatedAt := time.Now()

	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1", ProjectUUID: "project-1"},
		artifactsByLookup: map[string]*model.Artifact{
			"provider-1": {UUID: "artifact-1", Handle: "provider-1", Type: constants.LLMProvider, OrganizationUUID: "org-1"},
		},
		mappedAssociations: []*model.ApplicationAssociationTarget{
			{TargetUUID: "artifact-1", TargetHandle: "provider-1", Type: constants.LLMProvider},
		},
		mappedKeys: []*model.ApplicationAPIKey{
			{ID: "key-1", APIKeyUUID: "uuid-1", Name: "key-1", ArtifactID: "artifact-1", ArtifactHandle: "provider-1", ArtifactType: constants.LLMProvider, CreatedAt: createdAt, UpdatedAt: updatedAt},
			{ID: "key-2", APIKeyUUID: "uuid-2", Name: "key-2", ArtifactID: "artifact-2", ArtifactHandle: "proxy-1", ArtifactType: constants.LLMProxy, CreatedAt: createdAt, UpdatedAt: updatedAt},
		},
	}

	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

	resp, err := svc.ListMappedAPIKeysForAssociation("my-app", "provider-1", "org-1", 20, 0)
	if err != nil {
		t.Fatalf("ListMappedAPIKeysForAssociation returned error: %v", err)
	}

	if resp.Count != 1 {
		t.Fatalf("expected count 1, got %d", resp.Count)
	}
	if len(resp.List) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(resp.List))
	}
	if resp.List[0].KeyId != "key-1" {
		t.Fatalf("expected key-1, got %s", resp.List[0].KeyId)
	}
	if resp.List[0].AssociatedEntity.Id != "provider-1" {
		t.Fatalf("expected associated entity provider-1, got %s", resp.List[0].AssociatedEntity.Id)
	}
	if resp.Pagination.Total != 1 {
		t.Fatalf("expected total 1, got %d", resp.Pagination.Total)
	}
	if resp.Pagination.Limit != 20 {
		t.Fatalf("expected limit 20, got %d", resp.Pagination.Limit)
	}
}

func TestListMappedAPIKeysForAssociation_ErrorsWhenAssociationMissing(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1", ProjectUUID: "project-1"},
		artifactsByLookup: map[string]*model.Artifact{
			"provider-1": {UUID: "artifact-1", Handle: "provider-1", Type: constants.LLMProvider, OrganizationUUID: "org-1"},
		},
		mappedAssociations: []*model.ApplicationAssociationTarget{},
	}

	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

	_, err := svc.ListMappedAPIKeysForAssociation("my-app", "provider-1", "org-1", 20, 0)
	if !errors.Is(err, constants.ErrArtifactNotFound) {
		t.Fatalf("expected ErrArtifactNotFound, got %v", err)
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
		projectByUUID: &model.Project{ID: projectID, Handle: projectID, OrganizationID: orgID},
	}
	orgRepo := &mockApplicationOrganizationRepository{
		org: &model.Organization{ID: orgID},
	}

	svc := &ApplicationService{
		appRepo:     appRepo,
		projectRepo: projectRepo,
		orgRepo:     orgRepo,
		identity:    newTestIdentityService(),
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

	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

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

	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

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

	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

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

	svc := &ApplicationService{appRepo: appRepo, gatewayEventsService: &GatewayEventsService{}, identity: newTestIdentityService()}

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

func TestAddApplicationAssociations_AssociatesProviderAndProxy(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", ProjectUUID: "project-1", OrganizationUUID: "org-1"},
		artifactsByLookup: map[string]*model.Artifact{
			"provider-1": {
				UUID:             "artifact-provider-1",
				Handle:           "provider-1",
				Type:             constants.LLMProvider,
				OrganizationUUID: "org-1",
			},
			"proxy-1": {
				UUID:             "artifact-proxy-1",
				Handle:           "proxy-1",
				Type:             constants.LLMProxy,
				OrganizationUUID: "org-1",
			},
		},
		proxyProjectByID: map[string]string{
			"artifact-proxy-1": "project-1",
		},
	}

	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

	_, err := svc.AddApplicationAssociations("my-app", &AddApplicationAssociationsRequest{Associations: []ApplicationAssociationSelector{
		{Id: "provider-1", Kind: constants.LLMProvider},
		{Id: "proxy-1", Kind: constants.LLMProxy},
	}}, "org-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !appRepo.addAssociationsCalled {
		t.Fatalf("expected AddApplicationAssociations to be called")
	}
	if len(appRepo.addedAssociationIDs) != 2 {
		t.Fatalf("expected 2 mapped associations, got %d", len(appRepo.addedAssociationIDs))
	}
}

func TestAddApplicationAssociations_RejectsCrossProjectProxy(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", ProjectUUID: "project-1", OrganizationUUID: "org-1"},
		artifactsByLookup: map[string]*model.Artifact{
			"proxy-1": {
				UUID:             "artifact-proxy-1",
				Handle:           "proxy-1",
				Type:             constants.LLMProxy,
				OrganizationUUID: "org-1",
			},
		},
		proxyProjectByID: map[string]string{
			"artifact-proxy-1": "project-2",
		},
	}

	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

	_, err := svc.AddApplicationAssociations("my-app", &AddApplicationAssociationsRequest{Associations: []ApplicationAssociationSelector{{
		Id:   "proxy-1",
		Kind: constants.LLMProxy,
	}}}, "org-1")
	if !errors.Is(err, constants.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestListApplicationAssociations_AppliesPagination(t *testing.T) {
	createdAt := time.Now().Add(-time.Hour)

	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", OrganizationUUID: "org-1"},
		mappedAssociations: []*model.ApplicationAssociationTarget{
			{TargetUUID: "artifact-1", TargetHandle: "provider-1", TargetName: "Provider 1", TargetVersion: "v1", Type: constants.LLMProvider, CreatedAt: createdAt},
			{TargetUUID: "artifact-2", TargetHandle: "proxy-1", TargetName: "Proxy 1", TargetVersion: "v1", Type: constants.LLMProxy, CreatedAt: createdAt},
		},
	}

	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

	resp, err := svc.ListApplicationAssociations("my-app", "org-1", 1, 0)
	if err != nil {
		t.Fatalf("ListApplicationAssociations returned error: %v", err)
	}
	if resp.Count != 1 || len(resp.List) != 1 {
		t.Fatalf("expected one item in first page, got count=%d len=%d", resp.Count, len(resp.List))
	}
	if resp.List[0].Id != "provider-1" {
		t.Fatalf("expected provider-1, got %s", resp.List[0].Id)
	}
}

func TestRemoveApplicationAssociation_RemovesByResolvedTarget(t *testing.T) {
	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-uuid", ProjectUUID: "project-1", OrganizationUUID: "org-1"},
		artifactsByLookup: map[string]*model.Artifact{
			"proxy-1": {
				UUID:             "artifact-proxy-1",
				Handle:           "proxy-1",
				Type:             constants.LLMProxy,
				OrganizationUUID: "org-1",
			},
		},
		proxyProjectByID: map[string]string{
			"artifact-proxy-1": "project-1",
		},
	}

	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

	err := svc.RemoveApplicationAssociation("my-app", "proxy-1", "org-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !appRepo.removeAssociationCalled {
		t.Fatalf("expected RemoveApplicationAssociation to be called")
	}
	if appRepo.removedAssociationID != "artifact-proxy-1" {
		t.Fatalf("expected removed association uuid artifact-proxy-1, got %s", appRepo.removedAssociationID)
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

	svc := &ApplicationService{appRepo: appRepo, gatewayEventsService: &GatewayEventsService{}, identity: newTestIdentityService()}

	err := svc.RemoveMappedAPIKey("my-app", "key-1", "orders-api", "org-1", "creator-user")
	if err != nil {
		t.Fatalf("expected nil error when broadcast fails, got %v", err)
	}
	if !appRepo.removeMappedCalled {
		t.Fatalf("expected RemoveApplicationAPIKey to be called")
	}
}

// TestSetAPIKeyApplication_DissociateBroadcastsToPriorApp verifies that dissociating a key (the
// apikey.application_updated event with a null application) captures the previously-owning
// application before removing the mapping and broadcasts a mapping update to that application's
// gateways — otherwise the gateway would keep the stale key→application mapping until pull-sync.
func TestSetAPIKeyApplication_DissociateBroadcastsToPriorApp(t *testing.T) {
	orgID := "org-1"

	appRepo := &mockApplicationRepository{
		apiKeysByLookupKey: map[string]*model.ApplicationAPIKey{
			apiKeyLookupKey("my-key", "order-api"): {
				ID:           "key-1",
				APIKeyUUID:   "key-uuid-1",
				Name:         "my-key",
				ArtifactID:   "artifact-1",
				ArtifactType: constants.RestApi,
			},
		},
		artifactByID: map[string]*model.Artifact{
			// Resolves the api ref_id to the artifact handle, and (as the broadcast hint) to a
			// supported artifact type so gateway targeting runs.
			"artifact-1": {UUID: "artifact-1", Handle: "order-api", Type: constants.RestApi},
		},
		// The key currently belongs to this application; the dissociation must broadcast for it.
		appsByAPIKey: []*model.Application{
			{UUID: "app-1", Handle: "my-app", OrganizationUUID: orgID},
		},
	}

	svc := &ApplicationService{appRepo: appRepo, gatewayEventsService: &GatewayEventsService{}}

	// appIDOrHandle empty => dissociation.
	if err := svc.SetAPIKeyApplication("my-key", "artifact-1", constants.RestApi, "", orgID, ""); err != nil {
		t.Fatalf("expected nil error on dissociation, got %v", err)
	}

	if !appRepo.getAppsByKeyCalled {
		t.Fatalf("expected GetApplicationsByAPIKeyID to be called before removing the mapping")
	}
	if !appRepo.removeAllCalled || appRepo.removedAllAPIKeyID != "key-1" {
		t.Fatalf("expected RemoveAPIKeyFromAllApplications for key-1, got called=%v id=%q",
			appRepo.removeAllCalled, appRepo.removedAllAPIKeyID)
	}
	// The broadcast must target the removed key's artifact so the right gateways are notified even
	// though the application now has no keys left.
	found := false
	for _, a := range appRepo.deployedGatewayLookups {
		if a == "artifact-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected dissociation to broadcast for the removed key's artifact (artifact-1); gateway lookups=%v",
			appRepo.deployedGatewayLookups)
	}
}

// TestDeleteApplication_BroadcastsMappingClear verifies that deleting an application broadcasts an
// empty mapping set to the gateways that held its keys, so they drop the key→application mappings
// (the Platform API DB clears them via cascade, but nothing else notifies the gateways).
func TestDeleteApplication_BroadcastsMappingClear(t *testing.T) {
	orgID := "org-1"

	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "app-1", Handle: "my-app", Name: "My App", Type: "web", OrganizationUUID: orgID},
		mappedKeys: []*model.ApplicationAPIKey{
			{ID: "key-1", APIKeyUUID: "key-uuid-1", ArtifactID: "artifact-1"},
		},
		artifactByID: map[string]*model.Artifact{
			"artifact-1": {UUID: "artifact-1", Handle: "order-api", Type: constants.RestApi},
		},
	}

	svc := &ApplicationService{appRepo: appRepo, gatewayEventsService: &GatewayEventsService{}, auditRepo: &noopAuditRepo{}}

	if err := svc.DeleteApplication("my-app", orgID, ""); err != nil {
		t.Fatalf("expected nil error deleting application, got %v", err)
	}
	if !appRepo.deleteCalled {
		t.Fatalf("expected the application to be deleted")
	}
	// The mapping-clear broadcast must target the deleted app's key artifact so the right gateways
	// are notified even though the application (and its remaining keys) are gone.
	found := false
	for _, a := range appRepo.deployedGatewayLookups {
		if a == "artifact-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected delete to broadcast for the removed key's artifact (artifact-1); gateway lookups=%v",
			appRepo.deployedGatewayLookups)
	}
}

func TestCreateApplication_AllowsMissingProjectID(t *testing.T) {
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
		auditRepo:   &noopAuditRepo{},
		identity:    newTestIdentityService(),
	}

	// project_uuid is optional: creating without a project id succeeds and persists a project-less
	// application rather than erroring.
	resp, err := svc.CreateApplication(&api.CreateApplicationRequest{
		DisplayName: "Sample App",
		Type:        api.ApplicationType("genai"),
	}, orgID, "")
	if err != nil {
		t.Fatalf("expected no error when project id is missing, got %v", err)
	}
	if !appRepo.createCalled {
		t.Fatalf("expected repository create to be called when project id is missing")
	}
	if appRepo.createdApplication == nil || appRepo.createdApplication.ProjectUUID != "" {
		t.Fatalf("expected created application to have an empty project uuid, got %+v", appRepo.createdApplication)
	}
	if resp == nil {
		t.Fatalf("expected a non-nil response")
	}
	if resp.ProjectId != "" {
		t.Fatalf("expected an empty projectId in the response, got %q", resp.ProjectId)
	}
}

func TestCreateApplication_AcceptsWebType(t *testing.T) {
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
		auditRepo:   &noopAuditRepo{},
	}

	// "web" is an accepted application type (alongside "genai"): webhook-reconciled applications
	// carry their own type, so creation must not be hardcoded to genai.
	resp, err := svc.CreateApplication(&api.CreateApplicationRequest{
		DisplayName: "My Mobile App",
		Type:        api.ApplicationType("web"),
	}, orgID, "")
	if err != nil {
		t.Fatalf("expected no error for web type, got %v", err)
	}
	if appRepo.createdApplication == nil || appRepo.createdApplication.Type != "web" {
		t.Fatalf("expected created application type 'web', got %+v", appRepo.createdApplication)
	}
	if resp == nil || resp.Type != api.ApplicationType("web") {
		t.Fatalf("expected response type 'web', got %+v", resp)
	}
}

func TestCreateApplication_RejectsUnsupportedType(t *testing.T) {
	orgID := "org-1"

	svc := &ApplicationService{
		appRepo:     &mockApplicationRepository{},
		projectRepo: &mockProjectRepository{},
		orgRepo:     &mockApplicationOrganizationRepository{org: &model.Organization{ID: orgID}},
		auditRepo:   &noopAuditRepo{},
	}

	_, err := svc.CreateApplication(&api.CreateApplicationRequest{
		DisplayName: "Bad Type App",
		Type:        api.ApplicationType("mobile"),
	}, orgID, "")
	if !errors.Is(err, constants.ErrUnsupportedApplicationType) {
		t.Fatalf("expected ErrUnsupportedApplicationType, got %v", err)
	}
}

func TestUpdateApplication_RejectsHandleChange(t *testing.T) {
	orgID := "org-1"

	appRepo := &mockApplicationRepository{
		app: &model.Application{UUID: "uuid-1", Handle: "my-app", Name: "My App", ProjectUUID: "proj-1"},
	}
	svc := &ApplicationService{appRepo: appRepo, identity: newTestIdentityService()}

	resp, err := svc.UpdateApplication("my-app", &api.Application{
		Id: "renamed-app",
	}, orgID, "user-1")
	if !errors.Is(err, constants.ErrHandleImmutable) {
		t.Fatalf("expected ErrHandleImmutable, got %v", err)
	}
	if resp != nil {
		t.Fatalf("expected nil response on handle mismatch")
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
		identity:    newTestIdentityService(),
	}

	_, err := svc.CreateApplication(&api.CreateApplicationRequest{
		DisplayName: "Sample App",
		ProjectId:   "some-project-handle",
		Type:        api.ApplicationType("genai"),
	}, orgID, "")
	if !errors.Is(err, constants.ErrProjectNotFound) {
		t.Fatalf("expected ErrProjectNotFound, got %v", err)
	}
}

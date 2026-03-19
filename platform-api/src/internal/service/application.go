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
	"fmt"
	"log/slog"
	"strings"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	"github.com/google/uuid"
)

type ApplicationService struct {
	appRepo              repository.ApplicationRepository
	projectRepo          repository.ProjectRepository
	orgRepo              repository.OrganizationRepository
	apiRepo              repository.APIRepository
	gatewayEventsService *GatewayEventsService
	slogger              *slog.Logger
}

func NewApplicationService(
	appRepo repository.ApplicationRepository,
	projectRepo repository.ProjectRepository,
	orgRepo repository.OrganizationRepository,
	apiRepo repository.APIRepository,
	gatewayEventsService *GatewayEventsService,
	slogger *slog.Logger,
) *ApplicationService {
	return &ApplicationService{
		appRepo:              appRepo,
		projectRepo:          projectRepo,
		orgRepo:              orgRepo,
		apiRepo:              apiRepo,
		gatewayEventsService: gatewayEventsService,
		slogger:              slogger,
	}
}

func (s *ApplicationService) CreateApplication(req *api.CreateApplicationRequest, orgID string) (*api.Application, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, constants.ErrInvalidApplicationName
	}
	appType, err := normalizeApplicationType(string(req.Type))
	if err != nil {
		return nil, err
	}

	org, err := s.orgRepo.GetOrganizationByUUID(orgID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, constants.ErrOrganizationNotFound
	}

	projectID := strings.TrimSpace(utils.OpenAPIUUIDToString(req.ProjectId))
	if projectID == "" {
		return nil, constants.ErrProjectNotFound
	}

	project, err := s.projectRepo.GetProjectByUUID(projectID)
	if err != nil {
		return nil, err
	}
	if project == nil || project.OrganizationID != orgID {
		return nil, constants.ErrProjectNotFound
	}

	existingByName, err := s.appRepo.GetApplicationByNameInProject(strings.TrimSpace(req.Name), projectID, orgID)
	if err != nil {
		return nil, err
	}
	if existingByName != nil {
		return nil, constants.ErrApplicationExists
	}

	handle := strings.TrimSpace(valueOrEmptyApplication(req.Id))
	if handle == "" {
		handle, err = utils.GenerateHandle(req.Name, s.HandleExistsCheck(orgID))
		if err != nil {
			return nil, err
		}
	} else {
		if err := utils.ValidateHandle(handle); err != nil {
			return nil, err
		}
		exists, err := s.appRepo.CheckApplicationHandleExists(handle, orgID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, constants.ErrHandleExists
		}
	}

	app := &model.Application{
		UUID:             uuid.New().String(),
		Handle:           handle,
		ProjectUUID:      projectID,
		OrganizationUUID: orgID,
		CreatedBy:        strings.TrimSpace(valueOrEmptyApplication(req.CreatedBy)),
		Name:             strings.TrimSpace(req.Name),
		Description:      strings.TrimSpace(valueOrEmptyApplication(req.Description)),
		Type:             appType,
	}

	if err := s.appRepo.CreateApplication(app); err != nil {
		return nil, err
	}

	return s.modelToApplicationResponse(app), nil
}

func (s *ApplicationService) GetApplicationByID(appIDOrHandle, orgID string) (*api.Application, error) {
	app, err := s.appRepo.GetApplicationByIDOrHandle(appIDOrHandle, orgID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, constants.ErrApplicationNotFound
	}

	return s.modelToApplicationResponse(app), nil
}

func (s *ApplicationService) GetApplicationsByOrganization(orgID, projectID string, limit, offset int) (*api.ApplicationListResponse, error) {
	org, err := s.orgRepo.GetOrganizationByUUID(orgID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, constants.ErrOrganizationNotFound
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, constants.ErrProjectNotFound
	}

	project, err := s.projectRepo.GetProjectByUUID(projectID)
	if err != nil {
		return nil, err
	}
	if project == nil || project.OrganizationID != orgID {
		return nil, constants.ErrProjectNotFound
	}

	apps, err := s.appRepo.GetApplicationsByProjectIDPaginated(projectID, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	totalCount, err := s.appRepo.CountApplicationsByProjectID(projectID, orgID)
	if err != nil {
		return nil, err
	}

	response := &api.ApplicationListResponse{
		Count: len(apps),
		List:  make([]api.Application, 0, len(apps)),
		Pagination: api.Pagination{
			Total:  totalCount,
			Offset: offset,
			Limit:  limit,
		},
	}

	for _, app := range apps {
		mapped := s.modelToApplicationResponse(app)
		if mapped != nil {
			response.List = append(response.List, *mapped)
		}
	}

	return response, nil
}

func (s *ApplicationService) UpdateApplication(appIDOrHandle string, req *api.UpdateApplicationRequest, orgID, userID string) (*api.Application, error) {
	app, err := s.appRepo.GetApplicationByIDOrHandle(appIDOrHandle, orgID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, constants.ErrApplicationNotFound
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, constants.ErrInvalidApplicationName
		}
		if name != app.Name {
			existing, err := s.appRepo.GetApplicationByNameInProject(name, app.ProjectUUID, orgID)
			if err != nil {
				return nil, err
			}
			if existing != nil && existing.UUID != app.UUID {
				return nil, constants.ErrApplicationExists
			}
			app.Name = name
		}
	}

	if req.Description != nil {
		app.Description = strings.TrimSpace(*req.Description)
	}

	if req.Type != nil {
		appType, err := normalizeApplicationType(string(*req.Type))
		if err != nil {
			return nil, err
		}
		app.Type = appType
	}

	if err := s.appRepo.UpdateApplication(app); err != nil {
		return nil, err
	}

	broadcastKeys, err := s.listMappedAPIKeysForBroadcast(app.UUID)
	if err != nil {
		return nil, err
	}

	if err := s.broadcastApplicationMappingUpdate(app, userID, broadcastKeys); err != nil && s.slogger != nil {
		s.slogger.Warn("Application update succeeded but failed to broadcast application mapping update event", "applicationId", app.Handle, "error", err)
	}

	return s.modelToApplicationResponse(app), nil
}

func (s *ApplicationService) DeleteApplication(appIDOrHandle, orgID string) error {
	app, err := s.appRepo.GetApplicationByIDOrHandle(appIDOrHandle, orgID)
	if err != nil {
		return err
	}
	if app == nil {
		return constants.ErrApplicationNotFound
	}

	return s.appRepo.DeleteApplication(app.UUID, orgID)
}

func (s *ApplicationService) ListMappedAPIKeys(appIDOrHandle, orgID string, limit, offset int) (*api.MappedAPIKeyListResponse, error) {
	app, err := s.getApplication(appIDOrHandle, orgID)
	if err != nil {
		return nil, err
	}

	keys, err := s.buildMappedAPIKeyListPaginated(app.UUID, limit, offset)
	if err != nil {
		return nil, err
	}

	return keys, nil
}

func (s *ApplicationService) AddMappedAPIKeys(appIDOrHandle string, req *api.AddApplicationAPIKeysRequest, orgID, userID string) (*api.MappedAPIKeyListResponse, error) {
	app, err := s.getApplication(appIDOrHandle, orgID)
	if err != nil {
		return nil, err
	}

	apiKeyIDs, err := s.resolveAPIKeyIDs(req.ApiKeys, orgID, userID)
	if err != nil {
		return nil, err
	}

	if err := s.appRepo.AddApplicationAPIKeys(app.UUID, apiKeyIDs); err != nil {
		return nil, err
	}

	keys, err := s.buildMappedAPIKeyList(app.UUID)
	if err != nil {
		return nil, err
	}
	broadcastKeys, err := s.listMappedAPIKeysForBroadcast(app.UUID)
	if err != nil {
		return nil, err
	}

	if err := s.broadcastApplicationMappingUpdate(app, userID, broadcastKeys); err != nil && s.slogger != nil {
		s.slogger.Warn("Add mapped API keys succeeded but failed to broadcast application mapping update event", "applicationId", app.Handle, "error", err)
	}

	return keys, nil
}

func (s *ApplicationService) RemoveMappedAPIKey(appIDOrHandle, keyID, entityID, orgID, userID string) error {
	app, err := s.getApplication(appIDOrHandle, orgID)
	if err != nil {
		return err
	}

	key, err := s.resolveAPIKey(api.APIKeyMappingSelector{
		KeyId: keyID,
		AssociatedEntity: api.APIKeyMappingAssociatedEntity{
			Id: entityID,
		},
	}, orgID)
	if err != nil {
		return err
	}

	if err := s.appRepo.RemoveApplicationAPIKey(app.UUID, key.ID); err != nil {
		return err
	}

	broadcastKeys, err := s.listMappedAPIKeysForBroadcast(app.UUID)
	if err != nil {
		return err
	}

	if err := s.broadcastApplicationMappingUpdateWithArtifactHints(app, userID, broadcastKeys, []string{key.ArtifactID}); err != nil && s.slogger != nil {
		s.slogger.Warn("Remove mapped API key succeeded but failed to broadcast application mapping update event", "applicationId", app.Handle, "error", err)
	}

	return nil
}

func (s *ApplicationService) HandleExistsCheck(orgID string) func(string) bool {
	return func(handle string) bool {
		exists, err := s.appRepo.CheckApplicationHandleExists(handle, orgID)
		if err != nil {
			return true
		}
		return exists
	}
}

func (s *ApplicationService) getApplication(appIDOrHandle, orgID string) (*model.Application, error) {
	app, err := s.appRepo.GetApplicationByIDOrHandle(appIDOrHandle, orgID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, constants.ErrApplicationNotFound
	}
	return app, nil
}

func (s *ApplicationService) resolveAPIKeyIDs(selectors []api.APIKeyMappingSelector, orgID, userID string) ([]string, error) {
	keys, err := s.resolveAPIKeys(selectors, orgID)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(keys))
	for _, key := range keys {
		if err := s.validateAPIKeyBindingPermission(key, userID); err != nil {
			return nil, err
		}
		result = append(result, key.ID)
	}

	return result, nil
}

func (s *ApplicationService) resolveAPIKeys(selectors []api.APIKeyMappingSelector, orgID string) ([]*model.ApplicationAPIKey, error) {
	seen := make(map[string]struct{})
	result := make([]*model.ApplicationAPIKey, 0, len(selectors))

	for _, selector := range selectors {
		key, err := s.resolveAPIKey(selector, orgID)
		if err != nil {
			return nil, err
		}

		if _, ok := seen[key.ID]; ok {
			continue
		}

		seen[key.ID] = struct{}{}
		result = append(result, key)
	}

	return result, nil
}

func (s *ApplicationService) resolveAPIKey(selector api.APIKeyMappingSelector, orgID string) (*model.ApplicationAPIKey, error) {
	keyID := strings.TrimSpace(selector.KeyId)
	entityID := strings.TrimSpace(selector.AssociatedEntity.Id)

	if keyID == "" || entityID == "" {
		return nil, constants.ErrInvalidAPIKey
	}

	key, err := s.appRepo.GetAPIKeyByNameAndArtifactHandle(keyID, entityID, orgID)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, constants.ErrAPIKeyNotFound
	}

	return key, nil
}

func (s *ApplicationService) validateAPIKeyBindingPermission(key *model.ApplicationAPIKey, userID string) error {
	if key == nil {
		return constants.ErrAPIKeyNotFound
	}

	creator := strings.TrimSpace(key.CreatedBy)
	requester := strings.TrimSpace(userID)

	if creator == "" || requester == "" {
		return nil
	}

	if creator != requester {
		return constants.ErrAPIKeyForbidden
	}

	return nil
}

func (s *ApplicationService) buildMappedAPIKeyList(applicationUUID string) (*api.MappedAPIKeyListResponse, error) {
	return s.buildMappedAPIKeyListPaginated(applicationUUID, -1, 0)
}

func (s *ApplicationService) buildMappedAPIKeyListPaginated(applicationUUID string, limit, offset int) (*api.MappedAPIKeyListResponse, error) {
	keys, err := s.appRepo.ListMappedAPIKeys(applicationUUID)
	if err != nil {
		return nil, err
	}

	if offset < 0 {
		offset = 0
	}

	total := len(keys)
	if offset > total {
		offset = total
	}

	pagedKeys := keys
	effectiveLimit := len(keys)
	if limit > 0 {
		effectiveLimit = limit
		end := offset + limit
		if end > total {
			end = total
		}
		pagedKeys = keys[offset:end]
	}

	response := &api.MappedAPIKeyListResponse{
		Count: len(pagedKeys),
		List:  make([]api.MappedAPIKey, 0, len(pagedKeys)),
		Pagination: api.Pagination{
			Total:  total,
			Offset: offset,
			Limit:  effectiveLimit,
		},
	}

	for _, key := range pagedKeys {
		response.List = append(response.List, s.modelToMappedAPIKeyResponse(key))
	}

	return response, nil
}

func (s *ApplicationService) modelToApplicationResponse(app *model.Application) *api.Application {
	if app == nil {
		return nil
	}
	projectID := utils.ParseOpenAPIUUIDOrZero(app.ProjectUUID)

	return &api.Application{
		Id:          app.Handle,
		Name:        app.Name,
		ProjectId:   projectID,
		Type:        api.ApplicationType(app.Type),
		Description: utils.StringPtrIfNotEmpty(app.Description),
		CreatedBy:   utils.StringPtrIfNotEmpty(app.CreatedBy),
		CreatedAt:   utils.TimePtrIfNotZero(app.CreatedAt),
		UpdatedAt:   utils.TimePtrIfNotZero(app.UpdatedAt),
	}
}

func (s *ApplicationService) modelToMappedAPIKeyResponse(key *model.ApplicationAPIKey) api.MappedAPIKey {
	if key == nil {
		return api.MappedAPIKey{}
	}

	return api.MappedAPIKey{
		KeyId: key.Name,
		AssociatedEntity: api.AssociatedEntity{
			Id:   key.ArtifactHandle,
			Kind: key.ArtifactKind,
		},
		Status:    utils.StringPtrIfNotEmpty(key.Status),
		UserId:    utils.StringPtrIfNotEmpty(key.CreatedBy),
		CreatedAt: utils.TimePtrIfNotZero(key.CreatedAt),
		UpdatedAt: utils.TimePtrIfNotZero(key.UpdatedAt),
		ExpiresAt: key.ExpiresAt,
	}
}

func (s *ApplicationService) listMappedAPIKeysForBroadcast(applicationUUID string) ([]*model.ApplicationAPIKey, error) {
	return s.appRepo.ListMappedAPIKeys(applicationUUID)
}

func valueOrEmptyApplication(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeApplicationType(appType string) (string, error) {
	trimmed := strings.TrimSpace(appType)
	if trimmed == "" {
		return "", constants.ErrInvalidApplicationType
	}
	if strings.EqualFold(trimmed, "genai") {
		return "genai", nil
	}
	return "", constants.ErrUnsupportedApplicationType
}

func (s *ApplicationService) broadcastApplicationMappingUpdate(app *model.Application, userID string, keys []*model.ApplicationAPIKey) error {
	return s.broadcastApplicationMappingUpdateWithArtifactHints(app, userID, keys, nil)
}

func (s *ApplicationService) broadcastApplicationMappingUpdateWithArtifactHints(app *model.Application, userID string, keys []*model.ApplicationAPIKey, artifactHints []string) error {
	if s.appRepo == nil || s.gatewayEventsService == nil {
		return nil
	}

	entryByKey := make(map[string]model.ApplicationKeyMapping)
	affectedArtifactIDs := make(map[string]struct{})
	gatewayIDs := make(map[string]struct{})

	for _, key := range keys {
		if key == nil {
			continue
		}

		if key.APIKeyUUID == "" {
			return fmt.Errorf("mapped API key is missing UUID for artifact id %s", key.ArtifactID)
		}
		if strings.TrimSpace(key.ArtifactID) == "" {
			continue
		}

		entry := model.ApplicationKeyMapping{ApiKeyUuid: key.APIKeyUUID}
		entryByKey[entry.ApiKeyUuid] = entry
		affectedArtifactIDs[key.ArtifactID] = struct{}{}
	}

	for _, artifactID := range artifactHints {
		trimmed := strings.TrimSpace(artifactID)
		if trimmed == "" {
			continue
		}
		affectedArtifactIDs[trimmed] = struct{}{}
	}

	for artifactID := range affectedArtifactIDs {
		artifact, err := s.appRepo.GetArtifactByUUID(artifactID, app.OrganizationUUID)
		if err != nil {
			return fmt.Errorf("failed to resolve mapped artifact by artifact id %s: %w", artifactID, err)
		}
		if artifact == nil {
			continue
		}

		switch artifact.Kind {
		case constants.LLMProvider, constants.LLMProxy:
			// Supported artifact kinds for gateway association lookups.
		default:
			if s.slogger != nil {
				s.slogger.Warn("Skipping unsupported artifact kind for application mapping broadcast",
					"applicationId", app.Handle,
					"artifactId", artifactID,
					"artifactKind", artifact.Kind,
				)
			}
			continue
		}

		gatewayIDsForArtifact, err := s.appRepo.GetDeployedGatewayIDsByArtifactUUID(artifact.UUID, app.OrganizationUUID)
		if err != nil {
			return fmt.Errorf("failed to resolve deployed gateways for artifact %s (%s): %w", artifact.Handle, artifact.Kind, err)
		}
		for _, gatewayID := range gatewayIDsForArtifact {
			gatewayIDs[gatewayID] = struct{}{}
		}
	}

	mappings := make([]model.ApplicationKeyMapping, 0, len(entryByKey))
	for _, mapping := range entryByKey {
		mappings = append(mappings, mapping)
	}

	event := &model.ApplicationUpdatedEvent{
		ApplicationId:   app.Handle,
		ApplicationUuid: app.UUID,
		ApplicationName: app.Name,
		ApplicationType: app.Type,
		Mappings:        mappings,
	}

	if len(gatewayIDs) == 0 {
		if s.slogger != nil {
			s.slogger.Debug("No target gateways found for application mapping broadcast", "applicationId", app.Handle)
		}
		return nil
	}

	successCount := 0
	failureCount := 0
	var lastError error

	for gatewayID := range gatewayIDs {
		err := s.gatewayEventsService.BroadcastApplicationUpdatedEvent(gatewayID, userID, event)
		if err != nil {
			failureCount++
			lastError = err
			if s.slogger != nil {
				s.slogger.Error("Failed to broadcast application mapping update event", "applicationId", app.Handle, "gatewayId", gatewayID, "error", err)
			}
			continue
		}
		successCount++
	}

	if successCount == 0 {
		return fmt.Errorf("failed to deliver application mapping update event to any gateway: %w", lastError)
	}
	if failureCount > 0 && s.slogger != nil {
		s.slogger.Warn("Partial delivery of application mapping update event", "applicationId", app.Handle, "success", successCount, "failed", failureCount)
	}

	return nil
}

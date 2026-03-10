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
	"strings"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	"github.com/google/uuid"
)

type ApplicationService struct {
	appRepo     repository.ApplicationRepository
	projectRepo repository.ProjectRepository
	orgRepo     repository.OrganizationRepository
}

func NewApplicationService(
	appRepo repository.ApplicationRepository,
	projectRepo repository.ProjectRepository,
	orgRepo repository.OrganizationRepository,
) *ApplicationService {
	return &ApplicationService{
		appRepo:     appRepo,
		projectRepo: projectRepo,
		orgRepo:     orgRepo,
	}
}

func (s *ApplicationService) CreateApplication(req *dto.CreateApplicationRequest, orgID string) (*dto.ApplicationResponse, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, constants.ErrInvalidApplicationName
	}
	appType, err := normalizeApplicationType(req.Type)
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

	project, err := s.projectRepo.GetProjectByUUID(req.ProjectId)
	if err != nil {
		return nil, err
	}
	if project == nil || project.OrganizationID != orgID {
		return nil, constants.ErrProjectNotFound
	}

	existingByName, err := s.appRepo.GetApplicationByNameInProject(strings.TrimSpace(req.Name), req.ProjectId, orgID)
	if err != nil {
		return nil, err
	}
	if existingByName != nil {
		return nil, constants.ErrApplicationExists
	}

	handle := strings.TrimSpace(req.Id)
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
		ProjectUUID:      req.ProjectId,
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

func (s *ApplicationService) GetApplicationByID(appIDOrHandle, orgID string) (*dto.ApplicationResponse, error) {
	app, err := s.appRepo.GetApplicationByIDOrHandle(appIDOrHandle, orgID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, constants.ErrApplicationNotFound
	}

	return s.modelToApplicationResponse(app), nil
}

func (s *ApplicationService) GetApplicationsByOrganization(orgID, projectID string) (*dto.ApplicationListResponse, error) {
	org, err := s.orgRepo.GetOrganizationByUUID(orgID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, constants.ErrOrganizationNotFound
	}

	var apps []*model.Application
	if projectID != "" {
		project, err := s.projectRepo.GetProjectByUUID(projectID)
		if err != nil {
			return nil, err
		}
		if project == nil || project.OrganizationID != orgID {
			return nil, constants.ErrProjectNotFound
		}

		apps, err = s.appRepo.GetApplicationsByProjectID(projectID, orgID)
		if err != nil {
			return nil, err
		}
	} else {
		apps, err = s.appRepo.GetApplicationsByOrganizationID(orgID)
		if err != nil {
			return nil, err
		}
	}

	response := &dto.ApplicationListResponse{
		Count: len(apps),
		List:  make([]*dto.ApplicationResponse, 0, len(apps)),
		Pagination: dto.Pagination{
			Total:  len(apps),
			Offset: 0,
			Limit:  len(apps),
		},
	}

	for _, app := range apps {
		response.List = append(response.List, s.modelToApplicationResponse(app))
	}

	return response, nil
}

func (s *ApplicationService) UpdateApplication(appIDOrHandle string, req *dto.UpdateApplicationRequest, orgID string) (*dto.ApplicationResponse, error) {
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
		appType, err := normalizeApplicationType(*req.Type)
		if err != nil {
			return nil, err
		}
		app.Type = appType
	}

	if err := s.appRepo.UpdateApplication(app); err != nil {
		return nil, err
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

	return s.appRepo.DeleteApplication(app.UUID)
}

func (s *ApplicationService) ListMappedAPIKeys(appIDOrHandle, orgID string) (*dto.MappedAPIKeyListResponse, error) {
	app, err := s.getApplication(appIDOrHandle, orgID)
	if err != nil {
		return nil, err
	}
	return s.buildMappedAPIKeyList(app.UUID)
}

func (s *ApplicationService) ReplaceMappedAPIKeys(appIDOrHandle string, req *dto.ReplaceApplicationAPIKeysRequest, orgID string) (*dto.MappedAPIKeyListResponse, error) {
	app, err := s.getApplication(appIDOrHandle, orgID)
	if err != nil {
		return nil, err
	}

	apiKeyIDs, err := s.resolveAPIKeyIDs(req.ApiKeyIds, orgID)
	if err != nil {
		return nil, err
	}

	if err := s.appRepo.ReplaceApplicationAPIKeys(app.UUID, apiKeyIDs); err != nil {
		return nil, err
	}

	return s.buildMappedAPIKeyList(app.UUID)
}

func (s *ApplicationService) AddMappedAPIKeys(appIDOrHandle string, req *dto.AddApplicationAPIKeysRequest, orgID string) (*dto.MappedAPIKeyListResponse, error) {
	app, err := s.getApplication(appIDOrHandle, orgID)
	if err != nil {
		return nil, err
	}

	apiKeyIDs, err := s.resolveAPIKeyIDs(req.ApiKeyIds, orgID)
	if err != nil {
		return nil, err
	}

	if err := s.appRepo.AddApplicationAPIKeys(app.UUID, apiKeyIDs); err != nil {
		return nil, err
	}

	return s.buildMappedAPIKeyList(app.UUID)
}

func (s *ApplicationService) RemoveMappedAPIKey(appIDOrHandle, keyID, orgID string) error {
	app, err := s.getApplication(appIDOrHandle, orgID)
	if err != nil {
		return err
	}

	key, err := s.appRepo.GetAPIKeyByID(keyID, orgID)
	if err != nil {
		return err
	}
	if key == nil {
		return constants.ErrAPIKeyNotFound
	}

	return s.appRepo.RemoveApplicationAPIKey(app.UUID, key.ID)
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

func (s *ApplicationService) resolveAPIKeyIDs(ids []string, orgID string) ([]string, error) {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(ids))

	for _, id := range ids {
		keyID := strings.TrimSpace(id)
		if keyID == "" {
			return nil, constants.ErrInvalidAPIKey
		}
		if _, ok := seen[keyID]; ok {
			continue
		}

		key, err := s.appRepo.GetAPIKeyByID(keyID, orgID)
		if err != nil {
			return nil, err
		}
		if key == nil {
			return nil, constants.ErrAPIKeyNotFound
		}

		seen[keyID] = struct{}{}
		result = append(result, keyID)
	}

	return result, nil
}

func (s *ApplicationService) buildMappedAPIKeyList(applicationUUID string) (*dto.MappedAPIKeyListResponse, error) {
	keys, err := s.appRepo.ListMappedAPIKeys(applicationUUID)
	if err != nil {
		return nil, err
	}

	response := &dto.MappedAPIKeyListResponse{
		Count: len(keys),
		List:  make([]*dto.MappedAPIKeyResponse, 0, len(keys)),
		Pagination: dto.Pagination{
			Total:  len(keys),
			Offset: 0,
			Limit:  len(keys),
		},
	}

	for _, key := range keys {
		response.List = append(response.List, s.modelToMappedAPIKeyResponse(key))
	}

	return response, nil
}

func (s *ApplicationService) modelToApplicationResponse(app *model.Application) *dto.ApplicationResponse {
	if app == nil {
		return nil
	}

	return &dto.ApplicationResponse{
		Id:          app.Handle,
		Name:        app.Name,
		ProjectId:   app.ProjectUUID,
		Type:        app.Type,
		Description: utils.StringPtrIfNotEmpty(app.Description),
		CreatedBy:   utils.StringPtrIfNotEmpty(app.CreatedBy),
		CreatedAt:   utils.TimePtrIfNotZero(app.CreatedAt),
		UpdatedAt:   utils.TimePtrIfNotZero(app.UpdatedAt),
	}
}

func (s *ApplicationService) modelToMappedAPIKeyResponse(key *model.ApplicationAPIKey) *dto.MappedAPIKeyResponse {
	if key == nil {
		return nil
	}

	return &dto.MappedAPIKeyResponse{
		Id:         key.ID,
		Name:       key.Name,
		ArtifactId: key.ArtifactID,
		Status:     utils.StringPtrIfNotEmpty(key.Status),
		CreatedBy:  utils.StringPtrIfNotEmpty(key.CreatedBy),
		CreatedAt:  utils.TimePtrIfNotZero(key.CreatedAt),
		UpdatedAt:  utils.TimePtrIfNotZero(key.UpdatedAt),
		ExpiresAt:  key.ExpiresAt,
	}
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

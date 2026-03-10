/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"platform-api/src/api"
	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
	"regexp"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

type OrganizationService struct {
	orgRepo           repository.OrganizationRepository
	projectRepo       repository.ProjectRepository
	devPortalService  *DevPortalService
	llmTemplateSeeder *LLMTemplateSeeder
	config            *config.Server
	slogger           *slog.Logger
}

func NewOrganizationService(orgRepo repository.OrganizationRepository,
	projectRepo repository.ProjectRepository, devPortalService *DevPortalService, llmTemplateSeeder *LLMTemplateSeeder, cfg *config.Server, slogger *slog.Logger) *OrganizationService {
	return &OrganizationService{
		orgRepo:           orgRepo,
		projectRepo:       projectRepo,
		devPortalService:  devPortalService,
		llmTemplateSeeder: llmTemplateSeeder,
		config:            cfg,
		slogger:           slogger,
	}
}

func (s *OrganizationService) RegisterOrganization(id string, handle string, name string, region string) (*api.Organization, error) {
	// Validate handle is URL friendly
	if !s.isURLFriendly(handle) {
		return nil, constants.ErrInvalidHandle
	}

	// Check if id or handle already exists
	existingOrg, err := s.orgRepo.GetOrganizationByIdOrHandle(id, handle)
	if err != nil {
		return nil, err
	}
	if existingOrg != nil {
		if existingOrg.ID == id {
			return nil, constants.ErrOrganizationExists
		}
		return nil, constants.ErrHandleExists
	}

	if name == "" {
		name = handle // Default name to handle if not provided
	}

	// Generate default project ID upfront before persisting any data
	projectID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate default project ID: %w", err)
	}

	// Create organization in platform-api database first
	org := &api.Organization{
		Id:        &openapi_types.UUID{},
		Handle:    handle,
		Name:      name,
		Region:    region,
		CreatedAt: utils.TimePtrIfNotZero(time.Now()),
	}

	orgModel := s.apiToModel(org, id)
	err = s.orgRepo.CreateOrganization(orgModel)
	if err != nil {
		return nil, err
	}

	// Seed default LLM provider templates for the new organization (best-effort)
	if s.llmTemplateSeeder != nil {
		if seedErr := s.llmTemplateSeeder.SeedForOrg(id); seedErr != nil {
			s.slogger.Warn("Failed to seed default LLM templates for organization", "organization", name, "error", seedErr)
		}
	}

	// Create default DevPortal if enabled
	if s.devPortalService != nil && s.config != nil && s.config.DefaultDevPortal.Enabled {
		defaultDevPortal, devPortalErr := s.devPortalService.CreateDefaultDevPortal(id)
		if devPortalErr != nil {
			s.slogger.Warn("Failed to create default DevPortal for organization", "organization", name, "error", devPortalErr)
			// Don't fail organization creation, but log the error
		} else if defaultDevPortal != nil {
			s.slogger.Info("Created default DevPortal for organization", "devPortal", defaultDevPortal.Name, "organization", name)
		}
		// No organization sync during creation - sync happens during DevPortal activation
	}

	// Create default project for the organization
	defaultProject := &model.Project{
		ID:             projectID,
		Name:           "default",
		OrganizationID: id,
		Description:    "Default project",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err = s.projectRepo.CreateProject(defaultProject)
	orgResponse, convErr := s.modelToAPI(orgModel)
	if convErr != nil {
		return nil, convErr
	}
	if err != nil {
		// If project creation fails, return the organization anyway
		// (we don't rollback organization creation)
		return orgResponse, err
	}

	return orgResponse, nil
}

func (s *OrganizationService) GetOrganizationByUUID(orgId string) (*api.Organization, error) {
	orgModel, err := s.orgRepo.GetOrganizationByUUID(orgId)
	if err != nil {
		return nil, err
	}

	if orgModel == nil {
		return nil, constants.ErrOrganizationNotFound
	}

	org, convErr := s.modelToAPI(orgModel)
	if convErr != nil {
		return nil, convErr
	}

	return org, nil
}

func (s *OrganizationService) isURLFriendly(handle string) bool {
	// URL friendly: lowercase letters, numbers, hyphens, underscores
	// Must start with letter, no consecutive special chars
	pattern := `^[a-z][a-z0-9_-]*[a-z0-9]$|^[a-z]$`
	matched, _ := regexp.MatchString(pattern, strings.ToLower(handle))
	return matched && handle == strings.ToLower(handle)
}

// Mapping functions
func (s *OrganizationService) apiToModel(org *api.Organization, id string) *model.Organization {
	if org == nil {
		return nil
	}

	createdAt := time.Now()
	if org.CreatedAt != nil {
		createdAt = *org.CreatedAt
	}

	return &model.Organization{
		ID:        id,
		Handle:    org.Handle,
		Name:      org.Name,
		Region:    org.Region,
		CreatedAt: createdAt,
		UpdatedAt: time.Now(),
	}
}

func (s *OrganizationService) modelToAPI(orgModel *model.Organization) (*api.Organization, error) {
	if orgModel == nil {
		return nil, nil
	}

	orgID, err := utils.ParseOpenAPIUUID(orgModel.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse organization ID as UUID: %w", err)
	}

	return &api.Organization{
		Id:        orgID,
		Handle:    orgModel.Handle,
		Name:      orgModel.Name,
		Region:    orgModel.Region,
		CreatedAt: utils.TimePtrIfNotZero(orgModel.CreatedAt),
		UpdatedAt: utils.TimePtrIfNotZero(orgModel.UpdatedAt),
	}, nil
}

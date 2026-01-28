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
	"log"
	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type OrganizationService struct {
	orgRepo           repository.OrganizationRepository
	projectRepo       repository.ProjectRepository
	devPortalService  *DevPortalService
	llmTemplateSeeder *LLMTemplateSeeder
	config            *config.Server
}

func NewOrganizationService(orgRepo repository.OrganizationRepository,
	projectRepo repository.ProjectRepository, devPortalService *DevPortalService, llmTemplateSeeder *LLMTemplateSeeder, cfg *config.Server) *OrganizationService {
	return &OrganizationService{
		orgRepo:           orgRepo,
		projectRepo:       projectRepo,
		devPortalService:  devPortalService,
		llmTemplateSeeder: llmTemplateSeeder,
		config:            cfg,
	}
}

func (s *OrganizationService) RegisterOrganization(id string, handle string, name string, region string) (*dto.Organization, error) {
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

	// Create organization in platform-api database first
	org := &dto.Organization{
		ID:        id,
		Handle:    handle,
		Name:      name,
		Region:    region,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	orgModel := s.dtoToModel(org)
	err = s.orgRepo.CreateOrganization(orgModel)
	if err != nil {
		return nil, err
	}

	// Seed default LLM provider templates for the new organization (best-effort)
	if s.llmTemplateSeeder != nil {
		if seedErr := s.llmTemplateSeeder.SeedForOrg(id); seedErr != nil {
			log.Printf("[OrganizationService] Failed to seed default LLM templates for organization %s: %v", name, seedErr)
		}
	}

	// Create default DevPortal if enabled
	if s.devPortalService != nil && s.config != nil && s.config.DefaultDevPortal.Enabled {
		defaultDevPortal, devPortalErr := s.devPortalService.CreateDefaultDevPortal(id)
		if devPortalErr != nil {
			log.Printf("[OrganizationService] Failed to create default DevPortal for organization %s: %v", name, devPortalErr)
			// Don't fail organization creation, but log the error
		} else if defaultDevPortal != nil {
			log.Printf("[OrganizationService] Created default DevPortal %s for organization %s",
				defaultDevPortal.Name, name)
		}
		// No organization sync during creation - sync happens during DevPortal activation
	}

	// Create default project for the organization
	defaultProject := &model.Project{
		ID:             uuid.New().String(),
		Name:           "default",
		OrganizationID: org.ID,
		Description:    "Default project",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err = s.projectRepo.CreateProject(defaultProject)
	if err != nil {
		// If project creation fails, roll back the organization creation
		return org, err
	}

	return org, nil
}

func (s *OrganizationService) GetOrganizationByUUID(orgId string) (*dto.Organization, error) {
	orgModel, err := s.orgRepo.GetOrganizationByUUID(orgId)
	if err != nil {
		return nil, err
	}

	if orgModel == nil {
		return nil, constants.ErrOrganizationNotFound
	}

	org := s.modelToDTO(orgModel)
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
func (s *OrganizationService) dtoToModel(dto *dto.Organization) *model.Organization {
	if dto == nil {
		return nil
	}

	return &model.Organization{
		ID:        dto.ID,
		Handle:    dto.Handle,
		Name:      dto.Name,
		Region:    dto.Region,
		CreatedAt: dto.CreatedAt,
		UpdatedAt: dto.UpdatedAt,
	}
}

func (s *OrganizationService) modelToDTO(model *model.Organization) *dto.Organization {
	if model == nil {
		return nil
	}

	return &dto.Organization{
		ID:        model.ID,
		Handle:    model.Handle,
		Name:      model.Name,
		Region:    model.Region,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}

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
	"time"
)

type OrganizationService struct {
	orgRepo           repository.OrganizationRepository
	projectRepo       repository.ProjectRepository
	applicationRepo   repository.ApplicationRepository
	apiRepo           repository.APIRepository
	gatewayRepo       repository.GatewayRepository
	llmProviderRepo   repository.LLMProviderRepository
	llmProxyRepo      repository.LLMProxyRepository
	mcpProxyRepo      repository.MCPProxyRepository
	llmTemplateSeeder *LLMTemplateSeeder
	auditRepo         repository.AuditRepository
	config            *config.Server
	slogger           *slog.Logger
}

func NewOrganizationService(orgRepo repository.OrganizationRepository,
	projectRepo repository.ProjectRepository,
	applicationRepo repository.ApplicationRepository,
	apiRepo repository.APIRepository,
	gatewayRepo repository.GatewayRepository,
	llmProviderRepo repository.LLMProviderRepository,
	llmProxyRepo repository.LLMProxyRepository,
	mcpProxyRepo repository.MCPProxyRepository,
	llmTemplateSeeder *LLMTemplateSeeder,
	auditRepo repository.AuditRepository,
	cfg *config.Server,
	slogger *slog.Logger,
) *OrganizationService {
	return &OrganizationService{
		orgRepo:           orgRepo,
		projectRepo:       projectRepo,
		applicationRepo:   applicationRepo,
		apiRepo:           apiRepo,
		gatewayRepo:       gatewayRepo,
		llmProviderRepo:   llmProviderRepo,
		llmProxyRepo:      llmProxyRepo,
		mcpProxyRepo:      mcpProxyRepo,
		llmTemplateSeeder: llmTemplateSeeder,
		auditRepo:         auditRepo,
		config:            cfg,
		slogger:           slogger,
	}
}

func (s *OrganizationService) RegisterOrganization(id string, handle string, name string, region string, performedBy string) (*api.Organization, error) {
	// Auto-generate handle from name if not provided; otherwise validate the explicit handle.
	if handle == "" {
		generated, genErr := utils.GenerateHandle(name, func(h string) bool {
			existing, _ := s.orgRepo.GetOrganizationByIdOrHandle("", h)
			return existing != nil
		})
		if genErr != nil {
			return nil, fmt.Errorf("failed to generate organization handle: %w", genErr)
		}
		handle = generated
	} else {
		if err := utils.ValidateHandle(handle); err != nil {
			return nil, err
		}
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
		Id:          &handle,
		DisplayName: name,
		Region:      region,
		CreatedAt:   utils.TimePtrIfNotZero(time.Now()),
	}

	orgModel := s.apiToModel(org, id)
	err = s.orgRepo.CreateOrganization(orgModel)
	if err != nil {
		return nil, err
	}
	_ = s.auditRepo.Record("CREATE", orgModel.ID, "organization", orgModel.ID, performedBy)

	// Seed default LLM provider templates for the new organization (best-effort)
	if s.llmTemplateSeeder != nil {
		if seedErr := s.llmTemplateSeeder.SeedForOrg(id); seedErr != nil {
			s.slogger.Warn("Failed to seed default LLM templates for organization", "organization", name, "error", seedErr)
		}
	}

	// Create default project for the organization
	defaultProject := &model.Project{
		ID:             projectID,
		Handle:         "default",
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

func containsStr(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
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

// ListOrganizations returns a paginated list of organizations along with the
// total number of organizations available across all pages.
func (s *OrganizationService) ListOrganizations(limit, offset int) ([]api.Organization, int, error) {
	total, err := s.orgRepo.CountOrganizations()
	if err != nil {
		return nil, 0, err
	}

	orgModels, err := s.orgRepo.ListOrganizations(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	orgs := make([]api.Organization, 0, len(orgModels))
	for _, orgModel := range orgModels {
		org, convErr := s.modelToAPI(orgModel)
		if convErr != nil {
			return nil, 0, convErr
		}
		orgs = append(orgs, *org)
	}

	return orgs, total, nil
}

func (s *OrganizationService) GetOrganizationByHandle(handle string) (*api.Organization, error) {
	orgModel, err := s.orgRepo.GetOrganizationByHandle(handle)
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

// Mapping functions
func (s *OrganizationService) apiToModel(org *api.Organization, id string) *model.Organization {
	if org == nil {
		return nil
	}

	createdAt := time.Now()
	if org.CreatedAt != nil {
		createdAt = *org.CreatedAt
	}

	handle := ""
	if org.Id != nil {
		handle = *org.Id
	}
	return &model.Organization{
		ID:        id,
		Handle:    handle,
		Name:      org.DisplayName,
		Region:    org.Region,
		CreatedAt: createdAt,
		UpdatedAt: time.Now(),
	}
}

func (s *OrganizationService) modelToAPI(orgModel *model.Organization) (*api.Organization, error) {
	if orgModel == nil {
		return nil, nil
	}

	return &api.Organization{
		Id:          &orgModel.Handle,
		DisplayName: orgModel.Name,
		Region:      orgModel.Region,
		CreatedAt:   utils.TimePtrIfNotZero(orgModel.CreatedAt),
		UpdatedAt:   utils.TimePtrIfNotZero(orgModel.UpdatedAt),
	}, nil
}

func intPtr(value int) *int {
	return &value
}

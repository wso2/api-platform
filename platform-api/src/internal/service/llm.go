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
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

const (
	llmStatusPending  = "pending"
	llmStatusDeployed = "deployed"
	llmStatusFailed   = "failed"
)

type LLMProviderTemplateService struct {
	repo repository.LLMProviderTemplateRepository
}

type LLMProviderService struct {
	repo           repository.LLMProviderRepository
	templateRepo   repository.LLMProviderTemplateRepository
	orgRepo        repository.OrganizationRepository
	templateSeeder *LLMTemplateSeeder
}

type LLMProxyService struct {
	repo         repository.LLMProxyRepository
	providerRepo repository.LLMProviderRepository
	projectRepo  repository.ProjectRepository
}

func NewLLMProviderTemplateService(repo repository.LLMProviderTemplateRepository) *LLMProviderTemplateService {
	return &LLMProviderTemplateService{repo: repo}
}

func NewLLMProviderService(repo repository.LLMProviderRepository, templateRepo repository.LLMProviderTemplateRepository, orgRepo repository.OrganizationRepository, templateSeeder *LLMTemplateSeeder) *LLMProviderService {
	return &LLMProviderService{repo: repo, templateRepo: templateRepo, orgRepo: orgRepo, templateSeeder: templateSeeder}
}

func NewLLMProxyService(repo repository.LLMProxyRepository, providerRepo repository.LLMProviderRepository, projectRepo repository.ProjectRepository) *LLMProxyService {
	return &LLMProxyService{repo: repo, providerRepo: providerRepo, projectRepo: projectRepo}
}

func (s *LLMProviderTemplateService) Create(orgUUID, createdBy string, req *api.LLMProviderTemplate) (*api.LLMProviderTemplate, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id == "" || req.Name == "" {
		return nil, constants.ErrInvalidInput
	}

	exists, err := s.repo.Exists(req.Id, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check template exists: %w", err)
	}
	if exists {
		return nil, constants.ErrLLMProviderTemplateExists
	}

	m := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               req.Id,
		Name:             req.Name,
		Description:      valueOrEmpty(req.Description),
		CreatedBy:        createdBy,
		Metadata:         mapTemplateMetadataAPI(req.Metadata),
		PromptTokens:     mapExtractionIdentifierAPI(req.PromptTokens),
		CompletionTokens: mapExtractionIdentifierAPI(req.CompletionTokens),
		TotalTokens:      mapExtractionIdentifierAPI(req.TotalTokens),
		RemainingTokens:  mapExtractionIdentifierAPI(req.RemainingTokens),
		RequestModel:     mapExtractionIdentifierAPI(req.RequestModel),
		ResponseModel:    mapExtractionIdentifierAPI(req.ResponseModel),
	}
	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrLLMProviderTemplateExists
		}
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	return mapTemplateModelToAPI(m), nil
}

func (s *LLMProviderTemplateService) List(orgUUID string, limit, offset int) (*api.LLMProviderTemplateListResponse, error) {
	items, err := s.repo.List(orgUUID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	totalCount, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count templates: %w", err)
	}
	resp := &api.LLMProviderTemplateListResponse{
		Count: len(items),
		Pagination: api.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}
	resp.List = make([]api.LLMProviderTemplateListItem, 0, len(items))
	for _, t := range items {
		id := t.ID
		name := t.Name
		desc := stringPtrIfNotEmpty(t.Description)
		createdBy := stringPtrIfNotEmpty(t.CreatedBy)
		resp.List = append(resp.List, api.LLMProviderTemplateListItem{
			Id:          &id,
			Name:        &name,
			Description: desc,
			CreatedBy:   createdBy,
			CreatedAt:   timePtr(t.CreatedAt),
			UpdatedAt:   timePtr(t.UpdatedAt),
		})
	}
	return resp, nil
}

func (s *LLMProviderTemplateService) Get(orgUUID, handle string) (*api.LLMProviderTemplate, error) {
	if handle == "" {
		return nil, constants.ErrInvalidInput
	}
	m, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	if m == nil {
		return nil, constants.ErrLLMProviderTemplateNotFound
	}
	return mapTemplateModelToAPI(m), nil
}

func (s *LLMProviderTemplateService) Update(orgUUID, handle string, req *api.LLMProviderTemplate) (*api.LLMProviderTemplate, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id != "" && req.Id != handle {
		return nil, constants.ErrInvalidInput
	}
	if req.Name == "" {
		return nil, constants.ErrInvalidInput
	}

	m := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               handle,
		Name:             req.Name,
		Description:      valueOrEmpty(req.Description),
		Metadata:         mapTemplateMetadataAPI(req.Metadata),
		PromptTokens:     mapExtractionIdentifierAPI(req.PromptTokens),
		CompletionTokens: mapExtractionIdentifierAPI(req.CompletionTokens),
		TotalTokens:      mapExtractionIdentifierAPI(req.TotalTokens),
		RemainingTokens:  mapExtractionIdentifierAPI(req.RemainingTokens),
		RequestModel:     mapExtractionIdentifierAPI(req.RequestModel),
		ResponseModel:    mapExtractionIdentifierAPI(req.ResponseModel),
	}

	if err := s.repo.Update(m); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrLLMProviderTemplateNotFound
		}
		return nil, fmt.Errorf("failed to update template: %w", err)
	}

	updated, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated template: %w", err)
	}
	if updated == nil {
		return nil, constants.ErrLLMProviderTemplateNotFound
	}
	return mapTemplateModelToAPI(updated), nil
}

func (s *LLMProviderTemplateService) Delete(orgUUID, handle string) error {
	if handle == "" {
		return constants.ErrInvalidInput
	}
	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrLLMProviderTemplateNotFound
		}
		return fmt.Errorf("failed to delete template: %w", err)
	}
	return nil
}

func (s *LLMProviderService) Create(orgUUID, createdBy string, req *api.LLMProvider) (*api.LLMProvider, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id == "" || req.Name == "" || req.Version == "" || req.Template == "" {
		return nil, constants.ErrInvalidInput
	}
	if err := validateModelProviders(req.Template, req.ModelProviders); err != nil {
		return nil, err
	}
	if s.orgRepo != nil {
		org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate organization: %w", err)
		}
		if org == nil {
			return nil, constants.ErrOrganizationNotFound
		}
	}

	if err := validateUpstream(req.Upstream); err != nil {
		return nil, err
	}
	if err := validateRateLimitingConfig(req.RateLimiting); err != nil {
		return nil, err
	}

	// Ensure template exists
	tpl, err := s.templateRepo.GetByID(req.Template, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate template: %w", err)
	}
	if tpl == nil && s.templateSeeder != nil {
		// Try to seed defaults for this org and re-fetch
		if seedErr := s.templateSeeder.SeedForOrg(orgUUID); seedErr != nil {
			return nil, fmt.Errorf("failed to seed default templates: %w", seedErr)
		}
		tpl, err = s.templateRepo.GetByID(req.Template, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate template after seeding: %w", err)
		}
	}
	if tpl == nil {
		return nil, constants.ErrLLMProviderTemplateNotFound
	}

	exists, err := s.repo.Exists(req.Id, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check provider exists: %w", err)
	}
	if exists {
		return nil, constants.ErrLLMProviderExists
	}

	providerCount, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count providers: %w", err)
	}
	if err := validateLLMResourceLimit(providerCount, constants.MaxLLMProvidersPerOrganization, constants.ErrLLMProviderLimitReached); err != nil {
		return nil, err
	}

	contextValue := defaultStringPtr(req.Context, "/")
	m := &model.LLMProvider{
		OrganizationUUID: orgUUID,
		ID:               req.Id,
		Name:             req.Name,
		Description:      valueOrEmpty(req.Description),
		CreatedBy:        createdBy,
		Version:          req.Version,
		TemplateUUID:     tpl.UUID,
		OpenAPISpec:      valueOrEmpty(req.Openapi),
		ModelProviders:   mapModelProvidersAPI(req.ModelProviders),
		Status:           llmStatusPending,
		Configuration: model.LLMProviderConfig{
			Context:       &contextValue,
			VHost:         req.Vhost,
			Upstream:      mapUpstreamAPIToModel(req.Upstream),
			AccessControl: mapAccessControlAPI(&req.AccessControl),
			RateLimiting:  mapRateLimitingAPIToModel(req.RateLimiting),
			Policies:      mapPoliciesAPIToModel(req.Policies),
			Security:      mapSecurityAPIToModel(req.Security),
		},
	}

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrLLMProviderExists
		}
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	created, err := s.repo.GetByID(req.Id, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created provider: %w", err)
	}
	if created == nil {
		return nil, constants.ErrLLMProviderNotFound
	}
	return mapProviderModelToAPI(created, tpl.ID), nil
}

func (s *LLMProviderService) List(orgUUID string, limit, offset int) (*api.LLMProviderListResponse, error) {
	items, err := s.repo.List(orgUUID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}
	totalCount, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count providers: %w", err)
	}
	resp := &api.LLMProviderListResponse{
		Count: len(items),
		Pagination: api.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}
	resp.List = make([]api.LLMProviderListItem, 0, len(items))
	for _, p := range items {
		// Look up template handle from UUID
		tplHandle := ""
		if p.TemplateUUID != "" {
			tpl, err := s.templateRepo.GetByUUID(p.TemplateUUID, orgUUID)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve template for provider %s: %w", p.ID, err)
			}
			if tpl != nil {
				tplHandle = tpl.ID
			}
		}
		id := p.ID
		name := p.Name
		desc := stringPtrIfNotEmpty(p.Description)
		createdBy := stringPtrIfNotEmpty(p.CreatedBy)
		version := p.Version
		template := stringPtrIfNotEmpty(tplHandle)
		status := api.LLMProviderListItemStatus(p.Status)
		resp.List = append(resp.List, api.LLMProviderListItem{
			Id:          &id,
			Name:        &name,
			Description: desc,
			CreatedBy:   createdBy,
			Version:     &version,
			Template:    template,
			Status:      &status,
			CreatedAt:   timePtr(p.CreatedAt),
			UpdatedAt:   timePtr(p.UpdatedAt),
		})
	}
	return resp, nil
}

func (s *LLMProviderService) Get(orgUUID, handle string) (*api.LLMProvider, error) {
	if handle == "" {
		return nil, constants.ErrInvalidInput
	}
	m, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}
	if m == nil {
		return nil, constants.ErrLLMProviderNotFound
	}
	// Look up template handle from UUID
	tplHandle := ""
	if m.TemplateUUID != "" {
		tpl, err := s.templateRepo.GetByUUID(m.TemplateUUID, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve template for provider %s: %w", m.ID, err)
		}
		if tpl != nil {
			tplHandle = tpl.ID
		}
	}
	return mapProviderModelToAPI(m, tplHandle), nil
}

func (s *LLMProviderService) Update(orgUUID, handle string, req *api.LLMProvider) (*api.LLMProvider, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id != "" && req.Id != handle {
		return nil, constants.ErrInvalidInput
	}
	// Fetch existing provider to preserve sensitive fields on update
	existing, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch existing provider: %w", err)
	}
	if existing == nil {
		return nil, constants.ErrLLMProviderNotFound
	}
	if req.Name == "" || req.Version == "" || req.Template == "" {
		return nil, constants.ErrInvalidInput
	}
	if err := validateModelProviders(req.Template, req.ModelProviders); err != nil {
		return nil, err
	}
	if err := validateUpstream(req.Upstream); err != nil {
		return nil, err
	}
	if err := validateRateLimitingConfig(req.RateLimiting); err != nil {
		return nil, err
	}

	// Validate template exists
	tpl, err := s.templateRepo.GetByID(req.Template, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate template: %w", err)
	}
	if tpl == nil {
		return nil, constants.ErrLLMProviderTemplateNotFound
	}

	contextValue := defaultStringPtr(req.Context, "/")
	m := &model.LLMProvider{
		OrganizationUUID: orgUUID,
		ID:               handle,
		Name:             req.Name,
		Description:      valueOrEmpty(req.Description),
		Version:          req.Version,
		TemplateUUID:     tpl.UUID,
		OpenAPISpec:      valueOrEmpty(req.Openapi),
		ModelProviders:   mapModelProvidersAPI(req.ModelProviders),
		Status:           llmStatusPending,
		Configuration: model.LLMProviderConfig{
			Context:       &contextValue,
			VHost:         req.Vhost,
			Upstream:      mapUpstreamAPIToModel(req.Upstream),
			AccessControl: mapAccessControlAPI(&req.AccessControl),
			RateLimiting:  mapRateLimitingAPIToModel(req.RateLimiting),
			Policies:      mapPoliciesAPIToModel(req.Policies),
			Security:      mapSecurityAPIToModel(req.Security),
		},
	}

	// Preserve stored upstream auth credential only when auth object is provided with an empty value.
	// If auth object is omitted, treat it as explicit removal and clear stored auth.
	m.Configuration.Upstream = preserveUpstreamAuthValue(existing.Configuration.Upstream, m.Configuration.Upstream)

	if err := s.repo.Update(m); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrLLMProviderNotFound
		}
		return nil, fmt.Errorf("failed to update provider: %w", err)
	}

	updated, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated provider: %w", err)
	}
	if updated == nil {
		return nil, constants.ErrLLMProviderNotFound
	}
	return mapProviderModelToAPI(updated, tpl.ID), nil
}

func (s *LLMProviderService) Delete(orgUUID, handle string) error {
	if handle == "" {
		return constants.ErrInvalidInput
	}
	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrLLMProviderNotFound
		}
		return fmt.Errorf("failed to delete provider: %w", err)
	}
	return nil
}

func (s *LLMProxyService) Create(orgUUID, createdBy string, req *api.LLMProxy) (*api.LLMProxy, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id == "" || req.Name == "" || req.Version == "" || req.Provider.Id == "" || req.ProjectId == "" {
		return nil, constants.ErrInvalidInput
	}
	if s.projectRepo != nil {
		project, err := s.projectRepo.GetProjectByUUID(req.ProjectId)
		if err != nil {
			return nil, fmt.Errorf("failed to validate project: %w", err)
		}
		if project == nil || project.OrganizationID != orgUUID {
			return nil, constants.ErrProjectNotFound
		}
	}

	// Validate provider exists
	prov, err := s.providerRepo.GetByID(req.Provider.Id, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate provider: %w", err)
	}
	if prov == nil {
		return nil, constants.ErrLLMProviderNotFound
	}

	exists, err := s.repo.Exists(req.Id, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check proxy exists: %w", err)
	}
	if exists {
		return nil, constants.ErrLLMProxyExists
	}

	proxyCount, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count proxies: %w", err)
	}
	if err := validateLLMResourceLimit(proxyCount, constants.MaxLLMProxiesPerOrganization, constants.ErrLLMProxyLimitReached); err != nil {
		return nil, err
	}

	contextValue := defaultStringPtr(req.Context, "/")
	m := &model.LLMProxy{
		OrganizationUUID: orgUUID,
		ProjectUUID:      req.ProjectId,
		ID:               req.Id,
		Name:             req.Name,
		Description:      valueOrEmpty(req.Description),
		CreatedBy:        createdBy,
		Version:          req.Version,
		ProviderUUID:     prov.UUID,
		OpenAPISpec:      valueOrEmpty(req.Openapi),
		Status:           llmStatusPending,
		Configuration: model.LLMProxyConfig{
			Context:      &contextValue,
			Vhost:        req.Vhost,
			Provider:     req.Provider.Id,
			UpstreamAuth: mapUpstreamAuthAPIToModel(req.Provider.Auth),
			Policies:     mapPoliciesAPIToModel(req.Policies),
			Security:     mapSecurityAPIToModel(req.Security),
		},
	}

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrLLMProxyExists
		}
		return nil, fmt.Errorf("failed to create proxy: %w", err)
	}

	created, err := s.repo.GetByID(req.Id, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created proxy: %w", err)
	}
	if created == nil {
		return nil, constants.ErrLLMProxyNotFound
	}
	return mapProxyModelToAPI(created), nil
}

func (s *LLMProxyService) List(orgUUID string, projectUUID *string, limit, offset int) (*api.LLMProxyListResponse, error) {
	if projectUUID != nil && *projectUUID != "" && s.projectRepo != nil {
		project, err := s.projectRepo.GetProjectByUUID(*projectUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate project: %w", err)
		}
		if project == nil || project.OrganizationID != orgUUID {
			return nil, constants.ErrProjectNotFound
		}
	}

	var items []*model.LLMProxy
	var err error
	if projectUUID != nil && *projectUUID != "" {
		items, err = s.repo.ListByProject(orgUUID, *projectUUID, limit, offset)
	} else {
		items, err = s.repo.List(orgUUID, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list proxies: %w", err)
	}
	var totalCount int
	if projectUUID != nil && *projectUUID != "" {
		totalCount, err = s.repo.CountByProject(orgUUID, *projectUUID)
	} else {
		totalCount, err = s.repo.Count(orgUUID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to count proxies: %w", err)
	}
	resp := &api.LLMProxyListResponse{
		Count: len(items),
		Pagination: api.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}
	resp.List = make([]api.LLMProxyListItem, 0, len(items))
	for _, p := range items {
		id := p.ID
		name := p.Name
		desc := stringPtrIfNotEmpty(p.Description)
		createdBy := stringPtrIfNotEmpty(p.CreatedBy)
		contextValue := (*string)(nil)
		if p.Configuration.Context != nil {
			v := *p.Configuration.Context
			contextValue = &v
		}
		version := p.Version
		projectID := p.ProjectUUID
		provider := p.Configuration.Provider
		status := api.LLMProxyListItemStatus(p.Status)
		resp.List = append(resp.List, api.LLMProxyListItem{
			Id:          &id,
			Name:        &name,
			Description: desc,
			CreatedBy:   createdBy,
			Context:     contextValue,
			Version:     &version,
			ProjectId:   &projectID,
			Provider:    &provider,
			Status:      &status,
			CreatedAt:   timePtr(p.CreatedAt),
			UpdatedAt:   timePtr(p.UpdatedAt),
		})
	}
	return resp, nil
}

func (s *LLMProxyService) ListByProvider(orgUUID, providerID string, limit, offset int) (*api.LLMProxyListResponse, error) {
	if providerID == "" {
		return nil, constants.ErrInvalidInput
	}
	if s.providerRepo == nil {
		return nil, fmt.Errorf("could not initialize llmprovider repository")
	}
	prov, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate provider: %w", err)
	}
	if prov == nil {
		return nil, constants.ErrLLMProviderNotFound
	}

	items, err := s.repo.ListByProvider(orgUUID, prov.UUID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list proxies by provider: %w", err)
	}
	totalCount, err := s.repo.CountByProvider(orgUUID, prov.UUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count proxies by provider: %w", err)
	}
	resp := &api.LLMProxyListResponse{
		Count: len(items),
		Pagination: api.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}
	resp.List = make([]api.LLMProxyListItem, 0, len(items))
	for _, p := range items {
		id := p.ID
		name := p.Name
		desc := stringPtrIfNotEmpty(p.Description)
		createdBy := stringPtrIfNotEmpty(p.CreatedBy)
		contextValue := (*string)(nil)
		if p.Configuration.Context != nil {
			v := *p.Configuration.Context
			contextValue = &v
		}
		version := p.Version
		projectID := p.ProjectUUID
		provider := p.Configuration.Provider
		status := api.LLMProxyListItemStatus(p.Status)
		resp.List = append(resp.List, api.LLMProxyListItem{
			Id:          &id,
			Name:        &name,
			Description: desc,
			CreatedBy:   createdBy,
			Context:     contextValue,
			Version:     &version,
			ProjectId:   &projectID,
			Provider:    &provider,
			Status:      &status,
			CreatedAt:   timePtr(p.CreatedAt),
			UpdatedAt:   timePtr(p.UpdatedAt),
		})
	}
	return resp, nil
}

func (s *LLMProxyService) Get(orgUUID, handle string) (*api.LLMProxy, error) {
	if handle == "" {
		return nil, constants.ErrInvalidInput
	}
	m, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get proxy: %w", err)
	}
	if m == nil {
		return nil, constants.ErrLLMProxyNotFound
	}
	return mapProxyModelToAPI(m), nil
}

func (s *LLMProxyService) Update(orgUUID, handle string, req *api.LLMProxy) (*api.LLMProxy, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id != "" && req.Id != handle {
		return nil, constants.ErrInvalidInput
	}
	if req.Name == "" || req.Version == "" || req.Provider.Id == "" {
		return nil, constants.ErrInvalidInput
	}

	existing, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing proxy: %w", err)
	}
	if existing == nil {
		return nil, constants.ErrLLMProxyNotFound
	}

	// Validate provider exists
	prov, err := s.providerRepo.GetByID(req.Provider.Id, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate provider: %w", err)
	}
	if prov == nil {
		return nil, constants.ErrLLMProviderNotFound
	}

	contextValue := defaultStringPtr(req.Context, "/")
	m := &model.LLMProxy{
		OrganizationUUID: orgUUID,
		ID:               handle,
		Name:             req.Name,
		Description:      valueOrEmpty(req.Description),
		Version:          req.Version,
		ProviderUUID:     prov.UUID,
		OpenAPISpec:      valueOrEmpty(req.Openapi),
		Status:           llmStatusPending,
		Configuration: model.LLMProxyConfig{
			Context:      &contextValue,
			Vhost:        req.Vhost,
			Provider:     req.Provider.Id,
			UpstreamAuth: mapUpstreamAuthAPIToModel(req.Provider.Auth),
			Policies:     mapPoliciesAPIToModel(req.Policies),
			Security:     mapSecurityAPIToModel(req.Security),
		},
	}

	// Preserve stored upstream auth credential when not supplied in update payload
	m.Configuration.UpstreamAuth = preserveUpstreamAuthCredential(existing.Configuration.UpstreamAuth, m.Configuration.UpstreamAuth)
	if err := s.repo.Update(m); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrLLMProxyNotFound
		}
		return nil, fmt.Errorf("failed to update proxy: %w", err)
	}

	updated, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated proxy: %w", err)
	}
	if updated == nil {
		return nil, constants.ErrLLMProxyNotFound
	}
	return mapProxyModelToAPI(updated), nil
}

func (s *LLMProxyService) Delete(orgUUID, handle string) error {
	if handle == "" {
		return constants.ErrInvalidInput
	}
	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrLLMProxyNotFound
		}
		return fmt.Errorf("failed to delete proxy: %w", err)
	}
	return nil
}

// ---- helpers ----

func isSQLiteUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func validateUpstream(u api.Upstream) error {
	mainUrl := valueOrEmpty(u.Main.Url)
	mainRef := valueOrEmpty(u.Main.Ref)
	if strings.TrimSpace(mainUrl) == "" && strings.TrimSpace(mainRef) == "" {
		return constants.ErrInvalidInput
	}
	return nil
}

func preserveUpstreamAuthValue(existing, updated *model.UpstreamConfig) *model.UpstreamConfig {
	if updated == nil {
		return existing
	}
	if existing == nil {
		return updated
	}
	if updated.Main == nil {
		return existing
	}
	if existing.Main == nil || existing.Main.Auth == nil {
		return updated
	}
	if updated.Main.Auth == nil {
		return updated
	}
	if updated.Main.Auth.Value == "" {
		updated.Main.Auth.Value = existing.Main.Auth.Value
	}
	return updated
}

func preserveUpstreamAuthCredential(existing, updated *model.UpstreamAuth) *model.UpstreamAuth {
	if updated == nil {
		return existing
	}
	if existing == nil {
		return updated
	}
	if updated.Type == "" {
		updated.Type = existing.Type
	}
	if updated.Header == "" {
		updated.Header = existing.Header
	}
	if updated.Value == "" {
		updated.Value = existing.Value
	}
	return updated
}

func defaultStringPtr(v *string, def string) string {
	if v == nil {
		return def
	}
	if strings.TrimSpace(*v) == "" {
		return def
	}
	return *v
}

func valueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func stringPtrIfNotEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	v := s
	return &v
}

func timePtr(t time.Time) *time.Time {
	tt := t
	return &tt
}

func validateLLMResourceLimit(currentCount int, maxAllowed int, limitErr error) error {
	if currentCount >= maxAllowed {
		return limitErr
	}
	return nil
}

func mapExtractionIdentifierAPI(in *api.ExtractionIdentifier) *model.ExtractionIdentifier {
	if in == nil {
		return nil
	}
	return &model.ExtractionIdentifier{Location: string(in.Location), Identifier: in.Identifier}
}

func mapAccessControlAPI(in *api.LLMAccessControl) *model.LLMAccessControl {
	if in == nil {
		return nil
	}
	out := &model.LLMAccessControl{Mode: string(in.Mode)}
	if in.Exceptions != nil && len(*in.Exceptions) > 0 {
		out.Exceptions = make([]model.RouteException, 0, len(*in.Exceptions))
		for _, e := range *in.Exceptions {
			methods := make([]string, 0, len(e.Methods))
			for _, m := range e.Methods {
				methods = append(methods, string(m))
			}
			out.Exceptions = append(out.Exceptions, model.RouteException{Path: e.Path, Methods: methods})
		}
	}
	return out
}

func mapPoliciesAPIToModel(in *[]api.LLMPolicy) []model.LLMPolicy {
	if in == nil || len(*in) == 0 {
		return nil
	}
	out := make([]model.LLMPolicy, 0, len(*in))
	for _, p := range *in {
		paths := make([]model.LLMPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			methods := make([]string, 0, len(pp.Methods))
			for _, m := range pp.Methods {
				methods = append(methods, string(m))
			}
			paths = append(paths, model.LLMPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		out = append(out, model.LLMPolicy{Name: p.Name, Version: p.Version, Paths: paths})
	}
	return out
}

func mapUpstreamAuthAPIToModel(in *api.UpstreamAuth) *model.UpstreamAuth {
	if in == nil {
		return nil
	}
	authType := ""
	if in.Type != nil {
		authType = normalizeUpstreamAuthType(string(*in.Type))
	}
	return &model.UpstreamAuth{
		Type:   authType,
		Header: valueOrEmpty(in.Header),
		Value:  valueOrEmpty(in.Value),
	}
}

func normalizeUpstreamAuthType(authType string) string {
	normalized := strings.TrimSpace(authType)
	if normalized == "" {
		return ""
	}

	canonical := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(normalized, "-", ""), "_", ""))
	switch canonical {
	case "apikey":
		return string(api.ApiKey)
	case "basic":
		return string(api.Basic)
	case "bearer":
		return string(api.Bearer)
	default:
		return normalized
	}
}

func mapUpstreamAPIToModel(in api.Upstream) *model.UpstreamConfig {
	out := &model.UpstreamConfig{}
	out.Main = &model.UpstreamEndpoint{
		URL: valueOrEmpty(in.Main.Url),
		Ref: valueOrEmpty(in.Main.Ref),
	}
	if in.Main.Auth != nil {
		out.Main.Auth = mapUpstreamAuthAPIToModel(in.Main.Auth)
	}
	if in.Sandbox != nil {
		out.Sandbox = &model.UpstreamEndpoint{
			URL: valueOrEmpty(in.Sandbox.Url),
			Ref: valueOrEmpty(in.Sandbox.Ref),
		}
		if in.Sandbox.Auth != nil {
			out.Sandbox.Auth = mapUpstreamAuthAPIToModel(in.Sandbox.Auth)
		}
	}
	return out
}

func mapUpstreamModelToAPI(in *model.UpstreamConfig) api.Upstream {
	main := api.UpstreamDefinition{}
	if in != nil && in.Main != nil {
		if strings.TrimSpace(in.Main.URL) != "" {
			u := in.Main.URL
			main.Url = &u
		}
		if strings.TrimSpace(in.Main.Ref) != "" {
			r := in.Main.Ref
			main.Ref = &r
		}
		if in.Main.Auth != nil {
			main.Auth = mapUpstreamAuthModelToAPI(in.Main.Auth)
		}
	}
	var sandbox *api.UpstreamDefinition
	if in != nil && in.Sandbox != nil {
		s := api.UpstreamDefinition{}
		if strings.TrimSpace(in.Sandbox.URL) != "" {
			u := in.Sandbox.URL
			s.Url = &u
		}
		if strings.TrimSpace(in.Sandbox.Ref) != "" {
			r := in.Sandbox.Ref
			s.Ref = &r
		}
		if in.Sandbox.Auth != nil {
			s.Auth = mapUpstreamAuthModelToAPI(in.Sandbox.Auth)
		}
		sandbox = &s
	}
	return api.Upstream{Main: main, Sandbox: sandbox}
}

// mapUpstreamConfigToDTO maps upstream config to API type with auth values redacted for security
func mapUpstreamConfigToDTO(in *model.UpstreamConfig) api.Upstream {
	main := api.UpstreamDefinition{}
	if in != nil && in.Main != nil {
		if strings.TrimSpace(in.Main.URL) != "" {
			u := in.Main.URL
			main.Url = &u
		}
		if strings.TrimSpace(in.Main.Ref) != "" {
			r := in.Main.Ref
			main.Ref = &r
		}
		if in.Main.Auth != nil {
			// Redact auth value for security
			authType := (*api.UpstreamAuthType)(nil)
			if in.Main.Auth.Type != "" {
				t := api.UpstreamAuthType(in.Main.Auth.Type)
				authType = &t
			}
			main.Auth = &api.UpstreamAuth{
				Type:   authType,
				Header: stringPtrIfNotEmpty(in.Main.Auth.Header),
				Value:  nil, // Redact value
			}
		}
	}
	var sandbox *api.UpstreamDefinition
	if in != nil && in.Sandbox != nil {
		s := api.UpstreamDefinition{}
		if strings.TrimSpace(in.Sandbox.URL) != "" {
			u := in.Sandbox.URL
			s.Url = &u
		}
		if strings.TrimSpace(in.Sandbox.Ref) != "" {
			r := in.Sandbox.Ref
			s.Ref = &r
		}
		if in.Sandbox.Auth != nil {
			// Redact auth value for security
			authType := (*api.UpstreamAuthType)(nil)
			if in.Sandbox.Auth.Type != "" {
				t := api.UpstreamAuthType(in.Sandbox.Auth.Type)
				authType = &t
			}
			s.Auth = &api.UpstreamAuth{
				Type:   authType,
				Header: stringPtrIfNotEmpty(in.Sandbox.Auth.Header),
				Value:  nil, // Redact value
			}
		}
		sandbox = &s
	}
	return api.Upstream{Main: main, Sandbox: sandbox}
}

func mapUpstreamAuthModelToAPI(in *model.UpstreamAuth) *api.UpstreamAuth {
	if in == nil {
		return nil
	}
	var authType *api.UpstreamAuthType
	if normalized := normalizeUpstreamAuthType(in.Type); normalized != "" {
		t := api.UpstreamAuthType(normalized)
		authType = &t
	}
	return &api.UpstreamAuth{
		Type:   authType,
		Header: stringPtrIfNotEmpty(in.Header),
		Value:  stringPtrIfNotEmpty(in.Value),
	}
}

func mapRateLimitingAPIToModel(in *api.LLMRateLimitingConfig) *model.LLMRateLimitingConfig {
	if in == nil {
		return nil
	}
	return &model.LLMRateLimitingConfig{
		ProviderLevel: mapRateLimitingScopeAPIToModel(in.ProviderLevel),
		ConsumerLevel: mapRateLimitingScopeAPIToModel(in.ConsumerLevel),
	}
}

func mapRateLimitingScopeAPIToModel(in *api.RateLimitingScopeConfig) *model.RateLimitingScopeConfig {
	if in == nil {
		return nil
	}
	return &model.RateLimitingScopeConfig{
		Global:       mapRateLimitingLimitAPIToModel(in.Global),
		ResourceWise: mapResourceWiseRateLimitingAPIToModel(in.ResourceWise),
	}
}

func mapRateLimitingLimitAPIToModel(in *api.RateLimitingLimitConfig) *model.RateLimitingLimitConfig {
	if in == nil {
		return nil
	}
	return &model.RateLimitingLimitConfig{
		Request: mapRequestRateLimitAPIToModel(in.Request),
		Token:   mapTokenRateLimitAPIToModel(in.Token),
		Cost:    mapCostRateLimitAPIToModel(in.Cost),
	}
}

func mapRequestRateLimitAPIToModel(in *api.RequestRateLimitDimension) *model.RequestRateLimit {
	if in == nil {
		return nil
	}
	enabled := false
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	count := 0
	if in.Count != nil {
		count = *in.Count
	}
	reset := model.RateLimitResetWindow{}
	if in.Reset != nil {
		reset = mapRateLimitResetWindowAPIToModel(*in.Reset)
	}
	return &model.RequestRateLimit{
		Enabled: enabled,
		Count:   count,
		Reset:   reset,
	}
}

func mapTokenRateLimitAPIToModel(in *api.TokenRateLimitDimension) *model.TokenRateLimit {
	if in == nil {
		return nil
	}
	enabled := false
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	count := 0
	if in.Count != nil {
		count = *in.Count
	}
	reset := model.RateLimitResetWindow{}
	if in.Reset != nil {
		reset = mapRateLimitResetWindowAPIToModel(*in.Reset)
	}
	return &model.TokenRateLimit{
		Enabled: enabled,
		Count:   count,
		Reset:   reset,
	}
}

func mapCostRateLimitAPIToModel(in *api.CostRateLimitDimension) *model.CostRateLimit {
	if in == nil {
		return nil
	}
	enabled := false
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	amount := 0.0
	if in.Amount != nil {
		amount = float64(*in.Amount)
	}
	reset := model.RateLimitResetWindow{}
	if in.Reset != nil {
		reset = mapRateLimitResetWindowAPIToModel(*in.Reset)
	}
	return &model.CostRateLimit{
		Enabled: enabled,
		Amount:  amount,
		Reset:   reset,
	}
}

func mapRateLimitResetWindowAPIToModel(in api.RateLimitResetWindow) model.RateLimitResetWindow {
	return model.RateLimitResetWindow{
		Duration: in.Duration,
		Unit:     string(in.Unit),
	}
}

func mapResourceWiseRateLimitingAPIToModel(in *api.ResourceWiseRateLimitingConfig) *model.ResourceWiseRateLimitingConfig {
	if in == nil {
		return nil
	}
	resources := make([]model.RateLimitingResourceLimit, 0, len(in.Resources))
	for _, r := range in.Resources {
		methods := make([]string, 0, len(r.Methods))
		for _, m := range r.Methods {
			methods = append(methods, string(m))
		}
		resources = append(resources, model.RateLimitingResourceLimit{
			Methods:  methods,
			Resource: r.Resource,
			Limit:    *mapRateLimitingLimitAPIToModel(&r.Limit),
		})
	}
	return &model.ResourceWiseRateLimitingConfig{
		Default:   *mapRateLimitingLimitAPIToModel(&in.Default),
		Resources: resources,
	}
}

func mapTemplateModelToAPI(m *model.LLMProviderTemplate) *api.LLMProviderTemplate {
	if m == nil {
		return nil
	}
	return &api.LLMProviderTemplate{
		Id:               m.ID,
		Name:             m.Name,
		Description:      stringPtrIfNotEmpty(m.Description),
		CreatedBy:        stringPtrIfNotEmpty(m.CreatedBy),
		Metadata:         mapTemplateMetadataModelToAPI(m.Metadata),
		PromptTokens:     mapExtractionIdentifierModelToAPI(m.PromptTokens),
		CompletionTokens: mapExtractionIdentifierModelToAPI(m.CompletionTokens),
		TotalTokens:      mapExtractionIdentifierModelToAPI(m.TotalTokens),
		RemainingTokens:  mapExtractionIdentifierModelToAPI(m.RemainingTokens),
		RequestModel:     mapExtractionIdentifierModelToAPI(m.RequestModel),
		ResponseModel:    mapExtractionIdentifierModelToAPI(m.ResponseModel),
		CreatedAt:        timePtr(m.CreatedAt),
		UpdatedAt:        timePtr(m.UpdatedAt),
	}
}

func mapTemplateMetadataAPI(in *api.LLMProviderTemplateMetadata) *model.LLMProviderTemplateMetadata {
	if in == nil {
		return nil
	}
	var auth *model.LLMProviderTemplateAuth
	if in.Auth != nil {
		auth = &model.LLMProviderTemplateAuth{
			Type:        valueOrEmpty(in.Auth.Type),
			Header:      valueOrEmpty(in.Auth.Header),
			ValuePrefix: valueOrEmpty(in.Auth.ValuePrefix),
		}
	}
	out := &model.LLMProviderTemplateMetadata{
		EndpointURL:    strings.TrimSpace(valueOrEmpty(in.EndpointUrl)),
		Auth:           auth,
		LogoURL:        strings.TrimSpace(valueOrEmpty(in.LogoUrl)),
		OpenapiSpecURL: strings.TrimSpace(valueOrEmpty(in.OpenapiSpecUrl)),
	}
	if out.EndpointURL == "" && out.LogoURL == "" && out.Auth == nil && out.OpenapiSpecURL == "" {
		return nil
	}
	return out
}

func mapTemplateMetadataModelToAPI(in *model.LLMProviderTemplateMetadata) *api.LLMProviderTemplateMetadata {
	if in == nil {
		return nil
	}
	var auth *api.LLMProviderTemplateAuth
	if in.Auth != nil {
		auth = &api.LLMProviderTemplateAuth{
			Type:        stringPtrIfNotEmpty(in.Auth.Type),
			Header:      stringPtrIfNotEmpty(in.Auth.Header),
			ValuePrefix: stringPtrIfNotEmpty(in.Auth.ValuePrefix),
		}
	}
	return &api.LLMProviderTemplateMetadata{
		EndpointUrl:    stringPtrIfNotEmpty(in.EndpointURL),
		Auth:           auth,
		LogoUrl:        stringPtrIfNotEmpty(in.LogoURL),
		OpenapiSpecUrl: stringPtrIfNotEmpty(in.OpenapiSpecURL),
	}
}

func mapExtractionIdentifierModelToAPI(m *model.ExtractionIdentifier) *api.ExtractionIdentifier {
	if m == nil {
		return nil
	}
	return &api.ExtractionIdentifier{Location: api.ExtractionIdentifierLocation(m.Location), Identifier: m.Identifier}
}

func mapProviderModelToAPI(m *model.LLMProvider, templateHandle string) *api.LLMProvider {
	if m == nil {
		return nil
	}
	ctx := (*string)(nil)
	if m.Configuration.Context != nil {
		v := *m.Configuration.Context
		ctx = &v
	}
	// Use redacted upstream mapping (never expose auth credential values)
	upstream := mapUpstreamConfigToDTO(m.Configuration.Upstream)
	ac := api.LLMAccessControl{Mode: api.LLMAccessControlMode("deny_all")}
	if m.Configuration.AccessControl != nil {
		ac.Mode = api.LLMAccessControlMode(m.Configuration.AccessControl.Mode)
		exc := make([]api.RouteException, 0, len(m.Configuration.AccessControl.Exceptions))
		for _, e := range m.Configuration.AccessControl.Exceptions {
			methods := make([]api.RouteExceptionMethods, 0, len(e.Methods))
			for _, mm := range e.Methods {
				methods = append(methods, api.RouteExceptionMethods(mm))
			}
			exc = append(exc, api.RouteException{Path: e.Path, Methods: methods})
		}
		if exc == nil {
			exc = []api.RouteException{}
		}
		ac.Exceptions = &exc
	} else {
		exc := []api.RouteException{}
		ac.Exceptions = &exc
	}

	policies := mapPoliciesModelToAPI(m.Configuration.Policies)
	if policies == nil {
		empty := []api.LLMPolicy{}
		policies = &empty
	}

	modelProviders := mapModelProvidersModelToAPI(m.ModelProviders)
	if modelProviders == nil {
		empty := []api.LLMModelProvider{}
		modelProviders = &empty
	}

	out := &api.LLMProvider{
		Id:             m.ID,
		Name:           m.Name,
		Description:    stringPtrIfNotEmpty(m.Description),
		CreatedBy:      stringPtrIfNotEmpty(m.CreatedBy),
		Version:        m.Version,
		Context:        ctx,
		Vhost:          m.Configuration.VHost,
		Template:       templateHandle,
		Openapi:        stringPtrIfNotEmpty(m.OpenAPISpec),
		ModelProviders: modelProviders,
		RateLimiting:   mapRateLimitingModelToAPI(m.Configuration.RateLimiting),
		Upstream:       upstream,
		AccessControl:  ac,
		Policies:       policies,
		Security:       mapSecurityModelToAPI(m.Configuration.Security),
		CreatedAt:      timePtr(m.CreatedAt),
		UpdatedAt:      timePtr(m.UpdatedAt),
	}
	return out
}

func validateModelProviders(template string, providers *[]api.LLMModelProvider) error {
	if providers == nil || len(*providers) == 0 {
		return nil
	}

	aggregatorTemplates := map[string]bool{
		"awsbedrock":     true,
		"azureaifoundry": true,
	}
	if !aggregatorTemplates[template] && len(*providers) > 1 {
		return constants.ErrInvalidInput
	}

	seenProviders := make(map[string]struct{}, len(*providers))
	for _, p := range *providers {
		if strings.TrimSpace(p.Id) == "" {
			return constants.ErrInvalidInput
		}
		if _, ok := seenProviders[p.Id]; ok {
			return constants.ErrInvalidInput
		}
		seenProviders[p.Id] = struct{}{}

		models := []api.LLMModel{}
		if p.Models != nil {
			models = *p.Models
		}
		seenModels := make(map[string]struct{}, len(models))
		for _, m := range models {
			if strings.TrimSpace(m.Id) == "" {
				return constants.ErrInvalidInput
			}
			if _, ok := seenModels[m.Id]; ok {
				return constants.ErrInvalidInput
			}
			seenModels[m.Id] = struct{}{}
		}
	}
	return nil
}

func mapModelProvidersAPI(in *[]api.LLMModelProvider) []model.LLMModelProvider {
	if in == nil || len(*in) == 0 {
		return nil
	}
	out := make([]model.LLMModelProvider, 0, len(*in))
	for _, p := range *in {
		models := make([]model.LLMModel, 0)
		if p.Models != nil {
			models = make([]model.LLMModel, 0, len(*p.Models))
			for _, m := range *p.Models {
				models = append(models, model.LLMModel{ID: m.Id, Name: valueOrEmpty(m.Name), Description: valueOrEmpty(m.Description)})
			}
		}
		out = append(out, model.LLMModelProvider{ID: p.Id, Name: valueOrEmpty(p.Name), Models: models})
	}
	return out
}

func mapModelProvidersModelToAPI(in []model.LLMModelProvider) *[]api.LLMModelProvider {
	if len(in) == 0 {
		return nil
	}
	out := make([]api.LLMModelProvider, 0, len(in))
	for _, p := range in {
		models := make([]api.LLMModel, 0, len(p.Models))
		for _, m := range p.Models {
			models = append(models, api.LLMModel{Id: m.ID, Name: stringPtrIfNotEmpty(m.Name), Description: stringPtrIfNotEmpty(m.Description)})
		}
		modelsPtr := &models
		out = append(out, api.LLMModelProvider{Id: p.ID, Name: stringPtrIfNotEmpty(p.Name), Models: modelsPtr})
	}
	return &out
}

func mapPoliciesModelToAPI(in []model.LLMPolicy) *[]api.LLMPolicy {
	if len(in) == 0 {
		return nil
	}
	out := make([]api.LLMPolicy, 0, len(in))
	for _, p := range in {
		paths := make([]api.LLMPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			methods := make([]api.LLMPolicyPathMethods, 0, len(pp.Methods))
			for _, m := range pp.Methods {
				methods = append(methods, api.LLMPolicyPathMethods(m))
			}
			paths = append(paths, api.LLMPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		out = append(out, api.LLMPolicy{Name: p.Name, Version: p.Version, Paths: paths})
	}
	return &out
}

func mapRateLimitingModelToAPI(in *model.LLMRateLimitingConfig) *api.LLMRateLimitingConfig {
	if in == nil {
		return nil
	}
	return &api.LLMRateLimitingConfig{
		ProviderLevel: mapRateLimitingScopeModelToAPI(in.ProviderLevel),
		ConsumerLevel: mapRateLimitingScopeModelToAPI(in.ConsumerLevel),
	}
}

func mapRateLimitingScopeModelToAPI(in *model.RateLimitingScopeConfig) *api.RateLimitingScopeConfig {
	if in == nil {
		return nil
	}
	return &api.RateLimitingScopeConfig{
		Global:       mapRateLimitingLimitModelToAPIPtr(in.Global),
		ResourceWise: mapResourceWiseRateLimitingModelToAPI(in.ResourceWise),
	}
}

func mapRateLimitingLimitModelToAPIPtr(in *model.RateLimitingLimitConfig) *api.RateLimitingLimitConfig {
	if in == nil {
		return nil
	}
	v := mapRateLimitingLimitModelToAPIValue(in)
	return &v
}

func mapRateLimitingLimitModelToAPIValue(in *model.RateLimitingLimitConfig) api.RateLimitingLimitConfig {
	out := api.RateLimitingLimitConfig{}
	if in == nil {
		return out
	}
	if in.Request != nil {
		enabled := in.Request.Enabled
		count := in.Request.Count
		var reset *api.RateLimitResetWindow
		if in.Request.Reset.Duration > 0 && strings.TrimSpace(in.Request.Reset.Unit) != "" {
			r := api.RateLimitResetWindow{Duration: in.Request.Reset.Duration, Unit: api.RateLimitResetWindowUnit(in.Request.Reset.Unit)}
			reset = &r
		}
		out.Request = &api.RequestRateLimitDimension{Enabled: &enabled, Count: &count, Reset: reset}
	}
	if in.Token != nil {
		enabled := in.Token.Enabled
		count := in.Token.Count
		var reset *api.RateLimitResetWindow
		if in.Token.Reset.Duration > 0 && strings.TrimSpace(in.Token.Reset.Unit) != "" {
			r := api.RateLimitResetWindow{Duration: in.Token.Reset.Duration, Unit: api.RateLimitResetWindowUnit(in.Token.Reset.Unit)}
			reset = &r
		}
		out.Token = &api.TokenRateLimitDimension{Enabled: &enabled, Count: &count, Reset: reset}
	}
	if in.Cost != nil {
		enabled := in.Cost.Enabled
		amount := float32(in.Cost.Amount)
		var reset *api.RateLimitResetWindow
		if in.Cost.Reset.Duration > 0 && strings.TrimSpace(in.Cost.Reset.Unit) != "" {
			r := api.RateLimitResetWindow{Duration: in.Cost.Reset.Duration, Unit: api.RateLimitResetWindowUnit(in.Cost.Reset.Unit)}
			reset = &r
		}
		out.Cost = &api.CostRateLimitDimension{Enabled: &enabled, Amount: &amount, Reset: reset}
	}
	return out
}

func mapResourceWiseRateLimitingModelToAPI(in *model.ResourceWiseRateLimitingConfig) *api.ResourceWiseRateLimitingConfig {
	if in == nil {
		return nil
	}
	resources := make([]api.RateLimitingResourceLimit, 0, len(in.Resources))
	for _, r := range in.Resources {
		methods := make([]api.RateLimitingResourceLimitMethods, 0, len(r.Methods))
		for _, m := range r.Methods {
			methods = append(methods, api.RateLimitingResourceLimitMethods(m))
		}
		resources = append(resources, api.RateLimitingResourceLimit{Resource: r.Resource, Methods: methods, Limit: mapRateLimitingLimitModelToAPIValue(&r.Limit)})
	}
	return &api.ResourceWiseRateLimitingConfig{
		Default:   mapRateLimitingLimitModelToAPIValue(&in.Default),
		Resources: resources,
	}
}

func validateRateLimitingConfig(cfg *api.LLMRateLimitingConfig) error {
	if cfg == nil {
		return nil
	}
	if err := validateRateLimitingScope(cfg.ProviderLevel); err != nil {
		return err
	}
	if err := validateRateLimitingScope(cfg.ConsumerLevel); err != nil {
		return err
	}
	return nil
}

func validateRateLimitingScope(scope *api.RateLimitingScopeConfig) error {
	if scope == nil {
		return nil
	}
	if (scope.Global == nil && scope.ResourceWise == nil) || (scope.Global != nil && scope.ResourceWise != nil) {
		return constants.ErrInvalidInput
	}
	if scope.Global != nil {
		return validateRateLimitingLimit(scope.Global)
	}
	return validateResourceWiseRateLimiting(scope.ResourceWise)
}

func validateResourceWiseRateLimiting(cfg *api.ResourceWiseRateLimitingConfig) error {
	if cfg == nil {
		return constants.ErrInvalidInput
	}
	if err := validateRateLimitingLimit(&cfg.Default); err != nil {
		return err
	}
	if len(cfg.Resources) == 0 {
		return constants.ErrInvalidInput
	}
	for _, r := range cfg.Resources {
		if err := validateRateLimitingLimit(&r.Limit); err != nil {
			return err
		}
	}
	return nil
}

func boolPtrTrue(b *bool) bool {
	return b != nil && *b
}

func validateRateLimitingLimit(cfg *api.RateLimitingLimitConfig) error {
	if cfg == nil {
		return constants.ErrInvalidInput
	}
	requestEnabled := cfg.Request != nil && boolPtrTrue(cfg.Request.Enabled)
	tokenEnabled := cfg.Token != nil && boolPtrTrue(cfg.Token.Enabled)
	costEnabled := cfg.Cost != nil && boolPtrTrue(cfg.Cost.Enabled)

	if !requestEnabled && !tokenEnabled && !costEnabled {
		return nil
	}

	if requestEnabled {
		if cfg.Request.Count == nil || *cfg.Request.Count <= 0 || cfg.Request.Reset == nil || cfg.Request.Reset.Duration <= 0 {
			return constants.ErrInvalidInput
		}
		if !isValidResetUnit(string(cfg.Request.Reset.Unit)) {
			return constants.ErrInvalidInput
		}
	}
	if tokenEnabled {
		if cfg.Token.Count == nil || *cfg.Token.Count <= 0 || cfg.Token.Reset == nil || cfg.Token.Reset.Duration <= 0 {
			return constants.ErrInvalidInput
		}
		if !isValidResetUnit(string(cfg.Token.Reset.Unit)) {
			return constants.ErrInvalidInput
		}
	}
	if costEnabled {
		if cfg.Cost.Amount == nil || *cfg.Cost.Amount < 0 || cfg.Cost.Reset == nil || cfg.Cost.Reset.Duration <= 0 {
			return constants.ErrInvalidInput
		}
		if !isValidResetUnit(string(cfg.Cost.Reset.Unit)) {
			return constants.ErrInvalidInput
		}
	}
	return nil
}

func isValidResetUnit(unit string) bool {
	switch unit {
	case "minute", "hour", "day", "week", "month":
		return true
	default:
		return false
	}
}

func mapProxyModelToAPI(m *model.LLMProxy) *api.LLMProxy {
	if m == nil {
		return nil
	}
	contextValue := (*string)(nil)
	if m.Configuration.Context != nil {
		v := *m.Configuration.Context
		contextValue = &v
	}
	vhostValue := (*string)(nil)
	if m.Configuration.Vhost != nil {
		v := *m.Configuration.Vhost
		vhostValue = &v
	}
	policies := mapPoliciesModelToAPI(m.Configuration.Policies)
	if policies == nil {
		empty := []api.LLMPolicy{}
		policies = &empty
	}
	createdAt := timePtr(m.CreatedAt)
	updatedAt := timePtr(m.UpdatedAt)
	out := &api.LLMProxy{
		Id:          m.ID,
		Name:        m.Name,
		Description: stringPtrIfNotEmpty(m.Description),
		CreatedBy:   stringPtrIfNotEmpty(m.CreatedBy),
		Version:     m.Version,
		ProjectId:   m.ProjectUUID,
		Context:     contextValue,
		Vhost:       vhostValue,
		Provider: api.LLMProxyProvider{
			Id:   m.Configuration.Provider,
			Auth: nil,
		},
		Openapi:   stringPtrIfNotEmpty(m.OpenAPISpec),
		Security:  mapSecurityModelToAPI(m.Configuration.Security),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	if m.Configuration.UpstreamAuth != nil {
		authType := (*api.UpstreamAuthType)(nil)
		if m.Configuration.UpstreamAuth.Type != "" {
			t := api.UpstreamAuthType(m.Configuration.UpstreamAuth.Type)
			authType = &t
		}
		out.Provider.Auth = &api.UpstreamAuth{
			Type:   authType,
			Header: stringPtrIfNotEmpty(m.Configuration.UpstreamAuth.Header),
			Value:  nil, // Redact auth credential value
		}
	}
	if len(m.Configuration.Policies) > 0 {
		policyList := make([]api.LLMPolicy, 0, len(m.Configuration.Policies))
		for _, p := range m.Configuration.Policies {
			paths := make([]api.LLMPolicyPath, 0, len(p.Paths))
			for _, pp := range p.Paths {
				methods := make([]api.LLMPolicyPathMethods, 0, len(pp.Methods))
				for _, m := range pp.Methods {
					methods = append(methods, api.LLMPolicyPathMethods(m))
				}
				paths = append(paths, api.LLMPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
			}
			policyList = append(policyList, api.LLMPolicy{Name: p.Name, Version: p.Version, Paths: paths})
		}
		out.Policies = &policyList
	}
	if out.Policies == nil {
		empty := []api.LLMPolicy{}
		out.Policies = &empty
	}
	return out
}

func mapSecurityAPIToModel(in *api.SecurityConfig) *model.SecurityConfig {
	if in == nil {
		return nil
	}
	out := &model.SecurityConfig{Enabled: in.Enabled}
	if in.ApiKey != nil {
		key := valueOrEmpty(in.ApiKey.Key)
		inLoc := ""
		if in.ApiKey.In != nil {
			inLoc = string(*in.ApiKey.In)
		}
		out.APIKey = &model.APIKeySecurity{Enabled: in.ApiKey.Enabled, Key: key, In: inLoc}
	}
	return out
}

func mapSecurityModelToAPI(in *model.SecurityConfig) *api.SecurityConfig {
	if in == nil {
		return nil
	}
	out := &api.SecurityConfig{Enabled: in.Enabled}
	if in.APIKey != nil {
		var inLoc *api.APIKeySecurityIn
		if strings.TrimSpace(in.APIKey.In) != "" {
			v := api.APIKeySecurityIn(in.APIKey.In)
			inLoc = &v
		}
		out.ApiKey = &api.APIKeySecurity{Enabled: in.APIKey.Enabled, Key: stringPtrIfNotEmpty(in.APIKey.Key), In: inLoc}
	}
	return out
}

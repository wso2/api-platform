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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
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

func (s *LLMProviderTemplateService) Create(orgUUID, createdBy string, req *dto.LLMProviderTemplate) (*dto.LLMProviderTemplate, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.ID == "" || req.Name == "" {
		return nil, constants.ErrInvalidInput
	}

	exists, err := s.repo.Exists(req.ID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check template exists: %w", err)
	}
	if exists {
		return nil, constants.ErrLLMProviderTemplateExists
	}

	m := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               req.ID,
		Name:             req.Name,
		Description:      req.Description,
		CreatedBy:        createdBy,
		Metadata:         mapTemplateMetadata(req.Metadata),
		PromptTokens:     mapExtractionIdentifier(req.PromptTokens),
		CompletionTokens: mapExtractionIdentifier(req.CompletionTokens),
		TotalTokens:      mapExtractionIdentifier(req.TotalTokens),
		RemainingTokens:  mapExtractionIdentifier(req.RemainingTokens),
		RequestModel:     mapExtractionIdentifier(req.RequestModel),
		ResponseModel:    mapExtractionIdentifier(req.ResponseModel),
	}
	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrLLMProviderTemplateExists
		}
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	return mapTemplateModelToDTO(m), nil
}

func (s *LLMProviderTemplateService) List(orgUUID string, limit, offset int) (*dto.LLMProviderTemplateListResponse, error) {
	items, err := s.repo.List(orgUUID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	totalCount, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count templates: %w", err)
	}
	resp := &dto.LLMProviderTemplateListResponse{
		Count: len(items),
		Pagination: dto.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}
	resp.List = make([]dto.LLMProviderTemplateListItem, 0, len(items))
	for _, t := range items {
		resp.List = append(resp.List, dto.LLMProviderTemplateListItem{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			CreatedBy:   t.CreatedBy,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
		})
	}
	return resp, nil
}

func (s *LLMProviderTemplateService) Get(orgUUID, handle string) (*dto.LLMProviderTemplate, error) {
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
	return mapTemplateModelToDTO(m), nil
}

func (s *LLMProviderTemplateService) Update(orgUUID, handle string, req *dto.LLMProviderTemplate) (*dto.LLMProviderTemplate, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.ID != "" && req.ID != handle {
		return nil, constants.ErrInvalidInput
	}
	if req.Name == "" {
		return nil, constants.ErrInvalidInput
	}

	m := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               handle,
		Name:             req.Name,
		Description:      req.Description,
		Metadata:         mapTemplateMetadata(req.Metadata),
		PromptTokens:     mapExtractionIdentifier(req.PromptTokens),
		CompletionTokens: mapExtractionIdentifier(req.CompletionTokens),
		TotalTokens:      mapExtractionIdentifier(req.TotalTokens),
		RemainingTokens:  mapExtractionIdentifier(req.RemainingTokens),
		RequestModel:     mapExtractionIdentifier(req.RequestModel),
		ResponseModel:    mapExtractionIdentifier(req.ResponseModel),
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
	return mapTemplateModelToDTO(updated), nil
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

func (s *LLMProviderService) Create(orgUUID, createdBy string, req *dto.LLMProvider) (*dto.LLMProvider, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.ID == "" || req.Name == "" || req.Version == "" || req.Template == "" {
		return nil, constants.ErrInvalidInput
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

	exists, err := s.repo.Exists(req.ID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check provider exists: %w", err)
	}
	if exists {
		return nil, constants.ErrLLMProviderExists
	}

	m := &model.LLMProvider{
		OrganizationUUID: orgUUID,
		ID:               req.ID,
		Name:             req.Name,
		Description:      req.Description,
		CreatedBy:        createdBy,
		Version:          req.Version,
		Context:          defaultString(req.Context, "/"),
		VHost:            req.VHost,
		Template:         req.Template,
		UpstreamURL:      req.Upstream.URL,
		UpstreamAuth:     mapUpstreamAuth(req.Upstream.Auth),
		OpenAPISpec:      req.OpenAPI,
		AccessControl:    mapAccessControl(&req.AccessControl),
		RateLimiting:     mapRateLimiting(req.RateLimiting),
		Policies:         mapPolicies(req.Policies),
		Security:         mapSecurityConfig(req.Security),
		Status:           llmStatusPending,
	}

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrLLMProviderExists
		}
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	created, err := s.repo.GetByID(req.ID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created provider: %w", err)
	}
	if created == nil {
		return nil, constants.ErrLLMProviderNotFound
	}
	return mapProviderModelToDTO(created), nil
}

func (s *LLMProviderService) List(orgUUID string, limit, offset int) (*dto.LLMProviderListResponse, error) {
	items, err := s.repo.List(orgUUID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}
	totalCount, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count providers: %w", err)
	}
	resp := &dto.LLMProviderListResponse{
		Count: len(items),
		Pagination: dto.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}
	resp.List = make([]dto.LLMProviderListItem, 0, len(items))
	for _, p := range items {
		resp.List = append(resp.List, dto.LLMProviderListItem{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			CreatedBy:   p.CreatedBy,
			Version:     p.Version,
			Template:    p.Template,
			Status:      p.Status,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}
	return resp, nil
}

func (s *LLMProviderService) Get(orgUUID, handle string) (*dto.LLMProvider, error) {
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
	return mapProviderModelToDTO(m), nil
}

func (s *LLMProviderService) Update(orgUUID, handle string, req *dto.LLMProvider) (*dto.LLMProvider, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.ID != "" && req.ID != handle {
		return nil, constants.ErrInvalidInput
	}
	if req.Name == "" || req.Version == "" || req.Template == "" {
		return nil, constants.ErrInvalidInput
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

	m := &model.LLMProvider{
		OrganizationUUID: orgUUID,
		ID:               handle,
		Name:             req.Name,
		Description:      req.Description,
		Version:          req.Version,
		Context:          defaultString(req.Context, "/"),
		VHost:            req.VHost,
		Template:         req.Template,
		UpstreamURL:      req.Upstream.URL,
		UpstreamAuth:     mapUpstreamAuth(req.Upstream.Auth),
		OpenAPISpec:      req.OpenAPI,
		AccessControl:    mapAccessControl(&req.AccessControl),
		RateLimiting:     mapRateLimiting(req.RateLimiting),
		Policies:         mapPolicies(req.Policies),
		Security:         mapSecurityConfig(req.Security),
		Status:           llmStatusPending,
	}

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
	return mapProviderModelToDTO(updated), nil
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

func (s *LLMProxyService) Create(orgUUID, createdBy string, req *dto.LLMProxy) (*dto.LLMProxy, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.ID == "" || req.Name == "" || req.Version == "" || req.Provider == "" || req.ProjectID == "" {
		return nil, constants.ErrInvalidInput
	}
	if s.projectRepo != nil {
		project, err := s.projectRepo.GetProjectByUUID(req.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate project: %w", err)
		}
		if project == nil || project.OrganizationID != orgUUID {
			return nil, constants.ErrProjectNotFound
		}
	}

	// Validate provider exists
	prov, err := s.providerRepo.GetByID(req.Provider, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate provider: %w", err)
	}
	if prov == nil {
		return nil, constants.ErrLLMProviderNotFound
	}

	exists, err := s.repo.Exists(req.ID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check proxy exists: %w", err)
	}
	if exists {
		return nil, constants.ErrLLMProxyExists
	}

	m := &model.LLMProxy{
		OrganizationUUID: orgUUID,
		ProjectUUID:      req.ProjectID,
		ID:               req.ID,
		Name:             req.Name,
		Description:      req.Description,
		CreatedBy:        createdBy,
		Version:          req.Version,
		Context:          defaultString(req.Context, "/"),
		VHost:            req.VHost,
		Provider:         req.Provider,
		OpenAPISpec:      req.OpenAPI,
		AccessControl:    mapAccessControl(req.AccessControl),
		Policies:         mapPolicies(req.Policies),
		Security:         mapSecurityConfig(req.Security),
		Status:           llmStatusPending,
	}

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrLLMProxyExists
		}
		return nil, fmt.Errorf("failed to create proxy: %w", err)
	}

	created, err := s.repo.GetByID(req.ID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created proxy: %w", err)
	}
	if created == nil {
		return nil, constants.ErrLLMProxyNotFound
	}
	return mapProxyModelToDTO(created), nil
}

func (s *LLMProxyService) List(orgUUID string, projectUUID *string, limit, offset int) (*dto.LLMProxyListResponse, error) {
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
	resp := &dto.LLMProxyListResponse{
		Count: len(items),
		Pagination: dto.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}
	resp.List = make([]dto.LLMProxyListItem, 0, len(items))
	for _, p := range items {
		resp.List = append(resp.List, dto.LLMProxyListItem{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			CreatedBy:   p.CreatedBy,
			Version:     p.Version,
			ProjectID:   p.ProjectUUID,
			Provider:    p.Provider,
			Status:      p.Status,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}
	return resp, nil
}

func (s *LLMProxyService) ListByProvider(orgUUID, providerID string, limit, offset int) (*dto.LLMProxyListResponse, error) {
	if providerID == "" {
		return nil, constants.ErrInvalidInput
	}
	if s.providerRepo != nil {
		prov, err := s.providerRepo.GetByID(providerID, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate provider: %w", err)
		}
		if prov == nil {
			return nil, constants.ErrLLMProviderNotFound
		}
	}

	items, err := s.repo.ListByProvider(orgUUID, providerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list proxies by provider: %w", err)
	}
	totalCount, err := s.repo.CountByProvider(orgUUID, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to count proxies by provider: %w", err)
	}
	resp := &dto.LLMProxyListResponse{
		Count: len(items),
		Pagination: dto.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}
	resp.List = make([]dto.LLMProxyListItem, 0, len(items))
	for _, p := range items {
		resp.List = append(resp.List, dto.LLMProxyListItem{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			CreatedBy:   p.CreatedBy,
			Version:     p.Version,
			ProjectID:   p.ProjectUUID,
			Provider:    p.Provider,
			Status:      p.Status,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}
	return resp, nil
}

func (s *LLMProxyService) Get(orgUUID, handle string) (*dto.LLMProxy, error) {
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
	return mapProxyModelToDTO(m), nil
}

func (s *LLMProxyService) Update(orgUUID, handle string, req *dto.LLMProxy) (*dto.LLMProxy, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.ID != "" && req.ID != handle {
		return nil, constants.ErrInvalidInput
	}
	if req.Name == "" || req.Version == "" || req.Provider == "" {
		return nil, constants.ErrInvalidInput
	}

	// Validate provider exists
	prov, err := s.providerRepo.GetByID(req.Provider, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate provider: %w", err)
	}
	if prov == nil {
		return nil, constants.ErrLLMProviderNotFound
	}

	m := &model.LLMProxy{
		OrganizationUUID: orgUUID,
		ID:               handle,
		Name:             req.Name,
		Description:      req.Description,
		Version:          req.Version,
		Context:          defaultString(req.Context, "/"),
		VHost:            req.VHost,
		Provider:         req.Provider,
		OpenAPISpec:      req.OpenAPI,
		AccessControl:    mapAccessControl(req.AccessControl),
		Policies:         mapPolicies(req.Policies),
		Security:         mapSecurityConfig(req.Security),
		Status:           llmStatusPending,
	}
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
	return mapProxyModelToDTO(updated), nil
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

func validateUpstream(u dto.LLMUpstream) error {
	if u.URL == "" {
		return constants.ErrInvalidInput
	}
	return nil
}

func defaultString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func mapExtractionIdentifier(in *dto.ExtractionIdentifier) *model.ExtractionIdentifier {
	if in == nil {
		return nil
	}
	return &model.ExtractionIdentifier{Location: in.Location, Identifier: in.Identifier}
}

func mapAccessControl(in *dto.LLMAccessControl) *model.LLMAccessControl {
	if in == nil {
		return nil
	}
	out := &model.LLMAccessControl{Mode: in.Mode}
	if len(in.Exceptions) > 0 {
		out.Exceptions = make([]model.RouteException, 0, len(in.Exceptions))
		for _, e := range in.Exceptions {
			out.Exceptions = append(out.Exceptions, model.RouteException{Path: e.Path, Methods: e.Methods})
		}
	}
	return out
}

func mapPolicies(in []dto.LLMPolicy) []model.LLMPolicy {
	if len(in) == 0 {
		return nil
	}
	out := make([]model.LLMPolicy, 0, len(in))
	for _, p := range in {
		paths := make([]model.LLMPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			paths = append(paths, model.LLMPolicyPath{Path: pp.Path, Methods: pp.Methods, Params: pp.Params})
		}
		out = append(out, model.LLMPolicy{Name: p.Name, Version: p.Version, Paths: paths})
	}
	return out
}

func mapUpstreamAuth(in *dto.LLMUpstreamAuth) *model.UpstreamAuth {
	if in == nil {
		return nil
	}
	return &model.UpstreamAuth{Type: in.Type, Header: in.Header, Value: in.Value}
}

func mapRateLimiting(in *dto.LLMRateLimitingConfig) *model.LLMRateLimitingConfig {
	if in == nil {
		return nil
	}
	return &model.LLMRateLimitingConfig{
		ProviderLevel: mapRateLimitingScope(in.ProviderLevel),
		ConsumerLevel: mapRateLimitingScope(in.ConsumerLevel),
	}
}

func mapRateLimitingScope(in *dto.RateLimitingScopeConfig) *model.RateLimitingScopeConfig {
	if in == nil {
		return nil
	}
	return &model.RateLimitingScopeConfig{
		Global:       mapRateLimitingLimit(in.Global),
		ResourceWise: mapResourceWiseRateLimiting(in.ResourceWise),
	}
}

func mapRateLimitingLimit(in *dto.RateLimitingLimitConfig) *model.RateLimitingLimitConfig {
	if in == nil {
		return nil
	}
	return &model.RateLimitingLimitConfig{
		RequestCount:         in.RequestCount,
		RequestResetDuration: in.RequestResetDuration,
		RequestResetUnit:     in.RequestResetUnit,
		TokenCount:           in.TokenCount,
		TokenResetDuration:   in.TokenResetDuration,
		TokenResetUnit:       in.TokenResetUnit,
		Cost:                 in.Cost,
		CostResetDuration:    in.CostResetDuration,
		CostResetUnit:        in.CostResetUnit,
	}
}

func mapResourceWiseRateLimiting(in *dto.ResourceWiseRateLimitingConfig) *model.ResourceWiseRateLimitingConfig {
	if in == nil {
		return nil
	}
	resources := make([]model.RateLimitingResourceLimit, 0, len(in.Resources))
	for _, r := range in.Resources {
		resources = append(resources, model.RateLimitingResourceLimit{
			Resource: r.Resource,
			Limit:    *mapRateLimitingLimit(&r.Limit),
		})
	}
	return &model.ResourceWiseRateLimitingConfig{
		Default:   *mapRateLimitingLimit(&in.Default),
		Resources: resources,
	}
}

func mapSecurityConfig(in *dto.SecurityConfig) *model.SecurityConfig {
	if in == nil {
		return nil
	}
	out := &model.SecurityConfig{Enabled: in.Enabled}
	if in.APIKey != nil {
		out.APIKey = &model.APIKeySecurity{Enabled: in.APIKey.Enabled, Header: in.APIKey.Header, Query: in.APIKey.Query, Cookie: in.APIKey.Cookie}
	}
	if in.OAuth2 != nil {
		var grantTypes *model.OAuth2GrantTypes
		if in.OAuth2.GrantTypes != nil {
			grantTypes = &model.OAuth2GrantTypes{}
			if in.OAuth2.GrantTypes.AuthorizationCode != nil {
				grantTypes.AuthorizationCode = &model.AuthorizationCodeGrant{
					Enabled:     in.OAuth2.GrantTypes.AuthorizationCode.Enabled,
					CallbackURL: in.OAuth2.GrantTypes.AuthorizationCode.CallbackURL,
				}
			}
			if in.OAuth2.GrantTypes.Implicit != nil {
				grantTypes.Implicit = &model.ImplicitGrant{
					Enabled:     in.OAuth2.GrantTypes.Implicit.Enabled,
					CallbackURL: in.OAuth2.GrantTypes.Implicit.CallbackURL,
				}
			}
			if in.OAuth2.GrantTypes.Password != nil {
				grantTypes.Password = &model.PasswordGrant{Enabled: in.OAuth2.GrantTypes.Password.Enabled}
			}
			if in.OAuth2.GrantTypes.ClientCredentials != nil {
				grantTypes.ClientCredentials = &model.ClientCredentialsGrant{Enabled: in.OAuth2.GrantTypes.ClientCredentials.Enabled}
			}
		}
		out.OAuth2 = &model.OAuth2Security{GrantTypes: grantTypes, Scopes: in.OAuth2.Scopes}
	}
	if in.XHubSignature != nil {
		out.XHubSignature = &model.XHubSignatureSecurity{
			Enabled:   in.XHubSignature.Enabled,
			Header:    in.XHubSignature.Header,
			Secret:    in.XHubSignature.Secret,
			Algorithm: in.XHubSignature.Algorithm,
		}
	}
	return out
}

func mapSecurityConfigDTO(in *model.SecurityConfig) *dto.SecurityConfig {
	if in == nil {
		return nil
	}
	out := &dto.SecurityConfig{Enabled: in.Enabled}
	if in.APIKey != nil {
		out.APIKey = &dto.APIKeySecurity{Enabled: in.APIKey.Enabled, Header: in.APIKey.Header, Query: in.APIKey.Query, Cookie: in.APIKey.Cookie}
	}
	if in.OAuth2 != nil {
		var grantTypes *dto.OAuth2GrantTypes
		if in.OAuth2.GrantTypes != nil {
			grantTypes = &dto.OAuth2GrantTypes{}
			if in.OAuth2.GrantTypes.AuthorizationCode != nil {
				grantTypes.AuthorizationCode = &dto.AuthorizationCodeGrant{
					Enabled:     in.OAuth2.GrantTypes.AuthorizationCode.Enabled,
					CallbackURL: in.OAuth2.GrantTypes.AuthorizationCode.CallbackURL,
				}
			}
			if in.OAuth2.GrantTypes.Implicit != nil {
				grantTypes.Implicit = &dto.ImplicitGrant{
					Enabled:     in.OAuth2.GrantTypes.Implicit.Enabled,
					CallbackURL: in.OAuth2.GrantTypes.Implicit.CallbackURL,
				}
			}
			if in.OAuth2.GrantTypes.Password != nil {
				grantTypes.Password = &dto.PasswordGrant{Enabled: in.OAuth2.GrantTypes.Password.Enabled}
			}
			if in.OAuth2.GrantTypes.ClientCredentials != nil {
				grantTypes.ClientCredentials = &dto.ClientCredentialsGrant{Enabled: in.OAuth2.GrantTypes.ClientCredentials.Enabled}
			}
		}
		out.OAuth2 = &dto.OAuth2Security{GrantTypes: grantTypes, Scopes: in.OAuth2.Scopes}
	}
	if in.XHubSignature != nil {
		out.XHubSignature = &dto.XHubSignatureSecurity{
			Enabled:   in.XHubSignature.Enabled,
			Header:    in.XHubSignature.Header,
			Secret:    in.XHubSignature.Secret,
			Algorithm: in.XHubSignature.Algorithm,
		}
	}
	return out
}

func mapTemplateModelToDTO(m *model.LLMProviderTemplate) *dto.LLMProviderTemplate {
	if m == nil {
		return nil
	}
	return &dto.LLMProviderTemplate{
		ID:               m.ID,
		Name:             m.Name,
		Description:      m.Description,
		CreatedBy:        m.CreatedBy,
		Metadata:         mapTemplateMetadataDTO(m.Metadata),
		PromptTokens:     mapExtractionIdentifierDTO(m.PromptTokens),
		CompletionTokens: mapExtractionIdentifierDTO(m.CompletionTokens),
		TotalTokens:      mapExtractionIdentifierDTO(m.TotalTokens),
		RemainingTokens:  mapExtractionIdentifierDTO(m.RemainingTokens),
		RequestModel:     mapExtractionIdentifierDTO(m.RequestModel),
		ResponseModel:    mapExtractionIdentifierDTO(m.ResponseModel),
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}

func mapTemplateMetadata(in *dto.LLMProviderTemplateMetadata) *model.LLMProviderTemplateMetadata {
	if in == nil {
		return nil
	}
	var auth *model.LLMProviderTemplateAuth
	if in.Auth != nil {
		auth = &model.LLMProviderTemplateAuth{
			Type:        in.Auth.Type,
			Header:      in.Auth.Header,
			ValuePrefix: in.Auth.ValuePrefix,
		}
	}
	out := &model.LLMProviderTemplateMetadata{
		EndpointURL: strings.TrimSpace(in.EndpointURL),
		Auth:        auth,
		LogoURL:     strings.TrimSpace(in.LogoURL),
	}
	if out.EndpointURL == "" && out.LogoURL == "" && out.Auth == nil {
		return nil
	}
	return out
}

func mapTemplateMetadataDTO(in *model.LLMProviderTemplateMetadata) *dto.LLMProviderTemplateMetadata {
	if in == nil {
		return nil
	}
	var auth *dto.LLMProviderTemplateAuth
	if in.Auth != nil {
		auth = &dto.LLMProviderTemplateAuth{
			Type:        in.Auth.Type,
			Header:      in.Auth.Header,
			ValuePrefix: in.Auth.ValuePrefix,
		}
	}
	return &dto.LLMProviderTemplateMetadata{
		EndpointURL: in.EndpointURL,
		Auth:        auth,
		LogoURL:     in.LogoURL,
	}
}

func mapExtractionIdentifierDTO(m *model.ExtractionIdentifier) *dto.ExtractionIdentifier {
	if m == nil {
		return nil
	}
	return &dto.ExtractionIdentifier{Location: m.Location, Identifier: m.Identifier}
}

func mapProviderModelToDTO(m *model.LLMProvider) *dto.LLMProvider {
	if m == nil {
		return nil
	}
	out := &dto.LLMProvider{
		ID:           m.ID,
		Name:         m.Name,
		Description:  m.Description,
		CreatedBy:    m.CreatedBy,
		Version:      m.Version,
		Context:      m.Context,
		VHost:        m.VHost,
		Template:     m.Template,
		OpenAPI:      m.OpenAPISpec,
		RateLimiting: mapRateLimitingDTO(m.RateLimiting),
		Upstream: dto.LLMUpstream{
			URL: m.UpstreamURL,
		},
		AccessControl: dto.LLMAccessControl{Mode: "deny_all"},
		Policies:      nil,
		Security:      mapSecurityConfigDTO(m.Security),
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
	if m.UpstreamAuth != nil {
		out.Upstream.Auth = &dto.LLMUpstreamAuth{Type: m.UpstreamAuth.Type, Header: m.UpstreamAuth.Header, Value: m.UpstreamAuth.Value}
	}
	if m.AccessControl != nil {
		ac := dto.LLMAccessControl{Mode: m.AccessControl.Mode}
		if len(m.AccessControl.Exceptions) > 0 {
			ac.Exceptions = make([]dto.RouteException, 0, len(m.AccessControl.Exceptions))
			for _, e := range m.AccessControl.Exceptions {
				ac.Exceptions = append(ac.Exceptions, dto.RouteException{Path: e.Path, Methods: e.Methods})
			}
		}
		out.AccessControl = ac
	}
	if len(m.Policies) > 0 {
		out.Policies = make([]dto.LLMPolicy, 0, len(m.Policies))
		for _, p := range m.Policies {
			paths := make([]dto.LLMPolicyPath, 0, len(p.Paths))
			for _, pp := range p.Paths {
				paths = append(paths, dto.LLMPolicyPath{Path: pp.Path, Methods: pp.Methods, Params: pp.Params})
			}
			out.Policies = append(out.Policies, dto.LLMPolicy{Name: p.Name, Version: p.Version, Paths: paths})
		}
	}
	return out
}

func mapRateLimitingDTO(in *model.LLMRateLimitingConfig) *dto.LLMRateLimitingConfig {
	if in == nil {
		return nil
	}
	return &dto.LLMRateLimitingConfig{
		ProviderLevel: mapRateLimitingScopeDTO(in.ProviderLevel),
		ConsumerLevel: mapRateLimitingScopeDTO(in.ConsumerLevel),
	}
}

func mapRateLimitingScopeDTO(in *model.RateLimitingScopeConfig) *dto.RateLimitingScopeConfig {
	if in == nil {
		return nil
	}
	return &dto.RateLimitingScopeConfig{
		Global:       mapRateLimitingLimitDTO(in.Global),
		ResourceWise: mapResourceWiseRateLimitingDTO(in.ResourceWise),
	}
}

func mapRateLimitingLimitDTO(in *model.RateLimitingLimitConfig) *dto.RateLimitingLimitConfig {
	if in == nil {
		return nil
	}
	return &dto.RateLimitingLimitConfig{
		RequestCount:         in.RequestCount,
		RequestResetDuration: in.RequestResetDuration,
		RequestResetUnit:     in.RequestResetUnit,
		TokenCount:           in.TokenCount,
		TokenResetDuration:   in.TokenResetDuration,
		TokenResetUnit:       in.TokenResetUnit,
		Cost:                 in.Cost,
		CostResetDuration:    in.CostResetDuration,
		CostResetUnit:        in.CostResetUnit,
	}
}

func mapResourceWiseRateLimitingDTO(in *model.ResourceWiseRateLimitingConfig) *dto.ResourceWiseRateLimitingConfig {
	if in == nil {
		return nil
	}
	resources := make([]dto.RateLimitingResourceLimit, 0, len(in.Resources))
	for _, r := range in.Resources {
		resources = append(resources, dto.RateLimitingResourceLimit{
			Resource: r.Resource,
			Limit: dto.RateLimitingLimitConfig{
				RequestCount:         r.Limit.RequestCount,
				RequestResetDuration: r.Limit.RequestResetDuration,
				RequestResetUnit:     r.Limit.RequestResetUnit,
				TokenCount:           r.Limit.TokenCount,
				TokenResetDuration:   r.Limit.TokenResetDuration,
				TokenResetUnit:       r.Limit.TokenResetUnit,
				Cost:                 r.Limit.Cost,
				CostResetDuration:    r.Limit.CostResetDuration,
				CostResetUnit:        r.Limit.CostResetUnit,
			},
		})
	}
	return &dto.ResourceWiseRateLimitingConfig{
		Default: dto.RateLimitingLimitConfig{
			RequestCount:         in.Default.RequestCount,
			RequestResetDuration: in.Default.RequestResetDuration,
			RequestResetUnit:     in.Default.RequestResetUnit,
			TokenCount:           in.Default.TokenCount,
			TokenResetDuration:   in.Default.TokenResetDuration,
			TokenResetUnit:       in.Default.TokenResetUnit,
			Cost:                 in.Default.Cost,
			CostResetDuration:    in.Default.CostResetDuration,
			CostResetUnit:        in.Default.CostResetUnit,
		},
		Resources: resources,
	}
}

func validateRateLimitingConfig(cfg *dto.LLMRateLimitingConfig) error {
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

func validateRateLimitingScope(scope *dto.RateLimitingScopeConfig) error {
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

func validateResourceWiseRateLimiting(cfg *dto.ResourceWiseRateLimitingConfig) error {
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

func validateRateLimitingLimit(cfg *dto.RateLimitingLimitConfig) error {
	if cfg == nil {
		return constants.ErrInvalidInput
	}
	if cfg.RequestCount <= 0 || cfg.RequestResetDuration <= 0 {
		return constants.ErrInvalidInput
	}
	if !isValidResetUnit(cfg.RequestResetUnit) {
		return constants.ErrInvalidInput
	}
	if (cfg.TokenCount == nil && cfg.Cost == nil) || (cfg.TokenCount != nil && cfg.Cost != nil) {
		return constants.ErrInvalidInput
	}
	if cfg.TokenCount != nil {
		if *cfg.TokenCount <= 0 {
			return constants.ErrInvalidInput
		}
		if cfg.TokenResetDuration == nil || *cfg.TokenResetDuration <= 0 {
			return constants.ErrInvalidInput
		}
		if cfg.TokenResetUnit == nil || !isValidResetUnit(*cfg.TokenResetUnit) {
			return constants.ErrInvalidInput
		}
	}
	if cfg.Cost != nil {
		if *cfg.Cost < 0 {
			return constants.ErrInvalidInput
		}
		if cfg.CostResetDuration == nil || *cfg.CostResetDuration <= 0 {
			return constants.ErrInvalidInput
		}
		if cfg.CostResetUnit == nil || !isValidResetUnit(*cfg.CostResetUnit) {
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

func mapProxyModelToDTO(m *model.LLMProxy) *dto.LLMProxy {
	if m == nil {
		return nil
	}
	out := &dto.LLMProxy{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		CreatedBy:   m.CreatedBy,
		Version:     m.Version,
		ProjectID:   m.ProjectUUID,
		Context:     m.Context,
		VHost:       m.VHost,
		Provider:    m.Provider,
		OpenAPI:     m.OpenAPISpec,
		Security:    mapSecurityConfigDTO(m.Security),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
	if m.AccessControl != nil {
		ac := &dto.LLMAccessControl{Mode: m.AccessControl.Mode}
		if len(m.AccessControl.Exceptions) > 0 {
			ac.Exceptions = make([]dto.RouteException, 0, len(m.AccessControl.Exceptions))
			for _, e := range m.AccessControl.Exceptions {
				ac.Exceptions = append(ac.Exceptions, dto.RouteException{Path: e.Path, Methods: e.Methods})
			}
		}
		out.AccessControl = ac
	}
	if len(m.Policies) > 0 {
		out.Policies = make([]dto.LLMPolicy, 0, len(m.Policies))
		for _, p := range m.Policies {
			paths := make([]dto.LLMPolicyPath, 0, len(p.Paths))
			for _, pp := range p.Paths {
				paths = append(paths, dto.LLMPolicyPath{Path: pp.Path, Methods: pp.Methods, Params: pp.Params})
			}
			out.Policies = append(out.Policies, dto.LLMPolicy{Name: p.Name, Version: p.Version, Paths: paths})
		}
	}
	return out
}

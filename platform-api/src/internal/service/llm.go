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

func (s *LLMProviderTemplateService) Create(orgUUID string, req *dto.LLMProviderTemplate) (*dto.LLMProviderTemplate, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.ID == "" || req.DisplayName == "" {
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
		DisplayName:      req.DisplayName,
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
			DisplayName: t.DisplayName,
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
	if req.DisplayName == "" {
		return nil, constants.ErrInvalidInput
	}

	m := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               handle,
		DisplayName:      req.DisplayName,
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

func (s *LLMProviderService) Create(orgUUID string, req *dto.LLMProvider) (*dto.LLMProvider, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.ID == "" || req.DisplayName == "" || req.Version == "" || req.Template == "" {
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
		DisplayName:      req.DisplayName,
		Version:          req.Version,
		Context:          defaultString(req.Context, "/"),
		VHost:            req.VHost,
		Template:         req.Template,
		UpstreamURL:      req.Upstream.URL,
		UpstreamAuth:     mapUpstreamAuth(req.Upstream.Auth),
		OpenAPISpec:      req.OpenAPI,
		AccessControl:    mapAccessControl(&req.AccessControl),
		Policies:         mapPolicies(req.Policies),
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
			DisplayName: p.DisplayName,
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
	if req.DisplayName == "" || req.Version == "" || req.Template == "" {
		return nil, constants.ErrInvalidInput
	}
	if err := validateUpstream(req.Upstream); err != nil {
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
		DisplayName:      req.DisplayName,
		Version:          req.Version,
		Context:          defaultString(req.Context, "/"),
		VHost:            req.VHost,
		Template:         req.Template,
		UpstreamURL:      req.Upstream.URL,
		UpstreamAuth:     mapUpstreamAuth(req.Upstream.Auth),
		OpenAPISpec:      req.OpenAPI,
		AccessControl:    mapAccessControl(&req.AccessControl),
		Policies:         mapPolicies(req.Policies),
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

func (s *LLMProxyService) Create(orgUUID string, req *dto.LLMProxy) (*dto.LLMProxy, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.ID == "" || req.DisplayName == "" || req.Version == "" || req.Provider == "" || req.ProjectID == "" {
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
		DisplayName:      req.DisplayName,
		Version:          req.Version,
		Context:          defaultString(req.Context, "/"),
		VHost:            req.VHost,
		Provider:         req.Provider,
		OpenAPISpec:      req.OpenAPI,
		AccessControl:    mapAccessControl(req.AccessControl),
		Policies:         mapPolicies(req.Policies),
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
			DisplayName: p.DisplayName,
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
			DisplayName: p.DisplayName,
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
	if req.DisplayName == "" || req.Version == "" || req.Provider == "" {
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
		DisplayName:      req.DisplayName,
		Version:          req.Version,
		Context:          defaultString(req.Context, "/"),
		VHost:            req.VHost,
		Provider:         req.Provider,
		OpenAPISpec:      req.OpenAPI,
		AccessControl:    mapAccessControl(req.AccessControl),
		Policies:         mapPolicies(req.Policies),
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

func mapTemplateModelToDTO(m *model.LLMProviderTemplate) *dto.LLMProviderTemplate {
	if m == nil {
		return nil
	}
	return &dto.LLMProviderTemplate{
		ID:               m.ID,
		DisplayName:      m.DisplayName,
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
		ID:          m.ID,
		DisplayName: m.DisplayName,
		Version:     m.Version,
		Context:     m.Context,
		VHost:       m.VHost,
		Template:    m.Template,
		OpenAPI:     m.OpenAPISpec,
		Upstream: dto.LLMUpstream{
			URL: m.UpstreamURL,
		},
		AccessControl: dto.LLMAccessControl{Mode: "deny_all"},
		Policies:      nil,
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

func mapProxyModelToDTO(m *model.LLMProxy) *dto.LLMProxy {
	if m == nil {
		return nil
	}
	out := &dto.LLMProxy{
		ID:          m.ID,
		DisplayName: m.DisplayName,
		Version:     m.Version,
		ProjectID:   m.ProjectUUID,
		Context:     m.Context,
		VHost:       m.VHost,
		Provider:    m.Provider,
		OpenAPI:     m.OpenAPISpec,
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

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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
)

const (
	llmStatusPending  = "pending"
	llmStatusDeployed = "deployed"
	llmStatusFailed   = "failed"
)

type LLMProviderTemplateService struct {
	repo      repository.LLMProviderTemplateRepository
	auditRepo repository.AuditRepository
}

type LLMProviderService struct {
	repo                 repository.LLMProviderRepository
	templateRepo         repository.LLMProviderTemplateRepository
	orgRepo              repository.OrganizationRepository
	templateSeeder       *LLMTemplateSeeder
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	gatewayEventsService *GatewayEventsService
	secretService        *SecretService
	slogger              *slog.Logger
	auditRepo            repository.AuditRepository
}

type LLMProxyService struct {
	repo                 repository.LLMProxyRepository
	providerRepo         repository.LLMProviderRepository
	projectRepo          repository.ProjectRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	gatewayEventsService *GatewayEventsService
	slogger              *slog.Logger
	auditRepo            repository.AuditRepository
}

func NewLLMProviderTemplateService(repo repository.LLMProviderTemplateRepository, auditRepo repository.AuditRepository) *LLMProviderTemplateService {
	return &LLMProviderTemplateService{repo: repo, auditRepo: auditRepo}
}

func NewLLMProviderService(
	repo repository.LLMProviderRepository,
	templateRepo repository.LLMProviderTemplateRepository,
	orgRepo repository.OrganizationRepository,
	templateSeeder *LLMTemplateSeeder,
	deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository,
	gatewayEventsService *GatewayEventsService,
	slogger *slog.Logger,
	auditRepo repository.AuditRepository,
) *LLMProviderService {
	return &LLMProviderService{
		repo:                 repo,
		templateRepo:         templateRepo,
		orgRepo:              orgRepo,
		templateSeeder:       templateSeeder,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		gatewayEventsService: gatewayEventsService,
		slogger:              slogger,
		auditRepo:            auditRepo,
	}
}

// SetSecretService injects the SecretService for placeholder validation.
// Called after both services are constructed to avoid circular dependency.
func (s *LLMProviderService) SetSecretService(ss *SecretService) {
	s.secretService = ss
}

func NewLLMProxyService(
	repo repository.LLMProxyRepository,
	providerRepo repository.LLMProviderRepository,
	projectRepo repository.ProjectRepository,
	deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository,
	gatewayEventsService *GatewayEventsService,
	slogger *slog.Logger,
	auditRepo repository.AuditRepository,
) *LLMProxyService {
	return &LLMProxyService{
		repo:                 repo,
		providerRepo:         providerRepo,
		projectRepo:          projectRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		gatewayEventsService: gatewayEventsService,
		slogger:              slogger,
		auditRepo:            auditRepo,
	}
}

func (s *LLMProviderTemplateService) Create(orgUUID, createdBy string, req *api.LLMProviderTemplate) (*api.LLMProviderTemplate, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.DisplayName == "" {
		return nil, constants.ErrInvalidInput
	}

	baseHandle, err := utils.GenerateHandle(req.DisplayName, nil)
	if err != nil || baseHandle == "" {
		return nil, constants.ErrInvalidInput
	}
	version := "v1.0"
	if v := req.Version; v != "" {
		normalized, ok := normalizeTemplateVersion(v)
		if !ok || normalized != version {
			return nil, constants.ErrInvalidInput
		}
	}
	handle := makeTemplateHandle(baseHandle, version)

	if req.ManagedBy != nil && strings.TrimSpace(*req.ManagedBy) == constants.PolicyManagedByWSO2 {
		return nil, constants.ErrLLMProviderTemplateManagedByReserved
	}

	exists, err := s.repo.Exists(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check template exists: %w", err)
	}
	if exists {
		return nil, constants.ErrLLMProviderTemplateExists
	}

	m := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               handle,
		GroupID:          baseHandle,
		Version:          version,
		Name:             req.DisplayName,
		Description:      utils.ValueOrEmpty(req.Description),
		ManagedBy:        defaultTemplateManagedBy(req.ManagedBy),
		CreatedBy:        createdBy,
		OpenAPISpec:      utils.ValueOrEmpty(req.Openapi),
		Metadata:         mapTemplateMetadataAPI(req.Metadata),
		PromptTokens:     mapExtractionIdentifierAPI(req.PromptTokens),
		CompletionTokens: mapExtractionIdentifierAPI(req.CompletionTokens),
		TotalTokens:      mapExtractionIdentifierAPI(req.TotalTokens),
		RemainingTokens:  mapExtractionIdentifierAPI(req.RemainingTokens),
		RequestModel:     mapExtractionIdentifierAPI(req.RequestModel),
		ResponseModel:    mapExtractionIdentifierAPI(req.ResponseModel),
		Origin:           constants.OriginCP,
	}
	resourceMappings, err := mapTemplateResourceMappingsAPI(req.ResourceMappings)
	if err != nil {
		return nil, err
	}
	m.ResourceMappings = resourceMappings

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrLLMProviderTemplateExists
		}
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	_ = s.auditRepo.Record("CREATE", m.UUID, "llm_provider_template", orgUUID, createdBy)

	return mapTemplateModelToAPI(m), nil
}

func (s *LLMProviderTemplateService) List(orgUUID string, limit, offset int, allVersions bool) (*api.LLMProviderTemplateListResponse, error) {
	listFn := s.repo.List
	countFn := s.repo.Count
	if allVersions {
		listFn = s.repo.ListAllVersions
		countFn = s.repo.CountAllVersions
	}
	items, err := listFn(orgUUID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	totalCount, err := countFn(orgUUID)
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
		resp.List = append(resp.List, templateListItem(t))
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

func (s *LLMProviderTemplateService) Update(orgUUID, handle, updatedBy string, req *api.LLMProviderTemplate) (*api.LLMProviderTemplate, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id != nil && *req.Id != "" && *req.Id != handle {
		return nil, constants.ErrHandleImmutable
	}
	if req.DisplayName == "" {
		return nil, constants.ErrInvalidInput
	}
	if req.ManagedBy != nil && strings.TrimSpace(*req.ManagedBy) == constants.PolicyManagedByWSO2 {
		return nil, constants.ErrLLMProviderTemplateManagedByReserved
	}

	existing, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve template: %w", err)
	}
	if existing == nil {
		return nil, constants.ErrLLMProviderTemplateNotFound
	}
	if existing.ManagedBy == "wso2" {
		return nil, constants.ErrLLMProviderTemplateReadOnly
	}
	if err := ensureOriginMutable(existing.Origin); err != nil {
		return nil, err
	}
	// In-place update never changes the version; a new version is created via
	// POST /llm-provider-templates/{id}/versions. Reject a request that tries to change it
	// rather than silently ignoring the supplied value.
	if req.Version != "" && req.Version != existing.Version {
		return nil, fmt.Errorf("%w: template version cannot be changed via update; use the versions endpoint", constants.ErrInvalidInput)
	}

	managedBy := existing.ManagedBy
	if req.ManagedBy != nil {
		managedBy = defaultTemplateManagedBy(req.ManagedBy)
	}
	openapiSpec := existing.OpenAPISpec
	if req.Openapi != nil {
		openapiSpec = utils.ValueOrEmpty(req.Openapi)
	}

	m := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               handle,
		Name:             req.DisplayName,
		Description:      utils.ValueOrEmpty(req.Description),
		UpdatedBy:        updatedBy,
		ManagedBy:        managedBy,
		OpenAPISpec:      openapiSpec,
		Metadata:         mapTemplateMetadataAPI(req.Metadata),
		PromptTokens:     mapExtractionIdentifierAPI(req.PromptTokens),
		CompletionTokens: mapExtractionIdentifierAPI(req.CompletionTokens),
		TotalTokens:      mapExtractionIdentifierAPI(req.TotalTokens),
		RemainingTokens:  mapExtractionIdentifierAPI(req.RemainingTokens),
		RequestModel:     mapExtractionIdentifierAPI(req.RequestModel),
		ResponseModel:    mapExtractionIdentifierAPI(req.ResponseModel),
	}
	resourceMappings, err := mapTemplateResourceMappingsAPI(req.ResourceMappings)
	if err != nil {
		return nil, err
	}
	m.ResourceMappings = resourceMappings

	if err := s.repo.Update(m); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrLLMProviderTemplateNotFound
		}
		return nil, fmt.Errorf("failed to update template: %w", err)
	}

	base, baseErr := s.repo.GetGroupID(handle, orgUUID)
	if baseErr != nil {
		return nil, fmt.Errorf("failed to resolve template family: %w", baseErr)
	}
	if base != "" {
		if err := s.repo.RenameFamily(base, orgUUID, req.DisplayName); err != nil {
			return nil, fmt.Errorf("failed to propagate template name: %w", err)
		}
	}

	updated, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated template: %w", err)
	}
	if updated == nil {
		return nil, constants.ErrLLMProviderTemplateNotFound
	}

	_ = s.auditRepo.Record("UPDATE", updated.UUID, "llm_provider_template", orgUUID, updatedBy)

	return mapTemplateModelToAPI(updated), nil
}

var templateVersionPattern = regexp.MustCompile(`^[vV]\d+\.\d+$`)

func normalizeTemplateVersion(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if !templateVersionPattern.MatchString(v) {
		return "", false
	}
	return "v" + strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V"), true
}
func makeTemplateHandle(baseHandle, version string) string {
	return baseHandle + "-" + strings.ReplaceAll(strings.ToLower(strings.TrimSpace(version)), ".", "-")
}

func (s *LLMProviderTemplateService) CreateVersion(orgUUID, handle, createdBy string, req *api.CreateLLMProviderTemplateVersionRequest) (*api.LLMProviderTemplate, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.DisplayName == "" {
		return nil, constants.ErrInvalidInput
	}
	version, ok := normalizeTemplateVersion(req.Version)
	if !ok {
		return nil, constants.ErrInvalidInput
	}

	managedBy := defaultTemplateManagedBy(req.ManagedBy)
	if managedBy == constants.PolicyManagedByWSO2 {
		managedBy = constants.PolicyManagedByCustomer
	}

	baseHandle, err := s.repo.GetGroupID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve template family: %w", err)
	}
	if baseHandle == "" {
		return nil, constants.ErrLLMProviderTemplateNotFound
	}

	m := &model.LLMProviderTemplate{
		OrganizationUUID: orgUUID,
		ID:               makeTemplateHandle(baseHandle, version),
		GroupID:          baseHandle,
		Name:             req.DisplayName,
		Description:      utils.ValueOrEmpty(req.Description),
		ManagedBy:        managedBy,
		CreatedBy:        createdBy,
		Version:          version,
		OpenAPISpec:      utils.ValueOrEmpty(req.Openapi),
		Metadata:         mapTemplateMetadataAPI(req.Metadata),
		PromptTokens:     mapExtractionIdentifierAPI(req.PromptTokens),
		CompletionTokens: mapExtractionIdentifierAPI(req.CompletionTokens),
		TotalTokens:      mapExtractionIdentifierAPI(req.TotalTokens),
		RemainingTokens:  mapExtractionIdentifierAPI(req.RemainingTokens),
		RequestModel:     mapExtractionIdentifierAPI(req.RequestModel),
		ResponseModel:    mapExtractionIdentifierAPI(req.ResponseModel),
		Origin:           constants.OriginCP,
	}
	resourceMappings, err := mapTemplateResourceMappingsAPI(req.ResourceMappings)
	if err != nil {
		return nil, err
	}
	m.ResourceMappings = resourceMappings

	if err := s.repo.CreateNewVersion(m); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, constants.ErrLLMProviderTemplateNotFound
		case errors.Is(err, constants.ErrLLMProviderTemplateVersionExists):
			return nil, constants.ErrLLMProviderTemplateVersionExists
		default:
			return nil, fmt.Errorf("failed to create new template version: %w", err)
		}
	}

	return mapTemplateModelToAPI(m), nil
}

func (s *LLMProviderTemplateService) ListVersions(orgUUID, handle string, limit, offset int) (*api.LLMProviderTemplateListResponse, error) {
	if handle == "" {
		return nil, constants.ErrInvalidInput
	}
	total, err := s.repo.CountVersions(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count template versions: %w", err)
	}
	if total == 0 {
		return nil, constants.ErrLLMProviderTemplateNotFound
	}
	items, err := s.repo.ListVersions(handle, orgUUID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list template versions: %w", err)
	}
	resp := &api.LLMProviderTemplateListResponse{
		Count: len(items),
		Pagination: api.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  total,
		},
	}
	resp.List = make([]api.LLMProviderTemplateListItem, 0, len(items))
	for _, t := range items {
		resp.List = append(resp.List, templateListItem(t))
	}
	return resp, nil
}

func (s *LLMProviderTemplateService) GetVersion(orgUUID, handle, version string) (*api.LLMProviderTemplate, error) {
	v := strings.TrimSpace(version)
	if handle == "" || v == "" {
		return nil, constants.ErrInvalidInput
	}
	normalized, ok := normalizeTemplateVersion(v)
	if !ok {
		return nil, constants.ErrInvalidInput
	}
	v = normalized
	m, err := s.repo.GetByVersion(handle, orgUUID, v)
	if err != nil {
		return nil, fmt.Errorf("failed to get template version: %w", err)
	}
	if m == nil {
		return nil, constants.ErrLLMProviderTemplateNotFound
	}
	return mapTemplateModelToAPI(m), nil
}

// SetVersionEnabled enables or disables a specific version of a template.
// Disabling is blocked when any provider was created from this specific version.
func (s *LLMProviderTemplateService) SetVersionEnabled(orgUUID, handle, version string, enabled bool) (*api.LLMProviderTemplate, error) {
	v := strings.TrimSpace(version)
	if handle == "" || v == "" {
		return nil, constants.ErrInvalidInput
	}
	normalized, ok := normalizeTemplateVersion(v)
	if !ok {
		return nil, constants.ErrInvalidInput
	}
	v = normalized
	if !enabled {
		inUse, err := s.repo.CountProvidersUsingTemplate(handle, orgUUID, v)
		if err != nil {
			return nil, fmt.Errorf("failed to check template version usage: %w", err)
		}
		if inUse > 0 {
			return nil, constants.ErrLLMProviderTemplateInUse
		}
	}
	// Read-only versions (built-in 'wso2'-managed or DP-imported) cannot be toggled, matching
	// the guard applied by Update/Delete/DeleteVersion.
	target, err := s.repo.GetByVersion(handle, orgUUID, v)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve template version: %w", err)
	}
	if target == nil {
		return nil, constants.ErrLLMProviderTemplateNotFound
	}
	if target.ManagedBy == "wso2" {
		return nil, constants.ErrLLMProviderTemplateReadOnly
	}
	if err := ensureOriginMutable(target.Origin); err != nil {
		return nil, err
	}
	if err := s.repo.SetEnabled(handle, orgUUID, v, enabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrLLMProviderTemplateNotFound
		}
		return nil, fmt.Errorf("failed to set template version enabled: %w", err)
	}
	m, err := s.repo.GetByVersion(handle, orgUUID, v)
	if err != nil {
		return nil, fmt.Errorf("failed to reload template version: %w", err)
	}
	if m == nil {
		return nil, constants.ErrLLMProviderTemplateNotFound
	}
	return mapTemplateModelToAPI(m), nil
}

func (s *LLMProviderTemplateService) Delete(orgUUID, handle, deletedBy string) error {
	if handle == "" {
		return constants.ErrInvalidInput
	}
	tpl, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to resolve template: %w", err)
	}
	if tpl == nil {
		return constants.ErrLLMProviderTemplateNotFound
	}
	if tpl.ManagedBy == "wso2" {
		return constants.ErrLLMProviderTemplateReadOnly
	}
	if err := ensureOriginMutable(tpl.Origin); err != nil {
		return err
	}
	// Block deletion while any provider (built from any version) still depends on it.
	inUse, err := s.repo.CountProvidersUsingTemplate(handle, orgUUID, "")
	if err != nil {
		return fmt.Errorf("failed to check template usage: %w", err)
	}
	if inUse > 0 {
		return constants.ErrLLMProviderTemplateInUse
	}
	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrLLMProviderTemplateNotFound
		}
		return fmt.Errorf("failed to delete template: %w", err)
	}
	// Family-level delete: log the stable handle rather than a single version's UUID.
	_ = s.auditRepo.Record("DELETE", handle, "llm_provider_template", orgUUID, deletedBy)
	return nil
}

func (s *LLMProviderTemplateService) DeleteVersion(orgUUID, handle, version string) error {
	v := strings.TrimSpace(version)
	if handle == "" || v == "" {
		return constants.ErrInvalidInput
	}
	normalized, ok := normalizeTemplateVersion(v)
	if !ok {
		return constants.ErrInvalidInput
	}
	v = normalized
	target, err := s.repo.GetByVersion(handle, orgUUID, v)
	if err != nil {
		return fmt.Errorf("failed to resolve template version: %w", err)
	}
	if target == nil {
		return constants.ErrLLMProviderTemplateNotFound
	}
	if target.ManagedBy == "wso2" {
		return constants.ErrLLMProviderTemplateReadOnly
	}
	if err := ensureOriginMutable(target.Origin); err != nil {
		return err
	}
	// Block deletion while any provider built from this specific version still depends on it.
	inUse, err := s.repo.CountProvidersUsingTemplate(handle, orgUUID, v)
	if err != nil {
		return fmt.Errorf("failed to check template version usage: %w", err)
	}
	if inUse > 0 {
		return constants.ErrLLMProviderTemplateInUse
	}
	if err := s.repo.DeleteVersion(handle, orgUUID, v); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrLLMProviderTemplateNotFound
		}
		return fmt.Errorf("failed to delete template version: %w", err)
	}
	return nil
}

func (s *LLMProviderService) Create(orgUUID, createdBy string, req *api.LLMProvider) (*api.LLMProvider, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.DisplayName == "" || req.Version == "" || req.Template == "" {
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

	// Determine handle: use provided id or auto-generate from displayName
	var handle string
	if req.Id != nil && *req.Id != "" {
		handle = *req.Id
		exists, err := s.repo.Exists(handle, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to check provider exists: %w", err)
		}
		if exists {
			return nil, constants.ErrLLMProviderExists
		}
	} else {
		var err error
		handle, err = utils.GenerateHandle(req.DisplayName, func(h string) bool {
			exists, _ := s.repo.Exists(h, orgUUID)
			return exists
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate provider handle: %w", err)
		}
	}
	req.Id = &handle

	// Validate {{ secret "..." }} placeholders in the upstream config
	if s.secretService != nil {
		configJSON, err := marshalUpstreamForValidation(req.Upstream)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal upstream config for secret validation: %w", err)
		}
		if err := s.secretService.ValidateSecretRefs(orgUUID, configJSON); err != nil {
			return nil, err
		}
	}

	providerCount, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count providers: %w", err)
	}
	if err := validateLLMResourceLimit(providerCount, constants.MaxLLMProvidersPerOrganization, constants.ErrLLMProviderLimitReached); err != nil {
		return nil, err
	}
	if !tpl.Enabled {
		return nil, constants.ErrInvalidInput
	}

	openapiSpec := utils.ValueOrEmpty(req.Openapi)
	if openapiSpec == "" {
		openapiSpec = tpl.OpenAPISpec
	}

	contextValue := utils.DefaultStringPtr(req.Context, "/")
	m := &model.LLMProvider{
		OrganizationUUID: orgUUID,
		ID:               handle,
		Name:             req.DisplayName,
		Description:      utils.ValueOrEmpty(req.Description),
		CreatedBy:        createdBy,
		Version:          req.Version,
		TemplateUUID:     tpl.UUID,
		OpenAPISpec:      openapiSpec,
		ModelProviders:   mapModelProvidersAPI(req.ModelProviders),
		Configuration: model.LLMProviderConfig{
			Context:           &contextValue,
			VHost:             req.Vhost,
			Upstream:          mapUpstreamAPIToModel(req.Upstream),
			AccessControl:     mapAccessControlAPI(&req.AccessControl),
			RateLimiting:      mapRateLimitingAPIToModel(req.RateLimiting),
			GlobalPolicies:    mapGlobalPoliciesAPIToModel(req.GlobalPolicies),
			OperationPolicies: mapOperationPoliciesAPIToModel(req.OperationPolicies),
			Policies:          mapPoliciesAPIToModel(req.Policies),
			Security:          mapSecurityAPIToModel(req.Security),
		},
		Origin: constants.OriginCP,
	}
	migrateLegacyProviderPoliciesInPlace(&m.Configuration)

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrLLMProviderExists
		}
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	created, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created provider: %w", err)
	}
	if created == nil {
		return nil, constants.ErrLLMProviderNotFound
	}

	_ = s.auditRepo.Record("CREATE", created.UUID, "llm_provider", orgUUID, createdBy)

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
		desc := utils.StringPtrIfNotEmpty(p.Description)
		createdBy := utils.StringPtrIfNotEmpty(p.CreatedBy)
		version := p.Version
		template := utils.StringPtrIfNotEmpty(tplHandle)
		resp.List = append(resp.List, api.LLMProviderListItem{
			Id:          &id,
			DisplayName: name,
			Description: desc,
			CreatedBy:   createdBy,
			Version:     &version,
			Template:    template,
			ReadOnly:    utils.BoolPtr(p.Origin == constants.OriginDP),
			CreatedAt:   utils.TimePtr(p.CreatedAt),
			UpdatedAt:   utils.TimePtr(p.UpdatedAt),
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

func (s *LLMProviderService) Update(orgUUID, handle, updatedBy string, req *api.LLMProvider) (*api.LLMProvider, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id != nil && *req.Id != "" && *req.Id != handle {
		return nil, constants.ErrHandleImmutable
	}
	// Fetch existing provider to preserve sensitive fields on update
	existing, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch existing provider: %w", err)
	}
	if existing == nil {
		return nil, constants.ErrLLMProviderNotFound
	}
	// DP-originated artifacts are read-only in the control plane.
	if err := ensureOriginMutable(existing.Origin); err != nil {
		return nil, err
	}
	if req.DisplayName == "" || req.Version == "" || req.Template == "" {
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

	// Validate {{ secret "..." }} placeholders in the upstream config
	if s.secretService != nil {
		configJSON, err := marshalUpstreamForValidation(req.Upstream)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal upstream config for secret validation: %w", err)
		}
		if err := s.secretService.ValidateSecretRefs(orgUUID, configJSON); err != nil {
			return nil, err
		}
	}

	contextValue := utils.DefaultStringPtr(req.Context, "/")
	m := &model.LLMProvider{
		OrganizationUUID: orgUUID,
		ID:               handle,
		Name:             req.DisplayName,
		Description:      utils.ValueOrEmpty(req.Description),
		UpdatedBy:        updatedBy,
		Version:          req.Version,
		TemplateUUID:     tpl.UUID,
		OpenAPISpec:      utils.ValueOrEmpty(req.Openapi),
		ModelProviders:   mapModelProvidersAPI(req.ModelProviders),
		Configuration: model.LLMProviderConfig{
			Context:           &contextValue,
			VHost:             req.Vhost,
			Upstream:          mapUpstreamAPIToModel(req.Upstream),
			AccessControl:     mapAccessControlAPI(&req.AccessControl),
			RateLimiting:      mapRateLimitingAPIToModel(req.RateLimiting),
			GlobalPolicies:    mapGlobalPoliciesAPIToModel(req.GlobalPolicies),
			OperationPolicies: mapOperationPoliciesAPIToModel(req.OperationPolicies),
			Policies:          mapPoliciesAPIToModel(req.Policies),
			Security:          mapSecurityAPIToModel(req.Security),
		},
	}
	migrateLegacyProviderPoliciesInPlace(&m.Configuration)

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

	_ = s.auditRepo.Record("UPDATE", updated.UUID, "llm_provider", orgUUID, updatedBy)

	return mapProviderModelToAPI(updated, tpl.ID), nil
}

func (s *LLMProviderService) Delete(orgUUID, handle, deletedBy string) error {
	if handle == "" {
		return constants.ErrInvalidInput
	}

	// Get the provider UUID before deletion (needed for deployment lookup)
	provider, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to get LLM provider: %w", err)
	}
	if provider == nil {
		return constants.ErrLLMProviderNotFound
	}

	// DP-originated artifacts may only be deleted once undeployed on all gateways.
	if err := ensureOriginDeletable(s.deploymentRepo, provider.Origin, provider.UUID, orgUUID); err != nil {
		return err
	}

	// Get all gateways in the organization to broadcast deletion event.
	// We broadcast to all gateways (not just those with active deployments) because
	// deployment_status rows may have been cascade-deleted when deployments were removed,
	// leaving stale artifacts on gateways that would otherwise never receive the delete event.
	var gateways []*model.Gateway
	if s.gatewayRepo != nil {
		gws, err := s.gatewayRepo.GetByOrganizationID(orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to get gateways for LLM provider deletion", "error", err, "providerUUID", provider.UUID)
		} else {
			gateways = gws
		}
	}

	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrLLMProviderNotFound
		}
		return fmt.Errorf("failed to delete provider: %w", err)
	}

	_ = s.auditRepo.Record("DELETE", provider.UUID, "llm_provider", orgUUID, deletedBy)

	// Send deletion events to all gateways in the organization
	if s.gatewayEventsService != nil && len(gateways) > 0 {
		for _, gateway := range gateways {
			deletionEvent := &model.LLMProviderDeletionEvent{
				ProviderId: provider.UUID,
			}

			if err := s.gatewayEventsService.BroadcastLLMProviderDeletionEvent(gateway.ID, deletionEvent); err != nil {
				s.slogger.Warn("Failed to broadcast LLM provider deletion event", "error", err, "gatewayID", gateway.ID, "providerUUID", provider.UUID)
			} else {
				s.slogger.Info("LLM provider deletion event sent", "gatewayID", gateway.ID, "providerUUID", provider.UUID)
			}
		}
	}

	return nil
}

func (s *LLMProxyService) Create(orgUUID, createdBy string, req *api.LLMProxy) (*api.LLMProxy, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.DisplayName == "" || req.Version == "" || req.Provider.Id == "" || req.ProjectId == "" {
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

	// Determine handle: use provided id or auto-generate from displayName
	var handle string
	if req.Id != nil && *req.Id != "" {
		handle = *req.Id
		exists, err := s.repo.Exists(handle, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to check proxy exists: %w", err)
		}
		if exists {
			return nil, constants.ErrLLMProxyExists
		}
	} else {
		var err error
		handle, err = utils.GenerateHandle(req.DisplayName, func(h string) bool {
			exists, _ := s.repo.Exists(h, orgUUID)
			return exists
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate proxy handle: %w", err)
		}
	}
	req.Id = &handle

	proxyCount, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count proxies: %w", err)
	}
	if err := validateLLMResourceLimit(proxyCount, constants.MaxLLMProxiesPerOrganization, constants.ErrLLMProxyLimitReached); err != nil {
		return nil, err
	}

	contextValue := utils.DefaultStringPtr(req.Context, "/")
	m := &model.LLMProxy{
		OrganizationUUID: orgUUID,
		ProjectUUID:      req.ProjectId,
		ID:               handle,
		Name:             req.DisplayName,
		Description:      utils.ValueOrEmpty(req.Description),
		CreatedBy:        createdBy,
		Version:          req.Version,
		ProviderUUID:     prov.UUID,
		OpenAPISpec:      utils.ValueOrEmpty(req.Openapi),
		Configuration: model.LLMProxyConfig{
			Context:           &contextValue,
			Vhost:             req.Vhost,
			Provider:          req.Provider.Id,
			UpstreamAuth:      mapUpstreamAuthAPIToModel(req.Provider.Auth),
			GlobalPolicies:    mapGlobalPoliciesAPIToModel(req.GlobalPolicies),
			OperationPolicies: mapOperationPoliciesAPIToModel(req.OperationPolicies),
			Policies:          mapPoliciesAPIToModel(req.Policies),
			Security:          mapSecurityAPIToModel(req.Security),
		},
		Origin: constants.OriginCP,
	}
	migrateLegacyProxyPoliciesInPlace(&m.Configuration)

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrLLMProxyExists
		}
		return nil, fmt.Errorf("failed to create proxy: %w", err)
	}

	_ = s.auditRepo.Record("CREATE", m.UUID, "llm_proxy", orgUUID, createdBy)
	created, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created proxy: %w", err)
	}
	if created == nil {
		return nil, constants.ErrLLMProxyNotFound
	}
	return mapProxyModelToAPI(created), nil
}

func (s *LLMProxyService) List(orgUUID string, projectHandle *string, limit, offset int) (*api.LLMProxyListResponse, error) {
	var resolvedProjectUUID *string
	if projectHandle != nil && *projectHandle != "" {
		if s.projectRepo == nil {
			return nil, constants.ErrProjectNotFound
		}
		project, err := s.projectRepo.GetProjectByHandleAndOrgID(*projectHandle, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate project: %w", err)
		}
		if project == nil || project.OrganizationID != orgUUID {
			return nil, constants.ErrProjectNotFound
		}
		resolvedProjectUUID = &project.ID
	}

	var items []*model.LLMProxy
	var err error
	if resolvedProjectUUID != nil {
		items, err = s.repo.ListByProject(orgUUID, *resolvedProjectUUID, limit, offset)
	} else {
		items, err = s.repo.List(orgUUID, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list proxies: %w", err)
	}
	var totalCount int
	if resolvedProjectUUID != nil {
		totalCount, err = s.repo.CountByProject(orgUUID, *resolvedProjectUUID)
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
		desc := utils.StringPtrIfNotEmpty(p.Description)
		createdBy := utils.StringPtrIfNotEmpty(p.CreatedBy)
		contextValue := (*string)(nil)
		if p.Configuration.Context != nil {
			v := *p.Configuration.Context
			contextValue = &v
		}
		version := p.Version
		projectID := p.ProjectUUID
		provider := p.Configuration.Provider
		resp.List = append(resp.List, api.LLMProxyListItem{
			Id:          &id,
			DisplayName: name,
			Description: desc,
			CreatedBy:   createdBy,
			Context:     contextValue,
			Version:     &version,
			ProjectId:   &projectID,
			Provider:    &provider,
			ReadOnly:    utils.BoolPtr(p.Origin == constants.OriginDP),
			CreatedAt:   utils.TimePtr(p.CreatedAt),
			UpdatedAt:   utils.TimePtr(p.UpdatedAt),
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
		desc := utils.StringPtrIfNotEmpty(p.Description)
		createdBy := utils.StringPtrIfNotEmpty(p.CreatedBy)
		contextValue := (*string)(nil)
		if p.Configuration.Context != nil {
			v := *p.Configuration.Context
			contextValue = &v
		}
		version := p.Version
		projectID := p.ProjectUUID
		provider := p.Configuration.Provider
		resp.List = append(resp.List, api.LLMProxyListItem{
			Id:          &id,
			DisplayName: name,
			Description: desc,
			CreatedBy:   createdBy,
			Context:     contextValue,
			Version:     &version,
			ProjectId:   &projectID,
			Provider:    &provider,
			ReadOnly:    utils.BoolPtr(p.Origin == constants.OriginDP),
			CreatedAt:   utils.TimePtr(p.CreatedAt),
			UpdatedAt:   utils.TimePtr(p.UpdatedAt),
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

func (s *LLMProxyService) Update(orgUUID, handle, updatedBy string, req *api.LLMProxy) (*api.LLMProxy, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id != nil && *req.Id != "" && *req.Id != handle {
		return nil, constants.ErrHandleImmutable
	}
	if req.DisplayName == "" || req.Version == "" || req.Provider.Id == "" {
		return nil, constants.ErrInvalidInput
	}

	existing, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing proxy: %w", err)
	}
	if existing == nil {
		return nil, constants.ErrLLMProxyNotFound
	}
	// DP-originated artifacts are read-only in the control plane.
	if err := ensureOriginMutable(existing.Origin); err != nil {
		return nil, err
	}

	// Validate provider exists
	prov, err := s.providerRepo.GetByID(req.Provider.Id, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate provider: %w", err)
	}
	if prov == nil {
		return nil, constants.ErrLLMProviderNotFound
	}

	contextValue := utils.DefaultStringPtr(req.Context, "/")
	m := &model.LLMProxy{
		OrganizationUUID: orgUUID,
		ID:               handle,
		Name:             req.DisplayName,
		Description:      utils.ValueOrEmpty(req.Description),
		UpdatedBy:        updatedBy,
		Version:          req.Version,
		ProviderUUID:     prov.UUID,
		OpenAPISpec:      utils.ValueOrEmpty(req.Openapi),
		Configuration: model.LLMProxyConfig{
			Context:           &contextValue,
			Vhost:             req.Vhost,
			Provider:          req.Provider.Id,
			UpstreamAuth:      mapUpstreamAuthAPIToModel(req.Provider.Auth),
			GlobalPolicies:    mapGlobalPoliciesAPIToModel(req.GlobalPolicies),
			OperationPolicies: mapOperationPoliciesAPIToModel(req.OperationPolicies),
			Policies:          mapPoliciesAPIToModel(req.Policies),
			Security:          mapSecurityAPIToModel(req.Security),
		},
	}
	migrateLegacyProxyPoliciesInPlace(&m.Configuration)

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
	_ = s.auditRepo.Record("UPDATE", existing.UUID, "llm_proxy", orgUUID, updatedBy)
	return mapProxyModelToAPI(updated), nil
}

func (s *LLMProxyService) Delete(orgUUID, handle, deletedBy string) error {
	if handle == "" {
		return constants.ErrInvalidInput
	}

	// Get the proxy UUID before deletion (needed for deployment lookup)
	proxy, err := s.repo.GetByID(handle, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to get LLM proxy: %w", err)
	}
	if proxy == nil {
		return constants.ErrLLMProxyNotFound
	}

	// DP-originated artifacts may only be deleted once undeployed on all gateways.
	if err := ensureOriginDeletable(s.deploymentRepo, proxy.Origin, proxy.UUID, orgUUID); err != nil {
		return err
	}

	// Get all gateways in the organization to broadcast deletion event.
	// We broadcast to all gateways (not just those with active deployments) because
	// deployment_status rows may have been cascade-deleted when deployments were removed,
	// leaving stale artifacts on gateways that would otherwise never receive the delete event.
	var gateways []*model.Gateway
	if s.gatewayRepo != nil {
		gws, err := s.gatewayRepo.GetByOrganizationID(orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to get gateways for LLM proxy deletion", "error", err, "proxyUUID", proxy.UUID)
		} else {
			gateways = gws
		}
	}

	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrLLMProxyNotFound
		}
		return fmt.Errorf("failed to delete proxy: %w", err)
	}

	_ = s.auditRepo.Record("DELETE", proxy.UUID, "llm_proxy", orgUUID, deletedBy)
	// Send deletion events to all gateways in the organization
	if s.gatewayEventsService != nil && len(gateways) > 0 {
		for _, gateway := range gateways {
			deletionEvent := &model.LLMProxyDeletionEvent{
				ProxyId: proxy.UUID,
			}

			if err := s.gatewayEventsService.BroadcastLLMProxyDeletionEvent(gateway.ID, deletionEvent); err != nil {
				s.slogger.Warn("Failed to broadcast LLM proxy deletion event", "error", err, "gatewayID", gateway.ID, "proxyUUID", proxy.UUID)
			} else {
				s.slogger.Info("LLM proxy deletion event sent", "gatewayID", gateway.ID, "proxyUUID", proxy.UUID)
			}
		}
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
	mainUrl := utils.ValueOrEmpty(u.Main.Url)
	mainRef := utils.ValueOrEmpty(u.Main.Ref)
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

// mapGlobalPoliciesAPIToLLMPolicies flattens global (api-level) policies into the legacy
// model.LLMPolicy shape used by liftLLMPolicies. Global policies carry their params at the
// policy level (no path); the CP->DP forward conversion emits api-key-auth security and
// api-level (global) rate limits here. They are wrapped into a synthetic "/*" path so the
// shared lift logic — which reads params off paths and treats "/*" as the global scope —
// reconstructs them uniformly.
func mapGlobalPoliciesAPIToLLMPolicies(in *[]api.Policy) []model.LLMPolicy {
	if in == nil || len(*in) == 0 {
		return nil
	}
	out := make([]model.LLMPolicy, 0, len(*in))
	for _, p := range *in {
		var params map[string]interface{}
		if p.Params != nil {
			params = *p.Params
		}
		out = append(out, model.LLMPolicy{
			Name:    p.Name,
			Version: p.Version,
			Paths:   []model.LLMPolicyPath{{Path: "/*", Methods: []string{"*"}, Params: params}},
		})
	}
	return out
}

// mapOperationPoliciesAPIToLLMPolicies flattens operation policies into the legacy
// model.LLMPolicy shape used by liftLLMPolicies. The CP->DP forward conversion promotes
// the resource-scoped rate-limit policies it assembles into operationPolicies (see
// generateLLMProviderDeploymentYAML), so on DP->CP import they are read back from there.
func mapOperationPoliciesAPIToLLMPolicies(in *[]api.OperationPolicy) []model.LLMPolicy {
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

func mapGlobalPoliciesAPIToModel(in *[]api.Policy) []model.GlobalPolicy {
	if in == nil || len(*in) == 0 {
		return nil
	}
	out := make([]model.GlobalPolicy, 0, len(*in))
	for _, p := range *in {
		ec := ""
		if p.ExecutionCondition != nil {
			ec = *p.ExecutionCondition
		}
		var params map[string]interface{}
		if p.Params != nil {
			params = *p.Params
		}
		out = append(out, model.GlobalPolicy{Name: p.Name, Version: p.Version, ExecutionCondition: ec, Params: params})
	}
	return out
}

func mapOperationPoliciesAPIToModel(in *[]api.OperationPolicy) []model.OperationPolicy {
	if in == nil || len(*in) == 0 {
		return nil
	}
	out := make([]model.OperationPolicy, 0, len(*in))
	for _, p := range *in {
		ec := ""
		if p.ExecutionCondition != nil {
			ec = *p.ExecutionCondition
		}
		paths := make([]model.OperationPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			methods := make([]string, 0, len(pp.Methods))
			for _, mm := range pp.Methods {
				methods = append(methods, string(mm))
			}
			paths = append(paths, model.OperationPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		out = append(out, model.OperationPolicy{Name: p.Name, Version: p.Version, ExecutionCondition: ec, Paths: paths})
	}
	return out
}

func mapGlobalPoliciesModelToAPI(in []model.GlobalPolicy) *[]api.Policy {
	if len(in) == 0 {
		return nil
	}
	out := make([]api.Policy, 0, len(in))
	for _, p := range in {
		entry := api.Policy{Name: p.Name, Version: p.Version}
		if p.ExecutionCondition != "" {
			entry.ExecutionCondition = &p.ExecutionCondition
		}
		if p.Params != nil {
			params := p.Params
			entry.Params = &params
		}
		out = append(out, entry)
	}
	return &out
}

func mapOperationPoliciesModelToAPI(in []model.OperationPolicy) *[]api.OperationPolicy {
	if len(in) == 0 {
		return nil
	}
	out := make([]api.OperationPolicy, 0, len(in))
	for _, p := range in {
		paths := make([]api.OperationPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			methods := make([]api.OperationPolicyPathMethods, 0, len(pp.Methods))
			for _, mm := range pp.Methods {
				methods = append(methods, api.OperationPolicyPathMethods(mm))
			}
			paths = append(paths, api.OperationPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		entry := api.OperationPolicy{Name: p.Name, Version: p.Version, Paths: paths}
		if p.ExecutionCondition != "" {
			entry.ExecutionCondition = &p.ExecutionCondition
		}
		out = append(out, entry)
	}
	return &out
}

// migrateLegacyProviderPoliciesInPlace folds any legacy `policies` entries into
// globalPolicies / operationPolicies, then clears `policies`.
// Rules:
//   - a path entry with path == "/*" AND methods == ["*"] → GlobalPolicy (deduped by name)
//   - any other path entry                                → OperationPolicy path (merged by name)
//
// Empty or nil Policies → no-op.
func migrateLegacyProviderPoliciesInPlace(cfg *model.LLMProviderConfig) {
	migrateLegacyPolicies(&cfg.GlobalPolicies, &cfg.OperationPolicies, cfg.Policies)
	cfg.Policies = nil
}

// migrateLegacyProxyPoliciesInPlace is the proxy-config counterpart.
func migrateLegacyProxyPoliciesInPlace(cfg *model.LLMProxyConfig) {
	migrateLegacyPolicies(&cfg.GlobalPolicies, &cfg.OperationPolicies, cfg.Policies)
	cfg.Policies = nil
}

// migrateLegacyPolicies is the shared migration kernel.
func migrateLegacyPolicies(globalPolicies *[]model.GlobalPolicy, operationPolicies *[]model.OperationPolicy, legacyPolicies []model.LLMPolicy) {
	for _, p := range legacyPolicies {
		for _, pe := range p.Paths {
			if pe.Path == "/*" && isWildcardOnlyMethods(pe.Methods) {
				if !hasGlobalPolicyByName(*globalPolicies, p.Name) {
					*globalPolicies = append(*globalPolicies, model.GlobalPolicy{
						Name:    p.Name,
						Version: p.Version,
						Params:  pe.Params,
					})
				}
			} else {
				appendLegacyOperationPath(operationPolicies, p.Name, p.Version, model.OperationPolicyPath{
					Path:    pe.Path,
					Methods: pe.Methods,
					Params:  pe.Params,
				})
			}
		}
	}
}

// isWildcardOnlyMethods reports whether methods is exactly ["*"].
func isWildcardOnlyMethods(methods []string) bool {
	return len(methods) == 1 && methods[0] == "*"
}

// hasGlobalPolicyByName reports whether a GlobalPolicy with the given name already exists.
func hasGlobalPolicyByName(policies []model.GlobalPolicy, name string) bool {
	for _, p := range policies {
		if p.Name == name {
			return true
		}
	}
	return false
}

// appendLegacyOperationPath merges a path entry into an existing OperationPolicy of the same
// name+version, or appends a new OperationPolicy if none exists.
func appendLegacyOperationPath(policies *[]model.OperationPolicy, name, version string, path model.OperationPolicyPath) {
	for i := range *policies {
		if (*policies)[i].Name == name && (*policies)[i].Version == version {
			(*policies)[i].Paths = append((*policies)[i].Paths, path)
			return
		}
	}
	*policies = append(*policies, model.OperationPolicy{
		Name:    name,
		Version: version,
		Paths:   []model.OperationPolicyPath{path},
	})
}

// splitLegacyPoliciesForRead converts a stored legacy policies list into the two
// canonical lists for read responses, using the same rule as the save-time migration:
//   - path "/*" + methods ["*"] → GlobalPolicy (shared api-level bucket)
//   - any other path            → OperationPolicy (per-path bucket)
//
// Called only when both new lists are empty and the legacy list is non-empty.
func splitLegacyPoliciesForRead(legacy []model.LLMPolicy) ([]model.GlobalPolicy, []model.OperationPolicy) {
	var global []model.GlobalPolicy
	var operation []model.OperationPolicy
	for _, p := range legacy {
		for _, pe := range p.Paths {
			if pe.Path == "/*" && isWildcardOnlyMethods(pe.Methods) {
				if !hasGlobalPolicyByName(global, p.Name) {
					global = append(global, model.GlobalPolicy{
						Name:    p.Name,
						Version: p.Version,
						Params:  pe.Params,
					})
				}
			} else {
				appendLegacyOperationPath(&operation, p.Name, p.Version, model.OperationPolicyPath{
					Path:    pe.Path,
					Methods: pe.Methods,
					Params:  pe.Params,
				})
			}
		}
	}
	return global, operation
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
		Header: utils.ValueOrEmpty(in.Header),
		Value:  utils.ValueOrEmpty(in.Value),
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
		URL: utils.ValueOrEmpty(in.Main.Url),
		Ref: utils.ValueOrEmpty(in.Main.Ref),
	}
	if in.Main.Auth != nil {
		out.Main.Auth = mapUpstreamAuthAPIToModel(in.Main.Auth)
	}
	if in.Sandbox != nil {
		out.Sandbox = &model.UpstreamEndpoint{
			URL: utils.ValueOrEmpty(in.Sandbox.Url),
			Ref: utils.ValueOrEmpty(in.Sandbox.Ref),
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
				Header: utils.StringPtrIfNotEmpty(in.Main.Auth.Header),
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
				Header: utils.StringPtrIfNotEmpty(in.Sandbox.Auth.Header),
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
		Header: utils.StringPtrIfNotEmpty(in.Header),
		Value:  utils.StringPtrIfNotEmpty(in.Value),
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
		resources = append(resources, model.RateLimitingResourceLimit{
			Resource: r.Resource,
			Limit:    *mapRateLimitingLimitAPIToModel(&r.Limit),
		})
	}
	return &model.ResourceWiseRateLimitingConfig{
		Default:   *mapRateLimitingLimitAPIToModel(&in.Default),
		Resources: resources,
	}
}

func templateListItem(t *model.LLMProviderTemplate) api.LLMProviderTemplateListItem {
	id := t.ID
	name := t.Name
	version := t.Version
	isLatest := t.IsLatest
	enabled := t.Enabled
	var logoURL string
	if t.Metadata != nil {
		logoURL = t.Metadata.LogoURL
	}
	return api.LLMProviderTemplateListItem{
		Id:          &id,
		GroupId:     utils.StringPtrIfNotEmpty(t.GroupID),
		DisplayName: name,
		Description: utils.StringPtrIfNotEmpty(t.Description),
		ManagedBy:   utils.StringPtrIfNotEmpty(t.ManagedBy),
		CreatedBy:   utils.StringPtrIfNotEmpty(t.CreatedBy),
		Version:     &version,
		IsLatest:    &isLatest,
		Enabled:     &enabled,
		LogoUrl:     utils.StringPtrIfNotEmpty(logoURL),
		ReadOnly:    utils.BoolPtr(t.Origin == constants.OriginDP),
		CreatedAt:   utils.TimePtr(t.CreatedAt),
		UpdatedAt:   utils.TimePtr(t.UpdatedAt),
	}
}

func mapTemplateModelToAPI(m *model.LLMProviderTemplate) *api.LLMProviderTemplate {
	if m == nil {
		return nil
	}
	isLatest := m.IsLatest
	enabled := m.Enabled
	return &api.LLMProviderTemplate{
		Id:               &m.ID,
		GroupId:          utils.StringPtrIfNotEmpty(m.GroupID),
		DisplayName:      m.Name,
		Description:      utils.StringPtrIfNotEmpty(m.Description),
		ManagedBy:        utils.StringPtrIfNotEmpty(m.ManagedBy),
		CreatedBy:        utils.StringPtrIfNotEmpty(m.CreatedBy),
		UpdatedBy:        utils.StringPtrIfNotEmpty(m.UpdatedBy),
		Version:          m.Version,
		IsLatest:         &isLatest,
		Enabled:          &enabled,
		Openapi:          utils.StringPtrIfNotEmpty(m.OpenAPISpec),
		Metadata:         mapTemplateMetadataModelToAPI(m.Metadata),
		PromptTokens:     mapExtractionIdentifierModelToAPI(m.PromptTokens),
		CompletionTokens: mapExtractionIdentifierModelToAPI(m.CompletionTokens),
		TotalTokens:      mapExtractionIdentifierModelToAPI(m.TotalTokens),
		RemainingTokens:  mapExtractionIdentifierModelToAPI(m.RemainingTokens),
		RequestModel:     mapExtractionIdentifierModelToAPI(m.RequestModel),
		ResponseModel:    mapExtractionIdentifierModelToAPI(m.ResponseModel),
		ResourceMappings: mapTemplateResourceMappingsModelToAPI(m.ResourceMappings),
		ReadOnly:         utils.BoolPtr(m.Origin == constants.OriginDP),
		CreatedAt:        utils.TimePtr(m.CreatedAt),
		UpdatedAt:        utils.TimePtr(m.UpdatedAt),
	}
}

func mapTemplateResourceMappingsAPI(in *api.LLMProviderTemplateResourceMappings) (*model.LLMProviderTemplateResourceMappings, error) {
	if in == nil {
		return nil, nil
	}
	out := &model.LLMProviderTemplateResourceMappings{}
	if in.Resources != nil {
		resources := make([]model.LLMProviderTemplateResourceMapping, 0, len(*in.Resources))
		for _, r := range *in.Resources {
			mapped, err := mapTemplateResourceMappingAPI(&r)
			if err != nil {
				return nil, err
			}
			if mapped != nil {
				resources = append(resources, *mapped)
			}
		}
		out.Resources = resources
	}
	if len(out.Resources) == 0 {
		return nil, nil
	}
	return out, nil
}

func mapTemplateResourceMappingAPI(in *api.LLMProviderTemplateResourceMapping) (*model.LLMProviderTemplateResourceMapping, error) {
	if in == nil {
		return nil, nil
	}
	resource, isValid := utils.NormalizeAndValidateLLMResourcePath(in.Resource)
	if !isValid {
		return nil, fmt.Errorf("%w: resource mapping resource must be a valid path pattern", constants.ErrInvalidInput)
	}
	return &model.LLMProviderTemplateResourceMapping{
		Resource: resource,
		LLMProviderTemplateExtractionFields: model.LLMProviderTemplateExtractionFields{
			PromptTokens:     mapExtractionIdentifierAPI(in.PromptTokens),
			CompletionTokens: mapExtractionIdentifierAPI(in.CompletionTokens),
			TotalTokens:      mapExtractionIdentifierAPI(in.TotalTokens),
			RemainingTokens:  mapExtractionIdentifierAPI(in.RemainingTokens),
			RequestModel:     mapExtractionIdentifierAPI(in.RequestModel),
			ResponseModel:    mapExtractionIdentifierAPI(in.ResponseModel),
		},
	}, nil
}

func mapTemplateResourceMappingsModelToAPI(in *model.LLMProviderTemplateResourceMappings) *api.LLMProviderTemplateResourceMappings {
	if in == nil {
		return nil
	}
	out := &api.LLMProviderTemplateResourceMappings{}
	if len(in.Resources) > 0 {
		resources := make([]api.LLMProviderTemplateResourceMapping, 0, len(in.Resources))
		for _, r := range in.Resources {
			mapped := mapTemplateResourceMappingModelToAPI(&r)
			if mapped != nil {
				resources = append(resources, *mapped)
			}
		}
		out.Resources = &resources
	}
	if out.Resources == nil || len(*out.Resources) == 0 {
		return nil
	}
	return out
}

func mapTemplateResourceMappingModelToAPI(in *model.LLMProviderTemplateResourceMapping) *api.LLMProviderTemplateResourceMapping {
	if in == nil {
		return nil
	}
	return &api.LLMProviderTemplateResourceMapping{
		Resource:         strings.TrimSpace(in.Resource),
		PromptTokens:     mapExtractionIdentifierModelToAPI(in.PromptTokens),
		CompletionTokens: mapExtractionIdentifierModelToAPI(in.CompletionTokens),
		TotalTokens:      mapExtractionIdentifierModelToAPI(in.TotalTokens),
		RemainingTokens:  mapExtractionIdentifierModelToAPI(in.RemainingTokens),
		RequestModel:     mapExtractionIdentifierModelToAPI(in.RequestModel),
		ResponseModel:    mapExtractionIdentifierModelToAPI(in.ResponseModel),
	}
}

// defaultTemplateManagedBy normalizes the managedBy label supplied on a custom
// template. An empty value defaults to "customer"; built-in templates are seeded
// with "wso2" by the template seeder/loader.
func defaultTemplateManagedBy(in *string) string {
	v := strings.TrimSpace(utils.ValueOrEmpty(in))
	if v == "" {
		return "customer"
	}
	return v
}

func mapTemplateMetadataAPI(in *api.LLMProviderTemplateMetadata) *model.LLMProviderTemplateMetadata {
	if in == nil {
		return nil
	}
	var auth *model.LLMProviderTemplateAuth
	if in.Auth != nil {
		auth = &model.LLMProviderTemplateAuth{
			Type:        utils.ValueOrEmpty(in.Auth.Type),
			Header:      utils.ValueOrEmpty(in.Auth.Header),
			ValuePrefix: utils.ValueOrEmpty(in.Auth.ValuePrefix),
		}
	}
	out := &model.LLMProviderTemplateMetadata{
		EndpointURL:    strings.TrimSpace(utils.ValueOrEmpty(in.EndpointUrl)),
		Auth:           auth,
		LogoURL:        strings.TrimSpace(utils.ValueOrEmpty(in.LogoUrl)),
		OpenapiSpecURL: strings.TrimSpace(utils.ValueOrEmpty(in.OpenapiSpecUrl)),
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
			Type:        utils.StringPtrIfNotEmpty(in.Auth.Type),
			Header:      utils.StringPtrIfNotEmpty(in.Auth.Header),
			ValuePrefix: utils.StringPtrIfNotEmpty(in.Auth.ValuePrefix),
		}
	}
	return &api.LLMProviderTemplateMetadata{
		EndpointUrl:    utils.StringPtrIfNotEmpty(in.EndpointURL),
		Auth:           auth,
		LogoUrl:        utils.StringPtrIfNotEmpty(in.LogoURL),
		OpenapiSpecUrl: utils.StringPtrIfNotEmpty(in.OpenapiSpecURL),
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

	globalPolicyCfg := m.Configuration.GlobalPolicies
	operationPolicyCfg := m.Configuration.OperationPolicies
	// For legacy rows stored before v1alpha2 migration: split policies on read.
	if len(globalPolicyCfg) == 0 && len(operationPolicyCfg) == 0 && len(m.Configuration.Policies) > 0 {
		globalPolicyCfg, operationPolicyCfg = splitLegacyPoliciesForRead(m.Configuration.Policies)
	}
	globalPolicies := mapGlobalPoliciesModelToAPI(globalPolicyCfg)
	if globalPolicies == nil {
		empty := []api.Policy{}
		globalPolicies = &empty
	}
	operationPolicies := mapOperationPoliciesModelToAPI(operationPolicyCfg)
	if operationPolicies == nil {
		empty := []api.OperationPolicy{}
		operationPolicies = &empty
	}

	modelProviders := mapModelProvidersModelToAPI(m.ModelProviders)
	if modelProviders == nil {
		empty := []api.LLMModelProvider{}
		modelProviders = &empty
	}

	out := &api.LLMProvider{
		Id:                &m.ID,
		DisplayName:       m.Name,
		Description:       utils.StringPtrIfNotEmpty(m.Description),
		CreatedBy:         utils.StringPtrIfNotEmpty(m.CreatedBy),
		Version:           m.Version,
		Context:           ctx,
		Vhost:             m.Configuration.VHost,
		Template:          templateHandle,
		Openapi:           utils.StringPtrIfNotEmpty(m.OpenAPISpec),
		ModelProviders:    modelProviders,
		RateLimiting:      mapRateLimitingModelToAPI(m.Configuration.RateLimiting),
		Upstream:          upstream,
		AccessControl:     ac,
		GlobalPolicies:    globalPolicies,
		OperationPolicies: operationPolicies,
		Policies:          nil,
		Security:          mapSecurityModelToAPI(m.Configuration.Security),
		ReadOnly:          utils.BoolPtr(m.Origin == constants.OriginDP),
		CreatedAt:         utils.TimePtr(m.CreatedAt),
		UpdatedAt:         utils.TimePtr(m.UpdatedAt),
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
		providerID := strings.TrimSpace(utils.ValueOrEmpty(p.Id))
		if providerID == "" {
			return constants.ErrInvalidInput
		}
		if _, ok := seenProviders[providerID]; ok {
			return constants.ErrInvalidInput
		}
		seenProviders[providerID] = struct{}{}

		models := []api.LLMModel{}
		if p.Models != nil {
			models = *p.Models
		}
		seenModels := make(map[string]struct{}, len(models))
		for _, m := range models {
			modelID := strings.TrimSpace(utils.ValueOrEmpty(m.Id))
			if modelID == "" {
				return constants.ErrInvalidInput
			}
			if _, ok := seenModels[modelID]; ok {
				return constants.ErrInvalidInput
			}
			seenModels[modelID] = struct{}{}
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
				models = append(models, model.LLMModel{ID: utils.ValueOrEmpty(m.Id), Name: m.DisplayName, Description: utils.ValueOrEmpty(m.Description)})
			}
		}
		out = append(out, model.LLMModelProvider{ID: utils.ValueOrEmpty(p.Id), Name: p.DisplayName, Models: models})
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
			models = append(models, api.LLMModel{Id: &m.ID, DisplayName: m.Name, Description: utils.StringPtrIfNotEmpty(m.Description)})
		}
		modelsPtr := &models
		out = append(out, api.LLMModelProvider{Id: &p.ID, DisplayName: p.Name, Models: modelsPtr})
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
		resources = append(resources, api.RateLimitingResourceLimit{Resource: r.Resource, Limit: mapRateLimitingLimitModelToAPIValue(&r.Limit)})
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
	globalPolicyCfgProxy := m.Configuration.GlobalPolicies
	operationPolicyCfgProxy := m.Configuration.OperationPolicies
	// For legacy rows stored before v1alpha2 migration: split policies on read.
	if len(globalPolicyCfgProxy) == 0 && len(operationPolicyCfgProxy) == 0 && len(m.Configuration.Policies) > 0 {
		globalPolicyCfgProxy, operationPolicyCfgProxy = splitLegacyPoliciesForRead(m.Configuration.Policies)
	}
	globalPoliciesProxy := mapGlobalPoliciesModelToAPI(globalPolicyCfgProxy)
	if globalPoliciesProxy == nil {
		empty := []api.Policy{}
		globalPoliciesProxy = &empty
	}
	operationPoliciesProxy := mapOperationPoliciesModelToAPI(operationPolicyCfgProxy)
	if operationPoliciesProxy == nil {
		empty := []api.OperationPolicy{}
		operationPoliciesProxy = &empty
	}
	createdAt := utils.TimePtr(m.CreatedAt)
	updatedAt := utils.TimePtr(m.UpdatedAt)
	out := &api.LLMProxy{
		Id:          &m.ID,
		DisplayName: m.Name,
		Description: utils.StringPtrIfNotEmpty(m.Description),
		CreatedBy:   utils.StringPtrIfNotEmpty(m.CreatedBy),
		Version:     m.Version,
		ProjectId:   m.ProjectUUID,
		Context:     contextValue,
		Vhost:       vhostValue,
		Provider: api.LLMProxyProvider{
			Id:   m.Configuration.Provider,
			Auth: nil,
		},
		Openapi:   utils.StringPtrIfNotEmpty(m.OpenAPISpec),
		Security:  mapSecurityModelToAPI(m.Configuration.Security),
		ReadOnly:  utils.BoolPtr(m.Origin == constants.OriginDP),
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
			Header: utils.StringPtrIfNotEmpty(m.Configuration.UpstreamAuth.Header),
			Value:  nil, // Redact auth credential value
		}
	}
	out.GlobalPolicies = globalPoliciesProxy
	out.OperationPolicies = operationPoliciesProxy
	out.Policies = nil
	return out
}

func mapSecurityAPIToModel(in *api.SecurityConfig) *model.SecurityConfig {
	if in == nil {
		return nil
	}
	out := &model.SecurityConfig{Enabled: in.Enabled}
	if in.ApiKey != nil {
		key := utils.ValueOrEmpty(in.ApiKey.Key)
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
		out.ApiKey = &api.APIKeySecurity{Enabled: in.APIKey.Enabled, Key: utils.StringPtrIfNotEmpty(in.APIKey.Key), In: inLoc}
	}
	return out
}

// marshalUpstreamForValidation serialises the upstream config to JSON so
// ValidateSecretRefs can scan it for {{ secret "..." }} placeholders.
func marshalUpstreamForValidation(upstream interface{}) (string, error) {
	b, err := json.Marshal(upstream)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

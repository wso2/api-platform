/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package service

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

const (
	mcpStatusPending  = "pending"
	mcpStatusDeployed = "deployed"
	mcpStatusFailed   = "failed"
)

// MCPProxyService handles business logic for MCP proxy operations
type MCPProxyService struct {
	repo                 repository.MCPProxyRepository
	projectRepo          repository.ProjectRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	gatewayEventsService *GatewayEventsService
	secretService        *SecretService
	slogger              *slog.Logger
	auditRepo            repository.AuditRepository
	cfg                  *config.Server
	identity             *IdentityService
}

// NewMCPProxyService creates a new MCPProxyService instance
func NewMCPProxyService(repo repository.MCPProxyRepository, projectRepo repository.ProjectRepository,
	deploymentRepo repository.DeploymentRepository, gatewayRepo repository.GatewayRepository,
	gatewayEventsService *GatewayEventsService, slogger *slog.Logger, auditRepo repository.AuditRepository,
	cfg *config.Server, identity *IdentityService) *MCPProxyService {
	return &MCPProxyService{
		repo:                 repo,
		projectRepo:          projectRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		gatewayEventsService: gatewayEventsService,
		slogger:              slogger,
		auditRepo:            auditRepo,
		cfg:                  cfg,
		identity:             identity,
	}
}

// toMCPProxyAPI converts m via mapMCPProxyModelToAPI and resolves its
// createdBy/updatedBy UUIDs to their raw external identity.
func (s *MCPProxyService) toMCPProxyAPI(m *model.MCPProxy) (*api.MCPProxy, error) {
	resp := mapMCPProxyModelToAPI(m)
	if resp == nil {
		return nil, nil
	}
	if err := s.identity.ResolveIdentityField(&resp.CreatedBy); err != nil {
		return nil, err
	}
	if err := s.identity.ResolveIdentityField(&resp.UpdatedBy); err != nil {
		return nil, err
	}
	return resp, nil
}

// WithSecretService injects the SecretService for secret-ref validation.
func (s *MCPProxyService) WithSecretService(ss *SecretService) {
	s.secretService = ss
}

// Create creates a new MCP proxy
func (s *MCPProxyService) Create(orgUUID, createdBy string, req *api.MCPProxy) (*api.MCPProxy, error) {
	if req == nil {
		return nil, apperror.ValidationFailed.New("A request body is required.")
	}
	if req.DisplayName == "" || req.Version == "" {
		return nil, apperror.ValidationFailed.New("The displayName and version fields are required.")
	}
	if err := validatePolicyVersions(req.Policies); err != nil {
		return nil, err
	}

	if req.Upstream.Main.Url == nil || *req.Upstream.Main.Url == "" {
		return nil, apperror.ValidationFailed.New("The upstream main url field is required.")
	}

	// req.ProjectId is the project handle; resolve it to the project UUID so the
	// proxy is stored against the same identifier List filters on. Without the
	// project repository we cannot resolve the handle, and must not fall back to
	// storing the raw handle as a UUID — fail fast instead.
	var projectUUID *string
	if req.ProjectId != nil && *req.ProjectId != "" {
		if s.projectRepo == nil {
			return nil, fmt.Errorf("cannot resolve project handle: project repository unavailable")
		}
		project, err := s.projectRepo.GetProjectByHandleAndOrgID(*req.ProjectId, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate project: %w", err)
		}
		if project == nil || project.OrganizationID != orgUUID {
			return nil, apperror.ProjectRefNotFound.New()
		}
		projectUUID = &project.ID
	}

	// Determine handle: use provided id or auto-generate from displayName
	var handle string
	if req.Id != nil && *req.Id != "" {
		handle = *req.Id
		exists, err := s.repo.Exists(handle, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to check MCP proxy exists: %w", err)
		}
		if exists {
			return nil, apperror.MCPProxyExists.New()
		}
	} else {
		var err error
		handle, err = utils.GenerateHandle(req.DisplayName, func(h string) bool {
			exists, _ := s.repo.Exists(h, orgUUID)
			return exists
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate MCP proxy handle: %w", err)
		}
	}
	req.Id = &handle

	// Enforce the per-organization MCP proxy limit (unlimited when not configured).
	proxyCount, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count existing MCP proxies: %w", err)
	}
	if config.LimitReached(proxyCount, s.cfg.ArtifactLimits.MaxMCPProxiesPerOrg) {
		return nil, apperror.MCPProxyLimitReached.New()
	}

	// Validate {{ secret "..." }} placeholders anywhere in the request — the
	// gateway-controller's template engine resolves placeholders generically
	// across the whole artifact (policies included), not just upstream.auth,
	// so validation must cover the same surface.
	if s.secretService != nil {
		configJSON, err := marshalUpstreamForValidation(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request for secret validation: %w", err)
		}
		if err := s.secretService.ValidateSecretRefs(orgUUID, configJSON); err != nil {
			return nil, err
		}
	}

	// Resolve any associated gateways up-front so they can be persisted within the
	// same transaction as the MCP proxy create.
	associatedGateways, err := resolveAssociatedGateways(s.gatewayRepo, orgUUID, req.AssociatedGateways)
	if err != nil {
		return nil, err
	}

	// Create MCP proxy model
	m := &model.MCPProxy{
		Handle:           handle,
		OrganizationUUID: orgUUID,
		ProjectUUID:      projectUUID,
		Name:             req.DisplayName,
		Description:      utils.ValueOrEmpty(req.Description),
		CreatedBy:        createdBy,
		UpdatedBy:        createdBy,
		Version:          req.Version,
		Configuration: model.MCPProxyConfiguration{
			Name:         req.DisplayName,
			Version:      req.Version,
			Context:      req.Context,
			Vhost:        req.Vhost,
			SpecVersion:  mcpSpecVersionToString(req.McpSpecVersion),
			Upstream:     *mapUpstreamAPIToModel(req.Upstream),
			Policies:     mapMCPPoliciesAPIToModel(req.Policies),
			Capabilities: mapMcpCapabilitiesAPIToModel(req.Capabilities),
		},
		Origin:             constants.OriginCP,
		AssociatedGateways: associatedGateways,
	}

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, apperror.MCPProxyExists.Wrap(err)
		}
		return nil, fmt.Errorf("failed to create MCP proxy: %w", err)
	}

	_ = s.auditRepo.Record("CREATE", m.UUID, "mcp_proxy", orgUUID, createdBy)
	return s.Get(orgUUID, handle)
}

// List retrieves all MCP proxies for an organization
func (s *MCPProxyService) List(orgUUID string, limit, offset int) (*api.MCPProxyListResponse, error) {
	proxies, err := s.repo.List(orgUUID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP proxies: %w", err)
	}

	totalCount, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count MCP proxies: %w", err)
	}

	resp := &api.MCPProxyListResponse{
		Count: len(proxies),
		Pagination: api.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}

	resp.List = make([]api.MCPProxyListItem, 0, len(proxies))
	createdByFields := make([]**string, 0, len(proxies))
	for _, p := range proxies {
		item := mapMCPProxyModelToListItem(p)
		if item == nil {
			continue
		}
		resp.List = append(resp.List, *item)
		createdByFields = append(createdByFields, &resp.List[len(resp.List)-1].CreatedBy)
	}
	if err := s.identity.ResolveIdentityFields(createdByFields); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListByProject retrieves MCP proxies for an organization filtered by project ID
func (s *MCPProxyService) ListByProject(orgUUID, projectHandle string, limit, offset int) (*api.MCPProxyListResponse, error) {
	// projectHandle is the project handle; resolve it to the project UUID so the
	// filter matches the identifier proxies are stored against. Without the
	// project repository we cannot resolve the handle, and must not filter on the
	// raw handle as if it were a UUID — fail fast instead.
	if s.projectRepo == nil {
		return nil, fmt.Errorf("cannot resolve project handle: project repository unavailable")
	}
	project, err := s.projectRepo.GetProjectByHandleAndOrgID(projectHandle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate project: %w", err)
	}
	if project == nil || project.OrganizationID != orgUUID {
		return nil, apperror.ProjectRefNotFound.New()
	}
	projectUUID := project.ID

	// TODO: pagination
	proxies, err := s.repo.ListByProject(orgUUID, projectUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP proxies by project: %w", err)
	}

	totalCount, err := s.repo.CountByProject(orgUUID, projectUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count MCP proxies by project: %w", err)
	}

	resp := &api.MCPProxyListResponse{
		Count: len(proxies),
		Pagination: api.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}

	resp.List = make([]api.MCPProxyListItem, 0, len(proxies))
	createdByFields := make([]**string, 0, len(proxies))
	for _, p := range proxies {
		item := mapMCPProxyModelToListItem(p)
		if item == nil {
			continue
		}
		resp.List = append(resp.List, *item)
		createdByFields = append(createdByFields, &resp.List[len(resp.List)-1].CreatedBy)
	}
	if err := s.identity.ResolveIdentityFields(createdByFields); err != nil {
		return nil, err
	}

	return resp, nil
}

// Get retrieves an MCP proxy by its handle
func (s *MCPProxyService) Get(orgUUID, handle string) (*api.MCPProxy, error) {
	if handle == "" {
		return nil, apperror.ValidationFailed.New("The MCP proxy id is required.")
	}

	m, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP proxy: %w", err)
	}
	if m == nil {
		return nil, apperror.MCPProxyNotFound.New()
	}

	return s.toMCPProxyAPI(m)
}

// Update updates an existing MCP proxy
func (s *MCPProxyService) Update(orgUUID, handle, updatedBy string, req *api.MCPProxy) (*api.MCPProxy, error) {
	if handle == "" || req == nil {
		return nil, apperror.ValidationFailed.New("The MCP proxy id and a request body are required.")
	}
	if req.DisplayName == "" || req.Version == "" {
		return nil, apperror.ValidationFailed.New("The displayName and version fields are required.")
	}
	if err := validatePolicyVersions(req.Policies); err != nil {
		return nil, err
	}

	if req.Upstream.Main.Url == nil || *req.Upstream.Main.Url == "" {
		return nil, apperror.ValidationFailed.New("The upstream main url field is required.")
	}

	// Get existing proxy
	existing, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP proxy: %w", err)
	}
	if existing == nil {
		return nil, apperror.MCPProxyNotFound.New()
	}

	// Validate {{ secret "..." }} placeholders anywhere in the request — the
	// gateway-controller's template engine resolves placeholders generically
	// across the whole artifact (policies included), not just upstream.auth,
	// so validation must cover the same surface.
	if s.secretService != nil {
		configJSON, err := marshalUpstreamForValidation(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request for secret validation: %w", err)
		}
		if err := s.secretService.ValidateSecretRefs(orgUUID, configJSON); err != nil {
			return nil, err
		}
	}

	// Store existing upstream config for auth preservation
	existingUpstreamConfig := existing.Configuration.Upstream

	// Snapshot the gateway-owned runtime fields so a DP-originated proxy can preserve them
	// (only the description is control-plane editable — restored below).
	origName := existing.Name
	origVersion := existing.Version
	origConfiguration := existing.Configuration

	// Update fields
	existing.Name = req.DisplayName
	existing.Version = req.Version
	existing.UpdatedBy = updatedBy
	existing.Description = utils.ValueOrEmpty(req.Description)
	existing.Configuration = model.MCPProxyConfiguration{
		Name:         req.DisplayName,
		Version:      req.Version,
		Context:      req.Context,
		Vhost:        req.Vhost,
		SpecVersion:  mcpSpecVersionToString(req.McpSpecVersion),
		Upstream:     *mapUpstreamAPIToModel(req.Upstream),
		Policies:     mapMCPPoliciesAPIToModel(req.Policies),
		Capabilities: mapMcpCapabilitiesAPIToModel(req.Capabilities),
	}

	// Preserve existing upstream auth credential if not provided in update request
	// (the auth value is redacted in GET responses, so clients send empty value on updates)
	existing.Configuration.Upstream = *preserveMCPUpstreamAuthValue(&existingUpstreamConfig, &existing.Configuration.Upstream)

	// The gateway owns the runtime configuration of a DP-originated proxy: preserve it
	// verbatim and keep only the control-plane-editable description from the request.
	// This keeps the gateway runtime artifact unchanged without depending on the update
	// payload round-tripping the (masked) upstream credential.
	if existing.Origin == constants.OriginDP {
		existing.Name = origName
		existing.Version = origVersion
		existing.Configuration = origConfiguration
	}

	// Gateway associations are managed only when the field is present in the request. An
	// omitted field leaves associations untouched; an explicit (possibly empty) list
	// replaces the full set, removing any mapping no longer listed. Deployment state is not
	// consulted and deployment records are never modified here.
	requested, manage, err := resolveManagedAssociatedGateways(s.gatewayRepo, orgUUID, req.AssociatedGateways)
	if err != nil {
		return nil, err
	}
	if manage {
		existing.AssociatedGateways = requested
		existing.ReplaceAssociatedGateways = true
	}

	if err := s.repo.Update(existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.MCPProxyNotFound.Wrap(err)
		}
		return nil, fmt.Errorf("failed to update MCP proxy: %w", err)
	}

	// Best-effort: delete the secret the credential was rotated away from. Must
	// run after the update above persists the new reference, so the in-use
	// check below no longer sees this proxy pointing at the old handle.
	if s.secretService != nil {
		s.secretService.cleanupRotatedSecret(
			orgUUID,
			mainUpstreamAuthValue(&existingUpstreamConfig),
			mainUpstreamAuthValue(&existing.Configuration.Upstream),
			updatedBy,
			s.slogger,
		)
	}

	_ = s.auditRepo.Record("UPDATE", existing.UUID, "mcp_proxy", orgUUID, updatedBy)
	return s.Get(orgUUID, handle)
}

// Delete deletes an MCP proxy by its handle
func (s *MCPProxyService) Delete(orgUUID, handle, deletedBy string) error {
	if handle == "" {
		return apperror.ValidationFailed.New("The MCP proxy id is required.")
	}

	// Get the MCP proxy UUID before deletion (needed for deployment lookup)
	mcpProxy, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to get MCP proxy: %w", err)
	}
	if mcpProxy == nil {
		return apperror.MCPProxyNotFound.New()
	}

	// DP-originated artifacts may only be deleted once undeployed on all gateways.
	if err := ensureOriginDeletable(s.deploymentRepo, mcpProxy.Origin, mcpProxy.UUID, orgUUID); err != nil {
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
			s.slogger.Warn("Failed to get gateways for MCP proxy deletion", "error", err, "proxyUUID", mcpProxy.UUID)
		} else {
			gateways = gws
		}
	}

	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.MCPProxyNotFound.Wrap(err)
		}
		return fmt.Errorf("failed to delete MCP proxy: %w", err)
	}

	_ = s.auditRepo.Record("DELETE", mcpProxy.UUID, "mcp_proxy", orgUUID, deletedBy)
	// Send deletion events to all gateways in the organization
	if s.gatewayEventsService != nil && len(gateways) > 0 {
		for _, gateway := range gateways {
			deletionEvent := &model.MCPProxyDeletionEvent{
				ProxyId: mcpProxy.UUID,
			}

			if err := s.gatewayEventsService.BroadcastMCPProxyDeletionEvent(gateway.ID, deletionEvent); err != nil {
				s.slogger.Warn("Failed to broadcast MCP proxy deletion event", "error", err, "gatewayID", gateway.ID, "proxyUUID", mcpProxy.UUID)
			} else {
				s.slogger.Info("MCP proxy deletion event sent", "gatewayID", gateway.ID, "proxyUUID", mcpProxy.UUID)
			}
		}
	}

	return nil
}

// FetchServerInfo fetches server information from an MCP backend.
// When proxyId is provided, the URL and auth are fetched from the stored proxy configuration.
// When proxyId is not provided, url is required and auth is optional.
func (s *MCPProxyService) FetchServerInfo(orgUUID string, req *api.MCPServerInfoFetchRequest) (*api.MCPServerInfoFetchResponse, error) {
	if req == nil {
		return nil, apperror.ValidationFailed.New("A request body is required.")
	}

	var url string
	var headerName, headerValue string

	if req.ProxyId != nil && *req.ProxyId != "" {
		if req.Auth != nil {
			s.slogger.Warn("Auth override is not allowed when proxyId is provided. Ignoring auth in request and using stored auth from proxy configuration.", "org_id", orgUUID, "proxy_id", *req.ProxyId)
		}
		// ProxyId provided - fetch stored configuration (refetch flow)
		// Auth override is NOT allowed in refetch - use exactly what's stored
		proxy, err := s.repo.GetByHandle(*req.ProxyId, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get MCP proxy: %w", err)
		}
		if proxy == nil {
			return nil, apperror.MCPProxyNotFound.New()
		}

		// Use the stored URL from the proxy configuration verbatim. The stored value is the
		// full MCP endpoint URL that was validated at creation time; the gateway forwards to
		// exactly this upstream path, so we must not append "/mcp" (or otherwise manipulate it)
		// here — doing so would diverge from what is stored and deployed to the gateway.
		if proxy.Configuration.Upstream.Main != nil && proxy.Configuration.Upstream.Main.URL != "" {
			url = proxy.Configuration.Upstream.Main.URL
		}

		// Use stored auth from proxy configuration
		if proxy.Configuration.Upstream.Main != nil && proxy.Configuration.Upstream.Main.Auth != nil {
			headerName = proxy.Configuration.Upstream.Main.Auth.Header
			headerValue = proxy.Configuration.Upstream.Main.Auth.Value
		}
	} else {
		// No proxyId - initial creation flow, url is required
		if req.Url == nil || *req.Url == "" {
			return nil, apperror.ValidationFailed.New("The url field is required when proxyId is not provided.")
		}
		url = *req.Url

		// Use provided auth (optional for initial fetch)
		if req.Auth != nil && req.Auth.Header != nil && req.Auth.Value != nil {
			headerName = *req.Auth.Header
			headerValue = *req.Auth.Value
		}
	}

	if err := utils.ValidateURL(url); err != nil {
		return nil, apperror.ValidationFailed.Wrap(err, "The provided URL is invalid.").
			WithLogMessage(fmt.Sprintf("invalid MCP server URL: %s", url))
	}

	if err := utils.CheckURLReachability(url, 10*time.Second); err != nil {
		return nil, apperror.ValidationFailed.Wrap(err, "The provided URL is unreachable.").
			WithLogMessage(fmt.Sprintf("MCP server URL is unreachable: %s", url))
	}

	return utils.FetchMCPServerInfo(url, headerName, headerValue)
}

// Helper functions

func mcpSpecVersionToString(v *api.MCPProxyMcpSpecVersion) string {
	if v == nil {
		return ""
	}
	return string(*v)
}

func mapMCPProxyModelToAPI(m *model.MCPProxy) *api.MCPProxy {
	if m == nil {
		return nil
	}

	desc := m.Description
	createdBy := m.CreatedBy

	var specVersion *api.MCPProxyMcpSpecVersion
	if m.Configuration.SpecVersion != "" {
		sv := api.MCPProxyMcpSpecVersion(m.Configuration.SpecVersion)
		specVersion = &sv
	}

	out := &api.MCPProxy{
		Id:             &m.Handle,
		DisplayName:    m.Name,
		Description:    &desc,
		CreatedBy:      &createdBy,
		Version:        m.Version,
		ProjectId:      m.ProjectUUID,
		Context:        m.Configuration.Context,
		Vhost:          m.Configuration.Vhost,
		McpSpecVersion: specVersion,
		Upstream:       mapMCPUpstreamModelToAPI(&m.Configuration.Upstream),
		Policies:       mapMCPPoliciesModelToAPI(m.Configuration.Policies),
		Capabilities:   mapMcpCapabilitiesModelToAPI(m.Configuration.Capabilities),
		ReadOnly:       utils.BoolPtr(m.Origin == constants.OriginDP),
		CreatedAt:      utils.TimePtr(m.CreatedAt),
		UpdatedAt:      utils.TimePtr(m.UpdatedAt),
		UpdatedBy:      utils.StringPtrIfNotEmpty(m.UpdatedBy),
	}
	if associated := mapAssociatedGatewaysModelToAPI(m.AssociatedGateways); associated != nil {
		out.AssociatedGateways = associated
	}
	return out
}

func mapMCPProxyModelToListItem(m *model.MCPProxy) *api.MCPProxyListItem {
	if m == nil {
		return nil
	}

	return &api.MCPProxyListItem{
		Id:             utils.StringPtrIfNotEmpty(m.Handle),
		DisplayName:    m.Name,
		Description:    utils.StringPtrIfNotEmpty(m.Description),
		CreatedBy:      utils.StringPtrIfNotEmpty(m.CreatedBy),
		Version:        utils.StringPtrIfNotEmpty(m.Version),
		ProjectId:      m.ProjectUUID,
		Context:        m.Configuration.Context,
		McpSpecVersion: utils.StringPtrIfNotEmpty(m.Configuration.SpecVersion),
		ReadOnly:       utils.BoolPtr(m.Origin == constants.OriginDP),
		CreatedAt:      utils.TimePtr(m.CreatedAt),
		UpdatedAt:      utils.TimePtr(m.UpdatedAt),
	}
}

// mapMCPPoliciesAPIToModel converts API policies to model policies
func mapMCPPoliciesAPIToModel(in *[]api.Policy) []model.Policy {
	if in == nil || len(*in) == 0 {
		return nil
	}
	out := make([]model.Policy, 0, len(*in))
	for _, p := range *in {
		policy := model.Policy{
			Name:    p.Name,
			Version: p.Version,
		}
		if p.ExecutionCondition != nil {
			policy.ExecutionCondition = p.ExecutionCondition
		}
		if p.Params != nil {
			policy.Params = p.Params
		}
		out = append(out, policy)
	}
	return out
}

// mapMCPPoliciesModelToAPI converts model policies to API policies
func mapMCPPoliciesModelToAPI(in []model.Policy) *[]api.Policy {
	if len(in) == 0 {
		return nil
	}
	out := make([]api.Policy, 0, len(in))
	for _, p := range in {
		policy := api.Policy{
			Name:    p.Name,
			Version: p.Version,
		}
		if p.ExecutionCondition != nil {
			policy.ExecutionCondition = p.ExecutionCondition
		}
		if p.Params != nil {
			policy.Params = p.Params
		}
		out = append(out, policy)
	}
	return &out
}

// mapMcpCapabilitiesAPIToModel converts API capabilities to model capabilities
func mapMcpCapabilitiesAPIToModel(in *api.MCPProxyCapabilities) *model.MCPProxyCapabilities {
	if in == nil {
		return nil
	}
	return &model.MCPProxyCapabilities{
		Prompts:   in.Prompts,
		Resources: in.Resources,
		Tools:     in.Tools,
	}
}

// mapMcpCapabilitiesModelToAPI converts model capabilities to API capabilities
func mapMcpCapabilitiesModelToAPI(in *model.MCPProxyCapabilities) *api.MCPProxyCapabilities {
	if in == nil {
		return nil
	}
	return &api.MCPProxyCapabilities{
		Prompts:   in.Prompts,
		Resources: in.Resources,
		Tools:     in.Tools,
	}
}

// mapMCPUpstreamModelToAPI maps upstream config to API type with auth values redacted for security
func mapMCPUpstreamModelToAPI(in *model.UpstreamConfig) api.Upstream {
	main := api.UpstreamDefinition{}
	if in != nil && in.Main != nil {
		if in.Main.URL != "" {
			u := in.Main.URL
			main.Url = &u
		}
		if in.Main.Ref != "" {
			r := in.Main.Ref
			main.Ref = &r
		}
		if in.Main.Auth != nil {
			// Redact auth value for security
			var authType *api.UpstreamAuthType
			if in.Main.Auth.Type != "" {
				t := api.UpstreamAuthType(in.Main.Auth.Type)
				authType = &t
			}
			main.Auth = &api.UpstreamAuth{
				Type:   authType,
				Header: utils.StringPtrIfNotEmpty(in.Main.Auth.Header),
				Value:  nil, // Redact value - never expose auth credential
			}
		}
	}
	var sandbox *api.UpstreamDefinition
	if in != nil && in.Sandbox != nil {
		s := api.UpstreamDefinition{}
		if in.Sandbox.URL != "" {
			u := in.Sandbox.URL
			s.Url = &u
		}
		if in.Sandbox.Ref != "" {
			r := in.Sandbox.Ref
			s.Ref = &r
		}
		if in.Sandbox.Auth != nil {
			// Redact auth value for security
			var authType *api.UpstreamAuthType
			if in.Sandbox.Auth.Type != "" {
				t := api.UpstreamAuthType(in.Sandbox.Auth.Type)
				authType = &t
			}
			s.Auth = &api.UpstreamAuth{
				Type:   authType,
				Header: utils.StringPtrIfNotEmpty(in.Sandbox.Auth.Header),
				Value:  nil, // Redact value - never expose auth credential
			}
		}
		sandbox = &s
	}
	return api.Upstream{Main: main, Sandbox: sandbox}
}

// preserveMCPUpstreamAuthValue preserves the existing upstream auth value when the update
// request doesn't provide a new one (empty string). This prevents accidental credential
// loss when the client receives a redacted response and sends it back in an update.
func preserveMCPUpstreamAuthValue(existing, updated *model.UpstreamConfig) *model.UpstreamConfig {
	if updated == nil {
		return existing
	}
	if existing == nil {
		return updated
	}
	if updated.Main == nil {
		return updated
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

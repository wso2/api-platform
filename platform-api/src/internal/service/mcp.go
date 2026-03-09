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
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
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
	gatewayRepo          repository.GatewayRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayEventsService *GatewayEventsService
	slogger              *slog.Logger
}

// NewMCPProxyService creates a new MCPProxyService instance
func NewMCPProxyService(repo repository.MCPProxyRepository, projectRepo repository.ProjectRepository,
	gatewayRepo repository.GatewayRepository, deploymentRepo repository.DeploymentRepository,
	gatewayEventsService *GatewayEventsService, slogger *slog.Logger) *MCPProxyService {
	return &MCPProxyService{
		repo:                 repo,
		projectRepo:          projectRepo,
		gatewayRepo:          gatewayRepo,
		deploymentRepo:       deploymentRepo,
		gatewayEventsService: gatewayEventsService,
		slogger:              slogger,
	}
}

// Create creates a new MCP proxy
func (s *MCPProxyService) Create(orgUUID, createdBy string, req *api.MCPProxy) (*api.MCPProxy, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id == "" || req.Name == "" || req.Version == "" {
		return nil, constants.ErrInvalidInput
	}

	// Validate project exists if provided
	if s.projectRepo != nil && req.ProjectId != nil && *req.ProjectId != "" {
		project, err := s.projectRepo.GetProjectByUUID(*req.ProjectId)
		if err != nil {
			return nil, fmt.Errorf("failed to validate project: %w", err)
		}
		if project == nil {
			return nil, constants.ErrProjectNotFound
		}
		if project.OrganizationID != orgUUID {
			return nil, constants.ErrProjectNotFound
		}
	}

	// Check if MCP proxy already exists
	exists, err := s.repo.Exists(req.Id, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check MCP proxy exists: %w", err)
	}
	if exists {
		return nil, constants.ErrMCPProxyExists
	}

	// Create MCP proxy model
	m := &model.MCPProxy{
		Handle:           req.Id,
		OrganizationUUID: orgUUID,
		ProjectUUID:      req.ProjectId,
		Name:             req.Name,
		Description:      utils.ValueOrEmpty(req.Description),
		CreatedBy:        createdBy,
		Version:          req.Version,
		Status:           mcpStatusPending,
		Configuration: model.MCPProxyConfiguration{
			Name:         req.Name,
			Version:      req.Version,
			Context:      req.Context,
			Vhost:        req.Vhost,
			SpecVersion:  mcpSpecVersionToString(req.McpSpecVersion),
			Upstream:     *mapUpstreamAPIToModel(req.Upstream),
			Policies:     mapMCPPoliciesAPIToModel(req.Policies),
			Capabilities: mapMcpCapabilitiesAPIToModel(req.Capabilities),
		},
	}

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrMCPProxyExists
		}
		return nil, fmt.Errorf("failed to create MCP proxy: %w", err)
	}

	return s.Get(orgUUID, req.Id)
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
	for _, p := range proxies {
		resp.List = append(resp.List, *mapMCPProxyModelToListItem(p))
	}

	return resp, nil
}

// Get retrieves an MCP proxy by its handle
func (s *MCPProxyService) Get(orgUUID, handle string) (*api.MCPProxy, error) {
	if handle == "" {
		return nil, constants.ErrInvalidInput
	}

	m, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP proxy: %w", err)
	}
	if m == nil {
		return nil, constants.ErrMCPProxyNotFound
	}

	return mapMCPProxyModelToAPI(m), nil
}

// Update updates an existing MCP proxy
func (s *MCPProxyService) Update(orgUUID, handle string, req *api.MCPProxy) (*api.MCPProxy, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id != "" && req.Id != handle {
		return nil, constants.ErrInvalidInput
	}
	if req.Name == "" || req.Version == "" {
		return nil, constants.ErrInvalidInput
	}

	// Get existing proxy
	existing, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP proxy: %w", err)
	}
	if existing == nil {
		return nil, constants.ErrMCPProxyNotFound
	}

	// Update fields
	existing.Name = req.Name
	existing.Version = req.Version
	existing.Description = utils.ValueOrEmpty(req.Description)
	existing.Configuration = model.MCPProxyConfiguration{
		Name:         req.Name,
		Version:      req.Version,
		Context:      req.Context,
		Vhost:        req.Vhost,
		SpecVersion:  mcpSpecVersionToString(req.McpSpecVersion),
		Upstream:     *mapUpstreamAPIToModel(req.Upstream),
		Policies:     mapMCPPoliciesAPIToModel(req.Policies),
		Capabilities: mapMcpCapabilitiesAPIToModel(req.Capabilities),
	}

	if err := s.repo.Update(existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrMCPProxyNotFound
		}
		return nil, fmt.Errorf("failed to update MCP proxy: %w", err)
	}

	return s.Get(orgUUID, handle)
}

// Delete deletes an MCP proxy by its handle
func (s *MCPProxyService) Delete(orgUUID, handle string) error {
	if handle == "" {
		return constants.ErrInvalidInput
	}

	// Get the MCP proxy UUID before deletion (needed for deployment lookup)
	mcpProxy, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to get MCP proxy: %w", err)
	}
	if mcpProxy == nil {
		return constants.ErrMCPProxyNotFound
	}

	// Get all active gateway deployments BEFORE deletion
	// (deployments will be cascade deleted with the artifact)
	var gatewayDeployments []*model.Deployment
	if s.deploymentRepo != nil {
		// Get all deployments with DEPLOYED status for this MCP proxy
		statusDeployed := string(model.DeploymentStatusDeployed)
		deployments, err := s.deploymentRepo.GetDeploymentsWithState(mcpProxy.UUID, orgUUID, nil, &statusDeployed, 100)
		if err != nil {
			// Log warning but don't fail - proceed with deletion
			s.slogger.Warn("Failed to get gateway deployments for MCP proxy deletion", "error", err, "proxyUUID", mcpProxy.UUID)
		}
		gatewayDeployments = deployments
	}

	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrMCPProxyNotFound
		}
		return fmt.Errorf("failed to delete MCP proxy: %w", err)
	}

	// Send deletion events to all gateways where this proxy was deployed
	if s.gatewayEventsService != nil && len(gatewayDeployments) > 0 {
		for _, deployment := range gatewayDeployments {
			// Get gateway details to retrieve vhost
			gateway, err := s.gatewayRepo.GetByUUID(deployment.GatewayID)
			if err != nil {
				s.slogger.Warn("Failed to get gateway for MCP deletion event", "error", err, "gatewayID", deployment.GatewayID)
				continue
			}
			if gateway == nil {
				s.slogger.Warn("Gateway not found for MCP deletion event", "gatewayID", deployment.GatewayID)
				continue
			}

			// Create and send MCP proxy deletion event
			deletionEvent := &model.MCPProxyDeletionEvent{
				ProxyId: mcpProxy.UUID,
				Vhost:   gateway.Vhost,
			}

			if err := s.gatewayEventsService.BroadcastMCPProxyDeletionEvent(deployment.GatewayID, deletionEvent); err != nil {
				s.slogger.Warn("Failed to broadcast MCP proxy deletion event", "error", err, "gatewayID", deployment.GatewayID, "proxyUUID", mcpProxy.UUID)
			} else {
				s.slogger.Info("MCP proxy deletion event sent", "gatewayID", deployment.GatewayID, "proxyUUID", mcpProxy.UUID, "vhost", gateway.Vhost)
			}
		}
	}

	return nil
}

// FetchServerInfo fetches server information from an MCP backend
func (s *MCPProxyService) FetchServerInfo(req *api.MCPServerInfoFetchRequest) (*api.MCPServerInfoFetchResponse, error) {
	if req == nil || req.Url == "" {
		return nil, constants.ErrInvalidInput
	}

	if err := utils.ValidateURL(context.Background(), req.Url); err != nil {
		return nil, fmt.Errorf("%w: %v", constants.ErrInvalidURL, err)
	}

	// Extract header info from auth if provided
	var headerName, headerValue string
	if req.Auth != nil && req.Auth.Header != nil && req.Auth.Value != nil {
		headerName = *req.Auth.Header
		headerValue = *req.Auth.Value
	}

	return utils.FetchMCPServerInfo(req.Url, headerName, headerValue)
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

	return &api.MCPProxy{
		Id:             m.Handle,
		Name:           m.Name,
		Description:    &desc,
		CreatedBy:      &createdBy,
		Version:        m.Version,
		ProjectId:      m.ProjectUUID,
		Context:        m.Configuration.Context,
		Vhost:          m.Configuration.Vhost,
		McpSpecVersion: specVersion,
		Upstream:       mapUpstreamModelToAPI(&m.Configuration.Upstream),
		Policies:       mapMCPPoliciesModelToAPI(m.Configuration.Policies),
		Capabilities:   mapMcpCapabilitiesModelToAPI(m.Configuration.Capabilities),
	}
}

func mapMCPProxyModelToListItem(m *model.MCPProxy) *api.MCPProxyListItem {
	if m == nil {
		return nil
	}

	status := api.MCPProxyListItemStatus(m.Status)

	return &api.MCPProxyListItem{
		Id:             utils.StringPtrIfNotEmpty(m.Handle),
		Name:           utils.StringPtrIfNotEmpty(m.Name),
		Description:    utils.StringPtrIfNotEmpty(m.Description),
		CreatedBy:      utils.StringPtrIfNotEmpty(m.CreatedBy),
		Version:        utils.StringPtrIfNotEmpty(m.Version),
		ProjectId:      m.ProjectUUID,
		Context:        m.Configuration.Context,
		McpSpecVersion: utils.StringPtrIfNotEmpty(m.Configuration.SpecVersion),
		Status:         &status,
		CreatedAt:      utils.TimePtr(m.CreatedAt),
		UpdatedAt:      utils.TimePtr(m.UpdatedAt),
	}
}

// Generate generates MCP configuration from the given URL
func Generate(url string, outputDir string, headerName string, headerValue string) error {
	fmt.Printf("Generating MCP configuration for server: %s\n", url)

	// Use FetchMCPServerInfo to get all server information
	serverInfo, err := utils.FetchMCPServerInfo(url, headerName, headerValue)
	if err != nil {
		return err
	}

	if serverInfo.Tools != nil {
		fmt.Printf("→ Available Tools: %d\n", len(*serverInfo.Tools))
	}
	if serverInfo.Prompts != nil {
		fmt.Printf("→ Available Prompts: %d\n", len(*serverInfo.Prompts))
	}
	if serverInfo.Resources != nil {
		fmt.Printf("→ Available Resources: %d\n", len(*serverInfo.Resources))
	}
	fmt.Println("MCP generated successfully.")
	return nil
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

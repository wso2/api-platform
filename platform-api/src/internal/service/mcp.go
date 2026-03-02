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

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

const (
	mcpStatusPending  = "pending"
	mcpStatusDeployed = "deployed"
	mcpStatusFailed   = "failed"
)

// MCPProxyService handles business logic for MCP proxy operations
type MCPProxyService struct {
	repo        repository.MCPProxyRepository
	projectRepo repository.ProjectRepository
}

// NewMCPProxyService creates a new MCPProxyService instance
func NewMCPProxyService(repo repository.MCPProxyRepository, projectRepo repository.ProjectRepository) *MCPProxyService {
	return &MCPProxyService{
		repo:        repo,
		projectRepo: projectRepo,
	}
}

// Create creates a new MCP proxy
func (s *MCPProxyService) Create(orgUUID, createdBy string, req *api.MCPProxy) (*api.MCPProxy, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id == "" || req.Name == "" || req.Version == "" || req.ProjectId == "" {
		return nil, constants.ErrInvalidInput
	}

	// Validate project exists
	if s.projectRepo != nil {
		project, err := s.projectRepo.GetProjectByUUID(req.ProjectId)
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
		Description:      valueOrEmpty(req.Description),
		CreatedBy:        createdBy,
		Version:          req.Version,
		Status:           mcpStatusPending,
		Configuration: model.MCPProxyConfiguration{
			Name:        req.Name,
			Version:     req.Version,
			Context:     valueOrEmpty(req.Context),
			Vhost:       valueOrEmpty(req.Vhost),
			SpecVersion: mcpSpecVersionToString(req.McpSpecVersion),
			Upstream:    *mapUpstreamAPIToModel(req.Upstream),
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

	// Validate project if changed
	if s.projectRepo != nil && req.ProjectId != "" && req.ProjectId != existing.ProjectUUID {
		project, err := s.projectRepo.GetProjectByUUID(req.ProjectId)
		if err != nil {
			return nil, fmt.Errorf("failed to validate project: %w", err)
		}
		if project == nil {
			return nil, constants.ErrProjectNotFound
		}
		if project.OrganizationID != orgUUID {
			return nil, constants.ErrProjectNotFound
		}
		existing.ProjectUUID = req.ProjectId
	}

	// Update fields
	existing.Name = req.Name
	existing.Version = req.Version
	existing.Description = valueOrEmpty(req.Description)
	existing.Configuration = model.MCPProxyConfiguration{
		Name:        req.Name,
		Version:     req.Version,
		Context:     valueOrEmpty(req.Context),
		Vhost:       valueOrEmpty(req.Vhost),
		SpecVersion: mcpSpecVersionToString(req.McpSpecVersion),
		Upstream:    *mapUpstreamAPIToModel(req.Upstream),
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

	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrMCPProxyNotFound
		}
		return fmt.Errorf("failed to delete MCP proxy: %w", err)
	}

	return nil
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

	ctx := m.Configuration.Context
	vhost := m.Configuration.Vhost
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
		Context:        &ctx,
		Vhost:          &vhost,
		McpSpecVersion: specVersion,
		Upstream:       mapUpstreamModelToAPI(&m.Configuration.Upstream),
	}
}

func mapMCPProxyModelToListItem(m *model.MCPProxy) *api.MCPProxyListItem {
	if m == nil {
		return nil
	}

	status := api.MCPProxyListItemStatus(m.Status)

	return &api.MCPProxyListItem{
		Id:             stringPtrIfNotEmpty(m.Handle),
		Name:           stringPtrIfNotEmpty(m.Name),
		Description:    stringPtrIfNotEmpty(m.Description),
		CreatedBy:      stringPtrIfNotEmpty(m.CreatedBy),
		Version:        stringPtrIfNotEmpty(m.Version),
		ProjectId:      stringPtrIfNotEmpty(m.ProjectUUID),
		Context:        stringPtrIfNotEmpty(m.Configuration.Context),
		McpSpecVersion: stringPtrIfNotEmpty(m.Configuration.SpecVersion),
		Status:         &status,
		CreatedAt:      timePtr(m.CreatedAt),
		UpdatedAt:      timePtr(m.UpdatedAt),
	}
}

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

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
)

// WebSubAPIService handles business logic for WebSub API operations
type WebSubAPIService struct {
	repo                 repository.WebSubAPIRepository
	projectRepo          repository.ProjectRepository
	gatewayRepo          repository.GatewayRepository
	devPortalService     *DevPortalService
	gatewayEventsService *GatewayEventsService
	apiUtil              *utils.APIUtil
	slogger              *slog.Logger
}

// NewWebSubAPIService creates a new WebSubAPIService instance
func NewWebSubAPIService(
	repo repository.WebSubAPIRepository,
	projectRepo repository.ProjectRepository,
	gatewayRepo repository.GatewayRepository,
	devPortalService *DevPortalService,
	gatewayEventsService *GatewayEventsService,
	apiUtil *utils.APIUtil,
	slogger *slog.Logger,
) *WebSubAPIService {
	return &WebSubAPIService{
		repo:                 repo,
		projectRepo:          projectRepo,
		gatewayRepo:          gatewayRepo,
		devPortalService:     devPortalService,
		gatewayEventsService: gatewayEventsService,
		apiUtil:              apiUtil,
		slogger:              slogger,
	}
}

// Create creates a new WebSub API
func (s *WebSubAPIService) Create(orgUUID, createdBy string, req *api.WebSubAPI) (*api.WebSubAPI, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id == "" || req.Name == "" || req.Version == "" {
		return nil, constants.ErrInvalidInput
	}
	if req.ProjectId == "" {
		return nil, constants.ErrInvalidInput
	}
	if len(req.Channels) == 0 {
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

	// Check if already exists
	exists, err := s.repo.Exists(req.Id, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check WebSub API exists: %w", err)
	}
	if exists {
		return nil, constants.ErrWebSubAPIExists
	}

	// Check org limit
	count, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count existing WebSub APIs: %w", err)
	}
	if count >= constants.MaxWebSubAPIsPerOrganization {
		return nil, constants.ErrWebSubAPILimitReached
	}

	transport := []string{"http", "https"}
	if req.Transport != nil && len(*req.Transport) > 0 {
		transport = make([]string, 0, len(*req.Transport))
		for _, t := range *req.Transport {
			transport = append(transport, string(t))
		}
	}

	lifeCycleStatus := "CREATED"
	if req.LifeCycleStatus != nil {
		lifeCycleStatus = string(*req.LifeCycleStatus)
	}

	var subscriptionPlans []string
	if req.SubscriptionPlans != nil {
		subscriptionPlans = *req.SubscriptionPlans
	}

	channels := req.Channels
	m := &model.WebSubAPI{
		Handle:           req.Id,
		OrganizationUUID: orgUUID,
		ProjectUUID:      req.ProjectId,
		Name:             req.Name,
		Description:      utils.ValueOrEmpty(req.Description),
		CreatedBy:        createdBy,
		Version:          req.Version,
		LifeCycleStatus:  lifeCycleStatus,
		Transport:        transport,
		Configuration: model.WebSubAPIConfiguration{
			Name:              req.Name,
			Version:           req.Version,
			Context:           req.Context,
			Channels:          s.apiUtil.ChannelsAPIToModel(&channels),
			Upstream:          *mapUpstreamAPIToModel(req.Upstream),
			Policies:          mapMCPPoliciesAPIToModel(req.Policies),
			SubscriptionPlans: subscriptionPlans,
		},
	}

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrWebSubAPIExists
		}
		return nil, fmt.Errorf("failed to create WebSub API: %w", err)
	}

	return s.Get(orgUUID, req.Id)
}

// Get retrieves a WebSub API by its handle
func (s *WebSubAPIService) Get(orgUUID, handle string) (*api.WebSubAPI, error) {
	if handle == "" {
		return nil, constants.ErrInvalidInput
	}

	m, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get WebSub API: %w", err)
	}
	if m == nil {
		return nil, constants.ErrWebSubAPINotFound
	}

	return mapWebSubAPIModelToAPI(m, s.apiUtil), nil
}

// List retrieves WebSub APIs for an organization filtered by project
func (s *WebSubAPIService) List(orgUUID, projectUUID string, limit, offset int) (*api.WebSubAPIListResponse, error) {
	apis, err := s.repo.List(orgUUID, projectUUID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list WebSub APIs: %w", err)
	}

	var totalCount int
	if projectUUID != "" {
		totalCount, err = s.repo.CountByProject(orgUUID, projectUUID)
	} else {
		totalCount, err = s.repo.Count(orgUUID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to count WebSub APIs: %w", err)
	}

	resp := &api.WebSubAPIListResponse{
		Count: len(apis),
		Pagination: api.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}

	resp.List = make([]api.WebSubAPIListItem, 0, len(apis))
	for _, a := range apis {
		resp.List = append(resp.List, *mapWebSubAPIModelToListItem(a))
	}

	return resp, nil
}

// Update updates an existing WebSub API
func (s *WebSubAPIService) Update(orgUUID, handle string, req *api.WebSubAPI) (*api.WebSubAPI, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id != "" && req.Id != handle {
		return nil, constants.ErrInvalidInput
	}
	if req.Name == "" || req.Version == "" {
		return nil, constants.ErrInvalidInput
	}
	if len(req.Channels) == 0 {
		return nil, constants.ErrInvalidInput
	}

	// Get existing
	existing, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get WebSub API: %w", err)
	}
	if existing == nil {
		return nil, constants.ErrWebSubAPINotFound
	}

	transport := existing.Transport
	if req.Transport != nil && len(*req.Transport) > 0 {
		transport = make([]string, 0, len(*req.Transport))
		for _, t := range *req.Transport {
			transport = append(transport, string(t))
		}
	}

	lifeCycleStatus := existing.LifeCycleStatus
	if req.LifeCycleStatus != nil {
		lifeCycleStatus = string(*req.LifeCycleStatus)
	}

	var subscriptionPlans []string
	if req.SubscriptionPlans != nil {
		subscriptionPlans = *req.SubscriptionPlans
	}

	channels := req.Channels
	existing.Name = req.Name
	existing.Version = req.Version
	existing.Description = utils.ValueOrEmpty(req.Description)
	existing.LifeCycleStatus = lifeCycleStatus
	existing.Transport = transport
	existing.Configuration = model.WebSubAPIConfiguration{
		Name:              req.Name,
		Version:           req.Version,
		Context:           req.Context,
		Channels:          s.apiUtil.ChannelsAPIToModel(&channels),
		Upstream:          *mapUpstreamAPIToModel(req.Upstream),
		Policies:          mapMCPPoliciesAPIToModel(req.Policies),
		SubscriptionPlans: subscriptionPlans,
	}

	if err := s.repo.Update(existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrWebSubAPINotFound
		}
		return nil, fmt.Errorf("failed to update WebSub API: %w", err)
	}

	return s.Get(orgUUID, handle)
}

// Delete deletes a WebSub API by its handle
func (s *WebSubAPIService) Delete(orgUUID, handle string) error {
	if handle == "" {
		return constants.ErrInvalidInput
	}

	// Get the WebSub API UUID before deletion (needed for gateway deletion event)
	websubAPI, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to get WebSub API: %w", err)
	}
	if websubAPI == nil {
		return constants.ErrWebSubAPINotFound
	}

	// Get all gateways in the organization to broadcast deletion event
	var gateways []*model.Gateway
	if s.gatewayRepo != nil {
		gws, err := s.gatewayRepo.GetByOrganizationID(orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to get gateways for WebSub API deletion", "error", err, "apiUUID", websubAPI.UUID)
		} else {
			gateways = gws
		}
	}

	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrWebSubAPINotFound
		}
		return fmt.Errorf("failed to delete WebSub API: %w", err)
	}

	// Send deletion events to all gateways in the organization
	if s.gatewayEventsService != nil && len(gateways) > 0 {
		for _, gateway := range gateways {
			deletionEvent := &model.APIDeletionEvent{
				ApiId: websubAPI.UUID,
			}
			if err := s.gatewayEventsService.BroadcastAPIDeletionEvent(gateway.ID, deletionEvent); err != nil {
				s.slogger.Warn("Failed to broadcast WebSub API deletion event", "error", err, "gatewayID", gateway.ID, "apiUUID", websubAPI.UUID)
			} else {
				s.slogger.Info("WebSub API deletion event sent", "gatewayID", gateway.ID, "apiUUID", websubAPI.UUID)
			}
		}
	}

	return nil
}

// PublishToDevPortal publishes a WebSub API to a DevPortal
func (s *WebSubAPIService) PublishToDevPortal(orgUUID, handle string, req *api.PublishToDevPortalRequest) error {
	websubAPI, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to get WebSub API: %w", err)
	}
	if websubAPI == nil {
		return constants.ErrWebSubAPINotFound
	}

	// Build a RESTAPI adapter for the devportal service
	restAPIAdapter := websubAPIModelToRESTAPIAdapter(websubAPI)
	return s.devPortalService.PublishAPIToDevPortal(websubAPI.UUID, restAPIAdapter, req, orgUUID)
}

// UnpublishFromDevPortal unpublishes a WebSub API from a DevPortal
func (s *WebSubAPIService) UnpublishFromDevPortal(orgUUID, handle, devPortalUUID string) error {
	websubAPI, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to get WebSub API: %w", err)
	}
	if websubAPI == nil {
		return constants.ErrWebSubAPINotFound
	}

	return s.devPortalService.UnpublishAPIFromDevPortal(devPortalUUID, orgUUID, websubAPI.UUID)
}

// Count returns the total number of WebSub APIs for an organization
func (s *WebSubAPIService) Count(orgUUID string) (int, error) {
	return s.repo.Count(orgUUID)
}

// websubAPIModelToRESTAPIAdapter creates a minimal api.RESTAPI from model.WebSubAPI for DevPortal operations
func websubAPIModelToRESTAPIAdapter(m *model.WebSubAPI) *api.RESTAPI {
	handle := m.Handle
	desc := m.Description
	createdBy := m.CreatedBy
	ctx := ""
	if m.Configuration.Context != nil {
		ctx = *m.Configuration.Context
	}
	return &api.RESTAPI{
		Id:          &handle,
		Name:        m.Name,
		Version:     m.Version,
		Context:     ctx,
		Description: &desc,
		CreatedBy:   &createdBy,
		CreatedAt:   utils.TimePtrIfNotZero(m.CreatedAt),
		UpdatedAt:   utils.TimePtrIfNotZero(m.UpdatedAt),
	}
}

// mapWebSubAPIModelToAPI converts a model.WebSubAPI to api.WebSubAPI
func mapWebSubAPIModelToAPI(m *model.WebSubAPI, apiUtil *utils.APIUtil) *api.WebSubAPI {
	if m == nil {
		return nil
	}

	desc := m.Description
	createdBy := m.CreatedBy
	kind := constants.WebSubApi
	lifeCycleStatus := api.WebSubAPILifeCycleStatus(m.LifeCycleStatus)

	var transport *[]api.WebSubAPITransport
	if len(m.Transport) > 0 {
		items := make([]api.WebSubAPITransport, 0, len(m.Transport))
		for _, t := range m.Transport {
			items = append(items, api.WebSubAPITransport(t))
		}
		transport = &items
	}

	channels := apiUtil.ChannelsModelToAPI(m.Configuration.Channels)
	var channelsSlice []api.Channel
	if channels != nil {
		channelsSlice = *channels
	}

	var subscriptionPlans *[]string
	if len(m.Configuration.SubscriptionPlans) > 0 {
		subscriptionPlans = &m.Configuration.SubscriptionPlans
	}

	result := &api.WebSubAPI{
		Id:                m.Handle,
		Name:              m.Name,
		Version:           m.Version,
		ProjectId:         m.ProjectUUID,
		Description:       &desc,
		CreatedBy:         &createdBy,
		Kind:              &kind,
		LifeCycleStatus:   &lifeCycleStatus,
		Transport:         transport,
		Context:           m.Configuration.Context,
		Upstream:          mapUpstreamModelToAPI(&m.Configuration.Upstream),
		Channels:          channelsSlice,
		Policies:          mapMCPPoliciesModelToAPI(m.Configuration.Policies),
		SubscriptionPlans: subscriptionPlans,
		CreatedAt:         utils.TimePtr(m.CreatedAt),
		UpdatedAt:         utils.TimePtr(m.UpdatedAt),
	}

	return result
}

// mapWebSubAPIModelToListItem converts a model.WebSubAPI to api.WebSubAPIListItem
func mapWebSubAPIModelToListItem(m *model.WebSubAPI) *api.WebSubAPIListItem {
	if m == nil {
		return nil
	}

	lifeCycleStatus := api.WebSubAPIListItemLifeCycleStatus(m.LifeCycleStatus)

	return &api.WebSubAPIListItem{
		Id:              utils.StringPtrIfNotEmpty(m.Handle),
		Name:            utils.StringPtrIfNotEmpty(m.Name),
		Version:         utils.StringPtrIfNotEmpty(m.Version),
		ProjectId:       utils.StringPtrIfNotEmpty(m.ProjectUUID),
		Context:         m.Configuration.Context,
		LifeCycleStatus: &lifeCycleStatus,
		CreatedAt:       utils.TimePtr(m.CreatedAt),
		UpdatedAt:       utils.TimePtr(m.UpdatedAt),
	}
}

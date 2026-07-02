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
	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	coreservice "platform-api/src/internal/service"
	"platform-api/src/internal/utils"
)

// WebSubAPIService handles business logic for WebSub API operations
type WebSubAPIService struct {
	repo                 repository.WebSubAPIRepository
	projectRepo          repository.ProjectRepository
	gatewayRepo          repository.GatewayRepository
	gatewayEventsService *coreservice.GatewayEventsService
	apiUtil              *utils.APIUtil
	slogger              *slog.Logger
	auditRepo            repository.AuditRepository
	cfg                  *config.Server
	identity             *coreservice.IdentityService
}

// NewWebSubAPIService creates a new WebSubAPIService instance
func NewWebSubAPIService(
	repo repository.WebSubAPIRepository,
	projectRepo repository.ProjectRepository,
	gatewayRepo repository.GatewayRepository,
	gatewayEventsService *coreservice.GatewayEventsService,
	apiUtil *utils.APIUtil,
	slogger *slog.Logger,
	auditRepo repository.AuditRepository,
	cfg *config.Server,
	identity *coreservice.IdentityService,
) *WebSubAPIService {
	return &WebSubAPIService{
		repo:                 repo,
		projectRepo:          projectRepo,
		gatewayRepo:          gatewayRepo,
		gatewayEventsService: gatewayEventsService,
		apiUtil:              apiUtil,
		slogger:              slogger,
		auditRepo:            auditRepo,
		cfg:                  cfg,
		identity:             identity,
	}
}

// toWebSubAPI converts m via mapWebSubAPIModelToAPI and resolves its
// createdBy/updatedBy UUIDs to their raw external identity.
func (s *WebSubAPIService) toWebSubAPI(m *model.WebSubAPI) (*api.WebSubAPI, error) {
	resp := mapWebSubAPIModelToAPI(m, s.apiUtil)
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

// webSubAPIListItemResolved converts m via mapWebSubAPIModelToListItem and
// resolves its createdBy UUID to its raw external identity.
func (s *WebSubAPIService) webSubAPIListItemResolved(m *model.WebSubAPI) (*api.WebSubAPIListItem, error) {
	item := mapWebSubAPIModelToListItem(m)
	if item == nil {
		return nil, nil
	}
	if err := s.identity.ResolveIdentityField(&item.CreatedBy); err != nil {
		return nil, err
	}
	return item, nil
}

// Create creates a new WebSub API
func (s *WebSubAPIService) Create(orgUUID, createdBy string, req *api.WebSubAPI) (*api.WebSubAPI, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if utils.ValueOrEmpty(req.Id) == "" || req.DisplayName == "" || req.Version == "" {
		return nil, constants.ErrInvalidInput
	}
	if req.ProjectId == "" {
		return nil, constants.ErrInvalidInput
	}

	handle := utils.ValueOrEmpty(req.Id)

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
	exists, err := s.repo.Exists(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check WebSub API exists: %w", err)
	}
	if exists {
		return nil, constants.ErrWebSubAPIExists
	}

	// Enforce the per-organization WebSub API limit (unlimited when not configured).
	count, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count existing WebSub APIs: %w", err)
	}
	if config.LimitReached(count, s.cfg.ArtifactLimits.MaxWebSubAPIsPerOrg) {
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

	m := &model.WebSubAPI{
		Handle:           handle,
		OrganizationUUID: orgUUID,
		ProjectUUID:      req.ProjectId,
		Name:             req.DisplayName,
		Description:      utils.ValueOrEmpty(req.Description),
		CreatedBy:        createdBy,
		Version:          req.Version,
		LifeCycleStatus:  lifeCycleStatus,
		Origin:           constants.OriginCP,
		Configuration: model.WebSubAPIConfiguration{
			Name:              req.DisplayName,
			Version:           req.Version,
			Context:           req.Context,
			Transport:         transport,
			Channels:          mapWebSubChannelsAPIToModel(&req.Channels),
			Upstream:          *mapUpstreamAPIToModel(req.Upstream),
			AllChannels:       mapWebSubAllChannelPoliciesAPIToModel(req.AllChannels),
			SubscriptionPlans: subscriptionPlans,
		},
	}

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrWebSubAPIExists
		}
		return nil, fmt.Errorf("failed to create WebSub API: %w", err)
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("CREATE", m.UUID, "websub_api", orgUUID, createdBy)
	}
	return s.Get(orgUUID, handle)
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

	return s.toWebSubAPI(m)
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
		item, err := s.webSubAPIListItemResolved(a)
		if err != nil {
			return nil, err
		}
		if item != nil {
			resp.List = append(resp.List, *item)
		}
	}

	return resp, nil
}

// Update updates an existing WebSub API
func (s *WebSubAPIService) Update(orgUUID, handle, updatedBy string, req *api.WebSubAPI) (*api.WebSubAPI, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id != nil && *req.Id != "" && *req.Id != handle {
		return nil, constants.ErrHandleImmutable
	}
	if req.DisplayName == "" || req.Version == "" {
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
	// DP-originated artifacts are read-only in the control plane.
	if err := ensureOriginMutable(existing.Origin); err != nil {
		return nil, err
	}

	transport := existing.Configuration.Transport
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

	existing.Name = req.DisplayName
	existing.Version = req.Version
	existing.Description = utils.ValueOrEmpty(req.Description)
	existing.UpdatedBy = updatedBy
	existing.LifeCycleStatus = lifeCycleStatus
	existing.Configuration = model.WebSubAPIConfiguration{
		Name:              req.DisplayName,
		Version:           req.Version,
		Context:           req.Context,
		Transport:         transport,
		Channels:          mapWebSubChannelsAPIToModel(&req.Channels),
		Upstream:          *mapUpstreamAPIToModel(req.Upstream),
		AllChannels:       mapWebSubAllChannelPoliciesAPIToModel(req.AllChannels),
		SubscriptionPlans: subscriptionPlans,
	}

	if err := s.repo.Update(existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrWebSubAPINotFound
		}
		return nil, fmt.Errorf("failed to update WebSub API: %w", err)
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("UPDATE", existing.UUID, "websub_api", orgUUID, updatedBy)
	}
	return s.Get(orgUUID, handle)
}

// Delete deletes a WebSub API by its handle
func (s *WebSubAPIService) Delete(orgUUID, handle, deletedBy string) error {
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
	// DP-originated artifacts are read-only in the control plane and cannot be deleted from the CP.
	if err := ensureOriginMutable(websubAPI.Origin); err != nil {
		return err
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

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("DELETE", websubAPI.UUID, "websub_api", orgUUID, deletedBy)
	}
	// Send deletion events to all gateways in the organization
	if s.gatewayEventsService != nil && len(gateways) > 0 {
		for _, gateway := range gateways {
			deletionEvent := &model.WebSubAPIDeletionEvent{
				ApiId: websubAPI.UUID,
			}
			if err := s.gatewayEventsService.BroadcastWebSubAPIDeletionEvent(gateway.ID, deletionEvent); err != nil {
				s.slogger.Warn("Failed to broadcast WebSub API deletion event", "error", err, "gatewayID", gateway.ID, "apiUUID", websubAPI.UUID)
			} else {
				s.slogger.Info("WebSub API deletion event sent", "gatewayID", gateway.ID, "apiUUID", websubAPI.UUID)
			}
		}
	}

	return nil
}

// Count returns the total number of WebSub APIs for an organization
func (s *WebSubAPIService) Count(orgUUID string) (int, error) {
	return s.repo.Count(orgUUID)
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
	if len(m.Configuration.Transport) > 0 {
		items := make([]api.WebSubAPITransport, 0, len(m.Configuration.Transport))
		for _, t := range m.Configuration.Transport {
			items = append(items, api.WebSubAPITransport(t))
		}
		transport = &items
	}

	var subscriptionPlans *[]string
	if len(m.Configuration.SubscriptionPlans) > 0 {
		subscriptionPlans = &m.Configuration.SubscriptionPlans
	}

	result := &api.WebSubAPI{
		Id:                utils.StringPtrIfNotEmpty(m.Handle),
		DisplayName:       m.Name,
		Version:           m.Version,
		ProjectId:         m.ProjectUUID,
		Description:       &desc,
		CreatedBy:         &createdBy,
		Kind:              &kind,
		LifeCycleStatus:   &lifeCycleStatus,
		Transport:         transport,
		Context:           m.Configuration.Context,
		Upstream:          mapUpstreamModelToAPI(&m.Configuration.Upstream),
		Channels:          *mapWebSubChannelsModelToAPI(m.Configuration.Channels),
		AllChannels:       mapWebSubAllChannelPoliciesModelToAPI(m.Configuration.AllChannels),
		SubscriptionPlans: subscriptionPlans,
		ReadOnly:          utils.BoolPtr(m.Origin == constants.OriginDP),
		CreatedAt:         utils.TimePtr(m.CreatedAt),
		UpdatedAt:         utils.TimePtr(m.UpdatedAt),
		UpdatedBy:         utils.StringPtrIfNotEmpty(m.UpdatedBy),
	}

	return result
}

// mapWebSubChannelsAPIToModel converts the API channel map to the model channel map.
func mapWebSubChannelsAPIToModel(in *map[string]api.WebSubChannel) map[string]model.WebSubChannel {
	if in == nil {
		return nil
	}
	out := make(map[string]model.WebSubChannel, len(*in))
	for name, ch := range *in {
		out[name] = model.WebSubChannel{
			OnSubscription:    mapEventPoliciesAPIToModel(ch.OnSubscription),
			OnUnsubscription:  mapEventPoliciesAPIToModel(ch.OnUnsubscription),
			OnMessageReceived: mapEventPoliciesAPIToModel(ch.OnMessageReceived),
			OnMessageDelivery: mapEventPoliciesAPIToModel(ch.OnMessageDelivery),
		}
	}
	return out
}

// mapEventPoliciesAPIToModel converts API event policies to model.
func mapEventPoliciesAPIToModel(in *api.WebSubEventPolicies) *model.WebSubEventPolicies {
	if in == nil {
		return nil
	}
	return &model.WebSubEventPolicies{
		Policies: mapAPIPolicySliceToModel(in.Policies),
	}
}

// mapWebSubAllChannelPoliciesAPIToModel converts API all-channel policies to model.
func mapWebSubAllChannelPoliciesAPIToModel(in *api.WebSubAllChannelPolicies) *model.WebSubAllChannelPolicies {
	if in == nil {
		return nil
	}
	return &model.WebSubAllChannelPolicies{
		OnSubscription:    mapEventPoliciesAPIToModel(in.OnSubscription),
		OnUnsubscription:  mapEventPoliciesAPIToModel(in.OnUnsubscription),
		OnMessageReceived: mapEventPoliciesAPIToModel(in.OnMessageReceived),
		OnMessageDelivery: mapEventPoliciesAPIToModel(in.OnMessageDelivery),
	}
}

// mapWebSubChannelsModelToAPI converts the model channel map to the API channel map.
func mapWebSubChannelsModelToAPI(in map[string]model.WebSubChannel) *map[string]api.WebSubChannel {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]api.WebSubChannel, len(in))
	for name, ch := range in {
		out[name] = api.WebSubChannel{
			OnSubscription:    mapEventPoliciesModelToAPI(ch.OnSubscription),
			OnUnsubscription:  mapEventPoliciesModelToAPI(ch.OnUnsubscription),
			OnMessageReceived: mapEventPoliciesModelToAPI(ch.OnMessageReceived),
			OnMessageDelivery: mapEventPoliciesModelToAPI(ch.OnMessageDelivery),
		}
	}
	return &out
}

// mapEventPoliciesModelToAPI converts model event policies to API.
func mapEventPoliciesModelToAPI(in *model.WebSubEventPolicies) *api.WebSubEventPolicies {
	if in == nil {
		return nil
	}
	return &api.WebSubEventPolicies{
		Policies: mapModelPolicySliceToAPI(in.Policies),
	}
}

// mapWebSubAllChannelPoliciesModelToAPI converts model all-channel policies to API.
func mapWebSubAllChannelPoliciesModelToAPI(in *model.WebSubAllChannelPolicies) *api.WebSubAllChannelPolicies {
	if in == nil {
		return nil
	}
	return &api.WebSubAllChannelPolicies{
		OnSubscription:    mapEventPoliciesModelToAPI(in.OnSubscription),
		OnUnsubscription:  mapEventPoliciesModelToAPI(in.OnUnsubscription),
		OnMessageReceived: mapEventPoliciesModelToAPI(in.OnMessageReceived),
		OnMessageDelivery: mapEventPoliciesModelToAPI(in.OnMessageDelivery),
	}
}

// mapAPIPolicySliceToModel converts a pointer to API policy slice to a model policy slice.
func mapAPIPolicySliceToModel(in *[]api.Policy) []model.Policy {
	if in == nil {
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

// mapModelPolicySliceToAPI converts a model policy slice to a pointer to API policy slice.
func mapModelPolicySliceToAPI(in []model.Policy) *[]api.Policy {
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

// mapWebSubAPIModelToListItem converts a model.WebSubAPI to api.WebSubAPIListItem
func mapWebSubAPIModelToListItem(m *model.WebSubAPI) *api.WebSubAPIListItem {
	if m == nil {
		return nil
	}

	lifeCycleStatus := api.WebSubAPIListItemLifeCycleStatus(m.LifeCycleStatus)

	return &api.WebSubAPIListItem{
		Id:              utils.StringPtrIfNotEmpty(m.Handle),
		DisplayName:     m.Name,
		Version:         utils.StringPtrIfNotEmpty(m.Version),
		ProjectId:       utils.StringPtrIfNotEmpty(m.ProjectUUID),
		Context:         m.Configuration.Context,
		LifeCycleStatus: &lifeCycleStatus,
		ReadOnly:        utils.BoolPtr(m.Origin == constants.OriginDP),
		CreatedBy:       utils.StringPtrIfNotEmpty(m.CreatedBy),
		CreatedAt:       utils.TimePtr(m.CreatedAt),
		UpdatedAt:       utils.TimePtr(m.UpdatedAt),
	}
}

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
	coreservice "platform-api/src/internal/service"
	"platform-api/src/internal/utils"
)

// WebBrokerAPIService handles business logic for WebBroker API operations
type WebBrokerAPIService struct {
	repo                 repository.WebBrokerAPIRepository
	projectRepo          repository.ProjectRepository
	gatewayRepo          repository.GatewayRepository
	gatewayEventsService *coreservice.GatewayEventsService
	apiUtil              *utils.APIUtil
	slogger              *slog.Logger
	auditRepo            repository.AuditRepository
}

// NewWebBrokerAPIService creates a new WebBrokerAPIService instance
func NewWebBrokerAPIService(
	repo repository.WebBrokerAPIRepository,
	projectRepo repository.ProjectRepository,
	gatewayRepo repository.GatewayRepository,
	gatewayEventsService *coreservice.GatewayEventsService,
	apiUtil *utils.APIUtil,
	slogger *slog.Logger,
	auditRepo repository.AuditRepository,
) *WebBrokerAPIService {
	return &WebBrokerAPIService{
		repo:                 repo,
		projectRepo:          projectRepo,
		gatewayRepo:          gatewayRepo,
		gatewayEventsService: gatewayEventsService,
		apiUtil:              apiUtil,
		slogger:              slogger,
		auditRepo:            auditRepo,
	}
}

// Create creates a new WebBroker API
func (s *WebBrokerAPIService) Create(orgUUID, createdBy string, req *api.WebBrokerAPI) (*api.WebBrokerAPI, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if utils.ValueOrEmpty(req.Id) == "" || req.Name == "" || req.Version == "" {
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
		return nil, fmt.Errorf("failed to check WebBroker API exists: %w", err)
	}
	if exists {
		return nil, constants.ErrWebBrokerAPIExists
	}

	// Check org limit
	count, err := s.repo.Count(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count existing WebBroker APIs: %w", err)
	}
	if count >= constants.MaxWebBrokerAPIsPerOrganization {
		return nil, constants.ErrWebBrokerAPILimitReached
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

	m := &model.WebBrokerAPI{
		Handle:           handle,
		OrganizationUUID: orgUUID,
		ProjectUUID:      req.ProjectId,
		Name:             req.Name,
		Description:      utils.ValueOrEmpty(req.Description),
		CreatedBy:        createdBy,
		Version:          req.Version,
		LifeCycleStatus:  lifeCycleStatus,
		Configuration: model.WebBrokerAPIConfiguration{
			Name:              req.Name,
			Version:           req.Version,
			Context:           req.Context,
			Transport:         transport,
			Channels:          mapWebBrokerChannelsAPIToModel(req.Channels),
			Receiver:          mapWebBrokerReceiverAPIToModel(req.Receiver),
			Broker:            mapWebBrokerBrokerAPIToModel(req.Broker),
			AllChannels:       mapWebBrokerAllChannelPoliciesAPIToModel(req.AllChannels),
			SubscriptionPlans: subscriptionPlans,
		},
		Origin: constants.OriginCP,
	}

	if err := s.repo.Create(m); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return nil, constants.ErrWebBrokerAPIExists
		}
		return nil, fmt.Errorf("failed to create WebBroker API: %w", err)
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("CREATE", m.UUID, "webbroker_api", orgUUID, createdBy)
	}
	return s.Get(orgUUID, handle)
}

// Get retrieves a WebBroker API by its handle
func (s *WebBrokerAPIService) Get(orgUUID, handle string) (*api.WebBrokerAPI, error) {
	if handle == "" {
		return nil, constants.ErrInvalidInput
	}

	m, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get WebBroker API: %w", err)
	}
	if m == nil {
		return nil, constants.ErrWebBrokerAPINotFound
	}

	return mapWebBrokerAPIModelToAPI(m, s.apiUtil), nil
}

// List retrieves WebBroker APIs for an organization filtered by project
func (s *WebBrokerAPIService) List(orgUUID, projectUUID string, limit, offset int) (*api.WebBrokerAPIListResponse, error) {
	apis, err := s.repo.List(orgUUID, projectUUID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list WebBroker APIs: %w", err)
	}

	var totalCount int
	if projectUUID != "" {
		totalCount, err = s.repo.CountByProject(orgUUID, projectUUID)
	} else {
		totalCount, err = s.repo.Count(orgUUID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to count WebBroker APIs: %w", err)
	}

	resp := &api.WebBrokerAPIListResponse{
		Count: len(apis),
		Pagination: api.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}

	resp.List = make([]api.WebBrokerAPIListItem, 0, len(apis))
	for _, a := range apis {
		resp.List = append(resp.List, *mapWebBrokerAPIModelToListItem(a))
	}

	return resp, nil
}

// Update updates an existing WebBroker API
func (s *WebBrokerAPIService) Update(orgUUID, handle, updatedBy string, req *api.WebBrokerAPI) (*api.WebBrokerAPI, error) {
	if handle == "" || req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Id != nil && *req.Id != "" && *req.Id != handle {
		return nil, constants.ErrInvalidInput
	}
	if req.Name == "" || req.Version == "" {
		return nil, constants.ErrInvalidInput
	}
	// Get existing
	existing, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get WebBroker API: %w", err)
	}
	if existing == nil {
		return nil, constants.ErrWebBrokerAPINotFound
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

	existing.Name = req.Name
	existing.Version = req.Version
	existing.Description = utils.ValueOrEmpty(req.Description)
	existing.UpdatedBy = updatedBy
	existing.LifeCycleStatus = lifeCycleStatus
	existing.Configuration = model.WebBrokerAPIConfiguration{
		Name:              req.Name,
		Version:           req.Version,
		Context:           req.Context,
		Transport:         transport,
		Channels:          mapWebBrokerChannelsAPIToModel(req.Channels),
		Receiver:          mapWebBrokerReceiverAPIToModel(req.Receiver),
		Broker:            mapWebBrokerBrokerAPIToModel(req.Broker),
		AllChannels:       mapWebBrokerAllChannelPoliciesAPIToModel(req.AllChannels),
		SubscriptionPlans: subscriptionPlans,
	}

	if err := s.repo.Update(existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrWebBrokerAPINotFound
		}
		return nil, fmt.Errorf("failed to update WebBroker API: %w", err)
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("UPDATE", existing.UUID, "webbroker_api", orgUUID, updatedBy)
	}
	return s.Get(orgUUID, handle)
}

// Delete deletes a WebBroker API by its handle
func (s *WebBrokerAPIService) Delete(orgUUID, handle, deletedBy string) error {
	if handle == "" {
		return constants.ErrInvalidInput
	}

	// Get the WebBroker API UUID before deletion (needed for gateway deletion event)
	webbrokerAPI, err := s.repo.GetByHandle(handle, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to get WebBroker API: %w", err)
	}
	if webbrokerAPI == nil {
		return constants.ErrWebBrokerAPINotFound
	}
	// DP-originated artifacts are read-only in the control plane and cannot be deleted from the CP.
	if err := ensureOriginMutable(webbrokerAPI.Origin); err != nil {
		return err
	}

	// Get all gateways in the organization to broadcast deletion event
	var gateways []*model.Gateway
	if s.gatewayRepo != nil {
		gws, err := s.gatewayRepo.GetByOrganizationID(orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to get gateways for WebBroker API deletion", "error", err, "apiUUID", webbrokerAPI.UUID)
		} else {
			gateways = gws
		}
	}

	if err := s.repo.Delete(handle, orgUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrWebBrokerAPINotFound
		}
		return fmt.Errorf("failed to delete WebBroker API: %w", err)
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("DELETE", webbrokerAPI.UUID, "webbroker_api", orgUUID, deletedBy)
	}
	// Send deletion events to all gateways in the organization
	if s.gatewayEventsService != nil && len(gateways) > 0 {
		for _, gateway := range gateways {
			deletionEvent := &model.WebBrokerAPIDeletionEvent{
				ApiId: webbrokerAPI.UUID,
			}
			if err := s.gatewayEventsService.BroadcastWebBrokerAPIDeletionEvent(gateway.ID, deletionEvent); err != nil {
				s.slogger.Warn("Failed to broadcast WebBroker API deletion event", "error", err, "gatewayID", gateway.ID, "apiUUID", webbrokerAPI.UUID)
			} else {
				s.slogger.Info("WebBroker API deletion event sent", "gatewayID", gateway.ID, "apiUUID", webbrokerAPI.UUID)
			}
		}
	}

	return nil
}

// Count returns the total number of WebBroker APIs for an organization
func (s *WebBrokerAPIService) Count(orgUUID string) (int, error) {
	return s.repo.Count(orgUUID)
}

// mapWebBrokerAPIModelToAPI converts a model.WebBrokerAPI to api.WebBrokerAPI
func mapWebBrokerAPIModelToAPI(m *model.WebBrokerAPI, apiUtil *utils.APIUtil) *api.WebBrokerAPI {
	if m == nil {
		return nil
	}

	desc := m.Description
	createdBy := m.CreatedBy
	kind := constants.WebBrokerApi
	lifeCycleStatus := api.WebBrokerAPILifeCycleStatus(m.LifeCycleStatus)

	var transport *[]api.WebBrokerAPITransport
	if len(m.Configuration.Transport) > 0 {
		items := make([]api.WebBrokerAPITransport, 0, len(m.Configuration.Transport))
		for _, t := range m.Configuration.Transport {
			items = append(items, api.WebBrokerAPITransport(t))
		}
		transport = &items
	}

	var subscriptionPlans *[]string
	if len(m.Configuration.SubscriptionPlans) > 0 {
		subscriptionPlans = &m.Configuration.SubscriptionPlans
	}

	result := &api.WebBrokerAPI{
		Id:                utils.StringPtrIfNotEmpty(m.Handle),
		Name:              m.Name,
		Version:           m.Version,
		ProjectId:         m.ProjectUUID,
		Description:       &desc,
		CreatedBy:         &createdBy,
		Kind:              &kind,
		LifeCycleStatus:   &lifeCycleStatus,
		Transport:         transport,
		Context:           m.Configuration.Context,
		Receiver:          mapWebBrokerReceiverModelToAPI(m.Configuration.Receiver),
		Broker:            mapWebBrokerBrokerModelToAPI(m.Configuration.Broker),
		Channels:          mapWebBrokerChannelsModelToAPI(m.Configuration.Channels),
		AllChannels:       mapWebBrokerAllChannelPoliciesModelToAPI(m.Configuration.AllChannels),
		SubscriptionPlans: subscriptionPlans,
		ReadOnly:          utils.BoolPtr(m.Origin == constants.OriginDP),
		CreatedAt:         utils.TimePtr(m.CreatedAt),
		UpdatedAt:         utils.TimePtr(m.UpdatedAt),
	}

	return result
}

// mapWebBrokerReceiverAPIToModel converts API receiver to model receiver
func mapWebBrokerReceiverAPIToModel(in struct {
	Name       string                       `json:"name" yaml:"name"`
	Properties *map[string]interface{}      `json:"properties,omitempty" yaml:"properties,omitempty"`
	Type       api.WebBrokerAPIReceiverType `json:"type" yaml:"type"`
}) model.WebBrokerReceiver {
	r := model.WebBrokerReceiver{
		Name: in.Name,
		Type: string(in.Type),
	}
	if in.Properties != nil {
		r.Properties = *in.Properties
	}
	return r
}

// mapWebBrokerReceiverModelToAPI converts model receiver to API receiver
func mapWebBrokerReceiverModelToAPI(in model.WebBrokerReceiver) struct {
	Name       string                       `json:"name" yaml:"name"`
	Properties *map[string]interface{}      `json:"properties,omitempty" yaml:"properties,omitempty"`
	Type       api.WebBrokerAPIReceiverType `json:"type" yaml:"type"`
} {
	out := struct {
		Name       string                       `json:"name" yaml:"name"`
		Properties *map[string]interface{}      `json:"properties,omitempty" yaml:"properties,omitempty"`
		Type       api.WebBrokerAPIReceiverType `json:"type" yaml:"type"`
	}{
		Name: in.Name,
		Type: api.WebBrokerAPIReceiverType(in.Type),
	}
	if in.Properties != nil {
		out.Properties = &in.Properties
	}
	return out
}

// mapWebBrokerBrokerAPIToModel converts API broker to model broker
func mapWebBrokerBrokerAPIToModel(in struct {
	Name       string                     `json:"name" yaml:"name"`
	Properties *map[string]interface{}    `json:"properties,omitempty" yaml:"properties,omitempty"`
	Type       api.WebBrokerAPIBrokerType `json:"type" yaml:"type"`
}) model.WebBrokerBroker {
	var properties map[string]interface{}
	if in.Properties != nil {
		properties = *in.Properties
	}
	return model.WebBrokerBroker{
		Name:       in.Name,
		Type:       string(in.Type),
		Properties: properties,
	}
}

// mapWebBrokerBrokerModelToAPI converts model broker to API broker
func mapWebBrokerBrokerModelToAPI(in model.WebBrokerBroker) struct {
	Name       string                     `json:"name" yaml:"name"`
	Properties *map[string]interface{}    `json:"properties,omitempty" yaml:"properties,omitempty"`
	Type       api.WebBrokerAPIBrokerType `json:"type" yaml:"type"`
} {
	var properties *map[string]interface{}
	if len(in.Properties) > 0 {
		properties = &in.Properties
	}
	return struct {
		Name       string                     `json:"name" yaml:"name"`
		Properties *map[string]interface{}    `json:"properties,omitempty" yaml:"properties,omitempty"`
		Type       api.WebBrokerAPIBrokerType `json:"type" yaml:"type"`
	}{
		Name:       in.Name,
		Type:       api.WebBrokerAPIBrokerType(in.Type),
		Properties: properties,
	}
}

// mapWebBrokerChannelsAPIToModel converts the API channel map to the model channel map.
func mapWebBrokerChannelsAPIToModel(in map[string]api.WebBrokerChannel) map[string]model.WebBrokerChannel {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]model.WebBrokerChannel, len(in))
	for name, ch := range in {
		modelCh := model.WebBrokerChannel{
			OnConnectionInit: mapWebBrokerEventPoliciesAPIToModel(ch.OnConnectionInit),
			OnProduce:        mapWebBrokerEventPoliciesAPIToModel(ch.OnProduce),
			OnConsume:        mapWebBrokerEventPoliciesAPIToModel(ch.OnConsume),
		}
		if ch.ProduceTo != nil && ch.ProduceTo.Topic != nil {
			modelCh.ProduceTo = &model.WebBrokerTopic{
				Topic: *ch.ProduceTo.Topic,
			}
		}
		if ch.ConsumeFrom != nil && ch.ConsumeFrom.Topic != nil {
			modelCh.ConsumeFrom = &model.WebBrokerTopic{
				Topic: *ch.ConsumeFrom.Topic,
			}
		}
		out[name] = modelCh
	}
	return out
}

// mapWebBrokerEventPoliciesAPIToModel converts API event policies to model.
func mapWebBrokerEventPoliciesAPIToModel(in *api.WebBrokerEventPolicies) *model.WebBrokerEventPolicies {
	if in == nil {
		return nil
	}
	return &model.WebBrokerEventPolicies{
		Policies: mapAPIPolicySliceToModel(in.Policies),
	}
}

// mapWebBrokerAllChannelPoliciesAPIToModel converts API all-channel policies to model.
func mapWebBrokerAllChannelPoliciesAPIToModel(in *api.WebBrokerAllChannelPolicies) *model.WebBrokerAllChannelPolicies {
	if in == nil {
		return nil
	}
	return &model.WebBrokerAllChannelPolicies{
		OnConnectionInit: mapWebBrokerEventPoliciesAPIToModel(in.OnConnectionInit),
		OnProduce:        mapWebBrokerEventPoliciesAPIToModel(in.OnProduce),
		OnConsume:        mapWebBrokerEventPoliciesAPIToModel(in.OnConsume),
	}
}

// mapWebBrokerChannelsModelToAPI converts the model channel map to the API channel map.
func mapWebBrokerChannelsModelToAPI(in map[string]model.WebBrokerChannel) map[string]api.WebBrokerChannel {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]api.WebBrokerChannel, len(in))
	for name, ch := range in {
		apiCh := api.WebBrokerChannel{
			OnConnectionInit: mapWebBrokerEventPoliciesModelToAPI(ch.OnConnectionInit),
			OnProduce:        mapWebBrokerEventPoliciesModelToAPI(ch.OnProduce),
			OnConsume:        mapWebBrokerEventPoliciesModelToAPI(ch.OnConsume),
		}
		if ch.ProduceTo != nil {
			topic := ch.ProduceTo.Topic
			apiCh.ProduceTo = &struct {
				Topic *string `json:"topic,omitempty" yaml:"topic,omitempty"`
			}{
				Topic: &topic,
			}
		}
		if ch.ConsumeFrom != nil {
			topic := ch.ConsumeFrom.Topic
			apiCh.ConsumeFrom = &struct {
				Topic *string `json:"topic,omitempty" yaml:"topic,omitempty"`
			}{
				Topic: &topic,
			}
		}
		out[name] = apiCh
	}
	return out
}

// mapWebBrokerEventPoliciesModelToAPI converts model event policies to API.
func mapWebBrokerEventPoliciesModelToAPI(in *model.WebBrokerEventPolicies) *api.WebBrokerEventPolicies {
	if in == nil {
		return nil
	}
	return &api.WebBrokerEventPolicies{
		Policies: mapModelPolicySliceToAPI(in.Policies),
	}
}

// mapWebBrokerAllChannelPoliciesModelToAPI converts model all-channel policies to API.
func mapWebBrokerAllChannelPoliciesModelToAPI(in *model.WebBrokerAllChannelPolicies) *api.WebBrokerAllChannelPolicies {
	if in == nil {
		return nil
	}
	return &api.WebBrokerAllChannelPolicies{
		OnConnectionInit: mapWebBrokerEventPoliciesModelToAPI(in.OnConnectionInit),
		OnProduce:        mapWebBrokerEventPoliciesModelToAPI(in.OnProduce),
		OnConsume:        mapWebBrokerEventPoliciesModelToAPI(in.OnConsume),
	}
}

// mapWebBrokerAPIModelToListItem converts a model.WebBrokerAPI to api.WebBrokerAPIListItem
func mapWebBrokerAPIModelToListItem(m *model.WebBrokerAPI) *api.WebBrokerAPIListItem {
	if m == nil {
		return nil
	}

	lifeCycleStatus := api.WebBrokerAPIListItemLifeCycleStatus(m.LifeCycleStatus)

	return &api.WebBrokerAPIListItem{
		Id:              utils.StringPtrIfNotEmpty(m.Handle),
		Name:            utils.StringPtrIfNotEmpty(m.Name),
		Version:         utils.StringPtrIfNotEmpty(m.Version),
		ProjectId:       utils.StringPtrIfNotEmpty(m.ProjectUUID),
		Context:         m.Configuration.Context,
		LifeCycleStatus: &lifeCycleStatus,
		ReadOnly:        utils.BoolPtr(m.Origin == constants.OriginDP),
		CreatedAt:       utils.TimePtr(m.CreatedAt),
		UpdatedAt:       utils.TimePtr(m.UpdatedAt),
	}
}

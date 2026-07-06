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
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	coreservice "github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"gopkg.in/yaml.v3"
)

// WebBrokerAPIDeploymentService handles deployment operations for WebBroker APIs
type WebBrokerAPIDeploymentService struct {
	artifactRepo         repository.ArtifactRepository
	apiRepo              repository.APIRepository
	webbrokerAPIRepo     repository.WebBrokerAPIRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	gatewayEventsService *coreservice.GatewayEventsService
	cfg                  *config.Server
	slogger              *slog.Logger
}

// NewWebBrokerAPIDeploymentService creates a new WebBrokerAPIDeploymentService
func NewWebBrokerAPIDeploymentService(
	webbrokerAPIRepo repository.WebBrokerAPIRepository,
	deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository,
	artifactRepo repository.ArtifactRepository,
	apiRepo repository.APIRepository,
	gatewayEventsService *coreservice.GatewayEventsService,
	cfg *config.Server,
	slogger *slog.Logger,
) *WebBrokerAPIDeploymentService {
	return &WebBrokerAPIDeploymentService{
		webbrokerAPIRepo:     webbrokerAPIRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		orgRepo:              orgRepo,
		artifactRepo:         artifactRepo,
		apiRepo:              apiRepo,
		gatewayEventsService: gatewayEventsService,
		cfg:                  cfg,
		slogger:              slogger,
	}
}

// DeployWebBrokerAPIByHandle creates a new immutable deployment using WebBroker API handle
func (s *WebBrokerAPIDeploymentService) DeployWebBrokerAPIByHandle(apiHandle string, req *api.DeployRequest, orgUUID, createdBy string) (*api.DeploymentResponse, error) {
	apiUUID, err := s.getWebBrokerAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.deployWebBrokerAPI(apiUUID, req, orgUUID, createdBy)
}

// RestoreWebBrokerAPIDeploymentByHandle restores a previous deployment using WebBroker API handle
func (s *WebBrokerAPIDeploymentService) RestoreWebBrokerAPIDeploymentByHandle(apiHandle, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	apiUUID, err := s.getWebBrokerAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.restoreWebBrokerAPIDeployment(apiUUID, &deploymentID, &gatewayID, orgUUID)
}

// UndeployWebBrokerAPIDeploymentByHandle undeploys a deployment using WebBroker API handle
func (s *WebBrokerAPIDeploymentService) UndeployWebBrokerAPIDeploymentByHandle(apiHandle, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	apiUUID, err := s.getWebBrokerAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.undeployWebBrokerAPIDeployment(apiUUID, &deploymentID, &gatewayID, orgUUID)
}

// DeleteWebBrokerAPIDeploymentByHandle deletes a deployment using WebBroker API handle
func (s *WebBrokerAPIDeploymentService) DeleteWebBrokerAPIDeploymentByHandle(apiHandle, deploymentID, orgUUID string) error {
	apiUUID, err := s.getWebBrokerAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return err
	}
	return s.deleteWebBrokerAPIDeployment(apiUUID, deploymentID, orgUUID)
}

// GetWebBrokerAPIDeploymentByHandle retrieves a single deployment using WebBroker API handle
func (s *WebBrokerAPIDeploymentService) GetWebBrokerAPIDeploymentByHandle(apiHandle, deploymentID, orgUUID string) (*api.DeploymentResponse, error) {
	apiUUID, err := s.getWebBrokerAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.getWebBrokerAPIDeployment(apiUUID, deploymentID, orgUUID)
}

// GetWebBrokerAPIDeploymentsByHandle retrieves deployments for a WebBroker API using handle
func (s *WebBrokerAPIDeploymentService) GetWebBrokerAPIDeploymentsByHandle(apiHandle, gatewayID, status, orgUUID string) (*api.DeploymentListResponse, error) {
	apiUUID, err := s.getWebBrokerAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	var gatewayIdPtr *string
	var statusPtr *string
	if handle := strings.TrimSpace(gatewayID); handle != "" {
		// The gatewayId filter is a gateway handle (matching deploy/undeploy); resolve
		// it to the internal gateway UUID stored in deployments before filtering.
		gateway, err := s.gatewayRepo.GetByHandleAndOrgID(handle, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve gateway handle: %w", err)
		}
		if gateway == nil {
			// The filter names a gateway that does not exist in this org: no match.
			return &api.DeploymentListResponse{Count: 0, List: []api.DeploymentResponse{}}, nil
		}
		gatewayIdPtr = &gateway.ID
	}
	if status != "" {
		statusPtr = &status
	}

	return s.getWebBrokerAPIDeployments(apiUUID, orgUUID, gatewayIdPtr, statusPtr)
}

// deployWebBrokerAPI deploys a WebBroker API to a gateway
func (s *WebBrokerAPIDeploymentService) deployWebBrokerAPI(apiUUID string, req *api.DeployRequest, orgID, createdBy string) (*api.DeploymentResponse, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	// DP-originated artifacts are read-only in the control plane; deployment cannot be CP-initiated.
	if err := ensureArtifactMutableByUUID(s.artifactRepo, apiUUID, orgID); err != nil {
		return nil, err
	}
	if req.Base == "" {
		return nil, constants.ErrDeploymentBaseRequired
	}
	gatewayHandle := strings.TrimSpace(req.GatewayId)
	if gatewayHandle == "" {
		return nil, constants.ErrDeploymentGatewayIDRequired
	}
	metadata := utils.MapValueOrEmpty(req.Metadata)

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayHandle, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, constants.ErrGatewayNotFound
	}
	gatewayID := gateway.ID

	webbrokerAPI, err := s.webbrokerAPIRepo.GetByUUID(apiUUID, orgID)
	if err != nil {
		return nil, err
	}
	if webbrokerAPI == nil {
		return nil, constants.ErrWebBrokerAPINotFound
	}

	// Generate deployment ID
	deploymentID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployment ID: %w", err)
	}

	var baseDeploymentID *string
	var contentBytes []byte

	if req.Base == "current" {
		d := buildWebBrokerAPIDeploymentYAML(webbrokerAPI)
		contentBytes, err = yaml.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal WebBroker API deployment YAML: %w", err)
		}
	} else {
		baseDeployment, err := s.deploymentRepo.GetWithContent(req.Base, apiUUID, orgID)
		if err != nil {
			if errors.Is(err, constants.ErrDeploymentNotFound) {
				return nil, constants.ErrBaseDeploymentNotFound
			}
			return nil, fmt.Errorf("failed to get base deployment: %w", err)
		}
		contentBytes = baseDeployment.Content
		baseDeploymentID = &req.Base
	}

	deployment := &model.Deployment{
		DeploymentID:     deploymentID,
		Name:             req.Name,
		ArtifactID:       apiUUID,
		OrganizationID:   orgID,
		GatewayID:        gatewayID,
		BaseDeploymentID: baseDeploymentID,
		Content:          contentBytes,
		Metadata:         metadata,
		CreatedBy:        createdBy,
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway limit config must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	if err := s.deploymentRepo.CreateWithLimitEnforcement(deployment, hardLimit); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	// Ensure API-Gateway association exists
	if err := s.ensureAPIGatewayAssociation(apiUUID, gatewayID, orgID); err != nil {
		s.slogger.Warn("Failed to ensure API-gateway association", "error", err)
	}

	initialStatus := model.DeploymentStatusDeployed
	if s.cfg.Deployments.TransitionalStatusEnabled {
		initialStatus = model.DeploymentStatusDeploying
	}
	performedAt := time.Now().Truncate(time.Millisecond)
	if _, err := s.deploymentRepo.SetCurrentWithDetails(
		apiUUID, orgID, gatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	); err != nil {
		return nil, fmt.Errorf("failed to set deployment status for WebBroker API: %w", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.WebBrokerAPIDeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}
		if err := s.gatewayEventsService.BroadcastWebBrokerAPIDeploymentEvent(gatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast WebBroker API deployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
		s.gatewayRepo,
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		initialStatus,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		deployment.UpdatedAt,
		nil,
	)
}

// undeployWebBrokerAPIDeployment undeploys a WebBroker API from a gateway
func (s *WebBrokerAPIDeploymentService) undeployWebBrokerAPIDeployment(apiUUID string, deploymentId *string, gatewayId *string, orgID string) (*api.DeploymentResponse, error) {
	// DP-originated artifacts are read-only in the control plane: their deploy/undeploy
	// lifecycle is owned by the data-plane gateway, so the control plane must not
	// initiate an undeployment for them.
	if s.artifactRepo != nil {
		artifact, err := s.artifactRepo.GetByUUID(apiUUID, orgID)
		if err != nil {
			return nil, fmt.Errorf("failed to look up artifact origin: %w", err)
		}
		if artifact != nil {
			if err := ensureOriginMutable(artifact.Origin); err != nil {
				return nil, err
			}
		}
	}

	webbrokerAPI, err := s.webbrokerAPIRepo.GetByUUID(apiUUID, orgID)
	if err != nil {
		return nil, err
	}
	if webbrokerAPI == nil {
		return nil, constants.ErrWebBrokerAPINotFound
	}

	var deployment *model.Deployment
	if deploymentId != nil {
		deployment, err = s.deploymentRepo.GetWithState(*deploymentId, apiUUID, orgID)
		if err != nil {
			return nil, err
		}
		if deployment == nil {
			return nil, constants.ErrDeploymentNotFound
		}
	} else if gatewayId != nil {
		deployment, err = s.deploymentRepo.GetCurrentByGateway(apiUUID, *gatewayId, orgID)
		if err != nil {
			return nil, err
		}
		if deployment == nil {
			return nil, constants.ErrDeploymentNotFound
		}
	} else {
		return nil, constants.ErrInvalidInput
	}

	if gatewayId != nil && deployment.GatewayID != *gatewayId {
		return nil, constants.ErrGatewayIDMismatch
	}

	if deployment.Status == nil || *deployment.Status != model.DeploymentStatusDeployed {
		return nil, constants.ErrDeploymentNotActive
	}

	gateway, err := s.gatewayRepo.GetByUUID(deployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, constants.ErrGatewayNotFound
	}

	initialStatus := model.DeploymentStatusUndeployed
	if s.cfg.Deployments.TransitionalStatusEnabled {
		initialStatus = model.DeploymentStatusUndeploying
	}
	performedAt := time.Now().Truncate(time.Millisecond)
	newUpdatedAt, err := s.deploymentRepo.SetCurrentWithDetails(
		apiUUID, orgID, deployment.GatewayID, deployment.DeploymentID,
		initialStatus, string(model.DeploymentStatusUndeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Send undeployment event to gateway
	if s.gatewayEventsService != nil {
		undeploymentEvent := &model.WebBrokerAPIUndeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: deployment.DeploymentID,
			PerformedAt:  performedAt,
		}
		if err := s.gatewayEventsService.BroadcastWebBrokerAPIUndeploymentEvent(deployment.GatewayID, undeploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast WebBroker API undeployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
		s.gatewayRepo,
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		initialStatus,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		&newUpdatedAt,
		nil,
	)
}

// restoreWebBrokerAPIDeployment restores a previously undeployed WebBroker API deployment
func (s *WebBrokerAPIDeploymentService) restoreWebBrokerAPIDeployment(apiUUID string, deploymentId *string, gatewayId *string, orgID string) (*api.DeploymentResponse, error) {
	// DP-originated artifacts are read-only in the control plane; their deployment
	// lifecycle is owned by the data-plane gateway, so restore cannot be CP-initiated.
	if err := ensureArtifactMutableByUUID(s.artifactRepo, apiUUID, orgID); err != nil {
		return nil, err
	}

	targetDeployment, err := s.deploymentRepo.GetWithContent(*deploymentId, apiUUID, orgID)
	if err != nil {
		return nil, err
	}
	if targetDeployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}

	// Only allow restoring ARCHIVED (nil status) or UNDEPLOYED deployments
	if targetDeployment.Status != nil && *targetDeployment.Status != model.DeploymentStatusUndeployed {
		return nil, constants.ErrInvalidDeploymentRestoreState
	}

	if targetDeployment.GatewayID != *gatewayId {
		return nil, constants.ErrGatewayIDMismatch
	}

	currentDeploymentID, status, _, err := s.deploymentRepo.GetStatus(apiUUID, orgID, targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status: %w", err)
	}
	if currentDeploymentID == *deploymentId && status == model.DeploymentStatusDeployed {
		return nil, constants.ErrDeploymentAlreadyDeployed
	}

	gateway, err := s.gatewayRepo.GetByUUID(targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgID {
		return nil, constants.ErrGatewayNotFound
	}

	initialStatus := model.DeploymentStatusDeployed
	if s.cfg.Deployments.TransitionalStatusEnabled {
		initialStatus = model.DeploymentStatusDeploying
	}
	performedAt := time.Now().Truncate(time.Millisecond)
	updatedAt, err := s.deploymentRepo.SetCurrentWithDetails(
		apiUUID, orgID, targetDeployment.GatewayID, *deploymentId,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set current deployment: %w", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.WebBrokerAPIDeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: *deploymentId,
			PerformedAt:  performedAt,
		}
		if err := s.gatewayEventsService.BroadcastWebBrokerAPIDeploymentEvent(targetDeployment.GatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast WebBroker API deployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
		s.gatewayRepo,
		targetDeployment.DeploymentID,
		targetDeployment.Name,
		targetDeployment.GatewayID,
		initialStatus,
		targetDeployment.BaseDeploymentID,
		targetDeployment.Metadata,
		targetDeployment.CreatedAt,
		&updatedAt,
		nil,
	)
}

// getWebBrokerAPIDeployment retrieves a specific WebBroker API deployment
func (s *WebBrokerAPIDeploymentService) getWebBrokerAPIDeployment(apiUUID, deploymentID, orgID string) (*api.DeploymentResponse, error) {
	webbrokerAPI, err := s.webbrokerAPIRepo.GetByUUID(apiUUID, orgID)
	if err != nil {
		return nil, err
	}
	if webbrokerAPI == nil {
		return nil, constants.ErrWebBrokerAPINotFound
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, apiUUID, orgID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}

	return toAPIDeploymentResponse(
		s.gatewayRepo,
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		*deployment.Status,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		deployment.UpdatedAt,
		deployment.StatusReason,
	)
}

// getWebBrokerAPIDeployments retrieves all deployments for a WebBroker API
func (s *WebBrokerAPIDeploymentService) getWebBrokerAPIDeployments(apiUUID, orgID string, gatewayId *string, status *string) (*api.DeploymentListResponse, error) {
	webbrokerAPI, err := s.webbrokerAPIRepo.GetByUUID(apiUUID, orgID)
	if err != nil {
		return nil, err
	}
	if webbrokerAPI == nil {
		return nil, constants.ErrWebBrokerAPINotFound
	}

	if status != nil {
		validStatuses := map[string]bool{
			string(model.DeploymentStatusDeployed):    true,
			string(model.DeploymentStatusUndeployed):  true,
			string(model.DeploymentStatusArchived):    true,
			string(model.DeploymentStatusDeploying):   true,
			string(model.DeploymentStatusUndeploying): true,
			string(model.DeploymentStatusFailed):      true,
		}
		if !validStatuses[*status] {
			return nil, constants.ErrInvalidDeploymentStatus
		}
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway config value must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}

	deployments, err := s.deploymentRepo.GetDeploymentsWithState(apiUUID, orgID, gatewayId, status, s.cfg.Deployments.MaxPerAPIGateway)
	if err != nil {
		return nil, err
	}

	items := make([]api.DeploymentResponse, 0, len(deployments))
	for _, d := range deployments {
		mapped, err := toAPIDeploymentResponse(
			s.gatewayRepo,
			d.DeploymentID,
			d.Name,
			d.GatewayID,
			*d.Status,
			d.BaseDeploymentID,
			d.Metadata,
			d.CreatedAt,
			d.UpdatedAt,
			d.StatusReason,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, *mapped)
	}

	return &api.DeploymentListResponse{
		Count: len(items),
		List:  items,
	}, nil
}

// deleteWebBrokerAPIDeployment deletes a WebBroker API deployment
func (s *WebBrokerAPIDeploymentService) deleteWebBrokerAPIDeployment(apiUUID, deploymentID, orgID string) error {
	webbrokerAPI, err := s.webbrokerAPIRepo.GetByUUID(apiUUID, orgID)
	if err != nil {
		return err
	}
	if webbrokerAPI == nil {
		return constants.ErrWebBrokerAPINotFound
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, apiUUID, orgID)
	if err != nil {
		return err
	}
	if deployment == nil {
		return constants.ErrDeploymentNotFound
	}

	if deployment.Status != nil && *deployment.Status == model.DeploymentStatusDeployed {
		return constants.ErrDeploymentIsDeployed
	}

	if err := s.deploymentRepo.Delete(deploymentID, apiUUID, orgID); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

// ensureAPIGatewayAssociation creates an API-gateway association if one does not already exist.
func (s *WebBrokerAPIDeploymentService) ensureAPIGatewayAssociation(apiUUID, gatewayID, orgUUID string) error {
	associations, err := s.apiRepo.GetAPIAssociations(apiUUID, constants.AssociationTypeGateway, orgUUID)
	if err != nil {
		s.slogger.Error("Failed to get API-gateway associations", "apiUUID", apiUUID, "gatewayID", gatewayID, "error", err)
		return err
	}
	for _, assoc := range associations {
		if assoc.GatewayID == gatewayID {
			s.slogger.Info("API-gateway association already exists, skipping", "apiUUID", apiUUID, "gatewayID", gatewayID)
			return nil
		}
	}
	association := &model.APIAssociation{
		ArtifactID:     apiUUID,
		OrganizationID: orgUUID,
		GatewayID:      gatewayID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.apiRepo.CreateAPIAssociation(association); err != nil {
		s.slogger.Error("Failed to create API-gateway association", "apiUUID", apiUUID, "gatewayID", gatewayID, "orgUUID", orgUUID, "error", err)
		return err
	}
	return nil
}

// getWebBrokerAPIUUIDByHandle retrieves the artifact UUID by its handle from the artifact table
func (s *WebBrokerAPIDeploymentService) getWebBrokerAPIUUIDByHandle(handle, orgUUID string) (string, error) {
	if handle == "" {
		return "", errors.New("artifact handle is required")
	}

	artifact, err := s.artifactRepo.GetByHandle(handle, orgUUID)
	if err != nil {
		return "", err
	}
	if artifact == nil {
		return "", constants.ErrArtifactNotFound
	}

	return artifact.UUID, nil
}

// buildWebBrokerAPIDeploymentYAML builds the WebBroker API deployment YAML struct
func buildWebBrokerAPIDeploymentYAML(webbrokerAPI *model.WebBrokerAPI) *model.WebBrokerAPIDeploymentYAML {
	contextValue := "/"
	if webbrokerAPI.Configuration.Context != nil && *webbrokerAPI.Configuration.Context != "" {
		contextValue = *webbrokerAPI.Configuration.Context
	}

	d := &model.WebBrokerAPIDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.WebBrokerApi,
		Metadata: model.DeploymentMetadata{
			Name: webbrokerAPI.Handle,
		},
		Spec: model.WebBrokerAPIDeploymentSpec{
			DisplayName: webbrokerAPI.Name,
			Version:     webbrokerAPI.Version,
			Context:     contextValue,
			Vhosts: &model.WebBrokerAPIDeploymentVhosts{
				Main: constants.VhostGatewayDefault,
			},
			AllChannels:     buildWebBrokerAllChannelPolicies(webbrokerAPI.Configuration.AllChannels),
			Receiver:        buildWebBrokerDeployReceiver(&webbrokerAPI.Configuration.Receiver, webbrokerAPI.Configuration.AllChannels),
			Broker:          buildWebBrokerDeployBroker(&webbrokerAPI.Configuration.Broker, webbrokerAPI.Configuration.AllChannels),
			Channels:        buildWebBrokerDeployChannels(webbrokerAPI.Configuration.Channels),
			DeploymentState: "deployed",
		},
	}

	if webbrokerAPI.ProjectUUID != "" {
		d.Metadata.Labels = map[string]string{
			"projectId": webbrokerAPI.ProjectUUID,
		}
	}

	return d
}

// buildWebBrokerAllChannelPolicies builds the global all-channel policies for the deployment YAML.
func buildWebBrokerAllChannelPolicies(p *model.WebBrokerAllChannelPolicies) *model.WebBrokerDeployAllChannelPolicies {
	if p == nil {
		return nil
	}
	return &model.WebBrokerDeployAllChannelPolicies{
		OnConnectionInit: generateWebBrokerEventPolicyList(p.OnConnectionInit),
		OnProduce:        generateWebBrokerEventPolicyList(p.OnProduce),
		OnConsume:        generateWebBrokerEventPolicyList(p.OnConsume),
	}
}

// buildWebBrokerDeployChannels builds the per-channel deployment map from the model channels.
func buildWebBrokerDeployChannels(channels map[string]model.WebBrokerChannel) map[string]model.WebBrokerDeployChannel {
	if len(channels) == 0 {
		return nil
	}
	result := make(map[string]model.WebBrokerDeployChannel, len(channels))
	for name, ch := range channels {
		deployChannel := model.WebBrokerDeployChannel{
			OnConnectionInit: generateWebBrokerEventPolicyList(ch.OnConnectionInit),
			OnProduce:        generateWebBrokerEventPolicyList(ch.OnProduce),
			OnConsume:        generateWebBrokerEventPolicyList(ch.OnConsume),
		}
		if ch.ProduceTo != nil {
			deployChannel.ProduceTo = &model.WebBrokerDeployTopic{
				Topic: ch.ProduceTo.Topic,
			}
		}
		if ch.ConsumeFrom != nil {
			deployChannel.ConsumeFrom = &model.WebBrokerDeployTopic{
				Topic: ch.ConsumeFrom.Topic,
			}
		}
		result[name] = deployChannel
	}
	return result
}

func generateWebBrokerEventPolicyList(ep *model.WebBrokerEventPolicies) *model.WebBrokerDeployEventPolicies {
	if ep == nil {
		return nil
	}
	policies := generatePolicyList(ep.Policies)
	if policies == nil {
		return &model.WebBrokerDeployEventPolicies{Policies: &[]model.Policy{}}
	}
	return &model.WebBrokerDeployEventPolicies{Policies: policies}
}

// buildWebBrokerDeployReceiver builds the receiver section for the deployment YAML.
func buildWebBrokerDeployReceiver(receiver *model.WebBrokerReceiver, allChannelPolicies *model.WebBrokerAllChannelPolicies) *model.WebBrokerDeployReceiver {
	if receiver == nil {
		return nil
	}
	policies := []model.Policy{}
	if allChannelPolicies != nil && allChannelPolicies.OnConnectionInit != nil && len(allChannelPolicies.OnConnectionInit.Policies) > 0 {
		policies = append(policies, allChannelPolicies.OnConnectionInit.Policies...)
	}
	return &model.WebBrokerDeployReceiver{
		Name:       receiver.Name,
		Type:       receiver.Type,
		Properties: receiver.Properties,
		Policies:   policies,
	}
}

// buildWebBrokerDeployBroker builds the broker section for the deployment YAML.
func buildWebBrokerDeployBroker(broker *model.WebBrokerBroker, allChannelPolicies *model.WebBrokerAllChannelPolicies) *model.WebBrokerDeployBroker {
	if broker == nil {
		return nil
	}
	policies := []model.Policy{}
	if allChannelPolicies != nil {
		if allChannelPolicies.OnProduce != nil && len(allChannelPolicies.OnProduce.Policies) > 0 {
			policies = append(policies, allChannelPolicies.OnProduce.Policies...)
		}
		if allChannelPolicies.OnConsume != nil && len(allChannelPolicies.OnConsume.Policies) > 0 {
			policies = append(policies, allChannelPolicies.OnConsume.Policies...)
		}
	}
	return &model.WebBrokerDeployBroker{
		Name:       broker.Name,
		Type:       broker.Type,
		Properties: broker.Properties,
		Policies:   policies,
	}
}

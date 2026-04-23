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

	"platform-api/src/api"
	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	"gopkg.in/yaml.v3"
)

// WebSubAPIDeploymentService handles deployment operations for WebSub APIs
type WebSubAPIDeploymentService struct {
	artifactRepo         repository.ArtifactRepository
	apiRepo              repository.APIRepository
	websubAPIRepo        repository.WebSubAPIRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	gatewayEventsService *GatewayEventsService
	cfg                  *config.Server
	slogger              *slog.Logger
}

// NewWebSubAPIDeploymentService creates a new WebSubAPIDeploymentService
func NewWebSubAPIDeploymentService(
	websubAPIRepo repository.WebSubAPIRepository,
	deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository,
	artifactRepo repository.ArtifactRepository,
	apiRepo repository.APIRepository,
	gatewayEventsService *GatewayEventsService,
	cfg *config.Server,
	slogger *slog.Logger,
) *WebSubAPIDeploymentService {
	return &WebSubAPIDeploymentService{
		websubAPIRepo:        websubAPIRepo,
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

// DeployWebSubAPIByHandle creates a new immutable deployment using WebSub API handle
func (s *WebSubAPIDeploymentService) DeployWebSubAPIByHandle(apiHandle string, req *api.DeployRequest, orgUUID string) (*api.DeploymentResponse, error) {
	apiUUID, err := s.getWebSubAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.deployWebSubAPI(apiUUID, req, orgUUID)
}

// RestoreWebSubAPIDeploymentByHandle restores a previous deployment using WebSub API handle
func (s *WebSubAPIDeploymentService) RestoreWebSubAPIDeploymentByHandle(apiHandle, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	apiUUID, err := s.getWebSubAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.restoreWebSubAPIDeployment(apiUUID, &deploymentID, &gatewayID, orgUUID)
}

// UndeployWebSubAPIDeploymentByHandle undeploys a deployment using WebSub API handle
func (s *WebSubAPIDeploymentService) UndeployWebSubAPIDeploymentByHandle(apiHandle, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	apiUUID, err := s.getWebSubAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.undeployWebSubAPIDeployment(apiUUID, &deploymentID, &gatewayID, orgUUID)
}

// DeleteWebSubAPIDeploymentByHandle deletes a deployment using WebSub API handle
func (s *WebSubAPIDeploymentService) DeleteWebSubAPIDeploymentByHandle(apiHandle, deploymentID, orgUUID string) error {
	apiUUID, err := s.getWebSubAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return err
	}
	return s.deleteWebSubAPIDeployment(apiUUID, deploymentID, orgUUID)
}

// GetWebSubAPIDeploymentByHandle retrieves a single deployment using WebSub API handle
func (s *WebSubAPIDeploymentService) GetWebSubAPIDeploymentByHandle(apiHandle, deploymentID, orgUUID string) (*api.DeploymentResponse, error) {
	apiUUID, err := s.getWebSubAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}
	return s.getWebSubAPIDeployment(apiUUID, deploymentID, orgUUID)
}

// GetWebSubAPIDeploymentsByHandle retrieves deployments for a WebSub API using handle
func (s *WebSubAPIDeploymentService) GetWebSubAPIDeploymentsByHandle(apiHandle, gatewayID, status, orgUUID string) (*api.DeploymentListResponse, error) {
	apiUUID, err := s.getWebSubAPIUUIDByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	var gatewayIdPtr *string
	var statusPtr *string
	if gatewayID != "" {
		gatewayIdPtr = &gatewayID
	}
	if status != "" {
		statusPtr = &status
	}

	return s.getWebSubAPIDeployments(apiUUID, orgUUID, gatewayIdPtr, statusPtr)
}

// deployWebSubAPI deploys a WebSub API to a gateway
func (s *WebSubAPIDeploymentService) deployWebSubAPI(apiUUID string, req *api.DeployRequest, orgID string) (*api.DeploymentResponse, error) {
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Base == "" {
		return nil, constants.ErrDeploymentBaseRequired
	}
	gatewayID := utils.OpenAPIUUIDToString(req.GatewayId)
	if gatewayID == "" {
		return nil, constants.ErrDeploymentGatewayIDRequired
	}
	metadata := utils.MapValueOrEmpty(req.Metadata)

	// Validate gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByUUID(gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgID {
		return nil, constants.ErrGatewayNotFound
	}

	websubAPI, err := s.websubAPIRepo.GetByUUID(apiUUID, orgID)
	if err != nil {
		return nil, err
	}
	if websubAPI == nil {
		return nil, constants.ErrWebSubAPINotFound
	}

	// Generate deployment ID
	deploymentID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployment ID: %w", err)
	}

	var baseDeploymentID *string
	var contentBytes []byte

	if req.Base == "current" {
		d := buildWebSubAPIDeploymentYAML(websubAPI)
		contentBytes, err = yaml.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal WebSub API deployment YAML: %w", err)
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
		return nil, fmt.Errorf("failed to set deployment status for WebSub API: %w", err)
	}

	// Send deployment event to gateway
	if s.gatewayEventsService != nil {
		deploymentEvent := &model.WebSubAPIDeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: deploymentID,
			PerformedAt:  performedAt,
		}
		if err := s.gatewayEventsService.BroadcastWebSubAPIDeploymentEvent(gatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast WebSub API deployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
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

// undeployWebSubAPIDeployment undeploys a WebSub API from a gateway
func (s *WebSubAPIDeploymentService) undeployWebSubAPIDeployment(apiUUID string, deploymentId *string, gatewayId *string, orgID string) (*api.DeploymentResponse, error) {
	websubAPI, err := s.websubAPIRepo.GetByUUID(apiUUID, orgID)
	if err != nil {
		return nil, err
	}
	if websubAPI == nil {
		return nil, constants.ErrWebSubAPINotFound
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

	if s.gatewayEventsService != nil {
		undeploymentEvent := &model.WebSubAPIUndeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: deployment.DeploymentID,
			PerformedAt:  performedAt,
		}
		if err := s.gatewayEventsService.BroadcastWebSubAPIUndeploymentEvent(deployment.GatewayID, undeploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast WebSub API undeployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
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

// restoreWebSubAPIDeployment restores a previously undeployed WebSub API deployment
func (s *WebSubAPIDeploymentService) restoreWebSubAPIDeployment(apiUUID string, deploymentId *string, gatewayId *string, orgID string) (*api.DeploymentResponse, error) {
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

	if s.gatewayEventsService != nil {
		deploymentEvent := &model.WebSubAPIDeploymentEvent{
			ApiId:        apiUUID,
			DeploymentID: *deploymentId,
			PerformedAt:  performedAt,
		}
		if err := s.gatewayEventsService.BroadcastWebSubAPIDeploymentEvent(targetDeployment.GatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast WebSub API deployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
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

// getWebSubAPIDeployment retrieves a specific WebSub API deployment
func (s *WebSubAPIDeploymentService) getWebSubAPIDeployment(apiUUID, deploymentID, orgID string) (*api.DeploymentResponse, error) {
	websubAPI, err := s.websubAPIRepo.GetByUUID(apiUUID, orgID)
	if err != nil {
		return nil, err
	}
	if websubAPI == nil {
		return nil, constants.ErrWebSubAPINotFound
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, apiUUID, orgID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}

	return toAPIDeploymentResponse(
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

// getWebSubAPIDeployments retrieves all deployments for a WebSub API
func (s *WebSubAPIDeploymentService) getWebSubAPIDeployments(apiUUID, orgID string, gatewayId *string, status *string) (*api.DeploymentListResponse, error) {
	websubAPI, err := s.websubAPIRepo.GetByUUID(apiUUID, orgID)
	if err != nil {
		return nil, err
	}
	if websubAPI == nil {
		return nil, constants.ErrWebSubAPINotFound
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

// deleteWebSubAPIDeployment deletes a WebSub API deployment
func (s *WebSubAPIDeploymentService) deleteWebSubAPIDeployment(apiUUID, deploymentID, orgID string) error {
	websubAPI, err := s.websubAPIRepo.GetByUUID(apiUUID, orgID)
	if err != nil {
		return err
	}
	if websubAPI == nil {
		return constants.ErrWebSubAPINotFound
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
func (s *WebSubAPIDeploymentService) ensureAPIGatewayAssociation(apiUUID, gatewayID, orgUUID string) error {
	associations, err := s.apiRepo.GetAPIAssociations(apiUUID, constants.AssociationTypeGateway, orgUUID)
	if err != nil {
		s.slogger.Error("Failed to get API-gateway associations", "apiUUID", apiUUID, "gatewayID", gatewayID, "error", err)
		return err
	}
	for _, assoc := range associations {
		if assoc.ResourceID == gatewayID {
			s.slogger.Info("API-gateway association already exists, skipping", "apiUUID", apiUUID, "gatewayID", gatewayID)
			return nil
		}
	}
	association := &model.APIAssociation{
		ArtifactID:      apiUUID,
		OrganizationID:  orgUUID,
		ResourceID:      gatewayID,
		AssociationType: constants.AssociationTypeGateway,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := s.apiRepo.CreateAPIAssociation(association); err != nil {
		s.slogger.Error("Failed to create API-gateway association", "apiUUID", apiUUID, "gatewayID", gatewayID, "orgUUID", orgUUID, "error", err)
		return err
	}
	return nil
}

// getWebSubAPIUUIDByHandle retrieves the artifact UUID by its handle from the artifact table
func (s *WebSubAPIDeploymentService) getWebSubAPIUUIDByHandle(handle, orgUUID string) (string, error) {
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

// buildWebSubAPIDeploymentYAML builds the WebSub API deployment YAML struct
func buildWebSubAPIDeploymentYAML(websubAPI *model.WebSubAPI) *model.WebSubAPIDeploymentYAML {
	contextValue := "/"
	if websubAPI.Configuration.Context != nil && *websubAPI.Configuration.Context != "" {
		contextValue = *websubAPI.Configuration.Context
	}

	channels := make([]model.WebSubAPIDeploymentChannel, 0, len(websubAPI.Configuration.Channels))
	for _, ch := range websubAPI.Configuration.Channels {
		method := "SUB"
		if ch.Request != nil && ch.Request.Method != "" {
			method = ch.Request.Method
		}
		channelName := ch.Name
		if ch.Request != nil && ch.Request.Name != "" {
			channelName = ch.Request.Name
		}
		channelName = strings.TrimPrefix(channelName, "/")
		channels = append(channels, model.WebSubAPIDeploymentChannel{
			Name:   channelName,
			Method: method,
		})
	}

	d := &model.WebSubAPIDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.WebSubApi,
		Metadata: model.DeploymentMetadata{
			Name: websubAPI.Handle,
		},
		Spec: model.WebSubAPIDeploymentSpec{
			DisplayName: websubAPI.Name,
			Version:     websubAPI.Version,
			Context:     contextValue,
			Vhosts: &model.WebSubAPIDeploymentVhosts{
				Main: constants.VhostGatewayDefault,
			},
			Channels: channels,
			Policies: websubAPI.Configuration.Policies,
		},
	}

	if websubAPI.ProjectUUID != "" {
		d.Metadata.Labels = map[string]string{
			"projectId": websubAPI.ProjectUUID,
		}
	}

	return d
}

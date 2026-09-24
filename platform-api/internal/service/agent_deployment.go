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
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"gopkg.in/yaml.v3"
)

// AgentDeploymentService serves the deployment lifecycle of Agent proxies.
//
// Public operations address an Agent proxy by its org-scoped handle; everything
// past the boundary (deployments, status, gateway notifications) uses the
// artifact UUID. Deployments are ordinary platform deployment records, so the
// storage, retention and status machinery is the shared one.
type AgentDeploymentService struct {
	agentRepo            repository.AgentProxyRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	apiKeyRepo           repository.APIKeyRepository
	gatewayEventsService *GatewayEventsService
	cfg                  *config.Server
	// builds is the shared build store every artifact kind uses; a deploy from
	// `current` renders through it so the snapshot commits with the deployment.
	builds  *BuildService
	slogger *slog.Logger
}

// NewAgentDeploymentService creates a new AgentDeploymentService.
func NewAgentDeploymentService(agentRepo repository.AgentProxyRepository, deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository, artifactRepo repository.ArtifactRepository,
	apiKeyRepo repository.APIKeyRepository, gatewayEventsService *GatewayEventsService,
	definitions ArtifactDefinitions, cfg *config.Server, slogger *slog.Logger) *AgentDeploymentService {
	return &AgentDeploymentService{
		agentRepo:            agentRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		apiKeyRepo:           apiKeyRepo,
		gatewayEventsService: gatewayEventsService,
		cfg:                  cfg,
		builds:               NewBuildService(artifactRepo, deploymentRepo, definitions, cfg, slogger),
		slogger:              slogger,
	}
}

// DeployByHandle creates an immutable deployment of an Agent proxy on one
// gateway. The returned deployment is DEPLOYING: the record exists, the gateway
// has not yet acknowledged it.
func (s *AgentDeploymentService) DeployByHandle(handle string, req *api.DeployRequest, orgUUID, createdBy string) (*api.DeploymentResponse, error) {
	if req == nil {
		return nil, apperror.AgentProxyDeploymentValidationFailed.New("A request body is required.")
	}
	// The request is validated in full before anything is resolved or stored.
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperror.AgentProxyDeploymentValidationFailed.New("A deployment name is required.")
	}
	base, requestedBuild, err := ValidateDeployBase(req.Base, req.BuildId,
		apperror.AgentProxyDeploymentValidationFailed)
	if err != nil {
		return nil, err
	}
	gatewayHandle := strings.TrimSpace(req.GatewayId)
	if gatewayHandle == "" {
		return nil, apperror.AgentProxyDeploymentValidationFailed.New("A gatewayId is required.")
	}

	proxy, err := s.resolveAgentProxy(handle, orgUUID)
	if err != nil {
		return nil, err
	}
	proxyUUID := proxy.UUID

	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayHandle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, apperror.GatewayNotFound.New()
	}

	// DP-originated artifacts are read-only in the control plane and cannot be
	// (re)deployed from it.
	if err := ensureOriginMutable(proxy.Origin); err != nil {
		return nil, err
	}

	// What this deploy ships: a build prepared earlier, or a snapshot of the
	// Agent proxy as it stands now. The snapshot comes back unstored so it
	// commits with the deployment below.
	source, err := s.builds.SourceForDeploy(proxyUUID, orgUUID, constants.AgentProxy, createdBy, base, requestedBuild)
	if err != nil {
		return nil, err
	}
	definition, ok := source.Definition.(*model.AgentProxyDeploymentYAML)
	if !ok {
		// Only reachable if the artifact row claims another kind, which would be
		// a corrupted row rather than anything a caller did.
		return nil, fmt.Errorf("artifact %s did not render as an Agent proxy definition", proxyUUID)
	}

	contentBytes, targetDataVersion, err := s.translateForGateway(definition, source.DataVersion, gateway)
	if err != nil {
		return nil, err
	}

	deploymentID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployment ID: %w", err)
	}

	// Ensure the gateway association exists before the deployment is recorded.
	// The first deployment to a gateway creates it and seeds its metadata from
	// this request; an existing association's metadata is never modified here —
	// an omitted metadata field falls back to what the association stores.
	metadata := utils.MapValueOrEmpty(req.Metadata)
	deployMetaJSON, err := marshalDeploymentMetadata(metadata)
	if err != nil {
		return nil, err
	}
	effectiveMetaJSON, err := s.agentRepo.EnsureGatewayAssociation(proxyUUID, gateway.ID, orgUUID, createdBy,
		deployMetaJSON, req.Metadata != nil)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure gateway association: %w", err)
	}
	if metadata, err = unmarshalDeploymentMetadata(effectiveMetaJSON); err != nil {
		return nil, err
	}

	deployment := &model.Deployment{
		DeploymentID:   deploymentID,
		Name:           req.Name,
		ArtifactID:     proxyUUID,
		OrganizationID: orgUUID,
		GatewayID:      gateway.ID,
		BuildUUID:      source.BuildUUID,
		BuildID:        source.BuildID,
		Content:        contentBytes,
		Metadata:       metadata,
		CreatedBy:      createdBy,
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway limit config must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	// Hard limit = configured soft limit + a buffer for concurrent deployments.
	// Retention pruning happens in the same transaction as the insert.
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	// A build rendered for this deploy is stored with the deployment, in one
	// transaction: a recorded deployment always has the build it runs, and a
	// deploy that fails leaves no build behind.
	if source.NewBuild != nil {
		err = s.deploymentRepo.CreateWithBuild(deployment, source.NewBuild,
			s.cfg.Deployments.MaxBuildsPerAPI, hardLimit)
	} else {
		err = s.deploymentRepo.CreateWithLimitEnforcement(deployment, hardLimit)
	}
	if err != nil {
		if limitErr := s.builds.LimitError(err); limitErr != err {
			return nil, limitErr
		}
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	// Transitional until the gateway acknowledges the artifact. performedAt is
	// the concurrency token the acknowledgement is matched against.
	initialStatus := model.DeploymentStatusDeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := s.deploymentRepo.SetCurrentWithDetails(
		proxyUUID, orgUUID, gateway.ID, deploymentID,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	); err != nil {
		return nil, fmt.Errorf("failed to set deployment status for Agent proxy: %w", err)
	}

	s.slogger.Info("Agent proxy deployment created",
		"deploymentID", deploymentID, "artifactUUID", proxyUUID,
		"gatewayID", gateway.ID, "gatewayVersion", gateway.Version,
		"sourceDataVersion", source.DataVersion, "targetDataVersion", targetDataVersion)

	// Push the Agent proxy's existing active API keys to the gateway.
	BackfillAPIKeysToGateway(s.apiKeyRepo, s.gatewayRepo, s.gatewayEventsService, s.slogger, proxyUUID, gateway.ID, createdBy)

	resp, err := toAPIDeploymentResponse(
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
	return namingBuild(resp, err, deployment.BuildID)
}

// translateForGateway produces the bytes a deployment stores for one gateway.
//
// The translator consumes deployment YAML, which is in the gateway's vocabulary,
// so it is keyed by the gateway kind (Agent), never the control-plane kind.
//
// Existing kinds down-convert silently. Agent does not get to: it has no older
// shape to fall back to, so the target data version is returned for the caller
// to log against the deployment. The stored content's apiVersion is the durable
// record of the same fact.
func (s *AgentDeploymentService) translateForGateway(definition *model.AgentProxyDeploymentYAML,
	sourceDataVersion string, gateway *model.Gateway) ([]byte, gatewaytranslator.GatewayDataVersion, error) {

	gatewayKind, ok := gatewaytranslator.GatewayKindForPlatformKind(constants.AgentProxy)
	if !ok {
		return nil, "", fmt.Errorf("no gateway kind registered for %s", constants.AgentProxy)
	}
	source := gatewaytranslator.PlatformDataVersion(sourceDataVersion)
	target := gatewaytranslator.GatewayDataVersionForGateway(gateway.Version)
	if err := gatewaytranslator.Translate(gatewayKind, source, target, definition); err != nil {
		return nil, "", fmt.Errorf("failed to transform Agent proxy deployment for gateway %s: %w", gateway.Version, err)
	}
	contentBytes, err := yaml.Marshal(definition)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal Agent proxy deployment YAML: %w", err)
	}
	return contentBytes, target, nil
}

// UndeployByHandle begins undeploying a deployment from its bound gateway. The
// gatewayHandle must name that gateway; the returned deployment is UNDEPLOYING.
func (s *AgentDeploymentService) UndeployByHandle(handle, deploymentID, gatewayHandle, orgUUID string) (*api.DeploymentResponse, error) {
	proxy, err := s.resolveAgentProxy(handle, orgUUID)
	if err != nil {
		return nil, err
	}
	proxyUUID := proxy.UUID
	// DP-originated artifacts are read-only in the control plane; their
	// deployment lifecycle is owned by the data-plane gateway.
	if err := ensureOriginMutable(proxy.Origin); err != nil {
		return nil, err
	}
	gatewayUUID, err := s.resolveGatewayUUID(gatewayHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, proxyUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}
	if deployment.GatewayID != gatewayUUID {
		return nil, apperror.DeploymentGatewayMismatch.New()
	}
	if deployment.Status == nil || !deployment.Status.IsDeployedOrDeploying() {
		return nil, apperror.DeploymentNotActive.New("Agent proxy")
	}

	initialStatus := model.DeploymentStatusUndeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	updatedAt, err := s.deploymentRepo.SetCurrentWithDetails(
		proxyUUID, orgUUID, deployment.GatewayID, deployment.DeploymentID,
		initialStatus, string(model.DeploymentStatusUndeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	resp, err := toAPIDeploymentResponse(
		s.gatewayRepo,
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		initialStatus,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		&updatedAt,
		nil,
	)
	return namingBuild(resp, err, deployment.BuildID)
}

// RestoreByHandle begins restoring a previous deployment on its bound gateway.
// The stored snapshot is shipped as it is — never re-rendered or re-translated —
// so a restore puts back exactly what was deployed. The returned deployment is
// DEPLOYING.
func (s *AgentDeploymentService) RestoreByHandle(handle, deploymentID, gatewayHandle, orgUUID string) (*api.DeploymentResponse, error) {
	proxy, err := s.resolveAgentProxy(handle, orgUUID)
	if err != nil {
		return nil, err
	}
	proxyUUID := proxy.UUID
	// DP-originated artifacts are read-only in the control plane; their
	// deployment lifecycle is owned by the data-plane gateway.
	if err := ensureOriginMutable(proxy.Origin); err != nil {
		return nil, err
	}
	gatewayUUID, err := s.resolveGatewayUUID(gatewayHandle, orgUUID)
	if err != nil {
		return nil, err
	}

	target, err := s.deploymentRepo.GetWithContent(deploymentID, proxyUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, apperror.DeploymentNotFound.New()
	}
	if target.GatewayID != gatewayUUID {
		return nil, apperror.DeploymentGatewayMismatch.New()
	}

	currentDeploymentID, status, _, err := s.deploymentRepo.GetStatus(proxyUUID, orgUUID, target.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status: %w", err)
	}
	if currentDeploymentID == deploymentID && status.IsDeployedOrDeploying() {
		return nil, apperror.DeploymentRestoreConflict.New()
	}

	initialStatus := model.DeploymentStatusDeploying
	performedAt := time.Now().UTC().Truncate(time.Millisecond)
	updatedAt, err := s.deploymentRepo.SetCurrentWithDetails(
		proxyUUID, orgUUID, target.GatewayID, deploymentID,
		initialStatus, string(model.DeploymentStatusDeployed),
		&performedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set current deployment: %w", err)
	}

	BackfillAPIKeysToGateway(s.apiKeyRepo, s.gatewayRepo, s.gatewayEventsService, s.slogger, proxyUUID, target.GatewayID, "")

	resp, err := toAPIDeploymentResponse(
		s.gatewayRepo,
		target.DeploymentID,
		target.Name,
		target.GatewayID,
		initialStatus,
		target.BaseDeploymentID,
		target.Metadata,
		target.CreatedAt,
		&updatedAt,
		nil,
	)
	return namingBuild(resp, err, target.BuildID)
}

// DeleteByHandle deletes one deployment record. Only an UNDEPLOYED deployment is
// deletable. This removes a record, not the Agent proxy, so no gateway is
// notified.
func (s *AgentDeploymentService) DeleteByHandle(handle, deploymentID, orgUUID string) error {
	proxy, err := s.resolveAgentProxy(handle, orgUUID)
	if err != nil {
		return err
	}
	proxyUUID := proxy.UUID
	// A DP-originated Agent proxy's deployment history is the data plane's
	// record, and is read-only here like the rest of it.
	if err := ensureOriginMutable(proxy.Origin); err != nil {
		return err
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, proxyUUID, orgUUID)
	if err != nil {
		return err
	}
	if deployment == nil {
		return apperror.DeploymentNotFound.New()
	}
	if err := ensureAgentDeploymentDeletable(deployment.Status); err != nil {
		return err
	}

	if err := s.deploymentRepo.Delete(deploymentID, proxyUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}
	return nil
}

// ensureAgentDeploymentDeletable admits UNDEPLOYED only. A nil status is an
// ARCHIVED (superseded) record, which is not deletable either.
func ensureAgentDeploymentDeletable(status *model.DeploymentStatus) error {
	if status != nil && *status == model.DeploymentStatusUndeployed {
		return nil
	}
	if status != nil && status.IsDeployedOrDeploying() {
		return apperror.DeploymentActive.New()
	}
	return apperror.AgentProxyDeploymentNotUndeployed.New()
}

// GetByHandle returns one deployment with its current state and, on failure,
// its statusReason.
func (s *AgentDeploymentService) GetByHandle(handle, deploymentID, orgUUID string) (*api.DeploymentResponse, error) {
	proxy, err := s.resolveAgentProxy(handle, orgUUID)
	if err != nil {
		return nil, err
	}
	proxyUUID := proxy.UUID

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, proxyUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, apperror.DeploymentNotFound.New()
	}

	resp, err := toAPIDeploymentResponse(
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
	return namingBuild(resp, err, deployment.BuildID)
}

// ListByHandle returns an Agent proxy's deployments, optionally filtered by
// gateway handle and status. Pagination is applied by the caller.
func (s *AgentDeploymentService) ListByHandle(handle, gatewayHandle, status, orgUUID string) (*api.DeploymentListResponse, error) {
	proxy, err := s.resolveAgentProxy(handle, orgUUID)
	if err != nil {
		return nil, err
	}
	proxyUUID := proxy.UUID

	var statusPtr *string
	if status != "" {
		if !isValidDeploymentStatusFilter(status) {
			return nil, apperror.DeploymentInvalidStatus.New()
		}
		statusPtr = &status
	}

	var gatewayHandlePtr *string
	if gatewayHandle != "" {
		gatewayHandlePtr = &gatewayHandle
	}
	// The gatewayId filter is a gateway handle (matching deploy/undeploy);
	// resolve it to the gateway UUID stored on deployments.
	gatewayUUID, found, err := resolveGatewayFilter(s.gatewayRepo, gatewayHandlePtr, orgUUID)
	if err != nil {
		return nil, err
	}
	if !found {
		// The filter names a gateway that does not exist in this org.
		return &api.DeploymentListResponse{Count: 0, List: []api.DeploymentResponse{}}, nil
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway config value must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	deployments, err := s.deploymentRepo.GetDeploymentsWithState(proxyUUID, orgUUID, gatewayUUID, statusPtr, s.cfg.Deployments.MaxPerAPIGateway)
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
		mapped.BuildId = d.BuildID
		items = append(items, *mapped)
	}

	return &api.DeploymentListResponse{Count: len(items), List: items}, nil
}

// isValidDeploymentStatusFilter accepts the full six-state enum, ARCHIVED
// included: an Agent proxy deployment is archived like any other.
func isValidDeploymentStatusFilter(status string) bool {
	switch model.DeploymentStatus(status) {
	case model.DeploymentStatusDeployed, model.DeploymentStatusUndeployed, model.DeploymentStatusArchived,
		model.DeploymentStatusDeploying, model.DeploymentStatusUndeploying, model.DeploymentStatusFailed:
		return true
	}
	return false
}

// resolveAgentProxy turns a public handle into the Agent proxy it names.
//
// The lookup is made against agent_proxies alone. Handles are unique only
// WITHIN a kind, so a cross-kind artifact lookup can return another kind's row
// for the same handle and hide an Agent proxy that exists.
func (s *AgentDeploymentService) resolveAgentProxy(handle, orgUUID string) (*model.AgentProxy, error) {
	if strings.TrimSpace(handle) == "" {
		return nil, apperror.AgentProxyNotFound.New()
	}
	proxy, err := s.agentRepo.GetByHandle(handle, orgUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, apperror.AgentProxyNotFound.New()
	}
	return proxy, nil
}

// resolveGatewayUUID turns a gateway handle into the gateway UUID deployments
// store, within the caller's organization.
func (s *AgentDeploymentService) resolveGatewayUUID(gatewayHandle, orgUUID string) (string, error) {
	gatewayHandle = strings.TrimSpace(gatewayHandle)
	if gatewayHandle == "" {
		return "", apperror.AgentProxyDeploymentValidationFailed.New("A gatewayId is required.")
	}
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayHandle, orgUUID)
	if err != nil {
		return "", fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return "", apperror.GatewayNotFound.New()
	}
	return gateway.ID, nil
}

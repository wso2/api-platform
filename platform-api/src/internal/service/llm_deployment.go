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
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"platform-api/src/api"
	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"gopkg.in/yaml.v3"
)

// TODO: Temporary
const (
	tokenBasedRateLimitPolicyName = "token-based-ratelimit"
	advancedRateLimitPolicyName   = "advanced-ratelimit"
	apiKeyAuthPolicyName          = "api-key-auth"
	rateLimitPolicyVersion        = "v0"
	apiKeyAuthPolicyVersion       = "v0"
)

// LLMProviderDeploymentService handles business logic for LLM provider deployment operations
// using the shared deployments table and status model.
type LLMProviderDeploymentService struct {
	providerRepo         repository.LLMProviderRepository
	templateRepo         repository.LLMProviderTemplateRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	gatewayEventsService *GatewayEventsService
	cfg                  *config.Server
	slogger              *slog.Logger
}

// LLMProxyDeploymentService handles business logic for LLM proxy deployment operations
// using the shared deployments table and status model.
type LLMProxyDeploymentService struct {
	proxyRepo            repository.LLMProxyRepository
	deploymentRepo       repository.DeploymentRepository
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	gatewayEventsService *GatewayEventsService
	cfg                  *config.Server
	slogger              *slog.Logger
}

// NewLLMProviderDeploymentService creates a new LLM provider deployment service
func NewLLMProviderDeploymentService(
	providerRepo repository.LLMProviderRepository,
	templateRepo repository.LLMProviderTemplateRepository,
	deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository,
	gatewayEventsService *GatewayEventsService,
	cfg *config.Server,
	slogger *slog.Logger,
) *LLMProviderDeploymentService {
	return &LLMProviderDeploymentService{
		providerRepo:         providerRepo,
		templateRepo:         templateRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		orgRepo:              orgRepo,
		gatewayEventsService: gatewayEventsService,
		cfg:                  cfg,
		slogger:              slogger,
	}
}

// NewLLMProxyDeploymentService creates a new LLM proxy deployment service
func NewLLMProxyDeploymentService(
	proxyRepo repository.LLMProxyRepository,
	deploymentRepo repository.DeploymentRepository,
	gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository,
	gatewayEventsService *GatewayEventsService,
	cfg *config.Server,
	slogger *slog.Logger,
) *LLMProxyDeploymentService {
	return &LLMProxyDeploymentService{
		proxyRepo:            proxyRepo,
		deploymentRepo:       deploymentRepo,
		gatewayRepo:          gatewayRepo,
		orgRepo:              orgRepo,
		gatewayEventsService: gatewayEventsService,
		cfg:                  cfg,
		slogger:              slogger,
	}
}

// DeployLLMProvider creates a new immutable deployment artifact and deploys it to a gateway
func (s *LLMProviderDeploymentService) DeployLLMProvider(providerID string, req *api.DeployRequest, orgUUID string) (*api.DeploymentResponse, error) {
	// Validate request
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Base == "" {
		return nil, constants.ErrDeploymentBaseRequired
	}
	if req.GatewayId == (openapi_types.UUID{}) {
		return nil, constants.ErrDeploymentGatewayIDRequired
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
	if gateway == nil || gateway.OrganizationID != orgUUID {
		return nil, constants.ErrGatewayNotFound
	}

	// Get LLM provider
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, constants.ErrLLMProviderNotFound
	}

	// Validate deployment name is provided
	if req.Name == "" {
		return nil, constants.ErrDeploymentNameRequired
	}

	var baseDeploymentID *string
	var contentBytes []byte

	// Determine the source: "current" or existing deployment
	if req.Base == "current" {
		tplHandle, err := s.getTemplateHandle(provider.TemplateUUID, orgUUID)
		if err != nil {
			return nil, err
		}
		providerYaml, err := generateLLMProviderDeploymentYAML(provider, tplHandle)
		if err != nil {
			return nil, fmt.Errorf("failed to generate LLM provider deployment YAML: %w", err)
		}
		contentBytes = []byte(providerYaml)
	} else {
		// Use existing deployment as base
		baseDeployment, err := s.deploymentRepo.GetWithContent(req.Base, provider.UUID, orgUUID)
		if err != nil {
			if errors.Is(err, constants.ErrDeploymentNotFound) {
				return nil, constants.ErrBaseDeploymentNotFound
			}
			return nil, fmt.Errorf("failed to get base deployment: %w", err)
		}
		contentBytes = baseDeployment.Content
		baseDeploymentID = &req.Base
	}

	// Generate deployment ID
	deploymentID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployment ID: %w", err)
	}
	deployed := model.DeploymentStatusDeployed

	deployment := &model.Deployment{
		DeploymentID:     deploymentID,
		Name:             req.Name,
		ArtifactID:       provider.UUID,
		OrganizationID:   orgUUID,
		GatewayID:        gatewayID,
		BaseDeploymentID: baseDeploymentID,
		Content:          contentBytes,
		Metadata:         metadata,
		Status:           &deployed,
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway limit config must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	if err := s.deploymentRepo.CreateWithLimitEnforcement(deployment, hardLimit); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	// Broadcast LLM provider deployment event to gateway
	if s.gatewayEventsService != nil {
		vhost := ""
		if provider.Configuration.VHost != nil {
			vhost = *provider.Configuration.VHost
		}
		deploymentEvent := &model.LLMProviderDeploymentEvent{
			ProviderId:   provider.ID,
			DeploymentID: deploymentID,
			Vhost:        vhost,
			Environment:  "production",
		}

		if err := s.gatewayEventsService.BroadcastLLMProviderDeploymentEvent(gatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast LLM provider deployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		model.DeploymentStatusDeployed,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		deployment.UpdatedAt,
	)
}

// RestoreLLMProviderDeployment restores a previous deployment (ARCHIVED or UNDEPLOYED)
func (s *LLMProviderDeploymentService) RestoreLLMProviderDeployment(providerID, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, constants.ErrLLMProviderNotFound
	}

	targetDeployment, err := s.deploymentRepo.GetWithContent(deploymentID, provider.UUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if targetDeployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}
	if targetDeployment.GatewayID != gatewayID {
		return nil, constants.ErrGatewayIDMismatch
	}

	currentDeploymentID, status, _, err := s.deploymentRepo.GetStatus(provider.UUID, orgUUID, targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status: %w", err)
	}
	if currentDeploymentID == deploymentID && status == model.DeploymentStatusDeployed {
		return nil, constants.ErrDeploymentAlreadyDeployed
	}

	gateway, err := s.gatewayRepo.GetByUUID(targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgUUID {
		return nil, constants.ErrGatewayNotFound
	}

	updatedAt, err := s.deploymentRepo.SetCurrent(provider.UUID, orgUUID, targetDeployment.GatewayID, deploymentID, model.DeploymentStatusDeployed)
	if err != nil {
		return nil, fmt.Errorf("failed to set current deployment: %w", err)
	}

	// Broadcast LLM provider deployment event to gateway
	if s.gatewayEventsService != nil {
		vhost := ""
		if provider.Configuration.VHost != nil {
			vhost = *provider.Configuration.VHost
		}
		deploymentEvent := &model.LLMProviderDeploymentEvent{
			ProviderId:   provider.ID,
			DeploymentID: deploymentID,
			Vhost:        vhost,
			Environment:  "production",
		}

		if err := s.gatewayEventsService.BroadcastLLMProviderDeploymentEvent(targetDeployment.GatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast LLM provider deployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
		targetDeployment.DeploymentID,
		targetDeployment.Name,
		targetDeployment.GatewayID,
		model.DeploymentStatusDeployed,
		targetDeployment.BaseDeploymentID,
		targetDeployment.Metadata,
		targetDeployment.CreatedAt,
		&updatedAt,
	)
}

// UndeployLLMProviderDeployment undeploys an active deployment
func (s *LLMProviderDeploymentService) UndeployLLMProviderDeployment(providerID, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, constants.ErrLLMProviderNotFound
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, provider.UUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}
	if deployment.GatewayID != gatewayID {
		return nil, constants.ErrGatewayIDMismatch
	}
	if deployment.Status == nil || *deployment.Status != model.DeploymentStatusDeployed {
		return nil, constants.ErrDeploymentNotActive
	}

	gateway, err := s.gatewayRepo.GetByUUID(deployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgUUID {
		return nil, constants.ErrGatewayNotFound
	}

	newUpdatedAt, err := s.deploymentRepo.SetCurrent(provider.UUID, orgUUID, deployment.GatewayID, deploymentID, model.DeploymentStatusUndeployed)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Broadcast LLM provider undeployment event to gateway
	if s.gatewayEventsService != nil {
		vhost := ""
		if provider.Configuration.VHost != nil {
			vhost = *provider.Configuration.VHost
		}
		undeploymentEvent := &model.LLMProviderUndeploymentEvent{
			ProviderId:  provider.ID,
			Vhost:       vhost,
			Environment: "production",
		}

		if err := s.gatewayEventsService.BroadcastLLMProviderUndeploymentEvent(deployment.GatewayID, undeploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast LLM provider undeployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		model.DeploymentStatusUndeployed,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		&newUpdatedAt,
	)
}

// DeleteLLMProviderDeployment permanently deletes an undeployed deployment artifact
func (s *LLMProviderDeploymentService) DeleteLLMProviderDeployment(providerID, deploymentID, orgUUID string) error {
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return err
	}
	if provider == nil {
		return constants.ErrLLMProviderNotFound
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, provider.UUID, orgUUID)
	if err != nil {
		return err
	}
	if deployment == nil {
		return constants.ErrDeploymentNotFound
	}
	if deployment.Status != nil && *deployment.Status == model.DeploymentStatusDeployed {
		return constants.ErrDeploymentIsDeployed
	}

	if err := s.deploymentRepo.Delete(deploymentID, provider.UUID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

// GetLLMProviderDeployments retrieves all deployments for a provider with optional filters
func (s *LLMProviderDeploymentService) GetLLMProviderDeployments(providerID, orgUUID string, gatewayID *string, status *string) (*api.DeploymentListResponse, error) {
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, constants.ErrLLMProviderNotFound
	}

	if status != nil {
		validStatuses := map[string]bool{
			string(model.DeploymentStatusDeployed):   true,
			string(model.DeploymentStatusUndeployed): true,
			string(model.DeploymentStatusArchived):   true,
		}
		if !validStatuses[*status] {
			return nil, constants.ErrInvalidDeploymentStatus
		}
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway config value must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	deployments, err := s.deploymentRepo.GetDeploymentsWithState(provider.UUID, orgUUID, gatewayID, status, s.cfg.Deployments.MaxPerAPIGateway)
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

// GetLLMProviderDeployment retrieves a specific deployment by ID
func (s *LLMProviderDeploymentService) GetLLMProviderDeployment(providerID, deploymentID, orgUUID string) (*api.DeploymentResponse, error) {
	provider, err := s.providerRepo.GetByID(providerID, orgUUID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, constants.ErrLLMProviderNotFound
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, provider.UUID, orgUUID)
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
	)
}

func (s *LLMProviderDeploymentService) getTemplateHandle(templateUUID, orgUUID string) (string, error) {
	if templateUUID == "" {
		return "", constants.ErrLLMProviderTemplateNotFound
	}
	tpl, err := s.templateRepo.GetByUUID(templateUUID, orgUUID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve template: %w", err)
	}
	if tpl == nil {
		return "", constants.ErrLLMProviderTemplateNotFound
	}
	return tpl.ID, nil
}

func generateLLMProviderDeploymentYAML(provider *model.LLMProvider, templateHandle string) (string, error) {
	if provider == nil {
		return "", errors.New("provider is required")
	}
	if templateHandle == "" {
		return "", errors.New("template handle is required")
	}
	if provider.Configuration.Upstream == nil || provider.Configuration.Upstream.Main == nil {
		return "", constants.ErrInvalidInput
	}
	main := provider.Configuration.Upstream.Main
	if main.URL == "" && main.Ref == "" {
		return "", constants.ErrInvalidInput
	}

	contextValue := "/"
	if provider.Configuration.Context != nil && *provider.Configuration.Context != "" {
		contextValue = *provider.Configuration.Context
	}
	vhostValue := ""
	if provider.Configuration.VHost != nil {
		vhostValue = *provider.Configuration.VHost
	}

	accessControl := api.LLMAccessControl{Mode: api.DenyAll}
	if provider.Configuration.AccessControl != nil {
		accessControl.Mode = api.LLMAccessControlMode(provider.Configuration.AccessControl.Mode)
		if len(provider.Configuration.AccessControl.Exceptions) > 0 {
			exceptions := make([]api.RouteException, 0, len(provider.Configuration.AccessControl.Exceptions))
			for _, e := range provider.Configuration.AccessControl.Exceptions {
				methods := make([]api.RouteExceptionMethods, 0, len(e.Methods))
				for _, m := range e.Methods {
					methods = append(methods, api.RouteExceptionMethods(m))
				}
				exceptions = append(exceptions, api.RouteException{Path: e.Path, Methods: methods})
			}
			accessControl.Exceptions = &exceptions
		}
	}

	policies := make([]api.LLMPolicy, 0, len(provider.Configuration.Policies))

	// Transform security config
	security := provider.Configuration.Security
	if security != nil && isBoolTrue(security.Enabled) {
		if security.APIKey != nil && isBoolTrue(security.APIKey.Enabled) {
			key := strings.TrimSpace(security.APIKey.Key)
			if key == "" {
				return "", fmt.Errorf("invalid api key security configuration: key is required")
			}

			in := strings.ToLower(strings.TrimSpace(security.APIKey.In))
			if in != "header" && in != "query" {
				return "", fmt.Errorf("invalid api key security configuration: in must be 'header' or 'query', got %q", security.APIKey.In)
			}

			addOrAppendPolicyPath(&policies, apiKeyAuthPolicyName, apiKeyAuthPolicyVersion, api.LLMPolicyPath{
				Path:    "/*",
				Methods: []api.LLMPolicyPathMethods{"*"},
				Params: map[string]interface{}{
					"key": key,
					"in":  in,
				},
			})
		}
	}

	// Transform rate limit config
	// Step 1: Convert rate limit config to policy format
	rateLimit := provider.Configuration.RateLimiting
	if rateLimit != nil {
		// Step 2: Provider level rate limit
		providerLevel := rateLimit.ProviderLevel
		if providerLevel != nil {
			// Priority to global rate limit configuration if both global and resource-wise are present
			if providerLevel.Global != nil {
				// Step 2.1 Handle global rate limiting
				// TODO: Confirm with gateway interpret this as a global policy
				if providerLevel.Global.Token != nil && providerLevel.Global.Token.Enabled {
					tokenLimit := providerLevel.Global.Token
					duration, err := formatRateLimitDuration(tokenLimit.Reset.Duration, tokenLimit.Reset.Unit)
					if err != nil {
						return "", fmt.Errorf("invalid token reset window: %w", err)
					}
					policies = append(policies, api.LLMPolicy{
						// TODO: This should be taken from config
						Name:    tokenBasedRateLimitPolicyName,
						Version: rateLimitPolicyVersion,
						Paths: []api.LLMPolicyPath{
							{
								Path:    "/*",
								Methods: []api.LLMPolicyPathMethods{"*"},
								Params: map[string]interface{}{
									"totalTokenLimits": []map[string]interface{}{
										{
											"count":    tokenLimit.Count,
											"duration": duration,
										},
									},
								},
							},
						},
					})
				}
				if providerLevel.Global.Request != nil && providerLevel.Global.Request.Enabled {
					requestLimit := providerLevel.Global.Request
					duration, err := formatRateLimitDuration(requestLimit.Reset.Duration, requestLimit.Reset.Unit)
					if err != nil {
						return "", fmt.Errorf("invalid request reset window: %w", err)
					}
					policies = append(policies, api.LLMPolicy{
						// TODO: This should be taken from config
						Name:    advancedRateLimitPolicyName,
						Version: rateLimitPolicyVersion,
						Paths: []api.LLMPolicyPath{
							{
								Path:    "/*",
								Methods: []api.LLMPolicyPathMethods{"*"},
								Params: map[string]interface{}{
									// TODO: Is this correct?
									// TODO: Is `algorithm` and `backend` not available?
									"quotas": []map[string]interface{}{
										{
											"name": "request-limit",
											"limits": []map[string]interface{}{
												{
													"limit":    requestLimit.Count,
													"duration": duration,
												},
											},
										},
									},
								},
							},
						},
					})
				}
			} else if providerLevel.ResourceWise != nil {
				// Step 2.2 Handle resource-wise rate limiting
				defaultLimit := &providerLevel.ResourceWise.Default

				// Step 2.2.1 Default resource-wise rate limit
				if defaultLimit.Token != nil && defaultLimit.Token.Enabled {
					tokenLimit := defaultLimit.Token
					duration, err := formatRateLimitDuration(tokenLimit.Reset.Duration, tokenLimit.Reset.Unit)
					if err != nil {
						return "", fmt.Errorf("invalid token reset window: %w", err)
					}
					policies = append(policies, api.LLMPolicy{
						// TODO: This should be taken from config
						Name:    tokenBasedRateLimitPolicyName,
						Version: rateLimitPolicyVersion,
						Paths: []api.LLMPolicyPath{
							{
								Path:    "/*",
								Methods: []api.LLMPolicyPathMethods{"*"},
								Params: map[string]interface{}{
									"totalTokenLimits": []map[string]interface{}{
										{
											"count":    tokenLimit.Count,
											"duration": duration,
										},
									},
								},
							},
						},
					})
				}

				if defaultLimit.Request != nil && defaultLimit.Request.Enabled {
					requestLimit := defaultLimit.Request
					duration, err := formatRateLimitDuration(requestLimit.Reset.Duration, requestLimit.Reset.Unit)
					if err != nil {
						return "", fmt.Errorf("invalid request reset window: %w", err)
					}
					policies = append(policies, api.LLMPolicy{
						// TODO: This should be taken from config
						Name:    advancedRateLimitPolicyName,
						Version: rateLimitPolicyVersion,
						Paths: []api.LLMPolicyPath{
							{
								Path:    "/*",
								Methods: []api.LLMPolicyPathMethods{"*"},
								Params: map[string]interface{}{
									"quotas": []map[string]interface{}{
										{
											"name": "request-limit",
											"limits": []map[string]interface{}{
												{
													"limit":    requestLimit.Count,
													"duration": duration,
												},
											},
										},
									},
								},
							},
						},
					})
				}

				// Step 2.2.2 Resource-wise rate limit
				for _, r := range providerLevel.ResourceWise.Resources {
					if r.Limit.Token != nil && r.Limit.Token.Enabled {
						tokenLimit := r.Limit.Token
						duration, err := formatRateLimitDuration(tokenLimit.Reset.Duration, tokenLimit.Reset.Unit)
						if err != nil {
							return "", fmt.Errorf("invalid token reset window for resource %s: %w", r.Resource, err)
						}
						// TODO: the methods should be coming as input
						addOrAppendPolicyPath(&policies, tokenBasedRateLimitPolicyName, rateLimitPolicyVersion, api.LLMPolicyPath{
							Path:    r.Resource,
							Methods: []api.LLMPolicyPathMethods{"*"},
							Params: map[string]interface{}{
								"totalTokenLimits": []map[string]interface{}{
									{
										"count":    tokenLimit.Count,
										"duration": duration,
									},
								},
							},
						})
					}
					if r.Limit.Request != nil && r.Limit.Request.Enabled {
						requestLimit := r.Limit.Request
						duration, err := formatRateLimitDuration(requestLimit.Reset.Duration, requestLimit.Reset.Unit)
						if err != nil {
							return "", fmt.Errorf("invalid request reset window for resource %s: %w", r.Resource, err)
						}
						// TODO: the methods should be coming as input
						addOrAppendPolicyPath(&policies, advancedRateLimitPolicyName, rateLimitPolicyVersion, api.LLMPolicyPath{
							Path:    r.Resource,
							Methods: []api.LLMPolicyPathMethods{"*"},
							Params: map[string]interface{}{
								"quotas": []map[string]interface{}{
									{
										"name": "request-limit",
										"limits": []map[string]interface{}{
											{
												"limit":    requestLimit.Count,
												"duration": duration,
											},
										},
									},
								},
							},
						})
					}
				}
			}
		}

		// Step 3: Consumer level rate limit
		consumerLevel := rateLimit.ConsumerLevel
		if consumerLevel != nil {
			if consumerLevel.Global != nil {
				// Handle global rate limiting
				// TODO: Convert global rate limit to policy format
			} else if consumerLevel.ResourceWise != nil {
				// Handle resource-wise rate limiting
				// TODO: Convert resource-wise rate limit to policy format
			}
		}
	}

	for _, p := range provider.Configuration.Policies {
		paths := make([]api.LLMPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			methods := make([]api.LLMPolicyPathMethods, 0, len(pp.Methods))
			for _, m := range pp.Methods {
				methods = append(methods, api.LLMPolicyPathMethods(m))
			}
			paths = append(paths, api.LLMPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		policies = append(policies, api.LLMPolicy{Name: p.Name, Version: normalizePolicyVersionToMajor(p.Version), Paths: paths})
	}

	upstream := dto.LLMUpstreamYAML{URL: main.URL, Ref: main.Ref}
	if main.Auth != nil {
		upstream.Auth = mapModelAuthToAPI(main.Auth)
	}

	providerDeployment := dto.LLMProviderDeploymentYAML{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       constants.LLMProvider,
		Metadata: dto.DeploymentMetadata{
			Name: provider.ID,
		},
		Spec: dto.LLMProviderDeploymentSpec{
			DisplayName:   provider.Name,
			Version:       provider.Version,
			Context:       contextValue,
			VHost:         vhostValue,
			Template:      templateHandle,
			Upstream:      upstream,
			AccessControl: accessControl,
			Policies:      policies,
		},
	}

	yamlBytes, err := yaml.Marshal(providerDeployment)
	if err != nil {
		return "", fmt.Errorf("failed to marshal LLM provider to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

func formatRateLimitDuration(duration int, unit string) (string, error) {
	if duration <= 0 {
		return "", fmt.Errorf("duration must be positive, got %d", duration)
	}

	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "minute":
		return fmt.Sprintf("%dm", duration), nil
	case "hour":
		return fmt.Sprintf("%dh", duration), nil
	case "day":
		return fmt.Sprintf("%dh", duration*24), nil
	case "week":
		return fmt.Sprintf("%dh", duration*24*7), nil
	case "month":
		// policy accepts Go duration units; month is represented as 30 days.
		return fmt.Sprintf("%dh", duration*24*30), nil
	default:
		return "", fmt.Errorf("unsupported reset unit: %q", unit)
	}
}

func normalizePolicyVersionToMajor(version string) string {
	trimmedVersion := strings.TrimSpace(version)
	if trimmedVersion == "" {
		return trimmedVersion
	}

	versionWithoutPrefix := trimmedVersion
	if strings.HasPrefix(strings.ToLower(versionWithoutPrefix), "v") {
		versionWithoutPrefix = versionWithoutPrefix[1:]
	}
	if versionWithoutPrefix == "" {
		return trimmedVersion
	}

	majorVersion := versionWithoutPrefix
	if idx := strings.Index(majorVersion, "."); idx >= 0 {
		majorVersion = majorVersion[:idx]
	}
	if idx := strings.Index(majorVersion, "-"); idx >= 0 {
		majorVersion = majorVersion[:idx]
	}
	majorVersion = strings.TrimSpace(majorVersion)
	if majorVersion == "" {
		return trimmedVersion
	}

	if _, err := strconv.Atoi(majorVersion); err != nil {
		return trimmedVersion
	}

	return "v" + majorVersion
}

func addOrAppendPolicyPath(policies *[]api.LLMPolicy, name, version string, path api.LLMPolicyPath) {
	for i := range *policies {
		if (*policies)[i].Name == name && (*policies)[i].Version == version {
			// TODO: Temporary
			for _, existingPath := range (*policies)[i].Paths {
				if existingPath.Path == path.Path {
					// Keep first occurrence and avoid duplicates.
					return
				}
			}
			(*policies)[i].Paths = append((*policies)[i].Paths, path)
			return
		}
	}

	*policies = append(*policies, api.LLMPolicy{
		Name:    name,
		Version: version,
		Paths:   []api.LLMPolicyPath{path},
	})
}

func isBoolTrue(v *bool) bool {
	return v != nil && *v
}

// DeployLLMProxy creates a new immutable deployment artifact and deploys it to a gateway
func (s *LLMProxyDeploymentService) DeployLLMProxy(proxyID string, req *api.DeployRequest, orgUUID string) (*api.DeploymentResponse, error) {
	// Validate request
	if req == nil {
		return nil, constants.ErrInvalidInput
	}
	if req.Base == "" {
		return nil, constants.ErrDeploymentBaseRequired
	}
	if req.GatewayId == (openapi_types.UUID{}) {
		return nil, constants.ErrDeploymentGatewayIDRequired
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
	if gateway == nil || gateway.OrganizationID != orgUUID {
		return nil, constants.ErrGatewayNotFound
	}

	// Get LLM proxy
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, constants.ErrLLMProxyNotFound
	}

	// Validate deployment name is provided
	if req.Name == "" {
		return nil, constants.ErrDeploymentNameRequired
	}

	var baseDeploymentID *string
	var contentBytes []byte

	// Determine the source: "current" or existing deployment
	if req.Base == "current" {
		proxyYaml, err := generateLLMProxyDeploymentYAML(proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to generate LLM proxy deployment YAML: %w", err)
		}
		contentBytes = []byte(proxyYaml)
	} else {
		// Use existing deployment as base
		baseDeployment, err := s.deploymentRepo.GetWithContent(req.Base, proxy.UUID, orgUUID)
		if err != nil {
			if errors.Is(err, constants.ErrDeploymentNotFound) {
				return nil, constants.ErrBaseDeploymentNotFound
			}
			return nil, fmt.Errorf("failed to get base deployment: %w", err)
		}
		contentBytes = baseDeployment.Content
		baseDeploymentID = &req.Base
	}

	// Generate deployment ID
	deploymentID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployment ID: %w", err)
	}
	deployed := model.DeploymentStatusDeployed

	deployment := &model.Deployment{
		DeploymentID:     deploymentID,
		Name:             req.Name,
		ArtifactID:       proxy.UUID,
		OrganizationID:   orgUUID,
		GatewayID:        gatewayID,
		BaseDeploymentID: baseDeploymentID,
		Content:          contentBytes,
		Metadata:         metadata,
		Status:           &deployed,
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway limit config must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	hardLimit := s.cfg.Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer
	if err := s.deploymentRepo.CreateWithLimitEnforcement(deployment, hardLimit); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	// Broadcast LLM proxy deployment event to gateway
	if s.gatewayEventsService != nil {
		vhost := ""
		if proxy.Configuration.Vhost != nil {
			vhost = *proxy.Configuration.Vhost
		}
		deploymentEvent := &model.LLMProxyDeploymentEvent{
			ProxyId:      proxy.ID,
			DeploymentID: deploymentID,
			Vhost:        vhost,
			Environment:  "production",
		}

		if err := s.gatewayEventsService.BroadcastLLMProxyDeploymentEvent(gatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast LLM proxy deployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		model.DeploymentStatusDeployed,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		deployment.UpdatedAt,
	)
}

// RestoreLLMProxyDeployment restores a previous deployment (ARCHIVED or UNDEPLOYED)
func (s *LLMProxyDeploymentService) RestoreLLMProxyDeployment(proxyID, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, constants.ErrLLMProxyNotFound
	}

	targetDeployment, err := s.deploymentRepo.GetWithContent(deploymentID, proxy.UUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if targetDeployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}
	if targetDeployment.GatewayID != gatewayID {
		return nil, constants.ErrGatewayIDMismatch
	}

	currentDeploymentID, status, _, err := s.deploymentRepo.GetStatus(proxy.UUID, orgUUID, targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status: %w", err)
	}
	if currentDeploymentID == deploymentID && status == model.DeploymentStatusDeployed {
		return nil, constants.ErrDeploymentAlreadyDeployed
	}

	gateway, err := s.gatewayRepo.GetByUUID(targetDeployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgUUID {
		return nil, constants.ErrGatewayNotFound
	}

	updatedAt, err := s.deploymentRepo.SetCurrent(proxy.UUID, orgUUID, targetDeployment.GatewayID, deploymentID, model.DeploymentStatusDeployed)
	if err != nil {
		return nil, fmt.Errorf("failed to set current deployment: %w", err)
	}

	// Broadcast LLM proxy deployment event to gateway
	if s.gatewayEventsService != nil {
		vhost := ""
		if proxy.Configuration.Vhost != nil {
			vhost = *proxy.Configuration.Vhost
		}
		deploymentEvent := &model.LLMProxyDeploymentEvent{
			ProxyId:      proxy.ID,
			DeploymentID: deploymentID,
			Vhost:        vhost,
			Environment:  "production",
		}

		if err := s.gatewayEventsService.BroadcastLLMProxyDeploymentEvent(targetDeployment.GatewayID, deploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast LLM proxy deployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
		targetDeployment.DeploymentID,
		targetDeployment.Name,
		targetDeployment.GatewayID,
		model.DeploymentStatusDeployed,
		targetDeployment.BaseDeploymentID,
		targetDeployment.Metadata,
		targetDeployment.CreatedAt,
		&updatedAt,
	)
}

// UndeployLLMProxyDeployment undeploys an active deployment
func (s *LLMProxyDeploymentService) UndeployLLMProxyDeployment(proxyID, deploymentID, gatewayID, orgUUID string) (*api.DeploymentResponse, error) {
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, constants.ErrLLMProxyNotFound
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, proxy.UUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, constants.ErrDeploymentNotFound
	}
	if deployment.GatewayID != gatewayID {
		return nil, constants.ErrGatewayIDMismatch
	}
	if deployment.Status == nil || *deployment.Status != model.DeploymentStatusDeployed {
		return nil, constants.ErrDeploymentNotActive
	}

	gateway, err := s.gatewayRepo.GetByUUID(deployment.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgUUID {
		return nil, constants.ErrGatewayNotFound
	}

	newUpdatedAt, err := s.deploymentRepo.SetCurrent(proxy.UUID, orgUUID, deployment.GatewayID, deploymentID, model.DeploymentStatusUndeployed)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Broadcast LLM proxy undeployment event to gateway
	if s.gatewayEventsService != nil {
		vhost := ""
		if proxy.Configuration.Vhost != nil {
			vhost = *proxy.Configuration.Vhost
		}
		undeploymentEvent := &model.LLMProxyUndeploymentEvent{
			ProxyId:     proxy.ID,
			Vhost:       vhost,
			Environment: "production",
		}

		if err := s.gatewayEventsService.BroadcastLLMProxyUndeploymentEvent(deployment.GatewayID, undeploymentEvent); err != nil {
			s.slogger.Warn("Failed to broadcast LLM proxy undeployment event", "error", err)
		}
	}

	return toAPIDeploymentResponse(
		deployment.DeploymentID,
		deployment.Name,
		deployment.GatewayID,
		model.DeploymentStatusUndeployed,
		deployment.BaseDeploymentID,
		deployment.Metadata,
		deployment.CreatedAt,
		&newUpdatedAt,
	)
}

// DeleteLLMProxyDeployment permanently deletes an undeployed deployment artifact
func (s *LLMProxyDeploymentService) DeleteLLMProxyDeployment(proxyID, deploymentID, orgUUID string) error {
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return err
	}
	if proxy == nil {
		return constants.ErrLLMProxyNotFound
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, proxy.UUID, orgUUID)
	if err != nil {
		return err
	}
	if deployment == nil {
		return constants.ErrDeploymentNotFound
	}
	if deployment.Status != nil && *deployment.Status == model.DeploymentStatusDeployed {
		return constants.ErrDeploymentIsDeployed
	}

	if err := s.deploymentRepo.Delete(deploymentID, proxy.UUID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

// GetLLMProxyDeployments retrieves all deployments for a proxy with optional filters
func (s *LLMProxyDeploymentService) GetLLMProxyDeployments(proxyID, orgUUID string, gatewayID *string, status *string) (*api.DeploymentListResponse, error) {
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, constants.ErrLLMProxyNotFound
	}

	if status != nil {
		validStatuses := map[string]bool{
			string(model.DeploymentStatusDeployed):   true,
			string(model.DeploymentStatusUndeployed): true,
			string(model.DeploymentStatusArchived):   true,
		}
		if !validStatuses[*status] {
			return nil, constants.ErrInvalidDeploymentStatus
		}
	}

	if s.cfg.Deployments.MaxPerAPIGateway < 1 {
		return nil, fmt.Errorf("MaxPerAPIGateway config value must be at least 1, got %d", s.cfg.Deployments.MaxPerAPIGateway)
	}
	deployments, err := s.deploymentRepo.GetDeploymentsWithState(proxy.UUID, orgUUID, gatewayID, status, s.cfg.Deployments.MaxPerAPIGateway)
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

// GetLLMProxyDeployment retrieves a specific deployment by ID
func (s *LLMProxyDeploymentService) GetLLMProxyDeployment(proxyID, deploymentID, orgUUID string) (*api.DeploymentResponse, error) {
	proxy, err := s.proxyRepo.GetByID(proxyID, orgUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, constants.ErrLLMProxyNotFound
	}

	deployment, err := s.deploymentRepo.GetWithState(deploymentID, proxy.UUID, orgUUID)
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
	)
}

func generateLLMProxyDeploymentYAML(proxy *model.LLMProxy) (string, error) {
	if proxy == nil {
		return "", errors.New("proxy is required")
	}
	if proxy.Configuration.Provider == "" {
		return "", constants.ErrInvalidInput
	}

	contextValue := "/"
	if proxy.Configuration.Context != nil && *proxy.Configuration.Context != "" {
		contextValue = *proxy.Configuration.Context
	}
	vhostValue := ""
	if proxy.Configuration.Vhost != nil {
		vhostValue = *proxy.Configuration.Vhost
	}

	policies := make([]api.LLMPolicy, 0, len(proxy.Configuration.Policies))

	// Transform security config
	security := proxy.Configuration.Security
	if security != nil && isBoolTrue(security.Enabled) {
		if security.APIKey != nil && isBoolTrue(security.APIKey.Enabled) {
			key := strings.TrimSpace(security.APIKey.Key)
			if key == "" {
				return "", fmt.Errorf("invalid api key security configuration: key is required")
			}

			in := strings.ToLower(strings.TrimSpace(security.APIKey.In))
			if in != "header" && in != "query" {
				return "", fmt.Errorf("invalid api key security configuration: in must be 'header' or 'query', got %q", security.APIKey.In)
			}

			addOrAppendPolicyPath(&policies, apiKeyAuthPolicyName, apiKeyAuthPolicyVersion, api.LLMPolicyPath{
				Path:    "/*",
				Methods: []api.LLMPolicyPathMethods{"*"},
				Params: map[string]interface{}{
					"key": key,
					"in":  in,
				},
			})
		}
	}

	for _, p := range proxy.Configuration.Policies {
		paths := make([]api.LLMPolicyPath, 0, len(p.Paths))
		for _, pp := range p.Paths {
			methods := make([]api.LLMPolicyPathMethods, 0, len(pp.Methods))
			for _, m := range pp.Methods {
				methods = append(methods, api.LLMPolicyPathMethods(m))
			}
			paths = append(paths, api.LLMPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		policies = append(policies, api.LLMPolicy{Name: p.Name, Version: normalizePolicyVersionToMajor(p.Version), Paths: paths})
	}

	proxyDeployment := dto.LLMProxyDeploymentYAML{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       constants.LLMProxy,
		Metadata: dto.DeploymentMetadata{
			Name: proxy.ID,
		},
		Spec: dto.LLMProxyDeploymentSpec{
			DisplayName: proxy.Name,
			Version:     proxy.Version,
			Context:     contextValue,
			VHost:       vhostValue,
			Provider: dto.LLMProxyDeploymentProvider{
				ID: proxy.Configuration.Provider,
			},
			Policies: policies,
		},
	}

	if proxy.Configuration.UpstreamAuth != nil {
		proxyDeployment.Spec.Provider.Auth = mapModelUpstreamAuthToAPI(proxy.Configuration.UpstreamAuth)
	}

	yamlBytes, err := yaml.Marshal(proxyDeployment)
	if err != nil {
		return "", fmt.Errorf("failed to marshal LLM proxy to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// mapModelAuthToAPI converts model.UpstreamAuth to api.UpstreamAuth with pointer fields
func mapModelAuthToAPI(auth *model.UpstreamAuth) *api.UpstreamAuth {
	if auth == nil {
		return nil
	}
	var authType *api.UpstreamAuthType
	if normalized := normalizeUpstreamAuthType(auth.Type); normalized != "" {
		t := api.UpstreamAuthType(normalized)
		authType = &t
	}
	return &api.UpstreamAuth{
		Type:   authType,
		Header: stringPtrIfNotEmpty(auth.Header),
		Value:  stringPtrIfNotEmpty(auth.Value),
	}
}

// mapModelUpstreamAuthToAPI converts model.UpstreamAuth to api.UpstreamAuth (alias for mapModelAuthToAPI)
func mapModelUpstreamAuthToAPI(auth *model.UpstreamAuth) *api.UpstreamAuth {
	return mapModelAuthToAPI(auth)
}

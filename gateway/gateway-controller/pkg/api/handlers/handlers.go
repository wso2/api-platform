/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
 */

package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/resolver"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"

	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	commonmodels "github.com/wso2/api-platform/common/models"
	adminapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/admin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// APIServer implements the generated ServerInterface
type APIServer struct {
	*RestAPIHandler // embedded — promotes CreateRestAPI, ListRestAPIs, GetRestAPIById, UpdateRestAPI, DeleteRestAPI

	store                       *storage.ConfigStore
	db                          storage.Storage
	snapshotManager             *xds.SnapshotManager
	policyManager               *policyxds.PolicyManager
	policyDefinitions           map[string]models.PolicyDefinition // key name|version
	policyDefMu                 sync.RWMutex
	parser                      *config.Parser
	validator                   config.Validator
	logger                      *slog.Logger
	deploymentService           *utils.APIDeploymentService
	mcpDeploymentService        *utils.MCPDeploymentService
	llmDeploymentService        *utils.LLMDeploymentService
	secretService               *secrets.SecretService
	policyResolver              *resolver.PolicyResolver
	apiKeyService               *utils.APIKeyService
	apiKeyXDSManager            *apikeyxds.APIKeyStateManager
	controlPlaneClient          controlplane.ControlPlaneClient
	routerConfig                *config.RouterConfig
	httpClient                  *http.Client
	systemConfig                *config.Config
	eventHub                    eventhub.EventHub
	gatewayID                   string
	subscriptionSnapshotUpdater utils.SubscriptionSnapshotUpdater
	subscriptionResourceService *utils.SubscriptionResourceService
}

// NewAPIServer creates a new API server with dependencies
func NewAPIServer(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
	lazyResourceManager *lazyresourcexds.LazyResourceStateManager,
	logger *slog.Logger,
	controlPlaneClient controlplane.ControlPlaneClient,
	policyDefinitions map[string]models.PolicyDefinition,
	templateDefinitions map[string]*api.LLMProviderTemplate,
	validator config.Validator,
	apiKeyXDSManager *apikeyxds.APIKeyStateManager,
	systemConfig *config.Config,
	eventHub eventhub.EventHub,
	subscriptionSnapshotUpdater utils.SubscriptionSnapshotUpdater,
	secretService *secrets.SecretService,
	policyResolver *resolver.PolicyResolver,
) *APIServer {
	if db == nil {
		panic("APIServer requires non-nil storage")
	}
	if eventHub == nil {
		panic("APIServer requires non-nil EventHub")
	}
	if systemConfig == nil {
		panic("APIServer requires non-nil system config")
	}
	gatewayID := strings.TrimSpace(systemConfig.Controller.Server.GatewayID)
	if gatewayID == "" {
		panic("APIServer requires non-empty gateway ID")
	}

	deploymentService := utils.NewAPIDeploymentService(store, db, snapshotManager, validator, &systemConfig.Router, policyResolver, eventHub, gatewayID)
	apiKeyService := utils.NewAPIKeyService(store, db, apiKeyXDSManager, &systemConfig.APIKey, eventHub, gatewayID)
	subscriptionResourceService := utils.NewSubscriptionResourceService(db, subscriptionSnapshotUpdater, eventHub, gatewayID)

	policyVersionResolver := utils.NewLoadedPolicyVersionResolver(policyDefinitions)
	policyValidator := config.NewPolicyValidator(policyDefinitions)
	parser := config.NewParser()
	httpClient := &http.Client{Timeout: 10 * time.Second}
	routerConfig := &systemConfig.Router
	mcpDeploymentService := utils.NewMCPDeploymentService(store, db, snapshotManager, policyManager, policyValidator, eventHub, gatewayID)

	server := &APIServer{
		store:                store,
		db:                   db,
		snapshotManager:      snapshotManager,
		policyManager:        policyManager,
		policyDefinitions:    policyDefinitions,
		parser:               parser,
		validator:            validator,
		logger:               logger,
		deploymentService:    deploymentService,
		mcpDeploymentService: mcpDeploymentService,
		llmDeploymentService: utils.NewLLMDeploymentService(store, db, snapshotManager, lazyResourceManager, templateDefinitions,
			deploymentService, routerConfig, policyVersionResolver, policyValidator),
		secretService:               secretService,
		policyResolver:              policyResolver,
		apiKeyService:               apiKeyService,
		apiKeyXDSManager:            apiKeyXDSManager,
		controlPlaneClient:          controlPlaneClient,
		routerConfig:                routerConfig,
		httpClient:                  httpClient,
		systemConfig:                systemConfig,
		eventHub:                    eventHub,
		gatewayID:                   gatewayID,
		subscriptionSnapshotUpdater: subscriptionSnapshotUpdater,
		subscriptionResourceService: subscriptionResourceService,
	}
	// Create RestAPI service and handler
	restAPIService := restapi.NewRestAPIService(
		store, db, snapshotManager, policyManager,
		policyDefinitions, &server.policyDefMu,
		deploymentService, apiKeyXDSManager,
		controlPlaneClient, routerConfig, systemConfig,
		httpClient, parser, validator, logger,
		eventHub, policyResolver,
	)
	server.RestAPIHandler = NewRestAPIHandler(restAPIService, logger)

	// Register status update callback
	snapshotManager.SetStatusCallback(server.handleStatusUpdate)

	return server
}

func (s *APIServer) getSubscriptionResourceService() *utils.SubscriptionResourceService {
	if s.subscriptionResourceService != nil {
		return s.subscriptionResourceService
	}

	s.subscriptionResourceService = utils.NewSubscriptionResourceService(s.db, s.subscriptionSnapshotUpdater, s.eventHub, s.gatewayID)

	return s.subscriptionResourceService
}

// handleStatusUpdate is called by SnapshotManager after xDS deployment
func (s *APIServer) handleStatusUpdate(configID string, success bool, correlationID string) {
	// Create a logger with correlation ID if provided
	log := s.logger
	if correlationID != "" {
		log = s.logger.With(slog.String("correlation_id", correlationID))
	}

	cfg, err := s.store.Get(configID)
	if err != nil {
		log.Warn("Config not found for status update", slog.String("id", configID))
		return
	}

	if success {
		log.Info("Configuration deployed successfully",
			slog.String("id", configID),
			slog.String("kind", cfg.Kind),
			slog.String("handle", cfg.Handle))
	} else {
		log.Error("Configuration deployment failed",
			slog.String("id", configID),
			slog.String("kind", cfg.Kind),
			slog.String("handle", cfg.Handle))
	}
}

// GetXDSSyncStatus implements the GET /xds_sync_status endpoint.
func (s *APIServer) GetXDSSyncStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.GetXDSSyncStatusResponse())
}

// GetXDSSyncStatusResponse builds the xDS sync status response payload.
func (s *APIServer) GetXDSSyncStatusResponse() adminapi.XDSSyncStatusResponse {
	timestamp := time.Now()
	component := "gateway-controller"
	policyChainVersion := s.getPolicyChainVersionString()

	return adminapi.XDSSyncStatusResponse{
		Component:          &component,
		Timestamp:          &timestamp,
		PolicyChainVersion: &policyChainVersion,
	}
}

func (s *APIServer) SearchDeployments(c *gin.Context, kind string) {
	filterKeys := []string{"displayName", "version", "context", "status"}
	filters := make(map[string]string)
	for _, k := range filterKeys {
		if v := c.Query(k); v != "" {
			filters[k] = v
		}
	}

	if s.store == nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"count":  0,
		})
		return
	}

	configs := s.store.GetAllByKind(kind)
	if kind == string(api.Mcp) && s.mcpDeploymentService != nil {
		var err error
		configs, err = s.mcpDeploymentService.ListMCPProxies()
		if err != nil {
			s.logger.Error("Failed to list MCP proxies", slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to list MCP proxies",
			})
			return
		}
	}

	// Filter based on kind to return appropriate response format
	if kind == string(api.Mcp) {
		// Return MCP proxy format
		mcpItems := make([]api.MCPProxyListItem, 0)
		for _, cfg := range configs {
			if v, ok := filters["displayName"]; ok && cfg.DisplayName != v {
				continue
			}
			if v, ok := filters["version"]; ok && cfg.Version != v {
				continue
			}
			cfgContext, err := cfg.GetContext()
			if err != nil {
				s.logger.Warn("Failed to get context for MCP config",
					slog.String("id", cfg.UUID),
					slog.String("displayName", cfg.DisplayName),
					slog.Any("error", err))
				continue
			}
			if v, ok := filters["context"]; ok && cfgContext != v {
				continue
			}
			if v, ok := filters["status"]; ok && string(cfg.DesiredState) != v {
				continue
			}

			status := api.MCPProxyListItemStatus(cfg.DesiredState)
			// Convert SourceConfiguration to MCPProxyConfiguration to get spec fields
			var mcp api.MCPProxyConfiguration
			j, _ := json.Marshal(cfg.SourceConfiguration)
			err = json.Unmarshal(j, &mcp)
			if err != nil {
				s.logger.Error("Failed to unmarshal stored MCP configuration",
					slog.String("id", cfg.UUID),
					slog.String("displayName", cfg.DisplayName))
				continue
			}

			li := api.MCPProxyListItem{
				Id:          stringPtr(cfg.Handle),
				DisplayName: stringPtr(mcp.Spec.DisplayName),
				Version:     stringPtr(mcp.Spec.Version),
				Status:      &status,
				CreatedAt:   timePtr(cfg.CreatedAt),
				UpdatedAt:   timePtr(cfg.UpdatedAt),
			}
			if mcp.Spec.Context != nil {
				li.Context = mcp.Spec.Context
			}
			mcpItems = append(mcpItems, li)
		}

		c.JSON(http.StatusOK, gin.H{
			"status":     "success",
			"count":      len(mcpItems),
			"mcpProxies": mcpItems,
		})
	} else if kind == string(api.WebSubApi) {
		// Return WebSub API format
		websubItems := make([]api.WebSubAPIListItem, 0)
		for _, cfg := range configs {
			if v, ok := filters["displayName"]; ok && cfg.DisplayName != v {
				continue
			}
			if v, ok := filters["version"]; ok && cfg.Version != v {
				continue
			}
			cfgContext, err := cfg.GetContext()
			if err != nil {
				s.logger.Warn("Failed to get context for config",
					slog.String("id", cfg.UUID),
					slog.String("displayName", cfg.DisplayName),
					slog.Any("error", err))
				continue
			}
			if v, ok := filters["context"]; ok && cfgContext != v {
				continue
			}
			if v, ok := filters["status"]; ok && string(cfg.DesiredState) != v {
				continue
			}

			status := string(cfg.DesiredState)
			websubItems = append(websubItems, api.WebSubAPIListItem{
				Id:          stringPtr(cfg.Handle),
				DisplayName: stringPtr(cfg.DisplayName),
				Version:     stringPtr(cfg.Version),
				Context:     stringPtr(cfgContext),
				Status:      (*api.WebSubAPIListItemStatus)(&status),
				CreatedAt:   timePtr(cfg.CreatedAt),
				UpdatedAt:   timePtr(cfg.UpdatedAt),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"status":     "success",
			"count":      len(websubItems),
			"websubApis": websubItems,
		})
	} else {
		// Return REST API format
		apiItems := make([]api.RestAPIListItem, 0)
		for _, cfg := range configs {
			if v, ok := filters["displayName"]; ok && cfg.DisplayName != v {
				continue
			}
			if v, ok := filters["version"]; ok && cfg.Version != v {
				continue
			}
			cfgContext, err := cfg.GetContext()
			if err != nil {
				s.logger.Warn("Failed to get context for config",
					slog.String("id", cfg.UUID),
					slog.String("displayName", cfg.DisplayName),
					slog.Any("error", err))
				continue
			}
			if v, ok := filters["context"]; ok && cfgContext != v {
				continue
			}
			if v, ok := filters["status"]; ok && string(cfg.DesiredState) != v {
				continue
			}

			status := string(cfg.DesiredState)
			apiItems = append(apiItems, api.RestAPIListItem{
				Id:          stringPtr(cfg.Handle),
				DisplayName: stringPtr(cfg.DisplayName),
				Version:     stringPtr(cfg.Version),
				Context:     stringPtr(cfgContext),
				Status:      (*api.RestAPIListItemStatus)(&status),
				CreatedAt:   timePtr(cfg.CreatedAt),
				UpdatedAt:   timePtr(cfg.UpdatedAt),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"count":  len(apiItems),
			"apis":   apiItems,
		})
	}
}

// GetAPIByNameVersion implements ServerInterface.GetAPIByNameVersion
// (GET /apis/{name}/{version})
func (s *APIServer) GetAPIByNameVersion(c *gin.Context, name string, version string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)

	cfg, err := s.store.GetByKindNameAndVersion(models.KindRestApi, name, version)
	if err != nil || cfg == nil {
		log.Warn("API configuration not found",
			slog.String("name", name),
			slog.String("version", version))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("RestAPI with name '%s' and version '%s' not found", name, version),
		})
		return
	}

	apiDetail := gin.H{
		"id":            cfg.Handle,
		"configuration": cfg.Configuration,
		"metadata": gin.H{
			"status":    string(cfg.DesiredState),
			"createdAt": cfg.CreatedAt.Format(time.RFC3339),
			"updatedAt": cfg.UpdatedAt.Format(time.RFC3339),
		},
	}

	if cfg.DeployedAt != nil {
		apiDetail["metadata"].(gin.H)["deployedAt"] = cfg.DeployedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"api":    apiDetail,
	})
}

// CreateWebSubAPI implements ServerInterface.CreateWebSubAPI
// (POST /websub-apis)
func (s *APIServer) CreateWebSubAPI(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Status:  "error",
		Message: "WebSub API management is not implemented",
	})
}

// ListWebSubAPIs implements ServerInterface.ListWebSubAPIs
// (GET /websub-apis)
func (s *APIServer) ListWebSubAPIs(c *gin.Context, params api.ListWebSubAPIsParams) {
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Status:  "error",
		Message: "WebSub API management is not implemented",
	})
}

// GetWebSubAPIById implements ServerInterface.GetWebSubAPIById
// (GET /websub-apis/{id})
func (s *APIServer) GetWebSubAPIById(c *gin.Context, id string) {
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Status:  "error",
		Message: "WebSub API management is not implemented",
	})
}

// UpdateWebSubAPI implements ServerInterface.UpdateWebSubAPI
// (PUT /websub-apis/{id})
func (s *APIServer) UpdateWebSubAPI(c *gin.Context, id string) {
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Status:  "error",
		Message: "WebSub API management is not implemented",
	})
}

// DeleteWebSubAPI implements ServerInterface.DeleteWebSubAPI
// (DELETE /websub-apis/{id})
func (s *APIServer) DeleteWebSubAPI(c *gin.Context, id string) {
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Status:  "error",
		Message: "WebSub API management is not implemented",
	})
}

// CreateLLMProviderTemplate implements ServerInterface.CreateLLMProviderTemplate
// (POST /llm-provider-templates)
func (s *APIServer) CreateLLMProviderTemplate(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	storedTemplate, err := s.llmDeploymentService.CreateLLMProviderTemplate(utils.LLMTemplateParams{
		Spec:          body,
		ContentType:   c.GetHeader("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})

	if err != nil {
		log.Error("Failed to parse template configuration", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to parse template configuration: %v", err),
		})
		return
	}

	log.Info("LLM provider template created successfully",
		slog.String("uuid", storedTemplate.UUID),
		slog.String("handle", storedTemplate.GetHandle()))

	c.JSON(http.StatusCreated, api.LLMProviderTemplateCreateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("LLM provider template created successfully"),
		Id:        stringPtr(storedTemplate.GetHandle()),
		CreatedAt: timePtr(storedTemplate.CreatedAt),
	})
}

// ListLLMProviderTemplates implements ServerInterface.ListLLMProviderTemplates
// (GET /llm-providers/templates)
func (s *APIServer) ListLLMProviderTemplates(c *gin.Context, params api.ListLLMProviderTemplatesParams) {
	templates := s.llmDeploymentService.ListLLMProviderTemplates(params.DisplayName)

	items := make([]api.LLMProviderTemplateListItem, len(templates))
	for i, tmpl := range templates {
		items[i] = api.LLMProviderTemplateListItem{
			Id:          stringPtr(tmpl.GetHandle()),
			DisplayName: stringPtr(tmpl.Configuration.Spec.DisplayName),
			CreatedAt:   timePtr(tmpl.CreatedAt),
			UpdatedAt:   timePtr(tmpl.UpdatedAt),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"count":     len(items),
		"templates": items,
	})
}

// GetLLMProviderTemplateById implements ServerInterface.GetLLMProviderTemplateById
// (GET /llm-provider-templates/{id})
func (s *APIServer) GetLLMProviderTemplateById(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)

	template, err := s.llmDeploymentService.GetLLMProviderTemplateByHandle(id)
	if err != nil {
		log.Warn("LLM provider template not found", slog.String("handle", id))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Template with id '%s' not found", id),
		})
		return
	}

	// Return response with a simple JSON structure similar to GetAPIByNameVersion
	tmplDetail := gin.H{
		"id":            id,
		"configuration": template.Configuration,
		"metadata": gin.H{
			"createdAt": template.CreatedAt,
			"updatedAt": template.UpdatedAt,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"template": tmplDetail,
	})
}

// UpdateLLMProviderTemplate implements ServerInterface.UpdateLLMProviderTemplate
// (PUT /llm-provider-templates/{id})
func (s *APIServer) UpdateLLMProviderTemplate(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	updated, err := s.llmDeploymentService.UpdateLLMProviderTemplate(id, utils.LLMTemplateParams{
		Spec:          body,
		ContentType:   c.GetHeader("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to parse template configuration", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to parse template configuration: %v", err),
		})
		return
	}

	log.Info("LLM provider template updated successfully",
		slog.String("uuid", updated.UUID),
		slog.String("handle", updated.GetHandle()))

	c.JSON(http.StatusOK, api.LLMProviderTemplateUpdateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("LLM provider template updated successfully"),
		Id:        stringPtr(updated.GetHandle()),
		UpdatedAt: timePtr(updated.UpdatedAt),
	})
}

// DeleteLLMProviderTemplate implements ServerInterface.DeleteLLMProviderTemplate
// (DELETE /llm-provider-templates/{id})
func (s *APIServer) DeleteLLMProviderTemplate(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)

	deleted, err := s.llmDeploymentService.DeleteLLMProviderTemplate(id, correlationID, log)
	if err != nil {
		log.Warn("LLM provider template not found for deletion", slog.String("handle", id))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Template with id '%s' not found", id),
		})
		return
	}

	log.Info("LLM provider template deleted successfully",
		slog.String("uuid", deleted.UUID),
		slog.String("handle", deleted.GetHandle()))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "LLM provider template deleted successfully",
		"id":      deleted.GetHandle(),
	})
}

// ListLLMProviders implements ServerInterface.ListLLMProviders
// (GET /llm-providers)
func (s *APIServer) ListLLMProviders(c *gin.Context, params api.ListLLMProvidersParams) {
	log := middleware.GetLogger(c, s.logger)
	configs := s.llmDeploymentService.ListLLMProviders(params)

	items := make([]api.LLMProviderListItem, len(configs))
	for i, cfg := range configs {
		status := api.LLMProviderListItemStatus(cfg.DesiredState)

		// Convert SourceConfiguration to LLMProviderConfiguration
		var prov api.LLMProviderConfiguration
		j, _ := json.Marshal(cfg.SourceConfiguration)
		if err := json.Unmarshal(j, &prov); err != nil {
			log.Error("Failed to unmarshal stored LLM provider configuration",
				slog.String("uuid", cfg.UUID), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error",
				Message: "Failed to get stored LLM provider configuration"})
			return
		}

		items[i] = api.LLMProviderListItem{
			Id:          stringPtr(prov.Metadata.Name),
			DisplayName: stringPtr(prov.Spec.DisplayName),
			Version:     stringPtr(prov.Spec.Version),
			Template:    stringPtr(prov.Spec.Template),
			Status:      &status,
			CreatedAt:   timePtr(cfg.CreatedAt),
			UpdatedAt:   timePtr(cfg.UpdatedAt),
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "count": len(items), "providers": items})
}

// CreateLLMProvider implements ServerInterface.CreateLLMProvider
// (POST /llm-providers)
func (s *APIServer) CreateLLMProvider(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(c)

	// Delegate to service which parses/validates/transforms and persists
	// Important: The result stored configuration contains resolved secrets. Do not expose them in responses.
	result, err := s.llmDeploymentService.CreateLLMProvider(utils.LLMDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		CorrelationID: correlationID,
		Origin:        models.OriginGatewayAPI,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to create LLM provider", slog.Any("error", err))
		if utils.IsPolicyDefinitionMissingError(err) {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: utils.PolicyDefinitionMissingUserMessage,
			})
			return
		}
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	stored := result.StoredConfig

	if !result.IsStale {
		if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() && s.systemConfig.Controller.ControlPlane.DeploymentPushEnabled {
			go s.waitForDeploymentAndPush(stored.UUID, correlationID, log)
		}
	}

	log.Info("LLM provider created successfully",
		slog.String("uuid", stored.UUID),
		slog.String("handle", stored.Handle))

	c.JSON(http.StatusCreated, api.LLMProviderCreateResponse{
		Status:  stringPtr("success"),
		Message: stringPtr("LLM provider created successfully"),
		Id:      stringPtr(stored.Handle), CreatedAt: timePtr(stored.CreatedAt)})

}

// GetLLMProviderById implements ServerInterface.GetLLMProviderById
// (GET /llm-providers/{id})
func (s *APIServer) GetLLMProviderById(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)

	// Service lookup is DB-first so reads still work before this replica has
	// replayed the corresponding EventHub event into its in-memory store.
	cfg, err := s.llmDeploymentService.GetLLMProviderByHandle(id)
	if err != nil {
		if !storage.IsNotFoundError(err) && !strings.Contains(strings.ToLower(err.Error()), "not found") {
			log.Error("Failed to look up LLM provider", slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to look up LLM provider",
			})
			return
		}
		log.Warn("LLM provider configuration not found",
			slog.String("handle", id),
			slog.Any("error", err))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("LLM provider configuration with handle '%s' not found", id),
		})
		return
	}

	// Build response
	providerDetail := gin.H{
		"configuration": cfg.SourceConfiguration,
		"metadata": gin.H{
			"status":    string(cfg.DesiredState),
			"createdAt": cfg.CreatedAt.Format(time.RFC3339),
			"updatedAt": cfg.UpdatedAt.Format(time.RFC3339),
		},
	}

	if cfg.DeployedAt != nil {
		providerDetail["metadata"].(gin.H)["deployedAt"] = cfg.DeployedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"provider": providerDetail,
	})
}

// UpdateLLMProvider implements ServerInterface.UpdateLLMProvider
// (PUT /llm-providers/{id})
func (s *APIServer) UpdateLLMProvider(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID
	correlationID := middleware.GetCorrelationID(c)

	// Delegate to service update wrapper
	// Important: The result stored configuration contains resolved secrets. Do not expose them in responses.
	result, err := s.llmDeploymentService.UpdateLLMProvider(id, utils.LLMDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to update LLM provider configuration", slog.Any("error", err))
		if utils.IsPolicyDefinitionMissingError(err) {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: utils.PolicyDefinitionMissingUserMessage,
			})
			return
		}
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	updated := result.StoredConfig

	c.JSON(http.StatusOK, api.LLMProviderUpdateResponse{
		Id:        stringPtr(updated.Handle),
		Message:   stringPtr("LLM provider updated successfully"),
		Status:    stringPtr("success"),
		UpdatedAt: timePtr(updated.UpdatedAt),
	})

}

// DeleteLLMProvider implements ServerInterface.DeleteLLMProvider
// (DELETE /llm-providers/{id})
func (s *APIServer) DeleteLLMProvider(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)

	cfg, err := s.llmDeploymentService.DeleteLLMProvider(id, correlationID, log)
	if err != nil {
		log.Warn("Failed to delete LLM provider configuration", slog.String("handle", id))
		// Check if it's a not found error
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "LLM provider deleted successfully",
		"id":      cfg.Handle,
	})

}

// ListLLMProxies implements ServerInterface.ListLLMProxies
// (GET /llm-proxies)
func (s *APIServer) ListLLMProxies(c *gin.Context, params api.ListLLMProxiesParams) {
	log := middleware.GetLogger(c, s.logger)
	configs := s.llmDeploymentService.ListLLMProxies(params)

	items := make([]api.LLMProxyListItem, len(configs))
	for i, cfg := range configs {
		status := api.LLMProxyListItemStatus(cfg.DesiredState)

		// Convert SourceConfiguration to LLMProxyConfiguration
		var proxy api.LLMProxyConfiguration
		j, _ := json.Marshal(cfg.SourceConfiguration)
		if err := json.Unmarshal(j, &proxy); err != nil {
			log.Error("Failed to unmarshal stored LLM proxy configuration", slog.String("uuid", cfg.UUID),
				slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status: "error", Message: "Failed to get stored LLM proxy configuration"})
			return
		}

		items[i] = api.LLMProxyListItem{
			Id:          stringPtr(proxy.Metadata.Name),
			DisplayName: stringPtr(proxy.Spec.DisplayName),
			Version:     stringPtr(proxy.Spec.Version),
			Provider:    stringPtr(proxy.Spec.Provider.Id),
			Status:      &status,
			CreatedAt:   timePtr(cfg.CreatedAt),
			UpdatedAt:   timePtr(cfg.UpdatedAt),
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "count": len(items), "proxies": items})
}

// CreateLLMProxy implements ServerInterface.CreateLLMProxy
// (POST /llm-proxies)
func (s *APIServer) CreateLLMProxy(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(c)

	// Delegate to service which parses/validates/transforms and persists
	// Important: The result stored configuration contains resolved secrets. Do not expose them in responses.
	result, err := s.llmDeploymentService.CreateLLMProxy(utils.LLMDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
		Origin:        models.OriginGatewayAPI,
	})
	if err != nil {
		log.Error("Failed to create LLM proxy", slog.Any("error", err))
		if utils.IsPolicyDefinitionMissingError(err) {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: utils.PolicyDefinitionMissingUserMessage,
			})
			return
		}
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	stored := result.StoredConfig

	if !result.IsStale {
		if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() && s.systemConfig.Controller.ControlPlane.DeploymentPushEnabled {
			go s.waitForDeploymentAndPush(stored.UUID, correlationID, log)
		}
	}

	log.Info("LLM proxy created successfully",
		slog.String("uuid", stored.UUID),
		slog.String("handle", stored.Handle))

	c.JSON(http.StatusCreated, api.LLMProxyCreateResponse{
		Status:  stringPtr("success"),
		Message: stringPtr("LLM proxy created successfully"),
		Id:      stringPtr(stored.Handle), CreatedAt: timePtr(stored.CreatedAt)})

}

// GetLLMProxyById implements ServerInterface.GetLLMProxyById
// (GET /llm-proxies/{id})
func (s *APIServer) GetLLMProxyById(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)

	cfg, err := s.llmDeploymentService.GetLLMProxyByHandle(id)
	if err != nil {
		if !storage.IsNotFoundError(err) && !strings.Contains(strings.ToLower(err.Error()), "not found") {
			log.Error("Failed to look up LLM proxy", slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to look up LLM proxy",
			})
			return
		}
		log.Warn("LLM proxy configuration not found",
			slog.String("handle", id),
			slog.Any("error", err))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("LLM proxy configuration with handle '%s' not found", id),
		})
		return
	}

	// Build response
	proxyDetail := gin.H{
		"configuration": cfg.SourceConfiguration,
		"metadata": gin.H{
			"status":    string(cfg.DesiredState),
			"createdAt": cfg.CreatedAt.Format(time.RFC3339),
			"updatedAt": cfg.UpdatedAt.Format(time.RFC3339),
		},
	}

	if cfg.DeployedAt != nil {
		proxyDetail["metadata"].(gin.H)["deployedAt"] = cfg.DeployedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"proxy":  proxyDetail,
	})
}

// UpdateLLMProxy implements ServerInterface.UpdateLLMProxy
// (PUT /llm-proxies/{id})
func (s *APIServer) UpdateLLMProxy(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID
	correlationID := middleware.GetCorrelationID(c)

	// Delegate to service update wrapper
	result, err := s.llmDeploymentService.UpdateLLMProxy(id, utils.LLMDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to update LLM proxy configuration", slog.Any("error", err))
		if utils.IsPolicyDefinitionMissingError(err) {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: utils.PolicyDefinitionMissingUserMessage,
			})
			return
		}
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	updated := result.StoredConfig

	c.JSON(http.StatusOK, api.LLMProxyUpdateResponse{
		Id:        stringPtr(updated.Handle),
		Message:   stringPtr("LLM proxy updated successfully"),
		Status:    stringPtr("success"),
		UpdatedAt: timePtr(updated.UpdatedAt),
	})

}

// DeleteLLMProxy implements ServerInterface.DeleteLLMProxy
// (DELETE /llm-proxies/{id})
func (s *APIServer) DeleteLLMProxy(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)

	cfg, err := s.llmDeploymentService.DeleteLLMProxy(id, correlationID, log)
	if err != nil {
		log.Warn("Failed to delete LLM proxy configuration", slog.String("handle", id), slog.Any("error", err))
		// Check if it's a not found error
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "LLM proxy deleted successfully",
		"id":      cfg.Handle,
	})

}

// convertAPIPolicy converts generated api.Policy to policyenginev1.PolicyInstance.
// resolvedVersion is the full semver (e.g. v1.0.0) to send to the policy engine.
func convertAPIPolicy(p api.Policy, attachedTo policy.Level, resolvedVersion string) policyenginev1.PolicyInstance {
	paramsMap := make(map[string]interface{})
	if p.Params != nil {
		for k, v := range *p.Params {
			paramsMap[k] = v
		}
	}

	// Add attachedTo metadata to parameters
	if attachedTo != "" {
		paramsMap["attachedTo"] = string(attachedTo)
	}

	return policyenginev1.PolicyInstance{
		Name:               p.Name,
		Version:            resolvedVersion,
		Enabled:            true, // Default to enabled
		ExecutionCondition: p.ExecutionCondition,
		Parameters:         paramsMap,
	}
}

// CreateMCPProxy implements ServerInterface.CreateMCPProxy
// (POST /mcp-proxies)
func (s *APIServer) CreateMCPProxy(c *gin.Context) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(c)

	// Deploy MCP configuration using the utility service
	result, err := s.mcpDeploymentService.CreateMCPProxy(utils.MCPDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		ID:            "", // Empty to generate new UUID
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	})

	if err != nil {
		log.Error("Failed to deploy MCP proxy configuration", slog.Any("error", err))
		if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		}
		return
	}

	cfg := result.StoredConfig

	// Return success response (id is the handle)
	c.JSON(http.StatusCreated, api.MCPProxyCreateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("MCP proxy configuration created successfully"),
		Id:        stringPtr(cfg.Handle),
		CreatedAt: timePtr(cfg.CreatedAt),
	})

	if result.IsStale {
		return
	}

	if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() && s.systemConfig.Controller.ControlPlane.DeploymentPushEnabled {
		go s.waitForDeploymentAndPush(cfg.UUID, correlationID, log)
	}

}

// ListMCPProxies implements ServerInterface.ListMCPProxies
// (GET /mcp-proxies)
func (s *APIServer) ListMCPProxies(c *gin.Context, params api.ListMCPProxiesParams) {
	if (params.DisplayName != nil && *params.DisplayName != "") || (params.Version != nil && *params.Version != "") || (params.Context != nil && *params.Context != "") || (params.Status != nil && *params.Status != "") {
		s.SearchDeployments(c, string(api.Mcp))
		return
	}
	configs, err := s.mcpDeploymentService.ListMCPProxies()
	if err != nil {
		s.logger.Error("Failed to list MCP proxies", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to list MCP proxies",
		})
		return
	}

	items := make([]api.MCPProxyListItem, len(configs))
	for i, cfg := range configs {
		status := api.MCPProxyListItemStatus(cfg.DesiredState)
		// Convert SourceConfiguration to MCPProxyConfiguration
		var mcp api.MCPProxyConfiguration
		j, _ := json.Marshal(cfg.SourceConfiguration)
		err := json.Unmarshal(j, &mcp)
		if err != nil {
			s.logger.Error("Failed to unmarshal stored MCP configuration",
				slog.String("id", cfg.UUID),
				slog.String("displayName", cfg.DisplayName))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to get stored MCP configuration",
			})
			return
		}
		li := api.MCPProxyListItem{
			Id:          stringPtr(cfg.Handle),
			DisplayName: stringPtr(mcp.Spec.DisplayName),
			Version:     stringPtr(mcp.Spec.Version),
			Status:      &status,
			CreatedAt:   timePtr(cfg.CreatedAt),
			UpdatedAt:   timePtr(cfg.UpdatedAt),
		}
		if mcp.Spec.Context != nil {
			li.Context = mcp.Spec.Context
		}
		items[i] = li
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"count":      len(items),
		"mcpProxies": items,
	})
}

// GetMCPProxyById implements ServerInterface.GetMCPProxyById
// (GET /mcp-proxies/{id})
func (s *APIServer) GetMCPProxyById(c *gin.Context, id string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)

	handle := id

	cfg, err := s.mcpDeploymentService.GetMCPProxyByHandle(handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			c.JSON(http.StatusServiceUnavailable, api.ErrorResponse{
				Status:  "error",
				Message: "Database storage not available",
			})
			return
		}
		if strings.Contains(err.Error(), "not found") {
			log.Warn("MCP proxy configuration not found",
				slog.String("handle", handle))
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("MCP proxy configuration with handle '%s' not found", handle),
			})
			return
		}

		log.Error("Failed to retrieve MCP proxy configuration",
			slog.String("handle", handle),
			slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve MCP proxy configuration",
		})
		return

	}

	// Check deployment kind is MCP
	if cfg.Kind != string(api.Mcp) {
		log.Warn("Configuration kind mismatch",
			slog.String("expected", string(api.Mcp)),
			slog.String("actual", cfg.Kind),
			slog.String("handle", handle))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Configuration with handle '%s' is not of kind MCP", handle),
		})
		return
	}

	mcpDetail := gin.H{
		"id":            cfg.Handle,
		"configuration": cfg.SourceConfiguration,
		"metadata": gin.H{
			"status":    string(cfg.DesiredState),
			"createdAt": cfg.CreatedAt.Format(time.RFC3339),
			"updatedAt": cfg.UpdatedAt.Format(time.RFC3339),
		},
	}

	if cfg.DeployedAt != nil {
		mcpDetail["metadata"].(gin.H)["deployedAt"] = cfg.DeployedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"mcp":    mcpDetail,
	})
}

// UpdateMCPProxy implements ServerInterface.UpdateMCPProxy
// (PUT /mcp-proxies/{handle})
func (s *APIServer) UpdateMCPProxy(c *gin.Context, id string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)

	handle := id

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID
	correlationID := middleware.GetCorrelationID(c)

	// Delegate to service update wrapper
	updated, err := s.mcpDeploymentService.UpdateMCPProxy(handle, utils.MCPDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	}, log)

	if err != nil {
		log.Warn("MCP proxy configuration not found",
			slog.String("handle", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("MCP configuration with handle '%s' not found", handle),
		})
		return
	}

	log.Info("MCP proxy configuration updated",
		slog.String("id", updated.UUID),
		slog.String("handle", handle))

	// Return success response (id is the handle)
	c.JSON(http.StatusOK, api.MCPProxyUpdateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("MCP proxy configuration updated successfully"),
		Id:        stringPtr(updated.Handle),
		UpdatedAt: timePtr(updated.UpdatedAt),
	})
}

// DeleteMCPProxy implements ServerInterface.DeleteMCPProxy
// (DELETE /mcp-proxies/{handle})
func (s *APIServer) DeleteMCPProxy(c *gin.Context, id string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)

	handle := id
	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(c)

	cfg, err := s.mcpDeploymentService.DeleteMCPProxy(handle, correlationID, log)
	if err != nil {
		log.Warn("Failed to delete MCP proxy configuration", slog.String("handle", handle), slog.Any("error", err))
		// Check if it's a not found error
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	log.Info("MCP proxy configuration deleted",
		slog.String("id", cfg.UUID),
		slog.String("handle", handle))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MCP proxy configuration deleted successfully",
		"id":      handle,
	})
}

// waitForDeploymentAndPush waits for API deployment to complete and pushes it to the control plane
// This is only called for APIs created directly via gateway endpoint (not from platform API)
func (s *APIServer) waitForDeploymentAndPush(configID string, correlationID string, log *slog.Logger) {
	// Create a logger with correlation ID if provided
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	// Poll for deployment status with timeout
	timeout := time.NewTimer(30 * time.Second)       // 30 second timeout
	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms
	defer timeout.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			log.Warn("Timeout waiting for API deployment to complete before pushing to control plane",
				slog.String("config_id", configID))
			return

		case <-ticker.C:
			cfg, err := s.store.Get(configID)
			if err != nil {
				log.Warn("Config not found while waiting for deployment completion",
					slog.String("config_id", configID))
				return
			}

			if cfg.DeployedAt != nil {
				log.Info("API deployed successfully, pushing to control plane",
					slog.String("config_id", configID),
					slog.String("displayName", cfg.DisplayName))

				apiID := configID
				deploymentID := cfg.DeploymentID

				if err := s.controlPlaneClient.PushAPIDeployment(apiID, cfg, deploymentID); err != nil {
					log.Error("Failed to push deployment to control plane",
						slog.String("api_id", apiID),
						slog.Any("error", err))
				} else {
					log.Info("Successfully pushed deployment to control plane",
						slog.String("api_id", apiID))
				}
				return
			}
		}
	}
}

// GetConfigDump implements the GET /config_dump endpoint
func (s *APIServer) GetConfigDump(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)
	log.Info("Retrieving configuration dump")

	response, err := s.BuildConfigDumpResponse(log)
	if err != nil {
		log.Error("Failed to retrieve configuration dump", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, *response)
	log.Info("Configuration dump retrieved successfully",
		slog.Int("apis", len(*response.Apis)),
		slog.Int("policies", len(*response.Policies)),
		slog.Int("certificates", len(*response.Certificates)))
}

// BuildConfigDumpResponse builds the complete configuration dump response payload.
func (s *APIServer) BuildConfigDumpResponse(log *slog.Logger) (*adminapi.ConfigDumpResponse, error) {
	log.Info("Retrieving configuration dump")

	// Get all APIs
	allConfigs := s.store.GetAll()

	// Build API list with metadata using the generated types
	apisSlice := make([]adminapi.ConfigDumpAPIItem, 0, len(allConfigs))

	for _, cfg := range allConfigs {
		// Use handle (metadata.name) as the id in the dump
		configHandle := cfg.Handle
		if configHandle == "" {
			log.Warn("Config missing handle, skipping in dump", slog.String("id", cfg.UUID))
			continue
		}

		// Convert desired state to the admin API status type
		var status adminapi.ConfigDumpAPIMetadataStatus
		switch cfg.DesiredState {
		case models.StateDeployed:
			status = adminapi.Deployed
		case models.StateUndeployed:
			status = adminapi.Undeployed
		default:
			status = adminapi.Deployed
		}

		configuration, err := toGenericMap(cfg.Configuration)
		if err != nil {
			return nil, fmt.Errorf("failed to convert API configuration: %w", err)
		}

		item := adminapi.ConfigDumpAPIItem{
			Configuration: &configuration,
			Id:            convertHandleToUUID(configHandle),
			Metadata: &adminapi.ConfigDumpAPIMetadata{
				CreatedAt:  &cfg.CreatedAt,
				UpdatedAt:  &cfg.UpdatedAt,
				DeployedAt: cfg.DeployedAt,
				Status:     &status,
			},
		}
		apisSlice = append(apisSlice, item)
	}

	// Get all policies (excluding system policies)
	s.policyDefMu.RLock()
	policies := make([]map[string]interface{}, 0, len(s.policyDefinitions))
	for _, policy := range s.policyDefinitions {
		policyMap, err := toGenericMap(policy)
		if err != nil {
			s.policyDefMu.RUnlock()
			return nil, fmt.Errorf("failed to convert policy definition: %w", err)
		}
		policies = append(policies, policyMap)
	}
	s.policyDefMu.RUnlock()

	// Sort policies for consistent output
	sort.Slice(policies, func(i, j int) bool {
		nameI, _ := policies[i]["name"].(string)
		nameJ, _ := policies[j]["name"].(string)
		if nameI == nameJ {
			versionI, _ := policies[i]["version"].(string)
			versionJ, _ := policies[j]["version"].(string)
			return versionI < versionJ
		}
		return nameI < nameJ
	})

	// Get all certificates
	certificates := make([]adminapi.CertificateResponse, 0)
	totalBytes := 0

	certs, err := s.db.ListCertificates()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve certificates: %w", err)
	}

	for _, cert := range certs {
		totalBytes += len(cert.Certificate)

		certStatus := "success"
		certificates = append(certificates, adminapi.CertificateResponse{
			Id:       &cert.UUID,
			Name:     &cert.Name,
			Subject:  &cert.Subject,
			Issuer:   &cert.Issuer,
			NotAfter: &cert.NotAfter,
			Count:    &cert.CertCount,
			Status:   &certStatus,
		})
	}

	// Calculate statistics
	totalApis := len(apisSlice)
	totalPolicies := len(policies)
	totalCertificates := len(certificates)

	timestamp := time.Now()
	status := "success"
	policyChainVersion := s.getPolicyChainVersionString()

	// Build response
	response := &adminapi.ConfigDumpResponse{
		Status:       &status,
		Timestamp:    &timestamp,
		Apis:         &apisSlice,
		Policies:     &policies,
		Certificates: &certificates,
		Statistics: &struct {
			TotalApis             *int `json:"totalApis,omitempty" yaml:"totalApis,omitempty"`
			TotalCertificateBytes *int `json:"totalCertificateBytes,omitempty" yaml:"totalCertificateBytes,omitempty"`
			TotalCertificates     *int `json:"totalCertificates,omitempty" yaml:"totalCertificates,omitempty"`
			TotalPolicies         *int `json:"totalPolicies,omitempty" yaml:"totalPolicies,omitempty"`
		}{
			TotalApis:             &totalApis,
			TotalPolicies:         &totalPolicies,
			TotalCertificates:     &totalCertificates,
			TotalCertificateBytes: &totalBytes,
		},
		XdsSync: &adminapi.ConfigDumpXDSSync{
			PolicyChainVersion: &policyChainVersion,
		},
	}

	return response, nil
}

func (s *APIServer) getPolicyChainVersionString() string {
	if s.policyManager == nil {
		return "0"
	}
	return strconv.FormatInt(s.policyManager.GetResourceVersion(), 10)
}

// CreateAPIKey implements ServerInterface.CreateAPIKey
// (POST /apis/{id}/api-keys)
// Handles both local key generation and external key injection based on request payload
func (s *APIServer) CreateAPIKey(c *gin.Context, id string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	// Extract authenticated user from context
	user, ok := s.extractAuthenticatedUser(c, "CreateAPIKey", correlationID)
	if !ok {
		return // Error response already sent by extractAuthenticatedUser
	}

	log.Debug("Starting API key creation by generating or injecting a new key",
		slog.String("handle", handle),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Parse and validate request body
	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		log.Error("Failed to parse request body for API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// Prepare parameters
	params := utils.APIKeyCreationParams{
		Kind:          models.KindRestApi,
		Handle:        handle,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.CreateAPIKey(params)
	if err != nil {
		// Check error type to determine appropriate status code
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		}
		return
	}

	log.Info("API key creation completed",
		slog.String("handle", handle),
		slog.String("key name", result.Response.ApiKey.Name),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Return the response using the generated schema
	c.JSON(http.StatusCreated, result.Response)
}

// RevokeAPIKey implements ServerInterface.RevokeAPIKey
// (DELETE /apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) RevokeAPIKey(c *gin.Context, id string, apiKeyName string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	// Extract authenticated user from context
	user, ok := s.extractAuthenticatedUser(c, "RevokeAPIKey", correlationID)
	if !ok {
		return // Error response already sent by extractAuthenticatedUser
	}

	log.Debug("Starting API key revocation",
		slog.String("handle", handle),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Prepare parameters
	params := utils.APIKeyRevocationParams{
		Handle:        handle,
		APIKeyName:    apiKeyName,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RevokeAPIKey(params)
	if err != nil {
		// Check error type to determine appropriate status code
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		}
		return
	}

	log.Info("API key revoked successfully",
		slog.String("handle", handle),
		slog.String("key", apiKeyName),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Return the response using the generated schema
	c.JSON(http.StatusOK, result.Response)
}

// UpdateAPIKey implements ServerInterface.UpdateAPIKey
// (PUT /apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) UpdateAPIKey(c *gin.Context, id string, apiKeyName string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	// Extract authenticated user from context
	user, ok := s.extractAuthenticatedUser(c, "UpdateAPIKey", correlationID)
	if !ok {
		return // Error response already sent by extractAuthenticatedUser
	}

	log.Debug("Starting API key update",
		slog.String("handle", handle),
		slog.String("key_name", apiKeyName),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Parse and validate request body
	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		log.Warn("Invalid request body for API key update",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// If plain-text API key is not provided, return an error
	if request.ApiKey == nil || strings.TrimSpace(*request.ApiKey) == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "apiKey is required",
		})
		return
	}

	// Prepare parameters
	params := utils.APIKeyUpdateParams{
		Kind:          models.KindRestApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.UpdateAPIKey(params)
	if err != nil {
		// Check error type to determine appropriate status code
		if storage.IsOperationNotAllowedError(err) {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		}
		return
	}

	log.Info("API key updated successfully",
		slog.String("handle", handle),
		slog.String("key_name", apiKeyName),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	c.JSON(http.StatusOK, result.Response)
}

// RegenerateAPIKey implements ServerInterface.RegenerateAPIKey
// (POST /apis/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RegenerateAPIKey(c *gin.Context, id string, apiKeyName string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	// Extract authenticated user from context
	user, ok := s.extractAuthenticatedUser(c, "RegenerateAPIKey", correlationID)
	if !ok {
		return // Error response already sent by extractAuthenticatedUser
	}

	log.Debug("Starting API key rotation",
		slog.String("handle", handle),
		slog.String("key name", apiKeyName),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Parse and validate request body
	var request api.APIKeyRegenerationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		log.Warn("Invalid request body for API key rotation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// Prepare parameters
	params := utils.APIKeyRegenerationParams{
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RegenerateAPIKey(params)
	if err != nil {
		// Check error type to determine appropriate status code
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		}
		return
	}

	log.Info("API key rotation completed",
		slog.String("handle", handle),
		slog.String("key_name", apiKeyName),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	c.JSON(http.StatusOK, result.Response)
}

// ListAPIKeys implements ServerInterface.ListAPIKeys
// (GET /apis/{id}/api-keys)
func (s *APIServer) ListAPIKeys(c *gin.Context, id string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	// Extract authenticated user from context
	user, ok := s.extractAuthenticatedUser(c, "ListAPIKeys", correlationID)
	if !ok {
		return // Error response already sent by extractAuthenticatedUser
	}

	log.Debug("Starting API key listing",
		slog.String("handle", handle),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Prepare parameters
	params := utils.ListAPIKeyParams{
		Handle:        handle,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.ListAPIKeys(params)
	if err != nil {
		// Check error type to determine appropriate status code
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		}
		return
	}

	log.Info("API key listing completed",
		slog.String("handle", handle),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Return the response using the generated schema
	c.JSON(http.StatusOK, result.Response)
}

// resolveAPIIDByHandle resolves an API identifier (deployment ID or handle) to the internal deployment ID.
// It first attempts a direct ID lookup; if that fails, it falls back to handle-based resolution.
// Returns (apiID, nil) on success; on failure writes the HTTP response and returns ("", err).
func (s *APIServer) resolveAPIIDByHandle(c *gin.Context, handle string, log *slog.Logger) (string, error) {
	// First, try treating the input as a deployment ID.
	cfgByID, err := s.db.GetConfig(handle)
	if err != nil {
		if !storage.IsNotFoundError(err) {
			log.Error("Failed to look up API configuration by ID",
				slog.String("id", handle),
				slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to resolve API identifier",
			})
			return "", fmt.Errorf("database error")
		}
	} else if cfgByID != nil {
		if cfgByID.Kind != string(api.RestApi) {
			log.Warn("Configuration is not a REST API",
				slog.String("id", handle),
				slog.String("kind", cfgByID.Kind))
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("Configuration with identifier '%s' is not a REST API", handle),
			})
			return "", fmt.Errorf("invalid api kind")
		}
		return cfgByID.UUID, nil
	}

	// Fallback: resolve by handle (metadata.name)
	cfg, err := s.db.GetConfigByKindAndHandle(models.KindRestApi, handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			log.Warn("API configuration not found", slog.String("handle_or_id", handle))
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("RestAPI with identifier '%s' not found", handle),
			})
			return "", fmt.Errorf("api not found")
		}
		log.Error("Failed to look up API configuration by handle",
			slog.String("handle", handle),
			slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to resolve API identifier",
		})
		return "", fmt.Errorf("database error")
	}
	if cfg == nil {
		log.Warn("API configuration not found", slog.String("handle_or_id", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("RestAPI with identifier '%s' not found", handle),
		})
		return "", fmt.Errorf("api not found")
	}
	return cfg.UUID, nil
}

// CreateSubscription implements ServerInterface.CreateSubscription (POST /subscriptions)
func (s *APIServer) CreateSubscription(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	var req api.SubscriptionCreateRequest
	if err := s.bindRequestBody(c, &req); err != nil {
		log.Warn("Invalid subscription create body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	if strings.TrimSpace(req.ApiId) == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "apiId is required"})
		return
	}
	if strings.TrimSpace(req.SubscriptionToken) == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "subscriptionToken is required"})
		return
	}

	// Resolve apiId (deployment ID or handle) to the internal deployment ID used for persistence.
	apiID, err := s.resolveAPIIDByHandle(c, req.ApiId, log)
	if err != nil {
		// resolveAPIIDByHandle already wrote the appropriate response.
		return
	}

	// Validate subscription plan when provided: must exist, be ACTIVE, and be enabled for this API.
	if req.SubscriptionPlanId != nil && *req.SubscriptionPlanId != "" {
		plan, err := s.db.GetSubscriptionPlanByID(*req.SubscriptionPlanId, "")
		if err != nil || plan == nil {
			log.Warn("Subscription plan not found for subscription creation",
				slog.String("subscription_plan_id", *req.SubscriptionPlanId),
				slog.String("api_id", apiID))
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: "Subscription plan not found or not enabled",
			})
			return
		}
		if plan.Status != models.SubscriptionPlanStatusActive {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: "Subscription plan is not active",
			})
			return
		}
		cfg, err := s.db.GetConfig(apiID)
		if err != nil || cfg == nil {
			log.Error("Failed to load API configuration for subscription plan validation",
				slog.String("api_id", apiID), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to validate subscription plan",
			})
			return
		}
		if cfg.Kind == string(api.RestApi) {
			if restAPI, ok := cfg.Configuration.(api.RestAPI); ok {
				if restAPI.Spec.SubscriptionPlans != nil && len(*restAPI.Spec.SubscriptionPlans) > 0 {
					enabled := false
					for _, name := range *restAPI.Spec.SubscriptionPlans {
						if strings.EqualFold(name, plan.PlanName) {
							enabled = true
							break
						}
					}
					if !enabled {
						c.JSON(http.StatusBadRequest, api.ErrorResponse{
							Status:  "error",
							Message: fmt.Sprintf("Subscription plan %q is not enabled for this API", plan.PlanName),
						})
						return
					}
				}
			}
		}
	}

	status := models.SubscriptionStatusActive
	if req.Status != nil {
		st := models.SubscriptionStatus(*req.Status)
		switch st {
		case models.SubscriptionStatusActive,
			models.SubscriptionStatusInactive,
			models.SubscriptionStatusRevoked:
			status = st
		default:
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("invalid status: %s", *req.Status),
			})
			return
		}
	}
	var appID *string
	if req.ApplicationId != nil && *req.ApplicationId != "" {
		appID = req.ApplicationId
	}
	sub := &models.Subscription{
		ID:                 uuid.New().String(),
		APIID:              apiID,
		ApplicationID:      appID,
		SubscriptionPlanID: req.SubscriptionPlanId,
		Status:             status,
		SubscriptionToken:  strings.TrimSpace(req.SubscriptionToken),
	}
	if err := s.getSubscriptionResourceService().SaveSubscription(sub, correlationID, log); err != nil {
		if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: "Application already subscribed to this API"})
			return
		}
		log.Error("Failed to save subscription", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create subscription"})
		return
	}
	resp := subscriptionToResponseWithToken(sub)
	c.JSON(http.StatusCreated, resp)
}

// ListSubscriptions implements ServerInterface.ListSubscriptions (GET /subscriptions)
func (s *APIServer) ListSubscriptions(c *gin.Context, params api.ListSubscriptionsParams) {
	log := middleware.GetLogger(c, s.logger)

	var apiID, appID, status *string
	if params.ApiId != nil && *params.ApiId != "" {
		// Normalize apiId to the internal deployment ID (accepts handle or deployment ID).
		resolvedID, err := s.resolveAPIIDByHandle(c, *params.ApiId, log)
		if err != nil {
			// resolveAPIIDByHandle already wrote the response.
			return
		}
		apiIDCopy := resolvedID
		apiID = &apiIDCopy
	}
	if params.ApplicationId != nil && *params.ApplicationId != "" {
		appID = params.ApplicationId
	}
	if params.Status != nil && *params.Status != "" {
		st := string(*params.Status)
		status = &st
	}
	// apiId is an optional filter. When omitted, all subscriptions for this gateway are returned
	// (optionally filtered by applicationId and/or status).
	apiIDValue := ""
	if apiID != nil {
		apiIDValue = *apiID
	}
	list, err := s.db.ListSubscriptionsByAPI(apiIDValue, "", appID, status)
	if err != nil {
		log.Error("Failed to list subscriptions", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list subscriptions"})
		return
	}
	out := make([]api.SubscriptionResponse, 0, len(list))
	for _, sub := range list {
		out = append(out, subscriptionToResponse(sub))
	}
	c.JSON(http.StatusOK, api.SubscriptionListResponse{
		Subscriptions: &out,
		Count:         ptr(int(len(list))),
	})
}

// GetSubscription implements ServerInterface.GetSubscription (GET /subscriptions/{subscriptionId})
func (s *APIServer) GetSubscription(c *gin.Context, subscriptionId string) {
	log := middleware.GetLogger(c, s.logger)

	sub, err := s.db.GetSubscriptionByID(subscriptionId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to get subscription", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription"})
		return
	}
	if sub == nil {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
		return
	}
	c.JSON(http.StatusOK, subscriptionToResponse(sub))
}

// UpdateSubscription implements ServerInterface.UpdateSubscription (PUT /subscriptions/{subscriptionId})
func (s *APIServer) UpdateSubscription(c *gin.Context, subscriptionId string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	sub, err := s.db.GetSubscriptionByID(subscriptionId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to get subscription for update", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription"})
		return
	}
	if sub == nil {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
		return
	}
	var req api.SubscriptionUpdateRequest
	if err := s.bindRequestBody(c, &req); err != nil {
		log.Warn("Invalid subscription update body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	if req.Status != nil {
		st := models.SubscriptionStatus(*req.Status)
		switch st {
		case models.SubscriptionStatusActive,
			models.SubscriptionStatusInactive,
			models.SubscriptionStatusRevoked:
			sub.Status = st
		default:
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("invalid status: %s", *req.Status),
			})
			return
		}
	}
	if err := s.getSubscriptionResourceService().UpdateSubscription(sub, correlationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to update subscription", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update subscription"})
		return
	}
	c.JSON(http.StatusOK, subscriptionToResponse(sub))
}

// DeleteSubscription implements ServerInterface.DeleteSubscription (DELETE /subscriptions/{subscriptionId})
func (s *APIServer) DeleteSubscription(c *gin.Context, subscriptionId string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	sub, err := s.db.GetSubscriptionByID(subscriptionId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to get subscription for deletion", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription"})
		return
	}
	if sub == nil {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
		return
	}
	if err := s.getSubscriptionResourceService().DeleteSubscription(subscriptionId, correlationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to delete subscription", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to delete subscription"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ========================================
// Subscription Plan Handlers
// ========================================

// validateThrottleLimits ensures throttleLimitCount and throttleLimitUnit are provided together,
// count is positive, and unit is one of Day, Hour, Min, Month.
func validateThrottleLimits(count *int, unit *string) error {
	countProvided := count != nil
	unitProvided := unit != nil && *unit != ""
	if countProvided != unitProvided {
		return fmt.Errorf("throttleLimitCount and throttleLimitUnit must be provided together")
	}
	if !countProvided {
		return nil
	}
	if *count <= 0 {
		return fmt.Errorf("throttleLimitCount must be positive")
	}
	switch *unit {
	case "Day", "Hour", "Min", "Month":
		return nil
	default:
		return fmt.Errorf("throttleLimitUnit must be one of: Day, Hour, Min, Month")
	}
}

// CreateSubscriptionPlan implements ServerInterface.CreateSubscriptionPlan (POST /subscription-plans)
func (s *APIServer) CreateSubscriptionPlan(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	var req api.SubscriptionPlanCreateRequest
	if err := s.bindRequestBody(c, &req); err != nil {
		log.Warn("Invalid subscription plan create body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	planName := strings.TrimSpace(req.PlanName)
	if planName == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "planName is required"})
		return
	}

	var unitStr *string
	if req.ThrottleLimitUnit != nil {
		s := string(*req.ThrottleLimitUnit)
		unitStr = &s
	}
	if err := validateThrottleLimits(req.ThrottleLimitCount, unitStr); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	status := models.SubscriptionPlanStatusActive
	if req.Status != nil {
		st := models.SubscriptionPlanStatus(*req.Status)
		switch st {
		case models.SubscriptionPlanStatusActive, models.SubscriptionPlanStatusInactive:
			status = st
		default:
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("invalid status: %s", *req.Status)})
			return
		}
	}

	plan := &models.SubscriptionPlan{
		ID:               uuid.New().String(),
		PlanName:         planName,
		StopOnQuotaReach: true,
		Status:           status,
	}
	if req.BillingPlan != nil {
		plan.BillingPlan = req.BillingPlan
	}
	if req.StopOnQuotaReach != nil {
		plan.StopOnQuotaReach = *req.StopOnQuotaReach
	}
	if req.ThrottleLimitCount != nil && req.ThrottleLimitUnit != nil {
		s := string(*req.ThrottleLimitUnit)
		plan.ThrottleLimitCount = req.ThrottleLimitCount
		plan.ThrottleLimitUnit = &s
	}
	if req.ExpiryTime != nil {
		plan.ExpiryTime = req.ExpiryTime
	}

	if err := s.getSubscriptionResourceService().SaveSubscriptionPlan(plan, correlationID, log); err != nil {
		if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: "Subscription plan already exists"})
			return
		}
		log.Error("Failed to save subscription plan", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create subscription plan"})
		return
	}
	c.JSON(http.StatusCreated, subscriptionPlanToResponse(plan))
}

// ListSubscriptionPlans implements ServerInterface.ListSubscriptionPlans (GET /subscription-plans)
func (s *APIServer) ListSubscriptionPlans(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)

	list, err := s.db.ListSubscriptionPlans("")
	if err != nil {
		log.Error("Failed to list subscription plans", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list subscription plans"})
		return
	}
	items := make([]api.SubscriptionPlanResponse, 0, len(list))
	for _, p := range list {
		items = append(items, subscriptionPlanToResponse(p))
	}
	count := len(items)
	c.JSON(http.StatusOK, api.SubscriptionPlanListResponse{SubscriptionPlans: &items, Count: &count})
}

// GetSubscriptionPlan implements ServerInterface.GetSubscriptionPlan (GET /subscription-plans/{planId})
func (s *APIServer) GetSubscriptionPlan(c *gin.Context, planId string) {
	log := middleware.GetLogger(c, s.logger)

	plan, err := s.db.GetSubscriptionPlanByID(planId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
			return
		}
		log.Error("Failed to get subscription plan", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription plan"})
		return
	}
	if plan == nil {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
		return
	}
	c.JSON(http.StatusOK, subscriptionPlanToResponse(plan))
}

// UpdateSubscriptionPlan implements ServerInterface.UpdateSubscriptionPlan (PUT /subscription-plans/{planId})
func (s *APIServer) UpdateSubscriptionPlan(c *gin.Context, planId string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	existing, err := s.db.GetSubscriptionPlanByID(planId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
			return
		}
		log.Error("Failed to get subscription plan for update", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription plan"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
		return
	}

	var req api.SubscriptionPlanUpdateRequest
	if err := s.bindRequestBody(c, &req); err != nil {
		log.Warn("Invalid subscription plan update body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
		return
	}

	var unitStr *string
	if req.ThrottleLimitUnit != nil {
		s := string(*req.ThrottleLimitUnit)
		unitStr = &s
	}
	if err := validateThrottleLimits(req.ThrottleLimitCount, unitStr); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	if req.PlanName != nil {
		trimmed := strings.TrimSpace(*req.PlanName)
		if trimmed == "" {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "planName cannot be empty"})
			return
		}
		existing.PlanName = trimmed
	}
	if req.BillingPlan != nil {
		existing.BillingPlan = req.BillingPlan
	}
	if req.StopOnQuotaReach != nil {
		existing.StopOnQuotaReach = *req.StopOnQuotaReach
	}
	if req.ThrottleLimitCount != nil && req.ThrottleLimitUnit != nil {
		s := string(*req.ThrottleLimitUnit)
		existing.ThrottleLimitCount = req.ThrottleLimitCount
		existing.ThrottleLimitUnit = &s
	}
	if req.ExpiryTime != nil {
		existing.ExpiryTime = req.ExpiryTime
	}
	if req.Status != nil {
		st := models.SubscriptionPlanStatus(*req.Status)
		switch st {
		case models.SubscriptionPlanStatusActive, models.SubscriptionPlanStatusInactive:
			existing.Status = st
		default:
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("invalid status: %s", *req.Status)})
			return
		}
	}

	if err := s.getSubscriptionResourceService().UpdateSubscriptionPlan(existing, correlationID, log); err != nil {
		log.Error("Failed to update subscription plan", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update subscription plan"})
		return
	}
	c.JSON(http.StatusOK, subscriptionPlanToResponse(existing))
}

// DeleteSubscriptionPlan implements ServerInterface.DeleteSubscriptionPlan (DELETE /subscription-plans/{planId})
func (s *APIServer) DeleteSubscriptionPlan(c *gin.Context, planId string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	if err := s.getSubscriptionResourceService().DeleteSubscriptionPlan(planId, correlationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
			return
		}
		log.Error("Failed to delete subscription plan", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to delete subscription plan"})
		return
	}
	c.Status(http.StatusNoContent)
}

// CreateSecret handles POST /secrets
func (s *APIServer) CreateSecret(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(c)

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	// Delegate to service which parses/validates/encrypt and persists
	secret, err := s.secretService.CreateSecret(secrets.SecretParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to encrypt Secret", slog.Any("error", err))
		if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	log.Info("Secret created successfully",
		slog.String("secret_handle", secret.Handle),
		slog.String("correlation_id", correlationID))

	// Return created secret
	c.JSON(http.StatusCreated, gin.H{
		"id":        secret.Handle,
		"createdAt": secret.CreatedAt,
		"updatedAt": secret.UpdatedAt,
	})
}

// ListSecrets implements ServerInterface.ListSecrets
// (GET /secrets)
func (s *APIServer) ListSecrets(c *gin.Context) {
	log := s.logger
	correlationID := middleware.GetCorrelationID(c)

	log.Debug("Retrieving secretsList", slog.String("correlation_id", correlationID))

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	secretsMeta, err := s.secretService.GetSecrets(correlationID)
	if err != nil {
		log.Error("Failed to retrieve secretsList",
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve secretsList",
		})
		return
	}

	// Convert []SecretMeta to API response type
	secretsList := make([]struct {
		CreatedAt   time.Time `json:"createdAt" yaml:"createdAt"`
		DisplayName string    `json:"displayName" yaml:"displayName"`
		Id          string    `json:"id" yaml:"id"`
		UpdatedAt   time.Time `json:"updatedAt" yaml:"updatedAt"`
	}, 0, len(secretsMeta))
	for _, meta := range secretsMeta {
		secretsList = append(secretsList, struct {
			CreatedAt   time.Time `json:"createdAt" yaml:"createdAt"`
			DisplayName string    `json:"displayName" yaml:"displayName"`
			Id          string    `json:"id" yaml:"id"`
			UpdatedAt   time.Time `json:"updatedAt" yaml:"updatedAt"`
		}{
			CreatedAt:   meta.CreatedAt,
			DisplayName: meta.DisplayName,
			Id:          meta.Handle,
			UpdatedAt:   meta.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, api.SecretListResponse{
		Status:     stringPtr("success"),
		TotalCount: intPtr(len(secretsList)),
		Secrets:    &secretsList,
	})
}

// GetSecret handles GET /secrets/{id}
func (s *APIServer) GetSecret(c *gin.Context, id string) {
	log := s.logger
	correlationID := middleware.GetCorrelationID(c)

	log.Debug("Retrieving secret",
		slog.String("secret_handle", id),
		slog.String("correlation_id", correlationID))

	// Validate secret ID format
	if id == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Missing required field: id",
		})
		return
	}
	if len(id) > 255 {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Secret ID too long (max 255 characters)",
		})
		return
	}

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	// Retrieve secret
	secret, err := s.secretService.Get(id, correlationID)
	if err != nil {
		// Check for not found error
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}

		// Generic error for decryption failures (security-first)
		log.Error("Failed to retrieve secret",
			slog.String("secret_handle", id),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to decrypt secret",
		})
		return
	}

	log.Debug("Secret retrieved successfully",
		slog.String("secret_handle", secret.Handle),
		slog.String("correlation_id", correlationID))

	// Reconstruct the SecretConfiguration from stored fields
	configuration := api.SecretConfiguration{
		ApiVersion: api.SecretConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.Secret,
		Metadata: api.Metadata{
			Name: secret.Handle,
		},
		Spec: api.SecretConfigData{
			DisplayName: secret.DisplayName,
			Description: secret.Description,
			Value:       secret.Value,
		},
	}

	// Return full secret detail
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"secret": gin.H{
			"id":            secret.Handle,
			"configuration": configuration,
			"metadata": gin.H{
				"createdAt": secret.CreatedAt.Format(time.RFC3339),
				"updatedAt": secret.UpdatedAt.Format(time.RFC3339),
			},
		},
	})
}

// UpdateSecret handles PUT /secrets/{id}
func (s *APIServer) UpdateSecret(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(c)

	// Validate secret ID format
	if id == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Missing required field: id",
		})
		return
	}
	if len(id) > 255 {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Secret ID too long (max 255 characters)",
		})
		return
	}

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	// Delegate to service which parses/validates/encrypt and persists
	secret, err := s.secretService.UpdateSecret(id, secrets.SecretParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to encrypt Secret", slog.Any("error", err))
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	log.Info("Secret updated successfully",
		slog.String("secret_handle", secret.Handle),
		slog.String("correlation_id", correlationID))

	// Return created secret
	c.JSON(http.StatusOK, gin.H{
		"id":        secret.Handle,
		"createdAt": secret.CreatedAt,
		"updatedAt": secret.UpdatedAt,
	})
}

// DeleteSecret handles DELETE /secrets/{id}
func (s *APIServer) DeleteSecret(c *gin.Context, id string) {
	log := s.logger
	correlationID := middleware.GetCorrelationID(c)

	log.Debug("Deleting secret",
		slog.String("secret_id", id),
		slog.String("correlation_id", correlationID))

	// Validate secret ID format
	if id == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Missing required field: id",
		})
		return
	}
	if len(id) > 255 {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Secret ID too long (max 255 characters)",
		})
		return
	}

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	// Delete secret
	if err := s.secretService.Delete(id, correlationID); err != nil {
		// Check for not found error
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}

		// Generic error for storage failures
		log.Error("Failed to delete secret",
			slog.String("secret_id", id),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete secret",
		})
		return
	}

	log.Info("Secret deleted successfully",
		slog.String("secret_id", id),
		slog.String("correlation_id", correlationID))

	// Return 200 OK on successful deletion
	c.Status(http.StatusOK)
}

func subscriptionPlanToResponse(plan *models.SubscriptionPlan) api.SubscriptionPlanResponse {
	resp := api.SubscriptionPlanResponse{
		Id:               ptr(plan.ID),
		PlanName:         ptr(plan.PlanName),
		GatewayId:        ptr(plan.GatewayID),
		StopOnQuotaReach: ptr(plan.StopOnQuotaReach),
		CreatedAt:        &plan.CreatedAt,
		UpdatedAt:        &plan.UpdatedAt,
	}
	if plan.BillingPlan != nil && *plan.BillingPlan != "" {
		resp.BillingPlan = plan.BillingPlan
	}
	if plan.ThrottleLimitCount != nil {
		resp.ThrottleLimitCount = plan.ThrottleLimitCount
	}
	if plan.ThrottleLimitUnit != nil && *plan.ThrottleLimitUnit != "" {
		resp.ThrottleLimitUnit = plan.ThrottleLimitUnit
	}
	if plan.ExpiryTime != nil {
		resp.ExpiryTime = plan.ExpiryTime
	}
	if plan.Status != "" {
		st := api.SubscriptionPlanResponseStatus(plan.Status)
		resp.Status = &st
	}
	return resp
}

// subscriptionToResponse builds a response without the subscription token.
// DB reads only have subscription_token_hash; token is never stored. Token is returned only at creation via subscriptionToResponseWithToken.
func subscriptionToResponse(sub *models.Subscription) api.SubscriptionResponse {
	resp := api.SubscriptionResponse{
		Id:                ptr(sub.ID),
		ApiId:             ptr(sub.APIID),
		GatewayId:         ptr(sub.GatewayID),
		CreatedAt:         &sub.CreatedAt,
		UpdatedAt:         &sub.UpdatedAt,
		SubscriptionToken: nil, // Explicitly omit; gateway does not store token, use Platform-API to retrieve
	}
	if sub.ApplicationID != nil {
		resp.ApplicationId = sub.ApplicationID
	}
	if sub.SubscriptionPlanID != nil {
		resp.SubscriptionPlanId = sub.SubscriptionPlanID
	}
	if sub.Status != "" {
		st := api.SubscriptionResponseStatus(sub.Status)
		resp.Status = &st
	}
	return resp
}

// subscriptionToResponseWithToken adds the token to the response (create flow only).
// Call only when sub has the raw token from creation, never from DB reads.
func subscriptionToResponseWithToken(sub *models.Subscription) api.SubscriptionResponse {
	resp := subscriptionToResponse(sub)
	if sub.SubscriptionToken != "" {
		resp.SubscriptionToken = ptr(sub.SubscriptionToken)
	}
	return resp
}

func ptr[T any](v T) *T { return &v }

// extractAuthenticatedUser extracts and validates the authenticated user from Gin context
// Returns the AuthenticatedUser object and handles error responses automatically
func (s *APIServer) extractAuthenticatedUser(c *gin.Context, operationName string, correlationID string) (*commonmodels.AuthContext, bool) {
	log := s.logger

	// Extract authentication context
	authCtxValue, exists := c.Get(constants.AuthContextKey)
	if !exists {
		log.Error("Authentication context not found",
			slog.String("operation", operationName),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{
			Status:  "error",
			Message: "Authentication context not available",
		})
		return nil, false
	}

	// Type assert to AuthContext
	user, ok := authCtxValue.(commonmodels.AuthContext)
	if !ok {
		log.Error("Invalid authentication context type",
			slog.String("operation", operationName),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Invalid authentication context",
		})
		return nil, false
	}

	log.Debug("Authenticated user extracted",
		slog.String("operation", operationName),
		slog.String("user_id", user.UserID),
		slog.Any("roles", user.Roles),
		slog.String("correlation_id", correlationID))

	return &user, true
}

// bindRequestBody binds the request body based on Content-Type header.
// Supports both JSON and YAML content types.
// Handles Content-Type headers case-insensitively and strips parameters (e.g., charset).
func (s *APIServer) bindRequestBody(c *gin.Context, request interface{}) error {
	contentType := c.GetHeader("Content-Type")

	// Normalize the Content-Type: trim whitespace, split off parameters, and convert to lowercase
	contentType = strings.TrimSpace(contentType)
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = contentType[:idx]
	}
	contentType = strings.TrimSpace(contentType)
	contentType = strings.ToLower(contentType)

	// Check for YAML content types (case-insensitive, normalized)
	if contentType == "application/yaml" || contentType == "text/yaml" {
		return c.ShouldBindYAML(request)
	}

	// Default to JSON for application/json or when no content type is specified
	return c.ShouldBindJSON(request)
}

// getLLMProviderTemplate extracts the template name from sourceConfig and retrieves the template.
// Returns the template configuration if found, nil otherwise.
func (s *APIServer) getLLMProviderTemplate(sourceConfig any) (*api.LLMProviderTemplate, error) {
	if sourceConfig == nil {
		return nil, fmt.Errorf("sourceConfig is nil")
	}

	// Try to extract the template name from sourceConfig
	// and get the template from the store
	templateName, err := utils.GetValueFromSourceConfig(sourceConfig, "spec.template")
	if err != nil {
		return nil, fmt.Errorf("failed to extract template name: %w", err)
	}
	templateNameStr, ok := templateName.(string)
	if !ok {
		return nil, fmt.Errorf("template name is not a string: %v", templateName)
	}
	if templateNameStr == "" {
		return nil, fmt.Errorf("template name is empty")
	}

	storedTemplate, err := s.store.GetTemplateByHandle(templateNameStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get template '%s' from store: %w", templateNameStr, err)
	}

	return &storedTemplate.Configuration, nil
}

// populatePropsForSystemPolicies populates the props for system policies
// based on the source configuration
// Note: Template handle is now passed via route metadata instead of props
func (s *APIServer) populatePropsForSystemPolicies(srcConfig any, props map[string]any) {
	if srcConfig == nil {
		return
	}
	// Template handle is now extracted and added to route metadata in translator.go
	// No need to pass template via props anymore
}

func toGenericMap(value interface{}) (map[string]interface{}, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

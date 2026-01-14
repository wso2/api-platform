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
	"context"
	"encoding/json"
	"fmt"

	"github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	gatewayconstants "github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"

	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
	"go.uber.org/zap"
)

// APIServer implements the generated ServerInterface
type APIServer struct {
	store                *storage.ConfigStore
	db                   storage.Storage
	snapshotManager      *xds.SnapshotManager
	policyManager        *policyxds.PolicyManager
	policyDefinitions    map[string]api.PolicyDefinition // key name|version
	policyDefMu          sync.RWMutex
	parser               *config.Parser
	validator            config.Validator
	logger               *zap.Logger
	deploymentService    *utils.APIDeploymentService
	mcpDeploymentService *utils.MCPDeploymentService
	llmDeploymentService *utils.LLMDeploymentService
	apiKeyService        *utils.APIKeyService
	apiKeyXDSManager     *apikeyxds.APIKeyStateManager
	controlPlaneClient   controlplane.ControlPlaneClient
	routerConfig         *config.RouterConfig
	httpClient           *http.Client
	systemConfig         *config.Config
}

// NewAPIServer creates a new API server with dependencies
func NewAPIServer(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
	logger *zap.Logger,
	controlPlaneClient controlplane.ControlPlaneClient,
	policyDefinitions map[string]api.PolicyDefinition,
	templateDefinitions map[string]*api.LLMProviderTemplate,
	validator config.Validator,
	apiKeyXDSManager *apikeyxds.APIKeyStateManager,
	systemConfig *config.Config,
) *APIServer {
	deploymentService := utils.NewAPIDeploymentService(store, db, snapshotManager, validator, &systemConfig.GatewayController.Router)
	server := &APIServer{
		store:                store,
		db:                   db,
		snapshotManager:      snapshotManager,
		policyManager:        policyManager,
		policyDefinitions:    policyDefinitions,
		parser:               config.NewParser(),
		validator:            validator,
		logger:               logger,
		deploymentService:    deploymentService,
		mcpDeploymentService: utils.NewMCPDeploymentService(store, db, snapshotManager),
		llmDeploymentService: utils.NewLLMDeploymentService(store, db, snapshotManager, templateDefinitions,
			deploymentService, &systemConfig.GatewayController.Router),
		apiKeyService:      utils.NewAPIKeyService(store, db, apiKeyXDSManager),
		apiKeyXDSManager:   apiKeyXDSManager,
		controlPlaneClient: controlPlaneClient,
		routerConfig:       &systemConfig.GatewayController.Router,
		httpClient:         &http.Client{Timeout: 10 * time.Second},
		systemConfig:       systemConfig,
	}

	// Register status update callback
	snapshotManager.SetStatusCallback(server.handleStatusUpdate)

	return server
}

// handleStatusUpdate is called by SnapshotManager after xDS deployment
func (s *APIServer) handleStatusUpdate(configID string, success bool, version int64, correlationID string) {
	// Create a logger with correlation ID if provided
	log := s.logger
	if correlationID != "" {
		log = s.logger.With(zap.String("correlation_id", correlationID))
	}

	cfg, err := s.store.Get(configID)
	if err != nil {
		log.Warn("Config not found for status update", zap.String("id", configID))
		return
	}

	now := time.Now()
	if success {
		cfg.Status = models.StatusDeployed
		cfg.DeployedAt = &now
		cfg.DeployedVersion = version
		log.Info("Configuration deployed successfully",
			zap.String("id", configID),
			zap.String("displayName", cfg.GetDisplayName()),
			zap.Int64("version", version))
	} else {
		cfg.Status = models.StatusFailed
		cfg.DeployedAt = nil
		cfg.DeployedVersion = 0
		log.Error("Configuration deployment failed",
			zap.String("id", configID),
			zap.String("displayName", cfg.GetDisplayName()),
			zap.String("kind", cfg.Kind))
	}

	cfg.UpdatedAt = now

	// Update database (only if persistent mode)
	if s.db != nil {
		if err := s.db.UpdateConfig(cfg); err != nil {
			log.Error("Failed to update config status in database", zap.Error(err), zap.String("id", configID))
		}
	}

	// Update in-memory store
	if err := s.store.Update(cfg); err != nil {
		log.Error("Failed to update config status in memory", zap.Error(err), zap.String("id", configID))
	}
}

// HealthCheck implements ServerInterface.HealthCheck
// (GET /health)
func (s *APIServer) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// CreateAPI implements ServerInterface.CreateAPI
// (POST /apis)
func (s *APIServer) CreateAPI(c *gin.Context) {
	startTime := time.Now()
	operation := "create"

	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", zap.Error(err))
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", "rest_api").Inc()
		metrics.ValidationErrorsTotal.WithLabelValues(operation, "read_body_failed").Inc()
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(c)

	// Deploy API configuration using the utility service
	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		APIID:         "", // Empty to generate new UUID
		CorrelationID: correlationID,
		Logger:        log,
	})

	if err != nil {
		log.Error("Failed to deploy API configuration", zap.Error(err))
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", "rest_api").Inc()
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

	// Record successful operation metrics
	metrics.APIOperationsTotal.WithLabelValues(operation, "success", "rest_api").Inc()
	metrics.APIOperationDurationSeconds.WithLabelValues(operation, "rest_api").Observe(time.Since(startTime).Seconds())
	metrics.APIsTotal.WithLabelValues("rest_api", "active").Inc()

	// Set up a callback to notify platform API after successful deployment
	// This is specific to direct API creation via gateway endpoint
	if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() {
		go s.waitForDeploymentAndNotify(result.StoredConfig.ID, correlationID, log)
	}

	// Return success response (id is the handle)
	c.JSON(http.StatusCreated, api.APICreateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("API configuration created successfully"),
		Id:        stringPtr(result.StoredConfig.GetHandle()),
		CreatedAt: timePtr(result.StoredConfig.CreatedAt),
	})

	// Build and add policy config derived from API configuration if policies are present
	if s.policyManager != nil {
		storedPolicy := s.buildStoredPolicyFromAPI(result.StoredConfig)
		if storedPolicy != nil {
			if err := s.policyManager.AddPolicy(storedPolicy); err != nil {
				log.Error("Failed to add derived policy configuration", zap.Error(err))
			} else {
				log.Info("Derived policy configuration added",
					zap.String("policy_id", storedPolicy.ID),
					zap.Int("route_count", len(storedPolicy.Configuration.Routes)))
			}
		} else if result.IsUpdate {
			// API was updated and no longer has policies, remove the existing policy configuration
			policyID := result.StoredConfig.ID + "-policies"
			if err := s.policyManager.RemovePolicy(policyID); err != nil {
				// Log at debug level since policy may not exist if API never had policies
				log.Debug("No policy configuration to remove", zap.String("policy_id", policyID))
			} else {
				log.Info("Derived policy configuration removed (API no longer has policies)",
					zap.String("policy_id", policyID))
			}
		}
	}
}

// ListAPIs implements ServerInterface.ListAPIs
// (GET /apis)
func (s *APIServer) ListAPIs(c *gin.Context, params api.ListAPIsParams) {
	if (params.DisplayName != nil && *params.DisplayName != "") || (params.Version != nil && *params.Version != "") || (params.Context != nil && *params.Context != "") || (params.Status != nil && *params.Status != "") {
		s.SearchDeployments(c, string(api.RestApi))
		return
	}
	configs := s.store.GetAllByKind(string(api.RestApi))

	items := make([]api.APIListItem, 0, len(configs))
	for _, cfg := range configs {
		status := string(cfg.Status)
		items = append(items, api.APIListItem{
			Id:          stringPtr(cfg.GetHandle()),
			DisplayName: stringPtr(cfg.GetDisplayName()),
			Version:     stringPtr(cfg.GetVersion()),
			Context:     stringPtr(cfg.GetContext()),
			Status:      (*api.APIListItemStatus)(&status),
			CreatedAt:   timePtr(cfg.CreatedAt),
			UpdatedAt:   timePtr(cfg.UpdatedAt),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"count":  len(items),
		"apis":   items,
	})
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

	// Filter based on kind to return appropriate response format
	if kind == string(api.Mcp) {
		// Return MCP proxy format
		mcpItems := make([]api.MCPProxyListItem, 0)
		for _, cfg := range configs {
			if v, ok := filters["displayName"]; ok && cfg.GetDisplayName() != v {
				continue
			}
			if v, ok := filters["version"]; ok && cfg.GetVersion() != v {
				continue
			}
			if v, ok := filters["context"]; ok && cfg.GetContext() != v {
				continue
			}
			if v, ok := filters["status"]; ok && string(cfg.Status) != v {
				continue
			}

			status := api.MCPProxyListItemStatus(cfg.Status)
			// Convert SourceConfiguration to MCPProxyConfiguration to get spec fields
			var mcp api.MCPProxyConfiguration
			j, _ := json.Marshal(cfg.SourceConfiguration)
			err := json.Unmarshal(j, &mcp)
			if err != nil {
				s.logger.Error("Failed to unmarshal stored MCP configuration",
					zap.String("id", cfg.ID),
					zap.String("displayName", cfg.GetDisplayName()))
				continue
			}

			li := api.MCPProxyListItem{
				Id:          stringPtr(cfg.GetHandle()),
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
			"status":      "success",
			"count":       len(mcpItems),
			"mcp_proxies": mcpItems,
		})
	} else {
		// Return API format
		apiItems := make([]api.APIListItem, 0)
		for _, cfg := range configs {
			if v, ok := filters["displayName"]; ok && cfg.GetDisplayName() != v {
				continue
			}
			if v, ok := filters["version"]; ok && cfg.GetVersion() != v {
				continue
			}
			if v, ok := filters["context"]; ok && cfg.GetContext() != v {
				continue
			}
			if v, ok := filters["status"]; ok && string(cfg.Status) != v {
				continue
			}

			status := string(cfg.Status)
			apiItems = append(apiItems, api.APIListItem{
				Id:          stringPtr(cfg.GetHandle()),
				DisplayName: stringPtr(cfg.GetDisplayName()),
				Version:     stringPtr(cfg.GetVersion()),
				Context:     stringPtr(cfg.GetContext()),
				Status:      (*api.APIListItemStatus)(&status),
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

	cfg, err := s.store.GetByNameVersion(name, version)
	if err != nil {
		log.Warn("API configuration not found",
			zap.String("name", name),
			zap.String("version", version))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("API configuration with name '%s' and version '%s' not found", name, version),
		})
		return
	}

	apiDetail := gin.H{
		"id":            cfg.GetHandle(),
		"configuration": cfg.Configuration,
		"metadata": gin.H{
			"status":     string(cfg.Status),
			"created_at": cfg.CreatedAt.Format(time.RFC3339),
			"updated_at": cfg.UpdatedAt.Format(time.RFC3339),
		},
	}

	if cfg.DeployedAt != nil {
		apiDetail["metadata"].(gin.H)["deployed_at"] = cfg.DeployedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"api":    apiDetail,
	})
}

// GetAPIById implements ServerInterface.GetAPIById
// (GET /apis/{id})
func (s *APIServer) GetAPIById(c *gin.Context, id string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)
	handle := id

	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, api.ErrorResponse{
			Status:  "error",
			Message: "Database storage not available",
		})
		return
	}

	cfg, err := s.db.GetConfigByHandle(handle)
	if err != nil {
		log.Warn("API configuration not found",
			zap.String("handle", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("API configuration with handle '%s' not found", handle),
		})
		return
	}

	if cfg.Kind != string(api.RestApi) && cfg.Kind != string(api.WebSubApi) {
		log.Warn("Configuration kind mismatch",
			zap.String("expected", "RestApi or async/websub"),
			zap.String("actual", cfg.Kind),
			zap.String("handle", handle))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Configuration with handle '%s' is not an API", handle),
		})
		return
	}

	apiDetail := gin.H{
		"id":            cfg.GetHandle(),
		"configuration": cfg.Configuration,
		"metadata": gin.H{
			"status":     string(cfg.Status),
			"created_at": cfg.CreatedAt.Format(time.RFC3339),
			"updated_at": cfg.UpdatedAt.Format(time.RFC3339),
		},
	}

	if cfg.DeployedAt != nil {
		apiDetail["metadata"].(gin.H)["deployed_at"] = cfg.DeployedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"api":    apiDetail,
	})
}

// UpdateAPI implements ServerInterface.UpdateAPI
// (PUT /apis/{handle})
func (s *APIServer) UpdateAPI(c *gin.Context, id string) {
	startTime := time.Now()
	operation := "update"

	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)
	handle := id

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", zap.Error(err))
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", "rest_api").Inc()
		metrics.ValidationErrorsTotal.WithLabelValues(operation, "read_body_failed").Inc()
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Parse configuration
	contentType := c.GetHeader("Content-Type")
	var apiConfig api.APIConfiguration
	err = s.parser.Parse(body, contentType, &apiConfig)
	if err != nil {
		log.Error("Failed to parse configuration", zap.Error(err))
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", "rest_api").Inc()
		metrics.ValidationErrorsTotal.WithLabelValues(operation, "parse_failed").Inc()
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to parse configuration",
		})
		return
	}

	// Validate that the handle in the YAML matches the path parameter
	if apiConfig.Metadata.Name != "" {
		if apiConfig.Metadata.Name != handle {
			log.Warn("Handle mismatch between path and YAML metadata",
				zap.String("path_handle", handle),
				zap.String("yaml_handle", apiConfig.Metadata.Name))
			metrics.APIOperationsTotal.WithLabelValues(operation, "error", "rest_api").Inc()
			metrics.ValidationErrorsTotal.WithLabelValues(operation, "handle_mismatch").Inc()
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("Handle mismatch: path has '%s' but YAML metadata.name has '%s'", handle, apiConfig.Metadata.Name),
			})
			return
		}
	}

	if apiConfig.Kind == api.WebSubApi {
		webhookData, err := apiConfig.Spec.AsWebhookAPIData()
		if err != nil {
			log.Error("Failed to parse configuration", zap.Error(err))
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to parse configuration",
			})
			return
		}
		// Ensure an upstream main exists for async/websub configs so downstream
		// logic can safely rely on the field being present. Create an empty
		// upstream if it is missing and save it back into the union spec.
		if webhookData.Upstream.Main == nil {
			url := fmt.Sprintf("%s:%d", s.routerConfig.EventGateway.WebSubHubURL, s.routerConfig.EventGateway.WebSubHubPort)
			webhookData.Upstream.Main = &api.Upstream{
				Url: &url,
			}
			if err := apiConfig.Spec.FromWebhookAPIData(webhookData); err != nil {
				log.Error("Failed to parse configuration", zap.Error(err))
				c.JSON(http.StatusBadRequest, api.ErrorResponse{
					Status:  "error",
					Message: "Error while processing configuration",
				})
				return
			}
		}
	}

	// Validate configuration
	validationErrors := s.validator.Validate(&apiConfig)
	if len(validationErrors) > 0 {
		log.Warn("Configuration validation failed",
			zap.String("handle", handle),
			zap.Int("num_errors", len(validationErrors)))

		metrics.APIOperationsTotal.WithLabelValues(operation, "error", "rest_api").Inc()
		metrics.ValidationErrorsTotal.WithLabelValues(operation, "validation_failed").Add(float64(len(validationErrors)))

		errors := make([]api.ValidationError, len(validationErrors))
		for i, e := range validationErrors {
			errors[i] = api.ValidationError{
				Field:   stringPtr(e.Field),
				Message: stringPtr(e.Message),
			}
		}

		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Configuration validation failed",
			Errors:  &errors,
		})
		return
	}

	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, api.ErrorResponse{
			Status:  "error",
			Message: "Database storage not available",
		})
		return
	}

	// Check if config exists
	existing, err := s.db.GetConfigByHandle(handle)
	if err != nil {
		log.Warn("API configuration not found",
			zap.String("handle", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("API configuration with handle '%s' not found", handle),
		})
		return
	}

	// Update stored configuration
	now := time.Now()
	existing.Configuration = apiConfig
	existing.Status = models.StatusPending
	existing.UpdatedAt = now
	existing.DeployedAt = nil
	existing.DeployedVersion = 0

	if apiConfig.Kind == api.WebSubApi {
		topicsToRegister, topicsToUnregister := s.deploymentService.GetTopicsForUpdate(*existing)
		// TODO: Pre configure the dynamic forward proxy rules for event gw
		// This was communication bridge will be created on the gw startup
		// Can perform internal communication with websub hub without relying on the dynamic rules
		// Execute topic operations with wait group and errors tracking
		var wg2 sync.WaitGroup
		var regErrs int32
		var deregErrs int32

		if len(topicsToRegister) > 0 {
			wg2.Add(1)
			go func(list []string) {
				defer wg2.Done()
				log.Info("Starting topic registration", zap.Int("total_topics", len(list)), zap.String("api_id", existing.ID))
				//fmt.Println("Topics Registering Started")
				var childWg sync.WaitGroup
				for _, topic := range list {
					childWg.Add(1)
					go func(topic string) {
						defer childWg.Done()
						ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.routerConfig.EventGateway.TimeoutSeconds)*time.Second)
						defer cancel()
						if err := s.deploymentService.RegisterTopicWithHub(ctx, s.httpClient, topic, s.routerConfig.EventGateway.RouterHost, s.routerConfig.EventGateway.WebSubHubListenerPort, log); err != nil {
							log.Error("Failed to register topic with WebSubHub",
								zap.Error(err),
								zap.String("topic", topic),
								zap.String("api_id", existing.ID))
							atomic.AddInt32(&regErrs, 1)
						} else {
							log.Info("Successfully registered topic with WebSubHub",
								zap.String("topic", topic),
								zap.String("api_id", existing.ID))
						}
					}(topic)
				}
				childWg.Wait()
			}(topicsToRegister)
		}

		if len(topicsToUnregister) > 0 {
			wg2.Add(1)
			go func(list []string) {
				defer wg2.Done()
				log.Info("Starting topic deregistration", zap.Int("total_topics", len(list)), zap.String("api_id", existing.ID))
				var childWg sync.WaitGroup
				for _, topic := range list {
					childWg.Add(1)
					go func(topic string) {
						defer childWg.Done()
						ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.routerConfig.EventGateway.TimeoutSeconds)*time.Second)
						defer cancel()
						if err := s.deploymentService.UnregisterTopicWithHub(ctx, s.httpClient, topic, s.routerConfig.EventGateway.RouterHost, s.routerConfig.EventGateway.WebSubHubListenerPort, log); err != nil {
							log.Error("Failed to deregister topic from WebSubHub",
								zap.Error(err),
								zap.String("topic", topic),
								zap.String("api_id", existing.ID))
							atomic.AddInt32(&deregErrs, 1)
						} else {
							log.Info("Successfully deregistered topic from WebSubHub",
								zap.String("topic", topic),
								zap.String("api_id", existing.ID))
						}
					}(topic)
				}
				childWg.Wait()
			}(topicsToUnregister)
		}
		wg2.Wait()

		log.Info("Topic lifecycle operations completed",
			zap.String("api_id", existing.ID),
			zap.Int("registered", len(topicsToRegister)),
			zap.Int("deregistered", len(topicsToUnregister)),
			zap.Int("register_errors", int(regErrs)),
			zap.Int("deregister_errors", int(deregErrs)))

		// Check if topic operations failed and return error
		if regErrs > 0 || deregErrs > 0 {
			log.Error("Failed to register & deregister topics", zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Topic lifecycle operations failed",
			})
			return
		}
	}

	// Atomic dual-write: database + in-memory
	// Update database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.UpdateConfig(existing); err != nil {
			log.Error("Failed to update config in database", zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to persist configuration update",
			})
			return
		}
	}

	if err := s.store.Update(existing); err != nil {
		// Log conflict errors at info level, other errors at error level
		if storage.IsConflictError(err) {
			log.Info("API configuration handle already exists",
				zap.String("id", existing.ID),
				zap.String("handle", handle))
			c.JSON(http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else {
			log.Error("Failed to update config in memory store", zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to update configuration in memory store",
			})
		}
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(c)

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.routerConfig.EventGateway.TimeoutSeconds)*time.Second)
		defer cancel()

		if err := s.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
			log.Error("Failed to update xDS snapshot", zap.Error(err))
		}
	}()

	log.Info("API configuration updated",
		zap.String("id", existing.ID),
		zap.String("handle", handle))

	// Record successful operation metrics
	metrics.APIOperationsTotal.WithLabelValues(operation, "success", "rest_api").Inc()
	metrics.APIOperationDurationSeconds.WithLabelValues(operation, "rest_api").Observe(time.Since(startTime).Seconds())

	// Return success response (id is the handle)
	c.JSON(http.StatusOK, api.APIUpdateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("API configuration updated successfully"),
		Id:        stringPtr(existing.GetHandle()),
		UpdatedAt: timePtr(existing.UpdatedAt),
	})

	// Rebuild and update derived policy configuration
	if s.policyManager != nil {
		storedPolicy := s.buildStoredPolicyFromAPI(existing)
		if storedPolicy != nil {
			if err := s.policyManager.AddPolicy(storedPolicy); err != nil {
				log.Error("Failed to update derived policy configuration", zap.Error(err))
			} else {
				log.Info("Derived policy configuration updated",
					zap.String("policy_id", storedPolicy.ID),
					zap.Int("route_count", len(storedPolicy.Configuration.Routes)))
			}
		} else {
			// API no longer has policies, remove the existing policy configuration
			policyID := existing.ID + "-policies"
			if err := s.policyManager.RemovePolicy(policyID); err != nil {
				// Log at debug level since policy may not exist if API never had policies
				log.Debug("No policy configuration to remove", zap.String("policy_id", policyID))
			} else {
				log.Info("Derived policy configuration removed (API no longer has policies)",
					zap.String("policy_id", policyID))
			}
		}
	}
}

// DeleteAPI implements ServerInterface.DeleteAPI
// (DELETE /apis/{handle})
func (s *APIServer) DeleteAPI(c *gin.Context, id string) {
	startTime := time.Now()
	operation := "delete"

	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)

	handle := id

	if s.db == nil {
		log.Error("Database storage not available")
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", "rest_api").Inc()
		c.JSON(http.StatusServiceUnavailable, api.ErrorResponse{
			Status:  "error",
			Message: "Database storage not available",
		})
		return
	}

	// Check if config exists
	cfg, err := s.db.GetConfigByHandle(handle)
	if err != nil {
		log.Warn("API configuration not found",
			zap.String("handle", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("API configuration with handle '%s' not found", handle),
		})
		return
	}

	// Delete from database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.DeleteConfig(cfg.ID); err != nil {
			log.Error("Failed to delete config from database", zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to delete configuration",
			})
			return
		}

		// Delete associated API keys from database
		err := s.db.RemoveAPIKeysAPI(cfg.ID)
		if err != nil {
			log.Warn("Failed to remove API keys from database",
				zap.String("handle", handle),
				zap.Error(err))
		}
	}

	// Remove API keys from ConfigStore
	if err := s.store.RemoveAPIKeysByAPI(cfg.ID); err != nil {
		log.Warn("Failed to remove API keys from ConfigStore",
			zap.String("handle", handle),
			zap.Error(err))
	}

	// Remove API keys from policy engine via xDS
	if s.apiKeyXDSManager != nil {
		// Extract API name and version from the config
		apiConfig, err := cfg.Configuration.Spec.AsAPIConfigData()
		if err == nil {
			apiId := cfg.ID
			apiName := apiConfig.DisplayName
			apiVersion := apiConfig.Version
			correlationID := middleware.GetCorrelationID(c)

			if err := s.apiKeyXDSManager.RemoveAPIKeysByAPI(apiId, apiName, apiVersion, correlationID); err != nil {
				log.Warn("Failed to remove API keys from policy engine",
					zap.String("api_id", apiId),
					zap.String("handle", handle),
					zap.String("api_name", apiName),
					zap.String("api_version", apiVersion),
					zap.String("correlation_id", correlationID),
					zap.Error(err))
			} else {
				log.Info("Successfully removed API keys from policy engine",
					zap.String("api_id", apiId),
					zap.String("handle", handle),
					zap.String("api_name", apiName),
					zap.String("api_version", apiVersion),
					zap.String("correlation_id", correlationID))
			}
		} else {
			log.Warn("Failed to extract API config data for API key removal",
				zap.String("handle", handle),
				zap.Error(err))
		}
	}

	if cfg.Configuration.Kind == api.WebSubApi {
		topicsToUnregister := s.deploymentService.GetTopicsForDelete(*cfg)

		var deregErrs int32
		var wg sync.WaitGroup

		if len(topicsToUnregister) > 0 {
			wg.Add(1)
			go func(list []string) {
				defer wg.Done()
				log.Info("Starting topic deregistration", zap.Int("total_topics", len(list)), zap.String("api_id", cfg.ID))
				var childWg sync.WaitGroup
				for _, topic := range list {
					childWg.Add(1)
					go func(topic string) {
						defer childWg.Done()
						ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.routerConfig.EventGateway.TimeoutSeconds)*time.Second)
						defer cancel()
						if err := s.deploymentService.UnregisterTopicWithHub(ctx, s.httpClient, topic, s.routerConfig.EventGateway.RouterHost, s.routerConfig.EventGateway.WebSubHubListenerPort, log); err != nil {
							log.Error("Failed to deregister topic from WebSubHub",
								zap.Error(err),
								zap.String("topic", topic),
								zap.String("api_id", cfg.ID))
							atomic.AddInt32(&deregErrs, 1)
						} else {
							log.Info("Successfully deregistered topic from WebSubHub",
								zap.String("topic", topic),
								zap.String("api_id", cfg.ID))
						}
					}(topic)
				}
				childWg.Wait()
			}(topicsToUnregister)
		}

		wg.Wait()

		log.Info("Topic lifecycle operations completed",
			zap.String("api_id", cfg.ID),
			zap.Int("deregistered", len(topicsToUnregister)),
			zap.Int("deregister_errors", int(deregErrs)))

		// Check if topic operations failed and return error
		if deregErrs > 0 {
			log.Error("Failed to register & deregister topics", zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Topic lifecycle operations failed",
			})
			return
		}
	}

	// Delete from in-memory store
	if err := s.store.Delete(cfg.ID); err != nil {
		log.Error("Failed to delete config from memory store", zap.Error(err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete configuration",
		})
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(c)

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
			log.Error("Failed to update xDS snapshot", zap.Error(err))
		}
	}()

	log.Info("API configuration deleted",
		zap.String("id", cfg.ID),
		zap.String("handle", handle))

	// Record successful operation metrics
	metrics.APIOperationsTotal.WithLabelValues(operation, "success", "rest_api").Inc()
	metrics.APIOperationDurationSeconds.WithLabelValues(operation, "rest_api").Observe(time.Since(startTime).Seconds())
	metrics.APIsTotal.WithLabelValues("rest_api", "active").Dec()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "API configuration deleted successfully",
		"id":      handle,
	})

	// Remove derived policy configuration
	if s.policyManager != nil {
		policyID := cfg.ID + "-policies"
		if err := s.policyManager.RemovePolicy(policyID); err != nil {
			log.Warn("Failed to remove derived policy configuration", zap.Error(err), zap.String("policy_id", policyID))
		} else {
			log.Info("Derived policy configuration removed", zap.String("policy_id", policyID))
		}
	}
}

// CreateLLMProviderTemplate implements ServerInterface.CreateLLMProviderTemplate
// (POST /llm-provider-templates)
func (s *APIServer) CreateLLMProviderTemplate(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	storedTemplate, err := s.llmDeploymentService.CreateLLMProviderTemplate(utils.LLMTemplateParams{
		Spec:        body,
		ContentType: c.GetHeader("Content-Type"),
		Logger:      log,
	})

	if err != nil {
		log.Error("Failed to parse template configuration", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to parse template configuration: %v", err),
		})
		return
	}

	log.Info("LLM provider template created successfully",
		zap.String("uuid", storedTemplate.ID),
		zap.String("handle", storedTemplate.GetHandle()))

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
		log.Warn("LLM provider template not found", zap.String("handle", id))
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
			"created_at": template.CreatedAt,
			"updated_at": template.UpdatedAt,
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

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	updated, err := s.llmDeploymentService.UpdateLLMProviderTemplate(id, utils.LLMTemplateParams{
		Spec:        body,
		ContentType: c.GetHeader("Content-Type"),
		Logger:      log,
	})
	if err != nil {
		log.Error("Failed to parse template configuration", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to parse template configuration: %v", err),
		})
		return
	}

	log.Info("LLM provider template updated successfully",
		zap.String("uuid", updated.ID),
		zap.String("handle", updated.GetHandle()))

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

	deleted, err := s.llmDeploymentService.DeleteLLMProviderTemplate(id)
	if err != nil {
		log.Warn("LLM provider template not found for deletion", zap.String("handle", id))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Template with id '%s' not found", id),
		})
		return
	}

	log.Info("LLM provider template deleted successfully",
		zap.String("uuid", deleted.ID),
		zap.String("handle", deleted.GetHandle()))

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
		status := api.LLMProviderListItemStatus(cfg.Status)

		// Convert SourceConfiguration to LLMProviderConfiguration
		var prov api.LLMProviderConfiguration
		j, _ := json.Marshal(cfg.SourceConfiguration)
		if err := json.Unmarshal(j, &prov); err != nil {
			log.Error("Failed to unmarshal stored LLM provider configuration",
				zap.String("uuid", cfg.ID), zap.Error(err))
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
		log.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(c)

	// Delegate to service which parses/validates/transforms and persists
	stored, err := s.llmDeploymentService.CreateLLMProvider(utils.LLMDeploymentParams{
		Data:        body,
		ContentType: c.GetHeader("Content-Type"),
		Logger:      log,
	})
	if err != nil {
		log.Error("Failed to create LLM provider", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	// Set up a callback to notify platform API after successful deployment
	// This is specific to direct API creation via gateway endpoint
	if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() {
		go s.waitForDeploymentAndNotify(stored.ID, correlationID, log)
	}

	log.Info("LLM provider created successfully",
		zap.String("uuid", stored.ID),
		zap.String("handle", stored.GetHandle()))

	c.JSON(http.StatusCreated, api.LLMProviderCreateResponse{
		Status:  stringPtr("success"),
		Message: stringPtr("LLM provider created successfully"),
		Id:      stringPtr(stored.GetHandle()), CreatedAt: timePtr(stored.CreatedAt)})

	// Build and add policy config derived from API configuration if policies are present
	if s.policyManager != nil {
		storedPolicy := s.buildStoredPolicyFromAPI(stored)
		if storedPolicy != nil {
			if err := s.policyManager.AddPolicy(storedPolicy); err != nil {
				log.Error("Failed to add derived policy configuration", zap.Error(err))
			} else {
				log.Info("Derived policy configuration added",
					zap.String("policy_id", storedPolicy.ID),
					zap.Int("route_count", len(storedPolicy.Configuration.Routes)))
			}
		}
	}
}

// GetLLMProviderById implements ServerInterface.GetLLMProviderById
// (GET /llm-providers/{id})
func (s *APIServer) GetLLMProviderById(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)

	cfg := s.store.GetByKindAndHandle(string(api.LlmProvider), id)
	if cfg == nil {
		log.Warn("LLM provider configuration not found",
			zap.String("handle", id))
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
			"status":     string(cfg.Status),
			"created_at": cfg.CreatedAt.Format(time.RFC3339),
			"updated_at": cfg.UpdatedAt.Format(time.RFC3339),
		},
	}

	if cfg.DeployedAt != nil {
		providerDetail["metadata"].(gin.H)["deployed_at"] = cfg.DeployedAt.Format(time.RFC3339)
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
		log.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID
	correlationID := middleware.GetCorrelationID(c)

	// Delegate to service update wrapper
	updated, err := s.llmDeploymentService.UpdateLLMProvider(id, utils.LLMDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to update LLM provider configuration", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, api.LLMProviderUpdateResponse{
		Id:        stringPtr(updated.GetHandle()),
		Message:   stringPtr("LLM provider updated successfully"),
		Status:    stringPtr("success"),
		UpdatedAt: timePtr(updated.UpdatedAt),
	})

	// Rebuild and update derived policy configuration
	if s.policyManager != nil {
		storedPolicy := s.buildStoredPolicyFromAPI(updated)
		if storedPolicy != nil {
			if err := s.policyManager.AddPolicy(storedPolicy); err != nil {
				log.Error("Failed to update derived policy configuration", zap.Error(err))
			} else {
				log.Info("Derived policy configuration updated",
					zap.String("policy_id", storedPolicy.ID),
					zap.Int("route_count", len(storedPolicy.Configuration.Routes)))
			}
		} else {
			// LLM provider no longer has policies, remove the existing policy configuration
			policyID := updated.ID + "-policies"
			if err := s.policyManager.RemovePolicy(policyID); err != nil {
				// Log at debug level since policy may not exist if LLM provider never had policies
				log.Debug("No policy configuration to remove", zap.String("policy_id", policyID))
			} else {
				log.Info("Derived policy configuration removed (LLM provider no longer has policies)",
					zap.String("policy_id", policyID))
			}
		}
	}
}

// DeleteLLMProvider implements ServerInterface.DeleteLLMProvider
// (DELETE /llm-providers/{id})
func (s *APIServer) DeleteLLMProvider(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)

	cfg, err := s.llmDeploymentService.DeleteLLMProvider(id, correlationID, log)
	if err != nil {
		log.Warn("Failed to delete LLM provider configuration", zap.String("handle", id))
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
		"id":      cfg.GetHandle(),
	})

	// Remove derived policy configuration
	if s.policyManager != nil {
		policyID := cfg.ID + "-policies"
		if err := s.policyManager.RemovePolicy(policyID); err != nil {
			log.Warn("Failed to remove derived policy configuration", zap.Error(err), zap.String("policy_id", policyID))
		} else {
			log.Info("Derived policy configuration removed", zap.String("policy_id", policyID))
		}
	}
}

// ListLLMProxies implements ServerInterface.ListLLMProxies
// (GET /llm-proxies)
func (s *APIServer) ListLLMProxies(c *gin.Context, params api.ListLLMProxiesParams) {
	log := middleware.GetLogger(c, s.logger)
	configs := s.llmDeploymentService.ListLLMProxies(params)

	items := make([]api.LLMProxyListItem, len(configs))
	for i, cfg := range configs {
		status := api.LLMProxyListItemStatus(cfg.Status)

		// Convert SourceConfiguration to LLMProxyConfiguration
		var proxy api.LLMProxyConfiguration
		j, _ := json.Marshal(cfg.SourceConfiguration)
		if err := json.Unmarshal(j, &proxy); err != nil {
			log.Error("Failed to unmarshal stored LLM proxy configuration", zap.String("uuid", cfg.ID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status: "error", Message: "Failed to get stored LLM proxy configuration"})
			return
		}

		items[i] = api.LLMProxyListItem{
			Id:          stringPtr(proxy.Metadata.Name),
			DisplayName: stringPtr(proxy.Spec.DisplayName),
			Version:     stringPtr(proxy.Spec.Version),
			Provider:    stringPtr(proxy.Spec.Provider),
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
		log.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(c)

	// Delegate to service which parses/validates/transforms and persists
	stored, err := s.llmDeploymentService.CreateLLMProxy(utils.LLMDeploymentParams{
		Data:        body,
		ContentType: c.GetHeader("Content-Type"),
		Logger:      log,
	})
	if err != nil {
		log.Error("Failed to create LLM proxy", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	// Set up a callback to notify platform API after successful deployment
	// This is specific to direct API creation via gateway endpoint
	if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() {
		go s.waitForDeploymentAndNotify(stored.ID, correlationID, log)
	}

	log.Info("LLM proxy created successfully",
		zap.String("uuid", stored.ID),
		zap.String("handle", stored.GetHandle()))

	c.JSON(http.StatusCreated, api.LLMProxyCreateResponse{
		Status:  stringPtr("success"),
		Message: stringPtr("LLM proxy created successfully"),
		Id:      stringPtr(stored.GetHandle()), CreatedAt: timePtr(stored.CreatedAt)})

	// Build and add policy config derived from API configuration if policies are present
	if s.policyManager != nil {
		storedPolicy := s.buildStoredPolicyFromAPI(stored)
		if storedPolicy != nil {
			if err := s.policyManager.AddPolicy(storedPolicy); err != nil {
				log.Error("Failed to add derived policy configuration", zap.Error(err))
			} else {
				log.Info("Derived policy configuration added",
					zap.String("policy_id", storedPolicy.ID),
					zap.Int("route_count", len(storedPolicy.Configuration.Routes)))
			}
		}
	}
}

// GetLLMProxyById implements ServerInterface.GetLLMProxyById
// (GET /llm-proxies/{id})
func (s *APIServer) GetLLMProxyById(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)

	cfg := s.store.GetByKindAndHandle(string(api.LlmProxy), id)
	if cfg == nil {
		log.Warn("LLM proxy configuration not found",
			zap.String("handle", id))
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
			"status":     string(cfg.Status),
			"created_at": cfg.CreatedAt.Format(time.RFC3339),
			"updated_at": cfg.UpdatedAt.Format(time.RFC3339),
		},
	}

	if cfg.DeployedAt != nil {
		proxyDetail["metadata"].(gin.H)["deployed_at"] = cfg.DeployedAt.Format(time.RFC3339)
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
		log.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID
	correlationID := middleware.GetCorrelationID(c)

	// Delegate to service update wrapper
	updated, err := s.llmDeploymentService.UpdateLLMProxy(id, utils.LLMDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to update LLM proxy configuration", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, api.LLMProxyUpdateResponse{
		Id:        stringPtr(updated.GetHandle()),
		Message:   stringPtr("LLM proxy updated successfully"),
		Status:    stringPtr("success"),
		UpdatedAt: timePtr(updated.UpdatedAt),
	})

	// Rebuild and update derived policy configuration
	if s.policyManager != nil {
		storedPolicy := s.buildStoredPolicyFromAPI(updated)
		if storedPolicy != nil {
			if err := s.policyManager.AddPolicy(storedPolicy); err != nil {
				log.Error("Failed to update derived policy configuration", zap.Error(err))
			} else {
				log.Info("Derived policy configuration updated",
					zap.String("policy_id", storedPolicy.ID),
					zap.Int("route_count", len(storedPolicy.Configuration.Routes)))
			}
		} else {
			// LLM proxy no longer has policies, remove the existing policy configuration
			policyID := updated.ID + "-policies"
			if err := s.policyManager.RemovePolicy(policyID); err != nil {
				// Log at debug level since policy may not exist if LLM provider never had policies
				log.Debug("No policy configuration to remove", zap.String("policy_id", policyID))
			} else {
				log.Info("Derived policy configuration removed (LLM provider no longer has policies)",
					zap.String("policy_id", policyID))
			}
		}
	}
}

// DeleteLLMProxy implements ServerInterface.DeleteLLMProxy
// (DELETE /llm-proxies/{id})
func (s *APIServer) DeleteLLMProxy(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)

	cfg, err := s.llmDeploymentService.DeleteLLMProxy(id, correlationID, log)
	if err != nil {
		log.Warn("Failed to delete LLM proxy configuration", zap.String("handle", id), zap.Error(err))
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
		"id":      cfg.GetHandle(),
	})

	// Remove derived policy configuration
	if s.policyManager != nil {
		policyID := cfg.ID + "-policies"
		if err := s.policyManager.RemovePolicy(policyID); err != nil {
			log.Warn("Failed to remove derived policy configuration", zap.Error(err), zap.String("policy_id", policyID))
		} else {
			log.Info("Derived policy configuration removed", zap.String("policy_id", policyID))
		}
	}
}

// ListPolicies implements ServerInterface.ListPolicies
// (GET /policies)
func (s *APIServer) ListPolicies(c *gin.Context) {
	// Collect and sort policies loaded from files at startup (excluding system policies)
	s.policyDefMu.RLock()
	list := make([]api.PolicyDefinition, 0, len(s.policyDefinitions))
	for _, d := range s.policyDefinitions {
		list = append(list, d)
	}
	s.policyDefMu.RUnlock()

	sort.Slice(list, func(i, j int) bool {
		if list[i].Name == list[j].Name {
			return list[i].Version < list[j].Version
		}
		return list[i].Name < list[j].Name
	})

	count := len(list)
	resp := struct {
		Status   string                 `json:"status"`
		Count    int                    `json:"count"`
		Policies []api.PolicyDefinition `json:"policies"`
	}{Status: "success", Count: count, Policies: list}
	c.JSON(http.StatusOK, resp)
}

// buildStoredPolicyFromAPI constructs a StoredPolicyConfig from an API config
// Merging rules: When operation has policies, they define the order (can reorder, override, or extend API policies).
// Remaining API-level policies not mentioned in operation policies are appended at the end.
// When operation has no policies, API-level policies are used in their declared order.
// RouteKey uses the fully qualified route path (context + operation path) and must match the route name format
// used by the xDS translator for consistency.
func (s *APIServer) buildStoredPolicyFromAPI(cfg *models.StoredConfig) *models.StoredPolicyConfig {
	// TODO: (renuka) duplicate buildStoredPolicyFromAPI funcs. Refactor this.
	apiCfg := &cfg.Configuration

	// Collect API-level policies
	apiPolicies := make(map[string]policyenginev1.PolicyInstance) // name -> policy
	if cfg.GetPolicies() != nil {
		for _, p := range *cfg.GetPolicies() {
			apiPolicies[p.Name] = convertAPIPolicy(p)
		}
	}

	routes := make([]policyenginev1.PolicyChain, 0)
	switch apiCfg.Kind {
	case api.WebSubApi:
		// Build routes with merged policies
		apiData, err := apiCfg.Spec.AsWebhookAPIData()
		if err != nil {
			// Handle error appropriately (e.g., log or return)
			return nil
		}
		for _, ch := range apiData.Channels {
			var finalPolicies []policyenginev1.PolicyInstance

			if ch.Subscribe.Policies != nil && len(*ch.Subscribe.Policies) > 0 {
				// Operation has policies: use operation policy order as authoritative
				// This allows operations to reorder, override, or extend API-level policies
				finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*ch.Subscribe.Policies))
				addedNames := make(map[string]struct{})

				for _, opPolicy := range *ch.Subscribe.Policies {
					finalPolicies = append(finalPolicies, convertAPIPolicy(opPolicy))
					addedNames[opPolicy.Name] = struct{}{}
				}

				// Add any API-level policies not mentioned in operation policies (append at end)
				if apiData.Policies != nil {
					for _, apiPolicy := range *apiData.Policies {
						if _, exists := addedNames[apiPolicy.Name]; !exists {
							finalPolicies = append(finalPolicies, apiPolicies[apiPolicy.Name])
						}
					}
				}
			} else {
				// No operation policies: use API-level policies in their declared order
				if apiData.Policies != nil {
					finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*apiData.Policies))
					for _, p := range *apiData.Policies {
						finalPolicies = append(finalPolicies, apiPolicies[p.Name])
					}
				}
			}

			routeKey := xds.GenerateRouteName("POST", apiData.Context, apiData.Version, ch.Path, s.routerConfig.GatewayHost)

			// Inject system policies into the chain
			props := make(map[string]any)
			injectedPolicies := utils.InjectSystemPolicies(finalPolicies, s.systemConfig, props)

			routes = append(routes, policyenginev1.PolicyChain{
				RouteKey: routeKey,
				Policies: injectedPolicies,
			})
		}
	case api.RestApi:
		// Build routes with merged policies
		apiData, err := apiCfg.Spec.AsAPIConfigData()
		if err != nil {
			// Handle error appropriately (e.g., log or return)
			return nil
		}
		for _, op := range apiData.Operations {
			var finalPolicies []policyenginev1.PolicyInstance

			if op.Policies != nil && len(*op.Policies) > 0 {
				// Operation has policies: use operation policy order as authoritative
				// This allows operations to reorder, override, or extend API-level policies
				finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*op.Policies))
				addedNames := make(map[string]struct{})

				for _, opPolicy := range *op.Policies {
					finalPolicies = append(finalPolicies, convertAPIPolicy(opPolicy))
					addedNames[opPolicy.Name] = struct{}{}
				}

				// Add any API-level policies not mentioned in operation policies (append at end)
				if apiData.Policies != nil {
					for _, apiPolicy := range *apiData.Policies {
						if _, exists := addedNames[apiPolicy.Name]; !exists {
							finalPolicies = append(finalPolicies, apiPolicies[apiPolicy.Name])
						}
					}
				}
			} else {
				// No operation policies: use API-level policies in their declared order
				if apiData.Policies != nil {
					finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*apiData.Policies))
					for _, p := range *apiData.Policies {
						finalPolicies = append(finalPolicies, apiPolicies[p.Name])
					}
				}
			}

			// Determine effective vhosts (fallback to global router defaults when not provided)
			effectiveMainVHost := s.routerConfig.VHosts.Main.Default
			effectiveSandboxVHost := s.routerConfig.VHosts.Sandbox.Default
			if apiData.Vhosts != nil {
				if strings.TrimSpace(apiData.Vhosts.Main) != "" {
					effectiveMainVHost = apiData.Vhosts.Main
				}
				if apiData.Vhosts.Sandbox != nil && strings.TrimSpace(*apiData.Vhosts.Sandbox) != "" {
					effectiveSandboxVHost = *apiData.Vhosts.Sandbox
				}
			}

			vhosts := []string{effectiveMainVHost}
			if apiData.Upstream.Sandbox != nil && apiData.Upstream.Sandbox.Url != nil &&
				strings.TrimSpace(*apiData.Upstream.Sandbox.Url) != "" {
				vhosts = append(vhosts, effectiveSandboxVHost)
			}

			// Populate props for system policies
			props := make(map[string]any)
			s.populatePropsForSystemPolicies(cfg.SourceConfiguration, props)

			// If this is an LLM provider, get the template and pass it to analytics policy
			for _, vhost := range vhosts {
				// Inject system policies into the chain
				injectedPolicies := utils.InjectSystemPolicies(finalPolicies, s.systemConfig, props)

				routes = append(routes, policyenginev1.PolicyChain{
					RouteKey: xds.GenerateRouteName(string(op.Method), apiData.Context, apiData.Version, op.Path, vhost),
					Policies: injectedPolicies,
				})
			}
		}
	}

	// If there are no policies at all (including system policies), return nil (skip creation)
	policyCount := 0
	for _, r := range routes {
		policyCount += len(r.Policies)
	}
	if policyCount == 0 {
		return nil
	}

	now := time.Now().Unix()
	stored := &models.StoredPolicyConfig{
		ID: cfg.ID + "-policies",
		Configuration: policyenginev1.Configuration{
			Routes: routes,
			Metadata: policyenginev1.Metadata{
				CreatedAt:       now,
				UpdatedAt:       now,
				ResourceVersion: 0,
				APIName:         cfg.GetDisplayName(),
				Version:         cfg.GetVersion(),
				Context:         cfg.GetContext(),
			},
		},
		Version: 0,
	}
	return stored
}

// convertAPIPolicy converts generated api.Policy to policyenginev1.PolicyInstance
func convertAPIPolicy(p api.Policy) policyenginev1.PolicyInstance {
	paramsMap := make(map[string]interface{})
	if p.Params != nil {
		for k, v := range *p.Params {
			paramsMap[k] = v
		}
	}
	return policyenginev1.PolicyInstance{
		Name:               p.Name,
		Version:            p.Version,
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
		log.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(c)

	// Deploy MCP configuration using the utility service
	cfg, err := s.mcpDeploymentService.CreateMCPProxy(utils.MCPDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		ID:            "", // Empty to generate new UUID
		CorrelationID: correlationID,
		Logger:        log,
	})

	if err != nil {
		log.Error("Failed to deploy MCP proxy configuration", zap.Error(err))
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

	// Return success response (id is the handle)
	c.JSON(http.StatusCreated, api.MCPProxyCreateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("MCP proxy configuration created successfully"),
		Id:        stringPtr(cfg.GetHandle()),
		CreatedAt: timePtr(cfg.CreatedAt),
	})

	// Set up a callback to notify platform API after successful deployment
	// This is specific to direct API creation via gateway endpoint
	if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() {
		go s.waitForDeploymentAndNotify(cfg.ID, correlationID, log)
	}

	// Build and add policy config derived from API configuration if policies are present
	if s.policyManager != nil {
		storedPolicy := s.buildStoredPolicyFromAPI(cfg)
		if storedPolicy != nil {
			if err := s.policyManager.AddPolicy(storedPolicy); err != nil {
				log.Error("Failed to add derived policy configuration", zap.Error(err))
			} else {
				log.Info("Derived policy configuration added",
					zap.String("policy_id", storedPolicy.ID),
					zap.Int("route_count", len(storedPolicy.Configuration.Routes)))
			}
		}
	}
}

// ListMCPProxies implements ServerInterface.ListMCPProxies
// (GET /mcp-proxies)
func (s *APIServer) ListMCPProxies(c *gin.Context, params api.ListMCPProxiesParams) {
	if (params.DisplayName != nil && *params.DisplayName != "") || (params.Version != nil && *params.Version != "") || (params.Context != nil && *params.Context != "") || (params.Status != nil && *params.Status != "") {
		s.SearchDeployments(c, string(api.Mcp))
		return
	}
	configs := s.store.GetAllByKind(string(api.Mcp))

	items := make([]api.MCPProxyListItem, len(configs))
	for i, cfg := range configs {
		status := api.MCPProxyListItemStatus(cfg.Status)
		// Convert SourceConfiguration to MCPProxyConfiguration
		var mcp api.MCPProxyConfiguration
		j, _ := json.Marshal(cfg.SourceConfiguration)
		err := json.Unmarshal(j, &mcp)
		if err != nil {
			s.logger.Error("Failed to unmarshal stored MCP configuration",
				zap.String("id", cfg.ID),
				zap.String("displayName", cfg.GetDisplayName()))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to get stored MCP configuration",
			})
			return
		}
		li := api.MCPProxyListItem{
			Id:          stringPtr(cfg.GetHandle()),
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
		"status":      "success",
		"count":       len(items),
		"mcp_proxies": items,
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
				zap.String("handle", handle))
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("MCP proxy configuration with handle '%s' not found", handle),
			})
			return
		}

		log.Error("Failed to retrieve MCP proxy configuration",
			zap.String("handle", handle),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve MCP proxy configuration",
		})
		return

	}

	// Check deployment kind is MCP
	if cfg.Kind != string(api.Mcp) {
		log.Warn("Configuration kind mismatch",
			zap.String("expected", string(api.Mcp)),
			zap.String("actual", cfg.Kind),
			zap.String("handle", handle))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Configuration with handle '%s' is not of kind MCP", handle),
		})
		return
	}

	mcpDetail := gin.H{
		"id":            cfg.GetHandle(),
		"configuration": cfg.SourceConfiguration,
		"metadata": gin.H{
			"status":     string(cfg.Status),
			"created_at": cfg.CreatedAt.Format(time.RFC3339),
			"updated_at": cfg.UpdatedAt.Format(time.RFC3339),
		},
	}

	if cfg.DeployedAt != nil {
		mcpDetail["metadata"].(gin.H)["deployed_at"] = cfg.DeployedAt.Format(time.RFC3339)
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
		log.Error("Failed to read request body", zap.Error(err))
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
		CorrelationID: correlationID,
		Logger:        log,
	}, log)

	if err != nil {
		log.Warn("MCP proxy configuration not found",
			zap.String("handle", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("MCP configuration with handle '%s' not found", handle),
		})
		return
	}

	log.Info("MCP proxy configuration updated",
		zap.String("id", updated.ID),
		zap.String("handle", handle))

	// Rebuild and update derived policy configuration
	if s.policyManager != nil {
		storedPolicy := s.buildStoredPolicyFromAPI(updated)
		if storedPolicy != nil {
			if err := s.policyManager.AddPolicy(storedPolicy); err != nil {
				log.Error("Failed to update derived policy configuration", zap.Error(err))
			} else {
				log.Info("Derived policy configuration updated",
					zap.String("policy_id", storedPolicy.ID),
					zap.Int("route_count", len(storedPolicy.Configuration.Routes)))
			}
		} else {
			// MCP proxy no longer has policies, remove the existing policy configuration
			policyID := updated.ID + "-policies"
			if err := s.policyManager.RemovePolicy(policyID); err != nil {
				// Log at debug level since policy may not exist if MCP proxy never had policies
				log.Debug("No policy configuration to remove", zap.String("policy_id", policyID))
			} else {
				log.Info("Derived policy configuration removed (MCP proxy no longer has policies)",
					zap.String("policy_id", policyID))
			}
		}
	}

	// Return success response (id is the handle)
	c.JSON(http.StatusOK, api.MCPProxyUpdateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("MCP proxy configuration updated successfully"),
		Id:        stringPtr(updated.GetHandle()),
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
		log.Warn("Failed to delete MCP proxy configuration", zap.String("handle", handle), zap.Error(err))
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

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
			log.Error("Failed to update xDS snapshot", zap.Error(err))
		}
	}()

	// Remove derived policy configuration
	if s.policyManager != nil {
		policyID := cfg.ID + "-policies"
		if err := s.policyManager.RemovePolicy(policyID); err != nil {
			log.Warn("Failed to remove derived policy configuration", zap.Error(err), zap.String("policy_id", policyID))
		} else {
			log.Info("Derived policy configuration removed", zap.String("policy_id", policyID))
		}
	}

	log.Info("MCP proxy configuration deleted",
		zap.String("id", cfg.ID),
		zap.String("handle", handle))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MCP proxy configuration deleted successfully",
		"id":      handle,
	})
}

// waitForDeploymentAndNotify waits for API deployment to complete and notifies platform API
// This is only called for APIs created directly via gateway endpoint (not from platform API)
func (s *APIServer) waitForDeploymentAndNotify(configID string, correlationID string, log *zap.Logger) {
	// Create a logger with correlation ID if provided
	if correlationID != "" {
		log = log.With(zap.String("correlation_id", correlationID))
	}

	// Poll for deployment status with timeout
	timeout := time.NewTimer(30 * time.Second)       // 30 second timeout
	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms
	defer timeout.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			log.Warn("Timeout waiting for API deployment to complete for platform API notification",
				zap.String("config_id", configID))
			return

		case <-ticker.C:
			cfg, err := s.store.Get(configID)
			if err != nil {
				log.Warn("Config not found while waiting for deployment completion",
					zap.String("config_id", configID))
				return
			}

			if cfg.Status == models.StatusDeployed {
				// // API successfully deployed, notify platform API
				log.Info("API deployed successfully, notifying platform API",
					zap.String("config_id", configID),
					zap.String("displayName", cfg.GetDisplayName()))

				// Extract API ID from stored config (use config ID as API ID)
				apiID := configID

				// Use empty revision ID for now (can be made configurable later)
				revisionID := ""

				if err := s.controlPlaneClient.NotifyAPIDeployment(apiID, cfg, revisionID); err != nil {
					log.Error("Failed to notify platform-api of successful deployment",
						zap.String("api_id", apiID),
						zap.Error(err))
				} else {
					log.Info("Successfully notified platform API of deployment",
						zap.String("api_id", apiID))
				}
				return

			} else if cfg.Status == models.StatusFailed {
				log.Warn("API deployment failed, skipping platform API notification",
					zap.String("config_id", configID),
					zap.String("displayName", cfg.GetDisplayName()))
				return
			}
			// Continue waiting if status is still pending
		}
	}
}

// GetConfigDump implements the GET /config_dump endpoint
func (s *APIServer) GetConfigDump(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)
	log.Info("Retrieving configuration dump")

	// Get all APIs
	allConfigs := s.store.GetAll()

	// Build API list with metadata using the generated types
	apisSlice := make([]api.ConfigDumpAPIItem, 0, len(allConfigs))

	for _, cfg := range allConfigs {
		// Use handle (metadata.name) as the id in the dump
		configHandle := cfg.GetHandle()
		if configHandle == "" {
			log.Warn("Config missing handle, skipping in dump", zap.String("id", cfg.ID))
			continue
		}

		// Convert status to the correct type
		var status api.ConfigDumpAPIMetadataStatus
		switch cfg.Status {
		case models.StatusDeployed:
			status = api.ConfigDumpAPIMetadataStatusDeployed
		case models.StatusFailed:
			status = api.ConfigDumpAPIMetadataStatusFailed
		case models.StatusPending:
			status = api.ConfigDumpAPIMetadataStatusPending
		default:
			status = api.ConfigDumpAPIMetadataStatusPending
		}

		item := api.ConfigDumpAPIItem{
			Configuration: &cfg.Configuration,
			Id:            convertHandleToUUID(configHandle),
			Metadata: &api.ConfigDumpAPIMetadata{
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
	policies := make([]api.PolicyDefinition, 0, len(s.policyDefinitions))
	for _, policy := range s.policyDefinitions {
		policies = append(policies, policy)
	}
	s.policyDefMu.RUnlock()

	// Sort policies for consistent output
	sort.Slice(policies, func(i, j int) bool {
		if policies[i].Name == policies[j].Name {
			return policies[i].Version < policies[j].Version
		}
		return policies[i].Name < policies[j].Name
	})

	// Get all certificates
	var certificates []api.CertificateResponse
	totalBytes := 0

	if s.db == nil {
		// Memory-only mode: return empty certificate list
		log.Debug("Storage is memory-only, returning empty certificate list")
	} else {
		certs, err := s.db.ListCertificates()
		if err != nil {
			log.Error("Failed to retrieve certificates", zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to retrieve certificates",
			})
			return
		}

		for _, cert := range certs {
			totalBytes += len(cert.Certificate)

			certStatus := api.CertificateResponseStatus("success")
			certificates = append(certificates, api.CertificateResponse{
				Id:       &cert.ID,
				Name:     &cert.Name,
				Subject:  &cert.Subject,
				Issuer:   &cert.Issuer,
				NotAfter: &cert.NotAfter,
				Count:    &cert.CertCount,
				Status:   &certStatus,
			})
		}
	}

	// Calculate statistics
	totalApis := len(apisSlice)
	totalPolicies := len(policies)
	totalCertificates := len(certificates)

	timestamp := time.Now()
	status := "success"

	// Build response
	response := api.ConfigDumpResponse{
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
	}

	c.JSON(http.StatusOK, response)
	log.Info("Configuration dump retrieved successfully",
		zap.Int("apis", len(apisSlice)),
		zap.Int("policies", len(policies)),
		zap.Int("certificates", len(certificates)))
}

// GenerateAPIKey implements ServerInterface.GenerateAPIKey
// (POST /apis/{id}/generate-api-key)
func (s *APIServer) GenerateAPIKey(c *gin.Context, id string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	// Extract authenticated user from context
	user, ok := s.extractAuthenticatedUser(c, "GenerateAPIKey", correlationID)
	if !ok {
		return // Error response already sent by extractAuthenticatedUser
	}

	log.Debug("Starting API key generation",
		zap.String("handle", handle),
		zap.String("user", user.UserID),
		zap.String("correlation_id", correlationID))

	// Parse and validate request body
	var request api.APIKeyGenerationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Warn("Invalid request body for API key generation",
			zap.Error(err),
			zap.String("handle", handle),
			zap.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// Prepare parameters
	params := utils.APIKeyGenerationParams{
		Handle:        handle,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.GenerateAPIKey(params)
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

	log.Info("API key generation completed",
		zap.String("handle", handle),
		zap.String("key name", result.Response.ApiKey.Name),
		zap.String("user", user.UserID),
		zap.String("correlation_id", correlationID))

	// Return the response using the generated schema
	c.JSON(http.StatusCreated, result.Response)
}

// RevokeAPIKey implements ServerInterface.RevokeAPIKey
// (POST /apis/{id}/revoke-api-key)
func (s *APIServer) RevokeAPIKey(c *gin.Context, id string) {
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
		zap.String("handle", handle),
		zap.String("user", user.UserID),
		zap.String("correlation_id", correlationID))

	// Parse and validate request body
	var request api.APIKeyRevocationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Warn("Invalid request body for API key revocation",
			zap.Error(err),
			zap.String("handle", handle),
			zap.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// Prepare parameters
	params := utils.APIKeyRevocationParams{
		Handle:        handle,
		Request:       request,
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
		zap.String("handle", handle),
		zap.String("key", s.apiKeyService.MaskAPIKey(request.ApiKey)),
		zap.String("user", user.UserID),
		zap.String("correlation_id", correlationID))

	// Return the response using the generated schema
	c.JSON(http.StatusOK, result.Response)
}

// RotateAPIKey implements ServerInterface.RotateAPIKey
// (POST /apis/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RotateAPIKey(c *gin.Context, id string, apiKeyName string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	// Extract authenticated user from context
	user, ok := s.extractAuthenticatedUser(c, "RotateAPIKey", correlationID)
	if !ok {
		return // Error response already sent by extractAuthenticatedUser
	}

	log.Debug("Starting API key rotation",
		zap.String("handle", handle),
		zap.String("key name", apiKeyName),
		zap.String("user", user.UserID),
		zap.String("correlation_id", correlationID))

	// Parse and validate request body
	var request api.APIKeyRotationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Warn("Invalid request body for API key rotation",
			zap.Error(err),
			zap.String("handle", handle),
			zap.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// Prepare parameters
	params := utils.APIKeyRotationParams{
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RotateAPIKey(params)
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
		zap.String("handle", handle),
		zap.String("key name", apiKeyName),
		zap.String("user", user.UserID),
		zap.String("correlation_id", correlationID))

	// Return the response using the generated schema
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
		zap.String("handle", handle),
		zap.String("user", user.UserID),
		zap.String("correlation_id", correlationID))

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
		zap.String("handle", handle),
		zap.String("user", user.UserID),
		zap.String("correlation_id", correlationID))

	// Return the response using the generated schema
	c.JSON(http.StatusOK, result.Response)
}

// extractAuthenticatedUser extracts and validates the authenticated user from Gin context
// Returns the AuthenticatedUser object and handles error responses automatically
func (s *APIServer) extractAuthenticatedUser(c *gin.Context, operationName string, correlationID string) (*commonmodels.AuthContext, bool) {
	log := s.logger

	// Extract authentication context
	authCtxValue, exists := c.Get(constants.AuthContextKey)
	if !exists {
		log.Error("Authentication context not found",
			zap.String("operation", operationName),
			zap.String("correlation_id", correlationID))
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
			zap.String("operation", operationName),
			zap.String("correlation_id", correlationID))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Invalid authentication context",
		})
		return nil, false
	}

	log.Debug("Authenticated user extracted",
		zap.String("operation", operationName),
		zap.String("user_id", user.UserID),
		zap.Strings("roles", user.Roles),
		zap.String("correlation_id", correlationID))

	return &user, true
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
func (s *APIServer) populatePropsForSystemPolicies(srcConfig any, props map[string]any) {
	if srcConfig == nil {
		return
	}

	// If this is an LLM provider, get the template and pass it to analytics policy
	// Check if sourceConfig is an LLM provider by checking its kind
	kind, err := utils.GetValueFromSourceConfig(srcConfig, "kind")
	if err == nil {
		if kindStr, ok := kind.(string); ok && kindStr == string(api.LlmProvider) {
			template, err := s.getLLMProviderTemplate(srcConfig)
			if err != nil {
				s.logger.Debug("Failed to get LLM provider template", zap.Error(err))
			} else if template != nil {
				// Pass the template to analytics policy
				analyticsProps := make(map[string]interface{})
				analyticsProps["providerTemplate"] = template
				props[gatewayconstants.ANALYTICS_SYSTEM_POLICY_NAME] = analyticsProps
			}
		}
	}
}

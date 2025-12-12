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
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
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
	controlPlaneClient   controlplane.ControlPlaneClient
	routerConfig         *config.RouterConfig
	httpClient           *http.Client
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
	validator config.Validator,
	routerConfig *config.RouterConfig,
) *APIServer {
	deploymentService := utils.NewAPIDeploymentService(store, db, snapshotManager, validator)
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
		llmDeploymentService: utils.NewLLMDeploymentService(store, db, snapshotManager, deploymentService),
		controlPlaneClient:   controlPlaneClient,
		routerConfig:         routerConfig,
		httpClient:           &http.Client{Timeout: 10 * time.Second},
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
			zap.String("name", cfg.GetName()),
			zap.Int64("version", version))
	} else {
		cfg.Status = models.StatusFailed
		cfg.DeployedAt = nil
		cfg.DeployedVersion = 0
		log.Error("Configuration deployment failed",
			zap.String("id", configID),
			zap.String("name", cfg.GetName()),
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
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	// Set up a callback to notify platform API after successful deployment
	// This is specific to direct API creation via gateway endpoint
	if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() {
		go s.waitForDeploymentAndNotify(result.StoredConfig.ID, correlationID, log)
	}

	// Return success response
	id, _ := uuidToOpenAPIUUID(result.StoredConfig.ID)
	c.JSON(http.StatusCreated, api.APICreateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("API configuration created successfully"),
		Id:        id,
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
		}
	}
}

// ListAPIs implements ServerInterface.ListAPIs
// (GET /apis)
func (s *APIServer) ListAPIs(c *gin.Context) {
	configs := s.store.GetAllByKind(string(api.APIConfigurationKindRestApi))

	items := make([]api.APIListItem, 0, len(configs))
	for _, cfg := range configs {
		id, _ := uuidToOpenAPIUUID(cfg.ID)
		status := string(cfg.Status)
		items = append(items, api.APIListItem{
			Id:        id,
			Name:      stringPtr(cfg.GetName()),
			Version:   stringPtr(cfg.GetVersion()),
			Context:   stringPtr(cfg.GetContext()),
			Status:    (*api.APIListItemStatus)(&status),
			CreatedAt: timePtr(cfg.CreatedAt),
			UpdatedAt: timePtr(cfg.UpdatedAt),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"count":  len(items),
		"apis":   items,
	})
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
		"id":            cfg.ID,
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
// (PUT /apis/{name}/{version})
func (s *APIServer) UpdateAPI(c *gin.Context, name string, version string) {
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

	// Parse configuration
	contentType := c.GetHeader("Content-Type")
	var apiConfig api.APIConfiguration
	err = s.parser.Parse(body, contentType, &apiConfig)
	if err != nil {
		log.Error("Failed to parse configuration", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to parse configuration",
		})
		return
	}

	// Validate configuration
	validationErrors := s.validator.Validate(&apiConfig)
	if len(validationErrors) > 0 {
		log.Warn("Configuration validation failed",
			zap.String("name", name),
			zap.Int("num_errors", len(validationErrors)))

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

	// Check if config exists
	existing, err := s.store.GetByNameVersion(name, version)
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

	// Update stored configuration
	now := time.Now()
	existing.Configuration = apiConfig
	existing.Status = models.StatusPending
	existing.UpdatedAt = now
	existing.DeployedAt = nil
	existing.DeployedVersion = 0

	if apiConfig.Kind == api.APIConfigurationKindWebsubApi {
		topicsToRegister, topicsToUnregister := s.deploymentService.GetTopicsForUpdate(*existing)
		// TODO: Pre configure the dynamic forward proxy rules for event gw
		// This was communication bridge will be created on the gw startup
		// Can perform internal communication with websub hub without relying on the dynamic rules
		// Execute topic operations with wait group and errors tracking
		var wg2 sync.WaitGroup
		var regErrs int32
		var deregErrs int32

		var waitCount int
		if len(topicsToRegister) > 0 {
			waitCount++
		}
		if len(topicsToUnregister) > 0 {
			waitCount++
		}

		wg2.Add(waitCount)

		if len(topicsToRegister) > 0 {
			go func(list []string) {
				defer wg2.Done()
				log.Info("Starting topic registration", zap.Int("total_topics", len(list)), zap.String("api_id", existing.ID))
				//fmt.Println("Topics Registering Started")
				for _, topic := range list {
					if err := s.deploymentService.RegisterTopicWithHub(s.httpClient, topic, s.routerConfig.GatewayHost, log); err != nil {
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
				}

			}(topicsToRegister)
		}

		if len(topicsToUnregister) > 0 {
			go func(list []string) {
				defer wg2.Done()
				log.Info("Starting topic deregistration", zap.Int("total_topics", len(list)), zap.String("api_id", existing.ID))
				for _, topic := range list {
					if err := s.deploymentService.UnregisterTopicWithHub(s.httpClient, topic, s.routerConfig.GatewayHost, log); err != nil {
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
				}
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
			log.Info("API configuration name/version already exists",
				zap.String("id", existing.ID),
				zap.String("name", name),
				zap.String("version", version))
		} else {
			log.Error("Failed to update config in memory store", zap.Error(err))
		}
		c.JSON(http.StatusConflict, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
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

	log.Info("API configuration updated",
		zap.String("id", existing.ID),
		zap.String("name", name),
		zap.String("version", version))

	// Return success response
	updateId, _ := uuidToOpenAPIUUID(existing.ID)
	c.JSON(http.StatusOK, api.APIUpdateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("API configuration updated successfully"),
		Id:        updateId,
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
		}
	}
}

// DeleteAPI implements ServerInterface.DeleteAPI
// (DELETE /apis/{name}/{version})
func (s *APIServer) DeleteAPI(c *gin.Context, name string, version string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)

	// Check if config exists
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
	}

	if cfg.Configuration.Kind == api.APIConfigurationKindWebsubApi {
		topicsToUnregister := s.deploymentService.GetTopicsForDelete(*cfg)

		// TODO: Pre configure the dynamic forward proxy rules for event gw
		// This was communication bridge will be created on the gw startup
		// Can perform internal communication with websub hub without relying on the dynamic rules
		// Execute topic operations with wait group and errors tracking
		var wg2 sync.WaitGroup
		var deregErrs int32

		if len(topicsToUnregister) > 0 {
			wg2.Add(1)
			go func(list []string) {
				defer wg2.Done()
				log.Info("Starting topic deregistration", zap.Int("total_topics", len(list)), zap.String("api_id", cfg.ID))
				for _, topic := range list {
					if err := s.deploymentService.UnregisterTopicWithHub(s.httpClient, topic, s.routerConfig.GatewayHost, log); err != nil {
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
				}
			}(topicsToUnregister)
			wg2.Wait()
		}

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
		zap.String("name", name),
		zap.String("version", version))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "API configuration deleted successfully",
		"name":    name,
		"version": version,
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
// (POST /llm-providers/templates)
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
		zap.String("id", storedTemplate.ID),
		zap.String("name", storedTemplate.GetName()))

	id, _ := uuidToOpenAPIUUID(storedTemplate.ID)

	c.JSON(http.StatusCreated, api.LLMProviderTemplateCreateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("LLM provider template created successfully"),
		Id:        id,
		CreatedAt: timePtr(storedTemplate.CreatedAt),
	})
}

// ListLLMProviderTemplates implements ServerInterface.ListLLMProviderTemplates
// (GET /llm-providers/templates)
func (s *APIServer) ListLLMProviderTemplates(c *gin.Context) {
	templates := s.llmDeploymentService.ListLLMProviderTemplates()

	items := make([]api.LLMProviderTemplateListItem, len(templates))
	for i, tmpl := range templates {
		id, _ := uuidToOpenAPIUUID(tmpl.ID)
		items[i] = api.LLMProviderTemplateListItem{
			Id:        id,
			Name:      stringPtr(tmpl.GetName()),
			CreatedAt: timePtr(tmpl.CreatedAt),
			UpdatedAt: timePtr(tmpl.UpdatedAt),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"count":     len(items),
		"templates": items,
	})
}

// GetLLMProviderTemplateByName implements ServerInterface.GetLLMProviderTemplateByName
// (GET /llm-providers/templates/{name})
func (s *APIServer) GetLLMProviderTemplateByName(c *gin.Context, name string) {
	log := middleware.GetLogger(c, s.logger)

	template, err := s.llmDeploymentService.GetLLMProviderTemplateByName(name)
	if err != nil {
		log.Warn("LLM provider template not found", zap.String("name", name))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Template with name '%s' not found", name),
		})
		return
	}

	id, _ := uuidToOpenAPIUUID(template.ID)

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
// (PUT /llm-providers/templates/{name})
func (s *APIServer) UpdateLLMProviderTemplate(c *gin.Context, name string) {
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

	updated, err := s.llmDeploymentService.UpdateLLMProviderTemplate(name, utils.LLMTemplateParams{
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
		zap.String("id", updated.ID),
		zap.String("name", updated.GetName()))

	id, _ := uuidToOpenAPIUUID(updated.ID)
	c.JSON(http.StatusOK, api.LLMProviderTemplateUpdateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("LLM provider template updated successfully"),
		Id:        id,
		UpdatedAt: timePtr(updated.UpdatedAt),
	})
}

// DeleteLLMProviderTemplate implements ServerInterface.DeleteLLMProviderTemplate
// (DELETE /llm-providers/templates/{name})
func (s *APIServer) DeleteLLMProviderTemplate(c *gin.Context, name string) {
	log := middleware.GetLogger(c, s.logger)

	deleted, err := s.llmDeploymentService.DeleteLLMProviderTemplate(name)
	if err != nil {
		log.Warn("LLM provider template not found for deletion", zap.String("name", name))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Template with name '%s' not found", name),
		})
		return
	}

	log.Info("LLM provider template deleted successfully",
		zap.String("id", deleted.ID),
		zap.String("name", name))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "LLM provider template deleted successfully",
		"id":      deleted.ID,
	})
}

// ListLLMProviders implements ServerInterface.ListLLMProviders
// (GET /llm-providers)
func (s *APIServer) ListLLMProviders(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)
	configs := s.llmDeploymentService.ListLLMProviders()

	items := make([]api.LLMProviderListItem, len(configs))
	for i, cfg := range configs {
		id, _ := uuidToOpenAPIUUID(cfg.ID)
		status := api.LLMProviderListItemStatus(cfg.Status)

		// Convert SourceConfiguration to LLMProviderConfiguration
		var prov api.LLMProviderConfiguration
		j, _ := json.Marshal(cfg.SourceConfiguration)
		if err := json.Unmarshal(j, &prov); err != nil {
			log.Error("Failed to unmarshal stored LLM provider configuration", zap.String("id", cfg.ID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get stored LLM provider configuration"})
			return
		}

		items[i] = api.LLMProviderListItem{
			Id:        id,
			Name:      stringPtr(prov.Spec.Name),
			Version:   stringPtr(prov.Spec.Version),
			Template:  stringPtr(prov.Spec.Template),
			Status:    &status,
			CreatedAt: timePtr(cfg.CreatedAt),
			UpdatedAt: timePtr(cfg.UpdatedAt),
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
		zap.String("id", stored.ID),
		zap.String("name", stored.GetName()))

	id, _ := uuidToOpenAPIUUID(stored.ID)
	c.JSON(http.StatusCreated, api.LLMProviderCreateResponse{
		Status:  stringPtr("success"),
		Message: stringPtr("LLM provider created successfully"),
		Id:      id, CreatedAt: timePtr(stored.CreatedAt)})

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

// GetLLMProviderByNameVersion implements ServerInterface.GetLLMProviderByNameVersion
// (GET /llm-providers/{name}/{version})
func (s *APIServer) GetLLMProviderByNameVersion(c *gin.Context, name string, version string) {
	log := middleware.GetLogger(c, s.logger)

	cfg := s.store.GetByKindNameAndVersion(string(api.Llmprovider), name, version)
	if cfg == nil {
		log.Warn("LLM provider configuration not found",
			zap.String("name", name),
			zap.String("version", version))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("LLM provider configuration with name '%s' and version '%s' not found", name, version),
		})
		return
	}

	// Build response similar to GetAPIByNameVersion
	providerDetail := gin.H{
		"id":            cfg.ID,
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
// (PUT /llm-providers/{name}/{version})
func (s *APIServer) UpdateLLMProvider(c *gin.Context, name string, version string) {
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
	updated, err := s.llmDeploymentService.UpdateLLMProvider(name, version, utils.LLMDeploymentParams{
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

	id, _ := uuidToOpenAPIUUID(updated.ID)
	c.JSON(http.StatusOK, api.LLMProviderUpdateResponse{
		Id:        id,
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
		}
	}
}

// DeleteLLMProvider implements ServerInterface.DeleteLLMProvider
// (DELETE /llm-providers/{name}/{version})
func (s *APIServer) DeleteLLMProvider(c *gin.Context, name string, version string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)

	cfg, err := s.llmDeploymentService.DeleteLLMProvider(name, version, correlationID, log)
	if err != nil {
		log.Warn("Failed to delete LLM provider configuration", zap.String("name", name), zap.String("version", version), zap.Error(err))
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
		"name":    name,
		"version": version,
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
	// Collect and sort policies loaded from files at startup
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
	if apiCfg.Kind == api.APIConfigurationKindWebsubApi {
		// Build routes with merged policies
		apiData, err := apiCfg.Spec.AsWebhookAPIData()
		if err != nil {
			// Handle error appropriately (e.g., log or return)
			return nil
		}
		for _, ch := range apiData.Channels {
			var finalPolicies []policyenginev1.PolicyInstance

			if ch.Policies != nil && len(*ch.Policies) > 0 {
				// Operation has policies: use operation policy order as authoritative
				// This allows operations to reorder, override, or extend API-level policies
				finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*ch.Policies))
				addedNames := make(map[string]struct{})

				for _, opPolicy := range *ch.Policies {
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

			routeKey := xds.GenerateRouteName("SUBSCRIBE", apiData.Context, apiData.Version, ch.Path, s.routerConfig.GatewayHost)
			routes = append(routes, policyenginev1.PolicyChain{
				RouteKey: routeKey,
				Policies: finalPolicies,
			})
		}
	} else if apiCfg.Kind == api.APIConfigurationKindRestApi {
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

			routeKey := xds.GenerateRouteName(string(op.Method), apiData.Context, apiData.Version, op.Path, s.routerConfig.GatewayHost)
			routes = append(routes, policyenginev1.PolicyChain{
				RouteKey: routeKey,
				Policies: finalPolicies,
			})
		}
	}

	// If there are no policies at all, return nil (skip creation)
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
				APIName:         cfg.GetName(),
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
	result, err := s.mcpDeploymentService.DeployMCPConfiguration(utils.MCPDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		ID:            "", // Empty to generate new UUID
		CorrelationID: correlationID,
		Logger:        log,
	})

	if err != nil {
		log.Error("Failed to deploy MCP proxy configuration", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	// Set up a callback to notify platform API after successful deployment
	// This is specific to direct API creation via gateway endpoint
	if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() {
		go s.waitForDeploymentAndNotify(result.StoredConfig.ID, correlationID, log)
	}

	// Return success response
	id, _ := uuidToOpenAPIUUID(result.StoredConfig.ID)
	c.JSON(http.StatusCreated, api.APICreateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("MCP configuration created successfully"),
		Id:        id,
		CreatedAt: timePtr(result.StoredConfig.CreatedAt),
	})
}

// ListMCPProxies implements ServerInterface.ListMCPProxies
// (GET /mcp-proxies)
func (s *APIServer) ListMCPProxies(c *gin.Context) {
	configs := s.store.GetAllByKind(string(api.Mcp))

	items := make([]api.MCPProxyListItem, len(configs))
	for i, cfg := range configs {
		id, _ := uuidToOpenAPIUUID(cfg.ID)
		status := api.MCPProxyListItemStatus(cfg.Status)
		// Convert SourceConfiguration to MCPProxyConfiguration
		var mcp api.MCPProxyConfiguration
		j, _ := json.Marshal(cfg.SourceConfiguration)
		err := json.Unmarshal(j, &mcp)
		if err != nil {
			s.logger.Error("Failed to unmarshal stored MCP configuration",
				zap.String("id", cfg.ID),
				zap.String("name", cfg.GetName()))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to get stored MCP configuration",
			})
			return
		}
		items[i] = api.MCPProxyListItem{
			Id:        id,
			Name:      stringPtr(mcp.Spec.Name),
			Version:   stringPtr(mcp.Spec.Version),
			Context:   stringPtr(mcp.Spec.Context),
			Status:    &status,
			CreatedAt: timePtr(cfg.CreatedAt),
			UpdatedAt: timePtr(cfg.UpdatedAt),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"count":       len(items),
		"mcp_proxies": items,
	})
}

// GetMCPProxyByNameVersion implements ServerInterface.GetMCPProxyByNameVersion
// (GET /mcp-proxies/{name}/{version})
func (s *APIServer) GetMCPProxyByNameVersion(c *gin.Context, name string, version string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)

	cfg, err := s.store.GetByNameVersion(name, version)
	if err != nil {
		log.Warn("MCP proxy configuration not found",
			zap.String("name", name),
			zap.String("version", version))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("MCP proxy configuration with name '%s' and version '%s' not found", name, version),
		})
		return
	}

	mcpDetail := gin.H{
		"status": "success",
		"mcp": gin.H{
			"id":            cfg.ID,
			"configuration": cfg.SourceConfiguration,
			"metadata": gin.H{
				"status":     string(cfg.Status),
				"created_at": cfg.CreatedAt.Format(time.RFC3339),
				"updated_at": cfg.UpdatedAt.Format(time.RFC3339),
			},
		},
	}
	if cfg.DeployedAt != nil {
		mcpDetail["mcp"].(gin.H)["metadata"].(gin.H)["deployed_at"] = cfg.DeployedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, mcpDetail)
}

// UpdateMCPProxy implements ServerInterface.UpdateMCPProxy
// (PUT /mcp-proxies/{name}/{version})
func (s *APIServer) UpdateMCPProxy(c *gin.Context, name string, version string) {
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

	// Parse configuration
	contentType := c.GetHeader("Content-Type")
	var mcpConfig api.MCPProxyConfiguration
	err = s.parser.Parse(body, contentType, &mcpConfig)
	if err != nil {
		log.Error("Failed to parse configuration", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to parse configuration",
		})
		return
	}

	mcpValidator := config.NewMCPValidator()
	// Validate configuration
	validationErrors := mcpValidator.Validate(&mcpConfig)
	if len(validationErrors) > 0 {
		log.Warn("Configuration validation failed",
			zap.String("name", mcpConfig.Spec.Name),
			zap.Int("num_errors", len(validationErrors)))

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

	// Check if config exists
	existing, err := s.store.GetByNameVersion(name, version)
	if err != nil {
		log.Warn("MCP configuration not found",
			zap.String("name", name),
			zap.String("version", version))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("MCP configuration with name '%s' and version '%s' not found", name, version),
		})
		return
	}

	// Ensure existing config is of kind MCP
	if existing.Kind != string(api.Mcp) {
		log.Warn("Configuration kind mismatch",
			zap.String("expected", string(api.Mcp)),
			zap.String("actual", existing.Kind),
			zap.String("name", name),
			zap.String("version", version))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Configuration with name '%s' and version '%s' is not of kind MCP", name, version),
		})
		return
	}

	// Transform to API configuration using MCPTransformer
	var apiConfig api.APIConfiguration
	transformer := &utils.MCPTransformer{}
	transformedAPIConfig, err := transformer.Transform(&mcpConfig, &apiConfig)
	if transformedAPIConfig == nil {
		log.Error("Failed to transform MCP configuration to API configuration",
			zap.String("name", name),
			zap.String("version", version))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to transform MCP configuration",
		})
		return
	}
	apiConfig = *transformedAPIConfig

	// Update stored configuration
	now := time.Now()
	existing.Configuration = apiConfig
	existing.SourceConfiguration = mcpConfig
	existing.Status = models.StatusPending
	existing.UpdatedAt = now
	existing.DeployedAt = nil
	existing.DeployedVersion = 0

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
			log.Info("MCP configuration name/version already exists",
				zap.String("id", existing.ID),
				zap.String("name", existing.GetName()),
				zap.String("version", existing.GetVersion()))
		} else {
			log.Error("Failed to update config in memory store", zap.Error(err))
		}
		c.JSON(http.StatusConflict, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
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

	log.Info("MCP configuration updated",
		zap.String("id", existing.ID),
		zap.String("name", existing.GetName()),
		zap.String("version", existing.GetVersion()))

	// Return success response
	updateId, _ := uuidToOpenAPIUUID(existing.ID)
	c.JSON(http.StatusOK, api.APIUpdateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("MCP configuration updated successfully"),
		Id:        updateId,
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
		}
	}
}

// DeleteMCPProxy implements ServerInterface.DeleteMCPProxy
// (DELETE /mcp-proxies/{name}/{version})
func (s *APIServer) DeleteMCPProxy(c *gin.Context, name string, version string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)

	// Check if config exists
	cfg, err := s.store.GetByNameVersion(name, version)
	if err != nil {
		log.Warn("MCP proxy configuration not found",
			zap.String("name", name),
			zap.String("version", version))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("MCP proxy configuration with name '%s' and version '%s' not found", name, version),
		})
		return
	}

	// Ensure existing config is of kind MCP
	if cfg.Kind != string(api.Mcp) {
		log.Warn("Configuration kind mismatch",
			zap.String("expected", string(api.Mcp)),
			zap.String("actual", cfg.Kind),
			zap.String("name", name),
			zap.String("version", version))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Configuration with name '%s' and version '%s' is not of kind MCP", name, version),
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
		zap.String("name", cfg.GetName()),
		zap.String("version", cfg.GetVersion()))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MCP proxy configuration deleted successfully",
		"name":    name,
		"version": version,
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
					zap.String("name", cfg.GetName()))

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
					zap.String("name", cfg.GetName()))
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

	// Build API list with metadata using the exact generated types
	apisSlice := make([]struct {
		Configuration *api.APIConfiguration `json:"configuration,omitempty" yaml:"configuration,omitempty"`
		Id            *openapi_types.UUID   `json:"id,omitempty" yaml:"id,omitempty"`
		Metadata      *struct {
			CreatedAt  *time.Time                                `json:"created_at,omitempty" yaml:"created_at,omitempty"`
			DeployedAt *time.Time                                `json:"deployed_at,omitempty" yaml:"deployed_at,omitempty"`
			Status     *api.ConfigDumpResponseApisMetadataStatus `json:"status,omitempty" yaml:"status,omitempty"`
			UpdatedAt  *time.Time                                `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
		} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	}, 0, len(allConfigs))

	for _, cfg := range allConfigs {
		configUUID, err := uuidToOpenAPIUUID(cfg.ID)
		if err != nil {
			log.Warn("Failed to parse config ID as UUID", zap.String("id", cfg.ID), zap.Error(err))
			continue
		}

		// Convert status to the correct type
		var status api.ConfigDumpResponseApisMetadataStatus
		switch cfg.Status {
		case models.StatusDeployed:
			status = api.ConfigDumpResponseApisMetadataStatusDeployed
		case models.StatusFailed:
			status = api.ConfigDumpResponseApisMetadataStatusFailed
		case models.StatusPending:
			status = api.ConfigDumpResponseApisMetadataStatusPending
		default:
			status = api.ConfigDumpResponseApisMetadataStatusPending
		}

		item := struct {
			Configuration *api.APIConfiguration `json:"configuration,omitempty" yaml:"configuration,omitempty"`
			Id            *openapi_types.UUID   `json:"id,omitempty" yaml:"id,omitempty"`
			Metadata      *struct {
				CreatedAt  *time.Time                                `json:"created_at,omitempty" yaml:"created_at,omitempty"`
				DeployedAt *time.Time                                `json:"deployed_at,omitempty" yaml:"deployed_at,omitempty"`
				Status     *api.ConfigDumpResponseApisMetadataStatus `json:"status,omitempty" yaml:"status,omitempty"`
				UpdatedAt  *time.Time                                `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
			} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
		}{
			Configuration: &cfg.Configuration,
			Id:            configUUID,
			Metadata: &struct {
				CreatedAt  *time.Time                                `json:"created_at,omitempty" yaml:"created_at,omitempty"`
				DeployedAt *time.Time                                `json:"deployed_at,omitempty" yaml:"deployed_at,omitempty"`
				Status     *api.ConfigDumpResponseApisMetadataStatus `json:"status,omitempty" yaml:"status,omitempty"`
				UpdatedAt  *time.Time                                `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
			}{
				CreatedAt:  &cfg.CreatedAt,
				UpdatedAt:  &cfg.UpdatedAt,
				DeployedAt: cfg.DeployedAt,
				Status:     &status,
			},
		}
		apisSlice = append(apisSlice, item)
	}

	// Get all policies
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

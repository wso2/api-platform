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
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"

	"github.com/gin-gonic/gin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v3"
)

// APIServer implements the generated ServerInterface
type APIServer struct {
	store              *storage.ConfigStore
	db                 storage.Storage
	snapshotManager    *xds.SnapshotManager
	policyManager      *policyxds.PolicyManager
	policyDefinitions  map[string]api.PolicyDefinition // key name|version
	policyDefMu        sync.RWMutex
	parser             *config.Parser
	validator          *config.Validator
	logger             *zap.Logger
	deploymentService  *utils.APIDeploymentService
	controlPlaneClient controlplane.ControlPlaneClient
}

// NewAPIServer creates a new API server with dependencies
func NewAPIServer(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
	logger *zap.Logger,
	controlPlaneClient controlplane.ControlPlaneClient,
) *APIServer {
	server := &APIServer{
		store:              store,
		db:                 db,
		snapshotManager:    snapshotManager,
		policyManager:      policyManager,
		policyDefinitions:  make(map[string]api.PolicyDefinition),
		parser:             config.NewParser(),
		validator:          config.NewValidator(),
		logger:             logger,
		deploymentService:  utils.NewAPIDeploymentService(store, db, snapshotManager),
		controlPlaneClient: controlPlaneClient,
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
		log.Info("API configuration deployed successfully",
			zap.String("id", configID),
			zap.String("name", cfg.Configuration.Data.Name),
			zap.Int64("version", version))
	} else {
		cfg.Status = models.StatusFailed
		cfg.DeployedAt = nil
		cfg.DeployedVersion = 0
		log.Error("API configuration deployment failed",
			zap.String("id", configID),
			zap.String("name", cfg.Configuration.Data.Name))
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
		storedPolicy := buildStoredPolicyFromAPI(result.StoredConfig)
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
	configs := s.store.GetAll()

	items := make([]api.APIListItem, len(configs))
	for i, cfg := range configs {
		id, _ := uuidToOpenAPIUUID(cfg.ID)
		status := string(cfg.Status)
		items[i] = api.APIListItem{
			Id:        id,
			Name:      stringPtr(cfg.Configuration.Data.Name),
			Version:   stringPtr(cfg.Configuration.Data.Version),
			Context:   stringPtr(cfg.Configuration.Data.Context),
			Status:    (*api.APIListItemStatus)(&status),
			CreatedAt: timePtr(cfg.CreatedAt),
			UpdatedAt: timePtr(cfg.UpdatedAt),
		}
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
	apiConfig, err := s.parser.Parse(body, contentType)
	if err != nil {
		log.Error("Failed to parse configuration", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to parse configuration",
		})
		return
	}

	// Validate configuration
	validationErrors := s.validator.Validate(apiConfig)
	if len(validationErrors) > 0 {
		log.Warn("Configuration validation failed",
			zap.String("name", apiConfig.Data.Name),
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
	existing.Configuration = *apiConfig
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
			log.Info("API configuration name/version already exists",
				zap.String("id", existing.ID),
				zap.String("name", apiConfig.Data.Name),
				zap.String("version", apiConfig.Data.Version))
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
		zap.String("name", apiConfig.Data.Name),
		zap.String("version", apiConfig.Data.Version))

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
		storedPolicy := buildStoredPolicyFromAPI(existing)
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
		zap.String("name", cfg.Configuration.Data.Name),
		zap.String("version", cfg.Configuration.Data.Version))

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

// CreatePolicies implements ServerInterface.CreatePolicies
// (POST /policies)
func (s *APIServer) CreatePolicies(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Failed to read request body"})
		return
	}
	contentType := c.GetHeader("Content-Type")
	var defs []api.PolicyDefinition
	if strings.Contains(contentType, "yaml") || strings.Contains(contentType, "yml") {
		if err := yaml.Unmarshal(body, &defs); err != nil {
			log.Error("Failed to parse YAML policy definitions", zap.Error(err))
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid YAML policy definitions"})
			return
		}
	} else {
		if err := json.Unmarshal(body, &defs); err != nil {
			log.Error("Failed to parse JSON policy definitions", zap.Error(err))
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid JSON policy definitions"})
			return
		}
	}
	if len(defs) == 0 {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "No policy definitions provided"})
		return
	}
	// Validate uniqueness within incoming set and required fields
	seen := make(map[string]struct{})
	for _, d := range defs {
		if strings.TrimSpace(d.Name) == "" || strings.TrimSpace(d.Version) == "" || strings.TrimSpace(d.Provider) == "" {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Policy name, version, and provider are required"})
			return
		}
		key := d.Name + "|" + d.Version
		if _, exists := seen[key]; exists {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Duplicate policy definition in request: %s", key)})
			return
		}
		seen[key] = struct{}{}
	}

	// Replace in-memory snapshot atomically
	s.policyDefMu.Lock()
	s.policyDefinitions = make(map[string]api.PolicyDefinition, len(defs))
	for _, d := range defs {
		key := d.Name + "|" + d.Version
		s.policyDefinitions[key] = d
	}
	s.policyDefMu.Unlock()

	// Persist authoritative state if persistent storage configured
	if s.db != nil {
		if err := s.db.ReplacePolicyDefinitions(defs); err != nil {
			log.Error("Failed to persist policy definitions", zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to persist policy definitions"})
			return
		}
	}

	count := len(defs)
	resp := api.PolicyCreateResponse{Status: stringPtr("success"), Message: stringPtr("Policy definitions updated successfully"), Count: &count, Created: &defs}
	log.Info("Policy definitions replaced", zap.Int("count", count))
	c.JSON(http.StatusCreated, resp)
}

// ListPolicies implements ServerInterface.ListPolicies
// (GET /policies)
func (s *APIServer) ListPolicies(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)
	// If memory empty and persistent storage exists, hydrate from DB
	s.policyDefMu.RLock()
	empty := len(s.policyDefinitions) == 0
	s.policyDefMu.RUnlock()
	if empty && s.db != nil {
		defs, err := s.db.GetAllPolicyDefinitions()
		if err != nil {
			log.Error("Failed to load policy definitions from storage", zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to load policy definitions"})
			return
		}
		s.policyDefMu.Lock()
		for _, d := range defs {
			key := d.Name + "|" + d.Version
			s.policyDefinitions[key] = d
		}
		s.policyDefMu.Unlock()
	}

	// Collect and sort
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
// Merging rules: Operation-level policies override API-level policies with same name; API-level policies are appended otherwise.
// RouteKey uses the fully qualified route path (context + operation path).
func buildStoredPolicyFromAPI(cfg *models.StoredAPIConfig) *models.StoredPolicyConfig {
	apiCfg := &cfg.Configuration
	apiData := apiCfg.Data

	// Collect API-level policies
	apiPolicies := make(map[string]models.Policy) // name -> policy
	if apiData.Policies != nil {
		for _, p := range *apiData.Policies {
			apiPolicies[p.Name] = convertAPIPolicy(p)
		}
	}

	// Build routes with merged policies
	routes := make([]models.RoutePolicy, 0)
	for _, op := range apiData.Operations {
		// Map to track operation-level policies by name
		merged := make(map[string]models.Policy)
		if op.Policies != nil {
			for _, p := range *op.Policies {
				merged[p.Name] = convertAPIPolicy(p)
			}
		}
		// Add API-level policies not overridden
		for name, pol := range apiPolicies {
			if _, exists := merged[name]; !exists {
				merged[name] = pol
			}
		}
		// Convert map to slice preserving no particular order
		finalPolicies := make([]models.Policy, 0, len(merged))
		for _, pol := range merged {
			finalPolicies = append(finalPolicies, pol)
		}
		// Construct route key including HTTP method and API version for uniqueness and correlation.
		// Format: <METHOD>|<API_VERSION>|<CONTEXT><PATH>
		// Example: GET|1.0.0|/petstore/v1/pets
		routeKey := fmt.Sprintf("%s|%s|%s%s", op.Method, apiData.Version, apiData.Context, op.Path)
		routes = append(routes, models.RoutePolicy{
			RouteKey: routeKey,
			Policies: finalPolicies,
		})
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
		Configuration: models.PolicyConfiguration{
			Routes: routes,
			Metadata: models.Metadata{
				CreatedAt:       now,
				UpdatedAt:       now,
				ResourceVersion: 0,
				APIName:         apiData.Name,
				Version:         apiData.Version,
				Context:         apiData.Context,
			},
		},
		Version: 0,
	}
	return stored
}

// convertAPIPolicy converts generated api.Policy to models.Policy
func convertAPIPolicy(p api.Policy) models.Policy {
	paramsMap := make(map[string]interface{})
	if p.Params != nil {
		for k, v := range *p.Params {
			paramsMap[k] = v
		}
	}
	return models.Policy{
		Name:               p.Name,
		Version:            p.Version,
		ExecutionCondition: p.ExecutionCondition,
		Params:             paramsMap,
	}
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
				// API successfully deployed, notify platform API
				log.Info("API deployed successfully, notifying platform API",
					zap.String("config_id", configID),
					zap.String("name", cfg.Configuration.Data.Name))

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
					zap.String("name", cfg.Configuration.Data.Name))
				return
			}
			// Continue waiting if status is still pending
		}
	}
}

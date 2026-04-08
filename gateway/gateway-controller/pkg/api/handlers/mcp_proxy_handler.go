/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

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

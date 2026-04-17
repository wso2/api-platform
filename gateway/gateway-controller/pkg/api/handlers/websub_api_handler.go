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
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// CreateWebSubAPI implements ServerInterface.CreateWebSubAPI
// (POST /websub-apis)
func (s *APIServer) CreateWebSubAPI(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(c)

	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		Kind:          "WebSubApi",
		APIID:         "",
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to deploy WebSub API configuration", slog.Any("error", err))
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

	c.JSON(http.StatusCreated, api.WebSubAPICreateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("WebSub API configuration created successfully"),
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

// ListWebSubAPIs implements ServerInterface.ListWebSubAPIs
// (GET /websub-apis)
func (s *APIServer) ListWebSubAPIs(c *gin.Context, params api.ListWebSubAPIsParams) {
	if (params.DisplayName != nil && *params.DisplayName != "") ||
		(params.Version != nil && *params.Version != "") ||
		(params.Context != nil && *params.Context != "") ||
		(params.Status != nil && *params.Status != "") {
		s.SearchDeployments(c, string(api.WebSubApi))
		return
	}

	configs, err := s.db.GetAllConfigsByKind(string(api.WebSubApi))
	if err != nil {
		s.logger.Error("Failed to list WebSub APIs", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to list WebSub API configurations",
		})
		return
	}

	items := make([]api.WebSubAPIListItem, 0, len(configs))
	for _, cfg := range configs {
		cfgContext, err := cfg.GetContext()
		if err != nil {
			s.logger.Warn("Failed to get context for WebSub API config",
				slog.String("id", cfg.UUID),
				slog.Any("error", err))
			continue
		}

		status := string(cfg.DesiredState)
		items = append(items, api.WebSubAPIListItem{
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
		"count":      len(items),
		"websubApis": items,
	})
}

// GetWebSubAPIById implements ServerInterface.GetWebSubAPIById
// (GET /websub-apis/{id})
func (s *APIServer) GetWebSubAPIById(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			c.JSON(http.StatusServiceUnavailable, api.ErrorResponse{
				Status:  "error",
				Message: "Database storage not available",
			})
			return
		}
		log.Warn("WebSub API configuration not found",
			slog.String("handle", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("WebSub API configuration with handle '%s' not found", handle),
		})
		return
	}

	apiDetail := gin.H{
		"id":            cfg.Handle,
		"configuration": cfg.SourceConfiguration,
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

// UpdateWebSubAPI implements ServerInterface.UpdateWebSubAPI
// (PUT /websub-apis/{id})
func (s *APIServer) UpdateWebSubAPI(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	existing, err := s.db.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		log.Warn("WebSub API configuration not found",
			slog.String("handle", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("WebSub API configuration with handle '%s' not found", handle),
		})
		return
	}

	correlationID := middleware.GetCorrelationID(c)

	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		Kind:          "WebSubApi",
		APIID:         existing.UUID,
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to update WebSub API configuration", slog.Any("error", err))
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

	updated := result.StoredConfig

	log.Info("WebSub API configuration updated",
		slog.String("id", updated.UUID),
		slog.String("handle", handle))

	c.JSON(http.StatusOK, api.WebSubAPIUpdateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("WebSub API configuration updated successfully"),
		Id:        stringPtr(updated.Handle),
		UpdatedAt: timePtr(updated.UpdatedAt),
	})
}

// DeleteWebSubAPI implements ServerInterface.DeleteWebSubAPI
// (DELETE /websub-apis/{id})
func (s *APIServer) DeleteWebSubAPI(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		log.Warn("WebSub API configuration not found",
			slog.String("handle", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("WebSub API configuration with handle '%s' not found", handle),
		})
		return
	}

	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		log.Error("Failed to delete WebSub API config from database", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete configuration",
		})
		return
	}

	if err := s.db.RemoveAPIKeysAPI(cfg.UUID); err != nil {
		log.Warn("Failed to remove API keys from database",
			slog.String("handle", handle),
			slog.Any("error", err))
	}

	// Deregister WebSub topics
	topicsToUnregister := s.deploymentService.GetTopicsForDelete(*cfg)
	for _, topic := range topicsToUnregister {
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(s.routerConfig.EventGateway.TimeoutSeconds)*time.Second)
		if err := s.deploymentService.UnregisterTopicWithHub(ctx, s.httpClient, topic,
			s.routerConfig.EventGateway.RouterHost, s.routerConfig.EventGateway.WebSubHubListenerPort, log); err != nil {
			log.Error("Failed to deregister topic from WebSubHub",
				slog.Any("error", err),
				slog.String("topic", topic))
		} else {
			log.Info("Successfully deregistered topic from WebSubHub",
				slog.String("topic", topic))
		}
		cancel()
	}

	s.publishWebSubEvent(eventhub.EventTypeAPI, "DELETE", cfg.UUID, correlationID, log)

	log.Info("WebSub API configuration deleted",
		slog.String("id", cfg.UUID),
		slog.String("handle", handle))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "WebSub API configuration deleted successfully",
		"id":      handle,
	})
}

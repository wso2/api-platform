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
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// CreateWebBrokerApi handles POST /webbroker-apis
func (s *APIServer) CreateWebBrokerApi(c *gin.Context) {
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
		Kind:          "WebBrokerApi",
		APIID:         "",
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to deploy WebBrokerApi configuration", slog.Any("error", err))
		if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		if mapRenderError(c, "create", err) {
			return
		}
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	cfg := result.StoredConfig

	c.JSON(http.StatusCreated, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))

	if result.IsStale {
		return
	}

	if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() && s.systemConfig.Controller.ControlPlane.DeploymentPushEnabled {
		go s.waitForDeploymentAndPush(cfg.UUID, correlationID, log)
	}
}

// ListWebBrokerApis handles GET /webbroker-apis
func (s *APIServer) ListWebBrokerApis(c *gin.Context, params api.ListWebBrokerApisParams) {
	configs, err := s.db.GetAllConfigsByKind(string(models.KindWebBrokerApi))
	if err != nil {
		s.logger.Error("Failed to list WebBrokerApis", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to list WebBrokerApi configurations",
		})
		return
	}

	// TODO: Implement query parameter filtering (displayName, version, status)
	// For now, returning all WebBrokerApis without filtering
	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		items = append(items, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"count":         len(items),
		"webBrokerApis": items,
	})
}

// GetWebBrokerApiById handles GET /webbroker-apis/{id}
func (s *APIServer) GetWebBrokerApiById(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindWebBrokerApi, handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			log.Error("Database unavailable", slog.Any("error", err))
			c.JSON(http.StatusServiceUnavailable, api.ErrorResponse{
				Status:  "error",
				Message: "Database is temporarily unavailable",
			})
			return
		}
		log.Warn("WebBrokerApi not found", slog.String("handle", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: "WebBrokerApi not found",
		})
		return
	}

	c.JSON(http.StatusOK, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
}

// DeleteWebBrokerApiById handles DELETE /webbroker-apis/{id}
func (s *APIServer) DeleteWebBrokerApiById(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindWebBrokerApi, handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			log.Error("Database unavailable", slog.Any("error", err))
			c.JSON(http.StatusServiceUnavailable, api.ErrorResponse{
				Status:  "error",
				Message: "Database is temporarily unavailable",
			})
			return
		}
		log.Warn("WebBrokerApi not found", slog.String("handle", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: "WebBrokerApi not found",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(c)

	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		log.Error("Failed to delete WebBrokerApi from database", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete configuration",
		})
		return
	}

	// Publish delete event for xDS propagation
	s.publishWebSubEvent(eventhub.EventTypeAPI, "DELETE", cfg.UUID, correlationID, log)

	log.Info("WebBrokerApi deleted successfully",
		slog.String("id", id),
		slog.String("correlation_id", correlationID))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "WebBrokerApi deleted successfully",
	})
}

// CreateWebBrokerAPIKey implements ServerInterface.CreateWebBrokerAPIKey
// (POST /webbroker-apis/{id}/api-keys)
func (s *APIServer) CreateWebBrokerAPIKey(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "CreateWebBrokerAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		log.Error("Failed to parse request body for WebBroker API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyCreationParams{
		Kind:          models.KindWebBrokerApi,
		Handle:        handle,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.CreateAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API '%s' not found", handle)})
		} else if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to create WebBroker API key", slog.String("handle", handle), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create API key"})
		}
		return
	}

	c.JSON(http.StatusCreated, result.Response)
}

// ListWebBrokerAPIKeys implements ServerInterface.ListWebBrokerAPIKeys
// (GET /webbroker-apis/{id}/api-keys)
func (s *APIServer) ListWebBrokerAPIKeys(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "ListWebBrokerAPIKeys", correlationID)
	if !ok {
		return
	}

	params := utils.ListAPIKeyParams{
		Kind:          models.KindWebBrokerApi,
		Handle:        handle,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.ListAPIKeys(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API '%s' not found", handle)})
		} else {
			log.Error("Failed to list WebBroker API keys", slog.String("handle", handle), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list API keys"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// RevokeWebBrokerAPIKey implements ServerInterface.RevokeWebBrokerAPIKey
// (DELETE /webbroker-apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) RevokeWebBrokerAPIKey(c *gin.Context, id string, apiKeyName string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "RevokeWebBrokerAPIKey", correlationID)
	if !ok {
		return
	}

	params := utils.APIKeyRevocationParams{
		Kind:          models.KindWebBrokerApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RevokeAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API '%s' not found", handle)})
		} else {
			log.Error("Failed to revoke WebBroker API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to revoke API key"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// UpdateWebBrokerAPIKey implements ServerInterface.UpdateWebBrokerAPIKey
// (PUT /webbroker-apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) UpdateWebBrokerAPIKey(c *gin.Context, id string, apiKeyName string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "UpdateWebBrokerAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	if request.ApiKey == nil || strings.TrimSpace(*request.ApiKey) == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "apiKey is required"})
		return
	}

	params := utils.APIKeyUpdateParams{
		Kind:          models.KindWebBrokerApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.UpdateAPIKey(params)
	if err != nil {
		if storage.IsOperationNotAllowedError(err) {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API or API key '%s' not found", apiKeyName)})
		} else if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to update WebBroker API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update API key"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// RegenerateWebBrokerAPIKey implements ServerInterface.RegenerateWebBrokerAPIKey
// (POST /webbroker-apis/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RegenerateWebBrokerAPIKey(c *gin.Context, id string, apiKeyName string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "RegenerateWebBrokerAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyRegenerationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyRegenerationParams{
		Kind:          models.KindWebBrokerApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RegenerateAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API or API key '%s' not found", apiKeyName)})
		} else {
			log.Error("Failed to regenerate WebBroker API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to regenerate API key"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

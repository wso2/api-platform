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
	gwapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// CreateWebBrokerAPI handles POST /webbroker-apis
func (s *EventAPIServer) CreateWebBrokerAPI(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, gwapi.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(c)

	result, err := s.svc.APIDeploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
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
			c.JSON(http.StatusConflict, gwapi.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gwapi.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	cfg := result.StoredConfig

	c.JSON(http.StatusCreated, buildResourceResponse(cfg.SourceConfiguration, cfg))

	// TODO: implement waitForDeploymentAndPush for control plane push support
}

// ListWebBrokerAPIs handles GET /webbroker-apis
func (s *EventAPIServer) ListWebBrokerAPIs(c *gin.Context) {
	configs, err := s.svc.Storage.GetAllConfigsByKind(string(models.KindWebBrokerApi))
	if err != nil {
		s.svc.Logger.Error("Failed to list WebBrokerApis", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{
			Status:  "error",
			Message: "Failed to list WebBrokerApi configurations",
		})
		return
	}

	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		items = append(items, buildResourceResponse(cfg.SourceConfiguration, cfg))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"count":         len(items),
		"webBrokerApis": items,
	})
}

// GetWebBrokerAPIById handles GET /webbroker-apis/:id
func (s *EventAPIServer) GetWebBrokerAPIById(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")

	cfg, err := s.svc.Storage.GetConfigByKindAndHandle(models.KindWebBrokerApi, handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			log.Error("Database unavailable", slog.Any("error", err))
			c.JSON(http.StatusServiceUnavailable, gwapi.ErrorResponse{
				Status:  "error",
				Message: "Database is temporarily unavailable",
			})
			return
		}
		log.Warn("WebBrokerApi not found", slog.String("handle", handle))
		c.JSON(http.StatusNotFound, gwapi.ErrorResponse{
			Status:  "error",
			Message: "WebBrokerApi not found",
		})
		return
	}

	c.JSON(http.StatusOK, buildResourceResponse(cfg.SourceConfiguration, cfg))
}

// DeleteWebBrokerAPI handles DELETE /webbroker-apis/:id
func (s *EventAPIServer) DeleteWebBrokerAPI(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")

	cfg, err := s.svc.Storage.GetConfigByKindAndHandle(models.KindWebBrokerApi, handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			log.Error("Database unavailable", slog.Any("error", err))
			c.JSON(http.StatusServiceUnavailable, gwapi.ErrorResponse{
				Status:  "error",
				Message: "Database is temporarily unavailable",
			})
			return
		}
		log.Warn("WebBrokerApi not found", slog.String("handle", handle))
		c.JSON(http.StatusNotFound, gwapi.ErrorResponse{
			Status:  "error",
			Message: "WebBrokerApi not found",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(c)

	if err := s.svc.Storage.DeleteConfig(cfg.UUID); err != nil {
		log.Error("Failed to delete WebBrokerApi from database", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete configuration",
		})
		return
	}

	s.publishEvent(eventhub.EventTypeAPI, "DELETE", cfg.UUID, correlationID, log)

	log.Info("WebBrokerApi deleted successfully",
		slog.String("id", handle),
		slog.String("correlation_id", correlationID))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "WebBrokerApi deleted successfully",
	})
}

// CreateWebBrokerAPIKey handles POST /webbroker-apis/:id/api-keys
func (s *EventAPIServer) CreateWebBrokerAPIKey(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "CreateWebBrokerAPIKey", correlationID)
	if !ok {
		return
	}

	var request gwapi.APIKeyCreationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		log.Error("Failed to parse request body for WebBroker API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
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
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API '%s' not found", handle)})
		} else if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, gwapi.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to create WebBroker API key", slog.String("handle", handle), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to create API key"})
		}
		return
	}

	c.JSON(http.StatusCreated, result.Response)
}

// ListWebBrokerAPIKeys handles GET /webbroker-apis/:id/api-keys
func (s *EventAPIServer) ListWebBrokerAPIKeys(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
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
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API '%s' not found", handle)})
		} else {
			log.Error("Failed to list WebBroker API keys", slog.String("handle", handle), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to list API keys"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// RevokeWebBrokerAPIKey handles DELETE /webbroker-apis/:id/api-keys/:apiKeyName
func (s *EventAPIServer) RevokeWebBrokerAPIKey(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	apiKeyName := c.Param("apiKeyName")
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
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API '%s' not found", handle)})
		} else {
			log.Error("Failed to revoke WebBroker API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to revoke API key"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// UpdateWebBrokerAPIKey handles PUT /webbroker-apis/:id/api-keys/:apiKeyName
func (s *EventAPIServer) UpdateWebBrokerAPIKey(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	apiKeyName := c.Param("apiKeyName")
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "UpdateWebBrokerAPIKey", correlationID)
	if !ok {
		return
	}

	var request gwapi.APIKeyCreationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	if request.ApiKey == nil || strings.TrimSpace(*request.ApiKey) == "" {
		c.JSON(http.StatusBadRequest, gwapi.ErrorResponse{Status: "error", Message: "apiKey is required"})
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
			c.JSON(http.StatusBadRequest, gwapi.ErrorResponse{Status: "error", Message: err.Error()})
		} else if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API or API key '%s' not found", apiKeyName)})
		} else if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, gwapi.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to update WebBroker API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to update API key"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// RegenerateWebBrokerAPIKey handles POST /webbroker-apis/:id/api-keys/:apiKeyName/regenerate
func (s *EventAPIServer) RegenerateWebBrokerAPIKey(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	apiKeyName := c.Param("apiKeyName")
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "RegenerateWebBrokerAPIKey", correlationID)
	if !ok {
		return
	}

	var request gwapi.APIKeyRegenerationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
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
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API or API key '%s' not found", apiKeyName)})
		} else {
			log.Error("Failed to regenerate WebBroker API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to regenerate API key"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

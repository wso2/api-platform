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
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/eventhub"
	gwapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// CreateWebSubAPI handles POST /websub-apis
func (s *EventAPIServer) CreateWebSubAPI(c *gin.Context) {
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
		Kind:          "WebSubApi",
		APIID:         "",
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to deploy WebSub API configuration", slog.Any("error", err))
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

// ListWebSubAPIs handles GET /websub-apis
func (s *EventAPIServer) ListWebSubAPIs(c *gin.Context) {
	displayName := c.Query("displayName")
	version := c.Query("version")
	context_ := c.Query("context")
	status := c.Query("status")

	if displayName != "" || version != "" || context_ != "" || status != "" {
		s.searchWebSubAPIs(c)
		return
	}

	configs, err := s.svc.Storage.GetAllConfigsByKind(string(gwapi.WebSubAPIKindWebSubApi))
	if err != nil {
		s.svc.Logger.Error("Failed to list WebSub APIs", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{
			Status:  "error",
			Message: "Failed to list WebSub API configurations",
		})
		return
	}

	contextFilter := context_ != ""
	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		_, err := cfg.GetContext()
		if err != nil {
			s.svc.Logger.Warn("Failed to get context for WebSub API config",
				slog.String("id", cfg.UUID),
				slog.String("displayName", cfg.DisplayName),
				slog.Any("error", err))
			if contextFilter {
				continue
			}
		}
		items = append(items, buildResourceResponse(cfg.SourceConfiguration, cfg))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"count":      len(items),
		"websubApis": items,
	})
}

// searchWebSubAPIs handles filtered listing of WebSub APIs.
func (s *EventAPIServer) searchWebSubAPIs(c *gin.Context) {
	displayName := c.Query("displayName")
	version := c.Query("version")
	context_ := c.Query("context")
	status := c.Query("status")

	configs, err := s.svc.Storage.GetAllConfigsByKind(string(gwapi.WebSubAPIKindWebSubApi))
	if err != nil {
		s.svc.Logger.Error("Failed to search WebSub APIs", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{
			Status:  "error",
			Message: "Failed to search WebSub API configurations",
		})
		return
	}

	items := make([]any, 0)
	for _, cfg := range configs {
		if displayName != "" && !strings.Contains(strings.ToLower(cfg.DisplayName), strings.ToLower(displayName)) {
			continue
		}
		if version != "" && cfg.Version != version {
			continue
		}
		if status != "" && string(cfg.DesiredState) != status {
			continue
		}
		if context_ != "" {
			ctx, err := cfg.GetContext()
			if err != nil || ctx != context_ {
				continue
			}
		}
		items = append(items, buildResourceResponse(cfg.SourceConfiguration, cfg))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"count":      len(items),
		"websubApis": items,
	})
}

// GetWebSubAPIById handles GET /websub-apis/:id
func (s *EventAPIServer) GetWebSubAPIById(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")

	cfg, err := s.svc.Storage.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			c.JSON(http.StatusServiceUnavailable, gwapi.ErrorResponse{
				Status:  "error",
				Message: "Database storage not available",
			})
			return
		}
		log.Warn("WebSub API configuration not found",
			slog.String("handle", handle))
		c.JSON(http.StatusNotFound, gwapi.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("WebSub API configuration with handle '%s' not found", handle),
		})
		return
	}

	c.JSON(http.StatusOK, buildResourceResponse(cfg.SourceConfiguration, cfg))
}

// UpdateWebSubAPI handles PUT /websub-apis/:id
func (s *EventAPIServer) UpdateWebSubAPI(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, gwapi.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	existing, err := s.svc.Storage.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		log.Warn("WebSub API configuration not found",
			slog.String("handle", handle))
		c.JSON(http.StatusNotFound, gwapi.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("WebSub API configuration with handle '%s' not found", handle),
		})
		return
	}

	correlationID := middleware.GetCorrelationID(c)

	result, err := s.svc.APIDeploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
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

	updated := result.StoredConfig

	log.Info("WebSub API configuration updated",
		slog.String("id", updated.UUID),
		slog.String("handle", handle))

	c.JSON(http.StatusOK, buildResourceResponse(updated.SourceConfiguration, updated))
}

// DeleteWebSubAPI handles DELETE /websub-apis/:id
func (s *EventAPIServer) DeleteWebSubAPI(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	correlationID := middleware.GetCorrelationID(c)

	cfg, err := s.svc.Storage.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		log.Warn("WebSub API configuration not found",
			slog.String("handle", handle))
		c.JSON(http.StatusNotFound, gwapi.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("WebSub API configuration with handle '%s' not found", handle),
		})
		return
	}

	if err := s.svc.Storage.DeleteConfig(cfg.UUID); err != nil {
		log.Error("Failed to delete WebSub API config from database", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete configuration",
		})
		return
	}

	if err := s.svc.Storage.RemoveAPIKeysAPI(cfg.UUID); err != nil {
		log.Warn("Failed to remove API keys from database",
			slog.String("handle", handle),
			slog.Any("error", err))
	}

	topicsToUnregister := s.svc.APIDeploymentService.GetTopicsForDelete(*cfg)
	for _, topic := range topicsToUnregister {
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(s.svc.RouterConfig.EventGateway.TimeoutSeconds)*time.Second)
		if err := s.svc.APIDeploymentService.UnregisterTopicWithHub(ctx, s.httpClient, topic,
			s.svc.RouterConfig.EventGateway.RouterHost, s.svc.RouterConfig.EventGateway.WebSubHubListenerPort, log); err != nil {
			log.Error("Failed to deregister topic from WebSubHub",
				slog.Any("error", err),
				slog.String("topic", topic))
		} else {
			log.Info("Successfully deregistered topic from WebSubHub",
				slog.String("topic", topic))
		}
		cancel()
	}

	s.publishEvent(eventhub.EventTypeAPI, "DELETE", cfg.UUID, correlationID, log)

	log.Info("WebSub API configuration deleted",
		slog.String("id", cfg.UUID),
		slog.String("handle", handle))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "WebSub API configuration deleted successfully",
		"id":      handle,
	})
}

// CreateWebSubAPIKey handles POST /websub-apis/:id/api-keys
func (s *EventAPIServer) CreateWebSubAPIKey(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "CreateWebSubAPIKey", correlationID)
	if !ok {
		return
	}

	var request gwapi.APIKeyCreationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		log.Error("Failed to parse request body for WebSub API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyCreationParams{
		Kind:          models.KindWebSubApi,
		Handle:        handle,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.CreateAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		} else if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, gwapi.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to create WebSub API key", slog.String("handle", handle), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to create API key"})
		}
		return
	}

	c.JSON(http.StatusCreated, result.Response)
}

// ListWebSubAPIKeys handles GET /websub-apis/:id/api-keys
func (s *EventAPIServer) ListWebSubAPIKeys(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "ListWebSubAPIKeys", correlationID)
	if !ok {
		return
	}

	params := utils.ListAPIKeyParams{
		Kind:          models.KindWebSubApi,
		Handle:        handle,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.ListAPIKeys(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		} else {
			log.Error("Failed to list WebSub API keys", slog.String("handle", handle), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to list API keys"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// RevokeWebSubAPIKey handles DELETE /websub-apis/:id/api-keys/:apiKeyName
func (s *EventAPIServer) RevokeWebSubAPIKey(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	apiKeyName := c.Param("apiKeyName")
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "RevokeWebSubAPIKey", correlationID)
	if !ok {
		return
	}

	params := utils.APIKeyRevocationParams{
		Kind:          models.KindWebSubApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RevokeAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		} else {
			log.Error("Failed to revoke WebSub API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to revoke API key"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// UpdateWebSubAPIKey handles PUT /websub-apis/:id/api-keys/:apiKeyName
func (s *EventAPIServer) UpdateWebSubAPIKey(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	apiKeyName := c.Param("apiKeyName")
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "UpdateWebSubAPIKey", correlationID)
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
		Kind:          models.KindWebSubApi,
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
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API or API key '%s' not found", apiKeyName)})
		} else if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, gwapi.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to update WebSub API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to update API key"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// RegenerateWebSubAPIKey handles POST /websub-apis/:id/api-keys/:apiKeyName/regenerate
func (s *EventAPIServer) RegenerateWebSubAPIKey(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	apiKeyName := c.Param("apiKeyName")
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "RegenerateWebSubAPIKey", correlationID)
	if !ok {
		return
	}

	var request gwapi.APIKeyRegenerationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyRegenerationParams{
		Kind:          models.KindWebSubApi,
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
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API or API key '%s' not found", apiKeyName)})
		} else {
			log.Error("Failed to regenerate WebSub API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to regenerate API key"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// CreateWebSubAPISecret handles POST /websub-apis/:id/secrets
func (s *EventAPIServer) CreateWebSubAPISecret(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	correlationID := middleware.GetCorrelationID(c)

	cfg, err := s.svc.Storage.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
			return
		}
		if storage.IsDatabaseUnavailableError(err) {
			c.JSON(http.StatusServiceUnavailable, gwapi.ErrorResponse{Status: "error", Message: "Storage unavailable"})
			return
		}
		log.Error("Failed to look up WebSub API", slog.String("handle", handle), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Internal error looking up WebSub API"})
		return
	}
	if cfg == nil {
		c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		return
	}

	var request gwapi.WebhookSecretCreationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %s", err.Error())})
		return
	}

	ws, plaintext, err := s.webhookSecretService.Generate(cfg.UUID, request.DisplayName, correlationID)
	if err != nil {
		if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, gwapi.ErrorResponse{Status: "error", Message: err.Error()})
			return
		}
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: err.Error()})
			return
		}
		log.Error("Failed to generate webhook secret", slog.String("handle", handle), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to generate webhook secret"})
		return
	}

	secretStatus := gwapi.WebhookSecretInfoStatus(ws.Status)
	c.JSON(http.StatusCreated, gwapi.WebhookSecretCreationResponse{
		Status:  "success",
		Message: "Webhook secret generated successfully",
		Secret:  plaintext,
		WebhookSecret: &gwapi.WebhookSecretInfo{
			Name:        &ws.Name,
			DisplayName: &ws.DisplayName,
			Status:      &secretStatus,
			CreatedAt:   &ws.CreatedAt,
			UpdatedAt:   &ws.UpdatedAt,
		},
	})
}

// ListWebSubAPISecrets handles GET /websub-apis/:id/secrets
func (s *EventAPIServer) ListWebSubAPISecrets(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")

	cfg, err := s.svc.Storage.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
			return
		}
		if storage.IsDatabaseUnavailableError(err) {
			c.JSON(http.StatusServiceUnavailable, gwapi.ErrorResponse{Status: "error", Message: "Storage unavailable"})
			return
		}
		log.Error("Failed to look up WebSub API", slog.String("handle", handle), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Internal error looking up WebSub API"})
		return
	}
	if cfg == nil {
		c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		return
	}

	wsList, err := s.webhookSecretService.List(cfg.UUID)
	if err != nil {
		log.Error("Failed to list webhook secrets", slog.String("handle", handle), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to list webhook secrets"})
		return
	}

	items := make([]gwapi.WebhookSecretInfo, 0, len(wsList))
	for _, ws := range wsList {
		secretStatus := gwapi.WebhookSecretInfoStatus(ws.Status)
		items = append(items, gwapi.WebhookSecretInfo{
			Name:        &ws.Name,
			DisplayName: &ws.DisplayName,
			Status:      &secretStatus,
			CreatedAt:   &ws.CreatedAt,
			UpdatedAt:   &ws.UpdatedAt,
		})
	}

	total := len(items)
	status := "success"
	c.JSON(http.StatusOK, gwapi.WebhookSecretListResponse{
		Status:     &status,
		TotalCount: &total,
		Secrets:    &items,
	})
}

// RegenerateWebSubAPISecret handles POST /websub-apis/:id/secrets/:secretName/regenerate
func (s *EventAPIServer) RegenerateWebSubAPISecret(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	secretName := c.Param("secretName")
	correlationID := middleware.GetCorrelationID(c)

	cfg, err := s.svc.Storage.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
			return
		}
		if storage.IsDatabaseUnavailableError(err) {
			c.JSON(http.StatusServiceUnavailable, gwapi.ErrorResponse{Status: "error", Message: "Storage unavailable"})
			return
		}
		log.Error("Failed to look up WebSub API", slog.String("handle", handle), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Internal error looking up WebSub API"})
		return
	}
	if cfg == nil {
		c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		return
	}

	ws, plaintext, err := s.webhookSecretService.Regenerate(cfg.UUID, secretName, correlationID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("Webhook secret '%s' not found", secretName)})
			return
		}
		log.Error("Failed to regenerate webhook secret", slog.String("handle", handle), slog.String("secret", secretName), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to regenerate webhook secret"})
		return
	}

	secretStatus := gwapi.WebhookSecretInfoStatus(ws.Status)
	c.JSON(http.StatusOK, gwapi.WebhookSecretCreationResponse{
		Status:  "success",
		Message: "Webhook secret regenerated successfully",
		Secret:  plaintext,
		WebhookSecret: &gwapi.WebhookSecretInfo{
			Name:        &ws.Name,
			DisplayName: &ws.DisplayName,
			Status:      &secretStatus,
			CreatedAt:   &ws.CreatedAt,
			UpdatedAt:   &ws.UpdatedAt,
		},
	})
}

// DeleteWebSubAPISecret handles DELETE /websub-apis/:id/secrets/:secretName
func (s *EventAPIServer) DeleteWebSubAPISecret(c *gin.Context) {
	log := middleware.GetLogger(c, s.svc.Logger)
	handle := c.Param("id")
	secretName := c.Param("secretName")
	correlationID := middleware.GetCorrelationID(c)

	cfg, err := s.svc.Storage.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
			return
		}
		if storage.IsDatabaseUnavailableError(err) {
			c.JSON(http.StatusServiceUnavailable, gwapi.ErrorResponse{Status: "error", Message: "Storage unavailable"})
			return
		}
		log.Error("Failed to look up WebSub API", slog.String("handle", handle), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Internal error looking up WebSub API"})
		return
	}
	if cfg == nil {
		c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		return
	}

	if err := s.webhookSecretService.Delete(cfg.UUID, secretName, correlationID); err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, gwapi.ErrorResponse{Status: "error", Message: fmt.Sprintf("Webhook secret '%s' not found", secretName)})
			return
		}
		log.Error("Failed to delete webhook secret", slog.String("handle", handle), slog.String("secret", secretName), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gwapi.ErrorResponse{Status: "error", Message: "Failed to delete webhook secret"})
		return
	}

	c.Status(http.StatusNoContent)
}

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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// ListLLMProxies implements ServerInterface.ListLLMProxies
// (GET /llm-proxies)
func (s *APIServer) ListLLMProxies(c *gin.Context, params api.ListLLMProxiesParams) {
	log := middleware.GetLogger(c, s.logger)
	configs := s.llmDeploymentService.ListLLMProxies(params)

	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		// Re-materialise SourceConfiguration into a typed LLMProxyConfiguration
		// so each list item has a strongly-typed k8s-shaped body with status.
		proxy, err := rematerializeLLMProxyConfig(log, cfg.UUID, cfg.DisplayName, cfg.SourceConfiguration)
		if err != nil {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status: "error", Message: "Failed to get stored LLM proxy configuration"})
			return
		}

		items = append(items, buildResourceResponseFromStored(proxy, cfg))
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
		if mapRenderError(c, "create", err) {
			return
		}
		if utils.IsPolicyDefinitionMissingError(err) {
			log.Error("Failed to create LLM proxy - policy definition missing", slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: utils.PolicyDefinitionMissingUserMessage,
			})
			return
		}
		if errors.Is(err, utils.ErrLLMProxyValidation) {
			log.Warn("LLM proxy configuration invalid", slog.Any("error", err))
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
			return
		}
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: "LLM proxy configuration not found",
			})
			return
		}
		if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
			return
		}
		log.Error("Failed to create LLM proxy", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create LLM proxy"})
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

	proxy, err := rematerializeLLMProxyConfig(log, stored.UUID, stored.DisplayName, stored.SourceConfiguration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: "Failed to get stored LLM proxy configuration"})
		return
	}

	c.JSON(http.StatusCreated, buildResourceResponseFromStored(proxy, stored))
}

// GetLLMProxyById implements ServerInterface.GetLLMProxyById
// (GET /llm-proxies/{id})
func (s *APIServer) GetLLMProxyById(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)

	cfg, err := s.llmDeploymentService.GetLLMProxyByHandle(id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			log.Warn("LLM proxy configuration not found", slog.String("handle", id))
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("LLM proxy configuration with handle '%s' not found", id),
			})
			return
		}
		log.Error("Failed to look up LLM proxy", slog.String("handle", id), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to look up LLM proxy",
		})
		return
	}

	proxy, err := rematerializeLLMProxyConfig(log, cfg.UUID, cfg.DisplayName, cfg.SourceConfiguration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: "Failed to get stored LLM proxy configuration"})
		return
	}

	c.JSON(http.StatusOK, buildResourceResponseFromStored(proxy, cfg))
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
		if mapRenderError(c, "update", err) {
			return
		}
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("LLM proxy configuration with handle '%s' not found", id),
			})
			return
		}
		if utils.IsPolicyDefinitionMissingError(err) {
			log.Error("Failed to update LLM proxy - policy definition missing", slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: utils.PolicyDefinitionMissingUserMessage,
			})
			return
		}
		if errors.Is(err, utils.ErrLLMProxyValidation) {
			log.Warn("LLM proxy configuration invalid", slog.Any("error", err))
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
			return
		}
		log.Error("Failed to update LLM proxy configuration", slog.String("id", id), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update LLM proxy configuration"})
		return
	}

	updated := result.StoredConfig

	proxy, err := rematerializeLLMProxyConfig(log, updated.UUID, updated.DisplayName, updated.SourceConfiguration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: "Failed to get stored LLM proxy configuration"})
		return
	}

	c.JSON(http.StatusOK, buildResourceResponseFromStored(proxy, updated))
}

// DeleteLLMProxy implements ServerInterface.DeleteLLMProxy
// (DELETE /llm-proxies/{id})
func (s *APIServer) DeleteLLMProxy(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)

	cfg, err := s.llmDeploymentService.DeleteLLMProxy(id, correlationID, log)
	if err != nil {
		if storage.IsNotFoundError(err) {
			log.Warn("LLM proxy configuration not found for deletion", slog.String("handle", id))
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("LLM proxy configuration with handle '%s' not found", id),
			})
			return
		}
		log.Error("Failed to delete LLM proxy configuration", slog.String("handle", id), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete LLM proxy configuration",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "LLM proxy deleted successfully",
		"id":      cfg.Handle,
	})

}

// CreateLLMProxyAPIKey implements ServerInterface.CreateLLMProxyAPIKey
// (POST /llm-proxies/{id}/api-keys)
func (s *APIServer) CreateLLMProxyAPIKey(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "CreateLLMProxyAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		log.Error("Failed to parse request body for LLM proxy API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyCreationParams{
		Kind:          models.KindLlmProxy,
		Handle:        handle,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.CreateAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("LLM proxy '%s' not found", handle)})
		} else if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to create LLM proxy API key", slog.String("handle", handle), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create API key"})
		}
		return
	}

	c.JSON(http.StatusCreated, result.Response)
}

// RevokeLLMProxyAPIKey implements ServerInterface.RevokeLLMProxyAPIKey
// (DELETE /llm-proxies/{id}/api-keys/{apiKeyName})
func (s *APIServer) RevokeLLMProxyAPIKey(c *gin.Context, id string, apiKeyName string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "RevokeLLMProxyAPIKey", correlationID)
	if !ok {
		return
	}

	params := utils.APIKeyRevocationParams{
		Kind:          models.KindLlmProxy,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RevokeAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("LLM proxy '%s' not found", handle)})
		} else {
			log.Error("Failed to revoke LLM proxy API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to revoke API key"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// UpdateLLMProxyAPIKey implements ServerInterface.UpdateLLMProxyAPIKey
// (PUT /llm-proxies/{id}/api-keys/{apiKeyName})
func (s *APIServer) UpdateLLMProxyAPIKey(c *gin.Context, id string, apiKeyName string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "UpdateLLMProxyAPIKey", correlationID)
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
		Kind:          models.KindLlmProxy,
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
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("LLM proxy or API key '%s' not found", apiKeyName)})
		} else if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to update LLM proxy API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update API key"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// RegenerateLLMProxyAPIKey implements ServerInterface.RegenerateLLMProxyAPIKey
// (POST /llm-proxies/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RegenerateLLMProxyAPIKey(c *gin.Context, id string, apiKeyName string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "RegenerateLLMProxyAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyRegenerationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyRegenerationParams{
		Kind:          models.KindLlmProxy,
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
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("LLM proxy or API key '%s' not found", apiKeyName)})
		} else {
			log.Error("Failed to regenerate LLM proxy API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to regenerate API key"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// ListLLMProxyAPIKeys implements ServerInterface.ListLLMProxyAPIKeys
// (GET /llm-proxies/{id}/api-keys)
func (s *APIServer) ListLLMProxyAPIKeys(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "ListLLMProxyAPIKeys", correlationID)
	if !ok {
		return
	}

	params := utils.ListAPIKeyParams{
		Kind:          models.KindLlmProxy,
		Handle:        handle,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.ListAPIKeys(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("LLM proxy '%s' not found", handle)})
		} else {
			log.Error("Failed to list LLM proxy API keys", slog.String("handle", handle), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list API keys"})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// rematerializeLLMProxyConfig re-encodes persisted SourceConfiguration into the
// generated API type. Logs marshal/unmarshal failures with full context; callers
// return 500.
func rematerializeLLMProxyConfig(log *slog.Logger, id, displayName string, source any) (api.LLMProxyConfiguration, error) {
	j, err := json.Marshal(source)
	if err != nil {
		log.Error("Failed to marshal stored LLM proxy source configuration",
			slog.String("id", id),
			slog.String("displayName", displayName),
			slog.Any("sourceConfiguration", source),
			slog.Any("error", err))
		return api.LLMProxyConfiguration{}, fmt.Errorf("marshal LLM proxy config: %w", err)
	}
	var proxy api.LLMProxyConfiguration
	if err := json.Unmarshal(j, &proxy); err != nil {
		log.Error("Failed to unmarshal stored LLM proxy configuration",
			slog.String("id", id),
			slog.String("displayName", displayName),
			slog.Any("sourceConfiguration", source),
			slog.Any("error", err))
		return api.LLMProxyConfiguration{}, fmt.Errorf("unmarshal LLM proxy config: %w", err)
	}
	return proxy, nil
}

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

	"github.com/gin-gonic/gin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// ListLLMProviders implements ServerInterface.ListLLMProviders
// (GET /llm-providers)
func (s *APIServer) ListLLMProviders(c *gin.Context, params api.ListLLMProvidersParams) {
	log := middleware.GetLogger(c, s.logger)
	configs := s.llmDeploymentService.ListLLMProviders(params)

	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		// Re-materialise SourceConfiguration into a typed LLMProviderConfiguration
		// so each list item has a strongly-typed k8s-shaped body with status.
		prov, err := rematerializeLLMProviderConfig(log, cfg.UUID, cfg.DisplayName, cfg.SourceConfiguration)
		if err != nil {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error",
				Message: "Failed to get stored LLM provider configuration"})
			return
		}

		items = append(items, buildResourceResponseFromStored(prov, cfg))
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
		if mapRenderError(c, "create", err) {
			return
		}
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

	// Re-materialise stored source config into a typed LLMProviderConfiguration
	// so the response is a k8s-shaped body (server-managed Status is injected by
	// buildResourceResponseFromStored).
	prov, err := rematerializeLLMProviderConfig(log, stored.UUID, stored.DisplayName, stored.SourceConfiguration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to get stored LLM provider configuration",
		})
		return
	}

	c.JSON(http.StatusCreated, buildResourceResponseFromStored(prov, stored))
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

	// Re-materialise the stored source configuration into a typed
	// LLMProviderConfiguration so we can attach server-managed status fields.
	prov, err := rematerializeLLMProviderConfig(log, cfg.UUID, cfg.DisplayName, cfg.SourceConfiguration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to get stored LLM provider configuration",
		})
		return
	}

	c.JSON(http.StatusOK, buildResourceResponseFromStored(prov, cfg))
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
		if mapRenderError(c, "update", err) {
			return
		}
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

	prov, err := rematerializeLLMProviderConfig(log, updated.UUID, updated.DisplayName, updated.SourceConfiguration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to get stored LLM provider configuration",
		})
		return
	}

	c.JSON(http.StatusOK, buildResourceResponseFromStored(prov, updated))
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

// CreateLLMProviderAPIKey implements ServerInterface.CreateLLMProviderAPIKey
// (POST /llm-providers/{id}/api-keys)
func (s *APIServer) CreateLLMProviderAPIKey(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "CreateLLMProviderAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		log.Error("Failed to parse request body for LLM provider API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyCreationParams{
		Kind:          models.KindLlmProvider,
		Handle:        handle,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.CreateAPIKey(params)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, result.Response)
}

// RevokeLLMProviderAPIKey implements ServerInterface.RevokeLLMProviderAPIKey
// (DELETE /llm-providers/{id}/api-keys/{apiKeyName})
func (s *APIServer) RevokeLLMProviderAPIKey(c *gin.Context, id string, apiKeyName string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "RevokeLLMProviderAPIKey", correlationID)
	if !ok {
		return
	}

	params := utils.APIKeyRevocationParams{
		Kind:          models.KindLlmProvider,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RevokeAPIKey(params)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// UpdateLLMProviderAPIKey implements ServerInterface.UpdateLLMProviderAPIKey
// (PUT /llm-providers/{id}/api-keys/{apiKeyName})
func (s *APIServer) UpdateLLMProviderAPIKey(c *gin.Context, id string, apiKeyName string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "UpdateLLMProviderAPIKey", correlationID)
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
		Kind:          models.KindLlmProvider,
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
		} else if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// RegenerateLLMProviderAPIKey implements ServerInterface.RegenerateLLMProviderAPIKey
// (POST /llm-providers/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RegenerateLLMProviderAPIKey(c *gin.Context, id string, apiKeyName string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "RegenerateLLMProviderAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyRegenerationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyRegenerationParams{
		Kind:          models.KindLlmProvider,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RegenerateAPIKey(params)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// ListLLMProviderAPIKeys implements ServerInterface.ListLLMProviderAPIKeys
// (GET /llm-providers/{id}/api-keys)
func (s *APIServer) ListLLMProviderAPIKeys(c *gin.Context, id string) {
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	user, ok := s.extractAuthenticatedUser(c, "ListLLMProviderAPIKeys", correlationID)
	if !ok {
		return
	}

	params := utils.ListAPIKeyParams{
		Kind:          models.KindLlmProvider,
		Handle:        handle,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.ListAPIKeys(params)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, result.Response)
}

// rematerializeLLMProviderConfig re-encodes persisted SourceConfiguration into
// the generated API type. Logs marshal/unmarshal failures with full context and
// returns a non-nil error (callers return 500 after persistence).
func rematerializeLLMProviderConfig(log *slog.Logger, id, displayName string, source any) (api.LLMProviderConfiguration, error) {
	j, err := json.Marshal(source)
	if err != nil {
		log.Error("Failed to marshal stored LLM provider source configuration",
			slog.String("id", id),
			slog.String("displayName", displayName),
			slog.Any("sourceConfiguration", source),
			slog.Any("error", err))
		return api.LLMProviderConfiguration{}, fmt.Errorf("marshal LLM provider config: %w", err)
	}
	var prov api.LLMProviderConfiguration
	if err := json.Unmarshal(j, &prov); err != nil {
		log.Error("Failed to unmarshal stored LLM provider configuration",
			slog.String("id", id),
			slog.String("displayName", displayName),
			slog.Any("sourceConfiguration", source),
			slog.Any("error", err))
		return api.LLMProviderConfiguration{}, fmt.Errorf("unmarshal LLM provider config: %w", err)
	}
	return prov, nil
}

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

// CreateAPIKey implements ServerInterface.CreateAPIKey
// (POST /apis/{id}/api-keys)
// Handles both local key generation and external key injection based on request payload
func (s *APIServer) CreateAPIKey(c *gin.Context, id string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	// Extract authenticated user from context
	user, ok := s.extractAuthenticatedUser(c, "CreateAPIKey", correlationID)
	if !ok {
		return // Error response already sent by extractAuthenticatedUser
	}

	log.Debug("Starting API key creation by generating or injecting a new key",
		slog.String("handle", handle),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Parse and validate request body
	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		log.Error("Failed to parse request body for API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// Prepare parameters
	params := utils.APIKeyCreationParams{
		Kind:          models.KindRestApi,
		Handle:        handle,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.CreateAPIKey(params)
	if err != nil {
		// Check error type to determine appropriate status code
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else {
			log.Error("Failed to create API key",
				slog.Any("error", err),
				slog.String("handle", handle),
				slog.String("correlation_id", correlationID))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "internal server error",
			})
		}
		return
	}

	log.Info("API key creation completed",
		slog.String("handle", handle),
		slog.String("key name", result.Response.ApiKey.Name),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Return the response using the generated schema
	c.JSON(http.StatusCreated, result.Response)
}

// RevokeAPIKey implements ServerInterface.RevokeAPIKey
// (DELETE /apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) RevokeAPIKey(c *gin.Context, id string, apiKeyName string) {
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
		slog.String("handle", handle),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Prepare parameters
	params := utils.APIKeyRevocationParams{
		Handle:        handle,
		APIKeyName:    apiKeyName,
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
			log.Error("Failed to revoke API key",
				slog.Any("error", err),
				slog.String("handle", handle),
				slog.String("correlation_id", correlationID))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "internal server error",
			})
		}
		return
	}

	log.Info("API key revoked successfully",
		slog.String("handle", handle),
		slog.String("key", apiKeyName),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Return the response using the generated schema
	c.JSON(http.StatusOK, result.Response)
}

// UpdateAPIKey implements ServerInterface.UpdateAPIKey
// (PUT /apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) UpdateAPIKey(c *gin.Context, id string, apiKeyName string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	// Extract authenticated user from context
	user, ok := s.extractAuthenticatedUser(c, "UpdateAPIKey", correlationID)
	if !ok {
		return // Error response already sent by extractAuthenticatedUser
	}

	log.Debug("Starting API key update",
		slog.String("handle", handle),
		slog.String("key_name", apiKeyName),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Parse and validate request body
	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		log.Warn("Invalid request body for API key update",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// If plain-text API key is not provided, return an error
	if request.ApiKey == nil || strings.TrimSpace(*request.ApiKey) == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "apiKey is required",
		})
		return
	}

	// Prepare parameters
	params := utils.APIKeyUpdateParams{
		Kind:          models.KindRestApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.UpdateAPIKey(params)
	if err != nil {
		// Check error type to determine appropriate status code
		if storage.IsOperationNotAllowedError(err) {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else {
			log.Error("Failed to update API key",
				slog.Any("error", err),
				slog.String("handle", handle),
				slog.String("correlation_id", correlationID))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "internal server error",
			})
		}
		return
	}

	log.Info("API key updated successfully",
		slog.String("handle", handle),
		slog.String("key_name", apiKeyName),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	c.JSON(http.StatusOK, result.Response)
}

// RegenerateAPIKey implements ServerInterface.RegenerateAPIKey
// (POST /apis/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RegenerateAPIKey(c *gin.Context, id string, apiKeyName string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(c, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(c)

	// Extract authenticated user from context
	user, ok := s.extractAuthenticatedUser(c, "RegenerateAPIKey", correlationID)
	if !ok {
		return // Error response already sent by extractAuthenticatedUser
	}

	log.Debug("Starting API key rotation",
		slog.String("handle", handle),
		slog.String("key name", apiKeyName),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Parse and validate request body
	var request api.APIKeyRegenerationRequest
	if err := s.bindRequestBody(c, &request); err != nil {
		log.Warn("Invalid request body for API key rotation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// Prepare parameters
	params := utils.APIKeyRegenerationParams{
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RegenerateAPIKey(params)
	if err != nil {
		// Check error type to determine appropriate status code
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
		} else {
			log.Error("Failed to regenerate API key",
				slog.Any("error", err),
				slog.String("handle", handle),
				slog.String("correlation_id", correlationID))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "internal server error",
			})
		}
		return
	}

	log.Info("API key rotation completed",
		slog.String("handle", handle),
		slog.String("key_name", apiKeyName),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

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
		slog.String("handle", handle),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

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
			log.Error("Failed to list API keys",
				slog.Any("error", err),
				slog.String("handle", handle),
				slog.String("correlation_id", correlationID))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "internal server error",
			})
		}
		return
	}

	log.Info("API key listing completed",
		slog.String("handle", handle),
		slog.String("user", user.UserID),
		slog.String("correlation_id", correlationID))

	// Return the response using the generated schema
	c.JSON(http.StatusOK, result.Response)
}

// resolveAPIIDByHandle resolves an API identifier (deployment ID or handle) to the internal deployment ID.
// It first attempts a direct ID lookup; if that fails, it falls back to handle-based resolution.
// Returns (apiID, nil) on success; on failure writes the HTTP response and returns ("", err).
func (s *APIServer) resolveAPIIDByHandle(c *gin.Context, handle string, log *slog.Logger) (string, error) {
	// First, try treating the input as a deployment ID.
	cfgByID, err := s.db.GetConfig(handle)
	if err != nil {
		if !storage.IsNotFoundError(err) {
			log.Error("Failed to look up API configuration by ID",
				slog.String("id", handle),
				slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to resolve API identifier",
			})
			return "", fmt.Errorf("database error")
		}
	} else if cfgByID != nil {
		if cfgByID.Kind != string(api.RestAPIKindRestApi) {
			log.Warn("Configuration is not a REST API",
				slog.String("id", handle),
				slog.String("kind", cfgByID.Kind))
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("Configuration with identifier '%s' is not a REST API", handle),
			})
			return "", fmt.Errorf("invalid api kind")
		}
		return cfgByID.UUID, nil
	}

	// Fallback: resolve by handle (metadata.name)
	cfg, err := s.db.GetConfigByKindAndHandle(models.KindRestApi, handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			log.Warn("API configuration not found", slog.String("handle_or_id", handle))
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("RestAPI with identifier '%s' not found", handle),
			})
			return "", fmt.Errorf("api not found")
		}
		log.Error("Failed to look up API configuration by handle",
			slog.String("handle", handle),
			slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to resolve API identifier",
		})
		return "", fmt.Errorf("database error")
	}
	if cfg == nil {
		log.Warn("API configuration not found", slog.String("handle_or_id", handle))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("RestAPI with identifier '%s' not found", handle),
		})
		return "", fmt.Errorf("api not found")
	}
	return cfg.UUID, nil
}

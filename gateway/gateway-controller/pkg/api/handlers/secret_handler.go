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
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// CreateSecret handles POST /secrets
func (s *APIServer) CreateSecret(c *gin.Context) {
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

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	// Delegate to service which parses/validates/encrypt and persists
	secret, err := s.secretService.CreateSecret(secrets.SecretParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to encrypt Secret", slog.Any("error", err))
		if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	log.Info("Secret created successfully",
		slog.String("secret_handle", secret.Handle),
		slog.String("correlation_id", correlationID))

	// Echo back the created secret in the k8s-shaped resource form. The plaintext
	// value is omitted so response logs and clients don't surface secret material.
	c.JSON(http.StatusCreated, buildSecretResourceResponse(secret, false))
}

// ListSecrets implements ServerInterface.ListSecrets
// (GET /secrets)
func (s *APIServer) ListSecrets(c *gin.Context) {
	log := s.logger
	correlationID := middleware.GetCorrelationID(c)

	log.Debug("Retrieving secretsList", slog.String("correlation_id", correlationID))

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	secretsMeta, err := s.secretService.GetSecrets(correlationID)
	if err != nil {
		log.Error("Failed to retrieve secretsList",
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve secretsList",
		})
		return
	}

	items := make([]any, 0, len(secretsMeta))
	for _, meta := range secretsMeta {
		items = append(items, buildSecretMetaResourceResponse(meta))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"count":   len(items),
		"secrets": items,
	})
}

// GetSecret handles GET /secrets/{id}
func (s *APIServer) GetSecret(c *gin.Context, id string) {
	log := s.logger
	correlationID := middleware.GetCorrelationID(c)

	log.Debug("Retrieving secret",
		slog.String("secret_handle", id),
		slog.String("correlation_id", correlationID))

	// Validate secret ID format
	if id == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Missing required field: id",
		})
		return
	}
	if len(id) > 255 {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Secret ID too long (max 255 characters)",
		})
		return
	}

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	// Retrieve secret
	secret, err := s.secretService.Get(id, correlationID)
	if err != nil {
		// Check for not found error
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}

		// Generic error for decryption failures (security-first)
		log.Error("Failed to retrieve secret",
			slog.String("secret_handle", id),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to decrypt secret",
		})
		return
	}

	log.Debug("Secret retrieved successfully",
		slog.String("secret_handle", secret.Handle),
		slog.String("correlation_id", correlationID))

	// Include the decrypted value on single-item GET (caller supplied id). The
	// value is omitted from list views but exposed here so automation can read
	// back a secret it just created/updated.
	c.JSON(http.StatusOK, buildSecretResourceResponse(secret, true))
}

// UpdateSecret handles PUT /secrets/{id}
func (s *APIServer) UpdateSecret(c *gin.Context, id string) {
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

	// Validate secret ID format
	if id == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Missing required field: id",
		})
		return
	}
	if len(id) > 255 {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Secret ID too long (max 255 characters)",
		})
		return
	}

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	// Delegate to service which parses/validates/encrypt and persists
	secret, err := s.secretService.UpdateSecret(id, secrets.SecretParams{
		Data:          body,
		ContentType:   c.GetHeader("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to encrypt Secret", slog.Any("error", err))
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	log.Info("Secret updated successfully",
		slog.String("secret_handle", secret.Handle),
		slog.String("correlation_id", correlationID))

	c.JSON(http.StatusOK, buildSecretResourceResponse(secret, false))
}

// DeleteSecret handles DELETE /secrets/{id}
func (s *APIServer) DeleteSecret(c *gin.Context, id string) {
	log := s.logger
	correlationID := middleware.GetCorrelationID(c)

	log.Debug("Deleting secret",
		slog.String("secret_id", id),
		slog.String("correlation_id", correlationID))

	// Validate secret ID format
	if id == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Missing required field: id",
		})
		return
	}
	if len(id) > 255 {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Secret ID too long (max 255 characters)",
		})
		return
	}

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	// Delete secret
	if err := s.secretService.Delete(id, correlationID); err != nil {
		// Check for not found error
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}

		// Generic error for storage failures
		log.Error("Failed to delete secret",
			slog.String("secret_id", id),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete secret",
		})
		return
	}

	log.Info("Secret deleted successfully",
		slog.String("secret_id", id),
		slog.String("correlation_id", correlationID))

	// Return 200 OK on successful deletion
	c.Status(http.StatusOK)
}

/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/model"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// WebSubAPIHmacSecretHandler handles HMAC secret CRUD for WebSub APIs.
type WebSubAPIHmacSecretHandler struct {
	secretService *service.WebSubAPIHmacSecretService
	slogger       *slog.Logger
}

// NewWebSubAPIHmacSecretHandler creates a new WebSubAPIHmacSecretHandler.
func NewWebSubAPIHmacSecretHandler(secretService *service.WebSubAPIHmacSecretService, slogger *slog.Logger) *WebSubAPIHmacSecretHandler {
	return &WebSubAPIHmacSecretHandler{
		secretService: secretService,
		slogger:       slogger,
	}
}

// RegisterRoutes registers the HMAC secret routes.
func (h *WebSubAPIHmacSecretHandler) RegisterRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1/websub-apis/:apiHandle/secrets")
	{
		v1.POST("", h.CreateHmacSecret)
		v1.GET("", h.ListHmacSecrets)
		v1.DELETE("/:secretName", h.DeleteHmacSecret)
		v1.POST("/:secretName/regenerate", h.RegenerateHmacSecret)
	}
}

func (h *WebSubAPIHmacSecretHandler) featureUnavailable(c *gin.Context) bool {
	if h.secretService == nil {
		c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
			"HMAC secret management is not configured on this server"))
		return true
	}
	return false
}

// CreateHmacSecret handles POST /api/v1/websub-apis/:apiHandle/secrets
func (h *WebSubAPIHmacSecretHandler) CreateHmacSecret(c *gin.Context) {
	if h.featureUnavailable(c) {
		return
	}
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := c.Param("apiHandle")
	if apiHandle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle is required"))
		return
	}

	var req api.WebSubAPIHmacSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if req.Secret != nil && *req.Secret == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "secret must not be empty; omit the field to auto-generate"))
		return
	}

	var externalSecret string
	if req.Secret != nil {
		externalSecret = *req.Secret
	}

	userID, _ := middleware.GetUserIDFromContext(c)
	secret, plaintext, err := h.secretService.Generate(orgID, apiHandle, req.DisplayName, externalSecret, userID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	msg := "HMAC secret generated successfully. Save the secret value — it will not be shown again."
	if externalSecret != "" {
		msg = "HMAC secret stored successfully."
	}
	c.JSON(http.StatusCreated, api.WebSubAPIHmacSecretCreationResponse{
		Secret:        plaintext,
		WebhookSecret: secretToInfo(secret),
		Message:       msg,
	})
}

// ListHmacSecrets handles GET /api/v1/websub-apis/:apiHandle/secrets
func (h *WebSubAPIHmacSecretHandler) ListHmacSecrets(c *gin.Context) {
	if h.featureUnavailable(c) {
		return
	}
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := c.Param("apiHandle")
	if apiHandle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle is required"))
		return
	}

	secrets, err := h.secretService.List(orgID, apiHandle)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	items := make([]api.WebSubAPIHmacSecretInfo, 0, len(secrets))
	for _, s := range secrets {
		items = append(items, *secretToInfo(s))
	}
	c.JSON(http.StatusOK, api.WebSubAPIHmacSecretListResponse{Secrets: items})
}

// DeleteHmacSecret handles DELETE /api/v1/websub-apis/:apiHandle/secrets/:secretName
func (h *WebSubAPIHmacSecretHandler) DeleteHmacSecret(c *gin.Context) {
	if h.featureUnavailable(c) {
		return
	}
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := c.Param("apiHandle")
	secretName := c.Param("secretName")
	if apiHandle == "" || secretName == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle and secret name are required"))
		return
	}

	if err := h.secretService.Delete(orgID, apiHandle, secretName); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// RegenerateHmacSecret handles POST /api/v1/websub-apis/:apiHandle/secrets/:secretName/regenerate
func (h *WebSubAPIHmacSecretHandler) RegenerateHmacSecret(c *gin.Context) {
	if h.featureUnavailable(c) {
		return
	}
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := c.Param("apiHandle")
	secretName := c.Param("secretName")
	if apiHandle == "" || secretName == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle and secret name are required"))
		return
	}

	var req api.WebSubAPIHmacSecretRegenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.AbortWithStatusJSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if req.Secret != nil && *req.Secret == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "secret must not be empty; omit the field to auto-generate"))
		return
	}

	var externalSecret string
	if req.Secret != nil {
		externalSecret = *req.Secret
	}

	userID, _ := middleware.GetUserIDFromContext(c)
	secret, plaintext, err := h.secretService.Regenerate(orgID, apiHandle, secretName, externalSecret, userID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	msg := "HMAC secret regenerated successfully. Save the new secret value — it will not be shown again."
	if externalSecret != "" {
		msg = "HMAC secret rotated to the provided value successfully."
	}
	c.JSON(http.StatusOK, api.WebSubAPIHmacSecretCreationResponse{
		Secret:        plaintext,
		WebhookSecret: secretToInfo(secret),
		Message:       msg,
	})
}

func (h *WebSubAPIHmacSecretHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, constants.ErrWebSubAPINotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebSub API not found"))
	case errors.Is(err, constants.ErrHmacSecretNotFound):
		c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "HMAC secret not found"))
	case errors.Is(err, constants.ErrHmacSecretAlreadyExists):
		c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "An HMAC secret with this name already exists"))
	case errors.Is(err, constants.ErrHmacSecretInvalidValue):
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Secret value must be at least 32 characters"))
	case errors.Is(err, constants.ErrHmacSecretEncryptionKeyMissing):
		h.slogger.Error("HMAC secret encryption key is not configured")
		c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable", "HMAC secret management is not configured on this server"))
	default:
		h.slogger.Error("HMAC secret service error", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}

func secretToInfo(s *model.WebSubAPIHmacSecret) *api.WebSubAPIHmacSecretInfo {
	if s == nil {
		return nil
	}
	return &api.WebSubAPIHmacSecretInfo{
		Uuid:        s.UUID,
		Name:        s.Handle,
		DisplayName: s.Name,
		Status:      s.Status,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

type SecretHandler struct {
	secretService *service.SecretService
	slogger       *slog.Logger
}

func NewSecretHandler(secretService *service.SecretService, slogger *slog.Logger) *SecretHandler {
	return &SecretHandler{secretService: secretService, slogger: slogger}
}

func (h *SecretHandler) RegisterRoutes(r *gin.Engine) {
	for _, version := range []string{"/api/v0.9", "/api/v1"} {
		g := r.Group(version)
		g.POST("/secrets", h.CreateSecret)
		g.GET("/secrets", h.ListSecrets)
		g.GET("/secrets/:id", h.GetSecret)
		g.PUT("/secrets/:id", h.UpdateSecret)
		g.DELETE("/secrets/:id", h.DeleteSecret)
	}
}

func (h *SecretHandler) CreateSecret(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	username, _ := middleware.GetUsernameFromContext(c)

	var req dto.CreateSecretRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	resp, err := h.secretService.Create(orgID, username, &req)
	if err != nil {
		if errors.Is(err, constants.ErrSecretAlreadyExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "A secret with this name already exists in this scope"))
			return
		}
		if errors.Is(err, constants.ErrInvalidSecretType) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
			return
		}
		h.slogger.Error("failed to create secret", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create secret"))
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *SecretHandler) ListSecrets(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	limit := 25
	offset := 0
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			if v > 100 {
				v = 100
			}
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	var updatedAfter *time.Time
	if ua := c.Query("updatedAfter"); ua != "" {
		t, err := time.Parse(time.RFC3339, ua)
		if err != nil {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "updatedAfter must be an RFC3339 timestamp"))
			return
		}
		updatedAfter = &t
	}

	resp, err := h.secretService.List(orgID, limit, offset, updatedAfter)
	if err != nil {
		h.slogger.Error("failed to list secrets", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list secrets"))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *SecretHandler) GetSecret(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	handle := c.Param("id")
	if handle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Secret name is required"))
		return
	}

	summary, err := h.secretService.Get(orgID, handle)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Secret not found"))
			return
		}
		h.slogger.Error("failed to get secret", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get secret"))
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *SecretHandler) UpdateSecret(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	handle := c.Param("id")
	if handle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Secret name is required"))
		return
	}

	username, _ := middleware.GetUsernameFromContext(c)

	var req dto.UpdateSecretRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	resp, err := h.secretService.Update(orgID, handle, username, &req)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Secret not found"))
			return
		}
		h.slogger.Error("failed to update secret", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update secret"))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *SecretHandler) DeleteSecret(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	handle := c.Param("id")
	if handle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Secret name is required"))
		return
	}

	username, _ := middleware.GetUsernameFromContext(c)

	err := h.secretService.Delete(orgID, handle, username)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Secret not found"))
			return
		}

		var inUseErr *service.SecretInUseError
		if errors.As(err, &inUseErr) {
			refs := make([]dto.SecretReferenceDTO, 0, len(inUseErr.References))
			for _, r := range inUseErr.References {
				refs = append(refs, dto.SecretReferenceDTO{Type: r.Type, Handle: r.Handle, Name: r.Name})
			}
			c.JSON(http.StatusConflict, dto.SecretDeleteConflictResponse{
				Error:      "secret is referenced by active resources",
				References: refs,
			})
			return
		}

		h.slogger.Error("failed to delete secret", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete secret"))
		return
	}

	c.Status(http.StatusNoContent)
}


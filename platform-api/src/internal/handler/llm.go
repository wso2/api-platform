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
	"strings"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

type LLMHandler struct {
	templateService *service.LLMProviderTemplateService
	providerService *service.LLMProviderService
	proxyService    *service.LLMProxyService
	slogger         *slog.Logger
}

func NewLLMHandler(
	templateService *service.LLMProviderTemplateService,
	providerService *service.LLMProviderService,
	proxyService *service.LLMProxyService,
	slogger *slog.Logger,
) *LLMHandler {
	return &LLMHandler{templateService: templateService, providerService: providerService, proxyService: proxyService, slogger: slogger}
}

func (h *LLMHandler) RegisterRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	{
		// LLM Provider Templates
		v1.POST("/llm-provider-templates", h.CreateLLMProviderTemplate)
		v1.GET("/llm-provider-templates", h.ListLLMProviderTemplates)
		v1.GET("/llm-provider-templates/:id", h.GetLLMProviderTemplate)
		v1.PUT("/llm-provider-templates/:id", h.UpdateLLMProviderTemplate)
		v1.DELETE("/llm-provider-templates/:id", h.DeleteLLMProviderTemplate)

		// LLM Providers
		v1.POST("/llm-providers", h.CreateLLMProvider)
		v1.GET("/llm-providers", h.ListLLMProviders)
		v1.GET("/llm-providers/:id", h.GetLLMProvider)
		v1.GET("/llm-providers/:id/llm-proxies", h.ListLLMProxiesByProvider)
		v1.PUT("/llm-providers/:id", h.UpdateLLMProvider)
		v1.DELETE("/llm-providers/:id", h.DeleteLLMProvider)

		// LLM Proxies
		v1.POST("/llm-proxies", h.CreateLLMProxy)
		v1.GET("/llm-proxies", h.ListLLMProxies)
		v1.GET("/llm-proxies/:id", h.GetLLMProxy)
		v1.PUT("/llm-proxies/:id", h.UpdateLLMProxy)
		v1.DELETE("/llm-proxies/:id", h.DeleteLLMProxy)
	}
}

// ---- Templates ----

func (h *LLMHandler) CreateLLMProviderTemplate(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.LLMProviderTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}
	createdBy, _ := middleware.GetUsernameFromContext(c)

	created, err := h.templateService.Create(orgID, createdBy, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateExists):
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "LLM provider template already exists"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid input"))
			return
		default:
			h.slogger.Error("Failed to create LLM provider template", "organizationId", orgID, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create LLM provider template"))
			return
		}
	}

	c.JSON(http.StatusCreated, created)
}

func (h *LLMHandler) ListLLMProviderTemplates(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	resp, err := h.templateService.List(orgID, limit, offset)
	if err != nil {
		h.slogger.Error("Failed to list LLM provider templates", "organizationId", orgID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list LLM provider templates"))
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LLMHandler) GetLLMProviderTemplate(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	resp, err := h.templateService.Get(orgID, id)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "LLM provider template not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid template id"))
			return
		default:
			h.slogger.Error("Failed to get LLM provider template", "organizationId", orgID, "templateId", id, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get LLM provider template"))
			return
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LLMHandler) UpdateLLMProviderTemplate(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	var req api.LLMProviderTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	resp, err := h.templateService.Update(orgID, id, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "LLM provider template not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid input"))
			return
		default:
			h.slogger.Error("Failed to update LLM provider template", "organizationId", orgID, "templateId", id, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update LLM provider template"))
			return
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LLMHandler) DeleteLLMProviderTemplate(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	if err := h.templateService.Delete(orgID, id); err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "LLM provider template not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid template id"))
			return
		default:
			h.slogger.Error("Failed to delete LLM provider template", "organizationId", orgID, "templateId", id, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete LLM provider template"))
			return
		}
	}

	c.Status(http.StatusNoContent)
}

// ---- Providers ----

func (h *LLMHandler) CreateLLMProvider(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.LLMProvider
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}
	createdBy, _ := middleware.GetUsernameFromContext(c)

	created, err := h.providerService.Create(orgID, createdBy, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderLimitReached):
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "LLM provider limit reached for organization"))
			return
		case errors.Is(err, constants.ErrLLMProviderExists):
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "LLM provider already exists"))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Referenced template not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid input"))
			return
		default:
			h.slogger.Error("Failed to create LLM provider", "organizationId", orgID, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create LLM provider"))
			return
		}
	}
	c.JSON(http.StatusCreated, created)
}

func (h *LLMHandler) ListLLMProviders(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	resp, err := h.providerService.List(orgID, limit, offset)
	if err != nil {
		h.slogger.Error("Failed to list LLM providers", "organizationId", orgID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list LLM providers"))
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LLMHandler) GetLLMProvider(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	resp, err := h.providerService.Get(orgID, id)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "LLM provider not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid provider id"))
			return
		default:
			h.slogger.Error("Failed to get LLM provider", "organizationId", orgID, "providerId", id, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get LLM provider"))
			return
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LLMHandler) UpdateLLMProvider(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	var req api.LLMProvider
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	resp, err := h.providerService.Update(orgID, id, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "LLM provider not found"))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Referenced template not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid input"))
			return
		default:
			h.slogger.Error("Failed to update LLM provider", "organizationId", orgID, "providerId", id, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update LLM provider"))
			return
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LLMHandler) DeleteLLMProvider(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	if err := h.providerService.Delete(orgID, id); err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "LLM provider not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid provider id"))
			return
		default:
			h.slogger.Error("Failed to delete LLM provider", "organizationId", orgID, "providerId", id, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete LLM provider"))
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// ---- Proxies ----

func (h *LLMHandler) CreateLLMProxy(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.LLMProxy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}
	if req.ProjectId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project ID is required"))
		return
	}
	createdBy, _ := middleware.GetUsernameFromContext(c)

	created, err := h.proxyService.Create(orgID, createdBy, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyLimitReached):
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "LLM proxy limit reached for organization"))
			return
		case errors.Is(err, constants.ErrLLMProxyExists):
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "LLM proxy already exists"))
			return
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Referenced provider not found"))
			return
		case errors.Is(err, constants.ErrProjectNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Project not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid input"))
			return
		default:
			h.slogger.Error("Failed to create LLM proxy", "organizationId", orgID, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create LLM proxy"))
			return
		}
	}
	c.JSON(http.StatusCreated, created)
}

func (h *LLMHandler) ListLLMProxies(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	projectID := strings.TrimSpace(c.Query("projectId"))
	if projectID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "projectId query parameter is required"))
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	resp, err := h.proxyService.List(orgID, &projectID, limit, offset)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Project not found"))
			return
		}
		h.slogger.Error("Failed to list LLM proxies", "organizationId", orgID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list LLM proxies"))
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LLMHandler) ListLLMProxiesByProvider(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	providerID := c.Param("id")

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	resp, err := h.proxyService.ListByProvider(orgID, providerID, limit, offset)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "LLM provider not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid provider id"))
			return
		default:
			h.slogger.Error("Failed to list LLM proxies by provider", "organizationId", orgID, "providerId", providerID, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list LLM proxies"))
			return
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LLMHandler) GetLLMProxy(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	resp, err := h.proxyService.Get(orgID, id)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "LLM proxy not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid proxy id"))
			return
		default:
			h.slogger.Error("Failed to get LLM proxy", "organizationId", orgID, "proxyId", id, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get LLM proxy"))
			return
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LLMHandler) UpdateLLMProxy(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	var req api.LLMProxy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	resp, err := h.proxyService.Update(orgID, id, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "LLM proxy not found"))
			return
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Referenced provider not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid input"))
			return
		default:
			h.slogger.Error("Failed to update LLM proxy", "organizationId", orgID, "proxyId", id, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update LLM proxy"))
			return
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LLMHandler) DeleteLLMProxy(c *gin.Context) {
	orgID, ok := middleware.GetOrganizationFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}
	id := c.Param("id")

	if err := h.proxyService.Delete(orgID, id); err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "LLM proxy not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid proxy id"))
			return
		default:
			h.slogger.Error("Failed to delete LLM proxy", "organizationId", orgID, "proxyId", id, "error", err)
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete LLM proxy"))
			return
		}
	}
	c.Status(http.StatusNoContent)
}

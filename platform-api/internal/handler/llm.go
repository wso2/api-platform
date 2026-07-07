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
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

type LLMHandler struct {
	templateService *service.LLMProviderTemplateService
	providerService *service.LLMProviderService
	proxyService    *service.LLMProxyService
	identity        *service.IdentityService
	slogger         *slog.Logger
}

func NewLLMHandler(
	templateService *service.LLMProviderTemplateService,
	providerService *service.LLMProviderService,
	proxyService *service.LLMProxyService,
	identity *service.IdentityService,
	slogger *slog.Logger,
) *LLMHandler {
	return &LLMHandler{templateService: templateService, providerService: providerService, proxyService: proxyService, identity: identity, slogger: slogger}
}

func (h *LLMHandler) RegisterRoutes(mux *http.ServeMux) {
	// LLM Provider Templates
	mux.HandleFunc("POST "+constants.APIBasePath+"/llm-provider-templates", h.CreateLLMProviderTemplate)
	mux.HandleFunc("POST "+constants.APIBasePath+"/llm-provider-templates/copy", h.CopyLLMProviderTemplateVersion)
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-provider-templates", h.ListLLMProviderTemplates)
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-provider-templates/{llmProviderTemplateId}", h.GetLLMProviderTemplate)
	mux.HandleFunc("PUT "+constants.APIBasePath+"/llm-provider-templates/{llmProviderTemplateId}", h.UpdateLLMProviderTemplate)
	mux.HandleFunc("PATCH "+constants.APIBasePath+"/llm-provider-templates/{llmProviderTemplateId}", h.SetLLMProviderTemplateVersionEnabled)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/llm-provider-templates/{llmProviderTemplateId}", h.DeleteLLMProviderTemplateVersion)

	// LLM Providers
	mux.HandleFunc("POST "+constants.APIBasePath+"/llm-providers", h.CreateLLMProvider)
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-providers", h.ListLLMProviders)
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-providers/{llmProviderId}", h.GetLLMProvider)
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-providers/{llmProviderId}/llm-proxies", h.ListLLMProxiesByProvider)
	mux.HandleFunc("PUT "+constants.APIBasePath+"/llm-providers/{llmProviderId}", h.UpdateLLMProvider)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/llm-providers/{llmProviderId}", h.DeleteLLMProvider)

	// LLM Proxies
	mux.HandleFunc("POST "+constants.APIBasePath+"/llm-proxies", h.CreateLLMProxy)
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-proxies", h.ListLLMProxies)
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-proxies/{llmProxyId}", h.GetLLMProxy)
	mux.HandleFunc("PUT "+constants.APIBasePath+"/llm-proxies/{llmProxyId}", h.UpdateLLMProxy)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/llm-proxies/{llmProxyId}", h.DeleteLLMProxy)
}

// templateQuery holds the fields parsed from the ?query= search DSL used by the
// LLM provider template endpoints.
type templateQuery struct {
	GroupID string
	Version string
	Latest  bool
}

func parseTemplateQuery(raw string) (q templateQuery, found bool) {
	for _, part := range strings.Split(raw, "&") {
		key, value, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "groupId":
			q.GroupID = strings.TrimSpace(value)
			found = true
		case "version":
			q.Version = strings.TrimSpace(value)
		case "latest":
			q.Latest = strings.EqualFold(strings.TrimSpace(value), "true")
		}
	}
	return q, found
}

func (h *LLMHandler) CreateLLMProviderTemplate(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	var req api.LLMProviderTemplate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Invalid request body"))
		return
	}

	createdBy, ok := resolveActor(w, r, h.identity, h.slogger, "create LLM provider template")
	if !ok {
		return
	}

	created, err := h.templateService.Create(orgID, createdBy, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateExists):
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateExists, "An LLM provider template with this ID already exists."))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateManagedByReserved):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateManagedByReserved, "'wso2' is reserved and cannot be used as managedBy on custom templates."))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid input"))
			return
		default:
			h.slogger.Error("Failed to create LLM provider template", "organizationId", orgID, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to create LLM provider template"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusCreated, created)
}

// CopyLLMProviderTemplateVersion creates a new version within a family by
// cloning an existing version. The source version, the derived destination
// handle and the new version are given as query params
// (fromTemplateId, toTemplateId, toVersion); an optional body overrides fields
// on top of the copied config.
func (h *LLMHandler) CopyLLMProviderTemplateVersion(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	createdBy, ok := resolveActor(w, r, h.identity, h.slogger, "copy LLM provider template version")
	if !ok {
		return
	}

	fromTemplateID := strings.TrimSpace(r.URL.Query().Get("fromTemplateId"))
	toTemplateID := strings.TrimSpace(r.URL.Query().Get("toTemplateId"))
	toVersion := strings.TrimSpace(r.URL.Query().Get("toVersion"))
	if fromTemplateID == "" || toVersion == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "fromTemplateId and toVersion are required"))
		return
	}

	// The body is optional; only decode overrides when one is present.
	var overrides *api.CreateLLMProviderTemplateVersionRequest
	if r.Body != nil && r.ContentLength != 0 {
		var vreq api.CreateLLMProviderTemplateVersionRequest
		if err := json.NewDecoder(r.Body).Decode(&vreq); err != nil && !errors.Is(err, io.EOF) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid request body"))
			return
		}
		overrides = &vreq
	}

	created, err := h.templateService.CopyVersion(orgID, fromTemplateID, toTemplateID, toVersion, createdBy, overrides)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateNotFound, "The source LLM provider template version could not be found."))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateVersionExists):
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateExists, "This template version already exists."))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateManagedByReserved):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateManagedByReserved, "'wso2' is reserved and cannot be used as managedBy on custom templates."))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid input. toVersion must match the v<major>.<minor> pattern starting from v1.0 (e.g. v1.0), and toTemplateId must match the family"))
			return
		default:
			h.slogger.Error("Failed to copy LLM provider template version", "organizationId", orgID, "fromTemplateId", fromTemplateID, "toVersion", toVersion, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to copy LLM provider template version"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *LLMHandler) ListLLMProviderTemplates(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "20"
	}
	offsetStr := r.URL.Query().Get("offset")
	if offsetStr == "" {
		offsetStr = "0"
	}

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

	q, familyScoped := parseTemplateQuery(r.URL.Query().Get("query"))
	if familyScoped {
		groupID := q.GroupID
		if groupID == "" {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid groupId"))
			return
		}
		version := q.Version
		if version != "" {
			resp, err := h.templateService.GetVersion(orgID, groupID, version)
			if err != nil {
				switch {
				case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
					httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
						utils.CodeLLMProviderTemplateNotFound, "The specified LLM provider template version could not be found."))
					return
				case errors.Is(err, constants.ErrInvalidInput):
					httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
						utils.CodeCommonValidationFailed, "Invalid version. Version must match the v<major>.<minor> pattern (e.g. v1.0)"))
					return
				default:
					h.slogger.Error("Failed to get LLM provider template version", "organizationId", orgID, "groupId", groupID, "version", version, "error", err)
					httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
						utils.CodeCommonInternalError, "Failed to get LLM provider template version"))
					return
				}
			}
			httputil.WriteJSON(w, http.StatusOK, resp)
			return
		}
		resp, err := h.templateService.ListVersions(orgID, groupID, limit, offset)
		if err != nil {
			switch {
			case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
				httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
					utils.CodeLLMProviderTemplateNotFound, "The specified LLM provider template could not be found."))
				return
			case errors.Is(err, constants.ErrInvalidInput):
				httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
					utils.CodeCommonValidationFailed, "Invalid groupId"))
				return
			default:
				h.slogger.Error("Failed to list LLM provider template versions", "organizationId", orgID, "groupId", groupID, "error", err)
				httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
					utils.CodeCommonInternalError, "Failed to list LLM provider template versions"))
				return
			}
		}
		httputil.WriteJSON(w, http.StatusOK, resp)
		return
	}

	latestOnly := q.Latest

	resp, err := h.templateService.List(orgID, limit, offset, latestOnly)
	if err != nil {
		h.slogger.Error("Failed to list LLM provider templates", "organizationId", orgID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to list LLM provider templates"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *LLMHandler) GetLLMProviderTemplate(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}
	id := r.PathValue("llmProviderTemplateId")

	resp, err := h.templateService.Get(orgID, id)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateNotFound, "The specified LLM provider template could not be found."))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid template id"))
			return
		default:
			h.slogger.Error("Failed to get LLM provider template", "organizationId", orgID, "templateId", id, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to get LLM provider template"))
			return
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *LLMHandler) SetLLMProviderTemplateVersionEnabled(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}
	id := r.PathValue("llmProviderTemplateId")

	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Request body must include a boolean 'enabled' field"))
		return
	}

	resp, err := h.templateService.SetEnabledByHandle(orgID, id, *body.Enabled)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateNotFound, "The specified LLM provider template version could not be found."))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid template id"))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateInUse):
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateInUse, "Cannot disable this template version while providers are using it."))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateNotToggleable):
			httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateNotToggleable, "Only built-in templates can be enabled or disabled."))
			return
		default:
			h.slogger.Error("Failed to set LLM provider template version enabled", "organizationId", orgID, "templateId", id, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to update template version"))
			return
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *LLMHandler) UpdateLLMProviderTemplate(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}
	id := r.PathValue("llmProviderTemplateId")

	var req api.LLMProviderTemplate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Invalid request body"))
		return
	}

	updatedBy, ok := resolveActor(w, r, h.identity, h.slogger, "update LLM provider template")
	if !ok {
		return
	}
	resp, err := h.templateService.Update(orgID, id, updatedBy, &req)
	if err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateNotFound, "The specified LLM provider template could not be found."))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateReadOnly):
			httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateReadOnly, "Built-in templates are read-only and cannot be edited."))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateManagedByReserved):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateManagedByReserved, "'wso2' is reserved and cannot be used as managedBy on custom templates."))
			return
		case errors.Is(err, constants.ErrHandleImmutable):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, err.Error()))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid input"))
			return
		default:
			h.slogger.Error("Failed to update LLM provider template", "organizationId", orgID, "templateId", id, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to update LLM provider template"))
			return
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

// DeleteLLMProviderTemplateVersion removes a single version of a template.
func (h *LLMHandler) DeleteLLMProviderTemplateVersion(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}
	id := r.PathValue("llmProviderTemplateId")

	if err := h.templateService.DeleteByHandle(orgID, id); err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateNotFound, "The specified LLM provider template version could not be found."))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateInUse):
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateInUse, "This template version cannot be deleted while providers are using it."))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateReadOnly):
			httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateReadOnly, "Built-in template versions are read-only and cannot be deleted."))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid template id"))
			return
		default:
			h.slogger.Error("Failed to delete LLM provider template version", "organizationId", orgID, "templateId", id, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to delete template version"))
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- Providers ----

func (h *LLMHandler) CreateLLMProvider(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	var req api.LLMProvider
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Invalid request body"))
		return
	}
	createdBy, ok := resolveActor(w, r, h.identity, h.slogger, "create LLM provider")
	if !ok {
		return
	}

	created, err := h.providerService.Create(orgID, createdBy, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderLimitReached):
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderLimitReached, "LLM provider limit reached for the organization."))
			return
		case errors.Is(err, constants.ErrLLMProviderExists):
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderExists, "An LLM provider with this ID already exists."))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateNotFound, "The referenced LLM provider template could not be found."))
			return
		case errors.Is(err, constants.ErrSecretRefMissing):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, err.Error()))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid input"))
			return
		default:
			h.slogger.Error("Failed to create LLM provider", "organizationId", orgID, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to create LLM provider"))
			return
		}
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *LLMHandler) ListLLMProviders(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "20"
	}
	offsetStr := r.URL.Query().Get("offset")
	if offsetStr == "" {
		offsetStr = "0"
	}

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
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to list LLM providers"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *LLMHandler) GetLLMProvider(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}
	id := r.PathValue("llmProviderId")

	resp, err := h.providerService.Get(orgID, id)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderNotFound, "The specified LLM provider could not be found."))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid provider id"))
			return
		default:
			h.slogger.Error("Failed to get LLM provider", "organizationId", orgID, "providerId", id, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to get LLM provider"))
			return
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *LLMHandler) UpdateLLMProvider(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}
	id := r.PathValue("llmProviderId")

	var req api.LLMProvider
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Invalid request body"))
		return
	}

	updatedBy, ok := resolveActor(w, r, h.identity, h.slogger, "update LLM provider")
	if !ok {
		return
	}
	resp, err := h.providerService.Update(orgID, id, updatedBy, &req)
	if err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderNotFound, "The specified LLM provider could not be found."))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderTemplateNotFound, "The referenced LLM provider template could not be found."))
			return
		case errors.Is(err, constants.ErrSecretRefMissing):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, err.Error()))
			return
		case errors.Is(err, constants.ErrHandleImmutable):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, err.Error()))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid input"))
			return
		default:
			h.slogger.Error("Failed to update LLM provider", "organizationId", orgID, "providerId", id, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to update LLM provider"))
			return
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *LLMHandler) DeleteLLMProvider(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}
	id := r.PathValue("llmProviderId")
	deletedBy, ok := resolveActor(w, r, h.identity, h.slogger, "delete LLM provider")
	if !ok {
		return
	}

	if err := h.providerService.Delete(orgID, id, deletedBy); err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderNotFound, "The specified LLM provider could not be found."))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid provider id"))
			return
		default:
			h.slogger.Error("Failed to delete LLM provider", "organizationId", orgID, "providerId", id, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to delete LLM provider"))
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Proxies ----

func (h *LLMHandler) CreateLLMProxy(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	var req api.LLMProxy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Invalid request body"))
		return
	}
	if req.ProjectId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Project ID is required"))
		return
	}
	createdBy, ok := resolveActor(w, r, h.identity, h.slogger, "create LLM proxy")
	if !ok {
		return
	}

	created, err := h.proxyService.Create(orgID, createdBy, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyLimitReached):
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponseWithCode(
				utils.CodeLLMProxyLimitReached, "LLM proxy limit reached for the organization."))
			return
		case errors.Is(err, constants.ErrLLMProxyExists):
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponseWithCode(
				utils.CodeLLMProxyExists, "An LLM proxy with this ID already exists."))
			return
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderNotFound, "The referenced LLM provider could not be found."))
			return
		case errors.Is(err, constants.ErrProjectNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeProjectNotFound, "The specified project could not be found."))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid input"))
			return
		default:
			h.slogger.Error("Failed to create LLM proxy", "organizationId", orgID, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to create LLM proxy"))
			return
		}
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *LLMHandler) ListLLMProxies(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "projectId query parameter is required"))
		return
	}

	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "20"
	}
	offsetStr := r.URL.Query().Get("offset")
	if offsetStr == "" {
		offsetStr = "0"
	}

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
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeProjectNotFound, "The specified project could not be found."))
			return
		}
		h.slogger.Error("Failed to list LLM proxies", "organizationId", orgID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to list LLM proxies"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *LLMHandler) ListLLMProxiesByProvider(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}
	providerID := r.PathValue("llmProviderId")

	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "20"
	}
	offsetStr := r.URL.Query().Get("offset")
	if offsetStr == "" {
		offsetStr = "0"
	}

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
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderNotFound, "The specified LLM provider could not be found."))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid provider id"))
			return
		default:
			h.slogger.Error("Failed to list LLM proxies by provider", "organizationId", orgID, "providerId", providerID, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to list LLM proxies"))
			return
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *LLMHandler) GetLLMProxy(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}
	id := r.PathValue("llmProxyId")

	resp, err := h.proxyService.Get(orgID, id)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProxyNotFound, "The specified LLM proxy could not be found."))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid proxy id"))
			return
		default:
			h.slogger.Error("Failed to get LLM proxy", "organizationId", orgID, "proxyId", id, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to get LLM proxy"))
			return
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *LLMHandler) UpdateLLMProxy(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}
	id := r.PathValue("llmProxyId")

	var req api.LLMProxy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Invalid request body"))
		return
	}

	updatedBy, ok := resolveActor(w, r, h.identity, h.slogger, "update LLM proxy")
	if !ok {
		return
	}
	resp, err := h.proxyService.Update(orgID, id, updatedBy, &req)
	if err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProxyNotFound, "The specified LLM proxy could not be found."))
			return
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeLLMProviderNotFound, "The referenced LLM provider could not be found."))
			return
		case errors.Is(err, constants.ErrHandleImmutable):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, err.Error()))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid input"))
			return
		default:
			h.slogger.Error("Failed to update LLM proxy", "organizationId", orgID, "proxyId", id, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to update LLM proxy"))
			return
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *LLMHandler) DeleteLLMProxy(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}
	id := r.PathValue("llmProxyId")
	deletedBy, ok := resolveActor(w, r, h.identity, h.slogger, "delete LLM proxy")
	if !ok {
		return
	}

	if err := h.proxyService.Delete(orgID, id, deletedBy); err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProxyNotFound, "The specified LLM proxy could not be found."))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "Invalid proxy id"))
			return
		default:
			h.slogger.Error("Failed to delete LLM proxy", "organizationId", orgID, "proxyId", id, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
				utils.CodeCommonInternalError, "Failed to delete LLM proxy"))
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

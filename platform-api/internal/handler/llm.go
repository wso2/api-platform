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
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
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
	mux.HandleFunc("POST "+constants.APIBasePath+"/llm-provider-templates", middleware.MapErrors(h.slogger, h.CreateLLMProviderTemplate))
	mux.HandleFunc("POST "+constants.APIBasePath+"/llm-provider-templates/copy", middleware.MapErrors(h.slogger, h.CopyLLMProviderTemplateVersion))
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-provider-templates", middleware.MapErrors(h.slogger, h.ListLLMProviderTemplates))
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-provider-templates/{llmProviderTemplateId}", middleware.MapErrors(h.slogger, h.GetLLMProviderTemplate))
	mux.HandleFunc("PUT "+constants.APIBasePath+"/llm-provider-templates/{llmProviderTemplateId}", middleware.MapErrors(h.slogger, h.UpdateLLMProviderTemplate))
	mux.HandleFunc("PATCH "+constants.APIBasePath+"/llm-provider-templates/{llmProviderTemplateId}", middleware.MapErrors(h.slogger, h.SetLLMProviderTemplateVersionEnabled))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/llm-provider-templates/{llmProviderTemplateId}", middleware.MapErrors(h.slogger, h.DeleteLLMProviderTemplateVersion))

	// LLM Providers
	mux.HandleFunc("POST "+constants.APIBasePath+"/llm-providers", middleware.MapErrors(h.slogger, h.CreateLLMProvider))
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-providers", middleware.MapErrors(h.slogger, h.ListLLMProviders))
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-providers/{llmProviderId}", middleware.MapErrors(h.slogger, h.GetLLMProvider))
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-providers/{llmProviderId}/llm-proxies", middleware.MapErrors(h.slogger, h.ListLLMProxiesByProvider))
	mux.HandleFunc("PUT "+constants.APIBasePath+"/llm-providers/{llmProviderId}", middleware.MapErrors(h.slogger, h.UpdateLLMProvider))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/llm-providers/{llmProviderId}", middleware.MapErrors(h.slogger, h.DeleteLLMProvider))

	// LLM Proxies
	mux.HandleFunc("POST "+constants.APIBasePath+"/llm-proxies", middleware.MapErrors(h.slogger, h.CreateLLMProxy))
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-proxies", middleware.MapErrors(h.slogger, h.ListLLMProxies))
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-proxies/{llmProxyId}", middleware.MapErrors(h.slogger, h.GetLLMProxy))
	mux.HandleFunc("PUT "+constants.APIBasePath+"/llm-proxies/{llmProxyId}", middleware.MapErrors(h.slogger, h.UpdateLLMProxy))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/llm-proxies/{llmProxyId}", middleware.MapErrors(h.slogger, h.DeleteLLMProxy))
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

func (h *LLMHandler) CreateLLMProviderTemplate(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var req api.LLMProviderTemplate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body")
	}

	createdBy, err := resolveActorErr(r, h.identity, "create LLM provider template")
	if err != nil {
		return err
	}

	created, err := h.templateService.Create(orgID, createdBy, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateExists):
			return apperror.LLMProviderTemplateExists.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderTemplateManagedByReserved):
			return apperror.LLMProviderTemplateManagedByReserved.Wrap(err)
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid input")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to create LLM provider template in org %s", orgID))
		}
	}

	setLocation(w, "llm-provider-templates", strOrEmpty(created.Id))
	httputil.WriteJSON(w, http.StatusCreated, created)
	return nil
}

// CopyLLMProviderTemplateVersion creates a new version within a family by
// cloning an existing version. The source version, the derived destination
// handle and the new version are given as query params
// (fromTemplateId, toTemplateId, toVersion); an optional body overrides fields
// on top of the copied config.
func (h *LLMHandler) CopyLLMProviderTemplateVersion(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	createdBy, err := resolveActorErr(r, h.identity, "copy LLM provider template version")
	if err != nil {
		return err
	}

	fromTemplateID := strings.TrimSpace(r.URL.Query().Get("fromTemplateId"))
	toTemplateID := strings.TrimSpace(r.URL.Query().Get("toTemplateId"))
	toVersion := strings.TrimSpace(r.URL.Query().Get("toVersion"))
	if fromTemplateID == "" || toVersion == "" {
		return apperror.ValidationFailed.New("fromTemplateId and toVersion are required")
	}

	// The body is optional; only decode overrides when one is present.
	var overrides *api.CreateLLMProviderTemplateVersionRequest
	if r.Body != nil && r.ContentLength != 0 {
		var vreq api.CreateLLMProviderTemplateVersionRequest
		if err := json.NewDecoder(r.Body).Decode(&vreq); err != nil && !errors.Is(err, io.EOF) {
			return apperror.ValidationFailed.Wrap(err, "Invalid request body")
		}
		overrides = &vreq
	}

	created, err := h.templateService.CopyVersion(orgID, fromTemplateID, toTemplateID, toVersion, createdBy, overrides)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			return apperror.LLMProviderTemplateVersionNotFound.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderTemplateVersionExists):
			return apperror.LLMProviderTemplateVersionExists.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderTemplateManagedByReserved):
			return apperror.LLMProviderTemplateManagedByReserved.Wrap(err)
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid input. toVersion must match the v<major>.<minor> pattern starting from v1.0 (e.g. v1.0), and toTemplateId must match the family")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to copy LLM provider template version from %s to version %s in org %s", fromTemplateID, toVersion, orgID))
		}
	}

	setLocation(w, "llm-provider-templates", strOrEmpty(created.Id))
	httputil.WriteJSON(w, http.StatusCreated, created)
	return nil
}

func (h *LLMHandler) ListLLMProviderTemplates(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
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
			return apperror.ValidationFailed.New("Invalid groupId")
		}
		version := q.Version
		if version != "" {
			resp, err := h.templateService.GetVersion(orgID, groupID, version)
			if err != nil {
				switch {
				case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
					return apperror.LLMProviderTemplateVersionNotFound.Wrap(err)
				case errors.Is(err, constants.ErrInvalidInput):
					return apperror.ValidationFailed.Wrap(err, "Invalid version. Version must match the v<major>.<minor> pattern (e.g. v1.0)")
				default:
					return apperror.Internal.Wrap(err).
						WithLogMessage(fmt.Sprintf("failed to get LLM provider template version %s of group %s in org %s", version, groupID, orgID))
				}
			}
			httputil.WriteJSON(w, http.StatusOK, resp)
			return nil
		}
		resp, err := h.templateService.ListVersions(orgID, groupID, limit, offset)
		if err != nil {
			switch {
			case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
				return apperror.LLMProviderTemplateNotFound.Wrap(err)
			case errors.Is(err, constants.ErrInvalidInput):
				return apperror.ValidationFailed.Wrap(err, "Invalid groupId")
			default:
				return apperror.Internal.Wrap(err).
					WithLogMessage(fmt.Sprintf("failed to list LLM provider template versions of group %s in org %s", groupID, orgID))
			}
		}
		httputil.WriteJSON(w, http.StatusOK, resp)
		return nil
	}

	latestOnly := q.Latest

	resp, err := h.templateService.List(orgID, limit, offset, latestOnly)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to list LLM provider templates in org %s", orgID))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *LLMHandler) GetLLMProviderTemplate(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("llmProviderTemplateId")

	resp, err := h.templateService.Get(orgID, id)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			return apperror.LLMProviderTemplateNotFound.Wrap(err)
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid template id")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to get LLM provider template %s in org %s", id, orgID))
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *LLMHandler) SetLLMProviderTemplateVersionEnabled(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("llmProviderTemplateId")

	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		return apperror.ValidationFailed.New("Request body must include a boolean 'enabled' field")
	}

	resp, err := h.templateService.SetEnabledByHandle(orgID, id, *body.Enabled)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			return apperror.LLMProviderTemplateVersionNotFound.Wrap(err)
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid template id")
		case errors.Is(err, constants.ErrLLMProviderTemplateInUse):
			return apperror.LLMProviderTemplateInUse.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderTemplateNotToggleable):
			return apperror.LLMProviderTemplateNotToggleable.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to set LLM provider template version enabled for template %s in org %s", id, orgID))
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *LLMHandler) UpdateLLMProviderTemplate(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("llmProviderTemplateId")

	var req api.LLMProviderTemplate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body")
	}

	if err := utils.ValidateHandleImmutable(id, req.Id); err != nil {
		return apperror.ValidationFailed.Wrap(err, "LLM provider template id is immutable and cannot be changed")
	}

	updatedBy, err := resolveActorErr(r, h.identity, "update LLM provider template")
	if err != nil {
		return err
	}
	resp, err := h.templateService.Update(orgID, id, updatedBy, &req)
	if err != nil {
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			return apperror.LLMProviderTemplateNotFound.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderTemplateReadOnly):
			return apperror.LLMProviderTemplateReadOnly.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderTemplateManagedByReserved):
			return apperror.LLMProviderTemplateManagedByReserved.Wrap(err)
		case errors.Is(err, constants.ErrHandleImmutable):
			return apperror.ValidationFailed.Wrap(err, "The id is immutable and cannot be changed")
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid input")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to update LLM provider template %s in org %s", id, orgID))
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// DeleteLLMProviderTemplateVersion removes a single version of a template.
func (h *LLMHandler) DeleteLLMProviderTemplateVersion(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("llmProviderTemplateId")

	if err := h.templateService.DeleteByHandle(orgID, id); err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			return apperror.LLMProviderTemplateVersionNotFound.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderTemplateInUse):
			return apperror.LLMProviderTemplateInUse.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderTemplateReadOnly):
			return apperror.LLMProviderTemplateReadOnly.Wrap(err)
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid template id")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to delete LLM provider template version %s in org %s", id, orgID))
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ---- Providers ----

func (h *LLMHandler) CreateLLMProvider(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var req api.LLMProvider
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body")
	}
	createdBy, err := resolveActorErr(r, h.identity, "create LLM provider")
	if err != nil {
		return err
	}

	created, err := h.providerService.Create(orgID, createdBy, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderLimitReached):
			return apperror.LLMProviderLimitReached.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderExists):
			return apperror.LLMProviderExists.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			return apperror.LLMProviderTemplateRefNotFound.Wrap(err)
		case errors.Is(err, constants.ErrSecretRefMissing):
			return apperror.ValidationFailed.Wrap(err, "One or more referenced secrets do not exist")
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid input")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to create LLM provider in org %s", orgID))
		}
	}
	setLocation(w, "llm-providers", strOrEmpty(created.Id))
	httputil.WriteJSON(w, http.StatusCreated, created)
	return nil
}

func (h *LLMHandler) ListLLMProviders(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
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
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to list LLM providers in org %s", orgID))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *LLMHandler) GetLLMProvider(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("llmProviderId")

	resp, err := h.providerService.Get(orgID, id)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			return apperror.LLMProviderNotFound.Wrap(err)
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid provider id")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to get LLM provider %s in org %s", id, orgID))
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *LLMHandler) UpdateLLMProvider(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("llmProviderId")

	var req api.LLMProvider
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body")
	}

	if err := utils.ValidateHandleImmutable(id, req.Id); err != nil {
		return apperror.ValidationFailed.Wrap(err, "LLM provider id is immutable and cannot be changed")
	}

	updatedBy, err := resolveActorErr(r, h.identity, "update LLM provider")
	if err != nil {
		return err
	}
	resp, err := h.providerService.Update(orgID, id, updatedBy, &req)
	if err != nil {
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			return apperror.LLMProviderNotFound.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			return apperror.LLMProviderTemplateRefNotFound.Wrap(err)
		case errors.Is(err, constants.ErrSecretRefMissing):
			return apperror.ValidationFailed.Wrap(err, "One or more referenced secrets do not exist")
		case errors.Is(err, constants.ErrHandleImmutable):
			return apperror.ValidationFailed.Wrap(err, "The id is immutable and cannot be changed")
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid input")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to update LLM provider %s in org %s", id, orgID))
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *LLMHandler) DeleteLLMProvider(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("llmProviderId")
	deletedBy, err := resolveActorErr(r, h.identity, "delete LLM provider")
	if err != nil {
		return err
	}

	if err := h.providerService.Delete(orgID, id, deletedBy); err != nil {
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			return apperror.LLMProviderNotFound.Wrap(err)
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid provider id")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to delete LLM provider %s in org %s", id, orgID))
		}
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ---- Proxies ----

func (h *LLMHandler) CreateLLMProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var req api.LLMProxy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body")
	}
	if req.ProjectId == "" {
		return apperror.ValidationFailed.New("Project ID is required")
	}
	createdBy, err := resolveActorErr(r, h.identity, "create LLM proxy")
	if err != nil {
		return err
	}

	created, err := h.proxyService.Create(orgID, createdBy, &req)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyLimitReached):
			return apperror.LLMProxyLimitReached.Wrap(err)
		case errors.Is(err, constants.ErrLLMProxyExists):
			return apperror.LLMProxyExists.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			return apperror.LLMProviderRefNotFound.Wrap(err)
		case errors.Is(err, constants.ErrProjectNotFound):
			return apperror.ProjectNotFound.Wrap(err)
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid input")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to create LLM proxy in org %s", orgID))
		}
	}
	setLocation(w, "llm-proxies", strOrEmpty(created.Id))
	httputil.WriteJSON(w, http.StatusCreated, created)
	return nil
}

func (h *LLMHandler) ListLLMProxies(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectID == "" {
		return apperror.ValidationFailed.New("projectId query parameter is required")
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
			return apperror.ProjectNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to list LLM proxies in org %s", orgID))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *LLMHandler) ListLLMProxiesByProvider(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
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
			return apperror.LLMProviderNotFound.Wrap(err)
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid provider id")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to list LLM proxies by provider %s in org %s", providerID, orgID))
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *LLMHandler) GetLLMProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("llmProxyId")

	resp, err := h.proxyService.Get(orgID, id)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			return apperror.LLMProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid proxy id")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to get LLM proxy %s in org %s", id, orgID))
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *LLMHandler) UpdateLLMProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("llmProxyId")

	var req api.LLMProxy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body")
	}

	if err := utils.ValidateHandleImmutable(id, req.Id); err != nil {
		return apperror.ValidationFailed.Wrap(err, "LLM proxy id is immutable and cannot be changed")
	}

	updatedBy, err := resolveActorErr(r, h.identity, "update LLM proxy")
	if err != nil {
		return err
	}
	resp, err := h.proxyService.Update(orgID, id, updatedBy, &req)
	if err != nil {
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			return apperror.LLMProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			return apperror.LLMProviderRefNotFound.Wrap(err)
		case errors.Is(err, constants.ErrHandleImmutable):
			return apperror.ValidationFailed.Wrap(err, "The id is immutable and cannot be changed")
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid input")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to update LLM proxy %s in org %s", id, orgID))
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *LLMHandler) DeleteLLMProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("llmProxyId")
	deletedBy, err := resolveActorErr(r, h.identity, "delete LLM proxy")
	if err != nil {
		return err
	}

	if err := h.proxyService.Delete(orgID, id, deletedBy); err != nil {
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			return apperror.LLMProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.ValidationFailed.Wrap(err, "Invalid proxy id")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to delete LLM proxy %s in org %s", id, orgID))
		}
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

type OrganizationHandler struct {
	orgService *service.OrganizationService
	identity   *service.IdentityService
	slogger    *slog.Logger
}

func NewOrganizationHandler(orgService *service.OrganizationService, identity *service.IdentityService, slogger *slog.Logger) *OrganizationHandler {
	return &OrganizationHandler{
		orgService: orgService,
		identity:   identity,
		slogger:    slogger,
	}
}

// RegisterOrganization handles POST /api/v0.9/organizations
func (h *OrganizationHandler) RegisterOrganization(w http.ResponseWriter, r *http.Request) {
	var req api.Organization

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
	if req.DisplayName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"displayName is required"))
		return
	}
	if req.Region == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Region is required"))
		return
	}

	// UUID is always server-generated
	id, genErr := utils.GenerateUUID()
	if genErr != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to generate organization ID"))
		return
	}

	// Extract handle from id field (optional — auto-generated from displayName if absent)
	var handle string
	if req.Id != nil {
		handle = *req.Id
	}

	performedBy, ok := resolveActor(w, r, h.identity, h.slogger, "register organization")
	if !ok {
		return
	}
	// The IDP's organization UUID is derived server-side from the token's raw
	// organization claim (the IDP org id), never client-supplied. This uses the
	// unresolved claim rather than the resolved platform UUID, since the
	// organization being created does not exist yet. Empty in file-based mode.
	idpOrgRefUUID, _ := middleware.GetIdpOrgRefFromRequest(r)
	org, err := h.orgService.RegisterOrganization(id, handle, req.DisplayName, req.Region, idpOrgRefUUID, performedBy)
	if err != nil {
		if errors.Is(err, constants.ErrHandleExists) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Organization already exists"))
			return
		}
		if errors.Is(err, constants.ErrOrganizationExists) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Organization with the given ID already exists"))
			return
		}
		h.slogger.Error("Failed to create organization", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create organization"))
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, org)
}

// HeadOrganization handles HEAD /api/v0.9/organizations/{organizationId}
func (h *OrganizationHandler) HeadOrganization(w http.ResponseWriter, r *http.Request) {
	organizationIdFromContext, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	handle := r.PathValue("organizationId")

	h.slogger.Debug("Organization from token", "organizationId", organizationIdFromContext)
	// to do: enable this check after finalizing authentication method

	// if orgID != organizationIdFromContext {
	// 	httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden",
	// 		"Organization ID in token does not match the requested organization ID"))
	// 	return
	// }

	_, err := h.orgService.GetOrganizationByHandle(handle)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetOrganizationByID handles GET /api/v0.9/organizations/{organizationId}
func (h *OrganizationHandler) GetOrganizationByID(w http.ResponseWriter, r *http.Request) {
	handle := r.PathValue("organizationId")

	org, err := h.orgService.GetOrganizationByHandle(handle)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Organization not found"))
			return
		}
		h.slogger.Error("Failed to get organization by handle", "handle", handle, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get organization"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, org)
}

// ListOrganizations handles GET /api/v0.9/organizations
func (h *OrganizationHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}

	offset := 0
	if v := strings.TrimSpace(r.URL.Query().Get("offset")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			offset = parsed
		}
	}

	orgs, total, err := h.orgService.ListOrganizations(limit, offset)
	if err != nil {
		h.slogger.Error("Failed to list organizations", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to list organizations"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, api.OrganizationListResponse{
		Count: len(orgs),
		List:  orgs,
		Pagination: api.Pagination{
			Total:  total,
			Offset: offset,
			Limit:  limit,
		},
	})
}

func (h *OrganizationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/organizations", h.RegisterOrganization)
	mux.HandleFunc("GET "+constants.APIBasePath+"/organizations", h.ListOrganizations)
	mux.HandleFunc("HEAD "+constants.APIBasePath+"/organizations/{organizationId}", h.HeadOrganization)
	mux.HandleFunc("GET "+constants.APIBasePath+"/organizations/{organizationId}", h.GetOrganizationByID)
}

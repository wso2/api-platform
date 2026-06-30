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

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

type OrganizationHandler struct {
	orgService *service.OrganizationService
	slogger    *slog.Logger
}

func NewOrganizationHandler(orgService *service.OrganizationService, slogger *slog.Logger) *OrganizationHandler {
	return &OrganizationHandler{
		orgService: orgService,
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

	performedBy, _ := middleware.GetUserIDFromRequest(r)
	org, err := h.orgService.RegisterOrganization(id, handle, req.DisplayName, req.Region, performedBy)
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

// HeadOrganizationByUuid handles HEAD /api/v0.9/organizations/{uuid}
func (h *OrganizationHandler) HeadOrganizationByUuid(w http.ResponseWriter, r *http.Request) {
	organizationIdFromContext, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	orgID := r.PathValue("uuid")

	h.slogger.Debug("Organization from token", "organizationId", organizationIdFromContext)
	// to do: enable this check after finalizing authentication method

	// if orgID != organizationIdFromContext {
	// 	httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden",
	// 		"Organization ID in token does not match the requested organization ID"))
	// 	return
	// }

	_, err := h.orgService.GetOrganizationByUUID(orgID)
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

// GetOrganizationByUUID handles GET /api/v0.9/organizations/{uuid}
func (h *OrganizationHandler) GetOrganizationByUUID(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("uuid")

	org, err := h.orgService.GetOrganizationByUUID(orgID)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Organization not found"))
			return
		}
		h.slogger.Error("Failed to get organization by UUID", "organizationId", orgID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get organization"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, org)
}

// GetOrganization handles GET /api/v0.9/organizations
func (h *OrganizationHandler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	org, err := h.orgService.GetOrganizationByUUID(orgID)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Organization not found"))
			return
		}
		if errors.Is(err, constants.ErrMultipleOrganizations) {
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Data integrity error: multiple organizations found"))
			return
		}
		h.slogger.Error("Failed to get organization", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get organization"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, org)
}

func (h *OrganizationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/organizations", h.RegisterOrganization)
	mux.HandleFunc("GET "+constants.APIBasePath+"/organizations", h.GetOrganization)
	mux.HandleFunc("HEAD "+constants.APIBasePath+"/organizations/{uuid}", h.HeadOrganizationByUuid)
	mux.HandleFunc("GET "+constants.APIBasePath+"/organizations/{uuid}", h.GetOrganizationByUUID)
}

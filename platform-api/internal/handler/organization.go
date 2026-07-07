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
	"fmt"
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
func (h *OrganizationHandler) RegisterOrganization(w http.ResponseWriter, r *http.Request) error {
	var req api.Organization

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body")
	}

	// Validate required fields
	if req.DisplayName == "" {
		return apperror.ValidationFailed.New("displayName is required")
	}
	if req.Region == "" {
		return apperror.ValidationFailed.New("Region is required")
	}

	// UUID is always server-generated
	id, genErr := utils.GenerateUUID()
	if genErr != nil {
		return apperror.Internal.Wrap(genErr).
			WithLogMessage("failed to generate organization ID")
	}

	// Extract handle from id field (optional — auto-generated from displayName if absent)
	var handle string
	if req.Id != nil {
		handle = *req.Id
	}

	performedBy, err := resolveActorErr(r, h.identity, "register organization")
	if err != nil {
		return err
	}
	// The IDP's organization UUID is derived server-side from the token's raw
	// organization claim (the IDP org id), never client-supplied. This uses the
	// unresolved claim rather than the resolved platform UUID, since the
	// organization being created does not exist yet. Empty in file-based mode.
	idpOrgRefUUID, _ := middleware.GetIdpOrgRefFromRequest(r)
	org, err := h.orgService.RegisterOrganization(id, handle, req.DisplayName, req.Region, idpOrgRefUUID, performedBy)
	if err != nil {
		if errors.Is(err, constants.ErrHandleExists) {
			return apperror.OrganizationExists.Wrap(err)
		}
		if errors.Is(err, constants.ErrOrganizationExists) {
			return apperror.OrganizationExists.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage("failed to create organization")
	}

	httputil.WriteJSON(w, http.StatusCreated, org)
	return nil
}

// HeadOrganization handles HEAD /api/v0.9/organizations/{organizationId}
func (h *OrganizationHandler) HeadOrganization(w http.ResponseWriter, r *http.Request) error {
	organizationIdFromContext, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	handle := r.PathValue("organizationId")

	h.slogger.Debug("Organization from token", "organizationId", organizationIdFromContext)

	if handle != organizationIdFromContext {
		return apperror.Forbidden.New().
			WithLogMessage("Organization ID in token does not match the requested organization ID")
	}

	_, err := h.orgService.GetOrganizationByHandle(handle)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			return apperror.OrganizationNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to get organization by handle %s", handle))
	}

	w.WriteHeader(http.StatusOK)
	return nil
}

// GetOrganizationByID handles GET /api/v0.9/organizations/{organizationId}
func (h *OrganizationHandler) GetOrganizationByID(w http.ResponseWriter, r *http.Request) error {
	handle := r.PathValue("organizationId")

	org, err := h.orgService.GetOrganizationByHandle(handle)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			return apperror.OrganizationNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to get organization by handle %s", handle))
	}

	httputil.WriteJSON(w, http.StatusOK, org)
	return nil
}

// ListOrganizations handles GET /api/v0.9/organizations
func (h *OrganizationHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) error {
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
		return apperror.Internal.Wrap(err).
			WithLogMessage("failed to list organizations")
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
	return nil
}

func (h *OrganizationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/organizations", middleware.MapErrors(h.slogger, h.RegisterOrganization))
	mux.HandleFunc("GET "+constants.APIBasePath+"/organizations", middleware.MapErrors(h.slogger, h.ListOrganizations))
	mux.HandleFunc("HEAD "+constants.APIBasePath+"/organizations/{organizationId}", middleware.MapErrors(h.slogger, h.HeadOrganization))
	mux.HandleFunc("GET "+constants.APIBasePath+"/organizations/{organizationId}", middleware.MapErrors(h.slogger, h.GetOrganizationByID))
}

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
	authzMode  string
	slogger    *slog.Logger
}

func NewOrganizationHandler(orgService *service.OrganizationService, identity *service.IdentityService, authzMode string, slogger *slog.Logger) *OrganizationHandler {
	return &OrganizationHandler{
		orgService: orgService,
		identity:   identity,
		authzMode:  authzMode,
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
	// The IDP's organization UUID is derived server-side from the token's
	// organization claim, never client-supplied. Since the organization being
	// created does not exist yet, OrganizationResolverMiddleware has nothing to
	// resolve the claim against, so this still reflects the raw IDP org id.
	// Empty in file-based mode.
	idpOrgRefUUID, _ := middleware.GetOrganizationFromRequest(r)
	org, err := h.orgService.RegisterOrganization(id, handle, req.DisplayName, req.Region, idpOrgRefUUID, performedBy)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage("failed to create organization")
	}

	setLocation(w, "organizations", strOrEmpty(org.Id))
	httputil.WriteJSON(w, http.StatusCreated, org)
	return nil
}

// resolveOrganizationForRequest resolves the organization strictly from the trusted
// token/context organization ID (a UUID), never from the user-supplied path handle,
// then verifies the path handle refers to that same organization. Comparing a handle
// field against the context's UUID directly is always false and would forbid every
// request, so the lookup must go through the UUID first.
//
// Returns apperror.Unauthorized if the token has no organization claim,
// apperror.OrganizationNotFound (404) if the token's organization no longer exists,
// and apperror.Forbidden (403) if pathHandle does not match the caller's organization.
func (h *OrganizationHandler) resolveOrganizationForRequest(r *http.Request, pathHandle string) (*api.Organization, error) {
	organizationIdFromContext, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return nil, apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	org, err := h.orgService.GetOrganizationByUUID(organizationIdFromContext)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, apperror.Internal.Wrap(err).
			WithLogMessage("failed to get organization from token context")
	}

	if org.Id == nil || *org.Id != pathHandle {
		return nil, apperror.Forbidden.New().
			WithLogMessage("organization in token does not match the requested organization")
	}

	return org, nil
}

// HeadOrganization handles HEAD /api/v0.9/organizations/{organizationId}
func (h *OrganizationHandler) HeadOrganization(w http.ResponseWriter, r *http.Request) error {
	handle := r.PathValue("organizationId")

	if _, err := h.resolveOrganizationForRequest(r, handle); err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	return nil
}

// GetOrganizationByID handles GET /api/v0.9/organizations/{organizationId}
func (h *OrganizationHandler) GetOrganizationByID(w http.ResponseWriter, r *http.Request) error {
	handle := r.PathValue("organizationId")

	org, err := h.resolveOrganizationForRequest(r, handle)
	if err != nil {
		return err
	}

	httputil.WriteJSON(w, http.StatusOK, org)
	return nil
}

// ListOrganizations handles GET /api/v0.9/organizations
func (h *OrganizationHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) error {
	limit, offset := parsePagination(r)

	// resolveActorErr also creates the caller's user_idp_references row on first
	// use, which is what makes the membership heal below FK-safe.
	performedBy, err := resolveActorErr(r, h.identity, "list organizations")
	if err != nil {
		return err
	}

	var orgs []api.Organization
	var total int
	if middleware.HasEffectiveScope(r, h.authzMode, "ap:organization:manage") {
		orgs, total, err = h.orgService.ListOrganizations(limit, offset)
	} else {
		resolvedOrgUUID, _ := middleware.GetOrganizationFromRequest(r)
		orgs, total, err = h.orgService.ListOrganizationsForUser(performedBy, resolvedOrgUUID, limit, offset)
	}
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

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
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/api-platform/httpkit/httputil"
)

// APIPortalHandler exposes /api-portals CRUD. The generated OpenAPI types
// (api.CreateApiPortalRequest / api.ApiPortalResponse / …) are the wire contract
// AND the service-layer contract — the service speaks in these directly so its
// methods also satisfy pdk.APIPortals for plugins.
type APIPortalHandler struct {
	svc      *service.APIPortalService
	identity *service.IdentityService
	slogger  *slog.Logger
}

// NewAPIPortalHandler constructs an APIPortalHandler.
func NewAPIPortalHandler(svc *service.APIPortalService, identity *service.IdentityService, slogger *slog.Logger) *APIPortalHandler {
	return &APIPortalHandler{svc: svc, identity: identity, slogger: slogger}
}

// CreateAPIPortal — POST /api-portals
func (h *APIPortalHandler) CreateAPIPortal(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	var req api.CreateApiPortalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	createdBy, err := resolveActorErr(r, h.identity, "create api portal")
	if err != nil {
		return err
	}

	resp, err := h.svc.CreateAPIPortal(&req, orgID, createdBy)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to create api portal %q for org %s by user %s", req.Handle, orgID, createdBy))
	}

	setLocation(w, "api-portals", derefStr(resp.Handle))
	httputil.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

// GetAPIPortal — GET /api-portals/{apiPortalId}
func (h *APIPortalHandler) GetAPIPortal(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	handle := strings.TrimSpace(r.PathValue("apiPortalId"))
	if handle == "" {
		return apperror.ValidationFailed.New("API Portal ID is required")
	}

	resp, err := h.svc.GetAPIPortal(handle, orgID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get api portal %q in org %s", handle, orgID))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// ListAPIPortals — GET /api-portals
func (h *APIPortalHandler) ListAPIPortals(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	opts := parseListOptions(r)

	resp, err := h.svc.ListAPIPortals(orgID, opts.Limit, opts.Offset, opts.SortBy, opts.SortOrder, opts.Search)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to list api portals for org %s", orgID))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// UpdateAPIPortal — PUT /api-portals/{apiPortalId}
func (h *APIPortalHandler) UpdateAPIPortal(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	handle := strings.TrimSpace(r.PathValue("apiPortalId"))
	if handle == "" {
		return apperror.ValidationFailed.New("API Portal ID is required")
	}

	var req api.UpdateApiPortalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	updatedBy, err := resolveActorErr(r, h.identity, "update api portal")
	if err != nil {
		return err
	}

	resp, err := h.svc.UpdateAPIPortal(handle, &req, orgID, updatedBy)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to update api portal %q in org %s by user %s", handle, orgID, updatedBy))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// DeleteAPIPortal — DELETE /api-portals/{apiPortalId}
func (h *APIPortalHandler) DeleteAPIPortal(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	handle := strings.TrimSpace(r.PathValue("apiPortalId"))
	if handle == "" {
		return apperror.ValidationFailed.New("API Portal ID is required")
	}

	actor, err := resolveActorErr(r, h.identity, "delete api portal")
	if err != nil {
		return err
	}

	if err := h.svc.DeleteAPIPortal(handle, orgID, actor); err != nil {
		return serviceError(err, fmt.Sprintf("failed to delete api portal %q in org %s by user %s", handle, orgID, actor))
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// RegisterRoutes wires all /api-portals routes onto the shared mux.
func (h *APIPortalHandler) RegisterRoutes(mux router.Router) {
	base := constants.APIBasePath + "/api-portals"
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.CreateAPIPortal))
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.ListAPIPortals))
	mux.HandleFunc("GET "+base+"/{apiPortalId}", middleware.MapErrors(h.slogger, h.GetAPIPortal))
	mux.HandleFunc("PUT "+base+"/{apiPortalId}", middleware.MapErrors(h.slogger, h.UpdateAPIPortal))
	mux.HandleFunc("DELETE "+base+"/{apiPortalId}", middleware.MapErrors(h.slogger, h.DeleteAPIPortal))
}

// derefStr returns the pointed-to string or "" when nil. Local helper used by
// setLocation to source the Location header from the api-generated response.
func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

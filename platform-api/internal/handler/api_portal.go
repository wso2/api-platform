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
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/go-httpkit/httputil"
)

// APIPortalHandler exposes /api-portals CRUD. The generated OpenAPI types
// (api.CreateApiPortalRequest / api.ApiPortalResponse / …) are the wire contract;
// this file only translates between them and the service layer.
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

	svcReq := &service.CreateAPIPortalRequest{
		Handle:      strings.TrimSpace(req.Handle),
		Name:        strings.TrimSpace(req.Name),
		Description: deref(req.Description),
		URL:         req.Url,
		AuthType:    string(req.AuthType),
		AuthConfig:  authConfigStructToMap(req.AuthConfig),
		Metadata:    derefMetadata(req.Metadata),
	}
	portal, err := h.svc.CreateAPIPortal(svcReq, orgID, createdBy)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to create api portal %q for org %s by user %s", svcReq.Handle, orgID, createdBy))
	}

	setLocation(w, "api-portals", portal.Handle)
	httputil.WriteJSON(w, http.StatusCreated, modelToAPIPortalResponse(portal))
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

	portal, err := h.svc.GetAPIPortal(handle, orgID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get api portal %q in org %s", handle, orgID))
	}
	httputil.WriteJSON(w, http.StatusOK, modelToAPIPortalResponse(portal))
	return nil
}

// ListAPIPortals — GET /api-portals
func (h *APIPortalHandler) ListAPIPortals(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	opts := service.APIPortalListOptions{ListOptions: parseListOptions(r)}

	resp, err := h.svc.ListAPIPortals(orgID, opts)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to list api portals for org %s", orgID))
	}
	httputil.WriteJSON(w, http.StatusOK, apiPortalListResponse(resp))
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

	svcReq := &service.UpdateAPIPortalRequest{
		Name:        req.Name,
		Description: req.Description,
		URL:         req.Url,
		AuthConfig:  authConfigStructToMap(req.AuthConfig),
		Metadata:    derefMetadata(req.Metadata),
	}
	if req.AuthType != nil {
		v := string(*req.AuthType)
		svcReq.AuthType = &v
	}

	portal, err := h.svc.UpdateAPIPortal(handle, svcReq, orgID, updatedBy)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to update api portal %q in org %s by user %s", handle, orgID, updatedBy))
	}
	httputil.WriteJSON(w, http.StatusOK, modelToAPIPortalResponse(portal))
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

// --- translation helpers ---

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// derefMetadata converts the generated Metadata type (a map alias) into a plain
// map[string]interface{} for the service layer, dropping the nil pointer.
func derefMetadata(m *api.ApiPortalMetadata) map[string]interface{} {
	if m == nil {
		return nil
	}
	return map[string]interface{}(*m)
}

// authConfigStructToMap flattens the generated ApiPortalAuthConfig struct into
// the map shape the service layer expects. Nil pointer fields are dropped so
// downstream validation sees "missing" (rather than "present but empty").
func authConfigStructToMap(c *api.ApiPortalAuthConfig) map[string]interface{} {
	if c == nil {
		return nil
	}
	out := map[string]interface{}{}
	if c.StsTokenUrl != nil {
		out[constants.APIPortalAuthConfigKeySTSTokenURL] = *c.StsTokenUrl
	}
	if c.ClientId != nil {
		out[constants.APIPortalAuthConfigKeyClientID] = *c.ClientId
	}
	if c.ClientSecret != nil {
		out[constants.APIPortalAuthConfigKeyClientSecret] = *c.ClientSecret
	}
	return out
}

// stripSensitiveAuthConfig deletes any keys that carry secret material before
// the config leaves the server. Belt-and-suspenders alongside the OAS
// `writeOnly: true` marker on ClientSecret — even if a client somehow round-
// trips a plaintext secret through storage (e.g. during migration or if the
// storage-encrypt step is ever skipped), the response strip guarantees it
// never appears on the wire.
func stripSensitiveAuthConfig(cfg map[string]interface{}) map[string]interface{} {
	if cfg == nil {
		return nil
	}
	out := make(map[string]interface{}, len(cfg))
	for k, v := range cfg {
		out[k] = v
	}
	for _, key := range constants.APIPortalAuthConfigSensitiveKeys {
		delete(out, key)
	}
	return out
}

// mapToAuthConfigStruct rebuilds the generated struct from the stored map for
// response serialization. Sensitive keys are stripped first, so the generated
// ClientSecret pointer stays nil (and — since it's marked omitempty — won't
// appear in the JSON output).
func mapToAuthConfigStruct(m map[string]interface{}) *api.ApiPortalAuthConfig {
	stripped := stripSensitiveAuthConfig(m)
	if stripped == nil {
		return nil
	}
	c := &api.ApiPortalAuthConfig{}
	if v, ok := stripped[constants.APIPortalAuthConfigKeySTSTokenURL].(string); ok && v != "" {
		s := v
		c.StsTokenUrl = &s
	}
	if v, ok := stripped[constants.APIPortalAuthConfigKeyClientID].(string); ok && v != "" {
		s := v
		c.ClientId = &s
	}
	// ClientSecret is intentionally never populated on the response side.
	return c
}

func modelToAPIPortalResponse(p *model.APIPortal) *api.ApiPortalResponse {
	if p == nil {
		return nil
	}
	id := p.Handle
	handle := p.Handle
	createdAt := p.CreatedAt
	updatedAt := p.UpdatedAt

	resp := &api.ApiPortalResponse{
		Id:        &id,
		Handle:    &handle,
		Name:      p.Name,
		Url:       p.URL,
		AuthType:  api.ApiPortalResponseAuthType(p.AuthType),
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}
	if p.Description != "" {
		desc := p.Description
		resp.Description = &desc
	}
	if p.AuthConfig != nil {
		resp.AuthConfig = mapToAuthConfigStruct(p.AuthConfig)
	}
	if p.Metadata != nil {
		m := api.ApiPortalMetadata(p.Metadata)
		resp.Metadata = &m
	}
	return resp
}

func modelToAPIPortalListItem(p *model.APIPortal) api.ApiPortalListItem {
	item := api.ApiPortalListItem{
		Id:        p.Handle,
		Handle:    p.Handle,
		Name:      p.Name,
		Url:       p.URL,
		AuthType:  api.ApiPortalListItemAuthType(p.AuthType),
		CreatedAt: p.CreatedAt,
	}
	if p.Description != "" {
		desc := p.Description
		item.Description = &desc
	}
	return item
}

func apiPortalListResponse(resp *service.APIPortalListResponse) *api.ApiPortalListResponse {
	out := &api.ApiPortalListResponse{
		Count: resp.Count,
		List:  make([]api.ApiPortalListItem, 0, len(resp.List)),
		Pagination: api.Pagination{
			Total:  resp.Pagination.Total,
			Offset: resp.Pagination.Offset,
			Limit:  resp.Pagination.Limit,
		},
	}
	for _, p := range resp.List {
		out.List = append(out.List, modelToAPIPortalListItem(p))
	}
	return out
}

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
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/api-platform/httpkit/httputil"
)

// GraphQLAPIHandler handles CRUD routes for GraphQL APIs. GraphQL is a core
// artifact kind (like RestApi/LlmProvider/LlmProxy/Mcp), so this handler is
// wired into the server the same way APIHandler/MCPProxyHandler are, not via
// a plugin.
type GraphQLAPIHandler struct {
	graphqlAPIService *service.GraphQLAPIService
	identity          *service.IdentityService
	slogger           *slog.Logger
}

// NewGraphQLAPIHandler creates a new GraphQLAPIHandler instance.
func NewGraphQLAPIHandler(graphqlAPIService *service.GraphQLAPIService, identity *service.IdentityService, slogger *slog.Logger) *GraphQLAPIHandler {
	return &GraphQLAPIHandler{
		graphqlAPIService: graphqlAPIService,
		identity:          identity,
		slogger:           slogger,
	}
}

// CreateGraphQLAPI handles POST /api/v0.9/graphql-apis and creates a new GraphQL API.
func (h *GraphQLAPIHandler) CreateGraphQLAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var req api.CreateGraphQLAPIRequest
	if err := decodeCreateGraphQLAPIRequest(r, &req); err != nil {
		return apperror.NewValidation(err)
	}

	if req.DisplayName == "" {
		return apperror.ValidationFailed.New("API name is required")
	}
	if req.Context == "" {
		return apperror.ValidationFailed.New("API context is required")
	}
	if req.Version == "" {
		return apperror.ValidationFailed.New("API version is required")
	}
	if strings.TrimSpace(req.ProjectId) == "" {
		return apperror.ValidationFailed.New("Project ID is required")
	}

	createdBy, err := resolveActorErr(r, h.identity, "create GraphQL API")
	if err != nil {
		return err
	}
	apiResponse, err := h.graphqlAPIService.Create(orgId, createdBy, &req)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to create GraphQL API in org %s", orgId))
	}

	setLocation(w, "graphql-apis", strOrEmpty(apiResponse.Id))
	httputil.WriteJSON(w, http.StatusCreated, apiResponse)
	return nil
}

// GetGraphQLAPI handles GET /api/v0.9/graphql-apis/:graphqlApiId and retrieves a GraphQL API by its handle.
func (h *GraphQLAPIHandler) GetGraphQLAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("graphqlApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	apiResponse, err := h.graphqlAPIService.GetDetail(orgId, apiId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get GraphQL API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, apiResponse)
	return nil
}

// GetGraphQLAPISDL handles GET /api/v0.9/graphql-apis/:graphqlApiId/sdl and
// retrieves a GraphQL API's resolved SDL text — split out from
// GetGraphQLAPI's response since sdl can be large and most callers only need
// the metadata.
func (h *GraphQLAPIHandler) GetGraphQLAPISDL(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("graphqlApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	sdl, err := h.graphqlAPIService.GetSDL(orgId, apiId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get GraphQL API %s SDL in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, api.GraphQLAPISDLResponse{Sdl: sdl})
	return nil
}

// ListGraphQLAPIs handles GET /api/v0.9/graphql-apis and lists GraphQL APIs for an organization filtered by project.
func (h *GraphQLAPIHandler) ListGraphQLAPIs(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	projectId := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectId == "" {
		return apperror.ValidationFailed.New("projectId query parameter is required")
	}

	opts := parseListOptions(r)

	resp, err := h.graphqlAPIService.List(orgId, projectId, opts)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get GraphQL APIs for project %s in org %s", projectId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// UpdateGraphQLAPI handles PUT /api/v0.9/graphql-apis/:graphqlApiId and updates an existing GraphQL API.
func (h *GraphQLAPIHandler) UpdateGraphQLAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("graphqlApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	var req api.GraphQLAPI
	if err := decodeUpdateGraphQLAPIRequest(r, &req); err != nil {
		return apperror.NewValidation(err)
	}

	updatedBy, err := resolveActorErr(r, h.identity, "update GraphQL API")
	if err != nil {
		return err
	}
	apiResponse, err := h.graphqlAPIService.Update(orgId, apiId, updatedBy, &req)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to update GraphQL API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, apiResponse)
	return nil
}

// DeleteGraphQLAPI handles DELETE /api/v0.9/graphql-apis/:graphqlApiId and deletes a GraphQL API by its handle.
func (h *GraphQLAPIHandler) DeleteGraphQLAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("graphqlApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	deletedBy, err := resolveActorErr(r, h.identity, "delete GraphQL API")
	if err != nil {
		return err
	}
	if err := h.graphqlAPIService.Delete(orgId, apiId, deletedBy); err != nil {
		return serviceError(err, fmt.Sprintf("failed to delete GraphQL API %s in org %s", apiId, orgId))
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// AddGatewaysToAPI handles POST /api/v0.9/graphql-apis/:graphqlApiId/gateways to
// associate gateways with a GraphQL API. Mirrors APIHandler.AddGatewaysToAPI.
func (h *GraphQLAPIHandler) AddGatewaysToAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("graphqlApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	var req []api.AddGatewayToRESTAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	if len(req) == 0 {
		return apperror.ValidationFailed.New("At least one gateway ID is required")
	}

	gatewayIds := make([]string, len(req))
	for i, gw := range req {
		gatewayIds[i] = gw.GatewayId
	}

	createdBy, err := resolveActorErr(r, h.identity, "associate gateways with GraphQL API")
	if err != nil {
		return err
	}

	gatewaysResponse, err := h.graphqlAPIService.AddGatewaysToAPI(apiId, gatewayIds, orgId, createdBy)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to associate gateways with GraphQL API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, gatewaysResponse)
	return nil
}

// GetAPIGateways handles GET /api/v0.9/graphql-apis/:graphqlApiId/gateways to get
// gateways associated with a GraphQL API including deployment details. Mirrors
// APIHandler.GetAPIGateways.
func (h *GraphQLAPIHandler) GetAPIGateways(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("graphqlApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	limit, offset := parsePagination(r)

	gatewaysResponse, err := h.graphqlAPIService.GetAPIGateways(apiId, orgId, limit, offset)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get gateways for GraphQL API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, gatewaysResponse)
	return nil
}

// decodeCreateGraphQLAPIRequest decodes a create request from either
// application/json (the metadata struct directly) or multipart/form-data
// (a JSON "metadata" field plus an optional "sdlFile" upload) — see
// GraphQLAPIMultipartRequest in resources/openapi.yaml. A file part always
// wins over any sdl/sdlUrl present in metadata.
func decodeCreateGraphQLAPIRequest(r *http.Request, req *api.CreateGraphQLAPIRequest) error {
	if !utils.IsMultipartFormRequest(r) {
		return json.NewDecoder(r.Body).Decode(req)
	}
	metadataJSON, sdl, err := utils.ParseGraphQLAPIMultipartRequest(r)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(metadataJSON, req); err != nil {
		return err
	}
	if sdl != "" {
		req.Sdl = &sdl
		req.SdlUrl = nil
	}
	return nil
}

// decodeUpdateGraphQLAPIRequest is decodeCreateGraphQLAPIRequest's update
// counterpart — same multipart/JSON split, targeting api.GraphQLAPI instead
// of api.CreateGraphQLAPIRequest (oapi-codegen generates these as distinct,
// non-embedding struct types, so the two can't share one generic function).
func decodeUpdateGraphQLAPIRequest(r *http.Request, req *api.GraphQLAPI) error {
	if !utils.IsMultipartFormRequest(r) {
		return json.NewDecoder(r.Body).Decode(req)
	}
	metadataJSON, sdl, err := utils.ParseGraphQLAPIMultipartRequest(r)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(metadataJSON, req); err != nil {
		return err
	}
	if sdl != "" {
		req.Sdl = &sdl
		req.SdlUrl = nil
	}
	return nil
}

// RegisterRoutes registers all GraphQL API routes.
func (h *GraphQLAPIHandler) RegisterRoutes(mux router.Router) {
	h.slogger.Debug("Registering GraphQL API routes")
	base := constants.APIBasePath + "/graphql-apis"
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.CreateGraphQLAPI))
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.ListGraphQLAPIs))
	mux.HandleFunc("GET "+base+"/{graphqlApiId}", middleware.MapErrors(h.slogger, h.GetGraphQLAPI))
	mux.HandleFunc("GET "+base+"/{graphqlApiId}/sdl", middleware.MapErrors(h.slogger, h.GetGraphQLAPISDL))
	mux.HandleFunc("PUT "+base+"/{graphqlApiId}", middleware.MapErrors(h.slogger, h.UpdateGraphQLAPI))
	mux.HandleFunc("DELETE "+base+"/{graphqlApiId}", middleware.MapErrors(h.slogger, h.DeleteGraphQLAPI))
	mux.HandleFunc("GET "+base+"/{graphqlApiId}/gateways", middleware.MapErrors(h.slogger, h.GetAPIGateways))
	mux.HandleFunc("POST "+base+"/{graphqlApiId}/gateways", middleware.MapErrors(h.slogger, h.AddGatewaysToAPI))
}

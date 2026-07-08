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
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/go-httpkit/httputil"
)

type APIHandler struct {
	apiService *service.APIService
	identity   *service.IdentityService
	slogger    *slog.Logger
}

func NewAPIHandler(apiService *service.APIService, identity *service.IdentityService, slogger *slog.Logger) *APIHandler {
	return &APIHandler{
		apiService: apiService,
		identity:   identity,
		slogger:    slogger,
	}
}

// CreateAPI handles POST /api/v0.9/rest-apis and creates a new API
func (h *APIHandler) CreateAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var req api.CreateRESTAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	// Validate required fields
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
	if isEmptyUpstreamDefinition(req.Upstream.Main) && (req.Upstream.Sandbox == nil || isEmptyUpstreamDefinition(*req.Upstream.Sandbox)) {
		return apperror.ValidationFailed.New("At least one upstream endpoint (main or sandbox) is required").
			WithLogMessage(fmt.Sprintf("no upstream endpoints provided for org %s", orgId))
	}

	createdBy, err := resolveActorErr(r, h.identity, "create API")
	if err != nil {
		return err
	}
	apiResponse, err := h.apiService.CreateAPI(&req, orgId, createdBy)
	if err != nil {
		if errors.Is(err, constants.ErrHandleExists) {
			return apperror.RESTAPIExists.Wrap(err, "An API with this handle already exists.").
				WithLogMessage(fmt.Sprintf("API handle already exists in org %s", orgId))
		}
		if errors.Is(err, constants.ErrAPINameVersionAlreadyExists) {
			return apperror.RESTAPIExists.Wrap(err, "An API with the same name and version already exists in the organization.").
				WithLogMessage(fmt.Sprintf("API with same name and version already exists in org %s", orgId))
		}
		if errors.Is(err, constants.ErrAPIAlreadyExists) {
			return apperror.RESTAPIExists.Wrap(err, "An API already exists in the project.").
				WithLogMessage(fmt.Sprintf("API already exists in the project in org %s", orgId))
		}
		if errors.Is(err, constants.ErrProjectNotFound) {
			return apperror.ProjectNotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("project not found in org %s", orgId))
		}
		if errors.Is(err, constants.ErrInvalidAPIName) {
			return apperror.ValidationFailed.Wrap(err, "Invalid API name format").
				WithLogMessage(fmt.Sprintf("invalid API name format in org %s", orgId))
		}
		if errors.Is(err, constants.ErrInvalidAPIContext) {
			return apperror.ValidationFailed.Wrap(err, "Invalid API context format").
				WithLogMessage(fmt.Sprintf("invalid API context format in org %s", orgId))
		}
		if errors.Is(err, constants.ErrInvalidAPIVersion) {
			return apperror.ValidationFailed.Wrap(err, "Invalid API version format").
				WithLogMessage(fmt.Sprintf("invalid API version format in org %s", orgId))
		}
		if errors.Is(err, constants.ErrInvalidLifecycleState) {
			return apperror.ValidationFailed.Wrap(err, "Invalid lifecycle status").
				WithLogMessage(fmt.Sprintf("invalid lifecycle status in org %s", orgId))
		}
		if errors.Is(err, constants.ErrInvalidAPIType) {
			return apperror.ValidationFailed.Wrap(err, "Invalid API type").
				WithLogMessage(fmt.Sprintf("invalid API type in org %s", orgId))
		}
		if errors.Is(err, constants.ErrInvalidTransport) {
			return apperror.ValidationFailed.Wrap(err, "Invalid transport protocol").
				WithLogMessage(fmt.Sprintf("invalid transport protocol in org %s", orgId))
		}
		if errors.Is(err, constants.ErrInvalidPolicyVersion) {
			return apperror.ValidationFailed.Wrap(err, "Invalid policy version format").
				WithLogMessage(fmt.Sprintf("invalid policy version format in org %s", orgId))
		}
		if errors.Is(err, constants.ErrSubscriptionPlanNotFoundOrInactive) {
			return apperror.ValidationFailed.Wrap(err, "Subscription plan not found or not active").
				WithLogMessage(fmt.Sprintf("subscription plan not found or not active in org %s", orgId))
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to create API in org %s", orgId))
	}

	setLocation(w, "rest-apis", strOrEmpty(apiResponse.Id))
	httputil.WriteJSON(w, http.StatusCreated, apiResponse)
	return nil
}

// GetAPI handles GET /api/v0.9/rest-apis/:apiId and retrieves an API by its handle
func (h *APIHandler) GetAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	apiResponse, err := h.apiService.GetAPIByHandle(apiId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("API %s not found in org %s", apiId, orgId))
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to get API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, apiResponse)
	return nil
}

// ListAPIs handles GET /api/v0.9/rest-apis and lists APIs for an organization filtered by project
func (h *APIHandler) ListAPIs(w http.ResponseWriter, r *http.Request) error {
	// Get organization from JWT token
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	projectId := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectId == "" {
		return apperror.ValidationFailed.New("projectId query parameter is required")
	}

	apis, err := h.apiService.GetAPIsByOrganization(orgId, projectId)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			return apperror.ProjectNotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("project %s not found in org %s", projectId, orgId))
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to get APIs for project %s in org %s", projectId, orgId))
	}

	response := api.RESTAPIListResponse{
		Count: len(apis),
		List:  apis,
		Pagination: api.Pagination{
			Total:  len(apis),
			Offset: 0,
			Limit:  len(apis),
		},
	}

	httputil.WriteJSON(w, http.StatusOK, response)
	return nil
}

// UpdateAPI updates an existing API identified by handle
func (h *APIHandler) UpdateAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	var req api.RESTAPI
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	// Validate upstream configuration if provided
	if isEmptyUpstreamDefinition(req.Upstream.Main) && (req.Upstream.Sandbox == nil || isEmptyUpstreamDefinition(*req.Upstream.Sandbox)) {
		return apperror.ValidationFailed.New("At least one upstream endpoint (main or sandbox) is required")
	}

	updatedBy, err := resolveActorErr(r, h.identity, "update API")
	if err != nil {
		return err
	}
	apiResponse, err := h.apiService.UpdateAPIByHandle(apiId, &req, orgId, updatedBy)
	if err != nil {
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("API %s not found in org %s", apiId, orgId))
		}
		if errors.Is(err, constants.ErrHandleImmutable) {
			return apperror.ValidationFailed.Wrap(err, "The id is immutable and cannot be changed")
		}
		if errors.Is(err, constants.ErrInvalidLifecycleState) {
			return apperror.ValidationFailed.Wrap(err, "Invalid lifecycle status").
				WithLogMessage(fmt.Sprintf("invalid lifecycle status for API %s in org %s", apiId, orgId))
		}
		if errors.Is(err, constants.ErrInvalidAPIType) {
			return apperror.ValidationFailed.Wrap(err, "Invalid API type").
				WithLogMessage(fmt.Sprintf("invalid API type for API %s in org %s", apiId, orgId))
		}
		if errors.Is(err, constants.ErrInvalidTransport) {
			return apperror.ValidationFailed.Wrap(err, "Invalid transport protocol").
				WithLogMessage(fmt.Sprintf("invalid transport protocol for API %s in org %s", apiId, orgId))
		}
		if errors.Is(err, constants.ErrInvalidPolicyVersion) {
			return apperror.ValidationFailed.Wrap(err, "Invalid policy version format").
				WithLogMessage(fmt.Sprintf("invalid policy version format for API %s in org %s", apiId, orgId))
		}
		if errors.Is(err, constants.ErrSubscriptionPlanNotFoundOrInactive) {
			return apperror.ValidationFailed.Wrap(err, "Subscription plan not found or not active").
				WithLogMessage(fmt.Sprintf("subscription plan not found or not active for API %s in org %s", apiId, orgId))
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to update API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, apiResponse)
	return nil
}

// DeleteAPI handles DELETE /api/v0.9/rest-apis/:apiId and deletes an API by its handle
func (h *APIHandler) DeleteAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	deletedBy, err := resolveActorErr(r, h.identity, "delete API")
	if err != nil {
		return err
	}
	if err := h.apiService.DeleteAPIByHandle(apiId, orgId, deletedBy); err != nil {
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("API %s not found in org %s", apiId, orgId))
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to delete API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

// AddGatewaysToAPI handles POST /api/v0.9/rest-apis/:apiId/gateways to associate gateways with an API
func (h *APIHandler) AddGatewaysToAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
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

	// Extract gateway IDs from request
	gatewayIds := make([]string, len(req))
	for i, gw := range req {
		gatewayIds[i] = gw.GatewayId
	}

	gatewaysResponse, err := h.apiService.AddGatewaysToAPIByHandle(apiId, gatewayIds, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("API %s not found in org %s", apiId, orgId))
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			return apperror.GatewayNotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("one or more gateways not found for API %s in org %s", apiId, orgId))
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to associate gateways with API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, gatewaysResponse)
	return nil
}

// GetAPIGateways handles GET /api/v0.9/rest-apis/:apiId/gateways to get gateways associated with an API including deployment details
func (h *APIHandler) GetAPIGateways(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	gatewaysResponse, err := h.apiService.GetAPIGatewaysByHandle(apiId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("API %s not found in org %s", apiId, orgId))
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to get gateways for API %s in org %s", apiId, orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, gatewaysResponse)
	return nil
}

// RegisterRoutes registers all API routes
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	h.slogger.Debug("Registering REST API routes")
	base := constants.APIBasePath + "/rest-apis"
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.CreateAPI))
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.ListAPIs))
	mux.HandleFunc("GET "+base+"/{restApiId}", middleware.MapErrors(h.slogger, h.GetAPI))
	mux.HandleFunc("PUT "+base+"/{restApiId}", middleware.MapErrors(h.slogger, h.UpdateAPI))
	mux.HandleFunc("DELETE "+base+"/{restApiId}", middleware.MapErrors(h.slogger, h.DeleteAPI))
	mux.HandleFunc("GET "+base+"/{restApiId}/gateways", middleware.MapErrors(h.slogger, h.GetAPIGateways))
	mux.HandleFunc("POST "+base+"/{restApiId}/gateways", middleware.MapErrors(h.slogger, h.AddGatewaysToAPI))
}

func isEmptyUpstreamDefinition(definition api.UpstreamDefinition) bool {
	if definition.Url != nil && *definition.Url != "" {
		return false
	}
	if definition.Ref != nil && *definition.Ref != "" {
		return false
	}
	return true
}

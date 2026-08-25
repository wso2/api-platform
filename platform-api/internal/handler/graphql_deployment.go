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

// GraphQLAPIDeploymentHandler handles GraphQL API deployment endpoints using the
// shared deployment model. Mirrors LLMProviderDeploymentHandler
// (internal/handler/llm_deployment.go) — see GraphQLAPIDeploymentService's doc
// comment for why GraphQL gets its own dedicated deployment service/handler
// pair rather than reusing DeploymentHandler/DeploymentService.
type GraphQLAPIDeploymentHandler struct {
	deploymentService *service.GraphQLAPIDeploymentService
	identity          *service.IdentityService
	slogger           *slog.Logger
}

// NewGraphQLAPIDeploymentHandler creates a new GraphQL API deployment handler.
func NewGraphQLAPIDeploymentHandler(deploymentService *service.GraphQLAPIDeploymentService, identity *service.IdentityService, slogger *slog.Logger) *GraphQLAPIDeploymentHandler {
	return &GraphQLAPIDeploymentHandler{deploymentService: deploymentService, identity: identity, slogger: slogger}
}

// DeployGraphQLAPI handles POST /api/v0.9/graphql-apis/{graphqlApiId}/deployments
func (h *GraphQLAPIDeploymentHandler) DeployGraphQLAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("graphqlApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("GraphQL API ID is required")
	}

	var req api.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid GraphQL API deployment request body for API %s", apiId))
	}

	if req.Name == "" {
		return apperror.GraphQLAPIDeploymentValidationFailed.New("name is required")
	}
	if req.Base == "" {
		return apperror.GraphQLAPIDeploymentValidationFailed.New("base is required (use 'current' or a deploymentId)")
	}
	if strings.TrimSpace(req.GatewayId) == "" {
		return apperror.GraphQLAPIDeploymentValidationFailed.New("gatewayId is required")
	}

	createdBy, err := resolveActorErr(r, h.identity, "deploy GraphQL API")
	if err != nil {
		return err
	}

	deployment, err := h.deploymentService.DeployGraphQLAPI(apiId, &req, orgId, createdBy)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to deploy GraphQL API %s", apiId))
	}

	setLocation(w, "graphql-apis", apiId, "deployments", deployment.DeploymentId.String())
	httputil.WriteJSON(w, http.StatusCreated, deployment)
	return nil
}

// UndeployGraphQLAPIDeployment handles POST /api/v0.9/graphql-apis/{graphqlApiId}/deployments/{deploymentId}/undeploy
func (h *GraphQLAPIDeploymentHandler) UndeployGraphQLAPIDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("graphqlApiId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")

	if apiId == "" {
		return apperror.ValidationFailed.New("GraphQL API ID is required")
	}
	deployment, err := h.deploymentService.UndeployGraphQLAPIDeployment(apiId, deploymentId, gatewayId, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to undeploy GraphQL API %s deployment %s on gateway %q", apiId, deploymentId, gatewayId))
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// RestoreGraphQLAPIDeployment handles POST /api/v0.9/graphql-apis/{graphqlApiId}/deployments/{deploymentId}/restore
func (h *GraphQLAPIDeploymentHandler) RestoreGraphQLAPIDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("graphqlApiId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")

	if apiId == "" {
		return apperror.ValidationFailed.New("GraphQL API ID is required")
	}
	deployment, err := h.deploymentService.RestoreGraphQLAPIDeployment(apiId, deploymentId, gatewayId, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to restore GraphQL API %s deployment %s on gateway %q", apiId, deploymentId, gatewayId))
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// DeleteGraphQLAPIDeployment handles DELETE /api/v0.9/graphql-apis/{graphqlApiId}/deployments/{deploymentId}
func (h *GraphQLAPIDeploymentHandler) DeleteGraphQLAPIDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("graphqlApiId")
	deploymentId := r.PathValue("deploymentId")

	if apiId == "" {
		return apperror.ValidationFailed.New("GraphQL API ID is required")
	}
	if deploymentId == "" {
		return apperror.ValidationFailed.New("Deployment ID is required")
	}

	if err := h.deploymentService.DeleteGraphQLAPIDeployment(apiId, deploymentId, orgId); err != nil {
		return serviceError(err, fmt.Sprintf("failed to delete GraphQL API %s deployment %s", apiId, deploymentId))
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GetGraphQLAPIDeployment handles GET /api/v0.9/graphql-apis/{graphqlApiId}/deployments/{deploymentId}
func (h *GraphQLAPIDeploymentHandler) GetGraphQLAPIDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("graphqlApiId")
	deploymentId := r.PathValue("deploymentId")

	if apiId == "" {
		return apperror.ValidationFailed.New("GraphQL API ID is required")
	}
	if deploymentId == "" {
		return apperror.ValidationFailed.New("Deployment ID is required")
	}

	deployment, err := h.deploymentService.GetGraphQLAPIDeployment(apiId, deploymentId, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get GraphQL API %s deployment %s", apiId, deploymentId))
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// GetGraphQLAPIDeployments handles GET /api/v0.9/graphql-apis/{graphqlApiId}/deployments
func (h *GraphQLAPIDeploymentHandler) GetGraphQLAPIDeployments(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("graphqlApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("GraphQL API ID is required")
	}

	q := r.URL.Query()
	var gatewayId, status *string
	if v := q.Get("gatewayId"); v != "" {
		gatewayId = &v
	}
	if v := q.Get("status"); v != "" {
		status = &v
	}

	limit, offset := parsePagination(r)

	deployments, err := h.deploymentService.GetGraphQLAPIDeployments(apiId, orgId, gatewayId, status)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get GraphQL API %s deployments", apiId))
	}

	paginateDeploymentList(deployments, limit, offset)
	httputil.WriteJSON(w, http.StatusOK, deployments)
	return nil
}

// RegisterRoutes registers all GraphQL API deployment-related routes.
func (h *GraphQLAPIDeploymentHandler) RegisterRoutes(mux router.Router) {
	base := constants.APIBasePath + "/graphql-apis/{graphqlApiId}"
	mux.HandleFunc("POST "+base+"/deployments", middleware.MapErrors(h.slogger, h.DeployGraphQLAPI))
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/undeploy", middleware.MapErrors(h.slogger, h.UndeployGraphQLAPIDeployment))
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/restore", middleware.MapErrors(h.slogger, h.RestoreGraphQLAPIDeployment))
	mux.HandleFunc("GET "+base+"/deployments", middleware.MapErrors(h.slogger, h.GetGraphQLAPIDeployments))
	mux.HandleFunc("GET "+base+"/deployments/{deploymentId}", middleware.MapErrors(h.slogger, h.GetGraphQLAPIDeployment))
	mux.HandleFunc("DELETE "+base+"/deployments/{deploymentId}", middleware.MapErrors(h.slogger, h.DeleteGraphQLAPIDeployment))
}

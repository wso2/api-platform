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
	"log/slog"
	"net/http"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/api-platform/httpkit/httputil"
)

// agentProxyDeployMaxBodyBytes bounds a deployment request body. It carries a
// name, a base reference, a gateway handle and optional metadata — never an
// artifact — so the ceiling is small.
const agentProxyDeployMaxBodyBytes = 64 << 10 // 64 KiB

// AgentProxyDeploymentHandler serves the Agent proxy deployment operations.
//
// Deployment creation is 201: the immutable record exists when the response is
// written. Undeploy and restore are 202: they record a transition the gateway
// has yet to acknowledge. In every case the Location names the deployment, whose
// GET reports the eventual state — a 201/202 is never a completed gateway
// deployment.
type AgentProxyDeploymentHandler struct {
	deploymentService *service.AgentDeploymentService
	identity          *service.IdentityService
	slogger           *slog.Logger
}

// NewAgentProxyDeploymentHandler creates a new AgentProxyDeploymentHandler.
func NewAgentProxyDeploymentHandler(deploymentService *service.AgentDeploymentService, identity *service.IdentityService, slogger *slog.Logger) *AgentProxyDeploymentHandler {
	return &AgentProxyDeploymentHandler{
		deploymentService: deploymentService,
		identity:          identity,
		slogger:           slogger,
	}
}

// RegisterRoutes wires the Agent proxy deployment routes. The patterns must match
// resources/openapi.yaml exactly — ScopeEnforcer keys on them.
func (h *AgentProxyDeploymentHandler) RegisterRoutes(mux router.Router) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/agent-proxies/{agentProxyId}/deployments", middleware.MapErrors(h.slogger, h.CreateDeployment))
	mux.HandleFunc("GET "+constants.APIBasePath+"/agent-proxies/{agentProxyId}/deployments", middleware.MapErrors(h.slogger, h.ListDeployments))
	mux.HandleFunc("GET "+constants.APIBasePath+"/agent-proxies/{agentProxyId}/deployments/{deploymentId}", middleware.MapErrors(h.slogger, h.GetDeployment))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/agent-proxies/{agentProxyId}/deployments/{deploymentId}", middleware.MapErrors(h.slogger, h.DeleteDeployment))
	mux.HandleFunc("POST "+constants.APIBasePath+"/agent-proxies/{agentProxyId}/deployments/{deploymentId}/undeploy", middleware.MapErrors(h.slogger, h.UndeployDeployment))
	mux.HandleFunc("POST "+constants.APIBasePath+"/agent-proxies/{agentProxyId}/deployments/{deploymentId}/restore", middleware.MapErrors(h.slogger, h.RestoreDeployment))
}

// CreateDeployment handles POST /api/v0.9/agent-proxies/{agentProxyId}/deployments
func (h *AgentProxyDeploymentHandler) CreateDeployment(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	handle := r.PathValue("agentProxyId")

	if err := requireJSONContentType(r); err != nil {
		return err
	}
	r.Body = http.MaxBytesReader(w, r.Body, agentProxyDeployMaxBodyBytes)
	var req api.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
		}
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid Agent proxy deployment request body for %s", handle))
	}

	createdBy, err := resolveActorErr(r, h.identity, "deploy Agent proxy")
	if err != nil {
		return err
	}

	deployment, err := h.deploymentService.DeployByHandle(handle, &req, orgID, createdBy)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to deploy Agent proxy %s", handle))
	}

	setLocation(w, "agent-proxies", handle, "deployments", deployment.DeploymentId.String())
	httputil.WriteJSON(w, http.StatusCreated, deployment)
	return nil
}

// UndeployDeployment handles POST /api/v0.9/agent-proxies/{agentProxyId}/deployments/{deploymentId}/undeploy
func (h *AgentProxyDeploymentHandler) UndeployDeployment(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	handle := r.PathValue("agentProxyId")
	deploymentID := r.PathValue("deploymentId")
	gatewayID := r.URL.Query().Get("gatewayId")
	if gatewayID == "" {
		return apperror.ValidationFailed.New("gatewayId is required")
	}

	deployment, err := h.deploymentService.UndeployByHandle(handle, deploymentID, gatewayID, orgID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to undeploy Agent proxy %s deployment %s on gateway %s", handle, deploymentID, gatewayID))
	}

	setLocation(w, "agent-proxies", handle, "deployments", deployment.DeploymentId.String())
	httputil.WriteJSON(w, http.StatusAccepted, deployment)
	return nil
}

// RestoreDeployment handles POST /api/v0.9/agent-proxies/{agentProxyId}/deployments/{deploymentId}/restore
func (h *AgentProxyDeploymentHandler) RestoreDeployment(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	handle := r.PathValue("agentProxyId")
	deploymentID := r.PathValue("deploymentId")
	gatewayID := r.URL.Query().Get("gatewayId")
	if gatewayID == "" {
		return apperror.ValidationFailed.New("gatewayId is required")
	}

	deployment, err := h.deploymentService.RestoreByHandle(handle, deploymentID, gatewayID, orgID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to restore Agent proxy %s deployment %s on gateway %s", handle, deploymentID, gatewayID))
	}

	setLocation(w, "agent-proxies", handle, "deployments", deployment.DeploymentId.String())
	httputil.WriteJSON(w, http.StatusAccepted, deployment)
	return nil
}

// DeleteDeployment handles DELETE /api/v0.9/agent-proxies/{agentProxyId}/deployments/{deploymentId}
func (h *AgentProxyDeploymentHandler) DeleteDeployment(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	handle := r.PathValue("agentProxyId")
	deploymentID := r.PathValue("deploymentId")

	if err := h.deploymentService.DeleteByHandle(handle, deploymentID, orgID); err != nil {
		return serviceError(err, fmt.Sprintf("failed to delete Agent proxy %s deployment %s", handle, deploymentID))
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GetDeployment handles GET /api/v0.9/agent-proxies/{agentProxyId}/deployments/{deploymentId}
func (h *AgentProxyDeploymentHandler) GetDeployment(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	handle := r.PathValue("agentProxyId")
	deploymentID := r.PathValue("deploymentId")

	deployment, err := h.deploymentService.GetByHandle(handle, deploymentID, orgID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get Agent proxy %s deployment %s", handle, deploymentID))
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// ListDeployments handles GET /api/v0.9/agent-proxies/{agentProxyId}/deployments
func (h *AgentProxyDeploymentHandler) ListDeployments(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	handle := r.PathValue("agentProxyId")
	query := r.URL.Query()
	limit, offset := parsePagination(r)

	deployments, err := h.deploymentService.ListByHandle(handle, query.Get("gatewayId"), query.Get("status"), orgID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get Agent proxy %s deployments", handle))
	}

	paginateDeploymentList(deployments, limit, offset)
	httputil.WriteJSON(w, http.StatusOK, deployments)
	return nil
}

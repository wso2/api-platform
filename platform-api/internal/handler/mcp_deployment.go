/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

// MCPProxyDeploymentHandler handles MCP proxy deployment endpoints
type MCPProxyDeploymentHandler struct {
	deploymentService *service.MCPDeploymentService
	identity          *service.IdentityService
	slogger           *slog.Logger
}

// NewMCPProxyDeploymentHandler creates a new MCP proxy deployment handler
func NewMCPProxyDeploymentHandler(deploymentService *service.MCPDeploymentService, identity *service.IdentityService, slogger *slog.Logger) *MCPProxyDeploymentHandler {
	return &MCPProxyDeploymentHandler{
		deploymentService: deploymentService,
		identity:          identity,
		slogger:           slogger,
	}
}

// RegisterRoutes registers all MCP proxy deployment-related routes
func (h *MCPProxyDeploymentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}/deployments", h.DeployMCPProxy)
	mux.HandleFunc("POST "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}/deployments/{deploymentId}/undeploy", h.UndeployMCPProxyDeployment)
	mux.HandleFunc("POST "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}/deployments/{deploymentId}/restore", h.RestoreMCPProxyDeployment)
	mux.HandleFunc("GET "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}/deployments", h.GetMCPProxyDeployments)
	mux.HandleFunc("GET "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}/deployments/{deploymentId}", h.GetMCPProxyDeployment)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}/deployments/{deploymentId}", h.DeleteMCPProxyDeployment)
}

// DeployMCPProxy handles POST /api/v0.9/mcp-proxies/:id/deployments
func (h *MCPProxyDeploymentHandler) DeployMCPProxy(w http.ResponseWriter, r *http.Request) {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := r.PathValue("mcpProxyId")
	if proxyId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"MCP proxy ID is required"))
		return
	}

	var req api.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	if req.Name == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"name is required"))
		return
	}
	if req.Base == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"base is required (use 'current' or a deploymentId)"))
		return
	}
	if strings.TrimSpace(req.GatewayId) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayId is required"))
		return
	}

	createdBy, ok := resolveActor(w, r, h.identity, h.slogger, "deploy MCP proxy")
	if !ok {
		return
	}
	deployment, err := h.deploymentService.DeployMCPProxyByHandle(proxyId, &req, orgId, createdBy)
	if err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		case errors.Is(err, constants.ErrGatewayNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		case errors.Is(err, constants.ErrBaseDeploymentNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Base deployment not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNameRequired):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment name is required"))
			return
		case errors.Is(err, constants.ErrDeploymentBaseRequired):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Base is required"))
			return
		case errors.Is(err, constants.ErrDeploymentGatewayIDRequired):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Gateway ID is required"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid input"))
			return
		default:
			h.slogger.Error("Failed to deploy MCP proxy", "proxyId", proxyId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to deploy MCP proxy"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusCreated, deployment)
}

// UndeployMCPProxyDeployment handles POST /api/v0.9/mcp-proxies/:id/deployments/:deploymentId/undeploy
func (h *MCPProxyDeploymentHandler) UndeployMCPProxyDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := r.PathValue("mcpProxyId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"deploymentId is required"))
		return
	}
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayId is required"))
		return
	}

	if proxyId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"MCP proxy ID is required"))
		return
	}

	deployment, err := h.deploymentService.UndeployDeploymentByHandle(proxyId, deploymentId, gatewayId, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: undeployment cannot be initiated from the CP.
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		case errors.Is(err, constants.ErrGatewayNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotActive):
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"No active deployment found for this MCP proxy on the gateway"))
			return
		case errors.Is(err, constants.ErrGatewayIDMismatch):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment is bound to a different gateway"))
			return
		default:
			h.slogger.Error("Failed to undeploy MCP proxy", "proxyId", proxyId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to undeploy deployment"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// RestoreMCPProxyDeployment handles POST /api/v0.9/mcp-proxies/:id/deployments/:deploymentId/restore
func (h *MCPProxyDeploymentHandler) RestoreMCPProxyDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := r.PathValue("mcpProxyId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")

	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"deploymentId is required"))
		return
	}
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayId is required"))
		return
	}
	if proxyId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"MCP proxy ID is required"))
		return
	}

	deployment, err := h.deploymentService.RestoreMCPDeploymentByHandle(proxyId, deploymentId, gatewayId, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: restore cannot be initiated from the CP.
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		case errors.Is(err, constants.ErrGatewayNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		case errors.Is(err, constants.ErrDeploymentAlreadyDeployed):
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Cannot restore currently deployed deployment"))
			return
		case errors.Is(err, constants.ErrGatewayIDMismatch):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment is bound to a different gateway"))
			return
		default:
			h.slogger.Error("Failed to restore MCP proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to restore deployment"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// DeleteMCPProxyDeployment handles DELETE /api/v0.9/mcp-proxies/:id/deployments/:deploymentId
func (h *MCPProxyDeploymentHandler) DeleteMCPProxyDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := r.PathValue("mcpProxyId")
	deploymentId := r.PathValue("deploymentId")

	if proxyId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"MCP proxy ID is required"))
		return
	}
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	err := h.deploymentService.DeleteDeploymentByHandle(proxyId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		case errors.Is(err, constants.ErrDeploymentIsDeployed):
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Cannot delete an active deployment - undeploy it first"))
			return
		default:
			h.slogger.Error("Failed to delete MCP proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete deployment"))
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetMCPProxyDeployment handles GET /api/v0.9/mcp-proxies/:id/deployments/:deploymentId
func (h *MCPProxyDeploymentHandler) GetMCPProxyDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := r.PathValue("mcpProxyId")
	deploymentId := r.PathValue("deploymentId")

	if proxyId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"MCP proxy ID is required"))
		return
	}
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	deployment, err := h.deploymentService.GetDeploymentByHandle(proxyId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		default:
			h.slogger.Error("Failed to get MCP proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to retrieve deployment"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// GetMCPProxyDeployments handles GET /api/v0.9/mcp-proxies/:id/deployments
func (h *MCPProxyDeploymentHandler) GetMCPProxyDeployments(w http.ResponseWriter, r *http.Request) {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := r.PathValue("mcpProxyId")
	if proxyId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"MCP proxy ID is required"))
		return
	}

	var params api.GetMCPProxyDeploymentsParams
	if v := r.URL.Query().Get("gatewayId"); v != "" {
		gid := api.GatewayIdQ(v)
		params.GatewayId = &gid
	}
	if v := r.URL.Query().Get("status"); v != "" {
		st := api.GetMCPProxyDeploymentsParamsStatus(v)
		params.Status = &st
	}

	gatewayVal := ""
	if params.GatewayId != nil {
		gatewayVal = string(*params.GatewayId)
	}

	statusVal := ""
	if params.Status != nil {
		statusVal = string(*params.Status)
	}

	deployments, err := h.deploymentService.GetDeploymentsByHandle(proxyId, gatewayVal, statusVal, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		case errors.Is(err, constants.ErrInvalidDeploymentStatus):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid deployment status"))
			return
		default:
			h.slogger.Error("Failed to get MCP proxy deployments", "proxyId", proxyId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to retrieve deployments"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployments)
}

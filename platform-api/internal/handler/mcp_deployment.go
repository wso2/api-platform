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
	mux.HandleFunc("POST "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}/deployments", middleware.MapErrors(h.slogger, h.DeployMCPProxy))
	mux.HandleFunc("POST "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}/deployments/{deploymentId}/undeploy", middleware.MapErrors(h.slogger, h.UndeployMCPProxyDeployment))
	mux.HandleFunc("POST "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}/deployments/{deploymentId}/restore", middleware.MapErrors(h.slogger, h.RestoreMCPProxyDeployment))
	mux.HandleFunc("GET "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}/deployments", middleware.MapErrors(h.slogger, h.GetMCPProxyDeployments))
	mux.HandleFunc("GET "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}/deployments/{deploymentId}", middleware.MapErrors(h.slogger, h.GetMCPProxyDeployment))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/mcp-proxies/{mcpProxyId}/deployments/{deploymentId}", middleware.MapErrors(h.slogger, h.DeleteMCPProxyDeployment))
}

// DeployMCPProxy handles POST /api/v0.9/mcp-proxies/:id/deployments
func (h *MCPProxyDeploymentHandler) DeployMCPProxy(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyId := r.PathValue("mcpProxyId")
	if proxyId == "" {
		return apperror.ValidationFailed.New("MCP proxy ID is required")
	}

	var req api.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid MCP proxy deployment request body for proxy %s", proxyId))
	}

	if req.Name == "" {
		return apperror.MCPProxyDeploymentValidationFailed.New("name is required")
	}
	if req.Base == "" {
		return apperror.MCPProxyDeploymentValidationFailed.New("base is required (use 'current' or a deploymentId)")
	}
	if strings.TrimSpace(req.GatewayId) == "" {
		return apperror.MCPProxyDeploymentValidationFailed.New("gatewayId is required")
	}

	createdBy, err := resolveActorErr(r, h.identity, "deploy MCP proxy")
	if err != nil {
		return err
	}
	deployment, err := h.deploymentService.DeployMCPProxyByHandle(proxyId, &req, orgId, createdBy)
	if err != nil {
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			return apperror.MCPProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrGatewayNotFound):
			return apperror.GatewayNotFound.Wrap(err)
		case errors.Is(err, constants.ErrBaseDeploymentNotFound):
			return apperror.DeploymentBaseNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNameRequired):
			return apperror.MCPProxyDeploymentValidationFailed.Wrap(err, "Deployment name is required")
		case errors.Is(err, constants.ErrDeploymentBaseRequired):
			return apperror.MCPProxyDeploymentValidationFailed.Wrap(err, "Base is required")
		case errors.Is(err, constants.ErrDeploymentGatewayIDRequired):
			return apperror.MCPProxyDeploymentValidationFailed.Wrap(err, "Gateway ID is required")
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.MCPProxyDeploymentValidationFailed.Wrap(err, "Invalid input")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to deploy MCP proxy %s", proxyId))
		}
	}

	httputil.WriteJSON(w, http.StatusCreated, deployment)
	return nil
}

// UndeployMCPProxyDeployment handles POST /api/v0.9/mcp-proxies/:id/deployments/:deploymentId/undeploy
func (h *MCPProxyDeploymentHandler) UndeployMCPProxyDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyId := r.PathValue("mcpProxyId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")
	if deploymentId == "" {
		return apperror.ValidationFailed.New("deploymentId is required")
	}
	if gatewayId == "" {
		return apperror.ValidationFailed.New("gatewayId is required")
	}

	if proxyId == "" {
		return apperror.ValidationFailed.New("MCP proxy ID is required")
	}

	deployment, err := h.deploymentService.UndeployDeploymentByHandle(proxyId, deploymentId, gatewayId, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: undeployment cannot be initiated from the CP.
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			return apperror.MCPProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotFound):
			return apperror.DeploymentNotFound.Wrap(err)
		case errors.Is(err, constants.ErrGatewayNotFound):
			return apperror.GatewayNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotActive):
			return apperror.DeploymentNotActive.Wrap(err, "MCP proxy")
		case errors.Is(err, constants.ErrGatewayIDMismatch):
			return apperror.DeploymentGatewayMismatch.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to undeploy MCP proxy %s deployment %s on gateway %s", proxyId, deploymentId, gatewayId))
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// RestoreMCPProxyDeployment handles POST /api/v0.9/mcp-proxies/:id/deployments/:deploymentId/restore
func (h *MCPProxyDeploymentHandler) RestoreMCPProxyDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyId := r.PathValue("mcpProxyId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")

	if deploymentId == "" {
		return apperror.ValidationFailed.New("deploymentId is required")
	}
	if gatewayId == "" {
		return apperror.ValidationFailed.New("gatewayId is required")
	}
	if proxyId == "" {
		return apperror.ValidationFailed.New("MCP proxy ID is required")
	}

	deployment, err := h.deploymentService.RestoreMCPDeploymentByHandle(proxyId, deploymentId, gatewayId, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: restore cannot be initiated from the CP.
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			return apperror.MCPProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotFound):
			return apperror.DeploymentNotFound.Wrap(err)
		case errors.Is(err, constants.ErrGatewayNotFound):
			return apperror.GatewayNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentAlreadyDeployed):
			return apperror.DeploymentRestoreConflict.Wrap(err)
		case errors.Is(err, constants.ErrGatewayIDMismatch):
			return apperror.DeploymentGatewayMismatch.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to restore MCP proxy %s deployment %s on gateway %s", proxyId, deploymentId, gatewayId))
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// DeleteMCPProxyDeployment handles DELETE /api/v0.9/mcp-proxies/:id/deployments/:deploymentId
func (h *MCPProxyDeploymentHandler) DeleteMCPProxyDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyId := r.PathValue("mcpProxyId")
	deploymentId := r.PathValue("deploymentId")

	if proxyId == "" {
		return apperror.ValidationFailed.New("MCP proxy ID is required")
	}
	if deploymentId == "" {
		return apperror.ValidationFailed.New("Deployment ID is required")
	}

	err := h.deploymentService.DeleteDeploymentByHandle(proxyId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			return apperror.MCPProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotFound):
			return apperror.DeploymentNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentIsDeployed):
			return apperror.DeploymentActive.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to delete MCP proxy %s deployment %s", proxyId, deploymentId))
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GetMCPProxyDeployment handles GET /api/v0.9/mcp-proxies/:id/deployments/:deploymentId
func (h *MCPProxyDeploymentHandler) GetMCPProxyDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyId := r.PathValue("mcpProxyId")
	deploymentId := r.PathValue("deploymentId")

	if proxyId == "" {
		return apperror.ValidationFailed.New("MCP proxy ID is required")
	}
	if deploymentId == "" {
		return apperror.ValidationFailed.New("Deployment ID is required")
	}

	deployment, err := h.deploymentService.GetDeploymentByHandle(proxyId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrMCPProxyNotFound):
			return apperror.MCPProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotFound):
			return apperror.DeploymentNotFound.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to get MCP proxy %s deployment %s", proxyId, deploymentId))
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// GetMCPProxyDeployments handles GET /api/v0.9/mcp-proxies/:id/deployments
func (h *MCPProxyDeploymentHandler) GetMCPProxyDeployments(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyId := r.PathValue("mcpProxyId")
	if proxyId == "" {
		return apperror.ValidationFailed.New("MCP proxy ID is required")
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
			return apperror.MCPProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrInvalidDeploymentStatus):
			return apperror.DeploymentInvalidStatus.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to get MCP proxy %s deployments", proxyId))
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployments)
	return nil
}

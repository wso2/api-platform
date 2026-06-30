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

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/utils"
	egservice "platform-api/src/plugins/eventgateway/service"

	"github.com/wso2/go-httpkit/httputil"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// WebSubAPIDeploymentHandler handles deployment routes for WebSub APIs
type WebSubAPIDeploymentHandler struct {
	websubAPIDeploymentService *egservice.WebSubAPIDeploymentService
	slogger                    *slog.Logger
}

// NewWebSubAPIDeploymentHandler creates a new WebSubAPIDeploymentHandler
func NewWebSubAPIDeploymentHandler(websubAPIDeploymentService *egservice.WebSubAPIDeploymentService, slogger *slog.Logger) *WebSubAPIDeploymentHandler {
	return &WebSubAPIDeploymentHandler{
		websubAPIDeploymentService: websubAPIDeploymentService,
		slogger:                    slogger,
	}
}

// RegisterRoutes registers WebSub API deployment routes
func (h *WebSubAPIDeploymentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/websub-apis/{id}/deployments", h.DeployWebSubAPI)
	mux.HandleFunc("POST "+constants.APIBasePath+"/websub-apis/{id}/deployments/{deploymentId}/undeploy", h.UndeployDeployment)
	mux.HandleFunc("POST "+constants.APIBasePath+"/websub-apis/{id}/deployments/{deploymentId}/restore", h.RestoreDeployment)
	mux.HandleFunc("GET "+constants.APIBasePath+"/websub-apis/{id}/deployments", h.GetDeployments)
	mux.HandleFunc("GET "+constants.APIBasePath+"/websub-apis/{id}/deployments/{deploymentId}", h.GetDeployment)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/websub-apis/{id}/deployments/{deploymentId}", h.DeleteDeployment)
}

// DeployWebSubAPI handles POST /api/v0.9/websub-apis/:apiId/deployments
func (h *WebSubAPIDeploymentHandler) DeployWebSubAPI(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("id")
	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API ID is required"))
		return
	}

	var req api.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	if req.Name == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "name is required"))
		return
	}
	if req.Base == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "base is required"))
		return
	}
	if req.GatewayId == (openapi_types.UUID{}) {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "gatewayId is required"))
		return
	}

	createdBy, _ := middleware.GetUserIDFromRequest(r)
	deployment, err := h.websubAPIDeploymentService.DeployWebSubAPIByHandle(apiId, &req, orgId, createdBy)
	if err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		h.handleDeploymentError(w, err, apiId)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, deployment)
}

// UndeployDeployment handles POST /api/v0.9/websub-apis/:apiId/deployments/:deploymentId/undeploy
func (h *WebSubAPIDeploymentHandler) UndeployDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("id")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "deploymentId is required"))
		return
	}
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "gatewayId is required"))
		return
	}

	deployment, err := h.websubAPIDeploymentService.UndeployWebSubAPIDeploymentByHandle(apiId, deploymentId, gatewayId, orgId)
	if err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		h.handleDeploymentError(w, err, apiId)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// RestoreDeployment handles POST /api/v0.9/websub-apis/:apiId/deployments/:deploymentId/restore
func (h *WebSubAPIDeploymentHandler) RestoreDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("id")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "deploymentId is required"))
		return
	}
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "gatewayId is required"))
		return
	}

	deployment, err := h.websubAPIDeploymentService.RestoreWebSubAPIDeploymentByHandle(apiId, deploymentId, gatewayId, orgId)
	if err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		h.handleDeploymentError(w, err, apiId)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// GetDeployments handles GET /api/v0.9/websub-apis/:apiId/deployments
func (h *WebSubAPIDeploymentHandler) GetDeployments(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("id")
	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API ID is required"))
		return
	}

	var params api.GetDeploymentsParams
	if v := r.URL.Query().Get("gatewayId"); v != "" {
		gid := api.GatewayIdQ(v)
		params.GatewayId = &gid
	}
	if v := r.URL.Query().Get("status"); v != "" {
		st := api.GetDeploymentsParamsStatus(v)
		params.Status = &st
	}

	var gatewayId, status string
	if params.GatewayId != nil {
		gatewayId = string(*params.GatewayId)
	}
	if params.Status != nil {
		status = string(*params.Status)
	}

	deployments, err := h.websubAPIDeploymentService.GetWebSubAPIDeploymentsByHandle(apiId, gatewayId, status, orgId)
	if err != nil {
		h.handleDeploymentError(w, err, apiId)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployments)
}

// GetDeployment handles GET /api/v0.9/websub-apis/:apiId/deployments/:deploymentId
func (h *WebSubAPIDeploymentHandler) GetDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("id")
	deploymentId := r.PathValue("deploymentId")

	deployment, err := h.websubAPIDeploymentService.GetWebSubAPIDeploymentByHandle(apiId, deploymentId, orgId)
	if err != nil {
		h.handleDeploymentError(w, err, apiId)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// DeleteDeployment handles DELETE /api/v0.9/websub-apis/:apiId/deployments/:deploymentId
func (h *WebSubAPIDeploymentHandler) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("id")
	deploymentId := r.PathValue("deploymentId")

	if err := h.websubAPIDeploymentService.DeleteWebSubAPIDeploymentByHandle(apiId, deploymentId, orgId); err != nil {
		h.handleDeploymentError(w, err, apiId)
		return
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *WebSubAPIDeploymentHandler) handleDeploymentError(w http.ResponseWriter, err error, apiId string) {
	switch {
	case errors.Is(err, constants.ErrWebSubAPINotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebSub API not found"))
	case errors.Is(err, constants.ErrGatewayNotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Gateway not found"))
	case errors.Is(err, constants.ErrDeploymentNotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Deployment not found"))
	case errors.Is(err, constants.ErrBaseDeploymentNotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Base deployment not found"))
	case errors.Is(err, constants.ErrDeploymentNotActive):
		httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "No active deployment found for this API on the gateway"))
	case errors.Is(err, constants.ErrDeploymentIsDeployed):
		httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Cannot delete an active deployment - undeploy it first"))
	case errors.Is(err, constants.ErrDeploymentAlreadyDeployed):
		httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Cannot restore currently deployed deployment"))
	case errors.Is(err, constants.ErrInvalidDeploymentRestoreState):
		httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Deployment cannot be restored: only ARCHIVED or UNDEPLOYED deployments are eligible"))
	case errors.Is(err, constants.ErrGatewayIDMismatch):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Deployment is bound to a different gateway"))
	case errors.Is(err, constants.ErrAPINoBackendServices):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API must have at least one backend service configured"))
	default:
		h.slogger.Error("WebSub API deployment error", "apiId", apiId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}

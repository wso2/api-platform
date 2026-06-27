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

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

type DeploymentHandler struct {
	deploymentService *service.DeploymentService
	slogger           *slog.Logger
}

func NewDeploymentHandler(deploymentService *service.DeploymentService, slogger *slog.Logger) *DeploymentHandler {
	return &DeploymentHandler{
		deploymentService: deploymentService,
		slogger:           slogger,
	}
}

// DeployAPI handles POST /api/v0.9/rest-apis/:apiHandle/deployments
// Creates a new immutable deployment artifact and deploys it to a gateway
func (h *DeploymentHandler) DeployAPI(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("apiHandle")
	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	var req api.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
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
	if req.GatewayHandle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayHandle is required"))
		return
	}

	createdBy, _ := middleware.GetUsernameFromRequest(r)
	deployment, err := h.deploymentService.DeployAPIByHandle(apiId, &req, orgId, createdBy)
	if err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		if errors.Is(err, constants.ErrBaseDeploymentNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Base deployment not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNameRequired) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment name is required"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentBaseRequired) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Base is required (use 'current' or a deploymentId)"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentGatewayIDRequired) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Gateway ID is required"))
			return
		}
		if errors.Is(err, constants.ErrAPINoBackendServices) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"API must have at least one backend service attached before deployment"))
			return
		}
		h.slogger.Error("Failed to deploy API", "apiId", apiId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to deploy API"))
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, deployment)
}

// UndeployDeployment handles POST /api/v0.9/rest-apis/:apiId/deployments/:deploymentId/undeploy
func (h *DeploymentHandler) UndeployDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("apiHandle")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"deploymentId is required"))
		return
	}
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayHandle is required"))
		return
	}
	if deploymentId == "00000000-0000-0000-0000-000000000000" || gatewayId == "00000000-0000-0000-0000-000000000000" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"deploymentId/gatewayId cannot be zero-value UUID"))
		return
	}

	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}
	actor, _ := middleware.GetUsernameFromRequest(r)
	deployment, err := h.deploymentService.UndeployDeploymentByHandle(apiId, deploymentId, gatewayId, orgId, actor)
	if err != nil {
		// DP-originated artifacts are read-only: undeployment cannot be initiated from the CP.
		if respondArtifactGuardError(w, err) {
			return
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"No active deployment found for this API on the gateway"))
			return
		}
		if errors.Is(err, constants.ErrGatewayIDMismatch) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment is bound to a different gateway"))
			return
		}
		h.slogger.Error("Failed to undeploy", "apiId", apiId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to undeploy deployment"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// RestoreDeployment handles POST /api/v0.9/rest-apis/:apiId/deployments/:deploymentId/restore
func (h *DeploymentHandler) RestoreDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("apiHandle")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"deploymentId is required"))
		return
	}
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayHandle is required"))
		return
	}
	if deploymentId == "00000000-0000-0000-0000-000000000000" || gatewayId == "00000000-0000-0000-0000-000000000000" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"deploymentId/gatewayId cannot be zero-value UUID"))
		return
	}

	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}
	actor, _ := middleware.GetUsernameFromRequest(r)
	deployment, err := h.deploymentService.RestoreDeploymentByHandle(apiId, deploymentId, gatewayId, orgId, actor)
	if err != nil {
		// DP-originated artifacts are read-only: restore cannot be initiated from the CP.
		if respondArtifactGuardError(w, err) {
			return
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentAlreadyDeployed) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Cannot restore currently deployed deployment"))
			return
		}
		if errors.Is(err, constants.ErrGatewayIDMismatch) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment is bound to a different gateway"))
			return
		}
		h.slogger.Error("Failed to restore deployment", "apiId", apiId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to restore deployment"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// DeleteDeployment handles DELETE /api/v0.9/rest-apis/:apiHandle/deployments/:deploymentId
// Permanently deletes an undeployed deployment artifact
func (h *DeploymentHandler) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("apiHandle")
	deploymentId := r.PathValue("deploymentId")

	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	actor, _ := middleware.GetUsernameFromRequest(r)
	err := h.deploymentService.DeleteDeploymentByHandle(apiId, deploymentId, orgId, actor)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentIsDeployed) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Cannot delete an active deployment - undeploy it first"))
			return
		}
		if respondArtifactGuardError(w, err) {
			return
		}
		h.slogger.Error("Failed to delete deployment", "apiId", apiId, "deploymentId", deploymentId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete deployment"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetDeployment handles GET /api/v0.9/rest-apis/:apiHandle/deployments/:deploymentId
// Retrieves metadata for a specific deployment artifact
func (h *DeploymentHandler) GetDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("apiHandle")
	deploymentId := r.PathValue("deploymentId")

	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	deployment, err := h.deploymentService.GetDeploymentByHandle(apiId, deploymentId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		}
		h.slogger.Error("Failed to get deployment", "apiId", apiId, "deploymentId", deploymentId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to retrieve deployment"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// GetDeployments handles GET /api/v0.9/rest-apis/:apiHandle/deployments
// Retrieves all deployment records for an API with optional filters
func (h *DeploymentHandler) GetDeployments(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("apiHandle")
	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	var params api.GetDeploymentsParams
	if v := r.URL.Query().Get("gatewayId"); v != "" {
		gid := api.GatewayIdQ(v)
		params.GatewayHandle = &gid
	}
	if v := r.URL.Query().Get("status"); v != "" {
		st := api.GetDeploymentsParamsStatus(v)
		params.Status = &st
	}

	var gatewayHandle, status string
	if params.GatewayHandle != nil {
		gatewayHandle = string(*params.GatewayHandle)
	}
	if params.Status != nil {
		status = string(*params.Status)
	}

	deployments, err := h.deploymentService.GetDeploymentsByHandle(apiId, gatewayHandle, status, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidDeploymentStatus) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid deployment status"))
			return
		}
		h.slogger.Error("Failed to get deployments", "apiId", apiId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to retrieve deployments"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployments)
}

// RegisterRoutes registers all deployment-related routes
func (h *DeploymentHandler) RegisterRoutes(mux *http.ServeMux) {
	h.slogger.Debug("Registering deployment routes")
	base := constants.APIBasePath + "/rest-apis/{apiHandle}"
	mux.HandleFunc("POST "+base+"/deployments", h.DeployAPI)
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/undeploy", h.UndeployDeployment)
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/restore", h.RestoreDeployment)
	mux.HandleFunc("GET "+base+"/deployments", h.GetDeployments)
	mux.HandleFunc("GET "+base+"/deployments/{deploymentId}", h.GetDeployment)
	mux.HandleFunc("DELETE "+base+"/deployments/{deploymentId}", h.DeleteDeployment)
}

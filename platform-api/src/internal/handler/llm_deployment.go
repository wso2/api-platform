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
	"log/slog"
	"net/http"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// LLMProviderDeploymentHandler handles LLM provider deployment endpoints
// using the shared deployment model.
type LLMProviderDeploymentHandler struct {
	deploymentService *service.LLMProviderDeploymentService
	slogger           *slog.Logger
}

// LLMProxyDeploymentHandler handles LLM proxy deployment endpoints
// using the shared deployment model.
type LLMProxyDeploymentHandler struct {
	deploymentService *service.LLMProxyDeploymentService
	slogger           *slog.Logger
}

func NewLLMProviderDeploymentHandler(deploymentService *service.LLMProviderDeploymentService, slogger *slog.Logger) *LLMProviderDeploymentHandler {
	return &LLMProviderDeploymentHandler{deploymentService: deploymentService, slogger: slogger}
}

func NewLLMProxyDeploymentHandler(deploymentService *service.LLMProxyDeploymentService, slogger *slog.Logger) *LLMProxyDeploymentHandler {
	return &LLMProxyDeploymentHandler{deploymentService: deploymentService, slogger: slogger}
}

// DeployLLMProvider handles POST /api/v0.9/llm-providers/{llmProviderId}/deployments
func (h *LLMProviderDeploymentHandler) DeployLLMProvider(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := r.PathValue("llmProviderId")
	if providerId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
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
	if req.GatewayId == (openapi_types.UUID{}) {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayId is required"))
		return
	}

	deployment, err := h.deploymentService.DeployLLMProvider(providerId, &req, orgId)
	if err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
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
				"Base is required (use 'current' or a deploymentId)"))
			return
		case errors.Is(err, constants.ErrDeploymentGatewayIDRequired):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Gateway ID is required"))
			return
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Referenced template not found"))
			return
		case errors.Is(err, constants.ErrInvalidInput):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid input"))
			return
		default:
			h.slogger.Error("Failed to deploy LLM provider", "providerId", providerId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to deploy LLM provider"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusCreated, deployment)
}

// UndeployLLMProviderDeployment handles POST /api/v0.9/llm-providers/{llmProviderId}/deployments/{deploymentId}/undeploy
func (h *LLMProviderDeploymentHandler) UndeployLLMProviderDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := r.PathValue("llmProviderId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")

	if providerId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}
	deployment, err := h.deploymentService.UndeployLLMProviderDeployment(providerId, deploymentId, gatewayId, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: undeployment cannot be initiated from the CP.
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
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
				"No active deployment found for this LLM provider on the gateway"))
			return
		case errors.Is(err, constants.ErrGatewayIDMismatch):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment is bound to a different gateway"))
			return
		default:
			h.slogger.Error("Failed to undeploy LLM provider", "providerId", providerId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to undeploy deployment"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// RestoreLLMProviderDeployment handles POST /api/v0.9/llm-providers/{llmProviderId}/deployments/{deploymentId}/restore
func (h *LLMProviderDeploymentHandler) RestoreLLMProviderDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := r.PathValue("llmProviderId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")

	if providerId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}
	deployment, err := h.deploymentService.RestoreLLMProviderDeployment(providerId, deploymentId, gatewayId, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: restore cannot be initiated from the CP.
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
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
			h.slogger.Error("Failed to restore LLM provider deployment", "providerId", providerId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to restore deployment"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// DeleteLLMProviderDeployment handles DELETE /api/v0.9/llm-providers/{llmProviderId}/deployments/{deploymentId}
func (h *LLMProviderDeploymentHandler) DeleteLLMProviderDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := r.PathValue("llmProviderId")
	deploymentId := r.PathValue("deploymentId")

	if providerId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	err := h.deploymentService.DeleteLLMProviderDeployment(providerId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
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
			h.slogger.Error("Failed to delete LLM provider deployment", "providerId", providerId, "deploymentId", deploymentId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete deployment"))
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetLLMProviderDeployment handles GET /api/v0.9/llm-providers/{llmProviderId}/deployments/{deploymentId}
func (h *LLMProviderDeploymentHandler) GetLLMProviderDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := r.PathValue("llmProviderId")
	deploymentId := r.PathValue("deploymentId")

	if providerId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	deployment, err := h.deploymentService.GetLLMProviderDeployment(providerId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		default:
			h.slogger.Error("Failed to get LLM provider deployment", "providerId", providerId, "deploymentId", deploymentId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to retrieve deployment"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// GetLLMProviderDeployments handles GET /api/v0.9/llm-providers/{llmProviderId}/deployments
func (h *LLMProviderDeploymentHandler) GetLLMProviderDeployments(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerId := r.PathValue("llmProviderId")
	if providerId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}

	q := r.URL.Query()
	var gatewayId, status *string
	if v := q.Get("gatewayId"); v != "" {
		gatewayId = &v
	}
	if v := q.Get("status"); v != "" {
		status = &v
	}

	deployments, err := h.deploymentService.GetLLMProviderDeployments(providerId, orgId, gatewayId, status)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
			return
		case errors.Is(err, constants.ErrInvalidDeploymentStatus):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid deployment status"))
			return
		default:
			h.slogger.Error("Failed to get LLM provider deployments", "providerId", providerId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to retrieve deployments"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployments)
}

// RegisterRoutes registers all LLM provider deployment-related routes
func (h *LLMProviderDeploymentHandler) RegisterRoutes(mux *http.ServeMux) {
	base := constants.APIBasePath + "/llm-providers/{llmProviderId}"
	mux.HandleFunc("POST "+base+"/deployments", h.DeployLLMProvider)
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/undeploy", h.UndeployLLMProviderDeployment)
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/restore", h.RestoreLLMProviderDeployment)
	mux.HandleFunc("GET "+base+"/deployments", h.GetLLMProviderDeployments)
	mux.HandleFunc("GET "+base+"/deployments/{deploymentId}", h.GetLLMProviderDeployment)
	mux.HandleFunc("DELETE "+base+"/deployments/{deploymentId}", h.DeleteLLMProviderDeployment)
}

// DeployLLMProxy handles POST /api/v0.9/llm-proxies/{llmProxyId}/deployments
func (h *LLMProxyDeploymentHandler) DeployLLMProxy(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := r.PathValue("llmProxyId")
	if proxyId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
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
	if req.GatewayId == (openapi_types.UUID{}) {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayId is required"))
		return
	}

	deployment, err := h.deploymentService.DeployLLMProxy(proxyId, &req, orgId)
	if err != nil {
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
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
				"Base is required (use 'current' or a deploymentId)"))
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
			h.slogger.Error("Failed to deploy LLM proxy", "proxyId", proxyId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to deploy LLM proxy"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusCreated, deployment)
}

// UndeployLLMProxyDeployment handles POST /api/v0.9/llm-proxies/{llmProxyId}/deployments/{deploymentId}/undeploy
func (h *LLMProxyDeploymentHandler) UndeployLLMProxyDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := r.PathValue("llmProxyId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")

	if proxyId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}
	deployment, err := h.deploymentService.UndeployLLMProxyDeployment(proxyId, deploymentId, gatewayId, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: undeployment cannot be initiated from the CP.
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
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
				"No active deployment found for this LLM proxy on the gateway"))
			return
		case errors.Is(err, constants.ErrGatewayIDMismatch):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Deployment is bound to a different gateway"))
			return
		default:
			h.slogger.Error("Failed to undeploy LLM proxy", "proxyId", proxyId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to undeploy deployment"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// RestoreLLMProxyDeployment handles POST /api/v0.9/llm-proxies/{llmProxyId}/deployments/{deploymentId}/restore
func (h *LLMProxyDeploymentHandler) RestoreLLMProxyDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := r.PathValue("llmProxyId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")

	if proxyId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}
	deployment, err := h.deploymentService.RestoreLLMProxyDeployment(proxyId, deploymentId, gatewayId, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: restore cannot be initiated from the CP.
		if respondArtifactGuardError(w, err) {
			return
		}
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
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
			h.slogger.Error("Failed to restore LLM proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "gatewayId", gatewayId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to restore deployment"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// DeleteLLMProxyDeployment handles DELETE /api/v0.9/llm-proxies/{llmProxyId}/deployments/{deploymentId}
func (h *LLMProxyDeploymentHandler) DeleteLLMProxyDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := r.PathValue("llmProxyId")
	deploymentId := r.PathValue("deploymentId")

	if proxyId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	err := h.deploymentService.DeleteLLMProxyDeployment(proxyId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
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
			h.slogger.Error("Failed to delete LLM proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete deployment"))
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetLLMProxyDeployment handles GET /api/v0.9/llm-proxies/{llmProxyId}/deployments/{deploymentId}
func (h *LLMProxyDeploymentHandler) GetLLMProxyDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := r.PathValue("llmProxyId")
	deploymentId := r.PathValue("deploymentId")

	if proxyId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Deployment ID is required"))
		return
	}

	deployment, err := h.deploymentService.GetLLMProxyDeployment(proxyId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
			return
		case errors.Is(err, constants.ErrDeploymentNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Deployment not found"))
			return
		default:
			h.slogger.Error("Failed to get LLM proxy deployment", "proxyId", proxyId, "deploymentId", deploymentId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to retrieve deployment"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// GetLLMProxyDeployments handles GET /api/v0.9/llm-proxies/{llmProxyId}/deployments
func (h *LLMProxyDeploymentHandler) GetLLMProxyDeployments(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	proxyId := r.PathValue("llmProxyId")
	if proxyId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM proxy ID is required"))
		return
	}

	q := r.URL.Query()
	var gatewayId, status *string
	if v := q.Get("gatewayId"); v != "" {
		gatewayId = &v
	}
	if v := q.Get("status"); v != "" {
		status = &v
	}

	deployments, err := h.deploymentService.GetLLMProxyDeployments(proxyId, orgId, gatewayId, status)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
			return
		case errors.Is(err, constants.ErrInvalidDeploymentStatus):
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid deployment status"))
			return
		default:
			h.slogger.Error("Failed to get LLM proxy deployments", "proxyId", proxyId, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Failed to retrieve deployments"))
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployments)
}

// RegisterRoutes registers all LLM proxy deployment-related routes
func (h *LLMProxyDeploymentHandler) RegisterRoutes(mux *http.ServeMux) {
	base := constants.APIBasePath + "/llm-proxies/{llmProxyId}"
	mux.HandleFunc("POST "+base+"/deployments", h.DeployLLMProxy)
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/undeploy", h.UndeployLLMProxyDeployment)
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/restore", h.RestoreLLMProxyDeployment)
	mux.HandleFunc("GET "+base+"/deployments", h.GetLLMProxyDeployments)
	mux.HandleFunc("GET "+base+"/deployments/{deploymentId}", h.GetLLMProxyDeployment)
	mux.HandleFunc("DELETE "+base+"/deployments/{deploymentId}", h.DeleteLLMProxyDeployment)
}

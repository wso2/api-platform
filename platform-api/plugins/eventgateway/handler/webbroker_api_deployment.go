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
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	egservice "github.com/wso2/api-platform/platform-api/plugins/eventgateway/service"

	"github.com/wso2/go-httpkit/httputil"
)

// WebBrokerAPIDeploymentHandler handles deployment routes for WebBroker APIs
type WebBrokerAPIDeploymentHandler struct {
	webbrokerAPIDeploymentService *egservice.WebBrokerAPIDeploymentService
	identity                      *service.IdentityService
	slogger                       *slog.Logger
}

// NewWebBrokerAPIDeploymentHandler creates a new WebBrokerAPIDeploymentHandler
func NewWebBrokerAPIDeploymentHandler(webbrokerAPIDeploymentService *egservice.WebBrokerAPIDeploymentService, identity *service.IdentityService, slogger *slog.Logger) *WebBrokerAPIDeploymentHandler {
	return &WebBrokerAPIDeploymentHandler{
		webbrokerAPIDeploymentService: webbrokerAPIDeploymentService,
		identity:                      identity,
		slogger:                       slogger,
	}
}

// RegisterRoutes registers WebBroker API deployment routes
func (h *WebBrokerAPIDeploymentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/webbroker-apis/{webBrokerApiId}/deployments", h.DeployWebBrokerAPI)
	mux.HandleFunc("POST "+constants.APIBasePath+"/webbroker-apis/{webBrokerApiId}/deployments/{deploymentId}/undeploy", h.UndeployDeployment)
	mux.HandleFunc("POST "+constants.APIBasePath+"/webbroker-apis/{webBrokerApiId}/deployments/{deploymentId}/restore", h.RestoreDeployment)
	mux.HandleFunc("GET "+constants.APIBasePath+"/webbroker-apis/{webBrokerApiId}/deployments", h.GetDeployments)
	mux.HandleFunc("GET "+constants.APIBasePath+"/webbroker-apis/{webBrokerApiId}/deployments/{deploymentId}", h.GetDeployment)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/webbroker-apis/{webBrokerApiId}/deployments/{deploymentId}", h.DeleteDeployment)
}

// DeployWebBrokerAPI handles POST /api/v0.9/webbroker-apis/:apiId/deployments
func (h *WebBrokerAPIDeploymentHandler) DeployWebBrokerAPI(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, apperror.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("webBrokerApiId")
	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "API ID is required"))
		return
	}

	var req api.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	if req.Name == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "name is required"))
		return
	}
	if req.Base == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "base is required"))
		return
	}
	if strings.TrimSpace(req.GatewayId) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "gatewayId is required"))
		return
	}

	createdBy, ok := resolveActor(w, r, h.identity, h.slogger, "deploy WebBroker API")
	if !ok {
		return
	}
	deployment, err := h.webbrokerAPIDeploymentService.DeployWebBrokerAPIByHandle(apiId, &req, orgId, createdBy)
	if err != nil {
		h.handleDeploymentError(w, err, apiId)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, deployment)
}

// UndeployDeployment handles POST /api/v0.9/webbroker-apis/:apiId/deployments/:deploymentId/undeploy
func (h *WebBrokerAPIDeploymentHandler) UndeployDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, apperror.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("webBrokerApiId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "deploymentId is required"))
		return
	}
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "gatewayId is required"))
		return
	}

	deployment, err := h.webbrokerAPIDeploymentService.UndeployWebBrokerAPIDeploymentByHandle(apiId, deploymentId, gatewayId, orgId)
	if err != nil {
		h.handleDeploymentError(w, err, apiId)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// RestoreDeployment handles POST /api/v0.9/webbroker-apis/:apiId/deployments/:deploymentId/restore
func (h *WebBrokerAPIDeploymentHandler) RestoreDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, apperror.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("webBrokerApiId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")
	if deploymentId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "deploymentId is required"))
		return
	}
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "gatewayId is required"))
		return
	}

	deployment, err := h.webbrokerAPIDeploymentService.RestoreWebBrokerAPIDeploymentByHandle(apiId, deploymentId, gatewayId, orgId)
	if err != nil {
		h.handleDeploymentError(w, err, apiId)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// GetDeployments handles GET /api/v0.9/webbroker-apis/:apiId/deployments
func (h *WebBrokerAPIDeploymentHandler) GetDeployments(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, apperror.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("webBrokerApiId")
	if apiId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, apperror.NewErrorResponse(400, "Bad Request", "API ID is required"))
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

	deployments, err := h.webbrokerAPIDeploymentService.GetWebBrokerAPIDeploymentsByHandle(apiId, gatewayId, status, orgId)
	if err != nil {
		h.handleDeploymentError(w, err, apiId)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployments)
}

// GetDeployment handles GET /api/v0.9/webbroker-apis/:apiId/deployments/:deploymentId
func (h *WebBrokerAPIDeploymentHandler) GetDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, apperror.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("webBrokerApiId")
	deploymentId := r.PathValue("deploymentId")

	deployment, err := h.webbrokerAPIDeploymentService.GetWebBrokerAPIDeploymentByHandle(apiId, deploymentId, orgId)
	if err != nil {
		h.handleDeploymentError(w, err, apiId)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
}

// DeleteDeployment handles DELETE /api/v0.9/webbroker-apis/:apiId/deployments/:deploymentId
func (h *WebBrokerAPIDeploymentHandler) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, apperror.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiId := r.PathValue("webBrokerApiId")
	deploymentId := r.PathValue("deploymentId")

	if err := h.webbrokerAPIDeploymentService.DeleteWebBrokerAPIDeploymentByHandle(apiId, deploymentId, orgId); err != nil {
		h.handleDeploymentError(w, err, apiId)
		return
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *WebBrokerAPIDeploymentHandler) handleDeploymentError(w http.ResponseWriter, err error, apiId string) {
	// Deployment conditions (gateway/deployment not found, restore conflicts,
	// gateway mismatch, validation failures) are catalog errors raised by the
	// service and already carry status, code, and a sterile client message.
	if respondCatalogError(w, h.slogger, err) {
		return
	}
	h.slogger.Error("WebBroker API deployment error", "apiId", apiId, "error", err)
	httputil.WriteJSON(w, http.StatusInternalServerError, apperror.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
}

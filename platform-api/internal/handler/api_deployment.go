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

type DeploymentHandler struct {
	deploymentService *service.DeploymentService
	identity          *service.IdentityService
	slogger           *slog.Logger
}

func NewDeploymentHandler(deploymentService *service.DeploymentService, identity *service.IdentityService, slogger *slog.Logger) *DeploymentHandler {
	return &DeploymentHandler{
		deploymentService: deploymentService,
		identity:          identity,
		slogger:           slogger,
	}
}

// DeployAPI handles POST /api/v0.9/rest-apis/:apiId/deployments
// Creates a new immutable deployment artifact and deploys it to a gateway
func (h *DeploymentHandler) DeployAPI(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	var req api.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	// Validate required fields
	if req.Name == "" {
		return apperror.RESTAPIDeploymentValidationFailed.New("name is required")
	}
	if req.Base == "" {
		return apperror.RESTAPIDeploymentValidationFailed.New("base is required (use 'current' or a deploymentId)")
	}
	if strings.TrimSpace(req.GatewayId) == "" {
		return apperror.RESTAPIDeploymentValidationFailed.New("gatewayId is required")
	}

	createdBy, err := resolveActorErr(r, h.identity, "deploy API")
	if err != nil {
		return err
	}
	deployment, err := h.deploymentService.DeployAPIByHandle(apiId, &req, orgId, createdBy)
	if err != nil {
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			return apperror.GatewayNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrBaseDeploymentNotFound) {
			return apperror.DeploymentBaseNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrDeploymentNameRequired) {
			return apperror.RESTAPIDeploymentValidationFailed.Wrap(err, "Deployment name is required")
		}
		if errors.Is(err, constants.ErrDeploymentBaseRequired) {
			return apperror.RESTAPIDeploymentValidationFailed.Wrap(err, "Base is required (use 'current' or a deploymentId)")
		}
		if errors.Is(err, constants.ErrDeploymentGatewayIDRequired) {
			return apperror.RESTAPIDeploymentValidationFailed.Wrap(err, "Gateway ID is required")
		}
		if errors.Is(err, constants.ErrAPINoBackendServices) {
			return apperror.RESTAPIDeploymentValidationFailed.Wrap(err, "API must have at least one backend service attached before deployment")
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to deploy API %s", apiId))
	}

	httputil.WriteJSON(w, http.StatusCreated, deployment)
	return nil
}

// UndeployDeployment handles POST /api/v0.9/rest-apis/:apiId/deployments/:deploymentId/undeploy
func (h *DeploymentHandler) UndeployDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")
	if deploymentId == "" {
		return apperror.ValidationFailed.New("deploymentId is required")
	}
	if gatewayId == "" {
		return apperror.ValidationFailed.New("gatewayId is required")
	}
	if deploymentId == "00000000-0000-0000-0000-000000000000" || gatewayId == "00000000-0000-0000-0000-000000000000" {
		return apperror.ValidationFailed.New("deploymentId/gatewayId cannot be zero-value UUID")
	}

	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}
	actor, err := resolveActorErr(r, h.identity, "undeploy API")
	if err != nil {
		return err
	}
	deployment, err := h.deploymentService.UndeployDeploymentByHandle(apiId, deploymentId, gatewayId, orgId, actor)
	if err != nil {
		// DP-originated artifacts are read-only: undeployment cannot be initiated from the CP.
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrDeploymentNotFound) {
			return apperror.DeploymentNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			return apperror.GatewayNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			return apperror.DeploymentNotActive.Wrap(err, "API")
		}
		if errors.Is(err, constants.ErrGatewayIDMismatch) {
			return apperror.DeploymentGatewayMismatch.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to undeploy API %s deployment %s from gateway %s", apiId, deploymentId, gatewayId))
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// RestoreDeployment handles POST /api/v0.9/rest-apis/:apiId/deployments/:deploymentId/restore
func (h *DeploymentHandler) RestoreDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")
	if deploymentId == "" {
		return apperror.ValidationFailed.New("deploymentId is required")
	}
	if gatewayId == "" {
		return apperror.ValidationFailed.New("gatewayId is required")
	}
	if deploymentId == "00000000-0000-0000-0000-000000000000" || gatewayId == "00000000-0000-0000-0000-000000000000" {
		return apperror.ValidationFailed.New("deploymentId/gatewayId cannot be zero-value UUID")
	}

	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}
	actor, err := resolveActorErr(r, h.identity, "restore API deployment")
	if err != nil {
		return err
	}
	deployment, err := h.deploymentService.RestoreDeploymentByHandle(apiId, deploymentId, gatewayId, orgId, actor)
	if err != nil {
		// DP-originated artifacts are read-only: restore cannot be initiated from the CP.
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrDeploymentNotFound) {
			return apperror.DeploymentNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			return apperror.GatewayNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrDeploymentAlreadyDeployed) {
			return apperror.DeploymentRestoreConflict.Wrap(err)
		}
		if errors.Is(err, constants.ErrGatewayIDMismatch) {
			return apperror.DeploymentGatewayMismatch.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to restore API %s deployment %s on gateway %s", apiId, deploymentId, gatewayId))
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// DeleteDeployment handles DELETE /api/v0.9/rest-apis/:apiId/deployments/:deploymentId
// Permanently deletes an undeployed deployment artifact
func (h *DeploymentHandler) DeleteDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	deploymentId := r.PathValue("deploymentId")

	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}
	if deploymentId == "" {
		return apperror.ValidationFailed.New("Deployment ID is required")
	}

	actor, err := resolveActorErr(r, h.identity, "delete API deployment")
	if err != nil {
		return err
	}
	if err := h.deploymentService.DeleteDeploymentByHandle(apiId, deploymentId, orgId, actor); err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrDeploymentNotFound) {
			return apperror.DeploymentNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrDeploymentIsDeployed) {
			return apperror.DeploymentActive.Wrap(err)
		}
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to delete API %s deployment %s", apiId, deploymentId))
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GetDeployment handles GET /api/v0.9/rest-apis/:apiId/deployments/:deploymentId
// Retrieves metadata for a specific deployment artifact
func (h *DeploymentHandler) GetDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	deploymentId := r.PathValue("deploymentId")

	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}
	if deploymentId == "" {
		return apperror.ValidationFailed.New("Deployment ID is required")
	}

	deployment, err := h.deploymentService.GetDeploymentByHandle(apiId, deploymentId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrDeploymentNotFound) {
			return apperror.DeploymentNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to get API %s deployment %s", apiId, deploymentId))
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// GetDeployments handles GET /api/v0.9/rest-apis/:apiId/deployments
// Retrieves all deployment records for an API with optional filters
func (h *DeploymentHandler) GetDeployments(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiId := r.PathValue("restApiId")
	if apiId == "" {
		return apperror.ValidationFailed.New("API ID is required")
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

	deployments, err := h.deploymentService.GetDeploymentsByHandle(apiId, gatewayId, status, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrInvalidDeploymentStatus) {
			return apperror.DeploymentInvalidStatus.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to get deployments for API %s", apiId))
	}

	httputil.WriteJSON(w, http.StatusOK, deployments)
	return nil
}

// RegisterRoutes registers all deployment-related routes
func (h *DeploymentHandler) RegisterRoutes(mux *http.ServeMux) {
	h.slogger.Debug("Registering deployment routes")
	base := constants.APIBasePath + "/rest-apis/{restApiId}"
	mux.HandleFunc("POST "+base+"/deployments", middleware.MapErrors(h.slogger, h.DeployAPI))
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/undeploy", middleware.MapErrors(h.slogger, h.UndeployDeployment))
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/restore", middleware.MapErrors(h.slogger, h.RestoreDeployment))
	mux.HandleFunc("GET "+base+"/deployments", middleware.MapErrors(h.slogger, h.GetDeployments))
	mux.HandleFunc("GET "+base+"/deployments/{deploymentId}", middleware.MapErrors(h.slogger, h.GetDeployment))
	mux.HandleFunc("DELETE "+base+"/deployments/{deploymentId}", middleware.MapErrors(h.slogger, h.DeleteDeployment))
}

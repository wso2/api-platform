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
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/go-httpkit/httputil"
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
func (h *LLMProviderDeploymentHandler) DeployLLMProvider(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	providerId := r.PathValue("llmProviderId")
	if providerId == "" {
		return apperror.ValidationFailed.New("LLM provider ID is required")
	}

	var req api.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid LLM provider deployment request body for provider %s", providerId))
	}

	if req.Name == "" {
		return apperror.LLMProviderDeploymentValidationFailed.New("name is required")
	}
	if req.Base == "" {
		return apperror.LLMProviderDeploymentValidationFailed.New("base is required (use 'current' or a deploymentId)")
	}
	if strings.TrimSpace(req.GatewayId) == "" {
		return apperror.LLMProviderDeploymentValidationFailed.New("gatewayId is required")
	}

	deployment, err := h.deploymentService.DeployLLMProvider(providerId, &req, orgId)
	if err != nil {
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			return apperror.LLMProviderNotFound.Wrap(err)
		case errors.Is(err, constants.ErrGatewayNotFound):
			return apperror.GatewayNotFound.Wrap(err)
		case errors.Is(err, constants.ErrBaseDeploymentNotFound):
			return apperror.DeploymentBaseNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNameRequired):
			return apperror.LLMProviderDeploymentValidationFailed.Wrap(err, "Deployment name is required")
		case errors.Is(err, constants.ErrDeploymentBaseRequired):
			return apperror.LLMProviderDeploymentValidationFailed.Wrap(err, "Base is required (use 'current' or a deploymentId)")
		case errors.Is(err, constants.ErrDeploymentGatewayIDRequired):
			return apperror.LLMProviderDeploymentValidationFailed.Wrap(err, "Gateway ID is required")
		case errors.Is(err, constants.ErrLLMProviderTemplateNotFound):
			return apperror.LLMProviderDeploymentValidationFailed.Wrap(err, "Referenced template not found")
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.LLMProviderDeploymentValidationFailed.Wrap(err, "Invalid input")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to deploy LLM provider %s", providerId))
		}
	}

	httputil.WriteJSON(w, http.StatusCreated, deployment)
	return nil
}

// UndeployLLMProviderDeployment handles POST /api/v0.9/llm-providers/{llmProviderId}/deployments/{deploymentId}/undeploy
func (h *LLMProviderDeploymentHandler) UndeployLLMProviderDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	providerId := r.PathValue("llmProviderId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")

	if providerId == "" {
		return apperror.ValidationFailed.New("LLM provider ID is required")
	}
	deployment, err := h.deploymentService.UndeployLLMProviderDeployment(providerId, deploymentId, gatewayId, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: undeployment cannot be initiated from the CP.
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			return apperror.LLMProviderNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotFound):
			return apperror.DeploymentNotFound.Wrap(err)
		case errors.Is(err, constants.ErrGatewayNotFound):
			return apperror.GatewayNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotActive):
			return apperror.DeploymentNotActive.Wrap(err, "LLM provider")
		case errors.Is(err, constants.ErrGatewayIDMismatch):
			return apperror.DeploymentGatewayMismatch.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to undeploy LLM provider %s deployment %s on gateway %q", providerId, deploymentId, gatewayId))
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// RestoreLLMProviderDeployment handles POST /api/v0.9/llm-providers/{llmProviderId}/deployments/{deploymentId}/restore
func (h *LLMProviderDeploymentHandler) RestoreLLMProviderDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	providerId := r.PathValue("llmProviderId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")

	if providerId == "" {
		return apperror.ValidationFailed.New("LLM provider ID is required")
	}
	deployment, err := h.deploymentService.RestoreLLMProviderDeployment(providerId, deploymentId, gatewayId, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: restore cannot be initiated from the CP.
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			return apperror.LLMProviderNotFound.Wrap(err)
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
				WithLogMessage(fmt.Sprintf("failed to restore LLM provider %s deployment %s on gateway %q", providerId, deploymentId, gatewayId))
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// DeleteLLMProviderDeployment handles DELETE /api/v0.9/llm-providers/{llmProviderId}/deployments/{deploymentId}
func (h *LLMProviderDeploymentHandler) DeleteLLMProviderDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	providerId := r.PathValue("llmProviderId")
	deploymentId := r.PathValue("deploymentId")

	if providerId == "" {
		return apperror.ValidationFailed.New("LLM provider ID is required")
	}
	if deploymentId == "" {
		return apperror.ValidationFailed.New("Deployment ID is required")
	}

	err := h.deploymentService.DeleteLLMProviderDeployment(providerId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			return apperror.LLMProviderNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotFound):
			return apperror.DeploymentNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentIsDeployed):
			return apperror.DeploymentActive.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to delete LLM provider %s deployment %s", providerId, deploymentId))
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GetLLMProviderDeployment handles GET /api/v0.9/llm-providers/{llmProviderId}/deployments/{deploymentId}
func (h *LLMProviderDeploymentHandler) GetLLMProviderDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	providerId := r.PathValue("llmProviderId")
	deploymentId := r.PathValue("deploymentId")

	if providerId == "" {
		return apperror.ValidationFailed.New("LLM provider ID is required")
	}
	if deploymentId == "" {
		return apperror.ValidationFailed.New("Deployment ID is required")
	}

	deployment, err := h.deploymentService.GetLLMProviderDeployment(providerId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProviderNotFound):
			return apperror.LLMProviderNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotFound):
			return apperror.DeploymentNotFound.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to get LLM provider %s deployment %s", providerId, deploymentId))
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// GetLLMProviderDeployments handles GET /api/v0.9/llm-providers/{llmProviderId}/deployments
func (h *LLMProviderDeploymentHandler) GetLLMProviderDeployments(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	providerId := r.PathValue("llmProviderId")
	if providerId == "" {
		return apperror.ValidationFailed.New("LLM provider ID is required")
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
			return apperror.LLMProviderNotFound.Wrap(err)
		case errors.Is(err, constants.ErrInvalidDeploymentStatus):
			return apperror.DeploymentInvalidStatus.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to get LLM provider %s deployments", providerId))
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployments)
	return nil
}

// RegisterRoutes registers all LLM provider deployment-related routes
func (h *LLMProviderDeploymentHandler) RegisterRoutes(mux *http.ServeMux) {
	base := constants.APIBasePath + "/llm-providers/{llmProviderId}"
	mux.HandleFunc("POST "+base+"/deployments", middleware.MapErrors(h.slogger, h.DeployLLMProvider))
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/undeploy", middleware.MapErrors(h.slogger, h.UndeployLLMProviderDeployment))
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/restore", middleware.MapErrors(h.slogger, h.RestoreLLMProviderDeployment))
	mux.HandleFunc("GET "+base+"/deployments", middleware.MapErrors(h.slogger, h.GetLLMProviderDeployments))
	mux.HandleFunc("GET "+base+"/deployments/{deploymentId}", middleware.MapErrors(h.slogger, h.GetLLMProviderDeployment))
	mux.HandleFunc("DELETE "+base+"/deployments/{deploymentId}", middleware.MapErrors(h.slogger, h.DeleteLLMProviderDeployment))
}

// DeployLLMProxy handles POST /api/v0.9/llm-proxies/{llmProxyId}/deployments
func (h *LLMProxyDeploymentHandler) DeployLLMProxy(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyId := r.PathValue("llmProxyId")
	if proxyId == "" {
		return apperror.ValidationFailed.New("LLM proxy ID is required")
	}

	var req api.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid LLM proxy deployment request body for proxy %s", proxyId))
	}

	if req.Name == "" {
		return apperror.LLMProxyDeploymentValidationFailed.New("name is required")
	}
	if req.Base == "" {
		return apperror.LLMProxyDeploymentValidationFailed.New("base is required (use 'current' or a deploymentId)")
	}
	if strings.TrimSpace(req.GatewayId) == "" {
		return apperror.LLMProxyDeploymentValidationFailed.New("gatewayId is required")
	}

	deployment, err := h.deploymentService.DeployLLMProxy(proxyId, &req, orgId)
	if err != nil {
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			return apperror.LLMProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrGatewayNotFound):
			return apperror.GatewayNotFound.Wrap(err)
		case errors.Is(err, constants.ErrBaseDeploymentNotFound):
			return apperror.DeploymentBaseNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNameRequired):
			return apperror.LLMProxyDeploymentValidationFailed.Wrap(err, "Deployment name is required")
		case errors.Is(err, constants.ErrDeploymentBaseRequired):
			return apperror.LLMProxyDeploymentValidationFailed.Wrap(err, "Base is required (use 'current' or a deploymentId)")
		case errors.Is(err, constants.ErrDeploymentGatewayIDRequired):
			return apperror.LLMProxyDeploymentValidationFailed.Wrap(err, "Gateway ID is required")
		case errors.Is(err, constants.ErrInvalidInput):
			return apperror.LLMProxyDeploymentValidationFailed.Wrap(err, "Invalid input")
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to deploy LLM proxy %s", proxyId))
		}
	}

	httputil.WriteJSON(w, http.StatusCreated, deployment)
	return nil
}

// UndeployLLMProxyDeployment handles POST /api/v0.9/llm-proxies/{llmProxyId}/deployments/{deploymentId}/undeploy
func (h *LLMProxyDeploymentHandler) UndeployLLMProxyDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyId := r.PathValue("llmProxyId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")

	if proxyId == "" {
		return apperror.ValidationFailed.New("LLM proxy ID is required")
	}
	deployment, err := h.deploymentService.UndeployLLMProxyDeployment(proxyId, deploymentId, gatewayId, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: undeployment cannot be initiated from the CP.
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			return apperror.LLMProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotFound):
			return apperror.DeploymentNotFound.Wrap(err)
		case errors.Is(err, constants.ErrGatewayNotFound):
			return apperror.GatewayNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotActive):
			return apperror.DeploymentNotActive.Wrap(err, "LLM proxy")
		case errors.Is(err, constants.ErrGatewayIDMismatch):
			return apperror.DeploymentGatewayMismatch.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to undeploy LLM proxy %s deployment %s on gateway %q", proxyId, deploymentId, gatewayId))
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// RestoreLLMProxyDeployment handles POST /api/v0.9/llm-proxies/{llmProxyId}/deployments/{deploymentId}/restore
func (h *LLMProxyDeploymentHandler) RestoreLLMProxyDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyId := r.PathValue("llmProxyId")
	deploymentId := r.PathValue("deploymentId")
	gatewayId := r.URL.Query().Get("gatewayId")

	if proxyId == "" {
		return apperror.ValidationFailed.New("LLM proxy ID is required")
	}
	deployment, err := h.deploymentService.RestoreLLMProxyDeployment(proxyId, deploymentId, gatewayId, orgId)
	if err != nil {
		// DP-originated artifacts are read-only: restore cannot be initiated from the CP.
		if guardErr := mapArtifactGuardError(err); guardErr != nil {
			return guardErr
		}
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			return apperror.LLMProxyNotFound.Wrap(err)
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
				WithLogMessage(fmt.Sprintf("failed to restore LLM proxy %s deployment %s on gateway %q", proxyId, deploymentId, gatewayId))
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// DeleteLLMProxyDeployment handles DELETE /api/v0.9/llm-proxies/{llmProxyId}/deployments/{deploymentId}
func (h *LLMProxyDeploymentHandler) DeleteLLMProxyDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyId := r.PathValue("llmProxyId")
	deploymentId := r.PathValue("deploymentId")

	if proxyId == "" {
		return apperror.ValidationFailed.New("LLM proxy ID is required")
	}
	if deploymentId == "" {
		return apperror.ValidationFailed.New("Deployment ID is required")
	}

	err := h.deploymentService.DeleteLLMProxyDeployment(proxyId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			return apperror.LLMProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotFound):
			return apperror.DeploymentNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentIsDeployed):
			return apperror.DeploymentActive.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to delete LLM proxy %s deployment %s", proxyId, deploymentId))
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GetLLMProxyDeployment handles GET /api/v0.9/llm-proxies/{llmProxyId}/deployments/{deploymentId}
func (h *LLMProxyDeploymentHandler) GetLLMProxyDeployment(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyId := r.PathValue("llmProxyId")
	deploymentId := r.PathValue("deploymentId")

	if proxyId == "" {
		return apperror.ValidationFailed.New("LLM proxy ID is required")
	}
	if deploymentId == "" {
		return apperror.ValidationFailed.New("Deployment ID is required")
	}

	deployment, err := h.deploymentService.GetLLMProxyDeployment(proxyId, deploymentId, orgId)
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrLLMProxyNotFound):
			return apperror.LLMProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrDeploymentNotFound):
			return apperror.DeploymentNotFound.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to get LLM proxy %s deployment %s", proxyId, deploymentId))
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployment)
	return nil
}

// GetLLMProxyDeployments handles GET /api/v0.9/llm-proxies/{llmProxyId}/deployments
func (h *LLMProxyDeploymentHandler) GetLLMProxyDeployments(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyId := r.PathValue("llmProxyId")
	if proxyId == "" {
		return apperror.ValidationFailed.New("LLM proxy ID is required")
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
			return apperror.LLMProxyNotFound.Wrap(err)
		case errors.Is(err, constants.ErrInvalidDeploymentStatus):
			return apperror.DeploymentInvalidStatus.Wrap(err)
		default:
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to get LLM proxy %s deployments", proxyId))
		}
	}

	httputil.WriteJSON(w, http.StatusOK, deployments)
	return nil
}

// RegisterRoutes registers all LLM proxy deployment-related routes
func (h *LLMProxyDeploymentHandler) RegisterRoutes(mux *http.ServeMux) {
	base := constants.APIBasePath + "/llm-proxies/{llmProxyId}"
	mux.HandleFunc("POST "+base+"/deployments", middleware.MapErrors(h.slogger, h.DeployLLMProxy))
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/undeploy", middleware.MapErrors(h.slogger, h.UndeployLLMProxyDeployment))
	mux.HandleFunc("POST "+base+"/deployments/{deploymentId}/restore", middleware.MapErrors(h.slogger, h.RestoreLLMProxyDeployment))
	mux.HandleFunc("GET "+base+"/deployments", middleware.MapErrors(h.slogger, h.GetLLMProxyDeployments))
	mux.HandleFunc("GET "+base+"/deployments/{deploymentId}", middleware.MapErrors(h.slogger, h.GetLLMProxyDeployment))
	mux.HandleFunc("DELETE "+base+"/deployments/{deploymentId}", middleware.MapErrors(h.slogger, h.DeleteLLMProxyDeployment))
}

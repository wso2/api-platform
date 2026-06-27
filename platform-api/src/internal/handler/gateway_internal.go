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
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"
	"strconv"
	"strings"
	"time"

	"platform-api/src/internal/service"

	"github.com/wso2/go-httpkit/httputil"
)

type GatewayInternalAPIHandler struct {
	gatewayService         *service.GatewayService
	gatewayInternalService *service.GatewayInternalAPIService
	artifactImportService  *service.ArtifactImportService
	hmacSecretService      *service.WebSubAPIHmacSecretService
	secretService          *service.SecretService
	slogger                *slog.Logger
}

func NewGatewayInternalAPIHandler(gatewayService *service.GatewayService,
	gatewayInternalService *service.GatewayInternalAPIService,
	hmacSecretService *service.WebSubAPIHmacSecretService, artifactImportService *service.ArtifactImportService,
	secretService *service.SecretService,
	slogger *slog.Logger) *GatewayInternalAPIHandler {
	return &GatewayInternalAPIHandler{
		gatewayService:         gatewayService,
		gatewayInternalService: gatewayInternalService,
		hmacSecretService:      hmacSecretService,
		secretService:          secretService,
		artifactImportService:  artifactImportService,
		slogger:                slogger,
	}
}

// authenticateGateway validates the API key and returns the authenticated gateway.
func (h *GatewayInternalAPIHandler) authenticateGateway(apiKey string) (*model.Gateway, error) {
	if apiKey == "" {
		return nil, constants.ErrMissingAPIKey
	}
	return h.gatewayService.VerifyToken(apiKey)
}

// authenticateRequest extracts the API key from headers and authenticates the gateway.
func (h *GatewayInternalAPIHandler) authenticateRequest(w http.ResponseWriter, r *http.Request) (orgID, gatewayID string, ok bool) {
	clientIP := r.RemoteAddr
	if i := strings.LastIndex(clientIP, ":"); i != -1 {
		clientIP = clientIP[:i]
	}
	apiKey := r.Header.Get("api-key")

	gateway, err := h.authenticateGateway(apiKey)
	if err != nil {
		if errors.Is(err, constants.ErrMissingAPIKey) {
			h.slogger.Warn("Unauthorized access attempt - Missing API key", "clientIP", clientIP)
			httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
				"API key is required. Provide 'api-key' header."))
		} else if errors.Is(err, constants.ErrInvalidAPIToken) {
			h.slogger.Warn("Authentication failed - Invalid API key", "clientIP", clientIP)
			httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
				"Invalid or expired API key"))
		} else {
			h.slogger.Error("Authentication failed", "clientIP", clientIP, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Error while validating API key"))
		}
		return "", "", false
	}
	return gateway.OrganizationID, gateway.ID, true
}

// GetAPI handles GET /api/internal/v1/apis/:apiId
func (h *GatewayInternalAPIHandler) GetAPI(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	apiID := r.PathValue("apiId")
	if apiID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	api, err := h.gatewayInternalService.GetActiveDeploymentByGateway(apiID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"No active deployment found for this API on this gateway"))
			return
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get API"))
		return
	}

	// Create ZIP file from API YAML file
	zipData, err := utils.CreateAPIYamlZip(api)
	if err != nil {
		h.slogger.Error("Failed to create ZIP file", "apiID", apiID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create API package"))
		return
	}

	// Set headers for ZIP file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"api-%s.zip\"", apiID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipData)
}

// ImportGatewayArtifacts handles POST /api/internal/v1/artifacts/import-gateway-artifacts.
// It is the generic bulk DP->CP push endpoint. The request is multipart/form-data with an
// "artifacts" zip part (containing the artifacts.json file — a JSON array of
// ImportGatewayArtifactRequest) and an advisory "total" field. The control plane creates or
// updates each artifact (read-only, origin "gateway_api") in dependency order and is
// continue-on-error: a failure on one artifact is recorded against its dpid and does not
// abort the rest. The response maps each artifact's dpid to its result, with total/success/
// failed counts. Only a malformed request (bad multipart/zip) returns a non-200.
func (h *GatewayInternalAPIHandler) ImportGatewayArtifacts(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	reqs, err := utils.ParseGatewayArtifactsRequest(r)
	if err != nil {
		clientIP := r.RemoteAddr
		if i := strings.LastIndex(clientIP, ":"); i != -1 {
			clientIP = clientIP[:i]
		}
		h.slogger.Warn("Invalid import-gateway-artifacts request", "clientIP", clientIP, "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}
	// 'total' is advisory: log a mismatch but proceed with what the zip actually contained.
	if totalStr := r.FormValue("total"); totalStr != "" {
		if total, convErr := strconv.Atoi(totalStr); convErr == nil && total != len(reqs) {
			h.slogger.Warn("import-gateway-artifacts total mismatch",
				"declaredTotal", total, "zipCount", len(reqs), "gatewayID", gatewayID)
		}
	}

	resp := h.artifactImportService.ImportArtifacts(orgID, gatewayID, reqs)
	h.slogger.Info("Imported gateway artifacts batch",
		"gatewayID", gatewayID, "total", resp.Total, "success", resp.Success, "failed", resp.Failed)
	httputil.WriteJSON(w, http.StatusOK, resp)
}

// GetLLMProvider handles GET /api/internal/v1/llm-providers/:providerId
func (h *GatewayInternalAPIHandler) GetLLMProvider(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	providerID := r.PathValue("providerId")
	if providerID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Provider ID is required"))
		return
	}

	provider, err := h.gatewayInternalService.GetActiveLLMProviderDeploymentByGateway(providerID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"No active deployment found for this LLM provider on this gateway"))
			return
		}
		if errors.Is(err, constants.ErrLLMProviderNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
			return
		}
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get LLM provider"))
		return
	}

	// Create ZIP file from LLM provider YAML file
	zipData, err := utils.CreateLLMProviderYamlZip(provider)
	if err != nil {
		h.slogger.Error("Failed to create ZIP file", "providerID", providerID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create LLM provider package"))
		return
	}

	// Set headers for ZIP file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"llm-provider-%s.zip\"", providerID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipData)
}

// GetLLMProxy handles GET /api/internal/v1/llm-proxies/:proxyId
func (h *GatewayInternalAPIHandler) GetLLMProxy(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	proxyID := r.PathValue("proxyId")
	if proxyID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Proxy ID is required"))
		return
	}

	proxy, err := h.gatewayInternalService.GetActiveLLMProxyDeploymentByGateway(proxyID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"No active deployment found for this LLM proxy on this gateway"))
			return
		}
		if errors.Is(err, constants.ErrLLMProxyNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM proxy not found"))
			return
		}
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get LLM proxy"))
		return
	}

	// Create ZIP file from LLM proxy YAML file
	zipData, err := utils.CreateLLMProxyYamlZip(proxy)
	if err != nil {
		h.slogger.Error("Failed to create ZIP file", "proxyID", proxyID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create LLM proxy package"))
		return
	}

	// Set headers for ZIP file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"llm-proxy-%s.zip\"", proxyID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipData)
}

// GetGatewayDeployments handles GET /api/internal/v1/deployments
// Returns the list of deployments that should be active on a gateway for startup sync
func (h *GatewayInternalAPIHandler) GetGatewayDeployments(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	// Parse optional "since" query parameter for incremental sync
	var since *time.Time
	sinceStr := r.URL.Query().Get("since")
	if sinceStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid 'since' parameter. Expected ISO 8601 format (e.g., 2026-03-04T10:00:00Z)"))
			return
		}
		since = &parsedTime
	}

	deployments, err := h.gatewayInternalService.GetDeploymentsByGateway(orgID, gatewayID, since)
	if err != nil {
		if errors.Is(err, constants.ErrGatewayNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		h.slogger.Error("Failed to get gateway deployments", "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get deployments"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, deployments)
}

// BatchFetchDeployments handles POST /api/internal/v1/deployments/fetch-batch
// Fetches multiple deployment artifacts in a single request for gateway startup sync
func (h *GatewayInternalAPIHandler) BatchFetchDeployments(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	// Enforce Accept header - only application/x-tar+gzip is supported
	if accept := r.Header.Get("Accept"); accept != "application/x-tar+gzip" {
		httputil.WriteJSON(w, http.StatusNotAcceptable, utils.NewErrorResponse(406, "Not Acceptable",
			"This endpoint only supports Accept: application/x-tar+gzip"))
		return
	}

	// Parse request body
	var req dto.DeploymentsBatchFetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body: "+err.Error()))
		return
	}

	if len(req.DeploymentIDs) == 0 {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"At least one deployment ID is required"))
		return
	}

	// Fetch deployment content
	contentMap, err := h.gatewayInternalService.GetDeploymentContentBatch(orgID, gatewayID, req.DeploymentIDs)
	if err != nil {
		if errors.Is(err, constants.ErrGatewayNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		h.slogger.Error("Failed to get deployment content batch", "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get deployment content"))
		return
	}

	// Create TAR.GZ archive from deployment content
	tarGzData, err := utils.CreateBatchDeploymentTarGz(contentMap)
	if err != nil {
		h.slogger.Error("Failed to create batch TAR.GZ archive", "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create deployment package"))
		return
	}

	// Set headers for TAR.GZ download
	w.Header().Set("Content-Type", "application/x-tar+gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"deployments-batch-%s.tar.gz\"", gatewayID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(tarGzData)))

	// Return TAR.GZ archive
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(tarGzData)
}

// GetSubscriptions handles GET /api/internal/v1/apis/:apiId/subscriptions
func (h *GatewayInternalAPIHandler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	apiID := r.PathValue("apiId")
	if apiID == "" {
		clientIP := r.RemoteAddr
		if i := strings.LastIndex(clientIP, ":"); i != -1 {
			clientIP = clientIP[:i]
		}
		h.slogger.Error("API ID is required for subscriptions request",
			"clientIP", clientIP,
			"organizationId", orgID,
			"apiId", apiID)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	if err := h.gatewayInternalService.IsAPIDeployedOnGateway(apiID, gatewayID, orgID); err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found when listing subscriptions",
				"apiId", apiID,
				"organizationId", orgID,
				"gatewayId", gatewayID,
				"error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			h.slogger.Error("Subscription list denied - API has no active deployment status on gateway",
				"apiId", apiID,
				"organizationId", orgID,
				"gatewayId", gatewayID)
			httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden",
				"API is not associated with this gateway"))
			return
		}
		h.slogger.Error("Failed to verify API deployment for subscriptions",
			"apiId", apiID,
			"organizationId", orgID,
			"gatewayId", gatewayID,
			"error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to verify API deployment"))
		return
	}

	subs, err := h.gatewayInternalService.ListSubscriptionsForAPI(apiID, orgID)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found when listing subscriptions",
				"apiId", apiID,
				"organizationId", orgID,
				"error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API not found"))
			return
		}
		h.slogger.Error("Failed to list subscriptions for API",
			"apiId", apiID,
			"organizationId", orgID,
			"error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get subscriptions"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, subs)
}

// GetSubscriptionPlans handles GET /api/internal/v1/subscription-plans
func (h *GatewayInternalAPIHandler) GetSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	orgID, _, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	plans, err := h.gatewayInternalService.ListSubscriptionPlansForOrg(orgID)
	if err != nil {
		h.slogger.Error("Failed to list subscription plans",
			"organizationId", orgID,
			"error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get subscription plans"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, plans)
}

// GetMCPProxy handles GET /api/internal/v1/mcp-proxies/:proxyId
func (h *GatewayInternalAPIHandler) GetMCPProxy(w http.ResponseWriter, r *http.Request) {

	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	proxyID := r.PathValue("proxyId")
	if proxyID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Proxy ID is required"))
		return
	}

	proxy, err := h.gatewayInternalService.GetActiveMCPProxyDeploymentByGateway(proxyID, orgID, gatewayID)
	if err != nil {
		clientIP := r.RemoteAddr
		if i := strings.LastIndex(clientIP, ":"); i != -1 {
			clientIP = clientIP[:i]
		}
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			h.slogger.Error("No active deployment found for MCP proxy", "clientIP", clientIP, "proxyID", proxyID, "orgID", orgID, "gatewayID", gatewayID, "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"No active deployment found for this MCP proxy on this gateway"))
			return
		}
		if errors.Is(err, constants.ErrMCPProxyNotFound) {
			h.slogger.Error("MCP proxy not found", "clientIP", clientIP, "proxyID", proxyID, "orgID", orgID, "gatewayID", gatewayID, "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"MCP proxy not found"))
			return
		}
		h.slogger.Error("Failed to get MCP proxy", "clientIP", clientIP, "proxyID", proxyID, "orgID", orgID, "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get MCP proxy"))
		return
	}

	// Create ZIP file from MCP proxy YAML file
	zipData, err := utils.CreateMCPProxyYamlZip(proxy)
	if err != nil {
		h.slogger.Error("Failed to create ZIP file", "proxyID", proxyID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create MCP proxy package"))
		return
	}

	// Set headers for ZIP file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"mcp-proxy-%s.zip\"", proxyID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipData)
}

// GetWebSubAPI handles GET /api/internal/v1/websub-apis/:apiId
func (h *GatewayInternalAPIHandler) GetWebSubAPI(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	apiID := r.PathValue("apiId")
	if apiID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	api, err := h.gatewayInternalService.GetActiveWebSubAPIDeploymentByGateway(apiID, orgID, gatewayID)
	if err != nil {
		clientIP := r.RemoteAddr
		if i := strings.LastIndex(clientIP, ":"); i != -1 {
			clientIP = clientIP[:i]
		}
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			h.slogger.Error("No active deployment found for WebSub API", "clientIP", clientIP, "apiID", apiID, "orgID", orgID, "gatewayID", gatewayID, "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"No active deployment found for this WebSub API on this gateway"))
			return
		}
		if errors.Is(err, constants.ErrWebSubAPINotFound) {
			h.slogger.Error("WebSub API not found", "clientIP", clientIP, "apiID", apiID, "orgID", orgID, "gatewayID", gatewayID, "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"WebSub API not found"))
			return
		}
		h.slogger.Error("Failed to get WebSub API", "clientIP", clientIP, "apiID", apiID, "orgID", orgID, "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get WebSub API"))
		return
	}

	// Create ZIP file from WebSub API YAML file
	zipData, err := utils.CreateWebSubAPIYamlZip(api)
	if err != nil {
		h.slogger.Error("Failed to create ZIP file", "apiID", apiID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create WebSub API package"))
		return
	}

	// Set headers for ZIP file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"websub-api-%s.zip\"", apiID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipData)
}

// GetWebBrokerAPI handles GET /api/internal/v1/webbroker-apis/:apiId
func (h *GatewayInternalAPIHandler) GetWebBrokerAPI(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	apiID := r.PathValue("apiId")
	if apiID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API ID is required"))
		return
	}

	api, err := h.gatewayInternalService.GetActiveWebBrokerAPIDeploymentByGateway(apiID, orgID, gatewayID)
	if err != nil {
		clientIP := r.RemoteAddr
		if i := strings.LastIndex(clientIP, ":"); i != -1 {
			clientIP = clientIP[:i]
		}
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			h.slogger.Error("No active deployment found for WebBroker API", "clientIP", clientIP, "apiID", apiID, "orgID", orgID, "gatewayID", gatewayID, "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"No active deployment found for this WebBroker API on this gateway"))
			return
		}
		if errors.Is(err, constants.ErrWebBrokerAPINotFound) {
			h.slogger.Error("WebBroker API not found", "clientIP", clientIP, "apiID", apiID, "orgID", orgID, "gatewayID", gatewayID, "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"WebBroker API not found"))
			return
		}
		h.slogger.Error("Failed to get WebBroker API", "clientIP", clientIP, "apiID", apiID, "orgID", orgID, "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get WebBroker API"))
		return
	}

	// Create ZIP file from WebBroker API YAML file
	zipData, err := utils.CreateWebBrokerAPIYamlZip(api)
	if err != nil {
		h.slogger.Error("Failed to create ZIP file", "apiID", apiID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create WebBroker API package"))
		return
	}

	// Set headers for ZIP file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"webbroker-api-%s.zip\"", apiID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipData)
}

// ReceiveGatewayManifest handles POST /api/internal/v1/gateways/:gatewayId/manifest
// Called by the gateway controller to post back its installed custom policy manifest.
func (h *GatewayInternalAPIHandler) ReceiveGatewayManifest(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	var body struct {
		Version           string                       `json:"version"`
		FunctionalityType string                       `json:"functionalityType"`
		Policies          []service.GatewayPolicyInput `json:"policies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	if err := h.gatewayService.ReceiveGatewayManifest(orgID, gatewayID, body.Version, body.FunctionalityType, body.Policies); err != nil {
		if errors.Is(err, constants.ErrGatewayVersionMismatch) {
			h.slogger.Warn("Gateway manifest rejected: version mismatch", "gatewayID", gatewayID, "error", err)
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", err.Error()))
			return
		}
		if errors.Is(err, constants.ErrGatewayFunctionalityTypeMismatch) {
			h.slogger.Warn("Gateway manifest rejected: functionality type mismatch", "gatewayID", gatewayID, "error", err)
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", err.Error()))
			return
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", err.Error()))
			return
		}
		h.slogger.Error("Failed to store gateway manifest", "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to store gateway manifest"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetRestAPIAPIKeys handles GET /api/internal/v1/apis/api-keys
func (h *GatewayInternalAPIHandler) GetRestAPIAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	issuer := r.URL.Query().Get("issuer")
	keys, err := h.gatewayInternalService.GetAPIKeysByKind(gatewayID, orgID, constants.RestApi, issuer)
	if err != nil {
		h.slogger.Error("Failed to get API keys for REST APIs", "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get API keys"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
}

// GetLLMProviderAPIKeys handles GET /api/internal/v1/llm-providers/api-keys
func (h *GatewayInternalAPIHandler) GetLLMProviderAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	issuer := r.URL.Query().Get("issuer")
	keys, err := h.gatewayInternalService.GetAPIKeysByKind(gatewayID, orgID, constants.LLMProvider, issuer)
	if err != nil {
		h.slogger.Error("Failed to get API keys for LLM providers", "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get API keys"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
}

// GetLLMProxyAPIKeys handles GET /api/internal/v1/llm-proxies/api-keys
func (h *GatewayInternalAPIHandler) GetLLMProxyAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	issuer := r.URL.Query().Get("issuer")
	keys, err := h.gatewayInternalService.GetAPIKeysByKind(gatewayID, orgID, constants.LLMProxy, issuer)
	if err != nil {
		h.slogger.Error("Failed to get API keys for LLM proxies", "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get API keys"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
}

// GetWebSubAPIAPIKeys handles GET /api/internal/v1/websub-apis/api-keys
func (h *GatewayInternalAPIHandler) GetWebSubAPIAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	issuer := r.URL.Query().Get("issuer")
	keys, err := h.gatewayInternalService.GetAPIKeysByKind(gatewayID, orgID, constants.WebSubApi, issuer)
	if err != nil {
		h.slogger.Error("Failed to get API keys for WebSub APIs", "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get API keys"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
}

// GetWebBrokerAPIAPIKeys handles GET /api/internal/v1/webbroker-apis/api-keys
func (h *GatewayInternalAPIHandler) GetWebBrokerAPIAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	issuer := r.URL.Query().Get("issuer")
	keys, err := h.gatewayInternalService.GetAPIKeysByKind(gatewayID, orgID, constants.WebBrokerApi, issuer)
	if err != nil {
		h.slogger.Error("Failed to get API keys for WebBroker APIs", "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get API keys"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
}

// CheckArtifactsExist handles POST /api/internal/v1/artifacts/exists
// Returns the subset of provided artifact UUIDs that still exist on the platform.
// Used by the gateway during sync to avoid deleting artifacts that still exist
// but have no active deployment (e.g., after deployment deletion).
func (h *GatewayInternalAPIHandler) CheckArtifactsExist(w http.ResponseWriter, r *http.Request) {
	orgID, _, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	var req dto.ArtifactsExistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body: artifactIds is required and must be a non-empty array"))
		return
	}

	existingIDs, err := h.gatewayInternalService.CheckArtifactsExist(orgID, req.ArtifactIDs)
	if err != nil {
		h.slogger.Error("Failed to check artifact existence", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to check artifact existence"))
		return
	}

	// Build a set of existing IDs for O(1) lookup
	existingSet := make(map[string]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		existingSet[id] = struct{}{}
	}

	// Build response with true/false for every requested ID
	artifacts := make([]dto.ArtifactExistenceInfo, len(req.ArtifactIDs))
	for i, id := range req.ArtifactIDs {
		_, exists := existingSet[id]
		artifacts[i] = dto.ArtifactExistenceInfo{
			ArtifactID: id,
			Exists:     exists,
		}
	}

	httputil.WriteJSON(w, http.StatusOK, dto.ArtifactsExistResponse{
		Artifacts: artifacts,
	})
}

// GetWebSubAPIHmacSecrets handles GET /api/internal/v1/websub-apis/:apiId/secrets
// Returns decrypted plaintext HMAC secrets for the gateway-controller to load into its webhook secret store.
func (h *GatewayInternalAPIHandler) GetWebSubAPIHmacSecrets(w http.ResponseWriter, r *http.Request) {
	_, _, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	apiID := r.PathValue("apiId")
	if apiID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API ID is required"))
		return
	}

	if h.hmacSecretService == nil {
		h.slogger.Warn("HMAC secret service not configured", "apiID", apiID)
		httputil.WriteJSON(w, http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable", "HMAC secret management is not configured on this server"))
		return
	}

	secrets, err := h.hmacSecretService.ListByArtifactUUID(apiID)
	if err != nil {
		h.slogger.Error("Failed to list HMAC secrets for WebSub API", "apiID", apiID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get HMAC secrets"))
		return
	}

	items := make([]dto.GatewayHmacSecretInfo, 0, len(secrets))
	for _, s := range secrets {
		plaintext, err := h.hmacSecretService.DecryptSecret(s)
		if err != nil {
			h.slogger.Error("Failed to decrypt HMAC secret", "apiID", apiID, "secretName", s.Handle, "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to decrypt HMAC secret"))
			return
		}
		items = append(items, dto.GatewayHmacSecretInfo{Name: s.Handle, Secret: plaintext})
	}

	httputil.WriteJSON(w, http.StatusOK, dto.GatewayHmacSecretsResponse{ArtifactID: apiID, Secrets: items})
}

// GetGatewaySecrets handles GET /api/internal/v1/secrets
// Returns secret metadata for secrets referenced by this gateway's deployed artifacts.
// Supports ?updatedAfter=<RFC3339> for incremental sync.
// Supports ?includeValues=true for startup bulk fetch — decrypts all secrets server-side
// and returns plaintext values in a single response, avoiding N per-secret round trips.
func (h *GatewayInternalAPIHandler) GetGatewaySecrets(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	var updatedAfter *time.Time
	if s := r.URL.Query().Get("updatedAfter"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid 'updatedAfter' parameter. Expected RFC3339 format."))
			return
		}
		updatedAfter = &t
	}

	includeValues := r.URL.Query().Get("includeValues") == "true"

	secrets, err := h.gatewayInternalService.GetSecretsByGateway(orgID, gatewayID, updatedAfter)
	if err != nil {
		h.slogger.Error("Failed to list gateway secrets", "orgID", orgID, "gatewayID", gatewayID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to retrieve secrets"))
		return
	}

	items := make([]dto.SecretSyncItem, 0, len(secrets))
	for _, s := range secrets {
		item := dto.SecretSyncItem{
			Handle:      s.Handle,
			DisplayName: s.DisplayName,
			Type:        s.Type,
			Provider:    s.Provider,
			Status:      s.Status,
			Hash:        s.Hash,
			CreatedAt:   s.CreatedAt,
			UpdatedAt:   s.UpdatedAt,
		}
		if includeValues && s.Status == model.SecretStatusActive {
			plaintext, err := h.secretService.DecryptCiphertext(s.Ciphertext)
			if err != nil {
				h.slogger.Error("Failed to decrypt secret for bulk fetch", "orgID", orgID, "handle", s.Handle, "error", err)
				httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
					"Failed to decrypt secret"))
				return
			}
			item.Value = &plaintext
		}
		items = append(items, item)
	}
	httputil.WriteJSON(w, http.StatusOK, dto.SecretSyncListResponse{List: items, Count: len(items)})
}

// GetGatewaySecretValue handles GET /api/internal/v1/secrets/{handle}/value
// Returns the decrypted plaintext value of a secret. Called by the GW controller
// only when the secret's hash has changed, minimising decryption calls.
// Authenticated via gateway api-key — no JWT required.
func (h *GatewayInternalAPIHandler) GetGatewaySecretValue(w http.ResponseWriter, r *http.Request) {
	orgID, gatewayID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	handle := r.PathValue("handle")
	if handle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Secret handle is required"))
		return
	}

	// Only serve secrets that are referenced by artifacts deployed on this gateway.
	deployed, err := h.gatewayInternalService.IsSecretDeployedOnGateway(orgID, gatewayID, handle)
	if err != nil {
		h.slogger.Error("Failed to check secret deployment scope", "orgID", orgID, "gatewayID", gatewayID, "handle", handle, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to verify secret access"))
		return
	}
	if !deployed {
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Secret not found"))
		return
	}

	plaintext, err := h.secretService.Decrypt(orgID, handle)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Secret not found"))
			return
		}
		h.slogger.Error("Failed to decrypt secret for gateway", "orgID", orgID, "handle", handle, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to decrypt secret"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"value": plaintext})
}

func (h *GatewayInternalAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/internal/v1/apis/api-keys", h.GetRestAPIAPIKeys)
	mux.HandleFunc("GET /api/internal/v1/apis/{apiId}", h.GetAPI)
	mux.HandleFunc("GET /api/internal/v1/apis/{apiId}/subscriptions", h.GetSubscriptions)
	mux.HandleFunc("GET /api/internal/v1/subscription-plans", h.GetSubscriptionPlans)
	mux.HandleFunc("GET /api/internal/v1/secrets", h.GetGatewaySecrets)
	mux.HandleFunc("GET /api/internal/v1/secrets/{handle}/value", h.GetGatewaySecretValue)
	mux.HandleFunc("GET /api/internal/v1/llm-providers/api-keys", h.GetLLMProviderAPIKeys)
	mux.HandleFunc("GET /api/internal/v1/llm-providers/{providerId}", h.GetLLMProvider)
	mux.HandleFunc("GET /api/internal/v1/llm-proxies/api-keys", h.GetLLMProxyAPIKeys)
	mux.HandleFunc("GET /api/internal/v1/llm-proxies/{proxyId}", h.GetLLMProxy)
	mux.HandleFunc("GET /api/internal/v1/deployments", h.GetGatewayDeployments)
	mux.HandleFunc("POST /api/internal/v1/deployments/fetch-batch", h.BatchFetchDeployments)
	mux.HandleFunc("GET /api/internal/v1/mcp-proxies/{proxyId}", h.GetMCPProxy)
	mux.HandleFunc("GET /api/internal/v1/websub-apis/api-keys", h.GetWebSubAPIAPIKeys)
	mux.HandleFunc("GET /api/internal/v1/websub-apis/{apiId}", h.GetWebSubAPI)
	mux.HandleFunc("GET /api/internal/v1/websub-apis/{apiId}/secrets", h.GetWebSubAPIHmacSecrets)
	mux.HandleFunc("GET /api/internal/v1/webbroker-apis/api-keys", h.GetWebBrokerAPIAPIKeys)
	mux.HandleFunc("GET /api/internal/v1/webbroker-apis/{apiId}", h.GetWebBrokerAPI)
	mux.HandleFunc("POST /api/internal/v1/gateways/{gatewayId}/manifest", h.ReceiveGatewayManifest)
	mux.HandleFunc("POST /api/internal/v1/artifacts/exists", h.CheckArtifactsExist)
	mux.HandleFunc("POST /api/internal/v1/artifacts/import-gateway-artifacts", h.ImportGatewayArtifacts)
}

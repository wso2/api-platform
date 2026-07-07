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
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

// hmacSecretDecrypter is the minimal surface needed from a WebSub HMAC secret
// service. Defined as a local interface so the handler does not import the
// concrete implementation (which lives in the event-gateway plugin).
type hmacSecretDecrypter interface {
	ListByArtifactUUID(artifactUUID string) ([]*model.WebSubAPIHmacSecret, error)
	DecryptSecret(s *model.WebSubAPIHmacSecret) (string, error)
}

type GatewayInternalAPIHandler struct {
	gatewayService         *service.GatewayService
	gatewayInternalService *service.GatewayInternalAPIService
	artifactImportService  *service.ArtifactImportService
	hmacSecretService      hmacSecretDecrypter // nil in OSS builds
	secretService          *service.SecretService
	slogger                *slog.Logger
}

func NewGatewayInternalAPIHandler(gatewayService *service.GatewayService,
	gatewayInternalService *service.GatewayInternalAPIService,
	artifactImportService *service.ArtifactImportService,
	secretService *service.SecretService,
	slogger *slog.Logger) *GatewayInternalAPIHandler {
	return &GatewayInternalAPIHandler{
		gatewayService:         gatewayService,
		gatewayInternalService: gatewayInternalService,
		secretService:          secretService,
		artifactImportService:  artifactImportService,
		slogger:                slogger,
	}
}

// SetHmacSecretService wires in the HMAC secret service. Called by the server
// after the event-gateway plugin is initialized (experimental builds only).
func (h *GatewayInternalAPIHandler) SetHmacSecretService(svc hmacSecretDecrypter) {
	h.hmacSecretService = svc
}

// authenticateGateway validates the API key and returns the authenticated gateway.
func (h *GatewayInternalAPIHandler) authenticateGateway(apiKey string) (*model.Gateway, error) {
	if apiKey == "" {
		return nil, constants.ErrMissingAPIKey
	}
	return h.gatewayService.VerifyToken(apiKey)
}

// authenticateRequest extracts the API key from headers and authenticates the gateway. Per the
// unified auth-failure rule, a missing/invalid key returns the identical generic 401; the
// specific reason travels internally via WithLogMessage only.
func (h *GatewayInternalAPIHandler) authenticateRequest(r *http.Request) (orgID, gatewayID string, err error) {
	clientIP := r.RemoteAddr
	if i := strings.LastIndex(clientIP, ":"); i != -1 {
		clientIP = clientIP[:i]
	}
	apiKey := r.Header.Get("api-key")

	gateway, authErr := h.authenticateGateway(apiKey)
	if authErr != nil {
		if errors.Is(authErr, constants.ErrMissingAPIKey) {
			return "", "", apperror.Unauthorized.New().
				WithLogMessage(fmt.Sprintf("unauthorized access attempt - missing API key, clientIP=%s", clientIP))
		}
		if errors.Is(authErr, constants.ErrInvalidAPIToken) {
			return "", "", apperror.Unauthorized.Wrap(authErr).
				WithLogMessage(fmt.Sprintf("authentication failed - invalid API key, clientIP=%s", clientIP))
		}
		return "", "", apperror.Internal.Wrap(authErr).
			WithLogMessage(fmt.Sprintf("authentication failed, clientIP=%s", clientIP))
	}
	return gateway.OrganizationID, gateway.ID, nil
}

// GetAPI handles GET /api/internal/v1/apis/:apiId
func (h *GatewayInternalAPIHandler) GetAPI(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	apiID := r.PathValue("apiId")
	if apiID == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	api, err := h.gatewayInternalService.GetActiveDeploymentByGateway(apiID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			return apperror.DeploymentNotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("no active deployment found for API %s on gateway %s", apiID, gatewayID))
		}
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get API %s", apiID))
	}

	// Create ZIP file from API YAML file
	zipData, err := utils.CreateAPIYamlZip(api)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to create ZIP file for API %s", apiID))
	}

	// Set headers for ZIP file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"api-%s.zip\"", apiID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipData)
	return nil
}

// ImportGatewayArtifacts handles POST /api/internal/v1/artifacts/import-gateway-artifacts.
// It is the generic bulk DP->CP push endpoint. The request is multipart/form-data with an
// "artifacts" zip part (containing the artifacts.json file — a JSON array of
// ImportGatewayArtifactRequest) and an advisory "total" field. The control plane creates or
// updates each artifact (read-only, origin "gateway_api") in dependency order and is
// continue-on-error: a failure on one artifact is recorded against its dpid and does not
// abort the rest. The response maps each artifact's dpid to its result, with total/success/
// failed counts. Only a malformed request (bad multipart/zip) returns a non-200.
func (h *GatewayInternalAPIHandler) ImportGatewayArtifacts(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	reqs, err := utils.ParseGatewayArtifactsRequest(r)
	if err != nil {
		clientIP := r.RemoteAddr
		if i := strings.LastIndex(clientIP, ":"); i != -1 {
			clientIP = clientIP[:i]
		}
		return apperror.ValidationFailed.Wrap(err, err.Error()).
			WithLogMessage(fmt.Sprintf("invalid import-gateway-artifacts request, clientIP=%s", clientIP))
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
	return nil
}

// GetLLMProvider handles GET /api/internal/v1/llm-providers/:providerId
func (h *GatewayInternalAPIHandler) GetLLMProvider(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	providerID := r.PathValue("providerId")
	if providerID == "" {
		return apperror.ValidationFailed.New("Provider ID is required")
	}

	provider, err := h.gatewayInternalService.GetActiveLLMProviderDeploymentByGateway(providerID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			return apperror.DeploymentNotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("no active deployment found for LLM provider %s on gateway %s", providerID, gatewayID))
		}
		if errors.Is(err, constants.ErrLLMProviderNotFound) {
			return apperror.LLMProviderNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get LLM provider %s", providerID))
	}

	// Create ZIP file from LLM provider YAML file
	zipData, err := utils.CreateLLMProviderYamlZip(provider)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to create ZIP file for LLM provider %s", providerID))
	}

	// Set headers for ZIP file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"llm-provider-%s.zip\"", providerID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipData)
	return nil
}

// GetLLMProxy handles GET /api/internal/v1/llm-proxies/:proxyId
func (h *GatewayInternalAPIHandler) GetLLMProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	proxyID := r.PathValue("proxyId")
	if proxyID == "" {
		return apperror.ValidationFailed.New("Proxy ID is required")
	}

	proxy, err := h.gatewayInternalService.GetActiveLLMProxyDeploymentByGateway(proxyID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			return apperror.DeploymentNotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("no active deployment found for LLM proxy %s on gateway %s", proxyID, gatewayID))
		}
		if errors.Is(err, constants.ErrLLMProxyNotFound) {
			return apperror.LLMProxyNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get LLM proxy %s", proxyID))
	}

	// Create ZIP file from LLM proxy YAML file
	zipData, err := utils.CreateLLMProxyYamlZip(proxy)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to create ZIP file for LLM proxy %s", proxyID))
	}

	// Set headers for ZIP file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"llm-proxy-%s.zip\"", proxyID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipData)
	return nil
}

// GetGatewayDeployments handles GET /api/internal/v1/deployments
// Returns the list of deployments that should be active on a gateway for startup sync
func (h *GatewayInternalAPIHandler) GetGatewayDeployments(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	// Parse optional "since" query parameter for incremental sync
	var since *time.Time
	sinceStr := r.URL.Query().Get("since")
	if sinceStr != "" {
		parsedTime, parseErr := time.Parse(time.RFC3339, sinceStr)
		if parseErr != nil {
			return apperror.ValidationFailed.Wrap(parseErr,
				"Invalid 'since' parameter. Expected ISO 8601 format (e.g., 2026-03-04T10:00:00Z)")
		}
		since = &parsedTime
	}

	deployments, err := h.gatewayInternalService.GetDeploymentsByGateway(orgID, gatewayID, since)
	if err != nil {
		if errors.Is(err, constants.ErrGatewayNotFound) {
			return apperror.GatewayNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get gateway deployments for gateway %s", gatewayID))
	}

	httputil.WriteJSON(w, http.StatusOK, deployments)
	return nil
}

// BatchFetchDeployments handles POST /api/internal/v1/deployments/fetch-batch
// Fetches multiple deployment artifacts in a single request for gateway startup sync
func (h *GatewayInternalAPIHandler) BatchFetchDeployments(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	// Enforce Accept header - only application/x-tar+gzip is supported
	if accept := r.Header.Get("Accept"); accept != "application/x-tar+gzip" {
		return apperror.NotAcceptable.New()
	}

	// Parse request body
	var req dto.DeploymentsBatchFetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body: "+err.Error())
	}

	if len(req.DeploymentIDs) == 0 {
		return apperror.ValidationFailed.New("At least one deployment ID is required")
	}

	// Fetch deployment content
	contentMap, err := h.gatewayInternalService.GetDeploymentContentBatch(orgID, gatewayID, req.DeploymentIDs)
	if err != nil {
		if errors.Is(err, constants.ErrGatewayNotFound) {
			return apperror.GatewayNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get deployment content batch for gateway %s", gatewayID))
	}

	// Create TAR.GZ archive from deployment content
	tarGzData, err := utils.CreateBatchDeploymentTarGz(contentMap)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to create batch TAR.GZ archive for gateway %s", gatewayID))
	}

	// Set headers for TAR.GZ download
	w.Header().Set("Content-Type", "application/x-tar+gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"deployments-batch-%s.tar.gz\"", gatewayID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(tarGzData)))

	// Return TAR.GZ archive
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(tarGzData)
	return nil
}

// GetSubscriptions handles GET /api/internal/v1/apis/:apiId/subscriptions
func (h *GatewayInternalAPIHandler) GetSubscriptions(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	apiID := r.PathValue("apiId")
	if apiID == "" {
		return apperror.ValidationFailed.New("API ID is required").
			WithLogMessage(fmt.Sprintf("API ID is required for subscriptions request, orgID=%s", orgID))
	}

	if err := h.gatewayInternalService.IsAPIDeployedOnGateway(apiID, gatewayID, orgID); err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("API not found when listing subscriptions, apiId=%s, orgID=%s, gatewayID=%s", apiID, orgID, gatewayID))
		}
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			return apperror.Forbidden.New().
				WithLogMessage(fmt.Sprintf("subscription list denied - API has no active deployment status on gateway, apiId=%s, orgID=%s, gatewayID=%s", apiID, orgID, gatewayID))
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to verify API deployment for subscriptions, apiId=%s, orgID=%s, gatewayID=%s", apiID, orgID, gatewayID))
	}

	subs, err := h.gatewayInternalService.ListSubscriptionsForAPI(apiID, orgID)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.RESTAPINotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("API not found when listing subscriptions, apiId=%s, orgID=%s", apiID, orgID))
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to list subscriptions for API, apiId=%s, orgID=%s", apiID, orgID))
	}

	httputil.WriteJSON(w, http.StatusOK, subs)
	return nil
}

// GetSubscriptionPlans handles GET /api/internal/v1/subscription-plans
func (h *GatewayInternalAPIHandler) GetSubscriptionPlans(w http.ResponseWriter, r *http.Request) error {
	orgID, _, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	plans, err := h.gatewayInternalService.ListSubscriptionPlansForOrg(orgID)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to list subscription plans, orgID=%s", orgID))
	}

	httputil.WriteJSON(w, http.StatusOK, plans)
	return nil
}

// GetMCPProxy handles GET /api/internal/v1/mcp-proxies/:proxyId
func (h *GatewayInternalAPIHandler) GetMCPProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}
	proxyID := r.PathValue("proxyId")
	if proxyID == "" {
		return apperror.ValidationFailed.New("Proxy ID is required")
	}

	proxy, err := h.gatewayInternalService.GetActiveMCPProxyDeploymentByGateway(proxyID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			return apperror.DeploymentNotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("no active deployment found for MCP proxy %s on gateway %s", proxyID, gatewayID))
		}
		if errors.Is(err, constants.ErrMCPProxyNotFound) {
			return apperror.MCPProxyNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get MCP proxy %s", proxyID))
	}

	// Create ZIP file from MCP proxy YAML file
	zipData, err := utils.CreateMCPProxyYamlZip(proxy)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to create ZIP file for MCP proxy %s", proxyID))
	}

	// Set headers for ZIP file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"mcp-proxy-%s.zip\"", proxyID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipData)
	return nil
}

// GetWebSubAPI handles GET /api/internal/v1/websub-apis/:apiId
func (h *GatewayInternalAPIHandler) GetWebSubAPI(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	apiID := r.PathValue("apiId")
	if apiID == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	api, err := h.gatewayInternalService.GetActiveWebSubAPIDeploymentByGateway(apiID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			return apperror.DeploymentNotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("no active deployment found for WebSub API %s on gateway %s", apiID, gatewayID))
		}
		if errors.Is(err, constants.ErrWebSubAPINotFound) {
			return apperror.ArtifactNotFound.Wrap(err).WithLogMessage(fmt.Sprintf("WebSub API not found, apiID=%s", apiID))
		}
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get WebSub API %s", apiID))
	}

	// Create ZIP file from WebSub API YAML file
	zipData, err := utils.CreateWebSubAPIYamlZip(api)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to create ZIP file for WebSub API %s", apiID))
	}

	// Set headers for ZIP file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"websub-api-%s.zip\"", apiID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipData)
	return nil
}

// GetWebBrokerAPI handles GET /api/internal/v1/webbroker-apis/:apiId
func (h *GatewayInternalAPIHandler) GetWebBrokerAPI(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	apiID := r.PathValue("apiId")
	if apiID == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	api, err := h.gatewayInternalService.GetActiveWebBrokerAPIDeploymentByGateway(apiID, orgID, gatewayID)
	if err != nil {
		if errors.Is(err, constants.ErrDeploymentNotActive) {
			return apperror.DeploymentNotFound.Wrap(err).
				WithLogMessage(fmt.Sprintf("no active deployment found for WebBroker API %s on gateway %s", apiID, gatewayID))
		}
		if errors.Is(err, constants.ErrWebBrokerAPINotFound) {
			return apperror.ArtifactNotFound.Wrap(err).WithLogMessage(fmt.Sprintf("WebBroker API not found, apiID=%s", apiID))
		}
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get WebBroker API %s", apiID))
	}

	// Create ZIP file from WebBroker API YAML file
	zipData, err := utils.CreateWebBrokerAPIYamlZip(api)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to create ZIP file for WebBroker API %s", apiID))
	}

	// Set headers for ZIP file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"webbroker-api-%s.zip\"", apiID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	// Return ZIP file
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipData)
	return nil
}

// ReceiveGatewayManifest handles POST /api/internal/v1/gateways/:gatewayId/manifest
// Called by the gateway controller to post back its installed custom policy manifest.
func (h *GatewayInternalAPIHandler) ReceiveGatewayManifest(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	var body struct {
		Version           string                       `json:"version"`
		FunctionalityType string                       `json:"functionalityType"`
		Policies          []service.GatewayPolicyInput `json:"policies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.ValidationFailed.Wrap(err, err.Error())
	}

	if err := h.gatewayService.ReceiveGatewayManifest(orgID, gatewayID, body.Version, body.FunctionalityType, body.Policies); err != nil {
		if errors.Is(err, constants.ErrGatewayVersionMismatch) {
			return apperror.Conflict.Wrap(err).WithLogMessage(fmt.Sprintf("gateway manifest rejected: version mismatch, gatewayID=%s", gatewayID))
		}
		if errors.Is(err, constants.ErrGatewayFunctionalityTypeMismatch) {
			return apperror.Conflict.Wrap(err).WithLogMessage(fmt.Sprintf("gateway manifest rejected: functionality type mismatch, gatewayID=%s", gatewayID))
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			return apperror.GatewayNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to store gateway manifest, gatewayID=%s", gatewayID))
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GetRestAPIAPIKeys handles GET /api/internal/v1/apis/api-keys
func (h *GatewayInternalAPIHandler) GetRestAPIAPIKeys(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}
	issuer := r.URL.Query().Get("issuer")
	keys, err := h.gatewayInternalService.GetAPIKeysByKind(gatewayID, orgID, constants.RestApi, issuer)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get API keys for REST APIs, gatewayID=%s", gatewayID))
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
	return nil
}

// GetLLMProviderAPIKeys handles GET /api/internal/v1/llm-providers/api-keys
func (h *GatewayInternalAPIHandler) GetLLMProviderAPIKeys(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}
	issuer := r.URL.Query().Get("issuer")
	keys, err := h.gatewayInternalService.GetAPIKeysByKind(gatewayID, orgID, constants.LLMProvider, issuer)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get API keys for LLM providers, gatewayID=%s", gatewayID))
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
	return nil
}

// GetLLMProxyAPIKeys handles GET /api/internal/v1/llm-proxies/api-keys
func (h *GatewayInternalAPIHandler) GetLLMProxyAPIKeys(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}
	issuer := r.URL.Query().Get("issuer")
	keys, err := h.gatewayInternalService.GetAPIKeysByKind(gatewayID, orgID, constants.LLMProxy, issuer)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get API keys for LLM proxies, gatewayID=%s", gatewayID))
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
	return nil
}

// GetWebSubAPIAPIKeys handles GET /api/internal/v1/websub-apis/api-keys
func (h *GatewayInternalAPIHandler) GetWebSubAPIAPIKeys(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}
	issuer := r.URL.Query().Get("issuer")
	keys, err := h.gatewayInternalService.GetAPIKeysByKind(gatewayID, orgID, constants.WebSubApi, issuer)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get API keys for WebSub APIs, gatewayID=%s", gatewayID))
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
	return nil
}

// GetWebBrokerAPIAPIKeys handles GET /api/internal/v1/webbroker-apis/api-keys
func (h *GatewayInternalAPIHandler) GetWebBrokerAPIAPIKeys(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}
	issuer := r.URL.Query().Get("issuer")
	keys, err := h.gatewayInternalService.GetAPIKeysByKind(gatewayID, orgID, constants.WebBrokerApi, issuer)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get API keys for WebBroker APIs, gatewayID=%s", gatewayID))
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
	return nil
}

// CheckArtifactsExist handles POST /api/internal/v1/artifacts/exists
// Returns the subset of provided artifact UUIDs that still exist on the platform.
// Used by the gateway during sync to avoid deleting artifacts that still exist
// but have no active deployment (e.g., after deployment deletion).
func (h *GatewayInternalAPIHandler) CheckArtifactsExist(w http.ResponseWriter, r *http.Request) error {
	orgID, _, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	var req dto.ArtifactsExistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body: artifactIds is required and must be a non-empty array")
	}

	existingIDs, err := h.gatewayInternalService.CheckArtifactsExist(orgID, req.ArtifactIDs)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage("failed to check artifact existence")
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
	return nil
}

// GetWebSubAPIHmacSecrets handles GET /api/internal/v1/websub-apis/:apiId/secrets
// Returns decrypted plaintext HMAC secrets for the gateway-controller to load into its webhook secret store.
func (h *GatewayInternalAPIHandler) GetWebSubAPIHmacSecrets(w http.ResponseWriter, r *http.Request) error {
	_, _, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	apiID := r.PathValue("apiId")
	if apiID == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}

	if h.hmacSecretService == nil {
		return apperror.ServiceUnavailable.New().
			WithLogMessage(fmt.Sprintf("HMAC secret service not configured, apiID=%s", apiID))
	}

	secrets, err := h.hmacSecretService.ListByArtifactUUID(apiID)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to list HMAC secrets for WebSub API %s", apiID))
	}

	items := make([]dto.GatewayHmacSecretInfo, 0, len(secrets))
	for _, s := range secrets {
		plaintext, err := h.hmacSecretService.DecryptSecret(s)
		if err != nil {
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to decrypt HMAC secret, apiID=%s, secretName=%s", apiID, s.Handle))
		}
		items = append(items, dto.GatewayHmacSecretInfo{Name: s.Handle, Secret: plaintext})
	}

	httputil.WriteJSON(w, http.StatusOK, dto.GatewayHmacSecretsResponse{ArtifactID: apiID, Secrets: items})
	return nil
}

// GetGatewaySecrets handles GET /api/internal/v1/secrets
// Returns secret metadata for secrets referenced by this gateway's deployed artifacts.
// Supports ?updatedAfter=<RFC3339> for incremental sync.
// Supports ?includeValues=true for startup bulk fetch — decrypts all secrets server-side
// and returns plaintext values in a single response, avoiding N per-secret round trips.
func (h *GatewayInternalAPIHandler) GetGatewaySecrets(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	var updatedAfter *time.Time
	if s := r.URL.Query().Get("updatedAfter"); s != "" {
		t, parseErr := time.Parse(time.RFC3339, s)
		if parseErr != nil {
			return apperror.ValidationFailed.Wrap(parseErr, "Invalid 'updatedAfter' parameter. Expected RFC3339 format.")
		}
		updatedAfter = &t
	}

	includeValues := r.URL.Query().Get("includeValues") == "true"

	secrets, err := h.gatewayInternalService.GetSecretsByGateway(orgID, gatewayID, updatedAfter)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to list gateway secrets, orgID=%s, gatewayID=%s", orgID, gatewayID))
	}

	items := make([]dto.SecretSyncItem, 0, len(secrets))
	for _, s := range secrets {
		item := dto.SecretSyncItem{
			ID:          s.UUID,
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
				return apperror.Internal.Wrap(err).
					WithLogMessage(fmt.Sprintf("failed to decrypt secret for bulk fetch, orgID=%s, handle=%s", orgID, s.Handle))
			}
			item.Value = &plaintext
		}
		items = append(items, item)
	}
	httputil.WriteJSON(w, http.StatusOK, dto.SecretSyncListResponse{List: items, Count: len(items)})
	return nil
}

// GetGatewaySecretValue handles GET /api/internal/v1/secrets/{handle}/value
// Returns the decrypted plaintext value of a secret. Called by the GW controller
// only when the secret's hash has changed, minimising decryption calls.
// Authenticated via gateway api-key — no JWT required.
func (h *GatewayInternalAPIHandler) GetGatewaySecretValue(w http.ResponseWriter, r *http.Request) error {
	orgID, gatewayID, err := h.authenticateRequest(r)
	if err != nil {
		return err
	}

	handle := r.PathValue("handle")
	if handle == "" {
		return apperror.ValidationFailed.New("Secret handle is required")
	}

	// Only serve secrets that are referenced by artifacts deployed on this gateway.
	deployed, err := h.gatewayInternalService.IsSecretDeployedOnGateway(orgID, gatewayID, handle)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to check secret deployment scope, orgID=%s, gatewayID=%s, handle=%s", orgID, gatewayID, handle))
	}
	if !deployed {
		return apperror.SecretNotFound.New()
	}

	plaintext, err := h.secretService.Decrypt(orgID, handle)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			return apperror.SecretNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to decrypt secret for gateway, orgID=%s, handle=%s", orgID, handle))
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"value": plaintext})
	return nil
}

func (h *GatewayInternalAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/internal/v1/apis/api-keys", middleware.MapErrors(h.slogger, h.GetRestAPIAPIKeys))
	mux.HandleFunc("GET /api/internal/v1/apis/{apiId}", middleware.MapErrors(h.slogger, h.GetAPI))
	mux.HandleFunc("GET /api/internal/v1/apis/{apiId}/subscriptions", middleware.MapErrors(h.slogger, h.GetSubscriptions))
	mux.HandleFunc("GET /api/internal/v1/subscription-plans", middleware.MapErrors(h.slogger, h.GetSubscriptionPlans))
	mux.HandleFunc("GET /api/internal/v1/secrets", middleware.MapErrors(h.slogger, h.GetGatewaySecrets))
	mux.HandleFunc("GET /api/internal/v1/secrets/{handle}/value", middleware.MapErrors(h.slogger, h.GetGatewaySecretValue))
	mux.HandleFunc("GET /api/internal/v1/llm-providers/api-keys", middleware.MapErrors(h.slogger, h.GetLLMProviderAPIKeys))
	mux.HandleFunc("GET /api/internal/v1/llm-providers/{providerId}", middleware.MapErrors(h.slogger, h.GetLLMProvider))
	mux.HandleFunc("GET /api/internal/v1/llm-proxies/api-keys", middleware.MapErrors(h.slogger, h.GetLLMProxyAPIKeys))
	mux.HandleFunc("GET /api/internal/v1/llm-proxies/{proxyId}", middleware.MapErrors(h.slogger, h.GetLLMProxy))
	mux.HandleFunc("GET /api/internal/v1/deployments", middleware.MapErrors(h.slogger, h.GetGatewayDeployments))
	mux.HandleFunc("POST /api/internal/v1/deployments/fetch-batch", middleware.MapErrors(h.slogger, h.BatchFetchDeployments))
	mux.HandleFunc("GET /api/internal/v1/mcp-proxies/{proxyId}", middleware.MapErrors(h.slogger, h.GetMCPProxy))
	mux.HandleFunc("GET /api/internal/v1/websub-apis/api-keys", middleware.MapErrors(h.slogger, h.GetWebSubAPIAPIKeys))
	mux.HandleFunc("GET /api/internal/v1/websub-apis/{apiId}", middleware.MapErrors(h.slogger, h.GetWebSubAPI))
	mux.HandleFunc("GET /api/internal/v1/websub-apis/{apiId}/secrets", middleware.MapErrors(h.slogger, h.GetWebSubAPIHmacSecrets))
	mux.HandleFunc("GET /api/internal/v1/webbroker-apis/api-keys", middleware.MapErrors(h.slogger, h.GetWebBrokerAPIAPIKeys))
	mux.HandleFunc("GET /api/internal/v1/webbroker-apis/{apiId}", middleware.MapErrors(h.slogger, h.GetWebBrokerAPI))
	mux.HandleFunc("POST /api/internal/v1/gateways/{gatewayId}/manifest", middleware.MapErrors(h.slogger, h.ReceiveGatewayManifest))
	mux.HandleFunc("POST /api/internal/v1/artifacts/exists", middleware.MapErrors(h.slogger, h.CheckArtifactsExist))
	mux.HandleFunc("POST /api/internal/v1/artifacts/import-gateway-artifacts", middleware.MapErrors(h.slogger, h.ImportGatewayArtifacts))
}

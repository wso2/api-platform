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
	"regexp"
	"strings"

	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

// gatewayVersionPattern accepts two shapes for the registration version field:
//   - `major.minor`     (e.g. "1.0", "1.1") — LTS-style stable releases
//   - `YYYY.MM.DD`      (e.g. "2026.05.13") — CalVer stable releases; the
//     leading segment must be 4 digits to distinguish a calendar year from a
//     `major.minor` value.
var gatewayVersionPattern = regexp.MustCompile(`^([0-9]{1,6}\.[0-9]{1,6}|[0-9]{4}\.[0-9]{1,2}\.[0-9]{1,2})$`)

// GatewayHandler handles HTTP requests for gateway operations
type GatewayHandler struct {
	gatewayService *service.GatewayService
	slogger        *slog.Logger
}

// NewGatewayHandler creates a new gateway handler
func NewGatewayHandler(gatewayService *service.GatewayService, slogger *slog.Logger) *GatewayHandler {
	return &GatewayHandler{
		gatewayService: gatewayService,
		slogger:        slogger,
	}
}

// manifestSyncResponse is the response body for manifest-sync endpoints
type manifestSyncResponse struct {
	Policies json.RawMessage `json:"policies,omitempty"`
}

// CreateGateway handles POST /api/v0.9/gateways
func (h *GatewayHandler) CreateGateway(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var req api.CreateGatewayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Convert functionality type to string
	functionalityType := string(req.FunctionalityType)

	// Extract values from pointers
	var description string
	if req.Description != nil {
		description = *req.Description
	}

	var isCritical bool
	if req.IsCritical != nil {
		isCritical = *req.IsCritical
	}

	var properties map[string]interface{}
	if req.Properties != nil {
		properties = *req.Properties
	}

	// TODO: make `version` required when registering a gateway. For now it is optional and defaults to "1.0".
	var version string
	if req.Version != nil {
		version = strings.TrimSpace(*req.Version)
	}
	if version != "" && !gatewayVersionPattern.MatchString(version) {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"version must be in 'major.minor' format (e.g. '1.0') or CalVer 'YYYY.MM.DD' format (e.g. '2026.05.13')"))
		return
	}

	createdBy, _ := middleware.GetUserIDFromRequest(r)
	gateway, err := h.gatewayService.RegisterGateway(orgId, req.Id, req.DisplayName, description, req.Endpoints,
		isCritical, functionalityType, version, createdBy, properties)
	if err != nil {
		errMsg := err.Error()

		// Check for specific error types
		if strings.Contains(errMsg, "organization not found") {
			h.slogger.Error("Organization not found during gateway creation", "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		if strings.Contains(errMsg, "already exists") {
			h.slogger.Error("Gateway already exists", "error", err)
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", errMsg))
			return
		}

		if strings.Contains(errMsg, "required") || strings.Contains(errMsg, "invalid") ||
			strings.Contains(errMsg, "must") || strings.Contains(errMsg, "cannot") {
			h.slogger.Error("Invalid gateway creation request", "error", err)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
			return
		}

		// Internal server error
		h.slogger.Error("Failed to register gateway", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to register gateway"))
		return
	}

	// Return 201 Created with response
	httputil.WriteJSON(w, http.StatusCreated, gateway)
}

// ListGateways handles GET /api/v0.9/gateways with constitution-compliant response
func (h *GatewayHandler) ListGateways(w http.ResponseWriter, r *http.Request) {
	organizationID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	gateways, err := h.gatewayService.ListGateways(&organizationID)
	if err != nil {
		h.slogger.Error("Failed to list gateways", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to list gateways"))
		return
	}

	// Return 200 OK with constitution-compliant envelope structure
	httputil.WriteJSON(w, http.StatusOK, gateways)
}

// GetGateway handles GET /api/v0.9/gateways/:gatewayId
func (h *GatewayHandler) GetGateway(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract UUID path parameter
	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway ID is required"))
		return
	}

	gateway, err := h.gatewayService.GetGateway(gatewayId, orgId)
	if err != nil {
		errMsg := err.Error()

		// Check for specific error types
		if strings.Contains(errMsg, "not found") {
			h.slogger.Error("Gateway not found", "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		if strings.Contains(errMsg, "invalid UUID") {
			h.slogger.Error("Invalid gateway UUID", "error", err)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
			return
		}

		// Internal server error
		h.slogger.Error("Failed to retrieve gateway", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to retrieve gateway"))
		return
	}

	// Return 200 OK with gateway details
	httputil.WriteJSON(w, http.StatusOK, gateway)
}

// GetGatewayStatus retrieves gateway status, optionally filtered by gatewayId query param.
func (h *GatewayHandler) GetGatewayStatus(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	gatewayId := r.URL.Query().Get("gatewayId")
	var gatewayIdPtr *string
	if gatewayId != "" {
		gatewayIdPtr = &gatewayId
	}

	status, err := h.gatewayService.GetGatewayStatus(orgId, gatewayIdPtr)
	if err != nil {
		if strings.Contains(err.Error(), "gateway not found") {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get gateway status"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, status)
}

// UpdateGateway handles PUT /api/v0.9/gateways/:gatewayId
func (h *GatewayHandler) UpdateGateway(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway ID is required"))
		return
	}

	var req api.GatewayResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	if req.Id != nil && *req.Id != gatewayId {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway id is immutable and cannot be changed"))
		return
	}

	if req.DisplayName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"displayName is required"))
		return
	}

	updatedBy, _ := middleware.GetUserIDFromRequest(r)
	response, err := h.gatewayService.UpdateGateway(gatewayId, orgId, updatedBy, &req)
	if err != nil {
		if errors.Is(err, constants.ErrGatewayNotFound) {
			h.slogger.Error("Gateway not found during update", "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		h.slogger.Error("Failed to update gateway", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to update gateway"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// DeleteGateway handles DELETE /api/v0.9/gateways/:gatewayId
func (h *GatewayHandler) DeleteGateway(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract UUID path parameter
	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway ID is required"))
		return
	}

	deletedBy, _ := middleware.GetUserIDFromRequest(r)
	err := h.gatewayService.DeleteGateway(gatewayId, orgId, deletedBy)
	if err != nil {
		// Check for specific error types
		if errors.Is(err, constants.ErrGatewayNotFound) {
			h.slogger.Error("Gateway not found during deletion", "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"The specified resource does not exist"))
			return
		}
		if errors.Is(err, constants.ErrGatewayHasAssociatedAPIs) {
			h.slogger.Error("Gateway has associated APIs during deletion", "error", err)
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"The gateway has associated APIs. Please remove all API associations before deleting the gateway"))
			return
		}

		if strings.Contains(err.Error(), "invalid UUID") {
			h.slogger.Error("Invalid UUID during gateway deletion", "error", err)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid gateway ID format"))
			return
		}

		// Internal server error
		h.slogger.Error("Failed to delete gateway", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"The server encountered an internal error. Please contact administrator."))
		return
	}

	// Return 204 No Content on successful deletion
	w.WriteHeader(http.StatusNoContent)
}

// ListTokens handles GET /api/v0.9/gateways/:gatewayId/tokens
func (h *GatewayHandler) ListTokens(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway ID is required"))
		return
	}

	tokens, err := h.gatewayService.ListTokens(gatewayId, orgId)
	if err != nil {
		errMsg := err.Error()

		if strings.Contains(errMsg, "gateway not found") {
			h.slogger.Error("Gateway not found during token listing", "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		h.slogger.Error("Failed to list tokens", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to list tokens"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, tokens)
}

// RotateToken handles POST /api/v0.9/gateways/:gatewayId/tokens
func (h *GatewayHandler) RotateToken(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	// Extract ID path parameter
	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway ID is required"))
		return
	}

	createdBy, _ := middleware.GetUserIDFromRequest(r)
	response, err := h.gatewayService.RotateToken(gatewayId, orgId, createdBy)
	if err != nil {
		errMsg := err.Error()

		// Check for specific error types
		if strings.Contains(errMsg, "gateway not found") {
			h.slogger.Error("Gateway not found during token rotation", "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		if strings.Contains(errMsg, "maximum") || strings.Contains(errMsg, "Revoke") {
			h.slogger.Error("Token rotation request validation failed", "error", err)
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
			return
		}

		// Internal server error
		h.slogger.Error("Failed to rotate token", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to rotate token"))
		return
	}

	// Return 201 Created with response
	httputil.WriteJSON(w, http.StatusCreated, response)
}

// RevokeToken handles DELETE /api/v0.9/gateways/:gatewayId/tokens/:tokenId
func (h *GatewayHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway ID is required"))
		return
	}

	tokenId := r.PathValue("tokenId")
	if tokenId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Token ID is required"))
		return
	}

	revokedBy, _ := middleware.GetUserIDFromRequest(r)
	err := h.gatewayService.RevokeToken(gatewayId, tokenId, orgId, revokedBy)
	if err != nil {
		errMsg := err.Error()

		if strings.Contains(errMsg, "not found") {
			h.slogger.Error("Resource not found during token revocation", "error", err)
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", errMsg))
			return
		}

		h.slogger.Error("Failed to revoke token", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to revoke token"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"message": "Token revoked successfully"})
}

// GetGatewayManifest handles GET /api/v0.9/gateways/{gatewayId}/manifest
// Called by APIM to retrieve the manifest pushed by the gateway controller on connect.
func (h *GatewayHandler) GetGatewayManifest(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Gateway ID is required"))
		return
	}

	dataFromDb, err := h.gatewayService.GetStoredManifest(gatewayId, orgId)
	if err != nil {
		if strings.Contains(err.Error(), "invalid UUID") {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"Invalid gateway ID format"))
			return
		}
		if strings.Contains(err.Error(), "gateway not found") {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Gateway not found"))
			return
		}
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to retrieve gateway manifest"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, manifestSyncResponse{
		Policies: dataFromDb.Policies,
	})
}

// SyncCustomPolicy handles POST /api/v0.9/gateway-custom-policies/sync
// It upserts a custom policy from the gateway's stored manifest into the gateway_custom_policies table.
func (h *GatewayHandler) SyncCustomPolicy(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	gatewayId := r.URL.Query().Get("gatewayId")
	policyName := r.URL.Query().Get("policyName")
	version := r.URL.Query().Get("policyVersion")

	if gatewayId == "" || policyName == "" || version == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"gatewayId, policyName and policyVersion are required"))
		return
	}

	policy, err := h.gatewayService.SyncCustomPolicy(gatewayId, orgId, policyName, version)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "gateway not found") {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", msg))
			return
		}
		if strings.Contains(msg, "not found in gateway manifest") {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", msg))
			return
		}
		if strings.Contains(msg, "not a custom policy") || strings.Contains(msg, "manifest is not available") {
			httputil.WriteJSON(w, http.StatusUnprocessableEntity, utils.NewErrorResponse(422, "Unprocessable Entity", msg))
			return
		}
		if strings.Contains(msg, "already exists") || strings.Contains(msg, "patch version updates are not allowed") || strings.Contains(msg, "cannot downgrade") {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", msg))
			return
		}
		h.slogger.Error("Failed to sync custom policy", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to sync custom policy"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, policy)
}

// GetCustomPolicy handles GET /api/v0.9/gateway-custom-policies/:customPolicyUuid/versions/:version
func (h *GatewayHandler) GetCustomPolicy(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	policyUUID := r.PathValue("gatewayCustomPolicyId")
	version := r.PathValue("version")
	if policyUUID == "" || version == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"customPolicyUuid and version are required"))
		return
	}

	policy, err := h.gatewayService.GetCustomPolicyByUUID(orgId, policyUUID, version)
	if err != nil {
		if errors.Is(err, constants.ErrCustomPolicyNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Custom policy not found"))
			return
		}
		if errors.Is(err, constants.ErrCustomPolicyVersionMismatch) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Custom policy not found with the specified version"))
			return
		}
		h.slogger.Error("Failed to get custom policy", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get custom policy"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, policy)
}

// DeleteCustomPolicy handles DELETE /api/v0.9/gateway-custom-policies/:customPolicyUuid/versions/:version
func (h *GatewayHandler) DeleteCustomPolicy(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	policyUUID := r.PathValue("gatewayCustomPolicyId")
	version := r.PathValue("version")
	if policyUUID == "" || version == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"customPolicyUuid and version are required"))
		return
	}

	err := h.gatewayService.DeleteCustomPolicyByUUID(orgId, policyUUID, version)
	if err != nil {
		if errors.Is(err, constants.ErrCustomPolicyNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Custom policy not found"))
			return
		}
		if errors.Is(err, constants.ErrCustomPolicyVersionMismatch) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Custom policy not found with the specified version"))
			return
		}
		if errors.Is(err, constants.ErrCustomPolicyInUse) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Custom policy is in use by one or more APIs and cannot be deleted"))
			return
		}
		h.slogger.Error("Failed to delete custom policy", "org_id", orgId, "policy_uuid", policyUUID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to delete custom policy"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListCustomPolicies handles GET /api/v0.9/gateway-custom-policies
func (h *GatewayHandler) ListCustomPolicies(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	policies, err := h.gatewayService.ListCustomPolicies(orgId)
	if err != nil {
		h.slogger.Error("Failed to list custom policies", "org_id", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to list custom policies"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, policies)
}

// RegisterRoutes registers gateway routes with the router
func (h *GatewayHandler) RegisterRoutes(mux *http.ServeMux) {
	h.slogger.Debug("Registering gateway routes")
	mux.HandleFunc("POST "+constants.APIBasePath+"/gateways", h.CreateGateway)
	mux.HandleFunc("GET "+constants.APIBasePath+"/gateways", h.ListGateways)
	mux.HandleFunc("GET "+constants.APIBasePath+"/gateways/{gatewayId}", h.GetGateway)
	mux.HandleFunc("PUT "+constants.APIBasePath+"/gateways/{gatewayId}", h.UpdateGateway)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/gateways/{gatewayId}", h.DeleteGateway)
	mux.HandleFunc("GET "+constants.APIBasePath+"/gateways/{gatewayId}/tokens", h.ListTokens)
	mux.HandleFunc("POST "+constants.APIBasePath+"/gateways/{gatewayId}/tokens", h.RotateToken)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/gateways/{gatewayId}/tokens/{tokenId}", h.RevokeToken)
	mux.HandleFunc("GET "+constants.APIBasePath+"/gateways/{gatewayId}/manifest", h.GetGatewayManifest)

	mux.HandleFunc("GET "+constants.APIBasePath+"/gateway-custom-policies", h.ListCustomPolicies)
	mux.HandleFunc("POST "+constants.APIBasePath+"/gateway-custom-policies/sync", h.SyncCustomPolicy)
	mux.HandleFunc("GET "+constants.APIBasePath+"/gateway-custom-policies/{gatewayCustomPolicyId}/versions/{version}", h.GetCustomPolicy)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/gateway-custom-policies/{gatewayCustomPolicyId}/versions/{version}", h.DeleteCustomPolicy)
}

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
	"regexp"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"

	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

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
	identity       *service.IdentityService
	slogger        *slog.Logger
}

// NewGatewayHandler creates a new gateway handler
func NewGatewayHandler(gatewayService *service.GatewayService, identity *service.IdentityService, slogger *slog.Logger) *GatewayHandler {
	return &GatewayHandler{
		gatewayService: gatewayService,
		identity:       identity,
		slogger:        slogger,
	}
}

// manifestSyncResponse is the response body for manifest-sync endpoints
type manifestSyncResponse struct {
	Policies json.RawMessage `json:"policies,omitempty"`
}

// CreateGateway handles POST /api/v0.9/gateways
func (h *GatewayHandler) CreateGateway(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	var req api.CreateGatewayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
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
		return apperror.ValidationFailed.New(
			"version must be in 'major.minor' format (e.g. '1.0') or CalVer 'YYYY.MM.DD' format (e.g. '2026.05.13')")
	}

	createdBy, err := resolveActorErr(r, h.identity, "register gateway")
	if err != nil {
		return err
	}
	gateway, err := h.gatewayService.RegisterGateway(orgId, req.Id, req.DisplayName, description, req.Endpoints,
		isCritical, functionalityType, version, createdBy, properties)
	if err != nil {
		// The service constructs typed catalog errors (not found, conflict,
		// validation) at the point of failure — pass them through untouched.
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to register gateway")
	}

	// Return 201 Created with response
	setLocation(w, "gateways", strOrEmpty(gateway.Id))
	httputil.WriteJSON(w, http.StatusCreated, gateway)
	return nil
}

// ListGateways handles GET /api/v0.9/gateways with constitution-compliant response
func (h *GatewayHandler) ListGateways(w http.ResponseWriter, r *http.Request) error {
	organizationID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	opts := parseListOptions(r)

	gateways, err := h.gatewayService.ListGateways(&organizationID, opts)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage("failed to list gateways")
	}

	// Return 200 OK with constitution-compliant envelope structure
	httputil.WriteJSON(w, http.StatusOK, gateways)
	return nil
}

// GetGateway handles GET /api/v0.9/gateways/:gatewayId
func (h *GatewayHandler) GetGateway(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	// Extract UUID path parameter
	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		return apperror.ValidationFailed.New("Gateway ID is required")
	}

	gateway, err := h.gatewayService.GetGateway(gatewayId, orgId)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		if strings.Contains(err.Error(), "invalid UUID") {
			return apperror.ValidationFailed.Wrap(err, "Invalid gateway ID format")
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to retrieve gateway")
	}

	// Return 200 OK with gateway details
	httputil.WriteJSON(w, http.StatusOK, gateway)
	return nil
}

// GetGatewayStatus retrieves gateway status, optionally filtered by gatewayId query param.
func (h *GatewayHandler) GetGatewayStatus(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	gatewayId := r.URL.Query().Get("gatewayId")
	var gatewayIdPtr *string
	if gatewayId != "" {
		gatewayIdPtr = &gatewayId
	}

	status, err := h.gatewayService.GetGatewayStatus(orgId, gatewayIdPtr)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to get gateway status")
	}

	httputil.WriteJSON(w, http.StatusOK, status)
	return nil
}

// UpdateGateway handles PUT /api/v0.9/gateways/:gatewayId
func (h *GatewayHandler) UpdateGateway(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		return apperror.ValidationFailed.New("Gateway ID is required")
	}

	var req api.GatewayResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	if err := utils.ValidateHandleImmutable(gatewayId, req.Id); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Gateway id is immutable and cannot be changed")
	}

	if req.DisplayName == "" {
		return apperror.ValidationFailed.New("displayName is required")
	}

	updatedBy, err := resolveActorErr(r, h.identity, "update gateway")
	if err != nil {
		return err
	}
	response, err := h.gatewayService.UpdateGateway(gatewayId, orgId, updatedBy, &req)
	if err != nil {
		if errors.Is(err, constants.ErrGatewayNotFound) {
			return apperror.GatewayNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to update gateway")
	}

	httputil.WriteJSON(w, http.StatusOK, response)
	return nil
}

// DeleteGateway handles DELETE /api/v0.9/gateways/:gatewayId
func (h *GatewayHandler) DeleteGateway(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	// Extract UUID path parameter
	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		return apperror.ValidationFailed.New("Gateway ID is required")
	}

	deletedBy, err := resolveActorErr(r, h.identity, "delete gateway")
	if err != nil {
		return err
	}
	if err := h.gatewayService.DeleteGateway(gatewayId, orgId, deletedBy); err != nil {
		// Check for specific error types
		if errors.Is(err, constants.ErrGatewayNotFound) {
			return apperror.GatewayNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrGatewayHasAssociatedAPIs) {
			return apperror.GatewayHasActiveDeployments.Wrap(err)
		}

		if strings.Contains(err.Error(), "invalid UUID") {
			return apperror.ValidationFailed.Wrap(err, "Invalid gateway ID format")
		}

		// Internal server error
		return apperror.Internal.Wrap(err).WithLogMessage("failed to delete gateway")
	}

	// Return 204 No Content on successful deletion
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ListTokens handles GET /api/v0.9/gateways/:gatewayId/tokens
func (h *GatewayHandler) ListTokens(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		return apperror.ValidationFailed.New("Gateway ID is required")
	}

	limit, offset := parsePagination(r)

	tokens, err := h.gatewayService.ListTokens(gatewayId, orgId, limit, offset)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to list tokens")
	}

	httputil.WriteJSON(w, http.StatusOK, tokens)
	return nil
}

// RotateToken handles POST /api/v0.9/gateways/:gatewayId/tokens
func (h *GatewayHandler) RotateToken(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	// Extract ID path parameter
	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		return apperror.ValidationFailed.New("Gateway ID is required")
	}

	createdBy, err := resolveActorErr(r, h.identity, "rotate gateway token")
	if err != nil {
		return err
	}
	response, err := h.gatewayService.RotateToken(gatewayId, orgId, createdBy)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to rotate token")
	}

	// Return 201 Created with response
	tokenId := ""
	if response.Id != nil {
		tokenId = response.Id.String()
	}
	setLocation(w, "gateways", gatewayId, "tokens", tokenId)
	httputil.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// RevokeToken handles DELETE /api/v0.9/gateways/:gatewayId/tokens/:tokenId
func (h *GatewayHandler) RevokeToken(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		return apperror.ValidationFailed.New("Gateway ID is required")
	}

	tokenId := r.PathValue("tokenId")
	if tokenId == "" {
		return apperror.ValidationFailed.New("Token ID is required")
	}

	revokedBy, err := resolveActorErr(r, h.identity, "revoke gateway token")
	if err != nil {
		return err
	}
	if err := h.gatewayService.RevokeToken(gatewayId, tokenId, orgId, revokedBy); err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to revoke token")
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"message": "Token revoked successfully"})
	return nil
}

// GetGatewayManifest handles GET /api/v0.9/gateways/{gatewayId}/manifest
// Called by APIM to retrieve the manifest pushed by the gateway controller on connect.
func (h *GatewayHandler) GetGatewayManifest(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	gatewayId := r.PathValue("gatewayId")
	if gatewayId == "" {
		return apperror.ValidationFailed.New("Gateway ID is required")
	}

	dataFromDb, err := h.gatewayService.GetStoredManifest(gatewayId, orgId)
	if err != nil {
		if strings.Contains(err.Error(), "invalid UUID") {
			return apperror.ValidationFailed.Wrap(err, "Invalid gateway ID format")
		}
		if errors.Is(err, constants.ErrGatewayNotFound) {
			return apperror.GatewayNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to retrieve gateway manifest")
	}

	httputil.WriteJSON(w, http.StatusOK, manifestSyncResponse{
		Policies: dataFromDb.Policies,
	})
	return nil
}

// SyncCustomPolicy handles POST /api/v0.9/gateway-custom-policies/sync
// It upserts a custom policy from the gateway's stored manifest into the gateway_custom_policies table.
func (h *GatewayHandler) SyncCustomPolicy(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	gatewayId := r.URL.Query().Get("gatewayId")
	policyName := r.URL.Query().Get("policyName")
	version := r.URL.Query().Get("policyVersion")

	if gatewayId == "" || policyName == "" || version == "" {
		return apperror.ValidationFailed.New("gatewayId, policyName and policyVersion are required")
	}

	policy, err := h.gatewayService.SyncCustomPolicy(gatewayId, orgId, policyName, version)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to sync custom policy")
	}

	httputil.WriteJSON(w, http.StatusOK, policy)
	return nil
}

// GetCustomPolicy handles GET /api/v0.9/gateway-custom-policies/:customPolicyUuid/versions/:version
func (h *GatewayHandler) GetCustomPolicy(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	policyUUID := r.PathValue("gatewayCustomPolicyId")
	version := r.PathValue("version")
	if policyUUID == "" || version == "" {
		return apperror.ValidationFailed.New("customPolicyUuid and version are required")
	}

	policy, err := h.gatewayService.GetCustomPolicyByUUID(orgId, policyUUID, version)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to get custom policy")
	}

	httputil.WriteJSON(w, http.StatusOK, policy)
	return nil
}

// DeleteCustomPolicy handles DELETE /api/v0.9/gateway-custom-policies/:customPolicyUuid/versions/:version
func (h *GatewayHandler) DeleteCustomPolicy(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	policyUUID := r.PathValue("gatewayCustomPolicyId")
	version := r.PathValue("version")
	if policyUUID == "" || version == "" {
		return apperror.ValidationFailed.New("customPolicyUuid and version are required")
	}

	if err := h.gatewayService.DeleteCustomPolicyByUUID(orgId, policyUUID, version); err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		// Repository-origin sentinels (delete-if-unused path) are still untyped.
		if errors.Is(err, constants.ErrCustomPolicyNotFound) {
			return apperror.CustomPolicyNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrCustomPolicyInUse) {
			return apperror.PolicyInUse.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to delete custom policy, orgID=%s, policyUUID=%s", orgId, policyUUID))
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ListCustomPolicies handles GET /api/v0.9/gateway-custom-policies
func (h *GatewayHandler) ListCustomPolicies(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	limit, offset := parsePagination(r)

	policies, total, err := h.gatewayService.ListCustomPolicies(orgId, limit, offset)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to list custom policies, orgID=%s", orgId))
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"count": len(policies),
		"list":  policies,
		"pagination": api.Pagination{
			Total:  total,
			Offset: offset,
			Limit:  limit,
		},
	})
	return nil
}

// RegisterRoutes registers gateway routes with the router
func (h *GatewayHandler) RegisterRoutes(mux *http.ServeMux) {
	h.slogger.Debug("Registering gateway routes")
	mux.HandleFunc("POST "+constants.APIBasePath+"/gateways", middleware.MapErrors(h.slogger, h.CreateGateway))
	mux.HandleFunc("GET "+constants.APIBasePath+"/gateways", middleware.MapErrors(h.slogger, h.ListGateways))
	mux.HandleFunc("GET "+constants.APIBasePath+"/gateways/{gatewayId}", middleware.MapErrors(h.slogger, h.GetGateway))
	mux.HandleFunc("PUT "+constants.APIBasePath+"/gateways/{gatewayId}", middleware.MapErrors(h.slogger, h.UpdateGateway))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/gateways/{gatewayId}", middleware.MapErrors(h.slogger, h.DeleteGateway))
	mux.HandleFunc("GET "+constants.APIBasePath+"/gateways/{gatewayId}/tokens", middleware.MapErrors(h.slogger, h.ListTokens))
	mux.HandleFunc("POST "+constants.APIBasePath+"/gateways/{gatewayId}/tokens", middleware.MapErrors(h.slogger, h.RotateToken))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/gateways/{gatewayId}/tokens/{tokenId}", middleware.MapErrors(h.slogger, h.RevokeToken))
	mux.HandleFunc("GET "+constants.APIBasePath+"/gateways/{gatewayId}/manifest", middleware.MapErrors(h.slogger, h.GetGatewayManifest))

	mux.HandleFunc("GET "+constants.APIBasePath+"/gateway-custom-policies", middleware.MapErrors(h.slogger, h.ListCustomPolicies))
	mux.HandleFunc("POST "+constants.APIBasePath+"/gateway-custom-policies/sync", middleware.MapErrors(h.slogger, h.SyncCustomPolicy))
	mux.HandleFunc("GET "+constants.APIBasePath+"/gateway-custom-policies/{gatewayCustomPolicyId}/versions/{version}", middleware.MapErrors(h.slogger, h.GetCustomPolicy))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/gateway-custom-policies/{gatewayCustomPolicyId}/versions/{version}", middleware.MapErrors(h.slogger, h.DeleteCustomPolicy))
}

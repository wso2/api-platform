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
	"strconv"
	"strings"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/model"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

// SubscriptionHandler handles application-level subscription CRUD
type SubscriptionHandler struct {
	subscriptionService     *service.SubscriptionService
	subscriptionPlanService *service.SubscriptionPlanService
	slogger                 *slog.Logger
}

// NewSubscriptionHandler creates a new subscription handler
func NewSubscriptionHandler(subscriptionService *service.SubscriptionService, subscriptionPlanService *service.SubscriptionPlanService, slogger *slog.Logger) *SubscriptionHandler {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &SubscriptionHandler{
		subscriptionService:     subscriptionService,
		subscriptionPlanService: subscriptionPlanService,
		slogger:                 slogger,
	}
}

// CreateSubscriptionRequest is the body for POST /api/v0.9/subscriptions
type CreateSubscriptionRequest struct {
	APIID              string  `json:"apiId" binding:"required"`
	SubscriberID       string  `json:"subscriberId" binding:"required"`
	ApplicationID      *string `json:"applicationId,omitempty"`
	SubscriptionPlanID *string `json:"subscriptionPlanId,omitempty"`
	Status             string  `json:"status,omitempty"`
}

// UpdateSubscriptionRequest is the body for PUT /api/v0.9/subscriptions/:subscriptionId
type UpdateSubscriptionRequest struct {
	Status string `json:"status,omitempty"`
}

// CreateSubscription handles POST /api/v0.9/subscriptions
func (h *SubscriptionHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		h.slogger.Error("Organization claim not found in token when creating subscription")
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}
	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Error("Invalid create subscription request body", "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}
	if req.APIID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API ID is required"))
		return
	}
	if req.SubscriberID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "subscriberId is required"))
		return
	}
	switch req.Status {
	case "", "ACTIVE", "INACTIVE", "REVOKED":
	default:
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid status value"))
		return
	}
	sub, err := h.subscriptionService.CreateSubscription(req.APIID, orgId, req.SubscriberID, req.ApplicationID, req.SubscriptionPlanID, req.Status)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API not found"))
			return
		}
		if errors.Is(err, constants.ErrSubscriptionAlreadyExists) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Subscription already exists for this API"))
			return
		}
		h.slogger.Error("Failed to create subscription", "apiId", req.APIID, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create subscription"))
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, h.toSubscriptionResponse(sub, orgId))
}

// ListSubscriptions handles GET /api/v0.9/subscriptions
func (h *SubscriptionHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	apiId := r.URL.Query().Get("apiId")
	subscriberID := r.URL.Query().Get("subscriberId")
	applicationID := r.URL.Query().Get("applicationId")
	status := r.URL.Query().Get("status")

	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var apiIDPtr, subscriberIDPtr, appIDPtr, statusPtr *string
	if apiId != "" {
		apiIDPtr = &apiId
	}
	if subscriberID != "" {
		subscriberIDPtr = &subscriberID
	}
	if applicationID != "" {
		appIDPtr = &applicationID
	}
	if status != "" {
		switch status {
		case "ACTIVE", "INACTIVE", "REVOKED":
		default:
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid status value"))
			return
		}
		statusPtr = &status
	}
	var limitStr string
	if v := r.URL.Query().Get("limit"); v != "" {
		limitStr = v
	} else {
		limitStr = "20"
	}
	var offsetStr string
	if v := r.URL.Query().Get("offset"); v != "" {
		offsetStr = v
	} else {
		offsetStr = "0"
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}
	list, total, err := h.subscriptionService.ListSubscriptionsByFilters(orgId, apiIDPtr, subscriberIDPtr, appIDPtr, statusPtr, limit, offset)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API not found"))
			return
		}
		h.slogger.Error("Failed to list subscriptions", "apiId", apiId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list subscriptions"))
		return
	}
	// Bulk fetch API handles and plan names to avoid N+1 queries
	apiUUIDSet := make(map[string]struct{})
	planIDSet := make(map[string]struct{})
	for _, sub := range list {
		if sub.ArtifactUUID != "" {
			apiUUIDSet[sub.ArtifactUUID] = struct{}{}
		}
		if sub.SubscriptionPlanID != nil && *sub.SubscriptionPlanID != "" {
			planIDSet[*sub.SubscriptionPlanID] = struct{}{}
		}
	}
	apiUUIDs := make([]string, 0, len(apiUUIDSet))
	for u := range apiUUIDSet {
		apiUUIDs = append(apiUUIDs, u)
	}
	planIDs := make([]string, 0, len(planIDSet))
	for id := range planIDSet {
		planIDs = append(planIDs, id)
	}
	apiHandleMap, err := h.subscriptionService.GetAPIHandleMap(apiUUIDs, orgId)
	if err != nil {
		h.slogger.Error("Failed to bulk fetch API handles for list", "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list subscriptions"))
		return
	}
	planNameMap, err := h.subscriptionPlanService.GetPlanNameMap(planIDs, orgId)
	if err != nil {
		h.slogger.Error("Failed to bulk fetch plan names for list", "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list subscriptions"))
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, sub := range list {
		items = append(items, h.toSubscriptionResponseWithMaps(sub, orgId, apiHandleMap, planNameMap))
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"subscriptions": items,
		"count":         len(items),
		"pagination": map[string]any{
			"total":  total,
			"offset": offset,
			"limit":  limit,
		},
	})
}

// GetSubscription handles GET /api/v0.9/subscriptions/:subscriptionId
func (h *SubscriptionHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}
	subscriptionId := r.PathValue("subscriptionId")
	if subscriptionId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Subscription ID is required"))
		return
	}
	sub, err := h.subscriptionService.GetSubscription(subscriptionId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription not found"))
			return
		}
		h.slogger.Error("Failed to get subscription", "subscriptionId", subscriptionId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get subscription"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, h.toSubscriptionResponse(sub, orgId))
}

// UpdateSubscription handles PUT /api/v0.9/subscriptions/:subscriptionId
func (h *SubscriptionHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}
	subscriptionId := r.PathValue("subscriptionId")
	if subscriptionId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Subscription ID is required"))
		return
	}
	var req UpdateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}
	switch req.Status {
	case "", "ACTIVE", "INACTIVE", "REVOKED":
	default:
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid subscription status"))
		return
	}
	subscriberID, ok := requireSubscriptionSubscriberQuery(w, r)
	if !ok {
		return
	}
	sub, err := h.subscriptionService.UpdateSubscription(subscriptionId, orgId, subscriberID, req.Status)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription not found"))
			return
		}
		if errors.Is(err, constants.ErrSubscriptionSubscriberMismatch) {
			httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden", "subscriberId does not match this subscription"))
			return
		}
		h.slogger.Error("Failed to update subscription", "subscriptionId", subscriptionId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update subscription"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, h.toSubscriptionResponse(sub, orgId))
}

// DeleteSubscription handles DELETE /api/v0.9/subscriptions/:subscriptionId
func (h *SubscriptionHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}
	subscriptionId := r.PathValue("subscriptionId")
	if subscriptionId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Subscription ID is required"))
		return
	}
	subscriberID, ok := requireSubscriptionSubscriberQuery(w, r)
	if !ok {
		return
	}
	err := h.subscriptionService.DeleteSubscription(subscriptionId, orgId, subscriberID)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription not found"))
			return
		}
		if errors.Is(err, constants.ErrSubscriptionSubscriberMismatch) {
			httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden", "subscriberId does not match this subscription"))
			return
		}
		h.slogger.Error("Failed to delete subscription", "subscriptionId", subscriptionId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete subscription"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func requireSubscriptionSubscriberQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	q := strings.TrimSpace(r.URL.Query().Get("subscriberId"))
	if q == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "subscriberId query parameter is required"))
		return "", false
	}
	return q, true
}

func (h *SubscriptionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/subscriptions", h.CreateSubscription)
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscriptions", h.ListSubscriptions)
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscriptions/{subscriptionId}", h.GetSubscription)
	mux.HandleFunc("PUT "+constants.APIBasePath+"/subscriptions/{subscriptionId}", h.UpdateSubscription)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/subscriptions/{subscriptionId}", h.DeleteSubscription)
}

func (h *SubscriptionHandler) toSubscriptionResponse(sub *model.Subscription, orgId string) map[string]any {
	// apiId in response should be the handle (e.g. "samp1"), not the internal UUID
	apiIdForResponse := h.subscriptionService.ResolveAPIHandle(sub.ArtifactUUID, orgId)
	if apiIdForResponse == "" {
		apiIdForResponse = sub.ArtifactUUID // fallback to UUID
	}
	resp := map[string]any{
		"uuid":           sub.UUID,
		"apiId":          apiIdForResponse,
		"subscriberId":   sub.SubscriberID,
		"organizationId": sub.OrganizationUUID,
		"status":         string(sub.Status),
		"createdAt":      sub.CreatedAt,
		"updatedAt":      sub.UpdatedAt,
	}
	if sub.ApplicationID != nil {
		resp["applicationId"] = *sub.ApplicationID
	}
	if sub.SubscriptionPlanID != nil {
		resp["subscriptionPlanId"] = *sub.SubscriptionPlanID
		if h.subscriptionPlanService != nil {
			plan, err := h.subscriptionPlanService.GetPlan(*sub.SubscriptionPlanID, orgId)
			if err == nil && plan != nil {
				resp["subscriptionPlanName"] = plan.Name
			}
		}
	}
	// subscriptionToken is decrypted from DB; empty for legacy hashed tokens
	if sub.SubscriptionToken != "" {
		resp["subscriptionToken"] = sub.SubscriptionToken
	}
	return resp
}

// toSubscriptionResponseWithMaps builds a subscription response using pre-fetched lookup maps.
// Used by ListSubscriptions to avoid N+1 queries.
func (h *SubscriptionHandler) toSubscriptionResponseWithMaps(sub *model.Subscription, orgId string, apiHandleMap, planNameMap map[string]string) map[string]any {
	apiIdForResponse := apiHandleMap[sub.ArtifactUUID]
	if apiIdForResponse == "" {
		apiIdForResponse = sub.ArtifactUUID // fallback to UUID
	}
	resp := map[string]any{
		"uuid":           sub.UUID,
		"apiId":          apiIdForResponse,
		"subscriberId":   sub.SubscriberID,
		"organizationId": sub.OrganizationUUID,
		"status":         string(sub.Status),
		"createdAt":      sub.CreatedAt,
		"updatedAt":      sub.UpdatedAt,
	}
	if sub.ApplicationID != nil {
		resp["applicationId"] = *sub.ApplicationID
	}
	if sub.SubscriptionPlanID != nil {
		resp["subscriptionPlanId"] = *sub.SubscriptionPlanID
		if name := planNameMap[*sub.SubscriptionPlanID]; name != "" {
			resp["subscriptionPlanName"] = name
		}
	}
	if sub.SubscriptionToken != "" {
		resp["subscriptionToken"] = sub.SubscriptionToken
	}
	return resp
}

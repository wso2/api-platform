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
	"strconv"
	"time"

	api "platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/model"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

// SubscriptionPlanHandler handles subscription plan CRUD
type SubscriptionPlanHandler struct {
	planService *service.SubscriptionPlanService
	slogger     *slog.Logger
}

// NewSubscriptionPlanHandler creates a new subscription plan handler
func NewSubscriptionPlanHandler(planService *service.SubscriptionPlanService, slogger *slog.Logger) *SubscriptionPlanHandler {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &SubscriptionPlanHandler{
		planService: planService,
		slogger:     slogger,
	}
}

// validateThrottleLimitPair ensures throttleLimitCount and throttleLimitUnit are provided together,
// count is at least 1, and unit is one of the accepted throttle limit units.
func validateThrottleLimitPair(count *int, unit *string) string {
	if (count != nil && unit == nil) || (count == nil && unit != nil) {
		return "throttleLimitCount and throttleLimitUnit must be provided together"
	}
	if count != nil && unit != nil {
		if *count < 1 {
			return "throttleLimitCount must be at least 1"
		}
		if !constants.ValidThrottleLimitUnits[*unit] {
			return "throttleLimitUnit must be one of: MINUTE, HOUR, DAY, MONTH"
		}
	}
	return ""
}

// CreateSubscriptionPlanRequest is the body for POST /api/v0.9/subscription-plans
type CreateSubscriptionPlanRequest struct {
	Id                 string  `json:"id" binding:"required"`
	DisplayName        string  `json:"displayName" binding:"required"`
	BillingPlan        string  `json:"billingPlan,omitempty"`
	StopOnQuotaReach   *bool   `json:"stopOnQuotaReach,omitempty"`
	ThrottleLimitCount *int    `json:"throttleLimitCount,omitempty"`
	ThrottleLimitUnit  *string `json:"throttleLimitUnit,omitempty"`
	ExpiryTime         *string `json:"expiryTime,omitempty"`
	Status             string  `json:"status,omitempty"`
}


// CreateSubscriptionPlan handles POST /api/v0.9/subscription-plans
func (h *SubscriptionPlanHandler) CreateSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req CreateSubscriptionPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Error("Invalid create subscription plan request body", "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if req.Status != "" {
		switch req.Status {
		case "ACTIVE", "INACTIVE":
		default:
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid status value; must be ACTIVE or INACTIVE"))
			return
		}
	}

	if errMsg := validateThrottleLimitPair(req.ThrottleLimitCount, req.ThrottleLimitUnit); errMsg != "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
		return
	}

	var throttleLimitUnit string
	if req.ThrottleLimitUnit != nil {
		throttleLimitUnit = *req.ThrottleLimitUnit
	}
	plan := &model.SubscriptionPlan{
		Handle:             req.Id,
		Name:               req.DisplayName,
		BillingPlan:        req.BillingPlan,
		StopOnQuotaReach:   true,
		ThrottleLimitCount: req.ThrottleLimitCount,
		ThrottleLimitUnit:  throttleLimitUnit,
		Status:             model.SubscriptionPlanStatus(req.Status),
	}
	if req.StopOnQuotaReach != nil {
		plan.StopOnQuotaReach = *req.StopOnQuotaReach
	}
	if req.ExpiryTime != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiryTime)
		if err != nil {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid expiryTime format; use RFC3339"))
			return
		}
		plan.ExpiryTime = &t
	}

	actor, ok := middleware.GetUserIDFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "User ID claim not found in token"))
		return
	}
	created, err := h.planService.CreatePlan(orgId, actor, plan)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionPlanAlreadyExists) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", err.Error()))
			return
		}
		h.slogger.Error("Failed to create subscription plan", "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create subscription plan"))
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, toSubscriptionPlanResponse(created))
}

// ListSubscriptionPlans handles GET /api/v0.9/subscription-plans
func (h *SubscriptionPlanHandler) ListSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
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

	list, err := h.planService.ListPlans(orgId, limit, offset)
	if err != nil {
		h.slogger.Error("Failed to list subscription plans", "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list subscription plans"))
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, p := range list {
		items = append(items, toSubscriptionPlanResponse(p))
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"subscriptionPlans": items, "count": len(items)})
}

// GetSubscriptionPlan handles GET /api/v0.9/subscription-plans/:planId
func (h *SubscriptionPlanHandler) GetSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	planId := r.PathValue("id")
	if planId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Plan ID is required"))
		return
	}

	plan, err := h.planService.GetPlan(planId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionPlanNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription plan not found"))
			return
		}
		h.slogger.Error("Failed to get subscription plan", "planId", planId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get subscription plan"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, toSubscriptionPlanResponse(plan))
}

// UpdateSubscriptionPlan handles PUT /api/v0.9/subscription-plans/:planId
func (h *SubscriptionPlanHandler) UpdateSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	planId := r.PathValue("id")
	if planId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Plan ID is required"))
		return
	}

	var req api.SubscriptionPlan
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Error("Invalid update subscription plan request body", "planId", planId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if req.Id != nil && *req.Id != "" && *req.Id != planId {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"The plan id is immutable and cannot be changed"))
		return
	}

	if errMsg := validateThrottleLimitPair(req.ThrottleLimitCount, req.ThrottleLimitUnit); errMsg != "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
		return
	}

	displayName := req.DisplayName
	if displayName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "displayName is required"))
		return
	}
	update := &model.SubscriptionPlanUpdate{
		Name:               &displayName,
		BillingPlan:        req.BillingPlan,
		StopOnQuotaReach:   req.StopOnQuotaReach,
		ThrottleLimitCount: req.ThrottleLimitCount,
		ThrottleLimitUnit:  req.ThrottleLimitUnit,
		ExpiryTime:         req.ExpiryTime,
	}
	if req.Status != nil {
		switch model.SubscriptionPlanStatus(*req.Status) {
		case model.SubscriptionPlanStatusActive, model.SubscriptionPlanStatusInactive:
			st := model.SubscriptionPlanStatus(*req.Status)
			update.Status = &st
		default:
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid status value; must be ACTIVE or INACTIVE"))
			return
		}
	}

	actor, ok := middleware.GetUserIDFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "User ID claim not found in token"))
		return
	}
	updated, err := h.planService.UpdatePlan(planId, orgId, actor, update)
	if err != nil {
		if errors.Is(err, constants.ErrHandleImmutable) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
				"The plan id is immutable and cannot be changed"))
			return
		}
		if errors.Is(err, constants.ErrSubscriptionPlanNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription plan not found"))
			return
		}
		if errors.Is(err, constants.ErrSubscriptionPlanAlreadyExists) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", err.Error()))
			return
		}
		h.slogger.Error("Failed to update subscription plan", "planId", planId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update subscription plan"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, toSubscriptionPlanResponse(updated))
}

// DeleteSubscriptionPlan handles DELETE /api/v0.9/subscription-plans/:planId
func (h *SubscriptionPlanHandler) DeleteSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	planId := r.PathValue("id")
	if planId == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Plan ID is required"))
		return
	}

	actor, ok := middleware.GetUserIDFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "User ID claim not found in token"))
		return
	}
	err := h.planService.DeletePlan(planId, orgId, actor)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionPlanNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription plan not found"))
			return
		}
		h.slogger.Error("Failed to delete subscription plan", "planId", planId, "organizationId", orgId, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete subscription plan"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RegisterRoutes registers subscription plan routes
func (h *SubscriptionPlanHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/subscription-plans", h.CreateSubscriptionPlan)
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscription-plans", h.ListSubscriptionPlans)
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscription-plans/{id}", h.GetSubscriptionPlan)
	mux.HandleFunc("PUT "+constants.APIBasePath+"/subscription-plans/{id}", h.UpdateSubscriptionPlan)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/subscription-plans/{id}", h.DeleteSubscriptionPlan)
}

func toSubscriptionPlanResponse(plan *model.SubscriptionPlan) map[string]any {
	resp := map[string]any{
		"uuid":             plan.UUID,
		"id":               plan.Handle,
		"displayName":      plan.Name,
		"billingPlan":      plan.BillingPlan,
		"stopOnQuotaReach": plan.StopOnQuotaReach,
		"organizationId":   plan.OrganizationUUID,
		"status":           string(plan.Status),
		"createdAt":        plan.CreatedAt,
		"updatedAt":        plan.UpdatedAt,
	}
	if plan.ThrottleLimitCount != nil {
		resp["throttleLimitCount"] = *plan.ThrottleLimitCount
	}
	if plan.ThrottleLimitUnit != "" {
		resp["throttleLimitUnit"] = plan.ThrottleLimitUnit
	}
	if plan.ExpiryTime != nil {
		resp["expiryTime"] = plan.ExpiryTime
	}
	return resp
}

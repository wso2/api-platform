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
	"strconv"
	"time"

	api "github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/go-httpkit/httputil"
)

// SubscriptionPlanHandler handles subscription plan CRUD
type SubscriptionPlanHandler struct {
	planService *service.SubscriptionPlanService
	identity    *service.IdentityService
	slogger     *slog.Logger
}

// NewSubscriptionPlanHandler creates a new subscription plan handler
func NewSubscriptionPlanHandler(planService *service.SubscriptionPlanService, identity *service.IdentityService, slogger *slog.Logger) *SubscriptionPlanHandler {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &SubscriptionPlanHandler{
		planService: planService,
		identity:    identity,
		slogger:     slogger,
	}
}

// SubscriptionPlanLimitRequest is a single throttling limit entry within a
// subscription plan create/update request.
//
// NOTE: SINGLE-LIMIT ASSUMPTION. subscription_plan_limits supports multiple limits
// per plan, but the platform-api currently only persists and enforces the first
// entry of the limits array on a request; any further entries are accepted but
// silently ignored. This must be improved to write/enforce all submitted limits.
type SubscriptionPlanLimitRequest struct {
	LimitType        string `json:"limitType,omitempty"`
	TimeUnit         string `json:"timeUnit"`
	TimeAmount       int    `json:"timeAmount,omitempty"`
	LimitCount       int    `json:"limitCount"`
	LimitCountUnit   string `json:"limitCountUnit,omitempty"`
	StopOnQuotaReach *bool  `json:"stopOnQuotaReach,omitempty"`
}

// normalizeAndValidateLimit fills in defaults and validates a single limit entry.
// Returns an error message if the entry is invalid.
func normalizeAndValidateLimit(l *SubscriptionPlanLimitRequest) string {
	if l.LimitType == "" {
		l.LimitType = constants.LimitTypeRequestCount
	} else if l.LimitType != constants.LimitTypeRequestCount {
		return "limitType: only REQUEST_COUNT is currently supported"
	}
	if !constants.ValidThrottleLimitUnits[l.TimeUnit] {
		return "timeUnit is required and must be one of: MINUTE, HOUR, DAY, MONTH"
	}
	if l.LimitCount < 1 {
		return "limitCount must be at least 1"
	}
	if l.TimeAmount == 0 {
		l.TimeAmount = 1
	} else if l.TimeAmount < 0 {
		return "timeAmount must be at least 1"
	}
	return ""
}

// apiLimitsToRequests converts generated api.SubscriptionPlanLimit entries (used by the
// PUT update body) into the internal SubscriptionPlanLimitRequest shape.
func apiLimitsToRequests(limits []api.SubscriptionPlanLimit) []SubscriptionPlanLimitRequest {
	out := make([]SubscriptionPlanLimitRequest, 0, len(limits))
	for _, l := range limits {
		var limitType string
		if l.LimitType != nil {
			limitType = string(*l.LimitType)
		}
		var limitCountUnit string
		if l.LimitCountUnit != nil {
			limitCountUnit = *l.LimitCountUnit
		}
		var timeAmount int
		if l.TimeAmount != nil {
			timeAmount = *l.TimeAmount
		}
		out = append(out, SubscriptionPlanLimitRequest{
			LimitType:        limitType,
			TimeUnit:         string(l.TimeUnit),
			TimeAmount:       timeAmount,
			LimitCount:       l.LimitCount,
			LimitCountUnit:   limitCountUnit,
			StopOnQuotaReach: l.StopOnQuotaReach,
		})
	}
	return out
}

// firstLimit returns a pointer to the first entry of limits, or nil if empty.
// Any entries beyond the first are ignored (see SubscriptionPlanLimitRequest).
func firstLimit(limits []SubscriptionPlanLimitRequest) *SubscriptionPlanLimitRequest {
	if len(limits) == 0 {
		return nil
	}
	l := limits[0]
	return &l
}

// CreateSubscriptionPlanRequest is the body for POST /api/v0.9/subscription-plans
type CreateSubscriptionPlanRequest struct {
	Id          string                         `json:"id" binding:"required"`
	DisplayName string                         `json:"displayName" binding:"required"`
	Limits      []SubscriptionPlanLimitRequest `json:"limits,omitempty"`
	ExpiryTime  *string                        `json:"expiryTime,omitempty"`
	Status      string                         `json:"status,omitempty"`
}

// CreateSubscriptionPlan handles POST /api/v0.9/subscription-plans
func (h *SubscriptionPlanHandler) CreateSubscriptionPlan(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var req CreateSubscriptionPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid create subscription plan request body for org %s", orgId))
	}

	if req.Id == "" {
		return apperror.ValidationFailed.New("id is required")
	}
	if req.DisplayName == "" {
		return apperror.ValidationFailed.New("displayName is required")
	}

	if req.Status != "" {
		switch req.Status {
		case "ACTIVE", "INACTIVE":
		default:
			return apperror.ValidationFailed.New("Invalid status value; must be ACTIVE or INACTIVE")
		}
	}

	plan := &model.SubscriptionPlan{
		Handle:           req.Id,
		Name:             req.DisplayName,
		StopOnQuotaReach: true,
		Status:           model.SubscriptionPlanStatus(req.Status),
	}
	if limit := firstLimit(req.Limits); limit != nil {
		if errMsg := normalizeAndValidateLimit(limit); errMsg != "" {
			return apperror.ValidationFailed.New(errMsg)
		}
		count := limit.LimitCount
		plan.ThrottleLimitCount = &count
		plan.ThrottleLimitUnit = limit.TimeUnit
		if limit.StopOnQuotaReach != nil {
			plan.StopOnQuotaReach = *limit.StopOnQuotaReach
		}
	}
	if req.ExpiryTime != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiryTime)
		if err != nil {
			return apperror.ValidationFailed.Wrap(err, "Invalid expiryTime format; use RFC3339")
		}
		plan.ExpiryTime = &t
	}

	rawActor, ok := middleware.GetActorIdentityFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("user id claim not found in token")
	}
	actor, err := h.identity.ToInternalUUID(rawActor)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage("failed to resolve user identity")
	}
	created, err := h.planService.CreatePlan(orgId, actor, plan)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionPlanAlreadyExists) {
			return apperror.SubscriptionPlanExists.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to create subscription plan for org %s", orgId))
	}
	resp, err := h.toSubscriptionPlanResponse(created, true)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to resolve subscription plan identity for org %s", orgId))
	}
	httputil.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

// ListSubscriptionPlans handles GET /api/v0.9/subscription-plans
func (h *SubscriptionPlanHandler) ListSubscriptionPlans(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
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
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to list subscription plans for org %s", orgId))
	}
	items := make([]map[string]any, 0, len(list))
	for _, p := range list {
		item, err := h.toSubscriptionPlanResponse(p, false)
		if err != nil {
			return apperror.Internal.Wrap(err).
				WithLogMessage(fmt.Sprintf("failed to resolve subscription plan identity for org %s", orgId))
		}
		items = append(items, item)
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"subscriptionPlans": items, "count": len(items)})
	return nil
}

// GetSubscriptionPlan handles GET /api/v0.9/subscription-plans/:planId
func (h *SubscriptionPlanHandler) GetSubscriptionPlan(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	planId := r.PathValue("subscriptionPlanId")
	if planId == "" {
		return apperror.ValidationFailed.New("Plan ID is required")
	}

	plan, err := h.planService.GetPlan(planId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionPlanNotFound) {
			return apperror.SubscriptionPlanNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to get subscription plan %s in org %s", planId, orgId))
	}
	resp, err := h.toSubscriptionPlanResponse(plan, true)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to resolve subscription plan identity for plan %s in org %s", planId, orgId))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// UpdateSubscriptionPlan handles PUT /api/v0.9/subscription-plans/:planId
func (h *SubscriptionPlanHandler) UpdateSubscriptionPlan(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	planId := r.PathValue("subscriptionPlanId")
	if planId == "" {
		return apperror.ValidationFailed.New("Plan ID is required")
	}

	var req api.SubscriptionPlan
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid update subscription plan request body for plan %s in org %s", planId, orgId))
	}

	if req.Id != nil && *req.Id != "" && *req.Id != planId {
		return apperror.ValidationFailed.New("The plan id is immutable and cannot be changed")
	}

	displayName := req.DisplayName
	if displayName == "" {
		return apperror.ValidationFailed.New("displayName is required")
	}
	update := &model.SubscriptionPlanUpdate{
		Name:       &displayName,
		ExpiryTime: req.ExpiryTime,
	}
	if req.Limits != nil {
		if limit := firstLimit(apiLimitsToRequests(*req.Limits)); limit != nil {
			if errMsg := normalizeAndValidateLimit(limit); errMsg != "" {
				return apperror.ValidationFailed.New(errMsg)
			}
			count := limit.LimitCount
			update.ThrottleLimitCount = &count
			update.ThrottleLimitUnit = &limit.TimeUnit
			stopOnQuotaReach := true
			if limit.StopOnQuotaReach != nil {
				stopOnQuotaReach = *limit.StopOnQuotaReach
			}
			update.StopOnQuotaReach = &stopOnQuotaReach
		} else {
			update.ClearLimit = true
		}
	}
	if req.Status != nil {
		switch model.SubscriptionPlanStatus(*req.Status) {
		case model.SubscriptionPlanStatusActive, model.SubscriptionPlanStatusInactive:
			st := model.SubscriptionPlanStatus(*req.Status)
			update.Status = &st
		default:
			return apperror.ValidationFailed.New("Invalid status value; must be ACTIVE or INACTIVE")
		}
	}

	rawActor, ok := middleware.GetActorIdentityFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("user id claim not found in token")
	}
	actor, err := h.identity.ToInternalUUID(rawActor)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage("failed to resolve user identity")
	}
	updated, err := h.planService.UpdatePlan(planId, orgId, actor, update)
	if err != nil {
		if errors.Is(err, constants.ErrHandleImmutable) {
			return apperror.ValidationFailed.Wrap(err, "The plan id is immutable and cannot be changed")
		}
		if errors.Is(err, constants.ErrSubscriptionPlanNotFound) {
			return apperror.SubscriptionPlanNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrSubscriptionPlanAlreadyExists) {
			return apperror.SubscriptionPlanExists.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to update subscription plan %s in org %s", planId, orgId))
	}
	resp, err := h.toSubscriptionPlanResponse(updated, true)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to resolve subscription plan identity for plan %s in org %s", planId, orgId))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// DeleteSubscriptionPlan handles DELETE /api/v0.9/subscription-plans/:planId
func (h *SubscriptionPlanHandler) DeleteSubscriptionPlan(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	planId := r.PathValue("subscriptionPlanId")
	if planId == "" {
		return apperror.ValidationFailed.New("Plan ID is required")
	}

	rawActor, ok := middleware.GetActorIdentityFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("user id claim not found in token")
	}
	actor, err := h.identity.ToInternalUUID(rawActor)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage("failed to resolve user identity")
	}
	err = h.planService.DeletePlan(planId, orgId, actor)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionPlanNotFound) {
			return apperror.SubscriptionPlanNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to delete subscription plan %s in org %s", planId, orgId))
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// RegisterRoutes registers subscription plan routes
func (h *SubscriptionPlanHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/subscription-plans", middleware.MapErrors(h.slogger, h.CreateSubscriptionPlan))
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscription-plans", middleware.MapErrors(h.slogger, h.ListSubscriptionPlans))
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscription-plans/{subscriptionPlanId}", middleware.MapErrors(h.slogger, h.GetSubscriptionPlan))
	mux.HandleFunc("PUT "+constants.APIBasePath+"/subscription-plans/{subscriptionPlanId}", middleware.MapErrors(h.slogger, h.UpdateSubscriptionPlan))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/subscription-plans/{subscriptionPlanId}", middleware.MapErrors(h.slogger, h.DeleteSubscriptionPlan))
}

// toSubscriptionPlanResponse builds the API response for a plan.
// updatedBy is only included when detail is true (GET/POST/PUT single-plan responses),
// matching the platform-wide policy of omitting it from list responses.
//
// NOTE: SINGLE-LIMIT ASSUMPTION. The "limits" array holds at most one entry today
// even though subscription_plan_limits supports many; see model.SubscriptionPlan.
func (h *SubscriptionPlanHandler) toSubscriptionPlanResponse(plan *model.SubscriptionPlan, detail bool) (map[string]any, error) {
	createdBy, err := h.identity.SubForUUID(plan.CreatedBy)
	if err != nil {
		return nil, err
	}
	resp := map[string]any{
		"id":             plan.Handle,
		"displayName":    plan.Name,
		"organizationId": h.planService.ResolveOrgHandle(plan.OrganizationUUID),
		"status":         string(plan.Status),
		"createdBy":      createdBy,
		"createdAt":      plan.CreatedAt,
		"updatedAt":      plan.UpdatedAt,
	}
	if detail {
		updatedBy, err := h.identity.SubForUUID(plan.UpdatedBy)
		if err != nil {
			return nil, err
		}
		resp["updatedBy"] = updatedBy
	}
	limits := []map[string]any{}
	if plan.ThrottleLimitCount != nil {
		limits = append(limits, map[string]any{
			"limitType":        constants.LimitTypeRequestCount,
			"timeUnit":         plan.ThrottleLimitUnit,
			"timeAmount":       1,
			"limitCount":       *plan.ThrottleLimitCount,
			"stopOnQuotaReach": plan.StopOnQuotaReach,
		})
	}
	resp["limits"] = limits
	if plan.ExpiryTime != nil {
		resp["expiryTime"] = plan.ExpiryTime
	}
	return resp, nil
}

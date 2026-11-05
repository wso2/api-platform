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
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	api "github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/api-platform/httpkit/httputil"
)

// maxPlanBodyBytes caps a plan create or update body: room for the most conditions a plan can have
// and its pricing.
const maxPlanBodyBytes = 128 << 10

// SubscriptionPlanHandler handles subscription plan CRUD
type SubscriptionPlanHandler struct {
	planService *service.SubscriptionPlanService
	identity    *service.IdentityService
	slogger     *slog.Logger
	// pricingRepo reads plan pricing for the read-only `pricing` of a paid plan. Nil leaves it out.
	pricingRepo repository.SubscriptionPlanPricingRepository
}

// SetPricingRepository makes plan responses carry the plan's pricing.
func (h *SubscriptionPlanHandler) SetPricingRepository(repo repository.SubscriptionPlanPricingRepository) {
	h.pricingRepo = repo
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

// SubscriptionPlanLimitRequest is one entry of the old "limits" field. Each becomes a window of the
// plan's request-count condition.
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

// limitsToRequestCondition turns old "limits" into a request-count condition. As before, only the first
// limit is validated; later limits that are invalid or repeat a window length are skipped. Blocking
// over the limit is taken from the first limit.
func limitsToRequestCondition(limits []SubscriptionPlanLimitRequest) (*model.SubscriptionPlanCondition, string) {
	if len(limits) == 0 {
		return nil, ""
	}
	if errMsg := normalizeAndValidateLimit(&limits[0]); errMsg != "" {
		return nil, errMsg
	}
	windows := make([]model.ConditionWindow, 0, len(limits))
	seen := map[string]bool{}
	for i := range limits {
		if i > 0 && normalizeAndValidateLimit(&limits[i]) != "" {
			continue
		}
		key := windowKey(limits[i].TimeAmount, limits[i].TimeUnit)
		if seen[key] || len(windows) == 10 {
			continue
		}
		seen[key] = true
		windows = append(windows, model.ConditionWindow{
			Count: limits[i].LimitCount, TimeAmount: limits[i].TimeAmount, TimeUnit: limits[i].TimeUnit,
		})
	}
	stopOnQuotaReach := limits[0].StopOnQuotaReach == nil || *limits[0].StopOnQuotaReach
	c := model.NewRequestCountCondition(windows, stopOnQuotaReach)
	return &c, ""
}

// windowKey names a window by its length, so 60 MINUTE and 1 HOUR match. Months match only months.
func windowKey(amount int, unit string) string {
	switch unit {
	case "MINUTE":
		return fmt.Sprintf("m%d", amount)
	case "HOUR":
		return fmt.Sprintf("m%d", amount*60)
	case "DAY":
		return fmt.Sprintf("m%d", amount*24*60)
	}
	return fmt.Sprintf("M%d", amount)
}

// decodeConditions reads the "conditions" field. null or a non-array is refused.
func decodeConditions(raw json.RawMessage) ([]model.SubscriptionPlanCondition, error) {
	var conditions []model.SubscriptionPlanCondition
	if string(raw) == "null" {
		return nil, apperror.ValidationFailed.New("conditions must be an array; send [] for a plan with no conditions")
	}
	if err := json.Unmarshal(raw, &conditions); err != nil {
		return nil, apperror.ValidationFailed.Wrap(err, "conditions must be an array of {id, type, params}")
	}
	return conditions, nil
}

// CreateSubscriptionPlanRequest is the body for POST /api/v0.9/subscription-plans. Send Conditions, or
// Limits from older clients. Pricing makes the plan paid.
type CreateSubscriptionPlanRequest struct {
	Id          string                         `json:"id" binding:"required"`
	DisplayName string                         `json:"displayName" binding:"required"`
	Description string                         `json:"description,omitempty"`
	Conditions  json.RawMessage                `json:"conditions,omitempty"`
	Limits      []SubscriptionPlanLimitRequest `json:"limits,omitempty"`
	ExpiryTime  *string                        `json:"expiryTime,omitempty"`
	Status      string                         `json:"status,omitempty"`
	Pricing     json.RawMessage                `json:"pricing,omitempty"`
}

// CreateSubscriptionPlan handles POST /api/v0.9/subscription-plans
func (h *SubscriptionPlanHandler) CreateSubscriptionPlan(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPlanBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
		}
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("failed to read create subscription plan request body for org %s", orgId))
	}
	var req CreateSubscriptionPlanRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid create subscription plan request body for org %s", orgId))
	}
	// A null pricing is a free plan, as is none.
	var pricing []byte
	if len(req.Pricing) > 0 && string(req.Pricing) != "null" {
		pricing = req.Pricing
	}

	if req.Id == "" {
		return apperror.ValidationFailed.New("id is required")
	}
	if utf8.RuneCountInString(req.Id) > model.MaxSubscriptionPlanHandleChars {
		return apperror.ValidationFailed.New(fmt.Sprintf("id must be at most %d characters", model.MaxSubscriptionPlanHandleChars))
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
		Handle:      req.Id,
		Name:        req.DisplayName,
		Description: strings.TrimSpace(req.Description),
		Status:      model.SubscriptionPlanStatus(req.Status),
		Conditions:  []model.SubscriptionPlanCondition{},
	}
	if len(req.Conditions) > 0 {
		if len(req.Limits) > 0 {
			return apperror.ValidationFailed.New("send conditions or limits, not both")
		}
		conditions, err := decodeConditions(req.Conditions)
		if err != nil {
			return err
		}
		plan.Conditions = conditions
	} else {
		requestLimit, errMsg := limitsToRequestCondition(req.Limits)
		if errMsg != "" {
			return apperror.ValidationFailed.New(errMsg)
		}
		if requestLimit != nil {
			plan.Conditions = []model.SubscriptionPlanCondition{*requestLimit}
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
	created, err := h.planService.CreatePlan(orgId, actor, plan, pricing)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to create subscription plan for org %s", orgId))
	}
	resp, err := h.toSubscriptionPlanResponse(created, true)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to resolve subscription plan identity for org %s", orgId))
	}
	setLocation(w, "subscription-plans", created.Handle)
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

	opts := parseListOptions(r)
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && status != string(model.SubscriptionPlanStatusActive) && status != string(model.SubscriptionPlanStatusInactive) {
		return apperror.ValidationFailed.New("status must be ACTIVE or INACTIVE")
	}

	total, err := h.planService.CountPlans(orgId, opts.Search, status)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to count subscription plans for org %s", orgId))
	}

	list, err := h.planService.ListPlans(orgId, opts, status)
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
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"list":  items,
		"count": len(items),
		"pagination": api.Pagination{
			Total:  total,
			Offset: opts.Offset,
			Limit:  opts.Limit,
		},
	})
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
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to get subscription plan %s in org %s", planId, orgId))
	}
	resp, err := h.toSubscriptionPlanResponse(plan, true)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to resolve subscription plan identity for plan %s in org %s", planId, orgId))
	}
	// What offers or uses the plan, so a client can show a delete, or a change of name or conditions, as
	// refused before it is tried. Only a single plan's read carries it: working it out reads every
	// REST API of the organization.
	usage, err := h.planService.GetUsage(plan)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to read the usage of subscription plan %s in org %s", planId, orgId))
	}
	resp["usage"] = map[string]any{
		"inUse":         usage.InUse(),
		"restApis":      usage.RESTAPIs,
		"drafts":        usage.Drafts,
		"publications":  usage.Publications,
		"subscriptions": usage.Subscriptions,
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

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPlanBodyBytes))
	if err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("failed to read update subscription plan request body for plan %s in org %s", planId, orgId))
	}
	// Two fields are read from the raw body, where the typed request would lose the difference:
	//   - expiryTime: an explicit null clears the expiry, while an omitted one leaves it unchanged
	//     (*time.Time cannot tell the two apart);
	//   - pricing: refused even when null, since pricing is set only when a plan is created.
	//   - conditions and description: read as sent; an omitted one leaves the plan's unchanged.
	var presence struct {
		ExpiryTime  json.RawMessage `json:"expiryTime"`
		Pricing     json.RawMessage `json:"pricing"`
		Conditions  json.RawMessage `json:"conditions"`
		Description *string         `json:"description"`
	}
	if err := json.Unmarshal(body, &presence); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid update subscription plan request body for plan %s in org %s", planId, orgId))
	}
	if presence.Pricing != nil {
		return apperror.SubscriptionPlanPricingLocked.New()
	}
	var req api.SubscriptionPlan
	if err := json.Unmarshal(body, &req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid update subscription plan request body for plan %s in org %s", planId, orgId))
	}

	if err := utils.ValidateHandleImmutable(planId, req.Id); err != nil {
		return apperror.ValidationFailed.Wrap(err, "The plan id is immutable and cannot be changed")
	}

	displayName := req.DisplayName
	if displayName == "" {
		return apperror.ValidationFailed.New("displayName is required")
	}
	update := &model.SubscriptionPlanUpdate{
		Name:            &displayName,
		Description:     presence.Description,
		ExpiryTime:      req.ExpiryTime,
		ClearExpiryTime: req.ExpiryTime == nil && string(presence.ExpiryTime) == "null",
	}
	if presence.Conditions != nil {
		if req.Limits != nil && len(*req.Limits) > 0 {
			return apperror.ValidationFailed.New("send conditions or limits, not both")
		}
		conditions, err := decodeConditions(presence.Conditions)
		if err != nil {
			return err
		}
		update.Conditions = &conditions
	} else if req.Limits != nil {
		requestLimit, errMsg := limitsToRequestCondition(apiLimitsToRequests(*req.Limits))
		if errMsg != "" {
			return apperror.ValidationFailed.New(errMsg)
		}
		if requestLimit != nil {
			update.RequestLimit = requestLimit
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
		return serviceError(err, fmt.Sprintf("failed to update subscription plan %s in org %s", planId, orgId))
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
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to delete subscription plan %s in org %s", planId, orgId))
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// RegisterRoutes registers subscription plan routes
func (h *SubscriptionPlanHandler) RegisterRoutes(mux router.Router) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/subscription-plans", middleware.MapErrors(h.slogger, h.CreateSubscriptionPlan))
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscription-plans", middleware.MapErrors(h.slogger, h.ListSubscriptionPlans))
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscription-plans/{subscriptionPlanId}", middleware.MapErrors(h.slogger, h.GetSubscriptionPlan))
	mux.HandleFunc("PUT "+constants.APIBasePath+"/subscription-plans/{subscriptionPlanId}", middleware.MapErrors(h.slogger, h.UpdateSubscriptionPlan))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/subscription-plans/{subscriptionPlanId}", middleware.MapErrors(h.slogger, h.DeleteSubscriptionPlan))
}

// toSubscriptionPlanResponse builds the API response for a plan. updatedBy is only included when detail
// is true, as list responses omit it. "limits" is kept for older clients (requestLimits).
func (h *SubscriptionPlanHandler) toSubscriptionPlanResponse(plan *model.SubscriptionPlan, detail bool) (map[string]any, error) {
	createdBy, err := h.identity.SubForUUID(plan.CreatedBy)
	if err != nil {
		return nil, err
	}
	conditions := plan.Conditions
	if conditions == nil {
		conditions = []model.SubscriptionPlanCondition{}
	}
	resp := map[string]any{
		"id":             plan.Handle,
		"displayName":    plan.Name,
		"description":    plan.Description,
		"conditions":     conditions,
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
	resp["limits"] = requestLimits(plan)
	if plan.ExpiryTime != nil {
		resp["expiryTime"] = plan.ExpiryTime
	}
	if h.pricingRepo != nil {
		// A read failure leaves pricing out rather than failing the plan read.
		pricing, err := h.pricingRepo.GetByPlan(plan.UUID, plan.OrganizationUUID)
		if err != nil {
			h.slogger.Warn("Failed to read subscription plan pricing", "plan", plan.Handle, "error", err)
		} else if pricing != nil {
			resp["pricing"] = pricingResponse(plan.Handle, pricing)
		}
	}
	return resp, nil
}

// requestLimits returns the request-count windows in the old "limits" shape.
func requestLimits(plan *model.SubscriptionPlan) []map[string]any {
	limits := []map[string]any{}
	c := model.FindCondition(plan.Conditions, model.ConditionTypeRequestCount)
	if c == nil {
		return limits
	}
	var params model.ConditionLimits
	if err := json.Unmarshal(c.Params, &params); err != nil {
		return limits
	}
	for _, w := range params.Limits {
		limits = append(limits, map[string]any{
			"limitType":        constants.LimitTypeRequestCount,
			"timeUnit":         w.TimeUnit,
			"timeAmount":       w.TimeAmount,
			"limitCount":       w.Count,
			"stopOnQuotaReach": params.OnExceed != model.ConditionOnExceedAllow,
		})
	}
	return limits
}

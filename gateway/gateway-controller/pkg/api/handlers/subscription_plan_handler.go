/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	auditservice "github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/audit"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// validateThrottleLimits ensures throttleLimitCount and throttleLimitUnit are provided together,
// count is positive, and unit is one of Day, Hour, Min, Month.
func validateThrottleLimits(count *int, unit *string) error {
	countProvided := count != nil
	unitProvided := unit != nil && *unit != ""
	if countProvided != unitProvided {
		return fmt.Errorf("throttleLimitCount and throttleLimitUnit must be provided together")
	}
	if !countProvided {
		return nil
	}
	if *count <= 0 {
		return fmt.Errorf("throttleLimitCount must be positive")
	}
	switch *unit {
	case "Day", "Hour", "Min", "Month":
		return nil
	default:
		return fmt.Errorf("throttleLimitUnit must be one of: Day, Hour, Min, Month")
	}
}

// CreateSubscriptionPlan implements ServerInterface.CreateSubscriptionPlan (POST /subscription-plans)
func (s *APIServer) CreateSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSubscriptionRequestBodyBytes)
	var req api.SubscriptionPlanCreateRequest
	if err := s.bindRequestBody(r, &req); err != nil {
		log.Warn("Invalid subscription plan create body", slog.Any("error", err))
		writeSubscriptionBodyError(w, err)
		return
	}
	planName := strings.TrimSpace(req.PlanName)
	if planName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "planName is required"})
		return
	}

	var unitStr *string
	if req.ThrottleLimitUnit != nil {
		s := string(*req.ThrottleLimitUnit)
		unitStr = &s
	}
	if err := validateThrottleLimits(req.ThrottleLimitCount, unitStr); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	status := models.SubscriptionPlanStatusActive
	if req.Status != nil {
		st := models.SubscriptionPlanStatus(*req.Status)
		switch st {
		case models.SubscriptionPlanStatusActive, models.SubscriptionPlanStatusInactive:
			status = st
		default:
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("invalid status: %s", *req.Status)})
			return
		}
	}

	plan := &models.SubscriptionPlan{
		ID:               uuid.New().String(),
		PlanName:         planName,
		StopOnQuotaReach: true,
		Status:           status,
	}
	if req.BillingPlan != nil {
		plan.BillingPlan = req.BillingPlan
	}
	if req.StopOnQuotaReach != nil {
		plan.StopOnQuotaReach = *req.StopOnQuotaReach
	}
	if req.ThrottleLimitCount != nil && req.ThrottleLimitUnit != nil {
		s := string(*req.ThrottleLimitUnit)
		plan.ThrottleLimitCount = req.ThrottleLimitCount
		plan.ThrottleLimitUnit = &s
	}
	if req.ExpiryTime != nil {
		plan.ExpiryTime = req.ExpiryTime
	}

	if err := s.getSubscriptionResourceService().SaveSubscriptionPlan(plan, correlationID, log); err != nil {
		if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: "Subscription plan already exists"})
			return
		}
		log.Error("Failed to save subscription plan", slog.Any("error", err))
		s.auditService.Record(r.Context(), &auditservice.AuditEvent{
			Action:       models.AuditActionCreate,
			ResourceType: models.AuditResourceTypeSubscriptionPlan,
			Outcome:      models.AuditOutcomeFailed,
			PerformedBy:  actorFromRequest(r),
			ResourceUUID: plan.ID,
			ResourceName: plan.PlanName,
		})
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create subscription plan"})
		return
	}
	s.auditService.Record(r.Context(), &auditservice.AuditEvent{
		Action:       models.AuditActionCreate,
		ResourceType: models.AuditResourceTypeSubscriptionPlan,
		Outcome:      models.AuditOutcomeSucceeded,
		PerformedBy:  actorFromRequest(r),
		ResourceUUID: plan.ID,
		ResourceName: plan.PlanName,
		EventData:    planAuditFields(plan),
	})
	httputil.WriteJSON(w, http.StatusCreated, subscriptionPlanToResponse(plan))
}

// ListSubscriptionPlans implements ServerInterface.ListSubscriptionPlans (GET /subscription-plans)
func (s *APIServer) ListSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)

	list, err := s.db.ListSubscriptionPlans("")
	if err != nil {
		log.Error("Failed to list subscription plans", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list subscription plans"})
		return
	}
	items := make([]api.SubscriptionPlanResponse, 0, len(list))
	for _, p := range list {
		items = append(items, subscriptionPlanToResponse(p))
	}
	count := len(items)
	httputil.WriteJSON(w, http.StatusOK, api.SubscriptionPlanListResponse{SubscriptionPlans: &items, Count: &count})
}

// GetSubscriptionPlan implements ServerInterface.GetSubscriptionPlan (GET /subscription-plans/{planId})
func (s *APIServer) GetSubscriptionPlan(w http.ResponseWriter, r *http.Request, planId string) {
	log := middleware.GetLogger(r, s.logger)

	plan, err := s.db.GetSubscriptionPlanByID(planId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
			return
		}
		log.Error("Failed to get subscription plan", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription plan"})
		return
	}
	if plan == nil {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, subscriptionPlanToResponse(plan))
}

// UpdateSubscriptionPlan implements ServerInterface.UpdateSubscriptionPlan (PUT /subscription-plans/{planId})
func (s *APIServer) UpdateSubscriptionPlan(w http.ResponseWriter, r *http.Request, planId string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	existing, err := s.db.GetSubscriptionPlanByID(planId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
			return
		}
		log.Error("Failed to get subscription plan for update", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription plan"})
		return
	}
	if existing == nil {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSubscriptionRequestBodyBytes)
	before := planAuditFields(existing)
	var req api.SubscriptionPlanUpdateRequest
	fields, err := s.bindRequestBodyWithFields(r, &req)
	if err != nil {
		log.Warn("Invalid subscription plan update body", slog.Any("error", err))
		writeSubscriptionBodyError(w, err)
		return
	}

	// Apply the changes to a copy, so a rejected request leaves the stored plan untouched.
	updated := *existing

	if req.PlanName != nil {
		trimmed := strings.TrimSpace(*req.PlanName)
		if trimmed == "" {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "planName cannot be empty"})
			return
		}
		updated.PlanName = trimmed
	}

	// Omitted fields stay unchanged; an explicit null (or empty string) removes billingPlan.
	if _, sent := fields["billingPlan"]; sent {
		if req.BillingPlan == nil || strings.TrimSpace(*req.BillingPlan) == "" {
			updated.BillingPlan = nil
		} else {
			billingPlan := *req.BillingPlan
			updated.BillingPlan = &billingPlan
		}
	}

	if req.StopOnQuotaReach != nil {
		updated.StopOnQuotaReach = *req.StopOnQuotaReach
	}

	// The throttle limit is a pair: both fields are set together, or both sent as null to make the
	// plan unlimited.
	_, countSent := fields["throttleLimitCount"]
	_, unitSent := fields["throttleLimitUnit"]
	if countSent || unitSent {
		countCleared := countSent && req.ThrottleLimitCount == nil
		unitCleared := unitSent && (req.ThrottleLimitUnit == nil || *req.ThrottleLimitUnit == "")
		switch {
		case countCleared && unitCleared:
			updated.ThrottleLimitCount = nil
			updated.ThrottleLimitUnit = nil
		case countCleared || unitCleared:
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: "throttleLimitCount and throttleLimitUnit must be set or cleared together",
			})
			return
		default:
			var unitStr *string
			if req.ThrottleLimitUnit != nil {
				unit := string(*req.ThrottleLimitUnit)
				unitStr = &unit
			}
			if err := validateThrottleLimits(req.ThrottleLimitCount, unitStr); err != nil {
				httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
				return
			}
			count := *req.ThrottleLimitCount
			updated.ThrottleLimitCount = &count
			updated.ThrottleLimitUnit = unitStr
		}
	}

	// Omitted expiryTime stays unchanged; an explicit null removes it.
	if _, sent := fields["expiryTime"]; sent {
		updated.ExpiryTime = req.ExpiryTime
	}

	if req.Status != nil {
		st := models.SubscriptionPlanStatus(*req.Status)
		switch st {
		case models.SubscriptionPlanStatusActive, models.SubscriptionPlanStatusInactive:
			updated.Status = st
		default:
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("invalid status: %s", *req.Status)})
			return
		}
	}

	if err := s.getSubscriptionResourceService().UpdateSubscriptionPlan(&updated, correlationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
			return
		}
		if storage.IsConflictError(err) {
			log.Warn("Subscription plan update rejected due to a conflict", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: "A subscription plan with this name already exists"})
			return
		}
		log.Error("Failed to update subscription plan", slog.Any("error", err))
		s.auditService.Record(r.Context(), &auditservice.AuditEvent{
			Action:       models.AuditActionUpdate,
			ResourceType: models.AuditResourceTypeSubscriptionPlan,
			Outcome:      models.AuditOutcomeFailed,
			PerformedBy:  actorFromRequest(r),
			ResourceUUID: existing.ID,
			ResourceName: existing.PlanName,
		})
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update subscription plan"})
		return
	}
	s.auditService.Record(r.Context(), &auditservice.AuditEvent{
		Action:       models.AuditActionUpdate,
		ResourceType: models.AuditResourceTypeSubscriptionPlan,
		Outcome:      models.AuditOutcomeSucceeded,
		PerformedBy:  actorFromRequest(r),
		ResourceUUID: updated.ID,
		ResourceName: updated.PlanName,
		EventData:    auditservice.AuditDiff(before, planAuditFields(&updated)),
	})
	httputil.WriteJSON(w, http.StatusOK, subscriptionPlanToResponse(&updated))
}

// DeleteSubscriptionPlan implements ServerInterface.DeleteSubscriptionPlan (DELETE /subscription-plans/{planId})
func (s *APIServer) DeleteSubscriptionPlan(w http.ResponseWriter, r *http.Request, planId string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	if err := s.getSubscriptionResourceService().DeleteSubscriptionPlan(planId, correlationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
			return
		}
		if storage.IsConflictError(err) {
			log.Warn("Subscription plan delete rejected while in use", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: "Subscription plan is in use by one or more subscriptions"})
			return
		}
		log.Error("Failed to delete subscription plan", slog.Any("error", err))
		s.auditService.Record(r.Context(), &auditservice.AuditEvent{
			Action:       models.AuditActionDelete,
			ResourceType: models.AuditResourceTypeSubscriptionPlan,
			Outcome:      models.AuditOutcomeFailed,
			PerformedBy:  actorFromRequest(r),
			ResourceUUID: planId,
		})
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to delete subscription plan"})
		return
	}
	s.auditService.Record(r.Context(), &auditservice.AuditEvent{
		Action:       models.AuditActionDelete,
		ResourceType: models.AuditResourceTypeSubscriptionPlan,
		Outcome:      models.AuditOutcomeSucceeded,
		PerformedBy:  actorFromRequest(r),
		ResourceUUID: planId,
	})
	w.WriteHeader(http.StatusNoContent)
}

// planAuditFields is the audit event data for a subscription plan. Unset throttle
// limits are recorded as nil so every snapshot has the same keys.
func planAuditFields(p *models.SubscriptionPlan) map[string]any {
	fields := map[string]any{
		"name":               p.PlanName,
		"status":             string(p.Status),
		"stopOnQuotaReach":   p.StopOnQuotaReach,
		"throttleLimitCount": nil,
		"throttleLimitUnit":  nil,
		"billingPlan":        nil,
		"expiryTime":         nil,
	}
	if p.ThrottleLimitCount != nil {
		fields["throttleLimitCount"] = *p.ThrottleLimitCount
	}
	if p.ThrottleLimitUnit != nil {
		fields["throttleLimitUnit"] = *p.ThrottleLimitUnit
	}
	if p.BillingPlan != nil {
		fields["billingPlan"] = *p.BillingPlan
	}
	if p.ExpiryTime != nil {
		fields["expiryTime"] = p.ExpiryTime.UTC().Format(time.RFC3339)
	}
	return fields
}

func subscriptionPlanToResponse(plan *models.SubscriptionPlan) api.SubscriptionPlanResponse {
	resp := api.SubscriptionPlanResponse{
		Id:               ptr(plan.ID),
		PlanName:         ptr(plan.PlanName),
		GatewayId:        ptr(plan.GatewayID),
		StopOnQuotaReach: ptr(plan.StopOnQuotaReach),
		CreatedAt:        &plan.CreatedAt,
		UpdatedAt:        &plan.UpdatedAt,
	}
	if plan.BillingPlan != nil && *plan.BillingPlan != "" {
		resp.BillingPlan = plan.BillingPlan
	}
	if plan.ThrottleLimitCount != nil {
		resp.ThrottleLimitCount = plan.ThrottleLimitCount
	}
	if plan.ThrottleLimitUnit != nil && *plan.ThrottleLimitUnit != "" {
		resp.ThrottleLimitUnit = plan.ThrottleLimitUnit
	}
	if plan.ExpiryTime != nil {
		resp.ExpiryTime = plan.ExpiryTime
	}
	if plan.Status != "" {
		st := api.SubscriptionPlanResponseStatus(plan.Status)
		resp.Status = &st
	}
	return resp
}

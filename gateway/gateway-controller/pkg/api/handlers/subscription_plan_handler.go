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
	"errors"
	"log/slog"
	"net/http"

	"github.com/wso2/api-platform/httpkit/httputil"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/subscription"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// CreateSubscriptionPlan implements ServerInterface.CreateSubscriptionPlan (POST /subscription-plans)
func (s *APIServer) CreateSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	var req api.SubscriptionPlanCreateRequest
	if err := s.bindRequestBody(r, &req); err != nil {
		log.Warn("Invalid subscription plan create body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
		return
	}

	result, err := s.getSubscriptionService().CreatePlan(subscription.CreatePlanParams{
		Request:       req,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		mapPlanCreateError(w, log, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, subscriptionPlanToResponse(result.Plan))
}

// ListSubscriptionPlans implements ServerInterface.ListSubscriptionPlans (GET /subscription-plans)
func (s *APIServer) ListSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)

	result, err := s.getSubscriptionService().ListPlans()
	if err != nil {
		log.Error("Failed to list subscription plans", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list subscription plans"})
		return
	}

	items := make([]api.SubscriptionPlanResponse, 0, len(result.Plans))
	for _, plan := range result.Plans {
		items = append(items, subscriptionPlanToResponse(plan))
	}
	count := len(items)
	httputil.WriteJSON(w, http.StatusOK, api.SubscriptionPlanListResponse{SubscriptionPlans: &items, Count: &count})
}

// GetSubscriptionPlan implements ServerInterface.GetSubscriptionPlan (GET /subscription-plans/{planId})
func (s *APIServer) GetSubscriptionPlan(w http.ResponseWriter, r *http.Request, planId string) {
	log := middleware.GetLogger(r, s.logger)

	result, err := s.getSubscriptionService().GetPlan(planId)
	if err != nil {
		mapPlanGetError(w, log, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, subscriptionPlanToResponse(result.Plan))
}

// UpdateSubscriptionPlan implements ServerInterface.UpdateSubscriptionPlan (PUT /subscription-plans/{planId})
func (s *APIServer) UpdateSubscriptionPlan(w http.ResponseWriter, r *http.Request, planId string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	// Existence is checked before the body is bound so an update against an
	// unknown plan still answers 404 rather than 400 when the body is also
	// malformed, exactly as this handler did before the service layer existed.
	// UpdatePlan re-reads the row; that second primary-key lookup is the price
	// of keeping the status codes identical.
	if _, err := s.getSubscriptionService().GetPlan(planId); err != nil {
		mapPlanGetError(w, log, err)
		return
	}

	var req api.SubscriptionPlanUpdateRequest
	if err := s.bindRequestBody(r, &req); err != nil {
		log.Warn("Invalid subscription plan update body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
		return
	}

	result, err := s.getSubscriptionService().UpdatePlan(subscription.UpdatePlanParams{
		ID:            planId,
		Request:       req,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		mapPlanUpdateError(w, log, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, subscriptionPlanToResponse(result.Plan))
}

// DeleteSubscriptionPlan implements ServerInterface.DeleteSubscriptionPlan (DELETE /subscription-plans/{planId})
func (s *APIServer) DeleteSubscriptionPlan(w http.ResponseWriter, r *http.Request, planId string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	if err := s.getSubscriptionService().DeletePlan(planId, correlationID, log); err != nil {
		if errors.Is(err, subscription.ErrPlanNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
			return
		}
		log.Error("Failed to delete subscription plan", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to delete subscription plan"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// mapPlanCreateError reproduces the responses POST /subscription-plans returned
// before the service layer existed.
func mapPlanCreateError(w http.ResponseWriter, log *slog.Logger, err error) {
	var validationErr *subscription.ValidationError
	if errors.As(err, &validationErr) {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: validationErr.Message})
		return
	}
	if storage.IsConflictError(err) {
		httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: "Subscription plan already exists"})
		return
	}

	log.Error("Failed to save subscription plan", slog.Any("error", err))
	httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create subscription plan"})
}

// mapPlanGetError reproduces the responses a subscription-plan read returned
// before the service layer existed. It also serves the existence pre-check in
// UpdateSubscriptionPlan, which reported the same pair of failures.
func mapPlanGetError(w http.ResponseWriter, log *slog.Logger, err error) {
	if errors.Is(err, subscription.ErrPlanNotFound) {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
		return
	}

	log.Error("Failed to get subscription plan", slog.Any("error", err))
	httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription plan"})
}

// mapPlanUpdateError reproduces the responses PUT /subscription-plans/{planId}
// returned before the service layer existed. A failure to re-read the row is
// reported as a read failure, not an update failure, which is the distinction
// the original handler drew.
func mapPlanUpdateError(w http.ResponseWriter, log *slog.Logger, err error) {
	var validationErr *subscription.ValidationError
	if errors.As(err, &validationErr) {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: validationErr.Message})
		return
	}
	if errors.Is(err, subscription.ErrPlanNotFound) {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
		return
	}

	var opErr *subscription.OpError
	if errors.As(err, &opErr) && opErr.Op == subscription.OpLoad {
		log.Error("Failed to get subscription plan for update", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription plan"})
		return
	}

	log.Error("Failed to update subscription plan", slog.Any("error", err))
	httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update subscription plan"})
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

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

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/subscription"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/httpkit/httputil"
)

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

	result, err := s.getSubscriptionService().CreatePlan(subscription.CreatePlanParams{
		Request:       req,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		var validationErr *subscription.ValidationError
		switch {
		case errors.As(err, &validationErr):
			writeSubscriptionError(w, http.StatusBadRequest, validationErr.Message)
		case storage.IsConflictError(err):
			writeSubscriptionError(w, http.StatusConflict, "Subscription plan already exists")
		default:
			log.Error("Failed to save subscription plan", slog.Any("error", storageCause(err)))
			writeSubscriptionError(w, http.StatusInternalServerError, "Failed to create subscription plan")
		}
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, subscriptionPlanToResponse(result.Plan))
}

// ListSubscriptionPlans implements ServerInterface.ListSubscriptionPlans (GET /subscription-plans)
func (s *APIServer) ListSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)

	result, err := s.getSubscriptionService().ListPlans()
	if err != nil {
		log.Error("Failed to list subscription plans", slog.Any("error", storageCause(err)))
		writeSubscriptionError(w, http.StatusInternalServerError, "Failed to list subscription plans")
		return
	}
	items := make([]api.SubscriptionPlanResponse, 0, len(result.Plans))
	for _, p := range result.Plans {
		items = append(items, subscriptionPlanToResponse(p))
	}
	count := len(items)
	httputil.WriteJSON(w, http.StatusOK, api.SubscriptionPlanListResponse{SubscriptionPlans: &items, Count: &count})
}

// GetSubscriptionPlan implements ServerInterface.GetSubscriptionPlan (GET /subscription-plans/{planId})
func (s *APIServer) GetSubscriptionPlan(w http.ResponseWriter, r *http.Request, planId string) {
	log := middleware.GetLogger(r, s.logger)

	result, err := s.getSubscriptionService().GetPlan(planId)
	if err != nil {
		writePlanLoadError(w, log, err, "Failed to get subscription plan")
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

	svc := s.getSubscriptionService()
	existing, err := svc.GetPlan(planId)
	if err != nil {
		writePlanLoadError(w, log, err, "Failed to get subscription plan for update")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSubscriptionRequestBodyBytes)
	var req api.SubscriptionPlanUpdateRequest
	fields, err := s.bindRequestBodyWithFields(r, &req)
	if err != nil {
		log.Warn("Invalid subscription plan update body", slog.Any("error", err))
		writeSubscriptionBodyError(w, err)
		return
	}

	result, err := svc.UpdatePlan(subscription.UpdatePlanParams{
		Existing:      existing.Plan,
		Request:       req,
		Fields:        fields,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		var validationErr *subscription.ValidationError
		switch {
		case errors.Is(err, subscription.ErrPlanNotFound):
			writeSubscriptionError(w, http.StatusNotFound, "Subscription plan not found")
		case errors.As(err, &validationErr):
			writeSubscriptionError(w, http.StatusBadRequest, validationErr.Message)
		case storage.IsConflictError(err):
			log.Warn("Subscription plan update rejected due to a conflict", slog.Any("error", storageCause(err)))
			writeSubscriptionError(w, http.StatusConflict, "A subscription plan with this name already exists")
		default:
			log.Error("Failed to update subscription plan", slog.Any("error", storageCause(err)))
			writeSubscriptionError(w, http.StatusInternalServerError, "Failed to update subscription plan")
		}
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
		switch {
		case errors.Is(err, subscription.ErrPlanNotFound):
			writeSubscriptionError(w, http.StatusNotFound, "Subscription plan not found")
		case storage.IsConflictError(err):
			log.Warn("Subscription plan delete rejected while in use", slog.Any("error", storageCause(err)))
			writeSubscriptionError(w, http.StatusConflict, "Subscription plan is in use by one or more subscriptions")
		default:
			log.Error("Failed to delete subscription plan", slog.Any("error", storageCause(err)))
			writeSubscriptionError(w, http.StatusInternalServerError, "Failed to delete subscription plan")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writePlanLoadError writes the response for a failed plan read: 404 when it does not exist;
// otherwise it logs logMessage and returns 500.
func writePlanLoadError(w http.ResponseWriter, log *slog.Logger, err error, logMessage string) {
	if errors.Is(err, subscription.ErrPlanNotFound) {
		writeSubscriptionError(w, http.StatusNotFound, "Subscription plan not found")
		return
	}
	log.Error(logMessage, slog.Any("error", storageCause(err)))
	writeSubscriptionError(w, http.StatusInternalServerError, "Failed to get subscription plan")
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

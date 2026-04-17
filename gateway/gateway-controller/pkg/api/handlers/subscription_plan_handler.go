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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
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
func (s *APIServer) CreateSubscriptionPlan(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	var req api.SubscriptionPlanCreateRequest
	if err := s.bindRequestBody(c, &req); err != nil {
		log.Warn("Invalid subscription plan create body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	planName := strings.TrimSpace(req.PlanName)
	if planName == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "planName is required"})
		return
	}

	var unitStr *string
	if req.ThrottleLimitUnit != nil {
		s := string(*req.ThrottleLimitUnit)
		unitStr = &s
	}
	if err := validateThrottleLimits(req.ThrottleLimitCount, unitStr); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	status := models.SubscriptionPlanStatusActive
	if req.Status != nil {
		st := models.SubscriptionPlanStatus(*req.Status)
		switch st {
		case models.SubscriptionPlanStatusActive, models.SubscriptionPlanStatusInactive:
			status = st
		default:
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("invalid status: %s", *req.Status)})
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
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: "Subscription plan already exists"})
			return
		}
		log.Error("Failed to save subscription plan", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create subscription plan"})
		return
	}
	c.JSON(http.StatusCreated, subscriptionPlanToResponse(plan))
}

// ListSubscriptionPlans implements ServerInterface.ListSubscriptionPlans (GET /subscription-plans)
func (s *APIServer) ListSubscriptionPlans(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)

	list, err := s.db.ListSubscriptionPlans("")
	if err != nil {
		log.Error("Failed to list subscription plans", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list subscription plans"})
		return
	}
	items := make([]api.SubscriptionPlanResponse, 0, len(list))
	for _, p := range list {
		items = append(items, subscriptionPlanToResponse(p))
	}
	count := len(items)
	c.JSON(http.StatusOK, api.SubscriptionPlanListResponse{SubscriptionPlans: &items, Count: &count})
}

// GetSubscriptionPlan implements ServerInterface.GetSubscriptionPlan (GET /subscription-plans/{planId})
func (s *APIServer) GetSubscriptionPlan(c *gin.Context, planId string) {
	log := middleware.GetLogger(c, s.logger)

	plan, err := s.db.GetSubscriptionPlanByID(planId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
			return
		}
		log.Error("Failed to get subscription plan", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription plan"})
		return
	}
	if plan == nil {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
		return
	}
	c.JSON(http.StatusOK, subscriptionPlanToResponse(plan))
}

// UpdateSubscriptionPlan implements ServerInterface.UpdateSubscriptionPlan (PUT /subscription-plans/{planId})
func (s *APIServer) UpdateSubscriptionPlan(c *gin.Context, planId string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	existing, err := s.db.GetSubscriptionPlanByID(planId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
			return
		}
		log.Error("Failed to get subscription plan for update", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription plan"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
		return
	}

	var req api.SubscriptionPlanUpdateRequest
	if err := s.bindRequestBody(c, &req); err != nil {
		log.Warn("Invalid subscription plan update body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
		return
	}

	var unitStr *string
	if req.ThrottleLimitUnit != nil {
		s := string(*req.ThrottleLimitUnit)
		unitStr = &s
	}
	if err := validateThrottleLimits(req.ThrottleLimitCount, unitStr); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		return
	}

	if req.PlanName != nil {
		trimmed := strings.TrimSpace(*req.PlanName)
		if trimmed == "" {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "planName cannot be empty"})
			return
		}
		existing.PlanName = trimmed
	}
	if req.BillingPlan != nil {
		existing.BillingPlan = req.BillingPlan
	}
	if req.StopOnQuotaReach != nil {
		existing.StopOnQuotaReach = *req.StopOnQuotaReach
	}
	if req.ThrottleLimitCount != nil && req.ThrottleLimitUnit != nil {
		s := string(*req.ThrottleLimitUnit)
		existing.ThrottleLimitCount = req.ThrottleLimitCount
		existing.ThrottleLimitUnit = &s
	}
	if req.ExpiryTime != nil {
		existing.ExpiryTime = req.ExpiryTime
	}
	if req.Status != nil {
		st := models.SubscriptionPlanStatus(*req.Status)
		switch st {
		case models.SubscriptionPlanStatusActive, models.SubscriptionPlanStatusInactive:
			existing.Status = st
		default:
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("invalid status: %s", *req.Status)})
			return
		}
	}

	if err := s.getSubscriptionResourceService().UpdateSubscriptionPlan(existing, correlationID, log); err != nil {
		log.Error("Failed to update subscription plan", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update subscription plan"})
		return
	}
	c.JSON(http.StatusOK, subscriptionPlanToResponse(existing))
}

// DeleteSubscriptionPlan implements ServerInterface.DeleteSubscriptionPlan (DELETE /subscription-plans/{planId})
func (s *APIServer) DeleteSubscriptionPlan(c *gin.Context, planId string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	if err := s.getSubscriptionResourceService().DeleteSubscriptionPlan(planId, correlationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription plan not found"})
			return
		}
		log.Error("Failed to delete subscription plan", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to delete subscription plan"})
		return
	}
	c.Status(http.StatusNoContent)
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

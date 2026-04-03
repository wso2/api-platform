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
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/model"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
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
// count is at least 1, and unit is one of Min, Hour, Day, Month.
func validateThrottleLimitPair(count *int, unit *string) string {
	if (count != nil && unit == nil) || (count == nil && unit != nil) {
		return "throttleLimitCount and throttleLimitUnit must be provided together"
	}
	if count != nil && unit != nil {
		if *count < 1 {
			return "throttleLimitCount must be at least 1"
		}
		switch *unit {
		case "Min", "Hour", "Day", "Month":
		default:
			return "throttleLimitUnit must be one of: Min, Hour, Day, Month"
		}
	}
	return ""
}

// CreateSubscriptionPlanRequest is the body for POST /api/v1/subscription-plans
type CreateSubscriptionPlanRequest struct {
	PlanName           string  `json:"planName" binding:"required"`
	BillingPlan        string  `json:"billingPlan,omitempty"`
	StopOnQuotaReach   *bool   `json:"stopOnQuotaReach,omitempty"`
	ThrottleLimitCount *int    `json:"throttleLimitCount,omitempty"`
	ThrottleLimitUnit  *string `json:"throttleLimitUnit,omitempty"`
	ExpiryTime         *string `json:"expiryTime,omitempty"`
	Status             string  `json:"status,omitempty"`
}

// UpdateSubscriptionPlanRequest is the body for PUT /api/v1/subscription-plans/:planId
// All fields use pointers for patch semantics: nil = omitted, non-nil = set (including clear-to-empty).
type UpdateSubscriptionPlanRequest struct {
	PlanName           *string `json:"planName,omitempty"`
	BillingPlan        *string `json:"billingPlan,omitempty"`
	StopOnQuotaReach   *bool   `json:"stopOnQuotaReach,omitempty"`
	ThrottleLimitCount *int    `json:"throttleLimitCount,omitempty"`
	ThrottleLimitUnit  *string `json:"throttleLimitUnit,omitempty"`
	ExpiryTime         *string `json:"expiryTime,omitempty"`
	Status             *string `json:"status,omitempty"`
}

// CreateSubscriptionPlan handles POST /api/v1/subscription-plans
func (h *SubscriptionPlanHandler) CreateSubscriptionPlan(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req CreateSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("Invalid create subscription plan request body", "organizationId", orgId, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if req.Status != "" {
		switch req.Status {
		case "ACTIVE", "INACTIVE":
		default:
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid status value; must be ACTIVE or INACTIVE"))
			return
		}
	}

	if errMsg := validateThrottleLimitPair(req.ThrottleLimitCount, req.ThrottleLimitUnit); errMsg != "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
		return
	}

	var throttleLimitUnit string
	if req.ThrottleLimitUnit != nil {
		throttleLimitUnit = *req.ThrottleLimitUnit
	}
	plan := &model.SubscriptionPlan{
		PlanName:           req.PlanName,
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
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid expiryTime format; use RFC3339"))
			return
		}
		plan.ExpiryTime = &t
	}

	created, err := h.planService.CreatePlan(orgId, plan)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionPlanAlreadyExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", err.Error()))
			return
		}
		h.slogger.Error("Failed to create subscription plan", "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create subscription plan"))
		return
	}
	c.JSON(http.StatusCreated, toSubscriptionPlanResponse(created))
}

// ListSubscriptionPlans handles GET /api/v1/subscription-plans
func (h *SubscriptionPlanHandler) ListSubscriptionPlans(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
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
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list subscription plans"))
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, p := range list {
		items = append(items, toSubscriptionPlanResponse(p))
	}
	c.JSON(http.StatusOK, gin.H{"subscriptionPlans": items, "count": len(items)})
}

// GetSubscriptionPlan handles GET /api/v1/subscription-plans/:planId
func (h *SubscriptionPlanHandler) GetSubscriptionPlan(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	planId := c.Param("planId")
	if planId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Plan ID is required"))
		return
	}

	plan, err := h.planService.GetPlan(planId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionPlanNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription plan not found"))
			return
		}
		h.slogger.Error("Failed to get subscription plan", "planId", planId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get subscription plan"))
		return
	}
	c.JSON(http.StatusOK, toSubscriptionPlanResponse(plan))
}

// UpdateSubscriptionPlan handles PUT /api/v1/subscription-plans/:planId
func (h *SubscriptionPlanHandler) UpdateSubscriptionPlan(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	planId := c.Param("planId")
	if planId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Plan ID is required"))
		return
	}

	var req UpdateSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("Invalid update subscription plan request body", "planId", planId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if errMsg := validateThrottleLimitPair(req.ThrottleLimitCount, req.ThrottleLimitUnit); errMsg != "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", errMsg))
		return
	}

	update := &model.SubscriptionPlanUpdate{
		StopOnQuotaReach:   req.StopOnQuotaReach,
		ThrottleLimitCount: req.ThrottleLimitCount,
	}
	if req.PlanName != nil {
		update.PlanName = req.PlanName
	}
	if req.BillingPlan != nil {
		update.BillingPlan = req.BillingPlan
	}
	if req.ThrottleLimitUnit != nil {
		update.ThrottleLimitUnit = req.ThrottleLimitUnit
	}
	if req.Status != nil {
		switch model.SubscriptionPlanStatus(*req.Status) {
		case model.SubscriptionPlanStatusActive, model.SubscriptionPlanStatusInactive:
			st := model.SubscriptionPlanStatus(*req.Status)
			update.Status = &st
		default:
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid status value; must be ACTIVE or INACTIVE"))
			return
		}
	}
	if req.ExpiryTime != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiryTime)
		if err != nil {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid expiryTime format; use RFC3339"))
			return
		}
		update.ExpiryTime = &t
	}

	updated, err := h.planService.UpdatePlan(planId, orgId, update)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionPlanNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription plan not found"))
			return
		}
		if errors.Is(err, constants.ErrSubscriptionPlanAlreadyExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", err.Error()))
			return
		}
		h.slogger.Error("Failed to update subscription plan", "planId", planId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update subscription plan"))
		return
	}
	c.JSON(http.StatusOK, toSubscriptionPlanResponse(updated))
}

// DeleteSubscriptionPlan handles DELETE /api/v1/subscription-plans/:planId
func (h *SubscriptionPlanHandler) DeleteSubscriptionPlan(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	planId := c.Param("planId")
	if planId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Plan ID is required"))
		return
	}

	err := h.planService.DeletePlan(planId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionPlanNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription plan not found"))
			return
		}
		h.slogger.Error("Failed to delete subscription plan", "planId", planId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete subscription plan"))
		return
	}
	c.Status(http.StatusNoContent)
}

// RegisterRoutes registers subscription plan routes
func (h *SubscriptionPlanHandler) RegisterRoutes(r *gin.Engine) {
	group := r.Group("/api/v1/subscription-plans")
	{
		group.POST("", h.CreateSubscriptionPlan)
		group.GET("", h.ListSubscriptionPlans)
		group.GET("/:planId", h.GetSubscriptionPlan)
		group.PUT("/:planId", h.UpdateSubscriptionPlan)
		group.DELETE("/:planId", h.DeleteSubscriptionPlan)
	}
}

func toSubscriptionPlanResponse(plan *model.SubscriptionPlan) gin.H {
	resp := gin.H{
		"id":               plan.UUID,
		"planName":         plan.PlanName,
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

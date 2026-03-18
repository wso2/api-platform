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
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/model"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
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

// CreateSubscriptionRequest is the body for POST /api/v1/subscriptions
type CreateSubscriptionRequest struct {
	APIID              string  `json:"apiId" binding:"required"`
	ApplicationID      *string `json:"applicationId,omitempty"`
	SubscriptionPlanID *string `json:"subscriptionPlanId,omitempty"`
	Status             string  `json:"status,omitempty"`
}

// UpdateSubscriptionRequest is the body for PUT /api/v1/subscriptions/:subscriptionId
type UpdateSubscriptionRequest struct {
	Status string `json:"status,omitempty"`
}

// CreateSubscription handles POST /api/v1/subscriptions
func (h *SubscriptionHandler) CreateSubscription(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		h.slogger.Error("Organization claim not found in token when creating subscription")
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}
	var req CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("Invalid create subscription request body", "organizationId", orgId, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}
	if req.APIID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API ID is required"))
		return
	}
	switch req.Status {
	case "", "ACTIVE", "INACTIVE", "REVOKED":
	default:
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid status value"))
		return
	}
	sub, err := h.subscriptionService.CreateSubscription(req.APIID, orgId, req.ApplicationID, req.SubscriptionPlanID, req.Status)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API not found"))
			return
		}
		if errors.Is(err, constants.ErrSubscriptionAlreadyExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Subscription already exists for this API"))
			return
		}
		h.slogger.Error("Failed to create subscription", "apiId", req.APIID, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create subscription"))
		return
	}
	c.JSON(http.StatusCreated, h.toSubscriptionResponse(sub, orgId))
}

// ListSubscriptions handles GET /api/v1/subscriptions
func (h *SubscriptionHandler) ListSubscriptions(c *gin.Context) {
	apiId := c.Query("apiId")
	applicationID := c.Query("applicationId")
	status := c.Query("status")

	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	var apiIDPtr, appIDPtr, statusPtr *string
	if apiId != "" {
		apiIDPtr = &apiId
	}
	if applicationID != "" {
		appIDPtr = &applicationID
	}
	if status != "" {
		switch status {
		case "ACTIVE", "INACTIVE", "REVOKED":
		default:
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid status value"))
			return
		}
		statusPtr = &status
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
	list, total, err := h.subscriptionService.ListSubscriptionsByFilters(orgId, apiIDPtr, appIDPtr, statusPtr, limit, offset)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API not found"))
			return
		}
		h.slogger.Error("Failed to list subscriptions", "apiId", apiId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list subscriptions"))
		return
	}
	// Bulk fetch API handles and plan names to avoid N+1 queries
	apiUUIDSet := make(map[string]struct{})
	planIDSet := make(map[string]struct{})
	for _, sub := range list {
		if sub.APIUUID != "" {
			apiUUIDSet[sub.APIUUID] = struct{}{}
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
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list subscriptions"))
		return
	}
	planNameMap, err := h.subscriptionPlanService.GetPlanNameMap(planIDs, orgId)
	if err != nil {
		h.slogger.Error("Failed to bulk fetch plan names for list", "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list subscriptions"))
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, sub := range list {
		items = append(items, h.toSubscriptionResponseWithMaps(sub, orgId, apiHandleMap, planNameMap))
	}
	c.JSON(http.StatusOK, gin.H{
		"subscriptions": items,
		"count":          len(items),
		"pagination": gin.H{
			"total":  total,
			"offset": offset,
			"limit":  limit,
		},
	})
}

// GetSubscription handles GET /api/v1/subscriptions/:subscriptionId
func (h *SubscriptionHandler) GetSubscription(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}
	subscriptionId := c.Param("subscriptionId")
	if subscriptionId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Subscription ID is required"))
		return
	}
	sub, err := h.subscriptionService.GetSubscription(subscriptionId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription not found"))
			return
		}
		h.slogger.Error("Failed to get subscription", "subscriptionId", subscriptionId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get subscription"))
		return
	}
	c.JSON(http.StatusOK, h.toSubscriptionResponse(sub, orgId))
}

// UpdateSubscription handles PUT /api/v1/subscriptions/:subscriptionId
func (h *SubscriptionHandler) UpdateSubscription(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}
	subscriptionId := c.Param("subscriptionId")
	if subscriptionId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Subscription ID is required"))
		return
	}
	var req UpdateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}
	switch req.Status {
	case "", "ACTIVE", "INACTIVE", "REVOKED":
	default:
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid subscription status"))
		return
	}
	sub, err := h.subscriptionService.UpdateSubscription(subscriptionId, orgId, req.Status)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription not found"))
			return
		}
		h.slogger.Error("Failed to update subscription", "subscriptionId", subscriptionId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update subscription"))
		return
	}
	c.JSON(http.StatusOK, h.toSubscriptionResponse(sub, orgId))
}

// DeleteSubscription handles DELETE /api/v1/subscriptions/:subscriptionId
func (h *SubscriptionHandler) DeleteSubscription(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}
	subscriptionId := c.Param("subscriptionId")
	if subscriptionId == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Subscription ID is required"))
		return
	}
	err := h.subscriptionService.DeleteSubscription(subscriptionId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription not found"))
			return
		}
		h.slogger.Error("Failed to delete subscription", "subscriptionId", subscriptionId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete subscription"))
		return
	}
	c.Status(http.StatusNoContent)
}

// RegisterRoutes registers subscription routes under the given router
func (h *SubscriptionHandler) RegisterRoutes(r *gin.Engine) {
	subGroup := r.Group("/api/v1/subscriptions")
	{
		subGroup.POST("", h.CreateSubscription)
		subGroup.GET("", h.ListSubscriptions)
		subGroup.GET("/:subscriptionId", h.GetSubscription)
		subGroup.PUT("/:subscriptionId", h.UpdateSubscription)
		subGroup.DELETE("/:subscriptionId", h.DeleteSubscription)
	}
}

func (h *SubscriptionHandler) toSubscriptionResponse(sub *model.Subscription, orgId string) gin.H {
	// apiId in response should be the handle (e.g. "samp1"), not the internal UUID
	apiIdForResponse := h.subscriptionService.ResolveAPIHandle(sub.APIUUID, orgId)
	if apiIdForResponse == "" {
		apiIdForResponse = sub.APIUUID // fallback to UUID
	}
	resp := gin.H{
		"id":             sub.UUID,
		"apiId":          apiIdForResponse,
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
		// Resolve plan name for display (subscription_plans.plan_name)
		if h.subscriptionPlanService != nil {
			plan, err := h.subscriptionPlanService.GetPlan(*sub.SubscriptionPlanID, orgId)
			if err == nil && plan != nil {
				resp["subscriptionPlanName"] = plan.PlanName
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
func (h *SubscriptionHandler) toSubscriptionResponseWithMaps(sub *model.Subscription, orgId string, apiHandleMap, planNameMap map[string]string) gin.H {
	apiIdForResponse := apiHandleMap[sub.APIUUID]
	if apiIdForResponse == "" {
		apiIdForResponse = sub.APIUUID // fallback to UUID
	}
	resp := gin.H{
		"id":             sub.UUID,
		"apiId":          apiIdForResponse,
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

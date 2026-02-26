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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/model"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// SubscriptionHandler handles application-level subscription CRUD
type SubscriptionHandler struct {
	subscriptionService *service.SubscriptionService
	slogger             *slog.Logger
}

// NewSubscriptionHandler creates a new subscription handler
func NewSubscriptionHandler(subscriptionService *service.SubscriptionService, slogger *slog.Logger) *SubscriptionHandler {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &SubscriptionHandler{
		subscriptionService: subscriptionService,
		slogger:             slogger,
	}
}

// CreateSubscriptionRequest is the body for POST /api/v1/subscriptions
type CreateSubscriptionRequest struct {
	APIID         string `json:"apiId" binding:"required"`
	ApplicationID string `json:"applicationId" binding:"required"`
	Status        string `json:"status,omitempty"` // ACTIVE, INACTIVE, REVOKED; default ACTIVE
}

// UpdateSubscriptionRequest is the body for PUT /api/v1/subscriptions/:subscriptionId
type UpdateSubscriptionRequest struct {
	Status string `json:"status,omitempty"` // ACTIVE, INACTIVE, REVOKED
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
		h.slogger.Error("API ID is required for subscription creation", "organizationId", orgId, "applicationId", req.ApplicationID)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API ID is required"))
		return
	}
	// Validate status if provided to prevent invalid values from being persisted.
	switch req.Status {
	case "", "ACTIVE", "INACTIVE", "REVOKED":
		// ok
	default:
		h.slogger.Error("Invalid subscription status in create request",
			"apiId", req.APIID, "organizationId", orgId, "applicationId", req.ApplicationID, "status", req.Status)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid status value"))
		return
	}
	sub, err := h.subscriptionService.CreateSubscription(req.APIID, orgId, req.ApplicationID, req.Status)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found when creating subscription", "apiId", req.APIID, "organizationId", orgId, "applicationId", req.ApplicationID)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API not found"))
			return
		}
		if errors.Is(err, constants.ErrSubscriptionAlreadyExists) {
			h.slogger.Error("Subscription already exists",
				"apiId", req.APIID, "organizationId", orgId, "applicationId", req.ApplicationID)
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Application is already subscribed to this API"))
			return
		}
		h.slogger.Error("Failed to create subscription", "apiId", req.APIID, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create subscription"))
		return
	}
	c.JSON(http.StatusCreated, toSubscriptionResponse(sub))
}

// ListSubscriptions handles GET /api/v1/subscriptions
func (h *SubscriptionHandler) ListSubscriptions(c *gin.Context) {
	apiId := c.Query("apiId")
	applicationID := c.Query("applicationId")
	status := c.Query("status")

	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		h.slogger.Error("Organization claim not found in token when listing subscriptions",
			"organizationClaim", "missing",
			"apiId", apiId,
			"applicationId", applicationID)
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
		// Validate status filter before passing it to service layer.
		switch status {
		case "ACTIVE", "INACTIVE", "REVOKED":
			// ok
		default:
			h.slogger.Error("Invalid status filter in list subscriptions request",
				"organizationId", orgId, "apiId", apiId, "applicationId", applicationID, "status", status)
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid status value"))
			return
		}
		statusPtr = &status
	}
	list, err := h.subscriptionService.ListSubscriptionsByFilters(orgId, apiIDPtr, appIDPtr, statusPtr)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			h.slogger.Error("API not found when listing subscriptions",
				"organizationId", orgId, "apiId", apiId, "applicationId", applicationID, "status", status)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API not found"))
			return
		}
		h.slogger.Error("Failed to list subscriptions", "apiId", apiId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list subscriptions"))
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, sub := range list {
		items = append(items, toSubscriptionResponse(sub))
	}
	c.JSON(http.StatusOK, gin.H{"subscriptions": items, "count": len(items)})
}

// GetSubscription handles GET /api/v1/subscriptions/:subscriptionId
func (h *SubscriptionHandler) GetSubscription(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		h.slogger.Error("Organization claim not found in token when getting subscription")
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}
	subscriptionId := c.Param("subscriptionId")
	if subscriptionId == "" {
		h.slogger.Error("Subscription ID is required in get subscription request", "organizationId", orgId)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Subscription ID is required"))
		return
	}
	sub, err := h.subscriptionService.GetSubscription(subscriptionId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			h.slogger.Error("Subscription not found",
				"subscriptionId", subscriptionId, "organizationId", orgId)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription not found"))
			return
		}
		h.slogger.Error("Failed to get subscription", "subscriptionId", subscriptionId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get subscription"))
		return
	}
	c.JSON(http.StatusOK, toSubscriptionResponse(sub))
}

// UpdateSubscription handles PUT /api/v1/subscriptions/:subscriptionId
func (h *SubscriptionHandler) UpdateSubscription(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		h.slogger.Error("Organization claim not found in token when updating subscription")
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}
	subscriptionId := c.Param("subscriptionId")
	if subscriptionId == "" {
		h.slogger.Error("Subscription ID is required in update subscription request", "organizationId", orgId)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Subscription ID is required"))
		return
	}
	var req UpdateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.slogger.Error("Invalid update subscription request body", "subscriptionId", subscriptionId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}
	// Validate status if provided.
	switch req.Status {
	case "", "ACTIVE", "INACTIVE", "REVOKED":
		// ok
	default:
		h.slogger.Error("Invalid subscription status in update request",
			"subscriptionId", subscriptionId, "organizationId", orgId, "status", req.Status)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid status value"))
		return
	}
	sub, err := h.subscriptionService.UpdateSubscription(subscriptionId, orgId, req.Status)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			h.slogger.Error("Subscription not found when updating",
				"subscriptionId", subscriptionId, "organizationId", orgId)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription not found"))
			return
		}
		h.slogger.Error("Failed to update subscription", "subscriptionId", subscriptionId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update subscription"))
		return
	}
	c.JSON(http.StatusOK, toSubscriptionResponse(sub))
}

// DeleteSubscription handles DELETE /api/v1/subscriptions/:subscriptionId
func (h *SubscriptionHandler) DeleteSubscription(c *gin.Context) {
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		h.slogger.Error("Organization claim not found in token when deleting subscription")
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}
	subscriptionId := c.Param("subscriptionId")
	if subscriptionId == "" {
		h.slogger.Error("Subscription ID is required in delete subscription request", "organizationId", orgId)
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Subscription ID is required"))
		return
	}
	err := h.subscriptionService.DeleteSubscription(subscriptionId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			h.slogger.Error("Subscription not found when deleting",
				"subscriptionId", subscriptionId, "organizationId", orgId)
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Subscription not found"))
			return
		}
		h.slogger.Error("Failed to delete subscription", "subscriptionId", subscriptionId, "organizationId", orgId, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete subscription"))
		return
	}
	c.Status(http.StatusNoContent)
}

// toSubscriptionResponse maps model.Subscription to JSON response
func toSubscriptionResponse(sub *model.Subscription) gin.H {
	return gin.H{
		"id":             sub.UUID,
		"apiId":          sub.APIUUID,
		"applicationId":  sub.ApplicationID,
		"organizationId": sub.OrganizationUUID,
		"status":         string(sub.Status),
		"createdAt":      sub.CreatedAt,
		"updatedAt":      sub.UpdatedAt,
	}
}

// RegisterRoutes registers subscription routes under the given router
func (h *SubscriptionHandler) RegisterRoutes(r *gin.Engine) {
	// Root-level /api/v1/subscriptions resource
	subGroup := r.Group("/api/v1/subscriptions")
	{
		subGroup.POST("", h.CreateSubscription)
		subGroup.GET("", h.ListSubscriptions)
		subGroup.GET("/:subscriptionId", h.GetSubscription)
		subGroup.PUT("/:subscriptionId", h.UpdateSubscription)
		subGroup.DELETE("/:subscriptionId", h.DeleteSubscription)
	}
}

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

// CreateSubscription implements ServerInterface.CreateSubscription (POST /subscriptions)
func (s *APIServer) CreateSubscription(c *gin.Context) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	var req api.SubscriptionCreateRequest
	if err := s.bindRequestBody(c, &req); err != nil {
		log.Warn("Invalid subscription create body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	if strings.TrimSpace(req.ApiId) == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "apiId is required"})
		return
	}
	if strings.TrimSpace(req.SubscriptionToken) == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "subscriptionToken is required"})
		return
	}

	// Resolve apiId (deployment ID or handle) to the internal deployment ID used for persistence.
	apiID, err := s.resolveAPIIDByHandle(c, req.ApiId, log)
	if err != nil {
		// resolveAPIIDByHandle already wrote the appropriate response.
		return
	}

	// Validate subscription plan when provided: must exist, be ACTIVE, and be enabled for this API.
	if req.SubscriptionPlanId != nil && *req.SubscriptionPlanId != "" {
		plan, err := s.db.GetSubscriptionPlanByID(*req.SubscriptionPlanId, "")
		if err != nil || plan == nil {
			log.Warn("Subscription plan not found for subscription creation",
				slog.String("subscription_plan_id", *req.SubscriptionPlanId),
				slog.String("api_id", apiID))
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: "Subscription plan not found or not enabled",
			})
			return
		}
		if plan.Status != models.SubscriptionPlanStatusActive {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: "Subscription plan is not active",
			})
			return
		}
		cfg, err := s.db.GetConfig(apiID)
		if err != nil || cfg == nil {
			log.Error("Failed to load API configuration for subscription plan validation",
				slog.String("api_id", apiID), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to validate subscription plan",
			})
			return
		}
		if cfg.Kind == string(api.RestAPIKindRestApi) {
			if restAPI, ok := cfg.Configuration.(api.RestAPI); ok {
				if restAPI.Spec.SubscriptionPlans != nil && len(*restAPI.Spec.SubscriptionPlans) > 0 {
					enabled := false
					for _, name := range *restAPI.Spec.SubscriptionPlans {
						if strings.EqualFold(name, plan.PlanName) {
							enabled = true
							break
						}
					}
					if !enabled {
						c.JSON(http.StatusBadRequest, api.ErrorResponse{
							Status:  "error",
							Message: fmt.Sprintf("Subscription plan %q is not enabled for this API", plan.PlanName),
						})
						return
					}
				}
			}
		}
	}

	status := models.SubscriptionStatusActive
	if req.Status != nil {
		st := models.SubscriptionStatus(*req.Status)
		switch st {
		case models.SubscriptionStatusActive,
			models.SubscriptionStatusInactive,
			models.SubscriptionStatusRevoked:
			status = st
		default:
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("invalid status: %s", *req.Status),
			})
			return
		}
	}
	var appID *string
	if req.ApplicationId != nil && *req.ApplicationId != "" {
		appID = req.ApplicationId
	}
	sub := &models.Subscription{
		ID:                    uuid.New().String(),
		APIID:                 apiID,
		ApplicationID:         appID,
		SubscriptionPlanID:    req.SubscriptionPlanId,
		BillingCustomerID:     req.BillingCustomerId,
		BillingSubscriptionID: req.BillingSubscriptionId,
		Status:                status,
		SubscriptionToken:     strings.TrimSpace(req.SubscriptionToken),
	}
	if err := s.getSubscriptionResourceService().SaveSubscription(sub, correlationID, log); err != nil {
		if storage.IsConflictError(err) {
			c.JSON(http.StatusConflict, api.ErrorResponse{Status: "error", Message: "Application already subscribed to this API"})
			return
		}
		log.Error("Failed to save subscription", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create subscription"})
		return
	}
	resp := subscriptionToResponseWithToken(sub)
	c.JSON(http.StatusCreated, resp)
}

// ListSubscriptions implements ServerInterface.ListSubscriptions (GET /subscriptions)
func (s *APIServer) ListSubscriptions(c *gin.Context, params api.ListSubscriptionsParams) {
	log := middleware.GetLogger(c, s.logger)

	var apiID, appID, status *string
	if params.ApiId != nil && *params.ApiId != "" {
		// Normalize apiId to the internal deployment ID (accepts handle or deployment ID).
		resolvedID, err := s.resolveAPIIDByHandle(c, *params.ApiId, log)
		if err != nil {
			// resolveAPIIDByHandle already wrote the response.
			return
		}
		apiIDCopy := resolvedID
		apiID = &apiIDCopy
	}
	if params.ApplicationId != nil && *params.ApplicationId != "" {
		appID = params.ApplicationId
	}
	if params.Status != nil && *params.Status != "" {
		st := string(*params.Status)
		status = &st
	}
	// apiId is an optional filter. When omitted, all subscriptions for this gateway are returned
	// (optionally filtered by applicationId and/or status).
	apiIDValue := ""
	if apiID != nil {
		apiIDValue = *apiID
	}
	list, err := s.db.ListSubscriptionsByAPI(apiIDValue, "", appID, status)
	if err != nil {
		log.Error("Failed to list subscriptions", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list subscriptions"})
		return
	}
	out := make([]api.SubscriptionResponse, 0, len(list))
	for _, sub := range list {
		out = append(out, subscriptionToResponse(sub))
	}
	c.JSON(http.StatusOK, api.SubscriptionListResponse{
		Subscriptions: &out,
		Count:         ptr(int(len(list))),
	})
}

// GetSubscription implements ServerInterface.GetSubscription (GET /subscriptions/{subscriptionId})
func (s *APIServer) GetSubscription(c *gin.Context, subscriptionId string) {
	log := middleware.GetLogger(c, s.logger)

	sub, err := s.db.GetSubscriptionByID(subscriptionId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to get subscription", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription"})
		return
	}
	if sub == nil {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
		return
	}
	c.JSON(http.StatusOK, subscriptionToResponse(sub))
}

// UpdateSubscription implements ServerInterface.UpdateSubscription (PUT /subscriptions/{subscriptionId})
func (s *APIServer) UpdateSubscription(c *gin.Context, subscriptionId string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	sub, err := s.db.GetSubscriptionByID(subscriptionId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to get subscription for update", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription"})
		return
	}
	if sub == nil {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
		return
	}
	var req api.SubscriptionUpdateRequest
	if err := s.bindRequestBody(c, &req); err != nil {
		log.Warn("Invalid subscription update body", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	if req.Status != nil {
		st := models.SubscriptionStatus(*req.Status)
		switch st {
		case models.SubscriptionStatusActive,
			models.SubscriptionStatusInactive,
			models.SubscriptionStatusRevoked:
			sub.Status = st
		default:
			c.JSON(http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("invalid status: %s", *req.Status),
			})
			return
		}
	}
	if err := s.getSubscriptionResourceService().UpdateSubscription(sub, correlationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to update subscription", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update subscription"})
		return
	}
	c.JSON(http.StatusOK, subscriptionToResponse(sub))
}

// DeleteSubscription implements ServerInterface.DeleteSubscription (DELETE /subscriptions/{subscriptionId})
func (s *APIServer) DeleteSubscription(c *gin.Context, subscriptionId string) {
	log := middleware.GetLogger(c, s.logger)
	correlationID := middleware.GetCorrelationID(c)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	sub, err := s.db.GetSubscriptionByID(subscriptionId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to get subscription for deletion", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription"})
		return
	}
	if sub == nil {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
		return
	}
	if err := s.getSubscriptionResourceService().DeleteSubscription(subscriptionId, correlationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to delete subscription", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to delete subscription"})
		return
	}
	c.Status(http.StatusNoContent)
}

// subscriptionToResponse builds a response without the subscription token.
// DB reads only have subscription_token_hash; token is never stored. Token is returned only at creation via subscriptionToResponseWithToken.
func subscriptionToResponse(sub *models.Subscription) api.SubscriptionResponse {
	resp := api.SubscriptionResponse{
		Id:                ptr(sub.ID),
		ApiId:             ptr(sub.APIID),
		GatewayId:         ptr(sub.GatewayID),
		CreatedAt:         &sub.CreatedAt,
		UpdatedAt:         &sub.UpdatedAt,
		SubscriptionToken: nil, // Explicitly omit; gateway does not store token, use Platform-API to retrieve
	}
	if sub.ApplicationID != nil {
		resp.ApplicationId = sub.ApplicationID
	}
	if sub.SubscriptionPlanID != nil {
		resp.SubscriptionPlanId = sub.SubscriptionPlanID
	}
	if sub.BillingCustomerID != nil {
		resp.BillingCustomerId = sub.BillingCustomerID
	}
	if sub.BillingSubscriptionID != nil {
		resp.BillingSubscriptionId = sub.BillingSubscriptionID
	}
	if sub.Status != "" {
		st := api.SubscriptionResponseStatus(sub.Status)
		resp.Status = &st
	}
	return resp
}

// subscriptionToResponseWithToken adds the token to the response (create flow only).
// Call only when sub has the raw token from creation, never from DB reads.
func subscriptionToResponseWithToken(sub *models.Subscription) api.SubscriptionResponse {
	resp := subscriptionToResponse(sub)
	if sub.SubscriptionToken != "" {
		resp.SubscriptionToken = ptr(sub.SubscriptionToken)
	}
	return resp
}

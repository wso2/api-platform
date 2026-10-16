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
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// maxSubscriptionRequestBodyBytes caps subscription create/update request bodies.
const maxSubscriptionRequestBodyBytes = 64 << 10

// maxExternalRefIDLength matches the width of the subscriptions.uuid column.
const maxExternalRefIDLength = 64

// subscriptionConflictMessage is returned for every 409 on subscription create and update.
const subscriptionConflictMessage = "The subscription conflicts with an existing subscription"

// externalRefIDPattern restricts caller-supplied subscription IDs to characters that are safe in a URL path segment.
var externalRefIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// CreateSubscription implements ServerInterface.CreateSubscription (POST /subscriptions)
func (s *APIServer) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSubscriptionRequestBodyBytes)
	var req api.SubscriptionCreateRequest
	if err := s.bindRequestBody(r, &req); err != nil {
		log.Warn("Invalid subscription create body", slog.Any("error", err))
		writeSubscriptionBodyError(w, err)
		return
	}
	if strings.TrimSpace(req.ApiId) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "apiId is required"})
		return
	}
	if strings.TrimSpace(req.SubscriptionToken) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "subscriptionToken is required"})
		return
	}
	// An empty externalRefId is treated as absent, like the other optional identifiers on this request.
	externalRefID := ""
	if req.ExternalRefId != nil && *req.ExternalRefId != "" {
		externalRefID = *req.ExternalRefId
		if len(externalRefID) > maxExternalRefIDLength || !externalRefIDPattern.MatchString(externalRefID) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status: "error",
				Message: fmt.Sprintf("externalRefId must be 1-%d characters of letters, digits, '.', '_' or '-', "+
					"starting with a letter or digit", maxExternalRefIDLength),
			})
			return
		}
	}

	// Resolve apiId (deployment ID or handle) to the internal deployment ID used for persistence.
	apiID, err := s.resolveAPIIDByHandle(w, r, req.ApiId, log)
	if err != nil {
		// resolveAPIIDByHandle already wrote the appropriate response.
		return
	}

	// Validate subscription plan when provided: must exist, be ACTIVE, and be enabled for this API.
	if req.SubscriptionPlanId != nil && *req.SubscriptionPlanId != "" {
		if !s.validateSubscriptionPlanForAPI(w, *req.SubscriptionPlanId, apiID, log) {
			return
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
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
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
	subID := uuid.New().String()
	if externalRefID != "" {
		subID = externalRefID
	}
	sub := &models.Subscription{
		ID:                    subID,
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
			log.Warn("Subscription create rejected due to a conflict", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: subscriptionConflictMessage})
			return
		}
		log.Error("Failed to save subscription", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create subscription"})
		return
	}
	resp := subscriptionToResponseWithToken(sub)
	httputil.WriteJSON(w, http.StatusCreated, resp)
}

// ListSubscriptions implements ServerInterface.ListSubscriptions (GET /subscriptions)
func (s *APIServer) ListSubscriptions(w http.ResponseWriter, r *http.Request, params api.ListSubscriptionsParams) {
	log := middleware.GetLogger(r, s.logger)

	var apiID, appID, status *string
	if params.ApiId != nil && *params.ApiId != "" {
		// Normalize apiId to the internal deployment ID (accepts handle or deployment ID).
		resolvedID, err := s.resolveAPIIDByHandle(w, r, *params.ApiId, log)
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
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list subscriptions"})
		return
	}
	out := make([]api.SubscriptionResponse, 0, len(list))
	for _, sub := range list {
		out = append(out, subscriptionToResponse(sub))
	}
	httputil.WriteJSON(w, http.StatusOK, api.SubscriptionListResponse{
		Subscriptions: &out,
		Count:         ptr(int(len(list))),
	})
}

// GetSubscription implements ServerInterface.GetSubscription (GET /subscriptions/{subscriptionId})
func (s *APIServer) GetSubscription(w http.ResponseWriter, r *http.Request, subscriptionId string) {
	log := middleware.GetLogger(r, s.logger)

	sub, err := s.db.GetSubscriptionByID(subscriptionId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to get subscription", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription"})
		return
	}
	if sub == nil {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, subscriptionToResponse(sub))
}

// UpdateSubscription implements ServerInterface.UpdateSubscription (PUT /subscriptions/{subscriptionId})
func (s *APIServer) UpdateSubscription(w http.ResponseWriter, r *http.Request, subscriptionId string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	sub, err := s.db.GetSubscriptionByID(subscriptionId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to get subscription for update", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription"})
		return
	}
	if sub == nil {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSubscriptionRequestBodyBytes)
	var req api.SubscriptionUpdateRequest
	planIDPresent, err := s.bindSubscriptionUpdateBody(r, &req)
	if err != nil {
		log.Warn("Invalid subscription update body", slog.Any("error", err))
		writeSubscriptionBodyError(w, err)
		return
	}

	// Validate every field before changing sub, so a rejected request leaves it untouched.
	var newStatus *models.SubscriptionStatus
	if req.Status != nil {
		st := models.SubscriptionStatus(*req.Status)
		switch st {
		case models.SubscriptionStatusActive,
			models.SubscriptionStatusInactive,
			models.SubscriptionStatusRevoked:
			newStatus = &st
		default:
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("invalid status: %s", *req.Status),
			})
			return
		}
	}

	// An omitted subscriptionPlanId leaves the plan unchanged; an explicit null or empty string removes it.
	var newPlanID *string
	if planIDPresent && req.SubscriptionPlanId != nil && strings.TrimSpace(*req.SubscriptionPlanId) != "" {
		planID := strings.TrimSpace(*req.SubscriptionPlanId)
		newPlanID = &planID
		// Re-validate only on an actual change, so an unchanged plan that was later deactivated
		// doesn't block an unrelated status or token update.
		if sub.SubscriptionPlanID == nil || *sub.SubscriptionPlanID != planID {
			if !s.validateSubscriptionPlanForAPI(w, planID, sub.APIID, log) {
				return
			}
		}
	}

	newToken := ""
	if req.SubscriptionToken != nil {
		newToken = strings.TrimSpace(*req.SubscriptionToken)
		if newToken == "" {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "subscriptionToken must not be empty"})
			return
		}
	}

	if newStatus != nil {
		sub.Status = *newStatus
	}
	if planIDPresent {
		sub.SubscriptionPlanID = newPlanID
	}
	if newToken != "" {
		// UpdateSubscription re-hashes a non-empty plain token; otherwise it keeps the stored hash.
		sub.SubscriptionToken = newToken
	}
	if err := s.getSubscriptionResourceService().UpdateSubscription(sub, correlationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		if storage.IsConflictError(err) {
			log.Warn("Subscription update rejected due to a conflict", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: subscriptionConflictMessage})
			return
		}
		log.Error("Failed to update subscription", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update subscription"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, subscriptionToResponse(sub))
}

// DeleteSubscription implements ServerInterface.DeleteSubscription (DELETE /subscriptions/{subscriptionId})
func (s *APIServer) DeleteSubscription(w http.ResponseWriter, r *http.Request, subscriptionId string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	sub, err := s.db.GetSubscriptionByID(subscriptionId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to get subscription for deletion", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription"})
		return
	}
	if sub == nil {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
		return
	}
	if err := s.getSubscriptionResourceService().DeleteSubscription(subscriptionId, correlationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
			return
		}
		log.Error("Failed to delete subscription", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to delete subscription"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validateSubscriptionPlanForAPI checks that the plan exists, is ACTIVE and is enabled for the API.
// When it isn't, it writes the error response and returns false.
func (s *APIServer) validateSubscriptionPlanForAPI(w http.ResponseWriter, planID, apiID string, log *slog.Logger) bool {
	plan, err := s.db.GetSubscriptionPlanByID(planID, "")
	if err != nil || plan == nil {
		log.Warn("Subscription plan not found for subscription",
			slog.String("subscription_plan_id", planID),
			slog.String("api_id", apiID))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Subscription plan not found or not enabled",
		})
		return false
	}
	if plan.Status != models.SubscriptionPlanStatusActive {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Subscription plan is not active",
		})
		return false
	}
	cfg, err := s.db.GetConfig(apiID)
	if err != nil || cfg == nil {
		log.Error("Failed to load API configuration for subscription plan validation",
			slog.String("api_id", apiID), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to validate subscription plan",
		})
		return false
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
					httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
						Status:  "error",
						Message: fmt.Sprintf("Subscription plan %q is not enabled for this API", plan.PlanName),
					})
					return false
				}
			}
		}
	}
	return true
}

// bindSubscriptionUpdateBody binds the update body into req and reports whether subscriptionPlanId
// was present in it.
func (s *APIServer) bindSubscriptionUpdateBody(r *http.Request, req *api.SubscriptionUpdateRequest) (bool, error) {
	fields, err := s.bindRequestBodyWithFields(r, req)
	if err != nil {
		return false, err
	}
	_, present := fields["subscriptionPlanId"]
	return present, nil
}

// bindRequestBodyWithFields binds the body into dst and also returns the body's top-level fields.
// The generated pointer fields can't tell an omitted field from an explicit null, so partial-update
// handlers use the returned map to check whether a key was sent.
func (s *APIServer) bindRequestBodyWithFields(r *http.Request, dst interface{}) (map[string]interface{}, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err := s.bindRequestBody(r, dst); err != nil {
		return nil, err
	}
	var fields map[string]interface{}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err := s.bindRequestBody(r, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// writeSubscriptionBodyError writes 413 for an oversized body and 400 for any other unreadable body.
func writeSubscriptionBodyError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		httputil.WriteJSON(w, http.StatusRequestEntityTooLarge, api.ErrorResponse{Status: "error", Message: "Request body too large"})
		return
	}
	httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
}

// subscriptionToResponse builds a response without the subscription token.
// DB reads only have subscription_token_hash; token is never stored. Token is returned only at creation via subscriptionToResponseWithToken.
func subscriptionToResponse(sub *models.Subscription) api.SubscriptionResponse {
	resp := api.SubscriptionResponse{
		Id:                ptr(sub.ID),
		ExternalRefId:     ptr(sub.ID), // The subscription ID is the caller's externalRefId when one was supplied on create.
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

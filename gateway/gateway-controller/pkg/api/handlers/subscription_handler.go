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

// CreateSubscription implements ServerInterface.CreateSubscription (POST /subscriptions)
func (s *APIServer) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	var req api.SubscriptionCreateRequest
	if err := s.bindRequestBody(r, &req); err != nil {
		log.Warn("Invalid subscription create body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
		return
	}

	// The service resolves apiId (deployment ID or handle) itself, so there is no
	// pre-resolution step here; mapCreateSubscriptionError reproduces the
	// responses that resolution used to write directly.
	result, err := s.getSubscriptionService().Create(subscription.CreateParams{
		Request:       req,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		mapCreateSubscriptionError(w, log, req.ApiId, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, subscriptionToResponseWithToken(result.Subscription))
}

// ListSubscriptions implements ServerInterface.ListSubscriptions (GET /subscriptions)
func (s *APIServer) ListSubscriptions(w http.ResponseWriter, r *http.Request, params api.ListSubscriptionsParams) {
	log := middleware.GetLogger(r, s.logger)

	filter := subscription.ListFilter{}
	if params.ApiId != nil && *params.ApiId != "" {
		// Normalize apiId to the internal deployment ID (accepts handle or deployment ID).
		resolvedID, err := s.resolveAPIIDByHandle(w, *params.ApiId, log)
		if err != nil {
			// resolveAPIIDByHandle already wrote the response.
			return
		}
		filter.APIID = resolvedID
	}
	if params.ApplicationId != nil && *params.ApplicationId != "" {
		filter.ApplicationID = params.ApplicationId
	}
	if params.Status != nil && *params.Status != "" {
		status := string(*params.Status)
		filter.Status = &status
	}

	// apiId is an optional filter. When omitted, all subscriptions for this gateway are returned
	// (optionally filtered by applicationId and/or status).
	result, err := s.getSubscriptionService().List(filter)
	if err != nil {
		log.Error("Failed to list subscriptions", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list subscriptions"})
		return
	}

	out := make([]api.SubscriptionResponse, 0, len(result.Subscriptions))
	for _, sub := range result.Subscriptions {
		out = append(out, subscriptionToResponse(sub))
	}
	httputil.WriteJSON(w, http.StatusOK, api.SubscriptionListResponse{
		Subscriptions: &out,
		Count:         ptr(len(result.Subscriptions)),
	})
}

// GetSubscription implements ServerInterface.GetSubscription (GET /subscriptions/{subscriptionId})
func (s *APIServer) GetSubscription(w http.ResponseWriter, r *http.Request, subscriptionId string) {
	log := middleware.GetLogger(r, s.logger)

	result, err := s.getSubscriptionService().Get(subscriptionId)
	if err != nil {
		mapSubscriptionGetError(w, log, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, subscriptionToResponse(result.Subscription))
}

// UpdateSubscription implements ServerInterface.UpdateSubscription (PUT /subscriptions/{subscriptionId})
func (s *APIServer) UpdateSubscription(w http.ResponseWriter, r *http.Request, subscriptionId string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	// Existence is checked before the body is bound so an update against an
	// unknown subscription still answers 404 rather than 400 when the body is
	// also malformed, exactly as this handler did before the service layer
	// existed. Update re-reads the row; that second primary-key lookup is the
	// price of keeping the status codes identical.
	if _, err := s.getSubscriptionService().Get(subscriptionId); err != nil {
		mapSubscriptionGetError(w, log, err)
		return
	}

	var req api.SubscriptionUpdateRequest
	if err := s.bindRequestBody(r, &req); err != nil {
		log.Warn("Invalid subscription update body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Invalid request body"})
		return
	}

	result, err := s.getSubscriptionService().Update(subscription.UpdateParams{
		ID:            subscriptionId,
		Request:       req,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		mapSubscriptionUpdateError(w, log, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, subscriptionToResponse(result.Subscription))
}

// DeleteSubscription implements ServerInterface.DeleteSubscription (DELETE /subscriptions/{subscriptionId})
func (s *APIServer) DeleteSubscription(w http.ResponseWriter, r *http.Request, subscriptionId string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	if err := s.getSubscriptionService().Delete(subscriptionId, correlationID, log); err != nil {
		mapSubscriptionDeleteError(w, log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// mapCreateSubscriptionError reproduces the responses POST /subscriptions
// returned before the service layer existed, including the two that identifier
// resolution used to write for itself.
func mapCreateSubscriptionError(w http.ResponseWriter, log *slog.Logger, apiIdentifier string, err error) {
	var validationErr *subscription.ValidationError
	if errors.As(err, &validationErr) {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: validationErr.Message})
		return
	}

	var notRestAPI *subscription.NotRestAPIError
	if errors.As(err, &notRestAPI) {
		log.Warn("Configuration is not a REST API",
			slog.String("id", apiIdentifier),
			slog.String("kind", notRestAPI.Kind))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Configuration with identifier '" + apiIdentifier + "' is not a REST API",
		})
		return
	}
	if errors.Is(err, subscription.ErrAPINotFound) {
		log.Warn("API configuration not found", slog.String("handle_or_id", apiIdentifier))
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: "RestAPI with identifier '" + apiIdentifier + "' not found",
		})
		return
	}

	var opErr *subscription.OpError
	if errors.As(err, &opErr) {
		switch opErr.Op {
		case subscription.OpResolve:
			log.Error("Failed to look up API configuration", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to resolve API identifier"})
			return
		case subscription.OpValidate:
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to validate subscription plan"})
			return
		}
	}

	if storage.IsConflictError(err) {
		httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: "Application already subscribed to this API"})
		return
	}

	log.Error("Failed to save subscription", slog.Any("error", err))
	httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create subscription"})
}

// mapSubscriptionGetError reproduces the responses a subscription read returned
// before the service layer existed. It also serves the existence pre-check in
// UpdateSubscription, which reported the same pair of failures.
func mapSubscriptionGetError(w http.ResponseWriter, log *slog.Logger, err error) {
	if errors.Is(err, subscription.ErrSubscriptionNotFound) {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
		return
	}

	log.Error("Failed to get subscription", slog.Any("error", err))
	httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription"})
}

// mapSubscriptionUpdateError reproduces the responses PUT
// /subscriptions/{subscriptionId} returned before the service layer existed. A
// failure to re-read the row is reported as a read failure, not an update
// failure, which is the distinction the original handler drew.
func mapSubscriptionUpdateError(w http.ResponseWriter, log *slog.Logger, err error) {
	var validationErr *subscription.ValidationError
	if errors.As(err, &validationErr) {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: validationErr.Message})
		return
	}
	if errors.Is(err, subscription.ErrSubscriptionNotFound) {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
		return
	}

	var opErr *subscription.OpError
	if errors.As(err, &opErr) && opErr.Op == subscription.OpLoad {
		log.Error("Failed to get subscription for update", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription"})
		return
	}

	log.Error("Failed to update subscription", slog.Any("error", err))
	httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update subscription"})
}

// mapSubscriptionDeleteError reproduces the responses DELETE
// /subscriptions/{subscriptionId} returned before the service layer existed.
// The original handler read the row first and reported a read failure
// distinctly from a delete failure.
func mapSubscriptionDeleteError(w http.ResponseWriter, log *slog.Logger, err error) {
	if errors.Is(err, subscription.ErrSubscriptionNotFound) {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: "Subscription not found"})
		return
	}

	var opErr *subscription.OpError
	if errors.As(err, &opErr) && opErr.Op == subscription.OpLoad {
		log.Error("Failed to get subscription for deletion", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to get subscription"})
		return
	}

	log.Error("Failed to delete subscription", slog.Any("error", err))
	httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to delete subscription"})
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
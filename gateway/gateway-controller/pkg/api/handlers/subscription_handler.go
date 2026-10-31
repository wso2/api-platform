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
	"io"
	"log/slog"
	"net/http"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/subscription"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// maxSubscriptionRequestBodyBytes caps subscription create/update request bodies.
const maxSubscriptionRequestBodyBytes = 64 << 10

// maxExternalRefIDLength matches the width of the subscriptions.uuid column.
const maxExternalRefIDLength = subscription.MaxExternalRefIDLength

// subscriptionConflictMessage is returned for every 409 on subscription create and update.
const subscriptionConflictMessage = "The subscription conflicts with an existing subscription"

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

	result, err := s.getSubscriptionService().Create(subscription.CreateParams{
		Request:       req,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		var validationErr *subscription.ValidationError
		var opErr *subscription.OpError
		switch {
		case errors.As(err, &validationErr):
			writeSubscriptionError(w, http.StatusBadRequest, validationErr.Message)
		case isAPIIdentifierError(err):
			writeAPIIdentifierError(w, req.ApiId, log, err)
		case errors.As(err, &opErr) && opErr.Op == subscription.OpValidate:
			writeSubscriptionError(w, http.StatusInternalServerError, "Failed to validate subscription plan")
		case storage.IsConflictError(err):
			log.Warn("Subscription create rejected due to a conflict", slog.Any("error", storageCause(err)))
			writeSubscriptionError(w, http.StatusConflict, subscriptionConflictMessage)
		default:
			log.Error("Failed to save subscription", slog.Any("error", storageCause(err)))
			writeSubscriptionError(w, http.StatusInternalServerError, "Failed to create subscription")
		}
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, subscriptionToResponseWithToken(result.Subscription))
}

// ListSubscriptions implements ServerInterface.ListSubscriptions (GET /subscriptions)
func (s *APIServer) ListSubscriptions(w http.ResponseWriter, r *http.Request, params api.ListSubscriptionsParams) {
	log := middleware.GetLogger(r, s.logger)

	// apiId is an optional filter. When omitted, all subscriptions for this gateway are returned
	// (optionally filtered by applicationId and/or status).
	var filter subscription.ListFilter
	if params.ApiId != nil && *params.ApiId != "" {
		// Normalize apiId to the internal deployment ID (accepts handle or deployment ID).
		resolvedID, err := s.resolveAPIIDByHandle(w, r, *params.ApiId, log)
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
		st := string(*params.Status)
		filter.Status = &st
	}

	result, err := s.getSubscriptionService().List(filter)
	if err != nil {
		log.Error("Failed to list subscriptions", slog.Any("error", storageCause(err)))
		writeSubscriptionError(w, http.StatusInternalServerError, "Failed to list subscriptions")
		return
	}
	out := make([]api.SubscriptionResponse, 0, len(result.Subscriptions))
	for _, sub := range result.Subscriptions {
		out = append(out, subscriptionToResponse(sub))
	}
	httputil.WriteJSON(w, http.StatusOK, api.SubscriptionListResponse{
		Subscriptions: &out,
		Count:         ptr(int(len(result.Subscriptions))),
	})
}

// GetSubscription implements ServerInterface.GetSubscription (GET /subscriptions/{subscriptionId})
func (s *APIServer) GetSubscription(w http.ResponseWriter, r *http.Request, subscriptionId string) {
	log := middleware.GetLogger(r, s.logger)

	result, err := s.getSubscriptionService().Get(subscriptionId)
	if err != nil {
		writeSubscriptionLoadError(w, log, err, "Failed to get subscription")
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

	svc := s.getSubscriptionService()
	existing, err := svc.Get(subscriptionId)
	if err != nil {
		writeSubscriptionLoadError(w, log, err, "Failed to get subscription for update")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSubscriptionRequestBodyBytes)
	var req api.SubscriptionUpdateRequest
	fields, err := s.bindRequestBodyWithFields(r, &req)
	if err != nil {
		log.Warn("Invalid subscription update body", slog.Any("error", err))
		writeSubscriptionBodyError(w, err)
		return
	}

	result, err := svc.Update(subscription.UpdateParams{
		Existing:      existing.Subscription,
		Request:       req,
		Fields:        fields,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		var validationErr *subscription.ValidationError
		var opErr *subscription.OpError
		switch {
		case errors.Is(err, subscription.ErrSubscriptionNotFound):
			writeSubscriptionError(w, http.StatusNotFound, "Subscription not found")
		case errors.As(err, &validationErr):
			writeSubscriptionError(w, http.StatusBadRequest, validationErr.Message)
		case errors.As(err, &opErr) && opErr.Op == subscription.OpValidate:
			writeSubscriptionError(w, http.StatusInternalServerError, "Failed to validate subscription plan")
		case storage.IsConflictError(err):
			log.Warn("Subscription update rejected due to a conflict", slog.Any("error", storageCause(err)))
			writeSubscriptionError(w, http.StatusConflict, subscriptionConflictMessage)
		default:
			log.Error("Failed to update subscription", slog.Any("error", storageCause(err)))
			writeSubscriptionError(w, http.StatusInternalServerError, "Failed to update subscription")
		}
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
		var opErr *subscription.OpError
		switch {
		case errors.Is(err, subscription.ErrSubscriptionNotFound):
			writeSubscriptionError(w, http.StatusNotFound, "Subscription not found")
		case errors.As(err, &opErr) && opErr.Op == subscription.OpLoad:
			log.Error("Failed to get subscription for deletion", slog.Any("error", storageCause(err)))
			writeSubscriptionError(w, http.StatusInternalServerError, "Failed to get subscription")
		default:
			log.Error("Failed to delete subscription", slog.Any("error", storageCause(err)))
			writeSubscriptionError(w, http.StatusInternalServerError, "Failed to delete subscription")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeSubscriptionLoadError writes the response for a failed subscription read: 404 when it does
// not exist; otherwise it logs logMessage and returns 500.
func writeSubscriptionLoadError(w http.ResponseWriter, log *slog.Logger, err error, logMessage string) {
	if errors.Is(err, subscription.ErrSubscriptionNotFound) {
		writeSubscriptionError(w, http.StatusNotFound, "Subscription not found")
		return
	}
	log.Error(logMessage, slog.Any("error", storageCause(err)))
	writeSubscriptionError(w, http.StatusInternalServerError, "Failed to get subscription")
}

// writeSubscriptionError writes an error response with the given status and message.
func writeSubscriptionError(w http.ResponseWriter, status int, message string) {
	httputil.WriteJSON(w, status, api.ErrorResponse{Status: "error", Message: message})
}

// storageCause returns the storage error a service OpError wraps, so a log line carries the same
// error the handler logged before the service layer existed.
func storageCause(err error) error {
	var opErr *subscription.OpError
	if errors.As(err, &opErr) && opErr.Cause != nil {
		return opErr.Cause
	}
	return err
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

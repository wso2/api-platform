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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	api "github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/go-httpkit/httputil"
)

// SubscriptionHandler handles application-level subscription CRUD
type SubscriptionHandler struct {
	subscriptionService     *service.SubscriptionService
	subscriptionPlanService *service.SubscriptionPlanService
	identity                *service.IdentityService
	slogger                 *slog.Logger
}

// NewSubscriptionHandler creates a new subscription handler
func NewSubscriptionHandler(subscriptionService *service.SubscriptionService, subscriptionPlanService *service.SubscriptionPlanService, identity *service.IdentityService, slogger *slog.Logger) *SubscriptionHandler {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &SubscriptionHandler{
		subscriptionService:     subscriptionService,
		subscriptionPlanService: subscriptionPlanService,
		identity:                identity,
		slogger:                 slogger,
	}
}

// CreateSubscriptionRequest is the body for POST /api/v0.9/subscriptions
type CreateSubscriptionRequest struct {
	APIID              string  `json:"apiId" binding:"required"`
	Kind               string  `json:"kind" binding:"required"`
	SubscriberID       string  `json:"subscriberId" binding:"required"`
	ApplicationID      *string `json:"applicationId,omitempty"`
	SubscriptionPlanID *string `json:"subscriptionPlanId,omitempty"`
	Status             string  `json:"status,omitempty"`
}

// CreateSubscription handles POST /api/v0.9/subscriptions
func (h *SubscriptionHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token when creating subscription")
	}
	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid create subscription request body for org %s", orgId))
	}
	if req.APIID == "" {
		return apperror.ValidationFailed.New("API ID is required")
	}
	if req.SubscriberID == "" {
		return apperror.ValidationFailed.New("subscriberId is required")
	}
	if req.Kind == "" {
		return apperror.ValidationFailed.New("kind is required")
	}
	if !constants.ValidArtifactKinds[req.Kind] {
		return apperror.ValidationFailed.New("Invalid kind value")
	}
	switch req.Status {
	case "", "ACTIVE", "INACTIVE", "REVOKED":
	default:
		return apperror.ValidationFailed.New("Invalid status value")
	}
	actor, err := resolveActorErr(r, h.identity, "create subscription")
	if err != nil {
		return err
	}
	sub, err := h.subscriptionService.CreateSubscription(req.APIID, req.Kind, orgId, req.SubscriberID, req.ApplicationID, req.SubscriptionPlanID, "", req.Status, actor)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.ArtifactNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrSubscriptionAlreadyExists) {
			return apperror.SubscriptionExists.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to create subscription for api %s in org %s", req.APIID, orgId))
	}
	resp, err := h.toSubscriptionResponse(sub, orgId)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to resolve subscription identity for api %s in org %s", req.APIID, orgId))
	}
	setLocation(w, "subscriptions", sub.UUID)
	httputil.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

// ListSubscriptions handles GET /api/v0.9/subscriptions
func (h *SubscriptionHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) error {
	apiId := r.URL.Query().Get("apiId")
	subscriberID := r.URL.Query().Get("subscriberId")
	applicationID := r.URL.Query().Get("applicationId")
	status := r.URL.Query().Get("status")

	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var apiIDPtr, subscriberIDPtr, appIDPtr, statusPtr *string
	if apiId != "" {
		apiIDPtr = &apiId
	}
	if subscriberID != "" {
		subscriberIDPtr = &subscriberID
	}
	if applicationID != "" {
		appIDPtr = &applicationID
	}
	if status != "" {
		switch status {
		case "ACTIVE", "INACTIVE", "REVOKED":
		default:
			return apperror.ValidationFailed.New("Invalid status value")
		}
		statusPtr = &status
	}
	var limitStr string
	if v := r.URL.Query().Get("limit"); v != "" {
		limitStr = v
	} else {
		limitStr = "20"
	}
	var offsetStr string
	if v := r.URL.Query().Get("offset"); v != "" {
		offsetStr = v
	} else {
		offsetStr = "0"
	}
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
	list, total, err := h.subscriptionService.ListSubscriptionsByFilters(orgId, apiIDPtr, subscriberIDPtr, appIDPtr, statusPtr, limit, offset)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.ArtifactNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to list subscriptions for api %s in org %s", apiId, orgId))
	}
	// Bulk fetch API handles and plan names to avoid N+1 queries
	apiUUIDSet := make(map[string]struct{})
	planIDSet := make(map[string]struct{})
	for _, sub := range list {
		if sub.ArtifactUUID != "" {
			apiUUIDSet[sub.ArtifactUUID] = struct{}{}
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
	artifactMetaMap, err := h.subscriptionService.GetArtifactMetadataMap(apiUUIDs, orgId)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to bulk fetch artifact metadata for list in org %s", orgId))
	}
	planNameMap, err := h.subscriptionPlanService.GetPlanNameMap(planIDs, orgId)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to bulk fetch plan names for list in org %s", orgId))
	}
	// Bulk-resolve createdBy UUIDs to their raw identity to avoid N+1 lookups.
	createdByUUIDs := make([]string, 0, len(list))
	for _, sub := range list {
		createdByUUIDs = append(createdByUUIDs, sub.CreatedBy)
	}
	createdByMap, err := h.identity.SubsForUUIDs(createdByUUIDs)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to resolve subscription creator identities in org %s", orgId))
	}
	items := make([]map[string]any, 0, len(list))
	for _, sub := range list {
		items = append(items, h.toSubscriptionResponseWithMaps(sub, orgId, artifactMetaMap, planNameMap, createdByMap))
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"subscriptions": items,
		"count":         len(items),
		"pagination": map[string]any{
			"total":  total,
			"offset": offset,
			"limit":  limit,
		},
	})
	return nil
}

// GetSubscription handles GET /api/v0.9/subscriptions/:subscriptionId
func (h *SubscriptionHandler) GetSubscription(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	subscriptionId := r.PathValue("subscriptionId")
	if subscriptionId == "" {
		return apperror.ValidationFailed.New("Subscription ID is required")
	}
	sub, err := h.subscriptionService.GetSubscription(subscriptionId, orgId)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			return apperror.SubscriptionNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to get subscription %s in org %s", subscriptionId, orgId))
	}
	resp, err := h.toSubscriptionResponse(sub, orgId)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to resolve subscription identity for subscription %s in org %s", subscriptionId, orgId))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// UpdateSubscription handles PUT /api/v0.9/subscriptions/:subscriptionId
func (h *SubscriptionHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	subscriptionId := r.PathValue("subscriptionId")
	if subscriptionId == "" {
		return apperror.ValidationFailed.New("Subscription ID is required")
	}
	var req api.Subscription
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body")
	}
	var status string
	if req.Status != nil {
		status = string(*req.Status)
	}
	switch status {
	case "", "ACTIVE", "INACTIVE", "REVOKED":
	default:
		return apperror.ValidationFailed.New("Invalid subscription status")
	}
	subscriberID, err := requireSubscriptionSubscriberQuery(r)
	if err != nil {
		return err
	}
	actor, err := resolveActorErr(r, h.identity, "update subscription")
	if err != nil {
		return err
	}
	sub, err := h.subscriptionService.UpdateSubscription(subscriptionId, orgId, subscriberID, status, actor)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			return apperror.SubscriptionNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrSubscriptionSubscriberMismatch) {
			return apperror.SubscriptionForbidden.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to update subscription %s in org %s", subscriptionId, orgId))
	}
	resp, err := h.toSubscriptionResponse(sub, orgId)
	if err != nil {
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to resolve subscription identity for subscription %s in org %s", subscriptionId, orgId))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// DeleteSubscription handles DELETE /api/v0.9/subscriptions/:subscriptionId
func (h *SubscriptionHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	subscriptionId := r.PathValue("subscriptionId")
	if subscriptionId == "" {
		return apperror.ValidationFailed.New("Subscription ID is required")
	}
	subscriberID, err := requireSubscriptionSubscriberQuery(r)
	if err != nil {
		return err
	}
	actor, err := resolveActorErr(r, h.identity, "delete subscription")
	if err != nil {
		return err
	}
	if err := h.subscriptionService.DeleteSubscription(subscriptionId, orgId, subscriberID, actor); err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			return apperror.SubscriptionNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrSubscriptionSubscriberMismatch) {
			return apperror.SubscriptionForbidden.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to delete subscription %s in org %s", subscriptionId, orgId))
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func requireSubscriptionSubscriberQuery(r *http.Request) (string, error) {
	q := strings.TrimSpace(r.URL.Query().Get("subscriberId"))
	if q == "" {
		return "", apperror.ValidationFailed.New("subscriberId query parameter is required")
	}
	return q, nil
}

func (h *SubscriptionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/subscriptions", middleware.MapErrors(h.slogger, h.CreateSubscription))
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscriptions", middleware.MapErrors(h.slogger, h.ListSubscriptions))
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscriptions/{subscriptionId}", middleware.MapErrors(h.slogger, h.GetSubscription))
	mux.HandleFunc("PUT "+constants.APIBasePath+"/subscriptions/{subscriptionId}", middleware.MapErrors(h.slogger, h.UpdateSubscription))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/subscriptions/{subscriptionId}", middleware.MapErrors(h.slogger, h.DeleteSubscription))
}

func (h *SubscriptionHandler) toSubscriptionResponse(sub *model.Subscription, orgId string) (map[string]any, error) {
	// apiId in response should be the handle (e.g. "samp1"), not the internal UUID
	apiIdForResponse, kind := h.subscriptionService.ResolveArtifactHandleAndKind(sub.ArtifactUUID, orgId)
	if apiIdForResponse == "" {
		apiIdForResponse = sub.ArtifactUUID // fallback to UUID
	}
	createdBy, err := h.identity.SubForUUID(sub.CreatedBy)
	if err != nil {
		return nil, err
	}
	updatedBy, err := h.identity.SubForUUID(sub.UpdatedBy)
	if err != nil {
		return nil, err
	}
	resp := map[string]any{
		"id":             sub.UUID,
		"apiId":          apiIdForResponse,
		"subscriberId":   sub.SubscriberID,
		"organizationId": h.subscriptionService.ResolveOrgHandle(sub.OrganizationUUID),
		"status":         string(sub.Status),
		"createdBy":      createdBy,
		"updatedBy":      updatedBy,
		"createdAt":      sub.CreatedAt,
		"updatedAt":      sub.UpdatedAt,
	}
	if kind != "" {
		resp["kind"] = kind
	}
	if sub.ApplicationID != nil {
		resp["applicationId"] = *sub.ApplicationID
	}
	if sub.SubscriptionPlanID != nil {
		resp["subscriptionPlanId"] = *sub.SubscriptionPlanID
		if h.subscriptionPlanService != nil {
			plan, err := h.subscriptionPlanService.GetPlan(*sub.SubscriptionPlanID, orgId)
			if err == nil && plan != nil {
				resp["subscriptionPlanName"] = plan.Name
			}
		}
	}
	// subscriptionToken is decrypted from DB; empty for legacy hashed tokens
	if sub.SubscriptionToken != "" {
		resp["subscriptionToken"] = sub.SubscriptionToken
	}
	return resp, nil
}

// toSubscriptionResponseWithMaps builds a subscription response using pre-fetched lookup maps.
// Used by ListSubscriptions to avoid N+1 queries.
func (h *SubscriptionHandler) toSubscriptionResponseWithMaps(sub *model.Subscription, orgId string, artifactMetaMap map[string]*model.APIMetadata, planNameMap map[string]string, createdByMap map[string]string) map[string]any {
	apiIdForResponse := sub.ArtifactUUID // fallback to UUID
	var kind string
	if meta := artifactMetaMap[sub.ArtifactUUID]; meta != nil {
		if meta.Handle != "" {
			apiIdForResponse = meta.Handle
		}
		kind = meta.Kind
	}
	createdBy := createdByMap[sub.CreatedBy]
	if createdBy == "" {
		createdBy = constants.DeletedUser
	}
	resp := map[string]any{
		"id":             sub.UUID,
		"apiId":          apiIdForResponse,
		"subscriberId":   sub.SubscriberID,
		"organizationId": h.subscriptionService.ResolveOrgHandle(sub.OrganizationUUID),
		"status":         string(sub.Status),
		"createdBy":      createdBy,
		"createdAt":      sub.CreatedAt,
		"updatedAt":      sub.UpdatedAt,
	}
	if kind != "" {
		resp["kind"] = kind
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

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
	"strings"

	api "github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/api-platform/httpkit/httputil"
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
	ArtifactID         string  `json:"artifactId" binding:"required"`
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
	if req.ArtifactID == "" {
		return apperror.ValidationFailed.New("artifactId is required")
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
	sub, err := h.subscriptionService.CreateSubscription(req.ArtifactID, req.Kind, orgId, req.SubscriberID, req.ApplicationID, req.SubscriptionPlanID, "", req.Status, actor)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return serviceError(err, fmt.Sprintf("failed to create subscription for artifact %s in org %s", req.ArtifactID, orgId))
	}
	resp, err := h.toSubscriptionResponse(sub, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to resolve subscription identity for artifact %s in org %s", req.ArtifactID, orgId))
	}
	// The token is issued here. sub.SubscriptionToken holds the value generated in-process
	// by the repository; it is returned to the creator, and afterwards only to the
	// subscription's own subscriber (see addOwnerSubscriptionToken).
	if sub.SubscriptionToken != "" {
		resp["subscriptionToken"] = sub.SubscriptionToken
	}
	setLocation(w, "subscriptions", sub.UUID)
	httputil.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

// ListSubscriptions handles GET /api/v0.9/subscriptions
func (h *SubscriptionHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) error {
	artifactId := r.URL.Query().Get("artifactId")
	subscriberID := r.URL.Query().Get("subscriberId")
	applicationID := r.URL.Query().Get("applicationId")
	status := r.URL.Query().Get("status")

	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var artifactIDPtr, subscriberIDPtr, appIDPtr, statusPtr *string
	if artifactId != "" {
		artifactIDPtr = &artifactId
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
	limit, offset := parsePagination(r)
	list, total, err := h.subscriptionService.ListSubscriptionsByFilters(orgId, artifactIDPtr, subscriberIDPtr, appIDPtr, statusPtr, limit, offset)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return serviceError(err, fmt.Sprintf("failed to list subscriptions for artifact %s in org %s", artifactId, orgId))
	}
	// Bulk fetch artifact handles and plan names to avoid N+1 queries
	artifactUUIDSet := make(map[string]struct{})
	planIDSet := make(map[string]struct{})
	for _, sub := range list {
		if sub.ArtifactUUID != "" {
			artifactUUIDSet[sub.ArtifactUUID] = struct{}{}
		}
		if sub.SubscriptionPlanID != nil && *sub.SubscriptionPlanID != "" {
			planIDSet[*sub.SubscriptionPlanID] = struct{}{}
		}
	}
	artifactUUIDs := make([]string, 0, len(artifactUUIDSet))
	for u := range artifactUUIDSet {
		artifactUUIDs = append(artifactUUIDs, u)
	}
	planIDs := make([]string, 0, len(planIDSet))
	for id := range planIDSet {
		planIDs = append(planIDs, id)
	}
	artifactMetaMap, err := h.subscriptionService.GetArtifactMetadataMap(artifactUUIDs, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to bulk fetch artifact metadata for list in org %s", orgId))
	}
	planNameMap, err := h.subscriptionPlanService.GetPlanNameMap(planIDs, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to bulk fetch plan names for list in org %s", orgId))
	}
	// Bulk-resolve createdBy UUIDs to display names to avoid N+1 lookups.
	createdByUUIDs := make([]string, 0, len(list))
	for _, sub := range list {
		createdByUUIDs = append(createdByUUIDs, sub.CreatedBy)
	}
	createdByMap, err := h.identity.DisplayNamesForUUIDs(createdByUUIDs)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to resolve subscription creator identities in org %s", orgId))
	}
	subUUIDs := make([]string, 0, len(list))
	for _, sub := range list {
		subUUIDs = append(subUUIDs, sub.UUID)
	}
	billingMap, err := h.subscriptionService.GetBillingMany(subUUIDs, orgId)
	if err != nil {
		h.slogger.Warn("Failed to read subscription billing for list", "organizationId", orgId, "error", err)
		billingMap = nil
	}
	items := make([]map[string]any, 0, len(list))
	disclosed, withheld := 0, 0
	for _, sub := range list {
		item := h.toSubscriptionResponseWithMaps(sub, orgId, artifactMetaMap, planNameMap, createdByMap)
		if b := billingMap[sub.UUID]; b != nil {
			item["billing"] = billingResponse(b)
		}
		if addOwnerSubscriptionToken(item, r, sub) {
			disclosed++
		} else if sub.SubscriptionToken != "" {
			withheld++
		}
		items = append(items, item)
	}
	// A list is summarised in one log line rather than one audit row per subscription: a
	// publisher listing an API's subscribers withholds a token on nearly every item, which
	// would bury the audit table in rows that carry no signal. The single-subscription read
	// below is the one that is recorded.
	if disclosed > 0 || withheld > 0 {
		h.slogger.Info("Served subscription tokens on a list",
			"organizationId", orgId, "performedBy", h.auditActor(r),
			"disclosed", disclosed, "withheld", withheld)
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"list":  items,
		"count": len(items),
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
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return serviceError(err, fmt.Sprintf("failed to get subscription %s in org %s", subscriptionId, orgId))
	}
	resp, err := h.toSubscriptionResponse(sub, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to resolve subscription identity for subscription %s in org %s", subscriptionId, orgId))
	}
	if sub.SubscriptionToken != "" {
		// Both outcomes are recorded. A read by someone other than the subscriber is the
		// event worth seeing: on its own it is an ordinary read of subscription metadata,
		// but repeated across subscriptions it is someone walking the organization's tokens.
		if addOwnerSubscriptionToken(resp, r, sub) {
			h.subscriptionService.RecordTokenRead(sub.UUID, orgId, h.auditActor(r))
		} else {
			h.subscriptionService.RecordTokenReadDenied(sub.UUID, orgId, h.auditActor(r))
		}
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
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return serviceError(err, fmt.Sprintf("failed to update subscription %s in org %s", subscriptionId, orgId))
	}
	resp, err := h.toSubscriptionResponse(sub, orgId)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to resolve subscription identity for subscription %s in org %s", subscriptionId, orgId))
	}
	// An update already records its own audit row, so the disclosure is not recorded twice.
	addOwnerSubscriptionToken(resp, r, sub)
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
		return serviceError(err, fmt.Sprintf("failed to delete subscription %s in org %s", subscriptionId, orgId))
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

// subscriberSystemActor is the subscriber_id the API Portal sends for a subscription
// created by a machine credential rather than a user (its SYSTEM_ACTOR, "system"). It
// identifies no caller, so it must never satisfy the ownership check below.
const subscriberSystemActor = "system"

// callerOwnsSubscription reports whether the caller behind r is the subscriber that the
// subscription belongs to. subscriptions.subscriber_id holds the owner's identity-provider
// subject — the API Portal resolves it to the user's idp_id before sending it — which is
// the same value GetActorIdentityFromRequest resolves for the caller, so the two compare
// directly without a lookup.
//
// Sentinel subscriber ids belong to no caller and never match: "system" for a
// machine-credential subscription, constants.DeletedUser for one whose user is gone.
func callerOwnsSubscription(r *http.Request, sub *model.Subscription) bool {
	if sub == nil || sub.SubscriberID == "" ||
		sub.SubscriberID == subscriberSystemActor || sub.SubscriberID == constants.DeletedUser {
		return false
	}
	identity, ok := middleware.GetActorIdentityFromRequest(r)
	return ok && identity != "" && identity == sub.SubscriberID
}

// addOwnerSubscriptionToken adds the subscription's token to resp when the caller owns the
// subscription, and leaves the key out otherwise. It reports whether the token was added.
//
// A subscription token is an invocation credential belonging to one subscriber, so a read
// scopes it to that subscriber: get, list and update return it to the owner only. Creation
// is unaffected and still returns the token it issued.
func addOwnerSubscriptionToken(resp map[string]any, r *http.Request, sub *model.Subscription) bool {
	if sub.SubscriptionToken == "" || !callerOwnsSubscription(r, sub) {
		return false
	}
	resp["subscriptionToken"] = sub.SubscriptionToken
	return true
}

// auditActor is the internal user UUID recorded against a token disclosure, as the audit
// columns elsewhere hold it. An unauthenticated caller is recorded as the empty actor
// rather than as a freshly minted UUID: a token read is not a write that must be
// attributed to someone, and minting here would fill user_idp_references with rows for
// callers that have no identity. A mapping failure is logged and treated the same way,
// because it must not fail the read it is recording.
func (h *SubscriptionHandler) auditActor(r *http.Request) string {
	identity, ok := middleware.GetActorIdentityFromRequest(r)
	if !ok || identity == "" {
		return ""
	}
	actor, err := h.identity.ToInternalUUID(identity)
	if err != nil {
		h.slogger.Warn("Failed to resolve the caller's identity for a subscription token audit record", "error", err)
		return ""
	}
	return actor
}

func (h *SubscriptionHandler) RegisterRoutes(mux router.Router) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/subscriptions", middleware.MapErrors(h.slogger, h.CreateSubscription))
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscriptions", middleware.MapErrors(h.slogger, h.ListSubscriptions))
	mux.HandleFunc("GET "+constants.APIBasePath+"/subscriptions/{subscriptionId}", middleware.MapErrors(h.slogger, h.GetSubscription))
	mux.HandleFunc("PUT "+constants.APIBasePath+"/subscriptions/{subscriptionId}", middleware.MapErrors(h.slogger, h.UpdateSubscription))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/subscriptions/{subscriptionId}", middleware.MapErrors(h.slogger, h.DeleteSubscription))
}

func (h *SubscriptionHandler) toSubscriptionResponse(sub *model.Subscription, orgId string) (map[string]any, error) {
	// artifactId in response should be the handle (e.g. "samp1"), not the internal UUID
	artifactIdForResponse, kind := h.subscriptionService.ResolveArtifactHandleAndKind(sub.ArtifactUUID, orgId)
	if artifactIdForResponse == "" {
		artifactIdForResponse = sub.ArtifactUUID // fallback to UUID
	}
	createdBy, err := h.identity.DisplayNameForUUID(sub.CreatedBy)
	if err != nil {
		return nil, err
	}
	updatedBy, err := h.identity.DisplayNameForUUID(sub.UpdatedBy)
	if err != nil {
		return nil, err
	}
	resp := map[string]any{
		"id":             sub.UUID,
		"artifactId":     artifactIdForResponse,
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
	// subscriptionToken is deliberately absent here: it is a credential, not a field of
	// the subscription. CreateSubscription adds it for the subscription it just issued,
	// and a read adds it through addOwnerSubscriptionToken, which gives it to the
	// subscription's own subscriber only.
	h.addBilling(resp, sub, orgId)
	return resp, nil
}

// toSubscriptionResponseWithMaps builds a subscription response using pre-fetched lookup maps.
// Used by ListSubscriptions to avoid N+1 queries.
func (h *SubscriptionHandler) toSubscriptionResponseWithMaps(sub *model.Subscription, orgId string, artifactMetaMap map[string]*model.APIMetadata, planNameMap map[string]string, createdByMap map[string]string) map[string]any {
	artifactIdForResponse := sub.ArtifactUUID // fallback to UUID
	var kind string
	if meta := artifactMetaMap[sub.ArtifactUUID]; meta != nil {
		if meta.Handle != "" {
			artifactIdForResponse = meta.Handle
		}
		kind = meta.Kind
	}
	createdBy := createdByMap[sub.CreatedBy]
	if createdBy == "" {
		createdBy = constants.DeletedUser
	}
	resp := map[string]any{
		"id":             sub.UUID,
		"artifactId":     artifactIdForResponse,
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
	// subscriptionToken is deliberately absent here; ListSubscriptions adds it per item
	// through addOwnerSubscriptionToken. See toSubscriptionResponse.
	return resp
}

// billingResponse is the read-only `billing` of a paid subscription.
func billingResponse(b *model.SubscriptionBilling) map[string]any {
	return map[string]any{
		"status":         b.Status,
		"customerId":     b.CustomerID,
		"subscriptionId": b.SubscriptionID,
	}
}

// addBilling adds `billing` to a subscription response when the subscription is paid. A read
// failure leaves it out rather than failing the response.
func (h *SubscriptionHandler) addBilling(resp map[string]any, sub *model.Subscription, orgID string) {
	billing, err := h.subscriptionService.GetBillingMany([]string{sub.UUID}, orgID)
	if err != nil {
		h.slogger.Warn("Failed to read subscription billing", "subscriptionId", sub.UUID, "error", err)
		return
	}
	if b := billing[sub.UUID]; b != nil {
		resp["billing"] = billingResponse(b)
	}
}

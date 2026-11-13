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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/subscription"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// UnmarshalJSON decodes a spec and also records the keys it carried, as
// bindRequestBodyWithFields does for the REST API. The pointer fields alone
// cannot tell a key that was left out from one sent as null, and on update the
// two mean different things: keep the stored value, or remove it.
func (s *subscriptionSpec) UnmarshalJSON(data []byte) error {
	type plain subscriptionSpec
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var sent map[string]interface{}
	if err := json.Unmarshal(data, &sent); err != nil {
		return err
	}
	*s = subscriptionSpec(decoded)
	s.sent = sent
	return nil
}

// sentFields returns the spec's keys, in the form the service reads, to tell a
// field that was sent, from one that was left out. A spec built in code rather
// than decoded from JSON has no record of its keys, so its non-nil fields count
// as sent.
func (s *subscriptionSpec) sentFields() map[string]interface{} {
	if s.sent != nil {
		return s.sent
	}
	fields := map[string]interface{}{}
	if body, err := json.Marshal(s); err == nil {
		_ = json.Unmarshal(body, &fields)
	}
	return fields
}

// Calls SubscriptionService directly, exactly as subscription_handler.go and
// subscription_plan_handler.go do, and renders results through the same
// response builders those handlers use.
func (h *McpHandler) manageSubscriptions(ctx context.Context, _ *mcp.CallToolRequest, in manageSubscriptionsInput) (*mcp.CallToolResult, any, error) {
	op, err := resolveSubscriptionAction(in.Type, in.Action, in.ID)
	if err != nil {
		return nil, nil, err
	}
	if op.Mutating && h.immutable {
		return nil, nil, fmt.Errorf(
			"this Gateway runs in immutable mode; subscriptions are loaded at startup and cannot be changed at runtime")
	}
	if op.NeedsConfirm && !in.Confirm {
		return nil, nil, fmt.Errorf(
			"refusing to delete %s %q: confirm the id with the user, then retry with confirm=true",
			op.Type, in.ID)
	}
	if err := h.authz.authorize(ctx, op.RouteKey); err != nil {
		return nil, nil, err
	}

	if op.NeedsID && strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf(
			`%s requires "id"; call this tool with action=list to find it`, op.Action)
	}

	correlationID := uuid.NewString()
	log := h.logger.With(
		slog.String("correlation_id", correlationID),
		slog.String("source", "mcp"),
		slog.String("tool", toolManageSubscriptions),
		slog.String("type", op.Type),
		slog.String("action", op.Action))

	if op.Type == subTypePlan {
		return h.manageSubscriptionPlan(op, in, correlationID, log)
	}
	return h.manageSubscription(op, in, correlationID, log)
}

func (h *McpHandler) manageSubscription(op subAction, in manageSubscriptionsInput, correlationID string, log *slog.Logger) (*mcp.CallToolResult, any, error) {
	svc := h.subscriptionService

	switch op.Action {
	case subActionList:
		filter := subscription.ListFilter{}
		if spec := in.Spec; spec != nil {
			if spec.ApiId != nil && *spec.ApiId != "" {
				resolved, err := svc.ResolveAPIID(*spec.ApiId)
				if err != nil {
					log.Warn("MCP subscription list filter could not be resolved",
						slog.String("api_id", *spec.ApiId), slog.Any("error", err))
					return nil, nil, mcpSubscriptionError(op, *spec.ApiId, err)
				}
				filter.APIID = resolved
			}
			if spec.ApplicationId != nil && *spec.ApplicationId != "" {
				filter.ApplicationID = spec.ApplicationId
			}
			if status := normalizeSubscriptionStatus(spec.Status); status != nil {
				filter.Status = status
			}
		}

		result, err := svc.List(filter)
		if err != nil {
			log.Error("MCP subscription listing failed", slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, "", err)
		}
		items := make([]api.SubscriptionResponse, 0, len(result.Subscriptions))
		for _, sub := range result.Subscriptions {
			items = append(items, subscriptionToResponse(sub))
		}
		return nil, map[string]any{
			"type": op.Type, "count": len(items), "items": items,
		}, nil

	case subActionGet:
		result, err := svc.Get(in.ID)
		if err != nil {
			log.Warn("MCP subscription lookup failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		return nil, map[string]any{
			"type": op.Type, "id": in.ID, "resource": subscriptionToResponse(result.Subscription),
		}, nil

	case subActionDelete:
		if err := svc.Delete(in.ID, correlationID, log); err != nil {
			log.Error("MCP subscription deletion failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		return nil, map[string]any{
			"status": "success", "type": op.Type, "id": in.ID,
			"message": fmt.Sprintf("Subscription %q deleted successfully", in.ID),
		}, nil
	}

	// apply
	spec := in.Spec
	if spec == nil {
		return nil, nil, fmt.Errorf(`apply requires "spec"`)
	}

	if strings.TrimSpace(in.ID) != "" {
		// As PUT /subscriptions/{id}: the stored subscription is read before the
		// request, so an unknown id is reported first.
		existing, err := svc.Get(in.ID)
		if err != nil {
			log.Warn("MCP subscription lookup failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		req := spec.toSubscriptionUpdate()
		if subscriptionRequestTooLarge(req) {
			return nil, nil, errSubscriptionRequestTooLarge(op)
		}
		result, err := svc.Update(subscription.UpdateParams{
			Existing: existing.Subscription, Request: req, Fields: spec.sentFields(),
			CorrelationID: correlationID, Logger: log,
		})
		if err != nil {
			log.Error("MCP subscription update failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		// Mirrors PUT /subscriptions/{id}, which never returns the token
		return nil, map[string]any{
			"status": "success", "operation": "update", "type": op.Type, "id": in.ID,
			"resource": subscriptionToResponse(result.Subscription),
			"notice": "An update changes only a subscription's status, plan and token. Its API, application, " +
				"externalRefId and billing identifiers are unchanged, whatever the spec contained.",
		}, nil
	}

	req, err := spec.toSubscriptionCreate()
	if err != nil {
		return nil, nil, err
	}
	if subscriptionRequestTooLarge(req) {
		return nil, nil, errSubscriptionRequestTooLarge(op)
	}
	result, err := svc.Create(subscription.CreateParams{
		Request: req, CorrelationID: correlationID, Logger: log,
	})
	if err != nil {
		log.Error("MCP subscription creation failed", slog.Any("error", err))
		return nil, nil, mcpSubscriptionError(op, req.ApiId, err)
	}
	return nil, map[string]any{
		"status": "success", "operation": "create", "type": op.Type,
		// Mirrors POST /subscriptions, which echoes the token the caller
		// supplied. Only its hash is stored, so this is the last time it
		// appears anywhere.
		"resource": subscriptionToResponseWithToken(result.Subscription),
		"notice": "The subscription token in this response is stored only as a hash and cannot be read " +
			"back from this Gateway. Treat it as credential material and do not repeat it.",
	}, nil
}

func (h *McpHandler) manageSubscriptionPlan(op subAction, in manageSubscriptionsInput, correlationID string, log *slog.Logger) (*mcp.CallToolResult, any, error) {
	svc := h.subscriptionService

	switch op.Action {
	case subActionList:
		result, err := svc.ListPlans()
		if err != nil {
			log.Error("MCP subscription plan listing failed", slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, "", err)
		}
		items := make([]api.SubscriptionPlanResponse, 0, len(result.Plans))
		for _, plan := range result.Plans {
			items = append(items, subscriptionPlanToResponse(plan))
		}
		return nil, map[string]any{
			"type": op.Type, "count": len(items), "items": items,
		}, nil

	case subActionGet:
		result, err := svc.GetPlan(in.ID)
		if err != nil {
			log.Warn("MCP subscription plan lookup failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		return nil, map[string]any{
			"type": op.Type, "id": in.ID, "resource": subscriptionPlanToResponse(result.Plan),
		}, nil

	case subActionDelete:
		if err := svc.DeletePlan(in.ID, correlationID, log); err != nil {
			log.Error("MCP subscription plan deletion failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		return nil, map[string]any{
			"status": "success", "type": op.Type, "id": in.ID,
			"message": fmt.Sprintf("Subscription plan %q deleted successfully", in.ID),
		}, nil
	}

	// apply
	spec := in.Spec
	if spec == nil {
		return nil, nil, fmt.Errorf(`apply requires "spec"`)
	}

	if strings.TrimSpace(in.ID) != "" {
		// As PUT /subscription-plans/{id}: the stored plan is read before the
		// request, so an unknown id is reported first.
		existing, err := svc.GetPlan(in.ID)
		if err != nil {
			log.Warn("MCP subscription plan lookup failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		req, err := spec.toPlanUpdate()
		if err != nil {
			return nil, nil, err
		}
		if subscriptionRequestTooLarge(req) {
			return nil, nil, errSubscriptionRequestTooLarge(op)
		}
		result, err := svc.UpdatePlan(subscription.UpdatePlanParams{
			Existing: existing.Plan, Request: req, Fields: spec.sentFields(),
			CorrelationID: correlationID, Logger: log,
		})
		if err != nil {
			log.Error("MCP subscription plan update failed", slog.String("id", in.ID), slog.Any("error", err))
			return nil, nil, mcpSubscriptionError(op, in.ID, err)
		}
		return nil, map[string]any{
			"status": "success", "operation": "update", "type": op.Type, "id": in.ID,
			"resource": subscriptionPlanToResponse(result.Plan),
		}, nil
	}

	req, err := spec.toPlanCreate()
	if err != nil {
		return nil, nil, err
	}
	if subscriptionRequestTooLarge(req) {
		return nil, nil, errSubscriptionRequestTooLarge(op)
	}
	result, err := svc.CreatePlan(subscription.CreatePlanParams{
		Request: req, CorrelationID: correlationID, Logger: log,
	})
	if err != nil {
		log.Error("MCP subscription plan creation failed", slog.Any("error", err))
		return nil, nil, mcpSubscriptionError(op, req.PlanName, err)
	}
	return nil, map[string]any{
		"status": "success", "operation": "create", "type": op.Type,
		"resource": subscriptionPlanToResponse(result.Plan),
	}, nil
}

// Spec mapping
//
// Each mapper builds the generated request type the service already takes, so
// no validation is duplicated here: enum values, throttle-limit pairing and
// plan eligibility all stay in SubscriptionService, which is the single place
// the REST API validates them too. The only work done here is the conversion
// the generated types cannot express — an RFC 3339 string into *time.Time.

func (s *subscriptionSpec) toSubscriptionCreate() (api.SubscriptionCreateRequest, error) {
	req := api.SubscriptionCreateRequest{
		ExternalRefId:         s.ExternalRefId,
		ApplicationId:         s.ApplicationId,
		SubscriptionPlanId:    s.SubscriptionPlanId,
		BillingCustomerId:     s.BillingCustomerId,
		BillingSubscriptionId: s.BillingSubscriptionId,
	}
	if s.ApiId != nil {
		req.ApiId = *s.ApiId
	}
	if s.SubscriptionToken != nil {
		req.SubscriptionToken = *s.SubscriptionToken
	}
	if status := normalizeSubscriptionStatus(s.Status); status != nil {
		req.Status = ptr(api.SubscriptionCreateRequestStatus(*status))
	}
	return req, nil
}

// toSubscriptionUpdate carries a nil field through as nil: whether the key was
// left out or sent as null is read from sentFields, as PUT /subscriptions/{id}
// reads it from the body's keys.
func (s *subscriptionSpec) toSubscriptionUpdate() api.SubscriptionUpdateRequest {
	req := api.SubscriptionUpdateRequest{
		SubscriptionPlanId: s.SubscriptionPlanId,
		SubscriptionToken:  s.SubscriptionToken,
	}
	if status := normalizeSubscriptionStatus(s.Status); status != nil {
		req.Status = ptr(api.SubscriptionUpdateRequestStatus(*status))
	}
	return req
}

func (s *subscriptionSpec) toPlanCreate() (api.SubscriptionPlanCreateRequest, error) {
	expiry, err := s.planExpiry()
	if err != nil {
		return api.SubscriptionPlanCreateRequest{}, err
	}
	req := api.SubscriptionPlanCreateRequest{
		BillingPlan:        s.BillingPlan,
		StopOnQuotaReach:   s.StopOnQuotaReach,
		ThrottleLimitCount: s.ThrottleLimitCount,
		ExpiryTime:         expiry,
	}
	if s.PlanName != nil {
		req.PlanName = *s.PlanName
	}
	if unit := normalizeThrottleUnit(s.ThrottleLimitUnit); unit != nil {
		req.ThrottleLimitUnit = ptr(api.SubscriptionPlanCreateRequestThrottleLimitUnit(*unit))
	}
	if status := normalizeSubscriptionStatus(s.Status); status != nil {
		req.Status = ptr(api.SubscriptionPlanCreateRequestStatus(*status))
	}
	return req, nil
}

func (s *subscriptionSpec) toPlanUpdate() (api.SubscriptionPlanUpdateRequest, error) {
	expiry, err := s.planExpiry()
	if err != nil {
		return api.SubscriptionPlanUpdateRequest{}, err
	}
	req := api.SubscriptionPlanUpdateRequest{
		PlanName:           s.PlanName,
		BillingPlan:        s.BillingPlan,
		StopOnQuotaReach:   s.StopOnQuotaReach,
		ThrottleLimitCount: s.ThrottleLimitCount,
		ExpiryTime:         expiry,
	}
	if unit := normalizeThrottleUnit(s.ThrottleLimitUnit); unit != nil {
		req.ThrottleLimitUnit = ptr(api.SubscriptionPlanUpdateRequestThrottleLimitUnit(*unit))
	}
	if status := normalizeSubscriptionStatus(s.Status); status != nil {
		req.Status = ptr(api.SubscriptionPlanUpdateRequestStatus(*status))
	}
	return req, nil
}

// planExpiry parses spec.expiryTime. Null or absent gives nil; an empty string
// is refused rather than read as null, because the REST API rejects it as an
// invalid body.
func (s *subscriptionSpec) planExpiry() (*time.Time, error) {
	if s.ExpiryTime == nil {
		return nil, nil
	}
	if strings.TrimSpace(*s.ExpiryTime) == "" {
		return nil, fmt.Errorf(
			"spec.expiryTime is empty; use an RFC 3339 timestamp such as 2027-01-31T23:59:59Z, " +
				"or null for a plan that does not expire")
	}
	return parseTimestamp("spec.expiryTime", *s.ExpiryTime)
}

// subscriptionRequestTooLarge reports whether the REST request equivalent to
// req would exceed the body limit the subscription and plan endpoints enforce.
func subscriptionRequestTooLarge(req any) bool {
	body, err := json.Marshal(req)
	if err != nil {
		return true
	}
	return len(body) > maxSubscriptionRequestBodyBytes
}

// errSubscriptionRequestTooLarge is the refusal for a request over that limit.
// Like the REST API's 413, it does not state the limit.
func errSubscriptionRequestTooLarge(op subAction) error {
	return fmt.Errorf("the %s request is too large for this Gateway; nothing was stored", op.Type)
}

// normalizeSubscriptionStatus upper-cases a status so "active" reaches the
// service as ACTIVE. An unrecognised value is passed through untouched, so the
// service stays the one place that decides what is valid and produces the
// message naming the accepted set.
func normalizeSubscriptionStatus(raw *string) *string {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}
	return ptr(strings.ToUpper(strings.TrimSpace(*raw)))
}

// normalizeThrottleUnit resolves a throttle unit to the exact spelling the
// service accepts. As above, an unrecognised value is passed through so the
// service produces the authoritative error.
func normalizeThrottleUnit(raw *string) *string {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	canonical := map[string]string{
		"min": "Min", "minute": "Min", "hour": "Hour",
		"day": "Day", "month": "Month",
	}
	if resolved, ok := canonical[strings.ToLower(trimmed)]; ok {
		return &resolved
	}
	return &trimmed
}

// mcpSubscriptionError maps a SubscriptionService failure to a message for the
// model, following keyOpError's policy: request-shaped validation detail is
// echoed so the model can correct its spec and retry, while anything else stays
// generic with the real error in the log only.
func mcpSubscriptionError(op subAction, subject string, err error) error {
	// ValidationError.Message is written to reach the caller verbatim, and is
	// the whole reason a model can fix a rejected plan or status by itself.
	var validationErr *subscription.ValidationError
	if errors.As(err, &validationErr) {
		return fmt.Errorf("%s", validationErr.Message)
	}

	var notRestAPI *subscription.NotRestAPIError
	if errors.As(err, &notRestAPI) {
		return fmt.Errorf(
			"%q is a %s, not a RestApi. Only REST APIs bear subscriptions",
			notRestAPI.Identifier, notRestAPI.Kind)
	}
	if errors.Is(err, subscription.ErrAPINotFound) {
		return fmt.Errorf(
			"no RestApi with identifier %q on this Gateway; "+
				"call "+toolListResources+" with kind=RestApi to see what is deployed", subject)
	}
	if errors.Is(err, subscription.ErrSubscriptionNotFound) || errors.Is(err, subscription.ErrPlanNotFound) {
		return fmt.Errorf(
			"no %s with id %q; call this tool with action=list to see what exists", op.Type, subject)
	}

	var opErr *subscription.OpError
	if errors.As(err, &opErr) && opErr.Op == subscription.OpValidate {
		return fmt.Errorf("the Gateway could not validate the subscription plan")
	}

	if storage.IsConflictError(err) {
		switch {
		case op.Type == subTypeSubscription:
			// Generic for the same reason as the REST API's 409: naming the
			// clash would confirm that a token is in use.
			return fmt.Errorf("the subscription conflicts with an existing subscription")
		case op.Action == subActionDelete:
			return fmt.Errorf(
				"subscription plan %q is in use by one or more subscriptions, so it was not deleted; "+
					"move those subscriptions to another plan, or delete them, first", subject)
		case strings.HasPrefix(op.RouteKey, http.MethodPut+" "):
			return fmt.Errorf("a subscription plan with this name already exists; choose a different spec.planName")
		default:
			return fmt.Errorf("a subscription plan named %q already exists", subject)
		}
	}

	return fmt.Errorf("the Gateway could not %s the %s", op.Action, op.Type)
}

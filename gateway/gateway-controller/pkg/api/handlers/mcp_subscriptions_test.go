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
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// As in the certificate test, the expected route keys are written by hand so
// this compares the dispatch table against the routes as spelled in
// cmd/controller/main.go. The placeholders genuinely differ between the two
// collections — {subscriptionId} against {planId} — and getting one wrong fails
// closed silently, so each is asserted explicitly.
func TestResolveSubscriptionAction(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		action       string
		id           string
		wantType     string
		wantAction   string
		wantRouteKey string
		wantMutating bool
		wantNeedsID  bool
		wantConfirm  bool
	}{
		{"subscription list", "Subscription", "list", "",
			subTypeSubscription, subActionList, "GET /subscriptions", false, false, false},
		{"subscription get", "Subscription", "get", "s-1",
			subTypeSubscription, subActionGet, "GET /subscriptions/{subscriptionId}", false, true, false},
		{"subscription create", "Subscription", "apply", "",
			subTypeSubscription, subActionApply, "POST /subscriptions", true, false, false},
		{"subscription update", "Subscription", "apply", "s-1",
			subTypeSubscription, subActionApply, "PUT /subscriptions/{subscriptionId}", true, false, false},
		{"subscription delete", "Subscription", "delete", "s-1",
			subTypeSubscription, subActionDelete, "DELETE /subscriptions/{subscriptionId}", true, true, true},

		{"plan list", "SubscriptionPlan", "list", "",
			subTypePlan, subActionList, "GET /subscription-plans", false, false, false},
		{"plan get", "SubscriptionPlan", "get", "p-1",
			subTypePlan, subActionGet, "GET /subscription-plans/{planId}", false, true, false},
		{"plan create", "SubscriptionPlan", "apply", "",
			subTypePlan, subActionApply, "POST /subscription-plans", true, false, false},
		{"plan update", "SubscriptionPlan", "apply", "p-1",
			subTypePlan, subActionApply, "PUT /subscription-plans/{planId}", true, false, false},
		{"plan delete", "SubscriptionPlan", "delete", "p-1",
			subTypePlan, subActionDelete, "DELETE /subscription-plans/{planId}", true, true, true},

		// Spelling tolerance.
		{"snake case type", "subscription_plan", "list", "",
			subTypePlan, subActionList, "GET /subscription-plans", false, false, false},
		{"short type alias", "plan", "get", "p-1",
			subTypePlan, subActionGet, "GET /subscription-plans/{planId}", false, true, false},
		{"plural type", "subscriptions", "list", "",
			subTypeSubscription, subActionList, "GET /subscriptions", false, false, false},
		{"create aliases to apply", "Subscription", "create", "",
			subTypeSubscription, subActionApply, "POST /subscriptions", true, false, false},
		{"update aliases to apply, id still decides the route", "Subscription", "update", "s-1",
			subTypeSubscription, subActionApply, "PUT /subscriptions/{subscriptionId}", true, false, false},

		// An id that is only whitespace is not an id: it must route to create,
		// not to an update against an empty path segment.
		{"blank id routes to create", "Subscription", "apply", "   ",
			subTypeSubscription, subActionApply, "POST /subscriptions", true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, err := resolveSubscriptionAction(tt.resourceType, tt.action, tt.id)
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, op.Type)
			assert.Equal(t, tt.wantAction, op.Action)
			assert.Equal(t, tt.wantRouteKey, op.RouteKey)
			assert.Equal(t, tt.wantMutating, op.Mutating)
			assert.Equal(t, tt.wantNeedsID, op.NeedsID)
			assert.Equal(t, tt.wantConfirm, op.NeedsConfirm)
		})
	}
}

func TestResolveSubscriptionActionRejectsUnknown(t *testing.T) {
	_, err := resolveSubscriptionAction("Application", "list", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Subscription, SubscriptionPlan")

	_, err = resolveSubscriptionAction("Subscription", "reload", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list, get, apply, delete")
}

func TestSubscriptionGateAgreesWithHandler(t *testing.T) {
	h := &McpHandler{}

	for _, tt := range []struct{ resourceType, action, id string }{
		{"Subscription", "list", ""},
		{"Subscription", "get", "s-1"},
		{"Subscription", "apply", ""},
		{"Subscription", "apply", "s-1"},
		{"Subscription", "delete", "s-1"},
		{"SubscriptionPlan", "list", ""},
		{"SubscriptionPlan", "apply", ""},
		{"SubscriptionPlan", "apply", "p-1"},
		{"plan", "delete", "p-1"},
	} {
		t.Run(tt.resourceType+"/"+tt.action, func(t *testing.T) {
			args, err := json.Marshal(manageSubscriptionsInput{
				Type: tt.resourceType, Action: tt.action, ID: tt.id,
			})
			require.NoError(t, err)

			gateKeys, ok := h.routeKeysForCall("wso2_apip_gw_manage_subscriptions", args)
			require.True(t, ok)
			require.Len(t, gateKeys, 1, "a subscription call maps to exactly one route key")

			op, err := resolveSubscriptionAction(tt.resourceType, tt.action, tt.id)
			require.NoError(t, err)
			assert.Equal(t, op.RouteKey, gateKeys[0])
		})
	}
}

func TestManageSubscriptionsGuards(t *testing.T) {
	authorized := withMcpCaller(context.Background(), mcpCaller{Skipped: true})

	t.Run("immutable mode refuses writes but allows reads", func(t *testing.T) {
		h := &McpHandler{immutable: true, logger: slog.Default()}

		for _, action := range []string{"apply", "delete"} {
			_, _, err := h.manageSubscriptions(authorized, nil, manageSubscriptionsInput{
				Type: "Subscription", Action: action, ID: "s-1", Confirm: true,
			})
			require.Errorf(t, err, "%s should be refused in immutable mode", action)
			assert.Contains(t, err.Error(), "immutable mode")
		}

		// A read reaches past the guard; with no service wired it would panic,
		// so reaching the missing-id check proves the guard let it through.
		_, _, err := h.manageSubscriptions(authorized, nil, manageSubscriptionsInput{
			Type: "Subscription", Action: "get",
		})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "immutable mode")
	})

	t.Run("delete requires confirm", func(t *testing.T) {
		h := &McpHandler{logger: slog.Default()}
		_, _, err := h.manageSubscriptions(authorized, nil, manageSubscriptionsInput{
			Type: "SubscriptionPlan", Action: "delete", ID: "p-1",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "confirm=true")
	})

	t.Run("get and delete require an id", func(t *testing.T) {
		h := &McpHandler{logger: slog.Default()}
		for _, action := range []string{"get", "delete"} {
			_, _, err := h.manageSubscriptions(authorized, nil, manageSubscriptionsInput{
				Type: "Subscription", Action: action, Confirm: true,
			})
			require.Errorf(t, err, "%s should require an id", action)
			assert.Contains(t, err.Error(), "action=list")
		}
	})

	t.Run("apply requires a spec", func(t *testing.T) {
		h := &McpHandler{logger: slog.Default()}
		_, _, err := h.manageSubscriptions(authorized, nil, manageSubscriptionsInput{
			Type: "SubscriptionPlan", Action: "apply",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "spec")
	})
}

func TestManageSubscriptionsDeniesWithoutGate(t *testing.T) {
	h := &McpHandler{logger: slog.Default()}
	_, _, err := h.manageSubscriptions(context.Background(), nil, manageSubscriptionsInput{
		Type: "Subscription", Action: "list",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

// Spec mapping. The service owns validation, so these assert only the
// conversion the generated request types cannot express, plus the normalisation
// that keeps a lower-case value from being rejected for its casing alone.
func TestSubscriptionSpecMapping(t *testing.T) {
	t.Run("subscription create carries required fields through", func(t *testing.T) {
		spec := &subscriptionSpec{
			ApiId:              ptr("my-api"),
			SubscriptionToken:  ptr("tok-123"),
			ApplicationId:      ptr("app-1"),
			SubscriptionPlanId: ptr("plan-1"),
			Status:             ptr("active"),
		}
		req, err := spec.toSubscriptionCreate()
		require.NoError(t, err)
		assert.Equal(t, "my-api", req.ApiId)
		assert.Equal(t, "tok-123", req.SubscriptionToken)
		assert.Equal(t, "app-1", *req.ApplicationId)
		assert.Equal(t, "plan-1", *req.SubscriptionPlanId)
		require.NotNil(t, req.Status)
		assert.EqualValues(t, "ACTIVE", *req.Status, "status should reach the service upper-cased")
	})

	t.Run("subscription update refuses a spec with no status", func(t *testing.T) {
		// SubscriptionUpdateRequest carries only Status, so a spec without one
		// would report success for a call that changed nothing.
		_, err := (&subscriptionSpec{ApiId: ptr("my-api")}).toSubscriptionUpdate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status only")

		req, err := (&subscriptionSpec{Status: ptr("REVOKED")}).toSubscriptionUpdate()
		require.NoError(t, err)
		require.NotNil(t, req.Status)
		assert.EqualValues(t, "REVOKED", *req.Status)
	})

	t.Run("plan create parses expiry and normalises the throttle unit", func(t *testing.T) {
		spec := &subscriptionSpec{
			PlanName:           ptr("gold"),
			ThrottleLimitCount: ptr(100),
			ThrottleLimitUnit:  ptr("min"),
			ExpiryTime:         ptr("2027-01-31T23:59:59Z"),
			StopOnQuotaReach:   ptr(false),
		}
		req, err := spec.toPlanCreate()
		require.NoError(t, err)
		assert.Equal(t, "gold", req.PlanName)
		assert.Equal(t, 100, *req.ThrottleLimitCount)
		assert.EqualValues(t, "Min", *req.ThrottleLimitUnit)
		require.NotNil(t, req.ExpiryTime)
		assert.Equal(t, 2027, req.ExpiryTime.Year())
		assert.False(t, *req.StopOnQuotaReach)
	})

	t.Run("an unparseable expiry is an error, not a dropped field", func(t *testing.T) {
		_, err := (&subscriptionSpec{PlanName: ptr("gold"), ExpiryTime: ptr("2027-01-31")}).toPlanCreate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "spec.expiryTime")

		_, err = (&subscriptionSpec{ExpiryTime: ptr("tomorrow")}).toPlanUpdate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "RFC 3339")
	})

	t.Run("an unrecognised throttle unit reaches the service untouched", func(t *testing.T) {
		// The service is the one place that decides what is valid and produces
		// the message naming the accepted set; normalisation must not swallow
		// the value and hide that error.
		req, err := (&subscriptionSpec{PlanName: ptr("gold"), ThrottleLimitUnit: ptr("Fortnight")}).toPlanCreate()
		require.NoError(t, err)
		assert.EqualValues(t, "Fortnight", *req.ThrottleLimitUnit)
	})

	t.Run("absent optional fields stay absent", func(t *testing.T) {
		req, err := (&subscriptionSpec{PlanName: ptr("gold")}).toPlanCreate()
		require.NoError(t, err)
		assert.Nil(t, req.Status)
		assert.Nil(t, req.ExpiryTime)
		assert.Nil(t, req.ThrottleLimitUnit)
		assert.Nil(t, req.ThrottleLimitCount)
		assert.Nil(t, req.StopOnQuotaReach)
	})
}

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
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/subscription"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
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
		h := newAuthzTestHandler(true)

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
		h := newAuthzTestHandler(false)
		_, _, err := h.manageSubscriptions(authorized, nil, manageSubscriptionsInput{
			Type: "SubscriptionPlan", Action: "delete", ID: "p-1",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "confirm=true")
	})

	t.Run("get and delete require an id", func(t *testing.T) {
		h := newAuthzTestHandler(false)
		for _, action := range []string{"get", "delete"} {
			_, _, err := h.manageSubscriptions(authorized, nil, manageSubscriptionsInput{
				Type: "Subscription", Action: action, Confirm: true,
			})
			require.Errorf(t, err, "%s should require an id", action)
			assert.Contains(t, err.Error(), "action=list")
		}
	})

	t.Run("apply requires a spec", func(t *testing.T) {
		h := newAuthzTestHandler(false)
		_, _, err := h.manageSubscriptions(authorized, nil, manageSubscriptionsInput{
			Type: "SubscriptionPlan", Action: "apply",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "spec")
	})
}

func TestManageSubscriptionsDeniesWithoutGate(t *testing.T) {
	h := newAuthzTestHandler(false)
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

	t.Run("subscription create carries externalRefId through", func(t *testing.T) {
		req, err := (&subscriptionSpec{ApiId: ptr("my-api"), ExternalRefId: ptr("apim-sub-1")}).toSubscriptionCreate()
		require.NoError(t, err)
		require.NotNil(t, req.ExternalRefId)
		assert.Equal(t, "apim-sub-1", *req.ExternalRefId)
	})

	t.Run("subscription update carries status, plan and token, and nothing else", func(t *testing.T) {
		req := (&subscriptionSpec{
			ApiId: ptr("other-api"), Status: ptr("revoked"),
			SubscriptionPlanId: ptr("plan-2"), SubscriptionToken: ptr("new-token"),
		}).toSubscriptionUpdate()
		require.NotNil(t, req.Status)
		assert.EqualValues(t, "REVOKED", *req.Status)
		assert.Equal(t, "plan-2", *req.SubscriptionPlanId)
		assert.Equal(t, "new-token", *req.SubscriptionToken)

		// As PUT /subscriptions/{id}, a spec with none of the three is a valid,
		// empty update rather than an error.
		empty := (&subscriptionSpec{}).toSubscriptionUpdate()
		assert.Nil(t, empty.Status)
		assert.Nil(t, empty.SubscriptionPlanId)
		assert.Nil(t, empty.SubscriptionToken)
	})

	t.Run("an empty expiry is refused rather than read as null", func(t *testing.T) {
		_, err := (&subscriptionSpec{PlanName: ptr("gold"), ExpiryTime: ptr("")}).toPlanCreate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "spec.expiryTime is empty")

		_, err = (&subscriptionSpec{ExpiryTime: ptr("  ")}).toPlanUpdate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "spec.expiryTime is empty")
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

func toolsSubscriptionService(db storage.Storage) *subscription.SubscriptionService {
	return subscription.NewSubscriptionService(db,
		utils.NewSubscriptionResourceService(db, nil, &mockEventHub{}, "test-gateway"))
}

func subscriptionToolsHandler(db storage.Storage) *McpHandler {
	h := newAuthzTestHandler(false)
	h.logger = toolsDiscard
	h.subscriptionService = toolsSubscriptionService(db)
	return h
}

func TestManageSubscriptionPlanLifecycle(t *testing.T) {
	h := subscriptionToolsHandler(NewMockStorage())
	call := func(in manageSubscriptionsInput) (map[string]any, error) {
		in.Type = "SubscriptionPlan"
		_, out, err := h.manageSubscriptions(toolsSkippedCtx, nil, in)
		if err != nil {
			return nil, err
		}
		return resultMap(t, out), nil
	}

	res, err := call(manageSubscriptionsInput{Action: "apply", Spec: &subscriptionSpec{
		PlanName: ptr("gold"), ThrottleLimitCount: ptr(100), ThrottleLimitUnit: ptr("minute"),
	}})
	require.NoError(t, err)
	assert.Equal(t, "create", res["operation"])
	plan := toolsAs[api.SubscriptionPlanResponse](t, res["resource"])
	require.NotNil(t, plan.Id)
	planID := *plan.Id
	assert.Equal(t, "gold", *plan.PlanName)

	res, err = call(manageSubscriptionsInput{Action: "list"})
	require.NoError(t, err)
	assert.Equal(t, 1, res["count"])

	res, err = call(manageSubscriptionsInput{Action: "get", ID: planID})
	require.NoError(t, err)
	assert.Equal(t, planID, res["id"])

	res, err = call(manageSubscriptionsInput{Action: "apply", ID: planID, Spec: &subscriptionSpec{PlanName: ptr("platinum")}})
	require.NoError(t, err)
	assert.Equal(t, "update", res["operation"])
	assert.Equal(t, "platinum", *toolsAs[api.SubscriptionPlanResponse](t, res["resource"]).PlanName)

	res, err = call(manageSubscriptionsInput{Action: "delete", ID: planID, Confirm: true})
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("Subscription plan %q deleted successfully", planID), res["message"])

	_, err = call(manageSubscriptionsInput{Action: "get", ID: planID})
	require.Error(t, err)
	assert.Equal(t, fmt.Sprintf("no SubscriptionPlan with id %q; call this tool with action=list to see what exists", planID),
		err.Error())
}

// The plan update keeps what it always did: a field the spec sets is applied,
// a field it leaves out is kept.
func TestManageSubscriptionPlanUpdateKeepsWhatItDoesNotSet(t *testing.T) {
	h := subscriptionToolsHandler(NewMockStorage())
	call := func(in manageSubscriptionsInput) map[string]any {
		in.Type = "SubscriptionPlan"
		_, out, err := h.manageSubscriptions(toolsSkippedCtx, nil, in)
		require.NoError(t, err)
		return resultMap(t, out)
	}

	plan := toolsAs[api.SubscriptionPlanResponse](t, call(manageSubscriptionsInput{Action: "apply", Spec: &subscriptionSpec{
		PlanName: ptr("gold"), BillingPlan: ptr("stripe-gold"), ThrottleLimitCount: ptr(100), ThrottleLimitUnit: ptr("Min"),
	}})["resource"])

	updated := toolsAs[api.SubscriptionPlanResponse](t, call(manageSubscriptionsInput{Action: "apply", ID: *plan.Id, Spec: &subscriptionSpec{
		ThrottleLimitCount: ptr(500), ThrottleLimitUnit: ptr("hour"),
	}})["resource"])

	assert.Equal(t, 500, *updated.ThrottleLimitCount)
	assert.Equal(t, "Hour", *updated.ThrottleLimitUnit)
	assert.Equal(t, "stripe-gold", *updated.BillingPlan, "a field the spec leaves out is kept")
	assert.Equal(t, "gold", *updated.PlanName)
}

// A whitespace-only id carries no id, so apply creates: the same decision the
// authorization gate made when it authorized POST on the collection.
func TestManageSubscriptionPlanBlankIDCreates(t *testing.T) {
	h := subscriptionToolsHandler(NewMockStorage())

	_, out, err := h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "SubscriptionPlan", Action: "apply", ID: "   ", Spec: &subscriptionSpec{PlanName: ptr("silver")},
	})

	require.NoError(t, err)
	assert.Equal(t, "create", resultMap(t, out)["operation"])
}

func TestManageSubscriptionPlanFailures(t *testing.T) {
	h := subscriptionToolsHandler(NewMockStorage())

	_, _, err := h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "SubscriptionPlan", Action: "apply", Spec: &subscriptionSpec{PlanName: ptr("p"), ExpiryTime: ptr("soon")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `spec.expiryTime "soon" is not a valid RFC 3339 timestamp`)

	_, _, err = h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "SubscriptionPlan", Action: "apply", ID: "no-such-plan", Spec: &subscriptionSpec{PlanName: ptr("p")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no SubscriptionPlan with id "no-such-plan"`)

	_, _, err = h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "SubscriptionPlan", Action: "delete", ID: "no-such-plan", Confirm: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no SubscriptionPlan with id "no-such-plan"`)
}

func TestManageSubscriptionLifecycle(t *testing.T) {
	db := NewMockStorage()
	require.NoError(t, db.SaveConfig(&models.StoredConfig{UUID: "api-uuid-1", Kind: models.KindRestApi, Handle: "petstore"}))
	h := subscriptionToolsHandler(db)
	call := func(in manageSubscriptionsInput) (map[string]any, error) {
		in.Type = "Subscription"
		_, out, err := h.manageSubscriptions(toolsSkippedCtx, nil, in)
		if err != nil {
			return nil, err
		}
		return resultMap(t, out), nil
	}

	// The handle is accepted in place of the deployment id.
	res, err := call(manageSubscriptionsInput{Action: "apply", Spec: &subscriptionSpec{
		ApiId: ptr("petstore"), SubscriptionToken: ptr("tok-123"), ApplicationId: ptr("app-1"),
	}})
	require.NoError(t, err)
	assert.Equal(t, "create", res["operation"])
	assert.Contains(t, res["notice"], "stored only as a hash")
	sub := toolsAs[api.SubscriptionResponse](t, res["resource"])
	require.NotNil(t, sub.Id)
	subID := *sub.Id
	require.NotNil(t, sub.SubscriptionToken, "create echoes the token once, as POST /subscriptions does")
	assert.Equal(t, "tok-123", *sub.SubscriptionToken)

	res, err = call(manageSubscriptionsInput{Action: "list", Spec: &subscriptionSpec{
		ApiId: ptr("petstore"), ApplicationId: ptr("app-1"), Status: ptr("active"),
	}})
	require.NoError(t, err)
	assert.Equal(t, 1, res["count"])
	listed := toolsAs[[]api.SubscriptionResponse](t, res["items"])
	require.Len(t, listed, 1)
	assert.Nil(t, listed[0].SubscriptionToken, "list never returns the token")

	res, err = call(manageSubscriptionsInput{Action: "get", ID: subID})
	require.NoError(t, err)
	assert.Equal(t, subID, res["id"])

	res, err = call(manageSubscriptionsInput{Action: "apply", ID: subID, Spec: &subscriptionSpec{Status: ptr("inactive")}})
	require.NoError(t, err)
	assert.Equal(t, "update", res["operation"])
	assert.Equal(t, api.SubscriptionResponseStatus("INACTIVE"), *toolsAs[api.SubscriptionResponse](t, res["resource"]).Status)
	assert.Contains(t, res["notice"], "changes only a subscription's status, plan and token")

	res, err = call(manageSubscriptionsInput{Action: "delete", ID: subID, Confirm: true})
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("Subscription %q deleted successfully", subID), res["message"])

	_, err = call(manageSubscriptionsInput{Action: "get", ID: subID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("no Subscription with id %q", subID))
}

func TestManageSubscriptionFailures(t *testing.T) {
	db := NewMockStorage()
	require.NoError(t, db.SaveConfig(&models.StoredConfig{UUID: "mcp-uuid-1", Kind: models.KindMcp, Handle: "tools"}))
	h := subscriptionToolsHandler(db)
	call := func(in manageSubscriptionsInput) error {
		in.Type = "Subscription"
		_, _, err := h.manageSubscriptions(toolsSkippedCtx, nil, in)
		return err
	}

	err := call(manageSubscriptionsInput{Action: "apply", Spec: &subscriptionSpec{ApiId: ptr("nope"), SubscriptionToken: ptr("t")}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no RestApi with identifier "nope" on this Gateway`)

	err = call(manageSubscriptionsInput{Action: "apply", Spec: &subscriptionSpec{ApiId: ptr("mcp-uuid-1"), SubscriptionToken: ptr("t")}})
	require.Error(t, err)
	assert.Equal(t, `"mcp-uuid-1" is a Mcp, not a RestApi. Only REST APIs bear subscriptions`, err.Error())

	err = call(manageSubscriptionsInput{Action: "list", Spec: &subscriptionSpec{ApiId: ptr("nope")}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no RestApi with identifier "nope"`)

	// As PUT /subscriptions/{id}, the stored subscription is read first, so an
	// unknown id is reported whatever the spec contains.
	err = call(manageSubscriptionsInput{Action: "apply", ID: "sub-1", Spec: &subscriptionSpec{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no Subscription with id "sub-1"`)

	err = call(manageSubscriptionsInput{Action: "delete", ID: "no-such-sub", Confirm: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no Subscription with id "no-such-sub"`)
}

// specJSON decodes a spec as the SDK does, so the keys it carried, nulls
// included, are recorded exactly as an agent's call would record them.
func specJSON(t *testing.T, raw string) *subscriptionSpec {
	t.Helper()
	var spec subscriptionSpec
	require.NoError(t, json.Unmarshal([]byte(raw), &spec))
	return &spec
}

func TestSubscriptionSpecRecordsSentKeys(t *testing.T) {
	spec := specJSON(t, `{"billingPlan":null,"planName":"gold"}`)
	assert.Nil(t, spec.BillingPlan)
	assert.Equal(t, "gold", *spec.PlanName)
	fields := spec.sentFields()
	assert.Contains(t, fields, "billingPlan", "an explicit null is a sent key")
	assert.Contains(t, fields, "planName")
	assert.NotContains(t, fields, "expiryTime", "a key left out is not")

	// A spec built in code has no record, so its non-nil fields count as sent.
	built := (&subscriptionSpec{BillingPlan: ptr(""), PlanName: ptr("gold")}).sentFields()
	assert.Contains(t, built, "billingPlan")
	assert.Contains(t, built, "planName")
	assert.NotContains(t, built, "expiryTime")
}

// The subscription update has REST parity: PUT /subscriptions/{id} changes the
// status, moves or removes the plan, and rotates the token, each only when sent.
func TestManageSubscriptionPartialUpdate(t *testing.T) {
	db := NewMockStorage()
	require.NoError(t, db.SaveConfig(&models.StoredConfig{UUID: "api-uuid-1", Kind: models.KindRestApi, Handle: "petstore"}))
	h := subscriptionToolsHandler(db)
	call := func(in manageSubscriptionsInput) (map[string]any, error) {
		_, out, err := h.manageSubscriptions(toolsSkippedCtx, nil, in)
		if err != nil {
			return nil, err
		}
		return resultMap(t, out), nil
	}
	createPlan := func(name string) string {
		res, err := call(manageSubscriptionsInput{Type: "SubscriptionPlan", Action: "apply", Spec: &subscriptionSpec{PlanName: ptr(name)}})
		require.NoError(t, err)
		return *toolsAs[api.SubscriptionPlanResponse](t, res["resource"]).Id
	}
	update := func(raw string) (api.SubscriptionResponse, error) {
		res, err := call(manageSubscriptionsInput{Type: "Subscription", Action: "apply", ID: "apim-sub-1", Spec: specJSON(t, raw)})
		if err != nil {
			return api.SubscriptionResponse{}, err
		}
		return toolsAs[api.SubscriptionResponse](t, res["resource"]), nil
	}
	gold, silver := createPlan("gold"), createPlan("silver")

	// externalRefId becomes the subscription's id, so get and apply take it.
	res, err := call(manageSubscriptionsInput{Type: "Subscription", Action: "apply", Spec: &subscriptionSpec{
		ApiId: ptr("petstore"), SubscriptionToken: ptr("tok-1"), ExternalRefId: ptr("apim-sub-1"), SubscriptionPlanId: ptr(gold),
	}})
	require.NoError(t, err)
	assert.Equal(t, "apim-sub-1", *toolsAs[api.SubscriptionResponse](t, res["resource"]).Id)

	_, err = call(manageSubscriptionsInput{Type: "Subscription", Action: "apply", Spec: &subscriptionSpec{
		ApiId: ptr("petstore"), SubscriptionToken: ptr("tok-2"), ExternalRefId: ptr("apim-sub-1"),
	}})
	require.Error(t, err)
	assert.Equal(t, "the subscription conflicts with an existing subscription", err.Error())

	_, err = call(manageSubscriptionsInput{Type: "Subscription", Action: "apply", Spec: &subscriptionSpec{
		ApiId: ptr("petstore"), SubscriptionToken: ptr("tok-2"), ExternalRefId: ptr("-bad"),
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "externalRefId must be 1-64 characters")

	sub, err := update(`{"subscriptionPlanId":"` + silver + `"}`)
	require.NoError(t, err)
	assert.Equal(t, silver, *sub.SubscriptionPlanId, "the subscription moves to the plan sent")
	assert.EqualValues(t, "ACTIVE", *sub.Status)

	_, err = call(manageSubscriptionsInput{Type: "SubscriptionPlan", Action: "delete", ID: silver, Confirm: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is in use by one or more subscriptions, so it was not deleted")

	sub, err = update(`{"status":"inactive","subscriptionToken":"tok-rotated"}`)
	require.NoError(t, err)
	assert.EqualValues(t, "INACTIVE", *sub.Status)
	assert.Equal(t, silver, *sub.SubscriptionPlanId, "a plan left out is kept")
	assert.Nil(t, sub.SubscriptionToken, "an update never returns the token, as PUT /subscriptions/{id}")
	assert.Equal(t, "tok-rotated", db.subscriptions["apim-sub-1"].SubscriptionToken)

	sub, err = update(`{"subscriptionPlanId":null}`)
	require.NoError(t, err)
	assert.Nil(t, sub.SubscriptionPlanId, "an explicit null removes the plan")

	_, err = update(`{"subscriptionToken":"  "}`)
	require.Error(t, err)
	assert.Equal(t, "subscriptionToken must not be empty", err.Error())

	// With no subscription on it any more, the plan can be deleted.
	_, err = call(manageSubscriptionsInput{Type: "SubscriptionPlan", Action: "delete", ID: silver, Confirm: true})
	require.NoError(t, err)
}

// The plan update has REST parity: PUT /subscription-plans/{id} keeps a field
// left out and removes one sent as null.
func TestManageSubscriptionPlanNullRemoves(t *testing.T) {
	h := subscriptionToolsHandler(NewMockStorage())
	call := func(in manageSubscriptionsInput) (api.SubscriptionPlanResponse, error) {
		in.Type = "SubscriptionPlan"
		_, out, err := h.manageSubscriptions(toolsSkippedCtx, nil, in)
		if err != nil {
			return api.SubscriptionPlanResponse{}, err
		}
		return toolsAs[api.SubscriptionPlanResponse](t, resultMap(t, out)["resource"]), nil
	}

	plan, err := call(manageSubscriptionsInput{Action: "apply", Spec: specJSON(t,
		`{"planName":"gold","billingPlan":"stripe-gold","throttleLimitCount":100,"throttleLimitUnit":"Min","expiryTime":"2027-01-31T23:59:59Z"}`)})
	require.NoError(t, err)
	_, err = call(manageSubscriptionsInput{Action: "apply", Spec: &subscriptionSpec{PlanName: ptr("silver")}})
	require.NoError(t, err)

	_, err = call(manageSubscriptionsInput{Action: "apply", ID: *plan.Id, Spec: specJSON(t, `{"throttleLimitCount":null}`)})
	require.Error(t, err)
	assert.Equal(t, "throttleLimitCount and throttleLimitUnit must be set or cleared together", err.Error())

	_, err = call(manageSubscriptionsInput{Action: "apply", ID: *plan.Id, Spec: specJSON(t, `{"planName":"silver"}`)})
	require.Error(t, err)
	assert.Equal(t, "a subscription plan with this name already exists; choose a different spec.planName", err.Error())

	updated, err := call(manageSubscriptionsInput{Action: "apply", ID: *plan.Id, Spec: specJSON(t,
		`{"billingPlan":null,"throttleLimitCount":null,"throttleLimitUnit":null,"expiryTime":null}`)})
	require.NoError(t, err)
	assert.Nil(t, updated.BillingPlan)
	assert.Nil(t, updated.ThrottleLimitCount)
	assert.Nil(t, updated.ThrottleLimitUnit)
	assert.Nil(t, updated.ExpiryTime)
	assert.Equal(t, "gold", *updated.PlanName, "a field left out is kept")
}

// The tool applies the body limit POST and PUT on both collections enforce,
// checked after the stored resource on update, as PUT checks it.
func TestManageSubscriptionsRequestSizeLimit(t *testing.T) {
	db := NewMockStorage()
	require.NoError(t, db.SaveConfig(&models.StoredConfig{UUID: "api-uuid-1", Kind: models.KindRestApi, Handle: "petstore"}))
	h := subscriptionToolsHandler(db)
	huge := strings.Repeat("x", maxSubscriptionRequestBodyBytes)

	_, _, err := h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "Subscription", Action: "apply", Spec: &subscriptionSpec{ApiId: ptr("petstore"), SubscriptionToken: ptr(huge)},
	})
	require.Error(t, err)
	assert.Equal(t, "the Subscription request is too large for this Gateway; nothing was stored", err.Error())
	assert.Empty(t, db.subscriptions)

	_, _, err = h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "SubscriptionPlan", Action: "apply", Spec: &subscriptionSpec{PlanName: ptr("gold"), BillingPlan: ptr(huge)},
	})
	require.Error(t, err)
	assert.Equal(t, "the SubscriptionPlan request is too large for this Gateway; nothing was stored", err.Error())
	assert.Empty(t, db.subscriptionPlans)

	_, _, err = h.manageSubscriptions(toolsSkippedCtx, nil, manageSubscriptionsInput{
		Type: "SubscriptionPlan", Action: "apply", ID: "no-such-plan", Spec: &subscriptionSpec{BillingPlan: ptr(huge)},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no SubscriptionPlan with id "no-such-plan"`)
}

// Through the real MCP endpoint, a null in spec reaches the tool as a sent key:
// the SDK's input schema accepts it, and the spec records it on decode.
func TestManageSubscriptionsNullThroughTheEndpoint(t *testing.T) {
	server := createTestAPIServerWithDB(NewMockStorage())
	h := server.EnableMCP(map[string][]string{
		"POST /subscription-plans":         {"admin"},
		"PUT /subscription-plans/{planId}": {"admin"},
	}, map[string][]string{"admin": {"gw-admin"}}, "")
	admin := &commonmodels.AuthContext{UserID: "alice", Roles: []string{"admin"}}
	callTool := func(args map[string]any) api.SubscriptionPlanResponse {
		rec := mcpPost(t, h, admin, "tools/call", map[string]any{
			"name": "wso2_apip_gw_manage_subscriptions", "arguments": args,
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		result := toolsAs[map[string]any](t, decodeMCPResponse(t, rec)["result"])
		require.NotEqual(t, true, result["isError"], rec.Body.String())
		raw, err := json.Marshal(toolsAs[map[string]any](t, result["structuredContent"])["resource"])
		require.NoError(t, err)
		var plan api.SubscriptionPlanResponse
		require.NoError(t, json.Unmarshal(raw, &plan))
		return plan
	}

	plan := callTool(map[string]any{"type": "SubscriptionPlan", "action": "apply",
		"spec": map[string]any{"planName": "gold", "billingPlan": "stripe-gold"}})
	require.NotNil(t, plan.BillingPlan)

	updated := callTool(map[string]any{"type": "SubscriptionPlan", "action": "apply", "id": *plan.Id,
		"spec": map[string]any{"billingPlan": nil}})
	assert.Nil(t, updated.BillingPlan)
	assert.Equal(t, "gold", *updated.PlanName)
}

func TestMcpSubscriptionError(t *testing.T) {
	planOp := subAction{Type: subTypePlan, Action: subActionApply, RouteKey: "POST /subscription-plans"}
	planUpdateOp := subAction{Type: subTypePlan, Action: subActionApply, RouteKey: "PUT /subscription-plans/{planId}"}
	planDeleteOp := subAction{Type: subTypePlan, Action: subActionDelete, RouteKey: "DELETE /subscription-plans/{planId}"}
	subOp := subAction{Type: subTypeSubscription, Action: subActionApply, RouteKey: "POST /subscriptions"}
	subUpdateOp := subAction{Type: subTypeSubscription, Action: subActionApply, RouteKey: "PUT /subscriptions/{subscriptionId}"}

	for _, tt := range []struct {
		name string
		op   subAction
		err  error
		want string
	}{
		{"validation detail is echoed", subOp, &subscription.ValidationError{Message: "Subscription plan is not active"},
			"Subscription plan is not active"},
		{"not a REST API", subOp, &subscription.NotRestAPIError{Identifier: "tools", Kind: "Mcp"},
			`"tools" is a Mcp, not a RestApi. Only REST APIs bear subscriptions`},
		{"API not found", subOp, fmt.Errorf("%w: petstore", subscription.ErrAPINotFound),
			`no RestApi with identifier "petstore" on this Gateway; call wso2_apip_gw_list_resources with kind=RestApi to see what is deployed`},
		{"subscription not found", subOp, subscription.ErrSubscriptionNotFound,
			`no Subscription with id "petstore"; call this tool with action=list to see what exists`},
		{"plan not found", planOp, subscription.ErrPlanNotFound,
			`no SubscriptionPlan with id "petstore"; call this tool with action=list to see what exists`},
		{"duplicate plan", planOp, fmt.Errorf("%w: plan name", storage.ErrConflict),
			`a subscription plan named "petstore" already exists`},
		{"plan renamed onto another plan's name", planUpdateOp, fmt.Errorf("%w: plan name", storage.ErrConflict),
			"a subscription plan with this name already exists; choose a different spec.planName"},
		{"plan still in use", planDeleteOp, fmt.Errorf("%w: in use", storage.ErrConflict),
			`subscription plan "petstore" is in use by one or more subscriptions, so it was not deleted; ` +
				"move those subscriptions to another plan, or delete them, first"},
		// One message for every subscription conflict, as the REST API's 409.
		{"subscription create conflict", subOp, fmt.Errorf("%w: token", storage.ErrConflict),
			"the subscription conflicts with an existing subscription"},
		{"subscription update conflict", subUpdateOp, &subscription.OpError{Op: subscription.OpUpdate,
			Cause: fmt.Errorf("%w: token", storage.ErrConflict)},
			"the subscription conflicts with an existing subscription"},
		{"plan validation could not run", subOp, &subscription.OpError{Op: subscription.OpValidate,
			Cause: errors.New("database is locked")},
			"the Gateway could not validate the subscription plan"},
		{"anything else stays generic", subOp, errors.New("database is locked"),
			"the Gateway could not apply the Subscription"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mcpSubscriptionError(tt.op, "petstore", tt.err).Error())
		})
	}
}

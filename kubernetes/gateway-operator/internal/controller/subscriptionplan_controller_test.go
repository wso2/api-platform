/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

// subscriptionPlanServer records the body of each POST/PUT to /subscription-plans
// and answers GET with an empty plan list. It answers with the conditions it was
// sent, as a gateway that supports them does.
func subscriptionPlanServer(t *testing.T, bodies *[]map[string]any) *httptest.Server {
	return gatewayServer(t, bodies, nil, true)
}

// gatewayServer is subscriptionPlanServer for a gateway that does (keepsConditions)
// or does not support plan conditions; it records DELETE paths in deletes.
func gatewayServer(t *testing.T, bodies *[]map[string]any, deletes *[]string, keepsConditions bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			if deletes != nil {
				*deletes = append(*deletes, r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"subscriptionPlans": []any{}})
			return
		}
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		body := map[string]any{}
		require.NoError(t, json.Unmarshal(raw, &body))
		*bodies = append(*bodies, body)
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		resp := map[string]any{"id": "plan-1", "planName": body["planName"]}
		if keepsConditions && body["conditions"] != nil {
			resp["conditions"] = body["conditions"]
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func planWithConditions() *apiv1.SubscriptionPlan {
	cr := &apiv1.SubscriptionPlan{}
	cr.Spec.PlanName = "Gold"
	cr.Spec.Conditions = []apiv1.SubscriptionPlanCondition{
		{Id: "requests", Type: "request-count", Params: &runtime.RawExtension{Raw: []byte(`{"limits":[{"count":100,"timeUnit":"MINUTE"}]}`)}},
		{Id: "region", Type: "custom", Params: &runtime.RawExtension{Raw: []byte(`{"zone":"eu"}`)}},
	}
	return cr
}

func legacyPlan() *apiv1.SubscriptionPlan {
	count := int64(10)
	unit := "Min"
	cr := &apiv1.SubscriptionPlan{}
	cr.Spec.PlanName = "Bronze"
	cr.Spec.ThrottleLimitCount = &count
	cr.Spec.ThrottleLimitUnit = &unit
	return cr
}

func deployPlan(t *testing.T, cr *apiv1.SubscriptionPlan) map[string]any {
	t.Helper()
	var bodies []map[string]any
	srv := subscriptionPlanServer(t, &bodies)
	res, err := (&subscriptionPlanAdapter{}).Deploy(context.Background(), nil, srv.URL, cr, nil)
	require.NoError(t, err)
	require.Equal(t, "plan-1", res.Id)
	require.Len(t, bodies, 1)
	return bodies[0]
}

var wantConditions = []any{
	map[string]any{"id": "requests", "type": "request-count", "params": map[string]any{
		"limits": []any{map[string]any{"count": float64(100), "timeUnit": "MINUTE"}},
	}},
	map[string]any{"id": "region", "type": "custom", "params": map[string]any{"zone": "eu"}},
}

func TestSubscriptionPlanDeploy_CreateSendsConditions(t *testing.T) {
	body := deployPlan(t, planWithConditions())
	require.Equal(t, wantConditions, body["conditions"])
	require.NotContains(t, body, "throttleLimitCount")
	require.NotContains(t, body, "throttleLimitUnit")
}

func TestSubscriptionPlanDeploy_CreateLegacyOnly(t *testing.T) {
	body := deployPlan(t, legacyPlan())
	require.NotContains(t, body, "conditions")
	require.Equal(t, float64(10), body["throttleLimitCount"])
	require.Equal(t, "Min", body["throttleLimitUnit"])
}

func TestSubscriptionPlanDeploy_UpdateSendsConditions(t *testing.T) {
	cr := planWithConditions()
	cr.Status.Id = "plan-1"
	body := deployPlan(t, cr)
	require.Equal(t, wantConditions, body["conditions"])
	require.NotContains(t, body, "throttleLimitCount")
}

// Removing every condition from the CR must clear them on the gateway, which
// keeps conditions a PUT leaves out.
func TestSubscriptionPlanDeploy_UpdateWithoutLimitsClearsConditions(t *testing.T) {
	cr := &apiv1.SubscriptionPlan{}
	cr.Spec.PlanName = "Free"
	cr.Status.Id = "plan-1"
	body := deployPlan(t, cr)
	require.Equal(t, []any{}, body["conditions"])
}

// The gateway refuses conditions together with the legacy throttle fields.
func TestSubscriptionPlanDeploy_UpdateLegacyOnly(t *testing.T) {
	cr := legacyPlan()
	cr.Status.Id = "plan-1"
	body := deployPlan(t, cr)
	require.NotContains(t, body, "conditions")
	require.Equal(t, float64(10), body["throttleLimitCount"])
}

func TestPlanConditions_EmptyParamsSentAsObject(t *testing.T) {
	out := planConditions([]apiv1.SubscriptionPlanCondition{{Id: "flag", Type: "custom"}})
	require.Equal(t, []gatewayclient.SubscriptionPlanCondition{{Id: "flag", Type: "custom", Params: json.RawMessage("{}")}}, out)
	require.Nil(t, planConditions(nil))
}

// A gateway that predates plan conditions ignores them. The plan must not be
// reported as deployed, and a plan it created without them is removed again.
func TestSubscriptionPlanDeploy_GatewayWithoutConditionsSupport(t *testing.T) {
	var bodies []map[string]any
	var deletes []string
	srv := gatewayServer(t, &bodies, &deletes, false)
	adapter := &subscriptionPlanAdapter{}

	_, err := adapter.Deploy(context.Background(), nil, srv.URL, planWithConditions(), nil)
	require.ErrorContains(t, err, "did not store the plan's conditions")
	require.Len(t, deletes, 1)
	require.Contains(t, deletes[0], "plan-1")

	cr := planWithConditions()
	cr.Status.Id = "plan-1"
	_, err = adapter.Deploy(context.Background(), nil, srv.URL, cr, nil)
	require.ErrorContains(t, err, "did not store the plan's conditions")

	// Plans without conditions keep working on such a gateway.
	_, err = adapter.Deploy(context.Background(), nil, srv.URL, legacyPlan(), nil)
	require.NoError(t, err)
}

// fakePlanGateway keeps the plans it was sent, lists them on GET, and can refuse
// deletes; keepsConditions says whether it supports plan conditions.
type fakePlanGateway struct {
	keepsConditions bool
	deleteStatus    int
	plans           map[string]map[string]any // id -> stored plan
	posts, puts     int
}

func (g *fakePlanGateway) serve(t *testing.T) *httptest.Server {
	t.Helper()
	if g.plans == nil {
		g.plans = map[string]map[string]any{}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ""
		if !strings.HasSuffix(r.URL.Path, "/subscription-plans") {
			id = path.Base(r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			list := []any{}
			for _, p := range g.plans {
				list = append(list, p)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"subscriptionPlans": list})
			return
		case http.MethodDelete:
			if g.deleteStatus != 0 {
				w.WriteHeader(g.deleteStatus)
				return
			}
			delete(g.plans, id)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		body := map[string]any{}
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &body))
		if r.Method == http.MethodPost {
			g.posts++
			id = fmt.Sprintf("plan-%d", len(g.plans)+1)
			w.WriteHeader(http.StatusCreated)
		} else {
			g.puts++
		}
		stored := map[string]any{"id": id, "planName": body["planName"]}
		if g.keepsConditions && body["conditions"] != nil {
			stored["conditions"] = body["conditions"]
		}
		g.plans[id] = stored
		_ = json.NewEncoder(w).Encode(stored)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// When the plan made without its conditions can't be removed, the next reconcile
// finds it by name; it must update and check it, not report it as deployed.
func TestSubscriptionPlanDeploy_FailedCleanupThenRetryIsNotProgrammed(t *testing.T) {
	g := &fakePlanGateway{deleteStatus: http.StatusInternalServerError}
	srv := g.serve(t)
	adapter := &subscriptionPlanAdapter{}

	_, err := adapter.Deploy(context.Background(), nil, srv.URL, planWithConditions(), nil)
	require.ErrorContains(t, err, "did not store the plan's conditions")
	require.ErrorContains(t, err, "removing the plan it created also failed")
	require.Len(t, g.plans, 1)

	_, err = adapter.Deploy(context.Background(), nil, srv.URL, planWithConditions(), nil)
	require.ErrorContains(t, err, "did not store the plan's conditions")
	require.Equal(t, 1, g.posts, "the retry must not create a second plan")
	require.Equal(t, 1, g.puts, "the retry must update the plan it found")
}

// A plan found by name gets the CR's spec, on a gateway that supports conditions.
func TestSubscriptionPlanDeploy_RecoveredPlanIsUpdated(t *testing.T) {
	g := &fakePlanGateway{keepsConditions: true, plans: map[string]map[string]any{
		"plan-7": {"id": "plan-7", "planName": "Gold"},
	}}
	srv := g.serve(t)

	res, err := (&subscriptionPlanAdapter{}).Deploy(context.Background(), nil, srv.URL, planWithConditions(), nil)
	require.NoError(t, err)
	require.Equal(t, "plan-7", res.Id)
	require.Equal(t, 0, g.posts)
	require.Equal(t, 1, g.puts)
	require.Equal(t, wantConditions, g.plans["plan-7"]["conditions"])
}

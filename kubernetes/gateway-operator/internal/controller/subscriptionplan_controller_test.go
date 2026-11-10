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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

// subscriptionPlanServer records the body of each POST/PUT to /subscription-plans
// and answers GET with an empty plan list.
func subscriptionPlanServer(t *testing.T, bodies *[]map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "plan-1", "planName": body["planName"]})
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

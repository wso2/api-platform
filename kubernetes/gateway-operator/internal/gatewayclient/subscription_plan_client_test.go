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

package gatewayclient

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubscriptionPlanCreatePayload_Conditions(t *testing.T) {
	payload := SubscriptionPlanCreatePayload{
		PlanName: "Gold",
		Conditions: []SubscriptionPlanCondition{
			{Id: "requests", Type: "request-count", Params: json.RawMessage(`{"limits":[{"count":100,"timeUnit":"MINUTE"}],"onExceed":"reject"}`)},
			{Id: "region", Type: "custom", Params: json.RawMessage(`{"zone":"eu"}`)},
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"planName": "Gold",
		"conditions": [
			{"id": "requests", "type": "request-count", "params": {"limits": [{"count": 100, "timeUnit": "MINUTE"}], "onExceed": "reject"}},
			{"id": "region", "type": "custom", "params": {"zone": "eu"}}
		]
	}`, string(body))
}

func TestSubscriptionPlanCreatePayload_LegacyOnly(t *testing.T) {
	count := int64(10)
	unit := "Min"
	body, err := json.Marshal(SubscriptionPlanCreatePayload{PlanName: "Bronze", ThrottleLimitCount: &count, ThrottleLimitUnit: &unit})
	require.NoError(t, err)
	require.JSONEq(t, `{"planName":"Bronze","throttleLimitCount":10,"throttleLimitUnit":"Min"}`, string(body))
}

func TestSubscriptionPlanUpdatePayload_Conditions(t *testing.T) {
	name := "Gold"

	body, err := json.Marshal(SubscriptionPlanUpdatePayload{PlanName: &name})
	require.NoError(t, err)
	require.JSONEq(t, `{"planName":"Gold"}`, string(body), "nil conditions are left out")

	empty := []SubscriptionPlanCondition{}
	body, err = json.Marshal(SubscriptionPlanUpdatePayload{PlanName: &name, Conditions: &empty})
	require.NoError(t, err)
	require.JSONEq(t, `{"planName":"Gold","conditions":[]}`, string(body), "an empty list is sent to clear conditions")
}

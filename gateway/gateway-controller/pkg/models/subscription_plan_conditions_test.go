/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cond(id, conditionType, params string) SubscriptionPlanCondition {
	return SubscriptionPlanCondition{ID: id, Type: conditionType, Params: []byte(params)}
}

func intPtr(v int) *int { return &v }

// A plan synced before conditions gets its throttle limit as a request-count condition, whichever
// unit names it carries; a plan with conditions keeps them.
func TestEffectiveConditions(t *testing.T) {
	tests := []struct {
		name string
		plan SubscriptionPlan
		want string
	}{
		{"gateway unit", SubscriptionPlan{ThrottleLimitCount: intPtr(50), ThrottleLimitUnit: strPtr("Min"), StopOnQuotaReach: true},
			`[{"id":"requests","type":"request-count","params":{"limits":[{"count":50,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}}]`},
		{"control-plane unit, soft limit", SubscriptionPlan{ThrottleLimitCount: intPtr(9), ThrottleLimitUnit: strPtr("DAY")},
			`[{"id":"requests","type":"request-count","params":{"limits":[{"count":9,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"allow"}}]`},
		{"no limit", SubscriptionPlan{}, `[]`},
		{"unknown unit", SubscriptionPlan{ThrottleLimitCount: intPtr(1), ThrottleLimitUnit: strPtr("Week")}, `[]`},
		{"explicit conditions", SubscriptionPlan{ThrottleLimitCount: intPtr(1), ThrottleLimitUnit: strPtr("Min"),
			Conditions: []SubscriptionPlanCondition{cond("promo", "custom", `{}`)}},
			`[{"id":"promo","type":"custom","params":{}}]`},
		{"no conditions", SubscriptionPlan{ThrottleLimitCount: intPtr(1), ThrottleLimitUnit: strPtr("Min"),
			Conditions: []SubscriptionPlanCondition{}}, `[]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jsonString(tt.plan.EffectiveConditions())
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, got)
		})
	}
}

func TestNormalizeConditions_StoredForm(t *testing.T) {
	got, err := NormalizeConditions([]SubscriptionPlanCondition{
		cond("", "request-count", `{"limits":[{"count":100,"timeUnit":"minute"},{"count":5,"timeUnit":"DAY"}]}`),
		cond("", "token-count", `{"limits":[{"count":1000,"timeUnit":"MONTH"}]}`),
		cond("ip-ranges", "custom", `{"zone":"eu","cidr":"10.0.0.0/8"}`),
	})
	require.NoError(t, err)
	assert.Equal(t, "requests", got[0].ID)
	assert.Equal(t, `{"limits":[{"count":100,"timeAmount":1,"timeUnit":"MINUTE"},{"count":5,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"reject"}`, string(got[0].Params))
	assert.Equal(t, "tokens", got[1].ID)
	assert.Equal(t, `{"limits":[{"count":1000,"timeAmount":1,"timeUnit":"MONTH"}],"onExceed":"reject","tokenType":"total"}`, string(got[1].Params))
	assert.Equal(t, `{"cidr":"10.0.0.0/8","zone":"eu"}`, string(got[2].Params))
}

func TestNormalizeConditions_Refusals(t *testing.T) {
	limit := `{"limits":[{"count":1,"timeUnit":"MINUTE"}]}`
	for name, tc := range map[string]struct {
		conditions []SubscriptionPlanCondition
		message    string
	}{
		"unknown type":          {[]SubscriptionPlanCondition{cond("", "bandwidth", limit)}, "type must be one of"},
		"two of one type":       {[]SubscriptionPlanCondition{cond("", "request-count", limit), cond("", "request-count", limit)}, "only one request-count"},
		"wrong built-in id":     {[]SubscriptionPlanCondition{cond("calls", "request-count", limit)}, "the id of a request-count condition is requests"},
		"week unit":             {[]SubscriptionPlanCondition{cond("", "request-count", `{"limits":[{"count":1,"timeUnit":"WEEK"}]}`)}, "timeUnit must be one of"},
		"same-length windows":   {[]SubscriptionPlanCondition{cond("", "request-count", `{"limits":[{"count":1,"timeAmount":60,"timeUnit":"MINUTE"},{"count":2,"timeUnit":"HOUR"}]}`)}, "window of its own length"},
		"unknown param":         {[]SubscriptionPlanCondition{cond("", "request-count", `{"limits":[{"count":1,"timeUnit":"DAY"}],"burst":2}`)}, "invalid params"},
		"custom without id":     {[]SubscriptionPlanCondition{cond("", "custom", `{}`)}, "needs an id"},
		"custom built-in id":    {[]SubscriptionPlanCondition{cond("tokens", "custom", `{}`)}, "id of a built-in condition"},
		"event-count type":      {[]SubscriptionPlanCondition{cond("", "event-count", limit)}, "type must be one of request-count, tool-call-count, token-count or custom"},
		"custom params array":   {[]SubscriptionPlanCondition{cond("promo", "custom", `[1]`)}, "JSON object"},
		"custom params too big": {[]SubscriptionPlanCondition{cond("promo", "custom", `{"v":"`+strings.Repeat("x", 5000)+`"}`)}, "at most 4096 bytes"},
		"no type":               {[]SubscriptionPlanCondition{cond("x", "", `{}`)}, "type is required"},
		"two custom, one id":    {[]SubscriptionPlanCondition{cond("promo", "custom", `{}`), cond("promo", "custom", `{}`)}, "already has a condition promo"},
		"no params":             {[]SubscriptionPlanCondition{cond("", "tool-call-count", ``)}, "params with at least one limit"},
		"no limits":             {[]SubscriptionPlanCondition{cond("", "tool-call-count", `{"limits":[]}`)}, "needs at least one limit"},
		"too many limits":       {[]SubscriptionPlanCondition{cond("", "request-count", tooManyLimits())}, "at most 10 limits"},
		"zero count":            {[]SubscriptionPlanCondition{cond("", "request-count", `{"limits":[{"count":0,"timeUnit":"DAY"}]}`)}, "count must be at least 1"},
		"negative time amount":  {[]SubscriptionPlanCondition{cond("", "request-count", `{"limits":[{"count":1,"timeAmount":-2,"timeUnit":"DAY"}]}`)}, "timeAmount must be at least 1"},
		"bad onExceed":          {[]SubscriptionPlanCondition{cond("", "request-count", `{"limits":[{"count":1,"timeUnit":"DAY"}],"onExceed":"warn"}`)}, "onExceed must be reject or allow"},
		"bad tokenType":         {[]SubscriptionPlanCondition{cond("", "token-count", `{"limits":[{"count":1,"timeUnit":"DAY"}],"tokenType":"cached"}`)}, "tokenType must be"},
		"tokenType off tokens":  {[]SubscriptionPlanCondition{cond("", "request-count", `{"limits":[{"count":1,"timeUnit":"DAY"}],"tokenType":"prompt"}`)}, "token-count conditions only"},
		"custom empty key":      {[]SubscriptionPlanCondition{cond("promo", "custom", `{" ":1}`)}, "empty key"},
		"custom without params": {[]SubscriptionPlanCondition{cond("promo", "custom", `null`)}, "params is required"},
		"too many conditions":   {tooManyConditions(), "at most 20 conditions"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NormalizeConditions(tc.conditions)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.message)
		})
	}
}

// subscription-validation v1 gets the first one-unit window, or else the first window per unit, in
// the gateway's unit names; no request-count condition leaves the plan unlimited for it.
func TestSetThrottleLimitFromConditions(t *testing.T) {
	plan := &SubscriptionPlan{Conditions: []SubscriptionPlanCondition{
		cond("requests", "request-count", `{"limits":[{"count":9,"timeAmount":2,"timeUnit":"HOUR"},{"count":100,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"allow"}`)}}
	plan.SetThrottleLimitFromConditions()
	assert.Equal(t, 100, *plan.ThrottleLimitCount)
	assert.Equal(t, "Day", *plan.ThrottleLimitUnit)
	assert.False(t, plan.StopOnQuotaReach)

	plan.Conditions = []SubscriptionPlanCondition{cond("requests", "request-count", `{"limits":[{"count":9,"timeAmount":2,"timeUnit":"HOUR"}],"onExceed":"reject"}`)}
	plan.SetThrottleLimitFromConditions()
	assert.Equal(t, 9, *plan.ThrottleLimitCount, "9 per 2 hours keeps its count, per hour")
	assert.Equal(t, "Hour", *plan.ThrottleLimitUnit)

	plan.Conditions = []SubscriptionPlanCondition{
		cond("tool-calls", "tool-call-count", `{"limits":[{"count":9,"timeAmount":1,"timeUnit":"HOUR"}],"onExceed":"reject"}`),
		cond("tokens", "token-count", `{"limits":[{"count":9,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"allow"}`)}
	plan.SetThrottleLimitFromConditions()
	assert.Nil(t, plan.ThrottleLimitCount)
	assert.Nil(t, plan.ThrottleLimitUnit)
	assert.True(t, plan.StopOnQuotaReach)
}

func TestWithThrottleLimit(t *testing.T) {
	custom := cond("promo", "custom", `{}`)
	old := cond("requests", "request-count", `{"limits":[{"count":1,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}`)

	got := WithThrottleLimit([]SubscriptionPlanCondition{custom, old}, intPtr(20), "Hour", false)
	require.Len(t, got, 2)
	assert.Equal(t, "promo", got[0].ID)
	assert.JSONEq(t, `{"limits":[{"count":20,"timeAmount":1,"timeUnit":"HOUR"}],"onExceed":"allow"}`, string(got[1].Params))

	got = WithThrottleLimit([]SubscriptionPlanCondition{custom}, intPtr(5), "Day", true)
	require.Len(t, got, 2)
	assert.Equal(t, "requests", got[0].ID, "a new request-count condition goes first")

	got = WithThrottleLimit([]SubscriptionPlanCondition{old, custom}, nil, "", true)
	require.Len(t, got, 1)
	assert.Equal(t, "promo", got[0].ID)

	// The throttle limit fields change only the request-count condition, never another built-in one.
	toolCalls := cond("tool-calls", "tool-call-count", `{"limits":[{"count":7,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"reject"}`)
	got = WithThrottleLimit([]SubscriptionPlanCondition{toolCalls, old}, intPtr(3), "Min", true)
	require.Len(t, got, 2)
	assert.Equal(t, toolCalls, got[0])
	assert.Equal(t, "requests", got[1].ID)
	assert.JSONEq(t, `{"limits":[{"count":3,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}`, string(got[1].Params))

	got = WithThrottleLimit([]SubscriptionPlanCondition{toolCalls}, nil, "", true)
	assert.Equal(t, []SubscriptionPlanCondition{toolCalls}, got)
}

func TestWithOnExceed(t *testing.T) {
	in := []SubscriptionPlanCondition{
		cond("requests", "request-count", `{"limits":[{"count":1,"timeAmount":1,"timeUnit":"MINUTE"},{"count":9,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"reject"}`),
		cond("promo", "custom", `{"a":1}`),
		cond("tool-calls", "tool-call-count", `{"limits":[{"count":7,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"reject"}`),
	}
	got := WithOnExceed(in, false)
	assert.Equal(t, in[2].Params, got[2].Params, "only the request-count condition changes")
	assert.JSONEq(t, `{"limits":[{"count":1,"timeAmount":1,"timeUnit":"MINUTE"},{"count":9,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"allow"}`, string(got[0].Params))
	assert.Equal(t, `{"a":1}`, string(got[1].Params))
	assert.Contains(t, string(in[0].Params), `"reject"`, "the input is not changed")
}

func jsonString(v any) (string, error) {
	data, err := json.Marshal(v)
	return string(data), err
}

func tooManyLimits() string {
	windows := make([]string, maxConditionWindows+1)
	for i := range windows {
		windows[i] = fmt.Sprintf(`{"count":1,"timeAmount":%d,"timeUnit":"MINUTE"}`, i+1)
	}
	return `{"limits":[` + strings.Join(windows, ",") + `]}`
}

func tooManyConditions() []SubscriptionPlanCondition {
	out := make([]SubscriptionPlanCondition, maxPlanConditions+1)
	for i := range out {
		out[i] = cond(fmt.Sprintf("c%d", i), "custom", `{}`)
	}
	return out
}

// Values that are valid as sent are kept: a token type, and custom params of {}.
func TestNormalizeConditions_KeepsValidValues(t *testing.T) {
	got, err := NormalizeConditions([]SubscriptionPlanCondition{
		cond("", "token-count", `{"limits":[{"count":9,"timeUnit":"DAY"}],"tokenType":"prompt","onExceed":"allow"}`),
		cond("promo", "custom", `{}`),
	})
	require.NoError(t, err)
	assert.Contains(t, string(got[0].Params), `"tokenType":"prompt"`)
	assert.Equal(t, `{}`, string(got[1].Params))
}

// A stored request-count condition that can't be read gives subscription-validation v1 no limit, and
// is kept as it is when only onExceed changes.
func TestUnreadableRequestCondition(t *testing.T) {
	plan := &SubscriptionPlan{Conditions: []SubscriptionPlanCondition{cond("requests", "request-count", `not json`)}}
	plan.SetThrottleLimitFromConditions()
	assert.Nil(t, plan.ThrottleLimitCount)
	got := WithOnExceed(plan.Conditions, false)
	assert.Equal(t, `not json`, string(got[0].Params))
}

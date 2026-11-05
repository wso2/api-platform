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

package model

import "testing"

func TestNewRequestCountCondition(t *testing.T) {
	windows := []ConditionWindow{{Count: 100, TimeAmount: 1, TimeUnit: "MINUTE"}}
	hard := NewRequestCountCondition(windows, true)
	if hard.ID != ConditionIDRequests || hard.Type != ConditionTypeRequestCount ||
		string(hard.Params) != `{"limits":[{"count":100,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}` {
		t.Fatalf("hard limit = %s %s %s", hard.ID, hard.Type, hard.Params)
	}
	if soft := NewRequestCountCondition(windows, false); string(soft.Params) != `{"limits":[{"count":100,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"allow"}` {
		t.Fatalf("soft limit = %s", soft.Params)
	}
}

func TestFindCondition(t *testing.T) {
	conditions := []SubscriptionPlanCondition{
		{ID: "promo", Type: ConditionTypeCustom},
		{ID: "tool-calls", Type: ConditionTypeToolCallCount},
	}
	if c := FindCondition(conditions, ConditionTypeToolCallCount); c == nil || c.ID != "tool-calls" {
		t.Fatalf("found %+v", c)
	}
	if c := FindCondition(conditions, ConditionTypeRequestCount); c != nil {
		t.Fatalf("found %+v, want none", c)
	}
	// The result points into the list, so a caller can change the condition in place.
	FindCondition(conditions, ConditionTypeCustom).ID = "promo-2"
	if conditions[0].ID != "promo-2" {
		t.Fatal("FindCondition returned a copy")
	}
}

// An unreadable request-count condition leaves the plan with no legacy limit.
func TestSetLegacyLimit_UnreadableParams(t *testing.T) {
	count := 5
	p := &SubscriptionPlan{ThrottleLimitCount: &count, ThrottleLimitUnit: "DAY", StopOnQuotaReach: false,
		Conditions: []SubscriptionPlanCondition{{ID: ConditionIDRequests, Type: ConditionTypeRequestCount, Params: []byte(`not json`)}}}
	p.SetLegacyLimit()
	if p.ThrottleLimitCount != nil || p.ThrottleLimitUnit != "" || !p.StopOnQuotaReach {
		t.Fatalf("legacy limit = %v %q %v, want none", p.ThrottleLimitCount, p.ThrottleLimitUnit, p.StopOnQuotaReach)
	}
}

// Older gateways get the first request window that is one unit long, or else the first window applied
// per unit; a plan with no request-count condition is unlimited for them.
func TestSetLegacyLimit(t *testing.T) {
	request := func(params string) []SubscriptionPlanCondition {
		return []SubscriptionPlanCondition{{ID: ConditionIDRequests, Type: ConditionTypeRequestCount, Params: []byte(params)}}
	}
	tests := []struct {
		name       string
		conditions []SubscriptionPlanCondition
		count      int // 0: no limit
		unit       string
		stop       bool
	}{
		{"no conditions", nil, 0, "", true},
		{"first one-unit window", request(`{"limits":[{"count":9,"timeAmount":2,"timeUnit":"HOUR"},{"count":100,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"allow"}`),
			100, "DAY", false},
		{"no one-unit window: the first spread over each unit", request(`{"limits":[{"count":9,"timeAmount":2,"timeUnit":"HOUR"},{"count":50,"timeAmount":3,"timeUnit":"DAY"}],"onExceed":"reject"}`),
			4, "HOUR", true},
		{"fewer requests than units: 1 per unit", request(`{"limits":[{"count":1,"timeAmount":2,"timeUnit":"HOUR"}],"onExceed":"reject"}`),
			1, "HOUR", true},
		{"a unit older gateways don't know is skipped", request(`{"limits":[{"count":1,"timeAmount":1,"timeUnit":"WEEK"},{"count":40,"timeAmount":1,"timeUnit":"MONTH"}],"onExceed":"reject"}`),
			40, "MONTH", true},
		{"only other conditions", []SubscriptionPlanCondition{{ID: "tool-calls", Type: ConditionTypeToolCallCount,
			Params: []byte(`{"limits":[{"count":9,"timeAmount":1,"timeUnit":"HOUR"}],"onExceed":"reject"}`)}}, 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &SubscriptionPlan{Conditions: tt.conditions}
			p.SetLegacyLimit()
			got := 0
			if p.ThrottleLimitCount != nil {
				got = *p.ThrottleLimitCount
			}
			if got != tt.count || p.ThrottleLimitUnit != tt.unit || p.StopOnQuotaReach != tt.stop {
				t.Fatalf("legacy limit = %d %q %v, want %d %q %v", got, p.ThrottleLimitUnit, p.StopOnQuotaReach, tt.count, tt.unit, tt.stop)
			}
		})
	}
}

func TestSubscriptionPlanUsage_InUse(t *testing.T) {
	if (&SubscriptionPlanUsage{}).InUse() {
		t.Error("an unused plan reports in use")
	}
	for _, u := range []SubscriptionPlanUsage{{RESTAPIs: 1}, {MCPProxies: 1}, {Drafts: 1}, {Publications: 1}, {Subscriptions: 1}} {
		if !u.InUse() {
			t.Errorf("%+v reports not in use", u)
		}
	}
}

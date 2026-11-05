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

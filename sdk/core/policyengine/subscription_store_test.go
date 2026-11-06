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

package policyengine

import (
	"encoding/json"
	"testing"
)

// A subscription's ID, plan ID and plan conditions reach the entry policies look up, by token and by
// application, next to the throttle fields.
func TestSubscriptionStore_KeepsPlanConditions(t *testing.T) {
	store := NewSubscriptionStore()
	conditions := []SubscriptionCondition{{ID: "tool-calls", Type: "tool-call-count",
		Params: json.RawMessage(`{"limits":[{"count":10,"timeAmount":1,"timeUnit":"HOUR"}],"onExceed":"reject"}`)}}
	store.ReplaceAll([]SubscriptionData{{
		APIId: "api-1", ApplicationId: "app-1", SubscriptionToken: HashSubscriptionToken("tok"), Status: "ACTIVE",
		ThrottleLimitCount: 50, ThrottleLimitUnit: "Min", SubscriptionId: "sub-1", PlanId: "plan-1", Conditions: conditions,
	}})

	active, byToken := store.IsActiveByToken("api-1", HashSubscriptionToken("tok"))
	if !active || byToken == nil {
		t.Fatalf("token lookup: active = %v, entry = %v", active, byToken)
	}
	_, byApp := store.IsActiveByApplication("api-1", "app-1")
	for name, entry := range map[string]*SubscriptionEntry{"token": byToken, "application": byApp} {
		if entry.SubscriptionID != "sub-1" || entry.PlanID != "plan-1" || len(entry.Conditions) != 1 ||
			entry.Conditions[0].ID != "tool-calls" || entry.ThrottleLimitCount != 50 {
			t.Errorf("%s lookup entry = %+v", name, entry)
		}
	}
}

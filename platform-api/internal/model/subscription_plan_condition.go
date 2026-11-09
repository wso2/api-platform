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

import (
	"encoding/json"
	"fmt"
	"math"
)

// Condition types. A plan has at most one condition of each built-in type.
const (
	ConditionTypeRequestCount  = "request-count"
	ConditionTypeToolCallCount = "tool-call-count"
	ConditionTypeTokenCount    = "token-count"
	ConditionTypeEventCount    = "event-count"
	ConditionTypeCustom        = "custom"
)

// Fixed IDs of the built-in conditions, the same in every plan.
const (
	ConditionIDRequests  = "requests"
	ConditionIDToolCalls = "tool-calls"
	ConditionIDTokens    = "tokens"
	ConditionIDEvents    = "events"
)

// BuiltInConditionIDs maps each built-in condition type to its fixed ID.
var BuiltInConditionIDs = map[string]string{
	ConditionTypeRequestCount:  ConditionIDRequests,
	ConditionTypeToolCallCount: ConditionIDToolCalls,
	ConditionTypeTokenCount:    ConditionIDTokens,
	ConditionTypeEventCount:    ConditionIDEvents,
}

// What happens when a window is used up: reject the request, or allow it as overage.
const (
	ConditionOnExceedReject = "reject"
	ConditionOnExceedAllow  = "allow"
)

// The tokens a token-count condition counts.
const (
	ConditionTokenTypeTotal      = "total"
	ConditionTokenTypePrompt     = "prompt"
	ConditionTokenTypeCompletion = "completion"
)

// SubscriptionPlanCondition is one condition of a plan. Policies on the API enforce it.
type SubscriptionPlanCondition struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
}

// ConditionLimits is the params of a built-in condition. Every window must have room.
type ConditionLimits struct {
	Limits    []ConditionWindow `json:"limits"`
	OnExceed  string            `json:"onExceed"`
	TokenType string            `json:"tokenType,omitempty"`
}

// ConditionWindow allows Count units per TimeAmount TimeUnit.
type ConditionWindow struct {
	Count      int    `json:"count"`
	TimeAmount int    `json:"timeAmount"`
	TimeUnit   string `json:"timeUnit"`
}

// LegacyLimitWindows are the windows an API Portal's limits give a plan: its REQUEST_COUNT limits are
// the request-count windows and its EVENT_COUNT limits the event-count windows.
type LegacyLimitWindows struct {
	Requests []ConditionWindow
	Events   []ConditionWindow
}

// legacyLimitUnits are the units older gateways and subscription-validation v1 understand.
var legacyLimitUnits = map[string]bool{"MINUTE": true, "HOUR": true, "DAY": true, "MONTH": true}

// NewRequestCountCondition builds a request-count condition. stopOnQuotaReach false makes it a soft limit.
func NewRequestCountCondition(windows []ConditionWindow, stopOnQuotaReach bool) SubscriptionPlanCondition {
	return NewLimitCondition(ConditionTypeRequestCount, windows, stopOnQuotaReach)
}

// NewLimitCondition builds a built-in condition of the given type (not token-count, which needs a
// tokenType) with these windows. stopOnQuotaReach false makes it a soft limit.
func NewLimitCondition(conditionType string, windows []ConditionWindow, stopOnQuotaReach bool) SubscriptionPlanCondition {
	onExceed := ConditionOnExceedReject
	if !stopOnQuotaReach {
		onExceed = ConditionOnExceedAllow
	}
	params, _ := json.Marshal(ConditionLimits{Limits: windows, OnExceed: onExceed})
	return SubscriptionPlanCondition{ID: BuiltInConditionIDs[conditionType], Type: conditionType, Params: params}
}

// FindCondition returns the first condition of the given type, or nil.
func FindCondition(conditions []SubscriptionPlanCondition, conditionType string) *SubscriptionPlanCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// SetLegacyLimit fills the single request limit that older gateways and subscription-validation v1 read:
// a count per one MINUTE, HOUR, DAY or MONTH. They get the first one-unit window as it is. A plan with
// only longer windows gets its first one with the same count per one unit, as releases before
// conditions did: 1000 per 2 HOUR is enforced as 1000 per HOUR, which can admit up to twice the
// window's count but never blocks a subscriber within it. Without a request-count condition the plan
// is unlimited for them.
func (p *SubscriptionPlan) SetLegacyLimit() {
	p.ThrottleLimitCount = nil
	p.ThrottleLimitUnit = ""
	p.StopOnQuotaReach = true
	c := FindCondition(p.Conditions, ConditionTypeRequestCount)
	if c == nil {
		return
	}
	var limits ConditionLimits
	if err := json.Unmarshal(c.Params, &limits); err != nil {
		return
	}
	p.StopOnQuotaReach = limits.OnExceed != ConditionOnExceedAllow
	var picked *ConditionWindow
	for i, w := range limits.Limits {
		if !legacyLimitUnits[w.TimeUnit] {
			continue
		}
		if w.TimeAmount <= 1 {
			picked = &limits.Limits[i]
			break
		}
		if picked == nil {
			picked = &limits.Limits[i]
		}
	}
	if picked != nil {
		count := picked.Count
		p.ThrottleLimitCount = &count
		p.ThrottleLimitUnit = picked.TimeUnit
	}
}

// WindowKey names a window by its length, so 60 MINUTE and 1 HOUR match. Months match only months.
func WindowKey(amount int, unit string) string {
	minutes := map[string]int{"MINUTE": 1, "HOUR": 60, "DAY": 24 * 60}[unit]
	if minutes == 0 {
		return fmt.Sprintf("M%d", amount)
	}
	// A length in minutes past the int range can't equal any other window: keep it apart.
	if amount > math.MaxInt/minutes || amount < math.MinInt/minutes {
		return fmt.Sprintf("%s%d", unit, amount)
	}
	return fmt.Sprintf("m%d", amount*minutes)
}

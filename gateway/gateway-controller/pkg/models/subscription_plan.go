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

package models

import (
	"encoding/json"
	"strings"
	"time"
)

// SubscriptionPlanStatus represents the status of a subscription plan
type SubscriptionPlanStatus string

const (
	SubscriptionPlanStatusActive   SubscriptionPlanStatus = "ACTIVE"
	SubscriptionPlanStatusInactive SubscriptionPlanStatus = "INACTIVE"
)

// SubscriptionPlan represents an organization-scoped subscription plan
type SubscriptionPlan struct {
	ID                 string  `json:"id"`
	GatewayID          string  `json:"gatewayId" db:"gateway_id"`
	PlanName           string  `json:"planName" db:"plan_name"`
	BillingPlan        *string `json:"billingPlan,omitempty" db:"billing_plan"`
	StopOnQuotaReach   bool    `json:"stopOnQuotaReach" db:"stop_on_quota_reach"`
	ThrottleLimitCount *int    `json:"throttleLimitCount,omitempty" db:"throttle_limit_count"`
	ThrottleLimitUnit  *string `json:"throttleLimitUnit,omitempty" db:"throttle_limit_unit"`
	// Conditions are the plan's conditions. Nil for a plan synced before conditions, whose single
	// throttle limit is then its only limit (see EffectiveConditions).
	Conditions []SubscriptionPlanCondition `json:"conditions" db:"conditions"`
	ExpiryTime *time.Time                  `json:"expiryTime,omitempty" db:"expiry_time"`
	Status     SubscriptionPlanStatus      `json:"status" db:"status"`
	CreatedAt  time.Time                   `json:"createdAt" db:"created_at"`
	UpdatedAt  time.Time                   `json:"updatedAt" db:"updated_at"`
	Etag       string                      `json:"etag,omitempty"` // Transient; from CP sync response, not stored in DB
}

// SubscriptionPlanCondition is one condition of a plan: an ID, a type and the type's params. Policies
// on the API enforce it.
type SubscriptionPlanCondition struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
}

// Condition types and the fixed ID of the request-count condition.
const (
	ConditionTypeRequestCount  = "request-count"
	ConditionTypeToolCallCount = "tool-call-count"
	ConditionTypeTokenCount    = "token-count"
	ConditionTypeEventCount    = "event-count"
	ConditionTypeCustom        = "custom"
	ConditionIDRequests        = "requests"
)

// conditionTimeUnits maps the throttle units a plan may carry (the gateway's Min, Hour, Day, Month,
// or the control plane's MINUTE, HOUR, DAY, MONTH) to condition window units.
var conditionTimeUnits = map[string]string{
	"min": "MINUTE", "minute": "MINUTE", "hour": "HOUR", "day": "DAY", "month": "MONTH",
}

// EffectiveConditions returns the plan's conditions. A plan synced before conditions gets a
// request-count condition from its single throttle limit, or none when it has no limit.
func (p *SubscriptionPlan) EffectiveConditions() []SubscriptionPlanCondition {
	if p.Conditions != nil {
		return p.Conditions
	}
	if p.ThrottleLimitCount == nil || *p.ThrottleLimitCount <= 0 || p.ThrottleLimitUnit == nil {
		return []SubscriptionPlanCondition{}
	}
	unit, ok := conditionTimeUnits[strings.ToLower(strings.TrimSpace(*p.ThrottleLimitUnit))]
	if !ok {
		return []SubscriptionPlanCondition{}
	}
	onExceed := "reject"
	if !p.StopOnQuotaReach {
		onExceed = "allow"
	}
	params, _ := json.Marshal(map[string]any{
		"limits":   []map[string]any{{"count": *p.ThrottleLimitCount, "timeAmount": 1, "timeUnit": unit}},
		"onExceed": onExceed,
	})
	return []SubscriptionPlanCondition{{ID: ConditionIDRequests, Type: ConditionTypeRequestCount, Params: params}}
}

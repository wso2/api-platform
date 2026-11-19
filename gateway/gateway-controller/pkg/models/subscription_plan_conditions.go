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
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Limits on what a plan can hold, as in the Platform API.
const (
	maxPlanConditions             = 20
	maxConditionWindows           = 10
	maxCustomConditionParamsBytes = 4096
)

// builtInConditionIDs maps each built-in condition type to its fixed ID.
var builtInConditionIDs = map[string]string{
	ConditionTypeRequestCount:  ConditionIDRequests,
	ConditionTypeToolCallCount: "tool-calls",
	ConditionTypeTokenCount:    "tokens",
}

var customConditionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// windowMinutes tells windows of the same length apart (60 MINUTE equals 1 HOUR). Months compare
// only with months.
var windowMinutes = map[string]int{"MINUTE": 1, "HOUR": 60, "DAY": 24 * 60}

// conditionLimits is the params of a built-in condition.
type conditionLimits struct {
	Limits    []conditionWindow `json:"limits"`
	OnExceed  string            `json:"onExceed"`
	TokenType string            `json:"tokenType,omitempty"`
}

type conditionWindow struct {
	Count      int    `json:"count"`
	TimeAmount int    `json:"timeAmount"`
	TimeUnit   string `json:"timeUnit"`
}

// NormalizeConditions validates a plan's conditions and returns them with defaults filled in, by the
// Platform API's rules: at most one condition of each built-in type, with its fixed ID; 1 to 10
// windows of MINUTE, HOUR, DAY or MONTH, no two of the same length; custom conditions with their own
// ID and a JSON object of key-value pairs.
func NormalizeConditions(conditions []SubscriptionPlanCondition) ([]SubscriptionPlanCondition, error) {
	if len(conditions) > maxPlanConditions {
		return nil, fmt.Errorf("a plan can have at most %d conditions", maxPlanConditions)
	}
	out := make([]SubscriptionPlanCondition, 0, len(conditions))
	seen := map[string]bool{}
	for i, c := range conditions {
		field := fmt.Sprintf("conditions[%d]", i)
		c.Type = strings.TrimSpace(c.Type)
		c.ID = strings.TrimSpace(c.ID)
		var err error
		if id, builtIn := builtInConditionIDs[c.Type]; builtIn {
			if c.ID != "" && c.ID != id {
				return nil, fmt.Errorf("%s: the id of a %s condition is %s", field, c.Type, id)
			}
			if seen[id] {
				return nil, fmt.Errorf("%s: a plan can have only one %s condition", field, c.Type)
			}
			c.ID = id
			c.Params, err = normalizeLimitParams(field, c.Type, c.Params)
		} else if c.Type == ConditionTypeCustom {
			if !customConditionIDPattern.MatchString(c.ID) {
				return nil, fmt.Errorf("%s: a custom condition needs an id of lowercase letters, digits and hyphens, starting with a letter (at most 63 characters)", field)
			}
			for _, builtInID := range builtInConditionIDs {
				if c.ID == builtInID {
					return nil, fmt.Errorf("%s: %s is the id of a built-in condition", field, c.ID)
				}
			}
			if seen[c.ID] {
				return nil, fmt.Errorf("%s: the plan already has a condition %s", field, c.ID)
			}
			c.Params, err = normalizeCustomParams(field, c.Params)
		} else if c.Type == "" {
			return nil, fmt.Errorf("%s: type is required", field)
		} else {
			return nil, fmt.Errorf("%s: type must be one of request-count, tool-call-count, token-count or custom", field)
		}
		if err != nil {
			return nil, err
		}
		seen[c.ID] = true
		out = append(out, c)
	}
	return out, nil
}

func normalizeLimitParams(field, conditionType string, raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil, fmt.Errorf("%s: params with at least one limit is required", field)
	}
	var params conditionLimits
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&params); err != nil {
		return nil, fmt.Errorf("%s: invalid params: %s", field, err.Error())
	}
	if len(params.Limits) == 0 {
		return nil, fmt.Errorf("%s: params.limits needs at least one limit", field)
	}
	if len(params.Limits) > maxConditionWindows {
		return nil, fmt.Errorf("%s: a condition can have at most %d limits", field, maxConditionWindows)
	}
	seen := map[string]bool{}
	for j := range params.Limits {
		w := &params.Limits[j]
		limitField := fmt.Sprintf("%s.params.limits[%d]", field, j)
		if w.Count < 1 {
			return nil, fmt.Errorf("%s: count must be at least 1", limitField)
		}
		if w.TimeAmount == 0 {
			w.TimeAmount = 1
		} else if w.TimeAmount < 0 {
			return nil, fmt.Errorf("%s: timeAmount must be at least 1", limitField)
		}
		w.TimeUnit = strings.ToUpper(strings.TrimSpace(w.TimeUnit))
		if _, ok := conditionUnitNames[w.TimeUnit]; !ok {
			return nil, fmt.Errorf("%s: timeUnit must be one of MINUTE, HOUR, DAY or MONTH", limitField)
		}
		window := fmt.Sprintf("M%d", w.TimeAmount)
		if minutes, ok := windowMinutes[w.TimeUnit]; ok {
			window = fmt.Sprintf("m%d", minutes*w.TimeAmount)
		}
		if seen[window] {
			return nil, fmt.Errorf("%s: the condition already has a limit over %d %s; each limit needs a window of its own length", limitField, w.TimeAmount, w.TimeUnit)
		}
		seen[window] = true
	}
	switch params.OnExceed {
	case "":
		params.OnExceed = "reject"
	case "reject", "allow":
	default:
		return nil, fmt.Errorf("%s: params.onExceed must be reject or allow", field)
	}
	if conditionType == ConditionTypeTokenCount {
		switch params.TokenType {
		case "":
			params.TokenType = "total"
		case "total", "prompt", "completion":
		default:
			return nil, fmt.Errorf("%s: params.tokenType must be total, prompt or completion", field)
		}
	} else if params.TokenType != "" {
		return nil, fmt.Errorf("%s: params.tokenType is for token-count conditions only", field)
	}
	return json.Marshal(params)
}

func normalizeCustomParams(field string, raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, fmt.Errorf("%s: params is required; send {} for a custom condition with no values", field)
	}
	var params map[string]any
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&params); err != nil || params == nil {
		return nil, fmt.Errorf("%s: params must be a JSON object of key-value pairs", field)
	}
	for key := range params {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%s: params has an empty key", field)
		}
	}
	out, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	if len(out) > maxCustomConditionParamsBytes {
		return nil, fmt.Errorf("%s: params must be at most %d bytes", field, maxCustomConditionParamsBytes)
	}
	return out, nil
}

// conditionUnitNames maps condition window units to the gateway's throttle unit names.
var conditionUnitNames = map[string]string{"MINUTE": "Min", "HOUR": "Hour", "DAY": "Day", "MONTH": "Month"}

// SetThrottleLimitFromConditions fills the single throttle limit that subscription-validation v1
// reads from the request-count condition, as the Platform API does: its first one-unit window, or
// else its first window with the same count per one unit (1000 per 2 HOUR is enforced as 1000 per
// Hour, which can admit up to twice the window's count). No request-count condition leaves the plan
// unlimited for v1.
func (p *SubscriptionPlan) SetThrottleLimitFromConditions() {
	p.ThrottleLimitCount = nil
	p.ThrottleLimitUnit = nil
	p.StopOnQuotaReach = true
	for _, c := range p.Conditions {
		if c.Type != ConditionTypeRequestCount {
			continue
		}
		var params conditionLimits
		if err := json.Unmarshal(c.Params, &params); err != nil {
			return
		}
		p.StopOnQuotaReach = params.OnExceed != "allow"
		var pick *conditionWindow
		for i := range params.Limits {
			if _, ok := conditionUnitNames[params.Limits[i].TimeUnit]; !ok {
				continue
			}
			if params.Limits[i].TimeAmount <= 1 {
				pick = &params.Limits[i]
				break
			}
			if pick == nil {
				pick = &params.Limits[i]
			}
		}
		if pick != nil {
			count, unit := pick.Count, conditionUnitNames[pick.TimeUnit]
			p.ThrottleLimitCount = &count
			p.ThrottleLimitUnit = &unit
		}
		return
	}
}

// WithThrottleLimit returns the plan's conditions with the request-count condition replaced by one
// window of count per unit (the gateway's Min, Hour, Day or Month), or removed when count is nil.
// The other conditions are kept. It is how a request that sends the throttle limit fields changes a
// plan that has conditions.
func WithThrottleLimit(conditions []SubscriptionPlanCondition, count *int, unit string, stopOnQuotaReach bool) []SubscriptionPlanCondition {
	out := make([]SubscriptionPlanCondition, 0, len(conditions)+1)
	var requestLimit *SubscriptionPlanCondition
	if count != nil {
		legacy := SubscriptionPlan{ThrottleLimitCount: count, ThrottleLimitUnit: &unit, StopOnQuotaReach: stopOnQuotaReach}
		if derived := legacy.EffectiveConditions(); len(derived) == 1 {
			requestLimit = &derived[0]
		}
	}
	for _, c := range conditions {
		if c.Type != ConditionTypeRequestCount {
			out = append(out, c)
			continue
		}
		if requestLimit != nil {
			out = append(out, *requestLimit)
			requestLimit = nil
		}
	}
	if requestLimit != nil {
		out = append([]SubscriptionPlanCondition{*requestLimit}, out...)
	}
	return out
}

// WithOnExceed returns the conditions with the request-count condition's onExceed set from
// stopOnQuotaReach, keeping its windows. The other conditions are unchanged.
func WithOnExceed(conditions []SubscriptionPlanCondition, stopOnQuotaReach bool) []SubscriptionPlanCondition {
	out := make([]SubscriptionPlanCondition, len(conditions))
	copy(out, conditions)
	for i, c := range out {
		if c.Type != ConditionTypeRequestCount {
			continue
		}
		var params conditionLimits
		if err := json.Unmarshal(c.Params, &params); err != nil {
			continue
		}
		params.OnExceed = "reject"
		if !stopOnQuotaReach {
			params.OnExceed = "allow"
		}
		if data, err := json.Marshal(params); err == nil {
			out[i].Params = data
		}
	}
	return out
}

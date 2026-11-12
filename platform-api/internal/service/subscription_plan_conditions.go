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

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// Limits on what a plan can hold.
const (
	maxPlanConditions             = 20
	maxConditionWindows           = 10
	maxCustomConditionParamsBytes = 4096
)

// customConditionIDPattern is the ID format of a custom condition.
var customConditionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// normalizeConditions validates a plan's conditions and returns them with defaults filled in and the JSON
// written one way, so equal conditions are stored as equal bytes.
func normalizeConditions(conditions []model.SubscriptionPlanCondition) ([]model.SubscriptionPlanCondition, error) {
	if len(conditions) > maxPlanConditions {
		return nil, apperror.ValidationFailed.New(fmt.Sprintf("a plan can have at most %d conditions", maxPlanConditions))
	}
	out := make([]model.SubscriptionPlanCondition, 0, len(conditions))
	seenIDs := map[string]bool{}
	for i, c := range conditions {
		field := fmt.Sprintf("conditions[%d]", i)
		c.Type = strings.TrimSpace(c.Type)
		c.ID = strings.TrimSpace(c.ID)
		var err error
		switch c.Type {
		case model.ConditionTypeRequestCount, model.ConditionTypeToolCallCount, model.ConditionTypeTokenCount:
			id := model.BuiltInConditionIDs[c.Type]
			if c.ID != "" && c.ID != id {
				return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: the id of a %s condition is %s", field, c.Type, id))
			}
			if seenIDs[id] {
				return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: a plan can have only one %s condition", field, c.Type))
			}
			c.ID = id
			c.Params, err = normalizeLimitParams(field, c.Type, c.Params)
		case model.ConditionTypeCustom:
			if !customConditionIDPattern.MatchString(c.ID) {
				return nil, apperror.ValidationFailed.New(fmt.Sprintf(
					"%s: a custom condition needs an id of lowercase letters, digits and hyphens, starting with a letter (at most 63 characters)", field))
			}
			if isBuiltInConditionID(c.ID) {
				return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: %s is the id of a built-in condition", field, c.ID))
			}
			if seenIDs[c.ID] {
				return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: the plan already has a condition %s", field, c.ID))
			}
			c.Params, err = normalizeCustomParams(field, c.Params)
		case "":
			return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: type is required", field))
		default:
			return nil, apperror.ValidationFailed.New(fmt.Sprintf(
				"%s: type must be one of request-count, tool-call-count, token-count or custom", field))
		}
		if err != nil {
			return nil, err
		}
		seenIDs[c.ID] = true
		out = append(out, c)
	}
	return out, nil
}

func isBuiltInConditionID(id string) bool {
	for _, builtIn := range model.BuiltInConditionIDs {
		if id == builtIn {
			return true
		}
	}
	return false
}

// normalizeLimitParams validates the params of a built-in condition and fills in the defaults.
func normalizeLimitParams(field, conditionType string, raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: params with at least one limit is required", field))
	}
	var params model.ConditionLimits
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&params); err != nil {
		return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: invalid params: %s", field, err.Error()))
	}
	if len(params.Limits) == 0 {
		return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: params.limits needs at least one limit", field))
	}
	if len(params.Limits) > maxConditionWindows {
		return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: a condition can have at most %d limits", field, maxConditionWindows))
	}
	seenWindows := map[string]bool{}
	for j := range params.Limits {
		w := &params.Limits[j]
		limitField := fmt.Sprintf("%s.params.limits[%d]", field, j)
		if w.Count < 1 {
			return nil, apperror.ValidationFailed.New(limitField + ": count must be at least 1")
		}
		if w.TimeAmount == 0 {
			w.TimeAmount = 1
		} else if w.TimeAmount < 0 {
			return nil, apperror.ValidationFailed.New(limitField + ": timeAmount must be at least 1")
		}
		w.TimeUnit = strings.ToUpper(strings.TrimSpace(w.TimeUnit))
		if !constants.ValidThrottleLimitUnits[w.TimeUnit] {
			return nil, apperror.ValidationFailed.New(limitField + ": timeUnit must be one of MINUTE, HOUR, DAY or MONTH")
		}
		window := model.WindowKey(w.TimeAmount, w.TimeUnit)
		if seenWindows[window] {
			return nil, apperror.ValidationFailed.New(fmt.Sprintf(
				"%s: the condition already has a limit over %d %s; each limit needs a window of its own length", limitField, w.TimeAmount, w.TimeUnit))
		}
		seenWindows[window] = true
	}
	switch params.OnExceed {
	case "":
		params.OnExceed = model.ConditionOnExceedReject
	case model.ConditionOnExceedReject, model.ConditionOnExceedAllow:
	default:
		return nil, apperror.ValidationFailed.New(field + ": params.onExceed must be reject or allow")
	}
	if conditionType == model.ConditionTypeTokenCount {
		switch params.TokenType {
		case "":
			params.TokenType = model.ConditionTokenTypeTotal
		case model.ConditionTokenTypeTotal, model.ConditionTokenTypePrompt, model.ConditionTokenTypeCompletion:
		default:
			return nil, apperror.ValidationFailed.New(field + ": params.tokenType must be total, prompt or completion")
		}
	} else if params.TokenType != "" {
		return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: params.tokenType is for token-count conditions only", field))
	}
	out, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// normalizeCustomParams validates the key-value pairs of a custom condition: a JSON object of any values.
func normalizeCustomParams(field string, raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: params is required; send {} for a custom condition with no values", field))
	}
	var params map[string]any
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&params); err != nil || params == nil {
		return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: params must be a JSON object of key-value pairs", field))
	}
	for key := range params {
		if strings.TrimSpace(key) == "" {
			return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: params has an empty key", field))
		}
	}
	out, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	if len(out) > maxCustomConditionParamsBytes {
		return nil, apperror.ValidationFailed.New(fmt.Sprintf("%s: params must be at most %d bytes", field, maxCustomConditionParamsBytes))
	}
	return out, nil
}

// sameConditions reports whether two normalized condition lists are equal, in order.
func sameConditions(a, b []model.SubscriptionPlanCondition) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Type != b[i].Type || !bytes.Equal(a[i].Params, b[i].Params) {
			return false
		}
	}
	return true
}

// withRequestLimit replaces the request-count condition with requestLimit, or removes it when nil, and
// keeps the other conditions. Used when a client sends the old "limits" field.
func withRequestLimit(conditions []model.SubscriptionPlanCondition, requestLimit *model.SubscriptionPlanCondition) []model.SubscriptionPlanCondition {
	out := make([]model.SubscriptionPlanCondition, 0, len(conditions)+1)
	replaced := false
	for _, c := range conditions {
		if c.Type != model.ConditionTypeRequestCount {
			out = append(out, c)
			continue
		}
		if requestLimit != nil && !replaced {
			out = append(out, *requestLimit)
			replaced = true
		}
	}
	if requestLimit != nil && !replaced {
		out = append([]model.SubscriptionPlanCondition{*requestLimit}, out...)
	}
	return out
}

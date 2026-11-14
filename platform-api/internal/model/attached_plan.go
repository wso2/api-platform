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

package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// AttachedPlan is a subscription plan attached to an API: its handle, and the name it had and the
// time it was attached, stored so the API can report whether the plan changed since, as a
// publication does for its selected plans.
type AttachedPlan struct {
	Handle      string     `json:"id"`
	DisplayName string     `json:"displayName,omitempty"`
	AttachedAt  *time.Time `json:"attachedAt,omitempty"`
}

// AttachedPlans is an API's attached plans, stored in its configuration under subscriptionPlans.
// It is written as objects and read from objects or from plain handle strings, the form stored
// before the details were kept, so existing configurations read unchanged.
type AttachedPlans []AttachedPlan

// UnmarshalJSON reads a list whose entries are plan handle strings or AttachedPlan objects.
func (p *AttachedPlans) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*p = nil
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	plans := make(AttachedPlans, 0, len(raw))
	for i, entry := range raw {
		plan, err := decodeAttachedPlan(entry)
		if err != nil {
			return fmt.Errorf("subscriptionPlans[%d]: %w", i, err)
		}
		plans = append(plans, plan)
	}
	*p = plans
	return nil
}

func decodeAttachedPlan(entry json.RawMessage) (AttachedPlan, error) {
	trimmed := bytes.TrimSpace(entry)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var handle string
		if err := json.Unmarshal(trimmed, &handle); err != nil {
			return AttachedPlan{}, err
		}
		return AttachedPlan{Handle: handle}, nil
	}
	var plan AttachedPlan
	if err := json.Unmarshal(trimmed, &plan); err != nil {
		return AttachedPlan{}, err
	}
	return plan, nil
}

// Handles returns the plans' handles, in order, for everything that reads the list as handles
// (the gateway deployment, plan usage and the in-use lock).
func (p AttachedPlans) Handles() []string {
	if p == nil {
		return nil
	}
	handles := make([]string, 0, len(p))
	for _, plan := range p {
		handles = append(handles, plan.Handle)
	}
	return handles
}

// AttachedPlansFromHandles returns plans with only their handles set, for a list read from a
// request before the plans' details are looked up.
func AttachedPlansFromHandles(handles []string) AttachedPlans {
	if handles == nil {
		return nil
	}
	plans := make(AttachedPlans, 0, len(handles))
	for _, h := range handles {
		plans = append(plans, AttachedPlan{Handle: h})
	}
	return plans
}

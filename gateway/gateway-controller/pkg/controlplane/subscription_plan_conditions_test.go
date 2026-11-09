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

package controlplane

import (
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// planStorage records subscription plans, on top of the deletion mock.
type planStorage struct {
	*mockStorageForDeletion
	plans map[string]*models.SubscriptionPlan
}

func (m *planStorage) SaveSubscriptionPlan(plan *models.SubscriptionPlan) error {
	cp := *plan
	m.plans[plan.ID] = &cp
	return nil
}

func (m *planStorage) UpdateSubscriptionPlan(plan *models.SubscriptionPlan) error {
	return m.SaveSubscriptionPlan(plan)
}

func (m *planStorage) GetSubscriptionPlanByID(id, _ string) (*models.SubscriptionPlan, error) {
	p, ok := m.plans[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func newPlanEventClient() (*Client, *planStorage) {
	db := &planStorage{mockStorageForDeletion: newMockStorageForDeletion(), plans: map[string]*models.SubscriptionPlan{}}
	client := &Client{
		logger:    slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
		db:        db,
		eventHub:  &mockControlPlaneEventHub{},
		gatewayID: "test-gateway",
	}
	return client, db
}

func planEvent(eventType string, payload map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"type": eventType, "payload": payload,
		"timestamp": time.Now().Format(time.RFC3339), "correlationId": "corr-plan"}
}

// Plan events from the control plane carry the plan's conditions; events from a control plane that
// predates them carry none, and the plan is then held by its throttle limit.
func TestPlanEvents_StoreConditions(t *testing.T) {
	client, db := newPlanEventClient()

	client.handleSubscriptionPlanCreatedEvent(planEvent("subscriptionPlan.created", map[string]interface{}{
		"planId": "plan-1", "planName": "Agents", "status": "ACTIVE", "stopOnQuotaReach": true,
		"throttleLimitCount": 100, "throttleLimitUnit": "Min",
		"conditions": []interface{}{
			map[string]interface{}{"id": "requests", "type": "request-count",
				"params": map[string]interface{}{"limits": []interface{}{map[string]interface{}{"count": 100, "timeAmount": 1, "timeUnit": "MINUTE"}}, "onExceed": "reject"}},
			map[string]interface{}{"id": "tool-calls", "type": "tool-call-count",
				"params": map[string]interface{}{"limits": []interface{}{map[string]interface{}{"count": 5, "timeAmount": 1, "timeUnit": "HOUR"}}, "onExceed": "reject"}},
		},
	}))
	plan := db.plans["plan-1"]
	if plan == nil || len(plan.Conditions) != 2 || plan.Conditions[1].ID != "tool-calls" {
		t.Fatalf("created plan = %+v, want both conditions", plan)
	}

	// An update with other conditions replaces them.
	client.handleSubscriptionPlanUpdatedEvent(planEvent("subscriptionPlan.updated", map[string]interface{}{
		"planId": "plan-1", "planName": "Agents", "status": "ACTIVE", "stopOnQuotaReach": true,
		"conditions": []interface{}{
			map[string]interface{}{"id": "tokens", "type": "token-count",
				"params": map[string]interface{}{"limits": []interface{}{map[string]interface{}{"count": 1000, "timeAmount": 1, "timeUnit": "DAY"}}, "onExceed": "allow", "tokenType": "prompt"}},
			map[string]interface{}{"id": "region", "type": "custom", "params": map[string]interface{}{"zone": "eu"}},
		},
	}))
	plan = db.plans["plan-1"]
	if len(plan.Conditions) != 2 || plan.Conditions[0].ID != "tokens" || plan.Conditions[1].ID != "region" ||
		!strings.Contains(string(plan.Conditions[0].Params), `"tokenType":"prompt"`) || string(plan.Conditions[1].Params) != `{"zone":"eu"}` {
		t.Fatalf("updated plan conditions = %+v, want tokens and region as sent", plan.Conditions)
	}

	client.handleSubscriptionPlanUpdatedEvent(planEvent("subscriptionPlan.updated", map[string]interface{}{
		"planId": "plan-1", "planName": "Agents", "status": "ACTIVE", "stopOnQuotaReach": true,
		"conditions": []interface{}{},
	}))
	if plan := db.plans["plan-1"]; plan.Conditions == nil || len(plan.Conditions) != 0 {
		t.Fatalf("updated plan conditions = %v, want an empty list", plan.Conditions)
	}

	// An older control plane sends no conditions: the plan is then held by its throttle limit.
	client.handleSubscriptionPlanUpdatedEvent(planEvent("subscriptionPlan.updated", map[string]interface{}{
		"planId": "plan-1", "planName": "Agents", "status": "ACTIVE", "stopOnQuotaReach": true,
		"throttleLimitCount": 30, "throttleLimitUnit": "MINUTE",
	}))
	plan = db.plans["plan-1"]
	if plan.Conditions != nil {
		t.Fatalf("conditions = %v, want none recorded", plan.Conditions)
	}
	if effective := plan.EffectiveConditions(); len(effective) != 1 ||
		string(effective[0].Params) != `{"limits":[{"count":30,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}` {
		t.Fatalf("effective conditions = %+v", effective)
	}
}

// A plan whose only window is longer than one unit arrives with the throttle limit the Platform API
// set for subscription-validation v1 (the window's count per one unit); the gateway keeps it as sent,
// next to the condition.
func TestPlanEvents_LongerWindowKeepsTheControlPlaneLimit(t *testing.T) {
	client, db := newPlanEventClient()
	client.handleSubscriptionPlanCreatedEvent(planEvent("subscriptionPlan.created", map[string]interface{}{
		"planId": "plan-2h", "planName": "Two hours", "status": "ACTIVE", "stopOnQuotaReach": true,
		"throttleLimitCount": 1000, "throttleLimitUnit": "Hour",
		"conditions": []interface{}{map[string]interface{}{"id": "requests", "type": "request-count",
			"params": map[string]interface{}{"limits": []interface{}{map[string]interface{}{"count": 1000, "timeAmount": 2, "timeUnit": "HOUR"}}, "onExceed": "reject"}}},
	}))
	plan := db.plans["plan-2h"]
	if plan == nil || plan.ThrottleLimitCount == nil || *plan.ThrottleLimitCount != 1000 || *plan.ThrottleLimitUnit != "Hour" {
		t.Fatalf("plan = %+v, want 1000 per Hour for v1", plan)
	}
	if len(plan.Conditions) != 1 || !strings.Contains(string(plan.Conditions[0].Params), `"timeAmount":2`) {
		t.Fatalf("conditions = %+v, want the 2-hour window kept", plan.Conditions)
	}
}

// An event-count condition from the control plane is stored as sent, next to the plan's throttle
// limit, which stays the request-count one.
func TestPlanEvents_StoreEventCountCondition(t *testing.T) {
	client, db := newPlanEventClient()
	eventCount := map[string]interface{}{"id": "events", "type": "event-count",
		"params": map[string]interface{}{"limits": []interface{}{map[string]interface{}{"count": 500, "timeAmount": 1, "timeUnit": "MINUTE"}}, "onExceed": "allow"}}

	client.handleSubscriptionPlanCreatedEvent(planEvent("subscriptionPlan.created", map[string]interface{}{
		"planId": "plan-ws", "planName": "Streams", "status": "ACTIVE", "stopOnQuotaReach": true,
		"throttleLimitCount": 100, "throttleLimitUnit": "Min",
		"conditions": []interface{}{
			map[string]interface{}{"id": "requests", "type": "request-count",
				"params": map[string]interface{}{"limits": []interface{}{map[string]interface{}{"count": 100, "timeAmount": 1, "timeUnit": "MINUTE"}}, "onExceed": "reject"}},
			eventCount,
		},
	}))
	plan := db.plans["plan-ws"]
	if plan == nil || len(plan.Conditions) != 2 || plan.Conditions[1].ID != "events" || plan.Conditions[1].Type != models.ConditionTypeEventCount ||
		string(plan.Conditions[1].Params) != `{"limits":[{"count":500,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"allow"}` {
		t.Fatalf("created plan = %+v, want the event-count condition as sent", plan)
	}
	if *plan.ThrottleLimitCount != 100 || *plan.ThrottleLimitUnit != "Min" {
		t.Fatalf("throttle limit = %v %v, want 100 per Min", *plan.ThrottleLimitCount, *plan.ThrottleLimitUnit)
	}

	// An update with only an event-count condition leaves the plan without a request limit.
	client.handleSubscriptionPlanUpdatedEvent(planEvent("subscriptionPlan.updated", map[string]interface{}{
		"planId": "plan-ws", "planName": "Streams", "status": "ACTIVE", "stopOnQuotaReach": true,
		"conditions": []interface{}{eventCount},
	}))
	plan = db.plans["plan-ws"]
	if len(plan.Conditions) != 1 || plan.Conditions[0].Type != "event-count" || plan.ThrottleLimitCount != nil {
		t.Fatalf("updated plan = %+v, want only the event-count condition", plan)
	}
	if effective := plan.EffectiveConditions(); len(effective) != 1 || effective[0].ID != "events" {
		t.Fatalf("effective conditions = %+v", effective)
	}
}

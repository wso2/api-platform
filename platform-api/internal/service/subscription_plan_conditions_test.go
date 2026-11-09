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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

func condition(id, conditionType, params string) model.SubscriptionPlanCondition {
	return model.SubscriptionPlanCondition{ID: id, Type: conditionType, Params: []byte(params)}
}

// The stored form fills every default and writes the JSON one way.
func TestNormalizeConditions_StoredForm(t *testing.T) {
	got, err := normalizeConditions([]model.SubscriptionPlanCondition{
		condition("", "request-count", `{"limits":[{"count":100,"timeUnit":"minute"},{"count":10000,"timeAmount":1,"timeUnit":"DAY"}]}`),
		condition("tokens", " token-count ", `{"limits":[{"count":5000,"timeUnit":"HOUR"}],"onExceed":"allow"}`),
		condition("", "event-count", `{"limits":[{"count":50,"timeAmount":0,"timeUnit":"minute"}]}`),
		condition("ip-ranges", "custom", `{ "zone": "eu", "cidr": "10.0.0.0/8", "limit": 50 }`),
		condition("promo", "custom", `{}`),
	})
	if err != nil {
		t.Fatalf("normalizeConditions: %v", err)
	}
	want := []model.SubscriptionPlanCondition{
		condition("requests", "request-count", `{"limits":[{"count":100,"timeAmount":1,"timeUnit":"MINUTE"},{"count":10000,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"reject"}`),
		condition("tokens", "token-count", `{"limits":[{"count":5000,"timeAmount":1,"timeUnit":"HOUR"}],"onExceed":"allow","tokenType":"total"}`),
		condition("events", "event-count", `{"limits":[{"count":50,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}`),
		condition("ip-ranges", "custom", `{"cidr":"10.0.0.0/8","limit":50,"zone":"eu"}`),
		condition("promo", "custom", `{}`),
	}
	if !sameConditions(got, want) {
		for i := range got {
			t.Logf("got[%d] = %s %s %s", i, got[i].ID, got[i].Type, got[i].Params)
		}
		t.Fatal("stored form differs")
	}
}

func TestNormalizeConditions_Refusals(t *testing.T) {
	limit := `{"limits":[{"count":10,"timeUnit":"MINUTE"}]}`
	tests := []struct {
		name       string
		conditions []model.SubscriptionPlanCondition
		message    string
	}{
		{"unknown type", []model.SubscriptionPlanCondition{condition("", "bandwidth", limit)}, "type must be one of request-count, tool-call-count, token-count, event-count or custom"},
		{"no type", []model.SubscriptionPlanCondition{condition("x", "", `{}`)}, "type is required"},
		{"two of one built-in type", []model.SubscriptionPlanCondition{condition("", "request-count", limit), condition("", "request-count", limit)}, "only one request-count"},
		{"another id for a built-in type", []model.SubscriptionPlanCondition{condition("api-calls", "request-count", limit)}, "the id of a request-count condition is requests"},
		{"two event-count", []model.SubscriptionPlanCondition{condition("", "event-count", limit), condition("events", "event-count", limit)}, "only one event-count"},
		{"another id for event-count", []model.SubscriptionPlanCondition{condition("messages", "event-count", limit)}, "the id of a event-count condition is events"},
		{"tokenType on event-count", []model.SubscriptionPlanCondition{condition("", "event-count", `{"limits":[{"count":1,"timeUnit":"DAY"}],"tokenType":"total"}`)}, "token-count conditions only"},
		{"event-count windows of the same length", []model.SubscriptionPlanCondition{condition("", "event-count",
			`{"limits":[{"count":100,"timeAmount":24,"timeUnit":"HOUR"},{"count":200,"timeUnit":"DAY"}]}`)}, "window of its own length"},
		{"too many event-count windows", []model.SubscriptionPlanCondition{condition("", "event-count", `{"limits":[`+
			strings.TrimSuffix(strings.Repeat(`{"count":1,"timeUnit":"MONTH"},`, 11), ",")+`]}`)}, "at most 10 limits"},
		{"custom with the event-count id", []model.SubscriptionPlanCondition{condition("events", "custom", `{}`)}, "id of a built-in condition"},
		{"no limits", []model.SubscriptionPlanCondition{condition("", "tool-call-count", `{"limits":[]}`)}, "at least one limit"},
		{"no params", []model.SubscriptionPlanCondition{condition("", "tool-call-count", ``)}, "params with at least one limit"},
		{"zero count", []model.SubscriptionPlanCondition{condition("", "request-count", `{"limits":[{"count":0,"timeUnit":"MINUTE"}]}`)}, "count must be at least 1"},
		{"unsupported unit", []model.SubscriptionPlanCondition{condition("", "request-count", `{"limits":[{"count":1,"timeUnit":"WEEK"}]}`)}, "timeUnit must be one of"},
		{"windows of the same length", []model.SubscriptionPlanCondition{condition("", "request-count",
			`{"limits":[{"count":100,"timeAmount":60,"timeUnit":"MINUTE"},{"count":200,"timeUnit":"HOUR"}]}`)}, "window of its own length"},
		{"unknown field", []model.SubscriptionPlanCondition{condition("", "request-count", `{"limits":[{"count":1,"timeUnit":"DAY"}],"burst":5}`)}, "invalid params"},
		{"bad onExceed", []model.SubscriptionPlanCondition{condition("", "request-count", `{"limits":[{"count":1,"timeUnit":"DAY"}],"onExceed":"warn"}`)}, "onExceed must be reject or allow"},
		{"tokenType off token-count", []model.SubscriptionPlanCondition{condition("", "request-count", `{"limits":[{"count":1,"timeUnit":"DAY"}],"tokenType":"prompt"}`)}, "token-count conditions only"},
		{"bad tokenType", []model.SubscriptionPlanCondition{condition("", "token-count", `{"limits":[{"count":1,"timeUnit":"DAY"}],"tokenType":"cached"}`)}, "tokenType must be"},
		{"custom without id", []model.SubscriptionPlanCondition{condition("", "custom", `{}`)}, "needs an id"},
		{"custom id pattern", []model.SubscriptionPlanCondition{condition("IP_Ranges", "custom", `{}`)}, "needs an id"},
		{"custom with a built-in id", []model.SubscriptionPlanCondition{condition("tool-calls", "custom", `{}`)}, "id of a built-in condition"},
		{"two custom with one id", []model.SubscriptionPlanCondition{condition("promo", "custom", `{}`), condition("promo", "custom", `{}`)}, "already has a condition promo"},
		{"custom without params", []model.SubscriptionPlanCondition{condition("promo", "custom", ``)}, "params is required"},
		{"custom params null", []model.SubscriptionPlanCondition{condition("promo", "custom", `null`)}, "params is required"},
		{"custom params not an object", []model.SubscriptionPlanCondition{condition("promo", "custom", `["a"]`)}, "JSON object"},
		{"custom params too large", []model.SubscriptionPlanCondition{condition("promo", "custom", `{"v":"`+strings.Repeat("x", 5000)+`"}`)}, "at most 4096 bytes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeConditions(tt.conditions)
			if !apperror.ValidationFailed.Is(err) || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("err = %v, want a validation error containing %q", err, tt.message)
			}
		})
	}

	many := make([]model.SubscriptionPlanCondition, maxPlanConditions+1)
	for i := range many {
		many[i] = condition("c"+strings.Repeat("x", i), "custom", `{}`)
	}
	if _, err := normalizeConditions(many); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("%d conditions: err = %v, want a refusal", len(many), err)
	}
}

// A portal that predates conditions sends only the plan's request limits: they replace only its
// request-count condition, keeping its onExceed and the other conditions.
func TestPortalConditions_LimitsOnlyChangeTheRequestCondition(t *testing.T) {
	current, err := normalizeConditions([]model.SubscriptionPlanCondition{
		condition("", "request-count", `{"limits":[{"count":100,"timeUnit":"MINUTE"}],"onExceed":"allow"}`),
		condition("", "tool-call-count", `{"limits":[{"count":10,"timeUnit":"MINUTE"}]}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := portalConditions(current, nil, model.LegacyLimitWindows{Requests: []model.ConditionWindow{{Count: 500, TimeAmount: 1, TimeUnit: "HOUR"}}})
	if err != nil {
		t.Fatalf("portalConditions: %v", err)
	}
	if len(got) != 2 || string(got[0].Params) != `{"limits":[{"count":500,"timeAmount":1,"timeUnit":"HOUR"}],"onExceed":"allow"}` ||
		got[1].Type != model.ConditionTypeToolCallCount {
		t.Fatalf("conditions = %+v, want the new soft request limit and the tool-call condition", got)
	}

	cleared, err := portalConditions(current, nil, model.LegacyLimitWindows{})
	if err != nil || len(cleared) != 1 || cleared[0].Type != model.ConditionTypeToolCallCount {
		t.Fatalf("no request limits: %+v, %v; want only the tool-call condition", cleared, err)
	}

	sent := []model.SubscriptionPlanCondition{condition("promo", "custom", `{"until":"2026-12-31"}`)}
	replaced, err := portalConditions(current, &sent, model.LegacyLimitWindows{Requests: []model.ConditionWindow{{Count: 1, TimeAmount: 1, TimeUnit: "DAY"}}})
	if err != nil || len(replaced) != 1 || replaced[0].ID != "promo" {
		t.Fatalf("sent conditions: %+v, %v; want them as they are", replaced, err)
	}
}

// A portal that predates conditions sends request and event limits: they set the request-count and
// event-count conditions, each keeping its onExceed, and keep the other conditions. A type with no
// limits loses its condition.
func TestPortalConditions_LimitsSetRequestAndEventConditions(t *testing.T) {
	current, err := normalizeConditions([]model.SubscriptionPlanCondition{
		condition("", "tool-call-count", `{"limits":[{"count":10,"timeUnit":"MINUTE"}]}`),
		condition("", "event-count", `{"limits":[{"count":100,"timeUnit":"MINUTE"}],"onExceed":"allow"}`),
		condition("promo", "custom", `{}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := portalConditions(current, nil, model.LegacyLimitWindows{
		Requests: []model.ConditionWindow{{Count: 500, TimeAmount: 1, TimeUnit: "HOUR"}},
		Events:   []model.ConditionWindow{{Count: 50, TimeAmount: 1, TimeUnit: "MINUTE"}, {Count: 900, TimeAmount: 1, TimeUnit: "DAY"}},
	})
	if err != nil {
		t.Fatalf("portalConditions: %v", err)
	}
	want := []model.SubscriptionPlanCondition{
		condition("requests", "request-count", `{"limits":[{"count":500,"timeAmount":1,"timeUnit":"HOUR"}],"onExceed":"reject"}`),
		current[0],
		condition("events", "event-count", `{"limits":[{"count":50,"timeAmount":1,"timeUnit":"MINUTE"},{"count":900,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"allow"}`),
		current[2],
	}
	if !sameConditions(got, want) {
		t.Fatalf("conditions = %+v, want %+v", got, want)
	}

	// Event limits on a plan without an event-count condition add one, which blocks.
	added, err := portalConditions(current[:1], nil, model.LegacyLimitWindows{Events: []model.ConditionWindow{{Count: 7, TimeAmount: 1, TimeUnit: "DAY"}}})
	if err != nil || len(added) != 2 || added[0].ID != "events" ||
		string(added[0].Params) != `{"limits":[{"count":7,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"reject"}` {
		t.Fatalf("added: %+v, %v; want a new event-count condition first", added, err)
	}

	// Only request limits: the event-count condition is removed.
	onlyRequests, err := portalConditions(current, nil, model.LegacyLimitWindows{Requests: []model.ConditionWindow{{Count: 1, TimeAmount: 1, TimeUnit: "DAY"}}})
	if err != nil || len(onlyRequests) != 3 || model.FindCondition(onlyRequests, model.ConditionTypeEventCount) != nil ||
		model.FindCondition(onlyRequests, model.ConditionTypeRequestCount) == nil {
		t.Fatalf("only request limits: %+v, %v; want no event-count condition", onlyRequests, err)
	}

	// Sent conditions are taken as they are, event-count included.
	sent := []model.SubscriptionPlanCondition{condition("", "event-count", `{"limits":[{"count":3,"timeUnit":"HOUR"}]}`)}
	replaced, err := portalConditions(current, &sent, model.LegacyLimitWindows{})
	if err != nil || len(replaced) != 1 || replaced[0].ID != "events" {
		t.Fatalf("sent conditions: %+v, %v; want the event-count condition", replaced, err)
	}
}

// A portal copy gets every window of the plan's request-count condition, with its time amount.
func TestPortalLimits_EveryRequestWindow(t *testing.T) {
	plan := &model.SubscriptionPlan{Conditions: []model.SubscriptionPlanCondition{
		condition("tool-calls", "tool-call-count", `{"limits":[{"count":5,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}`),
		condition("requests", "request-count", `{"limits":[{"count":100,"timeAmount":1,"timeUnit":"MINUTE"},{"count":1000,"timeAmount":2,"timeUnit":"HOUR"}],"onExceed":"reject"}`),
	}}
	got := portalLimits(plan)
	want := []portalPlanLimit{{LimitType: "REQUEST_COUNT", LimitCount: 100, TimeUnit: "MINUTE", TimeAmount: 1},
		{LimitType: "REQUEST_COUNT", LimitCount: 1000, TimeUnit: "HOUR", TimeAmount: 2}}
	if !sameLimits(want, got) {
		t.Fatalf("portal limits = %+v, want %+v", got, want)
	}
	if limits := portalLimits(&model.SubscriptionPlan{}); limits == nil || len(limits) != 0 {
		t.Fatalf("a plan with no request-count condition: %+v, want an empty list", limits)
	}
}

// A portal copy gets the event-count windows as EVENT_COUNT limits, next to the REQUEST_COUNT ones.
func TestPortalLimits_EventWindows(t *testing.T) {
	plan := &model.SubscriptionPlan{Conditions: []model.SubscriptionPlanCondition{
		condition("events", "event-count", `{"limits":[{"count":60,"timeAmount":1,"timeUnit":"MINUTE"},{"count":5000,"timeAmount":2,"timeUnit":"DAY"}],"onExceed":"allow"}`),
		condition("requests", "request-count", `{"limits":[{"count":100,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}`),
	}}
	got := portalLimits(plan)
	want := []portalPlanLimit{{LimitType: "REQUEST_COUNT", LimitCount: 100, TimeUnit: "MINUTE", TimeAmount: 1},
		{LimitType: "EVENT_COUNT", LimitCount: 60, TimeUnit: "MINUTE", TimeAmount: 1},
		{LimitType: "EVENT_COUNT", LimitCount: 5000, TimeUnit: "DAY", TimeAmount: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("portal limits = %+v, want %+v", got, want)
	}
	onlyEvents := &model.SubscriptionPlan{Conditions: plan.Conditions[:1]}
	if limits := portalLimits(onlyEvents); len(limits) != 2 || limits[0].LimitType != "EVENT_COUNT" {
		t.Fatalf("a plan with only event-count: %+v, want its two EVENT_COUNT limits", limits)
	}
}

// withRequestLimit replaces only the request-count condition, puts a new one first, and removes it
// when given none.
func TestWithRequestLimit(t *testing.T) {
	custom := condition("promo", "custom", `{}`)
	old := *requestLimit(1, "MINUTE")
	next := *requestLimit(2, "HOUR")

	got := withRequestLimit([]model.SubscriptionPlanCondition{custom, old}, &next)
	if len(got) != 2 || got[0].ID != "promo" || string(got[1].Params) != string(next.Params) {
		t.Fatalf("replace: %+v", got)
	}
	got = withRequestLimit([]model.SubscriptionPlanCondition{custom}, &next)
	if len(got) != 2 || got[0].ID != "requests" || got[1].ID != "promo" {
		t.Fatalf("add: %+v, want the request limit first", got)
	}
	got = withRequestLimit([]model.SubscriptionPlanCondition{old, custom}, nil)
	if len(got) != 1 || got[0].ID != "promo" {
		t.Fatalf("remove: %+v", got)
	}
}

func TestSameConditions(t *testing.T) {
	a := []model.SubscriptionPlanCondition{*requestLimit(1, "MINUTE"), condition("promo", "custom", `{}`)}
	if !sameConditions(a, []model.SubscriptionPlanCondition{*requestLimit(1, "MINUTE"), condition("promo", "custom", `{}`)}) {
		t.Error("equal lists reported different")
	}
	for name, b := range map[string][]model.SubscriptionPlanCondition{
		"other order":  {condition("promo", "custom", `{}`), *requestLimit(1, "MINUTE")},
		"other params": {*requestLimit(2, "MINUTE"), condition("promo", "custom", `{}`)},
		"shorter":      {*requestLimit(1, "MINUTE")},
		"other id":     {*requestLimit(1, "MINUTE"), condition("promo2", "custom", `{}`)},
	} {
		if sameConditions(a, b) {
			t.Errorf("%s: reported the same", name)
		}
	}
	if !sameConditions(nil, []model.SubscriptionPlanCondition{}) {
		t.Error("nil and empty differ")
	}
}

// A plan that can't be stored because the database has no conditions table yet gets its own error
// with a clear message; other errors pass through.
func TestPlanStorageError(t *testing.T) {
	err := planStorageError(fmt.Errorf("save: %w", repository.ErrPlanConditionsTableMissing))
	if !apperror.SubscriptionPlanConditionsUnsupported.Is(err) || !strings.Contains(err.Error(), "database is upgraded") {
		t.Fatalf("err = %v, want SubscriptionPlanConditionsUnsupported", err)
	}
	other := errors.New("disk full")
	if got := planStorageError(other); got != other {
		t.Fatalf("err = %v, want it unchanged", got)
	}
}

// The gateway plan sync carries each plan's conditions next to its legacy request limit, whose unit
// is in the gateway's names.
func TestListSubscriptionPlansForOrg_CarriesConditions(t *testing.T) {
	plan := &model.SubscriptionPlan{UUID: "p1", Handle: "gold", Name: "Gold", Status: model.SubscriptionPlanStatusActive,
		Conditions: []model.SubscriptionPlanCondition{*requestLimit(100, "MINUTE"), condition("promo", "custom", `{"k":"v"}`)}}
	plan.SetLegacyLimit()
	svc := &GatewayInternalAPIService{subscriptionPlanRepo: &fakePlanRepo{plan: plan}}

	items, err := svc.ListSubscriptionPlansForOrg("org-1")
	if err != nil || len(items) != 1 {
		t.Fatalf("ListSubscriptionPlansForOrg = %v, %v", items, err)
	}
	got := items[0]
	if len(got.Conditions) != 2 || got.Conditions[1].ID != "promo" {
		t.Errorf("conditions = %+v", got.Conditions)
	}
	if got.ThrottleLimitCount == nil || *got.ThrottleLimitCount != 100 || got.ThrottleLimitUnit != "Min" || !got.StopOnQuotaReach {
		t.Errorf("legacy limit = %v %q %v", got.ThrottleLimitCount, got.ThrottleLimitUnit, got.StopOnQuotaReach)
	}
	body, err := json.Marshal(got)
	if err != nil || !strings.Contains(string(body), `"conditions":[{"id":"requests","type":"request-count","params":{"limits"`) {
		t.Errorf("JSON = %s, %v", body, err)
	}
}

// Plan events to the gateways carry the conditions, and an empty list rather than null.
func TestPlanEvents_CarryConditions(t *testing.T) {
	created, _ := json.Marshal(&model.SubscriptionPlanCreatedEvent{PlanId: "p1", PlanName: "Gold",
		Conditions: []model.SubscriptionPlanCondition{*requestLimit(5, "DAY")}})
	if !strings.Contains(string(created), `"conditions":[{"id":"requests"`) {
		t.Errorf("created event = %s", created)
	}
	updated, _ := json.Marshal(&model.SubscriptionPlanUpdatedEvent{PlanId: "p1", Conditions: []model.SubscriptionPlanCondition{}})
	if !strings.Contains(string(updated), `"conditions":[]`) {
		t.Errorf("updated event = %s", updated)
	}
}

func TestGatewayThrottleUnit(t *testing.T) {
	for in, want := range map[string]string{"MINUTE": "Min", "HOUR": "Hour", "DAY": "Day", "MONTH": "Month", "minute": "Min", "": "", "Min": "Min", "WEEK": "WEEK"} {
		if got := gatewayThrottleUnit(in); got != want {
			t.Errorf("gatewayThrottleUnit(%q) = %q, want %q", in, got, want)
		}
	}
}

// planEventHub records the plan events published to gateways.
type planEventHub struct {
	dpNoopEventHub
	events []dto.GatewayEventDTO
}

func (h *planEventHub) PublishEvent(_ string, e eventhub.Event) error {
	var ev dto.GatewayEventDTO
	if err := json.Unmarshal([]byte(e.EventData), &ev); err != nil {
		return err
	}
	h.events = append(h.events, ev)
	return nil
}

// Plan events to gateways carry the conditions and the legacy unit in the gateway's names.
func TestPlanEvents_SentToGateways(t *testing.T) {
	hub := &planEventHub{}
	repo := &webhookPlanRepo{plans: map[string]*model.SubscriptionPlan{}}
	svc := NewSubscriptionPlanService(repo, &stubOrgGatewaysRepo{gateways: []*model.Gateway{{ID: "gw-1"}}}, nil,
		NewGatewayEventsService(hub, nil, slog.Default()), nil, nil)

	if _, err := svc.CreatePlan("org-1", "", &model.SubscriptionPlan{Handle: "gold", Name: "Gold",
		Conditions: []model.SubscriptionPlanCondition{*requestLimit(100, "MINUTE")}}, nil); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	name := "Gold"
	if _, err := svc.UpdatePlan("gold", "org-1", "", &model.SubscriptionPlanUpdate{Name: &name, RequestLimit: requestLimit(5, "DAY")}); err != nil {
		t.Fatalf("UpdatePlan: %v", err)
	}
	if len(hub.events) != 2 || hub.events[0].Type != "subscriptionPlan.created" || hub.events[1].Type != "subscriptionPlan.updated" {
		t.Fatalf("events = %+v", hub.events)
	}
	for i, want := range []string{"Min", "Day"} {
		payload, _ := json.Marshal(hub.events[i].Payload)
		var got struct {
			ThrottleLimitUnit string                            `json:"throttleLimitUnit"`
			Conditions        []model.SubscriptionPlanCondition `json:"conditions"`
		}
		if err := json.Unmarshal(payload, &got); err != nil {
			t.Fatal(err)
		}
		if got.ThrottleLimitUnit != want || len(got.Conditions) != 1 || got.Conditions[0].ID != "requests" {
			t.Errorf("event %d payload = %s, want unit %s and the requests condition", i, payload, want)
		}
	}
}

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
	"context"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// meteredSync is a sync of the plans gold (paid, charging for calls and tokens) and free, with the
// two meters kept here.
func meteredSync(plans ...string) PlanSync {
	unit, tokenKey := "0.01", "metadata.aiTokenUsage.promptTokens"
	usage := func(meters ...string) *model.SubscriptionPlanPricing {
		pricing := &model.SubscriptionPlanPricing{PricingType: model.PricingTypeUsageBased, Currency: "USD", BillingPeriod: "MONTH"}
		for _, meter := range meters {
			pricing.Prices = append(pricing.Prices, model.SubscriptionPlanPrice{MeterHandle: meter,
				PricingModel: model.PricingModelPerUnit, UnitAmount: &unit})
		}
		return pricing
	}
	return PlanSync{
		Plans:   testPlans(plans...),
		Pricing: map[string]*model.SubscriptionPlanPricing{"uuid-gold": usage("calls", "tokens", "calls"), "uuid-silver": usage("calls")},
		Meters: map[string]*model.Meter{
			"calls": {Handle: "calls", DisplayName: "Calls", Type: model.MeterTypeRequestCount,
				FilterFields: []model.MeterFilterField{{Field: "HTTP_METHOD", EventKey: "request.verb"}}},
			"tokens": {Handle: "tokens", DisplayName: "Tokens", Description: "Prompt tokens", Type: model.MeterTypeTokens, TypeKey: &tokenKey,
				FilterFields: []model.MeterFilterField{}},
		},
	}
}

// The portal's copy of the calls meter, as its REST API gives it.
func portalCalls() map[string]any {
	return map[string]any{"id": "calls", "displayName": "Calls", "description": nil, "type": "REQUEST_COUNT", "typeKey": nil,
		"filterFields": []any{map[string]any{"field": "HTTP_METHOD", "eventKey": "request.verb"}}, "planCount": 0}
}

func runSync(t *testing.T, pp *planPortal, s PlanSync) (PlanSyncResult, error) {
	t.Helper()
	srv := pp.serve(t)
	return newTestHTTPPortalPublisher(t, "test-shared-key").SyncPlans(context.Background(), &model.APIPortal{URL: srv.URL}, s)
}

// A plan the portal lacks is created there after the meters its prices charge for. Each meter is
// sent once, as it is kept here; a meter the portal already has with the same settings is used as
// it is.
func TestSyncPlans_CreatesTheMetersAMissingPlanChargesForFirst(t *testing.T) {
	pp := &planPortal{meters: map[string]map[string]any{"calls": portalCalls()}}
	created, err := runSync(t, pp, meteredSync("gold"))
	if err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	description, tokenKey := "Prompt tokens", "metadata.aiTokenUsage.promptTokens"
	want := []portalMeter{{ID: "tokens", DisplayName: "Tokens", Description: &description, Type: "TOKENS", TypeKey: &tokenKey,
		FilterFields: []portalMeterFilterField{}}}
	if !reflect.DeepEqual(pp.postedMeters, want) {
		t.Fatalf("posted meters %+v, want only tokens, as kept here", pp.postedMeters)
	}
	if got := postedIDs(pp); !slices.Equal(got, []string{"gold"}) {
		t.Fatalf("posted plans %v", got)
	}
	if !reflect.DeepEqual(created, PlanSyncResult{Plans: []string{"gold"}, Meters: []string{"tokens"}}) {
		t.Fatalf("result %+v, want what was created", created)
	}
}

// A plan the portal already has needs nothing more: its meters are there with it.
func TestSyncPlans_APlanThePortalHasSendsNoMeters(t *testing.T) {
	pp := &planPortal{handles: []string{"silver"}, listed: map[string]map[string]any{"silver": {"id": "silver", "displayName": "Plan silver",
		"limits": []any{}, "pricing": map[string]any{"pricingType": "USAGE_BASED", "currency": "USD", "billingPeriod": "MONTH",
			"prices": []any{map[string]any{"meterId": "calls", "pricingModel": "PER_UNIT", "unitAmount": "0.01", "filters": []any{}}}}}}}
	created, err := runSync(t, pp, meteredSync("silver"))
	if err != nil || len(pp.postedMeters) != 0 || len(created.Meters) != 0 || len(created.Plans) != 0 {
		t.Fatalf("SyncPlans = %+v, %v; posted meters %+v", created, err, pp.postedMeters)
	}
}

// A meter copy with other settings can't change (a plan there may price it): nothing is written.
func TestSyncPlans_RefusesAMeterCopyWithOtherSettings(t *testing.T) {
	for name, change := range map[string]func(map[string]any){
		"name": func(m map[string]any) { m["displayName"] = "API calls" },
		"type": func(m map[string]any) { m["type"] = "TOKENS"; m["typeKey"] = "x" },
		"filter field": func(m map[string]any) {
			m["filterFields"] = []any{map[string]any{"field": "HTTP_METHOD", "eventKey": "verb"}}
		},
		"description": func(m map[string]any) { m["description"] = "Every call" },
	} {
		calls := portalCalls()
		change(calls)
		pp := &planPortal{meters: map[string]map[string]any{"calls": calls}}
		_, err := runSync(t, pp, meteredSync("gold"))
		var conflict *PortalMeterConflictError
		if !errors.As(err, &conflict) || conflict.Handle != "calls" {
			t.Errorf("%s: want a meter conflict on calls, got %v", name, err)
		}
		if len(pp.postedMeters) != 0 || len(pp.posted) != 0 || len(pp.deleted) != 0 {
			t.Errorf("%s: nothing may be written, posted meters %+v plans %v", name, pp.postedMeters, postedIDs(pp))
		}
	}
	// The same settings written differently are the same meter.
	calls := portalCalls()
	calls["displayName"], calls["description"] = " Calls ", ""
	if _, err := runSync(t, &planPortal{meters: map[string]map[string]any{"calls": calls, "tokens": {"id": "tokens", "displayName": "Tokens",
		"description": "Prompt tokens", "type": "TOKENS", "typeKey": "metadata.aiTokenUsage.promptTokens", "filterFields": []any{}}}},
		meteredSync("gold")); err != nil {
		t.Fatalf("the same meters: %v", err)
	}
}

// A write that fails takes back what the sync created before it: plans, then meters.
func TestSyncPlans_AFailedWriteRemovesWhatTheSyncCreated(t *testing.T) {
	pp := &planPortal{createStatus: http.StatusBadRequest}
	if _, err := runSync(t, pp, meteredSync("gold")); err == nil {
		t.Fatal("want the plan create's failure")
	}
	if !slices.Equal(pp.deleted, []string{"/meters/tokens", "/meters/calls"}) {
		t.Fatalf("deleted %v, want both new meters removed, last first", pp.deleted)
	}

	pp = &planPortal{handles: []string{"free"}, updateStatus: http.StatusBadRequest,
		listed: map[string]map[string]any{"free": {"id": "free", "displayName": "Old name", "limits": []any{}}}}
	if _, err := runSync(t, pp, meteredSync("gold", "free")); err == nil {
		t.Fatal("want the plan update's failure")
	}
	if !slices.Equal(pp.deleted, []string{"/subscription-plans/gold", "/meters/tokens", "/meters/calls"}) {
		t.Fatalf("deleted %v, want the new plan, then the new meters", pp.deleted)
	}

	// A meter create that fails takes back the meters created before it.
	pp = &planPortal{meterCreateStatus: http.StatusBadRequest}
	if _, err := runSync(t, pp, meteredSync("gold")); err == nil || len(pp.deleted) != 0 || len(pp.posted) != 0 {
		t.Fatalf("the first meter create failing: %v, deleted %v, posted plans %v", err, pp.deleted, postedIDs(pp))
	}
}

// UndoPlanSync removes plans, then meters. One already gone is fine; one the portal keeps is
// reported.
func TestUndoPlanSync_RemovesPlansThenMeters(t *testing.T) {
	pp := &planPortal{deleteStatus: map[string]int{"/meters/tokens": http.StatusNotFound, "/subscription-plans/gold": http.StatusConflict}}
	srv := pp.serve(t)
	err := newTestHTTPPortalPublisher(t, "test-shared-key").UndoPlanSync(context.Background(), &model.APIPortal{URL: srv.URL},
		PlanSyncResult{Plans: []string{"silver", "gold"}, Meters: []string{"calls", "tokens"}})
	if !slices.Equal(pp.deleted, []string{"/subscription-plans/gold", "/subscription-plans/silver", "/meters/tokens", "/meters/calls"}) {
		t.Fatalf("deleted %v", pp.deleted)
	}
	if err == nil || !strings.Contains(err.Error(), "did not remove plan gold (status 409)") || strings.Contains(err.Error(), "tokens") {
		t.Fatalf("want only gold reported, got %v", err)
	}
}

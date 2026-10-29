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
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// newTestMeters has two meters: api-calls, counting API requests, whose prices may filter on the
// property Status code, and llm-input, adding up the input tokens of AI API requests, whose prices
// may filter on the properties API type and Prompt tokens, the number the meter adds up.
func newTestMeters() *fakeMeterRepo {
	inputKey := "metadata.aiTokenUsage.promptTokens"
	return &fakeMeterRepo{meters: map[string]*model.Meter{
		"api-calls": {Handle: "api-calls", DisplayName: "API calls", Type: model.MeterTypeRequestCount,
			FilterFields: []model.MeterFilterField{{Field: "Status code", EventKey: "response.status"}}},
		"llm-input": {Handle: "llm-input", DisplayName: "LLM input tokens", Type: model.MeterTypeTokens,
			TypeKey: &inputKey, FilterFields: []model.MeterFilterField{{Field: "API type", EventKey: "metadata.apiType"},
				{Field: "Prompt tokens", EventKey: inputKey}}},
	}}
}

// newAdminTestService has the meters of newTestMeters and the plans gold and silver.
func newAdminTestService() (*MonetizationAdminService, *fakeMeterRepo, *fakePricingRepo) {
	meters := newTestMeters()
	pricing := &fakePricingRepo{pricing: map[string]*model.SubscriptionPlanPricing{}}
	limit := 100
	plans := &fakeHandlePlanRepo{plans: map[string]*model.SubscriptionPlan{
		"gold":   {UUID: "plan-1", Handle: "gold", Name: "Gold", ThrottleLimitCount: &limit, ThrottleLimitUnit: "MINUTE"},
		"silver": {UUID: "plan-2", Handle: "silver", Name: "Silver"},
	}}
	return NewMonetizationAdminService(meters, pricing, plans, nil, nil), meters, pricing
}

func TestMonetizationAdmin_ReadsComeFromTheStore(t *testing.T) {
	svc, _, pricing := newAdminTestService()
	if _, err := svc.GetMeter("org", "api-calls"); err != nil {
		t.Fatalf("GetMeter: %v", err)
	}
	if _, err := svc.GetMeter("org", "missing"); !apperror.MeterNotFound.Is(err) {
		t.Fatalf("expected METER_NOT_FOUND, got %v", err)
	}
	if _, err := svc.GetPlanPricing("org", "gold"); !apperror.SubscriptionPlanPricingNotFound.Is(err) {
		t.Fatalf("a free plan has no pricing, got %v", err)
	}
	pricing.pricing["plan-1"] = &model.SubscriptionPlanPricing{Currency: "USD", BillingPeriod: "MONTH"}
	if p, err := svc.GetPlanPricing("org", "gold"); err != nil || p.Currency != "USD" {
		t.Fatalf("GetPlanPricing = %+v, %v", p, err)
	}
	if _, err := svc.GetPlanPricing("org", "missing"); !apperror.SubscriptionPlanNotFound.Is(err) {
		t.Fatalf("expected SUBSCRIPTION_PLAN_NOT_FOUND, got %v", err)
	}
}

// A meter is kept here, like a plan: it reaches an API Portal only with a plan published there.
func TestMonetizationAdmin_MetersAreKeptHere(t *testing.T) {
	svc, meters, _ := newAdminTestService()
	// A custom token key, and filter properties named by the request, one of them reading the token key.
	body := `{"displayName":" Output tokens ","type":"TOKENS","typeKey":"usage.output_tokens",
		"filterFields":[{"field":"API type","eventKey":"custom.api.kind"},{"field":" Output ","eventKey":" usage.output_tokens "}]}`
	created, err := svc.CreateMeter(context.Background(), "org", "", []byte(body))
	if err != nil {
		t.Fatalf("CreateMeter: %v", err)
	}
	stored := meters.meters["output-tokens"]
	want := []model.MeterFilterField{{Field: "API type", EventKey: "custom.api.kind"}, {Field: "Output", EventKey: "usage.output_tokens"}}
	if stored == nil || created.Handle != "output-tokens" || stored.DisplayName != "Output tokens" || stored.OrganizationUUID != "org" ||
		*stored.TypeKey != "usage.output_tokens" || !reflect.DeepEqual(stored.FilterFields, want) || stored.SourceUpdatedAt.IsZero() {
		t.Fatalf("stored %+v, want the meter as the API Portal stores it, its handle derived from the name", stored)
	}

	_, err = svc.CreateMeter(context.Background(), "org", "", []byte(`{"id":"api-calls","displayName":"Calls","type":"REQUEST_COUNT"}`))
	var appErr *apperror.Error
	if !apperror.MeterExists.Is(err) || !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusConflict ||
		appErr.Message != `A meter with id "api-calls" already exists in this organization.` {
		t.Fatalf("an id in use: want 409 METER_EXISTS, got %v", err)
	}

	update := `{"displayName":"Calls","type":"REQUEST_COUNT","filterFields":[{"field":"Status code","eventKey":"status_code"}]}`
	updated, err := svc.UpdateMeter(context.Background(), "org", "", "api-calls", []byte(update))
	if err != nil || updated.Handle != "api-calls" || meters.meters["api-calls"].DisplayName != "Calls" ||
		meters.meters["api-calls"].FilterFields[0].EventKey != "status_code" {
		t.Fatalf("UpdateMeter = %+v, %v; stored %+v", updated, err, meters.meters["api-calls"])
	}
	renamed := `{"id":"calls","displayName":"Calls","type":"REQUEST_COUNT"}`
	if _, err := svc.UpdateMeter(context.Background(), "org", "", "api-calls", []byte(renamed)); !apperror.ValidationFailed.Is(err) ||
		err.Error() == "" || !errors.As(err, &appErr) || appErr.Message != "The id of a meter cannot be changed" {
		t.Fatalf("another id: want it refused, got %v", err)
	}
	if _, err := svc.UpdateMeter(context.Background(), "org", "", "missing", []byte(update)); !apperror.MeterNotFound.Is(err) {
		t.Fatalf("updating a missing meter: want METER_NOT_FOUND, got %v", err)
	}
	if err := svc.DeleteMeter(context.Background(), "org", "", "api-calls"); err != nil || meters.meters["api-calls"] != nil {
		t.Fatalf("DeleteMeter: %v, stored %+v", err, meters.meters["api-calls"])
	}
	if err := svc.DeleteMeter(context.Background(), "org", "", "api-calls"); !apperror.MeterNotFound.Is(err) {
		t.Fatalf("deleting a missing meter: want METER_NOT_FOUND, got %v", err)
	}
}

// A request the API Portal would refuse is refused here, in its words (model.ParseMeterRequest).
func TestMonetizationAdmin_MeterRequestsAreCheckedLikeThePortal(t *testing.T) {
	for _, conditions := range []string{`[{"field":"API type","operator":"IN"}]`, `[]`, `null`} {
		svc, meters, _ := newAdminTestService()
		body := []byte(`{"displayName":"LLM input tokens","type":"TOKENS","typeKey":"metadata.aiTokenUsage.promptTokens","conditions":` + conditions + `}`)
		_, createErr := svc.CreateMeter(context.Background(), "org", "", body)
		_, updateErr := svc.UpdateMeter(context.Background(), "org", "", "llm-input", body)
		for _, err := range []error{createErr, updateErr} {
			var appErr *apperror.Error
			if !apperror.ValidationFailed.Is(err) || !errors.As(err, &appErr) ||
				appErr.Message != "A meter has no conditions; each plan price gives its own filters" {
				t.Errorf("conditions %s: want them refused, got %v", conditions, err)
			}
		}
		if len(meters.meters) != 2 || meters.upserts != 0 {
			t.Errorf("conditions %s: nothing may be stored", conditions)
		}
	}
	svc, _, _ := newAdminTestService()
	_, err := svc.CreateMeter(context.Background(), "org", "", []byte(`{"displayName":"Calls","type":"REQUEST_COUNT","color":"red"}`))
	var appErr *apperror.Error
	if !apperror.ValidationFailed.Is(err) || !errors.As(err, &appErr) || appErr.Message != "color is not a known field" {
		t.Fatalf("an unknown field: want 400 naming it, got %v", err)
	}
}

// pricedOnMeter is a usage pricing with one price on the meter, filtering it on the given fields.
func pricedOnMeter(planUUID, meter string, fields ...string) *model.SubscriptionPlanPricing {
	unit := "1"
	price := model.SubscriptionPlanPrice{MeterHandle: meter, PricingModel: model.PricingModelPerUnit, UnitAmount: &unit,
		Filters: []model.PriceFilter{}}
	for _, field := range fields {
		price.Filters = append(price.Filters, model.PriceFilter{Field: field, Operator: model.FilterOperatorEquals, Values: []string{"x"}})
	}
	return &model.SubscriptionPlanPricing{SubscriptionPlanUUID: planUUID, PricingType: model.PricingTypeUsageBased,
		Currency: "USD", BillingPeriod: "MONTH", Prices: []model.SubscriptionPlanPrice{price}}
}

func TestMonetizationAdmin_ListMetersSearchesNameAndHandle(t *testing.T) {
	svc, _, _ := newAdminTestService()
	cases := map[string][]string{
		"":         {"api-calls", "llm-input"},
		"LLM":      {"llm-input"},
		"api-c":    {"api-calls"},
		" calls ":  {"api-calls"},
		"tokens":   {"llm-input"},
		"platinum": {},
	}
	for search, want := range cases {
		meters, err := svc.ListMeters("org", search)
		if err != nil {
			t.Fatalf("ListMeters(%q): %v", search, err)
		}
		got := []string{}
		for _, m := range meters {
			got = append(got, m.Handle)
		}
		if !slices.Equal(got, want) {
			t.Errorf("ListMeters(%q) = %v, want %v", search, got, want)
		}
	}
}

func TestMonetizationAdmin_MeterUsageNamesThePlansThatPriceEachMeter(t *testing.T) {
	svc, _, pricing := newAdminTestService()
	gold := pricedOnMeter("plan-1", "llm-input", "API type")
	// gold prices llm-input twice and api-calls once: each plan is named once per meter.
	gold.Prices = append(gold.Prices, pricedOnMeter("plan-1", "llm-input").Prices[0], pricedOnMeter("plan-1", "api-calls").Prices[0])
	pricing.pricing = map[string]*model.SubscriptionPlanPricing{"plan-2": pricedOnMeter("plan-2", "llm-input"), "plan-1": gold}
	usage, err := svc.MeterUsage("org")
	if err != nil {
		t.Fatalf("MeterUsage: %v", err)
	}
	want := map[string][]string{"llm-input": {"gold", "silver"}, "api-calls": {"gold"}}
	if len(usage) != len(want) || !slices.Equal(usage["llm-input"], want["llm-input"]) || !slices.Equal(usage["api-calls"], want["api-calls"]) {
		t.Fatalf("MeterUsage = %v, want %v", usage, want)
	}
	pricing.pricing = map[string]*model.SubscriptionPlanPricing{}
	if usage, err := svc.MeterUsage("org"); err != nil || len(usage) != 0 {
		t.Fatalf("with no pricing, MeterUsage = %v, %v", usage, err)
	}
}

// A meter a plan prices cannot change at all, its name and description included: the plan's prices
// were made from it.
func TestMonetizationAdmin_MeterPricedHereCannotChange(t *testing.T) {
	valid := `{"displayName":"Prompt tokens","type":"TOKENS","typeKey":"metadata.aiTokenUsage.promptTokens"}`
	cases := []struct {
		name    string
		pricing map[string]*model.SubscriptionPlanPricing
		body    string
		msg     string // empty when the update is made
	}{
		{"one plan", map[string]*model.SubscriptionPlanPricing{"plan-1": pricedOnMeter("plan-1", "llm-input", "API type")},
			`{"displayName":"LLM input tokens","type":"TOKENS","typeKey":"metadata.aiTokenUsage.promptTokens","filterFields":[{"field":"API type","eventKey":"metadata.apiType"}]}`,
			"The meter is in use, so it can't be updated."},
		{"only the event keys", map[string]*model.SubscriptionPlanPricing{"plan-1": pricedOnMeter("plan-1", "llm-input", "API type")},
			`{"displayName":"LLM input tokens","type":"TOKENS","typeKey":"usage.input_tokens",
			  "filterFields":[{"field":"API type","eventKey":"custom.api.kind"}]}`,
			"The meter is in use, so it can't be updated."},
		{"several plans", map[string]*model.SubscriptionPlanPricing{
			"plan-2": pricedOnMeter("plan-2", "llm-input"), "plan-1": pricedOnMeter("plan-1", "llm-input", "API type")},
			valid, "The meter is in use, so it can't be updated."},
		{"a price without filters", map[string]*model.SubscriptionPlanPricing{"plan-1": pricedOnMeter("plan-1", "llm-input")},
			valid, "The meter is in use, so it can't be updated."},
		{"another meter's price", map[string]*model.SubscriptionPlanPricing{"plan-1": pricedOnMeter("plan-1", "api-calls")}, valid, ""},
		{"no pricing", map[string]*model.SubscriptionPlanPricing{}, valid, ""},
	}
	for _, c := range cases {
		svc, meters, pricing := newAdminTestService()
		pricing.pricing = c.pricing
		_, err := svc.UpdateMeter(context.Background(), "org", "", "llm-input", []byte(c.body))
		if c.msg == "" {
			if err != nil || meters.meters["llm-input"].DisplayName != "Prompt tokens" {
				t.Errorf("%s: want the update made, got %v", c.name, err)
			}
			continue
		}
		var appErr *apperror.Error
		if !apperror.MeterInUse.Is(err) || !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusConflict || appErr.Message != c.msg {
			t.Errorf("%s: want 409 METER_IN_USE %q, got %v", c.name, c.msg, err)
		}
		if meters.upserts != 0 {
			t.Errorf("%s: nothing may be stored", c.name)
		}
	}
}

func TestMonetizationAdmin_MeterPricedHereCannotBeDeleted(t *testing.T) {
	svc, meters, pricing := newAdminTestService()
	pricing.pricing["plan-2"] = pricedOnMeter("plan-2", "llm-input")
	pricing.pricing["plan-1"] = pricedOnMeter("plan-1", "llm-input", "API type")

	err := svc.DeleteMeter(context.Background(), "org", "", "llm-input")
	var appErr *apperror.Error
	if !apperror.MeterInUse.Is(err) || !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusConflict ||
		appErr.Message != "The meter is in use, so it can't be deleted." {
		t.Fatalf("want 409 METER_IN_USE, got %v", err)
	}
	delete(pricing.pricing, "plan-2")
	err = svc.DeleteMeter(context.Background(), "org", "", "llm-input")
	if !apperror.MeterInUse.Is(err) || !errors.As(err, &appErr) ||
		appErr.Message != "The meter is in use, so it can't be deleted." {
		t.Fatalf("want 409 METER_IN_USE with one plan, got %v", err)
	}
	if meters.deletes != 0 || meters.meters["llm-input"] == nil {
		t.Fatal("a priced meter must stay")
	}
}

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
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// newPlanTestService has the plan gold.
func newPlanTestService() (*SubscriptionPlanService, *webhookPlanRepo) {
	limit := 100
	repo := &webhookPlanRepo{plans: map[string]*model.SubscriptionPlan{
		"gold": {UUID: "u1", Handle: "gold", Name: "Gold", ThrottleLimitCount: &limit, ThrottleLimitUnit: "MINUTE",
			StopOnQuotaReach: true, Status: model.SubscriptionPlanStatusActive},
	}}
	return NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil), repo
}

// A plan is deleted through the REST API only while nothing here offers or uses it.
func TestDeletePlan_IsRefusedWhileInUse(t *testing.T) {
	svc, repo := newPlanTestService()
	repo.usage = model.SubscriptionPlanUsage{RESTAPIs: 1, Subscriptions: 2}

	err := svc.DeletePlan("gold", "org-1", "actor")
	var appErr *apperror.Error
	if !apperror.SubscriptionPlanInUse.Is(err) || !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusConflict ||
		appErr.Message != "The plan Gold is in use and can't be deleted." {
		t.Fatalf("want 409 SUBSCRIPTION_PLAN_IN_USE, got %v", err)
	}
	if repo.deletes != 0 {
		t.Fatalf("nothing may be deleted, deletes = %d", repo.deletes)
	}

	repo.usage = model.SubscriptionPlanUsage{}
	if err := svc.DeletePlan("gold", "org-1", "actor"); err != nil || repo.deletes != 1 {
		t.Fatalf("unused plan: err = %v, deletes = %d", err, repo.deletes)
	}
	if err := svc.DeletePlan("gold", "org-1", "actor"); !apperror.SubscriptionPlanNotFound.Is(err) {
		t.Fatalf("a deleted plan is not found, got %v", err)
	}
}

// newPricingTestService creates plans with the meters of newTestMeters.
func newPricingTestService() (*SubscriptionPlanService, *webhookPlanRepo) {
	repo := &webhookPlanRepo{plans: map[string]*model.SubscriptionPlan{}}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)
	svc.SetPricingRepositories(newTestMeters(), &fakePricingRepo{pricing: map[string]*model.SubscriptionPlanPricing{}})
	return svc, repo
}

// threeUsagePrices is a usage based pricing request, the example in the REST API contract:
//   - input tokens of LlmProvider APIs;
//   - input tokens of two other API types (a second price on the same meter);
//   - graduated prices for API calls, without filters.
const threeUsagePrices = `{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[
	{"meterId":"llm-input","filters":[{"field":" api type ","operator":"equals","values":[" LlmProvider "]}],
	 "pricingModel":"per_unit","unitAmount":"2.50","unitQuantity":1000000,"tiers":null},
	{"meterId":"llm-input","filters":[{"field":"API TYPE","operator":"IN","values":["Mcp","LlmProxy","Mcp"]}],
	 "pricingModel":"PER_UNIT","unitAmount":"0.15","unitQuantity":1000000,"tiers":null},
	{"meterId":"api-calls","filters":[],"pricingModel":"GRADUATED","unitAmount":null,
	 "tiers":[{"upTo":10000,"unitAmount":"0","flatAmount":null},{"upTo":null,"unitAmount":"0.001","flatAmount":null}]}]}`

// A plan's pricing is checked and stored with the plan, in the same create.
func TestCreatePlan_StoresThePricingWithThePlan(t *testing.T) {
	svc, repo := newPricingTestService()
	if _, err := svc.CreatePlan("org", "actor", &model.SubscriptionPlan{Handle: "gold", Name: "Gold"},
		[]byte(`{"pricingType":"flat_rate","currency":"usd","billingPeriod":"MONTH","flatAmount":"99.00"}`)); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	// Stored as checked: codes upper-cased, and dated now.
	if p := repo.pricing; p == nil || p.PricingType != "FLAT_RATE" || p.Currency != "USD" || *p.FlatAmount != "99.00" ||
		len(p.Prices) != 0 || p.CreatedAt.IsZero() {
		t.Fatalf("stored %+v", p)
	}

	if _, err := svc.CreatePlan("org", "actor", &model.SubscriptionPlan{Handle: "silver", Name: "Silver"}, []byte(threeUsagePrices)); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	usage := repo.pricing
	if usage.PricingType != "USAGE_BASED" || usage.FlatAmount != nil || len(usage.Prices) != 3 {
		t.Fatalf("usage pricing %+v", usage)
	}
	// One meter priced twice, for different API types; codes upper-cased, each filter naming its
	// property as the meter spells it, and filter values normalized.
	if provider := usage.Prices[0]; provider.MeterHandle != "llm-input" || provider.PricingModel != "PER_UNIT" || *provider.UnitQuantity != 1000000 ||
		!reflect.DeepEqual(provider.Filters, []model.PriceFilter{{Field: "API type", Operator: "EQUALS", Values: []string{"LlmProvider"}}}) {
		t.Errorf("first price = %+v", provider)
	}
	if mini := usage.Prices[1]; mini.MeterHandle != "llm-input" ||
		!reflect.DeepEqual(mini.Filters, []model.PriceFilter{{Field: "API type", Operator: "IN", Values: []string{"Mcp", "LlmProxy"}}}) {
		t.Errorf("second price = %+v", mini)
	}
	if calls := usage.Prices[2]; calls.MeterHandle != "api-calls" || calls.PricingModel != "GRADUATED" ||
		calls.Filters == nil || len(calls.Filters) != 0 || len(calls.Tiers) != 2 || calls.Tiers[1].UpTo != nil {
		t.Errorf("third price = %+v", calls)
	}

	// A free plan has no pricing.
	if _, err := svc.CreatePlan("org", "actor", &model.SubscriptionPlan{Handle: "free", Name: "Free"}, nil); err != nil || repo.pricing != nil {
		t.Fatalf("free plan: err = %v, pricing = %+v", err, repo.pricing)
	}
}

// A comparison names one of the meter's properties in any case, and its number may be a JSON number.
// It is stored with the meter's spelling of the name and the number in digits.
func TestCreatePlan_StoresComparisons(t *testing.T) {
	svc, repo := newPricingTestService()
	body := `{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[
		{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"1","filters":[
			{"field":" prompt tokens ","operator":"greater_than","values":[333]}]},
		{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"2","filters":[
			{"field":"Prompt tokens","operator":"LESS_THAN_OR_EQUAL","values":["0333"]}]}]}`
	if _, err := svc.CreatePlan("org", "actor", &model.SubscriptionPlan{Handle: "gold", Name: "Gold"}, []byte(body)); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	for i, want := range [][]model.PriceFilter{
		{{Field: "Prompt tokens", Operator: "GREATER_THAN", Values: []string{"333"}}},
		{{Field: "Prompt tokens", Operator: "LESS_THAN_OR_EQUAL", Values: []string{"333"}}},
	} {
		if got := repo.pricing.Prices[i].Filters; !reflect.DeepEqual(got, want) {
			t.Fatalf("filters of price %d = %+v, want %+v", i+1, got, want)
		}
	}

	// The API Portal gets the filter by the property's name, with no event key.
	sent, err := json.Marshal(portalPricing(repo.pricing).Prices[0].Filters)
	if err != nil || string(sent) != `[{"field":"Prompt tokens","operator":"GREATER_THAN","values":["333"]}]` {
		t.Fatalf("sent to the portal %s, %v", sent, err)
	}
}

func TestCreatePlan_PricingIsCheckedBeforeThePlanIsCreated(t *testing.T) {
	calls := `{"meterId":"api-calls","pricingModel":"PER_UNIT","unitAmount":"0.01"}`
	usage := func(prices ...string) string {
		return `{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[` + strings.Join(prices, ",") + `]}`
	}
	byAPIType := func(operator, values string) string {
		return `{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"1","filters":[{"field":"API type","operator":"` + operator +
			`","values":` + values + `}]}`
	}
	twentyOne := make([]string, 21)
	for i := range twentyOne {
		twentyOne[i] = fmt.Sprintf(`{"meterId":"meter-%d","pricingModel":"PER_UNIT","unitAmount":"1"}`, i)
	}
	cases := []struct{ body, msg string }{
		// A field the pricing does not take is refused, and the message says where it is, in the
		// API Portal's words.
		{`{"pricingModel":"PER_UNIT","currency":"USD","billingPeriod":"MONTH","unitAmount":"1","meterId":"api-calls"}`,
			"pricingModel is not a known field"},
		{usage(`{"meterId":"api-calls","pricingModel":"PER_UNIT","unitAmount":"1","recurringFee":"9"}`),
			"recurringFee of price 1 is not a known field"},
		{usage(calls, `{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"1","conditionValues":[{"field":"API type","values":["LlmProvider"]}]}`),
			"conditionValues of price 2 is not a known field"},
		{usage(`{"displayName":"API calls","meterId":"api-calls","pricingModel":"PER_UNIT","unitAmount":"1"}`),
			"displayName of price 1 is not a known field"},
		{usage(`{"meterId":"api-calls","pricingModel":"PER_UNIT","unitQuantitiy":1000,"unitAmount":"1"}`),
			"unitQuantitiy of price 1 is not a known field"},
		{usage(`{"meterId":"api-calls","pricingModel":"PER_UNIT","unitAmount":"1","filters":[{"field":"Status code","operator":"EQUALS","values":["200"]},{"field":"Status code","operator":"EQUALS","value":"404"}]}`),
			"value of filter 2 of price 1 is not a known field"},
		{usage(`{"meterId":"api-calls","pricingModel":"VOLUME","tiers":[{"upTo":10,"unitAmount":"1"},{"upto":null,"unitAmount":"0.5"}]}`),
			"upto of tier 2 of price 1 is not a known field"},
		// A part's own fields are checked before the parts it holds, as the portal's schema check does.
		{`{"pricingType":"USAGE_BASED","currency":"USD","prices":[{"colour":"red"}],"period":"MONTH"}`,
			"period is not a known field"},
		{`{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"1","createdAt":"2026-10-22T00:00:00Z"}`,
			"createdAt is read-only"},
		{`{"currency":"USD","billingPeriod":"MONTH","flatAmount":"20.00"}`, "pricingType must be one of FLAT_RATE, USAGE_BASED"},
		{`{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"20.00","prices":[` + calls + `]}`,
			"A flat rate plan has no usage prices"},
		{`{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH"}`, "flatAmount is required"},
		{`{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"0"}`,
			"flatAmount must be more than 0. Create a free plan instead."},
		{`{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","flatAmount":"20.00","prices":[` + calls + `]}`,
			"A usage based plan has no flat amount"},
		{usage(), "A usage based plan needs 1 to 20 prices"},
		{usage(twentyOne...), "A usage based plan needs 1 to 20 prices"},
		{usage(byAPIType("IN", `["LlmProvider","Mcp"]`), calls, byAPIType("NOT_EQUALS", `["LlmProxy"]`)),
			"Prices 1 and 3 both count the same requests of the meter llm-input; give them different filters."},
		{usage(calls, `{"meterId":"api-calls","pricingModel":"VOLUME","tiers":[{"upTo":null,"unitAmount":"1"}]}`),
			"Prices 1 and 2 both count the same requests of the meter api-calls; give them different filters."},
		{usage(`{"meterId":"api-calls","pricingModel":"VOLUME","tiers":[{"upTo":100,"unitAmount":"1"},{"upTo":100,"unitAmount":"1"},{"upTo":null,"unitAmount":"1"}]}`),
			"Each tier of price 1 needs an upTo greater than the tier before it"},
		{usage(calls, byAPIType("MATCHES", `["Llm"]`)),
			"The operator of filter 1 of price 2 must be one of EQUALS, NOT_EQUALS, IN, NOT_IN, STARTS_WITH, GREATER_THAN, GREATER_THAN_OR_EQUAL, LESS_THAN, LESS_THAN_OR_EQUAL"},
		{usage(byAPIType("EQUALS", `["LlmProvider","Agent"]`)), "price 1: Operator EQUALS takes exactly one value"},
		{usage(`{"meterId":"api-calls","pricingModel":"PER_UNIT","unitAmount":"1","filters":[{"field":"Status code","operator":"GREATER_THAN","values":["4xx"]}]}`),
			"price 1: A comparison on Status code takes a whole number from 0 to 1000000000000"},
		{usage(`{"meterId":"api-calls","pricingModel":"PER_UNIT","unitAmount":"1","filters":[{"field":"Status code","operator":"GREATER_THAN","values":["499"]},{"field":"status code","operator":"NOT_EQUALS","values":["503"]}]}`),
			"price 1 filters on Status code more than once"},
		{usage(`{"meterId":"api-calls","pricingModel":"PER_UNIT","unitAmount":"1","filters":[{"field":"` + strings.Repeat("n", 41) + `","operator":"EQUALS","values":["a"]}]}`),
			"field of filter 1 of price 1 must be 1 to 40 characters"},
		{usage(`{"meterId":"api-calls","pricingModel":"PER_UNIT","unitAmount":"1","filters":[{"field":"Status code","operator":"EQUALS","values":[true]}]}`),
			"The request body is not a valid plan pricing"},
		{usage(`{"meterId":"api-calls","pricingModel":"FLAT"}`), "pricingModel of price 1 must be one of PER_UNIT, VOLUME, GRADUATED"},
		{usage(`{"meterId":"api-calls","pricingModel":"VOLUME","tiers":[{"upTo":10,"unitAmount":"1"},{"upTo":null,"unitAmount":"0.5"}],"unitAmount":"1"}`),
			"A VOLUME price (price 1) does not take unitAmount"},
		{`{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"WEEK","flatAmount":"1"}`, "billingPeriod must be one of MONTH, YEAR"},
		{usage(calls, `{"meterId":"no-such-meter","pricingModel":"PER_UNIT","unitAmount":"1"}`), "Unknown meter: no-such-meter"},
		// Each filter names one of its meter's properties: api-calls has Status code, llm-input API type
		// and Prompt tokens.
		{usage(calls, `{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"1","filters":[{"field":"HTTP method","operator":"EQUALS","values":["POST"]}]}`),
			"price 2: the meter llm-input has no property HTTP method"},
		{usage(`{"meterId":"api-calls","pricingModel":"PER_UNIT","unitAmount":"1","filters":[{"field":"Status code","operator":"EQUALS","values":["200"]},{"field":"api type","operator":"EQUALS","values":["LlmProvider"]}]}`),
			"price 1: the meter api-calls has no property api type"},
		{usage(`{"meterId":"api-calls","pricingModel":"PER_UNIT","unitAmount":"1","filters":[{"field":"Prompt tokens","operator":"GREATER_THAN","values":["0"]}]}`),
			"price 1: the meter api-calls has no property Prompt tokens"},
		// A filter reads the key of the property it names, so it has no event key of its own.
		{usage(`{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"1","filters":[{"field":"API type","eventKey":"metadata.apiType","operator":"EQUALS","values":["LlmProvider"]}]}`),
			"eventKey of filter 1 of price 1 is not a known field"},
		{`[1]`, "The request body is not a valid plan pricing"},
	}
	for _, c := range cases {
		svc, repo := newPricingTestService()
		_, err := svc.CreatePlan("org", "actor", &model.SubscriptionPlan{Handle: "gold", Name: "Gold"}, []byte(c.body))
		var appErr *apperror.Error
		if !apperror.ValidationFailed.Is(err) || !errors.As(err, &appErr) || appErr.Message != c.msg {
			t.Errorf("%s: want validation error %q, got %v", c.body, c.msg, err)
		}
		if repo.creates != 0 {
			t.Errorf("%s: no plan may be created with invalid pricing", c.body)
		}
	}
}

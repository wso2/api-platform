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
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestGatewayStatus_StopsAccessOnlyWhenBillingHasEnded(t *testing.T) {
	cases := []struct {
		status  SubscriptionStatus
		billing *SubscriptionBilling
		want    string
	}{
		{SubscriptionStatusActive, nil, "ACTIVE"},
		{SubscriptionStatusInactive, nil, "INACTIVE"},
		{SubscriptionStatusActive, &SubscriptionBilling{Status: BillingStatusActive}, "ACTIVE"},
		{SubscriptionStatusActive, &SubscriptionBilling{Status: BillingStatusPastDue}, "ACTIVE"},
		{SubscriptionStatusActive, &SubscriptionBilling{Status: BillingStatusUnpaid}, "INACTIVE"},
		{SubscriptionStatusActive, &SubscriptionBilling{Status: BillingStatusCancelled}, "INACTIVE"},
		// A subscriber's own suspension stays in force whatever the billing status.
		{SubscriptionStatusInactive, &SubscriptionBilling{Status: BillingStatusActive}, "INACTIVE"},
	}
	for _, c := range cases {
		if got := GatewayStatus(c.status, c.billing); got != c.want {
			t.Errorf("GatewayStatus(%s, %+v) = %s, want %s", c.status, c.billing, got, c.want)
		}
	}
}

func TestValidateMeter_TypeAndTypeKey(t *testing.T) {
	str := func(s string) *string { return &s }
	for _, m := range []*Meter{
		{Type: MeterTypeRequestCount},
		{Type: MeterTypeTokens, TypeKey: str("metadata.aiTokenUsage.promptTokens")},
		// Any key the user writes, for a gateway that names its analytics fields differently.
		{Type: MeterTypeTokens, TypeKey: str("usage.output_tokens")},
		{Type: MeterTypeTokens, TypeKey: str(strings.Repeat("k", 255))},
	} {
		if err := ValidateMeter(m); err != nil {
			t.Errorf("%s %v: %v", m.Type, m.TypeKey, err)
		}
	}

	cases := map[string]struct {
		meter   *Meter
		message string
	}{
		"unknown type":         {&Meter{Type: "CALLS"}, "type must be one of REQUEST_COUNT, TOKENS"},
		"tokens without a key": {&Meter{Type: MeterTypeTokens}, "A TOKENS meter needs a typeKey"},
		"tokens with an empty key": {&Meter{Type: MeterTypeTokens, TypeKey: str("")},
			"typeKey must be 1 to 255 characters with no spaces"},
		"tokens with a long key": {&Meter{Type: MeterTypeTokens, TypeKey: str(strings.Repeat("k", 256))},
			"typeKey must be 1 to 255 characters with no spaces"},
		"tokens with a space in the key": {&Meter{Type: MeterTypeTokens, TypeKey: str("metadata.ai tokens")},
			"typeKey must be 1 to 255 characters with no spaces"},
		"tokens with a control character": {&Meter{Type: MeterTypeTokens, TypeKey: str("metadata.tokens\x00")},
			"typeKey must be 1 to 255 characters with no spaces"},
		"requests with a key": {&Meter{Type: MeterTypeRequestCount, TypeKey: str("metadata.aiTokenUsage.promptTokens")},
			"typeKey applies only to a TOKENS meter"},
	}
	for name, c := range cases {
		if err := ValidateMeter(c.meter); err == nil || err.Error() != c.message {
			t.Errorf("%s: got %v, want %q", name, err, c.message)
		}
	}
}

func TestValidateMeter_FilterFields(t *testing.T) {
	all := &Meter{Type: MeterTypeRequestCount, FilterFields: []MeterFilterField{
		{Field: FilterFieldHTTPStatusCode, EventKey: "response.status"},
		{Field: FilterFieldHTTPMethod, EventKey: "request.verb"},
		{Field: FilterFieldResource, EventKey: "metadata.apiResourceTemplate"},
		{Field: FilterFieldAPIType, EventKey: "custom.api.kind"},
	}}
	if err := ValidateMeter(all); err != nil {
		t.Fatalf("every filter field once: %v", err)
	}
	// API type is allowed on a TOKENS meter as well.
	key := "metadata.aiTokenUsage.promptTokens"
	if err := ValidateMeter(&Meter{Type: MeterTypeTokens, TypeKey: &key,
		FilterFields: []MeterFilterField{{Field: FilterFieldAPIType, EventKey: "custom.kind"}}}); err != nil {
		t.Fatalf("API type on a TOKENS meter: %v", err)
	}
	if !all.AllowsFilterOn(FilterFieldAPIType) || (&Meter{}).AllowsFilterOn(FilterFieldAPIType) {
		t.Errorf("AllowsFilterOn must follow the meter's filter fields")
	}

	cases := map[string]struct {
		fields  []MeterFilterField
		message string
	}{
		"unknown field": {[]MeterFilterField{{Field: "TOKEN_TYPE", EventKey: "tokenType"}}, "filterFields: unknown field TOKEN_TYPE"},
		"repeated field": {[]MeterFilterField{{Field: FilterFieldAPIType, EventKey: "metadata.apiType"},
			{Field: FilterFieldAPIType, EventKey: "metadata.apiType"}}, "filterFields: API_TYPE is listed twice"},
		"no event key": {[]MeterFilterField{{Field: FilterFieldHTTPMethod}},
			"The event key of HTTP method must be 1 to 255 characters with no spaces"},
		"long event key": {[]MeterFilterField{{Field: FilterFieldResource, EventKey: strings.Repeat("k", 256)}},
			"The event key of Resource must be 1 to 255 characters with no spaces"},
		"event key with a space": {[]MeterFilterField{{Field: FilterFieldHTTPStatusCode, EventKey: "response status"}},
			"The event key of Status code must be 1 to 255 characters with no spaces"},
		"API type without an event key": {[]MeterFilterField{{Field: FilterFieldAPIType}},
			"The event key of API type must be 1 to 255 characters with no spaces"},
		"repeated API type": {[]MeterFilterField{{Field: FilterFieldAPIType, EventKey: "metadata.apiType"},
			{Field: FilterFieldAPIType, EventKey: "metadata.apiType"}}, "filterFields: API_TYPE is listed twice"},
	}
	for name, c := range cases {
		if err := ValidateMeter(&Meter{Type: MeterTypeRequestCount, FilterFields: c.fields}); err == nil || err.Error() != c.message {
			t.Errorf("%s: got %v, want %q", name, err, c.message)
		}
	}
}

func filter(field, operator string, values ...string) PriceFilter {
	return PriceFilter{Field: field, Operator: operator, Values: values}
}

func usagePrice(meter string, filters ...PriceFilter) SubscriptionPlanPrice {
	unit := "0.01"
	return SubscriptionPlanPrice{MeterHandle: meter, Filters: filters, PricingModel: PricingModelPerUnit, UnitAmount: &unit}
}

func usageBased(prices ...SubscriptionPlanPrice) *SubscriptionPlanPricing {
	return &SubscriptionPlanPricing{PricingType: PricingTypeUsageBased, Currency: "USD", BillingPeriod: BillingPeriodMonth, Prices: prices}
}

func TestValidatePlanPricing_FlatRateOrUsageBased(t *testing.T) {
	str := func(s string) *string { return &s }
	num := func(n int64) *int64 { return &n }
	tiered := func(model string, tiers ...PriceTier) SubscriptionPlanPrice {
		return SubscriptionPlanPrice{MeterHandle: "api-calls", PricingModel: model, Tiers: tiers}
	}
	flatRate := func(amount *string, prices ...SubscriptionPlanPrice) *SubscriptionPlanPricing {
		return &SubscriptionPlanPricing{PricingType: PricingTypeFlatRate, Currency: "USD", BillingPeriod: BillingPeriodMonth,
			FlatAmount: amount, Prices: prices}
	}
	with := func(p SubscriptionPlanPrice, change func(*SubscriptionPlanPrice)) SubscriptionPlanPrice {
		change(&p)
		return p
	}
	onEachMeter := func(n int) []SubscriptionPlanPrice {
		prices := make([]SubscriptionPlanPrice, n)
		for i := range prices {
			prices[i] = usagePrice(fmt.Sprintf("meter-%d", i))
		}
		return prices
	}

	valid := map[string]*SubscriptionPlanPricing{
		"flat rate":                   flatRate(str("99.00")),
		"flat rate with empty prices": flatRate(str("20"), []SubscriptionPlanPrice{}...),
		"one price": usageBased(with(usagePrice("llm-input"), func(p *SubscriptionPlanPrice) {
			p.UnitAmount = str("2.50")
			p.UnitQuantity = num(1_000_000)
		})),
		"one meter priced twice, for different API types": usageBased(
			usagePrice("llm-input", filter("API_TYPE", "EQUALS", "LlmProvider")),
			usagePrice("llm-input", filter("API_TYPE", "IN", "Mcp", "LlmProxy")),
			tiered(PricingModelGraduated, PriceTier{UpTo: num(1000), UnitAmount: str("0")}, PriceTier{UnitAmount: str("0.001")})),
		"volume tiers":                       usageBased(tiered(PricingModelVolume, PriceTier{UpTo: num(1000), FlatAmount: str("10")}, PriceTier{UnitAmount: str("0.005")})),
		"20 prices":                          usageBased(onEachMeter(20)...),
		"an offered currency other than USD": &SubscriptionPlanPricing{PricingType: PricingTypeFlatRate, Currency: "AFN", BillingPeriod: BillingPeriodMonth, FlatAmount: str("1")},
	}
	for name, p := range valid {
		if err := ValidatePlanPricing(p); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	invalid := map[string]struct {
		pricing *SubscriptionPlanPricing
		message string
	}{
		"no pricing type":          {&SubscriptionPlanPricing{Currency: "USD", BillingPeriod: BillingPeriodMonth, FlatAmount: str("1")}, "pricingType must be one of FLAT_RATE, USAGE_BASED"},
		"flat rate with prices":    {flatRate(str("20"), usagePrice("api-calls")), "A flat rate plan has no usage prices"},
		"flat rate without amount": {flatRate(nil), "flatAmount is required"},
		"flat rate of zero":        {flatRate(str("0.00")), "flatAmount must be more than 0. Create a free plan instead."},
		"flat rate bad amount":     {flatRate(str("9,99")), "flatAmount must be a decimal amount such as 9.99"},
		"usage based with an amount": {&SubscriptionPlanPricing{PricingType: PricingTypeUsageBased, Currency: "USD", BillingPeriod: BillingPeriodMonth,
			FlatAmount: str("20"), Prices: []SubscriptionPlanPrice{usagePrice("api-calls")}}, "A usage based plan has no flat amount"},
		"usage based without prices": {usageBased(), "A usage based plan needs 1 to 20 prices"},
		"21 prices":                  {usageBased(onEachMeter(21)...), "A usage based plan needs 1 to 20 prices"},
		"lower-case currency": {&SubscriptionPlanPricing{PricingType: PricingTypeFlatRate, Currency: "usd", BillingPeriod: BillingPeriodMonth, FlatAmount: str("1")},
			"currency must be the code of a currency with two decimal places, such as USD or EUR. Currencies with no or three decimal places cannot be used."},
		"zero-decimal currency": {&SubscriptionPlanPricing{PricingType: PricingTypeFlatRate, Currency: "JPY", BillingPeriod: BillingPeriodMonth, FlatAmount: str("500")},
			"currency must be the code of a currency with two decimal places, such as USD or EUR. Currencies with no or three decimal places cannot be used."},
		"three-decimal currency": {&SubscriptionPlanPricing{PricingType: PricingTypeFlatRate, Currency: "KWD", BillingPeriod: BillingPeriodMonth, FlatAmount: str("5")},
			"currency must be the code of a currency with two decimal places, such as USD or EUR. Currencies with no or three decimal places cannot be used."},
		"currency the portals do not offer": {&SubscriptionPlanPricing{PricingType: PricingTypeFlatRate, Currency: "XYZ", BillingPeriod: BillingPeriodMonth, FlatAmount: str("5")},
			"currency must be the code of a currency with two decimal places, such as USD or EUR. Currencies with no or three decimal places cannot be used."},
		"weekly": {&SubscriptionPlanPricing{PricingType: PricingTypeFlatRate, Currency: "USD", BillingPeriod: "WEEK", FlatAmount: str("1")},
			"billingPeriod must be one of MONTH, YEAR"},
		"flat is not a price model": {usageBased(SubscriptionPlanPrice{MeterHandle: "api-calls", PricingModel: "FLAT"}),
			"pricingModel of price 1 must be one of PER_UNIT, VOLUME, GRADUATED"},
		"price without a meter": {usageBased(usagePrice("api-calls"), usagePrice("")), "meterId of price 2 is required"},
		"per unit bad amount": {usageBased(with(usagePrice("api-calls"), func(p *SubscriptionPlanPrice) { p.UnitAmount = str("1,5") })),
			"unitAmount of price 1 must be a decimal amount such as 9.99"},
		"per unit without amount": {usageBased(with(usagePrice("api-calls"), func(p *SubscriptionPlanPrice) { p.UnitAmount = nil })),
			"unitAmount of price 1 is required"},
		"per unit zero quantity": {usageBased(with(usagePrice("api-calls"), func(p *SubscriptionPlanPrice) { p.UnitQuantity = num(0) })),
			"unitQuantity of price 1 must be a whole number from 1 to 1000000000"},
		"per unit with tiers": {usageBased(with(usagePrice("api-calls"), func(p *SubscriptionPlanPrice) { p.Tiers = []PriceTier{} })),
			"A PER_UNIT price (price 1) does not take tiers"},
		"volume without tiers": {usageBased(tiered(PricingModelVolume)), "price 1 needs 1 to 10 tiers"},
		"volume open middle tier": {usageBased(tiered(PricingModelVolume, PriceTier{UnitAmount: str("1")}, PriceTier{UnitAmount: str("1")})),
			"Each tier of price 1 needs an upTo greater than the tier before it"},
		"graduated decreasing tiers": {usageBased(tiered(PricingModelGraduated, PriceTier{UpTo: num(10), UnitAmount: str("1")},
			PriceTier{UpTo: num(5), UnitAmount: str("1")}, PriceTier{UnitAmount: str("1")})), "Each tier of price 1 needs an upTo greater than the tier before it"},
		"graduated empty tier": {usageBased(tiered(PricingModelGraduated, PriceTier{})), "Tier 1 of price 1 needs a unitAmount, a flatAmount or both"},
		"graduated with a unit amount": {usageBased(with(tiered(PricingModelGraduated, PriceTier{UnitAmount: str("1")}),
			func(p *SubscriptionPlanPrice) { p.UnitAmount = str("1") })), "A GRADUATED price (price 1) does not take unitAmount"},
	}
	for name, c := range invalid {
		if err := ValidatePlanPricing(c.pricing); err == nil || err.Error() != c.message {
			t.Errorf("%s: got %v, want %q", name, err, c.message)
		}
	}
}

func TestValidatePlanPricing_NormalizesFilterValues(t *testing.T) {
	pricing := usageBased(usagePrice("llm-input",
		filter("API_TYPE", "IN", " LlmProvider ", "LlmProxy", "LlmProvider"),
		filter("HTTP_METHOD", "NOT_IN", "get", "Post"),
		filter("HTTP_STATUS_CODE", "EQUALS", " 200 "),
		filter("RESOURCE", "STARTS_WITH", "/orders/")))
	if err := ValidatePlanPricing(pricing); err != nil {
		t.Fatalf("ValidatePlanPricing: %v", err)
	}
	// Filters in the order given; values trimmed, methods upper-cased, repeated values dropped, a
	// starts-with path without its trailing "/".
	want := []PriceFilter{
		filter("API_TYPE", "IN", "LlmProvider", "LlmProxy"),
		filter("HTTP_METHOD", "NOT_IN", "GET", "POST"),
		filter("HTTP_STATUS_CODE", "EQUALS", "200"),
		filter("RESOURCE", "STARTS_WITH", "/orders"),
	}
	if got := pricing.Prices[0].Filters; !reflect.DeepEqual(got, want) {
		t.Errorf("filters = %+v, want %+v", got, want)
	}
}

// API type values are free text, matched as written: trimmed and listed once, but never
// upper-cased.
func TestValidatePlanPricing_APITypeValuesAreKeptAsWritten(t *testing.T) {
	pricing := usageBased(usagePrice("api-calls", filter("API_TYPE", "IN", " RestApi ", "restapi", "Mcp", "RestApi")))
	if err := ValidatePlanPricing(pricing); err != nil {
		t.Fatalf("ValidatePlanPricing: %v", err)
	}
	if got, want := pricing.Prices[0].Filters, []PriceFilter{filter("API_TYPE", "IN", "RestApi", "restapi", "Mcp")}; !reflect.DeepEqual(got, want) {
		t.Errorf("filters = %+v, want %+v", got, want)
	}
	for _, operator := range []string{"EQUALS", "NOT_EQUALS", "NOT_IN"} {
		if err := ValidatePlanPricing(usageBased(usagePrice("api-calls", filter("API_TYPE", operator, "SomeOtherKind")))); err != nil {
			t.Errorf("API type %s any text: %v", operator, err)
		}
	}
}

// A price may filter on all four fields, one filter each.
func TestValidatePlanPricing_AFilterOnEveryField(t *testing.T) {
	pricing := usageBased(usagePrice("api-calls",
		filter("HTTP_STATUS_CODE", "EQUALS", "200"),
		filter("HTTP_METHOD", "EQUALS", "POST"),
		filter("RESOURCE", "STARTS_WITH", "/chat"),
		filter("API_TYPE", "EQUALS", "LlmProvider")))
	if err := ValidatePlanPricing(pricing); err != nil {
		t.Fatalf("four filters: %v", err)
	}
}

func TestValidatePlanPricing_RefusesBadFilters(t *testing.T) {
	manyTypes := make([]string, 21)
	for i := range manyTypes {
		manyTypes[i] = fmt.Sprintf("type-%d", i)
	}
	cases := map[string]struct {
		filter  PriceFilter
		message string
	}{
		"unknown field":                {filter("TOKEN_TYPE", "EQUALS", "INPUT"), "field of filter 1 of price 2 must be one of HTTP_STATUS_CODE, HTTP_METHOD, RESOURCE, API_TYPE, TOKEN"},
		"resource is one of":           {filter("RESOURCE", "IN", "/a"), "The RESOURCE operator of price 2 must be one of EQUALS, NOT_EQUALS, STARTS_WITH"},
		"two values for not equals":    {filter("HTTP_METHOD", "NOT_EQUALS", "GET", "POST"), "price 2: Operator NOT_EQUALS takes exactly one value"},
		"two prefixes":                 {filter("RESOURCE", "STARTS_WITH", "/a", "/b"), "price 2: Operator STARTS_WITH takes exactly one value"},
		"status code class":            {filter("HTTP_STATUS_CODE", "IN", "200", "5xx"), `price 2: "5xx" is not a status code such as 200 or 404`},
		"unknown method":               {filter("HTTP_METHOD", "EQUALS", "FETCH"), "price 2: HTTP method must be one of GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS"},
		"relative resource":            {filter("RESOURCE", "EQUALS", "orders"), "price 2: A resource must start with /"},
		"API type starts with":         {filter("API_TYPE", "STARTS_WITH", "Llm"), "The API_TYPE operator of price 2 must be one of EQUALS, NOT_EQUALS, IN, NOT_IN"},
		"two API types for equals":     {filter("API_TYPE", "EQUALS", "RestApi", "Mcp"), "price 2: Operator EQUALS takes exactly one value"},
		"no API types for is one of":   {filter("API_TYPE", "IN"), "price 2: A filter on API_TYPE needs 1 to 20 values"},
		"21 API types":                 {filter("API_TYPE", "NOT_IN", manyTypes...), "price 2: A filter on API_TYPE needs 1 to 20 values"},
		"blank API type":               {filter("API_TYPE", "EQUALS", " "), "price 2: Filter values for API_TYPE must be 1 to 255 characters"},
		"API type longer than allowed": {filter("API_TYPE", "EQUALS", strings.Repeat("a", 256)), "price 2: Filter values for API_TYPE must be 1 to 255 characters"},
	}
	for name, c := range cases {
		pricing := usageBased(usagePrice("api-calls"), usagePrice("llm-input", c.filter))
		if err := ValidatePlanPricing(pricing); err == nil || err.Error() != c.message {
			t.Errorf("%s: got %v, want %q", name, err, c.message)
		}
	}
	twice := usageBased(usagePrice("llm-input", filter("API_TYPE", "IN", "a"), filter("API_TYPE", "NOT_IN", "b")))
	if err := ValidatePlanPricing(twice); err == nil || err.Error() != "price 1 filters on API_TYPE more than once" {
		t.Errorf("two filters on a field: got %v", err)
	}
}

// Two prices on one meter overlap when a request could match both: on every field either filters
// on. A field only one price filters on matches any value for the other.
func TestValidatePlanPricing_OverlapAcrossFields(t *testing.T) {
	cases := []struct {
		name    string
		prices  []SubscriptionPlanPrice
		message string
	}{
		{"no filters on either", []SubscriptionPlanPrice{usagePrice("api-calls"), usagePrice("api-calls")},
			"Prices 1 and 2 both count the same requests of the meter api-calls; give them different filters."},
		{"a price without filters counts what the other counts", []SubscriptionPlanPrice{
			usagePrice("llm-input", filter("API_TYPE", "EQUALS", "LlmProvider")), usagePrice("llm-input")},
			"Prices 1 and 2 both count the same requests of the meter llm-input; give them different filters."},
		{"filters on different fields", []SubscriptionPlanPrice{
			usagePrice("llm-input", filter("API_TYPE", "EQUALS", "LlmProvider")),
			usagePrice("api-calls"),
			usagePrice("llm-input", filter("HTTP_METHOD", "EQUALS", "POST"))},
			"Prices 1 and 3 both count the same requests of the meter llm-input; give them different filters."},
		{"apart on one of two fields", []SubscriptionPlanPrice{
			usagePrice("api-calls", filter("API_TYPE", "EQUALS", "a"), filter("HTTP_METHOD", "EQUALS", "GET")),
			usagePrice("api-calls", filter("HTTP_METHOD", "EQUALS", "POST"), filter("API_TYPE", "EQUALS", "a"))}, ""},
		{"different meters are never compared", []SubscriptionPlanPrice{usagePrice("api-calls"), usagePrice("llm-input")}, ""},
		// The contract's example: calls of REST APIs and of LLM providers priced apart on one meter.
		{"apart on API type", []SubscriptionPlanPrice{
			usagePrice("api-calls", filter("API_TYPE", "EQUALS", "RestApi")),
			usagePrice("api-calls", filter("API_TYPE", "EQUALS", "LlmProvider"))}, ""},
		{"API types are matched as written", []SubscriptionPlanPrice{
			usagePrice("api-calls", filter("API_TYPE", "EQUALS", "RestApi")),
			usagePrice("api-calls", filter("API_TYPE", "EQUALS", "restapi"))}, ""},
		{"one API type and every other", []SubscriptionPlanPrice{
			usagePrice("api-calls", filter("API_TYPE", "EQUALS", "LlmProvider")),
			usagePrice("api-calls", filter("API_TYPE", "NOT_EQUALS", "LlmProvider"))}, ""},
		{"API types shared", []SubscriptionPlanPrice{
			usagePrice("api-calls", filter("API_TYPE", "IN", "RestApi", "Mcp")),
			usagePrice("api-calls", filter("API_TYPE", "EQUALS", "Mcp"))},
			"Prices 1 and 2 both count the same requests of the meter api-calls; give them different filters."},
		{"API type on one price only", []SubscriptionPlanPrice{
			usagePrice("api-calls", filter("API_TYPE", "EQUALS", "RestApi")), usagePrice("api-calls")},
			"Prices 1 and 2 both count the same requests of the meter api-calls; give them different filters."},
		{"apart on method, same API type", []SubscriptionPlanPrice{
			usagePrice("llm-input", filter("API_TYPE", "EQUALS", "LlmProvider"), filter("HTTP_METHOD", "EQUALS", "POST")),
			usagePrice("llm-input", filter("HTTP_METHOD", "EQUALS", "GET"), filter("API_TYPE", "EQUALS", "LlmProvider"))}, ""},
		{"the contract's example", []SubscriptionPlanPrice{
			usagePrice("llm-input", filter("API_TYPE", "EQUALS", "LlmProvider")),
			usagePrice("llm-input", filter("API_TYPE", "IN", "Mcp", "LlmProxy")),
			usagePrice("api-calls")}, ""},
	}
	for _, c := range cases {
		err := ValidatePlanPricing(usageBased(c.prices...))
		if c.message == "" && err != nil {
			t.Errorf("%s: the prices count different requests, got %v", c.name, err)
		}
		if c.message != "" && (err == nil || err.Error() != c.message) {
			t.Errorf("%s: got %v, want %q", c.name, err, c.message)
		}
	}
}

// Whether two filters on one field can match the same value, for every pair of operators.
func TestFilterValuesOverlap_EveryOperatorPair(t *testing.T) {
	f := func(operator string, values ...string) PriceFilter {
		return PriceFilter{Operator: operator, Values: values}
	}
	cases := []struct {
		name    string
		field   string
		a, b    PriceFilter
		overlap bool
	}{
		// EQUALS/IN with EQUALS/IN: the lists share a value.
		{"equals, equals: same", "API_TYPE", f("EQUALS", "a"), f("EQUALS", "a"), true},
		{"equals, equals: different", "API_TYPE", f("EQUALS", "a"), f("EQUALS", "b"), false},
		{"equals, is one of: in the list", "API_TYPE", f("EQUALS", "a"), f("IN", "b", "a"), true},
		{"equals, is one of: not in the list", "API_TYPE", f("EQUALS", "a"), f("IN", "b", "c"), false},
		{"is one of, is one of: shared", "API_TYPE", f("IN", "a", "b"), f("IN", "b", "c"), true},
		{"is one of, is one of: disjoint", "API_TYPE", f("IN", "a", "b"), f("IN", "c"), false},
		// EQUALS/IN with NOT_EQUALS/NOT_IN: a value of the first is not in the second.
		{"equals, not equals: other value", "API_TYPE", f("EQUALS", "a"), f("NOT_EQUALS", "b"), true},
		{"equals, not equals: same value", "API_TYPE", f("EQUALS", "a"), f("NOT_EQUALS", "a"), false},
		{"equals, is not one of: not excluded", "API_TYPE", f("EQUALS", "a"), f("NOT_IN", "b", "c"), true},
		{"equals, is not one of: excluded", "API_TYPE", f("EQUALS", "a"), f("NOT_IN", "b", "a"), false},
		{"is one of, not equals: one left", "API_TYPE", f("IN", "a", "b"), f("NOT_EQUALS", "a"), true},
		{"is one of, not equals: none left", "API_TYPE", f("IN", "a"), f("NOT_EQUALS", "a"), false},
		{"is one of, is not one of: one left", "HTTP_METHOD", f("IN", "GET", "POST"), f("NOT_IN", "GET"), true},
		{"is one of, is not one of: all excluded", "HTTP_METHOD", f("IN", "GET", "POST"), f("NOT_IN", "POST", "GET", "PUT"), false},
		// EQUALS/IN with STARTS_WITH: a value of the list is the path or under it, by whole segments.
		{"equals, starts with: under the prefix", "RESOURCE", f("EQUALS", "/orders/{id}"), f("STARTS_WITH", "/orders"), true},
		{"equals, starts with: elsewhere", "RESOURCE", f("EQUALS", "/customers"), f("STARTS_WITH", "/orders"), false},
		{"is one of, starts with: one under the prefix", "RESOURCE", f("IN", "/customers", "/orders/1"), f("STARTS_WITH", "/orders"), true},
		{"is one of, starts with: none under the prefix", "RESOURCE", f("IN", "/customers", "/items"), f("STARTS_WITH", "/orders"), false},
		{"equals, starts with: same text, another segment", "RESOURCE", f("EQUALS", "/orders-archive"), f("STARTS_WITH", "/orders"), false},
		{"equals, starts with: the path itself", "RESOURCE", f("EQUALS", "/orders"), f("STARTS_WITH", "/orders/"), true},
		{"equals, starts with: the root", "RESOURCE", f("EQUALS", "/customers"), f("STARTS_WITH", "/"), true},
		// STARTS_WITH with STARTS_WITH: one path is the other or under it.
		{"starts with, starts with: nested", "RESOURCE", f("STARTS_WITH", "/orders"), f("STARTS_WITH", "/orders/items"), true},
		{"starts with, starts with: shorter word", "RESOURCE", f("STARTS_WITH", "/order"), f("STARTS_WITH", "/orders"), false},
		{"starts with, starts with: separate", "RESOURCE", f("STARTS_WITH", "/orders"), f("STARTS_WITH", "/customers"), false},
		// NOT_EQUALS/NOT_IN with NOT_EQUALS/NOT_IN or STARTS_WITH: always, but for HTTP methods.
		{"not equals, not equals", "API_TYPE", f("NOT_EQUALS", "a"), f("NOT_EQUALS", "b"), true},
		{"not equals, is not one of", "HTTP_STATUS_CODE", f("NOT_EQUALS", "500"), f("NOT_IN", "404", "503"), true},
		{"is not one of, is not one of", "API_TYPE", f("NOT_IN", "a", "b"), f("NOT_IN", "c"), true},
		{"not equals, starts with", "RESOURCE", f("NOT_EQUALS", "/orders"), f("STARTS_WITH", "/orders"), true},
		{"is not one of, starts with", "RESOURCE", f("NOT_IN", "/orders", "/orders/1"), f("STARTS_WITH", "/orders"), true},
		{"methods not equal: five remain", "HTTP_METHOD", f("NOT_EQUALS", "GET"), f("NOT_EQUALS", "POST"), true},
		{"methods not one of: OPTIONS remains", "HTTP_METHOD", f("NOT_IN", "GET", "POST", "PUT"), f("NOT_IN", "PATCH", "DELETE", "HEAD"), true},
		{"methods not one of: all seven excluded", "HTTP_METHOD", f("NOT_IN", "GET", "POST", "PUT"), f("NOT_IN", "PATCH", "DELETE", "HEAD", "OPTIONS"), false},
		{"method not equal, methods not one of: all seven excluded", "HTTP_METHOD", f("NOT_EQUALS", "GET"),
			f("NOT_IN", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"), false},
	}
	for _, c := range cases {
		if got := filterValuesOverlap(c.field, c.a, c.b); got != c.overlap {
			t.Errorf("%s: overlap = %v, want %v", c.name, got, c.overlap)
		}
		if got := filterValuesOverlap(c.field, c.b, c.a); got != c.overlap {
			t.Errorf("%s (reversed): overlap = %v, want %v", c.name, got, c.overlap)
		}
	}
}

func TestNormalizeFilterValue_StatusCodes(t *testing.T) {
	for _, value := range []string{"200", "404", "599", " 100 "} {
		if _, err := normalizeFilterValue(FilterFieldHTTPStatusCode, value); err != nil {
			t.Errorf("%q: %v", value, err)
		}
	}
	for _, value := range []string{"2xx", "600", "99", "20"} {
		if _, err := normalizeFilterValue(FilterFieldHTTPStatusCode, value); err == nil {
			t.Errorf("%q: expected an error", value)
		}
	}
}

// The Platform API offers exactly the currencies both portals offer, so a plan it accepts is never
// refused when it is published.
func TestOfferedCurrencies_MatchThePortals(t *testing.T) {
	code := regexp.MustCompile(`code: '([A-Z]{3})'`)
	for _, file := range []string{
		"../../../portals/api-portal/src/utils/currencies.js",
		"../../../portals/api-control-plane/src/pages/appShell/appShellPages/settings/components/currencies.ts",
	} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		listed := map[string]bool{}
		for _, m := range code.FindAllStringSubmatch(string(source), -1) {
			listed[m[1]] = true
		}
		if !reflect.DeepEqual(listed, offeredCurrencies) {
			t.Errorf("%s lists %d currencies and the Platform API offers %d; they must be the same", file, len(listed), len(offeredCurrencies))
		}
	}
}

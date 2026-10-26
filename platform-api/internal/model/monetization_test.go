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

const (
	promptKey = "metadata.aiTokenUsage.promptTokens"
	totalKey  = "metadata.aiTokenUsage.totalTokens"
)

func TestValidateMeter_FilterFields(t *testing.T) {
	str := func(s string) *string { return &s }
	ten := make([]MeterFilterField, 0, 11)
	for i := 0; i < 11; i++ {
		ten = append(ten, MeterFilterField{Field: fmt.Sprintf("Property %d", i), EventKey: fmt.Sprintf("custom.k%d", i)})
	}
	valid := []*Meter{
		{Type: MeterTypeRequestCount, FilterFields: []MeterFilterField{
			{Field: "Status code", EventKey: "response.status"},
			{Field: "HTTP method", EventKey: "request.verb"},
			{Field: "Region", EventKey: "metadata.region"},
		}},
		// A property may read the number the meter adds up, so prices can compare it.
		{Type: MeterTypeTokens, TypeKey: str(promptKey), FilterFields: []MeterFilterField{
			{Field: "Prompt tokens", EventKey: promptKey}, {Field: "Total tokens", EventKey: totalKey}}},
		{Type: MeterTypeRequestCount, FilterFields: ten[:10]},
		{Type: MeterTypeRequestCount, FilterFields: []MeterFilterField{{Field: strings.Repeat("é", 40), EventKey: "k"}}},
	}
	for _, m := range valid {
		if err := ValidateMeter(m); err != nil {
			t.Errorf("%+v: %v", m.FilterFields, err)
		}
	}
	if p, ok := valid[0].Property("region"); !ok || p.Field != "Region" || p.EventKey != "metadata.region" {
		t.Errorf("Property must find a property by name, ignoring case: got %+v, %v", p, ok)
	}
	if _, ok := valid[0].Property("Country"); ok {
		t.Errorf("Property must not find a name the meter does not have")
	}

	cases := map[string]struct {
		fields  []MeterFilterField
		message string
	}{
		"more than 10":        {ten, "A meter has at most 10 filter properties"},
		"no name":             {[]MeterFilterField{{EventKey: "k"}}, "A filter property's name must be 1 to 40 characters"},
		"a long name":         {[]MeterFilterField{{Field: strings.Repeat("n", 41), EventKey: "k"}}, "A filter property's name must be 1 to 40 characters"},
		"a control character": {[]MeterFilterField{{Field: "Region\n", EventKey: "k"}}, "A filter property's name must be 1 to 40 characters"},
		"a name twice, in another case": {[]MeterFilterField{{Field: "Region", EventKey: "a"}, {Field: "region", EventKey: "b"}},
			"The meter has the property region more than once"},
		"no event key":           {[]MeterFilterField{{Field: "Region"}}, "The event key of Region must be 1 to 255 characters with no spaces"},
		"a long event key":       {[]MeterFilterField{{Field: "Region", EventKey: strings.Repeat("k", 256)}}, "The event key of Region must be 1 to 255 characters with no spaces"},
		"event key with a space": {[]MeterFilterField{{Field: "Region", EventKey: "metadata region"}}, "The event key of Region must be 1 to 255 characters with no spaces"},
		"an event key twice": {[]MeterFilterField{{Field: "Region", EventKey: "metadata.region"}, {Field: "Country", EventKey: "metadata.region"}},
			"Country and Region read the same event key metadata.region"},
	}
	for name, c := range cases {
		if err := ValidateMeter(&Meter{Type: MeterTypeRequestCount, FilterFields: c.fields}); err == nil || err.Error() != c.message {
			t.Errorf("%s: got %v, want %q", name, err, c.message)
		}
	}
}

// A meter's properties are what the request names them, trimmed, in order; there are no default keys.
func TestParseMeterRequest_FilterFields(t *testing.T) {
	m, err := ParseMeterRequest([]byte(`{"displayName":"API calls","type":"REQUEST_COUNT","filterFields":[
		{"field":" Status code ","eventKey":" response.status "},{"field":"region","eventKey":"metadata.Region"}]}`), true)
	if err != nil {
		t.Fatalf("ParseMeterRequest: %v", err)
	}
	want := []MeterFilterField{{Field: "Status code", EventKey: "response.status"}, {Field: "region", EventKey: "metadata.Region"}}
	if !reflect.DeepEqual(m.FilterFields, want) {
		t.Errorf("filter fields: got %v, want %v", m.FilterFields, want)
	}

	cases := map[string]struct {
		fields  string
		message string
	}{
		"no event key":        {`[{"field":"Region"}]`, "eventKey of filterField 1 is required"},
		"a null event key":    {`[{"field":"Region","eventKey":null}]`, "eventKey of filterField 1 must be a string"},
		"an empty event key":  {`[{"field":"Region","eventKey":" "}]`, "The event key of Region must be 1 to 255 characters with no spaces"},
		"no name":             {`[{"eventKey":"k"}]`, "field of filterField 1 is required"},
		"a name not a string": {`[{"field":7,"eventKey":"k"}]`, "field of filterField 1 must be a string"},
		"a blank name":        {`[{"field":"  ","eventKey":"k"}]`, "A filter property's name must be 1 to 40 characters"},
		"an unknown field":    {`[{"field":"Region","eventKey":"k","colour":"red"}]`, "colour of filterField 1 is not a known field"},
		"a name twice": {`[{"field":"Region","eventKey":"a"},{"field":"REGION","eventKey":"b"}]`,
			"The meter has the property REGION more than once"},
	}
	for name, c := range cases {
		body := `{"displayName":"API calls","type":"REQUEST_COUNT","filterFields":` + c.fields + `}`
		if _, err := ParseMeterRequest([]byte(body), true); err == nil || err.Error() != c.message {
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
			usagePrice("llm-input", filter("API type", "EQUALS", "LlmProvider")),
			usagePrice("llm-input", filter("API type", "IN", "Mcp", "LlmProxy")),
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

func TestValidatePlanPricing_NormalizesFilters(t *testing.T) {
	pricing := usageBased(usagePrice("llm-input",
		filter(" API type ", "IN", " LlmProvider ", "LlmProxy", "LlmProvider", "llmprovider"),
		filter("HTTP method", "NOT_IN", "get", "Post"),
		filter("Resource", "STARTS_WITH", "/orders/"),
		filter("Prompt tokens", "GREATER_THAN", " 0100 "),
		filter("Prompt tokens", "LESS_THAN_OR_EQUAL", "500")))
	if err := ValidatePlanPricing(pricing); err != nil {
		t.Fatalf("ValidatePlanPricing: %v", err)
	}
	// Filters in the order given; names and values trimmed, text kept as written with repeated values
	// dropped, a starts-with value unchanged, a number without leading zeros.
	want := []PriceFilter{
		filter("API type", "IN", "LlmProvider", "LlmProxy", "llmprovider"),
		filter("HTTP method", "NOT_IN", "get", "Post"),
		filter("Resource", "STARTS_WITH", "/orders/"),
		filter("Prompt tokens", "GREATER_THAN", "100"),
		filter("Prompt tokens", "LESS_THAN_OR_EQUAL", "500"),
	}
	if got := pricing.Prices[0].Filters; !reflect.DeepEqual(got, want) {
		t.Errorf("filters = %+v, want %+v", got, want)
	}
}

// Any operator works on any property, and no value is checked against what the property holds.
func TestValidatePlanPricing_AnyOperatorOnAnyProperty(t *testing.T) {
	values := map[string][]string{
		"EQUALS": {"5xx"}, "NOT_EQUALS": {"FETCH"}, "IN": {"200", "orders"}, "NOT_IN": {"tools/call"},
		"STARTS_WITH": {"Llm"}, "GREATER_THAN": {"499"}, "GREATER_THAN_OR_EQUAL": {"0"}, "LESS_THAN": {"600"},
		"LESS_THAN_OR_EQUAL": {"1000000000000"},
	}
	for _, property := range []string{"Status code", "HTTP method", "Resource", "Tool", "Prompt tokens"} {
		for operator, v := range values {
			if err := ValidatePlanPricing(usageBased(usagePrice("api-calls", filter(property, operator, v...)))); err != nil {
				t.Errorf("%s %s %v: %v", property, operator, v, err)
			}
		}
	}
	// A price may filter on a tool and on the MCP method together.
	if err := ValidatePlanPricing(usageBased(usagePrice("mcp",
		filter("MCP method", "EQUALS", "resources/read"), filter("Tool", "EQUALS", "search")))); err != nil {
		t.Errorf("tool and MCP method: %v", err)
	}
}

func TestValidatePlanPricing_RefusesBadFilters(t *testing.T) {
	many := make([]string, 21)
	for i := range many {
		many[i] = fmt.Sprintf("value-%d", i)
	}
	twentyOne := make([]PriceFilter, 0, 21)
	for i := 0; i < 21; i++ {
		twentyOne = append(twentyOne, filter(fmt.Sprintf("Property %d", i), "EQUALS", "a"))
	}
	const badNumber = "price 2: A comparison on Prompt tokens takes a whole number from 0 to 1000000000000"
	cases := map[string]struct {
		filters []PriceFilter
		message string
	}{
		"no name":     {[]PriceFilter{filter(" ", "EQUALS", "a")}, "field of filter 1 of price 2 must be 1 to 40 characters"},
		"a long name": {[]PriceFilter{filter(strings.Repeat("n", 41), "EQUALS", "a")}, "field of filter 1 of price 2 must be 1 to 40 characters"},
		"an unknown operator": {[]PriceFilter{filter("Region", "EQUALS", "a"), filter("Country", "MATCHES", "a")},
			"The operator of filter 2 of price 2 must be one of EQUALS, NOT_EQUALS, IN, NOT_IN, STARTS_WITH, GREATER_THAN, GREATER_THAN_OR_EQUAL, LESS_THAN, LESS_THAN_OR_EQUAL"},
		"two values for not equals":   {[]PriceFilter{filter("Region", "NOT_EQUALS", "a", "b")}, "price 2: Operator NOT_EQUALS takes exactly one value"},
		"two prefixes":                {[]PriceFilter{filter("Resource", "STARTS_WITH", "/a", "/b")}, "price 2: Operator STARTS_WITH takes exactly one value"},
		"two numbers for more than":   {[]PriceFilter{filter("Prompt tokens", "GREATER_THAN", "1", "2")}, "price 2: Operator GREATER_THAN takes exactly one value"},
		"no number for at most":       {[]PriceFilter{filter("Prompt tokens", "LESS_THAN_OR_EQUAL")}, "price 2: Operator LESS_THAN_OR_EQUAL takes exactly one value"},
		"no values for is one of":     {[]PriceFilter{filter("Region", "IN")}, "price 2: A filter on Region needs 1 to 20 values"},
		"21 values for is not one of": {[]PriceFilter{filter("Region", "NOT_IN", many...)}, "price 2: A filter on Region needs 1 to 20 values"},
		"a blank value":               {[]PriceFilter{filter("Region", "EQUALS", " ")}, "price 2: Filter values for Region must be 1 to 255 characters"},
		"a long value":                {[]PriceFilter{filter("Region", "IN", "a", strings.Repeat("v", 256))}, "price 2: Filter values for Region must be 1 to 255 characters"},
		"a negative number":           {[]PriceFilter{filter("Prompt tokens", "GREATER_THAN", "-1")}, badNumber},
		"a fraction":                  {[]PriceFilter{filter("Prompt tokens", "GREATER_THAN", "1.5")}, badNumber},
		"text":                        {[]PriceFilter{filter("Prompt tokens", "LESS_THAN", "many")}, badNumber},
		"a number too large":          {[]PriceFilter{filter("Prompt tokens", "GREATER_THAN_OR_EQUAL", "1000000000001")}, badNumber},
		"a blank number":              {[]PriceFilter{filter("Prompt tokens", "GREATER_THAN", " ")}, badNumber},
		"21 filters":                  {twentyOne, "price 2: a price has at most 20 filters"},
		"two matches on a property, in another case": {[]PriceFilter{filter("Region", "IN", "a"), filter("region", "NOT_IN", "b")},
			"price 2 filters on Region more than once"},
		"a match and a comparison": {[]PriceFilter{filter("Status code", "GREATER_THAN", "499"), filter("Status code", "NOT_EQUALS", "503")},
			"price 2 filters on Status code more than once"},
		"three comparisons": {[]PriceFilter{filter("Prompt tokens", "GREATER_THAN", "1"), filter("Prompt tokens", "LESS_THAN", "9"),
			filter("prompt tokens", "LESS_THAN", "8")}, "price 2: a price puts at most 2 comparisons on Prompt tokens"},
		"an empty range": {[]PriceFilter{filter("Prompt tokens", "GREATER_THAN", "500"), filter("Prompt tokens", "LESS_THAN", "100")},
			"price 2: the comparisons on Prompt tokens can't all be true"},
		"more than and at most the same": {[]PriceFilter{filter("Prompt tokens", "GREATER_THAN", "100"), filter("PROMPT TOKENS", "LESS_THAN_OR_EQUAL", "100")},
			"price 2: the comparisons on Prompt tokens can't all be true"},
		"less than nothing": {[]PriceFilter{filter("Prompt tokens", "LESS_THAN", "0")}, "price 2: the comparisons on Prompt tokens can't all be true"},
	}
	for name, c := range cases {
		pricing := usageBased(usagePrice("api-calls"), usagePrice("llm-input", c.filters...))
		if err := ValidatePlanPricing(pricing); err == nil || err.Error() != c.message {
			t.Errorf("%s: got %v, want %q", name, err, c.message)
		}
	}
	// At least and at most the same number is that number.
	if err := ValidatePlanPricing(usageBased(usagePrice("llm-input",
		filter("Prompt tokens", "GREATER_THAN_OR_EQUAL", "100"), filter("Prompt tokens", "LESS_THAN_OR_EQUAL", "100")))); err != nil {
		t.Errorf("a one-number range: %v", err)
	}
}

// Two prices on one meter overlap when a request could match both: on every property both filter on.
// A property only one price filters on matches any value for the other.
func TestValidatePlanPricing_OverlapAcrossProperties(t *testing.T) {
	cases := []struct {
		name    string
		prices  []SubscriptionPlanPrice
		message string
	}{
		{"no filters on either", []SubscriptionPlanPrice{usagePrice("api-calls"), usagePrice("api-calls")},
			"Prices 1 and 2 both count the same requests of the meter api-calls; give them different filters."},
		{"a price without filters counts what the other counts", []SubscriptionPlanPrice{
			usagePrice("llm-input", filter("API type", "EQUALS", "LlmProvider")), usagePrice("llm-input")},
			"Prices 1 and 2 both count the same requests of the meter llm-input; give them different filters."},
		{"filters on different properties", []SubscriptionPlanPrice{
			usagePrice("llm-input", filter("API type", "EQUALS", "LlmProvider")),
			usagePrice("api-calls"),
			usagePrice("llm-input", filter("HTTP method", "EQUALS", "POST"))},
			"Prices 1 and 3 both count the same requests of the meter llm-input; give them different filters."},
		{"apart on one of two properties", []SubscriptionPlanPrice{
			usagePrice("api-calls", filter("API type", "EQUALS", "a"), filter("HTTP method", "EQUALS", "GET")),
			usagePrice("api-calls", filter("HTTP method", "EQUALS", "POST"), filter("API type", "EQUALS", "a"))}, ""},
		{"different meters are never compared", []SubscriptionPlanPrice{usagePrice("api-calls"), usagePrice("llm-input")}, ""},
		{"names are matched ignoring case", []SubscriptionPlanPrice{
			usagePrice("api-calls", filter("region", "EQUALS", "eu")),
			usagePrice("api-calls", filter("Region", "EQUALS", "eu"))},
			"Prices 1 and 2 both count the same requests of the meter api-calls; give them different filters."},
		{"apart on a property named in two cases", []SubscriptionPlanPrice{
			usagePrice("api-calls", filter("region", "EQUALS", "eu")),
			usagePrice("api-calls", filter("Region", "EQUALS", "us"))}, ""},
		{"values are matched as written", []SubscriptionPlanPrice{
			usagePrice("api-calls", filter("API type", "EQUALS", "RestApi")),
			usagePrice("api-calls", filter("API type", "EQUALS", "restapi"))}, ""},
		{"one value and every other", []SubscriptionPlanPrice{
			usagePrice("api-calls", filter("API type", "EQUALS", "LlmProvider")),
			usagePrice("api-calls", filter("API type", "NOT_EQUALS", "LlmProvider"))}, ""},
		{"values shared", []SubscriptionPlanPrice{
			usagePrice("api-calls", filter("API type", "IN", "RestApi", "Mcp")),
			usagePrice("api-calls", filter("API type", "EQUALS", "Mcp"))},
			"Prices 1 and 2 both count the same requests of the meter api-calls; give them different filters."},
		{"the contract's example", []SubscriptionPlanPrice{
			usagePrice("llm-input", filter("API type", "EQUALS", "LlmProvider")),
			usagePrice("llm-input", filter("API type", "IN", "Mcp", "LlmProxy")),
			usagePrice("api-calls")}, ""},
		{"a tool and another method", []SubscriptionPlanPrice{
			usagePrice("mcp", filter("Tool", "EQUALS", "search")),
			usagePrice("mcp", filter("MCP method", "EQUALS", "prompts/get"))},
			"Prices 1 and 2 both count the same requests of the meter mcp; give them different filters."},
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

// Whether two text filters on one property can match the same value, for every pair of operators.
func TestFilterValuesOverlap_EveryOperatorPair(t *testing.T) {
	f := func(operator string, values ...string) PriceFilter {
		return PriceFilter{Operator: operator, Values: values}
	}
	cases := []struct {
		name    string
		a, b    PriceFilter
		overlap bool
	}{
		// EQUALS/IN with EQUALS/IN: the lists share a value.
		{"equals, equals: same", f("EQUALS", "a"), f("EQUALS", "a"), true},
		{"equals, equals: different", f("EQUALS", "a"), f("EQUALS", "b"), false},
		{"equals, equals: another case", f("EQUALS", "a"), f("EQUALS", "A"), false},
		{"equals, is one of: in the list", f("EQUALS", "a"), f("IN", "b", "a"), true},
		{"equals, is one of: not in the list", f("EQUALS", "a"), f("IN", "b", "c"), false},
		{"is one of, is one of: shared", f("IN", "a", "b"), f("IN", "b", "c"), true},
		{"is one of, is one of: disjoint", f("IN", "a", "b"), f("IN", "c"), false},
		// EQUALS/IN with NOT_EQUALS/NOT_IN: a value of the first is not excluded by the second.
		{"equals, not equals: other value", f("EQUALS", "a"), f("NOT_EQUALS", "b"), true},
		{"equals, not equals: same value", f("EQUALS", "a"), f("NOT_EQUALS", "a"), false},
		{"equals, is not one of: not excluded", f("EQUALS", "a"), f("NOT_IN", "b", "c"), true},
		{"equals, is not one of: excluded", f("EQUALS", "a"), f("NOT_IN", "b", "a"), false},
		{"is one of, not equals: one left", f("IN", "a", "b"), f("NOT_EQUALS", "a"), true},
		{"is one of, not equals: none left", f("IN", "a"), f("NOT_EQUALS", "a"), false},
		{"is one of, is not one of: all excluded", f("IN", "GET", "POST"), f("NOT_IN", "POST", "GET", "PUT"), false},
		// EQUALS/IN with STARTS_WITH: a value of the list starts with the prefix.
		{"equals, starts with: under the prefix", f("EQUALS", "/orders/{id}"), f("STARTS_WITH", "/orders"), true},
		{"equals, starts with: elsewhere", f("EQUALS", "/customers"), f("STARTS_WITH", "/orders"), false},
		{"equals, starts with: same text, another word", f("EQUALS", "/orders-archive"), f("STARTS_WITH", "/orders"), true},
		{"equals, starts with: shorter than the prefix", f("EQUALS", "/orders"), f("STARTS_WITH", "/orders/"), false},
		{"is one of, starts with: one under the prefix", f("IN", "/customers", "/orders/1"), f("STARTS_WITH", "/orders"), true},
		{"is one of, starts with: none under the prefix", f("IN", "/customers", "/items"), f("STARTS_WITH", "/orders"), false},
		// STARTS_WITH with STARTS_WITH: one prefix starts with the other.
		{"starts with, starts with: nested", f("STARTS_WITH", "/orders"), f("STARTS_WITH", "/orders/items"), true},
		{"starts with, starts with: shorter word", f("STARTS_WITH", "/order"), f("STARTS_WITH", "/orders"), true},
		{"starts with, starts with: separate", f("STARTS_WITH", "/orders"), f("STARTS_WITH", "/customers"), false},
		// NOT_EQUALS/NOT_IN with NOT_EQUALS/NOT_IN or STARTS_WITH: always.
		{"not equals, not equals", f("NOT_EQUALS", "a"), f("NOT_EQUALS", "b"), true},
		{"not equals, is not one of", f("NOT_EQUALS", "500"), f("NOT_IN", "404", "503"), true},
		{"is not one of, is not one of", f("NOT_IN", "GET", "POST", "PUT"), f("NOT_IN", "PATCH", "DELETE", "HEAD", "OPTIONS"), true},
		{"not equals, starts with", f("NOT_EQUALS", "/orders"), f("STARTS_WITH", "/orders"), true},
		{"is not one of, starts with", f("NOT_IN", "/orders", "/orders/1"), f("STARTS_WITH", "/orders"), true},
	}
	for _, c := range cases {
		if got := filterValuesOverlap(c.a, c.b); got != c.overlap {
			t.Errorf("%s: overlap = %v, want %v", c.name, got, c.overlap)
		}
		if got := filterValuesOverlap(c.b, c.a); got != c.overlap {
			t.Errorf("%s (reversed): overlap = %v, want %v", c.name, got, c.overlap)
		}
	}
}

// On one property, a price's comparisons form a range of whole numbers, every number without a
// comparison. Two prices are apart on the property when their ranges share no number, or when one
// matches only values that are not whole numbers in the other's range.
func TestValidatePlanPricing_ComparisonsOverlap(t *testing.T) {
	const overlap = "Prices 1 and 2 both count the same requests of the meter llm-input; give them different filters."
	tokens := func(operator, value string) PriceFilter { return filter("Prompt tokens", operator, value) }
	cases := []struct {
		name    string
		a, b    []PriceFilter
		message string
	}{
		{"at most 333 and more than 333", []PriceFilter{tokens("LESS_THAN_OR_EQUAL", "333")},
			[]PriceFilter{tokens("GREATER_THAN", "333")}, ""},
		{"more than 100 and more than 333", []PriceFilter{tokens("GREATER_THAN", "100")},
			[]PriceFilter{tokens("GREATER_THAN", "333")}, overlap},
		{"less than 333 and at least 333", []PriceFilter{tokens("LESS_THAN", "333")},
			[]PriceFilter{tokens("GREATER_THAN_OR_EQUAL", "333")}, ""},
		{"names in another case", []PriceFilter{filter("prompt tokens", "LESS_THAN", "333")},
			[]PriceFilter{tokens("GREATER_THAN_OR_EQUAL", "333")}, ""},
		{"101 to 500 and equal to 501", []PriceFilter{tokens("GREATER_THAN", "100"), tokens("LESS_THAN_OR_EQUAL", "500")},
			[]PriceFilter{tokens("EQUALS", "501")}, ""},
		{"101 to 500 and equal to 500", []PriceFilter{tokens("GREATER_THAN", "100"), tokens("LESS_THAN_OR_EQUAL", "500")},
			[]PriceFilter{tokens("EQUALS", "500")}, overlap},
		{"101 to 500 and one of 50 or text", []PriceFilter{tokens("GREATER_THAN", "100"), tokens("LESS_THAN_OR_EQUAL", "500")},
			[]PriceFilter{filter("Prompt tokens", "IN", "50", "many", "200.5")}, ""},
		{"more than 100 and one of 0150", []PriceFilter{tokens("GREATER_THAN", "100")},
			[]PriceFilter{filter("Prompt tokens", "IN", "50", "0150")}, overlap},
		{"more than 100 and a number too large to read", []PriceFilter{tokens("GREATER_THAN", "100")},
			[]PriceFilter{tokens("EQUALS", "99999999999999999999999")}, overlap},
		{"at most 500 and a number too large to read", []PriceFilter{tokens("LESS_THAN_OR_EQUAL", "500")},
			[]PriceFilter{tokens("EQUALS", "99999999999999999999999")}, ""},
		{"a range and not equal", []PriceFilter{tokens("GREATER_THAN", "100")},
			[]PriceFilter{tokens("NOT_EQUALS", "200")}, overlap},
		{"a range and starts with", []PriceFilter{tokens("LESS_THAN", "100")},
			[]PriceFilter{tokens("STARTS_WITH", "9")}, overlap},
		{"a price without a comparison covers every number", []PriceFilter{tokens("GREATER_THAN", "333")}, nil, overlap},
		{"comparisons on different properties", []PriceFilter{tokens("GREATER_THAN", "333")},
			[]PriceFilter{filter("Completion tokens", "LESS_THAN_OR_EQUAL", "10")}, overlap},
		{"apart on one of two properties", []PriceFilter{tokens("GREATER_THAN", "333"), filter("Total tokens", "GREATER_THAN", "0")},
			[]PriceFilter{filter("Total tokens", "GREATER_THAN", "0"), tokens("LESS_THAN_OR_EQUAL", "333")}, ""},
		{"same API type, apart on tokens", []PriceFilter{filter("API type", "EQUALS", "LlmProvider"), tokens("GREATER_THAN", "333")},
			[]PriceFilter{filter("API type", "EQUALS", "LlmProvider"), tokens("LESS_THAN_OR_EQUAL", "333")}, ""},
		{"apart on API type, same tokens", []PriceFilter{filter("API type", "EQUALS", "LlmProvider"), tokens("GREATER_THAN", "333")},
			[]PriceFilter{filter("API type", "EQUALS", "LlmProxy"), tokens("GREATER_THAN", "100")}, ""},
		{"same API type, tokens overlapping", []PriceFilter{filter("API type", "EQUALS", "LlmProvider"), tokens("GREATER_THAN", "333")},
			[]PriceFilter{filter("API type", "IN", "LlmProvider", "LlmProxy"), tokens("GREATER_THAN", "100")}, overlap},
	}
	for _, c := range cases {
		err := ValidatePlanPricing(usageBased(usagePrice("llm-input", c.a...), usagePrice("llm-input", c.b...)))
		if c.message == "" && err != nil {
			t.Errorf("%s: the prices count different requests, got %v", c.name, err)
		}
		if c.message != "" && (err == nil || err.Error() != c.message) {
			t.Errorf("%s: got %v, want %q", c.name, err, c.message)
		}
		err = ValidatePlanPricing(usageBased(usagePrice("llm-input", c.b...), usagePrice("llm-input", c.a...)))
		if (err == nil) != (c.message == "") {
			t.Errorf("%s (reversed): got %v", c.name, err)
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

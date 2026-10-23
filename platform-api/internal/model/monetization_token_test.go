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
	"reflect"
	"testing"
)

const (
	promptKey     = "metadata.aiTokenUsage.promptTokens"
	completionKey = "metadata.aiTokenUsage.completionTokens"
	totalKey      = "metadata.aiTokenUsage.totalTokens"
)

// token is a Token condition on the key.
func token(key, operator, value string) PriceFilter {
	return PriceFilter{Field: FilterFieldToken, EventKey: key, Operator: operator, Values: []string{value}}
}

func tokenField(key string) MeterFilterField {
	return MeterFilterField{Field: FilterFieldToken, EventKey: key}
}

func TestValidateMeter_TokenFields(t *testing.T) {
	row := promptKey
	valid := []*Meter{
		// Token fields on any meter, several, each with its own key.
		{Type: MeterTypeRequestCount, FilterFields: []MeterFilterField{tokenField(totalKey), tokenField(promptKey)}},
		{Type: MeterTypeTokens, TypeKey: &row, FilterFields: []MeterFilterField{
			{Field: FilterFieldAPIType, EventKey: "metadata.apiType"}, tokenField(completionKey), tokenField("custom.cached")}},
	}
	for _, m := range valid {
		if err := ValidateMeter(m); err != nil {
			t.Errorf("%+v: %v", m.FilterFields, err)
		}
	}
	if m := valid[1]; !m.HasTokenKey(promptKey) || !m.HasTokenKey(completionKey) || m.HasTokenKey(totalKey) {
		t.Errorf("HasTokenKey must follow the typeKey and the TOKEN fields")
	}

	eleven := make([]MeterFilterField, 0, 11)
	for i := 0; i < 11; i++ {
		eleven = append(eleven, tokenField(fmt.Sprintf("tokens.k%d", i)))
	}
	cases := map[string]struct {
		meter   *Meter
		message string
	}{
		"no key": {&Meter{Type: MeterTypeRequestCount, FilterFields: []MeterFilterField{{Field: FilterFieldToken}}},
			"A Token field needs an eventKey"},
		"a key with a space": {&Meter{Type: MeterTypeRequestCount, FilterFields: []MeterFilterField{tokenField("prompt tokens")}},
			"The event key of Token must be 1 to 255 characters with no spaces"},
		"the same key twice": {&Meter{Type: MeterTypeRequestCount, FilterFields: []MeterFilterField{tokenField(totalKey), tokenField(totalKey)}},
			"The meter lists the Token key " + totalKey + " more than once"},
		"the typeKey's key": {&Meter{Type: MeterTypeTokens, TypeKey: &row, FilterFields: []MeterFilterField{tokenField(promptKey)}},
			"The key " + promptKey + " is already the meter's Token key"},
		"more than 10 fields": {&Meter{Type: MeterTypeRequestCount, FilterFields: eleven}, "A meter has at most 10 filter fields"},
	}
	for name, c := range cases {
		if err := ValidateMeter(c.meter); err == nil || err.Error() != c.message {
			t.Errorf("%s: got %v, want %q", name, err, c.message)
		}
	}
}

func TestValidatePlanPricing_TokenConditions(t *testing.T) {
	pricing := usageBased(usagePrice("llm-input",
		filter("API_TYPE", "EQUALS", "LlmProvider"),
		token(" "+promptKey+" ", "GREATER_THAN", " 0100 "),
		token(promptKey, "LESS_THAN_OR_EQUAL", "500"),
		token(totalKey, "GREATER_THAN_OR_EQUAL", "1000000000000")))
	if err := ValidatePlanPricing(pricing); err != nil {
		t.Fatalf("ValidatePlanPricing: %v", err)
	}
	// The key trimmed and each value written as a plain whole number.
	want := []PriceFilter{filter("API_TYPE", "EQUALS", "LlmProvider"), token(promptKey, "GREATER_THAN", "100"),
		token(promptKey, "LESS_THAN_OR_EQUAL", "500"), token(totalKey, "GREATER_THAN_OR_EQUAL", "1000000000000")}
	if got := pricing.Prices[0].Filters; !reflect.DeepEqual(got, want) {
		t.Errorf("filters = %+v, want %+v", got, want)
	}
	for _, operator := range []string{"GREATER_THAN", "GREATER_THAN_OR_EQUAL", "LESS_THAN", "LESS_THAN_OR_EQUAL", "EQUALS"} {
		if err := ValidatePlanPricing(usageBased(usagePrice("llm-input", token(promptKey, operator, "333")))); err != nil {
			t.Errorf("%s: %v", operator, err)
		}
	}

	twentyOne := make([]PriceFilter, 0, 21)
	for i := 0; i < 21; i++ {
		twentyOne = append(twentyOne, token(fmt.Sprintf("tokens.k%d", i), "GREATER_THAN", "0"))
	}
	const badValue = "price 1: the Token value must be a whole number from 0 to 1000000000000"
	cases := map[string]struct {
		filters []PriceFilter
		message string
	}{
		"no key":                {[]PriceFilter{{Field: "TOKEN", Operator: "GREATER_THAN", Values: []string{"1"}}}, "price 1: a Token filter needs an eventKey"},
		"a key on API type":     {[]PriceFilter{{Field: "API_TYPE", EventKey: promptKey, Operator: "EQUALS", Values: []string{"LlmProvider"}}}, "price 1: only a Token filter takes an eventKey"},
		"is one of":             {[]PriceFilter{token(promptKey, "IN", "1")}, "The TOKEN operator of price 1 must be one of GREATER_THAN, GREATER_THAN_OR_EQUAL, LESS_THAN, LESS_THAN_OR_EQUAL, EQUALS"},
		"not equal":             {[]PriceFilter{token(promptKey, "NOT_EQUALS", "1")}, "The TOKEN operator of price 1 must be one of GREATER_THAN, GREATER_THAN_OR_EQUAL, LESS_THAN, LESS_THAN_OR_EQUAL, EQUALS"},
		"more than on API type": {[]PriceFilter{filter("API_TYPE", "GREATER_THAN", "1")}, "The API_TYPE operator of price 1 must be one of EQUALS, NOT_EQUALS, IN, NOT_IN"},
		"at most on status":     {[]PriceFilter{filter("HTTP_STATUS_CODE", "LESS_THAN_OR_EQUAL", "499")}, "The HTTP_STATUS_CODE operator of price 1 must be one of EQUALS, NOT_EQUALS, IN, NOT_IN"},
		"negative":              {[]PriceFilter{token(promptKey, "GREATER_THAN", "-1")}, badValue},
		"fraction":              {[]PriceFilter{token(promptKey, "GREATER_THAN", "1.5")}, badValue},
		"text":                  {[]PriceFilter{token(promptKey, "GREATER_THAN", "many")}, badValue},
		"too large":             {[]PriceFilter{token(promptKey, "GREATER_THAN", "1000000000001")}, badValue},
		"blank":                 {[]PriceFilter{token(promptKey, "GREATER_THAN", " ")}, badValue},
		"two values":            {[]PriceFilter{{Field: "TOKEN", EventKey: promptKey, Operator: "EQUALS", Values: []string{"1", "2"}}}, badValue},
		"no value":              {[]PriceFilter{{Field: "TOKEN", EventKey: promptKey, Operator: "EQUALS"}}, badValue},
		"three conditions": {[]PriceFilter{token(promptKey, "GREATER_THAN", "1"), token(promptKey, "LESS_THAN", "9"), token(promptKey, "LESS_THAN", "8")},
			"price 1: a price puts at most 2 Token conditions on " + promptKey},
		"an empty range": {[]PriceFilter{token(promptKey, "GREATER_THAN", "500"), token(promptKey, "LESS_THAN", "100")},
			"price 1: the Token conditions on " + promptKey + " can't all be true"},
		"more than and at most the same": {[]PriceFilter{token(promptKey, "GREATER_THAN", "100"), token(promptKey, "LESS_THAN_OR_EQUAL", "100")},
			"price 1: the Token conditions on " + promptKey + " can't all be true"},
		"equal to and more than": {[]PriceFilter{token(totalKey, "EQUALS", "5"), token(totalKey, "GREATER_THAN", "5")},
			"price 1: the Token conditions on " + totalKey + " can't all be true"},
		"less than nothing": {[]PriceFilter{token(promptKey, "LESS_THAN", "0")},
			"price 1: the Token conditions on " + promptKey + " can't all be true"},
		"21 filters": {twentyOne, "price 1: a price has at most 20 filters"},
	}
	for name, c := range cases {
		if err := ValidatePlanPricing(usageBased(usagePrice("llm-input", c.filters...))); err == nil || err.Error() != c.message {
			t.Errorf("%s: got %v, want %q", name, err, c.message)
		}
	}
	// At least and at most the same number is that number.
	if err := ValidatePlanPricing(usageBased(usagePrice("llm-input",
		token(promptKey, "GREATER_THAN_OR_EQUAL", "100"), token(promptKey, "LESS_THAN_OR_EQUAL", "100")))); err != nil {
		t.Errorf("a one-number range: %v", err)
	}
}

// For each Token key, a price's conditions form a range of whole numbers, all of them without a
// condition on the key; two prices are apart on the key when their ranges share no number.
func TestValidatePlanPricing_TokenRangesOverlap(t *testing.T) {
	const overlap = "Prices 1 and 2 both count the same requests of the meter llm-input; give them different filters."
	cases := []struct {
		name    string
		a, b    []PriceFilter
		message string
	}{
		{"at most 333 and more than 333", []PriceFilter{token(promptKey, "LESS_THAN_OR_EQUAL", "333")},
			[]PriceFilter{token(promptKey, "GREATER_THAN", "333")}, ""},
		{"more than 100 and more than 333", []PriceFilter{token(promptKey, "GREATER_THAN", "100")},
			[]PriceFilter{token(promptKey, "GREATER_THAN", "333")}, overlap},
		{"less than 333 and at least 333", []PriceFilter{token(promptKey, "LESS_THAN", "333")},
			[]PriceFilter{token(promptKey, "GREATER_THAN_OR_EQUAL", "333")}, ""},
		{"101 to 500 and equal to 501", []PriceFilter{token(promptKey, "GREATER_THAN", "100"), token(promptKey, "LESS_THAN_OR_EQUAL", "500")},
			[]PriceFilter{token(promptKey, "EQUALS", "501")}, ""},
		{"101 to 500 and equal to 500", []PriceFilter{token(promptKey, "GREATER_THAN", "100"), token(promptKey, "LESS_THAN_OR_EQUAL", "500")},
			[]PriceFilter{token(promptKey, "EQUALS", "500")}, overlap},
		{"a price without a condition covers every number", []PriceFilter{token(promptKey, "GREATER_THAN", "333")}, nil, overlap},
		{"conditions on different keys", []PriceFilter{token(promptKey, "GREATER_THAN", "333")},
			[]PriceFilter{token(completionKey, "LESS_THAN_OR_EQUAL", "10")}, overlap},
		{"apart on one of two keys", []PriceFilter{token(promptKey, "GREATER_THAN", "333"), token(totalKey, "GREATER_THAN", "0")},
			[]PriceFilter{token(totalKey, "GREATER_THAN", "0"), token(promptKey, "LESS_THAN_OR_EQUAL", "333")}, ""},
		// Mixed with the API type: prices apart on any one field are apart.
		{"same API type, apart on tokens", []PriceFilter{filter("API_TYPE", "EQUALS", "LlmProvider"), token(promptKey, "GREATER_THAN", "333")},
			[]PriceFilter{filter("API_TYPE", "EQUALS", "LlmProvider"), token(promptKey, "LESS_THAN_OR_EQUAL", "333")}, ""},
		{"apart on API type, same tokens", []PriceFilter{filter("API_TYPE", "EQUALS", "LlmProvider"), token(promptKey, "GREATER_THAN", "333")},
			[]PriceFilter{filter("API_TYPE", "EQUALS", "LlmProxy"), token(promptKey, "GREATER_THAN", "100")}, ""},
		{"same API type, tokens overlapping", []PriceFilter{filter("API_TYPE", "EQUALS", "LlmProvider"), token(promptKey, "GREATER_THAN", "333")},
			[]PriceFilter{filter("API_TYPE", "IN", "LlmProvider", "LlmProxy"), token(promptKey, "GREATER_THAN", "100")}, overlap},
	}
	for _, c := range cases {
		err := ValidatePlanPricing(usageBased(usagePrice("llm-input", c.a...), usagePrice("llm-input", c.b...)))
		if c.message == "" && err != nil {
			t.Errorf("%s: the prices count different requests, got %v", c.name, err)
		}
		if c.message != "" && (err == nil || err.Error() != c.message) {
			t.Errorf("%s: got %v, want %q", c.name, err, c.message)
		}
	}
}

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

package main

import (
	"slices"
	"strings"
	"testing"
)

// testdata/meters.json holds real Retail Prices API meters (eastus2 and
// swedencentral, October 2026), so the expectations below are Azure's prices.
func loadTestPrices(t *testing.T) (prices, []string) {
	t.Helper()
	meters, err := readMeters([]string{"testdata/meters.json"})
	if err != nil {
		t.Fatalf("reading meters: %v", err)
	}
	p, skipped, err := buildPrices(meters)
	if err != nil {
		t.Fatalf("buildPrices: %v", err)
	}
	return p, skipped
}

func TestBuildPrices_RealMeters(t *testing.T) {
	p, _ := loadTestPrices(t)
	const M = 1e-6
	tests := []struct {
		key, field string
		want       float64
	}{
		{"azure/gpt-5.6-sol", fieldInput, 4 * M},
		{"azure/gpt-5.6-sol", fieldOutput, 20 * M},
		{"azure/gpt-5.6-sol", fieldCacheRead, 0.4 * M},
		{"azure/gpt-5.6-sol", fieldCacheWrite, 5 * M},
		{"azure/gpt-5.6-sol", "input_cost_per_token_priority", 8 * M},
		{"azure/gpt-5.6-sol", "input_cost_per_token_flex", 2 * M},
		{"azure/gpt-5.6-sol", "input_cost_per_token_above_272k_tokens", 8 * M},
		{"azure/us/gpt-5.6-sol", fieldInput, 4.4 * M},
		{"azure/eu/gpt-5.6-sol", fieldInput, 4.4 * M},
		{"azure_ai/Llama-3.3-70B-Instruct", fieldInput, 0.71 * M},
		{"azure_ai/us/Llama-3.3-70B-Instruct", fieldInput, 0.781 * M},
		{"azure_ai/regional/Llama-3.3-70B-Instruct", fieldInput, 0.781 * M},
		{"azure_ai/grok-4.7", fieldInput, 2 * M},
		{"azure_ai/us/grok-4.7", fieldInput, 2.2 * M},
		{"azure_ai/eu/grok-4.7", fieldInput, 2.4 * M},
		{"azure_ai/kimi-k2.5", fieldCacheRead, 0.1 * M},
		{"azure_ai/Phi-4", fieldInput, 0.125 * M},
		{"azure_ai/Phi-4-mini-instruct", fieldOutput, 0.3 * M},
	}
	for _, tc := range tests {
		got, ok := p[tc.key][tc.field]
		if !ok || got != roundRate(tc.want) {
			t.Errorf("%s %s = %v (present=%v); want %v", tc.key, tc.field, got, ok, roundRate(tc.want))
		}
	}
}

// Regional prices come from the reference region only; swedencentral's own
// regional rate (0.859) must not overwrite eastus2's.
func TestBuildPrices_RegionalFromReferenceRegionOnly(t *testing.T) {
	p, _ := loadTestPrices(t)
	if got := p["azure_ai/regional/Llama-3.3-70B-Instruct"][fieldInput]; got != 7.81e-07 {
		t.Errorf("regional Llama input = %v; want eastus2's 7.81e-07", got)
	}
}

// Grok's long-context meters start below the policy's 272k threshold, so they
// are left out; fine-tuned, batch and hourly meters never price anything.
func TestBuildPrices_IgnoresMetersItCannotApply(t *testing.T) {
	p, _ := loadTestPrices(t)
	for key, fields := range p {
		if strings.Contains(key, "grok") {
			for field := range fields {
				if strings.Contains(field, "272k") {
					t.Errorf("%s carries %s from a meter with a different threshold", key, field)
				}
			}
		}
	}
	for _, key := range []string{"azure/gpt-5", "azure/gpt-4.1"} {
		if _, ok := p[key]; ok {
			t.Errorf("%s must come only from batch or hourly meters, which are ignored", key)
		}
	}
}

// gpt-4o-2024-08-06 has only a cached-input meter in the fixture. Writing it
// would erase the entry's input and output rates, so it is skipped.
func TestBuildPrices_SkipsIncompleteEntries(t *testing.T) {
	p, skipped := loadTestPrices(t)
	if _, ok := p["azure/gpt-4o-2024-08-06"]; ok {
		t.Error("an entry without input and output rates must not be written")
	}
	if !slices.Contains(skipped, "azure/gpt-4o-2024-08-06") {
		t.Errorf("skipped = %v; want it to name azure/gpt-4o-2024-08-06", skipped)
	}
}

func TestBuildPrices_ConflictingRatesFail(t *testing.T) {
	meters := []meter{
		{ProductName: "Azure Kimi", SkuName: "K2.5 Thinking Inp glbl", ArmRegionName: "eastus2", UnitPrice: 0.0006, UnitOfMeasure: "1K", Type: "Consumption"},
		{ProductName: "Azure Kimi", SkuName: "K2.5 Thinking Inp Global", ArmRegionName: "eastus2", UnitPrice: 0.0007, UnitOfMeasure: "1K", Type: "Consumption"},
	}
	if _, _, err := buildPrices(meters); err == nil {
		t.Error("two meters pricing one field differently must fail")
	}
}

func TestMatchMeter_AmbiguousStemFails(t *testing.T) {
	saved := models
	t.Cleanup(func() { models = saved })
	models = []modelSpec{
		{catalogAzureAI, "Azure Kimi", []string{"K2.5"}, []string{"kimi-a"}},
		{catalogAzureAI, "Azure Kimi", []string{"K2.5 Thinking", "K2.5"}, []string{"kimi-b"}},
	}
	m := meter{ProductName: "Azure Kimi", SkuName: "K2.5 cached glbl"}
	if _, _, _, err := matchMeter(m); err == nil {
		t.Error("a SKU matching two models must fail")
	}
}

// A stem matches whole words only: "5" is GPT-5, never GPT-5.1.
func TestClassifyAfterStem_WholeWordOnly(t *testing.T) {
	if _, ok := classifyAfterStem("5.1 pp inp gl", "5"); ok {
		t.Error(`stem "5" must not match "5.1 ..."`)
	}
	if _, ok := classifyAfterStem("5 pp inp gl", "5"); !ok {
		t.Error(`stem "5" must match "5 pp inp gl"`)
	}
}

func TestPerTokenRate(t *testing.T) {
	tests := []struct {
		price float64
		unit  string
		want  float64
		ok    bool
	}{
		{0.4, "1M", 4e-07, true},
		{0.000781, "1K", 7.81e-07, true},
		{1.7, "1 Hour", 0, false},
		{0, "1M", 0, false},
	}
	for _, tc := range tests {
		got, ok := perTokenRate(meter{UnitPrice: tc.price, UnitOfMeasure: tc.unit})
		if ok != tc.ok || got != tc.want {
			t.Errorf("perTokenRate(%v per %s) = %v, %v; want %v, %v", tc.price, tc.unit, got, ok, tc.want, tc.ok)
		}
	}
}

func TestEntryPrefix(t *testing.T) {
	tests := []struct {
		d      deployment
		region string
		want   string
		ok     bool
	}{
		{deploymentGlobal, "eastus2", "", true},
		{deploymentGlobal, "swedencentral", "", false},
		{deploymentRegional, "eastus2", "regional/", true},
		{deploymentRegional, "japaneast", "", false},
		{deploymentDataZone, "swedencentral", "eu/", true},
		{deploymentDataZone, "westus", "", false},
	}
	for _, tc := range tests {
		got, ok := entryPrefix(tc.d, tc.region)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("entryPrefix(%s, %s) = %q, %v; want %q, %v", tc.d, tc.region, got, ok, tc.want, tc.ok)
		}
	}
}

// roundRate applies the same 12-significant-digit rounding as perTokenRate.
func roundRate(v float64) float64 {
	r, _ := perTokenRate(meter{UnitPrice: v * 1e6, UnitOfMeasure: "1M"})
	return r
}

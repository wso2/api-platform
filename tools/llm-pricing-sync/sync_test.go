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
	"encoding/json"
	"os"
	"slices"
	"sort"
	"testing"
)

func loadSample(t *testing.T) *pricingFile {
	t.Helper()
	data, err := os.ReadFile("testdata/model_prices_sample.json")
	if err != nil {
		t.Fatalf("reading sample: %v", err)
	}
	f, err := parsePricingFile(data)
	if err != nil {
		t.Fatalf("parsing sample: %v", err)
	}
	return f
}

func entryFields(t *testing.T, f *pricingFile, key string) map[string]json.RawMessage {
	t.Helper()
	raw, ok := f.entries[key]
	if !ok {
		t.Fatalf("entry %q missing", key)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("decoding %q: %v", key, err)
	}
	return fields
}

// An update replaces the managed rates, matches the key case-insensitively,
// and keeps every field it does not manage, including nested objects.
func TestApplyPrices_UpdatesManagedFieldsOnly(t *testing.T) {
	f := loadSample(t)
	p := prices{"azure_ai/llama-3.3-70b-instruct": {fieldInput: 7.1e-07, fieldOutput: 7.2e-07}}

	report, err := applyPrices(f, p)
	if err != nil {
		t.Fatalf("applyPrices: %v", err)
	}
	if !slices.Contains(report.updated, "azure_ai/Llama-3.3-70B-Instruct") || len(report.added) != 0 {
		t.Fatalf("report = %+v; want the existing entry updated in place", report)
	}
	fields := entryFields(t, f, "azure_ai/Llama-3.3-70B-Instruct")
	if string(fields[fieldOutput]) != "7.2e-07" {
		t.Errorf("output rate = %s; want 7.2e-07", fields[fieldOutput])
	}
	if _, ok := fields["input_cost_per_token_priority"]; ok {
		t.Error("a managed rate Azure no longer meters must be dropped")
	}
	for _, kept := range []string{"max_tokens", "mode", "source", "search_context_cost_per_query"} {
		if _, ok := fields[kept]; !ok {
			t.Errorf("unmanaged field %q was lost", kept)
		}
	}
}

// New entries land in sorted position; dead tier keys and the model router are
// pruned; a global-standard duplicate goes only when its base was refreshed.
func TestApplyPrices_AddsAndPrunes(t *testing.T) {
	f := loadSample(t)
	p := prices{
		"azure/gpt-5.6-sol":    {fieldInput: 4e-06, fieldOutput: 2e-05},
		"azure/us/gpt-5.6-sol": {fieldInput: 4.4e-06, fieldOutput: 2.2e-05},
	}
	report, err := applyPrices(f, p)
	if err != nil {
		t.Fatalf("applyPrices: %v", err)
	}

	if !sort.StringsAreSorted(f.keys) {
		t.Errorf("keys out of order after insert: %v", f.keys)
	}
	added := entryFields(t, f, "azure/us/gpt-5.6-sol")
	if string(added["provider"]) != `"azure"` || string(added["mode"]) != `"chat"` {
		t.Errorf("new entry = %v; want provider azure and mode chat", added)
	}
	wantRemoved := []string{
		"azure/global-standard/gpt-5.6-sol", // base refreshed
		"azure/global/gpt-5.1",              // never read
		"azure_ai/global/grok-3",            // never read
		"azure_ai/model_router",             // no Azure meter
	}
	slices.Sort(report.removed)
	if !slices.Equal(report.removed, wantRemoved) {
		t.Errorf("removed = %v; want %v", report.removed, wantRemoved)
	}
	if _, ok := f.entries["azure/global-standard/gpt-4o-2024-08-06"]; !ok {
		t.Error("a global-standard key whose base was not refreshed must be kept")
	}
}

// A second sync with the same prices changes nothing.
func TestApplyPrices_Idempotent(t *testing.T) {
	f := loadSample(t)
	p := prices{"azure/gpt-5.6-sol": {fieldInput: 4e-06, fieldOutput: 2e-05}}
	if _, err := applyPrices(f, p); err != nil {
		t.Fatalf("first applyPrices: %v", err)
	}
	report, err := applyPrices(f, p)
	if err != nil {
		t.Fatalf("second applyPrices: %v", err)
	}
	if report.changed() {
		t.Errorf("second sync changed %+v; want no changes", report)
	}
}

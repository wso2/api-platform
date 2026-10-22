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

import "testing"

// The SKU suffixes below are spelled as the Retail Prices API spells them.
func TestClassify(t *testing.T) {
	tests := []struct {
		suffix string
		want   meterClass
	}{
		{"ShortCo Cd Wr PP DZ", meterClass{categoryCacheWrite, deploymentDataZone, tierPriority, false}},
		{"LoCo Inp Fl Gl", meterClass{categoryInput, deploymentGlobal, tierFlex, true}},
		{"Inp Data Zone", meterClass{categoryInput, deploymentDataZone, tierStandard, false}},
		{"cached Inp regnl", meterClass{categoryCacheRead, deploymentRegional, tierStandard, false}},
		{"Cached glbl L", meterClass{categoryCacheRead, deploymentGlobal, tierStandard, true}},
		{"cchd Inpt DZone", meterClass{categoryCacheRead, deploymentDataZone, tierStandard, false}},
		{"outpt Glbl", meterClass{categoryOutput, deploymentGlobal, tierStandard, false}},
		{"Input", meterClass{categoryInput, deploymentGlobal, tierStandard, false}},
	}
	for _, tc := range tests {
		t.Run(tc.suffix, func(t *testing.T) {
			got, ok := classify(splitTokens(tc.suffix))
			if !ok || got != tc.want {
				t.Errorf("classify(%q) = %+v, %v; want %+v", tc.suffix, got, ok, tc.want)
			}
		})
	}
}

// Anything this tool does not price, or cannot place unambiguously, must not
// classify.
func TestClassify_Rejects(t *testing.T) {
	for _, suffix := range []string{
		"Batch outpt Glbl", // batch jobs bypass the gateway
		"Inp FT DZ",        // fine-tuned deployments
		"pp Fl Inp Gl",     // two tiers at once
		"Inp Gl DZ",        // two deployment types at once
		"Inp Opt Gl",       // input and output at once
		"Gl",               // no token category
		"Cd Opt Gl",        // cached output does not exist
		"Wr Inp Gl",        // a write without the cache marker
		"hosting regional", // hourly fine-tune hosting
		"Image Input glbl", // image tokens
	} {
		if got, ok := classify(splitTokens(suffix)); ok {
			t.Errorf("classify(%q) = %+v; want rejected", suffix, got)
		}
	}
}

func TestFieldName(t *testing.T) {
	tests := []struct {
		class meterClass
		want  string
	}{
		{meterClass{category: categoryInput}, "input_cost_per_token"},
		{meterClass{category: categoryCacheWrite, tier: tierFlex}, "cache_creation_input_token_cost_flex"},
		{meterClass{category: categoryOutput, longContext: true}, "output_cost_per_token_above_272k_tokens"},
		{meterClass{category: categoryCacheRead, longContext: true, tier: tierPriority}, "cache_read_input_token_cost_above_272k_tokens_priority"},
	}
	for _, tc := range tests {
		if got := fieldName(tc.class); got != tc.want {
			t.Errorf("fieldName(%+v) = %q; want %q", tc.class, got, tc.want)
		}
	}
}

// Four categories, with or without long context, on three tiers.
func TestManagedFields(t *testing.T) {
	if len(managedFields) != 24 {
		t.Errorf("managedFields has %d names; want 24", len(managedFields))
	}
	for _, unmanaged := range []string{"max_tokens", "cache_creation_input_token_cost_above_1hr", "input_cost_per_token_batches"} {
		if managedFields[unmanaged] {
			t.Errorf("%q must not be managed", unmanaged)
		}
	}
}

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

// Base rate fields, as named in the pricing file shared by llm-cost and
// azure-llm-cost.
const (
	fieldInput      = "input_cost_per_token"
	fieldOutput     = "output_cost_per_token"
	fieldCacheRead  = "cache_read_input_token_cost"
	fieldCacheWrite = "cache_creation_input_token_cost"
)

const fieldLongContextSuffix = "_above_272k_tokens"

var baseFields = map[category]string{
	categoryInput:      fieldInput,
	categoryOutput:     fieldOutput,
	categoryCacheRead:  fieldCacheRead,
	categoryCacheWrite: fieldCacheWrite,
}

// fieldName is the rate field a meter sets: the base field, then the
// long-context suffix, then the tier suffix, matching azure-llm-cost's schema
// (e.g. input_cost_per_token_above_272k_tokens_priority).
func fieldName(c meterClass) string {
	name := baseFields[c.category]
	if c.longContext {
		name += fieldLongContextSuffix
	}
	if c.tier != tierStandard {
		name += "_" + string(c.tier)
	}
	return name
}

// managedFields are every field this tool can write. Updating an entry clears
// exactly these before writing fresh rates, so a meter Azure withdraws stops
// being billed, while fields the tool never writes are left alone.
var managedFields = func() map[string]bool {
	set := map[string]bool{}
	for cat := range baseFields {
		for _, long := range []bool{false, true} {
			for _, t := range []tier{tierStandard, tierPriority, tierFlex} {
				set[fieldName(meterClass{category: cat, longContext: long, tier: t})] = true
			}
		}
	}
	return set
}()

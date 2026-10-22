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
	"strings"
)

// category is the token class a meter prices.
type category string

const (
	categoryInput      category = "input"
	categoryOutput     category = "output"
	categoryCacheRead  category = "cacheRead"
	categoryCacheWrite category = "cacheWrite"
)

// deployment is the Azure deployment type a meter belongs to.
type deployment string

const (
	deploymentGlobal   deployment = "global"
	deploymentDataZone deployment = "datazone"
	deploymentRegional deployment = "regional"
)

// tier is the processing tier a meter belongs to.
type tier string

const (
	tierStandard tier = ""
	tierPriority tier = "priority"
	tierFlex     tier = "flex"
)

// meterClass is what a meter's SKU name says about the price it carries.
type meterClass struct {
	category    category
	deployment  deployment
	tier        tier
	longContext bool
}

// Azure abbreviates each attribute several ways across product families. A
// token not listed here makes the SKU unclassifiable, which is how batch,
// fine-tuning, realtime, audio and image meters are kept out.
var (
	inputTokens    = tokenSet("inp", "input", "inpt", "in")
	outputTokens   = tokenSet("opt", "outp", "output", "outpt", "out")
	cachedTokens   = tokenSet("cd", "cached", "cchd")
	writeTokens    = tokenSet("wr")
	priorityTokens = tokenSet("pp")
	flexTokens     = tokenSet("fl", "flex")
	longTokens     = tokenSet("longco", "loco", "l")
	neutralTokens  = tokenSet("std", "shortco", "shco")
	globalTokens   = tokenSet("gl", "glbl", "global")
	dataZoneTokens = tokenSet("dz", "dzone", "datazone", "data", "zone")
	regionalTokens = tokenSet("regnl", "rgnl", "regional")
)

func tokenSet(tokens ...string) map[string]bool {
	set := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		set[t] = true
	}
	return set
}

// splitTokens lowercases s and splits it on spaces and dashes.
func splitTokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ' ' || r == '-' })
}

// tokenFlags records which attribute tokens a SKU carries.
type tokenFlags struct {
	in, out, cached, write      bool
	priority, flex, longContext bool
	global, dataZone, regional  bool
}

// scanTokens sets a flag per known token and reports false on any other.
func scanTokens(tokens []string) (tokenFlags, bool) {
	var f tokenFlags
	for _, t := range tokens {
		switch {
		case inputTokens[t]:
			f.in = true
		case outputTokens[t]:
			f.out = true
		case cachedTokens[t]:
			f.cached = true
		case writeTokens[t]:
			f.write = true
		case priorityTokens[t]:
			f.priority = true
		case flexTokens[t]:
			f.flex = true
		case longTokens[t]:
			f.longContext = true
		case globalTokens[t]:
			f.global = true
		case dataZoneTokens[t]:
			f.dataZone = true
		case regionalTokens[t]:
			f.regional = true
		case neutralTokens[t]:
		default:
			return tokenFlags{}, false
		}
	}
	return f, true
}

// classify reads the attribute tokens that follow a model stem. It reports
// false when any token is unknown or the tokens contradict each other. A SKU
// naming no deployment type is a Global one, as Phi's are.
func classify(tokens []string) (meterClass, bool) {
	f, ok := scanTokens(tokens)
	if !ok {
		return meterClass{}, false
	}
	cat, ok := categoryOf(f.in, f.out, f.cached, f.write)
	if !ok || (f.priority && f.flex) || countTrue(f.global, f.dataZone, f.regional) > 1 {
		return meterClass{}, false
	}

	class := meterClass{category: cat, deployment: deploymentGlobal, tier: tierStandard, longContext: f.longContext}
	switch {
	case f.dataZone:
		class.deployment = deploymentDataZone
	case f.regional:
		class.deployment = deploymentRegional
	}
	switch {
	case f.priority:
		class.tier = tierPriority
	case f.flex:
		class.tier = tierFlex
	}
	return class, true
}

// categoryOf resolves the token flags to one category. A meter naming no
// category, or both input and output, prices nothing this tool understands.
func categoryOf(in, out, cached, write bool) (category, bool) {
	switch {
	case write:
		return categoryCacheWrite, cached
	case cached:
		return categoryCacheRead, !out
	case in && !out:
		return categoryInput, true
	case out && !in:
		return categoryOutput, true
	}
	return "", false
}

func countTrue(flags ...bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}

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
	"strconv"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// The gateway represents LLM security and rate-limiting as entries in the generic
// policy list, whereas the control plane (AI Workspace) models them as first-class
// Security and RateLimiting structures. These are the policy names the CP->DP
// conversion emits (see llm_deployment.go); the import reverses that mapping.
const (
	importPolicyAPIKeyAuth        = "api-key-auth"
	importPolicyTokenRateLimit    = "token-based-ratelimit"
	importPolicyAdvancedRateLimit = "advanced-ratelimit"
	importPolicyBasicRateLimit    = "basic-ratelimit"
	importPolicyCostRateLimit     = "llm-cost-based-ratelimit"
	importPolicyLLMCost           = "llm-cost"
)

// liftLLMPolicies reverses the CP->DP flattening: it extracts the security and
// rate-limiting settings that the gateway carries as policies back into the control
// plane's first-class Security and RateLimiting structures, and returns the
// remaining (genuine) policies in their original order.
//
// liftRateLimits controls whether rate-limit policies (token/advanced/cost) are lifted
// into the returned RateLimitingConfig.
// Notes / known lossiness (inherent to the forward conversion):
//   - Reset windows are recovered in canonical minute/hour units, because the forward
//     formatting collapses day/week/month into hours.
//   - Provider Global and Provider ResourceWise.Default both serialize to the "/*"
//     path, so a "/*" entry is reconstructed as Global.
func liftLLMPolicies(policies []model.LLMPolicy, liftRateLimits bool) (*model.SecurityConfig, *model.LLMRateLimitingConfig, []model.LLMPolicy) {
	var security *model.SecurityConfig
	rl := &rateLimitBuilder{}
	remaining := make([]model.LLMPolicy, 0, len(policies))

	for _, p := range policies {
		// When the caller has no rate-limiting field to store into (proxies),
		// keep rate-limit policies as ordinary policies
		if !liftRateLimits &&
			(p.Name == importPolicyTokenRateLimit ||
				p.Name == importPolicyAdvancedRateLimit ||
				p.Name == importPolicyBasicRateLimit ||
				p.Name == importPolicyCostRateLimit) {
			remaining = append(remaining, p)
			continue
		}
		switch p.Name {
		case importPolicyAPIKeyAuth:
			if s := liftAPIKeySecurity(p); s != nil {
				security = s
			}
		case importPolicyTokenRateLimit:
			for _, path := range p.Paths {
				rl.addToken(path)
			}
		case importPolicyAdvancedRateLimit:
			for _, path := range p.Paths {
				rl.addRequest(path)
			}
		case importPolicyBasicRateLimit:
			for _, path := range p.Paths {
				rl.addBasicRequest(path)
			}
		case importPolicyCostRateLimit:
			for _, path := range p.Paths {
				rl.addCost(path)
			}
		default:
			// Genuine policies (including the llm-cost tracker) are preserved in order.
			remaining = append(remaining, p)
		}
	}

	return security, rl.build(), remaining
}

// liftAPIKeySecurity reconstructs the first-class API-key security config from an
// api-key-auth policy.
func liftAPIKeySecurity(p model.LLMPolicy) *model.SecurityConfig {
	for _, path := range p.Paths {
		key := asString(path.Params["key"])
		in := asString(path.Params["in"])
		valuePrefix := asString(path.Params["valuePrefix"])
		if key == "" && in == "" {
			continue
		}
		enabled := true
		return &model.SecurityConfig{
			Enabled: &enabled,
			APIKey: &model.APIKeySecurity{
				Enabled:     &enabled,
				Key:         key,
				In:          in,
				ValuePrefix: valuePrefix,
			},
		}
	}
	return nil
}

// rateLimitBuilder accumulates the per-scope (provider/consumer) and per-path
// (global "/*" vs resource) limits decoded from rate-limit policies, then assembles
// them into the nested LLMRateLimitingConfig.
type rateLimitBuilder struct {
	providerGlobal *model.RateLimitingLimitConfig
	providerRes    map[string]*model.RateLimitingLimitConfig
	providerOrder  []string
	consumerGlobal *model.RateLimitingLimitConfig
	consumerRes    map[string]*model.RateLimitingLimitConfig
	consumerOrder  []string
	used           bool
}

// target returns (creating if needed) the limit config for the given scope and path.
func (b *rateLimitBuilder) target(consumer bool, path string) *model.RateLimitingLimitConfig {
	b.used = true
	global := path == "/*" || path == ""
	if consumer {
		if global {
			if b.consumerGlobal == nil {
				b.consumerGlobal = &model.RateLimitingLimitConfig{}
			}
			return b.consumerGlobal
		}
		if b.consumerRes == nil {
			b.consumerRes = map[string]*model.RateLimitingLimitConfig{}
		}
		if b.consumerRes[path] == nil {
			b.consumerRes[path] = &model.RateLimitingLimitConfig{}
			b.consumerOrder = append(b.consumerOrder, path)
		}
		return b.consumerRes[path]
	}
	if global {
		if b.providerGlobal == nil {
			b.providerGlobal = &model.RateLimitingLimitConfig{}
		}
		return b.providerGlobal
	}
	if b.providerRes == nil {
		b.providerRes = map[string]*model.RateLimitingLimitConfig{}
	}
	if b.providerRes[path] == nil {
		b.providerRes[path] = &model.RateLimitingLimitConfig{}
		b.providerOrder = append(b.providerOrder, path)
	}
	return b.providerRes[path]
}

func (b *rateLimitBuilder) addToken(path model.LLMPolicyPath) {
	limits := asSlice(path.Params["totalTokenLimits"])
	if len(limits) == 0 {
		return
	}
	first := asMap(limits[0])
	cfg := b.target(asBool(path.Params["consumerBased"]), path.Path)
	cfg.Token = &model.TokenRateLimit{
		Enabled: true,
		Count:   asInt(first["count"]),
		Reset:   parseImportResetWindow(asString(first["duration"])),
	}
}

func (b *rateLimitBuilder) addRequest(path model.LLMPolicyPath) {
	quotas := asSlice(path.Params["quotas"])
	if len(quotas) == 0 {
		return
	}
	q := asMap(quotas[0])
	// Consumer scope is signalled either by the consumerBased flag or by the
	// "consumer-"-prefixed quota name the forward conversion uses.
	consumer := asBool(path.Params["consumerBased"]) || strings.HasPrefix(asString(q["name"]), "consumer-")
	limits := asSlice(q["limits"])
	if len(limits) == 0 {
		return
	}
	first := asMap(limits[0])
	cfg := b.target(consumer, path.Path)
	cfg.Request = &model.RequestRateLimit{
		Enabled: true,
		Count:   asInt(first["limit"]),
		Reset:   parseImportResetWindow(asString(first["duration"])),
	}
}

// addBasicRequest lifts a basic-ratelimit policy (params: limits[].requests) back into
// a provider-level request limit. basic-ratelimit carries no keyExtraction and no quota
// wrapper; the "/*" path maps to the provider Global/Default and any other path to a
// resource-wise limit.
func (b *rateLimitBuilder) addBasicRequest(path model.LLMPolicyPath) {
	limits := asSlice(path.Params["limits"])
	if len(limits) == 0 {
		return
	}
	first := asMap(limits[0])
	cfg := b.target(false, path.Path)
	cfg.Request = &model.RequestRateLimit{
		Enabled: true,
		Count:   asInt(first["requests"]),
		Reset:   parseImportResetWindow(asString(first["duration"])),
	}
}

func (b *rateLimitBuilder) addCost(path model.LLMPolicyPath) {
	budgets := asSlice(path.Params["budgetLimits"])
	if len(budgets) == 0 {
		return
	}
	first := asMap(budgets[0])
	cfg := b.target(asBool(path.Params["consumerBased"]), path.Path)
	cfg.Cost = &model.CostRateLimit{
		Enabled: true,
		Amount:  asFloat(first["amount"]),
		Reset:   parseImportResetWindow(asString(first["duration"])),
	}
}

func (b *rateLimitBuilder) build() *model.LLMRateLimitingConfig {
	if !b.used {
		return nil
	}
	out := &model.LLMRateLimitingConfig{
		ProviderLevel: buildRateLimitScope(b.providerGlobal, b.providerRes, b.providerOrder),
		ConsumerLevel: buildRateLimitScope(b.consumerGlobal, b.consumerRes, b.consumerOrder),
	}
	if out.ProviderLevel == nil && out.ConsumerLevel == nil {
		return nil
	}
	return out
}

// buildRateLimitScope assembles a scope: if any resource-specific limits exist the
// scope is ResourceWise (with the "/*" entry as the default); otherwise it is Global.
func buildRateLimitScope(global *model.RateLimitingLimitConfig,
	res map[string]*model.RateLimitingLimitConfig, order []string) *model.RateLimitingScopeConfig {
	if global == nil && len(res) == 0 {
		return nil
	}
	if len(res) == 0 {
		return &model.RateLimitingScopeConfig{Global: global}
	}
	rw := &model.ResourceWiseRateLimitingConfig{}
	if global != nil {
		rw.Default = *global
	}
	for _, path := range order {
		rw.Resources = append(rw.Resources, model.RateLimitingResourceLimit{
			Resource: path,
			Limit:    *res[path],
		})
	}
	return &model.RateLimitingScopeConfig{ResourceWise: rw}
}

// parseImportResetWindow parses a gateway duration string ("Nm" / "Nh") into a reset
// window. The unit is canonicalised to minute/hour (the forward conversion is lossy
// for day/week/month, which all serialize to hours).
func parseImportResetWindow(s string) model.RateLimitResetWindow {
	s = strings.TrimSpace(s)
	if s == "" {
		return model.RateLimitResetWindow{}
	}
	var unit, num string
	switch {
	case strings.HasSuffix(s, "m"):
		unit, num = "minute", strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "h"):
		unit, num = "hour", strings.TrimSuffix(s, "h")
	default:
		// Unsupported unit suffix: do not import a misleading zero-duration window.
		return model.RateLimitResetWindow{}
	}
	n, err := strconv.Atoi(strings.TrimSpace(num))
	if err != nil {
		return model.RateLimitResetWindow{}
	}
	return model.RateLimitResetWindow{Duration: n, Unit: unit}
}

// --- generic accessors for the JSON-decoded policy params ---

func asSlice(v interface{}) []interface{} {
	switch s := v.(type) {
	case []interface{}:
		return s
	case []map[string]interface{}:
		out := make([]interface{}, len(s))
		for i := range s {
			out[i] = s[i]
		}
		return out
	}
	return nil
}

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func asBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func asInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

func asFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

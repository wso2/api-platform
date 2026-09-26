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
	"fmt"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// This file pins the full DP->CP import matrix for LLM providers and proxies:
// every combination of the three policy-attachment fields a gateway artifact can
// carry, for both resource kinds. It is the executable form of the manual matrix
// in gateway/examples/issue-3110 (see its README), raised while investigating
// https://github.com/wso2/api-platform/issues/3110.
//
// The three fields are spec.globalPolicies (api-level), spec.operationPolicies
// (per path+method) and the deprecated spec.policies. The gateway controller
// rejects the deprecated field alongside either new one (covered by
// TestValidateLLMProvider_PolicyListExclusivity / ...Proxy... in
// gateway-controller's pkg/config), so of the eight on/off masks only the five
// below can reach the importer at all.
//
// Each policy list deliberately mixes an auth policy, a rate-limit policy and
// plain guardrails, because the importer routes those three kinds differently.

const (
	matrixAPILevelKey  = "X-API-Key"
	matrixResourceKey  = "X-Resource-API-Key"
	matrixChatPath     = "/chat/completions"
	matrixModelsPath   = "/models"
	policyContentGuard = "content-length-guardrail"
	policyWordGuard    = "word-count-guardrail"
	policySetHeaders   = "set-headers"
)

func paramsPtr(m map[string]interface{}) *map[string]interface{} { return &m }

// matrixGlobalPolicies mirrors the globalPolicies block of the issue-3110 fixtures.
func matrixGlobalPolicies() []api.Policy {
	return []api.Policy{
		{Name: importPolicyAPIKeyAuth, Version: "v1",
			Params: paramsPtr(map[string]interface{}{"key": matrixAPILevelKey, "in": "header"})},
		{Name: policySetHeaders, Version: "v1",
			Params: paramsPtr(map[string]interface{}{"response": map[string]interface{}{"headers": []interface{}{
				map[string]interface{}{"name": "X-I3110-Scope", "value": "global"}}}})},
		{Name: importPolicyCostRateLimit, Version: "v1",
			Params: paramsPtr(map[string]interface{}{
				"budgetLimits":  []interface{}{map[string]interface{}{"amount": 100, "duration": "1h"}},
				"consumerBased": false})},
	}
}

// matrixOperationPolicies mirrors the operationPolicies block of the fixtures.
func matrixOperationPolicies() []api.OperationPolicy {
	return []api.OperationPolicy{
		{Name: policyContentGuard, Version: "v1", Paths: []api.OperationPolicyPath{{
			Path: matrixChatPath, Methods: []api.OperationPolicyPathMethods{"POST"},
			Params: map[string]interface{}{"request": map[string]interface{}{"enabled": true, "min": 1, "max": 5000}}}}},
		{Name: policyWordGuard, Version: "v1", Paths: []api.OperationPolicyPath{{
			Path: matrixModelsPath, Methods: []api.OperationPolicyPathMethods{"GET"},
			Params: map[string]interface{}{"request": map[string]interface{}{"enabled": true, "min": 0, "max": 500}}}}},
		{Name: importPolicyTokenRateLimit, Version: "v1", Paths: []api.OperationPolicyPath{{
			Path: matrixChatPath, Methods: []api.OperationPolicyPathMethods{"POST"},
			Params: map[string]interface{}{
				"totalTokenLimits": []interface{}{map[string]interface{}{"count": 20000, "duration": "1h"}},
				"consumerBased":    false}}}},
		{Name: importPolicyAPIKeyAuth, Version: "v1", Paths: []api.OperationPolicyPath{{
			Path: matrixModelsPath, Methods: []api.OperationPolicyPathMethods{"GET"},
			Params: map[string]interface{}{"key": matrixResourceKey, "in": "header"}}}},
	}
}

// matrixLegacyPolicies mirrors the deprecated policies block of the fixtures.
func matrixLegacyPolicies() []api.LLMPolicy {
	return []api.LLMPolicy{
		{Name: importPolicyTokenRateLimit, Version: "v1", Paths: []api.LLMPolicyPath{{
			Path: matrixChatPath, Methods: []api.LLMPolicyPathMethods{"POST"},
			Params: map[string]interface{}{
				"totalTokenLimits": []interface{}{map[string]interface{}{"count": 10000, "duration": "1h"}},
				"consumerBased":    false}}}},
		{Name: policyContentGuard, Version: "v1", Paths: []api.LLMPolicyPath{{
			Path: matrixChatPath, Methods: []api.LLMPolicyPathMethods{"POST"},
			Params: map[string]interface{}{"request": map[string]interface{}{"enabled": true, "min": 1, "max": 4000}}}}},
	}
}

// importedView is the read-side projection of an imported artifact: what the
// platform API surfaces to the AI Workspace. Policies are split with the same
// helper the read path uses, so the expectations below are what a client sees.
type importedView struct {
	apiKeySecurityKey string   // security.apiKey.key, "" when no api-level auth
	globalPolicies    []string // names, in order
	operationPolicies []string // "name@path[METHOD,...]", in order
	rateLimitingShape []string // human-readable rateLimiting summary, nil when absent
}

func newImportedView(security *model.SecurityConfig, rl *model.LLMRateLimitingConfig,
	legacy []model.LLMPolicy) importedView {
	v := importedView{}
	if security != nil && security.APIKey != nil {
		v.apiKeySecurityKey = security.APIKey.Key
	}
	global, operation := splitLegacyPoliciesForRead(legacy)
	for _, p := range global {
		v.globalPolicies = append(v.globalPolicies, p.Name)
	}
	for _, p := range operation {
		for _, pe := range p.Paths {
			v.operationPolicies = append(v.operationPolicies,
				fmt.Sprintf("%s@%s%v", p.Name, pe.Path, pe.Methods))
		}
	}
	v.rateLimitingShape = summariseRateLimiting(rl)
	return v
}

// summariseRateLimiting flattens the nested rate-limiting config into stable,
// comparable strings so a test expectation reads like the shape it asserts.
func summariseRateLimiting(rl *model.LLMRateLimitingConfig) []string {
	if rl == nil {
		return nil
	}
	var out []string
	for _, scope := range []struct {
		name string
		cfg  *model.RateLimitingScopeConfig
	}{{"providerLevel", rl.ProviderLevel}, {"consumerLevel", rl.ConsumerLevel}} {
		if scope.cfg == nil {
			continue
		}
		if scope.cfg.Global != nil {
			out = append(out, fmt.Sprintf("%s.global{%s}", scope.name, summariseLimit(*scope.cfg.Global)))
		}
		if scope.cfg.ResourceWise != nil {
			if s := summariseLimit(scope.cfg.ResourceWise.Default); s != "" {
				out = append(out, fmt.Sprintf("%s.resourceWise.default{%s}", scope.name, s))
			}
			for _, r := range scope.cfg.ResourceWise.Resources {
				out = append(out, fmt.Sprintf("%s.resourceWise[%s]{%s}",
					scope.name, r.Resource, summariseLimit(r.Limit)))
			}
		}
	}
	return out
}

func summariseLimit(l model.RateLimitingLimitConfig) string {
	var parts []string
	if l.Token != nil {
		parts = append(parts, fmt.Sprintf("token=%d/%d%s", l.Token.Count, l.Token.Reset.Duration, l.Token.Reset.Unit))
	}
	if l.Request != nil {
		parts = append(parts, fmt.Sprintf("request=%d/%d%s", l.Request.Count, l.Request.Reset.Duration, l.Request.Reset.Unit))
	}
	if l.Cost != nil {
		parts = append(parts, fmt.Sprintf("cost=%g/%d%s", l.Cost.Amount, l.Cost.Reset.Duration, l.Cost.Reset.Unit))
	}
	return joinParts(parts)
}

func joinParts(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}

func assertStrings(t *testing.T, field string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s = %v, want %v", field, got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s = %v, want %v", field, got, want)
			return
		}
	}
}

func assertView(t *testing.T, got, want importedView) {
	t.Helper()
	if got.apiKeySecurityKey != want.apiKeySecurityKey {
		t.Errorf("security.apiKey.key = %q, want %q", got.apiKeySecurityKey, want.apiKeySecurityKey)
	}
	assertStrings(t, "globalPolicies", got.globalPolicies, want.globalPolicies)
	assertStrings(t, "operationPolicies", got.operationPolicies, want.operationPolicies)
	assertStrings(t, "rateLimiting", got.rateLimitingShape, want.rateLimitingShape)
}

// policyMatrixCase is one row of the matrix: which of the three fields the
// gateway artifact carries, and what each resource kind must surface for it.
type policyMatrixCase struct {
	name      string
	global    []api.Policy
	operation []api.OperationPolicy
	legacy    []api.LLMPolicy
	provider  importedView
	proxy     importedView
}

// policyMatrix is the five deployable combinations, with the expectation for each
// kind stated separately — provider and proxy diverge because only the provider
// model has a RateLimiting field for the importer to lift rate limits into.
func policyMatrix() []policyMatrixCase {
	return []policyMatrixCase{
		{
			name:     "none",
			provider: importedView{},
			proxy:    importedView{},
		},
		{
			name:   "global",
			global: matrixGlobalPolicies(),
			// Provider: api-key-auth -> Security, cost limit -> rateLimiting,
			// set-headers is the only genuine policy left.
			provider: importedView{
				apiKeySecurityKey: matrixAPILevelKey,
				globalPolicies:    []string{policySetHeaders},
				rateLimitingShape: []string{"providerLevel.global{cost=100/1hour}"},
			},
			// Proxy: same auth lift, but no rate-limiting field, so the cost
			// limit stays an ordinary global policy.
			proxy: importedView{
				apiKeySecurityKey: matrixAPILevelKey,
				globalPolicies:    []string{policySetHeaders, importPolicyCostRateLimit},
			},
		},
		{
			name:      "operation",
			operation: matrixOperationPolicies(),
			// No api-level attachment: the resource-scoped api-key-auth must NOT
			// become Security, and must keep its own path binding.
			provider: importedView{
				operationPolicies: []string{
					policyContentGuard + "@" + matrixChatPath + "[POST]",
					policyWordGuard + "@" + matrixModelsPath + "[GET]",
					importPolicyAPIKeyAuth + "@" + matrixModelsPath + "[GET]",
				},
				rateLimitingShape: []string{"providerLevel.resourceWise[" + matrixChatPath + "]{token=20000/1hour}"},
			},
			proxy: importedView{
				operationPolicies: []string{
					policyContentGuard + "@" + matrixChatPath + "[POST]",
					policyWordGuard + "@" + matrixModelsPath + "[GET]",
					importPolicyTokenRateLimit + "@" + matrixChatPath + "[POST]",
					importPolicyAPIKeyAuth + "@" + matrixModelsPath + "[GET]",
				},
			},
		},
		{
			name:      "global-operation",
			global:    matrixGlobalPolicies(),
			operation: matrixOperationPolicies(),
			// Both scopes of api-key-auth present: the api-level one wins Security
			// and the resource-scoped one still surfaces as an operation policy.
			// Note the api-level cost limit lands under resourceWise.default (not
			// .global) once any resource-scoped limit exists.
			provider: importedView{
				apiKeySecurityKey: matrixAPILevelKey,
				globalPolicies:    []string{policySetHeaders},
				operationPolicies: []string{
					policyContentGuard + "@" + matrixChatPath + "[POST]",
					policyWordGuard + "@" + matrixModelsPath + "[GET]",
					importPolicyAPIKeyAuth + "@" + matrixModelsPath + "[GET]",
				},
				rateLimitingShape: []string{
					"providerLevel.resourceWise.default{cost=100/1hour}",
					"providerLevel.resourceWise[" + matrixChatPath + "]{token=20000/1hour}",
				},
			},
			proxy: importedView{
				apiKeySecurityKey: matrixAPILevelKey,
				globalPolicies:    []string{policySetHeaders, importPolicyCostRateLimit},
				operationPolicies: []string{
					policyContentGuard + "@" + matrixChatPath + "[POST]",
					policyWordGuard + "@" + matrixModelsPath + "[GET]",
					importPolicyTokenRateLimit + "@" + matrixChatPath + "[POST]",
					importPolicyAPIKeyAuth + "@" + matrixModelsPath + "[GET]",
				},
			},
		},
		{
			name:   "deprecated",
			legacy: matrixLegacyPolicies(),
			// The deprecated list is treated exactly like operationPolicies.
			provider: importedView{
				operationPolicies: []string{policyContentGuard + "@" + matrixChatPath + "[POST]"},
				rateLimitingShape: []string{"providerLevel.resourceWise[" + matrixChatPath + "]{token=10000/1hour}"},
			},
			proxy: importedView{
				operationPolicies: []string{
					importPolicyTokenRateLimit + "@" + matrixChatPath + "[POST]",
					policyContentGuard + "@" + matrixChatPath + "[POST]",
				},
			},
		},
	}
}

// TestImportPolicyMatrix_Provider walks every deployable policy-attachment
// combination through the LLM provider importer and asserts the whole read-side
// projection: what becomes Security, what becomes RateLimiting, and how the rest
// splits into globalPolicies/operationPolicies.
func TestImportPolicyMatrix_Provider(t *testing.T) {
	for _, tc := range policyMatrix() {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mapLLMProviderSpecToConfig(dto.LLMProviderDeploymentSpec{
				DisplayName:       "I3110 Provider " + tc.name,
				Version:           "v1.0",
				Context:           "/i3110-prov-" + tc.name,
				Template:          "openai",
				Upstream:          dto.LLMUpstreamYAML{URL: "https://api.openai.com/v1"},
				AccessControl:     api.LLMAccessControl{Mode: api.DenyAll},
				GlobalPolicies:    tc.global,
				OperationPolicies: tc.operation,
				Policies:          tc.legacy,
			})
			assertView(t, newImportedView(cfg.Security, cfg.RateLimiting, cfg.Policies), tc.provider)
		})
	}
}

// TestImportPolicyMatrix_Proxy is the proxy counterpart. A proxy config has no
// RateLimiting field, so rate-limit policies must stay in the policy lists
// instead of being lifted and dropped.
func TestImportPolicyMatrix_Proxy(t *testing.T) {
	for _, tc := range policyMatrix() {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mapLLMProxySpecToConfig(dto.LLMProxyDeploymentSpec{
				DisplayName:       "I3110 Proxy " + tc.name,
				Version:           "v1.0",
				Context:           "/i3110-proxy-" + tc.name,
				Provider:          &dto.LLMProxyDeploymentProvider{ID: "i3110-prov-none"},
				GlobalPolicies:    tc.global,
				OperationPolicies: tc.operation,
				Policies:          tc.legacy,
			})
			view := newImportedView(cfg.Security, nil, cfg.Policies)
			assertView(t, view, tc.proxy)
		})
	}
}

// TestImportPolicyMatrix_ProxyNeverLiftsRateLimiting states the proxy invariant
// directly rather than only implying it through the matrix above: the proxy
// importer must never produce a rate-limiting config, because LLMProxyConfig has
// nowhere to store one and a lifted limit would be silently discarded.
func TestImportPolicyMatrix_ProxyNeverLiftsRateLimiting(t *testing.T) {
	for _, tc := range policyMatrix() {
		t.Run(tc.name, func(t *testing.T) {
			liftInput := mapGlobalPoliciesAPIToLLMPolicies(&tc.global)
			liftInput = append(liftInput, mapOperationPoliciesAPIToLLMPolicies(&tc.operation)...)
			liftInput = append(liftInput, mapPoliciesAPIToModel(&tc.legacy)...)

			_, rl, remaining := liftLLMPolicies(liftInput, false)
			if rl != nil {
				t.Fatalf("proxy import produced rate limiting %v; it has nowhere to store it", summariseRateLimiting(rl))
			}
			// Every rate-limit policy in the input must still be present.
			for _, in := range liftInput {
				if in.Name != importPolicyTokenRateLimit && in.Name != importPolicyCostRateLimit {
					continue
				}
				if findPolicy(remaining, in.Name) == nil {
					t.Errorf("%s was dropped instead of kept as an ordinary policy", in.Name)
				}
			}
		})
	}
}

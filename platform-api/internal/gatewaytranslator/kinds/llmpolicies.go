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

package kinds

import "github.com/wso2/api-platform/platform-api/api"

// The LLM provider and proxy kinds share one policy shape and therefore one
// pair of transforms, defined here and called from llmprovider.go and
// llmproxy.go:
//
//   - splitLegacyPolicies brings the flat `policies` list of a legacy stored
//     artifact up to the split globalPolicies/operationPolicies shape;
//   - flattenPolicyLists is its exact inverse, for gateways that only
//     understand the flat list.

// Policy names used when ordering the flattened list. Must stay in sync with
// service/llm_deployment.go's constants.
const (
	policyNameLLMCost               = "llm-cost"
	policyNameLLMCostBasedRateLimit = "llm-cost-based-ratelimit"
)

// splitLegacyPolicies folds a legacy flat policies list into the split
// globalPolicies/operationPolicies lists, mirroring the rule already used by
// service.migrateLegacyPolicies (service/llm.go):
//   - a path entry "/*" with methods ["*"] -> a global policy (deduped by name)
//   - any other path entry                 -> an operation policy path (merged
//     by name+version)
//
// Existing entries in globalPolicies/operationPolicies (if any) are preserved;
// legacy entries are folded in alongside them.
func splitLegacyPolicies(legacy []api.LLMPolicy, globalPolicies *[]api.Policy, operationPolicies *[]api.OperationPolicy) {
	for _, p := range legacy {
		for _, pe := range p.Paths {
			if pe.Path == "/*" && isWildcardOnlyMethods(pe.Methods) {
				if !hasGlobalPolicyByName(*globalPolicies, p.Name) {
					*globalPolicies = append(*globalPolicies, api.Policy{
						Name:    p.Name,
						Version: p.Version,
						Params:  paramsPtr(pe.Params),
					})
				}
			} else {
				appendOperationPath(operationPolicies, p.Name, p.Version, api.OperationPolicyPath{
					Path:    pe.Path,
					Methods: toOperationMethods(pe.Methods),
					Params:  pe.Params,
				})
			}
		}
	}
}

// isWildcardOnlyMethods reports whether methods is exactly ["*"].
func isWildcardOnlyMethods(methods []api.LLMPolicyPathMethods) bool {
	return len(methods) == 1 && methods[0] == "*"
}

// hasGlobalPolicyByName reports whether a policy with the given name already
// exists in globalPolicies.
func hasGlobalPolicyByName(policies []api.Policy, name string) bool {
	for _, p := range policies {
		if p.Name == name {
			return true
		}
	}
	return false
}

// appendOperationPath merges a path entry into an existing OperationPolicy of
// the same name+version, or appends a new OperationPolicy if none exists.
func appendOperationPath(policies *[]api.OperationPolicy, name, version string, path api.OperationPolicyPath) {
	for i := range *policies {
		if (*policies)[i].Name == name && (*policies)[i].Version == version {
			(*policies)[i].Paths = append((*policies)[i].Paths, path)
			return
		}
	}
	*policies = append(*policies, api.OperationPolicy{
		Name:    name,
		Version: version,
		Paths:   []api.OperationPolicyPath{path},
	})
}

func toOperationMethods(methods []api.LLMPolicyPathMethods) []api.OperationPolicyPathMethods {
	out := make([]api.OperationPolicyPathMethods, 0, len(methods))
	for _, m := range methods {
		out = append(out, api.OperationPolicyPathMethods(m))
	}
	return out
}

func paramsPtr(m map[string]interface{}) *map[string]interface{} {
	if m == nil {
		return nil
	}
	return &m
}

// flattenPolicyLists flattens globalPolicies and operationPolicies into the
// legacy policies slice:
//   - each global policy    -> a legacy entry with a single {path:"/*", methods:["*"]} path
//   - each operation policy -> a legacy entry with its paths copied 1:1
//
// The result is appended to *legacyPolicies (which may already contain
// security/consumer entries the generator assembled), then re-ordered so that
// llm-cost-based-ratelimit always precedes llm-cost.
func flattenPolicyLists(globalPolicies []api.Policy, operationPolicies []api.OperationPolicy, legacyPolicies *[]api.LLMPolicy) {
	for _, gp := range globalPolicies {
		params := map[string]interface{}{}
		if gp.Params != nil {
			params = *gp.Params
		}
		*legacyPolicies = append(*legacyPolicies, api.LLMPolicy{
			Name:    gp.Name,
			Version: gp.Version,
			Paths:   []api.LLMPolicyPath{{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: params}},
		})
	}
	for _, op := range operationPolicies {
		paths := make([]api.LLMPolicyPath, 0, len(op.Paths))
		for _, pp := range op.Paths {
			methods := make([]api.LLMPolicyPathMethods, 0, len(pp.Methods))
			for _, m := range pp.Methods {
				methods = append(methods, api.LLMPolicyPathMethods(m))
			}
			paths = append(paths, api.LLMPolicyPath{Path: pp.Path, Methods: methods, Params: pp.Params})
		}
		*legacyPolicies = append(*legacyPolicies, api.LLMPolicy{
			Name:    op.Name,
			Version: op.Version,
			Paths:   paths,
		})
	}
	*legacyPolicies = orderLegacyPolicies(*legacyPolicies)
}

// orderLegacyPolicies ensures llm-cost-based-ratelimit always precedes
// llm-cost in the legacy policy list (llm-cost depends on the ratelimit
// policy running first).
func orderLegacyPolicies(policies []api.LLMPolicy) []api.LLMPolicy {
	costIdx, rateLimitIdx := -1, -1
	for i, p := range policies {
		switch p.Name {
		case policyNameLLMCost:
			costIdx = i
		case policyNameLLMCostBasedRateLimit:
			rateLimitIdx = i
		}
	}
	if costIdx != -1 && rateLimitIdx != -1 && costIdx < rateLimitIdx {
		policies[costIdx], policies[rateLimitIdx] = policies[rateLimitIdx], policies[costIdx]
	}
	return policies
}

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

package v1alpha1

import (
	"fmt"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
)

// Policy names used when ordering the down-converted legacy list. Must stay
// in sync with service/llm_deployment.go's constants.
const (
	policyNameLLMCost               = "llm-cost"
	policyNameLLMCostBasedRateLimit = "llm-cost-based-ratelimit"
)

func init() {
	shapeHandlers[constants.LLMProvider] = downConvertLLMProvider
	shapeHandlers[constants.LLMProxy] = downConvertLLMProxy
}

// downConvertLLMProvider is the only kind with a data-shape transform:
// gateways < 1.2.0 don't understand globalPolicies/operationPolicies, so both
// are flattened into the legacy policies field.
func downConvertLLMProvider(payload any) error {
	artifact, ok := payload.(*dto.LLMProviderDeploymentYAML)
	if !ok {
		return fmt.Errorf("expected *dto.LLMProviderDeploymentYAML, got %T", payload)
	}
	flattenPolicyLists(artifact.Spec.GlobalPolicies, artifact.Spec.OperationPolicies, &artifact.Spec.Policies)
	artifact.Spec.GlobalPolicies = nil
	artifact.Spec.OperationPolicies = nil
	return nil
}

// downConvertLLMProxy is the proxy-config counterpart.
func downConvertLLMProxy(payload any) error {
	artifact, ok := payload.(*dto.LLMProxyDeploymentYAML)
	if !ok {
		return fmt.Errorf("expected *dto.LLMProxyDeploymentYAML, got %T", payload)
	}
	flattenPolicyLists(artifact.Spec.GlobalPolicies, artifact.Spec.OperationPolicies, &artifact.Spec.Policies)
	artifact.Spec.GlobalPolicies = nil
	artifact.Spec.OperationPolicies = nil
	return nil
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

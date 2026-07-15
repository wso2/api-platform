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

package normalizer

import (
	"fmt"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
)

func init() {
	shapeHandlers[constants.LLMProvider] = normalizeLLMProviderPolicies
	shapeHandlers[constants.LLMProxy] = normalizeLLMProxyPolicies
}

func normalizeLLMProviderPolicies(_ string, payload any) error {
	artifact, ok := payload.(*dto.LLMProviderDeploymentYAML)
	if !ok {
		return fmt.Errorf("expected *dto.LLMProviderDeploymentYAML, got %T", payload)
	}
	if len(artifact.Spec.Policies) == 0 {
		return nil
	}
	splitLegacyPolicies(artifact.Spec.Policies, &artifact.Spec.GlobalPolicies, &artifact.Spec.OperationPolicies)
	artifact.Spec.Policies = nil
	return nil
}

func normalizeLLMProxyPolicies(_ string, payload any) error {
	artifact, ok := payload.(*dto.LLMProxyDeploymentYAML)
	if !ok {
		return fmt.Errorf("expected *dto.LLMProxyDeploymentYAML, got %T", payload)
	}
	if len(artifact.Spec.Policies) == 0 {
		return nil
	}
	splitLegacyPolicies(artifact.Spec.Policies, &artifact.Spec.GlobalPolicies, &artifact.Spec.OperationPolicies)
	artifact.Spec.Policies = nil
	return nil
}

// splitLegacyPolicies folds a legacy flat policies list into the split
// globalPolicies/operationPolicies lists — the exact inverse of
// versiontranslator/v1alpha1's flattenPolicyLists — mirroring the rule
// already used by service.migrateLegacyPolicies (service/llm.go):
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

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

// Package systempolicies defines built-in system policies that are automatically
// injected into every event-gateway policy chain based on the runtime configuration.
//
// To add a new system policy, append an entry to defaultSystemPolicies. No other
// changes are required — Inject() picks it up automatically.
package systempolicies

import (
	"log/slog"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/pkg/engine"
)

const (
	// SharedParamsKey is the key in additionalProps for parameters that apply to all system policies.
	SharedParamsKey = "_shared"
)

// systemPolicyConfig describes a single system policy and when it should be injected.
type systemPolicyConfig struct {
	// Name must match the compiled policy definition name.
	Name string
	// Version is the policy version (e.g. "v1").
	Version string
	// Enabled returns true when this policy should be injected given the current config.
	Enabled func(*config.Config) bool
	// Parameters holds the lowest-precedence default parameters (overridable via additionalProps).
	Parameters map[string]interface{}
	// ExecutionCondition is an optional CEL condition that gates policy execution at runtime.
	ExecutionCondition *string
}

// defaultSystemPolicies is the registry of all built-in system policies.
// Policies are prepended to every chain in the order listed here.
//
// Adding a new system policy:
//  1. Append a systemPolicyConfig entry below.
//  2. Set Enabled to the condition that gates it (e.g. cfg.SomeFeature.Enabled).
//  3. Optionally provide default Parameters that operators can override via additionalProps.
var defaultSystemPolicies = []systemPolicyConfig{}

// Inject prepends enabled system policies to specs.
//
// Parameter merging precedence (highest → lowest):
//  1. Policy-specific:  additionalProps[policyName]   e.g. additionalProps["wso2_apip_sys_analytics"]
//  2. Shared:           additionalProps["_shared"]     applies to every system policy
//  3. Default:          systemPolicyConfig.Parameters  defined in defaultSystemPolicies
//
// additionalProps may be nil.
func Inject(specs []engine.PolicySpec, cfg *config.Config, additionalProps map[string]any) []engine.PolicySpec {
	if cfg == nil {
		slog.Error("Configuration is nil, skipping system policy injection")
		return specs
	}

	// Fast path: count enabled policies to avoid allocations when nothing is active.
	enabledCount := 0
	for _, sp := range defaultSystemPolicies {
		if sp.Enabled(cfg) {
			enabledCount++
		}
	}
	if enabledCount == 0 {
		return specs
	}

	systemSpecs := make([]engine.PolicySpec, 0, enabledCount)

	for _, sp := range defaultSystemPolicies {
		if !sp.Enabled(cfg) {
			continue
		}

		// Copy default parameters so we never mutate the package-level map.
		effectiveDefaults := make(map[string]interface{}, len(sp.Parameters)+1)
		for k, v := range sp.Parameters {
			effectiveDefaults[k] = v
		}

		systemSpecs = append(systemSpecs, engine.PolicySpec{
			Name:               sp.Name,
			Version:            sp.Version,
			Enabled:            true,
			ExecutionCondition: sp.ExecutionCondition,
			Parameters:         mergeParameters(effectiveDefaults, additionalProps, sp.Name),
		})
	}

	// Prepend so system policies always execute first.
	return append(systemSpecs, specs...)
}

// mergeParameters merges parameters with the precedence order described on Inject.
func mergeParameters(
	defaultParams map[string]interface{},
	additionalProps map[string]any,
	policyName string,
) map[string]interface{} {
	if len(additionalProps) == 0 {
		if len(defaultParams) == 0 {
			return nil
		}
		result := make(map[string]interface{}, len(defaultParams))
		for k, v := range defaultParams {
			result[k] = v
		}
		return result
	}

	maxSize := len(defaultParams)
	if sharedParams, ok := additionalProps[SharedParamsKey].(map[string]interface{}); ok {
		maxSize += len(sharedParams)
	}
	if policyParams, ok := additionalProps[policyName].(map[string]interface{}); ok {
		maxSize += len(policyParams)
	}

	result := make(map[string]interface{}, maxSize)

	for k, v := range defaultParams {
		result[k] = v
	}
	if sharedParams, ok := additionalProps[SharedParamsKey].(map[string]interface{}); ok {
		for k, v := range sharedParams {
			result[k] = v
		}
	}
	if policyParams, ok := additionalProps[policyName].(map[string]interface{}); ok {
		for k, v := range policyParams {
			result[k] = v
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

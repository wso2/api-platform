/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package utils

import (
	"log/slog"

	config "github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

const (
	// SharedParamsKey is the key in additionalProps for shared parameters that apply to all system policies
	SharedParamsKey = "_shared"
)

// systemPolicyConfig represents a single system policy and its enablement condition.
// The condition is evaluated against the runtime configuration to decide whether the
// policy should be injected into the chain.
type systemPolicyConfig struct {
	// Name of the policy (must match the compiled policy definition)
	Name string
	// Version of the policy
	Version string
	// Enabled returns true if this system policy should be injected given the config.
	Enabled func(*config.Config) bool
	// Parameters contains the default parameters for the policy (can be overridden by additionalProps)
	Parameters map[string]interface{}
	// ExecutionCondition contains the execution condition for the policy
	ExecutionCondition *string
}

// defaultSystemPolicies lists the built-in system policies that can be injected into
// all routes. New system policies can be added here with minimal changes elsewhere.
//
// Parameters can be set in three ways (in order of precedence):
// 1. Policy-specific in additionalProps: additionalProps["wso2_apip_sys_analytics"] = map[string]interface{}{"key": "value"}
// 2. Shared in additionalProps: additionalProps["_shared"] = map[string]interface{}{"key": "value"} (applies to all)
// 3. Default here: Parameters field in systemPolicyConfig (lowest precedence)
//
// Example:
//
//	defaultSystemPolicies = []systemPolicyConfig{
//	  {
//	    Name: "wso2_apip_sys_analytics",
//	    Version: "v0.0.1",
//	    Parameters: map[string]interface{}{
//	      "defaultKey": "defaultValue",  // Can be overridden by additionalProps
//	    },
//	  },
//	}
//
//	additionalProps = map[string]any{
//	  "_shared": map[string]interface{}{  // Applies to all system policies
//	    "sharedParam": "sharedValue",
//	  },
//	  "wso2_apip_sys_analytics": map[string]interface{}{  // Policy-specific
//	    "analyticsKey": "analyticsValue",
//	  },
//	}
var defaultSystemPolicies = []systemPolicyConfig{
	{
		// Analytics system policy: only enabled when analytics is turned on.
		Name:    constants.ANALYTICS_SYSTEM_POLICY_NAME,
		Version: constants.ANALYTICS_SYSTEM_POLICY_VERSION,
		Enabled: func(cfg *config.Config) bool {
			if cfg == nil {
				return false
			}
			slog.Debug("Analytics state -> ", "state", cfg.Analytics.Enabled)
			return cfg.Analytics.Enabled
		},
		// Default parameters (can be overridden via additionalProps)
		Parameters: map[string]interface{}{
			"allow_payloads": false,
		},
		ExecutionCondition: nil,
	},
}

// mergeParameters efficiently merges parameters with the following precedence (highest to lowest):
// 1. Policy-specific parameters from additionalProps[policyName]
// 2. Shared parameters from additionalProps["_shared"]
// 3. Default parameters from systemPolicyConfig.Parameters
// This function is optimized for performance with minimal allocations.
func mergeParameters(
	defaultParams map[string]interface{},
	additionalProps map[string]any,
	policyName string,
) map[string]interface{} {
	// Fast path: no additional props, return defaults (or empty map)
	if len(additionalProps) == 0 {
		if len(defaultParams) == 0 {
			return nil
		}
		// Return a copy to avoid mutating the original
		result := make(map[string]interface{}, len(defaultParams))
		for k, v := range defaultParams {
			result[k] = v
		}
		return result
	}

	// Calculate maximum size needed to avoid reallocations
	maxSize := len(defaultParams)
	if sharedParams, ok := additionalProps[SharedParamsKey].(map[string]interface{}); ok {
		maxSize += len(sharedParams)
	}
	if policyParams, ok := additionalProps[policyName].(map[string]interface{}); ok {
		maxSize += len(policyParams)
	}

	// Allocate result map with estimated capacity
	result := make(map[string]interface{}, maxSize)

	// Copy default parameters (lowest precedence)
	if len(defaultParams) > 0 {
		for k, v := range defaultParams {
			result[k] = v
		}
	}

	// Merge shared parameters (medium precedence)
	if sharedParams, ok := additionalProps[SharedParamsKey].(map[string]interface{}); ok && len(sharedParams) > 0 {
		for k, v := range sharedParams {
			result[k] = v
		}
	}

	// Merge policy-specific parameters (highest precedence)
	if policyParams, ok := additionalProps[policyName].(map[string]interface{}); ok && len(policyParams) > 0 {
		for k, v := range policyParams {
			result[k] = v
		}
	}

	// Return nil if empty to save memory
	if len(result) == 0 {
		return nil
	}

	return result
}

// InjectSystemPolicies injects system policies into a policy chain based on configuration.
// System policies are prepended to the chain (executed first).
//
// Parameter merging strategy (highest to lowest precedence):
// 1. Policy-specific: additionalProps[policyName] (e.g., additionalProps["wso2_apip_sys_analytics"])
// 2. Shared: additionalProps["_shared"] (applies to all system policies)
// 3. Default: systemPolicyConfig.Parameters (defined in defaultSystemPolicies)
//
// Returns the modified chain with system policies injected.
func InjectSystemPolicies(policies []policyenginev1.PolicyInstance, cfg *config.Config, additionalProps map[string]any) []policyenginev1.PolicyInstance {
	if cfg == nil {
		slog.Error("Configuration is nil, cannot inject system policies")
		return policies
	}

	// Fast path: no enabled policies, return early
	enabledCount := 0
	for _, sysPol := range defaultSystemPolicies {
		if sysPol.Enabled(cfg) {
			enabledCount++
		}
	}
	if enabledCount == 0 {
		return policies
	}

	// Pre-allocate slice with exact capacity for performance
	systemPolicies := make([]policyenginev1.PolicyInstance, 0, enabledCount)

	// Collect enabled system policies with merged parameters
	for _, sysPol := range defaultSystemPolicies {
		if sysPol.Enabled(cfg) {
			// Build effective default parameters, allowing runtime config to control allow_payloads.
			effectiveDefaults := make(map[string]interface{}, len(sysPol.Parameters)+1)
			for k, v := range sysPol.Parameters {
				effectiveDefaults[k] = v
			}
			// For the analytics system policy, propagate the allow_payloads flag from runtime config.
			if sysPol.Name == constants.ANALYTICS_SYSTEM_POLICY_NAME {
				effectiveDefaults["allow_payloads"] = cfg.Analytics.AllowPayloads
			}

			// Merge parameters efficiently
			mergedParams := mergeParameters(effectiveDefaults, additionalProps, sysPol.Name)

			systemPolicies = append(systemPolicies, policyenginev1.PolicyInstance{
				Name:               sysPol.Name,
				Version:            sysPol.Version,
				Enabled:            true,
				ExecutionCondition: sysPol.ExecutionCondition,
				Parameters:         mergedParams,
			})
		}
	}

	// Prepend system policies to the chain (they execute first)
	return append(systemPolicies, policies...)
}

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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

func TestMergeParameters_EmptyAdditionalProps(t *testing.T) {
	defaultParams := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}

	result := mergeParameters(defaultParams, nil, "testPolicy")
	assert.Equal(t, "value1", result["key1"])
	assert.Equal(t, "value2", result["key2"])
}

func TestMergeParameters_EmptyDefaultParams(t *testing.T) {
	result := mergeParameters(nil, nil, "testPolicy")
	assert.Nil(t, result)
}

func TestMergeParameters_SharedParams(t *testing.T) {
	defaultParams := map[string]interface{}{
		"key1": "default1",
	}
	additionalProps := map[string]any{
		SharedParamsKey: map[string]interface{}{
			"sharedKey": "sharedValue",
			"key1":      "sharedOverride",
		},
	}

	result := mergeParameters(defaultParams, additionalProps, "testPolicy")
	assert.Equal(t, "sharedOverride", result["key1"]) // shared overrides default
	assert.Equal(t, "sharedValue", result["sharedKey"])
}

func TestMergeParameters_PolicySpecificParams(t *testing.T) {
	defaultParams := map[string]interface{}{
		"key1": "default1",
	}
	additionalProps := map[string]any{
		SharedParamsKey: map[string]interface{}{
			"key1": "sharedOverride",
		},
		"testPolicy": map[string]interface{}{
			"key1":      "policyOverride",
			"policyKey": "policyValue",
		},
	}

	result := mergeParameters(defaultParams, additionalProps, "testPolicy")
	assert.Equal(t, "policyOverride", result["key1"]) // policy-specific overrides shared
	assert.Equal(t, "policyValue", result["policyKey"])
}

func TestMergeParameters_Precedence(t *testing.T) {
	// Test full precedence: policy-specific > shared > default
	defaultParams := map[string]interface{}{
		"a": "default_a",
		"b": "default_b",
		"c": "default_c",
	}
	additionalProps := map[string]any{
		SharedParamsKey: map[string]interface{}{
			"b": "shared_b",
			"c": "shared_c",
		},
		"myPolicy": map[string]interface{}{
			"c": "policy_c",
		},
	}

	result := mergeParameters(defaultParams, additionalProps, "myPolicy")
	assert.Equal(t, "default_a", result["a"]) // only default
	assert.Equal(t, "shared_b", result["b"])  // shared overrides default
	assert.Equal(t, "policy_c", result["c"])  // policy-specific overrides both
}

func TestMergeParameters_NoMatchingPolicy(t *testing.T) {
	defaultParams := map[string]interface{}{
		"key1": "default1",
	}
	additionalProps := map[string]any{
		"otherPolicy": map[string]interface{}{
			"key1": "otherValue",
		},
	}

	result := mergeParameters(defaultParams, additionalProps, "testPolicy")
	assert.Equal(t, "default1", result["key1"]) // no override, use default
}

func TestInjectSystemPolicies_NilConfig(t *testing.T) {
	policies := []policyenginev1.PolicyInstance{
		{Name: "existing", Version: "v1.0.0"},
	}

	result := InjectSystemPolicies(policies, nil, nil)
	assert.Equal(t, policies, result)
}

func TestInjectSystemPolicies_AnalyticsDisabled(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled: false,
		},
	}
	policies := []policyenginev1.PolicyInstance{
		{Name: "existing", Version: "v1.0.0"},
	}

	result := InjectSystemPolicies(policies, cfg, nil)
	assert.Len(t, result, 1)
	assert.Equal(t, "existing", result[0].Name)
}

func TestInjectSystemPolicies_AnalyticsEnabled(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled:       true,
			AllowPayloads: false,
		},
	}
	policies := []policyenginev1.PolicyInstance{
		{Name: "existing", Version: "v1.0.0"},
	}

	result := InjectSystemPolicies(policies, cfg, nil)
	assert.Len(t, result, 2)
	// System policy should be first
	assert.Equal(t, constants.ANALYTICS_SYSTEM_POLICY_NAME, result[0].Name)
	assert.Equal(t, constants.ANALYTICS_SYSTEM_POLICY_VERSION, result[0].Version)
	assert.True(t, result[0].Enabled)
	// Original policy should be second
	assert.Equal(t, "existing", result[1].Name)
}

func TestInjectSystemPolicies_AllowPayloadsTrue(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled:       true,
			AllowPayloads: true,
		},
	}

	result := InjectSystemPolicies(nil, cfg, nil)
	assert.Len(t, result, 1)
	assert.Equal(t, constants.ANALYTICS_SYSTEM_POLICY_NAME, result[0].Name)
	assert.Equal(t, true, result[0].Parameters["allow_payloads"])
}

func TestInjectSystemPolicies_AllowPayloadsFalse(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled:       true,
			AllowPayloads: false,
		},
	}

	result := InjectSystemPolicies(nil, cfg, nil)
	assert.Len(t, result, 1)
	assert.Equal(t, false, result[0].Parameters["allow_payloads"])
}

func TestInjectSystemPolicies_WithAdditionalProps(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled: true,
		},
	}
	additionalProps := map[string]any{
		constants.ANALYTICS_SYSTEM_POLICY_NAME: map[string]interface{}{
			"custom_param": "custom_value",
		},
	}

	result := InjectSystemPolicies(nil, cfg, additionalProps)
	assert.Len(t, result, 1)
	assert.Equal(t, "custom_value", result[0].Parameters["custom_param"])
}

func TestInjectSystemPolicies_WithSharedParams(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled: true,
		},
	}
	additionalProps := map[string]any{
		SharedParamsKey: map[string]interface{}{
			"shared_param": "shared_value",
		},
	}

	result := InjectSystemPolicies(nil, cfg, additionalProps)
	assert.Len(t, result, 1)
	assert.Equal(t, "shared_value", result[0].Parameters["shared_param"])
}

func TestInjectSystemPolicies_EmptyPolicies(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled: true,
		},
	}

	result := InjectSystemPolicies([]policyenginev1.PolicyInstance{}, cfg, nil)
	assert.Len(t, result, 1)
	assert.Equal(t, constants.ANALYTICS_SYSTEM_POLICY_NAME, result[0].Name)
}

func TestInjectSystemPolicies_PreservesExistingPolicies(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled: true,
		},
	}
	policies := []policyenginev1.PolicyInstance{
		{Name: "policy1", Version: "v1.0.0"},
		{Name: "policy2", Version: "v2.0.0"},
	}

	result := InjectSystemPolicies(policies, cfg, nil)
	assert.Len(t, result, 3)
	// System policies come first
	assert.Equal(t, constants.ANALYTICS_SYSTEM_POLICY_NAME, result[0].Name)
	// Original policies follow
	assert.Equal(t, "policy1", result[1].Name)
	assert.Equal(t, "policy2", result[2].Name)
}

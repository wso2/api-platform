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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// ratelimitDefs is a small set of loaded policy definitions used across the LLM tests.
func ratelimitDefs() map[string]models.PolicyDefinition {
	return map[string]models.PolicyDefinition{
		"basic-ratelimit|v1.0.0":       {Name: "basic-ratelimit", Version: "v1.0.0"},
		"basic-ratelimit|v2.0.0":       {Name: "basic-ratelimit", Version: "v2.0.0"},
		"token-based-ratelimit|v1.0.0": {Name: "token-based-ratelimit", Version: "v1.0.0"},
	}
}

func TestPolicyValidator_ValidateLLMProviderPolicies_Valid(t *testing.T) {
	validator := NewPolicyValidator(ratelimitDefs())

	cfg := &api.LLMProviderConfiguration{
		Spec: api.LLMProviderConfigData{
			GlobalPolicies: &[]api.Policy{
				{Name: "basic-ratelimit", Version: "v1"},
			},
			OperationPolicies: &[]api.OperationPolicy{
				{Name: "token-based-ratelimit", Version: "v1"},
			},
		},
	}

	errors := validator.ValidateLLMProviderPolicies(cfg)
	assert.Empty(t, errors, "expected no errors for valid LLM provider policies")
}

func TestPolicyValidator_ValidateLLMProviderPolicies_NonExistentName(t *testing.T) {
	validator := NewPolicyValidator(ratelimitDefs())

	cfg := &api.LLMProviderConfiguration{
		Spec: api.LLMProviderConfigData{
			GlobalPolicies: &[]api.Policy{
				{Name: "this-policy-does-not-exist", Version: "v1"},
			},
		},
	}

	errors := validator.ValidateLLMProviderPolicies(cfg)
	assert.Len(t, errors, 1, "expected one error for a non-existent policy name")
	assert.Contains(t, errors[0].Field, "spec.globalPolicies[0]")
	assert.Contains(t, errors[0].Message, "not found")
}

func TestPolicyValidator_ValidateLLMProviderPolicies_NonExistentMajorVersion(t *testing.T) {
	validator := NewPolicyValidator(ratelimitDefs())

	// The reproduction from issue #2466: a policy that exists but at a non-existent major version.
	cfg := &api.LLMProviderConfiguration{
		Spec: api.LLMProviderConfigData{
			GlobalPolicies: &[]api.Policy{
				{Name: "basic-ratelimit", Version: "v999"},
			},
		},
	}

	errors := validator.ValidateLLMProviderPolicies(cfg)
	assert.Len(t, errors, 1, "expected one error for a non-existent major version")
	assert.Contains(t, errors[0].Field, "spec.globalPolicies[0].version")
	assert.Contains(t, errors[0].Message, "major version 'v999' not found")
}

func TestPolicyValidator_ValidateLLMProviderPolicies_EmptyVersionResolvesToLatest(t *testing.T) {
	validator := NewPolicyValidator(ratelimitDefs())

	// An empty version is a valid input: it resolves to the latest available version.
	cfg := &api.LLMProviderConfiguration{
		Spec: api.LLMProviderConfigData{
			GlobalPolicies: &[]api.Policy{
				{Name: "basic-ratelimit", Version: ""},
			},
		},
	}

	errors := validator.ValidateLLMProviderPolicies(cfg)
	assert.Empty(t, errors, "expected empty version to resolve to latest without errors")
}

func TestPolicyValidator_ValidateLLMProviderPolicies_LegacyAndOperationErrors(t *testing.T) {
	validator := NewPolicyValidator(ratelimitDefs())

	cfg := &api.LLMProviderConfiguration{
		Spec: api.LLMProviderConfigData{
			OperationPolicies: &[]api.OperationPolicy{
				{Name: "missing-op-policy", Version: "v1"},
			},
			// Deprecated policies list is still honoured and must be validated.
			Policies: &[]api.LLMPolicy{
				{Name: "missing-legacy-policy", Version: "v1"},
			},
		},
	}

	errors := validator.ValidateLLMProviderPolicies(cfg)
	assert.Len(t, errors, 2, "expected errors for both operation and legacy policies")
	fields := []string{errors[0].Field, errors[1].Field}
	assert.Contains(t, fields, "spec.operationPolicies[0].version")
	assert.Contains(t, fields, "spec.policies[0].version")
}

func TestPolicyValidator_ValidateLLMProxyPolicies_Valid(t *testing.T) {
	validator := NewPolicyValidator(ratelimitDefs())

	cfg := &api.LLMProxyConfiguration{
		Spec: api.LLMProxyConfigData{
			GlobalPolicies: &[]api.Policy{
				{Name: "basic-ratelimit", Version: "v2"},
			},
		},
	}

	errors := validator.ValidateLLMProxyPolicies(cfg)
	assert.Empty(t, errors, "expected no errors for a valid LLM proxy policy")
}

func TestPolicyValidator_ValidateLLMProxyPolicies_NonExistentMajorVersion(t *testing.T) {
	validator := NewPolicyValidator(ratelimitDefs())

	cfg := &api.LLMProxyConfiguration{
		Spec: api.LLMProxyConfigData{
			GlobalPolicies: &[]api.Policy{
				{Name: "basic-ratelimit", Version: "v999"},
			},
		},
	}

	errors := validator.ValidateLLMProxyPolicies(cfg)
	assert.Len(t, errors, 1, "expected one error for a non-existent major version")
	assert.Contains(t, errors[0].Message, "major version 'v999' not found")
}

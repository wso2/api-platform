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

// paramDefs returns definitions whose "token-based-ratelimit" policy declares a parameter
// schema, so per-path params on operation-level and deprecated policies can be exercised.
// additionalProperties:false mirrors the shipped policy definitions.
func paramDefs() map[string]models.PolicyDefinition {
	schema := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []interface{}{"limit"},
		"properties": map[string]interface{}{
			"limit":    map[string]interface{}{"type": "integer", "minimum": float64(1)},
			"duration": map[string]interface{}{"type": "string"},
		},
	}
	return map[string]models.PolicyDefinition{
		"token-based-ratelimit|v1.0.0": {Name: "token-based-ratelimit", Version: "v1.0.0", Parameters: &schema},
		"no-schema-policy|v1.0.0":      {Name: "no-schema-policy", Version: "v1.0.0"},
	}
}

func TestPolicyValidator_ValidateLLMProviderPolicies_OperationPolicyParamsValid(t *testing.T) {
	validator := NewPolicyValidator(paramDefs())

	cfg := &api.LLMProviderConfiguration{
		Spec: api.LLMProviderConfigData{
			OperationPolicies: &[]api.OperationPolicy{
				{Name: "token-based-ratelimit", Version: "v1", Paths: []api.OperationPolicyPath{
					{Path: "/chat/completions", Params: map[string]interface{}{"limit": 100, "duration": "1m"}},
				}},
			},
		},
	}

	assert.Empty(t, validator.ValidateLLMProviderPolicies(cfg))
}

func TestPolicyValidator_ValidateLLMProviderPolicies_OperationPolicyParamsInvalid(t *testing.T) {
	validator := NewPolicyValidator(paramDefs())

	cfg := &api.LLMProviderConfiguration{
		Spec: api.LLMProviderConfigData{
			OperationPolicies: &[]api.OperationPolicy{
				{Name: "token-based-ratelimit", Version: "v1", Paths: []api.OperationPolicyPath{
					{Path: "/chat/completions", Params: map[string]interface{}{"limit": 100}},
					{Path: "/embeddings", Params: map[string]interface{}{"duration": "1m"}},
					{Path: "/responses", Params: map[string]interface{}{"limit": 0}},
					{Path: "/models", Params: map[string]interface{}{"limit": 1, "bogus": "x"}},
				}},
			},
		},
	}

	errors := validator.ValidateLLMProviderPolicies(cfg)
	assert.Len(t, errors, 3, "expected one error each for the missing, out-of-range and unknown param")

	fields := make([]string, 0, len(errors))
	for _, e := range errors {
		fields = append(fields, e.Field)
	}
	assert.NotContains(t, fields, "spec.operationPolicies[0].paths[0].params",
		"paths[0] is valid and must not be reported")
	assert.Contains(t, fields, "spec.operationPolicies[0].paths[1].params")
	assert.Contains(t, errors[0].Message, "limit is required")
	assert.Equal(t, "spec.operationPolicies[0].paths[2].params.limit", errors[1].Field)
	assert.Contains(t, errors[2].Message, "Additional property bogus is not allowed")
}

func TestPolicyValidator_ValidateLLMProviderPolicies_OperationPolicyMissingParamsFailsRequired(t *testing.T) {
	validator := NewPolicyValidator(paramDefs())

	cfg := &api.LLMProviderConfiguration{
		Spec: api.LLMProviderConfigData{
			OperationPolicies: &[]api.OperationPolicy{
				{Name: "token-based-ratelimit", Version: "v1", Paths: []api.OperationPolicyPath{
					{Path: "/chat/completions"}, // no params at all
				}},
			},
		},
	}

	errors := validator.ValidateLLMProviderPolicies(cfg)
	assert.Len(t, errors, 1)
	assert.Equal(t, "spec.operationPolicies[0].paths[0].params", errors[0].Field)
	assert.Contains(t, errors[0].Message, "limit is required")
}

func TestPolicyValidator_ValidateLLMProviderPolicies_OperationPolicyParamsCoerced(t *testing.T) {
	validator := NewPolicyValidator(paramDefs())

	// A rendered template ({{ env "LIMIT" }}) always produces a string; coercion must run
	// before schema validation so "100" satisfies the integer param.
	params := map[string]interface{}{"limit": "100", "duration": "1m"}
	cfg := &api.LLMProviderConfiguration{
		Spec: api.LLMProviderConfigData{
			OperationPolicies: &[]api.OperationPolicy{
				{Name: "token-based-ratelimit", Version: "v1", Paths: []api.OperationPolicyPath{
					{Path: "/chat/completions", Params: params},
				}},
			},
		},
	}

	assert.Empty(t, validator.ValidateLLMProviderPolicies(cfg))
	assert.Equal(t, float64(100), params["limit"], "params must be coerced in place")
}

func TestPolicyValidator_ValidateLLMProviderPolicies_OperationPolicyNoSchemaSkipsParams(t *testing.T) {
	validator := NewPolicyValidator(paramDefs())

	cfg := &api.LLMProviderConfiguration{
		Spec: api.LLMProviderConfigData{
			OperationPolicies: &[]api.OperationPolicy{
				{Name: "no-schema-policy", Version: "v1", Paths: []api.OperationPolicyPath{
					{Path: "/chat/completions", Params: map[string]interface{}{"anything": "goes"}},
				}},
			},
		},
	}

	assert.Empty(t, validator.ValidateLLMProviderPolicies(cfg),
		"a definition without a parameter schema must not reject params")
}

func TestPolicyValidator_ValidateLLMProviderPolicies_BadRefSkipsParamValidation(t *testing.T) {
	validator := NewPolicyValidator(paramDefs())

	cfg := &api.LLMProviderConfiguration{
		Spec: api.LLMProviderConfigData{
			OperationPolicies: &[]api.OperationPolicy{
				{Name: "token-based-ratelimit", Version: "v999", Paths: []api.OperationPolicyPath{
					{Path: "/chat/completions", Params: map[string]interface{}{"bogus": "x"}},
				}},
			},
		},
	}

	errors := validator.ValidateLLMProviderPolicies(cfg)
	assert.Len(t, errors, 1, "an unresolvable reference must report once, not also per path")
	assert.Contains(t, errors[0].Message, "major version 'v999' not found")
}

func TestPolicyValidator_ValidateLLMProxyPolicies_LegacyPolicyParamsInvalid(t *testing.T) {
	validator := NewPolicyValidator(paramDefs())

	cfg := &api.LLMProxyConfiguration{
		Spec: api.LLMProxyConfigData{
			Policies: &[]api.LLMPolicy{
				{Name: "token-based-ratelimit", Version: "v1", Paths: []api.LLMPolicyPath{
					{Path: "/chat/completions", Params: map[string]interface{}{"limit": 100}},
					{Path: "/embeddings", Params: map[string]interface{}{"limit": "not-a-number"}},
				}},
			},
		},
	}

	errors := validator.ValidateLLMProxyPolicies(cfg)
	assert.Len(t, errors, 1)
	assert.Equal(t, "spec.policies[0].paths[1].params.limit", errors[0].Field)
}

// The LLM->RestAPI transform merges the provider template's extraction params
// (requestModel, promptTokens, ...) into every operation-level policy attachment. Those keys
// are declared by no policy schema, and most schemas set additionalProperties:false — so
// validation must run against the user-authored params, never the post-merge result.
func TestPolicyValidator_ValidateLLMProviderPolicies_TemplateExtractionParamsNotRequired(t *testing.T) {
	validator := NewPolicyValidator(paramDefs())

	cfg := &api.LLMProviderConfiguration{
		Spec: api.LLMProviderConfigData{
			OperationPolicies: &[]api.OperationPolicy{
				{Name: "token-based-ratelimit", Version: "v1", Paths: []api.OperationPolicyPath{
					{Path: "/chat/completions", Params: map[string]interface{}{"limit": 100}},
				}},
			},
		},
	}
	assert.Empty(t, validator.ValidateLLMProviderPolicies(cfg),
		"user-authored params alone must validate; template params are merged later")

	// Sanity check that the merged shape would indeed be rejected, which is why the
	// derived RestAPI is deliberately not the validation input.
	merged := map[string]interface{}{
		"limit":        100,
		"requestModel": map[string]interface{}{"location": "payload", "identifier": "$.model"},
	}
	def := paramDefs()["token-based-ratelimit|v1.0.0"]
	assert.NotEmpty(t, validator.validatePolicyParams(merged, *def.Parameters, "p"))
}

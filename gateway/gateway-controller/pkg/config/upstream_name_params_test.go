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
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// upstreamNameDefs holds a routing policy whose nested parameter names a provider
// (marked), and a policy with a same-named parameter that is not marked.
func upstreamNameDefs() map[string]models.PolicyDefinition {
	router := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"fallback": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"mappingPatterns": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"to":       map[string]interface{}{"type": "string"},
								"provider": map[string]interface{}{"type": "string", UpstreamNameMarker: true},
							},
						},
					},
				},
			},
			"backend": map[string]interface{}{
				"anyOf": []interface{}{map[string]interface{}{"type": "string", UpstreamNameMarker: true}},
			},
		},
	}
	unmarked := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"provider": map[string]interface{}{"type": "string"}},
	}
	return map[string]models.PolicyDefinition{
		"router|v1.0.0":   {Name: "router", Version: "v1.0.0", Parameters: &router},
		"unmarked|v1.0.0": {Name: "unmarked", Version: "v1.0.0", Parameters: &unmarked},
	}
}

func fallbackTo(providers ...string) map[string]interface{} {
	patterns := make([]interface{}, 0, len(providers))
	for _, p := range providers {
		patterns = append(patterns, map[string]interface{}{"to": "m", "provider": p})
	}
	return map[string]interface{}{"fallback": map[string]interface{}{"mappingPatterns": patterns}}
}

func proxyWith(global *[]api.Policy, operation *[]api.OperationPolicy) *api.LLMProxyConfiguration {
	alias := "bedrock"
	return &api.LLMProxyConfiguration{
		Spec: api.LLMProxyConfigData{
			Provider:            &api.LLMProxyProvider{Id: "azure-v1"},
			AdditionalProviders: &[]api.LLMProxyAdditionalProvider{{Id: "bedrock-provider", As: &alias}},
			GlobalPolicies:      global,
			OperationPolicies:   operation,
		},
	}
}

func TestUpstreamNameParams_LLMProxyAcceptsItsProviders(t *testing.T) {
	v := NewPolicyValidator(upstreamNameDefs(), nil)
	params := fallbackTo("azure-v1", "bedrock", " bedrock ")
	errs := v.ValidateLLMProxyPolicies(proxyWith(
		&[]api.Policy{{Name: "router", Version: "v1", Params: &params}},
		&[]api.OperationPolicy{{Name: "router", Version: "v1", Paths: []api.OperationPolicyPath{{Path: "/chat/completions", Params: fallbackTo("bedrock")}}}},
	))
	assert.Empty(t, errs)
}

func TestUpstreamNameParams_LLMProxyRejectsUnknownProvider(t *testing.T) {
	v := NewPolicyValidator(upstreamNameDefs(), nil)
	global := fallbackTo("bedrock", "bedrok")
	errs := v.ValidateLLMProxyPolicies(proxyWith(
		&[]api.Policy{{Name: "router", Version: "v1", Params: &global}},
		&[]api.OperationPolicy{{Name: "router", Version: "v1", Paths: []api.OperationPolicyPath{{Path: "/chat/completions", Params: fallbackTo("bedrock-provider")}}}},
	))
	require.Len(t, errs, 2)
	assert.Equal(t, "spec.globalPolicies[0].params.fallback.mappingPatterns[1].provider", errs[0].Field)
	assert.Equal(t, "'bedrok' does not match any provider of this artifact; use one of: azure-v1, bedrock", errs[0].Message)
	// A provider with an alias is selected by its alias, not its id.
	assert.Equal(t, "spec.operationPolicies[0].paths[0].params.fallback.mappingPatterns[0].provider", errs[1].Field)
}

func TestUpstreamNameParams_UnmarkedAndTemplatedValuesAreNotChecked(t *testing.T) {
	v := NewPolicyValidator(upstreamNameDefs(), nil)
	unmarked := map[string]interface{}{"provider": "anything"}
	templated := fallbackTo(`{{ env "FALLBACK_PROVIDER" }}`)
	errs := v.ValidateLLMProxyPolicies(proxyWith(&[]api.Policy{
		{Name: "unmarked", Version: "v1", Params: &unmarked},
		{Name: "router", Version: "v1", Params: &templated},
	}, nil))
	assert.Empty(t, errs)
}

func TestUpstreamNameParams_LLMProviderUsesUpstreamDefinitions(t *testing.T) {
	v := NewPolicyValidator(upstreamNameDefs(), nil)
	params := map[string]interface{}{"backend": "secondary"}
	cfg := &api.LLMProviderConfiguration{Spec: api.LLMProviderConfigData{
		GlobalPolicies: &[]api.Policy{{Name: "router", Version: "v1", Params: &params}},
	}}
	errs := v.ValidateLLMProviderPolicies(cfg)
	require.Len(t, errs, 1)
	assert.Equal(t, "'secondary' does not match any upstream definition: this artifact has none", errs[0].Message)

	cfg.Spec.UpstreamDefinitions = &[]api.UpstreamDefinition{{Name: "secondary"}}
	assert.Empty(t, v.ValidateLLMProviderPolicies(cfg))
}

func TestUpstreamNameParams_RestAPIUsesUpstreamDefinitions(t *testing.T) {
	v := NewPolicyValidator(upstreamNameDefs(), nil)
	params := map[string]interface{}{"backend": "backend-b"}
	apiConfig := &api.RestAPI{Spec: api.APIConfigData{
		UpstreamDefinitions: &[]api.UpstreamDefinition{{Name: "backend-a"}},
		Operations: []api.Operation{{
			Policies: &[]api.Policy{{Name: "router", Version: "v1", Params: &params}},
		}},
	}}
	errs := v.ValidateRestAPIPolicies(apiConfig)
	require.Len(t, errs, 1)
	assert.Equal(t, "spec.operations[0].policies[0].params.backend", errs[0].Field)

	params["backend"] = "backend-a"
	assert.Empty(t, v.ValidateRestAPIPolicies(apiConfig))
}

func TestUpstreamNameParams_ComposedObjectAndArraySchemasAreTraversed(t *testing.T) {
	marked := map[string]interface{}{"type": "string", UpstreamNameMarker: true}
	composed := map[string]interface{}{
		"allOf": []interface{}{
			map[string]interface{}{"properties": map[string]interface{}{"backend": marked}},
			// A second subschema naming the same property must not duplicate the error.
			map[string]interface{}{"properties": map[string]interface{}{"backend": map[string]interface{}{"type": "string"}}},
			map[string]interface{}{"properties": map[string]interface{}{
				"targets": map[string]interface{}{
					"oneOf": []interface{}{map[string]interface{}{"type": "array", "items": marked}},
				},
			}},
		},
	}
	defs := map[string]models.PolicyDefinition{"composed|v1.0.0": {Name: "composed", Version: "v1.0.0", Parameters: &composed}}
	v := NewPolicyValidator(defs, nil)
	params := map[string]interface{}{"backend": "typo", "targets": []interface{}{"primary", "other"}}
	cfg := &api.LLMProviderConfiguration{Spec: api.LLMProviderConfigData{
		UpstreamDefinitions: &[]api.UpstreamDefinition{{Name: "primary"}},
		GlobalPolicies:      &[]api.Policy{{Name: "composed", Version: "v1", Params: &params}},
	}}
	errs := v.ValidateLLMProviderPolicies(cfg)
	require.Len(t, errs, 2)
	assert.Equal(t, "spec.globalPolicies[0].params.backend", errs[0].Field)
	assert.Equal(t, "spec.globalPolicies[0].params.targets[1]", errs[1].Field)

	params = map[string]interface{}{"backend": "primary", "targets": []interface{}{"primary"}}
	assert.Empty(t, v.ValidateLLMProviderPolicies(cfg))
}

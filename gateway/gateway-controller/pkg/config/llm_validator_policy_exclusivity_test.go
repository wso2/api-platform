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
)

// policyExclusivityErr reports whether the validation errors contain the
// legacy/new policy-list coexistence error on spec.policies.
func policyExclusivityErr(errors []ValidationError) bool {
	for _, err := range errors {
		if err.Field == "spec.policies" &&
			err.Message == "The deprecated 'policies' field cannot be used together with 'globalPolicies' or "+
				"'operationPolicies'. Use either the legacy 'policies' list or the new "+
				"'globalPolicies'/'operationPolicies' lists, not both." {
			return true
		}
	}
	return false
}

func sampleGlobalPolicies() *[]api.Policy {
	return &[]api.Policy{{Name: "basic-ratelimit", Version: "v1"}}
}

func sampleOperationPolicies() *[]api.OperationPolicy {
	return &[]api.OperationPolicy{{
		Name:    "basic-ratelimit",
		Version: "v1",
		Paths:   []api.OperationPolicyPath{{Path: "/chat/completions", Methods: []api.OperationPolicyPathMethods{"GET"}}},
	}}
}

func sampleLegacyPolicies() *[]api.LLMPolicy {
	return &[]api.LLMPolicy{{
		Name:    "basic-ratelimit",
		Version: "v1",
		Paths:   []api.LLMPolicyPath{{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"GET"}}},
	}}
}

func providerWithPolicies(global *[]api.Policy, operation *[]api.OperationPolicy, legacy *[]api.LLMPolicy) *api.LLMProviderConfiguration {
	return &api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha2,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: "openai"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "my-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl:     api.LLMAccessControl{Mode: api.AllowAll},
			GlobalPolicies:    global,
			OperationPolicies: operation,
			Policies:          legacy,
		},
	}
}

func proxyWithPolicies(global *[]api.Policy, operation *[]api.OperationPolicy, legacy *[]api.LLMPolicy) *api.LLMProxyConfiguration {
	return &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha2,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "openai-proxy"},
		Spec: api.LLMProxyConfigData{
			DisplayName:       "my-proxy",
			Version:           "v1.0",
			Provider:          api.LLMProxyProvider{Id: "openai"},
			GlobalPolicies:    global,
			OperationPolicies: operation,
			Policies:          legacy,
		},
	}
}

func TestValidateLLMProvider_PolicyListExclusivity(t *testing.T) {
	validator := NewLLMValidator()

	tests := []struct {
		name      string
		global    *[]api.Policy
		operation *[]api.OperationPolicy
		legacy    *[]api.LLMPolicy
		wantErr   bool
	}{
		{name: "legacy only", legacy: sampleLegacyPolicies(), wantErr: false},
		{name: "globalPolicies only", global: sampleGlobalPolicies(), wantErr: false},
		{name: "operationPolicies only", operation: sampleOperationPolicies(), wantErr: false},
		{name: "global + operation (both new)", global: sampleGlobalPolicies(), operation: sampleOperationPolicies(), wantErr: false},
		{name: "no policies at all", wantErr: false},
		{name: "legacy + globalPolicies", global: sampleGlobalPolicies(), legacy: sampleLegacyPolicies(), wantErr: true},
		{name: "legacy + operationPolicies", operation: sampleOperationPolicies(), legacy: sampleLegacyPolicies(), wantErr: true},
		{name: "legacy + both new", global: sampleGlobalPolicies(), operation: sampleOperationPolicies(), legacy: sampleLegacyPolicies(), wantErr: true},
		// Empty (non-nil) slices must not count as "present".
		{name: "empty legacy + globalPolicies", global: sampleGlobalPolicies(), legacy: &[]api.LLMPolicy{}, wantErr: false},
		{name: "legacy + empty new lists", global: &[]api.Policy{}, operation: &[]api.OperationPolicy{}, legacy: sampleLegacyPolicies(), wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.Validate(providerWithPolicies(tt.global, tt.operation, tt.legacy))
			assert.Equal(t, tt.wantErr, policyExclusivityErr(errors),
				"unexpected policy-exclusivity result; errors=%v", errors)
		})
	}
}

func TestValidateLLMProxy_PolicyListExclusivity(t *testing.T) {
	validator := NewLLMValidator()

	tests := []struct {
		name      string
		global    *[]api.Policy
		operation *[]api.OperationPolicy
		legacy    *[]api.LLMPolicy
		wantErr   bool
	}{
		{name: "legacy only", legacy: sampleLegacyPolicies(), wantErr: false},
		{name: "globalPolicies only", global: sampleGlobalPolicies(), wantErr: false},
		{name: "operationPolicies only", operation: sampleOperationPolicies(), wantErr: false},
		{name: "no policies at all", wantErr: false},
		{name: "legacy + globalPolicies", global: sampleGlobalPolicies(), legacy: sampleLegacyPolicies(), wantErr: true},
		{name: "legacy + operationPolicies", operation: sampleOperationPolicies(), legacy: sampleLegacyPolicies(), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.Validate(proxyWithPolicies(tt.global, tt.operation, tt.legacy))
			assert.Equal(t, tt.wantErr, policyExclusivityErr(errors),
				"unexpected policy-exclusivity result; errors=%v", errors)
		})
	}
}

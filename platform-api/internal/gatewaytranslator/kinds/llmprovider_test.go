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

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/translate"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func newProviderArtifact() *dto.LLMProviderDeploymentYAML {
	return &dto.LLMProviderDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.LLMProvider}
}

func sampleGlobal(name string) api.Policy {
	return api.Policy{Name: name, Version: "v1"}
}

func sampleOperation(name, path string) api.OperationPolicy {
	return api.OperationPolicy{
		Name:    name,
		Version: "v1",
		Paths:   []api.OperationPolicyPath{{Path: path, Methods: []api.OperationPolicyPathMethods{"POST"}, Params: map[string]interface{}{}}},
	}
}

func apiAuth(authType string) *api.UpstreamAuth {
	tt := api.UpstreamAuthType(authType)
	header := "Authorization"
	value := `{{ secret "k" }}`
	return &api.UpstreamAuth{Type: &tt, Header: &header, Value: &value}
}

// --- Normalize: legacy flat policies -> split lists --------------------------

func TestLLMProvider_Normalize_LegacyFlat_WildcardBecomesGlobal(t *testing.T) {
	a := newProviderArtifact()
	a.Spec.Policies = []api.LLMPolicy{{
		Name:    "basic-ratelimit",
		Version: "v1",
		Paths:   []api.LLMPolicyPath{{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"requests": 10}}},
	}}

	require.NoError(t, LLMProvider.Normalize("1.0", a))

	require.Len(t, a.Spec.GlobalPolicies, 1)
	assert.Equal(t, "basic-ratelimit", a.Spec.GlobalPolicies[0].Name)
	assert.Equal(t, map[string]interface{}{"requests": 10}, *a.Spec.GlobalPolicies[0].Params)
	assert.Empty(t, a.Spec.OperationPolicies)
	assert.Empty(t, a.Spec.Policies)
}

func TestLLMProvider_Normalize_LegacyFlat_SpecificPathBecomesOperation(t *testing.T) {
	a := newProviderArtifact()
	a.Spec.Policies = []api.LLMPolicy{{
		Name:    "basic-ratelimit",
		Version: "v1",
		Paths:   []api.LLMPolicyPath{{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{}}},
	}}

	require.NoError(t, LLMProvider.Normalize("1.0", a))

	assert.Empty(t, a.Spec.GlobalPolicies)
	require.Len(t, a.Spec.OperationPolicies, 1)
	assert.Equal(t, "basic-ratelimit", a.Spec.OperationPolicies[0].Name)
	require.Len(t, a.Spec.OperationPolicies[0].Paths, 1)
	assert.Equal(t, "/chat/completions", a.Spec.OperationPolicies[0].Paths[0].Path)
	assert.Empty(t, a.Spec.Policies)
}

func TestLLMProvider_Normalize_AlreadySplit_Idempotent(t *testing.T) {
	a := newProviderArtifact()
	a.Spec.GlobalPolicies = []api.Policy{{Name: "existing-global", Version: "v1"}}

	require.NoError(t, LLMProvider.Normalize("1.1", a))

	require.Len(t, a.Spec.GlobalPolicies, 1)
	assert.Equal(t, "existing-global", a.Spec.GlobalPolicies[0].Name)
	assert.Empty(t, a.Spec.OperationPolicies)
}

func TestLLMProvider_Normalize_WrongPayloadType(t *testing.T) {
	assert.Error(t, LLMProvider.Normalize("1.0", &model.MCPProxyDeploymentYAML{}))
}

// --- Steps ------------------------------------------------------------------

func TestLLMProvider_FlattenPolicies(t *testing.T) {
	a := newProviderArtifact()
	a.Spec.GlobalPolicies = []api.Policy{sampleGlobal("llm-cost-based-ratelimit")}
	a.Spec.OperationPolicies = []api.OperationPolicy{sampleOperation("basic-auth", "/chat")}

	var r translate.Report
	require.NoError(t, llmProviderFlattenPolicies(a, &r))

	assert.Nil(t, a.Spec.GlobalPolicies)
	assert.Nil(t, a.Spec.OperationPolicies)
	require.Len(t, a.Spec.Policies, 2)
	names := []string{a.Spec.Policies[0].Name, a.Spec.Policies[1].Name}
	assert.Contains(t, names, "llm-cost-based-ratelimit")
	assert.Contains(t, names, "basic-auth")
	assert.True(t, r.Empty(), "flattening is lossless")
}

func TestLLMProvider_FlattenPolicies_OrdersRateLimitBeforeCost(t *testing.T) {
	a := newProviderArtifact()
	a.Spec.GlobalPolicies = []api.Policy{sampleGlobal("llm-cost"), sampleGlobal("llm-cost-based-ratelimit")}

	require.NoError(t, llmProviderFlattenPolicies(a, nil))

	require.Len(t, a.Spec.Policies, 2)
	assert.Equal(t, "llm-cost-based-ratelimit", a.Spec.Policies[0].Name)
	assert.Equal(t, "llm-cost", a.Spec.Policies[1].Name)
}

func TestLLMProvider_UpstreamAuth(t *testing.T) {
	t.Run("none is dropped silently", func(t *testing.T) {
		a := newProviderArtifact()
		a.Spec.Upstream.Auth = apiAuth("none")
		var r translate.Report
		require.NoError(t, llmProviderUpstreamAuth(a, &r))
		assert.Nil(t, a.Spec.Upstream.Auth)
		assert.True(t, r.Empty())
	})
	t.Run("other is kept with a warning", func(t *testing.T) {
		a := newProviderArtifact()
		a.Spec.Upstream.Auth = apiAuth("other")
		var r translate.Report
		require.NoError(t, llmProviderUpstreamAuth(a, &r))
		require.NotNil(t, a.Spec.Upstream.Auth)
		require.Len(t, r.Warnings(), 1)
		assert.Equal(t, "spec.upstream.auth", r.Warnings()[0].Field)
		assert.NotContains(t, r.Warnings()[0].Msg, "secret", "the warning never carries the credential")
	})
	t.Run("api-key and nil are untouched", func(t *testing.T) {
		a := newProviderArtifact()
		a.Spec.Upstream.Auth = apiAuth("api-key")
		var r translate.Report
		require.NoError(t, llmProviderUpstreamAuth(a, &r))
		require.NotNil(t, a.Spec.Upstream.Auth)
		assert.True(t, r.Empty())

		b := newProviderArtifact()
		require.NoError(t, llmProviderUpstreamAuth(b, &r))
		assert.Nil(t, b.Spec.Upstream.Auth)
		assert.True(t, r.Empty())
	})
	t.Run("a typeless auth block is untouched", func(t *testing.T) {
		a := newProviderArtifact()
		a.Spec.Upstream.Auth = &api.UpstreamAuth{}
		var r translate.Report
		require.NoError(t, llmProviderUpstreamAuth(a, &r))
		require.NotNil(t, a.Spec.Upstream.Auth)
		assert.True(t, r.Empty())
	})
}

// --- The whole kind through the engine ----------------------------------------

func TestLLMProvider_Run_SourceAndTargetCombinations(t *testing.T) {
	newLegacy := func() *dto.LLMProviderDeploymentYAML {
		a := newProviderArtifact()
		a.Spec.Policies = []api.LLMPolicy{{
			Name:  "llm-cost-based-ratelimit",
			Paths: []api.LLMPolicyPath{{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{}}},
		}}
		return a
	}
	newSplit := func() *dto.LLMProviderDeploymentYAML {
		a := newProviderArtifact()
		a.Spec.GlobalPolicies = []api.Policy{{Name: "llm-cost-based-ratelimit", Version: "v1"}}
		return a
	}

	t.Run("source 1.0 (legacy) to 1.2.0: normalized up to split, apiVersion v1", func(t *testing.T) {
		a := newLegacy()
		rep, err := translate.Run(LLMProvider, "1.0", "1.2.0", a)
		require.NoError(t, err)
		assert.Equal(t, constants.GatewayApiVersion, a.ApiVersion)
		require.Len(t, a.Spec.GlobalPolicies, 1)
		assert.Empty(t, a.Spec.Policies)
		assert.True(t, rep.Empty())
	})
	t.Run("source 1.0 (legacy) to 1.1.0: normalized then re-flattened, apiVersion v1alpha1", func(t *testing.T) {
		a := newLegacy()
		rep, err := translate.Run(LLMProvider, "1.0", "1.1.0", a)
		require.NoError(t, err)
		assert.Equal(t, constants.GatewayApiVersionV1Alpha1, a.ApiVersion)
		assert.Nil(t, a.Spec.GlobalPolicies)
		require.Len(t, a.Spec.Policies, 1)
		assert.True(t, rep.Empty())
	})
	t.Run("source 1.1 (split) to 1.2.0: untouched", func(t *testing.T) {
		a := newSplit()
		rep, err := translate.Run(LLMProvider, "1.1", "1.2.0", a)
		require.NoError(t, err)
		assert.Equal(t, constants.GatewayApiVersion, a.ApiVersion)
		require.Len(t, a.Spec.GlobalPolicies, 1)
		assert.Empty(t, a.Spec.Policies)
		assert.True(t, rep.Empty())
	})
	t.Run("source 1.1 (split) to 1.0.0: flattened, apiVersion v1alpha1", func(t *testing.T) {
		a := newSplit()
		rep, err := translate.Run(LLMProvider, "1.1", "1.0.0", a)
		require.NoError(t, err)
		assert.Equal(t, constants.GatewayApiVersionV1Alpha1, a.ApiVersion)
		assert.Nil(t, a.Spec.GlobalPolicies)
		require.Len(t, a.Spec.Policies, 1)
		assert.True(t, rep.Empty())
	})
}

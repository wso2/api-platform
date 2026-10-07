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
)

func newProxyArtifact() *dto.LLMProxyDeploymentYAML {
	a := &dto.LLMProxyDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.LLMProxy}
	a.Spec.Provider.ID = "openai"
	return a
}

func TestLLMProxy_Normalize_LegacyFlat_FoldsIntoSplitLists(t *testing.T) {
	a := newProxyArtifact()
	a.Spec.Policies = []api.LLMPolicy{{
		Name:  "llm-cost",
		Paths: []api.LLMPolicyPath{{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{}}},
	}}

	require.NoError(t, LLMProxy.Normalize("1.0", a))

	require.Len(t, a.Spec.GlobalPolicies, 1)
	assert.Equal(t, "llm-cost", a.Spec.GlobalPolicies[0].Name)
	assert.Empty(t, a.Spec.Policies)
}

func TestLLMProxy_FlattenPolicies(t *testing.T) {
	a := newProxyArtifact()
	a.Spec.GlobalPolicies = []api.Policy{sampleGlobal("basic-ratelimit")}

	require.NoError(t, llmProxyFlattenPolicies(a, nil))

	require.Len(t, a.Spec.Policies, 1)
	assert.Equal(t, "/*", a.Spec.Policies[0].Paths[0].Path)
	assert.Nil(t, a.Spec.GlobalPolicies)
}

func TestLLMProxy_DropAdditionalProviders(t *testing.T) {
	a := newProxyArtifact()
	a.Spec.AdditionalProviders = []dto.LLMProxyDeploymentAdditionalProvider{
		{ID: "anthropic", As: "claude"},
		{ID: "mistral", As: "mistral", Transformer: &api.LLMProxyTransformer{Type: "openai-to-mistral", Version: "v1"}},
	}

	var r translate.Report
	require.NoError(t, llmProxyDropAdditionalProviders(a, &r))

	assert.Nil(t, a.Spec.AdditionalProviders)
	ws := r.Warnings()
	require.Len(t, ws, 2, "one warning per dropped provider")
	assert.Equal(t, "spec.additionalProviders", ws[0].Field)
	assert.Contains(t, ws[0].Msg, "anthropic")
	assert.Contains(t, ws[1].Msg, "mistral")

	t.Run("nothing to drop, nothing to say", func(t *testing.T) {
		b := newProxyArtifact()
		var r translate.Report
		require.NoError(t, llmProxyDropAdditionalProviders(b, &r))
		assert.True(t, r.Empty())
	})
}

func TestLLMProxy_ProviderAuth(t *testing.T) {
	a := newProxyArtifact()
	a.Spec.Provider.Auth = apiAuth("none")
	var r translate.Report
	require.NoError(t, llmProxyProviderAuth(a, &r))
	assert.Nil(t, a.Spec.Provider.Auth)
	assert.True(t, r.Empty())

	b := newProxyArtifact()
	b.Spec.Provider.Auth = apiAuth("bearer")
	require.NoError(t, llmProxyProviderAuth(b, &r))
	require.NotNil(t, b.Spec.Provider.Auth)
	require.Len(t, r.Warnings(), 1)
	assert.Equal(t, "spec.provider.auth", r.Warnings()[0].Field)
}

// The whole kind through the engine, per target gateway.
func TestLLMProxy_Run(t *testing.T) {
	newArtifact := func() *dto.LLMProxyDeploymentYAML {
		a := newProxyArtifact()
		a.Spec.GlobalPolicies = []api.Policy{sampleGlobal("basic-ratelimit")}
		a.Spec.AdditionalProviders = []dto.LLMProxyDeploymentAdditionalProvider{{ID: "anthropic", As: "claude"}}
		return a
	}

	for _, old := range []string{"1.0.0", "1.1.0"} {
		t.Run("gateway "+old+" flattens and drops the additional providers", func(t *testing.T) {
			a := newArtifact()
			rep, err := translate.Run(LLMProxy, "1.1", old, a)
			require.NoError(t, err)
			assert.Equal(t, constants.GatewayApiVersionV1Alpha1, a.ApiVersion)
			assert.Nil(t, a.Spec.GlobalPolicies)
			require.Len(t, a.Spec.Policies, 1)
			assert.Nil(t, a.Spec.AdditionalProviders)
			require.Len(t, rep.Warnings(), 1)
			assert.Equal(t, "spec.additionalProviders", rep.Warnings()[0].Field)
		})
	}

	for _, current := range []string{"1.2.0", "2026.09.24", ""} {
		t.Run("gateway "+current+" is untouched", func(t *testing.T) {
			a := newArtifact()
			rep, err := translate.Run(LLMProxy, "1.1", current, a)
			require.NoError(t, err)
			assert.Equal(t, constants.GatewayApiVersion, a.ApiVersion)
			require.Len(t, a.Spec.GlobalPolicies, 1)
			assert.Empty(t, a.Spec.Policies)
			require.Len(t, a.Spec.AdditionalProviders, 1)
			assert.True(t, rep.Empty())
		})
	}
}

func TestLLMProxy_RejectsAnotherKindsArtifact(t *testing.T) {
	_, err := translate.Run(LLMProxy, "1.1", "1.1.0", newProviderArtifact())
	assert.Error(t, err)
}

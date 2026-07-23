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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func newProviderArtifact() *dto.LLMProviderDeploymentYAML {
	return &dto.LLMProviderDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.LLMProvider,
	}
}

func newProxyArtifact() *dto.LLMProxyDeploymentYAML {
	return &dto.LLMProxyDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.LLMProxy,
	}
}

func TestNormalize_Provider_LegacyFlat_WildcardBecomesGlobal(t *testing.T) {
	artifact := newProviderArtifact()
	artifact.Spec.Policies = []api.LLMPolicy{
		{
			Name:    "basic-ratelimit",
			Version: "v1",
			Paths:   []api.LLMPolicyPath{{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"requests": 10}}},
		},
	}

	err := Normalize(constants.LLMProvider, "1.0", artifact)

	require.NoError(t, err)
	require.Len(t, artifact.Spec.GlobalPolicies, 1)
	assert.Equal(t, "basic-ratelimit", artifact.Spec.GlobalPolicies[0].Name)
	assert.Equal(t, map[string]interface{}{"requests": 10}, *artifact.Spec.GlobalPolicies[0].Params)
	assert.Empty(t, artifact.Spec.OperationPolicies)
	assert.Empty(t, artifact.Spec.Policies)
}

func TestNormalize_Provider_LegacyFlat_SpecificPathBecomesOperation(t *testing.T) {
	artifact := newProviderArtifact()
	artifact.Spec.Policies = []api.LLMPolicy{
		{
			Name:    "basic-ratelimit",
			Version: "v1",
			Paths:   []api.LLMPolicyPath{{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{}}},
		},
	}

	err := Normalize(constants.LLMProvider, "1.0", artifact)

	require.NoError(t, err)
	assert.Empty(t, artifact.Spec.GlobalPolicies)
	require.Len(t, artifact.Spec.OperationPolicies, 1)
	assert.Equal(t, "basic-ratelimit", artifact.Spec.OperationPolicies[0].Name)
	require.Len(t, artifact.Spec.OperationPolicies[0].Paths, 1)
	assert.Equal(t, "/chat/completions", artifact.Spec.OperationPolicies[0].Paths[0].Path)
	assert.Empty(t, artifact.Spec.Policies)
}

func TestNormalize_Provider_AlreadySplit_Idempotent(t *testing.T) {
	artifact := newProviderArtifact()
	artifact.Spec.GlobalPolicies = []api.Policy{{Name: "existing-global", Version: "v1"}}

	err := Normalize(constants.LLMProvider, "1.1", artifact)

	require.NoError(t, err)
	require.Len(t, artifact.Spec.GlobalPolicies, 1)
	assert.Equal(t, "existing-global", artifact.Spec.GlobalPolicies[0].Name)
	assert.Empty(t, artifact.Spec.OperationPolicies)
}

func TestNormalize_Proxy_LegacyFlat_FoldsIntoSplitLists(t *testing.T) {
	artifact := newProxyArtifact()
	artifact.Spec.Policies = []api.LLMPolicy{
		{
			Name:  "llm-cost",
			Paths: []api.LLMPolicyPath{{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{}}},
		},
	}

	err := Normalize(constants.LLMProxy, "1.0", artifact)

	require.NoError(t, err)
	require.Len(t, artifact.Spec.GlobalPolicies, 1)
	assert.Equal(t, "llm-cost", artifact.Spec.GlobalPolicies[0].Name)
	assert.Empty(t, artifact.Spec.Policies)
}

func TestNormalize_NonLLMKinds_Identity(t *testing.T) {
	for _, kind := range []string{constants.RestApi, constants.WebSubApi, constants.WebBrokerApi, constants.MCPProxy, "SomeFutureKind"} {
		t.Run(kind, func(t *testing.T) {
			artifact := &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: kind}
			err := Normalize(kind, "1.0", artifact)
			require.NoError(t, err)
			assert.Equal(t, constants.GatewayApiVersion, artifact.ApiVersion)
		})
	}
}

func TestNormalize_WrongPayloadType_ReturnsError(t *testing.T) {
	err := Normalize(constants.LLMProvider, "1.0", &model.MCPProxyDeploymentYAML{})
	assert.Error(t, err)
}

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

package gatewaytranslator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// TestTranslate_LLMProvider_AllSourceTargetCombinations covers gateway 1.2.0
// (v1) and 1.1.0 (v1alpha1) against a legacy (1.0, flat policies) and current
// (1.1, split policies) stored source.
func TestTranslate_LLMProvider_AllSourceTargetCombinations(t *testing.T) {
	newLegacyArtifact := func() *dto.LLMProviderDeploymentYAML {
		a := &dto.LLMProviderDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.LLMProvider}
		a.Spec.Policies = []api.LLMPolicy{{
			Name:  "llm-cost-based-ratelimit",
			Paths: []api.LLMPolicyPath{{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{}}},
		}}
		return a
	}
	newSplitArtifact := func() *dto.LLMProviderDeploymentYAML {
		a := &dto.LLMProviderDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.LLMProvider}
		a.Spec.GlobalPolicies = []api.Policy{{Name: "llm-cost-based-ratelimit", Version: "v1"}}
		return a
	}

	t.Run("source 1.0 (legacy) target v1 (1.2.0): normalized up to split, apiVersion v1", func(t *testing.T) {
		artifact := newLegacyArtifact()
		err := Translate(constants.LLMProvider, "1.0", TargetGatewayDataVersion(ParseVersion("1.2.0")), artifact)
		require.NoError(t, err)
		assert.Equal(t, constants.GatewayApiVersion, artifact.ApiVersion)
		require.Len(t, artifact.Spec.GlobalPolicies, 1)
		assert.Equal(t, "llm-cost-based-ratelimit", artifact.Spec.GlobalPolicies[0].Name)
		assert.Empty(t, artifact.Spec.Policies)
	})

	t.Run("source 1.0 (legacy) target v1alpha1 (1.1.0): normalized then re-flattened, apiVersion v1alpha1", func(t *testing.T) {
		artifact := newLegacyArtifact()
		err := Translate(constants.LLMProvider, "1.0", TargetGatewayDataVersion(ParseVersion("1.1.0")), artifact)
		require.NoError(t, err)
		assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
		assert.Nil(t, artifact.Spec.GlobalPolicies)
		require.Len(t, artifact.Spec.Policies, 1)
		assert.Equal(t, "llm-cost-based-ratelimit", artifact.Spec.Policies[0].Name)
	})

	t.Run("source 1.1 (split) target v1 (1.2.0): untouched, apiVersion v1", func(t *testing.T) {
		artifact := newSplitArtifact()
		err := Translate(constants.LLMProvider, "1.1", TargetGatewayDataVersion(ParseVersion("1.2.0")), artifact)
		require.NoError(t, err)
		assert.Equal(t, constants.GatewayApiVersion, artifact.ApiVersion)
		require.Len(t, artifact.Spec.GlobalPolicies, 1)
		assert.Empty(t, artifact.Spec.Policies)
	})

	t.Run("source 1.1 (split) target v1alpha1 (1.1.0): flattened, apiVersion v1alpha1", func(t *testing.T) {
		artifact := newSplitArtifact()
		err := Translate(constants.LLMProvider, "1.1", TargetGatewayDataVersion(ParseVersion("1.1.0")), artifact)
		require.NoError(t, err)
		assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
		assert.Nil(t, artifact.Spec.GlobalPolicies)
		require.Len(t, artifact.Spec.Policies, 1)
	})
}

func TestTranslate_LLMProxy_OldGatewayFlattens(t *testing.T) {
	artifact := &dto.LLMProxyDeploymentYAML{ApiVersion: constants.GatewayApiVersion}
	artifact.Spec.GlobalPolicies = []api.Policy{{Name: "basic-ratelimit", Version: "v1"}}

	err := Translate(constants.LLMProxy, "1.1", TargetGatewayDataVersion(ParseVersion("1.1.0")), artifact)

	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
	require.Len(t, artifact.Spec.Policies, 1)
}

// TestTranslate_AllKinds_PassthroughOnNewGateway_ApiVersionOnlySwapOnOld covers
// every kind × both target gateways, guarding the #2492/#2547 regression: every
// kind must flip apiVersion on an old gateway, not just LLM.
func TestTranslate_AllKinds_PassthroughOnNewGateway_ApiVersionOnlySwapOnOld(t *testing.T) {
	newArtifacts := func() map[string]interface {
		GetApiVersion() string
		SetApiVersion(string)
	} {
		return map[string]interface {
			GetApiVersion() string
			SetApiVersion(string)
		}{
			constants.RestApi:      &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.RestApi},
			constants.WebSubApi:    &model.WebSubAPIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.WebSubApi},
			constants.WebBrokerApi: &model.WebBrokerAPIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.WebBrokerApi},
			constants.MCPProxy:     &model.MCPProxyDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.MCPProxy},
		}
	}

	for kind, artifact := range newArtifacts() {
		t.Run(kind+"/target-1.2.0", func(t *testing.T) {
			err := Translate(kind, "1.0", TargetGatewayDataVersion(ParseVersion("1.2.0")), artifact)
			require.NoError(t, err)
			assert.Equal(t, constants.GatewayApiVersion, artifact.GetApiVersion())
		})
	}
	for kind, artifact := range newArtifacts() {
		t.Run(kind+"/target-1.1.0", func(t *testing.T) {
			err := Translate(kind, "1.0", TargetGatewayDataVersion(ParseVersion("1.1.0")), artifact)
			require.NoError(t, err)
			assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.GetApiVersion())
		})
	}
}

func TestTranslate_UnknownKind_StillSwapsApiVersionOnOldGateway(t *testing.T) {
	artifact := &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: "SomeFutureKind"}

	err := Translate("SomeFutureKind", "1.0", TargetGatewayDataVersion(ParseVersion("1.1.0")), artifact)

	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
}

func TestTranslate_EmptyTargetGatewayVersion_TreatedAsOld(t *testing.T) {
	artifact := &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.RestApi}

	err := Translate(constants.RestApi, "1.0", TargetGatewayDataVersion(ParseVersion("")), artifact)

	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
}

func TestTranslate_WrongPayloadType_ReturnsError(t *testing.T) {
	err := Translate(constants.LLMProvider, "1.0", GatewayDataVersionV1Alpha1, &dto.LLMProxyDeploymentYAML{})
	assert.Error(t, err)
}

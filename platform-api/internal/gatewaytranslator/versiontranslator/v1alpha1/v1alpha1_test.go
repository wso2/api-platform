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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

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

func TestDownConvert_LLMProvider_FlattensAndSwapsApiVersion(t *testing.T) {
	artifact := &dto.LLMProviderDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.LLMProvider,
	}
	artifact.Spec.GlobalPolicies = []api.Policy{sampleGlobal("llm-cost-based-ratelimit")}
	artifact.Spec.OperationPolicies = []api.OperationPolicy{sampleOperation("basic-auth", "/chat")}

	err := DownConvert(constants.LLMProvider, artifact)

	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
	assert.Nil(t, artifact.Spec.GlobalPolicies)
	assert.Nil(t, artifact.Spec.OperationPolicies)
	require.Len(t, artifact.Spec.Policies, 2)
	names := []string{artifact.Spec.Policies[0].Name, artifact.Spec.Policies[1].Name}
	assert.Contains(t, names, "llm-cost-based-ratelimit")
	assert.Contains(t, names, "basic-auth")
}

func TestDownConvert_LLMProvider_OrdersRateLimitBeforeCost(t *testing.T) {
	artifact := &dto.LLMProviderDeploymentYAML{ApiVersion: constants.GatewayApiVersion}
	artifact.Spec.GlobalPolicies = []api.Policy{sampleGlobal("llm-cost"), sampleGlobal("llm-cost-based-ratelimit")}

	err := DownConvert(constants.LLMProvider, artifact)

	require.NoError(t, err)
	require.Len(t, artifact.Spec.Policies, 2)
	assert.Equal(t, "llm-cost-based-ratelimit", artifact.Spec.Policies[0].Name)
	assert.Equal(t, "llm-cost", artifact.Spec.Policies[1].Name)
}

func TestDownConvert_LLMProxy_FlattensAndSwapsApiVersion(t *testing.T) {
	artifact := &dto.LLMProxyDeploymentYAML{ApiVersion: constants.GatewayApiVersion}
	artifact.Spec.GlobalPolicies = []api.Policy{sampleGlobal("basic-ratelimit")}

	err := DownConvert(constants.LLMProxy, artifact)

	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
	require.Len(t, artifact.Spec.Policies, 1)
	assert.Equal(t, "/*", artifact.Spec.Policies[0].Paths[0].Path)
}

func TestDownConvert_RestApi_ApiVersionOnly(t *testing.T) {
	artifact := &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.RestApi}
	artifact.Spec.Policies = []dto.Policy{{Name: "keep-me"}}

	err := DownConvert(constants.RestApi, artifact)

	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
	require.Len(t, artifact.Spec.Policies, 1)
	assert.Equal(t, "keep-me", artifact.Spec.Policies[0].Name)
}

func TestDownConvert_Mcp_ApiVersionOnly(t *testing.T) {
	artifact := &model.MCPProxyDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.MCPProxy}

	err := DownConvert(constants.MCPProxy, artifact)

	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
}

func TestDownConvert_WebSubApi_ApiVersionOnly(t *testing.T) {
	artifact := &model.WebSubAPIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.WebSubApi}

	err := DownConvert(constants.WebSubApi, artifact)

	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
}

func TestDownConvert_WebBrokerApi_ApiVersionOnly(t *testing.T) {
	artifact := &model.WebBrokerAPIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.WebBrokerApi}

	err := DownConvert(constants.WebBrokerApi, artifact)

	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
}

// TestDownConvert_UnknownKind_StillSwapsApiVersion is the #2492/#2547 regression
// guard: a kind with no registered shape handler must still get the apiVersion
// swap, so any unlisted/future kind never ships a v1 artifact to a gateway that
// only understands v1alpha1.
func TestDownConvert_UnknownKind_StillSwapsApiVersion(t *testing.T) {
	artifact := &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: "SomeFutureKind"}

	err := DownConvert("SomeFutureKind", artifact)

	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
}

func TestDownConvert_PayloadNotDeploymentArtifact_ReturnsError(t *testing.T) {
	err := DownConvert("SomeFutureKind", struct{ Foo string }{Foo: "bar"})
	assert.Error(t, err)
}

func TestDownConvert_LLMProvider_WrongPayloadType_ReturnsError(t *testing.T) {
	err := DownConvert(constants.LLMProvider, &dto.LLMProxyDeploymentYAML{})
	assert.Error(t, err)
}

// TestAllDeploymentYAMLTypes_ImplementDeploymentArtifact guards against a future
// *DeploymentYAML type being added without the GetApiVersion/SetApiVersion pair
// the default apiVersion swap depends on.
func TestAllDeploymentYAMLTypes_ImplementDeploymentArtifact(t *testing.T) {
	var artifacts = []any{
		&dto.APIDeploymentYAML{},
		&dto.LLMProviderDeploymentYAML{},
		&dto.LLMProxyDeploymentYAML{},
		&model.MCPProxyDeploymentYAML{},
		&model.WebSubAPIDeploymentYAML{},
		&model.WebBrokerAPIDeploymentYAML{},
	}
	for _, a := range artifacts {
		_, ok := a.(deploymentArtifact)
		assert.Truef(t, ok, "%T does not implement deploymentArtifact", a)
	}
}

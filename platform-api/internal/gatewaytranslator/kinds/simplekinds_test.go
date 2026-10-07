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

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/translate"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// RestAPI, WebSub, WebBroker and Agent have a single stored shape and no step
// of their own: on an older gateway the only change is the apiVersion swap,
// and their contents survive untouched.

func TestRestAPI_ApiVersionSwapOnly(t *testing.T) {
	a := &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.RestApi}
	a.Spec.Policies = []dto.Policy{{Name: "keep-me"}}

	rep, err := translate.Run(RestAPI, "1.0", "1.1.0", a)
	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, a.ApiVersion)
	require.Len(t, a.Spec.Policies, 1)
	assert.Equal(t, "keep-me", a.Spec.Policies[0].Name)
	assert.True(t, rep.Empty())

	rep, err = translate.Run(RestAPI, "1.0", "1.2.0", a)
	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, a.ApiVersion, "Run never upgrades an apiVersion; generators emit v1")
	assert.True(t, rep.Empty())
}

func TestRestAPI_RejectsAnotherKindsArtifact(t *testing.T) {
	_, err := translate.Run(RestAPI, "1.0", "1.2.0", &model.MCPProxyDeploymentYAML{})
	assert.Error(t, err)
}

func TestWebSub_ApiVersionSwapOnly(t *testing.T) {
	a := &model.WebSubAPIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.WebSubApi}
	rep, err := translate.Run(WebSub, "1.0", "1.1.0", a)
	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, a.ApiVersion)
	assert.True(t, rep.Empty())
}

func TestWebBroker_ApiVersionSwapOnly(t *testing.T) {
	a := &model.WebBrokerAPIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.WebBrokerApi}
	rep, err := translate.Run(WebBroker, "1.0", "1.2.0", a)
	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersion, a.ApiVersion)
	assert.True(t, rep.Empty())
}

func TestAgent_IsIdentityOnCurrentGateways(t *testing.T) {
	a := &model.AgentProxyDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.GatewayKindAgent,
		Spec:       model.AgentProxyDeploymentSpec{DisplayName: "Weather Agent", Version: "v1.0"},
	}
	before := *a

	rep, err := translate.Run(Agent, "1.0", "2026.09.24", a)
	require.NoError(t, err)
	assert.Equal(t, before, *a)
	assert.True(t, rep.Empty())
}

func TestAgent_RejectsAnotherKindsArtifact(t *testing.T) {
	assert.Error(t, Agent.Normalize("1.0", &dto.LLMProxyDeploymentYAML{}))
}

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

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func TestNormalize_AgentIsRegisteredUnderTheGatewayKind(t *testing.T) {
	_, ok := shapeHandlers[constants.GatewayKindAgent]
	assert.True(t, ok)
	_, ok = shapeHandlers[constants.AgentProxy]
	assert.False(t, ok, "the translator consumes gateway documents; AgentProxy is not one")
}

func TestNormalize_AgentIsIdentity(t *testing.T) {
	artifact := &model.AgentProxyDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.GatewayKindAgent,
		Spec: model.AgentProxyDeploymentSpec{
			DisplayName: "Weather Agent",
			Version:     "v1.0",
			A2A: model.AgentDeploymentA2A{
				ProtocolVersion: "1.0",
				OperationConfigs: model.AgentDeploymentOperationConfigs{
					Transports: []model.AgentDeploymentTransport{{ProtocolBinding: "JSONRPC", PathPrefix: "/rpc"}},
				},
			},
		},
	}
	before := *artifact

	require.NoError(t, Normalize(constants.GatewayKindAgent, "1.0", artifact))
	assert.Equal(t, before, *artifact)
}

func TestNormalize_AgentRejectsAnotherKindsArtifact(t *testing.T) {
	err := Normalize(constants.GatewayKindAgent, "1.0", &dto.LLMProxyDeploymentYAML{})
	assert.Error(t, err)
}

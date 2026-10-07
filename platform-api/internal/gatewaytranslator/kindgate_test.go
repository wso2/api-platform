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

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/kinds"
)

// The kind-name map (what the control plane deploys) and the kind table (what
// the translator knows) must describe exactly the same set of gateway kinds.
func TestKindTable_MatchesKindNameMap(t *testing.T) {
	for platformKind, gatewayKind := range platformToGatewayKind {
		_, ok := kinds.Lookup(gatewayKind)
		assert.Truef(t, ok, "%s (gateway %s) has no translator definition", platformKind, gatewayKind)
	}
	for gatewayKind := range kinds.All {
		_, ok := PlatformKindForGatewayKind(gatewayKind)
		assert.Truef(t, ok, "translator kind %s has no control-plane kind", gatewayKind)
	}
}

func TestMinGatewayVersionForKind(t *testing.T) {
	min, ok := MinGatewayVersionForKind(constants.GatewayKindAgent)
	require.True(t, ok)
	assert.Equal(t, gwversion.MinAgentKindGatewayVersion, min)

	min, ok = MinGatewayVersionForKind(constants.RestApi)
	require.True(t, ok)
	assert.Empty(t, min)

	_, ok = MinGatewayVersionForKind("SomeFutureKind")
	assert.False(t, ok)
}

func TestEnsureKindSupported(t *testing.T) {
	tests := []struct {
		name           string
		gatewayKind    string
		gatewayVersion string
		wantRefused    bool
	}{
		{"Agent on 1.2.0 is refused", constants.GatewayKindAgent, "1.2.0", true},
		{"Agent on 1.1.0 is refused", constants.GatewayKindAgent, "1.1.0", true},
		{"Agent on its first release is allowed", constants.GatewayKindAgent, gwversion.MinAgentKindGatewayVersion, false},
		{"Agent on a later CalVer is allowed", constants.GatewayKindAgent, "2027.01.01", false},
		{"Agent on an unversioned gateway is allowed", constants.GatewayKindAgent, "", false},
		{"Agent on a dev build is allowed", constants.GatewayKindAgent, "it-e2e", false},
		{"WebBroker on 1.1.0 is refused", constants.WebBrokerApi, "1.1.0", true},
		{"WebBroker on 1.2.0 is allowed", constants.WebBrokerApi, "1.2.0", false},
		{"REST on 1.0.0 is allowed", constants.RestApi, "1.0.0", false},
		{"MCP on 1.0.0 is allowed", constants.MCPProxy, "1.0.0", false},
		{"unknown kind passes the gate", "SomeFutureKind", "1.0.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureKindSupported(tt.gatewayKind, tt.gatewayVersion)
			if !tt.wantRefused {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.True(t, apperror.DeploymentKindUnsupportedByGateway.Is(err))
		})
	}
}

// The user sees the control-plane kind they deployed and the release they need.
func TestEnsureKindSupported_MessageNamesPlatformKindAndVersion(t *testing.T) {
	err := EnsureKindSupported(constants.GatewayKindAgent, "1.2.0")
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, constants.AgentProxy)
	assert.NotContains(t, msg, "Agent artifacts", "the gateway kind name is not what the user typed")
	assert.Contains(t, msg, gwversion.MinAgentKindGatewayVersion)
}

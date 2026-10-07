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

// TestTranslate_Matrix drives every kind through the facade for each gateway
// release the control plane supports and pins the artifact shape and the
// warnings each one gets. The per-kind details are covered in kinds/; this is
// the end-to-end contract the deploy services rely on.
func TestTranslate_Matrix(t *testing.T) {
	type artifact interface {
		GetApiVersion() string
		SetApiVersion(string)
	}
	type check struct {
		kind      string
		build     func() artifact
		warnings  map[string]int // gateway version -> expected warning count
		assertOld func(t *testing.T, a artifact)
		assertNew func(t *testing.T, a artifact)
	}

	checks := []check{
		{
			kind: constants.RestApi,
			build: func() artifact {
				return &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.RestApi}
			},
			warnings: map[string]int{"1.0.0": 0, "1.1.0": 0, "1.2.0": 0, "2026.09.24": 0},
		},
		{
			kind: constants.MCPProxy,
			build: func() artifact {
				a := &model.MCPProxyDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.MCPProxy}
				a.Spec.Upstream.URL = "https://b/api/mcp"
				a.Spec.SpecVersions = []string{"2026-07-28", "2025-11-25"}
				return a
			},
			warnings: map[string]int{"1.0.0": 1, "1.1.0": 1, "1.2.0": 1, "2026.09.24": 0},
			assertOld: func(t *testing.T, a artifact) {
				m := a.(*model.MCPProxyDeploymentYAML)
				assert.Equal(t, "https://b/api", m.Spec.Upstream.URL)
				assert.Equal(t, "2025-11-25", m.Spec.SpecVersion)
				assert.Nil(t, m.Spec.SpecVersions)
			},
			assertNew: func(t *testing.T, a artifact) {
				m := a.(*model.MCPProxyDeploymentYAML)
				assert.Equal(t, "https://b/api/mcp", m.Spec.Upstream.URL)
			},
		},
		{
			kind: constants.LLMProvider,
			build: func() artifact {
				a := &dto.LLMProviderDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.LLMProvider}
				a.Spec.GlobalPolicies = []api.Policy{{Name: "llm-cost-based-ratelimit", Version: "v1"}}
				return a
			},
			warnings: map[string]int{"1.0.0": 0, "1.1.0": 0, "1.2.0": 0, "2026.09.24": 0},
			assertOld: func(t *testing.T, a artifact) {
				p := a.(*dto.LLMProviderDeploymentYAML)
				assert.Nil(t, p.Spec.GlobalPolicies)
				assert.Len(t, p.Spec.Policies, 1)
			},
			assertNew: func(t *testing.T, a artifact) {
				p := a.(*dto.LLMProviderDeploymentYAML)
				assert.Len(t, p.Spec.GlobalPolicies, 1)
				assert.Empty(t, p.Spec.Policies)
			},
		},
		{
			kind: constants.LLMProxy,
			build: func() artifact {
				a := &dto.LLMProxyDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.LLMProxy}
				a.Spec.GlobalPolicies = []api.Policy{{Name: "basic-ratelimit", Version: "v1"}}
				a.Spec.AdditionalProviders = []dto.LLMProxyDeploymentAdditionalProvider{{ID: "anthropic", As: "claude"}}
				return a
			},
			warnings: map[string]int{"1.0.0": 1, "1.1.0": 1, "1.2.0": 0, "2026.09.24": 0},
			assertOld: func(t *testing.T, a artifact) {
				p := a.(*dto.LLMProxyDeploymentYAML)
				assert.Nil(t, p.Spec.AdditionalProviders)
				assert.Len(t, p.Spec.Policies, 1)
			},
			assertNew: func(t *testing.T, a artifact) {
				p := a.(*dto.LLMProxyDeploymentYAML)
				assert.Len(t, p.Spec.AdditionalProviders, 1)
			},
		},
		{
			kind: constants.WebSubApi,
			build: func() artifact {
				return &model.WebSubAPIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.WebSubApi}
			},
			warnings: map[string]int{"1.0.0": 0, "1.1.0": 0, "1.2.0": 0, "2026.09.24": 0},
		},
	}

	for _, c := range checks {
		for _, gw := range []string{"1.0.0", "1.1.0", "1.2.0", "2026.09.24"} {
			t.Run(c.kind+"/gateway-"+gw, func(t *testing.T) {
				a := c.build()
				rep, err := Translate(c.kind, "1.1", gw, a)
				require.NoError(t, err)
				assert.Len(t, rep.Warnings(), c.warnings[gw])
				if GatewayDataVersionForGateway(gw) == GatewayDataVersionV1Alpha1 {
					assert.Equal(t, constants.GatewayApiVersionV1Alpha1, a.GetApiVersion())
					if c.assertOld != nil {
						c.assertOld(t, a)
					}
				} else {
					assert.Equal(t, constants.GatewayApiVersion, a.GetApiVersion())
					if c.assertNew != nil {
						c.assertNew(t, a)
					}
				}
			})
		}
	}
}

// A blank gateway version is a current build: the artifact keeps v1. This is
// the regression guard for gateways that registered without a version or
// report a non-semver dev tag.
func TestTranslate_BlankGatewayVersionIsCurrent(t *testing.T) {
	for _, raw := range []string{"", "   ", "it-e2e"} {
		a := &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.RestApi}
		rep, err := Translate(constants.RestApi, "1.0", raw, a)
		require.NoError(t, err)
		assert.Equal(t, constants.GatewayApiVersion, a.ApiVersion)
		assert.True(t, rep.Empty())
	}
}

// Every deployable kind has a definition, so a kind without one is a
// programming error rather than something to pass through.
func TestTranslate_UnknownKindIsAnError(t *testing.T) {
	a := &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: "SomeFutureKind"}
	_, err := Translate("SomeFutureKind", "1.0", "1.1.0", a)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SomeFutureKind")
}

func TestTranslate_WrongPayloadType_ReturnsError(t *testing.T) {
	_, err := Translate(constants.LLMProvider, "1.0", "1.1.0", &dto.LLMProxyDeploymentYAML{})
	assert.Error(t, err)
}

// Agent proxies translate under their gateway kind.
func TestTranslate_AgentUnderGatewayKind(t *testing.T) {
	a := &model.AgentProxyDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.GatewayKindAgent}
	rep, err := Translate(constants.GatewayKindAgent, "1.0", "2026.09.24", a)
	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersion, a.ApiVersion)
	assert.True(t, rep.Empty())

	_, err = Translate(constants.AgentProxy, "1.0", "2026.09.24", a)
	assert.Error(t, err, "AgentProxy is the control-plane name, not a gateway kind")
}

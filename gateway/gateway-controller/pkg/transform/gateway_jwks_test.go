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

package transform

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func jwksDefinitions() map[string]models.PolicyDefinition {
	return map[string]models.PolicyDefinition{
		constants.JWKS_SYSTEM_POLICY_NAME + "_v1.0.0": {Name: constants.JWKS_SYSTEM_POLICY_NAME, Version: "v1.0.0"},
	}
}

func TestGatewayJWKSRuntimeConfig(t *testing.T) {
	rdc, ok := GatewayJWKSRuntimeConfig(&config.Config{}, jwksDefinitions())
	require.True(t, ok)

	require.Len(t, rdc.Routes, 1)
	r := rdc.Routes[constants.GatewayJWKSRouteKey]
	require.NotNil(t, r, "the route must be keyed by the same name Envoy gives it")
	assert.Equal(t, constants.GatewayJWKSPath, r.Path)
	assert.Equal(t, constants.GatewayInternalSinkClusterName, r.Upstream.ClusterKey)
	assert.Empty(t, rdc.UpstreamClusters, "the route must have no upstream to forward to")
	assert.Equal(t, models.RouteKeyResolverName, rdc.PolicyChainResolver)

	chain := rdc.PolicyChains[constants.GatewayJWKSRouteKey]
	require.NotNil(t, chain)
	last := chain.Policies[len(chain.Policies)-1]
	assert.Equal(t, constants.JWKS_SYSTEM_POLICY_NAME, last.Name)
	assert.Equal(t, "v1", last.Version)
}

func TestGatewayJWKSRuntimeConfig_InjectsSystemPoliciesFirst(t *testing.T) {
	cfg := &config.Config{}
	cfg.Analytics.Enabled = true // enables the collector, the one injected system policy
	rdc, ok := GatewayJWKSRuntimeConfig(cfg, jwksDefinitions())
	require.True(t, ok)

	policies := rdc.PolicyChains[constants.GatewayJWKSRouteKey].Policies
	require.Len(t, policies, 2)
	assert.Equal(t, constants.ANALYTICS_SYSTEM_POLICY_NAME, policies[0].Name,
		"the collector must observe JWKS requests like any other route")
	assert.Equal(t, constants.JWKS_SYSTEM_POLICY_NAME, policies[1].Name)
}

func TestGatewayJWKSRuntimeConfig_AbsentFromBuild(t *testing.T) {
	_, ok := GatewayJWKSRuntimeConfig(&config.Config{}, map[string]models.PolicyDefinition{})
	assert.False(t, ok, "a build without the JWKS system policy must emit no route at all")
}

// The policy lives in its own module; the name is asserted against its
// definition and the build lock so a rename on either side fails here rather
// than as a "policy not found" chain in production.
func TestJWKSPolicyDefinitionAndBuildLock(t *testing.T) {
	raw, err := os.ReadFile("../../../system-policies/jwks/policy-definition.yaml")
	require.NoError(t, err)
	var definition struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &definition))
	assert.Equal(t, constants.JWKS_SYSTEM_POLICY_NAME, definition.Name)
	assert.Regexp(t, `^v\d+\.\d+\.\d+$`, definition.Version)

	raw, err = os.ReadFile("../../../system-policies/system-build-lock.yaml")
	require.NoError(t, err)
	var lock struct {
		Policies []struct {
			Name     string `yaml:"name"`
			FilePath string `yaml:"filePath"`
		} `yaml:"policies"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &lock))
	for _, entry := range lock.Policies {
		if entry.Name == constants.JWKS_SYSTEM_POLICY_NAME {
			assert.Equal(t, "./jwks", entry.FilePath)
			return
		}
	}
	t.Fatalf("%s is not listed in the system build lock", constants.JWKS_SYSTEM_POLICY_NAME)
}

func TestGatewayJWKSConflicts(t *testing.T) {
	route := func(path, vhost string) *models.Route { return &models.Route{Path: path, Vhost: vhost} }
	deployed := []*models.RuntimeDeployConfig{
		{Metadata: models.Metadata{Handle: "keys-api"}, Routes: map[string]*models.Route{
			"a": route("/.well-known/jwks", "api.example.com"),
			"b": route("/.well-known/openid-configuration", "api.example.com"),
		}},
		{Metadata: models.Metadata{Handle: "dotted"}, Routes: map[string]*models.Route{
			"a": route("/x/../.well-known/jwks", ""),
		}},
		{Metadata: models.Metadata{Handle: "nested"}, Routes: map[string]*models.Route{
			"a": route("/orders/.well-known/jwks", ""),
		}},
	}
	assert.Equal(t, []string{"dotted (default)", "keys-api (api.example.com)"}, GatewayJWKSConflicts(deployed))
	assert.Empty(t, GatewayJWKSConflicts(nil))
}

func TestGatewayJWKSRuntimeConfig_PicksLatestMajor(t *testing.T) {
	def := func(version string) (string, models.PolicyDefinition) {
		return constants.JWKS_SYSTEM_POLICY_NAME + "_" + version,
			models.PolicyDefinition{Name: constants.JWKS_SYSTEM_POLICY_NAME, Version: version}
	}
	definitions := func(versions ...string) map[string]models.PolicyDefinition {
		defs := map[string]models.PolicyDefinition{}
		for _, v := range versions {
			k, d := def(v)
			defs[k] = d
		}
		return defs
	}

	tests := []struct {
		name     string
		versions []string
		want     string
	}{
		{name: "several minors of one major", versions: []string{"v1.0.0", "v1.2.0"}, want: "v1"},
		{name: "a newer major present", versions: []string{"v1.0.0", "v1.2.0", "v2.1.0"}, want: "v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdc, ok := GatewayJWKSRuntimeConfig(&config.Config{}, definitions(tt.versions...))
			require.True(t, ok)

			policies := rdc.PolicyChains[constants.GatewayJWKSRouteKey].Policies
			last := policies[len(policies)-1]
			assert.Equal(t, constants.JWKS_SYSTEM_POLICY_NAME, last.Name)
			assert.Equal(t, tt.want, last.Version)
		})
	}
}

func TestGatewayJWKSRuntimeConfig_RouteAndMetadata(t *testing.T) {
	rdc, ok := GatewayJWKSRuntimeConfig(&config.Config{}, jwksDefinitions())
	require.True(t, ok)

	r := rdc.Routes[constants.GatewayJWKSRouteKey]
	require.NotNil(t, r)
	assert.Equal(t, "GET", r.Method)
	assert.Equal(t, "*", r.Vhost)
	assert.Equal(t, constants.GatewayJWKSPath, r.OperationPath)

	assert.Equal(t, GatewayJWKSKind, rdc.Metadata.Kind)
	assert.Equal(t, "gateway-jwks", rdc.Metadata.Handle)
	assert.Equal(t, "gateway-jwks", rdc.Metadata.UUID)
}

func TestGatewayJWKSConflicts_PathAndRouteShapes(t *testing.T) {
	route := func(method, path, vhost string) *models.Route {
		return &models.Route{Method: method, Path: path, Vhost: vhost}
	}

	tests := []struct {
		name     string
		deployed []*models.RuntimeDeployConfig
		want     []string
	}{
		{
			name: "path without a leading slash",
			deployed: []*models.RuntimeDeployConfig{
				{Metadata: models.Metadata{Handle: "relative"}, Routes: map[string]*models.Route{
					"a": route("GET", ".well-known/jwks", ""),
				}},
			},
			want: []string{"relative (default)"},
		},
		{
			name: "path with a trailing slash",
			deployed: []*models.RuntimeDeployConfig{
				{Metadata: models.Metadata{Handle: "trailing"}, Routes: map[string]*models.Route{
					"a": route("GET", "/.well-known/jwks/", ""),
				}},
			},
			want: []string{"trailing (default)"},
		},
		{
			name: "two methods on the same handle and vhost are reported once",
			deployed: []*models.RuntimeDeployConfig{
				{Metadata: models.Metadata{Handle: "keys-api"}, Routes: map[string]*models.Route{
					"get":  route("GET", "/.well-known/jwks", "api.example.com"),
					"head": route("HEAD", "/.well-known/jwks", "api.example.com"),
				}},
			},
			want: []string{"keys-api (api.example.com)"},
		},
		{
			name: "the same handle on two vhosts is reported per vhost",
			deployed: []*models.RuntimeDeployConfig{
				{Metadata: models.Metadata{Handle: "keys-api"}, Routes: map[string]*models.Route{
					"main":    route("GET", "/.well-known/jwks", "api.example.com"),
					"sandbox": route("GET", "/.well-known/jwks", "sandbox.example.com"),
				}},
			},
			want: []string{"keys-api (api.example.com)", "keys-api (sandbox.example.com)"},
		},
		{
			name: "a config with nil routes contributes nothing",
			deployed: []*models.RuntimeDeployConfig{
				{Metadata: models.Metadata{Handle: "empty"}},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GatewayJWKSConflicts(tt.deployed))
		})
	}
}

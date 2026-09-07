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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// graphqlUpstream builds the anonymous upstream struct api.GraphQLAPIConfigData embeds.
func graphqlUpstream(mainURL string, sandboxURL *string) struct {
	Main    api.Upstream  `json:"main" yaml:"main"`
	Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
} {
	u := struct {
		Main    api.Upstream  `json:"main" yaml:"main"`
		Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
	}{
		Main: api.Upstream{Url: ptrStr(mainURL)},
	}
	if sandboxURL != nil {
		u.Sandbox = &api.Upstream{Url: sandboxURL}
	}
	return u
}

// makeGraphQLAPIStoredConfig builds a minimal GraphQLApi StoredConfig for transformer
// tests. GraphQLAPIConfigData carries no schema field at all — the artifact never
// describes its own schema, so transformer behavior can only ever depend on
// context/upstream/policies, never on anything schema-shaped.
func makeGraphQLAPIStoredConfig(sandboxURL *string, policies []api.Policy) *models.StoredConfig {
	var specPolicies *[]api.Policy
	if policies != nil {
		specPolicies = &policies
	}

	spec := api.GraphQLAPIConfigData{
		DisplayName: "Countries GraphQL API",
		Context:     "/countries/$version",
		Version:     "v1.0",
		Upstream:    graphqlUpstream("http://backend:8080/graphql", sandboxURL),
		Policies:    specPolicies,
	}

	graphqlAPI := api.GraphQLAPI{
		Kind:     api.GraphQLAPIKindGraphQLApi,
		Metadata: api.Metadata{Name: "countries-graphql-api"},
		Spec:     spec,
	}

	return &models.StoredConfig{
		UUID:          "countries-graphql-api",
		Kind:          "GraphQLApi",
		Configuration: graphqlAPI,
	}
}

func TestGraphQLAPITransformer_SingleRoute(t *testing.T) {
	transformer := NewGraphQLAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})

	cfg := makeGraphQLAPIStoredConfig(nil, nil)
	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	assert.Len(t, rdc.Routes, 1)
}

func TestGraphQLAPITransformer_RouteShape(t *testing.T) {
	transformer := NewGraphQLAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeGraphQLAPIStoredConfig(nil, nil)

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	routeKey := xds.GenerateRouteName("POST", "/countries/$version", "v1.0", "", "main.local")
	route, ok := rdc.Routes[routeKey]
	require.True(t, ok, "expected route keyed %q, got keys %v", routeKey, keysOf(rdc.Routes))

	assert.Equal(t, "POST", route.Method)
	assert.Equal(t, "/countries/v1.0", route.Path)
	assert.Equal(t, "Exact", route.PathMatchType)
	assert.Equal(t, "main.local", route.Vhost)
	assert.NotEmpty(t, route.Upstream.ClusterKey)
	require.NotNil(t, route.Upstream.Default)
	assert.Equal(t, "http://backend:8080", route.Upstream.Default.URL)
}

func TestGraphQLAPITransformer_PolicyChainResolver(t *testing.T) {
	transformer := NewGraphQLAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeGraphQLAPIStoredConfig(nil, nil)

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	assert.Equal(t, "route-key", rdc.PolicyChainResolver)
}

func TestGraphQLAPITransformer_SandboxProducesSecondRoute(t *testing.T) {
	transformer := NewGraphQLAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeGraphQLAPIStoredConfig(ptrStr("http://sandbox-backend:8080/graphql"), nil)

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	assert.Len(t, rdc.Routes, 2)

	mainRouteKey := xds.GenerateRouteName("POST", "/countries/$version", "v1.0", "", "main.local")
	sandboxRouteKey := xds.GenerateRouteName("POST", "/countries/$version", "v1.0", "", "sandbox.local")

	mainRoute, ok := rdc.Routes[mainRouteKey]
	require.True(t, ok)
	sandboxRoute, ok := rdc.Routes[sandboxRouteKey]
	require.True(t, ok)

	assert.NotEqual(t, mainRoute.Upstream.ClusterKey, sandboxRoute.Upstream.ClusterKey)
	require.NotNil(t, sandboxRoute.Upstream.Default)
	assert.Equal(t, "http://sandbox-backend:8080", sandboxRoute.Upstream.Default.URL)

	// Both routes get the same (API-level) policy chain.
	require.Contains(t, rdc.PolicyChains, mainRouteKey)
	require.Contains(t, rdc.PolicyChains, sandboxRouteKey)
}

func TestGraphQLAPITransformer_NoOperationsLoop(t *testing.T) {
	// A GraphQLAPIConfigData has no Operations field at all (unlike api.APIConfigData) —
	// this test documents that expectation by confirming route count tracks upstream
	// slots (1 or 2), never anything resembling an operation count.
	transformer := NewGraphQLAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeGraphQLAPIStoredConfig(nil, nil)

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	assert.Len(t, rdc.Routes, 1)
}

func TestGraphQLAPITransformer_WrongConfigurationType(t *testing.T) {
	transformer := NewGraphQLAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := &models.StoredConfig{
		UUID:          "bad-config",
		Kind:          "GraphQLApi",
		Configuration: api.RestAPI{}, // wrong type on purpose
	}

	_, err := transformer.Transform(cfg)
	assert.Error(t, err)
}

func keysOf(m map[string]*models.Route) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

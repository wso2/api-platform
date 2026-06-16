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
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils/clusterkey"
)

// ptrStr is a helper to get a pointer to a string literal.
func ptrStr(s string) *string { return &s }

// testRouterCfg returns a minimal RouterConfig for transformer tests.
func testRouterCfg() *config.RouterConfig {
	return &config.RouterConfig{
		GatewayHost: "gw.local",
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "main.local"},
			Sandbox: config.VHostEntry{Default: "sandbox.local"},
		},
	}
}

// makeRestAPIStoredConfig builds a minimal RestAPI StoredConfig for transformer tests.
func makeRestAPIStoredConfig(apiPolicies []api.Policy, opPolicies []api.Policy) *models.StoredConfig {
	var specPolicies *[]api.Policy
	if apiPolicies != nil {
		specPolicies = &apiPolicies
	}

	var opPols *[]api.Policy
	if opPolicies != nil {
		opPols = &opPolicies
	}

	apiData := api.APIConfigData{
		DisplayName: "Test API",
		Context:     "/test",
		Version:     "1.0.0",
		Operations: []api.Operation{
			{Method: "GET", Path: "/hello", Policies: opPols},
		},
		Policies: specPolicies,
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{Url: ptrStr("http://backend:8080")},
		},
	}

	restAPI := api.RestAPI{
		Kind:     api.RestAPIKindRestApi,
		Metadata: api.Metadata{Name: "test-api"},
		Spec:     apiData,
	}

	return &models.StoredConfig{
		UUID:          "test-api",
		Kind:          string(api.RestAPIKindRestApi),
		Configuration: restAPI,
	}
}

// findPolicyInChain returns true if any policy chain for the given route key
// contains a policy with the given name.
func findPolicyInChain(rdc *models.RuntimeDeployConfig, routeKey, policyName string) bool {
	chain, ok := rdc.PolicyChains[routeKey]
	if !ok {
		return false
	}
	for _, p := range chain.Policies {
		if p.Name == policyName {
			return true
		}
	}
	return false
}

// TestRestAPITransformer_APILevelEmptyVersionResolvesToLatest verifies that an API-level
// policy with an empty version is resolved via the pre-computed latestVersions index
// and included in the policy chain for every operation.
func TestRestAPITransformer_APILevelEmptyVersionResolvesToLatest(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"header-mutate|v1.0.0": {Name: "header-mutate", Version: "v1.0.0"},
		"header-mutate|v2.0.0": {Name: "header-mutate", Version: "v2.0.0"},
	}

	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
	cfg := makeRestAPIStoredConfig(
		[]api.Policy{{Name: "header-mutate", Version: ""}}, // empty version
		nil,
	)

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	require.NotNil(t, rdc)

	// Route key for GET /hello on the main vhost
	routeKey := "GET|/test/hello|main.local"
	assert.True(t, findPolicyInChain(rdc, routeKey, "header-mutate"),
		"expected header-mutate to be in policy chain when empty version is specified")
}

// TestRestAPITransformer_OperationLevelEmptyVersionResolvesToLatest verifies that an
// operation-level policy with an empty version is resolved via the pre-computed index
// and included in the policy chain.
func TestRestAPITransformer_OperationLevelEmptyVersionResolvesToLatest(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"rate-limit|v1.0.0": {Name: "rate-limit", Version: "v1.0.0"},
	}

	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
	cfg := makeRestAPIStoredConfig(
		nil,
		[]api.Policy{{Name: "rate-limit", Version: ""}}, // empty version at op level
	)

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	require.NotNil(t, rdc)

	routeKey := "GET|/test/hello|main.local"
	assert.True(t, findPolicyInChain(rdc, routeKey, "rate-limit"),
		"expected rate-limit to be in policy chain when empty version is specified at operation level")
}

// TestRestAPITransformer_UnknownPolicySkipped verifies that a policy not present in
// the definitions is silently excluded from the policy chain without causing an error.
func TestRestAPITransformer_UnknownPolicySkipped(t *testing.T) {
	defs := map[string]models.PolicyDefinition{} // empty — policy won't resolve

	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
	cfg := makeRestAPIStoredConfig(
		[]api.Policy{{Name: "unknown-policy", Version: ""}},
		nil,
	)

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	require.NotNil(t, rdc)

	routeKey := "GET|/test/hello|main.local"
	assert.False(t, findPolicyInChain(rdc, routeKey, "unknown-policy"),
		"expected unknown-policy to be excluded from the policy chain")
}

// TestRestAPITransformer_LatestVersionIndexBuiltOnConstruction verifies that the
// pre-computed index is populated when the transformer is constructed, meaning
// repeated Transform calls resolve without re-scanning definitions.
func TestRestAPITransformer_LatestVersionIndexBuiltOnConstruction(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"auth|v1.0.0": {Name: "auth", Version: "v1.0.0"},
		"auth|v3.0.0": {Name: "auth", Version: "v3.0.0"},
		"auth|v2.0.0": {Name: "auth", Version: "v2.0.0"},
	}

	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)

	// Verify the pre-computed index has the correct latest version.
	assert.Equal(t, "v3.0.0", transformer.latestVersions["auth"],
		"pre-computed index should hold the highest semver for each policy")

	// Verify Transform works correctly using the index (empty version resolves to v3.0.0).
	cfg := makeRestAPIStoredConfig(
		[]api.Policy{{Name: "auth", Version: ""}},
		nil,
	)
	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	require.NotNil(t, rdc)

	routeKey := "GET|/test/hello|main.local"
	assert.True(t, findPolicyInChain(rdc, routeKey, "auth"))
}

// TestRestAPITransformer_EmptyVersionUsesResolvedVersionInChain verifies that the
// resolved full semver (not the original empty string) is stored in the policy chain,
// so the policy engine can match it to the correct policy definition.
func TestRestAPITransformer_EmptyVersionUsesResolvedVersionInChain(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"header-mutate|v2.0.0": {Name: "header-mutate", Version: "v2.0.0"},
	}

	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
	cfg := makeRestAPIStoredConfig(
		[]api.Policy{{Name: "header-mutate", Version: ""}},
		nil,
	)

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	require.NotNil(t, rdc)

	routeKey := "GET|/test/hello|main.local"
	chain, ok := rdc.PolicyChains[routeKey]
	require.True(t, ok)
	require.Len(t, chain.Policies, 1)
	assert.Equal(t, "v2", chain.Policies[0].Version,
		"resolved major version should be stored in the chain, not the original empty string")
}

// TestSanitizeUpstreamDefinitionName verifies that dots and colons are replaced
// for Envoy cluster name compatibility.
func TestSanitizeUpstreamDefinitionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-upstream", "my-upstream"},
		{"my.upstream", "my_upstream"},
		{"my:upstream", "my_upstream"},
		{"host.example.com:8080", "host_example_com_8080"},
		{"", ""},
		{"a.b.c:d", "a_b_c_d"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeUpstreamDefinitionName(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestResolveUpstreamURL verifies URL resolution from direct URL, ref, or missing config.
func TestResolveUpstreamURL(t *testing.T) {
	refName := "my-def"
	defURL := "http://upstream-from-def:9090"

	defs := &[]api.UpstreamDefinition{
		{
			Name: refName,
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{Url: defURL},
			},
		},
	}

	t.Run("direct URL", func(t *testing.T) {
		u := "http://direct:8080"
		up := &api.Upstream{Url: &u}
		got, _, err := resolveUpstreamURL("main", up, nil)
		require.NoError(t, err)
		assert.Equal(t, u, got)
	})

	t.Run("ref to existing definition", func(t *testing.T) {
		up := &api.Upstream{Ref: &refName}
		got, _, err := resolveUpstreamURL("main", up, defs)
		require.NoError(t, err)
		assert.Equal(t, defURL, got)
	})

	t.Run("ref but no definitions provided", func(t *testing.T) {
		up := &api.Upstream{Ref: &refName}
		_, _, err := resolveUpstreamURL("main", up, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), refName)
	})

	t.Run("ref to unknown definition", func(t *testing.T) {
		unknownRef := "unknown-def"
		up := &api.Upstream{Ref: &unknownRef}
		_, _, err := resolveUpstreamURL("main", up, defs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("neither URL nor ref", func(t *testing.T) {
		up := &api.Upstream{}
		_, _, err := resolveUpstreamURL("main", up, defs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no URL or ref")
	})

	t.Run("whitespace-only URL treated as missing", func(t *testing.T) {
		blank := "   "
		up := &api.Upstream{Url: &blank}
		_, _, err := resolveUpstreamURL("main", up, nil)
		require.Error(t, err)
	})

	t.Run("def with no upstreams", func(t *testing.T) {
		emptyDefs := &[]api.UpstreamDefinition{
			{
				Name: "empty-def",
				Upstreams: []struct {
					Url    string `json:"url" yaml:"url"`
					Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
				}{},
			},
		}
		emptyRef := "empty-def"
		up := &api.Upstream{Ref: &emptyRef}
		_, _, err := resolveUpstreamURL("main", up, emptyDefs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no URLs")
	})
}

// TestResolvePort checks port resolution with explicit, default-http and default-https.
func TestResolvePort(t *testing.T) {
	tests := []struct {
		rawURL   string
		expected int
	}{
		{"http://host:9090/path", 9090},
		{"http://host/path", 80},
		{"https://host/path", 443},
		{"https://host:8443/path", 8443},
	}

	for _, tt := range tests {
		t.Run(tt.rawURL, func(t *testing.T) {
			u, err := url.Parse(tt.rawURL)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, ResolvePort(u))
		})
	}
}

// TestRestAPITransformer_SandboxRouteClusterHeader pins the sandbox route's dynamic
// cluster selection: with upstreamDefinitions the sandbox route uses cluster_header
// routing (so a dynamic-endpoint policy can divert sandbox traffic) and defaults to the
// sandbox cluster; without them it stays a static sandbox route.
func TestRestAPITransformer_SandboxRouteClusterHeader(t *testing.T) {
	defs := map[string]models.PolicyDefinition{}
	const sandboxURL = "http://sandbox-backend:9080/sandbox"
	const sandboxRouteKey = "GET|/test/hello|sandbox.local"

	t.Run("without upstreamDefinitions the sandbox route is static", func(t *testing.T) {
		transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
		cfg := makeRestAPIStoredConfig(nil, nil)
		restAPI := cfg.Configuration.(api.RestAPI)
		restAPI.Spec.Upstream.Sandbox = &api.Upstream{Url: ptrStr(sandboxURL)}
		cfg.Configuration = restAPI

		rdc, err := transformer.Transform(cfg)
		require.NoError(t, err)
		r, exists := rdc.Routes[sandboxRouteKey]
		require.True(t, exists, "sandbox route should exist")
		assert.False(t, r.Upstream.UseClusterHeader)
		assert.Equal(t, "", r.Upstream.DefaultCluster)
	})

	t.Run("with upstreamDefinitions the sandbox route uses cluster_header defaulting to the sandbox cluster", func(t *testing.T) {
		transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
		cfg := makeRestAPIStoredConfig(nil, nil)
		restAPI := cfg.Configuration.(api.RestAPI)
		restAPI.Spec.Upstream.Sandbox = &api.Upstream{Url: ptrStr(sandboxURL)}
		restAPI.Spec.UpstreamDefinitions = &[]api.UpstreamDefinition{{Name: "alt-upstream"}}
		cfg.Configuration = restAPI

		rdc, err := transformer.Transform(cfg)
		require.NoError(t, err)
		r, exists := rdc.Routes[sandboxRouteKey]
		require.True(t, exists, "sandbox route should exist")
		assert.True(t, r.Upstream.UseClusterHeader)
		assert.True(t, strings.HasPrefix(r.Upstream.DefaultCluster, "sandbox_"),
			"sandbox route must default to the URL-stable sandbox cluster (sandbox_<hash>), not main; got %q", r.Upstream.DefaultCluster)
	})
}

// makeRestAPIWithOps builds a RestAPI StoredConfig with caller-supplied operations
// and both API-level main and sandbox upstreams configured.
func makeRestAPIWithOps(ops []api.Operation) *models.StoredConfig {
	apiData := api.APIConfigData{
		DisplayName: "Test API",
		Context:     "/test",
		Version:     "1.0.0",
		Operations:  ops,
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main:    api.Upstream{Url: ptrStr("http://api-main:8080")},
			Sandbox: &api.Upstream{Url: ptrStr("http://api-sandbox:8080")},
		},
	}
	restAPI := api.RestAPI{
		Kind:     api.RestAPIKindRestApi,
		Metadata: api.Metadata{Name: "test-api"},
		Spec:     apiData,
	}
	return &models.StoredConfig{
		UUID:          "test-api",
		Kind:          string(api.RestAPIKindRestApi),
		Configuration: restAPI,
	}
}

// TestRestAPITransformer_APILevelClusterNameShape asserts the URL-stable cluster
// naming contract for API-level main and sandbox upstreams:
//   - cluster names are "<env>_<24-hex>" derived from sha256(apiID), shared by main and sandbox
//   - ClusterKey and EnvoyClusterName are the SAME string (so the policy engine's
//     default_upstream_cluster metadata resolves to a real Envoy cluster)
func TestRestAPITransformer_APILevelClusterNameShape(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{Method: "GET", Path: "/users"},
	})

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	expectedMain := "main_" + clusterkey.APILevel(cfg.UUID)
	expectedSandbox := "sandbox_" + clusterkey.APILevel(cfg.UUID)

	mainRoute := rdc.Routes["GET|/test/users|main.local"]
	require.NotNil(t, mainRoute, "main route must exist")
	assert.Equal(t, expectedMain, mainRoute.Upstream.ClusterKey,
		"main cluster name should be <env>_<hash> derived from sha256(apiID)")

	sandboxRoute := rdc.Routes["GET|/test/users|sandbox.local"]
	require.NotNil(t, sandboxRoute, "sandbox route must exist")
	assert.Equal(t, expectedSandbox, sandboxRoute.Upstream.ClusterKey,
		"sandbox cluster name should be <env>_<hash> derived from sha256(apiID)")

	_, mainExists := rdc.UpstreamClusters[expectedMain]
	require.True(t, mainExists, "main cluster %q must be registered in UpstreamClusters", expectedMain)
	_, sandboxExists := rdc.UpstreamClusters[expectedSandbox]
	require.True(t, sandboxExists, "sandbox cluster %q must be registered in UpstreamClusters", expectedSandbox)
}

// TestRestAPITransformer_APILevelDefaultClusterMatchesRealCluster verifies that
// route.Upstream.DefaultCluster matches a cluster registered in
// rdc.UpstreamClusters whenever UseClusterHeader is enabled. The policy engine
// writes DefaultCluster into the x-target-upstream header and Envoy looks up
// the cluster by that value; if the name does not match a registered cluster,
// Envoy returns 503 NoRoute.
func TestRestAPITransformer_APILevelDefaultClusterMatchesRealCluster(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{Method: "GET", Path: "/users"},
	})
	// Add an upstreamDefinition so UseClusterHeader becomes true and
	// DefaultCluster is actually populated.
	spec := cfg.Configuration.(api.RestAPI)
	spec.Spec.UpstreamDefinitions = &[]api.UpstreamDefinition{
		{
			Name: "stub-def",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{Url: "http://stub-def-svc:8080"},
			},
		},
	}
	cfg.Configuration = spec

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	mainRoute := rdc.Routes["GET|/test/users|main.local"]
	require.NotNil(t, mainRoute)
	require.True(t, mainRoute.Upstream.UseClusterHeader,
		"upstreamDefinitions present, UseClusterHeader should be true so DefaultCluster is meaningful")
	require.NotEmpty(t, mainRoute.Upstream.DefaultCluster,
		"DefaultCluster must be populated when UseClusterHeader is true")

	_, exists := rdc.UpstreamClusters[mainRoute.Upstream.DefaultCluster]
	assert.True(t, exists,
		"DefaultCluster %q must reference a real registered cluster in UpstreamClusters "+
			"(prevents 503 NoRoute when policy engine writes x-target-upstream)",
		mainRoute.Upstream.DefaultCluster)
	assert.Equal(t, mainRoute.Upstream.ClusterKey, mainRoute.Upstream.DefaultCluster,
		"DefaultCluster and ClusterKey must be the same string")
}

// TestRestAPITransformer_APILevelURLStableAcrossURLEdit asserts that editing the
// API-level main upstream URL does NOT change the cluster name. This is the
// URL-stable contract: the route keeps pointing at the same named cluster and
// name-keyed stats stay continuous across URL edits.
func TestRestAPITransformer_APILevelURLStableAcrossURLEdit(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})

	cfgA := makeRestAPIWithOps([]api.Operation{{Method: "GET", Path: "/users"}})
	rdcA, err := transformer.Transform(cfgA)
	require.NoError(t, err)

	cfgB := makeRestAPIWithOps([]api.Operation{{Method: "GET", Path: "/users"}})
	specB := cfgB.Configuration.(api.RestAPI)
	specB.Spec.Upstream.Main.Url = ptrStr("http://api-main-v2:9090")
	cfgB.Configuration = specB
	rdcB, err := transformer.Transform(cfgB)
	require.NoError(t, err)

	nameA := rdcA.Routes["GET|/test/users|main.local"].Upstream.ClusterKey
	nameB := rdcB.Routes["GET|/test/users|main.local"].Upstream.ClusterKey
	assert.Equal(t, nameA, nameB,
		"API-level main cluster name must not depend on URL "+
			"(URL-stable contract: the name must survive URL edits)")
}

// TestRestAPITransformer_APILevelMainOnlyHasNoSandboxCluster verifies that an
// API with no sandbox upstream registers no sandbox_<hash> cluster and creates
// no sandbox route. The optional env must not leave a route pointing at a
// cluster absent from UpstreamClusters (which would surface as 503 NoRoute).
func TestRestAPITransformer_APILevelMainOnlyHasNoSandboxCluster(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{Method: "GET", Path: "/users"},
	})
	spec := cfg.Configuration.(api.RestAPI)
	spec.Spec.Upstream.Sandbox = nil // main-only API
	cfg.Configuration = spec

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	expectedMain := "main_" + clusterkey.APILevel(cfg.UUID)
	expectedSandbox := "sandbox_" + clusterkey.APILevel(cfg.UUID)

	_, mainExists := rdc.UpstreamClusters[expectedMain]
	require.True(t, mainExists, "main cluster %q must still be registered", expectedMain)

	_, sandboxExists := rdc.UpstreamClusters[expectedSandbox]
	assert.False(t, sandboxExists,
		"sandbox cluster %q must not be registered when no sandbox upstream is configured", expectedSandbox)

	_, sandboxRouteExists := rdc.Routes["GET|/test/users|sandbox.local"]
	assert.False(t, sandboxRouteExists,
		"no sandbox route should exist for a main-only API")
}

// TestRestAPITransformer_ClusterNameUsesSharedHelper locks the cross-builder
// naming contract: the transform path names the cluster exactly
// clusterkey.APILevelName(env, cfg.UUID), the same helper and argument the xDS
// translator uses (pinned on that side in pkg/xds tests), so the two builders
// cannot drift to different names for the same API.
func TestRestAPITransformer_ClusterNameUsesSharedHelper(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{Method: "GET", Path: "/users"},
	})

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	assert.Equal(t, clusterkey.APILevelName("main", cfg.UUID),
		rdc.Routes["GET|/test/users|main.local"].Upstream.ClusterKey)
	assert.Equal(t, clusterkey.APILevelName("sandbox", cfg.UUID),
		rdc.Routes["GET|/test/users|sandbox.local"].Upstream.ClusterKey)
}

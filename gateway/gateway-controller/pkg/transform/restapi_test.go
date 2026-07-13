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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils/clusterkey"
)

// ptrStr is a helper to get a pointer to a string literal.
func ptrStr(s string) *string { return &s }

// opRef builds the inline per-operation upstream target holding a ref.
func opRef(ref string) *struct {
	Ref api.UpstreamReference `json:"ref" yaml:"ref"`
} {
	return &struct {
		Ref api.UpstreamReference `json:"ref" yaml:"ref"`
	}{Ref: ref}
}

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
			{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/hello"), Policies: opPols},
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

// makeRestAPIStoredConfigWithResilience builds a RestAPI StoredConfig whose single
// operation (GET /hello) carries optional API-level and operation-level resilience blocks.
func makeRestAPIStoredConfigWithResilience(apiRes, opRes *api.Resilience) *models.StoredConfig {
	apiData := api.APIConfigData{
		DisplayName: "Test API",
		Context:     "/test",
		Version:     "1.0.0",
		Resilience:  apiRes,
		Operations: []api.Operation{
			{Method: api.Ptr(api.OperationMethod("GET")), Path: ptrStr("/hello"), Resilience: opRes},
		},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{Url: ptrStr("http://backend:8080")},
		},
	}
	return &models.StoredConfig{
		UUID:          "test-api",
		Kind:          string(api.RestAPIKindRestApi),
		Configuration: api.RestAPI{Kind: api.RestAPIKindRestApi, Metadata: api.Metadata{Name: "test-api"}, Spec: apiData},
	}
}

func TestRestAPITransformer_ResiliencePrecedence(t *testing.T) {
	const routeKey = "GET|/test/hello|main.local"
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})

	t.Run("operation-level overrides API-level", func(t *testing.T) {
		cfg := makeRestAPIStoredConfigWithResilience(
			&api.Resilience{Timeout: ptrStr("10s"), IdleTimeout: ptrStr("30s")},
			&api.Resilience{Timeout: ptrStr("2s")},
		)
		rdc, err := transformer.Transform(cfg)
		require.NoError(t, err)
		rt := rdc.Routes[routeKey].Timeout
		require.NotNil(t, rt)
		require.NotNil(t, rt.Timeout)
		assert.Equal(t, 2*time.Second, *rt.Timeout, "operation timeout should win")
		require.NotNil(t, rt.IdleTimeout)
		assert.Equal(t, 30*time.Second, *rt.IdleTimeout, "idleTimeout falls back to API-level when op omits it")
	})

	t.Run("API-level applies when operation omits resilience", func(t *testing.T) {
		cfg := makeRestAPIStoredConfigWithResilience(&api.Resilience{Timeout: ptrStr("7s")}, nil)
		rdc, err := transformer.Transform(cfg)
		require.NoError(t, err)
		rt := rdc.Routes[routeKey].Timeout
		require.NotNil(t, rt)
		require.NotNil(t, rt.Timeout)
		assert.Equal(t, 7*time.Second, *rt.Timeout)
		assert.Nil(t, rt.IdleTimeout)
	})

	t.Run("no resilience leaves Timeout nil (global default applies)", func(t *testing.T) {
		cfg := makeRestAPIStoredConfigWithResilience(nil, nil)
		rdc, err := transformer.Transform(cfg)
		require.NoError(t, err)
		assert.Nil(t, rdc.Routes[routeKey].Timeout)
	})

	t.Run("0s is preserved as explicit disable", func(t *testing.T) {
		cfg := makeRestAPIStoredConfigWithResilience(&api.Resilience{Timeout: ptrStr("0s")}, nil)
		rdc, err := transformer.Transform(cfg)
		require.NoError(t, err)
		rt := rdc.Routes[routeKey].Timeout
		require.NotNil(t, rt)
		require.NotNil(t, rt.Timeout)
		assert.Equal(t, time.Duration(0), *rt.Timeout)
	})
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

// makeRestAPIWithOps builds a RestAPI StoredConfig with caller-supplied operations,
// both API-level main and sandbox upstreams configured, and a set of common
// upstreamDefinitions that per-op tests can reference by name.
func makeRestAPIWithOps(ops []api.Operation) *models.StoredConfig {
	defs := []api.UpstreamDefinition{
		{Name: "user-svc-cluster", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://user-svc:8080"}}},
		{Name: "user-svc-test-cluster", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://user-svc-test:8080"}}},
		{Name: "shared-svc-cluster", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://shared-svc:8080"}}},
		{Name: "same-svc-cluster", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://same-svc:8080"}}},
		{Name: "user-svc-cluster-v2", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://user-svc:9090"}}},
		{Name: "per-op-cluster", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://per-op-main:9090"}}},
	}
	apiData := api.APIConfigData{
		DisplayName:         "Test API",
		Context:             "/test",
		Version:             "1.0.0",
		Operations:          ops,
		UpstreamDefinitions: &defs,
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

// TestRestAPITransformer_PerOpMainOverridesMainVhost asserts that a main-only override
// causes the main vhost route to use the definition cluster while the sandbox vhost route
// falls back to the API-level sandbox cluster.
func TestRestAPITransformer_PerOpMainOverridesMainVhost(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{
			Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users"),
			Upstream: &api.OperationUpstream{
				Main: opRef("user-svc-cluster"),
			},
		},
	})

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	require.NotNil(t, rdc)

	mainRoute := rdc.Routes["GET|/test/users|main.local"]
	require.NotNil(t, mainRoute)
	assert.Equal(t, clusterkey.DefinitionName("RestApi", cfg.UUID, "user-svc-cluster"), mainRoute.Upstream.ClusterKey,
		"main vhost should use the referenced definition cluster")
	// Per-op main is dynamic: cluster_header ON with the definition cluster as the
	// default, so a dynamic-endpoint policy can still steer it while a no-policy
	// request falls back to the per-op ref.
	assert.True(t, mainRoute.Upstream.UseClusterHeader,
		"per-op main route should use cluster_header so policies can override")
	assert.Equal(t, mainRoute.Upstream.ClusterKey, mainRoute.Upstream.DefaultCluster,
		"per-op main DefaultCluster must be the definition cluster key")

	sandboxRoute := rdc.Routes["GET|/test/users|sandbox.local"]
	require.NotNil(t, sandboxRoute)
	assert.False(t, strings.HasPrefix(sandboxRoute.Upstream.ClusterKey, "upstream_"),
		"sandbox vhost should fall back to API sandbox, got %q", sandboxRoute.Upstream.ClusterKey)
}

// TestRestAPITransformer_PerOpSandboxOverridesSandboxVhost asserts that a sandbox-only override
// causes the main vhost to fall back to the API main while the sandbox vhost uses the definition cluster.
func TestRestAPITransformer_PerOpSandboxOverridesSandboxVhost(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{
			Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users"),
			Upstream: &api.OperationUpstream{
				Sandbox: opRef("user-svc-test-cluster"),
			},
		},
	})

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	mainRoute := rdc.Routes["GET|/test/users|main.local"]
	require.NotNil(t, mainRoute)
	assert.False(t, strings.HasPrefix(mainRoute.Upstream.ClusterKey, "upstream_"),
		"main vhost should fall back to API main, got %q", mainRoute.Upstream.ClusterKey)

	sandboxRoute := rdc.Routes["GET|/test/users|sandbox.local"]
	require.NotNil(t, sandboxRoute)
	assert.Equal(t, clusterkey.DefinitionName("RestApi", cfg.UUID, "user-svc-test-cluster"), sandboxRoute.Upstream.ClusterKey,
		"sandbox vhost should use the referenced definition cluster")
}

// TestRestAPITransformer_PerOpBothOverrideBothVhosts asserts that both vhosts get distinct
// definition clusters when main and sandbox are overridden.
func TestRestAPITransformer_PerOpBothOverrideBothVhosts(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{
			Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users"),
			Upstream: &api.OperationUpstream{
				Main:    opRef("user-svc-cluster"),
				Sandbox: opRef("user-svc-test-cluster"),
			},
		},
	})

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	mainRoute := rdc.Routes["GET|/test/users|main.local"]
	sandboxRoute := rdc.Routes["GET|/test/users|sandbox.local"]
	require.NotNil(t, mainRoute)
	require.NotNil(t, sandboxRoute)

	assert.Equal(t, clusterkey.DefinitionName("RestApi", cfg.UUID, "user-svc-cluster"), mainRoute.Upstream.ClusterKey,
		"main vhost should use its referenced definition cluster")
	assert.Equal(t, clusterkey.DefinitionName("RestApi", cfg.UUID, "user-svc-test-cluster"), sandboxRoute.Upstream.ClusterKey,
		"sandbox vhost should use its referenced definition cluster")
	assert.NotEqual(t, mainRoute.Upstream.ClusterKey, sandboxRoute.Upstream.ClusterKey,
		"main and sandbox per-op vhosts must produce distinct cluster keys (definition names differ)")
}

// TestRestAPITransformer_NoPerOpUsesAPILevelClusters - regression - without per-op
// upstream the routes still use the API-level main/sandbox clusters.
func TestRestAPITransformer_NoPerOpUsesAPILevelClusters(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users")},
	})

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	mainRoute := rdc.Routes["GET|/test/users|main.local"]
	sandboxRoute := rdc.Routes["GET|/test/users|sandbox.local"]
	require.NotNil(t, mainRoute)
	require.NotNil(t, sandboxRoute)
	assert.False(t, strings.HasPrefix(mainRoute.Upstream.ClusterKey, "upstream_"))
	assert.False(t, strings.HasPrefix(sandboxRoute.Upstream.ClusterKey, "upstream_"))
}

// TestRestAPITransformer_TwoOpsSameRefReuseOneCluster verifies the core reuse
// property: two operations referencing the SAME upstream definition reuse exactly
// ONE definition cluster (no per-op clusters), and both routes point at it.
func TestRestAPITransformer_TwoOpsSameRefReuseOneCluster(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users"), Upstream: &api.OperationUpstream{Main: opRef("shared-svc")}},
		{Method: api.Ptr(api.OperationMethod("POST")), Path: api.Ptr("/users"), Upstream: &api.OperationUpstream{Main: opRef("shared-svc")}},
	})
	spec := cfg.Configuration.(api.RestAPI)
	spec.Spec.UpstreamDefinitions = &[]api.UpstreamDefinition{
		{
			Name: "shared-svc",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{Url: "http://shared-svc:8080"},
			},
		},
	}
	cfg.Configuration = spec

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	getRoute := rdc.Routes["GET|/test/users|main.local"]
	postRoute := rdc.Routes["POST|/test/users|main.local"]
	require.NotNil(t, getRoute, "GET route must exist")
	require.NotNil(t, postRoute, "POST route must exist")

	// Both ops reuse the SAME definition cluster (no per-op clusters).
	assert.Equal(t, getRoute.Upstream.ClusterKey, postRoute.Upstream.ClusterKey,
		"two ops sharing a ref must reuse the same definition cluster")
	assert.True(t, strings.HasPrefix(getRoute.Upstream.ClusterKey, "upstream_"),
		"per-op route must reuse the upstream_<def> definition cluster, got %q", getRoute.Upstream.ClusterKey)

	// Exactly ONE cluster registered for shared-svc.
	shared := 0
	for k := range rdc.UpstreamClusters {
		if strings.Contains(k, "shared-svc") {
			shared++
		}
	}
	assert.Equal(t, 1, shared, "shared-svc must produce exactly one reused definition cluster")
}

// TestRestAPITransformer_PerOpClusterIsolatedAcrossAPIs asserts that two APIs with the
// same operation referencing the same definition produce different definition cluster
// keys because the API ID is part of the cluster name.
func TestRestAPITransformer_PerOpClusterIsolatedAcrossAPIs(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})

	cfgA := makeRestAPIWithOps([]api.Operation{
		{
			Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users"),
			Upstream: &api.OperationUpstream{
				Main: opRef("shared-svc-cluster"),
			},
		},
	})
	cfgA.UUID = "api-aaa"

	cfgB := makeRestAPIWithOps([]api.Operation{
		{
			Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users"),
			Upstream: &api.OperationUpstream{
				Main: opRef("shared-svc-cluster"),
			},
		},
	})
	cfgB.UUID = "api-bbb"

	rdcA, err := transformer.Transform(cfgA)
	require.NoError(t, err)
	rdcB, err := transformer.Transform(cfgB)
	require.NoError(t, err)

	var keyA, keyB string
	for k := range rdcA.UpstreamClusters {
		if strings.HasPrefix(k, "upstream_") {
			keyA = k
		}
	}
	for k := range rdcB.UpstreamClusters {
		if strings.HasPrefix(k, "upstream_") {
			keyB = k
		}
	}

	require.NotEmpty(t, keyA)
	require.NotEmpty(t, keyB)
	assert.NotEqual(t, keyA, keyB, "same URL across different APIs must produce different definition cluster keys")
}

// TestRestAPITransformer_PerOpSandboxWithoutAPILevelSandbox - guard regression.
// API-level Sandbox is nil, but one op declares a per-op sandbox upstream. The
// sandbox vhost must be created only for that op; ops without per-op sandbox
// must NOT get a sandbox route (otherwise they'd silently route to the main
// cluster on the sandbox vhost).
func TestRestAPITransformer_PerOpSandboxWithoutAPILevelSandbox(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})

	sbDefs := []api.UpstreamDefinition{
		{Name: "user-svc-cluster", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://user-svc-test:8080"}}},
	}
	apiData := api.APIConfigData{
		DisplayName:         "Test API",
		Context:             "/test",
		Version:             "1.0.0",
		UpstreamDefinitions: &sbDefs,
		Operations: []api.Operation{
			{
				Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users"),
				Upstream: &api.OperationUpstream{
					Sandbox: opRef("user-svc-cluster"),
				},
			},
			{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/orders")},
		},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main:    api.Upstream{Url: ptrStr("http://api-main:8080")},
			Sandbox: nil,
		},
	}
	cfg := &models.StoredConfig{
		UUID: "test-api",
		Kind: string(api.RestAPIKindRestApi),
		Configuration: api.RestAPI{
			Kind:     api.RestAPIKindRestApi,
			Metadata: api.Metadata{Name: "test-api"},
			Spec:     apiData,
		},
	}

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	require.NotNil(t, rdc)

	usersMain := rdc.Routes["GET|/test/users|main.local"]
	require.NotNil(t, usersMain, "op with per-op sandbox must still have a main route")
	assert.False(t, strings.HasPrefix(usersMain.Upstream.ClusterKey, "upstream_"),
		"main vhost should fall back to API main cluster, got %q", usersMain.Upstream.ClusterKey)

	usersSandbox := rdc.Routes["GET|/test/users|sandbox.local"]
	require.NotNil(t, usersSandbox, "op with per-op sandbox must have a sandbox route")
	assert.True(t, strings.HasPrefix(usersSandbox.Upstream.ClusterKey, "upstream_"),
		"sandbox vhost should use definition cluster, got %q", usersSandbox.Upstream.ClusterKey)

	ordersMain := rdc.Routes["GET|/test/orders|main.local"]
	require.NotNil(t, ordersMain, "op without per-op upstream must have a main route")
	assert.False(t, strings.HasPrefix(ordersMain.Upstream.ClusterKey, "upstream_"))

	_, ordersHasSandbox := rdc.Routes["GET|/test/orders|sandbox.local"]
	assert.False(t, ordersHasSandbox,
		"op without per-op sandbox must NOT get a sandbox route when API-level sandbox is nil")
}

// TestRestAPITransformer_PerOpSandboxInheritsSandboxHostRewrite - a per-op sandbox
// override route carries no HostRewrite of its own, so it must inherit the API-level
// SANDBOX HostRewrite (not the API-level main). This guards the transform/xDS parity:
// the xDS path inherits the sandbox value, so the RDC path must too. With API-level
// main=auto and sandbox=manual, the per-op sandbox route must be manual (AutoHostRewrite=false).
func TestRestAPITransformer_PerOpSandboxInheritsSandboxHostRewrite(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})

	manual := api.Manual
	auto := api.Auto
	defs := []api.UpstreamDefinition{
		{Name: "op-sandbox-cluster", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://op-sandbox:8080"}}},
	}
	apiData := api.APIConfigData{
		DisplayName:         "Test API",
		Context:             "/test",
		Version:             "1.0.0",
		UpstreamDefinitions: &defs,
		Operations: []api.Operation{
			{
				Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users"),
				Upstream: &api.OperationUpstream{
					Sandbox: opRef("op-sandbox-cluster"),
				},
			},
		},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main:    api.Upstream{Url: ptrStr("http://api-main:8080"), HostRewrite: &auto},
			Sandbox: &api.Upstream{Url: ptrStr("http://api-sandbox:8080"), HostRewrite: &manual},
		},
	}
	cfg := &models.StoredConfig{
		UUID: "test-api",
		Kind: string(api.RestAPIKindRestApi),
		Configuration: api.RestAPI{
			Kind:     api.RestAPIKindRestApi,
			Metadata: api.Metadata{Name: "test-api"},
			Spec:     apiData,
		},
	}

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	usersSandbox := rdc.Routes["GET|/test/users|sandbox.local"]
	require.NotNil(t, usersSandbox, "op with per-op sandbox must have a sandbox route")
	assert.True(t, strings.HasPrefix(usersSandbox.Upstream.ClusterKey, "upstream_"),
		"sandbox vhost should use definition cluster, got %q", usersSandbox.Upstream.ClusterKey)
	assert.False(t, usersSandbox.AutoHostRewrite,
		"per-op sandbox route must inherit API-level SANDBOX hostRewrite (manual), not main (auto)")

	usersMain := rdc.Routes["GET|/test/users|main.local"]
	require.NotNil(t, usersMain)
	assert.True(t, usersMain.AutoHostRewrite,
		"main route must keep API-level main hostRewrite (auto)")
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
// cluster selection: whenever a sandbox upstream is configured — with or without
// upstreamDefinitions — the sandbox route uses cluster_header routing (so a
// dynamic-endpoint policy can divert sandbox traffic) and defaults to the sandbox
// cluster. This must mirror pkg/xds/translator.go's useClusterHeader computation,
// which Envoy's actual route uses; a mismatch leaves Envoy expecting a
// x-target-upstream header the policy engine never sets.
func TestRestAPITransformer_SandboxRouteClusterHeader(t *testing.T) {
	defs := map[string]models.PolicyDefinition{}
	const sandboxURL = "http://sandbox-backend:9080/sandbox"
	const sandboxRouteKey = "GET|/test/hello|sandbox.local"

	t.Run("without upstreamDefinitions the sandbox route still uses cluster_header defaulting to the sandbox cluster", func(t *testing.T) {
		transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
		cfg := makeRestAPIStoredConfig(nil, nil)
		restAPI := cfg.Configuration.(api.RestAPI)
		restAPI.Spec.Upstream.Sandbox = &api.Upstream{Url: ptrStr(sandboxURL)}
		cfg.Configuration = restAPI

		rdc, err := transformer.Transform(cfg)
		require.NoError(t, err)
		r, exists := rdc.Routes[sandboxRouteKey]
		require.True(t, exists, "sandbox route should exist")
		assert.True(t, r.Upstream.UseClusterHeader)
		assert.True(t, strings.HasPrefix(r.Upstream.DefaultCluster, "sandbox_"),
			"sandbox route must default to the URL-stable sandbox cluster (sandbox_<hash>), not main; got %q", r.Upstream.DefaultCluster)
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

// TestRestAPITransformer_DefaultClusterReferencesRealCluster guards against the cluster-header
// fallback pointing at a non-existent cluster. translateRuntimeConfig names Envoy clusters by the
// rdc.UpstreamClusters map key, so every route's DefaultCluster (used when no policy sets the
// upstream) must be one of those keys — otherwise a route that relies on the default returns 500.
func TestRestAPITransformer_DefaultClusterReferencesRealCluster(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})

	upDefs := []api.UpstreamDefinition{{Name: "alt-upstream"}}
	apiData := api.APIConfigData{
		DisplayName:         "multi-backend",
		Context:             "/test",
		Version:             "1.0.0",
		UpstreamDefinitions: &upDefs,
		Operations:          []api.Operation{{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/hello")}},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{Url: ptrStr("http://backend:8080")},
		},
	}
	cfg := &models.StoredConfig{
		UUID:          "multi-api",
		Kind:          string(api.RestAPIKindRestApi),
		Configuration: api.RestAPI{Kind: api.RestAPIKindRestApi, Metadata: api.Metadata{Name: "multi-api"}, Spec: apiData},
	}

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	for key, r := range rdc.Routes {
		require.True(t, r.Upstream.UseClusterHeader, "route %q should use cluster_header when upstreamDefinitions are present", key)
		_, ok := rdc.UpstreamClusters[r.Upstream.DefaultCluster]
		assert.True(t, ok,
			"route %q default cluster %q must be an actual UpstreamClusters key (got keys %v)",
			key, r.Upstream.DefaultCluster, upstreamClusterKeys(rdc))
	}
}

func upstreamClusterKeys(rdc *models.RuntimeDeployConfig) []string {
	keys := make([]string, 0, len(rdc.UpstreamClusters))
	for k := range rdc.UpstreamClusters {
		keys = append(keys, k)
	}
	return keys
}

func hdrMatch(name, value string) api.OperationHeaderMatch {
	return api.OperationHeaderMatch{Name: name, Value: value}
}

// mkMatch builds an operation match block (method + path + optional ANDed header matchers).
func mkMatch(method, path string, headers ...api.OperationHeaderMatch) *api.OperationMatch {
	m := &api.OperationMatch{
		Method: api.OperationMethod(method),
		Path:   api.OperationPathMatch{Value: path},
	}
	if len(headers) > 0 {
		m.Headers = &headers
	}
	return m
}

// TestRestAPITransformer_HeaderMatchRoutesDoNotCollide verifies that operations sharing the same
// method/path/vhost but differing by header matches produce distinct routes (no map collision),
// that header-matched routes carry a 4th discriminator segment while a header-less operation keeps
// the legacy 3-segment key, and that each route's Order reflects its operation index (used as the
// Gateway-API earlier-rule-wins tie-break). This is the regression guard for the
// HTTPRouteHeaderMatching / MatchingAcrossRoutes conformance behavior.
func TestRestAPITransformer_HeaderMatchRoutesDoNotCollide(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})

	apiData := api.APIConfigData{
		DisplayName: "Header Matching API",
		Context:     "/test",
		Version:     "1.0.0",
		Operations: []api.Operation{
			{Match: mkMatch("GET", "/", hdrMatch("version", "one"))},
			{Match: mkMatch("GET", "/", hdrMatch("version", "two"))},
			{Match: mkMatch("GET", "/", hdrMatch("version", "two"), hdrMatch("color", "orange"))},
			{Match: mkMatch("GET", "/", hdrMatch("color", "blue"))},
			{Match: mkMatch("GET", "/")}, // header-less: must keep the legacy 3-segment key
		},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{Url: ptrStr("http://backend:8080")},
		},
	}
	cfg := &models.StoredConfig{
		UUID:          "hdr-api",
		Kind:          string(api.RestAPIKindRestApi),
		Configuration: api.RestAPI{Kind: api.RestAPIKindRestApi, Metadata: api.Metadata{Name: "hdr-api"}, Spec: apiData},
	}

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	require.NotNil(t, rdc)

	// One vhost, no sandbox: 5 operations must yield 5 distinct routes (no collision).
	assert.Len(t, rdc.Routes, 5, "each operation must produce its own route; collision indicates the header-match bug")
	assert.Len(t, rdc.PolicyChains, 5, "each route must have its own policy chain")

	baseKey := "GET|/test/|main.local"
	headerCount, legacyCount := 0, 0
	orders := map[int]bool{}
	for key, r := range rdc.Routes {
		orders[r.Order] = true
		segments := strings.Count(key, "|") + 1
		switch segments {
		case 3:
			assert.Equal(t, baseKey, key, "the only 3-segment key must be the header-less operation")
			legacyCount++
		case 4:
			assert.True(t, strings.HasPrefix(key, baseKey+"|"),
				"header-matched key must extend the base key with a discriminator segment")
			headerCount++
		default:
			t.Fatalf("unexpected route key segment count %d for key %q", segments, key)
		}
	}
	assert.Equal(t, 4, headerCount, "expected 4 header-matched routes")
	assert.Equal(t, 1, legacyCount, "expected exactly 1 header-less (legacy-key) route")

	// Order must be populated from the operation index 0..4.
	assert.Equal(t, map[int]bool{0: true, 1: true, 2: true, 3: true, 4: true}, orders,
		"Order must reflect operation/rule index for the earlier-rule-wins tie-break")

	// The header-less operation is index 4.
	require.Contains(t, rdc.Routes, baseKey)
	assert.Equal(t, 4, rdc.Routes[baseKey].Order)
}

// TestSplitVhosts covers the ";"-separated vhosts.main parser: single host, multiple hosts,
// surrounding whitespace, empty entries, duplicate removal, and an empty input.
func TestSplitVhosts(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", []string{}},
		{"single", "api.example.com", []string{"api.example.com"}},
		{"multi", "api.example.com;docs.example.com;*.example.com", []string{"api.example.com", "docs.example.com", "*.example.com"}},
		{"whitespace", " api.example.com ; docs.example.com ", []string{"api.example.com", "docs.example.com"}},
		{"empty entries", "api.example.com;;;docs.example.com;", []string{"api.example.com", "docs.example.com"}},
		{"dedupe preserves order", "a.com;b.com;a.com", []string{"a.com", "b.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, splitVhosts(tc.in))
		})
	}
}

// TestRestAPITransformer_SemicolonVhostsExpandToMultipleHosts verifies that a ";"-separated
// vhosts.main produces one route per production hostname (each serving the main upstream), and
// that the first entry is the primary vhost. This is the replacement for the removed vhostList.
func TestRestAPITransformer_SemicolonVhostsExpandToMultipleHosts(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})

	apiData := api.APIConfigData{
		DisplayName: "Multi VHost API",
		Context:     "/test",
		Version:     "1.0.0",
		Operations:  []api.Operation{{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/hello")}},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{Url: ptrStr("http://backend:8080")},
		},
		Vhosts: &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: "api.example.com;docs.example.com;*.example.com"},
	}
	cfg := &models.StoredConfig{
		UUID:          "multi-vhost-api",
		Kind:          string(api.RestAPIKindRestApi),
		Configuration: api.RestAPI{Kind: api.RestAPIKindRestApi, Metadata: api.Metadata{Name: "multi-vhost-api"}, Spec: apiData},
	}

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	require.NotNil(t, rdc)

	// One operation across three production vhosts -> three distinct routes, one per host.
	assert.Len(t, rdc.Routes, 3, "each production vhost must produce its own route")
	for _, host := range []string{"api.example.com", "docs.example.com", "*.example.com"} {
		key := "GET|/test/hello|" + host
		require.Contains(t, rdc.Routes, key, "expected a route on vhost %q", host)
		assert.Equal(t, host, rdc.Routes[key].Vhost)
	}
}

// TestRestAPITransformer_MixedSchemeUpstreamDefinition guards the upstream-definitions loop: a
// single Envoy cluster has one transport socket and the model carries one TLS bit for the whole
// cluster, so a weighted definition that mixes https and non-https endpoints cannot be represented
// (the plaintext endpoints would be silently dialed over TLS). The transform must reject it with a
// clear error. Uniform definitions (all https or all plaintext) must still transform, preserving the
// previous TLS-enabled result.
func TestRestAPITransformer_MixedSchemeUpstreamDefinition(t *testing.T) {
	mkUpstreams := func(urls ...string) []struct {
		Url    string `json:"url" yaml:"url"`
		Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
	} {
		ups := make([]struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}, 0, len(urls))
		for _, u := range urls {
			ups = append(ups, struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{Url: u})
		}
		return ups
	}

	mkCfg := func(urls ...string) *models.StoredConfig {
		upDefs := []api.UpstreamDefinition{{Name: "weighted", Upstreams: mkUpstreams(urls...)}}
		apiData := api.APIConfigData{
			DisplayName:         "mixed-scheme",
			Context:             "/test",
			Version:             "1.0.0",
			UpstreamDefinitions: &upDefs,
			Operations:          []api.Operation{{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/hello")}},
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{Url: ptrStr("http://backend:8080")},
			},
		}
		return &models.StoredConfig{
			UUID:          "mixed-api",
			Kind:          string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{Kind: api.RestAPIKindRestApi, Metadata: api.Metadata{Name: "mixed-api"}, Spec: apiData},
		}
	}

	weightedCluster := func(rdc *models.RuntimeDeployConfig) *models.UpstreamCluster {
		for _, uc := range rdc.UpstreamClusters {
			if uc.Name == "weighted" {
				return uc
			}
		}
		return nil
	}

	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})

	t.Run("mixed https and http is rejected", func(t *testing.T) {
		_, err := transformer.Transform(mkCfg("https://a:8443", "http://b:8080"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mixes https and non-https")
	})

	t.Run("uniform https transforms with TLS enabled", func(t *testing.T) {
		rdc, err := transformer.Transform(mkCfg("https://a:8443", "https://b:8443"))
		require.NoError(t, err)
		uc := weightedCluster(rdc)
		require.NotNil(t, uc, "weighted upstream cluster should exist")
		require.NotNil(t, uc.TLS)
		assert.True(t, uc.TLS.Enabled, "uniform-https definition must keep TLS enabled")
	})

	t.Run("uniform plaintext transforms with TLS disabled", func(t *testing.T) {
		rdc, err := transformer.Transform(mkCfg("http://a:8080", "http://b:8080"))
		require.NoError(t, err)
		uc := weightedCluster(rdc)
		require.NotNil(t, uc, "weighted upstream cluster should exist")
		require.NotNil(t, uc.TLS)
		assert.False(t, uc.TLS.Enabled, "uniform-plaintext definition must keep TLS disabled")
	})
}

// TestRestAPITransformer_SimpleAndMatchSamePathCoexist verifies the user's case: a header-less
// simple-form operation and a match-form operation with a header, both on the same method+path,
// coexist as two distinct routes (the header match gives the second a discriminator segment), so
// header precedence — not a collision — decides which serves a given request.
func TestRestAPITransformer_SimpleAndMatchSamePathCoexist(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	apiData := api.APIConfigData{
		DisplayName: "Mixed Form API",
		Context:     "/test",
		Version:     "1.0.0",
		Operations: []api.Operation{
			{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/via-match")}, // simple, header-less
			{Match: mkMatch("GET", "/via-match", hdrMatch("x-variant", "alpha"))},      // match, header-conditioned
		},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: api.Upstream{Url: ptrStr("http://backend:8080")}},
	}
	cfg := &models.StoredConfig{UUID: "mixed", Kind: string(api.RestAPIKindRestApi),
		Configuration: api.RestAPI{Kind: api.RestAPIKindRestApi, Metadata: api.Metadata{Name: "mixed"}, Spec: apiData}}

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	require.Len(t, rdc.Routes, 2, "both operations must produce distinct routes (no collision)")

	base := "GET|/test/via-match|main.local"
	require.Contains(t, rdc.Routes, base, "the simple header-less op keeps the 3-segment key")
	assert.Empty(t, rdc.Routes[base].MatchHeaders, "simple op has no header matchers")

	var headerKey string
	for k := range rdc.Routes {
		if k != base {
			headerKey = k
		}
	}
	require.True(t, strings.HasPrefix(headerKey, base+"|"), "match+header op gets a discriminator segment: %q", headerKey)
	require.Len(t, rdc.Routes[headerKey].MatchHeaders, 1, "match op carries the x-variant header matcher")
	assert.Equal(t, "x-variant", rdc.Routes[headerKey].MatchHeaders[0].Name)
}

// TestRestAPITransformer_OperationFormCombinations proves the four supported authoring shapes all
// transform without error and produce the expected routes:
//  1. all operations in the simple (method+path) form
//  2. all operations in the match form
//  3. a mix of simple and match forms on DIFFERENT routes
//  4. a mix where the SAME path is defined both ways (simple header-less + match with a header)
func TestRestAPITransformer_OperationFormCombinations(t *testing.T) {
	mk := func(ops []api.Operation) *models.RuntimeDeployConfig {
		t.Helper()
		transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
		apiData := api.APIConfigData{
			DisplayName: "Forms API", Context: "/test", Version: "1.0.0", Operations: ops,
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{Main: api.Upstream{Url: ptrStr("http://backend:8080")}},
		}
		cfg := &models.StoredConfig{UUID: "forms", Kind: string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{Kind: api.RestAPIKindRestApi, Metadata: api.Metadata{Name: "forms"}, Spec: apiData}}
		rdc, err := transformer.Transform(cfg)
		require.NoError(t, err)
		return rdc
	}
	simple := func(method, path string) api.Operation {
		return api.Operation{Method: api.Ptr(api.OperationMethod(method)), Path: api.Ptr(path)}
	}

	t.Run("1: simple form only", func(t *testing.T) {
		rdc := mk([]api.Operation{simple("GET", "/a"), simple("POST", "/b")})
		require.Len(t, rdc.Routes, 2)
		require.Contains(t, rdc.Routes, "GET|/test/a|main.local")
		require.Contains(t, rdc.Routes, "POST|/test/b|main.local")
	})

	t.Run("2: match form only", func(t *testing.T) {
		rdc := mk([]api.Operation{
			{Match: mkMatch("GET", "/a")},
			{Match: mkMatch("GET", "/b", hdrMatch("x-variant", "alpha"))},
		})
		require.Len(t, rdc.Routes, 2)
		require.Contains(t, rdc.Routes, "GET|/test/a|main.local")
	})

	t.Run("3: mix on different routes", func(t *testing.T) {
		rdc := mk([]api.Operation{
			simple("GET", "/a"),
			{Match: mkMatch("GET", "/b", hdrMatch("x-variant", "alpha"))},
		})
		require.Len(t, rdc.Routes, 2)
		require.Contains(t, rdc.Routes, "GET|/test/a|main.local")
	})

	t.Run("4: same path defined both ways (simple header-less + match with header)", func(t *testing.T) {
		rdc := mk([]api.Operation{
			simple("GET", "/c"),
			{Match: mkMatch("GET", "/c", hdrMatch("x-variant", "alpha"))},
		})
		require.Len(t, rdc.Routes, 2, "both coexist: the header match gives a distinct key")
		base := "GET|/test/c|main.local"
		require.Contains(t, rdc.Routes, base)
		require.Empty(t, rdc.Routes[base].MatchHeaders)
	})
}

// TestRestAPITransformer_ConnectTimeoutFromDefinition is the transformer half of the
// connect-timeout regression guard: an upstreamDefinition's timeout.connect must be carried
// onto the RuntimeDeployConfig clusters (both the definition cluster and an API-level cluster
// that references it) so the RDC->Envoy translation can apply it instead of dropping it.
// A definition without a timeout leaves ConnectTimeout nil (the global default applies later).
func TestRestAPITransformer_ConnectTimeoutFromDefinition(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})

	build := func(connect *string) (*models.RuntimeDeployConfig, error) {
		def := api.UpstreamDefinition{
			Name: "timed-svc",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{{Url: "http://backend:8080"}},
		}
		if connect != nil {
			def.Timeout = &api.UpstreamTimeout{Connect: connect}
		}
		apiData := api.APIConfigData{
			DisplayName:         "Timeout API",
			Context:             "/test",
			Version:             "1.0.0",
			UpstreamDefinitions: &[]api.UpstreamDefinition{def},
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{Main: api.Upstream{Ref: ptrStr("timed-svc")}},
			Operations: []api.Operation{{Method: api.Ptr(api.OperationMethod("GET")), Path: ptrStr("/hello")}},
		}
		return transformer.Transform(&models.StoredConfig{
			UUID: "timeout-api", Kind: string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{Kind: api.RestAPIKindRestApi, Metadata: api.Metadata{Name: "timeout-api"}, Spec: apiData},
		})
	}

	t.Run("definition timeout maps onto every referencing cluster", func(t *testing.T) {
		rdc, err := build(ptrStr("8s"))
		require.NoError(t, err)
		// Both the API-level ref cluster and the definition cluster reference timed-svc.
		require.GreaterOrEqual(t, len(rdc.UpstreamClusters), 2,
			"expected the API-level ref cluster and the definition cluster")
		for name, uc := range rdc.UpstreamClusters {
			require.NotNil(t, uc.ConnectTimeout, "cluster %q must carry the definition connect timeout", name)
			assert.Equal(t, 8*time.Second, *uc.ConnectTimeout, "cluster %q connect timeout", name)
		}
	})

	t.Run("no definition timeout leaves ConnectTimeout nil", func(t *testing.T) {
		rdc, err := build(nil)
		require.NoError(t, err)
		require.NotEmpty(t, rdc.UpstreamClusters)
		for name, uc := range rdc.UpstreamClusters {
			assert.Nil(t, uc.ConnectTimeout, "cluster %q must have no connect timeout so the global default applies", name)
		}
	})
}

// TestRestAPITransformer_APILevelClusterNameShape asserts the URL-stable cluster
// naming contract for API-level main and sandbox upstreams:
//   - cluster names are "<env>_<64-hex>": main and sandbox share the sha256(apiID) digest, distinguished by the env prefix
//   - ClusterKey and EnvoyClusterName are the SAME string (so the policy engine's
//     default_upstream_cluster metadata resolves to a real Envoy cluster)
func TestRestAPITransformer_APILevelClusterNameShape(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users")},
	})

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	// Expected name is hard-coded (full sha256("test-api")), not computed via
	// clusterkey.HashedName, so a change to the hashing function is caught here.
	expectedMain := "main_2a28373e2cacc6ea903d8c7e52dd3c49f8a87f95ec65ba1156de7e6564ca9524"
	expectedSandbox := "sandbox_2a28373e2cacc6ea903d8c7e52dd3c49f8a87f95ec65ba1156de7e6564ca9524"

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
// Envoy returns a cluster-not-found 503.
func TestRestAPITransformer_APILevelDefaultClusterMatchesRealCluster(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users")},
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
			"(prevents a cluster-not-found 503 when the policy engine writes x-target-upstream)",
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

	cfgA := makeRestAPIWithOps([]api.Operation{{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users")}})
	rdcA, err := transformer.Transform(cfgA)
	require.NoError(t, err)

	cfgB := makeRestAPIWithOps([]api.Operation{{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users")}})
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
// cluster absent from UpstreamClusters (which would surface as a cluster-not-found 503).
func TestRestAPITransformer_APILevelMainOnlyHasNoSandboxCluster(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users")},
	})
	spec := cfg.Configuration.(api.RestAPI)
	spec.Spec.Upstream.Sandbox = nil // main-only API
	cfg.Configuration = spec

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	// Expected name is hard-coded (full sha256("test-api")), not computed via
	// clusterkey.HashedName, so a change to the hashing function is caught here.
	expectedMain := "main_2a28373e2cacc6ea903d8c7e52dd3c49f8a87f95ec65ba1156de7e6564ca9524"
	expectedSandbox := "sandbox_2a28373e2cacc6ea903d8c7e52dd3c49f8a87f95ec65ba1156de7e6564ca9524"

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
// clusterkey.HashedName(env, cfg.UUID), the same helper and argument the xDS
// translator uses (pinned on that side in pkg/xds tests), so the two builders
// cannot drift to different names for the same API.
func TestRestAPITransformer_ClusterNameUsesSharedHelper(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users")},
	})

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	assert.Equal(t, clusterkey.HashedName("main", cfg.UUID),
		rdc.Routes["GET|/test/users|main.local"].Upstream.ClusterKey)
	assert.Equal(t, clusterkey.HashedName("sandbox", cfg.UUID),
		rdc.Routes["GET|/test/users|sandbox.local"].Upstream.ClusterKey)
}

// TestRestAPITransformer_APILevelPolicyPrecedesOperationLevelInChain pins that
// buildPolicyChain places API-level policies before operation-level ones, so an
// operation-level policy is the last write and wins over an API-level one in the kernel.
func TestRestAPITransformer_APILevelPolicyPrecedesOperationLevelInChain(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"api-pol|v1.0.0": {Name: "api-pol", Version: "v1.0.0"},
		"op-pol|v1.0.0":  {Name: "op-pol", Version: "v1.0.0"},
	}

	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
	cfg := makeRestAPIStoredConfig(
		[]api.Policy{{Name: "api-pol", Version: ""}},
		[]api.Policy{{Name: "op-pol", Version: ""}},
	)

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	require.NotNil(t, rdc)

	routeKey := "GET|/test/hello|main.local"
	chain, ok := rdc.PolicyChains[routeKey]
	require.True(t, ok)
	require.Len(t, chain.Policies, 2)
	assert.Equal(t, "api-pol", chain.Policies[0].Name,
		"API-level policy must come first in the chain")
	assert.Equal(t, "op-pol", chain.Policies[1].Name,
		"operation-level policy must come after the API-level policy so it wins as the last write in the kernel")
}

// TestRestAPITransformer_PerOpMainKeptWhenVhostsEqual pins that a per-op main override
// survives when the main and sandbox vhosts are the same string and no sandbox upstream
// exists; the route dispatch must key on the vhost's role, not its name.
func TestRestAPITransformer_PerOpMainKeptWhenVhostsEqual(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	cfg := makeRestAPIWithOps([]api.Operation{
		{
			Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/users"),
			Upstream: &api.OperationUpstream{
				Main: opRef("user-svc-cluster"),
			},
		},
	})
	restAPI := cfg.Configuration.(api.RestAPI)
	restAPI.Spec.Upstream.Sandbox = nil
	same := "same.local"
	restAPI.Spec.Vhosts = &struct {
		Main    string  `json:"main" yaml:"main"`
		Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
	}{Main: same, Sandbox: &same}
	cfg.Configuration = restAPI

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	route := rdc.Routes["GET|/test/users|same.local"]
	require.NotNil(t, route, "main route must exist")
	want := clusterkey.DefinitionName("RestApi", cfg.UUID, "user-svc-cluster")
	assert.Equal(t, want, route.Upstream.ClusterKey,
		"per-op main override must survive equal main/sandbox vhosts")
	assert.Equal(t, want, route.Upstream.DefaultCluster,
		"cluster_header default must be the per-op cluster, not the API-level one")
}

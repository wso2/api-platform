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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// makeSoapAPIStoredConfig builds a minimal SoapAPI StoredConfig for transformer tests.
func makeSoapAPIStoredConfig(sandboxURL *string, apiPolicies []api.Policy) *models.StoredConfig {
	var specPolicies *[]api.Policy
	if apiPolicies != nil {
		specPolicies = &apiPolicies
	}

	spec := api.SoapAPIData{
		DisplayName: "StockQuote API",
		Context:     "/stockquote/$version",
		Version:     "v1.0",
		Policies:    specPolicies,
	}
	spec.Upstream.Main = api.Upstream{Url: ptrStr("http://stockquote-service:8080/services/StockQuote")}
	if sandboxURL != nil {
		spec.Upstream.Sandbox = &api.Upstream{Url: sandboxURL}
	}

	return &models.StoredConfig{
		UUID:   "stockquote-v1.0",
		Kind:   models.KindSoapApi,
		Handle: "stockquote-v1.0",
		Configuration: api.SoapAPI{
			Kind:     api.SoapAPIKindSoapApi,
			Metadata: api.Metadata{Name: "stockquote-v1.0"},
			Spec:     spec,
		},
	}
}

// soapDefs returns the policy definitions a SOAP API needs — the SOAP system policy,
// which every GET route carries — plus extra.
func soapDefs(extra map[string]models.PolicyDefinition) map[string]models.PolicyDefinition {
	defs := map[string]models.PolicyDefinition{
		constants.SOAP_SYSTEM_POLICY_NAME + "|v1.0.0": {Name: constants.SOAP_SYSTEM_POLICY_NAME, Version: "v1.0.0"},
	}
	for k, v := range extra {
		defs[k] = v
	}
	return defs
}

func soapRouteKey(method, vhost string) string {
	return xds.GenerateRouteName(method, "/stockquote/$version", "v1.0", SoapWildcardPath, vhost)
}

// A SOAP API is one upstream cluster and a wildcard POST + GET route pair at the
// context, carrying the SoapApi kind.
func TestSoapAPITransformer_PassthroughRoutes(t *testing.T) {
	transformer := NewSoapAPITransformer(testRouterCfg(), &config.Config{}, soapDefs(nil))

	rdc, err := transformer.Transform(makeSoapAPIStoredConfig(nil, nil))
	require.NoError(t, err)

	assert.Equal(t, models.KindSoapApi, rdc.Metadata.Kind)
	assert.Equal(t, "/stockquote/v1.0", rdc.Context)
	assert.Equal(t, "route-key", rdc.PolicyChainResolver)
	require.Len(t, rdc.UpstreamClusters, 1)
	require.Len(t, rdc.Routes, 2)

	for _, method := range []string{"POST", "GET"} {
		key := soapRouteKey(method, "main.local")
		route, ok := rdc.Routes[key]
		require.True(t, ok, "expected route keyed %q, got keys %v", key, keysOf(rdc.Routes))
		assert.Equal(t, method, route.Method)
		assert.Equal(t, "/stockquote/v1.0/*", route.Path)
		assert.Equal(t, SoapWildcardPath, route.OperationPath)
		assert.Equal(t, "main.local", route.Vhost)
		assert.True(t, route.AutoHostRewrite)
		require.NotNil(t, route.Upstream.Default)
		assert.Equal(t, "http://stockquote-service:8080", route.Upstream.Default.URL)
		assert.Contains(t, rdc.PolicyChains, key)
	}
}

// API-level policies apply to both routes, so WSDL retrieval (GET) is protected too.
func TestSoapAPITransformer_APILevelPolicyOnBothRoutes(t *testing.T) {
	defs := soapDefs(map[string]models.PolicyDefinition{
		"api-key-auth|v1.0.0": {Name: "api-key-auth", Version: "v1.0.0"},
	})
	transformer := NewSoapAPITransformer(testRouterCfg(), &config.Config{}, defs)

	rdc, err := transformer.Transform(makeSoapAPIStoredConfig(nil, []api.Policy{{Name: "api-key-auth", Version: "v1"}}))
	require.NoError(t, err)

	assert.True(t, findPolicyInChain(rdc, soapRouteKey("POST", "main.local"), "api-key-auth"))
	assert.True(t, findPolicyInChain(rdc, soapRouteKey("GET", "main.local"), "api-key-auth"))
}

// An unresolvable policy fails the deployment instead of being dropped: the API-level
// chain is the routes' whole chain, so dropping an auth policy would expose the API.
func TestSoapAPITransformer_UnresolvablePolicyFails(t *testing.T) {
	transformer := NewSoapAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})

	_, err := transformer.Transform(makeSoapAPIStoredConfig(nil, []api.Policy{{Name: "api-key-auth", Version: "v1"}}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api-key-auth")
}

func TestSoapAPITransformer_SandboxRoutes(t *testing.T) {
	transformer := NewSoapAPITransformer(testRouterCfg(), &config.Config{}, soapDefs(nil))

	rdc, err := transformer.Transform(makeSoapAPIStoredConfig(ptrStr("http://sandbox-service:8080/services/StockQuote"), nil))
	require.NoError(t, err)
	require.Len(t, rdc.Routes, 4)
	require.Len(t, rdc.UpstreamClusters, 2)

	for _, method := range []string{"POST", "GET"} {
		mainRoute, ok := rdc.Routes[soapRouteKey(method, "main.local")]
		require.True(t, ok)
		sbRoute, ok := rdc.Routes[soapRouteKey(method, "sandbox.local")]
		require.True(t, ok)
		assert.NotEqual(t, mainRoute.Upstream.ClusterKey, sbRoute.Upstream.ClusterKey)
		require.NotNil(t, sbRoute.Upstream.Default)
		assert.Equal(t, "http://sandbox-service:8080", sbRoute.Upstream.Default.URL)
	}
}

// Custom vhosts replace the gateway defaults.
func TestSoapAPITransformer_CustomVhosts(t *testing.T) {
	transformer := NewSoapAPITransformer(testRouterCfg(), &config.Config{}, soapDefs(nil))
	cfg := makeSoapAPIStoredConfig(ptrStr("http://sandbox-service:8080"), nil)
	soapAPI := cfg.Configuration.(api.SoapAPI)
	soapAPI.Spec.Vhosts = &struct {
		Main    string  `json:"main" yaml:"main"`
		Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
	}{Main: "soap.example.com", Sandbox: ptrStr("sb.soap.example.com")}
	cfg.Configuration = soapAPI

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	assert.Contains(t, rdc.Routes, soapRouteKey("POST", "soap.example.com"))
	assert.Contains(t, rdc.Routes, soapRouteKey("POST", "sb.soap.example.com"))
	assert.NotContains(t, rdc.Routes, soapRouteKey("POST", "main.local"))
}

// A sandbox upstream on the same vhost as main would collide route keys.
func TestSoapAPITransformer_SandboxSameVhostFails(t *testing.T) {
	routerCfg := testRouterCfg()
	routerCfg.VHosts.Sandbox.Default = routerCfg.VHosts.Main.Default
	transformer := NewSoapAPITransformer(routerCfg, &config.Config{}, soapDefs(nil))

	_, err := transformer.Transform(makeSoapAPIStoredConfig(ptrStr("http://sandbox-service:8080"), nil))
	require.Error(t, err)
}

func TestSoapAPITransformer_WrongKind(t *testing.T) {
	transformer := NewSoapAPITransformer(testRouterCfg(), &config.Config{}, soapDefs(nil))
	_, err := transformer.Transform(makeRestAPIStoredConfig(nil, nil))
	require.Error(t, err)
}

// Fault policies reach both routes' fault chains, never the normal chain, with the
// collector appended last when analytics is on — the same arrangement as a RestApi
// operation.
func TestSoapAPITransformer_FaultPolicies(t *testing.T) {
	defs := soapDefs(map[string]models.PolicyDefinition{
		"error-response-formatter|v1.0.0": {Name: "error-response-formatter", Version: "v1.0.0"},
	})
	sysCfg := &config.Config{Analytics: config.AnalyticsConfig{Enabled: true}}
	transformer := NewSoapAPITransformer(testRouterCfg(), sysCfg, defs)

	cfg := makeSoapAPIStoredConfig(nil, nil)
	soapAPI := cfg.Configuration.(api.SoapAPI)
	soapAPI.Spec.FaultPolicies = &[]api.Policy{{Name: "error-response-formatter", Version: "v1"}}
	cfg.Configuration = soapAPI

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)
	require.Len(t, rdc.PolicyChains, 2)
	for routeKey, chain := range rdc.PolicyChains {
		assert.Equal(t, []string{"error-response-formatter", constants.ANALYTICS_SYSTEM_POLICY_NAME}, faultNames(chain), routeKey)
		for _, p := range chain.Policies {
			assert.NotEqual(t, "error-response-formatter", p.Name, "fault policy leaked into the normal chain of %s", routeKey)
		}
	}
}

// Without fault policies of its own, a SOAP route still records its failures when
// analytics is on, and has an empty fault chain when it is off.
func TestSoapAPITransformer_NoFaultPolicies(t *testing.T) {
	on := NewSoapAPITransformer(testRouterCfg(), &config.Config{Analytics: config.AnalyticsConfig{Enabled: true}}, soapDefs(nil))
	rdc, err := on.Transform(makeSoapAPIStoredConfig(nil, nil))
	require.NoError(t, err)
	for routeKey, chain := range rdc.PolicyChains {
		assert.Equal(t, []string{constants.ANALYTICS_SYSTEM_POLICY_NAME}, faultNames(chain), routeKey)
	}

	off := NewSoapAPITransformer(testRouterCfg(), &config.Config{}, soapDefs(nil))
	rdc, err = off.Transform(makeSoapAPIStoredConfig(nil, nil))
	require.NoError(t, err)
	for _, chain := range rdc.PolicyChains {
		assert.Empty(t, chain.FaultPolicies)
	}
}

// The GET route ends with the SOAP system policy, configured for that route's vhost and
// upstream; the POST route does not carry it.
func TestSoapAPITransformer_WSDLRewriteOnGetRoutes(t *testing.T) {
	routerCfg := testRouterCfg()
	routerCfg.HTTPListener.PerConnectionBufferLimitBytes = 2 << 20
	transformer := NewSoapAPITransformer(routerCfg, &config.Config{}, soapDefs(nil))
	rdc, err := transformer.Transform(makeSoapAPIStoredConfig(ptrStr("http://sandbox-service:8080/sb/StockQuote"), nil))
	require.NoError(t, err)

	cases := []struct {
		vhost, upstream string
	}{
		{"main.local", "http://stockquote-service:8080/services/StockQuote"},
		{"sandbox.local", "http://sandbox-service:8080/sb/StockQuote"},
	}
	for _, tc := range cases {
		get := rdc.PolicyChains[soapRouteKey("GET", tc.vhost)]
		require.NotNil(t, get)
		require.NotEmpty(t, get.Policies)
		last := get.Policies[len(get.Policies)-1]
		assert.Equal(t, constants.SOAP_SYSTEM_POLICY_NAME, last.Name)
		assert.Equal(t, "v1", last.Version)
		assert.Equal(t, map[string]interface{}{
			"gatewayPath":      "/stockquote/v1.0",
			"upstreamUrl":      tc.upstream,
			"vhost":            tc.vhost,
			"maxResponseBytes": 2 << 20,
		}, last.Params["wsdlRewrite"])

		post := rdc.PolicyChains[soapRouteKey("POST", tc.vhost)]
		for _, p := range post.Policies {
			assert.NotEqual(t, constants.SOAP_SYSTEM_POLICY_NAME, p.Name, "the POST route must not buffer its response for the rewrite")
		}
	}
}

// The rewrite runs after the author's response policies are declared, i.e. it is last in
// the chain and so first in the response phase.
func TestSoapAPITransformer_WSDLRewriteIsLastInTheChain(t *testing.T) {
	defs := soapDefs(map[string]models.PolicyDefinition{"set-headers|v1.0.0": {Name: "set-headers", Version: "v1.0.0"}})
	transformer := NewSoapAPITransformer(testRouterCfg(), &config.Config{}, defs)
	rdc, err := transformer.Transform(makeSoapAPIStoredConfig(nil, []api.Policy{{Name: "set-headers", Version: "v1"}}))
	require.NoError(t, err)

	get := rdc.PolicyChains[soapRouteKey("GET", "main.local")]
	require.Len(t, get.Policies, 2)
	assert.Equal(t, "set-headers", get.Policies[0].Name)
	assert.Equal(t, constants.SOAP_SYSTEM_POLICY_NAME, get.Policies[1].Name)
}

// Without the system policy's definition the deployment fails rather than serving the
// backend's own addresses.
func TestSoapAPITransformer_MissingSystemPolicyFails(t *testing.T) {
	transformer := NewSoapAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
	_, err := transformer.Transform(makeSoapAPIStoredConfig(nil, nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), constants.SOAP_SYSTEM_POLICY_NAME)
}

// The SOAP system policy module spells the same names; see its
// TestParameterNamesMatchTheController.
func TestSoapSystemPolicyLiterals(t *testing.T) {
	assert.Equal(t, "wso2_apip_sys_soap", constants.SOAP_SYSTEM_POLICY_NAME)
	assert.Equal(t, "wsdlRewrite", constants.SOAP_POLICY_PARAM_WSDL_REWRITE)
	assert.Equal(t, "gatewayPath", constants.SOAP_POLICY_PARAM_GATEWAY_PATH)
	assert.Equal(t, "upstreamUrl", constants.SOAP_POLICY_PARAM_UPSTREAM_URL)
	assert.Equal(t, "vhost", constants.SOAP_POLICY_PARAM_VHOST)
	assert.Equal(t, "maxResponseBytes", constants.SOAP_POLICY_PARAM_MAX_BYTES)
}

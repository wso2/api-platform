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

package transform_test

import (
	"io"
	"log/slog"
	"regexp"
	"testing"

	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/transform"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// soapEnvoyRoutes translates one SOAP API through the transformer and the xDS
// translator — the path every deployed SoapApi takes — and returns its Envoy routes
// by name.
func soapEnvoyRoutes(t *testing.T, upstreamURL string) map[string]*route.Route {
	t.Helper()

	routerCfg := routingTestRouterConfig()
	systemCfg := &config.Config{Router: *routerCfg}
	translator, err := xds.NewTranslator(slog.New(slog.NewTextHandler(io.Discard, nil)), routerCfg, nil, systemCfg)
	require.NoError(t, err)
	soapT := transform.NewSoapAPITransformer(routerCfg, systemCfg, map[string]models.PolicyDefinition{
		constants.SOAP_SYSTEM_POLICY_NAME + "|v1.0.0": {Name: constants.SOAP_SYSTEM_POLICY_NAME, Version: "v1.0.0"},
	})
	translator.SetTransformers(map[string]models.ConfigTransformer{models.KindSoapApi: transform.NewRegistry(nil, nil, nil, nil, soapT)})

	spec := api.SoapAPIData{DisplayName: "Calculator", Context: "/calc/$version", Version: "v1.0"}
	spec.Upstream.Main = api.Upstream{Url: api.Ptr(upstreamURL)}
	stored := &models.StoredConfig{
		UUID: "calc-v1.0", Kind: models.KindSoapApi, Handle: "calc-v1.0", DesiredState: models.StateDeployed,
		Configuration: api.SoapAPI{Kind: api.SoapAPIKindSoapApi, Metadata: api.Metadata{Name: "calc-v1.0"}, Spec: spec},
	}
	// The transformer must accept the spec, or the translator would fall back to the
	// legacy path, which these expectations do not describe.
	_, err = soapT.Transform(stored)
	require.NoError(t, err)

	resources, err := translator.TranslateConfigs([]*models.StoredConfig{stored}, "")
	require.NoError(t, err)

	out := map[string]*route.Route{}
	for _, res := range resources[resource.RouteType] {
		for _, vh := range res.(*route.RouteConfiguration).GetVirtualHosts() {
			for _, r := range vh.GetRoutes() {
				out[r.GetName()] = r
			}
		}
	}
	return out
}

// rewrite applies an Envoy route's regex rewrite to a request path.
func rewrite(t *testing.T, r *route.Route, path string) string {
	t.Helper()
	rr := r.GetRoute().GetRegexRewrite()
	require.NotNil(t, rr, "route %q has no regex rewrite", r.GetName())
	// Envoy writes capture references as \1; Go's regexp uses ${1}.
	sub := regexp.MustCompile(`\\(\d)`).ReplaceAllString(rr.GetSubstitution(), `$${$1}`)
	return regexp.MustCompile(rr.GetPattern().GetRegex()).ReplaceAllString(path, sub)
}

// A SoapApi is served as a wildcard POST + GET pair under its context: every path
// below the context matches, and the part after the context is carried over to the
// backend path. The ?wsdl query string is not part of the path, so it reaches the
// backend unchanged.
func TestSoapAPI_EnvoyRoutesMatchAndRewriteUnderContext(t *testing.T) {
	routes := soapEnvoyRoutes(t, "http://backend:3002/calculator/v1")
	vhost := routingTestRouterConfig().VHosts.Main.Default

	for _, method := range []string{"POST", "GET"} {
		name := xds.GenerateRouteName(method, "/calc/$version", "v1.0", transform.SoapWildcardPath, vhost)
		r, ok := routes[name]
		require.True(t, ok, "expected Envoy route %q", name)

		match := regexp.MustCompile(r.GetMatch().GetSafeRegex().GetRegex())
		assert.True(t, match.MatchString("/calc/v1.0"))
		assert.True(t, match.MatchString("/calc/v1.0/soap11"))
		assert.False(t, match.MatchString("/calc/v1.0x"))
		assert.False(t, match.MatchString("/other/v1.0/soap11"))

		assert.Equal(t, "/calculator/v1/soap11", rewrite(t, r, "/calc/v1.0/soap11"))
		assert.Equal(t, "/calculator/v1", rewrite(t, r, "/calc/v1.0"))
	}
}

// With a root upstream the backend always receives at least "/".
func TestSoapAPI_EnvoyRoutesRewriteToRootUpstream(t *testing.T) {
	routes := soapEnvoyRoutes(t, "http://backend:3002")
	vhost := routingTestRouterConfig().VHosts.Main.Default
	r, ok := routes[xds.GenerateRouteName("POST", "/calc/$version", "v1.0", transform.SoapWildcardPath, vhost)]
	require.True(t, ok)

	assert.Equal(t, "/", rewrite(t, r, "/calc/v1.0"))
	assert.Equal(t, "/soap11", rewrite(t, r, "/calc/v1.0/soap11"))
}

// An upstream path ending in "/" keeps that slash for the context itself, and a
// sub-path joins it with a single slash. This is the path the WSDL rewrite
// advertises for an address under the base path, so it must reach that address.
func TestSoapAPI_EnvoyRoutesRewriteToTrailingSlashUpstream(t *testing.T) {
	routes := soapEnvoyRoutes(t, "http://backend:3002/calculator/v1/")
	vhost := routingTestRouterConfig().VHosts.Main.Default

	for _, method := range []string{"POST", "GET"} {
		r, ok := routes[xds.GenerateRouteName(method, "/calc/$version", "v1.0", transform.SoapWildcardPath, vhost)]
		require.True(t, ok)

		assert.Equal(t, "/calculator/v1/soap11", rewrite(t, r, "/calc/v1.0/soap11"))
		assert.Equal(t, "/calculator/v1/", rewrite(t, r, "/calc/v1.0"))
		assert.Equal(t, "/calculator/v1/", rewrite(t, r, "/calc/v1.0/"))
	}
}

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
	"slices"
	"sort"
	"strings"
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

// restAPIRouteDomains translates one REST API through the transformer and
// the xDS translator and returns the domains of each virtual host carrying
// its routes, sorted.
func restAPIRouteDomains(t *testing.T, spec api.APIConfigData) [][]string {
	t.Helper()

	routerCfg := routingTestRouterConfig()
	routerCfg.VHosts = config.VHostsConfig{
		Main:    config.VHostEntry{Default: "gw.example.com", Domains: []string{"gw.example.com", " gw.internal "}},
		Sandbox: config.VHostEntry{Default: "sandbox.gw.example.com"},
	}
	systemCfg := &config.Config{Router: *routerCfg}
	translator, err := xds.NewTranslator(slog.New(slog.NewTextHandler(io.Discard, nil)), routerCfg, nil, systemCfg)
	require.NoError(t, err)
	restT := transform.NewRestAPITransformer(routerCfg, systemCfg, map[string]models.PolicyDefinition{})
	translator.SetTransformers(map[string]models.ConfigTransformer{models.KindRestApi: transform.NewRegistry(restT, nil, nil)})

	stored := &models.StoredConfig{
		UUID: "vhost-api", Kind: models.KindRestApi, Handle: "vhost-api", DesiredState: models.StateDeployed,
		Configuration: api.RestAPI{Kind: api.RestAPIKindRestApi, Metadata: api.Metadata{Name: "vhost-api"}, Spec: spec},
	}
	// The transformer rejects a spec it cannot route; the translator then
	// falls back to another path, which these expectations do not describe.
	_, err = restT.Transform(stored)
	require.NoError(t, err)

	resources, err := translator.TranslateConfigs([]*models.StoredConfig{stored}, "")
	require.NoError(t, err)

	var out [][]string
	for _, res := range resources[resource.RouteType] {
		for _, vh := range res.(*route.RouteConfiguration).GetVirtualHosts() {
			if slices.ContainsFunc(vh.GetRoutes(), func(r *route.Route) bool { return strings.Contains(r.GetName(), "/vhost") }) {
				out = append(out, vh.GetDomains())
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// The domains a REST API's routes are served on, for the vhosts shapes the
// gateway accepts. Values are written as the translator produces them: the
// gateway-default sentinel, case and a port are routed as written.
func TestRestAPITransformer_RouteDomains(t *testing.T) {
	type vhosts struct {
		main    string
		sandbox *string
	}
	sandboxByURL := &api.Upstream{Url: api.Ptr("http://sandbox:8080")}
	sandboxByRef := &api.Upstream{Ref: api.Ptr("sandbox-def")}
	gwDefault := []string{"gw.example.com", "gw.example.com:*", "gw.internal", "gw.internal:*"}
	sbDefault := []string{"sandbox.gw.example.com", "sandbox.gw.example.com:*"}

	tests := []struct {
		name    string
		vhosts  *vhosts
		sandbox *api.Upstream
		want    [][]string
	}{
		{name: "sentinel main", vhosts: &vhosts{main: constants.VHostGatewayDefault, sandbox: api.Ptr("sb.example.com")}, sandbox: sandboxByURL,
			want: [][]string{{"_gateway_default_", "_gateway_default_:*"}, {"sb.example.com", "sb.example.com:*"}}},
		{name: "sentinel sandbox", vhosts: &vhosts{main: "a.example.com", sandbox: api.Ptr(constants.VHostGatewayDefault)}, sandbox: sandboxByURL,
			want: [][]string{{"_gateway_default_", "_gateway_default_:*"}, {"a.example.com", "a.example.com:*"}}},
		{name: "sentinel in a list", vhosts: &vhosts{main: "a.example.com;" + constants.VHostGatewayDefault},
			want: [][]string{{"_gateway_default_", "_gateway_default_:*"}, {"a.example.com", "a.example.com:*"}}},
		{name: "list with duplicates and whitespace", vhosts: &vhosts{main: " b1.example.com ; B2.Example.com;b1.example.com ;; "},
			want: [][]string{{"B2.Example.com", "B2.Example.com:*"}, {"b1.example.com", "b1.example.com:*"}}},
		{name: "sandbox via ref", vhosts: &vhosts{main: "a.example.com", sandbox: api.Ptr("sb.example.com")}, sandbox: sandboxByRef,
			want: [][]string{{"a.example.com", "a.example.com:*"}, {"sb.example.com", "sb.example.com:*"}}},
		{name: "sandbox via url", vhosts: &vhosts{main: "a.example.com", sandbox: api.Ptr("sb.example.com")}, sandbox: sandboxByURL,
			want: [][]string{{"a.example.com", "a.example.com:*"}, {"sb.example.com", "sb.example.com:*"}}},
		{name: "sandbox with surrounding whitespace", vhosts: &vhosts{main: "a.example.com", sandbox: api.Ptr(" sb.example.com ")}, sandbox: sandboxByURL,
			want: [][]string{{"a.example.com", "a.example.com:*"}, {"sb.example.com", "sb.example.com:*"}}},
		{name: "host and port", vhosts: &vhosts{main: "c.example.com:8443", sandbox: api.Ptr("sb-c.example.com:9443")}, sandbox: sandboxByURL,
			want: [][]string{{"c.example.com:8443"}, {"sb-c.example.com:9443"}}},
		{name: "uppercase names", vhosts: &vhosts{main: "PAY.Example.COM", sandbox: api.Ptr("SB.Example.com")}, sandbox: sandboxByURL,
			want: [][]string{{"PAY.Example.COM", "PAY.Example.COM:*"}, {"SB.Example.com", "SB.Example.com:*"}}},
		{name: "empty vhosts", vhosts: &vhosts{main: ""}, sandbox: sandboxByURL, want: [][]string{gwDefault, sbDefault}},
		{name: "whitespace vhosts", vhosts: &vhosts{main: "   ", sandbox: api.Ptr("  ")}, sandbox: sandboxByURL, want: [][]string{gwDefault, sbDefault}},
		{name: "no vhosts", sandbox: sandboxByURL, want: [][]string{gwDefault, sbDefault}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := api.APIConfigData{
				DisplayName: "Vhost API", Context: "/vhost", Version: "1.0.0",
				Operations: []api.Operation{{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/hello")}},
			}
			spec.Upstream.Main = api.Upstream{Url: api.Ptr("http://backend:8080")}
			spec.Upstream.Sandbox = tt.sandbox
			if tt.sandbox == sandboxByRef {
				spec.UpstreamDefinitions = &[]api.UpstreamDefinition{{Name: "sandbox-def", Upstreams: []struct {
					Url    string `json:"url" yaml:"url"`
					Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
				}{{Url: "http://sandbox-def:8080"}}}}
			}
			if tt.vhosts != nil {
				spec.Vhosts = &struct {
					Main    string  `json:"main" yaml:"main"`
					Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
				}{Main: tt.vhosts.main, Sandbox: tt.vhosts.sandbox}
			}
			assert.Equal(t, tt.want, restAPIRouteDomains(t, spec))
		})
	}
}

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

package xds

import (
	"strings"
	"testing"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	dfpcluster "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/dynamic_forward_proxy/v3"
	dfpfilter "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/dynamic_forward_proxy/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	upstreamhttp "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func templatedBackend(name, template string, tls *models.UpstreamTLS) BackendCluster {
	return BackendCluster{Name: name, ClusterName: "backend_" + name + "_0123456789ab", Upstream: &models.UpstreamCluster{
		Name: name, BasePath: "/", HostTemplate: template, TLS: tls,
	}}
}

func plainBackend(name string) BackendCluster {
	return BackendCluster{Name: name, ClusterName: "backend_" + name, Upstream: &models.UpstreamCluster{
		Name: name, BasePath: "/", Endpoints: []models.Endpoint{{Host: name, Port: 80}},
	}}
}

func translateAll(t *testing.T, translator *Translator) map[string][]types.Resource {
	t.Helper()
	resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test")
	require.NoError(t, err)
	out := map[string][]types.Resource{}
	for typ, list := range resources {
		out[typ] = list
	}
	return out
}

func dfpClusterConfig(t *testing.T, c *cluster.Cluster) *dfpcluster.ClusterConfig {
	t.Helper()
	custom := c.GetClusterType()
	require.NotNil(t, custom, "a templated Backend's cluster is a custom cluster type")
	assert.Equal(t, dynamicForwardProxyClusterType, custom.GetName())
	var cfg dfpcluster.ClusterConfig
	require.NoError(t, custom.GetTypedConfig().UnmarshalTo(&cfg))
	return &cfg
}

func protocolOptions(t *testing.T, c *cluster.Cluster) *upstreamhttp.HttpProtocolOptions {
	t.Helper()
	raw, ok := c.GetTypedExtensionProtocolOptions()[httpProtocolOptionsKey]
	require.True(t, ok, "the cluster sets HttpProtocolOptions")
	var opts upstreamhttp.HttpProtocolOptions
	require.NoError(t, raw.UnmarshalTo(&opts))
	return &opts
}

func httpFilterNames(t *testing.T, l *listener.Listener) []string {
	t.Helper()
	var m hcm.HttpConnectionManager
	require.NoError(t, l.GetFilterChains()[0].GetFilters()[0].GetTypedConfig().UnmarshalTo(&m))
	names := []string{}
	for _, f := range m.GetHttpFilters() {
		names = append(names, f.GetName())
	}
	return names
}

func sharedRouteConfig(t *testing.T, resources map[string][]types.Resource) *route.RouteConfiguration {
	t.Helper()
	for _, r := range resources[resourcev3.RouteType] {
		if rc, ok := r.(*route.RouteConfiguration); ok && rc.GetName() == SharedRouteConfigName {
			return rc
		}
	}
	t.Fatal("no shared route configuration")
	return nil
}

func validateAll(t *testing.T, resources map[string][]types.Resource) {
	t.Helper()
	for typ, list := range resources {
		for _, r := range list {
			if v, ok := r.(interface{ ValidateAll() error }); ok {
				require.NoError(t, v.ValidateAll(), "%s %T must pass the proto validation Envoy applies", typ, r)
			}
		}
	}
}

// A templated Backend becomes a dynamic forward proxy cluster on the shared DNS cache: no fixed
// endpoints, its connect timeout, and HTTP/1.1 as for any upstream.
func TestDynamicHostCluster_Shape(t *testing.T) {
	translator := createTestTranslator()
	connect := 3 * time.Second
	b := templatedBackend("env-router", "http://env-{key}.testbench:3000", nil)
	b.Upstream.ConnectTimeout = &connect
	translator.SetBackendSource(staticBackendSource{b, plainBackend("tenant-a")})

	resources := translateAll(t, translator)
	validateAll(t, resources)
	clusters := map[string]*cluster.Cluster{}
	for _, r := range resources[resourcev3.ClusterType] {
		clusters[r.(*cluster.Cluster).GetName()] = r.(*cluster.Cluster)
	}
	c, ok := clusters["backend_env-router_0123456789ab"]
	require.True(t, ok)

	cfg := dfpClusterConfig(t, c)
	assert.True(t, strings.HasPrefix(cfg.GetDnsCacheConfig().GetName(), backendDNSCachePrefix))
	assert.Equal(t, uint32(1024), cfg.GetDnsCacheConfig().GetMaxHosts().GetValue(), "the default max_hosts")
	assert.Equal(t, 5*time.Minute, cfg.GetDnsCacheConfig().GetHostTtl().AsDuration(), "the default host TTL")
	assert.True(t, cfg.GetAllowInsecureClusterOptions(), "a plaintext Backend has no SNI or SAN to check")
	assert.Equal(t, cluster.Cluster_CLUSTER_PROVIDED, c.GetLbPolicy())
	assert.Nil(t, c.GetLoadAssignment(), "no fixed endpoints")
	assert.Nil(t, c.GetTransportSocket(), "plaintext")
	assert.Equal(t, 3*time.Second, c.GetConnectTimeout().AsDuration())
	assert.NotNil(t, protocolOptions(t, c).GetExplicitHttpConfig().GetHttpProtocolOptions(), "HTTP/1.1, as for any upstream")

	plain := clusters["backend_tenant-a"]
	require.NotNil(t, plain)
	assert.Nil(t, plain.GetClusterType(), "a plain Backend keeps its STRICT_DNS cluster")
}

// TLS sits on the cluster's own transport socket, since a dynamic host carries no endpoint
// metadata for a transport socket match. It names no server and no SAN: auto SNI and SAN
// validation take both from the host each request resolves. Identity and trust come from the
// same SDS secrets as for a plain Backend, and the snapshot keeps them.
func TestDynamicHostCluster_TLS(t *testing.T) {
	routerCfg := testRouterConfig()
	routerCfg.Upstream.TLS.DisableSslVerification = false
	translator, err := NewTranslator(createTestLogger(), routerCfg, nil, testConfig())
	require.NoError(t, err)
	tls := &models.UpstreamTLS{Enabled: true, APITraffic: true, HasTLSBlock: true, IdentityName: "client-cert",
		TrustedCANames: []string{"backend-ca"}, VerifyHostName: true}
	translator.SetBackendSource(staticBackendSource{templatedBackend("env-router", "https://url-{key}.url.fr", tls)})

	resources := translateAll(t, translator)
	validateAll(t, resources)
	var c *cluster.Cluster
	for _, r := range resources[resourcev3.ClusterType] {
		if r.(*cluster.Cluster).GetName() == "backend_env-router_0123456789ab" {
			c = r.(*cluster.Cluster)
		}
	}
	require.NotNil(t, c)
	require.Empty(t, c.GetTransportSocketMatches(), "a transport socket match would never apply to a dynamic host")
	require.NotNil(t, c.GetTransportSocket())

	var tlsCtx tlsv3.UpstreamTlsContext
	require.NoError(t, c.GetTransportSocket().GetTypedConfig().UnmarshalTo(&tlsCtx))
	assert.Empty(t, tlsCtx.GetSni(), "the server name comes from each request's host")
	combined := tlsCtx.GetCommonTlsContext().GetCombinedValidationContext()
	require.NotNil(t, combined)
	assert.Equal(t, BackendCAValidationContextSecretName("backend_env-router_0123456789ab"),
		combined.GetValidationContextSdsSecretConfig().GetName())
	assert.Empty(t, combined.GetDefaultValidationContext().GetMatchTypedSubjectAltNames(),
		"no fixed SAN: auto SAN validation checks each request's host")
	assert.Equal(t, GatewayIdentitySecretName("client-cert"),
		tlsCtx.GetCommonTlsContext().GetTlsCertificateSdsSecretConfigs()[0].GetName())

	opts := protocolOptions(t, c).GetUpstreamHttpProtocolOptions()
	assert.True(t, opts.GetAutoSni())
	assert.True(t, opts.GetAutoSanValidation())
	assert.False(t, dfpClusterConfig(t, c).GetAllowInsecureClusterOptions())

	clusters := resources[resourcev3.ClusterType]
	listeners := resources[resourcev3.ListenerType]
	assert.True(t, SnapshotReferencesSDSSecret(clusters, listeners, GatewayIdentitySecretName("client-cert")),
		"the identity secret is kept in the snapshot")
	assert.True(t, SnapshotReferencesSDSSecret(clusters, listeners,
		BackendCAValidationContextSecretName("backend_env-router_0123456789ab")), "the trust secret is kept in the snapshot")

	t.Run("hostname verification off", func(t *testing.T) {
		off := *tls
		off.VerifyHostName = false
		translator.SetBackendSource(staticBackendSource{templatedBackend("env-router", "https://url-{key}.url.fr", &off)})
		resources := translateAll(t, translator)
		for _, r := range resources[resourcev3.ClusterType] {
			if c := r.(*cluster.Cluster); c.GetName() == "backend_env-router_0123456789ab" {
				assert.False(t, protocolOptions(t, c).GetUpstreamHttpProtocolOptions().GetAutoSanValidation())
				assert.True(t, dfpClusterConfig(t, c).GetAllowInsecureClusterOptions())
			}
		}
	})
}

// The listeners get the dynamic forward proxy filter, between Lua and the router, and the route
// configuration points it at the host header and strips that header, only while a templated
// Backend's cluster is built. Without one, both are as they were.
func TestDynamicHostFilter_OnlyWithATemplatedBackend(t *testing.T) {
	translator := createTestTranslator()
	translator.SetBackendSource(staticBackendSource{plainBackend("tenant-a")})
	without := translateAll(t, translator)
	for _, r := range without[resourcev3.ListenerType] {
		assert.NotContains(t, httpFilterNames(t, r.(*listener.Listener)), dynamicForwardProxyFilterName)
	}
	rc := sharedRouteConfig(t, without)
	assert.Empty(t, rc.GetTypedPerFilterConfig())
	assert.NotContains(t, rc.GetRequestHeadersToRemove(), constants.TargetUpstreamHostHeader)

	translator.SetBackendSource(staticBackendSource{plainBackend("tenant-a"),
		templatedBackend("env-router", "https://url-{key}.url.fr", &models.UpstreamTLS{Enabled: true})})
	with := translateAll(t, translator)
	validateAll(t, with)
	for _, r := range with[resourcev3.ListenerType] {
		names := httpFilterNames(t, r.(*listener.Listener))
		require.Contains(t, names, dynamicForwardProxyFilterName)
		dfp := indexOf(names, dynamicForwardProxyFilterName)
		assert.Equal(t, wellknown.Router, names[dfp+1], "the filter runs right before the router")
		assert.Less(t, indexOf(names, wellknown.Lua), dfp, "after Lua, which rewrites the path")
	}
	rc = sharedRouteConfig(t, with)
	var perRoute dfpfilter.PerRouteConfig
	require.NoError(t, rc.GetTypedPerFilterConfig()[dynamicForwardProxyFilterName].UnmarshalTo(&perRoute))
	assert.Equal(t, constants.TargetUpstreamHostHeader, perRoute.GetHostRewriteHeader())
	assert.Contains(t, rc.GetRequestHeadersToRemove(), constants.TargetUpstreamHostHeader)

	// Back to no templated Backend: the filter and the route setting go away again.
	translator.SetBackendSource(staticBackendSource{plainBackend("tenant-a")})
	again := translateAll(t, translator)
	for _, r := range again[resourcev3.ListenerType] {
		assert.NotContains(t, httpFilterNames(t, r.(*listener.Listener)), dynamicForwardProxyFilterName)
	}
}

func indexOf(list []string, value string) int {
	for i, v := range list {
		if v == value {
			return i
		}
	}
	return -1
}

// The filter and every templated Backend cluster share one DNS cache configuration, which Envoy
// requires. Its name follows its settings, so a settings change never reuses a name Envoy holds
// with other settings.
func TestBackendDNSCache_SharedAndNamedBySettings(t *testing.T) {
	translator := createTestTranslator()
	filter, err := translator.dynamicForwardProxyFilter()
	require.NoError(t, err)
	var filterCfg dfpfilter.FilterConfig
	require.NoError(t, filter.GetTypedConfig().UnmarshalTo(&filterCfg))

	c, err := translator.createDynamicHostCluster("backend_x_0123456789ab", &models.UpstreamCluster{HostTemplate: "http://x-{key}.example.com"}, "")
	require.NoError(t, err)
	assert.True(t, proto.Equal(filterCfg.GetDnsCacheConfig(), dfpClusterConfig(t, c).GetDnsCacheConfig()))

	before := filterCfg.GetDnsCacheConfig().GetName()
	translator.routerConfig.Upstream.BackendDNSCache = config.BackendDNSCacheConfig{MaxHosts: 64}
	changed, err := translator.backendDNSCacheConfig()
	require.NoError(t, err)
	assert.NotEqual(t, before, changed.GetName())
	assert.Equal(t, uint32(64), changed.GetMaxHosts().GetValue())
}

// With network access control enabled, the DNS cache drops every resolved address the access
// control refuses, so a key can't reach the cloud metadata address or, with private networks
// denied, a private one. Disabled, nothing is filtered.
func TestBackendDNSCache_FiltersAddressesAccessControlRefuses(t *testing.T) {
	translator := createTestTranslator()
	disabled, err := translator.backendDNSCacheConfig()
	require.NoError(t, err)
	assert.Nil(t, disabled.GetResolvedAddressFilter())

	cfg := testConfig()
	cfg.Controller.Server.NetworkSecurity.AccessControl = config.AccessControlConfig{Enabled: true, DenyPrivateNetworks: true}
	translator, err = NewTranslator(createTestLogger(), testRouterConfig(), nil, cfg)
	require.NoError(t, err)
	enabled, err := translator.backendDNSCacheConfig()
	require.NoError(t, err)
	ranges := map[string]bool{}
	for _, r := range enabled.GetResolvedAddressFilter().GetRanges() {
		ranges[r.GetAddressPrefix()+"/"+itoa(r.GetPrefixLen().GetValue())] = true
	}
	for _, want := range []string{"169.254.0.0/16", "10.0.0.0/8", "127.0.0.0/8", "::1/128"} {
		assert.True(t, ranges[want], "%s is filtered", want)
	}
	assert.False(t, enabled.GetResolvedAddressFilter().GetInvertMatch())
	assert.NotEqual(t, disabled.GetName(), enabled.GetName(), "the filter is part of the settings the name follows")
}

func itoa(v uint32) string {
	const digits = "0123456789"
	if v == 0 {
		return "0"
	}
	out := ""
	for v > 0 {
		out = string(digits[v%10]) + out
		v /= 10
	}
	return out
}

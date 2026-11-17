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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	dfpcluster "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/dynamic_forward_proxy/v3"
	dfpcommon "github.com/envoyproxy/go-control-plane/envoy/extensions/common/dynamic_forward_proxy/v3"
	dfpfilter "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/dynamic_forward_proxy/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	upstreamhttp "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// A templated Backend's endpoint host takes a key each request carries, so its cluster can't
// list fixed endpoints. It is an Envoy dynamic forward proxy (DFP) cluster: the policy engine
// expands the key into a host and puts it in TargetUpstreamHostHeader, the DFP HTTP filter
// resolves that host through a DNS cache (dropping any address the gateway's network access
// control refuses), and the cluster connects to it with the Backend's own TLS settings.
//
// The filter and the per-route setting that points it at TargetUpstreamHostHeader are added
// only while a templated Backend's cluster is built, so a gateway without one serves exactly the
// listeners and routes it did before. The filter passes through every request whose route
// cluster is not a DFP cluster.
const (
	// dynamicForwardProxyFilterName is the DFP HTTP filter, and the key of its per-route config.
	dynamicForwardProxyFilterName = "envoy.filters.http.dynamic_forward_proxy"
	// dynamicForwardProxyClusterType is the DFP cluster extension.
	dynamicForwardProxyClusterType = "envoy.clusters.dynamic_forward_proxy"
	// httpProtocolOptionsKey is the typed_extension_protocol_options key of HttpProtocolOptions.
	httpProtocolOptionsKey = "envoy.extensions.upstreams.http.v3.HttpProtocolOptions"
	// backendDNSCachePrefix starts the DNS cache name. The rest is a hash of the cache's
	// settings: Envoy refuses a second, different configuration under a name it already holds,
	// and a draining listener keeps the old one alive, so a settings change takes a new name.
	backendDNSCachePrefix = "backend_dns_cache_"
)

// backendDNSCacheConfig is the DNS cache every templated Backend cluster and the DFP filter
// share: router.upstream.backend_dns_cache sizes it, and, when network access control is
// enabled, its resolved_address_filter drops every address the access control refuses, so a
// key can't steer a request to an address an API upstream could not use either.
func (t *Translator) backendDNSCacheConfig() (*dfpcommon.DnsCacheConfig, error) {
	settings := t.routerConfig.Upstream.BackendDNSCache
	cfg := &dfpcommon.DnsCacheConfig{
		DnsLookupFamily: cluster.Cluster_V4_PREFERRED,
		MaxHosts:        wrapperspb.UInt32(settings.EffectiveMaxHosts()),
		HostTtl:         durationpb.New(settings.EffectiveHostTTL()),
	}
	if t.config != nil {
		policy, enabled, err := t.config.ArtifactAccessControlPolicy()
		if err != nil {
			return nil, err
		}
		if enabled {
			if prefixes := policy.DeniedPrefixes(); len(prefixes) > 0 {
				if cfg.ResolvedAddressFilter, err = addressMatcher(prefixes); err != nil {
					return nil, err
				}
			}
		}
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the backend DNS cache settings: %w", err)
	}
	sum := sha256.Sum256(raw)
	cfg.Name = backendDNSCachePrefix + hex.EncodeToString(sum[:])[:12]
	return cfg, nil
}

// addressMatcher matches the given prefixes. Its ranges are of the xds core CidrRange type, from
// a module the controller reaches only through go-control-plane, so the message is filled from
// its JSON form rather than by importing that module directly.
func addressMatcher(prefixes []netip.Prefix) (*matcher.AddressMatcher, error) {
	type cidrRange struct {
		AddressPrefix string `json:"address_prefix"`
		PrefixLen     int    `json:"prefix_len"`
	}
	ranges := make([]cidrRange, 0, len(prefixes))
	for _, prefix := range prefixes {
		ranges = append(ranges, cidrRange{AddressPrefix: prefix.Addr().String(), PrefixLen: prefix.Bits()})
	}
	raw, err := json.Marshal(map[string]any{"ranges": ranges})
	if err != nil {
		return nil, fmt.Errorf("failed to encode the resolved address filter: %w", err)
	}
	m := &matcher.AddressMatcher{}
	if err := protojson.Unmarshal(raw, m); err != nil {
		return nil, fmt.Errorf("failed to build the resolved address filter: %w", err)
	}
	return m, nil
}

// createDynamicHostCluster builds a templated Backend's cluster. TLS sits on the cluster's own
// transport socket: a DFP host carries no endpoint metadata, so a transport socket match would
// never apply and the cluster would silently dial plaintext. The server name and its certificate
// check come from the host each request resolves (auto SNI and SAN validation).
func (t *Translator) createDynamicHostCluster(name string, uc *models.UpstreamCluster, validationSecretName string) (*cluster.Cluster, error) {
	dnsCache, err := t.backendDNSCacheConfig()
	if err != nil {
		return nil, err
	}

	tlsEnabled := uc.TLS != nil && uc.TLS.Enabled
	verifyHostName := t.routerConfig.Upstream.TLS.VerifyHostName
	if uc.TLS != nil && uc.TLS.HasTLSBlock {
		verifyHostName = uc.TLS.VerifyHostName
	}
	sanValidation := tlsEnabled && verifyHostName && !t.routerConfig.Upstream.TLS.DisableSslVerification

	clusterConfig, err := anypb.New(&dfpcluster.ClusterConfig{
		ClusterImplementationSpecifier: &dfpcluster.ClusterConfig_DnsCacheConfig{DnsCacheConfig: dnsCache},
		// The cluster refuses to run without auto SNI and SAN validation unless told otherwise;
		// a plaintext Backend, or one whose hostname check is turned off, has neither.
		AllowInsecureClusterOptions: !sanValidation,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode the dynamic forward proxy cluster config: %w", err)
	}

	protocolOptions := &upstreamhttp.HttpProtocolOptions{
		UpstreamProtocolOptions: &upstreamhttp.HttpProtocolOptions_ExplicitHttpConfig_{
			ExplicitHttpConfig: &upstreamhttp.HttpProtocolOptions_ExplicitHttpConfig{
				ProtocolConfig: &upstreamhttp.HttpProtocolOptions_ExplicitHttpConfig_HttpProtocolOptions{
					HttpProtocolOptions: &core.Http1ProtocolOptions{},
				},
			},
		},
	}

	c := &cluster.Cluster{
		Name:           name,
		ConnectTimeout: durationpb.New(t.effectiveConnectTimeout(uc.ConnectTimeout)),
		ClusterDiscoveryType: &cluster.Cluster_ClusterType{ClusterType: &cluster.Cluster_CustomClusterType{
			Name:        dynamicForwardProxyClusterType,
			TypedConfig: clusterConfig,
		}},
		LbPolicy: cluster.Cluster_CLUSTER_PROVIDED,
	}

	if tlsEnabled {
		tlsContext, err := t.buildUpstreamTLSContext(nil, uc.HostTemplate, true, uc.TLS, validationSecretName)
		if err != nil {
			return nil, err
		}
		tlsAny, err := anypb.New(tlsContext)
		if err != nil {
			return nil, fmt.Errorf("failed to encode the upstream TLS context: %w", err)
		}
		c.TransportSocket = &core.TransportSocket{
			Name:       constants.EnvoyTLSTransportSocket,
			ConfigType: &core.TransportSocket_TypedConfig{TypedConfig: tlsAny},
		}
		protocolOptions.UpstreamHttpProtocolOptions = &core.UpstreamHttpProtocolOptions{
			AutoSni:           true,
			AutoSanValidation: sanValidation,
		}
	}

	optionsAny, err := anypb.New(protocolOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the upstream HTTP protocol options: %w", err)
	}
	c.TypedExtensionProtocolOptions = map[string]*anypb.Any{httpProtocolOptionsKey: optionsAny}
	return c, nil
}

// dynamicForwardProxyFilter is the DFP HTTP filter, on the same DNS cache as the clusters.
func (t *Translator) dynamicForwardProxyFilter() (*hcm.HttpFilter, error) {
	dnsCache, err := t.backendDNSCacheConfig()
	if err != nil {
		return nil, err
	}
	filterAny, err := anypb.New(&dfpfilter.FilterConfig{
		ImplementationSpecifier: &dfpfilter.FilterConfig_DnsCacheConfig{DnsCacheConfig: dnsCache},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode the dynamic forward proxy filter config: %w", err)
	}
	return &hcm.HttpFilter{
		Name:       dynamicForwardProxyFilterName,
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: filterAny},
	}, nil
}

// dynamicForwardProxyRouteConfig points the DFP filter at TargetUpstreamHostHeader for every
// route. The filter reads it only for a request routed to a DFP cluster, after the policy engine
// has set it, and the swap happens after route matching, so a virtual host is never re-chosen.
func dynamicForwardProxyRouteConfig() (*anypb.Any, error) {
	return anypb.New(&dfpfilter.PerRouteConfig{
		HostRewriteSpecifier: &dfpfilter.PerRouteConfig_HostRewriteHeader{
			HostRewriteHeader: constants.TargetUpstreamHostHeader,
		},
	})
}

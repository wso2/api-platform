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

// Package translator implements gateway-controller (core)'s
// xds.EventGatewayXDSHooks interface, moved out of core's
// pkg/xds/translator.go (translateAsyncAPIConfig, createInternalListenerForWebSubHub,
// createDynamicFwdListenerForWebSubHub, createDynamicForwardProxyCluster).
package translator

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	dfpcluster "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/dynamic_forward_proxy/v3"
	common_dfp "github.com/envoyproxy/go-control-plane/envoy/extensions/common/dynamic_forward_proxy/v3"
	dfpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/dynamic_forward_proxy/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	router "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"

	eventgateway "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/api/eventgateway"
	eventgatewayconfig "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/config"
)

const (
	// WebSubHubInternalClusterName mirrors core's xds.WebSubHubInternalClusterName.
	WebSubHubInternalClusterName = "WEBSUBHUB_INTERNAL_CLUSTER"
	// DynamicForwardProxyClusterName mirrors core's xds.DynamicForwardProxyClusterName.
	DynamicForwardProxyClusterName = "dynamic-forward-proxy-cluster"
	// dynamicForwardProxyFilterName is Envoy's registered name for the dynamic forward proxy HTTP filter.
	dynamicForwardProxyFilterName = "envoy.filters.http.dynamic_forward_proxy"
)

// Hooks implements xds.EventGatewayXDSHooks.
type Hooks struct {
	eventGatewayConfig eventgatewayconfig.EventGatewayConfig
}

// New creates a new Hooks instance.
func New(cfg eventgatewayconfig.EventGatewayConfig) *Hooks {
	return &Hooks{eventGatewayConfig: cfg}
}

var _ xds.EventGatewayXDSHooks = (*Hooks)(nil)

// BuildHubResources returns the dynamic-forward-proxy cluster, WebSubHub
// cluster, and internal listener(s) needed for WebSubHub connectivity.
func (h *Hooks) BuildHubResources(t *xds.Translator, httpsEnabled bool) ([]*cluster.Cluster, []*listener.Listener, error) {
	var clusters []*cluster.Cluster
	var listeners []*listener.Listener

	dynamicForwardProxyCluster := h.createDynamicForwardProxyCluster(t)
	if dynamicForwardProxyCluster == nil {
		return nil, nil, fmt.Errorf("failed to create dynamic forward proxy cluster")
	}
	clusters = append(clusters, dynamicForwardProxyCluster)

	dynamicProxyListener, err := h.createDynamicFwdListenerForWebSubHub(t, httpsEnabled)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create WebSub listener: %w", err)
	}
	listeners = append(listeners, dynamicProxyListener)

	parsedURL, err := url.Parse(h.eventGatewayConfig.WebSubHubURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid upstream URL: %w", err)
	}
	if parsedURL.Port() == "" {
		parsedURL.Host = fmt.Sprintf("%s:%d", parsedURL.Hostname(), h.eventGatewayConfig.WebSubHubPort)
	}
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "http"
	}
	websubhubCluster := t.CreateCluster(constants.WEBSUBHUB_INTERNAL_CLUSTER_NAME, parsedURL, nil, nil)
	clusters = append(clusters, websubhubCluster)

	websubInternalListener, err := h.createInternalListenerForWebSubHub(t, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create WebSub internal listener: %w", err)
	}
	listeners = append(listeners, websubInternalListener)

	if httpsEnabled {
		httpsListener, err := h.createInternalListenerForWebSubHub(t, true)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create HTTPS listener: %w", err)
		}
		listeners = append(listeners, httpsListener)
	}

	return clusters, listeners, nil
}

// TranslateWebSubAPI builds routes/clusters for a single WebSubApi config.
func (h *Hooks) TranslateWebSubAPI(t *xds.Translator, cfg *models.StoredConfig, allConfigs []*models.StoredConfig) ([]*route.Route, []*cluster.Cluster, error) {
	webSubCfg, ok := cfg.Configuration.(eventgateway.WebSubAPI)
	if !ok {
		return nil, nil, fmt.Errorf("configuration is not a WebSubAPI")
	}
	apiData := webSubCfg.Spec

	clusters := []*cluster.Cluster{}

	mainClusterName := constants.WEBSUBHUB_INTERNAL_CLUSTER_NAME
	parsedMainURL, err := url.Parse(h.eventGatewayConfig.WebSubHubURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid upstream URL: %w", err)
	}
	if parsedMainURL.Path == "" {
		parsedMainURL.Path = constants.WEBSUB_PATH
	}

	routesList := make([]*route.Route, 0)
	mainRoutesList := make([]*route.Route, 0)

	effectiveMainVHost := t.Config().Router.VHosts.Main.Default
	if apiData.Vhosts != nil {
		if strings.TrimSpace(apiData.Vhosts.Main) != "" {
			effectiveMainVHost = apiData.Vhosts.Main
		}
	}
	apiProjectID := xds.ExtractProjectIDFromConfig(cfg)

	if apiData.Channels != nil {
		for chName := range *apiData.Channels {
			if !strings.HasPrefix(chName, "/") {
				chName = "/" + chName
			}
			r := t.CreateRoutePerTopic(cfg.UUID, apiData.DisplayName, apiData.Version, apiData.Context, "SUB", chName,
				mainClusterName, effectiveMainVHost, cfg.Kind, apiProjectID)
			mainRoutesList = append(mainRoutesList, r)
			rUnsub := t.CreateRoutePerTopic(cfg.UUID, apiData.DisplayName, apiData.Version, apiData.Context, "UNSUB", chName,
				mainClusterName, effectiveMainVHost, cfg.Kind, apiProjectID)
			mainRoutesList = append(mainRoutesList, rUnsub)
		}
	}
	templateHandle := t.ExtractTemplateHandle(cfg, allConfigs)
	providerName := t.ExtractProviderName(cfg, allConfigs)
	r := t.CreateRoute(cfg.UUID, apiData.DisplayName, apiData.Version, apiData.Context, "POST", constants.WEBSUB_PATH, mainClusterName, "/", effectiveMainVHost, cfg.Kind, templateHandle, providerName, nil, apiProjectID, nil, false, nil)
	routesList = append(routesList, mainRoutesList...)
	routesList = append(routesList, r)

	return routesList, clusters, nil
}

func (h *Hooks) createInternalListenerForWebSubHub(t *xds.Translator, isHTTPS bool) (*listener.Listener, error) {
	routeConfig := &route.RouteConfiguration{
		Name: "websubhub-internal-route",
		VirtualHosts: []*route.VirtualHost{{
			Name:    "WEBSUBHUB_INTERNAL_VHOST",
			Domains: []string{"*"},
			Routes: []*route.Route{{
				Match: &route.RouteMatch{PathSpecifier: &route.RouteMatch_Path{Path: "/websubhub/operations"}},
				Action: &route.Route_Route{Route: &route.RouteAction{
					ClusterSpecifier: &route.RouteAction_Cluster{Cluster: WebSubHubInternalClusterName},
					Timeout:          durationpb.New(30 * time.Second),
					PrefixRewrite:    "/hub",
				}},
			}},
		}},
	}

	routerCfg := &router.Router{}
	routerAny, err := anypb.New(routerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create router config: %w", err)
	}

	httpFilters := make([]*hcm.HttpFilter, 0)

	extProcFilter, err := t.CreateExtProcFilter()
	if err != nil {
		return nil, fmt.Errorf("failed to create ext_proc filter: %w", err)
	}
	httpFilters = append(httpFilters, extProcFilter)

	luaFilter, err := t.CreateLuaFilter()
	if err != nil {
		return nil, fmt.Errorf("failed to create lua filter: %w", err)
	}
	httpFilters = append(httpFilters, luaFilter)

	httpFilters = append(httpFilters, &hcm.HttpFilter{
		Name: wellknown.Router,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: routerAny,
		},
	})

	manager := &hcm.HttpConnectionManager{
		CodecType:         hcm.HttpConnectionManager_AUTO,
		StatPrefix:        "http",
		GenerateRequestId: wrapperspb.Bool(true),
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: routeConfig,
		},
		HttpFilters: httpFilters,
	}

	if t.RouterConfig().AccessLogs.Enabled {
		accessLogs, err := t.CreateAccessLogConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create access log config: %w", err)
		}
		manager.AccessLog = accessLogs
	}

	tracingConfig, err := t.CreateTracingConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create tracing config: %w", err)
	}
	if tracingConfig != nil {
		manager.Tracing = tracingConfig
	}

	pbst, err := anypb.New(manager)
	if err != nil {
		return nil, err
	}

	var listenerName string
	var port uint32
	if isHTTPS {
		listenerName = fmt.Sprintf("listener_https_%d", constants.WEBSUB_HUB_INTERNAL_HTTPS_PORT)
		port = uint32(constants.WEBSUB_HUB_INTERNAL_HTTPS_PORT)
	} else {
		listenerName = fmt.Sprintf("listener_http_%d", constants.WEBSUB_HUB_INTERNAL_HTTP_PORT)
		port = uint32(constants.WEBSUB_HUB_INTERNAL_HTTP_PORT)
	}

	filterChain := &listener.FilterChain{
		Filters: []*listener.Filter{{
			Name: wellknown.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{
				TypedConfig: pbst,
			},
		}},
	}

	if isHTTPS {
		tlsContext, err := t.CreateDownstreamTLSContext()
		if err != nil {
			return nil, fmt.Errorf("failed to create downstream TLS context: %w", err)
		}

		tlsContextAny, err := anypb.New(tlsContext)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal downstream TLS context: %w", err)
		}

		filterChain.TransportSocket = &core.TransportSocket{
			Name: "envoy.transport_sockets.tls",
			ConfigType: &core.TransportSocket_TypedConfig{
				TypedConfig: tlsContextAny,
			},
		}
	}

	return &listener.Listener{
		Name: listenerName,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: port,
					},
				},
			},
		},
		FilterChains: []*listener.FilterChain{filterChain},
	}, nil
}

func (h *Hooks) createDynamicFwdListenerForWebSubHub(t *xds.Translator, isHTTPS bool) (*listener.Listener, error) {
	parsedHubURL, err := url.Parse(h.eventGatewayConfig.WebSubHubURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %w", err)
	}

	dynamicForwardProxyRouteConfig := &route.RouteConfiguration{
		Name: "dynamic-forward-proxy-routing",
		VirtualHosts: []*route.VirtualHost{{
			Name:                   "DYNAMIC_FORWARD_PROXY_VHOST_WEBSUBHUB",
			Domains:                []string{parsedHubURL.Host},
			RequestHeadersToRemove: []string{xds.EnvoyOriginalPathHeader},
			Routes: []*route.Route{{
				Match: &route.RouteMatch{PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/"}},
				Action: &route.Route_Route{Route: &route.RouteAction{
					ClusterSpecifier: &route.RouteAction_Cluster{Cluster: DynamicForwardProxyClusterName},
					Timeout:          durationpb.New(30 * time.Second),
					RetryPolicy: &route.RetryPolicy{
						RetryOn:    "5xx,reset,connect-failure,refused-stream",
						NumRetries: wrapperspb.UInt32(1),
					},
				}},
			}},
		}},
	}
	httpFilters := make([]*hcm.HttpFilter, 0)

	extProcFilter, err := t.CreateExtProcFilter()
	if err != nil {
		return nil, fmt.Errorf("failed to create ext_proc filter: %w", err)
	}
	httpFilters = append(httpFilters, extProcFilter)

	luaFilter, err := t.CreateLuaFilter()
	if err != nil {
		return nil, fmt.Errorf("failed to create lua filter: %w", err)
	}
	httpFilters = append(httpFilters, luaFilter)

	dnsCacheConfig := &common_dfp.DnsCacheConfig{
		Name:            "dynamic_forward_proxy_cache",
		DnsRefreshRate:  durationpb.New(60 * time.Second),
		HostTtl:         durationpb.New(300 * time.Second),
		DnsLookupFamily: cluster.Cluster_V4_PREFERRED,
		MaxHosts:        &wrapperspb.UInt32Value{Value: 1024},
	}

	dfpFilterConfig := &dfpv3.FilterConfig{
		ImplementationSpecifier: &dfpv3.FilterConfig_DnsCacheConfig{
			DnsCacheConfig: dnsCacheConfig,
		},
	}
	dynamicFwdAny, err := anypb.New(dfpFilterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dynamic forward proxy config: %w", err)
	}

	routerCfg := &router.Router{}
	routerAny, err := anypb.New(routerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal router config: %w", err)
	}

	httpFilters = append(httpFilters, &hcm.HttpFilter{
		Name: dynamicForwardProxyFilterName,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: dynamicFwdAny,
		},
	})

	httpFilters = append(httpFilters, &hcm.HttpFilter{
		Name: wellknown.Router,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: routerAny,
		},
	})

	httpConnManager := &hcm.HttpConnectionManager{
		CodecType:         hcm.HttpConnectionManager_AUTO,
		StatPrefix:        "http",
		GenerateRequestId: wrapperspb.Bool(true),
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: dynamicForwardProxyRouteConfig,
		},
		HttpFilters: httpFilters,
	}

	if t.RouterConfig().AccessLogs.Enabled {
		accessLogs, err := t.CreateAccessLogConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create access log config: %w", err)
		}
		httpConnManager.AccessLog = accessLogs
	}

	tracingCfgDFP, err := t.CreateTracingConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create tracing config: %w", err)
	}
	if tracingCfgDFP != nil {
		httpConnManager.Tracing = tracingCfgDFP
	}

	pbst, err := anypb.New(httpConnManager)
	if err != nil {
		return nil, err
	}

	var listenerName string
	var port uint32
	if isHTTPS {
		listenerName = fmt.Sprintf("listener_https_%d", constants.WEBSUB_HUB_DYNAMIC_HTTPS_PORT)
		port = uint32(constants.WEBSUB_HUB_DYNAMIC_HTTPS_PORT)
	} else {
		listenerName = fmt.Sprintf("listener_http_%d", constants.WEBSUB_HUB_DYNAMIC_HTTP_PORT)
		port = uint32(constants.WEBSUB_HUB_DYNAMIC_HTTP_PORT)
	}

	filterChain := &listener.FilterChain{
		Filters: []*listener.Filter{{
			Name: wellknown.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{
				TypedConfig: pbst,
			},
		}},
	}

	return &listener.Listener{
		Name: listenerName,
		Address: &core.Address{Address: &core.Address_SocketAddress{SocketAddress: &core.SocketAddress{
			Protocol:      core.SocketAddress_TCP,
			Address:       "0.0.0.0",
			PortSpecifier: &core.SocketAddress_PortValue{PortValue: port},
		}}},
		FilterChains: []*listener.FilterChain{filterChain},
	}, nil
}

func (h *Hooks) createDynamicForwardProxyCluster(t *xds.Translator) *cluster.Cluster {
	clusterConfig := &dfpcluster.ClusterConfig{}
	clusterTypeAny, err := anypb.New(clusterConfig)
	if err != nil {
		t.Logger().Error("Failed to marshal dynamic forward proxy cluster config", slog.Any("error", err))
		return nil
	}

	return &cluster.Cluster{
		Name:           DynamicForwardProxyClusterName,
		ConnectTimeout: durationpb.New(5 * time.Second),
		LbPolicy:       cluster.Cluster_CLUSTER_PROVIDED,
		ClusterDiscoveryType: &cluster.Cluster_ClusterType{
			ClusterType: &cluster.Cluster_CustomClusterType{
				Name:        "envoy.clusters.dynamic_forward_proxy",
				TypedConfig: clusterTypeAny,
			},
		},
		UpstreamConnectionOptions: &cluster.UpstreamConnectionOptions{
			TcpKeepalive: &core.TcpKeepalive{
				KeepaliveTime: &wrapperspb.UInt32Value{Value: 300},
			},
		},
	}
}

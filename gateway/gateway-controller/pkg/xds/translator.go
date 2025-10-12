package xds

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Translator converts API configurations to Envoy xDS resources
type Translator struct {
	logger *zap.Logger
}

// NewTranslator creates a new translator
func NewTranslator(logger *zap.Logger) *Translator {
	return &Translator{
		logger: logger,
	}
}

// TranslateConfigs translates all API configurations to Envoy resources
func (t *Translator) TranslateConfigs(configs []*models.StoredAPIConfig) (map[resource.Type][]types.Resource, error) {
	resources := make(map[resource.Type][]types.Resource)

	var listeners []types.Resource
	var routes []types.Resource
	var clusters []types.Resource

	// We'll use a single listener on port 8080 with multiple virtual hosts
	virtualHosts := make([]*route.VirtualHost, 0)
	clusterMap := make(map[string]*cluster.Cluster)

	for _, cfg := range configs {
		if cfg.Status != models.StatusDeployed {
			continue
		}

		// Create virtual host for this API
		vh, clusterList, err := t.translateAPIConfig(cfg)
		if err != nil {
			t.logger.Error("Failed to translate config",
				zap.String("id", cfg.ID),
				zap.String("name", cfg.GetAPIName()),
				zap.Error(err))
			continue
		}

		virtualHosts = append(virtualHosts, vh)

		// Add clusters (avoiding duplicates)
		for _, c := range clusterList {
			clusterMap[c.Name] = c
		}
	}

	// Create single listener with all virtual hosts
	if len(virtualHosts) > 0 {
		l, err := t.createListener(virtualHosts)
		if err != nil {
			return nil, fmt.Errorf("failed to create listener: %w", err)
		}
		listeners = append(listeners, l)
	}

	// Create route configuration
	if len(virtualHosts) > 0 {
		r := t.createRouteConfiguration(virtualHosts)
		routes = append(routes, r)
	}

	// Add all clusters
	for _, c := range clusterMap {
		clusters = append(clusters, c)
	}

	resources[resource.ListenerType] = listeners
	resources[resource.RouteType] = routes
	resources[resource.ClusterType] = clusters

	return resources, nil
}

// translateAPIConfig translates a single API configuration
func (t *Translator) translateAPIConfig(cfg *models.StoredAPIConfig) (*route.VirtualHost, []*cluster.Cluster, error) {
	apiData := cfg.Configuration.Data

	// Parse upstream URL
	if len(apiData.Upstream) == 0 {
		return nil, nil, fmt.Errorf("no upstream configured")
	}

	upstreamURL := apiData.Upstream[0].Url
	parsedURL, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid upstream URL: %w", err)
	}

	// Create cluster for this upstream
	clusterName := t.sanitizeClusterName(parsedURL.Host)
	c := t.createCluster(clusterName, parsedURL)

	// Create routes for each operation
	routesList := make([]*route.Route, 0)
	for _, op := range apiData.Operations {
		r := t.createRoute(string(op.Method), apiData.Context+op.Path, clusterName, parsedURL.Path)
		routesList = append(routesList, r)
	}

	// Create virtual host
	vh := &route.VirtualHost{
		Name:    fmt.Sprintf("vh_%s_%s", t.sanitizeName(apiData.Name), apiData.Version),
		Domains: []string{"*"},
		Routes:  routesList,
	}

	return vh, []*cluster.Cluster{c}, nil
}

// createListener creates an Envoy listener
func (t *Translator) createListener(virtualHosts []*route.VirtualHost) (*listener.Listener, error) {
	routeConfig := t.createRouteConfiguration(virtualHosts)

	// Create HTTP connection manager
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: "http",
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: routeConfig,
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name: wellknown.Router,
		}},
	}

	pbst, err := anypb.New(manager)
	if err != nil {
		return nil, err
	}

	return &listener.Listener{
		Name: "listener_http_8080",
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: 8080,
					},
				},
			},
		},
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: pbst,
				},
			}},
		}},
	}, nil
}

// createRouteConfiguration creates a route configuration
func (t *Translator) createRouteConfiguration(virtualHosts []*route.VirtualHost) *route.RouteConfiguration {
	return &route.RouteConfiguration{
		Name:         "local_route",
		VirtualHosts: virtualHosts,
	}
}

// createRoute creates a route for an operation
func (t *Translator) createRoute(method, path, clusterName, upstreamPath string) *route.Route {
	r := &route.Route{
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Path{
				Path: path,
			},
			Headers: []*route.HeaderMatcher{{
				Name: ":method",
				HeaderMatchSpecifier: &route.HeaderMatcher_ExactMatch{
					ExactMatch: method,
				},
			}},
		},
		Action: &route.Route_Route{
			Route: &route.RouteAction{
				ClusterSpecifier: &route.RouteAction_Cluster{
					Cluster: clusterName,
				},
			},
		},
	}

	// Add path rewriting if upstream has a path prefix
	if upstreamPath != "" && upstreamPath != "/" {
		r.GetRoute().PrefixRewrite = upstreamPath
	}

	return r
}

// createCluster creates an Envoy cluster
func (t *Translator) createCluster(name string, upstreamURL *url.URL) *cluster.Cluster {
	port := uint32(80)
	if upstreamURL.Scheme == "https" {
		port = 443
	}
	if upstreamURL.Port() != "" {
		fmt.Sscanf(upstreamURL.Port(), "%d", &port)
	}

	return &cluster.Cluster{
		Name:                 name,
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STRICT_DNS},
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: name,
			Endpoints: []*endpoint.LocalityLbEndpoints{{
				LbEndpoints: []*endpoint.LbEndpoint{{
					HostIdentifier: &endpoint.LbEndpoint_Endpoint{
						Endpoint: &endpoint.Endpoint{
							Address: &core.Address{
								Address: &core.Address_SocketAddress{
									SocketAddress: &core.SocketAddress{
										Protocol: core.SocketAddress_TCP,
										Address:  upstreamURL.Hostname(),
										PortSpecifier: &core.SocketAddress_PortValue{
											PortValue: port,
										},
									},
								},
							},
						},
					},
				}},
			}},
		},
	}
}

// pathToRegex converts a path with parameters to a regex pattern
func (t *Translator) pathToRegex(path string) string {
	// Replace path parameters with regex patterns
	regex := path
	regex = strings.ReplaceAll(regex, "/", "\\/")
	// Replace {param} with named capture group
	regex = strings.ReplaceAll(regex, "{", "(?P<")
	regex = strings.ReplaceAll(regex, "}", ">[^/]+)")
	return "^" + regex + "$"
}

// convertPathParameters converts {param} to Envoy format
func (t *Translator) convertPathParameters(path string) string {
	// Envoy uses the same {param} format, so we just return as-is
	return path
}

// sanitizeClusterName creates a valid cluster name from a hostname
func (t *Translator) sanitizeClusterName(hostname string) string {
	name := strings.ReplaceAll(hostname, ".", "_")
	name = strings.ReplaceAll(name, ":", "_")
	return "cluster_" + name
}

// sanitizeName creates a valid name from an API name
func (t *Translator) sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return name
}

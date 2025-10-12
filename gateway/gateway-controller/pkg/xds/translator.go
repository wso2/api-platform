package xds

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	fileaccesslog "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	router "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
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
		if cfg.Status == models.StatusDeployed {
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
	// Note: Route configuration is embedded inline in the listener,
	// so we don't add it as a standalone resource
	if len(virtualHosts) > 0 {
		l, err := t.createListener(virtualHosts)
		if err != nil {
			return nil, fmt.Errorf("failed to create listener: %w", err)
		}
		listeners = append(listeners, l)
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

// createListener creates an Envoy listener with access logging
func (t *Translator) createListener(virtualHosts []*route.VirtualHost) (*listener.Listener, error) {
	routeConfig := t.createRouteConfiguration(virtualHosts)

	// Create router filter with typed config
	routerConfig := &router.Router{}
	routerAny, err := anypb.New(routerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create router config: %w", err)
	}

	// Create access log configuration
	accessLogs, err := t.createAccessLogConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create access log config: %w", err)
	}

	// Create HTTP connection manager
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: "http",
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: routeConfig,
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name: wellknown.Router,
			ConfigType: &hcm.HttpFilter_TypedConfig{
				TypedConfig: routerAny,
			},
		}},
		AccessLog: accessLogs,
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
	// Check if path contains parameters (e.g., {country_code})
	hasParams := strings.Contains(path, "{")

	var pathSpecifier *route.RouteMatch_SafeRegex
	if hasParams {
		// Use regex matching for parameterized paths
		regexPattern := t.pathToRegex(path)
		pathSpecifier = &route.RouteMatch_SafeRegex{
			SafeRegex: &matcher.RegexMatcher{
				Regex: regexPattern,
			},
		}
	}

	r := &route.Route{
		Match: &route.RouteMatch{
			Headers: []*route.HeaderMatcher{{
				Name: ":method",
				HeaderMatchSpecifier: &route.HeaderMatcher_StringMatch{
					StringMatch: &matcher.StringMatcher{
						MatchPattern: &matcher.StringMatcher_Exact{
							Exact: method,
						},
					},
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

	// Set path specifier based on whether we have parameters
	if hasParams {
		r.Match.PathSpecifier = pathSpecifier
	} else {
		// Use exact path matching for non-parameterized paths
		r.Match.PathSpecifier = &route.RouteMatch_Path{
			Path: path,
		}
	}

	// Add path rewriting if upstream has a path prefix
	// The upstream path should be prepended to the full request path
	// For example: request /weather/US/Seattle with upstream /api/v2
	// should result in /api/v2/weather/US/Seattle
	if upstreamPath != "" && upstreamPath != "/" {
		// Use RegexRewrite to prepend the upstream path to the full request path
		r.GetRoute().RegexRewrite = &matcher.RegexMatchAndSubstitute{
			Pattern: &matcher.RegexMatcher{
				Regex: "^(.*)$",
			},
			Substitution: upstreamPath + "\\1",
		}
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
// Converts paths like /{country_code}/{city} to ^/[^/]+/[^/]+$
func (t *Translator) pathToRegex(path string) string {
	// Escape special regex characters in the path, except for {}
	regex := path

	// Replace {param} with a pattern that matches any non-slash characters
	// This handles parameters like {country_code}, {city}, etc.
	for strings.Contains(regex, "{") {
		start := strings.Index(regex, "{")
		end := strings.Index(regex, "}")
		if end > start {
			// Replace {paramName} with [^/]+ (matches one or more non-slash chars)
			regex = regex[:start] + "[^/]+" + regex[end+1:]
		} else {
			break
		}
	}

	// Anchor the regex to match the entire path
	return "^" + regex + "$"
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

// createAccessLogConfig creates access log configuration with JSON format to stdout
func (t *Translator) createAccessLogConfig() ([]*accesslog.AccessLog, error) {
	// Define JSON log format with standard fields
	jsonFormat := map[string]string{
		"start_time":             "%START_TIME%",
		"method":                 "%REQ(:METHOD)%",
		"path":                   "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%",
		"protocol":               "%PROTOCOL%",
		"response_code":          "%RESPONSE_CODE%",
		"response_flags":         "%RESPONSE_FLAGS%",
		"bytes_received":         "%BYTES_RECEIVED%",
		"bytes_sent":             "%BYTES_SENT%",
		"duration":               "%DURATION%",
		"upstream_service_time":  "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%",
		"x_forwarded_for":        "%REQ(X-FORWARDED-FOR)%",
		"user_agent":             "%REQ(USER-AGENT)%",
		"request_id":             "%REQ(X-REQUEST-ID)%",
		"authority":              "%REQ(:AUTHORITY)%",
		"upstream_host":          "%UPSTREAM_HOST%",
		"upstream_cluster":       "%UPSTREAM_CLUSTER%",
	}

	// Convert to structpb.Struct
	jsonStruct, err := structpb.NewStruct(convertToInterface(jsonFormat))
	if err != nil {
		return nil, fmt.Errorf("failed to create json struct: %w", err)
	}

	// Create file access log configuration
	fileAccessLog := &fileaccesslog.FileAccessLog{
		Path: "/dev/stdout",
		AccessLogFormat: &fileaccesslog.FileAccessLog_LogFormat{
			LogFormat: &core.SubstitutionFormatString{
				Format: &core.SubstitutionFormatString_JsonFormat{
					JsonFormat: jsonStruct,
				},
			},
		},
	}

	// Marshal to Any
	accessLogAny, err := anypb.New(fileAccessLog)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal access log config: %w", err)
	}

	return []*accesslog.AccessLog{{
		Name: "envoy.access_loggers.file",
		ConfigType: &accesslog.AccessLog_TypedConfig{
			TypedConfig: accessLogAny,
		},
	}}, nil
}

// convertToInterface converts map[string]string to map[string]interface{} for structpb
func convertToInterface(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	return result
}

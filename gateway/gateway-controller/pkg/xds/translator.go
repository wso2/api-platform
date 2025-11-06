/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// Translator converts API configurations to Envoy xDS resources
type Translator struct {
	logger          *zap.Logger
	accessLogConfig config.AccessLogsConfig
}

// NewTranslator creates a new translator
func NewTranslator(logger *zap.Logger, accessLogConfig config.AccessLogsConfig) *Translator {
	return &Translator{
		logger:          logger,
		accessLogConfig: accessLogConfig,
	}
}

// TranslateConfigs translates all API configurations to Envoy resources
// The correlationID parameter is optional and used for request tracing in logs
func (t *Translator) TranslateConfigs(configs []*models.StoredAPIConfig, correlationID string) (map[resource.Type][]types.Resource, error) {
	// Create a logger with correlation ID if provided
	log := t.logger
	if correlationID != "" {
		log = t.logger.With(zap.String("correlation_id", correlationID))
	}
	resources := make(map[resource.Type][]types.Resource)

	var listeners []types.Resource
	var routes []types.Resource
	var clusters []types.Resource

	// We'll use a single listener on port 8080 with a single virtual host
	// All API routes are consolidated into one virtual host to avoid wildcard domain conflicts
	allRoutes := make([]*route.Route, 0)
	clusterMap := make(map[string]*cluster.Cluster)

	for _, cfg := range configs {
		// Include ALL configs (both deployed and pending) in the snapshot
		// This ensures existing APIs are not overridden when deploying new APIs

		// Create routes and clusters for this API
		routesList, clusterList, err := t.translateAPIConfig(cfg)
		if err != nil {
			log.Error("Failed to translate config",
				zap.String("id", cfg.ID),
				zap.String("name", cfg.GetAPIName()),
				zap.Error(err))
			continue
		}

		allRoutes = append(allRoutes, routesList...)

		// Add clusters (avoiding duplicates)
		for _, c := range clusterList {
			clusterMap[c.Name] = c
		}
	}

	// Add a catch-all route that returns 404 for unmatched requests
	// This should be the last route (lowest priority)
	allRoutes = append(allRoutes, &route.Route{
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: "/",
			},
		},
		Action: &route.Route_DirectResponse{
			DirectResponse: &route.DirectResponseAction{
				Status: 404,
			},
		},
	})

	// Create a single virtual host with all routes
	virtualHost := &route.VirtualHost{
		Name:    "all_apis",
		Domains: []string{"*"},
		Routes:  allRoutes,
	}

	// Always create the listener, even with no APIs deployed
	l, err := t.createListener([]*route.VirtualHost{virtualHost})
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}
	listeners = append(listeners, l)

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
func (t *Translator) translateAPIConfig(cfg *models.StoredAPIConfig) ([]*route.Route, []*cluster.Cluster, error) {
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
		r := t.createRoute(apiData.Context, string(op.Method), op.Path, clusterName, parsedURL.Path)
		routesList = append(routesList, r)
	}

	return routesList, []*cluster.Cluster{c}, nil
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
	}

	// Add access logs if enabled
	if t.accessLogConfig.Enabled {
		accessLogs, err := t.createAccessLogConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create access log config: %w", err)
		}
		manager.AccessLog = accessLogs
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
func (t *Translator) createRoute(context, method, path, clusterName, upstreamPath string) *route.Route {
	// Combine context and path for matching
	fullPath := context + path

	// Check if path contains parameters (e.g., {country_code})
	hasParams := strings.Contains(path, "{")

	var pathSpecifier *route.RouteMatch_SafeRegex
	if hasParams {
		// Use regex matching for parameterized paths
		regexPattern := t.pathToRegex(fullPath)
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
			Path: fullPath,
		}
	}

	// Add path rewriting if upstream has a path prefix
	// Strip the API context and prepend the upstream path
	// For example: request /weather/US/Seattle with context /weather and upstream /api/v2
	// should result in /api/v2/US/Seattle (context stripped, upstream prepended)
	if upstreamPath != "" && upstreamPath != "/" {
		// Use RegexRewrite to strip the context and prepend the upstream path
		// Pattern captures everything after the context
		r.GetRoute().RegexRewrite = &matcher.RegexMatchAndSubstitute{
			Pattern: &matcher.RegexMatcher{
				Regex: "^" + context + "(.*)$",
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

// createAccessLogConfig creates access log configuration based on format (JSON or text) to stdout
func (t *Translator) createAccessLogConfig() ([]*accesslog.AccessLog, error) {
	var fileAccessLog *fileaccesslog.FileAccessLog

	if t.accessLogConfig.Format == "json" {
		// Use JSON log format fields from config
		jsonFormat := t.accessLogConfig.JSONFields
		if jsonFormat == nil || len(jsonFormat) == 0 {
			return nil, fmt.Errorf("json_fields not configured in access log config")
		}

		// Convert to structpb.Struct
		jsonStruct, err := structpb.NewStruct(convertToInterface(jsonFormat))
		if err != nil {
			return nil, fmt.Errorf("failed to create json struct: %w", err)
		}

		fileAccessLog = &fileaccesslog.FileAccessLog{
			Path: "/dev/stdout",
			AccessLogFormat: &fileaccesslog.FileAccessLog_LogFormat{
				LogFormat: &core.SubstitutionFormatString{
					Format: &core.SubstitutionFormatString_JsonFormat{
						JsonFormat: jsonStruct,
					},
				},
			},
		}
	} else {
		// Use text format from config
		textFormat := t.accessLogConfig.TextFormat
		if textFormat == "" {
			return nil, fmt.Errorf("text_format not configured in access log config")
		}

		fileAccessLog = &fileaccesslog.FileAccessLog{
			Path: "/dev/stdout",
			AccessLogFormat: &fileaccesslog.FileAccessLog_LogFormat{
				LogFormat: &core.SubstitutionFormatString{
					Format: &core.SubstitutionFormatString_TextFormatSource{
						TextFormatSource: &core.DataSource{
							Specifier: &core.DataSource_InlineString{
								InlineString: textFormat,
							},
						},
					},
				},
			},
		}
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

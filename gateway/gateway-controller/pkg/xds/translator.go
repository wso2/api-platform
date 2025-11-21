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
	"net"
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
	router "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Translator converts API configurations to Envoy xDS resources
type Translator struct {
	logger       *zap.Logger
	routerConfig *config.RouterConfig
}

// NewTranslator creates a new translator
func NewTranslator(logger *zap.Logger, routerConfig *config.RouterConfig) *Translator {
	return &Translator{
		logger:       logger,
		routerConfig: routerConfig,
	}
}

// TranslateConfigs translates all API configurations to Envoy resources
// The correlationID parameter is optional and used for request tracing in logs
func (t *Translator) TranslateConfigs(
	configs []*models.StoredAPIConfig,
	correlationID string,
) (map[resource.Type][]types.Resource, error) {
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
	// @TODO: Handle upstream certificates and pass them to createCluster
	c := t.createCluster(clusterName, parsedURL, nil)

	// Create routes for each operation
	routesList := make([]*route.Route, 0)
	for _, op := range apiData.Operations {
		// Build route key with method, version, context, and path to correlate with policies
		// Format: METHOD|API_VERSION|CONTEXT+PATH (same as handlers/buildStoredPolicyFromAPI)
		routeKey := fmt.Sprintf("%s|%s|%s%s", op.Method, apiData.Version, apiData.Context, op.Path)
		r := t.createRoute(apiData.Name, apiData.Version, apiData.Context, string(op.Method), op.Path, clusterName, parsedURL.Path, routeKey)
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
	if t.routerConfig.AccessLogs.Enabled {
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
func (t *Translator) createRoute(apiName, apiVersion, context, method, path, clusterName, upstreamPath, routeKey string) *route.Route {
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
				HostRewriteSpecifier: &route.RouteAction_AutoHostRewrite{
					AutoHostRewrite: &wrapperspb.BoolValue{
						Value: true,
					},
				},
				Timeout: durationpb.New(
					time.Duration(t.routerConfig.Upstream.Timeouts.RouteTimeoutInSeconds) * time.Second,
				),
				IdleTimeout: durationpb.New(
					time.Duration(t.routerConfig.Upstream.Timeouts.RouteIdleTimeoutInSeconds) * time.Second,
				),
				ClusterSpecifier: &route.RouteAction_Cluster{
					Cluster: clusterName,
				},
			},
		},
	}

	// Attach dynamic metadata for downstream correlation (policies, logging, tracing)
	metaMap := map[string]interface{}{
		"route_key":   routeKey,
		"api_name":    apiName,
		"api_version": apiVersion,
		"api_context": context,
		"path":        path,
		"method":      method,
	}
	if metaStruct, err := structpb.NewStruct(metaMap); err == nil {
		r.Metadata = &core.Metadata{FilterMetadata: map[string]*structpb.Struct{
			"wso2.route": metaStruct,
		}}
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
func (t *Translator) createCluster(
	name string,
	upstreamURL *url.URL,
	upstreamCerts map[string][]byte,
) *cluster.Cluster {
	endpoints, transportSocketMatch := t.processEndpoint(upstreamURL, upstreamCerts)

	c := &cluster.Cluster{
		Name:                 name,
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STRICT_DNS},
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: name,
			Endpoints:   endpoints,
		},
	}

	if transportSocketMatch != nil {
		c.TransportSocketMatches = []*cluster.Cluster_TransportSocketMatch{transportSocketMatch}
	}

	return c
}

// createUpstreamTLSContext creates an upstream TLS context for secure connections
func (t *Translator) createUpstreamTLSContext(certificate []byte, address string) *tlsv3.UpstreamTlsContext {
	// Create TLS context with base configuration
	upstreamTLSContext := &tlsv3.UpstreamTlsContext{
		CommonTlsContext: &tlsv3.CommonTlsContext{
			TlsParams: &tlsv3.TlsParameters{
				TlsMinimumProtocolVersion: t.createTLSProtocolVersion(
					t.routerConfig.Upstream.TLS.MinimumProtocolVersion,
				),
				TlsMaximumProtocolVersion: t.createTLSProtocolVersion(
					t.routerConfig.Upstream.TLS.MaximumProtocolVersion,
				),
				CipherSuites: t.parseCipherSuites(t.routerConfig.Upstream.TLS.Ciphers),
			},
		},
	}

	// Determine if address is IP or hostname and set SNI accordingly
	isIP := net.ParseIP(address) != nil
	if !isIP {
		upstreamTLSContext.Sni = address
	}

	// Configure SSL verification unless disabled
	if !t.routerConfig.Upstream.TLS.DisableSslVerification {
		var trustedCASource *core.DataSource
		if len(certificate) > 0 {
			trustedCASource = &core.DataSource{
				Specifier: &core.DataSource_InlineBytes{
					InlineBytes: certificate,
				},
			}
		} else if t.routerConfig.Upstream.TLS.TrustedCertPath != "" {
			trustedCASource = &core.DataSource{
				Specifier: &core.DataSource_Filename{
					Filename: t.routerConfig.Upstream.TLS.TrustedCertPath,
				},
			}
		}

		// Set trusted CA for validation if provided. Otherwise, Envoy will fall back to the system default trust store.
		if trustedCASource != nil {
			upstreamTLSContext.CommonTlsContext.ValidationContextType = &tlsv3.CommonTlsContext_ValidationContext{
				ValidationContext: &tlsv3.CertificateValidationContext{
					TrustedCa: trustedCASource,
				},
			}
		}

		// Add hostname verification if enabled
		if t.routerConfig.Upstream.TLS.VerifyHostName {
			sanType := tlsv3.SubjectAltNameMatcher_DNS
			if isIP {
				sanType = tlsv3.SubjectAltNameMatcher_IP_ADDRESS
			}

			if validationContext := upstreamTLSContext.CommonTlsContext.GetValidationContext(); validationContext != nil {
				validationContext.MatchTypedSubjectAltNames = []*tlsv3.SubjectAltNameMatcher{
					{
						SanType: sanType,
						Matcher: &matcher.StringMatcher{
							MatchPattern: &matcher.StringMatcher_Exact{
								Exact: address,
							},
						},
					},
				}
			}
		}
	}

	return upstreamTLSContext
}

// createTLSProtocolVersion converts string TLS version to Envoy TLS version enum
func (t *Translator) createTLSProtocolVersion(version string) tlsv3.TlsParameters_TlsProtocol {
	switch strings.ToUpper(version) {
	case constants.TLSVersion10:
		return tlsv3.TlsParameters_TLSv1_0
	case constants.TLSVersion11:
		return tlsv3.TlsParameters_TLSv1_1
	case constants.TLSVersion12:
		return tlsv3.TlsParameters_TLSv1_2
	case constants.TLSVersion13:
		return tlsv3.TlsParameters_TLSv1_3
	default:
		return tlsv3.TlsParameters_TLS_AUTO
	}
}

// parseCipherSuites splits and trims cipher suite string into array
func (t *Translator) parseCipherSuites(ciphers string) []string {
	if ciphers == "" {
		return nil
	}
	ciphersList := strings.Split(ciphers, constants.CipherSuiteSeparator)
	for i := range ciphersList {
		ciphersList[i] = strings.TrimSpace(ciphersList[i])
	}
	return ciphersList
}

// processEndpoint creates locality load endpoints for the given upstream URL and returns both endpoints and transport socket match if TLS is enabled
func (t *Translator) processEndpoint(
	upstreamURL *url.URL,
	upstreamCerts map[string][]byte,
) ([]*endpoint.LocalityLbEndpoints, *cluster.Cluster_TransportSocketMatch) {
	port := constants.HTTPDefaultPort
	if upstreamURL.Scheme == constants.SchemeHTTPS {
		port = constants.HTTPSDefaultPort
	}
	if upstreamURL.Port() != "" {
		var parsedPort uint32
		if _, err := fmt.Sscanf(upstreamURL.Port(), "%d", &parsedPort); err == nil {
			port = parsedPort
		}
	}

	localityLbEndpoints := &endpoint.LocalityLbEndpoints{
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
	}

	if upstreamURL.Scheme == constants.SchemeHTTPS {
		var epCert []byte
		if cert, found := upstreamCerts[upstreamURL.String()]; found {
			epCert = cert
		} else if defaultCerts, found := upstreamCerts[constants.DefaultCertificateKey]; found {
			epCert = defaultCerts
		}

		upstreamtlsContext := t.createUpstreamTLSContext(epCert, upstreamURL.Hostname())
		marshalledTLSContext, err := anypb.New(upstreamtlsContext)
		if err != nil {
			t.logger.Error("internal Error while marshalling the upstream TLS Context", zap.Error(err))
			return []*endpoint.LocalityLbEndpoints{localityLbEndpoints}, nil
		}

		// Create transport socket match with a unique identifier
		// We use index 0 since we're dealing with a single endpoint
		matchID := constants.DefaultMatchID
		transportSocketMatch := &cluster.Cluster_TransportSocketMatch{
			// Name format: ts0 (transport socket + match ID)
			Name: constants.TransportSocketPrefix + matchID,
			Match: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					// The lb_id field is used to match the endpoint with its transport socket
					constants.LoadBalancerIDKey: structpb.NewStringValue(matchID),
				},
			},
			TransportSocket: &core.TransportSocket{
				Name: constants.EnvoyTLSTransportSocket,
				ConfigType: &core.TransportSocket_TypedConfig{
					TypedConfig: marshalledTLSContext,
				},
			},
		}

		// Set metadata for transport socket matching
		// This metadata links the endpoint to its transport socket configuration
		localityLbEndpoints.LbEndpoints[0].Metadata = &core.Metadata{
			FilterMetadata: map[string]*structpb.Struct{
				constants.TransportSocketMatchKey: {
					Fields: map[string]*structpb.Value{
						constants.LoadBalancerIDKey: structpb.NewStringValue(matchID),
					},
				},
			},
		}

		return []*endpoint.LocalityLbEndpoints{localityLbEndpoints}, transportSocketMatch
	}

	return []*endpoint.LocalityLbEndpoints{localityLbEndpoints}, nil
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

	if t.routerConfig.AccessLogs.Format == "json" {
		// Use JSON log format fields from config
		jsonFormat := t.routerConfig.AccessLogs.JSONFields
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
		textFormat := t.routerConfig.AccessLogs.TextFormat
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

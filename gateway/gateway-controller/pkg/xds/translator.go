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
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	commonconstants "github.com/wso2/api-platform/common/constants"

	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	mutationrules "github.com/envoyproxy/go-control-plane/envoy/config/common/mutation_rules/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	tracev3 "github.com/envoyproxy/go-control-plane/envoy/config/trace/v3"
	fileaccesslog "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	grpc_accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/grpc/v3"
	dfpcluster "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/dynamic_forward_proxy/v3"
	common_dfp "github.com/envoyproxy/go-control-plane/envoy/extensions/common/dynamic_forward_proxy/v3"
	dfpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/dynamic_forward_proxy/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	luav3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/lua/v3"
	router "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/certstore"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	anypb "google.golang.org/protobuf/types/known/anypb"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	DynamicForwardProxyClusterName          = "dynamic-forward-proxy-cluster"
	ExternalProcessorGRPCServiceClusterName = "ext-processor-grpc-service"
	OTELCollectorClusterName                = "otel_collector"
	WebSubHubInternalClusterName            = "WEBSUBHUB_INTERNAL_CLUSTER"
)

// Translator converts API configurations to Envoy xDS resources
type Translator struct {
	logger       *slog.Logger
	routerConfig *config.RouterConfig
	certStore    *certstore.CertStore
	config       *config.Config
}

// resolvedTimeout represents parsed timeout values for an upstream.
// All fields are optional; nil means \"no override\" (use global defaults).
type resolvedTimeout struct {
	Connect *time.Duration
	Request *time.Duration
	Idle    *time.Duration
}

// NewTranslator creates a new translator
func NewTranslator(logger *slog.Logger, routerConfig *config.RouterConfig, db storage.Storage, config *config.Config) *Translator {
	// Initialize certificate store if custom certs path is configured
	var cs *certstore.CertStore
	if routerConfig.Upstream.TLS.CustomCertsPath != "" {
		cs = certstore.NewCertStore(
			logger,
			db,
			routerConfig.Upstream.TLS.CustomCertsPath,
			routerConfig.Upstream.TLS.TrustedCertPath,
		)

		// Load certificates at initialization
		if _, err := cs.LoadCertificates(); err != nil {
			logger.Warn("Failed to initialize certificate store, will use system certs only",
				slog.String("custom_certs_path", routerConfig.Upstream.TLS.CustomCertsPath),
				slog.Any("error", err))
			cs = nil // Don't use cert store if initialization failed
		}
	}

	return &Translator{
		logger:       logger,
		routerConfig: routerConfig,
		certStore:    cs,
		config:       config,
	}
}

// convertServerHeaderTransformation converts string configuration values to Envoy enum values
func convertServerHeaderTransformation(transformation string) hcm.HttpConnectionManager_ServerHeaderTransformation {
	switch transformation {
	case commonconstants.APPEND_IF_ABSENT:
		return hcm.HttpConnectionManager_APPEND_IF_ABSENT
	case commonconstants.OVERWRITE:
		return hcm.HttpConnectionManager_OVERWRITE
	case commonconstants.PASS_THROUGH:
		return hcm.HttpConnectionManager_PASS_THROUGH
	default:
		// Default to OVERWRITE if unknown value
		return hcm.HttpConnectionManager_OVERWRITE
	}
}

// GenerateRouteName creates a unique route name in the format: HttpMethod|RoutePath|Vhost
// This format is used by both Envoy routes and the policy engine for route matching
// It builds the full path by combining context, version, and path using ConstructFullPath
func GenerateRouteName(method, context, apiVersion, path, vhost string) string {
	fullPath := ConstructFullPath(context, apiVersion, path)
	return fmt.Sprintf("%s|%s|%s", method, fullPath, vhost)
}

// ConstructFullPath builds the full path by replacing $version placeholder in context and appending path
// If context contains $version, it will be replaced with the actual apiVersion value
// Example 1: context=/weather/$version, version=v1.0, path=/us/seattle -> /weather/v1.0/us/seattle
// Example 2: context=/weather, version=v1.0, path=/us/seattle -> /weather/us/seattle
func ConstructFullPath(context, apiVersion, path string) string {
	contextWithVersion := strings.ReplaceAll(context, "$version", apiVersion)
	// Allow root context "/"
	if contextWithVersion == "/" {
		contextWithVersion = ""
	}
	return contextWithVersion + path
}

// GetCertStore returns the certificate store instance
func (t *Translator) GetCertStore() *certstore.CertStore {
	return t.certStore
}

// TranslateConfigs translates all API configurations to Envoy resources
// The correlationID parameter is optional and used for request tracing in logs
func (t *Translator) TranslateConfigs(
	configs []*models.StoredConfig,
	correlationID string,
) (map[resource.Type][]types.Resource, error) {
	// Create a logger with correlation ID if provided
	log := t.logger
	if correlationID != "" {
		log = t.logger.With(slog.String("correlation_id", correlationID))
	}
	resources := make(map[resource.Type][]types.Resource)

	var listeners []types.Resource
	var clusters []types.Resource

	// We'll use a single listener on port 8080 with a single virtual host
	// All API routes are consolidated into one virtual host to avoid wildcard domain conflicts
	allRoutes := make([]*route.Route, 0)
	clusterMap := make(map[string]*cluster.Cluster)

	for _, cfg := range configs {
		// Include ALL configs (both deployed and pending) in the snapshot
		// This ensures existing APIs are not overridden when deploying new APIs

		// Create routes and clusters for this API
		var routesList []*route.Route
		var clusterList []*cluster.Cluster
		var err error
		if cfg.Configuration.Kind == api.WebSubApi {
			routesList, clusterList, err = t.translateAsyncAPIConfig(cfg, configs)
			if err != nil {
				log.Error("Failed to translate config",
					slog.String("id", cfg.ID),
					slog.String("displayName", cfg.GetDisplayName()),
					slog.Any("error", err))
				continue
			}
		} else {
			routesList, clusterList, err = t.translateAPIConfig(cfg, configs)
			if err != nil {
				log.Error("Failed to translate config",
					slog.String("id", cfg.ID),
					slog.String("displayName", cfg.GetDisplayName()),
					slog.Any("error", err))
				continue
			}
		}

		allRoutes = append(allRoutes, routesList...)
		// Add clusters (avoiding duplicates)
		for _, c := range clusterList {
			clusterMap[c.Name] = c
		}
	}

	// Group routes by vhost
	vhostMap := make(map[string][]*route.Route)

	for _, r := range allRoutes {
		// Extract vhost from route name: "METHOD|PATH|VHOST"
		parts := strings.Split(r.Name, "|")
		if len(parts) != 3 {
			// Routes without proper naming (e.g., catch-all 404) should be added to all vhosts later
			continue // or handle error
		}
		vhost := parts[2]

		vhostMap[vhost] = append(vhostMap[vhost], r)
	}

	// Create a virtual host for each vhost
	var virtualHosts []*route.VirtualHost
	for vhost, routes := range vhostMap {
		// Sort routes by priority (highest priority first) before adding to vhost
		routes = SortRoutesByPriority(routes)

		// Append the catch-all 404 route as the last route for each vhost (lowest priority)
		routes = append(routes, &route.Route{
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
		virtualHost := &route.VirtualHost{
			Name:    vhost,
			Domains: []string{vhost, vhost + ":*"},
			Routes:  routes,
		}
		virtualHosts = append(virtualHosts, virtualHost)
	}

	// Variable to hold the shared route configuration (created once, used by both listeners)
	var sharedRouteConfig *route.RouteConfiguration

	// Always create the HTTP listener, even with no APIs deployed
	httpListener, routeConfig, err := t.createListener(virtualHosts, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP listener: %w", err)
	}
	listeners = append(listeners, httpListener)
	sharedRouteConfig = routeConfig // Save route config for RDS

	// Create HTTPS listener if enabled
	if t.routerConfig.HTTPSEnabled {
		log.Info("HTTPS is enabled, creating HTTPS listener",
			slog.Int("https_port", t.routerConfig.HTTPSPort))
		httpsListener, _, err := t.createListener(virtualHosts, true)
		if err != nil {
			log.Error("Failed to create HTTPS listener", slog.Any("error", err))
			return nil, fmt.Errorf("failed to create HTTPS listener: %w", err)
		}
		log.Info("HTTPS listener created successfully",
			slog.String("listener_name", httpsListener.GetName()))
		listeners = append(listeners, httpsListener)
	} else {
		log.Info("HTTPS is disabled, skipping HTTPS listener creation")
	}

	// Add route configuration for RDS
	var routes []types.Resource
	if sharedRouteConfig != nil {
		routes = append(routes, sharedRouteConfig)
		log.Info("Added shared route configuration for RDS",
			slog.String("route_config_name", sharedRouteConfig.GetName()),
			slog.Int("num_virtual_hosts", len(sharedRouteConfig.GetVirtualHosts())))
	}

	// Add all clusters
	for _, c := range clusterMap {
		clusters = append(clusters, c)
	}

	// Add policy engine cluster if enabled
	if t.routerConfig.PolicyEngine.Enabled {
		policyEngineCluster := t.createPolicyEngineCluster()
		clusters = append(clusters, policyEngineCluster)
	}

	// Add ALS cluster if gRPC access log is enabled
	log.Debug("gRPC access log config", slog.Any("config", t.config.Analytics.GRPCAccessLogCfg))
	if t.config.Analytics.Enabled {
		log.Info("gRPC access log is enabled, creating ALS cluster")
		alsCluster := t.createALSCluster()
		clusters = append(clusters, alsCluster)
	}

	if t.routerConfig.EventGateway.Enabled {
		// Add dynamic forward proxy cluster for WebSubHub
		dynamicForwardProxyCluster := t.createDynamicForwardProxyCluster()
		if dynamicForwardProxyCluster == nil {
			return nil, fmt.Errorf("failed to create dynamic forward proxy cluster")
		}
		clusters = append(clusters, dynamicForwardProxyCluster)
		dynamicProxyListener, err := t.createDynamicFwdListenerForWebSubHub(t.routerConfig.HTTPSEnabled)
		if err != nil {
			return nil, fmt.Errorf("failed to create WebSub listener: %w", err)
		}
		listeners = append(listeners, dynamicProxyListener)

		parsedURL, err := url.Parse(t.routerConfig.EventGateway.WebSubHubURL)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream URL: %w", err)
		}
		if parsedURL.Port() == "" {
			parsedURL.Host = fmt.Sprintf("%s:%d", parsedURL.Hostname(), t.routerConfig.EventGateway.WebSubHubPort)
		}
		if parsedURL.Scheme == "" {
			parsedURL.Scheme = "http"
		}
		websubhubCluster := t.createCluster(constants.WEBSUBHUB_INTERNAL_CLUSTER_NAME, parsedURL, nil)
		clusters = append(clusters, websubhubCluster)
		websubInternalListener, err := t.createInternalListenerForWebSubHub(false)
		if err != nil {
			return nil, fmt.Errorf("failed to create WebSub internal listener: %w", err)
		}
		listeners = append(listeners, websubInternalListener)
		// Create HTTPS listener for WebSubHub communication if enabled
		if t.routerConfig.HTTPSEnabled {
			log.Info("HTTPS is enabled, creating HTTPS listener",
				slog.Int("https_port", t.routerConfig.HTTPSPort))
			httpsListener, err := t.createInternalListenerForWebSubHub(true)
			if err != nil {
				log.Error("Failed to create HTTPS listener", slog.Any("error", err))
				return nil, fmt.Errorf("failed to create HTTPS listener: %w", err)
			}
			log.Info("HTTPS listener created successfully",
				slog.String("listener_name", httpsListener.GetName()))
			listeners = append(listeners, httpsListener)
		} else {
			log.Info("HTTPS is disabled, skipping HTTPS listener creation")
		}
	}

	// Add SDS cluster if cert store is enabled
	// This cluster allows Envoy to fetch certificates from the SDS service
	if t.certStore != nil {
		sdsCluster := t.createSDSCluster()
		clusters = append(clusters, sdsCluster)
	}

	// Add OTEL collector cluster if tracing is enabled
	// This cluster allows Envoy to send traces to OpenTelemetry collector
	if t.config.TracingConfig.Enabled {
		otelCluster := t.createOTELCollectorCluster()
		if otelCluster != nil {
			clusters = append(clusters, otelCluster)
		}
	}

	resources[resource.ListenerType] = listeners
	// Add route configuration for RDS (Route Discovery Service)
	// This allows sharing route config between HTTP and HTTPS listeners
	resources[resource.RouteType] = routes
	resources[resource.ClusterType] = clusters

	log.Info("Translated resources ready for snapshot",
		slog.Int("num_listeners", len(listeners)),
		slog.Int("num_routes", len(routes)),
		slog.Int("num_clusters", len(clusters)))
	for i, l := range listeners {
		if listenerProto, ok := l.(*listener.Listener); ok {
			log.Info("Listener details",
				slog.Int("index", i),
				slog.String("name", listenerProto.GetName()),
				slog.Uint64("port", uint64(listenerProto.GetAddress().GetSocketAddress().GetPortValue())))
		}
	}

	return resources, nil
}

// translateAsyncAPIConfig translates a single API configuration
func (t *Translator) translateAsyncAPIConfig(cfg *models.StoredConfig, allConfigs []*models.StoredConfig) ([]*route.Route, []*cluster.Cluster, error) {
	apiData, err := cfg.Configuration.Spec.AsWebhookAPIData()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse WebSub config data: %w", err)
	}

	clusters := []*cluster.Cluster{}

	mainClusterName := constants.WEBSUBHUB_INTERNAL_CLUSTER_NAME
	parsedMainURL, err := url.Parse(t.routerConfig.EventGateway.WebSubHubURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid upstream URL: %w", err)
	}
	if parsedMainURL.Path == "" {
		parsedMainURL.Path = constants.WEBSUB_PATH
	}

	// Create routes for each operation (default to main cluster)
	routesList := make([]*route.Route, 0)
	mainRoutesList := make([]*route.Route, 0)

	// Determine effective vhosts (fallback to global router defaults when not provided)
	effectiveMainVHost := t.config.GatewayController.Router.VHosts.Main.Default
	if apiData.Vhosts != nil {
		if strings.TrimSpace(apiData.Vhosts.Main) != "" {
			effectiveMainVHost = apiData.Vhosts.Main
		}
	}
	// Extract project ID from labels
	apiProjectID := ""
	if cfg.Configuration.Metadata.Labels != nil {
		if pid, exists := (*cfg.Configuration.Metadata.Labels)["project-id"]; exists {
			apiProjectID = pid
		}
	}

	for _, ch := range apiData.Channels {
		chName := ch.Name
		if !strings.HasPrefix(chName, "/") {
			chName = "/" + chName
		}
		// Use mainClusterName by default; path rewrite based on main upstream path
		r := t.createRoutePerTopic(cfg.ID, apiData.DisplayName, apiData.Version, apiData.Context, string(ch.Method), chName,
			mainClusterName, effectiveMainVHost, cfg.Kind, apiProjectID)
		mainRoutesList = append(mainRoutesList, r)
	}
	// Extract template handle and provider name for LLM provider/proxy scenarios
	templateHandle := t.extractTemplateHandle(cfg, allConfigs)
	providerName := t.extractProviderName(cfg, allConfigs)
	r := t.createRoute(cfg.ID, apiData.DisplayName, apiData.Version, apiData.Context, "POST", constants.WEBSUB_PATH, mainClusterName, "/", effectiveMainVHost, cfg.Kind, templateHandle, providerName, nil, apiProjectID, nil)
	routesList = append(routesList, mainRoutesList...)
	routesList = append(routesList, r)

	return routesList, clusters, nil
}

// translateAPIConfig translates a single API configuration
func (t *Translator) translateAPIConfig(cfg *models.StoredConfig, allConfigs []*models.StoredConfig) ([]*route.Route, []*cluster.Cluster, error) {
	apiData, err := cfg.Configuration.Spec.AsAPIConfigData()
	cfg.GetContext()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse API config data: %w", err)
	}

	clusters := []*cluster.Cluster{}

	// -------- MAIN UPSTREAM --------
	mainClusterName, parsedMainURL, mainTimeout, err := t.resolveUpstreamCluster("main", &apiData.Upstream.Main, apiData.UpstreamDefinitions)
	if err != nil {
		return nil, nil, err
	}
	mainCluster := t.createCluster(mainClusterName, parsedMainURL, nil)
	clusters = append(clusters, mainCluster)

	// Create routes for each operation (default to main cluster)
	routesList := make([]*route.Route, 0)
	mainRoutesList := make([]*route.Route, 0)

	// Determine effective vhosts (fallback to global router defaults when not provided)
	effectiveMainVHost := t.config.GatewayController.Router.VHosts.Main.Default
	effectiveSandboxVHost := t.config.GatewayController.Router.VHosts.Sandbox.Default
	if apiData.Vhosts != nil {
		if strings.TrimSpace(apiData.Vhosts.Main) != "" {
			effectiveMainVHost = apiData.Vhosts.Main
		}
		if apiData.Vhosts.Sandbox != nil && strings.TrimSpace(*apiData.Vhosts.Sandbox) != "" {
			effectiveSandboxVHost = *apiData.Vhosts.Sandbox
		}
	}

	// Extract template handle and provider name for LLM provider/proxy scenarios
	templateHandle := t.extractTemplateHandle(cfg, allConfigs)
	providerName := t.extractProviderName(cfg, allConfigs)

	// Extract project ID from labels
	apiProjectID := ""
	if cfg.Configuration.Metadata.Labels != nil {
		if pid, exists := (*cfg.Configuration.Metadata.Labels)["project-id"]; exists {
			apiProjectID = pid
		}
	}

	for _, op := range apiData.Operations {
		// Use mainClusterName by default; path rewrite based on main upstream path
		r := t.createRoute(cfg.ID, apiData.DisplayName, apiData.Version, apiData.Context, string(op.Method), op.Path,
			mainClusterName, parsedMainURL.Path, effectiveMainVHost, cfg.Kind, templateHandle, providerName, apiData.Upstream.Main.HostRewrite, apiProjectID, mainTimeout)
		mainRoutesList = append(mainRoutesList, r)
	}
	routesList = append(routesList, mainRoutesList...)

	// -------- SANDBOX UPSTREAM --------
	if apiData.Upstream.Sandbox != nil {
		sbClusterName, parsedSbURL, sbTimeout, err := t.resolveUpstreamCluster("sandbox", apiData.Upstream.Sandbox, apiData.UpstreamDefinitions)
		if err != nil {
			return nil, nil, err
		}
		sandboxCluster := t.createCluster(sbClusterName, parsedSbURL, nil)
		clusters = append(clusters, sandboxCluster)

		// Create sandbox routes for each operation
		sbRoutesList := make([]*route.Route, 0)
		for _, op := range apiData.Operations {
			// Use sbClusterName for sandbox upstream path
			r := t.createRoute(cfg.ID, apiData.DisplayName, apiData.Version, apiData.Context, string(op.Method), op.Path,
				sbClusterName, parsedSbURL.Path, effectiveSandboxVHost, cfg.Kind, templateHandle, providerName, apiData.Upstream.Sandbox.HostRewrite, apiProjectID, sbTimeout)
			sbRoutesList = append(sbRoutesList, r)
		}
		routesList = append(routesList, sbRoutesList...)
	}

	return routesList, clusters, nil
}

// resolveUpstreamCluster validates an upstream (main or sandbox) and creates its cluster.
// Returns clusterName, parsedURL, timeout (can be nil), and error.
func (t *Translator) resolveUpstreamCluster(upstreamName string, up *api.Upstream, upstreamDefinitions *[]api.UpstreamDefinition) (string, *url.URL, *resolvedTimeout, error) {
	var rawURL string
	var timeout *resolvedTimeout

	// Resolve URL and timeout
	if up.Url != nil && strings.TrimSpace(*up.Url) != "" {
		// Direct URL provided
		rawURL = strings.TrimSpace(*up.Url)
		// No timeout override when using direct URL
		timeout = nil
	} else if up.Ref != nil && strings.TrimSpace(*up.Ref) != "" {
		// Reference to upstream definition
		refName := strings.TrimSpace(*up.Ref)
		definition, err := resolveUpstreamDefinition(refName, upstreamDefinitions)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to resolve %s upstream ref: %w", upstreamName, err)
		}

		// Extract URL from the first upstream target in the definition
		if len(definition.Upstreams) == 0 || len(definition.Upstreams[0].Urls) == 0 {
			return "", nil, nil, fmt.Errorf("upstream definition '%s' has no URLs configured", refName)
		}
		rawURL = definition.Upstreams[0].Urls[0]

		// Extract timeout if specified in the definition (may be nil or partial)
		if definition.Timeout != nil {
			resolved, err := resolveTimeoutFromDefinition(definition)
			if err != nil {
				return "", nil, nil, fmt.Errorf("invalid timeout in upstream definition '%s': %w", refName, err)
			}
			timeout = resolved
		}
	} else {
		return "", nil, nil, fmt.Errorf("no %s upstream configured", upstreamName)
	}

	// Parse URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", nil, nil, fmt.Errorf("invalid %s upstream URL: %w", upstreamName, err)
	}
	if parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return "", nil, nil, fmt.Errorf("invalid %s upstream URL: must include host and http/https scheme", upstreamName)
	}

	// Generate cluster name
	clusterName := t.sanitizeClusterName(parsedURL.Host, parsedURL.Scheme)

	return clusterName, parsedURL, timeout, nil
}

// SharedRouteConfigName is the name of the shared route configuration used by both HTTP and HTTPS listeners
const SharedRouteConfigName = "shared_route_config"

// createListener creates an Envoy listener with access logging
// If isHTTPS is true, creates an HTTPS listener with TLS configuration
// Uses RDS (Route Discovery Service) to share route configuration between listeners
func (t *Translator) createListener(virtualHosts []*route.VirtualHost, isHTTPS bool) (*listener.Listener, *route.RouteConfiguration, error) {
	routeConfig := t.createRouteConfiguration(virtualHosts)

	// Create router filter with typed config
	routerConfig := &router.Router{}
	routerAny, err := anypb.New(routerConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create router config: %w", err)
	}

	// Build HTTP filters chain
	httpFilters := make([]*hcm.HttpFilter, 0)

	// Add ext_proc filter if policy engine is enabled
	if t.routerConfig.PolicyEngine.Enabled {
		extProcFilter, err := t.createExtProcFilter()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create ext_proc filter: %w", err)
		}
		httpFilters = append(httpFilters, extProcFilter)

		luaFilter, err := t.createLuaFilter()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create lua filter: %w", err)
		}
		httpFilters = append(httpFilters, luaFilter)
	}

	// Add router filter (must be last)
	httpFilters = append(httpFilters, &hcm.HttpFilter{
		Name: wellknown.Router,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: routerAny,
		},
	})

	// Create HTTP connection manager with RDS (Route Discovery Service)
	// This allows route configuration to be shared between HTTP and HTTPS listeners
	manager := &hcm.HttpConnectionManager{
		CodecType:         hcm.HttpConnectionManager_AUTO,
		StatPrefix:        "http",
		GenerateRequestId: wrapperspb.Bool(true),
		RouteSpecifier: &hcm.HttpConnectionManager_Rds{
			Rds: &hcm.Rds{
				ConfigSource: &core.ConfigSource{
					ResourceApiVersion: core.ApiVersion_V3,
					ConfigSourceSpecifier: &core.ConfigSource_Ads{
						Ads: &core.AggregatedConfigSource{},
					},
					// No timeout - wait indefinitely for route config
					InitialFetchTimeout: durationpb.New(0),
				},
				RouteConfigName: SharedRouteConfigName,
			},
		},
		HttpFilters:                httpFilters,
		ServerHeaderTransformation: convertServerHeaderTransformation(t.routerConfig.HTTPListener.ServerHeaderTransformation),
		ServerName:                 t.routerConfig.HTTPListener.ServerHeaderValue,
	}

	// Add access logs if enabled
	if t.routerConfig.AccessLogs.Enabled {
		accessLogs, err := t.createAccessLogConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create access log config: %w", err)
		}
		manager.AccessLog = accessLogs
	}

	// Add tracing if enabled
	tracingConfig, err := t.createTracingConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create tracing config: %w", err)
	}
	if tracingConfig != nil {
		manager.Tracing = tracingConfig
	}

	pbst, err := anypb.New(manager)
	if err != nil {
		return nil, nil, err
	}

	// Determine listener name and port based on protocol
	var listenerName string
	var port uint32
	if isHTTPS {
		listenerName = fmt.Sprintf("listener_https_%d", t.routerConfig.HTTPSPort)
		port = uint32(t.routerConfig.HTTPSPort)
	} else {
		listenerName = fmt.Sprintf("listener_http_%d", t.routerConfig.ListenerPort)
		port = uint32(t.routerConfig.ListenerPort)
	}

	// Create filter chain
	filterChain := &listener.FilterChain{
		Filters: []*listener.Filter{{
			Name: wellknown.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{
				TypedConfig: pbst,
			},
		}},
	}

	// Add TLS configuration if HTTPS
	if isHTTPS {
		tlsContext, err := t.createDownstreamTLSContext()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create downstream TLS context: %w", err)
		}

		tlsContextAny, err := anypb.New(tlsContext)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal downstream TLS context: %w", err)
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
	}, routeConfig, nil
}

func (t *Translator) createInternalListenerForWebSubHub(isHTTPS bool) (*listener.Listener, error) {
	// Reverse proxy listener: exactly one route /websubhub/operations rewritten to /hub
	// This allows clients to call /websubhub/operations and internally reach /hub on upstream.

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
					PrefixRewrite:    "/hub", // rewrite path
				}},
			}},
		}},
	}

	// Create router filter with typed config
	routerConfig := &router.Router{}
	routerAny, err := anypb.New(routerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create router config: %w", err)
	}

	// Build HTTP filters chain
	httpFilters := make([]*hcm.HttpFilter, 0)

	// Add ext_proc filter if policy engine is enabled
	if t.routerConfig.PolicyEngine.Enabled {
		extProcFilter, err := t.createExtProcFilter()
		if err != nil {
			return nil, fmt.Errorf("failed to create ext_proc filter: %w", err)
		}
		httpFilters = append(httpFilters, extProcFilter)

		luaFilter, err := t.createLuaFilter()
		if err != nil {
			return nil, fmt.Errorf("failed to create lua filter: %w", err)
		}
		httpFilters = append(httpFilters, luaFilter)
	}

	// Add router filter (must be last)
	httpFilters = append(httpFilters, &hcm.HttpFilter{
		Name: wellknown.Router,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: routerAny,
		},
	})

	// Create HTTP connection manager
	manager := &hcm.HttpConnectionManager{
		CodecType:         hcm.HttpConnectionManager_AUTO,
		StatPrefix:        "http",
		GenerateRequestId: wrapperspb.Bool(true),
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: routeConfig,
		},
		HttpFilters: httpFilters,
	}

	// Add access logs if enabled
	if t.routerConfig.AccessLogs.Enabled {
		accessLogs, err := t.createAccessLogConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create access log config: %w", err)
		}
		manager.AccessLog = accessLogs
	}

	// Add tracing if enabled
	tracingConfig, err := t.createTracingConfig()
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

	// Determine listener name and port based on protocol
	// TODO: Use config values for port
	var listenerName string
	var port uint32
	if isHTTPS {
		listenerName = fmt.Sprintf("listener_https_%d", constants.WEBSUB_HUB_INTERNAL_HTTPS_PORT)
		port = uint32(constants.WEBSUB_HUB_INTERNAL_HTTPS_PORT)
	} else {
		listenerName = fmt.Sprintf("listener_http_%d", constants.WEBSUB_HUB_INTERNAL_HTTP_PORT)
		port = uint32(constants.WEBSUB_HUB_INTERNAL_HTTP_PORT)
	}

	// Create filter chain
	filterChain := &listener.FilterChain{
		Filters: []*listener.Filter{{
			Name: wellknown.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{
				TypedConfig: pbst,
			},
		}},
	}

	// Add TLS configuration if HTTPS
	if isHTTPS {
		tlsContext, err := t.createDownstreamTLSContext()
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

	// routeConfig := &route.RouteConfiguration{
	// 	Name: "websubhub-internal-route",
	// 	VirtualHosts: []*route.VirtualHost{{
	// 		Name:    "WEBSUBHUB_INTERNAL_VHOST",
	// 		Domains: []string{"*"},
	// 		Routes: []*route.Route{{
	// 			Match: &route.RouteMatch{PathSpecifier: &route.RouteMatch_Path{Path: "/websubhub/operations"}},
	// 			Action: &route.Route_Route{Route: &route.RouteAction{
	// 				ClusterSpecifier: &route.RouteAction_Cluster{Cluster: WebSubHubInternalClusterName},
	// 				Timeout:          durationpb.New(30 * time.Second),
	// 				PrefixRewrite:    "/hub", // rewrite path
	// 			}},
	// 		}},
	// 	}},
	// }

	// // Router filter
	// routerCfg := &router.Router{}
	// routerAny, err := anypb.New(routerCfg)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to marshal router config: %w", err)
	// }

	// // HttpConnectionManager for port 8083
	// hcmCfg := &hcm.HttpConnectionManager{
	// 	StatPrefix:        "websubhub_internal_8083",
	// 	CodecType:         hcm.HttpConnectionManager_AUTO,
	// 	GenerateRequestId: wrapperspb.Bool(true),
	// 	RouteSpecifier:    &hcm.HttpConnectionManager_RouteConfig{RouteConfig: routeConfig},
	// 	HttpFilters: []*hcm.HttpFilter{
	// 		{
	// 			Name:       wellknown.Router,
	// 			ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: routerAny},
	// 		},
	// 	},
	// }

	// // Attach access logs if enabled
	// if t.routerConfig.AccessLogs.Enabled {
	// 	accessLogs, err := t.createAccessLogConfig()
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to create access log config: %w", err)
	// 	}
	// 	hcmCfg.AccessLog = accessLogs
	// }

	// // Add tracing if enabled
	// tracingCfg, err := t.createTracingConfig()
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create tracing config: %w", err)
	// }
	// if tracingCfg != nil {
	// 	hcmCfg.Tracing = tracingCfg
	// }

	// hcmAny, err := anypb.New(hcmCfg)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to marshal http connection manager: %w", err)
	// }

	// return &listener.Listener{
	// 	Name: "websubhub-internal-8083",
	// 	Address: &core.Address{Address: &core.Address_SocketAddress{SocketAddress: &core.SocketAddress{
	// 		Protocol:      core.SocketAddress_TCP,
	// 		Address:       "0.0.0.0",
	// 		PortSpecifier: &core.SocketAddress_PortValue{PortValue: 8083},
	// 	}}},
	// 	FilterChains: []*listener.FilterChain{{
	// 		Filters: []*listener.Filter{{
	// 			Name:       wellknown.HTTPConnectionManager,
	// 			ConfigType: &listener.Filter_TypedConfig{TypedConfig: hcmAny},
	// 		}},
	// 	}},
	// }, nil
}

// createDynamicFwdListenerForWebSubHub creates an Envoy listener with access logging
func (t *Translator) createDynamicFwdListenerForWebSubHub(isHTTPS bool) (*listener.Listener, error) {
	// Build the route configuration for dynamic forward proxy listener
	// We ignore the passed virtualHosts here and construct the required one matching the sample.
	dynamicForwardProxyRouteConfig := &route.RouteConfiguration{
		Name: "dynamic-forward-proxy-routing",
		VirtualHosts: []*route.VirtualHost{{
			Name:    "DYNAMIXC_FORWARD_PROXY_VHOST_WEBSUBHUB",
			Domains: []string{t.routerConfig.EventGateway.WebSubHubURL}, // this should be websubhub domains
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
	// Build HTTP filters chain
	httpFilters := make([]*hcm.HttpFilter, 0)

	// Add ext_proc filter if policy engine is enabled
	if t.routerConfig.PolicyEngine.Enabled {
		extProcFilter, err := t.createExtProcFilter()
		if err != nil {
			return nil, fmt.Errorf("failed to create ext_proc filter: %w", err)
		}
		httpFilters = append(httpFilters, extProcFilter)

		luaFilter, err := t.createLuaFilter()
		if err != nil {
			return nil, fmt.Errorf("failed to create lua filter: %w", err)
		}
		httpFilters = append(httpFilters, luaFilter)
	}

	dnsCacheConfig := &common_dfp.DnsCacheConfig{
		// Required: unique name for the shared DNS cache
		Name: "dynamic_forward_proxy_cache",

		// Optional: how often DNS entries are refreshed
		DnsRefreshRate: durationpb.New(60 * time.Second),

		// Optional: how long hosts stay cached
		HostTtl: durationpb.New(300 * time.Second),

		// Optional: which DNS families to use (AUTO, V4_ONLY, V6_ONLY)
		DnsLookupFamily: cluster.Cluster_V4_ONLY,

		MaxHosts: &wrapperspb.UInt32Value{Value: 1024},
	}

	dfpFilterConfig := &dfpv3.FilterConfig{
		ImplementationSpecifier: &dfpv3.FilterConfig_DnsCacheConfig{
			DnsCacheConfig: dnsCacheConfig,
		},
	}
	// Dynamic forward proxy filter config placeholder (typed config fields omitted for compatibility with current go-control-plane version)
	dynamicFwdAny, err := anypb.New(dfpFilterConfig)

	if err != nil {
		return nil, fmt.Errorf("failed to marshal dynamic forward proxy config: %w", err)
	}

	// Router filter
	routerConfig := &router.Router{}
	routerAny, err := anypb.New(routerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal router config: %w", err)
	}

	// Add dynamic forward proxy router filter (must be last)
	httpFilters = append(httpFilters, &hcm.HttpFilter{
		Name: wellknown.Router,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: dynamicFwdAny,
		},
	})

	// Add router filter (must be last)
	httpFilters = append(httpFilters, &hcm.HttpFilter{
		Name: wellknown.Router,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: routerAny,
		},
	})

	// Create HTTP connection manager
	httpConnManager := &hcm.HttpConnectionManager{
		CodecType:         hcm.HttpConnectionManager_AUTO,
		StatPrefix:        "http",
		GenerateRequestId: wrapperspb.Bool(true),
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: dynamicForwardProxyRouteConfig,
		},
		HttpFilters: httpFilters,
	}

	// httpConnManager := &hcm.HttpConnectionManager{
	// 	StatPrefix:     "WEBSUBHUB_INBOUND_8082_LISTENER",
	// 	CodecType:      hcm.HttpConnectionManager_AUTO,
	// 	RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{RouteConfig: dynamicForwardProxyRouteConfig},
	// 	HttpFilters: []*hcm.HttpFilter{
	// 		{ // dynamic forward proxy filter
	// 			Name:       "envoy.filters.http.dynamic_forward_proxy",
	// 			ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: dynamicFwdAny},
	// 		},
	// 		{ // router filter must be last
	// 			Name:       wellknown.Router,
	// 			ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: routerAny},
	// 		},
	// 	},
	// }

	// Attach access logs if enabled
	if t.routerConfig.AccessLogs.Enabled {
		accessLogs, err := t.createAccessLogConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create access log config: %w", err)
		}
		httpConnManager.AccessLog = accessLogs
	}

	// Add tracing if enabled
	tracingCfgDFP, err := t.createTracingConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create tracing config: %w", err)
	}
	if tracingCfgDFP != nil {
		httpConnManager.Tracing = tracingCfgDFP
	}

	// hcmAny, err := anypb.New(httpConnManager)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to marshal http connection manager: %w", err)
	// }

	pbst, err := anypb.New(httpConnManager)
	if err != nil {
		return nil, err
	}

	// Determine listener name and port based on protocol
	// TODO: Use config values for port
	var listenerName string
	var port uint32
	if isHTTPS {
		listenerName = fmt.Sprintf("listener_https_%d", constants.WEBSUB_HUB_DYNAMIC_HTTPS_PORT)
		port = uint32(constants.WEBSUB_HUB_DYNAMIC_HTTPS_PORT)
	} else {
		listenerName = fmt.Sprintf("listener_http_%d", constants.WEBSUB_HUB_DYNAMIC_HTTP_PORT)
		port = uint32(constants.WEBSUB_HUB_DYNAMIC_HTTP_PORT)
	}

	//TODO: Add TLS Filter chain for HTTPS
	// if isHTTPS {
	// 	tlsContext, err := t.createDownstreamTLSContext()
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to create downstream TLS context: %w", err)
	// 	}

	// 	tlsContextAny, err := anypb.New(tlsContext)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to marshal downstream TLS context: %w", err)
	// 	}

	// 	filterChain.TransportSocket = &core.TransportSocket{
	// 		Name: "envoy.transport_sockets.tls",
	// 		ConfigType: &core.TransportSocket_TypedConfig{
	// 			TypedConfig: tlsContextAny,
	// 		},
	// 	}
	// }

	// Create filter chain
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
		// FilterChains: []*listener.FilterChain{{
		// 	Filters: []*listener.Filter{{
		// 		Name:       wellknown.HTTPConnectionManager,
		// 		ConfigType: &listener.Filter_TypedConfig{TypedConfig: hcmAny},
		// 	}},
		// }},
	}, nil
}

// createRouteConfiguration creates a route configuration
// Uses SharedRouteConfigName so it can be discovered via RDS
func (t *Translator) createRouteConfiguration(virtualHosts []*route.VirtualHost) *route.RouteConfiguration {
	return &route.RouteConfiguration{
		Name:         SharedRouteConfigName,
		VirtualHosts: virtualHosts,
	}
}

// getValueFromSourceConfig extracts a value from sourceConfig using a key path.
// The key can be a simple key (e.g., "kind") or a nested path (e.g., "spec.template").
// Returns the value if found, nil otherwise.
func getValueFromSourceConfig(sourceConfig any, key string) (any, error) {
	if sourceConfig == nil {
		return nil, fmt.Errorf("sourceConfig is nil")
	}

	// Convert sourceConfig to a map for easy traversal
	var configMap map[string]interface{}
	j, err := json.Marshal(sourceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sourceConfig: %w", err)
	}
	if err := json.Unmarshal(j, &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sourceConfig: %w", err)
	}

	// Split the key by dots to handle nested paths
	keys := strings.Split(key, ".")
	current := configMap

	// Traverse the nested structure
	for i, k := range keys {
		if i == len(keys)-1 {
			// Last key, return the value
			if val, ok := current[k]; ok {
				return val, nil
			}
			return nil, fmt.Errorf("key '%s' not found in sourceConfig", key)
		}

		// Navigate further down the nested structure
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			return nil, fmt.Errorf("key path '%s' is invalid: '%s' is not a map", key, strings.Join(keys[:i+1], "."))
		}
	}

	return nil, fmt.Errorf("key '%s' not found in sourceConfig", key)
}

// extractTemplateHandle extracts the template handle from source configuration
// For LlmProvider: extracts from spec.template
// For LlmProxy: resolves provider reference to get spec.template
func (t *Translator) extractTemplateHandle(cfg *models.StoredConfig, allConfigs []*models.StoredConfig) string {
	if cfg.SourceConfiguration == nil {
		return ""
	}

	// Get kind from source configuration
	kind, err := getValueFromSourceConfig(cfg.SourceConfiguration, "kind")
	if err != nil {
		return ""
	}

	kindStr, ok := kind.(string)
	if !ok {
		return ""
	}

	// For LlmProvider: extract template handle directly
	switch kindStr {
	case string(api.LlmProvider):
		templateHandle, err := getValueFromSourceConfig(cfg.SourceConfiguration, "spec.template")
		if err != nil {
			t.logger.Debug("Failed to extract template handle from LlmProvider", slog.Any("error", err))
			return ""
		}
		if templateHandleStr, ok := templateHandle.(string); ok && templateHandleStr != "" {
			return templateHandleStr
		}

	// For LlmProxy: resolve provider reference
	case string(api.LlmProxy):
		providerName, err := getValueFromSourceConfig(cfg.SourceConfiguration, "spec.provider")
		if err != nil {
			t.logger.Debug("Failed to extract provider name from LlmProxy", slog.Any("error", err))
			return ""
		}
		providerNameStr, ok := providerName.(string)
		if !ok || providerNameStr == "" {
			return ""
		}

		// Find the provider config
		for _, providerCfg := range allConfigs {
			if providerCfg.Kind == string(api.LlmProvider) {
				// Check if this is the provider we're looking for
				providerMetadataName, err := getValueFromSourceConfig(providerCfg.SourceConfiguration, "metadata.name")
				if err == nil {
					if providerMetadataNameStr, ok := providerMetadataName.(string); ok && providerMetadataNameStr == providerNameStr {
						// Found the provider, extract its template
						templateHandle, err := getValueFromSourceConfig(providerCfg.SourceConfiguration, "spec.template")
						if err == nil {
							if templateHandleStr, ok := templateHandle.(string); ok && templateHandleStr != "" {
								return templateHandleStr
							}
						}
					}
				}
			}
		}
	}

	return ""
}

// extractProviderName extracts the provider name for LLM provider/proxy scenarios
// For LlmProvider: returns the provider's own metadata.name
// For LlmProxy: returns the referenced provider's metadata.name
func (t *Translator) extractProviderName(cfg *models.StoredConfig, allConfigs []*models.StoredConfig) string {
	if cfg.SourceConfiguration == nil {
		return ""
	}

	// Get kind from source configuration
	kind, err := getValueFromSourceConfig(cfg.SourceConfiguration, "kind")
	if err != nil {
		return ""
	}

	kindStr, ok := kind.(string)
	if !ok {
		return ""
	}

	switch kindStr {
	case string(api.LlmProvider):
		// For LlmProvider: return its own metadata.name
		providerName, err := getValueFromSourceConfig(cfg.SourceConfiguration, "metadata.name")
		if err != nil {
			t.logger.Debug("Failed to extract provider name from LlmProvider", slog.Any("error", err))
			return ""
		}
		if providerNameStr, ok := providerName.(string); ok && providerNameStr != "" {
			return providerNameStr
		}

	case string(api.LlmProxy):
		// For LlmProxy: return the referenced provider name from spec.provider
		providerName, err := getValueFromSourceConfig(cfg.SourceConfiguration, "spec.provider")
		if err != nil {
			t.logger.Debug("Failed to extract provider reference from LlmProxy", slog.Any("error", err))
			return ""
		}
		if providerNameStr, ok := providerName.(string); ok && providerNameStr != "" {
			return providerNameStr
		}
	}

	return ""
}

// createRoute creates a route for an operation
func (t *Translator) createRoute(apiId, apiName, apiVersion, context, method, path, clusterName,
	upstreamPath string, vhost string, apiKind string, templateHandle string, providerName string, hostRewrite *api.UpstreamHostRewrite, projectID string, timeoutCfg *resolvedTimeout) *route.Route {
	// Resolve version placeholder in context
	context = strings.ReplaceAll(context, "$version", apiVersion)

	// Build the full path, handling root context "/" to avoid duplication
	var fullPath string
	if context == "/" {
		fullPath = path
	} else {
		fullPath = context + path
	}

	// Generate unique route name using the helper function
	// Format: HttpMethod|RoutePath|Vhost (e.g., "GET|/weather/v1.0/us/seattle|localhost")
	routeName := GenerateRouteName(method, context, "", path, vhost)

	// Check if path is a wildcard catch-all (e.g., /v1/*)
	isWildcardPath := strings.HasSuffix(path, "/*")

	// Check if path contains parameters (e.g., {country_code})
	hasParams := strings.Contains(path, "{")

	var pathSpecifier *route.RouteMatch_SafeRegex
	if isWildcardPath {
		// For wildcard paths, use prefix matching by removing the /*
		// This will match /v1/foo, /v1/foo/bar, etc.
		prefixPath := strings.TrimSuffix(fullPath, "/*")
		pathSpecifier = &route.RouteMatch_SafeRegex{
			SafeRegex: &matcher.RegexMatcher{
				Regex: "^" + regexp.QuoteMeta(prefixPath) + "/.*$",
			},
		}
	} else if hasParams {
		// Use regex matching for parameterized paths
		regexPattern := t.pathToRegex(fullPath)
		pathSpecifier = &route.RouteMatch_SafeRegex{
			SafeRegex: &matcher.RegexMatcher{
				Regex: regexPattern,
			},
		}
	}

	// Determine timeouts: use overrides if provided, otherwise use global config
	var requestTimeout time.Duration
	if timeoutCfg != nil && timeoutCfg.Request != nil {
		requestTimeout = *timeoutCfg.Request
	} else {
		requestTimeout = time.Duration(t.routerConfig.Upstream.Timeouts.RouteTimeoutInSeconds) * time.Second
	}

	var idleTimeout time.Duration
	if timeoutCfg != nil && timeoutCfg.Idle != nil {
		idleTimeout = *timeoutCfg.Idle
	} else {
		idleTimeout = time.Duration(t.routerConfig.Upstream.Timeouts.RouteIdleTimeoutInSeconds) * time.Second
	}

	routeAction := &route.Route_Route{
		Route: &route.RouteAction{
			Timeout:     durationpb.New(requestTimeout),
			IdleTimeout: durationpb.New(idleTimeout),
			ClusterSpecifier: &route.RouteAction_Cluster{
				Cluster: clusterName,
			},
		},
	}

	// Set host rewrite based on configuration
	if hostRewrite == nil || *hostRewrite != api.Manual {
		routeAction.Route.HostRewriteSpecifier = &route.RouteAction_AutoHostRewrite{
			AutoHostRewrite: &wrapperspb.BoolValue{
				Value: true,
			},
		}
	}

	r := &route.Route{
		Name:   routeName,
		Match:  &route.RouteMatch{},
		Action: routeAction,
	}

	// Only add headers if not a wildcard path
	if !isWildcardPath {
		r.Match = &route.RouteMatch{
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
		}
	}

	// Attach dynamic metadata for downstream correlation (policies, logging, tracing)
	metaMap := map[string]interface{}{
		"route_name":  routeName,
		"api_id":      apiId,
		"api_name":    apiName,
		"api_version": apiVersion,
		"api_context": context,
		"path":        path,
		"method":      method,
		"vhost":       vhost,
		"api_kind":    apiKind,
	}
	// Add template_handle if available (for LLM provider/proxy scenarios)
	if templateHandle != "" {
		metaMap["template_handle"] = templateHandle
	}
	// Add provider_name if available (for LLM provider/proxy scenarios)
	if providerName != "" {
		metaMap["provider_name"] = providerName
	}
	// Add projectID if available
	if projectID != "" {
		metaMap["project_id"] = projectID
	}
	if metaStruct, err := structpb.NewStruct(metaMap); err == nil {
		r.Metadata = &core.Metadata{FilterMetadata: map[string]*structpb.Struct{
			"wso2.route": metaStruct,
		}}
	}

	// Set path specifier based on whether we have parameters
	if isWildcardPath || hasParams {
		r.Match.PathSpecifier = pathSpecifier
	} else {
		// Use exact path matching for non-parameterized paths
		r.Match.PathSpecifier = &route.RouteMatch_Path{
			Path: fullPath,
		}
	}

	// Add path rewriting if upstream has a path prefix
	// Strip the API context (with version if included) and prepend the upstream path
	// Example 1: request /weather/v1.0/us/seattle with context /weather/$version and upstream /api/v2
	//            should result in /api/v2/us/seattle
	// Example 2: request /weather/us/seattle with context /weather and upstream /api/v2
	//            should result in /api/v2/us/seattle

	// Use RegexRewrite to strip the context (with version substituted if present) and prepend upstream path
	// Pattern captures everything after the context
	// Escape special regex characters (e.g., dots in version like v1.0)
	if upstreamPath == "/" {
		upstreamPath = ""
	}
	// For wildcard routes, construct the regex to match everything after the prefix
	var contextWithVersion string
	if isWildcardPath {
		// TODO: (renuka) Can't understand this code. Check with Nimsara.
		// Remove the /* from the end before constructing the context
		pathWithoutWildcard := strings.TrimSuffix(path, "/*")
		contextWithVersion = ConstructFullPath(context, apiVersion, pathWithoutWildcard)
	} else {
		contextWithVersion = ConstructFullPath(context, apiVersion, "")
	}
	escapedContext := regexp.QuoteMeta(contextWithVersion)
	r.GetRoute().RegexRewrite = &matcher.RegexMatchAndSubstitute{
		Pattern: &matcher.RegexMatcher{
			Regex: "^" + escapedContext + "(.*)$",
		},
		Substitution: upstreamPath + "\\1",
	}

	return r
}

// createRoutePerTopic creates a route for an operation
func (t *Translator) createRoutePerTopic(apiId, apiName, apiVersion, context, method, channelName, clusterName, vhost, apiKind, projectID string) *route.Route {
	routeName := GenerateRouteName(method, context, apiVersion, channelName, vhost)
	r := &route.Route{
		Name: routeName,
		Match: &route.RouteMatch{
			Headers: []*route.HeaderMatcher{{
				Name: ":method",
				HeaderMatchSpecifier: &route.HeaderMatcher_StringMatch{
					StringMatch: &matcher.StringMatcher{
						MatchPattern: &matcher.StringMatcher_Exact{
							Exact: "POST",
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

	metaMap := map[string]interface{}{
		"route_name":  routeName,
		"api_id":      apiId,
		"api_name":    apiName,
		"api_version": apiVersion,
		"api_context": context,
		"path":        channelName,
		"method":      method,
		"vhost":       vhost,
		"api_kind":    apiKind,
	}

	// Add projectID if available
	if projectID != "" {
		metaMap["project_id"] = projectID
	}

	if metaStruct, err := structpb.NewStruct(metaMap); err == nil {
		r.Metadata = &core.Metadata{FilterMetadata: map[string]*structpb.Struct{
			"wso2.route": metaStruct,
		}}
	}

	r.Match.PathSpecifier = &route.RouteMatch_Path{
		Path: ConstructFullPath(context, apiVersion, channelName),
	}

	r.GetRoute().PrefixRewrite = "/hub"

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

// createPolicyEngineCluster creates an Envoy cluster for the policy engine ext_proc service
func (t *Translator) createPolicyEngineCluster() *cluster.Cluster {
	policyEngine := t.routerConfig.PolicyEngine

	// Build the endpoint address
	address := &core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Protocol: core.SocketAddress_TCP,
				Address:  policyEngine.Host,
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: policyEngine.Port,
				},
			},
		},
	}

	// Create the load balancing endpoint
	lbEndpoint := &endpoint.LbEndpoint{
		HostIdentifier: &endpoint.LbEndpoint_Endpoint{
			Endpoint: &endpoint.Endpoint{
				Address: address,
			},
		},
	}

	// Create locality lb endpoints
	localityLbEndpoints := &endpoint.LocalityLbEndpoints{
		LbEndpoints: []*endpoint.LbEndpoint{lbEndpoint},
	}

	// Create the cluster with HTTP/2 support for gRPC
	c := &cluster.Cluster{
		Name:                 constants.PolicyEngineClusterName,
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STRICT_DNS},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: constants.PolicyEngineClusterName,
			Endpoints:   []*endpoint.LocalityLbEndpoints{localityLbEndpoints},
		},
		// Enable HTTP/2 for gRPC
		Http2ProtocolOptions: &core.Http2ProtocolOptions{},
	}

	// Add TLS transport socket if TLS is enabled
	if policyEngine.TLS.Enabled {
		tlsContext := &tlsv3.UpstreamTlsContext{
			CommonTlsContext: &tlsv3.CommonTlsContext{
				TlsParams: &tlsv3.TlsParameters{
					TlsMinimumProtocolVersion: tlsv3.TlsParameters_TLSv1_2,
					TlsMaximumProtocolVersion: tlsv3.TlsParameters_TLSv1_3,
				},
			},
		}

		// Add client certificates for mTLS if provided
		if policyEngine.TLS.CertPath != "" && policyEngine.TLS.KeyPath != "" {
			tlsContext.CommonTlsContext.TlsCertificates = []*tlsv3.TlsCertificate{
				{
					CertificateChain: &core.DataSource{
						Specifier: &core.DataSource_Filename{
							Filename: policyEngine.TLS.CertPath,
						},
					},
					PrivateKey: &core.DataSource{
						Specifier: &core.DataSource_Filename{
							Filename: policyEngine.TLS.KeyPath,
						},
					},
				},
			}
		}

		// Configure CA certificate for server verification
		if !policyEngine.TLS.SkipVerify {
			var trustedCASource *core.DataSource
			if policyEngine.TLS.CAPath != "" {
				trustedCASource = &core.DataSource{
					Specifier: &core.DataSource_Filename{
						Filename: policyEngine.TLS.CAPath,
					},
				}
			}

			if trustedCASource != nil {
				tlsContext.CommonTlsContext.ValidationContextType = &tlsv3.CommonTlsContext_ValidationContext{
					ValidationContext: &tlsv3.CertificateValidationContext{
						TrustedCa: trustedCASource,
					},
				}
			}
		}

		// Set SNI if server name is provided
		if policyEngine.TLS.ServerName != "" {
			tlsContext.Sni = policyEngine.TLS.ServerName
		}

		// Create transport socket
		tlsAny, err := anypb.New(tlsContext)
		if err != nil {
			t.logger.Error("Failed to marshal TLS context for policy engine cluster", slog.Any("error", err))
		} else {
			c.TransportSocket = &core.TransportSocket{
				Name: wellknown.TransportSocketTls,
				ConfigType: &core.TransportSocket_TypedConfig{
					TypedConfig: tlsAny,
				},
			}
		}
	}

	return c
}

// createALSCluster creates an Envoy cluster for the gRPC access log service
func (t *Translator) createALSCluster() *cluster.Cluster {
	grpcConfig := t.config.Analytics.GRPCAccessLogCfg

	address := &core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Protocol: core.SocketAddress_TCP,
				Address:  grpcConfig.Host,
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: uint32(t.config.Analytics.AccessLogsServiceCfg.ALSServerPort),
				},
			},
		},
	}

	lbEndpoint := &endpoint.LbEndpoint{
		HostIdentifier: &endpoint.LbEndpoint_Endpoint{
			Endpoint: &endpoint.Endpoint{
				Address: address,
			},
		},
	}

	localityLbEndpoints := &endpoint.LocalityLbEndpoints{
		LbEndpoints: []*endpoint.LbEndpoint{lbEndpoint},
	}

	return &cluster.Cluster{
		Name:                 constants.GRPCAccessLogClusterName,
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STRICT_DNS},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: constants.GRPCAccessLogClusterName,
			Endpoints:   []*endpoint.LocalityLbEndpoints{localityLbEndpoints},
		},
		// Enable HTTP/2 for gRPC
		Http2ProtocolOptions: &core.Http2ProtocolOptions{},
	}
}

// createOTELCollectorCluster creates an Envoy cluster for OpenTelemetry collector
func (t *Translator) createOTELCollectorCluster() *cluster.Cluster {
	// Return nil if tracing is not enabled
	if !t.config.TracingConfig.Enabled {
		return nil
	}

	// Parse endpoint to extract host and port
	otelEndpoint := t.config.TracingConfig.Endpoint
	if otelEndpoint == "" {
		otelEndpoint = "otel-collector:4317"
	}

	// Split host:port
	host, port := otelEndpoint, uint32(4317)
	if colonIdx := strings.LastIndex(otelEndpoint, ":"); colonIdx != -1 {
		host = otelEndpoint[:colonIdx]
		portStr := otelEndpoint[colonIdx+1:]
		if parsedPort, err := strconv.ParseUint(portStr, 10, 16); err != nil {
			t.logger.Warn("Invalid OTEL collector port, using default 4317",
				slog.String("endpoint", otelEndpoint))
			port = 4317
		} else {
			port = uint32(parsedPort)
		}
	}

	// Build the endpoint address
	address := &core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Protocol: core.SocketAddress_TCP,
				Address:  host,
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: port,
				},
			},
		},
	}

	// Create the load balancing endpoint
	lbEndpoint := &endpoint.LbEndpoint{
		HostIdentifier: &endpoint.LbEndpoint_Endpoint{
			Endpoint: &endpoint.Endpoint{
				Address: address,
			},
		},
	}

	// Create locality lb endpoints
	localityLbEndpoints := &endpoint.LocalityLbEndpoints{
		LbEndpoints: []*endpoint.LbEndpoint{lbEndpoint},
	}

	// Create the cluster with HTTP/2 support for gRPC
	c := &cluster.Cluster{
		Name:                 OTELCollectorClusterName,
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STRICT_DNS},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: OTELCollectorClusterName,
			Endpoints:   []*endpoint.LocalityLbEndpoints{localityLbEndpoints},
		},
		// Enable HTTP/2 for gRPC (OTLP uses gRPC)
		Http2ProtocolOptions: &core.Http2ProtocolOptions{},
	}

	t.logger.Info("Created OTEL collector cluster",
		slog.String("cluster_name", OTELCollectorClusterName),
		slog.String("endpoint", otelEndpoint))

	return c
}

// createSDSCluster creates an Envoy cluster for the SDS (Secret Discovery Service)
// This cluster allows Envoy to fetch TLS certificates dynamically via xDS
func (t *Translator) createSDSCluster() *cluster.Cluster {
	// SDS uses the same xDS server
	// In containerized environments, Envoy connects to the gateway-controller container
	// Use the same host/port configuration as the main xDS connection
	xdsHost := "gateway-controller" // Default for Docker Compose
	if envHost := os.Getenv("XDS_SERVER_HOST"); envHost != "" {
		xdsHost = envHost
	}

	xdsPort := t.config.GatewayController.Server.XDSPort
	if xdsPort == 0 {
		xdsPort = 18000 // Default xDS port
	}

	address := &core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Protocol: core.SocketAddress_TCP,
				Address:  xdsHost,
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: uint32(xdsPort),
				},
			},
		},
	}

	lbEndpoint := &endpoint.LbEndpoint{
		HostIdentifier: &endpoint.LbEndpoint_Endpoint{
			Endpoint: &endpoint.Endpoint{
				Address: address,
			},
		},
	}

	localityLbEndpoints := &endpoint.LocalityLbEndpoints{
		LbEndpoints: []*endpoint.LbEndpoint{lbEndpoint},
	}

	// Create the SDS cluster
	// Note: SDS must use HTTP/2 for gRPC communication
	return &cluster.Cluster{
		Name:                 "sds_cluster",
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STRICT_DNS},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: "sds_cluster",
			Endpoints:   []*endpoint.LocalityLbEndpoints{localityLbEndpoints},
		},
		// Enable HTTP/2 for gRPC
		Http2ProtocolOptions: &core.Http2ProtocolOptions{},
	}
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
		// Priority order for trusted CA certificates:
		// 1. SDS secret reference (if cert store is available) - Uses dynamic secret discovery
		// 2. Certificate parameter (per-upstream cert, currently unused but kept for future)
		// 3. Configured trusted cert path (system certs only)
		// 4. If none provided, Envoy falls back to system default trust store

		if t.certStore != nil {
			// Use SDS to dynamically fetch certificates
			// This is more efficient than inlining certificates in every cluster config
			sdsConfig := &core.ConfigSource{
				ResourceApiVersion: core.ApiVersion_V3,
				ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
					ApiConfigSource: &core.ApiConfigSource{
						ApiType:             core.ApiConfigSource_GRPC,
						TransportApiVersion: core.ApiVersion_V3,
						GrpcServices: []*core.GrpcService{
							{
								TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
									EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
										ClusterName: "sds_cluster",
									},
								},
							},
						},
					},
				},
			}

			upstreamTLSContext.CommonTlsContext.ValidationContextType = &tlsv3.CommonTlsContext_CombinedValidationContext{
				CombinedValidationContext: &tlsv3.CommonTlsContext_CombinedCertificateValidationContext{
					// Default validation context (for hostname verification, etc.)
					DefaultValidationContext: &tlsv3.CertificateValidationContext{},
					// Dynamic validation context from SDS
					ValidationContextSdsSecretConfig: &tlsv3.SdsSecretConfig{
						Name:      SecretNameUpstreamCA,
						SdsConfig: sdsConfig,
					},
				},
			}
			t.logger.Debug("Using SDS for upstream TLS certificates",
				slog.String("upstream", address),
				slog.String("secret_name", SecretNameUpstreamCA))
		} else if len(certificate) > 0 {
			// Use per-upstream certificate if provided
			upstreamTLSContext.CommonTlsContext.ValidationContextType = &tlsv3.CommonTlsContext_ValidationContext{
				ValidationContext: &tlsv3.CertificateValidationContext{
					TrustedCa: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{
							InlineBytes: certificate,
						},
					},
				},
			}
		} else if t.routerConfig.Upstream.TLS.TrustedCertPath != "" {
			// Fall back to system cert path
			upstreamTLSContext.CommonTlsContext.ValidationContextType = &tlsv3.CommonTlsContext_ValidationContext{
				ValidationContext: &tlsv3.CertificateValidationContext{
					TrustedCa: &core.DataSource{
						Specifier: &core.DataSource_Filename{
							Filename: t.routerConfig.Upstream.TLS.TrustedCertPath,
						},
					},
				},
			}
		}

		// Add hostname verification if enabled
		if t.routerConfig.Upstream.TLS.VerifyHostName {
			sanType := tlsv3.SubjectAltNameMatcher_DNS
			if isIP {
				sanType = tlsv3.SubjectAltNameMatcher_IP_ADDRESS
			}

			sanMatcher := []*tlsv3.SubjectAltNameMatcher{
				{
					SanType: sanType,
					Matcher: &matcher.StringMatcher{
						MatchPattern: &matcher.StringMatcher_Exact{
							Exact: address,
						},
					},
				},
			}

			// Apply SAN matching based on the validation context type
			if combinedContext := upstreamTLSContext.CommonTlsContext.GetCombinedValidationContext(); combinedContext != nil {
				// SDS case - add to default validation context
				if combinedContext.DefaultValidationContext != nil {
					combinedContext.DefaultValidationContext.MatchTypedSubjectAltNames = sanMatcher
				}
			} else if validationContext := upstreamTLSContext.CommonTlsContext.GetValidationContext(); validationContext != nil {
				// Non-SDS case - add to regular validation context
				validationContext.MatchTypedSubjectAltNames = sanMatcher
			}
		}
	}

	return upstreamTLSContext
}

// createDownstreamTLSContext creates a downstream TLS context for HTTPS listeners
func (t *Translator) createDownstreamTLSContext() (*tlsv3.DownstreamTlsContext, error) {
	// Read certificate and key files
	certBytes, err := os.ReadFile(t.routerConfig.DownstreamTLS.CertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	keyBytes, err := os.ReadFile(t.routerConfig.DownstreamTLS.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Create TLS certificate configuration
	tlsCert := &tlsv3.TlsCertificate{
		CertificateChain: &core.DataSource{
			Specifier: &core.DataSource_InlineBytes{
				InlineBytes: certBytes,
			},
		},
		PrivateKey: &core.DataSource{
			Specifier: &core.DataSource_InlineBytes{
				InlineBytes: keyBytes,
			},
		},
	}

	// Parse cipher suites
	var cipherSuites []string
	if t.routerConfig.DownstreamTLS.Ciphers != "" {
		cipherSuites = t.parseCipherSuites(t.routerConfig.DownstreamTLS.Ciphers)
	}

	// Create downstream TLS context
	downstreamTLSContext := &tlsv3.DownstreamTlsContext{
		CommonTlsContext: &tlsv3.CommonTlsContext{
			TlsCertificates: []*tlsv3.TlsCertificate{tlsCert},
			TlsParams: &tlsv3.TlsParameters{
				TlsMinimumProtocolVersion: t.createTLSProtocolVersion(
					t.routerConfig.DownstreamTLS.MinimumProtocolVersion,
				),
				TlsMaximumProtocolVersion: t.createTLSProtocolVersion(
					t.routerConfig.DownstreamTLS.MaximumProtocolVersion,
				),
				CipherSuites: cipherSuites,
			},
			AlpnProtocols: []string{constants.ALPNProtocolHTTP2, constants.ALPNProtocolHTTP11},
		},
	}

	return downstreamTLSContext, nil
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
			t.logger.Error("internal Error while marshalling the upstream TLS Context", slog.Any("error", err))
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

// createDynamicForwardProxyCluster creates a dynamic forward proxy cluster for WebSubHub
func (t *Translator) createDynamicForwardProxyCluster() *cluster.Cluster {
	// Note: Due to go-control-plane API limitations, we use a placeholder Any for the typed config
	// The actual DNS cache config should match the filter config in createListenerForWebSubHub
	clusterConfig := &dfpcluster.ClusterConfig{
		// optional: control connection pooling / subclusters here
	}
	clusterTypeAny, err := anypb.New(clusterConfig)
	if err != nil {
		t.logger.Error("Failed to marshal dynamic forward proxy cluster config", slog.Any("error", err))
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

// pathToRegex converts a path with parameters to a regex pattern
// Converts paths like /weather/v1.0/{country_code}/{city} to ^/weather/v1\.0/[^/]+/[^/]+$
// Special characters (like dots in version) are escaped, but {params} become [^/]+ patterns
func (t *Translator) pathToRegex(path string) string {
	// First, escape all special regex characters to handle literals like dots in versions
	regex := regexp.QuoteMeta(path)

	// Now replace escaped parameter placeholders \{paramName\} with [^/]+ pattern
	// After QuoteMeta, {param} becomes \{param\}, so we need to replace those
	for strings.Contains(regex, "\\{") {
		start := strings.Index(regex, "\\{")
		end := strings.Index(regex, "\\}")
		if end > start {
			// Replace \{paramName\} with [^/]+ (matches one or more non-slash chars)
			regex = regex[:start] + "[^/]+" + regex[end+2:]
		} else {
			break
		}
	}

	// Anchor the regex to match the entire path
	return "^" + regex + "$"
}

// sanitizeClusterName creates a valid cluster name from a hostname and scheme
func (t *Translator) sanitizeClusterName(hostname, scheme string) string {
	name := strings.ReplaceAll(hostname, ".", "_")
	name = strings.ReplaceAll(name, ":", "_")
	// Include scheme to differentiate HTTP and HTTPS clusters for the same host
	return "cluster_" + scheme + "_" + name
}

// createAccessLogConfig creates access log configuration based on format (JSON or text) to stdout
func (t *Translator) createAccessLogConfig() ([]*accesslog.AccessLog, error) {
	var accessLogs []*accesslog.AccessLog
	var fileAccessLog *fileaccesslog.FileAccessLog

	if t.routerConfig.AccessLogs.Format == "json" {
		// Use JSON log format fields from config
		jsonFormat := t.routerConfig.AccessLogs.JSONFields
		if len(jsonFormat) == 0 {
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
	fileAccessLogAny, err := anypb.New(fileAccessLog)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal access log config: %w", err)
	}

	// Add file access log to slice
	accessLogs = append(accessLogs, &accesslog.AccessLog{
		Name: "envoy.access_loggers.file",
		ConfigType: &accesslog.AccessLog_TypedConfig{
			TypedConfig: fileAccessLogAny,
		},
	})

	// If gRPC access log is enabled, create the configuration and append to existing access logs
	if t.config.Analytics.Enabled {
		t.logger.Info("Creating gRPC access log configuration")
		grpcAccessLog, err := t.createGRPCAccessLog()
		if err != nil {
			t.logger.Warn("Failed to create gRPC access log config, continuing without it",
				slog.Any("error", err))
		} else {
			accessLogs = append(accessLogs, grpcAccessLog)
		}
	}

	return accessLogs, nil
}

// createGRPCAccessLog creates a gRPC access log configuration for the gateway controller
func (t *Translator) createGRPCAccessLog() (*accesslog.AccessLog, error) {
	grpcConfig := t.config.Analytics.GRPCAccessLogCfg

	httpGrpcAccessLog := &grpc_accesslogv3.HttpGrpcAccessLogConfig{
		CommonConfig: &grpc_accesslogv3.CommonGrpcAccessLogConfig{
			TransportApiVersion: corev3.ApiVersion_V3,
			LogName:             grpcConfig.LogName,
			BufferFlushInterval: durationpb.New(time.Duration(grpcConfig.BufferFlushInterval)),
			BufferSizeBytes:     wrapperspb.UInt32(uint32(grpcConfig.BufferSizeBytes)),
			GrpcService: &corev3.GrpcService{
				TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{
						ClusterName: constants.GRPCAccessLogClusterName,
					},
				},
				Timeout: durationpb.New(time.Duration(grpcConfig.GRPCRequestTimeout)),
			},
		},
	}

	grpcAccessLogAny, err := anypb.New(httpGrpcAccessLog)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gRPC access log config: %w", err)
	}

	return &accesslog.AccessLog{
		Name: "envoy.access_loggers.http_grpc",
		ConfigType: &accesslog.AccessLog_TypedConfig{
			TypedConfig: grpcAccessLogAny,
		},
	}, nil
}

// createTracingConfig creates tracing configuration for HCM if tracing is enabled
func (t *Translator) createTracingConfig() (*hcm.HttpConnectionManager_Tracing, error) {
	// Return nil if tracing is not enabled
	if !t.config.TracingConfig.Enabled {
		return nil, nil
	}

	// Determine service name with fallback
	serviceName := t.config.GatewayController.Router.TracingServiceName
	if serviceName == "" {
		serviceName = "envoy-gateway"
	}

	// Determine sampling rate - convert from 0.0-1.0 to 0.0-100.0 (Envoy percentage)
	samplingRate := t.config.TracingConfig.SamplingRate
	if samplingRate <= 0.0 {
		samplingRate = 1.0 // Default to 100% sampling
	}
	samplingPercentage := samplingRate * 100.0

	// Create OpenTelemetry tracing configuration
	otelConfig := &tracev3.OpenTelemetryConfig{
		GrpcService: &core.GrpcService{
			TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
				EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
					ClusterName: OTELCollectorClusterName,
				},
			},
		},
		ServiceName: serviceName,
	}

	// Marshal to Any
	otelConfigAny, err := anypb.New(otelConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenTelemetry config: %w", err)
	}

	// Create tracing configuration
	tracingConfig := &hcm.HttpConnectionManager_Tracing{
		Provider: &tracev3.Tracing_Http{
			Name: "envoy.tracers.opentelemetry",
			ConfigType: &tracev3.Tracing_Http_TypedConfig{
				TypedConfig: otelConfigAny,
			},
		},
		SpawnUpstreamSpan: wrapperspb.Bool(true),
		RandomSampling: &typev3.Percent{
			Value: samplingPercentage,
		},
	}

	t.logger.Info("Tracing configuration created",
		slog.String("service_name", serviceName),
		slog.Float64("sampling_rate", samplingRate),
		slog.String("collector_cluster", OTELCollectorClusterName))

	return tracingConfig, nil
}

// convertToInterface converts map[string]string to map[string]interface{} for structpb
func convertToInterface(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	return result
}

// createLuaFilter creates an Envoy lua filter for request transformation
func (t *Translator) createLuaFilter() (*hcm.HttpFilter, error) {
	luaScriptPath := strings.TrimSpace(t.routerConfig.Lua.RequestTransformation.ScriptPath)
	if luaScriptPath == "" {
		luaScriptPath = strings.TrimSpace(t.routerConfig.LuaScriptPath)
	}
	if luaScriptPath == "" {
		luaScriptPath = config.DefaultLuaScriptPath
	}

	scriptBytes, err := os.ReadFile(luaScriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read lua script from %s: %w", luaScriptPath, err)
	}

	script := strings.TrimSpace(string(scriptBytes))
	if script == "" {
		return nil, fmt.Errorf("lua script at %s is empty", luaScriptPath)
	}

	luaConfig := &luav3.Lua{
		InlineCode: string(scriptBytes),
	}

	luaAny, err := anypb.New(luaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal lua config: %w", err)
	}

	return &hcm.HttpFilter{
		Name: "envoy.filters.http.lua",
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: luaAny,
		},
	}, nil
}

// createExtProcFilter creates an Envoy ext_proc filter for policy engine integration
func (t *Translator) createExtProcFilter() (*hcm.HttpFilter, error) {
	policyEngine := t.routerConfig.PolicyEngine

	// Convert route cache action string to enum
	routeCacheAction := extproc.ExternalProcessor_DEFAULT
	switch policyEngine.RouteCacheAction { // TODO: (renuka) This is not a config. Fix it.
	case constants.ExtProcRouteCacheActionRetain:
		routeCacheAction = extproc.ExternalProcessor_RETAIN
	case constants.ExtProcRouteCacheActionClear:
		routeCacheAction = extproc.ExternalProcessor_CLEAR
	}

	// Convert request header mode string to enum
	requestHeaderMode := extproc.ProcessingMode_DEFAULT
	switch policyEngine.RequestHeaderMode {
	case constants.ExtProcHeaderModeSend:
		requestHeaderMode = extproc.ProcessingMode_SEND
	case constants.ExtProcHeaderModeSkip:
		requestHeaderMode = extproc.ProcessingMode_SKIP
	}

	// Create ext_proc configuration
	extProcConfig := &extproc.ExternalProcessor{
		GrpcService: &core.GrpcService{
			TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
				EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
					ClusterName: constants.PolicyEngineClusterName,
				},
			},
			Timeout: durationpb.New(time.Duration(policyEngine.TimeoutMs) * time.Millisecond),
		},
		FailureModeAllow:  policyEngine.FailureModeAllow,
		RouteCacheAction:  routeCacheAction,
		AllowModeOverride: policyEngine.AllowModeOverride,
		RequestAttributes: []string{constants.ExtProcRequestAttributeRouteName, constants.ExtProcRequestAttributeRouteMetadata},
		ProcessingMode: &extproc.ProcessingMode{
			RequestHeaderMode: requestHeaderMode,
		},
		MessageTimeout: durationpb.New(time.Duration(policyEngine.MessageTimeoutMs) * time.Millisecond),
		MutationRules: &mutationrules.HeaderMutationRules{
			AllowAllRouting: wrapperspb.Bool(true),
		},
		MetadataOptions: &extproc.MetadataOptions{
			ReceivingNamespaces: &extproc.MetadataOptions_MetadataNamespaces{
				Untyped: []string{constants.ExtProcMetadataNamespace},
			},
			ForwardingNamespaces: &extproc.MetadataOptions_MetadataNamespaces{
				Untyped: []string{constants.ExtProcMetadataNamespace},
			},
		},
	}

	// Marshal to Any
	extProcAny, err := anypb.New(extProcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ext_proc config: %w", err)
	}

	return &hcm.HttpFilter{
		Name: constants.ExtProcFilterName,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: extProcAny,
		},
	}, nil
}

// resolveUpstreamDefinition finds an upstream definition by its reference name
// Returns the upstream definition and error if not found
func resolveUpstreamDefinition(ref string, definitions *[]api.UpstreamDefinition) (*api.UpstreamDefinition, error) {
	if definitions == nil {
		return nil, fmt.Errorf("upstream definition '%s' not found: no definitions provided", ref)
	}

	for _, def := range *definitions {
		if def.Name == ref {
			return &def, nil
		}
	}

	return nil, fmt.Errorf("upstream definition '%s' not found", ref)
}

// parseTimeout parses a duration string (e.g., "30s", "1m", "500ms") and returns a time.Duration.
// Returns nil if the input is nil or empty.
func parseTimeout(timeoutStr *string) (*time.Duration, error) {
	if timeoutStr == nil || strings.TrimSpace(*timeoutStr) == "" {
		return nil, nil
	}

	duration, err := time.ParseDuration(strings.TrimSpace(*timeoutStr))
	if err != nil {
		return nil, fmt.Errorf("invalid timeout format: %w", err)
	}

	return &duration, nil
}

// resolveTimeoutFromDefinition converts an UpstreamDefinition's timeout block into a resolvedTimeout.
// Returns nil if there is no timeout block or all fields are empty.
func resolveTimeoutFromDefinition(def *api.UpstreamDefinition) (*resolvedTimeout, error) {
	if def == nil || def.Timeout == nil {
		return nil, nil
	}

	var rt resolvedTimeout
	var err error

	if rt.Connect, err = parseTimeout(def.Timeout.Connect); err != nil {
		return nil, err
	}
	if rt.Request, err = parseTimeout(def.Timeout.Request); err != nil {
		return nil, err
	}
	if rt.Idle, err = parseTimeout(def.Timeout.Idle); err != nil {
		return nil, err
	}

	if rt.Connect == nil && rt.Request == nil && rt.Idle == nil {
		return nil, nil
	}

	return &rt, nil
}

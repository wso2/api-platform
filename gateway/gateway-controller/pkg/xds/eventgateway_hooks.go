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
	"log/slog"
	"net/url"
	"time"

	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// EventGatewayXDSHooks is the extension point through which an external
// event-gateway-controller binary supplies WebSub-specific xDS translation
// logic. Core never implements this interface itself; it is only ever
// satisfied by code living outside this module. See SetEventGatewayXDSHooks.
type EventGatewayXDSHooks interface {
	// BuildHubResources returns any extra clusters/listeners that should be
	// added to the shared xDS snapshot to support WebSubHub connectivity
	// (e.g. a dynamic-forward-proxy cluster and internal listener(s)).
	// httpsEnabled mirrors the core HTTPS-listener toggle so the hub
	// listener(s) can be built consistently with the rest of the snapshot.
	// Returning (nil, nil, nil) means "nothing to add".
	BuildHubResources(t *Translator, httpsEnabled bool) (clusters []*cluster.Cluster, listeners []*listener.Listener, err error)

	// TranslateWebSubAPI builds routes/clusters for a single WebSubApi
	// StoredConfig — the WebSubApi-kind equivalent of translateAPIConfig.
	TranslateWebSubAPI(t *Translator, cfg *models.StoredConfig, allConfigs []*models.StoredConfig) ([]*route.Route, []*cluster.Cluster, error)
}

// SetEventGatewayXDSHooks registers the event-gateway xDS extension. Passing
// nil (the default) means this binary has no event-gateway support compiled
// in, and any WebSubApi-kind config will fail translation with a clear error.
func (t *Translator) SetEventGatewayXDSHooks(h EventGatewayXDSHooks) {
	t.eventGatewayHooks = h
}

// The following exported accessors/wrappers exist solely so that an
// EventGatewayXDSHooks implementation living outside this module can reuse
// the same generic route/cluster-building primitives every other kind uses,
// without duplicating them.

// Logger returns the translator's logger.
func (t *Translator) Logger() *slog.Logger {
	return t.logger
}

// RouterConfig returns the translator's router configuration.
func (t *Translator) RouterConfig() *config.RouterConfig {
	return t.routerConfig
}

// Config returns the translator's full system configuration.
func (t *Translator) Config() *config.Config {
	return t.config
}

// CreateRoute exposes createRoute for use by EventGatewayXDSHooks implementations.
func (t *Translator) CreateRoute(apiId, apiName, apiVersion, context, method, path, clusterName,
	upstreamPath string, vhost string, apiKind string, templateHandle string, providerName string, hostRewrite *api.UpstreamHostRewrite, projectID string, timeoutCfg *resolvedTimeout, useClusterHeader bool, upstreamDefPaths map[string]string) *route.Route {
	return t.createRoute(apiId, apiName, apiVersion, context, method, path, clusterName,
		upstreamPath, vhost, apiKind, templateHandle, providerName, hostRewrite, projectID, timeoutCfg, useClusterHeader, upstreamDefPaths)
}

// CreateRoutePerTopic exposes createRoutePerTopic for use by EventGatewayXDSHooks implementations.
func (t *Translator) CreateRoutePerTopic(apiId, apiName, apiVersion, context, method, channelName, clusterName, vhost, apiKind, projectID string) *route.Route {
	return t.createRoutePerTopic(apiId, apiName, apiVersion, context, method, channelName, clusterName, vhost, apiKind, projectID)
}

// CreateCluster exposes createCluster for use by EventGatewayXDSHooks implementations.
func (t *Translator) CreateCluster(name string, upstreamURL *url.URL, upstreamCerts map[string][]byte, connectTimeout *time.Duration) *cluster.Cluster {
	return t.createCluster(name, upstreamURL, upstreamCerts, connectTimeout)
}

// ExtractTemplateHandle exposes extractTemplateHandle for use by EventGatewayXDSHooks implementations.
func (t *Translator) ExtractTemplateHandle(cfg *models.StoredConfig, allConfigs []*models.StoredConfig) string {
	return t.extractTemplateHandle(cfg, allConfigs)
}

// ExtractProviderName exposes extractProviderName for use by EventGatewayXDSHooks implementations.
func (t *Translator) ExtractProviderName(cfg *models.StoredConfig, allConfigs []*models.StoredConfig) string {
	return t.extractProviderName(cfg, allConfigs)
}

// ExtractProjectIDFromConfig exposes extractProjectIDFromConfig for use by EventGatewayXDSHooks implementations.
func ExtractProjectIDFromConfig(cfg *models.StoredConfig) string {
	return extractProjectIDFromConfig(cfg)
}

// EnvoyOriginalPathHeader exposes the envoyOriginalPathHeader constant for use
// by EventGatewayXDSHooks implementations building their own virtual hosts/routes.
const EnvoyOriginalPathHeader = envoyOriginalPathHeader

// CreateExtProcFilter exposes createExtProcFilter for use by EventGatewayXDSHooks implementations.
func (t *Translator) CreateExtProcFilter() (*hcm.HttpFilter, error) {
	return t.createExtProcFilter()
}

// CreateLuaFilter exposes createLuaFilter for use by EventGatewayXDSHooks implementations.
func (t *Translator) CreateLuaFilter() (*hcm.HttpFilter, error) {
	return t.createLuaFilter()
}

// CreateAccessLogConfig exposes createAccessLogConfig for use by EventGatewayXDSHooks implementations.
func (t *Translator) CreateAccessLogConfig() ([]*accesslog.AccessLog, error) {
	return t.createAccessLogConfig()
}

// CreateTracingConfig exposes createTracingConfig for use by EventGatewayXDSHooks implementations.
func (t *Translator) CreateTracingConfig() (*hcm.HttpConnectionManager_Tracing, error) {
	return t.createTracingConfig()
}

// CreateDownstreamTLSContext exposes createDownstreamTLSContext for use by EventGatewayXDSHooks implementations.
func (t *Translator) CreateDownstreamTLSContext() (*tlsv3.DownstreamTlsContext, error) {
	return t.createDownstreamTLSContext()
}

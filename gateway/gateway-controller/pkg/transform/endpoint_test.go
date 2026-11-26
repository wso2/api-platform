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

package transform

import (
	"io"
	"log/slog"
	"sort"
	"testing"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

const (
	routingPolicy     = "dynamic-endpoint"
	mainRouteKey      = "GET|/test/hello|main.local"
	mainClusterKey    = "upstream_main_backend_8080"
	endpointTestCA    = "partner-ca"
	endpointTestIdent = "partner-identity"
)

// endpointRoutingDefs is a policy set holding a routing policy shaped like
// dynamic-endpoint v1.1 (targetUpstream is its v1.0 parameter and carries no
// annotation) and an ordinary policy.
func endpointRoutingDefs() map[string]models.PolicyDefinition {
	return map[string]models.PolicyDefinition{
		routingPolicy + "|v1.1.0": {
			Name: routingPolicy, Version: "v1.1.0",
			Parameters: &map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"targetUpstream": map[string]interface{}{"type": "string"},
					"targetEndpoint": map[string]interface{}{
						"type": "string", config.EndpointRoutingAnnotation: config.EndpointRoutingEndpointName,
					},
					"endpointHeader": map[string]interface{}{
						"type": "string", config.EndpointRoutingAnnotation: config.EndpointRoutingHeaderName,
					},
					"endpointSelector": map[string]interface{}{"type": "object"},
				},
			},
		},
		"set-headers|v1.0.0": {Name: "set-headers", Version: "v1.0.0"},
	}
}

func routingPolicyWith(params map[string]interface{}) api.Policy {
	return api.Policy{Name: routingPolicy, Version: "v1", Params: &params}
}

// testStoredEndpoint builds a stored Endpoint with one target per URL.
func testStoredEndpoint(name string, urls ...string) *models.StoredEndpoint {
	endpoint := &models.StoredEndpoint{
		UUID: "uuid-" + name,
		Configuration: api.EndpointConfiguration{
			ApiVersion: api.EndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.EndpointConfigurationKindEndpoint,
			Metadata:   api.Metadata{Name: name},
		},
	}
	for _, u := range urls {
		endpoint.Configuration.Spec.Upstreams = append(endpoint.Configuration.Spec.Upstreams, struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{Url: u})
	}
	return endpoint
}

// ─── Divertible routes ──────────────────────────────────────────────────────

func TestRestAPITransformer_EndpointRoutingPolicyMakesTheRouteDivertible(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, endpointRoutingDefs())

	for name, params := range map[string]map[string]interface{}{
		"fixed target": {"targetEndpoint": "tenant-a"},
		"header mode": {
			"endpointHeader":   "x-endpoint",
			"endpointSelector": map[string]interface{}{"matchLabels": map[string]interface{}{"team": "orders"}},
		},
		"header mode with a local fallback": {"endpointHeader": "x-endpoint", "targetUpstream": "local"},
	} {
		t.Run(name, func(t *testing.T) {
			rdc, err := transformer.Transform(makeRestAPIStoredConfig([]api.Policy{routingPolicyWith(params)}, nil))
			require.NoError(t, err)

			r := rdc.Routes[mainRouteKey]
			require.NotNil(t, r)
			assert.True(t, r.Upstream.UseClusterHeader, "the policy engine must be able to choose the cluster")
			// The default is what a request gets when the policy sets no target,
			// so it has to be a cluster Envoy really has.
			assert.Equal(t, mainClusterKey, r.Upstream.DefaultCluster)
			assert.Contains(t, rdc.UpstreamClusters, r.Upstream.DefaultCluster)
			assert.Equal(t, mainClusterKey, r.Upstream.ClusterKey)
		})
	}
}

// The rule is per route: an operation without the policy keeps its fixed cluster.
func TestRestAPITransformer_EndpointRoutingIsDecidedPerRoute(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, endpointRoutingDefs())

	cfg := makeRestAPIStoredConfig(nil, nil)
	restAPI := cfg.Configuration.(api.RestAPI)
	routed := []api.Policy{routingPolicyWith(map[string]interface{}{"targetEndpoint": "tenant-a"})}
	restAPI.Spec.Operations = []api.Operation{
		{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/hello"), Policies: &routed},
		{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/plain")},
	}
	cfg.Configuration = restAPI

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	assert.True(t, rdc.Routes[mainRouteKey].Upstream.UseClusterHeader)
	plain := rdc.Routes["GET|/test/plain|main.local"]
	require.NotNil(t, plain)
	assert.False(t, plain.Upstream.UseClusterHeader)
	assert.Empty(t, plain.Upstream.DefaultCluster)
}

// v1.0-style configurations keep today's rule exactly.
func TestRestAPITransformer_PoliciesThatDoNotRouteToEndpointsChangeNothing(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, endpointRoutingDefs())

	for name, policies := range map[string][]api.Policy{
		"no policy":                 nil,
		"an ordinary policy":        {{Name: "set-headers", Version: "v1"}},
		"only the v1.0 parameter":   {routingPolicyWith(map[string]interface{}{"targetUpstream": "local"})},
		"an empty routing param":    {routingPolicyWith(map[string]interface{}{"targetEndpoint": ""})},
		"a policy that is unloaded": {{Name: "unknown", Version: "v1", Params: &map[string]interface{}{"targetEndpoint": "tenant-a"}}},
	} {
		t.Run(name, func(t *testing.T) {
			rdc, err := transformer.Transform(makeRestAPIStoredConfig(policies, nil))
			require.NoError(t, err)
			r := rdc.Routes[mainRouteKey]
			assert.False(t, r.Upstream.UseClusterHeader)
			assert.Empty(t, r.Upstream.DefaultCluster)
		})
	}
}

func TestRestAPITransformer_EndpointRoutingOnASandboxRouteDefaultsToTheSandbox(t *testing.T) {
	transformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, endpointRoutingDefs())

	cfg := makeRestAPIStoredConfig([]api.Policy{routingPolicyWith(map[string]interface{}{"targetEndpoint": "tenant-a"})}, nil)
	restAPI := cfg.Configuration.(api.RestAPI)
	restAPI.Spec.Upstream.Sandbox = &api.Upstream{Url: ptrStr("http://sandbox-backend:9080")}
	cfg.Configuration = restAPI

	rdc, err := transformer.Transform(cfg)
	require.NoError(t, err)

	main := rdc.Routes[mainRouteKey]
	assert.True(t, main.Upstream.UseClusterHeader)
	assert.Equal(t, mainClusterKey, main.Upstream.DefaultCluster)

	sandbox := rdc.Routes["GET|/test/hello|sandbox.local"]
	require.NotNil(t, sandbox)
	assert.True(t, sandbox.Upstream.UseClusterHeader)
	assert.Equal(t, "upstream_sandbox_sandbox-backend_9080", sandbox.Upstream.DefaultCluster)
}

// ─── Shared converter ───────────────────────────────────────────────────────

func TestUpstreamClusterFromDefinition(t *testing.T) {
	weightA, weightB := 80, 20
	basePath, connect := "/v1", "2s"
	def := api.UpstreamDefinition{
		Name:     "tenant-a",
		BasePath: &basePath,
		Timeout:  &api.UpstreamTimeout{Connect: &connect},
		Tls:      &map[string]interface{}{"identity": endpointTestIdent, "trustedCAs": []interface{}{endpointTestCA}},
	}
	def.Upstreams = append(def.Upstreams,
		struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{Url: "https://orders-a:9443", Weight: &weightA},
		struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{Url: "https://orders-b", Weight: &weightB},
	)

	uc, err := UpstreamClusterFromDefinition(def)
	require.NoError(t, err)
	assert.Equal(t, "tenant-a", uc.Name)
	assert.Equal(t, "/v1", uc.BasePath)
	require.NotNil(t, uc.ConnectTimeout)
	assert.Equal(t, 2*time.Second, *uc.ConnectTimeout)
	assert.Equal(t, []models.Endpoint{
		{Host: "orders-a", Port: 9443, Weight: &weightA},
		{Host: "orders-b", Port: 443, Weight: &weightB},
	}, uc.Endpoints, "every target is kept, with its weight")
	require.NotNil(t, uc.TLS)
	assert.True(t, uc.TLS.Enabled)
	assert.True(t, uc.TLS.HasTLSBlock)
	assert.Equal(t, endpointTestIdent, uc.TLS.IdentityName)
	assert.Equal(t, []string{endpointTestCA}, uc.TLS.TrustedCANames)
}

func TestUpstreamClusterFromDefinition_DefaultsAndErrors(t *testing.T) {
	plain := testStoredEndpoint("tenant-a", "http://orders-a:9080").UpstreamDefinition()
	uc, err := UpstreamClusterFromDefinition(plain)
	require.NoError(t, err)
	assert.Equal(t, "/", uc.BasePath)
	assert.Nil(t, uc.ConnectTimeout)
	assert.False(t, uc.TLS.Enabled)

	mixed := testStoredEndpoint("tenant-a", "https://orders-a:9443", "http://orders-b:9080").UpstreamDefinition()
	_, err = UpstreamClusterFromDefinition(mixed)
	require.ErrorContains(t, err, "mixes https and non-https")

	badTimeout := "soon"
	bad := testStoredEndpoint("tenant-a", "http://orders-a:9080").UpstreamDefinition()
	bad.Timeout = &api.UpstreamTimeout{Connect: &badTimeout}
	_, err = UpstreamClusterFromDefinition(bad)
	require.Error(t, err)
}

// An API's own definition and an Endpoint with the same fields must produce
// the same cluster model: they share one converter.
func TestEndpointAndUpstreamDefinitionProduceTheSameCluster(t *testing.T) {
	endpoint := testStoredEndpoint("shared-backend", "http://orders-a:9080", "http://orders-b:9080")
	basePath := "/v1"
	endpoint.Configuration.Spec.BasePath = &basePath
	fromEndpoint, err := EndpointUpstreamCluster(endpoint)
	require.NoError(t, err)

	cfg := makeRestAPIStoredConfig(nil, nil)
	restAPI := cfg.Configuration.(api.RestAPI)
	restAPI.Spec.UpstreamDefinitions = &[]api.UpstreamDefinition{endpoint.UpstreamDefinition()}
	cfg.Configuration = restAPI
	rdc, err := NewRestAPITransformer(testRouterCfg(), &config.Config{}, nil).Transform(cfg)
	require.NoError(t, err)

	assert.Equal(t, fromEndpoint, rdc.UpstreamClusters["upstream_RestApi_test-api_shared-backend"])
}

// ─── Cluster source ─────────────────────────────────────────────────────────

func TestEndpointClusterSource(t *testing.T) {
	store := storage.NewConfigStore()
	require.NoError(t, store.SetEndpoint(testStoredEndpoint("tenant-b", "http://orders-b:9080")))
	require.NoError(t, store.SetEndpoint(testStoredEndpoint("tenant-a", "http://orders-a:9080")))
	// Stored by something other than this gateway's validator.
	require.NoError(t, store.SetEndpoint(testStoredEndpoint("tenant-mixed", "https://x:9443", "http://y:9080")))

	clusters := NewEndpointClusterSource(store, slog.New(slog.NewTextHandler(io.Discard, nil))).EndpointClusters()

	var names []string
	for _, c := range clusters {
		names = append(names, c.Name)
		require.NotNil(t, c.Cluster)
	}
	assert.Equal(t, []string{"tenant-a", "tenant-b"}, names,
		"ordered by name, and an Endpoint that cannot become a cluster is left out")
}

// ─── Through the Envoy translator ───────────────────────────────────────────

// endpointTranslator builds an Envoy translator over the RestAPI transformer
// with the given policy definitions, reading Endpoints from store.
func endpointTranslator(t *testing.T, defs map[string]models.PolicyDefinition, store *storage.ConfigStore, db storage.Storage) *xds.Translator {
	t.Helper()
	routerCfg := testRouterCfg()
	routerCfg.LuaScriptPath = "../../lua/request_transformation.lua"
	systemCfg := &config.Config{Router: *routerCfg}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	translator, err := xds.NewTranslator(logger, routerCfg, db, systemCfg)
	require.NoError(t, err)
	restT := NewRestAPITransformer(routerCfg, systemCfg, defs)
	translator.SetTransformers(map[string]models.ConfigTransformer{models.KindRestApi: NewRegistry(restT, nil, nil, nil)})
	if store != nil {
		translator.SetEndpointSource(NewEndpointClusterSource(store, logger))
	}
	return translator
}

func deployedRestAPI(policies []api.Policy) *models.StoredConfig {
	cfg := makeRestAPIStoredConfig(policies, nil)
	cfg.Handle = "test-api"
	cfg.DesiredState = models.StateDeployed
	return cfg
}

func clustersByName(resources map[resource.Type][]types.Resource) map[string]*cluster.Cluster {
	out := map[string]*cluster.Cluster{}
	for _, res := range resources[resource.ClusterType] {
		c := res.(*cluster.Cluster)
		out[c.GetName()] = c
	}
	return out
}

func routeByName(t *testing.T, resources map[resource.Type][]types.Resource, name string) *route.Route {
	t.Helper()
	for _, res := range resources[resource.RouteType] {
		for _, vh := range res.(*route.RouteConfiguration).GetVirtualHosts() {
			for _, r := range vh.GetRoutes() {
				if r.GetName() == name {
					return r
				}
			}
		}
	}
	t.Fatalf("route %q not found", name)
	return nil
}

func TestTranslator_EmitsOneClusterPerEndpoint(t *testing.T) {
	store := storage.NewConfigStore()
	single := testStoredEndpoint("tenant-a", "http://orders-tenant-a:9080")
	weighted := testStoredEndpoint("tenant-b", "http://orders-b1:9080", "http://orders-b2:9081")
	w1, w2 := 80, 20
	weighted.Configuration.Spec.Upstreams[0].Weight = &w1
	weighted.Configuration.Spec.Upstreams[1].Weight = &w2
	connect := "2s"
	weighted.Configuration.Spec.Timeout = &api.UpstreamTimeout{Connect: &connect}
	require.NoError(t, store.SetEndpoint(single))
	require.NoError(t, store.SetEndpoint(weighted))

	translator := endpointTranslator(t, endpointRoutingDefs(), store, nil)

	// No API at all: the clusters exist anyway, so an Endpoint is routable the
	// moment a policy names it.
	resources, err := translator.TranslateConfigs(nil, "")
	require.NoError(t, err)
	clusters := clustersByName(resources)

	a := clusters["endpoint_tenant-a"]
	require.NotNil(t, a, "clusters: %v", clusterNamesOf(clusters))
	hosts := lbHosts(a)
	assert.Equal(t, []string{"orders-tenant-a:9080"}, hosts)

	b := clusters["endpoint_tenant-b"]
	require.NotNil(t, b)
	assert.Equal(t, []string{"orders-b1:9080", "orders-b2:9081"}, lbHosts(b))
	assert.Equal(t, []uint32{80, 20}, lbWeights(b), "weights are applied, not dropped")
	assert.Equal(t, 2*time.Second, b.GetConnectTimeout().AsDuration())
}

func TestTranslator_DivertibleRouteReadsItsClusterFromTheHeader(t *testing.T) {
	store := storage.NewConfigStore()
	require.NoError(t, store.SetEndpoint(testStoredEndpoint("tenant-a", "http://orders-tenant-a:9080")))
	translator := endpointTranslator(t, endpointRoutingDefs(), store, nil)

	api := deployedRestAPI([]api.Policy{routingPolicyWith(map[string]interface{}{"endpointHeader": "x-endpoint"})})
	resources, err := translator.TranslateConfigs([]*models.StoredConfig{api}, "")
	require.NoError(t, err)

	r := routeByName(t, resources, mainRouteKey)
	assert.Equal(t, constants.TargetUpstreamHeader, r.GetRoute().GetClusterHeader())
	assert.Contains(t, r.GetRequestHeadersToRemove(), constants.TargetUpstreamHeader,
		"the internal routing header must not reach the backend")

	clusters := clustersByName(resources)
	assert.Contains(t, clusters, "endpoint_tenant-a")
	assert.Contains(t, clusters, mainClusterKey, "the route's own cluster is still there as the default")
}

// An API with no endpoint-routing parameter must get exactly the xDS it got
// before Endpoints existed — whether or not the routing policy is installed,
// and whether or not the gateway has Endpoints.
func TestTranslator_APIsWithoutEndpointRoutingAreUnchanged(t *testing.T) {
	plainAPIs := func() []*models.StoredConfig {
		withHeaders := deployedRestAPI([]api.Policy{{Name: "set-headers", Version: "v1"}})
		return []*models.StoredConfig{withHeaders}
	}
	baselineDefs := map[string]models.PolicyDefinition{"set-headers|v1.0.0": {Name: "set-headers", Version: "v1.0.0"}}

	baseline, err := endpointTranslator(t, baselineDefs, nil, nil).TranslateConfigs(plainAPIs(), "")
	require.NoError(t, err)

	store := storage.NewConfigStore()
	require.NoError(t, store.SetEndpoint(testStoredEndpoint("tenant-a", "http://orders-tenant-a:9080")))
	withEndpoints, err := endpointTranslator(t, endpointRoutingDefs(), store, nil).TranslateConfigs(plainAPIs(), "")
	require.NoError(t, err)

	assertSameRoutes(t, baseline, withEndpoints)
	require.Len(t, withEndpoints[resource.ListenerType], len(baseline[resource.ListenerType]))

	baseClusters, newClusters := clustersByName(baseline), clustersByName(withEndpoints)
	for name, want := range baseClusters {
		got, ok := newClusters[name]
		require.True(t, ok, "cluster %q disappeared", name)
		assert.True(t, proto.Equal(want, got), "cluster %q changed", name)
	}
	delete(newClusters, "endpoint_tenant-a")
	assert.Len(t, newClusters, len(baseClusters), "the only new cluster is the Endpoint's own")

	// And with no Endpoints at all the snapshot is the same resource for resource.
	noEndpoints, err := endpointTranslator(t, endpointRoutingDefs(), storage.NewConfigStore(), nil).TranslateConfigs(plainAPIs(), "")
	require.NoError(t, err)
	require.Len(t, noEndpoints[resource.ClusterType], len(baseline[resource.ClusterType]))
	for name, want := range baseClusters {
		assert.True(t, proto.Equal(want, clustersByName(noEndpoints)[name]), "cluster %q changed", name)
	}
}

// Changing an Endpoint's backend replaces only its own cluster: the name is
// stable, so no route and no other cluster moves.
func TestTranslator_EndpointChangeTouchesOnlyItsCluster(t *testing.T) {
	store := storage.NewConfigStore()
	require.NoError(t, store.SetEndpoint(testStoredEndpoint("tenant-a", "http://orders-tenant-a:9080")))
	translator := endpointTranslator(t, endpointRoutingDefs(), store, nil)
	apis := []*models.StoredConfig{deployedRestAPI([]api.Policy{routingPolicyWith(map[string]interface{}{"targetEndpoint": "tenant-a"})})}

	before, err := translator.TranslateConfigs(apis, "")
	require.NoError(t, err)

	require.NoError(t, store.SetEndpoint(testStoredEndpoint("tenant-a", "http://orders-moved:9090")))
	after, err := translator.TranslateConfigs(apis, "")
	require.NoError(t, err)

	assertSameRoutes(t, before, after)
	beforeClusters, afterClusters := clustersByName(before), clustersByName(after)
	for name := range beforeClusters {
		if name == "endpoint_tenant-a" {
			continue
		}
		assert.True(t, proto.Equal(beforeClusters[name], afterClusters[name]), "cluster %q must not change", name)
	}
	assert.Equal(t, []string{"orders-moved:9090"}, lbHosts(afterClusters["endpoint_tenant-a"]))
}

func TestTranslator_EndpointTLSUsesItsOwnSecrets(t *testing.T) {
	mgr, ciphertext := encryptedTestKey(t)
	db := &certificateRows{certs: []*models.StoredCertificate{
		{UUID: "ca-1", Name: endpointTestCA, Certificate: pki.NewRootCA(t, "Partner CA").PEM(), Usage: models.CertificateUsageUpstream},
		{UUID: "id-1", Name: endpointTestIdent, Certificate: pki.NewSelfSignedLeaf(t, "partner").PEM(),
			Usage: models.CertificateUsageIdentity, PrivateKeyCiphertext: ciphertext},
	}}
	store := storage.NewConfigStore()
	secured := testStoredEndpoint("tenant-a", "https://orders-tenant-a:9443")
	secured.Configuration.Spec.Tls = &map[string]interface{}{
		"identity":   endpointTestIdent,
		"trustedCAs": []interface{}{endpointTestCA},
	}
	require.NoError(t, store.SetEndpoint(secured))

	translator := endpointTranslator(t, endpointRoutingDefs(), store, db)
	translator.GetCertStore().SetEncryptionManager(mgr)

	resources, err := translator.TranslateConfigs(nil, "")
	require.NoError(t, err)
	c := clustersByName(resources)["endpoint_tenant-a"]
	require.NotNil(t, c)

	var identities, trustBundles []string
	for _, tsm := range c.GetTransportSocketMatches() {
		var tlsCtx tlsv3.UpstreamTlsContext
		require.NoError(t, tsm.GetTransportSocket().GetTypedConfig().UnmarshalTo(&tlsCtx))
		for _, sds := range tlsCtx.GetCommonTlsContext().GetTlsCertificateSdsSecretConfigs() {
			identities = append(identities, sds.GetName())
		}
		if combined := tlsCtx.GetCommonTlsContext().GetCombinedValidationContext(); combined != nil {
			trustBundles = append(trustBundles, combined.GetValidationContextSdsSecretConfig().GetName())
		}
	}
	assert.Equal(t, []string{xds.GatewayIdentitySecretName(endpointTestIdent)}, identities)
	// The trust bundle belongs to the Endpoint, under a name no API's
	// definition can produce.
	assert.Equal(t, []string{"endpoint_ca:tenant-a"}, trustBundles)
	assert.Equal(t, "endpoint_ca:tenant-a", xds.EndpointCAValidationContextSecretName("tenant-a"))

	refs := translator.GetUpstreamTLSSecretRefs()
	require.Len(t, refs, 1)
	assert.Equal(t, "endpoint_ca:tenant-a", refs[0].SecretName)
	assert.Equal(t, endpointTestIdent, refs[0].IdentityName)
	assert.Equal(t, []string{endpointTestCA}, refs[0].TrustedCANames)

	// SDS serves both secrets the cluster names.
	sds := xds.NewSDSSecretManager(translator.GetCertStore(), nil, "router-node",
		slog.New(slog.NewTextHandler(io.Discard, nil)), "", "", false)
	secrets, err := sds.GetSecrets(refs)
	require.NoError(t, err)
	var secretNames []string
	for _, s := range secrets {
		secretNames = append(secretNames, s.(*tlsv3.Secret).GetName())
	}
	assert.Contains(t, secretNames, "endpoint_ca:tenant-a")
	assert.Contains(t, secretNames, xds.GatewayIdentitySecretName(endpointTestIdent))
	for _, name := range []string{"endpoint_ca:tenant-a", xds.GatewayIdentitySecretName(endpointTestIdent)} {
		assert.True(t, xds.SnapshotReferencesSDSSecret(resources[resource.ClusterType], nil, name),
			"secret %q must be included in the snapshot", name)
	}
}

// allRoutes returns every route of every virtual host, keyed by virtual host
// name and route name. Virtual hosts are emitted in map order, so routes are
// compared by name rather than by position.
func allRoutes(resources map[resource.Type][]types.Resource) map[string]*route.Route {
	out := map[string]*route.Route{}
	for _, res := range resources[resource.RouteType] {
		for _, vh := range res.(*route.RouteConfiguration).GetVirtualHosts() {
			for i, r := range vh.GetRoutes() {
				key := vh.GetName() + "/" + r.GetName()
				if r.GetName() == "" {
					key += "#" + itoa(uint32(i))
				}
				out[key] = r
			}
		}
	}
	return out
}

// assertSameRoutes fails unless both snapshots hold exactly the same routes.
func assertSameRoutes(t *testing.T, want, got map[resource.Type][]types.Resource) {
	t.Helper()
	wantRoutes, gotRoutes := allRoutes(want), allRoutes(got)
	require.Len(t, gotRoutes, len(wantRoutes))
	for name, w := range wantRoutes {
		g, ok := gotRoutes[name]
		require.True(t, ok, "route %q disappeared", name)
		assert.True(t, proto.Equal(w, g), "route %q changed", name)
	}
}

func clusterNamesOf(m map[string]*cluster.Cluster) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// lbHosts returns the host:port of every load-balancing endpoint of c.
func lbHosts(c *cluster.Cluster) []string {
	var hosts []string
	for _, locality := range c.GetLoadAssignment().GetEndpoints() {
		for _, lb := range locality.GetLbEndpoints() {
			addr := lb.GetEndpoint().GetAddress().GetSocketAddress()
			hosts = append(hosts, addr.GetAddress()+":"+itoa(addr.GetPortValue()))
		}
	}
	return hosts
}

// lbWeights returns the load-balancing weight of every endpoint of c.
func lbWeights(c *cluster.Cluster) []uint32 {
	var weights []uint32
	for _, locality := range c.GetLoadAssignment().GetEndpoints() {
		for _, lb := range locality.GetLbEndpoints() {
			weights = append(weights, lb.GetLoadBalancingWeight().GetValue())
		}
	}
	return weights
}

func itoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

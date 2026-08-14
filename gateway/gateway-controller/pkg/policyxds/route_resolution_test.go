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

package policyxds

import (
	"encoding/json"
	"log/slog"
	"os"
	"sort"
	"testing"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/common/chainkey"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// newTestRuntimeStore is a plain in-memory RuntimeConfigStore, so the tests below
// assert on what actually reached storage.
func newTestRuntimeStore() *storage.RuntimeConfigStore {
	return storage.NewRuntimeConfigStore()
}

func testTranslator() *Translator {
	return NewTranslator(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
}

// decodeRouteConfig unwraps an emitted RouteConfig resource back into the map the
// policy engine's xDS handler parses, so a test asserts on exactly what goes on the
// wire rather than on an intermediate struct.
func decodeRouteConfig(t *testing.T, res types.Resource) map[string]interface{} {
	t.Helper()
	outer, ok := res.(*anypb.Any)
	require.True(t, ok, "route config resource must be an Any")

	s := &structpb.Struct{}
	require.NoError(t, proto.Unmarshal(outer.Value, s))
	return s.AsMap()
}

// restRDC is a RuntimeDeployConfig shaped exactly the way RestAPITransformer builds
// one: an RDC-level "route-key" resolver, no per-route resolution fields at all.
func restRDC() *models.RuntimeDeployConfig {
	return &models.RuntimeDeployConfig{
		Metadata: models.Metadata{
			UUID: "api-1", Kind: "RestApi", Handle: "petstore",
			Version: "v1", DisplayName: "Petstore", ProjectID: "proj-1",
		},
		Context:             "/petstore",
		PolicyChainResolver: models.RouteKeyResolverName,
		Routes: map[string]*models.Route{
			"GET|/petstore/v1/pets|localhost": {
				Method: "GET", Path: "/petstore/v1/pets", OperationPath: "/pets",
				Vhost:    "localhost",
				Upstream: models.RouteUpstream{ClusterKey: "upstream_main"},
			},
			"POST|/petstore/v1/pets|localhost": {
				Method: "POST", Path: "/petstore/v1/pets", OperationPath: "/pets",
				Vhost:    "localhost",
				Upstream: models.RouteUpstream{ClusterKey: "upstream_main"},
			},
		},
		PolicyChains: map[string]*models.PolicyChain{
			"GET|/petstore/v1/pets|localhost":  {Policies: []models.Policy{{Name: "jwt-auth", Version: "v1"}}},
			"POST|/petstore/v1/pets|localhost": {Policies: []models.Policy{{Name: "jwt-auth", Version: "v1"}}},
		},
		UpstreamClusters: map[string]*models.UpstreamCluster{
			"upstream_main": {BasePath: "/", Endpoints: []models.Endpoint{{Host: "localhost", Port: 8080}}},
		},
	}
}

// ─── Invariant 5.2: no unintended RouteConfig churn ──────────────────────────

// An existing kind's RouteConfig resource must gain exactly one field —
// canonical_chain_key — and nothing else. Anything more re-versions every
// RouteConfig resource on every gateway at upgrade for no behavioural reason; a field
// serialising as {} or 0 is the usual way that happens.
func TestExistingKindGainsOnlyCanonicalChainKey(t *testing.T) {
	// The exact field set a pre-resolution controller emitted for a REST route.
	before := []string{
		"route_key", "metadata", "resolver_name",
		"upstream_base_path", "upstream_definition_paths",
	}

	resources, err := testTranslator().TranslateRuntimeConfigs([]*models.RuntimeDeployConfig{restRDC()})
	require.NoError(t, err)

	routes := resources[RouteConfigTypeURL]
	require.Len(t, routes, 2)

	for routeKey, res := range routes {
		data := decodeRouteConfig(t, res)

		got := make([]string, 0, len(data))
		for k := range data {
			got = append(got, k)
		}
		sort.Strings(got)

		want := append([]string{}, before...)
		want = append(want, "canonical_chain_key")
		sort.Strings(want)

		assert.Equal(t, want, got,
			"route %q must emit the pre-change field set plus canonical_chain_key and nothing else", routeKey)

		// For an identity route the new field's value is the route key, so nothing
		// about which chain is selected changes.
		assert.Equal(t, routeKey, data["canonical_chain_key"])
		assert.Equal(t, models.RouteKeyResolverName, data["resolver_name"])
	}
}

// The omission rule, stated directly: these three fields must be absent — not
// present-and-empty — on a route that does not need request-time resolution.
func TestEmptyResolutionFieldsAreOmitted(t *testing.T) {
	resources, err := testTranslator().TranslateRuntimeConfigs([]*models.RuntimeDeployConfig{restRDC()})
	require.NoError(t, err)

	for routeKey, res := range resources[RouteConfigTypeURL] {
		data := decodeRouteConfig(t, res)
		// operation_map is checked too: the field is gone from the contract entirely, so
		// a route emitting one would mean a stale code path is still writing it.
		for _, field := range []string{"operation_map", "resolver_config", "max_request_body_bytes"} {
			_, present := data[field]
			assert.False(t, present, "route %q must omit %q when unset, not emit an empty value", routeKey, field)
		}
	}
}

// Golden test: the complete emitted content for an existing kind's route, pinned
// value by value. It is a content comparison rather than a byte comparison on purpose
// — the resource bytes are produced by anypb.New over a Struct whose map fields have
// no defined wire order, and the LinearCache re-versions every resource it is handed
// regardless ("we assume all resources passed to SetResources are changed"), so byte
// stability is neither achievable nor what protects behaviour here. What protects it is
// that the *content* an existing kind emits is exactly what it was, plus the one new
// field whose value is the route key.
func TestExistingKindGoldenRouteConfigContent(t *testing.T) {
	resources, err := testTranslator().TranslateRuntimeConfigs([]*models.RuntimeDeployConfig{restRDC()})
	require.NoError(t, err)

	const routeKey = "GET|/petstore/v1/pets|localhost"
	got := decodeRouteConfig(t, resources[RouteConfigTypeURL][routeKey])

	assert.Equal(t, map[string]interface{}{
		"route_key": routeKey,
		"metadata": map[string]interface{}{
			"uuid":         "api-1",
			"kind":         "RestApi",
			"handle":       "petstore",
			"version":      "v1",
			"display_name": "Petstore",
			"project_id":   "proj-1",
			"api_context":  "/petstore",
			"vhost":        "localhost",
			"path":         "/pets",
		},
		"resolver_name":             models.RouteKeyResolverName,
		"upstream_base_path":        "/",
		"upstream_definition_paths": map[string]interface{}{},
		// The only addition. Equal to the route key, so which chain gets selected is
		// unchanged for every kind shipping today.
		"canonical_chain_key": routeKey,
	}, got)
}

// ─── Per-route resolver selection ────────────────────────────────────────────

// The reason resolver selection moved onto the route: one API can hold both shapes at
// once. Routes that need request-time resolution name a protocol resolver and are
// configured per route; routes that do not stay ordinary directly-resolved routes with
// their own chain key.
//
// No real protocol resolver exists yet, so this uses a placeholder name and asserts only
// the *wire shape* the model already enforces. What it pins is the pairing rule: a route
// naming a protocol resolver carries `resolver_config` and no `canonical_chain_key`,
// because the resolver's own configuration is the single source of the key and a second
// copy could disagree with nothing to arbitrate.
func TestProtocolResolvedRoutesCarryConfigAndNoChainKey(t *testing.T) {
	const resolverName = "fake-protocol"

	rdc := &models.RuntimeDeployConfig{
		Metadata: models.Metadata{UUID: "api-2", Kind: "RestApi", Handle: "svc", Version: "v1"},
		Context:  "/svc",
		// The RDC-level default stays directly-resolved; only the routes that need
		// request-time resolution override it.
		PolicyChainResolver: models.RouteKeyResolverName,
		Routes: map[string]*models.Route{
			// Many operations on one HTTP route: the operation is only knowable from the
			// request, so the resolver composes a key per request.
			"POST|/svc/v1/rpc|localhost": {
				Method: "POST", Path: "/svc/v1/rpc", Vhost: "localhost",
				ResolverName:        resolverName,
				ResolverConfig:      json.RawMessage(`{"mode":"multiplexed"}`),
				MaxRequestBodyBytes: 65536,
				Upstream:            models.RouteUpstream{ClusterKey: "upstream_main"},
			},
			// One route per operation, named in the route's own config. Same resolver,
			// different configuration — which is what makes two routes of one resolver
			// independent rather than forcing one shape on both.
			"POST|/svc/v1/op-one|localhost": {
				Method: "POST", Path: "/svc/v1/op-one", Vhost: "localhost",
				ResolverName:   resolverName,
				ResolverConfig: json.RawMessage(`{"mode":"single","operation":"OperationOne"}`),
				Upstream:       models.RouteUpstream{ClusterKey: "upstream_main"},
			},
			// Needs no resolution at all, so it keeps its own route-key chain.
			"GET|/svc/v1/status|localhost": {
				Method: "GET", Path: "/svc/v1/status", Vhost: "localhost",
				Upstream: models.RouteUpstream{ClusterKey: "upstream_main"},
			},
		},
		PolicyChains: map[string]*models.PolicyChain{
			chainkey.For("api-2", "localhost", "OperationOne"): {Policies: []models.Policy{{Name: "jwt-auth", Version: "v1"}}},
			chainkey.For("api-2", "localhost", "OperationTwo"): {Policies: []models.Policy{{Name: "jwt-auth", Version: "v1"}}},
			"GET|/svc/v1/status|localhost":                     {},
		},
		UpstreamClusters: map[string]*models.UpstreamCluster{
			"upstream_main": {BasePath: "/", Endpoints: []models.Endpoint{{Host: "localhost", Port: 8080}}},
		},
	}

	// A regression test for the pairing rule: emitting a canonical key beside
	// resolver_config would make the controller reject its own artifact.
	require.NoError(t, rdc.ValidateResolution())

	resources, err := testTranslator().TranslateRuntimeConfigs([]*models.RuntimeDeployConfig{rdc})
	require.NoError(t, err)
	routes := resources[RouteConfigTypeURL]
	require.Len(t, routes, 3)

	multiplexed := decodeRouteConfig(t, routes["POST|/svc/v1/rpc|localhost"])
	assert.Equal(t, resolverName, multiplexed["resolver_name"])
	assert.Equal(t, map[string]interface{}{"mode": "multiplexed"}, multiplexed["resolver_config"])
	assert.Equal(t, float64(65536), multiplexed["max_request_body_bytes"])
	_, hasMap := multiplexed["operation_map"]
	assert.False(t, hasMap, "there is no operation map on the wire under composed keys")

	perOperation := decodeRouteConfig(t, routes["POST|/svc/v1/op-one|localhost"])
	assert.Equal(t, resolverName, perOperation["resolver_name"])
	assert.Equal(t, map[string]interface{}{"mode": "single", "operation": "OperationOne"},
		perOperation["resolver_config"])

	for routeKey, data := range map[string]map[string]interface{}{
		"POST|/svc/v1/rpc|localhost":    multiplexed,
		"POST|/svc/v1/op-one|localhost": perOperation,
	} {
		_, hasKey := data["canonical_chain_key"]
		assert.False(t, hasKey,
			"route %q names a protocol resolver, so its key comes from resolver_config alone", routeKey)
	}

	status := decodeRouteConfig(t, routes["GET|/svc/v1/status|localhost"])
	assert.Equal(t, models.RouteKeyResolverName, status["resolver_name"])
	assert.Equal(t, "GET|/svc/v1/status|localhost", status["canonical_chain_key"],
		"a route that needs no resolution still carries its own key")
}

func TestEffectiveResolverName(t *testing.T) {
	rdc := &models.RuntimeDeployConfig{PolicyChainResolver: models.RouteKeyResolverName}

	assert.Equal(t, models.RouteKeyResolverName, rdc.EffectiveResolverName(&models.Route{}),
		"an empty route override inherits the RDC default")
	assert.Equal(t, "fake-multiplexed", rdc.EffectiveResolverName(&models.Route{ResolverName: "fake-multiplexed"}))

	// An RDC that sets no default at all still emits the empty value the policy engine
	// treats as identity, so nothing changes for a transformer that never set it.
	empty := &models.RuntimeDeployConfig{}
	assert.Equal(t, "", empty.EffectiveResolverName(&models.Route{}))
}

// ─── Invariant 5.5: referential integrity ────────────────────────────────────

func TestValidateResolution(t *testing.T) {
	chains := func(keys ...string) map[string]*models.PolicyChain {
		m := make(map[string]*models.PolicyChain, len(keys))
		for _, k := range keys {
			m[k] = &models.PolicyChain{}
		}
		return m
	}

	tests := []struct {
		name    string
		rdc     *models.RuntimeDeployConfig
		wantErr string
	}{
		{
			name: "identity route with its own chain",
			rdc: &models.RuntimeDeployConfig{
				PolicyChainResolver: models.RouteKeyResolverName,
				Routes:              map[string]*models.Route{"GET|/pets|h": {}},
				PolicyChains:        chains("GET|/pets|h"),
			},
		},
		{
			name: "identity route with an explicit canonical key equal to its route key",
			rdc: &models.RuntimeDeployConfig{
				Routes:       map[string]*models.Route{"GET|/pets|h": {CanonicalChainKey: "GET|/pets|h"}},
				PolicyChains: chains("GET|/pets|h"),
			},
		},
		{
			// A chain that exists but is neither this route's key nor a composed operation
			// key. Existence alone would accept it, and the route would then run whatever
			// policies that chain carries — the borrowed-policies failure is silent, so it
			// has to be refused here.
			name: "identity route pointed at an arbitrary existing chain",
			rdc: &models.RuntimeDeployConfig{
				Routes:       map[string]*models.Route{"GET|/pets|h": {CanonicalChainKey: "shared-chain"}},
				PolicyChains: chains("GET|/pets|h", "shared-chain"),
			},
			wantErr: `is neither the route key nor a composed operation key`,
		},
		{
			// The concrete shape of that mistake: a public route carrying another route's
			// key, which would silently borrow that route's authentication.
			name: "identity route pointed at another route's chain",
			rdc: &models.RuntimeDeployConfig{
				Routes: map[string]*models.Route{
					"GET|/pets|h":  {CanonicalChainKey: "GET|/admin|h"},
					"GET|/admin|h": {},
				},
				PolicyChains: chains("GET|/pets|h", "GET|/admin|h"),
			},
			wantErr: `canonical chain key "GET|/admin|h" is neither the route key nor a composed operation key`,
		},
		{
			// A directly-resolved route deliberately pointed at a composed operation key:
			// resolution still comes from the route, but the chain it names is an
			// operation's rather than its own route key. Accepted because the key is well
			// formed and belongs to this API and this route's vhost — the checks the two
			// rejection cases above exercise.
			name: "directly-resolved route pointed at a composed operation key",
			rdc: &models.RuntimeDeployConfig{
				Metadata: models.Metadata{UUID: "api-1"},
				Routes: map[string]*models.Route{"POST|/v1/op-one|h": {
					Vhost:             "h",
					CanonicalChainKey: chainkey.For("api-1", "h", "OperationOne"),
				}},
				PolicyChains: chains(chainkey.For("api-1", "h", "OperationOne")),
			},
		},
		{
			name: "resolver-bearing route with operation chains in its partition",
			rdc: &models.RuntimeDeployConfig{
				Metadata: models.Metadata{UUID: "api-1"},
				Routes: map[string]*models.Route{"POST|/rpc|h": {
					Vhost:        "h",
					ResolverName: "fake-multiplexed",
				}},
				PolicyChains: chains(
					chainkey.For("api-1", "h", "OperationOne"),
					chainkey.For("api-1", "h", "GetTask"),
				),
			},
		},
		{
			// The failure this validation exists for: the two xDS streams are
			// independent, so a route that can never reach a chain would otherwise
			// surface only at request time.
			name: "resolver-bearing route with no operation chains at all",
			rdc: &models.RuntimeDeployConfig{
				Metadata: models.Metadata{UUID: "api-1"},
				Routes: map[string]*models.Route{"POST|/rpc|h": {
					Vhost:        "h",
					ResolverName: "fake-multiplexed",
				}},
				PolicyChains: chains("GET|/pets|h"),
			},
			wantErr: `has no operation chains in its routing partition (vhost "h")`,
		},
		{
			// Chains exist, but for a different partition — every request to this route
			// would compose a key from its own vhost and find nothing.
			name: "operation chains in the wrong routing partition",
			rdc: &models.RuntimeDeployConfig{
				Metadata: models.Metadata{UUID: "api-1"},
				Routes: map[string]*models.Route{"POST|/rpc|sandbox": {
					Vhost:        "sandbox",
					ResolverName: "fake-multiplexed",
				}},
				PolicyChains: chains(chainkey.For("api-1", "main", "OperationOne")),
			},
			wantErr: `no operation chains in its routing partition (vhost "sandbox")`,
		},
		{
			// A resolver-bearing route composes its key per request, so a canonical key
			// on it would be read by nothing.
			name: "resolver-bearing route carrying a canonical chain key",
			rdc: &models.RuntimeDeployConfig{
				Metadata: models.Metadata{UUID: "api-1"},
				Routes: map[string]*models.Route{"POST|/rpc|h": {
					Vhost:             "h",
					ResolverName:      "fake-multiplexed",
					CanonicalChainKey: chainkey.For("api-1", "h", "OperationOne"),
				}},
				PolicyChains: chains(chainkey.For("api-1", "h", "OperationOne")),
			},
			wantErr: "must not carry a canonical chain key",
		},
		{
			// Catches a transformer that composed with the wrong field order or dropped
			// a component: the engine would compose three parts and never match.
			name: "malformed composed chain key",
			rdc: &models.RuntimeDeployConfig{
				Metadata:     models.Metadata{UUID: "api-1"},
				Routes:       map[string]*models.Route{"GET|/pets|h": {CanonicalChainKey: "api-1\x1fmessage/send"}},
				PolicyChains: chains("api-1\x1fmessage/send"),
			},
			wantErr: "is not a well-formed composed key",
		},
		{
			name: "composed chain key belonging to another API",
			rdc: &models.RuntimeDeployConfig{
				Metadata: models.Metadata{UUID: "api-1"},
				Routes: map[string]*models.Route{"POST|/rpc|h": {
					Vhost:        "h",
					ResolverName: "fake-multiplexed",
				}},
				PolicyChains: chains(chainkey.For("api-2", "h", "OperationOne")),
			},
			wantErr: `is composed for API "api-2", not this API ("api-1")`,
		},
		{
			// The default vhost is the empty string, and that is a legitimate partition
			// rather than a malformed key.
			name: "default vhost composes a valid key",
			rdc: &models.RuntimeDeployConfig{
				Metadata: models.Metadata{UUID: "api-1"},
				Routes: map[string]*models.Route{"POST|/rpc|": {
					ResolverName: "fake-multiplexed",
				}},
				PolicyChains: chains(chainkey.For("api-1", "", "OperationOne")),
			},
		},
		{
			// Existence is not enough: this chain is real, well-formed, and belongs to
			// this API — it just belongs to a different routing partition. A production
			// route pointed at a sandbox operation chain would run the sandbox's
			// authentication, authorization and rate limits.
			name: "identity route redirected into another partition's chain",
			rdc: &models.RuntimeDeployConfig{
				Metadata: models.Metadata{UUID: "api-1"},
				Routes: map[string]*models.Route{
					"POST|/v1/op-one|prod": {
						Vhost:             "prod",
						CanonicalChainKey: chainkey.For("api-1", "sandbox", "OperationOne"),
					},
				},
				PolicyChains: chains(chainkey.For("api-1", "sandbox", "OperationOne")),
			},
			wantErr: `belongs to routing partition (vhost) "sandbox", but the route serves "prod"`,
		},
		{
			// Same shape, across APIs. The chains pass would reject this chain on its own,
			// but the route-level check must name the route, since that is what is wrong.
			name: "identity route redirected into another API's chain",
			rdc: &models.RuntimeDeployConfig{
				Metadata: models.Metadata{UUID: "api-1"},
				Routes: map[string]*models.Route{
					"POST|/v1/op-one|h": {
						Vhost:             "h",
						CanonicalChainKey: chainkey.For("api-2", "h", "OperationOne"),
					},
				},
				PolicyChains: chains(chainkey.For("api-2", "h", "OperationOne")),
			},
			wantErr: `is composed for API "api-2"`,
		},
		{
			// The default vhost is the empty string on both sides, so it must match rather
			// than be treated as "unset, therefore anything goes".
			name: "default-vhost route matches a default-vhost chain",
			rdc: &models.RuntimeDeployConfig{
				Metadata: models.Metadata{UUID: "api-1"},
				Routes: map[string]*models.Route{
					"POST|/v1/op-one|": {CanonicalChainKey: chainkey.For("api-1", "", "OperationOne")},
				},
				PolicyChains: chains(chainkey.For("api-1", "", "OperationOne")),
			},
		},
		{
			name: "default-vhost route must not reach a named-vhost chain",
			rdc: &models.RuntimeDeployConfig{
				Metadata: models.Metadata{UUID: "api-1"},
				Routes: map[string]*models.Route{
					"POST|/v1/op-one|": {CanonicalChainKey: chainkey.For("api-1", "prod", "OperationOne")},
				},
				PolicyChains: chains(chainkey.For("api-1", "prod", "OperationOne")),
			},
			wantErr: `but the route serves ""`,
		},
		{
			// A present key with a nil value passes every reachability check — the key is
			// there — and then panics the snapshot translator, which dereferences it to
			// read Policies. Deploy time is where it has to be named.
			name: "nil policy chain value",
			rdc: &models.RuntimeDeployConfig{
				Routes:       map[string]*models.Route{"GET|/pets|h": {}},
				PolicyChains: map[string]*models.PolicyChain{"GET|/pets|h": nil},
			},
			wantErr: `policy chain "GET|/pets|h" is nil`,
		},
		{
			// The contrast that keeps the check honest: a chain with no policies is
			// legitimate and common (an operation whose policies are all inherited).
			name: "empty policy chain is valid",
			rdc: &models.RuntimeDeployConfig{
				Routes:       map[string]*models.Route{"GET|/pets|h": {}},
				PolicyChains: map[string]*models.PolicyChain{"GET|/pets|h": {}},
			},
		},
		{
			name: "nil route",
			rdc: &models.RuntimeDeployConfig{
				Routes:       map[string]*models.Route{"GET|/pets|h": nil},
				PolicyChains: chains("GET|/pets|h"),
			},
			wantErr: "nil route",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rdc.ValidateResolution()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// An RDC that fails validation must not reach the store, so neither xDS stream can
// ever publish it.
func TestAddRuntimeConfigRejectsUnresolvableReferences(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sm := NewSnapshotManager(logger)
	pm := NewPolicyManager(sm, logger)

	store := newTestRuntimeStore()
	sm.SetRuntimeStore(store)
	pm.SetRuntimeStore(store)

	bad := &models.RuntimeDeployConfig{
		Metadata: models.Metadata{UUID: "agent-1", Kind: "Agent", Handle: "assistant"},
		Routes: map[string]*models.Route{"POST|/rpc|h": {
			Vhost:        "h",
			ResolverName: "fake-multiplexed",
		}},
		// No operation chains were built, so no request to this route could resolve.
		PolicyChains: map[string]*models.PolicyChain{},
	}

	err := pm.AddRuntimeConfig("Agent:assistant", bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid policy chain resolution")

	_, exists := store.Get("Agent:assistant")
	assert.False(t, exists, "a config that fails validation must never be stored")
}

// A well-formed RDC still goes through.
func TestAddRuntimeConfigAcceptsValidReferences(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sm := NewSnapshotManager(logger)
	pm := NewPolicyManager(sm, logger)

	store := newTestRuntimeStore()
	sm.SetRuntimeStore(store)
	pm.SetRuntimeStore(store)

	require.NoError(t, pm.AddRuntimeConfig("RestApi:petstore", restRDC()))
	_, exists := store.Get("RestApi:petstore")
	assert.True(t, exists)
}

// The translator runs over configs read back from the store, including ones written
// before ValidateResolution rejected a nil chain, so it must not depend on that check
// having run: a panic here would take down snapshot generation for every API at once,
// not just the malformed one.
func TestTranslateSkipsNilChainsAndRoutesWithoutPanicking(t *testing.T) {
	rdc := &models.RuntimeDeployConfig{
		Metadata:            models.Metadata{UUID: "api-1", Kind: "RestApi", Handle: "petstore"},
		PolicyChainResolver: models.RouteKeyResolverName,
		Routes: map[string]*models.Route{
			"GET|/pets|h": {Method: "GET", Path: "/pets", Vhost: "h"},
			"GET|/nil|h":  nil,
		},
		PolicyChains: map[string]*models.PolicyChain{
			"GET|/pets|h": {},
			"GET|/nil|h":  nil,
		},
		UpstreamClusters: map[string]*models.UpstreamCluster{
			"upstream_main": {BasePath: "/", Endpoints: []models.Endpoint{{Host: "localhost", Port: 8080}}},
		},
	}

	resources, err := testTranslator().TranslateRuntimeConfigs([]*models.RuntimeDeployConfig{rdc})
	require.NoError(t, err)

	// The well-formed siblings still publish — one bad entry must not cost the rest.
	assert.Contains(t, resources[PolicyChainTypeURL], "GET|/pets|h")
	assert.Contains(t, resources[RouteConfigTypeURL], "GET|/pets|h")
	assert.NotContains(t, resources[PolicyChainTypeURL], "GET|/nil|h")
	assert.NotContains(t, resources[RouteConfigTypeURL], "GET|/nil|h")
}

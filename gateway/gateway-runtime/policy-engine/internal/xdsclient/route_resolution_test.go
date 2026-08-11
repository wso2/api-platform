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

package xdsclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
)

// ─── Fakes ───────────────────────────────────────────────────────────────────

type stubResolver struct {
	name string
	reqs resolver.Requirements
}

func (s *stubResolver) Name() string                        { return s.name }
func (s *stubResolver) Requirements() resolver.Requirements { return s.reqs }
func (s *stubResolver) Identify(resolver.RequestView) (resolver.Resolution, error) {
	return resolver.Resolution{}, nil
}

type preparingStubResolver struct {
	stubResolver
	err          error
	calls        int
	seenConfigs  []string
	returnedName string
}

func (p *preparingStubResolver) Prepare(cfg json.RawMessage) (any, error) {
	p.calls++
	p.seenConfigs = append(p.seenConfigs, string(cfg))
	if p.err != nil {
		return nil, p.err
	}
	return p.returnedName, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newRouteHandler(t *testing.T, resolvers resolver.ResolverRegistry) (*ResourceHandler, *kernel.Kernel) {
	t.Helper()
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{Policies: make(map[string]*registry.PolicyEntry)}
	return NewResourceHandler(k, reg, resolvers), k
}

func registryWithResolvers(t *testing.T, resolvers ...resolver.OperationResolver) resolver.ResolverRegistry {
	t.Helper()
	reg := resolver.NewRegistry()
	for _, r := range resolvers {
		require.NoError(t, reg.Register(r))
	}
	reg.Freeze()
	return reg
}

// routeConfigResource wraps a route config map the way the control plane does:
// a Struct inside an Any inside the outer resource Any.
func routeConfigResource(t *testing.T, data map[string]interface{}) *anypb.Any {
	t.Helper()
	s, err := structpb.NewStruct(data)
	require.NoError(t, err)
	structBytes, err := proto.Marshal(s)
	require.NoError(t, err)
	innerAny := &anypb.Any{TypeUrl: "type.googleapis.com/google.protobuf.Struct", Value: structBytes}
	innerBytes, err := proto.Marshal(innerAny)
	require.NoError(t, err)
	return &anypb.Any{TypeUrl: RouteConfigTypeURL, Value: innerBytes}
}

// ─── Identity routes ─────────────────────────────────────────────────────────

func TestRouteConfigUpdate_IdentityRouteParsesCanonicalChainKey(t *testing.T) {
	h, k := newRouteHandler(t, resolver.DefaultRegistry())

	err := h.HandleRouteConfigUpdate(context.Background(), []*anypb.Any{
		routeConfigResource(t, map[string]interface{}{
			"route_key":           "GET|/pets|example.com",
			"resolver_name":       "route-key",
			"canonical_chain_key": "GET|/pets|example.com",
			"metadata":            map[string]interface{}{"display_name": "pets", "vhost": "example.com"},
		}),
	}, "v1")
	require.NoError(t, err)

	rc := k.GetRouteConfig("GET|/pets|example.com")
	require.NotNil(t, rc)
	assert.Equal(t, "GET|/pets|example.com", rc.RouteKey)
	assert.Equal(t, "GET|/pets|example.com", rc.CanonicalChainKey)
	assert.Equal(t, "route-key", rc.ResolverName)
	assert.True(t, rc.IsIdentity())
}

// An older controller omits canonical_chain_key entirely. For an identity route the
// route key is the chain key, which is exactly what the pre-resolution engine assumed.
func TestRouteConfigUpdate_AbsentCanonicalChainKeyFallsBackToRouteKey(t *testing.T) {
	h, k := newRouteHandler(t, resolver.DefaultRegistry())

	err := h.HandleRouteConfigUpdate(context.Background(), []*anypb.Any{
		routeConfigResource(t, map[string]interface{}{
			"route_key":     "GET|/pets|example.com",
			"resolver_name": "route-key",
		}),
	}, "v1")
	require.NoError(t, err)

	rc := k.GetRouteConfig("GET|/pets|example.com")
	require.NotNil(t, rc)
	assert.Equal(t, "GET|/pets|example.com", rc.CanonicalChainKey)
}

// An empty resolver_name is identity too — that is what every existing kind's routes
// looked like before this field was populated per route.
func TestRouteConfigUpdate_EmptyResolverNameIsIdentity(t *testing.T) {
	h, k := newRouteHandler(t, resolver.DefaultRegistry())

	err := h.HandleRouteConfigUpdate(context.Background(), []*anypb.Any{
		routeConfigResource(t, map[string]interface{}{"route_key": "GET|/pets|example.com"}),
	}, "v1")
	require.NoError(t, err)

	rc := k.GetRouteConfig("GET|/pets|example.com")
	require.NotNil(t, rc)
	assert.True(t, rc.IsIdentity())
}

// ─── Non-identity routes ─────────────────────────────────────────────────────

func TestRouteConfigUpdate_ParsesResolverConfigAndBufferLimit(t *testing.T) {
	prep := &preparingStubResolver{
		stubResolver: stubResolver{name: "fake-jsonrpc", reqs: resolver.Requirements{BufferBody: true}},
		returnedName: "compiled",
	}
	h, k := newRouteHandler(t, registryWithResolvers(t, prep))

	err := h.HandleRouteConfigUpdate(context.Background(), []*anypb.Any{
		routeConfigResource(t, map[string]interface{}{
			"route_key":              "POST|/rpc|example.com",
			"resolver_name":          "fake-jsonrpc",
			"max_request_body_bytes": 4096,
			"resolver_config":        map[string]interface{}{"protocolVersion": "1.0"},
		}),
	}, "v1")
	require.NoError(t, err)

	rc := k.GetRouteConfig("POST|/rpc|example.com")
	require.NotNil(t, rc)
	assert.False(t, rc.IsIdentity())
	assert.Equal(t, int64(4096), rc.MaxRequestBodyBytes)
	assert.JSONEq(t, `{"protocolVersion":"1.0"}`, string(rc.ResolverConfig))

	// Prepare runs once per route at ingest, so no request pays for it, and its
	// result is what the resolver later reads as RequestView.RouteState.
	assert.Equal(t, 1, prep.calls)
	assert.Equal(t, "compiled", rc.RouteState)
}

// A route naming a resolver this binary does not have is dropped. Keeping it would
// mean every request to it fails at runtime; resolving it by identity would apply the
// route-level chain to every operation the route multiplexes.
func TestRouteConfigUpdate_UnknownResolverSkipsTheRoute(t *testing.T) {
	h, k := newRouteHandler(t, resolver.DefaultRegistry())

	err := h.HandleRouteConfigUpdate(context.Background(), []*anypb.Any{
		routeConfigResource(t, map[string]interface{}{
			"route_key":     "POST|/rpc|example.com",
			"resolver_name": "a2a-jsonrpc",
		}),
	}, "v1")

	require.NoError(t, err, "a per-entry problem must never NACK the snapshot")
	assert.Nil(t, k.GetRouteConfig("POST|/rpc|example.com"))
}

// Under composed keys a resolver-bearing route carries no operation map, so there is
// nothing to validate at ingest and the route must be admitted on its own. Whether the
// chains it will compose keys for actually exist is a deploy-time question the
// controller answers (it is the only side that can enumerate them); at request time a
// key with no chain is a per-request failure, not a reason to drop the route.
func TestRouteConfigUpdate_ResolverBearingRouteNeedsNoOperationMap(t *testing.T) {
	h, k := newRouteHandler(t, registryWithResolvers(t, &stubResolver{name: "fake-jsonrpc"}))

	require.NoError(t, h.HandleRouteConfigUpdate(context.Background(),
		[]*anypb.Any{routeConfigResource(t, map[string]interface{}{
			"route_key":     "POST|/rpc|example.com",
			"resolver_name": "fake-jsonrpc",
		})}, "v1"))

	rc := k.GetRouteConfig("POST|/rpc|example.com")
	require.NotNil(t, rc, "a resolver-bearing route is complete without an operation map")
	assert.False(t, rc.IsIdentity())
}

// An operation_map from an older controller is ignored rather than rejected: the field
// is gone from the contract, and a route that is otherwise valid must not be dropped
// over a value nothing reads.
func TestRouteConfigUpdate_StaleOperationMapIsIgnored(t *testing.T) {
	h, k := newRouteHandler(t, registryWithResolvers(t, &stubResolver{name: "fake-jsonrpc"}))

	require.NoError(t, h.HandleRouteConfigUpdate(context.Background(),
		[]*anypb.Any{routeConfigResource(t, map[string]interface{}{
			"route_key":     "POST|/rpc|example.com",
			"resolver_name": "fake-jsonrpc",
			"operation_map": map[string]interface{}{"SendMessage": "chain-send"},
		})}, "v1"))

	assert.NotNil(t, k.GetRouteConfig("POST|/rpc|example.com"))
}

// A Prepare error must drop only its own route. Under State-of-the-World a NACK keeps
// the previous version of every RouteConfig, so failing the whole update would freeze
// route updates for every API on the gateway.
func TestRouteConfigUpdate_PrepareErrorSkipsOnlyThatRoute(t *testing.T) {
	failing := &preparingStubResolver{
		stubResolver: stubResolver{name: "failing"},
		err:          errors.New("schema does not compile"),
	}
	ok := &preparingStubResolver{
		stubResolver: stubResolver{name: "working"},
		returnedName: "state",
	}
	h, k := newRouteHandler(t, registryWithResolvers(t, failing, ok))

	err := h.HandleRouteConfigUpdate(context.Background(), []*anypb.Any{
		routeConfigResource(t, map[string]interface{}{
			"route_key":     "POST|/bad|example.com",
			"resolver_name": "failing",
		}),
		routeConfigResource(t, map[string]interface{}{
			"route_key":     "POST|/good|example.com",
			"resolver_name": "working",
		}),
		routeConfigResource(t, map[string]interface{}{
			"route_key": "GET|/pets|example.com",
		}),
	}, "v1")

	require.NoError(t, err, "a failing Prepare must not NACK the snapshot")
	assert.Nil(t, k.GetRouteConfig("POST|/bad|example.com"), "the failing route is dropped")
	assert.NotNil(t, k.GetRouteConfig("POST|/good|example.com"), "a sibling route in the same update still applies")
	assert.NotNil(t, k.GetRouteConfig("GET|/pets|example.com"), "an identity route is unaffected")
}

// A nil resolver registry must be treated as identity-only rather than panicking or
// admitting a route it cannot serve.
func TestRouteConfigUpdate_NilResolverRegistryIsIdentityOnly(t *testing.T) {
	h, k := newRouteHandler(t, nil)

	err := h.HandleRouteConfigUpdate(context.Background(), []*anypb.Any{
		routeConfigResource(t, map[string]interface{}{"route_key": "GET|/pets|example.com"}),
		routeConfigResource(t, map[string]interface{}{
			"route_key":     "POST|/rpc|example.com",
			"resolver_name": "a2a-jsonrpc",
		}),
	}, "v1")

	require.NoError(t, err)
	assert.NotNil(t, k.GetRouteConfig("GET|/pets|example.com"))
	assert.Nil(t, k.GetRouteConfig("POST|/rpc|example.com"))
}

// A resolver that does not implement Preparer gets no per-route state, and the route
// is admitted regardless.
func TestRouteConfigUpdate_ResolverWithoutPreparer(t *testing.T) {
	h, k := newRouteHandler(t, registryWithResolvers(t, &stubResolver{name: "no-prepare"}))

	require.NoError(t, h.HandleRouteConfigUpdate(context.Background(), []*anypb.Any{
		routeConfigResource(t, map[string]interface{}{
			"route_key":       "POST|/rpc|example.com",
			"resolver_name":   "no-prepare",
			"resolver_config": map[string]interface{}{"ignored": true},
		}),
	}, "v1"))

	rc := k.GetRouteConfig("POST|/rpc|example.com")
	require.NotNil(t, rc)
	assert.Nil(t, rc.RouteState)
}

// ─── Numeric field parsing ───────────────────────────────────────────────────

// protojson renders every Struct number as a float64, and a producer may also send it
// as a string; a nonsensical value must read as "not configured" so the route falls
// back to the low default rather than being left unbounded.
func TestGetInt64FromMap(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want int64
	}{
		{"float", float64(4096), 4096},
		{"string", "8192", 8192},
		{"zero", float64(0), 0},
		{"negative", float64(-1), 0},
		{"negative string", "-1", 0},
		{"unparseable string", "many", 0},
		{"wrong type", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, getInt64FromMap(map[string]interface{}{"k": tt.in}, "k"))
		})
	}
	assert.Equal(t, int64(0), getInt64FromMap(map[string]interface{}{}, "missing"))
}

// ─── Capability advertisement ────────────────────────────────────────────────

// The control plane needs to know what this runtime can resolve before it sends a
// route that needs it, so every discovery request carries the advertisement.
func TestDiscoveryNode_AdvertisesResolverCapabilities(t *testing.T) {
	c := &Client{resolvers: registryWithResolvers(t,
		&stubResolver{name: "zeta"}, &stubResolver{name: "alpha"})}

	node := c.discoveryNode()
	require.NotNil(t, node.Metadata)

	version := node.Metadata.Fields["resolution_protocol_version"].GetNumberValue()
	assert.Equal(t, float64(resolver.ProtocolVersion), version)

	list := node.Metadata.Fields["supported_resolvers"].GetListValue()
	require.NotNil(t, list)
	var names []string
	for _, v := range list.Values {
		names = append(names, v.GetStringValue())
	}
	assert.Equal(t, []string{"alpha", "zeta"}, names,
		"the advertised list is sorted so the control plane can compare it cheaply")
}

// A client with no registry must still advertise, with an empty list — the control
// plane then withholds every resolver-bearing route rather than guessing.
func TestDiscoveryNode_NoRegistryAdvertisesEmptyList(t *testing.T) {
	c := &Client{}
	node := c.discoveryNode()

	require.NotNil(t, node.Metadata)
	assert.Empty(t, node.Metadata.Fields["supported_resolvers"].GetListValue().Values)
}

// A malformed byte ceiling must read as "not configured" so the route falls back to the
// engine's own low default, rather than being truncated to a nearby value or — worse —
// saturating. int64(float64(1<<63)) yields math.MaxInt64 on this platform, which would turn
// a nonsense value into an effectively unbounded ceiling on a limit whose whole job is
// bounding unauthenticated work.
func TestRouteConfigUpdate_MalformedBodyLimitReadsAsUnconfigured(t *testing.T) {
	for name, limit := range map[string]interface{}{
		"fractional below one": 0.5,
		"fractional":           4096.5,
		"above int64 range":    float64(1 << 63),
		"negative":             -4096.0,
		"zero":                 0.0,
		"not a number":         "not-a-number",
	} {
		t.Run(name, func(t *testing.T) {
			h, k := newRouteHandler(t, registryWithResolvers(t, &stubResolver{name: "fake-jsonrpc"}))

			require.NoError(t, h.HandleRouteConfigUpdate(context.Background(),
				[]*anypb.Any{routeConfigResource(t, map[string]interface{}{
					"route_key":              "POST|/rpc|example.com",
					"resolver_name":          "fake-jsonrpc",
					"max_request_body_bytes": limit,
				})}, "v1"))

			rc := k.GetRouteConfig("POST|/rpc|example.com")
			require.NotNil(t, rc, "a malformed limit must not drop the route")
			assert.Zero(t, rc.MaxRequestBodyBytes, "malformed limits read as not configured")
			assert.Equal(t, kernel.DefaultMaxResolverRequestBodyBytes, rc.EffectiveMaxRequestBodyBytes(),
				"and the route falls back to the engine default")
		})
	}
}

// The whole-number values a producer legitimately sends still arrive intact, including one
// emitted as a string.
func TestRouteConfigUpdate_ValidBodyLimitsAreAccepted(t *testing.T) {
	for name, tc := range map[string]struct {
		limit interface{}
		want  int64
	}{
		"float":         {limit: 4096.0, want: 4096},
		"string":        {limit: "4096", want: 4096},
		"one byte":      {limit: 1.0, want: 1},
		"largest int64": {limit: "9223372036854775807", want: 9223372036854775807},
	} {
		t.Run(name, func(t *testing.T) {
			h, k := newRouteHandler(t, registryWithResolvers(t, &stubResolver{name: "fake-jsonrpc"}))

			require.NoError(t, h.HandleRouteConfigUpdate(context.Background(),
				[]*anypb.Any{routeConfigResource(t, map[string]interface{}{
					"route_key":              "POST|/rpc|example.com",
					"resolver_name":          "fake-jsonrpc",
					"max_request_body_bytes": tc.limit,
				})}, "v1"))

			rc := k.GetRouteConfig("POST|/rpc|example.com")
			require.NotNil(t, rc)
			assert.Equal(t, tc.want, rc.MaxRequestBodyBytes)
		})
	}
}

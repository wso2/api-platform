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

package resolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/agentproto"
)

// ─── Fake resolvers ──────────────────────────────────────────────────────────
//
// These exercise the resolution seam itself — preparation, key validation,
// binding — independently of any real protocol. They are registered only into
// test-local registries, so nothing below is reachable in production.

// fakeResolver is a factory whose prepared resolvers report whatever the test
// configured, per route. prepare is optional; without it the factory prepares a
// resolver that returns `resolution`/`err` for every request.
type fakeResolver struct {
	name       string
	reqs       RequestRequirements
	resolution Resolution
	err        error

	// prepare, when set, overrides the default and receives the route's config, so a
	// test can prove two routes prepared by one factory are independent.
	prepare func(ResolverRouteConfig) (PreparedResolver, error)

	prepareCalls int
	lastConfig   ResolverRouteConfig
}

func (f *fakeResolver) Name() string { return f.name }

func (f *fakeResolver) Prepare(cfg ResolverRouteConfig) (PreparedResolver, error) {
	f.prepareCalls++
	f.lastConfig = cfg
	if f.prepare != nil {
		return f.prepare(cfg)
	}
	return &fakePrepared{reqs: f.reqs, resolution: f.resolution, err: f.err}, nil
}

// fakePrepared is one prepared route.
type fakePrepared struct {
	reqs       RequestRequirements
	resolution Resolution
	err        error
	calls      int
	lastView   RequestView
}

func (f *fakePrepared) Requirements() RequestRequirements { return f.reqs }

func (f *fakePrepared) Resolve(_ context.Context, view RequestView) (Resolution, error) {
	f.calls++
	f.lastView = view
	return f.resolution, f.err
}

// fakeStatic is a prepared resolver whose answer is fixed at ingest.
type fakeStatic struct {
	fakePrepared
	static Resolution
}

func (f *fakeStatic) StaticResolution() Resolution { return f.static }

func registryWith(t *testing.T, resolvers ...Resolver) *Registry {
	t.Helper()
	reg := NewRegistry()
	for _, r := range resolvers {
		require.NoError(t, reg.Register(r))
	}
	reg.Freeze()
	return reg
}

// prepareWith prepares one route through reg, failing the test if it cannot.
func prepareWith(t *testing.T, reg ResolverRegistry, cfg ResolverRouteConfig) *PreparedRoute {
	t.Helper()
	pr, err := PrepareRoute(reg, cfg)
	require.NoError(t, err)
	return pr
}

// fakeChain stands in for the kernel's policy chain: the binder is generic over the
// chain type and only ever checks whether the accessor produced one.
type fakeChain struct{ key string }

// chainsPresent builds the chain accessor Bind selects against, recording every key it
// was asked for so a test can assert how many lookups a binding actually cost.
type chainStore struct {
	present  map[string]struct{}
	lookedUp []string
}

func chainsPresent(keys ...string) *chainStore {
	s := &chainStore{present: make(map[string]struct{}, len(keys))}
	for _, k := range keys {
		s.present[k] = struct{}{}
	}
	return s
}

func (s *chainStore) get(key string) *fakeChain {
	s.lookedUp = append(s.lookedUp, key)
	if _, ok := s.present[key]; !ok {
		return nil
	}
	return &fakeChain{key: key}
}

// noChains is the accessor for a route whose partition has no chains at all.
func noChains(string) *fakeChain { return nil }

// ─── Identity resolver ───────────────────────────────────────────────────────

func TestRouteKeyResolver_PreparesAStaticDirectResolution(t *testing.T) {
	r := &RouteKeyResolver{}
	assert.Equal(t, "route-key", r.Name())

	prepared, err := r.Prepare(ResolverRouteConfig{
		RouteKey:          "GET|/api/v1/users|example.com",
		CanonicalChainKey: "GET|/api/v1/users|example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, RequestRequirements{Body: BodyNotRequired}, prepared.Requirements())
	assert.False(t, prepared.Requirements().BuffersBody(), "an identity route must never buffer a body")

	static, ok := prepared.(StaticPreparedResolver)
	require.True(t, ok, "route-key must be static so the request path never calls Resolve")
	res := static.StaticResolution()
	assert.Equal(t, "GET|/api/v1/users|example.com", res.ChainKey)
}

// The canonical key is read from the config, never rebuilt from the route key: that is
// the seam that keeps a later move to a separate key namespace a controller-only change.
func TestRouteKeyResolver_UsesTheCanonicalKeyNotTheRouteKey(t *testing.T) {
	prepared, err := (&RouteKeyResolver{}).Prepare(ResolverRouteConfig{
		RouteKey:          "POST|/op-one|example.com",
		CanonicalChainKey: "operation-chain-key",
	})
	require.NoError(t, err)
	assert.Equal(t, "operation-chain-key",
		prepared.(StaticPreparedResolver).StaticResolution().ChainKey)
}

// Ingest applies the older-controller fallback to the route key, exactly once. Applying
// it a second time here would create a second place for the two to disagree, so an
// empty effective key is a wiring fault and the route is refused.
func TestRouteKeyResolver_RefusesToReapplyTheFallback(t *testing.T) {
	_, err := (&RouteKeyResolver{}).Prepare(ResolverRouteConfig{RouteKey: "GET|/pets|h"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "effective chain key")
}

// ─── Registry ────────────────────────────────────────────────────────────────

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	r := &fakeResolver{name: "fake"}
	require.NoError(t, reg.Register(r))

	got, ok := reg.Get("fake")
	require.True(t, ok)
	assert.Same(t, r, got)

	_, ok = reg.Get("missing")
	assert.False(t, ok)
}

// A duplicate name means two resolvers answer to one wire value; that is always a
// build mistake, never something to resolve at runtime by picking one.
func TestRegistry_RejectsDuplicateRegistration(t *testing.T) {
	reg := NewRegistry()
	require.NoError(t, reg.Register(&fakeResolver{name: "fake"}))

	err := reg.Register(&fakeResolver{name: "fake"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_RejectsNilAndEmptyName(t *testing.T) {
	reg := NewRegistry()
	require.Error(t, reg.Register(nil))
	require.Error(t, reg.Register(&fakeResolver{name: ""}))
}

func TestRegistry_FreezeBlocksRegistration(t *testing.T) {
	reg := NewRegistry()
	require.NoError(t, reg.Register(&fakeResolver{name: "before"}))
	reg.Freeze()
	assert.True(t, reg.Frozen())

	require.Error(t, reg.Register(&fakeResolver{name: "after"}))

	_, ok := reg.Get("after")
	assert.False(t, ok)
}

func TestRegistry_NamesAreSorted(t *testing.T) {
	reg := registryWith(t, &fakeResolver{name: "zeta"}, &fakeResolver{name: "alpha"}, &fakeResolver{name: "mid"})
	assert.Equal(t, []string{"alpha", "mid", "zeta"}, reg.Names())
}

// The production registry's contents are the capability list the control plane
// withholds resolver-bearing routes against, so what is in it is a wire fact, not an
// implementation detail: asserting the exact set is what makes adding or renaming a
// resolver a deliberate change rather than a silent one.
func TestDefaultRegistry_HoldsTheShippedResolversAndIsFrozen(t *testing.T) {
	def := DefaultRegistry()
	assert.Equal(t, []string{agentproto.ResolverName, RouteKeyResolverName}, def.Names())

	for _, name := range def.Names() {
		r, ok := def.Get(name)
		require.True(t, ok)
		assert.Equal(t, name, r.Name())
	}

	assert.True(t, defaultRegistry.Frozen(), "DefaultRegistry must freeze the production registry")
}

// A test registry must not be able to leak a resolver into the production one.
func TestIndependentRegistryDoesNotAffectDefault(t *testing.T) {
	_ = registryWith(t, &fakeResolver{name: "test-only"})

	_, ok := DefaultRegistry().Get("test-only")
	assert.False(t, ok, "a resolver registered in a test registry must not appear in the production registry")
	assert.NotContains(t, DefaultRegistry().Names(), "test-only")
}

// ─── PrepareRoute ────────────────────────────────────────────────────────────

// An empty resolver_name is identity, normalised once here so nothing downstream has
// to know that "" and "route-key" mean the same thing.
func TestPrepareRoute_NormalizesEmptyResolverName(t *testing.T) {
	for _, name := range []string{"", RouteKeyResolverName} {
		t.Run(fmt.Sprintf("resolver_name=%q", name), func(t *testing.T) {
			pr := prepareWith(t, DefaultRegistry(), ResolverRouteConfig{
				RouteKey:          "GET|/pets|example.com",
				CanonicalChainKey: "GET|/pets|example.com",
				ResolverName:      name,
			})
			assert.Equal(t, RouteKeyResolverName, pr.ResolverName)
			assert.True(t, pr.IsStatic())
		})
	}
}

func TestPrepareRoute_CapturesThePartitionAndConfig(t *testing.T) {
	fake := &fakeResolver{name: "fake"}
	cfg := ResolverRouteConfig{
		RouteKey:          "POST|/rpc|api.example.com",
		CanonicalChainKey: "POST|/rpc|api.example.com",
		ResolverName:      "fake",
		APIID:             "api-1",
		Vhost:             "api.example.com",
		APIContext:        "/agent/v1",
		Method:            "POST",
		Path:              "/rpc",
		ResolverConfig:    json.RawMessage(`{"transport":"jsonrpc"}`),
	}

	pr := prepareWith(t, registryWith(t, fake), cfg)
	assert.Equal(t, 1, fake.prepareCalls, "Prepare runs once per route, at ingest")
	assert.Equal(t, cfg, fake.lastConfig, "the whole route config reaches Prepare unchanged")
	assert.Equal(t, "api-1", pr.APIID)
	assert.Equal(t, "api.example.com", pr.Vhost)
	assert.Equal(t, "POST|/rpc|api.example.com", pr.DirectChainKey)
}

func TestPrepareRoute_UnknownResolverIsClassified(t *testing.T) {
	_, err := PrepareRoute(registryWith(t, &fakeResolver{name: "known"}), ResolverRouteConfig{
		RouteKey:     "r",
		ResolverName: "not-registered",
	})
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureUnknownResolver, re.Kind)
}

// A nil registry is identity-only: a partially-wired server keeps serving every kind
// that resolves by route key rather than dropping every route on the gateway, but no
// protocol resolver is ever substituted for another.
func TestPrepareRoute_NilRegistryIsIdentityOnly(t *testing.T) {
	pr, err := PrepareRoute(nil, ResolverRouteConfig{
		RouteKey: "GET|/pets|h", CanonicalChainKey: "GET|/pets|h",
	})
	require.NoError(t, err)
	assert.True(t, pr.IsStatic())

	_, err = PrepareRoute(nil, ResolverRouteConfig{RouteKey: "r", ResolverName: "fake-protocol"})
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureUnknownResolver, re.Kind)
}

func TestPrepareRoute_ResolverErrorIsReturned(t *testing.T) {
	fake := &fakeResolver{name: "fake", prepare: func(ResolverRouteConfig) (PreparedResolver, error) {
		return nil, errors.New("bad schema")
	}}
	_, err := PrepareRoute(registryWith(t, fake), ResolverRouteConfig{ResolverName: "fake"})
	require.Error(t, err)
	assert.EqualError(t, err, "bad schema")

	var re *ResolutionError
	assert.False(t, errors.As(err, &re),
		"a resolver's own failure must not be mistaken for an unknown resolver")
}

// A factory returning (nil, nil) would otherwise store a route whose every request
// dereferences nil on the hot path.
func TestPrepareRoute_RejectsANilPreparedResolver(t *testing.T) {
	fake := &fakeResolver{name: "fake", prepare: func(ResolverRouteConfig) (PreparedResolver, error) {
		return nil, nil
	}}
	_, err := PrepareRoute(registryWith(t, fake), ResolverRouteConfig{ResolverName: "fake", RouteKey: "r"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil resolver")
}

// An unrecognised body requirement is refused rather than guessed at. Guessing the
// lenient way — "not required" — would let a resolver that asked for the body run without
// one and select a chain from nothing.
func TestPrepareRoute_RejectsAnUnknownBodyRequirement(t *testing.T) {
	factory := &fakeResolver{name: "future", prepare: func(ResolverRouteConfig) (PreparedResolver, error) {
		return &fakePrepared{reqs: RequestRequirements{Body: BodyRequirement(7)}}, nil
	}}

	_, err := PrepareRoute(registryWith(t, factory), ResolverRouteConfig{
		ResolverName: "future", RouteKey: "POST|/rpc|h", CanonicalChainKey: "POST|/rpc|h",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognised body requirement")
	assert.Contains(t, err.Error(), "unknown(7)")
}

// Whatever the requirement, an unrecognised one must never read as "needs nothing": the
// helper the request path uses is conservative, so even if such a value reached it the
// body would be provided rather than withheld.
func TestBuffersBody_TreatsAnUnknownRequirementAsNeedingTheBody(t *testing.T) {
	assert.False(t, RequestRequirements{}.BuffersBody(), "the zero value needs no body")
	assert.False(t, RequestRequirements{Body: BodyNotRequired}.BuffersBody())
	assert.True(t, RequestRequirements{Body: BodyBuffered}.BuffersBody())
	assert.True(t, RequestRequirements{Body: BodyRequirement(7)}.BuffersBody(),
		"an unknown requirement must not silently mean the body can be withheld")
}

// A resolver cannot be static and also need the request. The static branch is taken
// before the body-buffering check, so the declared requirement would be skipped silently
// — the request would resolve from a stored answer while the resolver believed it was
// being handed a body.
func TestPrepareRoute_RejectsAStaticResolverThatNeedsTheRequest(t *testing.T) {
	tests := []struct {
		name string
		reqs RequestRequirements
	}{
		{"buffered body", RequestRequirements{Body: BodyBuffered}},
		{"headers", RequestRequirements{Headers: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The key is a well-formed operation key for this route's own partition, so
			// nothing but the requirements can be what fails — the assertion below cannot
			// be satisfied by the key rule instead.
			factory := &fakeResolver{name: "contradictory", prepare: func(ResolverRouteConfig) (PreparedResolver, error) {
				return &fakeStatic{
					fakePrepared: fakePrepared{reqs: tt.reqs},
					static:       Resolution{ChainKey: ChainKeyFor("api-1", "h", "OperationOne")},
				}, nil
			}}

			_, err := PrepareRoute(registryWith(t, factory), ResolverRouteConfig{
				ResolverName: "contradictory", RouteKey: "POST|/rpc|h", CanonicalChainKey: "POST|/rpc|h",
				APIID: "api-1", Vhost: "h",
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "static resolution needs nothing from the request")
		})
	}
}

// route-key is the shape a static resolver must have: it declares nothing, so nothing it
// declared can be skipped.
func TestRouteKeyResolver_DeclaresNoRequestRequirements(t *testing.T) {
	pr := prepareWith(t, DefaultRegistry(), ResolverRouteConfig{
		RouteKey: "GET|/pets|h", CanonicalChainKey: "GET|/pets|h",
	})
	require.True(t, pr.IsStatic())
	assert.Equal(t, RequestRequirements{}, pr.Requirements)
}

// The point of preparing per route: one factory, two routes, different requirements.
// This is what lets one resolver read the body on a route that multiplexes operations
// while needing nothing at all on a route whose operation is fixed at deploy time.
func TestPrepareRoute_RequirementsArePerRouteNotPerFactory(t *testing.T) {
	factory := &fakeResolver{name: "fake-protocol", prepare: func(cfg ResolverRouteConfig) (PreparedResolver, error) {
		if cfg.Path == "/rpc" {
			return &fakePrepared{reqs: RequestRequirements{Body: BodyBuffered}}, nil
		}
		return &fakeStatic{
			static: Resolution{ChainKey: ChainKeyFor("api-1", "h", "OperationOne")},
		}, nil
	}}
	reg := registryWith(t, factory)

	multiplexed := prepareWith(t, reg, ResolverRouteConfig{ResolverName: "fake-protocol", Path: "/rpc", APIID: "api-1", Vhost: "h"})
	perOperation := prepareWith(t, reg, ResolverRouteConfig{ResolverName: "fake-protocol", Path: "/op-one", APIID: "api-1", Vhost: "h"})

	assert.True(t, multiplexed.Requirements.BuffersBody(),
		"a route carrying many operations can only know which one from the body")
	assert.False(t, multiplexed.IsStatic())

	assert.False(t, perOperation.Requirements.BuffersBody(),
		"a route dedicated to one operation knows it at deploy time")
	assert.True(t, perOperation.IsStatic(), "and therefore never runs on the request path")
}

// ─── Bind: directly-resolved routes ──────────────────────────────────────────
//
// Which rule the binder applies is decided by the route's normalised resolver name, fixed
// at preparation — not by anything the resolution carries. A request-time result cannot
// pick its own validation rule, which is what these two sections exercise from either side.

func TestBind_RouteKeySelectsTheRoutesOwnChain(t *testing.T) {
	pr := prepareWith(t, DefaultRegistry(), ResolverRouteConfig{
		RouteKey:          "GET|/pets|h",
		CanonicalChainKey: "GET|/pets|h",
	})

	store := chainsPresent("GET|/pets|h")
	bound, chain, err := BindStatic(pr, store.get)
	require.NoError(t, err)
	assert.Equal(t, "GET|/pets|h", bound.ChainKey)

	// The binding returns the chain it selected rather than reporting that one exists,
	// so this is the only lookup the request needs — and no eviction can slip between a
	// probe and a read.
	require.NotNil(t, chain)
	assert.Equal(t, "GET|/pets|h", chain.key)
	assert.Equal(t, []string{"GET|/pets|h"}, store.lookedUp,
		"the static fast path must cost exactly one chain lookup")
}

// The static fast path does no structural work per request: PrepareRoute validated the
// resolution once, so binding is a lookup and a struct copy. A resolution that would not
// validate never reaches this point — the route is refused at preparation.
func TestBindStatic_DoesNoPerRequestValidation(t *testing.T) {
	// A resolver whose static resolution names a chain outside its own route.
	factory := &fakeResolver{name: "bad-static", prepare: func(ResolverRouteConfig) (PreparedResolver, error) {
		return &fakeStatic{static: Resolution{ChainKey: "GET|/admin|h"}}, nil
	}}

	_, err := PrepareRoute(registryWith(t, factory), ResolverRouteConfig{
		ResolverName:      "bad-static",
		RouteKey:          "GET|/pets|h",
		CanonicalChainKey: "GET|/pets|h",
	})
	require.Error(t, err, "an invalid static resolution must be caught at ingest, not per request")
	assert.Contains(t, err.Error(), "invalid static resolution")
	// Named resolver, so the protocol rule applies: the key must be a composed operation
	// key, and a bare route key is not one.
	assert.Contains(t, err.Error(), "malformed operation chain key")

	// It must not be mistaken for an unknown resolver: the resolver was found and it is
	// the resolution it produced that is wrong, which ingest reports and counts separately.
	var re *ResolutionError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, FailureInternal, re.Kind)
	assert.NotEqual(t, FailureUnknownResolver, re.Kind)
}

// A direct route with no chain keeps the pre-resolution outcome: the kernel's own
// sterile 500, not a resolution failure the client could read anything into.
func TestBind_RouteKeyWithNoChainIsNotAResolutionFailure(t *testing.T) {
	pr := prepareWith(t, DefaultRegistry(), ResolverRouteConfig{
		RouteKey:          "GET|/pets|h",
		CanonicalChainKey: "GET|/pets|h",
	})

	_, _, err := BindStatic(pr, noChains)
	require.ErrorIs(t, err, ErrDirectRouteChainMissing)

	var re *ResolutionError
	assert.False(t, errors.As(err, &re),
		"this is the existing no-chain path, not a classified resolution failure")
}

// A resolver occupying the route-key name must return this route's own key and nothing
// else. Registered under that name rather than using the real RouteKeyResolver, because the
// real one cannot return a wrong key — the rule still has to be enforced against one that
// could.
func TestBind_RouteKeyRejectsAnyOtherKey(t *testing.T) {
	fake := &fakeResolver{
		name:       RouteKeyResolverName,
		resolution: Resolution{ChainKey: "GET|/admin|h"},
	}
	pr := prepareWith(t, registryWith(t, fake), ResolverRouteConfig{
		ResolverName:      RouteKeyResolverName,
		RouteKey:          "GET|/pets|h",
		CanonicalChainKey: "GET|/pets|h",
	})

	// The other route's chain exists, so only validation can stop the binding.
	_, chain, err := Bind(pr, fake.resolution, chainsPresent("GET|/admin|h").get)
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureInternal, re.Kind)
	assert.Contains(t, re.Cause.Error(), "route's own chain key")
	assert.Nil(t, chain, "the other route's chain must not have been bound")
}

// ─── Bind: protocol-resolved routes ──────────────────────────────────────────

// The mirror of the rule above: a protocol resolver may not return a bare route key and so
// slip into directly-resolved semantics for one request. Its key must be a composed
// operation key.
func TestBind_ProtocolResolverRejectsANonComposedKey(t *testing.T) {
	fake := &fakeResolver{name: "fake", resolution: Resolution{ChainKey: "GET|/pets|h"}}
	pr := prepareWith(t, registryWith(t, fake), ResolverRouteConfig{
		ResolverName:      "fake",
		RouteKey:          "GET|/pets|h",
		CanonicalChainKey: "GET|/pets|h",
		APIID:             "api-1",
		Vhost:             "h",
	})

	// The route's own chain exists, so only validation can stop the binding.
	_, chain, err := Bind(pr, fake.resolution, chainsPresent("GET|/pets|h").get)
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureInternal, re.Kind)
	assert.Contains(t, re.Cause.Error(), "malformed operation chain key")
	assert.Nil(t, chain, "the route-level chain must not have been bound")
}

func TestBind_ProtocolResolvedRoute(t *testing.T) {
	const (
		apiID = "api-1"
		vhost = "api.example.com"
	)
	key := func(operation string) string { return ChainKeyFor(apiID, vhost, operation) }

	tests := []struct {
		name      string
		operation string // the operation embedded in the key, and so the one reported
		chains    []string
		wantKind  FailureKind
	}{
		{
			name:      "the named chain exists",
			operation: "OperationOne",
			chains:    []string{key("OperationOne")},
		},
		{
			// Resolve returned a resolution at all, so the resolver has already vouched
			// for the operation. A missing chain is therefore always deployment or xDS
			// skew — never "unknown operation", which would blame the caller for a
			// server-side gap. An operation the resolver does not recognise never gets
			// this far: Resolve returns FailureUnknownOperation itself.
			name:      "no chain for a recognised operation is deployment skew",
			operation: "OperationOne",
			chains:    []string{key("GetTask")},
			wantKind:  FailureChainMissing,
		},
		{
			// Same outcome for an identifier from an open space: the resolver having
			// returned it is the claim that it is valid, so the binder does not
			// re-adjudicate.
			name:      "no chain for an open-space identifier is also skew",
			operation: "tools/call:unlisted",
			chains:    []string{key("tools/call:add")},
			wantKind:  FailureChainMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolution := Resolution{ChainKey: key(tt.operation)}
			fake := &fakeResolver{name: "fake", resolution: resolution}
			pr := prepareWith(t, registryWith(t, fake), ResolverRouteConfig{
				ResolverName:      "fake",
				RouteKey:          "POST|/rpc|" + vhost,
				CanonicalChainKey: "POST|/rpc|" + vhost,
				APIID:             apiID,
				Vhost:             vhost,
			})

			store := chainsPresent(tt.chains...)
			bound, _, err := Bind(pr, resolution, store.get)
			if tt.wantKind == "" {
				require.NoError(t, err)
				assert.Equal(t, key(tt.operation), bound.ChainKey)
				assert.Equal(t, tt.operation, bound.Operation,
					"the reported operation is read back out of the key that ran")
				assert.Equal(t, []string{key(tt.operation)}, store.lookedUp,
					"one resolution names one key, so binding costs exactly one lookup")
				return
			}
			var re *ResolutionError
			require.True(t, errors.As(err, &re))
			assert.Equal(t, tt.wantKind, re.Kind)
			assert.Empty(t, bound.ChainKey, "an unresolved request must never bind another chain")
		})
	}
}

// The reported operation always names the chain that actually ran. This is the divergence
// the derivation exists to make unrepresentable: a resolver returning the GetTask chain key
// cannot also report SendMessage, because there is no field with which to say so — so
// telemetry can never name one operation while another operation's authentication,
// authorization and rate limits are the ones enforced.
func TestBind_ReportedOperationAlwaysNamesTheChainThatRan(t *testing.T) {
	const (
		apiID = "api-1"
		vhost = "api.example.com"
	)
	getTask := ChainKeyFor(apiID, vhost, "GetTask")

	// A resolver that meant SendMessage but composed the GetTask key: the mistake is in
	// the key, and the key is what runs, so that is what must be reported.
	resolution := Resolution{ChainKey: getTask}
	fake := &fakeResolver{name: "fake", resolution: resolution}
	pr := prepareWith(t, registryWith(t, fake), ResolverRouteConfig{
		ResolverName: "fake", RouteKey: "POST|/rpc|" + vhost,
		CanonicalChainKey: "POST|/rpc|" + vhost, APIID: apiID, Vhost: vhost,
	})

	bound, chain, err := Bind(pr, resolution, chainsPresent(getTask, ChainKeyFor(apiID, vhost, "SendMessage")).get)
	require.NoError(t, err)
	require.NotNil(t, chain)

	assert.Equal(t, getTask, bound.ChainKey)
	assert.Equal(t, "GetTask", bound.Operation,
		"the operation is read out of the key that ran, never asserted alongside it")
	assert.Equal(t, getTask, chain.key,
		"and the chain executed is the one the reported operation names")
}

// A direct route reports no operation: the route chose the chain, not the resolver. It holds
// even when the route is pointed at a composed operation key, which a directly-resolved
// route may be.
func TestBind_DirectRouteReportsNoOperation(t *testing.T) {
	const (
		apiID = "api-1"
		vhost = "h"
	)
	composed := ChainKeyFor(apiID, vhost, "OperationOne")
	pr := prepareWith(t, DefaultRegistry(), ResolverRouteConfig{
		RouteKey: "POST|/op-one|" + vhost, CanonicalChainKey: composed,
		APIID: apiID, Vhost: vhost,
	})

	bound, _, err := BindStatic(pr, chainsPresent(composed).get)
	require.NoError(t, err)
	assert.Equal(t, composed, bound.ChainKey)
	assert.Empty(t, bound.Operation,
		"a direct route identified no operation, and the chain key is already on the span")
}

// A resolver cannot reach a chain outside its own route's partition, however it
// composed the key.
func TestBind_OperationTargetCannotCrossAPartition(t *testing.T) {
	const vhost = "api.example.com"
	tests := []struct {
		name string
		key  string
	}{
		{"another API's operation", ChainKeyFor("other-api", vhost, "OperationOne")},
		{"another vhost's operation", ChainKeyFor("api-1", "other.example.com", "OperationOne")},
		{"not a composed key at all", "OperationOne"},
		{"composed but with no operation", ChainKeyFor("api-1", vhost, "")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolution := Resolution{ChainKey: tt.key}
			fake := &fakeResolver{name: "fake", resolution: resolution}
			pr := prepareWith(t, registryWith(t, fake), ResolverRouteConfig{
				ResolverName: "fake", RouteKey: "POST|/rpc|" + vhost,
				CanonicalChainKey: "POST|/rpc|" + vhost, APIID: "api-1", Vhost: vhost,
			})

			// The probe says the key exists, so only validation can stop it.
			_, _, err := Bind(pr, resolution, chainsPresent(tt.key).get)
			var re *ResolutionError
			require.True(t, errors.As(err, &re))
			assert.Equal(t, FailureInternal, re.Kind)
		})
	}
}

// A malformed key fails the resolution and binds nothing. With one key per resolution
// there is nothing to fall back to, so this is simply a resolver bug — and it must not
// reach for some other chain that happens to exist.
func TestBind_MalformedKeyBindsNothing(t *testing.T) {
	const (
		apiID = "api-1"
		vhost = "api.example.com"
	)
	// A chain that does exist, to prove the failure does not quietly land on it.
	existing := ChainKeyFor(apiID, vhost, "tools/call")
	resolution := Resolution{ChainKey: "forged-not-a-composed-key"}
	fake := &fakeResolver{name: "fake", resolution: resolution}
	pr := prepareWith(t, registryWith(t, fake), ResolverRouteConfig{
		ResolverName: "fake", RouteKey: "POST|/rpc|" + vhost,
		CanonicalChainKey: "POST|/rpc|" + vhost, APIID: apiID, Vhost: vhost,
	})

	store := chainsPresent(existing)
	bound, chain, err := Bind(pr, resolution, store.get)
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureInternal, re.Kind)
	assert.Empty(t, bound.ChainKey)
	assert.Nil(t, chain)
	assert.Empty(t, store.lookedUp, "validation fails before any chain is looked up")
}

// There is no target to leave unset any more: the rule comes from the route's resolver
// name, which preparation normalises and fixes. What used to be "a resolver forgot to set a
// target" is now structurally impossible, and the two rules it used to select between are
// covered by TestBind_RouteKeyRejectsAnyOtherKey and
// TestBind_ProtocolResolverRejectsANonComposedKey.
func TestBind_ValidationRuleComesFromThePreparedResolver(t *testing.T) {
	const key = "GET|/pets|h"

	// One resolution value, two routes differing only by which resolver prepared them.
	resolution := Resolution{ChainKey: key}

	routeKeyRoute := prepareWith(t,
		registryWith(t, &fakeResolver{name: RouteKeyResolverName, resolution: resolution}),
		ResolverRouteConfig{
			ResolverName: RouteKeyResolverName, RouteKey: key, CanonicalChainKey: key,
			APIID: "api-1", Vhost: "h",
		})
	protocolRoute := prepareWith(t,
		registryWith(t, &fakeResolver{name: "fake", resolution: resolution}),
		ResolverRouteConfig{
			ResolverName: "fake", RouteKey: key, CanonicalChainKey: key,
			APIID: "api-1", Vhost: "h",
		})

	// Accepted on the route-key route: the key is that route's own.
	bound, chain, err := Bind(routeKeyRoute, resolution, chainsPresent(key).get)
	require.NoError(t, err)
	require.NotNil(t, chain)
	assert.Equal(t, key, bound.ChainKey)
	assert.Empty(t, bound.Operation, "a directly-resolved route reports no operation")

	// Refused on the protocol route: the same key is not a composed operation key. The
	// resolution did not change — only the resolver that prepared the route did.
	_, chain, err = Bind(protocolRoute, resolution, chainsPresent(key).get)
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureInternal, re.Kind)
	assert.Nil(t, chain)
}

func TestBind_RejectsAResolutionWithNoChainKey(t *testing.T) {
	pr := prepareWith(t, DefaultRegistry(), ResolverRouteConfig{
		RouteKey: "GET|/pets|h", CanonicalChainKey: "GET|/pets|h",
	})

	_, _, err := Bind(pr, Resolution{}, noChains)
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureInvalidRequest, re.Kind,
		"a resolver that identified nothing is a bad request, not an engine fault")
}

// A nil chain accessor must fail closed rather than resolve or panic: without it the chain
// would appear not to exist, which on a protocol-resolved route would be reported as
// deployment skew — a misdiagnosis of what is really an engine wiring fault.
func TestBind_NilChainAccessorFailsClosedAsInternal(t *testing.T) {
	resolution := Resolution{ChainKey: ChainKeyFor("a", "v", "Op")}
	fake := &fakeResolver{name: "fake", resolution: resolution}
	pr := prepareWith(t, registryWith(t, fake), ResolverRouteConfig{
		ResolverName: "fake", RouteKey: "r", CanonicalChainKey: "r", APIID: "a", Vhost: "v",
	})

	_, _, err := Bind[fakeChain](pr, resolution, nil)
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureInternal, re.Kind)
}

func TestBind_NilPreparedRouteFailsClosed(t *testing.T) {
	var pr *PreparedRoute
	_, _, err := Bind(pr, Resolution{ChainKey: "k"}, noChains)
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureInternal, re.Kind)
}

// The convergence property the whole design exists for: two routes reaching the same
// logical operation must select the same chain, whether the operation was read out of the
// request or fixed at deploy time. They converge because both compose from one canonical
// operation name with one shared helper — not because either was pointed at the other's
// key.
func TestBind_RoutesForOneOperationConverge(t *testing.T) {
	const (
		apiID     = "api-1"
		vhost     = "api.example.com"
		operation = "OperationOne"
	)
	composed := ChainKeyFor(apiID, vhost, operation)
	probe := chainsPresent(composed)

	// Resolved from the request: many operations share this route.
	perRequestResolution := Resolution{
		ChainKey: composed,
	}
	multiplexed := prepareWith(t,
		registryWith(t, &fakeResolver{name: "fake-multiplexed", resolution: perRequestResolution}),
		ResolverRouteConfig{
			ResolverName: "fake-multiplexed", RouteKey: "POST|/rpc|" + vhost,
			CanonicalChainKey: "POST|/rpc|" + vhost, APIID: apiID, Vhost: vhost,
		})
	fromPerRequest, _, err := Bind(multiplexed, perRequestResolution, probe.get)
	require.NoError(t, err)

	// Fixed at ingest: this route serves one operation, named in its configuration.
	staticResolution := Resolution{
		ChainKey: composed,
	}
	perOperation := prepareWith(t,
		registryWith(t, &fakeResolver{name: "fake-per-operation", prepare: func(ResolverRouteConfig) (PreparedResolver, error) {
			return &fakeStatic{static: staticResolution}, nil
		}}),
		ResolverRouteConfig{
			ResolverName: "fake-per-operation", RouteKey: "POST|/op-one|" + vhost,
			CanonicalChainKey: "POST|/op-one|" + vhost, APIID: apiID, Vhost: vhost,
		})
	require.True(t, perOperation.IsStatic())
	fromStatic, _, err := BindStatic(perOperation, probe.get)
	require.NoError(t, err)

	assert.Equal(t, fromPerRequest.ChainKey, fromStatic.ChainKey)
	assert.Equal(t, composed, fromPerRequest.ChainKey)
}

// ─── Errors ──────────────────────────────────────────────────────────────────

func TestNormalizeResolutionError(t *testing.T) {
	assert.Nil(t, NormalizeResolutionError(nil))

	// A typed error with no kind is still classified rather than left blank.
	re := NormalizeResolutionError(&ResolutionError{})
	assert.Equal(t, FailureInternal, re.Kind)

	// A typed error keeps its own classification.
	re = NormalizeResolutionError(&ResolutionError{Kind: FailureParse})
	assert.Equal(t, FailureParse, re.Kind)

	// Normalizing must not mutate the resolver's error value.
	original := &ResolutionError{Kind: FailureParse}
	_ = NormalizeResolutionError(original)
	assert.Equal(t, FailureParse, original.Kind)

	// A wrapped typed error keeps its classification.
	re = NormalizeResolutionError(fmt.Errorf("wrapped: %w", &ResolutionError{Kind: FailureUnknownOperation}))
	assert.Equal(t, FailureUnknownOperation, re.Kind)
}

// An untyped resolver error must not be guessed at: it becomes FailureInternal,
// which renders generically and never reaches the client.
func TestNormalizeResolutionError_UntypedBecomesInternal(t *testing.T) {
	re := NormalizeResolutionError(errors.New("boom"))
	assert.Equal(t, FailureInternal, re.Kind)
	assert.EqualError(t, re.Cause, "boom", "the cause is kept for the internal log only")
}

func TestRouteResolution_IsIdentity(t *testing.T) {
	assert.True(t, (&RouteResolution{}).IsIdentity())
	assert.True(t, (&RouteResolution{ResolverName: RouteKeyResolverName}).IsIdentity())
	assert.False(t, (&RouteResolution{ResolverName: "fake-multiplexed"}).IsIdentity())
}

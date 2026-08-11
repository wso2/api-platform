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
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Fake resolvers ──────────────────────────────────────────────────────────
//
// These exist so the resolution seam is fully testable before any real
// multiplexed kind ships: the production binary registers only "route-key", so
// nothing below is reachable in production.

// fakeResolver reports whatever it is configured to return, so the tests can drive
// every branch of ResolveChainKey without a protocol implementation.
type fakeResolver struct {
	name       string
	reqs       Requirements
	resolution Resolution
	err        error
	calls      int
}

func (f *fakeResolver) Name() string               { return f.name }
func (f *fakeResolver) Requirements() Requirements { return f.reqs }
func (f *fakeResolver) Identify(RequestView) (Resolution, error) {
	f.calls++
	return f.resolution, f.err
}

// preparingResolver additionally implements Preparer.
type preparingResolver struct {
	fakeResolver
	prepareErr   error
	prepareCalls int
	lastConfig   json.RawMessage
}

func (p *preparingResolver) Prepare(cfg json.RawMessage) (any, error) {
	p.prepareCalls++
	p.lastConfig = cfg
	if p.prepareErr != nil {
		return nil, p.prepareErr
	}
	return "prepared-state", nil
}

func oneOperation(candidates ...string) Resolution {
	return Resolution{Operations: []Operation{{Candidates: candidates}}}
}

func registryWith(t *testing.T, resolvers ...OperationResolver) *Registry {
	t.Helper()
	reg := NewRegistry()
	for _, r := range resolvers {
		require.NoError(t, reg.Register(r))
	}
	reg.Freeze()
	return reg
}

// ─── Identity resolver ───────────────────────────────────────────────────────

func TestRouteKeyResolver_Contract(t *testing.T) {
	r := &RouteKeyResolver{}
	assert.Equal(t, "route-key", r.Name())
	assert.Equal(t, Requirements{}, r.Requirements())

	res, err := r.Identify(RequestView{RouteKey: "GET|/api/v1/users|example.com"})
	require.NoError(t, err)
	require.Len(t, res.Operations, 1)
	assert.Equal(t, []string{"GET|/api/v1/users|example.com"}, res.Operations[0].Candidates)
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

// Freezing is what guarantees that what the runtime advertised to the control plane
// and what it can serve stay the same set for the process lifetime.
func TestRegistry_FrozenRejectsMutation(t *testing.T) {
	reg := NewRegistry()
	require.NoError(t, reg.Register(&fakeResolver{name: "before"}))
	reg.Freeze()

	assert.True(t, reg.Frozen())
	err := reg.Register(&fakeResolver{name: "after"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "frozen")

	_, ok := reg.Get("after")
	assert.False(t, ok)
}

func TestRegistry_NamesAreSorted(t *testing.T) {
	reg := registryWith(t, &fakeResolver{name: "zeta"}, &fakeResolver{name: "alpha"}, &fakeResolver{name: "mid"})
	assert.Equal(t, []string{"alpha", "mid", "zeta"}, reg.Names())
}

// The production registry ships identity-only, so nothing a resolver could read out
// of a request is reachable in the shipped binary yet.
func TestDefaultRegistry_IsIdentityOnlyAndFrozen(t *testing.T) {
	def := DefaultRegistry()
	assert.Equal(t, []string{RouteKeyResolverName}, def.Names())

	r, ok := def.Get(RouteKeyResolverName)
	require.True(t, ok)
	assert.Equal(t, RouteKeyResolverName, r.Name())

	assert.True(t, defaultRegistry.Frozen(), "DefaultRegistry must freeze the production registry")
}

// A test registry must not be able to leak a resolver into the production one.
func TestIndependentRegistryDoesNotAffectDefault(t *testing.T) {
	_ = registryWith(t, &fakeResolver{name: "test-only"})

	_, ok := DefaultRegistry().Get("test-only")
	assert.False(t, ok, "a resolver registered in a test registry must not appear in the production registry")
	assert.Equal(t, []string{RouteKeyResolverName}, DefaultRegistry().Names())
}

// ─── ResolveChainKey: identity ───────────────────────────────────────────────

func TestResolveChainKey_Identity(t *testing.T) {
	// A nil registry proves the identity path never consults one.
	for _, name := range []string{"", RouteKeyResolverName} {
		t.Run(fmt.Sprintf("resolver_name=%q", name), func(t *testing.T) {
			rc := &RouteResolution{
				RouteKey:          "GET|/pets|example.com",
				CanonicalChainKey: "GET|/pets|example.com",
				ResolverName:      name,
			}
			key, res, err := ResolveChainKey(nil, rc, RequestView{}, noChains)
			require.NoError(t, err)
			assert.Equal(t, "GET|/pets|example.com", key)
			assert.Empty(t, res.Operations)
		})
	}
}

// The canonical key is read from the field, never rebuilt from the route key: that
// is the seam that keeps a later move to a separate key namespace a controller-only
// change.
func TestResolveChainKey_IdentityReadsCanonicalKeyNotRouteKey(t *testing.T) {
	rc := &RouteResolution{
		RouteKey:          "POST|/message:send|example.com",
		CanonicalChainKey: "operation-chain-key",
	}
	key, _, err := ResolveChainKey(nil, rc, RequestView{}, noChains)
	require.NoError(t, err)
	assert.Equal(t, "operation-chain-key", key)
}

// ─── ResolveChainKey: non-identity ───────────────────────────────────────────

func TestResolveChainKey_UnknownResolver(t *testing.T) {
	reg := registryWith(t, &fakeResolver{name: "known"})
	rc := &RouteResolution{RouteKey: "r", CanonicalChainKey: "r", ResolverName: "not-registered"}

	key, _, err := ResolveChainKey(reg, rc, RequestView{}, noChains)
	assert.Empty(t, key, "an unresolved request must never fall back to the route-level chain")

	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureUnknownResolver, re.Kind)
	assert.False(t, re.ProtocolVisible(), "the protocol is exactly what is unknown; it must render generically")
}

// A nil registry on a non-identity route must fail closed, not panic and not resolve.
func TestResolveChainKey_NilRegistryFailsClosed(t *testing.T) {
	rc := &RouteResolution{RouteKey: "r", CanonicalChainKey: "r", ResolverName: "a2a-jsonrpc"}
	_, _, err := ResolveChainKey(nil, rc, RequestView{}, noChains)

	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureUnknownResolver, re.Kind)
}

// chainsPresent builds the chain-existence probe ResolveChainKey composes against.
func chainsPresent(keys ...string) func(string) bool {
	present := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		present[k] = struct{}{}
	}
	return func(key string) bool {
		_, ok := present[key]
		return ok
	}
}

// noChains is the probe for a route whose partition has no operation chains at all.
func noChains(string) bool { return false }

func TestResolveChainKey_CandidateLadder(t *testing.T) {
	const (
		apiID = "api-1"
		vhost = "api.example.com"
	)
	key := func(operation string) string { return ChainKeyFor(apiID, vhost, operation) }

	tests := []struct {
		name            string
		candidates      []string
		knownToProtocol bool
		chains          []string
		wantOperation   string
		wantKind        FailureKind
	}{
		{
			name:          "single candidate hit",
			candidates:    []string{"SendMessage"},
			chains:        []string{key("SendMessage")},
			wantOperation: "SendMessage",
		},
		{
			// The whole point of the ladder: a specific chain wins over the generic
			// fallback when it exists.
			name:          "first candidate wins over later ones",
			candidates:    []string{"tools/call:add", "tools/call"},
			chains:        []string{key("tools/call:add"), key("tools/call")},
			wantOperation: "tools/call:add",
		},
		{
			name:          "falls back to the next candidate",
			candidates:    []string{"tools/call:unlisted", "tools/call"},
			chains:        []string{key("tools/call")},
			wantOperation: "tools/call",
		},
		{
			// Open operation set: the client named a tool that does not exist, so this
			// is a 404-shaped failure rather than a deployment error.
			name:       "no candidate has a chain, open operation set",
			candidates: []string{"tools/call:unlisted"},
			chains:     []string{key("tools/call:add")},
			wantKind:   FailureUnknownOperation,
		},
		{
			// Closed operation set: the protocol says this operation exists, so a
			// missing chain means the controller built the deployment wrong. Rendering
			// it as "unknown operation" would blame the client for a server bug.
			name:            "no candidate has a chain, closed operation set",
			candidates:      []string{"SendMessage"},
			knownToProtocol: true,
			chains:          []string{key("GetTask")},
			wantKind:        FailureChainMissing,
		},
		{
			// A candidate carrying the key separator could otherwise compose the key of
			// a different (apiID, vhost, operation) triple, so it is skipped entirely.
			name:          "candidate containing the separator is skipped",
			candidates:    []string{"forged\x1fmessage/send", "SendMessage"},
			chains:        []string{key("SendMessage")},
			wantOperation: "SendMessage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeResolver{name: "fake", resolution: Resolution{
				Operations: []Operation{{Candidates: tt.candidates, KnownToProtocol: tt.knownToProtocol}},
			}}
			reg := registryWith(t, fake)
			rc := &RouteResolution{
				RouteKey:     "POST|/rpc|" + vhost,
				ResolverName: "fake",
			}
			view := RequestView{APIID: apiID, Vhost: vhost}

			got, _, err := ResolveChainKey(reg, rc, view, chainsPresent(tt.chains...))
			if tt.wantKind == "" {
				require.NoError(t, err)
				assert.Equal(t, key(tt.wantOperation), got)
				return
			}
			var re *ResolutionError
			require.True(t, errors.As(err, &re))
			assert.Equal(t, tt.wantKind, re.Kind)
			assert.Empty(t, got)
		})
	}
}

// The convergence property the whole design exists for: the JSON-RPC transport composes
// its key at request time from the parsed body, and the HTTP+JSON transport carries the
// key its transformer composed at deploy time. They must be the same string.
func TestResolveChainKey_TransportsConverge(t *testing.T) {
	const (
		apiID     = "agent-1"
		vhost     = "api.example.com"
		operation = "SendMessage"
	)
	composed := ChainKeyFor(apiID, vhost, operation)

	// JSON-RPC: a resolver identifies the operation and the engine composes.
	fake := &fakeResolver{name: "a2a-jsonrpc", resolution: Resolution{
		Operations: []Operation{{Candidates: []string{operation}, KnownToProtocol: true}},
	}}
	fromJSONRPC, _, err := ResolveChainKey(
		registryWith(t, fake),
		&RouteResolution{RouteKey: "POST|/agent|" + vhost, ResolverName: "a2a-jsonrpc"},
		RequestView{APIID: apiID, Vhost: vhost},
		chainsPresent(composed),
	)
	require.NoError(t, err)

	// HTTP+JSON: an identity route pointed at the key the controller composed.
	fromHTTPJSON, _, err := ResolveChainKey(
		nil,
		&RouteResolution{RouteKey: "POST|/agent/message:send|" + vhost, CanonicalChainKey: composed},
		RequestView{APIID: apiID, Vhost: vhost},
		noChains,
	)
	require.NoError(t, err)

	assert.Equal(t, fromJSONRPC, fromHTTPJSON)
	assert.Equal(t, composed, fromJSONRPC)
}

// An identity route never consults the probe: it has no operation to compose, and its
// chain is looked up by the kernel from the key it returns.
func TestResolveChainKey_IdentityIgnoresTheProbe(t *testing.T) {
	rc := &RouteResolution{RouteKey: "GET|/pets|h", CanonicalChainKey: "GET|/pets|h"}

	key, _, err := ResolveChainKey(nil, rc, RequestView{}, func(string) bool {
		t.Fatal("the probe must not be called on an identity route")
		return false
	})
	require.NoError(t, err)
	assert.Equal(t, "GET|/pets|h", key)
}

// A nil probe must fail closed rather than panic: a partially-wired caller then gets a
// resolution failure instead of taking the whole process down on the hot path.
func TestResolveChainKey_NilProbeFailsClosed(t *testing.T) {
	fake := &fakeResolver{name: "fake", resolution: oneOperation("SendMessage")}
	rc := &RouteResolution{ResolverName: "fake"}

	_, _, err := ResolveChainKey(registryWith(t, fake), rc, RequestView{APIID: "a", Vhost: "v"}, nil)
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureUnknownOperation, re.Kind)
}

func TestResolveChainKey_NoOperationIsInvalidRequest(t *testing.T) {
	fake := &fakeResolver{name: "fake", resolution: Resolution{ProtocolState: "state"}}
	reg := registryWith(t, fake)
	rc := &RouteResolution{ResolverName: "fake"}

	_, _, err := ResolveChainKey(reg, rc, RequestView{}, noChains)
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureInvalidRequest, re.Kind)
	assert.Equal(t, "state", re.ProtocolState, "protocol state must survive to the renderer")
}

// The Operations slice is a conjunction, reserved for a protocol where one request
// carries several operations (JSON-RPC batch). There is no composition rule yet, so
// it is refused rather than silently applying only the first.
func TestResolveChainKey_MultipleOperationsRejected(t *testing.T) {
	fake := &fakeResolver{name: "fake", resolution: Resolution{
		Operations:    []Operation{{Candidates: []string{"A"}}, {Candidates: []string{"B"}}},
		ProtocolState: "state",
	}}
	reg := registryWith(t, fake)
	rc := &RouteResolution{ResolverName: "fake"}

	key, _, err := ResolveChainKey(reg, rc, RequestView{}, noChains)
	assert.Empty(t, key)
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureMultiOperation, re.Kind)
}

func TestResolveChainKey_ResolverErrorIsPropagated(t *testing.T) {
	typed := &ResolutionError{Kind: FailureParse, Cause: errors.New("bad json")}
	fake := &fakeResolver{name: "fake", err: typed}
	reg := registryWith(t, fake)
	rc := &RouteResolution{ResolverName: "fake"}

	_, _, err := ResolveChainKey(reg, rc, RequestView{}, noChains)
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureParse, re.Kind)
	assert.True(t, re.ProtocolVisible())
}

// An untyped resolver error must not be guessed at: it becomes FailureInternal,
// which renders generically and never reaches the client.
func TestResolveChainKey_UntypedResolverErrorBecomesInternal(t *testing.T) {
	fake := &fakeResolver{name: "fake", err: errors.New("boom")}
	reg := registryWith(t, fake)
	rc := &RouteResolution{ResolverName: "fake"}

	_, _, err := ResolveChainKey(reg, rc, RequestView{}, noChains)
	var re *ResolutionError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, FailureInternal, re.Kind)
	assert.False(t, re.ProtocolVisible())
	assert.EqualError(t, re.Cause, "boom", "the cause is kept for the internal log only")
}

func TestNormalizeResolutionError(t *testing.T) {
	assert.Nil(t, NormalizeResolutionError(nil, nil))

	// A typed error with no kind is still classified rather than left blank.
	re := NormalizeResolutionError(&ResolutionError{}, "state")
	assert.Equal(t, FailureInternal, re.Kind)
	assert.Equal(t, "state", re.ProtocolState)

	// A typed error's own protocol state wins over the resolution's.
	re = NormalizeResolutionError(&ResolutionError{Kind: FailureParse, ProtocolState: "own"}, "outer")
	assert.Equal(t, "own", re.ProtocolState)

	// Normalizing must not mutate the resolver's error value.
	original := &ResolutionError{Kind: FailureParse}
	_ = NormalizeResolutionError(original, "state")
	assert.Nil(t, original.ProtocolState)

	// A wrapped typed error keeps its classification.
	re = NormalizeResolutionError(fmt.Errorf("wrapped: %w", &ResolutionError{Kind: FailureUnknownOperation}), nil)
	assert.Equal(t, FailureUnknownOperation, re.Kind)
}

func TestProtocolVisible(t *testing.T) {
	visible := []FailureKind{FailureParse, FailureInvalidRequest, FailureUnknownOperation}
	// Transport-level and configuration failures all render generically. An encoding
	// problem in particular happened below the protocol layer, so a protocol renderer
	// must not dress it up as a protocol-level error the client could act on.
	generic := []FailureKind{
		FailureMultiOperation, FailurePayloadTooLarge, FailureUnknownResolver,
		FailureChainMissing, FailureInternal,
		FailureUnsupportedEncoding, FailureUndecodableBody,
	}
	for _, k := range visible {
		assert.True(t, (&ResolutionError{Kind: k}).ProtocolVisible(), "%s should be protocol-visible", k)
	}
	for _, k := range generic {
		assert.False(t, (&ResolutionError{Kind: k}).ProtocolVisible(), "%s must render generically", k)
	}
}

func TestRouteResolution_IsIdentity(t *testing.T) {
	assert.True(t, (&RouteResolution{}).IsIdentity())
	assert.True(t, (&RouteResolution{ResolverName: RouteKeyResolverName}).IsIdentity())
	assert.False(t, (&RouteResolution{ResolverName: "a2a-jsonrpc"}).IsIdentity())
}

// ─── Preparer ────────────────────────────────────────────────────────────────

func TestPreparer_ReceivesRouteConfig(t *testing.T) {
	p := &preparingResolver{fakeResolver: fakeResolver{name: "prep"}}
	cfg := json.RawMessage(`{"schema":"x"}`)

	state, err := p.Prepare(cfg)
	require.NoError(t, err)
	assert.Equal(t, "prepared-state", state)
	assert.Equal(t, 1, p.prepareCalls)
	assert.JSONEq(t, `{"schema":"x"}`, string(p.lastConfig))
}

func TestPreparer_ErrorIsReturned(t *testing.T) {
	p := &preparingResolver{fakeResolver: fakeResolver{name: "prep"}, prepareErr: errors.New("bad schema")}
	_, err := p.Prepare(nil)
	require.Error(t, err)
}

// RouteState is how a Prepare result reaches the resolver, so per-route work happens
// once at ingest rather than per request.
func TestRequestView_CarriesRouteState(t *testing.T) {
	var seen any
	capturing := &captureResolver{name: "capture", onIdentify: func(v RequestView) { seen = v.RouteState }}
	reg := registryWith(t, capturing)
	rc := &RouteResolution{
		ResolverName: "capture",
		RouteState:   "prepared-state",
	}

	view := RequestView{APIID: "api-1", Vhost: "h", RouteState: rc.RouteState}
	key, _, err := ResolveChainKey(reg, rc, view, chainsPresent(ChainKeyFor("api-1", "h", "Op")))
	require.NoError(t, err)
	assert.Equal(t, ChainKeyFor("api-1", "h", "Op"), key)
	assert.Equal(t, "prepared-state", seen)
}

type captureResolver struct {
	name       string
	onIdentify func(RequestView)
}

func (c *captureResolver) Name() string               { return c.name }
func (c *captureResolver) Requirements() Requirements { return Requirements{} }
func (c *captureResolver) Identify(v RequestView) (Resolution, error) {
	c.onIdentify(v)
	return oneOperation("Op"), nil
}

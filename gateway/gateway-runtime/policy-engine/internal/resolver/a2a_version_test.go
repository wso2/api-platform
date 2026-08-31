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
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/agentproto"
)

// The A2A request protocol-version guard: the header-only phase every prepared A2A
// resolver runs before anything is bound, buffered or forwarded.
//
// A2A 1.0 §3.6.1 makes stating the version a client obligation and §3.6.2 fixes what
// silence means (0.3, not "whatever the server serves"). The tests below are written
// against those two rules rather than against the implementation: each names the rule
// it pins, because the rules are the part that must not drift.

// getTaskRoute prepares one of the nine HTTP+JSON operations whose resolution is
// static — the shape that would silently skip validation if the hook were placed
// after the static branch.
func getTaskRoute(t *testing.T) *PreparedRoute {
	t.Helper()
	return prepareWith(t, DefaultRegistry(), a2aRoute("GET|/weather/http/tasks/{id}|"+a2aVhost,
		a2aConfig(t, agentproto.V1_0, agentproto.TransportHTTPJSON, agentproto.GetTask)))
}

// headerView builds the header-phase view the kernel hands ValidateHeaders. Header
// names are given lowercased, as Envoy delivers them.
func headerView(path string, headers map[string][]string) HeaderRequestView {
	return HeaderRequestView{
		Method:  "POST",
		Path:    path,
		Headers: NewHeaderMap(headers),
	}
}

// versionHeader is the common case: one well-formed header value.
func versionHeader(value string) map[string][]string {
	return map[string][]string{A2AVersionHeader: {value}}
}

// ─── Every prepared form validates ───────────────────────────────────────────

// The guard has to reach all three prepared shapes, and each is reachable by a
// different path through the kernel: the static one never calls Resolve at all, the
// body-reading ones do not call it until the body has been buffered. A guard on only
// one of them would leave the other two unprotected while every test of that one
// passed.
func TestA2AVersion_EveryPreparedFormValidatesHeaders(t *testing.T) {
	routes := map[string]*PreparedRoute{
		"json-rpc":                 jsonRPCRoute(t, agentproto.V1_0),
		"http+json static":         getTaskRoute(t),
		"http+json body-resolving": sendMessageRoute(t),
	}

	for name, pr := range routes {
		t.Run(name, func(t *testing.T) {
			require.True(t, pr.ValidatesHeaders(),
				"every A2A operation route must validate the request's protocol version")

			assert.Nil(t, pr.ValidateRequestHeaders(context.Background(),
				headerView("/weather/rpc", versionHeader("1.0"))))

			failure := pr.ValidateRequestHeaders(context.Background(),
				headerView("/weather/rpc", versionHeader("0.3")))
			require.NotNil(t, failure)
			assert.Equal(t, FailureVersionNotSupported, failure.Kind)
		})
	}
}

// The identity resolver — every kind shipping today — must not acquire a phase it
// never asked for. This is what keeps a non-A2A static route on exactly the path it
// had before Section 8A existed.
func TestA2AVersion_RouteKeyResolverHasNoHeaderPhase(t *testing.T) {
	pr := prepareWith(t, DefaultRegistry(), ResolverRouteConfig{
		RouteKey:          "GET|/orders|" + a2aVhost,
		CanonicalChainKey: "GET|/orders|" + a2aVhost,
		ResolverName:      RouteKeyResolverName,
	})

	assert.False(t, pr.ValidatesHeaders())
	assert.Nil(t, pr.HeaderValidator)
	// Safe to call unconditionally: a route without a validator refuses nothing.
	assert.Nil(t, pr.ValidateRequestHeaders(context.Background(), headerView("/orders", nil)))
}

// ─── §3.6.1: the two representations ─────────────────────────────────────────

// Header lookup is case-insensitive (HTTP field names are), while the query
// parameter is compared as the specification spells it.
func TestA2AVersion_HeaderLookupIsCaseInsensitive(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	for _, spelling := range []string{"a2a-version", "A2A-Version", "A2A-VERSION"} {
		t.Run(spelling, func(t *testing.T) {
			assert.Nil(t, pr.ValidateRequestHeaders(context.Background(),
				headerView("/weather/rpc", map[string][]string{spelling: {"1.0"}})))
		})
	}
}

// The query alternative exists for a client that cannot set headers — an SSE
// consumer, a browser following a link.
func TestA2AVersion_QueryParameterIsAnEquivalentRepresentation(t *testing.T) {
	pr := getTaskRoute(t)

	assert.Nil(t, pr.ValidateRequestHeaders(context.Background(),
		headerView("/weather/http/tasks/t-1?A2A-Version=1.0", nil)))

	failure := pr.ValidateRequestHeaders(context.Background(),
		headerView("/weather/http/tasks/t-1?A2A-Version=0.3", nil))
	require.NotNil(t, failure)
	assert.Equal(t, FailureVersionNotSupported, failure.Kind)
}

// Query parameter names are case-sensitive, so a differently-cased spelling is a
// different parameter — which means the request stated nothing, which means 0.3.
// Folding it onto the specified name would invent a representation the
// specification does not define.
func TestA2AVersion_QueryParameterNameIsCaseSensitive(t *testing.T) {
	pr := getTaskRoute(t)

	failure := pr.ValidateRequestHeaders(context.Background(),
		headerView("/weather/http/tasks/t-1?a2a-version=1.0", nil))
	require.NotNil(t, failure)
	assert.Equal(t, FailureVersionNotSupported, failure.Kind,
		"a differently-cased query parameter states no version, so the implicit 0.3 applies")
}

// A percent-encoded value decodes to the same version. The header form gets optional
// whitespace trimmed instead; neither normalisation goes further than that.
func TestA2AVersion_ValuesAreDecodedAndTrimmedButNotOtherwiseNormalised(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	assert.Nil(t, pr.ValidateRequestHeaders(context.Background(),
		headerView("/weather/rpc?A2A-Version=1%2E0", nil)),
		"the query parser decodes percent-escapes")

	assert.Nil(t, pr.ValidateRequestHeaders(context.Background(),
		headerView("/weather/rpc", versionHeader("  1.0\t"))),
		"header optional whitespace is trimmed")
}

// Both representations together are allowed — a client retrying through an
// intermediary that strips one has reason to send both — but only when they agree.
func TestA2AVersion_HeaderAndQueryMustAgree(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	assert.Nil(t, pr.ValidateRequestHeaders(context.Background(),
		headerView("/weather/rpc?A2A-Version=1.0", versionHeader("1.0"))),
		"identical values in both representations are accepted")

	failure := pr.ValidateRequestHeaders(context.Background(),
		headerView("/weather/rpc?A2A-Version=0.3", versionHeader("1.0")))
	require.NotNil(t, failure)
	assert.Equal(t, FailureConflictingParameter, failure.Kind)
}

// Repetition is refused even when the repeated values are identical.
//
// This is the rule most likely to look over-strict and is the one worth keeping: a
// duplicate is collapsed differently by different intermediaries (first wins, last
// wins, comma-joined), so accepting it would make the effective version depend on the
// path the request happened to take rather than on what the client sent.
func TestA2AVersion_RepeatedValuesAreRefusedEvenWhenEqual(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	cases := map[string]HeaderRequestView{
		"repeated header": headerView("/weather/rpc",
			map[string][]string{A2AVersionHeader: {"1.0", "1.0"}}),
		"comma-combined header": headerView("/weather/rpc", versionHeader("1.0,1.0")),
		"repeated query parameter": headerView(
			"/weather/rpc?A2A-Version=1.0&A2A-Version=1.0", nil),
	}

	for name, view := range cases {
		t.Run(name, func(t *testing.T) {
			failure := pr.ValidateRequestHeaders(context.Background(), view)
			require.NotNil(t, failure)
			assert.Equal(t, FailureConflictingParameter, failure.Kind)
		})
	}
}

// ─── §3.6.2: absent and empty mean 0.3 ───────────────────────────────────────

// The rule the whole section exists for. An absent or empty statement is a 0.3
// client, never a client of whatever this route serves — reading it the other way
// would let every non-conformant client through and make §3.6.1 decorative.
func TestA2AVersion_AbsentOrEmptyMeansZeroPointThreeNotTheConfiguredVersion(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	cases := map[string]HeaderRequestView{
		"no representation at all": headerView("/weather/rpc", nil),
		"empty header":             headerView("/weather/rpc", versionHeader("")),
		"whitespace-only header":   headerView("/weather/rpc", versionHeader("   ")),
		"empty query parameter":    headerView("/weather/rpc?A2A-Version=", nil),
	}

	for name, view := range cases {
		t.Run(name, func(t *testing.T) {
			failure := pr.ValidateRequestHeaders(context.Background(), view)
			require.NotNil(t, failure, "a 1.0 route must refuse an implicit 0.3 client")
			assert.Equal(t, FailureVersionNotSupported, failure.Kind)
			assert.NotContains(t, failure.Error(), "1.0\" is not canonical",
				"the implicit value is well-formed; it is unsupported, not malformed")
		})
	}
}

// A query string carrying other parameters — including ones that will not decode —
// states no version by that representation. That stays the agent's business: the
// gateway is validating a service parameter, not the request's whole query string,
// and refusing here would report the wrong fault for it.
func TestA2AVersion_UnrelatedQueryParametersStateNothing(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	for _, query := range []string{"?historyLength=5", "?%zz", "?bad=%zz", "?a2a-version=0.3"} {
		t.Run(query, func(t *testing.T) {
			assert.Nil(t, pr.ValidateRequestHeaders(context.Background(),
				headerView("/weather/rpc"+query, versionHeader("1.0"))))
		})
	}
}

// A malformed *unrelated* parameter must not swallow a version that was stated
// perfectly well beside it.
//
// This is why the query is scanned rather than handed to url.ParseQuery: that
// function returns the pairs it could decode *together with* an error about the ones
// it could not, so discarding the map on error discards the good pair too. Doing
// that here meant "?A2A-Version=0.3&bad=%zz" alongside a header of "1.0" read as
// "header only" and was accepted — the conflict, and with it a request stating a
// version this route does not serve, disappeared into a neighbouring typo.
func TestA2AVersion_AMalformedNeighbourDoesNotHideTheStatedVersion(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	conflicting := map[string]string{
		"malformed pair after the version":  "?A2A-Version=0.3&bad=%zz",
		"malformed pair before the version": "?bad=%zz&A2A-Version=0.3",
		"malformed pair on both sides":      "?bad=%zz&A2A-Version=0.3&worse=%g",
	}
	for name, query := range conflicting {
		t.Run(name, func(t *testing.T) {
			failure := pr.ValidateRequestHeaders(context.Background(),
				headerView("/weather/rpc"+query, versionHeader("1.0")))
			require.NotNil(t, failure, "the query stated 0.3; the header stated 1.0")
			assert.Equal(t, FailureConflictingParameter, failure.Kind)
		})
	}

	// The same hazard with no header at all: the query is then the only statement
	// there is, and it must still be read.
	failure := pr.ValidateRequestHeaders(context.Background(),
		headerView("/weather/rpc?A2A-Version=0.3&bad=%zz", nil))
	require.NotNil(t, failure)
	assert.Equal(t, FailureVersionNotSupported, failure.Kind)
}

// A version parameter that cannot itself be decoded is an invalid parameter, not an
// absent one. Reading it as absent would apply the implicit 0.3 and report the
// request as stating an unsupported version, which names the wrong fault — the
// client did state something, and nothing can read it.
func TestA2AVersion_AnUndecodableVersionParameterIsInvalidNotAbsent(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	failure := pr.ValidateRequestHeaders(context.Background(),
		headerView("/weather/rpc?A2A-Version=%zz", nil))
	require.NotNil(t, failure)
	assert.Equal(t, FailureInvalidParameter, failure.Kind)
}

// A repeat must be seen as a repeat even when one of the repeats is unusable, and an
// encoded spelling of the parameter name must not smuggle a second statement past
// the check. Both are cases where a parser that dropped the pair it disliked would
// leave what looks like a single unambiguous value.
func TestA2AVersion_RepeatsAreDetectedBeforeAnyValueIsDecoded(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	for name, query := range map[string]string{
		"one repeat undecodable": "?A2A-Version=1.0&A2A-Version=%zz",
		"encoded parameter name": "?A2A-Version=1.0&A2A%2DVersion=1.0",
	} {
		t.Run(name, func(t *testing.T) {
			failure := pr.ValidateRequestHeaders(context.Background(),
				headerView("/weather/rpc"+query, nil))
			require.NotNil(t, failure)
			assert.Equal(t, FailureConflictingParameter, failure.Kind)
		})
	}
}

// ─── Form and support ────────────────────────────────────────────────────────

// Canonical Major.Minor only. "1.0.0" and "01.0" are refused rather than folded onto
// "1.0": a client sending either is not sending what the specification defines, and
// guessing which version it meant is not the gateway's to do.
func TestA2AVersion_NonCanonicalValuesAreInvalidParameters(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	for _, value := range []string{
		"1.0.0", "01.0", "1.00", "+1.0", "-1.0", "1", "1.", ".0", "v1.0", "1 .0", "one.zero",
	} {
		t.Run(value, func(t *testing.T) {
			failure := pr.ValidateRequestHeaders(context.Background(),
				headerView("/weather/rpc", versionHeader(value)))
			require.NotNil(t, failure)
			assert.Equal(t, FailureInvalidParameter, failure.Kind)
		})
	}

	// Surrounding whitespace is invalid in the query representation, which gets no
	// OWS trimming — that allowance belongs to the HTTP field grammar and stops at
	// the header. Asserted here so the two representations are not quietly
	// normalised the same way.
	failure := pr.ValidateRequestHeaders(context.Background(),
		headerView("/weather/rpc?A2A-Version=%201.0", nil))
	require.NotNil(t, failure)
	assert.Equal(t, FailureInvalidParameter, failure.Kind)
}

// A well-formed version this route does not expose is a different answer from a
// malformed one: the client is speaking a real protocol version, just not this one.
// There is no range match and no newest-version fallback (D19).
func TestA2AVersion_AWellFormedOtherVersionIsUnsupported(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	for _, value := range []string{"0.3", "1.1", "2.0", "99.0"} {
		t.Run(value, func(t *testing.T) {
			failure := pr.ValidateRequestHeaders(context.Background(),
				headerView("/weather/rpc", versionHeader(value)))
			require.NotNil(t, failure)
			assert.Equal(t, FailureVersionNotSupported, failure.Kind)
		})
	}
}

// A hostile value must cost a length comparison, not a match and an unbounded log
// line. The value is length-checked before the pattern runs, and whatever reaches
// the internal log is capped.
func TestA2AVersion_AnOverlongValueIsBoundedBeforeItIsMatchedOrLogged(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)
	huge := strings.Repeat("1", 64*1024)

	failure := pr.ValidateRequestHeaders(context.Background(),
		headerView("/weather/rpc", versionHeader(huge+".0")))
	require.NotNil(t, failure)
	assert.Equal(t, FailureInvalidParameter, failure.Kind)
	assert.Less(t, len(failure.Error()), 256,
		"a caller must not be able to write an unbounded line into the internal log")
}

// ─── Telemetry facts ─────────────────────────────────────────────────────────

// A rejection carries this route's transport and configured version, because the
// analytics policy that normally emits them lives in the chain this request never
// bound. It must not carry the version the caller stated: that value is unbounded and
// attacker-chosen, so it belongs in the internal log and nowhere that becomes an
// event dimension.
func TestA2AVersion_RejectionCarriesRouteFactsAndNeverTheStatedValue(t *testing.T) {
	cases := map[string]struct {
		route     *PreparedRoute
		view      HeaderRequestView
		transport agentproto.Transport
	}{
		"unsupported version": {
			route:     jsonRPCRoute(t, agentproto.V1_0),
			view:      headerView("/weather/rpc", versionHeader("0.3")),
			transport: agentproto.TransportJSONRPC,
		},
		"malformed version": {
			route:     getTaskRoute(t),
			view:      headerView("/weather/http/tasks/t-1", versionHeader("1.0.0")),
			transport: agentproto.TransportHTTPJSON,
		},
		"conflicting representations": {
			route:     sendMessageRoute(t),
			view:      headerView("/weather/http/message:send?A2A-Version=0.3", versionHeader("1.0")),
			transport: agentproto.TransportHTTPJSON,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			failure := tc.route.ValidateRequestHeaders(context.Background(), tc.view)
			require.NotNil(t, failure)
			assert.Equal(t, map[string]string{
				AttrA2ATransport:       string(tc.transport),
				AttrA2AProtocolVersion: "1.0",
			}, failure.Attributes)
		})
	}
}

// The attribute map travels out of the resolver into a span and an analytics event.
// A map built once at Prepare and shared would be one mutation away from a rejection
// reporting another route's binding, so each rejection allocates its own.
func TestA2AVersion_RejectionAttributesAreNotShared(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)
	view := headerView("/weather/rpc", versionHeader("0.3"))

	first := pr.ValidateRequestHeaders(context.Background(), view)
	second := pr.ValidateRequestHeaders(context.Background(), view)
	require.NotNil(t, first)
	require.NotNil(t, second)

	first.Attributes[AttrA2ATransport] = "tampered"
	assert.Equal(t, string(agentproto.TransportJSONRPC), second.Attributes[AttrA2ATransport])
}

// ─── Ordering: the guard runs before anything is read ────────────────────────

// The ordering regression the section exists to prevent. A wrong version paired with
// a body problem must be reported as the version problem: the guard runs first, so
// Resolve is never reached and the body is never asked for.
//
// The mirror case matters as much — a valid version must not mask a request error
// that follows it — so both directions are asserted against the same bodies.
func TestA2AVersion_HeaderValidationPrecedesEveryBodyOutcome(t *testing.T) {
	bodies := map[string][]byte{
		"a 0.3 method name": []byte(`{"jsonrpc":"2.0","id":1,"method":"message/send"}`),
		"an unknown method": jsonRPCBody("NotAnOperation"),
		"malformed JSON":    []byte(`{"jsonrpc":`),
		"a batch":           []byte(`[{"jsonrpc":"2.0","id":1,"method":"SendMessage"}]`),
		"no body at all":    nil,
	}
	wantWithValidVersion := map[string]FailureKind{
		"a 0.3 method name": FailureUnknownOperation,
		"an unknown method": FailureUnknownOperation,
		"malformed JSON":    FailureParse,
		"a batch":           FailureMultiOperation,
		"no body at all":    FailureInvalidRequest,
	}

	pr := jsonRPCRoute(t, agentproto.V1_0)

	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			// Wrong version: refused at the header phase, before the body is a
			// factor at all.
			failure := pr.ValidateRequestHeaders(context.Background(),
				headerView("/weather/rpc", versionHeader("0.3")))
			require.NotNil(t, failure)
			assert.Equal(t, FailureVersionNotSupported, failure.Kind)

			// Right version: the header phase passes and the body's own
			// classification is preserved exactly as Section 8 left it.
			require.Nil(t, pr.ValidateRequestHeaders(context.Background(),
				headerView("/weather/rpc", versionHeader("1.0"))))
			res, err := resolveJSONRPC(t, pr, body)
			requireFailure(t, res, err, wantWithValidVersion[name])
		})
	}
}

// A valid version leaves a good request entirely alone: the same chain key and the
// same attributes as before the guard existed.
func TestA2AVersion_ValidationDoesNotDisturbASuccessfulResolution(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	require.Nil(t, pr.ValidateRequestHeaders(context.Background(),
		headerView("/weather/rpc", versionHeader("1.0"))))

	res, err := resolveJSONRPC(t, pr, jsonRPCBody("SendMessage"))
	require.NoError(t, err)
	assert.Equal(t, ChainKeyFor(a2aAPIID, a2aVhost, "SendMessage"), res.ChainKey)
	assert.Equal(t, "SendMessage", res.Attributes[AttrA2AOperation])
}

// ─── Concurrency ─────────────────────────────────────────────────────────────

// One prepared resolver serves every request on its route, so the guard is shared
// state by construction. It holds only values fixed at Prepare and allocates its
// failures, which is what makes that safe — asserted under -race rather than argued.
func TestA2AVersion_OnePreparedResolverServesConcurrentRequests(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	views := []struct {
		view HeaderRequestView
		want FailureKind // "" means the request must be accepted
	}{
		{headerView("/weather/rpc", versionHeader("1.0")), ""},
		{headerView("/weather/rpc?A2A-Version=1.0", nil), ""},
		{headerView("/weather/rpc?A2A-Version=1.0", versionHeader("1.0")), ""},
		{headerView("/weather/rpc", nil), FailureVersionNotSupported},
		{headerView("/weather/rpc", versionHeader("1.0.0")), FailureInvalidParameter},
		{headerView("/weather/rpc?A2A-Version=0.3", versionHeader("1.0")), FailureConflictingParameter},
	}

	var wg sync.WaitGroup
	for range 32 {
		for _, tc := range views {
			wg.Go(func() {
				failure := pr.ValidateRequestHeaders(context.Background(), tc.view)
				if tc.want == "" {
					assert.Nil(t, failure)
					return
				}
				if assert.NotNil(t, failure) {
					assert.Equal(t, tc.want, failure.Kind)
				}
			})
		}
	}
	wg.Wait()
}

// ─── HeaderMap ───────────────────────────────────────────────────────────────

// The view is read-only and multi-value. Every value is returned rather than the
// first, because "was this field repeated" is itself something the guard rejects on
// and collapsing repeats here would hide it.
func TestHeaderMap_ReturnsEveryValueAndFoldsCase(t *testing.T) {
	h := NewHeaderMap(map[string][]string{
		A2AVersionHeader: {"1.0", "0.3"},
		"Content-Type":   {"application/json"},
	})

	assert.Equal(t, []string{"1.0", "0.3"}, h.Values(A2AVersionHeader))
	assert.Equal(t, []string{"application/json"}, h.Values("content-type"))
	assert.Nil(t, h.Values("accept"))
	assert.Nil(t, NewHeaderMap(nil).Values(A2AVersionHeader))
}

// A caller that built the map without normalising must not be able to smuggle a
// second value past the duplicate check by spelling the name differently.
func TestHeaderMap_UnnormalisedSpellingsAreStillSeenAsRepeats(t *testing.T) {
	h := NewHeaderMap(map[string][]string{
		"A2A-Version": {"1.0"},
		"a2a-VERSION": {"0.3"},
	})

	assert.Len(t, h.Values(A2AVersionHeader), 2)
}

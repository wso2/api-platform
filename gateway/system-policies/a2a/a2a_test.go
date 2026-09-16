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

package a2a

import (
	"bytes"
	"context"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

const (
	testCard = `{"name":"weather","protocolVersion":"1.0"}`
	testETag = `"c0ffee"`
)

// run calls the policy with the given request headers and the standard card
// parameters, and asserts the chain was terminated.
func run(t *testing.T, headers map[string][]string, params map[string]any) policy.ImmediateResponse {
	t.Helper()
	action := ins.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{APIId: "agent-uuid-1"},
		Headers:       policy.NewHeaders(headers),
		Method:        "GET",
		Path:          "/weather/.well-known/agent-card.json",
	}, params)

	immediate, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("expected the card policy to terminate the chain, got %T", action)
	}
	return immediate
}

// cardBlockParams wraps an agentCard block in the policy's top-level parameter
// map, which is the shape the controller writes.
func cardBlockParams(block map[string]any) map[string]any {
	return map[string]any{ParamAgentCard: block}
}

func cardParams() map[string]any {
	return cardBlockParams(map[string]any{ParamContent: testCard, ParamETag: testETag})
}

// The card policy must never let a card request through to the upstream: it is
// attached only to a managed card route, where forwarding would serve the
// upstream's own unvalidated card in place of the one the gateway owns.
func TestCardRequestAlwaysTerminatesTheChain(t *testing.T) {
	if !(policy.ImmediateResponse{}).StopExecution() {
		t.Fatal("ImmediateResponse must stop chain execution")
	}
	response := run(t, nil, cardParams())
	if response.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
}

func TestServesTheConfiguredCardBytesVerbatim(t *testing.T) {
	response := run(t, nil, cardParams())

	if got := string(response.Body); got != testCard {
		t.Errorf("body = %q, want %q", got, testCard)
	}
	if got := response.Headers["content-type"]; got != contentTypeJSON {
		t.Errorf("content-type = %q, want %q", got, contentTypeJSON)
	}
	if got := response.Headers["etag"]; got != testETag {
		t.Errorf("etag = %q, want %q", got, testETag)
	}
	if got := response.Headers["cache-control"]; got != cacheControlValue {
		t.Errorf("cache-control = %q, want %q", got, cacheControlValue)
	}
}

// A 304 carries the validator and the caching directives but no body — a client
// refreshing a stored copy must see the same metadata the full response carried.
func TestMatchingIfNoneMatchReturns304WithNoBody(t *testing.T) {
	response := run(t, map[string][]string{"If-None-Match": {testETag}}, cardParams())

	if response.StatusCode != 304 {
		t.Fatalf("status = %d, want 304", response.StatusCode)
	}
	if len(response.Body) != 0 {
		t.Errorf("304 carried a %d-byte body; it must carry none", len(response.Body))
	}
	if got := response.Headers["etag"]; got != testETag {
		t.Errorf("etag = %q, want %q", got, testETag)
	}
	if got := response.Headers["cache-control"]; got != cacheControlValue {
		t.Errorf("cache-control = %q, want %q", got, cacheControlValue)
	}
}

// If-None-Match uses weak comparison whatever the tags involved, may repeat, and
// may carry a list per occurrence. A tag for some other representation must not
// produce a 304 — that would serve the client a card it does not have.
func TestIfNoneMatchComparison(t *testing.T) {
	cases := []struct {
		name    string
		values  []string
		matches bool
	}{
		{"exact", []string{testETag}, true},
		{"weak candidate against strong tag", []string{`W/"c0ffee"`}, true},
		{"wildcard", []string{"*"}, true},
		{"one of a list", []string{`"other", "c0ffee"`}, true},
		{"second occurrence", []string{`"other"`, testETag}, true},
		{"list with spaces and empty entries", []string{`  ,  "c0ffee"  `}, true},
		{"different tag", []string{`"decaf"`}, false},
		{"unquoted tag", []string{"c0ffee"}, false},
		{"substring of the tag", []string{`"c0ff"`}, false},
		{"empty header", []string{""}, false},
		{"absent header", nil, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := ifNoneMatchSatisfied(testCase.values, testETag); got != testCase.matches {
				t.Errorf("ifNoneMatchSatisfied(%q, %q) = %v, want %v",
					testCase.values, testETag, got, testCase.matches)
			}
		})
	}
}

// A card with no entity tag still serves. The ETag header is omitted rather than
// sent empty, and no If-None-Match can then produce a 304 — including "*", which
// would otherwise 304 a client that holds nothing.
func TestCardWithoutAnETagServesWithoutOne(t *testing.T) {
	params := cardBlockParams(map[string]any{ParamContent: testCard})

	response := run(t, map[string][]string{"If-None-Match": {"*"}}, params)
	if response.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if _, present := response.Headers["etag"]; present {
		t.Error("an ETag header was sent for a card that has no entity tag")
	}
}

// A chain built without the card this policy exists to serve is a controller
// defect. It must fail closed rather than fall through to the upstream, and the
// body must not explain why (error-handling.md directive 1).
func TestMissingOrUnusableContentFailsClosed(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
	}{
		{"nil params", nil},
		{"no agentCard block", map[string]any{}},
		{"agentCard block is not an object", map[string]any{ParamAgentCard: testCard}},
		{"no content parameter", cardBlockParams(map[string]any{ParamETag: testETag})},
		{"empty content", cardBlockParams(map[string]any{ParamContent: ""})},
		{"content is not a string", cardBlockParams(map[string]any{
			ParamContent: map[string]any{"name": "weather"},
		})},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response := run(t, nil, testCase.params)
			if response.StatusCode != 500 {
				t.Fatalf("status = %d, want 500", response.StatusCode)
			}
			if body := string(response.Body); body != `{"error":"unavailable","message":"The Agent Card could not be served."}` {
				t.Errorf("failure body is not the sterile payload: %q", body)
			}
		})
	}
}

// A non-string etag is ignored rather than coerced: an ETag header built from a
// stringified map would be a validator no client could ever match, and 304
// decisions would be made against it.
func TestNonStringETagIsIgnored(t *testing.T) {
	params := cardBlockParams(map[string]any{ParamContent: testCard, ParamETag: 42})

	response := run(t, nil, params)
	if response.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if _, present := response.Headers["etag"]; present {
		t.Error("an ETag header was built from a non-string parameter")
	}
}

// The request body is declared because one job needs it — the JSON-RPC binding
// of a managed protected card, which has to echo the caller's request id, and
// that id is nowhere else. Neither response phase has anything to inspect: every
// response this policy produces, it produces itself.
func TestModeParticipatesInBothRequestPhasesOnly(t *testing.T) {
	mode := ins.Mode()

	if mode.RequestHeaderMode != policy.HeaderModeProcess {
		t.Errorf("RequestHeaderMode = %q, want %q", mode.RequestHeaderMode, policy.HeaderModeProcess)
	}
	if mode.RequestBodyMode != policy.BodyModeBuffer {
		t.Errorf("RequestBodyMode = %q, want %q", mode.RequestBodyMode, policy.BodyModeBuffer)
	}
	if mode.ResponseHeaderMode != policy.HeaderModeSkip {
		t.Errorf("ResponseHeaderMode = %q, want %q", mode.ResponseHeaderMode, policy.HeaderModeSkip)
	}
	if mode.ResponseBodyMode != policy.BodyModeSkip {
		t.Errorf("ResponseBodyMode = %q, want %q", mode.ResponseBodyMode, policy.BodyModeSkip)
	}
}

// The parameter names are the contract with the gateway controller's Agent
// transformer, which writes them from a separate Go module and so cannot share
// these constants. Renaming one here without renaming it there would leave a
// managed card route serving a 500.
func TestParameterNamesMatchTheControllerContract(t *testing.T) {
	for _, testCase := range []struct{ got, want string }{
		{ParamAgentCard, "agentCard"},
		{ParamContent, "content"},
		{ParamETag, "etag"},
		{ParamProtectedAgentCard, "protectedAgentCard"},
	} {
		if testCase.got != testCase.want {
			t.Errorf("parameter name = %q, want %q", testCase.got, testCase.want)
		}
	}
}

// The resolution attribute names are the contract with the a2a resolver in the
// policy engine, which spells them in an internal package of another module. A
// rename on one side alone is silent: the transport reads back empty and every
// managed protected card request answers 500. The engine's own pinning
// assertion is TestA2AAttributeSpellingsArePinned in internal/resolver.
func TestResolutionAttributeSpellingsMatchThePolicyEngine(t *testing.T) {
	for _, testCase := range []struct{ got, want string }{
		{attrA2ATransport, "a2a.transport"},
		{transportJSONRPC, "JSONRPC"},
		{transportHTTPJSON, "HTTP+JSON"},
	} {
		if testCase.got != testCase.want {
			t.Errorf("got %q, want %q", testCase.got, testCase.want)
		}
	}
}

func TestGetPolicyReturnsAUsableInstance(t *testing.T) {
	for name, factory := range map[string]policy.PolicyFactory{
		"GetPolicy":   GetPolicy,
		"GetPolicyV2": GetPolicyV2,
	} {
		t.Run(name, func(t *testing.T) {
			instance, err := factory(policy.PolicyMetadata{}, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, ok := instance.(policy.RequestHeaderPolicy); !ok {
				t.Fatalf("instance %T does not implement RequestHeaderPolicy", instance)
			}
			if _, ok := instance.(policy.RequestPolicy); !ok {
				t.Fatalf("instance %T does not implement RequestPolicy", instance)
			}
		})
	}
}

// ─── Protected (extended) Agent Card ─────────────────────────────────────────

const (
	testProtectedCard = `{"name":"weather","protocolVersion":"1.0","skills":[{"id":"book"}]}`
	sterile401        = `{"error":"unauthorized","message":"Invalid or expired credentials."}`
	sterile400        = `{"error":"invalid_request","message":"The request could not be processed."}`
)

// protectedParams is the shape the controller writes for a managed protected
// card. A passthrough card gets the same block with no content.
func protectedParams(content string) map[string]any {
	block := map[string]any{}
	if content != "" {
		block[ParamContent] = content
	}
	return map[string]any{ParamProtectedAgentCard: block}
}

// runBody calls the body phase the way the kernel does: with the transport the
// resolver recorded when it selected this chain, and with whatever authentication
// the author's policies established.
func runBody(
	t *testing.T,
	transport string,
	authenticated bool,
	body string,
	params map[string]any,
) policy.RequestAction {
	t.Helper()

	shared := &policy.SharedContext{
		APIId: "agent-uuid-1",
		ResolutionAttributes: policy.NewResolutionAttributes(map[string]string{
			"a2a.transport": transport,
			"a2a.operation": "GetExtendedAgentCard",
		}),
	}
	if authenticated {
		shared.AuthContext = &policy.AuthContext{Authenticated: true, Subject: "alice"}
	}

	return ins.OnRequestBody(context.Background(), &policy.RequestContext{
		SharedContext: shared,
		Headers:       policy.NewHeaders(nil),
		Body:          &policy.Body{Content: []byte(body), Present: body != "", EndOfStream: true},
		Path:          "/weather/v1/extendedAgentCard",
	}, params)
}

func requireImmediate(t *testing.T, action policy.RequestAction) policy.ImmediateResponse {
	t.Helper()
	immediate, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("expected the chain to be terminated, got %T", action)
	}
	return immediate
}

// The protected card is protected by this check and by nothing else. An Agent
// that configured one but attached no authentication must answer 401 rather than
// publish its extended card — in managed mode, where the bytes are right here,
// and in passthrough mode, where forwarding would have the upstream publish them.
func TestProtectedCardWithoutAuthenticationIsRefusedInBothModes(t *testing.T) {
	for _, mode := range []struct {
		name    string
		content string
	}{
		{"managed", testProtectedCard},
		{"passthrough", ""},
	} {
		for _, transport := range []string{transportJSONRPC, transportHTTPJSON} {
			t.Run(mode.name+"/"+transport, func(t *testing.T) {
				response := requireImmediate(t, runBody(t, transport, false,
					`{"jsonrpc":"2.0","id":1,"method":"GetExtendedAgentCard"}`,
					protectedParams(mode.content)))

				if response.StatusCode != 401 {
					t.Fatalf("status = %d, want 401", response.StatusCode)
				}
				if body := string(response.Body); body != sterile401 {
					t.Errorf("failure body is not the sterile payload: %q", body)
				}
				if got := response.Headers["cache-control"]; got != cacheControlNoStore {
					t.Errorf("cache-control = %q, want %q", got, cacheControlNoStore)
				}
				if bytes.Contains(response.Body, []byte("skills")) {
					t.Error("the refusal carried card bytes")
				}
			})
		}
	}
}

// A missing SharedContext is not "no opinion about authentication": there is no
// evidence the request was authenticated, so it is refused like any other.
func TestProtectedCardWithoutSharedContextIsRefused(t *testing.T) {
	action := ins.OnRequestBody(context.Background(), &policy.RequestContext{
		Headers: policy.NewHeaders(nil),
	}, protectedParams(testProtectedCard))

	if response := requireImmediate(t, action); response.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
}

// HTTP+JSON returns the Agent Card as the response body itself, uncached.
func TestManagedProtectedCardOnHTTPJSONServesTheBareCard(t *testing.T) {
	response := requireImmediate(t, runBody(t, transportHTTPJSON, true, "",
		protectedParams(testProtectedCard)))

	if response.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if got := string(response.Body); got != testProtectedCard {
		t.Errorf("body = %q, want the configured card verbatim", got)
	}
	if got := response.Headers["content-type"]; got != contentTypeJSON {
		t.Errorf("content-type = %q, want %q", got, contentTypeJSON)
	}
	// An authenticated response held by a shared cache would be one principal's
	// card returned for another principal's request.
	if got := response.Headers["cache-control"]; got != cacheControlNoStore {
		t.Errorf("cache-control = %q, want %q", got, cacheControlNoStore)
	}
	if _, present := response.Headers["etag"]; present {
		t.Error("a protected card response carries no validator")
	}
}

// JSON-RPC wraps the same bytes in a result envelope and echoes the request id
// as the JSON value it arrived as. A client correlates its requests on that
// value, so a number that comes back a string is a broken client, silently.
func TestManagedProtectedCardOnJSONRPCEchoesTheRequestID(t *testing.T) {
	for _, testCase := range []struct{ name, id string }{
		{"number", `7`},
		{"large number", `9007199254740993`},
		{"string", `"req-1"`},
		{"empty string", `""`},
		{"null", `null`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			body := `{"jsonrpc":"2.0","id":` + testCase.id + `,"method":"GetExtendedAgentCard","params":{}}`
			response := requireImmediate(t, runBody(t, transportJSONRPC, true, body,
				protectedParams(testProtectedCard)))

			if response.StatusCode != 200 {
				t.Fatalf("status = %d, want 200", response.StatusCode)
			}
			want := `{"jsonrpc":"2.0","id":` + testCase.id + `,"result":` + testProtectedCard + `}`
			if got := string(response.Body); got != want {
				t.Errorf("body  = %s\nwant = %s", got, want)
			}
			if got := response.Headers["cache-control"]; got != cacheControlNoStore {
				t.Errorf("cache-control = %q, want %q", got, cacheControlNoStore)
			}
		})
	}
}

// A JSON-RPC request with no id is a notification: the caller has said it wants
// no reply and has no id to correlate one against. 204 says the same at the HTTP
// level, and carries no card.
func TestJSONRPCNotificationGetsNoBody(t *testing.T) {
	response := requireImmediate(t, runBody(t, transportJSONRPC, true,
		`{"jsonrpc":"2.0","method":"GetExtendedAgentCard","params":{}}`,
		protectedParams(testProtectedCard)))

	if response.StatusCode != 204 {
		t.Fatalf("status = %d, want 204", response.StatusCode)
	}
	if len(response.Body) != 0 {
		t.Errorf("a 204 carried a body: %q", response.Body)
	}
}

// JSON-RPC 2.0 allows an id to be a string, a number, or null. Anything else has
// no correct echo, so it fails closed rather than being coerced into one.
func TestUnusableJSONRPCIDFailsClosed(t *testing.T) {
	for _, testCase := range []struct{ name, body string }{
		{"object id", `{"jsonrpc":"2.0","id":{"a":1},"method":"GetExtendedAgentCard"}`},
		{"array id", `{"jsonrpc":"2.0","id":[1],"method":"GetExtendedAgentCard"}`},
		{"boolean id", `{"jsonrpc":"2.0","id":true,"method":"GetExtendedAgentCard"}`},
		{"malformed envelope", `{"jsonrpc":"2.0",`},
		{"empty body", ``},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := requireImmediate(t, runBody(t, transportJSONRPC, true, testCase.body,
				protectedParams(testProtectedCard)))

			if response.StatusCode != 400 {
				t.Fatalf("status = %d, want 400", response.StatusCode)
			}
			if body := string(response.Body); body != sterile400 {
				t.Errorf("failure body is not the sterile payload: %q", body)
			}
			if bytes.Contains(response.Body, []byte("skills")) {
				t.Error("the refusal carried card bytes")
			}
		})
	}
}

// Passthrough carries no card. Once the request is authenticated it goes on to
// the upstream, whose own extended card is proxied unparsed.
func TestPassthroughProtectedCardForwardsOnceAuthenticated(t *testing.T) {
	for _, transport := range []string{transportJSONRPC, transportHTTPJSON} {
		t.Run(transport, func(t *testing.T) {
			action := runBody(t, transport, true,
				`{"jsonrpc":"2.0","id":1,"method":"GetExtendedAgentCard"}`, protectedParams(""))

			if _, ok := action.(policy.UpstreamRequestModifications); !ok {
				t.Fatalf("expected the request to be forwarded, got %T", action)
			}
			if action.StopExecution() {
				t.Error("a passthrough protected card must not terminate the chain")
			}
		})
	}
}

// A managed card whose chain lost the transport attribute has nowhere correct to
// put the card. Forwarding instead would serve the upstream's own unvalidated
// extended card under a configuration that says the gateway owns it, so it fails
// closed.
func TestManagedProtectedCardWithoutAKnownTransportFailsClosed(t *testing.T) {
	for _, transport := range []string{"", "GRPC"} {
		t.Run("transport="+transport, func(t *testing.T) {
			response := requireImmediate(t, runBody(t, transport, true, "",
				protectedParams(testProtectedCard)))

			if response.StatusCode != 500 {
				t.Fatalf("status = %d, want 500", response.StatusCode)
			}
		})
	}
}

// The protected block does its work at the body phase, so the header phase must
// let the request past — that is what lets the author's own policies, header and
// body alike, decide it first.
func TestProtectedCardContinuesAtTheHeaderPhase(t *testing.T) {
	action := ins.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{APIId: "agent-uuid-1"},
		Headers:       policy.NewHeaders(nil),
		Method:        "GET",
		Path:          "/weather/v1/extendedAgentCard",
	}, protectedParams(testProtectedCard))

	if _, ok := action.(policy.UpstreamRequestHeaderModifications); !ok {
		t.Fatalf("expected the header phase to continue, got %T", action)
	}
}

// An instance holding no block this policy recognises is a chain the controller
// built wrong. It must fail closed rather than forward: a block that went missing
// would otherwise become a bypass of whatever that instance was there to do.
func TestUnrecognisedParameterBlockFailsClosed(t *testing.T) {
	response := run(t, nil, map[string]any{"somethingElse": map[string]any{}})
	if response.StatusCode != 500 {
		t.Fatalf("status = %d, want 500", response.StatusCode)
	}
}

// The public card instance terminated the chain in the header phase, so it never
// reaches the body phase — and any other instance must pass through it rather
// than answer for a job that is not its own.
func TestBodyPhaseIgnoresEveryOtherBlock(t *testing.T) {
	for name, params := range map[string]map[string]any{
		"public card block": cardParams(),
		"no block at all":   {},
		"nil params":        nil,
	} {
		t.Run(name, func(t *testing.T) {
			action := runBody(t, transportHTTPJSON, true, "", params)
			if _, ok := action.(policy.UpstreamRequestModifications); !ok {
				t.Fatalf("expected the body phase to continue, got %T", action)
			}
		})
	}
}

// A protected-card block the controller mangled must not be read as passthrough.
//
// "Passthrough" is encoded by content's *absence*, so a block that cannot be read
// looks exactly like one — and guessing wrong here proxies the operation, serving
// the upstream's own unvalidated extended card under a configuration that says
// the gateway owns it. That substitution is silent, so every unreadable shape
// fails closed instead.
func TestMalformedProtectedCardBlockIsNotReadAsPassthrough(t *testing.T) {
	for name, params := range map[string]map[string]any{
		"block is not an object": {ParamProtectedAgentCard: testProtectedCard},
		"content is not a string": {ParamProtectedAgentCard: map[string]any{
			ParamContent: map[string]any{"name": "weather"},
		}},
		"content is empty": {ParamProtectedAgentCard: map[string]any{ParamContent: ""}},
	} {
		t.Run(name, func(t *testing.T) {
			action := runBody(t, transportHTTPJSON, true, "", params)
			if _, ok := action.(policy.UpstreamRequestModifications); ok {
				t.Fatal("an unreadable protected-card block was forwarded to the upstream")
			}
			response := requireImmediate(t, action)
			if response.StatusCode != 500 {
				t.Fatalf("status = %d, want 500", response.StatusCode)
			}
		})
	}
}

// The authentication check still comes first: an unreadable block is a
// controller defect, but an anonymous caller learns nothing about it.
func TestMalformedProtectedCardBlockStillRefusesAnUnauthenticatedCaller(t *testing.T) {
	response := requireImmediate(t, runBody(t, transportHTTPJSON, false, "",
		map[string]any{ParamProtectedAgentCard: testProtectedCard}))

	if response.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
}

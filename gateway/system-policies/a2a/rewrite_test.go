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
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// upstreamCard is a passthrough Agent Card as the agent behind the gateway
// serves it: it advertises the agent's own URLs, on both bindings, and carries a
// signature over those bytes plus an extension field.
//
// Both are load-bearing. The signature covers a document the gateway is about to
// change, so it has to be gone from the rewritten card; the extension field is
// the thing a rewrite must not disturb, and it is the one an author would notice
// missing.
const upstreamCard = `{
  "name": "Trip Planner",
  "protocolVersion": "1.0",
  "supportedInterfaces": [
    {"protocolBinding": "JSONRPC", "protocolVersion": "1.0", "url": "https://agent.internal/"},
    {"protocolBinding": "HTTP+JSON", "protocolVersion": "1.0", "url": "https://agent.internal/v1"}
  ],
  "capabilities": {"streaming": true, "extendedAgentCard": true},
  "signatures": [{"protected": "eyJhbGciOiJFUzI1NiJ9", "signature": "c2ln"}],
  "x-vendor-metadata": {"team": "trips", "regions": ["eu", "us"]}
}`

// rewriteMapping is the interface mapping the controller writes for an Agent at
// context /trips with JSON-RPC at /rpc and HTTP+JSON at /v1.
func rewriteMapping() map[string]any {
	return map[string]any{
		ParamProtocolVersion: "1.0",
		ParamInterfaces: []any{
			map[string]any{ParamProtocolBinding: transportJSONRPC, ParamPath: "/trips/rpc"},
			map[string]any{ParamProtocolBinding: transportHTTPJSON, ParamPath: "/trips/v1"},
		},
	}
}

// rewritingPolicy builds the instance the controller's parameters produce, going
// through GetPolicy rather than constructing the struct — the rewrite
// configuration is parsed there, and Mode() is read off the result, so a test
// that built the struct directly would not exercise either.
func rewritingPolicy(t *testing.T, params map[string]any) *A2ASystemPolicy {
	t.Helper()
	instance, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}
	typed, ok := instance.(*A2ASystemPolicy)
	if !ok {
		t.Fatalf("GetPolicy returned %T", instance)
	}
	return typed
}

// publicRewriteParams is what the controller writes for a passthrough public
// card that opted into rewriting: the mapping, and no document.
func publicRewriteParams() map[string]any {
	return cardBlockParams(map[string]any{ParamRewriteUrls: rewriteMapping()})
}

// protectedRewriteParams is the same for an explicit protected passthrough card.
func protectedRewriteParams() map[string]any {
	return map[string]any{
		ParamProtectedAgentCard: map[string]any{ParamRewriteUrls: rewriteMapping()},
	}
}

// runResponse calls the response phase the way the kernel does: with the
// downstream request snapshot captured before any policy mutated it, the
// transport the resolver recorded, and the upstream's decompressed body.
func runResponse(
	t *testing.T,
	instance *A2ASystemPolicy,
	params map[string]any,
	scheme, authority, transport string,
	status int,
	body string,
) policy.ResponseAction {
	t.Helper()

	shared := &policy.SharedContext{
		APIId: "agent-uuid-1",
		ResolutionAttributes: policy.NewResolutionAttributes(map[string]string{
			"a2a.transport": transport,
		}),
	}

	var responseBody *policy.Body
	if body != "" {
		responseBody = &policy.Body{Content: []byte(body), Present: true, EndOfStream: true}
	}

	return instance.OnResponseBody(context.Background(), &policy.ResponseContext{
		SharedContext:   shared,
		RequestPath:     "/trips/.well-known/agent-card.json",
		RequestMethod:   "GET",
		ResponseHeaders: policy.NewHeaders(nil),
		ResponseBody:    responseBody,
		ResponseStatus:  status,
		Downstream: &policy.DownstreamContext{Request: &policy.DownstreamRequest{
			Headers:   policy.NewHeaders(nil),
			Path:      "/trips/.well-known/agent-card.json",
			Method:    "GET",
			Scheme:    scheme,
			Authority: authority,
		}},
	}, params)
}

func requireModifications(t *testing.T, action policy.ResponseAction) policy.DownstreamResponseModifications {
	t.Helper()
	mods, ok := action.(policy.DownstreamResponseModifications)
	if !ok {
		t.Fatalf("expected the response to be forwarded with modifications, got %T", action)
	}
	return mods
}

func requireImmediateResponse(t *testing.T, action policy.ResponseAction) policy.ImmediateResponse {
	t.Helper()
	immediate, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("expected the response to be replaced, got %T", action)
	}
	return immediate
}

// decodeCard reads a rewritten card back as a generic document, failing the test
// if it is not valid JSON — a rewrite that produced unparseable bytes would
// otherwise show up as a confusing assertion failure several lines later.
func decodeCard(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatalf("rewritten card is not valid JSON: %v\n%s", err, body)
	}
	return document
}

// interfaceURLs is the advertised URL per protocol binding.
func interfaceURLs(t *testing.T, card map[string]any) map[string]string {
	t.Helper()
	entries, ok := card[cardFieldSupportedInterfaces].([]any)
	if !ok {
		t.Fatalf("card advertises no interface list: %#v", card[cardFieldSupportedInterfaces])
	}
	urls := map[string]string{}
	for _, entry := range entries {
		iface, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("interface entry is not an object: %#v", entry)
		}
		binding, _ := iface[cardFieldProtocolBinding].(string)
		url, _ := iface[cardFieldURL].(string)
		urls[binding] = url
	}
	return urls
}

// ─── The rewrite itself ──────────────────────────────────────────────────────

// The scheme a rewritten card advertises is the scheme of the *downstream*
// request, not of the gateway's own connection to the agent.
//
// A gateway terminating TLS in front of a plaintext agent is the ordinary
// deployment, and reading the upstream leg would publish http:// URLs to an
// https:// client — a card that fails to work for every client that fetched it
// over TLS. The reverse case is a plaintext dev gateway in front of an https
// agent, and it fails just as silently in the other direction.
func TestRewriteAdvertisesTheDownstreamRequestScheme(t *testing.T) {
	instance := rewritingPolicy(t, publicRewriteParams())

	for _, scheme := range []string{schemeHTTP, schemeHTTPS} {
		t.Run(scheme, func(t *testing.T) {
			mods := requireModifications(t, runResponse(t, instance, publicRewriteParams(),
				scheme, "agents.example.com:9095", "", 200, upstreamCard))

			urls := interfaceURLs(t, decodeCard(t, mods.Body))
			// The authority is the one the client dialled, port included: no static
			// configuration knows which port a client reached the gateway on.
			if got, want := urls[transportJSONRPC], scheme+"://agents.example.com:9095/trips/rpc"; got != want {
				t.Errorf("JSONRPC url = %q, want %q", got, want)
			}
			if got, want := urls[transportHTTPJSON], scheme+"://agents.example.com:9095/trips/v1"; got != want {
				t.Errorf("HTTP+JSON url = %q, want %q", got, want)
			}
		})
	}
}

// Every advertised interface is rewritten, and the entry's binding — not its
// position — selects the gateway endpoint.
//
// Position would be the easy thing to use and the wrong one: a card lists its
// interfaces in whatever order its author wrote them, so an Agent whose
// transports happen to be configured the other way round would advertise the
// JSON-RPC endpoint as its HTTP+JSON one. Both requests then reach a route that
// exists, with the wrong protocol, which is the worst shape of failure available
// here.
func TestRewriteSelectsTheEndpointByBindingNotByPosition(t *testing.T) {
	reversed := `{
	  "name": "Trip Planner",
	  "supportedInterfaces": [
	    {"protocolBinding": "HTTP+JSON", "url": "https://agent.internal/v1"},
	    {"protocolBinding": "JSONRPC", "url": "https://agent.internal/"}
	  ]
	}`

	instance := rewritingPolicy(t, publicRewriteParams())
	mods := requireModifications(t, runResponse(t, instance, publicRewriteParams(),
		schemeHTTPS, "agents.example.com", "", 200, reversed))

	urls := interfaceURLs(t, decodeCard(t, mods.Body))
	if got, want := urls[transportJSONRPC], "https://agents.example.com/trips/rpc"; got != want {
		t.Errorf("JSONRPC url = %q, want %q", got, want)
	}
	if got, want := urls[transportHTTPJSON], "https://agents.example.com/trips/v1"; got != want {
		t.Errorf("HTTP+JSON url = %q, want %q", got, want)
	}
}

// ─── Which host a rewritten URL advertises ───────────────────────────────────

// The Agent's configured virtual host decides the host, and the request decides
// the port.
//
// The vhost is the name the Agent was published under. The authority a client
// dialled may be an internal one — a cluster service name, a pod address, the
// hostname of an ingress in front of the gateway — and a public discovery
// document advertising that tells every other client to use it too. The port is
// the other way round: a vhost is a routing match and names none, while the
// client that got here has already demonstrated which port works.
func TestRewriteAdvertisesTheConfiguredVhostWithTheRequestPort(t *testing.T) {
	for name, testCase := range map[string]struct {
		vhost, authority, scheme, want string
	}{
		"vhost replaces the dialled host": {
			vhost: "agents.example.com", authority: "gateway.internal.svc:8080",
			scheme: schemeHTTP, want: "http://agents.example.com:8080/trips/v1",
		},
		"a standard port is left off": {
			vhost: "agents.example.com", authority: "gateway.internal.svc:443",
			scheme: schemeHTTPS, want: "https://agents.example.com/trips/v1",
		},
		"an authority with no port yields none": {
			vhost: "agents.example.com", authority: "gateway.internal.svc",
			scheme: schemeHTTPS, want: "https://agents.example.com/trips/v1",
		},
		// A wildcard is a matching pattern, not a host a client can dial, so the
		// request's own authority is used whole — host and port.
		"a wildcard vhost defers to the request": {
			vhost: "*", authority: "agents.example.com:9095",
			scheme: schemeHTTPS, want: "https://agents.example.com:9095/trips/v1",
		},
		"a wildcard subdomain defers too": {
			vhost: "*.example.com", authority: "eu.example.com:9095",
			scheme: schemeHTTPS, want: "https://eu.example.com:9095/trips/v1",
		},
		// An IPv6 authority has colons in its host, so the port must be split off
		// the bracketed form rather than at the last colon.
		"an IPv6 authority keeps its port": {
			vhost: "agents.example.com", authority: "[2001:db8::1]:8443",
			scheme: schemeHTTPS, want: "https://agents.example.com:8443/trips/v1",
		},
	} {
		t.Run(name, func(t *testing.T) {
			params := publicRewriteParams()
			instance := rewritingPolicy(t, params)

			mods := requireModifications(t, runCardRequestThenResponse(t, instance, params,
				testCase.vhost, testCase.scheme, testCase.authority, "", 200, upstreamCard))

			urls := interfaceURLs(t, decodeCard(t, mods.Body))
			if got := urls[transportHTTPJSON]; got != testCase.want {
				t.Errorf("HTTP+JSON url = %q, want %q", got, testCase.want)
			}
		})
	}
}

// The vhost reaches the response phase only because the request phase put it
// there, so the two halves are asserted together rather than through
// gatewayAuthority alone.
//
// A ResponseContext carries no vhost of its own — only the shared context and
// the downstream request snapshot — so a rewrite that never ran its request
// phase would silently fall back to the dialled authority. Running the phases in
// order is what makes that regression visible.
func runCardRequestThenResponse(
	t *testing.T,
	instance *A2ASystemPolicy,
	params map[string]any,
	vhost, scheme, authority, transport string,
	status int,
	body string,
) policy.ResponseAction {
	t.Helper()

	shared := &policy.SharedContext{
		APIId:    "agent-uuid-1",
		Metadata: map[string]any{},
		ResolutionAttributes: policy.NewResolutionAttributes(map[string]string{
			attrA2ATransport: transport,
		}),
	}

	instance.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: shared,
		Headers:       policy.NewHeaders(nil),
		Method:        "GET",
		Path:          "/trips/.well-known/agent-card.json",
		Authority:     authority,
		Scheme:        scheme,
		Vhost:         vhost,
	}, params)

	var responseBody *policy.Body
	if body != "" {
		responseBody = &policy.Body{Content: []byte(body), Present: true, EndOfStream: true}
	}

	return instance.OnResponseBody(context.Background(), &policy.ResponseContext{
		SharedContext:   shared,
		RequestPath:     "/trips/.well-known/agent-card.json",
		RequestMethod:   "GET",
		ResponseHeaders: policy.NewHeaders(nil),
		ResponseBody:    responseBody,
		ResponseStatus:  status,
		Downstream: &policy.DownstreamContext{Request: &policy.DownstreamRequest{
			Headers:   policy.NewHeaders(nil),
			Path:      "/trips/.well-known/agent-card.json",
			Method:    "GET",
			Scheme:    scheme,
			Authority: authority,
		}},
	}, params)
}

// A protected card's rewrite gets the vhost from the same request phase, even
// though it decides nothing there.
//
// Its request-header phase exists only to suppress conditional headers before
// forwarding — the operation itself is answered at the body phase — so it is the
// representation where the recording step is easiest to leave out.
func TestProtectedRewriteAdvertisesTheConfiguredVhost(t *testing.T) {
	params := protectedRewriteParams()
	instance := rewritingPolicy(t, params)

	body := `{"jsonrpc":"2.0","id":1,"result":` + upstreamCard + `}`
	mods := requireModifications(t, runCardRequestThenResponse(t, instance, params,
		"agents.example.com", schemeHTTPS, "gateway.internal.svc:8443",
		transportJSONRPC, 200, body))

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(mods.Body, &envelope); err != nil {
		t.Fatalf("rewritten envelope is not JSON: %v", err)
	}
	urls := interfaceURLs(t, decodeCard(t, envelope[jsonRPCFieldResult]))
	if got, want := urls[transportJSONRPC], "https://agents.example.com:8443/trips/rpc"; got != want {
		t.Errorf("JSONRPC url = %q, want %q", got, want)
	}
}

// A non-rewriting instance records nothing, so the key never appears for the
// managed cards and plain passthrough routes that make up most Agents.
func TestOnlyARewritingInstanceRecordsTheVhost(t *testing.T) {
	params := cardBlockParams(map[string]any{
		ParamContent: `{"name":"Trip Planner"}`,
		ParamETag:    `"v1"`,
	})
	instance := rewritingPolicy(t, params)

	shared := &policy.SharedContext{APIId: "agent-uuid-1", Metadata: map[string]any{}}
	instance.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: shared,
		Headers:       policy.NewHeaders(nil),
		Method:        "GET",
		Path:          "/trips/.well-known/agent-card.json",
		Vhost:         "agents.example.com",
	}, params)

	if _, present := shared.Metadata[metadataKeyRewriteVhost]; present {
		t.Error("a managed card recorded a vhost it has no rewrite to use it for")
	}
}

// An interface that states no protocol version is rewritten.
//
// The per-interface version is optional in a card, and one that omits it
// contradicts nothing — a card carrying the version only at the top level is a
// normal shape. Treating the omission as a mismatch would refuse those cards
// outright; treating it as a match is what keeps the version check a check on
// what the upstream actually claimed.
func TestAnInterfaceStatingNoVersionIsRewritten(t *testing.T) {
	versionless := `{
	  "name": "Trip Planner",
	  "protocolVersion": "1.0",
	  "supportedInterfaces": [
	    {"protocolBinding": "JSONRPC", "url": "https://agent.internal/"},
	    {"protocolBinding": "HTTP+JSON", "protocolVersion": "1.0", "url": "https://agent.internal/v1"}
	  ]
	}`

	params := publicRewriteParams()
	instance := rewritingPolicy(t, params)
	mods := requireModifications(t, runResponse(t, instance, params,
		schemeHTTPS, "agents.example.com", "", 200, versionless))

	urls := interfaceURLs(t, decodeCard(t, mods.Body))
	if got, want := urls[transportJSONRPC], "https://agents.example.com/trips/rpc"; got != want {
		t.Errorf("JSONRPC url = %q, want %q", got, want)
	}
	if got, want := urls[transportHTTPJSON], "https://agents.example.com/trips/v1"; got != want {
		t.Errorf("HTTP+JSON url = %q, want %q", got, want)
	}
}

// A version this gateway does not serve is only a failure on a binding it does
// front. On a binding it does not, the entry is left alone before the version is
// ever considered — the operator did not expose that transport at any version.
func TestAnUnservedBindingIsLeftAloneRegardlessOfItsVersion(t *testing.T) {
	grpcOldVersion := `{
	  "name": "Trip Planner",
	  "supportedInterfaces": [
	    {"protocolBinding": "GRPC", "protocolVersion": "0.3", "url": "https://agent.internal:9000"}
	  ]
	}`

	params := publicRewriteParams()
	instance := rewritingPolicy(t, params)
	mods := requireModifications(t, runResponse(t, instance, params,
		schemeHTTPS, "agents.example.com", "", 200, grpcOldVersion))

	if mods.Body != nil {
		t.Errorf("the response body was replaced: %s", mods.Body)
	}
}

// jsonrpcOnlyParams is what the controller writes for an Agent that exposes one
// transport, JSON-RPC — the case where the upstream advertises a binding the
// gateway was not configured to front.
func jsonrpcOnlyParams() map[string]any {
	return cardBlockParams(map[string]any{ParamRewriteUrls: map[string]any{
		ParamProtocolVersion: "1.0",
		ParamInterfaces: []any{
			map[string]any{ParamProtocolBinding: transportJSONRPC, ParamPath: "/trips/rpc"},
		},
	}})
}

// An interface whose binding no configured transport serves keeps the URL the
// upstream wrote; the ones the gateway does serve are still rewritten.
//
// Exposing one of an agent's two transports is a deliberate configuration, not
// an error, and the gateway has no endpoint to offer for the transport it was
// not asked to carry. The consequence is real and is the operator's to accept: a
// client selecting the untouched binding talks to the agent directly, outside
// every policy on this gateway.
func TestRewriteLeavesABindingNoTransportServesAlone(t *testing.T) {
	params := jsonrpcOnlyParams()
	instance := rewritingPolicy(t, params)
	mods := requireModifications(t, runResponse(t, instance, params,
		schemeHTTPS, "agents.example.com", "", 200, upstreamCard))

	urls := interfaceURLs(t, decodeCard(t, mods.Body))
	if got, want := urls[transportJSONRPC], "https://agents.example.com/trips/rpc"; got != want {
		t.Errorf("JSONRPC url = %q, want the gateway endpoint %q", got, want)
	}
	if got, want := urls[transportHTTPJSON], "https://agent.internal/v1"; got != want {
		t.Errorf("HTTP+JSON url = %q, want the upstream's own %q — no transport serves it", got, want)
	}
}

// A card whose every interface is on a binding this gateway does not front is
// forwarded exactly as it arrived, signatures and caching headers included.
//
// Nothing in the document was changed, so there is nothing for the gateway to
// invalidate: dropping the signature or the validators here would degrade a
// response the gateway decided not to touch.
//
// This is the one shape that is forwarded rather than refused, and it is a
// well-formed card describing transports the operator did not expose — not a
// document the gateway failed to read. A card it cannot read as a card fails
// closed instead; see TestUnsafeRewriteFailsClosed.
func TestCardWithNoServedBindingIsForwardedUnchanged(t *testing.T) {
	// A2A defines a gRPC binding this gateway does not expose, so a card carrying
	// only that one is the realistic shape — and it must not be mistaken for a
	// malformed card.
	// A2A defines a gRPC binding this gateway does not expose, so a card carrying
	// only that one is the realistic shape — and it must not be mistaken for a
	// malformed card.
	grpcOnly := `{
	  "name": "Trip Planner",
	  "supportedInterfaces": [
	    {"protocolBinding": "GRPC", "url": "https://agent.internal:9000"}
	  ],
	  "signatures": [{"protected": "eyJhbGciOiJFUzI1NiJ9", "signature": "c2ln"}]
	}`

	params := publicRewriteParams()
	instance := rewritingPolicy(t, params)
	mods := requireModifications(t, runResponse(t, instance, params,
		schemeHTTPS, "agents.example.com", "", 200, grpcOnly))

	if mods.Body != nil {
		t.Errorf("the response body was replaced: %s", mods.Body)
	}
	if len(mods.HeadersToSet) != 0 {
		t.Errorf("HeadersToSet = %v, want none for an unchanged response", mods.HeadersToSet)
	}
	if len(mods.HeadersToRemove) != 0 {
		t.Errorf("HeadersToRemove = %v, want none for an unchanged response", mods.HeadersToRemove)
	}
}

// The same on the protected card's JSON-RPC binding: an untouched card leaves
// its envelope untouched too, rather than being re-encoded around identical
// bytes.
func TestProtectedJSONRPCResponseWithNoServedBindingIsForwardedUnchanged(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":7,"result":{"supportedInterfaces":[` +
		`{"protocolBinding":"GRPC","url":"https://agent.internal:9000"}]}}`

	params := protectedRewriteParams()
	instance := rewritingPolicy(t, params)
	mods := requireModifications(t, runResponse(t, instance, params,
		schemeHTTPS, "agents.example.com", transportJSONRPC, 200, body))

	if mods.Body != nil {
		t.Errorf("the response body was replaced: %s", mods.Body)
	}
}

// Everything the rewrite does not own survives it, and the one thing it
// invalidates is removed.
//
// The extension field is the card author's, and an upstream that put structure
// there gets it back unchanged. The signature is not: it was computed over URLs
// that are no longer in the document, so a client verifying it would reject the
// card — and the gateway does not re-sign a passthrough card, because it did not
// author one.
func TestRewritePreservesTheDocumentAndDropsTheSignature(t *testing.T) {
	instance := rewritingPolicy(t, publicRewriteParams())
	mods := requireModifications(t, runResponse(t, instance, publicRewriteParams(),
		schemeHTTPS, "agents.example.com", "", 200, upstreamCard))

	card := decodeCard(t, mods.Body)
	if _, present := card[cardFieldSignatures]; present {
		t.Error("the upstream signature survived a rewrite it no longer covers")
	}
	if got := card["name"]; got != "Trip Planner" {
		t.Errorf("name = %v, want Trip Planner", got)
	}
	extension, ok := card["x-vendor-metadata"].(map[string]any)
	if !ok {
		t.Fatalf("the extension field did not survive: %#v", card["x-vendor-metadata"])
	}
	if got := extension["team"]; got != "trips" {
		t.Errorf("extension team = %v, want trips", got)
	}
	if got, ok := extension["regions"].([]any); !ok || len(got) != 2 {
		t.Errorf("extension regions = %#v, want two entries", extension["regions"])
	}
	capabilities, ok := card["capabilities"].(map[string]any)
	if !ok || capabilities["extendedAgentCard"] != true {
		t.Errorf("capabilities did not survive: %#v", card["capabilities"])
	}
}

// A rewritten card must not be cached, and the upstream's own validators must
// not travel with it.
//
// The document depends on the scheme and authority of the request that fetched
// it, so a shared cache holding one would serve an http client's card to an
// https client. The validators are worse than useless: they identify the
// upstream's bytes, so a client that stored one would revalidate its way back to
// the unrewritten card and go on talking to the agent directly.
func TestRewrittenResponseIsUncacheableAndDropsUpstreamValidators(t *testing.T) {
	instance := rewritingPolicy(t, publicRewriteParams())
	mods := requireModifications(t, runResponse(t, instance, publicRewriteParams(),
		schemeHTTPS, "agents.example.com", "", 200, upstreamCard))

	if got := mods.HeadersToSet["cache-control"]; got != cacheControlNoStore {
		t.Errorf("cache-control = %q, want %q", got, cacheControlNoStore)
	}
	for _, header := range []string{"etag", "last-modified"} {
		if !slices.Contains(mods.HeadersToRemove, header) {
			t.Errorf("%q is not removed from a rewritten response: %v", header, mods.HeadersToRemove)
		}
	}
}

// A conditional request would be answered 304 with no body, and a rewrite needs
// a body. The client's validator identifies the upstream's bytes rather than the
// gateway's, so revalidating against it would let the client keep a card
// advertising the upstream's URLs indefinitely.
func TestRewritingSuppressesConditionalRequestHeaders(t *testing.T) {
	instance := rewritingPolicy(t, publicRewriteParams())

	action := instance.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{APIId: "agent-uuid-1"},
		Headers:       policy.NewHeaders(map[string][]string{"If-None-Match": {`"upstream"`}}),
		Method:        "GET",
		Path:          "/trips/.well-known/agent-card.json",
	}, publicRewriteParams())

	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("a rewriting passthrough card must forward the request, got %T", action)
	}
	for _, header := range []string{"if-none-match", "if-modified-since"} {
		if !slices.Contains(mods.HeadersToRemove, header) {
			t.Errorf("%q is not suppressed: %v", header, mods.HeadersToRemove)
		}
	}
}

// ─── The protected representation ────────────────────────────────────────────

// On the JSON-RPC binding the card arrives inside a result envelope, and only
// the card is rewritten. The id comes back as the JSON value it arrived as: a
// client matches responses to requests on it, so coercing 7 to "7" breaks the
// correlation with nothing to report it.
func TestProtectedJSONRPCRewriteRewritesOnlyTheResult(t *testing.T) {
	for name, id := range map[string]string{
		"numeric id": `7`,
		"string id":  `"abc"`,
		"null id":    `null`,
	} {
		t.Run(name, func(t *testing.T) {
			envelope := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, id, upstreamCard)

			instance := rewritingPolicy(t, protectedRewriteParams())
			mods := requireModifications(t, runResponse(t, instance, protectedRewriteParams(),
				schemeHTTPS, "agents.example.com", transportJSONRPC, 200, envelope))

			var out struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      json.RawMessage `json:"id"`
				Result  json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal(mods.Body, &out); err != nil {
				t.Fatalf("rewritten envelope is not JSON: %v\n%s", err, mods.Body)
			}
			if out.JSONRPC != jsonRPCVersion {
				t.Errorf("jsonrpc = %q, want %q", out.JSONRPC, jsonRPCVersion)
			}
			if string(out.ID) != id {
				t.Errorf("id = %s, want %s (echoed as the JSON value it arrived as)", out.ID, id)
			}

			urls := interfaceURLs(t, decodeCard(t, out.Result))
			if got, want := urls[transportJSONRPC], "https://agents.example.com/trips/rpc"; got != want {
				t.Errorf("JSONRPC url = %q, want %q", got, want)
			}
		})
	}
}

// The HTTP+JSON binding returns the bare card, so it is rewritten in place.
func TestProtectedHTTPJSONRewriteRewritesTheBareCard(t *testing.T) {
	instance := rewritingPolicy(t, protectedRewriteParams())
	mods := requireModifications(t, runResponse(t, instance, protectedRewriteParams(),
		schemeHTTPS, "agents.example.com", transportHTTPJSON, 200, upstreamCard))

	urls := interfaceURLs(t, decodeCard(t, mods.Body))
	if got, want := urls[transportHTTPJSON], "https://agents.example.com/trips/v1"; got != want {
		t.Errorf("HTTP+JSON url = %q, want %q", got, want)
	}
}

// A JSON-RPC error response carries no card. Rewriting one would mean inventing
// a result, turning the upstream's failure into a success, so it is forwarded as
// it stands — the upstream's own error reaches the client unaltered.
func TestProtectedJSONRPCErrorResponseIsForwardedUnchanged(t *testing.T) {
	instance := rewritingPolicy(t, protectedRewriteParams())
	action := runResponse(t, instance, protectedRewriteParams(),
		schemeHTTPS, "agents.example.com", transportJSONRPC, 200,
		`{"jsonrpc":"2.0","id":1,"error":{"code":-32001,"message":"not found"}}`)

	mods := requireModifications(t, action)
	if mods.Body != nil {
		t.Errorf("a JSON-RPC error response was rewritten: %s", mods.Body)
	}
}

// A protected rewrite on a binding the resolver did not name cannot know where
// the card sits in the response, so there is nothing safe to rewrite. Forwarding
// would publish the upstream's own URLs under a configuration that says
// otherwise, so it fails closed instead.
func TestProtectedRewriteWithoutAKnownTransportFailsClosed(t *testing.T) {
	instance := rewritingPolicy(t, protectedRewriteParams())
	response := requireImmediateResponse(t, runResponse(t, instance, protectedRewriteParams(),
		schemeHTTPS, "agents.example.com", "", 200, upstreamCard))

	if response.StatusCode != 500 {
		t.Errorf("status = %d, want 500", response.StatusCode)
	}
	if strings.Contains(string(response.Body), "agent.internal") {
		t.Error("the failure response leaked the upstream's own card")
	}
}

// ─── What is left alone ──────────────────────────────────────────────────────

// Only a success carries a card. An upstream failure — including the 401 an
// upstream may answer a protected card request with — is the upstream's own
// answer, and a bodyless response has nothing to rewrite.
func TestNonSuccessAndBodylessResponsesAreForwardedUnchanged(t *testing.T) {
	instance := rewritingPolicy(t, publicRewriteParams())

	for name, testCase := range map[string]struct {
		status int
		body   string
	}{
		"upstream 404":        {404, `{"error":"not found"}`},
		"upstream 401":        {401, `{"error":"unauthorized"}`},
		"upstream 500":        {500, "oops"},
		"success with nobody": {204, ""},
	} {
		t.Run(name, func(t *testing.T) {
			mods := requireModifications(t, runResponse(t, instance, publicRewriteParams(),
				schemeHTTPS, "agents.example.com", "", testCase.status, testCase.body))
			if mods.Body != nil {
				t.Errorf("body was rewritten: %s", mods.Body)
			}
			if len(mods.HeadersToSet) != 0 || len(mods.HeadersToRemove) != 0 {
				t.Errorf("headers were changed: set=%v remove=%v", mods.HeadersToSet, mods.HeadersToRemove)
			}
		})
	}
}

// With rewriting off, the instance declares no response phase at all — that is
// what keeps every other chain, streaming responses above all, free of a
// response-body requirement it has no use for. The buffering is enabled per
// instance, not per policy.
func TestResponseBufferingIsEnabledOnlyForARewritingInstance(t *testing.T) {
	plain := rewritingPolicy(t, cardParams())
	if got := plain.Mode().ResponseBodyMode; got != policy.BodyModeSkip {
		t.Errorf("a managed card instance declares ResponseBodyMode %q, want %q",
			got, policy.BodyModeSkip)
	}
	if plain != ins {
		t.Error("a non-rewriting attachment should reuse the shared stateless instance")
	}

	rewriting := rewritingPolicy(t, publicRewriteParams())
	if got := rewriting.Mode().ResponseBodyMode; got != policy.BodyModeBuffer {
		t.Errorf("a rewriting instance declares ResponseBodyMode %q, want %q",
			got, policy.BodyModeBuffer)
	}
	if _, ok := policy.Policy(rewriting).(policy.ResponsePolicy); !ok {
		t.Error("a rewriting instance must implement ResponsePolicy")
	}
}

// A plain passthrough card forwards a conditional request untouched: the
// client's revalidation is between it and the upstream, exactly as it is with no
// A2A instance on the chain at all.
func TestPlainPassthroughDoesNotSuppressConditionalRequests(t *testing.T) {
	action := ins.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{APIId: "agent-uuid-1"},
		Headers:       policy.NewHeaders(nil),
		Method:        "GET",
		Path:          "/trips/v1/extendedAgentCard",
	}, protectedParams(""))

	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected the request to be forwarded, got %T", action)
	}
	if len(mods.HeadersToRemove) != 0 {
		t.Errorf("a non-rewriting instance suppressed headers: %v", mods.HeadersToRemove)
	}
}

// ─── Failing closed ──────────────────────────────────────────────────────────

// A successful response that cannot be rewritten safely is refused, not
// forwarded.
//
// Forwarding would publish the upstream's own interface URLs — the precise
// outcome the flag was enabled to prevent — and it would do it looking like a
// success, so nothing downstream would report it. The failure is sterile: every
// reason here names either the upstream's document or the gateway's own
// configuration, and neither is a client's business.
func TestUnsafeRewriteFailsClosed(t *testing.T) {
	cases := map[string]struct {
		body      string
		scheme    string
		authority string
	}{
		"not JSON": {
			body: "<html>not a card</html>", scheme: schemeHTTPS, authority: "agents.example.com",
		},
		// A successful response the gateway cannot read as a card is refused, not
		// forwarded: it cannot tell what URLs are in it, so it cannot tell whether
		// forwarding would publish the agent's own address.
		"no supportedInterfaces": {
			body: `{"name":"Trip Planner"}`, scheme: schemeHTTPS, authority: "agents.example.com",
		},
		"empty supportedInterfaces": {
			body:   `{"name":"Trip Planner","supportedInterfaces":[]}`,
			scheme: schemeHTTPS, authority: "agents.example.com",
		},
		// A JSON null decodes into a nil map without an error, so it reaches the
		// interface lookup looking like an object with no members.
		"a JSON null document": {
			body: "null", scheme: schemeHTTPS, authority: "agents.example.com",
		},
		// The gateway serves this binding, so the entry is one it would rewrite —
		// but its endpoint speaks 1.0, and pointing a 0.3 interface at it would
		// publish a protocol the endpoint does not implement.
		"an interface on a version the gateway does not serve": {
			body: `{"supportedInterfaces":[` +
				`{"protocolBinding":"JSONRPC","protocolVersion":"0.3","url":"https://agent.internal/"}]}`,
			scheme: schemeHTTPS, authority: "agents.example.com",
		},
		"an interface whose version is not a string": {
			body: `{"supportedInterfaces":[` +
				`{"protocolBinding":"JSONRPC","protocolVersion":3,"url":"https://agent.internal/"}]}`,
			scheme: schemeHTTPS, authority: "agents.example.com",
		},
		"interfaces not a list": {
			body:   `{"supportedInterfaces":{"protocolBinding":"JSONRPC"}}`,
			scheme: schemeHTTPS, authority: "agents.example.com",
		},
		"interface declares no binding": {
			body:   `{"supportedInterfaces":[{"url":"https://agent.internal/"}]}`,
			scheme: schemeHTTPS, authority: "agents.example.com",
		},
		// A card past the ceiling is refused rather than buffered: this is a public
		// discovery route, so an upstream could otherwise make the gateway hold
		// arbitrarily much for any anonymous caller.
		"oversized body": {
			body:   oversizedCard(),
			scheme: schemeHTTPS, authority: "agents.example.com",
		},
		"unusable scheme": {
			body: upstreamCard, scheme: "gopher", authority: "agents.example.com",
		},
		"no scheme at all": {
			body: upstreamCard, scheme: "", authority: "agents.example.com",
		},
		// With no authority on the request and no vhost recorded for the route, a
		// rewritten URL would name a host the gateway invented.
		"no authority and no vhost": {
			body: upstreamCard, scheme: schemeHTTPS, authority: "",
		},
		"authority carrying a path": {
			body: upstreamCard, scheme: schemeHTTPS, authority: "agents.example.com/evil",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			params := publicRewriteParams()
			instance := rewritingPolicy(t, params)
			response := requireImmediateResponse(t, runResponse(t, instance, params,
				testCase.scheme, testCase.authority, "", 200, testCase.body))

			if response.StatusCode != 500 {
				t.Errorf("status = %d, want 500", response.StatusCode)
			}
			if got := string(response.Body); !strings.Contains(got, "unavailable") {
				t.Errorf("body = %q, want the sterile unavailable response", got)
			}
			if strings.Contains(string(response.Body), "agent.internal") {
				t.Error("the failure response leaked the upstream's own card")
			}
		})
	}
}

// oversizedCard is a structurally valid card whose encoding is past the ceiling.
func oversizedCard() string {
	filler := strings.Repeat("x", maxCardResponseBytes)
	return `{"description":"` + filler + `","supportedInterfaces":[` +
		`{"protocolBinding":"JSONRPC","url":"https://agent.internal/"}]}`
}

// A rewrite block the controller wrote wrong fails this one route closed rather
// than the whole chain build.
//
// The distinction matters: an error from the factory fails the snapshot for
// every route on the node, over a defect in one Agent's parameters. Either way
// nothing is forwarded — a card configured to be rewritten is never proxied
// unrewritten, because that publishes the upstream's URLs.
func TestUnusableRewriteConfigurationFailsTheRouteClosed(t *testing.T) {
	cases := map[string]any{
		"not an object": "true",
		// The version is matched against every advertised interface, so a block
		// without one could not decide whether any interface is one this gateway
		// serves.
		"no protocol version": map[string]any{
			ParamInterfaces: []any{map[string]any{ParamProtocolBinding: transportJSONRPC, ParamPath: "/trips/rpc"}},
		},
		"empty protocol version": map[string]any{
			ParamProtocolVersion: "",
			ParamInterfaces:      []any{map[string]any{ParamProtocolBinding: transportJSONRPC, ParamPath: "/trips/rpc"}},
		},
		"no interface mapping":    map[string]any{ParamProtocolVersion: "1.0"},
		"empty interface mapping": map[string]any{ParamProtocolVersion: "1.0", ParamInterfaces: []any{}},
		"entry is not an object":  map[string]any{ParamProtocolVersion: "1.0", ParamInterfaces: []any{"JSONRPC"}},
		"entry with no path": map[string]any{
			ParamProtocolVersion: "1.0",
			ParamInterfaces:      []any{map[string]any{ParamProtocolBinding: transportJSONRPC}},
		},
		"entry with no binding": map[string]any{
			ParamProtocolVersion: "1.0",
			ParamInterfaces:      []any{map[string]any{ParamPath: "/trips/rpc"}},
		},
	}

	for name, block := range cases {
		t.Run(name, func(t *testing.T) {
			params := cardBlockParams(map[string]any{ParamRewriteUrls: block})

			instance := rewritingPolicy(t, params)
			if instance.configErr == nil {
				t.Fatal("an unusable rewrite block was accepted")
			}

			action := instance.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
				SharedContext: &policy.SharedContext{APIId: "agent-uuid-1"},
				Headers:       policy.NewHeaders(nil),
				Method:        "GET",
				Path:          "/trips/.well-known/agent-card.json",
			}, params)

			immediate, ok := action.(policy.ImmediateResponse)
			if !ok {
				t.Fatalf("expected the request to be refused, got %T", action)
			}
			if immediate.StatusCode != 500 {
				t.Errorf("status = %d, want 500", immediate.StatusCode)
			}
		})
	}
}

// The rewrite parameter names are the contract with the gateway controller's
// Agent transformer, which spells them in another module. A rename on one side
// alone is silent in the worst way: the block reads as absent, so the card route
// stops buffering its response and quietly proxies the upstream's own URLs.
func TestRewriteParameterNamesMatchTheControllerContract(t *testing.T) {
	for _, testCase := range []struct{ got, want string }{
		{ParamRewriteUrls, "rewriteUrls"},
		{ParamProtocolVersion, "protocolVersion"},
		{ParamInterfaces, "interfaces"},
		{ParamProtocolBinding, "protocolBinding"},
		{ParamPath, "path"},
	} {
		if testCase.got != testCase.want {
			t.Errorf("parameter name = %q, want %q", testCase.got, testCase.want)
		}
	}
}

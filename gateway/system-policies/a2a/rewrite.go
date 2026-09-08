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
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Interface URL rewriting for a passthrough Agent Card.
//
// A passthrough card is the upstream agent's own document, and the agent
// advertises the URLs *it* is reachable at. Proxied through the gateway
// unchanged, that document tells every client to talk to the agent directly —
// which is exactly the gateway the operator put in front of it, bypassed by its
// own discovery document. Rewriting replaces each advertised interface URL with
// the gateway endpoint serving that protocol binding, so a client configured
// from the card reaches the gateway and its policies.
//
// It is opt-in per representation (`rewriteUrls`) because it is a mutation of a
// document the gateway did not author: with it off, the response is forwarded
// byte for byte, signatures included.
//
// An interface is rewritten when this gateway serves both its protocol binding
// and the protocol version it advertises. A mismatched version is a failure —
// the binding is one the operator exposed, so half-rewriting it would publish a
// URL for a protocol the endpoint does not speak.
//
// A binding no configured transport serves is the other case, and it is left
// exactly as the upstream wrote it: the gateway has no endpoint to point it at, and the
// operator's transport list is the statement of which bindings this gateway
// carries — an interface outside it was never asked to come through here. The
// consequence is worth being explicit about: that URL still names the agent
// directly, so a client that selects that binding reaches the agent without
// traversing the gateway or any of its policies. Exposing a transport through
// the gateway is what brings its interface under those policies.
//
// What is deliberately *not* attempted: validating anything else about the card.
// Only the interface URLs are touched and only the `signatures` block is
// removed. The card's security declarations remain the upstream's claim, and
// remain unverifiable — enabling rewriting does not change that.

// Parameter names of the rewriteUrls block.
//
// Written by the gateway controller's Agent transformer (pkg/transform/agent.go,
// agentCardRewriteParams) and read here. The two live in separate Go modules and
// cannot share a constant, so the names are spelled once on each side and the
// coupling is asserted by a test on each side.
const (
	// ParamRewriteUrls is the nested block that turns rewriting on. Its presence
	// is the switch: a card block carrying neither it nor content is plain
	// passthrough and the response is never buffered.
	ParamRewriteUrls = "rewriteUrls"

	// ParamProtocolVersion is the A2A protocol version whose transport layout
	// ParamInterfaces describes.
	ParamProtocolVersion = "protocolVersion"

	// ParamInterfaces is the per-binding gateway endpoint mapping: one entry per
	// configured transport.
	ParamInterfaces = "interfaces"

	// ParamProtocolBinding and ParamPath are the fields of one mapping entry.
	// The binding values are the same protocolBinding enum an Agent Card's
	// supportedInterfaces entries carry, so the two compare without translation.
	ParamProtocolBinding = "protocolBinding"
	ParamPath            = "path"
)

// Agent Card field names read during a rewrite. The card is a free-form
// document, so the two fields this touches are named once here rather than
// spelled as literals at each use. They are the protobuf JSON names from the
// A2A definition (message AgentCard / AgentInterface).
const (
	cardFieldSupportedInterfaces = "supportedInterfaces"
	cardFieldSignatures          = "signatures"
	cardFieldProtocolBinding     = "protocolBinding"
	cardFieldProtocolVersion     = "protocolVersion"
	cardFieldURL                 = "url"
)

// JSON-RPC envelope fields. Only `result` is rewritten; everything else is
// carried through as the raw bytes it arrived as.
const (
	jsonRPCFieldResult = "result"
)

// maxCardResponseBytes bounds the buffered card response a rewrite will accept.
//
// 1 MiB is the same boundary a managed Agent Card is capped at by the
// controller, and for the same reason: it is the largest single object
// Kubernetes stores by default, so a card past it could not be configured on
// this platform either. Applying it to a *proxied* card as well is what keeps an
// upstream from making the gateway buffer arbitrarily much on a route any client
// can call — the card is a public discovery document.
const maxCardResponseBytes = 1024 * 1024

// jsonRPCEnvelopeAllowance is the extra room a JSON-RPC response gets on top of
// the card ceiling, for `{"jsonrpc":"2.0","id":…,"result":…}` around it. A
// generous fixed amount rather than a computed one: the envelope's own size is
// bounded by the caller's id, and the point of the allowance is that a card at
// the limit is not rejected merely for being wrapped.
const jsonRPCEnvelopeAllowance = 4096

// Schemes a rewritten URL may advertise.
//
// The scheme comes from the gateway's own :scheme metadata for the request, but
// it is checked against this set rather than trusted: an unexpected value would
// otherwise be pasted into a URL the gateway publishes to every client of the
// agent. http is as legitimate as https here — the request's own scheme is what
// gets advertised, and forcing https onto a plaintext deployment would publish a
// URL nothing listens on.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// errRewriteFailed is returned for every condition that makes a successful
// response unsafe to rewrite. One error rather than several because the caller
// does the same thing with all of them — answers a sterile failure and logs the
// detail — and because the distinction is never something a client should be
// able to read off a response.
var errRewriteFailed = errors.New("agent card response cannot be rewritten")

// rewriteConfig is the resolved rewriteUrls block: which gateway endpoint each
// protocol binding is served at, and how to address the gateway.
type rewriteConfig struct {
	// endpoints maps a protocolBinding to the absolute gateway path serving it.
	// It holds exactly the configured transports, so a binding missing from it is
	// one this gateway does not front, and that interface is left untouched.
	// Lookup is by binding rather than by position: a card lists its interfaces
	// in whatever order its author wrote them.
	endpoints map[string]string

	// protocolVersion is the A2A version the endpoints above serve, and half of
	// what an advertised interface has to match to be rewritten.
	//
	// An Agent's routes are generated for one protocol version, so an endpoint
	// here speaks that version and no other. A client reads an interface as the
	// triple (binding, version, url): pointing a url at an endpoint that serves a
	// different version than the entry claims publishes a combination the gateway
	// does not offer, and the client discovers that only when its calls fail.
	//
	// The card's own per-interface protocolVersion is never *changed* — this
	// policy rewrites URLs and does not restate what version the upstream claims
	// to speak. It is only read, to decide whether this gateway has an endpoint
	// that claim can point at.
	protocolVersion string

	// protected reports which representation this instance rewrites. It decides
	// the response shape, not the rewrite: a protected card on the JSON-RPC
	// binding arrives inside a JSON-RPC result envelope, while public discovery
	// and the HTTP+JSON binding return the bare document.
	protected bool
}

// parseRewriteConfig reads the rewriteUrls block out of a card parameter block.
//
// It returns (nil, nil) when the block is absent, which is the ordinary case:
// most card instances do not rewrite anything. A block that is present but
// unusable is an error rather than a silent "no rewriting" — the controller
// wrote it because an author asked for rewriting, and quietly proxying the
// upstream's own URLs instead would publish exactly the discovery document the
// flag exists to prevent.
func parseRewriteConfig(card map[string]any, protected bool) (*rewriteConfig, error) {
	if card == nil {
		return nil, nil
	}
	raw, present := card[ParamRewriteUrls]
	if !present {
		return nil, nil
	}
	block, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", ParamRewriteUrls)
	}

	cfg := &rewriteConfig{endpoints: map[string]string{}, protected: protected}
	// Required rather than optional: an advertised interface is matched on its
	// protocol version as well as its binding, so a block that names no version
	// could not decide whether any interface is one this gateway serves.
	version, ok := stringParam(block, ParamProtocolVersion)
	if !ok || version == "" {
		return nil, fmt.Errorf("%s.%s must be a non-empty string", ParamRewriteUrls, ParamProtocolVersion)
	}
	cfg.protocolVersion = version

	entries, ok := block[ParamInterfaces].([]any)
	if !ok || len(entries) == 0 {
		return nil, fmt.Errorf("%s.%s must be a non-empty list", ParamRewriteUrls, ParamInterfaces)
	}
	for _, entry := range entries {
		mapping, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s.%s entries must be objects", ParamRewriteUrls, ParamInterfaces)
		}
		binding, bindingOK := stringParam(mapping, ParamProtocolBinding)
		path, pathOK := stringParam(mapping, ParamPath)
		if !bindingOK || binding == "" || !pathOK || path == "" {
			return nil, fmt.Errorf("%s.%s entries must declare %s and %s",
				ParamRewriteUrls, ParamInterfaces, ParamProtocolBinding, ParamPath)
		}
		cfg.endpoints[binding] = path
	}
	return cfg, nil
}

// gatewayURL builds the URL a gateway path is advertised at.
//
// The path is the caller's, resolved from the configured transports. scheme and
// authority come from the request, so both are validated here rather than
// interpolated: this string is published to every client of the agent, and a
// value that arrived on the wire has no business reaching it unchecked.
func (c *rewriteConfig) gatewayURL(path, scheme, authority string) (string, error) {
	if scheme != schemeHTTP && scheme != schemeHTTPS {
		return "", fmt.Errorf("%w: request scheme %q is not http or https", errRewriteFailed, scheme)
	}
	if !validAuthority(authority) {
		return "", fmt.Errorf("%w: no usable gateway authority for the request", errRewriteFailed)
	}
	return (&url.URL{Scheme: scheme, Host: authority, Path: path}).String(), nil
}

// gatewayAuthority composes the authority a rewritten interface URL advertises,
// from the Agent's configured virtual host and the request the client made.
//
// The vhost decides the *host*, because that is the name the operator published
// the Agent under: the authority a client happened to dial may be an internal
// one — a cluster service name, a pod address, an ingress hostname — and putting
// that in a public discovery document tells every other client to use it too.
//
// The request supplies the *port*, because the vhost is a routing match and
// names none, while the port a client must connect on is exactly what the client
// that reached the gateway already demonstrated. A standard port for the scheme
// is left off, since it is implied and a URL that spells it out reads as a
// different origin to some clients.
//
// A wildcard vhost names no dialable host, so the request's own authority is
// used whole. (This is the same host/port split, and the same wildcard rule, the
// mcp-auth policy uses to build the resource URL in its WWW-Authenticate header;
// it differs only in the fallback, which is a configured host there and the
// request's own authority here.)
func gatewayAuthority(vhost, requestAuthority, scheme string) string {
	if vhost == "" || strings.Contains(vhost, "*") {
		return requestAuthority
	}
	port := authorityPort(requestAuthority)
	if port == "" || isStandardPort(scheme, port) {
		return vhost
	}
	return net.JoinHostPort(vhost, port)
}

// authorityPort is the port an authority names, or "" when it names none.
//
// net.SplitHostPort rather than a split on the last colon, so an IPv6 literal
// authority (`[::1]:8080`) yields its port rather than a fragment of its host.
func authorityPort(authority string) string {
	_, port, err := net.SplitHostPort(authority)
	if err != nil {
		return ""
	}
	return port
}

// isStandardPort reports whether a port is the default for the scheme, and so
// need not be spelled out in a published URL.
func isStandardPort(scheme, port string) bool {
	return (scheme == schemeHTTP && port == "80") || (scheme == schemeHTTPS && port == "443")
}

// validAuthority rejects an authority that cannot be published as one.
//
// The value is whatever gatewayAuthority composed, which is either the request's
// :authority outright or a configured host carrying the request's port — so at
// least part of it is always client-supplied: an empty one, one carrying a path or userinfo, or one carrying
// whitespace or control characters would produce a URL that means something
// other than what it reads as. A wildcard is rejected too — it is a virtual-host
// pattern, not a host a client can dial.
func validAuthority(authority string) bool {
	if authority == "" || len(authority) > 255 {
		return false
	}
	if strings.ContainsAny(authority, "/?#@*\\ \t\r\n") {
		return false
	}
	for _, r := range authority {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// rewriteResponse rewrites one buffered card response.
//
// body is the response as the upstream sent it, decompressed by the engine.
// The returned bytes replace it; an error means the response could not be
// rewritten safely and the caller must fail rather than return a partially
// rewritten card.
//
// A nil return with a nil error means "leave this response exactly as it is".
// Two responses get that: a JSON-RPC error, which holds no card to rewrite and
// which inventing one for would turn an upstream failure into a success; and a
// card whose interfaces are all on bindings this gateway does not front, which
// has nothing to point anywhere and is forwarded whole, signatures included.
// A card the gateway cannot read as a card at all is neither of those, and
// fails rather than being forwarded.
func (c *rewriteConfig) rewriteResponse(body []byte, transport, scheme, authority string) ([]byte, error) {
	if len(body) > c.maxResponseBytes() {
		return nil, fmt.Errorf("%w: response is %d bytes, over the %d byte ceiling",
			errRewriteFailed, len(body), c.maxResponseBytes())
	}

	if c.protected && transport == transportJSONRPC {
		return c.rewriteJSONRPCResponse(body, scheme, authority)
	}
	if c.protected && transport != transportHTTPJSON {
		// The binding was decided by the resolver that selected this chain, and
		// is read back from it. An unrecognised value means the response shape is
		// unknown, so where the card sits in it is unknown too — there is nothing
		// safe to rewrite.
		return nil, fmt.Errorf("%w: unrecognised A2A transport %q", errRewriteFailed, transport)
	}
	return c.rewriteCard(body, scheme, authority)
}

// maxResponseBytes is the ceiling for this instance's response shape.
func (c *rewriteConfig) maxResponseBytes() int {
	if c.protected {
		// The JSON-RPC binding wraps the card; the HTTP+JSON one does not. One
		// ceiling for both rather than a per-binding one, because the difference
		// is the envelope and the allowance is what the envelope is for.
		return maxCardResponseBytes + jsonRPCEnvelopeAllowance
	}
	return maxCardResponseBytes
}

// rewriteJSONRPCResponse rewrites the card inside a JSON-RPC 2.0 result
// envelope.
//
// Only `result` is touched. Every other member — `jsonrpc`, and above all `id` —
// is carried through as the raw bytes it arrived as, so a numeric id comes back
// a number and a string id a string: a client matches responses to requests on
// that value, and coercing 7 to "7" breaks the correlation silently.
//
// A response with no `result` is a JSON-RPC error (or something else this policy
// has no card in), and is returned unchanged.
func (c *rewriteConfig) rewriteJSONRPCResponse(body []byte, scheme, authority string) ([]byte, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("%w: response is not a JSON-RPC envelope: %v", errRewriteFailed, err)
	}
	// As in rewriteCard: a JSON `null` decodes into a nil map without error, and
	// it is not the "no result member" case below — it carries no error object
	// either, so there is nothing to forward as the upstream's own answer.
	if envelope == nil {
		return nil, fmt.Errorf("%w: response is a JSON null, not a JSON-RPC envelope", errRewriteFailed)
	}
	result, present := envelope[jsonRPCFieldResult]
	if !present {
		return nil, nil
	}

	rewritten, err := c.rewriteCard(result, scheme, authority)
	if err != nil {
		return nil, err
	}
	if rewritten == nil {
		// The card held nothing this gateway serves, so it was not changed. The
		// envelope is left alone with it, rather than re-encoded around identical
		// bytes — re-encoding would reorder the members of a document the gateway
		// has decided not to touch.
		return nil, nil
	}
	envelope[jsonRPCFieldResult] = rewritten

	out, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("%w: rewritten envelope cannot be encoded: %v", errRewriteFailed, err)
	}
	return out, nil
}

// assertInterfaceVersion refuses an interface whose protocol version this
// gateway's endpoint for that binding does not serve.
//
// A gateway endpoint is generated for the Agent's one configured protocol
// version, so pointing an interface at it asserts that the endpoint speaks the
// version that interface claims. When it does not, there is no correct URL to
// write: the gateway endpoint would be reached by clients expecting a protocol
// it does not implement, and the failure surfaces as malformed calls rather
// than as anything a client could attribute to the card.
//
// Refusing the whole response rather than skipping the entry is deliberate: the
// binding *is* one this gateway fronts, so skipping would leave the agent's own
// URL on an interface the operator did expose — a partial rewrite of exactly the
// kind that makes a card look as though the gateway were in the path.
//
// An interface that states no version is rewritten. It contradicts nothing, and
// a card that carries the version only at the top level is a normal shape.
func (c *rewriteConfig) assertInterfaceVersion(iface map[string]json.RawMessage, binding string) error {
	raw, present := iface[cardFieldProtocolVersion]
	if !present {
		return nil
	}
	var version string
	if err := json.Unmarshal(raw, &version); err != nil {
		return fmt.Errorf("%w: card interface declares an unreadable %s",
			errRewriteFailed, cardFieldProtocolVersion)
	}
	if version == "" || version == c.protocolVersion {
		return nil
	}
	return fmt.Errorf("%w: interface on %q advertises protocol version %q, and this gateway serves %q on that binding",
		errRewriteFailed, binding, version, c.protocolVersion)
}

// rewriteCard replaces every advertised interface URL with the gateway endpoint
// for that interface's binding, and drops the card's signatures.
//
// The document is walked as raw JSON members rather than decoded into typed
// values, so every field this does not touch — extension members, nested
// objects, large numbers — comes back byte-identical instead of surviving a
// decode/encode round trip that would reformat it. (Member *order* is not
// preserved; the bytes of each member are.)
//
// Only the interfaces this gateway fronts are rewritten; one whose binding no
// configured transport serves keeps the URL the upstream wrote, because there is
// no gateway endpoint that would serve it.
//
// A card whose every advertised interface is on a binding this gateway does not
// front is therefore returned unchanged, signalled by a nil result, meaning
// "forward this response as it stands". That
// keeps the signatures intact: they are removed because they no longer cover the
// document being returned, so a document that was not changed keeps them.
// Signatures are never replaced, only dropped: the gateway does not sign a
// passthrough card, and a card with no signature is honestly unsigned rather
// than falsely signed.
func (c *rewriteConfig) rewriteCard(card json.RawMessage, scheme, authority string) ([]byte, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(card, &document); err != nil {
		return nil, fmt.Errorf("%w: response is not an Agent Card object: %v", errRewriteFailed, err)
	}
	// A JSON `null` unmarshals into a nil map and reports no error, so the decode
	// succeeding is not on its own evidence that a card arrived.
	if document == nil {
		return nil, fmt.Errorf("%w: response is a JSON null, not an Agent Card", errRewriteFailed)
	}

	rawInterfaces, present := document[cardFieldSupportedInterfaces]
	if !present {
		return nil, fmt.Errorf("%w: card advertises no %s", errRewriteFailed, cardFieldSupportedInterfaces)
	}
	var interfaces []map[string]json.RawMessage
	if err := json.Unmarshal(rawInterfaces, &interfaces); err != nil {
		return nil, fmt.Errorf("%w: card %s is not a list of interfaces: %v",
			errRewriteFailed, cardFieldSupportedInterfaces, err)
	}
	if len(interfaces) == 0 {
		return nil, fmt.Errorf("%w: card %s is empty", errRewriteFailed, cardFieldSupportedInterfaces)
	}

	rewrote := false
	for _, iface := range interfaces {
		var binding string
		if err := json.Unmarshal(iface[cardFieldProtocolBinding], &binding); err != nil {
			// A structurally broken interface is still a hard failure, distinct from
			// a well-formed binding this gateway does not serve: the entry cannot be
			// identified at all, so there is no deciding whether it is one of ours.
			return nil, fmt.Errorf("%w: card interface declares no usable %s",
				errRewriteFailed, cardFieldProtocolBinding)
		}
		path, served := c.endpoints[binding]
		if !served {
			continue
		}
		if err := c.assertInterfaceVersion(iface, binding); err != nil {
			return nil, err
		}
		gateway, err := c.gatewayURL(path, scheme, authority)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(gateway)
		if err != nil {
			return nil, fmt.Errorf("%w: rewritten url cannot be encoded: %v", errRewriteFailed, err)
		}
		iface[cardFieldURL] = encoded
		rewrote = true
	}
	if !rewrote {
		return nil, nil
	}

	encodedInterfaces, err := json.Marshal(interfaces)
	if err != nil {
		return nil, fmt.Errorf("%w: rewritten interfaces cannot be encoded: %v", errRewriteFailed, err)
	}
	document[cardFieldSupportedInterfaces] = encodedInterfaces
	delete(document, cardFieldSignatures)

	out, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("%w: rewritten card cannot be encoded: %v", errRewriteFailed, err)
	}
	return out, nil
}

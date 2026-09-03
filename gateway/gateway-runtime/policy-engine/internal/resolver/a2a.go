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
	"net/url"
	"regexp"
	"strings"

	"github.com/wso2/api-platform/common/agentproto"
	"github.com/wso2/api-platform/common/chainkey"
)

// A2AResolver resolves a request on an Agent route to the canonical A2A operation
// it invokes, and therefore to that operation's policy chain.
//
// One factory serves both A2A transports, because the two differ only in *where*
// the operation is written, not in what it is. A JSON-RPC route carries every
// operation on one endpoint and names the one it wants in the request body, so its
// prepared resolver buffers the body and reads $.method per request. An HTTP+JSON
// route is generated per operation binding, so its operation is fixed at deploy
// time and its prepared resolver answers statically, costing the request path
// nothing. Both compose their key with the same helper from the same canonical
// operation name, which is what makes a JSON-RPC SendMessage and a
// POST /message:send run the identical chain — its authentication, its rate
// limits — rather than two chains that merely look alike.
//
// Everything the resolver knows about A2A comes from common/agentproto, keyed by
// the protocol version the route names. The controller generated that route's
// chains against one version's operation set, so resolving against a different
// one would select chains that were never emitted; there is deliberately no
// default version and no fallback to the only registered one.
type A2AResolver struct{}

func init() {
	// A2A is the first kind whose operation cannot be read off the route, so this
	// is the first non-identity entry in the production registry.
	RegisterDefault(&A2AResolver{})
}

// Name returns the wire value the controller writes into an Agent route's
// resolver_name. It is spelled once, in common/agentproto, because a mismatch
// between the two sides is silent: the route ships and every request to it fails
// to resolve.
func (*A2AResolver) Name() string { return agentproto.ResolverName }

// Prepare builds one Agent route's resolver from its resolver_config.
//
// Everything that can be decided without a request is decided here: the protocol
// version is resolved against the registry, the transport selects which shape of
// resolver this route gets, and — for a JSON-RPC route — the whole method table is
// composed into finished chain keys, so a request costs one map lookup and no
// string building at all.
//
// An error skips this one route (the caller counts it and moves on), so every
// rejection below is a route the engine refuses to serve rather than one it serves
// wrongly.
func (*A2AResolver) Prepare(cfg ResolverRouteConfig) (PreparedResolver, error) {
	routeCfg, err := parseA2AResolverConfig(cfg.ResolverConfig)
	if err != nil {
		return nil, err
	}

	// The partition is the controller's, not the request's, but a key composed from
	// an unusable component would reach a chain belonging to some other triple —
	// so it is checked once here rather than trusted.
	if !chainkey.ValidComponent(cfg.APIID) {
		return nil, fmt.Errorf("a2a resolver: route %q has an unusable API id", cfg.RouteKey)
	}
	// An empty vhost is the default vhost, so only the separator is disqualifying.
	if strings.Contains(cfg.Vhost, chainkey.Separator) {
		return nil, fmt.Errorf("a2a resolver: route %q has an unusable vhost", cfg.RouteKey)
	}

	// Fixed for every request on this route: the transport it was generated for and
	// the protocol version whose table it resolves against.
	facts := a2aRouteFacts{
		transport: string(routeCfg.Transport),
		version:   string(routeCfg.ProtocolVersion),
	}

	switch routeCfg.Transport {
	case agentproto.TransportJSONRPC:
		// The operation is in the body, so naming one here is a controller bug: it
		// would be silently ignored on every request, and the route would resolve to
		// whatever the caller asked for instead of what the config said.
		if routeCfg.Operation != "" {
			return nil, fmt.Errorf(
				"a2a resolver: route %q is %s and must not name an operation, but names %q",
				cfg.RouteKey, agentproto.TransportJSONRPC, routeCfg.Operation)
		}
		return newPreparedA2AJSONRPC(cfg, routeCfg.ProtocolVersion, facts)

	case agentproto.TransportHTTPJSON:
		if routeCfg.Operation == "" {
			return nil, fmt.Errorf("a2a resolver: route %q is %s and names no operation",
				cfg.RouteKey, agentproto.TransportHTTPJSON)
		}
		operation := string(routeCfg.Operation)
		if !agentproto.IsOperation(routeCfg.ProtocolVersion, operation) {
			return nil, fmt.Errorf("a2a resolver: route %q names %q, which is not an A2A %s operation",
				cfg.RouteKey, operation, routeCfg.ProtocolVersion)
		}
		key, err := composeA2AChainKey(cfg, operation)
		if err != nil {
			return nil, err
		}
		// Two operations carry their identifiers only in the request body, so those
		// routes read it; the rest have everything they contribute in the path and
		// stay on the static fast path. See carriesMessageInBody.
		if carriesMessageInBody(routeCfg.Operation) {
			return &preparedA2AHTTPJSONBody{
				a2aVersionGuard: a2aVersionGuard{facts: facts},
				chainKey:        key,
				operation:       operation,
			}, nil
		}
		// Nothing about this route varies per request, so its whole resolution —
		// protocol facts included — is built once, here.
		return &preparedA2AStatic{
			a2aVersionGuard: a2aVersionGuard{facts: facts},
			resolution: Resolution{
				ChainKey:   key,
				Attributes: facts.attributes(operation, a2aMessage{}),
			},
		}, nil

	default:
		return nil, fmt.Errorf("a2a resolver: route %q names unsupported transport %q",
			cfg.RouteKey, routeCfg.Transport)
	}
}

// parseA2AResolverConfig decodes and validates a route's resolver_config.
//
// Unknown fields are tolerated on purpose: a newer controller may add one, and a
// route this version can still serve correctly should not be dropped over a field
// it does not read. A version it does *not* understand is the opposite case and is
// refused outright.
func parseA2AResolverConfig(raw json.RawMessage) (agentproto.ResolverConfig, error) {
	var cfg agentproto.ResolverConfig
	if len(raw) == 0 {
		return cfg, errors.New("a2a resolver: route carries no resolver_config")
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("a2a resolver: resolver_config is not valid JSON: %w", err)
	}
	if cfg.ProtocolVersion == "" {
		return cfg, errors.New("a2a resolver: resolver_config names no protocolVersion")
	}
	// Never fall back to the newest or the only registered version: an Agent's
	// protocol version decides which operation set its chains were generated for,
	// so guessing here enforces a different set than the one its Agent Card
	// advertises.
	if !agentproto.IsSupportedVersion(cfg.ProtocolVersion) {
		return cfg, fmt.Errorf("a2a resolver: unsupported A2A protocol version %q (supported: %v)",
			cfg.ProtocolVersion, agentproto.Versions())
	}
	return cfg, nil
}

// composeA2AChainKey builds one operation's chain key for this route's partition,
// refusing an operation name that cannot be a key component.
//
// Canonical A2A names cannot contain the separator, so this never fires today. It
// is here because the alternative — trusting the table — puts the check nowhere,
// and a future version's entry that did contain one would compose the same key as a
// different (API, vhost, operation) triple.
func composeA2AChainKey(cfg ResolverRouteConfig, operation string) (string, error) {
	if !chainkey.ValidComponent(operation) {
		return "", fmt.Errorf("a2a resolver: route %q names operation %q, which cannot be a chain key component",
			cfg.RouteKey, operation)
	}
	return ChainKeyFor(cfg.APIID, cfg.Vhost, operation), nil
}

// ─── Request protocol version: the header-validation phase ───────────────────

// A2AVersionHeader is the header a client states its A2A protocol version in.
//
// Spelled lowercase because that is how header lookup is done — HTTP field names
// are case-insensitive and Envoy delivers them folded — while the query-parameter
// alternative below keeps the specification's exact casing, which is not.
const A2AVersionHeader = "a2a-version"

// A2AVersionQueryParam is the query-parameter alternative to the header.
//
// A2A 1.0 §3.6.1 lets a client that cannot set headers — a browser following a
// link, an SSE consumer — put the same value here instead. Query-parameter names
// are case-sensitive, so this is compared as spelled: "a2a-version=1.0" in a query
// string is a different parameter that this does not read, and a request carrying
// only that is treated as having stated no version at all.
const A2AVersionQueryParam = "A2A-Version"

// a2aImplicitVersion is what an absent or empty statement means.
//
// A2A 1.0 §3.6.2 fixes this: a client that says nothing is a 0.3 client, because
// 0.3 predates the requirement to say anything. It is emphatically *not* a default
// of "whatever this route serves" — reading it that way would let every
// non-conformant client through and make the requirement decorative.
const a2aImplicitVersion = "0.3"

// maxA2AVersionValueBytes bounds a stated version before it is matched or logged.
//
// The value is caller-controlled and reaches an internal debug log, so it is length-
// checked before the regular expression runs rather than after: a megabyte of digits
// should cost a comparison, not a match and a log line.
const maxA2AVersionValueBytes = 16

// a2aVersionPattern is the canonical Major.Minor form.
//
// Anchored, and deliberately narrow: no patch component, no sign, no leading-zero
// alias, no surrounding whitespace beyond the optional whitespace the header
// grammar itself allows. "1.0.0" and "01.0" are rejected rather than folded onto
// "1.0", because a client that sends either is not sending what the specification
// defines and the gateway is not the place to guess which version it meant.
var a2aVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// a2aVersionGuard enforces the request's stated protocol version against the one
// this route exposes.
//
// It runs before the route's chain is bound, before its body is requested and before
// anything reaches the agent, because the configured version *selects the operation
// table* the resolver is allowed to read the request against. A request stating 0.3
// and naming the 0.3 method "message/send" would otherwise be resolved against the
// 1.0 table, fail as an unknown 1.0 operation, and be reported as a client asking
// for something that does not exist — when what actually happened is a client
// speaking a protocol version this Agent does not expose.
//
// It performs no negotiation. D19 stands: one Agent exposes exactly one protocol
// version, so there is no range to match, no newest to fall back to and nothing to
// downgrade to. Success changes nothing about the request — the agent receives the
// header and query string byte-for-byte as sent, and may enforce the same rule
// itself.
type a2aVersionGuard struct {
	// facts are this route's configured transport and protocol version. The version
	// is the value a request must state; both travel with a rejection so telemetry
	// can say which binding of which version refused it.
	facts a2aRouteFacts
}

// ValidateHeaders applies A2A 1.0 §3.6 to one request.
//
// The order is: gather each representation, reject ambiguity, resolve the effective
// value, check its form, then check it against this route. Ambiguity is rejected
// before form, and form before support, so a caller sending two conflicting
// well-formed values is told it was ambiguous rather than that one of them was
// unsupported.
func (g a2aVersionGuard) ValidateHeaders(_ context.Context, view HeaderRequestView) error {
	stated, err := a2aStatedVersion(view)
	if err != nil {
		// The ambiguity checks are about the request alone and know nothing about
		// this route, so the route facts are attached on the way out — in one place,
		// so a reason added later cannot forget them.
		return g.withRouteFacts(err)
	}

	// Absent or empty means 0.3 (§3.6.2), never this route's version. On a 1.0 route
	// that is a mismatch like any other — which is the whole point: it is what makes
	// stating the version actually mandatory.
	if stated == "" {
		stated = a2aImplicitVersion
	}

	if len(stated) > maxA2AVersionValueBytes || !a2aVersionPattern.MatchString(stated) {
		return &ResolutionError{
			Kind: FailureInvalidParameter,
			// The value is the caller's own and is never echoed back; this cause
			// reaches the internal log only, already length-bounded above.
			Cause:      fmt.Errorf("A2A protocol version %q is not canonical Major.Minor", a2aBoundedValue(stated)),
			Attributes: g.failureAttributes(),
		}
	}

	if stated != g.facts.version {
		return &ResolutionError{
			Kind: FailureVersionNotSupported,
			Cause: fmt.Errorf("request states A2A protocol version %q; this route exposes %q",
				a2aBoundedValue(stated), g.facts.version),
			Attributes: g.failureAttributes(),
		}
	}
	return nil
}

// a2aStatedVersion reduces the two representations to the one value the client
// stated, or reports that it stated more than one.
//
// Both representations may be used together — the specification does not forbid it,
// and a client retrying through an intermediary that strips one is a real reason to
// send both — but then they have to agree exactly. Anything repeated is refused even
// when the repeats are equal: proxies and frameworks collapse duplicates
// differently (first wins, last wins, comma-joined), so accepting them would make
// the effective version depend on the route a request happened to take.
func a2aStatedVersion(view HeaderRequestView) (string, error) {
	header, headerPresent, err := a2aHeaderVersion(view.Headers)
	if err != nil {
		return "", err
	}
	query, queryPresent, err := a2aQueryVersion(view.Path)
	if err != nil {
		return "", err
	}

	switch {
	case headerPresent && queryPresent:
		if header != query {
			return "", &ResolutionError{
				Kind: FailureConflictingParameter,
				// Neither value is quoted: both are caller-supplied, and the fact
				// that they disagree is the whole diagnosis.
				Cause: errors.New("the A2A-Version header and query parameter state different protocol versions"),
			}
		}
		return header, nil
	case headerPresent:
		return header, nil
	case queryPresent:
		return query, nil
	default:
		return "", nil
	}
}

// a2aHeaderVersion returns the single value stated in the header, and whether the
// field was present at all.
//
// Optional whitespace is trimmed per the HTTP field grammar. A field carrying a
// comma is treated as list-valued and rejected: A2A-Version is a single-value field,
// so "1.0,1.0" is two statements that some intermediary combined, not one value that
// happens to contain a comma.
func a2aHeaderVersion(headers HeaderMap) (string, bool, error) {
	values := headers.Values(A2AVersionHeader)
	if len(values) == 0 {
		return "", false, nil
	}
	if len(values) > 1 {
		return "", false, &ResolutionError{
			Kind:  FailureConflictingParameter,
			Cause: fmt.Errorf("the A2A-Version header was sent %d times", len(values)),
		}
	}
	value := strings.TrimSpace(values[0])
	if strings.Contains(value, ",") {
		return "", false, &ResolutionError{
			Kind:  FailureConflictingParameter,
			Cause: errors.New("the A2A-Version header carries a comma-combined list of values"),
		}
	}
	return value, true, nil
}

// a2aQueryVersion returns the single value stated in the query string, and whether
// the parameter was present at all.
//
// It scans for this one parameter rather than calling url.ParseQuery, because
// ParseQuery's error semantics are wrong for a security check. It returns the pairs
// it *could* decode alongside an error describing the ones it could not, so a caller
// that discards the map on error discards good data: a request sending
// "?A2A-Version=0.3&bad=%zz" with a header of "1.0" would have had its conflicting
// query value silently dropped and been accepted. Trusting the map on error is no
// better — ParseQuery also drops a pair it rejected for its separator, so a repeat
// can vanish and leave what looks like a single unambiguous value.
//
// Scanning keeps the two concerns apart, which is the behaviour that was intended
// all along: a pair that is not this parameter is never decoded and never
// interpreted, so a malformed unrelated parameter stays the agent's to reject; a
// malformed *version* parameter is this function's business and is refused as the
// invalid parameter it is, rather than read as an absent one.
//
// Values are percent-decoded exactly as the standard parser would ("+" included) and
// otherwise not normalised.
func a2aQueryVersion(path string) (string, bool, error) {
	_, query, hasQuery := strings.Cut(path, "?")
	if !hasQuery {
		return "", false, nil
	}

	// Raw, still-encoded values, gathered before any of them is decoded: whether the
	// parameter was repeated has to be answered even when one of the repeats is the
	// unusable one.
	var stated []string
	for rest := query; rest != ""; {
		var pair string
		pair, rest, _ = strings.Cut(rest, "&")
		if pair == "" {
			continue
		}
		name, value, _ := strings.Cut(pair, "=")
		// The name is decoded too, so an encoded spelling of it ("A2A%2DVersion")
		// cannot smuggle a second statement past the repeat check below. A name that
		// will not decode cannot be this parameter, so it is skipped rather than
		// refused — it belongs to whatever the agent makes of it.
		decodedName, err := url.QueryUnescape(name)
		if err != nil || decodedName != A2AVersionQueryParam {
			continue
		}
		stated = append(stated, value)
	}

	switch len(stated) {
	case 0:
		return "", false, nil
	case 1:
		decoded, err := url.QueryUnescape(stated[0])
		if err != nil {
			return "", false, &ResolutionError{
				// Not "absent": the client did state a version, in a form nothing can
				// read. Reading it as absent would apply the implicit 0.3 and report
				// an unsupported version, which names the wrong fault.
				Kind:  FailureInvalidParameter,
				Cause: errors.New("the A2A-Version query parameter is not decodable"),
			}
		}
		return decoded, true, nil
	default:
		return "", false, &ResolutionError{
			Kind:  FailureConflictingParameter,
			Cause: fmt.Errorf("the A2A-Version query parameter was sent %d times", len(stated)),
		}
	}
}

// failureAttributes are the route facts a rejection carries into telemetry.
//
// A fresh map per rejection rather than one built at Prepare and shared: these
// travel out of the resolver into a span and an analytics event, and a shared map
// would be one mutation away from a rejection reporting another route's binding.
// Rejections are rare, so the allocation is not on any hot path.
//
// The stated version is deliberately absent. It is caller-controlled and unbounded,
// so it belongs in the internal log — where the causes above put it — and nowhere
// that becomes a metric label or an exported event dimension.
func (g a2aVersionGuard) failureAttributes() map[string]string {
	return map[string]string{
		AttrA2ATransport:       g.facts.transport,
		AttrA2AProtocolVersion: g.facts.version,
	}
}

// withRouteFacts stamps this route's facts onto a classified failure raised by the
// request-shape checks, which have no route to name on their own.
func (g a2aVersionGuard) withRouteFacts(err error) error {
	failure, ok := errors.AsType[*ResolutionError](err)
	if !ok {
		return err
	}
	out := *failure
	out.Attributes = g.failureAttributes()
	return &out
}

// a2aBoundedValue caps a caller-supplied value before it reaches an internal log, so
// a hostile client cannot write an arbitrarily long line into it. Truncation is safe
// here in a way it is not for the identifiers in addIdentifiers: this value is only
// ever read by a human diagnosing a rejection, never correlated on.
func a2aBoundedValue(value string) string {
	if len(value) <= maxA2AVersionValueBytes {
		return value
	}
	return value[:maxA2AVersionValueBytes] + "…"
}

// ─── JSON-RPC: one route, every operation ────────────────────────────────────

// preparedA2AJSONRPC is one JSON-RPC endpoint. It holds the finished chain key for
// every operation its protocol version defines, so resolution is a lookup rather
// than a parse-and-build.
type preparedA2AJSONRPC struct {
	// a2aVersionGuard validates the request's stated protocol version before this
	// route's body is ever asked for. Embedded, so ValidateHeaders is promoted and
	// every prepared A2A form carries it by construction rather than by remembering
	// to implement it.
	a2aVersionGuard

	// chainKeys maps a JSON-RPC method name to the chain key it binds to.
	//
	// In A2A the JSON-RPC method name of an operation is its canonical operation
	// name verbatim, so this is the version's operation set with its keys
	// pre-composed — not a translation table. Its membership is also what makes the
	// operation set closed: a method absent from it is not an A2A operation, which
	// is what lets a *missing chain* for a method that is present be reported as
	// deployment skew rather than blamed on the caller.
	chainKeys map[string]string
}

// newPreparedA2AJSONRPC composes the chain key for every operation of version.
func newPreparedA2AJSONRPC(
	cfg ResolverRouteConfig,
	version agentproto.ProtocolVersion,
	facts a2aRouteFacts,
) (PreparedResolver, error) {
	operations, ok := agentproto.Operations(version)
	if !ok {
		// Unreachable: the version was checked against the same registry a moment
		// ago. Handled rather than ignored so a future registry that could change
		// between the two reads fails closed instead of preparing an empty table
		// that rejects every request as an unknown operation.
		return nil, fmt.Errorf("a2a resolver: A2A %s has no operation table", version)
	}
	chainKeys := make(map[string]string, len(operations))
	for _, operation := range operations {
		key, err := composeA2AChainKey(cfg, string(operation))
		if err != nil {
			return nil, err
		}
		chainKeys[string(operation)] = key
	}
	return &preparedA2AJSONRPC{
		a2aVersionGuard: a2aVersionGuard{facts: facts},
		chainKeys:       chainKeys,
	}, nil
}

// Requirements asks for the buffered request body: the method is in it, and there
// is nowhere else on this route to read the operation from.
func (*preparedA2AJSONRPC) Requirements() RequestRequirements {
	return RequestRequirements{Body: BodyBuffered}
}

// Resolve reads the JSON-RPC method out of the request body and returns the chain
// key it binds to.
//
// It validates only what selecting a chain requires — the envelope is an object, it
// declares JSON-RPC 2.0, and it names one known method. Everything else about the
// request (params, id, whether the operation's arguments are well-formed) is the
// Agent's business, not the gateway's: rejecting more here would turn the gateway
// into a second, weaker A2A validator whose refusals the client cannot read anyway,
// because a resolution failure renders as the engine's sterile generic response.
func (r *preparedA2AJSONRPC) Resolve(_ context.Context, view RequestView) (Resolution, error) {
	body := trimLeadingJSONSpace(view.Body)

	// A request whose headers were end-of-stream never reaches a body callback, so a
	// BodyBuffered route is resolved at the header phase with no body at all. On this
	// route that is simply a JSON-RPC call with no envelope.
	if len(body) == 0 {
		return Resolution{}, &ResolutionError{
			Kind:  FailureInvalidRequest,
			Cause: errors.New("JSON-RPC request carries no body"),
		}
	}

	// Checked before parsing, so a batch is reported as the batch it is rather than
	// as a malformed single call. One request selects one chain, so a batch has no
	// answer here: two operations in one envelope would need two chains, and there is
	// no composition rule that says which of their authentications or rate limits win.
	if body[0] == '[' {
		return Resolution{}, &ResolutionError{
			Kind:  FailureMultiOperation,
			Cause: errors.New("JSON-RPC batch requests are not supported"),
		}
	}

	// One pass captures both what selects the chain and what is carried forward, so
	// the payload is never parsed twice for the same request.
	var envelope struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  struct {
			Message a2aMessage `json:"message"`
		} `json:"params"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return Resolution{}, &ResolutionError{Kind: FailureParse, Cause: err}
	}
	if envelope.JSONRPC != jsonRPCVersion {
		return Resolution{}, &ResolutionError{
			Kind:  FailureInvalidRequest,
			Cause: fmt.Errorf("request does not declare JSON-RPC %s", jsonRPCVersion),
		}
	}
	if envelope.Method == "" {
		return Resolution{}, &ResolutionError{
			Kind:  FailureInvalidRequest,
			Cause: errors.New("JSON-RPC request names no method"),
		}
	}

	// Exact match: A2A operation names are case-sensitive, and a near-miss is a
	// client sending an operation that does not exist, not a value to normalise into
	// one that does.
	key, known := r.chainKeys[envelope.Method]
	if !known {
		return Resolution{}, &ResolutionError{
			Kind: FailureUnknownOperation,
			// The method is the client's own input and is not echoed to it; this
			// cause reaches the internal log only.
			Cause: fmt.Errorf("no A2A operation named %q", envelope.Method),
		}
	}
	// The operation reported is the map key that produced this chain key: one entry,
	// built in one loop iteration from one operation name, so the attribute and the
	// chain that runs cannot name different operations.
	return Resolution{
		ChainKey:   key,
		Attributes: r.facts.attributes(envelope.Method, envelope.Params.Message),
	}, nil
}

// jsonRPCVersion is the only JSON-RPC version A2A uses.
const jsonRPCVersion = "2.0"

// trimLeadingJSONSpace drops the whitespace JSON permits before a document, so the
// batch check can look at the first structural byte. Only the four characters
// RFC 8259 calls whitespace count; anything else is content and the parser's problem.
func trimLeadingJSONSpace(body []byte) []byte {
	for i, b := range body {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return body[i:]
		}
	}
	return nil
}

// ─── Request attributes ──────────────────────────────────────────────────────

// Attribute names for the facts an A2A route carries forward about a request.
//
// They are namespaced, dot-segmented, because they land in a map shared with every
// other producer of request facts — an unprefixed "taskId" would be a collision
// waiting to happen — and because that is the shape the tracing and analytics
// conventions downstream already use.
//
// The set is deliberately closed and small. Everything here is either a bounded
// protocol fact or a single opaque identifier; the field is not a place to project
// slices of the payload.
const (
	// Identifiers read out of the request. Absent when the request carried none.
	AttrA2AMessageID = "a2a.message.id"
	AttrA2AContextID = "a2a.context.id"
	AttrA2ATaskID    = "a2a.task.id"

	// Protocol facts. Always present on a resolved A2A request, and bounded: the
	// operation comes from the version's closed set, the transport from a
	// two-valued enum, the version from the registry.
	AttrA2AOperation       = "a2a.operation"
	AttrA2ATransport       = "a2a.transport"
	AttrA2AProtocolVersion = "a2a.protocol.version"
)

// a2aRouteFacts are the protocol facts fixed when a route was prepared. Held once per
// prepared resolver rather than recomputed, since neither can change per request.
type a2aRouteFacts struct {
	transport string
	version   string
}

// attributes builds one resolution's attribute set: this route's protocol facts, the
// operation the request resolved to, and whichever identifiers the request carried.
//
// A fresh map every time. A static resolution's map is built once at Prepare and then
// shared by every request on that route, so consumers must treat Attributes as
// read-only either way (see Resolution.Attributes); allocating here is what keeps a
// request-derived map from ever aliasing a prepared one.
func (f a2aRouteFacts) attributes(operation string, message a2aMessage) map[string]string {
	attrs := make(map[string]string, 6)
	attrs[AttrA2AOperation] = operation
	attrs[AttrA2ATransport] = f.transport
	attrs[AttrA2AProtocolVersion] = f.version
	message.addIdentifiers(attrs)
	return attrs
}

// a2aMessage is the subset of the A2A Message the gateway carries forward: the three
// identifiers that correlate a request to a conversation and a task.
//
// lowerCamelCase only. The proto declares these snake_case and a protojson *parser*
// would accept either spelling, but nothing emits the snake_case form — protojson
// itself produces camelCase — and every A2A document in this repo, including the card
// the controller validates, is camelCase. Accepting a second spelling here would be
// the only place in the codebase that did.
type a2aMessage struct {
	MessageID string `json:"messageId"`
	ContextID string `json:"contextId"`
	TaskID    string `json:"taskId"`
}

// addIdentifiers adds whichever of the three identifiers this message carried. A
// message that carried none adds nothing, which is the common case for an operation
// whose payload has no message at all.
//
// Over-long values are dropped rather than truncated. These are opaque identifiers: a
// truncated one is not a shorter identifier, it is a different one, and correlating on
// it would silently group unrelated requests. Dropping leaves the attribute absent,
// which is honest.
func (m a2aMessage) addIdentifiers(attrs map[string]string) {
	add := func(name, value string) {
		if value == "" || len(value) > MaxResolutionAttributeValueBytes {
			return
		}
		attrs[name] = value
	}
	add(AttrA2AMessageID, m.MessageID)
	add(AttrA2AContextID, m.ContextID)
	add(AttrA2ATaskID, m.TaskID)
}

// carriesMessageInBody reports whether an operation's identifiers are reachable only
// from the request body on the HTTP+JSON binding.
//
// Only the two message-sending operations are: everything else in A2A 1.0 addresses a
// task through the path (/tasks/{id}, /tasks/{id}:cancel, …), where the identifier is
// already available at the header phase and costs no buffering at all. Keeping this an
// explicit two-entry predicate rather than "does this binding accept a body" is
// deliberate — CreateTaskPushNotificationConfig has a body too, but nothing in it that
// this carries forward, so buffering it would be pure cost.
func carriesMessageInBody(operation agentproto.Operation) bool {
	return operation == agentproto.SendMessage || operation == agentproto.SendStreamingMessage
}

// ─── HTTP+JSON: one route per operation binding ──────────────────────────────

// preparedA2AStatic is one HTTP+JSON route. Its operation was fixed when the
// controller generated the route, so the whole of its work was done at ingest and
// the request path never builds a view or calls Resolve.
type preparedA2AStatic struct {
	// Even here — where nothing about resolution varies per request — the stated
	// protocol version does, so this route validates headers like every other A2A
	// operation route. Header validation is not resolution: it adds no requirement
	// on the request body and does not stop this route being static.
	a2aVersionGuard

	resolution Resolution
}

// Requirements is the zero value, which it must be: PrepareRoute refuses a static
// resolver that also declares a need for the request, because the static branch is
// taken first and the declared requirement would be skipped with nothing to signal it.
func (*preparedA2AStatic) Requirements() RequestRequirements { return RequestRequirements{} }

// StaticResolution is this route's whole answer, validated once at ingest.
func (r *preparedA2AStatic) StaticResolution() Resolution { return r.resolution }

// Resolve returns the same resolution. Reached only by a caller that ignores
// StaticPreparedResolver; the kernel does not, so this never runs on the request path.
func (r *preparedA2AStatic) Resolve(context.Context, RequestView) (Resolution, error) {
	return r.resolution, nil
}

// preparedA2AHTTPJSONBody is an HTTP+JSON route whose operation is fixed at ingest but
// whose request attributes are only in the body — message:send and message:stream.
//
// It is deliberately *not* a StaticPreparedResolver: a static resolver must declare the
// zero-value requirements, and this one needs the body. The chain it selects is
// nonetheless the same on every request, which drives the failure policy below.
type preparedA2AHTTPJSONBody struct {
	a2aVersionGuard

	chainKey  string
	operation string
}

// Requirements asks for the buffered body. Note what this costs: the body is buffered
// before any policy runs, so on this route buffering precedes authentication (R4). It
// buys the same attributes the JSON-RPC transport gets for free, on the two operations
// where they exist nowhere else.
func (*preparedA2AHTTPJSONBody) Requirements() RequestRequirements {
	return RequestRequirements{Body: BodyBuffered}
}

// Resolve always returns this route's operation, enriched with whatever the body
// yielded.
//
// A missing, unparseable or unexpected body is **not** a failure here, which is the
// opposite of the JSON-RPC route's policy — and the difference is which question the
// body answers. There, the body names the operation, so an unreadable body means
// nothing can be selected. Here the route already fixed the operation, so an unreadable
// body costs only the attributes: the chain still runs, and the Agent — which is the
// component that actually validates A2A payloads — gets to reject the request itself.
// Failing here instead would make the gateway a second, weaker validator whose sterile
// rejection (L2) tells the client less than the Agent's would.
func (r *preparedA2AHTTPJSONBody) Resolve(_ context.Context, view RequestView) (Resolution, error) {
	var message a2aMessage

	// Anything the body cannot yield simply leaves the identifiers absent; the
	// protocol facts below are known from the route regardless.
	if body := trimLeadingJSONSpace(view.Body); len(body) > 0 && body[0] == '{' {
		var payload struct {
			Message a2aMessage `json:"message"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			message = payload.Message
		}
	}

	// A fresh attribute map per request, never a shared one: this resolver holds no
	// prepared Resolution precisely so a request-derived map cannot alias another
	// request's.
	return Resolution{
		ChainKey:   r.chainKey,
		Attributes: r.facts.attributes(r.operation, message),
	}, nil
}

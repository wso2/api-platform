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

// Package resolver derives the policy chain key for a request.
//
// Most API kinds identify their logical operation by HTTP method and path, so the
// Envoy route name already *is* the chain key. Multiplexed transports (A2A
// JSON-RPC, MCP, GraphQL) carry many logical operations on one HTTP route, so the
// operation has to be read out of the request itself.
//
// The split of responsibility is deliberate and load-bearing: a resolver
// identifies the *operation*, and this package composes the chain key from it. A
// resolver never builds a key itself, so there is exactly one construction site per
// process. The controller composes the same key with the same shared helper
// (common/chainkey) when it emits the chains, which is what makes two transports of
// one logical operation select one chain without either being told about the other.
//
//	kind-specific:   request                    ──extract──▶  canonical operation identifier
//	generic:         (apiID, vhost, operation)  ──compose──▶  chain key
package resolver

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/wso2/api-platform/common/chainkey"
)

// RouteKeyResolverName is the registered name of the identity resolver: the chain
// key is the route's own canonical chain key, with no request inspection at all.
// A route with an empty resolver_name is treated identically.
const RouteKeyResolverName = "route-key"

// ProtocolVersion is the operation-resolution wire contract this binary implements.
// It is advertised in the xDS Node metadata so the control plane can withhold
// resolver-bearing routes from a runtime that predates them, instead of sending
// routes whose every request would fail to resolve.
//
// Bump it only for a change that an older runtime would mis-handle rather than
// merely ignore — a new required route field, or changed semantics for an existing
// one. Adding a resolver is covered by the advertised resolver list, not by this.
const ProtocolVersion = 1

// OperationResolver identifies the logical operation a request carries, for routes
// whose HTTP method+path does not determine it. Implementations do not build chain
// keys — see ResolveChainKey.
type OperationResolver interface {
	// Name returns the resolver's registered name; it must match the
	// resolver_name emitted on the wire by the controller.
	Name() string

	// Requirements declares what must be available before Identify is called.
	Requirements() Requirements

	// Identify reads the request and returns the operation(s) it carries.
	Identify(RequestView) (Resolution, error)
}

// Requirements declares what request data a resolver needs before it can run.
// A resolver that needs the body forces the kernel to defer chain selection to
// the request-body phase (see the deferred-binding path in internal/kernel).
type Requirements struct {
	Headers    bool
	BufferBody bool
}

// RequestView is the read-only view of the request handed to a resolver.
type RequestView struct {
	RouteKey string

	// APIID and Vhost are the composition inputs for ChainKeyFor, taken from route
	// metadata the engine already receives. APIContext is here because a path-based
	// resolver needs it to strip the API's prefix before matching.
	//
	// A resolver may read all three but must not build a chain key from them — that
	// is ResolveChainKey's job, so the composition happens in one place.
	APIID      string
	Vhost      string
	APIContext string

	Method     string // upper-cased at extraction (GO-AUTH-006)
	Path       string
	Headers    map[string][]string
	Body       []byte // non-nil only when Requirements().BufferBody
	RouteState any    // whatever Prepare returned for this route; nil if unused
}

// ResponseKind is how an operation's response is delivered. It is a property of the
// *operation*, known once the operation is identified, and it is what keeps a streaming
// operation from being buffered.
//
// Buffering a stream is not a slow path, it is a broken one: the caller receives nothing
// until the upstream closes, which for a long-running agent task can be minutes or never.
// The response body mode therefore cannot be derived from the policy chain alone.
type ResponseKind string

const (
	// ResponseKindAuto means the operation does not declare how its response is
	// delivered, so the engine derives the mode exactly as it did before this field
	// existed: from the chain's capabilities and the shape of the upstream response.
	// Every kind shipping today resolves to this, which is what makes the field additive.
	ResponseKindAuto ResponseKind = ""

	// ResponseKindUnary means one complete response. Safe to buffer, and buffering is
	// what a response-body policy needs, so this is the deterministic form of what Auto
	// usually infers.
	ResponseKindUnary ResponseKind = "unary"

	// ResponseKindStreaming means the response is delivered incrementally (SSE, a
	// long-running task subscription). A chain whose response-body policies cannot run
	// in streaming mode is incompatible with this operation and must not silently
	// buffer it — see the fail-closed check in the kernel's response-header handling.
	ResponseKindStreaming ResponseKind = "streaming"
)

// Valid reports whether k is a kind this binary understands. An unrecognised value from
// a newer controller is not guessed at.
func (k ResponseKind) Valid() bool {
	switch k {
	case ResponseKindAuto, ResponseKindUnary, ResponseKindStreaming:
		return true
	default:
		return false
	}
}

// Resolution is what the request resolved to.
type Resolution struct {
	// Operations carried by this request. Exactly one is supported today; more than
	// one is rejected. The slice exists so a protocol where a single request carries
	// several operations can be added without changing this interface — only the
	// composition rule above it. Its real first consumer is the JSON-RPC batch
	// request, not GraphQL.
	Operations []Operation

	// ProtocolState is optional resolver-owned, already-validated state that a
	// renderer may use to shape an error response (e.g. a validated JSON-RPC
	// request id). It must never carry raw request bytes.
	ProtocolState any

	// ResponseKind is how the identified operation delivers its response. A protocol
	// where one operation streams and its sibling does not (A2A's SendMessage versus
	// SendStreamingMessage) is exactly why this belongs to the resolution rather than to the
	// route: on a multiplexed transport both arrive on the same route.
	//
	// Left at ResponseKindAuto by a resolver that has nothing to say, which is the
	// pre-existing behaviour.
	ResponseKind ResponseKind
}

// Operation is one logical operation, expressed as a specificity ladder: a chain key
// is composed for each candidate in turn and the first that has a chain wins. A kind
// with an unbounded identifier space (MCP tool names) uses this to fall back from a
// specific chain to a generic one.
//
// Note the two shapes are deliberately distinguished:
//   - Candidates within one Operation = alternatives, first hit wins.
//   - The Operations slice = conjunction, all apply (reserved, rejected today).
type Operation struct {
	// Candidates are canonical operation identifiers, most specific first. Each must
	// satisfy chainkey.ValidComponent; a resolver over a user-controlled identifier
	// space must reject an identifier that does not, rather than escape it.
	Candidates []string

	// KnownToProtocol reports that the last candidate is a valid operation of the
	// protocol itself. It decides how "no chain for any candidate" is classified: for
	// a closed operation set (A2A's fixed operation enum) a missing chain means the
	// deployment was built wrong, because the protocol says the operation exists; for
	// an open one (MCP tool names) it means the client named something that does not
	// exist.
	//
	// This is the distinction the controller-supplied operation map used to carry.
	// Answering it from the protocol definition instead does not depend on deployment
	// data being complete.
	KnownToProtocol bool
}

// FailureKind classifies why resolution failed, so the kernel can decide between a
// protocol-shaped response and a sterile generic one without inspecting error text.
type FailureKind string

const (
	// FailureParse means the request payload could not be parsed at all.
	FailureParse FailureKind = "parse"
	// FailureInvalidRequest means the payload parsed but is not a valid request
	// envelope for this protocol (this covers a resolver returning no operation).
	FailureInvalidRequest FailureKind = "invalid-request"
	// FailureUnknownOperation means a well-formed request named an operation the
	// protocol does not define — the client asked for something that does not exist.
	// Distinct from FailureChainMissing, which is a deployment problem.
	FailureUnknownOperation FailureKind = "unknown-operation"
	// FailureMultiOperation means the request carries more than one operation,
	// which no composition rule supports yet.
	FailureMultiOperation FailureKind = "multi-operation-unsupported"
	// FailurePayloadTooLarge means the request body exceeded a configured ceiling
	// before the resolver could run.
	FailurePayloadTooLarge FailureKind = "payload-too-large"
	// FailureUnsupportedEncoding means the request declared a content coding the
	// engine cannot decode, or stacked several. The body is never handed to the
	// resolver still encoded, because it would resolve to whatever the compressed
	// frame happens to look like rather than to the operation the client sent.
	FailureUnsupportedEncoding FailureKind = "unsupported-encoding"
	// FailureUndecodableBody means the request declared a supported content coding
	// but the body does not decode under it.
	FailureUndecodableBody FailureKind = "undecodable-body"
	// FailureUnknownResolver means the route names a resolver this binary does not
	// have. Always rendered generically — never protocol-shaped, since the
	// protocol is exactly what is unknown.
	FailureUnknownResolver FailureKind = "unknown-resolver"
	// FailureStreamingUnsupported means the identified operation streams its response
	// but the chain bound to it cannot run its response-body policies in streaming
	// mode. Like FailureChainMissing this is a deployment problem, not something the
	// caller did, so it renders as a sterile 500 and never as a protocol error.
	FailureStreamingUnsupported FailureKind = "streaming-unsupported"
	// FailureChainMissing means resolution succeeded — the operation is one the
	// protocol defines — but no chain exists under its composed key. That is a
	// controller construction error or xDS skew, not the protocol's "unknown
	// operation" case, so it renders generically rather than as a protocol error the
	// client could act on.
	FailureChainMissing FailureKind = "chain-missing"
	// FailureInternal is every unclassified resolver error.
	FailureInternal FailureKind = "internal"
)

// ResolutionError preserves the reason and any protocol state needed to render a
// correct response. Unclassified resolver errors are wrapped as FailureInternal.
type ResolutionError struct {
	Kind          FailureKind
	ProtocolState any
	Cause         error
}

func (e *ResolutionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("operation resolution failed (%s): %v", e.Kind, e.Cause)
	}
	return fmt.Sprintf("operation resolution failed (%s)", e.Kind)
}

// Unwrap exposes the underlying cause for errors.Is/As. The cause is for internal
// logs only; it is never rendered to a client (error-handling.md directive 1).
func (e *ResolutionError) Unwrap() error { return e.Cause }

// ProtocolVisible reports whether this failure is one the protocol itself can
// describe, and therefore a candidate for a resolver-supplied FailureRenderer.
// Everything else — unknown resolver, payload limits, missing chain, internal —
// uses the kernel's sterile generic response.
func (e *ResolutionError) ProtocolVisible() bool {
	switch e.Kind {
	case FailureParse, FailureInvalidRequest, FailureUnknownOperation:
		return true
	default:
		return false
	}
}

// RenderedFailure is a transport-shaped response body, built by a resolver.
type RenderedFailure struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// FailureRenderer is optional. The kernel uses a sterile generic renderer when the
// selected resolver does not implement it or when no resolver can be selected.
//
// It stays a method on OperationResolver rather than moving to its own route-keyed
// registry because protocol-shaped rendering is scoped to body-resolved transports
// only. Identity routes have no resolver and therefore render raw HTTP.
//
// A renderer may use only resolver-validated ProtocolState; it must not echo
// arbitrary request bytes back to the client.
type FailureRenderer interface {
	RenderFailure(RequestView, *ResolutionError) RenderedFailure
}

// PolicyRejectionRenderer is optional and separate from FailureRenderer: it
// re-renders a rejection produced by a *policy* (auth, rate limit, guardrail) into
// the transport's error shape, preserving the validated protocol state captured at
// resolution time.
//
// The HTTP status is preserved by the caller — only the body (and any headers the
// renderer sets) is replaced. A jwt-auth rejection stays a 401 and a rate-limit
// rejection stays a 429 so access logs, analytics outcomes, and operator
// dashboards stay keyed on a status that still means what it meant.
type PolicyRejectionRenderer interface {
	RenderRejection(view RequestView, protocolState any, in RenderedFailure) RenderedFailure
}

// Preparer is optional. When implemented, the kernel calls Prepare once per route
// at xDS ingest and hands the result back as RequestView.RouteState. A resolver
// that must compile a schema, build an index, or validate configuration does it
// here, not per request.
//
// A Prepare error skips that one route — its requests then take the existing
// sterile 500 path — and increments a failure metric. It must NOT NACK the
// snapshot: under State-of-the-World that keeps the previous version of every
// RouteConfig, so one bad deployment would freeze route updates for every API on
// the gateway.
type Preparer interface {
	Prepare(cfg json.RawMessage) (any, error)
}

// ResolverRegistry is injected into the kernel and the xDS handler. Production uses
// one immutable default instance; tests construct independent registries with fake
// resolvers rather than mutating the production one.
type ResolverRegistry interface {
	Get(name string) (OperationResolver, bool)
	Names() []string
}

// Registry is the concrete ResolverRegistry. It is mutable while being built and
// immutable once frozen, so nothing can register a resolver after the kernel and
// the xDS client have started reading it.
type Registry struct {
	mu     sync.RWMutex
	byName map[string]OperationResolver
	frozen bool
}

// NewRegistry returns an empty, unfrozen registry. Tests use this to build
// independent registries; production uses DefaultRegistry.
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]OperationResolver)}
}

// Register adds a resolver. It fails on a duplicate name (two resolvers answering
// to one wire value is always a build mistake) and on a frozen registry.
func (r *Registry) Register(res OperationResolver) error {
	if res == nil {
		return errors.New("resolver: cannot register a nil resolver")
	}
	name := res.Name()
	if name == "" {
		return errors.New("resolver: cannot register a resolver with an empty name")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return fmt.Errorf("resolver: registry is frozen, cannot register %q", name)
	}
	if _, exists := r.byName[name]; exists {
		return fmt.Errorf("resolver: %q is already registered", name)
	}
	r.byName[name] = res
	return nil
}

// MustRegister is Register for package init() use, where a failure is a build
// error rather than a runtime condition.
func (r *Registry) MustRegister(res OperationResolver) {
	if err := r.Register(res); err != nil {
		panic(err)
	}
}

// Freeze makes the registry immutable. Idempotent.
func (r *Registry) Freeze() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
}

// Frozen reports whether the registry has been frozen.
func (r *Registry) Frozen() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.frozen
}

// Get returns the resolver registered under name.
func (r *Registry) Get(name string) (OperationResolver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res, ok := r.byName[name]
	return res, ok
}

// Names returns every registered resolver name, sorted. Used by the capability
// advertisement and the admin config dump.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.byName))
	for name := range r.byName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// defaultRegistry is the production registry. Resolvers are added to it from
// package init() via RegisterDefault; DefaultRegistry freezes it on first read,
// which happens before the kernel and the xDS client start.
var defaultRegistry = NewRegistry()

// RegisterDefault adds a resolver to the production registry. Intended for
// package init() only — it panics on a duplicate name or after the registry has
// been frozen, both of which are build-time mistakes rather than runtime states.
func RegisterDefault(res OperationResolver) {
	defaultRegistry.MustRegister(res)
}

// DefaultRegistry freezes and returns the production registry. Callers receive the
// read-only interface: nothing outside this package can register into it after
// startup, and tests that need their own resolvers build an independent Registry
// with NewRegistry instead of mutating this one.
func DefaultRegistry() ResolverRegistry {
	defaultRegistry.Freeze()
	return defaultRegistry
}

func init() {
	// The identity resolver is registered for registry symmetry and so it shows up
	// in the capability advertisement and the admin config dump. ResolveChainKey
	// short-circuits identity before ever looking it up.
	RegisterDefault(&RouteKeyResolver{})
}

// RouteResolution is a route's resolution configuration, as delivered by the
// controller over xDS. It is embedded in the kernel's RouteConfig so the fields are
// reachable directly off the route (rc.CanonicalChainKey, rc.ResolverName, …) while
// ResolveChainKey can take a pointer to it without copying per request.
type RouteResolution struct {
	// RouteKey is the Envoy route name (METHOD|fullPath|vhost).
	RouteKey string

	// CanonicalChainKey is the chain key an identity route's requests use, as
	// composed by the controller. It equals RouteKey for every kind shipping today,
	// but it is always read from this field and never reconstructed: that is what
	// lets an identity route be pointed at a composed operation key (an A2A
	// HTTP+JSON operation route) without a wire change.
	//
	// It is not read for a resolver-bearing route, which has no chain of its own.
	CanonicalChainKey string

	// ResolverName is the effective resolver for this route. Empty and
	// RouteKeyResolverName both mean identity.
	ResolverName string

	// ResponseKind is how this route's operation delivers its response, for a route
	// where the operation is known at deploy time and no resolver runs — an A2A
	// HTTP+JSON operation route, one route per operation. A resolver-bearing route
	// leaves this at Auto and the answer comes from Resolution.ResponseKind instead,
	// since there one route carries operations of both kinds.
	ResponseKind ResponseKind

	// ResolverConfig is opaque per-route resolver configuration, passed to
	// Preparer.Prepare at ingest.
	ResolverConfig json.RawMessage

	// RouteState is whatever Prepare returned for this route; nil when the
	// resolver does not implement Preparer.
	RouteState any
}

// IsIdentity reports whether this route resolves its chain by route identity, with
// no request inspection.
func (r *RouteResolution) IsIdentity() bool {
	return r.ResolverName == "" || r.ResolverName == RouteKeyResolverName
}

// ChainKeyFor composes the policy chain key for one operation.
//
// The construction itself lives in common/chainkey, not here: the controller emits
// chains under the same key and cannot import this package (separate module, and this
// one is internal/). This is a re-export so resolver and kernel code reads naturally,
// not a second implementation.
//
// vhost carries the routing partition. A header-match discriminator is deliberately
// not part of the key; a caller that can produce two routes differing only by header
// match must reject that configuration instead.
func ChainKeyFor(apiID, vhost, operation string) string {
	return chainkey.For(apiID, vhost, operation)
}

// ResolveChainKey returns the policy chain key for a request.
//
// The identity path — every kind that ships today — returns before touching
// anything else: no allocation, no composition, no resolver call.
//
// For a resolver-bearing route the key is *composed* from the identified operation
// rather than looked up in controller-supplied data. hasChain reports whether a
// composed key has a chain; it is injected rather than read from a package-level map
// so this function stays testable and the kernel keeps one locking discipline over
// its chain map.
//
// Falling back to identity when resolution fails is forbidden: it would silently
// select a route-level chain for every logical operation and appear to work.
func ResolveChainKey(
	reg ResolverRegistry,
	rc *RouteResolution,
	req RequestView,
	hasChain func(string) bool,
) (string, Resolution, error) {
	if rc.IsIdentity() {
		return rc.CanonicalChainKey, Resolution{}, nil
	}

	r, ok := lookup(reg, rc.ResolverName)
	if !ok {
		return "", Resolution{}, &ResolutionError{Kind: FailureUnknownResolver}
	}

	res, err := r.Identify(req)
	if err != nil {
		return "", res, NormalizeResolutionError(err, res.ProtocolState)
	}
	if len(res.Operations) == 0 {
		return "", res, &ResolutionError{
			Kind: FailureInvalidRequest, ProtocolState: res.ProtocolState,
		}
	}
	if len(res.Operations) > 1 {
		return "", res, &ResolutionError{
			Kind: FailureMultiOperation, ProtocolState: res.ProtocolState,
		}
	}

	// Compose, do not look up: the key is a pure function of the operation, which is
	// what makes the two A2A transports converge without either being told about the
	// other. A candidate that cannot be a key component is skipped rather than
	// composed — composing it could otherwise produce the key of a *different*
	// (apiID, vhost, operation) triple.
	op := res.Operations[0]
	for _, candidate := range op.Candidates {
		if !chainkey.ValidComponent(candidate) {
			continue
		}
		if key := ChainKeyFor(req.APIID, req.Vhost, candidate); hasChain != nil && hasChain(key) {
			return key, res, nil
		}
	}

	// No candidate has a chain. Which failure that is depends on whether the
	// protocol's operation set is closed (see Operation.KnownToProtocol).
	kind := FailureUnknownOperation
	if op.KnownToProtocol {
		kind = FailureChainMissing
	}
	return "", res, &ResolutionError{Kind: kind, ProtocolState: res.ProtocolState}
}

// lookup guards against a nil registry so a partially-wired test or an
// identity-only binary fails closed with FailureUnknownResolver rather than
// panicking on the hot path.
func lookup(reg ResolverRegistry, name string) (OperationResolver, bool) {
	if reg == nil {
		return nil, false
	}
	return reg.Get(name)
}

// NormalizeResolutionError guarantees the kernel always has a typed failure to
// render from. A resolver that returns a *ResolutionError keeps its classification
// (and its own protocol state, if it set one); anything else becomes
// FailureInternal, which renders generically and never reaches the client.
func NormalizeResolutionError(err error, protocolState any) *ResolutionError {
	if err == nil {
		return nil
	}
	var re *ResolutionError
	if errors.As(err, &re) {
		out := *re
		if out.Kind == "" {
			out.Kind = FailureInternal
		}
		if out.ProtocolState == nil {
			out.ProtocolState = protocolState
		}
		return &out
	}
	return &ResolutionError{Kind: FailureInternal, ProtocolState: protocolState, Cause: err}
}

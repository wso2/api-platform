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
// The registry holds resolver *factories*. Each route is prepared once, at xDS
// ingest, into an immutable PreparedResolver that captures everything static about
// that route — its API ID, vhost, context, and its own configuration. Two routes
// prepared by one factory are independent: one may need a buffered body while its
// sibling needs nothing at all. That is what lets a single factory serve both a
// transport that multiplexes every operation onto one route — where the operation is
// only knowable from the request — and one with a route per operation, whose operation
// is fixed at deploy time and needs no request inspection whatsoever.
//
// A prepared resolver composes its own chain key, with the shared helper
// (common/chainkey) and the partition it captured at preparation. What stays central
// is *validation*: this package checks that key against the rules the route's own resolver
// implies, and against the partition captured for it, before the chain is looked up — so a
// resolver cannot reach a chain belonging to another route, API or vhost. The controller composes keys with the same helper when it
// emits the chains, which is what makes two transports of one logical operation select
// one chain without either being told about the other.
//
//	at ingest:    route config  ──prepare──▶  immutable prepared resolver
//	per request:  request       ──resolve──▶  one chain key
//	per request:  chain key     ──validate──▶ bound chain
package resolver

import (
	"context"
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

// Resolver is a registered resolver factory. It holds no per-route state: the
// registry stores one instance per name for the process lifetime, and every route
// that names it gets its own PreparedResolver.
type Resolver interface {
	// Name returns the resolver's registered name; it must match the
	// resolver_name emitted on the wire by the controller.
	Name() string

	// Prepare builds the immutable resolver for one route. It runs once per route at
	// xDS ingest, so a resolver that must validate configuration, compile a schema or
	// build an index does it here rather than per request.
	//
	// An error skips that one route — its requests then take the existing sterile 500
	// path — and increments an ingest failure metric. It must NOT NACK the snapshot:
	// under State-of-the-World that keeps the previous version of every RouteConfig, so
	// one bad deployment would freeze route updates for every API on the gateway.
	Prepare(ResolverRouteConfig) (PreparedResolver, error)
}

// PreparedResolver is one route's resolver, fixed at ingest. Implementations must be
// safe for concurrent use and must not mutate after Prepare returns.
type PreparedResolver interface {
	// Requirements declares what must be available before Resolve is called. It is a
	// property of this prepared route, not of the factory, and cannot be overridden by
	// configuration: letting a transport that must read the body opt out of buffering
	// would make correct resolution impossible.
	Requirements() RequestRequirements

	// Resolve reads the request and returns the chain key it binds to.
	//
	// It must tolerate a RequestView whose Body is nil or empty, even on a route that
	// declared BodyBuffered: a bodyless request (a GET, or any request whose headers are
	// end-of-stream) gets no request-body callback from Envoy, so the kernel resolves at
	// the header phase rather than waiting for a callback that cannot arrive. Treat that
	// as the invalid request it usually is — return a classified *ResolutionError — and
	// never index into Body without checking its length.
	Resolve(context.Context, RequestView) (Resolution, error)
}

// StaticPreparedResolver is an optional optimisation for a route whose resolution is
// entirely known at ingest. The kernel stores the result and neither builds a
// RequestView nor calls Resolve on the request path.
//
// route-key implements it, which is what keeps every kind shipping today on a path
// that costs a field read and a string comparison.
type StaticPreparedResolver interface {
	PreparedResolver

	// StaticResolution returns the resolution every request on this route produces.
	StaticResolution() Resolution
}

// BodyRequirement is whether a prepared resolver needs the request body.
type BodyRequirement uint8

const (
	// BodyNotRequired means the resolver decides from headers, path and its own
	// configuration alone, so its chain is bound at the request-headers callback.
	BodyNotRequired BodyRequirement = iota

	// BodyBuffered means the resolver reads the whole request body, which forces the
	// kernel to defer chain selection to the request-body callback (see the deferred
	// binding path in internal/kernel).
	//
	// It is a request for the body, not a guarantee of one. A request whose headers are
	// end-of-stream carries no body and produces no body callback, so such a request is
	// resolved at the header phase with RequestView.Body nil — see Resolve.
	BodyBuffered
)

// Valid reports whether b is a requirement this binary understands. An unrecognised
// value is rejected at preparation rather than guessed at: guessing "no body" would let a
// resolver that declared it needs the body run without one and select a chain from
// nothing.
func (b BodyRequirement) Valid() bool {
	switch b {
	case BodyNotRequired, BodyBuffered:
		return true
	default:
		return false
	}
}

// String names the requirement for error text.
func (b BodyRequirement) String() string {
	switch b {
	case BodyNotRequired:
		return "not-required"
	case BodyBuffered:
		return "buffered"
	default:
		return fmt.Sprintf("unknown(%d)", uint8(b))
	}
}

// RequestRequirements declares what request data a prepared resolver needs.
//
// A static resolution needs none of it, so a StaticPreparedResolver must declare the
// zero value; PrepareRoute rejects any other combination rather than letting the static
// branch silently win over a stated requirement.
type RequestRequirements struct {
	// Headers is currently advisory: the request view a resolver receives always carries
	// the headers, so nothing in the engine reads this field. It is kept because a
	// resolver declaring its inputs is the contract, and a future path that builds a
	// narrower view would have to honour it.
	Headers bool

	Body BodyRequirement
}

// BuffersBody reports whether this route defers chain selection to the body phase.
//
// Anything other than an explicit BodyNotRequired counts as needing the body. That is
// the conservative direction: providing a body to a resolver that did not want it costs
// a buffered callback, while withholding one from a resolver that did means it resolves
// from nothing. PrepareRoute rejects unrecognised values outright, so this is
// defence-in-depth rather than the primary guard.
func (r RequestRequirements) BuffersBody() bool { return r.Body != BodyNotRequired }

// RequestView is the read-only view of the request handed to a prepared resolver.
//
// It carries only what varies per request. Static partition data (API ID, vhost, API
// context) is not copied in here: the prepared resolver captured it at ingest, which
// is both cheaper and narrower — a resolver cannot be handed a partition that differs
// from the one its keys are validated against.
type RequestView struct {
	RouteKey string

	Method  string // upper-cased at extraction (GO-AUTH-006)
	Path    string
	Headers map[string][]string

	// Body is the decoded request body, populated only for a route that declared
	// BodyBuffered — and not even always then: it is nil when the request had no body at
	// all, because a request whose headers are end-of-stream never reaches a body
	// callback. A BodyBuffered resolver must therefore handle nil and empty alike, and
	// must not assume a non-empty slice (see PreparedResolver.Resolve).
	Body []byte
}

// Resolution is what a prepared resolver made of the request.
type Resolution struct {
	// ChainKey is the one chain this request binds to. Exactly one chain runs per
	// request, so the binder does no selecting — it validates this key and looks it up.
	//
	// How it is validated, and how a missing chain is classified, come from the route's
	// already-prepared resolver rather than from anything in here: a request-time result
	// cannot pick its own validation rule. Nor does it declare whether the operation is
	// one the protocol defines — returning a resolution at all is that claim. A resolver
	// that does not recognise the operation returns FailureUnknownOperation rather than a
	// resolution the binder would have to second-guess.
	//
	// Empty means the resolver could not identify anything to bind to, which is
	// rejected as FailureInvalidRequest.
	ChainKey string

	// Attributes are protocol-derived facts about this request — an A2A message's
	// contextId, an MCP tool's arguments digest — carried forward for telemetry and
	// as policy input.
	//
	// They exist so the request payload is parsed once. A resolver on a multiplexed
	// transport has already decoded the body to find the operation; without somewhere
	// to put what else it saw, every later consumer re-parses the same bytes (the
	// analytics policy alone unmarshals an MCP request body twice today). They are
	// also the only way a value from the body can reach a *header-phase* policy:
	// RequestHeaderContext carries no body, so anything a header-phase policy needs
	// must have been extracted before the chain ran.
	//
	// They select nothing. Bind never reads them, they take no part in key
	// validation, and BoundResolution.Operation stays derived from ChainKey — so
	// nothing in here can influence which chain executes, and a resolver cannot use
	// them to claim one operation while another's policies run.
	//
	// Values are attacker-controlled in the general case: they come out of a request
	// body. ValidateResolution enforces MaxResolutionAttributes and
	// MaxResolutionAttributeValueBytes as a backstop, but a resolver is expected to
	// cap and drop its own values rather than rely on that — tripping the backstop is
	// a resolver bug, not a request outcome.
	//
	// Read-only for consumers. A StaticPreparedResolver builds its resolution once at
	// ingest, so its map is shared by every request on that route; mutating what
	// arrives here would leak one request's data into the next.
	Attributes map[string]string
}

// Limits on Resolution.Attributes.
//
// A resolver runs before authentication on a body-resolved route, so anything it
// retains is retained on behalf of an unauthenticated caller. These are deliberately
// small: the field is for a handful of identifiers, not for slicing up the payload.
const (
	// MaxResolutionAttributes bounds how many attributes one resolution may carry.
	MaxResolutionAttributes = 8

	// MaxResolutionAttributeValueBytes bounds a single attribute value. Identifiers
	// in the protocols this serves are UUID-shaped; anything far larger is a caller
	// stuffing the field rather than naming something.
	MaxResolutionAttributeValueBytes = 256
)

// BoundResolution is the outcome of binding a resolution to a chain that exists.
type BoundResolution struct {
	// ChainKey is the key whose chain was selected.
	ChainKey string

	// Operation is the canonical operation whose chain ran, for telemetry. It is
	// *derived* from ChainKey rather than reported separately by the resolver: with one
	// key per resolution the two cannot legitimately differ, and a resolver that could
	// name a third value would let telemetry say SendMessage while the GetTask chain —
	// its authentication, its rate limits — actually ran.
	//
	// Empty for a direct route: there the route determined the chain, so the resolver
	// identified no operation, and the chain key is already on the span.
	Operation string

	// Attributes are the resolution's protocol-derived request facts, carried through
	// unchanged. Unlike Operation they are not derived from anything — there is
	// nothing to derive them from — so they are passed on exactly as the resolver
	// produced them, after ValidateResolution has bounded them. Nil for a direct
	// route, which inspected no request.
	Attributes map[string]string
}

// FailureKind classifies why resolution failed, so the kernel can pick a status and
// label a metric without inspecting error text. It never reaches the client: every
// failure is answered with the same sterile generic response.
type FailureKind string

const (
	// FailureParse means the request payload could not be parsed at all.
	FailureParse FailureKind = "parse"
	// FailureInvalidRequest means the payload parsed but is not a valid request
	// envelope for this protocol (this covers a resolver returning no chain key).
	FailureInvalidRequest FailureKind = "invalid-request"
	// FailureUnknownOperation means a well-formed request named an operation the
	// protocol does not define — the client asked for something that does not exist.
	// Distinct from FailureChainMissing, which is a deployment problem.
	FailureUnknownOperation FailureKind = "unknown-operation"
	// FailureMultiOperation means the request envelope carries more than one
	// operation (a JSON-RPC batch), which no composition rule supports: one request
	// selects one chain. Raised by the resolver that recognises the envelope, since
	// only it can tell a batch from a single call.
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
	// have, so nothing about the request could be interpreted at all.
	FailureUnknownResolver FailureKind = "unknown-resolver"
	// FailureChainMissing means resolution succeeded — the operation is one the
	// protocol defines — but no chain exists under its composed key. That is a
	// controller construction error or xDS skew, not the protocol's "unknown
	// operation" case — so it answers 500 rather than blaming the caller with a 404.
	FailureChainMissing FailureKind = "chain-missing"
	// FailureInternal is every unclassified resolver error, and every key a resolver
	// returned that this package refused to accept.
	FailureInternal FailureKind = "internal"
)

// ResolutionError carries the classified reason a resolution failed. Unclassified
// resolver errors are wrapped as FailureInternal. The Cause is for internal logs only.
type ResolutionError struct {
	Kind  FailureKind
	Cause error
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

// ResolverRegistry is injected into the kernel and the xDS handler. Production uses
// one immutable default instance; tests construct independent registries with fake
// resolvers rather than mutating the production one.
type ResolverRegistry interface {
	Get(name string) (Resolver, bool)
	Names() []string
}

// Registry is the concrete ResolverRegistry. It is mutable while being built and
// immutable once frozen, so nothing can register a resolver after the kernel and
// the xDS client have started reading it.
type Registry struct {
	mu     sync.RWMutex
	byName map[string]Resolver
	frozen bool
}

// NewRegistry returns an empty, unfrozen registry. Tests use this to build
// independent registries; production uses DefaultRegistry.
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]Resolver)}
}

// Register adds a resolver. It fails on a duplicate name (two resolvers answering
// to one wire value is always a build mistake) and on a frozen registry.
func (r *Registry) Register(res Resolver) error {
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
func (r *Registry) MustRegister(res Resolver) {
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

// Get returns the resolver factory registered under name.
func (r *Registry) Get(name string) (Resolver, bool) {
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
func RegisterDefault(res Resolver) {
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
	// The identity resolver every kind shipping today prepares to. It is a real
	// registry entry, not a special case: PrepareRoute normalises an empty
	// resolver_name to this name and prepares it like any other.
	RegisterDefault(&RouteKeyResolver{})
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

// NormalizeResolutionError guarantees the kernel always has a typed failure to
// classify. A resolver that returns a *ResolutionError keeps its classification;
// anything else becomes FailureInternal, which renders as the generic sterile response
// and never reaches the client.
func NormalizeResolutionError(err error) *ResolutionError {
	if err == nil {
		return nil
	}
	if re, ok := errors.AsType[*ResolutionError](err); ok {
		out := *re
		if out.Kind == "" {
			out.Kind = FailureInternal
		}
		return &out
	}
	return &ResolutionError{Kind: FailureInternal, Cause: err}
}

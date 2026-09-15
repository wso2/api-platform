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

	"github.com/wso2/api-platform/common/chainkey"
)

// ErrDirectRouteChainMissing reports that a direct-route resolution named a chain that
// does not exist. It is deliberately not a *ResolutionError: this is the pre-existing
// "route has no policy chain" condition, whose response is the kernel's own sterile
// 500, and classifying it as a resolution failure would change what every kind
// shipping today returns.
var ErrDirectRouteChainMissing = errors.New("resolver: no policy chain for this route")

// ResolverRouteConfig is one route's static resolution configuration, as delivered by
// the controller and normalised by xDS ingest. It is deployment-scoped data, not a
// request context — the name is deliberate: context.Context remains the
// cancellation/deadline carrier passed to request-time Resolve.
type ResolverRouteConfig struct {
	// RouteKey is the Envoy route name (METHOD|fullPath|vhost).
	RouteKey string

	// CanonicalChainKey is the effective direct chain key, meaningful only for a
	// directly-resolved route. Ingest applies the older-controller fallback to RouteKey
	// when the wire field is absent, so it is never empty and every consumer — a
	// Prepare implementation and the binder's route-key check — reads the same
	// value from one place.
	//
	// A protocol-resolved route carries no key on the wire: it derives one from
	// ResolverConfig, so this holds the fallback value and nothing reads it.
	CanonicalChainKey string

	// ResolverName is the effective resolver name, already normalised: never empty.
	ResolverName string

	// APIID and Vhost are the route's partition, and the composition inputs a prepared
	// resolver captures for ChainKeyFor. APIContext is here because a path-based
	// resolver needs it to strip the API's prefix before matching.
	APIID      string
	Vhost      string
	APIContext string

	Method string // upper-cased at ingest (GO-AUTH-006), like RequestView.Method
	Path   string

	// ResolverConfig is opaque per-route resolver configuration.
	ResolverConfig json.RawMessage
}

// RouteResolution is a route's resolution configuration as it arrived over xDS, plus
// the prepared resolver built from it. It is embedded in the kernel's RouteConfig so
// the fields are reachable directly off the route (rc.CanonicalChainKey, rc.Prepared,
// …) without copying the struct per request.
type RouteResolution struct {
	// RouteKey is the Envoy route name (METHOD|fullPath|vhost).
	RouteKey string

	// CanonicalChainKey is the effective direct chain key for this route, as composed
	// by the controller. It equals RouteKey for every kind shipping today, but it is
	// always read from this field and never reconstructed: that is what lets a directly
	// resolved route be pointed at a composed operation key without a wire change.
	CanonicalChainKey string

	// ResolverName is the effective resolver for this route. Empty and
	// RouteKeyResolverName both mean identity.
	ResolverName string

	// ResolverConfig is opaque per-route resolver configuration, passed to Prepare.
	ResolverConfig json.RawMessage

	// Prepared is the immutable result of preparing this route. Nil only for a route
	// that failed preparation, which ingest drops rather than serving.
	Prepared *PreparedRoute
}

// IsIdentity reports whether this route resolves its chain by route identity, with
// no request inspection.
func (r *RouteResolution) IsIdentity() bool {
	return r.ResolverName == "" || r.ResolverName == RouteKeyResolverName
}

// PreparedRoute is what the kernel stores per route: the immutable prepared resolver,
// the requirements it declared, and the static inputs the binder validates against.
// Everything here is fixed at ingest — nothing in it is derived per request.
type PreparedRoute struct {
	// ResolverName is the effective (normalised) resolver name, for logs and metrics.
	ResolverName string

	Resolver     PreparedResolver
	Requirements RequestRequirements

	// StaticResolution is non-nil when the prepared resolver implements
	// StaticPreparedResolver. Its presence is what lets the request path skip building
	// a RequestView and calling Resolve at all.
	StaticResolution *Resolution

	// HeaderValidator is non-nil when the prepared resolver implements
	// HeaderValidatingPreparedResolver.
	//
	// Held as the typed value rather than re-asserted per request, so the interface
	// check is paid once at ingest, and a route without one costs a nil comparison.
	// A static route may carry one: header validation is not resolution, so needing
	// it does not contradict knowing the resolution already (see validateRequirements,
	// which governs the request data Resolve needs and deliberately does not govern
	// this).
	HeaderValidator HeaderValidatingPreparedResolver

	// DirectChainKey is the effective CanonicalChainKey for this route, and
	// APIID/Vhost are its partition. Held here so key validation compares against
	// captured values rather than re-deriving anything on the request path.
	DirectChainKey string
	APIID          string
	Vhost          string
}

// PrepareRoute normalises the resolver name, looks the factory up and prepares this
// one route.
//
// An unknown resolver is reported as a *ResolutionError of kind
// FailureUnknownResolver so the caller can tell it apart from a resolver's own
// preparation failure; every other error is the resolver's, returned as-is.
func PrepareRoute(reg ResolverRegistry, cfg ResolverRouteConfig) (*PreparedRoute, error) {
	if cfg.ResolverName == "" {
		cfg.ResolverName = RouteKeyResolverName
	}

	factory, ok := lookupFactory(reg, cfg.ResolverName)
	if !ok {
		return nil, &ResolutionError{Kind: FailureUnknownResolver}
	}

	prepared, err := factory.Prepare(cfg)
	if err != nil {
		return nil, err
	}
	if prepared == nil {
		return nil, fmt.Errorf("resolver %q prepared a nil resolver for route %q",
			cfg.ResolverName, cfg.RouteKey)
	}

	static, isStatic := prepared.(StaticPreparedResolver)
	reqs := prepared.Requirements()
	if err := validateRequirements(reqs, isStatic); err != nil {
		return nil, fmt.Errorf("resolver %q declared unusable requirements for route %q: %w",
			cfg.ResolverName, cfg.RouteKey, err)
	}

	// Checked once, here, so the request path never type-asserts. A resolver either
	// validates headers for every request on this route or for none of them: making
	// it a property of the prepared route is what keeps it from being something a
	// request could influence.
	validator, _ := prepared.(HeaderValidatingPreparedResolver)

	pr := &PreparedRoute{
		ResolverName:    cfg.ResolverName,
		Resolver:        prepared,
		Requirements:    reqs,
		HeaderValidator: validator,
		// Captured once, from the already-resolved effective value, so the binder's
		// route-key check never re-derives the fallback.
		DirectChainKey: cfg.CanonicalChainKey,
		APIID:          cfg.APIID,
		Vhost:          cfg.Vhost,
	}
	if isStatic {
		resolution := static.StaticResolution()
		// Validated here, once, rather than on every request: a static resolution is
		// immutable, so an invalid one is a broken route, not a bad request. Catching it
		// at ingest drops that one route with a metric instead of failing every request
		// to it with a 500 — and it is what lets the request path skip validation
		// entirely (see BindStatic).
		if err := pr.ValidateResolution(resolution); err != nil {
			return nil, fmt.Errorf("resolver %q prepared an invalid static resolution for route %q: %w",
				cfg.ResolverName, cfg.RouteKey, err)
		}
		pr.StaticResolution = &resolution
	}
	return pr, nil
}

// validateRequirements refuses requirements the engine cannot honour.
//
// Two ways a resolver can declare something the request path would silently not deliver:
//
//   - An unrecognised BodyRequirement. It is not guessed at, because a wrong guess in the
//     lenient direction means a resolver that asked for the body selects a chain without
//     one.
//   - A static resolution paired with any request-dependent requirement. The static branch
//     is taken before the body-buffering check, so the declared requirement would be
//     skipped with nothing to signal it. A static resolution is by definition complete
//     without the request, so needing request data contradicts being static — which makes
//     this a construction error rather than a combination to arbitrate.
func validateRequirements(reqs RequestRequirements, isStatic bool) error {
	if !reqs.Body.Valid() {
		return fmt.Errorf("unrecognised body requirement %s", reqs.Body)
	}
	if isStatic && reqs != (RequestRequirements{}) {
		return fmt.Errorf(
			"a static resolution needs nothing from the request, but this one requires headers=%t body=%s",
			reqs.Headers, reqs.Body)
	}
	return nil
}

// lookupFactory resolves a resolver name, treating a nil registry as identity-only.
//
// Identity is answered from this package rather than the registry, so a partially-wired
// server still serves every kind that resolves by route key instead of dropping every
// route on the gateway. Anything else fails closed: no protocol resolver is ever
// substituted for another.
func lookupFactory(reg ResolverRegistry, name string) (Resolver, bool) {
	if reg == nil {
		if name == RouteKeyResolverName {
			return &RouteKeyResolver{}, true
		}
		return nil, false
	}
	return reg.Get(name)
}

// IsStatic reports whether this route's resolution was fully known at ingest, so the
// request path binds from the stored result without building a RequestView.
func (pr *PreparedRoute) IsStatic() bool {
	return pr != nil && pr.StaticResolution != nil
}

// ValidatesHeaders reports whether this route's resolver added a header-validation
// phase, so the kernel can skip building a header view for the routes that did not.
func (pr *PreparedRoute) ValidatesHeaders() bool {
	return pr != nil && pr.HeaderValidator != nil
}

// ValidateRequestHeaders runs the route's header-validation phase, if it has one, and
// returns the classified refusal or nil.
//
// It normalises whatever the resolver returned, exactly as the operation-resolution
// path does: a resolver that returns a bare error still produces a sterile response
// rather than an unclassified one the kernel would have to guess a status for.
//
// A route with no validator returns nil without calling anything, so this is safe to
// call unconditionally — though the kernel checks ValidatesHeaders first, to avoid
// building the view at all.
func (pr *PreparedRoute) ValidateRequestHeaders(ctx context.Context, view HeaderRequestView) *ResolutionError {
	if pr == nil || pr.HeaderValidator == nil {
		return nil
	}
	return NormalizeResolutionError(pr.HeaderValidator.ValidateHeaders(ctx, view))
}

// ValidateResolution checks a resolution's structure against this route: it names a key,
// and that key passes the rules this route's own resolver implies. It touches no chain map,
// so it is safe to run at ingest.
func (pr *PreparedRoute) ValidateResolution(res Resolution) *ResolutionError {
	if pr == nil {
		return &ResolutionError{
			Kind:  FailureInternal,
			Cause: errors.New("route was never prepared"),
		}
	}
	if res.ChainKey == "" {
		return &ResolutionError{
			Kind:  FailureInvalidRequest,
			Cause: errors.New("resolution named no chain key"),
		}
	}
	if err := pr.validateResolvedKey(res.ChainKey); err != nil {
		// FailureInternal renders generically, so the client learns nothing from a
		// resolver's own bug. There is nothing to fall back to — one resolution names one
		// key — so an invalid key fails the whole resolution.
		return &ResolutionError{Kind: FailureInternal, Cause: err}
	}
	if err := validateResolutionAttributes(res.Attributes); err != nil {
		return &ResolutionError{Kind: FailureInternal, Cause: err}
	}
	return nil
}

// validateResolutionAttributes bounds what a resolver may carry alongside the key.
//
// This is a backstop against a resolver bug, not a request-validation step: a resolver
// reading attacker-controlled values is expected to cap and drop them itself, so
// tripping this means the resolver did not. It fails the whole resolution rather than
// silently trimming, because a resolution carrying more than it should is not one whose
// remaining contents can be trusted — and quietly dropping the excess would hide the bug
// exactly where it matters, on the unauthenticated path.
func validateResolutionAttributes(attrs map[string]string) error {
	if len(attrs) == 0 {
		return nil
	}
	if len(attrs) > MaxResolutionAttributes {
		return fmt.Errorf("resolution carries %d attributes, more than the %d permitted",
			len(attrs), MaxResolutionAttributes)
	}
	for name, value := range attrs {
		if name == "" {
			return errors.New("resolution carries an unnamed attribute")
		}
		if len(value) > MaxResolutionAttributeValueBytes {
			// The value itself is never quoted: it is caller-controlled, and this
			// error reaches the internal log.
			return fmt.Errorf("resolution attribute %q is %d bytes, over the %d-byte limit",
				name, len(value), MaxResolutionAttributeValueBytes)
		}
	}
	return nil
}

// Bind validates a resolution and looks up its chain. Use it for a resolution produced per
// request; a route's static resolution was validated at ingest and goes through BindStatic
// instead.
//
// getChain returns the chain itself rather than reporting existence, so binding costs
// exactly one lookup and the selected chain cannot be evicted between a probe and a read.
// It is injected rather than handed to resolvers, so the kernel keeps a single locking
// discipline over its chain map: a resolver never decides whether a chain exists and never
// executes one.
//
// The returned error is nil, ErrDirectRouteChainMissing, or a *ResolutionError — callers
// switch on those three and must not re-normalise it.
func Bind[C any](pr *PreparedRoute, res Resolution, getChain func(string) *C) (BoundResolution, *C, error) {
	if err := pr.ValidateResolution(res); err != nil {
		return BoundResolution{}, nil, err
	}
	return selectChain(pr, res, getChain)
}

// BindStatic looks up the chain for this route's static resolution, the one fixed at
// ingest. It performs no validation: PrepareRoute already validated that resolution and
// it cannot have changed since, so the request path does no structural work at all — one
// chain lookup and a struct copy.
//
// It takes no resolution argument on purpose: the only resolution it can bind is the
// validated one, so there is nothing to pass that could differ from it.
func BindStatic[C any](pr *PreparedRoute, getChain func(string) *C) (BoundResolution, *C, error) {
	if pr == nil || pr.StaticResolution == nil {
		return BoundResolution{}, nil, &ResolutionError{
			Kind:  FailureInternal,
			Cause: errors.New("route has no static resolution"),
		}
	}
	return selectChain(pr, *pr.StaticResolution, getChain)
}

// isDirectlyResolved reports whether this route's chain comes from the route itself rather
// than from a protocol resolver reading the request.
//
// It is answered from the normalised resolver name fixed at preparation, which is what
// keeps a request-time result from choosing its own semantics: the same resolution binds,
// validates and classifies the same way on every request to this route.
func (pr *PreparedRoute) isDirectlyResolved() bool {
	return pr.ResolverName == RouteKeyResolverName
}

// selectChain looks up the resolution's chain and builds the bound result, or classifies
// why no chain was found. A free function rather than a method because Go does not allow
// methods to carry their own type parameter.
func selectChain[C any](pr *PreparedRoute, res Resolution, getChain func(string) *C) (BoundResolution, *C, error) {
	if getChain == nil {
		// Without an accessor the chain would appear not to exist, which on a
		// protocol-resolved route would report deployment skew — a misdiagnosis of what is
		// really an engine wiring fault.
		return BoundResolution{}, nil, &ResolutionError{
			Kind:  FailureInternal,
			Cause: errors.New("no chain accessor was provided"),
		}
	}

	if chain := getChain(res.ChainKey); chain != nil {
		return BoundResolution{
			ChainKey:  res.ChainKey,
			Operation: pr.operationFor(res.ChainKey),
			// Carried through as produced. A direct route inspected no request, so a
			// route-key resolver contributes nothing here.
			Attributes: res.Attributes,
		}, chain, nil
	}

	// A directly-resolved route keeps the pre-resolution outcome: the kernel's own sterile
	// 500 for a route with no chain.
	if pr.isDirectlyResolved() {
		return BoundResolution{}, nil, ErrDirectRouteChainMissing
	}

	// Resolve returned a resolution at all, so the protocol resolver has already
	// established that this is one of its operations — an unrecognised one comes back as
	// FailureUnknownOperation from Resolve, never as a resolution to be second-guessed
	// here. Its generated chain being absent is therefore deployment or xDS skew.
	return BoundResolution{}, nil, &ResolutionError{Kind: FailureChainMissing}
}

// validateResolvedKey enforces the boundary the route's own resolver implies. A resolver
// composes its own key; this is what stops a composition bug or a hostile identifier from
// reaching a chain that belongs to a different route, API or vhost. Callers check for an
// empty key first.
func (pr *PreparedRoute) validateResolvedKey(key string) error {
	if pr.isDirectlyResolved() {
		// Compared against the value captured at preparation, never re-derived: one
		// fallback site, so a second one cannot disagree with it.
		if key != pr.DirectChainKey {
			return errors.New("route-key resolver must return the route's own chain key")
		}
		return nil
	}

	apiID, vhost, operation, ok := chainkey.Split(key)
	if !ok || operation == "" {
		return errors.New("protocol resolver returned a malformed operation chain key")
	}
	if apiID != pr.APIID || vhost != pr.Vhost {
		return errors.New("protocol resolver crossed an API or routing partition")
	}
	return nil
}

// operationFor reports the canonical operation the selected chain serves, read back out of
// the key that was validated and looked up.
//
// Deriving it here is what makes the reported operation and the executed chain the same
// fact: a resolver has no field with which to claim a different one, so telemetry cannot
// name one operation while another operation's policies run.
//
// A directly-resolved route reports nothing. Its key is the route's own — not necessarily a
// composed one at all — and no resolver identified an operation, so attributing one to it
// would misreport who chose the chain.
func (pr *PreparedRoute) operationFor(key string) string {
	if pr.isDirectlyResolved() {
		return ""
	}
	_, _, operation, _ := chainkey.Split(key) // validated before the lookup
	return operation
}

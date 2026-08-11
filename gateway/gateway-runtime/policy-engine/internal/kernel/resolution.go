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

package kernel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/tracing"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// routeBindOutcome describes what the request-headers callback established about a
// request's policy chain.
type routeBindOutcome int

const (
	// bindReady means a policy chain is selected and the execution context is ready
	// to process phases normally. Every kind shipping today takes this path.
	bindReady routeBindOutcome = iota

	// bindPending means the route's resolver must read the request body, so no chain
	// is selected yet: the execution context exists with a nil chain, no policy has
	// run, and Envoy is asked to buffer the body and come back.
	bindPending

	// bindNoChain means there is no RouteConfig for this route key, or the route
	// resolves by identity and has no policy chain. This is the pre-existing sterile
	// 500 path and its response must stay byte-identical.
	bindNoChain

	// bindFailed means resolution ran and failed. The response is protocol-shaped
	// when the resolver supplies a renderer and the failure is one the protocol can
	// describe, and sterile-generic otherwise.
	bindFailed
)

// resolutionDenial carries everything the request-headers callback needs to render
// a resolution failure, captured at the point of failure so the caller never has to
// look the resolver up a second time (and cannot look up a different one).
type resolutionDenial struct {
	resolverName  string
	view          resolver.RequestView
	protocolState any
	renderer      resolver.FailureRenderer
	failure       *resolver.ResolutionError
}

// newResolutionDenial records a failure together with the resolver's own renderer,
// if it has one.
func newResolutionDenial(
	resolverName string,
	res resolver.OperationResolver,
	view resolver.RequestView,
	protocolState any,
	failure *resolver.ResolutionError,
) *resolutionDenial {
	d := &resolutionDenial{
		resolverName:  resolverName,
		view:          view,
		protocolState: protocolState,
		failure:       failure,
	}
	if fr, ok := res.(resolver.FailureRenderer); ok {
		d.renderer = fr
	}
	return d
}

// pendingResolution is the retained state of a request whose policy chain cannot be
// selected at the request-headers callback because the route's resolver must read
// the request body.
//
// Nothing is forwarded upstream while this is set, so ordering guarantees relative
// to the backend still hold: the resolved chain's request-header policies simply run
// one callback later than they would on an identity route.
type pendingResolution struct {
	route        *RouteConfig
	res          resolver.OperationResolver
	requirements resolver.Requirements

	// view is the header-phase request view, retained so the resolver observes the
	// headers, method and path as the client actually sent them rather than values
	// re-derived at the body callback.
	view resolver.RequestView
}

// buildRequestView snapshots what a resolver is allowed to see about a request.
// Only called for non-identity routes: an identity route resolves from a field read
// and must not pay for this allocation (see invariant 5.1).
func buildRequestView(routeKey string, rc *RouteConfig, headers *extprocv3.HttpHeaders) resolver.RequestView {
	view := resolver.RequestView{
		RouteKey: routeKey,
		// The composition inputs for the chain key, taken from metadata xDS already
		// populated on the route — nothing new is parsed per request. A resolver reads
		// APIContext to strip the API prefix from a path; it must not compose a key
		// itself (resolver.ResolveChainKey does that, once).
		APIID:      rc.Metadata.APIId,
		Vhost:      rc.Metadata.Vhost,
		APIContext: rc.Metadata.Context,
		RouteState: rc.RouteState,
	}

	if headers == nil || headers.Headers == nil {
		return view
	}

	hdrs := make(map[string][]string, len(headers.Headers.GetHeaders()))
	for _, h := range headers.Headers.GetHeaders() {
		value := string(h.RawValue)
		hdrs[h.Key] = append(hdrs[h.Key], value)
		switch h.Key {
		case ":method":
			// Normalized once, here, so no downstream comparison or map lookup can
			// miss on case (GO-AUTH-006).
			view.Method = strings.ToUpper(value)
		case ":path":
			view.Path = value
		}
	}
	view.Headers = hdrs
	return view
}

// attachRenderers records the optional renderers this resolver supplies, so a
// resolution failure and a later policy rejection can both be shaped by the
// transport. Both stay nil for a resolver that implements neither, which is what
// keeps rendering a structural no-op elsewhere.
func (ec *PolicyExecutionContext) attachRenderers(res resolver.OperationResolver) {
	if fr, ok := res.(resolver.FailureRenderer); ok {
		ec.failureRenderer = fr
	}
	if rr, ok := res.(resolver.PolicyRejectionRenderer); ok {
		ec.rejectionRenderer = rr
	}
}

// pendingModeOverride is the ProcessingMode returned from the request-headers
// callback of a route whose chain is not selected yet.
//
// It is derived from the resolver's Requirements, not from a policy chain — there
// is no chain yet. That is also why no controller-side ExtProcPerRoute processing
// mode override (or equivalent route flag) is used for this: it would duplicate
// Requirements().BufferBody in a second place that can disagree with it.
//
// Response modes are left at NONE and revisited from the response-header callback,
// once the chain is known: Envoy applies a ModeOverride only on responses to header
// callbacks, and the response-header callback still precedes its decision about how
// to deliver the upstream response body.
func pendingModeOverride(reqs resolver.Requirements) *extprocconfigv3.ProcessingMode {
	mode := &extprocconfigv3.ProcessingMode{
		ResponseHeaderMode:  extprocconfigv3.ProcessingMode_SEND,
		RequestTrailerMode:  extprocconfigv3.ProcessingMode_SKIP,
		ResponseTrailerMode: extprocconfigv3.ProcessingMode_SKIP,
		ResponseBodyMode:    extprocconfigv3.ProcessingMode_NONE,
	}
	if reqs.BufferBody {
		mode.RequestBodyMode = extprocconfigv3.ProcessingMode_BUFFERED
	} else {
		mode.RequestBodyMode = extprocconfigv3.ProcessingMode_NONE
	}
	return mode
}

// pendingResolutionResponse is the request-headers response for a deferred route:
// no mutation, no policy result, no analytics — only the instruction to buffer the
// body and call back.
func pendingResolutionResponse(reqs resolver.Requirements) *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{},
		},
		ModeOverride: pendingModeOverride(reqs),
	}
}

// bindPendingChainAndProcess runs at the request-body callback of a deferred route.
// It enforces the body ceilings, resolves the operation from the decoded bytes,
// binds the chain, and then runs the chain's request-header policies followed by its
// request-body policies — all emitted on this one body-phase response.
func (ec *PolicyExecutionContext) bindPendingChainAndProcess(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	pending := ec.pending
	wire := body.GetBody()

	// Acceptance ceiling, checked before the body is decompressed or parsed. On a
	// body-resolved route none of that work is authenticated — the chain holding
	// jwt-auth is what we are still trying to find — so an unauthenticated caller
	// controls how much of it happens.
	//
	// This bounds the work, not the buffering: BUFFERED mode means Envoy has already
	// collected the whole body and sent it here in one message by the time this runs.
	// The memory an unauthenticated caller can pin is bounded by Envoy's listener-wide
	// per_connection_buffer_limit_bytes and the ext_proc gRPC receive limit, neither of
	// which is per-route. See RouteConfig.MaxRequestBodyBytes and §8 R3.
	if limit := pending.route.EffectiveMaxRequestBodyBytes(); int64(len(wire)) > limit {
		return ec.denyResolution(ctx, &resolver.ResolutionError{
			Kind: resolver.FailurePayloadTooLarge,
			// The configured limit is never echoed to the client (file-access.md
			// directive 5); only the observed size is logged internally.
			Cause: fmt.Errorf("request body of %d wire bytes exceeds this route's acceptance limit", len(wire)),
		}), nil
	}

	decoded, decodeFailure := ec.decodeResolverRequestBody(wire, pending.view.Headers)
	if decodeFailure != nil {
		return ec.denyResolution(ctx, decodeFailure), nil
	}

	view := pending.view
	view.Body = decoded

	key, resolution, err := resolver.ResolveChainKey(
		ec.server.resolvers, &pending.route.RouteResolution, view, ec.server.hasPolicyChain)
	if err != nil {
		ec.requestView = view
		ec.protocolState = resolution.ProtocolState
		return ec.denyResolution(ctx, resolver.NormalizeResolutionError(err, resolution.ProtocolState)), nil
	}

	chain := ec.server.kernel.GetPolicyChain(key)
	if chain == nil {
		// ResolveChainKey only returns a key whose chain existed when it probed, so
		// reaching here means the chain was removed between the probe and this read —
		// two separate acquisitions of the kernel's lock. Rare, but it renders the
		// same way as any other skew: generically, never as a protocol-level error the
		// client could act on.
		ec.requestView = view
		ec.protocolState = resolution.ProtocolState
		return ec.denyResolution(ctx, &resolver.ResolutionError{
			Kind:  resolver.FailureChainMissing,
			Cause: fmt.Errorf("no policy chain for resolved key"),
		}), nil
	}

	// Bind.
	ec.policyChain = chain
	ec.chainKey = key
	ec.requestView = view
	ec.protocolState = resolution.ProtocolState
	ec.responseKind = effectiveResponseKind(pending.route, resolution)
	ec.attachRenderers(pending.res)
	ec.pending = nil
	ec.boundAtBodyPhase = true

	slog.DebugContext(ctx, "[resolution] chain bound at request-body phase",
		"route", ec.routeKey, "resolver", ec.resolverName, "chain_key", key,
		"decoded_bytes", len(decoded))

	// Reuse the decoded body for the body policies so it is never decompressed
	// twice. EndOfStream is true here by construction: the mode is BUFFERED, so
	// Envoy delivers the whole body in one callback.
	ec.requestBodyCtx.Body = &policy.Body{
		Content:     decoded,
		EndOfStream: body.GetEndOfStream(),
		Present:     true,
	}

	headerResult, err := ec.server.executor.ExecuteRequestHeaderPolicies(
		ctx,
		ec.policyChain.Policies,
		ec.requestHeaderCtx,
		ec.policyChain.PolicySpecs,
		ec.sharedCtx.APIName,
		ec.routeKey,
		ec.policyChain.HasExecutionConditions,
	)
	if err != nil {
		return ec.handlePolicyError(ctx, err, "request_headers_deferred"), nil
	}

	if !headerResult.ShortCircuited {
		applyRequestHeaderMutations(ec.requestHeaderCtx.Headers, headerResult.Results)
		ec.syncRequestPseudoHeaders()
	}

	// An empty body result is still a valid merge input: a chain with no body policy
	// contributes no body mutation, and the header-phase mutations it did produce
	// still have to be emitted on this response.
	bodyResult := &executor.RequestExecutionResult{}
	if !headerResult.ShortCircuited && ec.policyChain.RequiresRequestBody {
		bodyResult, err = ec.server.executor.ExecuteRequestPolicies(
			ctx,
			ec.policyChain.Policies,
			ec.requestBodyCtx,
			ec.policyChain.PolicySpecs,
			ec.sharedCtx.APIName,
			ec.routeKey,
			ec.policyChain.HasExecutionConditions,
		)
		if err != nil {
			return ec.handlePolicyError(ctx, err, "request_body"), nil
		}
	}

	return TranslateRequestBodyActionsWithHeaderMerge(headerResult, bodyResult, ec)
}

// denyResolution builds the response for a request whose operation could not be
// resolved to a policy chain, and marks the execution context as never having bound
// one.
//
// Falling back to identity resolution here is forbidden: it would select the
// route-level chain for every logical operation on a multiplexed route and appear
// to work, which is exactly the silent policy bypass this whole mechanism exists to
// prevent.
func (ec *PolicyExecutionContext) denyResolution(
	ctx context.Context,
	failure *resolver.ResolutionError,
) *extprocv3.ProcessingResponse {
	resp, outcome := renderResolutionFailure(ctx, ec.resolverName, ec.routeKey, ec.requestID,
		ec.requestView, ec.protocolState, ec.failureRenderer, failure)

	// The chain is never bound for this request. Later phases check this so a
	// response callback that arrives anyway cannot dereference a nil chain.
	ec.pending = nil
	ec.policyChain = nil
	ec.resolutionDenied = true
	ec.generated = generatedResponse{resp: resp, outcome: outcome}
	ec.terminal = outcome
	return resp
}

// renderResolutionFailure turns a typed resolution failure into an ext_proc
// ImmediateResponse plus the span outcome that describes it.
//
// A protocol-visible failure (parse, invalid request, unknown operation) is handed
// to the resolver's FailureRenderer when it has one, so a JSON-RPC client gets a
// JSON-RPC error object instead of a body its library cannot parse. Everything else
// — unknown resolver, payload limits, a resolved-but-missing chain, an internal
// error — is sterile and generic. The failure's Cause is logged, never returned
// (error-handling.md directive 1).
func renderResolutionFailure(
	ctx context.Context,
	resolverName string,
	routeKey string,
	requestID string,
	view resolver.RequestView,
	protocolState any,
	renderer resolver.FailureRenderer,
	failure *resolver.ResolutionError,
) (*extprocv3.ProcessingResponse, tracing.HTTPOutcome) {
	errorID := uuid.New().String()

	slog.WarnContext(ctx, "Operation resolution failed",
		"error_id", errorID,
		"request_id", requestID,
		"route", routeKey,
		"resolver", resolverName,
		"kind", string(failure.Kind),
		"error", failure.Cause,
	)
	metrics.ResolutionFailuresTotal.WithLabelValues(resolverName, string(failure.Kind)).Inc()

	rendered, protocolShaped := resolutionFailureBody(view, protocolState, renderer, failure, errorID)

	imm := &extprocv3.ImmediateResponse{
		Status:  &typev3.HttpStatus{Code: typev3.StatusCode(rendered.StatusCode)},
		Headers: buildHeaderValueOptions(rendered.Headers),
		Body:    rendered.Body,
	}
	resp := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{ImmediateResponse: imm},
	}

	reason := constants.TerminalReasonResolutionFailed
	if protocolShaped {
		reason = constants.TerminalReasonResolutionFailedProtocol
	}
	return resp, tracing.HTTPOutcome{
		StatusCode: rendered.StatusCode,
		Reason:     reason,
		ErrorID:    errorID,
	}
}

// resolutionFailureBody picks between the resolver's protocol-shaped rendering and
// the sterile generic one, and reports which was used.
func resolutionFailureBody(
	view resolver.RequestView,
	protocolState any,
	renderer resolver.FailureRenderer,
	failure *resolver.ResolutionError,
	errorID string,
) (resolver.RenderedFailure, bool) {
	if renderer != nil && failure.ProtocolVisible() {
		withState := *failure
		if withState.ProtocolState == nil {
			withState.ProtocolState = protocolState
		}
		rendered := renderer.RenderFailure(view, &withState)
		if rendered.StatusCode > 0 {
			return rendered, true
		}
		// A renderer that produced no status is treated as having declined; the
		// generic response below is used rather than emitting a 0 status.
	}
	return genericResolutionFailure(failure.Kind, errorID), false
}

// genericResolutionFailure is the sterile response for a resolution failure: an
// HTTP status, a fixed reason phrase, and a correlation id that also appears in the
// warning log. It never names the resolver, the operation, or the underlying cause.
func genericResolutionFailure(kind resolver.FailureKind, errorID string) resolver.RenderedFailure {
	status := http.StatusInternalServerError
	message := "Internal Server Error"

	switch kind {
	case resolver.FailureParse, resolver.FailureInvalidRequest, resolver.FailureMultiOperation,
		resolver.FailureUndecodableBody:
		status, message = http.StatusBadRequest, "Bad Request"
	case resolver.FailureUnknownOperation:
		status, message = http.StatusNotFound, "Not Found"
	case resolver.FailurePayloadTooLarge:
		status, message = http.StatusRequestEntityTooLarge, "Payload Too Large"
	case resolver.FailureUnsupportedEncoding:
		// The client named a coding this gateway cannot decode; 415 says exactly that,
		// where a 400 or 500 would send them looking at their payload or at us.
		status, message = http.StatusUnsupportedMediaType, "Unsupported Media Type"
	}

	return resolver.RenderedFailure{
		StatusCode: status,
		Headers: map[string]string{
			"content-type": "application/json",
			"x-error-id":   errorID,
		},
		Body: []byte(fmt.Sprintf(`{"error":%q,"error_id":%q}`, message, errorID)),
	}
}

// effectiveResponseKind decides how this request's response is delivered.
//
// A resolver's answer wins over the route's, because a multiplexed route carries
// operations of both kinds and only the resolver knows which one this request is. The
// route's value is for the opposite case: an operation known at deploy time with no
// resolver to ask (an A2A HTTP+JSON operation route). An unrecognised value from a newer
// controller is treated as Auto rather than guessed at — Auto is the pre-existing
// behaviour, so an unknown kind degrades to "derive it as before" instead of forcing a
// mode that may be wrong.
func effectiveResponseKind(route *RouteConfig, res resolver.Resolution) resolver.ResponseKind {
	if res.ResponseKind.Valid() && res.ResponseKind != resolver.ResponseKindAuto {
		return res.ResponseKind
	}
	if route != nil && route.ResponseKind.Valid() {
		return route.ResponseKind
	}
	return resolver.ResponseKindAuto
}

// streamingConflictResponse replaces the upstream response when a streaming operation
// is bound to a chain that cannot stream. It is deliberately the same sterile shape as
// any other internal resolution failure: a 500, a fixed reason phrase, and a correlation
// id that also appears in the error log. The client is not told which policy is
// incompatible, or that the mismatch is about streaming at all — that is operator
// information, and naming it would describe the gateway's internal policy wiring to a
// caller (error-handling.md directive 1).
func (ec *PolicyExecutionContext) streamingConflictResponse() *extprocv3.ProcessingResponse {
	errorID := uuid.New().String()
	rendered := genericResolutionFailure(resolver.FailureStreamingUnsupported, errorID)

	slog.Error("[resolution] streaming operation cannot be served by its policy chain",
		"error_id", errorID,
		"route", ec.routeKey,
		"chain_key", ec.chainKey,
		"resolver", ec.resolverName)
	metrics.ResolutionFailuresTotal.WithLabelValues(
		ec.resolverName, string(resolver.FailureStreamingUnsupported)).Inc()

	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extprocv3.ImmediateResponse{
				Status:  &typev3.HttpStatus{Code: typev3.StatusCode(rendered.StatusCode)},
				Headers: buildHeaderValueOptions(rendered.Headers),
				Body:    rendered.Body,
			},
		},
	}
}

// ─── Pass-throughs for a request that never bound a policy chain ─────────────
//
// Reachable only if Envoy delivers a callback after an ImmediateResponse, or if a
// deferred route's request-body callback never arrives. Each returns the phase's
// empty response — the same shape the "no execution context" branches in
// handleProcessingPhase already return — so an unexpected callback ordering can
// never turn into a nil-chain dereference mid-stream.

func (ec *PolicyExecutionContext) noChainPassThroughRequestBody() *extprocv3.ProcessingResponse {
	slog.Warn("[resolution] request body callback with no bound policy chain",
		"route", ec.routeKey, "resolver", ec.resolverName, "denied", ec.resolutionDenied)
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestBody{RequestBody: &extprocv3.BodyResponse{}},
	}
}

func (ec *PolicyExecutionContext) noChainPassThroughResponseHeaders() *extprocv3.ProcessingResponse {
	slog.Warn("[resolution] response headers callback with no bound policy chain",
		"route", ec.routeKey, "resolver", ec.resolverName, "denied", ec.resolutionDenied)
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseHeaders{ResponseHeaders: &extprocv3.HeadersResponse{}},
	}
}

func (ec *PolicyExecutionContext) noChainPassThroughResponseBody() *extprocv3.ProcessingResponse {
	slog.Warn("[resolution] response body callback with no bound policy chain",
		"route", ec.routeKey, "resolver", ec.resolverName, "denied", ec.resolutionDenied)
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseBody{ResponseBody: &extprocv3.BodyResponse{}},
	}
}

// recordResolutionAttributes stamps the resolver name and the selected chain key on a
// span, skipping whichever is not known yet.
//
// It is called twice for a body-resolved route — once at the request-headers callback,
// where only the resolver is known, and again at the request-body callback once the
// chain has actually been selected. Skipping an empty chain key rather than recording
// one keeps the attribute meaningful: an operator filtering on it sees only spans where
// a chain was really chosen, instead of every deferred request carrying "".
func (ec *PolicyExecutionContext) recordResolutionAttributes(span trace.Span) {
	if span == nil || !span.IsRecording() || ec.resolverName == "" {
		// An identity route has no resolver, and its chain key equals the route name
		// that is already on the span.
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String(constants.AttrResolverName, ec.resolverName),
	}
	if ec.chainKey != "" {
		attrs = append(attrs, attribute.String(constants.AttrPolicyChainKey, ec.chainKey))
	}
	span.SetAttributes(attrs...)
}

// ─── Request-body decoding for a resolver ────────────────────────────────────

// resolverDecodableCodings is the set of content codings the policy engine can
// actually decode. It is deliberately an allowlist rather than a denylist of "bad"
// values: decompressBody returns an unrecognised coding's bytes unchanged and
// reports no error, which is the right lenient behaviour for a body handed to a
// policy but the wrong behaviour for one handed to a resolver.
var resolverDecodableCodings = map[string]bool{
	"gzip": true,
	"br":   true,
}

// decodeResolverRequestBody returns the bytes a resolver may read, or the failure to
// deny the request with.
//
// A resolver decides which policy chain runs, so it must never be shown bytes the
// engine did not actually decode: it would resolve to whatever the compressed frame
// happens to look like — most likely nothing, but possibly a different operation than
// the client sent, with a different chain. That is a policy-selection bug, not a
// parsing inconvenience, so an encoding the engine cannot decode is rejected here
// rather than passed through.
//
// This gate is specific to the deferred (body-resolved) path. Identity routes keep
// decompressBody's lenient behaviour — pass the raw bytes to policies and log — because
// there the chain is already selected and no security decision hangs on the body's
// interpretation.
func (ec *PolicyExecutionContext) decodeResolverRequestBody(
	wire []byte,
	headers map[string][]string,
) ([]byte, *resolver.ResolutionError) {
	coding, err := resolverContentCoding(headers)
	if err != nil {
		return nil, &resolver.ResolutionError{Kind: resolver.FailureUnsupportedEncoding, Cause: err}
	}

	if coding == "" {
		// No coding, or `identity`, which RFC 9110 defines as the absence of one.
		// The decoded ceiling still applies, so the resolver's input is bounded by the
		// same number whether or not the request was compressed.
		if int64(len(wire)) > ec.server.maxRequestDecompressedBytes {
			return nil, &resolver.ResolutionError{
				Kind:  resolver.FailurePayloadTooLarge,
				Cause: fmt.Errorf("request body %d bytes exceeds the decoded body limit", len(wire)),
			}
		}
		return wire, nil
	}

	decoded, err := decompressBody(wire, coding, ec.server.maxRequestDecompressedBytes)
	if err != nil {
		if errors.Is(err, ErrDecompressedTooLarge) {
			return nil, &resolver.ResolutionError{Kind: resolver.FailurePayloadTooLarge, Cause: err}
		}
		// A body that claims a coding the engine supports but does not decode under it
		// is a malformed request, not an engine fault.
		return nil, &resolver.ResolutionError{Kind: resolver.FailureUndecodableBody, Cause: err}
	}

	// Pin the canonical coding for the rest of the request. The header may have said
	// "GZIP" or " gzip"; the recompression path re-encodes a policy-modified body from
	// this field and only matches the canonical token, so normalising it here is what
	// keeps a mutated body from being forwarded as plaintext under a compressed label.
	ec.requestContentEncoding = coding
	return decoded, nil
}

// resolverContentCoding reduces a request's Content-Encoding headers to the single
// coding the body is actually under, or reports why it cannot.
//
// It reads every header line rather than the single value captured at context-build
// time, because a client may split a coding list across lines and the capture keeps
// only the last — which would hide a stacked encoding entirely. Tokens are
// case-folded, since HTTP content codings are case-insensitive and `GZIP` would
// otherwise fall through as "unrecognised" and be passed to the resolver raw.
func resolverContentCoding(headers map[string][]string) (string, error) {
	var tokens []string
	for name, values := range headers {
		if !strings.EqualFold(name, "content-encoding") {
			continue
		}
		for _, value := range values {
			for _, part := range strings.Split(value, ",") {
				token := strings.ToLower(strings.TrimSpace(part))
				if token == "" || token == "identity" {
					continue
				}
				tokens = append(tokens, token)
			}
		}
	}

	switch len(tokens) {
	case 0:
		return "", nil
	case 1:
		if !resolverDecodableCodings[tokens[0]] {
			return "", fmt.Errorf("content coding %q cannot be decoded for operation resolution", tokens[0])
		}
		return tokens[0], nil
	default:
		// Stacked codings would need to be unwrapped in order, innermost last. The
		// engine decodes exactly one layer, so accepting this would hand the resolver
		// a still-encoded body.
		sort.Strings(tokens)
		return "", fmt.Errorf("stacked content codings %v cannot be decoded for operation resolution", tokens)
	}
}

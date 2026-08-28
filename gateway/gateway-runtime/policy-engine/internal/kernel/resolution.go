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
	"maps"
	"net/http"
	"sort"
	"strings"

	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/structpb"

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

	// bindFailed means resolution ran and failed. The response is the sterile generic
	// one; the failure kind reaches the log, the metric and the span, never the client.
	bindFailed
)

// resolutionDenial carries everything the request-headers callback needs to render
// a resolution failure, captured at the point of failure so the caller never has to
// look the resolver up a second time (and cannot look up a different one).
type resolutionDenial struct {
	resolverName string
	failure      *resolver.ResolutionError
}

// newResolutionDenial records a failure against the route that produced it.
func newResolutionDenial(pr *resolver.PreparedRoute, failure *resolver.ResolutionError) *resolutionDenial {
	return &resolutionDenial{resolverName: pr.ResolverName, failure: failure}
}

// pendingResolution is the retained state of a request whose policy chain cannot be
// selected at the request-headers callback because the route's resolver must read
// the request body.
//
// Nothing is forwarded upstream while this is set, so ordering guarantees relative
// to the backend still hold: the resolved chain's request-header policies simply run
// one callback later than they would on an identity route.
type pendingResolution struct {
	route    *RouteConfig
	prepared *resolver.PreparedRoute

	// view is the header-phase request view, retained so the resolver observes the
	// headers, method and path as the client actually sent them rather than values
	// re-derived at the body callback.
	view resolver.RequestView
}

// buildRequestView snapshots what a resolver is allowed to see about a request.
//
// Only called for a route whose resolution is not already known: a statically-resolved
// route (every kind shipping today, via route-key) binds from the result captured at
// ingest and must not pay for this allocation.
//
// The route's partition — API ID, vhost, API context — is deliberately absent: the
// prepared resolver captured it at ingest, so a resolver cannot be handed a partition
// that differs from the one its keys are validated against.
func buildRequestView(routeKey string, headers *extprocv3.HttpHeaders) resolver.RequestView {
	view := resolver.RequestView{RouteKey: routeKey}

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

// pendingModeOverride is the ProcessingMode returned from the request-headers
// callback of a route whose chain is not selected yet.
//
// It is derived from the prepared resolver's Requirements, not from a policy chain —
// there is no chain yet. That is also why no controller-side ExtProcPerRoute processing
// mode override (or equivalent route flag) is used for this: it would duplicate the
// route's body requirement in a second place that can disagree with it.
//
// Response modes are left at NONE and revisited from the response-header callback,
// once the chain is known: Envoy applies a ModeOverride only on responses to header
// callbacks, and the response-header callback still precedes its decision about how
// to deliver the upstream response body.
func pendingModeOverride(reqs resolver.RequestRequirements) *extprocconfigv3.ProcessingMode {
	mode := &extprocconfigv3.ProcessingMode{
		ResponseHeaderMode:  extprocconfigv3.ProcessingMode_SEND,
		RequestTrailerMode:  extprocconfigv3.ProcessingMode_SKIP,
		ResponseTrailerMode: extprocconfigv3.ProcessingMode_SKIP,
		ResponseBodyMode:    extprocconfigv3.ProcessingMode_NONE,
	}
	if reqs.BuffersBody() {
		mode.RequestBodyMode = extprocconfigv3.ProcessingMode_BUFFERED
	} else {
		mode.RequestBodyMode = extprocconfigv3.ProcessingMode_NONE
	}
	return mode
}

// pendingResolutionResponse is the request-headers response for a deferred route:
// no mutation, no policy result, no analytics — only the instruction to buffer the
// body and call back.
func pendingResolutionResponse(reqs resolver.RequestRequirements) *extprocv3.ProcessingResponse {
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
	// which is per-route. See RouteConfig.MaxRequestBodyBytes.
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

	resolution, err := pending.prepared.Resolver.Resolve(ctx, view)
	if err != nil {
		return ec.denyResolution(ctx, resolver.NormalizeResolutionError(err)), nil
	}

	bound, chain, err := resolver.Bind(pending.prepared, resolution, ec.server.kernel.GetPolicyChain)
	if err != nil {
		if errors.Is(err, resolver.ErrDirectRouteChainMissing) {
			// A route whose resolution is direct has no chain of its own. Not reachable
			// from a deferred route today — the only direct resolver is static and never
			// defers — but it is classified as the same deployment fault a direct route
			// gets elsewhere rather than as something the caller did.
			return ec.denyResolution(ctx, &resolver.ResolutionError{
				Kind:  resolver.FailureChainMissing,
				Cause: err,
			}), nil
		}
		return ec.denyResolution(ctx, resolver.NormalizeResolutionError(err)), nil
	}

	// Bind. The resolution is applied to the shared context before the chain's own
	// policies run below, so a policy on a body-resolved route sees the operation it
	// is running for rather than the empty value the header callback left behind.
	ec.policyChain = chain
	ec.chainKey = bound.ChainKey
	ec.applyBoundResolution(bound)
	ec.pending = nil
	ec.boundAtBodyPhase = true

	slog.DebugContext(ctx, "[resolution] chain bound at request-body phase",
		"route", ec.routeKey, "resolver", ec.resolverName, "chain_key", bound.ChainKey,
		"operation", bound.Operation, "decoded_bytes", len(decoded))

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
	resp, outcome := renderResolutionFailure(ctx, ec.resolverName, ec.routeKey, ec.requestID, failure,
		resolutionFailureAnalytics(nil, ec))

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
// Every failure renders as the sterile generic response: an HTTP status derived from the
// FailureKind, a fixed reason phrase and a correlation id that also appears in the
// warning log. The kind survives in the log and the metric, never in the body, and the
// failure's Cause is logged and never returned (error-handling.md directive 1).
func renderResolutionFailure(
	ctx context.Context,
	resolverName string,
	routeKey string,
	requestID string,
	failure *resolver.ResolutionError,
	analytics *structpb.Struct,
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

	rendered := genericResolutionFailure(failure.Kind, errorID)

	imm := &extprocv3.ImmediateResponse{
		Status:  &typev3.HttpStatus{Code: typev3.StatusCode(rendered.StatusCode)},
		Headers: buildHeaderValueOptions(rendered.Headers),
		Body:    rendered.Body,
	}
	resp := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{ImmediateResponse: imm},
	}
	if analytics != nil {
		resp.DynamicMetadata = buildDynamicMetadata(analytics, nil, nil)
	}
	return resp, tracing.HTTPOutcome{
		StatusCode: rendered.StatusCode,
		Reason:     constants.TerminalReasonResolutionFailed,
		ErrorID:    errorID,
	}
}

// resolutionFailureAnalytics builds the analytics payload for a request rejected before
// any policy chain was bound.
//
// Without it such a request produces no analytics event at all: the rejection is the
// only ext_proc response it ever generates, and it used to carry no dynamic metadata,
// so the access-log entry had nothing identifying the API it was aimed at. The failure
// was visible only in resolution_failures_total. For a protocol whose operation lives
// in the request payload that is the one failure mode most likely to be the caller's
// own — a malformed envelope, an operation the protocol version does not define — and
// it is exactly what a success-rate breakdown needs in order to attribute a failure to
// the client rather than to the agent.
//
// Exactly one of routeFields and execCtx carries the API identity, depending on which
// phase rejected: the request-headers callback has route metadata and no execution
// context, and the request-body callback has the context (whose shared context
// buildAnalyticsStruct reads) and does not re-derive the metadata.
//
// The failure *kind* is deliberately not included. It reaches the log, the metric and
// the span, and putting it on an event that leaves the process would publish which
// specific malformation a caller achieved.
func resolutionFailureAnalytics(routeFields map[string]any, execCtx *PolicyExecutionContext) *structpb.Struct {
	fields := make(map[string]any, len(routeFields)+1)
	maps.Copy(fields, routeFields)
	fields[TerminalReasonKey] = constants.TerminalReasonResolutionFailed

	built, err := buildAnalyticsStruct(fields, execCtx)
	if err != nil {
		// Never fail the rejection over its own telemetry: the sterile response still
		// has to go out.
		slog.Warn("Failed to build analytics metadata for a resolution failure", "error", err)
		return nil
	}
	return built
}

// sterileFailure is the kernel's own generic error response. It is deliberately not part
// of the resolver contract: a resolver classifies a failure and never shapes one.
type sterileFailure struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// genericResolutionFailure is the sterile response for a resolution failure: an
// HTTP status, a fixed reason phrase, and a correlation id that also appears in the
// warning log. It never names the resolver, the operation, or the underlying cause.
func genericResolutionFailure(kind resolver.FailureKind, errorID string) sterileFailure {
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

	return sterileFailure{
		StatusCode: status,
		Headers: map[string]string{
			"content-type": "application/json",
			"x-error-id":   errorID,
		},
		Body: []byte(fmt.Sprintf(`{"error":%q,"error_id":%q}`, message, errorID)),
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

// recordResolutionAttributes stamps the resolver name, the selected chain key, the
// resolved operation and the resolver's protocol-derived request facts on a span,
// skipping whichever is not known yet.
//
// It is called twice for a body-resolved route — once at the request-headers callback,
// where only the resolver is known, and again at the request-body callback once the
// chain has actually been selected. Skipping an empty attribute rather than recording
// one keeps them meaningful: an operator filtering on the chain key sees only spans
// where a chain was really chosen, instead of every deferred request carrying "".
//
// The request facts are recorded under the names their producing protocol gave them
// (already namespaced — "a2a.context.id"), so a trace can be searched by the
// conversation or task a request belonged to. A span is the right home for a
// caller-supplied identifier: it is per-request storage, unlike a metric label, whose
// cardinality it would multiply. Count and length are already bounded by
// ValidateResolution.
func (ec *PolicyExecutionContext) recordResolutionAttributes(span trace.Span) {
	if span == nil || !span.IsRecording() || ec.resolverName == "" {
		// An identity route has no resolver, and its chain key equals the route name
		// that is already on the span.
		return
	}
	attrs := make([]attribute.KeyValue, 0, 3+len(ec.resolutionAttributes))
	attrs = append(attrs, attribute.String(constants.AttrResolverName, ec.resolverName))
	if ec.chainKey != "" {
		attrs = append(attrs, attribute.String(constants.AttrPolicyChainKey, ec.chainKey))
	}
	if ec.operation != "" {
		attrs = append(attrs, attribute.String(constants.AttrResolvedOperation, ec.operation))
	}
	for name, value := range ec.resolutionAttributes {
		attrs = append(attrs, attribute.String(name, value))
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

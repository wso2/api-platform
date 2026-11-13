/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package kernel

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// pseudoHeaders Envoy delivers alongside the real ones. Read for context, and :path is the one
// a per-attempt policy most often rewrites — re-pointing an attempt at another provider is a
// path change.
// requestIDHeader ties one request's attempts together. Envoy stamps it because the HCM is
// generated with generate_request_id.
const requestIDHeader = "x-request-id"

const (
	pseudoPath      = ":path"
	pseudoMethod    = ":method"
	pseudoAuthority = ":authority"
	pseudoScheme    = ":scheme"
)

// handleUpstreamRequestHeaders runs the per-attempt chain for the cluster this attempt is
// routed to.
//
// Called once per upstream attempt, including every failover retry — unlike the downstream
// chain, which Envoy runs once per client request and never re-enters. That is the whole reason
// this path exists: work that must differ between attempts (the provider's context path, its
// transformation, its credential) can only happen here.
//
// An attempt whose cluster has no chain proceeds unmodified. That is the ordinary case, not an
// error: an upstream with no per-attempt work generates no chain at all.
//
// REQUEST ONLY, deliberately (FR-037a). There is no response half here: a response at this
// position carries no cluster attribute, so resolving its chain would mean carrying per-stream
// state from the request phase, and that state is exactly the retry/upstream-filter interaction
// Agent Router avoids by handling responses downstream. The downstream response phase already
// knows which target answered, from the attempt count (FR-030f-FR-030j).
func (s *ExternalProcessorServer) handleUpstreamRequestHeaders(
	ctx context.Context,
	req *extprocv3.ProcessingRequest,
	state *upstreamAttemptState,
) (*extprocv3.ProcessingResponse, error) {

	cluster := clusterFromRequest(req)
	chain := s.kernel.UpstreamChainForCluster(cluster)
	attempt := attemptNumberFromRequest(req)

	// Hand this attempt's identity to its own body phase, which Envoy sends without attributes.
	state.cluster = cluster

	headers := req.GetRequestHeaders().GetHeaders()
	incoming := upstreamRequestContext(req, headers)

	// Reset to the post-downstream baseline before running anything (FR-038, FR-038a).
	//
	// Envoy hands each attempt the request as the PREVIOUS attempt left it — both probes
	// measured this, a naive prefix rewrite producing "/ctx-2/ctx-1/chat/completions". So a
	// policy that derives a value from what it sees would compound it on every retry.
	//
	// The baseline cannot be published by the downstream pass, because Envoy applies the
	// route's own rewrite AFTER ext_proc: the engine never sees the exact bytes attempt 1
	// sends. It is therefore captured ON attempt 1 and held SERVER-SIDE, in a store keyed by
	// request id (attempt_baseline_store.go). Not dynamic metadata: that route would rest on an
	// assumption never measured — that an UPSTREAM ext_proc's metadata write survives into the
	// next attempt — where process memory simply does.
	requestID := requestIDFrom(headers)
	baseline, known := s.baselines.Lookup(requestID)
	if !known {
		// First attempt: it IS the baseline, and every later attempt resets to it.
		baseline = requestBaseline{Path: incoming.Path, Method: incoming.Method, Authority: incoming.Authority}
		s.baselines.Remember(requestID, baseline)

		// A retry with no baseline is a silent correctness failure, not a normal path: this
		// attempt becomes its own baseline, so the previous attempt's rewrites compound and
		// its headers are never undone -- while the client still receives a 200. The entry
		// expired mid-request, or the request carries no id to key it on. Logged rather than
		// failed because failing closed here would turn a degraded retry into a lost request
		// (FR-038a's TTL is the safety net, this is how anyone finds out it was needed).
		if attempt > 1 {
			slog.WarnContext(ctx, "per-attempt baseline missing on a retry; "+
				"this attempt cannot be reset to the post-downstream request",
				"attempt", attempt, "cluster", cluster, "has_request_id", requestID != "")
		}
	}
	// This attempt is, for now, the one that will serve. Recorded on every attempt so that when
	// the retries stop the stored value is whichever attempt answered — the downstream response
	// phase unwinds that attempt's chain (FR-037h). Written after the baseline exists, since the
	// store ignores a serving-cluster write for a request it has no entry for.
	s.baselines.RememberServingCluster(requestID, cluster)

	reqCtx := incoming
	reqCtx.Path = baseline.Path
	reqCtx.Method = baseline.Method
	reqCtx.Authority = baseline.Authority

	// Undo what EARLIER attempts' chains did to the headers, before this attempt's chain runs
	// and before a policy can read a leftover (FR-038, FR-038b). Envoy hands each retry the
	// request as the previous attempt left it, so without this the prior provider's credential
	// travels to the next provider -- a key handed to a third party, invisible to the client
	// because the request still succeeds (FR-042).
	//
	// Applied to the in-memory map AND emitted to Envoy: the first is what the chain reads,
	// the second is what the provider receives.
	headerValues := reqCtx.Headers.UnsafeInternalValues()
	restores := headerRestoresFor(baseline.Headers, headerValues)
	applyHeaderRestores(headerValues, restores)

	// Re-point the attempt at its provider's own context BEFORE the chain runs, so a policy
	// sees the path the provider will actually receive.
	//
	// Done here rather than by a policy because it must happen for EVERY attempt, including
	// ones whose provider has no authored policies at all — the route's compiled-in rewrite
	// sends all of them to the primary's context otherwise.
	var repointed *string
	if providerPath, routePath, ok := s.providerContextFor(req); ok {
		state.providerBasePath, state.routeBasePath = providerPath, routePath
		newPath := repointToProvider(baseline.Path, routePath, providerPath)
		if newPath != incoming.Path {
			repointed = &newPath
			reqCtx.Path = newPath
		}
	}

	// Tell the chain which attempt it is and what it is aimed at, BEFORE it runs. A policy
	// whose decision varies per attempt cannot derive either: the downstream chain ran once,
	// before any target existed, and Envoy reports the selection only as an attribute on this
	// message. Carried in metadata so the body phase of the same attempt sees it too.
	withAttemptContext(reqCtx.SharedContext, attempt, cluster)
	state.metadata = reqCtx.SharedContext.Metadata

	if chain == nil || len(chain.Policies) == 0 {
		// No per-attempt policies, but the re-point and the undo still have to be applied: a
		// provider with no chain of its own must not inherit the previous provider's headers.
		return upstreamPathOnlyResponse(repointed, restores), nil
	}

	// Ask for the body only if something in this chain rewrites it.
	modeOverride := upstreamModeOverride(chain)

	slog.DebugContext(ctx, "running per-attempt policy chain",
		"cluster", cluster, "attempt", attempt, "policies", len(chain.Policies))

	// What the headers held before this chain ran. Captured here because the executor writes a
	// policy's HeadersToSet straight into the map, so after execution the prior value is gone --
	// and the prior value is exactly what a later attempt has to be restored to.
	preChain := cloneHeaderValues(headerValues)

	result, err := s.executor.ExecuteRequestHeaderPolicies(
		ctx, chain.Policies, reqCtx, chain.PolicySpecs,
		"", UpstreamChainKey(cluster), false)
	if err != nil {
		// Fail closed. The router is configured with failure_mode_allow:false, so returning
		// an error stops the attempt rather than letting it reach the provider
		// untransformed or unauthenticated (FR-042).
		return nil, err
	}

	// Record what THIS chain touched so the next attempt can undo it (FR-038b). First-touch
	// wins inside the store, so an attempt re-setting a header another attempt already recorded
	// does not overwrite the post-downstream value with a leftover.
	s.baselines.RememberTouchedHeaders(requestID, captureHeaderBaselines(preChain, touchedHeaderNames(result)))

	// Post-chain, so the body phase starts from what this attempt will actually send.
	state.headers = cloneHeaderValues(reqCtx.Headers.UnsafeInternalValues())

	resp := upstreamHeadersResponse(result, repointed, restores)
	resp.ModeOverride = modeOverride
	return resp, nil
}

// requestIDFrom reads the id Envoy stamps on every request (the HCM is generated with
// generate_request_id), which is what ties one request's attempts together.
//
// Without it the attempts cannot be correlated, so each becomes its own baseline — the same
// behaviour as before this store existed, and no worse than compounding would have been.
func requestIDFrom(headers *corev3.HeaderMap) string {
	for _, h := range headers.GetHeaders() {
		if strings.ToLower(h.GetKey()) != requestIDHeader {
			continue
		}
		if v := h.GetValue(); v != "" {
			return v
		}
		return string(h.GetRawValue())
	}
	return ""
}

// requestBaseline is the request as it entered the upstream chain on the FIRST attempt — after
// the downstream chain ran and after Envoy applied the route's own rewrite, but before any
// per-attempt policy touched it.
type requestBaseline struct {
	Path      string
	Method    string
	Authority string

	// Headers a per-attempt chain changed, keyed by lower-case name, each holding the value
	// the header had BEFORE the first chain to touch it ran (FR-038b).
	//
	// Only touched headers are recorded, never the whole map. That keeps the client's headers,
	// the downstream chain's headers and Envoy's own (x-request-id, x-envoy-*, tracing) outside
	// this mechanism entirely, and avoids holding every request's credentials in process memory
	// for data that would almost never be restored.
	Headers map[string]headerBaseline
}

// upstreamRequestContext builds the context a per-attempt policy sees.
//
// Downstream is deliberately nil. It carries the snapshot of the CLIENT request, which belongs
// to the once-per-request position; a policy running here is serving one attempt, and reading
// client-request state would invite it to make decisions that cannot legitimately vary per
// attempt (FR-037).
func upstreamRequestContext(req *extprocv3.ProcessingRequest, headers *corev3.HeaderMap) *policy.RequestHeaderContext {
	values := map[string][]string{}
	var path, method, authority, scheme string

	for _, h := range headers.GetHeaders() {
		value := h.GetValue()
		if value == "" && len(h.GetRawValue()) > 0 {
			value = string(h.GetRawValue())
		}
		switch h.GetKey() {
		case pseudoPath:
			path = value
		case pseudoMethod:
			method = value
		case pseudoAuthority:
			authority = value
		case pseudoScheme:
			scheme = value
		}
		key := strings.ToLower(h.GetKey())
		values[key] = append(values[key], value)
	}

	return &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{
			// The chain the downstream pass published travels here in dynamic metadata, so a
			// per-attempt policy can see what the request was supposed to do.
			Metadata: upstreamMetadata(req),
		},
		Headers:   policy.NewHeaders(values),
		Path:      path,
		Method:    method,
		Authority: authority,
		Scheme:    scheme,
		// Downstream stays nil: see the doc comment above.
	}
}

// withAttemptContext writes the reserved wso2.attempt namespace into a per-attempt chain's shared
// context.
//
// This is the platform half of per-attempt policy support: it is what lets a policy act
// differently on attempt 2 than on attempt 1 without the engine knowing why. Both values are
// published because they answer different questions -- the number says "how many tries in", the
// cluster says "which target". A policy identifying its target should prefer the cluster: an index
// into a published ranking goes wrong as soon as a member is suspended or skipped.
func withAttemptContext(shared *policy.SharedContext, attempt int, cluster string) {
	if shared == nil {
		return
	}
	if shared.Metadata == nil {
		shared.Metadata = map[string]interface{}{}
	}
	shared.Metadata[constants.AttemptMetadataNamespace] = map[string]interface{}{
		constants.AttemptMetadataNumber:  attempt,
		constants.AttemptMetadataCluster: cluster,
	}
}

// upstreamMetadata lifts Envoy's forwarded dynamic metadata into the shared map a policy reads.
func upstreamMetadata(req *extprocv3.ProcessingRequest) map[string]interface{} {
	metadata := map[string]interface{}{}
	fields := req.GetMetadataContext().GetFilterMetadata()
	for namespace, value := range fields {
		metadata[namespace] = value.AsMap()
	}
	return metadata
}

// upstreamHeadersResponse turns the chain's modifications into the ext_proc response.
//
// Deliberately narrower than the downstream translator: there is no short-circuit handling, no
// analytics emission and no body mutation. A per-attempt policy re-points and re-signs a request
// — it does not decide whether the request happens, which was settled once, downstream.
func upstreamHeadersResponse(result *executor.RequestHeaderExecutionResult, repointed *string,
	restores []headerRestore) *extprocv3.ProcessingResponse {

	mutation := &extprocv3.HeaderMutation{}

	// Undo earlier attempts FIRST, so this chain's own mutations overwrite the restored values
	// rather than being overwritten by them.
	applyRestoresToMutation(mutation, restores)

	// The provider re-point is applied first so a policy returning its own Path still wins.
	if repointed != nil {
		mutation.SetHeaders = append(mutation.SetHeaders, setHeader(pseudoPath, *repointed))
	}

	for _, pr := range result.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		mods, ok := pr.Action.(policy.UpstreamRequestHeaderModifications)
		if !ok {
			continue
		}
		for k, v := range mods.HeadersToSet {
			mutation.SetHeaders = append(mutation.SetHeaders, setHeader(k, v))
		}
		for k, values := range mods.HeadersToAppend {
			for _, v := range values {
				mutation.SetHeaders = append(mutation.SetHeaders, appendHeader(k, v))
			}
		}
		mutation.RemoveHeaders = append(mutation.RemoveHeaders, mods.HeadersToRemove...)

		// Re-pointing the attempt. Allowed only because the router sets
		// mutation_rules.allow_all_routing on this filter — without it ext_proc drops a
		// :path change silently, with no error and no log.
		if mods.Path != nil {
			mutation.SetHeaders = append(mutation.SetHeaders, setHeader(pseudoPath, *mods.Path))
		}
		if mods.Host != nil {
			mutation.SetHeaders = append(mutation.SetHeaders, setHeader(pseudoAuthority, *mods.Host))
		}
		if mods.Method != nil {
			mutation.SetHeaders = append(mutation.SetHeaders, setHeader(pseudoMethod, *mods.Method))
		}
	}

	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{HeaderMutation: mutation},
			},
		},
	}
}

func setHeader(key, value string) *corev3.HeaderValueOption {
	return &corev3.HeaderValueOption{
		Header:       &corev3.HeaderValue{Key: key, RawValue: []byte(value)},
		AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
	}
}

func appendHeader(key, value string) *corev3.HeaderValueOption {
	return &corev3.HeaderValueOption{
		Header:       &corev3.HeaderValue{Key: key, RawValue: []byte(value)},
		AppendAction: corev3.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD,
	}
}

// applyRestoresToMutation emits the undo operations Envoy must apply to this attempt.
//
// A removal and a set are both expressed here rather than only in the in-memory map, because the
// map is what the policies read while this is what the provider actually receives.
func applyRestoresToMutation(mutation *extprocv3.HeaderMutation, restores []headerRestore) {
	for _, r := range restores {
		if r.Remove {
			mutation.RemoveHeaders = append(mutation.RemoveHeaders, r.Name)
			continue
		}
		for i, v := range r.Values {
			// The first write overwrites whatever the previous attempt left; the rest append,
			// so a multi-valued header is restored with all of its values rather than the last.
			if i == 0 {
				mutation.SetHeaders = append(mutation.SetHeaders, setHeader(r.Name, v))
				continue
			}
			mutation.SetHeaders = append(mutation.SetHeaders, appendHeader(r.Name, v))
		}
	}
}

// upstreamPathOnlyResponse carries the provider re-point and any undo of earlier attempts, for an
// attempt whose cluster has no per-attempt policies of its own.
func upstreamPathOnlyResponse(repointed *string, restores []headerRestore) *extprocv3.ProcessingResponse {
	if repointed == nil && len(restores) == 0 {
		return passthroughRequestHeaders()
	}
	mutation := &extprocv3.HeaderMutation{}
	applyRestoresToMutation(mutation, restores)
	// Guarded: an attempt can need the undo without needing a re-point, when its provider is
	// the one the route already points at.
	if repointed != nil {
		mutation.SetHeaders = append(mutation.SetHeaders, setHeader(pseudoPath, *repointed))
	}
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{HeaderMutation: mutation},
			},
		},
	}
}

// passthroughRequestHeaders leaves the attempt exactly as Envoy built it.
func passthroughRequestHeaders() *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{Response: &extprocv3.CommonResponse{}},
		},
	}
}

// providerContextFor returns the base path of the provider this attempt is routed to, and the
// base path the route itself compiled in.
//
// No new transport: a route config already carries `upstream_definition_paths` (definition name
// to base path) and `upstream_base_path` (the route's own). The cluster name encodes the
// definition it was built from, so matching one against the other recovers the provider's
// context without the control plane publishing anything extra.
//
// ok=false means the provider could not be resolved — the attempt is then left alone rather
// than re-pointed at a guess.
func (s *ExternalProcessorServer) providerContextFor(req *extprocv3.ProcessingRequest) (providerPath, routePath string, ok bool) {
	cluster := clusterFromRequest(req)
	if cluster == "" {
		return "", "", false
	}
	routeConfig := s.kernel.GetRouteConfig(routeKeyFromRequest(req))
	if routeConfig == nil {
		return "", "", false
	}

	routePath = routeConfig.Metadata.UpstreamBasePath
	for name, basePath := range routeConfig.Metadata.UpstreamDefinitionPaths {
		// The controller builds a definition's cluster as
		// upstream_<Kind>_<uuid>_<sanitized name>, so the suffix identifies it. Matching on
		// the suffix avoids having to reverse the sanitization.
		if strings.HasSuffix(cluster, "_"+sanitizeUpstreamDefinitionName(name)) {
			return basePath, routePath, true
		}
	}
	return "", "", false
}

// routeKeyFromRequest reads the route this attempt belongs to.
func routeKeyFromRequest(req *extprocv3.ProcessingRequest) string {
	attrs, ok := req.GetAttributes()[constants.ExtProcFilter]
	if !ok || attrs.GetFields() == nil {
		return ""
	}
	return attrs.GetFields()[constants.ExtProcAttributeRouteName].GetStringValue()
}

// repointToProvider rewrites a path from the route's compiled-in context to this attempt's
// provider context.
//
// The route's regex_rewrite points EVERY attempt at the primary provider's context — it is
// compiled once and cannot vary per attempt — so without this an attempt routed to a fallback
// cluster still arrives on the primary's context and reaches the wrong provider. That is the
// gap that made cross-provider failover return 503 while every other part of the mechanism
// worked.
//
// Built from the BASELINE path, never the incoming one: the incoming path already carries the
// previous attempt's rewrite, so deriving from it would compound (FR-038).
func repointToProvider(baselinePath, routeBasePath, providerBasePath string) string {
	remainder := baselinePath
	if routeBasePath != "" && routeBasePath != "/" {
		remainder = strings.TrimPrefix(baselinePath, routeBasePath)
	}
	if remainder == "" {
		remainder = "/"
	}
	if !strings.HasPrefix(remainder, "/") {
		remainder = "/" + remainder
	}
	if providerBasePath == "" || providerBasePath == "/" {
		return remainder
	}
	return strings.TrimRight(providerBasePath, "/") + remainder
}

// handleUpstreamRequestBody runs the per-attempt chain's body phase.
//
// Needed because a provider's transformer rewrites the PAYLOAD — an OpenAI-shaped request into
// Mistral's or Anthropic's shape — and it implements that as OnRequestBody. Without this phase a
// transformer in a per-attempt chain would simply never be called, and the fallback provider
// would receive a request in the primary's format.
//
// The body is requested per attempt (via the mode override returned from the headers phase)
// rather than held in memory between attempts. That re-sends it on each retry, which is the
// price of letting the EXISTING transformer policies work unchanged: they take the body through
// the standard policy interface, not from engine-internal state.
func (s *ExternalProcessorServer) handleUpstreamRequestBody(
	ctx context.Context,
	req *extprocv3.ProcessingRequest,
	state *upstreamAttemptState,
) (*extprocv3.ProcessingResponse, error) {

	// Resolved by this attempt's headers phase: the body message carries neither the cluster
	// attribute nor the metadata context (upstreamAttemptState).
	cluster := state.cluster
	chain := s.kernel.UpstreamChainForCluster(cluster)
	if chain == nil || !chain.RequiresRequestBody {
		return passthroughRequestBody(nil, nil, nil), nil
	}

	body := req.GetRequestBody().GetBody()
	reqCtx := upstreamBodyContext(req.GetRequestBody(), state)

	namespaces := make([]string, 0, len(reqCtx.Metadata))
	for ns := range reqCtx.Metadata {
		namespaces = append(namespaces, ns)
	}
	slog.DebugContext(ctx, "running per-attempt body chain",
		"cluster", cluster, "bytes", len(body), "metadata_namespaces", namespaces)

	result, err := s.executor.ExecuteRequestPolicies(
		ctx, chain.Policies, reqCtx, chain.PolicySpecs,
		"", UpstreamChainKey(cluster), false)
	if err != nil {
		return nil, err
	}

	var mutated []byte
	setHeaders := map[string]string{}
	var removeHeaders []string
	for _, pr := range result.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		mods, ok := pr.Action.(policy.UpstreamRequestModifications)
		if !ok {
			continue
		}
		if mods.Body != nil {
			mutated = mods.Body
		}
		// A transformer rewrites the vendor path alongside the payload (Anthropic's
		// /v1/messages). Re-point it the same way the headers phase re-pointed the request,
		// so it lands inside this provider's own context rather than at the gateway root.
		if mods.Path != nil {
			repointed := repointToProvider(*mods.Path, state.routeBasePath, state.providerBasePath)
			setHeaders[":path"] = repointed
		}
		// A body rewrite usually comes with header changes -- a corrected content-length, a
		// vendor API version. Dropping them would send the provider a payload its headers
		// contradict.
		for key, value := range mods.HeadersToSet {
			setHeaders[key] = value
		}
		removeHeaders = append(removeHeaders, mods.HeadersToRemove...)
	}
	return passthroughRequestBody(mutated, setHeaders, removeHeaders), nil
}

// upstreamBodyContext builds the context a per-attempt chain's body phase runs against.
//
// Present MUST be true. Every body-rewriting policy in this repo opens with a variant of
// "if Body == nil || !Body.Present || len(Content) == 0 { nothing to do }", so a body handed over
// with Present left at its zero value is silently ignored: the chain runs, the policy returns no
// modification, and the fallback provider is sent the primary's payload format with no error
// anywhere. BUFFERED mode (upstreamModeOverride) delivers the whole body in one message, so
// "present and complete" is simply the truth here.
// Headers must be non-nil for the same reason: the executor writes a policy's HeadersToSet
// directly into the map behind them, which panics on a nil one.
func upstreamBodyContext(httpBody *extprocv3.HttpBody, state *upstreamAttemptState) *policy.RequestContext {
	return &policy.RequestContext{
		SharedContext: &policy.SharedContext{Metadata: state.metadata},
		Headers:       policy.NewHeaders(cloneHeaderValues(state.headers)),
		Body: &policy.Body{
			Content:     httpBody.GetBody(),
			Present:     true,
			EndOfStream: httpBody.GetEndOfStream(),
		},
	}
}

// contentLengthHeader is corrected whenever a per-attempt chain rewrites the body.
const contentLengthHeader = "content-length"

// passthroughRequestBody forwards the body, replacing it only when a policy rewrote it.
func passthroughRequestBody(mutated []byte, setHeaders map[string]string, removeHeaders []string) *extprocv3.ProcessingResponse {
	common := &extprocv3.CommonResponse{}
	if mutated != nil {
		common.BodyMutation = &extprocv3.BodyMutation{
			Mutation: &extprocv3.BodyMutation_Body{Body: mutated},
		}
		// A rewritten body whose content-length still describes the old one is rejected
		// outright by Envoy -- "mismatch_between_content_length_and_the_length_of_the_mutated
		// _body", a 500 to the client. The policy that rewrote the body has no reason to know
		// its own output length matters here, so the dispatcher owns this.
		//
		// A policy that set content-length itself still wins: it was explicit about it.
		if setHeaders == nil {
			setHeaders = map[string]string{}
		}
		if _, explicit := setHeaders[contentLengthHeader]; !explicit {
			setHeaders[contentLengthHeader] = strconv.Itoa(len(mutated))
		}
	}
	if len(setHeaders) > 0 || len(removeHeaders) > 0 {
		mutation := &extprocv3.HeaderMutation{RemoveHeaders: removeHeaders}
		for key, value := range setHeaders {
			mutation.SetHeaders = append(mutation.SetHeaders, &corev3.HeaderValueOption{
				Header: &corev3.HeaderValue{Key: key, RawValue: []byte(value)},
			})
		}
		common.HeaderMutation = mutation
	}
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestBody{
			RequestBody: &extprocv3.BodyResponse{Response: common},
		},
	}
}

// upstreamModeOverride asks Envoy for the request body when, and only when, this attempt's chain
// needs one.
//
// The filter is generated with RequestBodyMode: NONE and AllowModeOverride: true, so an attempt
// whose chain is header-only pays no buffering cost at all — the body is requested per attempt,
// by the chains that actually rewrite it.
func upstreamModeOverride(chain *registry.PolicyChain) *extprocconfigv3.ProcessingMode {
	if chain == nil || !chain.RequiresRequestBody {
		return nil
	}
	return &extprocconfigv3.ProcessingMode{
		RequestHeaderMode: extprocconfigv3.ProcessingMode_SEND,
		RequestBodyMode:   extprocconfigv3.ProcessingMode_BUFFERED,
	}
}

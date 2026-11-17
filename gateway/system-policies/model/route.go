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

package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// routeConfig is one Model chain's route block, validated at chain build.
type routeConfig struct {
	modelHandle string
	modelName   string

	// scopeTemplate and scopeProvider are the endpoint's inbound scope the response
	// is switched back to.
	scopeTemplate string
	scopeProvider string

	// location is where the request carries the model, and format the endpoint's
	// inbound format every refusal is written in.
	location modelLocation
	format   string

	// targets keeps the Model's declared order: targets[0] is the default.
	targets []routeTarget
}

// modelLocation is where an endpoint's requests carry the model: the endpoint's
// inbound template's requestModel, as the dispatch resolver reads it.
type modelLocation struct {
	kind    string
	member  string         // payload: the top-level member
	pattern *regexp.Regexp // pathParam: one capture group, the model segment
	name    string         // header (lower-cased) or query parameter
}

// Location kinds, the four a provider template's requestModel may name.
const (
	locationPayload    = "payload"
	locationPathParam  = "pathParam"
	locationHeader     = "header"
	locationQueryParam = "queryParam"
)

var topLevelMemberPattern = regexp.MustCompile(`^\$\.([A-Za-z_][A-Za-z0-9_\-]*)$`)

// parseModelLocation reads the route block's modelLocation. Absent is the OpenAI
// body member `$.model`, which is what a controller that predates the field
// compiles. Only what the dispatch resolver can read is accepted: a nested body
// path, a pattern without exactly one capture group, are refused.
func parseModelLocation(raw any, present bool) (modelLocation, error) {
	if !present {
		return modelLocation{kind: locationPayload, member: "model"}, nil
	}
	block, ok := raw.(map[string]any)
	if !ok {
		return modelLocation{}, fmt.Errorf("%s must be an object", ParamModelLocation)
	}
	kind, _ := block["location"].(string)
	id, _ := block["identifier"].(string)
	id = strings.TrimSpace(id)
	switch kind {
	case locationPayload:
		m := topLevelMemberPattern.FindStringSubmatch(id)
		if m == nil {
			return modelLocation{}, fmt.Errorf("%s: payload identifier must name a top-level member", ParamModelLocation)
		}
		return modelLocation{kind: kind, member: m[1]}, nil
	case locationPathParam:
		re, err := regexp.Compile(id)
		if err != nil || id == "" || re.NumSubexp() != 1 {
			return modelLocation{}, fmt.Errorf("%s: pathParam identifier must be a pattern with one capture group", ParamModelLocation)
		}
		return modelLocation{kind: kind, pattern: re}, nil
	case locationHeader, locationQueryParam:
		if id == "" {
			return modelLocation{}, fmt.Errorf("%s: identifier must not be empty", ParamModelLocation)
		}
		if kind == locationHeader {
			id = strings.ToLower(id)
		}
		return modelLocation{kind: kind, name: id}, nil
	default:
		return modelLocation{}, fmt.Errorf("%s: location must be payload, pathParam, header or queryParam", ParamModelLocation)
	}
}

// routeTarget is one of the Model's targets.
type routeTarget struct {
	name          string
	provider      string
	upstreamModel string
	template      string
	displayName   string
	routed        bool
}

// parseRouteConfig reads and validates a route block.
//
// Duplicate target names and duplicate (provider, upstream model) pairs are
// refused. The controller enforces both when a Model is saved, so either here is a
// controller defect; and the pair is the identity target resolution matches on, so
// two targets sharing one would make the exact-pair step ambiguous.
func parseRouteConfig(raw any) (routeConfig, error) {
	block, ok := raw.(map[string]any)
	if !ok {
		return routeConfig{}, errors.New("must be an object")
	}

	var cfg routeConfig
	var err error
	if cfg.modelHandle, err = requiredString(block, ParamModelHandle); err != nil {
		return routeConfig{}, err
	}
	if cfg.modelName, err = requiredString(block, ParamModelName); err != nil {
		return routeConfig{}, err
	}

	scope, ok := block[ParamScope].(map[string]any)
	if !ok {
		return routeConfig{}, fmt.Errorf("%s must be an object", ParamScope)
	}
	if cfg.scopeTemplate, err = requiredString(scope, ParamTemplateHandle); err != nil {
		return routeConfig{}, fmt.Errorf("%s: %w", ParamScope, err)
	}
	if cfg.scopeProvider, err = requiredString(scope, ParamProviderName); err != nil {
		return routeConfig{}, fmt.Errorf("%s: %w", ParamScope, err)
	}

	rawLocation, hasLocation := block[ParamModelLocation]
	if cfg.location, err = parseModelLocation(rawLocation, hasLocation); err != nil {
		return routeConfig{}, err
	}
	if cfg.format, err = parseFormat(block); err != nil {
		return routeConfig{}, err
	}

	rawTargets, ok := block[ParamTargets].([]any)
	if !ok || len(rawTargets) == 0 {
		return routeConfig{}, fmt.Errorf("%s must be a non-empty array", ParamTargets)
	}

	names := make(map[string]bool, len(rawTargets))
	pairs := make(map[[2]string]bool, len(rawTargets))
	for i, rawTarget := range rawTargets {
		target, err := parseRouteTarget(rawTarget)
		if err != nil {
			return routeConfig{}, fmt.Errorf("%s[%d]: %w", ParamTargets, i, err)
		}
		if names[target.name] {
			return routeConfig{}, fmt.Errorf("%s[%d]: duplicate target name %q", ParamTargets, i, target.name)
		}
		pair := [2]string{target.provider, target.upstreamModel}
		if pairs[pair] {
			return routeConfig{}, fmt.Errorf("%s[%d]: duplicate provider and upstream model pair", ParamTargets, i)
		}
		names[target.name] = true
		pairs[pair] = true
		cfg.targets = append(cfg.targets, target)
	}
	return cfg, nil
}

func parseRouteTarget(raw any) (routeTarget, error) {
	block, ok := raw.(map[string]any)
	if !ok {
		return routeTarget{}, errors.New("must be an object")
	}

	var target routeTarget
	var err error
	if target.name, err = requiredString(block, ParamTargetName); err != nil {
		return routeTarget{}, err
	}
	if target.provider, err = requiredString(block, ParamTargetProvider); err != nil {
		return routeTarget{}, err
	}
	if target.upstreamModel, err = requiredString(block, ParamTargetUpstreamModel); err != nil {
		return routeTarget{}, err
	}
	if target.template, err = requiredString(block, ParamTargetTemplate); err != nil {
		return routeTarget{}, err
	}

	// Required rather than defaulted. Defaulting to direct would strip the
	// loopback marker from a routed target whose flag went missing; defaulting
	// to routed would leave a client-supplied marker on a direct one. Neither
	// guess is safe, so the controller has to say.
	routed, ok := block[ParamTargetRouted].(bool)
	if !ok {
		return routeTarget{}, fmt.Errorf("%s must be a boolean", ParamTargetRouted)
	}
	target.routed = routed

	if rawName, present := block[ParamTargetDisplayName]; present {
		name, ok := rawName.(string)
		if !ok {
			return routeTarget{}, fmt.Errorf("%s must be a string", ParamTargetDisplayName)
		}
		target.displayName = name
	}
	return target, nil
}

// ─── route ───────────────────────────────────────────────────────────────────

// routePolicy is one Model chain's route job.
type routePolicy struct {
	cfg routeConfig
	// responseModel replaces the model the response reports with the Model's name.
	responseModel modelRewriter
}

func newRoutePolicy(cfg routeConfig) *routePolicy {
	return &routePolicy{cfg: cfg, responseModel: newModelRewriter(cfg.format, cfg.modelName)}
}

// Mode declares both request phases — the header phase, where a provider that is
// already known is committed, and the body phase, where the router's final choice
// is read and acted on — and every response phase, where the scope is switched
// back and the model the response reports is replaced with the Model's name.
//
// The response body is declared STREAM, never BUFFER, and the streaming interface
// is implemented: a buffered declaration would force every Model chain to buffer
// its response, turning a streamed completion into one that arrives when the
// stream ends. It adds no ext_proc round trip either: the analytics collector at
// the head of every Model chain already takes the response body on an LlmProxy
// route, so the kernel is processing those chunks regardless.
func (*routePolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeStream,
	}
}

// metadataHeaderProvider records the provider the header phase committed, so the
// body phase can refuse to move off it. Private to this job; underscore-prefixed
// like the other policies' internal keys.
const metadataHeaderProvider = "__model.header_provider"

// OnRequestHeaders commits the provider in the header phase, when it is already
// known there.
//
// It has to, because some of the provider-specific policies after this job decide
// in the header phase only — an oauth2-generator credential is one — and their
// selected_provider condition is evaluated in the header pass, which the kernel
// runs for the whole chain before any body-phase policy. A provider chosen only in
// the body phase would reach those policies too late, and the request would leave
// without its credential.
//
// The provider is known here in two cases: a router that decides from headers has
// already named one of the Model's providers, or every target is on one provider
// so there is nothing to decide. In either case selected_provider is set to it and
// the commitment is recorded for the body phase. Otherwise nothing is committed,
// and the body phase decides alone.
//
// The scope is not switched here. The policies after this job that read it do so
// in the body and response phases, and the body phase switches it once the final
// target is known.
func (p *routePolicy) OnRequestHeaders(
	_ context.Context,
	reqCtx *policy.RequestHeaderContext,
	_ map[string]any,
) policy.RequestHeaderAction {
	if reqCtx == nil || reqCtx.SharedContext == nil {
		// Nothing to commit to; the body phase refuses a request without a context.
		return policy.UpstreamRequestHeaderModifications{}
	}
	provider := p.cfg.headerProvider(metadataString(reqCtx.SharedContext, MetadataSelectedProvider))
	if provider == "" {
		return policy.UpstreamRequestHeaderModifications{}
	}
	if reqCtx.Metadata == nil {
		reqCtx.Metadata = make(map[string]any)
	}
	reqCtx.Metadata[MetadataSelectedProvider] = provider
	reqCtx.Metadata[metadataHeaderProvider] = provider
	return policy.UpstreamRequestHeaderModifications{}
}

// headerProvider is the provider the header phase can commit to: the router's
// choice when it names one of the Model's providers, else the Model's only
// provider, else none.
func (cfg routeConfig) headerProvider(selectedProvider string) string {
	if selectedProvider != "" {
		for _, target := range cfg.targets {
			if target.provider == selectedProvider {
				return selectedProvider
			}
		}
	}
	only := cfg.targets[0].provider
	for _, target := range cfg.targets[1:] {
		if target.provider != only {
			return ""
		}
	}
	return only
}

// OnRequestBody resolves the router's choice to a target and rewrites the request
// for it.
//
// It runs after the router in the body phase. By then a router that decides from
// headers has decided in the header phase, and one that decides from the body has
// decided earlier in this one, so selected_provider and the request's model are
// whatever the router left — or, with no router, what the client sent.
func (p *routePolicy) OnRequestBody(
	_ context.Context,
	reqCtx *policy.RequestContext,
	_ map[string]any,
) policy.RequestAction {
	if reqCtx == nil || reqCtx.SharedContext == nil {
		slog.Error("Model system policy: route job reached without a request context; refusing to forward",
			"model", p.cfg.modelHandle)
		return serverErrorResponse(p.cfg.format)
	}

	requested, rewrite, refusal := p.readModel(reqCtx)
	if refusal != nil {
		return *refusal
	}

	target := p.cfg.resolve(metadataString(reqCtx.SharedContext, MetadataSelectedProvider), requested)

	// A provider committed in the header phase has already had its header-phase
	// credential applied to this request. Moving to another provider now would send
	// that credential to the wrong upstream, so the request is refused instead —
	// a router that names one provider in the header phase and another in the body
	// is a configuration this job cannot honour safely.
	if committed := metadataString(reqCtx.SharedContext, metadataHeaderProvider); committed != "" && committed != target.provider {
		slog.Error("Model system policy: the body phase selected a different provider from the one committed in the header phase; refusing to forward",
			"api_id", reqCtx.APIId, "model", p.cfg.modelHandle,
			"committed_provider", committed, "selected_provider", target.provider)
		return routingConflictResponse(p.cfg.format)
	}

	mods := policy.UpstreamRequestModifications{}
	if err := rewrite(&mods, target.upstreamModel); err != nil {
		// Unreachable for a request readModel accepted; handled so a decoder that
		// disagreed with itself fails closed instead of forwarding the request
		// unrewritten to a provider that would serve the wrong model.
		slog.Error("Model system policy: could not rewrite the request's model; refusing to forward",
			"api_id", reqCtx.APIId, "model", p.cfg.modelHandle, "error", err)
		return serverErrorResponse(p.cfg.format)
	}

	p.recordTarget(reqCtx.SharedContext, target)

	upstream := target.provider
	mods.UpstreamName = &upstream
	if !target.routed {
		mods.HeadersToRemove = []string{InternalLoopbackHeader}
	}
	return mods
}

// rewriteFunc writes the target's upstream model into the request.
type rewriteFunc func(mods *policy.UpstreamRequestModifications, upstreamModel string) error

// readModel reads the model the request names where the endpoint's format carries
// it, and returns how to rewrite it there. A request it cannot read is refused:
// the resolver bound a Model chain only for a request naming the Model, so this
// is a policy earlier in the chain having rewritten the request into something
// else, and the provider would receive a request this job could not point at it.
func (p *routePolicy) readModel(reqCtx *policy.RequestContext) (string, rewriteFunc, *policy.ImmediateResponse) {
	loc := p.cfg.location
	switch loc.kind {
	case locationPathParam:
		path := reqCtx.Path
		start, end, ok := loc.pathSegment(path, reqCtx.APIContext)
		if !ok {
			slog.Debug("Model system policy: request path names no model; refusing to forward",
				"api_id", reqCtx.APIId, "model", p.cfg.modelHandle)
			refusal := serverErrorResponse(p.cfg.format)
			return "", nil, &refusal
		}
		requested, err := url.PathUnescape(path[start:end])
		if err != nil {
			requested = ""
		}
		return requested, func(mods *policy.UpstreamRequestModifications, upstreamModel string) error {
			rewritten := path[:start] + url.PathEscape(upstreamModel) + path[end:]
			mods.Path = &rewritten
			return nil
		}, nil
	case locationHeader:
		var requested string
		if reqCtx.Headers != nil {
			if values := reqCtx.Headers.Get(loc.name); len(values) > 0 {
				requested = strings.TrimSpace(values[0])
			}
		}
		return requested, func(mods *policy.UpstreamRequestModifications, upstreamModel string) error {
			mods.HeadersToSet = map[string]string{loc.name: upstreamModel}
			return nil
		}, nil
	case locationQueryParam:
		var requested string
		base, rawQuery, found := strings.Cut(reqCtx.Path, "?")
		if found {
			if query, err := url.ParseQuery(rawQuery); err == nil {
				requested = query.Get(loc.name)
			}
		}
		// Rewritten in the path rather than with QueryParametersToAdd, which the
		// engine appends: the provider would receive the client's value beside the
		// target's, and many read the first.
		return requested, func(mods *policy.UpstreamRequestModifications, upstreamModel string) error {
			rewritten := base + "?" + replaceQueryParameter(rawQuery, loc.name, upstreamModel)
			mods.Path = &rewritten
			return nil
		}, nil
	default:
		var body []byte
		if reqCtx.Body != nil {
			body = reqCtx.Body.Content
		}
		requested, ok := topLevelMember(body, loc.member)
		if !ok {
			slog.Debug("Model system policy: request body is not a JSON object; refusing to forward",
				"api_id", reqCtx.APIId, "model", p.cfg.modelHandle)
			refusal := invalidJSONResponse(p.cfg.format)
			return "", nil, &refusal
		}
		return requested, func(mods *policy.UpstreamRequestModifications, upstreamModel string) error {
			rewritten, err := rewriteTopLevelMember(body, loc.member, upstreamModel)
			if err != nil {
				return err
			}
			mods.Body = rewritten
			return nil
		}, nil
	}
}

// replaceQueryParameter returns rawQuery with the parameter name set to value: its
// first occurrence replaced in place, any later one dropped, and every other
// parameter left exactly as the client sent it. A query without the parameter gets
// it at the end.
func replaceQueryParameter(rawQuery, name, value string) string {
	param := url.QueryEscape(name) + "=" + url.QueryEscape(value)
	if rawQuery == "" {
		return param
	}
	parts := strings.Split(rawQuery, "&")
	out := make([]string, 0, len(parts)+1)
	replaced := false
	for _, part := range parts {
		key, _, _ := strings.Cut(part, "=")
		if decoded, err := url.QueryUnescape(key); err == nil && decoded == name {
			if !replaced {
				out = append(out, param)
				replaced = true
			}
			continue
		}
		out = append(out, part)
	}
	if !replaced {
		out = append(out, param)
	}
	return strings.Join(out, "&")
}

// pathSegment finds the model segment in a request path: the pattern's capture,
// matched against the path below the API context and without its query, so a
// context that happens to look like the pattern is never read as the model. It
// returns the segment's offsets in path.
func (loc modelLocation) pathSegment(path, apiContext string) (int, int, bool) {
	base, _, _ := strings.Cut(path, "?")
	offset := 0
	if ctx := strings.TrimRight(apiContext, "/"); ctx != "" && strings.HasPrefix(base, ctx) {
		if rest := base[len(ctx):]; rest == "" || rest[0] == '/' {
			offset = len(ctx)
		}
	}
	m := loc.pattern.FindStringSubmatchIndex(base[offset:])
	if m == nil || m[2] < 0 || m[3] <= m[2] {
		return 0, 0, false
	}
	return offset + m[2], offset + m[3], true
}

// recordTarget publishes the selected target: the selection the provider's
// conditional policies match on, the provider's scope for the policies between
// here and providerScope, and the facts the analytics collector reports.
func (p *routePolicy) recordTarget(shared *policy.SharedContext, target routeTarget) {
	if shared.Metadata == nil {
		shared.Metadata = make(map[string]any)
	}
	md := shared.Metadata
	md[MetadataSelectedProvider] = target.provider
	md[MetadataModelHandle] = p.cfg.modelHandle
	md[MetadataModelName] = p.cfg.modelName
	md[MetadataModelTarget] = target.name
	md[MetadataProviderHandle] = target.provider
	md[MetadataProviderTemplate] = target.template
	md[MetadataProviderDisplayName] = target.displayName
	md[MetadataUpstreamModel] = target.upstreamModel
	setScope(shared, target.template, target.provider)
}

// OnResponseHeaders switches the response back to the endpoint's inbound scope.
func (p *routePolicy) OnResponseHeaders(
	_ context.Context,
	respCtx *policy.ResponseHeaderContext,
	_ map[string]any,
) policy.ResponseHeaderAction {
	if respCtx != nil {
		setScope(respCtx.SharedContext, p.cfg.scopeTemplate, p.cfg.scopeProvider)
	}
	return policy.DownstreamResponseHeaderModifications{}
}

// OnResponseBody switches a buffered response back to the endpoint's inbound scope,
// and replaces the model it reports with the Model's name. It is the fallback the
// kernel calls when some other policy in the chain cannot stream.
func (p *routePolicy) OnResponseBody(
	_ context.Context,
	respCtx *policy.ResponseContext,
	_ map[string]any,
) policy.ResponseAction {
	if respCtx == nil {
		return policy.DownstreamResponseModifications{}
	}
	setScope(respCtx.SharedContext, p.cfg.scopeTemplate, p.cfg.scopeProvider)

	if respCtx.ResponseBody == nil || !respCtx.ResponseBody.Present || len(respCtx.ResponseBody.Content) == 0 {
		return policy.DownstreamResponseModifications{}
	}
	body := respCtx.ResponseBody.Content
	stream := newResponseModelStream(p.responseModel)
	rewritten := stream.feed(body, true)
	recordResponseModel(respCtx.SharedContext, stream.reported)
	if bytes.Equal(rewritten, body) {
		return policy.DownstreamResponseModifications{}
	}
	return policy.DownstreamResponseModifications{Body: rewritten}
}

// metadataResponseModelStream holds a streamed response's rewriter between its
// chunks. Private to this job.
const metadataResponseModelStream = "__model.response_model_stream"

// OnResponseBodyChunk switches each streamed chunk back to the endpoint's inbound
// scope, and replaces the model it reports with the Model's name.
//
// Per chunk, not once: the provider-specific policies between providerScope and
// this job run on every chunk, and providerScope switches into the provider's
// scope for them each time.
//
// The rewriter keeps what it has not finished reading in the request's shared
// context, which is the one place that lasts from one chunk to the next. Without
// a shared context the chunks are forwarded unchanged: rewriting each on its own
// would cut a model split across two of them in half.
func (p *routePolicy) OnResponseBodyChunk(
	_ context.Context,
	respCtx *policy.ResponseStreamContext,
	chunk *policy.StreamBody,
	_ map[string]any,
) policy.StreamingResponseAction {
	if respCtx == nil {
		return policy.ForwardResponseChunk{}
	}
	shared := respCtx.SharedContext
	setScope(shared, p.cfg.scopeTemplate, p.cfg.scopeProvider)
	if shared == nil || chunk == nil {
		return policy.ForwardResponseChunk{}
	}

	if shared.Metadata == nil {
		shared.Metadata = make(map[string]any)
	}
	stream, ok := shared.Metadata[metadataResponseModelStream].(*responseModelStream)
	if !ok {
		stream = newResponseModelStream(p.responseModel)
		shared.Metadata[metadataResponseModelStream] = stream
	}
	out := stream.feed(chunk.Chunk, chunk.EndOfStream)
	recordResponseModel(shared, stream.reported)
	// out is never nil: nil would forward the chunk as it arrived, and the bytes the
	// rewriter is holding would then be forwarded twice.
	return policy.ForwardResponseChunk{Body: out}
}

// NeedsMoreResponseData is false. The model rewrite holds back what it cannot yet
// read itself rather than asking the kernel to: what the kernel accumulates is the
// provider's response, and what this job reads is that response after the
// provider's transformer has turned it into the endpoint's format.
func (*routePolicy) NeedsMoreResponseData([]byte) bool { return false }

// recordResponseModel records the model the provider's response named, once, for
// the analytics collector: the response the collector reads names the Model.
func recordResponseModel(shared *policy.SharedContext, reported string) {
	if shared == nil || reported == "" || metadataString(shared, MetadataResponseModel) != "" {
		return
	}
	if shared.Metadata == nil {
		shared.Metadata = make(map[string]any)
	}
	shared.Metadata[MetadataResponseModel] = reported
}

// resolve maps the router's choice to one of the Model's targets.
//
// The order is the Model routing contract:
//
//  1. the exact (provider, model) pair, which is what a router that names both
//     chose;
//  2. the target whose name is the model, when the provider, if any, is that
//     target's — a router configured with the Model's own names for its targets
//     (`model: fast`) rather than with their upstream models;
//  3. the provider alone, when exactly one target uses it — a router that names
//     only a provider, such as the header router;
//  4. the model alone, when exactly one target uses it — a router that names only
//     a model, and the no-router case where the client's model happens to be a
//     target's upstream model;
//  5. otherwise the first target, the Model's default and the same primary
//     semantics an LlmProxy has.
//
// A name comes before an upstream model alone because target names are unique,
// so a name never matches more than one target: a value that is one target's name
// and another's upstream model goes to the target it names.
//
// Whatever the router chose, the answer is one of the targets: a provider or
// model the Model does not name is never forwarded, because the upstream and the
// body are both rewritten from the target this returns.
func (cfg routeConfig) resolve(selectedProvider, model string) routeTarget {
	if selectedProvider != "" && model != "" {
		for _, target := range cfg.targets {
			if target.provider == selectedProvider && target.upstreamModel == model {
				return target
			}
		}
	}
	if model != "" {
		for _, target := range cfg.targets {
			if target.name == model && (selectedProvider == "" || target.provider == selectedProvider) {
				return target
			}
		}
	}
	if selectedProvider != "" {
		if target, ok := cfg.single(func(t routeTarget) bool { return t.provider == selectedProvider }); ok {
			return target
		}
	}
	if model != "" {
		if target, ok := cfg.single(func(t routeTarget) bool { return t.upstreamModel == model }); ok {
			return target
		}
	}
	return cfg.targets[0]
}

// single returns the one target matching, or false when none or several do.
func (cfg routeConfig) single(match func(routeTarget) bool) (routeTarget, bool) {
	var found routeTarget
	count := 0
	for _, target := range cfg.targets {
		if match(target) {
			found = target
			count++
		}
	}
	return found, count == 1
}

// ─── providerScope ───────────────────────────────────────────────────────────

// providerScopePolicy is the response-side half of the bracket. It is stateless:
// the provider it switches to is the one the route job recorded for this request.
type providerScopePolicy struct{}

var providerScopeInstance = &providerScopePolicy{}

// Mode declares only the response phases, streaming like the route job and for
// the same reason.
func (*providerScopePolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeStream,
	}
}

// switchToProvider sets the selected provider's scope, or leaves the scope alone
// when no target was recorded — a request a policy refused before the route job
// ran, whose response is the gateway's own and is in the endpoint's format.
func switchToProvider(shared *policy.SharedContext) {
	template := metadataString(shared, MetadataProviderTemplate)
	provider := metadataString(shared, MetadataProviderHandle)
	if template == "" || provider == "" {
		return
	}
	setScope(shared, template, provider)
}

func (*providerScopePolicy) OnResponseHeaders(
	_ context.Context,
	respCtx *policy.ResponseHeaderContext,
	_ map[string]any,
) policy.ResponseHeaderAction {
	if respCtx != nil {
		switchToProvider(respCtx.SharedContext)
	}
	return policy.DownstreamResponseHeaderModifications{}
}

func (*providerScopePolicy) OnResponseBody(
	_ context.Context,
	respCtx *policy.ResponseContext,
	_ map[string]any,
) policy.ResponseAction {
	if respCtx != nil {
		switchToProvider(respCtx.SharedContext)
	}
	return policy.DownstreamResponseModifications{}
}

func (*providerScopePolicy) OnResponseBodyChunk(
	_ context.Context,
	respCtx *policy.ResponseStreamContext,
	_ *policy.StreamBody,
	_ map[string]any,
) policy.StreamingResponseAction {
	if respCtx != nil {
		switchToProvider(respCtx.SharedContext)
	}
	return policy.ForwardResponseChunk{}
}

// NeedsMoreResponseData is false: nothing here reads the chunk.
func (*providerScopePolicy) NeedsMoreResponseData([]byte) bool { return false }

// ─── the request's top-level model member ────────────────────────────────────

// topLevelMember reports whether body is a single JSON object, and the value of
// its top-level member when that is a string.
//
// A model that is absent or not a string reads as "", which resolution treats as
// "no model chosen". When the member is repeated the last one is reported, which
// is what encoding/json — and so most upstreams — would take.
func topLevelMember(body []byte, member string) (string, bool) {
	if !json.Valid(body) {
		return "", false
	}
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return "", false
	}

	model := ""
	err := walkTopLevelMembers(body, func(key string, _, _ int64, value json.RawMessage) {
		if key != member {
			return
		}
		var s string
		if json.Unmarshal(value, &s) == nil {
			model = s
		} else {
			model = ""
		}
	})
	if err != nil {
		return "", false
	}
	return model, true
}

// rewriteTopLevelMember returns body with every top-level member's value replaced
// by model, and every other byte untouched.
//
// It splices rather than decoding and re-encoding the object. A round trip through
// a Go map would reorder the members, re-escape strings and normalise numbers —
// harmless to most providers, but it changes a request the client may have signed
// or hashed, and it is the kind of difference that only shows up as a provider
// rejecting something it accepted directly. Every top-level occurrence is
// replaced, not just one, so a body repeating the member cannot slip a second
// model past the rewrite to an upstream that reads the other occurrence.
//
// A body without the member gets one, as the object's first member.
func rewriteTopLevelMember(body []byte, member, model string) ([]byte, error) {
	encoded, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}

	type span struct{ start, end int64 }
	var spans []span
	err = walkTopLevelMembers(body, func(key string, start, end int64, _ json.RawMessage) {
		if key == member {
			spans = append(spans, span{start, end})
		}
	})
	if err != nil {
		return nil, err
	}

	if len(spans) == 0 {
		open := bytes.IndexByte(body, '{')
		if open < 0 {
			return nil, errors.New("body is not a JSON object")
		}
		name, err := json.Marshal(member)
		if err != nil {
			return nil, err
		}
		var out bytes.Buffer
		out.Grow(len(body) + len(encoded) + len(name) + 2)
		out.Write(body[:open+1])
		out.Write(name)
		out.WriteByte(':')
		out.Write(encoded)
		if hasMembers(body[open+1:]) {
			out.WriteByte(',')
		}
		out.Write(body[open+1:])
		return out.Bytes(), nil
	}

	var out bytes.Buffer
	out.Grow(len(body) + len(encoded)*len(spans))
	var cursor int64
	for _, s := range spans {
		out.Write(body[cursor:s.start])
		out.Write(encoded)
		cursor = s.end
	}
	out.Write(body[cursor:])
	return out.Bytes(), nil
}

// hasMembers reports whether what follows an object's opening brace is anything
// other than its closing one.
func hasMembers(rest []byte) bool {
	trimmed := bytes.TrimLeft(rest, " \t\r\n")
	return len(trimmed) > 0 && trimmed[0] != '}'
}

// walkTopLevelMembers calls visit for each member of the JSON object in body, with
// the byte span of its value. body must already be known to be valid JSON.
//
// The span comes from the decoder's own offsets: the value decoded into a
// RawMessage is the exact input bytes of that value, and the decoder's offset
// after it is the value's end, so its start is the end less its length.
func walkTopLevelMembers(body []byte, visit func(key string, start, end int64, value json.RawMessage)) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	open, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := open.(json.Delim); !ok || delim != '{' {
		return errors.New("body is not a JSON object")
	}
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return errors.New("object member has no string key")
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return err
		}
		end := dec.InputOffset()
		visit(key, end-int64(len(value)), end, value)
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("body carries more than one JSON value")
	}
	return nil
}

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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/wso2/api-platform/common/chainkey"
)

// LLMModelDispatchResolverName is the registered name of the model dispatch resolver,
// matching the resolver_name the controller writes on a model endpoint's dispatch
// routes.
const LLMModelDispatchResolverName = "llm-model-dispatch"

// LLMModelDispatchResolver resolves a request on a model endpoint to the policy chain of
// the Model it names.
//
// An endpoint serves every Model it names on one set of routes, and the request says
// which Model it wants where the endpoint's inbound format carries the model: the
// top-level `model` member of an OpenAI or Anthropic body, or a path segment of a Gemini
// request. The controller passes that location from the endpoint's inbound template
// (modelLocation). A body location buffers the body and reads that one member, the way
// the A2A JSON-RPC resolver reads `method`; any other location resolves at the header
// phase from the path, a header or the query, so nothing is buffered before the
// endpoint's own policies run. Each Model compiles to its own chain under the
// endpoint's partition — its own policies, its own router, its own targets — and the
// chain key is composed from the Model's handle.
//
// A request that names no usable Model is the one place this resolver departs from the
// others. It is not a resolution failure. A failure renders before any chain is bound,
// so it would answer before the endpoint's authentication had run, and a caller with no
// credential could then enumerate which model names exist by watching 404s turn into
// 401s. Instead every such request binds the route's reserved *error chain*, which the
// controller builds as analytics, then the endpoint's own policies (authentication first
// among them), then a system policy that answers in the endpoint's inbound format. The
// reason travels as an attribute, so that policy knows what to say without parsing the
// body again.
//
// The resolver reads the body as an exact client of that format would: the member the
// inbound template names (`model` for OpenAI and Anthropic), spelled exactly, appearing
// once. encoding/json folds member names and keeps the last match, so a body naming the
// model twice, or as "Model", reads one way here and possibly another way in a strict
// upstream. Such a body is an invalid model rather than a guess, for the same reason
// mcp_ambiguity.go exists.
type LLMModelDispatchResolver struct{}

func init() {
	RegisterDefault(&LLMModelDispatchResolver{})
}

// Name returns the wire value for model dispatch.
func (*LLMModelDispatchResolver) Name() string { return LLMModelDispatchResolverName }

// Attribute names this resolver publishes. Dot-segmented and namespaced like the A2A
// and MCP attributes, since all of them share one map.
const (
	// AttrLLMModelName is the model name the request asked for, verbatim. Published on a
	// successful dispatch and, when it fits the cap, on model_not_found — the error chain
	// may log it. It is the caller's own input either way.
	AttrLLMModelName = "llm.model.name"

	// AttrLLMModelHandle is the handle of the Model whose chain was bound. Published only on
	// a successful dispatch; it is the operation component of the bound chain key, so it
	// cannot name a different Model than the one whose policies run.
	AttrLLMModelHandle = "llm.model.handle"

	// AttrLLMRequestStream is "true" when the request asked for a streamed response —
	// `"stream": true` in an OpenAI or Anthropic body, a `:streamGenerateContent` path in
	// Gemini — and "false" otherwise. Absent when the signal could not be read: no body,
	// a body naming `stream` ambiguously, or a body signal on a route that reads none.
	AttrLLMRequestStream = "llm.request.stream"

	// AttrLLMDispatchError is why a request was bound to the error chain. One of the
	// LLMDispatchError* values below, and absent on a successful dispatch.
	AttrLLMDispatchError = "llm.dispatch.error"
)

// Values for AttrLLMDispatchError. A closed set, so the attribute stays span-safe and the
// error-chain policy can map each one to a fixed error in the endpoint's format without
// parsing anything.
const (
	// LLMDispatchInvalidJSON is a body that is not a single well-formed JSON object: no
	// body at all, unparseable JSON, or a top-level array or scalar.
	LLMDispatchInvalidJSON = "invalid_json"
	// LLMDispatchMissingModel is a request that names no model where its format carries
	// one: an object with no `model`, or one whose value is null or the empty string; a
	// path, header or query parameter without one.
	LLMDispatchMissingModel = "missing_model"
	// LLMDispatchInvalidModel is a `model` that is present but not a string, or that is
	// named more than once or under a non-canonical spelling; or a path, header or query
	// value that is repeated or does not decode.
	LLMDispatchInvalidModel = "invalid_model"
	// LLMDispatchModelNotFound is a string model that no deployed Model answers to.
	LLMDispatchModelNotFound = "model_not_found"
)

// MaxLLMModelNameBytes is the longest model name the resolver will look up or publish. The
// controller refuses to deploy a Model with a longer name, so a longer one in a request is
// an unknown model without a lookup, and is never copied into an attribute.
const MaxLLMModelNameBytes = 256

// LLMModelDispatchConfig is the route's resolver_config, as the controller writes it.
//
// Unknown fields are tolerated: a newer controller may add one, and a route this binary
// can still serve correctly should not be dropped over a field it does not read.
type LLMModelDispatchConfig struct {
	// Models maps a public model name to the operation — the Model's handle — whose chain
	// it binds. Empty is valid: an endpoint serving no Models answers every request with
	// model_not_found from its error chain.
	Models map[string]string `json:"models"`

	// ErrorOperation is the operation of the route's reserved error chain.
	ErrorOperation string `json:"errorOperation"`

	// ModelLocation is where requests carry the model name, from the endpoint's inbound
	// template's requestModel. Absent — a controller that predates it — is the OpenAI
	// body member `$.model`.
	ModelLocation *LLMValueLocation `json:"modelLocation,omitempty"`

	// Stream is where requests say they want a streamed response: a payload member that
	// is literally true, or a path pattern that matches. Absent is `$.stream`.
	Stream *LLMValueLocation `json:"stream,omitempty"`
}

// LLMValueLocation is a {location, identifier} pair, the shape provider templates and
// routing policies use.
type LLMValueLocation struct {
	Location   string `json:"location"`
	Identifier string `json:"identifier"`
}

// Location values.
const (
	llmLocationPayload    = "payload"
	llmLocationPathParam  = "pathParam"
	llmLocationHeader     = "header"
	llmLocationQueryParam = "queryParam"
)

// maxLLMModelPatternBytes bounds a path pattern; the controller writes it from a
// template, so a longer one is a construction error.
const maxLLMModelPatternBytes = 512

var (
	topLevelMemberIdentifier = regexp.MustCompile(`^\$\.([A-Za-z_][A-Za-z0-9_\-]*)$`)
	headerNameIdentifier     = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+.^_` + "`" + `|~-]+$`)
)

// preparedLocation is a location compiled once at ingest.
type preparedLocation struct {
	kind    string
	member  string         // payload: the top-level member name
	pattern *regexp.Regexp // pathParam
	name    string         // header (lower-cased) or query parameter name
}

// prepareLocation compiles a model or stream location. capture requires a path
// pattern to have exactly one capture group (the model); a stream pattern is a
// match only.
func prepareLocation(loc *LLMValueLocation, fallbackMember string, capture bool) (preparedLocation, error) {
	if loc == nil {
		return preparedLocation{kind: llmLocationPayload, member: fallbackMember}, nil
	}
	id := strings.TrimSpace(loc.Identifier)
	switch loc.Location {
	case llmLocationPayload:
		m := topLevelMemberIdentifier.FindStringSubmatch(id)
		if m == nil {
			// The resolver runs before the endpoint's authentication, so it reads one
			// top-level member and never follows a nested path into the body.
			return preparedLocation{}, fmt.Errorf("payload location %q is not a top-level member ($.<name>)", id)
		}
		return preparedLocation{kind: llmLocationPayload, member: m[1]}, nil
	case llmLocationPathParam:
		if id == "" || len(id) > maxLLMModelPatternBytes {
			return preparedLocation{}, fmt.Errorf("path pattern must be 1 to %d bytes", maxLLMModelPatternBytes)
		}
		re, err := regexp.Compile(id)
		if err != nil {
			return preparedLocation{}, fmt.Errorf("path pattern %q does not compile: %w", id, err)
		}
		if capture && re.NumSubexp() != 1 {
			return preparedLocation{}, fmt.Errorf("path pattern %q must have exactly one capture group", id)
		}
		return preparedLocation{kind: llmLocationPathParam, pattern: re}, nil
	case llmLocationHeader:
		if !headerNameIdentifier.MatchString(id) {
			return preparedLocation{}, fmt.Errorf("header location %q is not a header name", id)
		}
		return preparedLocation{kind: llmLocationHeader, name: strings.ToLower(id)}, nil
	case llmLocationQueryParam:
		if id == "" || strings.ContainsAny(id, "&=#? ") {
			return preparedLocation{}, fmt.Errorf("query location %q is not a parameter name", id)
		}
		return preparedLocation{kind: llmLocationQueryParam, name: id}, nil
	default:
		return preparedLocation{}, fmt.Errorf("location %q is not one of payload, pathParam, header, queryParam", loc.Location)
	}
}

// Prepare builds the route's resolver, composing every chain key up front so a request
// costs a map lookup and no string building.
//
// Any rejection here drops this one route; the controller generated every value, so an
// unusable one is a construction error to refuse, not a request outcome to tolerate.
func (*LLMModelDispatchResolver) Prepare(cfg ResolverRouteConfig) (PreparedResolver, error) {
	if len(cfg.ResolverConfig) == 0 {
		return nil, errors.New("llm-model-dispatch resolver: route carries no resolver_config")
	}
	var routeCfg LLMModelDispatchConfig
	if err := json.Unmarshal(cfg.ResolverConfig, &routeCfg); err != nil {
		return nil, fmt.Errorf("llm-model-dispatch resolver: resolver_config is not valid JSON: %w", err)
	}

	// The same partition checks as every protocol resolver: a key composed from an
	// unusable component would reach a chain belonging to some other triple.
	if !chainkey.ValidComponent(cfg.APIID) {
		return nil, fmt.Errorf("llm-model-dispatch resolver: route %q has an unusable API id", cfg.RouteKey)
	}
	if strings.Contains(cfg.Vhost, chainkey.Separator) {
		return nil, fmt.Errorf("llm-model-dispatch resolver: route %q has an unusable vhost", cfg.RouteKey)
	}

	// Required, because every request this resolver cannot dispatch binds it. A route
	// without one would have nowhere to send a request naming no model except a failure,
	// which is the unauthenticated answer the error chain exists to avoid.
	if !chainkey.ValidComponent(routeCfg.ErrorOperation) {
		return nil, fmt.Errorf("llm-model-dispatch resolver: route %q names no usable errorOperation", cfg.RouteKey)
	}

	chainKeys := make(map[string]string, len(routeCfg.Models))
	for name, operation := range routeCfg.Models {
		if name == "" || len(name) > MaxLLMModelNameBytes {
			return nil, fmt.Errorf(
				"llm-model-dispatch resolver: route %q maps a model name that is empty or longer than %d bytes",
				cfg.RouteKey, MaxLLMModelNameBytes)
		}
		if !chainkey.ValidComponent(operation) {
			return nil, fmt.Errorf("llm-model-dispatch resolver: route %q maps model %q to an unusable operation",
				cfg.RouteKey, name)
		}
		// An operation equal to the error operation would let a Model's name bind the
		// error chain, or the error chain answer as a Model. The controller reserves the
		// name; this is the check that it did.
		if operation == routeCfg.ErrorOperation {
			return nil, fmt.Errorf("llm-model-dispatch resolver: route %q maps model %q to the reserved error operation",
				cfg.RouteKey, name)
		}
		chainKeys[name] = ChainKeyFor(cfg.APIID, cfg.Vhost, operation)
	}

	model, err := prepareLocation(routeCfg.ModelLocation, "model", true)
	if err != nil {
		return nil, fmt.Errorf("llm-model-dispatch resolver: route %q modelLocation: %w", cfg.RouteKey, err)
	}
	stream, err := prepareLocation(routeCfg.Stream, "stream", false)
	if err != nil {
		return nil, fmt.Errorf("llm-model-dispatch resolver: route %q stream: %w", cfg.RouteKey, err)
	}

	return &preparedLLMModelDispatch{
		chainKeys:     chainKeys,
		operations:    routeCfg.Models,
		errorChainKey: ChainKeyFor(cfg.APIID, cfg.Vhost, routeCfg.ErrorOperation),
		model:         model,
		stream:        stream,
		apiContext:    strings.TrimRight(cfg.APIContext, "/"),
	}, nil
}

// preparedLLMModelDispatch is one model endpoint dispatch route, fixed at ingest.
type preparedLLMModelDispatch struct {
	// chainKeys maps a model name to its finished chain key, and operations to the handle
	// it was composed from. Two maps built in one loop, so an attribute and the chain key
	// it describes come from the same entry.
	chainKeys  map[string]string
	operations map[string]string

	errorChainKey string

	// model and stream are where the request carries them; apiContext is stripped
	// from the path before a path pattern is matched, so a context that happens to
	// look like the pattern can never be read as the model.
	model      preparedLocation
	stream     preparedLocation
	apiContext string
}

// Requirements asks for the buffered body only when the model is in it.
//
// Then the body is buffered before authentication runs, bounded by the route's
// max_request_body_bytes and the listener's buffer limit. That is the same trade every
// body-resolved route makes, and the reason this resolver reads one member and keeps
// nothing else. A model carried in the path, a header or the query is resolved at the
// header phase, so the endpoint's authentication runs before any body is buffered.
func (r *preparedLLMModelDispatch) Requirements() RequestRequirements {
	if r.model.kind == llmLocationPayload {
		return RequestRequirements{Body: BodyBuffered}
	}
	return RequestRequirements{Body: BodyNotRequired}
}

// Resolve reads the model where the route says and binds that Model's chain, or the
// error chain with the reason it was chosen.
//
// It never returns an error. Every outcome is a chain under this route's partition, so
// the binder validates each the same way, and a request that names nothing usable is
// answered by policies that run after authentication rather than by the engine before it.
func (r *preparedLLMModelDispatch) Resolve(_ context.Context, view RequestView) (Resolution, error) {
	if r.model.kind != llmLocationPayload {
		return r.resolveOutsideBody(view), nil
	}

	body := trimLeadingJSONSpace(view.Body)
	if len(body) == 0 || body[0] != '{' {
		// No body, whitespace only, or a top-level array or scalar. Checked before the
		// member scan so a batch is reported as malformed rather than decoded.
		return r.dispatchError(LLMDispatchInvalidJSON, nil), nil
	}

	members, ok := jsonObjectMembers(body)
	if !ok {
		return r.dispatchError(LLMDispatchInvalidJSON, nil), nil
	}

	attrs := make(map[string]string, 3)
	r.streamAttribute(view, members, attrs)

	modelRaw, modelState := exactMember(members, r.model.member)
	switch modelState {
	case memberAbsent:
		return r.dispatchError(LLMDispatchMissingModel, attrs), nil
	case memberAmbiguous:
		return r.dispatchError(LLMDispatchInvalidModel, attrs), nil
	}

	trimmed := bytes.TrimSpace(modelRaw)
	if bytes.Equal(trimmed, []byte("null")) {
		return r.dispatchError(LLMDispatchMissingModel, attrs), nil
	}
	var name string
	if len(trimmed) == 0 || trimmed[0] != '"' || json.Unmarshal(trimmed, &name) != nil {
		return r.dispatchError(LLMDispatchInvalidModel, attrs), nil
	}
	return r.bindName(name, attrs), nil
}

// resolveOutsideBody reads a model carried in the path, a header or the query.
func (r *preparedLLMModelDispatch) resolveOutsideBody(view RequestView) Resolution {
	attrs := make(map[string]string, 3)
	r.streamAttribute(view, nil, attrs)

	path, rawQuery, _ := strings.Cut(view.Path, "?")
	var name string
	switch r.model.kind {
	case llmLocationPathParam:
		m := r.model.pattern.FindStringSubmatch(r.relativePath(path))
		if m == nil || m[1] == "" {
			return r.dispatchError(LLMDispatchMissingModel, attrs)
		}
		decoded, err := url.PathUnescape(m[1])
		if err != nil {
			return r.dispatchError(LLMDispatchInvalidModel, attrs)
		}
		name = decoded
	case llmLocationHeader:
		var values []string
		for key, v := range view.Headers {
			if strings.EqualFold(key, r.model.name) {
				values = append(values, v...)
			}
		}
		switch len(values) {
		case 0:
			return r.dispatchError(LLMDispatchMissingModel, attrs)
		case 1:
			name = strings.TrimSpace(values[0])
		default:
			return r.dispatchError(LLMDispatchInvalidModel, attrs)
		}
	case llmLocationQueryParam:
		query, err := url.ParseQuery(rawQuery)
		if err != nil {
			return r.dispatchError(LLMDispatchInvalidModel, attrs)
		}
		switch values := query[r.model.name]; len(values) {
		case 0:
			return r.dispatchError(LLMDispatchMissingModel, attrs)
		case 1:
			name = values[0]
		default:
			return r.dispatchError(LLMDispatchInvalidModel, attrs)
		}
	}
	return r.bindName(name, attrs)
}

// relativePath strips the API context from a path, so a path pattern is matched
// against the part the endpoint's operations describe.
func (r *preparedLLMModelDispatch) relativePath(path string) string {
	if r.apiContext != "" && strings.HasPrefix(path, r.apiContext) {
		rest := path[len(r.apiContext):]
		if rest == "" || rest[0] == '/' {
			return rest
		}
	}
	return path
}

// streamAttribute publishes whether the request asked for a stream. members is the
// decoded body's members, nil when the body was not read.
func (r *preparedLLMModelDispatch) streamAttribute(view RequestView, members []jsonMember, attrs map[string]string) {
	switch r.stream.kind {
	case llmLocationPayload:
		if members == nil {
			return
		}
		streamRaw, streamState := exactMember(members, r.stream.member)
		if streamState == memberAmbiguous {
			// The upstream may read it either way, and a guess here would mislabel the
			// response a policy expects.
			return
		}
		// `stream` defaults to false, so absent and every non-true value read as "false".
		attrs[AttrLLMRequestStream] = "false"
		if streamState == memberPresent && bytes.Equal(bytes.TrimSpace(streamRaw), []byte("true")) {
			attrs[AttrLLMRequestStream] = "true"
		}
	case llmLocationPathParam:
		path, _, _ := strings.Cut(view.Path, "?")
		attrs[AttrLLMRequestStream] = "false"
		if r.stream.pattern.MatchString(r.relativePath(path)) {
			attrs[AttrLLMRequestStream] = "true"
		}
	}
}

// bindName binds the chain of the Model a name names, or the error chain.
func (r *preparedLLMModelDispatch) bindName(name string, attrs map[string]string) Resolution {
	if name == "" {
		return r.dispatchError(LLMDispatchMissingModel, attrs)
	}
	// Exact match: a model name is case-sensitive at every provider, and a near-miss is
	// a client asking for a model that does not exist, not a value to normalise into one
	// that does. A name over the cap was never deployable, so it is not looked up.
	key, known := "", false
	if len(name) <= MaxLLMModelNameBytes {
		key, known = r.chainKeys[name]
	}
	if !known {
		if len(name) <= MaxLLMModelNameBytes {
			attrs[AttrLLMModelName] = name
		}
		return r.dispatchError(LLMDispatchModelNotFound, attrs)
	}

	attrs[AttrLLMModelName] = name
	attrs[AttrLLMModelHandle] = r.operations[name]
	return Resolution{ChainKey: key, Attributes: attrs}
}

// dispatchError binds the error chain with the reason. attrs may be nil.
func (r *preparedLLMModelDispatch) dispatchError(reason string, attrs map[string]string) Resolution {
	if attrs == nil {
		attrs = make(map[string]string, 1)
	}
	attrs[AttrLLMDispatchError] = reason
	return Resolution{ChainKey: r.errorChainKey, Attributes: attrs}
}

// memberState is what a body says about one member name.
type memberState uint8

const (
	memberAbsent memberState = iota
	memberPresent
	memberAmbiguous
)

// exactMember finds the one member spelled exactly name.
//
// A body naming it more than once, or naming it under a spelling encoding/json would fold
// onto it ("Model", "MODEL"), is ambiguous — see foldsOntoOneName, whose rule this is,
// applied to one name.
func exactMember(members []jsonMember, name string) (json.RawMessage, memberState) {
	var value json.RawMessage
	state := memberAbsent
	for _, member := range members {
		if !strings.EqualFold(member.name, name) {
			continue
		}
		if member.name != name || state == memberPresent {
			return nil, memberAmbiguous
		}
		value, state = member.value, memberPresent
	}
	return value, state
}

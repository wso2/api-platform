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

// Package a2a is the gateway's internal A2A system policy: the parts of the A2A
// protocol the gateway answers itself rather than proxying to the agent behind
// it. It is attached by the Agent transformer, never by an Agent author.
//
// It does two such jobs, and each takes its own parameter block: serving a
// managed public Agent Card, and answering the protected (extended) Agent Card
// operation. The package and the policy are named for the protocol rather than
// for either of them so a further gateway-answered A2A concern joins this policy
// instead of adding another entry to the chain, another module, and another
// build-lock line. Nothing is shared between the blocks beyond the name: a chain
// carries one instance per job, and which block an instance holds is the whole
// of what that instance does.
//
// # Public Agent Card serving
//
// This exists as a policy rather than as route configuration because the card
// route needs a real policy chain: an Envoy DirectResponse route has no
// ext_proc interaction at all, which would take the CORS preflight and the
// system observability policies down with it. So the route is an ordinary
// proxying route, and this policy answers it from the request-header phase with
// an ImmediateResponse — the request never reaches the upstream.
//
// It is also not the existing `respond` policy, for one reason: conditional GET.
// A card is fetched repeatedly by every client that talks to the agent and
// changes only when the Agent is redeployed, so it is exactly the shape
// If-None-Match exists for. `respond` always answers 200 with a body.
//
// # Protected (extended) Agent Card
//
// The protected card is an A2A *operation*, not a document at a path: it is
// reached through GetExtendedAgentCard on whichever transports the Agent
// configures, and it shares that operation's canonical policy chain. The
// controller attaches this policy at the tail of that chain, after every policy
// the Agent author attached at either scope, so their policies decide the
// request first — the order they configure is theirs, and the gateway adds
// itself at the end of it rather than in the middle.
//
// What the instance then enforces is one thing: that the request reaching it was
// authenticated. It deliberately does not look at policy names, or at which
// scope the authentication came from. SharedContext.AuthContext is populated
// identically by every supported auth policy and is independent of transport and
// credential type, so a custom authentication policy protects this surface
// exactly as well as a built-in one. A deployment-time allowlist of known auth
// policy names would reject a custom one and still could not prove that a policy
// bearing an approved name had actually authenticated this request.
//
// The consequence is the one worth having: an Agent that configures a protected
// card but attaches no authentication at all answers 401, rather than publishing
// its extended card to anyone who asks.
//
// In managed mode the instance carries the card and answers with it. In
// passthrough mode it carries none and forwards the operation to the upstream
// once the check passes.
package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Parameter names.
//
// These are written by the gateway controller's Agent transformer
// (pkg/transform/agent.go, agentCardPolicyInstance and
// protectedCardPolicyInstance) and read here. The two live in separate Go
// modules and cannot share a constant, so the names are spelled once on each
// side and the coupling is asserted by a test on each side.
const (
	// ParamAgentCard is the block configuring public Agent Card serving. It is
	// nested rather than flattened because the policy is named for the protocol,
	// not for this one job: every other gateway-answered A2A concern gets a
	// sibling block, and none has to negotiate for a top-level parameter name.
	ParamAgentCard = "agentCard"

	// ParamContent is the exact card bytes to serve, as a JSON string.
	//
	// It is the serialized document, not the document itself, because the bytes
	// are what matters: the controller validated these bytes, and once card
	// signing lands it will sign these bytes. Passing the card as a structured
	// object would have this policy re-serialize it, and a signature computed
	// over the controller's encoding would not verify against the runtime's.
	ParamContent = "content"

	// ParamETag is the strong entity tag for ParamContent, quoted, ready to put
	// straight into an ETag header. Computed by the controller from the same
	// bytes, so a card and its tag cannot disagree.
	//
	// Public card serving only. A protected card response is authenticated and
	// uncacheable, and its JSON-RPC binding is a POST, so there is no conditional
	// GET for it to participate in.
	ParamETag = "etag"

	// ParamProtectedAgentCard is the block configuring the protected (extended)
	// Agent Card operation. Present in both protected modes; ParamContent inside
	// it is what distinguishes them, because it is the card to answer with and a
	// passthrough card has none.
	//
	// An instance holding no recognised block at all is a controller defect and
	// fails closed with a 500 rather than forwarding, so a block that went
	// missing cannot become a bypass.
	ParamProtectedAgentCard = "protectedAgentCard"
)

// A2A resolution attribute names, as the a2a resolver in the policy engine
// spells them into SharedContext.ResolutionAttributes.
//
// Read, never written: the resolver owns that map. The engine's copy lives in an
// internal package of another module, so the literal is mirrored here and pinned
// by a test on each side — the same arrangement the analytics system policy uses
// for the same names.
const (
	attrA2ATransport = "a2a.transport"

	// Transport values, matching the management API's protocolBinding enum.
	transportJSONRPC  = "JSONRPC"
	transportHTTPJSON = "HTTP+JSON"
)

const (
	// contentTypeJSON is the media type A2A Agent Cards are served as.
	contentTypeJSON = "application/json"

	// jsonRPCVersion is the only JSON-RPC version A2A uses.
	jsonRPCVersion = "2.0"

	// cacheControlNoStore is what an authenticated response carries.
	//
	// The public card gets a validator and revalidation (cacheControlValue
	// below); this one gets neither. A protected card is returned only to a
	// caller the gateway authenticated, so any shared cache holding it would be
	// holding one principal's authenticated response for another's request.
	cacheControlNoStore = "no-store"

	// cacheControlValue lets a client store the card but requires it to
	// revalidate before reusing it.
	//
	// A card is a routing and authentication discovery document: a stale one
	// sends requests to a path the gateway no longer serves, or tells a client
	// to authenticate in a way the gateway no longer accepts, and the client has
	// no way to tell. Forcing revalidation is what makes the ETag load-bearing
	// rather than decorative — the common case becomes a conditional GET
	// answered with 304 and no body, which is cheap, instead of a heuristically
	// cached copy of unknown age.
	cacheControlValue = "no-cache"
)

// A2ASystemPolicy answers the A2A requests the gateway serves itself. It holds
// no state: everything it needs arrives as parameters on every call, so one
// instance serves every route it is attached to.
type A2ASystemPolicy struct{}

var ins = &A2ASystemPolicy{}

// GetPolicy returns the policy instance.
func GetPolicy(
	_ policy.PolicyMetadata,
	_ map[string]any,
) (policy.Policy, error) {
	return ins, nil
}

// GetPolicyV2 is an alias for GetPolicy, provided for compatibility with
// plugin registries that call GetPolicyV2 on all plugins.
func GetPolicyV2(
	metadata policy.PolicyMetadata,
	params map[string]any,
) (policy.Policy, error) {
	return GetPolicy(metadata, params)
}

// Mode declares participation in both request phases.
//
// Neither response phase has anything to do: every response this policy produces
// it generates itself, so there is no upstream response left to inspect.
//
// The request body is needed by exactly one job — a managed protected card on
// the JSON-RPC binding, which must echo the caller's request id, and that id
// exists nowhere but the envelope. A ProcessingMode belongs to the policy rather
// than to the instance, so the body phase is declared for every job; it buys
// nothing new on either route this policy is attached to:
//
//   - A JSON-RPC route already buffers before any policy runs. Its resolver
//     reads the method out of the body to select this very chain, so the body is
//     in hand by the time anything here executes.
//   - The HTTP+JSON GetExtendedAgentCard binding is a bodyless GET. The kernel
//     runs body policies inline during the header phase for those, rather than
//     waiting for a callback Envoy will never send.
//   - The public card route terminates in the header phase, so its body callback
//     is never reached at all.
//
// So no route gains pre-authentication buffering from this.
//
// Serving at the body phase is also what makes the chain order mean what it
// says. The kernel runs every header policy before any body policy, so a
// responder that answered from the header phase would short-circuit before an
// operation-level *body*-phase policy had run — even though the controller
// placed that policy ahead of it in the chain.
func (a *A2ASystemPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequestHeaders dispatches on the parameter block this instance carries.
//
// Exactly one block is expected. An instance carrying none is a controller
// defect — a chain built without the thing the instance exists for — and fails
// closed with a 500 rather than forwarding. That is what keeps a block that went
// missing from becoming a bypass: an instance whose block vanished refuses the
// request instead of waving it through.
func (a *A2ASystemPolicy) OnRequestHeaders(
	_ context.Context,
	reqCtx *policy.RequestHeaderContext,
	params map[string]any,
) policy.RequestHeaderAction {
	switch {
	case hasParamBlock(params, ParamAgentCard):
		return a.servePublicCard(reqCtx, params)
	case hasParamBlock(params, ParamProtectedAgentCard):
		// Handled at the request-body phase instead. Deciding here would run
		// ahead of any body-phase policy the author attached, and would leave the
		// JSON-RPC binding with no request id to echo.
		return policy.UpstreamRequestHeaderModifications{}
	default:
		slog.Error("A2A system policy: instance carries no recognised parameter block; refusing to serve",
			"api_id", reqCtx.APIId, "path", reqCtx.Path)
		return unavailableResponse()
	}
}

// servePublicCard answers a managed public Agent Card request.
//
// It always terminates the chain — with the card, with a 304, or with a
// failure. It never falls through to the upstream: this block is attached only
// to a *managed* card route, where the gateway owns the document, and forwarding
// instead would quietly serve the upstream's own unvalidated card in its place.
func (a *A2ASystemPolicy) servePublicCard(
	reqCtx *policy.RequestHeaderContext,
	params map[string]any,
) policy.RequestHeaderAction {
	card := objectParam(params, ParamAgentCard)

	content, ok := stringParam(card, ParamContent)
	if !ok || content == "" {
		// Reachable only if the chain was built without the card the policy
		// exists to serve — a controller defect, not a client error. Fail closed
		// rather than forwarding: the response body says nothing about why.
		slog.Error("A2A system policy: no Agent Card content configured; refusing to serve",
			"api_id", reqCtx.APIId, "path", reqCtx.Path,
			"param", ParamAgentCard+"."+ParamContent)
		return unavailableResponse()
	}

	etag, _ := stringParam(card, ParamETag)

	if etag != "" && ifNoneMatchSatisfied(reqCtx.Headers.Get("if-none-match"), etag) {
		// 304 carries no body. It repeats the validator and the caching
		// directives so a client refreshing its stored copy sees the same
		// metadata it would have got with the full response.
		return policy.ImmediateResponse{
			StatusCode: 304,
			Headers: map[string]string{
				"etag":          etag,
				"cache-control": cacheControlValue,
			},
		}
	}

	headers := map[string]string{
		"content-type":  contentTypeJSON,
		"cache-control": cacheControlValue,
	}
	if etag != "" {
		headers["etag"] = etag
	}
	return policy.ImmediateResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       []byte(content),
	}
}

// OnRequestBody answers the protected (extended) Agent Card operation.
//
// This instance is the last thing in the chain, so every policy the Agent author
// attached — at the Agent-wide scope and at the operation's own — has already
// run and let the request through. Only then is the authentication check made,
// and only then, in managed mode, are any card bytes produced.
//
// The check is the whole of the protection, and it is deliberately indifferent
// to where authentication came from: the author chooses which policies run and
// in what order, and this asks only whether one of them authenticated the
// caller. An Agent that configured a protected card and attached no
// authentication anywhere therefore answers 401 rather than publishing its
// extended card.
//
// A public-card instance never gets here: it terminated the chain in the header
// phase. An instance carrying some other block continues, so a future
// gateway-answered concern is unaffected by this one.
func (a *A2ASystemPolicy) OnRequestBody(
	_ context.Context,
	reqCtx *policy.RequestContext,
	params map[string]any,
) policy.RequestAction {
	if !hasParamBlock(params, ParamProtectedAgentCard) {
		return policy.UpstreamRequestModifications{}
	}

	// Read before the check, not inside the logging call: SharedContext is
	// embedded by pointer, so APIId promotes through it and a nil one would
	// panic in the very branch that exists to handle a missing context.
	apiID := ""
	if reqCtx.SharedContext != nil {
		apiID = reqCtx.APIId
	}

	if reqCtx.SharedContext == nil || reqCtx.AuthContext == nil || !reqCtx.AuthContext.Authenticated {
		// Logged, not answered: the response is the same whatever the reason, so
		// the reason stays here and the caller is told only that it was not
		// authorized. An earlier policy that already rejected the request
		// short-circuited before this ran, so its own response stands.
		slog.Debug("A2A system policy: protected Agent Card requested without an authenticated context",
			"api_id", apiID, "path", reqCtx.Path)
		return unauthorizedResponse()
	}

	card, wellFormed := objectParamStrict(params, ParamProtectedAgentCard)
	if !wellFormed {
		// The block is present but is not an object, so whether this Agent's card
		// is managed or passthrough cannot be read off it. That distinction is
		// exactly what content's presence encodes, so guessing "passthrough" would
		// turn a managed card into a proxied one — serving the upstream's own
		// unvalidated extended card under a configuration that says the gateway
		// owns it. Fail closed instead.
		slog.Error("A2A system policy: protected Agent Card parameter block is not an object; refusing to serve",
			"api_id", apiID, "path", reqCtx.Path, "param", ParamProtectedAgentCard)
		return unavailableResponse()
	}

	content, hasContent := stringParam(card, ParamContent)
	if !hasContent {
		if _, present := card[ParamContent]; present {
			// Present but not a string. Same reasoning as above: this is a managed
			// card whose bytes did not survive, not a passthrough one.
			slog.Error("A2A system policy: protected Agent Card content is not a string; refusing to serve",
				"api_id", apiID, "path", reqCtx.Path,
				"param", ParamProtectedAgentCard+"."+ParamContent)
			return unavailableResponse()
		}
		// Passthrough: the gateway owns no document here, so the request goes on
		// to the upstream now that it is authenticated, and the upstream's own
		// extended card is proxied unparsed.
		return policy.UpstreamRequestModifications{}
	}
	if content == "" {
		// An empty string is a managed card that serialized to nothing, which no
		// controller writes. Passthrough omits the field rather than emptying it.
		slog.Error("A2A system policy: protected Agent Card content is empty; refusing to serve",
			"api_id", apiID, "path", reqCtx.Path,
			"param", ParamProtectedAgentCard+"."+ParamContent)
		return unavailableResponse()
	}

	// Which binding this is was decided by the resolver that selected this
	// chain, and is read back from it rather than re-derived here. Guessing from
	// the request — its method, its path, whether it happens to carry a body —
	// would be a second, weaker classification that can disagree with the one
	// that actually chose the policies.
	switch transport := reqCtx.ResolutionAttributes.Get(attrA2ATransport); transport {
	case transportHTTPJSON:
		// The binding returns the Agent Card as the response body itself.
		return policy.ImmediateResponse{
			StatusCode: 200,
			Headers: map[string]string{
				"content-type":  contentTypeJSON,
				"cache-control": cacheControlNoStore,
			},
			Body: []byte(content),
		}
	case transportJSONRPC:
		return jsonRPCCardResponse(reqCtx, content)
	default:
		// A managed card with nowhere correct to put it. Fail closed rather than
		// forwarding: forwarding would serve the upstream's own unvalidated
		// extended card under a configuration that says the gateway owns it,
		// which is a silent substitution with nothing to report it.
		slog.Error("A2A system policy: protected Agent Card request carries no recognised A2A transport",
			"api_id", reqCtx.APIId, "path", reqCtx.Path,
			"attribute", attrA2ATransport, "value", transport)
		return unavailableResponse()
	}
}

// jsonRPCCardResponse wraps the card in a JSON-RPC 2.0 result envelope, echoing
// the caller's request id.
//
// Only the id is read out of the request. The method was matched by the resolver
// when it selected this chain, so re-reading it here would be a second
// classification that could disagree with the one that chose these policies.
func jsonRPCCardResponse(reqCtx *policy.RequestContext, content string) policy.RequestAction {
	var body []byte
	if reqCtx.Body != nil {
		body = reqCtx.Body.Content
	}

	var envelope struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		// The resolver parsed this same envelope to get here, so a failure now
		// means the bytes changed underneath — not a client error anyone can act
		// on, but not something to answer with a card either.
		slog.Error("A2A system policy: protected Agent Card request body is not a JSON-RPC envelope",
			"api_id", reqCtx.APIId, "path", reqCtx.Path, "error", err)
		return badRequestResponse()
	}

	switch classifyJSONValue(envelope.ID) {
	case jsonAbsent:
		// A JSON-RPC request without an id is a notification: the caller has
		// declared it wants no reply, so returning the card would be a response
		// it has no id to correlate. 204 is the HTTP-level way to say the same.
		return policy.ImmediateResponse{
			StatusCode: 204,
			Headers:    map[string]string{"cache-control": cacheControlNoStore},
		}
	case jsonString, jsonNumber, jsonNull:
		// Echoed as the JSON value it arrived as. A number id must come back a
		// number and a string id a string — a client matches responses to
		// requests on this value, and coercing 7 to "7" breaks that silently.
	default:
		// JSON-RPC 2.0 restricts an id to a string, a number, or null. Anything
		// else is malformed, and answering it would mean inventing an id.
		slog.Debug("A2A system policy: protected Agent Card request carries an unusable JSON-RPC id",
			"api_id", reqCtx.APIId, "path", reqCtx.Path)
		return badRequestResponse()
	}

	// Assembled rather than marshalled from a struct: content is the exact card
	// bytes the controller validated and, once signing lands, signed. Decoding
	// and re-encoding them here would change them.
	var out bytes.Buffer
	out.Grow(len(content) + len(envelope.ID) + 40)
	out.WriteString(`{"jsonrpc":"` + jsonRPCVersion + `","id":`)
	out.Write(envelope.ID)
	out.WriteString(`,"result":`)
	out.WriteString(content)
	out.WriteString(`}`)

	return policy.ImmediateResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"content-type":  contentTypeJSON,
			"cache-control": cacheControlNoStore,
		},
		Body: out.Bytes(),
	}
}

// jsonValueKind classifies a raw JSON value by its first structural byte, which
// is all JSON needs to tell its types apart.
type jsonValueKind int

const (
	jsonAbsent jsonValueKind = iota
	jsonString
	jsonNumber
	jsonNull
	jsonOther
)

func classifyJSONValue(raw json.RawMessage) jsonValueKind {
	trimmed := bytes.TrimLeft(raw, " \t\n\r")
	if len(trimmed) == 0 {
		return jsonAbsent
	}
	switch first := trimmed[0]; {
	case first == '"':
		return jsonString
	case first == '-' || (first >= '0' && first <= '9'):
		return jsonNumber
	case first == 'n':
		return jsonNull
	default:
		return jsonOther
	}
}

// unavailableResponse is the sterile answer to a chain the controller built
// wrong. It names no parameter, no policy and no reason: everything that would
// help diagnose it has already gone to the log, where the readership is the
// operator rather than the caller.
func unavailableResponse() policy.ImmediateResponse {
	return policy.ImmediateResponse{
		StatusCode: 500,
		Headers:    map[string]string{"content-type": contentTypeJSON},
		Body:       []byte(`{"error":"unavailable","message":"The Agent Card could not be served."}`),
	}
}

// unauthorizedResponse is the single answer to every authentication failure on a
// protected card — missing credential, invalid credential, or an auth policy
// that never ran. One response for all of them, so the status and body cannot be
// used to tell those cases apart.
//
// It carries no card bytes and no hint of any, and it is never cached.
//
// On the JSON-RPC binding this is an HTTP-level rejection rather than a
// JSON-RPC error object, so a JSON-RPC client receives a body its transport does
// not describe. That is how the engine renders every rejected request today, not
// a decision taken here; it changes for all of them together when
// protocol-shaped error rendering lands.
func unauthorizedResponse() policy.ImmediateResponse {
	return policy.ImmediateResponse{
		StatusCode: 401,
		Headers: map[string]string{
			"content-type":  contentTypeJSON,
			"cache-control": cacheControlNoStore,
		},
		Body: []byte(`{"error":"unauthorized","message":"Invalid or expired credentials."}`),
	}
}

// badRequestResponse answers a request this policy cannot form a reply to.
func badRequestResponse() policy.ImmediateResponse {
	return policy.ImmediateResponse{
		StatusCode: 400,
		Headers: map[string]string{
			"content-type":  contentTypeJSON,
			"cache-control": cacheControlNoStore,
		},
		Body: []byte(`{"error":"invalid_request","message":"The request could not be processed."}`),
	}
}

// hasParamBlock reports whether this instance carries the named parameter block
// at all.
//
// Presence, not shape: a block is what tells one instance of this policy from
// another, and an instance carrying a *malformed* block must reach that block's
// own handler and fail closed there — not fall through to the next case and do
// some other job.
func hasParamBlock(params map[string]any, name string) bool {
	if params == nil {
		return false
	}
	_, present := params[name]
	return present
}

// objectParamStrict reads one nested parameter block and reports whether it was
// an object.
//
// It exists for the block where "absent field" is itself meaningful: a protected
// Agent Card with no content is a passthrough card, so a malformed block read as
// an empty one would be indistinguishable from a legitimate configuration and
// would change what the gateway serves. Where absence carries no meaning,
// objectParam's nil-reads-as-empty is the simpler thing to use.
func objectParamStrict(params map[string]any, name string) (map[string]any, bool) {
	if params == nil {
		return nil, false
	}
	block, ok := params[name].(map[string]any)
	return block, ok
}

// objectParam reads one nested parameter block, returning nil when it is absent
// or is not an object. A nil block reads as an empty one, so every field lookup
// below it reports "absent" rather than panicking.
func objectParam(params map[string]any, name string) map[string]any {
	if params == nil {
		return nil
	}
	block, _ := params[name].(map[string]any)
	return block
}

// stringParam reads one string parameter.
//
// Parameters arrive as decoded JSON, so a value the controller wrote as a
// string is a string here. A non-string is treated as absent rather than
// coerced: silently stringifying whatever arrived would serve a card body of
// "map[...]" with a 200.
func stringParam(params map[string]any, name string) (string, bool) {
	if params == nil {
		return "", false
	}
	value, ok := params[name].(string)
	return value, ok
}

// ifNoneMatchSatisfied reports whether an If-None-Match header selects the
// representation identified by etag, in which case the response is a 304.
//
// Comparison is weak, per RFC 9110 section 8.8.3.2, which is what If-None-Match
// specifies regardless of whether the tags involved are strong: the "W/" prefix
// is stripped from both sides before the opaque tags are compared. "*" matches
// whenever a representation exists, which on this route it always does — the
// caller has already established that there is a card to serve.
//
// The header may appear more than once and each occurrence may carry a
// comma-separated list, so every value of every occurrence is considered.
func ifNoneMatchSatisfied(headerValues []string, etag string) bool {
	target := strings.TrimPrefix(etag, "W/")
	for _, headerValue := range headerValues {
		for _, candidate := range strings.Split(headerValue, ",") {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			if candidate == "*" {
				return true
			}
			if strings.TrimPrefix(candidate, "W/") == target {
				return true
			}
		}
	}
	return false
}

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
// Today it does one such job — serving a managed public Agent Card. The package
// and the policy are named for the protocol rather than for that job so a second
// gateway-answered A2A concern joins this policy instead of adding another
// entry to the chain, another module, and another build-lock line. Each job
// takes its own parameter block; nothing here is shared between them beyond the
// name.
//
// # Agent Card serving
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
package a2a

import (
	"context"
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Parameter names.
//
// These are written by the gateway controller's Agent transformer
// (pkg/transform/agent.go, agentCardPolicyInstance) and read here. The two live
// in separate Go modules and cannot share a constant, so the names are spelled
// once on each side and the coupling is asserted by a test on each side.
const (
	// ParamAgentCard is the block configuring Agent Card serving. It is nested
	// rather than flattened because the policy is named for the protocol, not
	// for this one job: a second gateway-answered A2A concern gets a sibling
	// block, and neither has to negotiate for a top-level parameter name.
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
	ParamETag = "etag"
)

const (
	// contentTypeJSON is the media type A2A Agent Cards are served as.
	contentTypeJSON = "application/json"

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

// Mode declares participation in the request-header phase only.
//
// Nothing else has anything to do: the response is generated here, so there is
// no upstream response to inspect, and a card request carries no body worth
// buffering. Declaring any body phase would make the kernel buffer for no
// reason on a route whose whole point is to answer immediately.
func (a *A2ASystemPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequestHeaders answers the card request.
//
// It always terminates the chain — with the card, with a 304, or with a
// failure. It never falls through to the upstream: this policy is attached only
// to a *managed* card route, where the gateway owns the document, and forwarding
// instead would quietly serve the upstream's own unvalidated card in its place.
//
// A second gateway-answered A2A job branches here on its own parameter block
// before the card block is looked at, rather than sharing this one.
func (a *A2ASystemPolicy) OnRequestHeaders(
	_ context.Context,
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
		return policy.ImmediateResponse{
			StatusCode: 500,
			Headers:    map[string]string{"content-type": contentTypeJSON},
			Body: []byte(
				`{"error":"unavailable","message":"The Agent Card could not be served."}`),
		}
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

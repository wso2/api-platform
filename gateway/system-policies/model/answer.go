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
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

const (
	contentTypeJSON = "application/json"

	// cacheControlNoStore is what both gateway-answered responses carry. The
	// catalog is authenticated — it answers behind the endpoint's own policies — so
	// a shared cache holding it would be holding one caller's authorised view for
	// another's request; and an error is never worth caching.
	cacheControlNoStore = "no-store"
)

// OpenAI error types, as the OpenAI API spells them. Clients branch on the HTTP
// status first and read these second, so they are chosen to agree with it.
const (
	openAIInvalidRequest = "invalid_request_error"
	openAIServerError    = "server_error"
)

// invalidJSONResponse is the answer to a body that is not a JSON object.
func invalidJSONResponse(format string) policy.ImmediateResponse {
	return errorResponse(format, http.StatusBadRequest,
		"We could not parse the JSON body of your request. The body must be a JSON object.",
		"", DispatchErrorInvalidJSON)
}

// serverErrorResponse is the fail-closed answer: the gateway could not work out
// what to do with the request, and says nothing more about why. The reason is in
// the log.
func serverErrorResponse(format string) policy.ImmediateResponse {
	return errorResponse(format, http.StatusInternalServerError, "The server could not process the request.", "", "")
}

// routingConflictResponse is the answer to a request whose provider changed between
// the header and the body phase. A 500: the request was well formed, and it is the
// chain's configuration that cannot route it.
func routingConflictResponse(format string) policy.ImmediateResponse {
	return errorResponse(format, http.StatusInternalServerError, "The request could not be routed.", "", "routing_conflict")
}

// dispatchAnswer is the answer to each llm.dispatch.error value.
//
// The not-found message names no model on purpose. The value is the caller's own
// input, and echoing it would reflect caller-controlled text back in a gateway
// response for no benefit — the caller already knows what it sent. It also says
// "or you do not have access to it", as OpenAI does, so the answer is the same
// whether a model is absent or merely not deployed here.
func dispatchAnswer(format, kind string) (policy.ImmediateResponse, bool) {
	switch kind {
	case DispatchErrorInvalidJSON:
		return invalidJSONResponse(format), true
	case DispatchErrorMissingModel:
		return errorResponse(format, http.StatusBadRequest,
			"You must provide a model parameter.", "model", DispatchErrorMissingModel), true
	case DispatchErrorInvalidModel:
		return errorResponse(format, http.StatusBadRequest,
			"The model must be given once, as a string.", "model", DispatchErrorInvalidModel), true
	case DispatchErrorModelNotFound:
		return errorResponse(format, http.StatusNotFound,
			"The requested model does not exist or you do not have access to it.",
			"model", DispatchErrorModelNotFound), true
	}
	return policy.ImmediateResponse{}, false
}

// ─── dispatchError ───────────────────────────────────────────────────────────

// dispatchErrorPolicy answers the reserved dispatch chain, in the endpoint's
// format. The answer is chosen per request from the resolver's attribute.
type dispatchErrorPolicy struct {
	format string
}

// Mode declares the request headers only.
//
// The dispatch chain is bound before any policy runs — at the request-body callback
// when the model is in the body, at the header phase otherwise — and the kernel
// runs the chain's header policies before its body policies. So by the time this
// answers, every header-phase policy ahead of it, the endpoint's authentication
// included, has already run and let the request through. Answering from the
// header phase means a body-phase policy the operator attached to the endpoint
// does not run on a request that is being refused anyway.
func (*dispatchErrorPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequestHeaders answers with the error the resolver classified.
//
// An absent or unrecognised classification fails closed with a 500 rather than
// forwarding: the dispatch chain has no upstream a request could legitimately
// reach, so anything here that is not one of the four refusals is a resolver the
// policy does not understand.
func (p *dispatchErrorPolicy) OnRequestHeaders(
	_ context.Context,
	reqCtx *policy.RequestHeaderContext,
	_ map[string]any,
) policy.RequestHeaderAction {
	if reqCtx == nil || reqCtx.SharedContext == nil {
		slog.Error("Model system policy: dispatch error reached without a request context")
		return serverErrorResponse(p.format)
	}

	kind := reqCtx.ResolutionAttributes.Get(AttrDispatchError)
	answer, known := dispatchAnswer(p.format, kind)
	if !known {
		slog.Error("Model system policy: dispatch chain reached without a recognised dispatch error",
			"api_id", reqCtx.APIId, "path", reqCtx.Path, "dispatch_error", kind)
		return serverErrorResponse(p.format)
	}
	return answer
}

// ─── catalog ─────────────────────────────────────────────────────────────────

// catalogEntry is one Model the endpoint serves.
type catalogEntry struct {
	handle      string
	name        string
	displayName string
	created     int64
}

// catalogPolicy answers the model catalog routes in the endpoint's format, listing
// the Models the caller may reach.
type catalogPolicy struct {
	format  string
	entries []catalogEntry
}

// Mode declares the request headers only: the answer is known before the request
// arrives, and a GET has no body to wait for.
func (*catalogPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequestHeaders serves the catalog. It always terminates the chain: the
// catalog route has no upstream, and forwarding would send the request to the
// route's fail-closed default cluster.
//
// The list is filtered by the caller's API key, when the endpoint authenticated it
// with one: a key scoped to Models sees only those Models. A caller the endpoint
// authenticated some other way sees every Model the endpoint serves.
func (p *catalogPolicy) OnRequestHeaders(
	_ context.Context,
	reqCtx *policy.RequestHeaderContext,
	_ map[string]any,
) policy.RequestHeaderAction {
	var shared *policy.SharedContext
	if reqCtx != nil {
		shared = reqCtx.SharedContext
	}
	visible := filterCatalog(p.entries, keyScopeOf(shared))
	body, err := renderCatalog(p.format, visible)
	if err != nil {
		slog.Error("Model system policy: catalog could not be rendered", "error", err)
		return serverErrorResponse(p.format)
	}
	return policy.ImmediateResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"content-type":  contentTypeJSON,
			"cache-control": cacheControlNoStore,
		},
		Body: body,
	}
}

// keyScope is what an API key lets its caller see in the catalog.
type keyScope struct {
	// all is true when the caller is not limited: an endpoint key, or a caller not
	// authenticated by an API key at all.
	all bool
	// handles are the Models a scoped key reaches.
	handles map[string]bool
}

// keyScopeOf reads the caller's API key scope from the authentication context.
//
// api-key-auth records the key's Operations ("*", or a JSON array of the operations
// it reaches) when it admits a caller. A caller it admitted with no recorded scope
// sees nothing: the scope is unknown, and failing closed lists nothing rather than
// every Model.
func keyScopeOf(shared *policy.SharedContext) keyScope {
	var auth *policy.AuthContext
	if shared != nil {
		auth = shared.AuthContext
	}
	for ac := auth; ac != nil; ac = ac.Previous {
		if !ac.Authenticated || ac.AuthType != APIKeyAuthType {
			continue
		}
		operations, recorded := ac.Properties[AuthPropertyAPIKeyOperations]
		if !recorded {
			return keyScope{}
		}
		trimmed := strings.TrimSpace(operations)
		if trimmed == "" || trimmed == allOperations {
			return keyScope{all: true}
		}
		var entries []string
		if err := json.Unmarshal([]byte(trimmed), &entries); err != nil {
			return keyScope{}
		}
		scope := keyScope{handles: make(map[string]bool, len(entries))}
		for _, entry := range entries {
			if entry == allOperations {
				return keyScope{all: true}
			}
			scope.handles[entry] = true
		}
		return scope
	}
	return keyScope{all: true}
}

// filterCatalog returns the entries the scope may see.
func filterCatalog(entries []catalogEntry, scope keyScope) []catalogEntry {
	if scope.all {
		return entries
	}
	out := make([]catalogEntry, 0, len(entries))
	for _, e := range entries {
		if scope.handles[e.handle] {
			out = append(out, e)
		}
	}
	return out
}

/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
 *
 */

package handler

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/api-platform/httpkit/httputil"
)

// agentProxyMaxBodyBytes bounds an Agent proxy request body. A managed Agent
// Card is capped at 1 MiB by the contract and an Agent proxy may carry both a
// public and a protected one, so the ceiling leaves room for two of them plus
// the rest of the document — and nothing beyond that is read into memory.
const agentProxyMaxBodyBytes = 4 << 20 // 4 MiB

// agentCardFetchMaxBodyBytes bounds an Agent Card fetch request body. The body
// carries at most a URL and a credential, so the ceiling is small on purpose —
// it is not a card, and nothing about this operation justifies reading more.
const agentCardFetchMaxBodyBytes = 16 << 10 // 16 KiB

// AgentProxyHandler serves the Agent proxy CRUD operations.
//
// The route patterns below must match the paths in resources/openapi.yaml
// exactly. ScopeEnforcer is deny-by-default and keys on the registered pattern,
// so a path shape that drifts from the spec does not fall back to unprotected —
// it 403s. ValidateScopeRegistryRoutes checks that agreement at startup and
// scope_route_coverage_test.go checks it at build time.
type AgentProxyHandler struct {
	service  *service.AgentProxyService
	identity *service.IdentityService
	slogger  *slog.Logger
}

// NewAgentProxyHandler creates a new AgentProxyHandler instance.
func NewAgentProxyHandler(service *service.AgentProxyService, identity *service.IdentityService, slogger *slog.Logger) *AgentProxyHandler {
	return &AgentProxyHandler{
		service:  service,
		identity: identity,
		slogger:  slogger,
	}
}

// RegisterRoutes wires the Agent proxy collection and item routes.
func (h *AgentProxyHandler) RegisterRoutes(mux router.Router) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/agent-proxies", middleware.MapErrors(h.slogger, h.CreateAgentProxy))
	mux.HandleFunc("GET "+constants.APIBasePath+"/agent-proxies", middleware.MapErrors(h.slogger, h.ListAgentProxies))
	mux.HandleFunc("POST "+constants.APIBasePath+"/agent-proxies/fetch-agent-card", middleware.MapErrors(h.slogger, h.FetchAgentCard))
	mux.HandleFunc("GET "+constants.APIBasePath+"/agent-proxies/{agentProxyId}", middleware.MapErrors(h.slogger, h.GetAgentProxy))
	mux.HandleFunc("PUT "+constants.APIBasePath+"/agent-proxies/{agentProxyId}", middleware.MapErrors(h.slogger, h.UpdateAgentProxy))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/agent-proxies/{agentProxyId}", middleware.MapErrors(h.slogger, h.DeleteAgentProxy))
}

// CreateAgentProxy handles POST /api/v0.9/agent-proxies
func (h *AgentProxyHandler) CreateAgentProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	req, err := decodeAgentProxyBody(w, r)
	if err != nil {
		return err
	}

	createdBy, err := resolveActorErr(r, h.identity, "create Agent proxy")
	if err != nil {
		return err
	}

	resp, err := h.service.Create(orgID, createdBy, req)
	if err != nil {
		return h.mapServiceError(err)
	}

	setLocation(w, "agent-proxies", strOrEmpty(resp.Id))
	httputil.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

// ListAgentProxies handles GET /api/v0.9/agent-proxies
func (h *AgentProxyHandler) ListAgentProxies(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	limit, offset := parsePagination(r)

	// Presence, not emptiness: "?protocol=" is a supplied filter with an invalid
	// value, which the service rejects, while an absent parameter lists every
	// protocol. Query().Get collapses both to "", so the raw map is read instead.
	var protocol *string
	if values, present := r.URL.Query()["protocol"]; present && len(values) > 0 {
		protocol = &values[0]
	}

	resp, err := h.service.List(orgID, protocol, limit, offset)
	if err != nil {
		return h.mapServiceError(err)
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// GetAgentProxy handles GET /api/v0.9/agent-proxies/{agentProxyId}
func (h *AgentProxyHandler) GetAgentProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	resp, err := h.service.Get(orgID, r.PathValue("agentProxyId"))
	if err != nil {
		return h.mapServiceError(err)
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// UpdateAgentProxy handles PUT /api/v0.9/agent-proxies/{agentProxyId}
func (h *AgentProxyHandler) UpdateAgentProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("agentProxyId")

	req, err := decodeAgentProxyBody(w, r)
	if err != nil {
		return err
	}

	// An omitted body id keeps the path handle and is never regenerated; a body
	// id that disagrees with the path is a 400 rather than a silent rename.
	if err := utils.ValidateHandleImmutable(id, req.Id); err != nil {
		return err
	}

	updatedBy, err := resolveActorErr(r, h.identity, "update Agent proxy")
	if err != nil {
		return err
	}

	resp, err := h.service.Update(orgID, id, updatedBy, req)
	if err != nil {
		return h.mapServiceError(err)
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// DeleteAgentProxy handles DELETE /api/v0.9/agent-proxies/{agentProxyId}
func (h *AgentProxyHandler) DeleteAgentProxy(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	id := r.PathValue("agentProxyId")

	deletedBy, err := resolveActorErr(r, h.identity, "delete Agent proxy")
	if err != nil {
		return err
	}

	if err := h.service.Delete(orgID, id, deletedBy); err != nil {
		return h.mapServiceError(err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// FetchAgentCard handles POST /api/v0.9/agent-proxies/fetch-agent-card
//
// The route is registered ahead of no item route of the same shape — there is no
// POST on /agent-proxies/{agentProxyId} — but "fetch-agent-card" is reserved as
// an Agent proxy handle all the same, so a GET/PUT/DELETE on this path can never
// be shadowed by a resource that named itself after the discovery route.
func (h *AgentProxyHandler) FetchAgentCard(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	req, err := decodeAgentCardFetchBody(w, r)
	if err != nil {
		return err
	}

	result, err := h.service.FetchAgentCard(orgID, req, requestsNoCache(r))
	if err != nil {
		// Age is written before the error is handed to the mapper: a cached 503
		// carries one too, which is what lets a client say how long the upstream
		// has been unreachable rather than only that it is. No Cache-Control
		// accompanies it — a failure is held for the negative TTL, not the
		// positive one the success path reports, and stating the wrong number
		// would be worse than stating none.
		writeAgentCardAge(w, result)
		return h.mapServiceError(err)
	}

	writeAgentCardAge(w, result)
	writeAgentCardCacheControl(w, result)
	// The card is written through verbatim rather than re-encoded. It is
	// free-form by contract and is returned exactly as the upstream supplied it,
	// because re-serializing it would change the bytes a future Agent Card
	// signing implementation signs.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Card)
	return nil
}

// decodeAgentCardFetchBody reads a bounded request body and decodes it into one
// of the two disjoint fetch forms.
//
// The body is read in full before decoding for the same reason the Agent proxy
// body is: which form this is has to be decided from the keys present, which the
// generated union type cannot see — a null url decodes to the same nil pointer
// an omitted one does, and the contract rejects the first and accepts the second.
func decodeAgentCardFetchBody(w http.ResponseWriter, r *http.Request) (*dto.AgentCardFetchRequest, error) {
	if err := requireJSONContentType(r); err != nil {
		return nil, err
	}
	r.Body = http.MaxBytesReader(w, r.Body, agentCardFetchMaxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return nil, apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
		}
		return nil, apperror.ValidationFailed.Wrap(err, "The request body could not be read.")
	}
	if len(body) == 0 {
		return nil, apperror.ValidationFailed.New("A request body is required.")
	}

	req, err := dto.DecodeAgentCardFetchRequest(body)
	if err != nil {
		// DecodeAgentCardFetchRequest phrases every failure in terms of the
		// caller's own payload, so its message is the client-facing one.
		return nil, apperror.ValidationFailed.Wrap(err, err.Error())
	}
	return req, nil
}

// requestsNoCache reports whether the caller asked for a live upstream fetch.
//
// This is the authoring UI's explicit "refresh" and the only cache bypass there
// is. Per RFC 9110 it governs freshness alone: it never changes what is stored,
// and it is not a way to reach an Agent proxy the caller could not otherwise
// read — the organization check runs either way.
func requestsNoCache(r *http.Request) bool {
	for _, value := range r.Header.Values("Cache-Control") {
		for _, directive := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(directive), "no-cache") {
				return true
			}
		}
	}
	return false
}

// writeAgentCardAge reports how long ago the upstream was actually contacted.
//
// Freshness travels in headers and never in the body: the card is free-form, so
// an added `cached` or `fetchedAt` field would be indistinguishable from a real
// card field — and re-serializing the document to insert one would change the
// bytes a future Agent Card signing implementation signs.
//
// Only the stored-handle form participates in the cache, so the direct-URL form
// gets no header at all rather than an Age that would misdescribe it.
func writeAgentCardAge(w http.ResponseWriter, result *service.AgentCardFetchResult) {
	if result == nil || !result.Cacheable {
		return
	}
	w.Header().Set("Age", strconv.FormatInt(int64(result.Age.Seconds()), 10))
}

// writeAgentCardCacheControl reports how long a *fetched card* stays fresh. It
// belongs on the success path only: a failure is held for the negative TTL, and
// reporting the positive one beside a 503 would tell the client the wrong
// number.
func writeAgentCardCacheControl(w http.ResponseWriter, result *service.AgentCardFetchResult) {
	if result == nil || !result.Cacheable {
		return
	}
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", int64(result.MaxAge.Seconds())))
}

// decodeAgentProxyBody reads a bounded request body and decodes it into the
// typed Agent proxy variant its protocol discriminator selects.
//
// The body is read in full before decoding because the discriminator has to be
// inspected before the variant is known — hence the explicit ceiling, rather
// than streaming straight into a decoder.
func decodeAgentProxyBody(w http.ResponseWriter, r *http.Request) (*api.A2AAgentProxy, error) {
	if err := requireJSONContentType(r); err != nil {
		return nil, err
	}
	r.Body = http.MaxBytesReader(w, r.Body, agentProxyMaxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return nil, apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
		}
		return nil, apperror.ValidationFailed.Wrap(err, "The request body could not be read.")
	}
	if len(body) == 0 {
		return nil, apperror.ValidationFailed.New("A request body is required.")
	}

	req, err := dto.DecodeAgentProxyRequest(body)
	if err != nil {
		// DecodeAgentProxyRequest phrases every failure in terms of the caller's
		// own payload, so its message is the client-facing one.
		return nil, apperror.ValidationFailed.Wrap(err, err.Error())
	}
	return req, nil
}

// requireJSONContentType enforces the 415 half of the body-bearing operations'
// contract: a request that declares a media type this operation does not accept
// is refused before its body is read, rather than being parsed as JSON anyway
// because it happens to contain some.
//
// An absent Content-Type is permitted. RFC 9110 lets a recipient examine the
// content when the sender declares nothing, and that is what the JSON decode
// below does — a non-JSON body then fails as a 400, which is the honest answer.
// 415 is for a declared type that is wrong, which is what the spec's
// UnsupportedMediaType response describes.
func requireJSONContentType(r *http.Request) error {
	declared := r.Header.Get("Content-Type")
	if declared == "" {
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(declared)
	if err != nil {
		return apperror.UnsupportedMediaType.New().
			WithLogMessage("unparsable Content-Type on an Agent proxy request")
	}
	// Parameters such as "; charset=utf-8" are stripped by ParseMediaType, and a
	// structured suffix (application/merge-patch+json and friends) still carries
	// a JSON body.
	if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") {
		return nil
	}
	return apperror.UnsupportedMediaType.New().
		WithLogMessage("unsupported Content-Type on an Agent proxy request: " + mediaType)
}

// mapServiceError hands a service-layer error to the centralized error mapper.
// See serviceError in service_error.go.
func (h *AgentProxyHandler) mapServiceError(err error) error {
	return serviceError(err, "Agent proxy operation failed")
}

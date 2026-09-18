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
	"io"
	"log/slog"
	"mime"
	"net/http"
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

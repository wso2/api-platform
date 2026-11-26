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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/wso2/api-platform/httpkit/httputil"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/gatewayrelay"
	"github.com/wso2/api-platform/platform-api/internal/gatewayrelay/egress"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// invokeRequestStructuralBytes allows for the JSON around the body: ids,
// method, path, query and headers.
const invokeRequestStructuralBytes = 32 << 10

// invokeTargets finds where a test request goes (APIService.GetAPIInvokeTarget,
// GraphQLAPIService.GetAPIInvokeTarget).
type invokeTargets interface {
	GetAPIInvokeTarget(apiHandle, gatewayHandle, orgID string) (*service.APIInvokeTarget, error)
}

// relayer sends the request (gatewayrelay.Relay).
type relayer interface {
	Do(ctx context.Context, target gatewayrelay.Target, req gatewayrelay.Request) (gatewayrelay.Relayed, error)
}

// invokeErrors are one API kind's answers to a test request that was not relayed.
type invokeErrors struct {
	gatewayNoEndpoint        apperror.Def
	gatewayUnsupportedScheme apperror.Def
	targetNotAllowed         apperror.Def
	requestInvalid           apperror.Def
	busy                     apperror.Def
	upstreamTimeout          apperror.Def
	upstreamUnreachable      apperror.Def
}

var (
	restInvokeErrors = invokeErrors{
		gatewayNoEndpoint:        apperror.RESTAPIInvokeGatewayNoEndpoint,
		gatewayUnsupportedScheme: apperror.RESTAPIInvokeGatewayUnsupportedScheme,
		targetNotAllowed:         apperror.RESTAPIInvokeTargetNotAllowed,
		requestInvalid:           apperror.RESTAPIInvokeRequestInvalid,
		busy:                     apperror.RESTAPIInvokeBusy,
		upstreamTimeout:          apperror.RESTAPIInvokeUpstreamTimeout,
		upstreamUnreachable:      apperror.RESTAPIInvokeUpstreamUnreachable,
	}
	graphqlInvokeErrors = invokeErrors{
		gatewayNoEndpoint:        apperror.GraphQLAPIInvokeGatewayNoEndpoint,
		gatewayUnsupportedScheme: apperror.GraphQLAPIInvokeGatewayUnsupportedScheme,
		targetNotAllowed:         apperror.GraphQLAPIInvokeTargetNotAllowed,
		requestInvalid:           apperror.GraphQLAPIInvokeRequestInvalid,
		busy:                     apperror.GraphQLAPIInvokeBusy,
		upstreamTimeout:          apperror.GraphQLAPIInvokeUpstreamTimeout,
		upstreamUnreachable:      apperror.GraphQLAPIInvokeUpstreamUnreachable,
	}
)

// invokeKind is what differs between a REST and a GraphQL API's test request.
// label ("REST API") names the API in log lines, and the caller needs one of scopes.
type invokeKind struct {
	apiType api.APIInvokeRequestApiType
	label   string
	scopes  []string
	targets invokeTargets
	errs    invokeErrors
}

// APIInvokeHandler serves POST /gateway-relay/invoke.
type APIInvokeHandler struct {
	rest           invokeKind
	graphql        invokeKind
	relay          relayer
	authz          config.Authorization
	maxBodyBytes   int64
	requestTimeout time.Duration
	slogger        *slog.Logger
}

// NewAPIInvokeHandler builds the handler. maxRequestBytes is the relay's limit
// on the decoded request body, and requestTimeout the limit on one test request.
func NewAPIInvokeHandler(restTargets, graphqlTargets invokeTargets, relay relayer, authz config.Authorization, maxRequestBytes int64, requestTimeout time.Duration, slogger *slog.Logger) *APIInvokeHandler {
	return &APIInvokeHandler{
		rest: invokeKind{
			apiType: api.APIInvokeRequestApiTypeRestApi,
			label:   "REST API",
			scopes:  []string{constants.ScopeRESTAPIRead, constants.ScopeRESTAPIManage},
			targets: restTargets,
			errs:    restInvokeErrors,
		},
		graphql: invokeKind{
			apiType: api.APIInvokeRequestApiTypeGraphqlApi,
			label:   "GraphQL API",
			scopes:  []string{constants.ScopeGraphQLAPIRead, constants.ScopeGraphQLAPIManage},
			targets: graphqlTargets,
			errs:    graphqlInvokeErrors,
		},
		relay: relay,
		authz: authz,
		// The body travels as a JSON string. Base64 makes a binary body a third
		// larger; escaping doubles a quote, a backslash, \n, \r or \t in a text one.
		// Only the decoded body is held to maxRequestBytes (Relay.Do), so this
		// bounds the read without refusing a body of quotes that is within it.
		maxBodyBytes:   maxRequestBytes*2 + invokeRequestStructuralBytes,
		requestTimeout: requestTimeout,
		slogger:        slogger,
	}
}

// InvokeAPI handles POST /api/v0.9/gateway-relay/invoke.
func (h *APIInvokeHandler) InvokeAPI(w http.ResponseWriter, r *http.Request) error {
	// The answer describes one caller's gateway; nothing on the way may cache it.
	w.Header().Set("Cache-Control", "no-store")

	// request_timeout runs from here, not from the gateway call: the server's write
	// deadline starts when the request arrives, so the time spent reading the body
	// and finding the target is taken from the gateway call. Neither step watches
	// the deadline (server.timeouts.read bounds the read); the gateway call and the
	// wait for a slot end at it, leaving time to write a timeout answer.
	ctx, cancel := context.WithTimeout(r.Context(), h.requestTimeout)
	defer cancel()

	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	kind, apiID, gatewayID, req, err := h.decodeRequest(w, r)
	if err != nil {
		return err
	}
	// The route admits either kind's scopes; this keeps each caller to its own kind.
	if !middleware.HoldsAnyScope(r, h.authz, kind.scopes...) {
		return apperror.Forbidden.New().
			WithLogMessage(fmt.Sprintf("test request for %s %s without one of %v", kind.label, apiID, kind.scopes))
	}

	invokeTarget, err := kind.targets.GetAPIInvokeTarget(apiID, gatewayID, orgID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to resolve gateway %s for %s %s in org %s", gatewayID, kind.label, apiID, orgID))
	}
	target, err := gatewayrelay.BuildTarget(gatewayID, invokeTarget.Endpoints, invokeTarget.Context, invokeTarget.Vhost)
	if errors.Is(err, gatewayrelay.ErrUnsupportedScheme) {
		return kind.errs.gatewayUnsupportedScheme.New()
	}
	if err != nil {
		return kind.errs.gatewayNoEndpoint.New()
	}

	relayed, err := h.relay.Do(ctx, target, req)
	if err != nil {
		return relayError(err, kind, apiID, gatewayID)
	}

	// The relay accepted the method, so this is its normalized form, not raw input.
	method, _ := gatewayrelay.NormalizeMethod(req.Method)
	h.slogger.Info("Relayed a test request to a gateway",
		"apiType", kind.apiType,
		"apiId", apiID,
		"gatewayId", gatewayID,
		"method", method,
		"upstreamStatus", relayed.Status,
		"durationMs", relayed.DurationMs,
		"truncated", relayed.Truncated)

	httputil.WriteJSON(w, http.StatusOK, toInvokeResponse(relayed))
	return nil
}

// decodeRequest reads the bounded JSON request: the API and gateway to invoke,
// and the request to send. Content-Type must be JSON, which a page on another
// site cannot send without a CORS preflight.
func (h *APIInvokeHandler) decodeRequest(w http.ResponseWriter, r *http.Request) (kind invokeKind, apiID, gatewayID string, req gatewayrelay.Request, err error) {
	if r.Header.Get("Content-Type") == "" {
		return kind, "", "", req, apperror.UnsupportedMediaType.New().
			WithLogMessage("invoke request without a Content-Type")
	}
	if err := requireJSONContentType(r); err != nil {
		return kind, "", "", req, err
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var body api.APIInvokeRequest
	if err := dec.Decode(&body); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return kind, "", "", req, apperror.PayloadTooLarge.New("The request body exceeds the maximum allowed size.")
		}
		return kind, "", "", req, apperror.ValidationFailed.Wrap(err, "The request body is not a valid invoke request.")
	}
	switch body.ApiType {
	case api.APIInvokeRequestApiTypeRestApi:
		kind = h.rest
	case api.APIInvokeRequestApiTypeGraphqlApi:
		kind = h.graphql
	default:
		return kind, "", "", req, apperror.ValidationFailed.New("apiType must be rest-api or graphql-api.")
	}
	if body.ApiId == "" || body.GatewayId == "" {
		return kind, "", "", req, apperror.ValidationFailed.New("apiId and gatewayId are required.")
	}

	req = gatewayrelay.Request{
		Method: strOrEmpty(body.Method),
		Path:   strOrEmpty(body.Path),
		Body:   strOrEmpty(body.Body),
	}
	if body.BodyEncoding != nil {
		req.BodyEncoding = string(*body.BodyEncoding)
	}
	if body.Query != nil {
		req.Query = toRelayHeaders(*body.Query)
	}
	if body.Headers != nil {
		req.Headers = toRelayHeaders(*body.Headers)
	}

	if kind.apiType == api.APIInvokeRequestApiTypeGraphqlApi {
		if body.Method != nil || body.Path != nil || req.Body == "" {
			return kind, "", "", req, apperror.ValidationFailed.New("A GraphQL API's test request takes a body, and no method or path.")
		}
		// An empty path is the context itself: a GraphQL API's only route on the gateway.
		req.Method = http.MethodPost
		return kind, body.ApiId, body.GatewayId, req, nil
	}
	if req.Path == "" {
		req.Path = "/"
	}
	return kind, body.ApiId, body.GatewayId, req, nil
}

// relayError maps a failed relay to its response. The address and the check
// that refused the request go to the log only.
func relayError(err error, kind invokeKind, apiID, gatewayID string) error {
	logCtx := fmt.Sprintf("invoking %s %s on gateway %s", kind.label, apiID, gatewayID)
	switch {
	case errors.Is(err, egress.ErrTargetNotAllowed),
		errors.Is(err, egress.ErrNoTargetHost),
		errors.Is(err, egress.ErrAddressNotAllowed):
		return kind.errs.targetNotAllowed.Wrap(err).
			WithLogMessage("gateway relay egress policy refused the target while " + logCtx)
	case errors.Is(err, gatewayrelay.ErrBusy):
		return kind.errs.busy.Wrap(err)
	case errors.Is(err, gatewayrelay.ErrBodyTooLarge):
		// Over max_request_bytes once decoded: the same 413 as a request over the read limit.
		return apperror.PayloadTooLarge.New("The request body exceeds the maximum allowed size.")
	case isInvalidRelayRequest(err):
		return kind.errs.requestInvalid.Wrap(err).
			WithLogMessage("refused test request while " + logCtx)
	case errors.Is(err, context.DeadlineExceeded):
		return kind.errs.upstreamTimeout.Wrap(err).
			WithLogMessage("gateway timed out while " + logCtx)
	default:
		return kind.errs.upstreamUnreachable.Wrap(err).
			WithLogMessage("gateway unreachable while " + logCtx)
	}
}

// isInvalidRelayRequest reports whether the sanitizer refused the request itself.
func isInvalidRelayRequest(err error) bool {
	for _, sentinel := range []error{
		gatewayrelay.ErrInvalidMethod,
		gatewayrelay.ErrInvalidPath,
		gatewayrelay.ErrPathEscapesAPI,
		gatewayrelay.ErrInvalidHeaderName,
		gatewayrelay.ErrInvalidHeaderValue,
		gatewayrelay.ErrForbiddenHeader,
		gatewayrelay.ErrTooManyHeaders,
		gatewayrelay.ErrHeadersTooLarge,
		gatewayrelay.ErrTooManyQueryParams,
		gatewayrelay.ErrInvalidQueryName,
		gatewayrelay.ErrInvalidBodyEncoding,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

func toRelayHeaders(in []api.APIInvokeHeader) []gatewayrelay.Header {
	out := make([]gatewayrelay.Header, len(in))
	for i, h := range in {
		out[i] = gatewayrelay.Header{Name: h.Name, Value: h.Value}
	}
	return out
}

func toInvokeResponse(r gatewayrelay.Relayed) api.APIInvokeResponse {
	headers := make([]api.APIInvokeHeader, len(r.Headers))
	for i, h := range r.Headers {
		headers[i] = api.APIInvokeHeader{Name: h.Name, Value: h.Value}
	}
	return api.APIInvokeResponse{
		Outcome: api.Response,
		Response: api.APIInvokeResult{
			Status:       r.Status,
			StatusText:   r.StatusText,
			Headers:      headers,
			Body:         r.Body,
			BodyEncoding: api.APIInvokeBodyEncoding(r.BodyEncoding),
			Truncated:    r.Truncated,
			DurationMs:   r.DurationMs,
		},
	}
}

// RegisterRoutes registers the invoke route.
func (h *APIInvokeHandler) RegisterRoutes(mux router.Router) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/gateway-relay/invoke", middleware.MapErrors(h.slogger, h.InvokeAPI))
}

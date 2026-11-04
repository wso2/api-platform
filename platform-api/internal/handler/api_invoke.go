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

// invokeTargets finds where a test request goes (APIService.GetAPIInvokeTarget).
type invokeTargets interface {
	GetAPIInvokeTarget(apiHandle, gatewayHandle, orgID string) (*service.APIInvokeTarget, error)
}

// relayer sends the request (gatewayrelay.Relay).
type relayer interface {
	Do(ctx context.Context, target gatewayrelay.Target, req gatewayrelay.Request) (gatewayrelay.Relayed, error)
}

// APIInvokeHandler serves POST /gateway-relay/invoke.
type APIInvokeHandler struct {
	targets        invokeTargets
	relay          relayer
	maxBodyBytes   int64
	requestTimeout time.Duration
	slogger        *slog.Logger
}

// NewAPIInvokeHandler builds the handler. maxRequestBytes is the relay's limit
// on the decoded request body, and requestTimeout the limit on one test request.
func NewAPIInvokeHandler(targets invokeTargets, relay relayer, maxRequestBytes int64, requestTimeout time.Duration, slogger *slog.Logger) *APIInvokeHandler {
	return &APIInvokeHandler{
		targets: targets,
		relay:   relay,
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
	// deadline starts when the request arrives, so reading the body and finding the
	// target count against it too. Kept below server.timeouts.write (see config),
	// this leaves time to write a timeout answer before the connection is cut.
	ctx, cancel := context.WithTimeout(r.Context(), h.requestTimeout)
	defer cancel()

	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiID, gatewayID, req, err := h.decodeRequest(w, r)
	if err != nil {
		return err
	}

	invokeTarget, err := h.targets.GetAPIInvokeTarget(apiID, gatewayID, orgID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to resolve gateway %s for REST API %s in org %s", gatewayID, apiID, orgID))
	}
	target, err := gatewayrelay.BuildTarget(gatewayID, invokeTarget.Endpoints, invokeTarget.Context, invokeTarget.Vhost)
	if errors.Is(err, gatewayrelay.ErrUnsupportedScheme) {
		return apperror.RESTAPIInvokeGatewayUnsupportedScheme.New()
	}
	if err != nil {
		return apperror.RESTAPIInvokeGatewayNoEndpoint.New()
	}

	relayed, err := h.relay.Do(ctx, target, req)
	if err != nil {
		return relayError(err, apiID, gatewayID)
	}

	// The relay accepted the method, so this is its normalized form, not raw input.
	method, _ := gatewayrelay.NormalizeMethod(req.Method)
	h.slogger.Info("Relayed a test request to a gateway",
		"restApiId", apiID,
		"gatewayId", gatewayID,
		"method", method,
		"upstreamStatus", relayed.Status,
		"durationMs", relayed.DurationMs,
		"truncated", relayed.Truncated)

	httputil.WriteJSON(w, http.StatusOK, toInvokeResponse(relayed))
	return nil
}

// decodeRequest reads the bounded JSON request: the REST API and gateway to
// invoke, and the request to send. Content-Type must be JSON, which a page on
// another site cannot send without a CORS preflight.
func (h *APIInvokeHandler) decodeRequest(w http.ResponseWriter, r *http.Request) (apiID, gatewayID string, req gatewayrelay.Request, err error) {
	if r.Header.Get("Content-Type") == "" {
		return "", "", req, apperror.UnsupportedMediaType.New().
			WithLogMessage("invoke request without a Content-Type")
	}
	if err := requireJSONContentType(r); err != nil {
		return "", "", req, err
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var body api.RESTAPIInvokeRequest
	if err := dec.Decode(&body); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return "", "", req, apperror.PayloadTooLarge.New("The request body exceeds the maximum allowed size.")
		}
		return "", "", req, apperror.ValidationFailed.Wrap(err, "The request body is not a valid invoke request.")
	}
	if body.RestApiId == "" || body.GatewayId == "" {
		return "", "", req, apperror.ValidationFailed.New("restApiId and gatewayId are required.")
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
	return body.RestApiId, body.GatewayId, req, nil
}

// relayError maps a failed relay to its response. The address and the check
// that refused the request go to the log only.
func relayError(err error, apiID, gatewayID string) error {
	logCtx := fmt.Sprintf("invoking REST API %s on gateway %s", apiID, gatewayID)
	switch {
	case errors.Is(err, egress.ErrTargetNotAllowed),
		errors.Is(err, egress.ErrNoTargetHost),
		errors.Is(err, egress.ErrAddressNotAllowed):
		return apperror.RESTAPIInvokeTargetNotAllowed.Wrap(err).
			WithLogMessage("gateway relay egress policy refused the target while " + logCtx)
	case errors.Is(err, gatewayrelay.ErrBusy):
		return apperror.RESTAPIInvokeBusy.Wrap(err)
	case isInvalidRelayRequest(err):
		return apperror.RESTAPIInvokeRequestInvalid.Wrap(err).
			WithLogMessage("refused test request while " + logCtx)
	case errors.Is(err, context.DeadlineExceeded):
		return apperror.RESTAPIInvokeUpstreamTimeout.Wrap(err).
			WithLogMessage("gateway timed out while " + logCtx)
	default:
		return apperror.RESTAPIInvokeUpstreamUnreachable.Wrap(err).
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
		gatewayrelay.ErrBodyTooLarge,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

func toRelayHeaders(in []api.RESTAPIInvokeHeader) []gatewayrelay.Header {
	out := make([]gatewayrelay.Header, len(in))
	for i, h := range in {
		out[i] = gatewayrelay.Header{Name: h.Name, Value: h.Value}
	}
	return out
}

func toInvokeResponse(r gatewayrelay.Relayed) api.RESTAPIInvokeResponse {
	headers := make([]api.RESTAPIInvokeHeader, len(r.Headers))
	for i, h := range r.Headers {
		headers[i] = api.RESTAPIInvokeHeader{Name: h.Name, Value: h.Value}
	}
	return api.RESTAPIInvokeResponse{
		Outcome: api.Response,
		Response: api.RESTAPIInvokeResult{
			Status:       r.Status,
			StatusText:   r.StatusText,
			Headers:      headers,
			Body:         r.Body,
			BodyEncoding: api.RESTAPIInvokeBodyEncoding(r.BodyEncoding),
			Truncated:    r.Truncated,
			DurationMs:   r.DurationMs,
		},
	}
}

// RegisterRoutes registers the invoke route.
func (h *APIInvokeHandler) RegisterRoutes(mux router.Router) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/gateway-relay/invoke", middleware.MapErrors(h.slogger, h.InvokeAPI))
}

/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package kernel

import (
	"net/http"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/health"
)

// The Envoy route names of the gateway's own health routes. They must equal the names
// the gateway controller gives those routes (constants.GatewayReadyRouteName and
// constants.GatewayHealthyRouteName in the controller). The values are repeated on
// purpose: each component keeps its health code to itself, so neither depends on the
// other. If the two drift apart, the policy engine stops answering the routes and Envoy
// serves the routes' static 200, so the probes pass whatever the runtime's health.
const (
	gatewayReadyRouteName   = "gateway-ready"
	gatewayHealthyRouteName = "gateway-healthy"
)

// healthRoutes holds the checkers behind the gateway's health routes.
type healthRoutes struct {
	liveness  *health.Checker
	readiness *health.Checker
}

// WithHealthRoutes makes the server answer the gateway's health routes itself:
// liveness answers the healthy route and readiness the ready route. The answer is
// produced before any route lookup, so no policy runs for a health probe and the
// probe takes the same path to the policy engine as API traffic does.
func WithHealthRoutes(liveness, readiness *health.Checker) ServerOption {
	return func(s *ExternalProcessorServer) {
		s.health = &healthRoutes{liveness: liveness, readiness: readiness}
	}
}

// healthRouteResponse returns the response to a health route's request headers, and
// false for any other message or route. The routes accept GET and HEAD only; any other
// method gets 405 without the checks being evaluated. HEAD gets the same status and
// headers as GET without a body.
func (s *ExternalProcessorServer) healthRouteResponse(req *extprocv3.ProcessingRequest) (*extprocv3.ProcessingResponse, bool) {
	if s.health == nil || req.GetRequestHeaders() == nil {
		return nil, false
	}
	var checker *health.Checker
	switch s.extractRouteKey(req) {
	case gatewayHealthyRouteName:
		checker = s.health.liveness
	case gatewayReadyRouteName:
		checker = s.health.readiness
	}
	if checker == nil {
		return nil, false
	}

	method := requestMethod(req.GetRequestHeaders())
	if method != http.MethodGet && method != http.MethodHead {
		return immediateHealthResponse(&extprocv3.ImmediateResponse{
			Status: &typev3.HttpStatus{Code: typev3.StatusCode_MethodNotAllowed},
			Headers: buildHeaderValueOptions(map[string]string{
				"allow":         "GET, HEAD",
				"cache-control": "no-store",
			}),
		}), true
	}

	res := checker.Evaluate()
	immediate := &extprocv3.ImmediateResponse{
		Status: &typev3.HttpStatus{Code: typev3.StatusCode(health.StatusCode(res))},
		Headers: buildHeaderValueOptions(map[string]string{
			"content-type":  "application/json",
			"cache-control": "no-store",
		}),
	}
	body, err := health.RenderBody(res)
	if err != nil {
		immediate.Status = &typev3.HttpStatus{Code: typev3.StatusCode_InternalServerError}
	} else if method == http.MethodGet {
		immediate.Body = body
	}
	return immediateHealthResponse(immediate), true
}

// requestMethod returns the request's :method pseudo-header, or "" when it is absent.
func requestMethod(headers *extprocv3.HttpHeaders) string {
	for _, h := range headers.GetHeaders().GetHeaders() {
		if h.GetKey() == ":method" {
			if raw := h.GetRawValue(); len(raw) > 0 {
				return string(raw)
			}
			return h.GetValue()
		}
	}
	return ""
}

func immediateHealthResponse(immediate *extprocv3.ImmediateResponse) *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{ImmediateResponse: immediate},
	}
}

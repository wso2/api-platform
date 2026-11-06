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
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/health"
)

// The Envoy route names of the gateway's own health routes. They must equal the names
// the gateway controller gives those routes; the two components share no Go module,
// so the values are repeated here.
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
// false for any other message or route.
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

	res := checker.Evaluate()
	immediate := &extprocv3.ImmediateResponse{
		Status: &typev3.HttpStatus{Code: typev3.StatusCode(health.StatusCode(res))},
		Headers: buildHeaderValueOptions(map[string]string{
			"content-type":  "application/json",
			"cache-control": "no-store",
		}),
	}
	if body, err := health.RenderBody(res); err == nil {
		immediate.Body = body
	} else {
		immediate.Status = &typev3.HttpStatus{Code: typev3.StatusCode_InternalServerError}
	}
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{ImmediateResponse: immediate},
	}, true
}

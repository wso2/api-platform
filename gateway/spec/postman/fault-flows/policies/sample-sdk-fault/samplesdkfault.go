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

// Package samplesdkfault is a TEST policy: it rejects and describes the failure in FaultDetails.
package samplesdkfault

import (
	"context"
	"fmt"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Policy rejects every request or response with a configured FaultDetails.
type Policy struct {
	onResponse   bool
	status       int
	fault        policy.FaultDetails
	fallbackBody []byte
}

// GetPolicy creates a policy instance from its parameters.
func GetPolicy(_ policy.PolicyMetadata, params map[string]interface{}) (policy.Policy, error) {
	return newPolicy(params)
}

func newPolicy(params map[string]interface{}) (*Policy, error) {
	p := &Policy{status: 403}

	phase := str(params["phase"])
	switch phase {
	case "", "request":
		p.fault.Direction = policy.DirectionRequest
	case "response":
		p.onResponse = true
		p.fault.Direction = policy.DirectionResponse
	default:
		return nil, fmt.Errorf("'phase' must be request or response, got %q", phase)
	}

	if raw, ok := params["statusCode"]; ok && raw != nil {
		n, ok := toInt(raw)
		if !ok || n < 400 || n > 599 {
			return nil, fmt.Errorf("'statusCode' must be an integer from 400 to 599")
		}
		p.status = n
	}

	p.fault.Code = str(params["code"])
	p.fault.Type = str(params["type"])
	p.fault.Message = str(params["message"])
	p.fault.Description = str(params["description"])

	if raw, ok := params["guardrail"].(map[string]interface{}); ok {
		g := &policy.GuardrailDetails{
			InterveningGuardrail: str(raw["interveningGuardrail"]),
			Action:               str(raw["action"]),
			ActionReason:         str(raw["actionReason"]),
		}
		if a, ok := raw["assessments"].(map[string]interface{}); ok {
			g.Assessments = a
		}
		p.fault.Guardrail = g
	}

	if body := str(params["fallbackBody"]); body != "" {
		p.fallbackBody = []byte(body)
	}
	return p, nil
}

// Mode reads nothing it does not need: request headers to reject a request, the response body
// to reject a response.
func (p *Policy) Mode() policy.ProcessingMode {
	if p.onResponse {
		return policy.ProcessingMode{
			RequestHeaderMode:  policy.HeaderModeSkip,
			RequestBodyMode:    policy.BodyModeSkip,
			ResponseHeaderMode: policy.HeaderModeSkip,
			ResponseBodyMode:   policy.BodyModeBuffer,
		}
	}
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequestHeaders rejects the request with the configured description.
func (p *Policy) OnRequestHeaders(_ context.Context, _ *policy.RequestHeaderContext, _ map[string]interface{}) policy.RequestHeaderAction {
	if p.onResponse {
		return policy.UpstreamRequestHeaderModifications{}
	}
	fault := p.fault
	resp := policy.ImmediateResponse{
		StatusCode: p.status,
		IsFault:    true,
		Fault:      &fault,
	}
	if p.fallbackBody != nil {
		resp.Body = p.fallbackBody
		resp.Headers = map[string]string{"content-type": "application/json"}
	}
	return resp
}

// OnResponseBody rejects the upstream's response with the configured description.
func (p *Policy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	if !p.onResponse {
		return policy.DownstreamResponseModifications{}
	}
	fault := p.fault
	status := p.status
	mod := policy.DownstreamResponseModifications{
		StatusCode: &status,
		IsFault:    true,
		Fault:      &fault,
	}
	if p.fallbackBody != nil {
		mod.Body = p.fallbackBody
		mod.HeadersToSet = map[string]string{"content-type": "application/json"}
	}
	return mod
}

func str(v interface{}) string {
	s, _ := v.(string)
	return s
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n == float64(int(n)) {
			return int(n), true
		}
	}
	return 0, false
}

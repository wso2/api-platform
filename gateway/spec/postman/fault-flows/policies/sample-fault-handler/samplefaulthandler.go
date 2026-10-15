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

// Package samplefaulthandler is a TEST fault policy: it reports what it saw and can re-describe
// the failure, write a body, or change the status.
package samplefaulthandler

import (
	"context"
	"fmt"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// SeenHeader carries what the fault policy saw, so a test can assert it ran and on what.
const SeenHeader = "x-sample-fault-handler"

// Policy is a configurable fault-path handler.
type Policy struct {
	message, code, faultType string
	inherit                  bool
	body                     []byte
	contentType              string
	status                   *int
}

var _ policy.FaultPolicy = (*Policy)(nil)

// GetPolicy creates a policy instance from its parameters.
func GetPolicy(_ policy.PolicyMetadata, params map[string]interface{}) (policy.Policy, error) {
	return newPolicy(params)
}

func newPolicy(params map[string]interface{}) (*Policy, error) {
	p := &Policy{contentType: "application/json", inherit: true}
	raw, ok := params["fault"]
	if !ok || raw == nil {
		return p, nil
	}
	f, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("'fault' must be an object")
	}
	p.message = str(f["message"])
	p.code = str(f["code"])
	p.faultType = str(f["type"])
	if b, ok := f["inherit"].(bool); ok {
		p.inherit = b
	}
	if b, ok := f["body"].(string); ok {
		p.body = []byte(b)
	}
	if ct := str(f["contentType"]); ct != "" {
		p.contentType = ct
	}
	if s, ok := f["statusCode"]; ok && s != nil {
		n, ok := toInt(s)
		if !ok || n < 400 || n > 599 {
			return nil, fmt.Errorf("'fault.statusCode' must be an integer from 400 to 599")
		}
		p.status = &n
	}
	return p, nil
}

// Mode skips every normal phase: the policy only runs through OnFault.
func (p *Policy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnFault stamps what it saw and applies whatever it was configured to change.
func (p *Policy) OnFault(_ context.Context, faultCtx *policy.FaultContext, _ map[string]interface{}) *policy.FaultResponse {
	if faultCtx == nil {
		return nil
	}
	seenCode := ""
	if faultCtx.Fault != nil {
		seenCode = faultCtx.Fault.Code
	}
	resp := &policy.FaultResponse{
		HeadersToSet: map[string]string{
			SeenHeader: fmt.Sprintf("source=%s; policy=%s; code=%s", faultCtx.Source, faultCtx.Policy, seenCode),
		},
		StatusCode: p.status,
	}

	if p.message != "" || p.code != "" || p.faultType != "" {
		// With inherit (the default) the re-description starts from what the producer said, so
		// unset fields keep its values. Without it, only the configured fields are sent — and a
		// re-description replaces the producer's description as a whole.
		var redescribed policy.FaultDetails
		if p.inherit && faultCtx.Fault != nil {
			redescribed = *faultCtx.Fault
		}
		if p.message != "" {
			redescribed.Message = p.message
		}
		if p.code != "" {
			redescribed.Code = p.code
		}
		if p.faultType != "" {
			redescribed.Type = p.faultType
		}
		resp.Fault = &redescribed
	}

	if p.body != nil {
		resp.Body = p.body
		resp.HeadersToSet["content-type"] = p.contentType
	}
	return resp
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

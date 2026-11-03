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

package analytics

import (
	"context"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// A RestApi route with body capture fully disabled is the case Task 3 exists to optimize: no
// body buffering/streaming should be requested from the kernel at all.
func TestMode_RestApi_BodyCaptureDisabled_SkipsBothDirections(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{
		"request_body":  false,
		"response_body": false,
		"api_kind":      "RestApi",
	})
	if err != nil {
		t.Fatalf("GetPolicy returned error: %v", err)
	}
	mode := p.Mode()
	if mode.RequestBodyMode != policy.BodyModeSkip {
		t.Errorf("RequestBodyMode = %v, want %v", mode.RequestBodyMode, policy.BodyModeSkip)
	}
	if mode.ResponseBodyMode != policy.BodyModeSkip {
		t.Errorf("ResponseBodyMode = %v, want %v", mode.ResponseBodyMode, policy.BodyModeSkip)
	}
}

// Bodies are still requested (and, end-to-end, still captured) when request_body/response_body
// are explicitly enabled -- the counterpart to the skip case above.
func TestMode_RestApi_BodyCaptureEnabled_BuffersAndStreams(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{
		"request_body":  true,
		"response_body": true,
		"api_kind":      "RestApi",
	})
	if err != nil {
		t.Fatalf("GetPolicy returned error: %v", err)
	}
	mode := p.Mode()
	if mode.RequestBodyMode != policy.BodyModeBuffer {
		t.Errorf("RequestBodyMode = %v, want %v", mode.RequestBodyMode, policy.BodyModeBuffer)
	}
	if mode.ResponseBodyMode != policy.BodyModeStream {
		t.Errorf("ResponseBodyMode = %v, want %v", mode.ResponseBodyMode, policy.BodyModeStream)
	}

	apolicy, ok := p.(*AnalyticsPolicy)
	if !ok {
		t.Fatalf("GetPolicy returned %T, want *AnalyticsPolicy", p)
	}

	// End-to-end: OnRequestBody/OnResponseBody actually capture the payload when the modes
	// above told the kernel to buffer/stream the body in the first place.
	reqAction := apolicy.OnRequestBody(context.Background(), &policy.RequestContext{
		SharedContext: &policy.SharedContext{APIKind: policy.APIKindRestApi},
		Body:          &policy.Body{Content: []byte(`{"hello":"world"}`), Present: true},
	}, map[string]interface{}{"request_body": true, "response_body": true, "api_kind": "RestApi"})
	reqMods, ok := reqAction.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("OnRequestBody returned %T, want UpstreamRequestModifications", reqAction)
	}
	if got := reqMods.AnalyticsMetadata["request_payload"]; got != `{"hello":"world"}` {
		t.Errorf("request_payload = %v, want captured body", got)
	}

	respAction := apolicy.OnResponseBody(context.Background(), &policy.ResponseContext{
		SharedContext: &policy.SharedContext{APIKind: policy.APIKindRestApi},
		ResponseBody:  &policy.Body{Content: []byte(`{"ok":true}`), Present: true},
	}, map[string]interface{}{"request_body": true, "response_body": true, "api_kind": "RestApi"})
	respMods, ok := respAction.(policy.DownstreamResponseModifications)
	if !ok {
		t.Fatalf("OnResponseBody returned %T, want DownstreamResponseModifications", respAction)
	}
	if got := respMods.AnalyticsMetadata["response_payload"]; got != `{"ok":true}` {
		t.Errorf("response_payload = %v, want captured body", got)
	}
}

// MCP analytics (session id, JSON-RPC method) are extracted from the body unconditionally, so
// the body must still be requested even when request_body/response_body capture is off.
func TestMode_Mcp_BodyCaptureDisabled_StillNeedsBody(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{
		"request_body":  false,
		"response_body": false,
		"api_kind":      "Mcp",
	})
	if err != nil {
		t.Fatalf("GetPolicy returned error: %v", err)
	}
	mode := p.Mode()
	if mode.RequestBodyMode != policy.BodyModeBuffer {
		t.Errorf("RequestBodyMode = %v, want %v (Mcp needs the body regardless of capture config)", mode.RequestBodyMode, policy.BodyModeBuffer)
	}
	if mode.ResponseBodyMode != policy.BodyModeStream {
		t.Errorf("ResponseBodyMode = %v, want %v (Mcp needs the body regardless of capture config)", mode.ResponseBodyMode, policy.BodyModeStream)
	}
}

// LLM Provider/Proxy token-usage analytics and Agent analytics are likewise extracted from
// the body unconditionally.
func TestMode_LlmProviderAndProxy_BodyCaptureDisabled_StillNeedsBody(t *testing.T) {
	for _, kind := range []string{"LlmProvider", "LlmProxy", "Agent"} {
		t.Run(kind, func(t *testing.T) {
			p, err := GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{
				"request_body":  false,
				"response_body": false,
				"api_kind":      kind,
			})
			if err != nil {
				t.Fatalf("GetPolicy returned error: %v", err)
			}
			mode := p.Mode()
			if mode.RequestBodyMode != policy.BodyModeBuffer {
				t.Errorf("RequestBodyMode = %v, want %v", mode.RequestBodyMode, policy.BodyModeBuffer)
			}
			if mode.ResponseBodyMode != policy.BodyModeStream {
				t.Errorf("ResponseBodyMode = %v, want %v", mode.ResponseBodyMode, policy.BodyModeStream)
			}
		})
	}
}

// When api_kind is absent (a caller bypassing the standard controller injection path),
// Mode() must fail safe to today's unconditional Buffer/Stream rather than guess.
func TestMode_UnknownAPIKind_FallsBackToBufferStream(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{
		"request_body":  false,
		"response_body": false,
		// no api_kind
	})
	if err != nil {
		t.Fatalf("GetPolicy returned error: %v", err)
	}
	mode := p.Mode()
	if mode.RequestBodyMode != policy.BodyModeBuffer || mode.ResponseBodyMode != policy.BodyModeStream {
		t.Errorf("got RequestBodyMode=%v ResponseBodyMode=%v, want Buffer/Stream fallback", mode.RequestBodyMode, mode.ResponseBodyMode)
	}
}

// When request_body/response_body themselves are entirely absent (params==nil, or a caller that
// never went through InjectSystemPolicies), Mode() must also fail safe to Buffer/Stream, even if
// api_kind happens to be known.
func TestMode_MissingCaptureFlags_FallsBackToBufferStream(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, nil)
	if err != nil {
		t.Fatalf("GetPolicy returned error: %v", err)
	}
	mode := p.Mode()
	if mode.RequestBodyMode != policy.BodyModeBuffer || mode.ResponseBodyMode != policy.BodyModeStream {
		t.Errorf("got RequestBodyMode=%v ResponseBodyMode=%v, want Buffer/Stream fallback", mode.RequestBodyMode, mode.ResponseBodyMode)
	}

	p, err = GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{"api_kind": "RestApi"})
	if err != nil {
		t.Fatalf("GetPolicy returned error: %v", err)
	}
	mode = p.Mode()
	if mode.RequestBodyMode != policy.BodyModeBuffer || mode.ResponseBodyMode != policy.BodyModeStream {
		t.Errorf("got RequestBodyMode=%v ResponseBodyMode=%v, want Buffer/Stream fallback", mode.RequestBodyMode, mode.ResponseBodyMode)
	}
}

// A zero-value AnalyticsPolicy (as some existing tests construct directly for the header-phase
// methods) must not return an invalid empty BodyProcessingMode from Mode().
func TestMode_ZeroValuePolicy_FallsBackToBufferStream(t *testing.T) {
	mode := (&AnalyticsPolicy{}).Mode()
	if mode.RequestBodyMode != policy.BodyModeBuffer || mode.ResponseBodyMode != policy.BodyModeStream {
		t.Errorf("got RequestBodyMode=%v ResponseBodyMode=%v, want Buffer/Stream fallback", mode.RequestBodyMode, mode.ResponseBodyMode)
	}
}

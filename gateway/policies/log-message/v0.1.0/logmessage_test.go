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

package logmessage

import (
	"encoding/json"
	"strings"
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// Helper function to create test headers
func createTestHeaders(headers map[string]string) *policy.Headers {
	headerMap := make(map[string][]string)
	for key, value := range headers {
		headerMap[key] = []string{value}
	}
	return policy.NewHeaders(headerMap)
}

func TestLogMessagePolicy_Mode(t *testing.T) {
	p := &LogMessagePolicy{}
	mode := p.Mode()

	expectedMode := policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}

	if mode != expectedMode {
		t.Errorf("Expected mode %+v, got %+v", expectedMode, mode)
	}
}

func TestLogMessagePolicy_OnRequest_LogPayloadAndHeaders(t *testing.T) {
	p := &LogMessagePolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"user": "test", "action": "login"}`),
			Present: true,
		},
		Headers: createTestHeaders(map[string]string{
			"content-type":    "application/json",
			"x-request-id":    "test-123",
			"authorization":   "Bearer token123",
			"x-custom-header": "custom-value",
		}),
		Method: "POST",
		Path:   "/api/users/login",
	}

	params := map[string]interface{}{
		"logRequestPayload": true,
		"logRequestHeaders": true,
	}

	result := p.OnRequest(ctx, params)

	// Should return empty modifications (no request modification)
	if _, ok := result.(policy.UpstreamRequestModifications); !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}

	mods := result.(policy.UpstreamRequestModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification, got body: %s", string(mods.Body))
	}
}

func TestLogMessagePolicy_OnRequest_LogPayloadOnly(t *testing.T) {
	p := &LogMessagePolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"message": "hello world"}`),
			Present: true,
		},
		Headers: createTestHeaders(map[string]string{
			"content-type": "application/json",
			"x-request-id": "req-456",
		}),
		Method: "GET",
		Path:   "/api/messages",
	}

	params := map[string]interface{}{
		"logRequestPayload": true,
		"logRequestHeaders": false,
	}

	result := p.OnRequest(ctx, params)

	// Should return empty modifications
	mods := result.(policy.UpstreamRequestModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification, got body: %s", string(mods.Body))
	}
}

func TestLogMessagePolicy_OnRequest_LogHeadersOnly(t *testing.T) {
	p := &LogMessagePolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"data": "sensitive"}`),
			Present: true,
		},
		Headers: createTestHeaders(map[string]string{
			"content-type": "application/json",
			"user-agent":   "test-client/1.0",
		}),
		Method: "PUT",
		Path:   "/api/data",
	}

	params := map[string]interface{}{
		"logRequestPayload": false,
		"logRequestHeaders": true,
	}

	result := p.OnRequest(ctx, params)

	// Should return empty modifications
	mods := result.(policy.UpstreamRequestModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification, got body: %s", string(mods.Body))
	}
}

func TestLogMessagePolicy_OnRequest_ExcludeHeaders(t *testing.T) {
	p := &LogMessagePolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"test": true}`),
			Present: true,
		},
		Headers: createTestHeaders(map[string]string{
			"content-type":  "application/json",
			"authorization": "Bearer secret",
			"x-api-key":     "api-key-123",
			"user-agent":    "test-client",
		}),
		Method: "POST",
		Path:   "/api/test",
	}

	params := map[string]interface{}{
		"logRequestPayload":      true,
		"logRequestHeaders":      true,
		"excludedRequestHeaders": "Authorization,X-API-Key",
	}

	result := p.OnRequest(ctx, params)

	// Should return empty modifications
	mods := result.(policy.UpstreamRequestModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification, got body: %s", string(mods.Body))
	}
}

func TestLogMessagePolicy_OnRequest_EmptyBody(t *testing.T) {
	p := &LogMessagePolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte{},
			Present: false,
		},
		Headers: createTestHeaders(map[string]string{
			"content-type": "application/json",
		}),
		Method: "GET",
		Path:   "/api/status",
	}

	params := map[string]interface{}{
		"logRequestPayload": true,
		"logRequestHeaders": true,
	}

	result := p.OnRequest(ctx, params)

	// Should return empty modifications
	mods := result.(policy.UpstreamRequestModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification, got body: %s", string(mods.Body))
	}
}

func TestLogMessagePolicy_OnResponse_LogPayloadAndHeaders(t *testing.T) {
	p := &LogMessagePolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`{"status": "success", "data": {"id": 123}}`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders(map[string]string{
			"content-type":  "application/json",
			"x-request-id":  "resp-789",
			"cache-control": "no-cache",
		}),
		RequestMethod: "POST",
		RequestPath:   "/api/users",
	}

	params := map[string]interface{}{
		"logResponsePayload": true,
		"logResponseHeaders": true,
	}

	result := p.OnResponse(ctx, params)

	// Should return empty modifications
	if _, ok := result.(policy.UpstreamResponseModifications); !ok {
		t.Errorf("Expected UpstreamResponseModifications, got %T", result)
	}

	mods := result.(policy.UpstreamResponseModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification, got body: %s", string(mods.Body))
	}
}

func TestLogMessagePolicy_OnResponse_LogPayloadOnly(t *testing.T) {
	p := &LogMessagePolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`{"result": "processed"}`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders(map[string]string{
			"content-type": "application/json",
		}),
		RequestMethod: "GET",
		RequestPath:   "/api/process",
	}

	params := map[string]interface{}{
		"logResponsePayload": true,
		"logResponseHeaders": false,
	}

	result := p.OnResponse(ctx, params)

	// Should return empty modifications
	mods := result.(policy.UpstreamResponseModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification, got body: %s", string(mods.Body))
	}
}

func TestLogMessagePolicy_OnRequest_BothDisabled(t *testing.T) {
	p := &LogMessagePolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"test": true}`),
			Present: true,
		},
		Headers: createTestHeaders(map[string]string{
			"content-type": "application/json",
		}),
		Method: "POST",
		Path:   "/api/test",
	}

	params := map[string]interface{}{
		"logRequestPayload": false,
		"logRequestHeaders": false,
	}

	result := p.OnRequest(ctx, params)

	// Should return empty modifications and skip logging
	mods := result.(policy.UpstreamRequestModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification, got body: %s", string(mods.Body))
	}
}

func TestLogMessagePolicy_OnResponse_BothDisabled(t *testing.T) {
	p := &LogMessagePolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`{"result": "processed"}`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders(map[string]string{
			"content-type": "application/json",
		}),
		RequestMethod: "GET",
		RequestPath:   "/api/process",
	}

	params := map[string]interface{}{
		"logResponsePayload": false,
		"logResponseHeaders": false,
	}

	result := p.OnResponse(ctx, params)

	// Should return empty modifications and skip logging
	mods := result.(policy.UpstreamResponseModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification, got body: %s", string(mods.Body))
	}
}

func TestLogMessagePolicy_ParseExcludedHeaders(t *testing.T) {
	p := &LogMessagePolicy{}

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Single header",
			input:    "Authorization",
			expected: []string{"authorization"},
		},
		{
			name:     "Multiple headers",
			input:    "Authorization,X-API-Key,Content-Length",
			expected: []string{"authorization", "x-api-key", "content-length"},
		},
		{
			name:     "Headers with spaces",
			input:    " Authorization , X-API-Key , Content-Length ",
			expected: []string{"authorization", "x-api-key", "content-length"},
		},
		{
			name:     "Empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "Mixed case",
			input:    "AUTHORIZATION,x-api-key,Content-Type",
			expected: []string{"authorization", "x-api-key", "content-type"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.parseExcludedHeaders(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d headers, got %d", len(tt.expected), len(result))
			}

			for _, expectedHeader := range tt.expected {
				if _, exists := result[expectedHeader]; !exists {
					t.Errorf("Expected header '%s' to be excluded", expectedHeader)
				}
			}
		})
	}
}

func TestLogMessagePolicy_BuildHeadersMap(t *testing.T) {
	p := &LogMessagePolicy{}

	headers := createTestHeaders(map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token123",
		"X-API-Key":     "api-key-456",
		"User-Agent":    "test-client/1.0",
		"X-Request-ID":  "req-123",
	})

	// Test with excluded headers
	result := p.buildHeadersMap(headers, "X-API-Key,User-Agent")

	// Authorization should be masked (check with lowercase key since headers are normalized)
	var authFound bool
	for key, value := range result {
		if strings.ToLower(key) == "authorization" {
			if value != "***" {
				t.Errorf("Expected Authorization header to be masked with '***', got: %v", value)
			}
			authFound = true
			break
		}
	}
	if !authFound {
		t.Errorf("Expected Authorization header to be present and masked")
	}

	// X-API-Key should be excluded
	var apiKeyFound bool
	for key := range result {
		if strings.ToLower(key) == "x-api-key" {
			apiKeyFound = true
			break
		}
	}
	if apiKeyFound {
		t.Errorf("Expected X-API-Key header to be excluded")
	}

	// User-Agent should be excluded
	var userAgentFound bool
	for key := range result {
		if strings.ToLower(key) == "user-agent" {
			userAgentFound = true
			break
		}
	}
	if userAgentFound {
		t.Errorf("Expected User-Agent header to be excluded")
	}

	// Content-Type should be present
	var contentTypeFound bool
	for key, value := range result {
		if strings.ToLower(key) == "content-type" {
			if value != "application/json" {
				t.Errorf("Expected Content-Type header to have value 'application/json', got: %v", value)
			}
			contentTypeFound = true
			break
		}
	}
	if !contentTypeFound {
		t.Errorf("Expected Content-Type header to be present")
	}

	// X-Request-ID should be present
	var requestIDFound bool
	for key, value := range result {
		if strings.ToLower(key) == "x-request-id" {
			if value != "req-123" {
				t.Errorf("Expected X-Request-ID header to have value 'req-123', got: %v", value)
			}
			requestIDFound = true
			break
		}
	}
	if !requestIDFound {
		t.Errorf("Expected X-Request-ID header to be present")
	}
}

func TestLogMessagePolicy_GetRequestID(t *testing.T) {
	p := &LogMessagePolicy{}

	// Test with request ID present
	headersWithID := createTestHeaders(map[string]string{
		"x-request-id": "test-request-123",
		"content-type": "application/json",
	})

	requestID := p.getRequestID(headersWithID)
	if requestID != "test-request-123" {
		t.Errorf("Expected request ID 'test-request-123', got: %s", requestID)
	}

	// Test without request ID
	headersWithoutID := createTestHeaders(map[string]string{
		"content-type": "application/json",
	})

	requestID = p.getRequestID(headersWithoutID)
	if requestID != ErrMsgMissingReqID {
		t.Errorf("Expected request ID '%s', got: %s", ErrMsgMissingReqID, requestID)
	}
}

func TestLogRecord_JSONMarshaling(t *testing.T) {
	logRecord := LogRecord{
		MediationFlow: MediationFlowRequest,
		RequestID:     "test-123",
		HTTPMethod:    "POST",
		ResourcePath:  "/api/users",
		Payload:       `{"user": "test"}`,
		Headers: map[string]interface{}{
			"content-type": "application/json",
			"user-agent":   "test-client",
		},
	}

	jsonData, err := json.Marshal(logRecord)
	if err != nil {
		t.Fatalf("Failed to marshal log record: %v", err)
	}

	// Verify the JSON contains expected fields
	jsonStr := string(jsonData)
	if !strings.Contains(jsonStr, "mediation-flow") {
		t.Errorf("Expected JSON to contain 'mediation-flow' field")
	}
	if !strings.Contains(jsonStr, "request-id") {
		t.Errorf("Expected JSON to contain 'request-id' field")
	}
	if !strings.Contains(jsonStr, "http-method") {
		t.Errorf("Expected JSON to contain 'http-method' field")
	}
	if !strings.Contains(jsonStr, "resource-path") {
		t.Errorf("Expected JSON to contain 'resource-path' field")
	}
}

func TestGetPolicy(t *testing.T) {
	metadata := policy.PolicyMetadata{}
	params := map[string]interface{}{}

	policyInstance, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}

	if _, ok := policyInstance.(*LogMessagePolicy); !ok {
		t.Errorf("Expected *LogMessagePolicy, got %T", policyInstance)
	}
}

func TestLogMessagePolicy_SeparateRequestResponseFlows(t *testing.T) {
	p := &LogMessagePolicy{}

	// Test request flow only
	reqCtx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"request": "data"}`),
			Present: true,
		},
		Headers: createTestHeaders(map[string]string{
			"content-type":  "application/json",
			"x-request-id":  "test-123",
			"authorization": "Bearer secret",
		}),
		Method: "POST",
		Path:   "/api/test",
	}

	reqParams := map[string]interface{}{
		"logRequestPayload":      true,
		"logRequestHeaders":      true,
		"excludedRequestHeaders": "Authorization",
		"logResponsePayload":     false,
		"logResponseHeaders":     false,
	}

	reqResult := p.OnRequest(reqCtx, reqParams)
	if _, ok := reqResult.(policy.UpstreamRequestModifications); !ok {
		t.Errorf("Expected UpstreamRequestModifications for request, got %T", reqResult)
	}

	// Test response flow only
	respCtx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`{"response": "data"}`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders(map[string]string{
			"content-type": "application/json",
			"x-request-id": "test-123",
			"set-cookie":   "session=abc123",
		}),
		RequestMethod: "POST",
		RequestPath:   "/api/test",
	}

	respParams := map[string]interface{}{
		"logRequestPayload":       false,
		"logRequestHeaders":       false,
		"logResponsePayload":      true,
		"logResponseHeaders":      true,
		"excludedResponseHeaders": "Set-Cookie",
	}

	respResult := p.OnResponse(respCtx, respParams)
	if _, ok := respResult.(policy.UpstreamResponseModifications); !ok {
		t.Errorf("Expected UpstreamResponseModifications for response, got %T", respResult)
	}
}

func TestLogMessagePolicy_ExcludedResponseHeaders(t *testing.T) {
	p := &LogMessagePolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`{"status": "success"}`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders(map[string]string{
			"content-type":     "application/json",
			"x-request-id":     "test-456",
			"set-cookie":       "session=xyz789",
			"x-internal-token": "internal-secret",
		}),
		RequestMethod: "GET",
		RequestPath:   "/api/data",
	}

	params := map[string]interface{}{
		"logResponsePayload":      true,
		"logResponseHeaders":      true,
		"excludedResponseHeaders": "Set-Cookie,X-Internal-Token",
	}

	result := p.OnResponse(ctx, params)

	// Should return empty modifications
	mods := result.(policy.UpstreamResponseModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification, got body: %s", string(mods.Body))
	}
}

func TestLogMessagePolicy_DefaultBehavior_NoParams(t *testing.T) {
	p := &LogMessagePolicy{}

	// Test request with no parameters (should default to no logging)
	reqCtx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"test": "data"}`),
			Present: true,
		},
		Headers: createTestHeaders(map[string]string{
			"content-type": "application/json",
			"x-request-id": "test-123",
		}),
		Method: "POST",
		Path:   "/api/test",
	}

	// No parameters provided - should default to all false
	params := map[string]interface{}{}

	reqResult := p.OnRequest(reqCtx, params)
	if _, ok := reqResult.(policy.UpstreamRequestModifications); !ok {
		t.Errorf("Expected UpstreamRequestModifications for request, got %T", reqResult)
	}

	// Test response with no parameters (should default to no logging)
	respCtx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`{"result": "success"}`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders(map[string]string{
			"content-type": "application/json",
			"x-request-id": "test-123",
		}),
		RequestMethod: "POST",
		RequestPath:   "/api/test",
	}

	respResult := p.OnResponse(respCtx, params)
	if _, ok := respResult.(policy.UpstreamResponseModifications); !ok {
		t.Errorf("Expected UpstreamResponseModifications for response, got %T", respResult)
	}
}

func TestLogMessagePolicy_PartialParams(t *testing.T) {
	p := &LogMessagePolicy{}

	// Test with only some parameters specified (others should default to false)
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"test": "data"}`),
			Present: true,
		},
		Headers: createTestHeaders(map[string]string{
			"content-type": "application/json",
			"x-request-id": "test-456",
		}),
		Method: "GET",
		Path:   "/api/test",
	}

	// Only specify request payload logging - others should default to false
	params := map[string]interface{}{
		"logRequestPayload": true,
		// logRequestHeaders defaults to false
		// logResponsePayload defaults to false
		// logResponseHeaders defaults to false
	}

	result := p.OnRequest(ctx, params)
	if _, ok := result.(policy.UpstreamRequestModifications); !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}
}

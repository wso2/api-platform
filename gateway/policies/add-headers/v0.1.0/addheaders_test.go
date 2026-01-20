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

package addheaders

import (
	"strings"
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// Helper function to create test headers
func createTestHeaders(headers map[string]string) *policy.Headers {
	headerMap := make(map[string][]string)
	for k, v := range headers {
		headerMap[k] = []string{v}
	}
	return policy.NewHeaders(headerMap)
}

func TestAddHeadersPolicy_Mode(t *testing.T) {
	p := &AddHeadersPolicy{}
	mode := p.Mode()

	expectedMode := policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeSkip,
	}

	if mode != expectedMode {
		t.Errorf("Expected mode %+v, got %+v", expectedMode, mode)
	}
}

func TestGetPolicy(t *testing.T) {
	metadata := policy.PolicyMetadata{}
	params := map[string]interface{}{}

	p, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if p == nil {
		t.Fatal("Expected policy instance, got nil")
	}

	if _, ok := p.(*AddHeadersPolicy); !ok {
		t.Errorf("Expected AddHeadersPolicy, got %T", p)
	}
}

func TestAddHeadersPolicy_OnRequest_NoHeaders(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.RequestContext{
		Headers: createTestHeaders(map[string]string{
			"content-type": "application/json",
		}),
	}

	// No requestHeaders parameter
	params := map[string]interface{}{}
	result := p.OnRequest(ctx, params)

	// Should return empty modifications
	mods, ok := result.(policy.UpstreamRequestModifications)
	if !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}

	if len(mods.AppendHeaders) != 0 {
		t.Errorf("Expected no headers to be appended, got %d headers", len(mods.AppendHeaders))
	}
}

func TestAddHeadersPolicy_OnRequest_SingleHeader(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.RequestContext{
		Headers: createTestHeaders(map[string]string{
			"content-type": "application/json",
		}),
	}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Custom-Header",
				"value": "custom-value",
			},
		},
	}

	result := p.OnRequest(ctx, params)

	mods, ok := result.(policy.UpstreamRequestModifications)
	if !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}

	if len(mods.AppendHeaders) != 1 {
		t.Errorf("Expected 1 header to be appended, got %d headers", len(mods.AppendHeaders))
	}

	expectedHeaderName := "x-custom-header" // Should be normalized to lowercase
	if headerValues, exists := mods.AppendHeaders[expectedHeaderName]; !exists {
		t.Errorf("Expected header '%s' to be present in AppendHeaders", expectedHeaderName)
	} else if len(headerValues) != 1 || headerValues[0] != "custom-value" {
		t.Errorf("Expected header '%s' to have value ['custom-value'], got %v",
			expectedHeaderName, headerValues)
	}
}

func TestAddHeadersPolicy_OnRequest_MultipleHeaders(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.RequestContext{
		Headers: createTestHeaders(map[string]string{
			"content-type": "application/json",
		}),
	}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-API-Key",
				"value": "secret-key-123",
			},
			map[string]interface{}{
				"name":  "X-Client-Version",
				"value": "1.2.3",
			},
			map[string]interface{}{
				"name":  "X-Request-ID",
				"value": "req-456",
			},
		},
	}

	result := p.OnRequest(ctx, params)

	mods, ok := result.(policy.UpstreamRequestModifications)
	if !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}

	if len(mods.AppendHeaders) != 3 {
		t.Errorf("Expected 3 headers to be appended, got %d headers", len(mods.AppendHeaders))
	}

	expectedHeaders := map[string]string{
		"x-api-key":        "secret-key-123",
		"x-client-version": "1.2.3",
		"x-request-id":     "req-456",
	}

	for name, expectedValue := range expectedHeaders {
		if headerValues, exists := mods.AppendHeaders[name]; !exists {
			t.Errorf("Expected header '%s' to be present in AppendHeaders", name)
		} else if len(headerValues) != 1 || headerValues[0] != expectedValue {
			t.Errorf("Expected header '%s' to have value ['%s'], got %v",
				name, expectedValue, headerValues)
		}
	}
}

func TestAddHeadersPolicy_OnRequest_HeaderNameNormalization(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.RequestContext{
		Headers: createTestHeaders(map[string]string{}),
	}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{
			map[string]interface{}{
				"name":  "  X-UPPER-CASE  ", // With spaces and uppercase
				"value": "test-value",
			},
		},
	}

	result := p.OnRequest(ctx, params)

	mods, ok := result.(policy.UpstreamRequestModifications)
	if !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}

	expectedHeaderName := "x-upper-case" // Should be trimmed and lowercase
	if headerValues, exists := mods.AppendHeaders[expectedHeaderName]; !exists {
		t.Errorf("Expected header '%s' to be normalized and appended, got headers: %v",
			expectedHeaderName, mods.AppendHeaders)
	} else if len(headerValues) != 1 || headerValues[0] != "test-value" {
		t.Errorf("Expected header '%s' to have value ['test-value'], got %v",
			expectedHeaderName, headerValues)
	}
}

func TestAddHeadersPolicy_OnResponse_NoHeaders(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.ResponseContext{
		ResponseHeaders: createTestHeaders(map[string]string{
			"content-type": "application/json",
		}),
	}

	// No responseHeaders parameter
	params := map[string]interface{}{}
	result := p.OnResponse(ctx, params)

	// Should return empty modifications
	mods, ok := result.(policy.UpstreamResponseModifications)
	if !ok {
		t.Errorf("Expected UpstreamResponseModifications, got %T", result)
	}

	if len(mods.AppendHeaders) != 0 {
		t.Errorf("Expected no headers to be appended, got %d headers", len(mods.AppendHeaders))
	}
}

func TestAddHeadersPolicy_OnResponse_SingleHeader(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.ResponseContext{
		ResponseHeaders: createTestHeaders(map[string]string{
			"content-type": "application/json",
		}),
	}

	params := map[string]interface{}{
		"responseHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Response-Time",
				"value": "123ms",
			},
		},
	}

	result := p.OnResponse(ctx, params)

	mods, ok := result.(policy.UpstreamResponseModifications)
	if !ok {
		t.Errorf("Expected UpstreamResponseModifications, got %T", result)
	}

	if len(mods.AppendHeaders) != 1 {
		t.Errorf("Expected 1 header to be appended, got %d headers", len(mods.AppendHeaders))
	}

	expectedHeaderName := "x-response-time" // Should be normalized to lowercase
	if headerValues, exists := mods.AppendHeaders[expectedHeaderName]; !exists {
		t.Errorf("Expected header '%s' to be present in AppendHeaders", expectedHeaderName)
	} else if len(headerValues) != 1 || headerValues[0] != "123ms" {
		t.Errorf("Expected header '%s' to have value ['123ms'], got %v",
			expectedHeaderName, headerValues)
	}
}

func TestAddHeadersPolicy_OnResponse_MultipleHeaders(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.ResponseContext{
		ResponseHeaders: createTestHeaders(map[string]string{
			"content-type": "application/json",
		}),
	}

	params := map[string]interface{}{
		"responseHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Cache-Status",
				"value": "MISS",
			},
			map[string]interface{}{
				"name":  "X-Server-Version",
				"value": "2.1.0",
			},
			map[string]interface{}{
				"name":  "X-Content-Hash",
				"value": "abc123def456",
			},
		},
	}

	result := p.OnResponse(ctx, params)

	mods, ok := result.(policy.UpstreamResponseModifications)
	if !ok {
		t.Errorf("Expected UpstreamResponseModifications, got %T", result)
	}

	if len(mods.AppendHeaders) != 3 {
		t.Errorf("Expected 3 headers to be appended, got %d headers", len(mods.AppendHeaders))
	}

	expectedHeaders := map[string]string{
		"x-cache-status":   "MISS",
		"x-server-version": "2.1.0",
		"x-content-hash":   "abc123def456",
	}

	for name, expectedValue := range expectedHeaders {
		if headerValues, exists := mods.AppendHeaders[name]; !exists {
			t.Errorf("Expected header '%s' to be present in AppendHeaders", name)
		} else if len(headerValues) != 1 || headerValues[0] != expectedValue {
			t.Errorf("Expected header '%s' to have value ['%s'], got %v",
				name, expectedValue, headerValues)
		}
	}
}

func TestAddHeadersPolicy_BothRequestAndResponse(t *testing.T) {
	p := &AddHeadersPolicy{}

	// Test request phase
	reqCtx := &policy.RequestContext{
		Headers: createTestHeaders(map[string]string{}),
	}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Request-Header",
				"value": "request-value",
			},
		},
		"responseHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Response-Header",
				"value": "response-value",
			},
		},
	}

	reqResult := p.OnRequest(reqCtx, params)
	reqMods, ok := reqResult.(policy.UpstreamRequestModifications)
	if !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", reqResult)
	}

	if headerValues, exists := reqMods.AppendHeaders["x-request-header"]; !exists {
		t.Errorf("Expected request header to be appended")
	} else if len(headerValues) != 1 || headerValues[0] != "request-value" {
		t.Errorf("Expected request header to have value ['request-value'], got %v", headerValues)
	}

	// Test response phase
	respCtx := &policy.ResponseContext{
		ResponseHeaders: createTestHeaders(map[string]string{}),
	}

	respResult := p.OnResponse(respCtx, params)
	respMods, ok := respResult.(policy.UpstreamResponseModifications)
	if !ok {
		t.Errorf("Expected UpstreamResponseModifications, got %T", respResult)
	}

	if headerValues, exists := respMods.AppendHeaders["x-response-header"]; !exists {
		t.Errorf("Expected response header to be appended")
	} else if len(headerValues) != 1 || headerValues[0] != "response-value" {
		t.Errorf("Expected response header to have value ['response-value'], got %v", headerValues)
	}
}

func TestAddHeadersPolicy_EmptyHeadersList(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.RequestContext{
		Headers: createTestHeaders(map[string]string{}),
	}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{}, // Empty array
	}

	result := p.OnRequest(ctx, params)

	mods, ok := result.(policy.UpstreamRequestModifications)
	if !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}

	if len(mods.AppendHeaders) != 0 {
		t.Errorf("Expected no headers to be appended for empty array, got %d headers", len(mods.AppendHeaders))
	}
}

func TestAddHeadersPolicy_InvalidHeadersType(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.RequestContext{
		Headers: createTestHeaders(map[string]string{}),
	}

	params := map[string]interface{}{
		"requestHeaders": "not-an-array", // Invalid type
	}

	result := p.OnRequest(ctx, params)

	mods, ok := result.(policy.UpstreamRequestModifications)
	if !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}

	if len(mods.AppendHeaders) != 0 {
		t.Errorf("Expected no headers to be appended for invalid type, got %d headers", len(mods.AppendHeaders))
	}
}

func TestAddHeadersPolicy_InvalidHeaderEntry(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.RequestContext{
		Headers: createTestHeaders(map[string]string{}),
	}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{
			"not-an-object", // Invalid entry type
			map[string]interface{}{
				"name":  "Valid-Header",
				"value": "valid-value",
			},
		},
	}

	result := p.OnRequest(ctx, params)

	mods, ok := result.(policy.UpstreamRequestModifications)
	if !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}

	// Should only process valid entries
	if len(mods.AppendHeaders) != 1 {
		t.Errorf("Expected 1 valid header to be appended, got %d headers", len(mods.AppendHeaders))
	}

	if headerValues, exists := mods.AppendHeaders["valid-header"]; !exists {
		t.Errorf("Expected valid header to be processed correctly")
	} else if len(headerValues) != 1 || headerValues[0] != "valid-value" {
		t.Errorf("Expected valid header to have value ['valid-value'], got %v", headerValues)
	}
}

func TestAddHeadersPolicy_SpecialCharactersInValues(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.RequestContext{
		Headers: createTestHeaders(map[string]string{}),
	}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Special-Chars",
				"value": "value with spaces, symbols: !@#$%^&*()_+{}|:<>?[]\\;'\"",
			},
		},
	}

	result := p.OnRequest(ctx, params)

	mods, ok := result.(policy.UpstreamRequestModifications)
	if !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}

	expectedValue := "value with spaces, symbols: !@#$%^&*()_+{}|:<>?[]\\;'\""
	if headerValues, exists := mods.AppendHeaders["x-special-chars"]; !exists {
		t.Errorf("Expected special characters header to be present")
	} else if len(headerValues) != 1 || headerValues[0] != expectedValue {
		t.Errorf("Expected special characters to be preserved in header value, got %v", headerValues)
	}
}

// Test validation helper functions
func TestAddHeadersPolicy_ValidateHeaderEntries_ValidInput(t *testing.T) {
	p := &AddHeadersPolicy{}

	validHeaders := []interface{}{
		map[string]interface{}{
			"name":  "X-Test-Header",
			"value": "test-value",
		},
	}

	err := p.validateHeaderEntries(validHeaders, "testHeaders")
	if err != nil {
		t.Errorf("Expected no error for valid input, got: %v", err)
	}
}

func TestAddHeadersPolicy_ValidateHeaderEntries_NotArray(t *testing.T) {
	p := &AddHeadersPolicy{}

	err := p.validateHeaderEntries("not-an-array", "testHeaders")
	if err == nil || !strings.Contains(err.Error(), "must be an array") {
		t.Errorf("Expected 'must be an array' error, got: %v", err)
	}
}

func TestAddHeadersPolicy_ValidateHeaderEntries_EmptyArray(t *testing.T) {
	p := &AddHeadersPolicy{}

	err := p.validateHeaderEntries([]interface{}{}, "testHeaders")
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("Expected 'cannot be empty' error, got: %v", err)
	}
}

func TestAddHeadersPolicy_ValidateHeaderEntries_MissingName(t *testing.T) {
	p := &AddHeadersPolicy{}

	invalidHeaders := []interface{}{
		map[string]interface{}{
			"value": "test-value", // Missing name
		},
	}

	err := p.validateHeaderEntries(invalidHeaders, "testHeaders")
	if err == nil || !strings.Contains(err.Error(), "missing required 'name' field") {
		t.Errorf("Expected 'missing required name field' error, got: %v", err)
	}
}

func TestAddHeadersPolicy_ValidateHeaderEntries_MissingValue(t *testing.T) {
	p := &AddHeadersPolicy{}

	invalidHeaders := []interface{}{
		map[string]interface{}{
			"name": "X-Test-Header", // Missing value
		},
	}

	err := p.validateHeaderEntries(invalidHeaders, "testHeaders")
	if err == nil || !strings.Contains(err.Error(), "missing required 'value' field") {
		t.Errorf("Expected 'missing required value field' error, got: %v", err)
	}
}

func TestAddHeadersPolicy_ValidateHeaderEntries_EmptyName(t *testing.T) {
	p := &AddHeadersPolicy{}

	invalidHeaders := []interface{}{
		map[string]interface{}{
			"name":  "   ", // Empty/whitespace only name
			"value": "test-value",
		},
	}

	err := p.validateHeaderEntries(invalidHeaders, "testHeaders")
	if err == nil || !strings.Contains(err.Error(), "name cannot be empty") {
		t.Errorf("Expected 'name cannot be empty' error, got: %v", err)
	}
}

// Test Validate method
func TestAddHeadersPolicy_Validate_ValidConfiguration(t *testing.T) {
	p := &AddHeadersPolicy{}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Request-Header",
				"value": "request-value",
			},
		},
		"responseHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Response-Header",
				"value": "response-value",
			},
		},
	}

	err := p.Validate(params)
	if err != nil {
		t.Errorf("Expected no error for valid configuration, got: %v", err)
	}
}

func TestAddHeadersPolicy_Validate_OnlyRequestHeaders(t *testing.T) {
	p := &AddHeadersPolicy{}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Request-Header",
				"value": "request-value",
			},
		},
	}

	err := p.Validate(params)
	if err != nil {
		t.Errorf("Expected no error for valid requestHeaders only, got: %v", err)
	}
}

func TestAddHeadersPolicy_Validate_OnlyResponseHeaders(t *testing.T) {
	p := &AddHeadersPolicy{}

	params := map[string]interface{}{
		"responseHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Response-Header",
				"value": "response-value",
			},
		},
	}

	err := p.Validate(params)
	if err != nil {
		t.Errorf("Expected no error for valid responseHeaders only, got: %v", err)
	}
}

func TestAddHeadersPolicy_Validate_NoHeadersSpecified(t *testing.T) {
	p := &AddHeadersPolicy{}

	params := map[string]interface{}{}

	err := p.Validate(params)
	if err == nil || !strings.Contains(err.Error(), "at least one of 'requestHeaders' or 'responseHeaders' must be specified") {
		t.Errorf("Expected 'at least one must be specified' error, got: %v", err)
	}
}

func TestAddHeadersPolicy_Validate_InvalidRequestHeaders(t *testing.T) {
	p := &AddHeadersPolicy{}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{
			map[string]interface{}{
				"name": "X-Test-Header",
				// Missing value field
			},
		},
	}

	err := p.Validate(params)
	if err == nil || !strings.Contains(err.Error(), "missing required 'value' field") {
		t.Errorf("Expected 'missing required value field' error, got: %v", err)
	}
}

func TestAddHeadersPolicy_Validate_InvalidResponseHeaders(t *testing.T) {
	p := &AddHeadersPolicy{}

	params := map[string]interface{}{
		"responseHeaders": "not-an-array", // Invalid type
	}

	err := p.Validate(params)
	if err == nil || !strings.Contains(err.Error(), "must be an array") {
		t.Errorf("Expected 'must be an array' error, got: %v", err)
	}
}

func TestAddHeadersPolicy_Validate_EmptyRequestHeaders(t *testing.T) {
	p := &AddHeadersPolicy{}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{}, // Empty array
	}

	err := p.Validate(params)
	if err == nil || !strings.Contains(err.Error(), "requestHeaders cannot be empty") {
		t.Errorf("Expected 'requestHeaders cannot be empty' error, got: %v", err)
	}
}

func TestAddHeadersPolicy_Validate_BothInvalid(t *testing.T) {
	p := &AddHeadersPolicy{}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{
			map[string]interface{}{
				// Missing both name and value
			},
		},
		"responseHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Response-Header",
				"value": "response-value",
			},
		},
	}

	err := p.Validate(params)
	// Should fail on requestHeaders validation first
	if err == nil || !strings.Contains(err.Error(), "missing required 'name' field") {
		t.Errorf("Expected 'missing required name field' error, got: %v", err)
	}
}

func TestAddHeadersPolicy_MultipleHeadersSameName_Request(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.RequestContext{
		Headers: createTestHeaders(map[string]string{}),
	}

	// Configuration with multiple headers having the same name
	// This tests that the policy properly accumulates values for AppendHeaders
	params := map[string]interface{}{
		"requestHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Custom-Header",
				"value": "value1",
			},
			map[string]interface{}{
				"name":  "X-Custom-Header", // Same header name
				"value": "value2",
			},
			map[string]interface{}{
				"name":  "X-Another-Header",
				"value": "another-value",
			},
		},
	}

	result := p.OnRequest(ctx, params)

	mods, ok := result.(policy.UpstreamRequestModifications)
	if !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}

	// Should have 2 unique header names
	if len(mods.AppendHeaders) != 2 {
		t.Errorf("Expected 2 unique headers in AppendHeaders, got %d headers", len(mods.AppendHeaders))
	}

	// Check that X-Custom-Header has both values accumulated
	if headerValues, exists := mods.AppendHeaders["x-custom-header"]; !exists {
		t.Errorf("Expected 'x-custom-header' to be present")
	} else if len(headerValues) != 2 {
		t.Errorf("Expected 'x-custom-header' to have 2 values, got %d values: %v", len(headerValues), headerValues)
	} else if headerValues[0] != "value1" || headerValues[1] != "value2" {
		t.Errorf("Expected 'x-custom-header' to have values ['value1', 'value2'], got %v", headerValues)
	}

	// Check that the other header is present with single value
	if headerValues, exists := mods.AppendHeaders["x-another-header"]; !exists {
		t.Errorf("Expected 'x-another-header' to be present")
	} else if len(headerValues) != 1 || headerValues[0] != "another-value" {
		t.Errorf("Expected 'x-another-header' to have value ['another-value'], got %v", headerValues)
	}
}

func TestAddHeadersPolicy_MultipleHeadersSameName_Response(t *testing.T) {
	p := &AddHeadersPolicy{}
	ctx := &policy.ResponseContext{
		ResponseHeaders: createTestHeaders(map[string]string{}),
	}

	// Configuration with multiple headers having the same name
	params := map[string]interface{}{
		"responseHeaders": []interface{}{
			map[string]interface{}{
				"name":  "Set-Cookie",
				"value": "sessionid=abc123",
			},
			map[string]interface{}{
				"name":  "Set-Cookie", // Same header name - common for Set-Cookie
				"value": "userid=xyz789",
			},
		},
	}

	result := p.OnResponse(ctx, params)

	mods, ok := result.(policy.UpstreamResponseModifications)
	if !ok {
		t.Errorf("Expected UpstreamResponseModifications, got %T", result)
	}

	// Should have 1 unique header name with multiple values
	if len(mods.AppendHeaders) != 1 {
		t.Errorf("Expected 1 unique header in AppendHeaders, got %d headers", len(mods.AppendHeaders))
	}

	// Check that Set-Cookie has both values accumulated
	if headerValues, exists := mods.AppendHeaders["set-cookie"]; !exists {
		t.Errorf("Expected 'set-cookie' to be present")
	} else if len(headerValues) != 2 {
		t.Errorf("Expected 'set-cookie' to have 2 values, got %d values: %v", len(headerValues), headerValues)
	} else if headerValues[0] != "sessionid=abc123" || headerValues[1] != "userid=xyz789" {
		t.Errorf("Expected 'set-cookie' to have values ['sessionid=abc123', 'userid=xyz789'], got %v", headerValues)
	}
}

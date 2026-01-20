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

package setheaders

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

func TestSetHeadersPolicy_Mode(t *testing.T) {
	p := &SetHeadersPolicy{}
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

	if _, ok := p.(*SetHeadersPolicy); !ok {
		t.Errorf("Expected SetHeadersPolicy, got %T", p)
	}
}

func TestSetHeadersPolicy_OnRequest_NoHeaders(t *testing.T) {
	p := &SetHeadersPolicy{}
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

	if len(mods.SetHeaders) != 0 {
		t.Errorf("Expected no headers to be set, got %d headers", len(mods.SetHeaders))
	}
}

func TestSetHeadersPolicy_OnRequest_SingleHeader(t *testing.T) {
	p := &SetHeadersPolicy{}
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

	if len(mods.SetHeaders) != 1 {
		t.Errorf("Expected 1 header to be set, got %d headers", len(mods.SetHeaders))
	}

	expectedHeaderName := "x-custom-header" // Should be normalized to lowercase
	if mods.SetHeaders[expectedHeaderName] != "custom-value" {
		t.Errorf("Expected header '%s' to have value 'custom-value', got '%s'",
			expectedHeaderName, mods.SetHeaders[expectedHeaderName])
	}
}

func TestSetHeadersPolicy_OnRequest_MultipleHeaders(t *testing.T) {
	p := &SetHeadersPolicy{}
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

	if len(mods.SetHeaders) != 3 {
		t.Errorf("Expected 3 headers to be set, got %d headers", len(mods.SetHeaders))
	}

	expectedHeaders := map[string]string{
		"x-api-key":        "secret-key-123",
		"x-client-version": "1.2.3",
		"x-request-id":     "req-456",
	}

	for name, expectedValue := range expectedHeaders {
		if actualValue := mods.SetHeaders[name]; actualValue != expectedValue {
			t.Errorf("Expected header '%s' to have value '%s', got '%s'",
				name, expectedValue, actualValue)
		}
	}
}

func TestSetHeadersPolicy_OnRequest_HeaderNameNormalization(t *testing.T) {
	p := &SetHeadersPolicy{}
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
	if mods.SetHeaders[expectedHeaderName] != "test-value" {
		t.Errorf("Expected header '%s' to be normalized and set, got headers: %v",
			expectedHeaderName, mods.SetHeaders)
	}
}

func TestSetHeadersPolicy_OnResponse_NoHeaders(t *testing.T) {
	p := &SetHeadersPolicy{}
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

	if len(mods.SetHeaders) != 0 {
		t.Errorf("Expected no headers to be set, got %d headers", len(mods.SetHeaders))
	}
}

func TestSetHeadersPolicy_OnResponse_SingleHeader(t *testing.T) {
	p := &SetHeadersPolicy{}
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

	if len(mods.SetHeaders) != 1 {
		t.Errorf("Expected 1 header to be set, got %d headers", len(mods.SetHeaders))
	}

	expectedHeaderName := "x-response-time" // Should be normalized to lowercase
	if mods.SetHeaders[expectedHeaderName] != "123ms" {
		t.Errorf("Expected header '%s' to have value '123ms', got '%s'",
			expectedHeaderName, mods.SetHeaders[expectedHeaderName])
	}
}

func TestSetHeadersPolicy_OnResponse_MultipleHeaders(t *testing.T) {
	p := &SetHeadersPolicy{}
	ctx := &policy.ResponseContext{
		ResponseHeaders: createTestHeaders(map[string]string{
			"content-type": "application/json",
		}),
	}

	params := map[string]interface{}{
		"responseHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Cache-Status",
				"value": "HIT",
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

	if len(mods.SetHeaders) != 3 {
		t.Errorf("Expected 3 headers to be set, got %d headers", len(mods.SetHeaders))
	}

	expectedHeaders := map[string]string{
		"x-cache-status":   "HIT",
		"x-server-version": "2.1.0",
		"x-content-hash":   "abc123def456",
	}

	for name, expectedValue := range expectedHeaders {
		if actualValue := mods.SetHeaders[name]; actualValue != expectedValue {
			t.Errorf("Expected header '%s' to have value '%s', got '%s'",
				name, expectedValue, actualValue)
		}
	}
}

func TestSetHeadersPolicy_BothRequestAndResponse(t *testing.T) {
	p := &SetHeadersPolicy{}

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

	if reqMods.SetHeaders["x-request-header"] != "request-value" {
		t.Errorf("Expected request header to be set")
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

	if respMods.SetHeaders["x-response-header"] != "response-value" {
		t.Errorf("Expected response header to be set")
	}
}

func TestSetHeadersPolicy_EmptyHeadersList(t *testing.T) {
	p := &SetHeadersPolicy{}
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

	if len(mods.SetHeaders) != 0 {
		t.Errorf("Expected no headers to be set for empty array, got %d headers", len(mods.SetHeaders))
	}
}

func TestSetHeadersPolicy_InvalidHeadersType(t *testing.T) {
	p := &SetHeadersPolicy{}
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

	if len(mods.SetHeaders) != 0 {
		t.Errorf("Expected no headers to be set for invalid type, got %d headers", len(mods.SetHeaders))
	}
}

func TestSetHeadersPolicy_InvalidHeaderEntry(t *testing.T) {
	p := &SetHeadersPolicy{}
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
	if len(mods.SetHeaders) != 1 {
		t.Errorf("Expected 1 valid header to be set, got %d headers", len(mods.SetHeaders))
	}

	if mods.SetHeaders["valid-header"] != "valid-value" {
		t.Errorf("Expected valid header to be processed correctly")
	}
}

func TestSetHeadersPolicy_SpecialCharactersInValues(t *testing.T) {
	p := &SetHeadersPolicy{}
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
	if mods.SetHeaders["x-special-chars"] != expectedValue {
		t.Errorf("Expected special characters to be preserved in header value, got '%s'",
			mods.SetHeaders["x-special-chars"])
	}
}

// Test the key difference: overwrite behavior when same header name appears multiple times
func TestSetHeadersPolicy_MultipleHeadersSameName_OverwriteBehavior(t *testing.T) {
	p := &SetHeadersPolicy{}
	ctx := &policy.RequestContext{
		Headers: createTestHeaders(map[string]string{}),
	}

	// Configuration with multiple headers having the same name
	// This tests that the policy overwrites (last value wins) instead of appending
	params := map[string]interface{}{
		"requestHeaders": []interface{}{
			map[string]interface{}{
				"name":  "X-Custom-Header",
				"value": "first-value",
			},
			map[string]interface{}{
				"name":  "X-Custom-Header", // Same header name - should overwrite
				"value": "second-value",
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

	// Should have 2 unique header names (last value wins for duplicates)
	if len(mods.SetHeaders) != 2 {
		t.Errorf("Expected 2 unique headers in SetHeaders, got %d headers", len(mods.SetHeaders))
	}

	// Check that the last value for X-Custom-Header is used (overwrite behavior)
	if mods.SetHeaders["x-custom-header"] != "second-value" {
		t.Errorf("Expected 'x-custom-header' to have last value 'second-value' (overwrite), got '%s'",
			mods.SetHeaders["x-custom-header"])
	}

	// Check that the other header is present with single value
	if mods.SetHeaders["x-another-header"] != "another-value" {
		t.Errorf("Expected 'x-another-header' to have value 'another-value', got '%s'",
			mods.SetHeaders["x-another-header"])
	}
}

// Test validation methods
func TestSetHeadersPolicy_Validate_ValidConfiguration(t *testing.T) {
	p := &SetHeadersPolicy{}

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

func TestSetHeadersPolicy_Validate_OnlyRequestHeaders(t *testing.T) {
	p := &SetHeadersPolicy{}

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

func TestSetHeadersPolicy_Validate_OnlyResponseHeaders(t *testing.T) {
	p := &SetHeadersPolicy{}

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

func TestSetHeadersPolicy_Validate_NoHeadersSpecified(t *testing.T) {
	p := &SetHeadersPolicy{}

	params := map[string]interface{}{}

	err := p.Validate(params)
	if err == nil || !strings.Contains(err.Error(), "at least one of 'requestHeaders' or 'responseHeaders' must be specified") {
		t.Errorf("Expected 'at least one must be specified' error, got: %v", err)
	}
}

func TestSetHeadersPolicy_Validate_InvalidRequestHeaders(t *testing.T) {
	p := &SetHeadersPolicy{}

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

func TestSetHeadersPolicy_Validate_InvalidResponseHeaders(t *testing.T) {
	p := &SetHeadersPolicy{}

	params := map[string]interface{}{
		"responseHeaders": "not-an-array", // Invalid type
	}

	err := p.Validate(params)
	if err == nil || !strings.Contains(err.Error(), "must be an array") {
		t.Errorf("Expected 'must be an array' error, got: %v", err)
	}
}

func TestSetHeadersPolicy_Validate_EmptyRequestHeaders(t *testing.T) {
	p := &SetHeadersPolicy{}

	params := map[string]interface{}{
		"requestHeaders": []interface{}{}, // Empty array
	}

	err := p.Validate(params)
	if err == nil || !strings.Contains(err.Error(), "requestHeaders cannot be empty") {
		t.Errorf("Expected 'requestHeaders cannot be empty' error, got: %v", err)
	}
}

func TestSetHeadersPolicy_Validate_BothInvalid(t *testing.T) {
	p := &SetHeadersPolicy{}

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

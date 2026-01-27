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

package analyticsheaderfilter

import (
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

func TestGetPolicy(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, nil)
	if err != nil {
		t.Errorf("GetPolicy returned error: %v", err)
	}
	if p == nil {
		t.Error("GetPolicy returned nil policy")
	}
	if _, ok := p.(*AnalyticsHeaderFilterPolicy); !ok {
		t.Error("GetPolicy returned wrong policy type")
	}
}

func TestMode(t *testing.T) {
	p := &AnalyticsHeaderFilterPolicy{}
	mode := p.Mode()
	
	if mode.RequestHeaderMode != policy.HeaderModeProcess {
		t.Errorf("Expected RequestHeaderMode to be HeaderModeProcess, got %v", mode.RequestHeaderMode)
	}
	if mode.RequestBodyMode != policy.BodyModeSkip {
		t.Errorf("Expected RequestBodyMode to be BodyModeSkip, got %v", mode.RequestBodyMode)
	}
	if mode.ResponseHeaderMode != policy.HeaderModeProcess {
		t.Errorf("Expected ResponseHeaderMode to be HeaderModeProcess, got %v", mode.ResponseHeaderMode)
	}
	if mode.ResponseBodyMode != policy.BodyModeSkip {
		t.Errorf("Expected ResponseBodyMode to be BodyModeSkip, got %v", mode.ResponseBodyMode)
	}
}

func TestParseHeaderList(t *testing.T) {
	p := &AnalyticsHeaderFilterPolicy{}

	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty array",
			input:    []interface{}{},
			expected: []string{},
		},
		{
			name:     "valid headers",
			input:    []interface{}{"Authorization", "Content-Type", "X-Custom-Header"},
			expected: []string{"authorization", "content-type", "x-custom-header"},
		},
		{
			name:     "headers with whitespace",
			input:    []interface{}{" Authorization ", "\tContent-Type\t", "\nX-Custom\n"},
			expected: []string{"authorization", "content-type", "x-custom"},
		},
		{
			name:     "mixed valid and invalid headers",
			input:    []interface{}{"Authorization", 123, "", "  ", "Content-Type"},
			expected: []string{"authorization", "content-type"},
		},
		{
			name:     "non-array input",
			input:    "not-an-array",
			expected: nil,
		},
		{
			name:     "array with non-string elements",
			input:    []interface{}{123, true, nil},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.parseHeaderList(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d headers, got %d", len(tt.expected), len(result))
				return
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected header[%d] to be %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}

func TestParseOperation(t *testing.T) {
	p := &AnalyticsHeaderFilterPolicy{}

	tests := []struct {
		name        string
		input       interface{}
		expected    string
		expectError bool
	}{
		{
			name:        "nil input",
			input:       nil,
			expected:    "",
			expectError: true,
		},
		{
			name:        "valid allow operation",
			input:       "allow",
			expected:    "allow",
			expectError: false,
		},
		{
			name:        "valid deny operation",
			input:       "deny",
			expected:    "deny",
			expectError: false,
		},
		{
			name:        "valid allow operation with case",
			input:       "ALLOW",
			expected:    "allow",
			expectError: false,
		},
		{
			name:        "valid deny operation with whitespace",
			input:       " deny ",
			expected:    "deny",
			expectError: false,
		},
		{
			name:        "invalid operation",
			input:       "invalid",
			expected:    "",
			expectError: true,
		},
		{
			name:        "non-string input",
			input:       123,
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "whitespace only",
			input:       "  ",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.parseOperation(tt.input)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestParseHeaderFilterConfig(t *testing.T) {
	p := &AnalyticsHeaderFilterPolicy{}

	tests := []struct {
		name            string
		input           interface{}
		expectedOp      string
		expectedHeaders []string
		expectError     bool
	}{
		{
			name:            "nil input",
			input:           nil,
			expectedOp:      "",
			expectedHeaders: nil,
			expectError:     false,
		},
		{
			name: "valid config with allow operation",
			input: map[string]interface{}{
				"operation": "allow",
				"headers":   []interface{}{"Authorization", "Content-Type"},
			},
			expectedOp:      "allow",
			expectedHeaders: []string{"authorization", "content-type"},
			expectError:     false,
		},
		{
			name: "valid config with deny operation",
			input: map[string]interface{}{
				"operation": "deny",
				"headers":   []interface{}{"X-Debug", "X-Internal"},
			},
			expectedOp:      "deny",
			expectedHeaders: []string{"x-debug", "x-internal"},
			expectError:     false,
		},
		{
			name: "config with empty headers array",
			input: map[string]interface{}{
				"operation": "allow",
				"headers":   []interface{}{},
			},
			expectedOp:      "allow",
			expectedHeaders: []string{},
			expectError:     false,
		},
		{
			name: "config without headers field",
			input: map[string]interface{}{
				"operation": "deny",
			},
			expectedOp:      "deny",
			expectedHeaders: nil,
			expectError:     false,
		},
		{
			name: "config without operation field",
			input: map[string]interface{}{
				"headers": []interface{}{"Authorization"},
			},
			expectedOp:      "",
			expectedHeaders: nil,
			expectError:     true,
		},
		{
			name: "config with null operation",
			input: map[string]interface{}{
				"operation": nil,
				"headers":   []interface{}{"Authorization"},
			},
			expectedOp:      "",
			expectedHeaders: nil,
			expectError:     true,
		},
		{
			name: "config with invalid operation",
			input: map[string]interface{}{
				"operation": "invalid",
				"headers":   []interface{}{"Authorization"},
			},
			expectedOp:      "",
			expectedHeaders: nil,
			expectError:     true,
		},
		{
			name:            "non-object input",
			input:           "not-an-object",
			expectedOp:      "",
			expectedHeaders: nil,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation, headers, err := p.parseHeaderFilterConfig(tt.input)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if operation != tt.expectedOp {
				t.Errorf("Expected operation %s, got %s", tt.expectedOp, operation)
			}
			if len(headers) != len(tt.expectedHeaders) {
				t.Errorf("Expected %d headers, got %d", len(tt.expectedHeaders), len(headers))
				return
			}
			for i, expected := range tt.expectedHeaders {
				if headers[i] != expected {
					t.Errorf("Expected header[%d] to be %s, got %s", i, expected, headers[i])
				}
			}
		})
	}
}

func TestOnRequest(t *testing.T) {
	p := &AnalyticsHeaderFilterPolicy{}

	tests := []struct {
		name                     string
		params                   map[string]interface{}
		expectedDropAction       *policy.DropHeaderAction
		expectDropActionPresent  bool
	}{
		{
			name:                    "no requestHeadersToFilter param",
			params:                  map[string]interface{}{},
			expectedDropAction:      nil,
			expectDropActionPresent: false,
		},
		{
			name: "nil requestHeadersToFilter param",
			params: map[string]interface{}{
				"requestHeadersToFilter": nil,
			},
			expectedDropAction:      nil,
			expectDropActionPresent: false,
		},
		{
			name: "valid requestHeadersToFilter with allow operation",
			params: map[string]interface{}{
				"requestHeadersToFilter": map[string]interface{}{
					"operation": "allow",
					"headers":   []interface{}{"Authorization", "Content-Type"},
				},
			},
			expectedDropAction: &policy.DropHeaderAction{
				Action:  "allow",
				Headers: []string{"authorization", "content-type"},
			},
			expectDropActionPresent: true,
		},
		{
			name: "valid requestHeadersToFilter with deny operation",
			params: map[string]interface{}{
				"requestHeadersToFilter": map[string]interface{}{
					"operation": "deny",
					"headers":   []interface{}{"X-Debug", "X-Internal"},
				},
			},
			expectedDropAction: &policy.DropHeaderAction{
				Action:  "deny",
				Headers: []string{"x-debug", "x-internal"},
			},
			expectDropActionPresent: true,
		},
		{
			name: "invalid requestHeadersToFilter config",
			params: map[string]interface{}{
				"requestHeadersToFilter": map[string]interface{}{
					"headers": []interface{}{"Authorization"},
					// missing operation
				},
			},
			expectedDropAction:      nil,
			expectDropActionPresent: false,
		},
		{
			name: "both requestHeadersToFilter and responseHeadersToFilter present",
			params: map[string]interface{}{
				"requestHeadersToFilter": map[string]interface{}{
					"operation": "allow",
					"headers":   []interface{}{"Authorization"},
				},
				"responseHeadersToFilter": map[string]interface{}{
					"operation": "deny",
					"headers":   []interface{}{"X-Debug"},
				},
			},
			expectedDropAction: &policy.DropHeaderAction{
				Action:  "allow",
				Headers: []string{"authorization"},
			},
			expectDropActionPresent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := createMockRequestContext(nil)
			result := p.OnRequest(ctx, tt.params)

			if modifications, ok := result.(policy.UpstreamRequestModifications); ok {
				if tt.expectDropActionPresent {
					if modifications.DropHeadersFromAnalytics.Action != tt.expectedDropAction.Action {
						t.Errorf("Expected action %s, got %s", tt.expectedDropAction.Action, modifications.DropHeadersFromAnalytics.Action)
					}
					if len(modifications.DropHeadersFromAnalytics.Headers) != len(tt.expectedDropAction.Headers) {
						t.Errorf("Expected %d headers, got %d", len(tt.expectedDropAction.Headers), len(modifications.DropHeadersFromAnalytics.Headers))
						return
					}
					for i, expected := range tt.expectedDropAction.Headers {
						if modifications.DropHeadersFromAnalytics.Headers[i] != expected {
							t.Errorf("Expected header[%d] to be %s, got %s", i, expected, modifications.DropHeadersFromAnalytics.Headers[i])
						}
					}
				} else {
					if modifications.DropHeadersFromAnalytics.Action != "" || len(modifications.DropHeadersFromAnalytics.Headers) > 0 {
						t.Error("Expected no drop action but got one")
					}
				}
			} else {
				t.Errorf("Expected UpstreamRequestModifications, got %T", result)
			}
		})
	}
}

func TestOnResponse(t *testing.T) {
	p := &AnalyticsHeaderFilterPolicy{}

	tests := []struct {
		name                     string
		params                   map[string]interface{}
		expectedDropAction       *policy.DropHeaderAction
		expectDropActionPresent  bool
	}{
		{
			name:                    "no responseHeadersToFilter param",
			params:                  map[string]interface{}{},
			expectedDropAction:      nil,
			expectDropActionPresent: false,
		},
		{
			name: "nil responseHeadersToFilter param",
			params: map[string]interface{}{
				"responseHeadersToFilter": nil,
			},
			expectedDropAction:      nil,
			expectDropActionPresent: false,
		},
		{
			name: "valid responseHeadersToFilter with allow operation",
			params: map[string]interface{}{
				"responseHeadersToFilter": map[string]interface{}{
					"operation": "allow",
					"headers":   []interface{}{"Content-Type", "X-Custom"},
				},
			},
			expectedDropAction: &policy.DropHeaderAction{
				Action:  "allow",
				Headers: []string{"content-type", "x-custom"},
			},
			expectDropActionPresent: true,
		},
		{
			name: "valid responseHeadersToFilter with deny operation",
			params: map[string]interface{}{
				"responseHeadersToFilter": map[string]interface{}{
					"operation": "deny",
					"headers":   []interface{}{"X-Debug", "X-Internal"},
				},
			},
			expectedDropAction: &policy.DropHeaderAction{
				Action:  "deny",
				Headers: []string{"x-debug", "x-internal"},
			},
			expectDropActionPresent: true,
		},
		{
			name: "invalid responseHeadersToFilter config",
			params: map[string]interface{}{
				"responseHeadersToFilter": map[string]interface{}{
					"headers": []interface{}{"Content-Type"},
					// missing operation
				},
			},
			expectedDropAction:      nil,
			expectDropActionPresent: false,
		},
		{
			name: "both requestHeadersToFilter and responseHeadersToFilter present",
			params: map[string]interface{}{
				"requestHeadersToFilter": map[string]interface{}{
					"operation": "allow",
					"headers":   []interface{}{"Authorization"},
				},
				"responseHeadersToFilter": map[string]interface{}{
					"operation": "deny",
					"headers":   []interface{}{"X-Debug"},
				},
			},
			expectedDropAction: &policy.DropHeaderAction{
				Action:  "deny",
				Headers: []string{"x-debug"},
			},
			expectDropActionPresent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := createMockResponseContext(nil, nil)
			result := p.OnResponse(ctx, tt.params)

			if modifications, ok := result.(policy.UpstreamResponseModifications); ok {
				if tt.expectDropActionPresent {
					if modifications.DropHeadersFromAnalytics.Action != tt.expectedDropAction.Action {
						t.Errorf("Expected action %s, got %s", tt.expectedDropAction.Action, modifications.DropHeadersFromAnalytics.Action)
					}
					if len(modifications.DropHeadersFromAnalytics.Headers) != len(tt.expectedDropAction.Headers) {
						t.Errorf("Expected %d headers, got %d", len(tt.expectedDropAction.Headers), len(modifications.DropHeadersFromAnalytics.Headers))
						return
					}
					for i, expected := range tt.expectedDropAction.Headers {
						if modifications.DropHeadersFromAnalytics.Headers[i] != expected {
							t.Errorf("Expected header[%d] to be %s, got %s", i, expected, modifications.DropHeadersFromAnalytics.Headers[i])
						}
					}
				} else {
					if modifications.DropHeadersFromAnalytics.Action != "" || len(modifications.DropHeadersFromAnalytics.Headers) > 0 {
						t.Error("Expected no drop action but got one")
					}
				}
			} else {
				t.Errorf("Expected UpstreamResponseModifications, got %T", result)
			}
		})
	}
}

// Helper functions for creating mock contexts
func createMockRequestContext(headers map[string][]string) *policy.RequestContext {
	return &policy.RequestContext{
		SharedContext: &policy.SharedContext{
			RequestID: "test-request-id",
			Metadata:  make(map[string]any),
		},
		Headers: policy.NewHeaders(headers),
		Body:    nil,
		Path:    "/api/test",
		Method:  "GET",
		Scheme:  "http",
	}
}

func createMockResponseContext(requestHeaders, responseHeaders map[string][]string) *policy.ResponseContext {
	return &policy.ResponseContext{
		SharedContext: &policy.SharedContext{
			RequestID: "test-request-id",
			Metadata:  make(map[string]any),
		},
		RequestHeaders:  policy.NewHeaders(requestHeaders),
		ResponseHeaders: policy.NewHeaders(responseHeaders),
		RequestBody:     nil,
		ResponseBody:    nil,
	}
}
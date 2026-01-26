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

package removequeryparameter

import (
	"reflect"
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

func TestRemoveQueryParameterPolicy_Mode(t *testing.T) {
	p := &RemoveQueryParameterPolicy{}
	mode := p.Mode()

	expected := policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}

	if mode != expected {
		t.Errorf("Expected mode %+v, but got %+v", expected, mode)
	}
}

func TestRemoveQueryParameterPolicy_OnRequest(t *testing.T) {
	p := &RemoveQueryParameterPolicy{}
	ctx := &policy.RequestContext{
		Path: "/api/test?param1=value1&param2=value2",
	}

	tests := []struct {
		name     string
		params   map[string]interface{}
		expected []string
	}{
		{
			name:     "No queryParameters configured",
			params:   map[string]interface{}{},
			expected: nil,
		},
		{
			name: "Invalid queryParameters type",
			params: map[string]interface{}{
				"queryParameters": "invalid",
			},
			expected: nil,
		},
		{
			name: "Empty queryParameters array",
			params: map[string]interface{}{
				"queryParameters": []interface{}{},
			},
			expected: nil,
		},
		{
			name: "Single query parameter to remove",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name": "debug",
					},
				},
			},
			expected: []string{"debug"},
		},
		{
			name: "Multiple different query parameters to remove",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name": "debug",
					},
					map[string]interface{}{
						"name": "internal_token",
					},
					map[string]interface{}{
						"name": "temp_param",
					},
				},
			},
			expected: []string{"debug", "internal_token", "temp_param"},
		},
		{
			name: "Duplicate parameter names",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name": "debug",
					},
					map[string]interface{}{
						"name": "internal_token",
					},
					map[string]interface{}{
						"name": "debug", // duplicate
					},
				},
			},
			expected: []string{"debug", "internal_token", "debug"},
		},
		{
			name: "Mixed valid and invalid parameters",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name": "debug",
					},
					map[string]interface{}{
						// Missing name
					},
					map[string]interface{}{
						"name": "", // Empty name
					},
					map[string]interface{}{
						"name": "internal_token",
					},
					"invalid_entry", // Not an object
				},
			},
			expected: []string{"debug", "internal_token"},
		},
		{
			name: "Parameters with special characters",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name": "param with spaces",
					},
					map[string]interface{}{
						"name": "param&special",
					},
					map[string]interface{}{
						"name": "param.dotted",
					},
				},
			},
			expected: []string{"param with spaces", "param&special", "param.dotted"},
		},
		{
			name: "Common parameter names to remove",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name": "api_key",
					},
					map[string]interface{}{
						"name": "session_id",
					},
					map[string]interface{}{
						"name": "csrf_token",
					},
					map[string]interface{}{
						"name": "cache_buster",
					},
				},
			},
			expected: []string{"api_key", "session_id", "csrf_token", "cache_buster"},
		},
		{
			name: "Case sensitive parameter names",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name": "Debug",
					},
					map[string]interface{}{
						"name": "DEBUG",
					},
					map[string]interface{}{
						"name": "debug",
					},
				},
			},
			expected: []string{"Debug", "DEBUG", "debug"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.OnRequest(ctx, tt.params)

			mods, ok := result.(policy.UpstreamRequestModifications)
			if !ok {
				t.Fatalf("Expected UpstreamRequestModifications, got %T", result)
			}

			if !reflect.DeepEqual(mods.RemoveQueryParameters, tt.expected) {
				t.Errorf("Expected RemoveQueryParameters %v, but got %v", tt.expected, mods.RemoveQueryParameters)
			}

			// Verify other fields are empty/nil
			if mods.SetHeaders != nil {
				t.Errorf("Expected SetHeaders to be nil, got %v", mods.SetHeaders)
			}
			if mods.RemoveHeaders != nil {
				t.Errorf("Expected RemoveHeaders to be nil, got %v", mods.RemoveHeaders)
			}
			if mods.AppendHeaders != nil {
				t.Errorf("Expected AppendHeaders to be nil, got %v", mods.AppendHeaders)
			}
			if mods.AddQueryParameters != nil {
				t.Errorf("Expected AddQueryParameters to be nil, got %v", mods.AddQueryParameters)
			}
			if mods.Body != nil {
				t.Errorf("Expected Body to be nil, got %v", mods.Body)
			}
			if mods.Path != nil {
				t.Errorf("Expected Path to be nil, got %v", mods.Path)
			}
			if mods.Method != nil {
				t.Errorf("Expected Method to be nil, got %v", mods.Method)
			}
		})
	}
}

func TestRemoveQueryParameterPolicy_OnResponse(t *testing.T) {
	p := &RemoveQueryParameterPolicy{}
	ctx := &policy.ResponseContext{}

	result := p.OnResponse(ctx, map[string]interface{}{})
	if result != nil {
		t.Errorf("Expected OnResponse to return nil, but got %v", result)
	}
}

func TestGetPolicy(t *testing.T) {
	metadata := policy.PolicyMetadata{
		RouteName: "test-route",
	}
	params := map[string]interface{}{}

	pol, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("Expected no error, but got %v", err)
	}

	if pol != ins {
		t.Errorf("Expected policy instance to be the singleton instance")
	}
}

// Integration tests that simulate real URL processing scenarios
func TestRemoveQueryParameterPolicy_Integration(t *testing.T) {
	p := &RemoveQueryParameterPolicy{}

	integrationTests := []struct {
		name           string
		params         map[string]interface{}
		inputURL       string
		expectedParams []string
		description    string
	}{
		{
			name: "Remove single debug parameter",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{"name": "debug"},
				},
			},
			inputURL:       "/api/search?q=test&debug=true&limit=10",
			expectedParams: []string{"debug"},
			description:    "Should remove debug parameter while preserving others",
		},
		{
			name: "Remove multiple security-related parameters",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{"name": "api_key"},
					map[string]interface{}{"name": "session_id"},
					map[string]interface{}{"name": "csrf_token"},
				},
			},
			inputURL:       "/api/user/profile?user_id=123&api_key=secret&session_id=abc123&csrf_token=xyz789&format=json",
			expectedParams: []string{"api_key", "session_id", "csrf_token"},
			description:    "Should remove all security-related parameters",
		},
		{
			name: "Remove tracking parameters",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{"name": "utm_source"},
					map[string]interface{}{"name": "utm_medium"},
					map[string]interface{}{"name": "utm_campaign"},
					map[string]interface{}{"name": "fbclid"},
					map[string]interface{}{"name": "gclid"},
				},
			},
			inputURL:       "/products/shoes?category=sports&utm_source=google&utm_medium=cpc&price_min=50&fbclid=abc123",
			expectedParams: []string{"utm_source", "utm_medium", "utm_campaign", "fbclid", "gclid"},
			description:    "Should remove marketing tracking parameters while preserving functional ones",
		},
		{
			name: "Remove parameters that don't exist in URL",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{"name": "nonexistent"},
					map[string]interface{}{"name": "also_missing"},
				},
			},
			inputURL:       "/api/data?existing=value",
			expectedParams: []string{"nonexistent", "also_missing"},
			description:    "Should still return parameters to remove even if they don't exist in URL",
		},
		{
			name: "Remove cache-related parameters",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{"name": "_"},         // cache buster
					map[string]interface{}{"name": "cache"},     // cache control
					map[string]interface{}{"name": "timestamp"}, // timestamp
					map[string]interface{}{"name": "v"},         // version
				},
			},
			inputURL:       "/api/assets/script.js?v=1.2.3&_=1642684800&cache=false&minify=true",
			expectedParams: []string{"_", "cache", "timestamp", "v"},
			description:    "Should remove cache-related parameters",
		},
	}

	for _, tt := range integrationTests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &policy.RequestContext{
				Path: tt.inputURL,
			}

			result := p.OnRequest(ctx, tt.params)
			mods, ok := result.(policy.UpstreamRequestModifications)
			if !ok {
				t.Fatalf("Expected UpstreamRequestModifications, got %T", result)
			}

			if !reflect.DeepEqual(mods.RemoveQueryParameters, tt.expectedParams) {
				t.Errorf("%s\nExpected RemoveQueryParameters %v, but got %v",
					tt.description, tt.expectedParams, mods.RemoveQueryParameters)
			}
		})
	}
}

// Benchmark tests
func BenchmarkRemoveQueryParameterPolicy_OnRequest_SingleParam(b *testing.B) {
	p := &RemoveQueryParameterPolicy{}
	ctx := &policy.RequestContext{Path: "/api/test?debug=true&param1=value1"}
	params := map[string]interface{}{
		"queryParameters": []interface{}{
			map[string]interface{}{"name": "debug"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.OnRequest(ctx, params)
	}
}

func BenchmarkRemoveQueryParameterPolicy_OnRequest_MultipleParams(b *testing.B) {
	p := &RemoveQueryParameterPolicy{}
	ctx := &policy.RequestContext{Path: "/api/test?debug=true&param1=value1&api_key=secret&session_id=abc"}
	params := map[string]interface{}{
		"queryParameters": []interface{}{
			map[string]interface{}{"name": "debug"},
			map[string]interface{}{"name": "api_key"},
			map[string]interface{}{"name": "session_id"},
			map[string]interface{}{"name": "csrf_token"},
			map[string]interface{}{"name": "internal_flag"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.OnRequest(ctx, params)
	}
}

func BenchmarkRemoveQueryParameterPolicy_OnRequest_NoParams(b *testing.B) {
	p := &RemoveQueryParameterPolicy{}
	ctx := &policy.RequestContext{Path: "/api/test"}
	params := map[string]interface{}{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.OnRequest(ctx, params)
	}
}

func BenchmarkRemoveQueryParameterPolicy_OnRequest_InvalidConfig(b *testing.B) {
	p := &RemoveQueryParameterPolicy{}
	ctx := &policy.RequestContext{Path: "/api/test"}
	params := map[string]interface{}{
		"queryParameters": "invalid_config",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.OnRequest(ctx, params)
	}
}

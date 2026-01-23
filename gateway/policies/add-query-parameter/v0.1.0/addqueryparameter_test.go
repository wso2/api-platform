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

package addqueryparameter

import (
	"reflect"
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

func TestAddQueryParameterPolicy_Mode(t *testing.T) {
	p := &AddQueryParameterPolicy{}
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

func TestAddQueryParameterPolicy_OnRequest(t *testing.T) {
	p := &AddQueryParameterPolicy{}
	ctx := &policy.RequestContext{
		Path: "/api/test",
	}

	tests := []struct {
		name     string
		params   map[string]interface{}
		expected policy.UpstreamRequestModifications
	}{
		{
			name:     "No queryParameters configured",
			params:   map[string]interface{}{},
			expected: policy.UpstreamRequestModifications{},
		},
		{
			name: "Invalid queryParameters type",
			params: map[string]interface{}{
				"queryParameters": "invalid",
			},
			expected: policy.UpstreamRequestModifications{},
		},
		{
			name: "Empty queryParameters array",
			params: map[string]interface{}{
				"queryParameters": []interface{}{},
			},
			expected: policy.UpstreamRequestModifications{},
		},
		{
			name: "Single query parameter",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name":  "api_key",
						"value": "12345",
					},
				},
			},
			expected: policy.UpstreamRequestModifications{
				AddQueryParameters: map[string][]string{
					"api_key": {"12345"},
				},
			},
		},
		{
			name: "Multiple different query parameters",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name":  "api_key",
						"value": "12345",
					},
					map[string]interface{}{
						"name":  "version",
						"value": "2.0",
					},
					map[string]interface{}{
						"name":  "source",
						"value": "gateway",
					},
				},
			},
			expected: policy.UpstreamRequestModifications{
				AddQueryParameters: map[string][]string{
					"api_key": {"12345"},
					"version": {"2.0"},
					"source":  {"gateway"},
				},
			},
		},
		{
			name: "Multiple values for same parameter name",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name":  "category",
						"value": "books",
					},
					map[string]interface{}{
						"name":  "category",
						"value": "electronics",
					},
					map[string]interface{}{
						"name":  "api_key",
						"value": "12345",
					},
				},
			},
			expected: policy.UpstreamRequestModifications{
				AddQueryParameters: map[string][]string{
					"category": {"books", "electronics"},
					"api_key":  {"12345"},
				},
			},
		},
		{
			name: "Mixed valid and invalid parameters",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name":  "api_key",
						"value": "12345",
					},
					map[string]interface{}{
						// Missing name
						"value": "invalid",
					},
					map[string]interface{}{
						"name": "version",
						// Missing value
					},
					map[string]interface{}{
						"name":  "",
						"value": "empty_name",
					},
					map[string]interface{}{
						"name":  "source",
						"value": "gateway",
					},
					"invalid_entry",
				},
			},
			expected: policy.UpstreamRequestModifications{
				AddQueryParameters: map[string][]string{
					"api_key": {"12345"},
					"source":  {"gateway"},
				},
			},
		},
		{
			name: "Empty values allowed",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name":  "empty_param",
						"value": "",
					},
					map[string]interface{}{
						"name":  "api_key",
						"value": "12345",
					},
				},
			},
			expected: policy.UpstreamRequestModifications{
				AddQueryParameters: map[string][]string{
					"empty_param": {""},
					"api_key":     {"12345"},
				},
			},
		},
		{
			name: "Special characters in parameter names and values",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name":  "param with spaces",
						"value": "value with spaces",
					},
					map[string]interface{}{
						"name":  "param&special",
						"value": "value&special=chars",
					},
				},
			},
			expected: policy.UpstreamRequestModifications{
				AddQueryParameters: map[string][]string{
					"param with spaces": {"value with spaces"},
					"param&special":     {"value&special=chars"},
				},
			},
		},
		{
			name: "Complex scenario with duplicates and special characters",
			params: map[string]interface{}{
				"queryParameters": []interface{}{
					map[string]interface{}{
						"name":  "filter",
						"value": "type:book",
					},
					map[string]interface{}{
						"name":  "filter",
						"value": "category:fiction",
					},
					map[string]interface{}{
						"name":  "api_key",
						"value": "abc123",
					},
					map[string]interface{}{
						"name":  "debug",
						"value": "",
					},
					map[string]interface{}{
						"name":  "filter",
						"value": "price:<100",
					},
				},
			},
			expected: policy.UpstreamRequestModifications{
				AddQueryParameters: map[string][]string{
					"filter":  {"type:book", "category:fiction", "price:<100"},
					"api_key": {"abc123"},
					"debug":   {""},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.OnRequest(ctx, tt.params)

			mods, ok := result.(policy.UpstreamRequestModifications)
			if !ok {
				t.Fatalf("Expected UpstreamRequestModifications, got %T", result)
			}

			if !reflect.DeepEqual(mods.AddQueryParameters, tt.expected.AddQueryParameters) {
				t.Errorf("Expected AddQueryParameters %v, but got %v", tt.expected.AddQueryParameters, mods.AddQueryParameters)
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
			if mods.RemoveQueryParameters != nil {
				t.Errorf("Expected RemoveQueryParameters to be nil, got %v", mods.RemoveQueryParameters)
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

func TestAddQueryParameterPolicy_OnResponse(t *testing.T) {
	p := &AddQueryParameterPolicy{}
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

// Benchmark tests
func BenchmarkAddQueryParameterPolicy_OnRequest_SingleParam(b *testing.B) {
	p := &AddQueryParameterPolicy{}
	ctx := &policy.RequestContext{Path: "/api/test"}
	params := map[string]interface{}{
		"queryParameters": []interface{}{
			map[string]interface{}{
				"name":  "api_key",
				"value": "12345",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.OnRequest(ctx, params)
	}
}

func BenchmarkAddQueryParameterPolicy_OnRequest_MultipleParams(b *testing.B) {
	p := &AddQueryParameterPolicy{}
	ctx := &policy.RequestContext{Path: "/api/test"}
	params := map[string]interface{}{
		"queryParameters": []interface{}{
			map[string]interface{}{"name": "api_key", "value": "12345"},
			map[string]interface{}{"name": "version", "value": "2.0"},
			map[string]interface{}{"name": "source", "value": "gateway"},
			map[string]interface{}{"name": "filter", "value": "type:book"},
			map[string]interface{}{"name": "filter", "value": "category:fiction"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.OnRequest(ctx, params)
	}
}

func BenchmarkAddQueryParameterPolicy_OnRequest_NoParams(b *testing.B) {
	p := &AddQueryParameterPolicy{}
	ctx := &policy.RequestContext{Path: "/api/test"}
	params := map[string]interface{}{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.OnRequest(ctx, params)
	}
}

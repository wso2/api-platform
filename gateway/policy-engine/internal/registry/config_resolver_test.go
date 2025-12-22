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

package registry

import (
	"reflect"
	"testing"
)

func TestConfigResolver_ResolveValue(t *testing.T) {
	config := map[string]interface{}{
		"jwtauth": map[string]interface{}{
			"allowedalgorithms": "RS256,RS384,RS512",
			"headerName":        "Authorization",
		},
		"ratelimit": map[string]interface{}{
			"maxrequests": 100,
		},
	}

	resolver, err := NewConfigResolver(config)
	if err != nil {
		t.Fatalf("NewConfigResolver() unexpected error: %v", err)
	}

	tests := []struct {
		name      string
		input     interface{}
		expected  interface{}
		expectErr bool
	}{
		{
			name:     "resolve config reference - dot notation",
			input:    "${config.jwtauth.allowedalgorithms}",
			expected: "RS256,RS384,RS512",
		},
		{
			name:     "resolve nested config reference",
			input:    "${config.jwtauth.headerName}",
			expected: "Authorization",
		},
		{
			name:     "resolve integer config",
			input:    "${config.ratelimit.maxrequests}",
			expected: int64(100), // CEL returns int64
		},
		{
			name:     "resolve config reference - bracket notation",
			input:    "${config[\"jwtauth\"][\"allowedalgorithms\"]}",
			expected: "RS256,RS384,RS512",
		},
		{
			name:     "non-config string unchanged",
			input:    "Bearer",
			expected: "Bearer",
		},
		{
			name:     "integer unchanged",
			input:    401,
			expected: 401,
		},
		{
			name:      "invalid config reference returns error",
			input:     "${config.nonexistent.path}",
			expectErr: true,
		},
		{
			name:     "malformed reference unchanged (no ${} pattern match)",
			input:    "${config.incomplete",
			expected: "${config.incomplete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolver.ResolveValue(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Errorf("ResolveValue() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveValue() unexpected error: %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ResolveValue() = %v (%T), want %v (%T)", got, got, tt.expected, tt.expected)
			}
		})
	}
}

func TestConfigResolver_ResolveMap(t *testing.T) {
	config := map[string]interface{}{
		"jwtauth": map[string]interface{}{
			"allowedalgorithms": "RS256,RS384",
			"headername":        "Authorization",
		},
	}

	resolver, err := NewConfigResolver(config)
	if err != nil {
		t.Fatalf("NewConfigResolver() unexpected error: %v", err)
	}

	input := map[string]interface{}{
		"allowedAlgorithms": "${config.jwtauth.allowedalgorithms}",
		"headerName":        "${config.jwtauth.headername}",
		"statusCode":        401,
		"realm":             "Restricted",
	}

	expected := map[string]interface{}{
		"allowedAlgorithms": "RS256,RS384",
		"headerName":        "Authorization",
		"statusCode":        401, // Not a config ref, stays as original int
		"realm":             "Restricted",
	}

	got, err := resolver.ResolveMap(input)
	if err != nil {
		t.Errorf("ResolveMap() unexpected error: %v", err)
		return
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("ResolveMap() = %v, want %v", got, expected)
	}
}

func TestConfigResolver_ResolveNestedStructures(t *testing.T) {
	config := map[string]interface{}{
		"jwtauth": map[string]interface{}{
			"allowedalgorithms": []interface{}{"RS256", "RS384"},
		},
	}

	resolver, err := NewConfigResolver(config)
	if err != nil {
		t.Fatalf("NewConfigResolver() unexpected error: %v", err)
	}

	input := map[string]interface{}{
		"nested": map[string]interface{}{
			"algorithms": "${config.jwtauth.allowedalgorithms}",
		},
		"array": []interface{}{
			"${config.jwtauth.allowedalgorithms}",
			"plain-value",
		},
	}

	got, err := resolver.ResolveMap(input)
	if err != nil {
		t.Errorf("ResolveMap() unexpected error: %v", err)
		return
	}

	// Check nested map resolution
	nestedMap := got["nested"].(map[string]interface{})
	algorithms := nestedMap["algorithms"]
	expected := []interface{}{"RS256", "RS384"}
	if !reflect.DeepEqual(algorithms, expected) {
		t.Errorf("Nested map resolution failed: got %v (%T), want %v (%T)", algorithms, algorithms, expected, expected)
	}

	// Check array resolution
	arrayVal := got["array"].([]interface{})
	if !reflect.DeepEqual(arrayVal[0], expected) {
		t.Errorf("Array resolution failed: got %v (%T), want %v (%T)", arrayVal[0], arrayVal[0], expected, expected)
	}
	if arrayVal[1] != "plain-value" {
		t.Errorf("Array plain value changed: got %v", arrayVal[1])
	}
}

func TestConfigResolver_NilConfig(t *testing.T) {
	resolver, err := NewConfigResolver(nil)
	if err != nil {
		t.Fatalf("NewConfigResolver() unexpected error: %v", err)
	}

	input := "${config.some.path}"
	got, err := resolver.ResolveValue(input)
	if err != nil {
		t.Errorf("ResolveValue() unexpected error: %v", err)
		return
	}

	// With nil config, should return input unchanged
	if got != input {
		t.Errorf("Nil config should return input unchanged: got %v, want %v", got, input)
	}
}

func TestConfigResolver_CELComplexExpressions(t *testing.T) {
	config := map[string]interface{}{
		"jwtauth": map[string]interface{}{
			"enabled":           true,
			"allowedalgorithms": "RS256,RS384,RS512",
			"headerName":        "Authorization",
		},
		"ratelimit": map[string]interface{}{
			"maxrequests": 100,
		},
	}

	resolver, err := NewConfigResolver(config)
	if err != nil {
		t.Fatalf("NewConfigResolver() unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "CEL mixed notation",
			input:    "${config.jwtauth[\"headerName\"]}",
			expected: "Authorization",
		},
		{
			name:     "CEL boolean value",
			input:    "${config.jwtauth.enabled}",
			expected: true,
		},
		{
			name:     "CEL conditional expression",
			input:    "${config.jwtauth.enabled ? config.jwtauth.allowedalgorithms : \"HS256\"}",
			expected: "RS256,RS384,RS512",
		},
		{
			name:     "CEL conditional false branch",
			input:    "${!config.jwtauth.enabled ? config.jwtauth.allowedalgorithms : \"HS256\"}",
			expected: "HS256",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolver.ResolveValue(tt.input)
			if err != nil {
				t.Errorf("ResolveValue() unexpected error: %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ResolveValue() = %v (%T), want %v (%T)", got, got, tt.expected, tt.expected)
			}
		})
	}
}

func TestConfigResolver_CELArrayAccess(t *testing.T) {
	config := map[string]interface{}{
		"jwtauth": map[string]interface{}{
			"allowedalgorithms": []interface{}{"RS256", "RS384", "RS512"},
		},
	}

	resolver, err := NewConfigResolver(config)
	if err != nil {
		t.Fatalf("NewConfigResolver() unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "CEL array access - first element",
			input:    "${config.jwtauth.allowedalgorithms[0]}",
			expected: "RS256",
		},
		{
			name:     "CEL array access - second element",
			input:    "${config.jwtauth.allowedalgorithms[1]}",
			expected: "RS384",
		},
		{
			name:     "CEL entire array",
			input:    "${config.jwtauth.allowedalgorithms}",
			expected: []interface{}{"RS256", "RS384", "RS512"},
		},
		{
			name:     "CEL array size",
			input:    "${size(config.jwtauth.allowedalgorithms)}",
			expected: int64(3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolver.ResolveValue(tt.input)
			if err != nil {
				t.Errorf("ResolveValue() unexpected error: %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ResolveValue() = %v (%T), want %v (%T)", got, got, tt.expected, tt.expected)
			}
		})
	}
}

func TestConfigResolver_CELInvalidReference(t *testing.T) {
	config := map[string]interface{}{
		"jwtauth": map[string]interface{}{
			"allowedalgorithms": "RS256",
		},
	}

	resolver, err := NewConfigResolver(config)
	if err != nil {
		t.Fatalf("NewConfigResolver() unexpected error: %v", err)
	}

	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "CEL non-existent path",
			input:     "${config.nonexistent.path}",
			expectErr: true,
		},
		{
			name:      "CEL malformed expression - unclosed bracket",
			input:     "${config.jwtauth[}",
			expectErr: true,
		},
		{
			name:      "CEL malformed expression - missing closing brace",
			input:     "${config.jwtauth.allowedalgorithms",
			expectErr: false, // Doesn't match pattern, returns unchanged
		},
		{
			name:      "not a config reference",
			input:     "plain string value",
			expectErr: false, // Plain string, returns unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolver.ResolveValue(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Errorf("ResolveValue() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveValue() unexpected error: %v", err)
				return
			}
			// Should return original value when no pattern match or plain string
			if got != tt.input {
				t.Errorf("Invalid CEL reference should return original: got %v, want %v", got, tt.input)
			}
		})
	}
}

func TestConfigResolver_TemplateSubstitution(t *testing.T) {
	config := map[string]interface{}{
		"api": map[string]interface{}{
			"host":     "api.example.com",
			"port":     8080,
			"protocol": "https",
		},
		"timeout": 30,
		"service": map[string]interface{}{
			"name": "my-service",
		},
	}

	resolver, err := NewConfigResolver(config)
	if err != nil {
		t.Fatalf("NewConfigResolver() unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single expression in middle",
			input:    "The timeout is ${config.timeout} seconds",
			expected: "The timeout is 30 seconds",
		},
		{
			name:     "multiple expressions",
			input:    "${config.api.protocol}://${config.api.host}:${config.api.port}",
			expected: "https://api.example.com:8080",
		},
		{
			name:     "expression at start",
			input:    "${config.service.name} is running",
			expected: "my-service is running",
		},
		{
			name:     "expression at end",
			input:    "Service name: ${config.service.name}",
			expected: "Service name: my-service",
		},
		{
			name:     "same expression multiple times",
			input:    "${config.timeout}s timeout, retry after ${config.timeout}s",
			expected: "30s timeout, retry after 30s",
		},
		{
			name:     "complex template",
			input:    "Connect to ${config.api.protocol}://${config.api.host}:${config.api.port} (timeout: ${config.timeout}s)",
			expected: "Connect to https://api.example.com:8080 (timeout: 30s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolver.ResolveValue(tt.input)
			if err != nil {
				t.Errorf("ResolveValue() unexpected error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("ResolveValue() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfigResolver_SingleExpressionPreservesType(t *testing.T) {
	config := map[string]interface{}{
		"timeout":     30,
		"enabled":     true,
		"values":      []interface{}{"a", "b", "c"},
		"name":        "test",
		"ratelimit":   int64(100),
		"temperature": 98.6,
	}

	resolver, err := NewConfigResolver(config)
	if err != nil {
		t.Fatalf("NewConfigResolver() unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "integer type preserved",
			input:    "${config.timeout}",
			expected: int64(30),
		},
		{
			name:     "boolean type preserved",
			input:    "${config.enabled}",
			expected: true,
		},
		{
			name:     "array type preserved",
			input:    "${config.values}",
			expected: []interface{}{"a", "b", "c"},
		},
		{
			name:     "string type preserved",
			input:    "${config.name}",
			expected: "test",
		},
		{
			name:     "int64 type preserved",
			input:    "${config.ratelimit}",
			expected: int64(100),
		},
		{
			name:     "float type preserved",
			input:    "${config.temperature}",
			expected: 98.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolver.ResolveValue(tt.input)
			if err != nil {
				t.Errorf("ResolveValue() unexpected error: %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ResolveValue() = %v (%T), want %v (%T)", got, got, tt.expected, tt.expected)
			}
		})
	}
}

func TestConfigResolver_ObjectAndArrayTypes(t *testing.T) {
	config := map[string]interface{}{
		"database": map[string]interface{}{
			"host": "localhost",
			"port": 5432,
		},
		"allowedMethods": []interface{}{"GET", "POST", "PUT"},
		"nestedArray": []interface{}{
			map[string]interface{}{"id": 1, "name": "first"},
			map[string]interface{}{"id": 2, "name": "second"},
		},
	}

	resolver, err := NewConfigResolver(config)
	if err != nil {
		t.Fatalf("NewConfigResolver() unexpected error: %v", err)
	}

	t.Run("object type preserved", func(t *testing.T) {
		result, err := resolver.ResolveValue("${config.database}")
		if err != nil {
			t.Errorf("ResolveValue() unexpected error: %v", err)
			return
		}
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected map[string]interface{}, got %T", result)
		}
		if resultMap["host"] != "localhost" {
			t.Errorf("Expected host=localhost, got %v", resultMap["host"])
		}
		// Note: nested values preserve their original types from the config
		if resultMap["port"] != 5432 {
			t.Errorf("Expected port=5432, got %v (%T)", resultMap["port"], resultMap["port"])
		}
	})

	t.Run("array of strings type preserved", func(t *testing.T) {
		result, err := resolver.ResolveValue("${config.allowedMethods}")
		if err != nil {
			t.Errorf("ResolveValue() unexpected error: %v", err)
			return
		}
		resultArray, ok := result.([]interface{})
		if !ok {
			t.Fatalf("Expected []interface{}, got %T", result)
		}
		if len(resultArray) != 3 {
			t.Errorf("Expected array length 3, got %d", len(resultArray))
		}
		if resultArray[0] != "GET" {
			t.Errorf("Expected first element 'GET', got %v", resultArray[0])
		}
	})

	t.Run("array of objects type preserved", func(t *testing.T) {
		result, err := resolver.ResolveValue("${config.nestedArray}")
		if err != nil {
			t.Errorf("ResolveValue() unexpected error: %v", err)
			return
		}
		resultArray, ok := result.([]interface{})
		if !ok {
			t.Fatalf("Expected []interface{}, got %T", result)
		}
		if len(resultArray) != 2 {
			t.Errorf("Expected array length 2, got %d", len(resultArray))
		}
		firstItem, ok := resultArray[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected first item to be map[string]interface{}, got %T", resultArray[0])
		}
		if firstItem["name"] != "first" {
			t.Errorf("Expected name='first', got %v", firstItem["name"])
		}
	})
}

func TestConfigResolver_TemplateWithErrors(t *testing.T) {
	config := map[string]interface{}{
		"api": map[string]interface{}{
			"host": "api.example.com",
		},
	}

	resolver, err := NewConfigResolver(config)
	if err != nil {
		t.Fatalf("NewConfigResolver() unexpected error: %v", err)
	}

	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "partial resolution - one valid, one invalid - should error",
			input:     "Host: ${config.api.host}, Port: ${config.api.port}",
			expectErr: true,
		},
		{
			name:      "all invalid expressions should error",
			input:     "${config.invalid.a} and ${config.invalid.b}",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.ResolveValue(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Errorf("ResolveValue() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveValue() unexpected error: %v", err)
			}
		})
	}
}

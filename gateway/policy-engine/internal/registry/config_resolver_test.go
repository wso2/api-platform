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

	resolver := NewConfigResolver(config)

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "resolve config reference",
			input:    "$config(jwtauth.allowedalgorithms)",
			expected: "RS256,RS384,RS512",
		},
		{
			name:     "resolve nested config reference",
			input:    "$config(jwtauth.headerName)",
			expected: "Authorization",
		},
		{
			name:     "resolve integer config",
			input:    "$config(ratelimit.maxrequests)",
			expected: 100,
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
			name:     "invalid config reference unchanged",
			input:    "$config(nonexistent.path)",
			expected: "$config(nonexistent.path)",
		},
		{
			name:     "malformed reference unchanged",
			input:    "$config(incomplete",
			expected: "$config(incomplete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolver.ResolveValue(tt.input)
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

	resolver := NewConfigResolver(config)

	input := map[string]interface{}{
		"allowedAlgorithms": "$config(jwtauth.allowedalgorithms)",
		"headerName":        "$config(jwtauth.headername)",
		"statusCode":        401,
		"realm":             "Restricted",
	}

	expected := map[string]interface{}{
		"allowedAlgorithms": "RS256,RS384",
		"headerName":        "Authorization",
		"statusCode":        401,
		"realm":             "Restricted",
	}

	got := resolver.ResolveMap(input)

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("ResolveMap() = %v, want %v", got, expected)
	}
}

func TestConfigResolver_ResolveNestedStructures(t *testing.T) {
	config := map[string]interface{}{
		"jwtauth": map[string]interface{}{
			"allowedalgorithms": []string{"RS256", "RS384"},
		},
	}

	resolver := NewConfigResolver(config)

	input := map[string]interface{}{
		"nested": map[string]interface{}{
			"algorithms": "$config(jwtauth.allowedalgorithms)",
		},
		"array": []interface{}{
			"$config(jwtauth.allowedalgorithms)",
			"plain-value",
		},
	}

	got := resolver.ResolveMap(input)

	// Check nested map resolution
	nestedMap := got["nested"].(map[string]interface{})
	algorithms := nestedMap["algorithms"]
	if !reflect.DeepEqual(algorithms, []string{"RS256", "RS384"}) {
		t.Errorf("Nested map resolution failed: got %v", algorithms)
	}

	// Check array resolution
	arrayVal := got["array"].([]interface{})
	if !reflect.DeepEqual(arrayVal[0], []string{"RS256", "RS384"}) {
		t.Errorf("Array resolution failed: got %v", arrayVal[0])
	}
	if arrayVal[1] != "plain-value" {
		t.Errorf("Array plain value changed: got %v", arrayVal[1])
	}
}

func TestConfigResolver_NilConfig(t *testing.T) {
	resolver := NewConfigResolver(nil)

	input := "$config(some.path)"
	got := resolver.ResolveValue(input)

	if got != input {
		t.Errorf("Nil config should return input unchanged: got %v, want %v", got, input)
	}
}

func TestConfigResolver_ResolvePath_CaseInsensitive(t *testing.T) {
	// Viper lowercases all keys
	config := map[string]interface{}{
		"jwtauth": map[string]interface{}{
			"allowedalgorithms": "RS256",
		},
	}

	resolver := NewConfigResolver(config)

	// Test with original case
	tests := []string{
		"$config(JWTAuth.AllowedAlgorithms)",
		"$config(jwtauth.allowedalgorithms)",
		"$config(JWTAuth.allowedalgorithms)",
	}

	for _, input := range tests {
		got := resolver.ResolveValue(input)
		if got != "RS256" {
			t.Errorf("Case-insensitive resolution failed for %s: got %v, want RS256", input, got)
		}
	}
}

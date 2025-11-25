package policyv1alpha

import (
	"testing"
)

func TestHeaders_Get(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string][]string
		key      string
		expected []string
	}{
		{
			name: "get existing header",
			headers: map[string][]string{
				"content-type": {"application/json"},
			},
			key:      "content-type",
			expected: []string{"application/json"},
		},
		{
			name: "get case-insensitive",
			headers: map[string][]string{
				"content-type": {"application/json"},
			},
			key:      "Content-Type",
			expected: []string{"application/json"},
		},
		{
			name: "get multi-value header",
			headers: map[string][]string{
				"set-cookie": {"session=abc123", "user=john"},
			},
			key:      "set-cookie",
			expected: []string{"session=abc123", "user=john"},
		},
		{
			name: "get non-existent header",
			headers: map[string][]string{
				"content-type": {"application/json"},
			},
			key:      "authorization",
			expected: nil,
		},
		{
			name:     "get from nil headers",
			headers:  nil,
			key:      "content-type",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHeaders(tt.headers)
			result := h.Get(tt.key)

			if !sliceEqual(result, tt.expected) {
				t.Errorf("Get(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestHeaders_Get_ReturnsDefensiveCopy(t *testing.T) {
	// Test that modifying the returned slice doesn't affect the original
	original := map[string][]string{
		"x-custom": {"value1", "value2"},
	}
	h := NewHeaders(original)

	// Get the header
	result := h.Get("x-custom")

	// Modify the returned slice
	result[0] = "modified"
	result = append(result, "added")

	// Get the header again
	result2 := h.Get("x-custom")

	// Verify original is unchanged
	if result2[0] != "value1" {
		t.Errorf("Original header was modified! Got %v, want %v", result2, []string{"value1", "value2"})
	}
	if len(result2) != 2 {
		t.Errorf("Original header length changed! Got %d, want 2", len(result2))
	}
}

func TestHeaders_Has(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string][]string
		key      string
		expected bool
	}{
		{
			name: "has existing header",
			headers: map[string][]string{
				"content-type": {"application/json"},
			},
			key:      "content-type",
			expected: true,
		},
		{
			name: "has case-insensitive",
			headers: map[string][]string{
				"content-type": {"application/json"},
			},
			key:      "Content-Type",
			expected: true,
		},
		{
			name: "has non-existent header",
			headers: map[string][]string{
				"content-type": {"application/json"},
			},
			key:      "authorization",
			expected: false,
		},
		{
			name:     "has from nil headers",
			headers:  nil,
			key:      "content-type",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHeaders(tt.headers)
			result := h.Has(tt.key)

			if result != tt.expected {
				t.Errorf("Has(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestHeaders_GetAll(t *testing.T) {
	original := map[string][]string{
		"content-type": {"application/json"},
		"x-custom":     {"value1", "value2"},
	}
	h := NewHeaders(original)

	result := h.GetAll()

	// Verify all headers are present
	if len(result) != 2 {
		t.Errorf("GetAll() returned %d headers, want 2", len(result))
	}

	// Verify content matches
	if !sliceEqual(result["content-type"], []string{"application/json"}) {
		t.Errorf("GetAll()[content-type] = %v, want %v", result["content-type"], []string{"application/json"})
	}
	if !sliceEqual(result["x-custom"], []string{"value1", "value2"}) {
		t.Errorf("GetAll()[x-custom] = %v, want %v", result["x-custom"], []string{"value1", "value2"})
	}
}

func TestHeaders_GetAll_ReturnsDefensiveCopy(t *testing.T) {
	// Test that modifying the returned map doesn't affect the original
	original := map[string][]string{
		"x-custom": {"value1", "value2"},
	}
	h := NewHeaders(original)

	// Get all headers
	result := h.GetAll()

	// Modify the returned map and slices
	result["x-custom"][0] = "modified"
	result["x-custom"] = append(result["x-custom"], "added")
	result["new-header"] = []string{"new-value"}

	// Get all headers again
	result2 := h.GetAll()

	// Verify original is unchanged
	if len(result2) != 1 {
		t.Errorf("Original headers map was modified! Got %d headers, want 1", len(result2))
	}
	if result2["x-custom"][0] != "value1" {
		t.Errorf("Original header was modified! Got %v, want %v", result2["x-custom"], []string{"value1", "value2"})
	}
	if len(result2["x-custom"]) != 2 {
		t.Errorf("Original header length changed! Got %d, want 2", len(result2["x-custom"]))
	}
}

func TestHeaders_Iterate(t *testing.T) {
	original := map[string][]string{
		"content-type": {"application/json"},
		"x-custom":     {"value1", "value2"},
	}
	h := NewHeaders(original)

	// Collect iterated headers
	collected := make(map[string][]string)
	h.Iterate(func(name string, values []string) {
		collected[name] = values
	})

	// Verify all headers were iterated
	if len(collected) != 2 {
		t.Errorf("Iterate() collected %d headers, want 2", len(collected))
	}

	// Verify content matches
	if !sliceEqual(collected["content-type"], []string{"application/json"}) {
		t.Errorf("Iterate()[content-type] = %v, want %v", collected["content-type"], []string{"application/json"})
	}
	if !sliceEqual(collected["x-custom"], []string{"value1", "value2"}) {
		t.Errorf("Iterate()[x-custom] = %v, want %v", collected["x-custom"], []string{"value1", "value2"})
	}
}

func TestHeaders_Iterate_PassesDefensiveCopy(t *testing.T) {
	// Test that modifying values in iteration callback doesn't affect original
	original := map[string][]string{
		"x-custom": {"value1", "value2"},
	}
	h := NewHeaders(original)

	// Iterate and try to modify
	h.Iterate(func(name string, values []string) {
		values[0] = "modified"
		values = append(values, "added")
	})

	// Get the header to verify it's unchanged
	result := h.Get("x-custom")
	if result[0] != "value1" {
		t.Errorf("Original header was modified during iteration! Got %v, want %v", result, []string{"value1", "value2"})
	}
	if len(result) != 2 {
		t.Errorf("Original header length changed during iteration! Got %d, want 2", len(result))
	}
}

func TestHeaders_UnsafeInternalValues(t *testing.T) {
	original := map[string][]string{
		"content-type": {"application/json"},
		"x-custom":     {"value1", "value2"},
	}
	h := NewHeaders(original)

	// Get internal values (kernel-only API)
	internal := h.UnsafeInternalValues()

	// Verify we get the actual underlying map (not a copy)
	if len(internal) != 2 {
		t.Errorf("UnsafeInternalValues() returned %d headers, want 2", len(internal))
	}

	// Modify the internal map
	internal["content-type"][0] = "text/plain"

	// Verify the modification affected the Headers instance
	result := h.Get("content-type")
	if result[0] != "text/plain" {
		t.Errorf("UnsafeInternalValues() didn't return direct access. Got %v, want [text/plain]", result)
	}
}

func TestNewHeaders_WithNilMap(t *testing.T) {
	h := NewHeaders(nil)

	// Should create empty map, not panic
	result := h.GetAll()
	if result == nil || len(result) != 0 {
		t.Errorf("NewHeaders(nil).GetAll() = %v, want empty map", result)
	}

	// Should handle operations gracefully
	if h.Has("any-header") {
		t.Error("NewHeaders(nil).Has() should return false")
	}

	if got := h.Get("any-header"); got != nil {
		t.Errorf("NewHeaders(nil).Get() = %v, want nil", got)
	}
}

// Helper function to compare slices
func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

package policyv1alpha

import "strings"

// Headers provides read-only access to HTTP headers for policies.
// The underlying data is managed by the policy engine kernel.
//
// Policies should use Get(), Has(), and Iterate() methods for read-only access.
// Direct mutation is not allowed to maintain policy isolation guarantees.
type Headers struct {
	values map[string][]string
}

// ============================================
// PUBLIC API - Safe for policies to use
// ============================================

// Get retrieves all values for a header name (case-insensitive).
// Returns a defensive copy to prevent modification of the underlying data.
//
// Returns nil if the header does not exist.
//
// Example:
//
//	apiKeys := ctx.Headers.Get("x-api-key")
//	if len(apiKeys) > 0 {
//	    key := apiKeys[0]
//	}
func (h *Headers) Get(name string) []string {
	if h == nil || h.values == nil {
		return nil
	}
	vals := h.values[strings.ToLower(name)]
	if vals == nil {
		return nil
	}
	// Return defensive copy to prevent modification
	return append([]string(nil), vals...)
}

// Has checks if a header exists (case-insensitive).
//
// Example:
//
//	if ctx.Headers.Has("authorization") {
//	    // Process authorization header
//	}
func (h *Headers) Has(name string) bool {
	if h == nil || h.values == nil {
		return false
	}
	_, exists := h.values[strings.ToLower(name)]
	return exists
}

// GetAll returns a defensive copy of all headers.
// Useful for inspection but not recommended for iteration (use Iterate instead).
//
// The returned map and slices are copies, so modifications won't affect
// the underlying header data.
//
// Example:
//
//	allHeaders := ctx.Headers.GetAll()
//	for name, values := range allHeaders {
//	    fmt.Printf("%s: %v\n", name, values)
//	}
func (h *Headers) GetAll() map[string][]string {
	if h == nil || h.values == nil {
		return make(map[string][]string)
	}
	result := make(map[string][]string, len(h.values))
	for k, v := range h.values {
		result[k] = append([]string(nil), v...)
	}
	return result
}

// Iterate iterates over all headers with a callback function.
// Header values passed to the callback are defensive copies.
//
// Example:
//
//	ctx.Headers.Iterate(func(name string, values []string) {
//	    for _, value := range values {
//	        fmt.Printf("%s: %s\n", name, value)
//	    }
//	})
func (h *Headers) Iterate(fn func(name string, values []string)) {
	if h == nil || h.values == nil {
		return
	}
	for name, values := range h.values {
		// Pass defensive copies to prevent modification
		fn(name, append([]string(nil), values...))
	}
}

// ============================================
// INTERNAL API - For policy engine kernel ONLY
// DO NOT use these methods in policy implementations
// ============================================

// NewHeaders creates a new Headers instance from a map.
// For internal use by the policy engine kernel only.
//
// If values is nil, an empty map is created.
func NewHeaders(values map[string][]string) *Headers {
	if values == nil {
		values = make(map[string][]string)
	}
	return &Headers{values: values}
}

// UnsafeInternalValues returns direct mutable access to the underlying
// header map. This method is ONLY for the policy engine kernel.
//
// WARNING - UNSAFE - INTERNAL USE ONLY:
//   - Bypasses immutability guarantees
//   - Modifications affect all references to this Headers instance
//   - MUST only be called by policy engine kernel code (executor, translator)
//   - Policies MUST NEVER call this method
//
// Policies should use Get(), Has(), or Iterate() for read-only access.
//
// Improper use can lead to:
//   - Race conditions between policies
//   - Breaking policy isolation guarantees
//   - Undefined behavior in policy execution
//
// This method exists to allow the kernel to orchestrate header mutations
// while maintaining the policy contract of immutability.
func (h *Headers) UnsafeInternalValues() map[string][]string {
	if h == nil {
		return nil
	}
	return h.values
}

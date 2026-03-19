package policyv1alpha

import core "github.com/wso2/api-platform/sdk/core/policy"

// Headers provides read-only access to HTTP headers for policies.
// The underlying data is managed by the policy engine kernel.
//
// Policies should use Get(), Has(), and Iterate() methods for read-only access.
// Direct mutation is not allowed to maintain policy isolation guarantees.
type Headers = core.Headers

// NewHeaders creates a new Headers instance from a map.
// For internal use by the policy engine kernel only.
//
// If values is nil, an empty map is created.
func NewHeaders(values map[string][]string) *Headers {
	return core.NewHeaders(values)
}

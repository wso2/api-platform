package registry

import (
	"github.com/policy-engine/sdk/policy"
)

// PolicyChain is a container for a complete policy processing pipeline for a route
type PolicyChain struct {
	// Ordered list of policies to execute (all implement Policy interface)
	Policies []policy.Policy

	// Policy specifications (aligned with Policies)
	PolicySpecs []policy.PolicySpec

	// Shared metadata map for inter-policy communication
	// Initialized fresh for each request, persists through response phase
	// Key: string, Value: any (policy-specific data)
	Metadata map[string]interface{}

	// Computed flag: true if any policy requires request body access
	// Determines whether ext_proc uses SKIP or BUFFERED mode for request body
	RequiresRequestBody bool

	// Computed flag: true if any policy requires response body access
	// Determines whether ext_proc uses SKIP or BUFFERED mode for response body
	RequiresResponseBody bool
}

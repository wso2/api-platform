package uppercasebody

import (
	"strings"

	"github.com/policy-engine/sdk/policies"
)

// UppercaseBodyPolicy transforms request body text to uppercase
type UppercaseBodyPolicy struct{}

// NewPolicy creates a new UppercaseBodyPolicy instance
func NewPolicy() policies.Policy {
	return &UppercaseBodyPolicy{}
}

// Name returns the policy name
func (p *UppercaseBodyPolicy) Name() string {
	return "uppercaseBody"
}

// Validate validates the policy configuration
// This policy has no configuration parameters
func (p *UppercaseBodyPolicy) Validate(config map[string]interface{}) error {
	// No parameters to validate
	return nil
}

// ExecuteRequest transforms the request body to uppercase
func (p *UppercaseBodyPolicy) ExecuteRequest(ctx *policies.RequestContext, config map[string]interface{}) *policies.RequestPolicyAction {
	// Check if body is present
	if ctx.Body == nil || !ctx.Body.Present {
		// No body to transform, pass through
		return &policies.RequestPolicyAction{
			Action: policies.UpstreamRequestModifications{},
		}
	}

	// Transform body content to uppercase
	uppercasedBody := []byte(strings.ToUpper(string(ctx.Body.Content)))

	// Return modified body
	return &policies.RequestPolicyAction{
		Action: policies.UpstreamRequestModifications{
			Body: uppercasedBody,
		},
	}
}

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

// Mode returns the processing mode for this policy
func (p *UppercaseBodyPolicy) Mode() policies.ProcessingMode {
	return policies.ProcessingMode{
		RequestHeaderMode:  policies.HeaderModeSkip,   // Don't need request headers
		RequestBodyMode:    policies.BodyModeBuffer,   // Need full buffered request body
		ResponseHeaderMode: policies.HeaderModeSkip,   // Don't process response headers
		ResponseBodyMode:   policies.BodyModeSkip,     // Don't need response body
	}
}

// Validate validates the policy configuration
// This policy has no configuration parameters
func (p *UppercaseBodyPolicy) Validate(params map[string]interface{}) error {
	// No parameters to validate
	return nil
}

// OnRequest transforms the request body to uppercase
func (p *UppercaseBodyPolicy) OnRequest(ctx *policies.RequestContext, params map[string]interface{}) policies.RequestAction {
	// Check if body is present
	if ctx.Body == nil || !ctx.Body.Present {
		// No body to transform, pass through
		return policies.UpstreamRequestModifications{}
	}

	// Transform body content to uppercase
	uppercasedBody := []byte(strings.ToUpper(string(ctx.Body.Content)))

	// Return modified body
	return policies.UpstreamRequestModifications{
		Body: uppercasedBody,
	}
}

// OnResponse is not used by this policy (only modifies request body)
func (p *UppercaseBodyPolicy) OnResponse(ctx *policies.ResponseContext, params map[string]interface{}) policies.ResponseAction {
	return nil // No response processing needed
}

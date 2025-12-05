package uppercasebody

import (
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// UppercaseBodyPolicy transforms request body text to uppercase
type UppercaseBodyPolicy struct{}

// NewPolicy creates a new UppercaseBodyPolicy instance
func NewPolicy() policy.Policy {
	return &UppercaseBodyPolicy{}
}

// Mode returns the processing mode for this policy
func (p *UppercaseBodyPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip, // Don't need request headers
		RequestBodyMode:    policy.BodyModeBuffer, // Need full buffered request body
		ResponseHeaderMode: policy.HeaderModeSkip, // Don't process response headers
		ResponseBodyMode:   policy.BodyModeSkip,   // Don't need response body
	}
}

// Validate validates the policy configuration
// This policy has no configuration parameters
func (p *UppercaseBodyPolicy) Validate(params map[string]interface{}) error {
	// No parameters to validate
	return nil
}

// OnRequest transforms the request body to uppercase
func (p *UppercaseBodyPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Check if body is present
	if ctx.Body == nil || !ctx.Body.Present {
		// No body to transform, pass through
		return policy.UpstreamRequestModifications{}
	}

	// Transform body content to uppercase
	uppercasedBody := []byte(strings.ToUpper(string(ctx.Body.Content)))

	// Return modified body
	return policy.UpstreamRequestModifications{
		Body: uppercasedBody,
	}
}

// OnResponse is not used by this policy (only modifies request body)
func (p *UppercaseBodyPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return nil // No response processing needed
}

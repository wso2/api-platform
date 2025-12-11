package uppercasebody

import (
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// UppercaseBodyPolicy transforms request body text to uppercase
type UppercaseBodyPolicy struct{}

var ins = &UppercaseBodyPolicy{}

// NewPolicy creates a new BasicAuthPolicy instance
func NewPolicy(
	metadata policy.PolicyMetadata,
	initParams map[string]interface{},
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
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

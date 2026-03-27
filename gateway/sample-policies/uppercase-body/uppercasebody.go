package uppercasebody

import (
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// UppercaseBodyPolicy transforms request body text to uppercase
type UppercaseBodyPolicy struct{}

var ins = &UppercaseBodyPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	slog.Debug("[Uppercase Body]: GetPolicy called")
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

// OnRequestBody transforms the request body to uppercase
func (p *UppercaseBodyPolicy) OnRequestBody(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	slog.Debug("[Uppercase Body]: OnRequestBody called", "hasBody", ctx.Body != nil && ctx.Body.Present)

	// Check if body is present
	if ctx.Body == nil || !ctx.Body.Present {
		slog.Info("[Uppercase Body]: No request body present, passing through")
		// No body to transform, pass through
		return policy.UpstreamRequestModifications{}
	}

	originalSize := len(ctx.Body.Content)
	slog.Info("[Uppercase Body]: Transforming request body to uppercase", "originalSize", originalSize)

	// Transform body content to uppercase
	uppercasedBody := []byte(strings.ToUpper(string(ctx.Body.Content)))

	slog.Debug("[Uppercase Body]: Body transformation complete",
		"originalSize", originalSize,
		"transformedSize", len(uppercasedBody))

	// Return modified body
	return policy.UpstreamRequestModifications{
		Body: uppercasedBody,
	}
}


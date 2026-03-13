package uppercasebody

import (
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
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

// OnRequestBody transforms the request body to uppercase
func (p *UppercaseBodyPolicy) OnRequestBody(ctx *policy.RequestContext) policy.RequestAction {
	slog.Debug("[Uppercase Body]: OnRequestBody called", "hasBody", ctx.Body != nil && ctx.Body.Present)

	if ctx.Body == nil || !ctx.Body.Present {
		slog.Info("[Uppercase Body]: No request body present, passing through")
		return policy.UpstreamRequestModifications{}
	}

	originalSize := len(ctx.Body.Content)
	slog.Info("[Uppercase Body]: Transforming request body to uppercase", "originalSize", originalSize)

	uppercasedBody := []byte(strings.ToUpper(string(ctx.Body.Content)))

	slog.Debug("[Uppercase Body]: Body transformation complete",
		"originalSize", originalSize,
		"transformedSize", len(uppercasedBody))

	return policy.UpstreamRequestModifications{Body: uppercasedBody}
}

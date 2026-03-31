package transformpayloadcase

import (
	"context"
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// TransformPayloadCasePolicy transforms the request body to uppercase or lowercase
type TransformPayloadCasePolicy struct{}

var ins = &TransformPayloadCasePolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	slog.Debug("[Transform Payload Case]: GetPolicy called")
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *TransformPayloadCasePolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,   // Don't need request headers
		RequestBodyMode:    policy.BodyModeBuffer,    // Need full buffered request body
		ResponseHeaderMode: policy.HeaderModeSkip,    // Don't process response headers
		ResponseBodyMode:   policy.BodyModeSkip,      // Don't need response body
	}
}

// OnRequestBody transforms the request body case
func (p *TransformPayloadCasePolicy) OnRequestBody(_ context.Context, ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	slog.Debug("[Transform Payload Case]: OnRequest called", "hasBody", ctx.Body != nil && ctx.Body.Present)

	// Check if request body is present
	if ctx.Body == nil || !ctx.Body.Present {
		slog.Info("[Transform Payload Case]: No request body present, skipping")
		return nil
	}

	// Get target case parameter
	targetCase := "uppercase"
	if targetCaseRaw, ok := params["targetCase"]; ok {
		targetCase = strings.ToLower(targetCaseRaw.(string))
	}

	bodyText := string(ctx.Body.Content)
	slog.Info("[Transform Payload Case]: Transforming request body",
		"targetCase", targetCase,
		"bodySize", len(bodyText))

	// Transform the body case
	var transformedBody string
	if targetCase == "lowercase" {
		transformedBody = strings.ToLower(bodyText)
	} else {
		transformedBody = strings.ToUpper(bodyText)
	}

	slog.Debug("[Transform Payload Case]: Body transformed", "newSize", len(transformedBody))

	return policy.UpstreamRequestModifications{
		Body: []byte(transformedBody),
	}
}

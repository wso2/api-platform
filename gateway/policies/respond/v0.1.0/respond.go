package respond

import (
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// RespondPolicy implements immediate response functionality
// This policy terminates the request processing and returns an immediate response to the client
type RespondPolicy struct{}

var ins = &RespondPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *RespondPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Can use request headers for context
		RequestBodyMode:    policy.BodyModeSkip,      // Don't need request body
		ResponseHeaderMode: policy.HeaderModeSkip,    // Returns immediate response
		ResponseBodyMode:   policy.BodyModeSkip,      // Returns immediate response
	}
}

// OnRequest returns an immediate response to the client
func (p *RespondPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Extract statusCode (default to 200 OK)
	statusCode := 200
	if statusCodeRaw, ok := params["statusCode"]; ok {
		switch v := statusCodeRaw.(type) {
		case float64:
			statusCode = int(v)
		case int:
			statusCode = v
		}
	}

	// Extract body
	var body []byte
	if bodyRaw, ok := params["body"]; ok {
		switch v := bodyRaw.(type) {
		case string:
			body = []byte(v)
		case []byte:
			body = v
		}
	}

	// Extract headers
	headers := make(map[string]string)
	if headersRaw, ok := params["headers"]; ok {
		if headersList, ok := headersRaw.([]interface{}); ok {
			for _, headerRaw := range headersList {
				if headerMap, ok := headerRaw.(map[string]interface{}); ok {
					name := headerMap["name"].(string)
					value := headerMap["value"].(string)
					headers[name] = value
				}
			}
		}
	}

	// Return immediate response action
	return policy.ImmediateResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
	}
}

// OnResponse is not used by this policy (returns immediate response in request phase)
func (p *RespondPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return nil // No response processing needed
}

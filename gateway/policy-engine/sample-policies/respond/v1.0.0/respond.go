package respond

import (
	"fmt"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// RespondPolicy implements immediate response functionality
// This policy terminates the request processing and returns an immediate response to the client
type RespondPolicy struct{}

// NewPolicy creates a new RespondPolicy instance
func NewPolicy() policy.Policy {
	return &RespondPolicy{}
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

// Validate validates the policy configuration
func (p *RespondPolicy) Validate(params map[string]interface{}) error {
	// Validate statusCode parameter (optional, defaults to 200)
	if statusCodeRaw, ok := params["statusCode"]; ok {
		// Handle both float64 (from JSON) and int
		switch v := statusCodeRaw.(type) {
		case float64:
			statusCode := int(v)
			if statusCode < 100 || statusCode > 599 {
				return fmt.Errorf("statusCode must be between 100 and 599")
			}
		case int:
			if v < 100 || v > 599 {
				return fmt.Errorf("statusCode must be between 100 and 599")
			}
		default:
			return fmt.Errorf("statusCode must be a number")
		}
	}

	// Validate body parameter (optional)
	if bodyRaw, ok := params["body"]; ok {
		switch bodyRaw.(type) {
		case string:
			// Valid: string body
		case []byte:
			// Valid: byte array body
		default:
			return fmt.Errorf("body must be a string or byte array")
		}
	}

	// Validate headers parameter (optional)
	if headersRaw, ok := params["headers"]; ok {
		headers, ok := headersRaw.([]interface{})
		if !ok {
			return fmt.Errorf("headers must be an array")
		}

		for i, headerRaw := range headers {
			headerMap, ok := headerRaw.(map[string]interface{})
			if !ok {
				return fmt.Errorf("headers[%d] must be an object with 'name' and 'value' fields", i)
			}

			// Validate header name
			nameRaw, ok := headerMap["name"]
			if !ok {
				return fmt.Errorf("headers[%d] missing required 'name' field", i)
			}
			name, ok := nameRaw.(string)
			if !ok {
				return fmt.Errorf("headers[%d].name must be a string", i)
			}
			if len(name) == 0 {
				return fmt.Errorf("headers[%d].name cannot be empty", i)
			}

			// Validate header value
			valueRaw, ok := headerMap["value"]
			if !ok {
				return fmt.Errorf("headers[%d] missing required 'value' field", i)
			}
			_, ok = valueRaw.(string)
			if !ok {
				return fmt.Errorf("headers[%d].value must be a string", i)
			}
		}
	}

	return nil
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

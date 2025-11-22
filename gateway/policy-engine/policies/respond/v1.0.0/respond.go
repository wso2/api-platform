package respond

import (
	"fmt"

	"github.com/policy-engine/sdk/policies"
)

// RespondPolicy implements immediate response functionality
// This policy terminates the request processing and returns an immediate response to the client
type RespondPolicy struct{}

// NewPolicy creates a new RespondPolicy instance
func NewPolicy() policies.Policy {
	return &RespondPolicy{}
}

// Name returns the policy name
func (p *RespondPolicy) Name() string {
	return "respond"
}

// Validate validates the policy configuration
func (p *RespondPolicy) Validate(config map[string]interface{}) error {
	// Validate statusCode parameter (optional, defaults to 200)
	if statusCodeRaw, ok := config["statusCode"]; ok {
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
	if bodyRaw, ok := config["body"]; ok {
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
	if headersRaw, ok := config["headers"]; ok {
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

// ExecuteRequest returns an immediate response to the client
func (p *RespondPolicy) ExecuteRequest(ctx *policies.RequestContext, config map[string]interface{}) *policies.RequestPolicyAction {
	// Extract statusCode (default to 200 OK)
	statusCode := 200
	if statusCodeRaw, ok := config["statusCode"]; ok {
		switch v := statusCodeRaw.(type) {
		case float64:
			statusCode = int(v)
		case int:
			statusCode = v
		}
	}

	// Extract body
	var body []byte
	if bodyRaw, ok := config["body"]; ok {
		switch v := bodyRaw.(type) {
		case string:
			body = []byte(v)
		case []byte:
			body = v
		}
	}

	// Extract headers
	headers := make(map[string]string)
	if headersRaw, ok := config["headers"]; ok {
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
	return &policies.RequestPolicyAction{
		Action: policies.ImmediateResponse{
			StatusCode: statusCode,
			Headers:    headers,
			Body:       body,
		},
	}
}

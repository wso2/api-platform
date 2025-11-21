package apikey

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/policy-engine/sdk/policies"
)

// APIKeyPolicy validates API keys from request headers
type APIKeyPolicy struct{}

// Name returns the policy name
func (p *APIKeyPolicy) Name() string {
	return "APIKeyValidation"
}

// Validate validates the policy configuration parameters
func (p *APIKeyPolicy) Validate(config map[string]interface{}) error {
	// Validate headerName parameter
	headerName, ok := config["headerName"].(string)
	if !ok || headerName == "" {
		return fmt.Errorf("headerName parameter is required")
	}

	// Validate validKeys parameter
	validKeysParam, ok := config["validKeys"]
	if !ok {
		return fmt.Errorf("validKeys parameter is required")
	}

	validKeys, ok := validKeysParam.([]interface{})
	if !ok || len(validKeys) == 0 {
		return fmt.Errorf("validKeys must be a non-empty array")
	}

	// Validate each key
	for i, key := range validKeys {
		keyStr, ok := key.(string)
		if !ok {
			return fmt.Errorf("validKeys[%d] must be a string", i)
		}
		if len(keyStr) < 16 {
			return fmt.Errorf("validKeys[%d] must be at least 16 characters", i)
		}
	}

	return nil
}

// ExecuteRequest validates the API key in the request
func (p *APIKeyPolicy) ExecuteRequest(ctx *policies.RequestContext, config map[string]interface{}) *policies.RequestPolicyAction {
	// Get configuration parameters
	headerName := config["headerName"].(string)
	validKeysParam := config["validKeys"].([]interface{})

	caseSensitive := true
	if val, ok := config["caseSensitive"]; ok {
		if boolVal, ok := val.(bool); ok {
			caseSensitive = boolVal
		}
	}

	storeMetadata := true
	if val, ok := config["storeMetadata"]; ok {
		if boolVal, ok := val.(bool); ok {
			storeMetadata = boolVal
		}
	}

	unauthorizedMessage := "Unauthorized: Invalid or missing API key"
	if val, ok := config["unauthorizedMessage"]; ok {
		if strVal, ok := val.(string); ok && strVal != "" {
			unauthorizedMessage = strVal
		}
	}

	// Convert validKeys to string slice
	validKeys := make([]string, 0, len(validKeysParam))
	for _, key := range validKeysParam {
		if keyStr, ok := key.(string); ok {
			validKeys = append(validKeys, keyStr)
		}
	}

	// Extract API key from header
	apiKey := ""
	for headerKey, headerValues := range ctx.Headers {
		if strings.EqualFold(headerKey, headerName) && len(headerValues) > 0 {
			apiKey = headerValues[0]
			break
		}
	}

	// Check if API key is missing
	if apiKey == "" {
		slog.Warn("API key missing", "header", headerName, "path", ctx.Path)
		return &policies.RequestPolicyAction{
			Action: policies.ImmediateResponse{
				StatusCode: 401,
				Body:       []byte(unauthorizedMessage),
				Headers: map[string]string{
					"Content-Type":           "text/plain",
					"WWW-Authenticate":       fmt.Sprintf("API-Key realm=\"%s\"", headerName),
					"X-Policy-Rejection":     "APIKeyValidation",
					"X-Policy-Rejection-Reason": "Missing API key",
				},
			},
		}
	}

	// Validate API key
	valid := false
	for _, validKey := range validKeys {
		if caseSensitive {
			if apiKey == validKey {
				valid = true
				break
			}
		} else {
			if strings.EqualFold(apiKey, validKey) {
				valid = true
				break
			}
		}
	}

	if !valid {
		slog.Warn("Invalid API key", "header", headerName, "path", ctx.Path)
		return &policies.RequestPolicyAction{
			Action: policies.ImmediateResponse{
				StatusCode: 401,
				Body:       []byte(unauthorizedMessage),
				Headers: map[string]string{
					"Content-Type":           "text/plain",
					"WWW-Authenticate":       fmt.Sprintf("API-Key realm=\"%s\"", headerName),
					"X-Policy-Rejection":     "APIKeyValidation",
					"X-Policy-Rejection-Reason": "Invalid API key",
				},
			},
		}
	}

	// API key is valid
	slog.Info("API key validated", "header", headerName, "path", ctx.Path)

	// Store metadata if requested
	modifications := policies.UpstreamRequestModifications{
		SetHeaders:    make(map[string]string),
		RemoveHeaders: []string{},
	}

	if storeMetadata {
		// Store API key validation result in metadata
		ctx.Metadata["apikey.validated"] = "true"
		ctx.Metadata["apikey.header"] = headerName

		// Optionally add header for upstream service
		modifications.SetHeaders["X-API-Key-Validated"] = "true"
	}

	return &policies.RequestPolicyAction{
		Action: modifications,
	}
}

// NewPolicy creates a new instance of the API Key policy
func NewPolicy() policies.Policy {
	return &APIKeyPolicy{}
}

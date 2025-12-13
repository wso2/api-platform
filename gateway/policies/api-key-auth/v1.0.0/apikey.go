/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package apikey

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	// Metadata keys for context storage
	MetadataKeyAuthSuccess = "auth.success"
	MetadataKeyAuthMethod  = "auth.method"

	// Gateway controller configuration
	DefaultGatewayControllerBaseURL = "http://gateway-controller:9090/api/internal/v1"
	DefaultHTTPTimeout              = 5 * time.Second
)

// ApiKeyValidationRequest represents the request payload for API key validation
type ApiKeyValidationRequest struct {
	ApiKey string `json:"apiKey"`
}

// ApiKeyValidationResponse represents the response from API key validation
type ApiKeyValidationResponse struct {
	IsValid bool `json:"isValid"`
}

// ErrorResponse represents an error response from the gateway controller
type ErrorResponse struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// APIKeyPolicy implements API Key Authentication
type APIKeyPolicy struct {
	httpClient *http.Client
}

var ins = &APIKeyPolicy{
	httpClient: &http.Client{
		Timeout: DefaultHTTPTimeout,
	},
}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *APIKeyPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Process request headers for auth
		RequestBodyMode:    policy.BodyModeSkip,      // Don't need request body
		ResponseHeaderMode: policy.HeaderModeSkip,    // Don't process response headers
		ResponseBodyMode:   policy.BodyModeSkip,      // Don't need response body
	}
}

// OnRequest performs API Key Authentication
func (p *APIKeyPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Get configuration parameters
	keyName, ok := params["key"].(string)
	if !ok || keyName == "" {
		return p.handleAuthFailure(ctx, "missing or invalid 'key' configuration")
	}
	location, ok := params["in"].(string)
	if !ok || location == "" {
		return p.handleAuthFailure(ctx, "missing or invalid 'in' configuration")
	}

	var valuePrefix string
	if valuePrefixRaw, ok := params["value-prefix"]; ok {
		if vp, ok := valuePrefixRaw.(string); ok {
			valuePrefix = vp
		}
	}

	// Extract API key based on location
	var providedKey string

	if location == "header" {
		// Check header (case-insensitive)
		if headerValues := ctx.Headers.Get(strings.ToLower(keyName)); len(headerValues) > 0 {
			providedKey = headerValues[0]
		}
	} else if location == "query" {
		// Extract query parameters from the full path
		providedKey = extractQueryParam(ctx.Path, keyName)
	}

	// If no API key provided
	if providedKey == "" {
		return p.handleAuthFailure(ctx, "missing API key")
	}

	// Strip prefix if configured
	if valuePrefix != "" {
		providedKey = stripPrefix(providedKey, valuePrefix)
		// If after stripping prefix, the key is empty, treat as missing
		if providedKey == "" {
			return p.handleAuthFailure(ctx, "missing API key")
		}
	}

	apiName := ctx.APIName
	apiVersion := ctx.APIVersion

	if apiName == "" || apiVersion == "" {
		return p.handleAuthFailure(ctx, "API name or version not found")
	}

	// API key was provided - validate it using external validation
	isValid := p.validateAPIKey(apiName, apiVersion, providedKey)
	if !isValid {
		return p.handleAuthFailure(ctx, "invalid API key")
	}

	// Authentication successful
	return p.handleAuthSuccess(ctx)
}

// handleAuthSuccess handles successful authentication
func (p *APIKeyPolicy) handleAuthSuccess(ctx *policy.RequestContext) policy.RequestAction {
	// Set metadata indicating successful authentication
	ctx.Metadata[MetadataKeyAuthSuccess] = true
	ctx.Metadata[MetadataKeyAuthMethod] = "api-key"

	// Continue to upstream with no modifications
	return policy.UpstreamRequestModifications{}
}

// OnResponse is not used by this policy (authentication is request-only)
func (p *APIKeyPolicy) OnResponse(_ctx *policy.ResponseContext, _params map[string]interface{}) policy.ResponseAction {
	return nil // No response processing needed
}

// handleAuthFailure handles authentication failure
func (p *APIKeyPolicy) handleAuthFailure(ctx *policy.RequestContext, reason string) policy.RequestAction {
	// Set metadata indicating failed authentication
	ctx.Metadata[MetadataKeyAuthSuccess] = false
	ctx.Metadata[MetadataKeyAuthMethod] = "api-key"

	// Return 401 Unauthorized response
	headers := map[string]string{
		"content-type": "application/json",
	}

	body := fmt.Sprintf(`{"error": "Unauthorized", "message": "Valid API key required - %s"}`, reason)

	return policy.ImmediateResponse{
		StatusCode: 401,
		Headers:    headers,
		Body:       []byte(body),
	}
}

// validateAPIKey validates the provided API key against external store/service
func (p *APIKeyPolicy) validateAPIKey(apiName, apiVersion, apiKey string) bool {
	prefix := extractAPIKeyPrefix(apiKey)

	switch prefix {
	case "gw":
		return p.validateGatewayAPIKey(apiName, apiVersion, apiKey)
	case "mgt":
		return p.validateManagementPortalAPIKey(apiName, apiVersion, apiKey)
	case "dev":
		return p.validateDevPortalAPIKey(apiName, apiVersion, apiKey)
	default:
		return false
	}
}

// extractQueryParam extracts the first value of the given query parameter from the request path
func extractQueryParam(path, param string) string {
	// Parse the URL-encoded path
	decodedPath, err := url.PathUnescape(path)
	if err != nil {
		return ""
	}

	// Split the path into components
	parts := strings.Split(decodedPath, "?")
	if len(parts) != 2 {
		return ""
	}

	// Parse the query string
	queryString := parts[1]
	values, err := url.ParseQuery(queryString)
	if err != nil {
		return ""
	}

	// Get the first value of the specified parameter
	if value, ok := values[param]; ok && len(value) > 0 {
		return value[0]
	}

	return ""
}

// stripPrefix removes the specified prefix from the value (case-insensitive)
// Returns the value with prefix removed, or empty string if prefix doesn't match
func stripPrefix(value, prefix string) string {
	// Do exact case-insensitive prefix matching
	if len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix) {
		return value[len(prefix):]
	}

	// No matching prefix found, return empty string
	return ""
}

// extractAPIKeyPrefix extracts the prefix from an API key (everything before the first underscore)
func extractAPIKeyPrefix(apiKey string) string {
	parts := strings.SplitN(apiKey, "_", 2)
	if len(parts) >= 2 {
		return strings.ToLower(parts[0])
	}
	return ""
}

// validateGatewayAPIKey validates API keys with "gw_" prefix against the gateway controller database
func (p *APIKeyPolicy) validateGatewayAPIKey(apiName, apiVersion, apiKey string) bool {
	// Get gateway controller base URL from environment or use default
	baseURL := getGatewayControllerBaseURL()

	// Construct the URL path according to the internal API spec
	endpoint := fmt.Sprintf("/apis/%s/%s/validate-apikey",
		url.PathEscape(apiName),
		url.PathEscape(apiVersion))

	requestURL := baseURL + endpoint

	// Create request payload
	req := ApiKeyValidationRequest{
		ApiKey: apiKey,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		// Log error and return false for validation failure
		return false
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(payload))
	if err != nil {
		return false
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Make the request using the policy's HTTP client
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		// Network error or timeout - treat as validation failure
		return false
	}
	defer resp.Body.Close()

	// Handle different response status codes
	switch resp.StatusCode {
	case http.StatusOK:
		// Parse successful response
		var validationResp ApiKeyValidationResponse
		if err := json.NewDecoder(resp.Body).Decode(&validationResp); err != nil {
			return false
		}
		return validationResp.IsValid

	case http.StatusBadRequest:
		// Bad request - invalid API key format
		return false

	case http.StatusNotFound:
		// API not found - treat as validation failure
		return false

	case http.StatusInternalServerError:
		// Internal server error - treat as validation failure
		return false

	default:
		// Unexpected status code - treat as validation failure
		return false
	}
}

// getGatewayControllerBaseURL returns the gateway controller base URL
// This can be overridden via environment variables or configuration
func getGatewayControllerBaseURL() string {
	// In a real implementation, this could check environment variables:
	// if baseURL := os.Getenv("GATEWAY_CONTROLLER_BASE_URL"); baseURL != "" {
	//     return baseURL
	// }
	return DefaultGatewayControllerBaseURL
}

// validateManagementPortalAPIKey validates API keys with "mgt_" prefix against the management portal
func (p *APIKeyPolicy) validateManagementPortalAPIKey(apiName, apiVersion, apiKey string) bool {
	// TODO: Implement management portal API key validation
	// This should make an HTTP request to the management portal's API key validation endpoint
	// Example implementation:
	// 1. Use p.httpClient to make HTTP request
	// 2. Make POST request to management portal: POST /api/v1/validate-key
	// 3. Send payload: {"apiName": apiName, "apiVersion": apiVersion, "apiKey": apiKey}
	// 4. Parse response and return validation result

	return false
}

// validateDevPortalAPIKey validates API keys with "dev_" prefix against the developer portal
func (p *APIKeyPolicy) validateDevPortalAPIKey(apiName, apiVersion, apiKey string) bool {
	// TODO: Implement developer portal API key validation
	// This should make an HTTP request to the developer portal's API key validation endpoint
	// Example implementation:
	// 1. Use p.httpClient to make HTTP request
	// 2. Make POST request to developer portal: POST /api/v1/validate-key
	// 3. Send payload: {"apiName": apiName, "apiVersion": apiVersion, "apiKey": apiKey}
	// 4. Parse response and return validation result

	return false
}

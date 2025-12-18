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
	"fmt"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	"net/http"
	"net/url"
	"strings"
)

const (
	// Metadata keys for context storage
	MetadataKeyAuthSuccess = "auth.success"
	MetadataKeyAuthMethod  = "auth.method"
)

// APIKeyPolicy implements API Key Authentication
type APIKeyPolicy struct {
}

var ins = &APIKeyPolicy{}

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
		if headerValues := ctx.Headers.Get(http.CanonicalHeaderKey(keyName)); len(headerValues) > 0 {
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
	apiOperation := ctx.OperationPath
	operationMethod := ctx.Method

	if apiName == "" || apiVersion == "" || apiOperation == "" || operationMethod == "" {
		return p.handleAuthFailure(ctx, "missing API details for validation")
	}

	// API key was provided - validate it using external validation
	isValid, err := p.validateAPIKey(apiName, apiVersion, apiOperation, operationMethod, providedKey)
	if err != nil {
		return p.handleAuthFailure(ctx, "error validating API key")
	}
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
func (p *APIKeyPolicy) validateAPIKey(apiName, apiVersion, apiOperation, operationMethod, apiKey string) (bool, error) {
	// TODO - call the sdk validate api key method to validate the api key
	isValid, err := policy.ValidateAPIKey(apiName, apiVersion, apiOperation, operationMethod, apiKey)
	if err != nil {
		return false, fmt.Errorf("failed to validate API key via the policy engine")
	}
	return isValid, nil
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

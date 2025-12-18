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
	"encoding/json"
	"fmt"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	"log/slog"
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
	slog.Debug("API Key Auth Policy: OnRequest started",
		"path", ctx.Path,
		"method", ctx.Method,
		"apiName", ctx.APIName,
		"apiVersion", ctx.APIVersion,
	)

	// Get configuration parameters
	keyName, ok := params["key"].(string)
	if !ok || keyName == "" {
		slog.Debug("API Key Auth Policy: Missing or invalid 'key' configuration",
			"keyName", keyName,
			"ok", ok,
		)
		return p.handleAuthFailure(ctx, 401, "json", "Valid API key required",
			"missing or invalid 'key' configuration")
	}

	location, ok := params["in"].(string)
	if !ok || location == "" {
		slog.Debug("API Key Auth Policy: Missing or invalid 'in' configuration",
			"location", location,
			"ok", ok,
		)
		return p.handleAuthFailure(ctx, 401, "json", "Valid API key required",
			"missing or invalid 'in' configuration")
	}

	var valuePrefix string
	if valuePrefixRaw, ok := params["value-prefix"]; ok {
		if vp, ok := valuePrefixRaw.(string); ok {
			valuePrefix = vp
		}
	}

	slog.Debug("API Key Auth Policy: Configuration loaded",
		"keyName", keyName,
		"location", location,
		"valuePrefix", valuePrefix,
	)

	// Extract API key based on location
	var providedKey string

	if location == "header" {
		// Check header (case-insensitive)
		if headerValues := ctx.Headers.Get(http.CanonicalHeaderKey(keyName)); len(headerValues) > 0 {
			providedKey = headerValues[0]
			slog.Debug("API Key Auth Policy: Found API key in header",
				"headerName", keyName,
				"keyLength", len(providedKey),
			)
		}
	} else if location == "query" {
		// Extract query parameters from the full path
		providedKey = extractQueryParam(ctx.Path, keyName)
		if providedKey != "" {
			slog.Debug("API Key Auth Policy: Found API key in query parameter",
				"paramName", keyName,
				"keyLength", len(providedKey),
			)
		}
	}

	// If no API key provided
	if providedKey == "" {
		slog.Debug("API Key Auth Policy: No API key found",
			"location", location,
			"keyName", keyName,
		)
		return p.handleAuthFailure(ctx, 401, "json", "Valid API key required",
			"missing API key")
	}

	// Strip prefix if configured
	if valuePrefix != "" {
		originalLength := len(providedKey)
		providedKey = stripPrefix(providedKey, valuePrefix)
		slog.Debug("API Key Auth Policy: Processed value prefix",
			"prefix", valuePrefix,
			"originalLength", originalLength,
			"processedLength", len(providedKey),
		)

		// If after stripping prefix, the key is empty, treat as missing
		if providedKey == "" {
			slog.Debug("API Key Auth Policy: API key became empty after prefix removal")
			return p.handleAuthFailure(ctx, 401, "json", "Valid API key required",
				"missing API key")
		}
	}

	apiName := ctx.APIName
	apiVersion := ctx.APIVersion
	apiOperation := ctx.OperationPath
	operationMethod := ctx.Method

	if apiName == "" || apiVersion == "" || apiOperation == "" || operationMethod == "" {
		slog.Debug("API Key Auth Policy: Missing API details for validation",
			"apiName", apiName,
			"apiVersion", apiVersion,
			"apiOperation", apiOperation,
			"operationMethod", operationMethod,
		)
		return p.handleAuthFailure(ctx, 401, "json", "Valid API key required",
			"missing API details for validation")
	}

	slog.Debug("API Key Auth Policy: Starting validation",
		"apiName", apiName,
		"apiVersion", apiVersion,
		"apiOperation", apiOperation,
		"operationMethod", operationMethod,
		"keyLength", len(providedKey),
	)

	// API key was provided - validate it using external validation
	isValid, err := p.validateAPIKey(apiName, apiVersion, apiOperation, operationMethod, providedKey)
	if err != nil {
		slog.Debug("API Key Auth Policy: Validation error",
			"error", err,
		)
		return p.handleAuthFailure(ctx, 401, "json", "Valid API key required",
			"error validating API key")
	}
	if !isValid {
		slog.Debug("API Key Auth Policy: Invalid API key")
		return p.handleAuthFailure(ctx, 401, "json", "Valid API key required",
			"invalid API key")
	}

	// Authentication successful
	slog.Debug("API Key Auth Policy: Authentication successful")
	return p.handleAuthSuccess(ctx)
}

// handleAuthSuccess handles successful authentication
func (p *APIKeyPolicy) handleAuthSuccess(ctx *policy.RequestContext) policy.RequestAction {
	slog.Debug("API Key Auth Policy: handleAuthSuccess called",
		"apiName", ctx.APIName,
		"apiVersion", ctx.APIVersion,
		"method", ctx.Method,
		"path", ctx.Path,
	)

	// Set metadata indicating successful authentication
	ctx.Metadata[MetadataKeyAuthSuccess] = true
	ctx.Metadata[MetadataKeyAuthMethod] = "api-key"

	slog.Debug("API Key Auth Policy: Authentication metadata set",
		"authSuccess", true,
		"authMethod", "api-key",
	)

	// Continue to upstream with no modifications
	return policy.UpstreamRequestModifications{}
}

// OnResponse is not used by this policy (authentication is request-only)
func (p *APIKeyPolicy) OnResponse(_ctx *policy.ResponseContext, _params map[string]interface{}) policy.ResponseAction {
	return nil // No response processing needed
}

// handleAuthFailure handles authentication failure
func (p *APIKeyPolicy) handleAuthFailure(ctx *policy.RequestContext, statusCode int, errorFormat, errorMessage,
	reason string) policy.RequestAction {
	slog.Debug("API Key Auth Policy: handleAuthFailure called",
		"statusCode", statusCode,
		"errorFormat", errorFormat,
		"errorMessage", errorMessage,
		"reason", reason,
		"apiName", ctx.APIName,
		"apiVersion", ctx.APIVersion,
		"method", ctx.Method,
		"path", ctx.Path,
	)

	// Set metadata indicating failed authentication
	ctx.Metadata[MetadataKeyAuthSuccess] = false
	ctx.Metadata[MetadataKeyAuthMethod] = "api-key"

	headers := map[string]string{
		"content-type": "application/json",
	}

	var body string
	switch errorFormat {
	case "plain":
		body = errorMessage
		headers["content-type"] = "text/plain"
	default: // json
		errResponse := map[string]interface{}{
			"error":   "Unauthorized",
			"message": errorMessage,
		}
		bodyBytes, _ := json.Marshal(errResponse)
		body = string(bodyBytes)
	}

	slog.Debug("API Key Auth Policy: Returning immediate response",
		"statusCode", statusCode,
		"contentType", headers["content-type"],
		"bodyLength", len(body),
		"reason", reason,
	)

	return policy.ImmediateResponse{
		StatusCode: statusCode,
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

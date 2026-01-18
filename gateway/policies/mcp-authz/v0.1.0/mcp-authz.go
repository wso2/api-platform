/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package mcpauthz

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	WWWAuthenticateHeader     = "WWW-Authenticate"
	AuthMethodBearer          = "Bearer resource_metadata="
	WellKnownPath             = ".well-known/oauth-protected-resource"
	MetadataValidatedClaims   = "auth.validatedClaims"
	MetadataMcpMethod         = "mcp.method"
	MetadataMcpCapabilityType = "mcp.type"
	MetadataMcpCapabilityName = "mcp.name"
)

// MCPRequest represents the JSON-RPC MCP request structure
type MCPRequest struct {
	Method string           `json:"method"`
	Params MCPRequestParams `json:"params"`
}

// MCPRequestParams represents the params section of an MCP request
// Different MCP methods use different param structures:
// - tools/call: uses "name" (tool name) and "arguments"
// - resources/read: uses "uri" (resource URI)
// - prompts/get: uses "name" (prompt name)
type MCPRequestParams struct {
	Name      string         `json:"name"` // For tools/call, prompts/get
	Arguments map[string]any `json:"arguments"`
	URI       string         `json:"uri"` // For resources/read
}

// Rule represents a single authorization rule
type Rule struct {
	Attribute      Attribute
	RequiredClaims map[string]string
	RequiredScopes []string
}

// Attribute represents the MCP resource attribute being authorized
type Attribute struct {
	Type string
	Name string
}

type McpAuthzPolicy struct {
	Rules []Rule
}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]any,
) (policy.Policy, error) {
	slog.Debug("MCP Authorization Policy: GetPolicy called")

	p := &McpAuthzPolicy{}

	// Parse rules from params
	rules, err := parseRules(params)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rules: %w", err)
	}
	p.Rules = rules

	slog.Debug("MCP Authorization Policy: Parsed policy configuration",
		"rulesCount", len(p.Rules))

	return p, nil
}

// parseRules extracts and validates rules from params
func parseRules(params map[string]any) ([]Rule, error) {
	rulesRaw, ok := params["rules"]
	if !ok {
		return nil, fmt.Errorf("rules parameter is required")
	}

	rulesArray, ok := rulesRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("rules must be an array")
	}

	var rules []Rule
	for i, ruleRaw := range rulesArray {
		ruleMap, ok := ruleRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("rules[%d] must be an object", i)
		}

		rule, err := parseRule(ruleMap, i)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// parseRule parses a single rule from a map
func parseRule(ruleMap map[string]any, index int) (Rule, error) {
	rule := Rule{}

	// Parse attribute (required)
	attrRaw, ok := ruleMap["attribute"]
	if !ok {
		return rule, fmt.Errorf("rules[%d].attribute is required", index)
	}
	attrMap, ok := attrRaw.(map[string]any)
	if !ok {
		return rule, fmt.Errorf("rules[%d].attribute must be an object", index)
	}

	// Parse attribute type (required)
	attrType, ok := attrMap["type"]
	if !ok {
		return rule, fmt.Errorf("rules[%d].attribute.type is required", index)
	}
	attrTypeStr, ok := attrType.(string)
	if !ok {
		return rule, fmt.Errorf("rules[%d].attribute.type must be a string", index)
	}
	if !isValidAttributeType(attrTypeStr) {
		return rule, fmt.Errorf("rules[%d].attribute.type must be one of: tool, resource, prompt, method", index)
	}
	rule.Attribute.Type = attrTypeStr

	// Parse attribute name (optional, defaults to "*")
	if attrName, ok := attrMap["name"]; ok {
		attrNameStr, ok := attrName.(string)
		if !ok {
			return rule, fmt.Errorf("rules[%d].attribute.name must be a string", index)
		}
		rule.Attribute.Name = attrNameStr
	} else {
		rule.Attribute.Name = "*"
	}

	// Parse requiredClaims (optional)
	if claimsRaw, ok := ruleMap["requiredClaims"]; ok {
		claimsMap, ok := claimsRaw.(map[string]any)
		if !ok {
			return rule, fmt.Errorf("rules[%d].requiredClaims must be an object", index)
		}
		rule.RequiredClaims = make(map[string]string)
		for k, v := range claimsMap {
			vStr, ok := v.(string)
			if !ok {
				return rule, fmt.Errorf("rules[%d].requiredClaims[%s] must be a string", index, k)
			}
			rule.RequiredClaims[k] = vStr
		}
	}

	// Parse requiredScopes (optional)
	if scopesRaw, ok := ruleMap["requiredScopes"]; ok {
		scopesArray, ok := scopesRaw.([]any)
		if !ok {
			return rule, fmt.Errorf("rules[%d].requiredScopes must be an array", index)
		}
		for j, scopeRaw := range scopesArray {
			scopeStr, ok := scopeRaw.(string)
			if !ok {
				return rule, fmt.Errorf("rules[%d].requiredScopes[%d] must be a string", index, j)
			}
			rule.RequiredScopes = append(rule.RequiredScopes, scopeStr)
		}
	}

	return rule, nil
}

// isValidAttributeType checks if the attribute type is valid
func isValidAttributeType(attrType string) bool {
	return attrType == "tool" || attrType == "resource" || attrType == "prompt" || attrType == "method"
}

func (p *McpAuthzPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

func (p *McpAuthzPolicy) OnRequest(ctx *policy.RequestContext, params map[string]any) policy.RequestAction {
	if strings.EqualFold(ctx.Method, "POST") && strings.Contains(ctx.Path, "/mcp") {
		slog.Debug("MCP Authorization Policy: Processing MCP request for authorization")
	} else {
		slog.Debug("MCP Authorization Policy: Skipping authz...")
		return nil
	}
	// Extract JWT claims
	jwtClaims, ok := ctx.Metadata[MetadataValidatedClaims]
	if !ok {
		slog.Debug("MCP Authorization Policy: No validated claims found in metadata")
		return p.handleAuthFailure(ctx, "Unauthorized: scope/claim validation failed", nil)
	}
	claims, ok := jwtClaims.(jwt.MapClaims)
	if !ok {
		slog.Debug("MCP Authorization Policy: Invalid claims type in metadata")
		return p.handleAuthFailure(ctx, "Unauthorized: scope/claim validation failed", nil)
	}

	// Parse MCP request to extract method and name
	var mcpReq MCPRequest
	if err := json.Unmarshal(ctx.Body.Content, &mcpReq); err != nil {
		slog.Debug("MCP Authorization Policy: Failed to parse MCP request", "error", err)
		return p.handleAuthFailure(ctx, "Invalid MCP request format", nil)
	}

	slog.Debug("MCP Authorization Policy: Extracted MCP attributes",
		"method", mcpReq.Method,
		"name", mcpReq.Params.Name,
		"uri", mcpReq.Params.URI)

	// Determine attribute type from method
	attributeType, ok := p.getAttributeTypeFromMethod(mcpReq.Method)
	if !ok {
		slog.Debug("MCP Authorization Policy: Skipping since the method is not one of tools, resources, or prompts", "method", mcpReq.Method)
		return nil
	}

	// Extract attribute name/identifier based on method type
	attributeName := p.getAttributeNameFromParams(mcpReq.Method, mcpReq.Params)

	// Set MCP metadata in context for other policies
	ctx.Metadata[MetadataMcpMethod] = mcpReq.Method
	ctx.Metadata[MetadataMcpCapabilityType] = attributeType
	ctx.Metadata[MetadataMcpCapabilityName] = attributeName

	// Check authorization rules
	authorized, missingScopes := p.checkAuthorization(attributeType, attributeName, mcpReq.Method, claims)
	if !authorized {
		slog.Debug("MCP Authorization Policy: Authorization check failed",
			"attributeName", mcpReq.Params.Name,
			"method", mcpReq.Method)
		return p.handleAuthFailure(ctx, "Forbidden: insufficient permissions to access this MCP resource", missingScopes)
	}

	slog.Debug("MCP Authorization Policy: Authorization check passed")
	return nil
}

func (p *McpAuthzPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]any) policy.ResponseAction {
	return nil
}

// getAttributeTypeFromMethod extracts the attribute type from the MCP method
func (p *McpAuthzPolicy) getAttributeTypeFromMethod(method string) (string, bool) {
	parts := strings.Split(method, "/")
	if len(parts) != 2 {
		return "", false
	}

	resourceType := parts[0]
	switch resourceType {
	case "tools":
		return "tool", true
	case "resources":
		return "resource", true
	case "prompts":
		return "prompt", true
	default:
		return "", false
	}
}

// getAttributeNameFromParams extracts the attribute name/identifier from params based on method type
func (p *McpAuthzPolicy) getAttributeNameFromParams(method string, params MCPRequestParams) string {
	parts := strings.Split(method, "/")
	if len(parts) != 2 {
		return ""
	}

	resourceType := parts[0]
	switch resourceType {
	case "tools", "prompts":
		// For tools/call and prompts/get, use the "name" field
		return params.Name
	case "resources":
		// For resources/read (and other resource methods), use the "uri" field
		return params.URI
	default:
		return ""
	}
}

// checkAuthorization validates whether the request should be authorized
func (p *McpAuthzPolicy) checkAuthorization(attributeType, attributeName, method string, claims jwt.MapClaims) (bool, map[string]struct{}) {
	if len(p.Rules) == 0 {
		slog.Debug("MCP Authorization Policy: No rules configured")
		return true, nil
	}

	// Find matching rules (most specific first)
	matchingRules := p.findMatchingRules(attributeType, attributeName, method)
	if len(matchingRules) == 0 {
		slog.Debug("MCP Authorization Policy: No matching rules found")
		return true, nil
	}

	var missingScopes = make(map[string]struct{})
	// Check if any matching rule grants access
	isAuthorized := true
	for _, rule := range matchingRules {
		if ok, scopes := p.ruleGrantsAccess(rule, claims); !ok {
			slog.Debug("MCP Authorization Policy: Rule did not grant access",
				"attributeType", attributeType,
				"attributeName", attributeName,
				"missingScopes", scopes)
			isAuthorized = false
			for _, s := range scopes {
				if _, exists := missingScopes[s]; !exists {
					missingScopes[s] = struct{}{}
				}
			}
			continue
		}
	}

	return isAuthorized, missingScopes
}

// findMatchingRules returns rules that match the attribute, sorted by specificity
func (p *McpAuthzPolicy) findMatchingRules(attributeType, attributeName, method string) []Rule {
	var matching []Rule

	for _, rule := range p.Rules {
		// Special handling for method-based rules since attribute type is derived from the method prefix
		if rule.Attribute.Type == "method" && (rule.Attribute.Name == "*" || rule.Attribute.Name == method) {
			slog.Debug("MCP Authorization Policy: Found matching method-based rule", "method", method)
			matching = append(matching, rule)
			continue
		}

		if rule.Attribute.Type != attributeType {
			slog.Debug("MCP Authorization Policy: Skipping rule due to attribute type mismatch",
				"ruleAttributeType", rule.Attribute.Type,
				"requestAttributeType", attributeType)
			continue
		}

		// Match exact name or wildcard
		// Ignore the attribute name if it's empty. This handles cases where the callable capabilities
		// are not present (eg: tools/list).
		if attributeName != "" && (rule.Attribute.Name == "*" || rule.Attribute.Name == attributeName) {
			slog.Debug("MCP Authorization Policy: Found matching rule",
				"attributeType", attributeType,
				"attributeName", attributeName)
			matching = append(matching, rule)
		}
	}

	// Sort by specificity: exact names before wildcards
	specificRules := []Rule{}
	wildcardRules := []Rule{}
	for _, rule := range matching {
		if rule.Attribute.Name == "*" {
			wildcardRules = append(wildcardRules, rule)
		} else {
			specificRules = append(specificRules, rule)
		}
	}

	return append(specificRules, wildcardRules...)
}

// ruleGrantsAccess checks if a rule's claims and scopes are satisfied
func (p *McpAuthzPolicy) ruleGrantsAccess(rule Rule, claims jwt.MapClaims) (bool, []string) {
	// Check required claims
	if len(rule.RequiredClaims) > 0 {
		if !p.checkClaims(rule.RequiredClaims, claims) {
			return false, nil
		}
	}

	// Check required scopes
	if len(rule.RequiredScopes) > 0 {
		ok, missing := p.checkScopes(rule.RequiredScopes, claims)
		if !ok {
			return false, missing
		}
	}

	return true, nil
}

// checkClaims verifies that all required claims match their expected values
func (p *McpAuthzPolicy) checkClaims(requiredClaims map[string]string, claims jwt.MapClaims) bool {
	for claimName, expectedValue := range requiredClaims {
		claimValue, ok := claims[claimName]
		if !ok {
			slog.Debug("MCP Authorization Policy: Required claim not found",
				"claim", claimName)
			return false
		}

		// Convert claim value to string for comparison
		claimStr, ok := claimValue.(string)
		if !ok {
			slog.Debug("MCP Authorization Policy: Claim value is not a string",
				"claim", claimName)
			return false
		}

		if claimStr != expectedValue {
			slog.Debug("MCP Authorization Policy: Claim value mismatch",
				"claim", claimName,
				"expected", expectedValue,
				"actual", claimStr)
			return false
		}
	}

	return true
}

// checkScopes verifies that at least one required scope is present in the token
func (p *McpAuthzPolicy) checkScopes(requiredScopes []string, claims jwt.MapClaims) (bool, []string) {
	var missing []string
	// Get scopes from token (space-delimited "scope" or array "scp")
	tokenScopes := p.extractScopes(claims)
	if len(tokenScopes) == 0 {
		slog.Debug("MCP Authorization Policy: No scopes found in token")
		missing = append(missing, requiredScopes...)
		return false, missing
	}

	// Check if all required scopes are in token scopes
	for _, requiredScope := range requiredScopes {
		if slices.Contains(tokenScopes, requiredScope) {
			slog.Debug("MCP Authorization Policy: Scope match found",
				"scope", requiredScope)
		} else {
			missing = append(missing, requiredScope)
		}
	}

	if len(missing) == 0 {
		slog.Debug("MCP Authorization Policy: All required scopes are present")
		return true, nil
	}

	slog.Debug("MCP Authorization Policy: No matching scopes found",
		"required", requiredScopes,
		"available", tokenScopes)
	return false, missing
}

// extractScopes gets scopes from either "scope" (space-delimited) or "scp" (array) claim
func (p *McpAuthzPolicy) extractScopes(claims jwt.MapClaims) []string {
	var scopes []string

	// Try space-delimited "scope" claim
	if scopeVal, ok := claims["scope"]; ok {
		if scopeStr, ok := scopeVal.(string); ok {
			scopes = strings.Fields(scopeStr)
			return scopes
		}
	}

	// Try array "scp" claim
	if scpVal, ok := claims["scp"]; ok {
		if scpArray, ok := scpVal.([]any); ok {
			for _, s := range scpArray {
				if scopeStr, ok := s.(string); ok {
					scopes = append(scopes, scopeStr)
				}
			}
			return scopes
		}
	}

	return scopes
}

// generateResourcePath generates the full resource URL for the given resource path
func generateResourcePath(ctx *policy.RequestContext, gatewayHost string, resource string) string {
	slog.Debug("MCP Authorization Policy: Generating resource path for", "resource", resource)

	scheme := ctx.Scheme
	_, port := parseAuthority(ctx.Authority)

	// Determine the host - prefer vhost, fallback to gatewayHost param
	var host string
	if ctx.Vhost != "" && !strings.Contains(ctx.Vhost, "*") {
		host = ctx.Vhost
		slog.Debug("MCP Authorization Policy: Using VHost with port from context", "vhost", host)
	} else {
		if gatewayHost == "" {
			gatewayHost = "localhost"
		}
		host = gatewayHost
		slog.Debug("MCP Authorization Policy: VHost not found, using gateway host from params", "host", host)
	}

	// Determine port if not present in authority
	if port == -1 {
		slog.Debug("MCP Authorization Policy: No port specified, using default port based on scheme")
		if scheme == "https" {
			port = 8443
		} else {
			port = 8080
		}
	}

	// Build host:port, omitting standard ports
	hostWithPort := host
	if !isStandardPort(scheme, port) {
		slog.Debug("MCP Auth Policy: Adding non-standard port to host", "port", port)
		hostWithPort = fmt.Sprintf("%s:%d", host, port)
	}

	// Build the full URL path
	apiContext := ctx.APIContext
	if apiContext != "" {
		return fmt.Sprintf("%s://%s%s/%s", scheme, hostWithPort, apiContext, resource)
	}
	return fmt.Sprintf("%s://%s/%s", scheme, hostWithPort, resource)
}

// generateWwwAuthenticateHeader generates the WWW-Authenticate header value
func generateWwwAuthenticateHeader(ctx *policy.RequestContext, scopes []string, errorDesc string) string {
	slog.Debug("MCP Authorization Policy: Generating WWW-Authenticate header")
	gatewayHost, ok := ctx.Metadata["gatewayHost"]
	gatewayHostString, _ := gatewayHost.(string)
	if !ok || gatewayHostString == "" {
		slog.Debug("MCP Authorization Policy: gatewayHost is empty in metadata, using empty string")
		gatewayHostString = ""
	}
	headerValue := AuthMethodBearer + "\"" + generateResourcePath(ctx, gatewayHostString, WellKnownPath) + "\""
	if len(scopes) > 0 {
		slog.Debug("MCP Authorization Policy: Adding scopes to WWW-Authenticate header")
		headerValue += ", scope=\"" + strings.Join(scopes, " ") + "\""
	}
	if errorDesc != "" {
		slog.Debug("MCP Authorization Policy: Adding error description to WWW-Authenticate header")
		headerValue += ", error=\"invalid_token\", error_description=\"" + errorDesc + "\""
	}
	return headerValue
}

// parseAuthority extracts host and port from an authority string (e.g., "example.com:8080")
func parseAuthority(authority string) (host string, port int) {
	if authority == "" {
		return "", -1
	}
	hostPort := strings.SplitN(authority, ":", 2)
	host = hostPort[0]
	if len(hostPort) > 1 {
		port, _ = strconv.Atoi(hostPort[1])
	} else {
		port = -1
	}
	return host, port
}

// isStandardPort returns true if the port is the standard port for the given scheme
func isStandardPort(scheme string, port int) bool {
	return (scheme == "http" && port == 80) || (scheme == "https" && port == 443)
}

func (p *McpAuthzPolicy) handleAuthFailure(ctx *policy.RequestContext, errorMessage string, scopeMap map[string]struct{}) policy.RequestAction {
	slog.Debug("MCP Authorization Policy: handleAuthFailure called",
		"errorMessage", errorMessage,
	)

	var missingScopes []string
	for s := range scopeMap {
		missingScopes = append(missingScopes, s)
	}

	// Generate WWW-Authenticate header
	wwwAuthHeader := generateWwwAuthenticateHeader(ctx, missingScopes, errorMessage)

	headers := map[string]string{
		"content-type":        "application/json",
		WWWAuthenticateHeader: wwwAuthHeader,
	}

	errResponse := map[string]interface{}{
		"error":   "Forbidden",
		"message": errorMessage,
	}
	bodyBytes, _ := json.Marshal(errResponse)

	return policy.ImmediateResponse{
		StatusCode: 403,
		Headers:    headers,
		Body:       bodyBytes,
	}
}

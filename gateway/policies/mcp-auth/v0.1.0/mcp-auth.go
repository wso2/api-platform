/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package mcpauthn

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	jwtauth "github.com/policy-engine/policies/jwtauthentication"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	WWWAuthenticateHeader  = "WWW-Authenticate"
	AuthMethodBearer       = "Bearer resource_metadata="
	WellKnownPath          = ".well-known/oauth-protected-resource"
	McpSessionHeader       = "mcp-session-id"
	MetadataKeyAuthSuccess = "auth.success"
	MetadataKeyAuthMethod  = "auth.method"
)

type McpAuthPolicy struct{}

type ProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
}

var ins = &McpAuthPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]any,
) (policy.Policy, error) {
	slog.Debug("MCP Auth Policy: GetPolicy called")
	return ins, nil
}

func (p *McpAuthPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

func (p *McpAuthPolicy) OnRequest(ctx *policy.RequestContext, params map[string]any) policy.RequestAction {
	userIssuers := getStringArrayParam(params, "issuers", []string{})
	onFailureStatusCode := getIntParam(params, "onFailureStatusCode", 401)
	errorMessageFormat := getStringParam(params, "errorMessageFormat", "json")
	userRequiredScopes := getStringArrayParam(params, "requiredScopes", []string{})
	// Check for GET /.well-known/oauth-protected-resource
	if ctx.Method == "GET" && strings.Contains(ctx.Path, WellKnownPath) {
		slog.Debug("MCP Auth Policy: Handling well-known protected resource metadata request")
		sessionIds := ctx.Headers.Get(McpSessionHeader)
		sessionId := ""
		if len(sessionIds) > 0 {
			sessionId = sessionIds[0]
		}

		// Get key managers configuration
		keyManagersRaw, ok := params["keyManagers"]
		if !ok {
			slog.Debug("MCP Auth Policy: Key managers not configured in params")
			return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, "key managers not configured")
		}

		slog.Debug("MCP Auth Policy: Starting to parse key managers configuration")

		issuers := []string{}
		kms := make(map[string]string)
		keyManagersList, ok := keyManagersRaw.([]any)
		if ok {
			for _, km := range keyManagersList {
				if kmMap, ok := km.(map[string]any); ok {
					name := getString(kmMap["name"])
					issuer := getString(kmMap["issuer"])
					if name == "" || issuer == "" {
						continue
					}
					issuers = append(issuers, issuer)
					kms[name] = issuer
				}
			}
		}
		if len(issuers) == 0 {
			return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, "no valid key managers found")
		}

		if len(userIssuers) > 0 {
			filteredIssuers := []string{}
			for _, ui := range userIssuers {
				if issuer, ok := kms[ui]; ok {
					filteredIssuers = append(filteredIssuers, issuer)
					slog.Debug("MCP Auth Policy: Added issuer from user configuration", "issuer", issuer)
				}
			}
			issuers = filteredIssuers
		}

		if len(issuers) == 0 {
			return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, "no matching issuers found")
		}

		// todo: mcp auth flow
		prm := ProtectedResourceMetadata{
			Resource:             generateResourcePath(ctx, params, "mcp"),
			AuthorizationServers: issuers,
			ScopesSupported:      userRequiredScopes,
		}
		jsonOut, _ := json.Marshal(prm)
		return policy.ImmediateResponse{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type":   "application/json",
				McpSessionHeader: sessionId,
			},
			Body: jsonOut,
		}
	}
	return p.handleAuth(ctx, params, userRequiredScopes)
}

func (p *McpAuthPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]any) policy.ResponseAction {
	return nil
}

// handleAuth does the MCP specific authentication handling
func (p *McpAuthPolicy) handleAuth(ctx *policy.RequestContext, params map[string]any, scopes []string) policy.RequestAction {
	sessionIds := ctx.Headers.Get(McpSessionHeader)
	sessionId := ""
	if len(sessionIds) > 0 {
		sessionId = sessionIds[0]
	}

	slog.Debug("MCP Auth Policy: Delegating authentication to JWT Auth Policy")
	jwtPolicy, _ := jwtauth.GetPolicy(policy.PolicyMetadata{}, params)
	reqAction := jwtPolicy.OnRequest(ctx, params)
	if _, ok := reqAction.(policy.ImmediateResponse); ok {
		slog.Debug("MCP Auth Policy: Authentication failed in JWT Auth Policy, handling failure")
		headers := reqAction.(policy.ImmediateResponse).Headers
		headers[WWWAuthenticateHeader] = generateWwwAuthenticateHeader(ctx, params, scopes, reqAction.(policy.ImmediateResponse))
		headers[McpSessionHeader] = sessionId
		return policy.ImmediateResponse{
			StatusCode: reqAction.(policy.ImmediateResponse).StatusCode,
			Headers:    headers,
			Body:       reqAction.(policy.ImmediateResponse).Body,
		}
	}
	return reqAction
}

func (p *McpAuthPolicy) handleAuthFailure(ctx *policy.RequestContext, statusCode int, format string, reason any) policy.RequestAction {
	slog.Debug("MCP Auth Policy: Handling authentication failure", "statusCode", statusCode, "reason", reason)
	ctx.Metadata[MetadataKeyAuthSuccess] = false
	ctx.Metadata[MetadataKeyAuthMethod] = "mcpAuth"
	var body string
	headers := map[string]string{
		"content-type": "application/json",
	}
	switch format {
	case "plain":
		body = fmt.Sprintf("Authentication failed: %s", reason)
		headers["content-type"] = "text/plain"
	case "minimal":
		body = "Unauthorized"
	default: // json
		errResponse := map[string]interface{}{
			"error":   "Unauthorized",
			"message": fmt.Sprintf("MCP authentication failed: %s", reason),
		}
		bodyBytes, _ := json.Marshal(errResponse)
		body = string(bodyBytes)
	}

	return policy.ImmediateResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       []byte(body),
	}
}

// generateResourcePath generates the full resource URL for the given resource path
func generateResourcePath(ctx *policy.RequestContext, params map[string]any, resource string) string {
	slog.Debug("MCP Auth Policy: Generating resource path for", "resource", resource)

	scheme := ctx.Scheme
	_, port := parseAuthority(ctx.Authority)

	// Determine the host - prefer vhost, fallback to gatewayHost param
	var host string
	if ctx.Vhost != "" && !strings.Contains(ctx.Vhost, "*") {
		host = ctx.Vhost
		slog.Debug("MCP Auth Policy: Using VHost with port from context", "vhost", host)
	} else {
		host = getStringParam(params, "gatewayHost", "localhost")
		slog.Debug("MCP Auth Policy: VHost not found, using gateway host from params", "host", host)
	}

	// Determine port if not present in authority
	if port == -1 {
		slog.Debug("MCP Auth Policy: No port specified, using default port based on scheme")
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
func generateWwwAuthenticateHeader(ctx *policy.RequestContext, params map[string]any, scopes []string, ir policy.ImmediateResponse) string {
	slog.Debug("MCP Auth Policy: Generating WWW-Authenticate header")
	headerValue := AuthMethodBearer + "\"" + generateResourcePath(ctx, params, WellKnownPath) + "\""
	if len(scopes) > 0 {
		slog.Debug("MCP Auth Policy: Adding scopes to WWW-Authenticate header")
		headerValue += ", scope=\"" + strings.Join(scopes, " ") + "\""
	}
	contentType := ir.Headers["content-type"]
	if contentType == "application/json" {
		var errResp map[string]any
		if err := json.Unmarshal(ir.Body, &errResp); err == nil {
			if errDesc, ok := errResp["message"].(string); ok {
				escapedDesc := strings.ReplaceAll(errDesc, "\"", "'")
				headerValue += ", error=\"invalid_token\", error_description=\"" + escapedDesc + "\""
			}
		}
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

func getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

func getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if v, ok := params[key]; ok {
		if i, ok := v.(int); ok {
			return i
		}
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return defaultValue
}

func getStringArrayParam(params map[string]interface{}, key string, defaultValue []string) []string {
	if v, ok := params[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			var result []string
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return defaultValue
}

// Helper functions for type assertions
func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

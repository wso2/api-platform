/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"

	"gopkg.in/yaml.v3"
)

// MCP JSON-RPC constants
const (
	JsonRpcVersion      = "2.0"
	ProtocolVersion     = "2025-06-18"
	MethodInitialize    = "initialize"
	MethodInitialized   = "notifications/initialized"
	MethodToolsList     = "tools/list"
	MethodPromptsList   = "prompts/list"
	MethodResourcesList = "resources/list"
	ClientName          = "api-platform-mcp-client"
	ClientVersion       = "1.0.0"
	McpSessionHeader    = "mcp-session-id"
)

// JsonRPCRequest represents a JSON-RPC request
type JsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JsonRPCError represents a JSON-RPC error
type JsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolsResult represents the result of tools/list request
type ToolsResult struct {
	Result struct {
		Tools []map[string]interface{} `json:"tools"`
	} `json:"result"`
	Error *JsonRPCError `json:"error"`
}

// PromptsResult represents the result of prompts/list request
type PromptsResult struct {
	Result struct {
		Prompts []map[string]interface{} `json:"prompts"`
	} `json:"result"`
	Error *JsonRPCError `json:"error"`
}

// ResourcesResult represents the result of resources/list request
type ResourcesResult struct {
	Result struct {
		Resources []map[string]interface{} `json:"resources"`
	} `json:"result"`
	Error *JsonRPCError `json:"error"`
}

// InitializeResult represents the result of initialize request
type InitializeResult struct {
	Result struct {
		ProtocolVersion string                 `json:"protocolVersion"`
		ServerInfo      map[string]interface{} `json:"serverInfo"`
		Capabilities    map[string]interface{} `json:"capabilities"`
	} `json:"result"`
	Error *JsonRPCError `json:"error"`
}

type MCPUtils struct{}

func (u *MCPUtils) GenerateMCPDeploymentYAML(proxy *model.MCPProxy) (string, error) {

	contextValue := "/"
	if proxy.Configuration.Context != nil && *proxy.Configuration.Context != "" {
		contextValue = *proxy.Configuration.Context
	}
	var vhostValue *string
	if proxy.Configuration.Vhost != nil {
		vhostValue = proxy.Configuration.Vhost
	}

	mcpDeploymentYaml := model.MCPProxyDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.MCPProxy,
		Metadata: model.DeploymentMetadata{
			Name: proxy.Handle,
		},
		Spec: model.MCPProxyDeploymentSpec{
			DisplayName: proxy.Name,
			Version:     proxy.Version,
			Context:     contextValue,
			Vhost:       vhostValue,
			Upstream:    proxy.Configuration.Upstream,
			SpecVersion: proxy.Configuration.SpecVersion,
			Policies:    proxy.Configuration.Policies,
		},
	}

	if proxy.ProjectUUID != nil {
		mcpDeploymentYaml.Metadata.Labels = map[string]string{
			"projectId": *proxy.ProjectUUID,
		}
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(mcpDeploymentYaml)
	if err != nil {
		return "", fmt.Errorf("failed to marshal API to YAML: %w", err)
	}

	return string(yamlBytes), nil

}

// FetchMCPServerInfo fetches server information from an MCP backend including tools, prompts, resources, and server info
func FetchMCPServerInfo(url string, headerName string, headerValue string) (*api.MCPServerInfoFetchResponse, error) {
	// Step 1: Initialize MCP server
	sessionID, serverInfo, err := initializeMCPServer(url, headerName, headerValue)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP server: %w", err)
	}

	// Step 2: Send notifications/initialized
	notifyReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		Method:  MethodInitialized,
	}
	_, err = postJSONRPCWithSession(url, notifyReq, sessionID, headerName, headerValue)
	if err != nil {
		return nil, fmt.Errorf("failed to send notification: %w", err)
	}

	// Step 3: Fetch tools
	toolsReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      2,
		Method:  MethodToolsList,
	}
	toolsResp, err := postJSONRPCWithSession(url, toolsReq, sessionID, headerName, headerValue)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools: %w", err)
	}
	var toolsResult ToolsResult
	if err := json.Unmarshal(toolsResp, &toolsResult); err != nil {
		return nil, fmt.Errorf("failed to parse tools response: %w", err)
	}
	if toolsResult.Error != nil {
		return nil, fmt.Errorf("tools/list request returned an error: %s", toolsResult.Error.Message)
	}

	// Step 4: Fetch prompts
	promptsReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      3,
		Method:  MethodPromptsList,
	}
	promptsResp, err := postJSONRPCWithSession(url, promptsReq, sessionID, headerName, headerValue)
	if err != nil {
		return nil, fmt.Errorf("failed to get prompts: %w", err)
	}
	var promptsResult PromptsResult
	if err := json.Unmarshal(promptsResp, &promptsResult); err != nil {
		return nil, fmt.Errorf("failed to parse prompts response: %w", err)
	}
	if promptsResult.Error != nil {
		return nil, fmt.Errorf("prompts/list request returned an error: %s", promptsResult.Error.Message)
	}

	// Step 5: Fetch resources
	resourcesReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      4,
		Method:  MethodResourcesList,
	}
	resourcesResp, err := postJSONRPCWithSession(url, resourcesReq, sessionID, headerName, headerValue)
	if err != nil {
		return nil, fmt.Errorf("failed to get resources: %w", err)
	}
	var resourcesResult ResourcesResult
	if err := json.Unmarshal(resourcesResp, &resourcesResult); err != nil {
		return nil, fmt.Errorf("failed to parse resources response: %w", err)
	}
	if resourcesResult.Error != nil {
		return nil, fmt.Errorf("resources/list request returned an error: %s", resourcesResult.Error.Message)
	}

	// Build response
	resp := &api.MCPServerInfoFetchResponse{}

	if len(toolsResult.Result.Tools) > 0 {
		resp.Tools = &toolsResult.Result.Tools
	}
	if len(promptsResult.Result.Prompts) > 0 {
		resp.Prompts = &promptsResult.Result.Prompts
	}
	if len(resourcesResult.Result.Resources) > 0 {
		resp.Resources = &resourcesResult.Result.Resources
	}
	if serverInfo != nil {
		resp.ServerInfo = &serverInfo
	}

	return resp, nil
}

// initializeMCPServer initializes the MCP server and returns the session ID and server info
func initializeMCPServer(url string, headerName string, headerValue string) (string, map[string]any, error) {
	initReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      1,
		Method:  MethodInitialize,
		Params: map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"roots": map[string]bool{"listChanged": true}},
			"clientInfo":      map[string]string{"name": ClientName, "version": ClientVersion},
		},
	}
	data, err := json.Marshal(initReq)
	if err != nil {
		return "", nil, err
	}
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create init request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if headerName != "" {
		lower := strings.ToLower(headerName)
		if lower != McpSessionHeader && lower != "content-type" && lower != "accept" {
			httpReq.Header.Set(headerName, headerValue)
		}
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", nil, fmt.Errorf("failed to reach MCP server for initialize: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("initialize request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Check if response is event stream and parse it
	isEventStreamResp := isEventStream(resp)
	if isEventStreamResp {
		data, err := parseEventStream(body)
		if err == nil {
			body = data
		}
	}

	// Parse initialize response for server info
	var initResult InitializeResult
	if err := json.Unmarshal(body, &initResult); err != nil {
		// Only ignore unmarshal error if this was a valid event stream (parsed above)
		if !isEventStreamResp {
			return "", nil, fmt.Errorf("failed to parse initialize response: %w, body: %s", err, string(body))
		}
		// For event stream, if unmarshal fails after successful parsing, that's still an error
		return "", nil, fmt.Errorf("failed to parse initialize response from event stream: %w, body: %s", err, string(body))
	}

	if initResult.Error != nil {
		return "", nil, fmt.Errorf("initialize request returned an error: %s", initResult.Error.Message)
	}

	var serverInfo map[string]any
	if initResult.Result.ServerInfo != nil {
		serverInfo = initResult.Result.ServerInfo
	}

	sessionID := getSessionIDFromResponse(resp)
	return sessionID, serverInfo, nil
}

func getSessionIDFromResponse(resp *http.Response) string {
	return resp.Header.Get(McpSessionHeader)
}

// postJSONRPCWithSession sends a JSON-RPC request with mcp-session-id header if provided
func postJSONRPCWithSession(url string, req any, sessionID string, headerName string, headerValue string) ([]byte, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if headerName != "" {
		lower := strings.ToLower(headerName)
		if lower != McpSessionHeader && lower != "content-type" && lower != "accept" {
			httpReq.Header.Set(headerName, headerValue)
		}
	}
	if sessionID != "" {
		httpReq.Header.Set(McpSessionHeader, sessionID)
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Check if response is event stream
	if isEventStream(resp) {
		// Extract JSON data from event stream
		data, err := parseEventStream(body)
		if err != nil {
			return nil, fmt.Errorf("failed to parse event stream: %w, body: %s", err, string(body))
		}
		return data, nil
	}

	return body, nil
}

// parseEventStream extracts JSON data from event stream response
func parseEventStream(body []byte) ([]byte, error) {
	lines := bytes.Split(body, []byte("\n"))
	for _, line := range lines {
		if after, ok := bytes.CutPrefix(line, []byte("data: ")); ok {
			data := after
			data = bytes.TrimSpace(data)
			if len(data) > 0 && !bytes.Equal(data, []byte("{}")) {
				return data, nil
			}
		}
	}
	return nil, fmt.Errorf("no data found in event stream")
}

// isEventStream checks if the response is an event stream
func isEventStream(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return bytes.Contains([]byte(contentType), []byte("text/event-stream"))
}

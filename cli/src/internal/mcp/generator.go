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

package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	gwmodels "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"gopkg.in/yaml.v3"
)

const (
	JsonRpcVersion      = "2.0"
	ProtocolVersion     = "2025-06-18"
	ClientName          = "platform-gateway-client"
	ClientVersion       = "1.0.0"
	MethodInitialize    = "initialize"
	MethodInitialized   = "notifications/initialized"
	MethodToolsList     = "tools/list"
	MethodPromptsList   = "prompts/list"
	MethodResourcesList = "resources/list"
)

type JsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Intermediate types for unmarshaling JSON responses
type mcpToolJSON struct {
	Name         string          `json:"name"`
	Title        *string         `json:"title,omitempty"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
}

type mcpResourceJSON struct {
	Name        string  `json:"name"`
	Uri         string  `json:"uri"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	MimeType    *string `json:"mimeType,omitempty"`
	Size        *int    `json:"size,omitempty"`
}

type mcpPromptJSON struct {
	Name        string  `json:"name"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Arguments   *[]struct {
		Description *string `json:"description,omitempty"`
		Name        string  `json:"name"`
		Required    *bool   `json:"required,omitempty"`
		Title       *string `json:"title,omitempty"`
	} `json:"arguments,omitempty"`
}

type ToolsResult struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Result struct {
		Tools []mcpToolJSON `json:"tools"`
	} `json:"result"`
}

type PromptsResult struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Result struct {
		Prompts []mcpPromptJSON `json:"prompts"`
	} `json:"result"`
}

type ResourcesResult struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Result struct {
		Resources []mcpResourceJSON `json:"resources"`
	} `json:"result"`
}

// Generate generates MCP configuration from the given URL
func Generate(url string, outputDir string, headerName string, headerValue string) error {
	fmt.Printf("Generating MCP configuration for server: %s\n", url)

	// Step 1: initialize
	fmt.Println("→ Sending initialize...")
	sessionID, err := initializeMCPServer(url, headerName, headerValue)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP server: %w", err)
	}
	fmt.Println("---------------------------------------------------")

	// Step 2: notifications/initialized
	fmt.Println("→ Sending notifications/initialized...")
	notifyReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		Method:  MethodInitialized,
	}
	_, err = postJSONRPCWithSession(url, notifyReq, sessionID, headerName, headerValue)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	fmt.Println("---------------------------------------------------")

	// Step 3: tools/list
	fmt.Println("→ Sending tools/list...")
	toolsReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      2,
		Method:  MethodToolsList,
	}
	toolsResp, err := postJSONRPCWithSession(url, toolsReq, sessionID, headerName, headerValue)
	if err != nil {
		return fmt.Errorf("failed to get tools: %w", err)
	}
	var toolsResult ToolsResult
	if err := json.Unmarshal(toolsResp, &toolsResult); err != nil {
		return fmt.Errorf("failed to parse tools response: %w", err)
	}
	if toolsResult.Error != nil {
		return fmt.Errorf("tools/list request returned an error: %s", toolsResult.Error.Message)
	}
	fmt.Printf("→ Available Tools: %d\n", len(toolsResult.Result.Tools))
	fmt.Println("---------------------------------------------------")

	// Step 4: prompts/list
	fmt.Println("→ Sending prompts/list...")
	promptsReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      3,
		Method:  MethodPromptsList,
	}
	promptsResp, err := postJSONRPCWithSession(url, promptsReq, sessionID, headerName, headerValue)
	if err != nil {
		return fmt.Errorf("failed to get prompts: %w", err)
	}
	var promptsResult PromptsResult
	if err := json.Unmarshal(promptsResp, &promptsResult); err != nil {
		return fmt.Errorf("failed to parse prompts response: %w", err)
	}
	if promptsResult.Error != nil {
		return fmt.Errorf("prompts/list request returned an error: %s", promptsResult.Error.Message)
	}
	fmt.Printf("→ Available Prompts: %d\n", len(promptsResult.Result.Prompts))
	fmt.Println("---------------------------------------------------")

	// Step 5: resources/list
	fmt.Println("→ Sending resources/list...")
	resourcesReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      4,
		Method:  MethodResourcesList,
	}
	resourcesResp, err := postJSONRPCWithSession(url, resourcesReq, sessionID, headerName, headerValue)
	if err != nil {
		return fmt.Errorf("failed to get resources: %w", err)
	}
	var resourcesResult ResourcesResult
	if err := json.Unmarshal(resourcesResp, &resourcesResult); err != nil {
		return fmt.Errorf("failed to parse resources response: %w", err)
	}
	if resourcesResult.Error != nil {
		return fmt.Errorf("resources/list request returned an error: %s", resourcesResult.Error.Message)
	}
	fmt.Printf("→ Available Resources: %d\n", len(resourcesResult.Result.Resources))
	fmt.Println("---------------------------------------------------")

	// Generate MCP configuration file
	err = generateMCPConfigFile(url, toolsResult, resourcesResult, promptsResult, outputDir)
	if err != nil {
		return fmt.Errorf("failed to generate MCP configuration file: %w", err)
	}

	fmt.Println("MCP generated successfully.")
	return nil
}

// postJSONRPCWithSession sends a JSON-RPC request with mcp-session-id header if provided
func postJSONRPCWithSession(url string, req interface{}, sessionID string, headerName string, headerValue string) ([]byte, error) {
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
		if lower != "mcp-session-id" && lower != "content-type" && lower != "accept" {
			httpReq.Header.Set(headerName, headerValue)
		}
	}
	if sessionID != "" {
		httpReq.Header.Set("mcp-session-id", sessionID)
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check if response is event stream
	if isEventStream(resp) {
		// Extract JSON data from event stream
		data, err := parseEventStream(body)
		if err != nil {
			return body, nil // Return original body if parsing fails
		}
		return data, nil
	}

	return body, nil
}

// initializeMCPServer initializes the MCP server and returns the session ID
func initializeMCPServer(url string, headerName string, headerValue string) (string, error) {
	initReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      1,
		Method:  MethodInitialize,
		Params: map[string]interface{}{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]interface{}{"roots": map[string]bool{"listChanged": true}},
			"clientInfo":      map[string]string{"name": ClientName, "version": ClientVersion},
		},
	}
	data, err := json.Marshal(initReq)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("failed to create init request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if headerName != "" {
		lower := strings.ToLower(headerName)
		if lower != "mcp-session-id" && lower != "content-type" && lower != "accept" {
			httpReq.Header.Set(headerName, headerValue)
		}
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to reach MCP server for initialize: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if response is event stream and parse it
	if isEventStream(resp) {
		data, err := parseEventStream(body)
		if err == nil {
			body = data
		}
	}

	// Check for JSON-RPC error in response
	var initResponse map[string]interface{}
	if err := json.Unmarshal(body, &initResponse); err == nil {
		if errObj, hasError := initResponse["error"]; hasError {
			if errMap, ok := errObj.(map[string]interface{}); ok {
				if msg, ok := errMap["message"].(string); ok {
					return "", fmt.Errorf("initialize request returned an error: %s", msg)
				}
			}
			return "", fmt.Errorf("initialize request returned an error: %v", errObj)
		}
	}

	sessionID := getSessionIDFromResponse(resp)
	if sessionID == "" {
		fmt.Println("WARNING: No mcp-session-id received. Server might not support HTTP transport state.")
	} else {
		fmt.Printf("→ Captured Session ID: %s\n", sessionID)
	}
	return sessionID, nil
}

func getSessionIDFromResponse(resp *http.Response) string {
	return resp.Header.Get("mcp-session-id")
}

// parseEventStream extracts JSON data from event stream response
func parseEventStream(body []byte) ([]byte, error) {
	lines := bytes.Split(body, []byte("\n"))
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("data: ")) {
			data := bytes.TrimPrefix(line, []byte("data: "))
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

// generateMCPConfigFile generates the MCP configuration YAML file
func generateMCPConfigFile(url string, toolsResult ToolsResult,
	resourcesResult ResourcesResult, promptsResult PromptsResult, outputDir string) error {
	// Build YAML using gwmodels structs
	upstreamURL := strings.TrimSuffix(url, "/mcp")
	contextPath := "/generated"
	specVersion := ProtocolVersion

	// Convert intermediate types to gwmodels types
	tools := make([]gwmodels.MCPTool, 0, len(toolsResult.Result.Tools))
	for _, tool := range toolsResult.Result.Tools {
		// Convert json.RawMessage to JSON string
		inputSchemaStr := string(tool.InputSchema)
		var outputSchemaStr *string
		if len(tool.OutputSchema) > 0 {
			str := string(tool.OutputSchema)
			outputSchemaStr = &str
		}
		tools = append(tools, gwmodels.MCPTool{
			Name:         tool.Name,
			Title:        tool.Title,
			Description:  tool.Description,
			InputSchema:  inputSchemaStr,
			OutputSchema: outputSchemaStr,
		})
	}

	resources := make([]gwmodels.MCPResource, 0, len(resourcesResult.Result.Resources))
	for _, res := range resourcesResult.Result.Resources {
		resources = append(resources, gwmodels.MCPResource{
			Name:        res.Name,
			Uri:         res.Uri,
			Title:       res.Title,
			Description: res.Description,
			MimeType:    res.MimeType,
			Size:        res.Size,
		})
	}

	prompts := make([]gwmodels.MCPPrompt, 0, len(promptsResult.Result.Prompts))
	for _, prompt := range promptsResult.Result.Prompts {
		// Convert arguments if present to match gwmodels struct
		var args *[]struct {
			Description *string `json:"description,omitempty" yaml:"description,omitempty"`
			Name        string  `json:"name" yaml:"name"`
			Required    *bool   `json:"required,omitempty" yaml:"required,omitempty"`
			Title       *string `json:"title,omitempty" yaml:"title,omitempty"`
		}
		if prompt.Arguments != nil && len(*prompt.Arguments) > 0 {
			convertedArgs := make([]struct {
				Description *string `json:"description,omitempty" yaml:"description,omitempty"`
				Name        string  `json:"name" yaml:"name"`
				Required    *bool   `json:"required,omitempty" yaml:"required,omitempty"`
				Title       *string `json:"title,omitempty" yaml:"title,omitempty"`
			}, len(*prompt.Arguments))
			for i, arg := range *prompt.Arguments {
				convertedArgs[i].Description = arg.Description
				convertedArgs[i].Name = arg.Name
				convertedArgs[i].Required = arg.Required
				convertedArgs[i].Title = arg.Title
			}
			args = &convertedArgs
		}
		prompts = append(prompts, gwmodels.MCPPrompt{
			Name:        prompt.Name,
			Title:       prompt.Title,
			Description: prompt.Description,
			Arguments:   args,
		})
	}

	mcp := gwmodels.MCPProxyConfiguration{
		ApiVersion: gwmodels.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       gwmodels.Mcp,
		Metadata: gwmodels.Metadata{
			Name: "Generated-MCP-v1.0",
		},
		Spec: gwmodels.MCPProxyConfigData{
			DisplayName: "Generated-MCP",
			Version:     "v1.0",
			Context:     &contextPath,
			SpecVersion: &specVersion,
			Upstream: gwmodels.MCPProxyConfigData_Upstream{
				Url: &upstreamURL,
			},
			Policies:  nil,
			Tools:     &tools,
			Resources: &resources,
			Prompts:   &prompts,
		},
	}

	out, err := yaml.Marshal(&mcp)
	if err != nil {
		return err
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, "generated-mcp.yaml")
	err = os.WriteFile(outputPath, out, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("→ Generated MCP configuration YAML file: %s\n", outputPath)
	return nil
}

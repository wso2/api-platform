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

type Tool struct {
	Name         string          `json:"name"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
	Policies     []string        `json:"policies"`
}

// MarshalYAML customizes YAML marshaling to keep schemas as JSON strings
func (t Tool) MarshalYAML() (interface{}, error) {
	type ToolAlias struct {
		Name         string   `yaml:"name"`
		Title        string   `yaml:"title"`
		Description  string   `yaml:"description"`
		InputSchema  string   `yaml:"inputSchema"`
		OutputSchema string   `yaml:"outputSchema"`
		Policies     []string `yaml:"policies"`
	}

	return ToolAlias{
		Name:         t.Name,
		Title:        t.Title,
		Description:  t.Description,
		InputSchema:  string(t.InputSchema),
		OutputSchema: string(t.OutputSchema),
		Policies:     t.Policies,
	}, nil
}

type Resource struct {
	Name        string   `json:"name" yaml:"name"`
	URI         string   `json:"uri" yaml:"uri"`
	Description string   `json:"description" yaml:"description"`
	MimeType    string   `json:"mimeType" yaml:"mimeType"`
	Policies    []string `json:"policies" yaml:"policies"`
}

type Prompt struct {
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	Policies    []string `json:"policies" yaml:"policies"`
}

type McpYAML struct {
	Version string `yaml:"version"`
	Kind    string `yaml:"kind"`
	Spec    struct {
		Name        string `yaml:"name"`
		Version     string `yaml:"version"`
		Context     string `yaml:"context"`
		SpecVersion string `yaml:"specVersion"`
		Upstream    struct {
			URL string `yaml:"url"`
		} `yaml:"upstream"`
		Policies  []any      `yaml:"policies"`
		Tools     []Tool     `yaml:"tools"`
		Resources []Resource `yaml:"resources"`
		Prompts   []Prompt   `yaml:"prompts"`
	} `yaml:"spec"`
}

type ToolsResult struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Result struct {
		Tools []Tool `json:"tools"`
	} `json:"result"`
}

type PromptsResult struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Result struct {
		Prompts []Prompt `json:"prompts"`
	} `json:"result"`
}

type ResourcesResult struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Result struct {
		Resources []Resource `json:"resources"`
	} `json:"result"`
}

// Generate generates MCP configuration from the given URL
func Generate(url string, outputDir string) error {
	fmt.Printf("Generating MCP configuration for server: %s\n", url)

	// Step 1: initialize
	fmt.Println("→ Sending initialize...")
	sessionID, err := initializeMCPServer(url)
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
	_, err = postJSONRPCWithSession(url, notifyReq, sessionID)
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
	toolsResp, err := postJSONRPCWithSession(url, toolsReq, sessionID)
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
	promptsResp, err := postJSONRPCWithSession(url, promptsReq, sessionID)
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
	resourcesResp, err := postJSONRPCWithSession(url, resourcesReq, sessionID)
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
func postJSONRPCWithSession(url string, req interface{}, sessionID string) ([]byte, error) {
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
func initializeMCPServer(url string) (string, error) {
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
	// Build YAML
	mcp := McpYAML{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "mcp",
	}
	mcp.Spec.Name = "Generated-MCP"
	mcp.Spec.Version = "v1.0"
	mcp.Spec.Context = "/generated"
	mcp.Spec.SpecVersion = ProtocolVersion
	mcp.Spec.Upstream = struct {
		URL string `yaml:"url"`
	}{URL: strings.TrimSuffix(url, "/mcp")}
	mcp.Spec.Policies = []any{}
	mcp.Spec.Tools = toolsResult.Result.Tools
	mcp.Spec.Resources = resourcesResult.Result.Resources
	mcp.Spec.Prompts = promptsResult.Result.Prompts

	out, err := yaml.Marshal(&mcp)
	if err != nil {
		return err
	}

	outputPath := filepath.Join(outputDir, "generated-mcp.yaml")
	err = os.WriteFile(outputPath, out, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("→ Generated MCP configuration YAML file: %s\n", outputPath)
	return nil
}

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

	gwmodels "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
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

// mcpSession is the state the server hands back during initialization: the
// session id, which a stateless server leaves empty, and the protocol version it
// negotiated. Every request after initialization must echo that version in the
// MCP-Protocol-Version header, because a stateless server has nothing else to
// identify the connection with.
type mcpSession struct {
	id              string
	protocolVersion string
}

// Generate generates MCP configuration from the given URL
func Generate(url string, outputDir string, headerName string, headerValue string) error {
	fmt.Printf("Generating MCP configuration for server: %s\n", url)

	// Step 1: initialize
	fmt.Println("→ Sending initialize...")
	session, err := initializeMCPServer(url, headerName, headerValue)
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
	_, err = postJSONRPCWithSession(url, notifyReq, session, headerName, headerValue)
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
	toolsResp, err := postJSONRPCWithSession(url, toolsReq, session, headerName, headerValue)
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
	promptsResp, err := postJSONRPCWithSession(url, promptsReq, session, headerName, headerValue)
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
	resourcesResp, err := postJSONRPCWithSession(url, resourcesReq, session, headerName, headerValue)
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

// postJSONRPCWithSession sends a JSON-RPC request with the negotiated protocol
// version and the mcp-session-id header, when the session has them
func postJSONRPCWithSession(url string, req interface{}, session mcpSession, headerName string, headerValue string) ([]byte, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	// Match the response by its id: a server may write notifications on the same
	// stream before the answer, so the first data event is not always ours.
	var envelope struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if session.protocolVersion != "" {
		httpReq.Header.Set("MCP-Protocol-Version", session.protocolVersion)
	}
	// The caller's --header goes on last, so an explicitly requested protocol
	// version or content negotiation still wins over the defaults.
	if headerName != "" {
		lower := strings.ToLower(headerName)
		if lower != "mcp-session-id" && lower != "content-type" && lower != "accept" {
			httpReq.Header.Set(headerName, headerValue)
		}
	}
	if session.id != "" {
		httpReq.Header.Set("mcp-session-id", session.id)
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

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// A non-2xx answer is not a result, even when the body looks like one.
		if message, ok := responseErrorMessage(body, envelope.ID, isEventStream(resp)); ok {
			return nil, fmt.Errorf("%s request returned an error: %s", envelope.Method, message)
		}
		return nil, fmt.Errorf("%s request failed with status %s", envelope.Method, resp.Status)
	}

	// Check if response is event stream
	if isEventStream(resp) {
		if len(envelope.ID) == 0 {
			// A notification has no answer to wait for, so the stream is all there is.
			return body, nil
		}
		// Extract the event that answers this request from the event stream
		data, err := parseEventStreamResponse(body, envelope.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to read the response from the event stream: %w", err)
		}
		return data, nil
	}

	if len(envelope.ID) > 0 {
		var response struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to read the %s response: %w", envelope.Method, err)
		}
		if len(response.ID) > 0 && !bytes.Equal(compactJSON(response.ID), compactJSON(envelope.ID)) {
			return nil, fmt.Errorf("%s response does not answer request id %s", envelope.Method, compactJSON(envelope.ID))
		}
	}

	return body, nil
}

// initializeMCPServer initializes the MCP server and returns the session state
func initializeMCPServer(url string, headerName string, headerValue string) (mcpSession, error) {
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
		return mcpSession{}, err
	}
	idJSON, err := json.Marshal(initReq.ID)
	if err != nil {
		return mcpSession{}, fmt.Errorf("failed to encode the initialize request id: %w", err)
	}
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return mcpSession{}, fmt.Errorf("failed to create init request: %w", err)
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
		return mcpSession{}, fmt.Errorf("failed to reach MCP server for initialize: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcpSession{}, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// A failed initialize is not a server that merely omits the version: a
		// session is only worth creating when the call itself succeeded.
		if message, ok := responseErrorMessage(body, idJSON, isEventStream(resp)); ok {
			return mcpSession{}, fmt.Errorf("initialize request returned an error: %s", message)
		}
		return mcpSession{}, fmt.Errorf("initialize request failed with status %s", resp.Status)
	}

	// Check if response is event stream and parse it
	if isEventStream(resp) {
		data, err := parseEventStreamResponse(body, idJSON)
		if err != nil {
			return mcpSession{}, fmt.Errorf("failed to read the initialize response: %w", err)
		}
		body = data
	}

	// Check for JSON-RPC error in response and keep the negotiated version
	var initResponse map[string]interface{}
	protocolVersion := ""
	if message, isError := jsonRPCErrorMessage(body); isError {
		return mcpSession{}, fmt.Errorf("initialize request returned an error: %s", message)
	}
	// A 2xx body is not necessarily a response: a notification, an empty object
	// or broken JSON carries no result, and a session built on that has no server
	// capabilities behind it.
	if err := json.Unmarshal(body, &initResponse); err != nil {
		return mcpSession{}, fmt.Errorf("initialize response was not readable JSON: %w", err)
	}
	if got, hasID := initResponse["id"]; hasID {
		gotID, err := json.Marshal(got)
		if err != nil {
			return mcpSession{}, fmt.Errorf("initialize response carried an unreadable id: %w", err)
		}
		if !bytes.Equal(compactJSON(gotID), compactJSON(idJSON)) {
			return mcpSession{}, fmt.Errorf("initialize response does not answer request id %s", compactJSON(idJSON))
		}
	}
	result, hasResult := initResponse["result"].(map[string]interface{})
	if !hasResult {
		return mcpSession{}, fmt.Errorf("initialize response carried no result")
	}
	if raw, hasVersion := result["protocolVersion"]; hasVersion {
		version, isString := raw.(string)
		if !isString {
			return mcpSession{}, fmt.Errorf("initialize response carried a protocolVersion that is not a string: %T", raw)
		}
		protocolVersion = version
	}
	if protocolVersion == "" {
		// A server that reports no version has still been asked for one, so the
		// proposal is the best value later requests can carry. Say so rather than
		// sending it silently.
		fmt.Printf("WARNING: The initialize response did not report a protocol version. Falling back to %s.\n", ProtocolVersion)
		protocolVersion = ProtocolVersion
	}

	sessionID := getSessionIDFromResponse(resp)
	if sessionID == "" {
		fmt.Println("WARNING: No mcp-session-id received. Server might not support HTTP transport state.")
	} else {
		fmt.Printf("→ Captured Session ID: %s\n", sessionID)
	}
	return mcpSession{id: sessionID, protocolVersion: protocolVersion}, nil
}

func getSessionIDFromResponse(resp *http.Response) string {
	return resp.Header.Get("mcp-session-id")
}

// parseEventStreamResponse extracts the data event that answers the request with
// the given JSON-RPC id. A server may emit notifications (a logging message, a
// progress update) on the same stream before the response, and the first data
// event is then not the answer to the request. When no event carries an id at all,
// the first data event is returned, which is what a stream without ids produces.
//
// The id is compared as raw JSON so a string id matches as well as a number, and
// a data event split over several lines is read as one payload.
func parseEventStreamResponse(body []byte, id json.RawMessage) ([]byte, error) {
	want := compactJSON(id)
	var first []byte
	for _, event := range eventStreamData(body) {
		event = bytes.TrimSpace(event)
		if len(event) == 0 || bytes.Equal(event, []byte("{}")) {
			continue
		}
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(event, &envelope); err != nil {
			continue
		}
		// Only a response carries a result or an error. A server-to-client request
		// can repeat the id a client chose, and a notification has no result, so
		// neither is the answer to this call.
		if len(envelope.Result) == 0 && len(envelope.Error) == 0 {
			continue
		}
		if first == nil {
			first = event
		}
		if len(want) > 0 && bytes.Equal(compactJSON(envelope.ID), want) {
			return event, nil
		}
	}
	if len(want) > 0 {
		// Answering with some other event would hand the caller a result for a
		// different request, which is worse than saying the stream had none.
		return nil, fmt.Errorf("no event in the stream answers request id %s", want)
	}
	if first != nil {
		return first, nil
	}
	return nil, fmt.Errorf("no data found in event stream")
}

// eventStreamData returns the data payload of every event in the stream. The
// lines of one event are concatenated with a newline until a blank line ends it,
// as the event stream format specifies; event:, id: and retry: lines carry no
// payload.
func eventStreamData(body []byte) [][]byte {
	var events [][]byte
	var current []byte
	hasData := false
	flush := func() {
		if hasData {
			events = append(events, current)
			current = nil
			hasData = false
		}
	}
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if len(line) == 0 {
			flush()
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimPrefix(bytes.TrimPrefix(line, []byte("data:")), []byte(" "))
		if hasData {
			current = append(current, '\n')
		}
		hasData = true
		current = append(current, data...)
	}
	flush()
	return events
}

// compactJSON returns value without its insignificant whitespace, so two ids
// that differ only in formatting still compare equal. An unusable value becomes
// empty, which no event id matches.
func compactJSON(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, value); err != nil {
		return nil
	}
	return buf.Bytes()
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
		ApiVersion: gwmodels.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       gwmodels.MCPProxyConfigurationKindMcp,
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

// jsonRPCErrorMessage returns the message of a JSON-RPC error in body, if the
// body carries one. A server that rejects a request usually explains why in the
// error, which is more useful than the status code alone.
func jsonRPCErrorMessage(body []byte) (string, bool) {
	var response struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", false
	}
	if response.Error.Message == "" {
		return "", false
	}
	return response.Error.Message, true
}

// responseErrorMessage returns the message a failed response carries: the
// JSON-RPC error in the body, or in one of the events when the body is an event
// stream. The status code alone loses the reason the server gave.
func responseErrorMessage(body []byte, id json.RawMessage, stream bool) (string, bool) {
	if message, ok := jsonRPCErrorMessage(body); ok {
		return message, true
	}
	if !stream {
		return "", false
	}
	want := compactJSON(id)
	for _, event := range eventStreamData(body) {
		var envelope struct {
			ID    json.RawMessage `json:"id"`
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(event, &envelope); err != nil {
			continue
		}
		if envelope.Error.Message == "" {
			continue
		}
		if len(want) == 0 || bytes.Equal(compactJSON(envelope.ID), want) {
			return envelope.Error.Message, true
		}
	}
	return "", false
}

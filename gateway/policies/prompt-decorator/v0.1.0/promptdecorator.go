package promptdecorator

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
)

var arrayIndexRegex = regexp.MustCompile(`^([a-zA-Z0-9_]+)\[(-?\d+)\]$`)

// PromptDecoratorPolicy implements prompt decoration by applying custom decorations
type PromptDecoratorPolicy struct {
	params PromptDecoratorPolicyParams
}

type Decoration struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type PromptDecoratorConfig struct {
	// Decoration can be either:
	// 1. A string for text prompt decoration (e.g., "Summarize the following...")
	// 2. An array of Decoration objects for chat prompt decoration (e.g., [{"role": "system", "content": "..."}])
	Decoration interface{} `json:"decoration"`
}

type PromptDecoratorPolicyParams struct {
	PromptDecoratorConfig PromptDecoratorConfig
	JsonPath              string
	Append                bool
}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	p := &PromptDecoratorPolicy{}

	// Parse parameters
	policyParams, err := parseParams(params)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	p.params = policyParams

	slog.Debug("PromptDecorator: Policy initialized", "jsonPath", p.params.JsonPath, "append", p.params.Append)

	return p, nil
}

// parseParams parses and validates parameters from map to struct
func parseParams(params map[string]interface{}) (PromptDecoratorPolicyParams, error) {
	var result PromptDecoratorPolicyParams
	// Extract required promptDecoratorConfig parameter
	promptDecoratorConfigRaw, ok := params["promptDecoratorConfig"]
	if !ok {
		return result, fmt.Errorf("'promptDecoratorConfig' parameter is required")
	}

	var promptDecoratorConfig PromptDecoratorConfig
	switch v := promptDecoratorConfigRaw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &promptDecoratorConfig); err != nil {
			return result, fmt.Errorf("error unmarshaling promptDecoratorConfig: %w", err)
		}
	case map[string]interface{}:
		// Convert map to JSON and back to struct
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return result, fmt.Errorf("error marshaling promptDecoratorConfig: %w", err)
		}
		if err := json.Unmarshal(jsonBytes, &promptDecoratorConfig); err != nil {
			return result, fmt.Errorf("error unmarshaling promptDecoratorConfig: %w", err)
		}
	default:
		return result, fmt.Errorf("'promptDecoratorConfig' must be a JSON string or object")
	}

	// Validate decoration is not empty
	if promptDecoratorConfig.Decoration == nil {
		return result, fmt.Errorf("'promptDecoratorConfig.decoration' cannot be empty")
	}

	// Validate decoration format
	switch v := promptDecoratorConfig.Decoration.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return result, fmt.Errorf("'promptDecoratorConfig.decoration' cannot be empty when provided as string")
		}
	case []interface{}:
		if len(v) == 0 {
			return result, fmt.Errorf("'promptDecoratorConfig.decoration' cannot be empty when provided as array")
		}
		// Validate array elements
		for i, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if role, ok := itemMap["role"].(string); !ok || role == "" {
					return result, fmt.Errorf("'promptDecoratorConfig.decoration[%d].role' must be a non-empty string", i)
				}
				if content, ok := itemMap["content"].(string); !ok || content == "" {
					return result, fmt.Errorf("'promptDecoratorConfig.decoration[%d].content' must be a non-empty string", i)
				}
			} else {
				return result, fmt.Errorf("'promptDecoratorConfig.decoration[%d]' must be an object with 'role' and 'content' fields", i)
			}
		}
	default:
		return result, fmt.Errorf("'promptDecoratorConfig.decoration' must be a string or an array of objects")
	}

	result.PromptDecoratorConfig = promptDecoratorConfig

	// Extract required jsonPath parameter
	jsonPathRaw, ok := params["jsonPath"]
	if !ok {
		return result, fmt.Errorf("'jsonPath' parameter is required")
	}
	jsonPath, ok := jsonPathRaw.(string)
	if !ok {
		return result, fmt.Errorf("'jsonPath' must be a string")
	}
	if jsonPath == "" {
		return result, fmt.Errorf("'jsonPath' cannot be empty")
	}
	result.JsonPath = jsonPath

	// Extract optional append parameter
	if appendRaw, ok := params["append"]; ok {
		if appendVal, ok := appendRaw.(bool); ok {
			result.Append = appendVal
		} else {
			return result, fmt.Errorf("'append' must be a boolean")
		}
	}

	return result, nil
}

// Mode returns the processing mode for this policy
func (p *PromptDecoratorPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequest decorates request body
func (p *PromptDecoratorPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	var content []byte
	if ctx.Body != nil {
		content = ctx.Body.Content
	}

	// Check for empty or nil content before unmarshaling
	if ctx.Body == nil || len(content) == 0 {
		return p.buildErrorResponse("Empty request body", nil)
	}

	// Parse JSON payload
	var payloadData map[string]interface{}
	if err := json.Unmarshal(content, &payloadData); err != nil {
		slog.Debug("PromptDecorator: Error parsing JSON payload", "error", err)
		return p.buildErrorResponse("Error parsing JSON payload", err)
	}

	// Extract value using JSONPath
	extractedValue, err := utils.ExtractValueFromJsonpath(payloadData, p.params.JsonPath)
	if err != nil {
		slog.Debug("PromptDecorator: Error extracting value from JSONPath", "jsonPath", p.params.JsonPath, "error", err)
		return p.buildErrorResponse("Error extracting value from JSONPath", err)
	}

	// Check if we're decorating a string content field or an array of messages
	switch v := extractedValue.(type) {
	case string:
		// Decorating a content string (e.g., $.messages[-1].content)
		// Handle decoration - can be a string or array of objects
		var decorationStr string

		switch dec := p.params.PromptDecoratorConfig.Decoration.(type) {
		case string:
			// Simple string decoration (text prompt decoration mode)
			decorationStr = dec
		case []interface{}:
			// Array of decoration objects - extract content from each
			for _, item := range dec {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if content, ok := itemMap["content"].(string); ok && content != "" {
						if decorationStr != "" {
							decorationStr += "\n"
						}
						decorationStr += content
					}
				}
			}
		default:
			return p.buildErrorResponse("Invalid decoration format for string decoration", fmt.Errorf("decoration must be string or array"))
		}

		// Apply decoration (prepend or append)
		var updatedContent string
		if p.params.Append {
			updatedContent = v + " " + decorationStr
		} else {
			updatedContent = decorationStr + " " + v
		}

		slog.Debug("PromptDecorator: Applied string decoration", "jsonPath", p.params.JsonPath, "append", p.params.Append, "originalLength", len(v), "updatedLength", len(updatedContent))
		// Update the content field
		return p.updateStringAtPath(payloadData, p.params.JsonPath, updatedContent)

	case []interface{}:
		// Decorating an array of messages (e.g., $.messages)
		messages := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			if msg, ok := item.(map[string]interface{}); ok {
				messages = append(messages, msg)
			}
		}

		// Create decoration messages from decoration config
		decorationMessages, err := p.createDecorationMessages()
		if err != nil {
			slog.Debug("PromptDecorator: Error creating decoration messages", "error", err)
			return p.buildErrorResponse("Error creating decoration messages", err)
		}

		// Apply decoration (prepend or append)
		var updatedMessages []map[string]interface{}
		if p.params.Append {
			updatedMessages = append(messages, decorationMessages...)
		} else {
			updatedMessages = append(decorationMessages, messages...)
		}

		slog.Debug("PromptDecorator: Applied array decoration", "jsonPath", p.params.JsonPath, "append", p.params.Append, "originalCount", len(messages), "decorationCount", len(decorationMessages), "updatedCount", len(updatedMessages))
		// Update the messages array
		return p.updateArrayAtPath(payloadData, p.params.JsonPath, updatedMessages)

	case []map[string]interface{}:
		// Already in the right format
		messages := v

		// Create decoration messages from decoration config
		decorationMessages, err := p.createDecorationMessages()
		if err != nil {
			slog.Debug("PromptDecorator: Error creating decoration messages", "error", err)
			return p.buildErrorResponse("Error creating decoration messages", err)
		}

		// Apply decoration (prepend or append)
		var updatedMessages []map[string]interface{}
		if p.params.Append {
			updatedMessages = append(messages, decorationMessages...)
		} else {
			updatedMessages = append(decorationMessages, messages...)
		}

		slog.Debug("PromptDecorator: Applied array decoration", "jsonPath", p.params.JsonPath, "append", p.params.Append, "originalCount", len(messages), "decorationCount", len(decorationMessages), "updatedCount", len(updatedMessages))
		// Update the messages array
		return p.updateArrayAtPath(payloadData, p.params.JsonPath, updatedMessages)

	default:
		slog.Debug("PromptDecorator: Invalid extracted value type", "type", fmt.Sprintf("%T", extractedValue))
		return p.buildErrorResponse("Extracted value must be a string or an array of message objects", fmt.Errorf("unexpected type: %T", extractedValue))
	}
}

// createDecorationMessages creates decoration messages from the decoration config
// For chat prompt decoration, decoration must be an array of objects with role and content
func (p *PromptDecoratorPolicy) createDecorationMessages() ([]map[string]interface{}, error) {
	decoration := p.params.PromptDecoratorConfig.Decoration

	switch dec := decoration.(type) {
	case []interface{}:
		// Array of decoration objects (chat prompt decoration mode)
		// parseParams guarantees valid role/content, so this routine only converts
		// the already-validated objects into the expected message format
		decorationMessages := make([]map[string]interface{}, 0, len(dec))
		for i, item := range dec {
			if itemMap, ok := item.(map[string]interface{}); ok {
				// Type assertions are safe because parseParams validates role/content are non-empty strings
				role := itemMap["role"].(string)
				content := itemMap["content"].(string)

				decorationMessages = append(decorationMessages, map[string]interface{}{
					"role":    role,
					"content": content,
				})
			} else {
				return nil, fmt.Errorf("decoration[%d] must be an object with 'role' and 'content' fields", i)
			}
		}
		return decorationMessages, nil
	default:
		return nil, fmt.Errorf("decoration must be an array of objects with 'role' and 'content' for chat prompt decoration")
	}
}

// updateStringAtPath updates a string value at the given JSONPath
func (p *PromptDecoratorPolicy) updateStringAtPath(payloadData map[string]interface{}, jsonPath string, value string) policy.RequestAction {
	path := jsonPath
	if strings.HasPrefix(path, "$.") {
		path = strings.TrimPrefix(path, "$.")
	}
	if path == "" {
		return p.buildErrorResponse("Invalid JSONPath", fmt.Errorf("empty path"))
	}

	pathComponents := strings.Split(path, ".")
	current := interface{}(payloadData)

	// Navigate to parent
	for i := 0; i < len(pathComponents)-1; i++ {
		key := pathComponents[i]
		current = p.navigatePath(current, key)
		if current == nil {
			slog.Debug("PromptDecorator: Error navigating JSONPath", "jsonPath", jsonPath, "key", key)
			return p.buildErrorResponse("Error navigating JSONPath", fmt.Errorf("key not found: %s", key))
		}
	}

	// Update final key
	finalKey := pathComponents[len(pathComponents)-1]
	if err := p.setValueAtPath(current, finalKey, value); err != nil {
		slog.Debug("PromptDecorator: Error updating JSONPath", "jsonPath", jsonPath, "error", err)
		return p.buildErrorResponse("Error updating JSONPath", err)
	}

	updatedPayload, err := json.Marshal(payloadData)
	if err != nil {
		slog.Debug("PromptDecorator: Error marshaling updated JSON payload", "error", err)
		return p.buildErrorResponse("Error marshaling updated JSON payload", err)
	}

	return policy.UpstreamRequestModifications{
		Body: updatedPayload,
	}
}

// updateArrayAtPath updates an array value at the given JSONPath
func (p *PromptDecoratorPolicy) updateArrayAtPath(payloadData map[string]interface{}, jsonPath string, value []map[string]interface{}) policy.RequestAction {
	path := jsonPath
	if strings.HasPrefix(path, "$.") {
		path = strings.TrimPrefix(path, "$.")
	}
	if path == "" {
		return p.buildErrorResponse("Invalid JSONPath", fmt.Errorf("empty path"))
	}

	pathComponents := strings.Split(path, ".")
	current := interface{}(payloadData)

	// Navigate to parent
	for i := 0; i < len(pathComponents)-1; i++ {
		key := pathComponents[i]
		current = p.navigatePath(current, key)
		if current == nil {
			slog.Debug("PromptDecorator: Error navigating JSONPath", "jsonPath", jsonPath, "key", key)
			return p.buildErrorResponse("Error navigating JSONPath", fmt.Errorf("key not found: %s", key))
		}
	}

	// Convert []map[string]interface{} to []interface{}
	valueInterface := make([]interface{}, len(value))
	for i, v := range value {
		valueInterface[i] = v
	}

	// Update final key
	finalKey := pathComponents[len(pathComponents)-1]
	if err := p.setValueAtPath(current, finalKey, valueInterface); err != nil {
		slog.Debug("PromptDecorator: Error updating JSONPath", "jsonPath", jsonPath, "error", err)
		return p.buildErrorResponse("Error updating JSONPath", err)
	}

	updatedPayload, err := json.Marshal(payloadData)
	if err != nil {
		slog.Debug("PromptDecorator: Error marshaling updated JSON payload", "error", err)
		return p.buildErrorResponse("Error marshaling updated JSON payload", err)
	}

	return policy.UpstreamRequestModifications{
		Body: updatedPayload,
	}
}

// navigatePath navigates through a JSON structure using a key (which may contain array indices)
func (p *PromptDecoratorPolicy) navigatePath(current interface{}, key string) interface{} {
	if matches := arrayIndexRegex.FindStringSubmatch(key); len(matches) == 3 {
		arrayName := matches[1]
		idxStr := matches[2]
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			return nil
		}

		if node, ok := current.(map[string]interface{}); ok {
			if arrVal, exists := node[arrayName]; exists {
				if arr, ok := arrVal.([]interface{}); ok {
					if idx < 0 {
						idx = len(arr) + idx
					}
					if idx < 0 || idx >= len(arr) {
						return nil
					}
					return arr[idx]
				}
			}
		}
		return nil
	}

	if node, ok := current.(map[string]interface{}); ok {
		if val, exists := node[key]; exists {
			return val
		}
	}
	return nil
}

// setValueAtPath sets a value at a path (key may contain array indices)
func (p *PromptDecoratorPolicy) setValueAtPath(current interface{}, key string, value interface{}) error {
	if matches := arrayIndexRegex.FindStringSubmatch(key); len(matches) == 3 {
		arrayName := matches[1]
		idxStr := matches[2]
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			return fmt.Errorf("invalid array index: %s", idxStr)
		}

		if node, ok := current.(map[string]interface{}); ok {
			if arrVal, exists := node[arrayName]; exists {
				if arr, ok := arrVal.([]interface{}); ok {
					if idx < 0 {
						idx = len(arr) + idx
					}
					if idx < 0 || idx >= len(arr) {
						return fmt.Errorf("array index out of range: %s", idxStr)
					}
					arr[idx] = value
					return nil
				}
				return fmt.Errorf("not an array: %s", arrayName)
			}
			return fmt.Errorf("key not found: %s", arrayName)
		}
		return fmt.Errorf("invalid structure for key: %s", arrayName)
	}

	if node, ok := current.(map[string]interface{}); ok {
		node[key] = value
		return nil
	}
	return fmt.Errorf("invalid structure for key: %s", key)
}

// OnResponse is not used for this policy
func (p *PromptDecoratorPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return policy.UpstreamResponseModifications{}
}

// buildErrorResponse builds an error response
func (p *PromptDecoratorPolicy) buildErrorResponse(reason string, validationError error) policy.RequestAction {
	errorMessage := reason
	if validationError != nil {
		errorMessage = fmt.Sprintf("%s: %v", reason, validationError)
	}

	responseBody := map[string]interface{}{
		"type":    "PROMPT_DECORATOR_ERROR",
		"message": errorMessage,
	}

	bodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		bodyBytes = []byte(`{"type":"PROMPT_DECORATOR_ERROR","message":"Internal error"}`)
	}

	return policy.ImmediateResponse{
		StatusCode: 500,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: bodyBytes,
	}
}

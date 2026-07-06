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

package openaitoanthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

const (
	PolicyName                  = "openai-to-anthropic"
	AnthropicMessagesPath       = "/v1/messages"
	DefaultAnthropicVersion     = "2023-06-01"
	DefaultMaxTokens            = 4096
	MetadataKeySelectedProvider = "selected_provider"
)

type PolicyParams struct {
	Model            string
	AnthropicVersion string
	Id string
}

type TranslatorPolicy struct {
	params PolicyParams
}

func GetPolicy(_ policy.PolicyMetadata, rawParams map[string]interface{}) (policy.Policy, error) {
	parsed, err := parseParams(rawParams)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid params: %w", PolicyName, err)
	}
	return &TranslatorPolicy{params: parsed}, nil
}

func (p *TranslatorPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

func (p *TranslatorPolicy) OnRequestBody(
	_ context.Context,
	reqCtx *policy.RequestContext,
	_ map[string]interface{},
) policy.RequestAction {
	if !p.shouldRun(reqCtx) {
		return policy.UpstreamRequestModifications{}
	}

	if reqCtx.Body == nil || !reqCtx.Body.Present || len(reqCtx.Body.Content) == 0 {
		return errResponse(400, "Request body is empty.")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(reqCtx.Body.Content, &payload); err != nil {
		return errResponse(400, fmt.Sprintf("Invalid JSON in request body: %s", err.Error()))
	}

	model := p.params.Model
	if model == "" {
		return errResponse(400, "'model' policy parameter is required for Anthropic translation.")
	}

	slog.Debug(PolicyName+": translating request",
		"id", p.params.Id, "model", model, "path", AnthropicMessagesPath)

	mods := translateBody(payload, model, p.params)
	if p.params.Id != "" && mods.UpstreamName == nil {
		upstream := p.params.Id
		mods.UpstreamName = &upstream
	}
	return mods
}

// OnResponseBody translates the Anthropic Messages response into an OpenAI
// ChatCompletion-shaped body. Streaming SSE responses are passed through
// untouched — translating SSE requires a stateful chunk-level policy.
func (p *TranslatorPolicy) OnResponseBody(
	_ context.Context,
	respCtx *policy.ResponseContext,
	_ map[string]interface{},
) policy.ResponseAction {
	if !p.shouldRunResponse(respCtx) {
		return policy.DownstreamResponseModifications{}
	}

	if respCtx.ResponseBody == nil || !respCtx.ResponseBody.Present || len(respCtx.ResponseBody.Content) == 0 {
		return policy.DownstreamResponseModifications{}
	}

	body := respCtx.ResponseBody.Content
	if looksLikeSSE(body) {
		slog.Debug(PolicyName+": SSE response passthrough", "status", respCtx.ResponseStatus)
		return policy.DownstreamResponseModifications{}
	}

	slog.Debug(PolicyName+": translating response", "status", respCtx.ResponseStatus)
	return translateResponse(body, respCtx.ResponseStatus, p.params.Model)
}

func (p *TranslatorPolicy) shouldRun(reqCtx *policy.RequestContext) bool {
	return p.shouldRunForSelected(selectedProviderFromMetadata(reqCtx.SharedContext, reqCtx.Metadata))
}

func (p *TranslatorPolicy) shouldRunResponse(respCtx *policy.ResponseContext) bool {
	return p.shouldRunForSelected(selectedProviderFromMetadata(respCtx.SharedContext, respCtx.Metadata))
}

func (p *TranslatorPolicy) shouldRunForSelected(selected string) bool {
	if selected == "" {
		// Single-provider mode: no router selected a provider, so run.
		return true
	}
	return strings.EqualFold(selected, p.params.Id)
}

func selectedProviderFromMetadata(shared *policy.SharedContext, metadata map[string]interface{}) string {
	if shared == nil || metadata == nil {
		return ""
	}
	rawSelectedProvider, hasSelectedProvider := metadata[MetadataKeySelectedProvider]
	if !hasSelectedProvider {
		return ""
	}
	selectedProvider, isString := rawSelectedProvider.(string)
	if !isString {
		return ""
	}
	return strings.TrimSpace(selectedProvider)
}

func parseParams(params map[string]interface{}) (PolicyParams, error) {
	result := PolicyParams{AnthropicVersion: DefaultAnthropicVersion}

	model, err := optionalString(params, "model")
	if err != nil {
		return result, err
	}
	if model == "" {
		return result, fmt.Errorf("'model' is required")
	}
	result.Model = model

	if id, err := optionalString(params, "id"); err != nil {
		return result, err
	} else {
		result.Id = id
	}

	if anthropicVersion, err := optionalString(params, "anthropicVersion"); err != nil {
		return result, err
	} else if anthropicVersion != "" {
		result.AnthropicVersion = anthropicVersion
	}

	return result, nil
}

func optionalString(params map[string]interface{}, key string) (string, error) {
	rawValue, hasValue := params[key]
	if !hasValue || rawValue == nil {
		return "", nil
	}
	value, isString := rawValue.(string)
	if !isString {
		return "", fmt.Errorf("'%s' must be a string", key)
	}
	return strings.TrimSpace(value), nil
}

func errResponse(statusCode int, message string) policy.ImmediateResponse {
	body, _ := json.Marshal(map[string]string{"error": message})
	return policy.ImmediateResponse{
		StatusCode: statusCode,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}
}

// translateBody converts an OpenAI Chat Completions request payload into an
// Anthropic Messages request and returns the upstream modifications.
func translateBody(
	payload map[string]interface{}, model string, params PolicyParams,
) policy.UpstreamRequestModifications {
	anthropicBody := map[string]interface{}{"model": model}

	if messages, hasMessages := payload["messages"].([]interface{}); hasMessages {
		systemText, anthropicMessages := convertMessages(messages)
		if systemText != "" {
			anthropicBody["system"] = systemText
		}
		if len(anthropicMessages) > 0 {
			anthropicBody["messages"] = anthropicMessages
		}
	}

	// Anthropic requires max_tokens; OpenAI's newer max_completion_tokens wins
	// over legacy max_tokens, matching OpenAI's own precedence.
	if maxCompletionTokens, hasMaxCompletionTokens := payload["max_completion_tokens"]; hasMaxCompletionTokens {
		anthropicBody["max_tokens"] = maxCompletionTokens
	} else if maxTokens, hasMaxTokens := payload["max_tokens"]; hasMaxTokens {
		anthropicBody["max_tokens"] = maxTokens
	} else {
		anthropicBody["max_tokens"] = DefaultMaxTokens
	}

	if temperature, hasTemperature := payload["temperature"]; hasTemperature {
		anthropicBody["temperature"] = temperature
	}
	if topP, hasTopP := payload["top_p"]; hasTopP {
		anthropicBody["top_p"] = topP
	}
	if stream, hasStream := payload["stream"]; hasStream {
		anthropicBody["stream"] = stream
	}

	if stop, hasStop := payload["stop"]; hasStop && stop != nil {
		switch stopValue := stop.(type) {
		case string:
			anthropicBody["stop_sequences"] = []string{stopValue}
		case []interface{}:
			anthropicBody["stop_sequences"] = stopValue
		}
	}

	if tools, hasTools := payload["tools"].([]interface{}); hasTools && len(tools) > 0 {
		if anthropicTools := convertTools(tools); len(anthropicTools) > 0 {
			anthropicBody["tools"] = anthropicTools
		}
	}

	// "none" has no Anthropic equivalent — drop tools entirely so the model
	// cannot call any.
	if toolChoice, hasToolChoice := payload["tool_choice"]; hasToolChoice && toolChoice != nil {
		if anthropicToolChoice := convertToolChoice(toolChoice); anthropicToolChoice == nil {
			delete(anthropicBody, "tools")
		} else {
			anthropicBody["tool_choice"] = anthropicToolChoice
		}
	}

	newBody, err := json.Marshal(anthropicBody)
	if err != nil {
		return policy.UpstreamRequestModifications{
			Body: []byte(fmt.Sprintf(`{"error":"failed to marshal Anthropic body: %s"}`, err.Error())),
		}
	}

	newPath := AnthropicMessagesPath
	return policy.UpstreamRequestModifications{
		Body: newBody,
		Path: &newPath,
		HeadersToSet: map[string]string{
			"content-type":      "application/json",
			"anthropic-version": params.AnthropicVersion,
		},
	}
}

// convertMessages extracts system text and rewrites messages into Anthropic
// role/content blocks. Consecutive tool messages collapse into one user
// message of tool_result blocks (Anthropic groups them together).
func convertMessages(messages []interface{}) (string, []map[string]interface{}) {
	var systemParts []string
	var anthropicMessages []map[string]interface{}

	for messageIndex := 0; messageIndex < len(messages); messageIndex++ {
		message, isObject := messages[messageIndex].(map[string]interface{})
		if !isObject {
			continue
		}
		role, _ := message["role"].(string)

		switch role {
		case "system", "developer":
			if text := extractTextContent(message["content"]); text != "" {
				systemParts = append(systemParts, text)
			}
		case "user":
			anthropicMessages = append(anthropicMessages, map[string]interface{}{
				"role":    "user",
				"content": convertUserContent(message["content"]),
			})
		case "assistant":
			anthropicMessages = append(anthropicMessages, map[string]interface{}{
				"role":    "assistant",
				"content": convertAssistantContent(message),
			})
		case "tool":
			var toolResults []interface{}
			for messageIndex < len(messages) {
				toolMessage, isObject := messages[messageIndex].(map[string]interface{})
				if !isObject {
					break
				}
				if toolMessageRole, _ := toolMessage["role"].(string); toolMessageRole != "tool" {
					break
				}
				toolCallID, _ := toolMessage["tool_call_id"].(string)
				toolResults = append(toolResults, map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": toolCallID,
					"content":     extractTextContent(toolMessage["content"]),
				})
				messageIndex++
			}
			// Outer loop will increment; rewind so we revisit the terminator.
			messageIndex--
			anthropicMessages = append(anthropicMessages, map[string]interface{}{
				"role":    "user",
				"content": toolResults,
			})
		}
	}

	return strings.Join(systemParts, "\n"), anthropicMessages
}

func extractTextContent(content interface{}) string {
	switch typedContent := content.(type) {
	case string:
		return typedContent
	case []interface{}:
		var parts []string
		for _, contentPart := range typedContent {
			contentBlock, isObject := contentPart.(map[string]interface{})
			if !isObject || contentBlock["type"] != "text" {
				continue
			}
			if text, isString := contentBlock["text"].(string); isString {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	}
	return ""
}

func convertUserContent(content interface{}) interface{} {
	switch typedContent := content.(type) {
	case string:
		return typedContent
	case []interface{}:
		var blocks []interface{}
		for _, contentPart := range typedContent {
			contentBlock, isObject := contentPart.(map[string]interface{})
			if !isObject {
				continue
			}
			switch contentBlock["type"].(string) {
			case "text":
				blocks = append(blocks, map[string]interface{}{
					"type": "text",
					"text": contentBlock["text"],
				})
			case "image_url":
				if block := convertImage(contentBlock); block != nil {
					blocks = append(blocks, block)
				}
			}
		}
		if len(blocks) > 0 {
			return blocks
		}
		return content
	}
	return content
}

func convertAssistantContent(message map[string]interface{}) interface{} {
	toolCalls, hasToolCalls := message["tool_calls"].([]interface{})
	textContent := extractTextContent(message["content"])

	if !hasToolCalls || len(toolCalls) == 0 {
		if textContent != "" {
			return textContent
		}
		return message["content"]
	}

	var blocks []interface{}
	if textContent != "" {
		blocks = append(blocks, map[string]interface{}{
			"type": "text",
			"text": textContent,
		})
	}

	for _, rawToolCall := range toolCalls {
		toolCall, isObject := rawToolCall.(map[string]interface{})
		if !isObject {
			continue
		}
		id, _ := toolCall["id"].(string)
		functionDefinition, _ := toolCall["function"].(map[string]interface{})
		if functionDefinition == nil {
			continue
		}
		name, _ := functionDefinition["name"].(string)
		argumentsString, _ := functionDefinition["arguments"].(string)

		// OpenAI wire format is a JSON string; Anthropic wants the decoded
		// object inline. Fall back to empty object on parse failure so the
		// request still validates.
		var input interface{}
		if argumentsString != "" {
			if err := json.Unmarshal([]byte(argumentsString), &input); err != nil {
				input = map[string]interface{}{}
			}
		} else {
			input = map[string]interface{}{}
		}
		blocks = append(blocks, map[string]interface{}{
			"type":  "tool_use",
			"id":    id,
			"name":  name,
			"input": input,
		})
	}

	return blocks
}

func convertImage(contentBlock map[string]interface{}) map[string]interface{} {
	imageObject, isObject := contentBlock["image_url"].(map[string]interface{})
	if !isObject {
		return nil
	}
	imageURL, isString := imageObject["url"].(string)
	if !isString || imageURL == "" {
		return nil
	}

	if strings.HasPrefix(imageURL, "data:") {
		parts := strings.SplitN(imageURL, ",", 2)
		if len(parts) != 2 {
			return nil
		}
		meta := strings.TrimPrefix(parts[0], "data:")
		metaParts := strings.SplitN(meta, ";", 2)
		return map[string]interface{}{
			"type": "image",
			"source": map[string]interface{}{
				"type":       "base64",
				"media_type": metaParts[0],
				"data":       parts[1],
			},
		}
	}

	return map[string]interface{}{
		"type": "image",
		"source": map[string]interface{}{
			"type": "url",
			"url":  imageURL,
		},
	}
}

func convertTools(tools []interface{}) []map[string]interface{} {
	var result []map[string]interface{}
	for _, rawTool := range tools {
		tool, isObject := rawTool.(map[string]interface{})
		if !isObject {
			continue
		}
		functionDefinition, isObject := tool["function"].(map[string]interface{})
		if !isObject {
			continue
		}
		anthropicTool := map[string]interface{}{"name": functionDefinition["name"]}
		if description, hasDescription := functionDefinition["description"]; hasDescription {
			anthropicTool["description"] = description
		}
		if parameters, hasParameters := functionDefinition["parameters"]; hasParameters {
			anthropicTool["input_schema"] = parameters
		} else {
			anthropicTool["input_schema"] = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}
		result = append(result, anthropicTool)
	}
	return result
}

// convertToolChoice returns nil for "none" — the caller drops tools[] entirely
// since Anthropic has no negative tool_choice form.
func convertToolChoice(toolChoice interface{}) interface{} {
	switch typedToolChoice := toolChoice.(type) {
	case string:
		switch typedToolChoice {
		case "auto":
			return map[string]interface{}{"type": "auto"}
		case "none":
			return nil
		case "required":
			return map[string]interface{}{"type": "any"}
		}
	case map[string]interface{}:
		if functionDefinition, hasFunction := typedToolChoice["function"].(map[string]interface{}); hasFunction {
			if name, hasName := functionDefinition["name"].(string); hasName {
				return map[string]interface{}{"type": "tool", "name": name}
			}
		}
	}
	return map[string]interface{}{"type": "auto"}
}

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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

const (
	anthropicStopEndTurn      = "end_turn"
	anthropicStopMaxTokens    = "max_tokens"
	anthropicStopStopSequence = "stop_sequence"
	anthropicStopToolUse      = "tool_use"

	openaiFinishStop      = "stop"
	openaiFinishLength    = "length"
	openaiFinishToolCalls = "tool_calls"
)

type anthropicMessage struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Model        string                  `json:"model"`
	Content      []anthropicContentBlock `json:"content"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence,omitempty"`
	Usage        anthropicUsage          `json:"usage"`
}

type anthropicContentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
}

type anthropicErrorBody struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func translateResponse(respBody []byte, status int, requestModel string) policy.ResponseAction {
	if status >= 200 && status < 300 {
		return translateSuccessResponse(respBody, requestModel)
	}
	return translateErrorResponse(respBody, status)
}

func translateSuccessResponse(respBody []byte, requestModel string) policy.ResponseAction {
	var msg anthropicMessage
	if err := json.Unmarshal(respBody, &msg); err != nil {
		// Upstream returned 2xx but body isn't Anthropic JSON — forward it
		// unchanged rather than masking the payload behind a 500.
		return policy.DownstreamResponseModifications{}
	}

	responseModel := msg.Model
	if responseModel == "" {
		responseModel = requestModel
	}

	openAIResp := buildOpenAIChatCompletion(&msg, responseModel)
	newBody, err := json.Marshal(openAIResp)
	if err != nil {
		return errResponse(500, "failed to translate Anthropic response: "+err.Error())
	}

	return policy.DownstreamResponseModifications{
		Body: newBody,
		HeadersToSet: map[string]string{
			"content-type":   "application/json",
			"content-length": fmt.Sprintf("%d", len(newBody)),
		},
	}
}

func translateErrorResponse(respBody []byte, status int) policy.ResponseAction {
	openaiErr := map[string]interface{}{
		"error": map[string]interface{}{
			"type":    mapStatusToOpenAIErrorType(status),
			"message": string(respBody),
			"code":    fmt.Sprintf("%d", status),
		},
	}

	var anthropicErr anthropicErrorBody
	if err := json.Unmarshal(respBody, &anthropicErr); err == nil && anthropicErr.Error.Message != "" {
		errMap := openaiErr["error"].(map[string]interface{})
		if anthropicErr.Error.Type != "" {
			errMap["type"] = anthropicErr.Error.Type
		}
		errMap["message"] = anthropicErr.Error.Message
	}

	newBody, err := json.Marshal(openaiErr)
	if err != nil {
		return errResponse(500, "failed to translate Anthropic error: "+err.Error())
	}

	return policy.DownstreamResponseModifications{
		Body: newBody,
		HeadersToSet: map[string]string{
			"content-type":   "application/json",
			"content-length": fmt.Sprintf("%d", len(newBody)),
		},
	}
}

func buildOpenAIChatCompletion(msg *anthropicMessage, responseModel string) map[string]interface{} {
	var textParts []string
	var toolCalls []map[string]interface{}

	for i := range msg.Content {
		block := &msg.Content[i]
		switch block.Type {
		case "text":
			if block.Text != "" {
				textParts = append(textParts, block.Text)
			}
		case "tool_use":
			toolCalls = append(toolCalls, buildOpenAIToolCall(block))
		}
	}

	message := map[string]interface{}{
		"role":    "assistant",
		"content": strings.Join(textParts, ""),
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}

	choice := map[string]interface{}{
		"index":         0,
		"message":       message,
		"finish_reason": stopReasonToFinish(msg.StopReason, len(toolCalls) > 0),
	}

	usage := map[string]interface{}{
		"prompt_tokens":     msg.Usage.InputTokens,
		"completion_tokens": msg.Usage.OutputTokens,
		"total_tokens":      msg.Usage.InputTokens + msg.Usage.OutputTokens,
	}
	if msg.Usage.CacheReadInputTokens > 0 || msg.Usage.CacheCreationInputTokens > 0 {
		usage["prompt_tokens_details"] = map[string]interface{}{
			"cached_tokens":         msg.Usage.CacheReadInputTokens,
			"cache_creation_tokens": msg.Usage.CacheCreationInputTokens,
		}
	}

	id := msg.ID
	if id == "" {
		id = newChatCompletionID()
	} else {
		// Rewrite Anthropic "msg_*" id to OpenAI "chatcmpl-*" so prefix-aware
		// clients still match.
		id = "chatcmpl-" + strings.TrimPrefix(id, "msg_")
	}

	return map[string]interface{}{
		"id":      id,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   responseModel,
		"choices": []map[string]interface{}{choice},
		"usage":   usage,
	}
}

func buildOpenAIToolCall(block *anthropicContentBlock) map[string]interface{} {
	argsBytes, err := json.Marshal(block.Input)
	argsStr := string(argsBytes)
	if err != nil || block.Input == nil {
		argsStr = "{}"
	}
	return map[string]interface{}{
		"id":   block.ID,
		"type": "function",
		"function": map[string]interface{}{
			"name":      block.Name,
			"arguments": argsStr,
		},
	}
}

func stopReasonToFinish(stopReason string, hasToolCalls bool) string {
	if hasToolCalls {
		return openaiFinishToolCalls
	}
	switch stopReason {
	case anthropicStopMaxTokens:
		return openaiFinishLength
	case anthropicStopToolUse:
		return openaiFinishToolCalls
	case anthropicStopEndTurn, anthropicStopStopSequence:
		return openaiFinishStop
	default:
		return openaiFinishStop
	}
}

func mapStatusToOpenAIErrorType(status int) string {
	switch {
	case status == 400:
		return "invalid_request_error"
	case status == 401:
		return "authentication_error"
	case status == 403:
		return "permission_error"
	case status == 404:
		return "not_found_error"
	case status == 413:
		return "request_too_large"
	case status == 429:
		return "rate_limit_error"
	case status >= 500:
		return "server_error"
	default:
		return "api_error"
	}
}

func newChatCompletionID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	}
	return "chatcmpl-" + hex.EncodeToString(b[:])
}

func looksLikeSSE(body []byte) bool {
	trimmed := strings.TrimLeft(string(body), " \r\n\t")
	return strings.HasPrefix(trimmed, "event:") || strings.HasPrefix(trimmed, "data:")
}

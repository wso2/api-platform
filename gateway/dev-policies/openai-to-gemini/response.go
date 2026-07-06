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

package openaitogemini

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
	geminiFinishStop        = "STOP"
	geminiFinishMaxTokens   = "MAX_TOKENS"
	geminiFinishSafety      = "SAFETY"
	geminiFinishRecitation  = "RECITATION"
	geminiFinishOther       = "OTHER"
	geminiFinishBlocklist   = "BLOCKLIST"
	geminiFinishProhibited  = "PROHIBITED_CONTENT"
	geminiFinishSPII        = "SPII"
	geminiFinishMalfunction = "MALFORMED_FUNCTION_CALL"
	geminiFinishImageRecit  = "IMAGE_SAFETY"
	geminiFinishLanguage    = "LANGUAGE"
	geminiFinishUnspecified = "FINISH_REASON_UNSPECIFIED"

	openaiFinishStop          = "stop"
	openaiFinishLength        = "length"
	openaiFinishToolCalls     = "tool_calls"
	openaiFinishContentFilter = "content_filter"
)

type geminiResponse struct {
	Candidates     []geminiCandidate     `json:"candidates"`
	UsageMetadata  *geminiUsageMetadata  `json:"usageMetadata,omitempty"`
	ModelVersion   string                `json:"modelVersion,omitempty"`
	ResponseID     string                `json:"responseId,omitempty"`
	PromptFeedback *geminiPromptFeedback `json:"promptFeedback,omitempty"`
}

type geminiCandidate struct {
	Content      *geminiContent `json:"content,omitempty"`
	FinishReason string         `json:"finishReason,omitempty"`
	Index        int            `json:"index,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts,omitempty"`
}

type geminiPart struct {
	Text         string              `json:"text,omitempty"`
	Thought      bool                `json:"thought,omitempty"`
	FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
}

type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type geminiUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	TotalTokenCount         int `json:"totalTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount,omitempty"`
}

type geminiPromptFeedback struct {
	BlockReason string `json:"blockReason,omitempty"`
}

type geminiErrorBody struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func translateResponse(respBody []byte, status int, requestModel string) policy.ResponseAction {
	if status >= 200 && status < 300 {
		return translateSuccessResponse(respBody, requestModel)
	}
	return translateErrorResponse(respBody, status)
}

func translateSuccessResponse(respBody []byte, requestModel string) policy.ResponseAction {
	var resp geminiResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		// Upstream returned 2xx but body isn't Gemini JSON — forward it
		// unchanged rather than masking the payload behind a 500.
		return policy.DownstreamResponseModifications{}
	}

	responseModel := resp.ModelVersion
	if responseModel == "" {
		responseModel = requestModel
	}

	openAIResp := buildOpenAIChatCompletion(&resp, responseModel)
	newBody, err := json.Marshal(openAIResp)
	if err != nil {
		return errResponse(500, "failed to translate Gemini response: "+err.Error())
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

	var geminiErr geminiErrorBody
	if err := json.Unmarshal(respBody, &geminiErr); err == nil && geminiErr.Error.Message != "" {
		errMap := openaiErr["error"].(map[string]interface{})
		if geminiErr.Error.Status != "" {
			errMap["type"] = geminiErr.Error.Status
		}
		errMap["message"] = geminiErr.Error.Message
	}

	newBody, err := json.Marshal(openaiErr)
	if err != nil {
		return errResponse(500, "failed to translate Gemini error: "+err.Error())
	}

	return policy.DownstreamResponseModifications{
		Body: newBody,
		HeadersToSet: map[string]string{
			"content-type":   "application/json",
			"content-length": fmt.Sprintf("%d", len(newBody)),
		},
	}
}

func buildOpenAIChatCompletion(resp *geminiResponse, responseModel string) map[string]interface{} {
	choices := make([]map[string]interface{}, 0, len(resp.Candidates))
	for i, cand := range resp.Candidates {
		choices = append(choices, buildOpenAIChoice(&cand, i))
	}

	usage := buildOpenAIUsage(resp.UsageMetadata)

	id := resp.ResponseID
	if id == "" {
		id = newChatCompletionID()
	} else {
		id = "chatcmpl-" + id
	}

	return map[string]interface{}{
		"id":      id,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   responseModel,
		"choices": choices,
		"usage":   usage,
	}
}

func buildOpenAIChoice(cand *geminiCandidate, defaultIndex int) map[string]interface{} {
	var textParts []string
	var toolCalls []map[string]interface{}

	if cand.Content != nil {
		for i := range cand.Content.Parts {
			part := &cand.Content.Parts[i]
			switch {
			case part.FunctionCall != nil:
				toolCalls = append(toolCalls, buildOpenAIToolCall(part.FunctionCall))
			case part.Text != "" && !part.Thought:
				textParts = append(textParts, part.Text)
			}
		}
	}

	message := map[string]interface{}{
		"role":    "assistant",
		"content": strings.Join(textParts, ""),
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}

	idx := cand.Index
	if idx == 0 && defaultIndex > 0 {
		idx = defaultIndex
	}
	return map[string]interface{}{
		"index":         idx,
		"message":       message,
		"finish_reason": finishReasonToOpenAI(cand.FinishReason, len(toolCalls) > 0),
	}
}

func buildOpenAIToolCall(fc *geminiFunctionCall) map[string]interface{} {
	argsBytes, err := json.Marshal(fc.Args)
	argsStr := string(argsBytes)
	if err != nil || fc.Args == nil {
		argsStr = "{}"
	}
	return map[string]interface{}{
		"id":   "call_" + shortRandomID(),
		"type": "function",
		"function": map[string]interface{}{
			"name":      fc.Name,
			"arguments": argsStr,
		},
	}
}

func buildOpenAIUsage(meta *geminiUsageMetadata) map[string]interface{} {
	if meta == nil {
		return map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		}
	}

	completion := meta.CandidatesTokenCount + meta.ThoughtsTokenCount
	usage := map[string]interface{}{
		"prompt_tokens":     meta.PromptTokenCount,
		"completion_tokens": completion,
		"total_tokens":      meta.TotalTokenCount,
	}
	if meta.CachedContentTokenCount > 0 {
		usage["prompt_tokens_details"] = map[string]interface{}{
			"cached_tokens": meta.CachedContentTokenCount,
		}
	}
	if meta.ThoughtsTokenCount > 0 {
		usage["completion_tokens_details"] = map[string]interface{}{
			"reasoning_tokens": meta.ThoughtsTokenCount,
		}
	}
	return usage
}

func finishReasonToOpenAI(reason string, hasToolCalls bool) string {
	if hasToolCalls {
		return openaiFinishToolCalls
	}
	switch reason {
	case geminiFinishStop, "":
		return openaiFinishStop
	case geminiFinishMaxTokens:
		return openaiFinishLength
	case geminiFinishSafety, geminiFinishRecitation, geminiFinishBlocklist,
		geminiFinishProhibited, geminiFinishSPII, geminiFinishImageRecit, geminiFinishLanguage:
		return openaiFinishContentFilter
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

func shortRandomID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func looksLikeSSE(body []byte) bool {
	trimmed := strings.TrimLeft(string(body), " \r\n\t")
	return strings.HasPrefix(trimmed, "event:") || strings.HasPrefix(trimmed, "data:")
}

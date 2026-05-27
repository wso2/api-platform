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

// Package openaitogemini translates OpenAI Chat Completions requests/responses
// to and from the Google Gemini generateContent API.
package openaitogemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

const (
	PolicyName                  = "openai-to-gemini"
	DefaultAPIVersion           = "v1beta"
	MethodGenerateContent       = "generateContent"
	MethodStreamGenerateContent = "streamGenerateContent"
	MetadataKeySelectedProvider = "selected_provider"
)

type PolicyParams struct {
	Model      string
	APIVersion string
	// Id is the upstream provider this translator targets. It is both the
	// upstream cluster the request is routed to and the key matched
	// (case-insensitive) against SharedContext.Metadata["selected_provider"]
	// in multi-provider mode.
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
		return errResponse(400, "'model' policy parameter is required for Gemini translation.")
	}

	stream := false
	if v, ok := payload["stream"].(bool); ok {
		stream = v
	}

	mods := translateBody(payload, model, p.params, stream)
	if p.params.Id != "" && mods.UpstreamName == nil {
		upstream := p.params.Id
		mods.UpstreamName = &upstream
	}

	slog.Debug(PolicyName+": translating request",
		"id", p.params.Id, "model", model, "stream", stream)
	return mods
}

// OnResponseBody translates the Gemini response into an OpenAI ChatCompletion
// body. Streaming SSE responses are passed through untouched.
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

// shouldRun reports whether the request should be translated. When no upstream
// router (e.g. openai-header-router) has published a selected provider into the
// metadata, the proxy is in single-provider mode and the translator always
// runs. When a provider has been selected, the translator runs only if that
// selection matches its own "id".
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
	raw, ok := metadata[MetadataKeySelectedProvider]
	if !ok {
		return ""
	}
	v, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

func parseParams(params map[string]interface{}) (PolicyParams, error) {
	result := PolicyParams{APIVersion: DefaultAPIVersion}

	model, err := optionalString(params, "model")
	if err != nil {
		return result, err
	}
	if model == "" {
		return result, fmt.Errorf("'model' is required")
	}
	result.Model = model

	if v, err := optionalString(params, "id"); err != nil {
		return result, err
	} else {
		result.Id = v
	}

	if v, err := optionalString(params, "apiVersion"); err != nil {
		return result, err
	} else if v != "" {
		result.APIVersion = v
	}

	return result, nil
}

func optionalString(params map[string]interface{}, key string) (string, error) {
	raw, ok := params[key]
	if !ok || raw == nil {
		return "", nil
	}
	v, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("'%s' must be a string", key)
	}
	return strings.TrimSpace(v), nil
}

func errResponse(statusCode int, message string) policy.ImmediateResponse {
	body, _ := json.Marshal(map[string]string{"error": message})
	return policy.ImmediateResponse{
		StatusCode: statusCode,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}
}

// translateBody converts an OpenAI Chat Completions payload into a Gemini
// generateContent request and returns the upstream modifications.
func translateBody(
	payload map[string]interface{}, model string, params PolicyParams, stream bool,
) policy.UpstreamRequestModifications {
	geminiBody := map[string]interface{}{}

	if msgs, ok := payload["messages"].([]interface{}); ok {
		systemParts, contents := convertMessages(msgs)
		if len(systemParts) > 0 {
			geminiBody["systemInstruction"] = map[string]interface{}{"parts": systemParts}
		}
		if len(contents) > 0 {
			geminiBody["contents"] = contents
		}
	}

	if cfg := buildGenerationConfig(payload); len(cfg) > 0 {
		geminiBody["generationConfig"] = cfg
	}

	if tools, ok := payload["tools"].([]interface{}); ok && len(tools) > 0 {
		if geminiTools := convertTools(tools); len(geminiTools) > 0 {
			geminiBody["tools"] = geminiTools
		}
	}

	// "none" has no Gemini equivalent in the tool list; drop tools entirely
	// and set functionCallingConfig.mode=NONE.
	if tc, ok := payload["tool_choice"]; ok && tc != nil {
		toolConfig, drop := convertToolChoice(tc)
		if drop {
			delete(geminiBody, "tools")
		}
		if toolConfig != nil {
			geminiBody["toolConfig"] = toolConfig
		}
	}

	newBody, err := json.Marshal(geminiBody)
	if err != nil {
		return policy.UpstreamRequestModifications{
			Body: []byte(fmt.Sprintf(`{"error":"failed to marshal Gemini body: %s"}`, err.Error())),
		}
	}

	newPath := buildGeminiPath(params.APIVersion, model, stream)
	return policy.UpstreamRequestModifications{
		Body: newBody,
		Path: &newPath,
		HeadersToSet: map[string]string{
			"content-type": "application/json",
		},
	}
}

func buildGeminiPath(apiVersion, model string, stream bool) string {
	method := MethodGenerateContent
	suffix := ""
	if stream {
		method = MethodStreamGenerateContent
		suffix = "?alt=sse"
	}
	return fmt.Sprintf("/%s/models/%s:%s%s", apiVersion, model, method, suffix)
}

// convertMessages walks OpenAI messages[] and produces Gemini systemInstruction
// parts and contents[]. Consecutive user/tool parts flush into one Gemini
// "user" content; assistant messages flush as a "model" content.
func convertMessages(msgs []interface{}) ([]interface{}, []interface{}) {
	var systemParts []interface{}
	var contents []interface{}
	var pendingUserParts []interface{}

	flushUser := func() {
		if len(pendingUserParts) == 0 {
			return
		}
		contents = append(contents, map[string]interface{}{
			"role":  "user",
			"parts": pendingUserParts,
		})
		pendingUserParts = nil
	}

	for _, raw := range msgs {
		msg, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)

		switch role {
		case "system", "developer":
			if parts := convertTextOrPartsToGemini(msg["content"]); len(parts) > 0 {
				systemParts = append(systemParts, parts...)
			}
		case "user":
			if parts := convertUserContent(msg["content"]); len(parts) > 0 {
				pendingUserParts = append(pendingUserParts, parts...)
			}
		case "tool":
			toolCallID, _ := msg["tool_call_id"].(string)
			text := extractTextContent(msg["content"])
			pendingUserParts = append(pendingUserParts, map[string]interface{}{
				"functionResponse": map[string]interface{}{
					"name":     toolCallID,
					"response": map[string]interface{}{"output": text},
				},
			})
		case "assistant":
			flushUser()
			contents = append(contents, map[string]interface{}{
				"role":  "model",
				"parts": convertAssistantParts(msg),
			})
		}
	}
	flushUser()

	return systemParts, contents
}

func convertTextOrPartsToGemini(content interface{}) []interface{} {
	switch c := content.(type) {
	case string:
		if c == "" {
			return nil
		}
		return []interface{}{map[string]interface{}{"text": c}}
	case []interface{}:
		var parts []interface{}
		for _, p := range c {
			m, ok := p.(map[string]interface{})
			if !ok || m["type"] != "text" {
				continue
			}
			if text, ok := m["text"].(string); ok && text != "" {
				parts = append(parts, map[string]interface{}{"text": text})
			}
		}
		return parts
	}
	return nil
}

func convertUserContent(content interface{}) []interface{} {
	switch c := content.(type) {
	case string:
		if c == "" {
			return nil
		}
		return []interface{}{map[string]interface{}{"text": c}}
	case []interface{}:
		var parts []interface{}
		for _, p := range c {
			m, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			switch m["type"].(string) {
			case "text":
				if text, ok := m["text"].(string); ok && text != "" {
					parts = append(parts, map[string]interface{}{"text": text})
				}
			case "image_url":
				if part := convertImage(m); part != nil {
					parts = append(parts, part)
				}
			}
		}
		return parts
	}
	return nil
}

func extractTextContent(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var parts []string
		for _, p := range c {
			m, ok := p.(map[string]interface{})
			if !ok || m["type"] != "text" {
				continue
			}
			if text, ok := m["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	}
	return ""
}

// convertAssistantParts builds the Gemini "model" parts list from an OpenAI
// assistant message: text becomes a text part; each tool_call becomes a
// functionCall part with arguments parsed back into an object.
func convertAssistantParts(msg map[string]interface{}) []interface{} {
	var parts []interface{}

	if text := extractTextContent(msg["content"]); text != "" {
		parts = append(parts, map[string]interface{}{"text": text})
	}

	toolCalls, _ := msg["tool_calls"].([]interface{})
	for _, tc := range toolCalls {
		toolCall, ok := tc.(map[string]interface{})
		if !ok {
			continue
		}
		fn, _ := toolCall["function"].(map[string]interface{})
		if fn == nil {
			continue
		}
		name, _ := fn["name"].(string)
		argsStr, _ := fn["arguments"].(string)

		// OpenAI wire format is a JSON string; Gemini wants an object. Fall
		// back to empty object on parse failure so the request still validates.
		var args map[string]interface{}
		if argsStr != "" {
			if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
				args = map[string]interface{}{}
			}
		} else {
			args = map[string]interface{}{}
		}
		parts = append(parts, map[string]interface{}{
			"functionCall": map[string]interface{}{
				"name": name,
				"args": args,
			},
		})
	}

	return parts
}

func convertImage(m map[string]interface{}) map[string]interface{} {
	imgObj, ok := m["image_url"].(map[string]interface{})
	if !ok {
		return nil
	}
	imageURL, ok := imgObj["url"].(string)
	if !ok || imageURL == "" {
		return nil
	}

	if strings.HasPrefix(imageURL, "data:") {
		comma := strings.SplitN(imageURL, ",", 2)
		if len(comma) != 2 {
			return nil
		}
		meta := strings.TrimPrefix(comma[0], "data:")
		metaParts := strings.SplitN(meta, ";", 2)
		return map[string]interface{}{
			"inlineData": map[string]interface{}{
				"mimeType": metaParts[0],
				"data":     comma[1],
			},
		}
	}

	// Remote URL — Gemini accepts fileData with file_uri. mimeType defaults
	// to image/jpeg when the URL has no recognizable extension.
	mimeType := "image/jpeg"
	lower := strings.ToLower(imageURL)
	switch {
	case strings.HasSuffix(lower, ".png"):
		mimeType = "image/png"
	case strings.HasSuffix(lower, ".gif"):
		mimeType = "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		mimeType = "image/webp"
	}
	return map[string]interface{}{
		"fileData": map[string]interface{}{
			"mimeType": mimeType,
			"fileUri":  imageURL,
		},
	}
}

func buildGenerationConfig(payload map[string]interface{}) map[string]interface{} {
	cfg := map[string]interface{}{}

	if v, ok := payload["temperature"]; ok {
		cfg["temperature"] = v
	}
	if v, ok := payload["top_p"]; ok {
		cfg["topP"] = v
	}
	if v, ok := payload["max_completion_tokens"]; ok {
		cfg["maxOutputTokens"] = v
	} else if v, ok := payload["max_tokens"]; ok {
		cfg["maxOutputTokens"] = v
	}
	if v, ok := payload["stop"]; ok && v != nil {
		switch s := v.(type) {
		case string:
			cfg["stopSequences"] = []string{s}
		case []interface{}:
			cfg["stopSequences"] = s
		}
	}
	if v, ok := payload["n"]; ok {
		cfg["candidateCount"] = v
	}
	if v, ok := payload["seed"]; ok {
		cfg["seed"] = v
	}
	if v, ok := payload["frequency_penalty"]; ok {
		cfg["frequencyPenalty"] = v
	}
	if v, ok := payload["presence_penalty"]; ok {
		cfg["presencePenalty"] = v
	}

	return cfg
}

func convertTools(tools []interface{}) []map[string]interface{} {
	var decls []map[string]interface{}
	for _, t := range tools {
		tool, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		fn, ok := tool["function"].(map[string]interface{})
		if !ok {
			continue
		}
		decl := map[string]interface{}{"name": fn["name"]}
		if desc, ok := fn["description"]; ok {
			decl["description"] = desc
		}
		if params, ok := fn["parameters"]; ok {
			decl["parameters"] = params
		}
		decls = append(decls, decl)
	}
	if len(decls) == 0 {
		return nil
	}
	return []map[string]interface{}{
		{"functionDeclarations": decls},
	}
}

// convertToolChoice returns (toolConfig, dropTools). dropTools=true tells the
// caller to remove the tools[] field — used for the "none" choice where
// Gemini also expects mode=NONE.
func convertToolChoice(tc interface{}) (map[string]interface{}, bool) {
	switch v := tc.(type) {
	case string:
		switch v {
		case "auto":
			return map[string]interface{}{
				"functionCallingConfig": map[string]interface{}{"mode": "AUTO"},
			}, false
		case "required":
			return map[string]interface{}{
				"functionCallingConfig": map[string]interface{}{"mode": "ANY"},
			}, false
		case "none":
			return map[string]interface{}{
				"functionCallingConfig": map[string]interface{}{"mode": "NONE"},
			}, true
		}
	case map[string]interface{}:
		if fn, ok := v["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				return map[string]interface{}{
					"functionCallingConfig": map[string]interface{}{
						"mode":                 "ANY",
						"allowedFunctionNames": []string{name},
					},
				}, false
			}
		}
	}
	return map[string]interface{}{
		"functionCallingConfig": map[string]interface{}{"mode": "AUTO"},
	}, false
}

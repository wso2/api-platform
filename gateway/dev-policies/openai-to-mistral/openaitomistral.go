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

// Package openaitomistral translates OpenAI Chat Completions requests for
// Mistral's OpenAI-compatible chat completions endpoint.
package openaitomistral

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

const (
	PolicyName                  = "openai-to-mistral"
	MistralChatCompletionsPath  = "/v1/chat/completions"
	MetadataKeySelectedProvider = "selected_provider"
)

// unsupportedRequestFields lists OpenAI fields Mistral's API rejects. Keep in
// sync with https://docs.mistral.ai/api/.
var unsupportedRequestFields = []string{
	"logprobs",
	"top_logprobs",
	"logit_bias",
	"n",
	"service_tier",
	"store",
	"metadata",
	"user",
}

type PolicyParams struct {
	// Model overrides the OpenAI "model" field in the translated request.
	Model string
	// Id is the upstream provider this translator targets. It serves two
	// purposes: it is the upstream cluster the request is routed to, and it
	// is the key matched (case-insensitive) against
	// SharedContext.Metadata["selected_provider"] in multi-provider mode.
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
		return errResponse(400, "'model' policy parameter is required for Mistral translation.")
	}

	payload["model"] = model
	for _, key := range unsupportedRequestFields {
		delete(payload, key)
	}

	newBody, err := json.Marshal(payload)
	if err != nil {
		return errResponse(500, "failed to marshal Mistral body: "+err.Error())
	}

	newPath := MistralChatCompletionsPath
	slog.Debug(PolicyName+": translating request",
		"id", p.params.Id, "model", model, "path", newPath)

	mods := policy.UpstreamRequestModifications{
		Body:         newBody,
		Path:         &newPath,
		HeadersToSet: map[string]string{"content-type": "application/json"},
	}
	if p.params.Id != "" {
		upstream := p.params.Id
		mods.UpstreamName = &upstream
	}
	return mods
}

// OnResponseBody normalises the Mistral response into OpenAI shape. Mistral
// already emits OpenAI-shaped success bodies, so the work here is mostly
// error-envelope translation and ensuring the response model is non-empty.
// SSE streaming bodies pass through untouched.
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
	result := PolicyParams{}

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

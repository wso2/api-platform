/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package publishers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
)

func TestMoesifAIProvider(t *testing.T) {
	tests := map[string]string{
		"openai":          "openai",
		"anthropic":       "anthropic",
		"gemini":          "gemini",
		"awsbedrock":      "bedrock_converse",
		"azure-openai":    "azure",
		"azureai-foundry": "azure_ai",
		"mistralai":       "mistral",
		"MistralAI":       "mistral",
		"my-vllm":         "my-vllm",
		"":                "",
	}
	for template, want := range tests {
		assert.Equal(t, want, moesifAIProvider(template), "template %q", template)
	}
}

func TestAIContextBlock_ModelType(t *testing.T) {
	tests := map[string]string{
		"/openai/v1/chat/completions":                                                 "chat",
		"/anthropic/v1/messages":                                                      "chat",
		"/v1beta/models/gemini-2.5-pro:generateContent":                               "chat",
		"/v1beta/models/gemini-2.5-pro:streamGenerateContent":                         "chat",
		"/v1beta/models/text-embedding-004:embedContent":                              "embeddings",
		"/openai/v1/completions":                                                      "completion",
		"/openai/v1/embeddings":                                                       "embeddings",
		"/bedrock/model/us.anthropic.claude-sonnet-4-5-20250929-v1:0/converse":        "chat",
		"/bedrock/model/us.anthropic.claude-sonnet-4-5-20250929-v1:0/converse-stream": "chat",
		"/bedrock/model/anthropic.claude-v2/invoke":                                   "",
		"": "",
	}
	for route, want := range tests {
		event := createBaseEvent()
		event.Properties["aiMetadata"] = dto.AIMetadata{VendorName: "openai", Model: "gpt-4o"}

		block := aiContextBlock(event, route)

		require.NotNil(t, block, "route %q", route)
		if want == "" {
			assert.Nil(t, block.ModelType, "route %q", route)
		} else {
			require.NotNil(t, block.ModelType, "route %q", route)
			assert.Equal(t, want, *block.ModelType, "route %q", route)
		}
	}
}

func TestAIContextBlock_Full(t *testing.T) {
	event := createBaseEvent()
	event.Properties["aiMetadata"] = dto.AIMetadata{VendorName: "azure-openai", Model: "gpt-4o-2024-08-06", LLMCost: 0.0123}
	event.Properties[dto.PropKeyAIUsage] = dto.AIUsage{PromptTokens: ptr(100), CompletionTokens: ptr(50), TotalTokens: ptr(150)}

	block := aiContextBlock(event, "/azure/openai/deployments/d1/chat/completions")

	require.NotNil(t, block)
	assert.Equal(t, "azure", *block.Provider)
	assert.Equal(t, "gpt-4o-2024-08-06", *block.Model)
	assert.Equal(t, "chat", *block.ModelType)
	require.NotNil(t, block.Usage)
	assert.Equal(t, 100, *block.Usage.PromptTokens)
	assert.Equal(t, 50, *block.Usage.CompletionTokens)
	assert.Equal(t, 150, *block.Usage.TotalTokens)
	// The gateway's cost would turn off Moesif's own calculation, so it is never sent.
	assert.Nil(t, block.CostMetrics)
	assert.Nil(t, block.Attribution)
	assert.Nil(t, block.FinishReason)
}

func TestAIContextBlock_NoAIData(t *testing.T) {
	event := createBaseEvent()
	assert.Nil(t, aiContextBlock(event, "/chat/completions"))

	event.Properties["aiMetadata"] = dto.AIMetadata{}
	event.Properties[dto.PropKeyAIUsage] = dto.AIUsage{}
	assert.Nil(t, aiContextBlock(event, "/chat/completions"))
}

func TestAIContextBlock_UnreportedUsage(t *testing.T) {
	event := createBaseEvent()
	event.Properties["aiMetadata"] = dto.AIMetadata{VendorName: "openai", Model: "gpt-4o"}
	event.Properties[dto.PropKeyAIUsage] = dto.AIUsage{}

	block := aiContextBlock(event, "/chat/completions")

	require.NotNil(t, block)
	assert.Nil(t, block.Usage, "no reported count must not be sent as a zero-token call")
}

func TestAIContextBlock_PartialUsage(t *testing.T) {
	event := createBaseEvent()
	event.Properties["aiMetadata"] = dto.AIMetadata{VendorName: "openai", Model: "text-embedding-3-small"}
	event.Properties[dto.PropKeyAIUsage] = dto.AIUsage{PromptTokens: ptr(8), TotalTokens: ptr(8)}

	block := aiContextBlock(event, "/embeddings")

	require.NotNil(t, block)
	require.NotNil(t, block.Usage)
	assert.Equal(t, 8, *block.Usage.PromptTokens)
	assert.Nil(t, block.Usage.CompletionTokens)
	assert.Equal(t, 8, *block.Usage.TotalTokens)
}

// A negative count is not a usage Moesif can price: it is dropped like an unreported
// one, and a usage left with no count is omitted.
func TestAIContextBlock_NegativeUsageDropped(t *testing.T) {
	event := createBaseEvent()
	event.Properties["aiMetadata"] = dto.AIMetadata{VendorName: "openai", Model: "gpt-4o"}
	event.Properties[dto.PropKeyAIUsage] = dto.AIUsage{PromptTokens: ptr(-5), CompletionTokens: ptr(50), TotalTokens: ptr(0)}

	block := aiContextBlock(event, "/chat/completions")

	require.NotNil(t, block)
	require.NotNil(t, block.Usage)
	assert.Nil(t, block.Usage.PromptTokens)
	assert.Equal(t, 50, *block.Usage.CompletionTokens)
	assert.Equal(t, 0, *block.Usage.TotalTokens)

	event.Properties[dto.PropKeyAIUsage] = dto.AIUsage{PromptTokens: ptr(-1), CompletionTokens: ptr(-1), TotalTokens: ptr(-2)}
	block = aiContextBlock(event, "/chat/completions")

	require.NotNil(t, block)
	assert.Nil(t, block.Usage)
}

// The published document is the contract: ai_context sits beside metadata at the top
// level, and the aiMetadata and aiTokenUsage metadata keys keep their existing shape.
func TestPublish_LlmEventsCarryAIContextAlongsideMetadata(t *testing.T) {
	for _, apiType := range []string{"LlmProvider", "LlmProxy"} {
		t.Run(apiType, func(t *testing.T) {
			moesif := createTestMoesifWithoutAPI()
			event := createBaseEvent()
			event.API.APIType = apiType
			event.Operation.APIResourceTemplate = "/openai/v1/chat/completions"
			event.Properties["aiMetadata"] = dto.AIMetadata{Model: "gpt-4o", VendorName: "openai", VendorVersion: "v1.0", LLMCost: 0.5}
			hour := 9
			event.Properties["aiTokenUsage"] = dto.AITokenUsage{PromptToken: 100, CompletionToken: 50, TotalToken: 150, Hour: &hour}
			event.Properties[dto.PropKeyAIUsage] = dto.AIUsage{PromptTokens: ptr(100), CompletionTokens: ptr(50), TotalTokens: ptr(150)}

			moesif.Publish(event)

			require.Len(t, moesif.events, 1)
			raw, err := json.Marshal(moesif.events[0])
			require.NoError(t, err)
			var published map[string]any
			require.NoError(t, json.Unmarshal(raw, &published))

			assert.Equal(t, map[string]any{
				"provider":   "openai",
				"model":      "gpt-4o",
				"model_type": "chat",
				"usage": map[string]any{
					"prompt_tokens":     float64(100),
					"completion_tokens": float64(50),
					"total_tokens":      float64(150),
				},
			}, published["ai_context"])

			metadata, ok := published["metadata"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, map[string]any{
				"model": "gpt-4o", "vendorName": "openai", "vendorVersion": "v1.0", "llmCost": 0.5,
			}, metadata["aiMetadata"])
			assert.Equal(t, map[string]any{
				"promptTokens": float64(100), "completionTokens": float64(50), "totalTokens": float64(150), "hour": float64(9),
			}, metadata["aiTokenUsage"])
		})
	}
}

func TestPublish_NonLlmEventsCarryNoAIContext(t *testing.T) {
	for _, apiType := range []string{"Rest", "Mcp", "Agent", "GraphQLApi"} {
		t.Run(apiType, func(t *testing.T) {
			moesif := createTestMoesifWithoutAPI()
			event := createBaseEvent()
			event.API.APIType = apiType
			event.Properties["aiMetadata"] = dto.AIMetadata{Model: "gpt-4o", VendorName: "openai"}
			event.Properties[dto.PropKeyAIUsage] = dto.AIUsage{PromptTokens: ptr(1)}

			moesif.Publish(event)

			require.Len(t, moesif.events, 1)
			assert.Nil(t, moesif.events[0].AiContext)
		})
	}
}

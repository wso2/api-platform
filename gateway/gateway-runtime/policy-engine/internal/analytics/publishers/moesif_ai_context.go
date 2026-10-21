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

// Translation of the gateway's LLM analytics into Moesif's first-class `ai_context`
// event block (moesifapi-go v1.1.6+).
//
// Moesif computes an LLM call's cost from this block's provider, model and token
// usage. So the provider is sent under the name Moesif prices it by rather than the
// gateway's template name. The model is sent exactly as the provider returned it,
// because Moesif resolves it together with the provider.
//
// Cost is deliberately not sent. A `cost_metrics` object on the event turns off
// Moesif's own calculation. The llm-cost policy reports 0 for a model it has no price
// for, which would then be recorded as a free call. The gateway's own cost still
// reaches Moesif as metadata.aiMetadata.llmCost.
//
// Nothing is derived in this file: provider, model and usage are decided in the
// analytics package, and the model type comes from the same route classification the
// OTel publisher uses, so both sinks describe a call the same way.

import (
	"strings"

	"github.com/moesif/moesifapi-go/models"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
)

// moesifAIProviders maps an LLM provider template name onto the provider name Moesif
// prices by. A name not listed here is sent unchanged.
//
// Moesif files Bedrock models under two names and prices a model only when the
// provider matches the name it is filed under. bedrock_converse holds the current
// models (Claude 3.7 and later, Nova, ...); older ones (Claude 3.5, Llama 3.1, ...)
// are under bedrock and go unpriced with this name. bedrock instead leaves current
// Claude models unpriced or prices them at the AWS GovCloud rate, so an older model
// without a cost is the safer gap.
var moesifAIProviders = map[string]string{
	"openai":          "openai",
	"anthropic":       "anthropic",
	"gemini":          "gemini",
	"awsbedrock":      "bedrock_converse",
	"azure-openai":    "azure",
	"azureai-foundry": "azure_ai",
	"mistralai":       "mistral",
}

// moesifAIModelTypes maps the route's GenAI operation onto Moesif's model types.
var moesifAIModelTypes = map[string]string{
	"chat":             "chat",
	"generate_content": "chat",
	"text_completion":  "completion",
	"embeddings":       "embeddings",
}

// aiContextBlock builds the event's `ai_context` block from the AI properties the
// collector assembled, or returns nil when the event carried none.
//
// Returning nil keeps the block off the event entirely, so an LLM call the collector
// could not read anything from ingests exactly as it did before this field existed.
func aiContextBlock(event *dto.Event, route string) *models.AiContextModel {
	aiMetadata, _ := event.Properties["aiMetadata"].(dto.AIMetadata)
	usage := moesifAIUsage(event.Properties[dto.PropKeyAIUsage])
	if aiMetadata.VendorName == "" && aiMetadata.Model == "" && usage == nil {
		return nil
	}
	return &models.AiContextModel{
		Provider:  nonEmpty(moesifAIProvider(aiMetadata.VendorName)),
		Model:     nonEmpty(aiMetadata.Model),
		ModelType: nonEmpty(moesifAIModelTypes[otelGenAIOperationName(route)]),
		Usage:     usage,
	}
}

// moesifAIProvider returns Moesif's provider name for a template name, or the template
// name itself when it is not one Moesif is known to price.
func moesifAIProvider(templateName string) string {
	if provider, ok := moesifAIProviders[strings.ToLower(templateName)]; ok {
		return provider
	}
	return templateName
}

// moesifAIUsage returns the reported token counts, or nil when none was reported.
// An unreported count stays nil: sending it as 0 would price the call at zero tokens.
func moesifAIUsage(property any) *models.AiUsageModel {
	usage, ok := property.(dto.AIUsage)
	if !ok || (usage.PromptTokens == nil && usage.CompletionTokens == nil && usage.TotalTokens == nil) {
		return nil
	}
	return &models.AiUsageModel{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func nonEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

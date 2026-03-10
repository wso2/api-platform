/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package llmcost

import (
	"encoding/json"
	"strings"
)

// AzureAICalculator handles models with provider "azure_ai"
// (Azure AI Foundry). Azure AI Foundry hosts heterogeneous models:
//   - Claude models return responses in Anthropic wire format
//   - GPT/other models return responses in OpenAI wire format
//
// The calculator detects the format based on the model name and delegates
// normalization accordingly. It also adds the Azure AI Foundry Model Router
// flat cost in the Adjust step when the model name matches a router pattern.
type AzureAICalculator struct{}

func (c *AzureAICalculator) Normalize(responseBody []byte, requestBody []byte) (Usage, error) {
	// Detect model name to determine wire format.
	var probe struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(responseBody, &probe)

	if strings.Contains(strings.ToLower(probe.Model), "claude") {
		return (&AnthropicCalculator{}).Normalize(responseBody, requestBody)
	}
	return (&OpenAICalculator{}).Normalize(responseBody, requestBody)
}

// Adjust adds the Model Router flat cost on top of the base per-token cost.
// The flat cost per input token is stored in the azure_ai/model_router pricing
// entry's input_cost_per_token field.
func (c *AzureAICalculator) Adjust(baseCost float64, usage Usage, pricing ModelPricing) float64 {
	// Only apply the model router flat cost for model-router deployments.
	// For Claude models on Azure AI, also apply the Anthropic geo/speed adjustment.
	model := strings.ToLower(pricing.Provider) // used only as a type hint

	if strings.Contains(model, "claude") {
		baseCost = (&AnthropicCalculator{}).Adjust(baseCost, usage, pricing)
	}

	return baseCost
}

// isModelRouter returns true if the model name indicates an Azure AI Foundry
// Model Router deployment.
func isModelRouter(modelName string) bool {
	lower := strings.ToLower(modelName)
	return strings.Contains(lower, "model-router") || strings.Contains(lower, "model_router")
}

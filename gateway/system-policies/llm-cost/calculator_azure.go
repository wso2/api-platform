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

import "encoding/json"

// AzureOpenAICalculator handles models with provider "azure" and
// "azure_text". Azure OpenAI proxies the OpenAI wire format unchanged, so
// field names are identical to OpenAICalculator. This is a separate file so
// Azure-specific pricing quirks (e.g. PTU handling, audio tokens) can be
// added without touching the core OpenAI calculator.
type AzureOpenAICalculator struct{}

func (c *AzureOpenAICalculator) Normalize(responseBody []byte, _ []byte) (Usage, error) {
	var resp struct {
		Usage struct {
			PromptTokens        int64 `json:"prompt_tokens"`
			CompletionTokens    int64 `json:"completion_tokens"`
			TotalTokens         int64 `json:"total_tokens"`
			PromptTokensDetails struct {
				CachedTokens int64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			CompletionTokensDetails struct {
				ReasoningTokens int64 `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return Usage{}, err
	}
	u := resp.Usage
	return Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
		CachedReadTokens: u.PromptTokensDetails.CachedTokens,
		ReasoningTokens:  u.CompletionTokensDetails.ReasoningTokens,
	}, nil
}

func (c *AzureOpenAICalculator) Adjust(baseCost float64, _ Usage, _ ModelPricing) float64 {
	return baseCost
}

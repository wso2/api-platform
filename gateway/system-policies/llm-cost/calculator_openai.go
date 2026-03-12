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

// OpenAICalculator handles models with provider "openai" and
// "text-completion-openai". All fields follow the OpenAI chat completion
// response format.
type OpenAICalculator struct{}

func (c *OpenAICalculator) Normalize(responseBody []byte, requestBody []byte) (Usage, error) {
	var resp struct {
		// service_tier is returned at the top level of the response object.
		// Values: "default" | "flex" | "priority" | "scale" (deprecated).
		ServiceTier string `json:"service_tier"`
		Usage       struct {
			PromptTokens        int64 `json:"prompt_tokens"`
			CompletionTokens    int64 `json:"completion_tokens"`
			TotalTokens         int64 `json:"total_tokens"`
			PromptTokensDetails struct {
				CachedTokens int64 `json:"cached_tokens"`
				AudioTokens  int64 `json:"audio_tokens"`
			} `json:"prompt_tokens_details"`
			CompletionTokensDetails struct {
				ReasoningTokens int64 `json:"reasoning_tokens"`
				AudioTokens     int64 `json:"audio_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
		// Choices carries per-message annotations added by built-in tools.
		// url_citation annotations indicate that the web search tool was invoked.
		Choices []struct {
			Message struct {
				Annotations []struct {
					Type string `json:"type"`
				} `json:"annotations"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return Usage{}, err
	}

	// Map OpenAI's service_tier values to our internal tier strings.
	// "default" and "" both mean standard pricing (no override).
	var serviceTier string
	switch resp.ServiceTier {
	case "flex":
		serviceTier = "flex"
	case "priority":
		serviceTier = "priority"
	case "batch":
		serviceTier = "batch"
	}

	// Detect web search tool invocation: the API adds a url_citation annotation
	// to each choice whose message cites a web search result. One tool call is
	// billed per completion regardless of how many citations appear.
	var webSearchRequests int64
	for _, choice := range resp.Choices {
		for _, ann := range choice.Message.Annotations {
			if ann.Type == "url_citation" {
				webSearchRequests = 1
				break
			}
		}
		if webSearchRequests > 0 {
			break
		}
	}

	// Extract the search context size from the request when web search was used.
	// Possible values: "low" | "medium" | "high". Defaults to "medium" in billing.
	var searchContextSize string
	if webSearchRequests > 0 && len(requestBody) > 0 {
		var req struct {
			WebSearchOptions *struct {
				SearchContextSize string `json:"search_context_size"`
			} `json:"web_search_options"`
		}
		if err := json.Unmarshal(requestBody, &req); err == nil && req.WebSearchOptions != nil {
			searchContextSize = req.WebSearchOptions.SearchContextSize
		}
	}

	u := resp.Usage
	return Usage{
		PromptTokens:      u.PromptTokens,
		CompletionTokens:  u.CompletionTokens,
		TotalTokens:       u.TotalTokens,
		CachedReadTokens:  u.PromptTokensDetails.CachedTokens,
		AudioInputTokens:  u.PromptTokensDetails.AudioTokens,
		AudioOutputTokens: u.CompletionTokensDetails.AudioTokens,
		ReasoningTokens:   u.CompletionTokensDetails.ReasoningTokens,
		ServiceTier:       serviceTier,
		WebSearchRequests: webSearchRequests,
		SearchContextSize: searchContextSize,
	}, nil
}

func (c *OpenAICalculator) Adjust(baseCost float64, _ Usage, _ ModelPricing) float64 {
	return baseCost
}

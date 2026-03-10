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

// GeminiCalculator handles models with provider "gemini", "vertex_ai",
// and the various "vertex_ai-*" subfamilies. Gemini uses a different response
// field namespace ("usageMetadata") from OpenAI.
//
// Context-window tiering (>128k, >200k) is handled generically by
// genericCalculateCost — no per-provider logic is needed here.
type GeminiCalculator struct{}

func (c *GeminiCalculator) Normalize(responseBody []byte, _ []byte) (Usage, error) {
	var resp struct {
		UsageMetadata struct {
			PromptTokenCount        int64 `json:"promptTokenCount"`
			CandidatesTokenCount    int64 `json:"candidatesTokenCount"`
			TotalTokenCount         int64 `json:"totalTokenCount"`
			ThoughtsTokenCount      int64 `json:"thoughtsTokenCount"`      // Gemini thinking models
			CachedContentTokenCount int64 `json:"cachedContentTokenCount"` // Gemini context caching
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return Usage{}, err
	}
	m := resp.UsageMetadata
	return Usage{
		PromptTokens:     m.PromptTokenCount,
		CompletionTokens: m.CandidatesTokenCount,
		TotalTokens:      m.TotalTokenCount,
		ReasoningTokens:  m.ThoughtsTokenCount,
		CachedReadTokens: m.CachedContentTokenCount,
	}, nil
}

func (c *GeminiCalculator) Adjust(baseCost float64, _ Usage, _ ModelPricing) float64 {
	return baseCost
}

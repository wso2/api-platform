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

// MistralCalculator handles models with provider "mistral".
// Mistral's chat completion API is OpenAI-compatible, so field names are
// identical. This is a separate file so Mistral-specific features (e.g.
// citation tokens) can be added without touching the OpenAI calculator.
type MistralCalculator struct{}

func (c *MistralCalculator) Normalize(responseBody []byte, _ []byte) (Usage, error) {
	var resp struct {
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
			// PromptAudioSeconds is the duration of audio input for Voxtral chat models.
			// Mistral bills audio per-minute separately from text tokens; prompt_tokens
			// represents only the text portion. We convert to seconds for genericCalculateCost.
			PromptAudioSeconds *int64 `json:"prompt_audio_seconds"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return Usage{}, err
	}
	u := resp.Usage
	usage := Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
	}
	if u.PromptAudioSeconds != nil {
		usage.AudioInputSeconds = float64(*u.PromptAudioSeconds)
	}
	return usage, nil
}

func (c *MistralCalculator) Adjust(baseCost float64, _ Usage, _ ModelPricing) float64 {
	return baseCost
}

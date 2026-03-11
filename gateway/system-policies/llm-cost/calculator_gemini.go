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

// modalityTokenDetail is a single entry in Gemini's per-modality token arrays.
type modalityTokenDetail struct {
	Modality   string `json:"modality"`
	TokenCount int64  `json:"tokenCount"`
}

func (c *GeminiCalculator) Normalize(responseBody []byte, _ []byte) (Usage, error) {
	var resp struct {
		UsageMetadata struct {
			PromptTokenCount        int64 `json:"promptTokenCount"`
			CandidatesTokenCount    int64 `json:"candidatesTokenCount"`
			TotalTokenCount         int64 `json:"totalTokenCount"`
			ThoughtsTokenCount      int64 `json:"thoughtsTokenCount"`      // Gemini thinking models
			CachedContentTokenCount int64 `json:"cachedContentTokenCount"` // Gemini context caching

			// Prompt token breakdown by modality (TEXT / AUDIO / IMAGE / VIDEO).
			// Total counts (cached + non-cached); we subtract cacheTokensDetails below.
			PromptTokensDetails []modalityTokenDetail `json:"promptTokensDetails"`

			// Cached token breakdown by modality (explicit context caching only).
			CacheTokensDetails []modalityTokenDetail `json:"cacheTokensDetails"`

			// Candidate (output) token breakdown by modality.
			// Used by image-generation and native-audio output models.
			CandidatesTokensDetails []modalityTokenDetail `json:"candidatesTokensDetails"`

			// Gemini Live API only — output tokens with per-modality breakdown.
			ResponseTokenCount    int64                 `json:"responseTokenCount"`
			ResponseTokensDetails []modalityTokenDetail `json:"responseTokensDetails"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return Usage{}, err
	}
	m := resp.UsageMetadata

	// --- Prompt modality tokens ---
	// promptTokensDetails contains TOTAL prompt tokens per modality (including cached).
	// We then subtract cached audio/image/video from cacheTokensDetails to get
	// the non-cached portion billed at the higher input rate.
	var rawAudioIn, rawImageIn int64
	for _, d := range m.PromptTokensDetails {
		switch d.Modality {
		case "AUDIO":
			rawAudioIn = d.TokenCount
		case "IMAGE":
			rawImageIn = d.TokenCount
		}
	}
	// Subtract cached tokens per modality (explicit context caching).
	var cachedAudioIn, cachedImageIn int64
	for _, d := range m.CacheTokensDetails {
		switch d.Modality {
		case "AUDIO":
			cachedAudioIn = d.TokenCount
		case "IMAGE":
			cachedImageIn = d.TokenCount
		}
	}
	audioIn := rawAudioIn - cachedAudioIn
	if audioIn < 0 {
		audioIn = 0
	}
	imageIn := rawImageIn - cachedImageIn
	if imageIn < 0 {
		imageIn = 0
	}

	// --- Output modality tokens ---
	// candidatesTokensDetails is used for image-generation and non-streaming audio models.
	// responseTokensDetails is used for Gemini Live (streaming audio).
	// We read both and take whichever is non-zero.
	var audioOut, imageOut int64
	for _, d := range m.CandidatesTokensDetails {
		switch d.Modality {
		case "AUDIO":
			audioOut = d.TokenCount
		case "IMAGE":
			imageOut = d.TokenCount
		}
	}
	// Gemini Live: responseTokensDetails overrides candidatesTokensDetails for audio
	for _, d := range m.ResponseTokensDetails {
		if d.Modality == "AUDIO" {
			audioOut = d.TokenCount
		}
	}

	// Use responseTokenCount (Gemini Live) when available.
	completionTokens := m.CandidatesTokenCount
	if m.ResponseTokenCount > 0 {
		completionTokens = m.ResponseTokenCount
	}

	return Usage{
		PromptTokens:      m.PromptTokenCount,
		CompletionTokens:  completionTokens,
		TotalTokens:       m.TotalTokenCount,
		ReasoningTokens:   m.ThoughtsTokenCount,
		CachedReadTokens:  m.CachedContentTokenCount,
		AudioInputTokens:  audioIn,
		AudioOutputTokens: audioOut,
		ImageOutputTokens: imageOut,
		// Note: imageIn tokens are already counted in PromptTokens and priced at
		// the standard input rate for most Gemini models. A separate
		// input_cost_per_image_token would require an ImageInputTokens field;
		// currently no Gemini model in our pricing JSON has a distinct image
		// input rate so we leave imageIn unused.
	}, nil
}

func (c *GeminiCalculator) Adjust(baseCost float64, _ Usage, _ ModelPricing) float64 {
	return baseCost
}

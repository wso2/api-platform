/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package llmusage

// Usage holds token counts normalized from a provider response, with the
// provider's cache accounting already resolved.
type Usage struct {
	// TotalInputTokens is every input token the request consumed, including
	// cached and cache-write tokens.
	TotalInputTokens int64

	// UncachedInputTokens is the subset of TotalInputTokens billed at the
	// standard input rate.
	UncachedInputTokens int64

	// CachedReadTokens is the subset billed at the cache-read rate.
	CachedReadTokens int64

	// CacheWriteTokens and CacheWrite1hTokens are the subsets billed at the
	// cache-creation rates. CacheWrite1hTokens covers the extended TTL.
	CacheWriteTokens   int64
	CacheWrite1hTokens int64

	OutputTokens    int64
	ReasoningTokens int64

	AudioInputTokens  int64
	AudioOutputTokens int64

	// TotalTokens is the provider's reported total when it gives one, and
	// input + output otherwise. Providers do not always agree with the sum.
	TotalTokens int64

	// ServiceTier is the billing tier the provider served the request on, when
	// it is one that carries its own rates: "priority", "flex" or "batch". The
	// standard tier is reported as an empty string, whichever word the provider
	// happens to use for it.
	ServiceTier string

	// IsPriority reports the priority tier specifically, for consumers that only
	// need that distinction.
	IsPriority bool

	// Model is the model name resolved from the response or request.
	Model string

	// ModelCandidates lists the names considered, in the order they were
	// tried, so a consumer needing the fallback chain can see it.
	ModelCandidates []string
}

// rawCounts holds the values read straight out of a response, before cache
// accounting is applied.
type rawCounts struct {
	InputTokens        int64
	OutputTokens       int64
	TotalTokens        int64
	CachedTokens       int64
	CacheWriteTokens   int64
	CacheWrite1hTokens int64
	ReasoningTokens    int64
	AudioInputTokens   int64
	AudioOutputTokens  int64
	ServiceTier        string
	Model              string
	ModelCandidates    []string
}

// accountingAdditive is the cacheAccounting value meaning the cached and
// cache-write counts sit outside the reported input total.
const accountingAdditive = "additive"

// Service tiers that carry their own rates. Providers spell the standard tier
// variously ("default", "standard", absent), and all of those normalize to an
// empty string.
const (
	tierPriority = "priority"
	tierFlex     = "flex"
	tierBatch    = "batch"
)

// normalizeServiceTier keeps only the tiers that select different rates. Any
// other value, including the several spellings of the standard tier, becomes
// empty so a consumer applies standard pricing.
func normalizeServiceTier(reported string) string {
	switch reported {
	case tierPriority, tierFlex, tierBatch:
		return reported
	default:
		return ""
	}
}

// normalize resolves cache accounting into a Usage. With "additive" the cache
// counts are added to the reported input total; otherwise they are already
// inside it and only the uncached remainder is derived.
func normalize(raw rawCounts, accounting string) Usage {
	u := Usage{
		OutputTokens:      raw.OutputTokens,
		CachedReadTokens:  raw.CachedTokens,
		ReasoningTokens:   raw.ReasoningTokens,
		AudioInputTokens:  raw.AudioInputTokens,
		AudioOutputTokens: raw.AudioOutputTokens,
		ServiceTier:       normalizeServiceTier(raw.ServiceTier),
		IsPriority:        raw.ServiceTier == tierPriority,
		Model:             raw.Model,
		ModelCandidates:   raw.ModelCandidates,
	}

	u.CacheWriteTokens = raw.CacheWriteTokens
	u.CacheWrite1hTokens = raw.CacheWrite1hTokens

	if accounting == accountingAdditive {
		u.TotalInputTokens = raw.InputTokens + raw.CachedTokens +
			raw.CacheWriteTokens + raw.CacheWrite1hTokens
		u.UncachedInputTokens = raw.InputTokens
	} else {
		u.TotalInputTokens = raw.InputTokens
		u.UncachedInputTokens = raw.InputTokens - raw.CachedTokens -
			raw.CacheWriteTokens - raw.CacheWrite1hTokens
		if u.UncachedInputTokens < 0 {
			u.UncachedInputTokens = 0
		}
	}

	u.TotalTokens = raw.TotalTokens
	if u.TotalTokens == 0 {
		u.TotalTokens = u.TotalInputTokens + u.OutputTokens
	}

	return u
}

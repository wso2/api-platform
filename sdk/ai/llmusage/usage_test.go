package llmusage

import "testing"

func TestNormalize_InclusiveKeepsInputTotal(t *testing.T) {
	// OpenAI style: cached_tokens is already inside prompt_tokens.
	raw := rawCounts{InputTokens: 1000, OutputTokens: 200, CachedTokens: 800}

	got := normalize(raw, "inclusive")

	if got.TotalInputTokens != 1000 {
		t.Errorf("TotalInputTokens = %d, want 1000", got.TotalInputTokens)
	}
	if got.CachedReadTokens != 800 {
		t.Errorf("CachedReadTokens = %d, want 800", got.CachedReadTokens)
	}
	if got.UncachedInputTokens != 200 {
		t.Errorf("UncachedInputTokens = %d, want 200", got.UncachedInputTokens)
	}
}

func TestNormalize_AdditiveSumsIntoInputTotal(t *testing.T) {
	// Anthropic style: total input is input + cache_creation + cache_read.
	raw := rawCounts{
		InputTokens:        1000,
		OutputTokens:       200,
		CachedTokens:       500,
		CacheWriteTokens:   300,
		CacheWrite1hTokens: 100,
	}

	got := normalize(raw, "additive")

	if got.TotalInputTokens != 1900 {
		t.Errorf("TotalInputTokens = %d, want 1900", got.TotalInputTokens)
	}
	if got.UncachedInputTokens != 1000 {
		t.Errorf("UncachedInputTokens = %d, want 1000", got.UncachedInputTokens)
	}
}

func TestNormalize_AbsentAccountingIsInclusive(t *testing.T) {
	raw := rawCounts{InputTokens: 500, CachedTokens: 200}

	got := normalize(raw, "")

	if got.TotalInputTokens != 500 {
		t.Errorf("TotalInputTokens = %d, want 500", got.TotalInputTokens)
	}
}

func TestNormalize_UncachedNeverNegative(t *testing.T) {
	// A provider reporting more cached tokens than input must not produce a
	// negative billable count.
	raw := rawCounts{InputTokens: 100, CachedTokens: 500}

	got := normalize(raw, "inclusive")

	if got.UncachedInputTokens != 0 {
		t.Errorf("UncachedInputTokens = %d, want 0", got.UncachedInputTokens)
	}
}

func TestNormalize_TotalTokensPreferredWhenReported(t *testing.T) {
	raw := rawCounts{InputTokens: 100, OutputTokens: 50, TotalTokens: 175}

	got := normalize(raw, "inclusive")

	if got.TotalTokens != 175 {
		t.Errorf("TotalTokens = %d, want 175 (reported value, not the sum)", got.TotalTokens)
	}
}

func TestNormalize_TotalTokensDerivedWhenAbsent(t *testing.T) {
	raw := rawCounts{InputTokens: 100, OutputTokens: 50}

	got := normalize(raw, "inclusive")

	if got.TotalTokens != 150 {
		t.Errorf("TotalTokens = %d, want 150", got.TotalTokens)
	}
}

func TestNormalize_PriorityTierDetected(t *testing.T) {
	raw := rawCounts{InputTokens: 10, ServiceTier: "priority"}

	if got := normalize(raw, ""); !got.IsPriority {
		t.Error("IsPriority = false, want true for service_tier=priority")
	}
	if got := normalize(rawCounts{ServiceTier: "standard"}, ""); got.IsPriority {
		t.Error("IsPriority = true, want false for service_tier=standard")
	}
}

func TestNormalize_ServiceTierKeepsRateBearingTiers(t *testing.T) {
	// Only tiers with their own rates survive; every spelling of standard
	// becomes empty so the consumer applies standard pricing.
	tests := []struct {
		reported string
		want     string
	}{
		{"priority", "priority"},
		{"flex", "flex"},
		{"batch", "batch"},
		{"default", ""},
		{"standard", ""},
		{"", ""},
		{"scale", ""},
	}

	for _, tt := range tests {
		got := normalize(rawCounts{InputTokens: 10, ServiceTier: tt.reported}, "")
		if got.ServiceTier != tt.want {
			t.Errorf("ServiceTier for %q = %q, want %q", tt.reported, got.ServiceTier, tt.want)
		}
	}
}

func TestNormalize_IsPriorityOnlyForPriority(t *testing.T) {
	for _, tier := range []string{"flex", "batch", "default", ""} {
		if normalize(rawCounts{ServiceTier: tier}, "").IsPriority {
			t.Errorf("IsPriority = true for tier %q, want false", tier)
		}
	}
	if !normalize(rawCounts{ServiceTier: "priority"}, "").IsPriority {
		t.Error("IsPriority = false for priority, want true")
	}
}

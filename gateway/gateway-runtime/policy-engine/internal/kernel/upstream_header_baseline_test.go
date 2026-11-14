/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 */

package kernel

import (
	"testing"
	"time"
)

// The defect this whole mechanism exists for: a credential injected for the FIRST target must
// not be on the request when a later attempt reaches a DIFFERENT provider (FR-042).
func TestHeaderRestores_FailedAttemptsCredentialIsRemoved(t *testing.T) {
	// Attempt 1's chain added x-dead-key; it was absent in the post-downstream request.
	baselines := map[string]headerBaseline{"x-dead-key": {}}
	// What Envoy hands attempt 2: the request as attempt 1 left it.
	current := map[string][]string{
		"content-type": {"application/json"},
		"x-dead-key":   {"dead-vendor-credential"},
	}

	restores := headerRestoresFor(baselines, current)
	applyHeaderRestores(current, restores)

	if _, present := current["x-dead-key"]; present {
		t.Fatal("the failed attempt's credential survived onto the next attempt — this is the leak")
	}
	if got := current["content-type"]; len(got) != 1 || got[0] != "application/json" {
		t.Fatalf("content-type = %v, want the downstream value untouched", got)
	}
}

// Remove and restore are different operations. A header the chain OVERWROTE must get its prior
// value back; deleting it would discard a downstream decision while undoing an upstream one.
func TestHeaderRestores_OverwrittenHeaderIsRestoredNotDeleted(t *testing.T) {
	baselines := map[string]headerBaseline{
		"content-type": {Values: []string{"application/json"}, Present: true},
	}
	current := map[string][]string{"content-type": {"application/vnd.anthropic+json"}}

	applyHeaderRestores(current, headerRestoresFor(baselines, current))

	got := current["content-type"]
	if len(got) != 1 || got[0] != "application/json" {
		t.Fatalf("content-type = %v, want the pre-chain value restored", got)
	}
}

// Headers nobody's chain touched are outside this mechanism entirely — which is what makes an
// exemption list for x-request-id/x-envoy-*/tracing unnecessary rather than forgotten.
func TestHeaderRestores_UntouchedHeadersAreNeverConsidered(t *testing.T) {
	baselines := map[string]headerBaseline{"x-dead-key": {}}
	current := map[string][]string{
		"x-request-id":          {"abc-123"},
		"x-envoy-attempt-count": {"2"},
		"traceparent":           {"00-trace-span-01"},
		"authorization":         {"Bearer client-token"},
		"x-dead-key":            {"leftover"},
	}

	applyHeaderRestores(current, headerRestoresFor(baselines, current))

	for _, name := range []string{"x-request-id", "x-envoy-attempt-count", "traceparent", "authorization"} {
		if _, present := current[name]; !present {
			t.Errorf("%s was removed; only headers a per-attempt chain touched may be", name)
		}
	}
}

// A retry onto a provider whose chain sets the same header is the common case. Emitting a
// restore there would be undone microseconds later, so it must produce no operation at all.
func TestHeaderRestores_MatchingValueProducesNoOperation(t *testing.T) {
	baselines := map[string]headerBaseline{
		"content-type": {Values: []string{"application/json"}, Present: true},
	}
	current := map[string][]string{"content-type": {"application/json"}}

	if restores := headerRestoresFor(baselines, current); len(restores) != 0 {
		t.Fatalf("restores = %v, want none when the value already matches the baseline", restores)
	}
}

// captureHeaderBaselines must record ABSENCE distinctly from an empty value, or the restore
// cannot tell "delete this" from "set this to empty".
func TestCaptureHeaderBaselines_AbsenceIsDistinctFromEmpty(t *testing.T) {
	pre := map[string][]string{"x-present": {""}}

	captured := captureHeaderBaselines(pre, []string{"x-present", "x-absent"})

	if !captured["x-present"].Present {
		t.Error("x-present recorded as absent, but it was there with an empty value")
	}
	if captured["x-absent"].Present {
		t.Error("x-absent recorded as present, but it was not in the request")
	}
}

// First-touch wins across attempts: attempt 2 re-recording a header attempt 1 already touched
// would store attempt 1's LEFTOVER as if it were the post-downstream value, and the leak would
// survive the fix.
func TestRememberTouchedHeaders_FirstTouchWins(t *testing.T) {
	store := newAttemptBaselineStore()
	store.Remember("req-1", requestBaseline{Path: "/v1/chat"})

	store.RememberTouchedHeaders("req-1", map[string]headerBaseline{"x-key": {}})
	store.RememberTouchedHeaders("req-1", map[string]headerBaseline{
		"x-key": {Values: []string{"attempt-1-leftover"}, Present: true},
	})

	baseline, ok := store.Lookup("req-1")
	if !ok {
		t.Fatal("baseline missing")
	}
	if baseline.Headers["x-key"].Present {
		t.Fatal("the second attempt overwrote the first's record — a leftover is now the baseline")
	}
}

// T024k4: the gap the pure-function test leaves. TestRepointToProvider_BaselinePreventsCompounding
// proves the arithmetic with a baseline handed to it; this proves the entry is still THERE when
// a third attempt needs it. An early eviction would fail here and nowhere else — and it fails
// silently in production, because Lookup treats a miss as "this attempt is its own baseline".
func TestAttemptBaselineStore_SurvivesToTheThirdAttempt(t *testing.T) {
	now := time.Now()
	store := newAttemptBaselineStore()
	store.now = func() time.Time { return now }

	store.Remember("req-1", requestBaseline{Path: "/openai-ctx/v1/chat"})
	store.RememberTouchedHeaders("req-1", map[string]headerBaseline{"x-key": {}})

	// Attempts 2 and 3 arrive later in the same request.
	for _, elapsed := range []time.Duration{2 * time.Second, 30 * time.Second} {
		now = now.Add(elapsed)
		baseline, ok := store.Lookup("req-1")
		if !ok {
			t.Fatalf("baseline gone %s in; attempts would compound and headers would never be undone", elapsed)
		}
		if baseline.Path != "/openai-ctx/v1/chat" {
			t.Fatalf("path = %q, want the first attempt's", baseline.Path)
		}
		if _, recorded := baseline.Headers["x-key"]; !recorded {
			t.Fatal("touched-header record gone; the credential undo would not happen")
		}
	}
}

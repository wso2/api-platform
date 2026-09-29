/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package kernel

import (
	"strconv"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// The guardrail block must not collide with any code the ENGINE emits, or with any reserved
// classification range.
//
// The engine's own codes are the near miss: they sit at 905xxx, one thousand below the block,
// and both were allocated as "somewhere above the reserved ceiling". A future engine code walking
// up from 905007 has room, but not unlimited room.
func TestGuardrailBlockCollidesWithNothingTheEngineEmits(t *testing.T) {
	engineCodes := map[string]string{
		"engine internal":      codeEngineInternal,
		"payload too large":    codePayloadTooLarge,
		"no policy chain":      codeNoPolicyChain,
		"resolution bad req":   codeResolutionBadRequest,
		"unsupported coding":   codeResolutionUnsupportedEncoding,
		"upstream unreachable": codeUpstreamUnreachable,
		"upstream timeout":     codeUpstreamTimeout,
		"upstream unavailable": codeUpstreamUnavailable,
		"no route":             codeNoRoute,
		"upstream generic":     codeUpstreamGeneric,
	}
	for name, code := range engineCodes {
		n, err := strconv.Atoi(code)
		if err != nil {
			t.Errorf("%s: code %q is not numeric", name, code)
			continue
		}
		if n >= policy.GuardrailCodeRangeStart && n < policy.GuardrailCodeRangeEnd {
			t.Errorf("%s (%s) falls inside the guardrail block [%d, %d) — two unrelated "+
				"failures would report the same code", name, code,
				policy.GuardrailCodeRangeStart, policy.GuardrailCodeRangeEnd)
		}
	}

	classificationRanges := []struct {
		name       string
		start, end int
	}{
		{"auth", policy.AuthFailureRangeStart, policy.AuthFailureRangeEnd},
		{"throttled", policy.ThrottledFailureRangeStart, policy.ThrottledFailureRangeEnd},
		{"target", policy.TargetFailureRangeStart, policy.TargetFailureRangeEnd},
	}
	for _, r := range classificationRanges {
		if policy.GuardrailCodeRangeStart < r.end && r.start < policy.GuardrailCodeRangeEnd {
			t.Errorf("the guardrail block [%d, %d) overlaps the %s range [%d, %d) — a "+
				"content rejection would be reported as a %s failure",
				policy.GuardrailCodeRangeStart, policy.GuardrailCodeRangeEnd,
				r.name, r.start, r.end, r.name)
		}
	}
}

// IsGuardrailCode is the single answer to "is this a guardrail rejection?", and the engine
// leans on it: every guardrail code, the generic included, sits in one block, so the test is a
// range test that must not reach its neighbours.
func TestIsGuardrailCodeMatchesTheBlockAndNothingElse(t *testing.T) {
	for _, code := range []string{
		policy.GuardrailCodeIntervened, // the generic, at the head of the block
		policy.GuardrailCodeHate,       // content safety
		policy.GuardrailCodePII,        // sensitive data
		policy.GuardrailCodeWordCount,  // shape
		policy.GuardrailCodeSemanticMatch,
		"906000", "906399", // the block's own bounds
	} {
		if !policy.IsGuardrailCode(code) {
			t.Errorf("IsGuardrailCode(%q) = false, want true", code)
		}
	}
	for _, code := range []string{
		"905999", "906400", // either side of the block
		policy.FaultCodeAuthMissingCredentials,
		policy.FaultCodeThrottledAPI,
		"not-a-number", "",
	} {
		if policy.IsGuardrailCode(code) {
			t.Errorf("IsGuardrailCode(%q) = true, want false", code)
		}
	}
}

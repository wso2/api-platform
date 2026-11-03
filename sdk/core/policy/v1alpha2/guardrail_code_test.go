/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 *
 */

package policyv1alpha2

import (
	"strconv"
	"testing"
)

// The grouping is the contract, so it needs a test rather than a comment.
//
// A caller that wants "any content-safety rejection" tests the group bounds instead of
// enumerating categories — that is the whole reason the specific codes are worth having over
// the policy name. It only holds if every code lands inside the group it claims, which is a
// property a reviewer cannot check by eye once there are twenty of them.
func TestGuardrailCodesLandInTheirOwnGroup(t *testing.T) {
	groups := []struct {
		name       string
		start, end int
		codes      []string
	}{
		{
			"content safety", GuardrailContentSafetyRangeStart, GuardrailContentSafetyRangeEnd,
			[]string{
				GuardrailCodeHate, GuardrailCodeSexual, GuardrailCodeSelfHarm,
				GuardrailCodeViolence, GuardrailCodeHarassment, GuardrailCodeDangerousActivity,
				GuardrailCodeProfanity, GuardrailCodePromptInjection, GuardrailCodeUnsafeContent,
			},
		},
		{
			"sensitive data", GuardrailSensitiveDataRangeStart, GuardrailSensitiveDataRangeEnd,
			[]string{GuardrailCodePII, GuardrailCodeCredential},
		},
		{
			"shape", GuardrailShapeRangeStart, GuardrailShapeRangeEnd,
			[]string{
				GuardrailCodeWordCount, GuardrailCodeSentenceCount, GuardrailCodeContentLength,
				GuardrailCodeSchema, GuardrailCodePattern, GuardrailCodeURL,
			},
		},
		{
			"intent", GuardrailIntentRangeStart, GuardrailIntentRangeEnd,
			[]string{GuardrailCodeSemanticMatch},
		},
	}

	seen := map[string]string{}
	for _, g := range groups {
		for _, code := range g.codes {
			n, err := strconv.Atoi(code)
			if err != nil {
				t.Errorf("%s: code %q is not numeric", g.name, code)
				continue
			}
			if n < g.start || n >= g.end {
				t.Errorf("%s: code %s is outside its group [%d, %d) — a caller testing group "+
					"membership would not see it", g.name, code, g.start, g.end)
			}
			if n < GuardrailCodeRangeStart || n >= GuardrailCodeRangeEnd {
				t.Errorf("%s: code %s is outside the guardrail block [%d, %d)",
					g.name, code, GuardrailCodeRangeStart, GuardrailCodeRangeEnd)
			}
			if where, dup := seen[code]; dup {
				t.Errorf("code %s is used by both %s and %s — two rejections a caller cannot "+
					"tell apart is the problem these codes exist to solve", code, where, g.name)
			}
			seen[code] = g.name
			if len(code) != 6 {
				t.Errorf("%s: code %q must be six digits, like every gateway error code", g.name, code)
			}
		}
	}
}

// The generic code lives INSIDE the block, so "is this a guardrail rejection?" is one range
// test — the same rule every other category follows.
//
// It also has to sit outside all four groups, or a caller asking "was this any content-safety
// rejection?" would match the generic too.
func TestGuardrailGenericCodeIsInsideTheBlockButInNoGroup(t *testing.T) {
	n, err := strconv.Atoi(GuardrailCodeIntervened)
	if err != nil {
		t.Fatalf("GuardrailCodeIntervened is not numeric: %v", err)
	}
	if n < GuardrailCodeRangeStart || n >= GuardrailCodeRangeEnd {
		t.Fatalf("GuardrailCodeIntervened (%s) must live inside the block [%d, %d)",
			GuardrailCodeIntervened, GuardrailCodeRangeStart, GuardrailCodeRangeEnd)
	}
	if !IsGuardrailCode(GuardrailCodeIntervened) {
		t.Fatalf("IsGuardrailCode(%q) = false", GuardrailCodeIntervened)
	}

	for _, g := range []struct {
		name       string
		start, end int
	}{
		{"content safety", GuardrailContentSafetyRangeStart, GuardrailContentSafetyRangeEnd},
		{"sensitive data", GuardrailSensitiveDataRangeStart, GuardrailSensitiveDataRangeEnd},
		{"shape", GuardrailShapeRangeStart, GuardrailShapeRangeEnd},
		{"intent", GuardrailIntentRangeStart, GuardrailIntentRangeEnd},
	} {
		if n >= g.start && n < g.end {
			t.Errorf("the generic code %s falls in the %s group [%d, %d); it belongs to the "+
				"block as a whole, so a group test must not match it",
				GuardrailCodeIntervened, g.name, g.start, g.end)
		}
	}
}

// The block must not overlap the customer reservation, and its groups must not overlap each
// other. Both are the kind of arithmetic that looks right and is not.
func TestGuardrailBlockBoundariesDoNotOverlap(t *testing.T) {
	// Mirrors the 96xxxx policy space documented in docs/gateway/error-codes.md and
	// UserDefinedRangeStart/End in the engine's analytics constants. Restated because the
	// SDK cannot import an internal package — if that block ever moves, this is the test
	// that should fail.
	const policySpaceStart, policySpaceEnd = 960000, 970000

	if GuardrailCodeRangeStart < policySpaceEnd && policySpaceStart < GuardrailCodeRangeEnd {
		t.Errorf("guardrail block [%d, %d) overlaps the 96xxxx policy space [%d, %d)",
			GuardrailCodeRangeStart, GuardrailCodeRangeEnd, policySpaceStart, policySpaceEnd)
	}

	groups := [][2]int{
		{GuardrailContentSafetyRangeStart, GuardrailContentSafetyRangeEnd},
		{GuardrailSensitiveDataRangeStart, GuardrailSensitiveDataRangeEnd},
		{GuardrailShapeRangeStart, GuardrailShapeRangeEnd},
		{GuardrailIntentRangeStart, GuardrailIntentRangeEnd},
	}
	for i, a := range groups {
		if a[0] >= a[1] {
			t.Errorf("group %d is empty or inverted: [%d, %d)", i, a[0], a[1])
		}
		if a[0] < GuardrailCodeRangeStart || a[1] > GuardrailCodeRangeEnd {
			t.Errorf("group %d [%d, %d) escapes the block [%d, %d)",
				i, a[0], a[1], GuardrailCodeRangeStart, GuardrailCodeRangeEnd)
		}
		for j, b := range groups[i+1:] {
			if a[0] < b[1] && b[0] < a[1] {
				t.Errorf("groups %d [%d, %d) and %d [%d, %d) overlap",
					i, a[0], a[1], i+1+j, b[0], b[1])
			}
		}
	}
}

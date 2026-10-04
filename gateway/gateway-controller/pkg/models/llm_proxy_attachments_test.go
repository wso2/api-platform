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

package models

import (
	"strings"
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func strPtr(s string) *string { return &s }

// TestNormaliseLLMProxyAttachments_ShapesAgree is the property the whole
// two-shape contract rests on: the same proxy described either way normalises
// to the identical attachment list, so nothing downstream — policy attachment,
// validation, template resolution — can behave differently depending on which
// shape the author used.
func TestNormaliseLLMProxyAttachments_ShapesAgree(t *testing.T) {
	auth := &api.LLMUpstreamAuth{Type: "api-key", Header: strPtr("Authorization"), Value: strPtr("k")}
	transformer := &api.LLMProxyTransformer{Type: "openai-to-anthropic", Version: "v0"}

	legacy := api.LLMProxyConfigData{
		Provider: &api.LLMProxyProvider{Id: "openai-provider", Auth: auth},
		AdditionalProviders: &[]api.LLMProxyAdditionalProvider{
			{Id: "anthropic-provider", As: strPtr("claude"), Transformer: transformer},
			{Id: "gemini-provider"},
		},
	}
	canonical := api.LLMProxyConfigData{
		Providers: &[]api.LLMProxyProviderEntry{
			{Id: "openai-provider", IsPrimary: true, Auth: auth},
			{Id: "anthropic-provider", Alias: strPtr("claude"), Transformer: transformer},
			{Id: "gemini-provider"},
		},
	}

	fromLegacy, err := NormaliseLLMProxyAttachments(legacy)
	if err != nil {
		t.Fatalf("legacy shape failed to normalise: %v", err)
	}
	fromCanonical, err := NormaliseLLMProxyAttachments(canonical)
	if err != nil {
		t.Fatalf("canonical shape failed to normalise: %v", err)
	}
	if len(fromLegacy) != len(fromCanonical) {
		t.Fatalf("lengths differ: %d vs %d", len(fromLegacy), len(fromCanonical))
	}
	for i := range fromLegacy {
		l, c := fromLegacy[i], fromCanonical[i]
		if l.Id != c.Id || l.IsPrimary != c.IsPrimary || l.EffectiveName() != c.EffectiveName() {
			t.Fatalf("entry %d differs: %+v vs %+v", i, l, c)
		}
		if (l.Auth == nil) != (c.Auth == nil) || (l.Transformer == nil) != (c.Transformer == nil) {
			t.Fatalf("entry %d differs in auth/transformer presence: %+v vs %+v", i, l, c)
		}
		// Both shapes describe the same authored positions here, which is what
		// makes the field paths in validation errors line up.
		if l.SourceIndex != c.SourceIndex {
			t.Fatalf("entry %d source index differs: %d vs %d", i, l.SourceIndex, c.SourceIndex)
		}
	}
}

// TestNormaliseLLMProxyAttachments_PrimaryLeadsButKeepsItsAuthoredIndex covers
// the reordering rule together with the bookkeeping that makes it safe to
// report on: the primary is moved to the front, and each attachment still knows
// where it was written.
func TestNormaliseLLMProxyAttachments_PrimaryLeadsButKeepsItsAuthoredIndex(t *testing.T) {
	spec := api.LLMProxyConfigData{
		Providers: &[]api.LLMProxyProviderEntry{
			{Id: "anthropic-provider"},
			{Id: "gemini-provider"},
			{Id: "openai-provider", IsPrimary: true},
		},
	}

	attachments, err := NormaliseLLMProxyAttachments(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attachments[0].Id != "openai-provider" || !attachments[0].IsPrimary {
		t.Fatalf("expected the primary to lead, got %+v", attachments[0])
	}
	// Authored third, so it must still report index 2 — otherwise a validation
	// error about the primary would name somebody else's entry.
	if attachments[0].SourceIndex != 2 {
		t.Fatalf("expected the primary to keep authored index 2, got %d", attachments[0].SourceIndex)
	}
	if attachments[1].Id != "anthropic-provider" || attachments[1].SourceIndex != 0 {
		t.Fatalf("expected the first-authored entry to follow with index 0, got %+v", attachments[1])
	}
	if attachments[2].Id != "gemini-provider" || attachments[2].SourceIndex != 1 {
		t.Fatalf("expected declaration order preserved after the primary, got %+v", attachments[2])
	}
}

// TestNormaliseLLMProxyAttachments_LegacySourceIndex pins the legacy numbering,
// which is offset by one because the primary occupies its own field rather than
// a list slot.
func TestNormaliseLLMProxyAttachments_LegacySourceIndex(t *testing.T) {
	spec := api.LLMProxyConfigData{
		Provider: &api.LLMProxyProvider{Id: "openai-provider"},
		AdditionalProviders: &[]api.LLMProxyAdditionalProvider{
			{Id: "anthropic-provider"},
			{Id: "gemini-provider"},
		},
	}

	attachments, err := NormaliseLLMProxyAttachments(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int{0, 1, 2}
	for i, attachment := range attachments {
		if attachment.SourceIndex != want[i] {
			t.Fatalf("entry %d: expected source index %d, got %d", i, want[i], attachment.SourceIndex)
		}
	}
}

func TestNormaliseLLMProxyAttachments_Rejections(t *testing.T) {
	cases := []struct {
		name       string
		spec       api.LLMProxyConfigData
		wantErrHas string
	}{
		{
			name: "both shapes at once",
			spec: api.LLMProxyConfigData{
				Provider:  &api.LLMProxyProvider{Id: "openai-provider"},
				Providers: &[]api.LLMProxyProviderEntry{{Id: "openai-provider", IsPrimary: true}},
			},
			wantErrHas: "only one provider shape",
		},
		{
			name:       "empty canonical list",
			spec:       api.LLMProxyConfigData{Providers: &[]api.LLMProxyProviderEntry{}},
			wantErrHas: "must not be empty",
		},
		{
			name: "no primary marked",
			spec: api.LLMProxyConfigData{Providers: &[]api.LLMProxyProviderEntry{
				{Id: "openai-provider"},
				{Id: "anthropic-provider"},
			}},
			wantErrHas: "none is marked",
		},
		{
			name: "several primaries marked",
			spec: api.LLMProxyConfigData{Providers: &[]api.LLMProxyProviderEntry{
				{Id: "openai-provider", IsPrimary: true},
				{Id: "anthropic-provider", IsPrimary: true},
			}},
			wantErrHas: "2 are marked",
		},
		{
			name:       "no provider at all",
			spec:       api.LLMProxyConfigData{},
			wantErrHas: "must declare a provider",
		},
		{
			name: "additional providers with no primary field",
			spec: api.LLMProxyConfigData{
				AdditionalProviders: &[]api.LLMProxyAdditionalProvider{{Id: "anthropic-provider"}},
			},
			wantErrHas: "must declare a provider",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormaliseLLMProxyAttachments(tc.spec)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.wantErrHas) {
				t.Fatalf("expected the error to name the problem (%q), got: %v", tc.wantErrHas, err)
			}
		})
	}
}

func TestPrimaryLLMProxyAttachment(t *testing.T) {
	spec := api.LLMProxyConfigData{
		Providers: &[]api.LLMProxyProviderEntry{
			{Id: "anthropic-provider"},
			{Id: "openai-provider", IsPrimary: true, Alias: strPtr("openai")},
		},
	}

	primary, err := PrimaryLLMProxyAttachment(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if primary.Id != "openai-provider" || primary.EffectiveName() != "openai" {
		t.Fatalf("expected the primary attachment, got %+v", primary)
	}

	if _, err := PrimaryLLMProxyAttachment(api.LLMProxyConfigData{}); err == nil {
		t.Fatal("expected an error when the proxy declares no provider")
	}
}

func TestLLMProxyAttachment_EffectiveName(t *testing.T) {
	cases := []struct {
		name       string
		attachment LLMProxyAttachment
		want       string
	}{
		{"no alias falls back to the id", LLMProxyAttachment{Id: "anthropic-provider"}, "anthropic-provider"},
		{"alias wins", LLMProxyAttachment{Id: "anthropic-provider", Alias: strPtr("claude")}, "claude"},
		{"empty alias falls back to the id", LLMProxyAttachment{Id: "anthropic-provider", Alias: strPtr("")}, "anthropic-provider"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.attachment.EffectiveName(); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package model

import (
	"reflect"
	"strings"
	"testing"
)

// TestNormaliseLLMProxyAttachments_LegacyAndCanonicalAgree is the equivalence
// property at its narrowest: the same proxy described in each
// stored shape must normalise to the identical attachment list, so nothing
// downstream can tell which shape it came from.
func TestNormaliseLLMProxyAttachments_LegacyAndCanonicalAgree(t *testing.T) {
	auth := &UpstreamAuth{Type: "api-key", Header: "Authorization", Value: "{{ secret \"k\" }}"}
	transformer := &LLMProxyTransformer{Type: "openai-to-anthropic", Version: "v0"}

	legacy := LLMProxyConfig{
		Provider:     "openai-provider",
		UpstreamAuth: auth,
		AdditionalProviders: []LLMProxyAdditionalProvider{
			{ID: "anthropic-provider", As: "claude", Transformer: transformer},
			{ID: "gemini-provider"},
		},
	}
	canonical := LLMProxyConfig{
		Providers: []LLMProxyAttachment{
			{ID: "openai-provider", IsPrimary: true, Auth: auth},
			{ID: "anthropic-provider", Alias: "claude", Transformer: transformer},
			{ID: "gemini-provider"},
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
	if !reflect.DeepEqual(fromLegacy, fromCanonical) {
		t.Fatalf("the two shapes disagree:\nlegacy:    %+v\ncanonical: %+v", fromLegacy, fromCanonical)
	}
}

// TestNormaliseLLMProxyAttachments_PrimaryLeadsRegardlessOfPosition covers the
// ordering rule that makes the equivalence above hold: the legacy shape can only
// put its primary first, so the canonical shape must be reordered to match
// however the client listed it.
func TestNormaliseLLMProxyAttachments_PrimaryLeadsRegardlessOfPosition(t *testing.T) {
	config := LLMProxyConfig{
		Providers: []LLMProxyAttachment{
			{ID: "anthropic-provider"},
			{ID: "gemini-provider"},
			{ID: "openai-provider", IsPrimary: true},
		},
	}

	attachments, err := NormaliseLLMProxyAttachments(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := attachments[0].ID; got != "openai-provider" {
		t.Fatalf("expected the primary to lead, got %q", got)
	}
	// The remaining entries keep declaration order.
	if attachments[1].ID != "anthropic-provider" || attachments[2].ID != "gemini-provider" {
		t.Fatalf("expected declaration order after the primary, got %+v", attachments)
	}
}

func TestNormaliseLLMProxyAttachments_Rejections(t *testing.T) {
	cases := []struct {
		name       string
		config     LLMProxyConfig
		wantErrHas string
	}{
		{
			name: "no primary marked",
			config: LLMProxyConfig{Providers: []LLMProxyAttachment{
				{ID: "openai-provider"},
				{ID: "anthropic-provider"},
			}},
			wantErrHas: "none is marked",
		},
		{
			name: "several primaries marked",
			config: LLMProxyConfig{Providers: []LLMProxyAttachment{
				{ID: "openai-provider", IsPrimary: true},
				{ID: "anthropic-provider", IsPrimary: true},
			}},
			wantErrHas: "2 are marked",
		},
		{
			name:       "no provider at all",
			config:     LLMProxyConfig{},
			wantErrHas: "must declare a provider",
		},
		{
			name:       "empty canonical list falls through to the legacy branch and finds nothing",
			config:     LLMProxyConfig{Providers: []LLMProxyAttachment{}},
			wantErrHas: "must declare a provider",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormaliseLLMProxyAttachments(tc.config)
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
	config := LLMProxyConfig{Providers: []LLMProxyAttachment{
		{ID: "anthropic-provider"},
		{ID: "openai-provider", IsPrimary: true, Alias: "openai"},
	}}

	primary, err := PrimaryLLMProxyAttachment(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if primary.ID != "openai-provider" {
		t.Fatalf("expected the primary attachment, got %q", primary.ID)
	}
	if got := PrimaryLLMProxyProviderID(config); got != "openai-provider" {
		t.Fatalf("expected the primary id, got %q", got)
	}
	if got := PrimaryLLMProxyProviderID(LLMProxyConfig{}); got != "" {
		t.Fatalf("expected an empty id for a proxy declaring no provider, got %q", got)
	}
}

func TestLLMProxyAttachment_EffectiveName(t *testing.T) {
	if got := (LLMProxyAttachment{ID: "anthropic-provider"}).EffectiveName(); got != "anthropic-provider" {
		t.Fatalf("expected the id when no alias is set, got %q", got)
	}
	if got := (LLMProxyAttachment{ID: "anthropic-provider", Alias: "claude"}).EffectiveName(); got != "claude" {
		t.Fatalf("expected the alias to win, got %q", got)
	}
}

// TestReferencedLLMProviderIDs_CoversBothRoles is the deletion guard's premise:
// a provider attached in either role, in either stored shape,
// must be visible as a reference.
func TestReferencedLLMProviderIDs_CoversBothRoles(t *testing.T) {
	cases := []struct {
		name   string
		config LLMProxyConfig
		want   []string
	}{
		{
			name: "legacy row",
			config: LLMProxyConfig{
				Provider:            "openai-provider",
				AdditionalProviders: []LLMProxyAdditionalProvider{{ID: "anthropic-provider"}},
			},
			want: []string{"openai-provider", "anthropic-provider"},
		},
		{
			name: "canonical row",
			config: LLMProxyConfig{Providers: []LLMProxyAttachment{
				{ID: "openai-provider", IsPrimary: true},
				{ID: "anthropic-provider"},
			}},
			want: []string{"openai-provider", "anthropic-provider"},
		},
		{
			name: "malformed row still reports what it names",
			config: LLMProxyConfig{Providers: []LLMProxyAttachment{
				{ID: "openai-provider"},
				{ID: "anthropic-provider"},
			}},
			want: []string{"openai-provider", "anthropic-provider"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReferencedLLMProviderIDs(tc.config); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

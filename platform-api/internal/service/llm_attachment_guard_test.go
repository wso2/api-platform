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

package service

import (
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// What a client that predates the per-attachment fields can and cannot be
// allowed to write.
//
// An update is a full replace: the stored configuration is rebuilt from the
// request. A client that cannot express a field therefore erases it, and for a
// transformer that means the provider quietly stops translating — a failure
// that surfaces at invocation, not at save. These tests pin which writes are
// refused and, just as importantly, which are still allowed, so the guard
// cannot grow into a blanket refusal.

func legacyProxyRequest(as, transformerType string) *api.LLMProxy {
	provider := &api.LLMProxyProvider{Id: "openai-provider"}
	if as != "" {
		value := as
		provider.As = &value
	}
	if transformerType != "" {
		provider.Transformer = &api.LLMProxyTransformer{
			Type:    transformerType,
			Version: "v0",
		}
	}
	return &api.LLMProxy{Provider: provider}
}

func storedProxy(attachments ...model.LLMProxyAttachment) *model.LLMProxy {
	return &model.LLMProxy{
		Origin:        constants.OriginCP,
		Configuration: model.LLMProxyConfig{Providers: attachments},
	}
}

func TestEnsureLegacyWriteCanExpressProxy(t *testing.T) {
	primary := func(alias string, transformer *model.LLMProxyTransformer) model.LLMProxyAttachment {
		return model.LLMProxyAttachment{
			ID:          "openai-provider",
			Alias:       alias,
			IsPrimary:   true,
			Transformer: transformer,
		}
	}
	translator := &model.LLMProxyTransformer{Type: "openai-to-anthropic-transformer", Version: "v0"}

	for _, tc := range []struct {
		name           string
		existing       *model.LLMProxy
		request        *api.LLMProxy
		refused        bool
		reasonContains string
	}{
		{
			name:     "a plain single-provider proxy is still writable the old way",
			existing: storedProxy(primary("", nil)),
			request:  legacyProxyRequest("", ""),
		},
		{
			name:           "an upstream name the request cannot carry is refused",
			existing:       storedProxy(primary("openai-eu", nil)),
			request:        legacyProxyRequest("", ""),
			refused:        true,
			reasonContains: "upstream name",
		},
		{
			name:           "a transformer the request cannot carry is refused",
			existing:       storedProxy(primary("", translator)),
			request:        legacyProxyRequest("", ""),
			refused:        true,
			reasonContains: "transformer",
		},
		{
			name:     "a request carrying both back is allowed",
			existing: storedProxy(primary("openai-eu", translator)),
			request:  legacyProxyRequest("openai-eu", "openai-to-anthropic-transformer"),
		},
		{
			name: "a second provider is refused, as before",
			existing: storedProxy(
				primary("", nil),
				model.LLMProxyAttachment{ID: "anthropic-provider"},
			),
			request:        legacyProxyRequest("", ""),
			refused:        true,
			reasonContains: "2 providers",
		},
		{
			name: "a gateway-owned proxy is exempt: its stored configuration is never replaced",
			existing: &model.LLMProxy{
				Origin: constants.OriginDP,
				Configuration: model.LLMProxyConfig{
					Providers: []model.LLMProxyAttachment{primary("openai-eu", translator)},
				},
			},
			request: legacyProxyRequest("", ""),
		},
		{
			name:     "a canonical request is never judged by this guard",
			existing: storedProxy(primary("openai-eu", translator)),
			request: &api.LLMProxy{
				Providers: &[]api.LLMProxyProviderEntry{{Id: "openai-provider", IsPrimary: true}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ensureLegacyWriteCanExpressProxy(tc.request, tc.existing)
			if tc.refused {
				if err == nil {
					t.Fatalf("expected the write to be refused, but it was allowed")
				}
				if !strings.Contains(err.Error(), tc.reasonContains) {
					t.Fatalf("refusal should say why it refused; wanted %q in %q",
						tc.reasonContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("expected the write to be allowed, got: %v", err)
			}
		})
	}
}

// One provider attached twice under two names is a proxy with two accounts at
// the same vendor. Only the name a client routes on has to be unique, so the
// two attachments must be told apart by that and not by the provider id —
// otherwise both are given whichever credential came last and the other is
// lost on a round trip that sends back redacted auth.
func TestPreserveAttachmentCredentialsTellsTwoAccountsApart(t *testing.T) {
	secret := func(value string) *model.UpstreamAuth {
		return &model.UpstreamAuth{Type: "api-key", Header: "X-API-Key", Value: value}
	}
	redacted := func() *model.UpstreamAuth {
		return &model.UpstreamAuth{Type: "api-key", Header: "X-API-Key"}
	}

	existing := []model.LLMProxyAttachment{
		{ID: "openai-provider", Alias: "openai-eu", IsPrimary: true, Auth: secret("{{ secret \"eu\" }}")},
		{ID: "openai-provider", Alias: "openai-us", Auth: secret("{{ secret \"us\" }}")},
	}
	updated := []model.LLMProxyAttachment{
		{ID: "openai-provider", Alias: "openai-eu", IsPrimary: true, Auth: redacted()},
		{ID: "openai-provider", Alias: "openai-us", Auth: redacted()},
	}

	got := preserveAttachmentCredentials(existing, updated)

	for i, want := range []string{"{{ secret \"eu\" }}", "{{ secret \"us\" }}"} {
		if got[i].Auth == nil || got[i].Auth.Value != want {
			t.Fatalf("attachment %q kept %q, want %q",
				got[i].EffectiveName(), upstreamAuthValue(got[i].Auth), want)
		}
	}
}

// An attachment with no alias still resolves by its id, which is what its
// effective name falls back to — the common single-account case must not be
// disturbed by the keying above.
func TestPreserveAttachmentCredentialsWithoutAliases(t *testing.T) {
	existing := []model.LLMProxyAttachment{
		{ID: "openai-provider", IsPrimary: true,
			Auth: &model.UpstreamAuth{Type: "api-key", Value: "{{ secret \"only\" }}"}},
	}
	updated := []model.LLMProxyAttachment{
		{ID: "openai-provider", IsPrimary: true,
			Auth: &model.UpstreamAuth{Type: "api-key"}},
	}

	got := preserveAttachmentCredentials(existing, updated)
	if got[0].Auth == nil || got[0].Auth.Value != "{{ secret \"only\" }}" {
		t.Fatalf("the stored credential was not carried over: %q", upstreamAuthValue(got[0].Auth))
	}
}

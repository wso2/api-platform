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
	"context"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// TestValidateAPIKeyName pins the rule every caller-supplied API key id must satisfy
// before it's persisted — this must stay in lockstep with the gateway's own
// ValidateAPIKeyName (gateway/gateway-controller/pkg/utils/api_key_validation.go), or
// platform-api accepts names the gateway silently drops (issue #3163).
func TestValidateAPIKeyName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty", input: "", wantErr: true},
		{name: "1 char lower boundary valid", input: "a"},
		{name: "2 chars valid", input: "ab"},
		{name: "3 chars valid", input: "abc"},
		{name: "128 chars upper boundary valid", input: repeatChar('a', 128)},
		{name: "129 chars too long", input: repeatChar('a', 129), wantErr: true},
		{name: "uppercase rejected", input: "ABC", wantErr: true},
		{name: "dot rejected", input: "my.key", wantErr: true},
		{name: "space rejected", input: "my key", wantErr: true},
		{name: "at-symbol rejected", input: "my@key", wantErr: true},
		{name: "leading hyphen rejected", input: "-abc", wantErr: true},
		{name: "leading underscore rejected", input: "_abc", wantErr: true},
		{name: "trailing hyphen rejected", input: "abc-", wantErr: true},
		{name: "trailing underscore rejected", input: "abc_", wantErr: true},
		{name: "consecutive hyphens rejected", input: "ab--c", wantErr: true},
		{name: "consecutive underscores rejected", input: "ab__c", wantErr: true},
		{name: "mixed consecutive separators rejected", input: "ab-_c", wantErr: true},
		{name: "internal hyphen and underscore valid", input: "my-key_1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAPIKeyName(tt.input)
			if tt.wantErr != (err != nil) {
				t.Fatalf("validateAPIKeyName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				assertBadRequest(t, err)
			}
		})
	}
}

// nameValidationAPIKeyRepo is a minimal APIKeyRepository double that reports no
// name collision, so resolveUniqueKeyName's retry loop is never exercised here —
// only the validation gate at the top of the function is under test.
type nameValidationAPIKeyRepo struct {
	repository.APIKeyRepository
}

func (nameValidationAPIKeyRepo) GetByArtifactAndName(string, string) (*model.APIKey, error) {
	return nil, nil
}

// TestResolveUniqueKeyName_ValidatesCallerSuppliedId covers the REST API key path
// (issue #3163): a caller-supplied id must be rejected outside [apiKeyNameMinLength,
// apiKeyNameMaxLength] chars, while falling back to displayName-derived generation
// still works unchanged.
func TestResolveUniqueKeyName_ValidatesCallerSuppliedId(t *testing.T) {
	svc := &APIKeyService{apiKeyRepo: nameValidationAPIKeyRepo{}}

	t.Run("too long id rejected", func(t *testing.T) {
		_, err := svc.resolveUniqueKeyName("artifact-1", &api.CreateAPIKeyRequest{Id: strPtr(repeatChar('a', 129))}, "my-api")
		if err == nil {
			t.Fatal("resolveUniqueKeyName() = nil error, want rejection for a 129-char id")
		}
		assertBadRequest(t, err)
	})

	t.Run("short valid id accepted", func(t *testing.T) {
		got, err := svc.resolveUniqueKeyName("artifact-1", &api.CreateAPIKeyRequest{Id: strPtr("ab")}, "my-api")
		if err != nil {
			t.Fatalf("resolveUniqueKeyName() = %v, want success for a 2-char id", err)
		}
		if got != "ab" {
			t.Fatalf("resolveUniqueKeyName() = %q, want %q", got, "ab")
		}
	})

	t.Run("valid id accepted as-is", func(t *testing.T) {
		got, err := svc.resolveUniqueKeyName("artifact-1", &api.CreateAPIKeyRequest{Id: strPtr("production-key-01")}, "my-api")
		if err != nil {
			t.Fatalf("resolveUniqueKeyName() = %v, want success", err)
		}
		if got != "production-key-01" {
			t.Fatalf("resolveUniqueKeyName() = %q, want %q", got, "production-key-01")
		}
	})

	t.Run("no id falls back to displayName generation unaffected", func(t *testing.T) {
		got, err := svc.resolveUniqueKeyName("artifact-1", &api.CreateAPIKeyRequest{DisplayName: "My Key"}, "my-api")
		if err != nil {
			t.Fatalf("resolveUniqueKeyName() = %v, want success", err)
		}
		if got != "my-key" {
			t.Fatalf("resolveUniqueKeyName() = %q, want %q", got, "my-key")
		}
	})
}

// TestCreateLLMProviderAPIKey_RejectsInvalidId is the exact path the issue's repro
// steps hit: creating an LLM provider API key with a caller-supplied id outside
// [apiKeyNameMinLength, apiKeyNameMaxLength] chars, while a short id within the
// (now widened) valid range is accepted.
func TestCreateLLMProviderAPIKey_RejectsInvalidId(t *testing.T) {
	provider := &model.LLMProvider{UUID: "prov-uuid", ID: "prov", OrganizationUUID: "org-1", Name: "Prov", Version: "v1.0"}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(string, string) (*model.LLMProvider, error) { return provider, nil },
	}
	svc := NewLLMProviderAPIKeyService(providerRepo, dpKeyAPIRepo{}, &dpCapturingAPIKeyRepo{},
		newDPKeyEventsService(), newTestIdentityService(), newTestLogger())

	t.Run("too long id rejected", func(t *testing.T) {
		_, err := svc.CreateLLMProviderAPIKey(context.Background(), "prov", "org-1", "",
			&api.CreateLLMProviderAPIKeyRequest{DisplayName: "x", Id: strPtr(repeatChar('a', 129))})
		if err == nil {
			t.Fatal("CreateLLMProviderAPIKey() = nil error, want rejection for a 129-char id")
		}
		assertBadRequest(t, err)
	})

	t.Run("short valid id accepted", func(t *testing.T) {
		resp, err := svc.CreateLLMProviderAPIKey(context.Background(), "prov", "org-1", "",
			&api.CreateLLMProviderAPIKeyRequest{DisplayName: "x", Id: strPtr("a")})
		if err != nil {
			t.Fatalf("CreateLLMProviderAPIKey() = %v, want success for a 1-char id", err)
		}
		if resp == nil || resp.Id != "a" {
			t.Fatalf("CreateLLMProviderAPIKey() = %#v, want Id %q", resp, "a")
		}
	})

	t.Run("valid id accepted", func(t *testing.T) {
		resp, err := svc.CreateLLMProviderAPIKey(context.Background(), "prov", "org-1", "",
			&api.CreateLLMProviderAPIKeyRequest{DisplayName: "x", Id: strPtr("valid-key")})
		if err != nil {
			t.Fatalf("CreateLLMProviderAPIKey() = %v, want success", err)
		}
		if resp == nil || resp.Id != "valid-key" {
			t.Fatalf("CreateLLMProviderAPIKey() = %#v, want Id %q", resp, "valid-key")
		}
	})
}

// TestCreateLLMProxyAPIKey_RejectsInvalidId is the LLM-proxy counterpart.
func TestCreateLLMProxyAPIKey_RejectsInvalidId(t *testing.T) {
	proxy := &model.LLMProxy{UUID: "proxy-uuid", ID: "proxy", OrganizationUUID: "org-1", Name: "Proxy", Version: "v1.0"}
	proxyRepo := &mockLLMProxyRepo{
		getByIDFunc: func(string, string) (*model.LLMProxy, error) { return proxy, nil },
	}
	svc := NewLLMProxyAPIKeyService(proxyRepo, dpKeyAPIRepo{}, &dpCapturingAPIKeyRepo{},
		newDPKeyEventsService(), newTestIdentityService(), newTestLogger())

	t.Run("too long id rejected", func(t *testing.T) {
		_, err := svc.CreateLLMProxyAPIKey(context.Background(), "proxy", "org-1", "",
			&api.CreateLLMProxyAPIKeyRequest{DisplayName: "x", Id: strPtr(repeatChar('a', 129))})
		if err == nil {
			t.Fatal("CreateLLMProxyAPIKey() = nil error, want rejection for a 129-char id")
		}
		assertBadRequest(t, err)
	})

	t.Run("short valid id accepted", func(t *testing.T) {
		resp, err := svc.CreateLLMProxyAPIKey(context.Background(), "proxy", "org-1", "",
			&api.CreateLLMProxyAPIKeyRequest{DisplayName: "x", Id: strPtr("a")})
		if err != nil {
			t.Fatalf("CreateLLMProxyAPIKey() = %v, want success for a 1-char id", err)
		}
		if resp == nil || resp.Id != "a" {
			t.Fatalf("CreateLLMProxyAPIKey() = %#v, want Id %q", resp, "a")
		}
	})

	t.Run("valid id accepted", func(t *testing.T) {
		resp, err := svc.CreateLLMProxyAPIKey(context.Background(), "proxy", "org-1", "",
			&api.CreateLLMProxyAPIKeyRequest{DisplayName: "x", Id: strPtr("valid-key")})
		if err != nil {
			t.Fatalf("CreateLLMProxyAPIKey() = %v, want success", err)
		}
		if resp == nil || resp.Id != "valid-key" {
			t.Fatalf("CreateLLMProxyAPIKey() = %#v, want Id %q", resp, "valid-key")
		}
	})
}

func repeatChar(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

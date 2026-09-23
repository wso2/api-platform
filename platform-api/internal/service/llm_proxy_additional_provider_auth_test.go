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
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"gopkg.in/yaml.v3"
)

// newAdditionalProviderAuthTestService returns a proxy service whose repo serves
// `stored` until the first Update, then serves what was persisted.
func newAdditionalProviderAuthTestService(stored model.LLMProxyConfig) (*LLMProxyService, *mockLLMProxyRepo) {
	now := time.Now()
	proxyRepo := &mockLLMProxyRepo{}
	proxyRepo.getByIDFunc = func(proxyID, orgUUID string) (*model.LLMProxy, error) {
		if proxyRepo.updated == nil {
			return &model.LLMProxy{
				UUID: "proxy-uuid", ID: proxyID, Name: "Old Proxy", Version: "v1.0",
				ProjectUUID: "project-1", ProviderUUID: "provider-uuid",
				CreatedAt: now, UpdatedAt: now,
				Configuration: stored,
			}, nil
		}
		updated := *proxyRepo.updated
		updated.UUID = "proxy-uuid"
		updated.CreatedAt = now
		updated.UpdatedAt = now
		return &updated, nil
	}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: providerID + "-uuid", ID: providerID}, nil
		},
	}
	projectRepo := &mockProjectRepo{project: &model.Project{ID: "project-1", Handle: "project-1", OrganizationID: "org-1"}}
	service := NewLLMProxyService(proxyRepo, providerRepo, projectRepo, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
	return service, proxyRepo
}

func apiKeyAuth(header, value string) *api.UpstreamAuth {
	return &api.UpstreamAuth{Type: upstreamAuthTypePtr("api-key"), Header: stringPtr(header), Value: stringPtr(value)}
}

func additionalAuthByID(t *testing.T, providers []model.LLMProxyAdditionalProvider) map[string]*model.UpstreamAuth {
	t.Helper()
	out := make(map[string]*model.UpstreamAuth, len(providers))
	for _, ap := range providers {
		out[ap.ID] = ap.Auth
	}
	return out
}

func TestMapProxyModelToAPI_DoesNotExposeAdditionalProviderAuthValue(t *testing.T) {
	out := mapProxyModelToAPI(&model.LLMProxy{
		ID: "proxy-1", Name: "Proxy One", Version: "v1",
		Configuration: model.LLMProxyConfig{
			Provider: "provider-1",
			AdditionalProviders: []model.LLMProxyAdditionalProvider{{
				ID: "provider-2", As: "gpt-4o",
				Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-API-Key", Value: "super-secret-loopback"},
			}},
		},
	})

	if out.AdditionalProviders == nil || len(*out.AdditionalProviders) != 1 {
		t.Fatalf("expected one additional provider, got %+v", out.AdditionalProviders)
	}
	auth := (*out.AdditionalProviders)[0].Auth
	if auth == nil || auth.Type == nil || *auth.Type != api.ApiKey || auth.Header == nil || *auth.Header != "X-API-Key" {
		t.Fatalf("expected additional provider auth type and header to be returned, got %+v", auth)
	}
	if auth.Value != nil {
		t.Fatalf("expected additional provider auth value to be redacted, got %q", *auth.Value)
	}
}

func TestLLMProxyServiceCreate_StoresAdditionalProviderAuth(t *testing.T) {
	service, proxyRepo := newAdditionalProviderAuthTestService(model.LLMProxyConfig{})

	req := validProxyRequest("provider-1", "project-1")
	req.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{
		{Id: "provider-2", As: stringPtr("gpt-4o"), Auth: apiKeyAuth("X-API-Key", "loopback-key")},
		{Id: "provider-3", Auth: &api.UpstreamAuth{Type: upstreamAuthTypePtr("none"), Header: stringPtr("X-API-Key"), Value: stringPtr("dropped")}},
		{Id: "provider-4"},
	}

	if _, err := service.Create("org-1", "alice", req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	got := additionalAuthByID(t, proxyRepo.created.Configuration.AdditionalProviders)
	if a := got["provider-2"]; a == nil || a.Type != "api-key" || a.Header != "X-API-Key" || a.Value != "loopback-key" {
		t.Errorf("provider-2 auth = %+v, want api-key X-API-Key loopback-key", a)
	}
	if a := got["provider-3"]; a == nil || a.Type != "none" || a.Header != "" || a.Value != "" {
		t.Errorf("provider-3 auth = %+v, want type none with header/value dropped", a)
	}
	if a := got["provider-4"]; a != nil {
		t.Errorf("provider-4 auth = %+v, want nil (no auth configured)", a)
	}
}

// The value is redacted on read, so a round-tripped update re-sends each entry's
// auth without it. The stored credential must come back for the same provider ID,
// regardless of list order.
func TestLLMProxyServiceUpdate_PreservesAdditionalProviderAuthByID(t *testing.T) {
	service, proxyRepo := newAdditionalProviderAuthTestService(model.LLMProxyConfig{
		Provider: "provider-1",
		AdditionalProviders: []model.LLMProxyAdditionalProvider{
			{ID: "provider-2", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-API-Key", Value: "key-for-2"}},
			{ID: "provider-3", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-API-Key", Value: "key-for-3"}},
		},
	})

	req := validProxyRequest("provider-1", "project-1")
	req.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{
		{Id: "provider-3", Auth: &api.UpstreamAuth{Type: upstreamAuthTypePtr("api-key"), Header: stringPtr("X-API-Key")}},
		{Id: "provider-2", Auth: &api.UpstreamAuth{Type: upstreamAuthTypePtr("api-key")}},
	}

	if _, err := service.Update("org-1", "proxy-1", "alice", req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	got := additionalAuthByID(t, proxyRepo.updated.Configuration.AdditionalProviders)
	if a := got["provider-2"]; a == nil || a.Value != "key-for-2" || a.Header != "X-API-Key" {
		t.Errorf("provider-2 auth = %+v, want its own stored credential and header", a)
	}
	if a := got["provider-3"]; a == nil || a.Value != "key-for-3" {
		t.Errorf("provider-3 auth = %+v, want its own stored credential", a)
	}
}

func TestLLMProxyServiceUpdate_AdditionalProviderAuthNotInheritedAcrossTypeOrProvider(t *testing.T) {
	service, proxyRepo := newAdditionalProviderAuthTestService(model.LLMProxyConfig{
		Provider: "provider-1",
		AdditionalProviders: []model.LLMProxyAdditionalProvider{
			{ID: "provider-2", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-API-Key", Value: "key-for-2"}},
		},
	})

	req := validProxyRequest("provider-1", "project-1")
	req.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{
		// Same provider, different auth type: the old credential must not carry over.
		{Id: "provider-2", Auth: &api.UpstreamAuth{Type: upstreamAuthTypePtr("bearer"), Header: stringPtr("Authorization")}},
		// A provider with nothing stored has nothing to inherit.
		{Id: "provider-5", Auth: &api.UpstreamAuth{Type: upstreamAuthTypePtr("api-key"), Header: stringPtr("X-API-Key")}},
	}

	if _, err := service.Update("org-1", "proxy-1", "alice", req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	got := additionalAuthByID(t, proxyRepo.updated.Configuration.AdditionalProviders)
	if a := got["provider-2"]; a == nil || a.Value != "" {
		t.Errorf("provider-2 auth = %+v, want no credential after a type change", a)
	}
	if a := got["provider-5"]; a == nil || a.Value != "" {
		t.Errorf("provider-5 auth = %+v, want no credential for a newly added provider", a)
	}
}

func TestLLMProxyServiceUpdate_OmittedAdditionalProviderAuthIsRemoved(t *testing.T) {
	service, proxyRepo := newAdditionalProviderAuthTestService(model.LLMProxyConfig{
		Provider: "provider-1",
		AdditionalProviders: []model.LLMProxyAdditionalProvider{
			{ID: "provider-2", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-API-Key", Value: "key-for-2"}},
		},
	})

	req := validProxyRequest("provider-1", "project-1")
	req.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{{Id: "provider-2"}}

	if _, err := service.Update("org-1", "proxy-1", "alice", req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if a := proxyRepo.updated.Configuration.AdditionalProviders[0].Auth; a != nil {
		t.Errorf("provider-2 auth = %+v, want nil when the update omits auth", a)
	}
}

func TestLLMProxyService_AdditionalProviderMissingSecretRef_Rejected(t *testing.T) {
	missingRef := apiKeyAuth("X-API-Key", `{{ secret "nonexistent-loopback-secret" }}`)

	t.Run("create", func(t *testing.T) {
		service, proxyRepo := newAdditionalProviderAuthTestService(model.LLMProxyConfig{})
		service.SetSecretService(NewSecretService(newMockRepo(), &mockVault{}, newTestIdentityService()))
		req := validProxyRequest("provider-1", "project-1")
		req.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{{Id: "provider-2", Auth: missingRef}}

		if _, err := service.Create("org-1", "alice", req); !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed for missing secret ref, got: %v", err)
		}
		if proxyRepo.created != nil {
			t.Error("expected proxy creation to be aborted")
		}
	})

	t.Run("update", func(t *testing.T) {
		service, proxyRepo := newAdditionalProviderAuthTestService(model.LLMProxyConfig{Provider: "provider-1"})
		service.SetSecretService(NewSecretService(newMockRepo(), &mockVault{}, newTestIdentityService()))
		req := validProxyRequest("provider-1", "project-1")
		req.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{{Id: "provider-2", Auth: missingRef}}

		if _, err := service.Update("org-1", "proxy-1", "alice", req); !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed for missing secret ref, got: %v", err)
		}
		if proxyRepo.updated != nil {
			t.Error("expected proxy update to be aborted")
		}
	})
}

func TestLLMProxyServiceUpdate_CleansUpRotatedAdditionalProviderSecret(t *testing.T) {
	service, _ := newAdditionalProviderAuthTestService(model.LLMProxyConfig{
		Provider: "provider-1",
		AdditionalProviders: []model.LLMProxyAdditionalProvider{
			{ID: "provider-2", Auth: &model.UpstreamAuth{Type: "api-key", Header: "X-API-Key", Value: `{{ secret "old-handle" }}`}},
		},
	})
	secretRepo := newMockRepo()
	secretRepo.secrets["old-handle"] = &model.Secret{Handle: "old-handle", Status: model.SecretStatusActive}
	secretRepo.secrets["new-handle"] = &model.Secret{Handle: "new-handle", Status: model.SecretStatusActive}
	service.SetSecretService(NewSecretService(secretRepo, &mockVault{}, newTestIdentityService()))

	req := validProxyRequest("provider-1", "project-1")
	req.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{
		{Id: "provider-2", Auth: apiKeyAuth("X-API-Key", `{{ secret "new-handle" }}`)},
	}

	if _, err := service.Update("org-1", "proxy-1", "alice", req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if secretRepo.secrets["old-handle"].Status != model.SecretStatusDeprecated {
		t.Fatalf("expected rotated-out secret to be deprecated, got status=%v", secretRepo.secrets["old-handle"].Status)
	}
	if secretRepo.secrets["new-handle"].Status != model.SecretStatusActive {
		t.Fatalf("expected new secret to remain active, got status=%v", secretRepo.secrets["new-handle"].Status)
	}
}

func TestGenerateLLMProxyDeploymentYAML_CarriesAdditionalProviderAuth(t *testing.T) {
	proxy := &model.LLMProxy{
		ID: "test-proxy", Name: "Test Proxy", Version: "v1.0",
		Configuration: model.LLMProxyConfig{
			Provider: "test-provider",
			AdditionalProviders: []model.LLMProxyAdditionalProvider{
				{ID: "with-key", As: "gpt-4o", Auth: &model.UpstreamAuth{Type: "apiKey", Header: "X-API-Key", Value: "loopback-key"}},
				{ID: "credential-less", Auth: &model.UpstreamAuth{Type: "other"}},
				{ID: "no-auth"},
			},
		},
	}

	artifact, err := generateLLMProxyDeploymentYAML(proxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byID := map[string]int{}
	for i, ap := range artifact.Spec.AdditionalProviders {
		byID[ap.ID] = i
	}

	withKey := artifact.Spec.AdditionalProviders[byID["with-key"]].Auth
	if withKey == nil || withKey.Type == nil || *withKey.Type != api.ApiKey ||
		withKey.Header == nil || *withKey.Header != "X-API-Key" || withKey.Value == nil || *withKey.Value != "loopback-key" {
		t.Fatalf("with-key auth = %+v, want normalized api-key with header and value", withKey)
	}
	credentialLess := artifact.Spec.AdditionalProviders[byID["credential-less"]].Auth
	if credentialLess == nil || credentialLess.Type == nil || *credentialLess.Type != api.Other ||
		credentialLess.Header != nil || credentialLess.Value != nil {
		t.Fatalf("credential-less auth = %+v, want type 'other' only", credentialLess)
	}
	if a := artifact.Spec.AdditionalProviders[byID["no-auth"]].Auth; a != nil {
		t.Fatalf("no-auth auth = %+v, want omitted", a)
	}

	// The gateway parses spec.additionalProviders[].auth from the serialized artifact.
	out, err := yaml.Marshal(artifact)
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}
	if !strings.Contains(string(out), "value: loopback-key") {
		t.Fatalf("serialized artifact is missing the additional provider auth value:\n%s", out)
	}
}

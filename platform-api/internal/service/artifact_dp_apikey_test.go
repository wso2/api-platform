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

// Consumer API keys can be generated for DP-originated (gateway_api) artifacts from the
// control plane. API keys are not part of the gateway runtime artifact,
// so — unlike metadata/runtime edits (guarded elsewhere) — key generation carries NO origin
// guard: a DP-origin provider/proxy mints a key exactly like a control-plane one. These
// tests pin that contract so a future origin check added to the CRUD guards can't
// accidentally start rejecting API-key creation for DP artifacts.

import (
	"context"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"

	"github.com/wso2/api-platform/common/eventhub"
)

// dpNoopEventHub is an eventhub.EventHub that does nothing; PublishEvent succeeds so the
// API-key broadcast is a no-op. (The broadcast is fault-tolerant anyway — a delivery
// failure only logs; the key is still persisted and creation still succeeds.)
type dpNoopEventHub struct{}

func (dpNoopEventHub) Initialize() error                               { return nil }
func (dpNoopEventHub) RegisterGateway(string) error                    { return nil }
func (dpNoopEventHub) PublishEvent(string, eventhub.Event) error       { return nil }
func (dpNoopEventHub) Subscribe(string) (<-chan eventhub.Event, error) { return nil, nil }
func (dpNoopEventHub) Unsubscribe(string, <-chan eventhub.Event) error { return nil }
func (dpNoopEventHub) UnsubscribeAll(string) error                     { return nil }
func (dpNoopEventHub) CleanUpEvents() error                            { return nil }
func (dpNoopEventHub) Close() error                                    { return nil }

// dpKeyAPIRepo returns a single associated gateway so LLM API-key creation has a
// broadcast target. Association-scoped selection mirrors the REST key path.
type dpKeyAPIRepo struct {
	repository.APIRepository
}

func (dpKeyAPIRepo) GetAPIGatewaysWithDetails(string, string) ([]*model.APIGatewayWithDetails, error) {
	return []*model.APIGatewayWithDetails{{ID: "gw-1", Name: "gw-1"}}, nil
}

// dpKeyNoAssocAPIRepo models an artifact with NO gateway associations (undeployed).
type dpKeyNoAssocAPIRepo struct {
	repository.APIRepository
}

func (dpKeyNoAssocAPIRepo) GetAPIGatewaysWithDetails(string, string) ([]*model.APIGatewayWithDetails, error) {
	return nil, nil
}

// dpCapturingAPIKeyRepo captures the persisted API key so tests can assert it was created
// against the artifact's own UUID.
type dpCapturingAPIKeyRepo struct {
	repository.APIKeyRepository
	created *model.APIKey
}

func (c *dpCapturingAPIKeyRepo) Create(k *model.APIKey) error {
	c.created = k
	return nil
}

func newDPKeyEventsService() *GatewayEventsService {
	return NewGatewayEventsService(dpNoopEventHub{}, newTestIdentityService(), newTestLogger())
}

// TestLLMProviderAPIKey_AllowedForDPOrigin verifies a consumer API key can be generated
// for a DP-originated LLM provider (origin is not consulted by the key service).
func TestLLMProviderAPIKey_AllowedForDPOrigin(t *testing.T) {
	provider := &model.LLMProvider{
		UUID:             "dp-prov-uuid",
		ID:               "dp-prov",
		OrganizationUUID: "org-1",
		Name:             "DP Provider",
		Version:          "v1.0",
		Origin:           constants.OriginDP, // read-only for metadata/runtime edits, but keys are allowed
	}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(string, string) (*model.LLMProvider, error) { return provider, nil },
	}
	keyRepo := &dpCapturingAPIKeyRepo{}
	svc := NewLLMProviderAPIKeyService(providerRepo, dpKeyAPIRepo{}, keyRepo, newDPKeyEventsService(), newTestIdentityService(), newTestLogger())

	resp, err := svc.CreateLLMProviderAPIKey(context.Background(), "dp-prov", "org-1", "",
		&api.CreateLLMProviderAPIKeyRequest{DisplayName: "dp-consumer-key"})
	if err != nil {
		t.Fatalf("CreateLLMProviderAPIKey for DP-origin provider = %v, want success", err)
	}
	if resp == nil || resp.ApiKey == "" {
		t.Fatalf("expected a generated API key, got %#v", resp)
	}
	if keyRepo.created == nil {
		t.Fatalf("API key was not persisted")
	}
	if keyRepo.created.ArtifactUUID != provider.UUID {
		t.Errorf("persisted key ArtifactUUID = %q, want provider UUID %q", keyRepo.created.ArtifactUUID, provider.UUID)
	}
}

// TestLLMProxyAPIKey_AllowedForDPOrigin verifies a consumer API key can be generated for a
// DP-originated LLM proxy.
func TestLLMProxyAPIKey_AllowedForDPOrigin(t *testing.T) {
	proxy := &model.LLMProxy{
		UUID:             "dp-proxy-uuid",
		ID:               "dp-proxy",
		OrganizationUUID: "org-1",
		Name:             "DP Proxy",
		Version:          "v1.0",
		Origin:           constants.OriginDP,
	}
	proxyRepo := &mockLLMProxyRepo{
		getByIDFunc: func(string, string) (*model.LLMProxy, error) { return proxy, nil },
	}
	keyRepo := &dpCapturingAPIKeyRepo{}
	svc := NewLLMProxyAPIKeyService(proxyRepo, dpKeyAPIRepo{}, keyRepo, newDPKeyEventsService(), newTestIdentityService(), newTestLogger())

	resp, err := svc.CreateLLMProxyAPIKey(context.Background(), "dp-proxy", "org-1", "",
		&api.CreateLLMProxyAPIKeyRequest{DisplayName: "dp-consumer-key"})
	if err != nil {
		t.Fatalf("CreateLLMProxyAPIKey for DP-origin proxy = %v, want success", err)
	}
	if resp == nil || resp.ApiKey == "" {
		t.Fatalf("expected a generated API key, got %#v", resp)
	}
	if keyRepo.created == nil {
		t.Fatalf("API key was not persisted")
	}
	if keyRepo.created.ArtifactUUID != proxy.UUID {
		t.Errorf("persisted key ArtifactUUID = %q, want proxy UUID %q", keyRepo.created.ArtifactUUID, proxy.UUID)
	}
}

// TestCreateLLMProviderAPIKey_AssociationScoped verifies that creating a key for a
// provider with NO gateway associations persists the key and broadcasts to ZERO
// gateways — i.e. the broadcast is association-scoped, not org-wide (issue: LLM key
// events were previously sent to every org gateway, causing "artifact not found").
func TestCreateLLMProviderAPIKey_AssociationScoped(t *testing.T) {
	provider := &model.LLMProvider{
		UUID:             "prov-uuid",
		ID:               "prov",
		OrganizationUUID: "org-1",
		Name:             "Prov",
		Version:          "v1.0",
	}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(string, string) (*model.LLMProvider, error) { return provider, nil },
	}
	keyRepo := &dpCapturingAPIKeyRepo{}
	hub := &capturingEventHub{}
	events := NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger())

	svc := NewLLMProviderAPIKeyService(providerRepo, dpKeyNoAssocAPIRepo{}, keyRepo,
		events, newTestIdentityService(), newTestLogger())

	resp, err := svc.CreateLLMProviderAPIKey(context.Background(), "prov", "org-1", "",
		&api.CreateLLMProviderAPIKeyRequest{DisplayName: "k"})
	if err != nil {
		t.Fatalf("CreateLLMProviderAPIKey with no associations = %v, want success", err)
	}
	if resp == nil || resp.ApiKey == "" {
		t.Fatalf("expected a generated key, got %#v", resp)
	}
	if keyRepo.created == nil {
		t.Fatalf("key was not persisted")
	}
	if len(hub.published) != 0 {
		t.Fatalf("expected 0 broadcasts for an unassociated provider, got %d", len(hub.published))
	}
}

// TestCreateLLMProxyAPIKey_AssociationScoped is the LLM-proxy counterpart.
func TestCreateLLMProxyAPIKey_AssociationScoped(t *testing.T) {
	proxy := &model.LLMProxy{
		UUID:             "proxy-uuid",
		ID:               "proxy",
		OrganizationUUID: "org-1",
		Name:             "Proxy",
		Version:          "v1.0",
	}
	proxyRepo := &mockLLMProxyRepo{
		getByIDFunc: func(string, string) (*model.LLMProxy, error) { return proxy, nil },
	}
	keyRepo := &dpCapturingAPIKeyRepo{}
	hub := &capturingEventHub{}
	events := NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger())

	svc := NewLLMProxyAPIKeyService(proxyRepo, dpKeyNoAssocAPIRepo{}, keyRepo,
		events, newTestIdentityService(), newTestLogger())

	resp, err := svc.CreateLLMProxyAPIKey(context.Background(), "proxy", "org-1", "",
		&api.CreateLLMProxyAPIKeyRequest{DisplayName: "k"})
	if err != nil {
		t.Fatalf("CreateLLMProxyAPIKey with no associations = %v, want success", err)
	}
	if resp == nil || resp.ApiKey == "" {
		t.Fatalf("expected a generated key, got %#v", resp)
	}
	if keyRepo.created == nil {
		t.Fatalf("key was not persisted")
	}
	if len(hub.published) != 0 {
		t.Fatalf("expected 0 broadcasts for an unassociated proxy, got %d", len(hub.published))
	}
}

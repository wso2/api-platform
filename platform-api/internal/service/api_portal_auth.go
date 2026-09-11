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

// Shared-key outbound authentication for Platform-API to API Portal admin API calls.
// The registry decrypts each portal's key on first use and caches the built provider.

package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

// AuthProvider yields the Authorization header for outbound calls to a portal's admin REST endpoints.
type AuthProvider interface {
	// AuthorizationHeader returns the fully-formed Authorization header value (e.g. "SharedKey <raw>").
	AuthorizationHeader(ctx context.Context) (string, error)

	// InvalidateCache lets provider implementations flush an in-provider cache without a whole-registry drop.
	InvalidateCache()
}

// sharedKeyAuthProvider serves "SharedKey <raw>" for a single portal; fields are read-only after construction.
type sharedKeyAuthProvider struct {
	header string // "SharedKey <raw>", the exact bytes sent on the wire
}

// NewSharedKeyAuthProvider decrypts the stored key once and returns a provider that caches the resulting header.
func NewSharedKeyAuthProvider(v vault.SecretVault, encryptedKey []byte) (AuthProvider, error) {
	if v == nil {
		return nil, fmt.Errorf("shared-key AuthProvider: vault is nil")
	}
	if len(encryptedKey) == 0 {
		return nil, fmt.Errorf("shared-key AuthProvider: encrypted key is empty")
	}
	raw, err := v.Decrypt(context.Background(), encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("shared-key AuthProvider: decrypt: %w", err)
	}
	return &sharedKeyAuthProvider{
		header: constants.APIPortalSharedKeyAuthScheme + " " + raw,
	}, nil
}

func (p *sharedKeyAuthProvider) AuthorizationHeader(_ context.Context) (string, error) {
	return p.header, nil
}

func (p *sharedKeyAuthProvider) InvalidateCache() {
	// No-op: rotation is handled at the registry level by dropping the whole provider.
}

// APIPortalAuthRegistry caches at most one AuthProvider per (orgID, handle); providers are built on Get miss.
type APIPortalAuthRegistry struct {
	mu          sync.Mutex
	providers   map[string]AuthProvider // key = registryKey(orgID, handle)
	generations map[string]uint64       // bumped by Invalidate; guards cache-fill races
	portalRepo  repository.APIPortalRepository
	vault       vault.SecretVault
}

// NewAPIPortalAuthRegistry constructs the registry.
func NewAPIPortalAuthRegistry(portalRepo repository.APIPortalRepository, v vault.SecretVault) *APIPortalAuthRegistry {
	return &APIPortalAuthRegistry{
		providers:   map[string]AuthProvider{},
		generations: map[string]uint64{},
		portalRepo:  portalRepo,
		vault:       v,
	}
}

// registryKey composes a stable per-(org, portal) cache key so cross-org lookups never collide.
func registryKey(orgID, portalHandle string) string {
	return orgID + "/" + portalHandle
}

// Invalidate drops the cached provider and bumps the generation so a concurrent Get cannot repopulate a stale entry.
func (r *APIPortalAuthRegistry) Invalidate(portalHandle, orgID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := registryKey(orgID, portalHandle)
	delete(r.providers, key)
	r.generations[key]++
}

// Get returns the AuthProvider for the (org, portal) pair, constructing and caching on first call.
func (r *APIPortalAuthRegistry) Get(portalHandle, orgID string) (AuthProvider, error) {
	if r == nil {
		return nil, fmt.Errorf("shared-key AuthProvider registry is not initialised")
	}
	key := registryKey(orgID, portalHandle)

	r.mu.Lock()
	if p, ok := r.providers[key]; ok {
		r.mu.Unlock()
		return p, nil
	}
	genSnapshot := r.generations[key]
	r.mu.Unlock()

	portal, err := r.portalRepo.GetByHandleAndOrgID(portalHandle, orgID)
	if err != nil {
		return nil, err
	}
	if portal == nil {
		return nil, apperror.APIPortalNotFound.New()
	}

	provider, err := NewSharedKeyAuthProvider(r.vault, portal.InternalAuthKey)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.generations[key] != genSnapshot {
		// Invalidate ran during decrypt; skip cache write so a stale provider never sticks.
		return provider, nil
	}
	if existing, ok := r.providers[key]; ok {
		return existing, nil
	}
	r.providers[key] = provider
	return provider, nil
}

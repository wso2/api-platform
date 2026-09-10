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

// Shared-key outbound authentication for Platform-API → API Portal admin API
// calls. Platform-API stores each portal's shared-key encrypted-at-rest in the
// api_portals.internal_auth_key column; the outbound publisher calls
// AuthHeaderForPortal to get the "SharedKey <raw>" header value to attach to
// each publish request. The registry decrypts each portal's key exactly once
// (on first use), caches the plaintext in memory, and drops it when the row
// changes (Update / Delete). See SharedKey-Auth-Design.md for the mechanism.
//
// The registry is instantiated once at server startup and shared by every
// publisher; concurrent Gets for the same portal race safely (map is guarded
// by a mutex; a lost race is idempotent, both callers get equivalent
// providers).

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

// AuthProvider is the outbound-auth surface exposed to any component that
// needs to call a portal's admin REST endpoints.
type AuthProvider interface {
	// AuthorizationHeader returns the fully-formed Authorization header value
	// for the next outbound publish call to this portal, e.g.
	// "SharedKey <raw>". Provider implementations decrypt / cache the raw
	// once at construction and hand out the same header for the provider's
	// lifetime; rotation of the underlying stored key is handled at the
	// registry level (Invalidate drops the cached provider so the next Get
	// re-reads the row and re-decrypts).
	AuthorizationHeader(ctx context.Context) (string, error)

	// InvalidateCache is a per-provider no-op today. Included on the
	// interface so future provider implementations (e.g. one that fetches
	// the raw from OpenBao on every call) can flush an in-provider cache
	// without a whole-registry drop.
	InvalidateCache()
}

// sharedKeyAuthProvider serves "SharedKey <raw>" for a single portal. Fields
// are read-only after construction; safe to share across goroutines.
type sharedKeyAuthProvider struct {
	header string // "SharedKey <raw>" — the exact bytes sent on the wire
}

// NewSharedKeyAuthProvider constructs a provider from a portal's encrypted
// shared-key bytes. Decryption happens once, the plaintext lives inside the
// returned provider (never persisted, never re-encrypted).
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
	// no-op: SharedKey plaintext is fixed for a provider's lifetime; when
	// the stored key rotates, the service calls registry.Invalidate which
	// drops the whole provider so the next Get rebuilds from the fresh row.
}

// APIPortalAuthRegistry holds at most one AuthProvider per portal (keyed by
// orgID + handle). The service's Update / Delete paths call Invalidate so the
// next outbound call picks up whatever the row now says. Provider construction
// (Decrypt on the row's internal_auth_key) happens on Get miss.
type APIPortalAuthRegistry struct {
	mu         sync.Mutex
	providers  map[string]AuthProvider // key = registryKey(orgID, handle)
	portalRepo repository.APIPortalRepository
	vault      vault.SecretVault
}

// NewAPIPortalAuthRegistry constructs the registry. portalRepo is used to load
// a row's internal_auth_key when Get misses the cache; vault is used to
// decrypt that value.
func NewAPIPortalAuthRegistry(portalRepo repository.APIPortalRepository, v vault.SecretVault) *APIPortalAuthRegistry {
	return &APIPortalAuthRegistry{
		providers:  map[string]AuthProvider{},
		portalRepo: portalRepo,
		vault:      v,
	}
}

// registryKey composes a stable per-(org, portal) cache key. Same handle in
// different orgs get different entries so a Get for one org never returns
// another org's provider — the DB row lookup would fail cross-org anyway
// (GetByHandleAndOrgID filters on organization_uuid), but keeping the cache
// keyed on both means the miss path stays correct without racing.
func registryKey(orgID, portalHandle string) string {
	return orgID + "/" + portalHandle
}

// Invalidate drops the cached provider for a portal handle in an org. No-op
// when there is no cached entry (idempotent, safe to call from Delete paths).
// Called by the service on every Update / Delete of a portal row.
func (r *APIPortalAuthRegistry) Invalidate(portalHandle, orgID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, registryKey(orgID, portalHandle))
}

// Get returns the AuthProvider for the (org, portal) pair, constructing +
// caching on first call. Returns APIPortalNotFound when the row is not
// present. Any decryption failure (row bytes not encrypted with the current
// vault key, or corrupted ciphertext) surfaces as a plain error the caller
// treats as a permanent configuration problem.
//
// Concurrent Gets for the same key race safely: the first one wins the map
// slot, subsequent ones return that stored provider (double-check under lock
// avoids constructing more than once). A rare double-decrypt on a lost race
// is preferable to holding the map lock across an I/O call to portalRepo.
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
	// Another goroutine may have installed a provider while we were
	// decrypting; prefer the existing one to keep a single instance per key.
	if existing, ok := r.providers[key]; ok {
		r.mu.Unlock()
		return existing, nil
	}
	r.providers[key] = provider
	r.mu.Unlock()
	return provider, nil
}

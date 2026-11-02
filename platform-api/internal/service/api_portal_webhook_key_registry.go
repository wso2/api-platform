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

// Per-portal webhook signing-key registry. Caches the pre-built verifier +
// decryptor + owning org for each portal uuid, decrypting the secret from the
// vault on first use. Modelled on APIPortalAuthRegistry: the same generation-
// guarded fill-race pattern, but keyed on portal uuid alone since the receiver
// only has that identifier until the HMAC is verified.
//
// Cache entries expire after cacheEntryTTL so a rotation performed on one
// Platform API replica is honoured on every replica within that window
// without any cross-replica coordination. The TTL is deliberately much
// shorter than webhook.rotation_grace (default 10 min) so a stale replica
// never serves a rotated-out secret past the grace deadline — which would
// otherwise be a security hole: a leaked secret would stay valid on that
// replica until a restart, independently of the grace window ending.

package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/vault"
	"github.com/wso2/api-platform/platform-api/internal/webhook"
)

// cacheEntryTTL bounds how long a cached entry is served without re-reading
// the DB. Chosen to be much shorter than webhook.rotation_grace (default 10
// min) so a secret rotated on one Platform API replica is honoured on every
// replica well within the grace window — a stale replica cannot keep
// accepting a rotated-out secret past the grace deadline.
const cacheEntryTTL = 60 * time.Second

// cachedEntry pairs the per-portal keys with the time they were loaded, so
// Get can treat entries older than cacheEntryTTL as a miss.
type cachedEntry struct {
	keys     *webhook.PortalWebhookKeys
	loadedAt time.Time
}

// PortalWebhookKeyRegistry caches at most one *webhook.PortalWebhookKeys per
// portal uuid. Build-on-miss, generation-guarded against races with Invalidate.
// The cached value type lives in the webhook package so the receiver can
// consume it without importing service.
type PortalWebhookKeyRegistry struct {
	mu          sync.Mutex
	entries     map[string]cachedEntry // key = portal uuid
	generations map[string]uint64      // bumped by Invalidate; guards fill races
	portalRepo  repository.APIPortalRepository
	secretRepo  repository.APIPortalWebhookSecretRepository
	vault       vault.SecretVault
	tolerance   time.Duration
	// now is overridable in tests so the TTL path is deterministic.
	// Production code uses time.Now.
	now func() time.Time
}

// NewPortalWebhookKeyRegistry constructs the registry. tolerance is the HMAC
// signature freshness window from webhook.signature_tolerance.
func NewPortalWebhookKeyRegistry(
	portalRepo repository.APIPortalRepository,
	secretRepo repository.APIPortalWebhookSecretRepository,
	v vault.SecretVault,
	tolerance time.Duration,
) *PortalWebhookKeyRegistry {
	return &PortalWebhookKeyRegistry{
		entries:     map[string]cachedEntry{},
		generations: map[string]uint64{},
		portalRepo:  portalRepo,
		secretRepo:  secretRepo,
		vault:       v,
		tolerance:   tolerance,
		now:         time.Now,
	}
}

// Invalidate drops the cached entry for portalUUID and bumps the generation so
// a concurrent Get cannot repopulate stale state.
func (r *PortalWebhookKeyRegistry) Invalidate(portalUUID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, portalUUID)
	r.generations[portalUUID]++
}

// Get returns the *webhook.PortalWebhookKeys for portalUUID. On a cache miss it
// loads the portal (for the owning org) and its secret row, decrypts both
// secret slots, and builds verifier/decryptor pairs. An unknown portal uuid,
// or a portal with no webhook secret row, maps to apperror.APIPortalNotFound —
// the receiver converts this to a generic 401 so the response does not reveal
// whether the uuid identifies a real portal.
func (r *PortalWebhookKeyRegistry) Get(ctx context.Context, portalUUID string) (*webhook.PortalWebhookKeys, error) {
	if r == nil {
		return nil, fmt.Errorf("per-portal webhook key registry is not initialised")
	}

	for {
		now := r.now()
		r.mu.Lock()
		if entry, ok := r.entries[portalUUID]; ok && now.Sub(entry.loadedAt) < cacheEntryTTL {
			r.mu.Unlock()
			return entry.keys, nil
		}
		genSnapshot := r.generations[portalUUID]
		r.mu.Unlock()

		portal, err := r.portalRepo.GetByUUIDAny(portalUUID)
		if err != nil {
			return nil, err
		}
		if portal == nil {
			return nil, apperror.APIPortalNotFound.New()
		}

		secretRow, err := r.secretRepo.GetByPortalUUID(portalUUID)
		if err != nil {
			return nil, err
		}
		if secretRow == nil {
			return nil, apperror.APIPortalNotFound.New()
		}

		currentPlain, err := r.vault.Decrypt(ctx, secretRow.SecretEnc)
		if err != nil {
			return nil, fmt.Errorf("per-portal webhook key registry: decrypt current: %w", err)
		}
		currentDecryptor, err := webhook.NewDecryptor(currentPlain)
		if err != nil {
			return nil, fmt.Errorf("per-portal webhook key registry: build current decryptor: %w", err)
		}
		keys := &webhook.PortalWebhookKeys{
			PortalUUID:       portalUUID,
			OrganizationUUID: portal.OrganizationID,
			Current:          webhook.NewVerifier(currentPlain, r.tolerance),
			CurrentDecryptor: currentDecryptor,
		}
		// Only treat the previous slot as live inside its grace window. Past the
		// deadline, lazy-clear it so the DB row doesn't carry ciphertext for a
		// rotated-out secret indefinitely, and do not build a verifier/decryptor
		// for it.
		if len(secretRow.PreviousSecretEnc) > 0 && secretRow.PreviousValidUntil.Valid {
			if now.Before(secretRow.PreviousValidUntil.Time) {
				previousPlain, err := r.vault.Decrypt(ctx, secretRow.PreviousSecretEnc)
				if err != nil {
					return nil, fmt.Errorf("per-portal webhook key registry: decrypt previous: %w", err)
				}
				previousDecryptor, err := webhook.NewDecryptor(previousPlain)
				if err != nil {
					return nil, fmt.Errorf("per-portal webhook key registry: build previous decryptor: %w", err)
				}
				keys.Previous = webhook.NewVerifier(previousPlain, r.tolerance)
				keys.PreviousDecryptor = previousDecryptor
				keys.PreviousValidUntil = secretRow.PreviousValidUntil.Time
			} else {
				if err := r.secretRepo.ClearExpiredPrevious(portalUUID, now); err != nil {
					// Non-fatal — the receiver's own grace check still rejects the expired
					// previous secret, so the row's stale ciphertext isn't a correctness
					// risk even if the cleanup fails.
					slog.Warn("per-portal webhook key registry: ClearExpiredPrevious failed",
						"portalUUID", portalUUID, "err", err)
				}
			}
		}

		r.mu.Lock()
		if r.generations[portalUUID] != genSnapshot {
			// Invalidate ran during decrypt; the ciphertext we just read may be
			// stale — retry.
			r.mu.Unlock()
			continue
		}
		if existing, ok := r.entries[portalUUID]; ok && now.Sub(existing.loadedAt) < cacheEntryTTL {
			r.mu.Unlock()
			return existing.keys, nil
		}
		r.entries[portalUUID] = cachedEntry{keys: keys, loadedAt: now}
		r.mu.Unlock()
		return keys, nil
	}
}

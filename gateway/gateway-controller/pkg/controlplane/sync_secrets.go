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

package controlplane

import (
	"log/slog"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// syncSecrets pulls secrets from the Platform API and upserts them into local
// encrypted storage so {{ secret "handle" }} placeholders resolve at render time.
//
// Startup (empty cache): single bulk request with ?includeValues=true — the Platform
// API decrypts all referenced secrets server-side and returns plaintext in one response,
// avoiding N per-secret round trips.
//
// Reconnect (warm cache): metadata-only request, then per-secret /value calls only
// for secrets whose hash has changed since last sync.
func (c *Client) syncSecrets() {
	if c.IsOnPrem() {
		c.logger.Debug("Skipping secret sync: on-prem control plane detected")
		return
	}
	if c.apiUtilsService == nil {
		c.logger.Debug("Skipping secret sync: apiUtilsService is nil")
		return
	}
	if c.secretSyncer == nil {
		c.logger.Debug("Skipping secret sync: secretSyncer is nil")
		return
	}

	// Determine whether the hash cache is empty (startup / first connect).
	cacheEmpty := true
	c.secretHashCache.Range(func(_, _ any) bool {
		cacheEmpty = false
		return false
	})

	if cacheEmpty {
		c.syncSecretsBulk()
	} else {
		c.syncSecretsIncremental()
	}
}

// syncSecretsBulk is used on startup when the local hash cache is empty.
// Fetches all referenced secrets with decrypted values in a single request.
func (c *Client) syncSecretsBulk() {
	c.logger.Info("Starting bulk Platform API secret sync (startup)")

	metas, err := c.apiUtilsService.FetchPlatformSecrets(nil, true)
	if err != nil {
		c.logger.Error("Failed to bulk fetch platform secrets", slog.Any("error", err))
		return
	}

	synced, skipped, failed := 0, 0, 0
	activeHandles := make(map[string]struct{}, len(metas))

	for _, meta := range metas {
		if meta.Status != "ACTIVE" {
			skipped++
			continue
		}
		activeHandles[meta.Handle] = struct{}{}

		if meta.Value == nil {
			c.logger.Warn("Bulk fetch returned no value for secret — skipping",
				slog.String("handle", meta.Handle),
			)
			failed++
			continue
		}

		if err := c.secretSyncer.UpsertFromPlatform(meta.Handle, meta.DisplayName, *meta.Value); err != nil {
			c.logger.Error("Failed to upsert secret from platform",
				slog.String("handle", meta.Handle),
				slog.Any("error", err),
			)
			failed++
			continue
		}

		c.secretHashCache.Store(meta.Handle, meta.Hash)
		synced++
	}

	evicted := c.evictSecretsNotIn(activeHandles)

	c.logger.Info("Bulk Platform API secret sync complete",
		slog.Int("synced", synced),
		slog.Int("skipped", skipped),
		slog.Int("failed", failed),
		slog.Int("evicted", evicted),
	)
}

// syncSecretsIncremental is used on reconnect when the local hash cache is warm.
// Fetches metadata only, then fetches plaintext only for secrets whose hash changed.
func (c *Client) syncSecretsIncremental() {
	c.logger.Info("Starting incremental Platform API secret sync (reconnect)")

	metas, err := c.apiUtilsService.FetchPlatformSecrets(nil, false)
	if err != nil {
		c.logger.Error("Failed to fetch platform secrets metadata", slog.Any("error", err))
		return
	}

	synced, skipped, failed := 0, 0, 0
	activeHandles := make(map[string]struct{}, len(metas))

	for _, meta := range metas {
		if meta.Status != "ACTIVE" {
			skipped++
			continue
		}
		activeHandles[meta.Handle] = struct{}{}

		// Skip if hash unchanged since last sync.
		if cached, ok := c.secretHashCache.Load(meta.Handle); ok && cached.(string) == meta.Hash {
			skipped++
			continue
		}

		plaintext, err := c.apiUtilsService.FetchPlatformSecretValue(meta.Handle)
		if err != nil {
			c.logger.Error("Failed to fetch platform secret value",
				slog.String("handle", meta.Handle),
				slog.Any("error", err),
			)
			failed++
			continue
		}

		if err := c.secretSyncer.UpsertFromPlatform(meta.Handle, meta.DisplayName, plaintext); err != nil {
			c.logger.Error("Failed to upsert secret from platform",
				slog.String("handle", meta.Handle),
				slog.Any("error", err),
			)
			failed++
			continue
		}

		c.secretHashCache.Store(meta.Handle, meta.Hash)
		synced++
	}

	evicted := c.evictSecretsNotIn(activeHandles)

	c.logger.Info("Incremental Platform API secret sync complete",
		slog.Int("synced", synced),
		slog.Int("skipped", skipped),
		slog.Int("failed", failed),
		slog.Int("evicted", evicted),
	)
}

// evictSecretsNotIn is the poll-based recovery path for the same eviction that
// handleSecretDeletedEvent applies live: a gateway that is disconnected at the
// moment a secret is deleted only receives that secret.deleted WebSocket event if
// it's connected when the event is broadcast. This diffs the
// latest Platform API response against secretHashCache so that, on the next
// reconnect/poll, any cached handle that is no longer ACTIVE (permanently deleted,
// or flipped to a non-ACTIVE status) gets evicted from local storage even though
// the live event was missed.
//
// activeHandles is the set of handles the just-completed poll returned with
// status ACTIVE; every other handle currently in secretHashCache is stale.
func (c *Client) evictSecretsNotIn(activeHandles map[string]struct{}) int {
	var stale []string
	c.secretHashCache.Range(func(key, _ any) bool {
		handle, ok := key.(string)
		if !ok {
			return true
		}
		if _, ok := activeHandles[handle]; !ok {
			stale = append(stale, handle)
		}
		return true
	})

	for _, handle := range stale {
		correlationID := utils.GenerateDeterministicUUIDv7(handle, time.Now())
		if err := c.secretSyncer.Delete(handle, correlationID); err != nil {
			c.logger.Warn("Failed to evict stale secret from local store",
				slog.String("secret_handle", handle),
				slog.String("correlation_id", correlationID),
				slog.Any("error", err),
			)
			continue
		}
		c.secretHashCache.Delete(handle)
		c.logger.Info("Evicted stale secret from local store during poll sync",
			slog.String("secret_handle", handle),
			slog.String("correlation_id", correlationID),
		)
	}

	return len(stale)
}

// syncSecretRefsFromYAML extracts {{ secret "handle" }} placeholders from the
// supplied YAML, then fetches and upserts any handle that is not already in the
// local hash cache. This is called from deployment event handlers so that secrets
// created after the last startup/reconnect sync are available for template
// rendering when the artifact arrives.
func (c *Client) syncSecretRefsFromYAML(yamlData []byte, correlationID string) {
	if c.apiUtilsService == nil || c.secretSyncer == nil {
		return
	}

	matches := constants.SecretPlaceholderRe.FindAllSubmatch(yamlData, -1)
	if len(matches) == 0 {
		return
	}

	// Deduplicate handles.
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		if len(m) >= 2 {
			seen[string(m[1])] = struct{}{}
		}
	}

	for handle := range seen {
		// Skip handles that were already synced (present in the hash cache).
		if _, cached := c.secretHashCache.Load(handle); cached {
			continue
		}

		c.logger.Info("Fetching secret referenced in artifact",
			slog.String("secret_handle", handle),
			slog.String("correlation_id", correlationID),
		)

		plaintext, err := c.apiUtilsService.FetchPlatformSecretValue(handle)
		if err != nil {
			c.logger.Error("Failed to fetch secret value for artifact",
				slog.String("secret_handle", handle),
				slog.String("correlation_id", correlationID),
				slog.Any("error", err),
			)
			continue
		}

		if err := c.secretSyncer.UpsertFromPlatform(handle, handle, plaintext); err != nil {
			c.logger.Error("Failed to upsert secret for artifact",
				slog.String("secret_handle", handle),
				slog.String("correlation_id", correlationID),
				slog.Any("error", err),
			)
			continue
		}

		// Store a sentinel in the cache so subsequent events don't re-fetch.
		// The real hash is unknown here (we fetched by handle, not via list);
		// store an empty string so the incremental sync will still refresh it
		// on the next reconnect if the value changes.
		c.secretHashCache.Store(handle, "")
	}
}

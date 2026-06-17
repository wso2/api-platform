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

	for _, meta := range metas {
		if meta.Status != "ACTIVE" {
			skipped++
			continue
		}

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

	c.logger.Info("Bulk Platform API secret sync complete",
		slog.Int("synced", synced),
		slog.Int("skipped", skipped),
		slog.Int("failed", failed),
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

	for _, meta := range metas {
		if meta.Status != "ACTIVE" {
			skipped++
			continue
		}

		// Skip if hash unchanged since last sync.
		if cached, ok := c.secretHashCache.Load(meta.Handle); ok && cached.(string) == meta.Hash {
			skipped++
			continue
		}

		plaintext, err := c.apiUtilsService.FetchPlatformSecretValue(meta.ID)
		if err != nil {
			c.logger.Error("Failed to fetch platform secret value",
				slog.String("secret_id", meta.ID),
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

	c.logger.Info("Incremental Platform API secret sync complete",
		slog.Int("synced", synced),
		slog.Int("skipped", skipped),
		slog.Int("failed", failed),
	)
}

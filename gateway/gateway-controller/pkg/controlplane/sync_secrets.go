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
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
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
//
// Either way, deployed artifacts that use a secret whose value changed are
// re-rendered (rerenderSecretConsumers).
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
	changed := make(map[string]struct{})

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

		valueChanged, err := c.storeSecret(meta.Handle, meta.DisplayName, *meta.Value)
		if err != nil {
			c.logger.Error("Failed to upsert secret from platform",
				slog.String("handle", meta.Handle),
				slog.Any("error", err),
			)
			failed++
			continue
		}
		if valueChanged {
			changed[meta.Handle] = struct{}{}
		}

		c.secretHashCache.Store(meta.Handle, meta.Hash)
		synced++
	}
	c.rerenderSecretConsumers(changed, "")

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
	changed := make(map[string]struct{})

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

		plaintext, err := c.apiUtilsService.FetchPlatformSecretValue(meta.Handle)
		if err != nil {
			c.logger.Error("Failed to fetch platform secret value",
				slog.String("handle", meta.Handle),
				slog.Any("error", err),
			)
			failed++
			continue
		}

		valueChanged, err := c.storeSecret(meta.Handle, meta.DisplayName, plaintext)
		if err != nil {
			c.logger.Error("Failed to upsert secret from platform",
				slog.String("handle", meta.Handle),
				slog.Any("error", err),
			)
			failed++
			continue
		}
		if valueChanged {
			changed[meta.Handle] = struct{}{}
		}

		c.secretHashCache.Store(meta.Handle, meta.Hash)
		synced++
	}
	c.rerenderSecretConsumers(changed, "")

	c.logger.Info("Incremental Platform API secret sync complete",
		slog.Int("synced", synced),
		slog.Int("skipped", skipped),
		slog.Int("failed", failed),
	)
}

// syncSecretRefsFromYAML extracts {{ secret "handle" }} placeholders from the
// supplied YAML and fetches the current value of each one before the artifact
// is rendered. It is called from deployment event handlers. Every referenced
// handle is fetched, cached or not, so a deployment never renders a value that
// was rotated since the last sync; other deployed artifacts that use a changed
// value are re-rendered too.
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

	changed := make(map[string]struct{})
	for handle := range seen {
		c.logger.Info("Fetching secret referenced in artifact",
			slog.String("secret_handle", handle),
			slog.String("correlation_id", correlationID),
		)

		valueChanged, err := c.refreshSecret(handle)
		if err != nil {
			c.logger.Error("Failed to fetch secret value for artifact",
				slog.String("secret_handle", handle),
				slog.String("correlation_id", correlationID),
				slog.Any("error", err),
			)
			continue
		}
		if valueChanged {
			changed[handle] = struct{}{}
		}
	}
	c.rerenderSecretConsumers(changed, correlationID)
}

// handleSecretUpdatedEvent applies a secret.updated event, which the Platform
// API sends when a secret's value is rotated: it fetches the new value and
// re-renders the deployed artifacts that use it. The event carries the handle
// only, never the value.
func (c *Client) handleSecretUpdatedEvent(event map[string]interface{}) {
	correlationID, _ := event["correlationId"].(string)
	payload, _ := event["payload"].(map[string]interface{})
	handle, _ := payload["handle"].(string)
	if handle == "" {
		c.logger.Warn("Ignoring secret.updated event without a handle",
			slog.String("correlation_id", correlationID))
		return
	}
	if c.IsOnPrem() || c.apiUtilsService == nil || c.secretSyncer == nil {
		return
	}

	changed, err := c.refreshSecret(handle)
	if err != nil {
		c.logger.Error("Failed to refresh rotated secret",
			slog.String("secret_handle", handle),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		return
	}
	if changed {
		c.rerenderSecretConsumers(map[string]struct{}{handle: {}}, correlationID)
	}
}

// refreshSecret fetches handle's value from the Platform API by handle and
// stores it, reporting whether the stored value changed. The value's hash is
// not known here, so the cache entry is cleared to "" when the value changed
// and the next reconnect sync checks it again.
func (c *Client) refreshSecret(handle string) (bool, error) {
	plaintext, err := c.apiUtilsService.FetchPlatformSecretValue(handle)
	if err != nil {
		return false, err
	}
	changed, err := c.storeSecret(handle, "", plaintext)
	if err != nil {
		return false, err
	}
	if _, cached := c.secretHashCache.Load(handle); changed || !cached {
		c.secretHashCache.Store(handle, "")
	}
	return changed, nil
}

// storeSecret upserts a value fetched from the Platform API and reports whether
// it differs from the stored one. Comparing values rather than trusting the
// caller keeps a replica that already holds the value (secrets live in the
// shared database) from re-rendering again. An empty displayName keeps the
// stored one, or uses the handle for a new secret.
func (c *Client) storeSecret(handle, displayName, plaintext string) (bool, error) {
	existing, getErr := c.secretSyncer.Get(handle, "")
	if getErr == nil && displayName == "" {
		displayName = existing.DisplayName
	}
	if displayName == "" {
		displayName = handle
	}
	if getErr == nil && existing.Value == plaintext && existing.DisplayName == displayName {
		return false, nil
	}
	if err := c.secretSyncer.UpsertFromPlatform(handle, displayName, plaintext); err != nil {
		return false, err
	}
	return getErr != nil || existing.Value != plaintext, nil
}

// rerenderSecretConsumers publishes a replica-sync UPDATE event for every
// deployed artifact whose source configuration references one of handles. Each
// controller replica, this one included, then re-renders the artifact from the
// database with the stored secret values, the way it applies a deployment.
func (c *Client) rerenderSecretConsumers(handles map[string]struct{}, correlationID string) {
	if len(handles) == 0 || c.db == nil || c.eventHub == nil {
		return
	}
	configs, err := c.db.GetAllConfigs()
	if err != nil {
		c.logger.Error("Failed to load artifacts to re-render after a secret change",
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		return
	}
	for _, cfg := range configs {
		if cfg == nil || cfg.DesiredState != models.StateDeployed || !referencesSecret(cfg.SourceConfiguration, handles) {
			continue
		}
		eventType, ok := utils.ReplicaSyncEventType(cfg.Kind)
		if !ok {
			continue
		}
		event := eventhub.Event{
			GatewayID:           c.gatewayID,
			OriginatedTimestamp: time.Now(),
			EventType:           eventType,
			Action:              "UPDATE",
			EntityID:            cfg.UUID,
			EventID:             uuid.New().String(),
			EventData:           eventhub.EmptyEventData,
		}
		if err := c.eventHub.PublishEvent(c.gatewayID, event); err != nil {
			c.logger.Error("Failed to publish re-render event after a secret change",
				slog.String("id", cfg.UUID),
				slog.String("kind", cfg.Kind),
				slog.String("correlation_id", correlationID),
				slog.Any("error", err),
			)
			continue
		}
		c.logger.Info("Re-rendering artifact after a secret change",
			slog.String("id", cfg.UUID),
			slog.String("kind", cfg.Kind),
			slog.String("correlation_id", correlationID),
		)
	}
}

// referencesSecret reports whether the unrendered configuration has a
// {{ secret "handle" }} placeholder for one of handles.
func referencesSecret(source any, handles map[string]struct{}) bool {
	if source == nil {
		return false
	}
	raw, err := json.Marshal(source)
	if err != nil {
		return false
	}
	for _, m := range constants.SecretPlaceholderRe.FindAllSubmatch(raw, -1) {
		if _, ok := handles[string(m[1])]; ok {
			return true
		}
	}
	return false
}

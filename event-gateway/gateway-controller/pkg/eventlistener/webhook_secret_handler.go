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

// Package eventlistener implements gateway-controller (core)'s
// eventlistener.WebhookSecretEventHandler interface, moved out of core's
// pkg/eventlistener/webhook_secret_processor.go.
package eventlistener

import (
	"log/slog"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/common/webhooksecret"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"

	"github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/webhooksecretxds"
)

// WebhookSecretHandler implements gateway-controller's
// eventlistener.WebhookSecretEventHandler interface.
type WebhookSecretHandler struct {
	db                           storage.Storage
	providerManager              *encryption.ProviderManager
	webhookSecretStore           *webhooksecret.WebhookSecretStore
	webhookSecretSnapshotManager *webhooksecretxds.SnapshotManager
	logger                       *slog.Logger
}

// NewWebhookSecretHandler creates a new WebhookSecretHandler.
func NewWebhookSecretHandler(
	db storage.Storage,
	providerManager *encryption.ProviderManager,
	webhookSecretStore *webhooksecret.WebhookSecretStore,
	webhookSecretSnapshotManager *webhooksecretxds.SnapshotManager,
	logger *slog.Logger,
) *WebhookSecretHandler {
	return &WebhookSecretHandler{
		db:                           db,
		providerManager:              providerManager,
		webhookSecretStore:           webhookSecretStore,
		webhookSecretSnapshotManager: webhookSecretSnapshotManager,
		logger:                       logger,
	}
}

// HandleEvent dispatches webhook secret events by action.
func (h *WebhookSecretHandler) HandleEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE":
		h.handleWebhookSecretUpsert(event)
	case "DELETE":
		h.handleWebhookSecretDelete(event)
	default:
		h.logger.Warn("Unknown webhook secret event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

// handleWebhookSecretUpsert handles webhook secret create/regenerate events.
// It fetches the secret from the DB by UUID, decrypts it, and updates the in-memory store.
func (h *WebhookSecretHandler) handleWebhookSecretUpsert(event eventhub.Event) {
	artifactUUID, secretUUID, secretName, err := webhooksecret.ParseWebhookSecretEntityID(event.EntityID)
	if err != nil {
		h.logger.Error("Failed to parse webhook secret event entity ID",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID),
			slog.Any("error", err))
		return
	}

	h.logger.Info("Processing webhook secret upsert event",
		slog.String("action", event.Action),
		slog.String("artifact_uuid", artifactUUID),
		slog.String("secret_uuid", secretUUID),
		slog.String("secret_name", secretName),
		slog.String("event_id", event.EventID))

	if h.webhookSecretStore == nil {
		h.logger.Warn("Webhook secret store not available, skipping upsert event",
			slog.String("secret_uuid", secretUUID))
		return
	}

	if h.providerManager == nil {
		h.logger.Warn("Encryption provider manager not available, skipping webhook secret upsert event",
			slog.String("secret_uuid", secretUUID))
		return
	}

	ws, err := h.db.GetWebhookSecretByUUID(secretUUID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			h.logger.Warn("Webhook secret not found in database for upsert event",
				slog.String("secret_uuid", secretUUID),
				slog.String("event_id", event.EventID))
			return
		}
		h.logger.Error("Failed to fetch webhook secret from database",
			slog.String("secret_uuid", secretUUID),
			slog.Any("error", err))
		return
	}

	payload, err := encryption.UnmarshalPayload(string(ws.Ciphertext))
	if err != nil {
		h.logger.Error("Failed to unmarshal webhook secret ciphertext",
			slog.String("secret_uuid", secretUUID),
			slog.Any("error", err))
		return
	}

	plaintext, err := h.providerManager.Decrypt(payload)
	if err != nil {
		h.logger.Error("Failed to decrypt webhook secret",
			slog.String("secret_uuid", secretUUID),
			slog.Any("error", err))
		return
	}

	if err := h.webhookSecretStore.Store(ws.ArtifactUUID, ws.Name, string(plaintext)); err != nil {
		h.logger.Error("Failed to store webhook secret in memory store",
			slog.String("secret_uuid", secretUUID),
			slog.String("artifact_uuid", ws.ArtifactUUID),
			slog.Any("error", err))
		return
	}

	if h.webhookSecretSnapshotManager != nil {
		if err := h.webhookSecretSnapshotManager.RefreshSnapshot(); err != nil {
			h.logger.Error("Failed to refresh webhook secret xDS snapshot after upsert",
				slog.String("artifact_uuid", ws.ArtifactUUID),
				slog.Any("error", err))
		}
	}

	h.logger.Info("Successfully processed webhook secret upsert event",
		slog.String("action", event.Action),
		slog.String("artifact_uuid", ws.ArtifactUUID),
		slog.String("secret_name", ws.Name),
		slog.String("event_id", event.EventID))
}

// handleWebhookSecretDelete handles webhook secret delete events.
// The entity ID carries artifactUUID, secretUUID, and secretName to avoid a DB round-trip.
func (h *WebhookSecretHandler) handleWebhookSecretDelete(event eventhub.Event) {
	artifactUUID, secretUUID, secretName, err := webhooksecret.ParseWebhookSecretEntityID(event.EntityID)
	if err != nil {
		h.logger.Error("Failed to parse webhook secret delete event entity ID",
			slog.String("entity_id", event.EntityID),
			slog.Any("error", err))
		return
	}

	h.logger.Info("Processing webhook secret delete event",
		slog.String("artifact_uuid", artifactUUID),
		slog.String("secret_uuid", secretUUID),
		slog.String("secret_name", secretName),
		slog.String("event_id", event.EventID))

	if h.webhookSecretStore == nil {
		h.logger.Warn("Webhook secret store not available, skipping delete event",
			slog.String("secret_uuid", secretUUID))
		return
	}

	if err := h.webhookSecretStore.Remove(artifactUUID, secretName); err != nil && err != webhooksecret.ErrNotFound {
		h.logger.Error("Failed to remove webhook secret from memory store",
			slog.String("artifact_uuid", artifactUUID),
			slog.String("secret_name", secretName),
			slog.Any("error", err))
		return
	}

	if h.webhookSecretSnapshotManager != nil {
		if err := h.webhookSecretSnapshotManager.RefreshSnapshot(); err != nil {
			h.logger.Error("Failed to refresh webhook secret xDS snapshot after delete",
				slog.String("artifact_uuid", artifactUUID),
				slog.Any("error", err))
		}
	}

	h.logger.Info("Successfully processed webhook secret delete event",
		slog.String("artifact_uuid", artifactUUID),
		slog.String("secret_name", secretName),
		slog.String("event_id", event.EventID))
}

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

package eventgateway

import (
	"context"
	"log/slog"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/common/webhooksecret"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/webhooksecretxds"
)

// WebhookSecretProcessor handles EventTypeWebhookSecret events by syncing the
// in-memory webhook secret store and refreshing the xDS snapshot.
// It implements controllerext.ExtraEventProcessor.
type WebhookSecretProcessor struct {
	db                  storage.Storage
	store               *webhooksecret.WebhookSecretStore
	snapshotManager     *webhooksecretxds.SnapshotManager
	encryptionManager   *encryption.ProviderManager
	logger              *slog.Logger
}

// NewWebhookSecretProcessor creates a WebhookSecretProcessor.
func NewWebhookSecretProcessor(
	db storage.Storage,
	store *webhooksecret.WebhookSecretStore,
	snapshotManager *webhooksecretxds.SnapshotManager,
	encryptionManager *encryption.ProviderManager,
	logger *slog.Logger,
) *WebhookSecretProcessor {
	return &WebhookSecretProcessor{
		db:                db,
		store:             store,
		snapshotManager:   snapshotManager,
		encryptionManager: encryptionManager,
		logger:            logger,
	}
}

// HandlesEventType reports whether this processor handles the given event type.
func (p *WebhookSecretProcessor) HandlesEventType(t eventhub.EventType) bool {
	return t == eventhub.EventTypeWebhookSecret
}

// Process dispatches the event to the appropriate handler.
func (p *WebhookSecretProcessor) Process(_ context.Context, event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE":
		p.handleUpsert(event)
	case "DELETE":
		p.handleDelete(event)
	default:
		p.logger.Warn("Unknown webhook secret event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
	}
}

func (p *WebhookSecretProcessor) handleUpsert(event eventhub.Event) {
	artifactUUID, secretUUID, _, err := webhooksecret.ParseWebhookSecretEntityID(event.EntityID)
	if err != nil {
		p.logger.Error("Failed to parse webhook secret event entity ID",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID),
			slog.Any("error", err))
		return
	}

	if p.store == nil || p.encryptionManager == nil {
		p.logger.Warn("Webhook secret store or encryption manager not available, skipping upsert",
			slog.String("secret_uuid", secretUUID))
		return
	}

	ws, err := p.db.GetWebhookSecretByUUID(secretUUID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			p.logger.Warn("Webhook secret not found in database for upsert event",
				slog.String("secret_uuid", secretUUID),
				slog.String("event_id", event.EventID))
			return
		}
		p.logger.Error("Failed to fetch webhook secret from database",
			slog.String("secret_uuid", secretUUID),
			slog.Any("error", err))
		return
	}

	payload, err := encryption.UnmarshalPayload(string(ws.Ciphertext))
	if err != nil {
		p.logger.Error("Failed to unmarshal webhook secret ciphertext",
			slog.String("secret_uuid", secretUUID),
			slog.Any("error", err))
		return
	}

	plaintext, err := p.encryptionManager.Decrypt(payload)
	if err != nil {
		p.logger.Error("Failed to decrypt webhook secret",
			slog.String("secret_uuid", secretUUID),
			slog.Any("error", err))
		return
	}

	if err := p.store.Store(ws.ArtifactUUID, ws.Name, string(plaintext)); err != nil {
		p.logger.Error("Failed to store webhook secret in memory store",
			slog.String("secret_uuid", secretUUID),
			slog.String("artifact_uuid", ws.ArtifactUUID),
			slog.Any("error", err))
		return
	}

	if p.snapshotManager != nil {
		if err := p.snapshotManager.RefreshSnapshot(); err != nil {
			p.logger.Error("Failed to refresh webhook secret xDS snapshot after upsert",
				slog.String("artifact_uuid", ws.ArtifactUUID),
				slog.Any("error", err))
		}
	}

	p.logger.Info("Successfully processed webhook secret upsert event",
		slog.String("action", event.Action),
		slog.String("artifact_uuid", artifactUUID),
		slog.String("secret_name", ws.Name),
		slog.String("event_id", event.EventID))
}

func (p *WebhookSecretProcessor) handleDelete(event eventhub.Event) {
	artifactUUID, secretUUID, secretName, err := webhooksecret.ParseWebhookSecretEntityID(event.EntityID)
	if err != nil {
		p.logger.Error("Failed to parse webhook secret delete event entity ID",
			slog.String("entity_id", event.EntityID),
			slog.Any("error", err))
		return
	}

	if p.store == nil {
		p.logger.Warn("Webhook secret store not available, skipping delete",
			slog.String("secret_uuid", secretUUID))
		return
	}

	if err := p.store.Remove(artifactUUID, secretName); err != nil && err != webhooksecret.ErrNotFound {
		p.logger.Error("Failed to remove webhook secret from memory store",
			slog.String("artifact_uuid", artifactUUID),
			slog.String("secret_name", secretName),
			slog.Any("error", err))
		return
	}

	if p.snapshotManager != nil {
		if err := p.snapshotManager.RefreshSnapshot(); err != nil {
			p.logger.Error("Failed to refresh webhook secret xDS snapshot after delete",
				slog.String("artifact_uuid", artifactUUID),
				slog.Any("error", err))
		}
	}

	p.logger.Info("Successfully processed webhook secret delete event",
		slog.String("artifact_uuid", artifactUUID),
		slog.String("secret_name", secretName),
		slog.String("event_id", event.EventID))
}

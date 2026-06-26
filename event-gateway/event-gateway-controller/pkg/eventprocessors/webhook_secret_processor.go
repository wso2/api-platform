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

// Package eventprocessors holds event-hub event processors for the event-gateway extension.
package eventprocessors

import (
	"log/slog"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/common/webhooksecret"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	gwstorage "github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/webhooksecretxds"
	evstorage "github.com/wso2/api-platform/event-gateway/event-gateway-controller/pkg/storage"
)

// WebhookSecretProcessor handles EventHub events for webhook secret lifecycle changes.
// It keeps the in-memory WebhookSecretStore and the xDS snapshot in sync with the DB.
type WebhookSecretProcessor struct {
	db              evstorage.EventStorage
	store           *webhooksecret.WebhookSecretStore
	snapshotManager *webhooksecretxds.SnapshotManager
	providerManager *encryption.ProviderManager
	logger          *slog.Logger
}

// NewWebhookSecretProcessor creates a new processor.
func NewWebhookSecretProcessor(
	db evstorage.EventStorage,
	store *webhooksecret.WebhookSecretStore,
	snapshotManager *webhooksecretxds.SnapshotManager,
	providerManager *encryption.ProviderManager,
	logger *slog.Logger,
) *WebhookSecretProcessor {
	return &WebhookSecretProcessor{
		db:              db,
		store:           store,
		snapshotManager: snapshotManager,
		providerManager: providerManager,
		logger:          logger,
	}
}

// CanHandle reports whether this processor handles the given event type.
func (p *WebhookSecretProcessor) CanHandle(eventType eventhub.EventType) bool {
	return eventType == eventhub.EventTypeWebhookSecret
}

// Process dispatches a webhook secret event by action.
func (p *WebhookSecretProcessor) Process(event eventhub.Event) {
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
	artifactUUID, secretUUID, secretName, err := webhooksecret.ParseWebhookSecretEntityID(event.EntityID)
	if err != nil {
		p.logger.Error("Failed to parse webhook secret event entity ID",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID),
			slog.Any("error", err))
		return
	}

	p.logger.Info("Processing webhook secret upsert event",
		slog.String("action", event.Action),
		slog.String("artifact_uuid", artifactUUID),
		slog.String("secret_uuid", secretUUID),
		slog.String("secret_name", secretName),
		slog.String("event_id", event.EventID))

	ws, err := p.db.GetWebhookSecretByUUID(secretUUID)
	if err != nil {
		if gwstorage.IsNotFoundError(err) {
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

	if p.providerManager == nil {
		p.logger.Warn("No encryption provider configured; skipping webhook secret upsert",
			slog.String("secret_uuid", secretUUID))
		return
	}

	payload, err := encryption.UnmarshalPayload(string(ws.Ciphertext))
	if err != nil {
		p.logger.Error("Failed to unmarshal webhook secret ciphertext",
			slog.String("secret_uuid", secretUUID),
			slog.Any("error", err))
		return
	}

	plaintext, err := p.providerManager.Decrypt(payload)
	if err != nil {
		p.logger.Error("Failed to decrypt webhook secret",
			slog.String("secret_uuid", secretUUID),
			slog.Any("error", err))
		return
	}

	if p.store != nil {
		if err := p.store.Store(ws.ArtifactUUID, ws.Name, string(plaintext)); err != nil {
			p.logger.Error("Failed to store webhook secret in memory store",
				slog.String("secret_uuid", secretUUID),
				slog.String("artifact_uuid", ws.ArtifactUUID),
				slog.Any("error", err))
			return
		}
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
		slog.String("artifact_uuid", ws.ArtifactUUID),
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

	p.logger.Info("Processing webhook secret delete event",
		slog.String("artifact_uuid", artifactUUID),
		slog.String("secret_uuid", secretUUID),
		slog.String("secret_name", secretName),
		slog.String("event_id", event.EventID))

	if p.store != nil {
		if err := p.store.Remove(artifactUUID, secretName); err != nil && err != webhooksecret.ErrNotFound {
			p.logger.Error("Failed to remove webhook secret from memory store",
				slog.String("artifact_uuid", artifactUUID),
				slog.String("secret_name", secretName),
				slog.Any("error", err))
			return
		}
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

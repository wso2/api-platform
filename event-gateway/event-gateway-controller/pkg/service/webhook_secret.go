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

// Package service holds event-gateway-specific business logic services.
package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/common/webhooksecret"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	gwstorage "github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/webhooksecretxds"
	evstorage "github.com/wso2/api-platform/event-gateway/event-gateway-controller/pkg/storage"
)

const (
	webhookSecretPrefix = "whsec_"
	webhookSecretLen    = 32
)

// WebhookSecretService manages per-API HMAC secrets for the websub-hmac-auth policy.
type WebhookSecretService struct {
	db              evstorage.EventStorage
	providerManager *encryption.ProviderManager
	store           *webhooksecret.WebhookSecretStore
	snapshotManager *webhooksecretxds.SnapshotManager
	eventHub        eventhub.EventHub
	gatewayID       string
	logger          *slog.Logger
}

// NewWebhookSecretService creates a new WebhookSecretService.
func NewWebhookSecretService(
	db evstorage.EventStorage,
	store *webhooksecret.WebhookSecretStore,
	snapshotManager *webhooksecretxds.SnapshotManager,
	providerManager *encryption.ProviderManager,
	eventHub eventhub.EventHub,
	gatewayID string,
	logger *slog.Logger,
) *WebhookSecretService {
	if db == nil {
		panic("WebhookSecretService requires non-nil storage")
	}
	if eventHub == nil {
		panic("WebhookSecretService requires non-nil EventHub")
	}
	trimmedID := strings.TrimSpace(gatewayID)
	if trimmedID == "" {
		panic("WebhookSecretService requires non-empty gateway ID")
	}
	if providerManager == nil {
		logger.Warn("WebhookSecretService: no encryption provider configured; webhook secret operations will fail")
	}
	return &WebhookSecretService{
		db:              db,
		providerManager: providerManager,
		store:           store,
		snapshotManager: snapshotManager,
		eventHub:        eventHub,
		gatewayID:       trimmedID,
		logger:          logger,
	}
}

// Generate creates a new HMAC secret for the given artifact, encrypts it, persists
// it to the database, updates the in-memory store, and publishes a CREATE event.
// The plaintext is returned to the caller exactly once and never stored unencrypted.
func (s *WebhookSecretService) Generate(artifactUUID, displayName, correlationID string) (*models.WebhookSecret, string, error) {
	plaintext, err := generateWebhookSecretValue()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate secret value: %w", err)
	}

	name := slugify(displayName)
	if name == "" {
		suffix := correlationID
		if len(correlationID) >= 8 {
			suffix = correlationID[:8]
		}
		name = "secret-" + suffix
	}

	ciphertext, err := s.encrypt(plaintext)
	if err != nil {
		return nil, "", err
	}

	ws := &models.WebhookSecret{
		UUID:         uuid.NewString(),
		GatewayID:    s.gatewayID,
		ArtifactUUID: artifactUUID,
		Name:         name,
		DisplayName:  displayName,
		Ciphertext:   ciphertext,
		Status:       "active",
	}

	if err := s.db.SaveWebhookSecret(ws); err != nil {
		if gwstorage.IsConflictError(err) {
			return nil, "", fmt.Errorf("%w: a secret named %q already exists for this API", gwstorage.ErrConflict, name)
		}
		return nil, "", fmt.Errorf("failed to persist webhook secret: %w", err)
	}

	if s.store != nil {
		if err := s.store.Store(artifactUUID, ws.Name, plaintext); err != nil {
			s.logger.Warn("Webhook secret persisted but in-memory store update failed",
				slog.String("artifact_uuid", artifactUUID),
				slog.String("secret_name", ws.Name),
				slog.Any("error", err))
		}
	}

	if s.snapshotManager != nil {
		if err := s.snapshotManager.RefreshSnapshot(); err != nil {
			s.logger.Warn("Failed to refresh webhook secret xDS snapshot after generate",
				slog.String("artifact_uuid", artifactUUID),
				slog.Any("error", err))
		}
	}

	if err := s.publishWebhookSecretEvent("CREATE", artifactUUID, ws.UUID, ws.Name, correlationID); err != nil {
		return nil, "", fmt.Errorf("failed to publish webhook secret create event: %w", err)
	}

	return ws, plaintext, nil
}

// List returns all active webhook secrets for an artifact without plaintext values.
func (s *WebhookSecretService) List(artifactUUID string) ([]*models.WebhookSecret, error) {
	return s.db.GetWebhookSecretsByArtifact(artifactUUID)
}

// Regenerate replaces the value of an existing secret, returning the new plaintext once.
func (s *WebhookSecretService) Regenerate(artifactUUID, name, correlationID string) (*models.WebhookSecret, string, error) {
	ws, err := s.db.GetWebhookSecretByArtifactAndName(artifactUUID, name)
	if err != nil {
		if gwstorage.IsNotFoundError(err) {
			return nil, "", gwstorage.ErrNotFound
		}
		return nil, "", fmt.Errorf("failed to fetch webhook secret: %w", err)
	}

	plaintext, err := generateWebhookSecretValue()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate new secret value: %w", err)
	}

	ciphertext, err := s.encrypt(plaintext)
	if err != nil {
		return nil, "", err
	}

	ws.Ciphertext = ciphertext
	ws.UpdatedAt = time.Now().UTC()

	if err := s.db.UpdateWebhookSecret(ws); err != nil {
		return nil, "", fmt.Errorf("failed to update webhook secret: %w", err)
	}

	if s.store != nil {
		if err := s.store.Store(artifactUUID, ws.Name, plaintext); err != nil {
			s.logger.Warn("Webhook secret regenerated but in-memory store update failed",
				slog.String("artifact_uuid", artifactUUID),
				slog.String("secret_name", ws.Name),
				slog.Any("error", err))
		}
	}

	if s.snapshotManager != nil {
		if err := s.snapshotManager.RefreshSnapshot(); err != nil {
			s.logger.Warn("Failed to refresh webhook secret xDS snapshot after regenerate",
				slog.String("artifact_uuid", artifactUUID),
				slog.Any("error", err))
		}
	}

	if err := s.publishWebhookSecretEvent("UPDATE", artifactUUID, ws.UUID, ws.Name, correlationID); err != nil {
		return nil, "", fmt.Errorf("failed to publish webhook secret update event: %w", err)
	}

	return ws, plaintext, nil
}

// Delete permanently removes a webhook secret and removes it from the in-memory store.
func (s *WebhookSecretService) Delete(artifactUUID, name, correlationID string) error {
	ws, err := s.db.GetWebhookSecretByArtifactAndName(artifactUUID, name)
	if err != nil {
		if gwstorage.IsNotFoundError(err) {
			return gwstorage.ErrNotFound
		}
		return fmt.Errorf("failed to fetch webhook secret before delete: %w", err)
	}

	if err := s.db.DeleteWebhookSecret(artifactUUID, name); err != nil {
		return fmt.Errorf("failed to delete webhook secret: %w", err)
	}

	if s.store != nil {
		if err := s.store.Remove(artifactUUID, name); err != nil && err != webhooksecret.ErrNotFound {
			s.logger.Warn("Webhook secret deleted from DB but in-memory store removal failed",
				slog.String("artifact_uuid", artifactUUID),
				slog.String("secret_name", name),
				slog.Any("error", err))
		}
	}

	if s.snapshotManager != nil {
		if err := s.snapshotManager.RefreshSnapshot(); err != nil {
			s.logger.Warn("Failed to refresh webhook secret xDS snapshot after delete",
				slog.String("artifact_uuid", artifactUUID),
				slog.Any("error", err))
		}
	}

	if err := s.publishWebhookSecretEvent("DELETE", artifactUUID, ws.UUID, ws.Name, correlationID); err != nil {
		return fmt.Errorf("failed to publish webhook secret delete event: %w", err)
	}

	return nil
}

func (s *WebhookSecretService) encrypt(plaintext string) ([]byte, error) {
	if s.providerManager == nil {
		return nil, fmt.Errorf("no encryption provider configured")
	}
	payload, err := s.providerManager.Encrypt([]byte(plaintext))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt webhook secret: %w", err)
	}
	return []byte(encryption.MarshalPayload(payload)), nil
}

func (s *WebhookSecretService) publishWebhookSecretEvent(action, artifactUUID, secretUUID, secretName, correlationID string) error {
	event := eventhub.Event{
		EventType: eventhub.EventTypeWebhookSecret,
		Action:    action,
		EntityID:  webhooksecret.BuildWebhookSecretEntityID(artifactUUID, secretUUID, secretName),
		EventID:   correlationID,
		EventData: eventhub.EmptyEventData,
	}
	if err := s.eventHub.PublishEvent(s.gatewayID, event); err != nil {
		s.logger.Error("Failed to publish webhook secret event",
			slog.String("action", action),
			slog.String("artifact_uuid", artifactUUID),
			slog.String("secret_uuid", secretUUID),
			slog.Any("error", err))
		return err
	}
	return nil
}

func generateWebhookSecretValue() (string, error) {
	b := make([]byte, webhookSecretLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return webhookSecretPrefix + hex.EncodeToString(b), nil
}

func slugify(displayName string) string {
	s := strings.ToLower(strings.TrimSpace(displayName))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ' || r == '_':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

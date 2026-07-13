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

// Package webhooksecretservice manages per-API HMAC secrets for the
// websub-hmac-auth policy. Moved out of gateway-controller (core)
// pkg/utils/webhook_secret.go — the underlying DB row model
// (models.WebhookSecret) and storage.Storage CRUD methods stay in core since
// they are generic storage-layer infrastructure that pkg/storage itself
// depends on; only this service (encryption + in-memory store sync + event
// publication, all WebSub-specific) moves.
package webhooksecretservice

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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

const (
	// webhookSecretPrefix is prepended to every generated HMAC secret value.
	// Total length: 6 + 64 = 70 characters.
	webhookSecretPrefix = "whsec_"

	// webhookSecretLen is the number of random bytes to generate (64 hex chars).
	webhookSecretLen = 32
)

// WebhookSecretService manages per-API HMAC secrets for the websub-hmac-auth policy.
type WebhookSecretService struct {
	db              storage.Storage
	providerManager *encryption.ProviderManager
	store           *webhooksecret.WebhookSecretStore
	eventHub        eventhub.EventHub
	gatewayID       string
	logger          *slog.Logger
}

// NewWebhookSecretService creates a new WebhookSecretService.
func NewWebhookSecretService(
	db storage.Storage,
	providerManager *encryption.ProviderManager,
	store *webhooksecret.WebhookSecretStore,
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
	trimmedGatewayID := strings.TrimSpace(gatewayID)
	if trimmedGatewayID == "" {
		panic("WebhookSecretService requires non-empty gateway ID")
	}
	return &WebhookSecretService{
		db:              db,
		providerManager: providerManager,
		store:           store,
		eventHub:        eventHub,
		gatewayID:       trimmedGatewayID,
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
		if storage.IsConflictError(err) {
			return nil, "", fmt.Errorf("%w: a secret named %q already exists for this API", storage.ErrConflict, name)
		}
		return nil, "", fmt.Errorf("failed to persist webhook secret: %w", err)
	}

	if err := s.store.Store(artifactUUID, ws.Name, plaintext); err != nil {
		s.logger.Warn("Webhook secret persisted but in-memory store update failed",
			slog.String("artifact_uuid", artifactUUID),
			slog.String("secret_name", ws.Name),
			slog.Any("error", err))
	}

	s.publishWebhookSecretEvent("CREATE", artifactUUID, ws.UUID, ws.Name, correlationID)

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
		if storage.IsNotFoundError(err) {
			return nil, "", storage.ErrNotFound
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

	if err := s.store.Store(artifactUUID, ws.Name, plaintext); err != nil {
		s.logger.Warn("Webhook secret regenerated but in-memory store update failed",
			slog.String("artifact_uuid", artifactUUID),
			slog.String("secret_name", ws.Name),
			slog.Any("error", err))
	}

	s.publishWebhookSecretEvent("CREATE", artifactUUID, ws.UUID, ws.Name, correlationID)

	return ws, plaintext, nil
}

// Delete permanently removes a webhook secret and removes it from the in-memory store.
func (s *WebhookSecretService) Delete(artifactUUID, name, correlationID string) error {
	ws, err := s.db.GetWebhookSecretByArtifactAndName(artifactUUID, name)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return storage.ErrNotFound
		}
		return fmt.Errorf("failed to fetch webhook secret before delete: %w", err)
	}

	if err := s.db.DeleteWebhookSecret(artifactUUID, name); err != nil {
		return fmt.Errorf("failed to delete webhook secret: %w", err)
	}

	if err := s.store.Remove(artifactUUID, name); err != nil && err != webhooksecret.ErrNotFound {
		s.logger.Warn("Webhook secret deleted from DB but in-memory store removal failed",
			slog.String("artifact_uuid", artifactUUID),
			slog.String("secret_name", name),
			slog.Any("error", err))
	}

	s.publishWebhookSecretEvent("DELETE", artifactUUID, ws.UUID, ws.Name, correlationID)

	return nil
}

// encrypt AES-256-GCM encrypts plaintext using the provider manager and returns
// the serialised ciphertext envelope.
func (s *WebhookSecretService) encrypt(plaintext string) ([]byte, error) {
	payload, err := s.providerManager.Encrypt([]byte(plaintext))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt webhook secret: %w", err)
	}
	return []byte(encryption.MarshalPayload(payload)), nil
}

// publishWebhookSecretEvent publishes a lifecycle event for a webhook secret
// to the EventHub so all replicas (and the event-gateway) can sync their stores.
func (s *WebhookSecretService) publishWebhookSecretEvent(action, artifactUUID, secretUUID, secretName, correlationID string) {
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
	}
}

// generateWebhookSecretValue generates a cryptographically secure secret value.
// Format: whsec_ + hex(32 random bytes) = 70 characters total.
func generateWebhookSecretValue() (string, error) {
	b := make([]byte, webhookSecretLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return webhookSecretPrefix + hex.EncodeToString(b), nil
}

// slugify converts a display name to a URL-safe lowercase slug, e.g.
// "My GitHub Secret" → "my-github-secret". Used as the secret's immutable Name.
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

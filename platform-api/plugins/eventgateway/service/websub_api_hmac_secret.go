/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package service

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	coreservice "github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

const (
	hmacSecretPrefix = "whsec_"
	hmacSecretLen    = 32 // random bytes → 64 hex chars; total = 70 chars
)

// WebSubAPIHmacSecretService manages platform-side HMAC secrets for WebSub APIs.
type WebSubAPIHmacSecretService struct {
	repo                 repository.WebSubAPIHmacSecretRepository
	websubRepo           repository.WebSubAPIRepository
	encryptionKey        []byte
	gatewayEventsService *coreservice.GatewayEventsService
	gatewayRepo          repository.GatewayRepository
	slogger              *slog.Logger
}

// NewWebSubAPIHmacSecretService creates a new WebSubAPIHmacSecretService.
// encryptionKeyStr must be a 32-byte key encoded as 64 hex chars or base64
// (set via DATABASE_ENCRYPTION_KEY).
func NewWebSubAPIHmacSecretService(
	repo repository.WebSubAPIHmacSecretRepository,
	websubRepo repository.WebSubAPIRepository,
	gatewayEventsService *coreservice.GatewayEventsService,
	gatewayRepo repository.GatewayRepository,
	encryptionKeyStr string,
	slogger *slog.Logger,
) (*WebSubAPIHmacSecretService, error) {
	if encryptionKeyStr == "" {
		return nil, fmt.Errorf("%w", constants.ErrHmacSecretEncryptionKeyMissing)
	}
	key, err := utils.DeriveEncryptionKey(encryptionKeyStr)
	if err != nil {
		return nil, fmt.Errorf("invalid HMAC secret encryption key: %w", err)
	}
	return &WebSubAPIHmacSecretService{
		repo:                 repo,
		websubRepo:           websubRepo,
		encryptionKey:        key,
		gatewayEventsService: gatewayEventsService,
		gatewayRepo:          gatewayRepo,
		slogger:              slogger,
	}, nil
}

// Generate creates a new HMAC secret for the given WebSub API.
// externalSecret is an optional caller-supplied value; if empty, one is auto-generated.
// Returns the metadata and the plaintext value (returned once, never stored).
func (s *WebSubAPIHmacSecretService) Generate(orgUUID, apiHandle, name, externalSecret, userID string) (*model.WebSubAPIHmacSecret, string, error) {
	api, err := s.websubRepo.GetByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to look up WebSub API: %w", err)
	}
	if api == nil {
		return nil, "", constants.ErrWebSubAPINotFound
	}

	handle := slugifyHmacSecret(name)
	if len(handle) > 40 {
		handle = handle[:40]
	}
	if handle == "" {
		handle = "secret-" + apiHandle
		if len(handle) > 40 {
			handle = handle[:40]
		}
	}

	var plaintext string
	if externalSecret != "" {
		if len(externalSecret) < 32 {
			return nil, "", constants.ErrHmacSecretInvalidValue
		}
		plaintext = externalSecret
	} else {
		plaintext, err = generateHmacSecretValue()
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate secret value: %w", err)
		}
	}

	ciphertext, err := utils.EncryptSubscriptionToken(s.encryptionKey, plaintext)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encrypt secret: %w", err)
	}

	id, err := utils.GenerateUUID()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate UUID: %w", err)
	}

	secret := &model.WebSubAPIHmacSecret{
		UUID:            id,
		ArtifactUUID:    api.UUID,
		Handle:          handle,
		Name:            name,
		EncryptedSecret: ciphertext,
		Status:          "active",
		CreatedBy:       userID,
		UpdatedBy:       userID,
	}

	if err := s.repo.Create(secret); err != nil {
		if isUniqueConstraintError(err) {
			return nil, "", constants.ErrHmacSecretAlreadyExists
		}
		return nil, "", fmt.Errorf("failed to persist HMAC secret: %w", err)
	}

	s.broadcastSecretEvent(orgUUID, api.UUID, handle, "CREATED")
	return secret, plaintext, nil
}

// List returns all HMAC secrets for a WebSub API (no plaintext values).
func (s *WebSubAPIHmacSecretService) List(orgUUID, apiHandle string) ([]*model.WebSubAPIHmacSecret, error) {
	api, err := s.websubRepo.GetByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up WebSub API: %w", err)
	}
	if api == nil {
		return nil, constants.ErrWebSubAPINotFound
	}
	return s.repo.ListByArtifact(api.UUID)
}

// Regenerate replaces the secret value for an existing named secret.
// externalSecret is an optional caller-supplied value; if empty, a new value is auto-generated.
// Returns the metadata and new plaintext (returned once, never stored).
func (s *WebSubAPIHmacSecretService) Regenerate(orgUUID, apiHandle, secretName, externalSecret, userID string) (*model.WebSubAPIHmacSecret, string, error) {
	api, err := s.websubRepo.GetByHandle(apiHandle, orgUUID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to look up WebSub API: %w", err)
	}
	if api == nil {
		return nil, "", constants.ErrWebSubAPINotFound
	}

	existing, err := s.repo.GetByArtifactAndName(api.UUID, secretName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to look up HMAC secret: %w", err)
	}
	if existing == nil {
		return nil, "", constants.ErrHmacSecretNotFound
	}

	var plaintext string
	if externalSecret != "" {
		if len(externalSecret) < 32 {
			return nil, "", constants.ErrHmacSecretInvalidValue
		}
		plaintext = externalSecret
	} else {
		plaintext, err = generateHmacSecretValue()
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate secret value: %w", err)
		}
	}

	ciphertext, err := utils.EncryptSubscriptionToken(s.encryptionKey, plaintext)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encrypt secret: %w", err)
	}

	existing.EncryptedSecret = ciphertext
	existing.UpdatedBy = userID
	if err := s.repo.Update(existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", constants.ErrHmacSecretNotFound
		}
		return nil, "", fmt.Errorf("failed to update HMAC secret: %w", err)
	}

	s.broadcastSecretEvent(orgUUID, api.UUID, secretName, "UPDATED")
	return existing, plaintext, nil
}

// Delete permanently removes a named HMAC secret.
func (s *WebSubAPIHmacSecretService) Delete(orgUUID, apiHandle, secretName string) error {
	api, err := s.websubRepo.GetByHandle(apiHandle, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to look up WebSub API: %w", err)
	}
	if api == nil {
		return constants.ErrWebSubAPINotFound
	}

	existing, err := s.repo.GetByArtifactAndName(api.UUID, secretName)
	if err != nil {
		return fmt.Errorf("failed to look up HMAC secret: %w", err)
	}
	if existing == nil {
		return constants.ErrHmacSecretNotFound
	}

	if err := s.repo.Delete(api.UUID, secretName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrHmacSecretNotFound
		}
		return fmt.Errorf("failed to delete HMAC secret: %w", err)
	}

	s.broadcastSecretEvent(orgUUID, api.UUID, secretName, "DELETED")
	return nil
}

// DecryptSecret returns the plaintext for a given HMAC secret (used by the internal gateway endpoint).
func (s *WebSubAPIHmacSecretService) DecryptSecret(secret *model.WebSubAPIHmacSecret) (string, error) {
	return utils.DecryptSubscriptionToken(s.encryptionKey, secret.EncryptedSecret)
}

// ListByArtifactUUID returns all HMAC secrets for an artifact UUID (used by internal gateway endpoint).
func (s *WebSubAPIHmacSecretService) ListByArtifactUUID(artifactUUID string) ([]*model.WebSubAPIHmacSecret, error) {
	return s.repo.ListByArtifact(artifactUUID)
}

// broadcastSecretEvent sends a HMAC secret change event to all gateways in the org.
func (s *WebSubAPIHmacSecretService) broadcastSecretEvent(orgUUID, artifactUUID, secretName, action string) {
	if s.gatewayEventsService == nil || s.gatewayRepo == nil {
		return
	}
	gateways, err := s.gatewayRepo.GetByOrganizationID(orgUUID)
	if err != nil {
		s.slogger.Warn("Failed to list gateways for HMAC secret event broadcast",
			slog.String("artifactUUID", artifactUUID), slog.Any("error", err))
		return
	}
	event := &model.WebSubAPIHmacSecretEvent{
		ArtifactUUID: artifactUUID,
		SecretName:   secretName,
	}
	for _, gw := range gateways {
		if err := s.gatewayEventsService.BroadcastWebSubAPIHmacSecretEvent(gw.ID, action, event); err != nil {
			s.slogger.Warn("Failed to broadcast HMAC secret event",
				slog.String("gatewayID", gw.ID), slog.String("action", action), slog.Any("error", err))
		}
	}
}

// generateHmacSecretValue generates a cryptographically secure HMAC secret.
// Format: whsec_ + hex(32 random bytes) = 70 characters.
func generateHmacSecretValue() (string, error) {
	b := make([]byte, hmacSecretLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hmacSecretPrefix + hex.EncodeToString(b), nil
}

// isUniqueConstraintError reports whether err is a unique-constraint violation from
// SQLite ("UNIQUE constraint failed") or PostgreSQL ("duplicate key value").
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value")
}

// slugifyHmacSecret converts a name to a URL-safe handle, e.g. "My GitHub Secret" → "my-github-secret".
func slugifyHmacSecret(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
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

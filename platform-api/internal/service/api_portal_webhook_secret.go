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

// Per-portal webhook signing secret service. Generates, rotates, and reads the
// signing secret keyed on portal uuid. The plaintext leaves the service only to
// the trusted in-process plugin or the OSS secret-generation endpoint; it is
// never persisted. Encrypted bytes go through the shared vault helper used for
// api_portals.internal_auth_key.

package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

// APIPortalWebhookSecretService owns the per-portal webhook signing secret.
// EnsureWebhookSecret is the "give me a secret, generate if absent" call the
// plugin uses on Seed; RotateWebhookSecret is the explicit rotation path.
type APIPortalWebhookSecretService struct {
	portalRepo   repository.APIPortalRepository
	secretRepo   repository.APIPortalWebhookSecretRepository
	vault        vault.SecretVault
	grace        time.Duration
	keyRegistry  *PortalWebhookKeyRegistry
	slogger      *slog.Logger
}

// NewAPIPortalWebhookSecretService constructs the service. keyRegistry may be
// nil in tests that do not exercise the receiver; EnsureWebhookSecret and
// RotateWebhookSecret still work.
func NewAPIPortalWebhookSecretService(
	portalRepo repository.APIPortalRepository,
	secretRepo repository.APIPortalWebhookSecretRepository,
	secretVault vault.SecretVault,
	grace time.Duration,
	keyRegistry *PortalWebhookKeyRegistry,
	slogger *slog.Logger,
) *APIPortalWebhookSecretService {
	return &APIPortalWebhookSecretService{
		portalRepo:  portalRepo,
		secretRepo:  secretRepo,
		vault:       secretVault,
		grace:       grace,
		keyRegistry: keyRegistry,
		slogger:     slogger,
	}
}

// webhookSecretBytes is 32 bytes of entropy, matching the SharedKey format and
// `openssl rand -hex 32`. The 64-character hex encoding is what the API Portal
// subscriber row stores and what the HMAC signer feeds into crypto/hmac.
const webhookSecretBytes = 32

// generateWebhookSecret returns a fresh 64-character hex-encoded secret.
func generateWebhookSecret() (string, error) {
	buf := make([]byte, webhookSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate webhook secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// EnsureWebhookSecret returns the portal's current webhook signing secret,
// generating one on first call. Idempotent: a subsequent call returns the same
// value until RotateWebhookSecret runs. portalUUID is returned so callers that
// have only the (handle, orgID) pair can build the receiver URL in one step.
func (s *APIPortalWebhookSecretService) EnsureWebhookSecret(ctx context.Context, handle, orgID string) (secret, portalUUID string, err error) {
	portal, err := s.portalRepo.GetByHandleAndOrgID(handle, orgID)
	if err != nil {
		return "", "", err
	}
	if portal == nil {
		return "", "", apperror.APIPortalNotFound.New()
	}

	existing, err := s.secretRepo.GetByPortalUUID(portal.ID)
	if err != nil {
		return "", "", fmt.Errorf("ensure webhook secret: read: %w", err)
	}
	if existing != nil {
		plaintext, err := s.vault.Decrypt(ctx, existing.SecretEnc)
		if err != nil {
			return "", "", fmt.Errorf("ensure webhook secret: decrypt: %w", err)
		}
		return plaintext, portal.ID, nil
	}

	plaintext, err := generateWebhookSecret()
	if err != nil {
		return "", "", err
	}
	ciphertext, err := s.vault.Encrypt(ctx, plaintext)
	if err != nil {
		return "", "", fmt.Errorf("ensure webhook secret: encrypt: %w", err)
	}
	if err := s.secretRepo.Insert(portal.ID, ciphertext); err != nil {
		return "", "", fmt.Errorf("ensure webhook secret: insert: %w", err)
	}
	return plaintext, portal.ID, nil
}

// RotateWebhookSecret generates a new signing secret and keeps the previous one
// valid for the configured grace window so a delivery signed between this call
// and the subscriber-row update is still accepted. Returns the plaintext of the
// new secret.
func (s *APIPortalWebhookSecretService) RotateWebhookSecret(ctx context.Context, handle, orgID string) (secret, portalUUID string, err error) {
	portal, err := s.portalRepo.GetByHandleAndOrgID(handle, orgID)
	if err != nil {
		return "", "", err
	}
	if portal == nil {
		return "", "", apperror.APIPortalNotFound.New()
	}

	// If no row exists yet, Rotate has nothing to move into "previous" and would
	// fail — fall back to a plain Insert, matching Ensure's first-time path.
	existing, err := s.secretRepo.GetByPortalUUID(portal.ID)
	if err != nil {
		return "", "", fmt.Errorf("rotate webhook secret: read: %w", err)
	}

	plaintext, err := generateWebhookSecret()
	if err != nil {
		return "", "", err
	}
	ciphertext, err := s.vault.Encrypt(ctx, plaintext)
	if err != nil {
		return "", "", fmt.Errorf("rotate webhook secret: encrypt: %w", err)
	}

	if existing == nil {
		if err := s.secretRepo.Insert(portal.ID, ciphertext); err != nil {
			return "", "", fmt.Errorf("rotate webhook secret: insert: %w", err)
		}
	} else {
		previousValidUntil := time.Now().UTC().Add(s.grace)
		if err := s.secretRepo.Rotate(portal.ID, ciphertext, previousValidUntil); err != nil {
			return "", "", fmt.Errorf("rotate webhook secret: rotate: %w", err)
		}
	}

	s.invalidateKeyRegistry(portal.ID)
	return plaintext, portal.ID, nil
}

// invalidateKeyRegistry drops the cached verifier/decryptor so the next webhook
// request rebuilds from the just-rotated secret. No-op when keyRegistry is nil.
func (s *APIPortalWebhookSecretService) invalidateKeyRegistry(portalUUID string) {
	if s.keyRegistry == nil {
		return
	}
	s.keyRegistry.Invalidate(portalUUID)
}

// HasSecret reports whether a per-portal webhook signing secret row exists for
// portalUUID. Does not decrypt — it is a cheap PK existence check, used by the
// ApiPortalResponse builder to populate the webhookSecretConfigured flag on
// single-portal read responses.
func (s *APIPortalWebhookSecretService) HasSecret(portalUUID string) (bool, error) {
	row, err := s.secretRepo.GetByPortalUUID(portalUUID)
	if err != nil {
		return false, err
	}
	return row != nil, nil
}

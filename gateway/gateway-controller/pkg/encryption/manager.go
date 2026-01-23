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

package encryption

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// EncryptionProvider defines the interface for encryption implementations
type EncryptionProvider interface {
	// Name returns the provider identifier (e.g., "aesgcm", "vault")
	Name() string

	// Encrypt transforms plaintext into encrypted payload using the active key
	Encrypt(plaintext []byte) (*EncryptedPayload, error)

	// Decrypt transforms encrypted payload back to plaintext
	Decrypt(payload *EncryptedPayload) ([]byte, error)

	// HealthCheck validates provider initialization and key availability
	HealthCheck() error
}

// EncryptedPayload represents encrypted data with metadata
type EncryptedPayload struct {
	Provider   string // Provider type identifier (e.g., "aesgcm")
	KeyVersion string // Key name/version (e.g., "key-v2")
	Ciphertext []byte // Encrypted bytes (nonce || ciphertext || tag for AES-GCM)
}

// ProviderManager orchestrates the encryption provider chain
type ProviderManager struct {
	providers []EncryptionProvider
	storage   storage.Storage
	logger    *slog.Logger
}

// NewProviderManager creates a new provider manager with the given providers
func NewProviderManager(providers []EncryptionProvider, storage storage.Storage, logger *slog.Logger) (*ProviderManager, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("at least one encryption provider is required")
	}

	// Validate all providers
	for _, provider := range providers {
		if err := provider.HealthCheck(); err != nil {
			return nil, fmt.Errorf("provider %s failed health check: %w", provider.Name(), err)
		}
	}

	logger.Info("Initialized encryption provider chain",
		slog.Int("provider_count", len(providers)),
		slog.String("primary_provider", providers[0].Name()),
	)

	return &ProviderManager{
		providers: providers,
		storage:   storage,
		logger:    logger,
	}, nil
}

// Encrypt encrypts plaintext using the primary provider (first in chain)
func (m *ProviderManager) Encrypt(plaintext []byte) (*EncryptedPayload, error) {
	primaryProvider := m.providers[0]

	m.logger.Debug("Encrypting with primary provider",
		slog.String("provider", primaryProvider.Name()),
		slog.Int("plaintext_size", len(plaintext)),
	)

	payload, err := primaryProvider.Encrypt(plaintext)
	if err != nil {
		m.logger.Error("Encryption failed",
			slog.String("provider", primaryProvider.Name()),
			slog.Any("error", err),
		)
		return nil, &ErrEncryptionFailed{
			ProviderName: primaryProvider.Name(),
			Cause:        err,
		}
	}

	m.logger.Debug("Encryption successful",
		slog.String("provider", payload.Provider),
		slog.String("key_version", payload.KeyVersion),
	)

	return payload, nil
}

// Decrypt decrypts the payload using the provider chain
// It tries to match the provider by name from the payload metadata
func (m *ProviderManager) Decrypt(payload *EncryptedPayload) ([]byte, error) {
	if payload == nil {
		return nil, fmt.Errorf("encrypted payload is nil")
	}
	m.logger.Debug("Decrypting payload",
		slog.String("provider", payload.Provider),
		slog.String("key_version", payload.KeyVersion),
	)

	// Find the provider that can decrypt this payload
	// If the provider name matches, use it to decrypt
	// If no match, return error indicating no provider found
	// If decryption fails, return error indicating curruption or invalid data
	for _, provider := range m.providers {
		if provider.Name() == payload.Provider {
			plaintext, err := provider.Decrypt(payload)
			if err != nil {
				m.logger.Error("Decryption failed",
					slog.String("provider", provider.Name()),
					slog.String("key_version", payload.KeyVersion),
					slog.Any("error", err),
				)
				return nil, &ErrDecryptionFailed{
					ProviderName: provider.Name(),
					Cause:        err,
				}
			}

			m.logger.Debug("Decryption successful",
				slog.String("provider", provider.Name()),
				slog.Int("plaintext_size", len(plaintext)),
			)

			return plaintext, nil
		}
	}

	// No provider found that can decrypt this payload
	m.logger.Error("No provider found for decryption",
		slog.String("requested_provider", payload.Provider),
		slog.String("key_version", payload.KeyVersion),
	)

	return nil, &ErrProviderNotFound{
		ProviderName: payload.Provider,
	}
}

// HealthCheck validates all providers in the chain
func (m *ProviderManager) HealthCheck() error {
	for _, provider := range m.providers {
		if err := provider.HealthCheck(); err != nil {
			return fmt.Errorf("provider %s health check failed: %w", provider.Name(), err)
		}
	}
	return nil
}

// GetPrimaryProvider returns the primary encryption provider (first in chain)
func (m *ProviderManager) GetPrimaryProvider() EncryptionProvider {
	return m.providers[0]
}

// GetProviders returns all configured providers
func (m *ProviderManager) GetProviders() []EncryptionProvider {
	return m.providers
}

// MarshalPayload converts EncryptedPayload to storage format
// Format: enc:provider:v1:key-version:base64-ciphertext
func MarshalPayload(payload *EncryptedPayload) string {
	encoded := base64.StdEncoding.EncodeToString(payload.Ciphertext)
	return fmt.Sprintf("enc:%s:v1:%s:%s", payload.Provider, payload.KeyVersion, encoded)
}

// UnmarshalPayload converts storage format back to EncryptedPayload
// Expects format: enc:provider:v1:key-version:base64-ciphertext
func UnmarshalPayload(stored string) (*EncryptedPayload, error) {
	parts := strings.SplitN(stored, ":", 5)
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid payload format: expected 5 parts, got %d", len(parts))
	}

	if parts[0] != "enc" {
		return nil, fmt.Errorf("invalid payload prefix: expected 'enc', got '%s'", parts[0])
	}

	if parts[2] != "v1" {
		return nil, fmt.Errorf("unsupported payload version: %s", parts[2])
	}

	ciphertext, err := base64.StdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	return &EncryptedPayload{
		Provider:   parts[1],
		KeyVersion: parts[3],
		Ciphertext: ciphertext,
	}, nil

}

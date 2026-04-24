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

package secrets

import (
	"fmt"
	"log/slog"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

const (
	// MaxSecretSize is the maximum allowed size for a secret value (10KB)
	MaxSecretSize = 10 * 1024
)

// SecretParams carries input to encrypt/save a secret
type SecretParams struct {
	Data          []byte       // Raw configuration data (YAML/JSON)
	ContentType   string       // Content type for parsing
	CorrelationID string       // Correlation ID for tracking
	Logger        *slog.Logger // Logger
}

// SecretService handles business logic for secret operations
type SecretService struct {
	storage         storage.Storage
	providerManager *encryption.ProviderManager
	parser          *config.Parser
	validator       *config.SecretValidator
	logger          *slog.Logger
}

// NewSecretsService creates a new secret service
func NewSecretsService(
	storage storage.Storage,
	providerManager *encryption.ProviderManager,
	logger *slog.Logger,
) *SecretService {
	return &SecretService{
		storage:         storage,
		providerManager: providerManager,
		parser:          config.NewParser(),
		validator:       config.NewSecretValidator(),
		logger:          logger,
	}
}

// CreateSecret creates a new secret with encryption
func (s *SecretService) CreateSecret(params SecretParams) (*models.Secret, error) {
	var secretConfig api.SecretConfigurationRequest
	// Parse configuration
	err := s.parser.Parse(params.Data, params.ContentType, &secretConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}
	handle := secretConfig.Metadata.Name

	// Validate configuration
	validationErrors := s.validator.Validate(&secretConfig)
	if len(validationErrors) > 0 {
		errors := make([]string, 0, len(validationErrors))
		s.logger.Warn("Configuration validation failed",
			slog.String("secret_id", handle),
			slog.String("name", secretConfig.Spec.DisplayName),
			slog.Int("num_errors", len(validationErrors)))

		for i, e := range validationErrors {
			s.logger.Warn("Validation error",
				slog.String("field", e.Field),
				slog.String("message", e.Message))
			errors = append(errors, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}

		combinedMsg := strings.Join(errors, "; ")

		return nil, fmt.Errorf("configuration validation failed with %d error(s): %s",
			len(validationErrors), combinedMsg)
	}

	// Enforce secret value size limit before any storage or encryption
	if len(secretConfig.Spec.Value) > MaxSecretSize {
		return nil, fmt.Errorf("secret value exceeds maximum allowed size of %d bytes (got %d bytes)", MaxSecretSize, len(secretConfig.Spec.Value))
	}

	// Encrypt the secret value
	payload, err := s.providerManager.Encrypt([]byte(secretConfig.Spec.Value))
	if err != nil {
		s.logger.Error("Failed to encrypt secret",
			slog.String("secret_handle", handle),
			slog.String("correlation_id", params.CorrelationID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	// Serialize the encrypted payload for storage
	ciphertext := encryption.MarshalPayload(payload)

	// Create secret model
	secret := &models.Secret{
		Handle:      handle,
		DisplayName: secretConfig.Spec.DisplayName,
		Description: secretConfig.Spec.Description,
		Value:       "", // Don't store plaintext
		Ciphertext:  []byte(ciphertext),
	}

	// Persist encrypted secret
	if err := s.storage.SaveSecret(secret); err != nil {
		s.logger.Error("Failed to save secret",
			slog.String("secret_handle", handle),
			slog.String("correlation_id", params.CorrelationID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("storage failed: %w", err)
	}

	s.logger.Info("Secret created successfully",
		slog.String("secret_handle", handle),
		slog.String("correlation_id", params.CorrelationID),
		slog.String("provider", payload.Provider),
		slog.String("key_version", payload.KeyVersion),
	)

	// Return secret with plaintext value for response
	secret.Value = secretConfig.Spec.Value
	return secret, nil
}

// GetSecrets retrieves metadata for all secrets (no sensitive data)
func (s *SecretService) GetSecrets(correlationID string) ([]models.SecretMeta, error) {
	s.logger.Info("Retrieving all secrets",
		slog.String("correlation_id", correlationID),
	)

	// Retrieve all secret metadata from storage (no ciphertext)
	secrets, err := s.storage.GetSecrets()
	if err != nil {
		s.logger.Error("Failed to retrieve secrets",
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to retrieve secrets: %w", err)
	}

	s.logger.Info("Secrets retrieved successfully",
		slog.String("correlation_id", correlationID),
		slog.Int("count", len(secrets)),
	)

	return secrets, nil
}

// Get retrieves and decrypts a secret
func (s *SecretService) Get(handle string, correlationID string) (*models.Secret, error) {
	s.logger.Info("Retrieving secret",
		slog.String("secret_handle", handle),
		slog.String("correlation_id", correlationID),
	)

	// Retrieve encrypted secret from storage
	secret, err := s.storage.GetSecret(handle)
	if err != nil {
		// Don't log details for not found errors (common case)
		if ok := storage.IsNotFoundError(err); ok {
			s.logger.Debug("Secret not found",
				slog.String("secret_handle", handle),
				slog.String("correlation_id", correlationID),
			)
		} else {
			s.logger.Error("Failed to retrieve secret",
				slog.String("secret_handle", handle),
				slog.String("correlation_id", correlationID),
				slog.Any("error", err),
			)
		}
		return nil, err
	}

	// Deserialize the encrypted payload
	payload, err := encryption.UnmarshalPayload(string(secret.Ciphertext))
	if err != nil {
		s.logger.Error("Failed to unmarshal encrypted payload",
			slog.String("secret_handle", handle),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("payload deserialization failed: %w", err)
	}

	// Decrypt the secret value
	plaintext, err := s.providerManager.Decrypt(payload)
	if err != nil {
		s.logger.Error("Failed to decrypt secret",
			slog.String("secret_handle", handle),
			slog.String("provider", payload.Provider),
			slog.String("key_version", payload.KeyVersion),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	s.logger.Info("Secret retrieved successfully",
		slog.String("secret_handle", handle),
		slog.String("correlation_id", correlationID),
		slog.String("provider", payload.Provider),
		slog.String("key_version", payload.KeyVersion),
	)

	// Set plaintext value in secret
	secret.Value = string(plaintext)
	return secret, nil
}

// Resolve retrieves the plaintext value of the secret identified by handle.
// It satisfies the templateengine/funcs.SecretResolver interface.
func (s *SecretService) Resolve(handle string) (string, error) {
	secret, err := s.Get(handle, "")
	if err != nil {
		return "", err
	}
	return secret.Value, nil
}

// UpdateSecret updates an existing secret with re-encryption using current primary key
func (s *SecretService) UpdateSecret(handle string, params SecretParams) (*models.Secret, error) {
	var secretConfig api.SecretConfigurationRequest
	// Parse configuration
	err := s.parser.Parse(params.Data, params.ContentType, &secretConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Validate configuration
	validationErrors := s.validator.Validate(&secretConfig)
	if len(validationErrors) > 0 {
		errors := make([]string, 0, len(validationErrors))
		s.logger.Warn("Configuration validation failed",
			slog.String("secret_id", handle),
			slog.String("name", secretConfig.Spec.DisplayName),
			slog.Int("num_errors", len(validationErrors)))

		for i, e := range validationErrors {
			s.logger.Warn("Validation error",
				slog.String("field", e.Field),
				slog.String("message", e.Message))
			errors = append(errors, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}

		combinedMsg := strings.Join(errors, "; ")

		return nil, fmt.Errorf("configuration validation failed with %d error(s): %s",
			len(validationErrors), combinedMsg)
	}

	// Validate that the handle in the URL matches the handle in the payload.
	// Renaming secrets is not supported to avoid complexity and accidental changes.
	if secretConfig.Metadata.Name != handle {
		return nil, fmt.Errorf("secret id in payload ('%s') does not match the URL path id ('%s'): renaming secrets is not supported", secretConfig.Metadata.Name, handle)
	}

	// Enforce secret value size limit before any storage or encryption
	if len(secretConfig.Spec.Value) > MaxSecretSize {
		return nil, fmt.Errorf("secret value exceeds maximum allowed size of %d bytes (got %d bytes)", MaxSecretSize, len(secretConfig.Spec.Value))
	}

	// Encrypt with current primary key (automatic key migration)
	payload, err := s.providerManager.Encrypt([]byte(secretConfig.Spec.Value))
	if err != nil {
		s.logger.Error("Failed to encrypt secret",
			slog.String("secret_handle", handle),
			slog.String("correlation_id", params.CorrelationID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	// Serialize the encrypted payload
	ciphertext := encryption.MarshalPayload(payload)

	// Update secret model
	secret := &models.Secret{
		Handle:      handle,
		DisplayName: secretConfig.Spec.DisplayName,
		Description: secretConfig.Spec.Description,
		Ciphertext:  []byte(ciphertext),
	}

	// Persist updated secret — single round-trip, returns model with timestamps.
	// Relies on atomic UPDATE ... WHERE handle = ? returning rows affected = 0 for "not found".
	updatedSecret, err := s.storage.UpdateSecret(secret)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("secret configuration not found: id=%s: %w", handle, err)
		}
		s.logger.Error("Failed to update secret",
			slog.String("secret_handle", handle),
			slog.String("correlation_id", params.CorrelationID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("storage update failed: %w", err)
	}

	s.logger.Info("Secret updated successfully",
		slog.String("secret_handle", handle),
		slog.String("correlation_id", params.CorrelationID),
		slog.String("provider", payload.Provider),
		slog.String("key_version", payload.KeyVersion),
	)

	// Return secret with plaintext value
	updatedSecret.Value = secretConfig.Spec.Value
	return updatedSecret, nil
}

// Delete permanently removes a secret
func (s *SecretService) Delete(id string, correlationID string) error {
	s.logger.Info("Deleting secret",
		slog.String("secret_handle", id),
		slog.String("correlation_id", correlationID),
	)

	if err := s.storage.DeleteSecret(id); err != nil {
		// Don't log details for not found errors
		if ok := storage.IsNotFoundError(err); ok {
			s.logger.Debug("Secret not found for deletion",
				slog.String("secret_handle", id),
				slog.String("correlation_id", correlationID),
			)
		} else {
			s.logger.Error("Failed to delete secret",
				slog.String("secret_handle", id),
				slog.String("correlation_id", correlationID),
				slog.Any("error", err),
			)
		}
		return err
	}

	s.logger.Info("Secret deleted successfully",
		slog.String("secret_handle", id),
		slog.String("correlation_id", correlationID),
	)

	return nil
}

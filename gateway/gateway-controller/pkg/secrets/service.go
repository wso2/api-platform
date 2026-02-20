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
	"time"

	"github.com/google/uuid"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
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
	ID            string       // Optional ID; if empty, generated
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
	var secretConfig api.SecretConfiguration
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
		params.Logger.Warn("Configuration validation failed",
			slog.String("secret_id", handle),
			slog.String("name", secretConfig.Spec.DisplayName),
			slog.Int("num_errors", len(validationErrors)))

		for i, e := range validationErrors {
			params.Logger.Warn("Validation error",
				slog.String("field", e.Field),
				slog.String("message", e.Message))
			errors = append(errors, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}

		combinedMsg := strings.Join(errors, "; ")

		return nil, fmt.Errorf("configuration validation failed with %d error(s): %s",
			len(validationErrors), combinedMsg)
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
		ID:         generateUUID(),
		Handle:     handle,
		Value:      "", // Don't store plaintext
		Provider:   payload.Provider,
		KeyVersion: payload.KeyVersion,
		Ciphertext: []byte(ciphertext),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
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

// GetSecrets retrieves all secret handles/IDs
func (s *SecretService) GetSecrets(correlationID string) ([]string, error) {
	s.logger.Info("Retrieving all secrets",
		slog.String("correlation_id", correlationID),
	)

	// Retrieve all secret handles from storage
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

// UpdateSecret updates an existing secret with re-encryption using current primary key
func (s *SecretService) UpdateSecret(handle string, params SecretParams) (*models.Secret, error) {
	var secretConfig api.SecretConfiguration
	// Parse configuration
	err := s.parser.Parse(params.Data, params.ContentType, &secretConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Validate configuration
	validationErrors := s.validator.Validate(&secretConfig)
	if len(validationErrors) > 0 {
		errors := make([]string, 0, len(validationErrors))
		params.Logger.Warn("Configuration validation failed",
			slog.String("secret_id", handle),
			slog.String("name", secretConfig.Spec.DisplayName),
			slog.Int("num_errors", len(validationErrors)))

		for i, e := range validationErrors {
			params.Logger.Warn("Validation error",
				slog.String("field", e.Field),
				slog.String("message", e.Message))
			errors = append(errors, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}

		combinedMsg := strings.Join(errors, "; ")

		return nil, fmt.Errorf("configuration validation failed with %d error(s): %s",
			len(validationErrors), combinedMsg)
	}

	// Check if secret exists
	exists, err := s.storage.SecretExists(handle)
	if err != nil {
		return nil, fmt.Errorf("failed to check secret existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("secret configuration not found: id=%s", handle)
	}

	// Check for metadata.name conflicts
	if secretConfig.Metadata.Name != handle {
		conflict, err := s.storage.SecretExists(secretConfig.Metadata.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to check conflicting secret existence: %w", err)
		}
		if conflict {
			return nil, fmt.Errorf("unable to change the secret id because a secret with the id '%s'"+
				" already exists", secretConfig.Metadata.Name)
		}
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
		Handle:     handle,
		Provider:   payload.Provider,
		KeyVersion: payload.KeyVersion,
		Ciphertext: []byte(ciphertext),
	}

	// Persist updated secret
	if err := s.storage.UpdateSecret(secret); err != nil {
		s.logger.Error("Failed to update secret",
			slog.String("secret_handle", handle),
			slog.String("correlation_id", params.CorrelationID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("storage update failed: %w", err)
	}

	// Retrieve updated secret with timestamps
	updatedSecret, err := s.storage.GetSecret(handle)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated secret: %w", err)
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

// generateUUID generates a new UUID string
func generateUUID() string {
	return uuid.New().String()
}

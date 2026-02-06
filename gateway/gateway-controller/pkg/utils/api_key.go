/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"

	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// APIKeyCreationParams contains parameters for API key creation operations.
// Handles both local key generation and external key injection.
type APIKeyCreationParams struct {
	Handle        string                    // API handle/ID
	Request       api.APIKeyCreationRequest // Request body with API key creation details
	User          *commonmodels.AuthContext // User who initiated the request
	CorrelationID string                    // Correlation ID for tracking
	Logger        *slog.Logger              // Logger instance
}

// APIKeyCreationResult contains the result of API key creation.
// Used for both locally generated keys and externally injected keys.
type APIKeyCreationResult struct {
	Response api.APIKeyCreationResponse // Response following the generated schema
	IsRetry  bool                       // Whether this was a retry due to collision
}

// APIKeyRevocationParams contains parameters for API key revocation operations
type APIKeyRevocationParams struct {
	Handle        string                    // API handle/ID
	APIKeyName    string                    // // Name of the API key to revoke
	User          *commonmodels.AuthContext // User who initiated the request
	CorrelationID string                    // Correlation ID for tracking
	Logger        *slog.Logger              // Logger instance
}

// APIKeyRevocationResult contains the result of API key revocation
type APIKeyRevocationResult struct {
	Response api.APIKeyRevocationResponse // Response following the generated schema
}

// APIKeyRegenerationParams contains parameters for API key regeneration operations
type APIKeyRegenerationParams struct {
	Handle        string                        // API handle/ID
	APIKeyName    string                        // Name of the API key to regenerate
	Request       api.APIKeyRegenerationRequest // Request body with regeneration details
	User          *commonmodels.AuthContext     // User who initiated the request
	CorrelationID string                        // Correlation ID for tracking
	Logger        *slog.Logger                  // Logger instance
}

// APIKeyRegenerationResult contains the result of API key regeneration
type APIKeyRegenerationResult struct {
	Response api.APIKeyCreationResponse // Response following the generated schema
	IsRetry  bool                       // Whether this was a retry due to collision
}

// APIKeyUpdateParams contains parameters for API key update operations
type APIKeyUpdateParams struct {
	Handle        string                    // API handle/ID
	APIKeyName    string                    // Name of the API key to update
	Request       api.APIKeyCreationRequest // Request body with update details
	User          *commonmodels.AuthContext // User who initiated the request
	CorrelationID string                    // Correlation ID for tracking
	Logger        *slog.Logger              // Logger instance
}

// APIKeyUpdateResult contains the result of API key update
type APIKeyUpdateResult struct {
	Response api.APIKeyCreationResponse // Response following the generated schema
}

// ListAPIKeyParams contains parameters for listing API keys
type ListAPIKeyParams struct {
	Handle        string                    // API handle/ID
	User          *commonmodels.AuthContext // User who initiated the request
	CorrelationID string                    // Correlation ID for tracking
	Logger        *slog.Logger              // Logger instance
}

// ListAPIKeyResult contains the result of listing API keys
type ListAPIKeyResult struct {
	Response api.APIKeyListResponse // Response following the generated schema
}

// ParsedAPIKey represents a parsed API key with its components
type ParsedAPIKey struct {
	APIKey string
	ID     string
}

// XDSManager interface for API key operations
type XDSManager interface {
	StoreAPIKey(apiId, apiName, apiVersion string, apiKey *models.APIKey, correlationID string) error
	RevokeAPIKey(apiId, apiName, apiVersion, apiKeyName, correlationID string) error
	RemoveAPIKeysByAPI(apiId, apiName, apiVersion, correlationID string) error
}

// APIKeyService provides utilities for API configuration deployment
type APIKeyService struct {
	store        *storage.ConfigStore
	db           storage.Storage
	xdsManager   XDSManager
	apiKeyConfig *config.APIKeyConfig // Configuration for API keys
}

// NewAPIKeyService creates a new API key generation service
func NewAPIKeyService(store *storage.ConfigStore, db storage.Storage, xdsManager XDSManager,
	apiKeyConfig *config.APIKeyConfig) *APIKeyService {
	return &APIKeyService{
		store:        store,
		db:           db,
		xdsManager:   xdsManager,
		apiKeyConfig: apiKeyConfig,
	}
}

const (
	// Argon2id parameters (recommended for production security)
	argon2Time    = 1         // Number of iterations
	argon2Memory  = 64 * 1024 // Memory usage in KiB (64 MiB)
	argon2Threads = 4         // Number of threads
	argon2KeyLen  = 32        // Length of derived key in bytes
	argon2SaltLen = 16        // Length of salt in bytes

	// bcrypt parameters (alternative hashing method)
	bcryptCost = 12 // Cost parameter for bcrypt (recommended: 10-15)

	// SHA-256 parameters
	sha256SaltLen = 32 // Length of salt in bytes for SHA-256
)

// CreateAPIKey handles the complete API key creation process.
// Supports both local key generation by generating a new random key and external key injection
// (accepts key from external platforms).
func (s *APIKeyService) CreateAPIKey(params APIKeyCreationParams) (*APIKeyCreationResult, error) {
	baseLogger := params.Logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}
	user := params.User

	logger := baseLogger.With(
		slog.String("handle", params.Handle),
		slog.String("correlation_id", params.CorrelationID),
		slog.String("user_id", user.UserID),
	)

	// Determine operation type for context-aware messaging
	isExternalKeyInjection := params.Request.ApiKey != nil && strings.TrimSpace(*params.Request.ApiKey) != ""
	operationType := "generate"
	if isExternalKeyInjection {
		operationType = "register"
	}

	// Validate that API exists
	config, err := s.store.GetByHandle(params.Handle)
	if err != nil {
		logger.Error("API configuration not found for API Key generation",
			slog.String("operation", operationType+"_key"),
			slog.Any("error", err))
		return nil, fmt.Errorf("API configuration handle '%s' not found", params.Handle)
	}

	// Check API key limit enforcement
	if err := s.enforceAPIKeyLimit(config.ID, user.UserID, logger); err != nil {
		logger.Warn("API key generation limit exceeded",
			slog.String("api_id", config.ID),
			slog.String("operation", operationType+"_key"),
			slog.Any("error", err))
		return nil, err
	}

	result := &APIKeyCreationResult{
		IsRetry: false,
	}

	// Create the API key from request (generate new or register external)
	// For local keys, retry once if duplicate is detected during generation
	apiKey, err := s.createAPIKeyFromRequest(params.Handle, &params.Request, user.UserID, config)
	if err != nil {
		logger.Error("Failed to generate API key",
			slog.Any("error", err),
			slog.String("handle", params.Handle),
			slog.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Save API key to database (only if persistent mode)
	if s.db != nil {
		if err := s.db.SaveAPIKey(apiKey); err != nil {
			if errors.Is(err, storage.ErrConflict) {
				// Handle collision - only retry for locally generated keys
				if isExternalKeyInjection {
					// For external keys, collision means the key already exists
					logger.Error("External API key already exists in the system",
						slog.String("operation", operationType+"_key"))
					return nil, fmt.Errorf("%w: provided API key already exists", storage.ErrConflict)
				}

				// For local keys, retry with a new generated key
				logger.Warn("API key collision detected, generating new key",
					slog.String("operation", operationType+"_key"))

				// Generate a new key
				apiKey, err = s.createAPIKeyFromRequest(params.Handle, &params.Request, user.UserID, config)
				if err != nil {
					logger.Error("Failed to generate API key after collision",
						slog.String("operation", operationType+"_key"),
						slog.Any("error", err))
					return nil, fmt.Errorf("failed to generate API key after collision: %w", err)
				}

				// Try saving again
				if err := s.db.SaveAPIKey(apiKey); err != nil {
					logger.Error("Failed to save API key after retry",
						slog.String("operation", operationType+"_key"),
						slog.Any("error", err))
					return nil, fmt.Errorf("failed to save API key after retry: %w", err)
				}

				result.IsRetry = true
			} else {
				logger.Error("Failed to save API key to database",
					slog.String("operation", operationType+"_key"),
					slog.Any("error", err))
				return nil, fmt.Errorf("failed to save API key to database: %w", err)
			}
		}
	}

	plainAPIKey := apiKey.PlainAPIKey // Store plain API key for response
	apiKey.PlainAPIKey = ""           // Clear plain API key from the struct for security

	// Store the API key in the ConfigStore (for both generated and registered keys)
	if err := s.store.StoreAPIKey(apiKey); err != nil {
		logger.Error("Failed to store API key in ConfigStore",
			slog.Any("error", err),
			slog.String("operation", operationType+"_key"))

		// Rollback database save to maintain consistency
		if s.db != nil {
			if delErr := s.db.RemoveAPIKeyAPIAndName(apiKey.APIId, apiKey.Name); delErr != nil {
				logger.Error("Failed to rollback API key from database",
					slog.Any("error", delErr),
					slog.String("correlation_id", params.CorrelationID))
			}
		}
		return nil, fmt.Errorf("failed to store API key in ConfigStore: %w", err)
	}

	apiConfig, err := config.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		logger.Error("Failed to parse API configuration data",
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to parse API configuration data: %w", err)
	}

	apiId := config.ID
	apiName := apiConfig.DisplayName
	apiVersion := apiConfig.Version
	logger.Info("Storing API key in policy engine",
		slog.String("name", apiKey.Name),
		slog.String("api_name", apiName),
		slog.String("api_version", apiVersion),
		slog.String("operation", operationType+"_key"))

	// Send the API key to the policy engine via xDS
	if s.xdsManager != nil {
		if err := s.xdsManager.StoreAPIKey(apiId, apiName, apiVersion, apiKey, params.CorrelationID); err != nil {
			logger.Error("Failed to send API key to policy engine",
				slog.String("operation", operationType+"_key"),
				slog.Any("error", err))
			return nil, fmt.Errorf("failed to send API key to policy engine: %w", err)
		}
	}

	// Build response following the generated schema
	result.Response = s.buildAPIKeyResponse(apiKey, params.Handle, plainAPIKey, isExternalKeyInjection)

	logger.Info("API key successfully created",
		slog.String("name", apiKey.Name),
		slog.String("operation", operationType+"_key"),
		slog.Bool("is_retry", result.IsRetry))

	return result, nil
}

// RevokeAPIKey handles the API key revocation process
// TODO: checks if the index created in policy engine is removed
func (s *APIKeyService) RevokeAPIKey(params APIKeyRevocationParams) (*APIKeyRevocationResult, error) {
	baseLogger := params.Logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}
	user := params.User
	apiKeyName := params.APIKeyName

	logger := baseLogger.With(
		slog.String("correlation_id", params.CorrelationID),
		slog.String("handle", params.Handle),
		slog.String("user_id", user.UserID),
	)
	result := &APIKeyRevocationResult{
		Response: api.APIKeyRevocationResponse{
			Status:  "success",
			Message: "API key revoked successfully",
		},
	}

	// Validate that API exists
	config, err := s.store.GetByHandle(params.Handle)
	if err != nil {
		logger.Warn("API configuration not found for API key revocation",
			slog.Any("error", err))
		return nil, fmt.Errorf("API configuration handle '%s' not found", params.Handle)
	}

	var apiKey *models.APIKey
	var matchedKey *models.APIKey

	existingAPIKey, err := s.store.GetAPIKeyByName(config.ID, apiKeyName)
	if err != nil {
		// If memory store fails, try database
		if s.db != nil {
			existingAPIKey, err = s.db.GetAPIKeysByAPIAndName(config.ID, apiKeyName)
			if err != nil {
				logger.Debug("Failed to get API keys for revocation",
					slog.Any("error", err))
				// Continue with revocation for security reasons (don't leak info)
			}
		}
	}

	// If API key not found, log and continue for security reasons
	if existingAPIKey == nil {
		logger.Debug("API key not found for revocation",
			slog.String("api_key_name", apiKeyName))
	}

	apiKey = existingAPIKey
	matchedKey = existingAPIKey

	// For security reasons, perform all validations but don't return errors
	// This prevents information leakage about API key details
	if apiKey != nil {
		// Check if the API key belongs to the specified API
		if apiKey.APIId != config.ID {
			logger.Debug("API key does not belong to the specified API",
				slog.String("correlation_id", params.CorrelationID))
			return result, nil
		}

		err := s.canRevokeAPIKey(user, apiKey, logger)
		if err != nil {
			logger.Debug("User not authorized to revoke API key",
				slog.String("creator", apiKey.CreatedBy),
				slog.String("requesting_user", user.UserID))
			return nil, fmt.Errorf("API key revocation failed for API: '%s'", params.Handle)
		}

		// Check if the API key is already revoked
		if apiKey.Status == models.APIKeyStatusRevoked {
			logger.Debug("API key is already revoked")
			return result, nil
		}

		// At this point, all validations passed, proceed with actual revocation
		// Set status to revoked and update timestamp
		apiKey.Status = models.APIKeyStatusRevoked
		apiKey.UpdatedAt = time.Now()

		// Update the API key status in the database (if persistent mode)
		if s.db != nil {
			if err := s.db.UpdateAPIKey(apiKey); err != nil {
				logger.Error("Failed to update API key status in database",
					slog.Any("error", err))
				return nil, fmt.Errorf("failed to revoke API key: %w", err)
			}
		}

		// Remove the API key from memory store by name (since we have the matched key)
		if err := s.store.RemoveAPIKeyByID(config.ID, apiKey.ID); err != nil {
			logger.Error("Failed to remove API key from memory store",
				slog.Any("error", err))

			// Try to rollback database update if memory removal fails
			if s.db != nil {
				apiKey.Status = models.APIKeyStatusActive // Rollback status
				if rollbackErr := s.db.UpdateAPIKey(apiKey); rollbackErr != nil {
					logger.Error("Failed to rollback API key status in database",
						slog.Any("error", rollbackErr))
				}
			}
			return nil, fmt.Errorf("failed to revoke API key: %w", err)
		}
	}

	// Remove the API key from database (complete removal)
	// Note: This is cleanup only - the revocation is already complete
	if s.db != nil && matchedKey != nil {
		if err := s.db.RemoveAPIKeyAPIAndName(config.ID, matchedKey.Name); err != nil {
			logger.Warn("Failed to remove API key from database, but revocation was successful",
				slog.Any("error", err))
			// Don't return error - revocation was already successful
			// The key is marked as revoked in DB and removed from memory
		}
	}

	// remove the api key from the policy engine
	apiConfig, err := config.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		logger.Error("Failed to parse API configuration data",
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to revoke API key: %w", err)
	}

	apiId := config.ID
	apiName := apiConfig.DisplayName
	apiVersion := apiConfig.Version
	logger.Info("Removing API key from policy engine",
		slog.String("api key", apiKeyName),
		slog.String("api_name", apiName),
		slog.String("api_version", apiVersion))

	// Send the plain API key revocation to the policy engine via xDS
	// The policy engine will find and revoke the matching hashed key
	if s.xdsManager != nil {
		if err := s.xdsManager.RevokeAPIKey(apiId, apiName, apiVersion, apiKeyName, params.CorrelationID); err != nil {
			logger.Error("Failed to remove API key from policy engine",
				slog.Any("error", err))
			return nil, fmt.Errorf("failed to revoke API key: %w", err)
		}
	}

	logger.Info("API key revoked successfully",
		slog.String("api key", apiKeyName))

	return result, nil
}

// UpdateAPIKey updates an existing API key with a specific provided value
func (s *APIKeyService) UpdateAPIKey(params APIKeyUpdateParams) (*APIKeyUpdateResult, error) {
	baseLogger := params.Logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}

	// Create logger with pre-attached correlation ID and common fields
	logger := baseLogger.With(
		slog.String("correlation_id", params.CorrelationID),
		slog.String("handle", params.Handle),
		slog.String("api_key_name", params.APIKeyName),
	)

	user := params.User

	logger.Info("Starting API key update",
		slog.String("user", user.UserID))

	// Get the API configuration
	config, err := s.store.GetByHandle(params.Handle)
	if err != nil {
		logger.Warn("API configuration not found for API key update")
		return nil, fmt.Errorf("API configuration handle '%s' not found", params.Handle)
	}

	// Get the existing API key by name
	existingKey, err := s.store.GetAPIKeyByName(config.ID, params.APIKeyName)
	if err != nil {
		logger.Warn("API key not found for update")
		return nil, fmt.Errorf("API key '%s' not found for API '%s'", params.APIKeyName, params.Handle)
	}

	// Validate that only external API keys can be updated
	if existingKey.Source != "external" {
		logger.Warn("Attempted to update a locally generated API key",
			slog.String("source", existingKey.Source),
			slog.String("api_key_name", params.APIKeyName))
		return nil, fmt.Errorf("%w: updates are only allowed for externally generated API keys. For locally generated keys, please use the regenerate endpoint to create a new key", storage.ErrOperationNotAllowed)
	}

	// Check authorization - only creator can update their own key (unless admin)
	err = s.canRegenerateAPIKey(user, existingKey, logger)
	if err != nil {
		logger.Warn("User not authorized to update API key",
			slog.String("creator", existingKey.CreatedBy),
			slog.String("requesting_user", user.UserID))
		return nil, fmt.Errorf("not authorized to update API key '%s'", params.APIKeyName)
	}

	updatedKey, err := s.updateAPIKeyFromRequest(existingKey, params.Request, user.UserID, logger)
	if err != nil {
		logger.Error("Failed to update API key from request",
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to update API key from request: %w", err)
	}
	// Clear plaintext secret before persisting or storing
	updatedKey.PlainAPIKey = ""

	// Save to database (if persistent mode)
	if s.db != nil {
		if err := s.db.UpdateAPIKey(updatedKey); err != nil {
			logger.Error("Failed to update API key in database",
				slog.Any("error", err))
			return nil, fmt.Errorf("failed to update API key in database: %w", err)
		}
	}

	// Update in ConfigStore
	if err := s.store.StoreAPIKey(updatedKey); err != nil {
		logger.Error("Failed to update API key in ConfigStore",
			slog.Any("error", err))

		// Rollback database update if we have a persistent DB
		if s.db != nil {
			if rollbackErr := s.db.UpdateAPIKey(existingKey); rollbackErr != nil {
				logger.Error("Failed to rollback API key in database after ConfigStore failure",
					slog.Any("error", rollbackErr),
					slog.Any("original_error", err))
			} else {
				logger.Info("Successfully rolled back API key in database after ConfigStore failure")
			}
		}

		return nil, fmt.Errorf("failed to update API key in ConfigStore: %w", err)
	}

	apiConfig, err := config.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		logger.Error("Failed to parse API configuration data",
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to parse API configuration data: %w", err)
	}

	apiId := config.ID
	apiName := apiConfig.DisplayName
	apiVersion := apiConfig.Version
	logger.Info("Updating API key in policy engine",
		slog.String("api_name", apiName),
		slog.String("api_version", apiVersion),
		slog.String("user", user.UserID))

	// Update xDS snapshot to propagate to policy engine
	if s.xdsManager != nil {
		if err := s.xdsManager.StoreAPIKey(apiId, apiName, apiVersion, updatedKey, params.CorrelationID); err != nil {
			logger.Error("Failed to send updated API key to policy engine",
				slog.Any("error", err))
			return nil, fmt.Errorf("failed to send updated API key to policy engine: %w", err)
		}
	}

	// Build response
	// If API key was updated, use the new masked value; otherwise use existing
	responseMessage := "API key updated successfully"
	var responseAPIKey *string
	responseAPIKey = &updatedKey.MaskedAPIKey

	result := &APIKeyUpdateResult{
		Response: api.APIKeyCreationResponse{
			Status:  "success",
			Message: responseMessage,
			ApiKey: &api.APIKey{
				Name:        updatedKey.Name,
				DisplayName: &updatedKey.DisplayName,
				ApiKey:      responseAPIKey,
				ApiId:       params.Handle,
				Operations:  updatedKey.Operations,
				Status:      api.APIKeyStatus(updatedKey.Status),
				CreatedAt:   updatedKey.CreatedAt,
				CreatedBy:   updatedKey.CreatedBy,
				ExpiresAt:   updatedKey.ExpiresAt,
				Source:      api.APIKeySource(updatedKey.Source),
			},
		},
	}

	logger.Info("API key update completed successfully",
		slog.String("key_id", updatedKey.ID))

	return result, nil
}

// RegenerateAPIKey regenerates an existing API key
func (s *APIKeyService) RegenerateAPIKey(params APIKeyRegenerationParams) (*APIKeyRegenerationResult, error) {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	user := params.User

	logger.Info("Starting API key regeneration",
		slog.String("handle", params.Handle),
		slog.String("api_key_name", params.APIKeyName),
		slog.String("user", user.UserID),
		slog.String("correlation_id", params.CorrelationID))

	// Get the API configuration
	config, err := s.store.GetByHandle(params.Handle)
	if err != nil {
		logger.Warn("API configuration not found for API Key regeneration",
			slog.String("handle", params.Handle),
			slog.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("API configuration handle '%s' not found", params.Handle)
	}

	// Get the existing API key by name
	existingKey, err := s.store.GetAPIKeyByName(config.ID, params.APIKeyName)
	if err != nil {
		logger.Warn("API key not found for regeneration",
			slog.String("handle", params.Handle),
			slog.String("api_key_name", params.APIKeyName),
			slog.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("API key '%s' not found for API '%s'", params.APIKeyName, params.Handle)
	}

	err = s.canRegenerateAPIKey(user, existingKey, logger)
	if err != nil {
		logger.Warn("User attempting to regenerate API key is not the creator",
			slog.String("handle", params.Handle),
			slog.String("api_key_name", params.APIKeyName),
			slog.String("creator", existingKey.CreatedBy),
			slog.String("requesting_user", user.UserID),
			slog.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("API key regeneration failed for API: '%s'", params.Handle)
	}

	result := &APIKeyRegenerationResult{
		IsRetry: false,
	}

	// Regenerate API key using the extracted helper method
	// Retry once if duplicate is detected during generation
	regeneratedKey, err := s.regenerateAPIKey(existingKey, params.Request, user.UserID, logger)
	if err != nil {
		// Check if this is a duplicate key error
		if errors.Is(err, storage.ErrConflict) {
			// For local key regeneration, retry with a new generated key
			logger.Warn("API key collision detected during regeneration, retrying",
				slog.String("handle", params.Handle),
				slog.String("correlation_id", params.CorrelationID))

			regeneratedKey, err = s.regenerateAPIKey(existingKey, params.Request, user.UserID, logger)
			if err != nil {
				logger.Error("Failed to regenerate API key after retry",
					slog.Any("error", err),
					slog.String("handle", params.Handle),
					slog.String("correlation_id", params.CorrelationID))
				return nil, fmt.Errorf("failed to regenerate API key after retry: %w", err)
			}
			result.IsRetry = true
		} else {
			// Other error, return immediately
			logger.Error("Failed to regenerate API key",
				slog.Any("error", err),
				slog.String("handle", params.Handle),
				slog.String("correlation_id", params.CorrelationID))
			return nil, fmt.Errorf("failed to regenerate API key: %w", err)
		}
	}

	// Save regenerated API key to database (only if persistent mode)
	if s.db != nil {
		if err := s.db.UpdateAPIKey(regeneratedKey); err != nil {
			if errors.Is(err, storage.ErrConflict) {
				// Handle collision by retrying once with a new key
				logger.Warn("API key collision detected during regeneration, retrying",
					slog.String("handle", params.Handle),
					slog.String("correlation_id", params.CorrelationID))

				// Generate a new regenerated key
				regeneratedKey, err = s.regenerateAPIKey(existingKey, params.Request, user.UserID, logger)
				if err != nil {
					logger.Error("Failed to regenerate API key after collision",
						slog.Any("error", err),
						slog.String("correlation_id", params.CorrelationID))
					return nil, fmt.Errorf("failed to regenerate API key after collision: %w", err)
				}

				// Try saving again
				if err := s.db.UpdateAPIKey(regeneratedKey); err != nil {
					logger.Error("Failed to save regenerated API key after retry",
						slog.Any("error", err),
						slog.String("correlation_id", params.CorrelationID))
					return nil, fmt.Errorf("failed to save regenerated API key after retry: %w", err)
				}

				result.IsRetry = true
			} else {
				logger.Error("Failed to save regenerated API key to database",
					slog.Any("error", err),
					slog.String("handle", params.Handle),
					slog.String("correlation_id", params.CorrelationID))
				return nil, fmt.Errorf("failed to save regenerated API key to database: %w", err)
			}
		}
		// No need to revoke the old API key as the old one will be overwritten
	}

	plainAPIKey := regeneratedKey.PlainAPIKey // Store plain API key for response
	regeneratedKey.PlainAPIKey = ""           // Clear plain API key from the struct for security

	// Store the generated API key in the ConfigStore
	if err := s.store.StoreAPIKey(regeneratedKey); err != nil {
		logger.Error("Failed to store the regenerated API key in ConfigStore",
			slog.Any("error", err),
			slog.String("handle", params.Handle),
			slog.String("correlation_id", params.CorrelationID))

		// Rollback database save to maintain consistency
		if s.db != nil {
			if delErr := s.db.RemoveAPIKeyAPIAndName(regeneratedKey.APIId, regeneratedKey.Name); delErr != nil {
				logger.Error("Failed to rollback API key from database",
					slog.Any("error", delErr),
					slog.String("correlation_id", params.CorrelationID))
			}
		}
		return nil, fmt.Errorf("failed to store API key in ConfigStore: %w", err)
	}

	apiConfig, err := config.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		logger.Error("Failed to parse API configuration data",
			slog.Any("error", err),
			slog.String("handle", params.Handle),
			slog.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("failed to parse API configuration data: %w", err)
	}

	apiId := config.ID
	apiName := apiConfig.DisplayName
	apiVersion := apiConfig.Version
	logger.Info("Storing API key in policy engine",
		slog.String("handle", params.Handle),
		slog.String("name", regeneratedKey.Name),
		slog.String("api_name", apiName),
		slog.String("api_version", apiVersion),
		slog.String("user", user.UserID),
		slog.String("correlation_id", params.CorrelationID))

	// Update xDS snapshot if needed
	if s.xdsManager != nil {
		if err := s.xdsManager.StoreAPIKey(apiId, apiName, apiVersion, regeneratedKey, params.CorrelationID); err != nil {
			logger.Error("Failed to send regenerated API key to policy engine",
				slog.Any("error", err),
				slog.String("correlation_id", params.CorrelationID))
			return nil, fmt.Errorf("failed to send regenerated API key to policy engine: %w", err)
		}
	}

	// Build and return the response
	result.Response = s.buildAPIKeyResponse(regeneratedKey, params.Handle, plainAPIKey, false)

	logger.Info("API key regeneration completed successfully",
		slog.String("handle", params.Handle),
		slog.String("api_key_name", params.APIKeyName),
		slog.String("new_key_id", regeneratedKey.ID),
		slog.String("correlation_id", params.CorrelationID))

	return result, nil
}

// ListAPIKeys handles listing API keys for a specific API and user
func (s *APIKeyService) ListAPIKeys(params ListAPIKeyParams) (*ListAPIKeyResult, error) {
	logger := params.Logger
	user := params.User

	// Validate that API exists
	config, err := s.store.GetByHandle(params.Handle)
	if err != nil {
		logger.Warn("API configuration not found for API keys listing",
			slog.String("handle", params.Handle),
			slog.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("API configuration handle '%s' not found", params.Handle)
	}

	// Get all API keys for this API from memory store first
	var apiKeys []*models.APIKey

	// Try to get from memory store
	memoryKeys, err := s.store.GetAPIKeysByAPI(config.ID)
	if err != nil {
		logger.Debug("Failed to get API keys from memory store, trying database",
			slog.Any("error", err),
			slog.String("handle", params.Handle),
			slog.String("correlation_id", params.CorrelationID))

		// If memory store fails, try database
		if s.db != nil {
			dbKeys, dbErr := s.db.GetAPIKeysByAPI(config.ID)
			if dbErr != nil {
				logger.Error("Failed to get API keys from database",
					slog.Any("error", dbErr),
					slog.String("handle", params.Handle),
					slog.String("correlation_id", params.CorrelationID))
				return nil, fmt.Errorf("failed to retrieve API keys: %w", dbErr)
			}
			apiKeys = dbKeys
		} else {
			return nil, fmt.Errorf("failed to retrieve API keys: %w", err)
		}
	} else {
		apiKeys = memoryKeys
	}

	// Filter API keys by the requesting user (only show keys created by this user)
	// and only active keys
	userAPIKeys, err := s.filterAPIKeysByUser(user, apiKeys, logger)
	if err != nil {
		logger.Error("Failed to filter API keys by user",
			slog.Any("error", err),
			slog.String("handle", params.Handle),
			slog.String("user", user.UserID),
			slog.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("failed to filter API keys: %w", err)
	}

	// Filter only active API keys
	var activeUserAPIKeys []*models.APIKey
	for _, apiKey := range userAPIKeys {
		if apiKey.Status == models.APIKeyStatusActive {
			activeUserAPIKeys = append(activeUserAPIKeys, apiKey)
		}
	}

	// Build response API keys
	var responseAPIKeys []api.APIKey
	for _, key := range activeUserAPIKeys {
		// Return masked API key for display purposes
		responseAPIKey := api.APIKey{
			Name:          key.Name,
			DisplayName:   &key.DisplayName,
			ApiKey:        &key.MaskedAPIKey, // Return masked API key for security
			ApiId:         params.Handle,     // Use handle instead of internal API ID
			Operations:    key.Operations,
			Status:        api.APIKeyStatus(key.Status),
			CreatedAt:     key.CreatedAt,
			CreatedBy:     key.CreatedBy,
			ExpiresAt:     key.ExpiresAt,
			Source:        api.APIKeySource(key.Source),
			ExternalRefId: key.ExternalRefId,
		}
		responseAPIKeys = append(responseAPIKeys, responseAPIKey)
	}

	// Build the list response
	status := "success"
	totalCount := len(responseAPIKeys)

	result := &ListAPIKeyResult{
		Response: api.APIKeyListResponse{
			Status:     &status,
			ApiKeys:    &responseAPIKeys,
			TotalCount: &totalCount,
		},
	}

	logger.Info("API keys listed successfully",
		slog.String("handle", params.Handle),
		slog.String("user", user.UserID),
		slog.Int("total_count", totalCount),
		slog.String("correlation_id", params.CorrelationID))

	return result, nil
}

// createAPIKeyFromRequest creates a new API key from a request.
// Handles both local key generation (creates new random key) and external key injection
// (uses provided key from external platforms).
func (s *APIKeyService) createAPIKeyFromRequest(handle string, request *api.APIKeyCreationRequest, user string,
	config *models.StoredConfig) (*models.APIKey, error) {

	// Generate short unique ID (22 characters, URL-safe)
	// This is an internal ID for tracking and is always generated regardless of source
	id, err := s.generateShortUniqueID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique ID: %w", err)
	}

	// Determine if this is an external key injection or local key generation
	var plainAPIKeyValue string // The key value to be hashed
	var source string
	var isExternalKey bool

	if request.ApiKey != nil {
		// External key injection: use provided key AS-IS
		providedKey := strings.TrimSpace(*request.ApiKey)
		if err := s.ValidateAPIKeyValue(providedKey); err != nil {
			return nil, err
		}
		// Use the key as-is - we don't dictate format for external keys
		plainAPIKeyValue = providedKey
		source = "external"
		isExternalKey = true
	} else {
		// Local key generation: generate new random key with our standard format
		// Format: apip_{64_hex_chars} (32 bytes → hex encoded)
		plainAPIKeyValue, err = s.generateAPIKeyValue()
		if err != nil {
			return nil, err
		}
		source = "local"
		isExternalKey = false
	}

	// Hash the API key for storage and policy engine
	// Works for any format - we just hash whatever we receive
	hashedAPIKeyValue, err := s.hashAPIKey(plainAPIKeyValue)
	if err != nil {
		return nil, fmt.Errorf("failed to hash API key: %w", err)
	}

	// Generate masked API key for display purposes
	maskedAPIKeyValue := s.MaskAPIKey(plainAPIKeyValue)

	// Handle displayName - optional during creation
	var displayName string
	if request.DisplayName != nil && strings.TrimSpace(*request.DisplayName) != "" {
		// User provided a display name
		displayName = strings.TrimSpace(*request.DisplayName)

		// Validate user-provided displayName
		if err := ValidateDisplayName(displayName); err != nil {
			return nil, fmt.Errorf("invalid display name: %w", err)
		}
	} else {
		// Auto-generate display name: use handle + short ID portion
		// Example: "weather-api-jh~cPInv"
		displayName = fmt.Sprintf("%s-key-%s", handle, id[:8])
	}

	// Handle name - optional during creation
	var name string
	if request.Name != nil && strings.TrimSpace(*request.Name) != "" {
		// User provided a name
		name = strings.TrimSpace(*request.Name)
		if err := ValidateAPIKeyName(name); err != nil {
			return nil, fmt.Errorf("invalid name: %w", err)
		}
	} else {
		// Generate unique URL-safe name from displayName with collision handling
		// name is immutable after creation and used in path parameters
		// Use config.ID (API internal ID) not handle so uniqueness is checked per API
		name, err = s.generateUniqueAPIKeyName(config.ID, displayName, 5)
		if err != nil {
			return nil, fmt.Errorf("failed to generate unique API key name: %w", err)
		}
	}

	// Process operations
	operations := "[\"*\"]" // Default to all operations
	//if request.Operations != nil && len(*request.Operations) > 0 {
	//	operations = s.generateOperationsString(*request.Operations)
	//}

	now := time.Now()

	// Calculate expiration time
	var expiresAt *time.Time
	var unit *string
	var duration *int

	if request.ExpiresAt != nil {
		expiresAt = request.ExpiresAt
	} else if request.ExpiresIn != nil {
		// Store the original unit and duration values
		unitStr := string(request.ExpiresIn.Unit)
		unit = &unitStr
		duration = &request.ExpiresIn.Duration
		timeDuration := time.Duration(request.ExpiresIn.Duration)
		switch request.ExpiresIn.Unit {
		case api.APIKeyCreationRequestExpiresInUnitSeconds:
			timeDuration *= time.Second
		case api.APIKeyCreationRequestExpiresInUnitMinutes:
			timeDuration *= time.Minute
		case api.APIKeyCreationRequestExpiresInUnitHours:
			timeDuration *= time.Hour
		case api.APIKeyCreationRequestExpiresInUnitDays:
			timeDuration *= 24 * time.Hour
		case api.APIKeyCreationRequestExpiresInUnitWeeks:
			timeDuration *= 7 * 24 * time.Hour
		case api.APIKeyCreationRequestExpiresInUnitMonths:
			timeDuration *= 30 * 24 * time.Hour // Approximate month as 30 days
		default:
			return nil, fmt.Errorf("unsupported expiration unit: %s", request.ExpiresIn.Unit)
		}
		expiry := now.Add(timeDuration)
		expiresAt = &expiry
	}

	// Validate that expiresAt is in the future
	if expiresAt != nil && expiresAt.Before(now) {
		return nil, fmt.Errorf("API key expiration time must be in the future, got: %s (current time: %s)",
			expiresAt.Format(time.RFC3339), now.Format(time.RFC3339))
	}

	var indexKey *string
	if source == "external" {
		computedIndexKey := computeExternalKeyIndexKey(plainAPIKeyValue)
		if computedIndexKey == "" {
			return nil, fmt.Errorf("failed to compute index key")
		}
		indexKey = &computedIndexKey
	}

	apiKey := &models.APIKey{
		ID:           id,
		Name:         name,
		DisplayName:  displayName,
		APIKey:       hashedAPIKeyValue, // Store hashed key in database and policy engine
		MaskedAPIKey: maskedAPIKeyValue, // Store masked key for display
		APIId:        config.ID,
		Operations:   operations,
		Status:       models.APIKeyStatusActive,
		CreatedAt:    now,
		CreatedBy:    user,
		UpdatedAt:    now,
		ExpiresAt:    expiresAt,
		Unit:         unit,
		Duration:     duration,
		Source:       source, // "local" or "external"
		IndexKey:     indexKey,
	}

	// Set external reference fields if provided
	// external_ref_id is optional and used for tracing purposes only
	if request.ExternalRefId != nil && strings.TrimSpace(*request.ExternalRefId) != "" {
		externalRefId := strings.TrimSpace(*request.ExternalRefId)
		apiKey.ExternalRefId = &externalRefId
	}

	// Temporarily store the plain key for response generation
	// This field is not persisted and only used for returning to user
	// For external keys, we do NOT store the plain key (caller already has it)
	if !isExternalKey {
		apiKey.PlainAPIKey = plainAPIKeyValue
	}

	return apiKey, nil
}

// generateOperationsString creates a string array from operations in format "METHOD path"
// Example: ["GET /{country_code}/{city}", "POST /data"]
// Ignores the policies field from operations
func (s *APIKeyService) generateOperationsString(operations []api.Operation) string {
	if len(operations) == 0 {
		return "[\"*\"]" // Default to all operations if none specified
	}

	var operationStrings []string
	for _, op := range operations {
		// Format: "METHOD path" (ignoring policies)
		operationStr := fmt.Sprintf("%s %s", op.Method, op.Path)
		operationStrings = append(operationStrings, operationStr)
	}

	// Create JSON array string with comma-separated operations
	operationsJSON, err := json.Marshal(operationStrings)
	if err != nil {
		// Fallback to default if marshaling fails
		return "[\"*\"]"
	}

	return string(operationsJSON)
}

// buildAPIKeyResponse builds the response following the generated schema
func (s *APIKeyService) buildAPIKeyResponse(key *models.APIKey, handle string, plainAPIKey string, isExternalKeyInjection bool) api.APIKeyCreationResponse {
	if key == nil {
		return api.APIKeyCreationResponse{
			Status:  "error",
			Message: "API key is nil",
		}
	}

	// Use provided message or default
	var message string
	if isExternalKeyInjection {
		message = "API key registered successfully"
	} else {
		message = "API key generated successfully"
	}

	// Calculate remaining API key quota
	var remainingQuota *int
	currentCount, err := s.getCurrentAPIKeyCount(key.APIId, key.CreatedBy)
	if err == nil {
		maxAllowed := s.apiKeyConfig.APIKeysPerUserPerAPI
		remaining := maxAllowed - currentCount
		if remaining < 0 {
			remaining = 0
		}
		remainingQuota = &remaining
	}

	// Use plainAPIKey for response if available, otherwise mask the hashed key
	var responseAPIKey *string
	if plainAPIKey != "" && !isExternalKeyInjection {
		// Format: apip_{64_hex_chars}.{hex_encoded_id}
		// Since the ID is already base64url encoded (22 chars), we can use it directly
		formattedAPIKey := plainAPIKey + constants.APIKeySeparator + key.ID
		responseAPIKey = &formattedAPIKey
	} else {
		// For existing keys where plainAPIKey is not available, don't return the hashed key
		responseAPIKey = nil
	}

	return api.APIKeyCreationResponse{
		Status:               "success",
		Message:              message,
		RemainingApiKeyQuota: remainingQuota,
		ApiKey: &api.APIKey{
			Name:        key.Name,
			DisplayName: &key.DisplayName,
			ApiKey:      responseAPIKey, // Return plain key only for locally generated keys
			ApiId:       handle,
			Operations:  key.Operations,
			Status:      api.APIKeyStatus(key.Status),
			CreatedAt:   key.CreatedAt,
			CreatedBy:   key.CreatedBy,
			ExpiresAt:   key.ExpiresAt,
			Source:      api.APIKeySource(key.Source),
		},
	}
}

// updateAPIKeyFromRequest updates an existing API key with a specific provided value
// Only mutable fields (displayName, api_key value, expiration) can be updated
// Immutable fields (name, source, createdAt, createdBy) are preserved from existing key
func (s *APIKeyService) updateAPIKeyFromRequest(existingKey *models.APIKey, request api.APIKeyCreationRequest,
	user string, logger *slog.Logger) (*models.APIKey, error) {

	// Validate required field: api_key value
	if request.ApiKey == nil || strings.TrimSpace(*request.ApiKey) == "" {
		return nil, fmt.Errorf("api_key is required for update")
	}

	plainAPIKeyValue := strings.TrimSpace(*request.ApiKey)
	if err := s.ValidateAPIKeyValue(plainAPIKeyValue); err != nil {
		return nil, fmt.Errorf("invalid API key value: %w", err)
	}

	// Handle displayName - optional during update
	// If not provided or empty, keep the existing displayName
	var displayName string
	if request.DisplayName != nil && strings.TrimSpace(*request.DisplayName) != "" {
		displayName = strings.TrimSpace(*request.DisplayName)

		// Validate user-provided displayName
		if err := ValidateDisplayName(displayName); err != nil {
			return nil, fmt.Errorf("invalid display name: %w", err)
		}
	} else {
		return nil, fmt.Errorf("display name is required for update")
	}

	operations := "[\"*\"]" // Default to all operations

	// Hash the new API key for storage
	hashedAPIKeyValue, err := s.hashAPIKey(plainAPIKeyValue)
	if err != nil {
		return nil, fmt.Errorf("failed to hash API key: %w", err)
	}

	// Generate masked API key for display purposes
	maskedAPIKeyValue := s.MaskAPIKey(plainAPIKeyValue)

	now := time.Now()

	// Determine expiration settings based on request and existing key
	var expiresAt *time.Time
	var unit *string
	var duration *int

	if request.ExpiresAt != nil {
		if request.ExpiresAt.Before(now) {
			return nil, fmt.Errorf("API key expiration time must be in the future, got: %s (current time: %s)",
				request.ExpiresAt.Format(time.RFC3339), now.Format(time.RFC3339))
		}
		// If expires_at is explicitly provided, use it
		expiresAt = request.ExpiresAt
		logger.Info("Using provided expires_at for update", slog.Time("expires_at", *expiresAt))
	} else if request.ExpiresIn != nil {
		// If expires_in is provided, calculate expires_at from now
		unitStr := string(request.ExpiresIn.Unit)
		unit = &unitStr
		duration = &request.ExpiresIn.Duration

		timeDuration := time.Duration(request.ExpiresIn.Duration)
		switch request.ExpiresIn.Unit {
		case api.APIKeyCreationRequestExpiresInUnitSeconds:
			timeDuration *= time.Second
		case api.APIKeyCreationRequestExpiresInUnitMinutes:
			timeDuration *= time.Minute
		case api.APIKeyCreationRequestExpiresInUnitHours:
			timeDuration *= time.Hour
		case api.APIKeyCreationRequestExpiresInUnitDays:
			timeDuration *= 24 * time.Hour
		case api.APIKeyCreationRequestExpiresInUnitWeeks:
			timeDuration *= 7 * 24 * time.Hour
		case api.APIKeyCreationRequestExpiresInUnitMonths:
			timeDuration *= 30 * 24 * time.Hour
		default:
			return nil, fmt.Errorf("unsupported expiration unit: %s", request.ExpiresIn.Unit)
		}
		expiry := now.Add(timeDuration)
		expiresAt = &expiry
		logger.Info("Using provided expires_in for update",
			slog.String("unit", unitStr),
			slog.Int("duration", *duration),
			slog.Time("calculated_expires_at", *expiresAt))
	} else if request.ExpiresAt == nil && request.ExpiresIn == nil {
		// Existing key has no expiry, new key also has no expiry
		expiresAt = nil
		logger.Info("No expiry set for updated key (matching existing key)")
	}

	// Validate that expiresAt is in the future (if set)
	if expiresAt != nil && expiresAt.Before(now) {
		return nil, fmt.Errorf("API key expiration time must be in the future, got: %s (current time: %s)",
			expiresAt.Format(time.RFC3339), now.Format(time.RFC3339))
	}

	var indexKey *string
	if existingKey.Source == "external" {
		computedIndexKey := computeExternalKeyIndexKey(plainAPIKeyValue)
		if computedIndexKey == "" {
			return nil, fmt.Errorf("failed to compute index key")
		}
		indexKey = &computedIndexKey
	}

	// Create the regenerated API key
	updatedKey := &models.APIKey{
		ID:           existingKey.ID,
		Name:         existingKey.Name,
		DisplayName:  displayName,
		APIKey:       hashedAPIKeyValue, // Store hashed key
		MaskedAPIKey: maskedAPIKeyValue, // Store masked key for display
		APIId:        existingKey.APIId,
		Operations:   operations,
		Status:       models.APIKeyStatusActive,
		CreatedAt:    existingKey.CreatedAt,
		CreatedBy:    existingKey.CreatedBy,
		UpdatedAt:    now,
		ExpiresAt:    expiresAt,
		Unit:         unit,
		Duration:     duration,
		Source:       existingKey.Source, // Preserve source from original key.
		IndexKey:     indexKey,
	}

	// Temporarily store the plain key for response generation
	updatedKey.PlainAPIKey = plainAPIKeyValue

	return updatedKey, nil
}

// regenerateAPIKey creates a new API key for regeneration based on existing key and request parameters
func (s *APIKeyService) regenerateAPIKey(existingKey *models.APIKey, request api.APIKeyRegenerationRequest,
	user string, logger *slog.Logger) (*models.APIKey, error) {
	// Generate new API key value
	plainAPIKeyValue, err := s.generateAPIKeyValue()
	if err != nil {
		return nil, err
	}

	// Hash the new API key for storage
	hashedAPIKeyValue, err := s.hashAPIKey(plainAPIKeyValue)
	if err != nil {
		return nil, fmt.Errorf("failed to hash regenerated API key: %w", err)
	}

	// Generate masked API key for display purposes
	maskedAPIKeyValue := s.MaskAPIKey(plainAPIKeyValue)

	now := time.Now()

	// Determine expiration settings based on request and existing key
	var expiresAt *time.Time
	var unit *string
	var duration *int

	if request.ExpiresAt != nil {
		if request.ExpiresAt.Before(now) {
			return nil, fmt.Errorf("API key expiration time must be in the future, got: %s (current time: %s)",
				request.ExpiresAt.Format(time.RFC3339), now.Format(time.RFC3339))
		}
		// If expires_at is explicitly provided, use it
		expiresAt = request.ExpiresAt
		logger.Info("Using provided expires_at for regeneration", slog.Time("expires_at", *expiresAt))
	} else if request.ExpiresIn != nil {
		// If expires_in is provided, calculate expires_at from now
		unitStr := string(request.ExpiresIn.Unit)
		unit = &unitStr
		duration = &request.ExpiresIn.Duration

		timeDuration := time.Duration(request.ExpiresIn.Duration)
		switch request.ExpiresIn.Unit {
		case api.APIKeyRegenerationRequestExpiresInUnitSeconds:
			timeDuration *= time.Second
		case api.APIKeyRegenerationRequestExpiresInUnitMinutes:
			timeDuration *= time.Minute
		case api.APIKeyRegenerationRequestExpiresInUnitHours:
			timeDuration *= time.Hour
		case api.APIKeyRegenerationRequestExpiresInUnitDays:
			timeDuration *= 24 * time.Hour
		case api.APIKeyRegenerationRequestExpiresInUnitWeeks:
			timeDuration *= 7 * 24 * time.Hour
		case api.APIKeyRegenerationRequestExpiresInUnitMonths:
			timeDuration *= 30 * 24 * time.Hour
		default:
			return nil, fmt.Errorf("unsupported expiration unit: %s", request.ExpiresIn.Unit)
		}
		expiry := now.Add(timeDuration)
		expiresAt = &expiry
		logger.Info("Using provided expires_in for regeneration",
			slog.String("unit", unitStr),
			slog.Int("duration", *duration),
			slog.Time("calculated_expires_at", *expiresAt))
	} else {
		// No expiration provided in request, use existing key's logic
		if existingKey.Unit != nil && existingKey.Duration != nil {
			// Existing key has duration/unit, apply same duration from now
			unit = existingKey.Unit
			duration = existingKey.Duration

			timeDuration := time.Duration(*existingKey.Duration)
			switch *existingKey.Unit {
			case string(api.APIKeyRegenerationRequestExpiresInUnitSeconds):
				timeDuration *= time.Second
			case string(api.APIKeyRegenerationRequestExpiresInUnitMinutes):
				timeDuration *= time.Minute
			case string(api.APIKeyRegenerationRequestExpiresInUnitHours):
				timeDuration *= time.Hour
			case string(api.APIKeyRegenerationRequestExpiresInUnitDays):
				timeDuration *= 24 * time.Hour
			case string(api.APIKeyRegenerationRequestExpiresInUnitWeeks):
				timeDuration *= 7 * 24 * time.Hour
			case string(api.APIKeyRegenerationRequestExpiresInUnitMonths):
				timeDuration *= 30 * 24 * time.Hour
			default:
				return nil, fmt.Errorf("unsupported existing expiration unit: %s", *existingKey.Unit)
			}
			expiry := now.Add(timeDuration)
			expiresAt = &expiry
			logger.Info("Using existing key's duration settings for regeneration",
				slog.String("unit", *unit),
				slog.Int("duration", *duration),
				slog.Time("calculated_expires_at", *expiresAt))
		} else if existingKey.ExpiresAt != nil {
			// Existing key has absolute expiry, use same expiry
			expiresAt = existingKey.ExpiresAt
			logger.Info("Using existing key's expires_at for regeneration", slog.Time("expires_at", *expiresAt))
		} else {
			// Existing key has no expiry, new key also has no expiry
			expiresAt = nil
			logger.Info("No expiry set for regenerated key (matching existing key)")
		}
	}

	// Validate that expiresAt is in the future (if set)
	if expiresAt != nil && expiresAt.Before(now) {
		return nil, fmt.Errorf("API key expiration time must be in the future, got: %s (current time: %s)",
			expiresAt.Format(time.RFC3339), now.Format(time.RFC3339))
	}

	// Create the regenerated API key
	regeneratedKey := &models.APIKey{
		ID:           existingKey.ID,
		Name:         existingKey.Name,
		APIKey:       hashedAPIKeyValue, // Store hashed key
		MaskedAPIKey: maskedAPIKeyValue, // Store masked key for display
		APIId:        existingKey.APIId,
		Operations:   existingKey.Operations,
		Status:       models.APIKeyStatusActive,
		CreatedAt:    existingKey.CreatedAt,
		CreatedBy:    existingKey.CreatedBy,
		UpdatedAt:    now,
		ExpiresAt:    expiresAt,
		Unit:         unit,
		Duration:     duration,
		Source:       existingKey.Source, // Preserve source from original key
	}

	// Temporarily store the plain key for response generation
	regeneratedKey.PlainAPIKey = plainAPIKeyValue

	return regeneratedKey, nil
}

// canRevokeAPIKey determines if a user can revoke a specific API key
// Admin role can revoke any API key of an API. Other users can only revoke API keys that they created.
func (s *APIKeyService) canRevokeAPIKey(user *commonmodels.AuthContext, apiKey *models.APIKey, logger *slog.Logger) error {
	if user == nil {
		return fmt.Errorf("user authentication required")
	}

	if apiKey == nil {
		return fmt.Errorf("API key not found")
	}

	logger.Debug("Checking API key revocation authorization",
		slog.Any("roles", user.Roles),
		slog.String("api_key_name", apiKey.Name),
		slog.String("api_key_creator", apiKey.CreatedBy))

	// Admin role can revoke any API key
	if s.isAdmin(user) {
		logger.Debug("User has admin role, authorized to revoke any API key",
			slog.String("api_key_name", apiKey.Name))
		return nil
	}

	// Non-admin users can only revoke keys they created
	if apiKey.CreatedBy != user.UserID {
		logger.Warn("User cannot revoke API key - not the creator and not admin",
			slog.String("api_key_name", apiKey.Name),
			slog.String("api_key_creator", apiKey.CreatedBy))
		return fmt.Errorf("API key revocation not authorized for user")
	}

	logger.Debug("User authorized to revoke API key as creator",
		slog.String("api_key_name", apiKey.Name))

	return nil
}

// canRegenerateAPIKey determines if a user can regenerate a specific API key
// Only the user who created the API key can regenerate it
func (s *APIKeyService) canRegenerateAPIKey(user *commonmodels.AuthContext, apiKey *models.APIKey, logger *slog.Logger) error {
	if user == nil {
		return fmt.Errorf("user authentication required")
	}

	if apiKey == nil {
		return fmt.Errorf("API key not found")
	}

	logger.Debug("Checking API key regeneration authorization",
		slog.String("user_id", user.UserID),
		slog.Any("roles", user.Roles),
		slog.String("api_key_name", apiKey.Name),
		slog.String("api_key_creator", apiKey.CreatedBy))

	// Only the creator can regenerate the API key
	if apiKey.CreatedBy != user.UserID {
		logger.Warn("User cannot regenerate API key - not the creator",
			slog.String("user_id", user.UserID),
			slog.String("api_key_name", apiKey.Name),
			slog.String("api_key_creator", apiKey.CreatedBy))
		return fmt.Errorf("only the creator of the API key can regenerate it")
	}

	logger.Debug("User authorized to regenerate API key",
		slog.String("user_id", user.UserID),
		slog.String("api_key_name", apiKey.Name))

	return nil
}

// filterAPIKeysByUser filters a list of API keys based on the user's roles
// Admin role can list all keys of an API. Other users can view only API keys that they created.
func (s *APIKeyService) filterAPIKeysByUser(user *commonmodels.AuthContext, apiKeys []*models.APIKey,
	logger *slog.Logger) ([]*models.APIKey, error) {
	if user == nil {
		return nil, fmt.Errorf("user authentication required")
	}

	logger.Debug("Checking API key list authorization",
		slog.String("user_id", user.UserID),
		slog.Any("roles", user.Roles),
		slog.Int("total_keys", len(apiKeys)))

	// Admin role can see all API keys
	if s.isAdmin(user) {
		logger.Debug("User has admin role, returning all API keys",
			slog.String("user_id", user.UserID),
			slog.Int("returned_keys", len(apiKeys)))
		return apiKeys, nil
	}

	// Non-admin users can only see keys they created
	var userAPIKeys []*models.APIKey
	for _, apiKey := range apiKeys {
		if apiKey.CreatedBy == user.UserID {
			userAPIKeys = append(userAPIKeys, apiKey)
		}
	}

	logger.Debug("User can only see own API keys",
		slog.String("user_id", user.UserID),
		slog.Int("owned_keys", len(userAPIKeys)),
		slog.Int("total_keys", len(apiKeys)))

	return userAPIKeys, nil
}

// generateAPIKeyValue generates a new API key value with collision handling
func (s *APIKeyService) generateAPIKeyValue() (string, error) {
	randomBytes := make([]byte, constants.APIKeyLen)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return constants.APIKeyPrefix + hex.EncodeToString(randomBytes), nil
}

// MaskAPIKey masks an API key for secure logging, showing first 10 characters
func (s *APIKeyService) MaskAPIKey(apiKey string) string {
	if len(apiKey) <= 10 {
		return "**********"
	}
	return apiKey[:10] + "*********"
}

// isAdmin checks if the user has admin role
func (s *APIKeyService) isAdmin(user *commonmodels.AuthContext) bool {
	return slices.Contains(user.Roles, "admin")
}

// isDeveloper checks if the user has developer role
func (s *APIKeyService) isDeveloper(user *commonmodels.AuthContext) bool {
	return slices.Contains(user.Roles, "developer")
}

// hashAPIKey securely hashes an API key using the configured algorithm
// Returns the hashed API key that should be stored in database and policy engine
// If hashing is disabled, returns the plain API key
func (s *APIKeyService) hashAPIKey(plainAPIKey string) (string, error) {
	if plainAPIKey == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}

	// Hash based on configured algorithm
	switch strings.ToLower(s.apiKeyConfig.Algorithm) {
	case constants.HashingAlgorithmSHA256:
		return s.hashAPIKeyWithSHA256(plainAPIKey)
	case constants.HashingAlgorithmBcrypt:
		return s.hashAPIKeyWithBcrypt(plainAPIKey)
	case constants.HashingAlgorithmArgon2ID:
		return s.hashAPIKeyWithArgon2ID(plainAPIKey)
	default:
		// Default to SHA256 if algorithm is not recognized
		return s.hashAPIKeyWithSHA256(plainAPIKey)
	}
}

// hashAPIKeyWithSHA256 securely hashes an API key using SHA-256 with salt
// Returns the hashed API key that should be stored in database and policy engine
func (s *APIKeyService) hashAPIKeyWithSHA256(plainAPIKey string) (string, error) {
	if plainAPIKey == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}

	salt := make([]byte, sha256SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Generate hash using SHA-256
	hasher := sha256.New()
	hasher.Write([]byte(plainAPIKey))
	hasher.Write(salt)
	hash := hasher.Sum(nil)

	// Encode salt and hash using hex
	saltHex := hex.EncodeToString(salt)
	hashHex := hex.EncodeToString(hash)

	// Format: $sha256$<salt_hex>$<hash_hex>
	hashedKey := fmt.Sprintf("$sha256$%s$%s", saltHex, hashHex)

	return hashedKey, nil
}

// hashAPIKeyWithBcrypt securely hashes an API key using bcrypt
// Returns the hashed API key that should be stored in database and policy engine
func (s *APIKeyService) hashAPIKeyWithBcrypt(plainAPIKey string) (string, error) {
	if plainAPIKey == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}

	// Generate bcrypt hash with specified cost
	hashedKey, err := bcrypt.GenerateFromPassword([]byte(plainAPIKey), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash API key with bcrypt: %w", err)
	}

	return string(hashedKey), nil
}

// hashAPIKeyWithArgon2ID securely hashes an API key using Argon2id
// Returns the hashed API key that should be stored in database and policy engine
func (s *APIKeyService) hashAPIKeyWithArgon2ID(plainAPIKey string) (string, error) {
	if plainAPIKey == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}

	// Generate random salt
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Generate hash using Argon2id
	hash := argon2.IDKey([]byte(plainAPIKey), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Encode salt and hash using base64
	saltEncoded := base64.RawStdEncoding.EncodeToString(salt)
	hashEncoded := base64.RawStdEncoding.EncodeToString(hash)

	// Format: $argon2id$v=19$m=65536,t=1,p=4$<salt_b64>$<hash_b64>
	hashedKey := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory, argon2Time, argon2Threads, saltEncoded, hashEncoded)

	return hashedKey, nil
}

// compareAPIKeys compares API keys for external use
// Returns true if the plain API key matches the hash, false otherwise
// If hashing is disabled, performs plain text comparison
func (s *APIKeyService) compareAPIKeys(providedAPIKey, storedAPIKey string) bool {
	if providedAPIKey == "" || storedAPIKey == "" {
		return false
	}

	// Check if it's an SHA-256 hash (format: $sha256$<salt_hex>$<hash_hex>)
	if strings.HasPrefix(storedAPIKey, "$sha256$") {
		return s.compareSHA256Hash(providedAPIKey, storedAPIKey)
	}

	// Check if it's a bcrypt hash (starts with $2a$, $2b$, or $2y$)
	if strings.HasPrefix(storedAPIKey, "$2a$") ||
		strings.HasPrefix(storedAPIKey, "$2b$") ||
		strings.HasPrefix(storedAPIKey, "$2y$") {
		return s.compareBcryptHash(providedAPIKey, storedAPIKey)
	}

	// Check if it's an Argon2id hash
	if strings.HasPrefix(storedAPIKey, "$argon2id$") {
		err := s.compareArgon2id(providedAPIKey, storedAPIKey)
		return err == nil
	}

	// If no hash format is detected and hashing is enabled, try plain text comparison as fallback
	// This handles migration scenarios where some keys might still be stored as plain text
	return subtle.ConstantTimeCompare([]byte(providedAPIKey), []byte(storedAPIKey)) == 1
}

// compareSHA256Hash validates an encoded SHA-256 hash and compares it to the provided password.
// Expected format: $sha256$<salt_hex>$<hash_hex>
// Returns true if the plain API key matches the hash, false otherwise
func (s *APIKeyService) compareSHA256Hash(apiKey, encoded string) bool {
	if apiKey == "" || encoded == "" {
		return false
	}

	// Parse the hash format: $sha256$<salt_hex>$<hash_hex>
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[1] != "sha256" {
		return false
	}

	// Decode salt and hash from hex
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}

	storedHash, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}

	// Compute hash of the provided key with the stored salt
	hasher := sha256.New()
	hasher.Write([]byte(apiKey))
	hasher.Write(salt)
	computedHash := hasher.Sum(nil)

	// Constant-time comparison
	return subtle.ConstantTimeCompare(computedHash, storedHash) == 1
}

// compareBcryptHash validates an encoded bcrypt hash and compares it to the provided password.
// Returns true if the plain API key matches the hash, false otherwise
func (s *APIKeyService) compareBcryptHash(apiKey, encoded string) bool {
	if apiKey == "" || encoded == "" {
		return false
	}

	// Compare the provided key with the stored bcrypt hash
	err := bcrypt.CompareHashAndPassword([]byte(encoded), []byte(apiKey))
	return err == nil
}

// compareArgon2id parses an encoded Argon2id hash and compares it to the provided password.
// Expected format: $argon2id$v=19$m=<m>,t=<t>,p=<p>$<salt_b64>$<hash_b64>
func (s *APIKeyService) compareArgon2id(apiKey, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return fmt.Errorf("invalid argon2id hash format")
	}

	// parts[2] -> v=19
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return err
	}
	if version != argon2.Version {
		return fmt.Errorf("unsupported argon2 version: %d", version)
	}

	// parts[3] -> m=<m>,t=<t>,p=<p>
	var mem uint32
	var iters uint32
	var threads uint8
	var t, m, p uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return err
	}
	mem = m
	iters = t
	threads = uint8(p)

	// decode salt and hash (try RawStd then Std)
	salt, err := decodeBase64(parts[4])
	if err != nil {
		return err
	}
	hash, err := decodeBase64(parts[5])
	if err != nil {
		return err
	}

	derived := argon2.IDKey([]byte(apiKey), salt, iters, mem, threads, uint32(len(hash)))
	if subtle.ConstantTimeCompare(derived, hash) == 1 {
		return nil
	}
	return errors.New("API key mismatch")
}

// decodeBase64 decodes a base64 string, trying RawStdEncoding first, then StdEncoding
func decodeBase64(s string) ([]byte, error) {
	b, err := base64.RawStdEncoding.DecodeString(s)
	if err == nil {
		return b, nil
	}
	// try StdEncoding as a fallback
	return base64.StdEncoding.DecodeString(s)
}

// SetHashingConfig allows updating the hashing configuration at runtime
func (s *APIKeyService) SetHashingConfig(config *config.APIKeyConfig) {
	s.apiKeyConfig = config
}

// GetHashingConfig returns the current hashing configuration
func (s *APIKeyService) GetHashingConfig() *config.APIKeyConfig {
	return s.apiKeyConfig
}

// enforceAPIKeyLimit checks if the user has exceeded the configured API key limit for the given API
func (s *APIKeyService) enforceAPIKeyLimit(apiId, userID string, logger *slog.Logger) error {
	// Get the current count of active API keys for this user and API
	var currentCount int
	var err error

	// Try to get count from memory store first
	if currentCount, err = s.store.CountActiveAPIKeysByUserAndAPI(apiId, userID); err != nil {
		logger.Debug("Failed to count API keys from memory store, trying database",
			slog.Any("error", err),
			slog.String("api_id", apiId),
			slog.String("user_id", userID))

		// If memory store fails, try database
		if s.db != nil {
			if currentCount, err = s.db.CountActiveAPIKeysByUserAndAPI(apiId, userID); err != nil {
				logger.Error("Failed to count API keys from database",
					slog.Any("error", err),
					slog.String("api_id", apiId),
					slog.String("user_id", userID))
				return fmt.Errorf("failed to check API key count: %w", err)
			}
		} else {
			return fmt.Errorf("failed to check API key count: %w", err)
		}
	}

	maxAllowed := s.apiKeyConfig.APIKeysPerUserPerAPI

	logger.Debug("Checking API key limit",
		slog.String("api_id", apiId),
		slog.String("user_id", userID),
		slog.Int("current_count", currentCount),
		slog.Int("max_allowed", maxAllowed))

	if currentCount >= maxAllowed {
		logger.Warn("API key limit exceeded",
			slog.String("api_id", apiId),
			slog.String("user_id", userID),
			slog.Int("current_count", currentCount),
			slog.Int("max_allowed", maxAllowed))
		return fmt.Errorf("API key limit exceeded: user has %d active keys, maximum allowed is %d",
			currentCount, maxAllowed)
	}

	logger.Debug("API key limit check passed",
		slog.String("api_id", apiId),
		slog.String("user_id", userID),
		slog.Int("current_count", currentCount),
		slog.Int("max_allowed", maxAllowed))

	return nil
}

// getCurrentAPIKeyCount gets the current count of active API keys for a user and API
func (s *APIKeyService) getCurrentAPIKeyCount(apiId, userID string) (int, error) {
	// Try to get count from memory store first
	if currentCount, err := s.store.CountActiveAPIKeysByUserAndAPI(apiId, userID); err == nil {
		return currentCount, nil
	}

	// If memory store fails, try database
	if s.db != nil {
		if currentCount, err := s.db.CountActiveAPIKeysByUserAndAPI(apiId, userID); err == nil {
			return currentCount, nil
		}
	}

	// If both fail, return error
	return 0, fmt.Errorf("failed to get current API key count")
}

// generateShortSuffix generates a short 4-character URL-safe suffix
// Uses 3 random bytes encoded as base64url, similar to patterns used in the repository
// Returns a string like "efhh" or "xrhy"
func (s *APIKeyService) generateShortSuffix() (string, error) {
	// Generate 3 random bytes for a 4-character suffix
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for suffix: %w", err)
	}

	// Encode as base64url without padding (3 bytes = 4 chars)
	suffix := base64.RawURLEncoding.EncodeToString(randomBytes)

	// Replace any non-alphanumeric characters to ensure only lowercase letters and numbers
	// Convert to lowercase and replace special chars with random letters
	suffix = strings.ToLower(suffix)
	suffix = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		// Replace special chars with a random lowercase letter
		return 'a' + rune(randomBytes[0]%26)
	}, suffix)

	return suffix, nil
}

// generateUniqueAPIKeyName generates a unique name from displayName, handling collisions
// If a name collision occurs, appends a short suffix (e.g., "-efhh", "-xrhy")
// Retries up to maxRetries times to find a unique name
func (s *APIKeyService) generateUniqueAPIKeyName(apiId, displayName string, maxRetries int) (string, error) {
	// Generate base name from display name
	baseName, err := GenerateAPIKeyName(displayName)
	if err != nil {
		return "", fmt.Errorf("failed to generate base name: %w", err)
	}

	// Try base name first
	exists, err := s.checkAPIKeyNameExists(apiId, baseName)
	if err != nil {
		return "", fmt.Errorf("failed to check name existence: %w", err)
	}
	if !exists {
		return baseName, nil
	}

	// Name collision detected, try with suffixes
	for i := 0; i < maxRetries; i++ {
		suffix, err := s.generateShortSuffix()
		if err != nil {
			return "", err
		}

		uniqueName := baseName + "-" + suffix

		// Enforce max length (name field is typically 63 chars max)
		if len(uniqueName) > constants.APIKeyNameMaxLength {
			// Truncate base name to make room for suffix
			truncatedBase := baseName[:constants.APIKeyNameMaxLength-len(suffix)-1]
			uniqueName = truncatedBase + "-" + suffix
		}

		exists, err := s.checkAPIKeyNameExists(apiId, uniqueName)
		if err != nil {
			return "", fmt.Errorf("failed to check name existence: %w", err)
		}
		if !exists {
			return uniqueName, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique name after %d attempts", maxRetries)
}

// checkAPIKeyNameExists checks if an API key name already exists for the given API
func (s *APIKeyService) checkAPIKeyNameExists(apiId, name string) (bool, error) {
	if s.db != nil {
		if apiKey, _ := s.db.GetAPIKeysByAPIAndName(apiId, name); apiKey != nil {
			return true, nil
		}
	}

	// Fallback to memory store (for in-memory mode)
	if s.store != nil {
		if apiKey, err := s.store.GetAPIKeyByName(apiId, name); err == nil && apiKey != nil {
			return true, nil
		}
	}
	return false, nil
}

// generateShortUniqueID generates a 22-character URL-safe unique identifier
// Uses 16 random bytes (128 bits) encoded as base64url without padding
// Results in exactly 22 characters that are URL-safe and highly unique
// Note: Replaces any underscore characters with tilde (~) to avoid underscore usage
func (s *APIKeyService) generateShortUniqueID() (string, error) {
	// Generate 16 random bytes (128 bits of entropy)
	// This provides sufficient uniqueness for API key IDs
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for ID: %w", err)
	}

	// Encode as base64url without padding
	// 16 bytes -> 22 characters (base64 encoding: 4 chars per 3 bytes, so 16 bytes = ~21.33 -> 22 chars)
	// Use RawURLEncoding (base64url without padding) for URL-safe characters
	id := base64.RawURLEncoding.EncodeToString(randomBytes)

	// Replace any underscore characters with tilde (~) which is also URL-safe
	// This ensures the ID never contains underscores
	id = strings.ReplaceAll(id, "_", "~")

	return id, nil
}

// CreateExternalAPIKeyFromEvent creates an API key from an external event (websocket).
// This is used when platform-api broadcasts an apikey.created event.
// The plain API key is hashed before storage.
func (s *APIKeyService) CreateExternalAPIKeyFromEvent(
	handle string,
	user string,
	request *api.APIKeyCreationRequest,
	correlationID string,
	logger *slog.Logger,
) (*APIKeyCreationResult, error) {
	if request == nil {
		logger.Error("nil APIKeyCreationRequest",
			slog.String("api_id", handle),
			slog.String("correlation_id", correlationID),
		)
		return nil, fmt.Errorf("nil APIKeyCreationRequest for api %s", handle)
	}

	logger.Info("Creating external API key from event",
		slog.String("api_id", handle),
		slog.Bool("has_expiry", request.ExpiresAt != nil),
	)

	params := APIKeyCreationParams{
		Handle:  handle,
		Request: *request,
		User: &commonmodels.AuthContext{
			UserID: user,
		},
		Logger:        logger,
		CorrelationID: correlationID,
	}

	result, err := s.CreateAPIKey(params)
	if err != nil {
		logger.Error("Failed to create external API key", slog.Any("error", err))
		return nil, err
	}

	return result, nil
}

// RevokeExternalAPIKeyFromEvent revokes an API key from an external event (websocket).
// This is used when platform-api broadcasts an apikey.revoked event.
func (s *APIKeyService) RevokeExternalAPIKeyFromEvent(
	handle string,
	keyName string,
	user string,
	correlationID string,
	logger *slog.Logger,
) error {
	apiKeyRevocationParams := APIKeyRevocationParams{
		Handle:     handle,
		APIKeyName: keyName,
		User: &commonmodels.AuthContext{
			UserID: user,
		},
		Logger:        logger,
		CorrelationID: correlationID,
	}

	_, err := s.RevokeAPIKey(apiKeyRevocationParams)
	if err != nil {
		logger.Error("Failed to revoke external API key", slog.Any("error", err))
		return err
	}

	logger.Info("Successfully revoked external API key")

	return nil
}

// UpdateExternalAPIKeyFromEvent updates an API key from an external event (websocket).
// This is used when platform-api broadcasts an apikey.updated event.
func (s *APIKeyService) UpdateExternalAPIKeyFromEvent(
	handle string,
	apiKeyName string,
	request *api.APIKeyCreationRequest,
	user string,
	correlationID string,
	logger *slog.Logger,
) error {
	if request == nil {
		logger.Error("nil APIKeyCreationRequest",
			slog.String("api_id", handle),
			slog.String("correlation_id", correlationID),
		)
		return fmt.Errorf("nil APIKeyCreationRequest for api %s", handle)
	}

	apiKeyUpdateParams := APIKeyUpdateParams{
		Handle:     handle,
		APIKeyName: apiKeyName,
		Request:    *request,
		User: &commonmodels.AuthContext{
			UserID: user,
		},
		Logger:        logger,
		CorrelationID: correlationID,
	}
	_, err := s.UpdateAPIKey(apiKeyUpdateParams)
	if err != nil {
		logger.Error("Failed to update external API key", slog.Any("error", err),
			slog.String("correlation_id", correlationID),
			slog.String("user_id", user),
			slog.String("api_id", handle),
		)
		return err
	}

	logger.Info("Successfully updated external API key")

	return nil
}

// computeIndexKey computes a SHA-256 hash-based index key for fast lookup
// Returns the index key as "hash_hex" (SHA-256 of the plain key)
func computeExternalKeyIndexKey(plainAPIKey string) string {
	if plainAPIKey == "" {
		return ""
	}

	hasher := sha256.New()
	hasher.Write([]byte(plainAPIKey))
	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash)
}

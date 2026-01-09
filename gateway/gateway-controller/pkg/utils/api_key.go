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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"go.uber.org/zap"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// APIKeyGenerationParams contains parameters for API key generation operations
type APIKeyGenerationParams struct {
	Handle        string                      // API handle/ID
	Request       api.APIKeyGenerationRequest // Request body with API key generation details
	User          *commonmodels.AuthContext   // User who initiated the request
	CorrelationID string                      // Correlation ID for tracking
	Logger        *zap.Logger                 // Logger instance
}

// APIKeyGenerationResult contains the result of API key generation
type APIKeyGenerationResult struct {
	Response api.APIKeyGenerationResponse // Response following the generated schema
	IsRetry  bool                         // Whether this was a retry due to collision
}

// APIKeyRevocationParams contains parameters for API key revocation operations
type APIKeyRevocationParams struct {
	Handle        string                      // API handle/ID
	Request       api.APIKeyRevocationRequest // Request body with API key revocation details
	User          *commonmodels.AuthContext   // User who initiated the request
	CorrelationID string                      // Correlation ID for tracking
	Logger        *zap.Logger                 // Logger instance
}

// APIKeyRevocationResult contains the result of API key revocation
type APIKeyRevocationResult struct {
	Response api.APIKeyRevocationResponse // Response following the generated schema
}

// APIKeyRotationParams contains parameters for API key rotation operations
type APIKeyRotationParams struct {
	Handle        string                    // API handle/ID
	APIKeyName    string                    // Name of the API key to rotate
	Request       api.APIKeyRotationRequest // Request body with rotation details
	User          *commonmodels.AuthContext // User who initiated the request
	CorrelationID string                    // Correlation ID for tracking
	Logger        *zap.Logger               // Logger instance
}

// APIKeyRotationResult contains the result of API key rotation
type APIKeyRotationResult struct {
	Response api.APIKeyGenerationResponse // Response following the generated schema
	IsRetry  bool                         // Whether this was a retry due to collision
}

// ListAPIKeyParams contains parameters for listing API keys
type ListAPIKeyParams struct {
	Handle        string                    // API handle/ID
	User          *commonmodels.AuthContext // User who initiated the request
	CorrelationID string                    // Correlation ID for tracking
	Logger        *zap.Logger               // Logger instance
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
	RevokeAPIKey(apiId, apiName, apiVersion, apiKeyID, apiKeyValue, correlationID string) error
	RemoveAPIKeysByAPI(apiId, apiName, apiVersion, correlationID string) error
}

// APIKeyService provides utilities for API configuration deployment
type APIKeyService struct {
	store         *storage.ConfigStore
	db            storage.Storage
	xdsManager    XDSManager
	hashingConfig *config.APIKeyHashingConfig // Configuration for API key hashing
}

// NewAPIKeyService creates a new API key generation service
func NewAPIKeyService(store *storage.ConfigStore, db storage.Storage, xdsManager XDSManager,
	hashingConfig *config.APIKeyHashingConfig) *APIKeyService {
	return &APIKeyService{
		store:         store,
		db:            db,
		xdsManager:    xdsManager,
		hashingConfig: hashingConfig,
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

// GenerateAPIKey handles the complete API key generation process
func (s *APIKeyService) GenerateAPIKey(params APIKeyGenerationParams) (*APIKeyGenerationResult, error) {
	logger := params.Logger
	user := params.User

	// Validate that API exists
	config, err := s.store.GetByHandle(params.Handle)
	if err != nil {
		logger.Warn("API configuration not found for API Key generation",
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("API configuration handle '%s' not found", params.Handle)
	}

	// Generate the API key from request
	apiKey, err := s.generateAPIKeyFromRequest(params.Handle, &params.Request, user.UserID, config)
	if err != nil {
		logger.Error("Failed to generate API key",
			zap.Error(err),
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	result := &APIKeyGenerationResult{
		IsRetry: false,
	}

	// Save API key to database (only if persistent mode)
	if s.db != nil {
		if err := s.db.SaveAPIKey(apiKey); err != nil {
			if errors.Is(err, storage.ErrConflict) {
				// Handle collision by retrying once with a new key
				logger.Warn("API key collision detected, retrying",
					zap.String("handle", params.Handle),
					zap.String("correlation_id", params.CorrelationID))

				// Generate a new key
				apiKey, err = s.generateAPIKeyFromRequest(params.Handle, &params.Request, user.UserID, config)
				if err != nil {
					logger.Error("Failed to regenerate API key after collision",
						zap.Error(err),
						zap.String("correlation_id", params.CorrelationID))
					return nil, fmt.Errorf("failed to regenerate API key after collision: %w", err)
				}

				// Try saving again
				if err := s.db.SaveAPIKey(apiKey); err != nil {
					logger.Error("Failed to save API key after retry",
						zap.Error(err),
						zap.String("correlation_id", params.CorrelationID))
					return nil, fmt.Errorf("failed to save API key after retry: %w", err)
				}

				result.IsRetry = true
			} else {
				logger.Error("Failed to save API key to database",
					zap.Error(err),
					zap.String("handle", params.Handle),
					zap.String("correlation_id", params.CorrelationID))
				return nil, fmt.Errorf("failed to save API key to database: %w", err)
			}
		}
	}

	// Store the generated API key in the ConfigStore
	if err := s.store.StoreAPIKey(apiKey); err != nil {
		logger.Error("Failed to store API key in ConfigStore",
			zap.Error(err),
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))

		// Rollback database save to maintain consistency
		if s.db != nil {
			if delErr := s.db.RemoveAPIKeyAPIAndName(apiKey.APIId, apiKey.Name); delErr != nil {
				logger.Error("Failed to rollback API key from database",
					zap.Error(delErr),
					zap.String("correlation_id", params.CorrelationID))
			}
		}
		return nil, fmt.Errorf("failed to store API key in ConfigStore: %w", err)
	}

	apiConfig, err := config.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		logger.Error("Failed to parse API configuration data",
			zap.Error(err),
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("failed to parse API configuration data: %w", err)
	}

	apiId := config.ID
	apiName := apiConfig.DisplayName
	apiVersion := apiConfig.Version
	logger.Info("Storing API key in policy engine",
		zap.String("handle", params.Handle),
		zap.String("name", apiKey.Name),
		zap.String("api_name", apiName),
		zap.String("api_version", apiVersion),
		zap.String("user", user.UserID),
		zap.String("correlation_id", params.CorrelationID))

	// Send the API key to the policy engine via xDS
	if s.xdsManager != nil {
		if err := s.xdsManager.StoreAPIKey(apiId, apiName, apiVersion, apiKey, params.CorrelationID); err != nil {
			logger.Error("Failed to send API key to policy engine",
				zap.Error(err),
				zap.String("correlation_id", params.CorrelationID))
			return nil, fmt.Errorf("failed to send API key to policy engine: %w", err)
		}
	}

	// Build response following the generated schema
	result.Response = s.buildAPIKeyResponse(apiKey, params.Handle)

	logger.Info("API key generated successfully",
		zap.String("handle", params.Handle),
		zap.String("name", apiKey.Name),
		zap.String("user", user.UserID),
		zap.Bool("is_retry", result.IsRetry),
		zap.String("correlation_id", params.CorrelationID))

	return result, nil
}

// RevokeAPIKey handles the API key revocation process
func (s *APIKeyService) RevokeAPIKey(params APIKeyRevocationParams) (*APIKeyRevocationResult, error) {
	logger := params.Logger
	user := params.User
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
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("API configuration handle '%s' not found", params.Handle)
	}

	providedAPIKeyValue := strings.TrimSpace(params.Request.ApiKey)
	if providedAPIKeyValue == "" {
		logger.Warn("API key value is required for revocation",
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("API key value is required for revocation")
	}

	parsedAPIkey, ok := s.parseAPIKey(providedAPIKeyValue)
	if !ok {
		// invalid format
		logger.Warn("Invalid API key format",
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return result, nil
	}

	var apiKey *models.APIKey
	var matchedKey *models.APIKey

	existingAPIKey, err := s.store.GetAPIKeyByID(parsedAPIkey.ID)
	if err != nil {
		// If memory store fails, try database
		if s.db != nil {
			existingAPIKey, err = s.db.GetAPIKeyByID(parsedAPIkey.ID)
			if err != nil {
				logger.Debug("Failed to get API keys for revocation",
					zap.Error(err),
					zap.String("handle", params.Handle),
					zap.String("correlation_id", params.CorrelationID))
				// Continue with revocation for security reasons (don't leak info)
			}
		}
	}

	// Find the API key that matches the provided plain text key
	if existingAPIKey != nil {
		if s.compareAPIKeys(parsedAPIkey.APIKey, existingAPIKey.APIKey) {
			apiKey = existingAPIKey
			matchedKey = existingAPIKey
		}
	}

	// For security reasons, perform all validations but don't return errors
	// This prevents information leakage about API key details
	if apiKey != nil {
		// Check if the API key belongs to the specified API
		if apiKey.APIId != config.ID {
			logger.Debug("API key does not belong to the specified API",
				zap.String("correlation_id", params.CorrelationID))
			return result, nil
		}

		err := s.canRevokeAPIKey(user, apiKey, logger)
		if err != nil {
			logger.Debug("User not authorized to revoke API key",
				zap.String("handle", params.Handle),
				zap.String("creator", apiKey.CreatedBy),
				zap.String("requesting_user", user.UserID),
				zap.String("correlation_id", params.CorrelationID))
			return nil, fmt.Errorf("API key revocation failed for API: '%s'", params.Handle)
		}

		// Check if the API key is already revoked
		if apiKey.Status == models.APIKeyStatusRevoked {
			logger.Debug("API key is already revoked",
				zap.String("handle", params.Handle),
				zap.String("correlation_id", params.CorrelationID))
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
					zap.Error(err),
					zap.String("handle", params.Handle),
					zap.String("correlation_id", params.CorrelationID))
				return nil, fmt.Errorf("failed to revoke API key: %w", err)
			}
		}

		// Remove the API key from memory store by name (since we have the matched key)
		if err := s.store.RemoveAPIKeyByName(config.ID, apiKey.Name); err != nil {
			logger.Error("Failed to remove API key from memory store",
				zap.Error(err),
				zap.String("handle", params.Handle),
				zap.String("correlation_id", params.CorrelationID))

			// Try to rollback database update if memory removal fails
			if s.db != nil {
				apiKey.Status = models.APIKeyStatusActive // Rollback status
				if rollbackErr := s.db.UpdateAPIKey(apiKey); rollbackErr != nil {
					logger.Error("Failed to rollback API key status in database",
						zap.Error(rollbackErr),
						zap.String("correlation_id", params.CorrelationID))
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
				zap.Error(err),
				zap.String("handle", params.Handle),
				zap.String("correlation_id", params.CorrelationID))
			// Don't return error - revocation was already successful
			// The key is marked as revoked in DB and removed from memory
		}
	}

	// remove the api key from the policy engine
	apiConfig, err := config.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		logger.Error("Failed to parse API configuration data",
			zap.Error(err),
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("failed to revoke API key: %w", err)
	}

	apiId := config.ID
	apiName := apiConfig.DisplayName
	apiVersion := apiConfig.Version
	logger.Info("Removing API key from policy engine",
		zap.String("handle", params.Handle),
		zap.String("api key", s.MaskAPIKey(providedAPIKeyValue)),
		zap.String("api_name", apiName),
		zap.String("api_version", apiVersion),
		zap.String("user", user.UserID),
		zap.String("correlation_id", params.CorrelationID))

	// Send the plain API key revocation to the policy engine via xDS
	// The policy engine will find and revoke the matching hashed key
	if s.xdsManager != nil {
		if err := s.xdsManager.RevokeAPIKey(apiId, apiName, apiVersion, parsedAPIkey.ID, parsedAPIkey.APIKey,
			params.CorrelationID); err != nil {
			logger.Error("Failed to remove API key from policy engine",
				zap.Error(err),
				zap.String("correlation_id", params.CorrelationID))
			return nil, fmt.Errorf("failed to revoke API key: %w", err)
		}
	}

	logger.Info("API key revoked successfully",
		zap.String("handle", params.Handle),
		zap.String("api key", s.MaskAPIKey(providedAPIKeyValue)),
		zap.String("user", user.UserID),
		zap.String("correlation_id", params.CorrelationID))

	return result, nil
}

// RotateAPIKey rotates an existing API key
func (s *APIKeyService) RotateAPIKey(params APIKeyRotationParams) (*APIKeyRotationResult, error) {
	logger := params.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	user := params.User

	logger.Info("Starting API key rotation",
		zap.String("handle", params.Handle),
		zap.String("api_key_name", params.APIKeyName),
		zap.String("user", user.UserID),
		zap.String("correlation_id", params.CorrelationID))

	// Get the API configuration
	config, err := s.store.GetByHandle(params.Handle)
	if err != nil {
		logger.Warn("API configuration not found for API Key rotation",
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("API configuration handle '%s' not found", params.Handle)
	}

	// Get the existing API key by name
	existingKey, err := s.store.GetAPIKeyByName(config.ID, params.APIKeyName)
	if err != nil {
		logger.Warn("API key not found for rotation",
			zap.String("handle", params.Handle),
			zap.String("api_key_name", params.APIKeyName),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("API key '%s' not found for API '%s'", params.APIKeyName, params.Handle)
	}

	err = s.canRotateAPIKey(user, existingKey, logger)
	if err != nil {
		logger.Warn("User attempting to rotate API key is not the creator",
			zap.String("handle", params.Handle),
			zap.String("api_key_name", params.APIKeyName),
			zap.String("creator", existingKey.CreatedBy),
			zap.String("requesting_user", user.UserID),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("API key rotation failed for API: '%s'", params.Handle)
	}

	// Generate the rotated API key using the extracted helper method
	rotatedKey, err := s.generateRotatedAPIKey(existingKey, params.Request, user.UserID, logger)
	if err != nil {
		logger.Error("Failed to generate rotated API key",
			zap.Error(err),
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("failed to generate rotated API key: %w", err)
	}

	result := &APIKeyRotationResult{
		IsRetry: false,
	}

	// Save rotated API key to database (only if persistent mode)
	if s.db != nil {
		if err := s.db.SaveAPIKey(rotatedKey); err != nil {
			if errors.Is(err, storage.ErrConflict) {
				// Handle collision by retrying once with a new key
				logger.Warn("API key collision detected during rotation, retrying",
					zap.String("handle", params.Handle),
					zap.String("correlation_id", params.CorrelationID))

				// Generate a new rotated key
				rotatedKey, err = s.generateRotatedAPIKey(existingKey, params.Request, user.UserID, logger)
				if err != nil {
					logger.Error("Failed to regenerate rotated API key after collision",
						zap.Error(err),
						zap.String("correlation_id", params.CorrelationID))
					return nil, fmt.Errorf("failed to regenerate rotated API key after collision: %w", err)
				}

				// Try saving again
				if err := s.db.SaveAPIKey(rotatedKey); err != nil {
					logger.Error("Failed to save rotated API key after retry",
						zap.Error(err),
						zap.String("correlation_id", params.CorrelationID))
					return nil, fmt.Errorf("failed to save rotated API key after retry: %w", err)
				}

				result.IsRetry = true
			} else {
				logger.Error("Failed to save rotated API key to database",
					zap.Error(err),
					zap.String("handle", params.Handle),
					zap.String("correlation_id", params.CorrelationID))
				return nil, fmt.Errorf("failed to save rotated API key to database: %w", err)
			}
		}
		// No need to revoke the old API key as the old one will be overwritten
	}

	// Store the generated API key in the ConfigStore
	if err := s.store.StoreAPIKey(rotatedKey); err != nil {
		logger.Error("Failed to store the rotated API key in ConfigStore",
			zap.Error(err),
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))

		// Rollback database save to maintain consistency
		if s.db != nil {
			if delErr := s.db.RemoveAPIKeyAPIAndName(rotatedKey.APIId, rotatedKey.Name); delErr != nil {
				logger.Error("Failed to rollback API key from database",
					zap.Error(delErr),
					zap.String("correlation_id", params.CorrelationID))
			}
		}
		return nil, fmt.Errorf("failed to store API key in ConfigStore: %w", err)
	}

	apiConfig, err := config.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		logger.Error("Failed to parse API configuration data",
			zap.Error(err),
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("failed to parse API configuration data: %w", err)
	}

	apiId := config.ID
	apiName := apiConfig.DisplayName
	apiVersion := apiConfig.Version
	logger.Info("Storing API key in policy engine",
		zap.String("handle", params.Handle),
		zap.String("name", rotatedKey.Name),
		zap.String("api_name", apiName),
		zap.String("api_version", apiVersion),
		zap.String("user", user.UserID),
		zap.String("correlation_id", params.CorrelationID))

	// Update xDS snapshot if needed
	if s.xdsManager != nil {
		if err := s.xdsManager.StoreAPIKey(apiId, apiName, apiVersion, rotatedKey, params.CorrelationID); err != nil {
			logger.Error("Failed to send rotated API key to policy engine",
				zap.Error(err),
				zap.String("correlation_id", params.CorrelationID))
			return nil, fmt.Errorf("failed to send rotated API key to policy engine: %w", err)
		}
	}

	// Build and return the response
	result.Response = s.buildAPIKeyResponse(rotatedKey, params.Handle)

	logger.Info("API key rotation completed successfully",
		zap.String("handle", params.Handle),
		zap.String("api_key_name", params.APIKeyName),
		zap.String("new_key_id", rotatedKey.ID),
		zap.String("correlation_id", params.CorrelationID))

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
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("API configuration handle '%s' not found", params.Handle)
	}

	// Get all API keys for this API from memory store first
	var apiKeys []*models.APIKey

	// Try to get from memory store
	memoryKeys, err := s.store.GetAPIKeysByAPI(config.ID)
	if err != nil {
		logger.Debug("Failed to get API keys from memory store, trying database",
			zap.Error(err),
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))

		// If memory store fails, try database
		if s.db != nil {
			dbKeys, dbErr := s.db.GetAPIKeysByAPI(config.ID)
			if dbErr != nil {
				logger.Error("Failed to get API keys from database",
					zap.Error(dbErr),
					zap.String("handle", params.Handle),
					zap.String("correlation_id", params.CorrelationID))
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
			zap.Error(err),
			zap.String("handle", params.Handle),
			zap.String("user", user.UserID),
			zap.String("correlation_id", params.CorrelationID))
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
		// API key is not returned for security reasons
		responseAPIKey := api.APIKey{
			Name:       key.Name,
			ApiId:      params.Handle, // Use handle instead of internal API ID
			Operations: key.Operations,
			Status:     api.APIKeyStatus(key.Status),
			CreatedAt:  key.CreatedAt,
			CreatedBy:  key.CreatedBy,
			ExpiresAt:  key.ExpiresAt,
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
		zap.String("handle", params.Handle),
		zap.String("user", user.UserID),
		zap.Int("total_count", totalCount),
		zap.String("correlation_id", params.CorrelationID))

	return result, nil
}

// generateAPIKeyFromRequest creates a new API key based on the APIKeyGenerationRequest
func (s *APIKeyService) generateAPIKeyFromRequest(handle string, request *api.APIKeyGenerationRequest, user string,
	config *models.StoredConfig) (*models.APIKey, error) {

	// Generate UUID for the record ID
	id := uuid.New().String()

	// Generate 32 random bytes for the API key
	plainAPIKeyValue, err := s.generateAPIKeyValue()
	if err != nil {
		return nil, err
	}

	// Hash the API key for storage and policy engine
	hashedAPIKeyValue, err := s.hashAPIKey(plainAPIKeyValue)
	if err != nil {
		return nil, fmt.Errorf("failed to hash API key: %w", err)
	}

	// Set name - use provided name or generate a default one
	name := fmt.Sprintf("%s-key-%s", handle, id[:8]) // Default name
	if request.Name != nil && strings.TrimSpace(*request.Name) != "" {
		name = strings.TrimSpace(*request.Name)
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
		case api.APIKeyGenerationRequestExpiresInUnitSeconds:
			timeDuration *= time.Second
		case api.APIKeyGenerationRequestExpiresInUnitMinutes:
			timeDuration *= time.Minute
		case api.APIKeyGenerationRequestExpiresInUnitHours:
			timeDuration *= time.Hour
		case api.APIKeyGenerationRequestExpiresInUnitDays:
			timeDuration *= 24 * time.Hour
		case api.APIKeyGenerationRequestExpiresInUnitWeeks:
			timeDuration *= 7 * 24 * time.Hour
		case api.APIKeyGenerationRequestExpiresInUnitMonths:
			timeDuration *= 30 * 24 * time.Hour // Approximate month as 30 days
		default:
			return nil, fmt.Errorf("unsupported expiration unit: %s", request.ExpiresIn.Unit)
		}
		expiry := now.Add(timeDuration)
		expiresAt = &expiry
	}

	apiKey := &models.APIKey{
		ID:         id,
		Name:       name,
		APIKey:     hashedAPIKeyValue, // Store hashed key in database and policy engine
		APIId:      config.ID,
		Operations: operations,
		Status:     models.APIKeyStatusActive,
		CreatedAt:  now,
		CreatedBy:  user,
		UpdatedAt:  now,
		ExpiresAt:  expiresAt,
		Unit:       unit,
		Duration:   duration,
	}

	// Temporarily store the plain key for response generation
	// This field is not persisted and only used for returning to user
	apiKey.PlainAPIKey = plainAPIKeyValue

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
func (s *APIKeyService) buildAPIKeyResponse(key *models.APIKey, handle string) api.APIKeyGenerationResponse {
	if key == nil {
		return api.APIKeyGenerationResponse{
			Status:  "error",
			Message: "API key is nil",
		}
	}

	// Use PlainAPIKey for response if available, otherwise mask the hashed key
	var responseAPIKey *string
	if key.PlainAPIKey != "" {
		// Format: apip_{key}.{hex_encoded_id}
		hexEncodedID := hex.EncodeToString([]byte(key.ID))
		formattedAPIKey := key.PlainAPIKey + constants.APIKeySeparator + hexEncodedID
		responseAPIKey = &formattedAPIKey
	} else {
		// For existing keys where PlainAPIKey is not available, don't return the hashed key
		responseAPIKey = nil
	}

	return api.APIKeyGenerationResponse{
		Status:  "success",
		Message: "API key generated successfully",
		ApiKey: &api.APIKey{
			Name:       key.Name,
			ApiKey:     responseAPIKey, // Return plain key only during generation/rotation
			ApiId:      handle,
			Operations: key.Operations,
			Status:     api.APIKeyStatus(key.Status),
			CreatedAt:  key.CreatedAt,
			CreatedBy:  key.CreatedBy,
			ExpiresAt:  key.ExpiresAt,
		},
	}
}

// generateRotatedAPIKey creates a new API key for rotation based on existing key and request parameters
func (s *APIKeyService) generateRotatedAPIKey(existingKey *models.APIKey, request api.APIKeyRotationRequest,
	user string, logger *zap.Logger) (*models.APIKey, error) {
	// Generate new API key value
	plainAPIKeyValue, err := s.generateAPIKeyValue()
	if err != nil {
		return nil, err
	}

	// Hash the new API key for storage
	hashedAPIKeyValue, err := s.hashAPIKey(plainAPIKeyValue)
	if err != nil {
		return nil, fmt.Errorf("failed to hash rotated API key: %w", err)
	}

	now := time.Now()

	// Determine expiration settings based on request and existing key
	var expiresAt *time.Time
	var unit *string
	var duration *int

	if request.ExpiresAt != nil {
		// If expires_at is explicitly provided, use it
		expiresAt = request.ExpiresAt
		logger.Info("Using provided expires_at for rotation", zap.Time("expires_at", *expiresAt))
	} else if request.ExpiresIn != nil {
		// If expires_in is provided, calculate expires_at from now
		unitStr := string(request.ExpiresIn.Unit)
		unit = &unitStr
		duration = &request.ExpiresIn.Duration

		timeDuration := time.Duration(request.ExpiresIn.Duration)
		switch request.ExpiresIn.Unit {
		case api.APIKeyRotationRequestExpiresInUnitSeconds:
			timeDuration *= time.Second
		case api.APIKeyRotationRequestExpiresInUnitMinutes:
			timeDuration *= time.Minute
		case api.APIKeyRotationRequestExpiresInUnitHours:
			timeDuration *= time.Hour
		case api.APIKeyRotationRequestExpiresInUnitDays:
			timeDuration *= 24 * time.Hour
		case api.APIKeyRotationRequestExpiresInUnitWeeks:
			timeDuration *= 7 * 24 * time.Hour
		case api.APIKeyRotationRequestExpiresInUnitMonths:
			timeDuration *= 30 * 24 * time.Hour
		default:
			return nil, fmt.Errorf("unsupported expiration unit: %s", request.ExpiresIn.Unit)
		}
		expiry := now.Add(timeDuration)
		expiresAt = &expiry
		logger.Info("Using provided expires_in for rotation",
			zap.String("unit", unitStr),
			zap.Int("duration", *duration),
			zap.Time("calculated_expires_at", *expiresAt))
	} else {
		// No expiration provided in request, use existing key's logic
		if existingKey.Unit != nil && existingKey.Duration != nil {
			// Existing key has duration/unit, apply same duration from now
			unit = existingKey.Unit
			duration = existingKey.Duration

			timeDuration := time.Duration(*existingKey.Duration)
			switch *existingKey.Unit {
			case string(api.APIKeyRotationRequestExpiresInUnitSeconds):
				timeDuration *= time.Second
			case string(api.APIKeyRotationRequestExpiresInUnitMinutes):
				timeDuration *= time.Minute
			case string(api.APIKeyRotationRequestExpiresInUnitHours):
				timeDuration *= time.Hour
			case string(api.APIKeyRotationRequestExpiresInUnitDays):
				timeDuration *= 24 * time.Hour
			case string(api.APIKeyRotationRequestExpiresInUnitWeeks):
				timeDuration *= 7 * 24 * time.Hour
			case string(api.APIKeyRotationRequestExpiresInUnitMonths):
				timeDuration *= 30 * 24 * time.Hour
			default:
				return nil, fmt.Errorf("unsupported existing expiration unit: %s", *existingKey.Unit)
			}
			expiry := now.Add(timeDuration)
			expiresAt = &expiry
			logger.Info("Using existing key's duration settings for rotation",
				zap.String("unit", *unit),
				zap.Int("duration", *duration),
				zap.Time("calculated_expires_at", *expiresAt))
		} else if existingKey.ExpiresAt != nil {
			// Existing key has absolute expiry, use same expiry
			expiresAt = existingKey.ExpiresAt
			logger.Info("Using existing key's expires_at for rotation", zap.Time("expires_at", *expiresAt))
		} else {
			// Existing key has no expiry, new key also has no expiry
			expiresAt = nil
			logger.Info("No expiry set for rotated key (matching existing key)")
		}
	}

	// Create the rotated API key
	rotatedKey := &models.APIKey{
		ID:         existingKey.ID,
		Name:       existingKey.Name,
		APIKey:     hashedAPIKeyValue, // Store hashed key
		APIId:      existingKey.APIId,
		Operations: existingKey.Operations,
		Status:     models.APIKeyStatusActive,
		CreatedAt:  existingKey.CreatedAt,
		CreatedBy:  existingKey.CreatedBy,
		UpdatedAt:  now,
		ExpiresAt:  expiresAt,
		Unit:       unit,
		Duration:   duration,
	}

	// Temporarily store the plain key for response generation
	rotatedKey.PlainAPIKey = plainAPIKeyValue

	return rotatedKey, nil
}

// canRevokeAPIKey determines if a user can revoke a specific API key
// Admin role can revoke any API key of an API. Other users can only revoke API keys that they created.
func (s *APIKeyService) canRevokeAPIKey(user *commonmodels.AuthContext, apiKey *models.APIKey, logger *zap.Logger) error {
	if user == nil {
		return fmt.Errorf("user authentication required")
	}

	if apiKey == nil {
		return fmt.Errorf("API key not found")
	}

	logger.Debug("Checking API key revocation authorization",
		zap.String("user_id", user.UserID),
		zap.Strings("roles", user.Roles),
		zap.String("api_key_name", apiKey.Name),
		zap.String("api_key_creator", apiKey.CreatedBy))

	// Admin role can revoke any API key
	if s.isAdmin(user) {
		logger.Debug("User has admin role, authorized to revoke any API key",
			zap.String("user_id", user.UserID),
			zap.String("api_key_name", apiKey.Name))
		return nil
	}

	// Non-admin users can only revoke keys they created
	if apiKey.CreatedBy != user.UserID {
		logger.Warn("User cannot revoke API key - not the creator and not admin",
			zap.String("user_id", user.UserID),
			zap.String("api_key_name", apiKey.Name),
			zap.String("api_key_creator", apiKey.CreatedBy))
		return fmt.Errorf("API key revocation not authorized for user")
	}

	logger.Debug("User authorized to revoke API key as creator",
		zap.String("user_id", user.UserID),
		zap.String("api_key_name", apiKey.Name))

	return nil
}

// canRotateAPIKey determines if a user can rotate a specific API key
// Only the user who created the API key can rotate it
func (s *APIKeyService) canRotateAPIKey(user *commonmodels.AuthContext, apiKey *models.APIKey, logger *zap.Logger) error {
	if user == nil {
		return fmt.Errorf("user authentication required")
	}

	if apiKey == nil {
		return fmt.Errorf("API key not found")
	}

	logger.Debug("Checking API key rotation authorization",
		zap.String("user_id", user.UserID),
		zap.Strings("roles", user.Roles),
		zap.String("api_key_name", apiKey.Name),
		zap.String("api_key_creator", apiKey.CreatedBy))

	// Only the creator can rotate the API key
	if apiKey.CreatedBy != user.UserID {
		logger.Warn("User cannot rotate API key - not the creator",
			zap.String("user_id", user.UserID),
			zap.String("api_key_name", apiKey.Name),
			zap.String("api_key_creator", apiKey.CreatedBy))
		return fmt.Errorf("only the creator of the API key can rotate it")
	}

	logger.Debug("User authorized to rotate API key",
		zap.String("user_id", user.UserID),
		zap.String("api_key_name", apiKey.Name))

	return nil
}

// filterAPIKeysByUser filters a list of API keys based on the user's roles
// Admin role can list all keys of an API. Other users can view only API keys that they created.
func (s *APIKeyService) filterAPIKeysByUser(user *commonmodels.AuthContext, apiKeys []*models.APIKey,
	logger *zap.Logger) ([]*models.APIKey, error) {
	if user == nil {
		return nil, fmt.Errorf("user authentication required")
	}

	logger.Debug("Checking API key list authorization",
		zap.String("user_id", user.UserID),
		zap.Strings("roles", user.Roles),
		zap.Int("total_keys", len(apiKeys)))

	// Admin role can see all API keys
	if s.isAdmin(user) {
		logger.Debug("User has admin role, returning all API keys",
			zap.String("user_id", user.UserID),
			zap.Int("returned_keys", len(apiKeys)))
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
		zap.String("user_id", user.UserID),
		zap.Int("owned_keys", len(userAPIKeys)),
		zap.Int("total_keys", len(apiKeys)))

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

// MaskAPIKey masks an API key for secure logging, showing first 8 and last 4 characters
func (s *APIKeyService) MaskAPIKey(apiKey string) string {
	if len(apiKey) <= 12 {
		return "****"
	}
	return apiKey[:8] + "****" + apiKey[len(apiKey)-4:]
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

	// If hashing is disabled, return the plain API key
	if !s.hashingConfig.Enabled {
		return plainAPIKey, nil
	}

	// Hash based on configured algorithm
	switch strings.ToLower(s.hashingConfig.Algorithm) {
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
// Expected format: $argon2id$v=19$m=65536,t=3,p=4$<salt_b64>$<hash_b64>
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

	// parts[3] -> m=65536,t=3,p=4
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

// parseAPIKey splits an API key value into its key and ID components
func (s *APIKeyService) parseAPIKey(value string) (ParsedAPIKey, bool) {
	idx := strings.LastIndex(value, ".")
	if idx <= 0 || idx == len(value)-1 {
		return ParsedAPIKey{}, false
	}

	apiKey := value[:idx]
	hexEncodedID := value[idx+1:]

	// Decode the hex encoded ID back to the raw ID
	decodedIDBytes, err := hex.DecodeString(hexEncodedID)
	if err != nil {
		// If decoding fails, return the hex value as-is for backward compatibility
		return ParsedAPIKey{
			APIKey: apiKey,
			ID:     hexEncodedID,
		}, true
	}

	return ParsedAPIKey{
		APIKey: apiKey,
		ID:     string(decodedIDBytes),
	}, true
}

// SetHashingConfig allows updating the hashing configuration at runtime
func (s *APIKeyService) SetHashingConfig(config *config.APIKeyHashingConfig) {
	s.hashingConfig = config
}

// GetHashingConfig returns the current hashing configuration
func (s *APIKeyService) GetHashingConfig() *config.APIKeyHashingConfig {
	return s.hashingConfig
}

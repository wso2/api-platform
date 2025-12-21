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
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"go.uber.org/zap"
)

// APIKeyGenerationParams contains parameters for API key generation operations
type APIKeyGenerationParams struct {
	Handle        string                      // API handle/ID
	Request       api.APIKeyGenerationRequest // Request body with API key generation details
	User          string                      // User who initiated the request
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
	Handle        string      // API handle/ID
	APIKey        string      // APi key to be revoked
	User          string      // User who initiated the request
	CorrelationID string      // Correlation ID for tracking
	Logger        *zap.Logger // Logger instance
}

// APIKeyRotationParams contains parameters for API key rotation operations
type APIKeyRotationParams struct {
	Handle        string                    // API handle/ID
	APIKeyName    string                    // Name of the API key to rotate
	Request       api.APIKeyRotationRequest // Request body with rotation details
	User          string                    // User who initiated the request
	CorrelationID string                    // Correlation ID for tracking
	Logger        *zap.Logger               // Logger instance
}

// APIKeyRotationResult contains the result of API key rotation
type APIKeyRotationResult struct {
	Response api.APIKeyGenerationResponse // Response following the generated schema
	IsRetry  bool                         // Whether this was a retry due to collision
}

// APIKeyService provides utilities for API configuration deployment
type APIKeyService struct {
	store      *storage.ConfigStore
	db         storage.Storage
	xdsManager XDSManager
}

// XDSManager interface for API key operations
type XDSManager interface {
	StoreAPIKey(apiId, apiName, apiVersion string, apiKey *models.APIKey, correlationID string) error
	RevokeAPIKey(apiId, apiName, apiVersion, apiKeyValue, correlationID string) error
	RemoveAPIKeysByAPI(apiId, apiName, apiVersion, correlationID string) error
}

// NewAPIKeyService creates a new API key generation service
func NewAPIKeyService(store *storage.ConfigStore, db storage.Storage, xdsManager XDSManager) *APIKeyService {
	return &APIKeyService{
		store:      store,
		db:         db,
		xdsManager: xdsManager,
	}
}

const APIKeyPrefix = "apip_"

// GenerateAPIKey handles the complete API key generation process
func (s *APIKeyService) GenerateAPIKey(params APIKeyGenerationParams) (*APIKeyGenerationResult, error) {
	logger := params.Logger

	// Validate that API exists
	config, err := s.store.GetByHandle(params.Handle)
	if err != nil {
		logger.Warn("API configuration not found for API Key generation",
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("API configuration handle '%s' not found", params.Handle)
	}

	// Generate the API key from request
	apiKey, err := s.generateAPIKeyFromRequest(params.Handle, &params.Request, params.User, config)
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
				apiKey, err = s.generateAPIKeyFromRequest(params.Handle, &params.Request, params.User, config)
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
		zap.String("user", params.User),
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
		zap.String("user", params.User),
		zap.Bool("is_retry", result.IsRetry),
		zap.String("correlation_id", params.CorrelationID))

	return result, nil
}

// RevokeAPIKey handles the API key revocation process
func (s *APIKeyService) RevokeAPIKey(params APIKeyRevocationParams) error {
	logger := params.Logger

	// Validate that API exists
	config, err := s.store.GetByHandle(params.Handle)
	if err != nil {
		logger.Warn("API configuration not found for API key revocation",
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		return fmt.Errorf("API configuration handle '%s' not found", params.Handle)
	}

	// Get the API key by its value
	apiKey, err := s.store.GetAPIKeyByKey(params.APIKey)
	if err != nil {
		logger.Debug("API key not found for revocation",
			zap.String("handle", params.Handle),
			zap.String("correlation_id", params.CorrelationID))
		//return nil
	}

	// For security reasons, perform all validations but don't return errors
	// This prevents information leakage about API key details
	if apiKey != nil {
		// Check if the API key belongs to the specified API
		if apiKey.APIId != config.ID {
			logger.Debug("API key does not belong to the specified API",
				zap.String("correlation_id", params.CorrelationID))
			return fmt.Errorf("API key revocation failed for API: '%s'", params.Handle)
		}

		// Check if the user revoking the key is the same as the one who created it
		if apiKey.CreatedBy != params.User {
			logger.Debug("User attempting to revoke API key is not the creator",
				zap.String("handle", params.Handle),
				zap.String("creator", apiKey.CreatedBy),
				zap.String("requesting_user", params.User),
				zap.String("correlation_id", params.CorrelationID))
			return fmt.Errorf("API key revocation failed for API: '%s'", params.Handle)
		}

		// Check if the API key is already revoked
		if apiKey.Status == models.APIKeyStatusRevoked {
			logger.Debug("API key is already revoked",
				zap.String("handle", params.Handle),
				zap.String("correlation_id", params.CorrelationID))
			return nil
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
				return fmt.Errorf("failed to revoke API key: %w", err)
			}
		}

		// Remove the API key from memory store
		if err := s.store.RemoveAPIKey(params.APIKey); err != nil {
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
			return fmt.Errorf("failed to revoke API key: %w", err)
		}
	}

	// Remove the API key from database (complete removal)
	// Note: This is cleanup only - the revocation is already complete
	if s.db != nil {
		if err := s.db.DeleteAPIKey(params.APIKey); err != nil {
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
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	apiId := config.ID
	apiName := apiConfig.DisplayName
	apiVersion := apiConfig.Version
	logger.Info("Removing API key from policy engine",
		zap.String("handle", params.Handle),
		zap.String("api key", s.maskAPIKey(params.APIKey)),
		zap.String("api_name", apiName),
		zap.String("api_version", apiVersion),
		zap.String("user", params.User),
		zap.String("correlation_id", params.CorrelationID))

	// Send the API key revocation to the policy engine via xDS
	if s.xdsManager != nil {
		if err := s.xdsManager.RevokeAPIKey(apiId, apiName, apiVersion, params.APIKey, params.CorrelationID); err != nil {
			logger.Error("Failed to remove API key from policy engine",
				zap.Error(err),
				zap.String("correlation_id", params.CorrelationID))
			return fmt.Errorf("failed to revoke API key: %w", err)
		}
	}

	logger.Info("API key revoked successfully",
		zap.String("handle", params.Handle),
		zap.String("api key", s.maskAPIKey(params.APIKey)),
		zap.String("user", params.User),
		zap.String("correlation_id", params.CorrelationID))

	return nil
}

// RotateAPIKey rotates an existing API key
func (s *APIKeyService) RotateAPIKey(params APIKeyRotationParams) (*APIKeyRotationResult, error) {
	logger := params.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	logger.Info("Starting API key rotation",
		zap.String("handle", params.Handle),
		zap.String("api_key_name", params.APIKeyName),
		zap.String("user", params.User),
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

	// Generate the rotated API key using the extracted helper method
	rotatedKey, err := s.generateRotatedAPIKey(existingKey, params.Request, params.User, logger)
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
				rotatedKey, err = s.generateRotatedAPIKey(existingKey, params.Request, params.User, logger)
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
		zap.String("user", params.User),
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

// generateAPIKeyFromRequest creates a new API key based on the APIKeyGenerationRequest
func (s *APIKeyService) generateAPIKeyFromRequest(handle string, request *api.APIKeyGenerationRequest, user string,
	config *models.StoredConfig) (*models.APIKey, error) {

	// Generate UUID for the record ID
	id := uuid.New().String()

	// Generate 32 random bytes for the API key
	apiKeyValue, err := s.generateAPIKeyValue()
	if err != nil {
		return nil, err
	}

	// Set name - use provided name or generate a default one
	name := fmt.Sprintf("%s-key-%s", handle, id[:8]) // Default name
	if request.Name != nil && strings.TrimSpace(*request.Name) != "" {
		name = strings.TrimSpace(*request.Name)
	}

	// Process operations
	operations := "[*]" // Default to all operations
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

	if user == "" {
		user = "system"
	}

	return &models.APIKey{
		ID:         id,
		Name:       name,
		APIKey:     apiKeyValue,
		APIId:      config.ID,
		Operations: operations,
		Status:     models.APIKeyStatusActive,
		CreatedAt:  now,
		CreatedBy:  user,
		UpdatedAt:  now,
		ExpiresAt:  expiresAt,
		Unit:       unit,
		Duration:   duration,
	}, nil
}

// generateOperationsString creates a string array from operations in format "METHOD path"
// Example: ["GET /{country_code}/{city}", "POST /data"]
// Ignores the policies field from operations
func (s *APIKeyService) generateOperationsString(operations []api.Operation) string {
	if len(operations) == 0 {
		return "[*]" // Default to all operations if none specified
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
		return "[*]"
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

	return api.APIKeyGenerationResponse{
		Status:  "success",
		Message: "API key generated successfully",
		ApiKey: &api.APIKey{
			Name:       key.Name,
			ApiKey:     key.APIKey,
			ApiId:      handle,
			Operations: key.Operations,
			Status:     api.APIKeyStatus(key.Status),
			CreatedAt:  key.CreatedAt,
			CreatedBy:  key.CreatedBy,
			ExpiresAt:  key.ExpiresAt,
		},
	}
}

// maskAPIKey masks an API key for secure logging, showing first 8 and last 4 characters
func (s *APIKeyService) maskAPIKey(apiKey string) string {
	if len(apiKey) <= 12 {
		return "****"
	}
	return apiKey[:8] + "****" + apiKey[len(apiKey)-4:]
}

// generateAPIKeyValue generates a new API key value with collision handling
func (s *APIKeyService) generateAPIKeyValue() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return APIKeyPrefix + hex.EncodeToString(randomBytes), nil
}

// generateRotatedAPIKey creates a new API key for rotation based on existing key and request parameters
func (s *APIKeyService) generateRotatedAPIKey(existingKey *models.APIKey, request api.APIKeyRotationRequest,
	user string, logger *zap.Logger) (*models.APIKey, error) {
	// Generate new API key value
	newAPIKeyValue, err := s.generateAPIKeyValue()
	if err != nil {
		return nil, err
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

	if user == "" {
		user = "system"
	}

	// Create the rotated API key
	return &models.APIKey{
		ID:         uuid.New().String(),
		Name:       existingKey.Name,
		APIKey:     newAPIKeyValue,
		APIId:      existingKey.APIId,
		Operations: existingKey.Operations,
		Status:     models.APIKeyStatusActive,
		CreatedAt:  now,
		CreatedBy:  user,
		UpdatedAt:  now,
		ExpiresAt:  expiresAt,
		Unit:       unit,
		Duration:   duration,
	}, nil
}

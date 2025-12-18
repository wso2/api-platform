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
	CorrelationID string                      // Correlation ID for tracking
	Logger        *zap.Logger                 // Logger instance
}

// APIKeyGenerationResult contains the result of API key generation
type APIKeyGenerationResult struct {
	Response api.APIKeyGenerationResponse // Response following the generated schema
	IsRetry  bool                         // Whether this was a retry due to collision
}

// APIKeyService provides utilities for API configuration deployment
type APIKeyService struct {
	store *storage.ConfigStore
	db    storage.Storage
}

// NewAPIKeyService creates a new API key generation service
func NewAPIKeyService(store *storage.ConfigStore, db storage.Storage) *APIKeyService {
	return &APIKeyService{
		store: store,
		db:    db,
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
	apiKey, err := s.generateAPIKeyFromRequest(params.Handle, &params.Request, config)
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
				apiKey, err = s.generateAPIKeyFromRequest(params.Handle, &params.Request, config)
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
			if delErr := s.db.RemoveAPIKeyAPIAndName(apiKey.Handle, apiKey.Name); delErr != nil {
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

	apiName := apiConfig.DisplayName
	apiVersion := apiConfig.Version
	logger.Info("Storing API key in policy engine",
		zap.String("handle", params.Handle),
		zap.String("name", apiKey.Name),
		zap.String("api_name", apiName),
		zap.String("api_version", apiVersion),
		zap.String("correlation_id", params.CorrelationID))
	// TODO - Send the API key to the policy engine
	// StoreAPIKey(apiName, apiVersion string, apiKey *APIKey)

	// Build response following the generated schema
	result.Response = s.buildAPIKeyResponse(apiKey)

	logger.Info("API key generated successfully",
		zap.String("handle", params.Handle),
		zap.String("name", apiKey.Name),
		zap.Bool("is_retry", result.IsRetry),
		zap.String("correlation_id", params.CorrelationID))

	return result, nil
}

// generateAPIKeyFromRequest creates a new API key based on the APIKeyGenerationRequest
func (s *APIKeyService) generateAPIKeyFromRequest(handle string, request *api.APIKeyGenerationRequest, config *models.StoredConfig) (*models.APIKey, error) {
	// Generate UUID for the record ID
	id := uuid.New().String()

	// Generate 32 random bytes for the API key
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode to hex and prefix
	apiKeyValue := APIKeyPrefix + hex.EncodeToString(randomBytes)

	// Set name - use provided name or generate a default one
	name := fmt.Sprintf("%s-key-%s", handle, id[:8]) // Default name
	if request.Name != nil && strings.TrimSpace(*request.Name) != "" {
		name = strings.TrimSpace(*request.Name)
	}

	// Process operations
	operations := "[*]" // Default to all operations
	if request.Operations != nil && len(*request.Operations) > 0 {
		operations = s.generateOperationsString(*request.Operations)
	}

	now := time.Now()

	// Calculate expiration time
	var expiresAt *time.Time
	if request.ExpiresAt != nil {
		expiresAt = request.ExpiresAt
	} else if request.ExpiresIn != nil {
		duration := time.Duration(request.ExpiresIn.Duration)
		switch request.ExpiresIn.Unit {
		case api.Seconds:
			duration *= time.Second
		case api.Minutes:
			duration *= time.Minute
		case api.Hours:
			duration *= time.Hour
		case api.Days:
			duration *= 24 * time.Hour
		case api.Weeks:
			duration *= 7 * 24 * time.Hour
		case api.Months:
			duration *= 30 * 24 * time.Hour // Approximate month as 30 days
		default:
			return nil, fmt.Errorf("unsupported expiration unit: %s", request.ExpiresIn.Unit)
		}
		expiry := now.Add(duration)
		expiresAt = &expiry
	}

	// TODO - Add created_by field once user management is implemented. this is the user who generated the api key

	return &models.APIKey{
		ID:         id,
		Name:       name,
		APIKey:     apiKeyValue,
		Handle:     handle,
		Operations: operations,
		Status:     models.APIKeyStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
		ExpiresAt:  expiresAt,
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
func (s *APIKeyService) buildAPIKeyResponse(key *models.APIKey) api.APIKeyGenerationResponse {
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
			ApiId:      key.Handle,
			Operations: key.Operations,
			Status:     api.APIKeyStatus(key.Status),
			CreatedAt:  key.CreatedAt,
			ExpiresAt:  key.ExpiresAt,
		},
	}
}

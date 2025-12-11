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

package services

import (
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"go.uber.org/zap"
)

// APIKeyValidator handles API key validation across different services based on key prefix
type APIKeyValidator struct {
	store  *storage.ConfigStore
	db     storage.Storage
	logger *zap.Logger
}

// NewAPIKeyValidator creates a new API key validator instance
func NewAPIKeyValidator(
	store *storage.ConfigStore,
	db storage.Storage,
	logger *zap.Logger,
) *APIKeyValidator {
	return &APIKeyValidator{
		store:  store,
		db:     db,
		logger: logger,
	}
}

// ValidateAPIKey validates an API key for the specified API name and version
// Routes validation to appropriate service based on API key prefix
func (v *APIKeyValidator) ValidateAPIKey(apiName, apiVersion, apiKey string) (bool, error) {
	// Extract prefix from API key
	prefix := v.extractAPIKeyPrefix(apiKey)

	v.logger.Debug("Validating API key",
		zap.String("apiName", apiName),
		zap.String("apiVersion", apiVersion),
		zap.String("prefix", prefix),
	)

	switch prefix {
	case "gw":
		return v.validateGatewayAPIKey(apiName, apiVersion, apiKey)
	case "mgt":
		return v.validateManagementPortalAPIKey(apiName, apiVersion, apiKey)
	case "dev":
		return v.validateDevPortalAPIKey(apiName, apiVersion, apiKey)
	default:
		v.logger.Warn("Unknown API key prefix", zap.String("prefix", prefix))
		return false, nil
	}
}

// extractAPIKeyPrefix extracts the prefix from an API key (everything before the first underscore)
func (v *APIKeyValidator) extractAPIKeyPrefix(apiKey string) string {
	parts := strings.SplitN(apiKey, "_", 2)
	if len(parts) >= 2 {
		return strings.ToLower(parts[0])
	}
	return ""
}

// validateGatewayAPIKey validates API keys with "gw_" prefix against the gateway controller database
func (v *APIKeyValidator) validateGatewayAPIKey(apiName, apiVersion, apiKey string) (bool, error) {
	v.logger.Debug("Validating gateway API key",
		zap.String("apiName", apiName),
		zap.String("apiVersion", apiVersion),
	)

	// Look up the API key
	var storedAPIKey *models.APIKey
	var err error
	storedAPIKey, err = v.store.GetAPIKeyByKey(apiKey)
	if err != nil {
		v.logger.Debug("API key not found in memory",
			zap.String("apiName", apiName),
			zap.String("apiVersion", apiVersion),
			zap.Error(err))
		if v.db != nil {
			// Fallback to persistent storage
			storedAPIKey, err = v.db.GetAPIKeyByKey(apiKey)
			if err != nil {
				v.logger.Debug("API key not found in persistent storage",
					zap.String("apiName", apiName),
					zap.String("apiVersion", apiVersion),
					zap.Error(err))
				return false, nil // API key doesn't exist
			}
		} else {
			return false, nil // API key doesn't exist
		}
	}

	if storedAPIKey == nil {
		return false, nil // API key doesn't exist
	}

	// Verify that the API key is associated with the correct API
	if storedAPIKey.APIName != apiName || storedAPIKey.APIVersion != apiVersion {
		return false, nil
	}

	// Check if the API key is valid (active and not expired)
	isValid := storedAPIKey.IsValid()

	v.logger.Debug("Gateway API key validation result",
		zap.Bool("isValid", isValid),
		zap.String("keyStatus", string(storedAPIKey.Status)),
		zap.Time("expiresAt", func() time.Time {
			if storedAPIKey.ExpiresAt != nil {
				return *storedAPIKey.ExpiresAt
			}
			return time.Time{}
		}()))

	return isValid, nil
}

// validateManagementPortalAPIKey validates API keys with "mgt_" prefix against the management portal
func (v *APIKeyValidator) validateManagementPortalAPIKey(apiName, apiVersion, apiKey string) (bool, error) {
	v.logger.Debug("Validating management portal API key",
		zap.String("apiName", apiName),
		zap.String("apiVersion", apiVersion),
	)

	// TODO: Implement management portal API key validation
	// This should make an HTTP request to the management portal's API key validation endpoint
	// Example implementation:
	// 1. Create HTTP client
	// 2. Make POST request to management portal: POST /api/v1/validate-key
	// 3. Send payload: {"apiName": apiName, "apiVersion": apiVersion, "apiKey": apiKey}
	// 4. Parse response and return validation result

	v.logger.Warn("Management portal API key validation not yet implemented")
	return false, nil
}

// validateDevPortalAPIKey validates API keys with "dev_" prefix against the developer portal
func (v *APIKeyValidator) validateDevPortalAPIKey(apiName, apiVersion, apiKey string) (bool, error) {
	v.logger.Debug("Validating developer portal API key",
		zap.String("apiName", apiName),
		zap.String("apiVersion", apiVersion),
	)

	// TODO: Implement developer portal API key validation
	// This should make an HTTP request to the developer portal's API key validation endpoint
	// Example implementation:
	// 1. Create HTTP client
	// 2. Make POST request to developer portal: POST /api/v1/validate-key
	// 3. Send payload: {"apiName": apiName, "apiVersion": apiVersion, "apiKey": apiKey}
	// 4. Parse response and return validation result

	v.logger.Warn("Developer portal API key validation not yet implemented")
	return false, nil
}

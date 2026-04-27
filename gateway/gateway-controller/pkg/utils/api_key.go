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

	"github.com/wso2/api-platform/common/apikey"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"

	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// APIKeyCreationParams contains parameters for API key creation operations.
// Handles both local key generation and external key injection.
type APIKeyCreationParams struct {
	Kind          string                    // Artifact kind (e.g. RestApi, LlmProvider, LlmProxy); defaults to RestApi if empty
	Handle        string                    // API handle/ID
	Request       api.APIKeyCreationRequest // Request body with API key creation details
	User          *commonmodels.AuthContext // User who initiated the request
	CorrelationID string                    // Correlation ID for tracking
	Logger        *slog.Logger              // Logger instance
	// UUID is the pre-assigned key UUID from the platform API event path.
	// Nil in the REST API path (a new UUID is generated instead).
	UUID *string
	// ApiKeyHashes contains pre-computed hashes from the platform API event path.
	// Nil in the REST API path (which provides a plain-text key instead).
	ApiKeyHashes *string
	// CreatedAt is the creation timestamp from the platform API. When set (external event
	// path), this value is used instead of time.Now() so the gateway reflects the
	// authoritative timestamp from the control plane.
	CreatedAt *time.Time
	// UpdatedAt is the last-updated timestamp from the platform API. When set (external
	// event path), this value is used instead of time.Now().
	UpdatedAt *time.Time
}

// APIKeyCreationResult contains the result of API key creation.
// Used for both locally generated keys and externally injected keys.
type APIKeyCreationResult struct {
	Response api.APIKeyCreationResponse // Response following the generated schema
	IsRetry  bool                       // Whether this was a retry due to collision
}

// APIKeyRevocationParams contains parameters for API key revocation operations
type APIKeyRevocationParams struct {
	Kind          string                    // Artifact kind (e.g. RestApi, LlmProvider, LlmProxy); defaults to RestApi if empty
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
	Kind          string                        // Artifact kind (e.g. RestApi, LlmProvider, LlmProxy); defaults to RestApi if empty
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
	Kind          string                    // Artifact kind (e.g. RestApi, LlmProvider, LlmProxy); defaults to RestApi if empty
	Handle        string                    // API handle/ID
	APIKeyName    string                    // Name of the API key to update
	Request       api.APIKeyCreationRequest // Request body with update details
	User          *commonmodels.AuthContext // User who initiated the request
	CorrelationID string                    // Correlation ID for tracking
	Logger        *slog.Logger              // Logger instance
	// ApiKeyHashes contains pre-computed hashes from the platform API event path.
	// Nil in the REST API path (which provides a plain-text key instead).
	ApiKeyHashes *string
	// UpdatedAt is the last-updated timestamp from the platform API. When set (external
	// event path), this value is used instead of time.Now().
	UpdatedAt *time.Time
}

// APIKeyUpdateResult contains the result of API key update
type APIKeyUpdateResult struct {
	Response api.APIKeyCreationResponse // Response following the generated schema
}

// ListAPIKeyParams contains parameters for listing API keys
type ListAPIKeyParams struct {
	Kind          string                    // Artifact kind (e.g. RestApi, LlmProvider, LlmProxy); defaults to RestApi if empty
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
	RefreshSnapshot() error
}

// APIKeyService provides utilities for API configuration deployment
type APIKeyService struct {
	store        *storage.ConfigStore
	db           storage.Storage
	xdsManager   XDSManager
	apiKeyConfig *config.APIKeyConfig // Configuration for API keys
	eventHub     eventhub.EventHub
	gatewayID    string
}

// NewAPIKeyService creates a new API key generation service
func NewAPIKeyService(store *storage.ConfigStore, db storage.Storage, xdsManager XDSManager,
	apiKeyConfig *config.APIKeyConfig, eventHub eventhub.EventHub, gatewayID string) *APIKeyService {
	if db == nil {
		panic("APIKeyService requires non-nil storage")
	}
	trimmedGatewayID := requireReplicaSyncWiring("APIKeyService", eventHub, gatewayID)

	return &APIKeyService{
		store:        store,
		db:           db,
		xdsManager:   xdsManager,
		apiKeyConfig: apiKeyConfig,
		eventHub:     eventHub,
		gatewayID:    trimmedGatewayID,
	}
}

// getAPIConfigByHandle resolves an artifact configuration by kind and handle.
func (s *APIKeyService) getAPIConfigByHandle(kind models.ArtifactKind, handle string) (*models.StoredConfig, error) {
	cfg, err := s.db.GetConfigByKindAndHandle(kind, handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("database error while fetching config: %w", err)
	}
	if cfg == nil {
		return nil, storage.ErrNotFound
	}
	return cfg, nil
}

// getAPIKeyByAPIAndName resolves an API key by API UUID and key name from the database.
func (s *APIKeyService) getAPIKeyByAPIAndName(apiID, name string) (*models.APIKey, error) {
	apiKey, err := s.db.GetAPIKeysByAPIAndName(apiID, name)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("database error while fetching API key: %w", err)
	}
	if apiKey == nil {
		return nil, storage.ErrNotFound
	}
	return apiKey, nil
}

// PublishAPIKeyEvent publishes an API key event to the EventHub.
// Exported so callers outside this package (e.g. controlplane.Client) can use
// the same publishing path without duplicating event construction logic.
func (s *APIKeyService) PublishAPIKeyEvent(action, apiID, keyID, correlationID string, logger *slog.Logger) {
	s.publishAPIKeyEvent(action, apiID, keyID, correlationID, logger)
}

// publishAPIKeyEvent publishes an API key event to the EventHub.
func (s *APIKeyService) publishAPIKeyEvent(action, apiID, keyID, correlationID string, logger *slog.Logger) {
	event := eventhub.Event{
		EventType: eventhub.EventTypeAPIKey,
		Action:    action,
		EntityID:  apikey.BuildAPIKeyEntityID(apiID, keyID),
		EventID:   correlationID,
		EventData: eventhub.EmptyEventData,
	}
	if err := s.eventHub.PublishEvent(s.gatewayID, event); err != nil {
		logger.Error("Failed to publish API key event",
			slog.String("action", action),
			slog.String("api_id", apiID),
			slog.String("key_id", keyID),
			slog.Any("error", err))
	}
}

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
	// External key injection occurs when either pre-computed hashes (from platform API event)
	// or a plain-text API key (from REST API) is provided.
	isExternalKeyInjection := (params.ApiKeyHashes != nil && strings.TrimSpace(*params.ApiKeyHashes) != "") ||
		(params.Request.ApiKey != nil && strings.TrimSpace(*params.Request.ApiKey) != "")
	operationType := "generate"
	if isExternalKeyInjection {
		operationType = "register"
	}

	// Validate that API exists
	kind := params.Kind
	if kind == "" {
		kind = models.KindRestApi
	}
	config, err := s.getAPIConfigByHandle(kind, params.Handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			logger.Error("API configuration not found for API Key generation",
				slog.String("operation", operationType+"_key"),
				slog.Any("error", err))
			return nil, fmt.Errorf("%w: API configuration handle '%s' not found", storage.ErrNotFound, params.Handle)
		}
		logger.Error("Failed to retrieve API configuration for API key generation",
			slog.String("operation", operationType+"_key"),
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to retrieve API configuration for handle '%s': %w", params.Handle, err)
	}

	// Check API key limit enforcement
	if err := s.enforceAPIKeyLimit(config.UUID, user.UserID, logger); err != nil {
		logger.Warn("API key generation limit exceeded",
			slog.String("api_id", config.UUID),
			slog.String("operation", operationType+"_key"),
			slog.Any("error", err))
		return nil, err
	}

	result := &APIKeyCreationResult{
		IsRetry: false,
	}

	// Create the API key from request (generate new or register external)
	// For local keys, retry once if duplicate is detected during generation
	apiKey, err := s.createAPIKeyFromRequest(params.Handle, &params.Request, user.UserID, config, params.UUID, params.ApiKeyHashes, params.CreatedAt, params.UpdatedAt)
	if err != nil {
		logger.Error("Failed to generate API key",
			slog.Any("error", err),
			slog.String("handle", params.Handle),
			slog.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Persist to database.
	// UpsertAPIKey is used for all paths so that:
	//   - A new key is inserted when it does not yet exist.
	//   - An existing key is updated only when the incoming updated_at is strictly newer,
	//     preventing out-of-order or late-arriving events from overwriting fresher data.
	if err := s.db.UpsertAPIKey(apiKey); err != nil {
		if errors.Is(err, storage.ErrConflict) && !isExternalKeyInjection {
			// Hash-value collision on a locally generated key — retry once with a new key.
			logger.Warn("API key collision detected, generating new key",
				slog.String("operation", operationType+"_key"))
			apiKey, err = s.createAPIKeyFromRequest(params.Handle, &params.Request, user.UserID, config, params.UUID, params.ApiKeyHashes, params.CreatedAt, params.UpdatedAt)
			if err != nil {
				logger.Error("Failed to generate API key after collision",
					slog.String("operation", operationType+"_key"),
					slog.Any("error", err))
				return nil, fmt.Errorf("failed to generate API key after collision: %w", err)
			}
			if err := s.db.UpsertAPIKey(apiKey); err != nil {
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

	plainAPIKey := apiKey.PlainAPIKey // Store plain API key for response
	apiKey.PlainAPIKey = ""           // Clear plain API key from the struct for security

	apiId := config.UUID
	s.publishAPIKeyEvent("CREATE", apiId, apiKey.UUID, params.CorrelationID, logger)

	// Build response following the generated schema
	result.Response = s.buildAPIKeyResponse(apiKey, params.Handle, plainAPIKey, isExternalKeyInjection)

	logger.Info("API key successfully created",
		slog.String("name", apiKey.Name),
		slog.String("operation", operationType+"_key"),
		slog.Bool("is_retry", result.IsRetry))

	return result, nil
}

// extractConfigDisplayNameVersion extracts DisplayName and Version from the stored configuration
// based on the artifact kind. Supports RestApi, LlmProxy, and LlmProvider.
func extractConfigDisplayNameVersion(kind string, configuration any) (string, string, error) {
	switch kind {
	case models.KindRestApi:
		restCfg, ok := configuration.(api.RestAPI)
		if !ok {
			return "", "", fmt.Errorf("configuration is not a RestAPI (kind: %s)", kind)
		}
		return restCfg.Spec.DisplayName, restCfg.Spec.Version, nil
	case models.KindLlmProxy:
		proxyCfg, ok := configuration.(api.LLMProxyConfiguration)
		if !ok {
			return "", "", fmt.Errorf("configuration is not a LLMProxyConfiguration (kind: %s)", kind)
		}
		return proxyCfg.Spec.DisplayName, proxyCfg.Spec.Version, nil
	case models.KindLlmProvider:
		providerCfg, ok := configuration.(api.LLMProviderConfiguration)
		if !ok {
			return "", "", fmt.Errorf("configuration is not a LLMProviderConfiguration (kind: %s)", kind)
		}
		return providerCfg.Spec.DisplayName, providerCfg.Spec.Version, nil
	case models.KindWebSubApi:
		webSubCfg, ok := configuration.(api.WebSubAPI)
		if !ok {
			return "", "", fmt.Errorf("configuration is not a WebSubAPI (kind: %s)", kind)
		}
		return webSubCfg.Spec.DisplayName, webSubCfg.Spec.Version, nil
	default:
		return "", "", fmt.Errorf("unsupported kind for API key operation: '%s'", kind)
	}
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
	kind := params.Kind
	if kind == "" {
		kind = models.KindRestApi
	}
	config, err := s.getAPIConfigByHandle(kind, params.Handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			logger.Warn("API configuration not found for API key revoke",
				slog.Any("error", err))
			return nil, fmt.Errorf("%w: API configuration handle '%s' not found", storage.ErrNotFound, params.Handle)
		}
		logger.Error("Failed to retrieve API configuration for API key revoke",
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to retrieve API configuration for handle '%s': %w", params.Handle, err)
	}

	var apiKey *models.APIKey

	existingAPIKey, err := s.getAPIKeyByAPIAndName(config.UUID, apiKeyName)
	if err != nil {
		logger.Debug("Failed to get API key for revocation",
			slog.Any("error", err))
		// Continue with revocation for security reasons (don't leak info)
	}

	// If API key not found, log and continue for security reasons
	if existingAPIKey == nil {
		logger.Debug("API key not found for revocation",
			slog.String("api_key_name", apiKeyName))
	}

	apiKey = existingAPIKey

	// For security reasons, perform all validations but don't return errors
	// This prevents information leakage about API key details
	if apiKey != nil {
		// Check if the API key belongs to the specified API
		if apiKey.ArtifactUUID != config.UUID {
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

		// Update the API key status in the database
		if err := s.db.UpdateAPIKey(apiKey); err != nil {
			logger.Error("Failed to update API key status in database",
				slog.Any("error", err))
			return nil, fmt.Errorf("failed to revoke API key: %w", err)
		}

		apiId := config.UUID
		s.publishAPIKeyEvent("DELETE", apiId, apiKey.UUID, params.CorrelationID, logger)
		// Remove the API key from database (complete removal)
		// Note: This is cleanup only - the revocation is already complete
		if err := s.db.RemoveAPIKeyAPIAndName(config.UUID, apiKey.Name); err != nil {
			logger.Warn("Failed to remove API key from database, but revocation was successful",
				slog.Any("error", err))
		}
	}

	logger.Info("API key revoked successfully",
		slog.String("api key", apiKeyName))

	return result, nil
}

// UpdateAPIKey updates an existing API key with a specific provided value
// If the API key doesn't exist, creates a new one instead of failing
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

	// Validate that the name in the request body (if provided) matches the URL path parameter
	if params.Request.Name != nil && *params.Request.Name != "" && *params.Request.Name != params.APIKeyName {
		logger.Warn("API key name mismatch between URL and request body",
			slog.String("url_key_name", params.APIKeyName),
			slog.String("body_key_name", *params.Request.Name))
		return nil, fmt.Errorf("API key name mismatch: name in request body '%s' must match the key name in URL '%s'", *params.Request.Name, params.APIKeyName)
	}

	// Get the API configuration
	kind := params.Kind
	if kind == "" {
		kind = models.KindRestApi
	}
	config, err := s.getAPIConfigByHandle(kind, params.Handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			logger.Warn("API configuration not found for API key update",
				slog.Any("error", err))
			return nil, fmt.Errorf("%w: API configuration handle '%s' not found", storage.ErrNotFound, params.Handle)
		}
		logger.Error("Failed to retrieve API configuration for API key update",
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to retrieve API configuration for handle '%s': %w", params.Handle, err)
	}

	// Get the existing API key by name
	existingKey, err := s.getAPIKeyByAPIAndName(config.UUID, params.APIKeyName)
	if err != nil {
		// Only create a new API key if it's a "not found" error
		// For other errors (DB connection, etc.), return the error
		if storage.IsNotFoundError(err) && params.ApiKeyHashes != nil && strings.TrimSpace(*params.ApiKeyHashes) != "" {
			logger.Info("API key not found for update, creating new API key",
				slog.String("handle", params.Handle),
				slog.String("api_key_name", params.APIKeyName),
				slog.String("correlation_id", params.CorrelationID))

			// Always use the name from the URL path instead of the request body when creating a new key
			params.Request.Name = &params.APIKeyName

			// Create the new API key using the provided request
			creationParams := APIKeyCreationParams{
				Kind:          kind,
				Handle:        params.Handle,
				Request:       params.Request,
				User:          user,
				CorrelationID: params.CorrelationID,
				Logger:        logger,
				ApiKeyHashes:  params.ApiKeyHashes,
			}

			creationResult, err := s.CreateAPIKey(creationParams)
			if err != nil {
				logger.Error("Failed to create new API key during update",
					slog.Any("error", err),
					slog.String("handle", params.Handle),
					slog.String("correlation_id", params.CorrelationID))
				return nil, fmt.Errorf("failed to create new API key: %w", err)
			}

			// Convert creation result to update result
			return &APIKeyUpdateResult{
				Response: creationResult.Response,
			}, nil
		} else if storage.IsNotFoundError(err) {
			// Key not found and no API key value provided for creation
			logger.Warn("API key not found and no api_key value provided for creation",
				slog.String("handle", params.Handle),
				slog.String("api_key_name", params.APIKeyName),
				slog.String("correlation_id", params.CorrelationID))
			return nil, fmt.Errorf("%w: API key '%s' not found for API '%s' and no api_key value provided to create one", storage.ErrNotFound, params.APIKeyName, params.Handle)
		}

		// For non-"not found" errors, return the error
		logger.Warn("Failed to retrieve API key for update",
			slog.String("handle", params.Handle),
			slog.String("api_key_name", params.APIKeyName),
			slog.String("correlation_id", params.CorrelationID),
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to retrieve API key '%s' for API '%s': %w", params.APIKeyName, params.Handle, err)
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

	updatedKey, err := s.updateAPIKeyFromRequest(existingKey, params.Request, user.UserID, logger, params.ApiKeyHashes, params.UpdatedAt)
	if err != nil {
		logger.Error("Failed to update API key from request",
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to update API key from request: %w", err)
	}
	// Clear plaintext secret before persisting or storing
	updatedKey.PlainAPIKey = ""

	// Save to database
	if err := s.db.UpdateAPIKey(updatedKey); err != nil {
		logger.Error("Failed to update API key in database",
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to update API key in database: %w", err)
	}

	apiId := config.UUID
	s.publishAPIKeyEvent("UPDATE", apiId, updatedKey.UUID, params.CorrelationID, logger)

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
				Name:      updatedKey.Name,
				ApiKey:    responseAPIKey,
				ApiId:     params.Handle,
				Status:    api.APIKeyStatus(updatedKey.Status),
				CreatedAt: updatedKey.CreatedAt,
				CreatedBy: updatedKey.CreatedBy,
				ExpiresAt: updatedKey.ExpiresAt,
				Source:    api.APIKeySource(updatedKey.Source),
			},
		},
	}

	logger.Info("API key update completed successfully",
		slog.String("key_id", updatedKey.UUID))

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

	kind := params.Kind
	if kind == "" {
		kind = models.KindRestApi
	}

	// Get the API configuration
	config, err := s.getAPIConfigByHandle(kind, params.Handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			logger.Warn("API configuration not found for API Key regeneration",
				slog.String("handle", params.Handle),
				slog.String("correlation_id", params.CorrelationID),
				slog.Any("error", err))
			return nil, fmt.Errorf("%w: API configuration handle '%s' not found", storage.ErrNotFound, params.Handle)
		}
		logger.Error("Failed to retrieve API configuration for API key regeneration",
			slog.String("handle", params.Handle),
			slog.String("correlation_id", params.CorrelationID),
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to retrieve API configuration for handle '%s': %w", params.Handle, err)
	}

	// Get the existing API key by name
	existingKey, err := s.getAPIKeyByAPIAndName(config.UUID, params.APIKeyName)
	if err != nil {
		if !storage.IsNotFoundError(err) {
			logger.Error("Failed to retrieve API key for regeneration",
				slog.String("handle", params.Handle),
				slog.String("api_key_name", params.APIKeyName),
				slog.String("correlation_id", params.CorrelationID),
				slog.Any("error", err))
			return nil, fmt.Errorf("failed to retrieve API key '%s' for API '%s': %w", params.APIKeyName, params.Handle, err)
		}
		logger.Warn("API key not found for regeneration",
			slog.String("handle", params.Handle),
			slog.String("api_key_name", params.APIKeyName),
			slog.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("%w: API key '%s' not found for API '%s'", storage.ErrNotFound, params.APIKeyName, params.Handle)
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

	// Save regenerated API key to database
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

	plainAPIKey := regeneratedKey.PlainAPIKey // Store plain API key for response
	regeneratedKey.PlainAPIKey = ""           // Clear plain API key from the struct for security

	apiId := config.UUID
	s.publishAPIKeyEvent("UPDATE", apiId, regeneratedKey.UUID, params.CorrelationID, logger)

	// Build and return the response
	result.Response = s.buildAPIKeyResponse(regeneratedKey, params.Handle, plainAPIKey, false)

	logger.Info("API key regeneration completed successfully",
		slog.String("handle", params.Handle),
		slog.String("api_key_name", params.APIKeyName),
		slog.String("new_key_id", regeneratedKey.UUID),
		slog.String("correlation_id", params.CorrelationID))

	return result, nil
}

// ListAPIKeys handles listing API keys for a specific API and user
func (s *APIKeyService) ListAPIKeys(params ListAPIKeyParams) (*ListAPIKeyResult, error) {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	user := params.User

	kind := params.Kind
	if kind == "" {
		kind = models.KindRestApi
	}

	// Validate that API exists
	config, err := s.getAPIConfigByHandle(kind, params.Handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			logger.Warn("API configuration not found for API keys listing",
				slog.String("handle", params.Handle),
				slog.String("correlation_id", params.CorrelationID))
			return nil, fmt.Errorf("%w: API configuration handle '%s' not found", storage.ErrNotFound, params.Handle)
		}
		logger.Error("Failed to retrieve API configuration for API key listing",
			slog.Any("error", err),
			slog.String("handle", params.Handle),
			slog.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("failed to retrieve API configuration for handle '%s': %w", params.Handle, err)
	}

	apiKeys, err := s.db.GetAPIKeysByAPI(config.UUID)
	if err != nil {
		logger.Error("Failed to get API keys from database",
			slog.Any("error", err),
			slog.String("handle", params.Handle),
			slog.String("correlation_id", params.CorrelationID))
		return nil, fmt.Errorf("failed to retrieve API keys: %w", err)
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
			ApiKey:        &key.MaskedAPIKey, // Return masked API key for security
			ApiId:         params.Handle,     // Use handle instead of internal API ID
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
	config *models.StoredConfig, uuid *string, apiKeyHashes *string, createdAt, updatedAt *time.Time) (*models.APIKey, error) {

	// Generate short unique ID (22 characters, URL-safe) for the internal DB primary key
	id, err := s.generateShortUniqueID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique ID: %w", err)
	}

	// Resolve keyUUID: use the one from the platform API event if provided, otherwise generate locally
	var keyUUID string
	if uuid != nil && strings.TrimSpace(*uuid) != "" {
		keyUUID = strings.TrimSpace(*uuid)
	} else {
		keyUUID, err = GenerateUUID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate api key UUID: %w", err)
		}
	}

	// Determine if this is an external key injection or local key generation
	var plainAPIKeyValue string
	var hashedAPIKeyValue string
	var maskedAPIKeyValue string
	var source string
	var isExternalKey bool

	if request.ApiKey != nil && strings.TrimSpace(*request.ApiKey) != "" {
		// External key injection via REST API: plain-text key provided, hash it before storage
		plainAPIKeyValue = strings.TrimSpace(*request.ApiKey)
		hashedAPIKeyValue, err = s.hashAPIKey(plainAPIKeyValue)
		if err != nil {
			return nil, fmt.Errorf("failed to hash API key: %w", err)
		}
		maskedAPIKeyValue = s.maskAPIKey(plainAPIKeyValue)
		source = "external"
		isExternalKey = true
	} else if apiKeyHashes != nil && strings.TrimSpace(*apiKeyHashes) != "" {
		// External key injection via platform API event: pre-computed hashes provided, store directly
		hash, err := extractSHA256Hash(strings.TrimSpace(*apiKeyHashes))
		if err != nil {
			return nil, fmt.Errorf("invalid apiKeyHashes: %w", err)
		}
		hashedAPIKeyValue = hash
		// Use the masked key sent by the platform API
		if request.MaskedApiKey != nil {
			maskedAPIKeyValue = strings.TrimSpace(*request.MaskedApiKey)
		}
		source = "external"
		isExternalKey = true
	} else {
		// Local key generation: generate new random key with our standard format
		// Format: apip_{64_hex_chars} (32 bytes → hex encoded)
		plainAPIKeyValue, err = s.generateAPIKeyValue()
		if err != nil {
			return nil, err
		}
		// Hash the API key for storage and policy engine
		hashedAPIKeyValue, err = s.hashAPIKey(plainAPIKeyValue)
		if err != nil {
			return nil, fmt.Errorf("failed to hash API key: %w", err)
		}
		// Generate masked API key for display purposes
		maskedAPIKeyValue = s.maskAPIKey(plainAPIKeyValue)
		source = "local"
		isExternalKey = false
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
		// Generate unique URL-safe name: use handle + short ID portion as base
		// name is immutable after creation and used in path parameters
		// Use config.UUID (API internal ID) not handle so uniqueness is checked per API
		baseName := fmt.Sprintf("%s-key-%s", handle, id[:8])
		name, err = s.generateUniqueAPIKeyName(config.UUID, baseName, 5)
		if err != nil {
			return nil, fmt.Errorf("failed to generate unique API key name: %w", err)
		}
	}

	now := time.Now()

	// Calculate expiration time
	var expiresAt *time.Time

	if request.ExpiresAt != nil {
		expiresAt = request.ExpiresAt
	} else if request.ExpiresIn != nil {
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

	keyCreatedAt := now
	if createdAt != nil {
		keyCreatedAt = *createdAt
	}
	keyUpdatedAt := now
	if updatedAt != nil {
		keyUpdatedAt = *updatedAt
	}

	apiKey := &models.APIKey{
		UUID:         keyUUID,
		Name:         name,
		APIKey:       hashedAPIKeyValue, // Store hashed key in database and policy engine
		MaskedAPIKey: maskedAPIKeyValue, // Store masked key for display
		ArtifactUUID: config.UUID,
		Status:       models.APIKeyStatusActive,
		CreatedAt:    keyCreatedAt,
		CreatedBy:    user,
		UpdatedAt:    keyUpdatedAt,
		ExpiresAt:    expiresAt,
		Source:       source, // "local" or "external"
	}

	// Set external reference fields if provided
	// external_ref_id is optional and used for tracing purposes only
	if request.ExternalRefId != nil && strings.TrimSpace(*request.ExternalRefId) != "" {
		externalRefId := strings.TrimSpace(*request.ExternalRefId)
		apiKey.ExternalRefId = &externalRefId
	}

	// Set issuer (nil if not provided)
	if request.Issuer != nil && strings.TrimSpace(*request.Issuer) != "" {
		v := strings.TrimSpace(*request.Issuer)
		apiKey.Issuer = &v
	}
	// Temporarily store the plain key for response generation
	// This field is not persisted and only used for returning to user
	// For external keys, we do NOT store the plain key (caller already has it)
	if !isExternalKey {
		apiKey.PlainAPIKey = plainAPIKeyValue
	}

	return apiKey, nil
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
	currentCount, err := s.getCurrentAPIKeyCount(key.ArtifactUUID, key.CreatedBy)
	if err == nil {
		maxAllowed := s.apiKeyConfig.APIKeysPerUserPerAPI
		remaining := maxAllowed - currentCount
		if remaining < 0 {
			remaining = 0
		}
		remainingQuota = &remaining
	}

	// Use plainAPIKey for response if available, otherwise don't return the key
	var responseAPIKey *string
	if plainAPIKey != "" && !isExternalKeyInjection {
		// For newly generated local keys, return the plain API key
		// Format: apip_{64_hex_chars}
		responseAPIKey = &plainAPIKey
	} else {
		// For external keys or existing keys where plainAPIKey is not available, don't return it
		responseAPIKey = nil
	}

	return api.APIKeyCreationResponse{
		Status:               "success",
		Message:              message,
		RemainingApiKeyQuota: remainingQuota,
		ApiKey: &api.APIKey{
			Name:      key.Name,
			ApiKey:    responseAPIKey, // Return plain key only for locally generated keys
			ApiId:     handle,
			Status:    api.APIKeyStatus(key.Status),
			CreatedAt: key.CreatedAt,
			CreatedBy: key.CreatedBy,
			ExpiresAt: key.ExpiresAt,
			Source:    api.APIKeySource(key.Source),
		},
	}
}

// updateAPIKeyFromRequest updates an existing API key with a specific provided value
// Only mutable fields (displayName, api_key value, expiration) can be updated
// Immutable fields (name, source, createdAt, createdBy) are preserved from existing key
func (s *APIKeyService) updateAPIKeyFromRequest(existingKey *models.APIKey, request api.APIKeyCreationRequest,
	user string, logger *slog.Logger, apiKeyHashes *string, updatedAt *time.Time) (*models.APIKey, error) {

	// Validate that either a plain-text key (REST API) or pre-computed hashes (platform API event) is provided
	if (request.ApiKey == nil || strings.TrimSpace(*request.ApiKey) == "") &&
		(apiKeyHashes == nil || strings.TrimSpace(*apiKeyHashes) == "") {
		return nil, fmt.Errorf("apiKey or apiKeyHashes is required for update")
	}

	var hashedAPIKeyValue string
	var maskedAPIKeyValue string
	var err error

	if request.ApiKey != nil && strings.TrimSpace(*request.ApiKey) != "" {
		// Plain-text key from REST API: hash it before storage
		plainKey := strings.TrimSpace(*request.ApiKey)
		hashedAPIKeyValue, err = s.hashAPIKey(plainKey)
		if err != nil {
			return nil, fmt.Errorf("failed to hash API key: %w", err)
		}
		maskedAPIKeyValue = s.maskAPIKey(plainKey)
	} else {
		// Pre-computed hashes from platform API event: store directly
		hashedAPIKeyValue, err = extractSHA256Hash(strings.TrimSpace(*apiKeyHashes))
		if err != nil {
			return nil, fmt.Errorf("invalid apiKeyHashes: %w", err)
		}
		if request.MaskedApiKey != nil {
			maskedAPIKeyValue = strings.TrimSpace(*request.MaskedApiKey)
		}
	}

	now := time.Now()

	// Determine expiration time from request
	var expiresAt *time.Time

	if request.ExpiresAt != nil {
		if request.ExpiresAt.Before(now) {
			return nil, fmt.Errorf("API key expiration time must be in the future, got: %s (current time: %s)",
				request.ExpiresAt.Format(time.RFC3339), now.Format(time.RFC3339))
		}
		expiresAt = request.ExpiresAt
		logger.Info("Using provided expires_at for update", slog.Time("expires_at", *expiresAt))
	} else if request.ExpiresIn != nil {
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
			slog.String("unit", string(request.ExpiresIn.Unit)),
			slog.Int("duration", request.ExpiresIn.Duration),
			slog.Time("calculated_expires_at", *expiresAt))
	}

	// Validate that expiresAt is in the future (if set)
	if expiresAt != nil && expiresAt.Before(now) {
		return nil, fmt.Errorf("API key expiration time must be in the future, got: %s (current time: %s)",
			expiresAt.Format(time.RFC3339), now.Format(time.RFC3339))
	}

	keyUpdatedAt := now
	if updatedAt != nil {
		keyUpdatedAt = *updatedAt
	}

	updatedKey := &models.APIKey{
		UUID:         existingKey.UUID,
		Name:         existingKey.Name,
		APIKey:       hashedAPIKeyValue, // Store hashed key
		MaskedAPIKey: maskedAPIKeyValue, // Store masked key for display
		ArtifactUUID: existingKey.ArtifactUUID,
		Status:       models.APIKeyStatusActive,
		CreatedAt:    existingKey.CreatedAt,
		CreatedBy:    existingKey.CreatedBy,
		UpdatedAt:    keyUpdatedAt,
		ExpiresAt:    expiresAt,
		Source:       existingKey.Source, // Preserve source from original key.
	}

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
	maskedAPIKeyValue := s.maskAPIKey(plainAPIKeyValue)

	now := time.Now()

	// Determine expiration settings based on request and existing key
	var expiresAt *time.Time

	if request.ExpiresAt != nil {
		if request.ExpiresAt.Before(now) {
			return nil, fmt.Errorf("API key expiration time must be in the future, got: %s (current time: %s)",
				request.ExpiresAt.Format(time.RFC3339), now.Format(time.RFC3339))
		}
		expiresAt = request.ExpiresAt
		logger.Info("Using provided expires_at for regeneration", slog.Time("expires_at", *expiresAt))
	} else if request.ExpiresIn != nil {
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
			slog.String("unit", string(request.ExpiresIn.Unit)),
			slog.Int("duration", request.ExpiresIn.Duration),
			slog.Time("calculated_expires_at", *expiresAt))
	} else if existingKey.ExpiresAt != nil {
		// No expiration in request — preserve existing absolute expiry
		expiresAt = existingKey.ExpiresAt
		logger.Info("Using existing key's expires_at for regeneration", slog.Time("expires_at", *expiresAt))
	} else {
		logger.Info("No expiry set for regenerated key (matching existing key)")
	}

	// Validate that expiresAt is in the future (if set)
	if expiresAt != nil && expiresAt.Before(now) {
		return nil, fmt.Errorf("API key expiration time must be in the future, got: %s (current time: %s)",
			expiresAt.Format(time.RFC3339), now.Format(time.RFC3339))
	}

	// Create the regenerated API key
	regeneratedKey := &models.APIKey{
		UUID:         existingKey.UUID,
		Name:         existingKey.Name,
		APIKey:       hashedAPIKeyValue, // Store hashed key
		MaskedAPIKey: maskedAPIKeyValue, // Store masked key for display
		ArtifactUUID: existingKey.ArtifactUUID,
		Status:       models.APIKeyStatusActive,
		CreatedAt:    existingKey.CreatedAt,
		CreatedBy:    existingKey.CreatedBy,
		UpdatedAt:    now,
		ExpiresAt:    expiresAt,
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

// maskAPIKey returns an 8-character masked representation of the API key:
// "***" + last 5 characters. If the key is 5 characters or shorter, returns "********".
func (s *APIKeyService) maskAPIKey(apiKey string) string {
	if len(apiKey) <= 5 {
		return "********"
	}
	return "***" + apiKey[len(apiKey)-5:]
}

// isAdmin checks if the user has admin role
func (s *APIKeyService) isAdmin(user *commonmodels.AuthContext) bool {
	return slices.Contains(user.Roles, "admin")
}

// isDeveloper checks if the user has developer role
func (s *APIKeyService) isDeveloper(user *commonmodels.AuthContext) bool {
	return slices.Contains(user.Roles, "developer")
}

// extractSHA256Hash parses the apiKeyHashes JSON string and returns the sha256 hash value.
// Expected format: {"sha256": "<hex_hash>"}
func extractSHA256Hash(apiKeyHashes string) (string, error) {
	var hashes map[string]string
	if err := json.Unmarshal([]byte(apiKeyHashes), &hashes); err != nil {
		return "", fmt.Errorf("invalid apiKeyHashes format: %w", err)
	}
	hash, ok := hashes[constants.HashingAlgorithmSHA256]
	if !ok || strings.TrimSpace(hash) == "" {
		return "", fmt.Errorf("apiKeyHashes must contain a non-empty '%s' entry", constants.HashingAlgorithmSHA256)
	}
	return strings.TrimSpace(hash), nil
}

// hashAPIKey securely hashes an API key using the configured algorithm
// Returns the hashed API key that should be stored in database and policy engine
// If hashing is disabled, returns the plain API key
func (s *APIKeyService) hashAPIKey(plainAPIKey string) (string, error) {
	if plainAPIKey == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}

	// Only SHA256 is supported
	return s.hashAPIKeyWithSHA256(plainAPIKey)
}

// hashAPIKeyWithSHA256 hashes an API key using plain SHA-256 (no salt)
// Returns the hex-encoded hash (64 characters) that should be stored in database and policy engine
func (s *APIKeyService) hashAPIKeyWithSHA256(plainAPIKey string) (string, error) {
	// Normalize the API key by trimming whitespace
	trimmedAPIKey := strings.TrimSpace(plainAPIKey)
	if trimmedAPIKey == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}

	// Generate hash using SHA-256 (no salt - deterministic)
	hasher := sha256.New()
	hasher.Write([]byte(trimmedAPIKey))
	hash := hasher.Sum(nil)

	// Return hex-encoded hash (64 characters)
	return hex.EncodeToString(hash), nil
}

// compareAPIKeys compares API keys by hashing the provided key and comparing with stored hash
// Returns true if the plain API key matches the stored hash, false otherwise
func (s *APIKeyService) compareAPIKeys(providedAPIKey, storedAPIKey string) bool {
	// Normalize inputs by trimming whitespace
	providedAPIKey = strings.TrimSpace(providedAPIKey)
	storedAPIKey = strings.TrimSpace(storedAPIKey)

	if providedAPIKey == "" || storedAPIKey == "" {
		return false
	}

	// Compute SHA-256 hash of provided key
	hasher := sha256.New()
	hasher.Write([]byte(providedAPIKey))
	hash := hasher.Sum(nil)
	computedHash := hex.EncodeToString(hash)

	// Constant-time comparison with stored hash
	return subtle.ConstantTimeCompare([]byte(computedHash), []byte(storedAPIKey)) == 1
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
	currentCount, err := s.db.CountActiveAPIKeysByUserAndAPI(apiId, userID)
	if err != nil {
		logger.Error("Failed to count API keys from database",
			slog.Any("error", err),
			slog.String("api_id", apiId),
			slog.String("user_id", userID))
		return fmt.Errorf("failed to check API key count: %w", err)
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
	currentCount, err := s.db.CountActiveAPIKeysByUserAndAPI(apiId, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get current API key count: %w", err)
	}
	return currentCount, nil
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
	apiKey, err := s.db.GetAPIKeysByAPIAndName(apiId, name)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check API key name existence: %w", err)
	}
	if apiKey != nil {
		return true, nil
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
	artifactUUID string,
	user string,
	request *api.APIKeyCreationRequest,
	uuid *string,
	apiKeyHashes *string,
	correlationID string,
	logger *slog.Logger,
	createdAt, updatedAt *time.Time,
) (*APIKeyCreationResult, error) {
	if request == nil {
		logger.Error("nil APIKeyCreationRequest",
			slog.String("artifact_uuid", artifactUUID),
			slog.String("correlation_id", correlationID),
		)
		return nil, fmt.Errorf("nil APIKeyCreationRequest for artifact %s", artifactUUID)
	}

	// Resolve artifact UUID to kind and handle.
	storedConfig, err := s.getArtifactConfigByID(artifactUUID)
	if err != nil || storedConfig == nil {
		logger.Error("artifact not found for UUID",
			slog.String("artifact_uuid", artifactUUID),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("artifact not found for UUID %s", artifactUUID)
	}

	logger.Info("Creating external API key from event",
		slog.String("artifact_uuid", artifactUUID),
		slog.String("kind", storedConfig.Kind),
		slog.String("handle", storedConfig.Handle),
		slog.Bool("has_expiry", request.ExpiresAt != nil),
	)

	params := APIKeyCreationParams{
		Kind:    storedConfig.Kind,
		Handle:  storedConfig.Handle,
		Request: *request,
		User: &commonmodels.AuthContext{
			UserID: user,
		},
		Logger:        logger,
		CorrelationID: correlationID,
		UUID:          uuid,
		ApiKeyHashes:  apiKeyHashes,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
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
	artifactUUID string,
	keyName string,
	user string,
	correlationID string,
	logger *slog.Logger,
) error {
	// Resolve artifact UUID to kind and handle.
	storedConfig, err := s.getArtifactConfigByID(artifactUUID)
	if err != nil || storedConfig == nil {
		logger.Error("artifact not found for UUID",
			slog.String("artifact_uuid", artifactUUID),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		return fmt.Errorf("artifact not found for UUID %s", artifactUUID)
	}

	apiKeyRevocationParams := APIKeyRevocationParams{
		Kind:       storedConfig.Kind,
		Handle:     storedConfig.Handle,
		APIKeyName: keyName,
		User: &commonmodels.AuthContext{
			UserID: user,
		},
		Logger:        logger,
		CorrelationID: correlationID,
	}

	_, err = s.RevokeAPIKey(apiKeyRevocationParams)
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
	artifactUUID string,
	apiKeyName string,
	request *api.APIKeyCreationRequest,
	apiKeyHashes *string,
	user string,
	correlationID string,
	logger *slog.Logger,
	updatedAt *time.Time,
) error {
	if request == nil {
		logger.Error("nil APIKeyCreationRequest",
			slog.String("artifact_uuid", artifactUUID),
			slog.String("correlation_id", correlationID),
		)
		return fmt.Errorf("nil APIKeyCreationRequest for artifact %s", artifactUUID)
	}

	// Resolve artifact UUID to kind and handle.
	storedConfig, err := s.getArtifactConfigByID(artifactUUID)
	if err != nil || storedConfig == nil {
		logger.Error("artifact not found for UUID",
			slog.String("artifact_uuid", artifactUUID),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		return fmt.Errorf("artifact not found for UUID %s", artifactUUID)
	}

	apiKeyUpdateParams := APIKeyUpdateParams{
		Kind:         storedConfig.Kind,
		Handle:       storedConfig.Handle,
		APIKeyName:   apiKeyName,
		Request:      *request,
		ApiKeyHashes: apiKeyHashes,
		User: &commonmodels.AuthContext{
			UserID: user,
		},
		Logger:        logger,
		CorrelationID: correlationID,
		UpdatedAt:     updatedAt,
	}
	_, err = s.UpdateAPIKey(apiKeyUpdateParams)
	if err != nil {
		logger.Error("Failed to update external API key", slog.Any("error", err),
			slog.String("correlation_id", correlationID),
			slog.String("user_id", user),
			slog.String("artifact_uuid", artifactUUID),
		)
		return err
	}

	logger.Info("Successfully updated external API key")

	return nil
}

func (s *APIKeyService) getArtifactConfigByID(artifactUUID string) (*models.StoredConfig, error) {
	cfg, err := s.db.GetConfig(artifactUUID)
	if err == nil {
		return cfg, nil
	}
	if !storage.IsNotFoundError(err) {
		return nil, fmt.Errorf("database error while fetching artifact %s: %w", artifactUUID, err)
	}
	// Fallback: incoming UUID may be the APIM/control-plane UUID for a bottom-up
	// synced artifact. Look it up by cp_artifact_id.
	cfg, err = s.db.GetConfigByCPArtifactID(artifactUUID)
	if err == nil {
		return cfg, nil
	}
	if storage.IsNotFoundError(err) {
		return nil, storage.ErrNotFound
	}
	return nil, fmt.Errorf("database error while fetching artifact by cp id %s: %w", artifactUUID, err)
}

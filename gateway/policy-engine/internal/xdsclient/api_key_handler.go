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

package xdsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	// TODO: Migrate to common/apikey.APIkeyStore for better architecture
	// Currently using policy/v1alpha store to ensure validation and xDS use the same instance
	policyv1alpha "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// APIKeyOperationHandler handles API key operations received via xDS
type APIKeyOperationHandler struct {
	// TODO: Migrate to common/apikey.APIkeyStore for better architecture
	// Currently using policy/v1alpha store to ensure validation and xDS use the same instance
	apiKeyStore *policyv1alpha.APIkeyStore
	logger      *slog.Logger
}

// NewAPIKeyOperationHandler creates a new API key operation handler
func NewAPIKeyOperationHandler(apiKeyStore *policyv1alpha.APIkeyStore, logger *slog.Logger) *APIKeyOperationHandler {
	return &APIKeyOperationHandler{
		apiKeyStore: apiKeyStore,
		logger:      logger,
	}
}

// HandleAPIKeyOperation processes API key state received via xDS
func (h *APIKeyOperationHandler) HandleAPIKeyOperation(ctx context.Context, resources map[string]*anypb.Any) error {
	h.logger.Info("Received API key state via xDS", "resource_count", len(resources))

	for resourceName, resource := range resources {
		if resource.TypeUrl != APIKeyStateTypeURL {
			slog.WarnContext(ctx, "Skipping resource with unexpected type",
				"expected", APIKeyStateTypeURL,
				"actual", resource.TypeUrl)
			continue
		}

		// Unmarshal google.protobuf.Struct from the Any
		// The xDS server double-wraps: res.Value contains serialized Any,
		// which in turn contains the serialized Struct
		innerAny := &anypb.Any{}
		if err := proto.Unmarshal(resource.Value, innerAny); err != nil {
			return fmt.Errorf("failed to unmarshal inner Any from resource: %w", err)
		}

		// Now unmarshal the Struct from the inner Any's Value
		apiKeyStruct := &structpb.Struct{}
		if err := proto.Unmarshal(innerAny.Value, apiKeyStruct); err != nil {
			return fmt.Errorf("failed to unmarshal api keys struct from inner Any: %w", err)
		}

		// Convert Struct to JSON then to StoredPolicyConfig
		jsonBytes, err := protojson.Marshal(apiKeyStruct)
		if err != nil {
			return fmt.Errorf("failed to marshal api keys struct to JSON: %w", err)
		}

		var apiKeyState APIKeyStateResource
		if err := json.Unmarshal(jsonBytes, &apiKeyState); err != nil {
			h.logger.Error("Failed to unmarshal API key state",
				"error", err,
				"resource_name", resourceName)
			continue
		}

		h.logger.Info("Processing API key state",
			"version", apiKeyState.Version,
			"api_key_count", len(apiKeyState.APIKeys))

		// Replace all API keys with the new state (state-of-the-world approach)
		if err := h.replaceAllAPIKeys(apiKeyState.APIKeys); err != nil {
			h.logger.Error("Failed to replace API keys",
				"error", err,
				"resource_name", resourceName)
			continue
		}
	}

	return nil
}

// processAPIKeyOperation processes a single API key operation
func (h *APIKeyOperationHandler) processAPIKeyOperation(operation policyenginev1.APIKeyOperation) error {
	switch operation.Operation {
	case policyenginev1.APIKeyOperationStore:
		return h.handleStoreOperation(operation)
	case policyenginev1.APIKeyOperationRevoke:
		return h.handleRevokeOperation(operation)
	case policyenginev1.APIKeyOperationRemoveByAPI:
		return h.handleRemoveByAPIOperation(operation)
	default:
		return fmt.Errorf("unknown API key operation: %s", operation.Operation)
	}
}

// handleStoreOperation handles storing an API key
func (h *APIKeyOperationHandler) handleStoreOperation(operation policyenginev1.APIKeyOperation) error {
	if operation.APIKey == nil {
		return fmt.Errorf("API key data is required for store operation")
	}

	h.logger.Info("Storing API key in policy engine",
		"api_id", operation.APIId,
		"api_key_name", operation.APIKey.Name,
		"correlation_id", operation.CorrelationID)

	// Convert APIKeyData to policyv1alpha.APIKey
	apiKey := &policyv1alpha.APIKey{
		ID:         operation.APIKey.ID,
		Name:       operation.APIKey.Name,
		APIKey:     operation.APIKey.APIKey,
		APIId:      operation.APIKey.APIId,
		Operations: operation.APIKey.Operations,
		Status:     policyv1alpha.APIKeyStatus(operation.APIKey.Status),
		CreatedAt:  operation.APIKey.CreatedAt,
		CreatedBy:  operation.APIKey.CreatedBy,
		UpdatedAt:  operation.APIKey.UpdatedAt,
		ExpiresAt:  operation.APIKey.ExpiresAt,
		Source:     operation.APIKey.Source,
		IndexKey:   operation.APIKey.IndexKey,
	}

	// Store the API key in the policy validation store
	if err := h.apiKeyStore.StoreAPIKey(operation.APIId, apiKey); err != nil {
		return fmt.Errorf("failed to store API key in store: %w", err)
	}

	h.logger.Info("Successfully stored API key in policy engine",
		"api_id", operation.APIId,
		"api_key_name", operation.APIKey.Name,
		"correlation_id", operation.CorrelationID)

	return nil
}

// handleRevokeOperation handles revoking an API key
func (h *APIKeyOperationHandler) handleRevokeOperation(operation policyenginev1.APIKeyOperation) error {
	if operation.APIKeyValue == "" {
		return fmt.Errorf("API key value is required for revoke operation")
	}

	h.logger.Info("Revoking API key in policy engine",
		"api_id", operation.APIId,
		"correlation_id", operation.CorrelationID)

	// Revoke the API key
	if err := h.apiKeyStore.RevokeAPIKey(operation.APIId, operation.APIKeyValue); err != nil {
		return fmt.Errorf("failed to revoke API key in store: %w", err)
	}

	h.logger.Info("Successfully revoked API key in policy engine",
		"api_id", operation.APIId,
		"correlation_id", operation.CorrelationID)

	return nil
}

// handleRemoveByAPIOperation handles removing all API keys for an API
func (h *APIKeyOperationHandler) handleRemoveByAPIOperation(operation policyenginev1.APIKeyOperation) error {
	h.logger.Info("Removing all API keys for API in policy engine",
		"api_id", operation.APIId,
		"correlation_id", operation.CorrelationID)

	// Remove all API keys for the API
	if err := h.apiKeyStore.RemoveAPIKeysByAPI(operation.APIId); err != nil {
		return fmt.Errorf("failed to remove API keys by API in store: %w", err)
	}

	h.logger.Info("Successfully removed all API keys for API in policy engine",
		"api_id", operation.APIId,
		"correlation_id", operation.CorrelationID)

	return nil
}

// replaceAllAPIKeys replaces all API keys with the new state (state-of-the-world approach)
func (h *APIKeyOperationHandler) replaceAllAPIKeys(apiKeyDataList []APIKeyData) error {
	h.logger.Info("Replacing all API keys with new state", "api_key_count", len(apiKeyDataList))

	// First, clear all existing API keys
	if err := h.apiKeyStore.ClearAll(); err != nil {
		return fmt.Errorf("failed to clear existing API keys: %w", err)
	}

	// Then, add all API keys from the new state
	for i, apiKeyData := range apiKeyDataList {
		// Convert APIKeyData to policyv1alpha.APIKey
		apiKey := &policyv1alpha.APIKey{
			ID:         apiKeyData.ID,
			Name:       apiKeyData.Name,
			APIKey:     apiKeyData.APIKey,
			APIId:      apiKeyData.APIId,
			Operations: apiKeyData.Operations,
			Status:     policyv1alpha.APIKeyStatus(apiKeyData.Status),
			CreatedAt:  apiKeyData.CreatedAt,
			CreatedBy:  apiKeyData.CreatedBy,
			UpdatedAt:  apiKeyData.UpdatedAt,
			ExpiresAt:  apiKeyData.ExpiresAt,
			Source:     apiKeyData.Source,
			IndexKey:   apiKeyData.IndexKey,
		}

		// Store the API key in the policy validation store
		if err := h.apiKeyStore.StoreAPIKey(apiKeyData.APIId, apiKey); err != nil {
			h.logger.Error("Failed to store API key during state replacement",
				"error", err,
				"api_key_index", i,
				"api_key_id", apiKeyData.ID,
				"api_id", apiKeyData.APIId)
			return fmt.Errorf("failed to store API key %s: %w", apiKeyData.ID, err)
		}
	}

	h.logger.Info("Successfully replaced all API keys with new state",
		"api_key_count", len(apiKeyDataList))

	return nil
}

// APIKeyStateResource represents the complete state of API keys for the policy engine
type APIKeyStateResource struct {
	APIKeys   []APIKeyData `json:"apiKeys"`
	Version   int64        `json:"version"`
	Timestamp int64        `json:"timestamp"`
}

// APIKeyData represents an API key in the state resource
type APIKeyData struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	APIKey     string     `json:"apiKey"`
	APIId      string     `json:"apiId"`
	Operations string     `json:"operations"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	CreatedBy  string     `json:"createdBy"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	Source     string     `json:"source"`   // "local" | "external"
	IndexKey   string     `json:"indexKey"` // Pre-computed SHA-256 hash for O(1) lookup (external plain text keys only)
}

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
	"log/slog"
	"os"
	"testing"
	"time"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// Helper to create a fresh API key store for each test
func createTestAPIKeyStore() *policy.APIkeyStore {
	return policy.NewAPIkeyStore()
}

// Test helper functions to create test data

func createValidAPIKeyStateResource(t *testing.T) *anypb.Any {
	t.Helper()

	// Create the API key state
	state := APIKeyStateResource{
		Version:   1,
		Timestamp: time.Now().Unix(),
		APIKeys: []APIKeyData{
			{
				ID:         "key-1",
				Name:       "test-key",
				APIKey:     "test-api-key-value",
				APIId:      "api-1",
				Operations: "*",
				Status:     "active",
				CreatedAt:  time.Now(),
				CreatedBy:  "admin",
				UpdatedAt:  time.Now(),
			},
		},
	}

	// Convert to JSON
	jsonBytes, err := json.Marshal(state)
	require.NoError(t, err)

	// Convert JSON to Struct
	apiKeyStruct := &structpb.Struct{}
	err = apiKeyStruct.UnmarshalJSON(jsonBytes)
	require.NoError(t, err)

	// Marshal Struct to bytes
	structBytes, err := proto.Marshal(apiKeyStruct)
	require.NoError(t, err)

	// Create inner Any with Struct
	innerAny := &anypb.Any{
		TypeUrl: "type.googleapis.com/google.protobuf.Struct",
		Value:   structBytes,
	}

	// Marshal inner Any to bytes
	innerAnyBytes, err := proto.Marshal(innerAny)
	require.NoError(t, err)

	// Create outer Any (double-wrapped as per xDS protocol)
	outerAny := &anypb.Any{
		TypeUrl: APIKeyStateTypeURL,
		Value:   innerAnyBytes,
	}

	return outerAny
}

func createInvalidTypeURLResource(t *testing.T) *anypb.Any {
	t.Helper()

	return &anypb.Any{
		TypeUrl: "invalid.type.url",
		Value:   []byte("some data"),
	}
}

func createCorruptProtoResource(t *testing.T) *anypb.Any {
	t.Helper()

	return &anypb.Any{
		TypeUrl: APIKeyStateTypeURL,
		Value:   []byte("corrupt data that is not valid proto"),
	}
}

func createInvalidInnerStructResource(t *testing.T) *anypb.Any {
	t.Helper()

	// Create a valid outer Any, but with corrupt inner data
	innerAny := &anypb.Any{
		TypeUrl: "type.googleapis.com/google.protobuf.Struct",
		Value:   []byte("corrupt struct data"),
	}

	innerAnyBytes, err := proto.Marshal(innerAny)
	require.NoError(t, err)

	return &anypb.Any{
		TypeUrl: APIKeyStateTypeURL,
		Value:   innerAnyBytes,
	}
}

// TestHandleAPIKeyOperation_WrongTypeURL tests that resources with wrong TypeURL are skipped
func TestHandleAPIKeyOperation_WrongTypeURL(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	ctx := context.Background()
	resources := map[string]*anypb.Any{
		"resource-1": createInvalidTypeURLResource(t),
	}

	// Should not return error, just skip the resource
	err := handler.HandleAPIKeyOperation(ctx, resources)
	assert.NoError(t, err)
}

// TestHandleAPIKeyOperation_UnmarshalInnerAnyFails tests error when inner Any unmarshal fails
func TestHandleAPIKeyOperation_UnmarshalInnerAnyFails(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	ctx := context.Background()
	resources := map[string]*anypb.Any{
		"resource-1": createCorruptProtoResource(t),
	}

	err := handler.HandleAPIKeyOperation(ctx, resources)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal inner Any from resource")
}

// TestHandleAPIKeyOperation_UnmarshalStructFails tests error when Struct unmarshal fails
func TestHandleAPIKeyOperation_UnmarshalStructFails(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	ctx := context.Background()
	resources := map[string]*anypb.Any{
		"resource-1": createInvalidInnerStructResource(t),
	}

	err := handler.HandleAPIKeyOperation(ctx, resources)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal api keys struct from inner Any")
}

// TestHandleAPIKeyOperation_JSONUnmarshalFails tests that JSON unmarshal errors are logged but don't stop processing
func TestHandleAPIKeyOperation_JSONUnmarshalFails(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	ctx := context.Background()

	// Create a Struct with invalid JSON structure (missing required fields)
	invalidStruct, err := structpb.NewStruct(map[string]interface{}{
		"invalid": "data",
	})
	require.NoError(t, err)

	structBytes, err := proto.Marshal(invalidStruct)
	require.NoError(t, err)

	innerAny := &anypb.Any{
		TypeUrl: "type.googleapis.com/google.protobuf.Struct",
		Value:   structBytes,
	}

	innerAnyBytes, err := proto.Marshal(innerAny)
	require.NoError(t, err)

	outerAny := &anypb.Any{
		TypeUrl: APIKeyStateTypeURL,
		Value:   innerAnyBytes,
	}

	resources := map[string]*anypb.Any{
		"resource-1": outerAny,
	}

	// Should not return error, just log and continue
	err = handler.HandleAPIKeyOperation(ctx, resources)
	assert.NoError(t, err)
}

// TestHandleAPIKeyOperation_ReplaceAllFails tests that replaceAllAPIKeys errors are logged
// Note: Testing replaceAllAPIKeys failure is challenging with the real store as it doesn't fail easily.
// This test is omitted as the real store's ClearAll and StoreAPIKey are highly reliable.

// TestHandleAPIKeyOperation_Success tests successful processing
func TestHandleAPIKeyOperation_Success(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	ctx := context.Background()
	resources := map[string]*anypb.Any{
		"resource-1": createValidAPIKeyStateResource(t),
	}

	err := handler.HandleAPIKeyOperation(ctx, resources)
	assert.NoError(t, err)
	
	// Verify the key was stored - the store doesn't have a direct Get method,
	// but we can verify by attempting validation
	// Since we don't have direct access to verify storage, we'll just ensure no error
}

// TestProcessAPIKeyOperation_StoreOperation tests Store operation dispatching
func TestProcessAPIKeyOperation_StoreOperation(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	now := time.Now()
	operation := policyenginev1.APIKeyOperation{
		Operation: policyenginev1.APIKeyOperationStore,
		APIId:     "api-1",
		APIKey: &policyenginev1.APIKeyData{
			ID:         "key-1",
			Name:       "test-key",
			APIKey:     "test-value",
			APIId:      "api-1",
			Operations: "*",
			Status:     "active",
			CreatedAt:  now,
			CreatedBy:  "admin",
			UpdatedAt:  now,
		},
	}

	err := handler.processAPIKeyOperation(operation)
	assert.NoError(t, err)
}

// TestProcessAPIKeyOperation_RevokeOperation tests Revoke operation dispatching
func TestProcessAPIKeyOperation_RevokeOperation(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	operation := policyenginev1.APIKeyOperation{
		Operation:   policyenginev1.APIKeyOperationRevoke,
		APIId:       "api-1",
		APIKeyValue: "test-key-value",
	}

	err := handler.processAPIKeyOperation(operation)
	assert.NoError(t, err)
}

// TestProcessAPIKeyOperation_RemoveByAPIOperation tests RemoveByAPI operation dispatching
func TestProcessAPIKeyOperation_RemoveByAPIOperation(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	operation := policyenginev1.APIKeyOperation{
		Operation: policyenginev1.APIKeyOperationRemoveByAPI,
		APIId:     "api-1",
	}

	err := handler.processAPIKeyOperation(operation)
	assert.NoError(t, err)
}

// TestProcessAPIKeyOperation_UnknownOperation tests error for unknown operation
func TestProcessAPIKeyOperation_UnknownOperation(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	operation := policyenginev1.APIKeyOperation{
		Operation: "UNKNOWN_OPERATION",
		APIId:     "api-1",
	}

	err := handler.processAPIKeyOperation(operation)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown API key operation")
}

// TestHandleStoreOperation_Success tests successful store operation
func TestHandleStoreOperation_Success(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	now := time.Now()
	operation := policyenginev1.APIKeyOperation{
		Operation: policyenginev1.APIKeyOperationStore,
		APIId:     "api-1",
		APIKey: &policyenginev1.APIKeyData{
			ID:         "key-1",
			Name:       "test-key",
			APIKey:     "test-value",
			APIId:      "api-1",
			Operations: "*",
			Status:     "active",
			CreatedAt:  now,
			CreatedBy:  "admin",
			UpdatedAt:  now,
		},
	}

	err := handler.handleStoreOperation(operation)
	assert.NoError(t, err)
}

// TestHandleStoreOperation_NilAPIKey tests error when API key is nil
func TestHandleStoreOperation_NilAPIKey(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	operation := policyenginev1.APIKeyOperation{
		Operation: policyenginev1.APIKeyOperationStore,
		APIId:     "api-1",
		APIKey:    nil,
	}

	err := handler.handleStoreOperation(operation)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key data is required")
}

// TestHandleStoreOperation_StoreFailure tests error when store fails
// Note: The real APIkeyStore rarely fails, so this test covers the error path conceptually
// by documenting the expected behavior when a store error occurs.
func TestHandleStoreOperation_StoreFailure(t *testing.T) {
	// This test is more conceptual since the real store doesn't easily produce errors
	// The error handling code path exists in handleStoreOperation for future-proofing
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	now := time.Now()
	
	// First, add a key with the same ID
	firstOp := policyenginev1.APIKeyOperation{
		Operation: policyenginev1.APIKeyOperationStore,
		APIId:     "api-1",
		APIKey: &policyenginev1.APIKeyData{
			ID:         "key-1",
			Name:       "first-key",
			APIKey:     "value-1",
			APIId:      "api-1",
			Operations: "*",
			Status:     "active",
			CreatedAt:  now,
			CreatedBy:  "admin",
			UpdatedAt:  now,
		},
	}
	
	err := handler.handleStoreOperation(firstOp)
	assert.NoError(t, err)
	
	// Now try to add another key with same ID but different name
	// This should trigger ErrConflict from the store
	secondOp := policyenginev1.APIKeyOperation{
		Operation: policyenginev1.APIKeyOperationStore,
		APIId:     "api-1",
		APIKey: &policyenginev1.APIKeyData{
			ID:         "key-1",
			Name:       "second-key", // Different name
			APIKey:     "value-2",
			APIId:      "api-1",
			Operations: "*",
			Status:     "active",
			CreatedAt:  now,
			CreatedBy:  "admin",
			UpdatedAt:  now,
		},
	}
	
	err = handler.handleStoreOperation(secondOp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store API key in store")
}

// TestHandleRevokeOperation_Success tests successful revoke operation
func TestHandleRevokeOperation_Success(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	operation := policyenginev1.APIKeyOperation{
		Operation:   policyenginev1.APIKeyOperationRevoke,
		APIId:       "api-1",
		APIKeyValue: "test-key-value",
	}

	err := handler.handleRevokeOperation(operation)
	assert.NoError(t, err)
}

// TestHandleRevokeOperation_EmptyKeyValue tests error when key value is empty
func TestHandleRevokeOperation_EmptyKeyValue(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	operation := policyenginev1.APIKeyOperation{
		Operation:   policyenginev1.APIKeyOperationRevoke,
		APIId:       "api-1",
		APIKeyValue: "",
	}

	err := handler.handleRevokeOperation(operation)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key value is required")
}

// TestHandleRevokeOperation_RevokeFailure is conceptual - real store doesn't easily fail
// The error path exists for robustness but is challenging to trigger in unit tests

// TestHandleRemoveByAPIOperation_Success tests successful remove by API operation
func TestHandleRemoveByAPIOperation_Success(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	operation := policyenginev1.APIKeyOperation{
		Operation: policyenginev1.APIKeyOperationRemoveByAPI,
		APIId:     "api-1",
	}

	err := handler.handleRemoveByAPIOperation(operation)
	assert.NoError(t, err)
}

// TestHandleRemoveByAPIOperation_RemoveFailure is conceptual - real store doesn't easily fail
// The error path exists for robustness but is challenging to trigger in unit tests

// TestReplaceAllAPIKeys_Success tests successful state replacement
func TestReplaceAllAPIKeys_Success(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	apiKeyDataList := []APIKeyData{
		{
			ID:         "key-1",
			Name:       "test-key-1",
			APIKey:     "test-value-1",
			APIId:      "api-1",
			Operations: "*",
			Status:     "active",
			CreatedAt:  time.Now(),
			CreatedBy:  "admin",
			UpdatedAt:  time.Now(),
		},
		{
			ID:         "key-2",
			Name:       "test-key-2",
			APIKey:     "test-value-2",
			APIId:      "api-2",
			Operations: "*",
			Status:     "active",
			CreatedAt:  time.Now(),
			CreatedBy:  "admin",
			UpdatedAt:  time.Now(),
		},
	}

	err := handler.replaceAllAPIKeys(apiKeyDataList)
	assert.NoError(t, err)
}

// TestReplaceAllAPIKeys_ClearFails - conceptual test since real store rarely fails
// The error handling exists for robustness

// TestReplaceAllAPIKeys_StoreFailsMidLoop tests error when store fails on conflicting key
func TestReplaceAllAPIKeys_StoreFailsMidLoop(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	// Create list with duplicate key IDs for same API - will cause conflict
	apiKeyDataList := []APIKeyData{
		{
			ID:         "key-1",
			Name:       "test-key-1",
			APIKey:     "test-value-1",
			APIId:      "api-1",
			Operations: "*",
			Status:     "active",
			CreatedAt:  time.Now(),
			CreatedBy:  "admin",
			UpdatedAt:  time.Now(),
		},
		{
			ID:         "key-1", // Same ID, different name - will cause conflict
			Name:       "test-key-2",
			APIKey:     "test-value-2",
			APIId:      "api-1", // Same API
			Operations: "*",
			Status:     "active",
			CreatedAt:  time.Now(),
			CreatedBy:  "admin",
			UpdatedAt:  time.Now(),
		},
	}

	err := handler.replaceAllAPIKeys(apiKeyDataList)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store API key key-1")
}

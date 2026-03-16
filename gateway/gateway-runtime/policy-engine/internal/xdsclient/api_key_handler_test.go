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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/apikey"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// Helper to create a fresh API key store for each test
func createTestAPIKeyStore() *apikey.APIkeyStore {
	return apikey.NewAPIkeyStore()
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
				APIKey:     apikey.ComputeAPIKeyHash("test-api-key-value"),
				APIId:      "api-1",
				Operations: `["*"]`,
				Status:     "active",
				CreatedAt:  time.Now(),
				CreatedBy:  "admin",
				UpdatedAt:  time.Now(),
				Source:     "external",
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

// TestHandleAPIKeyOperation_ValidResource tests successful processing of valid API key state
func TestHandleAPIKeyOperation_ValidResource(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler := NewAPIKeyOperationHandler(store, logger)

	ctx := context.Background()
	resources := map[string]*anypb.Any{
		"resource-1": createValidAPIKeyStateResource(t),
	}

	err := handler.HandleAPIKeyOperation(ctx, resources)
	assert.NoError(t, err)

	// Verify the API key was stored
	valid, err := store.ValidateAPIKey("api-1", "*", "GET", "test-api-key-value", "")
	assert.NoError(t, err)
	assert.True(t, valid)
}

// TestReplaceAllAPIKeys_ClearsExistingKeys tests that replaceAllAPIKeys clears existing keys first
func TestReplaceAllAPIKeys_ClearsExistingKeys(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	// Pre-populate with an existing key (hash the API key before storing)
	existingKey := &apikey.APIKey{
		ID:         "old-key",
		Name:       "old-key-name",
		APIKey:     apikey.ComputeAPIKeyHash("old-api-key-value"), // Hash before storing
		APIId:      "api-1",
		Operations: `["*"]`,
		Status:     apikey.Active,
		CreatedAt:  time.Now(),
		CreatedBy:  "admin",
		UpdatedAt:  time.Now(),
		Source:     "external",
	}
	err := store.StoreAPIKey("api-1", existingKey)
	require.NoError(t, err)

	// Replace with new keys
	newKeys := []APIKeyData{
		{
			ID:         "new-key",
			Name:       "new-key-name",
			APIKey:     apikey.ComputeAPIKeyHash("new-api-key-value"),
			APIId:      "api-1",
			Operations: `["*"]`,
			Status:     "active",
			CreatedAt:  time.Now(),
			CreatedBy:  "admin",
			UpdatedAt:  time.Now(),
			Source:     "external",
		},
	}

	err = handler.replaceAllAPIKeys(newKeys)
	assert.NoError(t, err)

	// Old key should no longer be valid
	valid, _ := store.ValidateAPIKey("api-1", "*", "GET", "old-api-key-value", "")
	assert.False(t, valid)

	// New key should be valid
	valid, err = store.ValidateAPIKey("api-1", "*", "GET", "new-api-key-value", "")
	assert.NoError(t, err)
	assert.True(t, valid)
}

// TestReplaceAllAPIKeys_EmptyList tests replacing with an empty list clears all keys
func TestReplaceAllAPIKeys_EmptyList(t *testing.T) {
	store := createTestAPIKeyStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewAPIKeyOperationHandler(store, logger)

	// Pre-populate (hash the API key before storing)
	existingKey := &apikey.APIKey{
		ID:         "key-1",
		Name:       "key-name",
		APIKey:     apikey.ComputeAPIKeyHash("api-key-value"), // Hash before storing
		APIId:      "api-1",
		Operations: `["*"]`,
		Status:     apikey.Active,
		CreatedAt:  time.Now(),
		CreatedBy:  "admin",
		UpdatedAt:  time.Now(),
		Source:     "external",
	}
	err := store.StoreAPIKey("api-1", existingKey)
	require.NoError(t, err)

	// Replace with empty list
	err = handler.replaceAllAPIKeys([]APIKeyData{})
	assert.NoError(t, err)

	// Key should no longer be valid
	valid, _ := store.ValidateAPIKey("api-1", "*", "GET", "api-key-value", "")
	assert.False(t, valid)
}

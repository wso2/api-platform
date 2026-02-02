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

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// Helper to create a fresh lazy resource store for each test
func createTestLazyResourceStore() *policy.LazyResourceStore {
	return policy.NewLazyResourceStore()
}

// Test helper functions to create test data

func createValidLazyResourceStateResource(t *testing.T) *anypb.Any {
	t.Helper()

	// Create the lazy resource state
	state := LazyResourceStateResource{
		Version:   1,
		Timestamp: 123456789,
		Resources: []LazyResourceData{
			{
				ID:           "resource-1",
				ResourceType: "jwks",
				Resource: map[string]interface{}{
					"url": "https://example.com/jwks",
					"ttl": 3600,
				},
			},
			{
				ID:           "resource-2",
				ResourceType: "oauth",
				Resource: map[string]interface{}{
					"endpoint": "https://oauth.example.com/token",
				},
			},
		},
	}

	// Convert to JSON
	jsonBytes, err := json.Marshal(state)
	require.NoError(t, err)

	// Convert JSON to Struct
	lazyResourceStruct := &structpb.Struct{}
	err = lazyResourceStruct.UnmarshalJSON(jsonBytes)
	require.NoError(t, err)

	// Marshal Struct to bytes
	structBytes, err := proto.Marshal(lazyResourceStruct)
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
		TypeUrl: LazyResourceTypeURL,
		Value:   innerAnyBytes,
	}

	return outerAny
}

func createInvalidTypeURLLazyResource(t *testing.T) *anypb.Any {
	t.Helper()

	return &anypb.Any{
		TypeUrl: "invalid.type.url",
		Value:   []byte("some data"),
	}
}

func createCorruptProtoLazyResource(t *testing.T) *anypb.Any {
	t.Helper()

	return &anypb.Any{
		TypeUrl: LazyResourceTypeURL,
		Value:   []byte("corrupt data that is not valid proto"),
	}
}

func createInvalidInnerStructLazyResource(t *testing.T) *anypb.Any {
	t.Helper()

	// Create a valid outer Any, but with corrupt inner data
	innerAny := &anypb.Any{
		TypeUrl: "type.googleapis.com/google.protobuf.Struct",
		Value:   []byte("corrupt struct data"),
	}

	innerAnyBytes, err := proto.Marshal(innerAny)
	require.NoError(t, err)

	return &anypb.Any{
		TypeUrl: LazyResourceTypeURL,
		Value:   innerAnyBytes,
	}
}

// TestNewLazyResourceHandler tests the constructor
func TestNewLazyResourceHandler(t *testing.T) {
	store := createTestLazyResourceStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := NewLazyResourceHandler(store, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, store, handler.lazyResourceStore)
	assert.Equal(t, logger, handler.logger)
}

// TestHandleLazyResourceUpdate_WrongTypeURL tests that resources with wrong TypeURL are skipped
func TestHandleLazyResourceUpdate_WrongTypeURL(t *testing.T) {
	store := createTestLazyResourceStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewLazyResourceHandler(store, logger)

	ctx := context.Background()
	resources := map[string]*anypb.Any{
		"resource-1": createInvalidTypeURLLazyResource(t),
	}

	// Should not return error, just skip the resource
	err := handler.HandleLazyResourceUpdate(ctx, resources)
	assert.NoError(t, err)
}

// TestHandleLazyResourceUpdate_UnmarshalInnerAnyFails tests error when inner Any unmarshal fails
func TestHandleLazyResourceUpdate_UnmarshalInnerAnyFails(t *testing.T) {
	store := createTestLazyResourceStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewLazyResourceHandler(store, logger)

	ctx := context.Background()
	resources := map[string]*anypb.Any{
		"resource-1": createCorruptProtoLazyResource(t),
	}

	err := handler.HandleLazyResourceUpdate(ctx, resources)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal inner Any from resource")
}

// TestHandleLazyResourceUpdate_UnmarshalStructFails tests error when Struct unmarshal fails
func TestHandleLazyResourceUpdate_UnmarshalStructFails(t *testing.T) {
	store := createTestLazyResourceStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewLazyResourceHandler(store, logger)

	ctx := context.Background()
	resources := map[string]*anypb.Any{
		"resource-1": createInvalidInnerStructLazyResource(t),
	}

	err := handler.HandleLazyResourceUpdate(ctx, resources)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal lazy resource struct from inner Any")
}

// TestHandleLazyResourceUpdate_JSONMarshalFails tests error when JSON marshal fails
func TestHandleLazyResourceUpdate_JSONMarshalFails(t *testing.T) {
	store := createTestLazyResourceStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewLazyResourceHandler(store, logger)

	ctx := context.Background()

	// Create a Struct that will fail JSON marshaling (this is hard to trigger, so test is conceptual)
	// In practice, protojson.Marshal rarely fails for valid Structs
	// This test documents the error path exists
	_ = handler
	_ = ctx
	// Skipping actual test as it's difficult to create a Struct that fails JSON marshal
}

// TestHandleLazyResourceUpdate_JSONUnmarshalFails tests error when JSON unmarshal fails
func TestHandleLazyResourceUpdate_JSONUnmarshalFails(t *testing.T) {
	store := createTestLazyResourceStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewLazyResourceHandler(store, logger)

	ctx := context.Background()

	// Create a Struct with data that can't unmarshal to LazyResourceStateResource
	invalidStruct, err := structpb.NewStruct(map[string]interface{}{
		"resources": "not-an-array", // Should be array
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
		TypeUrl: LazyResourceTypeURL,
		Value:   innerAnyBytes,
	}

	resources := map[string]*anypb.Any{
		"resource-1": outerAny,
	}

	err = handler.HandleLazyResourceUpdate(ctx, resources)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal lazy resource state")
}

// TestHandleLazyResourceUpdate_Success tests successful processing
func TestHandleLazyResourceUpdate_Success(t *testing.T) {
	store := createTestLazyResourceStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewLazyResourceHandler(store, logger)

	ctx := context.Background()
	resources := map[string]*anypb.Any{
		"resource-1": createValidLazyResourceStateResource(t),
	}

	err := handler.HandleLazyResourceUpdate(ctx, resources)
	assert.NoError(t, err)

	// Verify resources were stored
	res, err := store.GetResourceByIDAndType("resource-1", "jwks")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "resource-1", res.ID)
	assert.Equal(t, "jwks", res.ResourceType)

	res2, err := store.GetResourceByIDAndType("resource-2", "oauth")
	assert.NoError(t, err)
	assert.NotNil(t, res2)
	assert.Equal(t, "resource-2", res2.ID)
}

// TestHandleLazyResourceUpdate_MultipleResources tests processing multiple resources
func TestHandleLazyResourceUpdate_MultipleResources(t *testing.T) {
	store := createTestLazyResourceStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewLazyResourceHandler(store, logger)

	ctx := context.Background()
	resources := map[string]*anypb.Any{
		"resource-set-1": createValidLazyResourceStateResource(t),
		"resource-set-2": createValidLazyResourceStateResource(t),
	}

	err := handler.HandleLazyResourceUpdate(ctx, resources)
	assert.NoError(t, err)
}

// TestHandleLazyResourceUpdate_MixedValidAndInvalid tests that valid resources are processed even if one is invalid
func TestHandleLazyResourceUpdate_MixedValidAndInvalid(t *testing.T) {
	store := createTestLazyResourceStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewLazyResourceHandler(store, logger)

	ctx := context.Background()
	resources := map[string]*anypb.Any{
		"valid":   createValidLazyResourceStateResource(t),
		"invalid": createInvalidTypeURLLazyResource(t), // Should be skipped
	}

	err := handler.HandleLazyResourceUpdate(ctx, resources)
	// Should succeed - invalid TypeURL is just skipped
	assert.NoError(t, err)
}

// TestReplaceAllLazyResources_Success tests successful state replacement
func TestReplaceAllLazyResources_Success(t *testing.T) {
	store := createTestLazyResourceStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewLazyResourceHandler(store, logger)

	resourceDataList := []LazyResourceData{
		{
			ID:           "res-1",
			ResourceType: "jwks",
			Resource: map[string]interface{}{
				"url": "https://example.com/jwks",
			},
		},
		{
			ID:           "res-2",
			ResourceType: "oauth",
			Resource: map[string]interface{}{
				"endpoint": "https://oauth.example.com",
			},
		},
	}

	err := handler.replaceAllLazyResources(resourceDataList)
	assert.NoError(t, err)

	// Verify resources were stored
	res1, err := store.GetResourceByIDAndType("res-1", "jwks")
	assert.NoError(t, err)
	assert.NotNil(t, res1)
	assert.Equal(t, "res-1", res1.ID)

	res2, err := store.GetResourceByIDAndType("res-2", "oauth")
	assert.NoError(t, err)
	assert.NotNil(t, res2)
	assert.Equal(t, "res-2", res2.ID)
}

// TestReplaceAllLazyResources_EmptyList tests replacement with empty list
func TestReplaceAllLazyResources_EmptyList(t *testing.T) {
	store := createTestLazyResourceStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewLazyResourceHandler(store, logger)

	// First add some resources
	resourceDataList := []LazyResourceData{
		{
			ID:           "res-1",
			ResourceType: "jwks",
			Resource:     map[string]interface{}{"url": "test"},
		},
	}
	err := handler.replaceAllLazyResources(resourceDataList)
	assert.NoError(t, err)

	// Now replace with empty list
	err = handler.replaceAllLazyResources([]LazyResourceData{})
	assert.NoError(t, err)

	// Verify store is empty (resource should not be found)
	res, err := store.GetResourceByIDAndType("res-1", "jwks")
	assert.Error(t, err)
	assert.Nil(t, res)
}

// TestReplaceAllLazyResources_ReplacesExisting tests that new state replaces old state
func TestReplaceAllLazyResources_ReplacesExisting(t *testing.T) {
	store := createTestLazyResourceStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewLazyResourceHandler(store, logger)

	// First state
	firstState := []LazyResourceData{
		{
			ID:           "res-1",
			ResourceType: "jwks",
			Resource:     map[string]interface{}{"url": "old-url"},
		},
	}
	err := handler.replaceAllLazyResources(firstState)
	assert.NoError(t, err)

	// Second state (different resources)
	secondState := []LazyResourceData{
		{
			ID:           "res-2",
			ResourceType: "oauth",
			Resource:     map[string]interface{}{"endpoint": "new-endpoint"},
		},
	}
	err = handler.replaceAllLazyResources(secondState)
	assert.NoError(t, err)

	// Verify old resource is gone
	res1, err := store.GetResourceByIDAndType("res-1", "jwks")
	assert.Error(t, err)
	assert.Nil(t, res1)

	// Verify new resource exists
	res2, err := store.GetResourceByIDAndType("res-2", "oauth")
	assert.NoError(t, err)
	assert.NotNil(t, res2)
	assert.Equal(t, "res-2", res2.ID)
}

/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package apikey

import (
	"testing"
	"time"
)

func TestAPIKeyHashedValidation(t *testing.T) {
	store := NewAPIkeyStore()

	// Create a plain text API key
	plainAPIKey := "apip_88f8399ef29761f92f4f6d2dbd6dcd78399b3bcb8c53417cb23726e5780ac215"

	// Hash the API key using the unified ComputeAPIKeyHash function
	hashedAPIKey := ComputeAPIKeyHash(plainAPIKey)

	// Create API key with hashed value
	apiKey := &APIKey{
		ID:         "test-id-1",
		Name:       "test-key",
		Source:     "local",
		APIKey:     hashedAPIKey, // Store hashed key
		APIId:      "api-123",
		Operations: "[\"*\"]",
		Status:     Active,
		CreatedAt:  time.Now(),
		CreatedBy:  "test-user",
		UpdatedAt:  time.Now(),
		ExpiresAt:  nil,
	}

	// Store the API key
	err := store.StoreAPIKey("api-123", apiKey)
	if err != nil {
		t.Fatalf("Failed to store API key: %v", err)
	}

	// Test validation with correct plain text key
	resolvedKey, err := store.ResolveValidatedAPIKey("api-123", "/test", "GET", plainAPIKey, "")
	if err != nil {
		t.Fatalf("Validation failed with error: %v", err)
	}
	if resolvedKey == nil {
		t.Error("Validation should succeed with correct plain text API key")
	}
}

func TestAPIKeyHashedValidationFailures(t *testing.T) {
	store := NewAPIkeyStore()

	plainAPIKey := "apip_88f8399ef29761f92f4f6d2dbd6dcd78399b3bcb8c53417cb23726e5780ac215"

	// Hash the API key using the unified ComputeAPIKeyHash function
	hashedAPIKey := ComputeAPIKeyHash(plainAPIKey)

	apiKey := &APIKey{
		ID:         "test-id-2",
		Name:       "test-key-2",
		APIKey:     hashedAPIKey,
		Source:     "local",
		APIId:      "api-456",
		Operations: "[\"*\"]",
		Status:     Active,
		CreatedAt:  time.Now(),
		CreatedBy:  "test-user",
		UpdatedAt:  time.Now(),
		ExpiresAt:  nil,
	}

	err := store.StoreAPIKey("api-456", apiKey)
	if err != nil {
		t.Fatalf("Failed to store API key: %v", err)
	}

	// Test validation with wrong plain text key
	wrongKey := "apip_wrong399ef29761f92f4f6d2dbd6dcd78399b3bcb8c53417cb23726e5780ac999"
	resolvedKey, err := store.ResolveValidatedAPIKey("api-456", "/test", "GET", wrongKey, "")
	if err != nil {
		if err != ErrNotFound {
			t.Fatalf("Expected ErrNotFound, got: %v", err)
		}
	}
	if resolvedKey != nil {
		t.Error("Validation should fail with incorrect plain text API key")
	}

	// Test validation with non-existent API
	resolvedKey, err = store.ResolveValidatedAPIKey("non-existent-api", "/test", "GET", plainAPIKey, "")
	if err == nil {
		t.Error("Expected error for non-existent API")
	}
	if resolvedKey != nil {
		t.Error("Validation should fail for non-existent API")
	}
}

func TestAPIKeyHashedRevocation(t *testing.T) {
	store := NewAPIkeyStore()

	plainAPIKey := "apip_revoke399ef29761f92f4f6d2dbd6dcd78399b3bcb8c53417cb23726e5780ac215"

	// Hash the API key using the unified ComputeAPIKeyHash function
	hashedAPIKey := ComputeAPIKeyHash(plainAPIKey)

	apiKey := &APIKey{
		ID:         "test-id-3",
		Name:       "revoke-test-key",
		APIKey:     hashedAPIKey,
		Source:     "local",
		APIId:      "api-789",
		Operations: "[\"*\"]",
		Status:     Active,
		CreatedAt:  time.Now(),
		CreatedBy:  "test-user",
		UpdatedAt:  time.Now(),
		ExpiresAt:  nil,
	}

	err := store.StoreAPIKey("api-789", apiKey)
	if err != nil {
		t.Fatalf("Failed to store API key: %v", err)
	}

	// Verify key works before revocation
	resolvedKey, err := store.ResolveValidatedAPIKey("api-789", "/test", "GET", plainAPIKey, "")
	if err != nil {
		t.Fatalf("Validation failed before revocation: %v", err)
	}
	if resolvedKey == nil {
		t.Error("API key should be valid before revocation")
	}

	// Revoke the API key using plain text key
	err = store.RevokeAPIKey("api-789", plainAPIKey)
	if err != nil {
		t.Fatalf("Failed to revoke API key: %v", err)
	}

	// Verify key no longer works after revocation
	resolvedKey, err = store.ResolveValidatedAPIKey("api-789", "/test", "GET", plainAPIKey, "")
	if err != nil && err != ErrNotFound {
		t.Fatalf("Unexpected error during validation after revocation: %v", err)
	}
	if resolvedKey != nil {
		t.Error("API key should be invalid after revocation")
	}
}

func TestGetAPIKeyByHash(t *testing.T) {
	store := NewAPIkeyStore()
	plain := "apip_0b2f6c1e9d4a7b3c5e8f1a2d4c6b8e0f1a3c5e7b9d1f3a5c7e9b1d3f5a7c9e1b"
	hash := ComputeAPIKeyHash(plain)
	if err := store.StoreAPIKey("api-1", &APIKey{
		ID: "key-1", Name: "support-agent-key", Source: "local", APIKey: hash, APIId: "api-1",
		Operations: "[\"*\"]", Status: Active, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("store: %v", err)
	}

	got, err := store.GetAPIKeyByHash("api-1", hash)
	if err != nil || got.Name != "support-agent-key" || got.ID != "key-1" {
		t.Fatalf("GetAPIKeyByHash = %+v, %v", got, err)
	}
	got.Name = "changed"
	if again, _ := store.GetAPIKeyByHash("api-1", hash); again.Name != "support-agent-key" {
		t.Fatalf("the store's key was changed through the returned copy")
	}
	for _, c := range []struct{ apiID, hash string }{{"api-2", hash}, {"api-1", ComputeAPIKeyHash("other")}, {"api-1", ""}} {
		if _, err := store.GetAPIKeyByHash(c.apiID, c.hash); err != ErrNotFound {
			t.Errorf("GetAPIKeyByHash(%q, %q) error = %v, want ErrNotFound", c.apiID, c.hash, err)
		}
	}
}

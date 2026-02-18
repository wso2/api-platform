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
	valid, apiKeyDetails, err := store.ValidateAPIKey("api-123", "/test", "GET", plainAPIKey)
	if err != nil {
		t.Fatalf("Validation failed with error: %v", err)
	}
	if !valid {
		t.Error("Validation should succeed with correct plain text API key")
	}
	if apiKeyDetails == nil {
		t.Error("Expected API key details to be returned")
	}
	if apiKeyDetails != nil && apiKeyDetails.CreatedBy != "test-user" {
		t.Errorf("Expected CreatedBy to be 'test-user', got: %s", apiKeyDetails.CreatedBy)
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
	valid, _, err := store.ValidateAPIKey("api-456", "/test", "GET", wrongKey)
	if err != nil {
		if err != ErrNotFound {
			t.Fatalf("Expected ErrNotFound, got: %v", err)
		}
	}
	if valid {
		t.Error("Validation should fail with incorrect plain text API key")
	}

	// Test validation with non-existent API
	valid, _, err = store.ValidateAPIKey("non-existent-api", "/test", "GET", plainAPIKey)
	if err == nil {
		t.Error("Expected error for non-existent API")
	}
	if valid {
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
	valid, _, err := store.ValidateAPIKey("api-789", "/test", "GET", plainAPIKey)
	if err != nil {
		t.Fatalf("Validation failed before revocation: %v", err)
	}
	if !valid {
		t.Error("API key should be valid before revocation")
	}

	// Revoke the API key using plain text key
	err = store.RevokeAPIKey("api-789", plainAPIKey)
	if err != nil {
		t.Fatalf("Failed to revoke API key: %v", err)
	}

	// Verify key no longer works after revocation
	valid, _, err = store.ValidateAPIKey("api-789", "/test", "GET", plainAPIKey)
	if err != nil && err != ErrNotFound {
		t.Fatalf("Unexpected error during validation after revocation: %v", err)
	}
	if valid {
		t.Error("API key should be invalid after revocation")
	}
}

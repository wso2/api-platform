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
	"strings"
	"testing"
)

func TestAPIKeyHashing(t *testing.T) {
	service := &APIKeyService{}

	// Test API key generation and hashing
	plainKey := "apip_test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Test hashing
	hashedKey, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash API key: %v", err)
	}

	// Verify the hashed key is different from plain key
	if hashedKey == plainKey {
		t.Error("Hashed key should be different from plain key")
	}

	// Verify the hash starts with argon2id prefix
	if !strings.HasPrefix(hashedKey, "$argon2id$v=19$") {
		t.Error("Hashed key should start with $argon2id$v=19$ prefix")
	}

	// Test validation with correct key
	valid := service.ValidateAPIKey(plainKey, hashedKey)
	if !valid {
		t.Error("Validation should succeed with correct plain key")
	}

	// Test validation with incorrect key
	wrongKey := "apip_wrong123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	valid = service.ValidateAPIKey(wrongKey, hashedKey)
	if valid {
		t.Error("Validation should fail with incorrect plain key")
	}

	// Test empty keys
	_, err = service.hashAPIKey("")
	if err == nil {
		t.Error("Hashing empty key should return error")
	}

	valid = service.ValidateAPIKey("", hashedKey)
	if valid {
		t.Error("Validation should fail with empty plain key")
	}

	valid = service.ValidateAPIKey(plainKey, "")
	if valid {
		t.Error("Validation should fail with empty hash")
	}
}

func TestAPIKeyHashParameters(t *testing.T) {
	service := &APIKeyService{}
	plainKey := "apip_test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	hashedKey, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash API key: %v", err)
	}

	// Verify the format includes the expected Argon2id parameters
	expectedParams := "m=65536,t=1,p=4"
	if !strings.Contains(hashedKey, expectedParams) {
		t.Errorf("Hash should contain parameters %s, got: %s", expectedParams, hashedKey)
	}
}

func TestAPIKeyHashDeterminism(t *testing.T) {
	service := &APIKeyService{}
	plainKey := "apip_test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Generate two hashes of the same key
	hash1, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash API key (1): %v", err)
	}

	hash2, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash API key (2): %v", err)
	}

	// Hashes should be different due to random salt
	if hash1 == hash2 {
		t.Error("Two hashes of the same key should be different (Argon2id uses random salt)")
	}

	// But both should validate against the same plain key
	if !service.ValidateAPIKey(plainKey, hash1) {
		t.Error("First hash should validate correctly")
	}

	if !service.ValidateAPIKey(plainKey, hash2) {
		t.Error("Second hash should validate correctly")
	}
}

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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"strings"
	"testing"
)

func TestArgon2IDAPIKeyHashing(t *testing.T) {
	// Create service with Argon2id hashing configuration
	service := &APIKeyService{
		hashingConfig: &config.APIKeyHashingConfig{
			Enabled:   true,
			Algorithm: constants.HashingAlgorithmArgon2ID,
		},
	}

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
	valid := service.compareAPIKeys(plainKey, hashedKey)
	if !valid {
		t.Error("Validation should succeed with correct plain key")
	}

	// Test validation with incorrect key
	wrongKey := "apip_wrong123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	valid = service.compareAPIKeys(wrongKey, hashedKey)
	if valid {
		t.Error("Validation should fail with incorrect plain key")
	}

	// Test empty keys
	_, err = service.hashAPIKey("")
	if err == nil {
		t.Error("Hashing empty key should return error")
	}

	valid = service.compareAPIKeys("", hashedKey)
	if valid {
		t.Error("Validation should fail with empty plain key")
	}

	valid = service.compareAPIKeys(plainKey, "")
	if valid {
		t.Error("Validation should fail with empty hash")
	}
}

func TestArgon2IDAPIKeyHashDeterminism(t *testing.T) {
	// Create service with Argon2id hashing configuration
	service := &APIKeyService{
		hashingConfig: &config.APIKeyHashingConfig{
			Enabled:   true,
			Algorithm: constants.HashingAlgorithmArgon2ID,
		},
	}
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
	if !service.compareAPIKeys(plainKey, hash1) {
		t.Error("First hash should validate correctly")
	}

	if !service.compareAPIKeys(plainKey, hash2) {
		t.Error("Second hash should validate correctly")
	}
}

func TestBcryptAPIKeyHashing(t *testing.T) {
	// Create service with bcrypt hashing configuration
	service := &APIKeyService{
		hashingConfig: &config.APIKeyHashingConfig{
			Enabled:   true,
			Algorithm: constants.HashingAlgorithmBcrypt,
		},
	}

	// Test API key generation and hashing with bcrypt
	plainKey := "apip_test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Test bcrypt hashing
	hashedKey, err := service.hashAPIKeyWithBcrypt(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash API key with bcrypt: %v", err)
	}

	// Verify the hashed key is different from plain key
	if hashedKey == plainKey {
		t.Error("Bcrypt hashed key should be different from plain key")
	}

	// Verify the hash starts with bcrypt prefix ($2a$, $2b$, or $2y$)
	if !strings.HasPrefix(hashedKey, "$2a$") &&
		!strings.HasPrefix(hashedKey, "$2b$") &&
		!strings.HasPrefix(hashedKey, "$2y$") {
		t.Errorf("Bcrypt hashed key should start with $2a$, $2b$, or $2y$ prefix, got: %s", hashedKey)
	}

	// Test validation with correct key
	valid := service.compareAPIKeys(plainKey, hashedKey)
	if !valid {
		t.Error("Bcrypt validation should succeed with correct plain key")
	}

	// Test validation with incorrect key
	wrongKey := "apip_wrong123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	valid = service.compareAPIKeys(wrongKey, hashedKey)
	if valid {
		t.Error("Bcrypt validation should fail with incorrect plain key")
	}

	// Test empty keys
	_, err = service.hashAPIKeyWithBcrypt("")
	if err == nil {
		t.Error("Bcrypt hashing empty key should return error")
	}

	valid = service.compareAPIKeys("", hashedKey)
	if valid {
		t.Error("Bcrypt validation should fail with empty plain key")
	}

	valid = service.compareAPIKeys(plainKey, "")
	if valid {
		t.Error("Bcrypt validation should fail with empty hash")
	}
}

func TestBcryptAPIKeyHashDeterminism(t *testing.T) {
	// Create service with bcrypt hashing configuration
	service := &APIKeyService{
		hashingConfig: &config.APIKeyHashingConfig{
			Enabled:   true,
			Algorithm: constants.HashingAlgorithmBcrypt,
		},
	}
	plainKey := "apip_test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Generate two bcrypt hashes of the same key
	hash1, err := service.hashAPIKeyWithBcrypt(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash API key with bcrypt (1): %v", err)
	}

	hash2, err := service.hashAPIKeyWithBcrypt(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash API key with bcrypt (2): %v", err)
	}

	// Hashes should be different due to random salt
	if hash1 == hash2 {
		t.Error("Two bcrypt hashes of the same key should be different (bcrypt uses random salt)")
	}

	// But both should validate against the same plain key
	if !service.compareAPIKeys(plainKey, hash1) {
		t.Error("First bcrypt hash should validate correctly")
	}

	if !service.compareAPIKeys(plainKey, hash2) {
		t.Error("Second bcrypt hash should validate correctly")
	}
}

func TestSHA256APIKeyHashing(t *testing.T) {
	// Create service with SHA256 hashing configuration
	service := &APIKeyService{
		hashingConfig: &config.APIKeyHashingConfig{
			Enabled:   true,
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	plainKey := "apip_test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Test SHA256 hashing
	hashedKey, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash API key with SHA256: %v", err)
	}

	// Verify the hashed key is different from plain key
	if hashedKey == plainKey {
		t.Error("SHA256 hashed key should be different from plain key")
	}

	// Verify the hash starts with SHA256 prefix
	if !strings.HasPrefix(hashedKey, "$sha256$") {
		t.Error("SHA256 hashed key should start with $sha256$ prefix")
	}

	// Test validation with correct key
	valid := service.compareAPIKeys(plainKey, hashedKey)
	if !valid {
		t.Error("SHA256 validation should succeed with correct plain key")
	}

	// Test validation with incorrect key
	wrongKey := "apip_wrong123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	valid = service.compareAPIKeys(wrongKey, hashedKey)
	if valid {
		t.Error("SHA256 validation should fail with incorrect plain key")
	}
}

func TestSHA256APIKeyHashDeterminism(t *testing.T) {
	// Create service with SHA256 hashing configuration
	service := &APIKeyService{
		hashingConfig: &config.APIKeyHashingConfig{
			Enabled:   true,
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}
	plainKey := "apip_test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Generate two SHA256 hashes of the same key
	hash1, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash API key with SHA256 (1): %v", err)
	}

	hash2, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash API key with SHA256 (2): %v", err)
	}

	// Hashes should be different due to random salt
	if hash1 == hash2 {
		t.Error("Two SHA256 hashes of the same key should be different (SHA256 uses random salt)")
	}

	// But both should validate against the same plain key
	if !service.compareAPIKeys(plainKey, hash1) {
		t.Error("First SHA256 hash should validate correctly")
	}

	if !service.compareAPIKeys(plainKey, hash2) {
		t.Error("Second SHA256 hash should validate correctly")
	}
}

func TestAPIKeyHashingDisabled(t *testing.T) {
	// Create service with hashing disabled
	service := &APIKeyService{
		hashingConfig: &config.APIKeyHashingConfig{
			Enabled: false,
		},
	}

	plainKey := "apip_test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Test hashing when disabled - should return plain key
	result, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to process API key with hashing disabled: %v", err)
	}

	// When hashing is disabled, should return the same plain key
	if result != plainKey {
		t.Error("When hashing is disabled, should return the original plain key")
	}

	// Test validation with hashing disabled - should do plain text comparison
	valid := service.compareAPIKeys(plainKey, result)
	if !valid {
		t.Error("Validation should succeed with plain text comparison when hashing is disabled")
	}

	// Test validation with wrong key
	wrongKey := "apip_wrong123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	valid = service.compareAPIKeys(wrongKey, result)
	if valid {
		t.Error("Validation should fail with wrong key when hashing is disabled")
	}

	// Test empty keys
	_, err = service.hashAPIKey("")
	if err == nil {
		t.Error("Hashing empty key should return error even when hashing is disabled")
	}

	valid = service.compareAPIKeys("", result)
	if valid {
		t.Error("Validation should fail with empty plain key")
	}

	valid = service.compareAPIKeys(plainKey, "")
	if valid {
		t.Error("Validation should fail with empty stored key")
	}
}

func TestAPIKeyHashingDisabledDeterminism(t *testing.T) {
	// Create service with hashing disabled
	service := &APIKeyService{
		hashingConfig: &config.APIKeyHashingConfig{
			Enabled: false,
		},
	}
	plainKey := "apip_test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Generate "hashes" (should be plain keys) multiple times
	result1, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to process API key with hashing disabled (1): %v", err)
	}

	result2, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to process API key with hashing disabled (2): %v", err)
	}

	// Results should be identical when hashing is disabled
	if result1 != result2 {
		t.Error("When hashing is disabled, multiple calls should return identical results")
	}

	// Both should be equal to the original plain key
	if result1 != plainKey || result2 != plainKey {
		t.Error("When hashing is disabled, results should equal the original plain key")
	}

	// Both should validate correctly
	if !service.compareAPIKeys(plainKey, result1) {
		t.Error("First result should validate correctly when hashing is disabled")
	}

	if !service.compareAPIKeys(plainKey, result2) {
		t.Error("Second result should validate correctly when hashing is disabled")
	}
}

func TestHashingConfigurationSwitching(t *testing.T) {
	service := &APIKeyService{}
	plainKey := "apip_test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Test with hashing disabled
	service.SetHashingConfig(&config.APIKeyHashingConfig{Enabled: false})
	plainResult, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to process plain API key: %v", err)
	}
	if plainResult != plainKey {
		t.Error("When hashing is disabled, should return plain key")
	}

	// Test switching to Argon2id
	service.SetHashingConfig(&config.APIKeyHashingConfig{
		Enabled:   true,
		Algorithm: constants.HashingAlgorithmArgon2ID,
	})
	argon2Result, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash with Argon2id: %v", err)
	}
	if !strings.HasPrefix(argon2Result, "$argon2id$") {
		t.Error("Argon2id hash should start with $argon2id$ prefix")
	}

	// Test switching to bcrypt
	service.SetHashingConfig(&config.APIKeyHashingConfig{
		Enabled:   true,
		Algorithm: constants.HashingAlgorithmBcrypt,
	})
	bcryptResult, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash with bcrypt: %v", err)
	}
	if !strings.HasPrefix(bcryptResult, "$2") {
		t.Error("bcrypt hash should start with $2 prefix")
	}

	// Test switching to SHA256
	service.SetHashingConfig(&config.APIKeyHashingConfig{
		Enabled:   true,
		Algorithm: constants.HashingAlgorithmSHA256,
	})
	sha256Result, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Failed to hash with SHA256: %v", err)
	}
	if !strings.HasPrefix(sha256Result, "$sha256$") {
		t.Error("SHA256 hash should start with $sha256$ prefix")
	}

	// Validate that all different hashes work with the same plain key
	service.SetHashingConfig(&config.APIKeyHashingConfig{Enabled: true}) // Reset for validation
	if !service.compareAPIKeys(plainKey, argon2Result) {
		t.Error("Argon2id hash should validate correctly")
	}
	if !service.compareAPIKeys(plainKey, bcryptResult) {
		t.Error("bcrypt hash should validate correctly")
	}
	if !service.compareAPIKeys(plainKey, sha256Result) {
		t.Error("SHA256 hash should validate correctly")
	}
}

func TestAPIKeyHashingDisabledMigrationScenario(t *testing.T) {
	// Test migration scenario where some keys are hashed and some are plain text
	service := &APIKeyService{
		hashingConfig: &config.APIKeyHashingConfig{
			Enabled: false,
		},
	}

	plainKey := "apip_test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Simulate having a pre-existing hashed key (Argon2id format)
	hashedKey := "$argon2id$v=19$m=65536,t=1,p=4$c2FsdA$aGFzaA" // Example format

	// With hashing disabled, validation should still handle hashed keys
	// But it should fall back to plain text comparison for the provided case
	valid := service.compareAPIKeys(plainKey, hashedKey)
	if valid {
		t.Error("Plain key should not validate against a different hash when hashing is disabled")
	}

	// Plain text key should validate against itself
	valid = service.compareAPIKeys(plainKey, plainKey)
	if !valid {
		t.Error("Plain key should validate against itself when hashing is disabled")
	}

	// Test with bcrypt format hash
	bcryptHash := "$2a$12$example" // Example format
	valid = service.compareAPIKeys(plainKey, bcryptHash)
	if valid {
		t.Error("Plain key should not validate against bcrypt hash when hashing is disabled")
	}

	// Test with SHA256 format hash
	sha256Hash := "$sha256$73616c74$68617368" // Example format
	valid = service.compareAPIKeys(plainKey, sha256Hash)
	if valid {
		t.Error("Plain key should not validate against SHA256 hash when hashing is disabled")
	}
}

func TestMixedAPIKeyFormatsValidation(t *testing.T) {
	// Test scenario where we have keys in different formats:
	// - Plain text keys (legacy)
	// - SHA256 hashed keys
	// - bcrypt hashed keys
	// - Argon2id hashed keys

	plainKey1 := "apip_plain123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	plainKey2 := "apip_test456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef01"
	plainKey3 := "apip_demo789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123"
	plainKey4 := "apip_sample9abcdef0123456789abcdef0123456789abcdef0123456789abcdef012345"

	// Generate hashes using different algorithms

	// 1. Plain text key (simulate legacy storage)
	service := &APIKeyService{
		hashingConfig: &config.APIKeyHashingConfig{
			Enabled: false,
		},
	}
	plainHashed, err := service.hashAPIKey(plainKey1)
	if err != nil {
		t.Fatalf("Failed to plain text hashing: %v", err)
	}

	// 2. SHA256 hashed key
	service.SetHashingConfig(&config.APIKeyHashingConfig{
		Enabled:   true,
		Algorithm: constants.HashingAlgorithmSHA256,
	})
	sha256Hashed, err := service.hashAPIKey(plainKey2)
	if err != nil {
		t.Fatalf("Failed to hash key with SHA256: %v", err)
	}

	// 3. bcrypt hashed key
	service.SetHashingConfig(&config.APIKeyHashingConfig{
		Enabled:   true,
		Algorithm: constants.HashingAlgorithmBcrypt,
	})
	bcryptHashed, err := service.hashAPIKey(plainKey3)
	if err != nil {
		t.Fatalf("Failed to hash key with bcrypt: %v", err)
	}

	// 4. Argon2id hashed key
	service.SetHashingConfig(&config.APIKeyHashingConfig{
		Enabled:   true,
		Algorithm: constants.HashingAlgorithmArgon2ID,
	})
	argon2idHashed, err := service.hashAPIKey(plainKey4)
	if err != nil {
		t.Fatalf("Failed to hash key with Argon2id: %v", err)
	}

	// Reset service to simulate runtime validation (algorithm agnostic)
	service.SetHashingConfig(&config.APIKeyHashingConfig{
		Enabled:   true,
		Algorithm: constants.HashingAlgorithmSHA256, // Current default
	})

	// Test validation of each key format

	// 1. Validate plain text key (should work via fallback)
	valid := service.compareAPIKeys(plainKey1, plainHashed)
	if !valid {
		t.Error("Plain text key should validate against plain text stored value")
	}

	// 2. Validate SHA256 hashed key
	valid = service.compareAPIKeys(plainKey2, sha256Hashed)
	if !valid {
		t.Error("Plain key should validate against SHA256 hash")
	}

	// Verify SHA256 format
	if !strings.HasPrefix(sha256Hashed, "$sha256$") {
		t.Error("SHA256 hash should start with $sha256$ prefix")
	}

	// 3. Validate bcrypt hashed key
	valid = service.compareAPIKeys(plainKey3, bcryptHashed)
	if !valid {
		t.Error("Plain key should validate against bcrypt hash")
	}

	// Verify bcrypt format
	if !strings.HasPrefix(bcryptHashed, "$2") {
		t.Error("bcrypt hash should start with $2 prefix")
	}

	// 4. Validate Argon2id hashed key
	valid = service.compareAPIKeys(plainKey4, argon2idHashed)
	if !valid {
		t.Error("Plain key should validate against Argon2id hash")
	}

	// Verify Argon2id format
	if !strings.HasPrefix(argon2idHashed, "$argon2id$") {
		t.Error("Argon2id hash should start with $argon2id$ prefix")
	}

	// Test cross-validation (should fail)

	// Plain key 1 should not validate against other hashes
	valid = service.compareAPIKeys(plainKey1, sha256Hashed)
	if valid {
		t.Error("Wrong plain key should not validate against SHA256 hash")
	}

	valid = service.compareAPIKeys(plainKey1, bcryptHashed)
	if valid {
		t.Error("Wrong plain key should not validate against bcrypt hash")
	}

	valid = service.compareAPIKeys(plainKey1, argon2idHashed)
	if valid {
		t.Error("Wrong plain key should not validate against Argon2id hash")
	}

	// Plain key 2 should not validate against other hashes
	valid = service.compareAPIKeys(plainKey2, plainHashed)
	if valid {
		t.Error("Wrong plain key should not validate against plain text stored value")
	}

	valid = service.compareAPIKeys(plainKey2, bcryptHashed)
	if valid {
		t.Error("Wrong plain key should not validate against bcrypt hash")
	}

	valid = service.compareAPIKeys(plainKey2, argon2idHashed)
	if valid {
		t.Error("Wrong plain key should not validate against Argon2id hash")
	}

	// Plain key 3 should not validate against other hashes
	valid = service.compareAPIKeys(plainKey3, plainHashed)
	if valid {
		t.Error("Wrong plain key should not validate against plain text stored value")
	}

	valid = service.compareAPIKeys(plainKey3, sha256Hashed)
	if valid {
		t.Error("Wrong plain key should not validate against sha256 hash")
	}

	valid = service.compareAPIKeys(plainKey3, argon2idHashed)
	if valid {
		t.Error("Wrong plain key should not validate against Argon2id hash")
	}

	// Plain key 4 should not validate against other hashes
	valid = service.compareAPIKeys(plainKey4, plainHashed)
	if valid {
		t.Error("Wrong plain key should not validate against plain text stored value")
	}

	valid = service.compareAPIKeys(plainKey4, sha256Hashed)
	if valid {
		t.Error("Wrong plain key should not validate against sha256 hash")
	}

	valid = service.compareAPIKeys(plainKey4, bcryptHashed)
	if valid {
		t.Error("Wrong plain key should not validate against bcrypt hash")
	}

	// Test with completely wrong keys
	wrongKey := "apip_wrong56789abcdef0123456789abcdef0123456789abcdef0123456789abcdef01234"

	valid = service.compareAPIKeys(wrongKey, plainHashed)
	if valid {
		t.Error("Wrong key should not validate against plain text")
	}

	valid = service.compareAPIKeys(wrongKey, sha256Hashed)
	if valid {
		t.Error("Wrong key should not validate against SHA256 hash")
	}

	valid = service.compareAPIKeys(wrongKey, bcryptHashed)
	if valid {
		t.Error("Wrong key should not validate against bcrypt hash")
	}

	valid = service.compareAPIKeys(wrongKey, argon2idHashed)
	if valid {
		t.Error("Wrong key should not validate against Argon2id hash")
	}

	// Test empty key scenarios
	valid = service.compareAPIKeys("", sha256Hashed)
	if valid {
		t.Error("Empty key should not validate")
	}

	valid = service.compareAPIKeys(plainKey1, "")
	if valid {
		t.Error("Key should not validate against empty hash")
	}
}

func TestMixedAPIKeyFormatsValidationWithHashingDisabled(t *testing.T) {
	// Test mixed formats when hashing is disabled
	service := &APIKeyService{
		hashingConfig: &config.APIKeyHashingConfig{
			Enabled: false,
		},
	}

	plainKey := "apip_test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Simulate pre-existing hashed keys from when hashing was enabled
	sha256Hash := "$sha256$73616c74$abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	bcryptHash := "$2a$12$abcdefghijklmnopqrstuv.abcdefghijklmnopqrstuv.abcdefghijklmnopqr"
	argon2idHash := "$argon2id$v=19$m=65536,t=1,p=4$c2FsdA$aGFzaEhhc2hIYXNoSGFzaEhhc2hIYXNoSGFzaEhhc2g"

	// With hashing disabled, only plain text comparison should work
	valid := service.compareAPIKeys(plainKey, plainKey)
	if !valid {
		t.Error("Plain key should validate against itself when hashing is disabled")
	}

	// Hashed keys should not validate when hashing is disabled
	valid = service.compareAPIKeys(plainKey, sha256Hash)
	if valid {
		t.Error("Plain key should not validate against SHA256 hash when hashing is disabled")
	}

	valid = service.compareAPIKeys(plainKey, bcryptHash)
	if valid {
		t.Error("Plain key should not validate against bcrypt hash when hashing is disabled")
	}

	valid = service.compareAPIKeys(plainKey, argon2idHash)
	if valid {
		t.Error("Plain key should not validate against Argon2id hash when hashing is disabled")
	}

	// Test that we can still process plain text keys
	result, err := service.hashAPIKey(plainKey)
	if err != nil {
		t.Fatalf("Should be able to process plain key when hashing is disabled: %v", err)
	}

	if result != plainKey {
		t.Error("With hashing disabled, should return the original plain key")
	}
}

func TestHashingConfigurationGetSet(t *testing.T) {
	// Initialize service with a default configuration
	defaultHashingConfig := &config.APIKeyHashingConfig{
		Enabled:   false,
		Algorithm: "",
	}
	service := &APIKeyService{
		hashingConfig: defaultHashingConfig,
	}

	// Test default configuration
	defaultConfig := service.GetHashingConfig()
	if defaultConfig.Enabled != false {
		t.Error("Default hashing config should be disabled")
	}

	// Test setting configuration
	newConfig := config.APIKeyHashingConfig{
		Enabled:   true,
		Algorithm: constants.HashingAlgorithmArgon2ID,
	}
	service.SetHashingConfig(&newConfig)

	retrievedConfig := service.GetHashingConfig()
	if retrievedConfig.Enabled != newConfig.Enabled {
		t.Error("Hashing config enabled flag should match")
	}
	if retrievedConfig.Algorithm != newConfig.Algorithm {
		t.Error("Hashing config algorithm should match")
	}
}

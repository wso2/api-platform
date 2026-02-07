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
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestNewAPIKeyService(t *testing.T) {
	store := storage.NewConfigStore()
	apiKeyConfig := &config.APIKeyConfig{
		APIKeysPerUserPerAPI: 10,
		Algorithm:            constants.HashingAlgorithmSHA256,
	}

	service := NewAPIKeyService(store, nil, nil, apiKeyConfig)
	assert.NotNil(t, service)
	assert.Equal(t, store, service.store)
	assert.Equal(t, apiKeyConfig, service.apiKeyConfig)
}

func TestAPIKeyGenerationParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	user := &commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"developer"},
	}

	params := APIKeyCreationParams{
		Handle:        "test-api-handle",
		Request:       api.APIKeyCreationRequest{},
		User:          user,
		CorrelationID: "corr-123",
		Logger:        logger,
	}

	assert.Equal(t, "test-api-handle", params.Handle)
	assert.Equal(t, "test-user", params.User.UserID)
	assert.Equal(t, "corr-123", params.CorrelationID)
}

func TestAPIKeyRevocationParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	user := &commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"developer"},
	}

	params := APIKeyRevocationParams{
		Handle:        "test-api-handle",
		APIKeyName:    "my-api-key",
		User:          user,
		CorrelationID: "corr-456",
		Logger:        logger,
	}

	assert.Equal(t, "test-api-handle", params.Handle)
	assert.Equal(t, "my-api-key", params.APIKeyName)
	assert.Equal(t, "test-user", params.User.UserID)
}

func TestAPIKeyRegenerationParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	user := &commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"developer"},
	}

	params := APIKeyRegenerationParams{
		Handle:        "test-api-handle",
		APIKeyName:    "my-api-key",
		Request:       api.APIKeyRegenerationRequest{},
		User:          user,
		CorrelationID: "corr-789",
		Logger:        logger,
	}

	assert.Equal(t, "test-api-handle", params.Handle)
	assert.Equal(t, "my-api-key", params.APIKeyName)
	assert.Equal(t, "test-user", params.User.UserID)
}

func TestListAPIKeyParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	user := &commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	}

	params := ListAPIKeyParams{
		Handle:        "test-api-handle",
		User:          user,
		CorrelationID: "corr-list",
		Logger:        logger,
	}

	assert.Equal(t, "test-api-handle", params.Handle)
	assert.Equal(t, "test-user", params.User.UserID)
}

func TestParsedAPIKey(t *testing.T) {
	parsed := ParsedAPIKey{
		APIKey: "apip_test123",
		ID:     "key-id-123",
	}

	assert.Equal(t, "apip_test123", parsed.APIKey)
	assert.Equal(t, "key-id-123", parsed.ID)
}

func TestMaskAPIKey(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Long API key",
			input:    "apip_abcdef1234567890abcdef1234567890",
			expected: "apip_abcde*********",
		},
		{
			name:     "Short API key",
			input:    "short",
			expected: "**********",
		},
		{
			name:     "Exactly 10 characters",
			input:    "1234567890",
			expected: "**********",
		},
		{
			name:     "11 characters",
			input:    "12345678901",
			expected: "1234567890*********",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.MaskAPIKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateAPIKeyValue(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	t.Run("Generates key with correct prefix", func(t *testing.T) {
		key, err := service.generateAPIKeyValue()
		assert.NoError(t, err)
		assert.True(t, strings.HasPrefix(key, constants.APIKeyPrefix))
	})

	t.Run("Generates unique keys", func(t *testing.T) {
		key1, err := service.generateAPIKeyValue()
		assert.NoError(t, err)

		key2, err := service.generateAPIKeyValue()
		assert.NoError(t, err)

		assert.NotEqual(t, key1, key2)
	})

	t.Run("Key has correct length", func(t *testing.T) {
		key, err := service.generateAPIKeyValue()
		assert.NoError(t, err)
		// apip_ prefix (5) + 32 bytes as hex (64)
		expectedLen := len(constants.APIKeyPrefix) + (constants.APIKeyLen * 2)
		assert.Len(t, key, expectedLen)
	})
}

func TestGenerateShortUniqueID(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	t.Run("Generates 22 character ID", func(t *testing.T) {
		id, err := service.generateShortUniqueID()
		assert.NoError(t, err)
		assert.Len(t, id, 22)
	})

	t.Run("Generates unique IDs", func(t *testing.T) {
		id1, err := service.generateShortUniqueID()
		assert.NoError(t, err)

		id2, err := service.generateShortUniqueID()
		assert.NoError(t, err)

		assert.NotEqual(t, id1, id2)
	})

	t.Run("ID does not contain underscores", func(t *testing.T) {
		// Generate multiple IDs to increase chance of catching underscores
		for i := 0; i < 100; i++ {
			id, err := service.generateShortUniqueID()
			assert.NoError(t, err)
			assert.NotContains(t, id, "_")
		}
	})
}

func TestIsAdmin(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	tests := []struct {
		name     string
		user     *commonmodels.AuthContext
		expected bool
	}{
		{
			name:     "User with admin role",
			user:     &commonmodels.AuthContext{UserID: "admin-user", Roles: []string{"admin"}},
			expected: true,
		},
		{
			name:     "User with admin and other roles",
			user:     &commonmodels.AuthContext{UserID: "admin-user", Roles: []string{"developer", "admin", "viewer"}},
			expected: true,
		},
		{
			name:     "User without admin role",
			user:     &commonmodels.AuthContext{UserID: "dev-user", Roles: []string{"developer"}},
			expected: false,
		},
		{
			name:     "User with no roles",
			user:     &commonmodels.AuthContext{UserID: "no-role-user", Roles: []string{}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isAdmin(tt.user)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsDeveloper(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	tests := []struct {
		name     string
		user     *commonmodels.AuthContext
		expected bool
	}{
		{
			name:     "User with developer role",
			user:     &commonmodels.AuthContext{UserID: "dev-user", Roles: []string{"developer"}},
			expected: true,
		},
		{
			name:     "User without developer role",
			user:     &commonmodels.AuthContext{UserID: "admin-user", Roles: []string{"admin"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isDeveloper(tt.user)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCanRevokeAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	t.Run("Admin can revoke any key", func(t *testing.T) {
		user := &commonmodels.AuthContext{UserID: "admin-user", Roles: []string{"admin"}}
		apiKey := &models.APIKey{Name: "test-key", CreatedBy: "other-user"}

		err := service.canRevokeAPIKey(user, apiKey, logger)
		assert.NoError(t, err)
	})

	t.Run("Creator can revoke own key", func(t *testing.T) {
		user := &commonmodels.AuthContext{UserID: "creator-user", Roles: []string{"developer"}}
		apiKey := &models.APIKey{Name: "test-key", CreatedBy: "creator-user"}

		err := service.canRevokeAPIKey(user, apiKey, logger)
		assert.NoError(t, err)
	})

	t.Run("Non-creator non-admin cannot revoke key", func(t *testing.T) {
		user := &commonmodels.AuthContext{UserID: "other-user", Roles: []string{"developer"}}
		apiKey := &models.APIKey{Name: "test-key", CreatedBy: "creator-user"}

		err := service.canRevokeAPIKey(user, apiKey, logger)
		assert.Error(t, err)
	})

	t.Run("Nil user returns error", func(t *testing.T) {
		apiKey := &models.APIKey{Name: "test-key", CreatedBy: "creator-user"}

		err := service.canRevokeAPIKey(nil, apiKey, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user authentication required")
	})

	t.Run("Nil API key returns error", func(t *testing.T) {
		user := &commonmodels.AuthContext{UserID: "user", Roles: []string{"developer"}}

		err := service.canRevokeAPIKey(user, nil, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key not found")
	})
}

func TestCanRegenerateAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	t.Run("Creator can regenerate own key", func(t *testing.T) {
		user := &commonmodels.AuthContext{UserID: "creator-user", Roles: []string{"developer"}}
		apiKey := &models.APIKey{Name: "test-key", CreatedBy: "creator-user"}

		err := service.canRegenerateAPIKey(user, apiKey, logger)
		assert.NoError(t, err)
	})

	t.Run("Non-creator cannot regenerate key", func(t *testing.T) {
		user := &commonmodels.AuthContext{UserID: "other-user", Roles: []string{"admin"}}
		apiKey := &models.APIKey{Name: "test-key", CreatedBy: "creator-user"}

		err := service.canRegenerateAPIKey(user, apiKey, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "only the creator")
	})

	t.Run("Nil user returns error", func(t *testing.T) {
		apiKey := &models.APIKey{Name: "test-key", CreatedBy: "creator-user"}

		err := service.canRegenerateAPIKey(nil, apiKey, logger)
		assert.Error(t, err)
	})

	t.Run("Nil API key returns error", func(t *testing.T) {
		user := &commonmodels.AuthContext{UserID: "user", Roles: []string{"developer"}}

		err := service.canRegenerateAPIKey(user, nil, logger)
		assert.Error(t, err)
	})
}

func TestFilterAPIKeysByUser(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	apiKeys := []*models.APIKey{
		{Name: "key1", CreatedBy: "user1"},
		{Name: "key2", CreatedBy: "user2"},
		{Name: "key3", CreatedBy: "user1"},
		{Name: "key4", CreatedBy: "user3"},
	}

	t.Run("Admin sees all keys", func(t *testing.T) {
		user := &commonmodels.AuthContext{UserID: "admin-user", Roles: []string{"admin"}}

		result, err := service.filterAPIKeysByUser(user, apiKeys, logger)
		assert.NoError(t, err)
		assert.Len(t, result, 4)
	})

	t.Run("Regular user sees only own keys", func(t *testing.T) {
		user := &commonmodels.AuthContext{UserID: "user1", Roles: []string{"developer"}}

		result, err := service.filterAPIKeysByUser(user, apiKeys, logger)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		for _, key := range result {
			assert.Equal(t, "user1", key.CreatedBy)
		}
	})

	t.Run("Nil user returns error", func(t *testing.T) {
		result, err := service.filterAPIKeysByUser(nil, apiKeys, logger)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Empty key list returns empty result", func(t *testing.T) {
		user := &commonmodels.AuthContext{UserID: "user1", Roles: []string{"developer"}}

		result, err := service.filterAPIKeysByUser(user, []*models.APIKey{}, logger)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestGenerateOperationsString(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	t.Run("Empty operations returns wildcard", func(t *testing.T) {
		result := service.generateOperationsString([]api.Operation{})
		assert.Equal(t, "[\"*\"]", result)
	})

	t.Run("Single operation", func(t *testing.T) {
		ops := []api.Operation{
			{Method: "GET", Path: "/users"},
		}
		result := service.generateOperationsString(ops)
		assert.Contains(t, result, "GET /users")
	})

	t.Run("Multiple operations", func(t *testing.T) {
		ops := []api.Operation{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/users"},
		}
		result := service.generateOperationsString(ops)
		assert.Contains(t, result, "GET /users")
		assert.Contains(t, result, "POST /users")
	})
}

func TestBuildAPIKeyResponse(t *testing.T) {
	service := &APIKeyService{
		store: storage.NewConfigStore(),
		apiKeyConfig: &config.APIKeyConfig{
			APIKeysPerUserPerAPI: 10,
			Algorithm:            constants.HashingAlgorithmSHA256,
		},
	}

	t.Run("Nil API key returns error response", func(t *testing.T) {
		response := service.buildAPIKeyResponse(nil, "test-handle", "", false)
		assert.Equal(t, "error", response.Status)
		assert.Equal(t, "API key is nil", response.Message)
	})

	t.Run("Valid API key returns success response", func(t *testing.T) {
		apiKey := &models.APIKey{
			ID:         "key-id-123",
			Name:       "my-test-key",
			APIKey:     "$sha256$salt$hash",
			APIId:      "api-id-123",
			Operations: "[\"*\"]",
			Status:     models.APIKeyStatusActive,
			CreatedAt:  time.Now(),
			CreatedBy:  "test-user",
		}
		plainKey := "apip_plain123456789"

		response := service.buildAPIKeyResponse(apiKey, "test-handle", plainKey, false)
		assert.Equal(t, "success", response.Status)
		assert.NotNil(t, response.ApiKey)
		assert.Equal(t, "my-test-key", response.ApiKey.Name)
	})

	t.Run("Without plain key does not expose hashed key", func(t *testing.T) {
		apiKey := &models.APIKey{
			ID:         "key-id-123",
			Name:       "my-test-key",
			APIKey:     "$sha256$salt$hash",
			APIId:      "api-id-123",
			Operations: "[\"*\"]",
			Status:     models.APIKeyStatusActive,
			CreatedAt:  time.Now(),
			CreatedBy:  "test-user",
		}

		response := service.buildAPIKeyResponse(apiKey, "test-handle", "", false)
		assert.Equal(t, "success", response.Status)
		assert.NotNil(t, response.ApiKey)
		assert.Nil(t, response.ApiKey.ApiKey) // Should not expose hashed key
	})
}

func TestDecodeBase64(t *testing.T) {
	t.Run("Decodes RawStdEncoding", func(t *testing.T) {
		encoded := "SGVsbG8" // "Hello" in base64 raw
		result, err := decodeBase64(encoded)
		assert.NoError(t, err)
		assert.Equal(t, "Hello", string(result))
	})

	t.Run("Decodes StdEncoding with padding", func(t *testing.T) {
		encoded := "SGVsbG8=" // "Hello" in base64 with padding
		result, err := decodeBase64(encoded)
		assert.NoError(t, err)
		assert.Equal(t, "Hello", string(result))
	})
}

func TestCompareSHA256Hash(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	t.Run("Empty inputs return false", func(t *testing.T) {
		result := service.compareSHA256Hash("", "hash")
		assert.False(t, result)

		result = service.compareSHA256Hash("key", "")
		assert.False(t, result)
	})

	t.Run("Invalid format returns false", func(t *testing.T) {
		result := service.compareSHA256Hash("key", "not-a-valid-hash")
		assert.False(t, result)
	})

	t.Run("Valid hash comparison works", func(t *testing.T) {
		plainKey := "apip_test123456789"
		hash, err := service.hashAPIKeyWithSHA256(plainKey)
		require.NoError(t, err)

		result := service.compareSHA256Hash(plainKey, hash)
		assert.True(t, result)
	})
}

func TestCompareBcryptHash(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmBcrypt,
		},
	}

	t.Run("Empty inputs return false", func(t *testing.T) {
		result := service.compareBcryptHash("", "hash")
		assert.False(t, result)

		result = service.compareBcryptHash("key", "")
		assert.False(t, result)
	})

	t.Run("Valid hash comparison works", func(t *testing.T) {
		plainKey := "apip_test123456789"
		hash, err := service.hashAPIKeyWithBcrypt(plainKey)
		require.NoError(t, err)

		result := service.compareBcryptHash(plainKey, hash)
		assert.True(t, result)
	})
}

func TestCompareArgon2id(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmArgon2ID,
		},
	}

	t.Run("Invalid format returns error", func(t *testing.T) {
		err := service.compareArgon2id("key", "not-valid")
		assert.Error(t, err)
	})

	t.Run("Valid hash comparison works", func(t *testing.T) {
		plainKey := "apip_test123456789"
		hash, err := service.hashAPIKeyWithArgon2ID(plainKey)
		require.NoError(t, err)

		err = service.compareArgon2id(plainKey, hash)
		assert.NoError(t, err)
	})
}

func TestCompareAPIKeys(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	t.Run("Empty inputs return false", func(t *testing.T) {
		result := service.compareAPIKeys("", "hash")
		assert.False(t, result)

		result = service.compareAPIKeys("key", "")
		assert.False(t, result)
	})

	t.Run("SHA256 hash detected and validated", func(t *testing.T) {
		plainKey := "apip_test123"
		hash, _ := service.hashAPIKeyWithSHA256(plainKey)

		result := service.compareAPIKeys(plainKey, hash)
		assert.True(t, result)
	})

	t.Run("Bcrypt hash detected and validated", func(t *testing.T) {
		plainKey := "apip_test123"
		hash, _ := service.hashAPIKeyWithBcrypt(plainKey)

		result := service.compareAPIKeys(plainKey, hash)
		assert.True(t, result)
	})

	t.Run("Argon2id hash detected and validated", func(t *testing.T) {
		plainKey := "apip_test123"
		hash, _ := service.hashAPIKeyWithArgon2ID(plainKey)

		result := service.compareAPIKeys(plainKey, hash)
		assert.True(t, result)
	})

	t.Run("Plain text fallback comparison", func(t *testing.T) {
		result := service.compareAPIKeys("same-key", "same-key")
		assert.True(t, result)

		result = service.compareAPIKeys("key1", "key2")
		assert.False(t, result)
	})
}

func TestHashAPIKey_EmptyKey(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	_, err := service.hashAPIKey("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestHashAPIKeyWithSHA256_EmptyKey(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	_, err := service.hashAPIKeyWithSHA256("")
	assert.Error(t, err)
}

func TestHashAPIKeyWithBcrypt_EmptyKey(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmBcrypt,
		},
	}

	_, err := service.hashAPIKeyWithBcrypt("")
	assert.Error(t, err)
}

func TestHashAPIKeyWithArgon2ID_EmptyKey(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmArgon2ID,
		},
	}

	_, err := service.hashAPIKeyWithArgon2ID("")
	assert.Error(t, err)
}

// Additional tests for uncovered lines

// Tests for expiration handling in createAPIKeyFromRequest
func TestCreateAPIKeyFromRequest_Expiration_AllUnits(t *testing.T) {
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			APIKeysPerUserPerAPI: 10,
			Algorithm:            constants.HashingAlgorithmSHA256,
		},
	}

	apiConfig := &models.StoredConfig{
		ID:   "test-api",
		Kind: "Api",
		Configuration: api.APIConfiguration{
			Metadata: api.Metadata{Name: "test-api"},
			Spec:     api.APIConfiguration_Spec{},
		},
	}

	t.Run("seconds unit", func(t *testing.T) {
		name := "key1"
		req := &api.APIKeyCreationRequest{
			Name: &name,
			ExpiresIn: &struct {
				Duration int                                    `json:"duration" yaml:"duration"`
				Unit     api.APIKeyCreationRequestExpiresInUnit `json:"unit" yaml:"unit"`
			}{
				Duration: 3600,
				Unit:     api.APIKeyCreationRequestExpiresInUnitSeconds,
			},
		}
		key, err := service.createAPIKeyFromRequest("h1", req, "u1", apiConfig)
		assert.NoError(t, err)
		assert.NotNil(t, key.ExpiresAt)
	})

	t.Run("past expiration fails", func(t *testing.T) {
		name := "key2"
		past := time.Now().Add(-1 * time.Hour)
		req := &api.APIKeyCreationRequest{
			Name:      &name,
			ExpiresAt: &past,
		}
		_, err := service.createAPIKeyFromRequest("h1", req, "u1", apiConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be in the future")
	})
}

// Tests for regenerateAPIKey expiration handling
func TestRegenerateAPIKey_Expiration_AllPaths(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := &APIKeyService{
		apiKeyConfig: &config.APIKeyConfig{
			Algorithm: constants.HashingAlgorithmSHA256,
		},
	}

	t.Run("uses existing key duration when no request expiration", func(t *testing.T) {
		unit := "days"
		dur := 30
		existing := &models.APIKey{
			ID:        "k1",
			Name:      "n1",
			CreatedBy: "u1",
			Unit:      &unit,
			Duration:  &dur,
		}
		req := api.APIKeyRegenerationRequest{}
		key, err := service.regenerateAPIKey(existing, req, "u1", logger)
		assert.NoError(t, err)
		assert.NotNil(t, key.ExpiresAt)
	})

	t.Run("uses existing absolute expiry", func(t *testing.T) {
		exp := time.Now().Add(10 * 24 * time.Hour)
		existing := &models.APIKey{
			ID:        "k1",
			Name:      "n1",
			CreatedBy: "u1",
			ExpiresAt: &exp,
		}
		req := api.APIKeyRegenerationRequest{}
		key, err := service.regenerateAPIKey(existing, req, "u1", logger)
		assert.NoError(t, err)
		assert.Equal(t, exp, *key.ExpiresAt)
	})

	t.Run("no expiry when existing has none", func(t *testing.T) {
		existing := &models.APIKey{
			ID:        "k1",
			Name:      "n1",
			CreatedBy: "u1",
		}
		req := api.APIKeyRegenerationRequest{}
		key, err := service.regenerateAPIKey(existing, req, "u1", logger)
		assert.NoError(t, err)
		assert.Nil(t, key.ExpiresAt)
	})

	t.Run("past expiration fails", func(t *testing.T) {
		existing := &models.APIKey{
			ID:        "k1",
			Name:      "n1",
			CreatedBy: "u1",
		}
		past := time.Now().Add(-1 * time.Hour)
		req := api.APIKeyRegenerationRequest{
			ExpiresAt: &past,
		}
		_, err := service.regenerateAPIKey(existing, req, "u1", logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be in the future")
	})
}

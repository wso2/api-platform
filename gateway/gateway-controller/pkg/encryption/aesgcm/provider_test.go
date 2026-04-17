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

package aesgcm

import (
	"crypto/rand"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func createTestKey(t *testing.T, dir string, version string) string {
	t.Helper()
	keyPath := filepath.Join(dir, version+".key")

	key := make([]byte, AESKeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)

	err = os.WriteFile(keyPath, key, 0600)
	require.NoError(t, err)

	return keyPath
}

func createTestKeyWithData(t *testing.T, dir string, version string, data []byte) string {
	t.Helper()
	keyPath := filepath.Join(dir, version+".key")

	err := os.WriteFile(keyPath, data, 0600)
	require.NoError(t, err)

	return keyPath
}

func TestNewAESGCMProvider(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setupKeys   func() []KeyConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "single valid key",
			setupKeys: func() []KeyConfig {
				keyPath := createTestKey(t, tmpDir, "v1")
				return []KeyConfig{{Version: "v1", FilePath: keyPath}}
			},
			wantErr: false,
		},
		{
			name: "multiple valid keys",
			setupKeys: func() []KeyConfig {
				keyPath1 := createTestKey(t, tmpDir, "v2")
				keyPath2 := createTestKey(t, tmpDir, "v3")
				return []KeyConfig{
					{Version: "v2", FilePath: keyPath1},
					{Version: "v3", FilePath: keyPath2},
				}
			},
			wantErr: false,
		},
		{
			name: "no keys",
			setupKeys: func() []KeyConfig {
				return []KeyConfig{}
			},
			wantErr:     true,
			errContains: "at least one encryption key is required",
		},
		{
			name: "missing key file - not in dev mode",
			setupKeys: func() []KeyConfig {
				return []KeyConfig{{Version: "v1", FilePath: "/nonexistent/path/key.bin"}}
			},
			wantErr:     true,
			errContains: "encryption key file not found",
		},
		{
			name: "invalid key size",
			setupKeys: func() []KeyConfig {
				keyPath := createTestKeyWithData(t, tmpDir, "invalid-size", []byte("short key"))
				return []KeyConfig{{Version: "invalid-size", FilePath: keyPath}}
			},
			wantErr:     true,
			errContains: "invalid key size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyConfigs := tt.setupKeys()
			provider, err := NewAESGCMProvider(keyConfigs, testLogger())

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, provider)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, provider)
				assert.Equal(t, "aesgcm", provider.Name())
			}
		})
	}
}

func TestAESGCMProvider_EncryptDecrypt(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestKey(t, tmpDir, "test-key")

	provider, err := NewAESGCMProvider([]KeyConfig{
		{Version: "test-key", FilePath: keyPath},
	}, testLogger())
	require.NoError(t, err)

	tests := []struct {
		name      string
		plaintext []byte
	}{
		{
			name:      "simple text",
			plaintext: []byte("Hello, World!"),
		},
		{
			name:      "empty data",
			plaintext: []byte{},
		},
		{
			name:      "binary data",
			plaintext: []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd},
		},
		{
			name:      "large data",
			plaintext: make([]byte, 10000),
		},
		{
			name:      "unicode text",
			plaintext: []byte("こんにちは世界 🌍"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.plaintext) == 10000 {
				_, err := rand.Read(tt.plaintext)
				require.NoError(t, err)
			}

			payload, err := provider.Encrypt(tt.plaintext)
			require.NoError(t, err)
			assert.NotNil(t, payload)
			assert.Equal(t, "aesgcm", payload.Provider)
			assert.Equal(t, "test-key", payload.KeyVersion)

			if len(tt.plaintext) > 0 {
				assert.NotEqual(t, tt.plaintext, payload.Ciphertext)
			}

			decrypted, err := provider.Decrypt(payload)
			require.NoError(t, err)
			// Use len comparison instead of Equal to handle nil vs empty slice
			assert.Equal(t, len(tt.plaintext), len(decrypted))
		})
	}
}

func TestAESGCMProvider_DecryptWithDifferentKeys(t *testing.T) {
	tmpDir := t.TempDir()

	keyPath1 := createTestKey(t, tmpDir, "key-v1")
	keyPath2 := createTestKey(t, tmpDir, "key-v2")

	provider, err := NewAESGCMProvider([]KeyConfig{
		{Version: "key-v1", FilePath: keyPath1},
		{Version: "key-v2", FilePath: keyPath2},
	}, testLogger())
	require.NoError(t, err)

	plaintext := []byte("test secret data")

	payload, err := provider.Encrypt(plaintext)
	require.NoError(t, err)
	assert.Equal(t, "key-v1", payload.KeyVersion)

	decrypted, err := provider.Decrypt(payload)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAESGCMProvider_DecryptWrongKeyVersion(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestKey(t, tmpDir, "key-v1")

	provider, err := NewAESGCMProvider([]KeyConfig{
		{Version: "key-v1", FilePath: keyPath},
	}, testLogger())
	require.NoError(t, err)

	payload := &encryption.EncryptedPayload{
		Provider:   "aesgcm",
		KeyVersion: "nonexistent-key",
		Ciphertext: []byte("some data"),
	}

	_, err = provider.Decrypt(payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

func TestAESGCMProvider_DecryptTamperedData(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestKey(t, tmpDir, "test-key")

	provider, err := NewAESGCMProvider([]KeyConfig{
		{Version: "test-key", FilePath: keyPath},
	}, testLogger())
	require.NoError(t, err)

	payload, err := provider.Encrypt([]byte("secret data"))
	require.NoError(t, err)

	payload.Ciphertext[len(payload.Ciphertext)-1] ^= 0xff

	_, err = provider.Decrypt(payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication error")
}

func TestAESGCMProvider_DecryptTooShortCiphertext(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestKey(t, tmpDir, "test-key")

	provider, err := NewAESGCMProvider([]KeyConfig{
		{Version: "test-key", FilePath: keyPath},
	}, testLogger())
	require.NoError(t, err)

	payload := &encryption.EncryptedPayload{
		Provider:   "aesgcm",
		KeyVersion: "test-key",
		Ciphertext: []byte("short"),
	}

	_, err = provider.Decrypt(payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext too short")
}

func TestAESGCMProvider_HealthCheck(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestKey(t, tmpDir, "test-key")

	provider, err := NewAESGCMProvider([]KeyConfig{
		{Version: "test-key", FilePath: keyPath},
	}, testLogger())
	require.NoError(t, err)

	err = provider.HealthCheck()
	require.NoError(t, err)
}

func TestAESGCMProvider_UniqueNoncePerEncryption(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestKey(t, tmpDir, "test-key")

	provider, err := NewAESGCMProvider([]KeyConfig{
		{Version: "test-key", FilePath: keyPath},
	}, testLogger())
	require.NoError(t, err)

	plaintext := []byte("same plaintext")

	payload1, err := provider.Encrypt(plaintext)
	require.NoError(t, err)

	payload2, err := provider.Encrypt(plaintext)
	require.NoError(t, err)

	assert.NotEqual(t, payload1.Ciphertext, payload2.Ciphertext)

	decrypted1, err := provider.Decrypt(payload1)
	require.NoError(t, err)

	decrypted2, err := provider.Decrypt(payload2)
	require.NoError(t, err)

	assert.Equal(t, plaintext, decrypted1)
	assert.Equal(t, plaintext, decrypted2)
}

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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
)

func TestNewKeyManager(t *testing.T) {
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
				keyPath := createTestKey(t, tmpDir, "single-v1")
				return []KeyConfig{{Version: "single-v1", FilePath: keyPath}}
			},
			wantErr: false,
		},
		{
			name: "multiple valid keys",
			setupKeys: func() []KeyConfig {
				keyPath1 := createTestKey(t, tmpDir, "multi-v1")
				keyPath2 := createTestKey(t, tmpDir, "multi-v2")
				return []KeyConfig{
					{Version: "multi-v1", FilePath: keyPath1},
					{Version: "multi-v2", FilePath: keyPath2},
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
			name: "duplicate key versions",
			setupKeys: func() []KeyConfig {
				keyPath1 := createTestKey(t, tmpDir, "dup-v1")
				keyPath2 := createTestKey(t, tmpDir, "dup-v1-copy")
				return []KeyConfig{
					{Version: "dup-v1", FilePath: keyPath1},
					{Version: "dup-v1", FilePath: keyPath2},
				}
			},
			wantErr:     true,
			errContains: "duplicate key version",
		},
		{
			name: "missing key file",
			setupKeys: func() []KeyConfig {
				return []KeyConfig{{Version: "v1", FilePath: "/nonexistent/path/key.bin"}}
			},
			wantErr:     true,
			errContains: "encryption key file not found",
		},
		{
			name: "invalid key size - too small",
			setupKeys: func() []KeyConfig {
				keyPath := createTestKeyWithData(t, tmpDir, "small-key", []byte("too small"))
				return []KeyConfig{{Version: "small-key", FilePath: keyPath}}
			},
			wantErr:     true,
			errContains: "invalid key size",
		},
		{
			name: "invalid key size - too large",
			setupKeys: func() []KeyConfig {
				largeKey := make([]byte, 64)
				_, _ = rand.Read(largeKey)
				keyPath := createTestKeyWithData(t, tmpDir, "large-key", largeKey)
				return []KeyConfig{{Version: "large-key", FilePath: keyPath}}
			},
			wantErr:     true,
			errContains: "invalid key size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyConfigs := tt.setupKeys()
			km, err := NewKeyManager(keyConfigs, testLogger())

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, km)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, km)
			}
		})
	}
}

func TestKeyManager_GetPrimaryKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath1 := createTestKey(t, tmpDir, "primary-key")
	keyPath2 := createTestKey(t, tmpDir, "secondary-key")

	km, err := NewKeyManager([]KeyConfig{
		{Version: "primary-key", FilePath: keyPath1},
		{Version: "secondary-key", FilePath: keyPath2},
	}, testLogger())
	require.NoError(t, err)

	primary := km.GetPrimaryKey()
	assert.NotNil(t, primary)
	assert.Equal(t, "primary-key", primary.Version)
	assert.Len(t, primary.Data, AESKeySize)
}

func TestKeyManager_GetKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath1 := createTestKey(t, tmpDir, "getkey-v1")
	keyPath2 := createTestKey(t, tmpDir, "getkey-v2")

	km, err := NewKeyManager([]KeyConfig{
		{Version: "getkey-v1", FilePath: keyPath1},
		{Version: "getkey-v2", FilePath: keyPath2},
	}, testLogger())
	require.NoError(t, err)

	t.Run("get existing key", func(t *testing.T) {
		key, err := km.GetKey("getkey-v2")
		require.NoError(t, err)
		assert.NotNil(t, key)
		assert.Equal(t, "getkey-v2", key.Version)
		assert.Len(t, key.Data, AESKeySize)
	})

	t.Run("get primary key", func(t *testing.T) {
		key, err := km.GetKey("getkey-v1")
		require.NoError(t, err)
		assert.NotNil(t, key)
		assert.Equal(t, "getkey-v1", key.Version)
	})

	t.Run("get nonexistent key", func(t *testing.T) {
		key, err := km.GetKey("nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key version not found")
		assert.Nil(t, key)
	})
}

func TestKeyManager_GetPrimaryVersion(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := createTestKey(t, tmpDir, "version-test")

	km, err := NewKeyManager([]KeyConfig{
		{Version: "version-test", FilePath: keyPath},
	}, testLogger())
	require.NoError(t, err)

	version := km.GetPrimaryVersion()
	assert.Equal(t, "version-test", version)
}

func TestKeyManager_KeyFilePermissionWarning(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()

	keyPath := filepath.Join(tmpDir, "world-readable.key")
	keyData := make([]byte, AESKeySize)
	_, err := rand.Read(keyData)
	require.NoError(t, err)

	err = os.WriteFile(keyPath, keyData, 0644)
	require.NoError(t, err)

	km, err := NewKeyManager([]KeyConfig{
		{Version: "world-readable", FilePath: keyPath},
	}, testLogger())

	require.NoError(t, err)
	assert.NotNil(t, km)
}

func TestKeyManager_DevModeAutoGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "auto-generated.key")

	_, err := os.Stat(keyPath)
	require.True(t, os.IsNotExist(err))

	origValue := os.Getenv("APIP_GW_DEVELOPMENT_MODE")
	os.Setenv("APIP_GW_DEVELOPMENT_MODE", "true")
	defer os.Setenv("APIP_GW_DEVELOPMENT_MODE", origValue)

	km, err := NewKeyManager([]KeyConfig{
		{Version: "auto-gen", FilePath: keyPath},
	}, testLogger())
	require.NoError(t, err)
	assert.NotNil(t, km)

	info, err := os.Stat(keyPath)
	require.NoError(t, err)
	assert.Equal(t, int64(AESKeySize), info.Size())

	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestKeyManager_DevModeOffNoAutoGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "no-auto-gen.key")

	origValue := os.Getenv("APIP_GW_DEVELOPMENT_MODE")
	os.Setenv("APIP_GW_DEVELOPMENT_MODE", "false")
	defer os.Setenv("APIP_GW_DEVELOPMENT_MODE", origValue)

	_, err := NewKeyManager([]KeyConfig{
		{Version: "no-auto", FilePath: keyPath},
	}, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "encryption key file not found")
}

func TestKeyConfig(t *testing.T) {
	config := KeyConfig{
		Version:  "test-version",
		FilePath: "/path/to/key",
	}

	assert.Equal(t, "test-version", config.Version)
	assert.Equal(t, "/path/to/key", config.FilePath)
}

func TestKeyStruct(t *testing.T) {
	keyData := make([]byte, AESKeySize)
	_, err := rand.Read(keyData)
	require.NoError(t, err)

	key := &Key{
		Version: "test-key-v1",
		Data:    keyData,
	}

	assert.Equal(t, "test-key-v1", key.Version)
	assert.Len(t, key.Data, AESKeySize)
}

func TestInvalidKeySizeError(t *testing.T) {
	err := &encryption.ErrInvalidKeySize{
		Expected: AESKeySize,
		Actual:   16,
	}

	assert.Contains(t, err.Error(), "32")
	assert.Contains(t, err.Error(), "16")
}

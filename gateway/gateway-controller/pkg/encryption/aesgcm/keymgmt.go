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
	"fmt"
	"log/slog"
	"os"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
)

const (
	// AESKeySize is the required key size for AES-256 (32 bytes)
	AESKeySize = 32
)

// Key represents a single encryption key with its version
type Key struct {
	Version string
	Data    []byte
}

// KeyManager manages loading and accessing encryption keys
type KeyManager struct {
	keys           map[string]*Key // version -> key mapping
	primaryKey     *Key            // primary key for encryption (first key in config)
	primaryVersion string
	logger         *slog.Logger
}

// NewKeyManager creates a new key manager and loads keys from files
func NewKeyManager(keyConfigs []KeyConfig, logger *slog.Logger) (*KeyManager, error) {
	if len(keyConfigs) == 0 {
		return nil, fmt.Errorf("at least one encryption key is required")
	}

	km := &KeyManager{
		keys:   make(map[string]*Key),
		logger: logger,
	}

	// Load all keys
	for i, config := range keyConfigs {
		key, err := km.loadKey(config)
		if err != nil {
			return nil, fmt.Errorf("failed to load key %s: %w", config.Version, err)
		}

		km.keys[config.Version] = key

		// First key is the primary key for encryption
		if i == 0 {
			km.primaryKey = key
			km.primaryVersion = config.Version
		}

		logger.Debug("Loaded encryption key",
			slog.String("version", config.Version),
			slog.Bool("is_primary", i == 0),
		)
	}

	logger.Info("Key manager initialized",
		slog.Int("total_keys", len(km.keys)),
		slog.String("primary_version", km.primaryVersion),
	)

	return km, nil
}

// loadKey reads a key from a file and validates its size
func (km *KeyManager) loadKey(config KeyConfig) (*Key, error) {
	// Check file permissions for security
	info, err := os.Stat(config.FilePath)
	if err != nil {
		return nil, &encryption.ErrKeyNotFound{KeyPath: config.FilePath}
	}

	// Warn if key file is world-readable (security risk)
	perm := info.Mode().Perm()
	if perm&0004 != 0 {
		km.logger.Warn("Encryption key file is world-readable - consider restricting permissions",
			slog.String("key_version", config.Version),
			slog.String("file_path", config.FilePath),
			slog.String("permissions", perm.String()),
		)
	}

	// Read key data
	data, err := os.ReadFile(config.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Validate key size (must be exactly 32 bytes for AES-256)
	if len(data) != AESKeySize {
		return nil, &encryption.ErrInvalidKeySize{
			Expected: AESKeySize,
			Actual:   len(data),
		}
	}

	return &Key{
		Version: config.Version,
		Data:    data,
	}, nil
}

// GetPrimaryKey returns the primary encryption key
func (km *KeyManager) GetPrimaryKey() *Key {
	return km.primaryKey
}

// GetKey returns a specific key by version
func (km *KeyManager) GetKey(version string) (*Key, error) {
	key, exists := km.keys[version]
	if !exists {
		return nil, fmt.Errorf("key version not found: %s", version)
	}
	return key, nil
}

// GetPrimaryVersion returns the primary key version
func (km *KeyManager) GetPrimaryVersion() string {
	return km.primaryVersion
}

// KeyConfig holds configuration for a single key
type KeyConfig struct {
	Version  string
	FilePath string
}

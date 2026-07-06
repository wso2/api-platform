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

package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"platform-api/src/internal/utils"
	"platform-api/src/internal/vault"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests verify the demo-mode secret-encryption-key lifecycle end to end:
// when APIP_DEMO_MODE is on (the default) and no encryption key is configured,
// LoadConfig persists a generated key to DATABASE_SECRET_ENCRYPTION_KEY_FILE on
// first start and reloads it on subsequent starts, so a secret encrypted before
// a restart still decrypts after it. Only when the key file is unusable does
// demo mode fall back to an ephemeral per-process key (secrets then do not
// survive restarts). Decryption is exercised through the same vault the server
// uses, not just by comparing key strings.

const secretUnderTest = "sk-super-secret-value-123"

// newSecretVault derives the 32-byte AES-256 key from a config key string exactly
// as server.go does, and builds the in-house vault used for stored secrets.
func newSecretVault(t *testing.T, keyStr string) *vault.InHouseVault {
	t.Helper()
	key, err := utils.DeriveEncryptionKey(keyStr)
	require.NoError(t, err, "deriving AES key from config value %q", keyStr)
	v, err := vault.NewInHouseVault(key)
	require.NoError(t, err)
	return v
}

// TestDemoModeKeyFile_SecretSurvivesRestart verifies the demo-mode fix: with no
// key configured, the first LoadConfig generates and persists a key file, a
// second LoadConfig ("restart") reloads the same key, and a secret encrypted
// before the restart still decrypts after it.
func TestDemoModeKeyFile_SecretSurvivesRestart(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "secret-encryption.key")
	t.Setenv("APIP_DEMO_MODE", "true")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_SECRET_ENCRYPTION_KEY_FILE", keyFile)

	// First start: the key file is generated and the secret encrypted with it.
	cfg1, err := LoadConfig("")
	require.NoError(t, err)
	require.NotEmpty(t, cfg1.Database.SecretEncryptionKey)
	info, err := os.Stat(keyFile)
	require.NoError(t, err, "demo mode must persist the generated key file")
	assert.Equal(t, int64(32), info.Size())
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	ciphertext, err := newSecretVault(t, cfg1.Database.SecretEncryptionKey).
		Encrypt(context.Background(), secretUnderTest)
	require.NoError(t, err)

	// Restart: the second LoadConfig must reload the persisted key, not mint a
	// new one, so the pre-restart ciphertext still decrypts.
	cfg2, err := LoadConfig("")
	require.NoError(t, err)
	require.Equal(t, cfg1.Database.SecretEncryptionKey, cfg2.Database.SecretEncryptionKey,
		"demo mode must reload the persisted key file across restarts")

	plaintext, err := newSecretVault(t, cfg2.Database.SecretEncryptionKey).
		Decrypt(context.Background(), ciphertext)
	require.NoError(t, err,
		"a secret encrypted before restart must stay decryptable via the persisted demo-mode key")
	assert.Equal(t, secretUnderTest, plaintext)
}

// TestDemoModeUnusableKeyFile_FallsBackToEphemeralKey covers the fallback path:
// if the key file cannot be used (here: wrong size), demo mode still starts but
// mints a fresh ephemeral key per process, so secrets do not survive a restart.
func TestDemoModeUnusableKeyFile_FallsBackToEphemeralKey(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "secret-encryption.key")
	require.NoError(t, os.WriteFile(keyFile, []byte("short"), 0600),
		"seeding an invalid (wrong-size) key file")
	t.Setenv("APIP_DEMO_MODE", "true")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_SECRET_ENCRYPTION_KEY_FILE", keyFile)

	cfg1, err := LoadConfig("")
	require.NoError(t, err)
	vault1 := newSecretVault(t, cfg1.Database.SecretEncryptionKey)
	ciphertext, err := vault1.Encrypt(context.Background(), secretUnderTest)
	require.NoError(t, err)

	// Sanity: the same key round-trips, so the failure below is the key change.
	roundTrip, err := vault1.Decrypt(context.Background(), ciphertext)
	require.NoError(t, err)
	require.Equal(t, secretUnderTest, roundTrip)

	cfg2, err := LoadConfig("")
	require.NoError(t, err)
	require.NotEqual(t, cfg1.Database.SecretEncryptionKey, cfg2.Database.SecretEncryptionKey,
		"an unusable key file must fall back to a fresh ephemeral key per start")

	_, err = newSecretVault(t, cfg2.Database.SecretEncryptionKey).
		Decrypt(context.Background(), ciphertext)
	assert.Error(t, err,
		"with an ephemeral fallback key, secrets encrypted before restart are unreadable")
}

// TestStableConfiguredKey_SecretSurvivesRestart: when
// PLATFORM_SECRET_ENCRYPTION_KEY is configured, the key is stable across restarts
// and a secret encrypted before a restart still decrypts after it.
func TestStableConfiguredKey_SecretSurvivesRestart(t *testing.T) {
	const stableKey = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	t.Setenv("APIP_DEMO_MODE", "true")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", stableKey)
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_ENCRYPTION_KEY", "")

	cfg1, err := LoadConfig("")
	require.NoError(t, err)
	ciphertext, err := newSecretVault(t, cfg1.Database.SecretEncryptionKey).
		Encrypt(context.Background(), secretUnderTest)
	require.NoError(t, err)

	cfg2, err := LoadConfig("")
	require.NoError(t, err)
	require.Equal(t, cfg1.Database.SecretEncryptionKey, cfg2.Database.SecretEncryptionKey,
		"a configured key must be stable across restarts")

	plaintext, err := newSecretVault(t, cfg2.Database.SecretEncryptionKey).
		Decrypt(context.Background(), ciphertext)
	require.NoError(t, err,
		"a secret must stay decryptable across restart when a stable key is configured")
	assert.Equal(t, secretUnderTest, plaintext)
}

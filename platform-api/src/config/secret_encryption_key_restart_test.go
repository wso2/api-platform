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
	"testing"

	"platform-api/src/internal/utils"
	"platform-api/src/internal/vault"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests reproduce the demo-mode secret-encryption-key bug end to end: when
// APIP_DEMO_MODE is on (the default) and no encryption key is configured,
// LoadConfig mints a fresh random PLATFORM_SECRET_ENCRYPTION_KEY on every start
// (config.go generateRandomSecret). A secret encrypted before a restart is then
// encrypted with a key the next process no longer has, so it can never be
// decrypted again — silent data loss for already-stored secrets.
//
// The existing config tests prove the key is random per LoadConfig call; these go
// one step further and prove the *consequence* (decryption failure) through the
// same vault the server uses, and that a stable configured key avoids it.

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

// TestDemoModeEphemeralKey_BreaksSecretDecryptionAcrossRestart is the bug: a
// secret encrypted on the first start cannot be decrypted after a "restart"
// (a second LoadConfig), because the demo-mode ephemeral key changed.
func TestDemoModeEphemeralKey_BreaksSecretDecryptionAcrossRestart(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "true")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_ENCRYPTION_KEY", "")

	// First start: encrypt a secret with the key from the freshly loaded config.
	cfg1, err := LoadConfig("")
	require.NoError(t, err)
	vault1 := newSecretVault(t, cfg1.Database.SecretEncryptionKey)
	ciphertext, err := vault1.Encrypt(context.Background(), secretUnderTest)
	require.NoError(t, err)

	// Sanity: the same key round-trips, so the ciphertext itself is valid — this
	// isolates the failure below to the key change, not a bad ciphertext.
	roundTrip, err := vault1.Decrypt(context.Background(), ciphertext)
	require.NoError(t, err)
	require.Equal(t, secretUnderTest, roundTrip)

	// Restart: a second LoadConfig mints a different ephemeral key (root cause).
	cfg2, err := LoadConfig("")
	require.NoError(t, err)
	require.NotEqual(t, cfg1.Database.SecretEncryptionKey, cfg2.Database.SecretEncryptionKey,
		"demo-mode must mint a new ephemeral key on each start")

	// The consequence: the pre-restart ciphertext no longer decrypts (AES-GCM
	// auth-tag mismatch under the new key).
	vault2 := newSecretVault(t, cfg2.Database.SecretEncryptionKey)
	_, err = vault2.Decrypt(context.Background(), ciphertext)
	assert.Error(t, err,
		"BUG: a secret encrypted before restart must not decrypt with the new demo-mode "+
			"ephemeral key — already-stored secrets become permanently unreadable after restart")
}

// TestStableConfiguredKey_SecretSurvivesRestart is the contrast / fix: when
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

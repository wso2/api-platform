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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TC-35: Missing PLATFORM_SECRET_ENCRYPTION_KEY with APIP_DEMO_MODE=true →
// server starts successfully with an auto-generated ephemeral key.
func TestLoadConfig_MissingSecretEncryptionKey_DemoMode_GeneratesEphemeralKey(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "true")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	// Ensure the koanf env-var alias doesn't accidentally provide a value.
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_ENCRYPTION_KEY", "")
	t.Setenv("AUTH_JWT_SECRET_KEY", "")

	cfg, err := LoadConfig("")
	require.NoError(t, err, "LoadConfig must succeed in DEMO_MODE even without a secret encryption key")
	assert.NotEmpty(t, cfg.Database.SecretEncryptionKey,
		"an ephemeral key must be generated when PLATFORM_SECRET_ENCRYPTION_KEY is absent in DEMO_MODE")
}

// TC-35 (negative): Missing key WITHOUT demo mode → fatal error returned.
// JWT auth is disabled so the JWT-key check doesn't fire before the encryption-key check.
func TestLoadConfig_MissingSecretEncryptionKey_NonDemoMode_ReturnsError(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "false")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_ENCRYPTION_KEY", "")
	t.Setenv("AUTH_JWT_SECRET_KEY", "")
	t.Setenv("AUTH_JWT_ENABLED", "false")
	// Use a blocking file as parent so the key file path resolves but can't be
	// created, ensuring LoadConfig reaches the missing-key error path.
	blockingFile := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(blockingFile, []byte("block"), 0600))
	t.Setenv("DATABASE_SECRET_ENCRYPTION_KEY_FILE", filepath.Join(blockingFile, "secret-encryption.key"))

	_, err := LoadConfig("")
	assert.Error(t, err, "LoadConfig must return an error when no encryption key is configured and DEMO_MODE is off")
	assert.Contains(t, err.Error(), "failed to load secret key file")
}

// Missing key with APIP_DEMO_MODE unset → demo mode is the default, so an
// ephemeral key is generated and LoadConfig succeeds.
func TestLoadConfig_MissingSecretEncryptionKey_UnsetDemoMode_DefaultsToDemo(t *testing.T) {
	os.Unsetenv("APIP_DEMO_MODE")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_ENCRYPTION_KEY", "")
	t.Setenv("AUTH_JWT_SECRET_KEY", "")

	cfg, err := LoadConfig("")
	require.NoError(t, err, "LoadConfig must succeed when APIP_DEMO_MODE is unset (demo is the default)")
	assert.NotEmpty(t, cfg.Database.SecretEncryptionKey,
		"an ephemeral key must be generated when no key is configured and APIP_DEMO_MODE is unset")
}

// TC-35: Ephemeral key must be unique each LoadConfig call (i.e. truly random, not a constant).
// Without an explicit key file path (e.g. postgres with no DATABASE_SECRET_ENCRYPTION_KEY_FILE and no DB path),
// no persistence is possible and the key is still ephemeral — each LoadConfig call produces a different value.
func TestLoadConfig_EphemeralKey_IsRandomPerCall_NoDatabasePath(t *testing.T) {
	// Use a temp file as the "parent directory" — os.MkdirAll can't create a
	// directory where a file already exists, so persistence always fails.
	blockingFile := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(blockingFile, []byte("block"), 0600))
	t.Setenv("DATABASE_SECRET_ENCRYPTION_KEY_FILE", filepath.Join(blockingFile, "secret-encryption.key"))
	t.Setenv("APIP_DEMO_MODE", "true")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_ENCRYPTION_KEY", "")
	t.Setenv("AUTH_JWT_SECRET_KEY", "")

	cfg1, err := LoadConfig("")
	require.NoError(t, err)
	cfg2, err := LoadConfig("")
	require.NoError(t, err)

	assert.NotEqual(t, cfg1.Database.SecretEncryptionKey, cfg2.Database.SecretEncryptionKey,
		"without a database path, ephemeral keys must differ between independent LoadConfig calls")
}

// With a key file path configured, the first LoadConfig generates and persists a 32-byte
// binary key file; subsequent calls load the same key — secrets survive restarts.
func TestLoadConfig_DemoMode_PersistsAndReloadsKey(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "secret-encryption.key")

	t.Setenv("APIP_DEMO_MODE", "true")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_ENCRYPTION_KEY", "")
	t.Setenv("AUTH_JWT_SECRET_KEY", "")
	t.Setenv("DATABASE_SECRET_ENCRYPTION_KEY_FILE", keyFile)

	cfg1, err := LoadConfig("")
	require.NoError(t, err)
	require.NotEmpty(t, cfg1.Database.SecretEncryptionKey)

	// Key file must be a 32-byte binary file.
	data, readErr := os.ReadFile(keyFile)
	require.NoError(t, readErr, "key file must exist after first LoadConfig")
	assert.Len(t, data, 32, "key file must contain exactly 32 bytes")

	// Second call must load the same key.
	cfg2, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, cfg1.Database.SecretEncryptionKey, cfg2.Database.SecretEncryptionKey,
		"second LoadConfig must reuse the persisted key so secrets remain readable after restart")
}

// TestLoadConfig_ExplicitSecretEncryptionKey verifies the normal path where the
// env var is set — no ephemeral generation occurs and the value is passed through.
func TestLoadConfig_ExplicitSecretEncryptionKey_UsedAsIs(t *testing.T) {
	const stableKey = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", stableKey)
	t.Setenv("APIP_DEMO_MODE", "false")
	t.Setenv("AUTH_JWT_ENABLED", "false")

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, stableKey, cfg.Database.SecretEncryptionKey)
}

// Ensure APIP_DEMO_MODE="1" is also accepted as a truthy value.
func TestLoadConfig_DemoModeOne_AcceptedAsTruthy(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "1")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.Database.SecretEncryptionKey)
}

// Ensure APIP_DEMO_MODE with surrounding whitespace is handled gracefully.
func TestLoadConfig_DemoModeWhitespace_Trimmed(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "  true  ")
	t.Setenv("PLATFORM_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("APIP_DATABASE_SECRET_ENCRYPTION_KEY", "")

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.Database.SecretEncryptionKey)
}

// cleanEnvForTest clears all environment variables that LoadConfig reads from the
// environment so each test starts from a known baseline.
func init() {
	// Clear vars that would leak from the host environment and break assertions.
	for _, v := range []string{
		"PLATFORM_SECRET_ENCRYPTION_KEY",
		"APIP_DATABASE_SECRET_ENCRYPTION_KEY",
		"APIP_DEMO_MODE",
	} {
		os.Unsetenv(v)
	}
}

// validateAuthModeExclusivity: IDP (JWKS) auth must not be enabled alongside the
// local JWT or file-based modes — the server must fail fast so operators turn the
// local modes off consciously and all tokens are validated against the IDP JWKS.
func TestValidateAuthModeExclusivity(t *testing.T) {
	tests := []struct {
		name    string
		auth    Auth
		wantErr bool
	}{
		{
			name:    "idp disabled — local modes allowed",
			auth:    Auth{IDP: IDP{Enabled: false}, JWT: JWT{Enabled: true}, FileBased: FileBased{Enabled: true}},
			wantErr: false,
		},
		{
			name:    "idp only",
			auth:    Auth{IDP: IDP{Enabled: true}, JWT: JWT{Enabled: false}, FileBased: FileBased{Enabled: false}},
			wantErr: false,
		},
		{
			name:    "idp and jwt both enabled",
			auth:    Auth{IDP: IDP{Enabled: true}, JWT: JWT{Enabled: true}, FileBased: FileBased{Enabled: false}},
			wantErr: true,
		},
		{
			name:    "idp and file_based both enabled",
			auth:    Auth{IDP: IDP{Enabled: true}, JWT: JWT{Enabled: false}, FileBased: FileBased{Enabled: true}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAuthModeExclusivity(&tt.auth)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

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
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A valid inline encryption key: 64 hex chars decoding to 32 bytes.
const validInlineKey = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

// clearKeyEnv resets all encryption-related env vars to empty so each test starts clean.
// t.Setenv restores the previous value automatically at test end.
func clearKeyEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_FILE", "")
	t.Setenv("DATABASE_DB_PATH", "")
	t.Setenv("APIP_DEMO_MODE", "")
}

// writeValidKeyFile writes a 32-byte binary key file and returns its path and the
// expected hex-encoded key value.
func writeValidKeyFile(t *testing.T, dir, name string) (path, hexKey string) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	path = filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, key, 0600))
	return path, hex.EncodeToString(key)
}

// setDemoDBPath points DATABASE_DB_PATH at a fresh temp file and returns the default
// key-file path (alongside the DB) that demo-mode resolution would use.
func setDemoDBPath(t *testing.T) (defaultKeyFile string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DATABASE_DB_PATH", filepath.Join(dir, "api_platform.db"))
	return filepath.Join(dir, "secret-encryption.key")
}

// --- Demo mode ---

// 1.i — Demo, neither provided, DB path present → a key is generated, persisted to the default
// key file, and reloaded (identical) on the next start.
func TestResolveKey_Demo_NeitherProvided_GeneratesAndPersists(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("APIP_DEMO_MODE", "true")
	keyFile := setDemoDBPath(t)

	cfg1, err := LoadConfig("")
	require.NoError(t, err)
	require.NotEmpty(t, cfg1.EncryptionKey)
	assert.Equal(t, keyFile, cfg1.EncryptionKeyFile, "key file path must default alongside the DB")

	data, readErr := os.ReadFile(keyFile)
	require.NoError(t, readErr, "key file must be created on first start")
	assert.Len(t, data, 32, "key file must contain exactly 32 bytes")

	cfg2, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, cfg1.EncryptionKey, cfg2.EncryptionKey,
		"the persisted key must be reloaded identically on restart")
}

// 1.i (edge) — Demo, neither provided, no DB path → falls back to an ephemeral key that differs
// per call (nothing to persist). Exercised directly since empty env values can't clear the
// default Database.Path (koanf skips empty env values).
func TestResolveKey_Demo_NeitherProvided_NoDBPath_Ephemeral(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "true")

	cfg1 := &Server{} // no EncryptionKey, no EncryptionKeyFile, empty Database.Path
	require.NoError(t, resolveEncryptionKey(cfg1))
	cfg2 := &Server{}
	require.NoError(t, resolveEncryptionKey(cfg2))

	require.NotEmpty(t, cfg1.EncryptionKey)
	assert.Empty(t, cfg1.EncryptionKeyFile, "no key file path can be derived without a DB path")
	assert.NotEqual(t, cfg1.EncryptionKey, cfg2.EncryptionKey,
		"without a persistable path, demo keys must be ephemeral and differ per call")
}

// 1.ii — Demo, only ENCRYPTION_KEY (valid) → used as-is; never written to the key file.
func TestResolveKey_Demo_InlineKeyValid_NotPersisted(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("APIP_DEMO_MODE", "true")
	keyFile := setDemoDBPath(t)
	t.Setenv("ENCRYPTION_KEY", validInlineKey)

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, validInlineKey, cfg.EncryptionKey)

	_, statErr := os.Stat(keyFile)
	assert.True(t, os.IsNotExist(statErr), "an inline key must never be written to the key file")
}

// 1.ii — Demo, only ENCRYPTION_KEY (invalid) → error, no fallback.
func TestResolveKey_Demo_InlineKeyInvalid_Errors(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("APIP_DEMO_MODE", "true")
	t.Setenv("ENCRYPTION_KEY", "not-a-valid-32-byte-key")

	_, err := LoadConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ENCRYPTION_KEY")
}

// 1.iii — Demo, only ENCRYPTION_KEY_FILE (valid) → read from file and used.
func TestResolveKey_Demo_KeyFileValid_Used(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("APIP_DEMO_MODE", "true")
	path, expected := writeValidKeyFile(t, t.TempDir(), "my.key")
	t.Setenv("ENCRYPTION_KEY_FILE", path)

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, expected, cfg.EncryptionKey)
}

// 1.iii — Demo, only ENCRYPTION_KEY_FILE (wrong size) → error, never auto-generated.
func TestResolveKey_Demo_KeyFileInvalidSize_Errors(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("APIP_DEMO_MODE", "true")
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.key")
	require.NoError(t, os.WriteFile(path, []byte("too-short"), 0600))
	t.Setenv("ENCRYPTION_KEY_FILE", path)

	_, err := LoadConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load ENCRYPTION_KEY_FILE")
}

// 1.iii — Demo, only ENCRYPTION_KEY_FILE (missing) → error, never auto-generated at that path.
func TestResolveKey_Demo_KeyFileMissing_Errors(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("APIP_DEMO_MODE", "true")
	t.Setenv("ENCRYPTION_KEY_FILE", filepath.Join(t.TempDir(), "does-not-exist.key"))

	_, err := LoadConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load ENCRYPTION_KEY_FILE")
}

// 1.iv / 2.iv — Both provided → error in demo mode.
func TestResolveKey_Demo_BothProvided_Errors(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("APIP_DEMO_MODE", "true")
	path, _ := writeValidKeyFile(t, t.TempDir(), "my.key")
	t.Setenv("ENCRYPTION_KEY", validInlineKey)
	t.Setenv("ENCRYPTION_KEY_FILE", path)

	_, err := LoadConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one of ENCRYPTION_KEY or ENCRYPTION_KEY_FILE")
}

// --- Non-demo (production) mode ---

// 2.i — Non-demo, neither provided → fatal error; never auto-generated.
func TestResolveKey_NonDemo_NeitherProvided_Errors(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("APIP_DEMO_MODE", "false")
	setDemoDBPath(t) // even with a DB path, non-demo must not generate.

	_, err := LoadConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no encryption key configured")
}

// 2.ii — Non-demo, only ENCRYPTION_KEY (valid) → used.
func TestResolveKey_NonDemo_InlineKeyValid_Used(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("APIP_DEMO_MODE", "false")
	t.Setenv("ENCRYPTION_KEY", validInlineKey)

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, validInlineKey, cfg.EncryptionKey)
}

// 2.ii — Non-demo, only ENCRYPTION_KEY (invalid) → error.
func TestResolveKey_NonDemo_InlineKeyInvalid_Errors(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("APIP_DEMO_MODE", "false")
	t.Setenv("ENCRYPTION_KEY", "short")

	_, err := LoadConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ENCRYPTION_KEY")
}

// 2.iii — Non-demo, only ENCRYPTION_KEY_FILE (valid) → read and used.
func TestResolveKey_NonDemo_KeyFileValid_Used(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("APIP_DEMO_MODE", "false")
	path, expected := writeValidKeyFile(t, t.TempDir(), "prod.key")
	t.Setenv("ENCRYPTION_KEY_FILE", path)

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, expected, cfg.EncryptionKey)
}

// 2.iv — Both provided → error in non-demo mode.
func TestResolveKey_NonDemo_BothProvided_Errors(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("APIP_DEMO_MODE", "false")
	path, _ := writeValidKeyFile(t, t.TempDir(), "prod.key")
	t.Setenv("ENCRYPTION_KEY", validInlineKey)
	t.Setenv("ENCRYPTION_KEY_FILE", path)

	_, err := LoadConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one of ENCRYPTION_KEY or ENCRYPTION_KEY_FILE")
}

// --- APIP_DEMO_MODE parsing ---

// APIP_DEMO_MODE unset → defaults to demo, so neither-provided generates a key.
func TestResolveKey_DemoModeUnset_DefaultsToDemo(t *testing.T) {
	clearKeyEnv(t)
	os.Unsetenv("APIP_DEMO_MODE")
	setDemoDBPath(t)

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.EncryptionKey)
}

// APIP_DEMO_MODE="1" and whitespace-padded values are treated as truthy (demo).
func TestResolveKey_DemoModeTruthyVariants(t *testing.T) {
	for _, v := range []string{"1", "  true  "} {
		t.Run(v, func(t *testing.T) {
			clearKeyEnv(t)
			t.Setenv("APIP_DEMO_MODE", v)
			setDemoDBPath(t)

			cfg, err := LoadConfig("")
			require.NoError(t, err)
			assert.NotEmpty(t, cfg.EncryptionKey)
		})
	}
}

// --- validEncryptionKey unit coverage ---

func TestValidEncryptionKey(t *testing.T) {
	require.True(t, validEncryptionKey(validInlineKey), "64 hex chars must be valid")
	// 32 bytes base64-encoded (standard encoding, 44 chars).
	require.True(t, validEncryptionKey("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="))
	require.False(t, validEncryptionKey(""), "empty must be invalid")
	require.False(t, validEncryptionKey("short"), "short strings must be invalid")
	require.False(t, validEncryptionKey("zz"+validInlineKey[2:]), "non-hex 64-char must be invalid")
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

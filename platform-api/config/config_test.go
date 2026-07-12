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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Valid 32-byte keys encoded as 64 hex chars.
const (
	validInlineKey = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	validJWTKey    = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
)

// setValidKeys provides both required keys so a test starts from a passing baseline.
// Individual tests override one of them to exercise the failure paths. t.Setenv restores
// the previous values automatically at test end.
func setValidKeys(t *testing.T) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY", validInlineKey)
	t.Setenv("AUTH_JWT_SECRET_KEY", validJWTKey)
	t.Setenv("APIP_DEMO_MODE", "")
}

// Both keys provided and valid → LoadConfig succeeds and passes the encryption key through.
func TestLoadConfig_ValidKeys_Succeeds(t *testing.T) {
	setValidKeys(t)

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, validInlineKey, cfg.EncryptionKey)
}

// ENCRYPTION_KEY is required and never generated — missing it fails startup (even in demo mode).
func TestLoadConfig_MissingEncryptionKey_Errors(t *testing.T) {
	setValidKeys(t)
	t.Setenv("APIP_DEMO_MODE", "true") // demo does not relax the requirement
	t.Setenv("ENCRYPTION_KEY", "")

	_, err := LoadConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ENCRYPTION_KEY is required")
}

// A provided ENCRYPTION_KEY must be an AES-256-sized key (64 hex / base64→32 bytes).
func TestLoadConfig_InvalidEncryptionKey_Errors(t *testing.T) {
	setValidKeys(t)
	t.Setenv("ENCRYPTION_KEY", "not-a-valid-32-byte-key")

	_, err := LoadConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ENCRYPTION_KEY")
}

// AUTH_JWT_SECRET_KEY is required (JWT auth is enabled by default) and never generated.
func TestLoadConfig_MissingJWTSecretKey_Errors(t *testing.T) {
	setValidKeys(t)
	t.Setenv("APIP_DEMO_MODE", "true") // demo does not relax the requirement
	t.Setenv("AUTH_JWT_SECRET_KEY", "")

	_, err := LoadConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AUTH_JWT_SECRET_KEY is required")
}

// A provided AUTH_JWT_SECRET_KEY must be an AES-256-sized key (64 hex / base64→32 bytes).
func TestLoadConfig_InvalidJWTSecretKey_Errors(t *testing.T) {
	setValidKeys(t)
	t.Setenv("AUTH_JWT_SECRET_KEY", "not-a-valid-32-byte-key")

	_, err := LoadConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid AUTH_JWT_SECRET_KEY")
}

// --- valid32ByteKey unit coverage ---

func TestValid32ByteKey(t *testing.T) {
	require.True(t, valid32ByteKey(validInlineKey), "64 hex chars must be valid")
	// 32 bytes base64-encoded (standard encoding, 44 chars).
	require.True(t, valid32ByteKey("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="))
	require.False(t, valid32ByteKey(""), "empty must be invalid")
	require.False(t, valid32ByteKey("short"), "short strings must be invalid")
	require.False(t, valid32ByteKey("zz"+validInlineKey[2:]), "non-hex 64-char must be invalid")
}

// Clear env vars that LoadConfig reads so each test starts from a known baseline and host
// environment values don't leak into assertions.
func init() {
	for _, v := range []string{
		"ENCRYPTION_KEY",
		"AUTH_JWT_SECRET_KEY",
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

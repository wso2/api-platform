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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Valid 32-byte keys encoded as 64 hex chars.
const (
	validInlineKey = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	validJWTKey    = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
)

// validKeysBase is a minimal config whose required secrets resolve from the
// APIP_CP_ENCRYPTION_KEY / APIP_CP_AUTH_JWT_SECRET_KEY env vars via {{ env }}
// interpolation. Environment variables reach config ONLY through these tokens now
// (there is no direct env-key override), so tests must go through a config file.
const validKeysBase = `
encryption_key = '{{ env "APIP_CP_ENCRYPTION_KEY" }}'

[auth.jwt]
enabled    = true
secret_key = '{{ env "APIP_CP_AUTH_JWT_SECRET_KEY" }}'
`

// loadTOML writes toml to a temp config file and loads it through LoadConfig.
func loadTOML(t *testing.T, toml string) (*Server, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(toml), 0o600))
	return LoadConfig(path)
}

// loadWithKeys sets both required secret env vars, appends extra TOML to the
// valid-keys base, and loads the result — a passing baseline for tests that then
// assert defaults, overrides, or validation of other fields.
func loadWithKeys(t *testing.T, extra string) (*Server, error) {
	t.Helper()
	t.Setenv("APIP_CP_ENCRYPTION_KEY", validInlineKey)
	t.Setenv("APIP_CP_AUTH_JWT_SECRET_KEY", validJWTKey)
	t.Setenv("APIP_DEMO_MODE", "")
	return loadTOML(t, validKeysBase+extra)
}

// Both keys provided and valid → LoadConfig succeeds and passes the encryption key through.
func TestLoadConfig_ValidKeys_Succeeds(t *testing.T) {
	cfg, err := loadWithKeys(t, "")
	require.NoError(t, err)
	assert.Equal(t, validInlineKey, cfg.EncryptionKey)
}

// The encryption key is required and never generated — a config that omits it fails
// startup (even in demo mode).
func TestLoadConfig_MissingEncryptionKey_Errors(t *testing.T) {
	t.Setenv("APIP_CP_AUTH_JWT_SECRET_KEY", validJWTKey)
	t.Setenv("APIP_DEMO_MODE", "true") // demo does not relax the requirement

	// Encryption key omitted entirely; the JWT secret resolves so the JWT check passes first.
	_, err := loadTOML(t, `
[auth.jwt]
enabled    = true
secret_key = '{{ env "APIP_CP_AUTH_JWT_SECRET_KEY" }}'
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EncryptionKey is required")
}

// A provided encryption key must be an AES-256-sized key (64 hex / base64→32 bytes).
func TestLoadConfig_InvalidEncryptionKey_Errors(t *testing.T) {
	t.Setenv("APIP_CP_AUTH_JWT_SECRET_KEY", validJWTKey)

	_, err := loadTOML(t, `
encryption_key = "not-a-valid-32-byte-key"

[auth.jwt]
enabled    = true
secret_key = '{{ env "APIP_CP_AUTH_JWT_SECRET_KEY" }}'
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid EncryptionKey")
}

// The JWT secret is required (JWT auth is enabled) and never generated.
func TestLoadConfig_MissingJWTSecretKey_Errors(t *testing.T) {
	t.Setenv("APIP_CP_ENCRYPTION_KEY", validInlineKey)
	t.Setenv("APIP_DEMO_MODE", "true") // demo does not relax the requirement

	_, err := loadTOML(t, `
encryption_key = '{{ env "APIP_CP_ENCRYPTION_KEY" }}'

[auth.jwt]
enabled = true
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Auth.JWT.SecretKey is required")
}

// A provided JWT secret must be an AES-256-sized key (64 hex / base64→32 bytes).
func TestLoadConfig_InvalidJWTSecretKey_Errors(t *testing.T) {
	t.Setenv("APIP_CP_ENCRYPTION_KEY", validInlineKey)

	_, err := loadTOML(t, `
encryption_key = '{{ env "APIP_CP_ENCRYPTION_KEY" }}'

[auth.jwt]
enabled    = true
secret_key = "not-a-valid-32-byte-key"
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Auth.JWT.SecretKey")
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
		"APIP_CP_ENCRYPTION_KEY",
		"APIP_CP_AUTH_JWT_SECRET_KEY",
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

// The HTTPS listener is on (and the plain-HTTP listener off) unless an operator
// explicitly opts otherwise, so a deployment that forgets the knob never
// silently downgrades to plain HTTP.
func TestLoadConfig_HTTPSEnabled_DefaultsToTrue(t *testing.T) {
	cfg, err := loadWithKeys(t, "")
	require.NoError(t, err)
	assert.True(t, cfg.HTTPS.Enabled, "https.enabled must default to true when unset")
	assert.Equal(t, "9243", cfg.HTTPS.Port, "https.port must default to 9243")
	assert.False(t, cfg.HTTP.Enabled, "http.enabled must default to false when unset")
}

// A {{ env }} token feeding a bool field must survive koanf's weakly-typed decode,
// so an operator can disable the TLS listener by pointing the field at an env var.
func TestLoadConfig_HTTPSEnabled_TokenDisables(t *testing.T) {
	t.Setenv("APIP_CP_HTTPS_ENABLED", "false")

	cfg, err := loadWithKeys(t, `
[https]
enabled = '{{ env "APIP_CP_HTTPS_ENABLED" }}'
`)
	require.NoError(t, err)
	assert.False(t, cfg.HTTPS.Enabled, "https.enabled from a {{ env }} token must decode to false")
}

// The plain-HTTP listener can be enabled independently on its own port via tokens.
func TestLoadConfig_HTTPListener_TokenEnables(t *testing.T) {
	t.Setenv("APIP_CP_HTTP_ENABLED", "true")
	t.Setenv("APIP_CP_HTTP_PORT", "9080")

	cfg, err := loadWithKeys(t, `
[http]
enabled = '{{ env "APIP_CP_HTTP_ENABLED" }}'
port    = '{{ env "APIP_CP_HTTP_PORT" }}'
`)
	require.NoError(t, err)
	assert.True(t, cfg.HTTP.Enabled, "http.enabled from a {{ env }} token must decode to true")
	assert.Equal(t, "9080", cfg.HTTP.Port)
}

// Listener timeouts must be finite by default, so a deployment that never sets
// them is still protected against a peer holding connections open (Slowloris).
func TestLoadConfig_Timeouts_DefaultToFiniteValues(t *testing.T) {
	cfg, err := loadWithKeys(t, "")
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, cfg.Timeouts.ReadHeader)
	assert.Equal(t, 60*time.Second, cfg.Timeouts.Read)
	assert.Equal(t, 120*time.Second, cfg.Timeouts.Write)
	assert.Equal(t, 120*time.Second, cfg.Timeouts.Idle)
}

// Duration strings resolved from {{ env }} tokens must decode into time.Duration fields.
func TestLoadConfig_Timeouts_TokenOverride(t *testing.T) {
	t.Setenv("APIP_CP_TIMEOUTS_READ_HEADER", "5s")
	t.Setenv("APIP_CP_TIMEOUTS_READ", "30s")
	t.Setenv("APIP_CP_TIMEOUTS_WRITE", "2m")
	t.Setenv("APIP_CP_TIMEOUTS_IDLE", "90s")

	cfg, err := loadWithKeys(t, `
[timeouts]
read_header = '{{ env "APIP_CP_TIMEOUTS_READ_HEADER" }}'
read        = '{{ env "APIP_CP_TIMEOUTS_READ" }}'
write       = '{{ env "APIP_CP_TIMEOUTS_WRITE" }}'
idle        = '{{ env "APIP_CP_TIMEOUTS_IDLE" }}'
`)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, cfg.Timeouts.ReadHeader)
	assert.Equal(t, 30*time.Second, cfg.Timeouts.Read)
	assert.Equal(t, 2*time.Minute, cfg.Timeouts.Write)
	assert.Equal(t, 90*time.Second, cfg.Timeouts.Idle)
}

// 0 is the net/http "no timeout" sentinel and must be accepted as-is, rather
// than being silently replaced by the default.
func TestLoadConfig_Timeouts_ZeroDisablesTimeout(t *testing.T) {
	cfg, err := loadWithKeys(t, `
[timeouts]
write = "0s"
`)
	require.NoError(t, err)
	assert.Zero(t, cfg.Timeouts.Write, "timeouts.write = 0 must disable the write timeout")
}

// A negative duration would expire immediately and break every request; a
// read_header bound above read would never be reached. Both must be rejected at
// load time rather than producing a server that fails at request time.
func TestLoadConfig_Timeouts_RejectsInvalidValues(t *testing.T) {
	t.Run("negative", func(t *testing.T) {
		_, err := loadWithKeys(t, `
[timeouts]
read = "-1s"
`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not be negative")
	})

	t.Run("read_header exceeds read", func(t *testing.T) {
		_, err := loadWithKeys(t, `
[timeouts]
read_header = "30s"
read        = "10s"
`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not exceed")
	})
}

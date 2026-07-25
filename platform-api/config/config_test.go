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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validInlineKey is a valid 32-byte AES key encoded as 64 hex chars, used for the
// at-rest encryption key (still a symmetric key — unrelated to JWT signing).
const validInlineKey = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

// RSA key-pair PEM fixtures for the asymmetric JWT config, generated once in init()
// and written to files, since JWT config now holds mounted-file paths rather than
// inline PEM content. validJWTPublicKeyFile/validJWTPrivateKeyFile form a matching
// pair; otherJWTPublicKeyFile is from a different pair, used to exercise the
// matching-pair validation.
var (
	validJWTPublicKeyFile  string
	validJWTPrivateKeyFile string
	otherJWTPublicKeyFile  string
)

// writePEMFile writes content to a fresh file under a package-lifetime temp
// directory and returns its path.
func writePEMFile(dir, name, content string) string {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		panic(err)
	}
	return path
}

// genRSAKeyPEMs generates a fresh RSA key pair and returns its public key (PKIX PEM)
// and private key (PKCS1 PEM).
func genRSAKeyPEMs() (pubPEM, privPEM string) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	privPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		panic(err)
	}
	pubPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}))
	return pubPEM, privPEM
}

// validKeysBase is a minimal config whose required secrets resolve from the
// APIP_CP_ENCRYPTION_KEY / APIP_CP_AUTH_JWT_PUBLIC_KEY_FILE env vars via {{ env }}
// interpolation. Environment variables reach config ONLY through these tokens now
// (there is no direct env-key override), so tests must go through a config file.
// The default auth mode is "external_token", which needs only the verification
// public key.
const validKeysBase = `
[platform_api.security]
encryption_key = '{{ env "APIP_CP_ENCRYPTION_KEY" }}'

[platform_api.auth.jwt]
public_key_file = '{{ env "APIP_CP_AUTH_JWT_PUBLIC_KEY_FILE" }}'
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
	t.Setenv("APIP_CP_AUTH_JWT_PUBLIC_KEY_FILE", validJWTPublicKeyFile)
	return loadTOML(t, validKeysBase+extra)
}

// Both keys provided and valid → LoadConfig succeeds and passes the encryption key through.
func TestLoadConfig_ValidKeys_Succeeds(t *testing.T) {
	cfg, err := loadWithKeys(t, "")
	require.NoError(t, err)
	assert.Equal(t, validInlineKey, cfg.Security.EncryptionKey)
}

// Issue #2835: the at-rest encryption key loads from a mounted file via a {{ file }}
// token — the mechanism the shipped config.toml now uses (setup.sh writes the key to
// /etc/platform-api/keys/encryption.key instead of an env var). The file is written
// with a trailing newline (as `openssl rand -hex 32 > file` produces); {{ file }} trims it.
func TestLoadConfig_EncryptionKeyFromFile_Succeeds(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "encryption.key")
	require.NoError(t, os.WriteFile(keyPath, []byte(validInlineKey+"\n"), 0o600))

	// Permit {{ file }} reads from the temp dir (default allowlist is /etc/platform-api,
	// /secrets/platform-api). Constant: configinterpolate.EnvFileSourceAllowlist.
	t.Setenv("APIP_CONFIG_FILE_SOURCE_ALLOWLIST", dir)
	// The JWT public-key path is an {{ env }} token (unaffected by the {{ file }} allowlist).
	t.Setenv("APIP_CP_AUTH_JWT_PUBLIC_KEY_FILE", validJWTPublicKeyFile)

	cfg, err := loadTOML(t, `
[platform_api.security]
encryption_key = '{{ file "`+keyPath+`" }}'

[platform_api.auth.jwt]
public_key_file = '{{ env "APIP_CP_AUTH_JWT_PUBLIC_KEY_FILE" }}'
`)
	require.NoError(t, err)
	assert.Equal(t, validInlineKey, cfg.Security.EncryptionKey)
}

// A merged multi-component config file also carries a foreign [developer_portal]
// section with its own interpolation tokens — here deliberately poisonous ones: an
// {{ env }} with no default that is left unset, and a {{ file }} path outside
// platform-api's allowlist. LoadConfig must interpolate and consume ONLY the
// [platform_api] subtree, leaving the foreign section (and its tokens) untouched.
// Guards the k.Cut(platformAPIConfigKey) scoping in LoadConfig: without it, the
// whole-tree interpolation would fail closed on these tokens.
func TestLoadConfig_IgnoresForeignComponentSection(t *testing.T) {
	// APIP_DP_SECURITY_ENCRYPTION_KEY is intentionally never set, and /etc/devportal
	// is not on platform-api's {{ file }} allowlist.
	cfg, err := loadWithKeys(t, `
[developer_portal.security]
encryption_key = '{{ env "APIP_DP_SECURITY_ENCRYPTION_KEY" }}'
[developer_portal.auth.local]
jwt_public_key = '{{ file "/etc/devportal/keys/jwt_public.pem" }}'
`)
	require.NoError(t, err)
	assert.Equal(t, validInlineKey, cfg.Security.EncryptionKey)
}

// The encryption key is required and never generated — a config that omits it fails startup.
func TestLoadConfig_MissingEncryptionKey_Errors(t *testing.T) {
	t.Setenv("APIP_CP_AUTH_JWT_PUBLIC_KEY_FILE", validJWTPublicKeyFile)

	// Encryption key omitted entirely; the JWT public key resolves so the JWT check passes first.
	_, err := loadTOML(t, `
[platform_api.auth.jwt]
public_key_file = '{{ env "APIP_CP_AUTH_JWT_PUBLIC_KEY_FILE" }}'
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EncryptionKey is required")
}

// A provided encryption key must be an AES-256-sized key (64 hex / base64→32 bytes).
func TestLoadConfig_InvalidEncryptionKey_Errors(t *testing.T) {
	t.Setenv("APIP_CP_AUTH_JWT_PUBLIC_KEY_FILE", validJWTPublicKeyFile)

	_, err := loadTOML(t, `
[platform_api.security]
encryption_key = "not-a-valid-32-byte-key"

[platform_api.auth.jwt]
public_key_file = '{{ env "APIP_CP_AUTH_JWT_PUBLIC_KEY_FILE" }}'
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid EncryptionKey")
}

// The JWT public key is required (default auth mode is "external_token") and never generated.
func TestLoadConfig_MissingJWTPublicKey_Errors(t *testing.T) {
	t.Setenv("APIP_CP_ENCRYPTION_KEY", validInlineKey)

	_, err := loadTOML(t, `
[platform_api.security]
encryption_key = '{{ env "APIP_CP_ENCRYPTION_KEY" }}'
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Auth.JWT.PublicKeyFile is required")
}

// A provided JWT public key file must contain a PEM-encoded RSA public key.
func TestLoadConfig_InvalidJWTPublicKey_Errors(t *testing.T) {
	t.Setenv("APIP_CP_ENCRYPTION_KEY", validInlineKey)
	invalidKeyFile := writePEMFile(t.TempDir(), "invalid_public.pem", "not-a-valid-rsa-public-key")

	_, err := loadTOML(t, `
[platform_api.security]
encryption_key = '{{ env "APIP_CP_ENCRYPTION_KEY" }}'

[platform_api.auth.jwt]
public_key_file = "`+invalidKeyFile+`"
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Auth.JWT.PublicKeyFile")
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
	// Generate the RSA key-pair fixtures used across the asymmetric JWT tests, and
	// write them to files since JWT config now holds mounted-file paths, not
	// inline PEM content.
	validJWTPublicKey, validJWTPrivateKey := genRSAKeyPEMs()
	otherJWTPublicKey, _ := genRSAKeyPEMs()

	keysDir, err := os.MkdirTemp("", "jwt-test-keys")
	if err != nil {
		panic(err)
	}
	validJWTPublicKeyFile = writePEMFile(keysDir, "valid_public.pem", validJWTPublicKey)
	validJWTPrivateKeyFile = writePEMFile(keysDir, "valid_private.pem", validJWTPrivateKey)
	otherJWTPublicKeyFile = writePEMFile(keysDir, "other_public.pem", otherJWTPublicKey)

	for _, v := range []string{
		"APIP_CP_ENCRYPTION_KEY",
		"APIP_CP_AUTH_JWT_PUBLIC_KEY_FILE",
		"APIP_CP_AUTH_JWT_PRIVATE_KEY_FILE",
	} {
		os.Unsetenv(v)
	}
}

// validateAuthConfig: auth.mode is a single discriminator, so exactly one mode
// is active and only that mode's section is validated. Unknown modes fail fast.
func TestValidateAuthConfig(t *testing.T) {
	tests := []struct {
		name    string
		auth    Auth
		wantErr string
	}{
		{
			name: "external_token mode with valid public key",
			auth: Auth{Mode: AuthModeExternalToken, JWT: JWT{PublicKeyFile: validJWTPublicKeyFile}},
		},
		{
			name:    "external_token mode without public key",
			auth:    Auth{Mode: AuthModeExternalToken},
			wantErr: "Auth.JWT.PublicKeyFile is required",
		},
		{
			name:    "file mode without private key",
			auth:    Auth{Mode: AuthModeFile, JWT: JWT{PublicKeyFile: validJWTPublicKeyFile, TokenTTL: time.Hour}},
			wantErr: "Auth.JWT.PrivateKeyFile is required",
		},
		{
			name: "file mode with mismatched key pair",
			auth: Auth{Mode: AuthModeFile, JWT: JWT{
				PublicKeyFile:  otherJWTPublicKeyFile,
				PrivateKeyFile: validJWTPrivateKeyFile,
				TokenTTL:       time.Hour,
			}},
			wantErr: "matching RSA key pair",
		},
		{
			name:    "file mode without token_ttl",
			auth:    Auth{Mode: AuthModeFile, JWT: JWT{PublicKeyFile: validJWTPublicKeyFile, PrivateKeyFile: validJWTPrivateKeyFile}},
			wantErr: "Auth.JWT.TokenTTL must be a positive duration",
		},
		{
			name: "file mode requires org and users",
			auth: Auth{Mode: AuthModeFile, JWT: JWT{PublicKeyFile: validJWTPublicKeyFile, PrivateKeyFile: validJWTPrivateKeyFile, TokenTTL: time.Hour}},
			// Default org fields are empty in a zero-value Auth — users check fires
			// after the org checks.
			wantErr: "auth.file.organization.id",
		},
		{
			name: "file mode fully configured",
			auth: Auth{
				Mode: AuthModeFile,
				JWT:  JWT{PublicKeyFile: validJWTPublicKeyFile, PrivateKeyFile: validJWTPrivateKeyFile, TokenTTL: time.Hour},
				File: FileBased{
					Organization: FileBasedOrg{ID: "default", DisplayName: "Default"},
					Users:        FileBasedUsers{{Username: "admin", PasswordHash: "$2a$12$hash"}},
				},
			},
		},
		{
			name:    "idp mode requires jwks_url",
			auth:    Auth{Mode: AuthModeIDP, IDP: IDP{ValidationMode: "scope"}},
			wantErr: "auth.idp.jwks_url",
		},
		{
			name: "idp mode fully configured",
			auth: Auth{Mode: AuthModeIDP, IDP: IDP{
				JWKSUrl:        "https://idp.example.com/jwks",
				Issuer:         []string{"https://idp.example.com"},
				ValidationMode: "scope",
			}},
		},
		{
			name:    "unknown mode rejected",
			auth:    Auth{Mode: "basic"},
			wantErr: "auth.mode must be",
		},
		{
			name:    "empty mode rejected",
			auth:    Auth{},
			wantErr: "auth.mode must be",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAuthConfig(&tt.auth)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
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
	assert.True(t, cfg.Listeners.HTTPS.Enabled, "server.https.enabled must default to true when unset")
	assert.Equal(t, 9243, cfg.Listeners.HTTPS.Port, "server.https.port must default to 9243")
	assert.False(t, cfg.Listeners.HTTP.Enabled, "server.http.enabled must default to false when unset")
	assert.Equal(t, "./data/certs/cert.pem", cfg.Listeners.HTTPS.CertFile)
	assert.Equal(t, "./data/certs/key.pem", cfg.Listeners.HTTPS.KeyFile)
}

// A {{ env }} token feeding a bool field must survive koanf's weakly-typed decode,
// so an operator can disable the TLS listener by pointing the field at an env var.
func TestLoadConfig_HTTPSEnabled_TokenDisables(t *testing.T) {
	t.Setenv("APIP_CP_SERVER_HTTPS_ENABLED", "false")

	cfg, err := loadWithKeys(t, `
[platform_api.server.https]
enabled = '{{ env "APIP_CP_SERVER_HTTPS_ENABLED" }}'
`)
	require.NoError(t, err)
	assert.False(t, cfg.Listeners.HTTPS.Enabled, "server.https.enabled from a {{ env }} token must decode to false")
}

// The plain-HTTP listener can be enabled independently on its own port via tokens;
// a numeric string from an env var must decode into the int port field.
func TestLoadConfig_HTTPListener_TokenEnables(t *testing.T) {
	t.Setenv("APIP_CP_SERVER_HTTP_ENABLED", "true")
	t.Setenv("APIP_CP_SERVER_HTTP_PORT", "9080")

	cfg, err := loadWithKeys(t, `
[platform_api.server.http]
enabled = '{{ env "APIP_CP_SERVER_HTTP_ENABLED" }}'
port    = '{{ env "APIP_CP_SERVER_HTTP_PORT" }}'
`)
	require.NoError(t, err)
	assert.True(t, cfg.Listeners.HTTP.Enabled, "server.http.enabled from a {{ env }} token must decode to true")
	assert.Equal(t, 9080, cfg.Listeners.HTTP.Port)
}

// Listener timeouts must be finite by default, so a deployment that never sets
// them is still protected against a peer holding connections open (Slowloris).
func TestLoadConfig_Timeouts_DefaultToFiniteValues(t *testing.T) {
	cfg, err := loadWithKeys(t, "")
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, cfg.Listeners.Timeouts.ReadHeader)
	assert.Equal(t, 60*time.Second, cfg.Listeners.Timeouts.Read)
	assert.Equal(t, 120*time.Second, cfg.Listeners.Timeouts.Write)
	assert.Equal(t, 120*time.Second, cfg.Listeners.Timeouts.Idle)
}

// Duration strings resolved from {{ env }} tokens must decode into time.Duration fields.
func TestLoadConfig_Timeouts_TokenOverride(t *testing.T) {
	t.Setenv("APIP_CP_SERVER_TIMEOUTS_READ_HEADER", "5s")
	t.Setenv("APIP_CP_SERVER_TIMEOUTS_READ", "30s")
	t.Setenv("APIP_CP_SERVER_TIMEOUTS_WRITE", "2m")
	t.Setenv("APIP_CP_SERVER_TIMEOUTS_IDLE", "90s")

	cfg, err := loadWithKeys(t, `
[platform_api.server.timeouts]
read_header = '{{ env "APIP_CP_SERVER_TIMEOUTS_READ_HEADER" }}'
read        = '{{ env "APIP_CP_SERVER_TIMEOUTS_READ" }}'
write       = '{{ env "APIP_CP_SERVER_TIMEOUTS_WRITE" }}'
idle        = '{{ env "APIP_CP_SERVER_TIMEOUTS_IDLE" }}'
`)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, cfg.Listeners.Timeouts.ReadHeader)
	assert.Equal(t, 30*time.Second, cfg.Listeners.Timeouts.Read)
	assert.Equal(t, 2*time.Minute, cfg.Listeners.Timeouts.Write)
	assert.Equal(t, 90*time.Second, cfg.Listeners.Timeouts.Idle)
}

// 0 is the net/http "no timeout" sentinel and must be accepted as-is, rather
// than being silently replaced by the default.
func TestLoadConfig_Timeouts_ZeroDisablesTimeout(t *testing.T) {
	cfg, err := loadWithKeys(t, `
[platform_api.server.timeouts]
write = "0s"
`)
	require.NoError(t, err)
	assert.Zero(t, cfg.Listeners.Timeouts.Write, "server.timeouts.write = 0 must disable the write timeout")
}

// A negative duration would expire immediately and break every request; a
// read_header bound above read would never be reached. Both must be rejected at
// load time rather than producing a server that fails at request time.
func TestLoadConfig_Timeouts_RejectsInvalidValues(t *testing.T) {
	t.Run("negative", func(t *testing.T) {
		_, err := loadWithKeys(t, `
[platform_api.server.timeouts]
read = "-1s"
`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not be negative")
	})

	t.Run("read_header exceeds read", func(t *testing.T) {
		_, err := loadWithKeys(t, `
[platform_api.server.timeouts]
read_header = "30s"
read        = "10s"
`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not exceed")
	})
}

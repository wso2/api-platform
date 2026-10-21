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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/sdk/core/signing"
)

const signingBaseConfig = `
[policy_engine.config_mode]
mode = "file"

[policy_engine.file_config]
path = "configs/policy-chains.yaml"
`

func writeECKey(t *testing.T, dir, name string, perm os.FileMode) string {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(k)
	require.NoError(t, err)
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), perm))
	require.NoError(t, os.Chmod(p, perm)) // WriteFile's perm is masked by umask
	return p
}

func TestLoad_SigningAbsentKeepsDefaults(t *testing.T) {
	cfg, err := Load(writePEInterpConfig(t, signingBaseConfig))
	require.NoError(t, err)
	assert.Empty(t, cfg.Signing.Keys)
	assert.Equal(t, DefaultJWKSCacheMaxAge, cfg.Signing.EffectiveJWKSCacheMaxAge())

	kr, err := BuildSigningKeyring(cfg.Signing, nil)
	require.NoError(t, err)
	signing.Install(kr)
	t.Cleanup(func() { signing.Install(nil) })
	assert.False(t, signing.Available())
	body, _ := signing.JWKS()
	assert.JSONEq(t, `{"keys":[]}`, string(body))
}

func TestLoad_SigningKeysFromPathAndInline(t *testing.T) {
	dir := t.TempDir()
	active := writeECKey(t, dir, "active.key", 0o400)
	old := writeECKey(t, dir, "old.key", 0o440)
	t.Setenv("APIP_CONFIG_FILE_SOURCE_ALLOWLIST", dir)

	cfg, err := Load(writePEInterpConfig(t, signingBaseConfig+fmt.Sprintf(`
[signing]
jwks_cache_max_age = "10m"
public_jwks_url = "https://gw.example.com/.well-known/jwks.json"

[[signing.keys]]
path = %q
active = true

[[signing.keys]]
inline = '{{ file "%s" }}'
kid = "previous"
`, active, old)))
	require.NoError(t, err)
	assert.Equal(t, 10*time.Minute, cfg.Signing.EffectiveJWKSCacheMaxAge())
	require.Len(t, cfg.Signing.Keys, 2)

	kr, err := BuildSigningKeyring(cfg.Signing, FileSourceAllowlist())
	require.NoError(t, err)
	signing.Install(kr)
	t.Cleanup(func() { signing.Install(nil) })

	info, ok := signing.ActiveKey()
	require.True(t, ok)
	assert.Equal(t, signing.ES256, info.Algorithm)
	assert.NotEqual(t, "previous", info.Kid)

	body, _ := signing.JWKS()
	assert.Contains(t, string(body), `"kid":"previous"`)
	assert.Contains(t, string(body), `"kid":"`+info.Kid+`"`)
}

func TestValidateSigningConfig(t *testing.T) {
	tests := []struct {
		name string
		toml string
		want string
	}{
		{"no active key", `
[[signing.keys]]
inline = "x"
[[signing.keys]]
inline = "y"`, "exactly one key must be marked active, found 0"},
		{"two active keys", `
[[signing.keys]]
inline = "x"
active = true
[[signing.keys]]
inline = "y"
active = true`, "exactly one key must be marked active, found 2"},
		{"path and inline", `
[[signing.keys]]
inline = "x"
path = "/etc/gateway-runtime/k.pem"
active = true`, "set exactly one of path or inline"},
		{"neither path nor inline", `
[[signing.keys]]
active = true`, "set exactly one of path or inline"},
		{"negative cache age", `
[signing]
jwks_cache_max_age = "-1m"`, "jwks_cache_max_age"},
		{"cache age too long", `
[signing]
jwks_cache_max_age = "48h"`, "jwks_cache_max_age"},
		{"relative jwks url", `
[signing]
public_jwks_url = "/.well-known/jwks.json"`, "public_jwks_url"},
		{"plain http jwks url", `
[signing]
public_jwks_url = "http://gw.example.com/.well-known/jwks.json"`, "public_jwks_url"},
		{"jwks url with credentials", `
[signing]
public_jwks_url = "https://user:pass@gw.example.com/.well-known/jwks.json"`, "public_jwks_url"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(writePEInterpConfig(t, signingBaseConfig+tc.toml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestBuildSigningKeyring_KeyFileOutsideAllowlistIsRejected(t *testing.T) {
	allowed, outside := t.TempDir(), t.TempDir()
	key := writeECKey(t, outside, "k.pem", 0o400)

	_, err := BuildSigningKeyring(SigningConfig{Keys: []SigningKeyConfig{{Path: key, Active: true}}}, []string{allowed})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in an allowed source directory")
}

func TestBuildSigningKeyring_SymlinkEscapingAllowlistIsRejected(t *testing.T) {
	allowed, outside := t.TempDir(), t.TempDir()
	target := writeECKey(t, outside, "k.pem", 0o400)
	link := filepath.Join(allowed, "k.pem")
	require.NoError(t, os.Symlink(target, link))

	_, err := BuildSigningKeyring(SigningConfig{Keys: []SigningKeyConfig{{Path: link, Active: true}}}, []string{allowed})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in an allowed source directory")
}

func TestBuildSigningKeyring_PermissiveKeyFileFailsClosed(t *testing.T) {
	for _, perm := range []os.FileMode{0o404, 0o644, 0o460, 0o600 | 0o002} {
		t.Run(fmt.Sprintf("%04o", perm), func(t *testing.T) {
			dir := t.TempDir()
			key := writeECKey(t, dir, "k.pem", perm)
			_, err := BuildSigningKeyring(SigningConfig{Keys: []SigningKeyConfig{{Path: key, Active: true}}}, []string{dir})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "must not be group-writable or accessible to others")
		})
	}
}

func TestBuildSigningKeyring_ErrorsDoNotLeakKeyMaterial(t *testing.T) {
	dir := t.TempDir()
	key := writeECKey(t, dir, "k.pem", 0o400)
	pemBytes, err := os.ReadFile(key)
	require.NoError(t, err)

	_, err = BuildSigningKeyring(SigningConfig{Keys: []SigningKeyConfig{
		{Path: key, Active: true},
		{Inline: string(pemBytes)}, // same key twice
	}}, []string{dir})
	require.Error(t, err)
	assert.False(t, strings.Contains(err.Error(), "PRIVATE KEY"), "error leaks key material: %v", err)
}

// jwks_endpoint_enabled is read by the gateway controller, which decides
// whether the JWKS route exists; the engine shares the file and must load it.
func TestLoad_SigningToleratesControllerJWKSSwitch(t *testing.T) {
	cfg, err := Load(writePEInterpConfig(t, signingBaseConfig+`
[signing]
jwks_endpoint_enabled = false
`))
	require.NoError(t, err)
	assert.Empty(t, cfg.Signing.Keys)
}

/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package config

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/internal/utils"
)

func TestValidateServiceAccountConfig(t *testing.T) {
	dir := t.TempDir()
	saPub, saPriv := genRSAKeyPEMs()
	_, oldPriv := genRSAKeyPEMs()
	saPrivFile, oldPrivFile := writePEMFile(dir, "sa.pem", saPriv), writePEMFile(dir, "sa_old.pem", oldPriv)
	pubOnlyFile := writePEMFile(dir, "sa_public.pem", saPub)
	ecFile := writePEMFile(dir, "ec.pem", pkcs8PEM(t, mustECKey(t)))
	p521File := writePEMFile(dir, "p521.pem", pkcs8PEM(t, mustECKeyOn(t, elliptic.P521())))
	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	edFile := writePEMFile(dir, "ed.pem", pkcs8PEM(t, edKey))
	bigFile := writePEMFile(dir, "big.pem", strings.Repeat("x", maxSigningKeyFileBytes+1))

	signing := func(keys ...ServiceAccountSigningKey) ServiceAccountSigning {
		return ServiceAccountSigning{Issuer: "platform-api-sa", Keys: keys}
	}
	active := ServiceAccountSigningKey{Path: saPrivFile, Active: true}
	old := ServiceAccountSigningKey{Path: oldPrivFile}
	tests := []struct {
		name    string
		mutate  func(a *Auth)
		wantErr string
	}{
		{"defaults are valid", func(a *Auth) {}, ""},
		{"one own key", func(a *Auth) { a.ServiceAccount.Signing = signing(active) }, ""},
		{"an active and a verify-only key", func(a *Auth) { a.ServiceAccount.Signing = signing(old, active) }, ""},
		{"zero token_ttl", func(a *Auth) { a.ServiceAccount.TokenTTL = 0 }, "token_ttl"},
		{"blank audience", func(a *Auth) { a.ServiceAccount.Audience = " " }, "audience"},
		{"zero poll_interval", func(a *Auth) { a.ServiceAccount.Revocation.PollInterval = 0 }, "poll_interval"},
		{"poll_interval too small to jitter", func(a *Auth) { a.ServiceAccount.Revocation.PollInterval = time.Nanosecond }, "poll_interval"},
		{"negative jwks_cache_max_age", func(a *Auth) { a.ServiceAccount.Signing.JWKSCacheMaxAge = -time.Second }, "jwks_cache_max_age"},
		{"jwks_cache_max_age over a day", func(a *Auth) { a.ServiceAccount.Signing.JWKSCacheMaxAge = 25 * time.Hour }, "jwks_cache_max_age"},
		{"issuer without keys", func(a *Auth) { a.ServiceAccount.Signing = signing() }, "needs auth.service_account.signing.keys"},
		{"keys without issuer", func(a *Auth) {
			a.ServiceAccount.Signing = signing(active)
			a.ServiceAccount.Signing.Issuer = ""
		}, "issuer is required"},
		{"issuer shared with auth.jwt", func(a *Auth) {
			a.ServiceAccount.Signing = signing(active)
			a.ServiceAccount.Signing.Issuer = a.JWT.Issuer
		}, "must differ from auth.jwt.issuer"},
		{"no active key", func(a *Auth) { a.ServiceAccount.Signing = signing(old) }, "exactly one key must be marked active, found 0"},
		{"two active keys", func(a *Auth) {
			a.ServiceAccount.Signing = signing(active, ServiceAccountSigningKey{Path: oldPrivFile, Active: true})
		}, "found 2"},
		{"empty path", func(a *Auth) {
			a.ServiceAccount.Signing = signing(active, ServiceAccountSigningKey{Path: " "})
		}, "keys[1]: path is required"},
		{"missing key file", func(a *Auth) {
			a.ServiceAccount.Signing = signing(ServiceAccountSigningKey{Path: dir + "/missing.pem", Active: true})
		}, "is not found"},
		{"directory as a key file", func(a *Auth) {
			a.ServiceAccount.Signing = signing(ServiceAccountSigningKey{Path: dir, Active: true})
		}, "is not a regular file"},
		{"oversize key file", func(a *Auth) {
			a.ServiceAccount.Signing = signing(ServiceAccountSigningKey{Path: bigFile, Active: true})
		}, "too large"},
		{"public key instead of a private one", func(a *Auth) {
			a.ServiceAccount.Signing = signing(ServiceAccountSigningKey{Path: pubOnlyFile, Active: true})
		}, "must be a PEM-encoded RSA or EC private key"},
		{"an EC key", func(a *Auth) {
			a.ServiceAccount.Signing = signing(ServiceAccountSigningKey{Path: ecFile, Active: true})
		}, ""},
		{"P-521", func(a *Auth) {
			a.ServiceAccount.Signing = signing(ServiceAccountSigningKey{Path: p521File, Active: true})
		}, "P-521 is not supported"},
		{"Ed25519", func(a *Auth) {
			a.ServiceAccount.Signing = signing(ServiceAccountSigningKey{Path: edFile, Active: true})
		}, "unsupported key type"},
		{"an algorithm that does not fit the key", func(a *Auth) {
			a.ServiceAccount.Signing = signing(ServiceAccountSigningKey{Path: ecFile, Active: true, Algorithm: "RS256"})
		}, "does not fit this key"},
		{"same key twice", func(a *Auth) {
			a.ServiceAccount.Signing = signing(active, ServiceAccountSigningKey{Path: saPrivFile})
		}, "keys[0] and [1] have the same kid"},
		{"repeated kid override", func(a *Auth) {
			a.ServiceAccount.Signing = signing(ServiceAccountSigningKey{Path: saPrivFile, Kid: "k", Active: true},
				ServiceAccountSigningKey{Path: oldPrivFile, Kid: "k"})
		}, `have the same kid "k"`},
		{"disabled skips every check", func(a *Auth) {
			a.ServiceAccount.Enabled = false
			a.ServiceAccount.TokenTTL = 0
			a.ServiceAccount.Signing = signing()
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := Auth{
				Mode:           AuthModeInternalToken,
				JWT:            JWT{Issuer: "platform-api", PublicKeyFile: validJWTPublicKeyFile},
				Authorization:  Authorization{Mode: AuthzModeScope},
				ServiceAccount: defaultConfig().Auth.ServiceAccount,
			}
			auth.ServiceAccount.Enabled = true
			tt.mutate(&auth)
			err := validateAuthConfig(&auth)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestServiceAccountSigningLoadKeys(t *testing.T) {
	dir := t.TempDir()
	a, b := mustRSAKey(t), mustRSAKey(t)
	aFile := writePEMFile(dir, "a.pem", string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(a)})))
	bDER, err := x509.MarshalPKCS8PrivateKey(b)
	require.NoError(t, err)
	bFile := writePEMFile(dir, "b.pem", string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: bDER})))

	s := ServiceAccountSigning{Keys: []ServiceAccountSigningKey{
		{Path: aFile, Kid: "old-key"},
		{Path: bFile, Active: true},
	}}
	keys, err := s.LoadKeys()
	require.NoError(t, err)
	require.Len(t, keys, 2)
	assert.Equal(t, "old-key", keys[0].JWK.Kid, "a kid override is used as is")
	assert.True(t, a.Equal(keys[0].Private))
	assert.False(t, keys[0].Active)
	assert.Equal(t, utils.Thumbprint(&b.PublicKey), keys[1].JWK.Kid, "the default kid is the thumbprint")
	assert.True(t, b.Equal(keys[1].Private), "PKCS #8 is accepted")
	assert.True(t, keys[1].Active)
}

func TestServiceAccountSigningEffectiveJWKSCacheMaxAge(t *testing.T) {
	assert.Equal(t, DefaultJWKSCacheMaxAge, (&ServiceAccountSigning{}).EffectiveJWKSCacheMaxAge())
	assert.Equal(t, time.Minute, (&ServiceAccountSigning{JWKSCacheMaxAge: time.Minute}).EffectiveJWKSCacheMaxAge())
}

func TestLoadConfig_ServiceAccountSigningKeys(t *testing.T) {
	dir := t.TempDir()
	_, active := genRSAKeyPEMs()
	_, old := genRSAKeyPEMs()
	activeFile, oldFile := writePEMFile(dir, "active.pem", active), writePEMFile(dir, "old.pem", old)

	cfg, err := loadWithKeys(t, `
[platform_api.auth.service_account]
enabled = true

[platform_api.auth.service_account.signing]
issuer = "platform-api-sa"
jwks_cache_max_age = "2m"

[[platform_api.auth.service_account.signing.keys]]
path = "`+activeFile+`"
active = true

[[platform_api.auth.service_account.signing.keys]]
path = "`+oldFile+`"
kid = "previous"
`)
	require.NoError(t, err)
	s := cfg.Auth.ServiceAccount.Signing
	assert.Equal(t, "platform-api-sa", s.Issuer)
	assert.Equal(t, 2*time.Minute, s.JWKSCacheMaxAge)
	assert.Equal(t, []ServiceAccountSigningKey{{Path: activeFile, Active: true}, {Path: oldFile, Kid: "previous"}}, s.Keys)
}

func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return k
}

func mustECKey(t *testing.T) *ecdsa.PrivateKey { return mustECKeyOn(t, elliptic.P256()) }

func mustECKeyOn(t *testing.T, curve elliptic.Curve) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)
	return k
}

func pkcs8PEM(t *testing.T, key crypto.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// ecParamsPEM is what `openssl ecparam -genkey` writes without -noout: the curve
// parameters, then the SEC 1 key.
func ecParamsPEM(t *testing.T, key *ecdsa.PrivateKey) string {
	t.Helper()
	oid, err := asn1.Marshal(asn1.ObjectIdentifier{1, 3, 132, 0, 34}) // secp384r1
	require.NoError(t, err)
	sec1, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PARAMETERS", Bytes: oid})) +
		string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1}))
}

func TestServiceAccountSigningAlgorithms(t *testing.T) {
	dir := t.TempDir()
	p256, p384, rsaKey := mustECKey(t), mustECKeyOn(t, elliptic.P384()), mustRSAKey(t)
	sec1, err := x509.MarshalECPrivateKey(p256)
	require.NoError(t, err)
	files := map[string]string{
		"rsa":    writePEMFile(dir, "rsa.pem", pkcs8PEM(t, rsaKey)),
		"p256":   writePEMFile(dir, "p256.pem", pkcs8PEM(t, p256)),
		"p384":   writePEMFile(dir, "p384.pem", pkcs8PEM(t, p384)),
		"params": writePEMFile(dir, "params.pem", ecParamsPEM(t, p384)),
		"sec1":   writePEMFile(dir, "sec1.pem", string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1}))),
	}
	tests := []struct {
		file, override, want string
	}{
		{"rsa", "", "RS256"},
		{"rsa", "RS256", "RS256"},
		{"rsa", "PS256", "PS256"},
		{"p256", "", "ES256"},
		{"p256", "ES256", "ES256"},
		{"p384", "", "ES384"},
		{"sec1", "", "ES256"},
		{"params", "", "ES384"},
	}
	for _, tt := range tests {
		s := ServiceAccountSigning{Keys: []ServiceAccountSigningKey{{Path: files[tt.file], Active: true, Algorithm: tt.override}}}
		keys, err := s.LoadKeys()
		require.NoError(t, err, "%s %q", tt.file, tt.override)
		assert.Equal(t, tt.want, keys[0].JWK.Alg, "%s %q", tt.file, tt.override)
	}

	for _, bad := range []struct{ file, override string }{
		{"rsa", "ES256"}, {"rsa", "RS512"}, {"rsa", "HS256"}, {"p256", "ES384"}, {"p384", "ES256"}, {"p256", "PS256"},
	} {
		s := ServiceAccountSigning{Keys: []ServiceAccountSigningKey{{Path: files[bad.file], Active: true, Algorithm: bad.override}}}
		_, err := s.LoadKeys()
		require.Error(t, err, "%s %q", bad.file, bad.override)
		assert.Contains(t, err.Error(), "signing.keys[0]")
	}
}

func TestFileUsersCannotUseTheServiceAccountPrefix(t *testing.T) {
	err := validateFileBasedConfig(&FileBased{
		Organization: FileBasedOrg{ID: "default", DisplayName: "Default"},
		Users:        FileBasedUsers{{Username: "sa:acme:bot:x", PasswordHash: "h", Roles: []string{"ap_admin"}}},
	}, &Authorization{RoleToScopeMapping: "/m.yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved for service accounts")
}

func TestServiceAccountsDisabledByDefault(t *testing.T) {
	assert.False(t, defaultConfig().Auth.ServiceAccount.Enabled)
}

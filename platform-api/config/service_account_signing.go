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
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/utils"
)

const (
	// DefaultJWKSCacheMaxAge is the JWKS Cache-Control max-age when none is set.
	// It is also the shortest wait between two rotation steps.
	DefaultJWKSCacheMaxAge = 5 * time.Minute
	maxJWKSCacheMaxAge     = 24 * time.Hour
	// An RSA-4096 PEM private key is about 3.3 KiB; anything near this is not a key.
	maxSigningKeyFileBytes = 64 << 10
)

// ServiceAccountSigning is [auth.service_account.signing], shaped like the
// gateway's [signing]: every key verifies and is published, the active one signs.
type ServiceAccountSigning struct {
	// Issuer is the iss of tokens signed with Keys. Required with Keys.
	Issuer string `koanf:"issuer"`
	// JWKSCacheMaxAge is the JWKS Cache-Control max-age. Zero means
	// DefaultJWKSCacheMaxAge; read it through EffectiveJWKSCacheMaxAge.
	JWKSCacheMaxAge time.Duration              `koanf:"jwks_cache_max_age"`
	Keys            []ServiceAccountSigningKey `koanf:"keys"`
}

// ServiceAccountSigningKey is one [[auth.service_account.signing.keys]] entry.
type ServiceAccountSigningKey struct {
	// Path is a PEM private key file: RSA, or EC on P-256 or P-384. The public
	// key is derived from it.
	Path string `koanf:"path"`
	// Kid overrides the default key ID, the RFC 7638 thumbprint.
	Kid string `koanf:"kid"`
	// Active marks the one key that signs.
	Active bool `koanf:"active"`
	// Algorithm overrides the one the key type implies: RS256 for RSA, ES256 for
	// P-256, ES384 for P-384. An RSA key may also use PS256.
	Algorithm string `koanf:"algorithm"`
}

// SigningKey is a loaded service-account signing key. JWK carries its kid and
// algorithm.
type SigningKey struct {
	JWK     utils.JWK
	Private crypto.Signer
	Active  bool
}

// EffectiveJWKSCacheMaxAge is the configured max-age, or the default when unset.
func (s *ServiceAccountSigning) EffectiveJWKSCacheMaxAge() time.Duration {
	if s.JWKSCacheMaxAge == 0 {
		return DefaultJWKSCacheMaxAge
	}
	return s.JWKSCacheMaxAge
}

// LoadKeys reads every key, in config order. It fails unless exactly one key is
// active, on a key that is not a supported private key or does not fit its
// algorithm, and on a repeated kid. Errors name entries by index and path,
// never by key material.
func (s *ServiceAccountSigning) LoadKeys() ([]SigningKey, error) {
	active := 0
	for i, k := range s.Keys {
		if strings.TrimSpace(k.Path) == "" {
			return nil, fmt.Errorf("auth.service_account.signing.keys[%d]: path is required", i)
		}
		if k.Active {
			active++
		}
	}
	if len(s.Keys) > 0 && active != 1 {
		return nil, fmt.Errorf("auth.service_account.signing.keys: exactly one key must be marked active, found %d", active)
	}

	keys := make([]SigningKey, 0, len(s.Keys))
	seen := make(map[string]int, len(s.Keys))
	for i, k := range s.Keys {
		priv, err := readSigningKey(k.Path)
		if err != nil {
			return nil, fmt.Errorf("auth.service_account.signing.keys[%d]: %w", i, err)
		}
		alg, err := resolveAlgorithm(priv.Public(), k.Algorithm)
		if err != nil {
			return nil, fmt.Errorf("auth.service_account.signing.keys[%d]: %w", i, err)
		}
		jwk, err := utils.NewJWK(priv.Public(), alg, k.Kid)
		if err != nil {
			return nil, fmt.Errorf("auth.service_account.signing.keys[%d]: %w", i, err)
		}
		if prev, dup := seen[jwk.Kid]; dup {
			return nil, fmt.Errorf("auth.service_account.signing.keys[%d] and [%d] have the same kid %q", prev, i, jwk.Kid)
		}
		seen[jwk.Kid] = i
		keys = append(keys, SigningKey{JWK: jwk, Private: priv, Active: k.Active})
	}
	return keys, nil
}

// readSigningKey reads a PEM private key: RSA (PKCS #1 or PKCS #8) or EC
// (SEC 1 or PKCS #8). The checks run on the opened file, so the file checked is
// the file read.
func readSigningKey(path string) (crypto.Signer, error) {
	f, err := os.Open(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("key file %q is not found", path)
	}
	if err != nil {
		return nil, fmt.Errorf("key file %q could not be opened: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("key file %q is not a regular file", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxSigningKeyFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("key file %q could not be read", path)
	}
	if len(data) > maxSigningKeyFileBytes {
		return nil, fmt.Errorf("key file %q is too large to be a private key", path)
	}
	key, err := parsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("key file %q must be a PEM-encoded RSA or EC private key: %w", path, err)
	}
	return key, nil
}

func parsePrivateKey(data []byte) (crypto.Signer, error) {
	block, rest := pem.Decode(data)
	// `openssl ecparam -genkey` without -noout writes the curve parameters first.
	for block != nil && block.Type == "EC PARAMETERS" {
		block, rest = pem.Decode(rest)
	}
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		switch k := key.(type) {
		case *rsa.PrivateKey:
			return k, nil
		case *ecdsa.PrivateKey:
			return k, nil
		}
		return nil, fmt.Errorf("unsupported key type %T", key)
	}
	return nil, fmt.Errorf("unsupported PEM block %q", block.Type)
}

// resolveAlgorithm returns override, or the algorithm pub implies when it is
// empty. It fails when the key type does not allow the algorithm.
func resolveAlgorithm(pub crypto.PublicKey, override string) (string, error) {
	var allowed []string // the first is the default
	switch k := pub.(type) {
	case *rsa.PublicKey:
		allowed = []string{"RS256", "PS256"}
	case *ecdsa.PublicKey:
		switch k.Curve {
		case elliptic.P256():
			allowed = []string{"ES256"}
		case elliptic.P384():
			allowed = []string{"ES384"}
		default:
			return "", fmt.Errorf("EC curve %s is not supported; use P-256 or P-384", k.Params().Name)
		}
	default:
		return "", fmt.Errorf("unsupported key type %T", pub)
	}
	if override == "" {
		return allowed[0], nil
	}
	if !slices.Contains(allowed, override) {
		return "", fmt.Errorf("algorithm %q does not fit this key; use %s", override, strings.Join(allowed, " or "))
	}
	return override, nil
}

// validateServiceAccountSigning checks [auth.service_account.signing] against
// the other issuers and loads every key, so a bad key fails startup.
func validateServiceAccountSigning(auth *Auth) error {
	s := &auth.ServiceAccount.Signing
	if s.JWKSCacheMaxAge < 0 || s.JWKSCacheMaxAge > maxJWKSCacheMaxAge {
		return fmt.Errorf("auth.service_account.signing.jwks_cache_max_age must be between 0 (default %s) and %s, got %s",
			DefaultJWKSCacheMaxAge, maxJWKSCacheMaxAge, s.JWKSCacheMaxAge)
	}
	if len(s.Keys) == 0 {
		if s.Issuer != "" {
			return fmt.Errorf("auth.service_account.signing.issuer needs auth.service_account.signing.keys; " +
				"without keys, service-account tokens are signed with auth.jwt and carry its issuer")
		}
		return nil
	}
	if strings.TrimSpace(s.Issuer) == "" {
		return fmt.Errorf("auth.service_account.signing.issuer is required with auth.service_account.signing.keys")
	}
	if s.Issuer == auth.JWT.Issuer {
		return fmt.Errorf("auth.service_account.signing.issuer must differ from auth.jwt.issuer (%q)", auth.JWT.Issuer)
	}
	if auth.Mode == AuthModeIDP {
		for _, iss := range auth.IDP.Issuer {
			if iss == s.Issuer {
				return fmt.Errorf("auth.service_account.signing.issuer must differ from every auth.idp.issuer (%q)", iss)
			}
		}
	}
	_, err := s.LoadKeys()
	return err
}

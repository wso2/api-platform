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
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

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
	// Path is a PEM RSA private key file. The public key is derived from it.
	Path string `koanf:"path"`
	// Kid overrides the default key ID, the RFC 7638 thumbprint.
	Kid string `koanf:"kid"`
	// Active marks the one key that signs.
	Active bool `koanf:"active"`
}

// SigningKey is a loaded service-account signing key.
type SigningKey struct {
	Kid     string
	Private *rsa.PrivateKey
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
// active, on a key that is not an RSA private key, and on a repeated kid. Errors
// name entries by index and path, never by key material.
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
		kid := k.Kid
		if kid == "" {
			kid = utils.RSAThumbprint(&priv.PublicKey)
		}
		if prev, dup := seen[kid]; dup {
			return nil, fmt.Errorf("auth.service_account.signing.keys[%d] and [%d] have the same kid %q", prev, i, kid)
		}
		seen[kid] = i
		keys = append(keys, SigningKey{Kid: kid, Private: priv, Active: k.Active})
	}
	return keys, nil
}

// readSigningKey reads a PEM RSA private key (PKCS #1 or PKCS #8). The checks
// run on the opened file, so the file checked is the file read.
func readSigningKey(path string) (*rsa.PrivateKey, error) {
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
	priv, err := jwt.ParseRSAPrivateKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("key file %q must be a PEM-encoded RSA private key: %w", path, err)
	}
	return priv, nil
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

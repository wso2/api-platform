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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/signing"
)

const (
	// DefaultJWKSCacheMaxAge is the Cache-Control max-age of the gateway JWKS.
	// It is also the minimum wait between rotation steps, so it trades how fast
	// a rotation can proceed against how often receivers refetch.
	DefaultJWKSCacheMaxAge = 5 * time.Minute
	maxJWKSCacheMaxAge     = 24 * time.Hour
)

// SigningConfig is the root-level [signing] section: the gateway's own signing
// keys, shared by every policy that signs what the gateway sends out (backend
// JWTs, A2A Agent Card signatures, ...) through sdk/core/signing, and published
// at the gateway JWKS endpoint.
//
// It is deliberately not a policy configuration: keys, rotation and the JWKS
// are a gateway capability, so one key set serves every signing policy and no
// policy reads key files itself. A policy's own legacy key setting (e.g. the
// backend-jwt signingKey) is not imported here — such a key keeps working for
// that policy but is not published until it is moved into this section.
type SigningConfig struct {
	// JWKSCacheMaxAge is the Cache-Control max-age the JWKS endpoint serves.
	// Zero means DefaultJWKSCacheMaxAge; read it through EffectiveJWKSCacheMaxAge.
	JWKSCacheMaxAge time.Duration `koanf:"jwks_cache_max_age"`
	// PublicJWKSURL is the externally reachable JWKS URL, advertised as "jku"
	// in signatures whose policy asks for it. Optional; https only, because a
	// verifier that follows jku must fetch it with integrity protection
	// (RFC 7515 §4.1.2).
	PublicJWKSURL string `koanf:"public_jwks_url"`
	// Keys lists every signing key. Exactly one is marked active and signs;
	// all of them are published so signatures made by any of them verify.
	Keys []SigningKeyConfig `koanf:"keys"`
}

// SigningKeyConfig is one [[signing.keys]] entry.
type SigningKeyConfig struct {
	// Path is a PEM private key file. It must not be group-writable or
	// accessible to "other" (0400 or 0440).
	Path string `koanf:"path"`
	// Inline is the PEM private key itself. Prefer Path: config.toml is also
	// read by the gateway-controller, which expands every {{ file }} and
	// {{ env }} token, so a key supplied inline through a token is read by the
	// controller too.
	Inline string `koanf:"inline"`
	// Kid overrides the default key ID (the RFC 7638 JWK thumbprint).
	Kid string `koanf:"kid"`
	// Algorithm overrides the algorithm implied by the key type.
	Algorithm string `koanf:"algorithm"`
	// Active marks the one key that signs.
	Active bool `koanf:"active"`
}

// EffectiveJWKSCacheMaxAge is the JWKS max-age to serve: the configured value,
// or DefaultJWKSCacheMaxAge when it is unset.
func (s SigningConfig) EffectiveJWKSCacheMaxAge() time.Duration {
	if s.JWKSCacheMaxAge == 0 {
		return DefaultJWKSCacheMaxAge
	}
	return s.JWKSCacheMaxAge
}

// validateSigningConfig checks the shape of [signing]. Key material is parsed
// and validated by BuildSigningKeyring, which needs the files.
func (c *Config) validateSigningConfig() error {
	s := c.Signing
	if s.JWKSCacheMaxAge < 0 || s.JWKSCacheMaxAge > maxJWKSCacheMaxAge {
		return fmt.Errorf("signing.jwks_cache_max_age must be between 0 (default %s) and %s, got %s",
			DefaultJWKSCacheMaxAge, maxJWKSCacheMaxAge, s.JWKSCacheMaxAge)
	}
	if s.PublicJWKSURL != "" {
		u, err := url.Parse(s.PublicJWKSURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return fmt.Errorf("signing.public_jwks_url must be an absolute https URL without credentials")
		}
	}

	active := 0
	for i, k := range s.Keys {
		if (k.Path == "") == (k.Inline == "") {
			return fmt.Errorf("signing.keys[%d]: set exactly one of path or inline", i)
		}
		if k.Active {
			active++
		}
	}
	if len(s.Keys) > 0 && active != 1 {
		return fmt.Errorf("signing.keys: exactly one key must be marked active, found %d", active)
	}
	return nil
}

// BuildSigningKeyring reads and parses the [signing] keys into the keyring that
// the policy engine installs for every policy (sdk/core/signing.Install); the
// keyring is the engine's implementation of the SDK signing service.
//
// A key path is operator configuration, like the config file itself, so it is
// read from wherever it points; it is not subject to the {{ file }} source
// allowlist. A key file that is group-writable or accessible to "other", or
// larger than any PEM private key, fails startup rather than being used. With
// no keys configured it returns an empty keyring, which publishes an empty
// JWKS and cannot sign.
func BuildSigningKeyring(s SigningConfig) (*signing.Keyring, error) {
	cfg := signing.Config{
		PublicJWKSURL:   s.PublicJWKSURL,
		JWKSCacheMaxAge: s.EffectiveJWKSCacheMaxAge(),
		Keys:            make([]signing.KeyEntry, 0, len(s.Keys)),
	}

	for i, k := range s.Keys {
		pemBytes := []byte(k.Inline)
		if k.Path != "" {
			data, err := readKeyFile(k.Path)
			if err != nil {
				return nil, fmt.Errorf("signing.keys[%d]: %w", i, err)
			}
			pemBytes = data
		}
		cfg.Keys = append(cfg.Keys, signing.KeyEntry{
			PEM:       pemBytes,
			Kid:       k.Kid,
			Algorithm: signing.Algorithm(k.Algorithm),
			Active:    k.Active,
		})
	}

	kr, err := signing.NewKeyring(cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid signing configuration: %w", err)
	}
	return kr, nil
}

// maxKeyFileBytes bounds a key file read. A PEM private key of the largest
// supported size (RSA-4096) is about 3.3 KiB; anything near this is not a key.
const maxKeyFileBytes = 64 << 10

// readKeyFile reads a private key file, failing closed on one that "other" can
// read or write or that its group can write. Group read is allowed: Kubernetes
// Secret volumes mounted with fsGroup are group-owned, and 0440 is the usual
// mode for them. The permissions are checked on the opened file, so the file
// that is checked is the file that is read.
func readKeyFile(path string) ([]byte, error) {
	f, err := os.Open(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("required file %q is not found", path)
	}
	if err != nil {
		// Most often a permission problem, such as a Secret mounted without
		// read access for the runtime user; say so rather than "not found".
		return nil, fmt.Errorf("required file %q could not be opened: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("required file %q is not a regular file", path)
	}
	if perm := info.Mode().Perm(); perm&0o027 != 0 {
		return nil, fmt.Errorf("private key file %q has permissions %04o; it must not be group-writable or accessible to others (use 0400 or 0440)", path, perm)
	}

	data, err := io.ReadAll(io.LimitReader(f, maxKeyFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("required file %q could not be read", path)
	}
	if len(data) > maxKeyFileBytes {
		return nil, fmt.Errorf("file %q exceeds the maximum allowed size for a private key", path)
	}
	return data, nil
}

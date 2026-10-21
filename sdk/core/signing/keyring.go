/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

// Package signing is the gateway signing service as policies see it: one
// process-wide set of operator-configured signing keys, exactly one of which
// is active, used by every policy that signs something the gateway sends out
// (backend JWTs, A2A Agent Card signatures, and so on).
//
// Policies sign through the functions in this package and never hold a key.
// They get signatures and the kid/algorithm that produced them; the private
// key never leaves this package. Every key, active or not, is published in the
// gateway JWKS (see JWKS) so a receiver can verify by kid.
//
// The policy engine builds the Keyring from its [signing] configuration and
// calls Install exactly once at startup — the same startup-only contract as
// utils.SetSharedHTTPClient. A policy must check Available before relying on
// gateway signing: on a gateway with no [signing] keys, or on an engine that
// predates this package, nothing is installed and every Sign* call returns
// ErrNotConfigured.
package signing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

// ErrNotConfigured is returned by every signing call when no gateway signing
// key is installed.
var ErrNotConfigured = errors.New("gateway signing key not configured")

// KeyInfo identifies the key that produced a signature.
type KeyInfo struct {
	Kid       string
	Algorithm Algorithm
}

// KeyEntry is one configured signing key, as the policy engine reads it from
// [[signing.keys]]. The engine is responsible for reading PEM from disk safely
// (path containment, size limit, file permissions); this package only parses it.
type KeyEntry struct {
	// PEM is a single private key: PKCS #1 RSA, SEC 1 EC, or PKCS #8
	// (RSA or EC). The public key is derived from it; no
	// certificate is needed.
	PEM []byte
	// Kid overrides the default key ID, the RFC 7638 thumbprint.
	Kid string
	// Algorithm overrides the algorithm implied by the key type. It must be
	// one the key can sign with (RS256/PS256 for RSA, ES256 for P-256,
	// ES384 for P-384).
	Algorithm Algorithm
	// Active marks the one key that signs. Exactly one entry must set it.
	Active bool
}

// Config is the input to NewKeyring.
type Config struct {
	Keys []KeyEntry
	// PublicJWKSURL, when set, is the externally reachable URL of the gateway
	// JWKS. It is added as the "jku" header only when a caller asks for it
	// with WithJKU.
	PublicJWKSURL string
	// JWKSCacheMaxAge is how long receivers may cache the JWKS before
	// refetching it (Cache-Control max-age). Zero means DefaultJWKSCacheMaxAge.
	JWKSCacheMaxAge time.Duration
}

// DefaultJWKSCacheMaxAge is the JWKS cache lifetime when none is configured.
const DefaultJWKSCacheMaxAge = 5 * time.Minute

// Keyring is an immutable, validated set of gateway signing keys.
type Keyring struct {
	active  *signingKey
	info    KeyInfo
	jwksURL string
	jwks    []byte
	etag    string
	maxAge  time.Duration
}

// NewKeyring parses and validates cfg. It fails if no key or more than one key
// is marked active, if two entries resolve to the same kid, or if any key
// cannot be parsed or cannot sign with its algorithm. Errors name entries by
// position and kid only, never by key material.
//
// An empty Config yields an empty Keyring: it publishes {"keys":[]} and
// cannot sign.
func NewKeyring(cfg Config) (*Keyring, error) {
	if cfg.JWKSCacheMaxAge < 0 {
		return nil, errors.New("JWKS cache max-age must not be negative")
	}
	kr := &Keyring{jwksURL: cfg.PublicJWKSURL, maxAge: cfg.JWKSCacheMaxAge}
	if kr.maxAge == 0 {
		kr.maxAge = DefaultJWKSCacheMaxAge
	}
	published := make([]PublicJWK, 0, len(cfg.Keys))
	seen := make(map[string]int, len(cfg.Keys))

	for i, entry := range cfg.Keys {
		signer, err := parsePrivateKeyPEM(entry.PEM)
		if err != nil {
			return nil, fmt.Errorf("signing key %d: %w", i, err)
		}
		alg, err := resolveAlgorithm(signer, entry.Algorithm)
		if err != nil {
			return nil, fmt.Errorf("signing key %d: %w", i, err)
		}
		kid := entry.Kid
		if kid == "" {
			if kid, err = Thumbprint(signer.Public()); err != nil {
				return nil, fmt.Errorf("signing key %d: %w", i, err)
			}
		}
		if prev, dup := seen[kid]; dup {
			return nil, fmt.Errorf("signing keys %d and %d have the same kid %q", prev, i, kid)
		}
		seen[kid] = i

		jwk, err := publicJWK(signer.Public(), alg)
		if err != nil {
			return nil, fmt.Errorf("signing key %d: %w", i, err)
		}
		jwk.Kid, jwk.Use = kid, "sig"
		published = append(published, jwk)

		if entry.Active {
			if kr.active != nil {
				return nil, fmt.Errorf("signing keys %q and %q are both marked active; exactly one key must be active", kr.info.Kid, kid)
			}
			kr.active = &signingKey{alg: alg, signer: signer}
			kr.info = KeyInfo{Kid: kid, Algorithm: alg}
		}
	}

	if len(cfg.Keys) > 0 && kr.active == nil {
		return nil, errors.New("no signing key is marked active; exactly one key must be active")
	}

	body, err := json.Marshal(struct {
		Keys []PublicJWK `json:"keys"`
	}{published})
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	kr.jwks = body
	kr.etag = `"` + hex.EncodeToString(sum[:16]) + `"`
	return kr, nil
}

var installed atomic.Pointer[Keyring]

// Install makes kr the process-wide gateway keyring. Called once by the policy
// engine at startup; not intended to be called by policies. Passing nil
// uninstalls it (tests only).
func Install(kr *Keyring) {
	installed.Store(kr)
}

// Available reports whether a gateway signing key is installed and can sign.
func Available() bool {
	kr := installed.Load()
	return kr != nil && kr.active != nil
}

// ActiveKey returns the kid and algorithm of the key that currently signs.
func ActiveKey() (KeyInfo, bool) {
	kr := installed.Load()
	if kr == nil || kr.active == nil {
		return KeyInfo{}, false
	}
	return kr.info, true
}

// JWKS returns the gateway JWK Set (every configured key, active or not) and
// its strong entity tag, quoted and ready for an ETag header. With nothing
// installed it returns an empty set.
func JWKS() (body []byte, etag string) {
	if kr := installed.Load(); kr != nil {
		return kr.jwks, kr.etag
	}
	return emptyJWKS, emptyJWKSETag
}

// JWKSCacheMaxAge is how long a receiver may cache the JWKS, for the
// Cache-Control header of the JWKS response.
func JWKSCacheMaxAge() time.Duration {
	if kr := installed.Load(); kr != nil {
		return kr.maxAge
	}
	return DefaultJWKSCacheMaxAge
}

var (
	emptyJWKS     = []byte(`{"keys":[]}`)
	emptyJWKSETag = func() string {
		sum := sha256.Sum256(emptyJWKS)
		return `"` + hex.EncodeToString(sum[:16]) + `"`
	}()
)

// activeKeyring returns the installed keyring if it can sign.
func activeKeyring() (*Keyring, error) {
	kr := installed.Load()
	if kr == nil || kr.active == nil {
		return nil, ErrNotConfigured
	}
	return kr, nil
}

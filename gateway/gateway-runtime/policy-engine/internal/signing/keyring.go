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

// Package signing is the policy engine's implementation of the gateway signing
// service (sdk/core/signing.Service): it parses the operator's [signing] keys,
// signs with the active one, and generates the gateway JWKS document that
// publishes every key.
//
// The engine builds one Keyring at startup and installs it through
// sdk/core/signing.Install; policies only ever reach it through that SDK
// package, so the private keys never leave this package.
package signing

import (
	"errors"
	"fmt"
	"time"

	sdksigning "github.com/wso2/api-platform/sdk/core/signing"
)

// KeyEntry is one configured signing key, as the policy engine reads it from
// [[signing.keys]]. The config loader is responsible for reading PEM from disk
// safely (path containment, size limit, file permissions); this package only
// parses it.
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
	// refetching it (Cache-Control max-age). Zero means
	// sdksigning.DefaultJWKSCacheMaxAge.
	JWKSCacheMaxAge time.Duration
}

// Keyring is an immutable, validated set of gateway signing keys. It
// implements sdksigning.Service.
//
// A nil *Keyring behaves as an empty keyring: it publishes {"keys":[]} and
// cannot sign. sdksigning.Install treats any non-nil interface value as
// installed, so a nil *Keyring handed to it must not panic on first use.
type Keyring struct {
	active  *signingKey
	info    sdksigning.KeyInfo
	jwksURL string
	jwks    []byte
	etag    string
	maxAge  time.Duration
}

var _ sdksigning.Service = (*Keyring)(nil)

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
		kr.maxAge = sdksigning.DefaultJWKSCacheMaxAge
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
			kr.info = sdksigning.KeyInfo{Kid: kid, Algorithm: alg}
		}
	}

	if len(cfg.Keys) > 0 && kr.active == nil {
		return nil, errors.New("no signing key is marked active; exactly one key must be active")
	}

	body, etag, err := buildJWKS(published)
	if err != nil {
		return nil, err
	}
	kr.jwks, kr.etag = body, etag
	return kr, nil
}

// ActiveKey returns the kid and algorithm of the key that signs.
func (kr *Keyring) ActiveKey() (sdksigning.KeyInfo, bool) {
	if kr == nil || kr.active == nil {
		return sdksigning.KeyInfo{}, false
	}
	return kr.info, true
}

// JWKS returns the gateway JWK Set (every configured key, active or not) and
// its strong entity tag, quoted and ready for an ETag header.
func (kr *Keyring) JWKS() (body []byte, etag string) {
	if kr == nil {
		return emptyJWKS, emptyJWKSETag
	}
	return kr.jwks, kr.etag
}

// JWKSCacheMaxAge is how long a receiver may cache the JWKS.
func (kr *Keyring) JWKSCacheMaxAge() time.Duration {
	if kr == nil {
		return sdksigning.DefaultJWKSCacheMaxAge
	}
	return kr.maxAge
}

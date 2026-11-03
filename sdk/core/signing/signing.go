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
// This package is only the contract. The keys, the signing itself and the
// gateway JWKS document are implemented by the policy engine, which builds its
// Service from the [signing] configuration and calls Install exactly once at
// startup — the same startup-only contract as utils.SetSharedHTTPClient.
// Policies sign through the functions in this package and never hold a key:
// they get signatures and the kid/algorithm that produced them.
//
// A policy must check Available before relying on gateway signing: on a
// gateway with no [signing] keys, or on an engine that predates this package,
// nothing that can sign is installed and every Sign* call returns
// ErrNotConfigured.
package signing

import (
	"bytes"
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

// Algorithm is a JOSE "alg" value (RFC 7518). Only the values below are ever
// produced; the algorithm is decided by the operator's key configuration and is
// never taken from a caller or a peer.
//
// The set is classical (RSA and EC) only. A post-quantum (ML-DSA) option is
// tracked in https://github.com/wso2/api-platform/issues/3450.
type Algorithm string

const (
	RS256 Algorithm = "RS256"
	PS256 Algorithm = "PS256"
	ES256 Algorithm = "ES256"
	ES384 Algorithm = "ES384"
)

// DefaultJWKSCacheMaxAge is the JWKS cache lifetime when none is configured.
const DefaultJWKSCacheMaxAge = 5 * time.Minute

// KeyInfo identifies the key that produced a signature.
type KeyInfo struct {
	Kid       string
	Algorithm Algorithm
}

// DetachedJWS is a JWS whose payload is carried separately (RFC 7515
// Appendix F). Its JSON form is the A2A AgentCardSignature object.
type DetachedJWS struct {
	Protected string         `json:"protected"`
	Signature string         `json:"signature"`
	Header    map[string]any `json:"header,omitempty"`
}

// Service is the gateway signing service the policy engine installs. Policies
// do not implement or call it directly; they use the package functions, which
// delegate to the installed Service.
//
// An implementation must always set "alg" and "kid" from the key that signs,
// must set "jku" only from the operator-configured JWKS URL and only when
// Options.JKU is set, and must reject any reserved header in Options.Headers.
type Service interface {
	// Sign returns the raw JWS-encoded signature over data (RFC 7518 §3;
	// r||s for ECDSA) made with the active key.
	Sign(data []byte) ([]byte, KeyInfo, error)
	// SignJWS returns a compact-serialised JWS (RFC 7515 §7.1) over payload.
	SignJWS(payload []byte, opts Options) (string, KeyInfo, error)
	// SignDetachedJWS returns a JWS over payload without the payload.
	SignDetachedJWS(payload []byte, opts Options) (DetachedJWS, KeyInfo, error)
	// ActiveKey returns the key that signs, if there is one.
	ActiveKey() (KeyInfo, bool)
	// JWKS returns the gateway JWK Set (every configured key, active or not)
	// and its strong entity tag, quoted and ready for an ETag header.
	JWKS() (body []byte, etag string)
	// JWKSCacheMaxAge is how long a receiver may cache the JWKS.
	JWKSCacheMaxAge() time.Duration
}

// serviceHolder lets atomic.Pointer hold an interface value.
type serviceHolder struct{ svc Service }

var installed atomic.Pointer[serviceHolder]

// Install makes svc the process-wide gateway signing service. Called once by
// the policy engine at startup; not intended to be called by policies.
// Passing nil uninstalls it (tests only).
func Install(svc Service) {
	if svc == nil {
		installed.Store(nil)
		return
	}
	installed.Store(&serviceHolder{svc: svc})
}

func current() Service {
	if h := installed.Load(); h != nil {
		return h.svc
	}
	return nil
}

// Available reports whether a gateway signing key is installed and can sign.
func Available() bool {
	_, ok := ActiveKey()
	return ok
}

// ActiveKey returns the kid and algorithm of the key that currently signs.
func ActiveKey() (KeyInfo, bool) {
	if svc := current(); svc != nil {
		return svc.ActiveKey()
	}
	return KeyInfo{}, false
}

// Sign returns the raw signature over data made with the active gateway key,
// and the key that made it. The encoding is the JWS one for the key's
// algorithm (RFC 7518 §3; r||s for ECDSA). Use it for formats that are not
// JOSE; prefer the JWS functions otherwise.
func Sign(data []byte) ([]byte, KeyInfo, error) {
	svc := current()
	if svc == nil {
		return nil, KeyInfo{}, ErrNotConfigured
	}
	return svc.Sign(data)
}

// SignJWS returns a compact-serialised JWS (RFC 7515 §7.1) over payload.
func SignJWS(payload []byte, opts ...Option) (string, KeyInfo, error) {
	svc := current()
	if svc == nil {
		return "", KeyInfo{}, ErrNotConfigured
	}
	o, err := ResolveOptions(opts...)
	if err != nil {
		return "", KeyInfo{}, err
	}
	return svc.SignJWS(payload, o)
}

// SignJWT marshals claims to JSON and returns them as a signed compact JWT
// (RFC 7519) with typ "JWT" unless WithType says otherwise. Claim validation
// (exp, iat, iss, ...) is the caller's responsibility.
func SignJWT(claims map[string]any, opts ...Option) (string, KeyInfo, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", KeyInfo{}, fmt.Errorf("marshal JWT claims: %w", err)
	}
	return SignJWS(payload, append([]Option{WithType("JWT")}, opts...)...)
}

// SignDetachedJWS signs payload and returns the signature without it. The
// verifier reconstructs the signing input from its own copy of the payload,
// so the payload must be byte-identical on both sides; for a JSON document,
// sign CanonicalJSON(document).
func SignDetachedJWS(payload []byte, opts ...Option) (DetachedJWS, KeyInfo, error) {
	svc := current()
	if svc == nil {
		return DetachedJWS{}, KeyInfo{}, ErrNotConfigured
	}
	o, err := ResolveOptions(opts...)
	if err != nil {
		return DetachedJWS{}, KeyInfo{}, err
	}
	return svc.SignDetachedJWS(payload, o)
}

// JWKS returns the gateway JWK Set (every configured key, active or not) and
// its strong entity tag, quoted and ready for an ETag header. With nothing
// installed it returns an empty set. The body is the caller's own copy.
func JWKS() (body []byte, etag string) {
	if svc := current(); svc != nil {
		return svc.JWKS()
	}
	return bytes.Clone(emptyJWKS), emptyJWKSETag
}

// JWKSCacheMaxAge is how long a receiver may cache the JWKS, for the
// Cache-Control header of the JWKS response.
func JWKSCacheMaxAge() time.Duration {
	if svc := current(); svc != nil {
		return svc.JWKSCacheMaxAge()
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

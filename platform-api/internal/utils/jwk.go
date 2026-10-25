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

package utils

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
)

// JWK holds the RFC 7517 members of an RSA or EC public signing key. The
// fields are in alphabetical order, so the JSON of an RSA key stays the same
// as the API spec's JWK has always produced.
type JWK struct {
	Alg string `json:"alg"`
	Crv string `json:"crv,omitempty"`
	E   string `json:"e,omitempty"`
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n,omitempty"`
	Use string `json:"use"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

// JWKS is a JSON Web Key Set (RFC 7517).
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// NewJWK encodes pub as a signing JWK for alg. An empty kid becomes the
// RFC 7638 thumbprint. It fails for a key that is not RSA or EC.
func NewJWK(pub crypto.PublicKey, alg, kid string) (JWK, error) {
	jwk, err := jwkMembers(pub)
	if err != nil {
		return JWK{}, err
	}
	jwk.Alg, jwk.Use, jwk.Kid = alg, "sig", kid
	if kid == "" {
		jwk.Kid = Thumbprint(pub)
	}
	return jwk, nil
}

// Thumbprint is the RFC 7638 SHA-256 thumbprint of pub, base64url-encoded. It
// is empty for a key that is not RSA or EC.
func Thumbprint(pub crypto.PublicKey) string {
	jwk, err := jwkMembers(pub)
	if err != nil {
		return ""
	}
	// Required members in lexicographic order, no whitespace (RFC 7638 section 3).
	canonical := `{"e":"` + jwk.E + `","kty":"RSA","n":"` + jwk.N + `"}`
	if jwk.Kty == "EC" {
		canonical = `{"crv":"` + jwk.Crv + `","kty":"EC","x":"` + jwk.X + `","y":"` + jwk.Y + `"}`
	}
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// jwkMembers fills the key members only.
func jwkMembers(pub crypto.PublicKey) (JWK, error) {
	switch k := pub.(type) {
	case *rsa.PublicKey:
		return JWK{Kty: "RSA",
			N: base64.RawURLEncoding.EncodeToString(k.N.Bytes()),
			E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes())}, nil
	case *ecdsa.PublicKey:
		// RFC 7518 section 6.2.1.2: x and y are as long as the curve, zero-padded.
		size := (k.Curve.Params().BitSize + 7) / 8
		return JWK{Kty: "EC", Crv: k.Curve.Params().Name,
			X: base64.RawURLEncoding.EncodeToString(k.X.FillBytes(make([]byte, size))),
			Y: base64.RawURLEncoding.EncodeToString(k.Y.FillBytes(make([]byte, size)))}, nil
	}
	return JWK{}, fmt.Errorf("unsupported public key type %T", pub)
}

// SetClaim writes value at path, a flat claim name ("roles") or a dot-separated
// path into nested objects ("realm_access.roles"). It mirrors the middleware's
// resolveClaimPath, so a mapping reads back the way it was written.
func SetClaim(claims map[string]interface{}, path string, value interface{}) {
	if path == "" {
		return
	}
	parts := strings.Split(path, ".")
	current := claims
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

// GetClaim reads a claim written by SetClaim.
func GetClaim(claims map[string]interface{}, path string) (interface{}, bool) {
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	current := claims
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = next
	}
	v, ok := current[parts[len(parts)-1]]
	return v, ok
}

// ClaimKey returns name, or def when the claim mapping is unset.
func ClaimKey(name, def string) string {
	if name == "" {
		return def
	}
	return name
}

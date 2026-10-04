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
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
	"strings"
)

// RSAPublicJWK holds the RFC 7517 members of an RSA public key.
type RSAPublicJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// NewRSAPublicJWK encodes pub as a signing JWK whose kid is its thumbprint.
func NewRSAPublicJWK(pub *rsa.PublicKey) RSAPublicJWK {
	n, e := rsaMembers(pub)
	return RSAPublicJWK{Kty: "RSA", Kid: RSAThumbprint(pub), Use: "sig", Alg: "RS256", N: n, E: e}
}

// RSAThumbprint is the RFC 7638 SHA-256 thumbprint of pub, base64url-encoded.
func RSAThumbprint(pub *rsa.PublicKey) string {
	n, e := rsaMembers(pub)
	// Members in lexicographic order, no whitespace (RFC 7638 section 3).
	sum := sha256.Sum256([]byte(`{"e":"` + e + `","kty":"RSA","n":"` + n + `"}`))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func rsaMembers(pub *rsa.PublicKey) (n, e string) {
	n = base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e = base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return n, e
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

// ClaimKey returns name, or def when the claim mapping is unset.
func ClaimKey(name, def string) string {
	if name == "" {
		return def
	}
	return name
}

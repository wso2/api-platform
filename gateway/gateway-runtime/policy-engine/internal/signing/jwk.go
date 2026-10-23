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

package signing

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
)

var b64 = base64.RawURLEncoding

// PublicJWK is a public JSON Web Key as published in the gateway JWKS.
//
// It is a closed struct with public members only. There is no field a private
// key component (d, p, q, dp, dq, qi) could be marshalled into, so
// a JWKS built from it cannot leak private material whatever key it came from.
type PublicJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	// RSA
	N string `json:"n,omitempty"`
	E string `json:"e,omitempty"`
	// EC
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

// publicJWK renders pub without a kid or use; callers fill those in.
func publicJWK(pub crypto.PublicKey, alg Algorithm) (PublicJWK, error) {
	switch k := pub.(type) {
	case *rsa.PublicKey:
		return PublicJWK{
			Kty: "RSA", Alg: string(alg),
			N: b64.EncodeToString(k.N.Bytes()),
			E: b64.EncodeToString(big.NewInt(int64(k.E)).Bytes()),
		}, nil
	case *ecdsa.PublicKey:
		size := (k.Curve.Params().BitSize + 7) / 8
		ecdh, err := k.Bytes() // uncompressed point: 0x04 || X || Y
		if err != nil {
			return PublicJWK{}, err
		}
		return PublicJWK{
			Kty: "EC", Alg: string(alg), Crv: k.Curve.Params().Name,
			X: b64.EncodeToString(ecdh[1 : 1+size]),
			Y: b64.EncodeToString(ecdh[1+size:]),
		}, nil
	default:
		return PublicJWK{}, fmt.Errorf("unsupported public key type %T", pub)
	}
}

// Thumbprint returns the RFC 7638 JWK Thumbprint (SHA-256, base64url) of pub,
// the gateway's default key ID.
//
// It hashes only the public key, so the same key always produces the same
// thumbprint: on every replica, and whether the key is read from a private key
// file or from a certificate.
func Thumbprint(pub crypto.PublicKey) (string, error) {
	jwk, err := publicJWK(pub, "")
	if err != nil {
		return "", err
	}

	// The required members, in lexicographic order, with no whitespace
	// (RFC 7638 §3.2). Every value is base64url or a fixed ASCII name, so
	// json.Marshal of a string is already the canonical form.
	var members [][2]string
	switch jwk.Kty {
	case "RSA":
		members = [][2]string{{"e", jwk.E}, {"kty", jwk.Kty}, {"n", jwk.N}}
	case "EC":
		members = [][2]string{{"crv", jwk.Crv}, {"kty", jwk.Kty}, {"x", jwk.X}, {"y", jwk.Y}}
	}

	buf := []byte{'{'}
	for i, m := range members {
		if i > 0 {
			buf = append(buf, ',')
		}
		name, _ := json.Marshal(m[0])
		value, _ := json.Marshal(m[1])
		buf = append(append(append(buf, name...), ':'), value...)
	}
	buf = append(buf, '}')

	sum := sha256.Sum256(buf)
	return b64.EncodeToString(sum[:]), nil
}

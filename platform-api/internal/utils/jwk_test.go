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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"reflect"
	"testing"
)

// The example key and thumbprint from RFC 7638 section 3.1.
const (
	rfc7638N          = "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw"
	rfc7638Thumbprint = "NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs"
)

func rfc7638Key(t *testing.T) *rsa.PublicKey {
	t.Helper()
	n, err := base64.RawURLEncoding.DecodeString(rfc7638N)
	if err != nil {
		t.Fatal(err)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: 65537}
}

func TestThumbprint_RFC7638Vector(t *testing.T) {
	if got := Thumbprint(rfc7638Key(t)); got != rfc7638Thumbprint {
		t.Fatalf("thumbprint = %q, want %q", got, rfc7638Thumbprint)
	}
}

func TestNewJWK_RSA(t *testing.T) {
	jwk, err := NewJWK(rfc7638Key(t), "RS256", "")
	if err != nil {
		t.Fatal(err)
	}
	want := JWK{Kty: "RSA", Kid: rfc7638Thumbprint, Use: "sig", Alg: "RS256", N: rfc7638N, E: "AQAB"}
	if jwk != want {
		t.Fatalf("jwk = %+v", jwk)
	}
	// The body of an RSA key, and so the JWKS ETag, must not change between releases.
	body, _ := json.Marshal(jwk)
	wantBody := `{"alg":"RS256","e":"AQAB","kid":"` + rfc7638Thumbprint + `","kty":"RSA","n":"` + rfc7638N + `","use":"sig"}`
	if string(body) != wantBody {
		t.Fatalf("body = %s", body)
	}
	if jwk, _ := NewJWK(rfc7638Key(t), "PS256", "mine"); jwk.Kid != "mine" || jwk.Alg != "PS256" {
		t.Fatalf("kid and alg overrides: %+v", jwk)
	}
}

// The example key from RFC 7515 appendix A.3.
func TestNewJWK_EC_RFC7515Key(t *testing.T) {
	const x, y = "f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU", "x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0"
	xb, _ := base64.RawURLEncoding.DecodeString(x)
	yb, _ := base64.RawURLEncoding.DecodeString(y)
	pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(xb), Y: new(big.Int).SetBytes(yb)}

	jwk, err := NewJWK(pub, "ES256", "")
	if err != nil {
		t.Fatal(err)
	}
	if jwk.Kty != "EC" || jwk.Crv != "P-256" || jwk.X != x || jwk.Y != y || jwk.N != "" || jwk.E != "" {
		t.Fatalf("jwk = %+v", jwk)
	}
	sum := sha256.Sum256([]byte(`{"crv":"P-256","kty":"EC","x":"` + x + `","y":"` + y + `"}`))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); jwk.Kid != want {
		t.Fatalf("kid = %q, want %q", jwk.Kid, want)
	}
}

// big.Int drops leading zeros, but x and y are as long as the curve.
func TestNewJWK_EC_PadsShortCoordinates(t *testing.T) {
	pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: big.NewInt(1), Y: big.NewInt(2)}
	jwk, err := NewJWK(pub, "ES256", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := base64.RawURLEncoding.EncodeToString(append(make([]byte, 31), 1)); jwk.X != want {
		t.Fatalf("x = %q, want %q", jwk.X, want)
	}
	if want := base64.RawURLEncoding.EncodeToString(append(make([]byte, 31), 2)); jwk.Y != want {
		t.Fatalf("y = %q, want %q", jwk.Y, want)
	}
}

func TestNewJWK_P384AndUnsupported(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	jwk, err := NewJWK(&key.PublicKey, "ES384", "")
	if err != nil || jwk.Crv != "P-384" {
		t.Fatalf("jwk = %+v, %v", jwk, err)
	}
	if b, _ := base64.RawURLEncoding.DecodeString(jwk.X); len(b) != 48 {
		t.Fatalf("x is %d bytes, want 48", len(b))
	}
	if _, err := NewJWK("not a key", "RS256", ""); err == nil {
		t.Fatal("an unsupported key type must fail")
	}
}

func TestSetAndGetClaim(t *testing.T) {
	claims := map[string]interface{}{}
	SetClaim(claims, "roles", []string{"a"})
	SetClaim(claims, "realm_access.roles", []string{"b"})
	SetClaim(claims, "", "ignored")
	if len(claims) != 2 {
		t.Fatalf("empty path must write nothing: %v", claims)
	}
	if v, ok := GetClaim(claims, "roles"); !ok || !reflect.DeepEqual(v, []string{"a"}) {
		t.Errorf("flat: %v %v", v, ok)
	}
	if v, ok := GetClaim(claims, "realm_access.roles"); !ok || !reflect.DeepEqual(v, []string{"b"}) {
		t.Errorf("nested: %v %v", v, ok)
	}

	// A non-object in the way is replaced on write, and stops a read.
	claims["x"] = "scalar"
	if _, ok := GetClaim(claims, "x.y"); ok {
		t.Error("read through a scalar must fail")
	}
	SetClaim(claims, "x.y", 1)
	if v, ok := GetClaim(claims, "x.y"); !ok || v != 1 {
		t.Errorf("write through a scalar: %v %v", v, ok)
	}
	for _, path := range []string{"", "missing", "realm_access.missing"} {
		if _, ok := GetClaim(claims, path); ok {
			t.Errorf("GetClaim(%q) found a value", path)
		}
	}
}

func TestClaimKey(t *testing.T) {
	if ClaimKey("", "scope") != "scope" || ClaimKey("scp", "scope") != "scp" {
		t.Fatal("ClaimKey must fall back to the default only when unset")
	}
}

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
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
)

// --- PEM parsing ------------------------------------------------------------

func TestParsePrivateKeyPEMAcceptedEncodings(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sec1, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		t.Fatal(err)
	}
	_, rsaKey := rsaPEM(t, 2048)
	pkcs8RSA, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	pkcs1, _ := rsaPEM(t, 2048)
	pkcs8EC, _ := ecPEM(t, elliptic.P384())

	tests := []struct {
		name string
		pem  []byte
		want string // %T of the parsed signer
	}{
		{"SEC 1 EC", pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1}), "*ecdsa.PrivateKey"},
		{"PKCS #8 RSA", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8RSA}), "*rsa.PrivateKey"},
		{"PKCS #1 RSA", pkcs1, "*rsa.PrivateKey"},
		{"PKCS #8 EC", pkcs8EC, "*ecdsa.PrivateKey"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			signer, err := parsePrivateKeyPEM(tc.pem)
			if err != nil {
				t.Fatalf("parsePrivateKeyPEM: %v", err)
			}
			if got := typeName(signer); got != tc.want {
				t.Fatalf("parsed %s, want %s", got, tc.want)
			}
		})
	}

	// The SEC 1 key is the same key that was encoded.
	signer, _ := parsePrivateKeyPEM(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1}))
	if !ecKey.Equal(signer) {
		t.Fatal("SEC 1 key did not round-trip")
	}
}

func TestParsePrivateKeyPEMRejected(t *testing.T) {
	x25519 := mustX25519PKCS8(t)
	garbage := []byte{0x30, 0x03, 0x02, 0x01, 0x00}

	tests := []struct {
		name  string
		block pem.Block
		want  string
	}{
		{"certificate", pem.Block{Type: "CERTIFICATE", Bytes: garbage}, `unsupported PEM block type "CERTIFICATE"`},
		{"encrypted private key", pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: garbage}, `unsupported PEM block type "ENCRYPTED PRIVATE KEY"`},
		{"public key", pem.Block{Type: "PUBLIC KEY", Bytes: garbage}, `unsupported PEM block type "PUBLIC KEY"`},
		{"corrupt PKCS #1", pem.Block{Type: "RSA PRIVATE KEY", Bytes: garbage}, ""},
		{"corrupt SEC 1", pem.Block{Type: "EC PRIVATE KEY", Bytes: garbage}, ""},
		{"corrupt PKCS #8", pem.Block{Type: "PRIVATE KEY", Bytes: garbage}, ""},
		{"PKCS #8 X25519", pem.Block{Type: "PRIVATE KEY", Bytes: x25519}, "unsupported PKCS #8 key type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			signer, err := parsePrivateKeyPEM(pem.EncodeToMemory(&tc.block))
			if err == nil {
				t.Fatalf("accepted, parsed %s", typeName(signer))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func mustX25519PKCS8(t *testing.T) []byte {
	t.Helper()
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

func typeName(v any) string {
	switch v.(type) {
	case *rsa.PrivateKey:
		return "*rsa.PrivateKey"
	case *ecdsa.PrivateKey:
		return "*ecdsa.PrivateKey"
	case ed25519.PrivateKey:
		return "ed25519.PrivateKey"
	default:
		return "other"
	}
}

// --- algorithm resolution ---------------------------------------------------

func TestResolveAlgorithm(t *testing.T) {
	_, rsa2048 := rsaPEM(t, 2048)
	_, rsa2047 := rsaPEM(t, 2047)
	_, p256 := ecPEM(t, elliptic.P256())
	_, p384 := ecPEM(t, elliptic.P384())
	_, p521 := ecPEM(t, elliptic.P521())

	// An Ed25519 PKCS #8 key parses (it is a crypto.Signer) but cannot be used.
	_, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	edDER, err := x509.MarshalPKCS8PrivateKey(edPriv)
	if err != nil {
		t.Fatal(err)
	}
	ed, err := parsePrivateKeyPEM(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: edDER}))
	if err != nil {
		t.Fatalf("Ed25519 PKCS #8 did not parse: %v", err)
	}

	tests := []struct {
		name     string
		signer   crypto.Signer
		override Algorithm
		want     Algorithm // empty means rejected
		wantErr  string
	}{
		{"P-521", p521, "", "", "unsupported EC curve P-521"},
		{"Ed25519", ed, "", "", "unsupported key type"},
		{"RSA 2047 bits", rsa2047, "", "", "RSA key is 2047 bits; at least 2048 are required"},
		{"RSA 2048 bits", rsa2048, "", RS256, ""},
		{"P-256 default", p256, "", ES256, ""},
		{"P-384 default", p384, "", ES384, ""},
		{"ES384 on P-256", p256, ES384, "", "cannot be used with this key"},
		{"ES256 on P-384", p384, ES256, "", "cannot be used with this key"},
		{"ES256 on RSA", rsa2048, ES256, "", "cannot be used with this key"},
		{"HS256 on RSA", rsa2048, "HS256", "", "cannot be used with this key"},
		{"HS256 on P-256", p256, "HS256", "", "cannot be used with this key"},
		{"none on RSA", rsa2048, "none", "", "cannot be used with this key"},
		{"none on P-256", p256, "none", "", "cannot be used with this key"},
		{"RS256 on RSA", rsa2048, RS256, RS256, ""},
		{"PS256 on RSA", rsa2048, PS256, PS256, ""},
		{"ES256 on P-256", p256, ES256, ES256, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveAlgorithm(tc.signer, tc.override)
			if tc.want == "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("resolveAlgorithm = %q, %v; want an error containing %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("resolveAlgorithm = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

// --- RFC 7638 thumbprint ----------------------------------------------------

// ecThumbprint computes the RFC 7638 thumbprint of an EC key without the
// production JWK code: the coordinates come from crypto/ecdh and the canonical
// JSON is written out by hand.
func ecThumbprint(t *testing.T, priv *ecdsa.PrivateKey, crv string, size int) string {
	t.Helper()
	ecdhPriv, err := priv.ECDH()
	if err != nil {
		t.Fatal(err)
	}
	point := ecdhPriv.PublicKey().Bytes() // 0x04 || X || Y
	if len(point) != 1+2*size {
		t.Fatalf("%s point is %d bytes, want %d", crv, len(point), 1+2*size)
	}
	enc := base64.RawURLEncoding
	canonical := `{"crv":"` + crv + `","kty":"EC","x":"` + enc.EncodeToString(point[1:1+size]) +
		`","y":"` + enc.EncodeToString(point[1+size:]) + `"}`
	sum := sha256.Sum256([]byte(canonical))
	return enc.EncodeToString(sum[:])
}

func TestThumbprintECKnownAnswer(t *testing.T) {
	tests := []struct {
		curve elliptic.Curve
		crv   string
		size  int
	}{
		{elliptic.P256(), "P-256", 32},
		{elliptic.P384(), "P-384", 48},
	}
	for _, tc := range tests {
		t.Run(tc.crv, func(t *testing.T) {
			keyPEM, priv := ecPEM(t, tc.curve)
			want := ecThumbprint(t, priv, tc.crv, tc.size)

			got, err := Thumbprint(&priv.PublicKey)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("thumbprint = %s, want %s", got, want)
			}

			// The default kid of a configured key is that thumbprint.
			kr, err := NewKeyring(Config{Keys: []KeyEntry{{PEM: keyPEM, Active: true}}})
			if err != nil {
				t.Fatal(err)
			}
			if info, _ := kr.ActiveKey(); info.Kid != want {
				t.Fatalf("default kid = %s, want %s", info.Kid, want)
			}
		})
	}
}

// The thumbprint hashes only the public key, so the algorithm an RSA key is
// configured to sign with does not change its kid.
func TestThumbprintIndependentOfRSAAlgorithm(t *testing.T) {
	keyPEM, _ := rsaPEM(t, 2048)
	kids := map[Algorithm]string{}
	for _, alg := range []Algorithm{RS256, PS256} {
		kr, err := NewKeyring(Config{Keys: []KeyEntry{{PEM: keyPEM, Algorithm: alg, Active: true}}})
		if err != nil {
			t.Fatal(err)
		}
		info, _ := kr.ActiveKey()
		if info.Algorithm != alg {
			t.Fatalf("alg = %s, want %s", info.Algorithm, alg)
		}
		kids[alg] = info.Kid
	}
	if kids[RS256] != kids[PS256] {
		t.Fatalf("kid under RS256 %s != kid under PS256 %s", kids[RS256], kids[PS256])
	}
}

func TestRSAPublicExponentEncoding(t *testing.T) {
	keyPEM, priv := rsaPEM(t, 2048)
	kr, err := NewKeyring(Config{Keys: []KeyEntry{{PEM: keyPEM, Active: true}}})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := kr.JWKS()
	var set struct{ Keys []PublicJWK }
	if err := json.Unmarshal(body, &set); err != nil || len(set.Keys) != 1 {
		t.Fatalf("JWKS = %s (%v)", body, err)
	}
	k := set.Keys[0]
	if k.E != "AQAB" {
		t.Fatalf("e = %q, want AQAB", k.E)
	}
	if k.N != base64.RawURLEncoding.EncodeToString(priv.N.Bytes()) {
		t.Fatal("n does not match the key modulus")
	}
}

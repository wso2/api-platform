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
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	sdksigning "github.com/wso2/api-platform/sdk/core/signing"
)

// --- key fixtures -----------------------------------------------------------

func rsaPEM(t *testing.T, bits int) ([]byte, *rsa.PrivateKey) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}), k
}

func ecPEM(t *testing.T, curve elliptic.Curve) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	k, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), k
}

func install(t *testing.T, cfg Config) *Keyring {
	t.Helper()
	kr, err := NewKeyring(cfg)
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	sdksigning.Install(kr)
	t.Cleanup(func() { sdksigning.Install(nil) })
	return kr
}

// --- RFC 7638 thumbprint --------------------------------------------------

func TestThumbprintRFC7638Vector(t *testing.T) {
	// RFC 7638 §3.1.
	n, _ := b64.DecodeString("0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw")
	pub := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: 65537}

	got, err := Thumbprint(pub)
	if err != nil {
		t.Fatal(err)
	}
	if want := "NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs"; got != want {
		t.Fatalf("thumbprint = %s, want %s", got, want)
	}
}

// The default kid must be the same whether it is derived from the private key
// or from a certificate for that key.
func TestThumbprintFromKeyEqualsFromCertificate(t *testing.T) {
	keyPEM, priv := ecPEM(t, elliptic.P256())
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "gw"},
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, priv.Public(), priv)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	fromCert, err := Thumbprint(cert.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	kr := install(t, Config{Keys: []KeyEntry{{PEM: keyPEM, Active: true}}})
	if kr.info.Kid != fromCert {
		t.Fatalf("kid from key %s != kid from certificate %s", kr.info.Kid, fromCert)
	}
}

// --- keyring validation ---------------------------------------------------

func TestNewKeyringValidation(t *testing.T) {
	k1, _ := ecPEM(t, elliptic.P256())
	k2, _ := ecPEM(t, elliptic.P256())
	weak, _ := rsaPEM(t, 1024)

	tests := []struct {
		name string
		keys []KeyEntry
		want string
	}{
		{"none active", []KeyEntry{{PEM: k1}, {PEM: k2}}, "no signing key is marked active"},
		{"two active", []KeyEntry{{PEM: k1, Active: true}, {PEM: k2, Active: true}}, "both marked active"},
		{"same key twice", []KeyEntry{{PEM: k1, Active: true}, {PEM: k1}}, "same kid"},
		{"duplicate explicit kid", []KeyEntry{{PEM: k1, Kid: "a", Active: true}, {PEM: k2, Kid: "a"}}, "same kid"},
		{"not PEM", []KeyEntry{{PEM: []byte("nope"), Active: true}}, "no PEM block"},
		{"two PEM blocks", []KeyEntry{{PEM: append(append([]byte{}, k1...), k2...), Active: true}}, "more than one PEM block"},
		{"weak RSA", []KeyEntry{{PEM: weak, Active: true}}, "at least 2048"},
		{"incompatible override", []KeyEntry{{PEM: k1, Algorithm: RS256, Active: true}}, "cannot be used with this key"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewKeyring(Config{Keys: tc.keys})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestErrorsNeverContainKeyMaterial(t *testing.T) {
	k1, _ := ecPEM(t, elliptic.P256())
	_, err := NewKeyring(Config{Keys: []KeyEntry{{PEM: k1}, {PEM: k1}}})
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "PRIVATE KEY") || strings.Contains(err.Error(), string(k1[40:80])) {
		t.Fatalf("error leaks key material: %v", err)
	}
}

// --- availability ----------------------------------------------------------

func TestEmptyKeyringPublishesEmptySet(t *testing.T) {
	install(t, Config{})
	if sdksigning.Available() {
		t.Fatal("an empty keyring must not be able to sign")
	}
	body, etag := sdksigning.JWKS()
	wantETag := jwksETag([]byte(`{"keys":[]}`))
	if string(body) != `{"keys":[]}` || etag != wantETag {
		t.Fatalf("JWKS() = %s %s", body, etag)
	}
}

func TestEmptyKeyringCannotSign(t *testing.T) {
	kr, err := NewKeyring(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := kr.ActiveKey(); ok {
		t.Fatal("an empty keyring reports an active key")
	}
	if _, _, err := kr.Sign([]byte("x")); !errors.Is(err, sdksigning.ErrNotConfigured) {
		t.Fatalf("Sign err = %v, want ErrNotConfigured", err)
	}
	if _, _, err := kr.SignJWS([]byte("x"), sdksigning.Options{}); !errors.Is(err, sdksigning.ErrNotConfigured) {
		t.Fatalf("SignJWS err = %v, want ErrNotConfigured", err)
	}
}

// sdksigning.Install stores any non-nil interface value, so a nil *Keyring
// counts as installed. Every call a policy can make must then behave like an
// empty keyring instead of panicking.
func TestNilKeyringInstalledBehavesAsEmpty(t *testing.T) {
	var kr *Keyring
	sdksigning.Install(kr)
	t.Cleanup(func() { sdksigning.Install(nil) })

	if sdksigning.Available() {
		t.Fatal("Available() with a nil keyring")
	}
	if _, ok := sdksigning.ActiveKey(); ok {
		t.Fatal("ActiveKey() reported a key for a nil keyring")
	}
	if _, _, err := sdksigning.Sign([]byte("x")); !errors.Is(err, sdksigning.ErrNotConfigured) {
		t.Fatalf("Sign err = %v, want ErrNotConfigured", err)
	}
	if _, _, err := sdksigning.SignJWT(map[string]any{"sub": "x"}); !errors.Is(err, sdksigning.ErrNotConfigured) {
		t.Fatalf("SignJWT err = %v, want ErrNotConfigured", err)
	}
	if _, _, err := sdksigning.SignDetachedJWS([]byte("x")); !errors.Is(err, sdksigning.ErrNotConfigured) {
		t.Fatalf("SignDetachedJWS err = %v, want ErrNotConfigured", err)
	}

	empty, err := NewKeyring(Config{})
	if err != nil {
		t.Fatal(err)
	}
	wantBody, wantETag := empty.JWKS()
	body, etag := sdksigning.JWKS()
	if string(body) != `{"keys":[]}` || string(body) != string(wantBody) || etag != wantETag {
		t.Fatalf("JWKS() = %s %s, want %s %s", body, etag, wantBody, wantETag)
	}
	if got := sdksigning.JWKSCacheMaxAge(); got != sdksigning.DefaultJWKSCacheMaxAge {
		t.Fatalf("JWKSCacheMaxAge() = %s", got)
	}
}

// Options is a plain struct, so the engine must not rely on the SDK option
// builders alone to keep reserved headers out.
func TestReservedHeadersRejectedInRawOptions(t *testing.T) {
	keyPEM, _ := ecPEM(t, elliptic.P256())
	kr, err := NewKeyring(Config{Keys: []KeyEntry{{PEM: keyPEM, Active: true}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alg", "kid", "jku", "crit", "b64", "typ"} {
		opts := sdksigning.Options{Headers: map[string]any{name: "none"}}
		if _, _, err := kr.SignJWS([]byte("x"), opts); err == nil {
			t.Fatalf("raw header %q was accepted", name)
		}
		if _, _, err := kr.SignDetachedJWS([]byte("x"), opts); err == nil {
			t.Fatalf("raw header %q was accepted by SignDetachedJWS", name)
		}
	}
}

// --- signing ----------------------------------------------------------------

// verifyCompact checks a compact JWS against pub using only the standard
// library, independently of the signing code.
func verifyCompact(t *testing.T, token string, pub crypto.PublicKey) (header, payload map[string]any) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("not a compact JWS: %s", token)
	}
	h, _ := b64.DecodeString(parts[0])
	p, _ := b64.DecodeString(parts[1])
	sig, err := b64.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(h, &header); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(p, &payload)
	verifySignature(t, header["alg"].(string), pub, []byte(parts[0]+"."+parts[1]), sig)
	return header, payload
}

func verifySignature(t *testing.T, alg string, pub crypto.PublicKey, input, sig []byte) {
	t.Helper()
	switch alg {
	case "RS256":
		d := sha256.Sum256(input)
		if err := rsa.VerifyPKCS1v15(pub.(*rsa.PublicKey), crypto.SHA256, d[:], sig); err != nil {
			t.Fatalf("RS256 verify: %v", err)
		}
	case "PS256":
		d := sha256.Sum256(input)
		if err := rsa.VerifyPSS(pub.(*rsa.PublicKey), crypto.SHA256, d[:], sig, nil); err != nil {
			t.Fatalf("PS256 verify: %v", err)
		}
	case "ES256", "ES384":
		var d []byte
		size := 32
		if alg == "ES256" {
			s := sha256.Sum256(input)
			d = s[:]
		} else {
			s := sha512.Sum384(input)
			d, size = s[:], 48
		}
		if len(sig) != 2*size {
			t.Fatalf("%s signature is %d bytes, want %d", alg, len(sig), 2*size)
		}
		r, s := new(big.Int).SetBytes(sig[:size]), new(big.Int).SetBytes(sig[size:])
		if !ecdsa.Verify(pub.(*ecdsa.PublicKey), d, r, s) {
			t.Fatalf("%s verify failed", alg)
		}
	default:
		t.Fatalf("unexpected alg %q", alg)
	}
}

func TestSignJWTEveryAlgorithm(t *testing.T) {
	rsaKey, rsaPriv := rsaPEM(t, 2048)
	p256, p256Priv := ecPEM(t, elliptic.P256())
	p384, p384Priv := ecPEM(t, elliptic.P384())

	tests := []struct {
		name    string
		entry   KeyEntry
		pub     crypto.PublicKey
		wantAlg Algorithm
		wantKty string
	}{
		{"RSA default", KeyEntry{PEM: rsaKey}, &rsaPriv.PublicKey, RS256, "RSA"},
		{"RSA PS256", KeyEntry{PEM: rsaKey, Algorithm: PS256}, &rsaPriv.PublicKey, PS256, "RSA"},
		{"P-256", KeyEntry{PEM: p256}, &p256Priv.PublicKey, ES256, "EC"},
		{"P-384", KeyEntry{PEM: p384}, &p384Priv.PublicKey, ES384, "EC"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.entry.Active = true
			install(t, Config{Keys: []KeyEntry{tc.entry}})

			token, info, err := sdksigning.SignJWT(map[string]any{"sub": "alice", "iss": "gw"})
			if err != nil {
				t.Fatal(err)
			}
			if info.Algorithm != tc.wantAlg {
				t.Fatalf("alg = %s, want %s", info.Algorithm, tc.wantAlg)
			}
			header, claims := verifyCompact(t, token, tc.pub)
			if header["kid"] != info.Kid || header["typ"] != "JWT" || header["alg"] != string(tc.wantAlg) {
				t.Fatalf("header = %v", header)
			}
			if claims["sub"] != "alice" {
				t.Fatalf("claims = %v", claims)
			}

			// The published key verifies the token and carries the same kid.
			var set struct{ Keys []PublicJWK }
			body, _ := sdksigning.JWKS()
			if err := json.Unmarshal(body, &set); err != nil || len(set.Keys) != 1 {
				t.Fatalf("JWKS = %s (%v)", body, err)
			}
			if k := set.Keys[0]; k.Kid != info.Kid || k.Kty != tc.wantKty || k.Alg != string(tc.wantAlg) || k.Use != "sig" {
				t.Fatalf("JWK = %+v", k)
			}
		})
	}
}

func TestOnlyTheActiveKeySignsAndAllArePublished(t *testing.T) {
	oldPEM, _ := ecPEM(t, elliptic.P256())
	newPEM, newPriv := ecPEM(t, elliptic.P256())
	install(t, Config{Keys: []KeyEntry{{PEM: oldPEM}, {PEM: newPEM, Active: true}}})

	token, info, err := sdksigning.SignJWS([]byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	newKid, _ := Thumbprint(&newPriv.PublicKey)
	if info.Kid != newKid {
		t.Fatalf("signed with %s, want the active key %s", info.Kid, newKid)
	}
	verifyCompact(t, token, &newPriv.PublicKey)

	var set struct{ Keys []PublicJWK }
	body, _ := sdksigning.JWKS()
	_ = json.Unmarshal(body, &set)
	if len(set.Keys) != 2 {
		t.Fatalf("JWKS publishes %d keys, want 2", len(set.Keys))
	}
}

func TestJWKSHasNoPrivateMembers(t *testing.T) {
	rsaKey, _ := rsaPEM(t, 2048)
	ec, _ := ecPEM(t, elliptic.P256())
	install(t, Config{Keys: []KeyEntry{{PEM: rsaKey, Active: true}, {PEM: ec}}})

	body, _ := sdksigning.JWKS()
	var set struct{ Keys []map[string]any }
	if err := json.Unmarshal(body, &set); err != nil {
		t.Fatal(err)
	}
	for _, k := range set.Keys {
		for _, private := range []string{"d", "p", "q", "dp", "dq", "qi", "oth", "k"} {
			if _, ok := k[private]; ok {
				t.Fatalf("JWK %v contains private member %q", k["kid"], private)
			}
		}
	}
}

func TestSignDetachedJWS(t *testing.T) {
	keyPEM, priv := ecPEM(t, elliptic.P256())
	install(t, Config{
		Keys:          []KeyEntry{{PEM: keyPEM, Active: true}},
		PublicJWKSURL: "https://gw.example.com/.well-known/jwks.json",
	})

	card, err := sdksigning.CanonicalJSON([]byte(`{"name": "agent", "version": "1.0"}`))
	if err != nil {
		t.Fatal(err)
	}
	sig, info, err := sdksigning.SignDetachedJWS(card, sdksigning.WithJKU())
	if err != nil {
		t.Fatal(err)
	}

	// A verifier rebuilds the signing input from its own copy of the payload.
	raw, _ := b64.DecodeString(sig.Signature)
	verifySignature(t, string(info.Algorithm), &priv.PublicKey, []byte(sig.Protected+"."+b64.EncodeToString(card)), raw)

	h, _ := b64.DecodeString(sig.Protected)
	var header map[string]any
	_ = json.Unmarshal(h, &header)
	if header["jku"] != "https://gw.example.com/.well-known/jwks.json" || header["kid"] != info.Kid {
		t.Fatalf("protected header = %v", header)
	}

	// Serialises as an A2A AgentCardSignature.
	out, _ := json.Marshal(sig)
	if !strings.HasPrefix(string(out), `{"protected":"`) || strings.Contains(string(out), `"header"`) {
		t.Fatalf("detached JWS JSON = %s", out)
	}
}

func TestReservedHeadersCannotBeOverridden(t *testing.T) {
	keyPEM, _ := ecPEM(t, elliptic.P256())
	install(t, Config{Keys: []KeyEntry{{PEM: keyPEM, Active: true}}})

	for _, name := range []string{"alg", "kid", "jku", "crit", "b64", "typ"} {
		if _, _, err := sdksigning.SignJWS([]byte("x"), sdksigning.WithHeader(name, "none")); err == nil {
			t.Fatalf("WithHeader(%q) was accepted", name)
		}
	}
	token, _, err := sdksigning.SignJWS([]byte("x"), sdksigning.WithHeader("cty", "example"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mustDecodeHeader(t, token), `"cty":"example"`) {
		t.Fatal("custom header missing")
	}
}

func TestWithJKUWithoutConfiguredURLIsANoOp(t *testing.T) {
	keyPEM, _ := ecPEM(t, elliptic.P256())
	install(t, Config{Keys: []KeyEntry{{PEM: keyPEM, Active: true}}})
	token, _, err := sdksigning.SignJWS([]byte("x"), sdksigning.WithJKU())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(mustDecodeHeader(t, token), "jku") {
		t.Fatal("jku set although no public JWKS URL is configured")
	}
}

func TestSignRaw(t *testing.T) {
	keyPEM, priv := rsaPEM(t, 2048)
	install(t, Config{Keys: []KeyEntry{{PEM: keyPEM, Active: true}}})
	sig, info, err := sdksigning.Sign([]byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	verifySignature(t, string(info.Algorithm), &priv.PublicKey, []byte("data"), sig)
}

func TestExplicitKidOverride(t *testing.T) {
	keyPEM, _ := ecPEM(t, elliptic.P256())
	install(t, Config{Keys: []KeyEntry{{PEM: keyPEM, Kid: "gw-2026-10", Active: true}}})
	if info, _ := sdksigning.ActiveKey(); info.Kid != "gw-2026-10" {
		t.Fatalf("kid = %s", info.Kid)
	}
}

func mustDecodeHeader(t *testing.T, token string) string {
	t.Helper()
	h, err := b64.DecodeString(strings.SplitN(token, ".", 2)[0])
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func TestJWKSCacheMaxAge(t *testing.T) {
	sdksigning.Install(nil)
	if sdksigning.JWKSCacheMaxAge() != sdksigning.DefaultJWKSCacheMaxAge {
		t.Fatalf("default max-age = %s", sdksigning.JWKSCacheMaxAge())
	}
	install(t, Config{JWKSCacheMaxAge: 10 * time.Minute})
	if sdksigning.JWKSCacheMaxAge() != 10*time.Minute {
		t.Fatalf("configured max-age = %s", sdksigning.JWKSCacheMaxAge())
	}
	if _, err := NewKeyring(Config{JWKSCacheMaxAge: -time.Second}); err == nil {
		t.Fatal("negative max-age accepted")
	}
}

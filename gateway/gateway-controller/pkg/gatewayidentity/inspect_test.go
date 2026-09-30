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

package gatewayidentity

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"
)

// ============ Certificate/key generation helpers ============
// These tests need RSA and Ed25519 leaves, which the shared pki helper does
// not generate.

func mustSerial(t *testing.T) *big.Int {
	t.Helper()
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		t.Fatalf("failed to generate serial number: %v", err)
	}
	return n
}

type certOption func(*x509.Certificate)

func withValidity(notBefore, notAfter time.Time) certOption {
	return func(c *x509.Certificate) {
		c.NotBefore = notBefore
		c.NotAfter = notAfter
	}
}

// issueLeaf creates a non-CA (clientAuth by default) certificate for cn,
// signed by parent (self-signed when parent/parentKey are nil), using key as
// the leaf's own keypair. key may be *rsa.PrivateKey, *ecdsa.PrivateKey or
// ed25519.PrivateKey — all implement crypto.Signer.
func issueLeaf(t *testing.T, cn string, key crypto.Signer, parent *x509.Certificate, parentKey crypto.Signer, opts ...certOption) []byte {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber:          mustSerial(t),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	for _, o := range opts {
		o(tmpl)
	}

	parentCert, signer := tmpl, key
	if parent != nil {
		parentCert, signer = parent, parentKey
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parentCert, key.Public(), signer)
	if err != nil {
		t.Fatalf("failed to create certificate for %q: %v", cn, err)
	}
	return der
}

func pemCert(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func pemPKCS8Key(t *testing.T, key crypto.Signer) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func genRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return key
}

func genECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}
	return key
}

func genEd25519Key(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate Ed25519 key: %v", err)
	}
	return key
}

// ============ Inspect: chain+key match across algorithms ============

func TestInspect_RSAKeyMatchesLeaf(t *testing.T) {
	key := genRSAKey(t)
	der := issueLeaf(t, "rsa-identity", key, nil, nil)

	bundle, err := Inspect(pemCert(der), pemPKCS8Key(t, key), time.Now())
	if err != nil {
		t.Fatalf("expected a matching RSA chain+key to inspect cleanly, got: %v", err)
	}
	if bundle.KeyAlgorithm != "RSA" {
		t.Errorf("expected KeyAlgorithm %q, got %q", "RSA", bundle.KeyAlgorithm)
	}
	if bundle.PrivateKey == nil {
		t.Errorf("expected a parsed private key on the bundle")
	}
}

func TestInspect_Ed25519KeyMatchesLeaf(t *testing.T) {
	key := genEd25519Key(t)
	der := issueLeaf(t, "ed25519-identity", key, nil, nil)

	bundle, err := Inspect(pemCert(der), pemPKCS8Key(t, key), time.Now())
	if err != nil {
		t.Fatalf("expected a matching Ed25519 chain+key to inspect cleanly, got: %v", err)
	}
	if bundle.KeyAlgorithm != "Ed25519" {
		t.Errorf("expected KeyAlgorithm %q, got %q", "Ed25519", bundle.KeyAlgorithm)
	}
}

// ============ Inspect: key/certificate mismatch ============

// ============ InspectPrivateKey: passphrase-protected keys ============

func TestInspectPrivateKey_LegacyProcTypeEncryptedHeader_ExactMessage(t *testing.T) {
	// OpenSSL-style RSA PRIVATE KEY with a "Proc-Type: 4,ENCRYPTED" header.
	block := pem.EncodeToMemory(&pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: map[string]string{"Proc-Type": "4,ENCRYPTED", "DEK-Info": "AES-128-CBC,0123456789ABCDEF"},
		Bytes:   []byte("not actually decryptable"),
	})

	_, _, err := InspectPrivateKey(block)
	if err == nil {
		t.Fatal("expected a passphrase-protected-key error, got none")
	}
	var fe *FieldError
	if !errors.As(err, &fe) {
		t.Fatalf("expected a *FieldError, got %T: %v", err, err)
	}
	if fe.Field != fieldPrivateKey {
		t.Errorf("expected field %q, got %q", fieldPrivateKey, fe.Field)
	}
	if fe.Message != msgPassphraseKey {
		t.Errorf("expected message %q, got %q", msgPassphraseKey, fe.Message)
	}
}

// ============ InspectCertificateChain: expiry ============

// ============ ClientAuthWarning ============

// ============ Chain length ============

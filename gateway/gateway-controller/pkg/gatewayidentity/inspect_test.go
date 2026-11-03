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

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
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

func requireFieldError(t *testing.T, err error, field, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %q on %s, got no error", message, field)
	}
	var fe *FieldError
	if !errors.As(err, &fe) {
		t.Fatalf("expected a *FieldError, got %T: %v", err, err)
	}
	if fe.Field != field || fe.Message != message {
		t.Fatalf("got %s: %q, want %s: %q", fe.Field, fe.Message, field, message)
	}
}

// ============ Private key blocks ============

func TestInspect_OpenSSLECKeyWithLeadingParametersBlock_Accepted(t *testing.T) {
	root := pki.NewRootCA(t, "EC Params Root CA")
	leaf := pki.NewLeaf(t, root, "ec-params-identity")

	bundle, err := Inspect(leaf.PEM(), leaf.ECKeyPEMWithParameters(), time.Now())
	if err != nil {
		t.Fatalf("expected an EC PARAMETERS block ahead of the key to be skipped, got: %v", err)
	}
	if bundle.KeyAlgorithm != "ECDSA" {
		t.Errorf("KeyAlgorithm = %q, want ECDSA", bundle.KeyAlgorithm)
	}
}

func TestInspectPrivateKey_NoKeyBlock_Refused(t *testing.T) {
	leaf := pki.NewLeaf(t, pki.NewRootCA(t, "No Key Root CA"), "no-key")
	onlyParameters := leaf.ECKeyPEMWithParameters()
	block, _ := pem.Decode(onlyParameters)

	_, _, err := InspectPrivateKey(pem.EncodeToMemory(block))
	requireFieldError(t, err, fieldPrivateKey, msgNotPEMKey)

	_, _, err = InspectPrivateKey(leaf.PEM())
	requireFieldError(t, err, fieldPrivateKey, msgNotPEMKey)
}

func TestInspectPrivateKey_TwoKeyBlocks_Refused(t *testing.T) {
	root := pki.NewRootCA(t, "Two Keys Root CA")
	first := pki.NewLeaf(t, root, "first")
	second := pki.NewLeaf(t, root, "second")

	_, _, err := InspectPrivateKey(append(first.KeyPEM(), second.KeyPEM()...))
	requireFieldError(t, err, fieldPrivateKey, msgSeveralKeys)
}

// ============ Key strength ============

func TestInspectPrivateKey_KeyStrength(t *testing.T) {
	for name, tc := range map[string]struct {
		key    func(t *testing.T) *pki.Entity
		refuse bool
	}{
		"RSA 1024": {key: func(t *testing.T) *pki.Entity {
			return pki.NewLeafWithKey(t, pki.NewRootCA(t, "RSA Root CA"), "rsa-1024", pki.NewRSAKey(t, 1024))
		}, refuse: true},
		"RSA 2048": {key: func(t *testing.T) *pki.Entity {
			return pki.NewLeafWithKey(t, pki.NewRootCA(t, "RSA Root CA"), "rsa-2048", pki.NewRSAKey(t, 2048))
		}},
		"ECDSA P-224": {key: func(t *testing.T) *pki.Entity {
			return pki.NewLeafWithKey(t, pki.NewRootCA(t, "EC Root CA"), "p224", pki.NewECKey(t, elliptic.P224()))
		}, refuse: true},
		"ECDSA P-384": {key: func(t *testing.T) *pki.Entity {
			return pki.NewLeafWithKey(t, pki.NewRootCA(t, "EC Root CA"), "p384", pki.NewECKey(t, elliptic.P384()))
		}},
		"ECDSA P-521": {key: func(t *testing.T) *pki.Entity {
			return pki.NewLeafWithKey(t, pki.NewRootCA(t, "EC Root CA"), "p521", pki.NewECKey(t, elliptic.P521()))
		}},
	} {
		t.Run(name, func(t *testing.T) {
			leaf := tc.key(t)
			_, err := Inspect(leaf.PEM(), leaf.KeyPEM(), time.Now())
			if tc.refuse {
				requireFieldError(t, err, fieldPrivateKey, msgWeakKey)
				return
			}
			if err != nil {
				t.Fatalf("expected the key to be accepted, got: %v", err)
			}
		})
	}
}

func TestInspectPrivateKey_WeakSEC1Key_Refused(t *testing.T) {
	leaf := pki.NewLeafWithKey(t, pki.NewRootCA(t, "EC Root CA"), "p224-sec1", pki.NewECKey(t, elliptic.P224()))

	_, _, err := InspectPrivateKey(leaf.ECKeyPEMWithParameters())
	requireFieldError(t, err, fieldPrivateKey, msgWeakKey)
}

// ============ Chain order and validity ============

func chainPEM(entities ...*pki.Entity) []byte {
	var out []byte
	for _, e := range entities {
		out = append(out, e.PEM()...)
	}
	return out
}

func TestInspectCertificateChain_OrderedChain_Accepted(t *testing.T) {
	root := pki.NewRootCA(t, "Ordered Root CA")
	intermediate := pki.NewIntermediate(t, root, "Ordered Intermediate CA")
	leaf := pki.NewLeaf(t, intermediate, "ordered-identity")

	chain, err := InspectCertificateChain(chainPEM(leaf, intermediate, root), time.Now())
	if err != nil {
		t.Fatalf("expected a leaf-first chain to be accepted, got: %v", err)
	}
	if len(chain) != 3 {
		t.Errorf("chain length = %d, want 3", len(chain))
	}
}

func TestInspectCertificateChain_OutOfOrder_Refused(t *testing.T) {
	root := pki.NewRootCA(t, "Unordered Root CA")
	intermediate := pki.NewIntermediate(t, root, "Unordered Intermediate CA")
	leaf := pki.NewLeaf(t, intermediate, "unordered-identity")
	unrelated := pki.NewRootCA(t, "Unrelated Root CA")

	for name, pemData := range map[string][]byte{
		"root before intermediate": chainPEM(leaf, root, intermediate),
		"unrelated issuer":         chainPEM(leaf, unrelated),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := InspectCertificateChain(pemData, time.Now())
			requireFieldError(t, err, fieldCertificate, msgChainOutOfOrder)
		})
	}
}

func TestInspectCertificateChain_ExpiredIntermediate_Refused(t *testing.T) {
	root := pki.NewRootCA(t, "Expired Intermediate Root CA")
	past := time.Now().Add(-48 * time.Hour)
	intermediate := pki.NewIntermediate(t, root, "Expired Intermediate CA", pki.WithValidity(past, time.Now().Add(-24*time.Hour)))
	leaf := pki.NewLeaf(t, intermediate, "under-expired-intermediate")

	_, err := InspectCertificateChain(chainPEM(leaf, intermediate, root), time.Now())
	requireFieldError(t, err, fieldCertificate,
		"an issuing certificate in the chain expired on "+intermediate.Cert.NotAfter.Format(time.RFC3339))
}

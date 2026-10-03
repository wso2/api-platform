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

// Package pki generates in-process test certificate authorities. It is not a
// _test.go file so other packages' tests can import it.
package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"testing"
	"time"
)

// Entity is a generated certificate plus the private key that created it.
type Entity struct {
	Cert *x509.Certificate
	Key  crypto.Signer
	DER  []byte // raw certificate DER, as produced by x509.CreateCertificate
}

// PEM returns the PEM-encoded certificate.
func (e *Entity) PEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: e.DER})
}

// KeyPEM returns the PEM-encoded, unencrypted PKCS#8 private key.
func (e *Entity) KeyPEM() []byte {
	der, err := x509.MarshalPKCS8PrivateKey(e.Key)
	if err != nil {
		// Only an unsupported key type fails, and this package makes RSA and
		// ECDSA keys.
		panic("pki: failed to marshal private key: " + err.Error())
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

// ECKeyPEMWithParameters returns an ECDSA key the way `openssl ecparam
// -genkey` writes it: an EC PARAMETERS block naming the curve, followed by
// the SEC1 EC PRIVATE KEY block. It panics for a non-ECDSA key.
func (e *Entity) ECKeyPEMWithParameters() []byte {
	key, ok := e.Key.(*ecdsa.PrivateKey)
	if !ok {
		panic("pki: ECKeyPEMWithParameters needs an ECDSA key")
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		panic("pki: failed to marshal EC private key: " + err.Error())
	}
	oid, ok := namedCurveOIDs[key.Curve]
	if !ok {
		panic("pki: no OID for curve " + key.Curve.Params().Name)
	}
	params, err := asn1.Marshal(oid)
	if err != nil {
		panic("pki: failed to marshal EC parameters: " + err.Error())
	}
	out := pem.EncodeToMemory(&pem.Block{Type: "EC PARAMETERS", Bytes: params})
	return append(out, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})...)
}

var namedCurveOIDs = map[elliptic.Curve]asn1.ObjectIdentifier{
	elliptic.P224(): {1, 3, 132, 0, 33},
	elliptic.P256(): {1, 2, 840, 10045, 3, 1, 7},
	elliptic.P384(): {1, 3, 132, 0, 34},
	elliptic.P521(): {1, 3, 132, 0, 35},
}

// NewECKey generates an ECDSA key on curve, for use with NewLeafWithKey.
func NewECKey(t testing.TB, curve elliptic.Curve) crypto.Signer {
	t.Helper()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatalf("pki: failed to generate %s key: %v", curve.Params().Name, err)
	}
	return key
}

// NewRSAKey generates an RSA key of bits, for use with NewLeafWithKey.
func NewRSAKey(t testing.TB, bits int) crypto.Signer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatalf("pki: failed to generate RSA-%d key: %v", bits, err)
	}
	return key
}

// Thumbprint returns the lowercase-hex SHA-256 digest of the certificate DER.
func (e *Entity) Thumbprint() string {
	sum := sha256.Sum256(e.DER)
	return hex.EncodeToString(sum[:])
}

// LeafOption customizes a certificate template before it is signed, for any
// kind of certificate.
type LeafOption func(*x509.Certificate)

// WithURISANs adds URI subject alternative names.
func WithURISANs(uris ...string) LeafOption {
	return func(c *x509.Certificate) {
		for _, u := range uris {
			parsed, err := url.Parse(u)
			if err != nil {
				panic("pki: invalid URI SAN " + u + ": " + err.Error())
			}
			c.URIs = append(c.URIs, parsed)
		}
	}
}

// WithDNSSANs adds DNS subject alternative names.
func WithDNSSANs(dns ...string) LeafOption {
	return func(c *x509.Certificate) {
		c.DNSNames = append(c.DNSNames, dns...)
	}
}

// WithIPSANs adds IP address subject alternative names, which a literal IP
// target is verified against.
func WithIPSANs(ips ...net.IP) LeafOption {
	return func(c *x509.Certificate) {
		c.IPAddresses = append(c.IPAddresses, ips...)
	}
}

// WithEKU overrides the certificate's extended key usage list.
func WithEKU(ekus ...x509.ExtKeyUsage) LeafOption {
	return func(c *x509.Certificate) {
		c.ExtKeyUsage = ekus
	}
}

// WithNoEKU removes the extended key usage extension.
func WithNoEKU() LeafOption {
	return func(c *x509.Certificate) {
		c.ExtKeyUsage = nil
	}
}

// WithValidity overrides the certificate's notBefore/notAfter window.
func WithValidity(notBefore, notAfter time.Time) LeafOption {
	return func(c *x509.Certificate) {
		c.NotBefore = notBefore
		c.NotAfter = notAfter
	}
}

func mustRandSerial(t testing.TB) *big.Int {
	t.Helper()
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		t.Fatalf("pki: failed to generate serial number: %v", err)
	}
	return n
}

func generateKey(t testing.TB) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("pki: failed to generate key: %v", err)
	}
	return key
}

func defaultValidity() (time.Time, time.Time) {
	now := time.Now()
	return now.Add(-1 * time.Hour), now.Add(10 * 365 * 24 * time.Hour)
}

// build creates a certificate for key signed by parent (self-signed when
// parent is nil).
func build(t testing.TB, subject pkix.Name, isCA bool, parent *Entity, key crypto.Signer, opts []LeafOption) *Entity {
	t.Helper()

	notBefore, notAfter := defaultValidity()

	tmpl := &x509.Certificate{
		SerialNumber:          mustRandSerial(t),
		Subject:               subject,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	if isCA {
		tmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	} else {
		tmpl.KeyUsage = x509.KeyUsageDigitalSignature
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	for _, opt := range opts {
		opt(tmpl)
	}

	var parentCert *x509.Certificate
	var signer crypto.Signer
	if parent == nil {
		parentCert = tmpl
		signer = key
	} else {
		parentCert = parent.Cert
		signer = parent.Key
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parentCert, key.Public(), signer)
	if err != nil {
		t.Fatalf("pki: failed to create certificate for %q: %v", subject.CommonName, err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("pki: failed to parse generated certificate for %q: %v", subject.CommonName, err)
	}

	return &Entity{Cert: cert, Key: key, DER: der}
}

// NewRootCA creates a self-signed CA certificate.
func NewRootCA(t testing.TB, cn string, opts ...LeafOption) *Entity {
	t.Helper()
	return build(t, pkix.Name{CommonName: cn}, true, nil, generateKey(t), opts)
}

// NewIntermediate creates a CA certificate issued by parent.
func NewIntermediate(t testing.TB, parent *Entity, cn string, opts ...LeafOption) *Entity {
	t.Helper()
	return build(t, pkix.Name{CommonName: cn}, true, parent, generateKey(t), opts)
}

// NewLeaf creates a non-CA (clientAuth by default) certificate issued by parent.
func NewLeaf(t testing.TB, parent *Entity, cn string, opts ...LeafOption) *Entity {
	t.Helper()
	return build(t, pkix.Name{CommonName: cn}, false, parent, generateKey(t), opts)
}

// NewLeafWithKey is NewLeaf for a caller-supplied key.
func NewLeafWithKey(t testing.TB, parent *Entity, cn string, key crypto.Signer, opts ...LeafOption) *Entity {
	t.Helper()
	return build(t, pkix.Name{CommonName: cn}, false, parent, key, opts)
}

// NewSelfSignedLeaf creates a non-CA certificate that signs itself.
func NewSelfSignedLeaf(t testing.TB, cn string, opts ...LeafOption) *Entity {
	t.Helper()
	return build(t, pkix.Name{CommonName: cn}, false, nil, generateKey(t), opts)
}

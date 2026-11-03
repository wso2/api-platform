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

package testpki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Fixture is one generated certificate with its private key.
type Fixture struct {
	// Name addresses the fixture, for example "client-valid".
	Name string
	// Certificate is the parsed certificate.
	Certificate *x509.Certificate
	// CertPEM is the certificate alone.
	CertPEM []byte
	// KeyPEM is the PKCS#8 private key.
	KeyPEM []byte
	// ChainPEM holds the issuing intermediates, immediate issuer first, up to but excluding
	// the root. It is empty for a root, a self-signed leaf and anything a root signed directly.
	ChainPEM []byte
	// Thumbprint is the lowercase hexadecimal SHA-256 of the certificate's DER encoding.
	Thumbprint string
}

// TLSCertificate returns the fixture as a client certificate, followed by its issuing
// intermediates when withChain is true.
func (f *Fixture) TLSCertificate(withChain bool) (tls.Certificate, error) {
	certPEM := f.CertPEM
	if withChain {
		if len(f.ChainPEM) == 0 {
			return tls.Certificate{}, fmt.Errorf("testpki: fixture %q has no issuing chain", f.Name)
		}
		certPEM = append(append([]byte{}, f.CertPEM...), f.ChainPEM...)
	}
	cert, err := tls.X509KeyPair(certPEM, f.KeyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("testpki: loading fixture %q: %w", f.Name, err)
	}
	return cert, nil
}

// Set is an immutable collection of fixtures.
type Set struct {
	fixtures map[string]*Fixture
}

// Get returns the named fixture.
func (s *Set) Get(name string) (*Fixture, error) {
	if s == nil {
		return nil, fmt.Errorf("testpki: no fixture set")
	}
	f, ok := s.fixtures[strings.TrimSpace(name)]
	if !ok {
		return nil, fmt.Errorf("testpki: unknown fixture %q", name)
	}
	return f, nil
}

// Names returns every fixture name in sorted order.
func (s *Set) Names() []string {
	if s == nil {
		return nil
	}
	names := make([]string, 0, len(s.fixtures))
	for name := range s.fixtures {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Default returns the fixture set for this process, generated on first use.
var Default = sync.OnceValues(func() (*Set, error) {
	return Generate(time.Now())
})

// issuer is a fixture that signs others, with its signing key and ancestry.
type issuer struct {
	fixture *Fixture
	key     *ecdsa.PrivateKey
	isRoot  bool
	// ancestors are the issuing intermediates above this certificate, immediate issuer
	// first, excluding the root.
	ancestors []*issuer
}

// spec describes one fixture to issue.
type spec struct {
	name      string
	subject   pkix.Name
	parent    string // empty: self-signed
	isCA      bool
	notBefore time.Time
	notAfter  time.Time
	uriSANs   []string
	dnsSANs   []string
	ekus      []x509.ExtKeyUsage // nil: clientAuth for a leaf, none for an authority
	noEKU     bool               // no extended key usage extension at all
	reuseKey  string             // reuse the named fixture's key
}

// Generate issues the whole fixture set relative to now. Fixtures are issued in
// declaration order, so a parent always precedes what it signs.
func Generate(now time.Time) (*Set, error) {
	if now.IsZero() {
		return nil, fmt.Errorf("testpki: generation time is zero")
	}
	g := &generator{now: now, issued: map[string]*issuer{}}
	for _, s := range append(catalogue(now), backendCatalogue()...) {
		if err := g.issue(s); err != nil {
			return nil, err
		}
	}
	fixtures := make(map[string]*Fixture, len(g.issued))
	for name, i := range g.issued {
		fixtures[name] = i.fixture
	}
	return &Set{fixtures: fixtures}, nil
}

// catalogue lists every fixture. Subjects, SANs, key usages and validity windows are
// what the scenarios assert against, so a change here is a change to those scenarios.
func catalogue(now time.Time) []spec {
	expiredFrom, expiredTo := now.Add(-2*365*24*time.Hour), now.Add(-365*24*time.Hour)
	futureFrom, futureTo := now.Add(365*24*time.Hour), now.Add(11*365*24*time.Hour)
	soonFrom, soonTo := now.Add(-time.Hour), now.Add(20*24*time.Hour)
	partnerA := func(cn string) pkix.Name { return pkix.Name{CommonName: cn, Organization: []string{"Partner A"}} }
	cn := func(name string) pkix.Name { return pkix.Name{CommonName: name} }
	serverAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	caA := partnerA("Partner A Root CA")

	return []spec{
		// Authorities.
		{name: "ca-a", subject: caA, isCA: true},
		{name: "ca-b", subject: pkix.Name{CommonName: "Partner B Root CA", Organization: []string{"Partner B"}}, isCA: true},
		{name: "ca-a-intermediate", subject: partnerA("Partner A Issuing CA 1"), parent: "ca-a", isCA: true},
		{name: "ca-a-intermediate-2", subject: partnerA("Partner A Issuing CA 2"), parent: "ca-a", isCA: true},
		{name: "ca-a-other-intermediate", subject: partnerA("Partner A Devices CA"), parent: "ca-a", isCA: true},
		{name: "ca-b-same-dn", subject: caA, isCA: true},
		{name: "ca-expired", subject: cn("Expired Root CA"), isCA: true, notBefore: expiredFrom, notAfter: expiredTo},
		{name: "ca-not-yet-valid", subject: cn("Not Yet Valid Root CA"), isCA: true, notBefore: futureFrom, notAfter: futureTo},
		{name: "ca-expires-soon", subject: cn("Expires Soon Root CA"), isCA: true, notBefore: soonFrom, notAfter: soonTo},
		{name: "backend-ca", subject: cn("Backend Root CA"), isCA: true},

		// Client leaves.
		{name: "client-valid", subject: cn("client-valid"), parent: "ca-a",
			uriSANs: []string{"urn:partner-a:payments"}, dnsSANs: []string{"client-valid.partner-a.test"}},
		{name: "client-valid-extra-sans", subject: cn("client-valid-extra-sans"), parent: "ca-a",
			uriSANs: []string{"urn:partner-a:payments", "urn:partner-a:other"},
			dnsSANs: []string{"client-valid.partner-a.test", "other.partner-a.test"}},
		{name: "client-renewed", subject: cn("client-valid"), parent: "ca-a",
			uriSANs: []string{"urn:partner-a:payments"}, dnsSANs: []string{"client-valid.partner-a.test"}},
		{name: "client-from-lookalike-ca", subject: cn("client-lookalike"), parent: "ca-b-same-dn",
			uriSANs: []string{"urn:partner-a:payments"}},
		{name: "client-via-intermediate", subject: cn("client-via-intermediate"), parent: "ca-a-intermediate"},
		{name: "client-via-intermediate-2", subject: cn("client-via-intermediate-2"), parent: "ca-a-intermediate-2"},
		{name: "client-via-other-intermediate", subject: cn("client-via-other-intermediate"), parent: "ca-a-other-intermediate"},
		{name: "client-expired", subject: cn("client-expired"), parent: "ca-a", notBefore: expiredFrom, notAfter: expiredTo},
		{name: "client-not-yet-valid", subject: cn("client-not-yet-valid"), parent: "ca-a", notBefore: futureFrom, notAfter: futureTo},
		{name: "client-wrong-ca", subject: cn("client-wrong-ca"), parent: "ca-b"},
		{name: "client-same-cn-ca-b", subject: cn("client-valid"), parent: "ca-b"},
		{name: "client-no-san", subject: cn("client-no-san"), parent: "ca-a"},
		{name: "client-multi-san", subject: cn("client-multi-san"), parent: "ca-a",
			uriSANs: []string{"urn:partner-a:first", "urn:partner-a:second"},
			dnsSANs: []string{"first.partner-a.test", "second.partner-a.test"}},
		{name: "client-serverauth-only", subject: cn("client-serverauth-only"), parent: "ca-a", ekus: serverAuth},

		// A chain five issuers deep: ca-a, four intermediates, then the leaf.
		{name: "client-chain-depth-5-int1", subject: cn("Chain Depth CA 1"), parent: "ca-a", isCA: true},
		{name: "client-chain-depth-5-int2", subject: cn("Chain Depth CA 2"), parent: "client-chain-depth-5-int1", isCA: true},
		{name: "client-chain-depth-5-int3", subject: cn("Chain Depth CA 3"), parent: "client-chain-depth-5-int2", isCA: true},
		{name: "client-chain-depth-5-int4", subject: cn("Chain Depth CA 4"), parent: "client-chain-depth-5-int3", isCA: true},
		{name: "client-chain-depth-5", subject: cn("client-chain-depth-5"), parent: "client-chain-depth-5-int4"},

		// Self-signed leaves and a certificate a leaf signed.
		{name: "client-selfsigned", subject: cn("acme-device-1"), uriSANs: []string{"urn:acme:device-1"}},
		{name: "client-selfsigned-b", subject: cn("acme-device-2")},
		{name: "client-selfsigned-renewed", subject: cn("acme-device-1"), reuseKey: "client-selfsigned"},
		{name: "client-signed-by-leaf", subject: cn("client-signed-by-leaf"), parent: "client-selfsigned"},

		// Gateway identities.
		{name: "gw-identity-a", subject: cn("gateway-a"), parent: "ca-a"},
		{name: "gw-identity-b", subject: cn("gateway-b"), parent: "ca-b"},
		{name: "gw-identity-via-intermediate", subject: cn("gateway-via-intermediate"), parent: "ca-a-intermediate"},
		{name: "gw-identity-no-eku", subject: cn("gw-identity-no-eku"), parent: "ca-a", noEKU: true},
		{name: "gw-identity-serverauth", subject: cn("gw-identity-serverauth"), parent: "ca-a", ekus: serverAuth},

		// Front proxies relaying a caller's certificate in a header.
		{name: "edge-lb-ca", subject: cn("Edge LB CA"), isCA: true},
		{name: "edge-lb", subject: cn("edge-lb"), parent: "edge-lb-ca", dnsSANs: []string{"edge-lb.internal"}},
		{name: "corp-ca", subject: cn("Corp CA"), isCA: true},
		{name: "edge-lb-corp", subject: cn("edge-lb-corp"), parent: "corp-ca", dnsSANs: []string{"lb.corp.test"}},
		{name: "corp-other-service", subject: cn("corp-other-service"), parent: "corp-ca", dnsSANs: []string{"other.corp.test"}},
	}
}

type generator struct {
	now    time.Time
	issued map[string]*issuer
}

func (g *generator) issue(s spec) error {
	if _, dup := g.issued[s.name]; dup {
		return fmt.Errorf("testpki: fixture %q is declared twice", s.name)
	}
	key, err := g.keyFor(s)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("testpki: serial for %q: %w", s.name, err)
	}
	notBefore, notAfter := s.notBefore, s.notAfter
	if notBefore.IsZero() && notAfter.IsZero() {
		notBefore, notAfter = g.now.Add(-time.Hour), g.now.Add(10*365*24*time.Hour)
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               s.subject,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		IsCA:                  s.isCA,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		DNSNames:              append([]string(nil), s.dnsSANs...),
	}
	if s.isCA {
		template.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	}
	switch {
	case s.noEKU:
	case len(s.ekus) > 0:
		template.ExtKeyUsage = s.ekus
	case !s.isCA:
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	for _, raw := range s.uriSANs {
		parsed, parseErr := url.Parse(raw)
		if parseErr != nil {
			return fmt.Errorf("testpki: URI SAN %q of %q: %w", raw, s.name, parseErr)
		}
		template.URIs = append(template.URIs, parsed)
	}

	parentCert, signer := template, key
	var parent *issuer
	if s.parent != "" {
		p, ok := g.issued[s.parent]
		if !ok {
			return fmt.Errorf("testpki: fixture %q names unknown or later issuer %q", s.name, s.parent)
		}
		parent, parentCert, signer = p, p.fixture.Certificate, p.key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parentCert, key.Public(), signer)
	if err != nil {
		return fmt.Errorf("testpki: issuing %q: %w", s.name, err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return fmt.Errorf("testpki: parsing %q: %w", s.name, err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("testpki: encoding the key of %q: %w", s.name, err)
	}
	sum := sha256.Sum256(der)
	i := &issuer{
		fixture: &Fixture{
			Name:        s.name,
			Certificate: cert,
			CertPEM:     pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
			KeyPEM:      pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
			Thumbprint:  hex.EncodeToString(sum[:]),
		},
		key:    key,
		isRoot: parent == nil && s.isCA,
	}
	if parent != nil && !parent.isRoot {
		i.ancestors = append([]*issuer{parent}, parent.ancestors...)
	}
	for _, a := range i.ancestors {
		i.fixture.ChainPEM = append(i.fixture.ChainPEM, a.fixture.CertPEM...)
	}
	g.issued[s.name] = i
	return nil
}

func (g *generator) keyFor(s spec) (*ecdsa.PrivateKey, error) {
	if s.reuseKey != "" {
		donor, ok := g.issued[s.reuseKey]
		if !ok {
			return nil, fmt.Errorf("testpki: fixture %q reuses the key of unknown fixture %q", s.name, s.reuseKey)
		}
		return donor.key, nil
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("testpki: key for %q: %w", s.name, err)
	}
	return key, nil
}

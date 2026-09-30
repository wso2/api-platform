//go:build ignore

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

// Command generate writes the PEM certificate and key fixtures for the mTLS
// integration tests, overwriting the previous set. Run it from gateway/it
// with `go run ./resources/mtls-pki/generate.go`.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// issued is a generated fixture: a certificate plus the private key that
// created it, and enough lineage information to build a chain file for
// fixtures issued through one or more intermediates.
type issued struct {
	name string
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	der  []byte

	// isRoot is true for a self-signed CA fixture (parent == nil, isCA == true).
	isRoot bool

	// ancestors holds this fixture's issuers from its immediate parent up to
	// (but excluding) the root, immediate parent first. Empty for a root and
	// for anything issued directly by a root.
	ancestors []*issued
}

func (i *issued) pemCert() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: i.der})
}

func (i *issued) thumbprint() string {
	sum := sha256.Sum256(i.der)
	return hex.EncodeToString(sum[:])
}

// issueOpts configures a single certificate issuance.
type issueOpts struct {
	subject   pkix.Name
	parent    *issued // nil => self-signed
	isCA      bool
	notBefore time.Time
	notAfter  time.Time
	uriSANs   []string
	dnsSANs   []string
	ekus      []x509.ExtKeyUsage // nil => default (clientAuth for a leaf, none for a CA)
	noEKU     bool               // force no ExtKeyUsage extension at all, overriding the default
	reuseKey  *ecdsa.PrivateKey  // reuse an existing key instead of generating a new one
}

var manifest = map[string]map[string]string{}

func mustRandSerial() *big.Int {
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	n, err := rand.Int(rand.Reader, max)
	check(err)
	return n
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func defaultValidity() (time.Time, time.Time) {
	now := time.Now()
	return now.Add(-1 * time.Hour), now.Add(10 * 365 * 24 * time.Hour)
}

func issue(name string, opts issueOpts) *issued {
	key := opts.reuseKey
	if key == nil {
		var err error
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		check(err)
	}

	notBefore, notAfter := opts.notBefore, opts.notAfter
	if notBefore.IsZero() && notAfter.IsZero() {
		notBefore, notAfter = defaultValidity()
	}

	tmpl := &x509.Certificate{
		SerialNumber:          mustRandSerial(),
		Subject:               opts.subject,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		IsCA:                  opts.isCA,
	}

	if opts.isCA {
		tmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	} else {
		tmpl.KeyUsage = x509.KeyUsageDigitalSignature
	}

	switch {
	case opts.noEKU:
		// leave tmpl.ExtKeyUsage nil: no EKU extension at all
	case len(opts.ekus) > 0:
		tmpl.ExtKeyUsage = opts.ekus
	case !opts.isCA:
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	for _, u := range opts.uriSANs {
		parsed, err := url.Parse(u)
		check(err)
		tmpl.URIs = append(tmpl.URIs, parsed)
	}
	tmpl.DNSNames = append(tmpl.DNSNames, opts.dnsSANs...)

	var parentCert *x509.Certificate
	var signerKey *ecdsa.PrivateKey
	if opts.parent == nil {
		parentCert = tmpl
		signerKey = key
	} else {
		parentCert = opts.parent.cert
		signerKey = opts.parent.key
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parentCert, key.Public(), signerKey)
	check(err)
	cert, err := x509.ParseCertificate(der)
	check(err)

	result := &issued{
		name:   name,
		cert:   cert,
		key:    key,
		der:    der,
		isRoot: opts.parent == nil && opts.isCA,
	}
	if opts.parent != nil && !opts.parent.isRoot {
		result.ancestors = append([]*issued{opts.parent}, opts.parent.ancestors...)
	}

	issuerName := name
	if opts.parent != nil {
		issuerName = opts.parent.name
	}
	manifest[name] = map[string]string{
		"subject":    cert.Subject.String(),
		"issuer":     issuerName,
		"thumbprint": result.thumbprint(),
		"notAfter":   cert.NotAfter.UTC().Format(time.RFC3339),
	}

	return result
}

func writeFile(dir, name string, data []byte) {
	check(os.WriteFile(filepath.Join(dir, name), data, 0o644))
}

func writeKey(dir string, i *issued) {
	der, err := x509.MarshalPKCS8PrivateKey(i.key)
	check(err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	writeFile(dir, i.name+".key", pemBytes)
}

func writeCert(dir string, i *issued) {
	writeFile(dir, i.name+".crt", i.pemCert())
}

func writeChainIfAny(dir string, i *issued) {
	if len(i.ancestors) == 0 {
		return
	}
	chain := append([]byte{}, i.pemCert()...)
	for _, a := range i.ancestors {
		chain = append(chain, a.pemCert()...)
	}
	writeFile(dir, i.name+".chain.crt", chain)
}

func writeAll(dir string, i *issued) {
	writeCert(dir, i)
	writeKey(dir, i)
	writeChainIfAny(dir, i)
}

// writeEncryptedKey writes "<name>.encrypted.key", a passphrase-protected
// PKCS#8 key, using `openssl pkcs8` because crypto/x509 has no PKCS#8
// encryptor. Without openssl it wraps the plaintext DER in an "ENCRYPTED
// PRIVATE KEY" PEM block, which is enough for a header-based rejection check.
func writeEncryptedKey(dir string, i *issued, passphrase string) {
	keyPath := filepath.Join(dir, i.name+".key")
	outPath := filepath.Join(dir, i.name+".encrypted.key")

	if opensslPath, err := exec.LookPath("openssl"); err == nil {
		cmd := exec.Command(opensslPath, "pkcs8", "-topk8", "-v2", "aes-256-cbc",
			"-passout", "pass:"+passphrase, "-in", keyPath, "-out", outPath)
		if out, err := cmd.CombinedOutput(); err == nil {
			manifest[i.name+".encrypted"] = manifest[i.name]
			return
		} else {
			fmt.Fprintf(os.Stderr, "mtls-pki: openssl pkcs8 encryption failed for %s, falling back to a header-only fixture: %s\n", i.name, out)
		}
	}

	der, err := x509.MarshalPKCS8PrivateKey(i.key)
	check(err)
	writeFile(dir, i.name+".encrypted.key", pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: der}))
	manifest[i.name+".encrypted"] = manifest[i.name]
}

func main() {
	dir, err := os.Getwd()
	check(err)
	// Write to resources/mtls-pki when run from gateway/it, else to the cwd.
	dir = filepath.Join(dir, "resources", "mtls-pki")
	if _, statErr := os.Stat(dir); statErr != nil {
		dir = "."
	}

	count := 0
	track := func(i *issued) *issued {
		writeAll(dir, i)
		count++
		return i
	}

	expiredNotBefore := time.Now().Add(-2 * 365 * 24 * time.Hour)
	expiredNotAfter := time.Now().Add(-1 * 365 * 24 * time.Hour)
	notYetValidNotBefore := time.Now().Add(1 * 365 * 24 * time.Hour)
	notYetValidNotAfter := time.Now().Add(11 * 365 * 24 * time.Hour)
	expiresSoonNotBefore := time.Now().Add(-1 * time.Hour)
	expiresSoonNotAfter := time.Now().Add(20 * 24 * time.Hour)

	// Authorities
	caA := track(issue("ca-a", issueOpts{
		subject: pkix.Name{CommonName: "Partner A Root CA", Organization: []string{"Partner A"}},
		isCA:    true,
	}))
	caB := track(issue("ca-b", issueOpts{
		subject: pkix.Name{CommonName: "Partner B Root CA", Organization: []string{"Partner B"}},
		isCA:    true,
	}))
	caAIntermediate := track(issue("ca-a-intermediate", issueOpts{
		subject: pkix.Name{CommonName: "Partner A Issuing CA 1", Organization: []string{"Partner A"}},
		parent:  caA,
		isCA:    true,
	}))
	caAIntermediate2 := track(issue("ca-a-intermediate-2", issueOpts{
		subject: pkix.Name{CommonName: "Partner A Issuing CA 2", Organization: []string{"Partner A"}},
		parent:  caA,
		isCA:    true,
	}))
	caAOtherIntermediate := track(issue("ca-a-other-intermediate", issueOpts{
		subject: pkix.Name{CommonName: "Partner A Devices CA", Organization: []string{"Partner A"}},
		parent:  caA,
		isCA:    true,
	}))
	caBSameDN := track(issue("ca-b-same-dn", issueOpts{
		subject: caA.cert.Subject, // byte-identical DN to ca-a, different key
		isCA:    true,
	}))
	track(issue("ca-expired", issueOpts{
		subject:   pkix.Name{CommonName: "Expired Root CA"},
		isCA:      true,
		notBefore: expiredNotBefore,
		notAfter:  expiredNotAfter,
	}))
	track(issue("ca-not-yet-valid", issueOpts{
		subject:   pkix.Name{CommonName: "Not Yet Valid Root CA"},
		isCA:      true,
		notBefore: notYetValidNotBefore,
		notAfter:  notYetValidNotAfter,
	}))
	track(issue("ca-expires-soon", issueOpts{
		subject:   pkix.Name{CommonName: "Expires Soon Root CA"},
		isCA:      true,
		notBefore: expiresSoonNotBefore,
		notAfter:  expiresSoonNotAfter,
	}))
	backendCA := track(issue("backend-ca", issueOpts{
		subject: pkix.Name{CommonName: "Backend Root CA"},
		isCA:    true,
	}))
	backendCAB := track(issue("backend-ca-b", issueOpts{
		subject: pkix.Name{CommonName: "Backend CA B"},
		isCA:    true,
	}))

	// Client leaves
	clientValid := track(issue("client-valid", issueOpts{
		subject: pkix.Name{CommonName: "client-valid"},
		parent:  caA,
		uriSANs: []string{"urn:partner-a:payments"},
		dnsSANs: []string{"client-valid.partner-a.test"},
	}))
	track(issue("client-valid-extra-sans", issueOpts{
		subject: pkix.Name{CommonName: "client-valid-extra-sans"},
		parent:  caA,
		uriSANs: []string{"urn:partner-a:payments", "urn:partner-a:other"},
		dnsSANs: []string{"client-valid.partner-a.test", "other.partner-a.test"},
	}))
	track(issue("client-from-lookalike-ca", issueOpts{
		subject: pkix.Name{CommonName: "client-lookalike"},
		parent:  caBSameDN,
		uriSANs: []string{"urn:partner-a:payments"},
	}))
	track(issue("client-via-intermediate", issueOpts{
		subject: pkix.Name{CommonName: "client-via-intermediate"},
		parent:  caAIntermediate,
	}))
	track(issue("client-via-intermediate-2", issueOpts{
		subject: pkix.Name{CommonName: "client-via-intermediate-2"},
		parent:  caAIntermediate2,
	}))
	track(issue("client-via-other-intermediate", issueOpts{
		subject: pkix.Name{CommonName: "client-via-other-intermediate"},
		parent:  caAOtherIntermediate,
	}))
	track(issue("client-expired", issueOpts{
		subject:   pkix.Name{CommonName: "client-expired"},
		parent:    caA,
		notBefore: expiredNotBefore,
		notAfter:  expiredNotAfter,
	}))
	track(issue("client-not-yet-valid", issueOpts{
		subject:   pkix.Name{CommonName: "client-not-yet-valid"},
		parent:    caA,
		notBefore: notYetValidNotBefore,
		notAfter:  notYetValidNotAfter,
	}))
	track(issue("client-wrong-ca", issueOpts{
		subject: pkix.Name{CommonName: "client-wrong-ca"},
		parent:  caB,
	}))
	track(issue("client-same-cn-ca-b", issueOpts{
		subject: pkix.Name{CommonName: "client-valid"},
		parent:  caB,
	}))
	track(issue("client-no-san", issueOpts{
		subject: pkix.Name{CommonName: "client-no-san"},
		parent:  caA,
	}))
	track(issue("client-multi-san", issueOpts{
		subject: pkix.Name{CommonName: "client-multi-san"},
		parent:  caA,
		uriSANs: []string{"urn:partner-a:first", "urn:partner-a:second"},
		dnsSANs: []string{"first.partner-a.test", "second.partner-a.test"},
	}))
	track(issue("client-serverauth-only", issueOpts{
		subject: pkix.Name{CommonName: "client-serverauth-only"},
		parent:  caA,
		ekus:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}))

	// client-chain-depth-5: ca-a -> int1 -> int2 -> int3 -> int4 -> leaf
	depth1 := track(issue("client-chain-depth-5-int1", issueOpts{
		subject: pkix.Name{CommonName: "Chain Depth CA 1"},
		parent:  caA,
		isCA:    true,
	}))
	depth2 := track(issue("client-chain-depth-5-int2", issueOpts{
		subject: pkix.Name{CommonName: "Chain Depth CA 2"},
		parent:  depth1,
		isCA:    true,
	}))
	depth3 := track(issue("client-chain-depth-5-int3", issueOpts{
		subject: pkix.Name{CommonName: "Chain Depth CA 3"},
		parent:  depth2,
		isCA:    true,
	}))
	depth4 := track(issue("client-chain-depth-5-int4", issueOpts{
		subject: pkix.Name{CommonName: "Chain Depth CA 4"},
		parent:  depth3,
		isCA:    true,
	}))
	track(issue("client-chain-depth-5", issueOpts{
		subject: pkix.Name{CommonName: "client-chain-depth-5"},
		parent:  depth4,
	}))

	clientSelfsigned := track(issue("client-selfsigned", issueOpts{
		subject: pkix.Name{CommonName: "acme-device-1"},
		uriSANs: []string{"urn:acme:device-1"},
	}))
	track(issue("client-selfsigned-b", issueOpts{
		subject: pkix.Name{CommonName: "acme-device-2"},
	}))
	track(issue("client-selfsigned-renewed", issueOpts{
		subject:  clientSelfsigned.cert.Subject,
		reuseKey: clientSelfsigned.key,
	}))
	track(issue("client-signed-by-leaf", issueOpts{
		subject: pkix.Name{CommonName: "client-signed-by-leaf"},
		parent:  clientSelfsigned, // a CA:FALSE issuer
	}))
	track(issue("client-renewed", issueOpts{
		// Same subject and SANs as client-valid with a fresh key, so only the
		// thumbprint differs.
		subject: clientValid.cert.Subject,
		parent:  caA,
		uriSANs: []string{"urn:partner-a:payments"},
		dnsSANs: []string{"client-valid.partner-a.test"},
	}))

	// Outbound mirror set
	track(issue("backend-server", issueOpts{
		subject: pkix.Name{CommonName: "backend-server"},
		parent:  backendCA,
		dnsSANs: []string{"mock-openapi-https", "localhost", "sample-backend"},
		ekus:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}))
	track(issue("backend-server-a", issueOpts{
		subject: pkix.Name{CommonName: "backend-server-a"},
		parent:  backendCA,
		dnsSANs: []string{"mtls-backend-a", "localhost"},
		ekus:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}))
	track(issue("backend-server-b", issueOpts{
		subject: pkix.Name{CommonName: "backend-server-b"},
		parent:  backendCAB,
		dnsSANs: []string{"mtls-backend-b"},
		ekus:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}))
	track(issue("backend-server-wronghost", issueOpts{
		subject: pkix.Name{CommonName: "backend-server-wronghost"},
		parent:  backendCA,
		dnsSANs: []string{"not-this-host.test"},
		ekus:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}))
	gwIdentityA := track(issue("gw-identity-a", issueOpts{
		subject: pkix.Name{CommonName: "gateway-a"},
		parent:  caA,
	}))
	writeEncryptedKey(dir, gwIdentityA, "test")
	track(issue("gw-identity-b", issueOpts{
		subject: pkix.Name{CommonName: "gateway-b"},
		parent:  caB,
	}))
	track(issue("gw-identity-via-intermediate", issueOpts{
		subject: pkix.Name{CommonName: "gateway-via-intermediate"},
		parent:  caAIntermediate,
	}))
	track(issue("gw-identity-no-eku", issueOpts{
		subject: pkix.Name{CommonName: "gw-identity-no-eku"},
		parent:  caA,
		noEKU:   true,
	}))
	track(issue("gw-identity-serverauth", issueOpts{
		subject: pkix.Name{CommonName: "gw-identity-serverauth"},
		parent:  caA,
		ekus:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}))

	// Header-relay fixtures: a front proxy identity and a second authority,
	// showing that a SAN-narrowed relay entry vouches only for that proxy.
	edgeLBCA := track(issue("edge-lb-ca", issueOpts{
		subject: pkix.Name{CommonName: "Edge LB CA"},
		isCA:    true,
	}))
	track(issue("edge-lb", issueOpts{
		subject: pkix.Name{CommonName: "edge-lb"},
		parent:  edgeLBCA,
		dnsSANs: []string{"edge-lb.internal"},
	}))
	corpCA := track(issue("corp-ca", issueOpts{
		subject: pkix.Name{CommonName: "Corp CA"},
		isCA:    true,
	}))
	track(issue("edge-lb-corp", issueOpts{
		subject: pkix.Name{CommonName: "edge-lb-corp"},
		parent:  corpCA,
		dnsSANs: []string{"lb.corp.test"},
	}))
	track(issue("corp-other-service", issueOpts{
		subject: pkix.Name{CommonName: "corp-other-service"},
		parent:  corpCA,
		dnsSANs: []string{"other.corp.test"},
	}))

	// key-mismatch.key: a key that matches no certificate.
	mismatchKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	check(err)
	mismatchDER, err := x509.MarshalPKCS8PrivateKey(mismatchKey)
	check(err)
	writeFile(dir, "key-mismatch.key", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mismatchDER}))

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	check(err)
	writeFile(dir, "manifest.json", manifestBytes)

	fmt.Printf("mtls-pki: generated %d certificate fixtures (+key-mismatch.key, manifest.json) into %s\n", count, dir)
}

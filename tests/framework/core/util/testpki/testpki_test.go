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
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"sync"
	"testing"
	"time"
)

func generated(t *testing.T) *Set {
	t.Helper()
	set, err := Generate(time.Now())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return set
}

func mustGet(t *testing.T, set *Set, name string) *Fixture {
	t.Helper()
	f, err := set.Get(name)
	if err != nil {
		t.Fatalf("Get(%q): %v", name, err)
	}
	return f
}

func verify(t *testing.T, leaf *Fixture, root *Fixture, intermediates ...*Fixture) error {
	t.Helper()
	roots := x509.NewCertPool()
	roots.AddCert(root.Certificate)
	pool := x509.NewCertPool()
	for _, i := range intermediates {
		pool.AddCert(i.Certificate)
	}
	_, err := leaf.Certificate.Verify(x509.VerifyOptions{
		Roots: roots, Intermediates: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	return err
}

func TestGenerateRejectsZeroTime(t *testing.T) {
	if _, err := Generate(time.Time{}); err == nil {
		t.Fatal("Generate(zero) succeeded, want an error")
	}
}

func TestEveryFixtureIsConsistent(t *testing.T) {
	set := generated(t)
	names := set.Names()
	if len(names) != len(catalogue(time.Now()))+len(backendCatalogue()) {
		t.Fatalf("got %d fixtures, want %d", len(names), len(catalogue(time.Now()))+len(backendCatalogue()))
	}
	for _, name := range names {
		f := mustGet(t, set, name)
		block, _ := pem.Decode(f.CertPEM)
		if block == nil || block.Type != "CERTIFICATE" {
			t.Fatalf("%s: CertPEM is not one PEM certificate", name)
		}
		sum := sha256.Sum256(block.Bytes)
		if f.Thumbprint != hex.EncodeToString(sum[:]) {
			t.Fatalf("%s: thumbprint does not match the certificate", name)
		}
		if _, err := f.TLSCertificate(false); err != nil {
			t.Fatalf("%s: key does not match certificate: %v", name, err)
		}
	}
}

func TestGetRejectsUnknownAndNilSet(t *testing.T) {
	if _, err := generated(t).Get("no-such-fixture"); err == nil {
		t.Fatal("Get(unknown) succeeded, want an error")
	}
	var nilSet *Set
	if _, err := nilSet.Get("ca-a"); err == nil {
		t.Fatal("nil Set Get succeeded, want an error")
	}
	if nilSet.Names() != nil {
		t.Fatal("nil Set Names returned names")
	}
}

func TestChainsFollowTheIssuers(t *testing.T) {
	set := generated(t)
	caA := mustGet(t, set, "ca-a")
	intermediate := mustGet(t, set, "ca-a-intermediate")
	viaIntermediate := mustGet(t, set, "client-via-intermediate")

	if len(mustGet(t, set, "client-valid").ChainPEM) != 0 {
		t.Fatal("a leaf a root signed directly carries a chain")
	}
	if !bytes.Equal(viaIntermediate.ChainPEM, intermediate.CertPEM) {
		t.Fatal("client-via-intermediate's chain is not its issuing intermediate")
	}
	if err := verify(t, viaIntermediate, caA); err == nil {
		t.Fatal("client-via-intermediate verified against ca-a without its intermediate")
	}
	if err := verify(t, viaIntermediate, caA, intermediate); err != nil {
		t.Fatalf("client-via-intermediate does not chain to ca-a: %v", err)
	}
	deep := mustGet(t, set, "client-chain-depth-5")
	if got := bytes.Count(deep.ChainPEM, []byte("BEGIN CERTIFICATE")); got != 4 {
		t.Fatalf("client-chain-depth-5 chain holds %d certificates, want 4", got)
	}
	withChain, err := deep.TLSCertificate(true)
	if err != nil {
		t.Fatalf("TLSCertificate(true): %v", err)
	}
	if len(withChain.Certificate) != 5 {
		t.Fatalf("client-chain-depth-5 with chain presents %d certificates, want 5", len(withChain.Certificate))
	}
	if _, err := mustGet(t, set, "client-valid").TLSCertificate(true); err == nil {
		t.Fatal("TLSCertificate(true) succeeded for a fixture with no chain")
	}
}

func TestLookalikeAuthorityDiffersOnlyByKey(t *testing.T) {
	set := generated(t)
	caA, lookalike := mustGet(t, set, "ca-a"), mustGet(t, set, "ca-b-same-dn")
	if !bytes.Equal(caA.Certificate.RawSubject, lookalike.Certificate.RawSubject) {
		t.Fatal("ca-b-same-dn does not carry ca-a's subject")
	}
	if err := verify(t, mustGet(t, set, "client-from-lookalike-ca"), caA); err == nil {
		t.Fatal("a certificate from the look-alike authority verified against ca-a")
	}
}

func TestRenewedCertificatesKeepIdentityAndChangeThumbprint(t *testing.T) {
	set := generated(t)
	valid, renewed := mustGet(t, set, "client-valid"), mustGet(t, set, "client-renewed")
	if valid.Certificate.Subject.String() != renewed.Certificate.Subject.String() ||
		valid.Certificate.URIs[0].String() != renewed.Certificate.URIs[0].String() {
		t.Fatal("client-renewed does not keep client-valid's subject and URI SAN")
	}
	if valid.Thumbprint == renewed.Thumbprint {
		t.Fatal("client-renewed shares client-valid's thumbprint")
	}
	selfSigned, selfRenewed := mustGet(t, set, "client-selfsigned"), mustGet(t, set, "client-selfsigned-renewed")
	if !bytes.Equal(selfSigned.KeyPEM, selfRenewed.KeyPEM) {
		t.Fatal("client-selfsigned-renewed does not reuse client-selfsigned's key")
	}
	if selfSigned.Thumbprint == selfRenewed.Thumbprint {
		t.Fatal("client-selfsigned-renewed shares client-selfsigned's thumbprint")
	}
}

func TestValidityWindowsAreRelativeToGeneration(t *testing.T) {
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	set, err := Generate(now)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !mustGet(t, set, "client-expired").Certificate.NotAfter.Before(now) {
		t.Fatal("client-expired is not expired")
	}
	if !mustGet(t, set, "client-not-yet-valid").Certificate.NotBefore.After(now) {
		t.Fatal("client-not-yet-valid is already valid")
	}
	soon := mustGet(t, set, "ca-expires-soon").Certificate.NotAfter
	if soon.Before(now) || soon.After(now.Add(30*24*time.Hour)) {
		t.Fatalf("ca-expires-soon expires at %s, want within thirty days of %s", soon, now)
	}
	if !mustGet(t, set, "client-valid").Certificate.NotAfter.After(now.Add(365 * 24 * time.Hour)) {
		t.Fatal("client-valid expires within a year")
	}
}

func TestKeyUsages(t *testing.T) {
	set := generated(t)
	cases := map[string][]x509.ExtKeyUsage{
		"client-valid":           {x509.ExtKeyUsageClientAuth},
		"client-serverauth-only": {x509.ExtKeyUsageServerAuth},
		"gw-identity-no-eku":     nil,
		"ca-a":                   nil,
	}
	for name, want := range cases {
		got := mustGet(t, set, name).Certificate.ExtKeyUsage
		if len(got) != len(want) || (len(want) == 1 && got[0] != want[0]) {
			t.Fatalf("%s: extended key usage %v, want %v", name, got, want)
		}
	}
	if mustGet(t, set, "client-signed-by-leaf").Certificate.CheckSignatureFrom(mustGet(t, set, "client-selfsigned").Certificate) == nil {
		t.Fatal("client-selfsigned, a CA:FALSE leaf, is accepted as an issuer")
	}
}

func TestIssueRejectsMalformedCatalogueEntries(t *testing.T) {
	g := &generator{now: time.Now(), issued: map[string]*issuer{}}
	if err := g.issue(spec{name: "orphan", parent: "missing"}); err == nil {
		t.Fatal("an unknown issuer was accepted")
	}
	if err := g.issue(spec{name: "reuser", reuseKey: "missing"}); err == nil {
		t.Fatal("an unknown key donor was accepted")
	}
	if err := g.issue(spec{name: "bad-uri", uriSANs: []string{"%zz"}}); err == nil {
		t.Fatal("a malformed URI SAN was accepted")
	}
	if err := g.issue(spec{name: "twice", isCA: true}); err != nil {
		t.Fatalf("first issue: %v", err)
	}
	if err := g.issue(spec{name: "twice", isCA: true}); err == nil {
		t.Fatal("a duplicate fixture name was accepted")
	}
}

func TestDefaultIsGeneratedOnceForConcurrentCallers(t *testing.T) {
	var wg sync.WaitGroup
	sets := make([]*Set, 8)
	for i := range sets {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			set, err := Default()
			if err != nil {
				t.Errorf("Default: %v", err)
			}
			sets[i] = set
		}(i)
	}
	wg.Wait()
	for _, set := range sets[1:] {
		if set != sets[0] {
			t.Fatal("Default returned different sets to concurrent callers")
		}
	}
}

func TestBackendServerCertificatesNameTheDialledHostAndChainToTheirAuthority(t *testing.T) {
	set := generated(t)
	for _, tc := range []struct{ server, authority, host string }{
		{"backend-server-a", "backend-ca", TLSBackendHost},
		{"backend-server-b", "backend-ca-b", TLSBackendHost},
		{"backend-server-wronghost", "backend-ca", WrongHostName},
	} {
		server, authority := mustGet(t, set, tc.server), mustGet(t, set, tc.authority)
		roots := x509.NewCertPool()
		roots.AddCert(authority.Certificate)
		if _, err := server.Certificate.Verify(x509.VerifyOptions{
			DNSName: tc.host, Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}); err != nil {
			t.Fatalf("%s does not verify for %q under %s: %v", tc.server, tc.host, tc.authority, err)
		}
		if tc.host == WrongHostName {
			if err := server.Certificate.VerifyHostname(TLSBackendHost); err == nil {
				t.Fatalf("%s must not be valid for %q", tc.server, TLSBackendHost)
			}
		}
	}
	other := mustGet(t, set, "backend-ca-b")
	roots := x509.NewCertPool()
	roots.AddCert(other.Certificate)
	if _, err := mustGet(t, set, "backend-server-a").Certificate.Verify(x509.VerifyOptions{
		DNSName: TLSBackendHost, Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err == nil {
		t.Fatal("backend-server-a verified under backend-ca-b")
	}
}

func TestKeyMismatchMatchesNoIdentity(t *testing.T) {
	set := generated(t)
	mismatch := mustGet(t, set, "key-mismatch")
	for _, name := range []string{"gw-identity-a", "gw-identity-b"} {
		if _, err := tls.X509KeyPair(mustGet(t, set, name).CertPEM, mismatch.KeyPEM); err == nil {
			t.Fatalf("the key-mismatch key pairs with %s", name)
		}
	}
}

func TestEncryptedKeyDecryptsToTheFixtureKey(t *testing.T) {
	f := mustGet(t, generated(t), "gw-identity-a")
	first, err := f.EncryptedKeyPEM("test")
	if err != nil {
		t.Fatalf("EncryptedKeyPEM: %v", err)
	}
	second, err := f.EncryptedKeyPEM("test")
	if err != nil {
		t.Fatalf("EncryptedKeyPEM: %v", err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("two encryptions share a salt and IV")
	}
	block, _ := pem.Decode(first)
	if block == nil || block.Type != "ENCRYPTED PRIVATE KEY" || len(block.Headers) != 0 {
		t.Fatalf("want one headerless ENCRYPTED PRIVATE KEY block, got %q", first)
	}
	var info encryptedPrivateKeyInfo
	if _, err := asn1.Unmarshal(block.Bytes, &info); err != nil || !info.EncryptionAlgorithm.Algorithm.Equal(oidPBES2) {
		t.Fatalf("not PBES2: %v", err)
	}
	var params pbes2Params
	if _, err := asn1.Unmarshal(info.EncryptionAlgorithm.Parameters.FullBytes, &params); err != nil {
		t.Fatalf("PBES2 parameters: %v", err)
	}
	var kdf pbkdf2Params
	if _, err := asn1.Unmarshal(params.KeyDerivationFunc.Parameters.FullBytes, &kdf); err != nil ||
		!params.KeyDerivationFunc.Algorithm.Equal(oidPBKDF2) || !kdf.PRF.Algorithm.Equal(oidHMACSHA256) {
		t.Fatalf("PBKDF2 parameters: %v", err)
	}
	var iv []byte
	if _, err := asn1.Unmarshal(params.EncryptionScheme.Parameters.FullBytes, &iv); err != nil ||
		!params.EncryptionScheme.Algorithm.Equal(oidAES256CBC) {
		t.Fatalf("AES-256-CBC parameters: %v", err)
	}
	key, err := pbkdf2.Key(sha256.New, "test", kdf.Salt, kdf.IterationCount, 32)
	if err != nil {
		t.Fatalf("pbkdf2: %v", err)
	}
	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	plain := make([]byte, len(info.EncryptedData))
	cipher.NewCBCDecrypter(aesBlock, iv).CryptBlocks(plain, info.EncryptedData)
	plain = plain[:len(plain)-int(plain[len(plain)-1])]
	want, _ := pem.Decode(f.KeyPEM)
	if !bytes.Equal(plain, want.Bytes) {
		t.Fatal("the decrypted key differs from the fixture key")
	}
	if _, err := x509.ParsePKCS8PrivateKey(plain); err != nil {
		t.Fatalf("decrypted key: %v", err)
	}
	if _, err := f.EncryptedKeyPEM(""); err == nil {
		t.Fatal("an empty passphrase was accepted")
	}
	if _, err := (&Fixture{Name: "empty"}).EncryptedKeyPEM("test"); err == nil {
		t.Fatal("a fixture without a key was encrypted")
	}
}

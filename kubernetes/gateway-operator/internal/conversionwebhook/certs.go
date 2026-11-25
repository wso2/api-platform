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

package conversionwebhook

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"sync/atomic"
	"time"
)

// Keys of the Secret that holds the webhook's CA and serving certificate.
const (
	SecretKeyCACert  = "ca.crt"
	SecretKeyCAKey   = "ca.key"
	SecretKeyTLSCert = "tls.crt"
	SecretKeyTLSKey  = "tls.key"
	// SecretKeyPreviousCA holds the CA(s) the webhook served under before the CA was
	// last replaced. The CRD keeps trusting them until PreviousCAUntilAnnotation, so
	// replicas still serving a certificate from the old CA are not rejected while they
	// pick up the new one.
	SecretKeyPreviousCA = "ca-previous.crt"
)

// PreviousCAUntilAnnotation on the webhook Secret is the RFC 3339 time until which the
// CRD trusts SecretKeyPreviousCA as well as the current CA.
const PreviousCAUntilAnnotation = "gateway.api-platform.wso2.com/previous-ca-until"

// The certificates use ECDSA P-256 signatures. The peer verifying them is the
// kube-apiserver, which cannot verify ML-DSA certificate signatures, so there is no
// post-quantum signature to offer it. The TLS key exchange is hybrid
// (X25519MLKEM768) by default; see tlsConfig.

// certBundle is the content of the webhook Secret: PEM data, plus the previous CA and
// how long it stays trusted after a CA replacement.
type certBundle struct {
	caCert, caKey, tlsCert, tlsKey []byte
	previousCA                     []byte
	previousUntil                  time.Time
}

func (b certBundle) data() map[string][]byte {
	data := map[string][]byte{
		SecretKeyCACert:  b.caCert,
		SecretKeyCAKey:   b.caKey,
		SecretKeyTLSCert: b.tlsCert,
		SecretKeyTLSKey:  b.tlsKey,
	}
	if len(b.previousCA) > 0 {
		data[SecretKeyPreviousCA] = b.previousCA
	}
	return data
}

// annotations returns the Secret annotations the bundle needs.
func (b certBundle) annotations() map[string]string {
	if len(b.previousCA) == 0 || b.previousUntil.IsZero() {
		return nil
	}
	return map[string]string{PreviousCAUntilAnnotation: b.previousUntil.UTC().Format(time.RFC3339)}
}

func bundleFromSecret(data map[string][]byte, annotations map[string]string) certBundle {
	b := certBundle{
		caCert:     data[SecretKeyCACert],
		caKey:      data[SecretKeyCAKey],
		tlsCert:    data[SecretKeyTLSCert],
		tlsKey:     data[SecretKeyTLSKey],
		previousCA: data[SecretKeyPreviousCA],
	}
	if until, err := time.Parse(time.RFC3339, annotations[PreviousCAUntilAnnotation]); err == nil {
		b.previousUntil = until
	}
	return b
}

// trustedCABundle is the caBundle the CRD must carry for b at now: the current CA,
// plus the previous CA while its overlap lasts.
func trustedCABundle(b certBundle, now time.Time) []byte {
	if len(b.previousCA) == 0 || !now.Before(b.previousUntil) {
		return b.caCert
	}
	out := append([]byte{}, b.caCert...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return append(out, b.previousCA...)
}

// previousCAs returns the CA certificates in pemBytes that are still valid at now,
// other than current, as PEM. It is what a replaced CA hands over to the overlap.
func previousCAs(pemBytes, current []byte, now time.Time) []byte {
	var currentDER []byte
	if block, _ := pem.Decode(current); block != nil {
		currentDER = block.Bytes
	}
	var out []byte
	for rest := pemBytes; len(rest) > 0; {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" || bytes.Equal(block.Bytes, currentDER) {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil || !cert.IsCA || now.After(cert.NotAfter) {
			continue
		}
		out = append(out, pem.EncodeToMemory(block)...)
	}
	return out
}

// serviceDNSNames returns the names the kube-apiserver may use to reach the webhook
// Service. It calls service.namespace.svc; the others cover in-cluster debugging.
func serviceDNSNames(service, namespace string) []string {
	return []string{
		service,
		service + "." + namespace,
		service + "." + namespace + ".svc",
		service + "." + namespace + ".svc.cluster.local",
	}
}

func randomSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

func encodeKey(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), nil
}

// newCA returns a self-signed CA certificate and key in PEM form.
func newCA(now time.Time, validity time.Duration) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate CA key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, fmt.Errorf("generate CA serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "gateway-operator-conversion-webhook-ca"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create CA certificate: %w", err)
	}
	keyPEM, err = encodeKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("encode CA key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), keyPEM, nil
}

// newServingCert returns a serving certificate for dnsNames, signed by the CA.
func newServingCert(caCertPEM, caKeyPEM []byte, dnsNames []string, now time.Time, validity time.Duration) (certPEM, keyPEM []byte, err error) {
	caCert, caKey, err := parseCA(caCertPEM, caKeyPEM)
	if err != nil {
		return nil, nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate serving key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, fmt.Errorf("generate serving serial: %w", err)
	}
	notAfter := now.Add(validity)
	if notAfter.After(caCert.NotAfter) {
		notAfter = caCert.NotAfter
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: dnsNames[len(dnsNames)-2]},
		DNSNames:     dnsNames,
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create serving certificate: %w", err)
	}
	keyPEM, err = encodeKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("encode serving key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), keyPEM, nil
}

func parseCertPEM(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("no PEM certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parseCA(certPEM, keyPEM []byte) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	cert, err := parseCertPEM(certPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA certificate: %w", err)
	}
	if !cert.IsCA {
		return nil, nil, errors.New("CA certificate is not a CA")
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, nil, errors.New("parse CA key: no PEM block")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA key: %w", err)
	}
	// A key from another CA cannot sign for this certificate; reporting it here makes
	// planRenewal replace both instead of failing every serving renewal.
	if !key.PublicKey.Equal(cert.PublicKey) {
		return nil, nil, errors.New("CA key does not match the CA certificate")
	}
	return cert, key, nil
}

// renewalPlan says what a certBundle needs before it can be served.
type renewalPlan int

const (
	renewNothing renewalPlan = iota
	// renewServing re-issues the serving certificate under the existing CA. The
	// caBundle on the CRD stays valid.
	renewServing
	// renewAll replaces the CA as well. The CRD's caBundle must follow.
	renewAll
)

// planRenewal checks a bundle against the names it must serve and the time left
// before each certificate expires.
func planRenewal(b certBundle, dnsNames []string, now time.Time, renewBefore time.Duration) renewalPlan {
	caCert, _, err := parseCA(b.caCert, b.caKey)
	if err != nil || now.Add(renewBefore).After(caCert.NotAfter) {
		return renewAll
	}
	leaf, err := parseCertPEM(b.tlsCert)
	if err != nil {
		return renewServing
	}
	if _, err := tls.X509KeyPair(b.tlsCert, b.tlsKey); err != nil {
		return renewServing
	}
	if now.Add(renewBefore).After(leaf.NotAfter) || now.Before(leaf.NotBefore) {
		return renewServing
	}
	got := slices.Clone(leaf.DNSNames)
	want := slices.Clone(dnsNames)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return renewServing
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:       pool,
		CurrentTime: now,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		return renewServing
	}
	return renewNothing
}

// renew returns b with whatever planRenewal asked for replaced.
func renew(b certBundle, plan renewalPlan, dnsNames []string, now time.Time, v validity) (certBundle, error) {
	out := b
	if plan == renewAll {
		caCert, caKey, err := newCA(now, v.ca)
		if err != nil {
			return certBundle{}, err
		}
		out.caCert, out.caKey = caCert, caKey
	}
	if plan == renewAll || plan == renewServing {
		cert, key, err := newServingCert(out.caCert, out.caKey, dnsNames, now, v.serving)
		if err != nil {
			return certBundle{}, err
		}
		out.tlsCert, out.tlsKey = cert, key
	}
	return out, nil
}

// validity holds the certificate lifetimes, the renewal margin, and how long the CRD
// keeps trusting a replaced CA.
type validity struct {
	ca, serving, renewBefore, caOverlap time.Duration
}

// certStore holds the serving certificate in memory for the TLS listener.
type certStore struct {
	cert    atomic.Pointer[tls.Certificate]
	certPEM atomic.Pointer[[]byte]
}

// set loads the serving keypair from a bundle. Callers serialize it through
// servingState.sync, which reads the Secret right before serving.
func (s *certStore) set(b certBundle) error {
	pair, err := tls.X509KeyPair(b.tlsCert, b.tlsKey)
	if err != nil {
		return fmt.Errorf("load serving keypair: %w", err)
	}
	pemCopy := append([]byte{}, b.tlsCert...)
	s.cert.Store(&pair)
	s.certPEM.Store(&pemCopy)
	return nil
}

// serving reports whether the store already serves the bundle's certificate.
func (s *certStore) serving(b certBundle) bool {
	p := s.certPEM.Load()
	return p != nil && bytes.Equal(*p, b.tlsCert)
}

func (s *certStore) ready() bool {
	return s.cert.Load() != nil
}

// GetCertificate implements tls.Config.GetCertificate.
func (s *certStore) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	if c := s.cert.Load(); c != nil {
		return c, nil
	}
	return nil, errors.New("conversion webhook serving certificate is not loaded yet")
}

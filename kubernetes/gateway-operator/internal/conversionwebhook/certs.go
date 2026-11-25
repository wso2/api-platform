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
)

// The certificates use ECDSA P-256 signatures. The peer verifying them is the
// kube-apiserver, which cannot verify ML-DSA certificate signatures, so there is no
// post-quantum signature to offer it. The TLS key exchange is hybrid
// (X25519MLKEM768) by default; see tlsConfig.

// certBundle is the PEM content of the webhook Secret.
type certBundle struct {
	caCert, caKey, tlsCert, tlsKey []byte
}

func (b certBundle) data() map[string][]byte {
	return map[string][]byte{
		SecretKeyCACert:  b.caCert,
		SecretKeyCAKey:   b.caKey,
		SecretKeyTLSCert: b.tlsCert,
		SecretKeyTLSKey:  b.tlsKey,
	}
}

func bundleFromData(data map[string][]byte) certBundle {
	return certBundle{
		caCert:  data[SecretKeyCACert],
		caKey:   data[SecretKeyCAKey],
		tlsCert: data[SecretKeyTLSCert],
		tlsKey:  data[SecretKeyTLSKey],
	}
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

// validity holds the certificate lifetimes and the renewal margin.
type validity struct {
	ca, serving, renewBefore time.Duration
}

// certStore holds the serving certificate in memory for the TLS listener.
type certStore struct {
	cert atomic.Pointer[tls.Certificate]
}

// set loads the serving keypair from a bundle.
func (s *certStore) set(b certBundle) error {
	pair, err := tls.X509KeyPair(b.tlsCert, b.tlsKey)
	if err != nil {
		return fmt.Errorf("load serving keypair: %w", err)
	}
	s.cert.Store(&pair)
	return nil
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

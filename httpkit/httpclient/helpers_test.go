package httpclient

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

// selfSignedCert is a self-signed certificate usable both as a leaf
// (presented on a connection) and, by adding certDER to a pool, as its own
// trust root — the same pattern net/http/httptest uses for its own test
// certificates.
type selfSignedCert struct {
	tlsCert tls.Certificate
	leaf    *x509.Certificate
	pool    *x509.CertPool
}

// newSelfSignedCert generates an ECDSA P-256 self-signed certificate with
// the given CommonName, DNS names, and IP addresses, valid for both server
// and client authentication so the same helper covers origin certs, proxy
// certs, and client certs across the test suite. commonName is distinct per
// call so tests that need to tell two certificates apart (e.g. "did the
// proxy see the proxy cert or the origin cert?") can assert on it.
func newSelfSignedCert(t *testing.T, commonName string, dnsNames []string, ips []net.IP) selfSignedCert {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("rand.Int: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              dnsNames,
		IPAddresses:           ips,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}

	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(leaf)

	return selfSignedCert{
		tlsCert: tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv, Leaf: leaf},
		leaf:    leaf,
		pool:    pool,
	}
}

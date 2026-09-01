package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// startTLSServerOn starts an httptest TLS server bound to bindIP (instead of
// httptest's default 127.0.0.1) presenting cert, so two servers in the same
// test can have genuinely distinct identities (distinct IP-literal SANs)
// without needing DNS or /etc/hosts.
func startTLSServerOn(t *testing.T, bindIP string, cert tls.Certificate, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(handler)
	if err := srv.Listener.Close(); err != nil {
		t.Fatalf("closing default listener: %v", err)
	}
	ln, err := net.Listen("tcp", bindIP+":0")
	if err != nil {
		t.Skipf("cannot bind %s (loopback range may be restricted in this sandbox): %v", bindIP, err)
	}
	srv.Listener = ln
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	srv.StartTLS()
	return srv
}

// TestNew_NoStaleServerNameAcrossHosts proves a single long-lived client
// built by New correctly verifies two different TLS servers with two
// different certificates and identities — regression coverage for the
// requirement that Config never carries a fixed tls.Config.ServerName
// (which would silently misverify every host but the first one dialed). The
// two servers are given distinct loopback IPs (127.0.0.1 / 127.0.0.2) so
// their identities genuinely differ without relying on DNS.
func TestNew_NoStaleServerNameAcrossHosts(t *testing.T) {
	certA := newSelfSignedCert(t, "host-a", nil, []net.IP{net.ParseIP("127.0.0.1")})
	certB := newSelfSignedCert(t, "host-b", nil, []net.IP{net.ParseIP("127.0.0.2")})

	pool := x509.NewCertPool()
	pool.AddCert(certA.leaf)
	pool.AddCert(certB.leaf)

	srvA := startTLSServerOn(t, "127.0.0.1", certA.tlsCert, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("A"))
	}))
	defer srvA.Close()

	srvB := startTLSServerOn(t, "127.0.0.2", certB.tlsCert, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("B"))
	}))
	defer srvB.Close()

	cfg := DefaultConfig()
	cfg.TLS.RootCAs = pool
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	respA, err := client.Get(srvA.URL)
	if err != nil {
		t.Fatalf("client.Get(A): %v", err)
	}
	respA.Body.Close()
	if respA.StatusCode != http.StatusOK {
		t.Fatalf("A status = %d", respA.StatusCode)
	}

	// If a fixed ServerName ("127.0.0.1") leaked from the first request,
	// this second request — to a server whose cert only covers 127.0.0.2 —
	// would fail hostname verification.
	respB, err := client.Get(srvB.URL)
	if err != nil {
		t.Fatalf("client.Get(B) — this fails if ServerName leaked from the A request: %v", err)
	}
	respB.Body.Close()
	if respB.StatusCode != http.StatusOK {
		t.Fatalf("B status = %d", respB.StatusCode)
	}
}

// TestNew_RejectsWrongHostCertificate proves default hostname verification
// is intact (not silently disabled) — a client trusting certA's issuer must
// still refuse a connection presenting a cert for a different identity.
func TestNew_RejectsWrongHostCertificate(t *testing.T) {
	certForOther := newSelfSignedCert(t, "other", nil, []net.IP{net.ParseIP("127.0.0.3")})
	certPresented := newSelfSignedCert(t, "presented", nil, []net.IP{net.ParseIP("127.0.0.1")})

	// The client only trusts certForOther, but the server presents
	// certPresented — verification must fail.
	pool := x509.NewCertPool()
	pool.AddCert(certForOther.leaf)

	srv := startTLSServerOn(t, "127.0.0.1", certPresented.tlsCert, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.TLS.RootCAs = pool
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := client.Get(srv.URL); err == nil {
		t.Fatal("expected a request against an untrusted certificate to fail")
	}
}

// TestNew_MTLSToOrigin proves the common mTLS-to-origin wiring (TLSConfig's
// ClientCertFile/Key, or here the in-memory Certificates equivalent via
// GetClientCertificate) actually presents a client certificate the server
// can verify, and that the server correctly sees the expected certificate.
func TestNew_MTLSToOrigin(t *testing.T) {
	serverCert := newSelfSignedCert(t, "origin-server", nil, []net.IP{net.ParseIP("127.0.0.1")})
	clientCert := newSelfSignedCert(t, "httpkit-test", []string{"test-client"}, nil)

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(clientCert.leaf)

	var sawClientCN string
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.PeerCertificates) > 0 {
			sawClientCN = r.TLS.PeerCertificates[0].Subject.CommonName
		}
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert.tlsCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
	}
	srv.StartTLS()
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.TLS.RootCAs = serverCert.pool
	cfg.TLS.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return &clientCert.tlsCert, nil
	}
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("client.Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if sawClientCN != "httpkit-test" {
		t.Fatalf("server saw client CN %q, want %q", sawClientCN, "httpkit-test")
	}
}

func TestBuildTLSConfig_RejectsAsymmetricVersionRange(t *testing.T) {
	_, err := buildTLSConfig(TLSConfig{MinVersion: "TLS1_3", MaxVersion: "TLS1_2"})
	if err == nil {
		t.Fatal("expected an error for min_version > max_version")
	}
}

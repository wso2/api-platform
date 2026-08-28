package httpclient

import (
	"bufio"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2/api-platform/httpkit/tlsconfig"
)

// startConnectProxy starts a minimal CONNECT-speaking forward proxy: it
// reads a CONNECT request, dials the requested target, replies 200, and
// splices bytes bidirectionally. It performs no TLS itself and no
// validation — it exists only to exercise this package's proxy+mTLS wiring
// against a real CONNECT tunnel.
func startConnectProxy(t *testing.T) (addr string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleConnectConn(conn)
		}
	}()
	return ln.Addr().String()
}

func handleConnectConn(conn net.Conn) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil || req.Method != http.MethodConnect {
		return
	}

	target, err := net.Dial("tcp", req.Host)
	if err != nil {
		conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer target.Close()

	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	done := make(chan struct{}, 2)
	go func() { io.Copy(target, br); done <- struct{}{} }()
	go func() { io.Copy(conn, target); done <- struct{}{} }()
	<-done
}

// TestNew_MTLSThroughCONNECTProxy proves the "common case" wiring from the
// design (Tier 1): a plain-HTTP forward proxy tunneling an mTLS connection
// to the origin, using only Transport.Proxy + Transport.TLSClientConfig —
// no custom DialTLSContext needed. This is the shape most real deployments
// (mTLS to a backend through a corporate/K8s egress proxy) will actually
// use.
func TestNew_MTLSThroughCONNECTProxy(t *testing.T) {
	serverCert := newSelfSignedCert(t, "origin-server", nil, []net.IP{net.ParseIP("127.0.0.1")})
	clientCert := newSelfSignedCert(t, "httpkit-test", []string{"tunnel-client"}, nil)

	clientCAs := clientCert.pool

	var sawClientCN string
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.PeerCertificates) > 0 {
			sawClientCN = r.TLS.PeerCertificates[0].Subject.CommonName
		}
		w.WriteHeader(http.StatusOK)
	}))
	origin.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert.tlsCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
	}
	origin.StartTLS()
	defer origin.Close()

	proxyAddr := startConnectProxy(t)

	cfg := DefaultConfig()
	cfg.TLS.RootCAs = serverCert.pool
	cfg.TLS.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return &clientCert.tlsCert, nil
	}
	cfg.Proxy.Mode = "url"
	cfg.Proxy.URL = "http://" + proxyAddr

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := client.Get(origin.URL)
	if err != nil {
		t.Fatalf("client.Get through CONNECT proxy: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if sawClientCN != "httpkit-test" {
		t.Fatalf("origin saw client CN %q, want %q — mTLS did not reach the origin over the tunnel", sawClientCN, "httpkit-test")
	}
}

// TestNew_PQCHybridCurve_NegotiatesWhenSupported proves a client configured
// with the hybrid group first still negotiates it when the peer supports
// it.
func TestNew_PQCHybridCurve_NegotiatesWhenSupported(t *testing.T) {
	cert := newSelfSignedCert(t, "pqc-server", nil, []net.IP{net.ParseIP("127.0.0.1")})

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{
		Certificates:     []tls.Certificate{cert.tlsCert},
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768},
	}
	srv.StartTLS()
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.TLS.RootCAs = cert.pool
	cfg.TLS.CurvePreferences = "X25519MLKEM768,X25519,P-256"
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("client.Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.TLS == nil {
		t.Fatal("resp.TLS is nil")
	}
	if got := tlsconfig.NegotiatedCurveName(*resp.TLS); got != "X25519MLKEM768" {
		t.Fatalf("negotiated curve = %q, want %q", got, "X25519MLKEM768")
	}
}

// TestNew_PQCHybridCurve_FallsBackToClassical proves a client configured
// with the hybrid group first still completes the handshake — falling back
// to a classical curve, never hard-failing — against a peer that only
// supports classical groups (a stand-in for a legacy backend that hasn't
// adopted PQC yet).
func TestNew_PQCHybridCurve_FallsBackToClassical(t *testing.T) {
	cert := newSelfSignedCert(t, "pqc-server", nil, []net.IP{net.ParseIP("127.0.0.1")})

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{
		Certificates:     []tls.Certificate{cert.tlsCert},
		CurvePreferences: []tls.CurveID{tls.X25519}, // no hybrid support
	}
	srv.StartTLS()
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.TLS.RootCAs = cert.pool
	cfg.TLS.CurvePreferences = "X25519MLKEM768,X25519,P-256" // hybrid first, classical retained
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("client.Get against a classical-only peer must still succeed via fallback, got: %v", err)
	}
	defer resp.Body.Close()

	if resp.TLS == nil {
		t.Fatal("resp.TLS is nil")
	}
	if got := tlsconfig.NegotiatedCurveName(*resp.TLS); got != "X25519" {
		t.Fatalf("negotiated curve = %q, want fallback %q", got, "X25519")
	}
}

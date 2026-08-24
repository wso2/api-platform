package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// startConnectTLSProxy starts a CONNECT-speaking forward proxy whose own
// connection requires a client certificate (mTLS to the proxy), verified
// against clientCAs. It records the CommonName of the last client
// certificate it saw so the test can assert the PROXY received the
// proxy-facing certificate rather than the origin-facing one. It reuses
// handleConnectConn (defined in transport_connect_test.go) to relay the
// tunnel once the proxy-facing TLS handshake completes.
func startConnectTLSProxy(t *testing.T, serverCert tls.Certificate, clientCAs *x509.CertPool) (addr string, sawClientCN func() string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	var mu sync.Mutex
	var cn string

	go func() {
		for {
			raw, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer raw.Close()
				tlsConn := tls.Server(raw, &tls.Config{
					Certificates: []tls.Certificate{serverCert},
					ClientAuth:   tls.RequireAndVerifyClientCert,
					ClientCAs:    clientCAs,
				})
				if err := tlsConn.Handshake(); err != nil {
					return
				}
				state := tlsConn.ConnectionState()
				if len(state.PeerCertificates) > 0 {
					mu.Lock()
					cn = state.PeerCertificates[0].Subject.CommonName
					mu.Unlock()
				}
				handleConnectConn(tlsConn)
			}()
		}
	}()

	return ln.Addr().String(), func() string {
		mu.Lock()
		defer mu.Unlock()
		return cn
	}
}

// TestNew_DistinctProxyAndOriginCerts proves Tier 2 (Proxy.ProxyTLS set)
// genuinely decouples the proxy-facing TLS handshake from the origin-facing
// one: the proxy must see the proxy client certificate, and the origin must
// see a DIFFERENT, origin client certificate, over the same tunneled
// connection.
func TestNew_DistinctProxyAndOriginCerts(t *testing.T) {
	proxyServerCert := newSelfSignedCert(t, "proxy-server", nil, []net.IP{net.ParseIP("127.0.0.1")})
	proxyClientCert := newSelfSignedCert(t, "proxy-client-cn", []string{"proxy-client"}, nil)
	originServerCert := newSelfSignedCert(t, "origin-server", nil, []net.IP{net.ParseIP("127.0.0.1")})
	originClientCert := newSelfSignedCert(t, "origin-client-cn", []string{"origin-client"}, nil)

	var sawOriginCN string
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.PeerCertificates) > 0 {
			sawOriginCN = r.TLS.PeerCertificates[0].Subject.CommonName
		}
		w.WriteHeader(http.StatusOK)
	}))
	origin.TLS = &tls.Config{
		Certificates: []tls.Certificate{originServerCert.tlsCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    originClientCert.pool,
	}
	origin.StartTLS()
	defer origin.Close()

	proxyAddr, sawProxyCN := startConnectTLSProxy(t, proxyServerCert.tlsCert, proxyClientCert.pool)

	cfg := DefaultConfig()
	cfg.TLS.RootCAs = originServerCert.pool
	cfg.TLS.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return &originClientCert.tlsCert, nil
	}
	cfg.Proxy.Mode = "url"
	cfg.Proxy.URL = "https://" + proxyAddr
	cfg.Proxy.ProxyTLS = &ProxyTLSConfig{
		RootCAs: proxyServerCert.pool,
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &proxyClientCert.tlsCert, nil
		},
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := client.Get(origin.URL)
	if err != nil {
		t.Fatalf("client.Get through mTLS proxy tunnel: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	if got := sawProxyCN(); got != "proxy-client-cn" {
		t.Fatalf("proxy saw client CN %q, want the proxy cert's CN %q", got, "proxy-client-cn")
	}
	if sawOriginCN != "origin-client-cn" {
		t.Fatalf("origin saw client CN %q, want the origin cert's CN %q", sawOriginCN, "origin-client-cn")
	}
}

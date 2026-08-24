package httpclient

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2/go-httpkit/netguard"
)

// TestNew_ManualCONNECT_Succeeds proves the ProxyEgressManualCONNECT path
// actually completes a real request through a plain CONNECT proxy once
// local SSRF validation passes.
func TestNew_ManualCONNECT_Succeeds(t *testing.T) {
	cert := newSelfSignedCert(t, "manual-connect-origin", nil, []net.IP{net.ParseIP("127.0.0.1")})

	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	origin.TLS = &tls.Config{Certificates: []tls.Certificate{cert.tlsCert}}
	origin.StartTLS()
	defer origin.Close()

	proxyAddr := startConnectProxy(t)

	cfg := DefaultConfig()
	cfg.TLS.RootCAs = cert.pool
	cfg.Proxy.Mode = "url"
	cfg.Proxy.URL = "http://" + proxyAddr
	cfg.Proxy.Egress = ProxyEgressManualCONNECT
	cfg.SSRF.Enabled = true
	cfg.SSRF.Policy = netguard.PermitPrivateBlockMetadata() // permits loopback

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := client.Get(origin.URL)
	if err != nil {
		t.Fatalf("client.Get via ProxyEgressManualCONNECT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

// TestNew_ManualCONNECT_RejectsDisallowedOrigin proves local origin
// validation actually runs and blocks the request BEFORE any CONNECT is
// attempted, when the origin fails the configured SSRF policy.
func TestNew_ManualCONNECT_RejectsDisallowedOrigin(t *testing.T) {
	cert := newSelfSignedCert(t, "manual-connect-origin2", nil, []net.IP{net.ParseIP("127.0.0.1")})

	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	origin.TLS = &tls.Config{Certificates: []tls.Certificate{cert.tlsCert}}
	origin.StartTLS()
	defer origin.Close()

	proxyAddr := startConnectProxy(t)

	cfg := DefaultConfig()
	cfg.TLS.RootCAs = cert.pool
	cfg.Proxy.Mode = "url"
	cfg.Proxy.URL = "http://" + proxyAddr
	cfg.Proxy.Egress = ProxyEgressManualCONNECT
	cfg.SSRF.Enabled = true
	cfg.SSRF.Policy = netguard.PublicOnly() // blocks loopback -- origin must be rejected

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := client.Get(origin.URL); err == nil {
		t.Fatal("expected a request to a loopback origin to be rejected under PublicOnly policy")
	}
}

func TestNewConnectRoundTripper_ValidatesConfig(t *testing.T) {
	baseTLS, err := buildTLSConfig(TLSConfig{})
	if err != nil {
		t.Fatalf("buildTLSConfig: %v", err)
	}

	t.Run("requires Mode url", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Proxy.Mode = "environment"
		cfg.SSRF.Enabled = true
		cfg.SSRF.Policy = netguard.PermitPrivateBlockMetadata()
		if _, err := newConnectRoundTripper(cfg, nil, baseTLS); err == nil {
			t.Fatal("expected an error when Proxy.Mode is not \"url\"")
		}
	})

	t.Run("requires SSRF enabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Proxy.Mode = "url"
		cfg.Proxy.URL = "http://proxy.example:3128"
		if _, err := newConnectRoundTripper(cfg, nil, baseTLS); err == nil {
			t.Fatal("expected an error when SSRF.Enabled is false")
		}
	})

	t.Run("requires Proxy.URL", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Proxy.Mode = "url"
		cfg.SSRF.Enabled = true
		cfg.SSRF.Policy = netguard.PermitPrivateBlockMetadata()
		if _, err := newConnectRoundTripper(cfg, nil, baseTLS); err == nil {
			t.Fatal("expected an error when Proxy.URL is empty")
		}
	})
}

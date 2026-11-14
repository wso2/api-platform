package httpclient

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/httpkit/netguard"
)

func guardedClient(t *testing.T, gc netguard.GuardConfig, mutate func(*Config)) *http.Client {
	t.Helper()
	policy, err := gc.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cfg := DefaultConfig()
	cfg.SSRF.Enabled = true
	cfg.SSRF.Policy = policy
	if mutate != nil {
		mutate(&cfg)
	}
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

func okServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestNew_HostRuleAppliesToTheRequestURL proves the hostname rule is enforced
// on the destination URL, with a positive control for a listed host.
func TestNew_HostRuleAppliesToTheRequestURL(t *testing.T) {
	srv := okServer(t)
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))

	client := guardedClient(t, netguard.GuardConfig{
		Enabled: true, Mode: netguard.ModeAllow, Hostnames: []string{"localhost"},
	}, nil)

	resp, err := client.Get("http://localhost:" + port + "/")
	if err != nil {
		t.Fatalf("listed host: %v", err)
	}
	resp.Body.Close()

	_, err = client.Get(srv.URL) // http://127.0.0.1:port — not listed
	if !errors.Is(err, netguard.ErrDestinationNotAllowed) {
		t.Fatalf("unlisted host = %v, want ErrDestinationNotAllowed", err)
	}
}

func TestNew_ExplicitSchemesApplyToTheFirstRequest(t *testing.T) {
	srv := okServer(t)
	client := guardedClient(t, netguard.GuardConfig{Enabled: true, AllowedSchemes: []string{"https"}}, nil)
	if _, err := client.Get(srv.URL); !errors.Is(err, netguard.ErrDestinationNotAllowed) {
		t.Fatalf("http request with only https allowed = %v, want ErrDestinationNotAllowed", err)
	}

	permit := guardedClient(t, netguard.GuardConfig{Enabled: true}, nil)
	resp, err := permit.Get(srv.URL)
	if err != nil {
		t.Fatalf("positive control with default schemes: %v", err)
	}
	resp.Body.Close()
}

// TestNew_PresetPoliciesKeepFirstRequestSchemeBehaviour guards existing
// callers: a preset policy lists no schemes, and must not start refusing
// plain-http requests.
func TestNew_PresetPoliciesKeepFirstRequestSchemeBehaviour(t *testing.T) {
	srv := okServer(t)
	cfg := DefaultConfig()
	cfg.SSRF.Enabled = true
	cfg.SSRF.Policy = netguard.PermitPrivateBlockMetadata()
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("preset policy plain-http request: %v", err)
	}
	resp.Body.Close()
}

func TestNew_DenyPrivateNetworksRefusesLoopbackOrigin(t *testing.T) {
	srv := okServer(t)
	client := guardedClient(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true}, nil)
	if _, err := client.Get(srv.URL); !errors.Is(err, netguard.ErrDestinationNotAllowed) {
		t.Fatalf("loopback origin = %v, want ErrDestinationNotAllowed", err)
	}
}

// tlsOrigin starts an https origin on 127.0.0.1 and returns it with the
// pool that trusts it.
func tlsOrigin(t *testing.T, name string) (*httptest.Server, *selfSignedCert) {
	t.Helper()
	cert := newSelfSignedCert(t, name, nil, []net.IP{net.ParseIP("127.0.0.1")})
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	origin.TLS = &tls.Config{Certificates: []tls.Certificate{cert.tlsCert}}
	origin.StartTLS()
	t.Cleanup(origin.Close)
	return origin, &cert
}

// TestNew_ProxyHopIsNotSubjectToDestinationRules proves a configured proxy on
// a private address stays reachable when deny_private_networks is on, and
// that the host rule still applies to the proxied destination. The proxy is
// operator configuration; the destination is what the rules govern.
func TestNew_ProxyHopIsNotSubjectToDestinationRules(t *testing.T) {
	origin, cert := tlsOrigin(t, "proxy-hop-origin")
	proxyAddr := startConnectProxy(t)

	for _, egress := range []ProxyEgressPolicy{ProxyEgressDelegated, ProxyEgressManualCONNECT} {
		withProxy := func(c *Config) {
			c.TLS.RootCAs = cert.pool
			c.Proxy.Mode = "url"
			c.Proxy.URL = "http://" + proxyAddr
			c.Proxy.Egress = egress
		}

		listed := guardedClient(t, netguard.GuardConfig{
			Enabled: true, Mode: netguard.ModeAllow, Hostnames: []string{`127\.0\.0\.1`},
		}, withProxy)
		resp, err := listed.Get(origin.URL)
		if err != nil {
			t.Fatalf("egress %d: listed destination through a loopback proxy: %v", egress, err)
		}
		resp.Body.Close()

		unlisted := guardedClient(t, netguard.GuardConfig{
			Enabled: true, Mode: netguard.ModeAllow, Hostnames: []string{`api\.example\.com`},
		}, withProxy)
		if _, err := unlisted.Get(origin.URL); !errors.Is(err, netguard.ErrDestinationNotAllowed) {
			t.Fatalf("egress %d: unlisted destination through a proxy = %v, want ErrDestinationNotAllowed", egress, err)
		}
	}

	// Delegated egress cannot see the destination's address, so with
	// deny_private_networks on, only the proxy hop decides — and it is exempt.
	delegated := guardedClient(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true}, func(c *Config) {
		c.TLS.RootCAs = cert.pool
		c.Proxy.Mode = "url"
		c.Proxy.URL = "http://" + proxyAddr
		c.Proxy.Egress = ProxyEgressDelegated
	})
	resp, err := delegated.Get(origin.URL)
	if err != nil {
		t.Fatalf("private proxy with deny_private_networks: %v", err)
	}
	resp.Body.Close()

	// Manual CONNECT validates the destination's address itself.
	manual := guardedClient(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true}, func(c *Config) {
		c.TLS.RootCAs = cert.pool
		c.Proxy.Mode = "url"
		c.Proxy.URL = "http://" + proxyAddr
		c.Proxy.Egress = ProxyEgressManualCONNECT
	})
	if _, err := manual.Get(origin.URL); !errors.Is(err, netguard.ErrDestinationNotAllowed) {
		t.Fatalf("manual CONNECT to a private destination = %v, want ErrDestinationNotAllowed", err)
	}
}

func TestNew_GuardEnabledAcceptsHostOnlyPolicy(t *testing.T) {
	rule, err := netguard.CompileHostRule(netguard.ModeDeny, []string{"x"})
	if err != nil {
		t.Fatalf("CompileHostRule: %v", err)
	}
	cfg := DefaultConfig()
	cfg.SSRF.Enabled = true
	cfg.SSRF.Policy = netguard.Policy{Hosts: rule}
	if _, err := New(cfg); err != nil {
		t.Fatalf("a policy with only a host rule is not zero: %v", err)
	}
}

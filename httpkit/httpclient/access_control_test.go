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

package httpclient

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	cfg.DestinationGuard.Enabled = true
	cfg.DestinationGuard.Policy = policy
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
	cfg.DestinationGuard.Enabled = true
	cfg.DestinationGuard.Policy = netguard.PermitPrivateBlockMetadata()
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
	cfg.DestinationGuard.Enabled = true
	cfg.DestinationGuard.Policy = netguard.Policy{Hosts: rule}
	if _, err := New(cfg); err != nil {
		t.Fatalf("a policy with only a host rule is not zero: %v", err)
	}
}

// TestNew_DirectRequestToTheProxyAddressGetsTheOriginPolicy covers a request
// that bypasses the proxy (NoProxy) but targets the proxy's own address, after
// another request has used that proxy. It must be dialed under the full policy,
// not the proxy hop's: being a proxy address elsewhere is not a reason to relax
// the destination rules for a direct request.
func TestNew_DirectRequestToTheProxyAddressGetsTheOriginPolicy(t *testing.T) {
	cert := newSelfSignedCert(t, "routing-origin", []string{"localhost"}, []net.IP{net.ParseIP("127.0.0.1")})
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	origin.TLS = &tls.Config{Certificates: []tls.Certificate{cert.tlsCert}}
	origin.StartTLS()
	t.Cleanup(origin.Close)
	_, originPort, _ := net.SplitHostPort(strings.TrimPrefix(origin.URL, "https://"))

	proxyAddr := startConnectProxy(t)
	client := guardedClient(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true}, func(c *Config) {
		c.TLS.RootCAs = cert.pool
		c.Proxy.Mode = "url"
		c.Proxy.URL = "http://" + proxyAddr
		c.Proxy.NoProxy = []string{"127.0.0.1"} // only "localhost" goes through the proxy
		c.Proxy.Egress = ProxyEgressDelegated
	})

	// Through the proxy: the proxy hop is exempt, so this is reachable.
	resp, err := client.Get("https://localhost:" + originPort + "/")
	if err != nil {
		t.Fatalf("request through the proxy: %v", err)
	}
	resp.Body.Close()

	// Direct (NoProxy) to the proxy's own address: the full policy applies.
	if _, err := client.Get("http://" + proxyAddr + "/"); !errors.Is(err, netguard.ErrDestinationNotAllowed) {
		t.Fatalf("direct request to the proxy address = %v, want ErrDestinationNotAllowed", err)
	}
}

// TestNew_RedirectSchemesNarrowOnlyRedirectHops: RedirectSchemes restricts the
// scheme of a redirect hop without restricting the first request.
func TestNew_RedirectSchemesNarrowOnlyRedirectHops(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/next", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := guardedClient(t, netguard.GuardConfig{Enabled: true}, func(c *Config) {
		c.DestinationGuard.RedirectSchemes = []string{"https"}
	})
	resp, err := client.Get(srv.URL + "/next")
	if err != nil {
		t.Fatalf("an http first request must still be allowed: %v", err)
	}
	resp.Body.Close()
	if _, err := client.Get(srv.URL + "/start"); !errors.Is(err, netguard.ErrDestinationNotAllowed) {
		t.Fatalf("an http redirect hop with https-only RedirectSchemes = %v, want ErrDestinationNotAllowed", err)
	}
}

// TestProxyRouting_FollowUpRequestDoesNotInheritAProxyRoute: a request built from an
// earlier, proxied request's context (as Response.Request is reused) and routed
// directly must be dialed under the origin policy, even at the proxy's own address.
func TestProxyRouting_FollowUpRequestDoesNotInheritAProxyRoute(t *testing.T) {
	proxyURL, _ := url.Parse("http://proxy.internal:3128")
	routing := &proxyRouting{proxyFunc: func(req *http.Request) (*url.URL, error) {
		if req.URL.Hostname() == "proxy.internal" { // bypassed, as by NoProxy
			return nil, nil
		}
		return proxyURL, nil
	}}

	var dialed []string
	dial := proxyHopDialer(
		func(context.Context, string, string) (net.Conn, error) {
			dialed = append(dialed, "proxy")
			return nil, errors.New("stop")
		},
		func(context.Context, string, string) (net.Conn, error) {
			dialed = append(dialed, "origin")
			return nil, errors.New("stop")
		},
	)

	first, _ := http.NewRequest(http.MethodGet, "https://api.example.com/", nil)
	first, err := routing.mark(first)
	if err != nil {
		t.Fatalf("mark: %v", err)
	}
	_, _ = dial(first.Context(), "tcp", "proxy.internal:3128")

	followUp, _ := http.NewRequestWithContext(first.Context(), http.MethodGet, "http://proxy.internal:3128/", nil)
	followUp, err = routing.mark(followUp)
	if err != nil {
		t.Fatalf("mark: %v", err)
	}
	_, _ = dial(followUp.Context(), "tcp", "proxy.internal:3128")

	if len(dialed) != 2 || dialed[0] != "proxy" || dialed[1] != "origin" {
		t.Fatalf("dialers used = %v, want [proxy origin]", dialed)
	}
}

// TestTransportProxyAddr_MatchesWhatTheTransportDials compares the proxy address
// proxyRouting records for a request with the address net/http's Transport actually
// dials for that proxy.
func TestTransportProxyAddr_MatchesWhatTheTransportDials(t *testing.T) {
	for _, raw := range []string{
		"http://proxy.internal",
		"https://proxy.internal",
		"http://proxy.internal:3128",
		"socks5://127.0.0.1",
		"socks5h://proxy.internal:9050",
		"http://bücher.example:3128",
		"http://PROXY.Example:3128",
		"http://[::1]:8080",
	} {
		proxyURL, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		var got string
		transport := &http.Transport{
			Proxy: func(*http.Request) (*url.URL, error) { return proxyURL, nil },
			DialContext: func(_ context.Context, _, addr string) (net.Conn, error) {
				got = addr
				return nil, errors.New("stop")
			},
		}
		req, _ := http.NewRequest(http.MethodGet, "http://origin.example/", nil)
		_, _ = transport.RoundTrip(req)
		if got == "" {
			t.Fatalf("%s: the Transport dialed nothing", raw)
		}
		marked, err := (&proxyRouting{proxyFunc: transport.Proxy}).mark(req)
		if err != nil {
			t.Fatalf("%s: mark: %v", raw, err)
		}
		if recorded, _ := marked.Context().Value(proxyRouteKey{}).(string); recorded != got {
			t.Errorf("%s: recorded %q, Transport dialed %q", raw, recorded, got)
		}
	}
}

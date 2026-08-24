package httpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// buildProxyFunc returns the function suitable for http.Transport.Proxy,
// selected by cfg.Mode. A nil, nil return means "no proxy".
func buildProxyFunc(cfg ProxyConfig) (func(*http.Request) (*url.URL, error), error) {
	switch cfg.Mode {
	case "", "none":
		return nil, nil
	case "environment":
		return http.ProxyFromEnvironment, nil
	case "url":
		return buildExplicitProxyFunc(cfg)
	default:
		return nil, fmt.Errorf("httpclient: unknown Proxy.Mode %q (expected \"none\", \"environment\", or \"url\")", cfg.Mode)
	}
}

func buildExplicitProxyFunc(cfg ProxyConfig) (func(*http.Request) (*url.URL, error), error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("httpclient: Proxy.Mode \"url\" requires Proxy.URL to be set")
	}
	proxyURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("httpclient: invalid Proxy.URL: %w", err)
	}
	if cfg.Username != "" {
		proxyURL.User = url.UserPassword(cfg.Username, cfg.Password)
	}

	bypass, err := newNoProxyMatcher(cfg.NoProxy)
	if err != nil {
		return nil, err
	}

	return func(req *http.Request) (*url.URL, error) {
		if bypass(req.URL.Hostname()) {
			return nil, nil
		}
		return proxyURL, nil
	}, nil
}

// newNoProxyMatcher builds a matcher for a NoProxy bypass list. Each entry
// is either a CIDR, a ".suffix" domain match, or an exact hostname match
// (case-insensitive).
func newNoProxyMatcher(entries []string) (func(host string) bool, error) {
	exact := make(map[string]bool)
	var suffixes []string
	var cidrs []*net.IPNet

	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if _, n, err := net.ParseCIDR(e); err == nil {
			cidrs = append(cidrs, n)
			continue
		}
		if strings.HasPrefix(e, ".") {
			suffixes = append(suffixes, strings.ToLower(e))
			continue
		}
		exact[strings.ToLower(e)] = true
	}

	return func(host string) bool {
		host = strings.ToLower(host)
		if exact[host] {
			return true
		}
		for _, s := range suffixes {
			if strings.HasSuffix(host, s) || host == strings.TrimPrefix(s, ".") {
				return true
			}
		}
		if ip := net.ParseIP(host); ip != nil {
			for _, c := range cidrs {
				if c.Contains(ip) {
					return true
				}
			}
		}
		return false
	}, nil
}

// dialProxyTLS returns a DialTLSContext function that hand-terminates the
// PROXY-facing TLS handshake using proxyTLS (Tier 2: the proxy itself needs
// its own client certificate, distinct from the origin's).
//
// http.Transport routes both the plain-proxy and https-proxy first-hop
// dial through DialContext/DialTLSContext with addr set to the PROXY's
// address (connectMethod.addr() returns the proxy's address whenever
// Transport.Proxy is set) — never the origin's, whether or not a CONNECT
// tunnel follows. This is why it is safe for dialProxyTLS to assume addr
// here is always the proxy, and why it must not attempt to apply origin
// policy at this layer; Transport performs its own second, nested TLS
// handshake for the origin afterwards using Transport.TLSClientConfig.
//
// This behavior is exercised by transport_connect_test.go against this
// module's pinned Go toolchain version; re-verify on every Go upgrade, since
// DialTLSContext's doc comment describes it more narrowly ("non-proxied
// HTTPS requests") than what net/http's dialConn gating actually does today.
func dialProxyTLS(dialFn func(context.Context, string, string) (net.Conn, error), proxyTLS *tls.Config, handshakeTimeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		raw, err := dialFn(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		host, _, splitErr := net.SplitHostPort(addr)
		if splitErr != nil {
			raw.Close()
			return nil, fmt.Errorf("httpclient: invalid proxy address")
		}

		hctx := ctx
		var cancel context.CancelFunc
		if handshakeTimeout > 0 {
			hctx, cancel = context.WithTimeout(ctx, handshakeTimeout)
			defer cancel()
		}

		tlsConn := tls.Client(raw, cloneTLSConfigForHost(proxyTLS, host))
		if err := tlsConn.HandshakeContext(hctx); err != nil {
			raw.Close()
			return nil, fmt.Errorf("httpclient: proxy TLS handshake failed")
		}
		return tlsConn, nil
	}
}

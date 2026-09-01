package httpclient

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/wso2/api-platform/httpkit/netguard"
)

// connectRoundTripper implements http.RoundTripper by hand for
// ProxyEgressManualCONNECT. It validates the origin hostname locally
// against the configured SSRF policy BEFORE ever issuing a CONNECT request
// or writing any bytes intended for the origin, then performs the proxy
// dial, CONNECT handshake, and (for an https target) the TLS handshake over
// the resulting tunnel itself.
//
// This intentionally bypasses http.Transport's connection pooling — each
// RoundTrip dials a fresh connection to the proxy. That's an accepted
// tradeoff for this advanced, comparatively rare mode; the common case
// (ProxyEgressDelegated, or no SSRF guard at all) uses the pooling
// *http.Transport from transport.go instead.
type connectRoundTripper struct {
	proxyURL            *url.URL
	dialProxy           func(ctx context.Context, network, addr string) (net.Conn, error)
	originPolicy        netguard.Policy
	originTLS           *tls.Config
	connectHeader       func(ctx context.Context, proxyURL *url.URL, target string) (http.Header, error)
	tlsHandshakeTimeout time.Duration
}

// newConnectRoundTripper builds the RoundTripper used for
// ProxyEgressManualCONNECT. It requires an explicit Proxy.URL (Mode
// "environment" resolves per-request and doesn't fit this mode's
// dial-before-you-know-the-target-is-safe model) and requires SSRF.Enabled,
// since this mode exists specifically to validate the origin against
// SSRF.Policy before it's ever handed to the proxy.
func newConnectRoundTripper(cfg Config, dialFn func(context.Context, string, string) (net.Conn, error), originTLS *tls.Config) (http.RoundTripper, error) {
	if cfg.Proxy.Mode != "url" {
		return nil, fmt.Errorf("httpclient: Proxy.Egress == ProxyEgressManualCONNECT requires Proxy.Mode \"url\" (an explicit, fixed proxy target)")
	}
	if cfg.Proxy.URL == "" {
		return nil, fmt.Errorf("httpclient: Proxy.Mode \"url\" requires Proxy.URL to be set")
	}
	if !cfg.SSRF.Enabled {
		return nil, fmt.Errorf("httpclient: Proxy.Egress == ProxyEgressManualCONNECT requires SSRF.Enabled (this mode exists to validate the origin against SSRF.Policy)")
	}

	proxyURL, err := url.Parse(cfg.Proxy.URL)
	if err != nil {
		return nil, fmt.Errorf("httpclient: invalid Proxy.URL: %w", err)
	}
	if cfg.Proxy.Username != "" {
		proxyURL.User = url.UserPassword(cfg.Proxy.Username, cfg.Proxy.Password)
	}

	return &connectRoundTripper{
		proxyURL:            proxyURL,
		dialProxy:           dialFn,
		originPolicy:        cfg.SSRF.Policy,
		originTLS:           originTLS,
		connectHeader:       cfg.Proxy.ConnectHeader,
		tlsHandshakeTimeout: cfg.Timeouts.TLSHandshake,
	}, nil
}

func (c *connectRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// Validate the origin BEFORE any network activity aimed at it — this is
	// the entire point of this mode. A generic error is returned; the
	// resolved address/reason is not, so as not to leak internal topology.
	if err := netguard.Validate(ctx, c.originPolicy, req.URL.Hostname()); err != nil {
		return nil, fmt.Errorf("httpclient: origin destination rejected")
	}

	conn, err := c.dialProxy(ctx, "tcp", canonicalAddr(c.proxyURL))
	if err != nil {
		return nil, fmt.Errorf("httpclient: failed to connect to proxy")
	}
	ok := false
	defer func() {
		if !ok {
			conn.Close()
		}
	}()

	if c.proxyURL.Scheme == "https" {
		conn, err = c.handshake(ctx, conn, c.proxyURL.Hostname(), &tls.Config{})
		if err != nil {
			return nil, fmt.Errorf("httpclient: proxy TLS handshake failed")
		}
	}

	target := canonicalAddr(req.URL)
	if req.URL.Scheme == "https" {
		if err := c.writeConnect(ctx, conn, target); err != nil {
			return nil, err
		}
		conn, err = c.handshake(ctx, conn, req.URL.Hostname(), c.originTLS)
		if err != nil {
			return nil, fmt.Errorf("httpclient: origin TLS handshake failed")
		}
	}

	if err := req.Write(conn); err != nil {
		return nil, fmt.Errorf("httpclient: failed to write request")
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, fmt.Errorf("httpclient: failed to read response")
	}
	ok = true // resp.Body now owns closing conn
	resp.Body = &connOwningBody{ReadCloser: resp.Body, conn: conn}
	return resp, nil
}

// handshake wraps conn in a TLS client connection for host, using
// cloneTLSConfigForHost so ServerName always comes from the intended
// hostname rather than the dialed address.
func (c *connectRoundTripper) handshake(ctx context.Context, conn net.Conn, host string, base *tls.Config) (net.Conn, error) {
	hctx := ctx
	if c.tlsHandshakeTimeout > 0 {
		var cancel context.CancelFunc
		hctx, cancel = context.WithTimeout(ctx, c.tlsHandshakeTimeout)
		defer cancel()
	}
	tlsConn := tls.Client(conn, cloneTLSConfigForHost(base, host))
	if err := tlsConn.HandshakeContext(hctx); err != nil {
		return nil, err
	}
	return tlsConn, nil
}

// writeConnect issues a CONNECT request for target over conn and consumes
// the proxy's response, returning an error unless it reports success.
func (c *connectRoundTripper) writeConnect(ctx context.Context, conn net.Conn, target string) error {
	header := make(http.Header)
	if c.connectHeader != nil {
		h, err := c.connectHeader(ctx, c.proxyURL, target)
		if err != nil {
			return fmt.Errorf("httpclient: failed to build CONNECT headers")
		}
		if h != nil {
			header = h
		}
	}
	if u := c.proxyURL.User; u != nil {
		password, _ := u.Password()
		header.Set("Proxy-Authorization", basicAuth(u.Username(), password))
	}

	connectReq := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: target},
		Host:   target,
		Header: header,
	}
	if err := connectReq.Write(conn); err != nil {
		return fmt.Errorf("httpclient: failed to write CONNECT request")
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), connectReq)
	if err != nil {
		return fmt.Errorf("httpclient: failed to read CONNECT response")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("httpclient: proxy refused CONNECT")
	}
	return nil
}

// canonicalAddr returns u's host:port, defaulting the port from the scheme
// when absent.
func canonicalAddr(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	port := "80"
	if u.Scheme == "https" {
		port = "443"
	}
	return net.JoinHostPort(u.Hostname(), port)
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

// connOwningBody closes the underlying connection when the response body is
// closed — http.ReadResponse's own Body, built over a bufio.Reader wrapping
// a raw net.Conn, does not take ownership of closing that conn itself.
type connOwningBody struct {
	io.ReadCloser
	conn net.Conn
}

func (b *connOwningBody) Close() error {
	bodyErr := b.ReadCloser.Close()
	connErr := b.conn.Close()
	if bodyErr != nil {
		return bodyErr
	}
	return connErr
}

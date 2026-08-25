package httpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
)

// buildTransport builds the *http.Transport used for every ProxyEgress
// mode except ProxyEgressManualCONNECT (which uses a hand-rolled
// RoundTripper instead — see roundtrip_connect.go).
//
// dialFn is always wired as Transport.DialContext, regardless of whether a
// proxy is configured. This is what makes the "guard only validates the
// proxy hop when proxying" behavior (ProxyEgressDelegated) automatic rather
// than a special case: net/http's Transport always dials
// connectMethod.addr(), which is the proxy's address whenever Proxy is set
// and the origin's address otherwise — dialFn sees whichever one Transport
// asks for and applies the same policy either way.
func buildTransport(cfg Config, dialFn func(context.Context, string, string) (net.Conn, error), originTLS, proxyTLS *tls.Config) (*http.Transport, error) {
	proxyFunc, err := buildProxyFunc(cfg.Proxy)
	if err != nil {
		return nil, err
	}

	t := &http.Transport{
		Proxy:                 proxyFunc,
		DialContext:           dialFn,
		TLSClientConfig:       originTLS,
		MaxIdleConns:          cfg.Pooling.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.Pooling.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.Pooling.MaxConnsPerHost,
		IdleConnTimeout:       cfg.Pooling.IdleConnTimeout,
		DisableKeepAlives:     cfg.Pooling.DisableKeepAlives,
		TLSHandshakeTimeout:   cfg.Timeouts.TLSHandshake,
		ResponseHeaderTimeout: cfg.Timeouts.ResponseHeader,
		ExpectContinueTimeout: cfg.Timeouts.ExpectContinue,
		// ForceAttemptHTTP2 defaults to false: Go's Transport already
		// disables HTTP/2 conservatively whenever a custom DialContext or
		// TLSClientConfig is set (both always true here) unless this is
		// explicitly requested — see PoolingConfig.EnableHTTP2's doc for the
		// connection-coalescing tradeoff this opt-in carries.
		ForceAttemptHTTP2: cfg.Pooling.EnableHTTP2,
	}

	if proxyTLS != nil {
		if proxyFunc == nil {
			return nil, fmt.Errorf("httpclient: Proxy.ProxyTLS is set but Proxy.Mode is \"none\" — a proxy must be configured to have its own TLS settings")
		}
		t.DialTLSContext = dialProxyTLS(dialFn, proxyTLS, cfg.Timeouts.TLSHandshake)
	}

	if cfg.Proxy.ConnectHeader != nil {
		connectHeader := cfg.Proxy.ConnectHeader
		t.GetProxyConnectHeader = func(ctx context.Context, proxyURL *url.URL, target string) (http.Header, error) {
			return connectHeader(ctx, proxyURL, target)
		}
	}

	return t, nil
}

// maxBytesRoundTripper wraps a RoundTripper so that reading a response body
// past a configured byte ceiling returns an error instead of continuing
// unbounded, per go-network-service-hardening.md's requirement that every
// inbound reader is bounded.
type maxBytesRoundTripper struct {
	next http.RoundTripper
	max  int64
}

func (m *maxBytesRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := m.next.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}
	resp.Body = &maxBytesReadCloser{body: resp.Body, max: m.max}
	return resp, nil
}

// maxBytesReadCloser bounds how many bytes may be read from body. Reading
// within the limit behaves exactly like the underlying reader (including a
// natural io.EOF for a body that ends at or before the limit); only once
// more than max bytes have actually been read does it return an error, so a
// body of precisely max bytes is never misreported as having exceeded the
// limit.
type maxBytesReadCloser struct {
	body io.ReadCloser
	max  int64
	read int64
}

func (m *maxBytesReadCloser) Read(p []byte) (int, error) {
	n, err := m.body.Read(p)
	m.read += int64(n)
	if m.read > m.max {
		return n, fmt.Errorf("httpclient: response body exceeds the configured maximum size (%d bytes)", m.max)
	}
	return n, err
}

func (m *maxBytesReadCloser) Close() error {
	return m.body.Close()
}

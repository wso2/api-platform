package httpclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/wso2/go-httpkit/netguard"
)

// Config describes how to build an outbound *http.Client. Use DefaultConfig
// to obtain sane pooling/timeout defaults, then override only the fields a
// caller needs to change — the TLS, Proxy, and SSRF zero values are all
// meaningful ("no client cert", "no proxy", "no SSRF guard") rather than
// placeholders that must be filled in.
type Config struct {
	Pooling  PoolingConfig
	Timeouts TimeoutsConfig
	TLS      TLSConfig
	Proxy    ProxyConfig
	SSRF     SSRFConfig
}

// PoolingConfig controls http.Transport's connection pooling behavior.
type PoolingConfig struct {
	// MaxIdleConns, MaxIdleConnsPerHost, and MaxConnsPerHost mirror the
	// identically-named http.Transport fields.
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	// IdleConnTimeout mirrors http.Transport.IdleConnTimeout.
	IdleConnTimeout time.Duration
	// KeepAlive mirrors net.Dialer.KeepAlive for the underlying TCP dialer.
	KeepAlive time.Duration
	// DisableKeepAlives mirrors http.Transport.DisableKeepAlives. Reusing
	// pooled connections carries no SSRF/DNS-rebinding risk (see the
	// netguard package doc), so this defaults to false even when the SSRF
	// guard is enabled.
	DisableKeepAlives bool
	// EnableHTTP2 opts into HTTP/2. It defaults to false: this package
	// always sets a custom DialContext and/or TLSClientConfig, which makes
	// Go's own Transport conservatively disable HTTP/2 unless explicitly
	// re-enabled — and HTTP/2 connection coalescing (RFC 7540 §9.1.1) can
	// reuse an established connection for a different, SAN-covered hostname
	// without a fresh DialContext validation. Only enable this if that
	// tradeoff has been considered for the caller's use case.
	EnableHTTP2 bool
}

// TimeoutsConfig bounds every phase of an outbound request. Per
// go-network-service-hardening.md, DefaultConfig never leaves these at
// Go's unbounded zero values.
type TimeoutsConfig struct {
	// Overall mirrors http.Client.Timeout — the end-to-end budget for a
	// single request including any redirects.
	Overall time.Duration
	// Dial bounds the TCP connect phase.
	Dial time.Duration
	// TLSHandshake mirrors http.Transport.TLSHandshakeTimeout.
	TLSHandshake time.Duration
	// ResponseHeader mirrors http.Transport.ResponseHeaderTimeout.
	ResponseHeader time.Duration
	// ExpectContinue mirrors http.Transport.ExpectContinueTimeout.
	ExpectContinue time.Duration
	// MaxResponseBytes bounds how much of a response body the returned
	// client will read before erroring, so an oversized or hostile response
	// cannot exhaust memory. 0 uses the package default
	// (defaultMaxResponseBytes); a negative value disables the bound
	// entirely (opt-in, for callers that stream large trusted payloads).
	MaxResponseBytes int64
}

// TLSConfig controls the TLS handshake used for the ORIGIN connection —
// i.e. the server ultimately being talked to, whether reached directly or
// through a CONNECT-tunneling proxy. See ProxyTLSConfig for the separate,
// proxy-facing TLS handshake.
type TLSConfig struct {
	// MinVersion and MaxVersion use the tlsconfig.ParseVersion vocabulary
	// ("TLS1_0".."TLS1_3"). Both empty uses Go's own defaults.
	MinVersion, MaxVersion string
	// CipherSuites is a comma-separated list of Go crypto/tls cipher suite
	// names. Empty uses Go's own default secure set. Only affects TLS 1.2
	// and below.
	CipherSuites string
	// CurvePreferences is a comma-separated, order-significant list of
	// curve/group names, e.g. "X25519MLKEM768,X25519,P-256" to prefer the
	// FIPS 203 ML-KEM-768 hybrid group while retaining classical fallbacks
	// for a peer that doesn't support it yet. Empty uses Go's own defaults
	// (no PQC) — enabling a hybrid group is always an explicit opt-in here,
	// never this package's unconditional default.
	CurvePreferences string

	// RootCAFile loads a PEM-encoded CA bundle from disk. RootCAs, if set,
	// takes precedence and is used as-is (e.g. for a caller that already
	// manages certificate rotation itself). Both empty uses Go's system
	// root pool.
	RootCAFile string
	RootCAs    *x509.CertPool

	// ClientCertFile/ClientKeyFile load a PEM client certificate/key pair
	// for mTLS to the origin. GetClientCertificate, if set, is used instead
	// (e.g. for rotation) and the two file fields are ignored.
	ClientCertFile, ClientKeyFile string
	GetClientCertificate          func(*tls.CertificateRequestInfo) (*tls.Certificate, error)

	// InsecureSkipVerify disables certificate chain and hostname
	// verification entirely. It is a narrow, explicitly-named, off-by-
	// default escape hatch: New returns an error unless
	// InsecureSkipVerifyAcknowledged is also true, and it can never be
	// combined with VerifyPeerCertificate/VerifyConnection (see below).
	InsecureSkipVerify             bool
	InsecureSkipVerifyAcknowledged bool

	// VerifyPeerCertificate and VerifyConnection are run IN ADDITION TO,
	// never instead of, Go's own default verification — New rejects either
	// one being set alongside InsecureSkipVerify == true, since a custom
	// callback would then silently become the only check performed (its
	// verifiedChains argument is empty when default verification didn't
	// run). Use these only to add an extra check (e.g. certificate
	// pinning) on top of a chain Go has already verified.
	VerifyPeerCertificate func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error
	VerifyConnection      func(tls.ConnectionState) error
}

// ProxyConfig configures forward-proxy use.
type ProxyConfig struct {
	// Mode selects how the proxy is determined: "none" (default, no proxy),
	// "environment" (use HTTP_PROXY/HTTPS_PROXY/NO_PROXY via Go's own
	// http.ProxyFromEnvironment), or "url" (use URL below, with NoProxy as
	// an explicit bypass list).
	Mode string
	// URL is the proxy URL, used when Mode == "url".
	URL string
	// Username and Password set basic auth credentials for the proxy
	// connection (Proxy-Authorization), used when Mode == "url".
	Username, Password string
	// NoProxy lists hosts to bypass the proxy for, used when Mode == "url".
	// Each entry is an exact host, a ".suffix" domain match, or a CIDR.
	NoProxy []string

	// ProxyTLS configures a SEPARATE, distinct TLS handshake to an
	// https:// proxy itself (e.g. the proxy requires its own client
	// certificate, different from TLS.ClientCertFile/Key used for the
	// origin). Leave nil for the common case where either the proxy is
	// plain (http://) or the proxy's own TLS needs no client cert.
	ProxyTLS *ProxyTLSConfig

	// ConnectHeader, if set, supplies additional headers on the CONNECT
	// request to the proxy (e.g. a bearer-token proxy-auth scheme) —
	// passed through to http.Transport.GetProxyConnectHeader.
	ConnectHeader func(ctx context.Context, proxyURL *url.URL, target string) (http.Header, error)

	// Egress must be set explicitly whenever Mode != "none" AND
	// SSRF.Enabled — see the package doc for why a dial-time IP guard
	// cannot protect the proxied origin.
	Egress ProxyEgressPolicy
}

// ProxyTLSConfig configures the TLS handshake to the proxy itself, fully
// decoupled from TLSConfig (which always governs the origin handshake).
type ProxyTLSConfig struct {
	// RootCAFile loads a PEM-encoded CA bundle from disk. RootCAs, if set,
	// takes precedence. Both empty uses Go's system root pool.
	RootCAFile string
	RootCAs    *x509.CertPool

	// ClientCertFile/ClientKeyFile load a PEM client certificate/key pair
	// for mTLS to the proxy. GetClientCertificate, if set, is used instead
	// and the two file fields are ignored.
	ClientCertFile, ClientKeyFile string
	GetClientCertificate          func(*tls.CertificateRequestInfo) (*tls.Certificate, error)

	InsecureSkipVerify             bool
	InsecureSkipVerifyAcknowledged bool
}

// ProxyEgressPolicy states how origin-destination SSRF risk is handled when
// a forward proxy is also configured.
type ProxyEgressPolicy int

const (
	// ProxyEgressUnset is the zero value. New returns an error if this is
	// left unset while both a proxy and the SSRF guard are configured —
	// this policy must always be a deliberate choice, never a default.
	ProxyEgressUnset ProxyEgressPolicy = iota
	// ProxyEgressDelegated trusts the proxy's own network egress controls
	// for the proxied origin. The dial-time guard still validates the
	// proxy's own resolved address (and CheckRedirect still applies its
	// scheme/host policy), but the origin itself is not validated by this
	// library while proxying.
	ProxyEgressDelegated
	// ProxyEgressManualCONNECT gives up http.Transport's native proxy
	// support and uses a hand-rolled http.RoundTripper that resolves and
	// validates the origin hostname itself, locally, before ever issuing a
	// CONNECT request. This is defense-in-depth against what this process
	// itself would resolve — it does not guarantee the proxy resolves or
	// routes the origin the same way.
	ProxyEgressManualCONNECT
)

// SSRFConfig configures the dial-time SSRF guard (see the netguard
// package). It is disabled by default: enabling it is always an explicit,
// per-caller choice, since two legitimate use cases already in this
// codebase disagree on which addresses should be reachable.
type SSRFConfig struct {
	// Enabled turns the guard on. When true, Policy must be a non-zero
	// netguard.Policy — New returns an error otherwise, rather than
	// silently falling back to one preset over another.
	Enabled bool
	Policy  netguard.Policy
	// MaxRedirects bounds redirect hops. 0 uses netguard's own default (5);
	// a negative value is not distinguished from zero.
	MaxRedirects int
}

// buildState accumulates the effect of functional Options during New.
type buildState struct {
	roundTripperWrappers []func(http.RoundTripper) http.RoundTripper
	dialOverride         func(ctx context.Context, network, addr string) (net.Conn, error)
}

// Option customizes client construction with a hook that cannot be
// expressed as plain configuration data.
type Option func(*buildState)

// WithRoundTripperWrapper wraps the built http.RoundTripper with wrap,
// closest to Client.Do — e.g. for attaching metrics/tracing instrumentation.
// Wrappers are applied in the order given.
func WithRoundTripperWrapper(wrap func(http.RoundTripper) http.RoundTripper) Option {
	return func(s *buildState) {
		s.roundTripperWrappers = append(s.roundTripperWrappers, wrap)
	}
}

// WithDialContext overrides the dial function New would otherwise choose
// (plain or netguard-guarded). Intended for tests; using it bypasses
// whatever SSRF policy Config.SSRF would otherwise have applied.
func WithDialContext(dial func(ctx context.Context, network, addr string) (net.Conn, error)) Option {
	return func(s *buildState) {
		s.dialOverride = dial
	}
}

// defaultMaxResponseBytes bounds a response body when TimeoutsConfig
// doesn't specify one, per go-network-service-hardening.md's requirement
// that inbound readers are never left unbounded.
const defaultMaxResponseBytes int64 = 10 << 20 // 10 MiB

// DefaultConfig returns a Config with sane, non-zero pooling and timeout
// defaults. TLS, Proxy, and SSRF are left at their zero values (no client
// cert, no proxy, no SSRF guard) for the caller to opt into explicitly.
func DefaultConfig() Config {
	return Config{
		Pooling: PoolingConfig{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			MaxConnsPerHost:     100,
			IdleConnTimeout:     90 * time.Second,
			KeepAlive:           30 * time.Second,
		},
		Timeouts: TimeoutsConfig{
			Overall:          30 * time.Second,
			Dial:             10 * time.Second,
			TLSHandshake:     10 * time.Second,
			ResponseHeader:   10 * time.Second,
			ExpectContinue:   1 * time.Second,
			MaxResponseBytes: defaultMaxResponseBytes,
		},
	}
}

// New builds an *http.Client from cfg. It validates cfg up front and fails
// closed on any ambiguous or unsafe combination (see the package doc)
// rather than silently choosing a default stance.
func New(cfg Config, opts ...Option) (*http.Client, error) {
	state := &buildState{}
	for _, opt := range opts {
		opt(state)
	}

	if cfg.SSRF.Enabled && isZeroPolicy(cfg.SSRF.Policy) {
		return nil, fmt.Errorf("httpclient: SSRF.Enabled requires a non-zero SSRF.Policy (see netguard.PermitPrivateBlockMetadata / netguard.PublicOnly)")
	}
	if cfg.Proxy.Mode != "" && cfg.Proxy.Mode != "none" && cfg.SSRF.Enabled && cfg.Proxy.Egress == ProxyEgressUnset {
		return nil, fmt.Errorf("httpclient: Proxy and SSRF are both configured — Proxy.Egress must be set explicitly to ProxyEgressDelegated or ProxyEgressManualCONNECT (a dial-time guard cannot see the proxied origin, only the proxy's own address)")
	}

	originTLS, err := buildTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}

	var proxyTLS *tls.Config
	if cfg.Proxy.ProxyTLS != nil {
		proxyTLS, err = buildProxyTLSConfig(*cfg.Proxy.ProxyTLS)
		if err != nil {
			return nil, err
		}
	}

	dialFn := state.dialOverride
	if dialFn == nil {
		if cfg.SSRF.Enabled {
			dialFn = netguard.DialContext(cfg.SSRF.Policy, cfg.Timeouts.Dial)
		} else {
			dialFn = (&net.Dialer{Timeout: cfg.Timeouts.Dial, KeepAlive: cfg.Pooling.KeepAlive}).DialContext
		}
	}

	var rt http.RoundTripper
	if cfg.Proxy.Egress == ProxyEgressManualCONNECT {
		rt, err = newConnectRoundTripper(cfg, dialFn, originTLS)
		if err != nil {
			return nil, err
		}
	} else {
		rt, err = buildTransport(cfg, dialFn, originTLS, proxyTLS)
		if err != nil {
			return nil, err
		}
	}

	maxBytes := cfg.Timeouts.MaxResponseBytes
	if maxBytes == 0 {
		maxBytes = defaultMaxResponseBytes
	}
	if maxBytes > 0 {
		rt = &maxBytesRoundTripper{next: rt, max: maxBytes}
	}

	for _, wrap := range state.roundTripperWrappers {
		rt = wrap(rt)
	}

	client := &http.Client{
		Transport: rt,
		Timeout:   cfg.Timeouts.Overall,
	}
	if cfg.SSRF.Enabled {
		redirectPolicy := cfg.SSRF.Policy
		if len(redirectPolicy.AllowedSchemes) == 0 {
			redirectPolicy.AllowedSchemes = []string{"https"}
		}
		client.CheckRedirect = netguard.CheckRedirect(redirectPolicy, cfg.SSRF.MaxRedirects)
	}

	return client, nil
}

// isZeroPolicy reports whether p is the zero-value Policy — used to detect
// a caller enabling SSRF.Enabled without picking a preset. Policy contains
// slice fields, so it cannot be compared with ==; every field is checked
// individually instead.
func isZeroPolicy(p netguard.Policy) bool {
	return !p.BlockPrivate && !p.BlockLoopback && !p.BlockLinkLocal && !p.BlockUnspecified &&
		!p.BlockMulticastBroadcast && !p.BlockCGNAT &&
		len(p.DenyCIDRs) == 0 && len(p.AllowCIDRs) == 0 && len(p.AllowedSchemes) == 0
}

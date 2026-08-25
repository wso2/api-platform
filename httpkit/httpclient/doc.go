// Package httpclient builds a secure-by-default outbound *http.Client for
// use by any Go component in this repo. It composes connection pooling,
// forward-proxy support (including mTLS tunneled through a CONNECT proxy),
// fine-grained TLS control (cipher suites, ECDH/curve preferences including
// post-quantum hybrid groups), and an optional SSRF dial-time guard, while
// keeping hostname verification a property that cannot be silently
// disabled.
//
// # Hostname verification
//
// Config never exposes a fixed tls.Config.ServerName: Go's own TLS dialing
// only fills that field in per-target when it is left empty, and a client
// built by this package is expected to be reused across many hosts. Setting
// TLS.InsecureSkipVerify requires also setting
// TLS.InsecureSkipVerifyAcknowledged — flipping one boolean is not enough to
// disable verification. A custom TLS.VerifyPeerCertificate or
// VerifyConnection may only be combined with InsecureSkipVerify == false;
// New returns an error otherwise, since a custom callback silently becomes
// the *only* check once Go's own verification is disabled.
//
// # SSRF guard composed with a forward proxy
//
// A dial-time IP guard (see the netguard package) can only validate the
// address this process itself dials. When a forward proxy is configured,
// that is the proxy's address, never the proxied origin's — the origin
// hostname is only ever placed in a CONNECT request line, generated after
// the guarded dial has already completed. Configuring both SSRF.Enabled and
// a non-"none" Proxy.Mode therefore requires an explicit Proxy.Egress
// choice; New returns an error if it is left at its zero value
// (ProxyEgressUnset), so that a caller can never end up believing the
// origin is SSRF-protected when it silently isn't.
package httpclient

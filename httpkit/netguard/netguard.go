// Package netguard provides a shared, dial-time SSRF guard for outbound
// HTTP(S) calls whose destination is influenced, even partially, by
// untrusted input (request bodies, tenant configuration, redirects).
//
// The guard resolves the target host and validates every candidate address
// against a caller-supplied Policy inside the dial itself — not as a
// separate pre-check — so a DNS answer that changes between a check and a
// later connection attempt (DNS rebinding) cannot smuggle a disallowed
// address past the guard. Once a connection has been dialed and validated,
// reusing it from an http.Transport's connection pool carries no additional
// rebinding risk: the socket is already bound to the specific IP that was
// checked, and rebinding only affects a *future* dial for that hostname.
//
// netguard has no built-in opinion on which addresses are legitimate — that
// is a property of the caller's own threat model (see Policy and the
// presets in presets.go) — and it has no awareness of HTTP proxying. A
// dial-time guard wired via DialContext only ever sees, and can only ever
// validate, the address this process itself dials; when a forward proxy is
// configured, that address is the proxy's, never the proxied origin's. See
// the httpclient package for how the two are composed.
package netguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Policy describes which resolved IP addresses a guarded dial may connect
// to. The zero value rejects every address — a Policy must be built via one
// of the presets in presets.go, or assembled explicitly, before use; there
// is deliberately no "default" stance, since two legitimate use cases
// already in this codebase disagree (one permits private/RFC1918 ranges as
// ordinary upstreams, the other requires a fully public address).
type Policy struct {
	// BlockPrivate refuses RFC 1918 (IPv4) and unique local (IPv6 ULA,
	// fc00::/7) addresses.
	BlockPrivate bool
	// BlockLoopback refuses 127.0.0.0/8 and ::1.
	BlockLoopback bool
	// BlockLinkLocal refuses 169.254.0.0/16 and fe80::/10 — this is where
	// the cloud instance metadata endpoint (169.254.169.254) lives, so this
	// is refused by every preset regardless of the private-address stance.
	BlockLinkLocal bool
	// BlockUnspecified refuses 0.0.0.0 and ::, which the OS can reinterpret
	// as "local host".
	BlockUnspecified bool
	// BlockMulticastBroadcast refuses multicast and the IPv4 broadcast
	// address, neither of which is a meaningful HTTP peer.
	BlockMulticastBroadcast bool
	// BlockCGNAT refuses 100.64.0.0/10 (RFC 6598 carrier-grade NAT shared
	// address space), which net.IP.IsPrivate does not cover but which can
	// still route to internal infrastructure.
	BlockCGNAT bool

	// DenyCIDRs lists additional address ranges to refuse, beyond the
	// Block* categories above (e.g. an operator's own internal VPC CIDR).
	DenyCIDRs []*net.IPNet
	// AllowCIDRs narrows DenyCIDRs and the Block* categories: an address
	// matching AllowCIDRs is permitted even if it would otherwise be
	// refused. This is an explicit, off-by-default admin opt-in — it must
	// never be used to widen policy implicitly, only to carve out a
	// specific, deliberately-approved exception.
	AllowCIDRs []*net.IPNet

	// AllowedSchemes lists the URL schemes CheckRedirect permits a redirect
	// to target. Empty defaults to {"https"} — "http" must be added
	// explicitly.
	AllowedSchemes []string
}

// allowed reports whether ip is permitted by the policy.
func (p Policy) allowed(ip net.IP) bool {
	if ip == nil {
		return false
	}

	for _, cidr := range p.AllowCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}

	if p.BlockLoopback && ip.IsLoopback() {
		return false
	}
	if p.BlockPrivate && (ip.IsPrivate() || isIPv6ULA(ip)) {
		return false
	}
	if p.BlockLinkLocal && (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		return false
	}
	if p.BlockUnspecified && ip.IsUnspecified() {
		return false
	}
	if p.BlockMulticastBroadcast && (ip.IsMulticast() || ip.Equal(net.IPv4bcast)) {
		return false
	}
	if p.BlockCGNAT && cgnatRange.Contains(ip) {
		return false
	}
	for _, cidr := range p.DenyCIDRs {
		if cidr.Contains(ip) {
			return false
		}
	}
	return true
}

// isIPv6ULA reports whether ip is an IPv6 Unique Local Address (fc00::/7).
// net.IP has no built-in check for this range.
func isIPv6ULA(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 != nil {
		return false
	}
	return len(ip) == net.IPv6len && ip[0]&0xfe == 0xfc
}

// cgnatRange is RFC 6598 shared address space (carrier-grade NAT):
// 100.64.0.0/10. net.IP has no built-in check for this range, yet it can
// route to internal infrastructure.
var cgnatRange = func() *net.IPNet {
	_, n, err := net.ParseCIDR("100.64.0.0/10")
	if err != nil {
		panic("netguard: invalid built-in CGNAT CIDR: " + err.Error())
	}
	return n
}()

// allowedSchemes returns p.AllowedSchemes, defaulting to {"https"} when
// unset.
func (p Policy) allowedSchemes() []string {
	if len(p.AllowedSchemes) > 0 {
		return p.AllowedSchemes
	}
	return []string{"https"}
}

func (p Policy) schemeAllowed(scheme string) bool {
	for _, s := range p.allowedSchemes() {
		if strings.EqualFold(s, scheme) {
			return true
		}
	}
	return false
}

// DialContext returns a dial function suitable for http.Transport.DialContext
// (or direct use with a net.Dialer-shaped caller) that resolves the target
// host, refuses to dial any candidate address the policy disallows, and then
// dials the exact resolved address it just approved — never the original
// hostname string again. Performing the resolution and the connection in a
// single step is what closes the DNS-rebinding window: a name that resolves
// to an allowed address during validation cannot resolve to a different one
// by the time the connection is actually made, because no time passes
// between the two.
//
// Rejections return a generic error; the specific resolved address and
// reason are deliberately not included, so a caller returning this error to
// an end user does not leak internal network topology. Callers that want the
// concrete reason for internal logging should wrap this dialer and inspect
// the address themselves before calling it.
func DialContext(policy Policy, timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("netguard: invalid address")
		}

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("netguard: failed to resolve host")
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("netguard: host has no addresses")
		}
		for _, ip := range ips {
			if !policy.allowed(ip.IP) {
				return nil, fmt.Errorf("netguard: host resolves to a disallowed address")
			}
		}

		dialer := &net.Dialer{Timeout: timeout}
		var lastErr error
		for _, ip := range ips {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		if lastErr != nil {
			return nil, fmt.Errorf("netguard: failed to connect to host")
		}
		return nil, fmt.Errorf("netguard: failed to connect to host")
	}
}

// Validate resolves host and checks every candidate address against
// policy, without dialing. It exists for callers that must validate a
// destination locally before routing the actual connection somewhere this
// package has no visibility into — e.g. httpclient's ProxyEgressManualCONNECT
// mode, which validates an origin hostname before handing it to a forward
// proxy in a CONNECT request. This is defense-in-depth only: it reflects
// what THIS process resolves, not necessarily what a downstream proxy will
// actually connect to.
func Validate(ctx context.Context, policy Policy, host string) error {
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("netguard: failed to resolve host")
	}
	if len(ips) == 0 {
		return fmt.Errorf("netguard: host has no addresses")
	}
	for _, ip := range ips {
		if !policy.allowed(ip.IP) {
			return fmt.Errorf("netguard: host resolves to a disallowed address")
		}
	}
	return nil
}

// defaultMaxRedirects is used by CheckRedirect when maxRedirects <= 0.
const defaultMaxRedirects = 5

// CheckRedirect builds an http.Client.CheckRedirect callback that bounds
// redirect loops, restricts the scheme of each hop to policy.AllowedSchemes,
// and refuses any hop that leaves the host of the original request.
//
// The host restriction exists because callers of a guarded client routinely
// pass their own credentials as headers (an upstream's auth header, for
// instance); net/http only strips Authorization/Cookie-style headers across
// a cross-host redirect, and forwards any custom header name verbatim. A
// malicious or compromised upstream could otherwise answer with a redirect
// to a host it controls and be handed the caller's credential. Same-host
// redirects still dial through the guarded DialContext, so the address
// policy is re-applied on every hop, not just the first.
//
// maxRedirects <= 0 uses defaultMaxRedirects (5); a negative value is not
// distinguished from zero — pass a Transport/Client with CheckRedirect
// itself set to reject all redirects if none should ever be followed.
func CheckRedirect(policy Policy, maxRedirects int) func(*http.Request, []*http.Request) error {
	if maxRedirects <= 0 {
		maxRedirects = defaultMaxRedirects
	}
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("netguard: too many redirects")
		}
		if !policy.schemeAllowed(req.URL.Scheme) {
			return fmt.Errorf("netguard: redirect to a disallowed scheme")
		}
		// via[0] is the original request; Host carries the port, so a port
		// change counts as a different host too.
		if len(via) > 0 && !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
			return fmt.Errorf("netguard: redirect to a different host")
		}
		return nil
	}
}

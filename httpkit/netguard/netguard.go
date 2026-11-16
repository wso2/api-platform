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
	"net/url"
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
	// It also refuses fd00:ec2::254, the EC2 metadata service's IPv6 address.
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

	// Hosts restricts which hostnames a destination may use (see
	// CompileHostRule). nil permits every hostname. It is applied to the
	// destination's URL hostname by Validate, CheckURL, CheckRedirect and
	// httpclient's request wrapper — never by DialContext, which may be
	// dialing a forward proxy rather than the destination itself.
	Hosts *HostRule
}

// allowed reports whether ip is permitted by the policy. An IPv6 address
// that embeds an IPv4 address (6to4, NAT64) must also have its embedded
// address permitted, so 2002:a9fe:a9fe:: cannot reach 169.254.169.254.
// (IPv4-mapped addresses are already normalized by net.IP's own checks.)
func (p Policy) allowed(ip net.IP) bool {
	if !p.allowedAddr(ip) {
		return false
	}
	if v4 := embeddedIPv4(ip); v4 != nil {
		return p.allowedAddr(v4)
	}
	return true
}

// awsIPv6MetadataAddr is the EC2 instance metadata service's IPv6 address.
// It sits in unique-local space (fc00::/7), not link-local, so BlockLinkLocal
// names it explicitly to keep "link-local blocks the metadata endpoint" true
// for IPv6 as well.
var awsIPv6MetadataAddr = net.ParseIP("fd00:ec2::254")

var (
	sixToFourPrefix = mustParseCIDR("2002::/16")
	nat64Prefix     = mustParseCIDR("64:ff9b::/96")
)

// embeddedIPv4 returns the IPv4 address carried inside a 6to4 (RFC 3056)
// or NAT64 well-known-prefix (RFC 6052) IPv6 address, or nil.
func embeddedIPv4(ip net.IP) net.IP {
	if ip == nil || ip.To4() != nil || len(ip) != net.IPv6len {
		return nil
	}
	switch {
	case sixToFourPrefix.Contains(ip):
		return net.IPv4(ip[2], ip[3], ip[4], ip[5])
	case nat64Prefix.Contains(ip):
		return net.IPv4(ip[12], ip[13], ip[14], ip[15])
	}
	return nil
}

// allowedAddr applies the policy's address categories and CIDR lists to a
// single address.
func (p Policy) allowedAddr(ip net.IP) bool {
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
	if p.BlockLinkLocal && (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.Equal(awsIPv6MetadataAddr)) {
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

// refusal is the error returned when a policy refuses a destination. Its
// message names the kind of refusal, never the resolved address; errors.Is
// matches it against ErrDestinationNotAllowed.
type refusal struct{ msg string }

func (r *refusal) Error() string { return r.msg }

func (r *refusal) Is(target error) bool { return target == ErrDestinationNotAllowed }

func refused(msg string) error { return &refusal{msg: msg} }

// CheckHost reports whether host (a URL hostname, possibly a bracketed IPv6
// literal) is permitted by p.Hosts. It does not resolve the host.
func (p Policy) CheckHost(host string) error {
	if !p.Hosts.Allows(host) {
		return refused("netguard: host is not allowed")
	}
	return nil
}

// ProxyHop returns the policy a forward proxy's own address is dialed
// under. A proxy is operator configuration, not a destination taken from
// tenant or user input, so neither the host rule nor the private-network
// categories apply to it — only the categories that are never a legitimate
// peer (link-local/metadata, unspecified, multicast/broadcast), as the
// policy itself sets them.
func (p Policy) ProxyHop() Policy {
	return Policy{
		BlockLinkLocal:          p.BlockLinkLocal,
		BlockUnspecified:        p.BlockUnspecified,
		BlockMulticastBroadcast: p.BlockMulticastBroadcast,
	}
}

// CheckURL validates a destination URL taken from tenant or user input,
// without connecting: the scheme must be allowed, the hostname must pass
// p.Hosts, and an IP-literal host must be an allowed address. When resolve
// is true a hostname is also resolved and every address checked (see
// Validate); pass false when the URL will be dialed from another network
// (e.g. a gateway's upstream checked by the control plane), where this
// process's DNS answer is meaningless. Refusals match
// ErrDestinationNotAllowed; a malformed URL or a failed lookup does not.
func (p Policy) CheckURL(ctx context.Context, rawURL string, resolve bool) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return fmt.Errorf("netguard: invalid URL")
	}
	if !p.schemeAllowed(u.Scheme) {
		return refused("netguard: scheme is not allowed")
	}
	host := u.Hostname()
	if err := p.CheckHost(host); err != nil {
		return err
	}
	// Parse an IP literal from the normalized host, the form the host rule
	// matches and the HTTP transport dials: a literal written with ideographic
	// full stops (169。254。169。254) is not an IP to net.ParseIP until it is.
	ipHost := host
	if normalized, err := NormalizeHost(host); err == nil {
		ipHost = normalized
	}
	if ip := net.ParseIP(ipHost); ip != nil {
		if !p.allowed(ip) {
			return refused("netguard: address is not allowed")
		}
		return nil
	}
	if !resolve {
		return nil
	}
	return Validate(ctx, p, host)
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
				return nil, refused("netguard: host resolves to a disallowed address")
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
//
// Validate also applies policy.Hosts to host, so a refused hostname is
// rejected before any lookup.
func Validate(ctx context.Context, policy Policy, host string) error {
	if err := policy.CheckHost(host); err != nil {
		return err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("netguard: failed to resolve host")
	}
	if len(ips) == 0 {
		return fmt.Errorf("netguard: host has no addresses")
	}
	for _, ip := range ips {
		if !policy.allowed(ip.IP) {
			return refused("netguard: host resolves to a disallowed address")
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
			return refused("netguard: redirect to a disallowed scheme")
		}
		// via[0] is the original request; Host carries the port, so a port
		// change counts as a different host too.
		if len(via) > 0 && !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
			return refused("netguard: redirect to a different host")
		}
		return policy.CheckHost(req.URL.Hostname())
	}
}

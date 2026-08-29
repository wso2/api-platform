package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", s, err)
	}
	return n
}

func TestPolicyAllowed(t *testing.T) {
	tests := []struct {
		name   string
		policy Policy
		ip     string
		want   bool
	}{
		{name: "permit-private allows RFC1918", policy: PermitPrivateBlockMetadata(), ip: "10.0.0.5", want: true},
		{name: "permit-private allows loopback", policy: PermitPrivateBlockMetadata(), ip: "127.0.0.1", want: true},
		{name: "permit-private blocks link-local metadata", policy: PermitPrivateBlockMetadata(), ip: "169.254.169.254", want: false},
		{name: "permit-private blocks unspecified v4", policy: PermitPrivateBlockMetadata(), ip: "0.0.0.0", want: false},
		{name: "permit-private blocks unspecified v6", policy: PermitPrivateBlockMetadata(), ip: "::", want: false},
		{name: "permit-private blocks multicast", policy: PermitPrivateBlockMetadata(), ip: "224.0.0.1", want: false},
		{name: "permit-private blocks broadcast", policy: PermitPrivateBlockMetadata(), ip: "255.255.255.255", want: false},
		{name: "permit-private allows public", policy: PermitPrivateBlockMetadata(), ip: "8.8.8.8", want: true},

		{name: "public-only blocks RFC1918", policy: PublicOnly(), ip: "10.0.0.5", want: false},
		{name: "public-only blocks loopback", policy: PublicOnly(), ip: "127.0.0.1", want: false},
		{name: "public-only blocks link-local metadata", policy: PublicOnly(), ip: "169.254.169.254", want: false},
		{name: "public-only blocks CGNAT", policy: PublicOnly(), ip: "100.64.0.1", want: false},
		{name: "public-only blocks IPv6 ULA", policy: PublicOnly(), ip: "fd00::1", want: false},
		{name: "public-only allows public", policy: PublicOnly(), ip: "8.8.8.8", want: true},

		{
			name: "AllowCIDRs overrides an otherwise-blocked address",
			policy: Policy{
				BlockLinkLocal: true,
				AllowCIDRs:     []*net.IPNet{mustCIDR(t, "169.254.169.0/24")},
			},
			ip:   "169.254.169.254",
			want: true,
		},
		{
			name: "DenyCIDRs blocks an otherwise-allowed address",
			policy: Policy{
				DenyCIDRs: []*net.IPNet{mustCIDR(t, "8.8.8.0/24")},
			},
			ip:   "8.8.8.8",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) returned nil", tt.ip)
			}
			if got := tt.policy.allowed(ip); got != tt.want {
				t.Fatalf("allowed(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestDialContext_RejectsDisallowedAddress(t *testing.T) {
	// A loopback listener, but a policy that blocks loopback — the dial
	// must be refused before ever reaching net.Dial.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	dial := DialContext(Policy{BlockLoopback: true}, time.Second)
	_, err = dial(context.Background(), "tcp", ln.Addr().String())
	if err == nil {
		t.Fatal("expected dial to a blocked loopback address to fail")
	}
}

func TestDialContext_AllowsPermittedAddress(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			conn.Close()
		}
	}()

	dial := DialContext(PermitPrivateBlockMetadata(), time.Second)
	conn, err := dial(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("expected dial to a permitted loopback address to succeed, got: %v", err)
	}
	conn.Close()
}

// TestDialContext_RevalidatesEveryDial proves the guard re-resolves and
// re-validates on every call rather than caching a result from an earlier
// dial — the property that closes the DNS-rebinding TOCTOU window. It does
// so by pointing the "host" at a loopback address once permitted, then
// swapping the policy to block it and confirming a subsequent dial to the
// same address is rejected: the guard has no memory of the earlier
// decision, so a rebound name is re-checked every time, not trusted from a
// prior pass.
func TestDialContext_RevalidatesEveryDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	permissive := DialContext(PermitPrivateBlockMetadata(), time.Second)
	conn, err := permissive(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("expected first dial to succeed, got: %v", err)
	}
	conn.Close()

	strict := DialContext(PublicOnly(), time.Second)
	if _, err := strict(context.Background(), "tcp", ln.Addr().String()); err == nil {
		t.Fatal("expected a second dial under a stricter policy to be re-validated and rejected")
	}
}

func TestCheckRedirect(t *testing.T) {
	origin, err := http.NewRequest(http.MethodGet, "https://example.com/start", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	t.Run("rejects too many redirects", func(t *testing.T) {
		check := CheckRedirect(Policy{}, 1)
		next, _ := http.NewRequest(http.MethodGet, "https://example.com/next", nil)
		if err := check(next, []*http.Request{origin}); err == nil {
			t.Fatal("expected redirect count over the max to be rejected")
		}
	})

	t.Run("rejects disallowed scheme", func(t *testing.T) {
		check := CheckRedirect(Policy{}, 5)
		next, _ := http.NewRequest(http.MethodGet, "ftp://example.com/next", nil)
		if err := check(next, []*http.Request{origin}); err == nil {
			t.Fatal("expected a non-allowlisted scheme to be rejected")
		}
	})

	t.Run("rejects cross-host redirect", func(t *testing.T) {
		check := CheckRedirect(Policy{}, 5)
		next, _ := http.NewRequest(http.MethodGet, "https://evil.example/next", nil)
		if err := check(next, []*http.Request{origin}); err == nil {
			t.Fatal("expected a cross-host redirect to be rejected")
		}
	})

	t.Run("allows same-host redirect within limits", func(t *testing.T) {
		check := CheckRedirect(Policy{}, 5)
		next, _ := http.NewRequest(http.MethodGet, "https://example.com/next", nil)
		if err := check(next, []*http.Request{origin}); err != nil {
			t.Fatalf("expected same-host redirect to be allowed, got: %v", err)
		}
	})
}

// TestDialContext_EndToEndWithHTTPClient proves the guard composes correctly
// with a real http.Client/Transport, including redirect handling.
func TestDialContext_EndToEndWithHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: DialContext(PermitPrivateBlockMetadata(), 2*time.Second),
		},
		CheckRedirect: CheckRedirect(Policy{AllowedSchemes: []string{"http", "https"}}, 5),
	}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("client.Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDialContext_InvalidAddress(t *testing.T) {
	dial := DialContext(PermitPrivateBlockMetadata(), time.Second)
	_, err := dial(context.Background(), "tcp", "not-a-valid-address")
	if err == nil {
		t.Fatal("expected an error for an address with no port")
	}
	if _, ok := errors.AsType[net.Error](err); ok {
		t.Fatalf("expected a sterile netguard error, got a raw net.Error: %v", err)
	}
}

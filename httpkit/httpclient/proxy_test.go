package httpclient

import (
	"net/http"
	"testing"
)

func TestNewNoProxyMatcher(t *testing.T) {
	match, err := newNoProxyMatcher([]string{"internal.svc", ".corp.example.com", "10.0.0.0/8"})
	if err != nil {
		t.Fatalf("newNoProxyMatcher: %v", err)
	}

	tests := []struct {
		host string
		want bool
	}{
		{"internal.svc", true},
		{"INTERNAL.SVC", true}, // case-insensitive
		{"other.svc", false},
		{"api.corp.example.com", true},
		{"corp.example.com", true}, // exact match on the suffix's own domain
		{"example.com", false},
		{"10.1.2.3", true},
		{"11.1.2.3", false},
		{"8.8.8.8", false},
	}
	for _, tt := range tests {
		if got := match(tt.host); got != tt.want {
			t.Errorf("match(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

func TestBuildProxyFunc_Modes(t *testing.T) {
	t.Run("none returns nil func", func(t *testing.T) {
		fn, err := buildProxyFunc(ProxyConfig{Mode: "none"})
		if err != nil || fn != nil {
			t.Fatalf("buildProxyFunc(none): fn != nil = %v, err = %v, want nil, nil", fn != nil, err)
		}
	})

	t.Run("empty mode defaults to none", func(t *testing.T) {
		fn, err := buildProxyFunc(ProxyConfig{})
		if err != nil || fn != nil {
			t.Fatalf("buildProxyFunc({}): fn != nil = %v, err = %v, want nil, nil", fn != nil, err)
		}
	})

	t.Run("environment returns a func", func(t *testing.T) {
		fn, err := buildProxyFunc(ProxyConfig{Mode: "environment"})
		if err != nil || fn == nil {
			t.Fatalf("buildProxyFunc(environment): fn != nil = %v, err = %v, want true, nil", fn != nil, err)
		}
	})

	t.Run("url mode resolves to the configured proxy", func(t *testing.T) {
		fn, err := buildProxyFunc(ProxyConfig{Mode: "url", URL: "http://proxy.example:3128"})
		if err != nil {
			t.Fatalf("buildProxyFunc(url): %v", err)
		}
		req, _ := http.NewRequest(http.MethodGet, "https://target.example/", nil)
		got, err := fn(req)
		if err != nil {
			t.Fatalf("proxy func: %v", err)
		}
		if got == nil || got.Host != "proxy.example:3128" {
			t.Fatalf("proxy func returned %v, want proxy.example:3128", got)
		}
	})

	t.Run("url mode honors NoProxy bypass", func(t *testing.T) {
		fn, err := buildProxyFunc(ProxyConfig{Mode: "url", URL: "http://proxy.example:3128", NoProxy: []string{"target.example"}})
		if err != nil {
			t.Fatalf("buildProxyFunc(url): %v", err)
		}
		req, _ := http.NewRequest(http.MethodGet, "https://target.example/", nil)
		got, err := fn(req)
		if err != nil {
			t.Fatalf("proxy func: %v", err)
		}
		if got != nil {
			t.Fatalf("expected NoProxy bypass to return nil proxy, got %v", got)
		}
	})

	t.Run("url mode sets basic auth from Username/Password", func(t *testing.T) {
		fn, err := buildProxyFunc(ProxyConfig{Mode: "url", URL: "http://proxy.example:3128", Username: "u", Password: "p"})
		if err != nil {
			t.Fatalf("buildProxyFunc(url): %v", err)
		}
		req, _ := http.NewRequest(http.MethodGet, "https://target.example/", nil)
		got, err := fn(req)
		if err != nil {
			t.Fatalf("proxy func: %v", err)
		}
		if got.User == nil {
			t.Fatal("expected proxy URL to carry basic-auth userinfo")
		}
		if user := got.User.Username(); user != "u" {
			t.Fatalf("proxy user = %q, want %q", user, "u")
		}
	})

	t.Run("unknown mode is rejected", func(t *testing.T) {
		if _, err := buildProxyFunc(ProxyConfig{Mode: "socks5"}); err == nil {
			t.Fatal("expected an error for an unknown proxy mode")
		}
	})

	t.Run("url mode without URL is rejected", func(t *testing.T) {
		if _, err := buildProxyFunc(ProxyConfig{Mode: "url"}); err == nil {
			t.Fatal("expected an error when Proxy.Mode is url but Proxy.URL is empty")
		}
	})
}

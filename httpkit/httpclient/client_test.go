package httpclient

import (
	"crypto/tls"
	"testing"

	"github.com/wso2/api-platform/httpkit/netguard"
)

func TestDefaultConfig_NeverUnboundedPoolingOrTimeouts(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Pooling.MaxIdleConns <= 0 {
		t.Error("DefaultConfig: MaxIdleConns must be positive")
	}
	if cfg.Pooling.MaxIdleConnsPerHost <= 0 {
		t.Error("DefaultConfig: MaxIdleConnsPerHost must be positive")
	}
	if cfg.Pooling.IdleConnTimeout <= 0 {
		t.Error("DefaultConfig: IdleConnTimeout must be positive")
	}
	if cfg.Timeouts.Overall <= 0 {
		t.Error("DefaultConfig: Timeouts.Overall must be positive")
	}
	if cfg.Timeouts.Dial <= 0 {
		t.Error("DefaultConfig: Timeouts.Dial must be positive")
	}
	if cfg.Timeouts.TLSHandshake <= 0 {
		t.Error("DefaultConfig: Timeouts.TLSHandshake must be positive")
	}
	if cfg.Timeouts.ResponseHeader <= 0 {
		t.Error("DefaultConfig: Timeouts.ResponseHeader must be positive")
	}
	if cfg.Timeouts.MaxResponseBytes <= 0 {
		t.Error("DefaultConfig: Timeouts.MaxResponseBytes must be positive")
	}
	if cfg.SSRF.Enabled {
		t.Error("DefaultConfig: SSRF must be disabled by default (opt-in)")
	}
}

func TestNew_BuildsAClientForDefaultConfig(t *testing.T) {
	client, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New(DefaultConfig()) unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("New(DefaultConfig()) returned a nil client")
	}
	if client.Transport == nil {
		t.Fatal("client.Transport is nil")
	}
}

func TestNew_InsecureSkipVerifyRequiresAcknowledgement(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TLS.InsecureSkipVerify = true
	// InsecureSkipVerifyAcknowledged left false.
	if _, err := New(cfg); err == nil {
		t.Fatal("expected New to reject InsecureSkipVerify without acknowledgement")
	}

	cfg.TLS.InsecureSkipVerifyAcknowledged = true
	if _, err := New(cfg); err != nil {
		t.Fatalf("expected New to accept InsecureSkipVerify with acknowledgement, got: %v", err)
	}
}

func TestNew_InsecureSkipVerifyRejectsCustomVerifyCallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TLS.InsecureSkipVerify = true
	cfg.TLS.InsecureSkipVerifyAcknowledged = true
	cfg.TLS.VerifyConnection = func(cs tls.ConnectionState) error { return nil }
	if _, err := New(cfg); err == nil {
		t.Fatal("expected New to reject InsecureSkipVerify combined with a custom VerifyConnection callback")
	}
}

func TestNew_SSRFEnabledRequiresNonZeroPolicy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SSRF.Enabled = true
	// Policy left zero-value.
	if _, err := New(cfg); err == nil {
		t.Fatal("expected New to reject SSRF.Enabled with a zero-value Policy")
	}

	cfg.SSRF.Policy = netguard.PermitPrivateBlockMetadata()
	if _, err := New(cfg); err != nil {
		t.Fatalf("expected New to accept SSRF.Enabled with a preset policy, got: %v", err)
	}
}

func TestNew_ProxyPlusSSRFRequiresExplicitEgress(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SSRF.Enabled = true
	cfg.SSRF.Policy = netguard.PermitPrivateBlockMetadata()
	cfg.Proxy.Mode = "environment"
	// Egress left at ProxyEgressUnset.
	if _, err := New(cfg); err == nil {
		t.Fatal("expected New to reject Proxy+SSRF configured together without an explicit Egress choice")
	}

	cfg.Proxy.Egress = ProxyEgressDelegated
	if _, err := New(cfg); err != nil {
		t.Fatalf("expected New to accept Proxy+SSRF with ProxyEgressDelegated, got: %v", err)
	}
}

func TestNew_ProxyWithoutSSRFDoesNotRequireEgress(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Proxy.Mode = "environment"
	// SSRF disabled entirely: Egress should not be required.
	if _, err := New(cfg); err != nil {
		t.Fatalf("expected New to accept a proxy with SSRF disabled and no Egress choice, got: %v", err)
	}
}

func TestNew_RejectsUnknownProxyMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Proxy.Mode = "bogus"
	if _, err := New(cfg); err == nil {
		t.Fatal("expected New to reject an unknown Proxy.Mode")
	}
}

func TestNew_RejectsBadCipherOrCurveNames(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TLS.CipherSuites = "NOT_A_REAL_SUITE"
	if _, err := New(cfg); err == nil {
		t.Fatal("expected New to reject an unknown cipher suite name")
	}

	cfg = DefaultConfig()
	cfg.TLS.CurvePreferences = "NotACurve"
	if _, err := New(cfg); err == nil {
		t.Fatal("expected New to reject an unknown curve name")
	}
}

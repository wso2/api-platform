package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/wso2/api-platform/httpkit/tlsconfig"
)

// buildTLSConfig builds the *tls.Config used for the ORIGIN handshake
// (whether dialed directly or reached through a CONNECT tunnel). It never
// sets ServerName: Go's own TLS dialing fills that in per-target only when
// it is left empty (see net/http's addTLS), and a client built by this
// package is expected to be reused across many hosts — a fixed ServerName
// here would silently misverify every host but the first.
func buildTLSConfig(cfg TLSConfig) (*tls.Config, error) {
	if err := tlsconfig.ValidateVersionRange(cfg.MinVersion, cfg.MaxVersion); err != nil {
		return nil, fmt.Errorf("httpclient: %w", err)
	}
	if err := validateVerificationConfig(cfg.InsecureSkipVerify, cfg.InsecureSkipVerifyAcknowledged, cfg.VerifyPeerCertificate != nil || cfg.VerifyConnection != nil); err != nil {
		return nil, err
	}

	curves, err := tlsconfig.ParseCurvePreferences(cfg.CurvePreferences)
	if err != nil {
		return nil, fmt.Errorf("httpclient: %w", err)
	}
	ciphers, err := tlsconfig.ParseCipherSuites(cfg.CipherSuites)
	if err != nil {
		return nil, fmt.Errorf("httpclient: %w", err)
	}

	tlsCfg := &tls.Config{
		CurvePreferences:      curves,
		CipherSuites:          ciphers,
		InsecureSkipVerify:    cfg.InsecureSkipVerify,
		VerifyPeerCertificate: cfg.VerifyPeerCertificate,
		VerifyConnection:      cfg.VerifyConnection,
		// ServerName intentionally left unset — see the doc comment above.
	}

	if cfg.MinVersion != "" {
		v, _ := tlsconfig.ParseVersion(cfg.MinVersion)
		tlsCfg.MinVersion = v
	}
	if cfg.MaxVersion != "" {
		v, _ := tlsconfig.ParseVersion(cfg.MaxVersion)
		tlsCfg.MaxVersion = v
	}

	if cfg.RootCAs != nil {
		tlsCfg.RootCAs = cfg.RootCAs
	} else if cfg.RootCAFile != "" {
		pool, err := loadCertPool(cfg.RootCAFile)
		if err != nil {
			return nil, err
		}
		tlsCfg.RootCAs = pool
	}

	switch {
	case cfg.GetClientCertificate != nil:
		tlsCfg.GetClientCertificate = cfg.GetClientCertificate
	case cfg.ClientCertFile != "" || cfg.ClientKeyFile != "":
		if cfg.ClientCertFile == "" || cfg.ClientKeyFile == "" {
			return nil, fmt.Errorf("httpclient: TLS.ClientCertFile and TLS.ClientKeyFile must both be set for mTLS")
		}
		cert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("httpclient: failed to load client certificate")
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}

// buildProxyTLSConfig builds the *tls.Config used for the SEPARATE,
// proxy-facing TLS handshake (Tier 2: the proxy itself requires its own
// client certificate, distinct from the origin's).
func buildProxyTLSConfig(cfg ProxyTLSConfig) (*tls.Config, error) {
	if err := validateVerificationConfig(cfg.InsecureSkipVerify, cfg.InsecureSkipVerifyAcknowledged, false); err != nil {
		return nil, err
	}

	tlsCfg := &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}

	if cfg.RootCAs != nil {
		tlsCfg.RootCAs = cfg.RootCAs
	} else if cfg.RootCAFile != "" {
		pool, err := loadCertPool(cfg.RootCAFile)
		if err != nil {
			return nil, err
		}
		tlsCfg.RootCAs = pool
	}

	switch {
	case cfg.GetClientCertificate != nil:
		tlsCfg.GetClientCertificate = cfg.GetClientCertificate
	case cfg.ClientCertFile != "" || cfg.ClientKeyFile != "":
		if cfg.ClientCertFile == "" || cfg.ClientKeyFile == "" {
			return nil, fmt.Errorf("httpclient: Proxy.ProxyTLS.ClientCertFile and ClientKeyFile must both be set for proxy mTLS")
		}
		cert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("httpclient: failed to load proxy client certificate")
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}

// validateVerificationConfig enforces the hostname-verification invariants
// shared by both TLSConfig and ProxyTLSConfig: InsecureSkipVerify requires
// explicit acknowledgement, and can never be combined with a custom verify
// callback (which would then silently become the only check performed,
// since its verifiedChains argument is empty when default verification
// didn't run).
func validateVerificationConfig(insecureSkipVerify, acknowledged, hasCustomVerify bool) error {
	if !insecureSkipVerify {
		return nil
	}
	if !acknowledged {
		return fmt.Errorf("httpclient: InsecureSkipVerify requires InsecureSkipVerifyAcknowledged to also be set")
	}
	if hasCustomVerify {
		return fmt.Errorf("httpclient: InsecureSkipVerify cannot be combined with VerifyPeerCertificate/VerifyConnection — a custom callback would become the only check performed")
	}
	return nil
}

// loadCertPool reads a PEM-encoded CA bundle from disk into an
// *x509.CertPool.
func loadCertPool(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("httpclient: failed to read CA bundle")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("httpclient: CA bundle contains no usable certificates")
	}
	return pool, nil
}

// cloneTLSConfigForHost returns a shallow clone of base with ServerName set
// to host. Every hand-rolled tls.Client/Handshake call in this package
// (the Tier-2 proxy dialer, the manual-CONNECT round tripper) must route
// through this helper rather than setting ServerName ad hoc or leaving it
// empty — the intended hostname must always come from the original request,
// never from a dialed IP address.
func cloneTLSConfigForHost(base *tls.Config, host string) *tls.Config {
	cfg := base.Clone()
	cfg.ServerName = host
	return cfg
}

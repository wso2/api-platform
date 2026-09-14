/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package proxy

import (
	"crypto/x509"
	"fmt"
	"net/http"
	"os"

	"github.com/wso2/api-platform/httpkit/httpclient"

	"ai-workspace-bff/internal/config"
)

// TLSClientOptions configures how the upstream (Platform API) certificate is
// trusted. The two are mutually exclusive in effect: when SkipVerify is true no
// verification happens and CAFile is ignored.
type TLSClientOptions struct {
	// CAFile is a PEM bundle appended to the system roots so a private or
	// self-signed upstream cert can be trusted with verification still on.
	CAFile string
	// SkipVerify disables certificate verification entirely (dev/demo only).
	SkipVerify bool
}

// NewTransport builds an http.RoundTripper for upstream calls with explicit
// timeouts and connection pooling, via the shared httpkit/httpclient builder.
// hc supplies every knob sourced from [ai_workspace.http_client] in config.toml
// (see config.HTTPClientConfig's doc comment); opts supplies the upstream TLS
// trust settings, sourced from [ai_workspace.control_plane] instead — kept as a
// separate parameter because that trust decision already has its own existing
// config keys (ControlPlaneConfig.CAFile/TLSSkipVerify) which must stay the
// single source of truth, never duplicated onto HTTPClientConfig. TLS applies
// only when the upstream URL is https:// — this transport is scheme-agnostic
// and does nothing for http://.
//
// This transport only ever talks to the fixed, operator-configured Platform
// API (cfg.ControlPlane.URL) — never a tenant/end-user-supplied destination —
// so no SSRF guard (httpclient's Config.SSRF) is enabled here.
//
// The return type is the interface, not the concrete *http.Transport:
// hc.Timeouts.MaxResponseBytes >= 0 (see HTTPClientTimeoutsConfig's doc
// comment) makes httpclient.New wrap the transport in its own
// maxBytesRoundTripper, which every caller here (http.Client.Transport,
// proxy.ReverseProxy) already accepts as an http.RoundTripper.
func NewTransport(hc config.HTTPClientConfig, opts TLSClientOptions) (http.RoundTripper, error) {
	cfg := httpclient.DefaultConfig()

	cfg.Pooling.MaxIdleConns = hc.Pooling.MaxIdleConns
	cfg.Pooling.MaxIdleConnsPerHost = hc.Pooling.MaxIdleConnsPerHost
	cfg.Pooling.MaxConnsPerHost = hc.Pooling.MaxConnsPerHost
	cfg.Pooling.IdleConnTimeout = hc.Pooling.IdleConnTimeout
	cfg.Pooling.KeepAlive = hc.Pooling.KeepAlive
	cfg.Pooling.DisableKeepAlives = hc.Pooling.DisableKeepAlives
	cfg.Pooling.EnableHTTP2 = hc.Pooling.EnableHTTP2

	cfg.Timeouts.Overall = hc.Timeouts.Overall
	cfg.Timeouts.Dial = hc.Timeouts.Dial
	cfg.Timeouts.TLSHandshake = hc.Timeouts.TLSHandshake
	cfg.Timeouts.ResponseHeader = hc.Timeouts.ResponseHeader
	cfg.Timeouts.ExpectContinue = hc.Timeouts.ExpectContinue
	// The default (see config.defaultConfig) is -1: this transport backs a reverse
	// proxy that streams SSE/long-running LLM output (see ReverseProxy's
	// FlushInterval) to/from the BFF's own fixed, trusted backend — not an
	// arbitrary or tenant-supplied target — so disabling httpclient's default
	// 10MiB cap is the documented default for exactly this case. An operator can
	// still opt into a cap via config; see the type-assertion note below.
	cfg.Timeouts.MaxResponseBytes = hc.Timeouts.MaxResponseBytes

	cfg.TLS.MinVersion = hc.TLS.MinVersion
	cfg.TLS.MaxVersion = hc.TLS.MaxVersion
	cfg.TLS.CipherSuites = hc.TLS.CipherSuites
	cfg.TLS.CurvePreferences = hc.TLS.CurvePreferences

	// #nosec G402 — SkipVerify is an explicit, demo-gated escape hatch
	// (validated in config); the secure default is false.
	cfg.TLS.InsecureSkipVerify = opts.SkipVerify
	if opts.SkipVerify {
		// Required by httpclient.New alongside InsecureSkipVerify=true: this
		// toggle is already an explicit, operator-controlled config option
		// (see TLSClientOptions.SkipVerify), so acknowledging it here
		// preserves behavior without weakening httpclient's safety gate.
		cfg.TLS.InsecureSkipVerifyAcknowledged = true
	} else if opts.CAFile != "" {
		// Built via caPool (system roots + this bundle appended) rather than
		// cfg.TLS.RootCAFile, which would replace the trust store outright —
		// this preserves the original "PEM bundle appended to the system
		// roots" behavior documented on CAFile above.
		pool, err := caPool(opts.CAFile)
		if err != nil {
			return nil, err
		}
		cfg.TLS.RootCAs = pool
	}

	switch hc.Proxy.Mode {
	case "", "none":
		// no proxy — cfg.Proxy stays at its zero value
	case "environment":
		cfg.Proxy.Mode = "environment"
	case "url":
		cfg.Proxy.Mode = "url"
		cfg.Proxy.URL = hc.Proxy.URL
		cfg.Proxy.Username = hc.Proxy.Username
		cfg.Proxy.Password = hc.Proxy.Password
		cfg.Proxy.NoProxy = hc.Proxy.NoProxy
		if hc.Proxy.TLS != (config.HTTPClientProxyTLSConfig{}) {
			cfg.Proxy.ProxyTLS = &httpclient.ProxyTLSConfig{
				RootCAFile:                     hc.Proxy.TLS.RootCAFile,
				ClientCertFile:                 hc.Proxy.TLS.ClientCertFile,
				ClientKeyFile:                  hc.Proxy.TLS.ClientKeyFile,
				InsecureSkipVerify:             hc.Proxy.TLS.InsecureSkipVerify,
				InsecureSkipVerifyAcknowledged: hc.Proxy.TLS.InsecureSkipVerify,
			}
		}
	default:
		return nil, fmt.Errorf("ai_workspace.http_client.proxy.mode: unrecognized value %q (want \"none\", \"environment\", or \"url\")", hc.Proxy.Mode)
	}

	client, err := httpclient.New(cfg)
	if err != nil {
		return nil, err
	}
	return client.Transport, nil
}

// caPool returns the system root pool with the PEM bundle at path appended, so
// public CAs keep working alongside a private/self-signed upstream cert.
func caPool(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read control_plane_ca_file %q: %w", path, err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no valid certificates in control_plane_ca_file %q", path)
	}
	return pool, nil
}

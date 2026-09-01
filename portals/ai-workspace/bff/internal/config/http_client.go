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

package config

import "time"

// HTTPClientConfig is [ai_workspace.http_client]: configures the single outbound
// *http.Transport this BFF uses for every call to its upstream Platform API (see
// proxy.NewTransport, and server.New's one call site). It mirrors
// github.com/wso2/api-platform/httpkit/httpclient.Config field-for-field (see that package's own
// doc comments for full semantics) — the same shape gateway-controller's and
// platform-api's own HTTPClientConfig use (gateway/gateway-controller/pkg/config/config.go,
// platform-api/config/config.go) — so every knob the library exposes that has a natural
// TOML shape is operator-configurable here rather than hardcoded in transport.go.
//
// SSRF is deliberately not represented here (unlike platform-api's HTTPClientConfig):
// this transport only ever talks to the fixed, operator-configured Platform API
// (ControlPlaneConfig.URL) — never a tenant/end-user-supplied destination — so there is
// nothing for httpclient's SSRF guard to protect against. See transport.go's own doc
// comment.
//
// TLS.RootCAFile/ClientCertFile/ClientKeyFile/InsecureSkipVerify are also deliberately
// not fields here: this component already has an existing setting for that same trust
// decision — ControlPlaneConfig.CAFile / ControlPlaneConfig.TLSSkipVerify, wired through
// proxy.TLSClientOptions — and duplicating it here would create two settings that must
// always be kept in sync.
type HTTPClientConfig struct {
	Pooling  HTTPClientPoolingConfig  `koanf:"pooling"`
	Timeouts HTTPClientTimeoutsConfig `koanf:"timeouts"`
	TLS      HTTPClientTLSConfig      `koanf:"tls"`
	Proxy    HTTPClientProxyConfig    `koanf:"proxy"`
}

// HTTPClientPoolingConfig mirrors httpclient.PoolingConfig.
type HTTPClientPoolingConfig struct {
	MaxIdleConns        int           `koanf:"max_idle_conns"`
	MaxIdleConnsPerHost int           `koanf:"max_idle_conns_per_host"`
	MaxConnsPerHost     int           `koanf:"max_conns_per_host"`
	IdleConnTimeout     time.Duration `koanf:"idle_conn_timeout"`
	KeepAlive           time.Duration `koanf:"keep_alive"`
	DisableKeepAlives   bool          `koanf:"disable_keep_alives"`
	// EnableHTTP2 opts into HTTP/2. See httpclient.PoolingConfig.EnableHTTP2's doc
	// comment on the HTTP/2 connection-coalescing caveat before disabling — true by
	// default here, matching this transport's previous hardcoded
	// ForceAttemptHTTP2: true behavior.
	EnableHTTP2 bool `koanf:"enable_http2"`
}

// HTTPClientTimeoutsConfig mirrors httpclient.TimeoutsConfig.
//
// Overall has no observable effect at this call site today: NewTransport extracts only
// the *http.Transport out of httpclient.New's *http.Client (see transport.go), and
// server.New applies its own, separate 60s http.Client.Timeout on top of it. Kept here
// anyway, defaulted to httpclient.DefaultConfig()'s own value, for shape parity with
// gateway-controller/platform-api and in case a future caller reads the full
// *http.Client instead.
type HTTPClientTimeoutsConfig struct {
	Overall        time.Duration `koanf:"overall"`
	Dial           time.Duration `koanf:"dial"`
	TLSHandshake   time.Duration `koanf:"tls_handshake"`
	ResponseHeader time.Duration `koanf:"response_header"`
	ExpectContinue time.Duration `koanf:"expect_continue"`
	// MaxResponseBytes bounds the response body read through this transport. Defaults
	// to -1 (disabled): this transport backs a reverse proxy streaming SSE/long-running
	// LLM output between the BFF and its own fixed, trusted Platform API — not an
	// arbitrary or tenant-supplied target — so truncating a legitimate long stream would
	// be worse than not bounding it. 0 = httpclient's own package default (10MiB); a
	// positive value applies that exact cap instead.
	MaxResponseBytes int64 `koanf:"max_response_bytes"`
}

// HTTPClientTLSConfig mirrors the TOML-expressible subset of httpclient.TLSConfig that
// is not already sourced from ControlPlaneConfig (see HTTPClientConfig's doc comment).
// MinVersion/MaxVersion both empty (the default) preserves today's behavior of "no
// override, use Go's own crypto/tls default" (currently a TLS 1.2 floor with no
// configured ceiling).
type HTTPClientTLSConfig struct {
	MinVersion string `koanf:"min_version"` // one of "TLS1_0".."TLS1_3"
	MaxVersion string `koanf:"max_version"` // one of "TLS1_0".."TLS1_3"
	// CipherSuites is a comma-separated list of Go crypto/tls cipher suite names.
	// Empty uses Go's own default secure set. Only affects TLS 1.2 and below.
	CipherSuites string `koanf:"cipher_suites"`
	// CurvePreferences is a comma-separated, order-significant list of curve/group
	// names, e.g. "X25519MLKEM768,X25519,P-256" to opt into the FIPS 203 ML-KEM-768
	// hybrid group while retaining classical fallbacks for a Platform API build that
	// doesn't support it yet. Empty (the default) uses Go's own defaults (no PQC).
	CurvePreferences string `koanf:"curve_preferences"`
}

// HTTPClientProxyConfig mirrors the TOML-expressible subset of httpclient.ProxyConfig.
// Egress is deliberately not a field here: httpclient.New only requires it when SSRF is
// also enabled, and this component has no SSRF field at all (see HTTPClientConfig's doc
// comment), so it would otherwise sit unused.
type HTTPClientProxyConfig struct {
	// Mode selects how the proxy is determined: "none", "environment"
	// (HTTP_PROXY/HTTPS_PROXY/NO_PROXY — matches this transport's previous hardcoded
	// http.ProxyFromEnvironment behavior, and remains the default here), or "url" (URL/
	// Username/Password/NoProxy below).
	Mode     string   `koanf:"mode"`
	URL      string   `koanf:"url"`
	Username string   `koanf:"username"`
	Password string   `koanf:"password"`
	NoProxy  []string `koanf:"no_proxy"` // exact host, ".suffix", or CIDR entries; only used when mode == "url"

	TLS HTTPClientProxyTLSConfig `koanf:"tls"`
}

// HTTPClientProxyTLSConfig mirrors httpclient.ProxyTLSConfig (the proxy's own TLS
// handshake, fully decoupled from the origin TLS handshake in HTTPClientTLSConfig).
type HTTPClientProxyTLSConfig struct {
	RootCAFile         string `koanf:"root_ca_file"`
	ClientCertFile     string `koanf:"client_cert_file"`
	ClientKeyFile      string `koanf:"client_key_file"`
	InsecureSkipVerify bool   `koanf:"insecure_skip_verify"`
}

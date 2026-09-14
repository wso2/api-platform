/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"fmt"

	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"
)

// BuildHTTPClientConfig translates HTTPClientConfig (sourced from the [http_client]
// section in config.toml) into an httpkit httpclient.Config for the WebSub Verifier and
// Deliverer HTTP clients (see HTTPClientConfig's doc comment for the shared-config
// rationale). The caller is expected to override the returned Timeouts.Overall with its
// own existing per-call-site timeout (Verifier's `timeout` argument / Deliverer's own
// delivery timeout) — this function only fills in the common default.
//
// SSRF guarding is unconditionally enabled here with netguard.PermitPrivateBlockMetadata()
// — every caller of this shared config dials a tenant/subscriber-supplied CallbackURL, so
// unlike gateway-controller's analogous translation there is no Enabled/Preset switch to
// interpret; only the redirect/scheme knobs in HTTPClientSSRFConfig are read from config.
// PermitPrivateBlockMetadata (not PublicOnly) is deliberate here, not an oversight: WebSub
// subscribers routinely live on private networks by design (a Kubernetes ClusterIP, a
// service-DNS name resolving into RFC 1918 space, a docker-compose service, a localhost
// port during development — see the preset's own doc comment) — blocking RFC 1918/loopback
// would break that core, intended deployment shape. What must still be refused is a
// destination that is never a legitimate subscriber upstream: link-local (which is where
// the cloud metadata endpoint 169.254.169.254 lives), the unspecified address, and
// multicast/broadcast — exactly what this preset blocks.
func BuildHTTPClientConfig(hc HTTPClientConfig) (httpclient.Config, error) {
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
	// A negative value would disable httpkit's response-size bound entirely
	// (see httpclient.TimeoutsConfig.MaxResponseBytes) — never acceptable
	// here, since every response body is read from a tenant/subscriber-
	// supplied CallbackURL. Reject it rather than forwarding it; 0 still
	// selects httpkit's own finite default (10MiB).
	if hc.Timeouts.MaxResponseBytes < 0 {
		return httpclient.Config{}, fmt.Errorf("http_client.timeouts.max_response_bytes must not be negative (a finite maximum is required for tenant-supplied callback responses)")
	}
	cfg.Timeouts.MaxResponseBytes = hc.Timeouts.MaxResponseBytes

	cfg.TLS.MinVersion = hc.TLS.MinVersion
	cfg.TLS.MaxVersion = hc.TLS.MaxVersion
	cfg.TLS.CipherSuites = hc.TLS.CipherSuites
	cfg.TLS.CurvePreferences = hc.TLS.CurvePreferences
	cfg.TLS.RootCAFile = hc.TLS.RootCAFile
	cfg.TLS.ClientCertFile = hc.TLS.ClientCertFile
	cfg.TLS.ClientKeyFile = hc.TLS.ClientKeyFile
	// InsecureSkipVerify is intentionally not exposed here: unlike gateway-controller
	// there is no existing single source-of-truth boolean for it to reuse, and these
	// clients dial arbitrary tenant-supplied CallbackURLs, so it stays at its safe
	// default (verified) rather than becoming a per-client opt-out.

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
		if hc.Proxy.TLS != (HTTPClientProxyTLSConfig{}) {
			cfg.Proxy.ProxyTLS = &httpclient.ProxyTLSConfig{
				RootCAFile:                     hc.Proxy.TLS.RootCAFile,
				ClientCertFile:                 hc.Proxy.TLS.ClientCertFile,
				ClientKeyFile:                  hc.Proxy.TLS.ClientKeyFile,
				InsecureSkipVerify:             hc.Proxy.TLS.InsecureSkipVerify,
				InsecureSkipVerifyAcknowledged: hc.Proxy.TLS.InsecureSkipVerifyAcknowledged,
			}
		}
	default:
		return httpclient.Config{}, fmt.Errorf("http_client.proxy.mode: unrecognized value %q (want \"none\", \"environment\", or \"url\")", hc.Proxy.Mode)
	}

	// Always on — see this function's doc comment and HTTPClientConfig's doc comment for
	// why PermitPrivateBlockMetadata (not PublicOnly) is the deliberate choice here.
	cfg.SSRF.Enabled = true
	cfg.SSRF.Policy = netguard.PermitPrivateBlockMetadata()
	cfg.SSRF.Policy.AllowedSchemes = hc.SSRF.AllowedSchemes
	cfg.SSRF.MaxRedirects = hc.SSRF.MaxRedirects

	if cfg.Proxy.Mode != "" && cfg.Proxy.Mode != "none" {
		switch hc.Proxy.Egress {
		case "delegated":
			cfg.Proxy.Egress = httpclient.ProxyEgressDelegated
		case "manual_connect":
			cfg.Proxy.Egress = httpclient.ProxyEgressManualCONNECT
		default:
			return httpclient.Config{}, fmt.Errorf("http_client.proxy.egress must be \"delegated\" or \"manual_connect\" when http_client.proxy.mode is set (SSRF guarding is always enabled for this client), got %q", hc.Proxy.Egress)
		}
	}

	return cfg, nil
}

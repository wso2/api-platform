/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package server

import (
	"fmt"

	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// InitSharedHTTPClient builds the single shared outbound *http.Client used by every
// SSRF-guarded call this process makes (MCP reachability/JSON-RPC calls, OpenAPI spec fetch
// by URL, JWKS fetch, the API Portal publisher) and hands it to the utils package. Every
// entry point (cmd/main.go and platform.New) calls it before StartPlatformAPIServer.
func InitSharedHTTPClient(cfg *config.Server) error {
	clientCfg, err := buildSharedHTTPClientConfig(cfg.HTTPClient)
	if err != nil {
		return fmt.Errorf("invalid platform_api.http_client configuration: %w", err)
	}
	client, err := httpclient.New(clientCfg)
	if err != nil {
		return fmt.Errorf("failed to build shared outbound HTTP client: %w", err)
	}
	utils.InitSharedHTTPClient(client, cfg.MCPResponseMaxBytes)
	return nil
}

// buildSharedHTTPClientConfig translates platform-api's HTTPClientConfig (sourced from
// platform_api.http_client in config.toml) into an httpkit httpclient.Config, mirroring
// every field the library exposes that has a natural TOML shape. Mirrors
// gateway-controller's own buildSharedHTTPClientConfig
// (gateway/gateway-controller/cmd/controller/main.go) field-for-field; unlike that one,
// TLS.InsecureSkipVerify is sourced directly from hc.TLS.InsecureSkipVerify rather than a
// separate existing setting, since platform-api has no equivalent to reuse (see
// config.HTTPClientConfig's doc comment).
func buildSharedHTTPClientConfig(hc config.HTTPClientConfig) (httpclient.Config, error) {
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
	cfg.Timeouts.MaxResponseBytes = hc.Timeouts.MaxResponseBytes

	cfg.TLS.MinVersion = hc.TLS.MinVersion
	cfg.TLS.MaxVersion = hc.TLS.MaxVersion
	cfg.TLS.CipherSuites = hc.TLS.CipherSuites
	cfg.TLS.CurvePreferences = hc.TLS.CurvePreferences
	cfg.TLS.RootCAFile = hc.TLS.RootCAFile
	cfg.TLS.ClientCertFile = hc.TLS.ClientCertFile
	cfg.TLS.ClientKeyFile = hc.TLS.ClientKeyFile
	cfg.TLS.InsecureSkipVerify = hc.TLS.InsecureSkipVerify             // #nosec G402 -- explicit operator-controlled opt-out for dev/test environments.
	cfg.TLS.InsecureSkipVerifyAcknowledged = hc.TLS.InsecureSkipVerify // required double-gate; mirrors InsecureSkipVerify, harmless when false.

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
		return httpclient.Config{}, fmt.Errorf("platform_api.http_client.proxy.mode: unrecognized value %q (want \"none\", \"environment\", or \"url\")", hc.Proxy.Mode)
	}

	if hc.SSRF.Enabled {
		cfg.SSRF.Enabled = true
		switch hc.SSRF.Preset {
		case "permit_private_block_metadata":
			cfg.SSRF.Policy = netguard.PermitPrivateBlockMetadata()
		case "public_only":
			cfg.SSRF.Policy = netguard.PublicOnly()
		default:
			return httpclient.Config{}, fmt.Errorf("platform_api.http_client.ssrf.preset: unrecognized value %q (want \"permit_private_block_metadata\" or \"public_only\")", hc.SSRF.Preset)
		}
		cfg.SSRF.Policy.AllowedSchemes = hc.SSRF.AllowedSchemes
		cfg.SSRF.MaxRedirects = hc.SSRF.MaxRedirects

		if cfg.Proxy.Mode != "" && cfg.Proxy.Mode != "none" {
			switch hc.Proxy.Egress {
			case "delegated":
				cfg.Proxy.Egress = httpclient.ProxyEgressDelegated
			case "manual_connect":
				cfg.Proxy.Egress = httpclient.ProxyEgressManualCONNECT
			default:
				return httpclient.Config{}, fmt.Errorf("platform_api.http_client.proxy.egress must be \"delegated\" or \"manual_connect\" when both proxy and SSRF are enabled, got %q", hc.Proxy.Egress)
			}
		}
	}

	return cfg, nil
}

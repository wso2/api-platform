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

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/builtins"
	"github.com/wso2/api-platform/platform-api/internal/logger"
	"github.com/wso2/api-platform/platform-api/internal/server"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"
)

// stringSliceFlag collects a repeatable string flag into a slice, preserving the
// order in which the flags were supplied on the command line.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ", ") }

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	// -config is repeatable: files are merged in the order given with last-wins
	// precedence. Env values reach config only through explicit {{ env }}
	// interpolation tokens in the file(s) — there is no independent env provider —
	// so at least one config file is required and there is no default path.
	var configFiles stringSliceFlag
	flag.Var(&configFiles, "config",
		"Path to a configuration file (required; repeatable, merged in order with last-wins precedence)")
	flag.Parse()

	if len(configFiles) == 0 {
		fmt.Fprintf(os.Stderr, "Error: -config flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -config <path-to-config.toml> [-config <overlay.toml> ...]\n", os.Args[0])
		os.Exit(1)
	}

	config.SetConfigPaths(configFiles...)

	cfg := config.GetConfig()

	// Initialize logger
	logConfig := logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	}
	slogger := logger.NewLogger(logConfig)

	// Build the single shared outbound *http.Client used by every SSRF-guarded call this
	// process makes (MCP reachability/JSON-RPC calls, OpenAPI spec fetch by URL) and hand it
	// to the utils package before any server/handler wiring that could reach those code
	// paths. This must run before server.StartPlatformAPIServer.
	sharedHTTPClientCfg, err := buildSharedHTTPClientConfig(cfg.HTTPClient)
	if err != nil {
		slogger.Error("Invalid platform_api.http_client configuration", "error", err)
		os.Exit(1)
	}
	sharedHTTPClient, err := httpclient.New(sharedHTTPClientCfg)
	if err != nil {
		slogger.Error("Failed to build shared outbound HTTP client", "error", err)
		os.Exit(1)
	}
	utils.InitSharedHTTPClient(sharedHTTPClient, cfg.MCPResponseMaxBytes)

	slogger.Info("Initializing Platform API server...")
	// Built-in (internal-tier) plugins are supplied here; the OSS entry point runs
	// no external-tier (pdk) plugins — those come from wrapper modules via the
	// platform façade. builtins.Plugins() is build-tag selected in its own package
	// so this entry point stays buildable as a single file (`go build ./cmd/main.go`).
	srv, err := server.StartPlatformAPIServer(cfg, slogger, builtins.Plugins(), nil)
	if err != nil {
		slogger.Error("Failed to create server", "error", err)
		os.Exit(1)
	}

	slogger.Info("Starting server",
		"http_enabled", cfg.Listeners.HTTP.Enabled, "http_port", cfg.Listeners.HTTP.Port,
		"https_enabled", cfg.Listeners.HTTPS.Enabled, "https_port", cfg.Listeners.HTTPS.Port)
	if err := srv.Start(cfg.Listeners, cfg.Listeners.Timeouts); err != nil {
		slogger.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
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

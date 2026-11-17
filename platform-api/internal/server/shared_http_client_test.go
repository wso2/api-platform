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
	"reflect"
	"testing"

	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

func resetSharedHTTPClient(t *testing.T) {
	t.Helper()
	utils.InitSharedHTTPClient(nil, 0)
	t.Cleanup(func() { utils.InitSharedHTTPClient(nil, 0) })
}

func TestInitSharedHTTPClientMakesClientAvailable(t *testing.T) {
	resetSharedHTTPClient(t)
	cfg := &config.Server{}
	cfg.HTTPClient.DestinationGuard = config.HTTPClientDestinationGuardConfig{
		Enabled:        true,
		Preset:         "permit_private_block_metadata",
		AllowedSchemes: []string{"https"},
		MaxRedirects:   3,
	}

	if err := InitSharedHTTPClient(cfg); err != nil {
		t.Fatalf("InitSharedHTTPClient: %v", err)
	}
	client, err := utils.NewUpstreamFetchClient(0)
	if err != nil || client == nil {
		t.Fatalf("want the shared client after init, got %v, %v", client, err)
	}
}

func TestInitSharedHTTPClientRejectsInvalidConfig(t *testing.T) {
	cases := map[string]config.HTTPClientConfig{
		"unknown proxy mode":               {Proxy: config.HTTPClientProxyConfig{Mode: "socks"}},
		"unknown destination guard preset": {DestinationGuard: config.HTTPClientDestinationGuardConfig{Enabled: true, Preset: "everything"}},
	}
	for name, hc := range cases {
		t.Run(name, func(t *testing.T) {
			resetSharedHTTPClient(t)
			if err := InitSharedHTTPClient(&config.Server{HTTPClient: hc}); err == nil {
				t.Fatal("want an error for the invalid configuration")
			}
			if _, err := utils.NewUpstreamFetchClient(0); err == nil {
				t.Fatal("want no shared client after a failed init")
			}
		})
	}
}

func TestBuildAccessControlClientConfig(t *testing.T) {
	enabled := config.AccessControl{Enabled: true, Mode: "deny", MaxRedirects: 5, AllowedSchemes: []string{"http", "https"}}

	t.Run("disabled keeps the shared client", func(t *testing.T) {
		_, on, err := buildAccessControlClientConfig(httpclient.DefaultConfig(), config.HTTPClientConfig{}, config.AccessControl{})
		if err != nil || on {
			t.Fatalf("got enabled=%v err=%v, want disabled and no error", on, err)
		}
	})

	t.Run("a legacy public_only preset can only tighten", func(t *testing.T) {
		shared := httpclient.DefaultConfig()
		shared.DestinationGuard = httpclient.DestinationGuardConfig{Enabled: true, Policy: netguard.PublicOnly()}
		out, on, err := buildAccessControlClientConfig(shared, config.HTTPClientConfig{}, enabled)
		if err != nil || !on {
			t.Fatalf("got enabled=%v err=%v", on, err)
		}
		if !out.DestinationGuard.Policy.BlockPrivate || !out.DestinationGuard.Policy.BlockLoopback {
			t.Fatal("deny_private_networks=false must not re-open private addresses a public_only preset refused")
		}
		if out.DestinationGuard.MaxRedirects != 5 {
			t.Fatalf("MaxRedirects = %d, want access control's 5", out.DestinationGuard.MaxRedirects)
		}
	})

	t.Run("a proxy without an egress choice defaults to delegated", func(t *testing.T) {
		shared := httpclient.DefaultConfig()
		shared.Proxy.Mode = "url"
		shared.Proxy.URL = "http://proxy.internal:3128"
		out, _, err := buildAccessControlClientConfig(shared, config.HTTPClientConfig{}, enabled)
		if err != nil {
			t.Fatalf("buildAccessControlClientConfig: %v", err)
		}
		if out.Proxy.Egress != httpclient.ProxyEgressDelegated {
			t.Fatalf("Egress = %v, want delegated", out.Proxy.Egress)
		}
		if _, err := httpclient.New(out); err != nil {
			t.Fatalf("the derived configuration must build: %v", err)
		}
	})

	t.Run("the existing preset's redirect schemes narrow redirects only", func(t *testing.T) {
		cases := []struct {
			legacy []string
			want   []string
		}{
			{nil, []string{"https"}}, // an empty list means https-only redirects
			{[]string{"http", "https"}, []string{"http", "https"}},
			{[]string{"https"}, []string{"https"}},
		}
		for _, tc := range cases {
			shared := httpclient.DefaultConfig()
			shared.DestinationGuard = httpclient.DestinationGuardConfig{Enabled: true, Policy: netguard.PermitPrivateBlockMetadata()}
			shared.DestinationGuard.Policy.AllowedSchemes = tc.legacy
			out, _, err := buildAccessControlClientConfig(shared, config.HTTPClientConfig{}, enabled)
			if err != nil {
				t.Fatalf("legacy %v: %v", tc.legacy, err)
			}
			if !reflect.DeepEqual(out.DestinationGuard.RedirectSchemes, tc.want) {
				t.Fatalf("legacy %v: RedirectSchemes = %v, want %v", tc.legacy, out.DestinationGuard.RedirectSchemes, tc.want)
			}
			if !reflect.DeepEqual(out.DestinationGuard.Policy.AllowedSchemes, []string{"http", "https"}) {
				t.Fatalf("legacy %v: first-request schemes = %v, want access control's http and https", tc.legacy, out.DestinationGuard.Policy.AllowedSchemes)
			}
		}
	})

	t.Run("no common redirect scheme is a configuration error", func(t *testing.T) {
		shared := httpclient.DefaultConfig()
		shared.DestinationGuard = httpclient.DestinationGuardConfig{Enabled: true, Policy: netguard.PermitPrivateBlockMetadata()}
		shared.DestinationGuard.Policy.AllowedSchemes = []string{"https"}
		httpOnly := enabled
		httpOnly.AllowedSchemes = []string{"http"}
		if _, _, err := buildAccessControlClientConfig(shared, config.HTTPClientConfig{}, httpOnly); err == nil {
			t.Fatal("want an error when no redirect scheme survives")
		}
	})

	t.Run("an invalid pattern is a configuration error", func(t *testing.T) {
		bad := enabled
		bad.Hostnames = []string{"api.(example"}
		if _, _, err := buildAccessControlClientConfig(httpclient.DefaultConfig(), config.HTTPClientConfig{}, bad); err == nil {
			t.Fatal("want an error for an invalid hostname pattern")
		}
	})
}

func TestInitSharedHTTPClientKeepsConfiguredDestinationsOffAccessControl(t *testing.T) {
	resetSharedHTTPClient(t)
	cfg := &config.Server{}
	cfg.HTTPClient.DestinationGuard = config.HTTPClientDestinationGuardConfig{Enabled: true, Preset: "permit_private_block_metadata", AllowedSchemes: []string{"http", "https"}}
	cfg.Listeners.NetworkSecurity.AccessControl = config.AccessControl{Enabled: true, Mode: "allow", Hostnames: []string{`api\.example\.com`}}

	if err := InitSharedHTTPClient(cfg); err != nil {
		t.Fatalf("InitSharedHTTPClient: %v", err)
	}
	tenant, err := utils.NewUpstreamFetchClient(0)
	if err != nil {
		t.Fatalf("NewUpstreamFetchClient: %v", err)
	}
	configured, err := utils.NewConfiguredDestinationClient()
	if err != nil {
		t.Fatalf("NewConfiguredDestinationClient: %v", err)
	}
	if tenant == configured {
		t.Fatal("tenant URLs and config.toml destinations must use different clients when access control is on")
	}
}

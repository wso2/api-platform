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
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkutils "github.com/wso2/api-platform/sdk/core/utils"
)

func userURLFactory(t *testing.T, ac AccessControlConfig) (sdkutils.UserURLClientFactory, sdkutils.UserURLChecker) {
	t.Helper()
	cfg, err := BuildUserURLClientConfig(defaultConfig().PolicyEngine.HTTPClient, ac)
	if err != nil {
		t.Fatalf("BuildUserURLClientConfig: %v", err)
	}
	return NewUserURLClientFactory(cfg)
}

func TestUserURLClient_RefusalMatchesSDKSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	factory, checker := userURLFactory(t, AccessControlConfig{Enabled: true, DenyPrivateNetworks: true})
	client, err := factory(sdkutils.UserURLClientOptions{})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if _, err := client.Get(srv.URL); !errors.Is(err, sdkutils.ErrDestinationNotAllowed) {
		t.Fatalf("loopback with private networks denied = %v, want ErrDestinationNotAllowed", err)
	}
	if err := checker(context.Background(), "http://169.254.169.254/"); !errors.Is(err, sdkutils.ErrDestinationNotAllowed) {
		t.Fatalf("checker(metadata) = %v, want ErrDestinationNotAllowed", err)
	}

	// Positive control: the default posture reaches a private backend.
	permitFactory, _ := userURLFactory(t, AccessControlConfig{Enabled: true})
	permit, err := permitFactory(sdkutils.UserURLClientOptions{})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	resp, err := permit.Get(srv.URL)
	if err != nil {
		t.Fatalf("private backend with private networks permitted: %v", err)
	}
	resp.Body.Close()
}

func TestUserURLClient_HostRuleInAllowMode(t *testing.T) {
	factory, checker := userURLFactory(t, AccessControlConfig{Enabled: true, Mode: "allow", Hostnames: []string{`opa\.example\.com`}})
	client, err := factory(sdkutils.UserURLClientOptions{})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if _, err := client.Get("http://127.0.0.1:1/"); !errors.Is(err, sdkutils.ErrDestinationNotAllowed) {
		t.Fatalf("unlisted host = %v, want ErrDestinationNotAllowed", err)
	}
	if err := checker(context.Background(), "https://evil.example.com/"); !errors.Is(err, sdkutils.ErrDestinationNotAllowed) {
		t.Fatalf("checker(unlisted) = %v, want ErrDestinationNotAllowed", err)
	}
}

func TestUserURLClient_RedirectsAreNotFollowedByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/next", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	factory, _ := userURLFactory(t, AccessControlConfig{Enabled: true})
	noFollow, _ := factory(sdkutils.UserURLClientOptions{})
	resp, err := noFollow.Get(srv.URL + "/start")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want the 302 returned as is", resp.StatusCode)
	}

	follow, _ := factory(sdkutils.UserURLClientOptions{FollowRedirects: true})
	resp, err = follow.Get(srv.URL + "/start")
	if err != nil {
		t.Fatalf("Get following redirects: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 after a same-host redirect", resp.StatusCode)
	}
}

func TestBuildUserURLClientConfig_LegacyPresetOnlyTightens(t *testing.T) {
	hc := defaultConfig().PolicyEngine.HTTPClient
	hc.DestinationGuard.Enabled = true
	hc.DestinationGuard.Preset = "public_only"
	cfg, err := BuildUserURLClientConfig(hc, AccessControlConfig{Enabled: true})
	if err != nil {
		t.Fatalf("BuildUserURLClientConfig: %v", err)
	}
	if !cfg.DestinationGuard.Policy.BlockPrivate {
		t.Fatal("deny_private_networks=false must not re-open what a public_only preset refused")
	}

	off, err := BuildUserURLClientConfig(defaultConfig().PolicyEngine.HTTPClient, AccessControlConfig{Enabled: false})
	if err != nil {
		t.Fatalf("BuildUserURLClientConfig: %v", err)
	}
	if off.DestinationGuard.Enabled {
		t.Fatal("with access control off the shared [policy_engine.http_client] behaviour is kept")
	}
}

func TestValidate_RejectsInvalidAccessControl(t *testing.T) {
	cfg := defaultConfig()
	cfg.Controller.Server.NetworkSecurity.AccessControl = AccessControlConfig{Enabled: true, Mode: "allow"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("allow mode with no hostnames must fail validation")
	}
}

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
package devportal

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/test/testutil"
	"github.com/wso2/api-platform/cli/utils"
)

func TestRunAddCommand_AddsDevPortalToDefaultPlatform(t *testing.T) {
	testutil.WithTempHome(t)

	addName = "portal"
	addPlatform = ""
	addServer = "https://devportal.example.com"
	addAuth = utils.AuthTypeAPIKey
	addUsername = ""
	addPassword = ""
	addToken = ""
	addAPIKey = "api-key"
	addNoInteractive = true

	if err := runAddCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	portal, err := cfg.GetDevPortalFromPlatform("default", "portal")
	if err != nil {
		t.Fatalf("failed to resolve added portal: %v", err)
	}
	if portal.Auth.Type != utils.AuthTypeAPIKey || portal.Auth.APIKey != "api-key" {
		t.Fatalf("unexpected portal auth config: %+v", portal.Auth)
	}
	active, err := cfg.GetActiveDevPortalFromPlatform("default")
	if err != nil {
		t.Fatalf("failed to resolve active portal: %v", err)
	}
	if active.Name != "portal" {
		t.Fatalf("expected first portal to become active, got %q", active.Name)
	}
}

func TestRunAddCommand_AddsDevPortalToNamedPlatform(t *testing.T) {
	testutil.WithTempHome(t)

	addName = "portal-eu"
	addPlatform = "eu"
	addServer = "https://eu.example.com"
	addAuth = utils.AuthTypeOAuth
	addUsername = ""
	addPassword = ""
	addToken = "token"
	addAPIKey = ""
	addNoInteractive = true

	if err := runAddCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if _, err := cfg.GetDevPortalFromPlatform("eu", "portal-eu"); err != nil {
		t.Fatalf("expected portal in eu platform: %v", err)
	}
}

func TestRunListCommand_PrintsActiveDevPortal(t *testing.T) {
	testutil.WithTempHome(t)
	writeDevPortalConfig(t, &config.Config{
		CurrentPlatform: "default",
		Platforms: map[string]*config.Platform{
			"default": {
				DevPortals: map[string]*config.DevPortal{
					"portal-a": {URL: "http://a.example.com", Auth: config.AuthConfig{Type: utils.AuthTypeBasic}},
					"portal-b": {URL: "http://b.example.com", Auth: config.AuthConfig{Type: utils.AuthTypeOAuth}},
				},
				ActiveDevPortal: "portal-b",
			},
		},
	})

	listPlatform = ""
	out := testutil.CaptureStdout(t, func() {
		if err := runListCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "portal-b") || !strings.Contains(out, "*") {
		t.Fatalf("expected active portal marker in output, got %q", out)
	}
}

func TestRunListCommand_EmptyPlatformMessage(t *testing.T) {
	testutil.WithTempHome(t)
	writeDevPortalConfig(t, &config.Config{
		CurrentPlatform: "default",
		Platforms: map[string]*config.Platform{
			"default": {DevPortals: map[string]*config.DevPortal{}},
		},
	})

	listPlatform = ""
	out := testutil.CaptureStdout(t, func() {
		if err := runListCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "No devportal configured for platform default") {
		t.Fatalf("expected empty-platform message, got %q", out)
	}
}

func TestRunRemoveCommand_RemovesActivePortal(t *testing.T) {
	testutil.WithTempHome(t)
	writeDevPortalConfig(t, baseDevPortalConfig("http://example.com"))

	removeName = "portal"
	removePlatform = ""
	if err := runRemoveCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	platform := cfg.Platforms["default"]
	if len(platform.DevPortals) != 0 {
		t.Fatalf("expected portal to be removed, got %+v", platform.DevPortals)
	}
	if platform.ActiveDevPortal != "" {
		t.Fatalf("expected active portal to be cleared, got %q", platform.ActiveDevPortal)
	}
}

func TestRunRemoveCommand_MissingPortal(t *testing.T) {
	testutil.WithTempHome(t)
	writeDevPortalConfig(t, &config.Config{
		CurrentPlatform: "default",
		Platforms: map[string]*config.Platform{
			"default": {DevPortals: map[string]*config.DevPortal{}},
		},
	})

	removeName = "missing"
	removePlatform = ""
	err := runRemoveCommand()
	if err == nil || !strings.Contains(err.Error(), "devportal 'missing' not found") {
		t.Fatalf("expected missing portal error, got %v", err)
	}
}

func TestRunUseCommand_SetsActivePortalAndCurrentPlatform(t *testing.T) {
	testutil.WithTempHome(t)
	writeDevPortalConfig(t, &config.Config{
		CurrentPlatform: "default",
		Platforms: map[string]*config.Platform{
			"default": {DevPortals: map[string]*config.DevPortal{}},
			"eu": {
				DevPortals: map[string]*config.DevPortal{
					"portal-eu": {
						URL:  "http://eu.example.com",
						Auth: config.AuthConfig{Type: utils.AuthTypeAPIKey, APIKey: "api-key"},
					},
				},
			},
		},
	})

	useName = "portal-eu"
	usePlatform = "eu"
	out := testutil.CaptureStdout(t, func() {
		if err := runUseCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if cfg.CurrentPlatform != "eu" {
		t.Fatalf("expected current platform to be eu, got %q", cfg.CurrentPlatform)
	}
	if cfg.Platforms["eu"].ActiveDevPortal != "portal-eu" {
		t.Fatalf("expected active portal to be set, got %q", cfg.Platforms["eu"].ActiveDevPortal)
	}
	if !strings.Contains(out, "Using credentials from configuration.") {
		t.Fatalf("expected credential source message, got %q", out)
	}
}

func TestRunUseCommand_MissingPortal(t *testing.T) {
	testutil.WithTempHome(t)
	writeDevPortalConfig(t, &config.Config{
		CurrentPlatform: "default",
		Platforms: map[string]*config.Platform{
			"default": {DevPortals: map[string]*config.DevPortal{}},
		},
	})

	useName = "missing"
	usePlatform = ""
	err := runUseCommand()
	if err == nil || !strings.Contains(err.Error(), "devportal 'missing' not found") {
		t.Fatalf("expected missing portal error, got %v", err)
	}
}

func TestRunCurrentCommand_PrintsActivePortal(t *testing.T) {
	testutil.WithTempHome(t)
	writeDevPortalConfig(t, baseDevPortalConfig("http://example.com"))

	currentPlatform = ""
	out := testutil.CaptureStdout(t, func() {
		if err := runCurrentCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Current devportal: portal - http://example.com") {
		t.Fatalf("expected current portal output, got %q", out)
	}
}

func TestRunCurrentCommand_NoActivePortal(t *testing.T) {
	testutil.WithTempHome(t)
	writeDevPortalConfig(t, &config.Config{
		CurrentPlatform: "default",
		Platforms: map[string]*config.Platform{
			"default": {DevPortals: map[string]*config.DevPortal{}},
		},
	})

	currentPlatform = ""
	err := runCurrentCommand()
	if err == nil || !strings.Contains(err.Error(), "no active devportal set for platform 'default'") {
		t.Fatalf("expected no-active-portal error, got %v", err)
	}
}

func TestRunHealthCommand_UsesAuthHeadersAndParsesResponse(t *testing.T) {
	tests := []struct {
		name     string
		auth     config.AuthConfig
		env      map[string]string
		assertFn func(*testing.T, *http.Request)
	}{
		{
			name: "basic auth",
			auth: config.AuthConfig{Type: utils.AuthTypeBasic, Username: "admin", Password: "secret"},
			assertFn: func(t *testing.T, req *http.Request) {
				t.Helper()
				want := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))
				if got := req.Header.Get("Authorization"); got != want {
					t.Fatalf("expected authorization header %q, got %q", want, got)
				}
			},
		},
		{
			name: "oauth auth",
			auth: config.AuthConfig{Type: utils.AuthTypeOAuth, Token: "token"},
			assertFn: func(t *testing.T, req *http.Request) {
				t.Helper()
				if got := req.Header.Get("Authorization"); got != "Bearer token" {
					t.Fatalf("expected bearer token, got %q", got)
				}
			},
		},
		{
			name: "api key env override",
			auth: config.AuthConfig{Type: utils.AuthTypeAPIKey, APIKey: "config-key"},
			env: map[string]string{
				utils.EnvDevPortalAPIKey: "env-key",
			},
			assertFn: func(t *testing.T, req *http.Request) {
				t.Helper()
				if got := req.Header.Get(utils.DevPortalAPIHeader); got != "env-key" {
					t.Fatalf("expected env api key header, got %q", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.WithTempHome(t)
			for key, value := range tt.env {
				original := os.Getenv(key)
				if err := os.Setenv(key, value); err != nil {
					t.Fatalf("failed to set env: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Setenv(key, original)
				})
			}

			server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET request, got %s", req.Method)
				}
				if req.URL.Path != utils.DevPortalHealthPath {
					t.Fatalf("unexpected health path %s", req.URL.Path)
				}
				tt.assertFn(t, req)
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"status":"healthy","timestamp":"2026-01-01T00:00:00Z"}`)
			})

			writeDevPortalConfig(t, &config.Config{
				CurrentPlatform: "default",
				Platforms: map[string]*config.Platform{
					"default": {
						DevPortals: map[string]*config.DevPortal{
							"portal": {URL: server.URL, Auth: tt.auth},
						},
						ActiveDevPortal: "portal",
					},
				},
			})

			healthPlatform = ""
			out := testutil.CaptureStdout(t, func() {
				if err := runHealthCommand(); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
			if !strings.Contains(out, "DevPortal Status: healthy") {
				t.Fatalf("expected healthy output, got %q", out)
			}
		})
	}
}

func TestRunHealthCommand_UnauthorizedShowsCredentialGuidance(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	})

	writeDevPortalConfig(t, &config.Config{
		CurrentPlatform: "default",
		Platforms: map[string]*config.Platform{
			"default": {
				DevPortals: map[string]*config.DevPortal{
					"portal": {URL: server.URL, Auth: config.AuthConfig{Type: utils.AuthTypeOAuth, Token: "token"}},
				},
				ActiveDevPortal: "portal",
			},
		},
	})

	healthPlatform = ""
	err := runHealthCommand()
	if err == nil || !strings.Contains(err.Error(), "Credentials were sourced from the configuration file.") {
		t.Fatalf("expected credential guidance error, got %v", err)
	}
}

func TestRunHealthCommand_MalformedPayloadFails(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	})

	writeDevPortalConfig(t, baseDevPortalConfig(server.URL))

	healthPlatform = ""
	err := runHealthCommand()
	if err == nil || !strings.Contains(err.Error(), "failed to parse health response") {
		t.Fatalf("expected parse failure error, got %v", err)
	}
}

func baseDevPortalConfig(url string) *config.Config {
	return &config.Config{
		CurrentPlatform: "default",
		Platforms: map[string]*config.Platform{
			"default": {
				DevPortals: map[string]*config.DevPortal{
					"portal": {
						URL: url,
						Auth: config.AuthConfig{
							Type:   utils.AuthTypeAPIKey,
							APIKey: "api-key",
						},
					},
				},
				ActiveDevPortal: "portal",
			},
		},
	}
}

func writeDevPortalConfig(t *testing.T, cfg *config.Config) {
	t.Helper()
	testutil.WriteCLIConfig(t, cfg)
}


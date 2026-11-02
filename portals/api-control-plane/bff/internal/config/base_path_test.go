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

import (
	"strings"
	"testing"
)

func loadServer(t *testing.T, serverKeys string) (*Config, error) {
	t.Helper()
	return Load(writeConfig(t, minimalControlPlane+"\n[api_control_plane.server]\n"+serverKeys+"\n"))
}

// Leaving the key out keeps today's root deployment: nothing new in the runtime
// config the SPA reads.
func TestLoad_BasePathDefaultsToRoot(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimalControlPlane))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.BasePath != "" {
		t.Errorf("BasePath = %q, want empty", cfg.Server.BasePath)
	}
	if _, ok := cfg.RuntimeConfig["appBasePath"]; ok {
		t.Error("appBasePath must not be emitted for a root deployment")
	}
	if got := cfg.RuntimeConfig["platformApiBaseUrl"]; got != "/proxy" {
		t.Errorf("platformApiBaseUrl = %q, want /proxy", got)
	}
	for _, key := range []string{"authLoginUrl", "authCredentialsUrl", "authSessionUrl", "authLogoutUrl", "gatewayRelayUrl"} {
		if _, ok := cfg.RuntimeConfig[key]; ok {
			t.Errorf("%s must not be emitted for a root deployment", key)
		}
	}
}

func TestLoad_BasePathNormalizedAndExposed(t *testing.T) {
	for _, in := range []string{"/console", "console", "/console/", " /console// "} {
		cfg, err := loadServer(t, `base_path = "`+in+`"`)
		if err != nil {
			t.Fatalf("Load(%q): %v", in, err)
		}
		if cfg.Server.BasePath != "/console" {
			t.Errorf("BasePath(%q) = %q, want /console", in, cfg.Server.BasePath)
		}
		if got := cfg.RuntimeConfig["appBasePath"]; got != "/console" {
			t.Errorf("appBasePath = %q, want /console", got)
		}
		for key, want := range map[string]string{
			"platformApiBaseUrl": "/console/proxy",
			"authLoginUrl":       "/console/api/auth/login",
			"authCredentialsUrl": "/console/api/login",
			"authSessionUrl":     "/console/api/session",
			"authLogoutUrl":      "/console/api/logout",
			"gatewayRelayUrl":    "/console/api/gateway-relay/invoke",
		} {
			if got := cfg.RuntimeConfig[key]; got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
	}
}

func TestLoad_BasePathSlashIsRoot(t *testing.T) {
	cfg, err := loadServer(t, `base_path = "/"`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.BasePath != "" {
		t.Errorf("BasePath = %q, want empty", cfg.Server.BasePath)
	}
}

func TestLoad_BasePathRejectsUnsafeValues(t *testing.T) {
	for _, in := range []string{"/a/../b", "/a//b", "/a b", "/a?x=1", "/a#f", "/a%2fb", "//evil.com", `/a\\b`, "/{id}", "/a;b"} {
		if _, err := loadServer(t, `base_path = "`+in+`"`); err == nil {
			t.Errorf("Load(base_path=%q) succeeded, want an error", in)
		}
	}
}

// A mount collision would panic in the mux at startup, so it fails validation with a
// readable error instead. Mounts are compared as composed with the base path.
func TestLoad_BasePathRejectsMountCollisions(t *testing.T) {
	const billing = `
[[api_control_plane.control_plane.upstreams]]
name = "billing"
url  = "https://billing:8443"
`
	const nested = `
[[api_control_plane.control_plane.upstreams]]
name   = "nested"
url    = "https://nested:8443"
prefix = "/console/proxy"
`
	cases := []struct{ basePath, upstream string }{
		{"/proxy", billing},
		{"/proxy/billing", billing},
		{"/console", nested},
	}
	for _, tc := range cases {
		body := "[api_control_plane.server]\nbase_path = \"" + tc.basePath + "\"\n" + minimalControlPlane + tc.upstream
		_, err := Load(writeConfig(t, body))
		if err == nil || !strings.Contains(err.Error(), "collide") {
			t.Errorf("Load(base_path=%q) err = %v, want a collision error", tc.basePath, err)
		}
	}
	// A distinct upstream under the same base path is fine.
	body := "[api_control_plane.server]\nbase_path = \"/console\"\n" + minimalControlPlane + billing
	if _, err := Load(writeConfig(t, body)); err != nil {
		t.Errorf("Load: %v", err)
	}
}

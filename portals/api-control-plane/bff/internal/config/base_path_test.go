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
		if got := cfg.RuntimeConfig["platformApiBaseUrl"]; got != "/console/proxy" {
			t.Errorf("platformApiBaseUrl = %q, want /console/proxy", got)
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

// A base path claiming a proxy's mux subtree would panic at startup, so it fails
// validation with a readable error instead.
func TestLoad_BasePathRejectsProxyPrefixCollision(t *testing.T) {
	upstream := `
[[api_control_plane.control_plane.upstreams]]
name = "billing"
url  = "https://billing:8443"
`
	for _, in := range []string{"/proxy", "/proxy/billing"} {
		body := "[api_control_plane.server]\nbase_path = \"" + in + "\"\n" + minimalControlPlane + upstream
		_, err := Load(writeConfig(t, body))
		if err == nil || !strings.Contains(err.Error(), "must differ") {
			t.Errorf("Load(base_path=%q) err = %v, want a collision error", in, err)
		}
	}
}

func TestLoad_XFrameOptions(t *testing.T) {
	cases := []struct{ keys, want string }{
		{"", XFrameDeny},
		{`x_frame_options = ""`, XFrameDeny},
		{`x_frame_options = "DENY"`, XFrameDeny},
		{`x_frame_options = "SAMEORIGIN"`, XFrameSameOrigin},
		{`x_frame_options = " sameorigin "`, XFrameSameOrigin},
	}
	for _, tc := range cases {
		cfg, err := loadServer(t, tc.keys)
		if err != nil {
			t.Fatalf("Load(%q): %v", tc.keys, err)
		}
		if cfg.Server.XFrameOptions != tc.want {
			t.Errorf("XFrameOptions(%q) = %q, want %q", tc.keys, cfg.Server.XFrameOptions, tc.want)
		}
	}
	if _, err := loadServer(t, `x_frame_options = "ALLOW-FROM https://example.com"`); err == nil {
		t.Error("an unsupported x_frame_options value must fail startup")
	}
}

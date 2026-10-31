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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// legacyScopesBaseConfig is a loadable config with an [auth.oidc] table and no
// scope key, so the default login request applies.
const legacyScopesBaseConfig = `
[api_control_plane.control_plane]
url = "https://platform-api:9243"

[api_control_plane.auth.oidc]
`

func loadWithOIDC(t *testing.T, oidcExtra string) *Config {
	t.Helper()
	body := legacyScopesBaseConfig
	if oidcExtra != "" {
		body = strings.Replace(body, "[api_control_plane.auth.oidc]\n", "[api_control_plane.auth.oidc]\n"+oidcExtra+"\n", 1)
	}
	cfg, err := Load(writeConfig(t, body))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return cfg
}

// With no scope key and no request_legacy_scopes key — an existing deployment
// after upgrade — the login request is today's: the permission-tree scopes the
// shipped roles grant plus every pre-permission-tree name.
func TestLoad_DefaultLoginScopesKeepLegacyNamesByDefault(t *testing.T) {
	cfg := loadWithOIDC(t, "")
	if !cfg.Auth.OIDC.RequestLegacyScopes {
		t.Error("RequestLegacyScopes defaults to false, want true so an upgrade keeps its scopes")
	}
	if cfg.Auth.OIDC.Scopes != defaultLoginScopes(true) {
		t.Errorf("Scopes = %q, want defaultOIDCScopes + legacyOIDCScopes", cfg.Auth.OIDC.Scopes)
	}
	got := strings.Fields(cfg.Auth.OIDC.Scopes)
	for _, s := range strings.Fields(legacyOIDCScopes) {
		if !slices.Contains(got, s) {
			t.Errorf("default request omits legacy scope %s", s)
		}
	}
}

// request_legacy_scopes = false requests only the minimal set.
func TestLoad_RequestLegacyScopesFalseRequestsMinimalSet(t *testing.T) {
	cfg := loadWithOIDC(t, "request_legacy_scopes = false")
	if cfg.Auth.OIDC.Scopes != defaultOIDCScopes {
		t.Errorf("Scopes = %q, want defaultOIDCScopes", cfg.Auth.OIDC.Scopes)
	}
}

// An explicit scope is used as written; request_legacy_scopes does not touch it.
func TestLoad_ExplicitScopeIgnoresRequestLegacyScopes(t *testing.T) {
	const explicit = "openid profile email offline_access ap:read"
	for _, extra := range []string{
		`scope = "` + explicit + `"`,
		`scope = "` + explicit + `"` + "\nrequest_legacy_scopes = true",
	} {
		if got := loadWithOIDC(t, extra).Auth.OIDC.Scopes; got != explicit {
			t.Errorf("with %q: Scopes = %q, want %q", extra, got, explicit)
		}
	}
}

// defaultOIDCScopes requests exactly the ap: scopes the shipped roles grant —
// no more, so the minimal request stays minimal, and no less, so an IDP role
// mirroring a shipped role is issued in full — minus the API-key ownership
// override, which is never requested. legacyOIDCScopes never repeats one.
func TestDefaultOIDCScopesMatchShippedRoles(t *testing.T) {
	rolesPath := filepath.Join("..", "..", "..", "..", "..", "platform-api", "resources", "role-to-scope-mapping.yaml")
	data, err := os.ReadFile(rolesPath)
	if err != nil {
		t.Skipf("role mapping not present: %s", rolesPath)
	}
	var doc struct {
		Roles []struct {
			Scopes []string `yaml:"scopes"`
		} `yaml:"roles"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", rolesPath, err)
	}
	want := map[string]bool{}
	for _, role := range doc.Roles {
		for _, s := range role.Scopes {
			if strings.HasPrefix(s, "ap:") && s != "ap:credential:api_key:all:manage" && s != "ap:api_key:all:manage" {
				want[s] = true
			}
		}
	}
	got := map[string]bool{}
	for _, s := range strings.Fields(defaultOIDCScopes) {
		if strings.HasPrefix(s, "ap:") {
			got[s] = true
		}
	}
	for s := range want {
		if !got[s] {
			t.Errorf("defaultOIDCScopes omits %s, which a shipped role grants", s)
		}
	}
	for s := range got {
		if !want[s] {
			t.Errorf("defaultOIDCScopes requests %s, which no shipped role grants — move it to legacyOIDCScopes", s)
		}
	}
	for _, s := range strings.Fields(legacyOIDCScopes) {
		if got[s] {
			t.Errorf("legacyOIDCScopes repeats %s from defaultOIDCScopes", s)
		}
	}
}

/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
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
	"testing"

	toml "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// loadRuntimeKoanf parses a TOML fragment the way the real loader does, so
// buildRuntimeConfig sees the value types koanf actually produces.
func loadRuntimeKoanf(t *testing.T, body string) *koanf.Koanf {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing the config fragment: %v", err)
	}
	k := koanf.New(".")
	if err := k.Load(file.Provider(path), toml.Parser()); err != nil {
		t.Fatalf("loading the config fragment: %v", err)
	}
	return k
}

// A feature flag is only honoured if it reaches the browser, so the key has to stay
// on browserSafeKeys and keep the name the SPA reads.
func TestBuildRuntimeConfig_FeatureFlagReachesTheSPA(t *testing.T) {
	for _, set := range []string{"false", "true"} {
		k := loadRuntimeKoanf(t, "[feature_flags]\nagent_proxy_enabled = "+set+"\n")

		out := buildRuntimeConfig(&Config{}, k)

		if got := out["APIP_AIW_FEATURE_FLAGS_AGENT_PROXY_ENABLED"]; got != set {
			t.Errorf("agent_proxy_enabled = %s reached the SPA as %q, want %q", set, got, set)
		}
	}
}

// A deployment that never mentions a feature must leave it on, which it does by the
// key being absent and the SPA falling back to its own default.
func TestBuildRuntimeConfig_AbsentFeatureFlagIsOmitted(t *testing.T) {
	k := loadRuntimeKoanf(t, "default_org_region = \"us\"\n")

	out := buildRuntimeConfig(&Config{}, k)

	if _, ok := out["APIP_AIW_FEATURE_FLAGS_AGENT_PROXY_ENABLED"]; ok {
		t.Error("agent_proxy_enabled was emitted although the config never declares it")
	}
}

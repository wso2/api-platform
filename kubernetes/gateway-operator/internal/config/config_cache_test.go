/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheExclusionsDefaultOn(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.Cache.ExcludeHelmReleaseSecrets {
		t.Error("ExcludeHelmReleaseSecrets = false, want true by default")
	}
	if !cfg.Cache.ExcludeGatewayChartConfigMaps {
		t.Error("ExcludeGatewayChartConfigMaps = false, want true by default")
	}
}

func TestCacheExclusionsCanBeTurnedOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "cache:\n  exclude_helm_release_secrets: false\n  exclude_gateway_chart_config_maps: false\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Cache.ExcludeHelmReleaseSecrets || cfg.Cache.ExcludeGatewayChartConfigMaps {
		t.Fatalf("Cache = %+v, want both exclusions off", cfg.Cache)
	}
}

func TestGatewayAPIEnabledByDefault(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.GatewayAPI.Enabled {
		t.Fatal("GatewayAPI.Enabled = false, want true by default")
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("gateway_api:\n  enabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.GatewayAPI.Enabled {
		t.Fatal("GatewayAPI.Enabled = true, want false from the config file")
	}
}

func TestCacheSyncTimeout(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Cache.SyncTimeout != 2*time.Minute {
		t.Errorf("SyncTimeout = %s, want 2m by default", cfg.Cache.SyncTimeout)
	}

	path := writeConfig(t, "cache:\n  sync_timeout: 10m\n")
	cfg, err = LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Cache.SyncTimeout != 10*time.Minute {
		t.Errorf("SyncTimeout = %s, want 10m from the config file", cfg.Cache.SyncTimeout)
	}

	for _, bad := range []string{"0s", "-1m"} {
		if _, err := LoadConfig(writeConfig(t, "cache:\n  sync_timeout: "+bad+"\n")); err == nil {
			t.Errorf("LoadConfig with sync_timeout %s: want an error", bad)
		}
	}
}

// Each of these keys has an underscore in its own name, so the generic GATEWAY_
// mapping would send the variable to the wrong key and leave the setting unchanged.
func TestEnvOverridesUnderscoredKeys(t *testing.T) {
	fileOpposite := writeConfig(t, `gateway_api:
  enabled: true
cache:
  sync_timeout: 3m
  exclude_helm_release_secrets: true
  exclude_gateway_chart_config_maps: true
`)
	for _, tc := range []struct {
		name       string
		configPath string
	}{
		{name: "over defaults", configPath: ""},
		{name: "over a conflicting config file", configPath: fileOpposite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GATEWAY_API_ENABLED", "false")
			t.Setenv("GATEWAY_CACHE_SYNC_TIMEOUT", "7m")
			t.Setenv("GATEWAY_CACHE_EXCLUDE_HELM_RELEASE_SECRETS", "false")
			t.Setenv("GATEWAY_CACHE_EXCLUDE_GATEWAY_CHART_CONFIG_MAPS", "false")

			cfg, err := LoadConfig(tc.configPath)
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if cfg.GatewayAPI.Enabled {
				t.Error("GatewayAPI.Enabled = true, want false from GATEWAY_API_ENABLED")
			}
			if cfg.Cache.SyncTimeout != 7*time.Minute {
				t.Errorf("SyncTimeout = %s, want 7m from GATEWAY_CACHE_SYNC_TIMEOUT", cfg.Cache.SyncTimeout)
			}
			if cfg.Cache.ExcludeHelmReleaseSecrets {
				t.Error("ExcludeHelmReleaseSecrets = true, want false from the environment")
			}
			if cfg.Cache.ExcludeGatewayChartConfigMaps {
				t.Error("ExcludeGatewayChartConfigMaps = true, want false from the environment")
			}
		})
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

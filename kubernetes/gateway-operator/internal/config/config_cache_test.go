/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"os"
	"path/filepath"
	"testing"
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

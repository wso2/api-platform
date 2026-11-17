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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/common/configcompat"
)

func loadEGWithDestinationGuard(t *testing.T, names ...string) (*Config, error) {
	t.Helper()
	body := "[kafka]\nbrokers = [\"localhost:9092\"]\n\n[controlplane]\nenabled = false\n"
	for _, name := range names {
		body += fmt.Sprintf("\n[http_client.%s]\nmax_redirects = 3\n", name)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, _, err := Load(path)
	return cfg, err
}

func TestLoad_DestinationGuard(t *testing.T) {
	cfg, err := loadEGWithDestinationGuard(t, configcompat.DestinationGuardKey)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPClient.DestinationGuard.MaxRedirects != 3 {
		t.Fatalf("max_redirects = %d, want 3", cfg.HTTPClient.DestinationGuard.MaxRedirects)
	}
}

// A config written before the rename keeps loading, under the new name.
func TestLoad_DestinationGuardUnderItsPreviousName(t *testing.T) {
	cfg, err := loadEGWithDestinationGuard(t, configcompat.LegacyDestinationGuardKey)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPClient.DestinationGuard.MaxRedirects != 3 {
		t.Fatalf("max_redirects = %d, want 3", cfg.HTTPClient.DestinationGuard.MaxRedirects)
	}
}

func TestLoad_DestinationGuardUnderBothNamesFails(t *testing.T) {
	_, err := loadEGWithDestinationGuard(t, configcompat.LegacyDestinationGuardKey, configcompat.DestinationGuardKey)
	if err == nil || !strings.Contains(err.Error(), "both set") {
		t.Fatalf("err = %v, want a both-set error", err)
	}
}

// An environment override written before the rename keeps working. Environment
// variables are loaded after the file's keys are migrated, so the loader has to
// rename the override's key itself.
func TestLoad_DestinationGuardEnvOverrideUnderItsPreviousName(t *testing.T) {
	t.Setenv("APIP_EGW_HTTP__CLIENT_SSRF_MAX__REDIRECTS", "7")
	cfg, err := loadEGWithDestinationGuard(t)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPClient.DestinationGuard.MaxRedirects != 7 {
		t.Fatalf("max_redirects = %d, want 7 from the environment", cfg.HTTPClient.DestinationGuard.MaxRedirects)
	}
}

// A file already using the new name and an override still using the old one are
// not a conflict: the override names a leaf, and it overrides the file as any
// environment variable does.
func TestLoad_DestinationGuardEnvOverrideUnderItsPreviousNameOverridesTheFile(t *testing.T) {
	t.Setenv("APIP_EGW_HTTP__CLIENT_SSRF_MAX__REDIRECTS", "7")
	cfg, err := loadEGWithDestinationGuard(t, configcompat.DestinationGuardKey)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPClient.DestinationGuard.MaxRedirects != 7 {
		t.Fatalf("max_redirects = %d, want 7 from the environment over the file's 3", cfg.HTTPClient.DestinationGuard.MaxRedirects)
	}
}

func TestLoad_DestinationGuardEnvOverride(t *testing.T) {
	t.Setenv("APIP_EGW_HTTP__CLIENT_DESTINATION__GUARD_MAX__REDIRECTS", "7")
	cfg, err := loadEGWithDestinationGuard(t)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPClient.DestinationGuard.MaxRedirects != 7 {
		t.Fatalf("max_redirects = %d, want 7", cfg.HTTPClient.DestinationGuard.MaxRedirects)
	}
}

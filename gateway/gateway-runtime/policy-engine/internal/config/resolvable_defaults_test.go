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
	"os"
	"path/filepath"
	"testing"
)

// Verifies the two new defaults reach a config that never mentions them:
// traffic_logging.masked_headers, and the ${config}-resolvable pricing_file
// (which must land in RawConfig, not just the struct).
func TestNewDefaultsReachBareConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bare.toml")
	if err := os.WriteFile(p, []byte("[policy_engine.logging]\nlevel = \"info\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	t.Logf("masked_headers = %v", cfg.TrafficLogging.MaskedHeaders)
	if len(cfg.TrafficLogging.MaskedHeaders) != 3 {
		t.Errorf("want 3 masked headers, got %v", cfg.TrafficLogging.MaskedHeaders)
	}

	pc, _ := cfg.PolicyConfigurations["llm_cost_v1"].(map[string]interface{})
	t.Logf("struct pricing_file = %v", pc["pricing_file"])

	// The resolver evaluates against RawConfig — this is the path that matters.
	raw, _ := cfg.PolicyEngine.RawConfig["policy_configurations"].(map[string]interface{})
	inner, _ := raw["llm_cost_v1"].(map[string]interface{})
	got, _ := inner["pricing_file"].(string)
	t.Logf("RawConfig pricing_file = %q", got)
	if got != DefaultLLMCostPricingFile {
		t.Errorf("RawConfig pricing_file = %q, want %q", got, DefaultLLMCostPricingFile)
	}
}

// An operator-set value must still win over the seeded default.
func TestOperatorOverridesPricingFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "over.toml")
	body := "[policy_configurations.llm_cost_v1]\npricing_file = \"/custom/prices.json\"\n" +
		"[traffic_logging]\nmasked_headers = [\"cookie\"]\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	raw, _ := cfg.PolicyEngine.RawConfig["policy_configurations"].(map[string]interface{})
	inner, _ := raw["llm_cost_v1"].(map[string]interface{})
	if got, _ := inner["pricing_file"].(string); got != "/custom/prices.json" {
		t.Errorf("override lost: got %q", got)
	}
	if got := cfg.TrafficLogging.MaskedHeaders; len(got) != 1 || got[0] != "cookie" {
		t.Errorf("masked_headers override lost: got %v", got)
	}
}

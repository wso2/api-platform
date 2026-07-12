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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePEInterpConfig(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

// TestLoad_Interpolation_QuotedNumericAndRawConfig verifies that a quoted {{ env }}
// token coerces to the target scalar type at unmarshal (WeaklyTypedInput), and that
// RawConfig — used for ${config} CEL resolution in policies — holds the RESOLVED
// value, proving interpolation runs before the RawConfig capture.
func TestLoad_Interpolation_QuotedNumericAndRawConfig(t *testing.T) {
	t.Setenv("PE_PORT", "9005")
	path := writePEInterpConfig(t, `
[policy_engine.server]
extproc_port = "{{ env \"PE_PORT\" }}"

[policy_engine.config_mode]
mode = "file"

[policy_engine.file_config]
path = "configs/policy-chains.yaml"

[policy_configurations]
shared_value = "{{ env \"PE_PORT\" }}"
`)
	cfg, err := Load(path)
	require.NoError(t, err)

	// Quoted string token coerced to int.
	assert.Equal(t, 9005, cfg.PolicyEngine.Server.ExtProcPort)

	// RawConfig reflects the resolved value, not the literal {{ ... }} token.
	pc, ok := cfg.PolicyEngine.RawConfig["policy_configurations"].(map[string]interface{})
	require.True(t, ok, "policy_configurations should be a map in RawConfig")
	assert.Equal(t, "9005", pc["shared_value"])
}

func TestLoad_Interpolation_EnvDefault(t *testing.T) {
	os.Unsetenv("PE_MISSING_LEVEL")
	path := writePEInterpConfig(t, `
[policy_engine.config_mode]
mode = "file"

[policy_engine.file_config]
path = "configs/policy-chains.yaml"

[policy_engine.logging]
level = "{{ env \"PE_MISSING_LEVEL\" \"debug\" }}"
`)
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.PolicyEngine.Logging.Level)
}

func TestLoad_Interpolation_MissingRequiredFailsClosed(t *testing.T) {
	os.Unsetenv("PE_ABSENT")
	path := writePEInterpConfig(t, `
[policy_engine.config_mode]
mode = "file"

[policy_engine.file_config]
path = "configs/policy-chains.yaml"

[policy_configurations]
secret = "{{ env \"PE_ABSENT\" }}"
`)
	cfg, err := Load(path)
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), `required env var "PE_ABSENT" is not found`)
}

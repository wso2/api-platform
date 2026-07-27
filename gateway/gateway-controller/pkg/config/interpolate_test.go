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
	"github.com/wso2/api-platform/common/configinterpolate"
)

func writeCtlInterpConfig(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func TestLoadConfig_Interpolation_EnvToken(t *testing.T) {
	t.Setenv("CI_CP_TOKEN", "resolved-token")
	path := writeCtlInterpConfig(t, `
[controller.controlplane]
host = "cp.example.com"
token = "{{ env \"CI_CP_TOKEN\" }}"
`)
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "resolved-token", cfg.Controller.ControlPlane.Token)
}

func TestLoadConfig_Interpolation_EnvDefault(t *testing.T) {
	os.Unsetenv("CI_MISSING_LEVEL")
	path := writeCtlInterpConfig(t, `
[controller.logging]
level = "{{ env \"CI_MISSING_LEVEL\" \"debug\" }}"
`)
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.Controller.Logging.Level)
}

func TestLoadConfig_Interpolation_MissingRequiredFailsClosed(t *testing.T) {
	os.Unsetenv("CI_ABSENT")
	path := writeCtlInterpConfig(t, `
[controller.controlplane]
token = "{{ env \"CI_ABSENT\" }}"
`)
	cfg, err := LoadConfig(path)
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), `required env var "CI_ABSENT" is not found`)
}

func TestLoadConfig_Interpolation_FileToken(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "cp-token")
	require.NoError(t, os.WriteFile(secret, []byte("file-token\n"), 0o600))
	// Point the shared allowlist env var at the temp dir so the file() source is allowed.
	t.Setenv(configinterpolate.EnvFileSourceAllowlist, dir)

	path := writeCtlInterpConfig(t, `
[controller.controlplane]
host = "cp.example.com"
token = "{{ file \"`+secret+`\" }}"
`)
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "file-token", cfg.Controller.ControlPlane.Token)
}

// TestLoadConfig_Interpolation_BasicAuthAdminCreds exercises the full path the
// shipped config.toml uses for its admin credentials: {{ env }} tokens resolved
// from the environment (as scripts/setup.sh writes them into api-platform.env).
func TestLoadConfig_Interpolation_BasicAuthAdminCreds(t *testing.T) {
	t.Setenv("APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_USERNAME", "gwadmin")
	t.Setenv("APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH", "$2y$10$C6UzMDM.H6dfI/f/IKcEeO3JxpH3nZ0z8oJ0kQ1yQ2wRxYzAbCdEe")
	path := writeCtlInterpConfig(t, `
[controller.auth.basic]
enabled = true

[[controller.auth.basic.users]]
username        = '{{ env "APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_USERNAME" "" }}'
password        = '{{ env "APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH" "" }}'
password_hashed = true
roles           = ["admin"]
`)
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.Len(t, cfg.Controller.Auth.Basic.Users, 1)
	u := cfg.Controller.Auth.Basic.Users[0]
	assert.Equal(t, "gwadmin", u.Username)
	assert.Equal(t, "$2y$10$C6UzMDM.H6dfI/f/IKcEeO3JxpH3nZ0z8oJ0kQ1yQ2wRxYzAbCdEe", u.Password)
	assert.True(t, u.PasswordHashed)
}

// TestLoadConfig_Interpolation_BasicAuthAdminCredsUnset_FailsClosed verifies the
// shipped-config scenario with the credential env vars unset: the {{ env }}
// tokens resolve to their empty defaults, leaving a user present but empty-valued,
// which validateAuthConfig rejects so the controller refuses to start.
func TestLoadConfig_Interpolation_BasicAuthAdminCredsUnset_FailsClosed(t *testing.T) {
	os.Unsetenv("APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_USERNAME")
	os.Unsetenv("APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH")
	path := writeCtlInterpConfig(t, `
[controller.auth.basic]
enabled = true

[[controller.auth.basic.users]]
username        = '{{ env "APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_USERNAME" "" }}'
password        = '{{ env "APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH" "" }}'
password_hashed = true
roles           = ["admin"]
`)
	cfg, err := LoadConfig(path)
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "empty username or password")
}

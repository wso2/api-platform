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
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
)

// Without an allow list, artifacts read any variable, as in earlier releases.
func TestArtifactEnvAccess_DefaultIsUnrestricted(t *testing.T) {
	for name, toml := range map[string]string{
		"no section":  "[controller.logging]\nlevel = \"info\"\n",
		"no key":      "[policy_engine.env_access]\n",
		"blank token": "[policy_engine.env_access]\nallow_list = '{{ env \"TEST_UNSET_ALLOW_LIST_12345\" \"\" }}'\n",
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := LoadConfig(writeTOML(t, t.TempDir(), "c.toml", toml))
			require.NoError(t, err)

			policy, err := cfg.ArtifactEnvAccessPolicy()
			require.NoError(t, err)
			assert.True(t, policy.Unrestricted())
			assert.True(t, policy.Allows("HOME"))
		})
	}
}

func TestArtifactEnvAccess_EmptyListPermitsNothing(t *testing.T) {
	cfg, err := LoadConfig(writeTOML(t, t.TempDir(), "c.toml", `
[policy_engine.env_access]
allow_list = []
`))
	require.NoError(t, err)

	policy, err := cfg.ArtifactEnvAccessPolicy()
	require.NoError(t, err)
	assert.False(t, policy.Unrestricted())
	assert.False(t, policy.Allows("HOME"))
}

func TestArtifactEnvAccess_ReadFromPolicyEngineSection(t *testing.T) {
	cfg, err := LoadConfig(writeTOML(t, t.TempDir(), "c.toml", `
[policy_engine.env_access]
allow_list = ["BACKEND_URL", "APP_*"]
`))
	require.NoError(t, err)

	policy, err := cfg.ArtifactEnvAccessPolicy()
	require.NoError(t, err)
	assert.True(t, policy.Allows("BACKEND_URL"))
	assert.True(t, policy.Allows("APP_TIMEOUT"))
	assert.False(t, policy.Allows("DB_PASSWORD"))
	assert.False(t, policy.Allows("APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN"))
}

// The allow list can be supplied through a config-interpolation token as a
// comma-separated string.
func TestArtifactEnvAccess_CommaSeparatedViaEnvToken(t *testing.T) {
	t.Setenv("TEST_ARTIFACT_ENV_ALLOW_LIST", "BACKEND_URL, APP_*")
	cfg, err := LoadConfig(writeTOML(t, t.TempDir(), "c.toml", `
[policy_engine.env_access]
allow_list = '{{ env "TEST_ARTIFACT_ENV_ALLOW_LIST" "" }}'
`))
	require.NoError(t, err)

	policy, err := cfg.ArtifactEnvAccessPolicy()
	require.NoError(t, err)
	assert.True(t, policy.Allows("BACKEND_URL"))
	assert.True(t, policy.Allows("APP_X"))
}

func TestArtifactEnvAccess_MalformedAllowListRefusesStartup(t *testing.T) {
	for _, allowList := range []string{`["*"]`, `["FOO*BAR"]`, `["1BAD"]`, `[1]`} {
		t.Run(allowList, func(t *testing.T) {
			_, err := LoadConfig(writeTOML(t, t.TempDir(), "c.toml", `
[policy_engine.env_access]
allow_list = `+allowList+`
`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "policy_engine.env_access")
		})
	}
}

// InstallArtifactEnvAccessPolicy installs the configured policy process-wide,
// which is what both the gateway-controller and the event-gateway controller
// call at startup.
func TestInstallArtifactEnvAccessPolicy(t *testing.T) {
	prev := funcs.SetEnvAccessPolicy(nil)
	t.Cleanup(func() { funcs.SetEnvAccessPolicy(prev) })

	restricted, err := LoadConfig(writeTOML(t, t.TempDir(), "c.toml", `
[policy_engine.env_access]
allow_list = ["BACKEND_URL"]
`))
	require.NoError(t, err)
	require.NoError(t, restricted.InstallArtifactEnvAccessPolicy(slog.Default()))
	installed := funcs.SetEnvAccessPolicy(nil)
	assert.True(t, installed.Allows("BACKEND_URL"))
	assert.False(t, installed.Allows("HOME"))

	unrestricted, err := LoadConfig(writeTOML(t, t.TempDir(), "c.toml", "[controller.logging]\nlevel = \"info\"\n"))
	require.NoError(t, err)
	require.NoError(t, unrestricted.InstallArtifactEnvAccessPolicy(nil))
	assert.True(t, funcs.SetEnvAccessPolicy(nil).Unrestricted())
}

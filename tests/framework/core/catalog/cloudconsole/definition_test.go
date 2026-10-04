/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudConsoleDefinition(t *testing.T) {
	definition := CloudConsole()
	require.NoError(t, definition.Validate())
	require.True(t, definition.IsExternal())
	require.False(t, definition.IsCompose())
	require.Empty(t, definition.Image.Ref)
	require.Equal(t, []string{EndpointAPIPBML}, definition.ExternalEndpointNames())
}

func TestLoadEnvironmentConfig(t *testing.T) {
	actual, err := LoadEnvironmentConfig("environments.example.toml")
	require.NoError(t, err)
	require.Equal(t, "development.example", actual.Environments["development"].CloudBaseDomain)

	t.Run("loads a mapping", func(t *testing.T) {
		path := writeConfig(t, "[environments.dev]\ncloud_base_domain = \"dev.example.com\"\n")
		config, err := LoadEnvironmentConfig(path)
		require.NoError(t, err)
		require.Equal(t, "dev.example.com", config.Environments["dev"].CloudBaseDomain)
		require.Equal(t, "dev.example.com", mustDomain(t, config, "dev"))
	})

	tests := map[string]string{
		"missing table":             "[cloud]\nname = \"value\"\n",
		"empty table":               "[environments]\n",
		"missing domain":            "[environments.dev]\n",
		"empty domain":              "[environments.dev]\ncloud_base_domain = \" \"\n",
		"domain has scheme":         "[environments.dev]\ncloud_base_domain = \"https://example.com\"\n",
		"domain has path":           "[environments.dev]\ncloud_base_domain = \"example.com/path\"\n",
		"domain has port":           "[environments.dev]\ncloud_base_domain = \"example.com:8443\"\n",
		"environment has separator": "[environments.\"dev/foo\"]\ncloud_base_domain = \"example.com\"\n",
		"environment not a table":   "[environments]\ndev = \"example.com\"\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := LoadEnvironmentConfig(writeConfig(t, content))
			require.Error(t, err)
		})
	}
}

func TestEnvironmentConfigDomain(t *testing.T) {
	config := EnvironmentConfig{Environments: map[string]EnvironmentEntry{
		"dev": {CloudBaseDomain: "dev.example.com"},
	}}
	_, err := config.Domain("")
	require.ErrorContains(t, err, "environment is required")
	_, err = config.Domain("missing")
	require.ErrorContains(t, err, "is not configured")
}

func TestResolveEndpoints(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, EnvironmentConfigPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("[environments.sample]\ncloud_base_domain = \"example.test\"\n"), 0o644))

	endpoints, err := resolveEndpoints(root, map[string]string{EnvironmentParameter: "sample"})
	require.NoError(t, err)
	require.Equal(t,
		"https://sample-wso2cloud.gateway.example.test/apip-bml-apip-bml-endpoint/api/v0.9",
		endpoints[EndpointAPIPBML])
}

func TestResolveEndpointsRejectsMissingInputs(t *testing.T) {
	root := t.TempDir()
	_, err := resolveEndpoints(root, nil)
	require.ErrorContains(t, err, "parameter \"environment\" is required")

	path := filepath.Join(root, EnvironmentConfigPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("[environments.dev]\ncloud_base_domain = \"dev.example.com\"\n"), 0o644))
	_, err = resolveEndpoints(root, map[string]string{EnvironmentParameter: "dev"})
	require.NoError(t, err)

	_, err = resolveEndpoints(root, map[string]string{EnvironmentParameter: "missing"})
	require.ErrorContains(t, err, "is not configured")

	_, err = resolveEndpoints(root, map[string]string{EnvironmentParameter: "dev/foo"})
	require.ErrorContains(t, err, "must be a single valid DNS label")
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "environments.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func mustDomain(t *testing.T, config EnvironmentConfig, environment string) string {
	t.Helper()
	domain, err := config.Domain(environment)
	require.NoError(t, err)
	return domain
}

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

package apiportal

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/builder"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
)

func TestAPIPortalDefinition(t *testing.T) {
	definition := APIPortal()
	require.Equal(t, "api-portal", definition.Name)
	require.True(t, definition.IsCompose())
	require.Equal(t, []string{"platform-api"}, definition.DependsOn)
	require.NotNil(t, definition.DB)
	require.NotEmpty(t, definition.Compose.GeneratedFiles)
	for _, key := range []string{
		"APIP_AP_SECURITY_ENCRYPTION_KEY",
		"APIP_AP_SECURITY_SESSION_SECRET",
	} {
		value, ok := definition.Compose.Env[key]
		require.True(t, ok, "%s must be injected into the portal runtime", key)
		require.Len(t, value, 64, "%s must be a 32-byte hex secret", key)
		_, err := hex.DecodeString(value)
		require.NoError(t, err, "%s must be hexadecimal", key)
	}
	_, ok := definition.Endpoint("http")
	require.True(t, ok)
}

func TestAPIPortalCoverageEnvironmentFollowsRunMode(t *testing.T) {
	t.Setenv(shared.EnvCoverageMode, "false")
	require.NotContains(t, APIPortal().Compose.Env, "NODE_V8_COVERAGE")

	t.Setenv(shared.EnvCoverageMode, "true")
	require.Equal(t, "/coverage", APIPortal().Compose.Env["NODE_V8_COVERAGE"])
}

func TestAPIPortalBuildDeclaresBrowserCoverage(t *testing.T) {
	spec, err := BuildSpec("test")
	require.NoError(t, err)
	require.Contains(t, spec.Coverage.Types, builder.NodeV8Coverage)
	require.Contains(t, spec.Coverage.Types, builder.BrowserJSCoverage)
	require.Equal(t, "portals/api-portal/src/scripts", spec.Coverage.Browser.SourceRoot)
	require.NotEmpty(t, spec.Coverage.Browser.Include)
}

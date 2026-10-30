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

package apicontrolplane

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/builder"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
)

func TestAPIControlPlaneDefinition(t *testing.T) {
	definition := APIControlPlane()
	require.Equal(t, "api-control-plane", definition.Name)
	require.True(t, definition.IsCompose())
	require.Equal(t, []string{"platform-api"}, definition.DependsOn)
	require.Equal(t, shared.ControlPlaneCrypto()["certs/cert.pem"], definition.Compose.GeneratedFiles["tls/cp-cert.pem"],
		"the BFF trusts the control plane's own certificate")
	require.NotNil(t, definition.Config)
	endpoint, ok := definition.Endpoint("http")
	require.True(t, ok)
	require.Equal(t, 8082, endpoint.Port)
	require.Equal(t, "http", definition.Health.Endpoint)
}

func TestAPIControlPlaneCoverageEnvironmentFollowsRunMode(t *testing.T) {
	t.Setenv(shared.EnvCoverageMode, "false")
	require.NotContains(t, APIControlPlane().Compose.Env, "GOCOVERDIR")

	t.Setenv(shared.EnvCoverageMode, "true")
	require.Equal(t, "/coverage", APIControlPlane().Compose.Env["GOCOVERDIR"])
}

func TestAPIControlPlaneBuildSpec(t *testing.T) {
	spec, err := BuildSpec("1.2.3")
	require.NoError(t, err)
	require.Equal(t, "ghcr.io/wso2/api-platform/api-control-plane:1.2.3", spec.Images[0].Name)
	require.Equal(t, []builder.CoverageType{builder.GoCoverage}, spec.Coverage.Types)

	commands, err := spec.Plan("/repo", "1.2.3", spec.Coverage)
	require.NoError(t, err)
	require.Len(t, commands, 1)
	require.Equal(t, "/repo/portals/api-control-plane", commands[0].Directory)
	require.Contains(t, commands[0].Args, "ENABLE_COVERAGE=true")
	require.Contains(t, commands[0].Args, "COVERAGE_PACKAGES=api-control-plane-bff/...")

	commands, err = spec.Plan("/repo", "1.2.3", builder.CoverageSpec{})
	require.NoError(t, err)
	require.NotContains(t, commands[0].Args, "ENABLE_COVERAGE=true", "a plain build is not instrumented")
}

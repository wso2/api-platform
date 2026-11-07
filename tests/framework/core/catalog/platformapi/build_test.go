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

package platformapi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/builder"
)

func TestPlatformAPIPlainBuildUsesBakeImageTarget(t *testing.T) {
	spec, err := BuildSpec("test")
	require.NoError(t, err)
	commands, err := spec.Plan("/repo", "test", builder.CoverageSpec{})
	require.NoError(t, err)
	require.Len(t, commands, 1)
	require.Equal(t, "docker buildx bake -f docker-bake.hcl --allow=fs.read=.. "+
		"--set image.tags=ghcr.io/wso2/api-platform/platform-api:test --load image",
		strings.Join(commands[0].Args, " "))
	require.Equal(t, "test", commands[0].Env["VERSION"])
	require.NotContains(t, commands[0].Env, "ENABLE_COVERAGE")
}

func TestPlatformAPICoverageBuildInstrumentsTheBinary(t *testing.T) {
	spec, err := BuildSpec("test")
	require.NoError(t, err)
	commands, err := spec.Plan("/repo", "test", spec.Coverage)
	require.NoError(t, err)
	require.Len(t, commands, 1)
	args := strings.Join(commands[0].Args, " ")
	require.Contains(t, args, "--set coverage.tags=ghcr.io/wso2/api-platform/platform-api:test")
	require.True(t, strings.HasSuffix(args, "--load coverage"), args)
	require.Equal(t, "true", commands[0].Env["ENABLE_COVERAGE"])
	require.Equal(t, "github.com/wso2/api-platform/platform-api/...", commands[0].Env["COVERAGE_PACKAGES"])
}

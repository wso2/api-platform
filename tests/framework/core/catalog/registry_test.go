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

package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for range 8 {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate repository root")
	return ""
}

func TestRegistry(t *testing.T) {
	registry, err := Registry()
	require.NoError(t, err)
	require.Equal(t, len(All()), registry.Len())

	seen := map[string]string{}
	for _, definition := range All() {
		require.NotEmpty(t, definition.Endpoints, definition.Name)
		require.NotEmpty(t, definition.Alias, definition.Name)
		require.Empty(t, seen[definition.Alias])
		seen[definition.Alias] = definition.Name

		for _, endpoint := range definition.Endpoints {
			require.Positive(t, endpoint.Port)
			require.NotEmpty(t, endpoint.Scheme)
		}
		if definition.DB != nil {
			for _, files := range definition.DB.Schema {
				for _, file := range files {
					_, err := os.Stat(filepath.Join(repoRoot(t), file))
					require.NoError(t, err, "%s schema %s", definition.Name, file)
				}
			}
		}
		if definition.Config != nil {
			_, err := os.Stat(filepath.Join(repoRoot(t), definition.Config.BaseConfigPath))
			require.NoError(t, err, definition.Name)
		}
		for _, mount := range definition.Files {
			_, err := os.Stat(filepath.Join(repoRoot(t), mount.HostPath))
			require.NoError(t, err, "%s mount %s", definition.Name, mount.HostPath)
		}
	}
}

func TestBuildSpecsCoverSourceBuiltProducts(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{"platform-gateway", "platform-api", "api-portal", "ai-workspace"} {
		t.Run(name, func(t *testing.T) {
			spec, err := BuildSpec(name, "test-version")
			require.NoError(t, err)
			require.Equal(t, name, spec.Component)
			require.NotEmpty(t, spec.SourceDir)
			require.NotEmpty(t, spec.Images)
			require.NotNil(t, spec.Plan)
			commands, err := spec.Plan(root, "test-version", spec.SupportsCoverage)
			require.NoError(t, err)
			require.NotEmpty(t, commands)
			for _, command := range commands {
				require.NotContains(t, command.Args, "make")
			}
			for _, image := range spec.Images {
				_, err := os.Stat(filepath.Join(root, image.Dockerfile))
				require.NoError(t, err, image.Dockerfile)
			}
		})
	}
}

func TestGatewayBuildPlanUsesNormalTagsForInstrumentedImages(t *testing.T) {
	spec, err := BuildSpec("platform-gateway", "1.2.0-SNAPSHOT")
	require.NoError(t, err)
	commands, err := spec.Plan(repoRoot(t), "1.2.0-SNAPSHOT", true)
	require.NoError(t, err)
	joined := make([]string, 0, len(commands))
	for _, command := range commands {
		joined = append(joined, strings.Join(command.Args, " "))
	}
	plan := strings.Join(joined, "\n")
	require.Contains(t, plan, "ENABLE_COVERAGE=true")
	require.NotContains(t, plan, "-coverage:")
	require.Contains(t, plan, "gateway-controller:1.2.0-SNAPSHOT")
	require.Contains(t, plan, "gateway-runtime:1.2.0-SNAPSHOT")
}

func TestBuildSpecRejectsUnknownProduct(t *testing.T) {
	_, err := BuildSpec("unknown", "test-version")
	require.ErrorContains(t, err, "no source builder")
}

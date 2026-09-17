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
	"github.com/wso2/api-platform/tests/framework/core/builder"
	platformgatewaycatalog "github.com/wso2/api-platform/tests/framework/core/catalog/platformgateway"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/topology"
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
		require.NotEmpty(t, definition.Alias, definition.Name)
		if definition.IsExternal() {
			require.NotEmpty(t, definition.External.Endpoints, definition.Name)
		} else {
			require.NotEmpty(t, definition.Endpoints, definition.Name)
		}
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
			commands, err := spec.Plan(root, "test-version", spec.Coverage)
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
	commands, err := spec.Plan(repoRoot(t), "1.2.0-SNAPSHOT", spec.Coverage)
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

func TestSourceBuildPlansOmitCoverageArgumentsWhenDisabled(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{"platform-gateway", "platform-api", "api-portal", "ai-workspace"} {
		t.Run(name, func(t *testing.T) {
			spec, err := BuildSpec(name, "test-version")
			require.NoError(t, err)
			commands, err := spec.Plan(root, "test-version", builder.CoverageSpec{})
			require.NoError(t, err)
			for _, command := range commands {
				require.NotContains(t, strings.Join(command.Args, " "), "ENABLE_COVERAGE=true")
			}
		})
	}
}

func TestCoverageArgumentsPrecedeDockerContext(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{"platform-gateway", "platform-api", "api-portal", "ai-workspace"} {
		t.Run(name, func(t *testing.T) {
			spec, err := BuildSpec(name, "test-version")
			require.NoError(t, err)
			commands, err := spec.Plan(root, "test-version", spec.Coverage)
			require.NoError(t, err)
			for _, command := range commands {
				contextIndex := -1
				for i, arg := range command.Args {
					if arg == "." {
						contextIndex = i
					}
				}
				if contextIndex >= 0 {
					require.NotContains(t, command.Args[contextIndex+1:], "--build-arg")
				}
			}
		})
	}
}

func TestBuildSpecRejectsUnknownProduct(t *testing.T) {
	_, err := BuildSpec("unknown", "test-version")
	require.ErrorContains(t, err, "no source builder")
}

func TestSourceProductsResolveVersionsForEveryBlock(t *testing.T) {
	resolved := &topology.Resolved{Blocks: []topology.ResolvedBlock{
		{Components: []topology.ResolvedComponent{
			{Def: &components.Definition{Name: "platform-gateway"}},
		}},
		{Components: []topology.ResolvedComponent{
			{Def: &components.Definition{Name: "platform-gateway"}},
		}},
	}}

	products, err := sourceProducts(resolved)
	require.NoError(t, err)
	require.Len(t, products, 1)
	require.NotEmpty(t, resolved.Blocks[0].Components[0].Version)
	require.Equal(t, resolved.Blocks[0].Components[0].Version,
		resolved.Blocks[1].Components[0].Version)
}

func TestPolicyProductsResolveSourceAndVersionedBuilds(t *testing.T) {
	source := "../gateway-controllers/policies"
	t.Run("source build", func(t *testing.T) {
		resolved := &topology.Resolved{Blocks: []topology.ResolvedBlock{{Components: []topology.ResolvedComponent{
			{Def: platformgatewaycatalog.PlatformGateway(), AddPoliciesFrom: source},
		}}}}

		products, err := policyProducts(resolved)
		require.NoError(t, err)
		require.Len(t, products, 1)
		require.True(t, products[0].buildFromSource)
		require.Equal(t, source, products[0].source)
		require.NotEmpty(t, products[0].version)
		require.Equal(t, products[0].version, resolved.Blocks[0].Components[0].Version)
		require.True(t, resolved.Blocks[0].Components[0].BuildFromSource)

		products, err = policyProducts(resolved)
		require.NoError(t, err)
		require.Len(t, products, 1)
		require.True(t, products[0].buildFromSource,
			"source-build mode must remain stable after the source version is stored")
	})

	t.Run("versioned extension", func(t *testing.T) {
		version := "legacy"
		resolved := &topology.Resolved{Blocks: []topology.ResolvedBlock{{Components: []topology.ResolvedComponent{
			{Def: platformgatewaycatalog.PlatformGateway().WithImageVersion(version), Version: version, AddPoliciesFrom: source},
		}}}}

		products, err := policyProducts(resolved)
		require.NoError(t, err)
		require.Len(t, products, 1)
		require.False(t, products[0].buildFromSource)
		controller, runtime, err := platformGatewayBaseImages(resolved, products[0])
		require.NoError(t, err)
		require.Equal(t, "ghcr.io/wso2/api-platform/gateway-controller:legacy", controller)
		require.Equal(t, "ghcr.io/wso2/api-platform/gateway-runtime:legacy", runtime)

		setPlatformGatewayImages(resolved, products[0], platformgatewaycatalog.DerivedImages{
			Controller: "local/controller:custom", Runtime: "local/runtime:custom",
		})
		require.Equal(t, "local/controller:custom", resolved.Blocks[0].Components[0].Def.Compose.Env[platformgatewaycatalog.EnvImagePGController])
		require.Equal(t, "local/runtime:custom", resolved.Blocks[0].Components[0].Def.Compose.Env[platformgatewaycatalog.EnvImagePGRuntime])
	})
}

func TestPolicyBuildsMatchComponentsByBuildMode(t *testing.T) {
	source := "../gateway-controllers/policies"
	version, ok := shared.SourceVersion("platform-gateway")
	require.True(t, ok)
	resolved := &topology.Resolved{Blocks: []topology.ResolvedBlock{{Components: []topology.ResolvedComponent{
		{Def: platformgatewaycatalog.PlatformGateway(), BuildFromSource: true, AddPoliciesFrom: source},
		{Def: platformgatewaycatalog.PlatformGateway().WithImageVersion(version), Version: version, AddPoliciesFrom: source},
	}}}}

	products, err := policyProducts(resolved)
	require.NoError(t, err)
	require.Len(t, products, 2)
	require.True(t, products[0].buildFromSource)
	require.False(t, products[1].buildFromSource)
	require.Equal(t, source, products[0].source)
	require.Equal(t, source, products[1].source)
	require.Equal(t, version, products[0].version)
	require.Equal(t, version, products[1].version)

	baseController, baseRuntime, err := platformGatewayBaseImages(resolved, products[1])
	require.NoError(t, err)
	require.Equal(t, "ghcr.io/wso2/api-platform/gateway-controller:"+version, baseController)
	require.Equal(t, "ghcr.io/wso2/api-platform/gateway-runtime:"+version, baseRuntime)

	sourceImages := platformgatewaycatalog.DerivedImages{
		Controller: "local/controller:source",
		Runtime:    "local/runtime:source",
	}
	setPlatformGatewayImages(resolved, products[0], sourceImages)
	require.Equal(t, sourceImages.Controller,
		resolved.Blocks[0].Components[0].Def.Compose.Env[platformgatewaycatalog.EnvImagePGController])
	require.Equal(t, sourceImages.Runtime,
		resolved.Blocks[0].Components[0].Def.Compose.Env[platformgatewaycatalog.EnvImagePGRuntime])
	require.Equal(t, baseController,
		resolved.Blocks[0].Components[1].Def.Compose.Env[platformgatewaycatalog.EnvImagePGController])
	require.Equal(t, baseRuntime,
		resolved.Blocks[0].Components[1].Def.Compose.Env[platformgatewaycatalog.EnvImagePGRuntime])

	versionedImages := platformgatewaycatalog.DerivedImages{
		Controller: "local/controller:versioned",
		Runtime:    "local/runtime:versioned",
	}
	setPlatformGatewayImages(resolved, products[1], versionedImages)
	require.Equal(t, sourceImages.Controller,
		resolved.Blocks[0].Components[0].Def.Compose.Env[platformgatewaycatalog.EnvImagePGController])
	require.Equal(t, sourceImages.Runtime,
		resolved.Blocks[0].Components[0].Def.Compose.Env[platformgatewaycatalog.EnvImagePGRuntime])
	require.Equal(t, versionedImages.Controller,
		resolved.Blocks[0].Components[1].Def.Compose.Env[platformgatewaycatalog.EnvImagePGController])
	require.Equal(t, versionedImages.Runtime,
		resolved.Blocks[0].Components[1].Def.Compose.Env[platformgatewaycatalog.EnvImagePGRuntime])
}

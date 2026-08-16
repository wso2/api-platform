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

package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestGatewayVersionResolvesFromVERSIONFile asserts the version is genuinely READ, not
// silently falling back to the constant.
//
// Without this the fallback could mask a broken resolver: a wrong repo-root walk would return
// gatewayVersionFallback, which happens to be right today and would go on being right until
// the next release bump, at which point every suite would pull a tag that does not exist.
func TestGatewayVersionResolvesFromVERSIONFile(t *testing.T) {
	root, ok := RepoRootFromCallerFile()
	require.True(t, ok,
		"could not locate the repository root from this package's source path — the resolver "+
			"in version.go is broken, and gatewayVersion() is silently returning its fallback")

	raw, err := os.ReadFile(filepath.Join(root, "gateway", "VERSION"))
	require.NoError(t, err)
	onDisk := strings.TrimSpace(string(raw))
	require.NotEmpty(t, onDisk, "gateway/VERSION is empty")

	require.Equal(t, onDisk, gatewayVersion(),
		"gatewayVersion() returned %q but gateway/VERSION says %q", gatewayVersion(), onDisk)
}

// TestGatewayBuildYAMLAgreesWithVERSION guards the product's own two copies of the version.
//
// gateway/VERSION is what the Makefile tags images with; gateway/build.yaml carries
// gateway.version for the policy builder. Nothing in the product forces them to agree, so if
// they drift, `make build` produces one tag while build.yaml describes another — and this
// framework, which pins the tag, is where that shows up as a confusing image-not-found.
//
// Reported as a skip rather than a failure when build.yaml is absent or unparseable: this is a
// cross-check on someone else's file, and it should not fail this framework's test run over a
// product-side change in an unrelated format.
func TestGatewayBuildYAMLAgreesWithVERSION(t *testing.T) {
	root, ok := RepoRootFromCallerFile()
	require.True(t, ok)

	buildFile := filepath.Join(root, "gateway", "build.yaml")
	raw, err := os.ReadFile(buildFile)
	if err != nil {
		t.Skipf("no readable gateway/build.yaml to cross-check (%v)", err)
	}

	var parsed struct {
		Gateway struct {
			Version string `yaml:"version"`
		} `yaml:"gateway"`
	}
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Skipf("gateway/build.yaml is not in the expected shape (%v)", err)
	}
	if parsed.Gateway.Version == "" {
		t.Skip("gateway/build.yaml declares no gateway.version")
	}

	require.Equal(t, gatewayVersion(), parsed.Gateway.Version,
		"gateway/VERSION says %q but gateway/build.yaml says %q — `make build` tags images "+
			"from VERSION, so the suites would pin a tag build.yaml disagrees with",
		gatewayVersion(), parsed.Gateway.Version)
}

// TestGatewayImagesAreNotCoverageBuilds pins gateway execution to the normal image names.
func TestGatewayRunImagesUseNormalNames(t *testing.T) {
	t.Setenv(EnvCoverageMode, "")
	for _, ref := range []string{GatewayControllerRunImage(), GatewayRuntimeRunImage()} {
		require.NotContains(t, ref, "-coverage",
			"%s is a coverage-instrumented build; a default run collects no server-side "+
				"coverage, so it would pay the slowest build in the pipeline for nothing", ref)
		require.Contains(t, ref, ":"+gatewayVersion(),
			"%s is not pinned to gateway/VERSION", ref)
		require.NotContains(t, ref, ":latest",
			"%s uses :latest, which also exists in the registry — a developer who had not run "+
				"`make build` would silently test a released image instead of their own code", ref)
	}
}

// TestGatewayImagesInCoverageMode asserts coverage mode keeps the normal image references.
func TestGatewayRunImagesRemainStableInCoverageMode(t *testing.T) {
	t.Setenv(EnvCoverageMode, "true")
	require.Equal(t, GatewayControllerImage(), GatewayControllerRunImage())
	require.Equal(t, GatewayRuntimeImage(), GatewayRuntimeRunImage())
}

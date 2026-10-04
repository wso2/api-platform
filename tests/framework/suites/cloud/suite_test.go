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

package cloud_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"

	frameworkbuilder "github.com/wso2/api-platform/tests/framework/core/builder"
	"github.com/wso2/api-platform/tests/framework/core/catalog"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/topology"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	cloudsteps "github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudconsole"
)

var selection topology.Selection

func TestMain(m *testing.M) {
	selection.Flags(flag.CommandLine)
	flag.Parse()

	coverageMode := "false"
	if selection.Coverage {
		coverageMode = "true"
	}
	if err := os.Setenv(shared.EnvCoverageMode, coverageMode); err != nil {
		fmt.Fprintln(os.Stderr, "setting coverage mode:", err)
		os.Exit(1)
	}

	if blocks, runners, ok := suiteShape(); ok {
		if to, raised := frameworkruntime.EnsureParallelBudget(blocks, runners); raised {
			fmt.Fprintln(os.Stderr, frameworkruntime.ParallelBudgetNotice(runtime.GOMAXPROCS(0), to))
		}
	}

	os.Exit(m.Run())
}

func suiteShape() (blocks, maxRunners int, ok bool) {
	dir, err := os.Getwd()
	if err != nil {
		return 0, 0, false
	}
	return suiteShapeAt(dir, selection)
}

func suiteShapeAt(dir string, selected topology.Selection) (blocks, maxRunners int, ok bool) {
	registry, err := catalog.Registry()
	if err != nil {
		return 0, 0, false
	}
	resolved, err := topology.LoadFile(filepath.Join(dir, "cloud-suite.yaml"), registry)
	if err != nil {
		return 0, 0, false
	}
	narrowed, err := selected.Apply(resolved)
	if err != nil {
		return 0, 0, false
	}
	for i := range narrowed.Blocks {
		if p := narrowed.Blocks[i].EffectiveParallel(); p > maxRunners {
			maxRunners = p
		}
	}
	return len(narrowed.Blocks), maxRunners, true
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("locating the working directory: %v", err)
	}
	for range 8 {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Skip("could not locate the repository root")
	return ""
}

// TestCloudSuite runs the externally hosted cloud suite declared in YAML.
func TestCloudSuite(t *testing.T) {
	if selection.CloudEnvironment == "" {
		t.Skip("cloud suite requires -cloud-env; external cloud coverage is opt-in")
	}
	root := repoRoot(t)
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("locating the suite directory: %v", err)
	}

	registry, err := catalog.Registry()
	if err != nil {
		t.Fatalf("building the component registry: %v", err)
	}
	resolved, err := topology.LoadFile(filepath.Join(dir, "cloud-suite.yaml"), registry)
	if err != nil {
		t.Fatalf("loading the suite: %v", err)
	}
	if err := topology.ValidateFeatureFiles(resolved, dir); err != nil {
		t.Fatalf("validating feature files: %v", err)
	}
	narrowed, err := selection.Apply(resolved)
	if err != nil {
		t.Fatalf("applying the selection: %v", err)
	}
	for _, skipped := range narrowed.SkippedRunners {
		t.Logf("skipped runner %s/%s: %s", skipped.Block, skipped.Runner, skipped.Reason)
	}
	for _, skipped := range narrowed.SkippedBlocks {
		t.Logf("skipped block %s: %s", skipped.Block, skipped.Reason)
	}
	if err := catalog.BuildSources(context.Background(), narrowed, root, frameworkbuilder.ExecRunner{}, selection.Coverage); err != nil {
		t.Fatalf("building source images: %v", err)
	}

	frameworkruntime.Run(t, narrowed, frameworkruntime.Deps{
		RepoRoot:    root,
		FeatureRoot: dir,
		Steps: func(topo *frameworkruntime.Topology) (frameworkruntime.StepRegistrar, error) {
			timeout := topo.PropagationTimeout
			if timeout <= 0 {
				timeout = retry.PropagationCeiling
			}
			client := httpx.NewClient(httpx.Options{
				Timeout:    timeout,
				MaxRetries: 3,
				RetryDelay: 2 * time.Second,
			})
			funnel := httpx.NewFunnel(client, 3, 2*time.Second)
			return func(sc *godog.ScenarioContext) {
				cloudsteps.Register(sc, topo, funnel)
			}, nil
		},
		CleanupDeleters: func(reg *cleanup.Registry, topo *frameworkruntime.Topology) {
			if err := cloudsteps.RegisterDeleters(reg, topo); err != nil {
				t.Fatalf("registering cloud cleanup handlers: %v", err)
			}
		},
	})
}

func TestSuiteShapeReturnsResolvedBlockAndRunnerCounts(t *testing.T) {
	dir := t.TempDir()
	fixture := `
suite: fixture-suite

blocks:
  - name: apip-cloud
    components:
      - name: cloud-console
    runners:
      - name: apip-basic
        features: [features/apip_basic.feature]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cloud-suite.yaml"), []byte(fixture), 0o644))

	blocks, maxRunners, ok := suiteShapeAt(dir, topology.Selection{CloudEnvironment: "development"})
	require.True(t, ok)
	require.Equal(t, 1, blocks)
	require.Equal(t, 1, maxRunners)
}

func TestCloudSuiteUsesFrameworkTimeoutDefaults(t *testing.T) {
	dir, err := os.Getwd()
	require.NoError(t, err)
	registry, err := catalog.Registry()
	require.NoError(t, err)
	resolved, err := topology.LoadFile(filepath.Join(dir, "cloud-suite.yaml"), registry)
	require.NoError(t, err)
	require.Equal(t, 5*time.Minute, resolved.Timeouts.Boot)
	require.Equal(t, 60*time.Second, resolved.Timeouts.Propagation)
}

func TestSuiteShapeRejectsMissingSuiteFile(t *testing.T) {
	blocks, maxRunners, ok := suiteShapeAt(t.TempDir(), topology.Selection{CloudEnvironment: "development"})
	require.False(t, ok)
	require.Zero(t, blocks)
	require.Zero(t, maxRunners)
}

func TestRepoRootFindsWorkspaceRoot(t *testing.T) {
	root := repoRoot(t)
	_, err := os.Stat(filepath.Join(root, "go.work"))
	require.NoError(t, err)
}

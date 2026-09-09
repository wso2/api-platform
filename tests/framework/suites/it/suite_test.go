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

package it_test

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	frameworkbuilder "github.com/wso2/api-platform/tests/framework/core/builder"
	"github.com/wso2/api-platform/tests/framework/core/catalog"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/coverage"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/topology"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/suites/it/steps"
)

// selection is populated from flags, so one suite file can be sharded across CI jobs.
var selection topology.Selection

func TestMain(m *testing.M) {
	selection.Flags(flag.CommandLine)
	flag.Parse()

	// Set coverage mode before catalog definitions are loaded.
	coverageMode := "false"
	if selection.Coverage {
		coverageMode = "true"
	}
	if err := os.Setenv(shared.EnvCoverageMode, coverageMode); err != nil {
		fmt.Fprintln(os.Stderr, "setting coverage mode:", err)
		os.Exit(1)
	}

	// Set the test parallel budget before subtests are scheduled.
	if blocks, runners, ok := suiteShape(); ok {
		if to, raised := frameworkruntime.EnsureParallelBudget(blocks, runners); raised {
			fmt.Fprintln(os.Stderr, frameworkruntime.ParallelBudgetNotice(runtime.GOMAXPROCS(0), to))
		}
	}

	os.Exit(m.Run())
}

// suiteShape returns the resolved block count and largest runner concurrency.
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
	resolved, err := topology.LoadFile(filepath.Join(dir, "it-suite.yaml"), registry)
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

// repoRoot locates the checkout containing go.work.
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

func suiteDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("locating the working directory: %v", err)
	}
	return dir
}

// TestIntegrationSuite runs the integration suite declared in YAML.
func TestIntegrationSuite(t *testing.T) {
	root := repoRoot(t)
	dir := suiteDir(t)

	registry, err := catalog.Registry()
	if err != nil {
		t.Fatalf("building the component registry: %v", err)
	}

	resolved, err := topology.LoadFile(filepath.Join(dir, "it-suite.yaml"), registry)
	if err != nil {
		t.Fatalf("loading the suite: %v", err)
	}

	// Feature files are validated before anything boots. A missing feature would otherwise
	// surface as a runner reporting zero scenarios and PASSING.
	if err := topology.ValidateFeatureFiles(resolved, dir); err != nil {
		t.Fatalf("validating feature files: %v", err)
	}

	narrowed, err := selection.Apply(resolved)
	if err != nil {
		t.Fatalf("applying the selection: %v", err)
	}
	if err := catalog.BuildSources(context.Background(), narrowed, root, frameworkbuilder.ExecRunner{}, selection.Coverage); err != nil {
		t.Fatalf("building source images: %v", err)
	}

	// One sink per run, wiped on creation so counters never merge across runs. Built here
	// and not in the engine because only the suite knows its own directory and whether the
	// images are instrumented.
	var sink *coverage.Sink
	if selection.Coverage {
		out := os.Getenv(coverage.EnvOut)
		if out == "" {
			out = filepath.Join(dir, "coverage-out")
		}
		sink, err = coverage.NewSink(out)
		if err != nil {
			t.Fatalf("preparing the coverage sink: %v", err)
		}
		t.Logf("coverage: collecting counters into %s", sink.Root())
	}

	frameworkruntime.Run(t, narrowed, frameworkruntime.Deps{
		RepoRoot:    root,
		FeatureRoot: dir,
		Coverage:    sink,
		Steps: func(sc *godog.ScenarioContext, topo *frameworkruntime.Topology) {
			steps.New(topo, dir).Register(sc)
		},
		CleanupDeleters: func(reg *cleanup.Registry, topo *frameworkruntime.Topology) {
			registerDeleters(reg, topo)
		},
	})
}

// TestSuiteShapeReturnsResolvedBlockAndRunnerCounts checks the resolution math itself —
// database-matrix expansion and EffectiveParallel — against a small fixture, not the
// checked-in it-suite.yaml. Asserting against the real suite would break on every routine
// topology edit (a runner added, a block's DB matrix changed) for reasons unrelated to
// whether the resolution logic is correct, and would silently reflect whatever -blocks/
// -feature-tags flags the test binary happened to be invoked with, since suiteShape()
// reads the package-level selection populated in TestMain.
func TestSuiteShapeReturnsResolvedBlockAndRunnerCounts(t *testing.T) {
	dir := t.TempDir()
	fixture := `
suite: fixture-suite

defaults:
  parallel: 3
  components:
    platform-gateway:
      db: sqlite
  timeouts:
    boot: 5m
    propagation: 180s

blocks:
  - name: block-a
    parallel: 5
    components:
      - name: platform-gateway
        db:
          matrix: [sqlite, postgres]
    runners:
      - name: runner-a
        features:
          - features/does-not-need-to-exist-a.feature

  - name: block-b
    parallel: 3
    components:
      - name: platform-gateway
        db:
          matrix: [sqlite, postgres]
    runners:
      - name: runner-b
        features:
          - features/does-not-need-to-exist-b.feature
`
	if err := os.WriteFile(filepath.Join(dir, "it-suite.yaml"), []byte(fixture), 0o644); err != nil {
		t.Fatalf("writing fixture suite: %v", err)
	}

	blocks, maxRunners, ok := suiteShapeAt(dir, topology.Selection{})

	if !ok {
		t.Fatal("suiteShapeAt() should load the fixture suite")
	}
	// 2 blocks x 2 database variants each = 4 resolved blocks.
	if blocks != 4 {
		t.Fatalf("suiteShapeAt() blocks = %d, want 4 resolved database variants", blocks)
	}
	// The larger of the two blocks' EffectiveParallel (block-a's 5 vs block-b's 3).
	if maxRunners != 5 {
		t.Fatalf("suiteShapeAt() max runners = %d, want 5", maxRunners)
	}
}

func TestSuiteShapeRejectsMissingSuiteFile(t *testing.T) {
	blocks, maxRunners, ok := suiteShapeAt(t.TempDir(), topology.Selection{})

	if ok || blocks != 0 || maxRunners != 0 {
		t.Fatalf("suiteShapeAt() = (%d, %d, %t), want (0, 0, false)", blocks, maxRunners, ok)
	}
}

func TestRepoRootFindsWorkspaceRoot(t *testing.T) {
	root := repoRoot(t)

	if _, err := os.Stat(filepath.Join(root, "go.work")); err != nil {
		t.Fatalf("repoRoot() = %q, expected go.work: %v", root, err)
	}
}

func TestBasicAuthForUsesTopologyCredentials(t *testing.T) {
	topo := &frameworkruntime.Topology{Admin: actor.Credentials{
		Username: "admin", Password: "secret",
	}}

	got := basicAuthFor(topo)
	want := steps.BasicAuthHeader("admin", "secret")
	if got != want {
		t.Fatalf("basicAuthFor() = %q, want %q", got, want)
	}
}

// registerDeleters configures cleanup handlers for gateway resources.
func registerDeleters(reg *cleanup.Registry, topo *frameworkruntime.Topology) {
	client := httpx.NewClient(httpx.Options{MaxRetries: 1})

	reg.RegisterDeleter(cleanup.KindAPI, func(ctx context.Context, res cleanup.Resource) error {
		base, err := topo.URL("platform-gateway", "rest")
		if err != nil {
			return err
		}

		resp, err := client.Do(ctx, httpx.Request{
			Method: http.MethodDelete,
			URL:    base + steps.ManagementBasePath + "/rest-apis/" + res.ID,
			Headers: map[string]string{
				"Authorization": basicAuthFor(topo),
			},
		}, 1, 0)
		if err != nil {
			return err
		}
		// A 404 means it is already gone, which is success for a sweep. Anything else is a
		// real leak signal and is reported.
		if resp.StatusCode == http.StatusNotFound || resp.Succeeded() {
			return nil
		}
		return errFromResponse(resp)
	})

	registerControllerDeleter(reg, topo, client, cleanup.KindLLMProvider, "/llm-providers")
	registerControllerDeleter(reg, topo, client, cleanup.KindLLMProxy, "/llm-proxies")
	registerControllerDeleter(reg, topo, client, cleanup.KindLLMProviderTemplate, "/llm-provider-templates")
	registerControllerDeleter(reg, topo, client, cleanup.KindMCPProxy, "/mcp-proxies")
	registerControllerDeleter(reg, topo, client, cleanup.KindCertificate, "/certificates")
	registerControllerDeleter(reg, topo, client, cleanup.KindSecret, "/secrets")
}

func registerControllerDeleter(
	reg *cleanup.Registry,
	topo *frameworkruntime.Topology,
	client *httpx.Client,
	kind cleanup.Kind,
	collection string,
) {
	reg.RegisterDeleter(kind, func(ctx context.Context, res cleanup.Resource) error {
		base, err := topo.URL("platform-gateway", "rest")
		if err != nil {
			return err
		}
		resp, err := client.Do(ctx, httpx.Request{
			Method: http.MethodDelete,
			URL:    base + steps.ManagementBasePath + collection + "/" + res.ID,
			Headers: map[string]string{
				"Authorization": basicAuthFor(topo),
			},
		}, 1, 0)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusNotFound || resp.Succeeded() {
			return nil
		}
		return errFromResponse(resp)
	})
}

func basicAuthFor(topo *frameworkruntime.Topology) string {
	return steps.BasicAuthHeader(topo.Admin.Username, topo.Admin.Password)
}

func errFromResponse(resp *httpx.Response) error {
	return &deleteError{resp: resp}
}

type deleteError struct{ resp *httpx.Response }

func (e *deleteError) Error() string { return e.resp.Describe() }

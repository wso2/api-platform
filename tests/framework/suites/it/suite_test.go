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

	// The catalog reads coverage mode from the environment when its definitions are built,
	// which happens on every catalog.Registry() call below and in the test proper — so the
	// flag must land in the environment before either.
	coverageMode := "false"
	if selection.Coverage {
		coverageMode = "true"
	}
	if err := os.Setenv(shared.EnvCoverageMode, coverageMode); err != nil {
		fmt.Fprintln(os.Stderr, "setting coverage mode:", err)
		os.Exit(1)
	}

	// The parallel budget MUST be settled here, before m.Run: the testing package reads
	// -parallel once when the run starts, so adjusting it from inside a test has no effect.
	//
	// Blocks and runners are both parallel subtests, and a parent holds its slot while waiting
	// for children. Below the required budget the runners can never be scheduled, so the suite
	// hangs after booting its topologies — containers alive, nothing running, no error. See
	// frameworkruntime.EnsureParallelBudget.
	//
	// Best effort by design: a suite that cannot be loaded here is not this function's problem
	// to report. TestGatewaySuite loads it again and fails with a proper message.
	if blocks, runners, ok := suiteShape(); ok {
		if to, raised := frameworkruntime.EnsureParallelBudget(blocks, runners); raised {
			fmt.Fprintln(os.Stderr, frameworkruntime.ParallelBudgetNotice(runtime.GOMAXPROCS(0), to))
		}
	}

	os.Exit(m.Run())
}

// suiteShape counts resolved blocks and the largest runner concurrency any block asks for.
//
// Resolved, not raw: a matrix block expands into one block per combination, and counting the
// declaration would undercount exactly the suites most likely to exhaust the budget.
func suiteShape() (blocks, maxRunners int, ok bool) {
	dir, err := os.Getwd()
	if err != nil {
		return 0, 0, false
	}
	registry, err := catalog.Registry()
	if err != nil {
		return 0, 0, false
	}
	resolved, err := topology.LoadFile(filepath.Join(dir, "it-suite.yaml"), registry)
	if err != nil {
		return 0, 0, false
	}
	for i := range resolved.Blocks {
		if p := resolved.Blocks[i].EffectiveParallel(); p > maxRunners {
			maxRunners = p
		}
	}
	return len(resolved.Blocks), maxRunners, true
}

// repoRoot walks up to the checkout, so the suite runs from any working directory.
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

// TestGatewaySuite is the whole pilot: one call, everything else declared in YAML.
//
// This function is the ENTIRE Go surface a suite author touches. Containers, databases,
// credentials, scoping, cleanup and both levels of concurrency are the framework's concern
// and appear nowhere here.
//
// There is deliberately no "skip if docker is unavailable" guard. A CI job whose docker
// daemon failed to start would otherwise report GREEN having tested nothing — the worst
// outcome a suite can produce, and the same reason a block that fails to boot is fatal
// rather than skipped. Without docker this fails loudly at the first container start.
func TestGatewaySuite(t *testing.T) {
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
			steps.New(topo).Register(sc)
		},
		CleanupDeleters: func(reg *cleanup.Registry, topo *frameworkruntime.Topology) {
			registerDeleters(reg, topo)
		},
	})
}

// registerDeleters teaches cleanup how to remove each resource kind.
//
// Registered per runner with the topology in hand, because deletion needs the running
// gateway's address — which is not knowable until it is up.
func registerDeleters(reg *cleanup.Registry, topo *frameworkruntime.Topology) {
	client := httpx.NewClient(httpx.Options{MaxRetries: 1})

	reg.RegisterDeleter(cleanup.KindAPI, func(ctx context.Context, res cleanup.Resource) error {
		base, err := topo.URL("platform-gateway", "rest")
		if err != nil {
			return err
		}

		user, _ := topo.Shared.Block(), "" // block name unused; credentials come from topology
		_ = user

		resp, err := client.Do(ctx, httpx.Request{
			Method: http.MethodDelete,
			URL:    base + "/api/management/v1/rest-apis/" + res.ID,
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
			URL:    base + "/api/management/v1" + collection + "/" + res.ID,
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

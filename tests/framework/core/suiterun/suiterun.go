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

package suiterun

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	frameworkbuilder "github.com/wso2/api-platform/tests/framework/core/builder"
	"github.com/wso2/api-platform/tests/framework/core/catalog"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/coverage"
	"github.com/wso2/api-platform/tests/framework/core/logcapture"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/topology"
)

// DefaultSuiteFile is the suite declaration read from Options.Dir when none is named.
const DefaultSuiteFile = "it-suite.yaml"

// workspaceMarkers identify the root of the project a suite runs in, nearest first.
var workspaceMarkers = []string{"go.work", ".git"}

// Options describes one suite.
type Options struct {
	// Dir holds the suite file and is the root feature paths resolve against. It defaults to
	// the working directory, which `go test` sets to the test package's directory.
	Dir string

	// SuiteFile names the suite declaration inside Dir. It defaults to DefaultSuiteFile.
	SuiteFile string

	// WorkspaceRoot is the project checkout. Relative policy trees and project files resolve
	// against it, and files the framework itself owns fall back to its embedded copies. It
	// defaults to the nearest ancestor of Dir that holds a go.work file or a .git entry.
	WorkspaceRoot string

	// Steps constructs the runner-isolated step definitions. It is required.
	Steps func(topo *frameworkruntime.Topology, featureRoot string) (frameworkruntime.StepRegistrar, error)

	// CleanupDeleters registers how each resource kind is deleted. It is optional.
	CleanupDeleters func(reg *cleanup.Registry, topo *frameworkruntime.Topology)
}

// state is populated from flags by Main so one suite file can be sharded across CI jobs.
var state struct {
	selection   topology.Selection
	logsEnabled bool
}

// Main parses the suite flags, prepares the process, runs the tests and exits. Call it from
// TestMain.
func Main(m *testing.M, opts Options) {
	state.selection.Flags(flag.CommandLine)
	flag.BoolVar(&state.logsEnabled, "logs", false,
		"capture every block's combined container output to files under IT_LOG_OUT")
	flag.Parse()

	// Coverage mode is read while catalog definitions are built, so it is set first.
	coverageMode := "false"
	if state.selection.Coverage {
		coverageMode = "true"
	}
	if err := os.Setenv(shared.EnvCoverageMode, coverageMode); err != nil {
		fmt.Fprintln(os.Stderr, "setting coverage mode:", err)
		os.Exit(1)
	}

	// The parallel budget must be raised before subtests are scheduled.
	if blocks, runners, ok := shapeOf(opts); ok {
		if to, raised := frameworkruntime.EnsureParallelBudget(blocks, runners); raised {
			fmt.Fprintln(os.Stderr, frameworkruntime.ParallelBudgetNotice(runtime.GOMAXPROCS(0), to))
		}
	}

	code := m.Run()
	// Shared components outlive every block and are reaped at process exit, so nothing
	// else flushes their log files.
	frameworkruntime.CloseSharedLogs()
	os.Exit(code)
}

// Selection returns the selection parsed from the command line by Main.
func Selection() topology.Selection { return state.selection }

// ShapeAt returns the resolved block count and largest runner concurrency of the suite
// declared in dir, after applying selected.
func ShapeAt(dir, suiteFile string, selected topology.Selection) (blocks, maxRunners int, ok bool) {
	registry, err := catalog.Registry()
	if err != nil {
		return 0, 0, false
	}
	resolved, err := topology.LoadFile(filepath.Join(dir, suiteName(suiteFile)), registry)
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

func shapeOf(opts Options) (blocks, maxRunners int, ok bool) {
	dir, err := suiteDir(opts)
	if err != nil {
		return 0, 0, false
	}
	selected := state.selection
	if selected.RepoRoot == "" {
		root, err := workspaceRoot(opts, dir)
		if err != nil {
			return 0, 0, false
		}
		selected.RepoRoot = root
	}
	return ShapeAt(dir, opts.SuiteFile, selected)
}

// Run executes the suite declared by opts. Call it from a test function.
func Run(t *testing.T, opts Options) {
	t.Helper()
	if opts.Steps == nil {
		t.Fatal("suiterun: Options.Steps is required")
	}
	dir, err := suiteDir(opts)
	if err != nil {
		t.Fatalf("locating the suite directory: %v", err)
	}
	root, err := workspaceRoot(opts, dir)
	if err != nil {
		t.Fatalf("locating the workspace root: %v", err)
	}
	selection := state.selection
	if selection.RepoRoot == "" {
		selection.RepoRoot = root
	}

	registry, err := catalog.Registry()
	if err != nil {
		t.Fatalf("building the component registry: %v", err)
	}

	resolved, err := topology.LoadFile(filepath.Join(dir, suiteName(opts.SuiteFile)), registry)
	if err != nil {
		t.Fatalf("loading the suite: %v", err)
	}

	// Feature files are validated before anything boots. A missing feature would otherwise
	// surface as a runner reporting zero scenarios and PASSING.
	if err := topology.ValidateFeatureFiles(resolved, dir); err != nil {
		t.Fatalf("validating feature files: %v", err)
	}

	narrowed, err := selection.Apply(resolved)
	if errors.Is(err, topology.ErrNothingToRun) {
		t.Skipf("nothing to run: %v", err)
	}
	if err != nil {
		t.Fatalf("applying the selection: %v", err)
	}
	// Runners resolved from a policy tree only exist after selection, so their features are
	// checked here too.
	if err := topology.ValidateFeatureFiles(narrowed, dir); err != nil {
		t.Fatalf("validating selected feature files: %v", err)
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

	// One sink per run, mirroring the coverage sink above: built here because only the
	// suite knows its own directory, and wiped on creation so a stale local run's log
	// files are never mixed with a fresh run's.
	var logs *logcapture.Sink
	if state.logsEnabled {
		out := os.Getenv(logcapture.EnvOut)
		if out == "" {
			out = filepath.Join(dir, "logs-out")
		}
		logs, err = logcapture.NewSink(out)
		if err != nil {
			t.Fatalf("preparing the log capture sink: %v", err)
		}
		t.Logf("logcapture: collecting container output into %s", logs.Root())
	}

	frameworkruntime.Run(t, narrowed, frameworkruntime.Deps{
		RepoRoot:    root,
		FeatureRoot: dir,
		Coverage:    sink,
		Logs:        logs,
		Steps: func(topo *frameworkruntime.Topology) (frameworkruntime.StepRegistrar, error) {
			return opts.Steps(topo, dir)
		},
		CleanupDeleters: opts.CleanupDeleters,
	})
}

func suiteName(name string) string {
	if name == "" {
		return DefaultSuiteFile
	}
	return name
}

func suiteDir(opts Options) (string, error) {
	if opts.Dir != "" {
		return opts.Dir, nil
	}
	return os.Getwd()
}

// workspaceRoot returns the configured root, or the nearest ancestor of dir that holds a
// workspace marker.
func workspaceRoot(opts Options, dir string) (string, error) {
	if opts.WorkspaceRoot != "" {
		return opts.WorkspaceRoot, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return FindWorkspaceRoot(abs)
}

// FindWorkspaceRoot returns the nearest directory at or above dir that holds a go.work file
// or a .git entry.
func FindWorkspaceRoot(dir string) (string, error) {
	for current := dir; ; current = filepath.Dir(current) {
		for _, marker := range workspaceMarkers {
			if _, err := os.Stat(filepath.Join(current, marker)); err == nil {
				return current, nil
			}
		}
		if parent := filepath.Dir(current); parent == current {
			return "", fmt.Errorf("no go.work or .git found at or above %q; set Options.WorkspaceRoot", dir)
		}
	}
}

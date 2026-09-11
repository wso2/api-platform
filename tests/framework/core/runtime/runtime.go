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

package runtime

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/coverage"
	"github.com/wso2/api-platform/tests/framework/core/logcapture"
	"github.com/wso2/api-platform/tests/framework/core/topology"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

// StepRegistrar wires a suite's step definitions to a running topology.
type StepRegistrar func(sc *godog.ScenarioContext, topo *Topology)

// Deps are what the engine needs from the suite.
type Deps struct {
	// RepoRoot resolves repo-relative paths.
	RepoRoot string

	// Steps registers step definitions.
	Steps StepRegistrar

	// FeatureRoot resolves the feature paths in the suite file.
	FeatureRoot string

	// Logger receives engine diagnostics.
	Logger *slog.Logger

	// CleanupDeleters registers how to delete each resource kind. Called per runner, so
	// each gets a registry wired with the suite's deleters.
	CleanupDeleters func(*cleanup.Registry, *Topology)

	// Coverage, when non-nil, makes every block harvest its containers' coverage
	// counters at teardown. Nil in a default run — the suite decides, because only it
	// knows whether the images are instrumented.
	Coverage *coverage.Sink

	// Logs, when non-nil, makes every block stream its containers' combined stdout/stderr
	// into one file for the block's whole lifetime. Nil in a default run — the suite
	// decides whether container output is being collected at all.
	Logs *logcapture.Sink
}

var runnerOutputMu sync.Mutex

// flushRunnerOutput writes one runner report without interleaving it with another report.
func flushRunnerOutput(dst io.Writer, label string, buf *bytes.Buffer) {
	runnerOutputMu.Lock()
	defer runnerOutputMu.Unlock()
	fmt.Fprintf(dst, "\n=== %s ===\n", label)
	_, _ = io.Copy(dst, buf)
}

// Run executes a resolved suite with parallel blocks and sequential scenarios per runner.
func Run(t *testing.T, resolved *topology.Resolved, deps Deps) {
	t.Helper()
	if resolved == nil {
		t.Fatal("runtime: resolved suite is required")
	}

	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	if deps.Steps == nil {
		t.Fatal("runtime: Deps.Steps is required — without step definitions no scenario can run")
	}

	// Count scenarios after parallel block subtests finish.
	var executedTotal atomic.Int64
	t.Cleanup(func() {
		if executedTotal.Load() == 0 {
			t.Errorf("the suite executed NO scenarios — every runner matched nothing; " +
				"a selection that selects nothing is an error, not a pass")
		}
	})

	// Limit the number of live block topologies.
	parallel := resolved.Parallel
	if parallel <= 0 {
		parallel = 1
	}
	slots := make(chan struct{}, parallel)

	for i := range resolved.Blocks {
		block := &resolved.Blocks[i]

		t.Run(block.Name, func(t *testing.T) {
			t.Parallel()

			slots <- struct{}{}
			t.Cleanup(func() { <-slots })

			runBlock(t, block, resolved, deps, log, &executedTotal)
		})
	}
}

// runBlock boots one topology and runs its runners against it.
func runBlock(
	t *testing.T, block *topology.ResolvedBlock, resolved *topology.Resolved, deps Deps,
	log *slog.Logger, executedTotal *atomic.Int64,
) {
	t.Helper()

	bootCtx, cancel := context.WithTimeout(context.Background(), resolved.Timeouts.Boot)
	defer cancel()

	var logWriter *logcapture.Writer
	if deps.Logs != nil {
		path, err := deps.Logs.FileFor(block.Name)
		if err != nil {
			log.Warn("log capture disabled for block", "block", block.Name, "error", err)
		} else if w, err := logcapture.NewWriter(path); err != nil {
			log.Warn("log capture disabled for block", "block", block.Name, "error", err)
		} else {
			logWriter = w
			// Registered before BootBlock so a boot failure still flushes and closes the
			// file, rather than leaking it whenever the block never reaches the main
			// teardown cleanup below.
			t.Cleanup(func() {
				w.Close()
				if dropped := w.Dropped(); dropped > 0 {
					log.Warn("log capture dropped lines", "block", block.Name, "dropped", dropped)
				}
			})
		}
	}

	start := time.Now()
	topo, err := BootBlock(bootCtx, block, deps.RepoRoot, logWriter)
	if err != nil {
		t.Fatalf("block %q failed to boot after %s: %v",
			block.Name, time.Since(start).Round(time.Millisecond), err)
	}
	log.Info("block booted", "block", block.Name,
		"components", topo.Instances.Len(), "elapsed", time.Since(start).Round(time.Millisecond))

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if deps.Coverage != nil {
			if err := topo.CollectCoverage(ctx, deps.Coverage); err != nil {
				log.Warn("coverage collection had errors", "block", block.Name, "error", err)
			}
		}
		if err := topo.Teardown(ctx); err != nil {
			log.Warn("block teardown had errors", "block", block.Name, "error", err)
		}
	})

	runnerSlots := make(chan struct{}, block.EffectiveParallel())

	for j := range block.Runners {
		runner := &block.Runners[j]

		t.Run(runner.Name, func(t *testing.T) {
			t.Parallel()

			runnerSlots <- struct{}{}
			defer func() { <-runnerSlots }()

			runRunner(t, runner, topo, deps, log, executedTotal, resolved.Cleanup.MaxAttempts)
		})
	}
}

// runRunner executes one runner's features against an already-running topology.
func runRunner(
	t *testing.T, runner *topology.Runner, topo *Topology, deps Deps,
	log *slog.Logger, executedTotal *atomic.Int64, cleanupMaxAttempts int,
) {
	t.Helper()

	local := tcontext.NewLocal(runner.Name)

	nameGen, err := unique.NewGenerator()
	if err != nil {
		t.Fatalf("runner %q: %v", runner.Name, err)
	}

	var executed atomic.Int64

	registry := cleanup.NewRegistry(log, cleanupMaxAttempts)
	if deps.CleanupDeleters != nil {
		deps.CleanupDeleters(registry, topo)
	}

	paths := make([]string, 0, len(runner.Features))
	for _, f := range runner.Features {
		paths = append(paths, featurePath(deps.FeatureRoot, f))
	}

	var out bytes.Buffer

	opts := godog.Options{
		Format:      "pretty",
		Output:      &out,
		Paths:       paths,
		Strict:      true,
		Concurrency: 1,
		TestingT:    t,
		Tags:        runner.Tags,
		NoColors:    true,
	}

	suite := godog.TestSuite{
		Name: topo.Block.Name + "/" + runner.Name,
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
				ctx = tcontext.WithShared(ctx, topo.Shared)
				ctx = tcontext.WithLocal(ctx, local)
				if err := unique.Install(ctx, nameGen); err != nil {
					return ctx, err
				}
				if err := cleanup.Install(ctx, registry); err != nil {
					return ctx, err
				}
				return ctx, nil
			})

			sc.After(func(ctx context.Context, sc *godog.Scenario, scenarioErr error) (context.Context, error) {
				executed.Add(1)
				executedTotal.Add(1)
				if err := cleanup.Sweep(ctx); err != nil {
					log.Warn("scenario cleanup had errors",
						"block", topo.Block.Name, "runner", runner.Name,
						"scenario", sc.Name, "error", err)
				}
				return ctx, scenarioErr
			})

			deps.Steps(sc, topo)
		},
		Options: &opts,
	}

	status := suite.Run()

	flushRunnerOutput(os.Stdout, topo.Block.Name+"/"+runner.Name, &out)

	if status != 0 {
		t.Errorf("runner %q finished with status %d", runner.Name, status)
	}

	if executed.Load() == 0 && runner.Tags == "" {
		t.Errorf("runner %q executed NO scenarios — features %v contain none; "+
			"a runner that runs nothing is an error, not a pass",
			runner.Name, runner.Features)
	}

	sweepCtx, cancel := context.WithTimeout(
		tcontext.WithLocal(tcontext.WithShared(context.Background(), topo.Shared), local),
		3*time.Minute,
	)
	defer cancel()
	if err := cleanup.Install(sweepCtx, registry); err != nil {
		log.Warn("runner cleanup setup had errors",
			"block", topo.Block.Name, "runner", runner.Name, "error", err)
	} else if err := cleanup.SweepRunner(sweepCtx); err != nil {
		log.Warn("runner cleanup had errors",
			"block", topo.Block.Name, "runner", runner.Name, "error", err)
	}
}

func featurePath(root, rel string) string {
	if root == "" || len(rel) > 0 && rel[0] == '/' {
		return rel
	}
	return root + "/" + rel
}

// EnsureParallelBudget raises the test parallelism budget for nested block and runner tests.
func EnsureParallelBudget(blocks, maxRunnersPerBlock int) (int, bool) {
	f := flag.Lookup("test.parallel")
	if f == nil {
		return 0, false
	}
	current, err := strconv.Atoi(f.Value.String())
	if err != nil || current <= 0 {
		return 0, false
	}

	// A parent block remains active while its runner subtests execute.
	needed := blocks + blocks*maxRunnersPerBlock + 1
	if current >= needed {
		return current, false
	}
	if err := f.Value.Set(strconv.Itoa(needed)); err != nil {
		return current, false
	}
	return needed, true
}

// ParallelBudgetNotice describes an automatic test parallelism adjustment.
func ParallelBudgetNotice(from, to int) string {
	return fmt.Sprintf(
		"runtime: raised -parallel from %d to %d — nested parallel subtests (blocks containing "+
			"runners) deadlock when the budget is below what they need, and the symptom is a "+
			"hang after boot rather than an error", from, to)
}

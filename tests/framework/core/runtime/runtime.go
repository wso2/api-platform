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
	"github.com/wso2/api-platform/tests/framework/core/topology"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

// StepRegistrar wires a suite's step definitions, given the running topology.
//
// The one thing a suite must supply. Everything else — containers, storage, credentials,
// scoping, concurrency — the engine owns, which is the point: a suite author writes steps
// and a YAML file, never Go testing plumbing.
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
}

// Run executes a resolved suite. Blocks and runners run as parallel subtests; scenarios within
// a runner execute sequentially in feature order.
var runnerOutputMu sync.Mutex

// flushRunnerOutput writes one runner report without interleaving it with another report.
func flushRunnerOutput(dst io.Writer, label string, buf *bytes.Buffer) {
	runnerOutputMu.Lock()
	defer runnerOutputMu.Unlock()
	fmt.Fprintf(dst, "\n=== %s ===\n", label)
	_, _ = io.Copy(dst, buf)
}

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

	// Counts scenarios executed across the WHOLE suite. Checked in a Cleanup because blocks
	// are parallel subtests: Run returns before they finish, so the total is only final here.
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

	start := time.Now()
	topo, err := BootBlock(bootCtx, block, deps.RepoRoot)
	if err != nil {
		// FATAL, never Skip.
		//
		// A skipped block leaves the build GREEN even though its containers never came up —
		// the single most expensive failure mode a suite can have, because it looks like
		// success. Everything about the lifecycle is arranged so this line is reachable.
		t.Fatalf("block %q failed to boot after %s: %v",
			block.Name, time.Since(start).Round(time.Millisecond), err)
	}
	log.Info("block booted", "block", block.Name,
		"components", topo.Instances.Len(), "elapsed", time.Since(start).Round(time.Millisecond))

	// Teardown runs on every path — pass, fail, or panic in a runner — because a leaked
	// topology is attributed to whatever runs next rather than to the block that leaked it.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		// Coverage is harvested BEFORE teardown: counters are written on graceful stop
		// and destroyed with the container. Warn-only, same policy as teardown itself —
		// a failed harvest must not turn a passing block red.
		if deps.Coverage != nil {
			if err := topo.CollectCoverage(ctx, deps.Coverage); err != nil {
				log.Warn("coverage collection had errors", "block", block.Name, "error", err)
			}
		}
		if err := topo.Teardown(ctx); err != nil {
			// A warning, not a failure: teardown trouble must not turn a passing block red,
			// but it is a real leak signal and has to be visible.
			log.Warn("block teardown had errors", "block", block.Name, "error", err)
		}
	})

	// Bounds runners sharing this one topology. A block whose feature restarts a container
	// must set this to 1, or a sibling runner is using that container mid-restart.
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

	// One local scope per runner, shared by all its scenarios — which is what lets a setup
	// feature hand fixtures to the features listed after it.
	local := tcontext.NewLocal(runner.Name)

	nameGen, err := unique.NewGenerator()
	if err != nil {
		t.Fatalf("runner %q: %v", runner.Name, err)
	}

	// Counts scenarios this runner actually EXECUTED. See the check after suite.Run.
	var executed atomic.Int64

	registry := cleanup.NewRegistry(log, cleanupMaxAttempts)
	if deps.CleanupDeleters != nil {
		deps.CleanupDeleters(registry, topo)
	}

	paths := make([]string, 0, len(runner.Features))
	for _, f := range runner.Features {
		paths = append(paths, featurePath(deps.FeatureRoot, f))
	}

	// Buffer each runner's report so concurrent formatters cannot interleave their output.
	var out bytes.Buffer

	opts := godog.Options{
		Format: "pretty",
		Output: &out,
		Paths:  paths,
		// Feature ORDER is the order written in the YAML. godog is given the paths in that
		// order deliberately — a setup feature listed first must run first, and relying on a
		// filename convention to achieve that is a trap.
		Strict: true,
		// Always sequential. Scenarios inside a runner run one after another in the declared
		// order, which is what allows a setup feature to build fixtures for the ones after
		// it. This is a fixed property of a runner, not a tunable.
		Concurrency: 1,
		TestingT:    t,
		Tags:        runner.Tags,
		NoColors:    true,
	}

	suite := godog.TestSuite{
		Name: topo.Block.Name + "/" + runner.Name,
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			// Every scenario gets the block's shared scope and its runner's local scope,
			// plus a name generator and a cleanup registry.
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

			// Per-scenario sweep. In a hook rather than a trailing step, because a step
			// after a failing one never runs — so resources would leak exactly when a test
			// fails, which is when residue is most confusing.
			sc.After(func(ctx context.Context, sc *godog.Scenario, scenarioErr error) (context.Context, error) {
				executed.Add(1)
				executedTotal.Add(1)
				if err := cleanup.Sweep(ctx); err != nil {
					log.Warn("scenario cleanup had errors",
						"block", topo.Block.Name, "runner", runner.Name,
						"scenario", sc.Name, "error", err)
				}
				// The scenario's own error is returned untouched: cleanup trouble must not
				// mask a real failure, nor invent one.
				return ctx, scenarioErr
			})

			deps.Steps(sc, topo)
		},
		Options: &opts,
	}

	status := suite.Run()

	// Flush the runner's output as ONE unit, serialised against every other runner. Taken
	// together with the per-runner buffer above, a runner's report is now contiguous and
	// attributable instead of shredded across its siblings'.
	flushRunnerOutput(os.Stdout, topo.Block.Name+"/"+runner.Name, &out)

	if status != 0 {
		// godog reports failures through TestingT already; this makes the runner's own
		// status unambiguous even if a formatter swallowed something.
		t.Errorf("runner %q finished with status %d", runner.Name, status)
	}

	// A runner that executes no scenarios is a failure unless it was explicitly filtered.
	//
	// godog exits 0 when nothing matches, so a runner bound to a feature file containing no
	// scenarios produces a green result that tested nothing — fast, green and meaningless.
	// This already happened here: `--tags "@metrics"` against untagged features reported ok
	// in 18s, and only an absent line in the output gave it away.
	//
	// The tag check is deliberately conditional. With a filter active, a runner matching
	// nothing is LEGITIMATE — that is what tag-based CI sharding does, and failing every
	// non-matching runner would make selection unusable. The suite-level check in Run covers
	// the case that actually matters: a selection matching nothing ANYWHERE.
	if executed.Load() == 0 && runner.Tags == "" {
		t.Errorf("runner %q executed NO scenarios — features %v contain none; "+
			"a runner that runs nothing is an error, not a pass",
			runner.Name, runner.Features)
	}

	// Per-runner sweep, for fixtures a setup feature created that had to survive across the
	// runner's scenarios. Idempotent, so anything the per-scenario hook already removed is
	// simply absent.
	sweepCtx := tcontext.WithLocal(tcontext.WithShared(context.Background(), topo.Shared), local)
	if err := cleanup.Install(sweepCtx, registry); err == nil {
		if err := cleanup.Sweep(sweepCtx); err != nil {
			log.Warn("runner cleanup had errors",
				"block", topo.Block.Name, "runner", runner.Name, "error", err)
		}
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

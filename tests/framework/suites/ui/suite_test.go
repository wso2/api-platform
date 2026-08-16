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

package ui_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cucumber/godog"

	frameworkbuilder "github.com/wso2/api-platform/tests/framework/core/builder"
	"github.com/wso2/api-platform/tests/framework/core/catalog"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/topology"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps"
)

var selection topology.Selection

func TestMain(m *testing.M) {
	selection.Flags(flag.CommandLine)
	flag.Parse()

	if selection.Coverage {
		if err := os.Setenv(shared.EnvCoverageMode, "true"); err != nil {
			fmt.Fprintln(os.Stderr, "setting coverage mode:", err)
			os.Exit(1)
		}
	}

	// The workspace lists only AI gateways; register this suite's real gateway as one so
	// providers deploy to it.
	if err := os.Setenv(shared.EnvGatewayFunctionalityType, "ai"); err != nil {
		fmt.Fprintln(os.Stderr, "setting gateway functionality type:", err)
		os.Exit(1)
	}

	// See the gateway suite's TestMain: the parallel budget must be settled before m.Run.
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
	registry, err := catalog.Registry()
	if err != nil {
		return 0, 0, false
	}
	resolved, err := topology.LoadFile(filepath.Join(dir, "ui-suite.yaml"), registry)
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

// TestUISuite is the whole UI suite: one call, everything else declared in YAML —
// exactly the gateway suite's shape, with a browser in the topology.
func TestUISuite(t *testing.T) {
	root := repoRoot(t)
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("locating the working directory: %v", err)
	}

	registry, err := catalog.Registry()
	if err != nil {
		t.Fatalf("building the component registry: %v", err)
	}

	resolved, err := topology.LoadFile(filepath.Join(dir, "ui-suite.yaml"), registry)
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
	if err := catalog.BuildSources(context.Background(), narrowed, root, frameworkbuilder.ExecRunner{}); err != nil {
		t.Fatalf("building source images: %v", err)
	}

	frameworkruntime.Run(t, narrowed, frameworkruntime.Deps{
		RepoRoot:    root,
		FeatureRoot: dir,
		Steps: func(sc *godog.ScenarioContext, topo *frameworkruntime.Topology) {
			steps.New(topo).Register(sc)
		},
	})
}

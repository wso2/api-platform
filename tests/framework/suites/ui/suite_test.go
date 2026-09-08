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
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps"
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
	resolved, err := topology.LoadFile(filepath.Join(dir, "ui-suite.yaml"), registry)
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
	if err := catalog.BuildSources(context.Background(), narrowed, root, frameworkbuilder.ExecRunner{}, selection.Coverage); err != nil {
		t.Fatalf("building source images: %v", err)
	}

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
			steps.New(topo, sink).Register(sc)
		},
		CleanupDeleters: func(reg *cleanup.Registry, topo *frameworkruntime.Topology) {
			registerUIDeleters(reg, topo)
		},
	})
}

// TestSuiteShapeReturnsResolvedBlockAndRunnerCounts checks the resolution math itself —
// block/runner counts after selection narrowing — against a small fixture, not the checked
// -in ui-suite.yaml. Asserting against the real suite would break on every routine topology
// edit for reasons unrelated to whether the resolution logic is correct.
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
	if err := os.WriteFile(filepath.Join(dir, "ui-suite.yaml"), []byte(fixture), 0o644); err != nil {
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

// registerUIDeleters configures cleanup handlers for every resource kind a UI scenario can
// create. A deleter runs independently of any browser page — by the time a sweep runs, the
// scenario's own page may already be closed — so every one here uses its own HTTP client
// and its own authentication against platform-api, never the browser's session.
//
// KindLLMProvider, KindLLMProxy, and KindSecret are shared with the IT suite, which targets
// platform-gateway's management API under those same names. Registries are per-suite and
// never collide at runtime, but the two suites mean different underlying resources by them.
func registerUIDeleters(reg *cleanup.Registry, topo *frameworkruntime.Topology) {
	client := httpx.NewClient(httpx.Options{MaxRetries: 1})
	auth := &platformAPIAuth{topo: topo, client: client}

	reg.RegisterDeleter(cleanup.KindLLMProvider, platformAPIDeleter(auth, "/api/v0.9/llm-providers"))
	reg.RegisterDeleter(cleanup.KindLLMProviderTemplate, platformAPIDeleter(auth, "/api/v0.9/llm-provider-templates"))
	reg.RegisterDeleter(cleanup.KindLLMProxy, platformAPIDeleter(auth, "/api/v0.9/llm-proxies"))
	reg.RegisterDeleter(cleanup.KindSecret, platformAPIDeleter(auth, "/api/v0.9/secrets"))
	reg.RegisterDeleter(cleanup.KindProject, platformAPIDeleter(auth, "/api/v0.9/projects"))
	reg.RegisterDeleter(cleanup.KindMCPServer, platformAPIDeleter(auth, "/api/v0.9/mcp-proxies"))
	reg.RegisterDeleter(cleanup.KindGateway, platformAPIDeleter(auth, "/api/v0.9/gateways"))
	reg.RegisterDeleter(cleanup.KindApplication, platformAPIDeleter(auth, "/api/v0.9/applications"))
	reg.RegisterDeleter(cleanup.KindAPIKey, deleteAPIKey(auth))
}

// platformAPIAuth authenticates against platform-api's own login endpoint the first time a
// deleter needs a token during a sweep, and caches it for the rest of that sweep.
type platformAPIAuth struct {
	topo   *frameworkruntime.Topology
	client *httpx.Client
	token  string
}

func (a *platformAPIAuth) baseURL() (string, error) {
	return a.topo.URL("platform-api", "https")
}

func (a *platformAPIAuth) authHeader(ctx context.Context) (string, error) {
	if a.token != "" {
		return "Bearer " + a.token, nil
	}
	base, err := a.baseURL()
	if err != nil {
		return "", err
	}
	form := url.Values{
		"username": {a.topo.Admin.Username},
		"password": {a.topo.Admin.Password},
	}
	resp, err := a.client.Do(ctx, httpx.Request{
		Method:      http.MethodPost,
		URL:         base + "/api/portal/v0.9/auth/login",
		ContentType: "application/x-www-form-urlencoded",
		Body:        []byte(form.Encode()),
	}, 1, 0)
	if err != nil {
		return "", fmt.Errorf("authenticating against platform-api: %w", err)
	}
	if !resp.Succeeded() {
		return "", fmt.Errorf("platform-api login failed: %s", resp.Describe())
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return "", fmt.Errorf("parsing the platform-api login response: %w", err)
	}
	if body.Token == "" {
		return "", fmt.Errorf("platform-api login returned no token")
	}
	a.token = body.Token
	return "Bearer " + a.token, nil
}

// platformAPIDeleter returns a Deleter issuing DELETE {collection}/{id} against
// platform-api. A 404 means the resource is already gone, which is success for a sweep.
func platformAPIDeleter(auth *platformAPIAuth, collection string) cleanup.Deleter {
	return func(ctx context.Context, res cleanup.Resource) error {
		base, err := auth.baseURL()
		if err != nil {
			return err
		}
		authHeader, err := auth.authHeader(ctx)
		if err != nil {
			return err
		}
		resp, err := auth.client.Do(ctx, httpx.Request{
			Method:  http.MethodDelete,
			URL:     base + collection + "/" + res.ID,
			Headers: map[string]string{"Authorization": authHeader},
		}, 1, 0)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusNotFound || resp.Succeeded() {
			return nil
		}
		return fmt.Errorf("cleanup: %s", resp.Describe())
	}
}

// deleteAPIKey removes an API key nested under either an LLM provider or an app LLM proxy
// — the two owners have separate, otherwise identically-shaped endpoints. The resource id
// is registered as "<collection>/<ownerID>/<keyName>" (collection is "llm-providers" or
// "llm-proxies"), since the endpoint is nested under the owner rather than a flat
// collection.
func deleteAPIKey(auth *platformAPIAuth) cleanup.Deleter {
	return func(ctx context.Context, res cleanup.Resource) error {
		parts := strings.SplitN(res.ID, "/", 3)
		if len(parts) != 3 {
			return fmt.Errorf("cleanup: api-key resource id %q is not \"collection/ownerID/keyName\"", res.ID)
		}
		collection, ownerID, keyName := parts[0], parts[1], parts[2]
		base, err := auth.baseURL()
		if err != nil {
			return err
		}
		authHeader, err := auth.authHeader(ctx)
		if err != nil {
			return err
		}
		resp, err := auth.client.Do(ctx, httpx.Request{
			Method:  http.MethodDelete,
			URL:     base + "/api/v0.9/" + collection + "/" + ownerID + "/api-keys/" + keyName,
			Headers: map[string]string{"Authorization": authHeader},
		}, 1, 0)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusNotFound || resp.Succeeded() {
			return nil
		}
		return fmt.Errorf("cleanup: %s", resp.Describe())
	}
}

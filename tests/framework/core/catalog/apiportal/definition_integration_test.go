//go:build integration

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

package apiportal

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	playwright "github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/builder"
	catalogbrowser "github.com/wso2/api-platform/tests/framework/core/catalog/browser"
	"github.com/wso2/api-platform/tests/framework/core/catalog/platformapi"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/coverage"
	"github.com/wso2/api-platform/tests/framework/core/runtime"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, ok := shared.RepoRootFromCallerFile()
	require.True(t, ok, "repository root not found")
	return root
}

// TestAPIPortalBoots answers one question before any subscription feature is written: does the
// developer portal come up under this framework, against the real control plane, sharing the
// control plane's key material?
//
// It boots BOTH products because the portal verifies tokens platform-api signs and calls it at
// startup — testing the portal alone would prove only that a web server starts.
func TestAPIPortalBoots(t *testing.T) {
	root := repoRoot(t)
	t.Setenv(shared.EnvCoverageMode, "true")
	for _, name := range []string{"platform-api", "api-portal"} {
		version, ok := shared.SourceVersion(name)
		require.True(t, ok)
		var spec builder.Spec
		var buildErr error
		if name == "platform-api" {
			spec, buildErr = platformapi.BuildSpec(version)
		} else {
			spec, buildErr = BuildSpec(version)
		}
		require.NoError(t, buildErr)
		require.NoError(t, builder.Build(context.Background(), spec, builder.Request{
			RepoRoot: root, Version: version, Coverage: true, Runner: builder.ExecRunner{},
		}), "building the instrumented %s image", name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	nw, err := runtime.NewNetwork(ctx, "ap-probe")
	require.NoError(t, err)
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	cp := platformapi.PlatformAPI()
	portal := APIPortal()

	prov, err := runtime.Provision(ctx, runtime.DatabaseOptions{
		Network: nw, RepoRoot: root,
		Requests: []runtime.Request{
			{Def: cp, Type: components.Postgres, Replicas: 1},
			{Def: portal, Type: components.Postgres, Replicas: 1},
		},
	})
	require.NoError(t, err, "provisioning storage")
	t.Cleanup(func() { _ = prov.Stop(context.Background()) })

	start := func(def *components.Definition) *runtime.ComposeStack {
		t.Helper()
		env := map[string]string{}
		for k, v := range prov.Env[runtime.KeyFor(def.Name, 0)] {
			env[k] = v
		}
		content, cerr := components.Assemble(def.Config, root, "", nil)
		require.NoError(t, cerr, "assembling config for %s", def.Name)

		spec := def.Compose.WithGenerated(map[string][]byte{
			"api-platform.env": components.EnvFileContent(env),
			"config.toml":      content,
		})
		stack, serr := runtime.LaunchCompose(ctx, def, spec,
			runtime.Options{Network: nw, RepoRoot: root, Env: env})
		if stack != nil {
			t.Cleanup(func() { _ = stack.Stop(context.Background()) })
		}
		if serr != nil {
			logs := ""
			if stack != nil {
				logs = stack.Logs(ctx)
			}
			t.Fatalf("%s did not start: %v\nlogs:\n%s", def.Name, serr, logs)
		}
		if herr := runtime.AwaitHealthy(ctx, stack.Instance, nil); herr != nil {
			t.Fatalf("%s never became healthy: %v\nlogs:\n%s", def.Name, herr, stack.Logs(ctx))
		}
		t.Logf("PROBE %s healthy", def.Name)
		return stack
	}

	start(cp)
	portalStack := start(portal)
	output := t.TempDir()
	if configured := os.Getenv(coverage.EnvOut); configured != "" {
		output = configured
	}
	sink, sinkErr := coverage.NewSink(output)
	require.NoError(t, sinkErr)

	browserStack, launchErr := runtime.Launch(ctx, catalogbrowser.Browser(), runtime.Options{Network: nw, Replicas: 1})
	require.NoError(t, launchErr, "launching the browser")
	t.Cleanup(func() { _ = browserStack.Stop(context.Background()) })
	port, portErr := browserStack.Instance.MappedPort("ws")
	require.NoError(t, portErr)
	ws := "ws://" + browserStack.Instance.Host() + ":" + strconv.Itoa(port) + catalogbrowser.PlaywrightWSPath
	// The browser container shares the portal network, so it resolves the service alias.
	pw, pwErr := playwright.Run()
	require.NoError(t, pwErr)
	t.Cleanup(func() { _ = pw.Stop() })
	remote, connectErr := pw.Chromium.Connect(ws)
	require.NoError(t, connectErr)
	t.Cleanup(func() { _ = remote.Close() })
	bctx, contextErr := remote.NewContext()
	require.NoError(t, contextErr)
	t.Cleanup(func() { _ = bctx.Close() })
	page, pageErr := bctx.NewPage()
	require.NoError(t, pageErr)
	_, gotoErr := page.Goto("http://api-portal:9543/")
	require.NoError(t, gotoErr)
	require.Eventually(t, func() bool {
		title, titleErr := page.Title()
		return titleErr == nil && title != ""
	}, 30*time.Second, time.Second)
	istanbulReport, reportErr := page.Evaluate("globalThis.__coverage__ || null")
	require.NoError(t, reportErr)
	require.NotNil(t, istanbulReport, "instrumented API Portal scripts must expose Istanbul coverage")
	dir, dirErr := sink.BrowserDir("api-portal", "portal-smoke")
	require.NoError(t, dirErr)
	require.NoError(t, coverage.WriteIstanbulReport(filepath.Join(dir, "raw-istanbul.json"), istanbulReport))

	// The portal must have been handed the control plane's OWN public key, not a second draw.
	require.Equal(t,
		string(shared.ControlPlaneCrypto()["keys/jwt_public.pem"]),
		string(portalCryptoFiles()["keys/jwt_public.pem"]),
		"the portal must verify tokens with the key platform-api signs them with")

	id, err := portalStack.ServiceContainerID(ctx, svcAPIPortal)
	require.NoError(t, err)
	require.NoError(t, portalStack.StopService(ctx, svcAPIPortal))
	dst, err := sink.Dir("api-portal", svcAPIPortal)
	require.NoError(t, err)
	require.NoError(t, coverage.CopyDir(ctx, id, coverage.GuestDir, dst))
	entries, err := os.ReadDir(filepath.Clean(dst))
	require.NoError(t, err)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "coverage-") && strings.HasSuffix(entry.Name(), ".json") {
			return
		}
	}
	require.Fail(t, "API Portal must flush Node/V8 coverage artifacts on graceful stop",
		"coverage JSON not found in %v", entries)
}

func TestAPIPortalCoverageCapability(t *testing.T) {
	t.Setenv(shared.EnvCoverageMode, "true")
	definition := APIPortal()
	require.Equal(t, "/coverage", definition.Compose.Env["NODE_V8_COVERAGE"])
	require.Equal(t, []string{"node-v8"}, definition.Compose.CoverageServices[0].Types)
}

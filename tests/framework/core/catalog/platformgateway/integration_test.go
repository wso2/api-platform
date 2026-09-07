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

package platformgateway

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/runtime"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, ok := shared.RepoRootFromCallerFile()
	require.True(t, ok, "repository root not found")
	return root
}

// bootPlatformGateway brings the gateway up as ONE component on the given engine.
func bootPlatformGateway(
	t *testing.T, ctx context.Context, block string, engine components.DBType, repoRoot string,
) (*runtime.ComposeStack, actor.Credentials, func()) {
	return bootPlatformGatewayWithOverlay(t, ctx, block, engine, repoRoot, "")
}

// bootPlatformGatewayWithOverlay is bootPlatformGateway with a block-scoped config overlay,
// so a probe can reproduce exactly what a block with that overlay runs.
func bootPlatformGatewayWithOverlay(
	t *testing.T, ctx context.Context, block string, engine components.DBType, repoRoot, overlay string,
) (*runtime.ComposeStack, actor.Credentials, func()) {
	t.Helper()

	gw := PlatformGateway()

	nw, err := runtime.NewNetwork(ctx, block)
	require.NoError(t, err)

	var stack *runtime.ComposeStack
	teardown := func() {
		if stack != nil {
			_ = stack.Stop(context.Background())
		}
		_ = nw.Remove(context.Background())
	}

	// Storage and configuration are provisioned by the framework; credentials come from the
	// test-only overlay.
	provisioned, err := runtime.Provision(ctx, runtime.DatabaseOptions{
		Network:  nw,
		RepoRoot: repoRoot,
		Requests: []runtime.Request{{Def: gw, Type: engine, Replicas: 1}},
	})
	if err != nil {
		teardown()
		require.NoError(t, err, "provisioning gateway storage on %s", engine)
	}
	origTeardown := teardown
	teardown = func() {
		origTeardown()
		_ = provisioned.Stop(context.Background())
	}

	adminCreds := actor.Administrator()

	// The env file carries the DSN in the controller's vocabulary.
	env := map[string]string{}
	for k, v := range provisioned.Env[runtime.KeyFor(gw.Name, 0)] {
		env[k] = v
	}

	// The probe stands in for a block, so it supplies the same ${BLOCK} the engine would —
	// an overlay that addresses a partitioned testbench service needs it to resolve.
	vars := components.Vars{components.VarBlock: block}
	configContent, err := components.Assemble(gw.Config, repoRoot, overlay, vars)
	if err != nil {
		teardown()
		require.NoError(t, err, "assembling gateway config")
	}

	spec := gw.Compose.WithGenerated(map[string][]byte{
		"config.toml":      configContent,
		"api-platform.env": components.EnvFileContent(env),
	})

	stack, err = runtime.LaunchCompose(ctx, gw, spec, runtime.Options{
		Network:  nw,
		RepoRoot: repoRoot,
	})
	if err != nil {
		// Without the service logs a compose readiness failure says only "context
		// deadline exceeded", which names neither the service nor the reason.
		t.Logf("staged compose files at %s", stack.StageDir())
		t.Logf("gateway service logs:\n%s", stack.Logs(context.Background()))
		teardown()
		require.NoError(t, err, "bringing up the platform gateway on %s", engine)
	}

	return stack, adminCreds, teardown
}

func TestPlatformGatewayBootsAsOneComponent(t *testing.T) {
	root := repoRoot(t)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	stack, _, teardown := bootPlatformGateway(t, ctx, "pg-single", components.SQLite, root)
	defer teardown()

	inst := stack.Instance
	require.NotNil(t, inst)

	t.Run("the data plane is reachable on an ephemeral port", func(t *testing.T) {
		// Two containers underneath, one addressable component on top.
		port, err := inst.MappedPort("http")
		require.NoError(t, err)
		require.NotEqual(t, 8080, port, "host port must not be the canonical container port")

		url, err := inst.URL("http")
		require.NoError(t, err)
		t.Logf("data plane at %s (container port 8080)", url)
	})

	t.Run("the controller's management API is addressable through the same component", func(t *testing.T) {
		// Consolidation must not hide the management plane: it is the same logical
		// component, so a test reaches it without knowing there is a second container.
		admin, err := inst.URL("admin")
		require.NoError(t, err)

		client := &http.Client{
			Timeout:   10 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec
		}
		resp, err := client.Get(admin + "/api/admin/v1/health")
		require.NoError(t, err, "the controller admin API should answer through the gateway component")
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("envoy is up but NOT ready before any API is deployed", func(t *testing.T) {
		// Pinning the product behaviour that makes /ready unusable as a boot gate: with no
		// listener config Envoy stays in PRE_INITIALIZING and answers 503. Asserting 200
		// here would be asserting something false, and gating a block on it would hang
		// every boot until the timeout.
		//
		// It becomes ready once an API is deployed — which is a post-deployment assertion,
		// and belongs in the health feature rather than here.
		admin, err := inst.URL("envoy-admin")
		require.NoError(t, err)

		resp, err := http.Get(admin + "/ready") //nolint:gosec,noctx
		require.NoError(t, err, "the admin interface itself must be reachable")
		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode,
			"envoy has no listeners until an API is deployed")
		require.Contains(t, string(body), "PRE_INITIALIZING")
	})
}

func TestTwoPlatformGatewaysRunConcurrently(t *testing.T) {
	root := repoRoot(t)

	// Each compose stack gets a unique identifier, without which compose would treat two
	// blocks' stacks as the SAME stack and the second would adopt the first's containers
	// instead of creating its own.
	type result struct {
		block string
		port  int
	}

	done := make(chan result, 2)
	errs := make(chan error, 2)

	for _, block := range []string{"pg-concurrent-a", "pg-concurrent-b"} {
		go func(block string) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
			defer cancel()

			stack, _, teardown := bootPlatformGateway(t, ctx, block, components.SQLite, root)
			defer teardown()

			port, err := stack.Instance.MappedPort("http")
			if err != nil {
				errs <- fmt.Errorf("%s: %w", block, err)
				return
			}
			done <- result{block: block, port: port}

			// Hold both up together so the assertion below is made while they coexist.
			time.Sleep(4 * time.Second)
		}(block)
	}

	var results []result
	for range 2 {
		select {
		case r := <-done:
			results = append(results, r)
		case err := <-errs:
			t.Fatalf("concurrent gateway boot failed: %v", err)
		case <-time.After(10 * time.Minute):
			t.Fatal("timed out waiting for concurrent gateways")
		}
	}

	require.Len(t, results, 2)
	require.NotEqual(t, results[0].port, results[1].port,
		"two concurrent gateways must not share a host port")
	t.Logf("concurrent data planes on ports %d and %d", results[0].port, results[1].port)
}

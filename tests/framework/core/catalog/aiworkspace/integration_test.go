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

package aiworkspace

import (
	"context"
	"crypto/tls"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	"github.com/wso2/api-platform/tests/framework/core/catalog/platformapi"
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

// TestAIWorkspaceBoots answers one question before any UI feature is written: does the AI
// portal come up under this framework against the real control plane, with its own TLS pair
// and the control plane's public certificate as its only upstream trust?
//
// It boots BOTH products because the BFF authenticates against platform-api and proxies to
// it — testing the workspace alone would prove only that a web server starts. The final
// check drives the BFF's own login with the block's admin credentials, which exercises the
// whole chain: config templating, ca_file trust, and platform-api's local login.
func TestAIWorkspaceBoots(t *testing.T) {
	root := repoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	nw, err := runtime.NewNetwork(ctx, "aiw-probe")
	require.NoError(t, err)
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	cp := platformapi.PlatformAPI()
	workspace := AIWorkspace()

	prov, err := runtime.Provision(ctx, runtime.DatabaseOptions{
		Network: nw, RepoRoot: root,
		Requests: []runtime.Request{
			{Def: cp, Type: components.SQLite, Replicas: 1},
		},
	})
	require.NoError(t, err, "provisioning storage")
	t.Cleanup(func() { _ = prov.Stop(context.Background()) })

	admin := actor.Administrator()

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
	aiw := start(workspace)

	// The whole point of the probe: a session login through the BFF's own door, with the
	// block's test credentials, over its self-signed TLS. This proves the BFF verified
	// platform-api through cp-cert.pem alone.
	base, err := aiw.Instance.URL("https")
	require.NoError(t, err)

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/ai-workspace/api/login",
		strings.NewReader(`{"username":"`+admin.Username+`","password":"`+admin.Password+`"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	// The BFF rejects state-changing calls without this CSRF header.
	req.Header.Set("X-Requested-By", "ai-workspace")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"BFF login with the block's admin credentials must succeed; logs:\n%s", aiw.Logs(ctx))
}

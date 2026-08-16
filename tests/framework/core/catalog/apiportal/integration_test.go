//go:build integration

package apiportal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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

// TestAPIPortalBoots answers one question before any subscription feature is written: does the
// developer portal come up under this framework, against the real control plane, sharing the
// control plane's key material?
//
// It boots BOTH products because the portal verifies tokens platform-api signs and calls it at
// startup — testing the portal alone would prove only that a web server starts.
func TestAPIPortalBoots(t *testing.T) {
	root := repoRoot(t)
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
	start(portal)

	// The portal must have been handed the control plane's OWN public key, not a second draw.
	require.Equal(t,
		string(shared.ControlPlaneCrypto()["keys/jwt_public.pem"]),
		string(portalCryptoFiles()["keys/jwt_public.pem"]),
		"the portal must verify tokens with the key platform-api signs them with")
}

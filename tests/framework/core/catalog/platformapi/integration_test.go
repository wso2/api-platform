//go:build integration

package platformapi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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

// TestPlatformAPIBoots answers the only question that matters before wiring the cross-plane
// block: does the REAL control plane come up under this framework, and can the provisioner
// mint a gateway registration token against it?
func TestPlatformAPIBoots(t *testing.T) {
	root := repoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	nw, err := runtime.NewNetwork(ctx, "pa-probe")
	require.NoError(t, err)
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	def := PlatformAPI()

	prov, err := runtime.Provision(ctx, runtime.DatabaseOptions{
		Network: nw, RepoRoot: root,
		Requests: []runtime.Request{{Def: def, Type: components.Postgres, Replicas: 1}},
	})
	require.NoError(t, err, "provisioning storage")
	t.Cleanup(func() { _ = prov.Stop(context.Background()) })

	env := map[string]string{}
	for k, v := range prov.Env[runtime.KeyFor(def.Name, 0)] {
		env[k] = v
	}
	t.Logf("PROBE db env: %v", env)

	content, err := components.Assemble(def.Config, root, "", nil)
	require.NoError(t, err, "assembling config")

	spec := def.Compose.WithGenerated(map[string][]byte{
		"api-platform.env": components.EnvFileContent(env),
		"config.toml":      content,
	})
	require.Contains(t, spec.GeneratedFiles, "certs/cert.pem", "generated crypto must survive WithGenerated")

	stack, err := runtime.LaunchCompose(ctx, def, spec,
		runtime.Options{Network: nw, RepoRoot: root, Env: env})
	if stack != nil {
		t.Cleanup(func() { _ = stack.Stop(context.Background()) })
	}
	if err != nil {
		t.Fatalf("platform-api did not start: %v\nlogs:\n%s", err, stack.Logs(ctx))
	}

	require.NoError(t, runtime.AwaitHealthy(ctx, stack.Instance, nil), "health gate")
	t.Log("PROBE platform-api is healthy")

	values, err := def.Provisions(ctx, stack.Instance)
	if err != nil {
		t.Fatalf("provisioner failed: %v\nlogs:\n%s", err, stack.Logs(ctx))
	}
	t.Logf("PROBE provisioned keys: %v", keysOf(values))
	require.NotEmpty(t, values["APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN"], "a gateway token must be minted")
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

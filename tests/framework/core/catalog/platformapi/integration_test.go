//go:build integration

package platformapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/builder"
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

// TestPlatformAPIBoots answers the only question that matters before wiring the cross-plane
// block: does the REAL control plane come up under this framework, and can the provisioner
// mint a gateway registration token against it?
func TestPlatformAPIBoots(t *testing.T) {
	root := repoRoot(t)
	t.Setenv(shared.EnvCoverageMode, "true")
	version, ok := shared.SourceVersion("platform-api")
	require.True(t, ok)
	buildSpec, err := BuildSpec(version)
	require.NoError(t, err)
	require.NoError(t, builder.Build(context.Background(), buildSpec, builder.Request{
		RepoRoot: root, Version: version, Coverage: true, Runner: builder.ExecRunner{},
	}), "building the instrumented Platform API image")
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
	coverageEnv, err := stack.Exec(ctx, "platform-api", []string{"sh", "-c", "printf '%s' \"$GOCOVERDIR\""})
	require.NoError(t, err)
	require.Equal(t, "/coverage", coverageEnv, "the instrumented service must receive GOCOVERDIR")

	values, err := def.Provisions(ctx, stack.Instance)
	if err != nil {
		t.Fatalf("provisioner failed: %v\nlogs:\n%s", err, stack.Logs(ctx))
	}
	t.Logf("PROBE provisioned keys: %v", keysOf(values))
	require.NotEmpty(t, values["APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN"], "a gateway token must be minted")

	sink, err := coverage.NewSink(t.TempDir())
	require.NoError(t, err)
	id, err := stack.ServiceContainerID(ctx, "platform-api")
	require.NoError(t, err)
	require.NoError(t, stack.StopService(ctx, "platform-api"))
	dst, err := sink.Dir("platform-api", "platform-api")
	require.NoError(t, err)
	require.NoError(t, coverage.CopyDir(ctx, id, coverage.GuestDir, dst))
	entries, err := os.ReadDir(filepath.Clean(dst))
	require.NoError(t, err)
	requireCoverageArtifact(t, entries, "covmeta.", "Platform API must flush Go metadata on graceful stop")
	requireCoverageArtifact(t, entries, "covcounters.", "Platform API must flush Go counters on graceful stop")
}

func requireCoverageArtifact(t *testing.T, entries []os.DirEntry, prefix, message string) {
	t.Helper()
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			return
		}
	}
	require.Fail(t, message, "artifact prefix %q not found in %v", prefix, entries)
}

func TestPlatformAPICoverageCapability(t *testing.T) {
	t.Setenv(shared.EnvCoverageMode, "true")
	definition := PlatformAPI()
	require.Equal(t, "/coverage", definition.Compose.Env["GOCOVERDIR"])
	require.Equal(t, []string{"go"}, definition.Compose.CoverageServices[0].Types)
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

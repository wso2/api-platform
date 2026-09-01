//go:build integration

package testbench

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/runtime"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, ok := shared.RepoRootFromCallerFile()
	require.True(t, ok, "repository root not found")
	return root
}

// TestTestbenchPortResolution checks the one thing that failed: does the Instance report the
// port docker actually mapped? A mismatch is invisible until a host-side call is refused.
func TestTestbenchPortResolution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	def := Testbench()
	nw, err := runtime.NewNetwork(ctx, "tb-probe")
	require.NoError(t, err)
	defer func() { _ = nw.Remove(context.Background()) }()

	c, err := runtime.LaunchShared(ctx, def, runtime.Options{Network: nw, RepoRoot: repoRoot(t)})
	require.NoError(t, err)

	for _, ep := range []string{"jwks", "echo"} {
		url, err := c.Instance.URL(ep)
		require.NoError(t, err)
		port, err := c.Instance.MappedPort(ep)
		require.NoError(t, err)
		t.Logf("INSTANCE %-5s -> %s (port %d)", ep, url, port)
	}

	out, err := exec.CommandContext(ctx, "docker", "ps", "--filter",
		"ancestor="+def.Image.Resolve("arm64"), "--format", "{{.ID}} {{.Ports}}").Output()
	require.NoError(t, err)
	t.Logf("DOCKER   %s", strings.TrimSpace(string(out)))
}

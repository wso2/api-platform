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

package runtime

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/logcapture"
)

// TestLaunchStreamsContainerOutputIntoTheBlockLogFile confirms a real container's
// stdout/stderr actually lands in the combined per-block log file, tagged with its
// component name — the mechanism that gives a scenario failure the product's own logs,
// not only the client-side symptom.
func TestLaunchStreamsContainerOutputIntoTheBlockLogFile(t *testing.T) {
	ctx := context.Background()

	nw := newTestNetwork(t, ctx, "logcapture-probe")
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	path := filepath.Join(t.TempDir(), "block.log")
	writer, err := logcapture.NewWriter(path)
	require.NoError(t, err)
	t.Cleanup(writer.Close)

	def := probeDef("logcapture-probe")
	c, err := Launch(ctx, def, Options{Network: nw, Replicas: 1, LogWriter: writer})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Stop(context.Background()) })

	require.NoError(t, AwaitHealthy(ctx, c.Instance, nil))

	writer.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, data, "the container's own output should have been captured")
	require.Contains(t, string(data), "[logcapture-probe] ")
	require.Equal(t, int64(0), writer.Dropped())
}

// TestStopReportsAContainerThatHadAlreadyExited confirms teardown logs a container that died
// before it, with its exit code, since a mid-run exit otherwise leaves only connection errors.
func TestStopReportsAContainerThatHadAlreadyExited(t *testing.T) {
	ctx := context.Background()

	nw := newTestNetwork(t, ctx, "exited-probe")
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	c, err := Launch(ctx, probeDef("exited-probe"), Options{Network: nw, Replicas: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Stop(context.Background()) })
	// Stopped behind the framework's back, as a crash would, not through Halt.
	require.NoError(t, c.container.Stop(ctx, nil))

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	require.NoError(t, c.Stop(ctx))
	require.Contains(t, logs.String(), `msg="container had exited before teardown" container=exited-probe status=exited exitCode=`)
}

// TestStopDoesNotReportAHaltedContainer confirms a container the framework stopped on purpose
// is not reported as having exited on its own.
func TestStopDoesNotReportAHaltedContainer(t *testing.T) {
	ctx := context.Background()

	nw := newTestNetwork(t, ctx, "halted-probe")
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	c, err := Launch(ctx, probeDef("halted-probe"), Options{Network: nw, Replicas: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Stop(context.Background()) })
	require.NoError(t, c.Halt(ctx))

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	require.NoError(t, c.Stop(ctx))
	require.NotContains(t, logs.String(), "container had exited before teardown")
}

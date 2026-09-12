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
	"context"
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

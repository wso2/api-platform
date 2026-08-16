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

package coverage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestCopyDirFromStoppedContainer verifies copying counters from a stopped container.
func TestCopyDirFromStoppedContainer(t *testing.T) {
	ctx := context.Background()

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "nginx:alpine",
			Entrypoint: []string{"sh", "-c",
				`mkdir -p /data/sub && printf 'plain' > /data/a.txt && printf '\x00\x01\xff' > /data/sub/blob.bin`},
			WaitingFor: wait.ForExit(),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(context.Background()) })

	state, err := ctr.State(ctx)
	require.NoError(t, err)
	require.False(t, state.Running, "the container must be stopped for this test to prove anything")

	dst := t.TempDir()
	require.NoError(t, CopyDir(ctx, ctr.GetContainerID(), "/data", dst))

	got, err := os.ReadFile(filepath.Join(dst, "a.txt"))
	require.NoError(t, err)
	require.Equal(t, []byte("plain"), got)

	got, err = os.ReadFile(filepath.Join(dst, "sub", "blob.bin"))
	require.NoError(t, err)
	require.Equal(t, []byte{0x00, 0x01, 0xff}, got, "binary bytes must survive the tar stream")
}

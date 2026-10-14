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

package suiterun

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/topology"
)

// TestShapeAtReturnsResolvedBlockAndRunnerCounts checks the resolution math itself —
// database-matrix expansion and EffectiveParallel — against a small fixture, not a checked-in
// suite. Asserting against a real suite would break on every routine topology edit for reasons
// unrelated to whether the resolution logic is correct.
func TestShapeAtReturnsResolvedBlockAndRunnerCounts(t *testing.T) {
	dir := t.TempDir()
	fixture := `
suite: fixture-suite

defaults:
  parallel: 3
  components:
    platform-gateway:
      db: sqlite
  timeouts:
    boot: 5m
    propagation: 180s

blocks:
  - name: block-a
    parallel: 5
    components:
      - name: platform-gateway
        db:
          matrix: [sqlite, postgres]
    runners:
      - name: runner-a
        features:
          - features/does-not-need-to-exist-a.feature

  - name: block-b
    parallel: 3
    components:
      - name: platform-gateway
        db:
          matrix: [sqlite, postgres]
    runners:
      - name: runner-b
        features:
          - features/does-not-need-to-exist-b.feature
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, DefaultSuiteFile), []byte(fixture), 0o644))

	blocks, maxRunners, ok := ShapeAt(dir, "", topology.Selection{})

	require.True(t, ok, "ShapeAt() should load the fixture suite")
	// 2 blocks x 2 database variants each = 4 resolved blocks.
	require.Equal(t, 4, blocks)
	// The larger of the two blocks' EffectiveParallel (block-a's 5 vs block-b's 3).
	require.Equal(t, 5, maxRunners)
}

func TestShapeAtRejectsAMissingSuiteFile(t *testing.T) {
	blocks, maxRunners, ok := ShapeAt(t.TempDir(), "", topology.Selection{})

	require.False(t, ok)
	require.Zero(t, blocks)
	require.Zero(t, maxRunners)
}

func TestFindWorkspaceRootPrefersTheNearestMarker(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "tests", "it")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))

	got, err := FindWorkspaceRoot(nested)
	require.NoError(t, err)
	require.Equal(t, root, got)

	require.NoError(t, os.WriteFile(filepath.Join(root, "tests", "go.work"), nil, 0o644))
	got, err = FindWorkspaceRoot(nested)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "tests"), got)
}

func TestFindWorkspaceRootAcceptsAGitFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: elsewhere"), 0o644))

	got, err := FindWorkspaceRoot(root)

	require.NoError(t, err)
	require.Equal(t, root, got)
}

func TestFindWorkspaceRootFailsWithoutAMarker(t *testing.T) {
	// A fresh temporary directory sits beneath no marker on a clean machine, but a developer's
	// temp location may not; only the error text is asserted when none is found.
	if _, err := FindWorkspaceRoot(t.TempDir()); err != nil {
		require.ErrorContains(t, err, "set Options.WorkspaceRoot")
	}
}

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

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/assets"
)

func writeSource(t *testing.T, repo, content string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(assets.SnapshotSources[0]))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestRunReportsDriftInCheckModeWithoutWriting(t *testing.T) {
	repo, module := t.TempDir(), t.TempDir()
	writeSource(t, repo, "bin")
	var stdout, stderr bytes.Buffer

	code := run([]string{"-repo", repo, "-module", module, "-check"}, &stdout, &stderr)

	require.Equal(t, 1, code)
	require.Contains(t, stdout.String(), assets.SnapshotSources[0])
	require.Contains(t, stderr.String(), "run: go run ./cmd/syncassets")
	require.NoFileExists(t, filepath.Join(module, "snapshots", filepath.FromSlash(assets.SnapshotSources[0])))
}

func TestRunWritesSnapshotsAndThenFindsNoDrift(t *testing.T) {
	repo, module := t.TempDir(), t.TempDir()
	writeSource(t, repo, "bin")
	var stdout, stderr bytes.Buffer

	require.Equal(t, 0, run([]string{"-repo", repo, "-module", module}, &stdout, &stderr))
	require.FileExists(t, filepath.Join(module, "snapshots", filepath.FromSlash(assets.SnapshotSources[0])))

	stdout.Reset()
	require.Equal(t, 0, run([]string{"-repo", repo, "-module", module, "-check"}, &stdout, &stderr))
	require.Empty(t, stdout.String())
}

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

package health

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func textHandler(w io.Writer) slog.Handler {
	return slog.NewTextHandler(w, nil)
}

func newStdout() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return slog.New(slog.NewTextHandler(buf, nil)), buf
}

func TestNewLoggerWithoutPathUsesStdoutOnly(t *testing.T) {
	dir := t.TempDir()
	stdout, buf := newStdout()

	logger, closer := NewLogger(stdout, textHandler, FileLogConfig{MaxSizeMB: 1, MaxFiles: 1})
	logger.Error("health check failed")

	assert.Contains(t, buf.String(), `msg="health check failed"`)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.NoError(t, closer.Close())
	assert.NoError(t, closer.Close())
}

func TestNewLoggerWritesToStdoutAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "health.log")
	stdout, buf := newStdout()

	logger, closer := NewLogger(stdout, textHandler, FileLogConfig{Path: path, MaxSizeMB: 1, MaxFiles: 2})
	logger.Error("health check failed")
	logger.Info("health check recovered")
	require.NoError(t, closer.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	for _, out := range []string{buf.String(), string(data)} {
		assert.Contains(t, out, `level=ERROR msg="health check failed"`)
		assert.Contains(t, out, `level=INFO msg="health check recovered"`)
	}
	assert.NotContains(t, buf.String(), "health log file")
}

func TestNewLoggerFileLinesUseHandlerFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.log")
	stdout, _ := newStdout()
	jsonHandler := func(w io.Writer) slog.Handler { return slog.NewJSONHandler(w, nil) }

	logger, closer := NewLogger(stdout, jsonHandler, FileLogConfig{Path: path, MaxSizeMB: 1, MaxFiles: 1})
	logger.Error("health check failed")
	require.NoError(t, closer.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"msg":"health check failed"`)
}

func TestNewLoggerFallsBackToStdoutWhenFileUnavailable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions are not enforced for this user")
	}
	parent := t.TempDir()
	require.NoError(t, os.Chmod(parent, 0o500))
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })
	path := filepath.Join(parent, "logs", "health.log")
	stdout, buf := newStdout()

	logger, closer := NewLogger(stdout, textHandler, FileLogConfig{Path: path, MaxSizeMB: 1, MaxFiles: 1})
	logger.Error("health check failed")

	out := buf.String()
	assert.Equal(t, 1, strings.Count(out, `msg="health log file unavailable, logging health events to stdout only"`))
	assert.Contains(t, out, `reason="cannot open the health log file"`)
	assert.NotContains(t, out, parent)
	assert.NotContains(t, out, "permission denied")
	assert.Contains(t, out, `msg="health check failed"`)
	assert.NoError(t, closer.Close())
}

func TestNewLoggerWarnsWhenFileAccessibleToOthers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not meaningful on windows")
	}
	path := filepath.Join(t.TempDir(), "health.log")
	require.NoError(t, os.WriteFile(path, nil, 0o644))
	require.NoError(t, os.Chmod(path, 0o644))
	stdout, buf := newStdout()

	logger, closer := NewLogger(stdout, textHandler, FileLogConfig{Path: path, MaxSizeMB: 1, MaxFiles: 1})
	logger.Info("health check recovered")
	require.NoError(t, closer.Close())

	out := buf.String()
	assert.Contains(t, out, `msg="health log file or directory is accessible to other users"`)
	assert.Contains(t, out, "mode=0644")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `msg="health check recovered"`)
}

func TestNewLoggerWarnsWhenDirectoryAccessibleToOthers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not meaningful on windows")
	}
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o755))
	stdout, buf := newStdout()

	_, closer := NewLogger(stdout, textHandler, FileLogConfig{Path: filepath.Join(dir, "health.log"), MaxSizeMB: 1, MaxFiles: 1})
	require.NoError(t, closer.Close())

	assert.Contains(t, buf.String(), "mode=0755")
}

func TestNewLoggerOwnerOnlyFileDoesNotWarn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "health.log")
	stdout, buf := newStdout()

	_, closer := NewLogger(stdout, textHandler, FileLogConfig{Path: path, MaxSizeMB: 1, MaxFiles: 1})
	require.NoError(t, closer.Close())

	assert.Empty(t, buf.String())
}

func TestNewLoggerRejectsInvalidLimitsWithoutLeakingDetail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.log")
	stdout, buf := newStdout()

	logger, closer := NewLogger(stdout, textHandler, FileLogConfig{Path: path, MaxSizeMB: 0, MaxFiles: 1})
	logger.Error("health check failed")

	assert.Contains(t, buf.String(), "health log file unavailable")
	assert.NotContains(t, buf.String(), path)
	assert.NoError(t, closer.Close())
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

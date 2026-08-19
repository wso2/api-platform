/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package publishers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

func fileSinkCfg(path string, maxSizeMB int) config.TrafficLogFileConfig {
	return config.TrafficLogFileConfig{Path: path, MaxSizeMB: maxSizeMB}
}

func readFileLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func TestFileSink_WritesLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.log")
	s, err := newFileSink(fileSinkCfg(path, 100))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	s.Write([]byte(`{"a":1}`))
	s.Write([]byte(`{"b":2}`))

	assert.Equal(t, []string{`{"a":1}`, `{"b":2}`}, readFileLines(t, path))
	assert.Equal(t, config.TrafficLogSinkFile, s.Name())
}

// The file carries request/response bodies, so the mode must be owner-only and
// must be established at creation rather than chmod'd afterwards.
func TestFileSink_CreatesRestrictivePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "traffic")
	path := filepath.Join(dir, "traffic.log")
	s, err := newFileSink(fileSinkCfg(path, 100))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	fi, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm(), "traffic log must be owner read/write only")

	di, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), di.Mode().Perm(), "traffic log directory must be owner-only")
}

func TestFileSink_RejectsBadPaths(t *testing.T) {
	for name, path := range map[string]string{
		"empty":     "",
		"relative":  "relative/traffic.log",
		"null byte": "/tmp/traf\x00fic.log",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := newFileSink(fileSinkCfg(path, 100))
			assert.Error(t, err)
		})
	}
}

// Rotation renames the live file aside and reopens, keeping exactly one backup, so
// the on-disk total stays bounded at 2 x max_size_mb.
func TestFileSink_RotatesAtCeiling(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.log")
	s, err := newFileSink(fileSinkCfg(path, 1)) // 1 MiB
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	line := []byte(strings.Repeat("x", 100*1024)) // 100 KiB + newline
	for i := 0; i < 12; i++ {                     // ~1.2 MiB total -> one rotation
		s.Write(line)
	}

	backup := path + rotatedSuffix
	require.FileExists(t, backup, "rotation must leave exactly one backup")

	live, err := os.Stat(path)
	require.NoError(t, err)
	assert.Less(t, live.Size(), int64(bytesPerMiB), "live file must be under the ceiling after rotation")

	rotated, err := os.Stat(backup)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), rotated.Mode().Perm(), "rotation must not widen permissions")
	assert.Equal(t, os.FileMode(0o600), live.Mode().Perm())
}

// A second rotation clobbers the previous backup rather than accumulating: each
// extra backup would be another copy of PII at rest.
func TestFileSink_SecondRotationClobbersBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "traffic.log")
	s, err := newFileSink(fileSinkCfg(path, 1))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	line := []byte(strings.Repeat("y", 100*1024))
	for i := 0; i < 25; i++ { // ~2.5 MiB -> two rotations
		s.Write(line)
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "only the live file and a single backup may exist")
}

// A line larger than the whole ceiling must land in a freshly rotated file and stay
// there — not trigger a rotation on every subsequent write.
func TestFileSink_OversizedLineDoesNotChurnRotations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "traffic.log")
	s, err := newFileSink(fileSinkCfg(path, 1))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	oversized := []byte(strings.Repeat("z", 2*int(bytesPerMiB)))
	s.Write(oversized) // size was 0, so no rotation; file is now over the ceiling
	s.Write(oversized) // rotates once, then writes

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	lines := readFileLines(t, path)
	require.Len(t, lines, 1, "the oversized line must be present, not dropped")
	assert.Len(t, lines[0], 2*int(bytesPerMiB))
}

// After a restart the sink appends to whatever the previous run left behind, and
// must seed its size counter from the file so the ceiling still binds.
func TestFileSink_SeedsSizeFromExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.log")
	require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("a", 900*1024)+"\n"), 0o600))

	s, err := newFileSink(fileSinkCfg(path, 1))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	assert.Greater(t, s.size, int64(900*1024), "size must be seeded from the existing file")

	s.Write([]byte(strings.Repeat("b", 200*1024))) // pushes past 1 MiB -> rotates
	require.FileExists(t, path+rotatedSuffix)
}

func TestFileSink_MaxSizeZeroDisablesRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "traffic.log")
	s, err := newFileSink(fileSinkCfg(path, 0))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	line := []byte(strings.Repeat("q", 200*1024))
	for i := 0; i < 10; i++ {
		s.Write(line)
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "rotation disabled -> no backup should ever appear")
}

func TestFileSink_CloseIsIdempotentAndStopsWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.log")
	s, err := newFileSink(fileSinkCfg(path, 100))
	require.NoError(t, err)

	s.Write([]byte(`{"before":true}`))
	require.NoError(t, s.Close(context.Background()))
	require.NoError(t, s.Close(context.Background()), "Close must be idempotent")

	s.Write([]byte(`{"after":true}`)) // must not panic
	assert.Equal(t, []string{`{"before":true}`}, readFileLines(t, path))
}

func TestFileSink_ConcurrentWritesProduceWholeLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.log")
	s, err := newFileSink(fileSinkCfg(path, 100))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			s.Write([]byte(fmt.Sprintf(`{"i":%d}`, i)))
		}(i)
	}
	wg.Wait()

	lines := readFileLines(t, path)
	assert.Len(t, lines, n)
	for _, line := range lines {
		assert.True(t, strings.HasPrefix(line, `{"i":`) && strings.HasSuffix(line, `}`),
			"concurrent writes must not interleave: got %q", line)
	}
}

// The entrypoint prefixes the policy engine's stdout with "[pol] ", which is what
// forced a fragile strip-the-prefix parser downstream and previously let an
// unparsed line through. The file sink bypasses that wrapper, so what lands on disk
// must be the raw JSON with no prefix of any kind.
func TestFileSink_WritesRawJSONWithoutComponentPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.log")
	s, err := newFileSink(fileSinkCfg(path, 100))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	s.Write([]byte(`{"requestBody":"hello"}`))

	lines := readFileLines(t, path)
	require.Len(t, lines, 1)
	assert.Equal(t, `{"requestBody":"hello"}`, lines[0])
	assert.NotContains(t, lines[0], "[pol]")
}

// TestFileSink_RenameFailureKeepsWriting pins the recovery rotate() documents.
// Previously rotate() reopened a usable handle but still returned the rename
// error, and Write dropped on ANY error — so the "keep working" path dropped
// every subsequent line, exactly what it claimed to avoid.
func TestFileSink_RenameFailureKeepsWriting(t *testing.T) {
	if os.Geteuid() == 0 {
		// root ignores directory permissions, so the rename would succeed and
		// this test would assert the opposite of what it is checking.
		t.Skip("running as root: directory permission checks do not apply")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "traffic.log")
	s, err := newFileSink(config.TrafficLogFileConfig{Path: path, MaxSizeMB: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	// Push past the ceiling so the next write attempts a rotation.
	big := bytes.Repeat([]byte("x"), 1<<20)
	s.Write(big)

	// Renaming needs write permission on the DIRECTORY; appending to the
	// already-open file does not. This makes rotation fail while leaving the
	// sink perfectly able to write.
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	before, err := os.Stat(path)
	require.NoError(t, err)
	s.Write([]byte(`{"after":"rename-failure"}`))
	after, err := os.Stat(path)
	require.NoError(t, err)

	assert.Greater(t, after.Size(), before.Size(),
		"a line must still be written when rotation fails but the handle is usable")
	assert.NoFileExists(t, path+rotatedSuffix, "the rename did not succeed, so no backup should exist")
}

// TestFileSink_RejectsPreExistingPermissiveFile pins CodeRabbit #4: O_CREATE
// applies its mode only when it creates, so a file left behind by an earlier run
// keeps its old mode. A world-readable file holding request and response bodies
// defeats the reason for choosing this sink, so construction must fail.
func TestFileSink_RejectsPreExistingPermissiveFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission checks do not apply")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "traffic.log")
	require.NoError(t, os.WriteFile(path, []byte("from a previous run\n"), 0o644))

	_, err := newFileSink(config.TrafficLogFileConfig{Path: path, MaxSizeMB: 10})
	require.Error(t, err, "a group/other-readable traffic log must not be opened")
	assert.Contains(t, err.Error(), "chmod 600", "the error must say how to fix it")

	// Tightening it makes the same path acceptable.
	require.NoError(t, os.Chmod(path, 0o600))
	s, err := newFileSink(config.TrafficLogFileConfig{Path: path, MaxSizeMB: 10})
	require.NoError(t, err)
	_ = s.Close(context.Background())
}

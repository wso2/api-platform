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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestWriter opens a writer under a temporary directory with a byte-sized threshold,
// driven by a fake clock.
func newTestWriter(t *testing.T, maxBytes int64, maxFiles int) (*RotatingWriter, string, *fakeClock) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "logs", "health.log")
	w, err := OpenRotatingWriter(path, 1, maxFiles)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	clk := newFakeClock()
	w.now = clk.Now
	w.lastCheck = clk.Now()
	w.maxBytes = maxBytes
	return w, path, clk
}

func line(i int) []byte {
	return []byte(fmt.Sprintf("line-%04d\n", i)) // 10 bytes
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func TestOpenRotatingWriterRejectsInvalidLimits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.log")
	for _, tt := range []struct{ size, files int }{{0, 5}, {10, 0}, {-1, -1}} {
		_, err := OpenRotatingWriter(path, tt.size, tt.files)
		assert.Error(t, err, "size=%d files=%d", tt.size, tt.files)
	}
}

func TestOpenRotatingWriterCreatesOwnerOnlyFileAndDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	_, path, _ := newTestWriter(t, 100, 2)

	fi, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, logFileMode, fi.Mode().Perm())

	di, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	assert.Equal(t, logDirMode, di.Mode().Perm())
}

func TestOpenRotatingWriterFailsOnUnwritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced for a non-root user")
	}
	parent := t.TempDir()
	require.NoError(t, os.Chmod(parent, 0o500))
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	_, err := OpenRotatingWriter(filepath.Join(parent, "logs", "health.log"), 1, 1)
	assert.Error(t, err)
}

func TestOpenRotatingWriterAppendsToExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.log")
	require.NoError(t, os.WriteFile(path, []byte("earlier\n"), 0o600))

	w, err := OpenRotatingWriter(path, 1, 1)
	require.NoError(t, err)
	defer w.Close()
	_, _ = w.Write([]byte("later\n"))

	assert.Equal(t, "earlier\nlater\n", readFile(t, path))
	assert.Equal(t, int64(len("earlier\nlater\n")), w.size)
}

func TestRotatingWriterRotatesAndKeepsBackups(t *testing.T) {
	// 30-byte threshold, 10-byte lines: three lines fit, the fourth rotates.
	w, path, _ := newTestWriter(t, 30, 2)

	for i := 1; i <= 10; i++ {
		n, err := w.Write(line(i))
		require.NoError(t, err)
		require.Equal(t, 10, n)
	}

	assert.Equal(t, string(line(10)), readFile(t, path))
	assert.Equal(t, string(line(7))+string(line(8))+string(line(9)), readFile(t, path+".1"))
	assert.Equal(t, string(line(4))+string(line(5))+string(line(6)), readFile(t, path+".2"))
	_, err := os.Stat(path + ".3")
	assert.True(t, os.IsNotExist(err), "backups beyond the configured count must be removed")
	assert.Equal(t, int64(0), w.Dropped())
}

func TestRotatingWriterBoundsDiskUse(t *testing.T) {
	const maxBytes, maxFiles = 50, 3
	w, path, _ := newTestWriter(t, maxBytes, maxFiles)

	for i := 0; i < 500; i++ {
		_, _ = w.Write(line(i))
	}

	var total int64
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	for _, e := range entries {
		info, err := e.Info()
		require.NoError(t, err)
		total += info.Size()
	}
	assert.Len(t, entries, maxFiles+1)
	assert.LessOrEqual(t, total, int64(maxBytes*(maxFiles+1)))
}

func TestRotatingWriterKeepsOversizedLineInFreshFile(t *testing.T) {
	w, path, _ := newTestWriter(t, 30, 2)
	big := []byte(strings.Repeat("x", 45) + "\n")

	_, _ = w.Write(line(1))
	_, _ = w.Write(big)
	_, _ = w.Write(line(2))

	assert.Equal(t, string(line(1)), readFile(t, path+".2"))
	assert.Equal(t, string(big), readFile(t, path+".1"))
	assert.Equal(t, string(line(2)), readFile(t, path))
}

func TestRotatingWriterReopensWhenFileRemoved(t *testing.T) {
	w, path, clk := newTestWriter(t, 1000, 1)
	_, _ = w.Write(line(1))
	require.NoError(t, os.Remove(path))

	// Within the check interval the old descriptor is still used, so the line is lost
	// with the removed file; that bounds the check to one stat per interval.
	clk.Advance(reopenCheckInterval / 2)
	_, _ = w.Write(line(2))
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))

	clk.Advance(reopenCheckInterval)
	_, _ = w.Write(line(3))
	assert.Equal(t, string(line(3)), readFile(t, path))
}

func TestRotatingWriterReopensWhenFileReplaced(t *testing.T) {
	w, path, clk := newTestWriter(t, 1000, 1)
	_, _ = w.Write(line(1))
	require.NoError(t, os.Rename(path, path+".moved"))
	require.NoError(t, os.WriteFile(path, nil, 0o600))

	clk.Advance(reopenCheckInterval)
	_, _ = w.Write(line(2))

	assert.Equal(t, string(line(1)), readFile(t, path+".moved"))
	assert.Equal(t, string(line(2)), readFile(t, path))
	assert.Equal(t, int64(len(line(2))), w.size)
}

func TestRotatingWriterDropsWhileFileUnavailable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced for a non-root user")
	}
	w, path, clk := newTestWriter(t, 1000, 1)
	dir := filepath.Dir(path)
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	clk.Advance(reopenCheckInterval)
	n, err := w.Write(line(1))

	assert.NoError(t, err, "a failed health log write must never surface to the caller")
	assert.Equal(t, len(line(1)), n)
	assert.Equal(t, int64(1), w.Dropped())

	require.NoError(t, os.Chmod(dir, 0o700))
	clk.Advance(reopenCheckInterval)
	_, _ = w.Write(line(2))
	assert.Equal(t, string(line(2)), readFile(t, path), "writing resumes once the file can be opened again")
}

func TestRotatingWriterConcurrentWriters(t *testing.T) {
	w, path, _ := newTestWriter(t, 200, 50)
	const writers, perWriter = 8, 50

	var wg sync.WaitGroup
	for g := 0; g < writers; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				_, _ = w.Write([]byte(fmt.Sprintf("w%d-%04d\n", g, i)))
			}
		}(g)
	}
	wg.Wait()
	require.NoError(t, w.Close())

	var all bytes.Buffer
	all.WriteString(readFile(t, path))
	for i := 1; i <= 50; i++ {
		if b, err := os.ReadFile(fmt.Sprintf("%s.%d", path, i)); err == nil {
			all.Write(b)
		}
	}
	lines := strings.Split(strings.TrimSpace(all.String()), "\n")
	assert.Len(t, lines, writers*perWriter, "no line may be lost or interleaved")
	for _, l := range lines {
		assert.Regexp(t, `^w\d-\d{4}$`, l)
	}
}

func TestRotatingWriterDropsAfterClose(t *testing.T) {
	w, path, _ := newTestWriter(t, 1000, 1)
	_, _ = w.Write(line(1))
	require.NoError(t, w.Close())
	require.NoError(t, w.Close(), "closing twice is harmless")

	n, err := w.Write(line(2))

	assert.NoError(t, err)
	assert.Equal(t, len(line(2)), n)
	assert.Equal(t, int64(1), w.Dropped())
	assert.Equal(t, string(line(1)), readFile(t, path))
}

func TestRotatingWriterUsesRealClockByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.log")
	w, err := OpenRotatingWriter(path, 1, 1)
	require.NoError(t, err)
	defer w.Close()

	assert.WithinDuration(t, time.Now(), w.lastCheck, time.Minute)
	assert.Equal(t, bytesPerMiB, w.maxBytes)
}

func TestRotatingWriterDropsWhenRotationFails(t *testing.T) {
	w, path, clk := newTestWriter(t, 30, 1)
	// A non-empty directory where the oldest backup belongs cannot be removed, so the
	// rotation fails before anything is renamed.
	blocker := path + ".1"
	require.NoError(t, os.MkdirAll(filepath.Join(blocker, "keep"), 0o700))

	for i := 1; i <= 3; i++ {
		_, _ = w.Write(line(i))
	}
	n, err := w.Write(line(4))

	assert.NoError(t, err)
	assert.Equal(t, len(line(4)), n)
	assert.Equal(t, int64(1), w.Dropped())
	assert.Equal(t, string(line(1))+string(line(2))+string(line(3)), readFile(t, path),
		"a failed rotation must leave the live file untouched")

	// Once the obstruction is gone, the next check reopens the file and rotation works.
	require.NoError(t, os.RemoveAll(blocker))
	clk.Advance(reopenCheckInterval)
	_, _ = w.Write(line(5))
	assert.Equal(t, string(line(5)), readFile(t, path))
	assert.Equal(t, string(line(1))+string(line(2))+string(line(3)), readFile(t, path+".1"))
}

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

package logcapture

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

// --- Sink ---

func TestNewSinkRejectsEmptyRoot(t *testing.T) {
	_, err := NewSink("   ")
	require.Error(t, err)
}

func TestNewSinkIsolatesEachRunBeneathTheRoot(t *testing.T) {
	root := t.TempDir()
	first, err := NewSink(root)
	require.NoError(t, err)
	second, err := NewSink(root)
	require.NoError(t, err)

	require.NotEqual(t, first.Root(), second.Root())
	require.True(t, strings.HasPrefix(first.Root(), root))
	require.True(t, strings.HasPrefix(second.Root(), root))
}

func TestSinkRootIsNilSafe(t *testing.T) {
	var s *Sink
	require.Equal(t, "", s.Root())
}

func TestFileForRejectsAnUninitializedSink(t *testing.T) {
	var s *Sink
	_, err := s.FileFor("block")
	require.Error(t, err)

	empty := &Sink{}
	_, err = empty.FileFor("block")
	require.Error(t, err)
}

func TestFileForRejectsAnEmptyBlockName(t *testing.T) {
	sink, err := NewSink(t.TempDir())
	require.NoError(t, err)

	_, err = sink.FileFor("  ")
	require.Error(t, err)
}

func TestFileForCreatesTheContainingDirectory(t *testing.T) {
	sink, err := NewSink(t.TempDir())
	require.NoError(t, err)

	path, err := sink.FileFor("platform-api-gateway/sqlite")
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(path, ".log"))

	info, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestFileForRejectsATraversalSegment(t *testing.T) {
	sink, err := NewSink(t.TempDir())
	require.NoError(t, err)

	_, err = sink.FileFor("../../etc/passwd")
	require.Error(t, err)
}

func TestFileForReplacesMatrixVariantSeparators(t *testing.T) {
	sink, err := NewSink(t.TempDir())
	require.NoError(t, err)

	path, err := sink.FileFor("platform-api-gateway/sqlite")
	require.NoError(t, err)
	base := filepath.Base(path)
	// A collision-resistance hash suffix is appended whenever sanitization changes the
	// name (here, "/" -> "-"), matching coverage.sanitize's same behavior.
	require.True(t, strings.HasPrefix(base, "platform-api-gateway-sqlite-"))
	require.True(t, strings.HasSuffix(base, ".log"))
	require.NotContains(t, base, "/")
}

func TestFileForRejectsANullByte(t *testing.T) {
	sink, err := NewSink(t.TempDir())
	require.NoError(t, err)

	_, err = sink.FileFor("bad\x00name")
	require.Error(t, err)
}

func TestFileForIsSafeUnderConcurrency(t *testing.T) {
	sink, err := NewSink(t.TempDir())
	require.NoError(t, err)

	const count = 20
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := sink.FileFor("block-" + string(rune('a'+n)))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

// --- Writer ---

func TestNewWriterRejectsAnEmptyPath(t *testing.T) {
	_, err := NewWriter("")
	require.Error(t, err)
}

func TestWriterWritesPrefixedInterleavedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "block.log")
	w, err := NewWriter(path)
	require.NoError(t, err)

	w.Consumer("platform-api").Accept(testcontainers.Log{Content: []byte("booting\n")})
	w.Consumer("gateway-runtime").Accept(testcontainers.Log{Content: []byte("listening")})
	w.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(data)
	require.Contains(t, text, "[platform-api] booting\n")
	require.Contains(t, text, "[gateway-runtime] listening\n") // a missing trailing newline is added
}

// TestAcceptCopiesContentRatherThanRetainingTheSourceBuffer guards against exactly what
// testcontainers-go's own log pipeline does: stdcopy.StdCopy reuses one buffer across
// the whole stream, and logConsumerWriter.Write hands Accept a slice into it. Storing
// that slice by reference would let a later line silently corrupt an earlier, still-
// queued one before the writer goroutine drains it.
func TestAcceptCopiesContentRatherThanRetainingTheSourceBuffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reused-buffer.log")
	w, err := NewWriter(path)
	require.NoError(t, err)
	consumer := w.Consumer("svc")

	shared := []byte("first line")
	consumer.Accept(testcontainers.Log{Content: shared})
	copy(shared, "SECOND!!!!") // mutates the same backing array a real reused buffer would
	consumer.Accept(testcontainers.Log{Content: shared})

	w.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "[svc] first line\n")
	require.Contains(t, string(data), "[svc] SECOND!!!!\n")
}

func TestWriterCloseIsSafeWithNoLinesWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.log")
	w, err := NewWriter(path)
	require.NoError(t, err)
	w.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Empty(t, data)
	require.Equal(t, int64(0), w.Dropped())
}

func TestWriterDroppedIsNilSafe(t *testing.T) {
	var w *Writer
	require.Equal(t, int64(0), w.Dropped())
}

// TestConsumerAcceptNeverBlocksOnAFullBuffer builds a Writer whose channel is never
// drained (no run() goroutine started) so a full buffer's overflow behavior is
// deterministic, rather than racing a live writer goroutine.
func TestConsumerAcceptNeverBlocksOnAFullBuffer(t *testing.T) {
	w := &Writer{lines: make(chan logLine, 2)}
	consumer := w.Consumer("svc")

	consumer.Accept(testcontainers.Log{Content: []byte("one")})
	consumer.Accept(testcontainers.Log{Content: []byte("two")})
	consumer.Accept(testcontainers.Log{Content: []byte("three")}) // buffer full, must drop, not block

	require.Equal(t, int64(1), w.Dropped())
	require.Len(t, w.lines, 2)
}

// TestAcceptDuringAndAfterCloseNeverPanicsOrBlocks guards against exactly the failure a
// container's own log-producer goroutine can trigger: it keeps calling Accept for as
// long as the container exists, independent of when the block closes its Writer. A
// naive close(lines) implementation panics here ("send on closed channel"). The producer
// goroutine sends a bounded number of lines and stops on its own — like a real
// container's log stream eventually does — rather than spinning until Close returns,
// which would let an arbitrarily fast producer starve Close of ever completing.
func TestAcceptDuringAndAfterCloseNeverPanicsOrBlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "race.log")
	w, err := NewWriter(path)
	require.NoError(t, err)
	consumer := w.Consumer("svc")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10_000; i++ {
			consumer.Accept(testcontainers.Log{Content: []byte("still logging")})
		}
	}()

	w.Close() // races the goroutine above — must not panic, and must not block on it
	wg.Wait()

	consumer.Accept(testcontainers.Log{Content: []byte("after close")}) // must not panic or block
}

func TestWriterSummarizesDroppedLinesOnClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dropped.log")
	w, err := NewWriter(path)
	require.NoError(t, err)
	w.dropped.Add(3)
	w.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "dropped 3 line(s)")
}

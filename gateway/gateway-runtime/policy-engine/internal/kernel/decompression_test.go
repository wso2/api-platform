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

package kernel

import (
	"bytes"
	"compress/gzip"
	"math"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMaxDecompressedBytes is a generous decompression ceiling used by tests that
// exercise correctness rather than the bomb guard itself.
const testMaxDecompressedBytes int64 = 10 * 1024 * 1024

// gzipCompress is a test helper that gzip-compresses a byte slice.
func gzipCompress(data []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}

// brotliCompress is a test helper that brotli-compresses a byte slice.
func brotliCompress(data []byte) []byte {
	var buf bytes.Buffer
	w := brotli.NewWriter(&buf)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}

// =============================================================================
// decompressBody Tests
// =============================================================================

func TestDecompressBody_Gzip(t *testing.T) {
	original := []byte(`{"model":"gpt-4","usage":{"prompt_tokens":10,"completion_tokens":20}}`)
	compressed := gzipCompress(original)

	result, err := decompressBody(compressed, "gzip", testMaxDecompressedBytes)

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestDecompressBody_Brotli(t *testing.T) {
	original := []byte(`{"model":"claude-3","usage":{"input_tokens":5,"output_tokens":15}}`)
	compressed := brotliCompress(original)

	result, err := decompressBody(compressed, "br", testMaxDecompressedBytes)

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestDecompressBody_UnknownEncoding_PassesThrough(t *testing.T) {
	original := []byte(`{"model":"gemini-pro"}`)

	result, err := decompressBody(original, "identity", testMaxDecompressedBytes)

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestDecompressBody_EmptyEncoding_PassesThrough(t *testing.T) {
	original := []byte(`{"model":"gemini-pro"}`)

	result, err := decompressBody(original, "", testMaxDecompressedBytes)

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestDecompressBody_InvalidGzip_ReturnsError(t *testing.T) {
	garbage := []byte("this is not gzip data")

	_, err := decompressBody(garbage, "gzip", testMaxDecompressedBytes)

	assert.Error(t, err)
}

// =============================================================================
// recompressBody Tests
// =============================================================================

func TestRecompressBody_Gzip_RoundTrip(t *testing.T) {
	original := []byte(`{"model":"gpt-4","usage":{"prompt_tokens":10}}`)

	compressed, err := recompressBody(original, "gzip")
	require.NoError(t, err)
	assert.NotEqual(t, original, compressed)

	// Decompress and verify we get the original back
	restored, err := decompressBody(compressed, "gzip", testMaxDecompressedBytes)
	require.NoError(t, err)
	assert.Equal(t, original, restored)
}

func TestRecompressBody_Brotli_RoundTrip(t *testing.T) {
	original := []byte(`{"model":"claude-3","usage":{"input_tokens":5}}`)

	compressed, err := recompressBody(original, "br")
	require.NoError(t, err)
	assert.NotEqual(t, original, compressed)

	// Decompress and verify we get the original back
	restored, err := decompressBody(compressed, "br", testMaxDecompressedBytes)
	require.NoError(t, err)
	assert.Equal(t, original, restored)
}

func TestRecompressBody_UnknownEncoding_PassesThrough(t *testing.T) {
	original := []byte(`{"model":"gemini-pro"}`)

	result, err := recompressBody(original, "identity")

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestRecompressBody_EmptyEncoding_PassesThrough(t *testing.T) {
	original := []byte(`{"model":"gemini-pro"}`)

	result, err := recompressBody(original, "")

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

// =============================================================================
// streamDecompressor Tests
// =============================================================================

// TestStreamDecompressor_Gzip_AllInOneChunk feeds the entire compressed body as
// a single EOS chunk — the simplest path through the streaming decompressor.
func TestStreamDecompressor_Gzip_AllInOneChunk(t *testing.T) {
	original := []byte(`{"model":"claude","usage":{"input_tokens":10,"output_tokens":20}}`)
	compressed := gzipCompress(original)

	sd := newStreamDecompressor("gzip", testMaxDecompressedBytes)
	result, err := sd.FeedChunk(compressed, true)

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

// TestStreamDecompressor_Gzip_MultipleChunks splits the compressed stream across
// two chunks to verify that the persistent goroutine decoder maintains state
// between calls and produces the correct output when all data is assembled.
func TestStreamDecompressor_Gzip_MultipleChunks(t *testing.T) {
	original := []byte(`{"model":"claude","usage":{"input_tokens":10,"output_tokens":20}}`)
	compressed := gzipCompress(original)

	sd := newStreamDecompressor("gzip", testMaxDecompressedBytes)

	half := len(compressed) / 2
	chunk1, err := sd.FeedChunk(compressed[:half], false)
	require.NoError(t, err)

	chunk2, err := sd.FeedChunk(compressed[half:], true)
	require.NoError(t, err)

	// The decoder may produce output on either chunk depending on DEFLATE block
	// boundaries — concatenating both chunks must equal the original.
	assert.Equal(t, original, append(chunk1, chunk2...))
}

// TestStreamDecompressor_Brotli_AllInOneChunk verifies brotli decoding works
// with the same io.Pipe + goroutine pattern as gzip.
func TestStreamDecompressor_Brotli_AllInOneChunk(t *testing.T) {
	original := []byte(`{"model":"claude","usage":{"input_tokens":5,"output_tokens":15}}`)
	compressed := brotliCompress(original)

	sd := newStreamDecompressor("br", testMaxDecompressedBytes)
	result, err := sd.FeedChunk(compressed, true)

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

// TestStreamDecompressor_Brotli_MultipleChunks verifies brotli handles split chunks.
func TestStreamDecompressor_Brotli_MultipleChunks(t *testing.T) {
	original := []byte(`{"model":"claude","usage":{"input_tokens":5,"output_tokens":15}}`)
	compressed := brotliCompress(original)

	sd := newStreamDecompressor("br", testMaxDecompressedBytes)

	half := len(compressed) / 2
	chunk1, err := sd.FeedChunk(compressed[:half], false)
	require.NoError(t, err)

	chunk2, err := sd.FeedChunk(compressed[half:], true)
	require.NoError(t, err)

	assert.Equal(t, original, append(chunk1, chunk2...))
}

// TestStreamDecompressor_EmptyNonEOSChunk verifies that feeding an empty
// non-EOS chunk returns empty output without error — this happens when the
// decoder needs more input before a full DEFLATE block can be produced.
func TestStreamDecompressor_EmptyNonEOSChunk(t *testing.T) {
	sd := newStreamDecompressor("gzip", testMaxDecompressedBytes)

	result, err := sd.FeedChunk(nil, false)

	require.NoError(t, err)
	assert.Empty(t, result)

	sd.Close()
}

// TestStreamDecompressor_UnknownEncoding_Passthrough verifies that an unknown
// encoding causes the raw bytes to pass through unchanged.
func TestStreamDecompressor_UnknownEncoding_Passthrough(t *testing.T) {
	original := []byte(`plain text, not compressed`)

	sd := newStreamDecompressor("identity", testMaxDecompressedBytes)
	result, err := sd.FeedChunk(original, true)

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

// TestStreamDecompressor_Close_DoesNotHang verifies that Close() terminates
// the background goroutine promptly even when no data has been fed.
func TestStreamDecompressor_Close_DoesNotHang(t *testing.T) {
	sd := newStreamDecompressor("gzip", testMaxDecompressedBytes)

	done := make(chan struct{})
	go func() {
		sd.Close()
		close(done)
	}()

	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("Close() hung — background goroutine was not released")
	}
}

// TestStreamDecompressor_RoundTrip verifies the full cycle the streaming path
// performs: incoming compressed bytes → decompress → policy modifies → recompress.
// The final recompressed bytes must decompress back to the (modified) original.
func TestStreamDecompressor_RoundTrip(t *testing.T) {
	original := []byte(`{"model":"claude","usage":{"input_tokens":10}}`)
	compressed := gzipCompress(original)

	// Decompress (as processStreamingResponseBody does)
	sd := newStreamDecompressor("gzip", testMaxDecompressedBytes)
	decompressed, err := sd.FeedChunk(compressed, true)
	require.NoError(t, err)
	assert.Equal(t, original, decompressed)

	// Recompress (as TranslateStreamingResponseChunkAction does)
	recompressed, err := recompressBody(decompressed, "gzip")
	require.NoError(t, err)

	// Must be valid gzip that decodes back to original
	final, err := decompressBody(recompressed, "gzip", testMaxDecompressedBytes)
	require.NoError(t, err)
	assert.Equal(t, original, final)
}

// =============================================================================
// Decompression-bomb guard tests
// =============================================================================

// bombLimit is a small ceiling used to trigger the guard deterministically.
const bombLimit int64 = 1024

// TestDecompressBody_Gzip_ExceedsLimit_ReturnsError feeds a highly compressible
// payload (1 MiB of zeros → a few hundred compressed bytes) that expands far past
// the ceiling — the classic decompression bomb. It must be rejected, not buffered.
func TestDecompressBody_Gzip_ExceedsLimit_ReturnsError(t *testing.T) {
	original := make([]byte, 1024*1024)
	compressed := gzipCompress(original)

	_, err := decompressBody(compressed, "gzip", bombLimit)

	require.ErrorIs(t, err, ErrDecompressedTooLarge)
}

func TestDecompressBody_Brotli_ExceedsLimit_ReturnsError(t *testing.T) {
	original := make([]byte, 1024*1024)
	compressed := brotliCompress(original)

	_, err := decompressBody(compressed, "br", bombLimit)

	require.ErrorIs(t, err, ErrDecompressedTooLarge)
}

// TestDecompressBody_ExactlyAtLimit_Succeeds verifies the boundary is inclusive:
// a body whose decompressed size equals the limit is accepted in full.
func TestDecompressBody_ExactlyAtLimit_Succeeds(t *testing.T) {
	original := make([]byte, bombLimit)
	for i := range original {
		original[i] = byte(i)
	}
	compressed := gzipCompress(original)

	result, err := decompressBody(compressed, "gzip", bombLimit)

	require.NoError(t, err)
	assert.Len(t, result, int(bombLimit))
}

// TestDecompressBody_OneOverLimit_ReturnsError verifies a body one byte past the
// limit is rejected (no silent truncation).
func TestDecompressBody_OneOverLimit_ReturnsError(t *testing.T) {
	original := make([]byte, bombLimit+1)
	for i := range original {
		original[i] = byte(i)
	}
	compressed := gzipCompress(original)

	_, err := decompressBody(compressed, "gzip", bombLimit)

	require.ErrorIs(t, err, ErrDecompressedTooLarge)
}

// TestDecompressBody_ZeroLimit_Unbounded verifies maxBytes <= 0 disables the bound.
func TestDecompressBody_ZeroLimit_Unbounded(t *testing.T) {
	original := make([]byte, 1024*1024)
	compressed := gzipCompress(original)

	result, err := decompressBody(compressed, "gzip", 0)

	require.NoError(t, err)
	assert.Len(t, result, len(original))
}

// TestDecompressBody_MaxIntLimit_DoesNotOverflow verifies that the one-byte
// overflow probe remains correct at the largest accepted int64 ceiling.
func TestDecompressBody_MaxIntLimit_DoesNotOverflow(t *testing.T) {
	original := []byte("small compressed payload")
	compressed := gzipCompress(original)

	result, err := decompressBody(compressed, "gzip", math.MaxInt64)

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

// TestStreamDecompressor_SingleBombChunk_ReturnsError verifies a single streamed
// chunk that expands beyond the per-chunk ceiling is rejected with the sentinel.
func TestStreamDecompressor_SingleBombChunk_ReturnsError(t *testing.T) {
	original := make([]byte, 1024*1024)
	compressed := gzipCompress(original)

	sd := newStreamDecompressor("gzip", bombLimit)
	defer sd.Close()

	_, err := sd.FeedChunk(compressed, true)

	require.ErrorIs(t, err, ErrDecompressedTooLarge)
}

// TestStreamDecompressor_NonEOSChunkExceedsLimitImmediately verifies that an
// oversized non-terminal chunk is charged to its own FeedChunk call, rather than
// leaving part of its output queued for a later call.
func TestStreamDecompressor_NonEOSChunkExceedsLimitImmediately(t *testing.T) {
	const limit int64 = 64 * 1024
	chunk := make([]byte, limit+1)

	sd := newStreamDecompressor("identity", limit)
	defer sd.Close()

	_, err := sd.FeedChunk(chunk, false)

	require.ErrorIs(t, err, ErrDecompressedTooLarge)
}

// TestStreamDecompressor_OutputBeyondChannelCapacity_DoesNotDeadlock verifies
// FeedChunk drains decoder output while writing input. The decoder channel holds
// 2 MiB, so a write-then-drain sequence would stall on this 3 MiB chunk even though
// it is below the configured limit.
func TestStreamDecompressor_OutputBeyondChannelCapacity_DoesNotDeadlock(t *testing.T) {
	const chunkSize = 3 * 1024 * 1024
	const limit int64 = 4 * 1024 * 1024

	sd := newStreamDecompressor("identity", limit)
	defer sd.Close()

	type result struct {
		body []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		body, err := sd.FeedChunk(make([]byte, chunkSize), true)
		done <- result{body: body, err: err}
	}()

	select {
	case got := <-done:
		require.NoError(t, got.err)
		assert.Len(t, got.body, chunkSize)
	case <-time.After(2 * time.Second):
		t.Fatal("FeedChunk deadlocked after decoder output filled outChan")
	}
}

// TestStreamDecompressor_OverflowBeyondChannelCapacity_ReturnsError verifies
// the limit is still enforced when output crosses it only after filling the
// decoder channel.
func TestStreamDecompressor_OverflowBeyondChannelCapacity_ReturnsError(t *testing.T) {
	const chunkSize = 3 * 1024 * 1024
	const limit int64 = 2 * 1024 * 1024

	sd := newStreamDecompressor("identity", limit)
	defer sd.Close()

	done := make(chan error, 1)
	go func() {
		_, err := sd.FeedChunk(make([]byte, chunkSize), true)
		done <- err
	}()

	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrDecompressedTooLarge)
	case <-time.After(2 * time.Second):
		t.Fatal("FeedChunk deadlocked before enforcing the decompression limit")
	}
}

// TestStreamDecompressor_LargeStreamUnderPerChunkLimit_Succeeds proves the guard is
// per-chunk, not a lifetime total: the cumulative output far exceeds the ceiling, yet
// because no single FeedChunk exceeds it, every chunk passes. A cumulative limit
// would wrongly reject this — the exact large-streaming case the design must allow.
//
// The decoder-input boundary ensures each call receives exactly its own identity
// output rather than output left queued by a previous call.
func TestStreamDecompressor_LargeStreamUnderPerChunkLimit_Succeeds(t *testing.T) {
	const limit int64 = 100 * 1024
	chunk := make([]byte, 10*1024) // 10 KiB — well under the per-chunk limit

	// identity passthrough: each FeedChunk returns exactly the bytes fed, making
	// the accounting deterministic.
	sd := newStreamDecompressor("identity", limit)
	defer sd.Close()

	var total int
	const chunks = 50 // 50 * 10 KiB = 500 KiB cumulative, 5x the per-chunk limit
	for i := 0; i < chunks; i++ {
		out, err := sd.FeedChunk(chunk, false)
		require.NoError(t, err)
		assert.Len(t, out, len(chunk), "chunk %d output must not be deferred to another call", i)
		total += len(out)
	}
	final, err := sd.FeedChunk(nil, true)
	require.NoError(t, err)
	total += len(final)

	assert.Equal(t, chunks*len(chunk), total, "all bytes should pass through despite exceeding the limit cumulatively")
	assert.Greater(t, int64(total), limit, "cumulative output must exceed the per-chunk limit for this test to be meaningful")
}

// TestStreamDecompressor_BombChunkAcrossFeeds_ReturnsError verifies the bomb is
// caught even when the compressed input is split across several non-EOS feeds
// before the terminating chunk.
func TestStreamDecompressor_BombChunkAcrossFeeds_ReturnsError(t *testing.T) {
	original := make([]byte, 1024*1024)
	compressed := gzipCompress(original)

	sd := newStreamDecompressor("gzip", bombLimit)
	defer sd.Close()

	var err error
	third := len(compressed) / 3
	for _, part := range [][]byte{compressed[:third], compressed[third : 2*third], compressed[2*third:]} {
		if _, err = sd.FeedChunk(part, false); err != nil {
			break
		}
	}
	if err == nil {
		_, err = sd.FeedChunk(nil, true)
	}

	require.ErrorIs(t, err, ErrDecompressedTooLarge)
}

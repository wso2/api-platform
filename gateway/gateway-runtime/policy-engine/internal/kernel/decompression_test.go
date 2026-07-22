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
	"io"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	result, err := decompressBody(compressed, "gzip")

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestDecompressBody_Brotli(t *testing.T) {
	original := []byte(`{"model":"claude-3","usage":{"input_tokens":5,"output_tokens":15}}`)
	compressed := brotliCompress(original)

	result, err := decompressBody(compressed, "br")

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestDecompressBody_UnknownEncoding_PassesThrough(t *testing.T) {
	original := []byte(`{"model":"gemini-pro"}`)

	result, err := decompressBody(original, "identity")

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestDecompressBody_EmptyEncoding_PassesThrough(t *testing.T) {
	original := []byte(`{"model":"gemini-pro"}`)

	result, err := decompressBody(original, "")

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestDecompressBody_InvalidGzip_ReturnsError(t *testing.T) {
	garbage := []byte("this is not gzip data")

	_, err := decompressBody(garbage, "gzip")

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
	restored, err := decompressBody(compressed, "gzip")
	require.NoError(t, err)
	assert.Equal(t, original, restored)
}

func TestRecompressBody_Brotli_RoundTrip(t *testing.T) {
	original := []byte(`{"model":"claude-3","usage":{"input_tokens":5}}`)

	compressed, err := recompressBody(original, "br")
	require.NoError(t, err)
	assert.NotEqual(t, original, compressed)

	// Decompress and verify we get the original back
	restored, err := decompressBody(compressed, "br")
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

	sd := newStreamDecompressor("gzip")
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

	sd := newStreamDecompressor("gzip")

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

	sd := newStreamDecompressor("br")
	result, err := sd.FeedChunk(compressed, true)

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

// TestStreamDecompressor_Brotli_MultipleChunks verifies brotli handles split chunks.
func TestStreamDecompressor_Brotli_MultipleChunks(t *testing.T) {
	original := []byte(`{"model":"claude","usage":{"input_tokens":5,"output_tokens":15}}`)
	compressed := brotliCompress(original)

	sd := newStreamDecompressor("br")

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
	sd := newStreamDecompressor("gzip")

	result, err := sd.FeedChunk(nil, false)

	require.NoError(t, err)
	assert.Empty(t, result)

	sd.Close()
}

// TestStreamDecompressor_UnknownEncoding_Passthrough verifies that an unknown
// encoding causes the raw bytes to pass through unchanged.
func TestStreamDecompressor_UnknownEncoding_Passthrough(t *testing.T) {
	original := []byte(`plain text, not compressed`)

	sd := newStreamDecompressor("identity")
	result, err := sd.FeedChunk(original, true)

	require.NoError(t, err)
	assert.Equal(t, original, result)
}

// TestStreamDecompressor_Close_DoesNotHang verifies that Close() terminates
// the background goroutine promptly even when no data has been fed.
func TestStreamDecompressor_Close_DoesNotHang(t *testing.T) {
	sd := newStreamDecompressor("gzip")

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
	sd := newStreamDecompressor("gzip")
	decompressed, err := sd.FeedChunk(compressed, true)
	require.NoError(t, err)
	assert.Equal(t, original, decompressed)

	// Recompress (as TranslateStreamingResponseChunkAction does)
	recompressed, err := recompressBody(decompressed, "gzip")
	require.NoError(t, err)

	// Must be valid gzip that decodes back to original
	final, err := decompressBody(recompressed, "gzip")
	require.NoError(t, err)
	assert.Equal(t, original, final)
}

// TestStreamDecompressor_ManyChunks_NoStall is the regression test for the
// mid-stream stall that broke large Claude Code tasks. The previous io.Pipe +
// bounded-channel (cap 64) design deadlocked once the decoder produced more
// undrained output blocks than the channel could hold while a chunk was still
// being consumed: the decoder goroutine blocked on the full channel, stopped
// reading the pipe, so the pipe Write never returned, so FeedChunk never drained
// the channel. It surfaced only on long/large responses (many chunks) — short
// ones never filled the channel, which is why the two-chunk tests missed it.
//
// This feeds a long stream as many separate sync-flushed chunks and asserts every
// byte comes back, within a deadline so a regression fails as a timeout, not a hang.
func TestStreamDecompressor_ManyChunks_NoStall(t *testing.T) {
	// Build a gzip stream flushed after each of many logical chunks, mirroring how
	// an upstream (Anthropic) emits a continuous gzip stream with per-event flushes.
	const numChunks = 500
	var raw bytes.Buffer
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	flushOffsets := make([]int, 0, numChunks)
	for i := 0; i < numChunks; i++ {
		// ~250 bytes per SSE event, including ~200 bytes of filler text.
		filler := string(bytes.Repeat([]byte("x"), 200))
		line := []byte(`data: {"type":"content_block_delta","index":0,` +
			`"delta":{"type":"text_delta","text":"token-` + filler + `"}}` + "\n\n")
		raw.Write(line)
		_, err := zw.Write(line)
		require.NoError(t, err)
		require.NoError(t, zw.Flush()) // sync-flush → a decodable boundary, like SSE
		flushOffsets = append(flushOffsets, compressed.Len())
	}
	require.NoError(t, zw.Close())

	sd := newStreamDecompressor("gzip")
	compBytes := compressed.Bytes()
	// Close() appended the gzip trailer after the last recorded flush offset — extend
	// the final segment to the true end so the EOS chunk carries the trailer.
	flushOffsets[len(flushOffsets)-1] = len(compBytes)

	// Feed the compressed stream in slices aligned to the flush boundaries, each as a
	// separate FeedChunk call — the shape that accumulated backlog in the old design.
	done := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		var out []byte
		prev := 0
		for i, off := range flushOffsets {
			eos := i == len(flushOffsets)-1
			part, err := sd.FeedChunk(compBytes[prev:off], eos)
			if err != nil {
				errCh <- err
				return
			}
			out = append(out, part...)
			prev = off
		}
		done <- out
	}()

	select {
	case err := <-errCh:
		t.Fatalf("FeedChunk returned error: %v", err)
	case out := <-done:
		assert.Equal(t, raw.Bytes(), out, "every decompressed byte must be returned across all chunks")
	case <-time.After(15 * time.Second):
		sd.Close()
		t.Fatal("streamDecompressor stalled on a long many-chunk stream (regression: mid-stream deadlock)")
	}
}

// TestStreamDecompressor_HighRatioChunk_NoStall feeds a single chunk that
// decompresses to far more than the old 64-block channel could buffer, exercising
// the second face of the same bug: one FeedChunk whose decoded output alone would
// have overrun the bounded channel mid-consume. With the unbounded output buffer
// this must complete promptly and return all bytes.
func TestStreamDecompressor_HighRatioChunk_NoStall(t *testing.T) {
	// 8 MiB of highly compressible data → tiny compressed input, ~256 blocks of
	// 32 KiB out — well past the old cap of 64.
	original := bytes.Repeat([]byte("A"), 8<<20)
	compressed := gzipCompress(original)

	sd := newStreamDecompressor("gzip")

	done := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		out, err := sd.FeedChunk(compressed, true)
		if err != nil {
			errCh <- err
			return
		}
		done <- out
	}()

	select {
	case err := <-errCh:
		t.Fatalf("FeedChunk returned error: %v", err)
	case out := <-done:
		assert.Equal(t, len(original), len(out))
		assert.True(t, bytes.Equal(original, out))
	case <-time.After(15 * time.Second):
		sd.Close()
		t.Fatal("streamDecompressor stalled on a high-ratio single chunk (regression: bounded-channel overrun)")
	}
}

// =============================================================================
// streamCompressor Tests
// =============================================================================

// gzipHeaderCount counts the number of gzip member headers (magic bytes 1f 8b 08)
// in a byte slice. A correct single continuous stream has exactly one.
func gzipHeaderCount(data []byte) int {
	return bytes.Count(data, []byte{0x1f, 0x8b, 0x08})
}

// singleMemberGunzip decodes only the FIRST gzip member, mimicking downstream HTTP
// decoders (such as the Claude Code client) that stop at the first member's trailer.
func singleMemberGunzip(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	zr.Multistream(false) // do NOT transparently continue into later members
	out, err := io.ReadAll(zr)
	require.NoError(t, err)
	return out
}

// TestStreamCompressor_Gzip_MultipleChunks is the regression test for the analytics
// streaming bug: multiple chunks must be re-compressed into ONE continuous gzip stream.
func TestStreamCompressor_Gzip_MultipleChunks(t *testing.T) {
	chunks := [][]byte{
		[]byte("event: message_start\ndata: {\"type\":\"message_start\"}\n\n"),
		[]byte("event: content_block_delta\ndata: {\"delta\":\"Hi\"}\n\n"),
		[]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"),
	}
	var full []byte
	for _, c := range chunks {
		full = append(full, c...)
	}

	sc := newStreamCompressor("gzip")
	var compressed []byte
	for i, c := range chunks {
		endOfStream := i == len(chunks)-1
		out, err := sc.FeedChunk(c, endOfStream)
		require.NoError(t, err)
		compressed = append(compressed, out...)
	}

	// The whole body must be a SINGLE gzip member — this is the property the bug violated.
	assert.Equal(t, 1, gzipHeaderCount(compressed),
		"streaming compression must emit exactly one gzip member, not one per chunk")

	// A single-member decoder (like the downstream client) must recover ALL chunks,
	// not just the first. This is what failed for Claude Code before the fix.
	assert.Equal(t, full, singleMemberGunzip(t, compressed))

	// And a normal (multistream) decode must also yield the full body.
	final, err := decompressBody(compressed, "gzip")
	require.NoError(t, err)
	assert.Equal(t, full, final)
}

// TestRecompressBody_PerChunk_ProducesMultipleMembers documents the OLD, buggy
// behaviour that streamCompressor replaces: re-compressing each chunk independently
// produces N gzip members, and a single-member decoder recovers only the first chunk.
func TestRecompressBody_PerChunk_ProducesMultipleMembers(t *testing.T) {
	chunks := [][]byte{
		[]byte("event: message_start\n\n"),
		[]byte("event: content_block_delta\n\n"),
		[]byte("event: message_stop\n\n"),
	}

	var buggy []byte
	for _, c := range chunks {
		out, err := recompressBody(c, "gzip")
		require.NoError(t, err)
		buggy = append(buggy, out...)
	}

	// Per-chunk recompress yields one member per chunk...
	assert.Equal(t, len(chunks), gzipHeaderCount(buggy))
	// ...and a single-member decoder sees only the first chunk — the dropped-stream bug.
	assert.Equal(t, chunks[0], singleMemberGunzip(t, buggy))
}

// TestStreamCompressor_Brotli_MultipleChunks verifies the brotli path round-trips
// across multiple chunks into a single continuous stream.
func TestStreamCompressor_Brotli_MultipleChunks(t *testing.T) {
	chunks := [][]byte{
		[]byte("first-chunk-payload"),
		[]byte("second-chunk-payload"),
		[]byte("third-chunk-payload"),
	}
	var full []byte
	for _, c := range chunks {
		full = append(full, c...)
	}

	sc := newStreamCompressor("br")
	var compressed []byte
	for i, c := range chunks {
		out, err := sc.FeedChunk(c, i == len(chunks)-1)
		require.NoError(t, err)
		compressed = append(compressed, out...)
	}

	final, err := decompressBody(compressed, "br")
	require.NoError(t, err)
	assert.Equal(t, full, final)
}

// TestStreamCompressor_Gzip_EmptyFinalChunk mirrors the real Envoy flow where the
// EndOfStream chunk carries zero bytes: the trailer must still be emitted so the
// stream is well-formed.
func TestStreamCompressor_Gzip_EmptyFinalChunk(t *testing.T) {
	sc := newStreamCompressor("gzip")

	out1, err := sc.FeedChunk([]byte("payload-one"), false)
	require.NoError(t, err)
	out2, err := sc.FeedChunk([]byte("payload-two"), false)
	require.NoError(t, err)
	// Final chunk with no data, only EndOfStream — as Envoy delivers it.
	out3, err := sc.FeedChunk(nil, true)
	require.NoError(t, err)

	compressed := append(append(append([]byte{}, out1...), out2...), out3...)
	assert.Equal(t, 1, gzipHeaderCount(compressed))
	assert.Equal(t, []byte("payload-onepayload-two"), singleMemberGunzip(t, compressed))
}

// TestStreamCompressor_UnknownEncoding_Passthrough verifies unknown encodings pass
// bytes through unchanged.
func TestStreamCompressor_UnknownEncoding_Passthrough(t *testing.T) {
	sc := newStreamCompressor("identity")
	out, err := sc.FeedChunk([]byte("raw-bytes"), true)
	require.NoError(t, err)
	assert.Equal(t, []byte("raw-bytes"), out)
}

// TestStreamCompressor_WriteAfterClose returns an error rather than corrupting output.
func TestStreamCompressor_WriteAfterClose(t *testing.T) {
	sc := newStreamCompressor("gzip")
	_, err := sc.FeedChunk([]byte("data"), true)
	require.NoError(t, err)
	_, err = sc.FeedChunk([]byte("more"), false)
	require.Error(t, err)
}

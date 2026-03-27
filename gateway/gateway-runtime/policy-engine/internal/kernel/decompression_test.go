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

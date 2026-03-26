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

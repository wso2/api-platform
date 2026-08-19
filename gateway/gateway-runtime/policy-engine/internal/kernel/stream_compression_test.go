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

package kernel

import (
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// singlePassGunzip decodes only the FIRST gzip member of the payload, modelling
// a client that stops there. Multistream(false) is what makes it stop — Go's
// gzip.Reader is multistream by default and net/http's transport does not disable
// it, so this is not a reproduction of Go transport behaviour. It stands in for
// clients that read a single member (and for the general case where concatenated
// members are not guaranteed to be read), which is why "one member spans the
// whole response" is the invariant asserted here.
func singlePassGunzip(t *testing.T, wire []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(wire))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer zr.Close()
	// Multistream(false) makes the reader stop at the first member, mirroring
	// clients that do not continue into subsequent members.
	zr.Multistream(false)
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	return out
}

// A streamed gzip response must be readable in full by a client that reads only
// the first gzip member. Regression test for the truncation reported against
// pii-masking-regex, which was really per-chunk re-compression in the kernel.
func TestStreamCompressor_GzipIsOneMemberAcrossChunks(t *testing.T) {
	chunks := []string{
		`{"id":"chatcmpl-1",`,
		`"choices":[{"message":`,
		`{"content":"hello there"}}],`,
		`"usage":{"total_tokens":30}}`,
	}
	want := strings.Join(chunks, "")

	sc := newStreamCompressor("gzip")
	if sc == nil {
		t.Fatal("expected a gzip stream compressor")
	}

	var wire bytes.Buffer
	for i, c := range chunks {
		out, err := sc.Compress([]byte(c), i == len(chunks)-1)
		if err != nil {
			t.Fatalf("chunk %d: %v", i, err)
		}
		wire.Write(out)
	}

	if got := string(singlePassGunzip(t, wire.Bytes())); got != want {
		t.Errorf("single-member decode mismatch\n got: %q\nwant: %q", got, want)
	}

	// There must be exactly one gzip header in the whole response.
	if n := bytes.Count(wire.Bytes(), []byte{0x1f, 0x8b}); n != 1 {
		t.Errorf("expected exactly 1 gzip member, found %d gzip headers", n)
	}
}

// Brotli has no multi-member concatenation, so a per-chunk writer made the tail
// of the response permanently undecodable. One stream must decode fully.
func TestStreamCompressor_BrotliIsOneStreamAcrossChunks(t *testing.T) {
	chunks := []string{"alpha ", "beta ", "gamma ", "delta"}
	want := strings.Join(chunks, "")

	sc := newStreamCompressor("br")
	if sc == nil {
		t.Fatal("expected a brotli stream compressor")
	}

	var wire bytes.Buffer
	for i, c := range chunks {
		out, err := sc.Compress([]byte(c), i == len(chunks)-1)
		if err != nil {
			t.Fatalf("chunk %d: %v", i, err)
		}
		wire.Write(out)
	}

	got, err := io.ReadAll(brotli.NewReader(bytes.NewReader(wire.Bytes())))
	if err != nil {
		t.Fatalf("brotli decode: %v", err)
	}
	if string(got) != want {
		t.Errorf("brotli decode mismatch\n got: %q\nwant: %q", got, want)
	}
}

// Chunks that produce no output (a policy suppressing a chunk) must not break
// the stream, and must not emit a standalone empty member.
func TestStreamCompressor_EmptyChunksDoNotBreakStream(t *testing.T) {
	sc := newStreamCompressor("gzip")
	var wire bytes.Buffer

	for _, c := range []string{"", "", "payload", ""} {
		out, err := sc.Compress([]byte(c), false)
		if err != nil {
			t.Fatalf("compress: %v", err)
		}
		wire.Write(out)
	}
	final, err := sc.Compress(nil, true)
	if err != nil {
		t.Fatalf("final compress: %v", err)
	}
	wire.Write(final)

	if got := string(singlePassGunzip(t, wire.Bytes())); got != "payload" {
		t.Errorf("got %q, want %q", got, "payload")
	}
	if n := bytes.Count(wire.Bytes(), []byte{0x1f, 0x8b}); n != 1 {
		t.Errorf("expected exactly 1 gzip member, found %d", n)
	}
}

// Data must reach the client incrementally: a non-final chunk has to produce
// output rather than sitting in the compressor until end of stream.
func TestStreamCompressor_FlushesPerChunk(t *testing.T) {
	sc := newStreamCompressor("gzip")
	out, err := sc.Compress([]byte(strings.Repeat("streaming payload ", 16)), false)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("non-final chunk produced no output; response would not stream incrementally")
	}
}

// Encodings the kernel cannot round-trip must yield no compressor, so callers
// forward the body untouched instead of corrupting it.
func TestStreamCompressor_UnsupportedEncodings(t *testing.T) {
	// "GZIP" belongs here: the codec switches are lowercase-only, so callers must
	// normalise before reaching them (buildRequest/ResponseContexts do). A
	// comma-separated chain is unsupported too — the kernel cannot round-trip one.
	for _, enc := range []string{"compress", "snappy", "identity", "", "GZIP", "gzip, br"} {
		if sc := newStreamCompressor(enc); sc != nil {
			t.Errorf("newStreamCompressor(%q) returned a compressor; want nil", enc)
		}
		if isRecompressibleEncoding(enc) {
			t.Errorf("isRecompressibleEncoding(%q) = true; want false", enc)
		}
	}
	for _, enc := range []string{"gzip", "br", "zstd", "deflate", "deflate-raw"} {
		if !isRecompressibleEncoding(enc) {
			t.Errorf("isRecompressibleEncoding(%q) = false; want true", enc)
		}
		if sc := newStreamCompressor(enc); sc == nil {
			t.Errorf("newStreamCompressor(%q) = nil; want a compressor", enc)
		}
	}
}

// newStreamCompressor returning nil must never reach a Compress/Close call.
// The streaming translators guard it explicitly rather than trusting the
// isRecompressibleEncoding gate in a different file, because a nil dereference
// there panics the ext_proc handler mid-message.
func TestTranslateStreamingChunkAction_NilCompressorFailsStreamNotPanics(t *testing.T) {
	// "identity" is deliberately chosen: it reaches the compressor branch (it is
	// a non-empty encoding) but yields no compressor, which is exactly the shape
	// a supported-encoding constructor failure takes.
	t.Run("response", func(t *testing.T) {
		execCtx := &PolicyExecutionContext{
			responseContentEncoding: "identity",
			analyticsMetadata:       map[string]any{},
			dynamicMetadata:         map[string]map[string]interface{}{},
		}
		_, err := TranslateStreamingResponseChunkAction(
			&executor.StreamingResponseExecutionResult{},
			&policy.StreamBody{Chunk: []byte("payload"), EndOfStream: true},
			execCtx,
		)
		if err == nil {
			t.Fatal("expected a stream error when no compressor is available; got nil")
		}
	})

	t.Run("request", func(t *testing.T) {
		execCtx := &PolicyExecutionContext{
			requestContentEncoding: "identity",
			analyticsMetadata:      map[string]any{},
			dynamicMetadata:        map[string]map[string]interface{}{},
		}
		_, err := TranslateStreamingRequestChunkAction(
			&executor.StreamingRequestExecutionResult{},
			&policy.StreamBody{Chunk: []byte("payload"), EndOfStream: true},
			execCtx,
		)
		if err == nil {
			t.Fatal("expected a stream error when no compressor is available; got nil")
		}
	})
}

// recompressBody must not answer a supported-but-unconstructable encoding with
// the plaintext body — that would put unencoded bytes under a compressed
// Content-Encoding header. A genuinely unknown encoding still passes through.
func TestRecompressBody_NilCompressorDistinguishesUnknownFromUnavailable(t *testing.T) {
	body := []byte(`{"content":"passthrough probe"}`)

	out, err := recompressBody(body, "identity")
	if err != nil {
		t.Fatalf("unknown encoding must pass through, got error: %v", err)
	}
	if !bytes.Equal(out, body) {
		t.Errorf("unknown encoding altered the body: got %q want %q", out, body)
	}
}

// Every supported encoding must survive a multi-chunk streaming round trip as
// ONE compressed stream. This is the regression that started this change: a
// per-chunk compressor emits N independent members and clients stop decoding
// after the first, so the body looks truncated while every byte was sent.
func TestStreamCompressor_RoundTripsEveryEncoding(t *testing.T) {
	chunks := []string{`{"choices":[{"delta":`, `{"content":"hello world"}`, `}]}`}
	var want strings.Builder
	for _, c := range chunks {
		want.WriteString(c)
	}

	for _, enc := range []string{"gzip", "br", "zstd", "deflate", "deflate-raw"} {
		t.Run(enc, func(t *testing.T) {
			sc := newStreamCompressor(enc)
			if sc == nil {
				t.Fatalf("newStreamCompressor(%q) = nil", enc)
			}
			var wire bytes.Buffer
			for i, c := range chunks {
				out, err := sc.Compress([]byte(c), i == len(chunks)-1)
				if err != nil {
					t.Fatalf("compress chunk %d: %v", i, err)
				}
				wire.Write(out)
			}

			got, err := decompressBody(wire.Bytes(), enc, 0)
			if err != nil {
				t.Fatalf("decompress: %v", err)
			}
			if string(got) != want.String() {
				t.Errorf("round trip mismatch\n got: %q\nwant: %q", got, want.String())
			}
		})
	}
}

// "deflate" on the wire is two different formats. Re-encoding raw input as
// zlib-wrapped (or the reverse) hands the peer a body its decoder rejects, so
// the arriving variant has to be detected and preserved.
func TestResolveDeflateVariant(t *testing.T) {
	payload := []byte(`{"content":"deflate variant probe"}`)

	zlibWrapped, err := recompressBody(payload, "deflate")
	if err != nil {
		t.Fatalf("zlib encode: %v", err)
	}
	raw, err := recompressBody(payload, "deflate-raw")
	if err != nil {
		t.Fatalf("raw deflate encode: %v", err)
	}

	if got := resolveDeflateVariant(zlibWrapped); got != "deflate" {
		t.Errorf("zlib-wrapped body detected as %q; want \"deflate\"", got)
	}
	if got := resolveDeflateVariant(raw); got != "deflate-raw" {
		t.Errorf("raw deflate body detected as %q; want \"deflate-raw\"", got)
	}
	// Too few bytes to decide falls back to the RFC-conformant form.
	if got := resolveDeflateVariant([]byte{0x78}); got != "deflate" {
		t.Errorf("1-byte body detected as %q; want \"deflate\"", got)
	}

	// The variants must not be interchangeable, or preserving them would be
	// pointless — decoding raw bytes as zlib has to fail.
	if _, err := decompressBody(raw, "deflate", 0); err == nil {
		t.Error("raw deflate decoded as zlib; the two variants are not distinguishable by this test")
	}
}

// A policy terminating the stream early (guardrail intervention) is an end of
// stream for the compressor too. Finalising only on the Envoy chunk's own
// EndOfStream flag would emit a gzip stream with no footer, which decodes as a
// truncated body — the same client-visible symptom this change exists to fix.
func TestTranslateStreamingResponseChunkAction_TerminatedStreamIsFinalised(t *testing.T) {
	execCtx := &PolicyExecutionContext{
		responseContentEncoding: "gzip",
		analyticsMetadata:       map[string]any{},
		dynamicMetadata:         map[string]map[string]interface{}{},
	}

	var wire bytes.Buffer
	chunks := []struct {
		body       string
		terminated bool
	}{
		{body: `{"choices":[{"delta":`, terminated: false},
		{body: `{"content":"blocked"}}]}`, terminated: true}, // policy stops the stream here
	}

	for i, c := range chunks {
		resp, err := TranslateStreamingResponseChunkAction(
			&executor.StreamingResponseExecutionResult{StreamTerminated: c.terminated},
			&policy.StreamBody{Chunk: []byte(c.body), EndOfStream: false}, // Envoy never signals EOS
			execCtx,
		)
		if err != nil {
			t.Fatalf("chunk %d: %v", i, err)
		}
		wire.Write(resp.GetResponseBody().GetResponse().GetBodyMutation().GetStreamedResponse().GetBody())
	}

	want := chunks[0].body + chunks[1].body
	if got := string(singlePassGunzip(t, wire.Bytes())); got != want {
		t.Errorf("terminated stream did not decode fully\n got: %q\nwant: %q", got, want)
	}
}

// Using a finalised compressor is a programming error, not silent corruption.
func TestStreamCompressor_RejectsUseAfterClose(t *testing.T) {
	sc := newStreamCompressor("gzip")
	if _, err := sc.Compress([]byte("done"), true); err != nil {
		t.Fatalf("final compress: %v", err)
	}
	if _, err := sc.Compress([]byte("more"), false); err == nil {
		t.Fatal("expected an error when compressing after end of stream")
	}
}

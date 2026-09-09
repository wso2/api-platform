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
	"context"
	"strings"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// recordingStreamPolicy is a minimal StreamingResponsePolicy that records what the
// kernel actually hands it. It buffers until it has seen the sentinel, mirroring
// how word-count/sentence-count guardrails and content-rewriting policies use
// NeedsMoreResponseData to assemble content spanning several chunks.
type recordingStreamPolicy struct {
	holdUntil string // keep buffering until the accumulated bytes contain this

	needsMoreCalls int      // how many times the kernel consulted the hook
	chunksSeen     []string // the body of every chunk delivered to the policy
}

func (p *recordingStreamPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeStream}
}

// OnResponseBody satisfies the buffered fallback embedded in StreamingResponsePolicy;
// these tests only exercise the streaming path.
func (p *recordingStreamPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{}
}

func (p *recordingStreamPolicy) NeedsMoreResponseData(accumulated []byte) bool {
	p.needsMoreCalls++
	if p.holdUntil == "" {
		return false // never asks the kernel to buffer
	}
	return !strings.Contains(string(accumulated), p.holdUntil)
}

func (p *recordingStreamPolicy) OnResponseBodyChunk(_ context.Context, _ *policy.ResponseStreamContext, chunk *policy.StreamBody, _ map[string]interface{}) policy.StreamingResponseAction {
	p.chunksSeen = append(p.chunksSeen, string(chunk.Chunk))
	return policy.ForwardResponseChunk{}
}

func newStreamingExecCtx(t *testing.T, pol policy.Policy, contentEncoding string) *PolicyExecutionContext {
	t.Helper()

	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
	chain := &registry.PolicyChain{
		RequiresResponseBody:      true,
		SupportsResponseStreaming: true,
		Policies:                  []policy.Policy{pol},
		PolicySpecs:               []policy.PolicySpec{{Enabled: true}},
	}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})

	respHeaders := []*corev3.HeaderValue{
		{Key: ":status", RawValue: []byte("200")},
		{Key: "content-type", RawValue: []byte("text/event-stream")},
	}
	if contentEncoding != "" {
		respHeaders = append(respHeaders, &corev3.HeaderValue{
			Key: "content-encoding", RawValue: []byte(contentEncoding),
		})
	}
	execCtx.buildResponseContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{Headers: respHeaders},
	})
	return execCtx
}

// THE contract test. A streaming policy must observe identical behaviour whether
// or not the upstream compressed the response. Before the streaming paths were
// unified, the compressed branch fed chunks straight to policies and never called
// NeedsMoreResponseData at all — so a guardrail that assembles content across
// chunks (word-count, sentence-count) silently evaluated isolated fragments as
// soon as the backend enabled gzip, with no error anywhere.
func TestStreamingResponse_PolicyContractIsIdenticalAcrossEncodings(t *testing.T) {
	// Split so the sentinel "END" only exists once the last chunk has arrived —
	// no single chunk contains it.
	bodyChunks := []string{`data: {"delta":"he`, `llo"}`, "\n\ndata: E", "ND\n\n"}
	wholeBody := ""
	for _, c := range bodyChunks {
		wholeBody += c
	}

	for _, encoding := range []string{"", "gzip", "br"} {
		name := encoding
		if name == "" {
			name = "plaintext"
		}
		t.Run(name, func(t *testing.T) {
			pol := &recordingStreamPolicy{holdUntil: "END"}
			execCtx := newStreamingExecCtx(t, pol, encoding)

			// Frame the wire the way a real streaming upstream does: one compressed
			// stream for the whole response, flushed after each logical chunk so the
			// data actually reaches the client incrementally.
			wireChunks := encodeStreamChunks(t, bodyChunks, encoding)

			for i, wc := range wireChunks {
				_, err := execCtx.processStreamingResponseBody(context.Background(), &extprocv3.HttpBody{
					Body:        wc,
					EndOfStream: i == len(wireChunks)-1,
				})
				require.NoError(t, err)
			}

			// The hook must be consulted on every encoding, not just plaintext.
			assert.Greater(t, pol.needsMoreCalls, 0,
				"NeedsMoreResponseData was never called — policy cannot buffer across chunks on this encoding")

			// And the policy must ultimately receive the complete body, assembled.
			joined := ""
			for _, c := range pol.chunksSeen {
				joined += c
			}
			assert.Equal(t, wholeBody, joined,
				"policy did not receive the full decompressed body")
			assert.Contains(t, pol.chunksSeen[len(pol.chunksSeen)-1], "END",
				"the buffered content was not released to the policy in one piece")
		})
	}
}

// encodeStreamChunks frames logical chunks the way a streaming upstream does:
// a single compressed stream flushed after every chunk, yielding one wire chunk
// per logical chunk. For plaintext the chunks pass through unchanged.
func encodeStreamChunks(t *testing.T, chunks []string, encoding string) [][]byte {
	t.Helper()
	if encoding == "" {
		out := make([][]byte, len(chunks))
		for i, c := range chunks {
			out[i] = []byte(c)
		}
		return out
	}
	sc := newStreamCompressor(encoding)
	require.NotNil(t, sc, "no stream compressor for encoding %q", encoding)
	out := make([][]byte, 0, len(chunks))
	for i, c := range chunks {
		b, err := sc.Compress([]byte(c), i == len(chunks)-1)
		require.NoError(t, err)
		out = append(out, b)
	}
	return out
}

// A policy that never wants buffering must still stream incrementally on a
// compressed response — the unified path must not turn every gzip response into
// a single end-of-stream flush.
func TestStreamingResponse_NoBufferingPolicyStillStreamsIncrementally(t *testing.T) {
	// An empty sentinel makes NeedsMoreResponseData always false — the policy never
	// asks the kernel to buffer.
	pol := &recordingStreamPolicy{holdUntil: ""}
	execCtx := newStreamingExecCtx(t, pol, "gzip")

	events := []string{"data: one\n\n", "data: two\n\n", "data: three\n\n"}
	body := ""
	for _, e := range events {
		body += e
	}
	wireChunks := encodeStreamChunks(t, events, "gzip")

	for i, wc := range wireChunks {
		_, err := execCtx.processStreamingResponseBody(context.Background(), &extprocv3.HttpBody{
			Body:        wc,
			EndOfStream: i == len(wireChunks)-1,
		})
		require.NoError(t, err)
	}

	assert.Greater(t, len(pol.chunksSeen), 1,
		"a non-buffering policy received a single end-of-stream flush; the response did not stream")

	joined := ""
	for _, c := range pol.chunksSeen {
		joined += c
	}
	assert.Equal(t, body, joined)
}

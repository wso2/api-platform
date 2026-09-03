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
	"context"
	"io"
	"testing"

	"github.com/andybalholm/brotli"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// The kernel is deliberately body-format agnostic: it frames, decompresses,
// hands bytes to policies, and re-compresses. These fixtures are the real wire
// shapes of the two providers most used through the gateway, kept here so a
// regression in framing shows up as a provider-shaped failure rather than an
// abstract byte mismatch. Anthropic is the important second case because it uses
// `event:` lines alongside `data:` and a different delta shape
// (content_block_delta/delta.text vs choices[].delta.content).

// openAIStreamEvents is an OpenAI /v1/chat/completions stream (stream: true).
var openAIStreamEvents = []string{
	`data: {"id":"chatcmpl-BxYz1","object":"chat.completion.chunk","created":1755172331,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}` + "\n\n",
	`data: {"id":"chatcmpl-BxYz1","object":"chat.completion.chunk","created":1755172331,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"Confirmation sent to "},"finish_reason":null}]}` + "\n\n",
	`data: {"id":"chatcmpl-BxYz1","object":"chat.completion.chunk","created":1755172331,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"john.doe@example.com"},"finish_reason":null}]}` + "\n\n",
	`data: {"id":"chatcmpl-BxYz1","object":"chat.completion.chunk","created":1755172331,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n",
	"data: [DONE]\n\n",
}

// anthropicStreamEvents is an Anthropic /v1/messages stream (stream: true).
// Note the `event:` lines and the heartbeat comment, both of which must survive
// framing untouched.
var anthropicStreamEvents = []string{
	"event: message_start\n" + `data: {"type":"message_start","message":{"id":"msg_01Xy","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[],"usage":{"input_tokens":24,"output_tokens":1}}}` + "\n\n",
	"event: content_block_start\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n",
	": heartbeat\n\n",
	"event: content_block_delta\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Confirmation sent to "}}` + "\n\n",
	"event: content_block_delta\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"john.doe@example.com"}}` + "\n\n",
	"event: content_block_stop\n" + `data: {"type":"content_block_stop","index":0}` + "\n\n",
	"event: message_delta\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":18}}` + "\n\n",
	"event: message_stop\n" + `data: {"type":"message_stop"}` + "\n\n",
}

// openAIBufferedBody is a non-streaming OpenAI response delivered chunked.
var openAIBufferedBody = `{"id":"chatcmpl-BxYz2","object":"chat.completion","created":1755172400,"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"Confirmation sent to john.doe@example.com. Call +1-555-0100 if anything changes."},"finish_reason":"stop"}],"usage":{"prompt_tokens":31,"completion_tokens":24,"total_tokens":55}}`

// anthropicBufferedBody is a non-streaming Anthropic response delivered chunked.
var anthropicBufferedBody = `{"id":"msg_01Xz","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"Confirmation sent to john.doe@example.com. Call +1-555-0100 if anything changes."}],"stop_reason":"end_turn","usage":{"input_tokens":31,"output_tokens":24}}`

func decodeWire(t *testing.T, wire []byte, encoding string) []byte {
	t.Helper()
	switch encoding {
	case "":
		return wire
	case "gzip":
		// Single-pass, first-member-only — exactly what httpx/urllib3/curl/Go do.
		zr, err := gzip.NewReader(bytes.NewReader(wire))
		require.NoError(t, err, "client could not start decoding the gzip response")
		defer zr.Close()
		zr.Multistream(false)
		out, err := io.ReadAll(zr)
		require.NoError(t, err, "client could not decode the gzip response to completion")
		return out
	case "br":
		out, err := io.ReadAll(brotli.NewReader(bytes.NewReader(wire)))
		require.NoError(t, err, "client could not decode the brotli response")
		return out
	}
	t.Fatalf("unknown encoding %q", encoding)
	return nil
}

// End-to-end: feed a provider-shaped response through the full kernel streaming
// path and assert the bytes a real client would reconstruct are byte-identical
// to what the upstream sent. This is the invariant the reported incident broke —
// the client received a 200 with a body it could not parse.
func TestStreamingResponse_ProviderFormatsRoundTripByteExact(t *testing.T) {
	for _, provider := range []struct {
		name        string
		events      []string
		contentType string
	}{
		{name: "openai-sse", events: openAIStreamEvents, contentType: "text/event-stream"},
		{name: "anthropic-sse", events: anthropicStreamEvents, contentType: "text/event-stream"},
		{name: "openai-buffered-chunked", events: splitString(openAIBufferedBody, 37), contentType: "application/json"},
		{name: "anthropic-buffered-chunked", events: splitString(anthropicBufferedBody, 37), contentType: "application/json"},
	} {
		for _, encoding := range []string{"", "gzip", "br"} {
			encName := encoding
			if encName == "" {
				encName = "plaintext"
			}
			t.Run(provider.name+"/"+encName, func(t *testing.T) {
				want := ""
				for _, e := range provider.events {
					want += e
				}

				// A pass-through policy: exercises the full pipeline (decompress →
				// policy → re-compress) without mutating bytes, so any difference is
				// the kernel's doing.
				pol := &recordingStreamPolicy{holdUntil: ""}
				execCtx := newStreamingExecCtx(t, pol, encoding)
				execCtx.responseStreamContext.ResponseHeaders = policy.NewHeaders(
					map[string][]string{"content-type": {provider.contentType}},
				)

				wireIn := encodeStreamChunks(t, provider.events, encoding)

				var wireOut bytes.Buffer
				for i, wc := range wireIn {
					resp, err := execCtx.processStreamingResponseBody(context.Background(), &extprocv3.HttpBody{
						Body:        wc,
						EndOfStream: i == len(wireIn)-1,
					})
					require.NoError(t, err)
					wireOut.Write(resp.GetResponseBody().GetResponse().GetBodyMutation().GetStreamedResponse().GetBody())
				}

				got := decodeWire(t, wireOut.Bytes(), encoding)
				assert.Equal(t, want, string(got),
					"the body a client reconstructs differs from what the upstream sent")

				if encoding == "gzip" {
					assert.Equal(t, 1, bytes.Count(wireOut.Bytes(), []byte{0x1f, 0x8b}),
						"response must be exactly one gzip member; more than one truncates the body for real clients")
				}

				// The policy must have seen the same bytes, fully decompressed.
				joined := ""
				for _, c := range pol.chunksSeen {
					joined += c
				}
				assert.Equal(t, want, joined, "policy did not observe the full decompressed body")
			})
		}
	}
}

// A guardrail-style policy that assembles content across events must work
// identically for both providers on a compressed stream. Before the streaming
// paths were unified this held only for plaintext.
func TestStreamingResponse_CrossEventAssemblyWorksForBothProviders(t *testing.T) {
	for _, provider := range []struct {
		name   string
		events []string
	}{
		{name: "openai", events: openAIStreamEvents},
		{name: "anthropic", events: anthropicStreamEvents},
	} {
		for _, encoding := range []string{"", "gzip", "br"} {
			encName := encoding
			if encName == "" {
				encName = "plaintext"
			}
			t.Run(provider.name+"/"+encName, func(t *testing.T) {
				// Hold until the address is fully assembled — it only exists once the
				// event carrying it has arrived, which is never the first event.
				pol := &recordingStreamPolicy{holdUntil: "john.doe@example.com"}
				execCtx := newStreamingExecCtx(t, pol, encoding)

				wireIn := encodeStreamChunks(t, provider.events, encoding)
				for i, wc := range wireIn {
					_, err := execCtx.processStreamingResponseBody(context.Background(), &extprocv3.HttpBody{
						Body:        wc,
						EndOfStream: i == len(wireIn)-1,
					})
					require.NoError(t, err)
				}

				assert.Greater(t, pol.needsMoreCalls, 0,
					"NeedsMoreResponseData never consulted — a guardrail could not assemble content on this encoding")

				var holding string
				for _, c := range pol.chunksSeen {
					if bytes.Contains([]byte(c), []byte("john.doe@example.com")) {
						holding = c
						break
					}
				}
				require.NotEmpty(t, holding,
					"the assembled address was never delivered to the policy in one piece")
			})
		}
	}
}

func splitString(s string, n int) []string {
	var out []string
	for len(s) > n {
		out = append(out, s[:n])
		s = s[n:]
	}
	if len(s) > 0 {
		out = append(out, s)
	}
	return out
}

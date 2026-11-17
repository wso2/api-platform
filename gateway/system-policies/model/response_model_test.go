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

package model

import (
	"bytes"
	"context"
	"strings"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// routeFor is the route job of a Model named "chat" on an endpoint of format.
func routeFor(t *testing.T, format string) *routePolicy {
	t.Helper()
	params := fourTargets()
	params[ParamRoute].(map[string]any)[ParamFormat] = format
	return mustRoute(t, params)
}

// buffered runs a whole response body through the route job and returns what the
// client receives, and the shared context the job left.
func buffered(t *testing.T, route *routePolicy, body string) (string, *policy.SharedContext) {
	t.Helper()
	shared := &policy.SharedContext{Metadata: map[string]any{}}
	action := route.OnResponseBody(context.Background(), &policy.ResponseContext{
		SharedContext: shared,
		ResponseBody:  &policy.Body{Content: []byte(body), Present: true, EndOfStream: true},
	}, nil)
	mods, ok := action.(policy.DownstreamResponseModifications)
	if !ok {
		t.Fatalf("expected DownstreamResponseModifications, got %T", action)
	}
	if mods.Body == nil {
		return body, shared
	}
	return string(mods.Body), shared
}

// streamed runs a response delivered as pieces through the route job, the last
// piece ending the stream, and returns everything the client receives.
func streamed(t *testing.T, route *routePolicy, pieces []string) (string, *policy.SharedContext) {
	t.Helper()
	shared := &policy.SharedContext{Metadata: map[string]any{}}
	var out []byte
	for i, piece := range pieces {
		action := route.OnResponseBodyChunk(context.Background(), &policy.ResponseStreamContext{SharedContext: shared},
			&policy.StreamBody{Chunk: []byte(piece), EndOfStream: i == len(pieces)-1, Index: uint64(i)}, nil)
		forward, ok := action.(policy.ForwardResponseChunk)
		if !ok {
			t.Fatalf("piece %d: expected ForwardResponseChunk, got %T", i, action)
		}
		if forward.Body == nil {
			t.Fatalf("piece %d: a nil body forwards the piece as it arrived, and duplicates what is held", i)
		}
		out = append(out, forward.Body...)
	}
	return string(out), shared
}

// cuts is every way to deliver body: whole, in two at every byte, one byte at a
// time, and whole followed by the empty end-of-stream piece the kernel often sends.
func cuts(body string) map[string][]string {
	all := map[string][]string{
		"whole":                {body},
		"then an empty end":    {body, ""},
		"one byte at a time":   strings.Split(body, ""),
		"an empty first piece": {"", body},
	}
	for i := 1; i < len(body); i++ {
		all["cut at "+itoa(i)] = []string{body[:i], body[i:]}
	}
	return all
}

func itoa(i int) string {
	var b [20]byte
	n := len(b)
	for {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
		if i == 0 {
			return string(b[n:])
		}
	}
}

// The responses each format's providers send, after the provider's transformer,
// and what the client must receive instead.
var responseCases = []struct {
	name, format, body, want, reported string
}{
	{
		name:   "OpenAI completion",
		format: FormatOpenAI,
		body: `{"id":"c1","object":"chat.completion","model":"gpt-4o-mini-2024-07-18",` +
			`"choices":[{"message":{"role":"assistant","content":"I am \"model\": gpt"}}]}`,
		want: `{"id":"c1","object":"chat.completion","model":"chat",` +
			`"choices":[{"message":{"role":"assistant","content":"I am \"model\": gpt"}}]}`,
		reported: "gpt-4o-mini-2024-07-18",
	},
	{
		name:     "OpenAI completion, pretty-printed and chunked",
		format:   FormatOpenAI,
		body:     "{\n  \"id\": \"c1\",\n  \"model\": \"gpt-4o-mini\",\n  \"choices\": [{\"message\": {\"content\": \"{}\"}}]\n}\n",
		want:     "{\n  \"id\": \"c1\",\n  \"model\": \"chat\",\n  \"choices\": [{\"message\": {\"content\": \"{}\"}}]\n}\n",
		reported: "gpt-4o-mini",
	},
	{
		name:   "OpenAI event stream",
		format: FormatOpenAI,
		body: "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o-mini\",\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
			"data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o-mini\",\"choices\":[{\"delta\":{\"content\":\"}{\"}}]}\n\n" +
			"data: [DONE]\n\n",
		want: "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"model\":\"chat\",\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
			"data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"model\":\"chat\",\"choices\":[{\"delta\":{\"content\":\"}{\"}}]}\n\n" +
			"data: [DONE]\n\n",
		reported: "gpt-4o-mini",
	},
	{
		name:     "Anthropic message",
		format:   FormatAnthropic,
		body:     `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5-5","content":[{"type":"text","text":"hi"}]}`,
		want:     `{"id":"msg_1","type":"message","role":"assistant","model":"chat","content":[{"type":"text","text":"hi"}]}`,
		reported: "claude-sonnet-5-5",
	},
	{
		name:   "Anthropic event stream, CRLF",
		format: FormatAnthropic,
		body: "event: message_start\r\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-sonnet-5-5\",\"content\":[]}}\r\n\r\n" +
			"event: content_block_delta\r\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}\r\n\r\n" +
			"event: message_stop\r\ndata: {\"type\":\"message_stop\"}\r\n\r\n",
		want: "event: message_start\r\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"chat\",\"content\":[]}}\r\n\r\n" +
			"event: content_block_delta\r\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}\r\n\r\n" +
			"event: message_stop\r\ndata: {\"type\":\"message_stop\"}\r\n\r\n",
		reported: "claude-sonnet-5-5",
	},
	{
		name:     "Gemini content",
		format:   FormatGemini,
		body:     `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}],"modelVersion":"gemini-2.0-flash","responseId":"r1"}`,
		want:     `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}],"modelVersion":"chat","responseId":"r1"}`,
		reported: "gemini-2.0-flash",
	},
	{
		name:   "Gemini event stream",
		format: FormatGemini,
		body: "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"h\"}]}}],\"modelVersion\":\"gemini-2.0-flash\"}\r\n\r\n" +
			"data: {\"candidates\":[],\"modelVersion\":\"gemini-2.0-flash\"}\r\n\r\n",
		want: "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"h\"}]}}],\"modelVersion\":\"chat\"}\r\n\r\n" +
			"data: {\"candidates\":[],\"modelVersion\":\"chat\"}\r\n\r\n",
		reported: "gemini-2.0-flash",
	},
	{
		name:   "Gemini JSON array stream",
		format: FormatGemini,
		body: "[{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"[{\"}]}}],\"modelVersion\":\"gemini-2.0-flash\"}\n" +
			",\r\n{\"candidates\":[],\"modelVersion\":\"gemini-2.0-flash\",\"usageMetadata\":{\"totalTokenCount\":3}}\n]",
		want: "[{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"[{\"}]}}],\"modelVersion\":\"chat\"}\n" +
			",\r\n{\"candidates\":[],\"modelVersion\":\"chat\",\"usageMetadata\":{\"totalTokenCount\":3}}\n]",
		reported: "gemini-2.0-flash",
	},
}

func TestABufferedResponseAnswersInTheModelsName(t *testing.T) {
	for _, tc := range responseCases {
		t.Run(tc.name, func(t *testing.T) {
			got, shared := buffered(t, routeFor(t, tc.format), tc.body)
			if got != tc.want {
				t.Fatalf("response =\n%q\nwant\n%q", got, tc.want)
			}
			if reported := metadataString(shared, MetadataResponseModel); reported != tc.reported {
				t.Fatalf("%s = %q, want %q", MetadataResponseModel, reported, tc.reported)
			}
		})
	}
}

// However the kernel cuts a streamed response, the client receives the same bytes
// as for the whole response at once.
func TestAStreamedResponseAnswersInTheModelsNameWhereverItIsCut(t *testing.T) {
	for _, tc := range responseCases {
		route := routeFor(t, tc.format)
		for cut, pieces := range cuts(tc.body) {
			got, shared := streamed(t, route, pieces)
			if got != tc.want {
				t.Fatalf("%s, %s: response =\n%q\nwant\n%q", tc.name, cut, got, tc.want)
			}
			if reported := metadataString(shared, MetadataResponseModel); reported != tc.reported {
				t.Fatalf("%s, %s: %s = %q, want %q", tc.name, cut, MetadataResponseModel, reported, tc.reported)
			}
		}
	}
}

// A completed event, or a completed element of a Gemini array, is forwarded in the
// piece that completes it: the rewrite never delays a stream.
func TestAStreamIsNotHeldBack(t *testing.T) {
	route := routeFor(t, FormatOpenAI)
	shared := &policy.SharedContext{Metadata: map[string]any{}}
	events := []string{
		"data: {\"model\":\"gpt-4o-mini\",\"choices\":[]}\n\n",
		"data: {\"model\":\"gpt-4o-mini\",\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n",
		"data: [DONE]\n\n",
	}
	for i, event := range events {
		action := route.OnResponseBodyChunk(context.Background(), &policy.ResponseStreamContext{SharedContext: shared},
			&policy.StreamBody{Chunk: []byte(event), Index: uint64(i)}, nil).(policy.ForwardResponseChunk)
		want := strings.ReplaceAll(event, `"gpt-4o-mini"`, `"chat"`)
		if string(action.Body) != want {
			t.Fatalf("event %d forwarded as %q, want %q", i, action.Body, want)
		}
	}

	gemini := routeFor(t, FormatGemini)
	shared = &policy.SharedContext{Metadata: map[string]any{}}
	first := "[{\"candidates\":[],\"modelVersion\":\"gemini-2.0-flash\"}\n,"
	action := gemini.OnResponseBodyChunk(context.Background(), &policy.ResponseStreamContext{SharedContext: shared},
		&policy.StreamBody{Chunk: []byte(first)}, nil).(policy.ForwardResponseChunk)
	if want := "[{\"candidates\":[],\"modelVersion\":\"chat\"}\n,"; string(action.Body) != want {
		t.Fatalf("first element forwarded as %q, want %q", action.Body, want)
	}
}

// Only the member the format reports the model in is replaced: nothing is added,
// a value that is not a string is left alone, and a "model" anywhere else — in a
// choice, in the text, in another format's place for it — is the response's own.
func TestOnlyTheReportedModelIsReplaced(t *testing.T) {
	for name, tc := range map[string]struct{ format, body, want string }{
		"no model":                    {FormatOpenAI, `{"id":"c1","choices":[]}`, ""},
		"a model that is null":        {FormatOpenAI, `{"id":"c1","model":null}`, ""},
		"a model that is an object":   {FormatOpenAI, `{"id":"c1","model":{"name":"gpt"}}`, ""},
		"a model inside a choice":     {FormatOpenAI, `{"choices":[{"model":"gpt-4o"}]}`, ""},
		"a model in the text":         {FormatOpenAI, `{"choices":[{"text":"\"model\":\"gpt-4o\""}]}`, ""},
		"Anthropic's place on OpenAI": {FormatOpenAI, `{"message":{"model":"claude"}}`, ""},
		"Gemini's place on OpenAI":    {FormatOpenAI, `{"modelVersion":"gemini-2.0-flash"}`, ""},
		"OpenAI's place on Gemini":    {FormatGemini, `{"model":"gpt-4o"}`, ""},
		"a top-level array on OpenAI": {FormatOpenAI, `[{"choices":[{"model":"gpt-4o"}]}]`, ""},
		"a JSON string":               {FormatOpenAI, `"model"`, ""},
		"plain text":                  {FormatOpenAI, "model: gpt-4o\n", ""},
		"an unfinished object":        {FormatOpenAI, `{"model":"gpt-4o","choices":[`, ""},
		"an unbalanced brace":         {FormatOpenAI, `}{"model":"gpt-4o"}`, ""},
		"the end of a stream":         {FormatOpenAI, "data: [DONE]\n\n", ""},
		"an event that is not JSON":   {FormatOpenAI, "data: {\"model\":\"gpt-4o\"\n\n", ""},
		"a comment line":              {FormatOpenAI, ": keep-alive\n\n", ""},
		"a data line without a space": {FormatOpenAI, "data:[DONE]\n\n", ""},
		// The last line is the response's end, so it is read and rewritten.
		"a last line without its end":            {FormatOpenAI, "data: {\"model\":\"gpt-4o\"}", `data: {"model":"chat"}`},
		"a data line without a space, rewritten": {FormatOpenAI, "data:{\"model\":\"gpt-4o\"}\n", "data:{\"model\":\"chat\"}\n"},
	} {
		t.Run(name, func(t *testing.T) {
			want := tc.want
			if want == "" {
				want = tc.body
			}
			route := routeFor(t, tc.format)
			if got, _ := buffered(t, route, tc.body); got != want {
				t.Fatalf("buffered response = %q, want %q", got, want)
			}
			for cut, pieces := range cuts(tc.body) {
				if got, _ := streamed(t, route, pieces); got != want {
					t.Fatalf("%s: response = %q, want %q", cut, got, want)
				}
			}
		})
	}
}

// Every occurrence of the member is replaced, as on the request: a body repeating
// it cannot show the client the provider's model in the occurrence its parser
// reads.
func TestARepeatedModelIsReplacedEverywhere(t *testing.T) {
	got, shared := buffered(t, routeFor(t, FormatOpenAI), `{"model":"gpt-4o","id":"c1","model":"gpt-4o-mini"}`)
	if want := `{"model":"chat","id":"c1","model":"chat"}`; got != want {
		t.Fatalf("response = %q, want %q", got, want)
	}
	if reported := metadataString(shared, MetadataResponseModel); reported != "gpt-4o" {
		t.Fatalf("%s = %q, want the first, gpt-4o", MetadataResponseModel, reported)
	}
}

// The provider's model is recorded once, from the first event that names it, and a
// value already recorded is kept.
func TestTheProvidersModelIsRecordedOnce(t *testing.T) {
	route := routeFor(t, FormatOpenAI)
	_, shared := streamed(t, route, []string{
		"data: {\"model\":\"gpt-4o-mini\"}\n\n",
		"data: {\"model\":\"gpt-4o\"}\n\n",
	})
	if reported := metadataString(shared, MetadataResponseModel); reported != "gpt-4o-mini" {
		t.Fatalf("%s = %q, want gpt-4o-mini", MetadataResponseModel, reported)
	}

	shared = &policy.SharedContext{Metadata: map[string]any{MetadataResponseModel: "earlier"}}
	route.OnResponseBody(context.Background(), &policy.ResponseContext{
		SharedContext: shared,
		ResponseBody:  &policy.Body{Content: []byte(`{"model":"gpt-4o"}`), Present: true},
	}, nil)
	if reported := metadataString(shared, MetadataResponseModel); reported != "earlier" {
		t.Fatalf("%s = %q, want the value already recorded", MetadataResponseModel, reported)
	}
}

// An object still unfinished when more than the hold limit is held is forwarded as
// the provider sent it, with nothing lost or reordered, and the rest of the stream
// after it as it arrives.
func TestAResponsePastTheHoldLimitIsForwardedUnchanged(t *testing.T) {
	route := routeFor(t, FormatOpenAI)
	const piece = 1 << 20
	body := `{"model":"gpt-4o","pad":"` + strings.Repeat("a", maxHeldResponseBytes+piece) + `"}`
	var pieces []string
	for start := 0; start < len(body); start += piece {
		end := start + piece
		if end > len(body) {
			end = len(body)
		}
		pieces = append(pieces, body[start:end])
	}
	got, shared := streamed(t, route, pieces)
	if got != body {
		t.Fatalf("response of %d bytes forwarded as %d bytes, or changed", len(body), len(got))
	}
	if reported := metadataString(shared, MetadataResponseModel); reported != "" {
		t.Fatalf("%s = %q for a response that was not rewritten", MetadataResponseModel, reported)
	}
}

// Without a shared context there is nowhere to keep a model cut across two chunks,
// so each chunk is forwarded as it arrived.
func TestAStreamWithoutAContextIsForwardedUnchanged(t *testing.T) {
	route := routeFor(t, FormatOpenAI)
	for _, ctx := range []*policy.ResponseStreamContext{nil, {}} {
		action := route.OnResponseBodyChunk(context.Background(), ctx,
			&policy.StreamBody{Chunk: []byte(`{"model":"gpt-4o"}`), EndOfStream: true}, nil)
		if forward, ok := action.(policy.ForwardResponseChunk); !ok || forward.Body != nil {
			t.Fatalf("expected the chunk forwarded unchanged, got %#v", action)
		}
	}
	if action := route.OnResponseBodyChunk(context.Background(),
		&policy.ResponseStreamContext{SharedContext: &policy.SharedContext{}}, nil, nil); action == nil {
		t.Fatal("a nil chunk must still be answered")
	}
}

// A buffered response without a body, or whose rewrite changes nothing, is left
// alone rather than replaced with an identical copy.
func TestABufferedResponseThatNeedsNothingIsLeftAlone(t *testing.T) {
	route := routeFor(t, FormatOpenAI)
	for name, body := range map[string]*policy.Body{
		"no body":       nil,
		"absent":        {Present: false},
		"empty":         {Content: []byte{}, Present: true},
		"nothing to do": {Content: []byte(`{"id":"c1"}`), Present: true},
	} {
		action := route.OnResponseBody(context.Background(), &policy.ResponseContext{
			SharedContext: &policy.SharedContext{Metadata: map[string]any{}},
			ResponseBody:  body,
		}, nil)
		if mods := action.(policy.DownstreamResponseModifications); mods.Body != nil {
			t.Fatalf("%s: body replaced with %q", name, mods.Body)
		}
	}
	if mods := route.OnResponseBody(context.Background(), nil, nil).(policy.DownstreamResponseModifications); mods.Body != nil {
		t.Fatal("a nil context must not produce a body")
	}
}

func TestEveryFormatHasAPlaceForTheModel(t *testing.T) {
	for format := range knownFormats {
		if len(responseModelPaths[format]) == 0 {
			t.Errorf("format %s has no place a response reports the model in", format)
		}
	}
	if !bytes.Equal(newModelRewriter(FormatOpenAI, `a"b`).encoded, []byte(`"a\"b"`)) {
		t.Error("the Model's name must be written as a JSON string")
	}
}

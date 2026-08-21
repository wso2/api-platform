/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package llmusage

import (
	"bytes"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func newSharedContext() *policy.SharedContext {
	return &policy.SharedContext{Metadata: map[string]interface{}{}}
}

func TestAccumulate_DedupesByChunkIndex(t *testing.T) {
	sc := newSharedContext()

	// Two policies see the same chunk, so it must be appended once.
	first := &policy.StreamBody{Chunk: []byte("abc"), Index: 0}
	Accumulate(sc, first)
	Accumulate(sc, first)

	got := Accumulate(sc, &policy.StreamBody{Chunk: []byte("def"), Index: 1})

	if string(got) != "abcdef" {
		t.Errorf("accumulated = %q, want %q", got, "abcdef")
	}
}

func TestAccumulate_OutOfOrderIndexIgnored(t *testing.T) {
	sc := newSharedContext()

	Accumulate(sc, &policy.StreamBody{Chunk: []byte("abc"), Index: 0})
	Accumulate(sc, &policy.StreamBody{Chunk: []byte("def"), Index: 1})
	got := Accumulate(sc, &policy.StreamBody{Chunk: []byte("STALE"), Index: 0})

	if string(got) != "abcdef" {
		t.Errorf("accumulated = %q, want the stale chunk ignored", got)
	}
}

func TestAccumulate_NilContextDoesNotPanic(t *testing.T) {
	if got := Accumulate(nil, &policy.StreamBody{Chunk: []byte("abc")}); got != nil {
		t.Errorf("got %q, want nil for a nil context", got)
	}
}

func TestGet_MemoizesAcrossCalls(t *testing.T) {
	sc := newSharedContext()
	sc.Metadata[MetadataTemplateHandle] = "openai"
	storeTestTemplate(t, "openai", openAITemplate())

	body := []byte(`{"model":"gpt-4o-mini","usage":{"prompt_tokens":10,"completion_tokens":5}}`)

	first, err := Get(sc, body, nil, "/chat/completions")
	if err != nil {
		t.Fatalf("first Get returned error: %v", err)
	}

	// A second call with a different body must return the memoized value,
	// proving extraction happened once for the request.
	second, err := Get(sc, []byte(`{"usage":{"prompt_tokens":999}}`), nil, "/chat/completions")
	if err != nil {
		t.Fatalf("second Get returned error: %v", err)
	}

	if second.TotalInputTokens != first.TotalInputTokens {
		t.Errorf("second call re-extracted: got %d, want the memoized %d",
			second.TotalInputTokens, first.TotalInputTokens)
	}
	if first.TotalInputTokens != 10 {
		t.Errorf("TotalInputTokens = %d, want 10", first.TotalInputTokens)
	}
}

func TestGet_NoTemplateHandleReportsStatus(t *testing.T) {
	sc := newSharedContext()

	_, err := Get(sc, []byte(`{"usage":{"prompt_tokens":10}}`), nil, "/chat/completions")

	if err == nil {
		t.Fatal("expected an error when the route carries no template handle")
	}
	if sc.Metadata[StatusKey] != StatusTemplateMissing {
		t.Errorf("status = %v, want %q", sc.Metadata[StatusKey], StatusTemplateMissing)
	}
}

func TestPublish_WritesUsageAndStatus(t *testing.T) {
	sc := newSharedContext()

	Publish(sc, Usage{TotalInputTokens: 100, OutputTokens: 20, TotalTokens: 120, Model: "m"})

	if sc.Metadata[StatusKey] != StatusExtracted {
		t.Errorf("status = %v, want %q", sc.Metadata[StatusKey], StatusExtracted)
	}
	published, ok := sc.Metadata[UsageKey].(Usage)
	if !ok {
		t.Fatalf("usage under %q is %T, want Usage", UsageKey, sc.Metadata[UsageKey])
	}
	if published.TotalInputTokens != 100 {
		t.Errorf("published TotalInputTokens = %d, want 100", published.TotalInputTokens)
	}
}

func TestPublish_NilContextDoesNotPanic(t *testing.T) {
	Publish(nil, Usage{TotalInputTokens: 1})
}

// storeTestTemplate puts a template into the shared lazy-resource store and
// removes it when the test ends, so tests do not leak state into each other.
func storeTestTemplate(t *testing.T, handle string, spec map[string]interface{}) {
	t.Helper()

	store := policy.GetLazyResourceStoreInstance()
	if err := store.StoreResource(&policy.LazyResource{
		ID:           handle,
		ResourceType: ResourceTypeLLMProviderTemplate,
		Resource:     spec,
	}); err != nil {
		t.Fatalf("failed to store test template: %v", err)
	}
	t.Cleanup(func() {
		_ = store.RemoveResourceByIDAndType(handle, ResourceTypeLLMProviderTemplate)
	})
}

// The kernel hands each chunk to every policy on the route in turn, so
// Accumulate is called once per policy per chunk. Each caller must come away
// with the whole stream, which means the buffer cannot be cleared when the
// first caller reaches the final chunk.
func TestAccumulate_EveryPolicySeesTheWholeStream(t *testing.T) {
	sc := &policy.SharedContext{Metadata: map[string]interface{}{}}
	chunks := []struct {
		data string
		eos  bool
	}{
		{"data: {\"model\":\"m\"}\n\n", false},
		{"data: {\"choices\":[]}\n\n", false},
		{"data: {\"usage\":{\"prompt_tokens\":6}}\n\n", true},
	}

	var first, second []byte
	for i, c := range chunks {
		chunk := &policy.StreamBody{Chunk: []byte(c.data), Index: uint64(i + 1), EndOfStream: c.eos}
		first = Accumulate(sc, chunk)  // first policy in the chain
		second = Accumulate(sc, chunk) // second policy, same chunk
	}

	if string(second) != string(first) {
		t.Errorf("second policy saw %d bytes, first saw %d; both must see the whole stream",
			len(second), len(first))
	}
	for _, want := range []string{`"model":"m"`, `"prompt_tokens":6`} {
		if !bytes.Contains(second, []byte(want)) {
			t.Errorf("second policy's buffer is missing %s", want)
		}
	}
}

// A redelivered final chunk must not restart the buffer either.
func TestAccumulate_RedeliveredFinalChunkKeepsBuffer(t *testing.T) {
	sc := &policy.SharedContext{Metadata: map[string]interface{}{}}

	Accumulate(sc, &policy.StreamBody{Chunk: []byte("aaaa"), Index: 1})
	first := Accumulate(sc, &policy.StreamBody{Chunk: []byte("bbbb"), Index: 2, EndOfStream: true})
	again := Accumulate(sc, &policy.StreamBody{Chunk: []byte("bbbb"), Index: 2, EndOfStream: true})

	if string(again) != string(first) {
		t.Errorf("redelivered final chunk gave %q, want %q", again, first)
	}
}

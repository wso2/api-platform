package llmusage

import (
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

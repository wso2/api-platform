package analytics

import (
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func field(identifier string, fallbacks ...string) map[string]interface{} {
	cfg := map[string]interface{}{"location": "payload", "identifier": identifier}
	if len(fallbacks) > 0 {
		list := make([]interface{}, len(fallbacks))
		for i, f := range fallbacks {
			list[i] = f
		}
		cfg["fallbackIdentifiers"] = list
	}
	return cfg
}

// openAITemplate mirrors the openai template: root fields for chat completions, and a
// /responses mapping whose fields carry the streamed form as a fallback.
func openAITemplate() map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": "openai"},
		"spec": map[string]interface{}{
			"promptTokens":     field("$.usage.prompt_tokens"),
			"completionTokens": field("$.usage.completion_tokens"),
			"totalTokens":      field("$.usage.total_tokens"),
			"requestModel":     field("$.model"),
			"responseModel":    field("$.model"),
			"resourceMappings": map[string]interface{}{
				"resources": []interface{}{
					map[string]interface{}{
						"resource":         "/responses",
						"promptTokens":     field("$.usage.input_tokens", "$.response.usage.input_tokens"),
						"completionTokens": field("$.usage.output_tokens", "$.response.usage.output_tokens"),
					},
				},
			},
		},
	}
}

func wantTokens(t *testing.T, info *LLMProviderAnalyticsInfo, prompt, completion int64) {
	t.Helper()
	if info.PromptTokens == nil || *info.PromptTokens != prompt {
		t.Errorf("PromptTokens = %v, want %d", info.PromptTokens, prompt)
	}
	if info.CompletionTokens == nil || *info.CompletionTokens != completion {
		t.Errorf("CompletionTokens = %v, want %d", info.CompletionTokens, completion)
	}
}

func TestExtractLLMAnalyticsAppliesResourceMapping(t *testing.T) {
	responses := []byte(`{"model":"gpt-4o","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}`)
	info, err := extractLLMProviderAnalyticsInfoFromBytes(openAITemplate(), nil, nil, nil, responses,
		"/openai/v1/responses", "/responses")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTokens(t, info, 10, 5)
	if info.TotalTokens == nil || *info.TotalTokens != 15 {
		t.Errorf("TotalTokens = %v, want 15 from the root field the mapping does not override", info.TotalTokens)
	}

	// A streamed response nests the document under "response": the mapping's fallback finds it.
	stream := []byte("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-4o\",\"usage\":{\"input_tokens\":7,\"output_tokens\":3}}}\n\n")
	info, err = extractLLMProviderAnalyticsInfoFromBytes(openAITemplate(), nil, nil, nil, stream,
		"/openai/v1/responses", "/responses")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTokens(t, info, 7, 3)

	// Another resource keeps the root fields.
	chat := []byte(`{"model":"gpt-4o","usage":{"prompt_tokens":19,"completion_tokens":10,"total_tokens":29}}`)
	info, err = extractLLMProviderAnalyticsInfoFromBytes(openAITemplate(), nil, nil, nil, chat,
		"/openai/v1/chat/completions", "/chat/completions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTokens(t, info, 19, 10)
}

// The awsbedrock template reads Converse's camelCase usage first and falls back to the
// model-native snake_case usage that InvokeModel returns for Claude.
func TestExtractLLMAnalyticsBedrockInvokeUsesFallbacks(t *testing.T) {
	template := bedrockStreamTemplate()
	spec := template["spec"].(map[string]interface{})
	spec["promptTokens"] = field("$.usage.inputTokens", "$.usage.input_tokens", "$.usage.prompt_tokens")
	spec["completionTokens"] = field("$.usage.outputTokens", "$.usage.output_tokens", "$.usage.completion_tokens")

	const path = "/bedrock/model/us.anthropic.claude-sonnet-4-5-20250929-v1:0/invoke"
	buffered := []byte(`{"type":"message","usage":{"input_tokens":1000,"output_tokens":500}}`)
	info, err := extractLLMProviderAnalyticsInfoFromBytes(template, nil, nil, nil, buffered, path, "/model/x/invoke")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTokens(t, info, 1000, 500)

	var stream []byte
	stream = append(stream, encodeEventStreamFrame("chunk", []byte(`{"type":"message_start","usage":{"input_tokens":1000}}`))...)
	stream = append(stream, encodeEventStreamFrame("chunk", []byte(`{"type":"message_delta","usage":{"output_tokens":500}}`))...)
	headers := policy.NewHeaders(map[string][]string{"content-type": {"application/vnd.amazon.eventstream"}})
	info, err = extractLLMProviderAnalyticsInfoFromBytes(template, nil, headers, nil, stream,
		path+"-with-response-stream", "/model/x/invoke-with-response-stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTokens(t, info, 1000, 500)
}

func TestLLMResourcePath(t *testing.T) {
	sc := &policy.SharedContext{APIContext: "/openai/$version", APIVersion: "v1"}
	for _, tc := range []struct {
		sc   *policy.SharedContext
		path string
		want string
	}{
		{sc, "/openai/v1/responses", "/responses"},
		{sc, "/openai/v1/responses?stream=true", "/responses"},
		{&policy.SharedContext{APIContext: "/"}, "/responses", "/responses"},
		{nil, "/openai/v1/responses", "/openai/v1/responses"},
		{sc, "/openai/v1", "/"},
	} {
		if got := llmResourcePath(tc.sc, tc.path); got != tc.want {
			t.Errorf("llmResourcePath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestSelectLLMResourceMappingPrefersTheMostSpecific(t *testing.T) {
	entry := func(pattern string) map[string]interface{} { return map[string]interface{}{"resource": pattern} }
	spec := map[string]interface{}{"resourceMappings": map[string]interface{}{"resources": []interface{}{
		entry("/*"), entry("/v1/*"), entry("/v1/responses"), entry("/v1/*/x"), "not a map",
	}}}
	for path, want := range map[string]string{
		"/v1/responses": "/v1/responses",
		"/v1/chat":      "/v1/*",
		"/other":        "/*",
	} {
		got := selectLLMResourceMapping(spec, path)
		if got == nil || got["resource"] != want {
			t.Errorf("selectLLMResourceMapping(%q) = %v, want %q", path, got, want)
		}
	}
	if llmPathsMatch("/v1/a/x", "/v1/*/x") {
		t.Error("an embedded wildcard must not match")
	}
	if selectLLMResourceMapping(map[string]interface{}{}, "/v1/responses") != nil {
		t.Error("a spec without resourceMappings must select nothing")
	}
}

func TestLLMFieldIdentifiers(t *testing.T) {
	got := llmFieldIdentifiers(field("$.a", "$.b", "", "$.c"))
	if len(got) != 3 || got[0] != "$.a" || got[1] != "$.b" || got[2] != "$.c" {
		t.Errorf("identifiers = %v, want [$.a $.b $.c]", got)
	}
	if llmFieldIdentifiers(map[string]interface{}{"fallbackIdentifiers": []interface{}{"$.b"}}) != nil {
		t.Error("a field without a primary identifier has no identifiers")
	}
}

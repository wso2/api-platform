package analytics

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// TestExtractPathParam covers the pathParam extraction used to meter the model
// id for providers that carry it in the URL path (AWS Bedrock, Gemini) rather
// than the body or a header.
func TestExtractPathParam(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		pattern string
		want    string
		wantErr bool
	}{
		{
			name:    "bedrock converse model id",
			path:    "/bedrock/model/us.amazon.nova-lite-v1:0/converse",
			pattern: "model/([A-Za-z0-9.:-]+)/",
			want:    "us.amazon.nova-lite-v1:0",
		},
		{
			name:    "bedrock streaming model id",
			path:    "/bedrock/model/us.anthropic.claude-3-5-sonnet-20240620-v1:0/converse-stream",
			pattern: "model/([A-Za-z0-9.:-]+)/",
			want:    "us.anthropic.claude-3-5-sonnet-20240620-v1:0",
		},
		{
			name:    "no capture group returns whole match",
			path:    "/v1beta/models/gemini-1.5-flash:generateContent",
			pattern: "models/[A-Za-z0-9.-]+",
			want:    "models/gemini-1.5-flash",
		},
		{
			name:    "empty path errors",
			path:    "",
			pattern: "model/([A-Za-z0-9.:-]+)/",
			wantErr: true,
		},
		{
			name:    "no match errors",
			path:    "/bedrock/health",
			pattern: "model/([A-Za-z0-9.:-]+)/",
			wantErr: true,
		},
		{
			// Go's regexp is RE2 — lookbehind is unsupported. This is exactly the
			// class of pattern that must never reach the extractor, which is why
			// the awsbedrock template uses a capture group instead.
			name:    "lookbehind pattern rejected by RE2",
			path:    "/bedrock/model/x/converse",
			pattern: "(?<=model/)[A-Za-z0-9.:-]+(?=/)",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractPathParam(tc.path, tc.pattern)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got value %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestExtractLLMAnalyticsBedrockPathParam verifies the AWS Bedrock template
// shape end-to-end: token counts come from the response payload ($.usage.*)
// while the model id is taken from the request URL path (location: pathParam),
// which is the only place native Bedrock exposes it. The path is present on both
// the buffered and streaming response contexts, so this resolves in either case.
func TestExtractLLMAnalyticsBedrockPathParam(t *testing.T) {
	template := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "awsbedrock"},
		"spec": map[string]interface{}{
			"displayName":      "AWS Bedrock",
			"promptTokens":     map[string]interface{}{"location": "payload", "identifier": "$.usage.inputTokens"},
			"completionTokens": map[string]interface{}{"location": "payload", "identifier": "$.usage.outputTokens"},
			"totalTokens":      map[string]interface{}{"location": "payload", "identifier": "$.usage.totalTokens"},
			"requestModel":     map[string]interface{}{"location": "pathParam", "identifier": "model/([A-Za-z0-9.:-]+)/"},
			"responseModel":    map[string]interface{}{"location": "pathParam", "identifier": "model/([A-Za-z0-9.:-]+)/"},
		},
	}

	// Bedrock Converse numbers arrive as JSON numbers → float64 once unmarshalled.
	responseJSON := map[string]interface{}{
		"usage": map[string]interface{}{
			"inputTokens":  float64(11),
			"outputTokens": float64(7),
			"totalTokens":  float64(18),
		},
	}

	const requestPath = "/bedrock/model/us.amazon.nova-lite-v1:0/converse"

	info, err := extractLLMAnalyticsFromJSON(template, nil, nil, nil, responseJSON, requestPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.PromptTokens == nil || *info.PromptTokens != 11 {
		t.Errorf("PromptTokens = %v, want 11", info.PromptTokens)
	}
	if info.CompletionTokens == nil || *info.CompletionTokens != 7 {
		t.Errorf("CompletionTokens = %v, want 7", info.CompletionTokens)
	}
	if info.TotalTokens == nil || *info.TotalTokens != 18 {
		t.Errorf("TotalTokens = %v, want 18", info.TotalTokens)
	}
	if info.RequestModel == nil || *info.RequestModel != "us.amazon.nova-lite-v1:0" {
		t.Errorf("RequestModel = %v, want us.amazon.nova-lite-v1:0", info.RequestModel)
	}
	if info.ResponseModel == nil || *info.ResponseModel != "us.amazon.nova-lite-v1:0" {
		t.Errorf("ResponseModel = %v, want us.amazon.nova-lite-v1:0", info.ResponseModel)
	}

	// The metadata that actually reaches analytics must carry the resolved model.
	meta := map[string]any{}
	populateTokenAnalyticsMetadata(meta, info)
	if meta[ModelIDMetadataKey] != "us.amazon.nova-lite-v1:0" {
		t.Errorf("%s = %v, want us.amazon.nova-lite-v1:0", ModelIDMetadataKey, meta[ModelIDMetadataKey])
	}
	if meta[PromptTokenCountMetadataKey] != "11" {
		t.Errorf("%s = %v, want \"11\"", PromptTokenCountMetadataKey, meta[PromptTokenCountMetadataKey])
	}
}

// eventStreamFrame encodes one Amazon event-stream frame with an :event-type header. The CRC
// fields are zero: the decoder leaves integrity to the transport.
func encodeEventStreamFrame(eventType string, payload []byte) []byte {
	name := ":event-type"
	headers := []byte{byte(len(name))}
	headers = append(headers, name...)
	headers = append(headers, 7)
	headers = binary.BigEndian.AppendUint16(headers, uint16(len(eventType)))
	headers = append(headers, eventType...)

	total := eventStreamPreludeLen + len(headers) + len(payload) + 4
	frame := binary.BigEndian.AppendUint32(nil, uint32(total))
	frame = binary.BigEndian.AppendUint32(frame, uint32(len(headers)))
	frame = append(frame, 0, 0, 0, 0)
	frame = append(frame, headers...)
	frame = append(frame, payload...)
	return append(frame, 0, 0, 0, 0)
}

func bedrockStreamTemplate() map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": "awsbedrock"},
		"spec": map[string]interface{}{
			"promptTokens":     map[string]interface{}{"location": "payload", "identifier": "$.usage.inputTokens"},
			"completionTokens": map[string]interface{}{"location": "payload", "identifier": "$.usage.outputTokens"},
			"totalTokens":      map[string]interface{}{"location": "payload", "identifier": "$.usage.totalTokens"},
			"requestModel":     map[string]interface{}{"location": "pathParam", "identifier": "model/([A-Za-z0-9.:-]+)/"},
			"responseModel":    map[string]interface{}{"location": "pathParam", "identifier": "model/([A-Za-z0-9.:-]+)/"},
		},
	}
}

// A Converse stream reports its usage in the closing metadata event, behind binary framing.
func TestExtractLLMAnalyticsBedrockConverseStream(t *testing.T) {
	var body []byte
	body = append(body, encodeEventStreamFrame("messageStart", []byte(`{"role":"assistant"}`))...)
	body = append(body, encodeEventStreamFrame("contentBlockDelta", []byte(`{"delta":{"text":"hi"},"contentBlockIndex":0}`))...)
	body = append(body, encodeEventStreamFrame("messageStop", []byte(`{"stopReason":"end_turn"}`))...)
	body = append(body, encodeEventStreamFrame("metadata",
		[]byte(`{"usage":{"inputTokens":1000,"outputTokens":500,"totalTokens":1500},"metrics":{"latencyMs":90}}`))...)
	headers := policy.NewHeaders(map[string][]string{"content-type": {"application/vnd.amazon.eventstream"}})

	info, err := extractLLMProviderAnalyticsInfoFromBytes(bedrockStreamTemplate(), nil, headers, nil, body,
		"/bedrock/model/us.anthropic.claude-sonnet-4-5-20250929-v1:0/converse-stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.PromptTokens == nil || *info.PromptTokens != 1000 {
		t.Errorf("PromptTokens = %v, want 1000", info.PromptTokens)
	}
	if info.CompletionTokens == nil || *info.CompletionTokens != 500 {
		t.Errorf("CompletionTokens = %v, want 500", info.CompletionTokens)
	}
	if info.TotalTokens == nil || *info.TotalTokens != 1500 {
		t.Errorf("TotalTokens = %v, want 1500", info.TotalTokens)
	}
	if info.ResponseModel == nil || *info.ResponseModel != "us.anthropic.claude-sonnet-4-5-20250929-v1:0" {
		t.Errorf("ResponseModel = %v", info.ResponseModel)
	}

	// Without the event-stream content type the same bytes are not decoded.
	plain, err := extractLLMProviderAnalyticsInfoFromBytes(bedrockStreamTemplate(), nil, nil, nil, body,
		"/bedrock/model/x/converse-stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plain.PromptTokens != nil {
		t.Errorf("PromptTokens = %v, want nil for an undeclared binary body", *plain.PromptTokens)
	}
}

// An InvokeModelWithResponseStream stream carries model-native chunks, base64-wrapped in some
// serializations. Usage spread across chunks is deep-merged.
func TestDecodeEventStreamInvokeChunks(t *testing.T) {
	wrap := func(raw string) []byte {
		return []byte(`{"bytes":"` + base64.StdEncoding.EncodeToString([]byte(raw)) + `"}`)
	}
	var body []byte
	body = append(body, encodeEventStreamFrame("chunk", wrap(`{"type":"message_start","message":{"model":"claude"},"usage":{"input_tokens":1000}}`))...)
	body = append(body, encodeEventStreamFrame("chunk", []byte(`{"type":"message_delta","usage":{"output_tokens":500}}`))...)

	var merged map[string]interface{}
	if err := json.Unmarshal(decodeEventStream(body), &merged); err != nil {
		t.Fatalf("decoded body is not JSON: %v", err)
	}
	usage, _ := merged["usage"].(map[string]interface{})
	if usage["input_tokens"] != float64(1000) || usage["output_tokens"] != float64(500) {
		t.Errorf("usage = %v, want input_tokens 1000 and output_tokens 500", usage)
	}
	if merged["type"] != "message_delta" {
		t.Errorf("type = %v, want the later event to win at the top level", merged["type"])
	}
}

func TestDecodeEventStreamLeavesUndecodableBodies(t *testing.T) {
	for name, body := range map[string][]byte{
		"empty":         nil,
		"json":          []byte(`{"usage":{"inputTokens":1}}`),
		"short prelude": {0, 0, 0},
		"bad length":    {0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		"truncated":     encodeEventStreamFrame("metadata", []byte(`{"usage":{}}`))[:20],
	} {
		if got := decodeEventStream(body); string(got) != string(body) {
			t.Errorf("%s: decodeEventStream changed an undecodable body", name)
		}
	}
}

// Only a POST invokes a model; reading a stored result back must not report its usage again.
func TestDropUsageUnlessInvocation(t *testing.T) {
	n := func(v int64) *int64 { return &v }
	model := "gpt-4o"
	for method, keep := range map[string]bool{"POST": true, "post": true, "": true, "GET": false, "DELETE": false} {
		info := &LLMProviderAnalyticsInfo{PromptTokens: n(1), CompletionTokens: n(2), TotalTokens: n(3), ResponseModel: &model}
		dropUsageUnlessInvocation(info, method)
		if kept := info.PromptTokens != nil && info.CompletionTokens != nil && info.TotalTokens != nil; kept != keep {
			t.Errorf("method %q: counts kept = %v, want %v", method, kept, keep)
		}
		if info.ResponseModel == nil {
			t.Errorf("method %q: the model must stay on the event", method)
		}
	}
}

package analytics

import "testing"

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

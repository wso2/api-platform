package analytics

import "testing"

// bedrockModelPattern mirrors the requestModel/responseModel identifier of the
// built-in awsbedrock template.
const bedrockModelPattern = "model/([A-Za-z0-9._:%-]+)/"

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
			pattern: bedrockModelPattern,
			want:    "us.amazon.nova-lite-v1:0",
		},
		{
			name:    "bedrock streaming model id",
			path:    "/bedrock/model/us.anthropic.claude-3-5-sonnet-20240620-v1:0/converse-stream",
			pattern: bedrockModelPattern,
			want:    "us.anthropic.claude-3-5-sonnet-20240620-v1:0",
		},
		{
			name:    "percent-encoded inference profile arn is decoded",
			path:    "/bedrock/model/arn%3Aaws%3Abedrock%3Aus-east-1%3A123456789012%3Ainference-profile%2Fus.anthropic.claude-3-5-sonnet-20240620-v1%3A0/converse",
			pattern: bedrockModelPattern,
			want:    "arn:aws:bedrock:us-east-1:123456789012:inference-profile/us.anthropic.claude-3-5-sonnet-20240620-v1:0",
		},
		{
			name:    "application inference profile arn on converse-stream",
			path:    "/bedrock/model/arn%3Aaws%3Abedrock%3Aeu-west-1%3A123456789012%3Aapplication-inference-profile%2Fa1b2c3d4e5f6/converse-stream",
			pattern: bedrockModelPattern,
			want:    "arn:aws:bedrock:eu-west-1:123456789012:application-inference-profile/a1b2c3d4e5f6",
		},
		{
			name:    "underscore in model id",
			path:    "/bedrock/model/my_imported_model/converse",
			pattern: bedrockModelPattern,
			want:    "my_imported_model",
		},
		{
			name:    "malformed percent escape is returned undecoded",
			path:    "/bedrock/model/bad%zzid/converse",
			pattern: bedrockModelPattern,
			want:    "bad%zzid",
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
			pattern: bedrockModelPattern,
			wantErr: true,
		},
		{
			name:    "no match errors",
			path:    "/bedrock/health",
			pattern: bedrockModelPattern,
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
			"requestModel":     map[string]interface{}{"location": "pathParam", "identifier": bedrockModelPattern},
			"responseModel":    map[string]interface{}{"location": "pathParam", "identifier": bedrockModelPattern},
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

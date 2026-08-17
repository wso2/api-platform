package llmusage

import "testing"

func TestExtractFromPath(t *testing.T) {
	const bedrockPattern = `model/([A-Za-z0-9.:%-]+)/`

	tests := []struct {
		name        string
		requestPath string
		pattern     string
		want        string
	}{
		{
			name:        "plain bedrock model id",
			requestPath: "/model/anthropic.claude-3-sonnet-20240229-v1:0/converse",
			pattern:     bedrockPattern,
			want:        "anthropic.claude-3-sonnet-20240229-v1:0",
		},
		{
			name:        "cross-region inference profile",
			requestPath: "/model/us.anthropic.claude-3-5-sonnet-20240620-v1:0/converse-stream",
			pattern:     bedrockPattern,
			want:        "us.anthropic.claude-3-5-sonnet-20240620-v1:0",
		},
		{
			name:        "percent-encoded arn is decoded",
			requestPath: "/model/arn%3Aaws%3Abedrock%3Aus-east-1%3A123%3Ainference-profile%2Fus.anthropic.claude-3-5-sonnet-20240620-v1%3A0/converse",
			pattern:     bedrockPattern,
			want:        "arn:aws:bedrock:us-east-1:123:inference-profile/us.anthropic.claude-3-5-sonnet-20240620-v1:0",
		},
		{
			name:        "gemini lookbehind pattern",
			requestPath: "/v1beta/models/gemini-2.5-flash:streamGenerateContent",
			pattern:     `models/([a-zA-Z0-9.\-]+)`,
			want:        "gemini-2.5-flash",
		},
		{
			name:        "no match returns empty",
			requestPath: "/chat/completions",
			pattern:     bedrockPattern,
			want:        "",
		},
		{
			name:        "unparseable pattern returns empty rather than panicking",
			requestPath: "/model/x/converse",
			pattern:     `model/([A-Za-z`,
			want:        "",
		},
		{
			name:        "pattern with no capture group returns empty",
			requestPath: "/model/x/converse",
			pattern:     `model/`,
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractFromPath(tt.requestPath, tt.pattern); got != tt.want {
				t.Errorf("extractFromPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractUsage_ModelFromPathParam(t *testing.T) {
	tmpl := map[string]interface{}{
		"promptTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.inputTokens",
		},
		"completionTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.outputTokens",
		},
		"responseModel": map[string]interface{}{
			"location": "pathParam", "identifier": `model/([A-Za-z0-9.:%-]+)/`,
		},
	}
	// A Bedrock Converse response carries no model field at all.
	body := []byte(`{"usage":{"inputTokens":100,"outputTokens":50}}`)

	got, err := extractUsage(tmpl, body, nil, "/model/anthropic.claude-3-sonnet-20240229-v1:0/converse")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}

	if got.Model != "anthropic.claude-3-sonnet-20240229-v1:0" {
		t.Errorf("Model = %q, want the id from the request path", got.Model)
	}
	if got.TotalInputTokens != 100 {
		t.Errorf("TotalInputTokens = %d, want 100", got.TotalInputTokens)
	}
}

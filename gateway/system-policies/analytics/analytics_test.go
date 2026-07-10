package analytics

import (
	"context"
	"encoding/json"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func TestDeriveMCPCapability(t *testing.T) {
	cases := []struct {
		method string
		want   string
	}{
		{"tools/call", "TOOL"},
		{"tools/list", "TOOL"},
		{"resources/read", "RESOURCE"},
		{"resources/list", "RESOURCE"},
		{"prompts/get", "PROMPT"},
		{"prompts/list", "PROMPT"},
		{"initialize", ""},
		{"ping", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := deriveMCPCapability(c.method); got != c.want {
			t.Errorf("deriveMCPCapability(%q) = %q, want %q", c.method, got, c.want)
		}
	}
}

// OnResponseHeaders must capture the response content type for every API kind (not just
// MCP), since the Envoy access log carries no response headers. It reads it from the live
// response headers and emits it as response_content_type analytics metadata.
func TestOnResponseHeaders_CapturesContentTypeForAllKinds(t *testing.T) {
	cases := []struct {
		name    string
		apiKind policy.APIKind
	}{
		{"rest", policy.APIKindRestApi},
		{"llm provider", policy.APIKindLlmProvider},
		{"mcp", policy.APIKindMCP},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			respCtx := &policy.ResponseHeaderContext{
				SharedContext: &policy.SharedContext{APIKind: c.apiKind},
				ResponseHeaders: policy.NewHeaders(map[string][]string{
					"content-type": {"application/json"},
				}),
				ResponseStatus: 200,
			}

			action := (&AnalyticsPolicy{}).OnResponseHeaders(context.Background(), respCtx, nil)

			mods, ok := action.(policy.DownstreamResponseHeaderModifications)
			if !ok {
				t.Fatalf("expected DownstreamResponseHeaderModifications, got %T", action)
			}
			if got := mods.AnalyticsMetadata["response_content_type"]; got != "application/json" {
				t.Errorf("response_content_type = %v, want application/json", got)
			}
		})
	}
}

func TestExtractMCPResponseAnalyticsProps_IsError(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    bool
	}{
		{"protocol error", `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`, true},
		{"tool result error", `{"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[]}}`, true},
		{"tool result success", `{"jsonrpc":"2.0","id":1,"result":{"isError":false,"content":[]}}`, false},
		{"result without isError", `{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`, false},
		{"null error is not an error", `{"jsonrpc":"2.0","id":1,"error":null,"result":{}}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(c.payload), &payload); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			props := extractMCPResponseAnalyticsProps(payload)
			if props == nil {
				t.Fatal("expected non-nil props")
			}
			if props.IsError == nil {
				t.Fatal("expected IsError to always be set")
			}
			if *props.IsError != c.want {
				t.Errorf("IsError = %v, want %v", *props.IsError, c.want)
			}
		})
	}
}

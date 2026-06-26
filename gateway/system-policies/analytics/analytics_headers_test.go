package analytics

import (
	"encoding/json"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func TestGetHeaderFlags(t *testing.T) {
	cases := []struct {
		name     string
		params   map[string]interface{}
		wantReq  bool
		wantResp bool
	}{
		{"nil params", nil, false, false},
		{"absent", map[string]interface{}{}, false, false},
		{"bool true", map[string]interface{}{"send_request_headers": true, "send_response_headers": true}, true, true},
		{"string true", map[string]interface{}{"send_request_headers": "true"}, true, false},
		{"mixed", map[string]interface{}{"send_request_headers": false, "send_response_headers": "yes"}, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotReq, gotResp := getHeaderFlags(c.params)
			if gotReq != c.wantReq || gotResp != c.wantResp {
				t.Fatalf("getHeaderFlags(%v) = (%v, %v), want (%v, %v)", c.params, gotReq, gotResp, c.wantReq, c.wantResp)
			}
		})
	}
}

func TestSerializeHeaders(t *testing.T) {
	// Empty headers -> empty string.
	if got := serializeHeaders(policy.NewHeaders(nil)); got != "" {
		t.Fatalf("serializeHeaders(empty) = %q, want \"\"", got)
	}

	h := policy.NewHeaders(map[string][]string{
		"Authorization": {"Bearer secret"},
		"X-Foo":         {"a", "b"},
	})
	got := serializeHeaders(h)
	if got == "" {
		t.Fatal("serializeHeaders returned empty for non-empty headers")
	}

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, got)
	}
	// NewHeaders lower-cases keys; multi-value headers are joined with ", ".
	if decoded["authorization"] != "Bearer secret" {
		t.Errorf("authorization = %q, want %q", decoded["authorization"], "Bearer secret")
	}
	if decoded["x-foo"] != "a, b" {
		t.Errorf("x-foo = %q, want %q", decoded["x-foo"], "a, b")
	}
}

// Note: payload-size capping moved out of the capture path (the collector now
// captures full bodies); truncation is applied output-side by the traffic-logging
// publisher (traffic_logging.max_payload_size) and tested there.

package analytics

import (
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
		{"bool true", map[string]interface{}{"request_headers": true, "response_headers": true}, true, true},
		{"string true", map[string]interface{}{"request_headers": "true"}, true, false},
		{"mixed", map[string]interface{}{"request_headers": false, "response_headers": "yes"}, false, true},
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

func TestFlattenHeaders(t *testing.T) {
	// Empty headers -> nil.
	if got := flattenHeaders(policy.NewHeaders(nil)); got != nil {
		t.Fatalf("flattenHeaders(empty) = %v, want nil", got)
	}

	h := policy.NewHeaders(map[string][]string{
		"Authorization": {"Bearer secret"},
		"X-Foo":         {"a", "b"},
	})
	got := flattenHeaders(h)
	if len(got) == 0 {
		t.Fatal("flattenHeaders returned empty for non-empty headers")
	}

	// NewHeaders lower-cases keys; multi-value headers are joined with ", ".
	if got["authorization"] != "Bearer secret" {
		t.Errorf("authorization = %q, want %q", got["authorization"], "Bearer secret")
	}
	if got["x-foo"] != "a, b" {
		t.Errorf("x-foo = %q, want %q", got["x-foo"], "a, b")
	}
}

// Note: payload-size capping moved out of the capture path (the collector now
// captures full bodies); truncation is applied output-side by the traffic-logging
// publisher (traffic_logging.max_payload_size) and tested there.

package kernel

import (
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

// End-to-end publishing check: take the exact map a cost policy returns in
// AnalyticsMetadata, run it through the kernel's analytics struct and dynamic
// metadata builders, and confirm aiCost arrives under analytics_data as a nested
// object — the shape the analytics pipeline asserts on.
func TestAICostReachesAnalyticsDataAsNestedObject(t *testing.T) {
	policyMetadata := map[string]any{
		"x-llm-cost":               0.01146,
		"aitoken:modelid":          "gemini-3-flash-preview",
		"aitoken:prompttokencount": "10000",
		constants.AICostMetadataKey: map[string]interface{}{
			"serviceTier":     "priority",
			"promptTokenCost": 0.009,
			"prompt_tokens_details": map[string]interface{}{
				"cached_tokens_cost": 0.0005,
			},
		},
	}

	analyticsStruct, err := buildAnalyticsStruct(policyMetadata, nil)
	if err != nil {
		t.Fatalf("buildAnalyticsStruct dropped everything: %v", err)
	}

	dyn := buildDynamicMetadata(analyticsStruct, nil, nil)
	if dyn == nil {
		t.Fatal("buildDynamicMetadata returned nil")
	}

	ns := dyn.Fields[constants.ExtProcFilterName]
	if ns == nil {
		t.Fatalf("ext_proc namespace absent; namespaces present: %v", dyn.Fields)
	}
	ad := ns.GetStructValue().GetFields()["analytics_data"]
	if ad == nil {
		t.Fatal("analytics_data absent")
	}
	fields := ad.GetStructValue().GetFields()

	raw := fields[constants.AICostMetadataKey]
	if raw == nil {
		t.Fatal("aiCost absent from analytics_data")
	}
	if s := raw.GetStringValue(); s != "" {
		t.Fatalf("aiCost was stringified, analytics will drop it: %s", s)
	}

	got, ok := raw.AsInterface().(map[string]interface{})
	if !ok {
		t.Fatalf("aiCost AsInterface = %T, analytics asserts map[string]interface{}", raw.AsInterface())
	}
	if got["serviceTier"] != "priority" {
		t.Errorf("serviceTier = %v", got["serviceTier"])
	}
	if got["promptTokenCost"] != 0.009 {
		t.Errorf("promptTokenCost = %v", got["promptTokenCost"])
	}
	nested, ok := got["prompt_tokens_details"].(map[string]interface{})
	if !ok {
		t.Fatalf("nesting lost: %#v", got)
	}
	if nested["cached_tokens_cost"] != 0.0005 {
		t.Errorf("cached_tokens_cost = %v", nested["cached_tokens_cost"])
	}
	t.Logf("aiCost published as: %#v", got)
}

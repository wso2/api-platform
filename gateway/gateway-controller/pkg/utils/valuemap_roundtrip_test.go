package utils

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// The loader converts template YAML into the typed model and re-marshals it for
// delivery, so a key absent from the type is dropped without an error. This
// pins valueMap to that path.
func TestTemplateYAMLRoundTripKeepsValueMap(t *testing.T) {
	source := []byte(`
serviceTier:
  location: payload
  identifier: $.usageMetadata.trafficType
  valueMap:
    ON_DEMAND_PRIORITY: priority
    ON_DEMAND_FLEX: flex
`)

	var generic map[string]interface{}
	if err := yaml.Unmarshal(source, &generic); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	asJSON, err := json.Marshal(generic)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var spec struct {
		ServiceTier api.ExtractionIdentifier `json:"serviceTier"`
	}
	if err := json.Unmarshal(asJSON, &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if spec.ServiceTier.ValueMap == nil {
		t.Fatal("valueMap was dropped by the typed model")
	}
	if got := (*spec.ServiceTier.ValueMap)["ON_DEMAND_PRIORITY"]; got != "priority" {
		t.Errorf("ON_DEMAND_PRIORITY = %q, want priority", got)
	}
	if got := (*spec.ServiceTier.ValueMap)["ON_DEMAND_FLEX"]; got != "flex" {
		t.Errorf("ON_DEMAND_FLEX = %q, want flex", got)
	}
}

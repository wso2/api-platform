package utils

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// The loader converts template YAML through the typed model, so a key the type
// does not carry is dropped without an error. This pins providerFields to that
// path, including that each entry keeps its location and identifier.
func TestTemplateYAMLRoundTripKeepsProviderFields(t *testing.T) {
	source := []byte(`
providerFields:
  cacheDetails:
    location: payload
    identifier: $.usage.cacheDetails
  cacheTokensDetails:
    location: payload
    identifier: $.usageMetadata.cacheTokensDetails
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
		ProviderFields *map[string]api.ExtractionIdentifier `json:"providerFields"`
	}
	if err := json.Unmarshal(asJSON, &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if spec.ProviderFields == nil {
		t.Fatal("providerFields was dropped by the typed model")
	}
	got := *spec.ProviderFields
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %#v", len(got), got)
	}
	if got["cacheDetails"].Identifier != "$.usage.cacheDetails" {
		t.Errorf("cacheDetails identifier = %q", got["cacheDetails"].Identifier)
	}
	if string(got["cacheTokensDetails"].Location) != "payload" {
		t.Errorf("cacheTokensDetails location = %q", got["cacheTokensDetails"].Location)
	}
}

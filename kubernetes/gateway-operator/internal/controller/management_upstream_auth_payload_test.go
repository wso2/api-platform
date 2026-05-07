package controller

import (
	"testing"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

func TestFlattenUpstreamAuthCredentialValue_nestedValueBecomesPlainString(t *testing.T) {
	token := "Bearer x"
	spec := apiv1.LLMProviderConfigData{
		DisplayName: "p",
		Version:     "v1.0",
		Template:    "openai",
		AccessControl: apiv1.LLMAccessControl{
			Mode: "allow_all",
		},
		Upstream: apiv1.LLMProviderUpstream{
			Url: strPtr("https://example/upstream"),
			Auth: &apiv1.LLMUpstreamAuth{
				Type:   "api-key",
				Header: strPtr("Authorization"),
				Value:  apiv1.SecretValueSource{Value: &token},
			},
		},
	}
	m, err := specToJSONMap(spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := flattenUpstreamAuthCredentialValue(m, "upstream", token); err != nil {
		t.Fatal(err)
	}
	upstream, ok := m["upstream"].(map[string]interface{})
	if !ok {
		t.Fatal("upstream not map")
	}
	auth, ok := upstream["auth"].(map[string]interface{})
	if !ok {
		t.Fatal("auth not map")
	}
	v, ok := auth["value"].(string)
	if !ok || v != token {
		t.Fatalf("auth.value got %v (%T), want string %q", auth["value"], auth["value"], token)
	}
}

func strPtr(s string) *string { return &s }

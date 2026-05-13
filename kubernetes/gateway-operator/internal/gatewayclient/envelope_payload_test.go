package gatewayclient

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	yamlv3 "gopkg.in/yaml.v3"
)

func TestBuildEnvelopeYAML_LlmProvider(t *testing.T) {
	context := "/openai"
	state := "deployed"
	header := "Authorization"
	val := "Bearer secret"
	spec := apiv1.LLMProviderConfigData{
		DisplayName: "OpenAI",
		Version:     "v1.0",
		Template:    "openai-template",
		AccessControl: apiv1.LLMAccessControl{
			Mode: "allow_all",
		},
		Upstream: apiv1.LLMProviderUpstream{
			Url: stringPtr("https://api.openai.com"),
			Auth: &apiv1.LLMUpstreamAuth{
				Type:   "api-key",
				Header: &header,
				Value:  apiv1.SecretValueSource{Value: &val},
			},
		},
		Context:         &context,
		DeploymentState: &state,
	}
	md := EnvelopeMetadata{
		Name:        "openai-provider",
		Labels:      map[string]string{"app": "ai"},
		Annotations: map[string]string{"team": "platform"},
	}

	b, err := BuildEnvelopeYAML(apiv1.GroupVersion.String(), "LlmProvider", md, spec)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, yamlv3.Unmarshal(b, &got))
	require.Equal(t, "LlmProvider", got["kind"])

	metadata, ok := got["metadata"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "openai-provider", metadata["name"])

	labels, ok := metadata["labels"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "ai", labels["app"])

	specOut, ok := got["spec"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "OpenAI", specOut["displayName"])
	require.Equal(t, "/openai", specOut["context"])
	require.Equal(t, "deployed", specOut["deploymentState"])

	upstream, ok := specOut["upstream"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "https://api.openai.com", upstream["url"])

	auth, ok := upstream["auth"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "api-key", auth["type"])
	value, ok := auth["value"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "Bearer secret", value["value"])
}

func TestBuildEnvelopeYAML_OmitsUnsetOptionals(t *testing.T) {
	spec := apiv1.LLMProviderTemplateData{
		DisplayName: "openai-template",
	}
	md := EnvelopeMetadata{Name: "openai-template"}

	b, err := BuildEnvelopeYAML(apiv1.GroupVersion.String(), "LlmProviderTemplate", md, spec)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, yamlv3.Unmarshal(b, &got))

	specOut, ok := got["spec"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "openai-template", specOut["displayName"])
	require.NotContains(t, specOut, "completionTokens")
	require.NotContains(t, specOut, "resourceMappings")
}

func stringPtr(s string) *string { return &s }

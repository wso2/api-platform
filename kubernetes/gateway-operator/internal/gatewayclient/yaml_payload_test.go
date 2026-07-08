package gatewayclient

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	yamlv3 "gopkg.in/yaml.v3"
)

func TestBuildRestAPIYAML_IncludesMetadataAnnotationsAndLabels(t *testing.T) {
	spec := apiv1.APIConfigData{
		Context:     "/hello",
		DisplayName: "hello",
		Version:     "v1.0",
		Operations: []apiv1.Operation{
			{Method: apiv1.OperationMethodGET, Path: "/"},
		},
		Upstream: apiv1.UpstreamConfig{
			Main: apiv1.Upstream{Url: stringPtr("http://hello.default.svc.cluster.local:9080")},
		},
	}
	md := RestAPIPayloadMetadata{
		Name:        "api-handle",
		Labels:      map[string]string{"app": "demo"},
		Annotations: map[string]string{"project-id": "1234567890"},
	}

	b, err := BuildRestAPIYAML(apiv1.GroupVersion.String(), "RestApi", md, spec)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, yamlv3.Unmarshal(b, &got))
	metadata, ok := got["metadata"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "api-handle", metadata["name"])

	labels, ok := metadata["labels"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "demo", labels["app"])

	annotations, ok := metadata["annotations"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "1234567890", annotations["project-id"])
}

// A RestApi spec using upstreamDefinitions + upstream.main.ref must survive the verbatim
// marshal to the management-API payload, so the operator reaches parity with a direct
// management-API deploy (the connect timeout is carried on the definition).
func TestBuildRestAPIYAML_CarriesUpstreamDefinitionsAndRef(t *testing.T) {
	spec := apiv1.APIConfigData{
		Context:     "/hello",
		DisplayName: "hello",
		Version:     "v1.0",
		Operations: []apiv1.Operation{
			{Method: apiv1.OperationMethodGET, Path: "/"},
		},
		UpstreamDefinitions: []apiv1.UpstreamDefinition{{
			Name:      "hello-backend",
			Timeout:   &apiv1.UpstreamTimeout{Connect: stringPtr("6s")},
			Upstreams: []apiv1.UpstreamTarget{{Url: "http://hello.default.svc.cluster.local:9080"}},
		}},
		Upstream: apiv1.UpstreamConfig{
			Main: apiv1.Upstream{Ref: stringPtr("hello-backend")},
		},
	}
	md := RestAPIPayloadMetadata{Name: "api-handle"}

	b, err := BuildRestAPIYAML(apiv1.GroupVersion.String(), "RestApi", md, spec)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, yamlv3.Unmarshal(b, &got))
	specMap, ok := got["spec"].(map[string]interface{})
	require.True(t, ok)

	// upstream.main.ref survives; url is omitted when ref is used.
	upstream := specMap["upstream"].(map[string]interface{})
	main := upstream["main"].(map[string]interface{})
	require.Equal(t, "hello-backend", main["ref"])
	_, hasURL := main["url"]
	require.False(t, hasURL, "url should be omitted when ref is set")

	// upstreamDefinitions survive with the connect timeout.
	defs, ok := specMap["upstreamDefinitions"].([]interface{})
	require.True(t, ok)
	require.Len(t, defs, 1)
	def0 := defs[0].(map[string]interface{})
	require.Equal(t, "hello-backend", def0["name"])
	timeout := def0["timeout"].(map[string]interface{})
	require.Equal(t, "6s", timeout["connect"])
}

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
			Main: apiv1.Upstream{Url: "http://hello.default.svc.cluster.local:9080"},
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

package gatewayclient

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	yamlv3 "gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestBuildRestAPIYAML_Deterministic guards the assumption behind the HTTPRoute reconciler's
// "skip redeploy when config unchanged" hash guard: building the same spec repeatedly must yield
// byte-identical output, including map-typed policy params. If this ever regresses (e.g. a Go map
// leaks non-deterministic ordering into the payload), the operator would re-deploy on every
// reconcile and the gateway-controller would churn xDS.
func TestBuildRestAPIYAML_Deterministic(t *testing.T) {
	rawParams := runtime.RawExtension{Raw: []byte(`{"targetUpstream":"ns-svc-8080","attachedTo":"route","extra":{"b":2,"a":1}}`)}
	spec := apiv1.APIConfigData{
		Context:     "/",
		DisplayName: "header-matching",
		Version:     "v1.0",
		Operations: []apiv1.Operation{
			{
				Method:       apiv1.OperationMethodGET,
				Path:         "/*",
				MatchHeaders: []apiv1.OperationHeaderMatch{{Name: "version", Value: "two", Type: "Exact"}},
				Policies:     []apiv1.Policy{{Name: "dynamic-endpoint", Version: "v1", Params: &rawParams}},
			},
		},
		UpstreamDefinitions: []apiv1.UpstreamDefinition{
			{Name: "ns-infra-backend-v1-8080", Upstreams: []apiv1.WeightedUpstream{{Url: "http://v1:8080"}}},
			{Name: "ns-infra-backend-v2-8080", Upstreams: []apiv1.WeightedUpstream{{Url: "http://v2:8080"}}},
		},
		Upstream: apiv1.UpstreamConfig{Main: apiv1.Upstream{Url: stringPtr("http://v1:8080")}},
	}
	md := RestAPIPayloadMetadata{Name: "api-handle"}

	first, err := BuildRestAPIYAML(apiv1.GroupVersion.String(), "RestApi", md, spec)
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		again, err := BuildRestAPIYAML(apiv1.GroupVersion.String(), "RestApi", md, spec)
		require.NoError(t, err)
		require.Equal(t, string(first), string(again), "payload must be byte-stable across builds (iteration %d)", i)
	}
}

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
			Upstreams: []apiv1.WeightedUpstream{{Url: "http://hello.default.svc.cluster.local:9080"}},
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

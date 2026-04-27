package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestPayloadMetadataForHTTPRoute_CopiesAllAnnotations(t *testing.T) {
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				AnnProjectID:                         "1234567890",
				"gateway.api-platform.wso2.com/extra": "keep-me",
			},
		},
	}
	md := payloadMetadataForHTTPRoute(route, "handle-1")
	require.Equal(t, "handle-1", md.Name)
	require.Equal(t, "1234567890", md.Annotations[AnnProjectID])
	require.Equal(t, "keep-me", md.Annotations["gateway.api-platform.wso2.com/extra"])
}

func TestPayloadMetadataForRestAPI_CopiesAllAnnotationsAndLabels(t *testing.T) {
	apiCfg := &apiv1.RestApi{
		ObjectMeta: metav1.ObjectMeta{
			Name: "api-1",
			Labels: map[string]string{
				"k": "v",
			},
			Annotations: map[string]string{
				AnnProjectID:                         "proj-1",
				"gateway.api-platform.wso2.com/extra": "x",
			},
		},
	}
	md := payloadMetadataForRestAPI(apiCfg)
	require.Equal(t, "api-1", md.Name)
	require.Equal(t, "v", md.Labels["k"])
	require.Equal(t, "proj-1", md.Annotations[AnnProjectID])
	require.Equal(t, "x", md.Annotations["gateway.api-platform.wso2.com/extra"])
}

/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helmgateway

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
)

const (
	testGateway   = "billing"
	testNamespace = "apk"
	managedClass  = "wso2-gateway"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	utilruntime.Must(apiv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.Install(scheme))
	return scheme
}

func testConfig() *config.OperatorConfig {
	cfg := &config.OperatorConfig{}
	cfg.GatewayAPI.GatewayClassNames = []string{managedClass}
	return cfg
}

func cleanupInput(t *testing.T, fromGatewayAPI bool, objs ...client.Object) legacyCleanupInput {
	t.Helper()
	return legacyCleanupInput{
		Client: fake.NewClientBuilder().
			WithScheme(testScheme(t)).
			WithObjects(objs...).
			Build(),
		Config:         testConfig(),
		GatewayName:    testGateway,
		Namespace:      testNamespace,
		FromGatewayAPI: fromGatewayAPI,
	}
}

func k8sGateway(class string) *gatewayv1.Gateway {
	return &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: testGateway, Namespace: testNamespace},
		Spec:       gatewayv1.GatewaySpec{GatewayClassName: gatewayv1.ObjectName(class)},
	}
}

func apiGateway() *apiv1.APIGateway {
	return &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{Name: testGateway, Namespace: testNamespace},
	}
}

// TestOtherKindClaimsGateway is the ownership question legacy cleanup turns on.
// The operator identifies a gateway by namespace and name alone, so a gateway of
// the other kind at the same key is a competing claim on the same Helm release.
func TestOtherKindClaimsGateway(t *testing.T) {
	for _, tc := range []struct {
		name           string
		fromGatewayAPI bool
		objs           []client.Object
		want           bool
	}{
		{
			name:           "APIGateway reconcile, no Kubernetes Gateway at this key",
			fromGatewayAPI: false,
			want:           false,
		},
		{
			name:           "APIGateway reconcile, a managed Kubernetes Gateway claims the same key",
			fromGatewayAPI: false,
			objs:           []client.Object{k8sGateway(managedClass)},
			want:           true,
		},
		{
			// Must stay false: a Gateway on somebody else's class never owned our
			// release, and treating it as a claim would orphan the legacy release
			// for good.
			name:           "APIGateway reconcile, Kubernetes Gateway on an unmanaged class",
			fromGatewayAPI: false,
			objs:           []client.Object{k8sGateway("other-vendor")},
			want:           false,
		},
		{
			name:           "Gateway API reconcile, no APIGateway at this key",
			fromGatewayAPI: true,
			want:           false,
		},
		{
			name:           "Gateway API reconcile, an APIGateway claims the same key",
			fromGatewayAPI: true,
			objs:           []client.Object{apiGateway()},
			want:           true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := otherKindClaimsGateway(context.Background(), cleanupInput(t, tc.fromGatewayAPI, tc.objs...))
			if err != nil {
				t.Fatalf("otherKindClaimsGateway() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("otherKindClaimsGateway() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestOtherKindClaimsGateway_ignoresTheSameKind pins that a reconcile is not its
// own competitor: only the other kind is looked up, so a gateway never declines
// to clean up after itself.
func TestOtherKindClaimsGateway_ignoresTheSameKind(t *testing.T) {
	got, err := otherKindClaimsGateway(context.Background(), cleanupInput(t, false, apiGateway()))
	if err != nil {
		t.Fatalf("otherKindClaimsGateway() error = %v", err)
	}
	if got {
		t.Fatal("an APIGateway reconcile must not treat its own APIGateway as a competing claim")
	}
}

package controller

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLlmProviderExternalDepsDrift_SecretRotation(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	params, err := json.Marshal(map[string]any{
		"k": map[string]any{"valueFrom": map[string]any{
			"secretKeyRef": map[string]any{"name": "pol-secret", "key": "x"},
		}},
	})
	require.NoError(t, err)

	hdr := "Authorization"
	authType := "api-key"
	key := "provider-token"

	cr := func(ann map[string]string) *apiv1.LlmProvider {
		return &apiv1.LlmProvider{
			ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "demo", Annotations: ann},
			Spec: apiv1.LLMProviderConfigData{
				DisplayName: "p1",
				Version:     "v1.0",
				Template:    "t1",
				AccessControl: apiv1.LLMAccessControl{
					Mode: "allow_all",
				},
				Upstream: apiv1.LLMProviderUpstream{
					Auth: &apiv1.LLMUpstreamAuth{
						Type:   authType,
						Header: &hdr,
						Value: apiv1.SecretValueSource{
							ValueFrom: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "upstream-secret"},
								Key:                  key,
							},
						},
					},
				},
				Policies: []apiv1.LLMPolicy{{
					Name:    "policy-a",
					Version: "v1",
					Paths: []apiv1.LLMPolicyPath{{
						Path:    "/x",
						Methods: []apiv1.HTTPMethod{"GET"},
						Params:  &runtime.RawExtension{Raw: params},
					}},
				}},
			},
		}
	}

	sec1 := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "upstream-secret", ResourceVersion: "10"}}
	sec2 := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "pol-secret", ResourceVersion: "11"}}
	c1 := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec1, sec2).Build()

	ad := &llmProviderAdapter{}
	fp1, err := llmProviderExternalDepsFingerprint(context.Background(), c1, cr(nil))
	require.NoError(t, err)
	require.NotEmpty(t, fp1)

	need1, err := ad.needsRedeployForExternalDeps(context.Background(), c1, cr(map[string]string{
		annLlmProviderPolicyValueFromFingerprint: fp1,
	}))
	require.NoError(t, err)
	require.False(t, need1)

	sec1b := sec1.DeepCopy()
	sec1b.ResourceVersion = "12"
	c2 := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec1b, sec2).Build()
	need2, err := ad.needsRedeployForExternalDeps(context.Background(), c2, cr(map[string]string{
		annLlmProviderPolicyValueFromFingerprint: fp1,
	}))
	require.NoError(t, err)
	require.True(t, need2)
}

func TestMcpExternalDepsDrift_ConfigMapRotation(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	raw, err := json.Marshal(map[string]any{
		"region": map[string]any{"valueFrom": map[string]any{
			"configMapKeyRef": map[string]any{"name": "cm1", "key": "k"},
		}},
	})
	require.NoError(t, err)

	cr := func(ann map[string]string) *apiv1.Mcp {
		return &apiv1.Mcp{
			ObjectMeta: metav1.ObjectMeta{Name: "m1", Namespace: "demo", Annotations: ann},
			Spec: apiv1.MCPProxyConfigData{
				DisplayName: "m1",
				Version:     "v1.0",
				Upstream:    apiv1.MCPUpstream{Url: ptr("http://x")},
				Policies: []apiv1.Policy{{
					Name:    "p",
					Version: "v1",
					Params:  &runtime.RawExtension{Raw: raw},
				}},
			},
		}
	}

	cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "cm1", ResourceVersion: "20"}}
	c1 := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm1).Build()
	ad := &mcpAdapter{}

	fp1, err := mcpExternalDepsFingerprint(context.Background(), c1, cr(nil))
	require.NoError(t, err)
	require.Equal(t, "configmap:demo/cm1@20", fp1)

	need1, err := ad.needsRedeployForExternalDeps(context.Background(), c1, cr(map[string]string{
		annMcpPolicyValueFromFingerprint: fp1,
	}))
	require.NoError(t, err)
	require.False(t, need1)

	cm2 := cm1.DeepCopy()
	cm2.ResourceVersion = "21"
	c2 := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm2).Build()
	need2, err := ad.needsRedeployForExternalDeps(context.Background(), c2, cr(map[string]string{
		annMcpPolicyValueFromFingerprint: fp1,
	}))
	require.NoError(t, err)
	require.True(t, need2)
}

func ptr[T any](v T) *T { return &v }

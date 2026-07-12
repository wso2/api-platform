package controller

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLlmProxyDeploy_ResolvesPolicyParamsValueFrom(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	raw, err := json.Marshal(map[string]any{
		"key": map[string]any{"valueFrom": map[string]any{
			"secretKeyRef": map[string]any{"name": "demo-management-secrets", "key": "apikey-header-name"},
		}},
		"in": "header",
	})
	require.NoError(t, err)

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apigateway-demo", Name: "demo-management-secrets"},
		Data:       map[string][]byte{"apikey-header-name": []byte("X-API-Key")},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec).Build()

	cr := &apiv1.LlmProxy{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-llm-proxy-apikey", Namespace: "apigateway-demo"},
		Spec: apiv1.LLMProxyConfigData{
			DisplayName: "Lifecycle LLM Proxy APIKey",
			Version:     "v1.0",
			Provider:    apiv1.LLMProxyProvider{Id: "demo-llm-provider-apikey"},
			Policies: []apiv1.LLMPolicy{{
				Name:    "api-key-auth",
				Version: "v1",
				Paths: []apiv1.LLMPolicyPath{{
					Path:    "/chat/completions",
					Methods: []apiv1.HTTPMethod{"POST"},
					Params:  &runtime.RawExtension{Raw: raw},
				}},
			}},
		},
	}

	var (
		mu      sync.Mutex
		payload string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
			return
		}
		if r.Method == http.MethodPost {
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			payload = string(b)
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	adapter := &llmProxyAdapter{}
	_, err = adapter.Deploy(context.Background(), k8sClient, srv.URL, cr, nil)
	require.NoError(t, err)

	mu.Lock()
	got := payload
	mu.Unlock()
	require.NotEmpty(t, got)
	require.Contains(t, got, "key: X-API-Key")
	require.NotContains(t, got, "secretKeyRef")
	require.NotContains(t, got, "valueFrom")
	require.True(t, strings.Contains(got, "name: api-key-auth"))
}

func TestLlmProxyDeploy_ResolvesAdditionalProviderAuthValueFrom(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apigateway-demo", Name: "provider-secrets"},
		Data: map[string][]byte{
			"primary":    []byte("Bearer primary-key"),
			"additional": []byte("Bearer additional-key"),
		},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec).Build()

	header := "Authorization"
	authType := "api-key"
	cr := &apiv1.LlmProxy{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-llm-proxy", Namespace: "apigateway-demo"},
		Spec: apiv1.LLMProxyConfigData{
			DisplayName: "Lifecycle LLM Proxy",
			Version:     "v1.0",
			Provider: apiv1.LLMProxyProvider{
				Id: "openai-provider",
				Auth: &apiv1.LLMUpstreamAuth{
					Type:   authType,
					Header: &header,
					Value: apiv1.SecretValueSource{ValueFrom: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "provider-secrets"},
						Key:                  "primary",
					}},
				},
			},
			AdditionalProviders: []apiv1.LLMProxyAdditionalProvider{{
				Id: "anthropic-provider",
				Auth: &apiv1.LLMUpstreamAuth{
					Type:   authType,
					Header: &header,
					Value: apiv1.SecretValueSource{ValueFrom: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "provider-secrets"},
						Key:                  "additional",
					}},
				},
			}},
		},
	}

	var (
		mu      sync.Mutex
		payload string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
			return
		}
		if r.Method == http.MethodPost {
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			payload = string(b)
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	adapter := &llmProxyAdapter{}
	_, err := adapter.Deploy(context.Background(), k8sClient, srv.URL, cr, nil)
	require.NoError(t, err)

	mu.Lock()
	got := payload
	mu.Unlock()
	require.Contains(t, got, "value: Bearer primary-key")
	require.Contains(t, got, "value: Bearer additional-key")
	require.Contains(t, got, "additionalProviders:")
	require.NotContains(t, got, "secretKeyRef")
	require.NotContains(t, got, "valueFrom")
}

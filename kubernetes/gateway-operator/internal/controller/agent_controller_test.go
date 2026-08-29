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

package controller

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

// capturedRequest is one request the fake gateway-controller management API saw.
type capturedRequest struct {
	Method string
	Path   string
	Body   string
}

// fakeManagementAPI stands in for the gateway-controller management API. The
// GET existence probe answers 404 unless exists is true, so a deploy is a POST
// by default and a PUT when the resource is already present.
func fakeManagementAPI(t *testing.T, exists bool) (endpoint string, seen func() []capturedRequest) {
	t.Helper()
	var (
		mu       sync.Mutex
		requests []capturedRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, capturedRequest{Method: r.Method, Path: r.URL.Path, Body: string(body)})
		mu.Unlock()

		switch r.Method {
		case http.MethodGet:
			if exists {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("{}"))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		case http.MethodPost, http.MethodPut, http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)

	return srv.URL, func() []capturedRequest {
		mu.Lock()
		defer mu.Unlock()
		out := make([]capturedRequest, len(requests))
		copy(out, requests)
		return out
	}
}

func agentTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))
	return scheme
}

func secretRefParams(t *testing.T, secretName, key string) *runtime.RawExtension {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"key": map[string]any{"valueFrom": map[string]any{
			"secretKeyRef": map[string]any{"name": secretName, "key": key},
		}},
	})
	require.NoError(t, err)
	return &runtime.RawExtension{Raw: raw}
}

// agentWithPolicyScopes builds an Agent carrying one secret-backed policy in
// each of the three scopes an Agent has.
func agentWithPolicyScopes(t *testing.T) *apiv1.Agent {
	t.Helper()
	ctxPath := "/weather"
	prefix := "/rpc"
	return &apiv1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "weather-agent", Namespace: "apigateway-demo"},
		Spec: apiv1.AgentConfigData{
			DisplayName: "Weather Agent",
			Version:     "v1.0",
			Context:     &ctxPath,
			Upstream:    apiv1.AgentUpstream{Url: ptr("https://weather.internal")},
			A2A: apiv1.A2AConfig{
				ProtocolVersion: "1.0",
				OperationConfigs: apiv1.A2AOperationConfigs{
					Transports: []apiv1.A2ATransport{
						{ProtocolBinding: apiv1.A2AProtocolBindingJSONRPC, PathPrefix: &prefix},
					},
					Policies: []apiv1.Policy{{
						Name: "api-key-auth", Version: "v1",
						Params: secretRefParams(t, "agent-secrets", "common"),
					}},
					Operations: []apiv1.A2AOperationConfig{{
						Name: "SendMessage",
						Policies: []apiv1.Policy{{
							Name: "set-headers", Version: "v1",
							Params: secretRefParams(t, "agent-secrets", "operation"),
						}},
					}},
				},
				AgentCard: apiv1.A2AAgentCard{
					Public: apiv1.A2APublicAgentCard{
						Mode: "managed",
						Policies: []apiv1.Policy{{
							Name: "cors", Version: "v1",
							Params: secretRefParams(t, "agent-secrets", "card"),
						}},
						Content: &runtime.RawExtension{
							Raw: []byte(`{"name":"Weather Agent","capabilities":{"streaming":true}}`),
						},
					},
				},
			},
		},
	}
}

func agentSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apigateway-demo", Name: "agent-secrets"},
		Data: map[string][]byte{
			"common":    []byte("common-value"),
			"operation": []byte("operation-value"),
			"card":      []byte("card-value"),
			"upstream":  []byte("Bearer upstream-key"),
		},
	}
}

// TestAgentPolicyRefs_CoversEveryScope pins the scope-coverage invariant:
// agentPolicyRefs is the single place that knows where an Agent keeps policies,
// and valueFrom resolution, fingerprinting, and the Secret/ConfigMap watches all
// read it. A scope added to the spec but not to this function would be silently
// skipped by all three.
func TestAgentPolicyRefs_CoversEveryScope(t *testing.T) {
	cr := agentWithPolicyScopes(t)

	refs := agentPolicyRefs(&cr.Spec)
	require.Len(t, refs, 3)

	scopes := make([]string, 0, len(refs))
	for _, ref := range refs {
		scopes = append(scopes, ref.Scope)
	}
	sort.Strings(scopes)
	require.Equal(t, []string{
		"a2a.agentCard.public",
		"a2a.operationConfigs",
		"a2a.operationConfigs.operations[SendMessage]",
	}, scopes)

	// The refs must point into the caller's spec, so a resolver can rewrite
	// params in place.
	refs[0].Policy.Name = "rewritten"
	require.Equal(t, "rewritten", cr.Spec.A2A.OperationConfigs.Policies[0].Name)
}

// TestAgentPolicyRefs_EmptyAgent covers the no-policies case: the ordering of
// scopes must not depend on any of them being populated.
func TestAgentPolicyRefs_EmptyAgent(t *testing.T) {
	require.Empty(t, agentPolicyRefs(nil))
	require.Empty(t, agentPolicyRefs(&apiv1.AgentConfigData{}))
}

// TestAgentDeploy_ResolvesPolicyParamsValueFromInEveryScope asserts the deploy
// payload carries resolved plaintext for all three policy scopes, and that no
// secret reference reaches the gateway.
func TestAgentDeploy_ResolvesPolicyParamsValueFromInEveryScope(t *testing.T) {
	k8sClient := fake.NewClientBuilder().
		WithScheme(agentTestScheme(t)).
		WithObjects(agentSecret()).
		Build()
	cr := agentWithPolicyScopes(t)

	endpoint, seen := fakeManagementAPI(t, false)
	_, err := (&agentAdapter{}).Deploy(context.Background(), k8sClient, endpoint, cr, nil)
	require.NoError(t, err)

	payload := deployBody(t, seen())
	require.Contains(t, payload, "common-value")
	require.Contains(t, payload, "operation-value")
	require.Contains(t, payload, "card-value")
	require.NotContains(t, payload, "secretKeyRef")
	require.NotContains(t, payload, "valueFrom")
}

// TestAgentDeploy_DoesNotMutateTheCR asserts resolution happens on a copy. The
// CR is a cluster object read through a cache; writing resolved plaintext back
// onto it would leak the credential into etcd on the next status/annotation
// patch.
func TestAgentDeploy_DoesNotMutateTheCR(t *testing.T) {
	k8sClient := fake.NewClientBuilder().
		WithScheme(agentTestScheme(t)).
		WithObjects(agentSecret()).
		Build()
	cr := agentWithPolicyScopes(t)
	before := string(cr.Spec.A2A.OperationConfigs.Policies[0].Params.Raw)

	endpoint, _ := fakeManagementAPI(t, false)
	_, err := (&agentAdapter{}).Deploy(context.Background(), k8sClient, endpoint, cr, nil)
	require.NoError(t, err)

	require.Equal(t, before, string(cr.Spec.A2A.OperationConfigs.Policies[0].Params.Raw))
	require.Contains(t, before, "secretKeyRef")
}

// TestAgentDeploy_FlattensUpstreamAuthValueFrom asserts the CR's richer
// SecretValueSource shape is flattened to the plain string the management API
// expects, without disturbing the rest of the spec.
func TestAgentDeploy_FlattensUpstreamAuthValueFrom(t *testing.T) {
	k8sClient := fake.NewClientBuilder().
		WithScheme(agentTestScheme(t)).
		WithObjects(agentSecret()).
		Build()

	cr := agentWithPolicyScopes(t)
	cr.Spec.Upstream.Auth = &apiv1.AgentUpstreamAuth{
		Type:   "api-key",
		Header: ptr("Authorization"),
		Value: &apiv1.SecretValueSource{ValueFrom: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "agent-secrets"},
			Key:                  "upstream",
		}},
	}

	endpoint, seen := fakeManagementAPI(t, false)
	_, err := (&agentAdapter{}).Deploy(context.Background(), k8sClient, endpoint, cr, nil)
	require.NoError(t, err)

	spec := deploySpec(t, seen())
	upstream := spec["upstream"].(map[string]interface{})
	auth := upstream["auth"].(map[string]interface{})
	require.Equal(t, "Bearer upstream-key", auth["value"])
	require.Equal(t, "api-key", auth["type"])
	require.Equal(t, "Authorization", auth["header"])
	require.Equal(t, "https://weather.internal", upstream["url"])
	// Flattening goes through a generic map; the A2A subtree must survive it.
	require.Equal(t, "1.0", spec["a2a"].(map[string]interface{})["protocolVersion"])
}

// TestAgentDeploy_PreservesAgentCardContent asserts the free-form card document
// reaches the gateway with its nested values, unknown extension fields, empty
// lists, and false values intact. The gateway signs the card content it is
// given, so a re-serialization that drops a field changes what gets signed.
func TestAgentDeploy_PreservesAgentCardContent(t *testing.T) {
	k8sClient := fake.NewClientBuilder().WithScheme(agentTestScheme(t)).Build()

	cr := agentWithPolicyScopes(t)
	cr.Spec.A2A.OperationConfigs.Policies = nil
	cr.Spec.A2A.OperationConfigs.Operations = nil
	cr.Spec.A2A.AgentCard.Public.Policies = nil
	cr.Spec.A2A.AgentCard.Public.Content = &runtime.RawExtension{Raw: []byte(
		`{"name":"Weather Agent","capabilities":{"streaming":true,"pushNotifications":false},` +
			`"x-vendor":{"nested":{"deep":["a",1]}},"extensions":[],"skills":[{"id":"get_weather"}]}`)}

	endpoint, seen := fakeManagementAPI(t, false)
	_, err := (&agentAdapter{}).Deploy(context.Background(), k8sClient, endpoint, cr, nil)
	require.NoError(t, err)

	content := deploySpec(t, seen())["a2a"].(map[string]interface{})["agentCard"].(map[string]interface{})["public"].(map[string]interface{})["content"].(map[string]interface{})

	require.Equal(t, "Weather Agent", content["name"])
	caps := content["capabilities"].(map[string]interface{})
	require.Equal(t, true, caps["streaming"])
	require.Equal(t, false, caps["pushNotifications"])
	require.Equal(t, []interface{}{}, content["extensions"])
	nested := content["x-vendor"].(map[string]interface{})["nested"].(map[string]interface{})
	require.Equal(t, []interface{}{"a", float64(1)}, nested["deep"])
}

// TestAgentDeploy_TargetsTheAgentsPath asserts the adapter addresses /agents,
// POSTs when the artifact is absent, PUTs when it is present, and stamps the
// envelope kind the management API dispatches on.
func TestAgentDeploy_TargetsTheAgentsPath(t *testing.T) {
	for _, tc := range []struct {
		name       string
		exists     bool
		wantMethod string
		wantPath   string
	}{
		{"create", false, http.MethodPost, gatewayclient.AgentsPath()},
		{"update", true, http.MethodPut, gatewayclient.AgentsPath() + "/weather-agent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k8sClient := fake.NewClientBuilder().WithScheme(agentTestScheme(t)).Build()
			cr := agentWithPolicyScopes(t)
			cr.Spec.A2A.OperationConfigs.Policies = nil
			cr.Spec.A2A.OperationConfigs.Operations = nil
			cr.Spec.A2A.AgentCard.Public.Policies = nil

			endpoint, seen := fakeManagementAPI(t, tc.exists)
			_, err := (&agentAdapter{}).Deploy(context.Background(), k8sClient, endpoint, cr, nil)
			require.NoError(t, err)

			requests := seen()
			require.Len(t, requests, 2, "expected an existence probe followed by the write")
			require.Equal(t, http.MethodGet, requests[0].Method)
			require.Equal(t, gatewayclient.AgentsPath()+"/weather-agent", requests[0].Path)
			require.Equal(t, tc.wantMethod, requests[1].Method)
			require.Equal(t, tc.wantPath, requests[1].Path)

			var envelope map[string]interface{}
			require.NoError(t, yaml.Unmarshal([]byte(requests[1].Body), &envelope))
			require.Equal(t, "Agent", envelope["kind"])
			require.Equal(t, gatewayclient.ManagementArtifactAPIVersion, envelope["apiVersion"])
			require.Equal(t, "weather-agent",
				envelope["metadata"].(map[string]interface{})["name"])
		})
	}
}

// TestAgentDelete_TargetsTheAgentsPath asserts undeploy addresses the same path.
func TestAgentDelete_TargetsTheAgentsPath(t *testing.T) {
	cr := &apiv1.Agent{ObjectMeta: metav1.ObjectMeta{Name: "weather-agent", Namespace: "apigateway-demo"}}

	endpoint, seen := fakeManagementAPI(t, true)
	require.NoError(t, (&agentAdapter{}).Delete(context.Background(), endpoint, cr, nil))

	requests := seen()
	require.Len(t, requests, 1)
	require.Equal(t, http.MethodDelete, requests[0].Method)
	require.Equal(t, gatewayclient.AgentsPath()+"/weather-agent", requests[0].Path)
}

// TestAgentAdapterIdentity pins the adapter contract the generic reconciler
// depends on: an Agent is addressed by CR name, not by a gateway-issued UUID.
func TestAgentAdapterIdentity(t *testing.T) {
	a := &agentAdapter{}
	cr := &apiv1.Agent{ObjectMeta: metav1.ObjectMeta{Name: "weather-agent"}}

	require.Equal(t, "Agent", a.Kind())
	require.False(t, a.IsUUIDKeyed())
	require.Equal(t, "weather-agent", a.Handle(cr))
	require.Equal(t, agentFinalizer, a.FinalizerName())
	require.IsType(t, &apiv1.Agent{}, a.NewObject())
	require.Same(t, &cr.Status, a.GetStatus(cr))
}

// TestAgentExternalDepsFingerprint_TracksEveryScope asserts the redeploy trigger
// notices a rotation of a Secret referenced from any policy scope, or from
// upstream auth.
func TestAgentExternalDepsFingerprint_TracksEveryScope(t *testing.T) {
	sec := agentSecret()
	sec.ResourceVersion = "1"
	k8sClient := fake.NewClientBuilder().
		WithScheme(agentTestScheme(t)).
		WithObjects(sec).
		Build()

	base := agentWithPolicyScopes(t)
	base.Spec.Upstream.Auth = &apiv1.AgentUpstreamAuth{
		Type: "api-key", Header: ptr("Authorization"),
		Value: &apiv1.SecretValueSource{ValueFrom: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "agent-secrets"},
			Key:                  "upstream",
		}},
	}

	fp, err := agentExternalDepsFingerprint(context.Background(), k8sClient, base)
	require.NoError(t, err)
	require.Contains(t, fp, "secret:apigateway-demo/agent-secrets@1")

	// An Agent referencing nothing external has no fingerprint, so the drift
	// check stays inert rather than redeploying on every reconcile.
	plain := agentWithPolicyScopes(t)
	plain.Spec.A2A.OperationConfigs.Policies = nil
	plain.Spec.A2A.OperationConfigs.Operations = nil
	plain.Spec.A2A.AgentCard.Public.Policies = nil
	empty, err := agentExternalDepsFingerprint(context.Background(), k8sClient, plain)
	require.NoError(t, err)
	require.Empty(t, empty)

	needs, err := (&agentAdapter{}).needsRedeployForExternalDeps(context.Background(), k8sClient, plain)
	require.NoError(t, err)
	require.False(t, needs)
}

// TestAgentNeedsRedeployForExternalDeps asserts drift is detected against the
// annotation written at the previous deploy.
func TestAgentNeedsRedeployForExternalDeps(t *testing.T) {
	sec := agentSecret()
	sec.ResourceVersion = "7"
	k8sClient := fake.NewClientBuilder().
		WithScheme(agentTestScheme(t)).
		WithObjects(sec).
		Build()
	cr := agentWithPolicyScopes(t)

	a := &agentAdapter{}
	needs, err := a.needsRedeployForExternalDeps(context.Background(), k8sClient, cr)
	require.NoError(t, err)
	require.True(t, needs, "no annotation yet, so the deployed state is unknown")

	fp, err := agentExternalDepsFingerprint(context.Background(), k8sClient, cr)
	require.NoError(t, err)
	cr.Annotations = map[string]string{annAgentPolicyValueFromFingerprint: fp}

	needs, err = a.needsRedeployForExternalDeps(context.Background(), k8sClient, cr)
	require.NoError(t, err)
	require.False(t, needs, "annotation matches the current backing state")
}

// TestAgentReferencesSecret asserts the watch predicate fires for a Secret
// referenced from any policy scope or from upstream auth, and does not fire for
// an unrelated Secret or a Secret in another namespace.
func TestAgentReferencesSecret(t *testing.T) {
	cr := agentWithPolicyScopes(t)

	require.True(t, agentReferencesSecret(cr, "apigateway-demo", "agent-secrets"))
	require.False(t, agentReferencesSecret(cr, "apigateway-demo", "other-secrets"))
	require.False(t, agentReferencesSecret(cr, "other-namespace", "agent-secrets"))

	// Upstream auth alone is enough, with no policies at all.
	upstreamOnly := agentWithPolicyScopes(t)
	upstreamOnly.Spec.A2A.OperationConfigs.Policies = nil
	upstreamOnly.Spec.A2A.OperationConfigs.Operations = nil
	upstreamOnly.Spec.A2A.AgentCard.Public.Policies = nil
	require.False(t, agentReferencesSecret(upstreamOnly, "apigateway-demo", "agent-secrets"))

	upstreamOnly.Spec.Upstream.Auth = &apiv1.AgentUpstreamAuth{
		Type: "api-key", Header: ptr("Authorization"),
		Value: &apiv1.SecretValueSource{ValueFrom: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "agent-secrets"},
			Key:                  "upstream",
		}},
	}
	require.True(t, agentReferencesSecret(upstreamOnly, "apigateway-demo", "agent-secrets"))

	// A "none" auth block carries no value and must not panic or match.
	noneAuth := agentWithPolicyScopes(t)
	noneAuth.Spec.A2A.OperationConfigs.Policies = nil
	noneAuth.Spec.A2A.OperationConfigs.Operations = nil
	noneAuth.Spec.A2A.AgentCard.Public.Policies = nil
	noneAuth.Spec.Upstream.Auth = &apiv1.AgentUpstreamAuth{Type: "none"}
	require.False(t, agentReferencesSecret(noneAuth, "apigateway-demo", "agent-secrets"))
}

// TestAgentEnqueueForSecretAndConfigMap asserts an Agent is enqueued when a
// Secret it references changes, and that a ConfigMap-backed param is picked up
// by the ConfigMap watch.
func TestAgentEnqueueForSecretAndConfigMap(t *testing.T) {
	scheme := agentTestScheme(t)
	cr := agentWithPolicyScopes(t)

	cmParams, err := json.Marshal(map[string]any{
		"key": map[string]any{"valueFrom": map[string]any{
			"configMapKeyRef": map[string]any{"name": "agent-config", "key": "header"},
		}},
	})
	require.NoError(t, err)
	cr.Spec.A2A.AgentCard.Public.Policies[0].Params = &runtime.RawExtension{Raw: cmParams}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr).Build()

	secReqs := enqueueAgentsForSecret(k8sClient)(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apigateway-demo", Name: "agent-secrets"},
	})
	require.Len(t, secReqs, 1)
	require.Equal(t, "weather-agent", secReqs[0].Name)

	cmReqs := enqueueAgentsForConfigMap(k8sClient)(context.Background(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apigateway-demo", Name: "agent-config"},
	})
	require.Len(t, cmReqs, 1)
	require.Equal(t, "weather-agent", cmReqs[0].Name)

	none := enqueueAgentsForSecret(k8sClient)(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apigateway-demo", Name: "unrelated"},
	})
	require.Empty(t, none)
}

// TestAgentListHelpersWired asserts AgentList is handled by the generic
// APIGateway-event mapper. Both switches default to nil/empty for an unknown
// list type, so a missing case is a silent no-op: an Agent would simply never
// redeploy when its gateway becomes ready.
func TestAgentListHelpersWired(t *testing.T) {
	require.IsType(t, &apiv1.AgentList{}, newObjectListSameType(&apiv1.AgentList{}))

	list := &apiv1.AgentList{Items: []apiv1.Agent{
		{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns"}},
	}}
	items := extractItems(list)
	require.Len(t, items, 2)
	require.Equal(t, "a", items[0].GetName())
	require.Equal(t, "b", items[1].GetName())
}

// deployBody returns the body of the write request (POST/PUT) the fake
// management API saw.
func deployBody(t *testing.T, requests []capturedRequest) string {
	t.Helper()
	for _, r := range requests {
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			return r.Body
		}
	}
	t.Fatal("no deploy request reached the management API")
	return ""
}

// deploySpec parses the spec out of the deployed envelope.
func deploySpec(t *testing.T, requests []capturedRequest) map[string]interface{} {
	t.Helper()
	var envelope map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(deployBody(t, requests)), &envelope))
	spec, ok := envelope["spec"].(map[string]interface{})
	require.True(t, ok, "envelope has no spec object: %v", envelope)
	return spec
}

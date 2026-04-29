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
	"fmt"

	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/secretsource"
)

const llmProviderFinalizer = "gateway.api-platform.wso2.com/llmprovider-finalizer"

// LlmProviderReconciler reconciles LlmProvider CRs against the gateway-
// controller management API at /llm-providers.
type LlmProviderReconciler struct {
	GenericReconciler
}

// NewLlmProviderReconciler constructs a fully wired LlmProvider reconciler.
func NewLlmProviderReconciler(c client.Client, cfg *config.OperatorConfig, logger *zap.Logger, tracker *ResourceTracker) *LlmProviderReconciler {
	r := &LlmProviderReconciler{}
	r.Client = c
	r.Config = cfg
	r.Logger = logger
	r.Tracker = tracker
	r.Adapter = &llmProviderAdapter{client: c}
	return r
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=llmproviders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=llmproviders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=llmproviders/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=apigateways,verbs=get;list;watch

// SetupWithManager registers the controller with mgr.
func (r *LlmProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles(r.Config.Reconciliation.MaxConcurrentReconciles)}
	return ctrl.NewControllerManagedBy(mgr).
		Named("llmprovider").
		WithOptions(opts).
		For(&apiv1.LlmProvider{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(enqueueAllOfKind(r.Client, &apiv1.LlmProviderList{})),
			builder.WithPredicates(gatewayWatchPredicate())).
		Watches(&apiv1.LlmProviderTemplate{},
			handler.EnqueueRequestsFromMapFunc(enqueueLlmProvidersReferencingTemplate(r.Client))).
		Complete(r)
}

// llmProviderAdapter implements ResourceAdapter for LlmProvider.
type llmProviderAdapter struct {
	client client.Client
}

func (a *llmProviderAdapter) Kind() string             { return "LlmProvider" }
func (a *llmProviderAdapter) FinalizerName() string    { return llmProviderFinalizer }
func (a *llmProviderAdapter) NewObject() client.Object { return &apiv1.LlmProvider{} }
func (a *llmProviderAdapter) IsUUIDKeyed() bool        { return false }

func (a *llmProviderAdapter) Handle(obj client.Object) string {
	return obj.GetName()
}

func (a *llmProviderAdapter) GetStatus(obj client.Object) *apiv1.ResourceStatus {
	return &obj.(*apiv1.LlmProvider).Status
}

func (a *llmProviderAdapter) SetStatusId(obj client.Object, id string) {
	obj.(*apiv1.LlmProvider).Status.Id = id
}

func (a *llmProviderAdapter) GatewaySelectionKey(obj client.Object) (string, map[string]string) {
	cr := obj.(*apiv1.LlmProvider)
	return cr.Namespace, cr.Labels
}

func (a *llmProviderAdapter) Deploy(ctx context.Context, k8sClient client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*apiv1.LlmProvider)

	specPayload := interface{}(cr.Spec)
	if cr.Spec.Upstream.Auth != nil {
		resolved, err := resolveLLMProviderUpstreamAuth(ctx, k8sClient, cr.Namespace, cr.Spec.Upstream.Auth, "spec.upstream.auth.value")
		if err != nil {
			return DeployResult{}, err
		}
		if resolved == nil {
			return DeployResult{}, &gatewayclient.NonRetryableError{Err: fmt.Errorf("internal: upstream.auth resolution produced nil")}
		}
		m, err := specToJSONMap(cr.Spec)
		if err != nil {
			return DeployResult{}, &gatewayclient.NonRetryableError{Err: err}
		}
		if err := flattenUpstreamAuthCredentialValue(m, "upstream", *resolved); err != nil {
			return DeployResult{}, &gatewayclient.NonRetryableError{Err: err}
		}
		specPayload = m
	}

	body, err := gatewayclient.BuildEnvelopeYAML(apiv1.GroupVersion.String(), "LlmProvider",
		gatewayclient.EnvelopeMetadata{
			Name:        cr.Name,
			Labels:      cr.Labels,
			Annotations: cr.Annotations,
		}, specPayload)
	if err != nil {
		return DeployResult{}, &gatewayclient.NonRetryableError{Err: fmt.Errorf("build payload: %w", err)}
	}
	if err := deployEnvelopeResource(ctx, gatewayEndpoint, gatewayclient.LLMProvidersPath(), cr.Name, body, authFn); err != nil {
		return DeployResult{}, rewriteLLMDeployDependencyErrors(err)
	}
	return DeployResult{}, nil
}

func (a *llmProviderAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*apiv1.LlmProvider)
	return gatewayclient.DeleteResource(ctx, gatewayEndpoint, gatewayclient.LLMProvidersPath(), cr.Name, authFn)
}

// resolveLLMProviderUpstreamAuth resolves an optional LLMUpstreamAuth.Value
// against a Secret, returning a pointer to the plaintext if the auth block
// is set. Returns nil when auth is nil (controller leaves spec unchanged).
// Resolve errors are classified with classifySecretSourceResolveError (Kubernetes
// apierrors + secretsource.Err*) so terminal config/Secret issues become
// gatewayclient.NonRetryableError and transient API reads propagate for retry.
func resolveLLMProviderUpstreamAuth(ctx context.Context, k8sClient client.Client, namespace string, auth *apiv1.LLMUpstreamAuth, fieldPath string) (*string, error) {
	if auth == nil {
		return nil, nil
	}
	val, err := secretsource.Resolve(ctx, k8sClient, fieldPath, auth.Value, namespace)
	if err != nil {
		return nil, classifySecretSourceResolveError(err)
	}
	return &val, nil
}

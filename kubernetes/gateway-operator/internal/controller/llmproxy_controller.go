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
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

const llmProxyFinalizer = "gateway.api-platform.wso2.com/llmproxy-finalizer"

// LlmProxyReconciler reconciles LlmProxy CRs against the gateway-controller
// management API at /llm-proxies.
type LlmProxyReconciler struct {
	GenericReconciler
}

// NewLlmProxyReconciler constructs a fully wired LlmProxy reconciler.
func NewLlmProxyReconciler(c client.Client, cfg *config.OperatorConfig, logger *zap.Logger, tracker *ResourceTracker) *LlmProxyReconciler {
	r := &LlmProxyReconciler{}
	r.Client = c
	r.Config = cfg
	r.Logger = logger
	r.Tracker = tracker
	r.Adapter = &llmProxyAdapter{}
	return r
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=llmproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=llmproxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=llmproxies/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// SetupWithManager registers the controller with mgr.
func (r *LlmProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles(r.Config.Reconciliation.MaxConcurrentReconciles)}
	return ctrl.NewControllerManagedBy(mgr).
		Named("llmproxy").
		WithOptions(opts).
		For(&apiv1.LlmProxy{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(enqueueAllOfKind(r.Client, &apiv1.LlmProxyList{})),
			builder.WithPredicates(gatewayWatchPredicate())).
		Watches(&apiv1.LlmProvider{},
			handler.EnqueueRequestsFromMapFunc(enqueueLlmProxiesReferencingProvider(r.Client))).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(enqueueLlmProxiesForSecret(r.Client)),
			builder.WithPredicates(secretMutationPredicate())).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(enqueueLlmProxiesForConfigMap(r.Client)),
			builder.WithPredicates(configMapMutationPredicate())).
		Complete(r)
}

type llmProxyAdapter struct{}

func (a *llmProxyAdapter) Kind() string             { return "LlmProxy" }
func (a *llmProxyAdapter) FinalizerName() string    { return llmProxyFinalizer }
func (a *llmProxyAdapter) NewObject() client.Object { return &apiv1.LlmProxy{} }
func (a *llmProxyAdapter) IsUUIDKeyed() bool        { return false }

func (a *llmProxyAdapter) Handle(obj client.Object) string {
	return obj.GetName()
}

func (a *llmProxyAdapter) GetStatus(obj client.Object) *apiv1.ResourceStatus {
	return &obj.(*apiv1.LlmProxy).Status
}

func (a *llmProxyAdapter) SetStatusId(obj client.Object, id string) {
	obj.(*apiv1.LlmProxy).Status.Id = id
}

func (a *llmProxyAdapter) GatewaySelectionKey(obj client.Object) (string, map[string]string) {
	cr := obj.(*apiv1.LlmProxy)
	return cr.Namespace, cr.Labels
}

func (a *llmProxyAdapter) needsRedeployForExternalDeps(ctx context.Context, c client.Client, obj client.Object) (bool, error) {
	cr := obj.(*apiv1.LlmProxy)
	fp, err := llmProxyExternalDepsFingerprint(ctx, c, cr)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(fp) == "" {
		return false, nil
	}
	cur := ""
	if obj.GetAnnotations() != nil {
		cur = strings.TrimSpace(obj.GetAnnotations()[annLlmProxyPolicyValueFromFingerprint])
	}
	return cur != fp, nil
}

func (a *llmProxyAdapter) onExternalDepsApplied(ctx context.Context, c client.Client, obj client.Object, extDepsFingerprint string) error {
	if strings.TrimSpace(extDepsFingerprint) == "" {
		return nil
	}
	var latest apiv1.LlmProxy
	if err := c.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}, &latest); err != nil {
		return err
	}
	base := latest.DeepCopy()
	ann := latest.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	} else {
		cp := make(map[string]string, len(ann)+1)
		for k, v := range ann {
			cp[k] = v
		}
		ann = cp
	}
	ann[annLlmProxyPolicyValueFromFingerprint] = extDepsFingerprint
	latest.SetAnnotations(ann)
	return c.Patch(ctx, &latest, client.MergeFrom(base))
}

func (a *llmProxyAdapter) Deploy(ctx context.Context, k8sClient client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*apiv1.LlmProxy)

	spec := *cr.Spec.DeepCopy()
	for i := range spec.Policies {
		for j := range spec.Policies[i].Paths {
			scope := fmt.Sprintf("policy %s %s path %s", spec.Policies[i].Name, spec.Policies[i].Version, spec.Policies[i].Paths[j].Path)
			if err := resolveRawExtensionValueFrom(ctx, k8sClient, cr.Namespace, spec.Policies[i].Paths[j].Params, spec.Policies[i].Name, scope); err != nil {
				return DeployResult{}, err
			}
		}
	}

	specPayload := interface{}(spec)
	if spec.Provider.Auth != nil {
		resolved, err := resolveLLMProviderUpstreamAuth(ctx, k8sClient, cr.Namespace, spec.Provider.Auth, "spec.provider.auth.value")
		if err != nil {
			return DeployResult{}, err
		}
		if resolved == nil {
			return DeployResult{}, &gatewayclient.NonRetryableError{Err: fmt.Errorf("internal: provider.auth resolution produced nil")}
		}
		m, err := specToJSONMap(spec)
		if err != nil {
			return DeployResult{}, &gatewayclient.NonRetryableError{Err: err}
		}
		if err := flattenUpstreamAuthCredentialValue(m, "provider", *resolved); err != nil {
			return DeployResult{}, &gatewayclient.NonRetryableError{Err: err}
		}
		specPayload = m
	}

	body, err := gatewayclient.BuildEnvelopeYAML(gatewayclient.ManagementArtifactAPIVersion, "LlmProxy",
		gatewayclient.EnvelopeMetadata{
			Name:        cr.Name,
			Labels:      cr.Labels,
			Annotations: cr.Annotations,
		}, specPayload)
	if err != nil {
		return DeployResult{}, &gatewayclient.NonRetryableError{Err: fmt.Errorf("build payload: %w", err)}
	}
	if err := deployEnvelopeResource(ctx, gatewayEndpoint, gatewayclient.LLMProxiesPath(), cr.Name, body, authFn); err != nil {
		return DeployResult{}, rewriteLLMDeployDependencyErrors(err)
	}
	// Compute the fingerprint of the deployed state so it can be written to the
	// annotation by onExternalDepsApplied without a racy re-computation.
	fp, err := llmProxyExternalDepsFingerprint(ctx, k8sClient, cr)
	if err != nil {
		return DeployResult{}, err
	}
	return DeployResult{Fingerprint: fp}, nil
}

func (a *llmProxyAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*apiv1.LlmProxy)
	return gatewayclient.DeleteResource(ctx, gatewayEndpoint, gatewayclient.LLMProxiesPath(), cr.Name, authFn)
}

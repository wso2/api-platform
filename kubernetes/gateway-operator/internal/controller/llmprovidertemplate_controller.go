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

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

const llmProviderTemplateFinalizer = "gateway.api-platform.wso2.com/llmprovidertemplate-finalizer"

// LlmProviderTemplateReconciler reconciles LlmProviderTemplate CRs against
// the gateway-controller management API at /llm-provider-templates.
type LlmProviderTemplateReconciler struct {
	GenericReconciler
}

// NewLlmProviderTemplateReconciler constructs a fully wired reconciler.
func NewLlmProviderTemplateReconciler(c client.Client, cfg *config.OperatorConfig, logger *zap.Logger, tracker *ResourceTracker) *LlmProviderTemplateReconciler {
	r := &LlmProviderTemplateReconciler{}
	r.Client = c
	r.Config = cfg
	r.Logger = logger
	r.Tracker = tracker
	r.Adapter = &llmProviderTemplateAdapter{}
	return r
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=llmprovidertemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=llmprovidertemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=llmprovidertemplates/finalizers,verbs=update

// SetupWithManager registers the controller with mgr.
func (r *LlmProviderTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles(r.Config.Reconciliation.MaxConcurrentReconciles)}
	return ctrl.NewControllerManagedBy(mgr).
		Named("llmprovidertemplate").
		WithOptions(opts).
		For(&apiv1.LlmProviderTemplate{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(enqueueAllOfKind(r.Client, &apiv1.LlmProviderTemplateList{})),
			builder.WithPredicates(gatewayWatchPredicate())).
		Complete(r)
}

type llmProviderTemplateAdapter struct{}

func (a *llmProviderTemplateAdapter) Kind() string             { return "LlmProviderTemplate" }
func (a *llmProviderTemplateAdapter) FinalizerName() string    { return llmProviderTemplateFinalizer }
func (a *llmProviderTemplateAdapter) NewObject() client.Object { return &apiv1.LlmProviderTemplate{} }
func (a *llmProviderTemplateAdapter) IsUUIDKeyed() bool        { return false }

func (a *llmProviderTemplateAdapter) Handle(obj client.Object) string {
	return obj.GetName()
}

func (a *llmProviderTemplateAdapter) GetStatus(obj client.Object) *apiv1.ResourceStatus {
	return &obj.(*apiv1.LlmProviderTemplate).Status
}

func (a *llmProviderTemplateAdapter) SetStatusId(obj client.Object, id string) {
	obj.(*apiv1.LlmProviderTemplate).Status.Id = id
}

func (a *llmProviderTemplateAdapter) GatewaySelectionKey(obj client.Object) (string, map[string]string) {
	cr := obj.(*apiv1.LlmProviderTemplate)
	return cr.Namespace, cr.Labels
}

func (a *llmProviderTemplateAdapter) Deploy(ctx context.Context, _ client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*apiv1.LlmProviderTemplate)
	body, err := gatewayclient.BuildEnvelopeYAML(gatewayclient.ManagementArtifactAPIVersion, "LlmProviderTemplate",
		gatewayclient.EnvelopeMetadata{
			Name:        cr.Name,
			Labels:      cr.Labels,
			Annotations: cr.Annotations,
		}, cr.Spec)
	if err != nil {
		return DeployResult{}, &gatewayclient.NonRetryableError{Err: fmt.Errorf("build payload: %w", err)}
	}
	if err := deployEnvelopeResource(ctx, gatewayEndpoint, gatewayclient.LLMProviderTemplatesPath(), cr.Name, body, authFn); err != nil {
		return DeployResult{}, err
	}
	return DeployResult{}, nil
}

func (a *llmProviderTemplateAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*apiv1.LlmProviderTemplate)
	return gatewayclient.DeleteResource(ctx, gatewayEndpoint, gatewayclient.LLMProviderTemplatesPath(), cr.Name, authFn)
}

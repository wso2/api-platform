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

const mcpFinalizer = "gateway.api-platform.wso2.com/mcp-finalizer"

// McpReconciler reconciles Mcp CRs against the gateway-controller
// management API at /mcp-proxies.
type McpReconciler struct {
	GenericReconciler
}

// NewMcpReconciler constructs a fully wired Mcp reconciler.
func NewMcpReconciler(c client.Client, cfg *config.OperatorConfig, logger *zap.Logger, tracker *ResourceTracker) *McpReconciler {
	r := &McpReconciler{}
	r.Client = c
	r.Config = cfg
	r.Logger = logger
	r.Tracker = tracker
	r.Adapter = &mcpAdapter{}
	return r
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=mcps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=mcps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=mcps/finalizers,verbs=update

// SetupWithManager registers the controller with mgr.
func (r *McpReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles(r.Config.Reconciliation.MaxConcurrentReconciles)}
	return ctrl.NewControllerManagedBy(mgr).
		Named("mcp").
		WithOptions(opts).
		For(&apiv1.Mcp{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(enqueueAllOfKind(r.Client, &apiv1.McpList{})),
			builder.WithPredicates(gatewayWatchPredicate())).
		Complete(r)
}

type mcpAdapter struct{}

func (a *mcpAdapter) Kind() string             { return "Mcp" }
func (a *mcpAdapter) FinalizerName() string    { return mcpFinalizer }
func (a *mcpAdapter) NewObject() client.Object { return &apiv1.Mcp{} }
func (a *mcpAdapter) IsUUIDKeyed() bool        { return false }

func (a *mcpAdapter) Handle(obj client.Object) string {
	return obj.GetName()
}

func (a *mcpAdapter) GetStatus(obj client.Object) *apiv1.ResourceStatus {
	return &obj.(*apiv1.Mcp).Status
}

func (a *mcpAdapter) SetStatusId(obj client.Object, id string) {
	obj.(*apiv1.Mcp).Status.Id = id
}

func (a *mcpAdapter) GatewaySelectionKey(obj client.Object) (string, map[string]string) {
	cr := obj.(*apiv1.Mcp)
	return cr.Namespace, cr.Labels
}

func (a *mcpAdapter) Deploy(ctx context.Context, k8sClient client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*apiv1.Mcp)

	specPayload := interface{}(cr.Spec)
	if cr.Spec.Upstream.Auth != nil {
		val, err := secretsource.Resolve(ctx, k8sClient, "spec.upstream.auth.value", cr.Spec.Upstream.Auth.Value, cr.Namespace)
		if err != nil {
			return DeployResult{}, classifySecretSourceResolveError(err)
		}
		m, err := specToJSONMap(cr.Spec)
		if err != nil {
			return DeployResult{}, &gatewayclient.NonRetryableError{Err: err}
		}
		if err := flattenUpstreamAuthCredentialValue(m, "upstream", val); err != nil {
			return DeployResult{}, &gatewayclient.NonRetryableError{Err: err}
		}
		specPayload = m
	}

	body, err := gatewayclient.BuildEnvelopeYAML(apiv1.GroupVersion.String(), "Mcp",
		gatewayclient.EnvelopeMetadata{
			Name:        cr.Name,
			Labels:      cr.Labels,
			Annotations: cr.Annotations,
		}, specPayload)
	if err != nil {
		return DeployResult{}, &gatewayclient.NonRetryableError{Err: fmt.Errorf("build payload: %w", err)}
	}
	if err := deployEnvelopeResource(ctx, gatewayEndpoint, gatewayclient.MCPProxiesPath(), cr.Name, body, authFn); err != nil {
		return DeployResult{}, err
	}
	return DeployResult{}, nil
}

func (a *mcpAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*apiv1.Mcp)
	return gatewayclient.DeleteResource(ctx, gatewayEndpoint, gatewayclient.MCPProxiesPath(), cr.Name, authFn)
}

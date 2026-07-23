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

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
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
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

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
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(enqueueMcpsForSecret(r.Client)),
			builder.WithPredicates(secretMutationPredicate())).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(enqueueMcpsForConfigMap(r.Client)),
			builder.WithPredicates(configMapMutationPredicate())).
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

func (a *mcpAdapter) needsRedeployForExternalDeps(ctx context.Context, c client.Client, obj client.Object) (bool, error) {
	cr := obj.(*apiv1.Mcp)
	fp, err := mcpExternalDepsFingerprint(ctx, c, cr)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(fp) == "" {
		return false, nil
	}
	cur := ""
	if obj.GetAnnotations() != nil {
		cur = strings.TrimSpace(obj.GetAnnotations()[annMcpPolicyValueFromFingerprint])
	}
	return cur != fp, nil
}

func (a *mcpAdapter) onExternalDepsApplied(ctx context.Context, c client.Client, obj client.Object, fingerprint string) error {
	if strings.TrimSpace(fingerprint) == "" {
		return nil
	}
	var latest apiv1.Mcp
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
	ann[annMcpPolicyValueFromFingerprint] = fingerprint
	latest.SetAnnotations(ann)
	return c.Patch(ctx, &latest, client.MergeFrom(base))
}

func (a *mcpAdapter) Deploy(ctx context.Context, k8sClient client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*apiv1.Mcp)

	spec := *cr.Spec.DeepCopy()
	if len(spec.Policies) > 0 {
		apiSpec := &apiv1.APIConfigData{Policies: spec.Policies}
		if _, err := resolveAPIConfigPolicyParamsValueFrom(ctx, k8sClient, cr.Namespace, apiSpec, nil); err != nil {
			return DeployResult{}, err
		}
		spec.Policies = apiSpec.Policies
	}

	specPayload := interface{}(spec)
	if spec.Upstream.Auth != nil {
		val, err := secretsource.Resolve(ctx, k8sClient, "spec.upstream.auth.value", spec.Upstream.Auth.Value, cr.Namespace)
		if err != nil {
			return DeployResult{}, classifySecretSourceResolveError(err)
		}
		m, err := specToJSONMap(spec)
		if err != nil {
			return DeployResult{}, &gatewayclient.NonRetryableError{Err: err}
		}
		if err := flattenUpstreamAuthCredentialValue(m, "upstream", val); err != nil {
			return DeployResult{}, &gatewayclient.NonRetryableError{Err: err}
		}
		specPayload = m
	}

	body, err := gatewayclient.BuildEnvelopeYAML(gatewayclient.ManagementArtifactAPIVersion, "Mcp",
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
	// Compute the fingerprint of the deployed state so it can be written to the
	// annotation by onExternalDepsApplied without a racy re-computation.
	fp, err := mcpExternalDepsFingerprint(ctx, k8sClient, cr)
	if err != nil {
		return DeployResult{}, err
	}
	return DeployResult{Fingerprint: fp}, nil
}

func (a *mcpAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*apiv1.Mcp)
	return gatewayclient.DeleteResource(ctx, gatewayEndpoint, gatewayclient.MCPProxiesPath(), cr.Name, authFn)
}

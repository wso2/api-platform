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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"log/slog"
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

const agentFinalizer = "gateway.api-platform.wso2.com/agent-finalizer"

// AgentReconciler reconciles Agent CRs against the gateway-controller
// management API at /agents.
type AgentReconciler struct {
	GenericReconciler
}

// NewAgentReconciler constructs a fully wired Agent reconciler.
func NewAgentReconciler(c client.Client, cfg *config.OperatorConfig, logger *slog.Logger, tracker *ResourceTracker) *AgentReconciler {
	r := &AgentReconciler{}
	r.Client = c
	r.Config = cfg
	r.Logger = logger
	r.Tracker = tracker
	r.Adapter = &agentAdapter{}
	return r
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=agents,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=agents/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=agents/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// SetupWithManager registers the controller with mgr.
func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles(r.Config.Reconciliation.MaxConcurrentReconciles)}
	return ctrl.NewControllerManagedBy(mgr).
		Named("agent").
		WithOptions(opts).
		For(&apiv1.Agent{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(enqueueAllOfKind(r.Client, &apiv1.AgentList{})),
			builder.WithPredicates(gatewayWatchPredicate())).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(enqueueAgentsForSecret(r.Client)),
			builder.WithPredicates(secretMutationPredicate())).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(enqueueAgentsForConfigMap(r.Client)),
			builder.WithPredicates(configMapMutationPredicate())).
		Complete(r)
}

type agentAdapter struct{}

func (a *agentAdapter) Kind() string             { return "Agent" }
func (a *agentAdapter) FinalizerName() string    { return agentFinalizer }
func (a *agentAdapter) NewObject() client.Object { return &apiv1.Agent{} }
func (a *agentAdapter) IsUUIDKeyed() bool        { return false }

func (a *agentAdapter) Handle(obj client.Object) string {
	return obj.GetName()
}

func (a *agentAdapter) GetStatus(obj client.Object) *apiv1.ResourceStatus {
	return &obj.(*apiv1.Agent).Status
}

func (a *agentAdapter) SetStatusId(obj client.Object, id string) {
	obj.(*apiv1.Agent).Status.Id = id
}

func (a *agentAdapter) GatewaySelectionKey(obj client.Object) (string, map[string]string) {
	cr := obj.(*apiv1.Agent)
	return cr.Namespace, cr.Labels
}

func (a *agentAdapter) needsRedeployForExternalDeps(ctx context.Context, c client.Client, obj client.Object) (bool, error) {
	cr := obj.(*apiv1.Agent)
	fp, err := agentExternalDepsFingerprint(ctx, c, cr)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(fp) == "" {
		return false, nil
	}
	cur := ""
	if obj.GetAnnotations() != nil {
		cur = strings.TrimSpace(obj.GetAnnotations()[annAgentPolicyValueFromFingerprint])
	}
	return cur != fp, nil
}

func (a *agentAdapter) onExternalDepsApplied(ctx context.Context, c client.Client, obj client.Object, fingerprint string) error {
	if strings.TrimSpace(fingerprint) == "" {
		return nil
	}
	var latest apiv1.Agent
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
	ann[annAgentPolicyValueFromFingerprint] = fingerprint
	latest.SetAnnotations(ann)
	return c.Patch(ctx, &latest, client.MergeFrom(base))
}

func (a *agentAdapter) Deploy(ctx context.Context, k8sClient client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*apiv1.Agent)

	// Fingerprint the external dependencies before anything below reads them.
	// This covers the same two sources the resolution steps consume — the
	// upstream auth credential and every policy-params valueFrom ref — so
	// computing it first means the annotation onExternalDepsApplied writes can
	// only ever name a revision at or older than what was deployed. Computing it
	// after the reads inverts that: a concurrent Secret/ConfigMap write landing
	// between a read and the fingerprint would record a revision that was never
	// deployed, and needsRedeployForExternalDeps would then match it and skip the
	// redeploy, losing the update. The worst case in this order is a redundant
	// redeploy, which converges. A failure here aborts before anything is
	// resolved or deployed.
	fp, err := agentExternalDepsFingerprint(ctx, k8sClient, cr)
	if err != nil {
		return DeployResult{}, err
	}

	// Resolve against a copy: the resolved plaintext must reach the gateway
	// payload only, never be written back onto the CR.
	spec := *cr.Spec.DeepCopy()
	for _, ref := range agentPolicyRefs(&spec) {
		if err := resolvePolicyParamsValueFrom(ctx, k8sClient, cr.Namespace, ref.Policy, ref.Scope, nil); err != nil {
			return DeployResult{}, err
		}
	}

	specPayload := interface{}(spec)
	if spec.Upstream.Auth != nil && spec.Upstream.Auth.Value != nil {
		val, err := secretsource.Resolve(ctx, k8sClient, "spec.upstream.auth.value", *spec.Upstream.Auth.Value, cr.Namespace)
		if err != nil {
			return DeployResult{}, classifySecretSourceResolveError(err)
		}
		m, err := specToJSONMap(spec)
		if err != nil {
			return DeployResult{}, &gatewayclient.NonRetryableError{Err: err}
		}
		// The management API takes upstream.auth.value as a plain string; the CR
		// carries the richer SecretValueSource shape, so it is flattened here.
		if err := flattenUpstreamAuthCredentialValue(m, "upstream", val); err != nil {
			return DeployResult{}, &gatewayclient.NonRetryableError{Err: err}
		}
		specPayload = m
	}

	body, err := gatewayclient.BuildEnvelopeYAML(gatewayclient.ManagementArtifactAPIVersion, "Agent",
		gatewayclient.EnvelopeMetadata{
			Name:        cr.Name,
			Labels:      cr.Labels,
			Annotations: cr.Annotations,
		}, specPayload)
	if err != nil {
		return DeployResult{}, &gatewayclient.NonRetryableError{Err: fmt.Errorf("build payload: %w", err)}
	}
	if err := deployEnvelopeResource(ctx, gatewayEndpoint, gatewayclient.AgentsPath(), cr.Name, body, authFn); err != nil {
		return DeployResult{}, err
	}
	return DeployResult{Fingerprint: fp}, nil
}

func (a *agentAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*apiv1.Agent)
	return gatewayclient.DeleteResource(ctx, gatewayEndpoint, gatewayclient.AgentsPath(), cr.Name, authFn)
}

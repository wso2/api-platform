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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	choreolabsv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/choreolabs/v1"
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

const (
	backendFinalizer = "gateway.choreolabs.com/backend-finalizer"
	// annBackendLabelsFingerprint records the labels of the Backend's last successful deploy.
	// A label change does not bump metadata.generation, but a Backend's labels choose its
	// gateways and are what a dynamic-endpoint backendSelector matches, so it must redeploy.
	annBackendLabelsFingerprint = "gateway.choreolabs.com/last-applied-labels-fingerprint"
)

var _ externalDepsDrifter = (*backendAdapter)(nil)

// BackendReconciler reconciles Backend CRs (gateway.choreolabs.com/v1) against the
// gateway-controller management API at /backends.
//
// Deleting a Backend that a deployed API names as its fixed target is refused by the gateway
// with 409. The finalizer then stays and the delete is retried with backoff, so the CR remains
// terminating until no API pins the Backend, as the gateway's own guard requires.
type BackendReconciler struct {
	GenericReconciler
}

// NewBackendReconciler constructs a fully wired reconciler.
func NewBackendReconciler(c client.Client, cfg *config.OperatorConfig, logger *slog.Logger, tracker *ResourceTracker) *BackendReconciler {
	r := &BackendReconciler{}
	r.Client = c
	r.Config = cfg
	r.Logger = logger
	r.Tracker = tracker
	r.Adapter = &backendAdapter{}
	return r
}

//+kubebuilder:rbac:groups=gateway.choreolabs.com,resources=backends,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.choreolabs.com,resources=backends/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.choreolabs.com,resources=backends/finalizers,verbs=update

// SetupWithManager registers the controller with mgr.
func (r *BackendReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles(r.Config.Reconciliation.MaxConcurrentReconciles)}
	return ctrl.NewControllerManagedBy(mgr).
		Named("backend").
		WithOptions(opts).
		For(&choreolabsv1.Backend{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(enqueueAllOfKind(r.Client, &choreolabsv1.BackendList{})),
			builder.WithPredicates(gatewayWatchPredicate())).
		Complete(r)
}

type backendAdapter struct{}

func (a *backendAdapter) Kind() string             { return "Backend" }
func (a *backendAdapter) FinalizerName() string    { return backendFinalizer }
func (a *backendAdapter) NewObject() client.Object { return &choreolabsv1.Backend{} }
func (a *backendAdapter) IsUUIDKeyed() bool        { return false }

func (a *backendAdapter) Handle(obj client.Object) string {
	return obj.GetName()
}

func (a *backendAdapter) GetStatus(obj client.Object) *apiv1.ResourceStatus {
	return &obj.(*choreolabsv1.Backend).Status
}

func (a *backendAdapter) SetStatusId(obj client.Object, id string) {
	obj.(*choreolabsv1.Backend).Status.Id = id
}

func (a *backendAdapter) GatewaySelectionKey(obj client.Object) (string, map[string]string) {
	cr := obj.(*choreolabsv1.Backend)
	return cr.Namespace, cr.Labels
}

// Deploy creates or updates the Backend on the gateway. The CR's labels travel as the Backend's
// metadata.labels, which a dynamic-endpoint backendSelector matches.
func (a *backendAdapter) Deploy(ctx context.Context, _ client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*choreolabsv1.Backend)
	body, err := gatewayclient.BuildEnvelopeYAML(gatewayclient.ChoreoLabsArtifactAPIVersion, "Backend",
		gatewayclient.EnvelopeMetadata{
			Name:        cr.Name,
			Labels:      cr.Labels,
			Annotations: cr.Annotations,
		}, cr.Spec)
	if err != nil {
		return DeployResult{}, &gatewayclient.NonRetryableError{Err: fmt.Errorf("build payload: %w", err)}
	}
	if err := deployEnvelopeResource(ctx, gatewayEndpoint, gatewayclient.BackendsPath(), cr.Name, body, authFn); err != nil {
		return DeployResult{}, err
	}
	return DeployResult{Fingerprint: backendLabelsFingerprint(cr.Labels)}, nil
}

// needsRedeployForExternalDeps reports a label change since the last successful deploy. The
// reconciler's skip check compares only metadata.generation, which labels do not bump.
func (a *backendAdapter) needsRedeployForExternalDeps(_ context.Context, _ client.Client, obj client.Object) (bool, error) {
	return obj.GetAnnotations()[annBackendLabelsFingerprint] != backendLabelsFingerprint(obj.GetLabels()), nil
}

// onExternalDepsApplied records the fingerprint of the labels Deploy sent, on the latest copy of
// the CR. A label change that lands after Deploy read the labels leaves the record stale, so the
// update it triggers redeploys.
func (a *backendAdapter) onExternalDepsApplied(ctx context.Context, c client.Client, obj client.Object, fingerprint string) error {
	if fingerprint == "" {
		return nil
	}
	var latest choreolabsv1.Backend
	if err := c.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}, &latest); err != nil {
		return err
	}
	if latest.Annotations[annBackendLabelsFingerprint] == fingerprint {
		return nil
	}
	base := latest.DeepCopy()
	annotations := make(map[string]string, len(latest.Annotations)+1)
	for key, value := range latest.Annotations {
		annotations[key] = value
	}
	annotations[annBackendLabelsFingerprint] = fingerprint
	latest.SetAnnotations(annotations)
	return c.Patch(ctx, &latest, client.MergeFrom(base))
}

// backendLabelsFingerprint is a stable digest of a Backend's labels, never empty.
func backendLabelsFingerprint(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, key := range keys {
		// Label keys and values cannot contain NUL, so the encoding is unambiguous.
		fmt.Fprintf(h, "%s\x00%s\x00", key, labels[key])
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (a *backendAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*choreolabsv1.Backend)
	return gatewayclient.DeleteResource(ctx, gatewayEndpoint, gatewayclient.BackendsPath(), cr.Name, authFn)
}

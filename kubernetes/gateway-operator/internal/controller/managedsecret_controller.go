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

const managedSecretFinalizer = "gateway.api-platform.wso2.com/managedsecret-finalizer"

// ManagedSecretReconciler reconciles ManagedSecret CRs against the
// gateway-controller management API at /secrets.
type ManagedSecretReconciler struct {
	GenericReconciler
}

// NewManagedSecretReconciler constructs a fully wired reconciler.
func NewManagedSecretReconciler(c client.Client, cfg *config.OperatorConfig, logger *zap.Logger, tracker *ResourceTracker) *ManagedSecretReconciler {
	r := &ManagedSecretReconciler{}
	r.Client = c
	r.Config = cfg
	r.Logger = logger
	r.Tracker = tracker
	r.Adapter = &managedSecretAdapter{}
	return r
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=managedsecrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=managedsecrets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=managedsecrets/finalizers,verbs=update

// SetupWithManager registers the controller with mgr.
func (r *ManagedSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles(r.Config.Reconciliation.MaxConcurrentReconciles)}
	return ctrl.NewControllerManagedBy(mgr).
		Named("managedsecret").
		WithOptions(opts).
		For(&apiv1.ManagedSecret{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(enqueueAllOfKind(r.Client, &apiv1.ManagedSecretList{})),
			builder.WithPredicates(gatewayWatchPredicate())).
		Complete(r)
}

type managedSecretAdapter struct{}

func (a *managedSecretAdapter) Kind() string             { return "ManagedSecret" }
func (a *managedSecretAdapter) FinalizerName() string    { return managedSecretFinalizer }
func (a *managedSecretAdapter) NewObject() client.Object { return &apiv1.ManagedSecret{} }
func (a *managedSecretAdapter) IsUUIDKeyed() bool        { return false }

func (a *managedSecretAdapter) Handle(obj client.Object) string {
	return obj.GetName()
}

func (a *managedSecretAdapter) GetStatus(obj client.Object) *apiv1.ResourceStatus {
	return &obj.(*apiv1.ManagedSecret).Status
}

func (a *managedSecretAdapter) SetStatusId(obj client.Object, id string) {
	obj.(*apiv1.ManagedSecret).Status.Id = id
}

func (a *managedSecretAdapter) GatewaySelectionKey(obj client.Object) (string, map[string]string) {
	cr := obj.(*apiv1.ManagedSecret)
	return cr.Namespace, cr.Labels
}

func (a *managedSecretAdapter) Deploy(ctx context.Context, k8sClient client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*apiv1.ManagedSecret)

	val, err := secretsource.Resolve(ctx, k8sClient, "spec.value", cr.Spec.Value, cr.Namespace)
	if err != nil {
		return DeployResult{}, &gatewayclient.NonRetryableError{Err: err}
	}

	// The /secrets management API expects {displayName, description, value}
	// without a SecretValueSource envelope, so we project to a plain shape
	// before serialising.
	flat := struct {
		DisplayName string  `json:"displayName" yaml:"displayName"`
		Description *string `json:"description,omitempty" yaml:"description,omitempty"`
		Value       string  `json:"value" yaml:"value"`
	}{
		DisplayName: cr.Spec.DisplayName,
		Description: cr.Spec.Description,
		Value:       val,
	}

	body, err := gatewayclient.BuildEnvelopeYAML(apiv1.GroupVersion.String(), "Secret",
		gatewayclient.EnvelopeMetadata{
			Name:        cr.Name,
			Labels:      cr.Labels,
			Annotations: cr.Annotations,
		}, flat)
	if err != nil {
		return DeployResult{}, &gatewayclient.NonRetryableError{Err: fmt.Errorf("build payload: %w", err)}
	}
	if err := deployEnvelopeResource(ctx, gatewayEndpoint, gatewayclient.SecretsPath(), cr.Name, body, authFn); err != nil {
		return DeployResult{}, err
	}
	return DeployResult{}, nil
}

func (a *managedSecretAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*apiv1.ManagedSecret)
	return gatewayclient.DeleteResource(ctx, gatewayEndpoint, gatewayclient.SecretsPath(), cr.Name, authFn)
}

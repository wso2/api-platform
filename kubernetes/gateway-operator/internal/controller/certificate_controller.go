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

	"go.uber.org/zap"
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

const certificateFinalizer = "gateway.api-platform.wso2.com/certificate-finalizer"

// CertificateReconciler reconciles Certificate CRs against the gateway-
// controller management API at /certificates. The gateway issues a UUID id
// on first POST that the controller persists to .status.id.
type CertificateReconciler struct {
	GenericReconciler
}

// NewCertificateReconciler constructs a fully wired Certificate reconciler.
func NewCertificateReconciler(c client.Client, cfg *config.OperatorConfig, logger *zap.Logger, tracker *ResourceTracker) *CertificateReconciler {
	r := &CertificateReconciler{}
	r.Client = c
	r.Config = cfg
	r.Logger = logger
	r.Tracker = tracker
	r.Adapter = &certificateAdapter{}
	return r
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=certificates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=certificates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=certificates/finalizers,verbs=update

// SetupWithManager registers the controller with mgr.
func (r *CertificateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles(r.Config.Reconciliation.MaxConcurrentReconciles)}
	return ctrl.NewControllerManagedBy(mgr).
		Named("certificate").
		WithOptions(opts).
		For(&apiv1.Certificate{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(enqueueAllOfKind(r.Client, &apiv1.CertificateList{})),
			builder.WithPredicates(gatewayWatchPredicate())).
		Complete(r)
}

type certificateAdapter struct{}

func (a *certificateAdapter) Kind() string             { return "Certificate" }
func (a *certificateAdapter) FinalizerName() string    { return certificateFinalizer }
func (a *certificateAdapter) NewObject() client.Object { return &apiv1.Certificate{} }
func (a *certificateAdapter) IsUUIDKeyed() bool        { return true }

func (a *certificateAdapter) Handle(obj client.Object) string {
	return obj.(*apiv1.Certificate).Status.Id
}

func (a *certificateAdapter) GetStatus(obj client.Object) *apiv1.ResourceStatus {
	return &obj.(*apiv1.Certificate).Status
}

func (a *certificateAdapter) SetStatusId(obj client.Object, id string) {
	obj.(*apiv1.Certificate).Status.Id = id
}

func (a *certificateAdapter) GatewaySelectionKey(obj client.Object) (string, map[string]string) {
	cr := obj.(*apiv1.Certificate)
	return cr.Namespace, cr.Labels
}

func (a *certificateAdapter) Deploy(ctx context.Context, k8sClient client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*apiv1.Certificate)
	pem, err := secretsource.Resolve(ctx, k8sClient, "spec.certificate", cr.Spec.Certificate, cr.Namespace)
	if err != nil {
		return DeployResult{}, classifySecretSourceResolveError(err)
	}

	// Certificates are immutable on the management API: when status.id is
	// already set we treat the CR as deployed and skip re-uploading. This
	// matches the OpenAPI contract (no PUT /certificates/{id}).
	if cr.Status.Id != "" {
		return DeployResult{Id: cr.Status.Id}, nil
	}

	resp, err := gatewayclient.UploadCertificate(ctx, gatewayEndpoint, gatewayclient.CertificateUploadPayload{
		Name:        cr.Spec.DisplayName,
		Certificate: pem,
	}, authFn)
	if err != nil {
		return DeployResult{}, err
	}
	return DeployResult{Id: resp.Id}, nil
}

func (a *certificateAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*apiv1.Certificate)
	if cr.Status.Id == "" {
		return nil
	}
	return gatewayclient.DeleteCertificate(ctx, gatewayEndpoint, cr.Status.Id, authFn)
}

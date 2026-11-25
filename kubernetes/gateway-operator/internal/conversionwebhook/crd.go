/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package conversionwebhook

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ConvertPath is where the webhook serves ConversionReview requests.
const ConvertPath = "/convert"

// servicePort is the port of the webhook Service the CRD points at.
const servicePort int32 = 443

// desiredConversion is the spec.conversion the CRD must carry for the API server to
// call this operator.
func desiredConversion(namespace, service string, caBundle []byte) *apiextensionsv1.CustomResourceConversion {
	path := ConvertPath
	port := servicePort
	return &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ConversionReviewVersions: []string{"v1"},
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				Service: &apiextensionsv1.ServiceReference{
					Namespace: namespace,
					Name:      service,
					Path:      &path,
					Port:      &port,
				},
				CABundle: caBundle,
			},
		},
	}
}

// crdReconciler keeps the converted CRD's spec.conversion pointed at this operator's
// webhook Service with the current CA. The CRD ships with a placeholder Service (the
// chart's crds/ directory cannot be templated), and kubectl, Helm or Flux put the
// placeholder back whenever they re-apply it, so this runs on every CRD change and
// periodically. Like the rotator it runs on every replica (see crdControllerOptions).
type crdReconciler struct {
	client    client.Client
	secrets   *secretManager
	crdName   string
	namespace string
	service   string
	resync    time.Duration
	logger    *slog.Logger
}

func (r *crdReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	bundle, _, found, err := r.secrets.read(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found || len(bundle.caCert) == 0 {
		// The rotator has not created the certificate yet.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: r.crdName}, crd); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("The CRD to convert is not installed", slog.String("crd", r.crdName))
			return ctrl.Result{RequeueAfter: r.resync}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get CRD %s: %w", r.crdName, err)
	}

	want := desiredConversion(r.namespace, r.service, bundle.caCert)
	if equality.Semantic.DeepEqual(crd.Spec.Conversion, want) {
		return ctrl.Result{RequeueAfter: r.resync}, nil
	}
	patch := client.MergeFrom(crd.DeepCopy())
	crd.Spec.Conversion = want
	if err := r.client.Patch(ctx, crd, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch CRD %s conversion: %w", r.crdName, err)
	}
	r.logger.Info("Pointed the CRD's conversion webhook at the operator",
		slog.String("crd", r.crdName),
		slog.String("service", r.namespace+"/"+r.service))
	return ctrl.Result{RequeueAfter: r.resync}, nil
}

// crdControllerOptions runs the CRD reconciler without leader election. During a
// rolling upgrade the old operator version holds the leader lease and does not know
// about the webhook, so a leader-only reconciler in the new pods would leave the CRD
// pointing at the placeholder for as long as the old pods run. Every replica patches
// to the same value (read from the shared Secret), so concurrent patches agree.
func crdControllerOptions() controller.Options {
	needLeaderElection := false
	return controller.Options{NeedLeaderElection: &needLeaderElection}
}

// setupWithManager watches only the converted CRD. The manager's cache must select
// it by name (see CacheByObject), so RBAC can be restricted to that one CRD.
func (r *crdReconciler) setupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("conversion-webhook-crd").
		WithOptions(crdControllerOptions()).
		For(&apiextensionsv1.CustomResourceDefinition{}, builder.WithPredicates(
			predicate.NewPredicateFuncs(func(obj client.Object) bool { return obj.GetName() == r.crdName }),
		)).
		Complete(r)
}

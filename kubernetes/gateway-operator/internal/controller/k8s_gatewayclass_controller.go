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

package controller

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	contctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
)

// K8sGatewayClassReconciler sets GatewayClass status (Accepted) for classes that
// point spec.controllerName at this operator.
type K8sGatewayClassReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.OperatorConfig
	Logger *zap.Logger
}

// NewK8sGatewayClassReconciler creates a reconciler for gateway.networking.k8s.io GatewayClass.
func NewK8sGatewayClassReconciler(cl client.Client, scheme *runtime.Scheme, cfg *config.OperatorConfig, logger *zap.Logger) *K8sGatewayClassReconciler {
	return &K8sGatewayClassReconciler{
		Client: cl,
		Scheme: scheme,
		Config: cfg,
		Logger: logger,
	}
}

//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gatewayclasses,verbs=get;list;watch
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gatewayclasses/status,verbs=get;update;patch

// Reconcile implements ctrl.Reconciler.
func (r *K8sGatewayClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	gc := &gatewayv1.GatewayClass{}
	if err := r.Get(ctx, req.NamespacedName, gc); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if gc.Spec.ControllerName != PlatformGatewayControllerName {
		return ctrl.Result{}, nil
	}

	log := r.Logger.With(
		zap.String("controller", "K8sGatewayClass"),
		zap.String("name", gc.Name),
		zap.Int64("generation", gc.Generation),
	)

	var cond metav1.Condition
	if r.Config.ManagedGatewayClass(gc.Name) {
		cond = metav1.Condition{
			Type:               string(gatewayv1.GatewayClassConditionStatusAccepted),
			Status:             metav1.ConditionTrue,
			Reason:             string(gatewayv1.GatewayClassReasonAccepted),
			Message:            fmt.Sprintf("GatewayClass %q is managed by this operator (listed in gateway class allowlist).", gc.Name),
			ObservedGeneration: gc.Generation,
			LastTransitionTime: metav1.Now(),
		}
		log.Debug("GatewayClass accepted: name in operator allowlist")
	} else {
		cond = metav1.Condition{
			Type:               string(gatewayv1.GatewayClassConditionStatusAccepted),
			Status:             metav1.ConditionFalse,
			Reason:             string(gatewayv1.GatewayClassReasonUnsupported),
			Message:            fmt.Sprintf("GatewayClass name %q is not in the operator gateway class allowlist (gatewayApi.managedGatewayClassNames / GATEWAY_API_GATEWAY_CLASS_NAMES).", gc.Name),
			ObservedGeneration: gc.Generation,
			LastTransitionTime: metav1.Now(),
		}
		log.Debug("GatewayClass not accepted: name not in operator allowlist")
	}

	if existing := meta.FindStatusCondition(gc.Status.Conditions, string(gatewayv1.GatewayClassConditionStatusAccepted)); gatewayClassAcceptedConditionUnchanged(existing, &cond) {
		return ctrl.Result{}, nil
	}

	latest := &gatewayv1.GatewayClass{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(gc), latest); err != nil {
		return ctrl.Result{}, err
	}
	base := latest.DeepCopy()
	meta.SetStatusCondition(&latest.Status.Conditions, cond)
	if err := r.Status().Patch(ctx, latest, client.MergeFrom(base)); err != nil {
		return ctrl.Result{}, err
	}
	log.Info("updated GatewayClass Accepted status",
		zap.String("accepted", string(cond.Status)),
		zap.String("reason", cond.Reason))
	return ctrl.Result{}, nil
}

func gatewayClassAcceptedConditionUnchanged(existing *metav1.Condition, desired *metav1.Condition) bool {
	if existing == nil {
		return false
	}
	return existing.Status == desired.Status &&
		existing.Reason == desired.Reason &&
		existing.Message == desired.Message &&
		existing.ObservedGeneration == desired.ObservedGeneration
}

// SetupWithManager registers the GatewayClass controller.
func (r *K8sGatewayClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := contctrl.Options{MaxConcurrentReconciles: r.Config.Reconciliation.MaxConcurrentReconciles}
	if opts.MaxConcurrentReconciles <= 0 {
		opts.MaxConcurrentReconciles = 1
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(opts).
		For(&gatewayv1.GatewayClass{}).
		Complete(r)
}

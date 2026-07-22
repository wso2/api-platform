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
	"time"

	"log/slog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

const subscriptionPlanFinalizer = "gateway.api-platform.wso2.com/subscriptionplan-finalizer"

// SubscriptionPlanReconciler reconciles SubscriptionPlan CRs against
// /subscription-plans. The gateway issues a UUID id on first POST that the
// controller persists to .status.id.
type SubscriptionPlanReconciler struct {
	GenericReconciler
}

// NewSubscriptionPlanReconciler constructs a fully wired reconciler.
func NewSubscriptionPlanReconciler(c client.Client, cfg *config.OperatorConfig, logger *slog.Logger, tracker *ResourceTracker) *SubscriptionPlanReconciler {
	r := &SubscriptionPlanReconciler{}
	r.Client = c
	r.Config = cfg
	r.Logger = logger
	r.Tracker = tracker
	r.Adapter = &subscriptionPlanAdapter{}
	return r
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=subscriptionplans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=subscriptionplans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=subscriptionplans/finalizers,verbs=update

// SetupWithManager registers the controller with mgr.
func (r *SubscriptionPlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles(r.Config.Reconciliation.MaxConcurrentReconciles)}
	return ctrl.NewControllerManagedBy(mgr).
		Named("subscriptionplan").
		WithOptions(opts).
		For(&apiv1.SubscriptionPlan{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(enqueueAllOfKind(r.Client, &apiv1.SubscriptionPlanList{})),
			builder.WithPredicates(gatewayWatchPredicate())).
		Complete(r)
}

type subscriptionPlanAdapter struct{}

func (a *subscriptionPlanAdapter) Kind() string             { return "SubscriptionPlan" }
func (a *subscriptionPlanAdapter) FinalizerName() string    { return subscriptionPlanFinalizer }
func (a *subscriptionPlanAdapter) NewObject() client.Object { return &apiv1.SubscriptionPlan{} }
func (a *subscriptionPlanAdapter) IsUUIDKeyed() bool        { return true }

func (a *subscriptionPlanAdapter) Handle(obj client.Object) string {
	return obj.(*apiv1.SubscriptionPlan).Status.Id
}

func (a *subscriptionPlanAdapter) GetStatus(obj client.Object) *apiv1.ResourceStatus {
	return &obj.(*apiv1.SubscriptionPlan).Status
}

func (a *subscriptionPlanAdapter) SetStatusId(obj client.Object, id string) {
	obj.(*apiv1.SubscriptionPlan).Status.Id = id
}

func (a *subscriptionPlanAdapter) GatewaySelectionKey(obj client.Object) (string, map[string]string) {
	cr := obj.(*apiv1.SubscriptionPlan)
	return cr.Namespace, cr.Labels
}

func (a *subscriptionPlanAdapter) Deploy(ctx context.Context, _ client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*apiv1.SubscriptionPlan)

	if cr.Status.Id == "" {
		// Recover when the plan exists in the gateway but status.id was never
		// persisted (optimistic-lock / ordering races). Listing by planName
		// avoids skipping forever in the "already Programmed" reconcile path.
		if recoverID, err := gatewayclient.FindSubscriptionPlanIDByPlanName(ctx, gatewayEndpoint, cr.Spec.PlanName, authFn); err != nil {
			return DeployResult{}, err
		} else if recoverID != "" {
			return DeployResult{Id: recoverID}, nil
		}

		payload := gatewayclient.SubscriptionPlanCreatePayload{
			PlanName:           cr.Spec.PlanName,
			BillingPlan:        cr.Spec.BillingPlan,
			Status:             cr.Spec.Status,
			StopOnQuotaReach:   cr.Spec.StopOnQuotaReach,
			ThrottleLimitCount: cr.Spec.ThrottleLimitCount,
			ThrottleLimitUnit:  cr.Spec.ThrottleLimitUnit,
		}
		if cr.Spec.ExpiryTime != nil {
			t := cr.Spec.ExpiryTime.Time
			payload.ExpiryTime = (*time.Time)(&t)
		}
		resp, err := gatewayclient.CreateSubscriptionPlan(ctx, gatewayEndpoint, payload, authFn)
		if err != nil {
			return DeployResult{}, err
		}
		return DeployResult{Id: resp.Id}, nil
	}

	updatePayload := gatewayclient.SubscriptionPlanUpdatePayload{
		PlanName:           &cr.Spec.PlanName,
		BillingPlan:        cr.Spec.BillingPlan,
		Status:             cr.Spec.Status,
		StopOnQuotaReach:   cr.Spec.StopOnQuotaReach,
		ThrottleLimitCount: cr.Spec.ThrottleLimitCount,
		ThrottleLimitUnit:  cr.Spec.ThrottleLimitUnit,
	}
	if cr.Spec.ExpiryTime != nil {
		t := cr.Spec.ExpiryTime.Time
		updatePayload.ExpiryTime = (*time.Time)(&t)
	}
	if _, err := gatewayclient.UpdateSubscriptionPlan(ctx, gatewayEndpoint, cr.Status.Id, updatePayload, authFn); err != nil {
		return DeployResult{}, err
	}
	return DeployResult{Id: cr.Status.Id}, nil
}

func (a *subscriptionPlanAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*apiv1.SubscriptionPlan)
	if cr.Status.Id == "" {
		return nil
	}
	return gatewayclient.DeleteSubscriptionPlan(ctx, gatewayEndpoint, cr.Status.Id, authFn)
}

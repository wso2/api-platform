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
	"errors"
	"fmt"
	"net/http"
	"strings"

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

const budgetFinalizer = "gateway.api-platform.wso2.com/budget-finalizer"

// controlPlaneOnlyBudgetFields are the spec fields a control plane Budget carries that the
// gateway has no use for. The CRD accepts them so such a manifest applies unchanged; the
// operator does not send them, as the control plane does not when it syncs a budget.
var controlPlaneOnlyBudgetFields = []string{"displayName", "description", "warnings"}

// BudgetReconciler reconciles Budget CRs against the gateway-controller management API
// at /budgets.
type BudgetReconciler struct {
	GenericReconciler
}

// NewBudgetReconciler constructs a fully wired Budget reconciler.
func NewBudgetReconciler(c client.Client, cfg *config.OperatorConfig, logger *slog.Logger, tracker *ResourceTracker) *BudgetReconciler {
	r := &BudgetReconciler{}
	r.Client = c
	r.Config = cfg
	r.Logger = logger
	r.Tracker = tracker
	r.Adapter = &budgetAdapter{}
	return r
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=budgets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=budgets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=budgets/finalizers,verbs=update

// SetupWithManager registers the controller with mgr.
func (r *BudgetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles(r.Config.Reconciliation.MaxConcurrentReconciles)}
	return ctrl.NewControllerManagedBy(mgr).
		Named("budget").
		WithOptions(opts).
		For(&apiv1.Budget{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(enqueueAllOfKind(r.Client, &apiv1.BudgetList{})),
			builder.WithPredicates(gatewayWatchPredicate())).
		Complete(r)
}

// budgetAdapter implements ResourceAdapter for Budget.
type budgetAdapter struct{}

func (a *budgetAdapter) Kind() string             { return "Budget" }
func (a *budgetAdapter) FinalizerName() string    { return budgetFinalizer }
func (a *budgetAdapter) NewObject() client.Object { return &apiv1.Budget{} }
func (a *budgetAdapter) IsUUIDKeyed() bool        { return false }

func (a *budgetAdapter) Handle(obj client.Object) string {
	return obj.GetName()
}

func (a *budgetAdapter) GetStatus(obj client.Object) *apiv1.ResourceStatus {
	return &obj.(*apiv1.Budget).Status
}

func (a *budgetAdapter) SetStatusId(obj client.Object, id string) {
	obj.(*apiv1.Budget).Status.Id = id
}

func (a *budgetAdapter) GatewaySelectionKey(obj client.Object) (string, map[string]string) {
	cr := obj.(*apiv1.Budget)
	return cr.Namespace, cr.Labels
}

// buildBudgetPayload builds the Budget kind sent to the gateway: the CR's spec without the
// control-plane-only fields, and spec.labels as the kind's metadata.labels (a list; the
// CR's own metadata.labels are a map and only select the gateway).
func buildBudgetPayload(cr *apiv1.Budget) ([]byte, error) {
	spec, err := specToJSONMap(cr.Spec)
	if err != nil {
		return nil, err
	}
	delete(spec, "labels")
	for _, f := range controlPlaneOnlyBudgetFields {
		delete(spec, f)
	}
	labels := make([]string, 0, len(cr.Spec.Labels))
	for _, l := range cr.Spec.Labels {
		labels = append(labels, string(l))
	}
	return gatewayclient.BuildBudgetYAML(gatewayclient.ManagementArtifactAPIVersion,
		gatewayclient.BudgetPayloadMetadata{Name: cr.Name, Labels: labels, Annotations: cr.Annotations}, spec)
}

func (a *budgetAdapter) Deploy(ctx context.Context, _ client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	cr := obj.(*apiv1.Budget)
	body, err := buildBudgetPayload(cr)
	if err != nil {
		return DeployResult{}, &gatewayclient.NonRetryableError{Err: fmt.Errorf("build payload: %w", err)}
	}
	if err := deployEnvelopeResource(ctx, gatewayEndpoint, gatewayclient.BudgetsPath(), cr.Name, body, authFn); err != nil {
		return DeployResult{}, rewriteBudgetGatewayErrors(err)
	}
	return DeployResult{}, nil
}

// Delete removes the budget from the gateway. A gateway in immutable mode refuses the
// DELETE (405), but it never took the budget from the operator either, so the CR can go.
func (a *budgetAdapter) Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error {
	cr := obj.(*apiv1.Budget)
	err := gatewayclient.DeleteResource(ctx, gatewayEndpoint, gatewayclient.BudgetsPath(), cr.Name, authFn)
	var nr *gatewayclient.NonRetryableError
	if errors.As(err, &nr) && nr.StatusCode == http.StatusMethodNotAllowed {
		return nil
	}
	return err
}

// rewriteBudgetGatewayErrors words what the gateway refuses with so the CR's Programmed
// condition says what to do. Every case stays non-retryable: none fixes itself.
//   - 404 "Budgets are not enabled": the gateway runs with ai_cost_management.budgets.enabled
//     off, and answers 404 on every /budgets path.
//   - 405: the gateway is in immutable mode and refuses POST, PUT and DELETE.
//
// Other statuses keep the gateway's message: 400 (validation, which sets the Invalid
// reason) and 409 (a name held by a budget the control plane owns, or another conflict).
func rewriteBudgetGatewayErrors(err error) error {
	var nr *gatewayclient.NonRetryableError
	if !errors.As(err, &nr) {
		return err
	}
	switch nr.StatusCode {
	case http.StatusNotFound:
		if strings.Contains(nr.Err.Error(), "Budgets are not enabled") {
			return &gatewayclient.NonRetryableError{
				Err:        errors.New("budgets are not enabled on the gateway: set ai_cost_management.budgets.enabled to true (APIP_GW_AI_COST_MANAGEMENT_BUDGETS_ENABLED)"),
				StatusCode: nr.StatusCode,
			}
		}
	case http.StatusMethodNotAllowed:
		return &gatewayclient.NonRetryableError{
			Err:        errors.New("the gateway is in immutable mode and does not accept budgets from the operator: provide them as artifact files"),
			StatusCode: nr.StatusCode,
		}
	}
	return err
}

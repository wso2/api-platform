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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

// onlineShopBudget builds the sample Budget: each Online Shop agent gets $25 a day on one
// provider, with an exempt agent, labels, and the control plane's own fields set.
func onlineShopBudget() *apiv1.Budget {
	return &apiv1.Budget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "online-shop-agent-daily", Namespace: "apigateway-demo",
			Labels: map[string]string{"gateway": "default"},
		},
		Spec: apiv1.BudgetConfigData{
			Labels: []apiv1.BudgetLabel{"finance", "agents"},
			Limit: apiv1.BudgetLimit{
				Amount: "25.00", Period: "day", Action: "block", BlockAt: ptr(int32(90)),
				SplitBy: &apiv1.BudgetSplit{Type: "agent"},
			},
			Filters: []apiv1.BudgetFilter{{Type: "llmProvider", Id: ptr("mock-openai")}},
			Exemptions: []apiv1.BudgetExemption{
				{Type: "agent", Project: ptr("online-shop"), Id: ptr("returns-bot")},
			},
			DisplayName: ptr("Online Shop agent budget"),
			Description: ptr("Each agent gets a daily usage"),
			Warnings:    &apiv1.BudgetWarnings{Thresholds: []int32{80, 100}, Channels: []string{"finance-webhook"}},
		},
	}
}

func budgetEnvelope(t *testing.T, r capturedRequest) map[string]interface{} {
	t.Helper()
	var envelope map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(r.Body), &envelope))
	return envelope
}

// gatewayAnswering stands in for a gateway-controller that answers every request with the
// given status and body, after the GET existence probe answers probe.
func gatewayAnswering(t *testing.T, probe, status int, body string) (endpoint string, seen func() []capturedRequest) {
	t.Helper()
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		requests = append(requests, capturedRequest{Method: r.Method, Path: r.URL.Path, Body: string(b)})
		code := status
		if r.Method == http.MethodGet {
			code = probe
		}
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL, func() []capturedRequest { return requests }
}

func TestBudgetDeploy_TargetsTheBudgetsPath(t *testing.T) {
	for _, tc := range []struct {
		name       string
		exists     bool
		wantMethod string
		wantPath   string
	}{
		{"create", false, http.MethodPost, gatewayclient.BudgetsPath()},
		{"update", true, http.MethodPut, gatewayclient.BudgetsPath() + "/online-shop-agent-daily"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			endpoint, seen := fakeManagementAPI(t, tc.exists)
			_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)
			require.NoError(t, err)

			requests := seen()
			require.Len(t, requests, 2, "expected an existence probe followed by the write")
			require.Equal(t, http.MethodGet, requests[0].Method)
			require.Equal(t, gatewayclient.BudgetsPath()+"/online-shop-agent-daily", requests[0].Path)
			require.Equal(t, tc.wantMethod, requests[1].Method)
			require.Equal(t, tc.wantPath, requests[1].Path)

			envelope := budgetEnvelope(t, requests[1])
			require.Equal(t, "Budget", envelope["kind"])
			require.Equal(t, gatewayclient.ManagementArtifactAPIVersion, envelope["apiVersion"])
		})
	}
}

func TestBudgetBudgetsPathIsTheManagementAPIs(t *testing.T) {
	require.Equal(t, "/api/management/v1/budgets", gatewayclient.BudgetsPath())
}

// The gateway's budget labels are a list of words; the CR's own labels are a map that only
// selects the gateway. Sending the map would be refused by the gateway.
func TestBudgetDeploy_SendsSpecLabelsAsAListAndNotTheGatewaySelector(t *testing.T) {
	endpoint, seen := fakeManagementAPI(t, false)
	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)
	require.NoError(t, err)

	envelope := budgetEnvelope(t, seen()[1])
	metadata := envelope["metadata"].(map[string]interface{})
	require.Equal(t, "online-shop-agent-daily", metadata["name"])
	require.Equal(t, []interface{}{"finance", "agents"}, metadata["labels"])
	spec := envelope["spec"].(map[string]interface{})
	require.NotContains(t, spec, "labels", "labels are the kind's metadata.labels, not part of its spec")
}

func TestBudgetDeploy_SendsTheLimitFiltersAndExemptions(t *testing.T) {
	endpoint, seen := fakeManagementAPI(t, false)
	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)
	require.NoError(t, err)

	spec := budgetEnvelope(t, seen()[1])["spec"].(map[string]interface{})
	limit := spec["limit"].(map[string]interface{})
	require.Equal(t, "25.00", limit["amount"], "the amount stays a string, as written")
	require.Equal(t, "day", limit["period"])
	require.Equal(t, "block", limit["action"])
	require.EqualValues(t, 90, limit["blockAt"])
	require.Equal(t, map[string]interface{}{"type": "agent"}, limit["splitBy"])
	require.NotContains(t, limit, "currency", "amounts are USD; there is no currency")
	require.Equal(t, []interface{}{map[string]interface{}{"type": "llmProvider", "id": "mock-openai"}}, spec["filters"])
	require.Equal(t, []interface{}{map[string]interface{}{"type": "agent", "project": "online-shop", "id": "returns-bot"}}, spec["exemptions"])
}

// The CRD accepts a control plane manifest's displayName, description and warnings, but the
// operator does not send them: the control plane does not either, when it syncs a budget.
func TestBudgetDeploy_LeavesOutTheControlPlaneOnlyFields(t *testing.T) {
	endpoint, seen := fakeManagementAPI(t, false)
	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)
	require.NoError(t, err)

	spec := budgetEnvelope(t, seen()[1])["spec"].(map[string]interface{})
	for _, field := range []string{"displayName", "description", "warnings"} {
		require.NotContains(t, spec, field)
	}
}

func TestBudgetDeploy_OmitsWhatTheCRLeavesOut(t *testing.T) {
	cr := onlineShopBudget()
	cr.Spec = apiv1.BudgetConfigData{Limit: apiv1.BudgetLimit{Amount: "0.50", Period: "month", Action: "allow"}}
	endpoint, seen := fakeManagementAPI(t, false)
	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, cr, nil)
	require.NoError(t, err)

	envelope := budgetEnvelope(t, seen()[1])
	require.NotContains(t, envelope["metadata"].(map[string]interface{}), "labels")
	spec := envelope["spec"].(map[string]interface{})
	require.Equal(t, map[string]interface{}{"amount": "0.50", "period": "month", "action": "allow"}, spec["limit"])
	for _, field := range []string{"filters", "exemptions", "status"} {
		require.NotContains(t, spec, field)
	}
}

func TestBudgetDeploy_SendsAnInactiveStatus(t *testing.T) {
	cr := onlineShopBudget()
	cr.Spec.Status = ptr("inactive")
	endpoint, seen := fakeManagementAPI(t, false)
	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, cr, nil)
	require.NoError(t, err)
	require.Equal(t, "inactive", budgetEnvelope(t, seen()[1])["spec"].(map[string]interface{})["status"])
}

// Annotations go with the budget, as on the other kinds: the gateway reads
// gateway.api-platform.wso2.com/artifact-id and project-id from them.
func TestBudgetDeploy_SendsTheCRsAnnotations(t *testing.T) {
	cr := onlineShopBudget()
	cr.Annotations = map[string]string{"gateway.api-platform.wso2.com/artifact-id": "5f0c6f0e-1d1b-4c7e-9d63-0a6c2f6a9b11"}
	endpoint, seen := fakeManagementAPI(t, false)
	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, cr, nil)
	require.NoError(t, err)

	metadata := budgetEnvelope(t, seen()[1])["metadata"].(map[string]interface{})
	require.Equal(t, map[string]interface{}{"gateway.api-platform.wso2.com/artifact-id": "5f0c6f0e-1d1b-4c7e-9d63-0a6c2f6a9b11"}, metadata["annotations"])
}

func TestBudgetDeploy_DoesNotMutateTheCR(t *testing.T) {
	cr := onlineShopBudget()
	before := cr.DeepCopy()
	endpoint, _ := fakeManagementAPI(t, false)
	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, cr, nil)
	require.NoError(t, err)
	require.Equal(t, before, cr)
}

// ---- what the gateway can refuse with ----

// budgetsDisabledBody is what the gateway-controller answers on every /budgets path when
// ai_cost_management.budgets.enabled is off (budgetsAvailable in its budget handler).
const budgetsDisabledBody = `{"status":"error","message":"Budgets are not enabled on this gateway. Set ai_cost_management.budgets.enabled to use them."}`

// With budgets off, the existence probe answers 404 too, so the operator tries to create
// and the create answers 404: not retryable, and worded for the person who owns the CR.
func TestBudgetDeploy_ExplainsAGatewayWithBudgetsOff(t *testing.T) {
	endpoint, seen := gatewayAnswering(t, http.StatusNotFound, http.StatusNotFound, budgetsDisabledBody)

	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)

	var nr *gatewayclient.NonRetryableError
	require.ErrorAs(t, err, &nr, "turning budgets on is not something a retry fixes")
	require.Equal(t, http.StatusNotFound, nr.StatusCode)
	require.Equal(t, "budgets are not enabled on the gateway: set ai_cost_management.budgets.enabled to true (APIP_GW_AI_COST_MANAGEMENT_BUDGETS_ENABLED)", nr.Error())
	require.Len(t, seen(), 2)
}

// A 404 without that message is something else (for example a gateway without budget
// support): it keeps the gateway's own words.
func TestBudgetDeploy_KeepsAnyOther404AsTheGatewayWordsIt(t *testing.T) {
	endpoint, _ := gatewayAnswering(t, http.StatusNotFound, http.StatusNotFound, "404 page not found")

	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)

	var nr *gatewayclient.NonRetryableError
	require.ErrorAs(t, err, &nr)
	require.Contains(t, nr.Error(), "404 page not found")
}

// An immutable-mode gateway refuses POST, PUT and DELETE with 405 and loads budgets from
// artifact files only.
func TestBudgetDeploy_ExplainsAnImmutableGateway(t *testing.T) {
	endpoint, _ := gatewayAnswering(t, http.StatusNotFound, http.StatusMethodNotAllowed,
		`{"status":"error","message":"Gateway is in immutable mode. Mutating operations are not allowed."}`)

	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)

	var nr *gatewayclient.NonRetryableError
	require.ErrorAs(t, err, &nr)
	require.Equal(t, http.StatusMethodNotAllowed, nr.StatusCode)
	require.Contains(t, nr.Error(), "immutable mode")
	require.Contains(t, nr.Error(), "artifact files")
}

// 409: a name held by a budget the control plane owns, or another conflict.
func TestBudgetDeploy_KeepsTheGatewaysWordsOnAConflict(t *testing.T) {
	const conflict = `{"status":"error","message":"budget 'online-shop-agent-daily' is owned by the control plane and cannot be changed here"}`
	endpoint, _ := gatewayAnswering(t, http.StatusNotFound, http.StatusConflict, conflict)

	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)

	var nr *gatewayclient.NonRetryableError
	require.ErrorAs(t, err, &nr)
	require.Equal(t, http.StatusConflict, nr.StatusCode)
	require.Contains(t, nr.Error(), "owned by the control plane")
}

// 400: the gateway's validation, which the CRD's structural schema cannot do (CEL that does
// not compile, a project filter on a project budget, ...).
func TestBudgetDeploy_KeepsTheGatewaysWordsOnValidationErrors(t *testing.T) {
	const invalid = `{"status":"error","message":"Budget validation failed","errors":[{"field":"spec.filters[0].expression","message":"does not compile"}]}`
	endpoint, _ := gatewayAnswering(t, http.StatusNotFound, http.StatusBadRequest, invalid)

	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)

	var nr *gatewayclient.NonRetryableError
	require.ErrorAs(t, err, &nr)
	require.Equal(t, http.StatusBadRequest, nr.StatusCode)
	require.Contains(t, nr.Error(), "does not compile")
}

// A gateway that is down or restarting is retried, as for every kind.
func TestBudgetDeploy_RetriesAGatewayThatIsUnavailable(t *testing.T) {
	endpoint, _ := gatewayAnswering(t, http.StatusNotFound, http.StatusServiceUnavailable, "unavailable")

	_, err := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)

	var re *gatewayclient.RetryableError
	require.ErrorAs(t, err, &re)
}

func TestRewriteBudgetGatewayErrors_LeavesOtherErrorsAlone(t *testing.T) {
	plain := errors.New("connection refused")
	require.Same(t, plain, rewriteBudgetGatewayErrors(plain))

	retryable := &gatewayclient.RetryableError{Err: errors.New("x"), StatusCode: http.StatusServiceUnavailable}
	require.Same(t, retryable, rewriteBudgetGatewayErrors(retryable))
}

// ---- delete ----

func TestBudgetDelete_TargetsTheBudgetsPath(t *testing.T) {
	endpoint, seen := fakeManagementAPI(t, true)
	require.NoError(t, (&budgetAdapter{}).Delete(context.Background(), endpoint, onlineShopBudget(), nil))

	requests := seen()
	require.Len(t, requests, 1)
	require.Equal(t, http.MethodDelete, requests[0].Method)
	require.Equal(t, gatewayclient.BudgetsPath()+"/online-shop-agent-daily", requests[0].Path)
}

// With budgets off the gateway answers 404 on DELETE as well: nothing was ever deployed, so
// the CR can go.
func TestBudgetDelete_IsDoneWhenTheGatewayHasNoSuchBudget(t *testing.T) {
	endpoint, _ := gatewayAnswering(t, http.StatusNotFound, http.StatusNotFound, budgetsDisabledBody)
	require.NoError(t, (&budgetAdapter{}).Delete(context.Background(), endpoint, onlineShopBudget(), nil))
}

// An immutable gateway refuses DELETE (405), but it never took the budget from the operator
// either; failing here would leave the CR stuck deleting behind its finalizer.
func TestBudgetDelete_IsDoneOnAnImmutableGateway(t *testing.T) {
	endpoint, _ := gatewayAnswering(t, http.StatusNotFound, http.StatusMethodNotAllowed,
		`{"status":"error","message":"Gateway is in immutable mode. Mutating operations are not allowed."}`)
	require.NoError(t, (&budgetAdapter{}).Delete(context.Background(), endpoint, onlineShopBudget(), nil))
}

func TestBudgetDelete_FailsAndRetriesOtherErrors(t *testing.T) {
	endpoint, _ := gatewayAnswering(t, http.StatusOK, http.StatusServiceUnavailable, "unavailable")
	require.Error(t, (&budgetAdapter{}).Delete(context.Background(), endpoint, onlineShopBudget(), nil))
}

// ---- adapter contract ----

func TestBudgetAdapterIdentity(t *testing.T) {
	a := &budgetAdapter{}
	cr := onlineShopBudget()

	require.Equal(t, "Budget", a.Kind())
	require.False(t, a.IsUUIDKeyed(), "a budget is addressed by its name, not a gateway-issued UUID")
	require.Equal(t, "online-shop-agent-daily", a.Handle(cr))
	require.Equal(t, budgetFinalizer, a.FinalizerName())
	require.IsType(t, &apiv1.Budget{}, a.NewObject())
	require.Same(t, &cr.Status, a.GetStatus(cr))

	a.SetStatusId(cr, "x")
	require.Equal(t, "x", cr.Status.Id)

	ns, labels := a.GatewaySelectionKey(cr)
	require.Equal(t, "apigateway-demo", ns)
	require.Equal(t, map[string]string{"gateway": "default"}, labels, "the CR's labels select the gateway, as for every kind")
}

func TestNewBudgetReconcilerWiresTheAdapter(t *testing.T) {
	r := NewBudgetReconciler(fake.NewClientBuilder().Build(), &config.OperatorConfig{}, slog.Default(), NewResourceTracker())
	require.IsType(t, &budgetAdapter{}, r.Adapter)
}

// ---- what the CR says ----

// budgetFailureCondition runs a deploy failure through the generic reconciler's error
// handling and returns the Programmed condition it leaves on the CR.
func budgetFailureCondition(t *testing.T, deployErr error) *metav1.Condition {
	t.Helper()
	cr := onlineShopBudget()
	cr.Generation = 1
	k8sClient := fake.NewClientBuilder().
		WithScheme(agentTestScheme(t)).
		WithObjects(cr).
		WithStatusSubresource(&apiv1.Budget{}).
		Build()
	r := NewBudgetReconciler(k8sClient, &config.OperatorConfig{}, slog.Default(), NewResourceTracker())

	entry := &ResourceTrackingEntry{Generation: 1, Status: ResourceStatusProcessing}
	_, err := r.handleDeploymentError(context.Background(), cr, "Budget|key", entry, deployErr)
	require.NoError(t, err)

	var latest apiv1.Budget
	require.NoError(t, k8sClient.Get(context.Background(), types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}, &latest))
	return meta.FindStatusCondition(latest.Status.Conditions, apiv1.ConditionProgrammed)
}

func TestBudgetCondition_SaysBudgetsAreOffOnTheGateway(t *testing.T) {
	endpoint, _ := gatewayAnswering(t, http.StatusNotFound, http.StatusNotFound, budgetsDisabledBody)
	_, deployErr := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)

	cond := budgetFailureCondition(t, deployErr)

	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionFalse, cond.Status)
	require.Equal(t, apiv1.ReasonDeploymentFailed, cond.Reason)
	require.Equal(t, "Deployment failed: budgets are not enabled on the gateway: set ai_cost_management.budgets.enabled to true (APIP_GW_AI_COST_MANAGEMENT_BUDGETS_ENABLED)", cond.Message)
	require.EqualValues(t, 1, cond.ObservedGeneration)
}

func TestBudgetCondition_SaysTheGatewayIsImmutable(t *testing.T) {
	endpoint, _ := gatewayAnswering(t, http.StatusNotFound, http.StatusMethodNotAllowed, `{"status":"error"}`)
	_, deployErr := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)

	cond := budgetFailureCondition(t, deployErr)

	require.Equal(t, apiv1.ReasonDeploymentFailed, cond.Reason)
	require.Contains(t, cond.Message, "immutable mode")
}

func TestBudgetCondition_ValidationErrorsAreInvalid(t *testing.T) {
	endpoint, _ := gatewayAnswering(t, http.StatusNotFound, http.StatusBadRequest,
		`{"status":"error","message":"Budget validation failed","errors":[{"field":"spec.limit.amount","message":"must be above zero"}]}`)
	_, deployErr := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)

	cond := budgetFailureCondition(t, deployErr)

	require.Equal(t, apiv1.ReasonInvalid, cond.Reason)
	require.Contains(t, cond.Message, "must be above zero")
}

func TestBudgetCondition_ConflictsAreDeploymentFailures(t *testing.T) {
	endpoint, _ := gatewayAnswering(t, http.StatusNotFound, http.StatusConflict,
		`{"status":"error","message":"budget is owned by the control plane"}`)
	_, deployErr := (&budgetAdapter{}).Deploy(context.Background(), nil, endpoint, onlineShopBudget(), nil)

	cond := budgetFailureCondition(t, deployErr)

	require.Equal(t, apiv1.ReasonDeploymentFailed, cond.Reason)
	require.Contains(t, cond.Message, "owned by the control plane")
}

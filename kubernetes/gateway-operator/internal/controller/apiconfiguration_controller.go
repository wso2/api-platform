/*
Copyright 2025.

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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/selector"
)

// APIConfigurationReconciler reconciles a APIConfiguration object
type APIConfigurationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.OperatorConfig
}

const (
	apiConditionReasonDeploymentFailed    = "DeploymentFailed"
	apiConditionReasonDeploymentSucceeded = "DeploymentSucceeded"
	apiConditionReasonWaitingForGateway   = "WaitingForGateway"

	apiFinalizerName = "api.api-platform.wso2.com/api-finalizer"
)

//+kubebuilder:rbac:groups=api.api-platform.wso2.com,resources=apiconfigurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=api.api-platform.wso2.com,resources=apiconfigurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=api.api-platform.wso2.com,resources=apiconfigurations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the APIConfiguration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *APIConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	apiConfig := &apiv1.APIConfiguration{}
	if err := r.Get(ctx, req.NamespacedName, apiConfig); err != nil {
		log.Error(err, "unable to fetch APIConfiguration")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling APIConfiguration",
		"name", apiConfig.Name,
		"namespace", apiConfig.Namespace)

	if !apiConfig.DeletionTimestamp.IsZero() {
		return r.reconcileAPIDeletion(ctx, apiConfig)
	}

	if !controllerutil.ContainsFinalizer(apiConfig, apiFinalizerName) {
		controllerutil.AddFinalizer(apiConfig, apiFinalizerName)
		if err := r.Update(ctx, apiConfig); err != nil {
			log.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	availableGateways, expectedGatewaysSet, missingGateways := r.resolveGateways(apiConfig)
	targetKeys := sortedGatewayKeys(availableGateways)
	expectedGateways := setToSortedSlice(expectedGatewaysSet)

	var deployed []string
	var deploymentErrors []string
	requeueAfter := false

	
	for _, key := range targetKeys {
		gateway := availableGateways[key]
		if err := r.deployAPIToGateway(ctx, apiConfig, gateway); err != nil {
			log.Error(err, "Failed to deploy API to gateway",
				"gateway", gateway.Name,
				"namespace", gateway.Namespace)
			deploymentErrors = append(deploymentErrors, fmt.Sprintf("%s: %v", key, err))
			requeueAfter = true
			continue
		}

		deployed = append(deployed, key)
		log.Info("Successfully deployed API to gateway",
			"api", apiConfig.Name,
			"gateway", gateway.Name,
			"namespace", gateway.Namespace)
	}
	

	if len(missingGateways) > 0 {
		requeueAfter = true
	}

	deployed = normalizeGatewayList(deployed)

	var (
		phase           apiv1.APIPhase
		conditionStatus metav1.ConditionStatus
		conditionReason string
		conditionMsg    string
	)

	switch {
	case len(deployed) > 0 && len(deploymentErrors) == 0 && len(missingGateways) == 0:
		phase = apiv1.APIPhaseDeployed
		conditionStatus = metav1.ConditionTrue
		conditionReason = apiConditionReasonDeploymentSucceeded
		conditionMsg = fmt.Sprintf("API deployed to gateways: %s", strings.Join(deployed, ", "))
	case len(deploymentErrors) > 0:
		phase = apiv1.APIPhaseFailed
		conditionStatus = metav1.ConditionFalse
		conditionReason = apiConditionReasonDeploymentFailed
		parts := []string{fmt.Sprintf("deployment failures: %s", strings.Join(deploymentErrors, "; "))}
		if len(deployed) > 0 {
			parts = append(parts, fmt.Sprintf("successful deployments: %s", strings.Join(deployed, ", ")))
		}
		if len(missingGateways) > 0 {
			parts = append(parts, fmt.Sprintf("waiting for gateways: %s", strings.Join(missingGateways, ", ")))
		}
		conditionMsg = strings.Join(parts, "; ")
	case len(missingGateways) > 0:
		phase = apiv1.APIPhasePending
		conditionStatus = metav1.ConditionFalse
		conditionReason = apiConditionReasonWaitingForGateway
		conditionMsg = fmt.Sprintf("Waiting for gateways to become available: %s", strings.Join(missingGateways, ", "))
	default:
		phase = apiv1.APIPhasePending
		conditionStatus = metav1.ConditionFalse
		conditionReason = apiConditionReasonWaitingForGateway
		if len(expectedGateways) == 0 {
			conditionMsg = "No matching gateways registered for this API"
		} else {
			conditionMsg = fmt.Sprintf("No available gateways matched: %s", strings.Join(expectedGateways, ", "))
		}
	}

	if _, err := r.updateAPIStatus(ctx, apiConfig, apiStatusUpdate{
		Phase: phase,
		Condition: &metav1.Condition{
			Type:    apiv1.APIConditionReady,
			Status:  conditionStatus,
			Reason:  conditionReason,
			Message: conditionMsg,
		},
		DeployedGateways: &deployed,
	}); err != nil {
		log.Error(err, "failed to update APIConfiguration status")
		return ctrl.Result{}, err
	}

	if requeueAfter {
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// deployAPIToGateway sends the API configuration to a specific gateway controller
func (r *APIConfigurationReconciler) deployAPIToGateway(ctx context.Context, apiConfig *apiv1.APIConfiguration, gateway *registry.GatewayInfo) error {
	log := log.FromContext(ctx)

	// Marshal the API spec to YAML
	var apiYAML []byte
	var err error
	apiYAML, err = yaml.Marshal(apiConfig.Spec.APIConfiguration)
	
	if err != nil {
		return fmt.Errorf("failed to marshal API spec to YAML: %w", err)
	}

	// Get the gateway controller endpoint
	endpoint := gateway.GetGatewayServiceEndpoint() + "/apis"

	log.Info("Deploying API to gateway",
		"endpoint", endpoint,
		"api", apiConfig.Name)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(apiYAML))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/yaml")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send API to gateway: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gateway returned error status %d: %s", resp.StatusCode, string(body))
	}

	log.Info("API successfully deployed to gateway", "status", resp.StatusCode)

	return nil
}

type apiStatusUpdate struct {
	Phase            apiv1.APIPhase
	Condition        *metav1.Condition
	DeployedGateways *[]string
}

func (r *APIConfigurationReconciler) updateAPIStatus(ctx context.Context, apiConfig *apiv1.APIConfiguration, update apiStatusUpdate) (bool, error) {
	base := apiConfig.DeepCopy()
	originalStatus := base.Status
	changed := false

	if update.Phase != "" && apiConfig.Status.Phase != update.Phase {
		apiConfig.Status.Phase = update.Phase
		changed = true
	}

	if apiConfig.Status.ObservedGeneration != apiConfig.Generation {
		apiConfig.Status.ObservedGeneration = apiConfig.Generation
		changed = true
	}

	if update.DeployedGateways != nil {
		normalized := normalizeGatewayList(*update.DeployedGateways)
		if !stringSliceEqual(apiConfig.Status.DeployedGateways, normalized) {
			apiConfig.Status.DeployedGateways = normalized
			changed = true
		}
	}

	if update.Condition != nil {
		cond := *update.Condition
		if cond.Type == "" {
			cond.Type = apiv1.APIConditionReady
		}
		cond.ObservedGeneration = apiConfig.Generation

		existing := meta.FindStatusCondition(apiConfig.Status.Conditions, cond.Type)
		needsUpdate := false

		if existing == nil {
			cond.LastTransitionTime = metav1.Now()
			needsUpdate = true
		} else {
			if existing.Status != cond.Status || existing.Reason != cond.Reason || existing.Message != cond.Message {
				cond.LastTransitionTime = metav1.Now()
				needsUpdate = true
			} else if existing.ObservedGeneration != cond.ObservedGeneration {
				cond.LastTransitionTime = existing.LastTransitionTime
				needsUpdate = true
			}
		}

		if needsUpdate {
			meta.SetStatusCondition(&apiConfig.Status.Conditions, cond)
			changed = true
		}
	}

	if !changed {
		apiConfig.Status = originalStatus
		return false, nil
	}

	now := metav1.Now()
	apiConfig.Status.LastUpdateTime = &now

	if err := r.Status().Patch(ctx, apiConfig, client.MergeFrom(base)); err != nil {
		return false, err
	}

	return true, nil
}

func (r *APIConfigurationReconciler) enqueueAPIsForGateway(ctx context.Context, obj client.Object) []reconcile.Request {
	gateway, ok := obj.(*apiv1.GatewayConfiguration)
	if !ok {
		return nil
	}

	logger := log.FromContext(ctx)

	apiList := &apiv1.APIConfigurationList{}
	if err := r.List(ctx, apiList); err != nil {
		logger.Error(err, "failed to list APIConfigurations for gateway event",
			"gateway", gateway.Name,
			"namespace", gateway.Namespace)
		return nil
	}

	sel := selector.NewAPISelector(r.Client)
	requests := make([]reconcile.Request, 0, len(apiList.Items))

	for i := range apiList.Items {
		api := &apiList.Items[i]
		wants, err := r.apiTargetsGateway(ctx, api, gateway, sel)
		if err != nil {
			logger.Error(err, "failed to evaluate API selection for gateway",
				"api", api.Name,
				"namespace", api.Namespace)
			continue
		}

		if !wants {
			continue
		}

		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: api.Namespace,
			Name:      api.Name,
		}})
	}

	return requests
}

func (r *APIConfigurationReconciler) apiTargetsGateway(ctx context.Context, api *apiv1.APIConfiguration, gateway *apiv1.GatewayConfiguration, sel *selector.APISelector) (bool, error) {
	for _, ref := range api.Spec.GatewayRefs {
		ns := ref.Namespace
		if ns == "" {
			ns = api.Namespace
		}
		if ref.Name == gateway.Name && ns == gateway.Namespace {
			return true, nil
		}
	}

	if len(api.Spec.GatewayRefs) > 0 {
		return false, nil
	}

	return sel.IsAPISelectedByGateway(ctx, api, gateway)
}

func (r *APIConfigurationReconciler) resolveGateways(apiConfig *apiv1.APIConfiguration) (map[string]*registry.GatewayInfo, map[string]struct{}, []string) {
	registryInstance := registry.GetGatewayRegistry()
	available := make(map[string]*registry.GatewayInfo)
	expected := make(map[string]struct{})
	missingSet := make(map[string]struct{})

	if len(apiConfig.Spec.GatewayRefs) > 0 {
		for _, ref := range apiConfig.Spec.GatewayRefs {
			ns := ref.Namespace
			if ns == "" {
				ns = apiConfig.Namespace
			}
			key := namespacedName(ns, ref.Name)
			expected[key] = struct{}{}
			if gw, ok := registryInstance.Get(ns, ref.Name); ok {
				available[key] = gw
			} else {
				missingSet[key] = struct{}{}
			}
		}
	} else {
		matched := registryInstance.FindMatchingGateways(apiConfig.Namespace, apiConfig.Labels)
		for _, gw := range matched {
			key := namespacedName(gw.Namespace, gw.Name)
			expected[key] = struct{}{}
			available[key] = gw
		}
	}

	return available, expected, setToSortedSlice(missingSet)
}

func (r *APIConfigurationReconciler) reconcileAPIDeletion(ctx context.Context, apiConfig *apiv1.APIConfiguration) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(apiConfig, apiFinalizerName) {
		return ctrl.Result{}, nil
	}

	if err := r.cleanupAPIDeployments(ctx, apiConfig); err != nil {
		log.Error(err, "failed to clean up API deployments")
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(apiConfig, apiFinalizerName)
	if err := r.Update(ctx, apiConfig); err != nil {
		log.Error(err, "failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *APIConfigurationReconciler) cleanupAPIDeployments(ctx context.Context, apiConfig *apiv1.APIConfiguration) error {
	log := log.FromContext(ctx)

	available, _, missingList := r.resolveGateways(apiConfig)
	missingSet := make(map[string]struct{}, len(missingList))
	for _, key := range missingList {
		missingSet[key] = struct{}{}
	}

	registryInstance := registry.GetGatewayRegistry()
	for _, key := range apiConfig.Status.DeployedGateways {
		if _, exists := available[key]; exists {
			continue
		}
		ns, name := splitNamespacedKey(key)
		if ns == "" {
			ns = apiConfig.Namespace
		}
		if gw, ok := registryInstance.Get(ns, name); ok {
			available[key] = gw
		} else {
			missingSet[key] = struct{}{}
		}
	}

	deleteKeys := sortedGatewayKeys(available)
	if len(deleteKeys) == 0 {
		if len(missingSet) > 0 {
			log.V(1).Info("No registered gateways available for API cleanup",
				"missingGateways", setToSortedSlice(missingSet))
		}
		return nil
	}

	apiName, apiVersion := extractAPIMetadata(apiConfig)
	if apiName == "" || apiVersion == "" {
		log.Info("API cleanup skipped due to missing name or version in spec")
		return nil
	}

	var errs []error
	for _, key := range deleteKeys {
		gateway := available[key]
		if err := r.deleteAPIFromGateway(ctx, apiName, apiVersion, gateway); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (r *APIConfigurationReconciler) deleteAPIFromGateway(ctx context.Context, apiName, apiVersion string, gateway *registry.GatewayInfo) error {
	log := log.FromContext(ctx)

	endpoint := fmt.Sprintf("%s/apis/%s/%s",
		gateway.GetGatewayServiceEndpoint(),
		url.PathEscape(apiName),
		url.PathEscape(apiVersion))

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusAccepted:
		log.Info("API removed from gateway",
			"gateway", gateway.Name,
			"namespace", gateway.Namespace)
		return nil
	case http.StatusNotFound:
		log.V(1).Info("API already absent from gateway",
			"gateway", gateway.Name,
			"namespace", gateway.Namespace)
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gateway returned error status %d: %s", resp.StatusCode, string(body))
	}
}

func extractAPIMetadata(apiConfig *apiv1.APIConfiguration) (string, string) {
	data := apiConfig.Spec.APIConfiguration.Spec
	name := strings.TrimSpace(data.Name)
	version := strings.TrimSpace(data.Version)
	return name, version
}

func sortedGatewayKeys(gateways map[string]*registry.GatewayInfo) []string {
	if len(gateways) == 0 {
		return nil
	}
	keys := make([]string, 0, len(gateways))
	for key := range gateways {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func splitNamespacedKey(key string) (string, string) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", key
}

func normalizeGatewayList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	copyValues := append([]string(nil), values...)
	sort.Strings(copyValues)

	result := copyValues[:0]
	last := ""
	for i, val := range copyValues {
		if i == 0 || val != last {
			result = append(result, val)
			last = val
		}
	}

	return append([]string(nil), result...)
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func setToSortedSlice(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func namespacedName(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "/" + name
}

// SetupWithManager sets up the controller with the Manager.
func (r *APIConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	gatewayPred := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(evt event.UpdateEvent) bool {
			newGateway, okNew := evt.ObjectNew.(*apiv1.GatewayConfiguration)
			oldGateway, okOld := evt.ObjectOld.(*apiv1.GatewayConfiguration)
			if !okNew || !okOld {
				return true
			}

			if newGateway.Generation != oldGateway.Generation {
				return true
			}

			return !equality.Semantic.DeepEqual(newGateway.Status, oldGateway.Status)
		},
		DeleteFunc: func(event event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(event event.GenericEvent) bool {
			return true
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.APIConfiguration{}).
		Watches(&apiv1.GatewayConfiguration{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueAPIsForGateway),
			builder.WithPredicates(gatewayPred)).
		Complete(r)
}

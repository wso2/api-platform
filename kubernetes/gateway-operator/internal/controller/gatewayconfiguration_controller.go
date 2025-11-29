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
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/k8sutil"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/selector"
)

const (
	gatewayFinalizerName = "api.api-platform.wso2.com/gateway-finalizer"

	conditionReasonReconciling        = "Reconciling"
	conditionReasonApplySucceeded     = "ApplySucceeded"
	conditionReasonApplyFailed        = "ApplyFailed"
	conditionReasonDeleting           = "Deleting"
	conditionReasonDeploymentsPending = "DeploymentsNotReady"
)

// GatewayConfigurationReconciler reconciles a GatewayConfiguration object
type GatewayConfigurationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.OperatorConfig
}

//+kubebuilder:rbac:groups=api.api-platform.wso2.com,resources=gatewayconfigurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=api.api-platform.wso2.com,resources=gatewayconfigurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=api.api-platform.wso2.com,resources=gatewayconfigurations/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services;persistentvolumeclaims;configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GatewayConfiguration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *GatewayConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the GatewayConfiguration instance
	gatewayConfig := &apiv1.GatewayConfiguration{}
	if err := r.Get(ctx, req.NamespacedName, gatewayConfig); err != nil {
		log.Error(err, "unable to fetch GatewayConfiguration")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling GatewayConfiguration",
		"name", gatewayConfig.Name,
		"namespace", gatewayConfig.Namespace)

	// Handle deletion
	if !gatewayConfig.DeletionTimestamp.IsZero() {
		if _, err := r.updateGatewayStatus(ctx, gatewayConfig, gatewayStatusUpdate{
			Phase: apiv1.GatewayPhaseDeleting,
			Condition: &metav1.Condition{
				Type:    apiv1.GatewayConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  conditionReasonDeleting,
				Message: "GatewayConfiguration is being deleted",
			},
		}); err != nil {
			log.Error(err, "failed to update status during deletion")
			return ctrl.Result{}, err
		}

		if controllerutil.ContainsFinalizer(gatewayConfig, gatewayFinalizerName) {
			// Perform cleanup
			if err := r.deleteGatewayResources(ctx, gatewayConfig); err != nil {
				log.Error(err, "failed to delete gateway resources")
				return ctrl.Result{}, err
			}

			// Remove finalizer
			controllerutil.RemoveFinalizer(gatewayConfig, gatewayFinalizerName)
			if err := r.Update(ctx, gatewayConfig); err != nil {
				log.Error(err, "failed to remove finalizer")
				return ctrl.Result{}, err
			}

			log.Info("Successfully cleaned up gateway resources and removed finalizer")
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(gatewayConfig, gatewayFinalizerName) {
		controllerutil.AddFinalizer(gatewayConfig, gatewayFinalizerName)
		if err := r.Update(ctx, gatewayConfig); err != nil {
			log.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to GatewayConfiguration")
		return ctrl.Result{Requeue: true}, nil
	}

	if gatewayConfig.Status.ObservedGeneration != gatewayConfig.Generation ||
		string(gatewayConfig.Status.Phase) == "" || gatewayConfig.Status.Phase == apiv1.GatewayPhaseFailed {
		if _, err := r.updateGatewayStatus(ctx, gatewayConfig, gatewayStatusUpdate{
			Phase: apiv1.GatewayPhaseReconciling,
			Condition: &metav1.Condition{
				Type:    apiv1.GatewayConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  conditionReasonReconciling,
				Message: "Reconciling gateway resources",
			},
		}); err != nil {
			log.Error(err, "failed to update status to reconciling")
			return ctrl.Result{}, err
		}
	}

	selectedCount, err := r.countSelectedAPIs(ctx, gatewayConfig)
	if err != nil {
		log.Error(err, "failed to evaluate selected APIs")
		if _, statusErr := r.updateGatewayStatus(ctx, gatewayConfig, gatewayStatusUpdate{
			Phase: apiv1.GatewayPhaseFailed,
			Condition: &metav1.Condition{
				Type:    apiv1.GatewayConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  conditionReasonApplyFailed,
				Message: "Failed to evaluate selected APIs: " + err.Error(),
			},
		}); statusErr != nil {
			log.Error(statusErr, "failed to update status after API selection error")
		}
		return ctrl.Result{}, err
	}

	var appliedGenPtr *int64
	if gatewayConfig.Status.AppliedGeneration != gatewayConfig.Generation {
		// Apply the gateway manifest
		if err := r.applyGatewayManifest(ctx, gatewayConfig); err != nil {
			log.Error(err, "failed to apply gateway manifest")
			if _, statusErr := r.updateGatewayStatus(ctx, gatewayConfig, gatewayStatusUpdate{
				Phase: apiv1.GatewayPhaseFailed,
				Condition: &metav1.Condition{
					Type:    apiv1.GatewayConditionReady,
					Status:  metav1.ConditionFalse,
					Reason:  conditionReasonApplyFailed,
					Message: "Failed to apply gateway manifest: " + err.Error(),
				},
			}); statusErr != nil {
				log.Error(statusErr, "failed to update status after manifest apply error")
			}
			return ctrl.Result{}, err
		}
		appliedGen := gatewayConfig.Generation
		appliedGenPtr = &appliedGen
	} else {
		log.V(1).Info("Gateway manifest already applied for current generation", "generation", gatewayConfig.Generation)
	}

	// Register the gateway in the registry
	r.registerGateway(gatewayConfig)

	ready, readinessMsg, err := r.evaluateGatewayReadiness(ctx, gatewayConfig)
	if err != nil {
		log.Error(err, "failed to evaluate gateway readiness")
		if _, statusErr := r.updateGatewayStatus(ctx, gatewayConfig, gatewayStatusUpdate{
			Phase: apiv1.GatewayPhaseFailed,
			Condition: &metav1.Condition{
				Type:    apiv1.GatewayConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  conditionReasonApplyFailed,
				Message: "Failed to evaluate gateway readiness: " + err.Error(),
			},
			SelectedAPIs: &selectedCount,
		}); statusErr != nil {
			log.Error(statusErr, "failed to update status after readiness evaluation error")
		}
		return ctrl.Result{}, err
	}

	if !ready {
		changed, err := r.updateGatewayStatus(ctx, gatewayConfig, gatewayStatusUpdate{
			Phase: apiv1.GatewayPhaseReconciling,
			Condition: &metav1.Condition{
				Type:    apiv1.GatewayConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  conditionReasonDeploymentsPending,
				Message: readinessMsg,
			},
			SelectedAPIs:      &selectedCount,
			AppliedGeneration: appliedGenPtr,
		})
		if err != nil {
			log.Error(err, "failed to update status while waiting for gateway readiness")
			return ctrl.Result{}, err
		}
		if changed {
			log.Info("Waiting for gateway deployments to become ready", "message", readinessMsg)
		} else {
			log.V(1).Info("Gateway deployments not ready yet; status unchanged", "message", readinessMsg)
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if changed, err := r.updateGatewayStatus(ctx, gatewayConfig, gatewayStatusUpdate{
		Phase: apiv1.GatewayPhaseReady,
		Condition: &metav1.Condition{
			Type:    apiv1.GatewayConditionReady,
			Status:  metav1.ConditionTrue,
			Reason:  conditionReasonApplySucceeded,
			Message: readinessMsg,
		},
		SelectedAPIs:      &selectedCount,
		AppliedGeneration: appliedGenPtr,
	}); err != nil {
		log.Error(err, "failed to update status to ready")
		return ctrl.Result{}, err
	} else if changed {
		log.Info("Gateway is ready", "message", readinessMsg)
	}

	return ctrl.Result{}, nil
}

// applyGatewayManifest applies the gateway using Helm only
func (r *GatewayConfigurationReconciler) applyGatewayManifest(ctx context.Context, owner *apiv1.GatewayConfiguration) error {
	namespace := owner.Namespace
	if namespace == "" {
		namespace = "default"
	}
	return r.deployGatewayWithHelm(ctx, owner, namespace)
}

// deployGatewayWithHelm deploys the gateway using Helm chart
func (r *GatewayConfigurationReconciler) deployGatewayWithHelm(ctx context.Context, owner *apiv1.GatewayConfiguration, namespace string) error {
	log := log.FromContext(ctx)

	log.Info("Deploying gateway using Helm",
		"chart", r.Config.Gateway.HelmChartPath,
		"version", r.Config.Gateway.HelmChartVersion)

	// Create Helm client
	helmClient := helm.NewClient()

	// Generate release name
	releaseName := helm.GetReleaseName(owner.Name)

	// Install or upgrade the chart
	err := helmClient.InstallOrUpgrade(ctx, helm.InstallOrUpgradeOptions{
		ReleaseName:     releaseName,
		Namespace:       namespace,
		ChartPath:       r.Config.Gateway.HelmChartPath,
		ValuesFilePath:  r.Config.Gateway.HelmValuesFilePath, // Optional custom values
		Version:         r.Config.Gateway.HelmChartVersion,
		CreateNamespace: true,
		Wait:            true,
		Timeout:         300, // 5 minutes
	})

	if err != nil {
		return fmt.Errorf("failed to install/upgrade Helm chart: %w", err)
	}

	log.Info("Successfully deployed gateway with Helm", "release", releaseName)
	return nil
}

// buildTemplateData creates template data from GatewayConfiguration spec
func (r *GatewayConfigurationReconciler) buildTemplateData(gatewayConfig *apiv1.GatewayConfiguration) *k8sutil.GatewayManifestTemplateData {
	// Start with defaults
	data := k8sutil.NewGatewayManifestTemplateData(gatewayConfig.Name)

	// Populate from spec
	if gatewayConfig.Spec.Infrastructure != nil {
		infra := gatewayConfig.Spec.Infrastructure

		if infra.Replicas != nil {
			data.Replicas = *infra.Replicas
		}

		if infra.Image != "" {
			data.GatewayImage = infra.Image
		}

		if infra.RouterImage != "" {
			data.RouterImage = infra.RouterImage
		}

		if infra.Resources != nil {
			data.Resources = &k8sutil.ResourceRequirements{}

			if infra.Resources.Requests != nil {
				data.Resources.Requests = &k8sutil.ResourceList{}
				if cpu := infra.Resources.Requests.Cpu(); cpu != nil {
					data.Resources.Requests.CPU = cpu.String()
				}
				if mem := infra.Resources.Requests.Memory(); mem != nil {
					data.Resources.Requests.Memory = mem.String()
				}
			}

			if infra.Resources.Limits != nil {
				data.Resources.Limits = &k8sutil.ResourceList{}
				if cpu := infra.Resources.Limits.Cpu(); cpu != nil {
					data.Resources.Limits.CPU = cpu.String()
				}
				if mem := infra.Resources.Limits.Memory(); mem != nil {
					data.Resources.Limits.Memory = mem.String()
				}
			}
		}

		if infra.NodeSelector != nil {
			data.NodeSelector = infra.NodeSelector
		}

		if infra.Tolerations != nil {
			data.Tolerations = infra.Tolerations
		}

		if infra.Affinity != nil {
			data.Affinity = infra.Affinity
		}
	}

	// Populate control plane configuration
	if gatewayConfig.Spec.ControlPlane != nil {
		cp := gatewayConfig.Spec.ControlPlane

		if cp.Host != "" {
			data.ControlPlaneHost = cp.Host
		}

		if cp.TokenSecretRef != nil {
			data.ControlPlaneTokenSecret = &k8sutil.SecretReference{
				Name: cp.TokenSecretRef.Name,
				Key:  cp.TokenSecretRef.Key,
			}
		}
	}

	// Populate storage configuration
	if gatewayConfig.Spec.Storage != nil {
		storage := gatewayConfig.Spec.Storage

		if storage.Type != "" {
			data.StorageType = storage.Type
		}

		if storage.SQLitePath != "" {
			data.StorageSQLitePath = storage.SQLitePath
		}
	}

	return data
}

// registerGateway registers the gateway in the in-memory registry
func (r *GatewayConfigurationReconciler) registerGateway(gatewayConfig *apiv1.GatewayConfiguration) {
	namespace := gatewayConfig.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// Create gateway info for registry
	gatewayInfo := &registry.GatewayInfo{
		Name:             gatewayConfig.Name,
		Namespace:        namespace,
		GatewayClassName: gatewayConfig.Spec.GatewayClassName,
		APISelector:      &gatewayConfig.Spec.APISelector,
		// The gateway controller service name follows the pattern: <gateway-name>-gateway-controller
		ServiceName: gatewayConfig.Name + "-gateway-controller",
		ServicePort: 9090, // Default gateway controller API port
	}

	if gatewayConfig.Spec.ControlPlane != nil {
		gatewayInfo.ControlPlaneHost = gatewayConfig.Spec.ControlPlane.Host
	}

	// Register in the global registry
	registry.GetGatewayRegistry().Register(gatewayInfo)
}

// countSelectedAPIs returns the number of APIConfigurations that match the gateway selector
func (r *GatewayConfigurationReconciler) countSelectedAPIs(ctx context.Context, gatewayConfig *apiv1.GatewayConfiguration) (int, error) {
	apiSelector := selector.NewAPISelector(r.Client)
	apis, err := apiSelector.SelectAPIsForGateway(ctx, gatewayConfig)
	if err != nil {
		return 0, err
	}
	return len(apis), nil
}

// evaluateGatewayReadiness inspects the gateway deployments and reports readiness status
func (r *GatewayConfigurationReconciler) evaluateGatewayReadiness(ctx context.Context, gatewayConfig *apiv1.GatewayConfiguration) (bool, string, error) {
	namespace := gatewayConfig.Namespace
	if namespace == "" {
		namespace = "default"
	}

	deployments := &appsv1.DeploymentList{}
	if err := r.List(ctx, deployments, client.InNamespace(namespace), client.MatchingLabels(map[string]string{
		"app.kubernetes.io/name": gatewayConfig.Name,
	})); err != nil {
		return false, "", fmt.Errorf("failed to list gateway deployments: %w", err)
	}

	if len(deployments.Items) == 0 {
		return false, "Gateway workloads have not been created yet", nil
	}

	var pending []string
	for _, deploy := range deployments.Items {
		desired := int32(1)
		if deploy.Spec.Replicas != nil {
			desired = *deploy.Spec.Replicas
		}
		ready := deploy.Status.ReadyReplicas
		if ready < desired {
			pending = append(pending, fmt.Sprintf("%s %d/%d ready", deploy.Name, ready, desired))
		}
	}

	if len(pending) > 0 {
		return false, "Waiting for deployments to become ready: " + strings.Join(pending, ", "), nil
	}

	return true, "Gateway resources reconciled successfully", nil
}

type gatewayStatusUpdate struct {
	Phase             apiv1.GatewayPhase
	Condition         *metav1.Condition
	SelectedAPIs      *int
	AppliedGeneration *int64
}

// updateGatewayStatus patches the status of the GatewayConfiguration if it has changes
func (r *GatewayConfigurationReconciler) updateGatewayStatus(ctx context.Context, gateway *apiv1.GatewayConfiguration, update gatewayStatusUpdate) (bool, error) {
	base := gateway.DeepCopy()
	originalStatus := base.Status
	changed := false

	if update.Phase != "" && gateway.Status.Phase != update.Phase {
		gateway.Status.Phase = update.Phase
		changed = true
	}

	if update.SelectedAPIs != nil && gateway.Status.SelectedAPIs != *update.SelectedAPIs {
		gateway.Status.SelectedAPIs = *update.SelectedAPIs
		changed = true
	}

	if gateway.Status.ObservedGeneration != gateway.Generation {
		gateway.Status.ObservedGeneration = gateway.Generation
		changed = true
	}

	if update.AppliedGeneration != nil && gateway.Status.AppliedGeneration != *update.AppliedGeneration {
		gateway.Status.AppliedGeneration = *update.AppliedGeneration
		changed = true
	}

	if update.Condition != nil {
		cond := *update.Condition
		if cond.Type == "" {
			cond.Type = apiv1.GatewayConditionReady
		}
		cond.ObservedGeneration = gateway.Generation

		existing := meta.FindStatusCondition(gateway.Status.Conditions, cond.Type)

		needsUpdate := false
		if existing == nil {
			needsUpdate = true
			cond.LastTransitionTime = metav1.Now()
		} else {
			if existing.Status != cond.Status || existing.Reason != cond.Reason || existing.Message != cond.Message {
				needsUpdate = true
				cond.LastTransitionTime = metav1.Now()
			} else if existing.ObservedGeneration != cond.ObservedGeneration {
				needsUpdate = true
				cond.LastTransitionTime = existing.LastTransitionTime
			}
		}

		if needsUpdate {
			meta.SetStatusCondition(&gateway.Status.Conditions, cond)
			changed = true
		}
	}

	if !changed {
		gateway.Status = originalStatus
		return false, nil
	}

	now := metav1.Now()
	gateway.Status.LastUpdateTime = &now

	if err := r.Status().Patch(ctx, gateway, client.MergeFrom(base)); err != nil {
		return false, err
	}

	return true, nil
}

// deleteGatewayResources deletes all Kubernetes resources created for the gateway
func (r *GatewayConfigurationReconciler) deleteGatewayResources(ctx context.Context, owner *apiv1.GatewayConfiguration) error {
	log := log.FromContext(ctx)

	// Unregister from the gateway registry
	namespace := owner.Namespace
	if namespace == "" {
		namespace = "default"
	}
	registry.GetGatewayRegistry().Unregister(namespace, owner.Name)

	// Check if Helm-based deployment is enabled
	if r.Config.Gateway.UseHelm {
		return r.deleteGatewayWithHelm(ctx, owner, namespace)
	}

	// For template-based deployment, resources are cleaned up via owner references
	log.Info("Gateway resources will be cleaned up automatically via owner references")

	// Since we set owner references when applying the manifest template,
	// Kubernetes will automatically delete all child resources (Services, Deployments, PVCs, ConfigMaps)
	// when the GatewayConfiguration is deleted. No explicit cleanup needed.

	return nil
}

// deleteGatewayWithHelm uninstalls the Helm release for the gateway
func (r *GatewayConfigurationReconciler) deleteGatewayWithHelm(ctx context.Context, owner *apiv1.GatewayConfiguration, namespace string) error {
	log := log.FromContext(ctx)

	releaseName := helm.GetReleaseName(owner.Name)
	log.Info("Uninstalling Helm release", "release", releaseName, "namespace", namespace)

	// Create Helm client
	helmClient := helm.NewClient()

	// Uninstall the release
	err := helmClient.Uninstall(ctx, helm.UninstallOptions{
		ReleaseName: releaseName,
		Namespace:   namespace,
		Wait:        true,
		Timeout:     300, // 5 minutes
	})

	if err != nil {
		return fmt.Errorf("failed to uninstall Helm release: %w", err)
	}

	log.Info("Successfully uninstalled Helm release", "release", releaseName)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.GatewayConfiguration{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

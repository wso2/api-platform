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
	"math"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/k8sutil"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/selector"
)

// GatewayTrackingStatus represents the deployment status tracked in memory
type GatewayTrackingStatus string

const (
	GatewayTrackingStatusProcessing GatewayTrackingStatus = "Processing"
	GatewayTrackingStatusRetrying   GatewayTrackingStatus = "Retrying"
	GatewayTrackingStatusDeployed   GatewayTrackingStatus = "Deployed"
)

// GatewayTrackingEntry tracks the state of a Gateway deployment
type GatewayTrackingEntry struct {
	Generation    int64
	Status        GatewayTrackingStatus
	RetryCount    int
	LastRetryTime time.Time
	NextRetryTime time.Time
}

// GatewayTracker manages in-memory tracking of Gateway deployment states
// Entries persist until the Gateway CR is deleted
type GatewayTracker struct {
	mu      sync.RWMutex
	entries map[string]*GatewayTrackingEntry // key: "namespace/name"
}

// NewGatewayTracker creates a new Gateway tracker
func NewGatewayTracker() *GatewayTracker {
	return &GatewayTracker{
		entries: make(map[string]*GatewayTrackingEntry),
	}
}

// Get returns a tracking entry if it exists
func (t *GatewayTracker) Get(key string) (*GatewayTrackingEntry, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	entry, ok := t.entries[key]
	if !ok {
		return nil, false
	}
	// Return a copy to avoid race conditions
	entryCopy := *entry
	return &entryCopy, true
}

// Set adds or updates a tracking entry
func (t *GatewayTracker) Set(key string, entry *GatewayTrackingEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries[key] = entry
}

// Delete removes a tracking entry (only called when Gateway CR is deleted)
func (t *GatewayTracker) Delete(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}

const (
	gatewayFinalizerName = "gateway.api-platform.wso2.com/gateway-finalizer"
)

// GatewayReconciler reconciles a Gateway object
type GatewayReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Config         *config.OperatorConfig
	gatewayTracker *GatewayTracker
}

// NewGatewayReconciler creates a new GatewayReconciler
func NewGatewayReconciler(client client.Client, scheme *runtime.Scheme, cfg *config.OperatorConfig) *GatewayReconciler {
	return &GatewayReconciler{
		Client:         client,
		Scheme:         scheme,
		Config:         cfg,
		gatewayTracker: NewGatewayTracker(),
	}
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=gateways,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=gateways/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=gateways/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services;persistentvolumeclaims;configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Gateway instance
	gatewayConfig := &apiv1.Gateway{}
	if err := r.Get(ctx, req.NamespacedName, gatewayConfig); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// CR deleted, clean up tracker
			r.gatewayTracker.Delete(req.String())
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Gateway")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Gateway",
		"name", gatewayConfig.Name,
		"namespace", gatewayConfig.Namespace,
		"generation", gatewayConfig.Generation)

	// Handle deletion
	if !gatewayConfig.DeletionTimestamp.IsZero() {
		return r.reconcileGatewayDeletion(ctx, gatewayConfig)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(gatewayConfig, gatewayFinalizerName) {
		controllerutil.AddFinalizer(gatewayConfig, gatewayFinalizerName)
		if err := r.Update(ctx, gatewayConfig); err != nil {
			log.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to Gateway")
		return ctrl.Result{Requeue: true}, nil
	}

	// Get tracking key and current state
	trackingKey := req.String()
	crGeneration := gatewayConfig.Generation
	programmedCond := meta.FindStatusCondition(gatewayConfig.Status.Conditions, apiv1.GatewayConditionProgrammed)
	statusObservedGen := int64(0)
	if programmedCond != nil {
		statusObservedGen = programmedCond.ObservedGeneration
	}

	// Get current tracking entry
	trackingEntry, hasTrackingEntry := r.gatewayTracker.Get(trackingKey)

	// Decision logic based on generation comparison
	return r.decideAndProcess(ctx, gatewayConfig, trackingKey, crGeneration, statusObservedGen, programmedCond, trackingEntry, hasTrackingEntry)
}

// decideAndProcess implements the decision logic for processing reconcile events
func (r *GatewayReconciler) decideAndProcess(
	ctx context.Context,
	gatewayConfig *apiv1.Gateway,
	trackingKey string,
	crGeneration int64,
	statusObservedGen int64,
	programmedCond *metav1.Condition,
	trackingEntry *GatewayTrackingEntry,
	hasTrackingEntry bool,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Case 1: CR generation == status observed generation and Programmed=True
	// This means the Gateway is already deployed (or controller restarted after successful deploy)
	if crGeneration == statusObservedGen && programmedCond != nil && programmedCond.Status == metav1.ConditionTrue {
		// Already deployed - update tracker and skip
		r.gatewayTracker.Set(trackingKey, &GatewayTrackingEntry{
			Generation: crGeneration,
			Status:     GatewayTrackingStatusDeployed,
		})
		log.V(1).Info("Gateway already deployed, skipping",
			"name", gatewayConfig.Name,
			"generation", crGeneration)

		// Ensure gateway is registered in the in-memory registry (controller may have restarted)
		if err := r.registerGateway(ctx, gatewayConfig); err != nil {
			log.Error(err, "failed to register gateway in registry after restart; will retry")
			// Return error so reconcile is retried and registration can be re-attempted
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Case 2: CR generation > status observed generation
	// Need to deploy/update
	if crGeneration > statusObservedGen {
		// Check tracker
		if hasTrackingEntry {
			if trackingEntry.Generation == crGeneration {
				// Tracker has same generation - check status
				if trackingEntry.Status == GatewayTrackingStatusProcessing {
					// FALSE POSITIVE - already processing this generation (avoid concurrent processing)
					log.V(1).Info("Already processing this generation, skipping false positive event",
						"name", gatewayConfig.Name,
						"generation", crGeneration,
						"status", trackingEntry.Status)
					return ctrl.Result{}, nil
				}
				// If Retrying, let it proceed to retry the Helm deployment
				if trackingEntry.Status == GatewayTrackingStatusRetrying {
					log.Info("Retrying Gateway deployment",
						"name", gatewayConfig.Name,
						"generation", crGeneration,
						"retryCount", trackingEntry.RetryCount)
					return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration)
				}
				// If Deployed but status not updated yet, wait for status propagation
				if trackingEntry.Status == GatewayTrackingStatusDeployed {
					log.V(1).Info("Deployment completed but status not yet propagated, skipping",
						"name", gatewayConfig.Name,
						"generation", crGeneration)
					return ctrl.Result{}, nil
				}
			}

			if trackingEntry.Generation < crGeneration {
				// UPDATE - new generation to process
				log.Info("Processing Gateway update",
					"name", gatewayConfig.Name,
					"oldGeneration", trackingEntry.Generation,
					"newGeneration", crGeneration)
				return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration)
			}
		} else {
			// No tracking entry
			if crGeneration == 1 {
				// NEW Gateway - first generation
				log.Info("Processing new Gateway",
					"name", gatewayConfig.Name,
					"generation", crGeneration)
				return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration)
			}

			// Controller restart scenario:
			// No tracker entry but CR exists with generation > 1
			if statusObservedGen > 0 && statusObservedGen < crGeneration {
				// Controller restarted while processing an update
				// The previous generation was deployed, now need to deploy new generation
				log.Info("Controller restart detected - processing pending update",
					"name", gatewayConfig.Name,
					"statusObservedGen", statusObservedGen,
					"crGeneration", crGeneration)
				return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration)
			}

			if statusObservedGen == 0 && crGeneration > 1 {
				// Controller restarted while processing initial deployment that never completed
				// Treat as new deployment
				log.Info("Controller restart detected - retrying incomplete initial deployment",
					"name", gatewayConfig.Name,
					"crGeneration", crGeneration)
				return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration)
			}

			// statusObservedGen == crGeneration but condition is not True
			// Something failed before, retry
			if statusObservedGen == crGeneration {
				log.Info("Retrying previously failed deployment",
					"name", gatewayConfig.Name,
					"generation", crGeneration)
				return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration)
			}
		}
	}

	// Default: nothing to do
	log.V(1).Info("No action needed",
		"name", gatewayConfig.Name,
		"crGeneration", crGeneration,
		"statusObservedGen", statusObservedGen)
	return ctrl.Result{}, nil
}

// processGatewayDeployment handles the actual deployment of the Gateway
func (r *GatewayReconciler) processGatewayDeployment(
	ctx context.Context,
	gatewayConfig *apiv1.Gateway,
	trackingKey string,
	generation int64,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get existing entry to preserve retry count if retrying same generation
	existingEntry, hasExisting := r.gatewayTracker.Get(trackingKey)
	retryCount := 0
	if hasExisting && existingEntry.Generation == generation {
		retryCount = existingEntry.RetryCount
	}

	// Update tracker to Processing
	entry := &GatewayTrackingEntry{
		Generation: generation,
		Status:     GatewayTrackingStatusProcessing,
		RetryCount: retryCount,
	}
	r.gatewayTracker.Set(trackingKey, entry)

	// Set initial conditions (Accepted=True, Programmed=False/Pending)
	if retryCount == 0 {
		if err := r.setGatewayInitialConditions(ctx, gatewayConfig, generation); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Get Docker credentials
	dockerUsername, dockerPassword, err := r.getDockerHubCredentials(ctx)
	if err != nil {
		log.Error(err, "Failed to get Docker Hub credentials")
		// Continue without auth for public repos
	}

	// Count selected APIs
	selectedCount, err := r.countSelectedAPIs(ctx, gatewayConfig)
	if err != nil {
		log.Error(err, "failed to evaluate selected APIs")
		return r.handleGatewayDeploymentError(ctx, gatewayConfig, trackingKey, entry,
			fmt.Errorf("failed to evaluate selected APIs: %w", err), selectedCount)
	}

	// Apply the gateway manifest
	if err := r.applyGatewayManifest(ctx, gatewayConfig, dockerUsername, dockerPassword); err != nil {
		log.Error(err, "failed to apply gateway manifest")
		return r.handleGatewayDeploymentError(ctx, gatewayConfig, trackingKey, entry, err, selectedCount)
	}

	// Register the gateway in the registry
	if err := r.registerGateway(ctx, gatewayConfig); err != nil {
		log.Error(err, "failed to register gateway in registry")
		return r.handleGatewayDeploymentError(ctx, gatewayConfig, trackingKey, entry,
			fmt.Errorf("failed to register gateway: %w", err), selectedCount)
	}

	// Evaluate readiness
	ready, readinessMsg, err := r.evaluateGatewayReadiness(ctx, gatewayConfig)
	if err != nil {
		log.Error(err, "failed to evaluate gateway readiness")
		return r.handleGatewayDeploymentError(ctx, gatewayConfig, trackingKey, entry,
			fmt.Errorf("failed to evaluate readiness: %w", err), selectedCount)
	}

	if !ready {
		// Deployments not ready yet - update status and requeue
		entry.Status = GatewayTrackingStatusRetrying
		r.gatewayTracker.Set(trackingKey, entry)

		if err := r.updateGatewayProgrammedCondition(ctx, gatewayConfig, metav1.Condition{
			Type:               apiv1.GatewayConditionProgrammed,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: 0,
			Reason:             apiv1.GatewayProgrammedReasonPending,
			Message:            readinessMsg,
			LastTransitionTime: metav1.Now(),
		}, &selectedCount); err != nil {
			return ctrl.Result{}, err
		}

		log.Info("Waiting for gateway deployments to become ready", "message", readinessMsg)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Success - Gateway is ready
	return r.handleGatewayDeploymentSuccess(ctx, gatewayConfig, trackingKey, entry, selectedCount, readinessMsg)
}

// setGatewayInitialConditions sets the Accepted and initial Programmed conditions
func (r *GatewayReconciler) setGatewayInitialConditions(ctx context.Context, gatewayConfig *apiv1.Gateway, generation int64) error {
	base := gatewayConfig.DeepCopy()

	acceptedCond := metav1.Condition{
		Type:               apiv1.GatewayConditionAccepted,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: generation,
		Reason:             apiv1.GatewayAcceptedReasonAccepted,
		Message:            "Gateway configuration accepted",
		LastTransitionTime: metav1.Now(),
	}

	programmedCond := metav1.Condition{
		Type:               apiv1.GatewayConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: 0, // Not set until deployment succeeds
		Reason:             apiv1.GatewayProgrammedReasonPending,
		Message:            "Deployment in progress",
		LastTransitionTime: metav1.Now(),
	}

	meta.SetStatusCondition(&gatewayConfig.Status.Conditions, acceptedCond)
	meta.SetStatusCondition(&gatewayConfig.Status.Conditions, programmedCond)

	// Keep Phase for backward compatibility
	gatewayConfig.Status.Phase = apiv1.GatewayPhaseReconciling
	gatewayConfig.Status.ObservedGeneration = generation

	now := metav1.Now()
	gatewayConfig.Status.LastUpdateTime = &now

	return r.Status().Patch(ctx, gatewayConfig, client.MergeFrom(base))
}

// handleGatewayDeploymentSuccess handles successful deployment
func (r *GatewayReconciler) handleGatewayDeploymentSuccess(
	ctx context.Context,
	gatewayConfig *apiv1.Gateway,
	trackingKey string,
	entry *GatewayTrackingEntry,
	selectedCount int,
	readinessMsg string,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Gateway deployment succeeded", "gateway", gatewayConfig.Name)

	// Update tracker to Deployed
	entry.Status = GatewayTrackingStatusDeployed
	r.gatewayTracker.Set(trackingKey, entry)

	// Update status with success
	if err := r.updateGatewayProgrammedCondition(ctx, gatewayConfig, metav1.Condition{
		Type:               apiv1.GatewayConditionProgrammed,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: entry.Generation,
		Reason:             apiv1.GatewayProgrammedReasonProgrammed,
		Message:            readinessMsg,
		LastTransitionTime: metav1.Now(),
	}, &selectedCount); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleGatewayDeploymentError handles deployment errors
func (r *GatewayReconciler) handleGatewayDeploymentError(
	ctx context.Context,
	gatewayConfig *apiv1.Gateway,
	trackingKey string,
	entry *GatewayTrackingEntry,
	err error,
	selectedCount int,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	entry.RetryCount++
	entry.LastRetryTime = time.Now()

	// Check if max retries exceeded
	maxRetries := r.Config.Reconciliation.MaxRetryAttempts
	if maxRetries <= 0 {
		maxRetries = 10 // sensible default fallback
	}

	if entry.RetryCount >= maxRetries {
		log.Error(err, "Max retries exceeded",
			"gateway", gatewayConfig.Name,
			"retryCount", entry.RetryCount,
			"maxRetries", maxRetries)

		// Mark as deployed (failed) - keeps tracking but won't retry
		entry.Status = GatewayTrackingStatusDeployed
		r.gatewayTracker.Set(trackingKey, entry)

		// Update status with final failure
		if updateErr := r.updateGatewayProgrammedCondition(ctx, gatewayConfig, metav1.Condition{
			Type:               apiv1.GatewayConditionProgrammed,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: entry.Generation,
			Reason:             apiv1.GatewayProgrammedReasonDeploymentFailed,
			Message:            fmt.Sprintf("Max retries (%d) exceeded. Last error: %s", maxRetries, err.Error()),
			LastTransitionTime: metav1.Now(),
		}, &selectedCount); updateErr != nil {
			return ctrl.Result{}, updateErr
		}

		return ctrl.Result{}, nil
	}

	// Calculate backoff
	backoff := r.calculateBackoff(entry.RetryCount)
	entry.NextRetryTime = time.Now().Add(backoff)
	entry.Status = GatewayTrackingStatusRetrying
	r.gatewayTracker.Set(trackingKey, entry)

	log.Info("Deployment failed, scheduling retry",
		"gateway", gatewayConfig.Name,
		"retryCount", entry.RetryCount,
		"maxRetries", maxRetries,
		"nextRetryIn", backoff,
		"error", err.Error())

	if updateErr := r.updateGatewayProgrammedCondition(ctx, gatewayConfig, metav1.Condition{
		Type:               apiv1.GatewayConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: 0,
		Reason:             apiv1.GatewayProgrammedReasonRetrying,
		Message:            fmt.Sprintf("Deployment failed, retrying (attempt %d/%d): %s", entry.RetryCount, maxRetries, err.Error()),
		LastTransitionTime: metav1.Now(),
	}, &selectedCount); updateErr != nil {
		return ctrl.Result{}, updateErr
	}

	return ctrl.Result{RequeueAfter: backoff}, nil
}

// calculateBackoff calculates exponential backoff duration using operator configuration
func (r *GatewayReconciler) calculateBackoff(retryCount int) time.Duration {
	cfg := r.Config.Reconciliation

	initial := cfg.InitialBackoff
	max := cfg.MaxBackoffDuration

	// Fallback sensible defaults if config is not set
	if initial <= 0 {
		initial = 1 * time.Second
	}
	if max <= 0 {
		max = 60 * time.Second
	}

	backoff := initial * time.Duration(math.Pow(2, float64(retryCount-1)))
	if backoff > max {
		backoff = max
	}
	return backoff
}

// updateGatewayProgrammedCondition updates the Programmed condition and related status fields
func (r *GatewayReconciler) updateGatewayProgrammedCondition(ctx context.Context, gatewayConfig *apiv1.Gateway, cond metav1.Condition, selectedCount *int) error {
	// Re-fetch to get latest version
	latest := &apiv1.Gateway{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: gatewayConfig.Namespace, Name: gatewayConfig.Name}, latest); err != nil {
		return err
	}

	base := latest.DeepCopy()

	// Check if update is needed
	existing := meta.FindStatusCondition(latest.Status.Conditions, cond.Type)
	needsUpdate := false
	if existing != nil {
		if existing.Status == cond.Status &&
			existing.Reason == cond.Reason &&
			existing.ObservedGeneration == cond.ObservedGeneration &&
			existing.Message == cond.Message {
			needsUpdate = false
		} else {
			needsUpdate = true
			// Only update LastTransitionTime if status changed
			if existing.Status == cond.Status {
				cond.LastTransitionTime = existing.LastTransitionTime
			}
		}
	} else {
		needsUpdate = true
	}

	if !needsUpdate && selectedCount != nil && latest.Status.SelectedAPIs == *selectedCount {
		return nil
	}

	meta.SetStatusCondition(&latest.Status.Conditions, cond)

	// Update Phase for backward compatibility
	if cond.Status == metav1.ConditionTrue {
		latest.Status.Phase = apiv1.GatewayPhaseReady
		latest.Status.AppliedGeneration = cond.ObservedGeneration
	} else if cond.Reason == apiv1.GatewayProgrammedReasonDeploymentFailed {
		latest.Status.Phase = apiv1.GatewayPhaseFailed
	} else {
		latest.Status.Phase = apiv1.GatewayPhaseReconciling
	}

	if selectedCount != nil {
		latest.Status.SelectedAPIs = *selectedCount
	}

	now := metav1.Now()
	latest.Status.LastUpdateTime = &now

	return r.Status().Patch(ctx, latest, client.MergeFrom(base))
}

// reconcileGatewayDeletion handles Gateway CR deletion
func (r *GatewayReconciler) reconcileGatewayDeletion(ctx context.Context, gatewayConfig *apiv1.Gateway) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(gatewayConfig, gatewayFinalizerName) {
		return ctrl.Result{}, nil
	}

	// Update status to deleting
	base := gatewayConfig.DeepCopy()
	gatewayConfig.Status.Phase = apiv1.GatewayPhaseDeleting
	meta.SetStatusCondition(&gatewayConfig.Status.Conditions, metav1.Condition{
		Type:               apiv1.GatewayConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: gatewayConfig.Generation,
		Reason:             "Deleting",
		Message:            "Gateway is being deleted",
		LastTransitionTime: metav1.Now(),
	})
	if err := r.Status().Patch(ctx, gatewayConfig, client.MergeFrom(base)); err != nil {
		log.Error(err, "failed to update status during deletion")
	}

	// Perform cleanup
	if err := r.deleteGatewayResources(ctx, gatewayConfig); err != nil {
		log.Error(err, "failed to delete gateway resources")
		return ctrl.Result{}, err
	}

	// Remove from tracker
	trackingKey := types.NamespacedName{Namespace: gatewayConfig.Namespace, Name: gatewayConfig.Name}.String()
	r.gatewayTracker.Delete(trackingKey)

	// Remove finalizer: re-fetch latest object to avoid UID/resourceVersion precondition failures
	latest := &apiv1.Gateway{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: gatewayConfig.Namespace, Name: gatewayConfig.Name}, latest); err != nil {
		if apierrors.IsNotFound(err) {
			// already deleted - nothing to do
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to re-fetch Gateway before removing finalizer")
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(latest, gatewayFinalizerName)
	if err := r.Update(ctx, latest); err != nil {
		log.Error(err, "failed to remove finalizer")
		return ctrl.Result{}, err
	}

	log.Info("Successfully cleaned up gateway resources and removed finalizer")
	return ctrl.Result{}, nil
}

// applyGatewayManifest applies the gateway using Helm only
func (r *GatewayReconciler) applyGatewayManifest(ctx context.Context, owner *apiv1.Gateway, dockerUserName, dockerPassword string) error {
	namespace := owner.Namespace
	if namespace == "" {
		namespace = "default"
	}
	return r.deployGatewayWithHelm(ctx, owner, namespace, dockerUserName, dockerPassword)
}

// deployGatewayWithHelm deploys the gateway using Helm chart
func (r *GatewayReconciler) deployGatewayWithHelm(ctx context.Context, owner *apiv1.Gateway, namespace, dockerUserName, dockerPassword string) error {
	log := log.FromContext(ctx)

	// Prepare Helm values based on ConfigRef
	var valuesYAML string
	var valuesFilePath string

	if owner.Spec.ConfigRef != nil {
		// Use custom ConfigMap values
		configMapValues, err := r.getConfigMapValues(ctx, owner.Spec.ConfigRef.Name, namespace)
		if err != nil {
			return fmt.Errorf("failed to get ConfigMap values: %w", err)
		}
		valuesYAML = configMapValues
		log.Info("Using custom Helm values from ConfigMap",
			"configMap", owner.Spec.ConfigRef.Name,
			"namespace", namespace)
	} else {
		// Use default mounted config
		valuesFilePath = r.Config.Gateway.HelmValuesFilePath
		log.Info("Using default Helm values file",
			"values_file", valuesFilePath)
	}

	log.Info("Deploying gateway using Helm",
		"chart", r.Config.Gateway.HelmChartPath,
		"chart_name", r.Config.Gateway.HelmChartName,
		"version", r.Config.Gateway.HelmChartVersion,
		"namespace", namespace,
		"release_name", helm.GetReleaseName(owner.Name),
	)

	// Create Helm client
	helmClient, err := helm.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Helm client: %w", err)
	}

	// Generate release name
	releaseName := helm.GetReleaseName(owner.Name)

	// Install or upgrade the chart
	err = helmClient.InstallOrUpgrade(ctx, helm.InstallOrUpgradeOptions{
		ReleaseName:     releaseName,
		Namespace:       namespace,
		ChartName:       r.Config.Gateway.HelmChartName,
		ValuesYAML:      valuesYAML,     // Custom values from ConfigMap
		ValuesFilePath:  valuesFilePath, // Default values file
		Version:         r.Config.Gateway.HelmChartVersion,
		CreateNamespace: true,
		Wait:            true,
		Timeout:         300, // 5 minutes
		Username:        dockerUserName,
		Password:        dockerPassword,
		Insecure:        r.Config.Gateway.InsecureRegistry,
		PlainHTTP:       r.Config.Gateway.PlainHTTP,
	})

	if err != nil {
		return fmt.Errorf("failed to install/upgrade Helm chart: %w", err)
	}

	log.Info("Successfully deployed gateway with Helm", "release", releaseName)
	return nil
}

// getConfigMapValues retrieves Helm values from a ConfigMap
func (r *GatewayReconciler) getConfigMapValues(ctx context.Context, configMapName, namespace string) (string, error) {
	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      configMapName,
		Namespace: namespace,
	}, configMap); err != nil {
		return "", fmt.Errorf("failed to get ConfigMap %s/%s: %w", namespace, configMapName, err)
	}

	// Look for "values.yaml" key in the ConfigMap
	valuesYAML, ok := configMap.Data["values.yaml"]
	if !ok {
		return "", fmt.Errorf("ConfigMap %s/%s does not contain 'values.yaml' key", namespace, configMapName)
	}

	if valuesYAML == "" {
		return "", fmt.Errorf("'values.yaml' key in ConfigMap %s/%s is empty", namespace, configMapName)
	}

	return valuesYAML, nil
}

// buildTemplateData creates template data from Gateway spec
func (r *GatewayReconciler) buildTemplateData(gatewayConfig *apiv1.Gateway) *k8sutil.GatewayManifestTemplateData {
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

// registerGateway registers the gateway in the in-memory registry by discovering the actual service
func (r *GatewayReconciler) registerGateway(ctx context.Context, gatewayConfig *apiv1.Gateway) error {
	log := log.FromContext(ctx)

	namespace := gatewayConfig.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// Discover the gateway controller service by looking for services with the correct labels
	// The service is created by the Helm chart with app.kubernetes.io/component: controller
	// and app.kubernetes.io/instance: <release-name>
	releaseName := fmt.Sprintf("%s-gateway", gatewayConfig.Name) // This matches helm.GetReleaseName()

	serviceList := &corev1.ServiceList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/instance":  releaseName,
			"app.kubernetes.io/component": "controller",
		},
	}

	if err := r.List(ctx, serviceList, listOpts...); err != nil {
		return fmt.Errorf("failed to list gateway controller services: %w", err)
	}

	if len(serviceList.Items) == 0 {
		return fmt.Errorf("gateway controller service not found for release %s in namespace %s", releaseName, namespace)
	}

	if len(serviceList.Items) > 1 {
		log.Info("Multiple gateway controller services found; using the first one", "count", len(serviceList.Items))
	}

	service := &serviceList.Items[0]
	log.Info("Discovered gateway controller service", "serviceName", service.Name, "namespace", service.Namespace)

	// Find the REST API port (port 9090)
	var restPort int32 = 9090
	for _, port := range service.Spec.Ports {
		if port.Name == "rest" || port.Port == 9090 {
			restPort = port.Port
			break
		}
	}

	// Create gateway info for registry
	gatewayInfo := &registry.GatewayInfo{
		Name:             gatewayConfig.Name,
		Namespace:        namespace,
		GatewayClassName: gatewayConfig.Spec.GatewayClassName,
		APISelector:      &gatewayConfig.Spec.APISelector,
		ServiceName:      service.Name,
		ServicePort:      restPort,
	}

	if gatewayConfig.Spec.ControlPlane != nil {
		gatewayInfo.ControlPlaneHost = gatewayConfig.Spec.ControlPlane.Host
	}

	// Register in the global registry
	registry.GetGatewayRegistry().Register(gatewayInfo)
	log.Info("Successfully registered gateway in registry", "service", gatewayInfo.ServiceName, "port", gatewayInfo.ServicePort)

	return nil
}

// countSelectedAPIs returns the number of RestApis that match the gateway selector
func (r *GatewayReconciler) countSelectedAPIs(ctx context.Context, gatewayConfig *apiv1.Gateway) (int, error) {
	apiSelector := selector.NewAPISelector(r.Client)
	apis, err := apiSelector.SelectAPIsForGateway(ctx, gatewayConfig)
	if err != nil {
		return 0, err
	}
	return len(apis), nil
}

// evaluateGatewayReadiness inspects the gateway deployments and reports readiness status
func (r *GatewayReconciler) evaluateGatewayReadiness(ctx context.Context, gatewayConfig *apiv1.Gateway) (bool, string, error) {
	namespace := gatewayConfig.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// The deployments are created by the Helm chart with a release name pattern:
	// <gatewayName>-gateway (e.g., "cluster-gateway-gateway")
	// They have the label app.kubernetes.io/instance set to the release name
	releaseName := fmt.Sprintf("%s-gateway", gatewayConfig.Name)

	deployments := &appsv1.DeploymentList{}
	if err := r.List(ctx, deployments, client.InNamespace(namespace), client.MatchingLabels(map[string]string{
		"app.kubernetes.io/instance": releaseName,
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

// deleteGatewayResources deletes all Kubernetes resources created for the gateway
func (r *GatewayReconciler) deleteGatewayResources(ctx context.Context, owner *apiv1.Gateway) error {
	// Unregister from the gateway registry
	namespace := owner.Namespace
	if namespace == "" {
		namespace = "default"
	}
	registry.GetGatewayRegistry().Unregister(namespace, owner.Name)

	return r.deleteGatewayWithHelm(ctx, owner, namespace)
}

// deleteGatewayWithHelm uninstalls the Helm release for the gateway
func (r *GatewayReconciler) deleteGatewayWithHelm(ctx context.Context, owner *apiv1.Gateway, namespace string) error {
	log := log.FromContext(ctx)

	releaseName := helm.GetReleaseName(owner.Name)
	log.Info("Uninstalling Helm release", "release", releaseName, "namespace", namespace)

	// Create Helm client
	helmClient, err := helm.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Helm client: %w", err)
	}

	// Uninstall the release without waiting for resources to be deleted
	// This prevents deletion from hanging when pods are stuck (e.g., ImagePullBackOff)
	// Kubernetes will continue cleaning up resources in the background
	err = helmClient.Uninstall(ctx, helm.UninstallOptions{
		ReleaseName: releaseName,
		Namespace:   namespace,
		Wait:        false,
		Timeout:     60, // 1 minute (only applies to the Helm uninstall API call, not resource deletion)
	})

	if err != nil {
		return fmt.Errorf("failed to uninstall Helm release: %w", err)
	}

	log.Info("Successfully initiated Helm release uninstall", "release", releaseName)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: r.Config.Reconciliation.MaxConcurrentReconciles}
	if opts.MaxConcurrentReconciles <= 0 {
		opts.MaxConcurrentReconciles = 1
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(opts).
		For(&apiv1.Gateway{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

// getDockerHubCredentials retrieves Docker Hub credentials from a Kubernetes Secret
func (r *GatewayReconciler) getDockerHubCredentials(ctx context.Context) (string, string, error) {
	secret := &corev1.Secret{}

	// Use configured secret reference or skip if not configured
	if r.Config.Gateway.RegistryCredentialsSecret == nil {
		return "", "", nil
	}

	secretRef := r.Config.Gateway.RegistryCredentialsSecret
	secretName := secretRef.Name
	secretNamespace := secretRef.Namespace

	if secretName == "" || secretNamespace == "" {
		return "", "", nil
	}

	usernameKey := secretRef.UsernameKey
	if usernameKey == "" {
		usernameKey = "username"
	}
	passwordKey := secretRef.PasswordKey
	if passwordKey == "" {
		passwordKey = "password"
	}

	err := r.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: secretNamespace,
	}, secret)
	if err != nil {
		return "", "", fmt.Errorf("failed to get secret %s/%s: %w", secretNamespace, secretName, err)
	}

	username := string(secret.Data[usernameKey])
	password := string(secret.Data[passwordKey])

	if username == "" || password == "" {
		return "", "", fmt.Errorf("username or password is empty in secret %s/%s", secretNamespace, secretName)
	}

	return username, password, nil
}

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
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/auth"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helmgateway"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/selector"
)

// GatewayTrackingStatus represents the deployment status tracked in memory
type GatewayTrackingStatus string

const (
	GatewayTrackingStatusProcessing    GatewayTrackingStatus = "Processing"
	GatewayTrackingStatusRetrying      GatewayTrackingStatus = "Retrying"
	GatewayTrackingStatusDeployed      GatewayTrackingStatus = "Deployed"
	GatewayTrackingStatusConfigChanged GatewayTrackingStatus = "ConfigChanged"
)

// GatewayTrackingEntry tracks the state of an APIGateway deployment
type GatewayTrackingEntry struct {
	Generation    int64
	Status        GatewayTrackingStatus
	RetryCount    int
	LastRetryTime time.Time
	NextRetryTime time.Time
}

// GatewayTracker manages in-memory tracking of APIGateway deployment states
// Entries persist until the APIGateway CR is deleted
type GatewayTracker struct {
	mu      sync.RWMutex
	entries map[string]*GatewayTrackingEntry // key: "namespace/name"
}

// NewGatewayTracker creates a new APIGateway tracker
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

// Delete removes a tracking entry (only called when APIGateway CR is deleted)
func (t *GatewayTracker) Delete(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}

const (
	apigatewayFinalizerName = "gateway.api-platform.wso2.com/apigateway-finalizer"
)

// GatewayReconciler reconciles an APIGateway object
type GatewayReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Config         *config.OperatorConfig
	gatewayTracker *GatewayTracker
	Logger         *zap.Logger
}

// NewGatewayReconciler creates a new GatewayReconciler
func NewGatewayReconciler(client client.Client, scheme *runtime.Scheme, cfg *config.OperatorConfig, logger *zap.Logger) *GatewayReconciler {
	return &GatewayReconciler{
		Client:         client,
		Scheme:         scheme,
		Config:         cfg,
		gatewayTracker: NewGatewayTracker(),
		Logger:         logger,
	}
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=apigateways,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=apigateways/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=apigateways/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services;persistentvolumeclaims;configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Logger.With(zap.String("controller", "APIGateway"), zap.String("namespace", req.Namespace), zap.String("name", req.Name))

	// Fetch the APIGateway instance
	gatewayConfig := &apiv1.APIGateway{}
	if err := r.Get(ctx, req.NamespacedName, gatewayConfig); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error("unable to fetch APIGateway", zap.Error(err))
		return ctrl.Result{}, err
	}

	log.Info("Reconciling APIGateway",
		zap.String("name", gatewayConfig.Name),
		zap.String("namespace", gatewayConfig.Namespace),
		zap.Int64("generation", gatewayConfig.Generation))

	// Handle deletion
	if !gatewayConfig.DeletionTimestamp.IsZero() {
		return r.reconcileGatewayDeletion(ctx, gatewayConfig)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(gatewayConfig, apigatewayFinalizerName) {
		controllerutil.AddFinalizer(gatewayConfig, apigatewayFinalizerName)
		if err := r.Update(ctx, gatewayConfig); err != nil {
			log.Error("failed to add finalizer", zap.Error(err))
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to APIGateway")
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
	gatewayConfig *apiv1.APIGateway,
	trackingKey string,
	crGeneration int64,
	statusObservedGen int64,
	programmedCond *metav1.Condition,
	trackingEntry *GatewayTrackingEntry,
	hasTrackingEntry bool,
) (ctrl.Result, error) {
	log := r.Logger.With(zap.String("controller", "APIGateway"), zap.String("name", gatewayConfig.Name))

	// Calculate current config hash
	currentConfigHash := ""
	if gatewayConfig.Spec.ConfigRef != nil {
		values, err := configMapValuesYAML(ctx, r.Client, gatewayConfig.Spec.ConfigRef.Name, gatewayConfig.Namespace)
		if err != nil {
			// If we can't read config map, it might be transient or deleted
			// We should probably fail to deploy/reconcile
			// But if we are already deployed, maybe we just log error?
			// For now, let's treat it as error so we can retry
			return ctrl.Result{}, fmt.Errorf("failed to get config map values: %w", err)
		}
		currentConfigHash = auth.CalculateConfigHash(values)
	}

	// Check if config has changed
	configChanged := currentConfigHash != gatewayConfig.Status.ConfigHash

	// Case 1: CR generation == status observed generation and Programmed=True
	// This means the Gateway is already deployed (or controller restarted after successful deploy)
	if crGeneration == statusObservedGen && programmedCond != nil && programmedCond.Status == metav1.ConditionTrue {
		// If config changed, we need to redeploy
		if configChanged {
			log.Info("Configuration changed, triggering redeployment",
				zap.String("oldHash", gatewayConfig.Status.ConfigHash),
				zap.String("newHash", currentConfigHash))

			// Update status to Programmed=False to trigger a new reconciliation loop
			// This effectively resets the state machine to "Not Ready"
			// The next reconciliation will see Programmed=False and trigger processGatewayDeployment

			// Reset tracking status to ConfigChanged with current generation.
			// This explicitly signals that we are pending a deployment due to config change.
			r.gatewayTracker.Set(trackingKey, &GatewayTrackingEntry{
				Generation: crGeneration,
				Status:     GatewayTrackingStatusConfigChanged,
				RetryCount: 0,
			})

			if err := r.updateGatewayProgrammedCondition(ctx, gatewayConfig, metav1.Condition{
				Type:               apiv1.GatewayConditionProgrammed,
				Status:             metav1.ConditionFalse,
				Reason:             "ConfigChanged",
				Message:            "Configuration changed, redeployment pending",
				LastTransitionTime: metav1.Now(),
			}, nil, ""); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}

		// Already deployed - update tracker and skip
		r.gatewayTracker.Set(trackingKey, &GatewayTrackingEntry{
			Generation: crGeneration,
			Status:     GatewayTrackingStatusDeployed,
		})
		log.Debug("APIGateway already deployed, skipping",
			zap.String("name", gatewayConfig.Name),
			zap.Int64("generation", crGeneration))

		// Ensure gateway is registered in the in-memory registry (controller may have restarted)
		if err := r.registerAPIGateway(ctx, gatewayConfig); err != nil {
			log.Error("failed to register gateway in registry after restart; will retry", zap.Error(err))
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
					log.Debug("Already processing this generation, skipping false positive event",
						zap.String("name", gatewayConfig.Name),
						zap.Int64("generation", crGeneration),
						zap.String("status", string(trackingEntry.Status)))
					return ctrl.Result{}, nil
				}
				// If Retrying, let it proceed to retry the Helm deployment
				if trackingEntry.Status == GatewayTrackingStatusRetrying {
					log.Info("Retrying APIGateway deployment",
						zap.String("name", gatewayConfig.Name),
						zap.Int64("generation", crGeneration),
						zap.Int("retryCount", trackingEntry.RetryCount))
					return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration, currentConfigHash)
				}
				// If Deployed but status not updated yet, wait for status propagation
				if trackingEntry.Status == GatewayTrackingStatusDeployed {
					log.Debug("Deployment completed but status not yet propagated, skipping",
						zap.String("name", gatewayConfig.Name),
						zap.Int64("generation", crGeneration))
					return ctrl.Result{}, nil
				}
				// If ConfigChanged, proceed to redeploy
				if trackingEntry.Status == GatewayTrackingStatusConfigChanged {
					log.Info("Processing APIGateway config change redeployment",
						zap.String("name", gatewayConfig.Name),
						zap.Int64("generation", crGeneration))
					return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration, currentConfigHash)
				}
			}

			if trackingEntry.Generation < crGeneration {
				// UPDATE - new generation to process
				log.Info("Processing APIGateway update",
					zap.String("name", gatewayConfig.Name),
					zap.Int64("oldGeneration", trackingEntry.Generation),
					zap.Int64("newGeneration", crGeneration))
				return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration, currentConfigHash)
			}
		} else {
			// No tracking entry
			if crGeneration == 1 {
				// NEW APIGateway - first generation
				log.Info("Processing new Gateway",
					zap.String("name", gatewayConfig.Name),
					zap.Int64("generation", crGeneration))
				return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration, currentConfigHash)
			}

			// Controller restart scenario:
			// No tracker entry but CR exists with generation > 1
			if statusObservedGen > 0 && statusObservedGen < crGeneration {
				// Controller restarted while processing an update
				// The previous generation was deployed, now need to deploy new generation
				log.Info("Controller restart detected - processing pending update",
					zap.String("name", gatewayConfig.Name),
					zap.Int64("statusObservedGen", statusObservedGen),
					zap.Int64("crGeneration", crGeneration))
				return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration, currentConfigHash)
			}

			if statusObservedGen == 0 && crGeneration > 1 {
				// Controller restarted while processing initial deployment that never completed
				// Treat as new deployment
				log.Info("Controller restart detected - retrying incomplete initial deployment",
					zap.String("name", gatewayConfig.Name),
					zap.Int64("crGeneration", crGeneration))
				return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration, currentConfigHash)
			}

			// statusObservedGen == crGeneration but condition is not True
			// Something failed before, retry
			if statusObservedGen == crGeneration {
				log.Info("Retrying previously failed deployment",
					zap.String("name", gatewayConfig.Name),
					zap.Int64("generation", crGeneration))
				return r.processGatewayDeployment(ctx, gatewayConfig, trackingKey, crGeneration, currentConfigHash)
			}
		}
	}

	// Default: nothing to do
	log.Debug("No action needed",
		zap.String("name", gatewayConfig.Name),
		zap.Int64("crGeneration", crGeneration),
		zap.Int64("statusObservedGen", statusObservedGen))
	return ctrl.Result{}, nil
}

// processGatewayDeployment handles the actual deployment of the Gateway
func (r *GatewayReconciler) processGatewayDeployment(
	ctx context.Context,
	gatewayConfig *apiv1.APIGateway,
	trackingKey string,
	generation int64,
	configHash string,
) (ctrl.Result, error) {
	log := r.Logger.With(zap.String("controller", "APIGateway"), zap.String("name", gatewayConfig.Name))

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
		log.Error("Failed to get Docker Hub credentials", zap.Error(err))
		// Continue without auth for public repos
	}

	// Count selected APIs
	selectedCount, err := r.countSelectedAPIs(ctx, gatewayConfig)
	if err != nil {
		log.Error("failed to evaluate selected APIs", zap.Error(err))
		return r.handleGatewayDeploymentError(ctx, gatewayConfig, trackingKey, entry,
			fmt.Errorf("failed to evaluate selected APIs: %w", err), selectedCount)
	}

	// Apply the gateway manifest
	if err := r.applyGatewayManifest(ctx, gatewayConfig, dockerUsername, dockerPassword); err != nil {
		log.Error("failed to apply gateway manifest", zap.Error(err))
		return r.handleGatewayDeploymentError(ctx, gatewayConfig, trackingKey, entry, err, selectedCount)
	}

	// Register the gateway in the registry
	if err := r.registerAPIGateway(ctx, gatewayConfig); err != nil {
		log.Error("failed to register gateway in registry", zap.Error(err))
		return r.handleGatewayDeploymentError(ctx, gatewayConfig, trackingKey, entry,
			fmt.Errorf("failed to register gateway: %w", err), selectedCount)
	}

	// Evaluate readiness
	ns := gatewayConfig.Namespace
	if ns == "" {
		ns = "default"
	}
	ready, readinessMsg, err := evaluateGatewayDeploymentsReady(ctx, r.Client, gatewayConfig.Name, ns)
	if err != nil {
		log.Error("failed to evaluate gateway readiness", zap.Error(err))
		return r.handleGatewayDeploymentError(ctx, gatewayConfig, trackingKey, entry,
			fmt.Errorf("failed to evaluate readiness: %w", err), selectedCount)
	}

	if !ready {
		entry.Status = GatewayTrackingStatusRetrying
		r.gatewayTracker.Set(trackingKey, entry)

		if err := r.updateGatewayProgrammedCondition(ctx, gatewayConfig, metav1.Condition{
			Type:               apiv1.GatewayConditionProgrammed,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: 0,
			Reason:             apiv1.GatewayProgrammedReasonPending,
			Message:            readinessMsg,
			LastTransitionTime: metav1.Now(),
		}, &selectedCount, ""); err != nil {
			return ctrl.Result{}, err
		}

		log.Info("Waiting for gateway deployments to become ready", zap.String("message", readinessMsg))
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Success - Gateway is ready
	return r.handleGatewayDeploymentSuccess(ctx, gatewayConfig, trackingKey, entry, selectedCount, readinessMsg, configHash)
}

// setGatewayInitialConditions sets the Accepted and initial Programmed conditions
func (r *GatewayReconciler) setGatewayInitialConditions(ctx context.Context, gatewayConfig *apiv1.APIGateway, generation int64) error {
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
	gatewayConfig *apiv1.APIGateway,
	trackingKey string,
	entry *GatewayTrackingEntry,
	selectedCount int,
	readinessMsg string,
	configHash string,
) (ctrl.Result, error) {
	log := r.Logger.With(zap.String("controller", "APIGateway"), zap.String("name", gatewayConfig.Name))
	log.Info("APIGateway deployment succeeded", zap.String("gateway", gatewayConfig.Name))

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
	}, &selectedCount, configHash); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleGatewayDeploymentError handles deployment errors
func (r *GatewayReconciler) handleGatewayDeploymentError(
	ctx context.Context,
	gatewayConfig *apiv1.APIGateway,
	trackingKey string,
	entry *GatewayTrackingEntry,
	err error,
	selectedCount int,
) (ctrl.Result, error) {
	log := r.Logger.With(zap.String("controller", "APIGateway"), zap.String("name", gatewayConfig.Name))

	entry.RetryCount++
	entry.LastRetryTime = time.Now()

	// Check if max retries exceeded
	maxRetries := r.Config.Reconciliation.MaxRetryAttempts
	if maxRetries <= 0 {
		maxRetries = 10 // sensible default fallback
	}

	if entry.RetryCount >= maxRetries {
		log.Error("Max retries exceeded",
			zap.Error(err),
			zap.String("gateway", gatewayConfig.Name),
			zap.Int("retryCount", entry.RetryCount),
			zap.Int("maxRetries", maxRetries))

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
		}, &selectedCount, ""); updateErr != nil {
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
		zap.String("gateway", gatewayConfig.Name),
		zap.Int("retryCount", entry.RetryCount),
		zap.Int("maxRetries", maxRetries),
		zap.Duration("nextRetryIn", backoff),
		zap.String("error", err.Error()))

	if updateErr := r.updateGatewayProgrammedCondition(ctx, gatewayConfig, metav1.Condition{
		Type:               apiv1.GatewayConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: 0,
		Reason:             apiv1.GatewayProgrammedReasonRetrying,
		Message:            fmt.Sprintf("Deployment failed, retrying (attempt %d/%d): %s", entry.RetryCount, maxRetries, err.Error()),
		LastTransitionTime: metav1.Now(),
	}, &selectedCount, ""); updateErr != nil {
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
func (r *GatewayReconciler) updateGatewayProgrammedCondition(ctx context.Context, gatewayConfig *apiv1.APIGateway, cond metav1.Condition, selectedCount *int, configHash string) error {
	// Re-fetch to get latest version
	latest := &apiv1.APIGateway{}
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

	if configHash != "" {
		latest.Status.ConfigHash = configHash
	}

	now := metav1.Now()
	latest.Status.LastUpdateTime = &now

	return r.Status().Patch(ctx, latest, client.MergeFrom(base))
}

// reconcileGatewayDeletion handles APIGateway CR deletion
func (r *GatewayReconciler) reconcileGatewayDeletion(ctx context.Context, gatewayConfig *apiv1.APIGateway) (ctrl.Result, error) {
	log := r.Logger.With(zap.String("controller", "APIGateway"), zap.String("name", gatewayConfig.Name))

	if !controllerutil.ContainsFinalizer(gatewayConfig, apigatewayFinalizerName) {
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
		Message:            "APIGateway is being deleted",
		LastTransitionTime: metav1.Now(),
	})
	if err := r.Status().Patch(ctx, gatewayConfig, client.MergeFrom(base)); err != nil {
		log.Error("failed to update status during deletion", zap.Error(err))
	}

	// Perform cleanup
	if err := r.deleteGatewayResources(ctx, gatewayConfig); err != nil {
		log.Error("failed to delete gateway resources", zap.Error(err))
		return ctrl.Result{}, err
	}

	// Remove from tracker
	trackingKey := types.NamespacedName{Namespace: gatewayConfig.Namespace, Name: gatewayConfig.Name}.String()
	r.gatewayTracker.Delete(trackingKey)

	// Remove finalizer: re-fetch latest object to avoid UID/resourceVersion precondition failures
	latest := &apiv1.APIGateway{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: gatewayConfig.Namespace, Name: gatewayConfig.Name}, latest); err != nil {
		if apierrors.IsNotFound(err) {
			// already deleted - nothing to do
			return ctrl.Result{}, nil
		}
		log.Error("failed to re-fetch APIGateway before removing finalizer", zap.Error(err))
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(latest, apigatewayFinalizerName)
	if err := r.Update(ctx, latest); err != nil {
		log.Error("failed to remove finalizer", zap.Error(err))
		return ctrl.Result{}, err
	}

	log.Info("Successfully cleaned up APIGateway resources and removed finalizer")
	return ctrl.Result{}, nil
}

// applyGatewayManifest applies the gateway using Helm only
func (r *GatewayReconciler) applyGatewayManifest(ctx context.Context, owner *apiv1.APIGateway, dockerUserName, dockerPassword string) error {
	namespace := owner.Namespace
	if namespace == "" {
		namespace = "default"
	}
	return r.deployGatewayWithHelm(ctx, owner, namespace, dockerUserName, dockerPassword)
}

// buildCRValuesOverlay generates a Helm values YAML overlay from CR infrastructure labels/annotations.
// The overlay sets commonLabels and commonAnnotations so they propagate to all gateway resources.
func buildCRValuesOverlay(owner *apiv1.APIGateway) (string, error) {
	if owner.Spec.Infrastructure == nil {
		return "", nil
	}
	infra := owner.Spec.Infrastructure
	if len(infra.Labels) == 0 && len(infra.Annotations) == 0 {
		return "", nil
	}
	overlay := map[string]interface{}{}
	if len(infra.Labels) > 0 {
		overlay["commonLabels"] = infra.Labels
	}
	if len(infra.Annotations) > 0 {
		overlay["commonAnnotations"] = infra.Annotations
	}
	out, err := yaml.Marshal(overlay)
	if err != nil {
		return "", fmt.Errorf("marshal CR values overlay: %w", err)
	}
	return string(out), nil
}

// deployGatewayWithHelm deploys the gateway using Helm chart
func (r *GatewayReconciler) deployGatewayWithHelm(ctx context.Context, owner *apiv1.APIGateway, namespace, dockerUserName, dockerPassword string) error {
	log := r.Logger.With(zap.String("controller", "APIGateway"), zap.String("name", owner.Name))

	valuesFilePath := r.Config.Gateway.HelmValuesFilePath
	var valuesYAML string

	// Build CR-derived values overlay from spec.infrastructure labels/annotations.
	// This is the lowest-priority override (ConfigMap can override these).
	crOverlay, err := buildCRValuesOverlay(owner)
	if err != nil {
		return fmt.Errorf("failed to build CR values overlay: %w", err)
	}

	if owner.Spec.ConfigRef != nil {
		configMapValues, err := configMapValuesYAML(ctx, r.Client, owner.Spec.ConfigRef.Name, namespace)
		if err != nil {
			return fmt.Errorf("failed to get ConfigMap values: %w", err)
		}
		// Merge order: CR overlay (base) → ConfigMap (higher priority).
		// MergeValuesYAML delegates to deepMergeValues, which performs a recursive
		// per-key merge: ConfigMap entries can add or override individual keys inside
		// nested maps like commonLabels/commonAnnotations, but cannot remove keys that
		// the CR infra overlay already set. If you need a wholesale map-level replace
		// (e.g. discard all CR-set labels and use only the ConfigMap's map), add
		// "commonLabels" or "commonAnnotations" to probeMergeReplaceKeys in
		// internal/helm/client.go, or use an equivalent replace mechanism.
		if crOverlay != "" {
			valuesYAML, err = helm.MergeValuesYAML(crOverlay, configMapValues)
			if err != nil {
				return fmt.Errorf("failed to merge CR overlay with ConfigMap values: %w", err)
			}
		} else {
			valuesYAML = configMapValues
		}
		log.Info("Merging Helm values: operator base file + CR overlay + APIGateway ConfigMap overlay",
			zap.String("configMap", owner.Spec.ConfigRef.Name),
			zap.String("namespace", namespace),
			zap.String("values_file", valuesFilePath))
	} else {
		valuesYAML = crOverlay
		log.Info("Using default Helm values file with CR overlay",
			zap.String("values_file", valuesFilePath))
	}

	log.Info("Deploying gateway using Helm",
		zap.String("chart_name", r.Config.Gateway.HelmChartName),
		zap.String("version", r.Config.Gateway.HelmChartVersion),
		zap.String("namespace", namespace))

	if err := helmgateway.InstallOrUpgrade(ctx, helmgateway.DeployInput{
		Logger:         r.Logger,
		Config:         r.Config,
		GatewayName:    owner.Name,
		Namespace:      namespace,
		ValuesYAML:     valuesYAML,
		ValuesFilePath: valuesFilePath,
		DockerUsername: dockerUserName,
		DockerPassword: dockerPassword,
	}); err != nil {
		return fmt.Errorf("failed to install/upgrade Helm chart: %w", err)
	}

	log.Info("Successfully deployed gateway with Helm", zap.String("release", helm.GetReleaseName(owner.Name)))
	return nil
}

// registerAPIGateway registers an APIGateway in the in-memory registry.
func (r *GatewayReconciler) registerAPIGateway(ctx context.Context, gatewayConfig *apiv1.APIGateway) error {
	log := r.Logger.With(zap.String("controller", "APIGateway"), zap.String("name", gatewayConfig.Name))
	namespace := gatewayConfig.Namespace
	if namespace == "" {
		namespace = "default"
	}
	var helmCM string
	if gatewayConfig.Spec.ConfigRef != nil {
		helmCM = gatewayConfig.Spec.ConfigRef.Name
	}
	cpHost := ""
	if gatewayConfig.Spec.ControlPlane != nil {
		cpHost = gatewayConfig.Spec.ControlPlane.Host
	}
	if err := registerGatewayInRegistry(ctx, r.Client, gatewayConfig.Name, namespace, &gatewayConfig.Spec.APISelector, cpHost, helmCM, false); err != nil {
		return err
	}
	log.Info("Successfully registered gateway in registry", zap.String("name", gatewayConfig.Name))
	return nil
}

// countSelectedAPIs returns the number of RestApis that match the gateway selector
func (r *GatewayReconciler) countSelectedAPIs(ctx context.Context, gatewayConfig *apiv1.APIGateway) (int, error) {
	apiSelector := selector.NewAPISelector(r.Client)
	apis, err := apiSelector.SelectAPIsForGateway(ctx, gatewayConfig)
	if err != nil {
		return 0, err
	}
	return len(apis), nil
}

// deleteGatewayResources deletes all Kubernetes resources created for the gateway
func (r *GatewayReconciler) deleteGatewayResources(ctx context.Context, owner *apiv1.APIGateway) error {
	// Unregister from the gateway registry
	namespace := owner.Namespace
	if namespace == "" {
		namespace = "default"
	}
	registry.GetGatewayRegistry().Unregister(namespace, owner.Name)

	return helmgateway.Uninstall(ctx, r.Logger, r.Config, owner.Name, namespace)
}

// enqueueGatewaysForConfigMap watches for ConfigMap changes and enqueues affected Gateways
func (r *GatewayReconciler) enqueueGatewaysForConfigMap(ctx context.Context, obj client.Object) []reconcile.Request {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return nil
	}

	logger := log.FromContext(ctx)

	// Find all Gateways that reference this ConfigMap
	gatewayList := &apiv1.APIGatewayList{}
	if err := r.List(ctx, gatewayList); err != nil {
		logger.Error(err, "failed to list Gateways for ConfigMap event",
			"configMap", configMap.Name,
			"namespace", configMap.Namespace)
		return nil
	}

	requests := make([]reconcile.Request, 0)

	for i := range gatewayList.Items {
		gateway := &gatewayList.Items[i]

		// Check if this Gateway references the ConfigMap
		if gateway.Spec.ConfigRef != nil &&
			gateway.Spec.ConfigRef.Name == configMap.Name &&
			gateway.Namespace == configMap.Namespace {

			logger.Info("Enqueuing APIGateway for ConfigMap change",
				"gateway", gateway.Name,
				"namespace", gateway.Namespace,
				"configMap", configMap.Name)

			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: gateway.Namespace,
					Name:      gateway.Name,
				},
			})
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{MaxConcurrentReconciles: r.Config.Reconciliation.MaxConcurrentReconciles}
	if opts.MaxConcurrentReconciles <= 0 {
		opts.MaxConcurrentReconciles = 1
	}

	// Predicate to only watch ConfigMap updates and deletes (not creates)
	configMapPred := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			// Don't trigger on create - Gateway will reconcile on its own creation
			return false
		},
		UpdateFunc: func(evt event.UpdateEvent) bool {
			// Trigger on any ConfigMap update
			return true
		},
		DeleteFunc: func(event event.DeleteEvent) bool {
			// Trigger on delete so Gateway can handle missing ConfigMap
			return true
		},
		GenericFunc: func(event event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(opts).
		For(&apiv1.APIGateway{}).
		Owns(&appsv1.Deployment{}).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueGatewaysForConfigMap),
			builder.WithPredicates(configMapPred)).
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

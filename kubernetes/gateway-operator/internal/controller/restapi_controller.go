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
	"net/http"
	"sync"
	"time"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/auth"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/selector"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"log/slog"
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
)

// APITrackingStatus represents the deployment status tracked in memory
type APITrackingStatus string

const (
	TrackingStatusProcessing APITrackingStatus = "Processing"
	TrackingStatusRetrying   APITrackingStatus = "Retrying"
	TrackingStatusDeployed   APITrackingStatus = "Deployed"
)

// APITrackingEntry tracks the state of an API deployment
type APITrackingEntry struct {
	Generation    int64
	Status        APITrackingStatus
	GatewayKey    string
	Fingerprint   string
	RetryCount    int
	LastRetryTime time.Time
	NextRetryTime time.Time
}

// APITracker manages in-memory tracking of API deployment states
// Entries persist until the API CR is deleted
type APITracker struct {
	mu      sync.RWMutex
	entries map[string]*APITrackingEntry // key: "namespace/name"
}

// NewAPITracker creates a new API tracker
func NewAPITracker() *APITracker {
	return &APITracker{
		entries: make(map[string]*APITrackingEntry),
	}
}

// Get returns a tracking entry if it exists
func (t *APITracker) Get(key string) (*APITrackingEntry, bool) {
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
func (t *APITracker) Set(key string, entry *APITrackingEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries[key] = entry
}

// Delete removes a tracking entry (only called when API CR is deleted)
func (t *APITracker) Delete(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}

// Retry configuration
const (
	apiFinalizerName = "gateway.api-platform.wso2.com/api-finalizer"
)

// RestApiReconciler reconciles a RestApi object
type RestApiReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Config              *config.OperatorConfig
	apiTracker          *APITracker
	valueFromRefIndexMu sync.RWMutex
	valueFromRefIndex   map[string]map[types.NamespacedName]struct{}
	restAPIValueFromRef map[types.NamespacedName]map[string]struct{}
	Logger              *slog.Logger
}

// NewRestApiReconciler creates a new RestApiReconciler
func NewRestApiReconciler(client client.Client, scheme *runtime.Scheme, cfg *config.OperatorConfig, logger *slog.Logger) *RestApiReconciler {
	return &RestApiReconciler{
		Client:              client,
		Scheme:              scheme,
		Config:              cfg,
		apiTracker:          NewAPITracker(),
		valueFromRefIndex:   make(map[string]map[types.NamespacedName]struct{}),
		restAPIValueFromRef: make(map[types.NamespacedName]map[string]struct{}),
		Logger:              logger,
	}
}

//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=restapis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=restapis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=restapis/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RestApiReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Logger.With(slog.String("controller", "RestApi"), slog.String("namespace", req.Namespace), slog.String("name", req.Name))

	// Fetch the RestApi CR
	apiConfig := &apiv1.RestApi{}
	if err := r.Get(ctx, req.NamespacedName, apiConfig); err != nil {
		if apierrors.IsNotFound(err) {
			// CR deleted, clean up tracker
			r.apiTracker.Delete(req.String())
			r.removeRestAPIFromValueFromIndex(req.NamespacedName)
			return ctrl.Result{}, nil
		}
		log.Error("unable to fetch RestApi", slog.Any("error", err))
		return ctrl.Result{}, err
	}
	r.upsertRestAPIValueFromIndex(apiConfig)

	log.Info("Reconciling RestApi",
		slog.String("name", apiConfig.Name),
		slog.String("namespace", apiConfig.Namespace),
		slog.Int64("generation", apiConfig.Generation))

	// Handle deletion
	if !apiConfig.DeletionTimestamp.IsZero() {
		return r.reconcileAPIDeletion(ctx, apiConfig)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(apiConfig, apiFinalizerName) {
		controllerutil.AddFinalizer(apiConfig, apiFinalizerName)
		if err := r.Update(ctx, apiConfig); err != nil {
			log.Error("failed to add finalizer", slog.Any("error", err))
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Get tracking key and current state
	trackingKey := req.String()
	crGeneration := apiConfig.Generation
	programmedCond := meta.FindStatusCondition(apiConfig.Status.Conditions, apiv1.APIConditionProgrammed)
	statusObservedGen := int64(0)
	if programmedCond != nil {
		statusObservedGen = programmedCond.ObservedGeneration
	}

	// Get current tracking entry
	trackingEntry, hasTrackingEntry := r.apiTracker.Get(trackingKey)

	// Decision logic based on generation comparison
	return r.decideAndProcess(ctx, apiConfig, trackingKey, crGeneration, statusObservedGen, programmedCond, trackingEntry, hasTrackingEntry)
}

// decideAndProcess implements the decision logic for processing reconcile events
func (r *RestApiReconciler) decideAndProcess(
	ctx context.Context,
	apiConfig *apiv1.RestApi,
	trackingKey string,
	crGeneration int64,
	statusObservedGen int64,
	programmedCond *metav1.Condition,
	trackingEntry *APITrackingEntry,
	hasTrackingEntry bool,
) (ctrl.Result, error) {
	log := r.Logger.With(slog.String("controller", "RestApi"), slog.String("name", apiConfig.Name))

	// Case 1: CR generation == status observed generation
	if crGeneration == statusObservedGen {
		fp, err := computeRestApiPolicyValueFromFingerprint(ctx, r.Client, apiConfig.Namespace, &apiConfig.Spec)
		if err != nil {
			entry := &APITrackingEntry{Generation: crGeneration, Status: TrackingStatusProcessing}
			if trackingEntry != nil {
				entry.RetryCount = trackingEntry.RetryCount
				entry.GatewayKey = trackingEntry.GatewayKey
				entry.NextRetryTime = trackingEntry.NextRetryTime
			}
			return r.handleDeploymentError(ctx, apiConfig, trackingKey, entry, err)
		}
		if fp != restApiPolicyValueFromAnnotation(apiConfig) {
			log.Info("RestApi policy valueFrom backing fingerprint changed; redeploying",
				slog.String("name", apiConfig.Name),
				slog.Int64("generation", crGeneration))
			rc := 0
			if trackingEntry != nil {
				rc = trackingEntry.RetryCount
			}
			r.apiTracker.Set(trackingKey, &APITrackingEntry{
				Generation: crGeneration,
				Status:     TrackingStatusRetrying,
				RetryCount: rc,
			})
			return r.processDeployment(ctx, apiConfig, trackingKey, crGeneration)
		}

		// Already deployed - update tracker and skip
		if programmedCond != nil && programmedCond.Status == metav1.ConditionTrue {
			r.apiTracker.Set(trackingKey, &APITrackingEntry{
				Generation: crGeneration,
				Status:     TrackingStatusDeployed,
			})
			log.Debug("API already deployed, skipping",
				slog.String("name", apiConfig.Name),
				slog.Int64("generation", crGeneration))
			return ctrl.Result{}, nil
		}
	}

	// Case 2: CR generation > status observed generation
	// Need to deploy/update
	if crGeneration > statusObservedGen {
		// Check tracker
		if hasTrackingEntry {
			if trackingEntry.Generation == crGeneration {
				// Tracker has same generation - check status
				if trackingEntry.Status == TrackingStatusProcessing {
					// FALSE POSITIVE - already processing this generation (avoid concurrent processing)
					log.Debug("Already processing this generation, skipping false positive event",
						slog.String("name", apiConfig.Name),
						slog.Int64("generation", crGeneration),
						slog.String("status", string(trackingEntry.Status)))
					return ctrl.Result{}, nil
				}
				// If Retrying, let it proceed to retry the API deployment
				if trackingEntry.Status == TrackingStatusRetrying {
					log.Info("Retrying API deployment",
						slog.String("name", apiConfig.Name),
						slog.Int64("generation", crGeneration),
						slog.Int("retryCount", trackingEntry.RetryCount))
					return r.processDeployment(ctx, apiConfig, trackingKey, crGeneration)
				}
				// If Deployed but status not updated yet, wait for status propagation
				if trackingEntry.Status == TrackingStatusDeployed {
					log.Debug("Deployment completed but status not yet propagated, skipping",
						slog.String("name", apiConfig.Name),
						slog.Int64("generation", crGeneration))
					return ctrl.Result{}, nil
				}
			}

			if trackingEntry.Generation < crGeneration {
				// UPDATE - new generation to process
				log.Info("Processing API update",
					slog.String("name", apiConfig.Name),
					slog.Int64("oldGeneration", trackingEntry.Generation),
					slog.Int64("newGeneration", crGeneration))
				return r.processDeployment(ctx, apiConfig, trackingKey, crGeneration)
			}
		} else {
			// No tracking entry
			log.Info("Processing API", slog.String("name", apiConfig.Name), slog.Int64("generation", crGeneration))
			return r.processDeployment(ctx, apiConfig, trackingKey, crGeneration)
		}
	}

	// Default: nothing to do
	log.Debug("No action needed",
		slog.String("name", apiConfig.Name),
		slog.Int64("crGeneration", crGeneration),
		slog.Int64("statusObservedGen", statusObservedGen))
	return ctrl.Result{}, nil
}

// processDeployment handles the actual deployment to gateway
func (r *RestApiReconciler) processDeployment(
	ctx context.Context,
	apiConfig *apiv1.RestApi,
	trackingKey string,
	generation int64,
) (ctrl.Result, error) {
	// Find target gateway
	gateway := r.findTargetGateway(apiConfig)
	if gateway == nil {
		return r.handleNoGateway(ctx, apiConfig)
	}

	gatewayKey := fmt.Sprintf("%s/%s", gateway.Namespace, gateway.Name)

	// Get existing entry to preserve retry count if retrying same generation
	existingEntry, hasExisting := r.apiTracker.Get(trackingKey)
	retryCount := 0
	if hasExisting && existingEntry.Generation == generation {
		retryCount = existingEntry.RetryCount

		// Respect backoff if set
		if !existingEntry.NextRetryTime.IsZero() {
			wait := time.Until(existingEntry.NextRetryTime)
			if wait > 0 {
				r.Logger.Info("Waiting for backoff",
					slog.String("api", apiConfig.Name),
					slog.Duration("wait", wait))
				return ctrl.Result{RequeueAfter: wait}, nil
			}
		}
	}

	// Update tracker to Processing
	entry := &APITrackingEntry{
		Generation: generation,
		Status:     TrackingStatusProcessing,
		GatewayKey: gatewayKey,
		RetryCount: retryCount,
	}
	r.apiTracker.Set(trackingKey, entry)

	// Set initial conditions (only on first attempt, not retries, and unless already programmed for this gen)
	if retryCount == 0 && !programmedTrueForGeneration(apiConfig, generation) {
		if err := r.setInitialConditions(ctx, apiConfig, generation); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Execute deployment (executeDeployment will check existence and decide POST vs PUT)
	fp, err := r.executeDeployment(ctx, apiConfig, gateway)
	if err != nil {
		return r.handleDeploymentError(ctx, apiConfig, trackingKey, entry, err)
	}
	entry.Fingerprint = fp

	// Success
	return r.handleDeploymentSuccess(ctx, apiConfig, trackingKey, entry, gatewayKey)
}

// setInitialConditions sets the Accepted and initial Programmed conditions
func (r *RestApiReconciler) setInitialConditions(ctx context.Context, apiConfig *apiv1.RestApi, generation int64) error {
	base := apiConfig.DeepCopy()

	acceptedCond := metav1.Condition{
		Type:               apiv1.APIConditionAccepted,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: generation,
		Reason:             apiv1.APIAcceptedReasonAccepted,
		Message:            "RestApi configuration accepted",
		LastTransitionTime: metav1.Now(),
	}

	programmedCond := metav1.Condition{
		Type:               apiv1.APIConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: 0, // Not set until deployment succeeds
		Reason:             apiv1.APIProgrammedReasonPending,
		Message:            "Deployment in progress",
		LastTransitionTime: metav1.Now(),
	}

	meta.SetStatusCondition(&apiConfig.Status.Conditions, acceptedCond)
	meta.SetStatusCondition(&apiConfig.Status.Conditions, programmedCond)

	now := metav1.Now()
	apiConfig.Status.LastUpdateTime = &now

	return r.Status().Patch(ctx, apiConfig, client.MergeFrom(base))
}

// executeDeployment performs the actual HTTP request to the gateway
func (r *RestApiReconciler) executeDeployment(ctx context.Context, apiConfig *apiv1.RestApi, gateway *registry.GatewayInfo) (string, error) {
	log := r.Logger.With(slog.String("controller", "RestApi"), slog.String("name", apiConfig.Name))

	spec := apiConfig.Spec.DeepCopy()
	fp, err := resolveAPIConfigPolicyParamsValueFrom(ctx, r.Client, apiConfig.Namespace, spec, log)
	if err != nil {
		return "", fmt.Errorf("resolve RestApi policy params valueFrom: %w", err)
	}

	apiYAML, err := gatewayclient.BuildRestAPIYAML(
		gatewayclient.ManagementArtifactAPIVersion,
		apiConfig.Kind,
		payloadMetadataForRestAPI(apiConfig),
		*spec,
	)
	if err != nil {
		return "", fmt.Errorf("build REST API YAML: %w", err)
	}

	handle := apiConfig.Name
	auth := func(c context.Context, req *http.Request) error {
		if err := r.addAuthToRequest(c, req, gateway); err != nil {
			return err
		}
		return nil
	}

	ep := gateway.GetGatewayServiceEndpoint()
	exists, err := gatewayclient.RestAPIExists(ctx, ep, handle, auth)
	if err != nil {
		return "", err
	}

	log.Info("Deploying API to gateway", slog.String("api", apiConfig.Name), slog.Bool("exists", exists))
	if err := gatewayclient.DeployRestAPI(ctx, ep, handle, apiYAML, exists, auth); err != nil {
		return "", err
	}
	return fp, nil
}

// handleDeploymentSuccess handles successful deployment
func (r *RestApiReconciler) handleDeploymentSuccess(
	ctx context.Context,
	apiConfig *apiv1.RestApi,
	trackingKey string,
	entry *APITrackingEntry,
	gatewayKey string,
) (ctrl.Result, error) {
	log := r.Logger.With(slog.String("controller", "RestApi"), slog.String("name", apiConfig.Name))
	log.Info("Deployment succeeded", slog.String("api", apiConfig.Name), slog.String("gateway", gatewayKey))

	// Update tracker to Deployed
	entry.Status = TrackingStatusDeployed
	r.apiTracker.Set(trackingKey, entry)

	// Update status with success
	if err := r.updateProgrammedCondition(ctx, apiConfig, metav1.Condition{
		Type:               apiv1.APIConditionProgrammed,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: entry.Generation,
		Reason:             apiv1.APIProgrammedReasonProgrammed,
		Message:            fmt.Sprintf("Successfully deployed to gateway %s", gatewayKey),
		LastTransitionTime: metav1.Now(),
	}); err != nil {
		return ctrl.Result{}, err
	}

	if err := patchRestApiPolicyValueFromFingerprintAnnotation(
		ctx, r.Client,
		types.NamespacedName{Namespace: apiConfig.Namespace, Name: apiConfig.Name},
		entry.Fingerprint,
	); err != nil {
		log.Error("patch policy valueFrom fingerprint annotation failed", slog.Any("error", err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleDeploymentError handles deployment errors
func (r *RestApiReconciler) handleDeploymentError(
	ctx context.Context,
	apiConfig *apiv1.RestApi,
	trackingKey string,
	entry *APITrackingEntry,
	err error,
) (ctrl.Result, error) {
	switch e := err.(type) {
	case *gatewayclient.RetryableError:
		return r.handleRetryableError(ctx, apiConfig, trackingKey, entry, e)
	case *gatewayclient.NonRetryableError:
		return r.handleNonRetryableError(ctx, apiConfig, trackingKey, entry, e)
	default:
		if IsInvalidHTTPRouteConfigError(err) {
			return r.handleNonRetryableError(ctx, apiConfig, trackingKey, entry, &gatewayclient.NonRetryableError{
				Err:        err,
				StatusCode: http.StatusBadRequest,
			})
		}
		return r.handleRetryableError(ctx, apiConfig, trackingKey, entry, &gatewayclient.RetryableError{Err: err})
	}
}

// handleRetryableError handles errors that should be retried
func (r *RestApiReconciler) handleRetryableError(
	ctx context.Context,
	apiConfig *apiv1.RestApi,
	trackingKey string,
	entry *APITrackingEntry,
	err *gatewayclient.RetryableError,
) (ctrl.Result, error) {
	log := r.Logger.With(slog.String("controller", "RestApi"), slog.String("name", apiConfig.Name))

	entry.RetryCount++
	entry.LastRetryTime = time.Now()

	// Check if max retries exceeded
	maxRetries := r.Config.Reconciliation.MaxRetryAttempts
	if maxRetries <= 0 {
		maxRetries = 10 // sensible default fallback
	}

	if entry.RetryCount >= maxRetries {
		log.Error("Max retries exceeded",
			slog.Any("error", err.Err),
			slog.String("api", apiConfig.Name),
			slog.Int("retryCount", entry.RetryCount),
			slog.Int("maxRetries", maxRetries))

		// Mark as deployed (failed) - keeps tracking but won't retry
		entry.Status = TrackingStatusDeployed
		r.apiTracker.Set(trackingKey, entry)

		// Update status with final failure
		if updateErr := r.updateProgrammedCondition(ctx, apiConfig, metav1.Condition{
			Type:               apiv1.APIConditionProgrammed,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: entry.Generation,
			Reason:             apiv1.APIProgrammedReasonDeploymentFailed,
			Message:            fmt.Sprintf("Max retries (%d) exceeded. Last error: %s", maxRetries, err.Error()),
			LastTransitionTime: metav1.Now(),
		}); updateErr != nil {
			return ctrl.Result{}, updateErr
		}

		return ctrl.Result{}, nil
	}

	// Calculate backoff
	backoff := r.calculateBackoff(entry.RetryCount)
	entry.NextRetryTime = time.Now().Add(backoff)
	entry.Status = TrackingStatusRetrying
	r.apiTracker.Set(trackingKey, entry)

	log.Info("Deployment failed, scheduling retry",
		slog.String("api", apiConfig.Name),
		slog.Int("retryCount", entry.RetryCount),
		slog.Int("maxRetries", maxRetries),
		slog.Duration("nextRetryIn", backoff),
		slog.String("error", err.Error()))

	if updateErr := r.updateProgrammedCondition(ctx, apiConfig, metav1.Condition{
		Type:               apiv1.APIConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: 0,
		Reason:             apiv1.APIProgrammedReasonRetrying,
		Message:            fmt.Sprintf("Gateway unavailable, retrying (attempt %d/%d)", entry.RetryCount, maxRetries),
		LastTransitionTime: metav1.Now(),
	}); updateErr != nil {
		return ctrl.Result{}, updateErr
	}

	return ctrl.Result{RequeueAfter: backoff}, nil
}

// handleNonRetryableError handles errors that should not be retried
func (r *RestApiReconciler) handleNonRetryableError(
	ctx context.Context,
	apiConfig *apiv1.RestApi,
	trackingKey string,
	entry *APITrackingEntry,
	err *gatewayclient.NonRetryableError,
) (ctrl.Result, error) {
	log := r.Logger.With(slog.String("controller", "RestApi"), slog.String("name", apiConfig.Name))
	log.Error("Non-retryable deployment error",
		slog.Any("error", err.Err),
		slog.String("api", apiConfig.Name),
		slog.Int("statusCode", err.StatusCode))

	// Mark as deployed (failed)
	entry.Status = TrackingStatusDeployed
	r.apiTracker.Set(trackingKey, entry)

	// Determine reason based on status code
	reason := apiv1.APIProgrammedReasonDeploymentFailed
	if err.StatusCode == http.StatusBadRequest || err.StatusCode == http.StatusUnprocessableEntity {
		reason = apiv1.APIProgrammedReasonInvalid
	}

	// Update status
	if updateErr := r.updateProgrammedCondition(ctx, apiConfig, metav1.Condition{
		Type:               apiv1.APIConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: entry.Generation,
		Reason:             reason,
		Message:            fmt.Sprintf("Deployment failed: %s", err.Error()),
		LastTransitionTime: metav1.Now(),
	}); updateErr != nil {
		return ctrl.Result{}, updateErr
	}

	return ctrl.Result{}, nil
}

// handleNoGateway handles the case when no gateway is available
func (r *RestApiReconciler) handleNoGateway(ctx context.Context, apiConfig *apiv1.RestApi) (ctrl.Result, error) {
	log := r.Logger.With(slog.String("controller", "RestApi"), slog.String("name", apiConfig.Name))
	log.Info("No matching gateway available", slog.String("api", apiConfig.Name))

	if err := r.updateProgrammedCondition(ctx, apiConfig, metav1.Condition{
		Type:               apiv1.APIConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: 0,
		Reason:             apiv1.APIProgrammedReasonGatewayNotReady,
		Message:            "No matching gateway available",
		LastTransitionTime: metav1.Now(),
	}); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
}

// calculateBackoff calculates exponential backoff duration using operator configuration
func (r *RestApiReconciler) calculateBackoff(retryCount int) time.Duration {
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

// findTargetGateway finds the gateway to deploy the API to
func (r *RestApiReconciler) findTargetGateway(apiConfig *apiv1.RestApi) *registry.GatewayInfo {
	registryInstance := registry.GetGatewayRegistry()
	matched := registryInstance.FindMatchingGateways(apiConfig.Namespace, apiConfig.Labels)
	if len(matched) == 0 {
		return nil
	}
	return matched[0]
}

// updateProgrammedCondition updates only the Programmed condition
func (r *RestApiReconciler) updateProgrammedCondition(ctx context.Context, apiConfig *apiv1.RestApi, cond metav1.Condition) error {
	// Re-fetch to get latest version
	latest := &apiv1.RestApi{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: apiConfig.Namespace, Name: apiConfig.Name}, latest); err != nil {
		return err
	}

	base := latest.DeepCopy()

	// Check if update is needed
	existing := meta.FindStatusCondition(latest.Status.Conditions, cond.Type)
	if existing != nil {
		if existing.Status == cond.Status &&
			existing.Reason == cond.Reason &&
			existing.ObservedGeneration == cond.ObservedGeneration &&
			existing.Message == cond.Message {
			return nil
		}
		// Only update LastTransitionTime if status changed
		if existing.Status == cond.Status {
			cond.LastTransitionTime = existing.LastTransitionTime
		}
	}

	meta.SetStatusCondition(&latest.Status.Conditions, cond)

	now := metav1.Now()
	latest.Status.LastUpdateTime = &now

	return r.Status().Patch(ctx, latest, client.MergeFrom(base))
}

// reconcileAPIDeletion handles CR deletion
func (r *RestApiReconciler) reconcileAPIDeletion(ctx context.Context, apiConfig *apiv1.RestApi) (ctrl.Result, error) {
	log := r.Logger.With(slog.String("controller", "RestApi"), slog.String("name", apiConfig.Name))

	if !controllerutil.ContainsFinalizer(apiConfig, apiFinalizerName) {
		return ctrl.Result{}, nil
	}

	// Clean up from gateway
	if err := r.cleanupAPIDeployments(ctx, apiConfig); err != nil {
		log.Error("failed to clean up API deployments", slog.Any("error", err))
		return ctrl.Result{}, err
	}

	// Remove from tracker
	trackingKey := types.NamespacedName{Namespace: apiConfig.Namespace, Name: apiConfig.Name}.String()
	r.apiTracker.Delete(trackingKey)
	r.removeRestAPIFromValueFromIndex(types.NamespacedName{Namespace: apiConfig.Namespace, Name: apiConfig.Name})

	// Remove finalizer
	controllerutil.RemoveFinalizer(apiConfig, apiFinalizerName)
	if err := r.Update(ctx, apiConfig); err != nil {
		log.Error("failed to remove finalizer", slog.Any("error", err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// cleanupAPIDeployments removes the API from all gateways
func (r *RestApiReconciler) cleanupAPIDeployments(ctx context.Context, apiConfig *apiv1.RestApi) error {
	log := r.Logger.With(slog.String("controller", "RestApi"), slog.String("name", apiConfig.Name))

	handle := apiConfig.Name
	if handle == "" {
		return nil
	}

	registryInstance := registry.GetGatewayRegistry()
	matched := registryInstance.FindMatchingGateways(apiConfig.Namespace, apiConfig.Labels)

	for _, gateway := range matched {
		if err := r.deleteAPIFromGateway(ctx, handle, gateway); err != nil {
			log.Error("failed to delete API from gateway",
				slog.Any("error", err),
				slog.String("api", handle),
				slog.String("gateway", gateway.Name))
			// Continue with other gateways even if one fails
		}
	}

	return nil
}

// deleteAPIFromGateway removes an API from a specific gateway
func (r *RestApiReconciler) deleteAPIFromGateway(ctx context.Context, handle string, gateway *registry.GatewayInfo) error {
	log := r.Logger.With(slog.String("controller", "RestApi"))

	auth := func(c context.Context, req *http.Request) error {
		if err := r.addAuthToRequest(c, req, gateway); err != nil {
			return err
		}
		return nil
	}

	if err := gatewayclient.DeleteRestAPI(ctx, gateway.GetGatewayServiceEndpoint(), handle, auth); err != nil {
		log.Error("failed to delete API from gateway", slog.Any("error", err), slog.String("gateway", gateway.Name))
		return err
	}
	log.Info("API deleted from gateway",
		slog.String("api", handle),
		slog.String("gateway", gateway.Name))
	return nil
}

func payloadMetadataForRestAPI(apiConfig *apiv1.RestApi) gatewayclient.RestAPIPayloadMetadata {
	md := gatewayclient.RestAPIPayloadMetadata{Name: apiConfig.Name}
	if len(apiConfig.Labels) > 0 {
		md.Labels = make(map[string]string, len(apiConfig.Labels))
		for k, v := range apiConfig.Labels {
			md.Labels[k] = v
		}
	}
	if len(apiConfig.Annotations) > 0 {
		md.Annotations = make(map[string]string, len(apiConfig.Annotations))
		for k, v := range apiConfig.Annotations {
			md.Annotations[k] = v
		}
	}
	return md
}

// addAuthToRequest adds authentication headers to an HTTP request based on gateway auth config
func (r *RestApiReconciler) addAuthToRequest(ctx context.Context, req *http.Request, gatewayInfo *registry.GatewayInfo) error {
	log := r.Logger.With(slog.String("controller", "RestApi"))

	authConfig, err := auth.GetAuthSettingsForRegistryGateway(ctx, r.Client, gatewayInfo)
	if err != nil {
		return fmt.Errorf("retrieve auth config for gateway %q: %w", gatewayInfo.Name, err)
	}

	var username, password string
	if authConfig != nil {
		// Try to get credentials from the auth config
		var ok bool
		username, password, ok = auth.GetBasicAuthCredentials(authConfig)
		if !ok {
			// Auth config exists but no valid basic auth, use default
			log.Debug("No valid basic auth in config, using default credentials",
				slog.String("gateway", gatewayInfo.Name))
			username, password = auth.GetDefaultBasicAuthCredentials()
		}
	} else {
		// No auth config, use default
		log.Debug("No auth config found, using default credentials",
			slog.String("gateway", gatewayInfo.Name))
		username, password = auth.GetDefaultBasicAuthCredentials()
	}

	// Encode and set the Authorization header
	encodedAuth := auth.EncodeBasicAuth(username, password)
	req.Header.Set("Authorization", "Basic "+encodedAuth)

	return nil
}

// enqueueAPIsForGateway watches for Gateway changes and enqueues affected RestApis
func (r *RestApiReconciler) enqueueAPIsForGateway(ctx context.Context, obj client.Object) []reconcile.Request {
	gateway, ok := obj.(*apiv1.APIGateway)
	if !ok {
		return nil
	}

	logger := log.FromContext(ctx)

	apiList := &apiv1.RestApiList{}
	if err := r.List(ctx, apiList); err != nil {
		logger.Error(err, "failed to list RestApis for gateway event",
			"gateway", gateway.Name,
			"namespace", gateway.Namespace)
		return nil
	}

	sel := selector.NewAPISelector(r.Client)
	requests := make([]reconcile.Request, 0, len(apiList.Items))

	for i := range apiList.Items {
		api := &apiList.Items[i]
		wants, err := sel.IsAPISelectedByGateway(ctx, api, gateway)
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

// SetupWithManager sets up the controller with the Manager.
func (r *RestApiReconciler) SetupWithManager(mgr ctrl.Manager) error {
	gatewayPred := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(evt event.UpdateEvent) bool {
			newGateway, okNew := evt.ObjectNew.(*apiv1.APIGateway)
			oldGateway, okOld := evt.ObjectOld.(*apiv1.APIGateway)
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

	opts := controller.Options{MaxConcurrentReconciles: r.Config.Reconciliation.MaxConcurrentReconciles}
	if opts.MaxConcurrentReconciles <= 0 {
		opts.MaxConcurrentReconciles = 1
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(opts).
		For(&apiv1.RestApi{}).
		Watches(&apiv1.APIGateway{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueAPIsForGateway),
			builder.WithPredicates(gatewayPred)).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueRestApisForSecret),
			builder.WithPredicates(secretMutationPredicate())).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueRestApisForConfigMap),
			builder.WithPredicates(configMapMutationPredicate())).
		Complete(r)
}

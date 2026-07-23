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
	"math"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/auth"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

// ResourceTrackingStatus tracks the deployment state in memory. Mirrors
// APITrackingStatus on RestApiReconciler but is shared across the new kinds
// reconciled via GenericReconciler.
type ResourceTrackingStatus string

const (
	ResourceStatusProcessing ResourceTrackingStatus = "Processing"
	ResourceStatusRetrying   ResourceTrackingStatus = "Retrying"
	ResourceStatusDeployed   ResourceTrackingStatus = "Deployed"
)

// ResourceTrackingEntry mirrors APITrackingEntry but is decoupled from the
// RestApi-specific reconciler so the same tracker can be reused by the
// generic reconciler.
type ResourceTrackingEntry struct {
	Generation    int64
	Status        ResourceTrackingStatus
	GatewayKey    string
	Id            string
	Fingerprint   string
	RetryCount    int
	LastRetryTime time.Time
	NextRetryTime time.Time
	// GatewayDeploySucceeded is true after Adapter.Deploy returns nil while the
	// in-memory programmed condition / status write may still be pending. Used
	// so a failed Status().Update does not strand the tracker on Deployed with
	// stale observedGeneration (see decideAndProcess + handleDeploymentSuccess).
	GatewayDeploySucceeded bool
}

// ResourceTracker is the shared in-memory tracker used by the generic
// reconciler. Keys are namespace/name/kind triples so multiple kinds can
// share a single tracker without collision.
type ResourceTracker struct {
	mu      sync.RWMutex
	entries map[string]*ResourceTrackingEntry
}

// NewResourceTracker constructs an empty ResourceTracker.
func NewResourceTracker() *ResourceTracker {
	return &ResourceTracker{entries: make(map[string]*ResourceTrackingEntry)}
}

// Get returns a copy of the entry under key (if any).
func (t *ResourceTracker) Get(key string) (*ResourceTrackingEntry, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	e, ok := t.entries[key]
	if !ok {
		return nil, false
	}
	c := *e
	return &c, true
}

// Set stores entry under key.
func (t *ResourceTracker) Set(key string, entry *ResourceTrackingEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries[key] = entry
}

// Delete removes the entry under key.
func (t *ResourceTracker) Delete(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}

// DeployResult captures the outcome of an adapter Deploy call. Id is set
// when the gateway-controller assigns a UUID (Subscription, SubscriptionPlan,
// Certificate) so the controller can persist it to Status.Id for use on
// subsequent update/delete.
type DeployResult struct {
	// Id is the gateway-issued identifier, when applicable.
	Id string
	// Fingerprint is the external-dependency fingerprint captured at deploy time.
	// When non-empty it is stored on the tracking entry and forwarded to
	// onExternalDepsApplied so the annotation reflects the exact state deployed
	// rather than a re-computed (potentially racy) value.
	Fingerprint string
}

// ResourceAdapter abstracts the per-kind specifics for the generic
// reconciler. Implementations are supplied by each per-kind controller and
// must be safe for concurrent use.
type ResourceAdapter interface {
	// Kind returns the controller's CR kind label, used in logs/keys.
	Kind() string

	// FinalizerName returns the finalizer attached to the CR.
	FinalizerName() string

	// NewObject returns a fresh empty CR pointer (e.g. &v1alpha1.LlmProvider{}).
	NewObject() client.Object

	// Handle returns the management-API handle for the CR. For envelope
	// kinds this is typically obj.GetName(); for UUID-keyed kinds it is
	// the persisted Status.Id (or empty when not yet deployed).
	Handle(obj client.Object) string

	// IsUUIDKeyed indicates that the resource is addressed by a
	// gateway-issued UUID after the first successful deploy. UUID-keyed
	// resources are never probed via Exists; the controller decides
	// POST vs PUT based on the persisted Status.Id.
	IsUUIDKeyed() bool

	// Status accessors for shared lifecycle fields.
	GetStatus(obj client.Object) *apiv1.ResourceStatus
	SetStatusId(obj client.Object, id string)

	// Selection: returns (namespace, labels) used to find a matching
	// gateway via registry.GatewayRegistry.FindMatchingGateways.
	GatewaySelectionKey(obj client.Object) (namespace string, labels map[string]string)

	// Deploy synchronises the CR to the gateway.
	Deploy(ctx context.Context, k8sClient client.Client, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) (DeployResult, error)

	// Delete removes the CR from the gateway. Implementations must treat
	// "not found" / missing-id as success so finalizer removal is
	// idempotent.
	Delete(ctx context.Context, gatewayEndpoint string, obj client.Object, authFn gatewayclient.AuthHeaderFunc) error
}

// GenericReconciler hosts the shared decision/tracker/retry/cleanup logic
// extracted from RestApiReconciler. Per-kind controllers compose this by
// injecting a ResourceAdapter.
type GenericReconciler struct {
	client.Client
	Adapter ResourceAdapter
	Config  *config.OperatorConfig
	Logger  *zap.Logger
	Tracker *ResourceTracker
}

// trackingKey is the per-CR key used in ResourceTracker.
func (r *GenericReconciler) trackingKey(req ctrl.Request) string {
	return fmt.Sprintf("%s|%s", r.Adapter.Kind(), req.String())
}

// Reconcile implements the controller-runtime Reconciler interface. It is
// expected to be wired up by the per-kind controller's SetupWithManager.
func (r *GenericReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Logger.With(
		zap.String("controller", r.Adapter.Kind()),
		zap.String("namespace", req.Namespace),
		zap.String("name", req.Name),
	)

	obj := r.Adapter.NewObject()
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			r.Tracker.Delete(r.trackingKey(req))
			return ctrl.Result{}, nil
		}
		log.Error("unable to fetch resource", zap.Error(err))
		return ctrl.Result{}, err
	}

	log.Info("Reconciling",
		zap.String("name", obj.GetName()),
		zap.String("namespace", obj.GetNamespace()),
		zap.Int64("generation", obj.GetGeneration()))

	if !obj.GetDeletionTimestamp().IsZero() {
		return r.reconcileDeletion(ctx, obj)
	}

	if !controllerutil.ContainsFinalizer(obj, r.Adapter.FinalizerName()) {
		controllerutil.AddFinalizer(obj, r.Adapter.FinalizerName())
		if err := r.Update(ctx, obj); err != nil {
			log.Error("failed to add finalizer", zap.Error(err))
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	statusPtr := r.Adapter.GetStatus(obj)
	progCond := meta.FindStatusCondition(statusPtr.Conditions, apiv1.ConditionProgrammed)
	statusObservedGen := int64(0)
	if progCond != nil {
		statusObservedGen = progCond.ObservedGeneration
	}

	trackingKey := r.trackingKey(req)
	entry, hasEntry := r.Tracker.Get(trackingKey)

	return r.decideAndProcess(ctx, obj, trackingKey, obj.GetGeneration(), statusObservedGen, progCond, entry, hasEntry)
}

// externalDepsDrifter is implemented by adapters whose effective spec can change
// without a metadata.generation bump (for example an ApiKey value resolved from
// a watched Secret). The generic reconciler consults this before the
// "already deployed, skipping" short-circuit.
type externalDepsDrifter interface {
	needsRedeployForExternalDeps(ctx context.Context, c client.Client, obj client.Object) (bool, error)
	// onExternalDepsApplied is called after a successful deployment. fingerprint
	// is the value returned by Deploy so the annotation is set to the exact
	// state that was deployed, avoiding a racy re-computation.
	onExternalDepsApplied(ctx context.Context, c client.Client, obj client.Object, fingerprint string) error
}

func deploymentAlreadySyncedToGeneration(status *apiv1.ResourceStatus, generation int64) bool {
	if status == nil {
		return false
	}
	acc := meta.FindStatusCondition(status.Conditions, apiv1.ConditionAccepted)
	prog := meta.FindStatusCondition(status.Conditions, apiv1.ConditionProgrammed)
	if acc == nil || prog == nil {
		return false
	}
	if acc.Status != metav1.ConditionTrue || prog.Status != metav1.ConditionTrue {
		return false
	}
	return acc.ObservedGeneration == generation && prog.ObservedGeneration == generation
}

func (r *GenericReconciler) decideAndProcess(
	ctx context.Context,
	obj client.Object,
	trackingKey string,
	crGeneration, statusObservedGen int64,
	progCond *metav1.Condition,
	entry *ResourceTrackingEntry,
	hasEntry bool,
) (ctrl.Result, error) {
	log := r.Logger.With(zap.String("controller", r.Adapter.Kind()), zap.String("name", obj.GetName()))

	if crGeneration == statusObservedGen && progCond != nil && progCond.Status == metav1.ConditionTrue {
		if drift, ok := r.Adapter.(externalDepsDrifter); ok {
			need, err := drift.needsRedeployForExternalDeps(ctx, r.Client, obj)
			if err != nil {
				return ctrl.Result{}, err
			}
			if need {
				log.Info("external dependency drift detected; redeploying gateway state",
					zap.String("name", obj.GetName()),
					zap.Int64("generation", crGeneration))
				return r.processDeployment(ctx, obj, trackingKey, crGeneration)
			}
		}
		if r.Adapter.IsUUIDKeyed() {
			if st := r.Adapter.GetStatus(obj); st != nil && st.Id == "" {
				log.Info("Programmed=True but status.id is empty; reconciling uuid-keyed gateway id",
					zap.String("name", obj.GetName()),
					zap.Int64("generation", crGeneration))
				return r.processDeployment(ctx, obj, trackingKey, crGeneration)
			}
		}
		next := &ResourceTrackingEntry{Generation: crGeneration, Status: ResourceStatusDeployed}
		if hasEntry && entry != nil {
			if entry.GatewayKey != "" {
				next.GatewayKey = entry.GatewayKey
			}
			if entry.Id != "" {
				next.Id = entry.Id
			}
		}
		if next.Id == "" {
			if st := r.Adapter.GetStatus(obj); st != nil && st.Id != "" {
				next.Id = st.Id
			}
		}
		r.Tracker.Set(trackingKey, next)
		log.Debug("already deployed, skipping",
			zap.String("name", obj.GetName()),
			zap.Int64("generation", crGeneration))
		return ctrl.Result{}, nil
	}

	if crGeneration > statusObservedGen {
		if hasEntry {
			if entry.Generation == crGeneration {
				switch entry.Status {
				case ResourceStatusProcessing:
					if entry.GatewayDeploySucceeded && entry.Generation == crGeneration {
						log.Info("retrying programmed status after gateway deploy",
							zap.String("name", obj.GetName()),
							zap.Int64("generation", crGeneration))
						return r.handleDeploymentSuccess(ctx, obj, trackingKey, entry, entry.GatewayKey)
					}
					log.Debug("already processing, skipping false positive",
						zap.String("name", obj.GetName()),
						zap.Int64("generation", crGeneration))
					return ctrl.Result{}, nil
				case ResourceStatusRetrying:
					log.Info("retrying deployment",
						zap.String("name", obj.GetName()),
						zap.Int64("generation", crGeneration),
						zap.Int("retryCount", entry.RetryCount))
					return r.processDeployment(ctx, obj, trackingKey, crGeneration)
				case ResourceStatusDeployed:
					if crGeneration > statusObservedGen {
						log.Info("retrying programmed status (tracker deployed, status not yet propagated)",
							zap.String("name", obj.GetName()),
							zap.Int64("generation", crGeneration))
						return r.handleDeploymentSuccess(ctx, obj, trackingKey, entry, entry.GatewayKey)
					}
					log.Debug("deployment and status in sync with tracker",
						zap.String("name", obj.GetName()),
						zap.Int64("generation", crGeneration))
					return ctrl.Result{}, nil
				}
			}
			if entry.Generation < crGeneration {
				log.Info("processing update",
					zap.String("name", obj.GetName()),
					zap.Int64("oldGeneration", entry.Generation),
					zap.Int64("newGeneration", crGeneration))
				return r.processDeployment(ctx, obj, trackingKey, crGeneration)
			}
		} else {
			log.Info("processing", zap.String("name", obj.GetName()), zap.Int64("generation", crGeneration))
			return r.processDeployment(ctx, obj, trackingKey, crGeneration)
		}
	}

	log.Debug("no action needed",
		zap.String("name", obj.GetName()),
		zap.Int64("crGeneration", crGeneration),
		zap.Int64("statusObservedGen", statusObservedGen))
	return ctrl.Result{}, nil
}

func (r *GenericReconciler) processDeployment(ctx context.Context, obj client.Object, trackingKey string, generation int64) (ctrl.Result, error) {
	gateway := r.findTargetGateway(obj)
	if gateway == nil {
		return r.handleNoGateway(ctx, obj)
	}
	gatewayKey := fmt.Sprintf("%s/%s", gateway.Namespace, gateway.Name)

	existingEntry, hasExisting := r.Tracker.Get(trackingKey)
	retryCount := 0
	if hasExisting && existingEntry.Generation == generation {
		retryCount = existingEntry.RetryCount
		if !existingEntry.NextRetryTime.IsZero() {
			wait := time.Until(existingEntry.NextRetryTime)
			if wait > 0 {
				r.Logger.Info("waiting for backoff",
					zap.String("kind", r.Adapter.Kind()),
					zap.String("name", obj.GetName()),
					zap.Duration("wait", wait))
				return ctrl.Result{RequeueAfter: wait}, nil
			}
		}
	}

	entry := &ResourceTrackingEntry{
		Generation: generation,
		Status:     ResourceStatusProcessing,
		GatewayKey: gatewayKey,
		Id:         r.Adapter.GetStatus(obj).Id,
		RetryCount: retryCount,
	}
	r.Tracker.Set(trackingKey, entry)

	statusPtr := r.Adapter.GetStatus(obj)
	if retryCount == 0 && !deploymentAlreadySyncedToGeneration(statusPtr, generation) {
		if err := r.setInitialConditions(ctx, obj, generation); err != nil {
			return ctrl.Result{}, err
		}
	}

	authFn := r.buildAuthFn(gateway)
	endpoint := gateway.GetGatewayServiceEndpoint()

	result, err := r.Adapter.Deploy(ctx, r.Client, endpoint, obj, authFn)
	if err != nil {
		return r.handleDeploymentError(ctx, obj, trackingKey, entry, err)
	}

	if result.Id != "" {
		r.Adapter.SetStatusId(obj, result.Id)
		entry.Id = result.Id
		r.Tracker.Set(trackingKey, entry)
	}

	if result.Fingerprint != "" {
		entry.Fingerprint = result.Fingerprint
	}

	entry.GatewayDeploySucceeded = true
	r.Tracker.Set(trackingKey, entry)

	return r.handleDeploymentSuccess(ctx, obj, trackingKey, entry, gatewayKey)
}

func (r *GenericReconciler) findTargetGateway(obj client.Object) *registry.GatewayInfo {
	registryInstance := registry.GetGatewayRegistry()
	ns, labels := r.Adapter.GatewaySelectionKey(obj)
	matched := registryInstance.FindMatchingGateways(ns, labels)
	if len(matched) == 0 {
		return nil
	}
	return matched[0]
}

func (r *GenericReconciler) buildAuthFn(gateway *registry.GatewayInfo) gatewayclient.AuthHeaderFunc {
	return func(ctx context.Context, req *http.Request) error {
		authConfig, err := auth.GetAuthSettingsForRegistryGateway(ctx, r.Client, gateway)
		if err != nil {
			return fmt.Errorf("retrieve auth config for gateway %q: %w", gateway.Name, err)
		}
		var username, password string
		if authConfig != nil {
			var ok bool
			username, password, ok = auth.GetBasicAuthCredentials(authConfig)
			if !ok {
				username, password = auth.GetDefaultBasicAuthCredentials()
			}
		} else {
			username, password = auth.GetDefaultBasicAuthCredentials()
		}
		req.Header.Set("Authorization", "Basic "+auth.EncodeBasicAuth(username, password))
		return nil
	}
}

func (r *GenericReconciler) setInitialConditions(ctx context.Context, obj client.Object, generation int64) error {
	statusPtr := r.Adapter.GetStatus(obj)
	now := metav1.Now()

	meta.SetStatusCondition(&statusPtr.Conditions, metav1.Condition{
		Type:               apiv1.ConditionAccepted,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: generation,
		Reason:             apiv1.ReasonAccepted,
		Message:            "Configuration accepted",
		LastTransitionTime: now,
	})
	meta.SetStatusCondition(&statusPtr.Conditions, metav1.Condition{
		Type:               apiv1.ConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: 0,
		Reason:             apiv1.ReasonProgrammedPending,
		Message:            "Deployment in progress",
		LastTransitionTime: now,
	})
	statusPtr.LastUpdateTime = &now

	return r.Status().Update(ctx, obj)
}

func (r *GenericReconciler) handleDeploymentSuccess(ctx context.Context, obj client.Object, trackingKey string, entry *ResourceTrackingEntry, gatewayKey string) (ctrl.Result, error) {
	log := r.Logger.With(zap.String("controller", r.Adapter.Kind()), zap.String("name", obj.GetName()))
	log.Info("deployment succeeded", zap.String("gateway", gatewayKey))

	if err := r.updateProgrammed(ctx, obj, metav1.Condition{
		Type:               apiv1.ConditionProgrammed,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: entry.Generation,
		Reason:             apiv1.ReasonProgrammed,
		Message:            fmt.Sprintf("Successfully deployed to gateway %s", gatewayKey),
		LastTransitionTime: metav1.Now(),
	}); err != nil {
		return ctrl.Result{}, err
	}

	if deps, ok := r.Adapter.(externalDepsDrifter); ok {
		if err := deps.onExternalDepsApplied(ctx, r.Client, obj, entry.Fingerprint); err != nil {
			return ctrl.Result{}, err
		}
	}

	entry.Status = ResourceStatusDeployed
	entry.GatewayDeploySucceeded = false
	r.Tracker.Set(trackingKey, entry)
	return ctrl.Result{}, nil
}

func (r *GenericReconciler) handleDeploymentError(ctx context.Context, obj client.Object, trackingKey string, entry *ResourceTrackingEntry, err error) (ctrl.Result, error) {
	switch e := err.(type) {
	case *gatewayclient.RetryableError:
		return r.handleRetryableError(ctx, obj, trackingKey, entry, e)
	case *gatewayclient.NonRetryableError:
		return r.handleNonRetryableError(ctx, obj, trackingKey, entry, e)
	default:
		return r.handleRetryableError(ctx, obj, trackingKey, entry, &gatewayclient.RetryableError{Err: err})
	}
}

func (r *GenericReconciler) handleRetryableError(ctx context.Context, obj client.Object, trackingKey string, entry *ResourceTrackingEntry, err *gatewayclient.RetryableError) (ctrl.Result, error) {
	log := r.Logger.With(zap.String("controller", r.Adapter.Kind()), zap.String("name", obj.GetName()))
	entry.GatewayDeploySucceeded = false
	entry.RetryCount++
	entry.LastRetryTime = time.Now()

	maxRetries := r.Config.Reconciliation.MaxRetryAttempts
	if maxRetries <= 0 {
		maxRetries = 10
	}

	if entry.RetryCount >= maxRetries {
		log.Error("max retries exceeded",
			zap.Error(err.Err),
			zap.Int("retryCount", entry.RetryCount),
			zap.Int("maxRetries", maxRetries))
		if updateErr := r.updateProgrammed(ctx, obj, metav1.Condition{
			Type:               apiv1.ConditionProgrammed,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: entry.Generation,
			Reason:             apiv1.ReasonDeploymentFailed,
			Message:            fmt.Sprintf("Max retries (%d) exceeded. Last error: %s", maxRetries, err.Error()),
			LastTransitionTime: metav1.Now(),
		}); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		entry.Status = ResourceStatusDeployed
		entry.GatewayDeploySucceeded = false
		r.Tracker.Set(trackingKey, entry)
		return ctrl.Result{}, nil
	}

	backoff := r.calculateBackoff(entry.RetryCount)
	entry.NextRetryTime = time.Now().Add(backoff)

	log.Info("deployment failed, scheduling retry",
		zap.Int("retryCount", entry.RetryCount),
		zap.Int("maxRetries", maxRetries),
		zap.Duration("nextRetryIn", backoff),
		zap.String("error", err.Error()))

	if updateErr := r.updateProgrammed(ctx, obj, metav1.Condition{
		Type:               apiv1.ConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: 0,
		Reason:             apiv1.ReasonRetrying,
		Message:            fmt.Sprintf("Gateway unavailable, retrying (attempt %d/%d)", entry.RetryCount, maxRetries),
		LastTransitionTime: metav1.Now(),
	}); updateErr != nil {
		return ctrl.Result{}, updateErr
	}

	entry.Status = ResourceStatusRetrying
	entry.GatewayDeploySucceeded = false
	r.Tracker.Set(trackingKey, entry)
	return ctrl.Result{RequeueAfter: backoff}, nil
}

func (r *GenericReconciler) handleNonRetryableError(ctx context.Context, obj client.Object, trackingKey string, entry *ResourceTrackingEntry, err *gatewayclient.NonRetryableError) (ctrl.Result, error) {
	log := r.Logger.With(zap.String("controller", r.Adapter.Kind()), zap.String("name", obj.GetName()))
	log.Error("non-retryable deployment error",
		zap.Error(err.Err),
		zap.Int("statusCode", err.StatusCode))

	entry.GatewayDeploySucceeded = false

	reason := apiv1.ReasonDeploymentFailed
	if err.StatusCode == http.StatusBadRequest || err.StatusCode == http.StatusUnprocessableEntity {
		reason = apiv1.ReasonInvalid
	}

	if updateErr := r.updateProgrammed(ctx, obj, metav1.Condition{
		Type:               apiv1.ConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: entry.Generation,
		Reason:             reason,
		Message:            fmt.Sprintf("Deployment failed: %s", err.Error()),
		LastTransitionTime: metav1.Now(),
	}); updateErr != nil {
		return ctrl.Result{}, updateErr
	}

	entry.Status = ResourceStatusDeployed
	entry.GatewayDeploySucceeded = false
	r.Tracker.Set(trackingKey, entry)
	return ctrl.Result{}, nil
}

func (r *GenericReconciler) handleNoGateway(ctx context.Context, obj client.Object) (ctrl.Result, error) {
	log := r.Logger.With(zap.String("controller", r.Adapter.Kind()), zap.String("name", obj.GetName()))
	log.Info("no matching gateway available")
	if err := r.updateProgrammed(ctx, obj, metav1.Condition{
		Type:               apiv1.ConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: 0,
		Reason:             apiv1.ReasonGatewayNotReady,
		Message:            "No matching gateway available",
		LastTransitionTime: metav1.Now(),
	}); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
}

func (r *GenericReconciler) calculateBackoff(retryCount int) time.Duration {
	cfg := r.Config.Reconciliation
	initial := cfg.InitialBackoff
	maxBackoff := cfg.MaxBackoffDuration
	if initial <= 0 {
		initial = 1 * time.Second
	}
	if maxBackoff <= 0 {
		maxBackoff = 60 * time.Second
	}
	backoff := initial * time.Duration(math.Pow(2, float64(retryCount-1)))
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return backoff
}

// updateProgrammed updates the Programmed condition on a fresh copy of the
// CR (mirrors RestApiReconciler.updateProgrammedCondition). It also
// persists Status.Id changes set by the adapter on the in-memory object.
func (r *GenericReconciler) updateProgrammed(ctx context.Context, obj client.Object, cond metav1.Condition) error {
	latest := r.Adapter.NewObject()
	if err := r.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}, latest); err != nil {
		return err
	}

	desiredStatus := r.Adapter.GetStatus(obj)
	latestStatus := r.Adapter.GetStatus(latest)

	// Detect whether we need to carry over the gateway-issued id (must not
	// mutate latestStatus.Id before the no-op check, or we skip persisting it).
	idChanged := desiredStatus.Id != "" && latestStatus.Id != desiredStatus.Id

	existing := meta.FindStatusCondition(latestStatus.Conditions, cond.Type)
	if existing != nil {
		if existing.Status == cond.Status &&
			existing.Reason == cond.Reason &&
			existing.ObservedGeneration == cond.ObservedGeneration &&
			existing.Message == cond.Message &&
			!idChanged {
			return nil
		}
		if existing.Status == cond.Status {
			cond.LastTransitionTime = existing.LastTransitionTime
		}
	}

	if idChanged {
		latestStatus.Id = desiredStatus.Id
	}

	meta.SetStatusCondition(&latestStatus.Conditions, cond)
	now := metav1.Now()
	latestStatus.LastUpdateTime = &now

	return r.Status().Update(ctx, latest)
}

func (r *GenericReconciler) reconcileDeletion(ctx context.Context, obj client.Object) (ctrl.Result, error) {
	log := r.Logger.With(zap.String("controller", r.Adapter.Kind()), zap.String("name", obj.GetName()))

	if !controllerutil.ContainsFinalizer(obj, r.Adapter.FinalizerName()) {
		return ctrl.Result{}, nil
	}

	if err := r.cleanupAllGateways(ctx, obj); err != nil {
		log.Error("failed to clean up deployments", zap.Error(err))
		return ctrl.Result{}, err
	}

	r.Tracker.Delete(r.trackingKey(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}}))

	controllerutil.RemoveFinalizer(obj, r.Adapter.FinalizerName())
	if err := r.Update(ctx, obj); err != nil {
		log.Error("failed to remove finalizer", zap.Error(err))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *GenericReconciler) cleanupAllGateways(ctx context.Context, obj client.Object) error {
	log := r.Logger.With(zap.String("controller", r.Adapter.Kind()), zap.String("name", obj.GetName()))
	registryInstance := registry.GetGatewayRegistry()
	ns, labels := r.Adapter.GatewaySelectionKey(obj)
	matched := registryInstance.FindMatchingGateways(ns, labels)

	var deleteErrs []error
	for _, gateway := range matched {
		authFn := r.buildAuthFn(gateway)
		if err := r.Adapter.Delete(ctx, gateway.GetGatewayServiceEndpoint(), obj, authFn); err != nil {
			log.Error("failed to delete from gateway",
				zap.Error(err),
				zap.String("gateway", gateway.Name))
			deleteErrs = append(deleteErrs, fmt.Errorf("%s/%s: %w", gateway.Namespace, gateway.Name, err))
		} else {
			log.Info("resource deleted from gateway", zap.String("gateway", gateway.Name))
		}
	}
	if len(deleteErrs) > 0 {
		return fmt.Errorf("cleanupAllGateways: %w", errors.Join(deleteErrs...))
	}
	return nil
}

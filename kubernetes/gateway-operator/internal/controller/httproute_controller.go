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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"log/slog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	contctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/auth"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

const httprouteFinalizer = "gateway.api-platform.wso2.com/httproute-finalizer"

// httpRouteGatewayConcurrency bounds how many Gateways one reconcile talks to at once, so a slow
// or unreachable gateway-controller doesn't hold up the others.
const httpRouteGatewayConcurrency = 4

const (
	requeueParentGatewayMissing = 15 * time.Second
	requeueGatewayNotRegistered = 10 * time.Second
	requeueGatewayRESTRetry     = 15 * time.Second
	requeueTransientRouteConfig = 30 * time.Second
	requeueStaleCleanup         = 10 * time.Second
)

// HTTPRouteReconciler maps HTTPRoute + backends to gateway-controller REST APIs, one deployment
// per parent Gateway the operator manages.
type HTTPRouteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.OperatorConfig
	Logger *slog.Logger

	// restAPI talks to gateway-controllers; nil means the real REST client.
	restAPI restAPIClient
}

type gatewayParentTarget struct {
	key client.ObjectKey
	ref gatewayv1.ParentReference
}

// NewHTTPRouteReconciler creates a reconciler for HTTPRoute.
func NewHTTPRouteReconciler(cl client.Client, scheme *runtime.Scheme, cfg *config.OperatorConfig, logger *slog.Logger) *HTTPRouteReconciler {
	return &HTTPRouteReconciler{
		Client: cl,
		Scheme: scheme,
		Config: cfg,
		Logger: logger,
	}
}

func (r *HTTPRouteReconciler) rest() restAPIClient {
	if r.restAPI == nil {
		return gatewayRESTClient{}
	}
	return r.restAPI
}

// managedParentGateway is a parent Gateway whose class this operator manages.
type managedParentGateway struct {
	group parentGatewayGroup
	gw    *gatewayv1.Gateway
}

type gatewayDeployState int

const (
	// gatewayNotTarget: no listener on the Gateway accepts the route.
	gatewayNotTarget gatewayDeployState = iota
	// gatewayDeployed: the current config is live on the Gateway.
	gatewayDeployed
	// gatewayPending: not deployed yet; retried later.
	gatewayPending
	// gatewayFailed: the gateway rejected the config; retried only when something changes.
	gatewayFailed
)

// parentGatewayOutcome is the result of reconciling the route against one Gateway.
type parentGatewayOutcome struct {
	state   gatewayDeployState
	entry   deployedGatewayEntry
	requeue time.Duration
	// err is a Kubernetes or encoding error that Reconcile returns as is.
	err   error
	conds []parentRefCondition
}

type parentRefCondition struct {
	ref  gatewayv1.ParentReference
	cond metav1.Condition
}

func (o *parentGatewayOutcome) setCondition(route *gatewayv1.HTTPRoute, ref gatewayv1.ParentReference, condType string, status metav1.ConditionStatus, reason, message string) {
	o.conds = append(o.conds, parentRefCondition{ref: ref, cond: metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: route.Generation,
		LastTransitionTime: metav1.Now(),
	}})
}

// cleanupCandidate is a Gateway the route's API may have to be removed from.
type cleanupCandidate struct {
	key        client.ObjectKey
	entry      deployedGatewayEntry
	recorded   bool
	specParent bool
}

// requeueAfter keeps the shortest non-zero requeue delay asked for.
type requeueAfter time.Duration

func (q *requeueAfter) at(d time.Duration) {
	if d > 0 && (*q == 0 || d < time.Duration(*q)) {
		*q = requeueAfter(d)
	}
}

//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/finalizers,verbs=update
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=referencegrants,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=apipolicies,verbs=get;list;watch
//+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=apipolicies/status,verbs=get;update;patch

func (r *HTTPRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	route := &gatewayv1.HTTPRoute{}
	if err := r.Get(ctx, req.NamespacedName, route); err != nil {
		if apierrors.IsNotFound(err) {
			if r.Logger != nil {
				r.Logger.Debug("HTTPRoute not found; likely deleted",
					slog.String("controller", "HTTPRoute"),
					slog.String("namespace", req.Namespace),
					slog.String("name", req.Name))
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log := r.Logger.With(slog.String("controller", "HTTPRoute"), slog.String("namespace", req.Namespace), slog.String("name", req.Name))
	log.Info("reconcile HTTPRoute",
		slog.Int64("generation", route.Generation),
		slog.String("resourceVersion", route.ResourceVersion))

	if !route.DeletionTimestamp.IsZero() {
		return r.reconcileDeletion(ctx, route, log)
	}

	var requeue requeueAfter
	handle := DefaultHTTPRouteAPIHandle(route)
	record := deployedGatewaysFromRoute(route)
	status := newRouteStatusPlan(route.Namespace)

	// Load each parent Gateway once. A Gateway of another class belongs to another controller:
	// the route is neither deployed there nor given status for it by us.
	var ours []managedParentGateway
	missing := map[client.ObjectKey]bool{}
	otherClass := map[client.ObjectKey]bool{}
	for _, group := range groupParentTargetsByGateway(parentGatewayRefs(route)) {
		gw := &gatewayv1.Gateway{}
		if err := r.Get(ctx, group.key, gw); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("parent Gateway not found; requeue",
					slog.String("parentNamespace", group.key.Namespace),
					slog.String("parentName", group.key.Name))
				missing[group.key] = true
				requeue.at(requeueParentGatewayMissing)
				continue
			}
			return ctrl.Result{}, err
		}
		if !r.Config.ManagedGatewayClass(string(gw.Spec.GatewayClassName)) {
			log.Debug("skip parent Gateway: unmanaged class",
				slog.String("parentNamespace", group.key.Namespace),
				slog.String("parentName", group.key.Name),
				slog.String("gatewayClass", string(gw.Spec.GatewayClassName)))
			otherClass[group.key] = true
			continue
		}
		ours = append(ours, managedParentGateway{group: group, gw: gw})
	}

	if len(ours) > 0 && !controllerutil.ContainsFinalizer(route, httprouteFinalizer) {
		controllerutil.AddFinalizer(route, httprouteFinalizer)
		if err := r.Update(ctx, route); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("added HTTPRoute finalizer; requeue")
		return ctrl.Result{Requeue: true}, nil
	}

	// Backends are Services, so they resolve the same way for every Gateway.
	var backends *HTTPRouteBackendResolution
	if len(ours) > 0 {
		resolved, err := resolveHTTPRouteBackendRefs(ctx, r.Client, route, r.Config.GatewayAPI.ClusterDomain)
		if err != nil {
			// Transient lookup failure: report a retry reason and requeue, without latching a
			// permanent ResolvedRefs result or touching existing deployments.
			log.Error("resolve backend refs", slog.Any("error", err))
			failure := &HTTPRouteBackendResolution{
				FirstFailureReason:  gatewayv1.RouteConditionReason("Retrying"),
				FirstFailureMessage: err.Error(),
			}
			for _, p := range ours {
				for _, t := range p.group.targets {
					status.set(t.ref, resolvedRefsCondition(route, failure))
				}
			}
			if err := r.applyRouteStatus(ctx, route, status, nil); err != nil {
				log.Error("patch HTTPRoute status", slog.Any("error", err))
			}
			return ctrl.Result{RequeueAfter: requeueTransientRouteConfig}, nil
		}
		backends = resolved
		for _, p := range ours {
			for _, t := range p.group.targets {
				status.set(t.ref, resolvedRefsCondition(route, backends))
			}
		}
	}

	outcomes := make([]parentGatewayOutcome, len(ours))
	forEachBounded(len(ours), httpRouteGatewayConcurrency, func(i int) {
		prev, hadPrev := record[gatewayKeyString(ours[i].group.key)]
		outcomes[i] = r.reconcileParentGateway(ctx, route, ours[i], backends, handle, prev, hadPrev, log)
	})

	next := deployedGateways{}
	wanted := map[string]bool{}
	rejected := map[string]bool{}
	settled := len(missing) == 0
	for i, p := range ours {
		o := outcomes[i]
		if o.err != nil {
			return ctrl.Result{}, o.err
		}
		for _, c := range o.conds {
			status.set(c.ref, c.cond)
		}
		key := gatewayKeyString(p.group.key)
		switch o.state {
		case gatewayNotTarget:
			rejected[key] = true
		case gatewayDeployed:
			next[key] = o.entry
			wanted[key] = true
		case gatewayPending, gatewayFailed:
			// Whatever was deployed there before stays recorded until it is replaced.
			if prev, ok := record[key]; ok {
				next[key] = prev
			}
			wanted[key] = true
			settled = false
			requeue.at(o.requeue)
		}
	}
	for key := range missing {
		k := gatewayKeyString(key)
		if prev, ok := record[k]; ok {
			next[k] = prev
		}
		wanted[k] = true
	}

	// A Gateway the route left is cleaned up only once every wanted Gateway has settled, so a
	// route moving to a Gateway that isn't ready yet keeps serving where it was. A Gateway whose
	// listeners now reject the route is cleaned up at once: it no longer allows the route.
	stale := staleCleanupCandidates(route, record, wanted)
	cleanupPending := map[string]bool{}
	keepStale := func(c cleanupCandidate) {
		k := gatewayKeyString(c.key)
		if c.recorded {
			next[k] = c.entry
		}
		cleanupPending[k] = true
	}
	var removeNow []cleanupCandidate
	for _, c := range stale {
		if settled || rejected[gatewayKeyString(c.key)] {
			removeNow = append(removeNow, c)
		} else {
			keepStale(c)
		}
	}
	var cleanupErr error
	done := make([]bool, len(removeNow))
	errs := make([]error, len(removeNow))
	forEachBounded(len(removeNow), httpRouteGatewayConcurrency, func(i int) {
		done[i], errs[i] = r.removeFromGateway(ctx, removeNow[i], handle, log)
	})
	for i, c := range removeNow {
		if done[i] {
			continue
		}
		keepStale(c)
		requeue.at(requeueStaleCleanup)
		if errs[i] != nil && !isGatewayRESTError(errs[i]) && cleanupErr == nil {
			cleanupErr = errs[i]
		}
	}

	if err := r.writeDeployedGateways(ctx, route, next); err != nil {
		return ctrl.Result{}, err
	}
	keep := func(ref gatewayv1.ParentReference) bool {
		key := parentRefGatewayKey(ref, route.Namespace)
		if cleanupPending[gatewayKeyString(key)] {
			return true
		}
		if otherClass[key] {
			return false
		}
		return routeHasParentRef(route, ref)
	}
	if err := r.applyRouteStatus(ctx, route, status, keep); err != nil {
		return ctrl.Result{}, err
	}
	if cleanupErr != nil {
		return ctrl.Result{}, cleanupErr
	}
	return ctrl.Result{RequeueAfter: time.Duration(requeue)}, nil
}

// reconcileParentGateway attaches, builds and deploys the route for one Gateway. It must not
// write to the API server: Reconcile runs several of these at once and writes the results.
func (r *HTTPRouteReconciler) reconcileParentGateway(
	ctx context.Context,
	route *gatewayv1.HTTPRoute,
	p managedParentGateway,
	backends *HTTPRouteBackendResolution,
	handle string,
	prev deployedGatewayEntry,
	hadPrev bool,
	log *slog.Logger,
) parentGatewayOutcome {
	log = log.With(slog.String("parentNamespace", p.group.key.Namespace), slog.String("parentName", p.group.key.Name))
	var o parentGatewayOutcome
	accepted := string(gatewayv1.RouteConditionAccepted)

	// Each parentRef is checked against its own Gateway's listeners.
	var attachedRefs []gatewayv1.ParentReference
	for _, t := range p.group.targets {
		attached, reason, message := evaluateHTTPRouteAttachment(ctx, r.Client, p.gw, route, t.ref)
		if attached {
			attachedRefs = append(attachedRefs, t.ref)
			o.setCondition(route, t.ref, accepted, metav1.ConditionTrue, string(gatewayv1.RouteReasonAccepted), "Route attaches to parent Gateway listener")
			continue
		}
		log.Info("HTTPRoute does not attach to parent Gateway listener",
			slog.String("parentSection", normalizeParentRefSectionName(t.ref)),
			slog.String("reason", string(reason)),
			slog.String("message", message))
		o.setCondition(route, t.ref, accepted, metav1.ConditionFalse, string(reason), message)
	}
	if len(attachedRefs) == 0 {
		o.state = gatewayNotTarget
		return o
	}

	gwInfo, registered := registry.GetGatewayRegistry().Get(p.group.key.Namespace, p.group.key.Name)
	if !registered {
		log.Info("parent Gateway not registered yet; waiting for deploy")
		o.state = gatewayPending
		o.requeue = requeueGatewayNotRegistered
		return o
	}

	spec, err := BuildAPIConfigFromHTTPRoute(ctx, r.Client, p.gw, route, p.group.targets, backends, r.Config.GatewayAPI.ClusterDomain, log)
	if err != nil {
		log.Error("build API config", slog.Any("error", err))
		reason := "Retrying"
		o.state = gatewayPending
		o.requeue = requeueTransientRouteConfig
		if IsHTTPRouteBackendRefError(err) {
			reason = string(BackendRefResolutionReason(err))
			o.state, o.requeue = gatewayFailed, 0
		} else if IsInvalidHTTPRouteConfigError(err) {
			reason = "Invalid"
			o.state, o.requeue = gatewayFailed, 0
		}
		failure := &HTTPRouteBackendResolution{
			FirstFailureReason:  gatewayv1.RouteConditionReason(reason),
			FirstFailureMessage: err.Error(),
		}
		for _, t := range p.group.targets {
			o.conds = append(o.conds, parentRefCondition{ref: t.ref, cond: resolvedRefsCondition(route, failure)})
		}
		return o
	}

	apiYAML, err := gatewayclient.BuildRestAPIYAML(
		gatewayclient.ManagementArtifactAPIVersion,
		"RestApi",
		payloadMetadataForHTTPRoute(route, handle),
		*spec,
	)
	if err != nil {
		o.err = err
		return o
	}

	auth := httprouteAuthFunc(r.Client, log, gwInfo)
	ep, err := managementEndpoint(ctx, r.Client, gwInfo)
	if err != nil {
		return restFailureOutcome(o, route, attachedRefs, log, err)
	}
	exists, err := r.rest().Exists(ctx, ep, handle, auth)
	if err != nil {
		return restFailureOutcome(o, route, attachedRefs, log, err)
	}

	// Skip the deploy when the freshly built payload is identical to what we last deployed here and
	// the API still exists on the gateway. Reconciles fire for many reasons (watched Gateways/Services,
	// resyncs); without this guard every reconcile re-POSTs identical config, making the
	// gateway-controller re-snapshot and continuously re-push xDS (route-cache churn -> 5xx).
	// The exists check ensures we still (re)deploy after a gateway-controller restart loses the API.
	configHash := hashRestAPIPayload(apiYAML)
	if exists && hadPrev && prev.Hash == configHash && prev.Handle == handle {
		log.Debug("HTTPRoute config unchanged; skipping redeploy",
			slog.String("handle", handle),
			slog.String("configHash", configHash))
	} else {
		if err := r.rest().Deploy(ctx, ep, handle, apiYAML, exists, auth); err != nil {
			return restFailureOutcome(o, route, attachedRefs, log, err)
		}
		log.Info("HTTPRoute deployed to gateway",
			slog.String("parentGateway", p.group.key.Name),
			slog.String("handle", handle),
			slog.String("gatewayEndpoint", ep),
			slog.Bool("updated", exists),
			slog.Int("operations", len(spec.Operations)),
			slog.Int("apiLevelPolicies", len(spec.Policies)))
	}

	for _, ref := range attachedRefs {
		o.setCondition(route, ref, accepted, metav1.ConditionTrue, string(gatewayv1.RouteReasonAccepted), "Route accepted by platform gateway operator")
	}

	// The api-handle annotation changed: the new handle is live, so remove the old one.
	if hadPrev && prev.Handle != "" && prev.Handle != handle {
		ep, err := managementEndpoint(ctx, r.Client, gwInfo)
		if err == nil {
			err = r.rest().Delete(ctx, ep, prev.Handle, auth)
		}
		if err != nil {
			log.Error("failed to remove previous API handle from gateway",
				slog.Any("error", err),
				slog.String("previousHandle", prev.Handle))
			// Keeping the old record entry makes the next reconcile retry the removal.
			o.state = gatewayPending
			o.requeue = requeueGatewayRESTRetry
			return o
		}
		log.Info("removed previous API handle from gateway", slog.String("previousHandle", prev.Handle))
	}

	o.state = gatewayDeployed
	o.entry = deployedGatewayEntry{Hash: configHash, Handle: handle}
	return o
}

// restFailureOutcome reports a gateway REST failure on this Gateway's attached parentRefs only.
func restFailureOutcome(o parentGatewayOutcome, route *gatewayv1.HTTPRoute, attachedRefs []gatewayv1.ParentReference, log *slog.Logger, err error) parentGatewayOutcome {
	log.Error("gateway REST", slog.Any("error", err))
	reason := "Retrying"
	o.state = gatewayPending
	o.requeue = requeueGatewayRESTRetry
	var nonRetryable *gatewayclient.NonRetryableError
	if errors.As(err, &nonRetryable) {
		reason = "DeploymentFailed"
		o.state, o.requeue = gatewayFailed, 0
	}
	for _, ref := range attachedRefs {
		o.setCondition(route, ref, string(gatewayv1.RouteConditionAccepted), metav1.ConditionFalse, reason, err.Error())
	}
	return o
}

func resolvedRefsCondition(route *gatewayv1.HTTPRoute, resolution *HTTPRouteBackendResolution) metav1.Condition {
	cond := metav1.Condition{
		Type:               string(gatewayv1.RouteConditionResolvedRefs),
		ObservedGeneration: route.Generation,
		LastTransitionTime: metav1.Now(),
	}
	if resolution.AllResolved {
		cond.Status = metav1.ConditionTrue
		cond.Reason = string(gatewayv1.RouteReasonResolvedRefs)
		cond.Message = "All backend references resolved"
		return cond
	}
	cond.Status = metav1.ConditionFalse
	cond.Reason = string(resolution.FirstFailureReason)
	if cond.Reason == "" {
		cond.Reason = string(gatewayv1.RouteReasonRefNotPermitted)
	}
	cond.Message = resolution.FirstFailureMessage
	return cond
}

func (r *HTTPRouteReconciler) reconcileDeletion(ctx context.Context, route *gatewayv1.HTTPRoute, log *slog.Logger) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(route, httprouteFinalizer) {
		return ctrl.Result{}, nil
	}

	handle := DefaultHTTPRouteAPIHandle(route)
	candidates := deletionCleanupCandidates(route, deployedGatewaysFromRoute(route))
	done := make([]bool, len(candidates))
	errs := make([]error, len(candidates))
	forEachBounded(len(candidates), httpRouteGatewayConcurrency, func(i int) {
		done[i], errs[i] = r.removeFromGateway(ctx, candidates[i], handle, log)
	})
	waiting := false
	var failures []error
	for i := range candidates {
		if errs[i] != nil {
			failures = append(failures, errs[i])
		} else if !done[i] {
			waiting = true
		}
	}
	if len(failures) > 0 {
		return ctrl.Result{}, errors.Join(failures...)
	}
	if waiting {
		return ctrl.Result{RequeueAfter: requeueGatewayNotRegistered}, nil
	}
	if len(candidates) == 0 {
		log.Info("no Gateway to clean up for HTTPRoute deletion; removing finalizer")
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(route), route); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	return r.removeHTTPRouteFinalizer(ctx, route)
}

func (r *HTTPRouteReconciler) removeHTTPRouteFinalizer(ctx context.Context, route *gatewayv1.HTTPRoute) (ctrl.Result, error) {
	controllerutil.RemoveFinalizer(route, httprouteFinalizer)
	if err := r.Update(ctx, route); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// removeFromGateway deletes the route's API from one Gateway. It reports done=false when the
// removal must be retried: err is set when a call failed, and nil while waiting for the Gateway
// to register with the operator.
func (r *HTTPRouteReconciler) removeFromGateway(ctx context.Context, c cleanupCandidate, handle string, log *slog.Logger) (bool, error) {
	log = log.With(slog.String("gatewayNamespace", c.key.Namespace), slog.String("gatewayName", c.key.Name))
	gw := &gatewayv1.Gateway{}
	if err := r.Get(ctx, c.key, gw); err != nil {
		if apierrors.IsNotFound(err) {
			// A deleted Gateway's runtime is uninstalled with it, so nothing is left to remove.
			log.Info("Gateway no longer exists; nothing to remove")
			return true, nil
		}
		return false, err
	}

	info, registered := registry.GetGatewayRegistry().Get(c.key.Namespace, c.key.Name)
	handles := handlesToRemove(c.entry, c.recorded, handle)
	if !gw.DeletionTimestamp.IsZero() {
		// Its runtime is being uninstalled: try once, but don't hold the route on it.
		if registered {
			auth := httprouteAuthFunc(r.Client, log, info)
			for _, h := range handles {
				ep, err := managementEndpoint(ctx, r.Client, info)
				if err == nil {
					err = r.rest().Delete(ctx, ep, h, auth)
				}
				if err != nil {
					log.Info("skip API removal from Gateway being deleted", slog.String("handle", h), slog.Any("error", err))
				}
			}
		}
		return true, nil
	}
	if registered {
		auth := httprouteAuthFunc(r.Client, log, info)
		for _, h := range handles {
			ep, err := managementEndpoint(ctx, r.Client, info)
			if err == nil {
				err = r.rest().Delete(ctx, ep, h, auth)
			}
			if err != nil {
				log.Error("failed to remove HTTPRoute API from Gateway", slog.Any("error", err), slog.String("handle", h))
				return false, &gatewayRESTError{err: err}
			}
		}
		log.Info("removed HTTPRoute API from Gateway", slog.Any("handles", handles))
		return true, nil
	}
	// A Gateway of ours registers once its runtime is up, so wait for one the route was deployed
	// to or still lists. Any other Gateway never registers, and waiting would block forever.
	if r.Config.ManagedGatewayClass(string(gw.Spec.GatewayClassName)) && (c.recorded || c.specParent) {
		log.Info("Gateway not registered yet; retrying HTTPRoute API removal")
		return false, nil
	}
	return true, nil
}

// gatewayRESTError marks a failed gateway-controller call, which is retried on a fixed delay
// rather than returned as a reconcile error.
type gatewayRESTError struct{ err error }

func (e *gatewayRESTError) Error() string { return e.err.Error() }
func (e *gatewayRESTError) Unwrap() error { return e.err }

func isGatewayRESTError(err error) bool {
	var restErr *gatewayRESTError
	return errors.As(err, &restErr)
}

// staleCleanupCandidates lists Gateways the route may still be deployed to but no longer wants:
// those in the record, plus those where our status says it was accepted (a backup for a lost
// record).
func staleCleanupCandidates(route *gatewayv1.HTTPRoute, record deployedGateways, wanted map[string]bool) []cleanupCandidate {
	specKeys := map[string]bool{}
	for _, t := range parentGatewayRefs(route) {
		specKeys[gatewayKeyString(t.key)] = true
	}
	byKey := map[string]*cleanupCandidate{}
	var order []string
	add := func(key client.ObjectKey) *cleanupCandidate {
		k := gatewayKeyString(key)
		if c, ok := byKey[k]; ok {
			return c
		}
		c := &cleanupCandidate{key: key, specParent: specKeys[k]}
		byKey[k] = c
		order = append(order, k)
		return c
	}
	for _, k := range sortedRecordKeys(record) {
		if wanted[k] {
			continue
		}
		key, _ := parseGatewayKey(k)
		c := add(key)
		c.entry, c.recorded = record[k], true
	}
	for _, key := range acceptedStatusGatewayKeys(route) {
		if wanted[gatewayKeyString(key)] {
			continue
		}
		add(key)
	}
	out := make([]cleanupCandidate, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out
}

// deletionCleanupCandidates lists every Gateway a deleted route may be deployed to.
func deletionCleanupCandidates(route *gatewayv1.HTTPRoute, record deployedGateways) []cleanupCandidate {
	out := staleCleanupCandidates(route, record, nil)
	seen := map[string]bool{}
	for _, c := range out {
		seen[gatewayKeyString(c.key)] = true
	}
	for _, t := range parentGatewayRefs(route) {
		k := gatewayKeyString(t.key)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, cleanupCandidate{key: t.key, specParent: true})
	}
	return out
}

func sortedRecordKeys(record deployedGateways) []string {
	keys := make([]string, 0, len(record))
	for k := range record {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// acceptedStatusGatewayKeys returns the Gateways where our status entries say the route was accepted.
func acceptedStatusGatewayKeys(route *gatewayv1.HTTPRoute) []client.ObjectKey {
	seen := map[client.ObjectKey]bool{}
	var out []client.ObjectKey
	for _, ps := range route.Status.Parents {
		if ps.ControllerName != PlatformGatewayControllerName {
			continue
		}
		if normalizeParentRefKind(ps.ParentRef) != "" && normalizeParentRefKind(ps.ParentRef) != "Gateway" {
			continue
		}
		if !meta.IsStatusConditionTrue(ps.Conditions, string(gatewayv1.RouteConditionAccepted)) {
			continue
		}
		key := parentRefGatewayKey(ps.ParentRef, route.Namespace)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

func parentRefGatewayKey(ref gatewayv1.ParentReference, routeNamespace string) client.ObjectKey {
	return client.ObjectKey{Namespace: normalizeParentRefNamespace(ref, routeNamespace), Name: string(ref.Name)}
}

func routeHasParentRef(route *gatewayv1.HTTPRoute, ref gatewayv1.ParentReference) bool {
	for _, p := range route.Spec.ParentRefs {
		if parentRefMatches(p, ref, route.Namespace) {
			return true
		}
	}
	return false
}

// hashRestAPIPayload returns a stable hex digest of the deployed RestApi payload, used to detect
// whether an HTTPRoute's generated config actually changed between reconciles.
func hashRestAPIPayload(apiYAML []byte) string {
	sum := sha256.Sum256(apiYAML)
	return hex.EncodeToString(sum[:])
}

// writeDeployedGateways stores the deployment record on the route, patching only when it changed.
func (r *HTTPRouteReconciler) writeDeployedGateways(ctx context.Context, route *gatewayv1.HTTPRoute, rec deployedGateways) error {
	latest := &gatewayv1.HTTPRoute{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(route), latest); err != nil {
		return err
	}
	base := latest.DeepCopy()
	changed := false
	for k, v := range deployedGatewaysAnnotations(rec, DefaultHTTPRouteAPIHandle(route)) {
		cur, has := latest.Annotations[k]
		if v == nil {
			if has {
				delete(latest.Annotations, k)
				changed = true
			}
			continue
		}
		if has && cur == *v {
			continue
		}
		if latest.Annotations == nil {
			latest.Annotations = map[string]string{}
		}
		latest.Annotations[k] = *v
		changed = true
	}
	if !changed {
		return nil
	}
	return r.Patch(ctx, latest, client.MergeFrom(base))
}

func parentGatewayRefs(route *gatewayv1.HTTPRoute) []gatewayParentTarget {
	out := make([]gatewayParentTarget, 0, len(route.Spec.ParentRefs))
	for _, p := range route.Spec.ParentRefs {
		if p.Name == "" {
			continue
		}
		if p.Kind != nil && string(*p.Kind) != "Gateway" {
			continue
		}
		if p.Group != nil && string(*p.Group) != gatewayv1.GroupName {
			continue
		}
		ns := route.Namespace
		if p.Namespace != nil {
			ns = string(*p.Namespace)
		}
		out = append(out, gatewayParentTarget{
			key: client.ObjectKey{Namespace: ns, Name: string(p.Name)},
			ref: p,
		})
	}
	return out
}

// forEachBounded calls fn for every index in [0, n), running at most limit at once.
func forEachBounded(n, limit int, fn func(i int)) {
	if n == 1 {
		fn(0)
		return
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(i)
		}(i)
	}
	wg.Wait()
}

func httprouteAuthFunc(c client.Client, log *slog.Logger, info *registry.GatewayInfo) gatewayclient.AuthHeaderFunc {
	return verifiedAuth(c, info, func(ctx context.Context, req *http.Request) error {
		authConfig, err := auth.GetAuthSettingsForRegistryGateway(ctx, c, info)
		if err != nil {
			if log != nil {
				log.Warn("auth config lookup failed", slog.Any("error", err))
			}
			return err
		}
		var username, password string
		var ok bool
		if authConfig != nil {
			username, password, ok = auth.GetBasicAuthCredentials(authConfig)
		}
		if !ok {
			username, password = auth.GetDefaultBasicAuthCredentials()
		}
		req.Header.Set("Authorization", "Basic "+auth.EncodeBasicAuth(username, password))
		return nil
	})
}

func payloadMetadataForHTTPRoute(route *gatewayv1.HTTPRoute, handle string) gatewayclient.RestAPIPayloadMetadata {
	md := gatewayclient.RestAPIPayloadMetadata{Name: handle}
	if route == nil {
		return md
	}
	if len(route.Annotations) > 0 {
		md.Annotations = make(map[string]string, len(route.Annotations))
		for k, v := range route.Annotations {
			// Skip the operator's own reconcile-bookkeeping annotations. They are written
			// back onto the HTTPRoute after every deploy, so including them here would make
			// the payload — and thus its config hash — depend on the previous deploy's hash.
			// That is self-referential: the hash never matches the stored value, the dedup
			// guard never trips, and the reconciler redeploys in a hot loop (~50/s). These
			// annotations are operator-internal and meaningless to the gateway-controller.
			if isOperatorBookkeepingAnnotation(k) {
				continue
			}
			md.Annotations[k] = v
		}
		if len(md.Annotations) == 0 {
			md.Annotations = nil
		}
	}
	return md
}

// isOperatorBookkeepingAnnotation reports whether an annotation key is one the operator
// writes onto the HTTPRoute during reconcile (deployment record, legacy hash and parent). These
// must never feed into the deployed payload/config hash — see payloadMetadataForHTTPRoute.
func isOperatorBookkeepingAnnotation(key string) bool {
	switch key {
	case AnnHTTPRouteDeployedGateways, AnnHTTPRouteLastDeployedConfigHash, AnnHTTPRouteLastDeployedParentGateway:
		return true
	default:
		return false
	}
}

func parentRefMatches(a, b gatewayv1.ParentReference, routeNamespace string) bool {
	if a.Name != b.Name {
		return false
	}
	if normalizeParentRefNamespace(a, routeNamespace) != normalizeParentRefNamespace(b, routeNamespace) {
		return false
	}
	if normalizeParentRefSectionName(a) != normalizeParentRefSectionName(b) {
		return false
	}
	if normalizeParentRefPort(a) != normalizeParentRefPort(b) {
		return false
	}
	if normalizeParentRefKind(a) != normalizeParentRefKind(b) {
		return false
	}
	if normalizeParentRefGroup(a) != normalizeParentRefGroup(b) {
		return false
	}
	return true
}

func normalizeParentRefNamespace(ref gatewayv1.ParentReference, routeNamespace string) string {
	if ref.Namespace == nil || strings.TrimSpace(string(*ref.Namespace)) == "" {
		return routeNamespace
	}
	return string(*ref.Namespace)
}

func normalizeParentRefSectionName(ref gatewayv1.ParentReference) string {
	if ref.SectionName == nil {
		return ""
	}
	return string(*ref.SectionName)
}

func normalizeParentRefPort(ref gatewayv1.ParentReference) int32 {
	if ref.Port == nil {
		return 0
	}
	return int32(*ref.Port)
}

func normalizeParentRefKind(ref gatewayv1.ParentReference) string {
	if ref.Kind == nil {
		return ""
	}
	return string(*ref.Kind)
}

func normalizeParentRefGroup(ref gatewayv1.ParentReference) string {
	if ref.Group == nil {
		return ""
	}
	return string(*ref.Group)
}

// SetupWithManager wires HTTPRoute reconciliation and Service -> HTTPRoute watch.
func (r *HTTPRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := contctrl.Options{MaxConcurrentReconciles: r.Config.Reconciliation.MaxConcurrentReconciles}
	if opts.MaxConcurrentReconciles <= 0 {
		opts.MaxConcurrentReconciles = 1
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(opts).
		For(&gatewayv1.HTTPRoute{}).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueHTTPRoutesForService),
			builder.WithPredicates(serviceMutationPredicate()),
		).
		Watches(
			&apiv1.APIPolicy{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueHTTPRouteForAPIPolicy),
			builder.WithPredicates(apiPolicyMutationPredicate()),
		).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueHTTPRoutesForSecret),
			builder.WithPredicates(secretMutationPredicate()),
		).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueHTTPRoutesForConfigMap),
			builder.WithPredicates(configMapMutationPredicate()),
		).
		Watches(
			&gatewayv1beta1.ReferenceGrant{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueHTTPRoutesForReferenceGrant),
			builder.WithPredicates(referenceGrantMutationPredicate()),
		).
		Watches(
			&gatewayv1.Gateway{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueHTTPRoutesForGateway),
		).
		Complete(r)
}

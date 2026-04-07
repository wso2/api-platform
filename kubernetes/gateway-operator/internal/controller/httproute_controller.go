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
	"net/http"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	contctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/auth"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

const (
	httprouteFinalizer      = "gateway.api-platform.wso2.com/httproute-finalizer"
	httprouteControllerName = gatewayv1.GatewayController("gateway.api-platform.wso2.com/gateway-operator")
)

// HTTPRouteReconciler maps HTTPRoute + backends to gateway-controller REST APIs.
type HTTPRouteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.OperatorConfig
	Logger *zap.Logger
}

type gatewayParentTarget struct {
	key client.ObjectKey
	ref gatewayv1.ParentReference
}

// NewHTTPRouteReconciler creates a reconciler for HTTPRoute.
func NewHTTPRouteReconciler(cl client.Client, scheme *runtime.Scheme, cfg *config.OperatorConfig, logger *zap.Logger) *HTTPRouteReconciler {
	return &HTTPRouteReconciler{
		Client: cl,
		Scheme: scheme,
		Config: cfg,
		Logger: logger,
	}
}

//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/finalizers,verbs=update
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch

func (r *HTTPRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	route := &gatewayv1.HTTPRoute{}
	if err := r.Get(ctx, req.NamespacedName, route); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log := r.Logger.With(zap.String("controller", "HTTPRoute"), zap.String("namespace", req.Namespace), zap.String("name", req.Name))

	if !route.DeletionTimestamp.IsZero() {
		parentKey, _, _ := parentGatewayRef(route)
		return r.reconcileDeletion(ctx, route, parentKey, log)
	}

	parentTargets := parentGatewayRefs(route)
	if len(parentTargets) == 0 {
		return ctrl.Result{}, nil
	}
	if len(parentTargets) > 1 {
		msg := "HTTPRoute with multiple Gateway parentRefs is not supported; use a single parent Gateway"
		log.Info("invalid HTTPRoute parentRefs", zap.Int("gatewayParents", len(parentTargets)), zap.String("reason", msg))
		for _, target := range parentTargets {
			_ = r.patchHTTPRouteParentCondition(ctx, route, target.ref, metav1.Condition{
				Type:    string(gatewayv1.RouteConditionResolvedRefs),
				Status:  metav1.ConditionFalse,
				Reason:  "Invalid",
				Message: msg,
			})
		}
		return ctrl.Result{}, nil
	}
	parentKey := parentTargets[0].key
	parentRef := parentTargets[0].ref

	parentGW := &gatewayv1.Gateway{}
	if err := r.Get(ctx, parentKey, parentGW); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if !r.Config.ManagedGatewayClass(string(parentGW.Spec.GatewayClassName)) {
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(route, httprouteFinalizer) {
		controllerutil.AddFinalizer(route, httprouteFinalizer)
		if err := r.Update(ctx, route); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	gwInfo, regOK := registry.GetGatewayRegistry().Get(parentKey.Namespace, parentKey.Name)
	if !regOK {
		log.Info("parent Gateway not registered yet; waiting")
		_ = r.patchHTTPRouteParentCondition(ctx, route, parentRef, metav1.Condition{
			Type:    string(gatewayv1.RouteConditionAccepted),
			Status:  metav1.ConditionFalse,
			Reason:  "GatewayPending",
			Message: "Platform gateway controller endpoint not registered",
		})
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	spec, err := BuildAPIConfigFromHTTPRoute(ctx, r.Client, route)
	if err != nil {
		log.Error("build API config", zap.Error(err))
		requeueAfter := time.Duration(0)
		reason := "Invalid"
		if !IsInvalidHTTPRouteConfigError(err) {
			requeueAfter = 30 * time.Second
			reason = "Retrying"
		}
		_ = r.patchHTTPRouteParentCondition(ctx, route, parentRef, metav1.Condition{
			Type:    string(gatewayv1.RouteConditionResolvedRefs),
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: err.Error(),
		})
		if requeueAfter > 0 {
			return ctrl.Result{RequeueAfter: requeueAfter}, nil
		}
		return ctrl.Result{}, nil
	}

	handle := DefaultHTTPRouteAPIHandle(route)
	apiYAML, err := gatewayclient.BuildRestAPIYAML(apiv1.GroupVersion.String(), "RestApi", handle, *spec)
	if err != nil {
		return ctrl.Result{}, err
	}

	auth := httprouteAuthFunc(r.Client, log, gwInfo)
	ep := gwInfo.GetGatewayServiceEndpoint()
	exists, err := gatewayclient.RestAPIExists(ctx, ep, handle, auth)
	if err != nil {
		return r.handleRESTError(ctx, route, parentRef, log, err)
	}

	if err := gatewayclient.DeployRestAPI(ctx, ep, handle, apiYAML, exists, auth); err != nil {
		return r.handleRESTError(ctx, route, parentRef, log, err)
	}
	log.Info("HTTPRoute deployed to gateway",
		zap.String("parentGateway", parentKey.Name),
		zap.String("handle", handle),
		zap.String("gatewayEndpoint", ep),
		zap.Bool("updated", exists))

	_ = r.patchHTTPRouteParentCondition(ctx, route, parentRef,
		metav1.Condition{
			Type:               string(gatewayv1.RouteConditionResolvedRefs),
			Status:             metav1.ConditionTrue,
			Reason:             "ResolvedRefs",
			Message:            "Backend references resolved and API deployed to platform gateway",
			ObservedGeneration: route.Generation,
			LastTransitionTime: metav1.Now(),
		},
		metav1.Condition{
			Type:               string(gatewayv1.RouteConditionAccepted),
			Status:             metav1.ConditionTrue,
			Reason:             string(gatewayv1.RouteReasonAccepted),
			Message:            "Route accepted by platform gateway operator",
			ObservedGeneration: route.Generation,
			LastTransitionTime: metav1.Now(),
		},
	)
	return ctrl.Result{}, nil
}

func (r *HTTPRouteReconciler) handleRESTError(ctx context.Context, route *gatewayv1.HTTPRoute, parentRef gatewayv1.ParentReference, log *zap.Logger, err error) (ctrl.Result, error) {
	log.Error("gateway REST", zap.Error(err))
	var msg string
	switch e := err.(type) {
	case *gatewayclient.NonRetryableError:
		msg = e.Error()
		_ = r.patchHTTPRouteParentCondition(ctx, route, parentRef, metav1.Condition{
			Type:    string(gatewayv1.RouteConditionAccepted),
			Status:  metav1.ConditionFalse,
			Reason:  "DeploymentFailed",
			Message: msg,
		})
		return ctrl.Result{}, nil
	default:
		msg = err.Error()
	}
	_ = r.patchHTTPRouteParentCondition(ctx, route, parentRef, metav1.Condition{
		Type:    string(gatewayv1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  "Retrying",
		Message: msg,
	})
	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
}

func (r *HTTPRouteReconciler) reconcileDeletion(ctx context.Context, route *gatewayv1.HTTPRoute, parentKey client.ObjectKey, log *zap.Logger) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(route, httprouteFinalizer) {
		return ctrl.Result{}, nil
	}

	handle := DefaultHTTPRouteAPIHandle(route)
	gwInfo, ok := registry.GetGatewayRegistry().Get(parentKey.Namespace, parentKey.Name)
	if !ok {
		log.Info("parent Gateway not registered during delete; retrying before finalizer removal",
			zap.String("parentNamespace", parentKey.Namespace),
			zap.String("parentName", parentKey.Name))
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	auth := httprouteAuthFunc(r.Client, log, gwInfo)
	if err := gatewayclient.DeleteRestAPI(ctx, gwInfo.GetGatewayServiceEndpoint(), handle, auth); err != nil {
		log.Error("delete REST API from gateway", zap.Error(err))
		return ctrl.Result{}, err
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(route), route); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	controllerutil.RemoveFinalizer(route, httprouteFinalizer)
	if err := r.Update(ctx, route); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
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

func parentGatewayRef(route *gatewayv1.HTTPRoute) (client.ObjectKey, gatewayv1.ParentReference, bool) {
	targets := parentGatewayRefs(route)
	if len(targets) == 0 {
		return client.ObjectKey{}, gatewayv1.ParentReference{}, false
	}
	return targets[0].key, targets[0].ref, true
}

func httprouteAuthFunc(c client.Client, log *zap.Logger, info *registry.GatewayInfo) gatewayclient.AuthHeaderFunc {
	return func(ctx context.Context, req *http.Request) error {
		authConfig, err := auth.GetAuthSettingsForRegistryGateway(ctx, c, info)
		if err != nil {
			log.Warn("auth config", zap.Error(err))
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
	}
}

func (r *HTTPRouteReconciler) patchHTTPRouteParentCondition(ctx context.Context, route *gatewayv1.HTTPRoute, parentRef gatewayv1.ParentReference, conds ...metav1.Condition) error {
	latest := &gatewayv1.HTTPRoute{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(route), latest); err != nil {
		return err
	}
	base := latest.DeepCopy()

	idx := -1
	for i := range latest.Status.Parents {
		if parentRefMatches(latest.Status.Parents[i].ParentRef, parentRef) &&
			latest.Status.Parents[i].ControllerName == httprouteControllerName {
			idx = i
			break
		}
	}
	if idx < 0 {
		latest.Status.Parents = append(latest.Status.Parents, gatewayv1.RouteParentStatus{
			ParentRef:      parentRef,
			ControllerName: httprouteControllerName,
		})
		idx = len(latest.Status.Parents) - 1
	}
	for _, c := range conds {
		meta.SetStatusCondition(&latest.Status.Parents[idx].Conditions, c)
	}
	return r.Status().Patch(ctx, latest, client.MergeFrom(base))
}

func parentRefMatches(a, b gatewayv1.ParentReference) bool {
	if a.Name != b.Name {
		return false
	}
	nsA, nsB := "", ""
	if a.Namespace != nil {
		nsA = string(*a.Namespace)
	}
	if b.Namespace != nil {
		nsB = string(*b.Namespace)
	}
	return nsA == nsB
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
		Complete(r)
}

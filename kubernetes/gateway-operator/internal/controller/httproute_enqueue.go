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
	"encoding/json"
	"strings"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func serviceMutationPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

func apiPolicyMutationPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

func secretMutationPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return secretWatchFilter(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return secretWatchFilter(e.ObjectNew) || secretWatchFilter(e.ObjectOld)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return secretWatchFilter(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

func secretWatchFilter(o client.Object) bool {
	s, ok := o.(*corev1.Secret)
	if !ok {
		return false
	}
	// Avoid reconciling on high-churn system secrets.
	if s.Type == corev1.SecretTypeServiceAccountToken {
		return false
	}
	return true
}

// httpRouteRequestForAPIPolicyTarget returns a reconcile request for the HTTPRoute referenced by ap.Spec.targetRef.
func httpRouteRequestForAPIPolicyTarget(ap *apiv1.APIPolicy) (reconcile.Request, bool) {
	if ap.Spec.TargetRef == nil {
		return reconcile.Request{}, false
	}
	ref := *ap.Spec.TargetRef
	if strings.TrimSpace(ref.Kind) != "HTTPRoute" {
		return reconcile.Request{}, false
	}
	if strings.TrimSpace(ref.Group) != gatewayv1.GroupName {
		return reconcile.Request{}, false
	}
	routeName := strings.TrimSpace(ref.Name)
	if routeName == "" {
		return reconcile.Request{}, false
	}
	ns := ap.Namespace
	if ref.Namespace != nil && strings.TrimSpace(*ref.Namespace) != "" {
		ns = strings.TrimSpace(*ref.Namespace)
	}
	return reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: routeName}}, true
}

func httpRouteReferencesAPIPolicy(route *gatewayv1.HTTPRoute, policyName string) bool {
	policyName = strings.TrimSpace(policyName)
	if policyName == "" {
		return false
	}
	for _, rule := range route.Spec.Rules {
		for _, f := range rule.Filters {
			if f.Type != gatewayv1.HTTPRouteFilterExtensionRef || f.ExtensionRef == nil {
				continue
			}
			ref := f.ExtensionRef
			if string(ref.Group) != apiv1.GroupVersion.Group || string(ref.Kind) != "APIPolicy" {
				continue
			}
			if string(ref.Name) == policyName {
				return true
			}
		}
	}
	return false
}

func (r *HTTPRouteReconciler) enqueueHTTPRoutesReferencingAPIPolicy(ctx context.Context, ap *apiv1.APIPolicy) []reconcile.Request {
	routes := &gatewayv1.HTTPRouteList{}
	if err := r.List(ctx, routes, client.InNamespace(ap.Namespace)); err != nil {
		if r.Logger != nil {
			r.Logger.Error("watch: list HTTPRoutes for APIPolicy ExtensionRef enqueue",
				zap.Error(err),
				zap.String("apiPolicy", client.ObjectKeyFromObject(ap).String()))
		}
		return nil
	}
	seen := make(map[types.NamespacedName]struct{})
	var reqs []reconcile.Request
	for i := range routes.Items {
		rt := &routes.Items[i]
		if !httpRouteReferencesAPIPolicy(rt, ap.Name) {
			continue
		}
		key := client.ObjectKeyFromObject(rt)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		reqs = append(reqs, reconcile.Request{NamespacedName: key})
	}
	if r.Logger != nil && len(reqs) > 0 {
		names := make([]string, 0, len(reqs))
		for _, q := range reqs {
			names = append(names, q.NamespacedName.String())
		}
		r.Logger.Info("watch: APIPolicy changed; enqueue HTTPRoutes referencing ExtensionRef",
			zap.String("controller", "HTTPRoute"),
			zap.String("apiPolicy", client.ObjectKeyFromObject(ap).String()),
			zap.Strings("httpRoutes", names))
	}
	return reqs
}

// enqueueHTTPRouteForAPIPolicy maps an APIPolicy event to reconcile of its target HTTPRoute so policy
// edits (and deletes) redeploy the API without requiring an HTTPRoute spec change.
func (r *HTTPRouteReconciler) enqueueHTTPRouteForAPIPolicy(ctx context.Context, obj client.Object) []reconcile.Request {
	ap, ok := obj.(*apiv1.APIPolicy)
	if !ok {
		return nil
	}
	if req, ok := httpRouteRequestForAPIPolicyTarget(ap); ok {
		if r.Logger != nil {
			r.Logger.Info("watch: APIPolicy changed; enqueue HTTPRoute",
				zap.String("controller", "HTTPRoute"),
				zap.String("apiPolicy", client.ObjectKeyFromObject(ap).String()),
				zap.String("targetHTTPRoute", req.NamespacedName.String()))
		}
		return []reconcile.Request{req}
	}
	return r.enqueueHTTPRoutesReferencingAPIPolicy(ctx, ap)
}

// enqueueHTTPRoutesForSecret enqueues HTTPRoutes whose APIPolicy params reference the Secret via valueFrom
// (same namespace as the APIPolicy unless valueFrom.namespace is set).
func (r *HTTPRouteReconciler) enqueueHTTPRoutesForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}
	secretNS := secret.Namespace
	secretName := secret.Name

	list := &apiv1.APIPolicyList{}
	if err := r.List(ctx, list); err != nil {
		if r.Logger != nil {
			r.Logger.Error("watch: list APIPolicies for Secret enqueue",
				zap.Error(err),
				zap.String("secret", client.ObjectKeyFromObject(secret).String()))
		}
		return nil
	}
	seen := make(map[types.NamespacedName]struct{})
	var reqs []reconcile.Request
	for i := range list.Items {
		ap := &list.Items[i]
		if !apiPolicyReferencesSecret(ap, secretNS, secretName) {
			continue
		}
		var toAdd []reconcile.Request
		if req, ok := httpRouteRequestForAPIPolicyTarget(ap); ok {
			toAdd = append(toAdd, req)
		} else {
			toAdd = append(toAdd, r.enqueueHTTPRoutesReferencingAPIPolicy(ctx, ap)...)
		}
		for _, req := range toAdd {
			if _, dup := seen[req.NamespacedName]; dup {
				continue
			}
			seen[req.NamespacedName] = struct{}{}
			reqs = append(reqs, req)
		}
	}
	if r.Logger != nil && len(reqs) > 0 {
		ns := make([]string, 0, len(reqs))
		for _, q := range reqs {
			ns = append(ns, q.NamespacedName.String())
		}
		r.Logger.Info("watch: Secret changed; enqueue HTTPRoutes referencing valueFrom",
			zap.String("controller", "HTTPRoute"),
			zap.String("secret", client.ObjectKeyFromObject(secret).String()),
			zap.Strings("httpRoutes", ns))
	}
	return reqs
}

func apiPolicyReferencesSecret(ap *apiv1.APIPolicy, secretNS, secretName string) bool {
	defaultNS := ap.Namespace
	for i := range ap.Spec.Policies {
		p := &ap.Spec.Policies[i]
		if p.Params == nil || len(p.Params.Raw) == 0 {
			continue
		}
		var root interface{}
		if err := json.Unmarshal(p.Params.Raw, &root); err != nil {
			continue
		}
		if jsonTreeReferencesSecret(root, secretNS, secretName, defaultNS) {
			return true
		}
	}
	return false
}

func jsonTreeReferencesSecret(v interface{}, secretNS, secretName string, defaultNS string) bool {
	switch x := v.(type) {
	case map[string]interface{}:
		if vf, ok := x["valueFrom"]; ok {
			if m, ok := vf.(map[string]interface{}); ok && valueFromMatchesSecret(m, secretNS, secretName, defaultNS) {
				return true
			}
		}
		for _, child := range x {
			if jsonTreeReferencesSecret(child, secretNS, secretName, defaultNS) {
				return true
			}
		}
	case []interface{}:
		for _, el := range x {
			if jsonTreeReferencesSecret(el, secretNS, secretName, defaultNS) {
				return true
			}
		}
	}
	return false
}

func valueFromMatchesSecret(vf map[string]interface{}, secretNS, secretName, defaultNS string) bool {
	name, _ := vf["name"].(string)
	if strings.TrimSpace(name) != secretName {
		return false
	}
	ns := defaultNS
	if n, ok := vf["namespace"].(string); ok && strings.TrimSpace(n) != "" {
		ns = strings.TrimSpace(n)
	}
	return ns == secretNS
}

func (r *HTTPRouteReconciler) enqueueHTTPRoutesForService(ctx context.Context, obj client.Object) []reconcile.Request {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		return nil
	}
	routes := &gatewayv1.HTTPRouteList{}
	if err := r.List(ctx, routes); err != nil {
		if r.Logger != nil {
			r.Logger.Error("watch: list HTTPRoutes for Service enqueue",
				zap.Error(err),
				zap.String("service", client.ObjectKeyFromObject(svc).String()))
		}
		return nil
	}
	var requests []reconcile.Request
	for i := range routes.Items {
		rt := &routes.Items[i]
		if httpRouteReferencesService(rt, svc.Namespace, svc.Name) {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(rt),
			})
		}
	}
	if r.Logger != nil && len(requests) > 0 {
		ns := make([]string, 0, len(requests))
		for _, q := range requests {
			ns = append(ns, q.NamespacedName.String())
		}
		r.Logger.Info("watch: Service changed; enqueue HTTPRoutes",
			zap.String("controller", "HTTPRoute"),
			zap.String("service", client.ObjectKeyFromObject(svc).String()),
			zap.Strings("httpRoutes", ns))
	}
	return requests
}

func httpRouteReferencesService(route *gatewayv1.HTTPRoute, svcNS, svcName string) bool {
	for _, rule := range route.Spec.Rules {
		for _, b := range rule.BackendRefs {
			if string(b.Name) != svcName {
				continue
			}
			ns := route.Namespace
			if b.Namespace != nil {
				ns = string(*b.Namespace)
			}
			if ns == svcNS {
				return true
			}
		}
	}
	return false
}

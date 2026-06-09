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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type listenerAttachmentContext struct {
	listener        gatewayv1.Listener
	allowedNs       map[string]struct{}
	allowsHTTPRoute bool
}

// computeAttachedHTTPRoutesByListener counts Accepted HTTPRoutes attached to each listener on the Gateway.
func computeAttachedHTTPRoutesByListener(ctx context.Context, cl client.Client, gw *gatewayv1.Gateway) (map[gatewayv1.SectionName]int32, error) {
	counts := make(map[gatewayv1.SectionName]int32, len(gw.Spec.Listeners))
	contexts := make([]listenerAttachmentContext, 0, len(gw.Spec.Listeners))

	for _, listener := range gw.Spec.Listeners {
		counts[listener.Name] = 0
		allowedNs, err := namespacesAllowedByListener(ctx, cl, listener, gw)
		if err != nil {
			return nil, err
		}
		allowedSet := make(map[string]struct{}, len(allowedNs))
		for _, ns := range allowedNs {
			allowedSet[ns] = struct{}{}
		}
		contexts = append(contexts, listenerAttachmentContext{
			listener:        listener,
			allowedNs:       allowedSet,
			allowsHTTPRoute: listenerAllowsHTTPRouteKind(listener),
		})
	}

	routeList := &gatewayv1.HTTPRouteList{}
	if err := cl.List(ctx, routeList); err != nil {
		return nil, err
	}

	for i := range routeList.Items {
		route := &routeList.Items[i]
		attachedListeners := make(map[gatewayv1.SectionName]struct{})

		for _, parentRef := range route.Spec.ParentRefs {
			if !parentRefTargetsGateway(parentRef, gw, route.Namespace) {
				continue
			}
			if !httpRouteAcceptedForParentRef(route, parentRef) {
				continue
			}

			for _, lctx := range contexts {
				if !lctx.allowsHTTPRoute {
					continue
				}
				if _, ok := lctx.allowedNs[route.Namespace]; !ok {
					continue
				}
				if !parentRefAttachesToListener(parentRef, lctx.listener) {
					continue
				}
				attachedListeners[lctx.listener.Name] = struct{}{}
			}
		}

		for name := range attachedListeners {
			counts[name]++
		}
	}

	return counts, nil
}

func namespacesAllowedByListener(ctx context.Context, cl client.Client, listener gatewayv1.Listener, gw *gatewayv1.Gateway) ([]string, error) {
	gwNs := gw.Namespace
	if gwNs == "" {
		gwNs = "default"
	}

	if listener.AllowedRoutes == nil || listener.AllowedRoutes.Namespaces == nil || listener.AllowedRoutes.Namespaces.From == nil {
		return []string{gwNs}, nil
	}

	switch *listener.AllowedRoutes.Namespaces.From {
	case gatewayv1.NamespacesFromAll:
		nsList := &corev1.NamespaceList{}
		if err := cl.List(ctx, nsList); err != nil {
			return nil, err
		}
		out := make([]string, 0, len(nsList.Items))
		for _, ns := range nsList.Items {
			out = append(out, ns.Name)
		}
		return out, nil
	case gatewayv1.NamespacesFromSame:
		return []string{gwNs}, nil
	case gatewayv1.NamespacesFromSelector:
		sel := listener.AllowedRoutes.Namespaces.Selector
		if sel == nil {
			return []string{gwNs}, nil
		}
		labelSel, err := metav1.LabelSelectorAsSelector(sel)
		if err != nil {
			return nil, err
		}
		nsList := &corev1.NamespaceList{}
		if err := cl.List(ctx, nsList, client.MatchingLabelsSelector{Selector: labelSel}); err != nil {
			return nil, err
		}
		out := make([]string, 0, len(nsList.Items))
		for _, ns := range nsList.Items {
			if labelSel.Matches(labels.Set(ns.Labels)) {
				out = append(out, ns.Name)
			}
		}
		return out, nil
	case gatewayv1.NamespacesFromNone:
		return nil, nil
	default:
		return []string{gwNs}, nil
	}
}

func listenerAllowsHTTPRouteKind(listener gatewayv1.Listener) bool {
	if listener.AllowedRoutes == nil || len(listener.AllowedRoutes.Kinds) == 0 {
		return true
	}
	for _, kind := range listener.AllowedRoutes.Kinds {
		groupMatches := kind.Group == nil || string(*kind.Group) == "" || string(*kind.Group) == gatewayv1.GroupVersion.Group
		if groupMatches && supportedRouteKinds[kind.Kind] {
			return true
		}
	}
	return false
}

func parentRefTargetsGateway(ref gatewayv1.ParentReference, gw *gatewayv1.Gateway, routeNamespace string) bool {
	if ref.Name != gatewayv1.ObjectName(gw.Name) {
		return false
	}
	gwNs := gw.Namespace
	if gwNs == "" {
		gwNs = "default"
	}
	if normalizeParentRefNamespace(ref, routeNamespace) != gwNs {
		return false
	}
	kind := normalizeParentRefKind(ref)
	if kind != "" && kind != "Gateway" {
		return false
	}
	group := normalizeParentRefGroup(ref)
	if group != "" && group != gatewayv1.GroupName {
		return false
	}
	return true
}

func parentRefAttachesToListener(ref gatewayv1.ParentReference, listener gatewayv1.Listener) bool {
	section := normalizeParentRefSectionName(ref)
	if section != "" && section != string(listener.Name) {
		return false
	}
	port := normalizeParentRefPort(ref)
	if port != 0 && port != int32(listener.Port) {
		return false
	}
	return true
}

func httpRouteAcceptedForParentRef(route *gatewayv1.HTTPRoute, parentRef gatewayv1.ParentReference) bool {
	for _, parent := range route.Status.Parents {
		if parent.ControllerName != PlatformGatewayControllerName {
			continue
		}
		if !parentRefMatches(parent.ParentRef, parentRef, route.Namespace) {
			continue
		}
		if meta.IsStatusConditionTrue(parent.Conditions, string(gatewayv1.RouteConditionAccepted)) {
			return true
		}
	}
	return false
}

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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// enqueueK8sGatewaysForReferenceGrant re-evaluates Gateways whose listeners reference a Secret in
// the grant's namespace from another namespace. Without it, a listener's ResolvedRefs condition
// keeps whatever the grants allowed when the Gateway was last reconciled.
func (r *K8sGatewayReconciler) enqueueK8sGatewaysForReferenceGrant(ctx context.Context, obj client.Object) []reconcile.Request {
	grant, ok := obj.(*gatewayv1beta1.ReferenceGrant)
	if !ok {
		return nil
	}
	gwList := &gatewayv1.GatewayList{}
	if err := r.List(ctx, gwList); err != nil {
		log.FromContext(ctx).Error(err, "failed to list Gateways for ReferenceGrant event",
			"referenceGrant", grant.Name, "namespace", grant.Namespace)
		return nil
	}
	var requests []reconcile.Request
	for i := range gwList.Items {
		gw := &gwList.Items[i]
		if !r.Config.ManagedGatewayClass(string(gw.Spec.GatewayClassName)) {
			continue
		}
		if !gatewayListenersReferenceSecretsIn(gw, grant.Namespace) {
			continue
		}
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name}})
	}
	return requests
}

// gatewayListenersReferenceSecretsIn reports whether a listener references a TLS Secret in
// another namespace, ns, which needs a ReferenceGrant there.
func gatewayListenersReferenceSecretsIn(gw *gatewayv1.Gateway, ns string) bool {
	if gw == nil || gw.Namespace == ns {
		return false
	}
	for _, l := range gw.Spec.Listeners {
		if l.TLS == nil {
			continue
		}
		for _, ref := range l.TLS.CertificateRefs {
			if ref.Group != nil && string(*ref.Group) != "" {
				continue
			}
			if ref.Kind != nil && string(*ref.Kind) != "Secret" {
				continue
			}
			if ref.Namespace != nil && string(*ref.Namespace) == ns {
				return true
			}
		}
	}
	return false
}

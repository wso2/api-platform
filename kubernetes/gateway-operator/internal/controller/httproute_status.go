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

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// routeStatusPlan collects the conditions a reconcile wants on each of our parentRefs, so the
// route's status is written once at the end.
type routeStatusPlan struct {
	namespace string
	parents   []gatewayv1.RouteParentStatus
}

func newRouteStatusPlan(routeNamespace string) *routeStatusPlan {
	return &routeStatusPlan{namespace: routeNamespace}
}

// set records a condition for a parentRef, replacing an earlier one of the same type.
func (p *routeStatusPlan) set(ref gatewayv1.ParentReference, cond metav1.Condition) {
	for i := range p.parents {
		if parentRefMatches(p.parents[i].ParentRef, ref, p.namespace) {
			meta.SetStatusCondition(&p.parents[i].Conditions, cond)
			return
		}
	}
	p.parents = append(p.parents, gatewayv1.RouteParentStatus{
		ParentRef:      ref,
		ControllerName: PlatformGatewayControllerName,
		Conditions:     []metav1.Condition{cond},
	})
}

// applyRouteStatus writes the plan onto the route's status. When keep is set, our own entries it
// rejects are removed. Entries of other controllers are never touched. status.parents is replaced
// as a whole by a merge patch, so the patch is guarded by resourceVersion and retried on conflict;
// otherwise another controller's entry written in between would be lost.
func (r *HTTPRouteReconciler) applyRouteStatus(ctx context.Context, route *gatewayv1.HTTPRoute, plan *routeStatusPlan, keep func(gatewayv1.ParentReference) bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &gatewayv1.HTTPRoute{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(route), latest); err != nil {
			return err
		}
		base := latest.DeepCopy()

		parents := make([]gatewayv1.RouteParentStatus, 0, len(latest.Status.Parents)+len(plan.parents))
		for _, ps := range latest.Status.Parents {
			if ps.ControllerName == PlatformGatewayControllerName && keep != nil && !keep(ps.ParentRef) {
				continue
			}
			parents = append(parents, ps)
		}
		for _, want := range plan.parents {
			idx := -1
			for i := range parents {
				if parents[i].ControllerName == PlatformGatewayControllerName &&
					parentRefMatches(parents[i].ParentRef, want.ParentRef, latest.Namespace) {
					idx = i
					break
				}
			}
			if idx < 0 {
				parents = append(parents, gatewayv1.RouteParentStatus{
					ParentRef:      want.ParentRef,
					ControllerName: PlatformGatewayControllerName,
				})
				idx = len(parents) - 1
			}
			for _, c := range want.Conditions {
				meta.SetStatusCondition(&parents[idx].Conditions, c)
			}
		}
		latest.Status.Parents = parents

		if equality.Semantic.DeepEqual(base.Status, latest.Status) {
			return nil
		}
		return r.Status().Patch(ctx, latest, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{}))
	})
}

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
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

// gatewayProgrammedAt returns when the registered gateway last became Programmed=True,
// read from its APIGateway (or Kubernetes Gateway API Gateway) status. ok is false when
// the gateway is not currently Programmed or cannot be read.
func gatewayProgrammedAt(ctx context.Context, c client.Client, gw *registry.GatewayInfo) (time.Time, bool) {
	if gw == nil {
		return time.Time{}, false
	}
	key := types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name}
	var conditions []metav1.Condition
	if gw.FromGatewayAPI {
		g := &gatewayv1.Gateway{}
		if err := c.Get(ctx, key, g); err != nil {
			return time.Time{}, false
		}
		conditions = g.Status.Conditions
	} else {
		g := &apiv1.APIGateway{}
		if err := c.Get(ctx, key, g); err != nil {
			return time.Time{}, false
		}
		conditions = g.Status.Conditions
	}
	cond := meta.FindStatusCondition(conditions, apiv1.GatewayConditionProgrammed)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		return time.Time{}, false
	}
	return cond.LastTransitionTime.Time, true
}

// gatewayReprogrammedSinceFailure reports whether a resource whose deployment ended in a
// terminal failure (Programmed=False, reason DeploymentFailed, at its current generation)
// should be deployed again because its target gateway became Programmed after that
// failure. This covers failures the resource itself cannot fix: the gateway rejecting
// the operator's credentials while a rotated admin password rolls out, the admin
// credential Secret being missing until it is created, or the gateway being redeployed.
//
// failedAt is the resource's status.lastUpdateTime, written with the terminal
// condition. A retry first resets the condition to Pending, which writes a newer
// lastUpdateTime, so the same gateway transition triggers at most one retry.
func gatewayReprogrammedSinceFailure(ctx context.Context, c client.Client, gw *registry.GatewayInfo, failedAt *metav1.Time) bool {
	if failedAt == nil {
		return false
	}
	programmedAt, ok := gatewayProgrammedAt(ctx, c, gw)
	return ok && programmedAt.After(failedAt.Time)
}

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

// gatewayObservation is the target gateway's Programmed state seen when a deployment
// attempt started. It is kept in the in-memory tracker entry of the attempt.
type gatewayObservation struct {
	// Recorded is false when no attempt recorded an observation, for example after an
	// operator restart cleared the tracker.
	Recorded bool
	// ProgrammedAt is the gateway's Programmed=True transition time, or zero when the
	// gateway was not Programmed.
	ProgrammedAt time.Time
}

// observeGateway records the target gateway's Programmed state at the start of a
// deployment attempt.
func observeGateway(ctx context.Context, c client.Client, gw *registry.GatewayInfo) gatewayObservation {
	programmedAt, ok := gatewayProgrammedAt(ctx, c, gw)
	if !ok {
		programmedAt = time.Time{}
	}
	return gatewayObservation{Recorded: true, ProgrammedAt: programmedAt}
}

// gatewayReprogrammedSinceFailure reports whether a resource whose deployment ended in a
// terminal failure (Programmed=False, reason DeploymentFailed, at its current generation)
// should be deployed again because its target gateway became Programmed after the
// attempt that failed. This covers failures the resource itself cannot fix: the gateway
// rejecting the operator's credentials while a rotated admin password rolls out, the
// admin credential Secret being missing until it is created, or the gateway being
// redeployed.
//
// observed is what the failed attempt saw when it started. The gateway counts as
// re-programmed when its current Programmed transition differs from that observation, so
// a gateway that recovers after the attempt but before the failure status is written
// still triggers a retry. The retried attempt records the new transition, so the same
// transition triggers at most one retry.
//
// Without an observation (the tracker was cleared, for example by an operator restart)
// the transition is compared with failedAt, the resource's status.lastUpdateTime written
// with the terminal condition.
func gatewayReprogrammedSinceFailure(ctx context.Context, c client.Client, gw *registry.GatewayInfo, observed gatewayObservation, failedAt *metav1.Time) bool {
	programmedAt, ok := gatewayProgrammedAt(ctx, c, gw)
	if !ok {
		return false
	}
	if observed.Recorded {
		return programmedAt.After(observed.ProgrammedAt)
	}
	return failedAt != nil && programmedAt.After(failedAt.Time)
}

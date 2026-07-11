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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// TestGatewayProgrammedConditionStale guards the early Programmed refresh: a fresh Gateway
// (CRD default condition, observedGeneration 0) or a spec bump must be detected as stale so
// the early status patch brings Programmed to the current generation before Helm runs (which
// can block up to 300s — or forever for a release that never becomes ready). A condition
// already at the current generation must NOT be considered stale, so steady-state
// Programmed=True Gateways are never flapped back to Unknown.
func TestGatewayProgrammedConditionStale(t *testing.T) {
	gw := func(generation int64, conds ...metav1.Condition) *gatewayv1.Gateway {
		g := &gatewayv1.Gateway{}
		g.Generation = generation
		g.Status.Conditions = conds
		return g
	}
	programmed := func(status metav1.ConditionStatus, observedGeneration int64) metav1.Condition {
		return metav1.Condition{
			Type:               string(gatewayv1.GatewayConditionProgrammed),
			Status:             status,
			Reason:             string(gatewayv1.GatewayReasonProgrammed),
			ObservedGeneration: observedGeneration,
		}
	}

	tests := []struct {
		name string
		gw   *gatewayv1.Gateway
		want bool
	}{
		{
			name: "no Programmed condition at all",
			gw:   gw(1),
			want: true,
		},
		{
			name: "CRD default condition with observedGeneration 0",
			gw:   gw(1, programmed(metav1.ConditionUnknown, 0)),
			want: true,
		},
		{
			name: "condition behind after spec update",
			gw:   gw(3, programmed(metav1.ConditionTrue, 2)),
			want: true,
		},
		{
			name: "steady-state Programmed=True at current generation is not stale",
			gw:   gw(2, programmed(metav1.ConditionTrue, 2)),
			want: false,
		},
		{
			name: "Programmed=False at current generation is not stale",
			gw:   gw(2, programmed(metav1.ConditionFalse, 2)),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gatewayProgrammedConditionStale(tt.gw); got != tt.want {
				t.Errorf("gatewayProgrammedConditionStale() = %v, want %v", got, tt.want)
			}
		})
	}
}

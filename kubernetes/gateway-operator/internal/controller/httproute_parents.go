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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// parentGatewayGroup is every parentRef of a route that points at one Gateway. Refs to the same
// Gateway with different sectionNames or ports stay together, since they share one deployment.
type parentGatewayGroup struct {
	key     client.ObjectKey
	targets []gatewayParentTarget
}

// groupParentTargetsByGateway groups parent targets by Gateway, in the order the Gateways first
// appear in spec.parentRefs.
func groupParentTargetsByGateway(targets []gatewayParentTarget) []parentGatewayGroup {
	index := make(map[client.ObjectKey]int, len(targets))
	out := make([]parentGatewayGroup, 0, len(targets))
	for _, t := range targets {
		i, ok := index[t.key]
		if !ok {
			i = len(out)
			index[t.key] = i
			out = append(out, parentGatewayGroup{key: t.key})
		}
		out[i].targets = append(out[i].targets, t)
	}
	return out
}

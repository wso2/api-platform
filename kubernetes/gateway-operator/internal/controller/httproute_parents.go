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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// distinctParentGatewayKeys returns unique Gateway object keys from parent targets.
func distinctParentGatewayKeys(targets []gatewayParentTarget) []client.ObjectKey {
	seen := make(map[client.ObjectKey]struct{}, len(targets))
	out := make([]client.ObjectKey, 0, len(targets))
	for _, t := range targets {
		if _, ok := seen[t.key]; ok {
			continue
		}
		seen[t.key] = struct{}{}
		out = append(out, t.key)
	}
	return out
}

// validateParentGatewayTargets ensures all parentRefs target at most one distinct Gateway.
func validateParentGatewayTargets(targets []gatewayParentTarget) error {
	keys := distinctParentGatewayKeys(targets)
	if len(keys) > 1 {
		return fmt.Errorf("HTTPRoute with multiple distinct Gateway parentRefs is not supported; use parentRefs on a single Gateway")
	}
	return nil
}

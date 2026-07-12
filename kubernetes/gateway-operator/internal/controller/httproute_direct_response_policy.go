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
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

// This file maps a gateway-terminated immediate response (a rule with no/failed backends)
// into the respond policy. Header matching, redirects, and direct responses are all realized
// as policies; the operator no longer emits an immediate response as an operation field.

const (
	respondPolicyName    = "respond"
	respondPolicyVersion = "v1"
)

// respondPolicyFromStatus builds a respond policy attachment that returns the given status
// code with an empty body. The respond policy short-circuits the request and returns the
// response directly, so the operation needs no backend routing.
func respondPolicyFromStatus(statusCode int) (apiv1.Policy, error) {
	return policyFromParams(respondPolicyName, respondPolicyVersion, map[string]interface{}{
		"statusCode": statusCode,
	})
}

// operationHasRespondPolicy reports whether the operation already carries a respond policy
// (a terminal action), used to keep such ops out of header-routing collapse.
func operationHasRespondPolicy(op apiv1.Operation) bool {
	for _, p := range op.Policies {
		if p.Name == respondPolicyName {
			return true
		}
	}
	return false
}

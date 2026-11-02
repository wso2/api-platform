/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package models

// PolicyDefinition represents the definition/schema of a policy
type PolicyDefinition struct {
	Name             string                  `json:"name" yaml:"name"`
	Version          string                  `json:"version" yaml:"version"`
	DisplayName      string                  `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Description      *string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Parameters       *map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	SystemParameters *map[string]interface{} `json:"systemParameters,omitempty" yaml:"systemParameters,omitempty"`
	ManagedBy        string                  `json:"managedBy" yaml:"managedBy,omitempty"`

	// ExecutionClass declares whether this policy may run at the UPSTREAM position, where it
	// executes once per retry attempt rather than once per client request.
	//
	// Absent means once-per-client-request, and that default is the point: a policy with side
	// effects -- charging quota, counting a rate limit, calling a third party -- would do them
	// N times for one client request if it ran per attempt. An author listing such a policy
	// per upstream would get silent multiple-counting, with nothing in the response to show
	// it. Failing closed on an undeclared policy means the mistake surfaces at deploy time as
	// a validation error naming the policy (FR-012f).
	ExecutionClass *string `json:"executionClass,omitempty" yaml:"executionClass,omitempty"`
}

// Execution classes. A policy declares one in its policy-definition.yaml.
const (
	// ExecutionClassOncePerClientRequest is the default when a policy declares nothing. Such
	// a policy runs in the downstream chain only, which is entered once per client request.
	ExecutionClassOncePerClientRequest = "once-per-client-request"

	// ExecutionClassPerAttempt marks a policy as safe to run again on every upstream attempt:
	// it shapes the request for the target this attempt is going to and carries no side
	// effect that repeating would duplicate.
	ExecutionClassPerAttempt = "per-attempt"
)

// RunsPerAttempt reports whether this policy may be placed in an upstream chain.
func (d PolicyDefinition) RunsPerAttempt() bool {
	return d.ExecutionClass != nil && *d.ExecutionClass == ExecutionClassPerAttempt
}

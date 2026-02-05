/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package registry

import (
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// PolicyChain is a container for a complete policy processing pipeline for a route
type PolicyChain struct {
	// Ordered list of policies to execute (all implement Policy interface)
	Policies []policy.Policy

	// Policy specifications (aligned with Policies)
	PolicySpecs []policy.PolicySpec

	// Computed flag: true if any policy requires request body access
	// Determines whether ext_proc uses SKIP or BUFFERED mode for request body
	RequiresRequestBody bool

	// Computed flag: true if any policy requires response body access
	// Determines whether ext_proc uses SKIP or BUFFERED mode for response body
	RequiresResponseBody bool

	// Computed flag: true if any policy has a CEL execution condition
	// When false, CEL evaluation is skipped entirely during execution
	HasExecutionConditions bool
}

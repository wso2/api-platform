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

package platformgateway

import (
	"fmt"
	"maps"
	"slices"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

const (
	customPoliciesComposeFile = "tests/framework/core/catalog/platformgateway/docker-compose.custom-policies.yaml"
	customPoliciesStagedName  = "custom-policies"
	customPoliciesSource      = "tests/framework/core/catalog/platformgateway/resources/custom-policies"
)

// withCustomPolicies returns a copy of def whose controller loads the integration-test policy
// definitions under resources/custom-policies, and a build manifest that lists all but
// it-builtin with a filePath, so the gateway reports those as custom policies. The runtime has
// no implementation for them.
func withCustomPolicies(def *components.Definition) (*components.Definition, error) {
	if def == nil || def.Compose == nil {
		return nil, fmt.Errorf("the gateway custom policies need a compose-backed definition")
	}
	if slices.Contains(def.Compose.ComposeOverrideFiles, customPoliciesComposeFile) {
		return nil, fmt.Errorf("%s already loads the gateway custom policies", def)
	}
	if _, exists := def.Endpoint(EndpointXDSControllerAdmin); exists {
		return nil, fmt.Errorf("%s runs two controllers; the gateway custom policies are loaded by one", def)
	}
	out := *def

	compose := *def.Compose
	compose.ComposeOverrideFiles = append(append([]string(nil), def.Compose.ComposeOverrideFiles...), customPoliciesComposeFile)
	compose.StagedFiles = maps.Clone(def.Compose.StagedFiles)
	if compose.StagedFiles == nil {
		compose.StagedFiles = map[string]string{}
	}
	compose.StagedFiles[customPoliciesStagedName] = customPoliciesSource
	out.Compose = &compose

	return &out, nil
}

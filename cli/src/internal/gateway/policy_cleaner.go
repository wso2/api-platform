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

package gateway

import (
	"encoding/json"
)

// CleanedPolicy represents a policy without filePath for PolicyHub
type CleanedPolicy struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	VersionResolution string `json:"versionResolution,omitempty"`
}

// CleanPolicyManifestForPolicyHub removes filePath and root-level fields,
// returning only the policies array suitable for PolicyHub API
func CleanPolicyManifestForPolicyHub(manifest *PolicyManifest) ([]byte, error) {
	cleanedPolicies := make([]CleanedPolicy, len(manifest.Policies))

	for i, policy := range manifest.Policies {
		cleanedPolicies[i] = CleanedPolicy{
			Name:              policy.Name,
			Version:           policy.Version,
			VersionResolution: policy.VersionResolution,
		}
	}

	return json.Marshal(cleanedPolicies)
}

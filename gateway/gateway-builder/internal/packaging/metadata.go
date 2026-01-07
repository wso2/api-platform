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

package packaging

import (
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

// GenerateDockerLabels creates Docker labels for the runtime image
func GenerateDockerLabels(metadata *types.PackagingMetadata) map[string]string {
	labels := make(map[string]string)

	// Standard OCI labels
	labels["org.opencontainers.image.created"] = metadata.BuildTimestamp.Format(time.RFC3339)
	labels["org.opencontainers.image.title"] = "API Platform Policy Engine"
	labels["org.opencontainers.image.description"] = "API Platform Policy Engine with compiled policies"
	labels["org.opencontainers.image.vendor"] = "WSO2"

	// Builder metadata
	labels["build.timestamp"] = metadata.BuildTimestamp.Format(time.RFC3339)

	// Policy list
	if len(metadata.Policies) > 0 {
		policyList := formatPolicyList(metadata.Policies)
		labels["build.policies"] = policyList
		labels["build.policy-count"] = fmt.Sprintf("%d", len(metadata.Policies))
	}

	// Add any custom labels
	for key, value := range metadata.Labels {
		labels[key] = value
	}

	return labels
}

// formatPolicyList creates a comma-separated list of policies for Docker labels
func formatPolicyList(policies []types.PolicyInfo) string {
	var parts []string
	for _, p := range policies {
		parts = append(parts, fmt.Sprintf("%s@%s", p.Name, p.Version))
	}
	return strings.Join(parts, ",")
}

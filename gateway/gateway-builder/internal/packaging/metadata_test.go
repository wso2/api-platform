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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

func TestGenerateDockerLabels_BasicLabels(t *testing.T) {
	timestamp := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	metadata := &types.PackagingMetadata{
		BuildTimestamp: timestamp,
		Policies:       []types.PolicyInfo{},
		Labels:         map[string]string{},
	}

	labels := GenerateDockerLabels(metadata)

	// Standard OCI labels
	assert.Equal(t, "2025-06-15T10:30:00Z", labels["org.opencontainers.image.created"])
	assert.Equal(t, "API Platform Policy Engine", labels["org.opencontainers.image.title"])
	assert.Equal(t, "API Platform Policy Engine with compiled policies", labels["org.opencontainers.image.description"])
	assert.Equal(t, "WSO2", labels["org.opencontainers.image.vendor"])

	// Build metadata
	assert.Equal(t, "2025-06-15T10:30:00Z", labels["build.timestamp"])

	// No policy labels when no policies
	_, hasPolicies := labels["build.policies"]
	assert.False(t, hasPolicies)
	_, hasCount := labels["build.policy-count"]
	assert.False(t, hasCount)
}

func TestGenerateDockerLabels_WithPolicies(t *testing.T) {
	timestamp := time.Date(2025, 1, 20, 8, 0, 0, 0, time.UTC)

	metadata := &types.PackagingMetadata{
		BuildTimestamp: timestamp,
		Policies: []types.PolicyInfo{
			{Name: "ratelimit", Version: "v1.0.0"},
			{Name: "jwt-auth", Version: "v0.1.0"},
		},
		Labels: map[string]string{},
	}

	labels := GenerateDockerLabels(metadata)

	assert.Equal(t, "ratelimit@v1.0.0,jwt-auth@v0.1.0", labels["build.policies"])
	assert.Equal(t, "2", labels["build.policy-count"])
}

func TestGenerateDockerLabels_SinglePolicy(t *testing.T) {
	metadata := &types.PackagingMetadata{
		BuildTimestamp: time.Now(),
		Policies: []types.PolicyInfo{
			{Name: "cors", Version: "v2.0.0"},
		},
		Labels: map[string]string{},
	}

	labels := GenerateDockerLabels(metadata)

	assert.Equal(t, "cors@v2.0.0", labels["build.policies"])
	assert.Equal(t, "1", labels["build.policy-count"])
}

func TestGenerateDockerLabels_WithCustomLabels(t *testing.T) {
	metadata := &types.PackagingMetadata{
		BuildTimestamp: time.Now(),
		Policies:       []types.PolicyInfo{},
		Labels: map[string]string{
			"custom.label":   "custom-value",
			"another.label":  "another-value",
			"build.override": "override-value",
		},
	}

	labels := GenerateDockerLabels(metadata)

	assert.Equal(t, "custom-value", labels["custom.label"])
	assert.Equal(t, "another-value", labels["another.label"])
	assert.Equal(t, "override-value", labels["build.override"])
}

func TestGenerateDockerLabels_ManyPolicies(t *testing.T) {
	metadata := &types.PackagingMetadata{
		BuildTimestamp: time.Now(),
		Policies: []types.PolicyInfo{
			{Name: "policy1", Version: "v1.0.0"},
			{Name: "policy2", Version: "v2.0.0"},
			{Name: "policy3", Version: "v3.0.0"},
			{Name: "policy4", Version: "v4.0.0"},
			{Name: "policy5", Version: "v5.0.0"},
		},
		Labels: map[string]string{},
	}

	labels := GenerateDockerLabels(metadata)

	assert.Equal(t, "5", labels["build.policy-count"])
	assert.Contains(t, labels["build.policies"], "policy1@v1.0.0")
	assert.Contains(t, labels["build.policies"], "policy5@v5.0.0")
}

func TestFormatPolicyList_Empty(t *testing.T) {
	policies := []types.PolicyInfo{}

	result := formatPolicyList(policies)

	assert.Equal(t, "", result)
}

func TestFormatPolicyList_SinglePolicy(t *testing.T) {
	policies := []types.PolicyInfo{
		{Name: "ratelimit", Version: "v1.0.0"},
	}

	result := formatPolicyList(policies)

	assert.Equal(t, "ratelimit@v1.0.0", result)
}

func TestFormatPolicyList_MultiplePolicies(t *testing.T) {
	policies := []types.PolicyInfo{
		{Name: "ratelimit", Version: "v1.0.0"},
		{Name: "jwt-auth", Version: "v0.1.0"},
		{Name: "cors", Version: "v2.0.0"},
	}

	result := formatPolicyList(policies)

	assert.Equal(t, "ratelimit@v1.0.0,jwt-auth@v0.1.0,cors@v2.0.0", result)
}

func TestFormatPolicyList_SpecialCharacters(t *testing.T) {
	policies := []types.PolicyInfo{
		{Name: "my-policy", Version: "v1.0.0-beta"},
		{Name: "another_policy", Version: "v2.0.0-rc.1"},
	}

	result := formatPolicyList(policies)

	assert.Equal(t, "my-policy@v1.0.0-beta,another_policy@v2.0.0-rc.1", result)
}

func TestGenerateDockerLabels_TimestampFormat(t *testing.T) {
	// Test various timestamps
	tests := []struct {
		name      string
		timestamp time.Time
		expected  string
	}{
		{
			name:      "UTC time",
			timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:  "2025-01-01T00:00:00Z",
		},
		{
			name:      "end of year",
			timestamp: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
			expected:  "2025-12-31T23:59:59Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &types.PackagingMetadata{
				BuildTimestamp: tt.timestamp,
				Policies:       []types.PolicyInfo{},
				Labels:         map[string]string{},
			}

			labels := GenerateDockerLabels(metadata)

			assert.Equal(t, tt.expected, labels["org.opencontainers.image.created"])
			assert.Equal(t, tt.expected, labels["build.timestamp"])
		})
	}
}

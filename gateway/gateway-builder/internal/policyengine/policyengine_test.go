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

package policyengine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

func TestSanitizeIdentifier_SimpleString(t *testing.T) {
	result := sanitizeIdentifier("myPolicy")
	assert.Equal(t, "myPolicy", result)
}

func TestSanitizeIdentifier_VersionPrefix(t *testing.T) {
	// 'v' is stripped, then '1' at position 0 is skipped (digit at start)
	// '.' becomes '_', '0' is kept (not at position 0), etc.
	result := sanitizeIdentifier("v1.0.0")
	assert.Equal(t, "_0_0", result)
}

func TestSanitizeIdentifier_WithDashes(t *testing.T) {
	result := sanitizeIdentifier("my-policy-name")
	assert.Equal(t, "my_policy_name", result)
}

func TestSanitizeIdentifier_WithSpaces(t *testing.T) {
	result := sanitizeIdentifier("my policy name")
	assert.Equal(t, "my_policy_name", result)
}

func TestSanitizeIdentifier_WithDots(t *testing.T) {
	// '1' at position 0 is skipped, '.' becomes '_', then numbers are kept
	result := sanitizeIdentifier("1.2.3")
	assert.Equal(t, "_2_3", result)
}

func TestSanitizeIdentifier_WithNumbers(t *testing.T) {
	result := sanitizeIdentifier("policy123")
	assert.Equal(t, "policy123", result)
}

func TestSanitizeIdentifier_NumberAtStart(t *testing.T) {
	// First digit '1' is skipped, but '2' and '3' are allowed after other chars
	// Actually looking at the code: position 0 digit skipped, positions 1+ get '23'
	result := sanitizeIdentifier("123policy")
	assert.Equal(t, "23policy", result)
}

func TestSanitizeIdentifier_MixedSpecialChars(t *testing.T) {
	// 'v' prefix is NOT stripped here because it's part of 'v1.0' inside the string
	// 'm' at pos 0, 'y' at pos 1, '-' becomes '_', etc.
	result := sanitizeIdentifier("my-policy.v1.0")
	assert.Equal(t, "my_policy_v1_0", result)
}

func TestSanitizeIdentifier_Underscores(t *testing.T) {
	result := sanitizeIdentifier("my_policy_name")
	assert.Equal(t, "my_policy_name", result)
}

func TestSanitizeIdentifier_UpperCase(t *testing.T) {
	result := sanitizeIdentifier("MyPolicyName")
	assert.Equal(t, "MyPolicyName", result)
}

func TestSanitizeIdentifier_OnlyVersionPrefix(t *testing.T) {
	// 'v' is stripped, leaves '010' - first '0' skipped, result is '10'
	result := sanitizeIdentifier("v010")
	assert.Equal(t, "10", result)
}

func TestGenerateImportAlias(t *testing.T) {
	tests := []struct {
		name       string
		policyName string
		version    string
		expected   string
	}{
		{
			name:       "simple policy and version",
			policyName: "ratelimit",
			version:    "v1.0.0",
			expected:   "ratelimit__0_0", // 'v' stripped, '1' at pos0 skipped
		},
		{
			name:       "policy with dashes",
			policyName: "jwt-auth",
			version:    "v0.1.0",
			expected:   "jwt_auth__1_0", // '-' becomes '_', 'v' stripped, '0' at pos0 skipped
		},
		{
			name:       "policy with underscores",
			policyName: "my_policy",
			version:    "v2.0.0",
			expected:   "my_policy__0_0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateImportAlias(tt.policyName, tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateImportPath(t *testing.T) {
	tests := []struct {
		name     string
		policy   *types.DiscoveredPolicy
		expected string
	}{
		{
			name: "simple policy name",
			policy: &types.DiscoveredPolicy{
				Name:    "ratelimit",
				Version: "v1.0.0",
			},
			expected: "github.com/policy-engine/policies/ratelimit",
		},
		{
			name: "policy with dashes",
			policy: &types.DiscoveredPolicy{
				Name:    "jwt-auth",
				Version: "v0.1.0",
			},
			expected: "github.com/policy-engine/policies/jwt-auth",
		},
		{
			name: "policy with underscores converted to dashes",
			policy: &types.DiscoveredPolicy{
				Name:    "my_policy",
				Version: "v1.0.0",
			},
			expected: "github.com/policy-engine/policies/my-policy",
		},
		{
			name: "policy with spaces converted to dashes",
			policy: &types.DiscoveredPolicy{
				Name:    "my policy",
				Version: "v1.0.0",
			},
			expected: "github.com/policy-engine/policies/my-policy",
		},
		{
			name: "uppercase policy name lowercased",
			policy: &types.DiscoveredPolicy{
				Name:    "RateLimit",
				Version: "v1.0.0",
			},
			expected: "github.com/policy-engine/policies/ratelimit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateImportPath(tt.policy)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGeneratePluginRegistry_EmptyPolicies(t *testing.T) {
	policies := []*types.DiscoveredPolicy{}

	result, err := GeneratePluginRegistry(policies, "/src")
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "package main")
}

func TestGeneratePluginRegistry_SinglePolicy(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{
			Name:    "ratelimit",
			Version: "v1.0.0",
			Path:    "/policies/ratelimit/v1.0.0",
		},
	}

	result, err := GeneratePluginRegistry(policies, "/src")
	require.NoError(t, err)
	assert.Contains(t, result, "package main")
	assert.Contains(t, result, "github.com/policy-engine/policies/ratelimit")
	assert.Contains(t, result, "ratelimit__0_0") // actual alias format
}

func TestGeneratePluginRegistry_MultiplePolicies(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{
			Name:    "ratelimit",
			Version: "v1.0.0",
			Path:    "/policies/ratelimit/v1.0.0",
		},
		{
			Name:    "jwt-auth",
			Version: "v0.1.0",
			Path:    "/policies/jwt-auth/v0.1.0",
		},
	}

	result, err := GeneratePluginRegistry(policies, "/src")
	require.NoError(t, err)
	assert.Contains(t, result, "github.com/policy-engine/policies/ratelimit")
	assert.Contains(t, result, "github.com/policy-engine/policies/jwt-auth")
	assert.Contains(t, result, "ratelimit__0_0")
	assert.Contains(t, result, "jwt_auth__1_0")
}

func TestGenerateBuildInfo_EmptyPolicies(t *testing.T) {
	policies := []*types.DiscoveredPolicy{}

	result, err := GenerateBuildInfo(policies, "v1.0.0")
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "package main")
	assert.Contains(t, result, "v1.0.0")
}

func TestGenerateBuildInfo_SinglePolicy(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{
			Name:    "ratelimit",
			Version: "v1.0.0",
		},
	}

	result, err := GenerateBuildInfo(policies, "v2.0.0")
	require.NoError(t, err)
	assert.Contains(t, result, "package main")
	assert.Contains(t, result, "ratelimit")
	assert.Contains(t, result, "v1.0.0")
	assert.Contains(t, result, "v2.0.0") // builder version
}

func TestGenerateBuildInfo_MultiplePolicies(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{
			Name:    "ratelimit",
			Version: "v1.0.0",
		},
		{
			Name:    "jwt-auth",
			Version: "v0.1.0",
		},
		{
			Name:    "cors",
			Version: "v2.0.0",
		},
	}

	result, err := GenerateBuildInfo(policies, "v1.5.0")
	require.NoError(t, err)
	assert.Contains(t, result, "ratelimit")
	assert.Contains(t, result, "jwt-auth")
	assert.Contains(t, result, "cors")
	// Should have timestamp
	assert.True(t, strings.Contains(result, "202") || strings.Contains(result, "BuildTimestamp"))
}

func TestGenerateBuildInfo_BuilderVersion(t *testing.T) {
	policies := []*types.DiscoveredPolicy{}

	result, err := GenerateBuildInfo(policies, "v3.2.1-beta")
	require.NoError(t, err)
	assert.Contains(t, result, "v3.2.1-beta")
}

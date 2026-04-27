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

package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// ---------------------------------------------------------------------------
// isExactVersion
// ---------------------------------------------------------------------------

func TestIsExactVersion(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		{"v1.0.0", true},
		{"v0.8.0", true},
		{"v1.2.3", true},
		{"v1", false},
		{"v1.0", false},
		{"V1.0.0", true}, // uppercase prefix
		{"1.0.0", true},  // no 'v' prefix
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			assert.Equal(t, tt.expected, isExactVersion(tt.version))
		})
	}
}

// ---------------------------------------------------------------------------
// compareVersions
// ---------------------------------------------------------------------------

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1, v2   string
		expected int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v2.0.0", "v1.0.0", 1},
		{"v1.0.0", "v2.0.0", -1},
		{"v1.2.0", "v1.1.9", 1},
		{"v0.8.1", "v0.8.0", 1},
		{"v0.0.1", "v0.0.2", -1},
	}

	for _, tt := range tests {
		t.Run(tt.v1+"_vs_"+tt.v2, func(t *testing.T) {
			assert.Equal(t, tt.expected, compareVersions(tt.v1, tt.v2))
		})
	}
}

// ---------------------------------------------------------------------------
// parseVersionPart
// ---------------------------------------------------------------------------

func TestParseVersionPart(t *testing.T) {
	tests := []struct {
		input     string
		expected  int
		expectOK  bool
	}{
		{"1", 1, true},
		{"123", 123, true},
		{"0", 0, true},
		{"1rc1", 1, true},  // numeric prefix only
		{"rc1", 0, false},  // no numeric prefix
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseVersionPart(tt.input)
			assert.Equal(t, tt.expectOK, ok)
			if tt.expectOK {
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// walkSchema
// ---------------------------------------------------------------------------

func TestWalkSchema_ExtractsResolveFields(t *testing.T) {
	pr := &PolicyResolver{resolveRules: make(map[string][]ResolveRule)}
	schema := map[string]interface{}{
		"resolve": []interface{}{"apiKey", "header"},
	}
	var rules []ResolveRule
	pr.walkSchema(schema, "params", &rules)

	require.Len(t, rules, 2)
	assert.Equal(t, "params.apiKey", rules[0].Path)
	assert.Equal(t, "params.header", rules[1].Path)
}

func TestWalkSchema_NestedProperties(t *testing.T) {
	pr := &PolicyResolver{resolveRules: make(map[string][]ResolveRule)}
	schema := map[string]interface{}{
		"properties": map[string]interface{}{
			"auth": map[string]interface{}{
				"resolve": []interface{}{"token"},
			},
		},
	}
	var rules []ResolveRule
	pr.walkSchema(schema, "params", &rules)

	require.Len(t, rules, 1)
	assert.Equal(t, "params.auth.token", rules[0].Path)
}

func TestWalkSchema_ArrayItems(t *testing.T) {
	pr := &PolicyResolver{resolveRules: make(map[string][]ResolveRule)}
	schema := map[string]interface{}{
		"items": map[string]interface{}{
			"resolve": []interface{}{"value"},
		},
	}
	var rules []ResolveRule
	pr.walkSchema(schema, "params.headers", &rules)

	require.Len(t, rules, 1)
	assert.Equal(t, "params.headers.*.value", rules[0].Path)
}

// ---------------------------------------------------------------------------
// NewPolicyResolver / buildResolveRules / GetResolveRules
// ---------------------------------------------------------------------------

func TestNewPolicyResolver_BuildsResolveRules(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"set-header|v1.0.0": {
			Name:    "set-header",
			Version: "v1.0.0",
			Parameters: &map[string]interface{}{
				"resolve": []interface{}{"apiKey"},
			},
		},
		"no-resolve|v1.0.0": {
			Name:    "no-resolve",
			Version: "v1.0.0",
			// No Parameters
		},
	}

	pr := NewPolicyResolver(defs)
	require.NotNil(t, pr)

	// set-header should have a resolve rule
	rules := pr.GetResolveRules(api.Policy{Name: "set-header", Version: "v1.0.0"})
	require.Len(t, rules, 1)
	assert.Equal(t, "params.apiKey", rules[0].Path)

	// no-resolve should have no rules
	rules = pr.GetResolveRules(api.Policy{Name: "no-resolve", Version: "v1.0.0"})
	assert.Empty(t, rules)
}

// ---------------------------------------------------------------------------
// GetResolveRulesForImplicitVersion
// ---------------------------------------------------------------------------

func TestGetResolveRulesForImplicitVersion_ExactMatch(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"auth|v1.0.0": {
			Name:    "auth",
			Version: "v1.0.0",
			Parameters: &map[string]interface{}{
				"resolve": []interface{}{"token"},
			},
		},
	}
	pr := NewPolicyResolver(defs)

	rules := pr.GetResolveRulesForImplicitVersion(api.Policy{Name: "auth", Version: "v1.0.0"})
	require.Len(t, rules, 1)
	assert.Equal(t, "params.token", rules[0].Path)
}

func TestGetResolveRulesForImplicitVersion_MajorOnlyResolution(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"auth|v1.0.0": {
			Name:    "auth",
			Version: "v1.0.0",
			Parameters: &map[string]interface{}{
				"resolve": []interface{}{"token"},
			},
		},
	}
	pr := NewPolicyResolver(defs)

	// v1 should match v1.0.0
	rules := pr.GetResolveRulesForImplicitVersion(api.Policy{Name: "auth", Version: "v1"})
	require.Len(t, rules, 1)
}

func TestGetResolveRulesForImplicitVersion_NoMatch(t *testing.T) {
	defs := map[string]models.PolicyDefinition{}
	pr := NewPolicyResolver(defs)

	rules := pr.GetResolveRulesForImplicitVersion(api.Policy{Name: "auth", Version: "v1"})
	assert.Empty(t, rules)
}

func TestGetResolveRulesForImplicitVersion_ExactVersionNoImplicitMatch(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"auth|v2.0.0": {
			Name:    "auth",
			Version: "v2.0.0",
			Parameters: &map[string]interface{}{
				"resolve": []interface{}{"token"},
			},
		},
	}
	pr := NewPolicyResolver(defs)

	// v1.0.0 is an exact version and should not match v2.0.0
	rules := pr.GetResolveRulesForImplicitVersion(api.Policy{Name: "auth", Version: "v1.0.0"})
	assert.Empty(t, rules)
}


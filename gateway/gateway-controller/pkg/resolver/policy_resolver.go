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
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// PolicyResolver resolves policy version matching for API configurations.
type PolicyResolver struct {
	policyDefinitions map[string]models.PolicyDefinition
	resolveRules      map[string][]ResolveRule // policyKey -> rules
}

type ResolveRule struct {
	Path string
}

// NewPolicyResolver creates a new policy resolver
func NewPolicyResolver(policyDefinitions map[string]models.PolicyDefinition) *PolicyResolver {
	resolver := &PolicyResolver{
		policyDefinitions: policyDefinitions,
		resolveRules:      make(map[string][]ResolveRule),
	}

	resolver.buildResolveRules()
	return resolver
}

// buildResolveRules extracts param paths needs to be resolved
func (pr *PolicyResolver) buildResolveRules() {
	for key, policyDef := range pr.policyDefinitions {
		if policyDef.Parameters == nil {
			continue
		}

		schema := *policyDef.Parameters
		var rules []ResolveRule

		pr.walkSchema(schema, "params", &rules)

		if len(rules) > 0 {
			pr.resolveRules[key] = rules
		}
	}
}

// walkSchema iterate through the policy definition and populates resolve rules
func (pr *PolicyResolver) walkSchema(schema map[string]interface{}, path string, rules *[]ResolveRule) {
	// Check resolve at this level
	if raw, ok := schema["resolve"]; ok {
		if fields, ok := raw.([]interface{}); ok {
			for _, f := range fields {
				if fieldName, ok := f.(string); ok {
					*rules = append(*rules, ResolveRule{
						Path: path + "." + fieldName,
					})
				}
			}
		}
	}

	// Walk properties
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for name, propSchema := range props {
			if propMap, ok := propSchema.(map[string]interface{}); ok {
				pr.walkSchema(propMap, path+"."+name, rules)
			}
		}
	}

	// Walk array items
	if items, ok := schema["items"].(map[string]interface{}); ok {
		pr.walkSchema(items, path+".*", rules)
	}
}

// GetResolveRules returns resolve rules per policy (exact match)
func (pr *PolicyResolver) GetResolveRules(policy api.Policy) []ResolveRule {
	key := policy.Name + "|" + policy.Version
	return pr.resolveRules[key]
}

// GetResolveRulesForImplicitVersion returns resolve rules for a policy with implicit version matching.
// The API policy version can be implicit (e.g., "v0", "v0.8") and will match resolve rules with
// exact versions that start with the implicit version (e.g., "v0.8.0").
// If multiple versions match, the latest (highest) version is returned.
// If the policy version is already a full version (e.g., "v0.8.0"), only exact matching is performed.
func (pr *PolicyResolver) GetResolveRulesForImplicitVersion(policy api.Policy) []ResolveRule {
	// First try exact match
	exactKey := policy.Name + "|" + policy.Version
	if rules, ok := pr.resolveRules[exactKey]; ok {
		return rules
	}

	// Only perform implicit version matching if the policy version is not a full/exact version
	// Full versions have at least 3 segments (e.g., v0.8.0)
	if isExactVersion(policy.Version) {
		return nil
	}

	// Try implicit version matching
	// API policy "v0" should match resolve rule "v0.8.0", "v0.8.1", etc. -> return latest
	// API policy "v0.8" should match resolve rule "v0.8.0", "v0.8.1" -> return latest
	implicitPrefix := policy.Name + "|" + policy.Version
	var bestKey string
	var bestRules []ResolveRule

	for key, rules := range pr.resolveRules {
		if strings.HasPrefix(key, implicitPrefix) {
			// Verify it's actually a version match (key should be "name|version...")
			// Ensure we don't have false positives like "set-headers|v0" matching "set-headers-v0|1.0.0"
			if !strings.HasPrefix(key, implicitPrefix+".") && key != implicitPrefix {
				continue
			}

			// Extract the full version from the key (after "name|")
			fullVersion := strings.TrimPrefix(key, policy.Name+"|")

			// If this is the first match or it's a higher version than current best
			if bestKey == "" || compareVersions(fullVersion, strings.TrimPrefix(bestKey, policy.Name+"|")) > 0 {
				bestKey = key
				bestRules = rules
			}
		}
	}

	return bestRules
}

// isExactVersion checks if a version string is a full/exact version (has 3+ segments like v0.8.0)
// Returns false for implicit versions like "v0", "v0.8"
func isExactVersion(version string) bool {
	// Remove 'v' or 'V' prefix if present
	v := strings.TrimPrefix(strings.TrimPrefix(version, "v"), "V")
	parts := strings.Split(v, ".")
	return len(parts) >= 3
}

// compareVersions compares two semantic version strings (without the 'v' prefix).
// Returns:
//
//	-1 if v1 < v2
//
//	 0 if v1 == v2
//	 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	// Remove 'v' or 'V' prefix if present
	v1 = strings.TrimPrefix(strings.TrimPrefix(v1, "v"), "V")
	v2 = strings.TrimPrefix(strings.TrimPrefix(v2, "v"), "V")

	// Split into parts
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Compare each part
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var num1, num2 int

		if i < len(parts1) {
			num1, _ = parseVersionPart(parts1[i])
		}
		if i < len(parts2) {
			num2, _ = parseVersionPart(parts2[i])
		}

		if num1 < num2 {
			return -1
		}
		if num1 > num2 {
			return 1
		}
	}

	return 0
}

// parseVersionPart parses a version part string into an integer.
// Returns 0 for non-numeric parts (like pre-release identifiers).
func parseVersionPart(part string) (int, bool) {
	// Handle cases like "1-rc1" - only take the numeric prefix
	numericPart := ""
	for _, ch := range part {
		if ch >= '0' && ch <= '9' {
			numericPart += string(ch)
		} else {
			break
		}
	}

	if numericPart == "" {
		return 0, false
	}

	result := 0
	for _, ch := range numericPart {
		result = result*10 + int(ch-'0')
	}
	return result, true
}


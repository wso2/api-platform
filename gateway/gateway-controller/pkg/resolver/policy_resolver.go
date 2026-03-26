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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
)

// secretTemplateRegex matches $secret{key} patterns in policy parameters.
// Compiled once at package initialization to avoid recompilation on each call.
var secretTemplateRegex = regexp.MustCompile(`\$secret\{([^}]+)\}`)

// PolicyResolver resolves resolve policy params
type PolicyResolver struct {
	policyDefinitions map[string]models.PolicyDefinition
	secretsService    *secrets.SecretService
	resolveRules      map[string][]ResolveRule // policyKey -> rules
}

type ResolveRule struct {
	Path string
}

// NewPolicyResolver creates a new policy resolver
func NewPolicyResolver(policyDefinitions map[string]models.PolicyDefinition,
	secretsService *secrets.SecretService) *PolicyResolver {
	resolver := &PolicyResolver{
		policyDefinitions: policyDefinitions,
		secretsService:    secretsService,
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

// ResolvePolicies resolve all resolve policy params in an API configuration
// Returns a new StoredConfig with resolved secrets without modifying the original
// If no policies need resolution, returns the original config
func (pr *PolicyResolver) ResolvePolicies(apiConfig *models.StoredConfig) (
	*models.StoredConfig, []config.ValidationError) {
	var errors []config.ValidationError

	if apiConfig.Configuration == nil {
		return apiConfig, nil
	}

	// Only support RESTAPI kind
	switch c := (apiConfig.Configuration).(type) {
	case api.RestAPI:
		apiData := c.Spec

		// First pass: check if any policies need resolution
		needsResolution := false

		// Check global policies
		if apiData.Policies != nil {
			for i := range *apiData.Policies {
				policy := (*apiData.Policies)[i]
				rules := pr.GetResolveRulesForImplicitVersion(policy)
				if len(rules) > 0 {
					needsResolution = true
					break
				}
			}
		}

		// Check operation-level policies if we haven't found any yet
		if !needsResolution && apiData.Operations != nil {
			for i := range apiData.Operations {
				operation := apiData.Operations[i]
				if operation.Policies != nil {
					for j := range *operation.Policies {
						policy := (*operation.Policies)[j]
						rules := pr.GetResolveRulesForImplicitVersion(policy)
						if len(rules) > 0 {
							needsResolution = true
							break
						}
					}
				}
				if needsResolution {
					break
				}
			}
		}

		// If no policies need resolution, return original config
		if !needsResolution {
			return apiConfig, nil
		}

		// Deep copy only when needed
		resolvedConfig, err := pr.deepCopyStoredConfig(apiConfig)
		if err != nil {
			errors = append(errors, config.ValidationError{
				Field:   "config",
				Message: fmt.Sprintf("Failed to create copy of config: %v", err),
			})
			return nil, errors
		}

		// Re-parse apiData from the copied config
		c, ok := resolvedConfig.Configuration.(api.RestAPI)
		if !ok {
			return nil, []config.ValidationError{{
				Field:   "config",
				Message: fmt.Sprintf("Unexpected configuration type after copy: %T", resolvedConfig.Configuration),
			}}
		}
		apiData = c.Spec

		// Process global policies
		if apiData.Policies != nil {
			for i := range *apiData.Policies {
				policy := &(*apiData.Policies)[i]
				if err := pr.resolvePolicyValues(policy); err != nil {
					errors = append(errors, config.ValidationError{
						Field:   fmt.Sprintf("spec.policies[%d]", i),
						Message: fmt.Sprintf("Failed to resolve policy %s: %v", policy.Name, err),
					})
				}
			}
		}

		// Process operation-level policies
		if apiData.Operations != nil {
			for i := range apiData.Operations {
				operation := &apiData.Operations[i]
				if operation.Policies != nil {
					for j := range *operation.Policies {
						policy := &(*operation.Policies)[j]
						fieldPath := fmt.Sprintf("spec.operations[%d].policies[%d]", i, j)
						if err := pr.resolvePolicyValues(policy); err != nil {
							errors = append(errors, config.ValidationError{
								Field: fieldPath,
								Message: fmt.Sprintf("Failed to resolve policy %s for %s %s: %v",
									policy.Name, operation.Method, operation.Path, err),
							})
						}
					}
				}
			}
		}

		if len(errors) > 0 {
			return nil, errors
		}

		return resolvedConfig, nil
	}
	return apiConfig, nil
}

// resolvePolicyValues checks if policy needs resolution and decrypts values
func (pr *PolicyResolver) resolvePolicyValues(policy *api.Policy) error {
	// Get resolve rules for this policy (with implicit version support)
	rules := pr.GetResolveRulesForImplicitVersion(*policy)

	if len(rules) == 0 {
		// No resolution needed for this policy
		return nil
	}

	if policy.Params == nil {
		return fmt.Errorf("policy %s has resolve rules but no params", policy.Name)
	}

	// Iterate through each resolve rule
	for _, rule := range rules {
		// Resolve (decrypt) values at this path
		if err := pr.resolveValuesByPath(*policy.Params, rule.Path); err != nil {
			return fmt.Errorf("failed to resolve values for path %s: %w", rule.Path, err)
		}
	}

	return nil
}

// resolveValuesByPath finds and decrypts values at the given path
// Handles templated secrets like "Bearer $secret{wso2-openai-apikey}" and "$secret{auth-type} $secret{key}"
func (pr *PolicyResolver) resolveValuesByPath(params map[string]interface{}, path string) error {
	// Split path into segments (e.g., ["params", "requestHeaders", "*", "value"])
	segments := strings.Split(path, ".")

	// Start from params, skip the first "params" segment
	if len(segments) == 0 || segments[0] != "params" {
		return fmt.Errorf("path must start with 'params'")
	}

	// Navigate to the parent of the target field
	targetField := segments[len(segments)-1]
	parentPath := segments[1 : len(segments)-1]

	// Get all parent objects that contain the target field
	parents, err := pr.navigateToParents(params, parentPath)
	if err != nil {
		return err
	}

	// Decrypt the target field in each parent
	for _, parent := range parents {
		if obj, ok := parent.(map[string]interface{}); ok {
			if templateValue, exists := obj[targetField]; exists {
				// Check if value is a string (templates are strings)
				if strValue, ok := templateValue.(string); ok {
					// Resolve all $secret{} templates in the string
					resolvedValue, err := pr.resolveSecretTemplates(strValue)
					if err != nil {
						return fmt.Errorf("failed to resolve secret templates: %w", err)
					}
					// Update the value in place
					obj[targetField] = resolvedValue
				}
			}
		}
	}

	return nil
}

// resolveSecretTemplates finds and replaces all $secret{key} templates with decrypted values
// Example: "Bearer $secret{wso2-openai-apikey}" -> "Bearer sk_xxx"
// Example: "$secret{auth-type} $secret{wso2-openai-apikey}" -> "Bearer sk_xxx"
func (pr *PolicyResolver) resolveSecretTemplates(templateStr string) (string, error) {
	if pr.secretsService == nil {
		return "", fmt.Errorf("secret service is not initialized properly")
	}

	// Find all matches first to check if there's any work to do
	matches := secretTemplateRegex.FindAllStringSubmatchIndex(templateStr, -1)
	if matches == nil {
		// No templates to resolve, return original
		return templateStr, nil
	}

	var result strings.Builder
	lastEnd := 0

	for _, match := range matches {
		// match[0], match[1] = full match start/end
		// match[2], match[3] = capture group start/end (the secret key)
		fullStart, fullEnd := match[0], match[1]
		keyStart, keyEnd := match[2], match[3]

		// Append text before this match
		result.WriteString(templateStr[lastEnd:fullStart])

		// Extract and resolve the secret key
		secretKey := templateStr[keyStart:keyEnd]
		decryptedSecret, err := pr.secretsService.Get(secretKey, "")
		if err != nil {
			// Return immediately on first error — preserves first failure and avoids extra work
			return "", fmt.Errorf("failed to decrypt secret '%s': %w", secretKey, err)
		}

		result.WriteString(decryptedSecret.Value)
		lastEnd = fullEnd
	}

	// Append remaining text after last match
	result.WriteString(templateStr[lastEnd:])

	return result.String(), nil
}

// navigateToParents navigates to all parent objects that contain the target field
func (pr *PolicyResolver) navigateToParents(params map[string]interface{}, path []string) ([]interface{}, error) {
	current := []interface{}{params}

	for _, segment := range path {
		var next []interface{}

		for _, item := range current {
			if segment == "*" {
				// Handle array wildcard
				if arr, ok := item.([]interface{}); ok {
					next = append(next, arr...)
				} else {
					return nil, fmt.Errorf("expected array at wildcard, got %T", item)
				}
			} else {
				// Handle object property
				if obj, ok := item.(map[string]interface{}); ok {
					if val, exists := obj[segment]; exists {
						next = append(next, val)
					}
				} else {
					return nil, fmt.Errorf("expected object at segment %s, got %T", segment, item)
				}
			}
		}

		current = next
	}

	return current, nil
}

// deepCopyStoredConfig creates a deep copy of StoredConfig
func (pr *PolicyResolver) deepCopyStoredConfig(original *models.StoredConfig) (*models.StoredConfig, error) {
	if original == nil {
		return nil, nil
	}

	copied := *original // shallow copy first

	switch conf := original.Configuration.(type) {

	case api.RestAPI:
		var newConf api.RestAPI
		data, err := json.Marshal(conf)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal REST API config: %w", err)
		}
		if err := json.Unmarshal(data, &newConf); err != nil {
			return nil, fmt.Errorf("failed to unmarshal REST API config: %w", err)
		}
		copied.Configuration = newConf

	default:
		return nil, fmt.Errorf("unsupported configuration type: %T", conf)
	}

	return &copied, nil
}

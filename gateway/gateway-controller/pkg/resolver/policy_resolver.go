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

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
)

// PolicyResolver resolves resolve policy params
type PolicyResolver struct {
	policyDefinitions map[string]api.PolicyDefinition
	secretsService    *secrets.SecretService
	resolveRules      map[string][]ResolveRule // policyKey -> rules
}

type ResolveRule struct {
	Path string
}

// NewPolicyResolver creates a new policy resolver
func NewPolicyResolver(policyDefinitions map[string]api.PolicyDefinition,
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

// GetResolveRules returns resolve rules per policy
func (pr *PolicyResolver) GetResolveRules(policy api.Policy) []ResolveRule {
	key := policy.Name + "|" + policy.Version
	return pr.resolveRules[key]
}

// ResolvePolicies resolve all resolve policy params in an API configuration
// Returns a new StoredConfig with resolved secrets without modifying the original
// If no policies need resolution, returns the original config
func (pr *PolicyResolver) ResolvePolicies(apiConfig *models.StoredConfig) (
	*models.StoredConfig, []config.ValidationError) {
	var errors []config.ValidationError

	// Only support RESTAPI kind
	if apiConfig.Configuration.Kind != api.RestApi {
		return apiConfig, nil
	}

	apiData, err := apiConfig.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		errors = append(errors, config.ValidationError{
			Field:   "spec",
			Message: fmt.Sprintf("Failed to parse API data for policy validation: %v", err),
		})
		return nil, errors
	}

	// First pass: check if any policies need resolution
	needsResolution := false

	// Check global policies
	if apiData.Policies != nil {
		for i := range *apiData.Policies {
			policy := (*apiData.Policies)[i]
			rules := pr.GetResolveRules(policy)
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
					rules := pr.GetResolveRules(policy)
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
	apiData, err = resolvedConfig.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		errors = append(errors, config.ValidationError{
			Field:   "spec",
			Message: fmt.Sprintf("Failed to parse copied API data: %v", err),
		})
		return nil, errors
	}

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

	err = resolvedConfig.Configuration.Spec.FromAPIConfigData(apiData)
	if err != nil {
		errors = append(errors, config.ValidationError{
			Field:   "spec",
			Message: fmt.Sprintf("Failed to rebuild API spec after policy resolution: %v", err),
		})
	}

	if len(errors) > 0 {
		return nil, errors
	}

	return resolvedConfig, nil
}

// resolvePolicyValues checks if policy needs resolution and decrypts values
func (pr *PolicyResolver) resolvePolicyValues(policy *api.Policy) error {
	// Get resolve rules for this policy
	rules := pr.GetResolveRules(*policy)

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

	// Pattern to match $secret{key}
	secretPattern := `\$secret\{([^}]+)\}`
	re := regexp.MustCompile(secretPattern)

	var resolveErr error
	resolved := re.ReplaceAllStringFunc(templateStr, func(match string) string {
		// Extract the secret key from $secret{key}
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			resolveErr = fmt.Errorf("invalid secret template format: %s", match)
			return match
		}

		secretKey := matches[1]

		// Decrypt the secret key
		decryptedSecret, err := pr.secretsService.Get(secretKey, "")
		if err != nil {
			resolveErr = fmt.Errorf("failed to decrypt secret '%s': %w", secretKey, err)
			return match
		}

		return decryptedSecret.Value
	})

	if resolveErr != nil {
		return "", resolveErr
	}

	return resolved, nil
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
	// Use JSON marshaling/unmarshaling for deep copy
	data, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var copied models.StoredConfig
	if err := json.Unmarshal(data, &copied); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &copied, nil
}

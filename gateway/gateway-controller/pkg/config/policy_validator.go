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

package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	versionutil "github.com/wso2/api-platform/common/version"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/xeipuuv/gojsonschema"
)

// PolicyValidator validates policies referenced in API configurations
type PolicyValidator struct {
	policyDefinitions map[string]models.PolicyDefinition
	latestVersions    map[string]string // policyName -> latest full semver, pre-computed at construction
}

// NewPolicyValidator creates a new policy validator
func NewPolicyValidator(policyDefinitions map[string]models.PolicyDefinition) *PolicyValidator {
	return &PolicyValidator{
		policyDefinitions: policyDefinitions,
		latestVersions:    BuildLatestVersionIndex(policyDefinitions),
	}
}

// BuildLatestVersionIndex scans policy definitions once and builds a map of
// policyName -> latest full semver. Used for O(1) empty-version resolution.
func BuildLatestVersionIndex(definitions map[string]models.PolicyDefinition) map[string]string {
	index := make(map[string]string)
	for _, def := range definitions {
		if !fullSemverPattern.MatchString(def.Version) {
			continue
		}
		existing, ok := index[def.Name]
		if !ok || versionutil.CompareSemver(def.Version, existing) > 0 {
			index[def.Name] = def.Version
		}
	}
	return index
}

// ValidateMCPProxyPolicies validates all policies in an MCP proxy configuration
func (pv *PolicyValidator) ValidateMCPProxyPolicies(mcpConfig *api.MCPProxyConfiguration) []ValidationError {
	var errors []ValidationError

	if mcpConfig.Spec.Policies == nil {
		return errors
	}

	for i, policy := range *mcpConfig.Spec.Policies {
		errs := pv.validatePolicy(policy, fmt.Sprintf("spec.policies[%d]", i))
		errors = append(errors, errs...)
	}

	return errors
}

// ValidateRestAPIPolicies validates all policies in a REST API configuration
func (pv *PolicyValidator) ValidateRestAPIPolicies(apiConfig *api.RestAPI) []ValidationError {
	var errors []ValidationError

	// Validate API-level policies
	if apiConfig.Spec.Policies != nil {
		for i, policy := range *apiConfig.Spec.Policies {
			errs := pv.validatePolicy(policy, fmt.Sprintf("spec.policies[%d]", i))
			errors = append(errors, errs...)
		}
	}

	// Validate operation-level policies
	for opIdx, operation := range apiConfig.Spec.Operations {
		if operation.Policies != nil {
			for pIdx, policy := range *operation.Policies {
				errs := pv.validatePolicy(policy, fmt.Sprintf("spec.operations[%d].policies[%d]", opIdx, pIdx))
				errors = append(errors, errs...)
			}
		}
	}

	return errors
}

// ValidateLLMProviderPolicies validates all policy references in an LLM provider configuration.
// Mirrors ValidateRestAPIPolicies: it checks the user-authored global, operation and (deprecated)
// policy references against the loaded policy definitions. Policies injected later by the
// LLM->RestAPI transform (e.g. upstream auth) are intentionally not validated here so the
// semantics match the REST API path, which only validates user-authored policies.
func (pv *PolicyValidator) ValidateLLMProviderPolicies(cfg *api.LLMProviderConfiguration) []ValidationError {
	return pv.validateLLMPolicyRefs(cfg.Spec.GlobalPolicies, cfg.Spec.OperationPolicies, cfg.Spec.Policies)
}

// ValidateLLMProxyPolicies validates all policy references in an LLM proxy configuration.
// See ValidateLLMProviderPolicies for the rationale on validating the source configuration.
func (pv *PolicyValidator) ValidateLLMProxyPolicies(cfg *api.LLMProxyConfiguration) []ValidationError {
	return pv.validateLLMPolicyRefs(cfg.Spec.GlobalPolicies, cfg.Spec.OperationPolicies, cfg.Spec.Policies)
}

// validateLLMPolicyRefs validates the three policy collections shared by LLM providers and
// proxies: api-level (global) policies, operation-level policies, and the deprecated policies
// list. An empty version resolves to the latest available version (handled by ResolvePolicyVersion).
func (pv *PolicyValidator) validateLLMPolicyRefs(globalPolicies *[]api.Policy, operationPolicies *[]api.OperationPolicy, legacyPolicies *[]api.LLMPolicy) []ValidationError {
	var errors []ValidationError

	// Global (api-level) policies carry params, so reuse validatePolicy to also validate them.
	if globalPolicies != nil {
		for i, policy := range *globalPolicies {
			errors = append(errors, pv.validatePolicy(policy, fmt.Sprintf("spec.globalPolicies[%d]", i))...)
		}
	}

	// Operation-level policies: validate name + version existence.
	if operationPolicies != nil {
		for i, policy := range *operationPolicies {
			_, errs := pv.validatePolicyRef(policy.Name, policy.Version, fmt.Sprintf("spec.operationPolicies[%d]", i))
			errors = append(errors, errs...)
		}
	}

	// Deprecated policies list (still honoured): validate name + version existence.
	if legacyPolicies != nil {
		for i, policy := range *legacyPolicies {
			_, errs := pv.validatePolicyRef(policy.Name, policy.Version, fmt.Sprintf("spec.policies[%d]", i))
			errors = append(errors, errs...)
		}
	}

	return errors
}

// validatePolicy validates a single policy reference (name + version existence) and, when the
// definition declares a parameter schema, the policy's params against that schema.
func (pv *PolicyValidator) validatePolicy(policy api.Policy, fieldPath string) []ValidationError {
	policyDef, errors := pv.validatePolicyRef(policy.Name, policy.Version, fieldPath)
	if len(errors) > 0 {
		return errors
	}

	// Coerce then validate policy parameters against the declared JSON schema.
	if policyDef.Parameters != nil {
		params := make(map[string]interface{})
		if policy.Params != nil {
			params = *policy.Params
			// Coerce template-rendered strings to their schema-declared types using the
			// already-resolved policyDef — avoids a second resolvePolicyVersion call.
			coerceParamsBySchema(params, *policyDef.Parameters)
		}
		schemaErrs := pv.validatePolicyParams(params, *policyDef.Parameters, fieldPath+".params")
		errors = append(errors, schemaErrs...)
	}

	return errors
}

// validatePolicyRef resolves and confirms a policy reference (name + version) exists in the
// loaded policy definitions, returning the resolved definition when found. Version resolution:
// - Accept full semantic versions as-is (vX.Y.Z)
// - Allow major-only versions (vX) and resolve them to a single matching full version
// - An empty version resolves to the latest available version for that policy name
func (pv *PolicyValidator) validatePolicyRef(name, version, fieldPath string) (*models.PolicyDefinition, []ValidationError) {
	resolvedVersion, err := pv.resolvePolicyVersion(name, version)
	if err != nil {
		return nil, []ValidationError{{
			Field:   fieldPath + ".version",
			Message: err.Error(),
		}}
	}

	// Check if policy definition exists
	key := name + "|" + resolvedVersion
	policyDef, exists := pv.policyDefinitions[key]
	if !exists {
		return nil, []ValidationError{{
			Field:   fieldPath + ".name",
			Message: fmt.Sprintf("Policy '%s' version '%s' not found in loaded policy definitions", name, resolvedVersion),
		}}
	}

	return &policyDef, nil
}

var (
	fullSemverPattern   = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	majorVersionPattern = regexp.MustCompile(`^v\d+$`)
)

// resolvePolicyVersion resolves a policy version string. Only major-only
// versions (e.g., v1) are accepted; they are resolved to the unique full
// version (vX.Y.Z) from the loaded definitions. Full semantic version
// (e.g., v1.0.0) is rejected.
func (pv *PolicyValidator) resolvePolicyVersion(name, version string) (string, error) {
	return ResolvePolicyVersion(pv.policyDefinitions, pv.latestVersions, name, version)
}

// ResolvePolicyVersion resolves a policy version using the given definitions map.
// Only major-only versions (e.g., v1) are accepted; they are resolved to the
// unique full version (vX.Y.Z) for that policy name. Full semantic version
// (e.g., v1.0.0) is rejected. Used by both the validator and the derivation path.
// latestVersions is an optional pre-computed index (policyName -> latest full semver)
// for O(1) empty-version resolution; pass nil to fall back to scanning definitions.
func ResolvePolicyVersion(definitions map[string]models.PolicyDefinition, latestVersions map[string]string, name, version string) (string, error) {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		// No version specified: resolve to the latest available full version for this policy.
		if latestVersions != nil {
			if latest, ok := latestVersions[name]; ok {
				return latest, nil
			}
			return "", fmt.Errorf("policy '%s' not found in loaded policy definitions", name)
		}
		// Fallback: scan definitions (no pre-computed index available).
		var latestFull string
		for _, def := range definitions {
			if def.Name != name || !fullSemverPattern.MatchString(def.Version) {
				continue
			}
			if latestFull == "" || versionutil.CompareSemver(def.Version, latestFull) > 0 {
				latestFull = def.Version
			}
		}
		if latestFull == "" {
			return "", fmt.Errorf("policy '%s' not found in loaded policy definitions", name)
		}
		return latestFull, nil
	}

	// Full semantic version (e.g., v1.0.0) – reject; only major-only is allowed
	if fullSemverPattern.MatchString(trimmed) {
		return "", fmt.Errorf("policy '%s' version must be major-only (e.g., v1); full semantic version (e.g., v1.0.0) is not allowed", name)
	}

	// Major-only version (e.g., v1) – resolve to a single matching full version
	if majorVersionPattern.MatchString(trimmed) {
		var matchingVersions []string
		majorPrefix := trimmed + "."

		for _, def := range definitions {
			if def.Name != name {
				continue
			}
			if fullSemverPattern.MatchString(def.Version) && strings.HasPrefix(def.Version, majorPrefix) {
				matchingVersions = append(matchingVersions, def.Version)
			}
		}

		if len(matchingVersions) == 0 {
			return "", fmt.Errorf("policy '%s' major version '%s' not found in loaded policy definitions", name, trimmed)
		}
		if len(matchingVersions) > 1 {
			return "", fmt.Errorf("multiple matching versions for policy '%s' major '%s'; cannot resolve uniquely", name, trimmed)
		}

		return matchingVersions[0], nil
	}

	// Unsupported version format
	return "", fmt.Errorf("invalid version format '%s' for policy '%s'; expected major-only version (e.g., v1)", version, name)
}

// CoerceRestAPIPolicies coerces policy param strings to their schema-declared types for
// a RestAPI config. Must be called after template rendering (e.g. in the event listener)
// so the in-memory store and xDS snapshot receive typed values, not rendered strings.
func (pv *PolicyValidator) CoerceRestAPIPolicies(config *api.RestAPI) {
	if config.Spec.Policies != nil {
		pv.coercePolicySlice(*config.Spec.Policies)
	}
	for i := range config.Spec.Operations {
		if config.Spec.Operations[i].Policies != nil {
			pv.coercePolicySlice(*config.Spec.Operations[i].Policies)
		}
	}
}

// CoerceMCPProxyPolicies coerces policy param strings to their schema-declared types for
// an MCPProxyConfiguration. Must be called after template rendering in the event listener.
func (pv *PolicyValidator) CoerceMCPProxyPolicies(config *api.MCPProxyConfiguration) {
	if config.Spec.Policies != nil {
		pv.coercePolicySlice(*config.Spec.Policies)
	}
}

// CoerceLLMPolicies coerces policy param strings to their schema-declared types for
// any LLM config (provider or proxy). Pass the three policy collections from the spec.
func (pv *PolicyValidator) CoerceLLMPolicies(globalPolicies *[]api.Policy, operationPolicies *[]api.OperationPolicy, legacyPolicies *[]api.LLMPolicy) {
	pv.coerceLLMPolicyRefs(globalPolicies, operationPolicies, legacyPolicies)
}

// coercePolicySlice calls coerceSinglePolicyParams for every policy in the slice.
func (pv *PolicyValidator) coercePolicySlice(policies []api.Policy) {
	for i := range policies {
		if policies[i].Params == nil {
			continue
		}
		pv.coerceSinglePolicyParams(policies[i].Name, policies[i].Version, *policies[i].Params)
	}
}

// coerceLLMPolicyRefs coerces the three policy collections shared by LLM providers and
// proxies: global (api-level), operation-level, and the deprecated legacy list.
func (pv *PolicyValidator) coerceLLMPolicyRefs(globalPolicies *[]api.Policy, operationPolicies *[]api.OperationPolicy, legacyPolicies *[]api.LLMPolicy) {
	if globalPolicies != nil {
		pv.coercePolicySlice(*globalPolicies)
	}
	if operationPolicies != nil {
		for i := range *operationPolicies {
			op := &(*operationPolicies)[i]
			for j := range op.Paths {
				pv.coerceSinglePolicyParams(op.Name, op.Version, op.Paths[j].Params)
			}
		}
	}
	if legacyPolicies != nil {
		for i := range *legacyPolicies {
			lp := &(*legacyPolicies)[i]
			for j := range lp.Paths {
				pv.coerceSinglePolicyParams(lp.Name, lp.Version, lp.Paths[j].Params)
			}
		}
	}
}

// coerceSinglePolicyParams is the shared core for all per-policy-collection coercers.
// It resolves the policy version, looks up the definition, and calls coerceParamsBySchema.
func (pv *PolicyValidator) coerceSinglePolicyParams(name, version string, params map[string]interface{}) {
	if len(params) == 0 {
		return
	}
	resolvedVersion, err := pv.resolvePolicyVersion(name, version)
	if err != nil {
		return
	}
	key := name + "|" + resolvedVersion
	policyDef, exists := pv.policyDefinitions[key]
	if !exists || policyDef.Parameters == nil {
		return
	}
	coerceParamsBySchema(params, *policyDef.Parameters)
}

// coerceParamsBySchema mutates params in-place, replacing string values with their
// parsed equivalents when the JSON-schema property for that key declares a numeric or
// boolean type. It walks the schema recursively through "properties" (nested objects)
// and "items" (array elements) so that params nested inside arrays of objects — for
// example, advanced-ratelimit's limit nested under quotas[].limits[] — are coerced too.
//
// Coercion table:
//   - "integer" or "number" → strconv.ParseFloat → float64 (gojsonschema accepts
//     float64 for both; it additionally verifies no fractional part for integer).
//   - "boolean"             → "true"/"false" literal → bool.
//   - "object"              → recurse into the nested property map.
//   - "array"               → iterate over elements, recursing for object items or
//     coercing each scalar item.
//   - anything else         → left unchanged.
func coerceParamsBySchema(params map[string]interface{}, schema map[string]interface{}) {
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return
	}
	for key, val := range params {
		propSchema, ok := properties[key].(map[string]interface{})
		if !ok {
			continue
		}
		expectedType, _ := propSchema["type"].(string)
		switch expectedType {
		case "object":
			if nested, ok := val.(map[string]interface{}); ok {
				coerceParamsBySchema(nested, propSchema)
			}
		case "array":
			items, hasItems := propSchema["items"].(map[string]interface{})
			arr, isSlice := val.([]interface{})
			if !hasItems || !isSlice {
				continue
			}
			itemType, _ := items["type"].(string)
			for i, item := range arr {
				if itemType == "object" {
					if itemMap, ok := item.(map[string]interface{}); ok {
						coerceParamsBySchema(itemMap, items)
					}
				} else {
					arr[i] = coerceScalarByType(item, itemType)
				}
			}
		default:
			params[key] = coerceScalarByType(val, expectedType)
		}
	}
}

// coerceScalarByType converts val to the type declared by expectedType when val is a
// string. Non-string values and unrecognised types are returned unchanged.
func coerceScalarByType(val interface{}, expectedType string) interface{} {
	s, isString := val.(string)
	if !isString {
		return val
	}
	switch expectedType {
	case "integer", "number":
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	case "boolean":
		// Accept only JSON-literal "true"/"false" — same semantics as json.Unmarshal,
		// but avoids a []byte allocation.
		if s == "true" {
			return true
		}
		if s == "false" {
			return false
		}
	}
	return val
}

// validatePolicyParams validates policy parameters against a JSON schema
func (pv *PolicyValidator) validatePolicyParams(params map[string]interface{}, schema map[string]interface{}, fieldPath string) []ValidationError {
	var errors []ValidationError

	// Create JSON schema loader
	schemaLoader := gojsonschema.NewGoLoader(schema)
	paramsLoader := gojsonschema.NewGoLoader(params)

	// Validate
	result, err := gojsonschema.Validate(schemaLoader, paramsLoader)
	if err != nil {
		errors = append(errors, ValidationError{
			Field:   fieldPath,
			Message: fmt.Sprintf("Failed to validate policy parameters: %v", err),
		})
		return errors
	}

	// Collect validation errors
	if !result.Valid() {
		for _, validationErr := range result.Errors() {
			// Extract field path from the error context
			fieldName := validationErr.Field()
			if fieldName == "(root)" {
				fieldName = fieldPath
			} else {
				// Remove the "(root)." prefix if present
				fieldName = strings.TrimPrefix(fieldName, "(root).")
				fieldName = fieldPath + "." + fieldName
			}

			errors = append(errors, ValidationError{
				Field:   fieldName,
				Message: validationErr.Description(),
			})
		}
	}

	return errors
}

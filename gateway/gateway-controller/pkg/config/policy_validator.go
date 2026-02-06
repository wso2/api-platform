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
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/xeipuuv/gojsonschema"
)

// PolicyValidator validates policies referenced in API configurations
type PolicyValidator struct {
	policyDefinitions map[string]api.PolicyDefinition
}

// NewPolicyValidator creates a new policy validator
func NewPolicyValidator(policyDefinitions map[string]api.PolicyDefinition) *PolicyValidator {
	return &PolicyValidator{
		policyDefinitions: policyDefinitions,
	}
}

// ValidatePolicies validates all policies in an API configuration
func (pv *PolicyValidator) ValidatePolicies(apiConfig *api.APIConfiguration) []ValidationError {
	var errors []ValidationError
	// TODO: Extend to other kinds if they support policies
	if apiConfig.Kind == api.RestApi {
		apiData, err := apiConfig.Spec.AsAPIConfigData()
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   "spec",
				Message: fmt.Sprintf("Failed to parse API data for policy validation: %v", err),
			})
			return errors
		}
		// Validate API-level policies
		if apiData.Policies != nil {
			for i, policy := range *apiData.Policies {
				errs := pv.validatePolicy(policy, fmt.Sprintf("spec.policies[%d]", i))
				errors = append(errors, errs...)
			}
		}

		// Validate operation-level policies
		for opIdx, operation := range apiData.Operations {
			if operation.Policies != nil {
				for pIdx, policy := range *operation.Policies {
					errs := pv.validatePolicy(policy, fmt.Sprintf("spec.operations[%d].policies[%d]", opIdx, pIdx))
					errors = append(errors, errs...)
				}
			}
		}
	}

	return errors
}

// validatePolicy validates a single policy reference
func (pv *PolicyValidator) validatePolicy(policy api.Policy, fieldPath string) []ValidationError {
	var errors []ValidationError

	// Resolve policy version:
	// - Accept full semantic versions as-is (vX.Y.Z)
	// - Allow major-only versions (vX) and resolve them to a single matching
	//   full version (vX.Y.Z) from the loaded policy definitions.
	resolvedVersion, err := pv.resolvePolicyVersion(policy.Name, policy.Version)
	if err != nil {
		errors = append(errors, ValidationError{
			Field:   fieldPath + ".version",
			Message: err.Error(),
		})
		return errors
	}

	// Check if policy definition exists
	key := policy.Name + "|" + resolvedVersion
	policyDef, exists := pv.policyDefinitions[key]
	if !exists {
		errors = append(errors, ValidationError{
			Field:   fieldPath + ".name",
			Message: fmt.Sprintf("Policy '%s' version '%s' not found in loaded policy definitions", policy.Name, resolvedVersion),
		})
		return errors
	}

	// Validate policy parameters against JSON schema if schema is defined
	if policyDef.Parameters != nil {
		// If params is nil, validate against an empty object to enforce required fields
		params := make(map[string]interface{})
		if policy.Params != nil {
			params = *policy.Params
		}
		schemaErrs := pv.validatePolicyParams(params, *policyDef.Parameters, fieldPath+".params")
		errors = append(errors, schemaErrs...)
	} else {

	}

	return errors
}

var (
	fullSemverPattern  = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	majorVersionPattern = regexp.MustCompile(`^v\d+$`)
)

// resolvePolicyVersion resolves a policy version string. Only major-only
// versions (e.g., v1) are accepted; they are resolved to the unique full
// version (vX.Y.Z) from the loaded definitions. Full semantic version
// (e.g., v1.0.0) is rejected.
func (pv *PolicyValidator) resolvePolicyVersion(name, version string) (string, error) {
	return ResolvePolicyVersion(pv.policyDefinitions, name, version)
}

// ResolvePolicyVersion resolves a policy version using the given definitions map.
// Only major-only versions (e.g., v1) are accepted; they are resolved to the
// unique full version (vX.Y.Z) for that policy name. Full semantic version
// (e.g., v1.0.0) is rejected. Used by both the validator and the derivation path.
func ResolvePolicyVersion(definitions map[string]api.PolicyDefinition, name, version string) (string, error) {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return "", fmt.Errorf("policy '%s' version is required", name)
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

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
	if apiConfig.Kind == "http/rest" {
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

	// Check if policy definition exists
	key := policy.Name + "|" + policy.Version
	policyDef, exists := pv.policyDefinitions[key]
	if !exists {
		errors = append(errors, ValidationError{
			Field:   fieldPath + ".name",
			Message: fmt.Sprintf("Policy '%s' version '%s' not found in loaded policy definitions", policy.Name, policy.Version),
		})
		return errors
	}

	// Validate policy parameters against JSON schema if schema is defined
	if policyDef.ParametersSchema != nil {
		// If params is nil, validate against an empty object to enforce required fields
		params := make(map[string]interface{})
		if policy.Params != nil {
			params = *policy.Params
		}
		schemaErrs := pv.validatePolicyParams(params, *policyDef.ParametersSchema, fieldPath+".params")
		errors = append(errors, schemaErrs...)
	}

	return errors
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

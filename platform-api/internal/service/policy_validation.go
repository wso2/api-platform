/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

// policyVersionPattern mirrors the `pattern: '^v\d+$'` constraint on Policy.version,
// LLMPolicy.version and OperationPolicy.version in openapi.yaml: only a major-only
// version (e.g. v0, v1) is accepted, since the Gateway Controller rejects anything else.
var policyVersionPattern = regexp.MustCompile(`^v\d+$`)

// isValidPolicyVersion accepts a major-only version (e.g. v1) and also an empty/blank
// version: the Gateway Controller's ResolvePolicyVersion treats an unspecified version as
// "use the latest available version" rather than an error, so platform-api must not reject
// it either. Only a non-empty value that fails the major-only pattern (e.g. v1.0.0, 1) is
// invalid.
func isValidPolicyVersion(version string) bool {
	if strings.TrimSpace(version) == "" {
		return true
	}
	return policyVersionPattern.MatchString(version)
}

// validatePolicyVersions checks the version of every Policy in the list.
func validatePolicyVersions(policies *[]api.Policy) error {
	if policies == nil {
		return nil
	}
	for _, p := range *policies {
		if !isValidPolicyVersion(p.Version) {
			return apperror.ValidationFailed.New(fmt.Sprintf(
				"Policy %q has an invalid version %q. Policy versions must be major-only, e.g. v1.", p.Name, p.Version))
		}
	}
	return nil
}

// validateLLMPolicyVersions checks the version of every deprecated flat LLMPolicy in the list.
func validateLLMPolicyVersions(policies *[]api.LLMPolicy) error {
	if policies == nil {
		return nil
	}
	for _, p := range *policies {
		if !isValidPolicyVersion(p.Version) {
			return apperror.ValidationFailed.New(fmt.Sprintf(
				"Policy %q has an invalid version %q. Policy versions must be major-only, e.g. v1.", p.Name, p.Version))
		}
	}
	return nil
}

// validateOperationPolicyVersions checks the version of every OperationPolicy in the list.
func validateOperationPolicyVersions(policies *[]api.OperationPolicy) error {
	if policies == nil {
		return nil
	}
	for _, p := range *policies {
		if !isValidPolicyVersion(p.Version) {
			return apperror.ValidationFailed.New(fmt.Sprintf(
				"Policy %q has an invalid version %q. Policy versions must be major-only, e.g. v1.", p.Name, p.Version))
		}
	}
	return nil
}

// validateOperationAndChannelPolicyVersions checks the policy versions attached to every
// REST API operation and channel.
func validateOperationAndChannelPolicyVersions(operations *[]api.Operation, channels *[]api.Channel) error {
	if operations != nil {
		for _, op := range *operations {
			if err := validatePolicyVersions(op.Request.Policies); err != nil {
				return err
			}
		}
	}
	if channels != nil {
		for _, ch := range *channels {
			if err := validatePolicyVersions(ch.Request.Policies); err != nil {
				return err
			}
		}
	}
	return nil
}

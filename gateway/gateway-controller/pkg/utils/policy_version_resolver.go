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

package utils

import (
	"errors"
	"fmt"

	versionutil "github.com/wso2/api-platform/common/version"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// PolicyDefinitionMissingUserMessage is returned to clients when a required policy definition is missing.
const PolicyDefinitionMissingUserMessage = "Required policy definition is missing in the gateway controller configuration. Please contact the administrator."

// PolicyDefinitionMissingError indicates a required policy definition was not loaded.
type PolicyDefinitionMissingError struct {
	PolicyName string
}

func (e *PolicyDefinitionMissingError) Error() string {
	return fmt.Sprintf("policy definition for '%s' not loaded", e.PolicyName)
}

// IsPolicyDefinitionMissingError checks whether err indicates a missing policy definition.
func IsPolicyDefinitionMissingError(err error) bool {
	var missingErr *PolicyDefinitionMissingError
	return errors.As(err, &missingErr)
}

// PolicyVersionResolver resolves a policy name to a version string.
type PolicyVersionResolver interface {
	Resolve(name string) (string, error)
}

// StaticPolicyVersionResolver resolves policy versions from a fixed map.
type StaticPolicyVersionResolver struct {
	versions map[string]string
}

// NewStaticPolicyVersionResolver creates a resolver from an explicit policy->version map.
func NewStaticPolicyVersionResolver(versions map[string]string) *StaticPolicyVersionResolver {
	clone := make(map[string]string, len(versions))
	for name, version := range versions {
		clone[name] = version
	}
	return &StaticPolicyVersionResolver{versions: clone}
}

// NewLoadedPolicyVersionResolver builds a resolver from loaded policy definitions.
// The highest semantic version per policy name is selected, then converted to
// a major-only version (e.g., v0) for compatibility with policy version
// validation rules.
func NewLoadedPolicyVersionResolver(policyDefinitions map[string]models.PolicyDefinition) *StaticPolicyVersionResolver {
	versions := make(map[string]string)
	for _, def := range policyDefinitions {
		existing, ok := versions[def.Name]
		if !ok || versionutil.CompareSemver(def.Version, existing) > 0 {
			versions[def.Name] = def.Version
		}
	}
	for name, version := range versions {
		versions[name] = versionutil.MajorVersion(version)
	}
	return NewStaticPolicyVersionResolver(versions)
}

// Resolve returns the version for a policy name or an error if missing.
func (r *StaticPolicyVersionResolver) Resolve(name string) (string, error) {
	if r == nil || r.versions == nil {
		return "", &PolicyDefinitionMissingError{PolicyName: name}
	}
	version, ok := r.versions[name]
	if !ok {
		return "", &PolicyDefinitionMissingError{PolicyName: name}
	}
	return version, nil
}

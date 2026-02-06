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
	"strconv"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
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
func NewLoadedPolicyVersionResolver(policyDefinitions map[string]api.PolicyDefinition) *StaticPolicyVersionResolver {
	versions := make(map[string]string)
	for _, def := range policyDefinitions {
		existing, ok := versions[def.Name]
		if !ok || compareSemver(def.Version, existing) > 0 {
			versions[def.Name] = def.Version
		}
	}
	for name, version := range versions {
		versions[name] = majorOnlyVersion(version)
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

func compareSemver(a, b string) int {
	aMajor, aMinor, aPatch, okA := parseSemver(a)
	bMajor, bMinor, bPatch, okB := parseSemver(b)
	if !okA || !okB {
		switch {
		case a > b:
			return 1
		case a < b:
			return -1
		default:
			return 0
		}
	}
	if aMajor != bMajor {
		return compareInts(aMajor, bMajor)
	}
	if aMinor != bMinor {
		return compareInts(aMinor, bMinor)
	}
	return compareInts(aPatch, bPatch)
}

func majorOnlyVersion(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return trimmed
	}

	raw := strings.TrimPrefix(trimmed, "v")
	parts := strings.Split(raw, ".")
	if len(parts) == 0 {
		return trimmed
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return trimmed
	}

	return fmt.Sprintf("v%d", major)
}

func parseSemver(v string) (int, int, int, bool) {
	raw := strings.TrimPrefix(v, "v")
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, false
	}
	return major, minor, patch, true
}

func compareInts(a, b int) int {
	switch {
	case a > b:
		return 1
	case a < b:
		return -1
	default:
		return 0
	}
}

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

package policy

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseManifest reads and parses a policy manifest file
func ParseManifest(manifestPath string) (*PolicyManifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest PolicyManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest file: %w", err)
	}

	// Validate manifest
	if err := validateManifest(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// ParseLockFile reads and parses a policy lock file
func ParseLockFile(lockPath string) (*PolicyLock, error) {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	var lock PolicyLock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("failed to parse lock file: %w", err)
	}

	return &lock, nil
}

// GenerateLockFile creates a policy lock file from processed policies
func GenerateLockFile(policies []ProcessedPolicy, outputPath string) error {
	lock := PolicyLock{
		Version:  "v1/alpha1",
		Policies: make([]LockPolicy, 0, len(policies)),
	}

	for _, p := range policies {
		source := "hub"
		if p.IsLocal {
			source = "local"
		}

		lockPolicy := LockPolicy{
			Name:     p.Name,
			Version:  p.Version,
			Checksum: p.Checksum,
			Source:   source,
		}

		// Include filePath for local policies
		if p.IsLocal && p.FilePath != "" {
			lockPolicy.FilePath = p.FilePath
		}

		lock.Policies = append(lock.Policies, lockPolicy)
	}

	data, err := yaml.Marshal(&lock)
	if err != nil {
		return fmt.Errorf("failed to marshal lock file: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	return nil
}

// validateManifest validates the manifest structure
func validateManifest(manifest *PolicyManifest) error {
	if manifest.Version == "" {
		return fmt.Errorf("manifest version is required")
	}

	if len(manifest.Policies) == 0 {
		return fmt.Errorf("manifest must contain at least one policy")
	}

	for i, policy := range manifest.Policies {
		if policy.Name == "" {
			return fmt.Errorf("policy at index %d: name is required", i)
		}

		// Policy must have either (version) or (filePath)
		hasVersion := policy.Version != ""
		hasFilePath := policy.FilePath != ""

		if !hasVersion && !hasFilePath {
			return fmt.Errorf("policy %s: must specify either version or filePath", policy.Name)
		}

		// Hub policies must have version
		if !hasFilePath && !hasVersion {
			return fmt.Errorf("policy %s: hub policies must specify version", policy.Name)
		}
	}

	return nil
}

// SeparatePolicies separates manifest policies into local and hub policies
func SeparatePolicies(manifest *PolicyManifest) (local, hub []ManifestPolicy) {
	for _, policy := range manifest.Policies {
		if policy.IsLocal() {
			local = append(local, policy)
		} else {
			hub = append(hub, policy)
		}
	}
	return
}

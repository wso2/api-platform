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
	"path/filepath"

	"github.com/wso2/api-platform/cli/utils"
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
func GenerateLockFile(policies []ProcessedPolicy, manifestVersion string, outputPath string) error {
	lock := PolicyLock{
		Version:  manifestVersion,
		Policies: make([]LockPolicy, 0, len(policies)),
	}

	for _, p := range policies {
		source := "hub"
		if p.IsLocal {
			source = "local"
		}

		lock.Policies = append(lock.Policies, LockPolicy{
			Name:     p.Name,
			Version:  p.Version,
			Checksum: p.Checksum,
			Source:   source,
		})
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

// GenerateLockFileWithPaths creates a workspace lock file with filePath entries
func GenerateLockFileWithPaths(policies []ProcessedPolicy, manifestVersion string, outputPath string, workspacePaths map[string]string) error {
	lock := PolicyLock{
		Version:  manifestVersion,
		Policies: make([]LockPolicy, 0, len(policies)),
	}

	for _, p := range policies {
		source := "hub"
		if p.IsLocal {
			source = "local"
		}

		// Get workspace path for this policy
		key := fmt.Sprintf("%s:%s", p.Name, p.Version)
		workspacePath := workspacePaths[key]

		lock.Policies = append(lock.Policies, LockPolicy{
			Name:     p.Name,
			Version:  p.Version,
			Checksum: p.Checksum,
			Source:   source,
			FilePath: workspacePath,
		})
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

		// Require version for all policies. Version is mandatory and not optional.
		if policy.Version == "" {
			return fmt.Errorf("policy[%d] (%s): 'version' field is required", i, policy.Name)
		}

		// Validate local policy zip file structure when a filePath is provided
		if policy.FilePath != "" {
			// Resolve relative paths relative to manifest directory
			policyPath := policy.FilePath
			if !filepath.IsAbs(policyPath) {
				// If relative, assume it's relative to working directory or manifest location
				policyPath = filepath.Clean(policyPath)
			}

			// Check if file exists
			if _, err := os.Stat(policyPath); os.IsNotExist(err) {
				return fmt.Errorf("policy %s: file not found at path: %s", policy.Name, policy.FilePath)
			}

			// Validate local policy zip structure
			if err := utils.ValidateLocalPolicyZip(policyPath); err != nil {
				return fmt.Errorf("policy %s: validation failed:\n%w\n\nLocal policies must:\n"+
					"  1. Be in zip format: <name>-<version>.zip (e.g., basic-auth-v1.0.0.zip)\n"+
					"  2. Contain a policy-definition.yaml at the root of the archive\n"+
					"  3. Ensure name and version fields exist inside policy-definition.yaml", policy.Name, err)
			}
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

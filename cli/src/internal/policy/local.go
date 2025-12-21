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
)

// ProcessLocalPolicies processes local policies from the manifest
func ProcessLocalPolicies(localPolicies []ManifestPolicy) ([]ProcessedPolicy, error) {
	if len(localPolicies) == 0 {
		return []ProcessedPolicy{}, nil
	}

	fmt.Printf("→ Processing %d local policies...\n", len(localPolicies))

	var processed []ProcessedPolicy

	for _, policy := range localPolicies {
		fmt.Printf("  %s %s: ", policy.Name, policy.Version)

		// Resolve the file path (can be relative or absolute)
		policyPath := policy.FilePath
		if !filepath.IsAbs(policyPath) {
			// Get current working directory
			cwd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to get current directory: %w", err)
			}
			policyPath = filepath.Join(cwd, policyPath)
		}

		// Check if path exists
		info, err := os.Stat(policyPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("local policy path not found: %s", policy.FilePath)
			}
			return nil, fmt.Errorf("failed to stat policy path %s: %w", policy.FilePath, err)
		}

		var zipPath string

		// Local policies must be provided as a zip file containing policy-definition.yaml at the root.
		if info.IsDir() {
			return nil, fmt.Errorf("local policy '%s' at path '%s' is a directory: local policies must be zip files containing a policy-definition.yaml at the root", policy.Name, policy.FilePath)
		}

		// It's a file: ensure it's a zip and validate structure
		zipPath = policyPath
		fmt.Printf("using zip, ")

		// Validate zip structure using central util so validation can be changed later
		_, _, err = utils.ValidateLocalPolicyZip(zipPath)
		if err != nil {
			return nil, fmt.Errorf("policy %s: validation failed: %w\n\nLocal policies must be a zip file containing a policy-definition.yaml at the root. The policy-definition.yaml must include 'name' and 'version' fields.", policy.Name, err)
		}

		// Calculate checksum
		checksum, err := utils.CalculateSHA256(zipPath)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate checksum for %s: %w", policy.Name, err)
		}

		fmt.Printf("checksum: %s\n", checksum[:20]+"...")

		processed = append(processed, ProcessedPolicy{
			Name:      policy.Name,
			Version:   policy.Version,
			Checksum:  checksum,
			Source:    "local",
			LocalPath: zipPath,
			IsLocal:   true,
			FilePath:  policy.FilePath,
		})
	}

	fmt.Printf("✓ Processed %d local policies\n\n", len(processed))
	return processed, nil
}

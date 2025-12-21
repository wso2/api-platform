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
		// Use a per-iteration closure so deferred cleanup runs at the end of
		// each iteration instead of at function exit (avoids accumulating
		// temporary zip files when processing many policies).
		err := func() error {
			fmt.Printf("  %s %s: ", policy.Name, policy.Version)

			// Resolve the file path (can be relative or absolute)
			policyPath := policy.FilePath
			if !filepath.IsAbs(policyPath) {
				// Get current working directory
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get current directory: %w", err)
				}
				policyPath = filepath.Join(cwd, policyPath)
			}

			// Check if path exists
			info, err := os.Stat(policyPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("local policy path not found: %s", policy.FilePath)
				}
				return fmt.Errorf("failed to stat policy path %s: %w", policy.FilePath, err)
			}

			var zipPath string
			var tempZipPath string

			// Local policies can be provided as a zip file or as a directory containing policy-definition.yaml.
			if info.IsDir() {
				// Create a temporary zip from directory for validation and checksum
				tempZip, err := os.CreateTemp("", "policy-zip-*.zip")
				if err != nil {
					return fmt.Errorf("failed to create temp zip for policy %s: %w", policy.Name, err)
				}
				tempZipPath = tempZip.Name()
				tempZip.Close()

				if err := utils.ZipDirectory(policyPath, tempZipPath); err != nil {
					_ = os.Remove(tempZipPath)
					return fmt.Errorf("failed to zip local policy directory %s: %w", policyPath, err)
				}

				// Ensure cleanup for this iteration only
				defer func() {
					_ = os.Remove(tempZipPath)
				}()

				zipPath = tempZipPath
				fmt.Printf("zipped directory to temp archive, ")
			} else {
				// It's a file: ensure it's a zip and validate structure
				zipPath = policyPath
				fmt.Printf("using zip, ")
			}

			// Validate zip structure using central util so validation can be changed later
			_, _, err = utils.ValidateLocalPolicyZip(zipPath)
			if err != nil {
				return fmt.Errorf("policy %s: validation failed: %w\n\nLocal policies must be a zip file containing a policy-definition.yaml at the root. The policy-definition.yaml must include 'name' and 'version' fields.", policy.Name, err)
			}

			// Calculate checksum
			checksum, err := utils.CalculateSHA256(zipPath)
			if err != nil {
				return fmt.Errorf("failed to calculate checksum for %s: %w", policy.Name, err)
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

			return nil
		}()

		if err != nil {
			return nil, err
		}
	}

	fmt.Printf("✓ Processed %d local policies\n\n", len(processed))
	return processed, nil
}

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

	fmt.Printf("  → Processing %d local policies...\n", len(localPolicies))

	var processed []ProcessedPolicy

	for _, policy := range localPolicies {
		// Use a per-iteration closure so deferred cleanup runs at the end of
		// each iteration instead of at function exit (avoids accumulating
		// temporary zip files when processing many policies).
		err := func() error {
			fmt.Printf("  %s: ", policy.Name)

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

			// Local policies must be directories containing policy-definition.yaml
			if !info.IsDir() {
				return fmt.Errorf("local policy '%s' must be a directory containing policy-definition.yaml; zip files are not supported", policy.FilePath)
			}

			policyDir := policyPath
			fmt.Printf("using dir, ")

			if err := utils.ValidateLocalPolicyDir(policyDir, policy.Name); err != nil {
				return fmt.Errorf("policy %s: validation failed: %w\n\nLocal policies must:\n  1. Be a directory containing policy-definition.yaml at the root\n  2. Have 'name' that matches the manifest", policy.Name, err)
			}

			processed = append(processed, ProcessedPolicy{
				Name:      policy.Name,
				Source:    "local",
				LocalPath: policyDir,
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

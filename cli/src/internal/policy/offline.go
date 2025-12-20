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

// VerifyOfflinePolicies verifies all policies from the lock file in offline mode
func VerifyOfflinePolicies(lockFile *PolicyLock, manifest *PolicyManifest) ([]ProcessedPolicy, error) {
	if len(lockFile.Policies) == 0 {
		return nil, fmt.Errorf("lock file contains no policies")
	}

	fmt.Printf("→ Verifying %d policies from lock file...\n", len(lockFile.Policies))

	// Create a map of local policy file paths from manifest
	localPolicyPaths := make(map[string]string)
	for _, manifestPolicy := range manifest.Policies {
		if manifestPolicy.IsLocal() {
			key := manifestPolicy.Name + ":" + manifestPolicy.Version
			localPolicyPaths[key] = manifestPolicy.FilePath
		}
	}

	var verified []ProcessedPolicy

	for _, lockPolicy := range lockFile.Policies {
		fmt.Printf("  %s %s [%s]: ", lockPolicy.Name, lockPolicy.Version, lockPolicy.Source)

		if lockPolicy.Source == "hub" {
			// Verify hub policy from cache
			processed, err := verifyHubPolicyOffline(lockPolicy)
			if err != nil {
				return nil, err
			}
			verified = append(verified, processed)
		} else if lockPolicy.Source == "local" {
			// Get filePath from manifest
			key := lockPolicy.Name + ":" + lockPolicy.Version
			filePath, exists := localPolicyPaths[key]
			if !exists {
				return nil, fmt.Errorf("\n  ✗ Local policy %s %s not found in manifest file", lockPolicy.Name, lockPolicy.Version)
			}

			// Verify local policy from manifest location
			processed, err := verifyLocalPolicyOffline(lockPolicy, filePath)
			if err != nil {
				return nil, err
			}
			verified = append(verified, processed)
		} else {
			return nil, fmt.Errorf("unknown policy source: %s", lockPolicy.Source)
		}
	}

	fmt.Printf("\n✓ Verified %d policies\n\n", len(verified))
	return verified, nil
}

// verifyHubPolicyOffline verifies a hub policy exists in cache with correct checksum
func verifyHubPolicyOffline(lockPolicy LockPolicy) (ProcessedPolicy, error) {
	cacheDir, err := utils.GetCacheDir()
	if err != nil {
		return ProcessedPolicy{}, fmt.Errorf("failed to get cache directory: %w", err)
	}

	policyFileName := utils.FormatPolicyFileName(lockPolicy.Name, lockPolicy.Version)
	cachePath := filepath.Join(cacheDir, policyFileName)

	// Check if policy exists in cache
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return ProcessedPolicy{}, fmt.Errorf("\n  ✗ Policy %s v%s not found in cache. Run without --offline first.", lockPolicy.Name, lockPolicy.Version)
	}

	// Verify checksum if enabled
	if utils.GatewayVerifyChecksumOnBuild {
		match, err := utils.VerifyChecksum(cachePath, lockPolicy.Checksum)
		if err != nil {
			return ProcessedPolicy{}, fmt.Errorf("failed to verify checksum: %w", err)
		}

		if !match {
			return ProcessedPolicy{}, fmt.Errorf("\n  ✗ Checksum mismatch for %s v%s. Cache may be corrupted. Run without --offline to refresh.", lockPolicy.Name, lockPolicy.Version)
		}

		fmt.Printf("found in cache, checksum verified\n")
	} else {
		fmt.Printf("found in cache (checksum verification disabled)\n")
	}

	return ProcessedPolicy{
		Name:      lockPolicy.Name,
		Version:   lockPolicy.Version,
		Checksum:  lockPolicy.Checksum,
		Source:    "hub",
		LocalPath: cachePath,
		IsLocal:   false,
	}, nil
}

// verifyLocalPolicyOffline verifies a local policy exists and has correct checksum
func verifyLocalPolicyOffline(lockPolicy LockPolicy, filePath string) (ProcessedPolicy, error) {
	policyPath := filePath

	// Check if the path exists
	if _, err := os.Stat(policyPath); os.IsNotExist(err) {
		return ProcessedPolicy{}, fmt.Errorf("\n  ✗ Local policy %s %s not found at path: %s", lockPolicy.Name, lockPolicy.Version, policyPath)
	}

	// Check if it's a zip file or directory
	info, err := os.Stat(policyPath)
	if err != nil {
		return ProcessedPolicy{}, fmt.Errorf("failed to stat policy path: %w", err)
	}

	var checksumPath string
	if info.IsDir() {
		// Need to zip it temporarily to calculate checksum
		tempDir, err := utils.GetTempDir()
		if err != nil {
			return ProcessedPolicy{}, fmt.Errorf("failed to get temp directory: %w", err)
		}

		zipFileName := utils.FormatPolicyFileName(lockPolicy.Name, lockPolicy.Version)
		checksumPath = filepath.Join(tempDir, zipFileName)

		if err := utils.ZipDirectory(policyPath, checksumPath); err != nil {
			return ProcessedPolicy{}, fmt.Errorf("failed to zip directory for checksum: %w", err)
		}
	} else {
		// It's already a zip file
		checksumPath = policyPath
	}

	// Verify checksum if enabled
	if utils.GatewayVerifyChecksumOnBuild {
		match, err := utils.VerifyChecksum(checksumPath, lockPolicy.Checksum)
		if err != nil {
			return ProcessedPolicy{}, fmt.Errorf("failed to verify checksum: %w", err)
		}

		if !match {
			return ProcessedPolicy{}, fmt.Errorf("\n  ✗ Checksum mismatch for local policy %s v%s. Policy may have been modified.", lockPolicy.Name, lockPolicy.Version)
		}

		fmt.Printf("found at %s, checksum verified\n", policyPath)
	} else {
		fmt.Printf("found at %s (checksum verification disabled)\n", policyPath)
	}

	return ProcessedPolicy{
		Name:      lockPolicy.Name,
		Version:   lockPolicy.Version,
		Checksum:  lockPolicy.Checksum,
		Source:    "local",
		LocalPath: policyPath,
		IsLocal:   true,
	}, nil
}

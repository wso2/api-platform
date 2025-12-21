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

package policyhub

import (
	"fmt"
	"os"

	"github.com/wso2/api-platform/cli/utils"
)

// RequestedPolicy represents a policy that was requested
type RequestedPolicy struct {
	Name    string
	Version string
}

// LoadedPolicyWithContents represents a loaded policy with its zip contents
type LoadedPolicyWithContents struct {
	PolicyData PolicyData
	Contents   map[string][]byte
}

// LoadPoliciesResult contains the result of loading policies
type LoadPoliciesResult struct {
	LoadedPolicies  []LoadedPolicyWithContents
	MissingPolicies []RequestedPolicy
}

// LoadPolicies downloads and caches policies from PolicyHub
func LoadPolicies(client *PolicyHubClient, cleanedPoliciesJSON []byte, requestedPolicies []RequestedPolicy) (*LoadPoliciesResult, error) {
	// Ensure policies directory exists
	policiesDir, err := EnsurePoliciesDir()
	if err != nil {
		return nil, err
	}

	fmt.Println()
	fmt.Println("=== Resolving Policies from PolicyHub ===")

	// Call PolicyHub to resolve policies
	response, err := client.ResolvePolicies(cleanedPoliciesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve policies: %w", err)
	}

	fmt.Printf("✓ PolicyHub resolved %d policies\n", len(response.Data))
	fmt.Println()

	// Check for missing policies
	missingPolicies := findMissingPolicies(requestedPolicies, response.Data)
	if len(missingPolicies) > 0 {
		fmt.Println("⚠ Warning: Some requested policies were not found:")
		for _, missing := range missingPolicies {
			fmt.Printf("  - %s v%s\n", missing.Name, missing.Version)
		}
		fmt.Println()
	}

	// Download and cache policies
	fmt.Println("=== Loading Policies ===")
	loadedPolicies := make([]LoadedPolicyWithContents, 0, len(response.Data))

	for _, policyData := range response.Data {
		zipPath := GetPolicyZipPath(policiesDir, policyData.PolicyName, policyData.Version)
		policyID := fmt.Sprintf("%s-%s", policyData.PolicyName, policyData.Version)

		// Check if policy already exists
		if utils.FileExists(zipPath) {
			fmt.Printf("%s: found locally, verifying...\n", policyID)

			// Verify checksum of cached file
			if err := utils.VerifyFileChecksum(zipPath, policyData.Checksum.Value); err != nil {
				fmt.Printf("%s: checksum verification failed, re-downloading...\n", policyID)
				if err := downloadAndVerify(client, &policyData, zipPath); err != nil {
					fmt.Printf("%s: ✗ failed to download: %v\n", policyID, err)
					continue
				}
				fmt.Printf("%s: ✓ downloaded and verified\n", policyID)
			} else {
				fmt.Printf("%s: ✓ verified\n", policyID)
			}
		} else {
			// Download the policy
			fmt.Printf("%s: downloading...\n", policyID)
			if err := downloadAndVerify(client, &policyData, zipPath); err != nil {
				fmt.Printf("%s: ✗ failed to download: %v\n", policyID, err)
				continue
			}
			fmt.Printf("%s: ✓ downloaded and verified\n", policyID)
		}

		// Load zip contents into memory
		fmt.Printf("%s: loading contents...\n", policyID)
		contents, err := ExtractZipToMemory(zipPath)
		if err != nil {
			fmt.Printf("%s: ✗ failed to load contents: %v\n", policyID, err)
			continue
		}
		fmt.Printf("%s: ✓ loaded %d files\n", policyID, len(contents))

		loadedPolicies = append(loadedPolicies, LoadedPolicyWithContents{
			PolicyData: policyData,
			Contents:   contents,
		})
	}

	fmt.Println()
	fmt.Printf("✓ All policies loaded (%d/%d successful)\n", len(loadedPolicies), len(response.Data))

	return &LoadPoliciesResult{
		LoadedPolicies:  loadedPolicies,
		MissingPolicies: missingPolicies,
	}, nil
}

// downloadAndVerify downloads a policy and verifies its checksum
func downloadAndVerify(client *PolicyHubClient, policyData *PolicyData, destPath string) error {
	if err := client.DownloadPolicy(policyData.DownloadURL, destPath); err != nil {
		return err
	}

	if err := utils.VerifyFileChecksum(destPath, policyData.Checksum.Value); err != nil {
		// Remove corrupted file
		os.Remove(destPath)
		return err
	}

	return nil
}

// findMissingPolicies identifies policies that were requested but not returned
func findMissingPolicies(requested []RequestedPolicy, received []PolicyData) []RequestedPolicy {
	// Create a map of received policies for quick lookup
	receivedMap := make(map[string]bool)
	for _, policy := range received {
		key := fmt.Sprintf("%s-%s", policy.PolicyName, policy.Version)
		receivedMap[key] = true
	}

	// Find missing policies
	var missing []RequestedPolicy
	for _, req := range requested {
		key := fmt.Sprintf("%s-%s", req.Name, req.Version)
		if !receivedMap[key] {
			missing = append(missing, req)
		}
	}

	return missing
}

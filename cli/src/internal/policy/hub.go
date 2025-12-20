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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wso2/api-platform/cli/utils"
)

// PolicyHubRequest represents a policy resolution request
type PolicyHubRequest struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	VersionResolution string `json:"versionResolution"`
}

// PolicyHubResponse represents the response from PolicyHub
type PolicyHubResponse struct {
	Data []PolicyHubData `json:"data"`
}

// PolicyHubData represents a resolved policy from PolicyHub
type PolicyHubData struct {
	PolicyName  string `json:"policy_name"`
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksum    struct {
		Algorithm string `json:"algorithm"`
		Value     string `json:"value"`
	} `json:"checksum"`
}

// ProcessHubPolicies resolves and downloads hub policies
func ProcessHubPolicies(hubPolicies []ManifestPolicy, rootVersionResolution string) ([]ProcessedPolicy, error) {
	if len(hubPolicies) == 0 {
		return []ProcessedPolicy{}, nil
	}

	fmt.Printf("→ Resolving %d hub policies...\n", len(hubPolicies))

	// Step 1: Resolve versions with PolicyHub
	resolvedPolicies, err := resolveWithPolicyHub(hubPolicies, rootVersionResolution)
	if err != nil {
		return nil, err
	}

	if len(resolvedPolicies) == 0 {
		// Build detailed error message with requested policies
		requestedPolicies := make([]string, 0, len(hubPolicies))
		for _, p := range hubPolicies {
			versionRes := p.GetVersionResolution(rootVersionResolution)
			requestedPolicies = append(requestedPolicies, fmt.Sprintf("  - %s %s (resolution: %s)", p.Name, p.Version, versionRes))
		}

		errMsg := fmt.Sprintf("no policies were resolved successfully from PolicyHub\n\nRequested policies:\n%s\n\nPossible reasons:\n  • Policy names may not exist in PolicyHub\n  • Versions may not be available\n  • Check policy names match exactly (case-sensitive)\n  • PolicyHub URL: %s",
			join(requestedPolicies, "\n"),
			utils.PolicyHubBaseURL)

		return nil, fmt.Errorf("%s", errMsg)
	}

	fmt.Printf("✓ Resolved %d policies from PolicyHub\n\n", len(resolvedPolicies))

	// Step 2: Download/verify policies
	processed, err := downloadAndVerifyPolicies(resolvedPolicies)
	if err != nil {
		return nil, err
	}

	return processed, nil
}

// resolveWithPolicyHub sends policies to PolicyHub for resolution
func resolveWithPolicyHub(hubPolicies []ManifestPolicy, rootVersionResolution string) ([]PolicyHubData, error) {
	// Build request
	requests := make([]PolicyHubRequest, 0, len(hubPolicies))
	seen := make(map[string]bool) // To deduplicate identical requests

	for _, policy := range hubPolicies {
		versionResolution := policy.GetVersionResolution(rootVersionResolution)

		// Create a unique key for deduplication
		key := fmt.Sprintf("%s:%s:%s", policy.Name, policy.Version, versionResolution)

		if !seen[key] {
			requests = append(requests, PolicyHubRequest{
				Name:              policy.Name,
				Version:           policy.Version,
				VersionResolution: versionResolution,
			})
			seen[key] = true
		}
	}

	fmt.Printf("  → Sending %d unique policies to PolicyHub\n", len(requests))

	// Send request to PolicyHub
	requestBody, err := json.Marshal(requests)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal PolicyHub request: %w", err)
	}

	url := utils.PolicyHubBaseURL + utils.PolicyHubResolvePath
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create PolicyHub request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact PolicyHub: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read PolicyHub response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Build detailed error with response
		requestedPolicies := make([]string, 0, len(requests))
		for _, req := range requests {
			requestedPolicies = append(requestedPolicies, fmt.Sprintf("  - %s %s (resolution: %s)", req.Name, req.Version, req.VersionResolution))
		}

		errMsg := fmt.Sprintf("PolicyHub returned status %d\n\nRequested policies:\n%s\n\nPolicyHub response:\n%s\n\nURL: %s",
			resp.StatusCode,
			join(requestedPolicies, "\n"),
			string(bodyBytes),
			url)

		return nil, fmt.Errorf("%s", errMsg)
	}

	// Parse response
	var hubResponse PolicyHubResponse
	if err := json.Unmarshal(bodyBytes, &hubResponse); err != nil {
		return nil, fmt.Errorf("failed to parse PolicyHub response: %w\n\nResponse body:\n%s", err, string(bodyBytes))
	}

	return hubResponse.Data, nil
}

// downloadAndVerifyPolicies downloads and verifies policies from PolicyHub using the new cache structure
func downloadAndVerifyPolicies(resolvedPolicies []PolicyHubData) ([]ProcessedPolicy, error) {
	fmt.Printf("→ Downloading and verifying %d policies...\n", len(resolvedPolicies))

	// Load policy index
	index, err := utils.LoadPolicyIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to load policy index: %w", err)
	}

	var processed []ProcessedPolicy

	for _, policy := range resolvedPolicies {
		// Normalize version to include "v" prefix
		version := policy.Version
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}

		// Validate version format
		if err := utils.ValidateVersionFormat(version); err != nil {
			return nil, fmt.Errorf("policy %s: %w", policy.PolicyName, err)
		}

		policyFileName := utils.FormatPolicyFileName(policy.PolicyName, version)
		expectedChecksum := fmt.Sprintf("sha256:%s", policy.Checksum.Value)

		fmt.Printf("  %s %s: ", policy.PolicyName, version)

		// Check if policy exists in index
		relativePath, inCache := utils.GetPolicyFromIndex(index, policy.PolicyName, version)

		var cachePath string
		if inCache {
			// Policy is in index, get full path
			fullPath, err := utils.GetPolicyCachePath(relativePath, policyFileName)
			if err != nil {
				return nil, fmt.Errorf("failed to get cache path for %s: %w", policy.PolicyName, err)
			}
			cachePath = fullPath

			// Verify the file actually exists
			if _, err := os.Stat(cachePath); err != nil {
				// File missing, treat as not cached
				fmt.Printf("cache entry exists but file missing, re-downloading...")
				inCache = false
			} else if utils.GatewayVerifyChecksumOnBuild {
				// Verify checksum
				match, err := utils.VerifyChecksum(cachePath, expectedChecksum)
				if err != nil {
					return nil, fmt.Errorf("failed to verify checksum for %s: %w", policy.PolicyName, err)
				}

				if match {
					fmt.Printf("found in cache, checksum verified\n")
				} else {
					fmt.Printf("cache checksum mismatch, re-downloading...")
					if err := downloadPolicy(policy.DownloadURL, cachePath); err != nil {
						return nil, fmt.Errorf("failed to download %s: %w", policy.PolicyName, err)
					}
					fmt.Printf(" done\n")
				}
			} else {
				fmt.Printf("found in cache (checksum verification disabled)\n")
			}
		}

		if !inCache {
			// Policy not in cache, need to cache it
			// Generate unique cache path
			relativePath = utils.GenerateUniqueCachePath(policy.PolicyName, version, index)
			fullPath, err := utils.GetPolicyCachePath(relativePath, policyFileName)
			if err != nil {
				return nil, fmt.Errorf("failed to generate cache path for %s: %w", policy.PolicyName, err)
			}
			cachePath = fullPath

			// Ensure directory exists
			if err := utils.EnsureDir(filepath.Dir(cachePath)); err != nil {
				return nil, fmt.Errorf("failed to create cache directory for %s: %w", policy.PolicyName, err)
			}

			// Download policy
			if !strings.Contains(fmt.Sprint(os.Stderr), "cache entry exists but file missing") {
				fmt.Printf("downloading...")
			}
			if err := downloadPolicy(policy.DownloadURL, cachePath); err != nil {
				return nil, fmt.Errorf("failed to download %s: %w", policy.PolicyName, err)
			}

			if utils.GatewayVerifyChecksumOnBuild {
				// Verify downloaded policy
				match, err := utils.VerifyChecksum(cachePath, expectedChecksum)
				if err != nil {
					return nil, fmt.Errorf("failed to verify downloaded policy %s: %w", policy.PolicyName, err)
				}
				if !match {
					return nil, fmt.Errorf("downloaded policy %s checksum mismatch", policy.PolicyName)
				}
				fmt.Printf(" done, verified\n")
			} else {
				fmt.Printf(" done (checksum verification disabled)\n")
			}

			// Add to index
			utils.AddPolicyToIndex(index, policy.PolicyName, version, relativePath)
		}

		processed = append(processed, ProcessedPolicy{
			Name:      policy.PolicyName,
			Version:   version,
			Checksum:  expectedChecksum,
			Source:    "hub",
			LocalPath: cachePath,
			IsLocal:   false,
		})
	}

	// Save updated index
	if err := utils.SavePolicyIndex(index); err != nil {
		return nil, fmt.Errorf("failed to save policy index: %w", err)
	}

	fmt.Printf("✓ Downloaded and verified %d policies\n\n", len(processed))
	return processed, nil
}

// downloadPolicy downloads a policy from a URL to a destination path
func downloadPolicy(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download policy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Copy data
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// join is a helper function to join strings with a separator
func join(items []string, sep string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += sep
		}
		result += item
	}
	return result
}

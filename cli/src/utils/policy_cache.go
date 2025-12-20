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
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"gopkg.in/yaml.v3"
)

// PolicyIndex represents the policy-index.yaml structure
type PolicyIndex struct {
	Policies map[string]map[string]string `yaml:"policies"` // PolicyName -> Version -> Path
	mu       sync.RWMutex                 `yaml:"-"`        // Mutex for thread-safe operations
}

const PolicyIndexFile = "policy-index.yaml"

// ToKebabCaseURLFriendly converts a string to kebab-case URL-friendly format
// Handles spaces, special characters, and maintains readability
func ToKebabCaseURLFriendly(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace spaces and underscores with hyphens
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Remove or replace special characters
	var result strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			result.WriteRune(r)
		}
	}

	// Remove multiple consecutive hyphens
	kebab := result.String()
	for strings.Contains(kebab, "--") {
		kebab = strings.ReplaceAll(kebab, "--", "-")
	}

	// Trim hyphens from start and end
	kebab = strings.Trim(kebab, "-")

	return kebab
}

// HashString generates a hash of the input string and returns first 8 characters
func HashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", hash)[:8]
}

// ValidateVersionFormat validates that version follows vX.X.X format
// Accepts: v1.0.0, v10.20.30, v1.2.3456, v1.0.0-beta, v1.0.0-rc.1
func ValidateVersionFormat(version string) error {
	// Regex pattern for semantic version with optional pre-release
	pattern := `^v\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?$`
	matched, err := regexp.MatchString(pattern, version)
	if err != nil {
		return fmt.Errorf("failed to validate version format: %w", err)
	}

	if !matched {
		return fmt.Errorf("invalid version format '%s': expected format vX.X.X (e.g., v1.0.0, v1.0.0-beta)", version)
	}

	return nil
}

// LoadPolicyIndex loads the policy index file, creates if doesn't exist
func LoadPolicyIndex() (*PolicyIndex, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	indexPath := filepath.Join(homeDir, PoliciesCachePath, PolicyIndexFile)

	// Check if file exists
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		// Create new index
		index := &PolicyIndex{
			Policies: make(map[string]map[string]string),
		}
		return index, nil
	}

	// Load existing index
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy index: %w", err)
	}

	var index PolicyIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse policy index: %w", err)
	}

	// Initialize map if nil
	if index.Policies == nil {
		index.Policies = make(map[string]map[string]string)
	}

	return &index, nil
}

// SavePolicyIndex saves the policy index file with write lock
func SavePolicyIndex(index *PolicyIndex) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Ensure cache directory exists
	cacheDir := filepath.Join(homeDir, PoliciesCachePath)
	if err := EnsureDir(cacheDir); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	indexPath := filepath.Join(cacheDir, PolicyIndexFile)

	// Lock for writing
	index.mu.Lock()
	defer index.mu.Unlock()

	// Marshal to YAML
	data, err := yaml.Marshal(index)
	if err != nil {
		return fmt.Errorf("failed to marshal policy index: %w", err)
	}

	// Write to file
	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write policy index: %w", err)
	}

	return nil
}

// GenerateUniqueCachePath generates a unique cache path for a policy, handling collisions
// Returns: relative path like "basic-auth/v1.0.0" or "basic-auth-a1b2c3d4/v1.0.0" if collision
func GenerateUniqueCachePath(policyName string, version string, index *PolicyIndex) string {
	baseKebab := ToKebabCaseURLFriendly(policyName)

	// Check if this exact path is already used by the same policy
	if versions, exists := index.Policies[policyName]; exists {
		if path, versionExists := versions[version]; versionExists {
			// Already cached with this path
			return path
		}
	}

	// Check for path collision with other policies
	basePath := filepath.Join(baseKebab, version)
	if !isPathCollidingWithOtherPolicy(basePath, policyName, index) {
		return basePath
	}

	// Collision detected, generate unique suffix
	attempt := policyName
	for i := 0; i < 100; i++ { // Reasonable limit to prevent infinite loop
		hash := HashString(attempt)
		candidateKebab := fmt.Sprintf("%s-%s", baseKebab, hash)
		candidatePath := filepath.Join(candidateKebab, version)

		if !isPathCollidingWithOtherPolicy(candidatePath, policyName, index) {
			return candidatePath
		}

		// Hash the hash for next iteration
		attempt = hash
	}

	// This should never happen, but just in case
	panic(fmt.Sprintf("failed to generate unique cache path for policy %s after 100 attempts", policyName))
}

// isPathCollidingWithOtherPolicy checks if a path is used by a different policy
func isPathCollidingWithOtherPolicy(path string, policyName string, index *PolicyIndex) bool {
	for name, versions := range index.Policies {
		if name == policyName {
			continue // Skip same policy
		}

		for _, existingPath := range versions {
			if existingPath == path {
				return true // Collision with different policy
			}
		}
	}
	return false
}

// AddPolicyToIndex adds or updates a policy in the index
func AddPolicyToIndex(index *PolicyIndex, policyName, version, cachePath string) {
	if index.Policies[policyName] == nil {
		index.Policies[policyName] = make(map[string]string)
	}
	index.Policies[policyName][version] = cachePath
}

// GetPolicyFromIndex retrieves a policy's cache path from the index
// Returns: (path, exists)
func GetPolicyFromIndex(index *PolicyIndex, policyName, version string) (string, bool) {
	if versions, exists := index.Policies[policyName]; exists {
		if path, versionExists := versions[version]; versionExists {
			return path, true
		}
	}
	return "", false
}

// GetPolicyCachePath returns the full absolute path to a cached policy zip
func GetPolicyCachePath(relativePath, zipFileName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(homeDir, PoliciesCachePath, relativePath, zipFileName), nil
}

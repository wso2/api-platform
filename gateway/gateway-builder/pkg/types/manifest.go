package types

import (
	"fmt"
	"strings"
)

// PolicyManifestLock represents the policy-manifest-lock.yaml file
type PolicyManifestLock struct {
	Version  string          `yaml:"version"`
	Policies []ManifestEntry `yaml:"policies"`
}

// ManifestEntry represents a single policy entry in the manifest lock
type ManifestEntry struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// DeriveFilePath derives the file path from policy name and version
// Converts policy name to kebab-case and appends version
// Example: "BasicAuth" with version "1.0.0" -> "basic-auth-v1.0.0"
func (e *ManifestEntry) DeriveFilePath() string {
	kebabName := toKebabCase(e.Name)
	return fmt.Sprintf("%s-v%s", kebabName, e.Version)
}

// toKebabCase converts a string to kebab-case
// Example: "BasicAuth" -> "basic-auth", "APIKey" -> "api-key", "HTTPSRedirect" -> "https-redirect"
func toKebabCase(s string) string {
	// Handle empty string
	if s == "" {
		return ""
	}

	runes := []rune(s)
	var result strings.Builder

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Insert hyphen before uppercase letter if:
		// 1. It's not the first character, AND
		// 2. Previous character is lowercase, OR
		// 3. Previous character is uppercase AND next character is lowercase (e.g., "HTTPSRedirect" -> "HTTPS-Redirect")
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := runes[i-1]
			// Insert hyphen if previous is lowercase
			if prev >= 'a' && prev <= 'z' {
				result.WriteRune('-')
			} else if prev >= 'A' && prev <= 'Z' && i+1 < len(runes) {
				// Insert hyphen if current is uppercase, previous is uppercase, and next is lowercase
				next := runes[i+1]
				if next >= 'a' && next <= 'z' {
					result.WriteRune('-')
				}
			}
		}

		result.WriteRune(r)
	}

	// Convert to lowercase
	kebab := strings.ToLower(result.String())

	return kebab
}

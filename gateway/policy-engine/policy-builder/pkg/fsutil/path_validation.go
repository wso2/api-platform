package fsutil

import (
	"fmt"
	"os"
)

// ValidatePathExists checks if a file or directory exists and is accessible.
// Returns nil if the path exists and is accessible.
// Returns an error with appropriate context if:
//   - The path does not exist (file/directory not found)
//   - The path exists but cannot be accessed (permission denied, I/O error, etc.)
//
// Parameters:
//   - path: The file or directory path to validate
//   - pathType: A description of what the path represents (e.g., "policy directory", "policy.yaml", "go.mod")
//
// Example:
//
//	if err := ValidatePathExists("/path/to/policy", "policy directory"); err != nil {
//	    return fmt.Errorf("validation failed: %w", err)
//	}
func ValidatePathExists(path string, pathType string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}

	if os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist: %s", pathType, path)
	}

	return fmt.Errorf("failed to access %s: %s: %w", pathType, path, err)
}

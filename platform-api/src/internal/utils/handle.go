/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package utils

import (
	"regexp"
	"strings"

	"platform-api/src/internal/constants"

	"github.com/google/uuid"
)

const (
	handleMinLength = 3
	handleMaxLength = 63
	maxRetries      = 5
	suffixLength    = 4
)

var (
	// validHandleRegex matches lowercase alphanumeric with hyphens (not at start/end, no consecutive)
	validHandleRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	// invalidCharsRegex matches any character that is not alphanumeric, hyphen, underscore, or space
	invalidCharsRegex = regexp.MustCompile(`[^a-z0-9\-_ ]`)
	// multipleHyphensRegex matches consecutive hyphens
	multipleHyphensRegex = regexp.MustCompile(`-+`)
)

// ValidateHandle validates a user-provided handle.
// Handle must be:
// - Lowercase only
// - Alphanumeric with hyphens allowed
// - No special characters
// - No consecutive hyphens
// - Cannot start or end with hyphen
// - Length between 3 and 63 characters
func ValidateHandle(handle string) error {
	if handle == "" {
		return constants.ErrHandleEmpty
	}
	if len(handle) < handleMinLength {
		return constants.ErrHandleTooShort
	}
	if len(handle) > handleMaxLength {
		return constants.ErrHandleTooLong
	}
	if !validHandleRegex.MatchString(handle) {
		return constants.ErrInvalidHandle
	}
	return nil
}

// GenerateHandle generates a URL-friendly handle from a given source string.
// If existsCheck is provided, it will verify the generated handle doesn't already exist.
// If the handle exists, it appends a random suffix and retries up to 5 times.
//
// Parameters:
//   - source: The string to generate handle from (e.g., API name)
//   - existsCheck: Optional function that returns true if handle already exists, nil if no check needed
//
// Returns:
//   - Generated handle string
//   - Error if source is empty or all retries exhausted
func GenerateHandle(source string, existsCheck func(string) bool) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", constants.ErrHandleSourceEmpty
	}

	// Generate base handle from source
	handle := sanitizeToHandle(source)

	// If no existence check needed, return the handle directly
	if existsCheck == nil {
		return handle, nil
	}

	// Check if handle exists and retry with suffix if needed
	if !existsCheck(handle) {
		return handle, nil
	}

	// Handle exists, try with random suffix
	for range maxRetries {
		suffix := generateRandomSuffix()
		candidateHandle := handle

		// Ensure we don't exceed max length when adding suffix
		maxBaseLength := handleMaxLength - suffixLength - 1 // -1 for the hyphen
		if len(candidateHandle) > maxBaseLength {
			candidateHandle = candidateHandle[:maxBaseLength]

			// Avoid creating consecutive hyphens after truncation.
			candidateHandle = strings.TrimRight(candidateHandle, "-")
		}

		candidateHandle = candidateHandle + "-" + suffix

		if !existsCheck(candidateHandle) {
			return candidateHandle, nil
		}
	}

	return "", constants.ErrHandleGenerationFailed
}

// sanitizeToHandle converts a string to a valid handle format
func sanitizeToHandle(s string) string {
	// Convert to lowercase
	handle := strings.ToLower(s)

	// Replace spaces and underscores with hyphens
	handle = strings.ReplaceAll(handle, " ", "-")
	handle = strings.ReplaceAll(handle, "_", "-")

	// Remove invalid characters
	handle = invalidCharsRegex.ReplaceAllString(handle, "")

	// Collapse multiple hyphens into single hyphen
	handle = multipleHyphensRegex.ReplaceAllString(handle, "-")

	// Trim leading and trailing hyphens
	handle = strings.Trim(handle, "-")

	// Enforce length limits
	if len(handle) > handleMaxLength {
		handle = handle[:handleMaxLength]
		// Trim trailing hyphen if truncation created one
		handle = strings.TrimRight(handle, "-")
	}

	// If handle is too short after sanitization, pad with random suffix
	if len(handle) < handleMinLength {
		if handle == "" {
			handle = generateRandomSuffix() + generateRandomSuffix()
		} else {
			handle = handle + "-" + generateRandomSuffix()
		}
	}

	return handle
}

// generateRandomSuffix generates a random 4-character alphanumeric suffix
func generateRandomSuffix() string {
	return uuid.New().String()[:suffixLength]
}

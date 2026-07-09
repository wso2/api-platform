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
	"fmt"
	"regexp"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/apperror"

	"github.com/google/uuid"
)

const (
	handleMinLength = 3
	handleMaxLength = 40
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

// ValidateHandleImmutable checks a PUT request body's optional handle/id field against
// the resource's path handle. A nil or empty body value is treated as "not provided"
// and allowed; only a non-empty value that differs from the path handle is rejected.
func ValidateHandleImmutable(pathHandle string, bodyHandle *string) error {
	if bodyHandle == nil || *bodyHandle == "" {
		return nil
	}
	return ValidateHandleImmutableRequired(pathHandle, *bodyHandle)
}

// ValidateHandleImmutableRequired is like ValidateHandleImmutable but requires the
// request body to explicitly include a handle/id value matching the path handle.
func ValidateHandleImmutableRequired(pathHandle, bodyHandle string) error {
	if bodyHandle == "" || bodyHandle != pathHandle {
		return apperror.ValidationFailed.New("The id is immutable and cannot be changed.")
	}
	return nil
}

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
		return apperror.ValidationFailed.New("The id cannot be empty.")
	}
	if len(handle) < handleMinLength {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The id must be at least %d characters.", handleMinLength))
	}
	if len(handle) > handleMaxLength {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The id must be at most %d characters.", handleMaxLength))
	}
	if !validHandleRegex.MatchString(handle) {
		return apperror.ValidationFailed.New("The id must be lowercase alphanumeric with hyphens only " +
			"(no consecutive hyphens, cannot start or end with a hyphen).")
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
		return "", apperror.Internal.New().
			WithLogMessage("source string cannot be empty for handle generation")
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

	return "", apperror.Internal.New().
		WithLogMessage(fmt.Sprintf("failed to generate a unique handle for %q after %d retries", source, maxRetries))
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

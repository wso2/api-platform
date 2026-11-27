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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

var (
	// validAPIKeyNameRegex matches lowercase alphanumeric with hyphens and underscores (not at start/end, no consecutive separators)
	validAPIKeyNameRegex = regexp.MustCompile(`^[a-z0-9]+([_-][a-z0-9]+)*$`)
	// invalidCharsRegex matches any character that is not alphanumeric, hyphen, underscore, or space
	invalidCharsRegex = regexp.MustCompile(`[^a-z0-9\-_ ]`)
	// multipleHyphensRegex matches consecutive hyphens
	multipleHyphensRegex = regexp.MustCompile(`-+`)
)

// ValidateAPIKeyValue validates a plain API key value for creation or update.
// It applies to plain-text keys supplied through the REST API. Keys from platform API events
// (apikey.created, apikey.updated) arrive only as hashes, so platform-api checks their length
// before hashing.
// Min and max length are read from the service's APIKeyConfig; defaults are used if not configured.
// Returns a descriptive error wrapping ErrInvalidAPIKeyValue if the key is empty, too short, or too long.
// Note: Expects the caller to trim whitespace before validation.
func (s *APIKeyService) ValidateAPIKeyValue(plainKey string) error {
	minLength := constants.DefaultMinAPIKeyLength
	maxLength := constants.DefaultMaxAPIKeyLength
	if s.apiKeyConfig != nil {
		if s.apiKeyConfig.MinKeyLength > 0 {
			minLength = s.apiKeyConfig.MinKeyLength
		}
		if s.apiKeyConfig.MaxKeyLength > 0 {
			maxLength = s.apiKeyConfig.MaxKeyLength
		}
	}
	if plainKey == "" {
		return fmt.Errorf("%w: API key cannot be empty", ErrInvalidAPIKeyValue)
	}
	if len(plainKey) < minLength {
		return fmt.Errorf("%w: API key is too short (minimum %d characters required)", ErrInvalidAPIKeyValue, minLength)
	}
	if len(plainKey) > maxLength {
		return fmt.Errorf("%w: API key is too long (maximum %d characters allowed)", ErrInvalidAPIKeyValue, maxLength)
	}
	return nil
}

// ValidateDisplayName validates the user-provided display name for an API key.
// Display name must be 1-100 UTF-8 characters (counted by runes, not bytes).
// Trims whitespace before validation.
func ValidateDisplayName(displayName string) error {
	trimmed := strings.TrimSpace(displayName)
	if trimmed == "" {
		return fmt.Errorf("display name cannot be empty")
	}

	runeCount := utf8.RuneCountInString(trimmed)
	if runeCount > constants.DisplayNameMaxLength {
		return fmt.Errorf("display name is too long (%d characters, maximum %d allowed)", runeCount, constants.DisplayNameMaxLength)
	}
	return nil
}

// ValidateAPIKeyName validates a user-provided API key name.
// Name must be:
// - Lowercase only
// - Alphanumeric with hyphens and underscores allowed
// - No special characters
// - No consecutive separators (hyphens/underscores)
// - Cannot start or end with hyphen or underscore
// - Length between 3 and 63 characters
func ValidateAPIKeyName(name string) error {
	if name == "" {
		return fmt.Errorf("API key name cannot be empty")
	}
	if len(name) < constants.APIKeyNameMinLength {
		return fmt.Errorf("API key name is too short (minimum %d characters required)", constants.APIKeyNameMinLength)
	}
	if len(name) > constants.APIKeyNameMaxLength {
		return fmt.Errorf("API key name is too long (maximum %d characters allowed)", constants.APIKeyNameMaxLength)
	}
	if !validAPIKeyNameRegex.MatchString(name) {
		return fmt.Errorf("API key name must be lowercase alphanumeric with hyphens or underscores (no consecutive separators, cannot start/end with hyphen or underscore)")
	}
	return nil
}

// GenerateAPIKeyName generates a URL-safe name from a display name.
// Transforms the displayName by:
// - Trimming whitespace
// - Converting to lowercase
// - Replacing spaces and underscores with hyphens
// - Removing invalid characters
// - Collapsing consecutive hyphens
// - Trimming leading/trailing hyphens
// - Enforcing length constraints (3-63 chars)
func GenerateAPIKeyName(displayName string) (string, error) {
	trimmed := strings.TrimSpace(displayName)
	// Convert to lowercase
	name := strings.ToLower(trimmed)

	// Replace spaces and underscores with hyphens
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Remove invalid characters
	name = invalidCharsRegex.ReplaceAllString(name, "")

	// Collapse multiple hyphens into single hyphen
	name = multipleHyphensRegex.ReplaceAllString(name, "-")

	// Trim leading and trailing hyphens
	name = strings.Trim(name, "-")

	// Enforce max length
	if len(name) > constants.APIKeyNameMaxLength {
		name = name[:constants.APIKeyNameMaxLength]
		// Trim trailing hyphen if truncation created one
		name = strings.TrimRight(name, "-")
	}

	// If name is too short after sanitization, pad with random hex characters
	if len(name) < constants.APIKeyNameMinLength {
		padding, err := randomHexString(constants.APIKeyNameMinLength - len(name))
		if err != nil {
			return "", fmt.Errorf("failed to generate random padding for short name: %w", err)
		}
		if name == "" {
			name = padding
		} else {
			name = name + "-" + padding
		}
		// Trim again to max length in case padding pushed it over
		if len(name) > constants.APIKeyNameMaxLength {
			name = name[:constants.APIKeyNameMaxLength]
			name = strings.TrimRight(name, "-")
		}
	}

	return name, nil
}

// randomHexString returns a lowercase hex string of exactly n characters.
func randomHexString(n int) (string, error) {
	// Each byte encodes to 2 hex chars, so we need ceil(n/2) bytes
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:n], nil
}

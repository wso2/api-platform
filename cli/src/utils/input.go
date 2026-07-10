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
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// stdinReader is a shared buffered reader over os.Stdin. Prompts must share one
// reader: a bufio.Reader reads ahead, so a fresh reader per prompt can discard
// input buffered by a previous prompt (breaking successive prompts on piped
// input).
var stdinReader = bufio.NewReader(os.Stdin)

// PromptInput prompts the user for input and returns the trimmed response
func PromptInput(prompt string) (string, error) {
	fmt.Print(prompt)
	input, err := stdinReader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(input), nil
}

// PromptSelect presents a numbered list of options and returns the option the
// user selects. The user may enter either the number (1-based) or the option
// value itself (case-insensitive). It re-prompts on invalid input and returns
// an error only when input cannot be read (e.g. EOF).
func PromptSelect(prompt string, options []string) (string, error) {
	fmt.Println(prompt)
	for i, option := range options {
		fmt.Printf("  %d) %s\n", i+1, option)
	}

	for {
		fmt.Printf("Enter number [1-%d]: ", len(options))
		input, err := stdinReader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(input)

		if n, convErr := strconv.Atoi(input); convErr == nil {
			if n >= 1 && n <= len(options) {
				return options[n-1], nil
			}
			fmt.Printf("Invalid selection %q; enter a number between 1 and %d.\n", input, len(options))
			continue
		}

		// Fall back to matching the option value itself.
		for _, option := range options {
			if strings.EqualFold(input, option) {
				return option, nil
			}
		}
		fmt.Printf("Invalid selection %q; enter a number between 1 and %d.\n", input, len(options))
	}
}

// PromptPassword prompts the user for a password with masked input
func PromptPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // Print newline after password input
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	return strings.TrimSpace(string(bytePassword)), nil
}

// PromptCredentials prompts for credentials based on auth type
// Returns username, password, token, and error
// Empty values indicate user chose to use environment variables
func PromptCredentials(authType string) (username, password, token string, err error) {
	switch authType {
	case AuthTypeBasic:
		username, err = PromptInput(fmt.Sprintf("Enter username (leave empty to use %s env var): ", EnvGatewayUsername))
		if err != nil {
			return "", "", "", err
		}

		password, err = PromptPassword(fmt.Sprintf("Enter password (leave empty to use %s env var): ", EnvGatewayPassword))
		if err != nil {
			return "", "", "", err
		}

		return username, password, "", nil

	case AuthTypeBearer:
		token, err = PromptPassword(fmt.Sprintf("Enter token (leave empty to use %s env var): ", EnvGatewayToken))
		if err != nil {
			return "", "", "", err
		}

		return "", "", token, nil

	case AuthTypeNone:
		// No credentials needed
		return "", "", "", nil

	default:
		return "", "", "", fmt.Errorf("unsupported auth type '%s'", authType)
	}
}

// HasCredentials checks if any credentials were provided
func HasCredentials(authType, username, password, token string) bool {
	switch authType {
	case AuthTypeBasic:
		return username != "" && password != ""
	case AuthTypeBearer:
		return token != ""
	case AuthTypeNone:
		return false
	default:
		return false
	}
}

// PromptDevPortalAPIKey prompts for a DevPortal API key with masked input.
// Empty values indicate the user chose to use the environment variable instead.
func PromptDevPortalAPIKey() (string, error) {
	return PromptPassword(fmt.Sprintf("Enter DevPortal API key (leave empty to use %s env var): ", EnvDevPortalAPIKey))
}

// PromptDevPortalCredentials prompts for DevPortal credentials based on auth type.
// Empty values indicate user chose to use environment variables.
func PromptDevPortalCredentials(authType string) (username, password, token, apiKey string, err error) {
	switch authType {
	case AuthTypeBasic:
		username, err = PromptInput(fmt.Sprintf("Enter DevPortal username (leave empty to use %s env var): ", EnvDevPortalUsername))
		if err != nil {
			return "", "", "", "", err
		}

		password, err = PromptPassword(fmt.Sprintf("Enter DevPortal password (leave empty to use %s env var): ", EnvDevPortalPassword))
		if err != nil {
			return "", "", "", "", err
		}

		return username, password, "", "", nil

	case AuthTypeOAuth:
		token, err = PromptPassword(fmt.Sprintf("Enter DevPortal OAuth token (leave empty to use %s env var): ", EnvDevPortalToken))
		if err != nil {
			return "", "", "", "", err
		}

		return "", "", token, "", nil

	case AuthTypeAPIKey:
		apiKey, err = PromptPassword(fmt.Sprintf("Enter DevPortal API key (leave empty to use %s env var): ", EnvDevPortalAPIKey))
		if err != nil {
			return "", "", "", "", err
		}

		return "", "", "", apiKey, nil

	default:
		return "", "", "", "", fmt.Errorf("unsupported devportal auth type '%s'", authType)
	}
}

// PromptAIWorkspaceCredentials prompts for AI workspace credentials based on auth type.
// Empty values indicate user chose to use environment variables.
func PromptAIWorkspaceCredentials(authType string) (username, password, token, apiKey string, err error) {
	switch authType {
	case AuthTypeBasic:
		username, err = PromptInput(fmt.Sprintf("Enter AI workspace username (leave empty to use %s env var): ", EnvAIWorkspaceUsername))
		if err != nil {
			return "", "", "", "", err
		}

		password, err = PromptPassword(fmt.Sprintf("Enter AI workspace password (leave empty to use %s env var): ", EnvAIWorkspacePassword))
		if err != nil {
			return "", "", "", "", err
		}

		return username, password, "", "", nil

	case AuthTypeOAuth:
		token, err = PromptPassword(fmt.Sprintf("Enter AI workspace OAuth token (leave empty to use %s env var): ", EnvAIWorkspaceToken))
		if err != nil {
			return "", "", "", "", err
		}

		return "", "", token, "", nil

	case AuthTypeAPIKey:
		apiKey, err = PromptPassword(fmt.Sprintf("Enter AI workspace API key (leave empty to use %s env var): ", EnvAIWorkspaceAPIKey))
		if err != nil {
			return "", "", "", "", err
		}

		return "", "", "", apiKey, nil

	default:
		return "", "", "", "", fmt.Errorf("unsupported ai-workspace auth type '%s'", authType)
	}
}

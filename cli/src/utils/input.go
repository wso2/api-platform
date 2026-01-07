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
	"strings"
	"syscall"

	"golang.org/x/term"
)

// PromptInput prompts the user for input and returns the trimmed response
func PromptInput(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(input), nil
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
		return username != "" || password != ""
	case AuthTypeBearer:
		return token != ""
	case AuthTypeNone:
		return false
	default:
		return false
	}
}

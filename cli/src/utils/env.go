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
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ResolveEnvVar resolves environment variable references in the format ${VAR_NAME}
// If the input is not an environment variable reference, returns it as-is
// Returns an error if the environment variable is not set or empty
func ResolveEnvVar(value string) (string, error) {
	// Check if value matches ${ENV_VAR_NAME} pattern
	re := regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)
	matches := re.FindStringSubmatch(value)

	if len(matches) == 2 {
		// Extract variable name and resolve from environment
		envVarName := matches[1]
		resolved := os.Getenv(envVarName)
		if resolved == "" {
			return "", fmt.Errorf("environment variable '%s' is not set or empty", envVarName)
		}
		return resolved, nil
	}

	// Not an env var reference, return as-is
	return value, nil
}

// GetPolicyHubBaseURL returns the PolicyHub base URL using the environment
// override `WSO2AP_POLICYHUB_BASE_URL` when set, otherwise returns the default.
func GetPolicyHubBaseURL() string {
	if v := os.Getenv(PolicyHubEnvVar); v != "" {
		return v
	}
	return PolicyHubBaseURLDefault
}

// ValidateAuthEnvVars checks if required environment variables are set for the given auth type
// Returns missing variable names and whether validation passed
func ValidateAuthEnvVars(authType string) (missing []string, ok bool) {
	switch authType {
	case AuthTypeBasic:
		if os.Getenv(EnvGatewayUsername) == "" {
			missing = append(missing, EnvGatewayUsername)
		}
		if os.Getenv(EnvGatewayPassword) == "" {
			missing = append(missing, EnvGatewayPassword)
		}
	case AuthTypeBearer:
		if os.Getenv(EnvGatewayToken) == "" {
			missing = append(missing, EnvGatewayToken)
		}
	case AuthTypeNone:
		// No env vars required
		return nil, true
	default:
		// Unknown auth type - return sentinel to trigger clear error message
		return []string{fmt.Sprintf("UNKNOWN_AUTH_TYPE:%s", authType)}, false
	}

	return missing, len(missing) == 0
}

// FormatMissingEnvVarsWarning formats a warning message for missing environment variables
func FormatMissingEnvVarsWarning(authType string, missing []string) string {
	if len(missing) == 0 {
		return ""
	}

	// Check for unknown auth type sentinel
	if len(missing) == 1 && strings.HasPrefix(missing[0], "UNKNOWN_AUTH_TYPE:") {
		unknownType := strings.TrimPrefix(missing[0], "UNKNOWN_AUTH_TYPE:")
		return fmt.Sprintf("Error: unsupported authentication type '%s'. Valid types: none, basic, bearer\n", unknownType)
	}

	msg := fmt.Sprintf("%s authentication requires the following environment variables:\n", strings.Title(authType))
	for _, envVar := range missing {
		msg += fmt.Sprintf("  %s\n", envVar)
	}
	return msg
}

// FormatMissingEnvVarsError formats an error message for missing environment variables
func FormatMissingEnvVarsError(authType string, missing []string) string {
	if len(missing) == 0 {
		return ""
	}

	msg := fmt.Sprintf("%s authentication requires the following environment variables:\n", strings.Title(authType))
	for _, envVar := range missing {
		msg += fmt.Sprintf("  %s\n", envVar)
	}
	return msg
}

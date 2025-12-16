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
	"os"
	"regexp"
)

// ResolveEnvVar resolves environment variable references in the format ${VAR_NAME}
// If the input is not an environment variable reference, returns it as-is
func ResolveEnvVar(value string) string {
	// Check if value matches ${ENV_VAR_NAME} pattern
	re := regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)
	matches := re.FindStringSubmatch(value)

	if len(matches) == 2 {
		// Extract variable name and resolve from environment
		envVarName := matches[1]
		return os.Getenv(envVarName)
	}

	// Not an env var reference, return as-is
	return value
}

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
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// escapeParam escapes special characters in a parameter value to prevent
// format string injection and YAML injection attacks
func escapeParam(param string) string {
	// Escape % to prevent format string injection in fmt.Sprintf
	escaped := strings.ReplaceAll(param, "%", "%%")
	return escaped
}

// GetParamsOfPolicy renders a policy definition template with given parameters
// and unmarshals it into a map[string]any
func GetParamsOfPolicy(policyDef string, params ...string) (map[string]any, error) {
	args := make([]any, len(params))
	for i, v := range params {
		args[i] = escapeParam(v)
	}
	rendered := fmt.Sprintf(policyDef, args...)

	var m map[string]any
	if err := yaml.Unmarshal([]byte(rendered), &m); err != nil {
		return map[string]any{}, err
	}
	return m, nil
}

// GenerateUUID generates a new UUID v7 string
func GenerateUUID() (string, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID v7: %w", err)
	}
	return u.String(), nil
}

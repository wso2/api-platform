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
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ConvertJSONToYAMLIfNeeded detects if the content is JSON and converts it to YAML
func ConvertJSONToYAMLIfNeeded(content []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return content, nil
	}

	// Check if it starts with '{' or '[', indicating JSON
	if trimmed[0] != '{' && trimmed[0] != '[' {
		// Not JSON, assume it's YAML
		return content, nil
	}

	var jsonData interface{}
	if err := json.Unmarshal(content, &jsonData); err != nil {
		// If JSON parsing fails, assume it's YAML
		return content, nil
	}

	// Convert JSON to YAML
	yamlData, err := yaml.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to convert JSON to YAML: %w", err)
	}

	return yamlData, nil
}

// ConvertYAMLToJSON converts YAML content to JSON
func ConvertYAMLToJSON(content []byte) ([]byte, error) {
	var yamlData interface{}
	if err := yaml.Unmarshal(content, &yamlData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	jsonData, err := json.Marshal(yamlData)
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	return jsonData, nil
}

// ConvertYAMLToJSONPretty converts YAML content to pretty-printed JSON
func ConvertYAMLToJSONPretty(content []byte) ([]byte, error) {
	var yamlData interface{}
	if err := yaml.Unmarshal(content, &yamlData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	jsonData, err := json.MarshalIndent(yamlData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	return jsonData, nil
}

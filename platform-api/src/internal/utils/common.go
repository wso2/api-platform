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
	"archive/zip"
	"bytes"
	"fmt"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// CreateAPIYamlZip creates a ZIP file containing API YAML files
func CreateAPIYamlZip(apiYamlMap map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for apiID, yamlContent := range apiYamlMap {
		fileName := fmt.Sprintf("api-%s.yaml", apiID)
		fileWriter, err := zipWriter.Create(fileName)
		if err != nil {
			if closeErr := zipWriter.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to create file in zip: %w (close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to create file in zip: %w", err)
		}

		_, err = fileWriter.Write([]byte(yamlContent))
		if err != nil {
			if closeErr := zipWriter.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to write file content: %w (close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to write file content: %w", err)
		}
	}

	err := zipWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// CreateLLMProviderYamlZip creates a ZIP file containing LLM provider YAML files
func CreateLLMProviderYamlZip(providerYamlMap map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for providerID, yamlContent := range providerYamlMap {
		fileName := fmt.Sprintf("llm-provider-%s.yaml", providerID)
		fileWriter, err := zipWriter.Create(fileName)
		if err != nil {
			if closeErr := zipWriter.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to create file in zip: %w (close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to create file in zip: %w", err)
		}

		_, err = fileWriter.Write([]byte(yamlContent))
		if err != nil {
			if closeErr := zipWriter.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to write file content: %w (close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to write file content: %w", err)
		}
	}

	err := zipWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// CreateLLMProxyYamlZip creates a ZIP file containing LLM proxy YAML files
func CreateLLMProxyYamlZip(proxyYamlMap map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for proxyID, yamlContent := range proxyYamlMap {
		fileName := fmt.Sprintf("llm-proxy-%s.yaml", proxyID)
		fileWriter, err := zipWriter.Create(fileName)
		if err != nil {
			if closeErr := zipWriter.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to create file in zip: %w (close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to create file in zip: %w", err)
		}

		_, err = fileWriter.Write([]byte(yamlContent))
		if err != nil {
			if closeErr := zipWriter.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to write file content: %w (close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to write file content: %w", err)
		}
	}

	err := zipWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// OpenAPIUUIDToString converts an OpenAPI UUID to string.
func OpenAPIUUIDToString(id openapi_types.UUID) string {
	return uuid.UUID(id).String()
}

// ParseOpenAPIUUID parses a UUID string into an OpenAPI UUID pointer.
func ParseOpenAPIUUID(id string) (*openapi_types.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	openapiUUID := openapi_types.UUID(parsed)
	return &openapiUUID, nil
}

// ParseOptionalOpenAPIUUID parses an optional UUID string pointer into an OpenAPI UUID pointer.
// Returns nil when input is nil, empty, or invalid.
func ParseOptionalOpenAPIUUID(id *string) *openapi_types.UUID {
	if id == nil || *id == "" {
		return nil
	}

	parsed, err := ParseOpenAPIUUID(*id)
	if err != nil {
		return nil
	}

	return parsed
}

// ParseOpenAPIUUIDOrZero parses a UUID string into an OpenAPI UUID value.
// Returns zero UUID when input is invalid.
func ParseOpenAPIUUIDOrZero(id string) openapi_types.UUID {
	parsed, err := ParseOpenAPIUUID(id)
	if err != nil || parsed == nil {
		return openapi_types.UUID{}
	}

	return *parsed
}

// StringPtrIfNotEmpty returns a pointer for non-empty strings.
func StringPtrIfNotEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// defaultStringPtr returns the string value if not nil, otherwise empty string.
func defaultStringPtr(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

// stringSlicePtr returns a pointer to a non-empty string slice or nil for an empty slice.
func stringSlicePtr(values []string) *[]string {
	if len(values) == 0 {
		return nil
	}

	return &values
}

// TimePtrIfNotZero returns a pointer for non-zero timestamps.
func TimePtrIfNotZero(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

// BoolPtr returns a pointer to the provided boolean.
func BoolPtr(value bool) *bool {
	return &value
}

func MapPtrIfNotEmpty(m map[string]interface{}) *map[string]interface{} {
	if len(m) == 0 {
		return nil
	}
	return &m
}

// MapValueOrEmpty returns the map value when non-nil or an empty map otherwise.
func MapValueOrEmpty(m *map[string]interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}

	return *m
}

// StringPtrValue returns the value of a string pointer or empty string if nil
func StringPtrValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// GenerateUUID generates a new UUID v7 string
func GenerateUUID() (string, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID v7: %w", err)
	}
	return u.String(), nil
}

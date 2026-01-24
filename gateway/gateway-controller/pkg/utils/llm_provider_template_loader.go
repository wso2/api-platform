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
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"gopkg.in/yaml.v3"
)

// LLMTemplateLoader loads LLM provider template definitions from files
type LLMTemplateLoader struct {
	logger *slog.Logger
}

// NewLLMTemplateLoader creates a new LLM template loader
func NewLLMTemplateLoader(logger *slog.Logger) *LLMTemplateLoader {
	return &LLMTemplateLoader{
		logger: logger,
	}
}

// LoadTemplatesFromDirectory loads all LLM provider template files from a directory
// Supports both JSON and YAML files
func (tl *LLMTemplateLoader) LoadTemplatesFromDirectory(dirPath string) (map[string]*api.LLMProviderTemplate, error) {
	templates := make(map[string]*api.LLMProviderTemplate)

	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		tl.logger.Warn("LLM templates directory does not exist", slog.String("path", dirPath))
		return templates, nil
	}

	// Walk through the directory
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process JSON and YAML files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			tl.logger.Debug("Skipping non-template file", slog.String("file", path))
			return nil
		}

		// Load the template definition
		template, err := tl.loadTemplateFile(path)
		if err != nil {
			tl.logger.Error("Failed to load template file",
				slog.String("file", path),
				slog.Any("error", err))
			return err
		}

		// Check for duplicates
		templateHandle := template.Metadata.Name
		if _, exists := templates[templateHandle]; exists {
			return fmt.Errorf("duplicate template handle: %s", templateHandle)
		}

		templates[templateHandle] = template
		tl.logger.Info("Loaded LLM provider template",
			slog.String("handle", templateHandle),
			slog.String("apiVersion", string(template.ApiVersion)),
			slog.String("file", path))

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load templates from directory: %w", err)
	}

	tl.logger.Info("Successfully loaded LLM provider templates",
		slog.Int("count", len(templates)),
		slog.String("directory", dirPath))

	return templates, nil
}

// loadTemplateFile loads a single LLM provider template file
func (tl *LLMTemplateLoader) loadTemplateFile(filePath string) (*api.LLMProviderTemplate, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	var templateConfig api.LLMProviderTemplate

	if ext == ".json" {
		if err := json.Unmarshal(data, &templateConfig); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	} else {
		// For YAML, unmarshal to a generic map first, then convert to JSON and unmarshal again
		// This works around the issue where yaml.v3 doesn't use json tags as fallback
		var yamlData map[string]interface{}
		if err := yaml.Unmarshal(data, &yamlData); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}

		// Convert to JSON and unmarshal to get proper field mapping via json tags
		jsonData, err := json.Marshal(yamlData)
		if err != nil {
			return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
		}

		if err := json.Unmarshal(jsonData, &templateConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal converted JSON: %w", err)
		}

		tl.logger.Debug("Parsed template from YAML",
			slog.String("file", filePath),
			slog.String("handle", templateConfig.Metadata.Name),
			slog.String("apiVersion", string(templateConfig.ApiVersion)))
	}

	// Log serialized JSON to see what will be stored
	jsonBytes, _ := json.Marshal(templateConfig)
	tl.logger.Debug("Serialized template to JSON",
		slog.String("file", filePath),
		slog.String("json", string(jsonBytes)))

	return &templateConfig, nil
}
